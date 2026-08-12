package esper

import (
	"context"
	"errors"
	"testing"
	"time"
)

func buildDataflowWindowJoin(t *testing.T, env *Environment, name string, options DataflowJoinOptions) DataflowDefinition {
	t.Helper()
	if _, err := RegisterStruct[dataflowJoinLeft](env, "JoinLeft"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dataflowJoinRight](env, "JoinRight"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, name).
		Emitter("left").
		Emitter("right").
		SelectJoin("select", options,
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
	return definition
}

func startDataflowWindowJoin(t *testing.T, engine *Engine, definition DataflowDefinition) (*DataflowInstance, *DataflowEmitter, *DataflowEmitter) {
	t.Helper()
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	captive, err := instance.StartCaptive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	left, leftOK := captive.Emitter("left")
	right, rightOK := captive.Emitter("right")
	if !leftOK || !rightOK {
		t.Fatalf("captive join emitters = %#v", captive.Emitters())
	}
	t.Cleanup(func() { _ = instance.Cancel(context.Background()) })
	return instance, left, right
}

func TestDataflowSelectJoinLengthWindowsEmitOnlyNewCartesianRows(t *testing.T) {
	env := NewEnvironment()
	options := (DataflowJoinOptions{Inputs: 2, Retention: DataflowJoinKeepAll}).
		WithLength(0, 2).
		WithLength(1, 2)
	definition := buildDataflowWindowJoin(t, env, "dataflow-join-length", options)
	instance, left, right := startDataflowWindowJoin(t, NewEngine(env), definition)

	for _, value := range []int{1, 2} {
		if err := left.Submit(context.Background(), dataflowJoinLeft{ID: value}); err != nil {
			t.Fatal(err)
		}
	}
	if err := right.Submit(context.Background(), dataflowJoinRight{ID: 10}); err != nil {
		t.Fatal(err)
	}
	if err := left.Submit(context.Background(), dataflowJoinLeft{ID: 3}); err != nil {
		t.Fatal(err)
	}
	if err := right.Submit(context.Background(), dataflowJoinRight{ID: 20}); err != nil {
		t.Fatal(err)
	}
	if err := right.Submit(context.Background(), dataflowJoinRight{ID: 30}); err != nil {
		t.Fatal(err)
	}

	want := [][2]int{{1, 10}, {2, 10}, {3, 10}, {2, 20}, {3, 20}, {2, 30}, {3, 30}}
	outputs := instance.Outputs()
	if len(outputs) != len(want) {
		t.Fatalf("length-window join outputs = %#v, want %d rows", outputs, len(want))
	}
	for index, pair := range want {
		assertDataflowJoinRow(t, outputs, index, pair[0], pair[1])
	}
}

func TestDataflowSelectFullOuterLengthEvictionEmitsUnmatchedTransitions(t *testing.T) {
	env := NewEnvironment()
	options := (DataflowJoinOptions{Inputs: 2, Kind: DataflowJoinFullOuter, Retention: DataflowJoinKeepAll}).
		On(OnSourcesEqual(0, Field[dataflowJoinLeft, int]("id"), 1, Field[dataflowJoinRight, int]("id"))).
		WithLength(1, 1)
	definition := buildDataflowWindowJoin(t, env, "dataflow-join-outer-length", options)
	instance, left, right := startDataflowWindowJoin(t, NewEngine(env), definition)

	if err := left.Submit(context.Background(), dataflowJoinLeft{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := right.Submit(context.Background(), dataflowJoinRight{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := right.Submit(context.Background(), dataflowJoinRight{ID: 2}); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 4 {
		t.Fatalf("outer length-window outputs = %#v, want 4 rows", outputs)
	}
	assertDataflowJoinOptionalRow(t, outputs[0], Present(1), Null())
	assertDataflowJoinOptionalRow(t, outputs[1], Present(1), Present(1))
	assertDataflowJoinOptionalRow(t, outputs[2], Present(1), Null())
	assertDataflowJoinOptionalRow(t, outputs[3], Null(), Present(2))
}

func TestDataflowSelectJoinTimeWindowExpiresAtExactVirtualBoundary(t *testing.T) {
	base := time.Unix(0, 0).UTC()
	env := NewEnvironment()
	options := (DataflowJoinOptions{Inputs: 2, Kind: DataflowJoinFullOuter, Retention: DataflowJoinKeepAll}).
		On(OnSourcesEqual(0, Field[dataflowJoinLeft, int]("id"), 1, Field[dataflowJoinRight, int]("id"))).
		WithTime(1, 5*time.Second)
	definition := buildDataflowWindowJoin(t, env, "dataflow-join-time", options)
	engine := NewEngine(env, WithStartTime(base))
	instance, left, right := startDataflowWindowJoin(t, engine, definition)

	if err := left.Submit(context.Background(), dataflowJoinLeft{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := right.Submit(context.Background(), dataflowJoinRight{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), base.Add(5*time.Second-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if outputs := instance.Outputs(); len(outputs) != 2 {
		t.Fatalf("time-window join expired early: %#v", outputs)
	}
	if err := engine.AdvanceTime(context.Background(), base.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 3 {
		t.Fatalf("time-window join outputs at boundary = %#v, want 3 rows", outputs)
	}
	assertDataflowJoinOptionalRow(t, outputs[2], Present(1), Null())
}

func TestDataflowSelectJoinWindowsRejectInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name    string
		windows []DataflowJoinWindow
	}{
		{name: "negative input", windows: []DataflowJoinWindow{{Input: -1, Length: 1}}},
		{name: "input out of range", windows: []DataflowJoinWindow{{Input: 2, Length: 1}}},
		{name: "empty policy", windows: []DataflowJoinWindow{{Input: 0}}},
		{name: "negative length", windows: []DataflowJoinWindow{{Input: 0, Length: -1}}},
		{name: "negative duration", windows: []DataflowJoinWindow{{Input: 0, Duration: -time.Second}}},
		{name: "combined policy", windows: []DataflowJoinWindow{{Input: 0, Length: 1, Duration: time.Second}}},
		{name: "duplicate input", windows: []DataflowJoinWindow{{Input: 0, Length: 1}, {Input: 0, Duration: time.Second}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[dataflowJoinLeft](env, "JoinLeft"); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[dataflowJoinRight](env, "JoinRight"); err != nil {
				t.Fatal(err)
			}
			_, err := DefineDataflow(env, "invalid-dataflow-join-window").
				Emitter("left").
				Emitter("right").
				SelectJoin("select", DataflowJoinOptions{Inputs: 2, Windows: test.windows},
					Alias("leftID", JoinField[int](0, "id")),
					Alias("rightID", JoinField[int](1, "id")),
				).
				Emitter("sink").
				ConnectInput("left", "select", 0).
				ConnectInput("right", "select", 1).
				Connect("select", "sink").
				Build()
			if err == nil || !errors.Is(err, ErrorInvalidRule) {
				t.Fatalf("invalid join window error = %v", err)
			}
		})
	}

	replaced := (DataflowJoinOptions{}).WithLength(0, 2).WithTime(0, time.Second)
	if len(replaced.Windows) != 1 || replaced.Windows[0].Length != 0 || replaced.Windows[0].Duration != time.Second {
		t.Fatalf("same-input fluent window replacement = %#v", replaced.Windows)
	}
}

func TestDataflowSelectJoinWindowsEnterStablePlanIdentity(t *testing.T) {
	build := func(options DataflowJoinOptions) Plan {
		t.Helper()
		env := NewEnvironment()
		buildDataflowWindowJoin(t, env, "dataflow-join-window-plan", options)
		plan, err := env.Build(From[dataflowJoinLeft](env, "JoinLeft").Query())
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	first := build((DataflowJoinOptions{Inputs: 2}).WithLength(0, 2).WithTime(1, time.Second))
	reordered := build((DataflowJoinOptions{Inputs: 2}).WithTime(1, time.Second).WithLength(0, 2))
	changed := build((DataflowJoinOptions{Inputs: 2}).WithLength(0, 3).WithTime(1, time.Second))
	if first.Hash() != reordered.Hash() || string(first.Canonical()) != string(reordered.Canonical()) {
		t.Fatal("equivalent join window declaration order changed plan identity")
	}
	if first.Hash() == changed.Hash() || string(first.Canonical()) == string(changed.Canonical()) {
		t.Fatal("different join windows share plan identity")
	}
}

func assertDataflowJoinOptionalRow(t *testing.T, output any, left, right Value) {
	t.Helper()
	row, ok := output.(Row)
	if !ok {
		t.Fatalf("join output = %#v, want Row", output)
	}
	if got := row.Get("leftID"); !got.Equal(left) {
		t.Fatalf("join leftID = %v, want %v", got, left)
	}
	if got := row.Get("rightID"); !got.Equal(right) {
		t.Fatalf("join rightID = %v, want %v", got, right)
	}
}
