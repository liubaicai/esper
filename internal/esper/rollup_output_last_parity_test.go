package esper

import (
	"context"
	"testing"
	"time"
)

type rollupOutputLastBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	LongBoxed    int64  `esper:"longBoxed"`
}

type rollupOutputLastRowWant struct {
	c0     any
	c0Null bool
	c1     any
	c1Null bool
	c2     any
	c2Null bool
}

func checkRollupOutputRow(t *testing.T, result Result, want rollupOutputLastRowWant) {
	t.Helper()
	row, ok := result.Row()
	if !ok {
		t.Fatalf("expected a row, got %#v", result)
	}
	check := func(name string, got any, wantNull bool) {
		t.Helper()
		value := row.Get(name)
		if wantNull {
			if !value.IsNull() {
				t.Fatalf("%s = %#v, want null", name, value)
			}
			return
		}
		if value.IsNull() || value.Any() != got {
			t.Fatalf("%s = %#v, want %#v", name, value, got)
		}
	}
	check("c0", want.c0, want.c0Null)
	check("c1", want.c1, want.c1Null)
	check("c2", want.c2, want.c2Null)
}

// TestRollupOutputLastParity mirrors the rollup-output-last parity scenario
// (Java ResultSetOutputLast{join=false}): an irstream rollup over a 3.5s time
// window emits output-last every second with only the changed groups, and old
// rows carry the previous emitted values (or null placeholders for groups that
// just arrived).
func TestRollupOutputLastParity(t *testing.T) {
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
		WithOutput(OutputLastEveryTime(time.Second)),
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

	if len(batches) != 3 {
		t.Fatalf("batches = %d, want 3", len(batches))
	}
	wantNew := [][]rollupOutputLastRowWant{
		{
			{c0: "E1", c1: 1, c2: int64(40)},
			{c0: "E1", c1: 2, c2: int64(20)},
			{c0: "E1", c1: nil, c1Null: true, c2: int64(60)},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: int64(60)},
		},
		{
			{c0: "E2", c1: 1, c2: int64(40)},
			{c0: "E1", c1: 2, c2: int64(70)},
			{c0: "E2", c1: nil, c1Null: true, c2: int64(40)},
			{c0: "E1", c1: nil, c1Null: true, c2: int64(110)},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: int64(150)},
		},
		{
			{c0: "E1", c1: 1, c2: int64(100)},
			{c0: "E1", c1: nil, c1Null: true, c2: int64(170)},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: int64(210)},
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
			{c0: "E2", c1: 1, c2: nil, c2Null: true},
			{c0: "E1", c1: 2, c2: int64(20)},
			{c0: "E2", c1: nil, c1Null: true, c2: nil, c2Null: true},
			{c0: "E1", c1: nil, c1Null: true, c2: int64(60)},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: int64(60)},
		},
		{
			{c0: "E1", c1: 1, c2: int64(40)},
			{c0: "E1", c1: nil, c1Null: true, c2: int64(110)},
			{c0: nil, c0Null: true, c1: nil, c1Null: true, c2: int64(150)},
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
