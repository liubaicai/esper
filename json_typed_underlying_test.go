package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

type typedJSONChild struct {
	Name   string `json:"name"`
	Scores []int  `json:"scores"`
}

type typedJSONEnvelope struct {
	TheString    string           `json:"theString"`
	IntPrimitive int              `json:"intPrimitive"`
	Optional     *string          `json:"optional,omitempty"`
	Nested       typedJSONChild   `json:"nested"`
	Items        []typedJSONChild `json:"items"`
	Labels       map[string]int   `json:"labels"`
}

func TestTypedJSONSchemaMaterializesStructAndNestedCollections(t *testing.T) {
	schema, err := NewJSONSchemaFor[typedJSONEnvelope]("TypedJSONEnvelope", nil)
	if err != nil {
		t.Fatal(err)
	}
	if schema.Kind() != SchemaJSON || schema.GoType() != reflect.TypeOf(typedJSONEnvelope{}) {
		t.Fatalf("typed JSON schema metadata = kind=%v goType=%v", schema.Kind(), schema.GoType())
	}
	event, err := ParseJSON(schema, []byte(`{
		"theString":"E1",
		"intPrimitive":"10",
		"nested":{"name":"N","scores":[1,"2"]},
		"items":[{"name":"A","scores":[3]},{"name":"B","scores":[4,5]}],
		"labels":{"x":"7"}
	}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	underlying, ok := event.Underlying().(typedJSONEnvelope)
	if !ok {
		t.Fatalf("typed JSON underlying = %T, want typedJSONEnvelope", event.Underlying())
	}
	if underlying.TheString != "E1" || underlying.IntPrimitive != 10 || underlying.Optional != nil {
		t.Fatalf("typed JSON scalar values = %#v", underlying)
	}
	if underlying.Nested.Name != "N" || !reflect.DeepEqual(underlying.Nested.Scores, []int{1, 2}) {
		t.Fatalf("typed JSON nested value = %#v", underlying.Nested)
	}
	if len(underlying.Items) != 2 || underlying.Items[1].Name != "B" || !reflect.DeepEqual(underlying.Labels, map[string]int{"x": 7}) {
		t.Fatalf("typed JSON collection values = %#v", underlying)
	}
	if event.Get("nested.name").Any() != "N" || event.Get("items[1].scores[0]").Any() != 4 {
		t.Fatalf("typed JSON property access = nested=%v item=%v", event.Get("nested.name"), event.Get("items[1].scores[0]"))
	}
	rendered, err := RenderJSON(event)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{`"theString":"E1"`, `"intPrimitive":10`, `"nested"`, `"items"`, `"labels":{"x":7}`} {
		if !strings.Contains(rendered, fragment) {
			t.Fatalf("typed JSON rendering %q is missing %q", rendered, fragment)
		}
	}
}

func TestTypedJSONSchemaSendJSONUsesStructUnderlying(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterJSONFor[typedJSONEnvelope](env, "TypedJSONEngine", nil); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[typedJSONEnvelope](env, "TypedJSONEngine").Query(StatementName("typed-json-engine")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var events []Event
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				events = append(events, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendJSON(context.Background(), "TypedJSONEngine", []byte(`{"theString":"E2","intPrimitive":12,"nested":{"name":"N2"},"items":[],"labels":{}}`)); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("typed JSON Engine events = %d, want 1", len(events))
	}
	underlying, ok := events[0].Underlying().(typedJSONEnvelope)
	if !ok || underlying.TheString != "E2" || underlying.IntPrimitive != 12 || underlying.Nested.Name != "N2" {
		t.Fatalf("typed JSON Engine underlying = %#v (%T)", events[0].Underlying(), events[0].Underlying())
	}
}

func TestTypedJSONSchemaRejectsNonStructUnderlying(t *testing.T) {
	if _, err := NewJSONSchemaFor[int]("InvalidTypedJSON", nil); err == nil {
		t.Fatal("typed JSON schema accepted a non-struct underlying type")
	}
}
