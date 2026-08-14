package esper

import (
	"context"
	"fmt"
	"testing"
)

type highCardinalityContextBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestHighCardinalityContextParity mirrors the shared high-cardinality-context
// parity scenario (Java ContextKeySegmentedLargeNumberPartitions reduced to 40
// groups): each unique key creates a segmented partition and the grouped
// aggregate retains per-partition state.
func TestHighCardinalityContextParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[highCardinalityContextBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "SegmentedByAString",
		Field[highCardinalityContextBean, string]("theString")); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[highCardinalityContextBean](env, "SupportBean").Aggregate(
		Alias("col1", Sum[int](Field[highCardinalityContextBean, int]("intPrimitive"))),
	).Query(StatementName("s0"), WithContext("SegmentedByAString")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	ctx := context.Background()
	snapshotCols := func(t *testing.T) []int {
		t.Helper()
		result, err := statement.Snapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		cols := make([]int, 0, len(result.Batch.New))
		for _, item := range result.Batch.New {
			row, ok := item.Row()
			if !ok {
				t.Fatalf("snapshot result is not a row: %#v", item)
			}
			cols = append(cols, row.Get("col1").Any().(int))
		}
		return cols
	}
	assertCols := func(t *testing.T, got []int, want []int) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("cols = %#v, want %#v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("cols[%d] = %d, want %d", i, got[i], want[i])
			}
		}
	}
	send := func(theString string, primitive int) {
		t.Helper()
		if err := engine.SendEvent(ctx, highCardinalityContextBean{TheString: theString, IntPrimitive: primitive}); err != nil {
			t.Fatal(err)
		}
	}
	first := make([]int, 40)
	for i := range first {
		first[i] = i + 1
		send(fmt.Sprintf("E%02d", i), i+1)
	}
	assertCols(t, snapshotCols(t), first)
	for i := 0; i < 10; i++ {
		send(fmt.Sprintf("E%02d", i), 100)
	}
	want := append([]int(nil), first...)
	for i := 0; i < 10; i++ {
		want[i] = 101 + i
	}
	assertCols(t, snapshotCols(t), want)
}
