package esper

import (
	"context"
	"testing"
	"time"
)

type rollupOutputEveryBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	LongBoxed    int64  `esper:"longBoxed"`
}

// TestRollupOutputEveryParity mirrors the shared rollup-output-every parity
// scenario (Java ResultSetOutputDefault{join=false}): an irstream rollup over
// a 3.5s time window emits every second with new/old rows for all rollup
// levels.
func TestRollupOutputEveryParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[rollupOutputEveryBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	theString := Field[rollupOutputEveryBean, string]("theString")
	intPrimitive := Field[rollupOutputEveryBean, int]("intPrimitive")
	plan, err := env.Build(From[rollupOutputEveryBean](env, "SupportBean").Window(TimeWindow(3500*time.Millisecond)).
		GroupByRollup(theString, intPrimitive).
		Select(
			Alias("c0", theString),
			Alias("c1", intPrimitive),
			Alias("c2", Sum[int64](Field[rollupOutputEveryBean, int64]("longBoxed"))),
		).Query(
		StatementName("s0"),
		WithOldStream(),
		WithOutput(OutputEveryTime(time.Second)),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
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
		if err := engine.SendEvent(ctx, rollupOutputEveryBean{TheString: s, IntPrimitive: i, LongBoxed: l}); err != nil {
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

	if len(batches) != 3 {
		t.Fatalf("batches = %d, want 3", len(batches))
	}
	wantCounts := [][2]int{{9, 9}, {6, 6}, {3, 3}}
	for index, counts := range wantCounts {
		if len(batches[index].New) != counts[0] || len(batches[index].Old) != counts[1] {
			t.Fatalf("batch %d new/old = %d/%d, want %d/%d", index, len(batches[index].New), len(batches[index].Old), counts[0], counts[1])
		}
	}
	last := batches[2]
	row, ok := last.New[0].Row()
	if !ok || row.Get("c0").Any() != "E1" || row.Get("c1").Any() != 1 || row.Get("c2").Any() != int64(100) {
		t.Fatalf("last new row = %#v", last.New[0])
	}
}
