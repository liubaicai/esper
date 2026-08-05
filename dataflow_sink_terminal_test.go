package esper

import (
	"context"
	"testing"
)

func TestDataflowEventBusSinkIsTerminalAndRejectsOutputEdges(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[runtimeTestTrade](env, "TradeCopy"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "eventbus-sink-terminal").
		EventBusSource("source", "Trade").
		EventBusSink("sink", "TradeCopy").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, operator := range definition.operators {
		if operator.Name == "sink" && len(operator.OutputPorts) != 0 {
			t.Fatalf("EventBusSink output ports = %#v, want none", operator.OutputPorts)
		}
	}
	if _, err := DefineDataflow(env, "eventbus-sink-linear-output").
		EventBusSource("source", "Trade").
		EventBusSink("sink", "TradeCopy").
		Emitter("after").
		Build(); err == nil {
		t.Fatal("EventBusSink with a linear output operator was accepted")
	}
	if _, err := DefineDataflow(env, "eventbus-sink-graph-output").
		Emitter("source").
		EventBusSink("sink", "TradeCopy").
		Emitter("after").
		Connect("source", "sink").
		Connect("sink", "after").
		Build(); err == nil {
		t.Fatal("EventBusSink graph output edge was accepted")
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
	if err := engine.Send(context.Background(), "Trade", runtimeTestTrade{Symbol: "terminal", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if instance.Stats().Emitted != 0 {
		t.Fatalf("terminal EventBusSink emitted dataflow output: %#v", instance.Stats())
	}
}
