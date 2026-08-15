package esper

import (
	"context"
	"testing"
	"time"
)

// TestRollupOutputAllParity mirrors the rollup-output-all parity scenario
// (Java ResultSetOutputAll{join=false}): a 3.5s SupportBean rollup with
// output all every second, including groups that became empty.
func TestRollupOutputAllParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[rollupOutputLastBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	theString := Field[rollupOutputLastBean, string]("theString")
	intPrimitive := Field[rollupOutputLastBean, int]("intPrimitive")
	plan, err := env.Build(From[rollupOutputLastBean](env, "SupportBean").Window(TimeWindow(3500*time.Millisecond)).
		GroupByRollup(theString, intPrimitive).
		Select(
			Alias("c0", theString),
			Alias("c1", intPrimitive),
			Alias("c2", Sum[int64](Field[rollupOutputLastBean, int64]("longBoxed"))),
		).Query(
		StatementName("s0"),
		WithOldStream(),
		WithOutput(OutputAllEveryTime(time.Second)),
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

	if len(batches) != 6 {
		t.Fatalf("batches = %d, want 6", len(batches))
	}
	wantNew := [][]rollupOutputLastRowWant{
		{
			{c0: "E1", c1: 1, c2: int64(40)},
			{c0: "E1", c1: 2, c2: int64(20)},
			{c0: "E1", c1: nil, c1Null: true, c2: int64(60)},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: int64(60)},
		},
		{
			{c0: "E1", c1: 1, c2: int64(40)},
			{c0: "E1", c1: 2, c2: int64(70)},
			{c0: "E2", c1: 1, c2: int64(40)},
			{c0: "E1", c1: nil, c1Null: true, c2: int64(110)},
			{c0: "E2", c1: nil, c1Null: true, c2: int64(40)},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: int64(150)},
		},
		{
			{c0: "E1", c1: 1, c2: int64(100)},
			{c0: "E1", c1: 2, c2: int64(70)},
			{c0: "E2", c1: 1, c2: int64(40)},
			{c0: "E1", c1: nil, c1Null: true, c2: int64(170)},
			{c0: "E2", c1: nil, c1Null: true, c2: int64(40)},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: int64(210)},
		},
		{
			{c0: "E1", c1: 1, c2: int64(130)},
			{c0: "E1", c1: 2, c2: int64(50)},
			{c0: "E2", c1: 1, c2: int64(40)},
			{c0: "E1", c1: nil, c1Null: true, c2: int64(180)},
			{c0: "E2", c1: nil, c1Null: true, c2: int64(40)},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: int64(220)},
		},
		{
			{c0: "E1", c1: 1, c2: int64(210)},
			{c0: "E1", c1: 2, c2: nil, c2Null: true},
			{c0: "E2", c1: 1, c2: nil, c2Null: true},
			{c0: "E1", c1: nil, c1Null: true, c2: int64(210)},
			{c0: "E2", c1: nil, c1Null: true, c2: nil, c2Null: true},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: int64(210)},
		},
		{
			{c0: "E1", c1: 1, c2: int64(240)},
			{c0: "E1", c1: 2, c2: nil, c2Null: true},
			{c0: "E2", c1: 1, c2: nil, c2Null: true},
			{c0: "E1", c1: nil, c1Null: true, c2: int64(240)},
			{c0: "E2", c1: nil, c1Null: true, c2: nil, c2Null: true},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: int64(240)},
		},
	}
	wantOld := [][]rollupOutputLastRowWant{
		{
			{c0: "E1", c1: 1, c2: nil, c2Null: true},
			{c0: "E1", c1: 2, c2: nil, c2Null: true},
			{c0: "E1", c1: nil, c1Null: true, c2: nil, c2Null: true},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: nil, c2Null: true},
		},
		{
			{c0: "E1", c1: 1, c2: int64(40)},
			{c0: "E1", c1: 2, c2: int64(20)},
			{c0: "E2", c1: 1, c2: nil, c2Null: true},
			{c0: "E1", c1: nil, c1Null: true, c2: int64(60)},
			{c0: "E2", c1: nil, c1Null: true, c2: nil, c2Null: true},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: int64(60)},
		},
		{
			{c0: "E1", c1: 1, c2: int64(40)},
			{c0: "E1", c1: 2, c2: int64(70)},
			{c0: "E2", c1: 1, c2: int64(40)},
			{c0: "E1", c1: nil, c1Null: true, c2: int64(110)},
			{c0: "E2", c1: nil, c1Null: true, c2: int64(40)},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: int64(150)},
		},
		{
			{c0: "E1", c1: 1, c2: int64(100)},
			{c0: "E1", c1: 2, c2: int64(70)},
			{c0: "E2", c1: 1, c2: int64(40)},
			{c0: "E1", c1: nil, c1Null: true, c2: int64(170)},
			{c0: "E2", c1: nil, c1Null: true, c2: int64(40)},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: int64(210)},
		},
		{
			{c0: "E1", c1: 1, c2: int64(130)},
			{c0: "E1", c1: 2, c2: int64(50)},
			{c0: "E2", c1: 1, c2: int64(40)},
			{c0: "E1", c1: nil, c1Null: true, c2: int64(180)},
			{c0: "E2", c1: nil, c1Null: true, c2: int64(40)},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: int64(220)},
		},
		{
			{c0: "E1", c1: 1, c2: int64(210)},
			{c0: "E1", c1: 2, c2: nil, c2Null: true},
			{c0: "E2", c1: 1, c2: nil, c2Null: true},
			{c0: "E1", c1: nil, c1Null: true, c2: int64(210)},
			{c0: "E2", c1: nil, c1Null: true, c2: nil, c2Null: true},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: int64(210)},
		},
	}
	for index, batch := range batches {
		if len(batch.New) != len(wantNew[index]) || len(batch.Old) != len(wantOld[index]) {
			t.Fatalf("batch %d new/old = %d/%d, want %d/%d", index, len(batch.New), len(batch.Old), len(wantNew[index]), len(wantOld[index]))
		}
		for rowIndex, want := range wantNew[index] {
			checkRollupOutputRow(t, batch.New[rowIndex], want)
		}
		for rowIndex, want := range wantOld[index] {
			checkRollupOutputRow(t, batch.Old[rowIndex], want)
		}
	}
}
