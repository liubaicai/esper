package esper

import (
	"context"
	"testing"
)

func TestDataflowSelectInnerJoinConditionFiltersAndComparesMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dataflowJoinLeft](env, "JoinLeft"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dataflowJoinRight](env, "JoinRight"); err != nil {
		t.Fatal(err)
	}
	condition := AllJoin(
		OnSourcesEqual(0, Field[dataflowJoinLeft, int]("id"), 1, Field[dataflowJoinRight, int]("id")),
		OnSourcesCompare(0, Field[dataflowJoinLeft, int]("id"), 1, Field[dataflowJoinRight, int]("id"), JoinGreaterOrEqual),
	)
	definition, err := DefineDataflow(env, "dataflow-select-join-condition").
		EventBusSource("left", "JoinLeft").
		EventBusSource("right", "JoinRight").
		SelectJoin("select", DataflowJoinOptions{Inputs: 2}.On(condition),
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

	if err := engine.SendEvent(context.Background(), dataflowJoinLeft{ID: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 12}); err != nil {
		t.Fatal(err)
	}
	if got := instance.Outputs(); len(got) != 0 {
		t.Fatalf("join condition emitted out-of-range row = %#v", got)
	}
	if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 10}); err != nil {
		t.Fatal(err)
	}
	assertDataflowJoinRow(t, instance.Outputs(), 0, 10, 10)
}

func TestDataflowSelectInnerJoinAnyConditionMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dataflowJoinLeft](env, "JoinLeft"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dataflowJoinRight](env, "JoinRight"); err != nil {
		t.Fatal(err)
	}
	condition := AnyJoin(
		OnSourcesEqual(0, Field[dataflowJoinLeft, int]("id"), 1, Field[dataflowJoinRight, int]("id")),
		OnSourcesCompare(0, Field[dataflowJoinLeft, int]("id"), 1, Field[dataflowJoinRight, int]("id"), JoinGreaterOrEqual),
	)
	definition, err := DefineDataflow(env, "dataflow-select-join-any-condition").
		EventBusSource("left", "JoinLeft").
		EventBusSource("right", "JoinRight").
		SelectJoin("select", DataflowJoinOptions{Inputs: 2}.On(condition),
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

	if err := engine.SendEvent(context.Background(), dataflowJoinLeft{ID: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 12}); err != nil {
		t.Fatal(err)
	}
	if got := instance.Outputs(); len(got) != 0 {
		t.Fatalf("any join condition emitted false row = %#v", got)
	}
	if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 8}); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 2 {
		t.Fatalf("any join condition output count = %d, want 2: %#v", len(outputs), outputs)
	}
	assertDataflowJoinRow(t, outputs, 0, 10, 10)
	assertDataflowJoinRow(t, outputs, 1, 10, 8)
}

func TestDataflowSelectJoinConditionRejectsInvalidSource(t *testing.T) {
	env := NewEnvironment()
	_, err := DefineDataflow(env, "dataflow-select-join-invalid-condition").
		Emitter("left").
		Emitter("right").
		SelectJoin("select", DataflowJoinOptions{Inputs: 2}.On(
			OnSourcesEqual(0, Field[dataflowJoinLeft, int]("id"), 2, Field[dataflowJoinRight, int]("id")),
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
		t.Fatal("SelectJoin Build accepted a condition source outside the input range")
	}
}
