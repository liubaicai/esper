package esper

import (
	"context"
	"testing"
	"time"
)

// TestRollupOutputSnapshotOrderLimitParity mirrors the
// rollup-output-snapshot-order-limit parity scenario (Java
// ResultSetOutputSnapshotOrderWLimit): an unbound rollup snapshot every
// second is ordered by aggregate value and limited to three rows.
func TestRollupOutputSnapshotOrderLimitParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[rollupOutputLastBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	theString := Field[rollupOutputLastBean, string]("theString")
	intPrimitive := Field[rollupOutputLastBean, int]("intPrimitive")
	plan, err := env.Build(From[rollupOutputLastBean](env, "SupportBean").
		GroupByRollup(theString).
		Select(
			Alias("c0", theString),
			Alias("c1", Sum[int](intPrimitive)),
		).Query(
		StatementName("s0"),
		WithOutput(OutputSnapshotEvery(time.Second)),
		OrderBy(Ascending(ResultField[int]("c1"))),
		Limit(3),
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
	send := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(ctx, rollupOutputLastBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(ctx, time.UnixMilli(0).UTC()); err != nil {
		t.Fatal(err)
	}
	send("E1", 12)
	send("E2", 11)
	send("E3", 10)
	send("E4", 13)
	send("E2", 5)
	if err := engine.AdvanceTime(ctx, time.UnixMilli(1000).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("batches = %d, want 1", len(batches))
	}
	batch := batches[0]
	if len(batch.New) != 3 || len(batch.Old) != 0 {
		t.Fatalf("snapshot-order-limit new/old = %d/%d, want 3/0", len(batch.New), len(batch.Old))
	}
	want := []struct {
		c0 string
		c1 int
	}{
		{"E3", 10},
		{"E1", 12},
		{"E4", 13},
	}
	for index, expected := range want {
		row, ok := batch.New[index].Row()
		if !ok || row.Get("c0").Any() != expected.c0 || row.Get("c1").Any() != expected.c1 {
			t.Fatalf("snapshot-order-limit row %d = %#v, want %#v", index, batch.New[index], expected)
		}
	}
}
