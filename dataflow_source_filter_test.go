package esper

import (
	"context"
	"testing"
)

func TestDataflowEventBusSourceWithFilterRoutesOnlyMatchingEventsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "eventbus-source-filter").
		EventBusSourceWithFilter("source", "Trade", StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal("A"))).
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer instance.Cancel(context.Background())
	// Use the instance's engine so the test also exercises the normal EventBus
	// dispatch path used by SendEvent.
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B1", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A1", Price: 2}); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 1 {
		t.Fatalf("source filter outputs = %#v, want one matching event", outputs)
	}
	event, ok := outputs[0].(Event)
	if !ok || event.Get("symbol").Any() != "A1" {
		t.Fatalf("source filter output = %#v, want A1 event", outputs[0])
	}
}

func TestDataflowEventBusSourceWithFilterWorksOnLinearPathMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[runtimeTestTrade](env, "Trade")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	first, err := newEvent(schema, runtimeTestTrade{Symbol: "B1", Price: 1}, engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	second, err := newEvent(schema, runtimeTestTrade{Symbol: "A1", Price: 2}, engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "eventbus-source-filter-linear").
		BeaconSource("beacon", first, second).
		EventBusSourceWithFilter("source", "Trade", StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal("A"))).
		Emitter("sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer instance.Cancel(context.Background())
	outputs := instance.Outputs()
	if len(outputs) != 1 {
		t.Fatalf("linear source filter outputs = %#v, want one matching event", outputs)
	}
	event, ok := outputs[0].(Event)
	if !ok || event.Get("symbol").Any() != "A1" {
		t.Fatalf("linear source filter output = %#v, want A1 event", outputs[0])
	}
}

func TestDataflowEventBusSourceWithFilterRejectsNonBooleanPredicate(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := DefineDataflow(env, "eventbus-source-filter-invalid").
		EventBusSourceWithFilter("source", "Trade", Literal("not-bool")).
		Emitter("sink").
		Connect("source", "sink").
		Build(); err == nil {
		t.Fatal("non-boolean EventBusSource filter was accepted")
	}
}
