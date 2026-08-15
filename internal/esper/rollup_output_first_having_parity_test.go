package esper

import (
	"context"
	"testing"
	"time"
)

// TestRollupOutputFirstHavingParity mirrors the rollup-output-first-having
// parity scenario (Java ResultSetOutputFirstHaving{join=false}): grouped
// rollup with having sum > 100 and output first every second.
func TestRollupOutputFirstHavingParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[rollupOutputLastBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	theString := Field[rollupOutputLastBean, string]("theString")
	intPrimitive := Field[rollupOutputLastBean, int]("intPrimitive")
	sum := Sum[int64](Field[rollupOutputLastBean, int64]("longBoxed"))
	plan, err := env.Build(From[rollupOutputLastBean](env, "SupportBean").Window(TimeWindow(3500*time.Millisecond)).
		GroupByRollup(theString, intPrimitive).
		Having(Greater[int64](sum, Literal(int64(100)))).
		Select(
			Alias("c0", theString),
			Alias("c1", intPrimitive),
			Alias("c2", sum),
		).Query(
		StatementName("s0"),
		WithOldStream(),
		WithOutput(OutputFirstEveryTime(time.Second)),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	advance := func(ms int64) {
		t.Helper()
		if err := engine.AdvanceTime(ctx, time.UnixMilli(ms).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	send := func(s string, i int, l int64) {
		t.Helper()
		if err := engine.SendEvent(ctx, rollupOutputLastBean{TheString: s, IntPrimitive: i, LongBoxed: l}); err != nil {
			t.Fatal(err)
		}
	}
	advance(0)
	send("E1", 1, 10)
	send("E1", 2, 20)
	send("E1", 1, 30)
	advance(1000)
	send("E2", 1, 40)
	send("E1", 2, 50)
	advance(2000)
	send("E1", 1, 60)
	advance(3000)
	send("E1", 1, 70)
	advance(4000)
	send("E1", 1, 80)
	advance(5000)
	send("E1", 1, 90)
	advance(6000)

	if len(batches) != 7 {
		t.Fatalf("batches = %d, want 7", len(batches))
	}
	want := [][][]any{
		{{"E1", nil, int64(110)}, {nil, nil, int64(150)}},
		{{"E1", nil, int64(170)}, {nil, nil, int64(210)}},
		{{"E1", 1, int64(170)}, {"E1", nil, int64(240)}, {nil, nil, int64(280)}},
		{{"E1", 1, int64(130)}, {"E1", nil, int64(180)}, {nil, nil, int64(220)}},
		{{"E1", nil, int64(210)}, {nil, nil, int64(210)}},
		{{"E1", 1, int64(300)}},
		{{"E1", 1, int64(240)}, {"E1", nil, int64(240)}, {nil, nil, int64(240)}},
	}
	for index, batch := range batches {
		if len(batch.New) != len(want[index]) || len(batch.Old) != len(want[index]) {
			t.Fatalf("batch %d new/old = %d/%d, want %d/%d", index, len(batch.New), len(batch.Old), len(want[index]), len(want[index]))
		}
		for sideIndex, side := range [][]Result{batch.New, batch.Old} {
			for rowIndex, rowWant := range want[index] {
				row, ok := side[rowIndex].Row()
				if !ok {
					t.Fatalf("batch %d side %d row %d is not a row", index, sideIndex, rowIndex)
				}
				if (rowWant[0] == nil && !row.Get("c0").IsNull()) || (rowWant[0] != nil && (row.Get("c0").IsNull() || row.Get("c0").Any() != rowWant[0])) {
					t.Fatalf("batch %d side %d row %d c0 = %#v, want %#v", index, sideIndex, rowIndex, row.Get("c0"), rowWant[0])
				}
				if (rowWant[1] == nil && !row.Get("c1").IsNull()) || (rowWant[1] != nil && (row.Get("c1").IsNull() || row.Get("c1").Any() != rowWant[1])) {
					t.Fatalf("batch %d side %d row %d c1 = %#v, want %#v", index, sideIndex, rowIndex, row.Get("c1"), rowWant[1])
				}
				if row.Get("c2").IsNull() || row.Get("c2").Any() != rowWant[2] {
					t.Fatalf("batch %d side %d row %d c2 = %#v, want %#v", index, sideIndex, rowIndex, row.Get("c2"), rowWant[2])
				}
			}
		}
	}
}
