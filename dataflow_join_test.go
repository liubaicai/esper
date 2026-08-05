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

func TestDataflowSelectLeftOuterJoinEmitsOnlyLeftUnmatchedRowsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dataflowJoinLeft](env, "JoinLeft"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dataflowJoinRight](env, "JoinRight"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "dataflow-select-left-outer").
		EventBusSource("left", "JoinLeft").
		EventBusSource("right", "JoinRight").
		SelectJoin("select", DataflowJoinOptions{Inputs: 2, Kind: DataflowJoinLeftOuter},
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
	if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 10}); err != nil {
		t.Fatal(err)
	}
	assertDataflowJoinRow(t, instance.Outputs(), 1, 1, 10)
	if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 20}); err != nil {
		t.Fatal(err)
	}
	assertDataflowJoinRow(t, instance.Outputs(), 2, 1, 20)
	if len(instance.Outputs()) != 3 {
		t.Fatalf("left outer emitted a right-only row = %#v", instance.Outputs())
	}
}

func TestDataflowSelectRightOuterJoinEmitsOnlyRightUnmatchedRowsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dataflowJoinLeft](env, "JoinLeft"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dataflowJoinRight](env, "JoinRight"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "dataflow-select-right-outer").
		EventBusSource("left", "JoinLeft").
		EventBusSource("right", "JoinRight").
		SelectJoin("select", DataflowJoinOptions{Inputs: 2, Kind: DataflowJoinRightOuter},
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

	if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 10}); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 1 {
		t.Fatalf("right outer unmatched output count = %d, want 1: %#v", len(outputs), outputs)
	}
	if row, ok := outputs[0].(Row); !ok || !row.Get("leftID").IsNull() || row.Get("rightID").Any() != 10 {
		t.Fatalf("right outer unmatched row = %#v", outputs[0])
	}
	if err := engine.SendEvent(context.Background(), dataflowJoinLeft{ID: 1}); err != nil {
		t.Fatal(err)
	}
	assertDataflowJoinRow(t, instance.Outputs(), 1, 1, 10)
	if err := engine.SendEvent(context.Background(), dataflowJoinLeft{ID: 2}); err != nil {
		t.Fatal(err)
	}
	assertDataflowJoinRow(t, instance.Outputs(), 2, 2, 10)
	if len(instance.Outputs()) != 3 {
		t.Fatalf("right outer emitted a left-only row = %#v", instance.Outputs())
	}
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

func TestDataflowSelectJoinConditionsMatchEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dataflowJoinLeft](env, "JoinLeft"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dataflowJoinRight](env, "JoinRight"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "dataflow-select-join-condition").
		EventBusSource("left", "JoinLeft").
		EventBusSource("right", "JoinRight").
		SelectJoin("select", DataflowJoinOptions{Inputs: 2}.On(
			OnSourcesEqual(0, Field[dataflowJoinLeft, int]("id"), 1, Field[dataflowJoinRight, int]("id")),
		),
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
	if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 2}); err != nil {
		t.Fatal(err)
	}
	if got := instance.Outputs(); len(got) != 0 {
		t.Fatalf("inner join emitted non-matching latest tuple = %#v", got)
	}
	if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 1}); err != nil {
		t.Fatal(err)
	}
	assertDataflowJoinRow(t, instance.Outputs(), 0, 1, 1)
	if err := engine.SendEvent(context.Background(), dataflowJoinLeft{ID: 3}); err != nil {
		t.Fatal(err)
	}
	if len(instance.Outputs()) != 1 {
		t.Fatalf("conditioned join emitted stale match after latest replacement = %#v", instance.Outputs())
	}
	if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 3}); err != nil {
		t.Fatal(err)
	}
	assertDataflowJoinRow(t, instance.Outputs(), 1, 3, 3)
}

func TestDataflowSelectFullOuterJoinConditionEmitsUnmatchedInput(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dataflowJoinLeft](env, "JoinLeft"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dataflowJoinRight](env, "JoinRight"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "dataflow-select-join-outer-condition").
		EventBusSource("left", "JoinLeft").
		EventBusSource("right", "JoinRight").
		SelectJoin("select", DataflowJoinOptions{
			Inputs:    2,
			Kind:      DataflowJoinFullOuter,
			Retention: DataflowJoinKeepAll,
		}.On(OnSourcesEqual(0, Field[dataflowJoinLeft, int]("id"), 1, Field[dataflowJoinRight, int]("id"))),
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
	if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 2}); err != nil {
		t.Fatal(err)
	}
	if len(instance.Outputs()) != 2 {
		t.Fatalf("conditioned full outer unmatched output count = %d, want 2: %#v", len(instance.Outputs()), instance.Outputs())
	}
	assertDataflowJoinRow(t, instance.Outputs(), 0, 1, 0)
	if row, ok := instance.Outputs()[1].(Row); !ok || !row.Get("leftID").IsNull() || row.Get("rightID").Any() != 2 {
		t.Fatalf("conditioned full outer right-unmatched row = %#v", instance.Outputs()[1])
	}

	if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if len(instance.Outputs()) != 3 {
		t.Fatalf("conditioned full outer matching output count = %d, want 3: %#v", len(instance.Outputs()), instance.Outputs())
	}
	assertDataflowJoinRow(t, instance.Outputs(), 2, 1, 1)
}

func TestDataflowSelectJoinRejectsInvalidCondition(t *testing.T) {
	env := NewEnvironment()
	_, err := DefineDataflow(env, "dataflow-select-join-invalid-condition").
		BeaconSource("left").
		BeaconSource("right").
		SelectJoin("select", DataflowJoinOptions{Inputs: 2}.On(
			OnSourcesEqual(0, Field[any, int]("id"), 2, Field[any, int]("id")),
		),
			Alias("leftID", JoinField[int](0, "id")),
			Alias("rightID", JoinField[int](1, "id")),
		).
		Emitter("sink").
		ConnectInput("left", "select", 0).
		ConnectInput("right", "select", 1).
		Connect("select", "sink").
		Build()
	if err == nil {
		t.Fatal("SelectJoin Build accepted a condition source outside the configured inputs")
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
