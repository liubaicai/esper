package esper

import (
	"context"
	"testing"
)

func TestDataflowEventBusSinkReentersAnotherSource(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[runtimeTestTrade](env, "TradeCopy"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "event-bus-reentry").
		EventBusSource("input", "Trade").
		EventBusSink("route", "TradeCopy").
		EventBusSource("routed", "TradeCopy").
		Emitter("sink").
		Connect("input", "route").
		Connect("routed", "sink").
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

	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "reentered"}); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 1 {
		t.Fatalf("reentrant dataflow outputs = %#v", outputs)
	}
	event, ok := outputs[0].(Event)
	if !ok || event.TypeName() != "TradeCopy" || event.Underlying().(runtimeTestTrade).Symbol != "reentered" {
		t.Fatalf("reentrant dataflow output = %#v", outputs[0])
	}
}
