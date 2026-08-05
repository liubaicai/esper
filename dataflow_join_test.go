package esper

import (
	"context"
	"testing"
)

type dataflowJoinLeft struct {
	ID int `esper:"id"`
}

type dataflowJoinRight struct {
	ID int `esper:"id"`
}

type dataflowJoinThird struct {
	ID int `esper:"id"`
}

func TestDataflowSelectInnerLastEventJoinWaitsForAllInputsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dataflowJoinLeft](env, "JoinLeft"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dataflowJoinRight](env, "JoinRight"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dataflowJoinThird](env, "JoinThird"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "dataflow-select-join-inner").
		EventBusSource("left", "JoinLeft").
		EventBusSource("right", "JoinRight").
		EventBusSource("third", "JoinThird").
		SelectJoin("select", DataflowJoinOptions{Inputs: 3},
			Alias("leftID", JoinField[int](0, "id")),
			Alias("rightID", JoinField[int](1, "id")),
			Alias("thirdID", JoinField[int](2, "id")),
		).
		Emitter("sink").
		ConnectInput("left", "select", 0).
		ConnectInput("right", "select", 1).
		ConnectInput("third", "select", 2).
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

	if err := engine.SendEvent(context.Background(), dataflowJoinLeft{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 10}); err != nil {
		t.Fatal(err)
	}
	if got := instance.Outputs(); len(got) != 0 {
		t.Fatalf("inner join emitted before all inputs arrived = %#v", got)
	}
	if err := engine.SendEvent(context.Background(), dataflowJoinThird{ID: 100}); err != nil {
		t.Fatal(err)
	}
	assertDataflowJoinRow3(t, instance.Outputs(), 0, 1, 10, 100)

	if err := engine.SendEvent(context.Background(), dataflowJoinThird{ID: 101}); err != nil {
		t.Fatal(err)
	}
	assertDataflowJoinRow3(t, instance.Outputs(), 1, 1, 10, 101)
}

func TestDataflowSelectFullOuterJoinEmitsMissingInputMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dataflowJoinLeft](env, "JoinLeft"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dataflowJoinRight](env, "JoinRight"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "dataflow-select-join-outer").
		EventBusSource("left", "JoinLeft").
		EventBusSource("right", "JoinRight").
		SelectJoin("select", DataflowJoinOptions{
			Inputs:    2,
			Kind:      DataflowJoinFullOuter,
			Retention: DataflowJoinKeepAll,
		},
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

	if err := engine.SendEvent(context.Background(), dataflowJoinLeft{ID: 1}); err != nil {
		t.Fatal(err)
	}
	assertDataflowJoinRow(t, instance.Outputs(), 0, 1, 0)
	if row, ok := instance.Outputs()[0].(Row); !ok || !row.Get("rightID").IsNull() {
		t.Fatalf("full outer missing side = %#v", instance.Outputs()[0])
	}

	if err := engine.SendEvent(context.Background(), dataflowJoinLeft{ID: 2}); err != nil {
		t.Fatal(err)
	}
	assertDataflowJoinRow(t, instance.Outputs(), 1, 2, 0)
	if len(instance.Outputs()) != 2 {
		t.Fatalf("full outer replayed old unmatched rows = %#v", instance.Outputs())
	}

	if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 10}); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 4 {
		t.Fatalf("full outer Cartesian output count = %d, want 4: %#v", len(outputs), outputs)
	}
	assertDataflowJoinRow(t, outputs, 2, 1, 10)
	assertDataflowJoinRow(t, outputs, 3, 2, 10)
}

func TestDataflowSelectJoinRejectsInvalidShape(t *testing.T) {
	env := NewEnvironment()
	_, err := DefineDataflow(env, "dataflow-select-join-invalid").
		BeaconSource("left").
		SelectJoin("select", DataflowJoinOptions{Inputs: 2},
			Alias("leftID", JoinField[int](0, "id")),
			Alias("rightID", JoinField[int](1, "id")),
		).
		Emitter("sink").
		ConnectInput("left", "select", 0).
		Connect("select", "sink").
		Build()
	if err == nil {
		t.Fatal("SelectJoin Build accepted a missing input port")
	}
}

func assertDataflowJoinRow(t *testing.T, outputs []any, index, left, right int) {
	t.Helper()
	if index < 0 || index >= len(outputs) {
		t.Fatalf("join output index %d missing in %#v", index, outputs)
	}
	row, ok := outputs[index].(Row)
	if !ok {
		t.Fatalf("join output %d = %#v, want Row", index, outputs[index])
	}
	if leftValue, ok := row.Get("leftID").Any().(int); !ok || leftValue != left {
		t.Fatalf("join leftID[%d] = %#v, want %d", index, row.Get("leftID").Any(), left)
	}
	if right == 0 {
		if !row.Get("rightID").IsNull() {
			t.Fatalf("join rightID[%d] = %#v, want Null", index, row.Get("rightID").Any())
		}
		return
	}
	if rightValue, ok := row.Get("rightID").Any().(int); !ok || rightValue != right {
		t.Fatalf("join rightID[%d] = %#v, want %d", index, row.Get("rightID").Any(), right)
	}
}

func assertDataflowJoinRow3(t *testing.T, outputs []any, index, left, right, third int) {
	t.Helper()
	if index < 0 || index >= len(outputs) {
		t.Fatalf("three-way join output index %d missing in %#v", index, outputs)
	}
	row, ok := outputs[index].(Row)
	if !ok {
		t.Fatalf("three-way join output %d = %#v, want Row", index, outputs[index])
	}
	for name, want := range map[string]int{"leftID": left, "rightID": right, "thirdID": third} {
		got, ok := row.Get(name).Any().(int)
		if !ok || got != want {
			t.Fatalf("three-way join %s[%d] = %#v, want %d", name, index, row.Get(name).Any(), want)
		}
	}
}
