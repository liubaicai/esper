package esper

import (
	"context"
	"reflect"
	"testing"
)

type indexedFirstLastParityBean struct {
	IntPrimitive int `esper:"intPrimitive"`
}

// TestResultSetAggregateFirstLastWindowIndexedParity mirrors
// ResultSetAggregateFirstLastIndexed (ordinal 5). Java runtime:
// java-runtime-30f62dc2e86a7a1e80a0.
func TestResultSetAggregateFirstLastWindowIndexedParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[indexedFirstLastParityBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	value := Field[indexedFirstLastParityBean, int]("intPrimitive")
	plan, err := env.Build(
		From[indexedFirstLastParityBean](env, "SupportBean").Window(LengthWindow(3)).Aggregate(
			Alias("f0", First[int](value, 0)), Alias("f1", First[int](value, 1)),
			Alias("f2", First[int](value, 2)), Alias("f3", First[int](value, 3)),
			Alias("l0", Last[int](value, 0)), Alias("l1", Last[int](value, 1)),
			Alias("l2", Last[int](value, 2)), Alias("l3", Last[int](value, 3)),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{10, 11, 12, 13} {
		if err := engine.SendEvent(context.Background(), indexedFirstLastParityBean{IntPrimitive: n}); err != nil {
			t.Fatal(err)
		}
	}
	want := []map[string]any{
		{"f0": 10, "f1": nil, "f2": nil, "f3": nil, "l0": 10, "l1": nil, "l2": nil, "l3": nil},
		{"f0": 10, "f1": 11, "f2": nil, "f3": nil, "l0": 11, "l1": 10, "l2": nil, "l3": nil},
		{"f0": 10, "f1": 11, "f2": 12, "f3": nil, "l0": 12, "l1": 11, "l2": 10, "l3": nil},
		{"f0": 11, "f1": 12, "f2": 13, "f3": nil, "l0": 13, "l1": 12, "l2": 11, "l3": nil},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %d, want %d", len(rows), len(want))
	}
	for i, row := range rows {
		got := make(map[string]any, len(want[i]))
		for name := range want[i] {
			got[name] = row.Get(name).Any()
		}
		if !reflect.DeepEqual(got, want[i]) {
			t.Fatalf("row %d = %#v, want %#v", i, got, want[i])
		}
	}
}
