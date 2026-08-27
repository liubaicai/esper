package esper

import (
	"context"
	"reflect"
	"testing"
)

type prevNthParityBean struct {
	IntPrimitive int `esper:"intPrimitive"`
}

// TestResultSetAggregatePrevNthIndexedFirstLastParity mirrors
// ResultSetAggregatePrevNthIndexedFirstLast. Java runtime:
// java-runtime-3733f40a5c6d7be2c175.
func TestResultSetAggregatePrevNthIndexedFirstLastParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[prevNthParityBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	value := Field[prevNthParityBean, int]("intPrimitive")
	plan, err := env.Build(
		From[prevNthParityBean](env, "SupportBean").Window(LengthWindow(3)).Aggregate(
			Alias("p0", Prev[int](0, value)), Alias("p1", Prev[int](1, value)), Alias("p2", Prev[int](2, value)),
			Alias("n0", Nth[int](value, 0)), Alias("n1", Nth[int](value, 1)), Alias("n2", Nth[int](value, 2)),
			Alias("l1", Last[int](value, 0)), Alias("l2", Last[int](value, 1)), Alias("l3", Last[int](value, 2)),
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
		if err := engine.SendEvent(context.Background(), prevNthParityBean{IntPrimitive: n}); err != nil {
			t.Fatal(err)
		}
	}
	want := []map[string]any{
		{"p0": 10, "p1": nil, "p2": nil, "n0": 10, "n1": nil, "n2": nil, "l1": 10, "l2": nil, "l3": nil},
		{"p0": 11, "p1": 10, "p2": nil, "n0": 11, "n1": 10, "n2": nil, "l1": 11, "l2": 10, "l3": nil},
		{"p0": 12, "p1": 11, "p2": 10, "n0": 12, "n1": 11, "n2": 10, "l1": 12, "l2": 11, "l3": 10},
		{"p0": 13, "p1": 12, "p2": 11, "n0": 13, "n1": 12, "n2": 11, "l1": 13, "l2": 12, "l3": 11},
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
