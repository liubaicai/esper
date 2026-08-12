package esper

import (
	"context"
	"testing"
)

// TestDataflowSelectJoinCaptiveEmittersMatchEsper keeps the Java dataflow
// shape explicit: two captive Emitter sources feed indexed SelectJoin ports,
// and the join waits until both source streams have supplied an event.
func TestDataflowSelectJoinCaptiveEmittersMatchEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dataflowJoinLeft](env, "JoinLeft"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dataflowJoinRight](env, "JoinRight"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "dataflow-select-join-captive").
		Emitter("left").
		Emitter("right").
		SelectJoin("select", DataflowJoinOptions{Inputs: 2},
			Alias("leftID", JoinField[int](0, "id")),
			Alias("rightID", JoinField[int](1, "id")),
		).
		Emitter("sink").
		ConnectInput("left", "select", 0).
		ConnectInput("right", "select", 1).
		Connect("select", "sink").
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
	left, err := instance.CaptiveEmitter("left")
	if err != nil {
		t.Fatal(err)
	}
	right, err := instance.CaptiveEmitter("right")
	if err != nil {
		t.Fatal(err)
	}
	if err := left.Submit(context.Background(), dataflowJoinLeft{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if len(instance.Outputs()) != 0 {
		t.Fatalf("captive join emitted before second source: %#v", instance.Outputs())
	}
	if err := right.Submit(context.Background(), dataflowJoinRight{ID: 10}); err != nil {
		t.Fatal(err)
	}
	assertDataflowJoinRow(t, instance.Outputs(), 0, 1, 10)
}
