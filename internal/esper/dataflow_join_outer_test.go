package esper

import (
	"context"
	"testing"
)

func TestDataflowSelectLeftAndRightOuterLastEventJoinsMatchEsper(t *testing.T) {
	tests := []struct {
		name string
		kind DataflowJoinKind
	}{
		{name: "left", kind: DataflowJoinLeftOuter},
		{name: "right", kind: DataflowJoinRightOuter},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[dataflowJoinLeft](env, "JoinLeft"); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[dataflowJoinRight](env, "JoinRight"); err != nil {
				t.Fatal(err)
			}
			definition, err := DefineDataflow(env, "dataflow-select-"+testCase.name+"-outer-lastevent").
				EventBusSource("left", "JoinLeft").
				EventBusSource("right", "JoinRight").
				SelectJoin("select", DataflowJoinOptions{Inputs: 2, Kind: testCase.kind},
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

			if testCase.kind == DataflowJoinLeftOuter {
				if err := engine.SendEvent(context.Background(), dataflowJoinLeft{ID: 1}); err != nil {
					t.Fatal(err)
				}
				assertDataflowJoinRow(t, instance.Outputs(), 0, 1, 0)
				if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 10}); err != nil {
					t.Fatal(err)
				}
				assertDataflowJoinRow(t, instance.Outputs(), 1, 1, 10)
				return
			}

			if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 10}); err != nil {
				t.Fatal(err)
			}
			if len(instance.Outputs()) != 1 {
				// RightOuter has no left-side row to retain yet; the right event
				// is nevertheless the selected outer edge and emits Null-left.
				t.Fatalf("right outer output count = %d, want 1: %#v", len(instance.Outputs()), instance.Outputs())
			}
			assertDataflowRightOuterRow(t, instance.Outputs(), 0, 0, 10)
			if err := engine.SendEvent(context.Background(), dataflowJoinLeft{ID: 1}); err != nil {
				t.Fatal(err)
			}
			assertDataflowRightOuterRow(t, instance.Outputs(), 1, 1, 10)
		})
	}
}

func TestDataflowSelectLeftAndRightOuterKeepAllJoinsMatchEsper(t *testing.T) {
	tests := []struct {
		name string
		kind DataflowJoinKind
	}{
		{name: "left", kind: DataflowJoinLeftOuter},
		{name: "right", kind: DataflowJoinRightOuter},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[dataflowJoinLeft](env, "JoinLeft"); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[dataflowJoinRight](env, "JoinRight"); err != nil {
				t.Fatal(err)
			}
			definition, err := DefineDataflow(env, "dataflow-select-"+testCase.name+"-outer-keepall").
				EventBusSource("left", "JoinLeft").
				EventBusSource("right", "JoinRight").
				SelectJoin("select", DataflowJoinOptions{
					Inputs:    2,
					Kind:      testCase.kind,
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

			if testCase.kind == DataflowJoinLeftOuter {
				if err := engine.SendEvent(context.Background(), dataflowJoinLeft{ID: 1}); err != nil {
					t.Fatal(err)
				}
				if err := engine.SendEvent(context.Background(), dataflowJoinLeft{ID: 2}); err != nil {
					t.Fatal(err)
				}
				assertDataflowJoinRow(t, instance.Outputs(), 0, 1, 0)
				assertDataflowJoinRow(t, instance.Outputs(), 1, 2, 0)
				if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 10}); err != nil {
					t.Fatal(err)
				}
				if len(instance.Outputs()) != 4 {
					t.Fatalf("left outer keep-all output count = %d, want 4: %#v", len(instance.Outputs()), instance.Outputs())
				}
				assertDataflowJoinRow(t, instance.Outputs(), 2, 1, 10)
				assertDataflowJoinRow(t, instance.Outputs(), 3, 2, 10)
				return
			}

			if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 10}); err != nil {
				t.Fatal(err)
			}
			assertDataflowRightOuterRow(t, instance.Outputs(), 0, 0, 10)
			if err := engine.SendEvent(context.Background(), dataflowJoinRight{ID: 11}); err != nil {
				t.Fatal(err)
			}
			assertDataflowRightOuterRow(t, instance.Outputs(), 1, 0, 11)
			if err := engine.SendEvent(context.Background(), dataflowJoinLeft{ID: 1}); err != nil {
				t.Fatal(err)
			}
			if len(instance.Outputs()) != 4 {
				t.Fatalf("right outer keep-all output count = %d, want 4: %#v", len(instance.Outputs()), instance.Outputs())
			}
			assertDataflowRightOuterRow(t, instance.Outputs(), 2, 1, 10)
			assertDataflowRightOuterRow(t, instance.Outputs(), 3, 1, 11)
		})
	}
}

func assertDataflowRightOuterRow(t *testing.T, outputs []any, index, left, right int) {
	t.Helper()
	if index < 0 || index >= len(outputs) {
		t.Fatalf("right outer output index %d missing in %#v", index, outputs)
	}
	row, ok := outputs[index].(Row)
	if !ok {
		t.Fatalf("right outer output %d = %#v, want Row", index, outputs[index])
	}
	if left == 0 {
		if !row.Get("leftID").IsNull() {
			t.Fatalf("right outer leftID[%d] = %#v, want Null", index, row.Get("leftID").Any())
		}
	} else if got, ok := row.Get("leftID").Any().(int); !ok || got != left {
		t.Fatalf("right outer leftID[%d] = %#v, want %d", index, row.Get("leftID").Any(), left)
	}
	if got, ok := row.Get("rightID").Any().(int); !ok || got != right {
		t.Fatalf("right outer rightID[%d] = %#v, want %d", index, row.Get("rightID").Any(), right)
	}
}
