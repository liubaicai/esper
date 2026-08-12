package esper

import (
	"context"
	"testing"
)

func TestDataflowSelectJoinOrderPermutationsMatchEsper(t *testing.T) {
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

	orders := [][]int{{0, 1, 2}, {2, 1, 0}, {1, 2, 0}}
	for orderIndex, order := range orders {
		t.Run(testJoinOrderName(orderIndex), func(t *testing.T) {
			inputBySource := make(map[int]int, len(order))
			for input, source := range order {
				inputBySource[source] = input
			}
			definition, err := DefineDataflow(env, testJoinOrderName(orderIndex)).
				Emitter("s0").
				Emitter("s1").
				Emitter("s2").
				SelectJoin("select", DataflowJoinOptions{Inputs: 3},
					Alias("s0id", JoinField[int](inputBySource[0], "id")),
					Alias("s1id", JoinField[int](inputBySource[1], "id")),
					Alias("s2id", JoinField[int](inputBySource[2], "id")),
				).
				Emitter("sink").
				ConnectInput("s0", "select", inputBySource[0]).
				ConnectInput("s1", "select", inputBySource[1]).
				ConnectInput("s2", "select", inputBySource[2]).
				Connect("select", "sink").
				Build()
			if err != nil {
				t.Fatal(err)
			}
			instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
			if err != nil {
				t.Fatal(err)
			}
			if err := instance.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			defer instance.Cancel(context.Background())

			sources := make([]*DataflowEmitter, 3)
			for source, name := range []string{"s0", "s1", "s2"} {
				sources[source], err = instance.CaptiveEmitter(name)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := sources[0].Submit(context.Background(), dataflowJoinLeft{ID: 1}); err != nil {
				t.Fatal(err)
			}
			if err := sources[1].Submit(context.Background(), dataflowJoinRight{ID: 10}); err != nil {
				t.Fatal(err)
			}
			if len(instance.Outputs()) != 0 {
				t.Fatalf("join order %v emitted before third source: %#v", order, instance.Outputs())
			}
			if err := sources[2].Submit(context.Background(), dataflowJoinThird{ID: 100}); err != nil {
				t.Fatal(err)
			}
			outputs := instance.Outputs()
			if len(outputs) != 1 {
				t.Fatalf("join order %v output count = %d, want 1: %#v", order, len(outputs), outputs)
			}
			row, ok := outputs[0].(Row)
			if !ok {
				t.Fatalf("join order %v output = %#v, want Row", order, outputs[0])
			}
			for name, want := range map[string]int{"s0id": 1, "s1id": 10, "s2id": 100} {
				if got, ok := row.Get(name).Any().(int); !ok || got != want {
					t.Fatalf("join order %v %s = %#v, want %d", order, name, row.Get(name).Any(), want)
				}
			}
		})
	}
}

func testJoinOrderName(index int) string {
	return "order-" + string(rune('a'+index))
}
