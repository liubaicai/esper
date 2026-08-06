package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

type containedSplitInput struct {
	Value string `esper:"value"`
}

type containedSplitBase struct{}

type containedSplitA struct {
	P0 string `esper:"p0"`
}

type containedSplitB struct {
	P1 string `esper:"p1"`
}

type containedSplitRawInput struct {
	Rows  []map[string]any `esper:"rows"`
	JSONs []string         `esper:"jsons"`
}

func TestContainedSplitEventArrayPreservesVariantMembersMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[containedSplitInput](env, "ContainedSplitInput"); err != nil {
		t.Fatal(err)
	}
	baseSchema, err := RegisterStruct[containedSplitBase](env, "ContainedSplitBase")
	if err != nil {
		t.Fatal(err)
	}
	aSchema, err := RegisterStruct[containedSplitA](env, "ContainedSplitA", WithSchemaParent(baseSchema))
	if err != nil {
		t.Fatal(err)
	}
	bSchema, err := RegisterStruct[containedSplitB](env, "ContainedSplitB", WithSchemaParent(baseSchema))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterVariant(env, "ContainedSplitVariant", aSchema, bSchema); err != nil {
		t.Fatal(err)
	}

	children := Func1[[]string, []Event]("split-event-array", func(tokens []string) []Event {
		result := make([]Event, 0, len(tokens))
		for _, token := range tokens {
			if len(token) < 2 {
				continue
			}
			var event Event
			if token[0] == 'A' {
				event, _ = NewEvent(aSchema, containedSplitA{P0: token[1:]}, time.Time{})
			} else if token[0] == 'B' {
				event, _ = NewEvent(bSchema, containedSplitB{P1: token[1:]}, time.Time{})
			}
			if event.Schema().valid() {
				result = append(result, event)
			}
		}
		return result
	}, Split(Field[containedSplitInput, string]("value"), Literal(",")))
	stream := UnnestEvents(
		From[containedSplitInput](env, "ContainedSplitInput"),
		children,
		"ContainedSplitVariant",
	)
	plan, err := env.Build(Select(stream,
		Alias("event", EventValue[Event]()),
		Alias("parentValue", ContainedParentField[string]("value")),
	).Query(StatementName("contained-split-event-array")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("contained split event result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), containedSplitInput{Value: "AE1,BE2,AE3"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("contained split event rows = %#v", rows)
	}
	wantTypes := []string{"ContainedSplitA", "ContainedSplitB", "ContainedSplitA"}
	for index, wantType := range wantTypes {
		event, ok := rows[index].Get("event").Any().(Event)
		if !ok || event.TypeName() != wantType {
			t.Fatalf("contained split event %d = %#v, want type %s", index, rows[index].Get("event"), wantType)
		}
		if rows[index].Get("parentValue").Any() != "AE1,BE2,AE3" {
			t.Fatalf("contained split parent %d = %#v", index, rows[index].Get("parentValue"))
		}
		if wantType == "ContainedSplitB" {
			if event.Get("p1").Any() != "E2" {
				t.Fatalf("contained split B event = %#v", event)
			}
		} else if event.Get("p0").Any() == nil {
			t.Fatalf("contained split A event = %#v", event)
		}
	}
}

func TestContainedTypeMaterializesMapAndJSONElementsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[containedSplitRawInput](env, "ContainedSplitRawInput"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[containedSplitA](env, "ContainedSplitWord"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterJSON(env, "ContainedSplitJSON", []FieldSpec{{Name: "p0", Type: reflect.TypeOf("")}}); err != nil {
		t.Fatal(err)
	}

	input := From[containedSplitRawInput](env, "ContainedSplitRawInput")
	mapStream := UnnestAs[containedSplitRawInput, map[string]any](
		input,
		Property[[]map[string]any](EventValue[containedSplitRawInput](), "rows"),
		"ContainedSplitWord",
	)
	jsonStream := UnnestAs[containedSplitRawInput, string](
		input,
		Property[[]string](EventValue[containedSplitRawInput](), "jsons"),
		"ContainedSplitJSON",
	)

	mapPlan, err := env.Build(Select(mapStream,
		Alias("value", Field[Event, string]("p0")),
		Alias("parent", ContainedParentField[[]string]("jsons")),
	).Query(StatementName("contained-type-map")))
	if err != nil {
		t.Fatal(err)
	}
	jsonPlan, err := env.Build(Select(jsonStream,
		Alias("value", Field[Event, string]("p0")),
		Alias("parent", ContainedParentField[[]string]("jsons")),
	).Query(StatementName("contained-type-json")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	mapDeployment, err := engine.Deploy(context.Background(), mapPlan)
	if err != nil {
		t.Fatal(err)
	}
	jsonDeployment, err := engine.Deploy(context.Background(), jsonPlan)
	if err != nil {
		t.Fatal(err)
	}
	var mapRows, jsonRows []Row
	if _, err := mapDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("contained map materialization is not a row: %#v", result)
			}
			mapRows = append(mapRows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := jsonDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("contained JSON materialization is not a row: %#v", result)
			}
			jsonRows = append(jsonRows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), containedSplitRawInput{
		Rows:  []map[string]any{{"p0": "map-1"}, {"p0": "map-2"}},
		JSONs: []string{`{"p0":"json-1"}`, `{"p0":"json-2"}`},
	}); err != nil {
		t.Fatal(err)
	}
	if len(mapRows) != 2 || mapRows[0].Get("value").Any() != "map-1" || mapRows[1].Get("value").Any() != "map-2" {
		t.Fatalf("contained map rows = %#v", mapRows)
	}
	if len(jsonRows) != 2 || jsonRows[0].Get("value").Any() != "json-1" || jsonRows[1].Get("value").Any() != "json-2" {
		t.Fatalf("contained JSON rows = %#v", jsonRows)
	}
	if parent, ok := jsonRows[0].Get("parent").Any().([]string); !ok || !reflect.DeepEqual(parent, []string{`{"p0":"json-1"}`, `{"p0":"json-2"}`}) {
		t.Fatalf("contained JSON parent = %#v", jsonRows[0].Get("parent"))
	}
}

func TestContainedTypeMaterializesMapElementsInFireAndForgetMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	inputSchema, err := RegisterStruct[containedSplitRawInput](env, "ContainedSplitFAFInput")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[containedSplitA](env, "ContainedSplitFAFWord"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "ContainedSplitFAFWindow", inputSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if err := engine.InsertNamedWindow(context.Background(), "ContainedSplitFAFWindow", containedSplitRawInput{
		Rows: []map[string]any{{"p0": "faf-1"}, {"p0": "faf-2"}},
	}); err != nil {
		t.Fatal(err)
	}
	stream := UnnestAs[containedSplitRawInput, map[string]any](
		FromNamedWindowAs[containedSplitRawInput](env, "ContainedSplitFAFWindow"),
		Property[[]map[string]any](EventValue[containedSplitRawInput](), "rows"),
		"ContainedSplitFAFWord",
	)
	plan, err := env.Build(Select(stream,
		Alias("value", Field[Event, string]("p0")),
	).Query(StatementName("contained-type-faf-map")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 2 {
		t.Fatalf("contained FAF map results = %#v", result.Results())
	}
	for index, want := range []string{"faf-1", "faf-2"} {
		row, ok := result.Results()[index].Row()
		if !ok || row.Get("value").Any() != want {
			t.Fatalf("contained FAF map row %d = %#v, want %s", index, result.Results()[index], want)
		}
	}
}

func TestContainedTypeRejectsUnknownAndIncompatibleTargets(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[containedSplitInput](env, "ContainedSplitInput"); err != nil {
		t.Fatal(err)
	}
	baseSchema, err := RegisterStruct[containedSplitBase](env, "ContainedSplitBase")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[containedSplitA](env, "ContainedSplitA", WithSchemaParent(baseSchema)); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[containedSplitB](env, "ContainedSplitB", WithSchemaParent(baseSchema)); err != nil {
		t.Fatal(err)
	}
	property := Func1[string, []Event]("invalid-split", func(value string) []Event {
		event, _ := NewEvent(envMustSchema(env, "ContainedSplitB"), containedSplitB{P1: value}, time.Time{})
		return []Event{event}
	}, Field[containedSplitInput, string]("value"))
	if _, err := env.Build(UnnestEvents(From[containedSplitInput](env, "ContainedSplitInput"), property, "MissingType").Query()); err == nil {
		t.Fatal("expected unknown @type target to fail Build")
	}
	if _, err := env.Build(UnnestEvents(From[containedSplitInput](env, "ContainedSplitInput"), property, "").Query()); err == nil {
		t.Fatal("expected empty @type target to fail Build")
	}

	plan, err := env.Build(UnnestEvents(From[containedSplitInput](env, "ContainedSplitInput"), property, "ContainedSplitA").Query(StatementName("contained-type-incompatible")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), containedSplitInput{Value: "x"}); err == nil {
		t.Fatal("expected incompatible contained Event target to fail at runtime")
	}
}

func TestContainedSplitRejectsUnsupportedExpressionForms(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[containedSplitInput](env, "ContainedInvalidSplitInput"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[containedSplitA](env, "ContainedInvalidSplitWord"); err != nil {
		t.Fatal(err)
	}
	innerSchema, err := RegisterMap(env, "ContainedInvalidSplitInner", []FieldSpec{
		FieldDef("value", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "ContainedInvalidSplitInnerWindow", innerSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	input := From[containedSplitInput](env, "ContainedInvalidSplitInput")
	assertInvalid := func(name string, property Expression[[]string], want string) {
		t.Helper()
		_, buildErr := env.Build(UnnestAs[containedSplitInput, string](input, property, "ContainedInvalidSplitWord").Query(StatementName(name)))
		if buildErr == nil || !strings.Contains(buildErr.Error(), want) {
			t.Fatalf("contained invalid rule %s error = %v, want substring %q", name, buildErr, want)
		}
	}

	assertInvalid("contained-invalid-subquery", SubqueryValues[string](
		FromNamedWindow(env, "ContainedInvalidSplitInnerWindow"),
		Field[any, string]("value"),
	), "does not support subqueries")
	assertInvalid("contained-invalid-aggregate", Func1[int64, []string](
		"aggregate-split",
		func(int64) []string { return nil },
		CountAll(),
	), "does not support aggregation")
	assertInvalid("contained-invalid-previous", Func1[string, []string](
		"previous-split",
		func(string) []string { return nil },
		Prev[string](0, Field[containedSplitInput, string]("value")),
	), "does not support previous or prior access")
}

func envMustSchema(env *Environment, name string) Schema {
	schema, ok := env.Schema(name)
	if !ok {
		panic("schema " + name + " is not registered")
	}
	return schema
}
