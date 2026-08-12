package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestCurrentTimestampExpressionUsesVirtualClockAndStablePlan(t *testing.T) {
	env, engine := newRuntimeTest(t)
	stream := From[runtimeTestTrade](env, "Trade")
	timestamp := CurrentTimestamp()
	plan, err := env.Build(Select(stream,
		Alias("t0", timestamp),
		Alias("t1", CurrentTimestamp()),
		Alias("next", Add[int64](timestamp, Literal[int64](1))),
	).Query(StatementName("current-timestamp")))
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(Select(stream,
		Alias("t0", CurrentTimestamp()),
		Alias("t1", CurrentTimestamp()),
		Alias("next", Add[int64](CurrentTimestamp(), Literal[int64](1))),
	).Query(StatementName("current-timestamp")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent current timestamp plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	if timestamp.Type() != reflect.TypeOf(int64(0)) {
		t.Fatalf("current timestamp type = %v, want int64", timestamp.Type())
	}

	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 2)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "current timestamp result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	first := time.Unix(0, int64(100*time.Millisecond)).UTC()
	if err := engine.AdvanceTime(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	second := time.Unix(0, int64(999*time.Millisecond)).UTC()
	if err := engine.AdvanceTime(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B", Price: 2}); err != nil {
		t.Fatal(err)
	}

	if len(rows) != 2 {
		t.Fatalf("current timestamp rows = %d, want 2", len(rows))
	}
	assertCurrentTimestampRow(t, rows[0], 100)
	assertCurrentTimestampRow(t, rows[1], 999)
}

func assertCurrentTimestampRow(t *testing.T, row Row, timestamp int64) {
	t.Helper()
	for _, name := range []string{"t0", "t1"} {
		value, err := As[int64](row.Get(name))
		if err != nil || value != timestamp {
			t.Fatalf("%s = %v (%v), want %d", name, row.Get(name), err, timestamp)
		}
	}
	next, err := As[int64](row.Get("next"))
	if err != nil || next != timestamp+1 {
		t.Fatalf("next = %v (%v), want %d", row.Get("next"), err, timestamp+1)
	}
}
