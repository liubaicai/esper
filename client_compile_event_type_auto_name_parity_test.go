package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type MyAutoNameEvent struct {
	P0 string `esper:"p0"`
}

type clientCompileAutoNameAmbiguousOne struct{}
type clientCompileAutoNameAmbiguousTwo struct{}
type clientCompileAutoNameConflictOne struct{}
type clientCompileAutoNameConflictTwo struct{}
type clientCompileAutoNameExpected struct{}
type clientCompileAutoNameOther struct{}

func TestClientCompileEventTypeAutoNameResolveMatchesEsper(t *testing.T) {
	registry := NewEventTypeAutoNameRegistry()
	if err := RegisterAutoNameType[MyAutoNameEvent](registry); err != nil {
		t.Fatal(err)
	}
	env := NewEnvironment()
	schema, err := RegisterAutoNamedStruct[MyAutoNameEvent](env, registry, "MANE")
	if err != nil {
		t.Fatal(err)
	}
	if schema.Name() != "MANE" || schema.GoType() != reflect.TypeOf(MyAutoNameEvent{}) {
		t.Fatalf("auto-named schema = name %q type %v", schema.Name(), schema.GoType())
	}

	plan, err := env.Build(Select(
		From[MyAutoNameEvent](env, "MANE"),
		Alias("p0", Field[MyAutoNameEvent, string]("p0")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var values []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("auto-name result is not a row: %#v", result)
			}
			value, ok := row.Get("p0").Any().(string)
			if !ok {
				t.Fatalf("auto-name p0 = %#v", row.Get("p0"))
			}
			values = append(values, value)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "MANE", MyAutoNameEvent{P0: "test"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values, []string{"test"}) {
		t.Fatalf("auto-name values = %v, want [test]", values)
	}
}

func TestClientCompileEventTypeAutoNameAmbiguousMatchesEsper(t *testing.T) {
	registry := NewEventTypeAutoNameRegistry()
	const shortName = "SupportAmbigousEventType"
	if err := RegisterAutoNameType[clientCompileAutoNameAmbiguousOne](registry,
		WithAutoName(shortName),
		WithAutoNameNamespace("com.espertech.esper.regressionlib.support.autoname.one"),
	); err != nil {
		t.Fatal(err)
	}
	if err := RegisterAutoNameType[clientCompileAutoNameAmbiguousTwo](registry,
		WithAutoName(shortName),
		WithAutoNameNamespace("com.espertech.esper.regressionlib.support.autoname.two"),
	); err != nil {
		t.Fatal(err)
	}
	_, err := registry.Resolve(shortName)
	if err == nil || !errors.Is(err, ErrorAmbiguous) {
		t.Fatalf("ambiguous auto-name error = %v", err)
	}
	message := err.Error()
	one := strings.Index(message, "com.espertech.esper.regressionlib.support.autoname.one")
	two := strings.Index(message, "com.espertech.esper.regressionlib.support.autoname.two")
	if one < 0 || two < 0 || one >= two {
		t.Fatalf("ambiguous auto-name namespaces are not stable and sorted: %v", err)
	}
}

func TestEventTypeAutoNameRegistryRejectsInvalidAndConflictingRegistrations(t *testing.T) {
	var zero EventTypeAutoNameRegistry
	if err := RegisterAutoNameType[MyAutoNameEvent](&zero); err != nil {
		t.Fatal(err)
	}
	if err := RegisterAutoNameType[MyAutoNameEvent](&zero); err != nil {
		t.Fatalf("duplicate registration must be idempotent: %v", err)
	}
	resolved, err := zero.Resolve("MyAutoNameEvent")
	if err != nil || resolved != reflect.TypeOf(MyAutoNameEvent{}) {
		t.Fatalf("zero-value registry resolve = %v, %v", resolved, err)
	}

	var wait sync.WaitGroup
	for range 32 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := RegisterAutoNameType[MyAutoNameEvent](&zero); err != nil {
				t.Errorf("concurrent idempotent registration: %v", err)
			}
			if _, err := zero.Resolve("MyAutoNameEvent"); err != nil {
				t.Errorf("concurrent resolve: %v", err)
			}
		}()
	}
	wait.Wait()

	if err := RegisterAutoNameType[clientCompileAutoNameConflictOne](&zero,
		WithAutoName("Conflict"), WithAutoNameNamespace("example.events"),
	); err != nil {
		t.Fatal(err)
	}
	if err := RegisterAutoNameType[clientCompileAutoNameConflictTwo](&zero,
		WithAutoName("Conflict"), WithAutoNameNamespace("example.events"),
	); err == nil || !errors.Is(err, ErrorAmbiguous) {
		t.Fatalf("same-namespace conflict error = %v", err)
	}
	conflict, err := zero.Resolve("Conflict")
	if err != nil || conflict != reflect.TypeOf(clientCompileAutoNameConflictOne{}) {
		t.Fatalf("failed registration changed prior candidate = %v, %v", conflict, err)
	}

	invalid := []struct {
		name string
		err  error
	}{
		{name: "nil registry register", err: RegisterAutoNameType[MyAutoNameEvent](nil)},
		{name: "blank short name", err: RegisterAutoNameType[clientCompileAutoNameConflictOne](&zero, WithAutoName(" "))},
		{name: "blank namespace", err: RegisterAutoNameType[clientCompileAutoNameConflictOne](&zero, WithAutoNameNamespace(" "))},
		{name: "non-struct type", err: RegisterAutoNameType[int](&zero)},
	}
	for _, test := range invalid {
		if test.err == nil {
			t.Fatalf("%s unexpectedly succeeded", test.name)
		}
	}
	var nilRegistry *EventTypeAutoNameRegistry
	if _, err := nilRegistry.Resolve("MyAutoNameEvent"); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("nil registry resolve error = %v", err)
	}
	if _, err := zero.Resolve(" "); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("blank resolve error = %v", err)
	}
	if _, err := zero.Resolve("Missing"); err == nil || !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("missing resolve error = %v", err)
	}

	mismatch := NewEventTypeAutoNameRegistry()
	if err := RegisterAutoNameType[clientCompileAutoNameOther](mismatch,
		WithAutoName("clientCompileAutoNameExpected"),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterAutoNamedStruct[clientCompileAutoNameExpected](NewEnvironment(), mismatch, "Expected"); err == nil || !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("resolved type mismatch error = %v", err)
	}
}
