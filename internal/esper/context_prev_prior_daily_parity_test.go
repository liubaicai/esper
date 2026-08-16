package esper

import (
	"context"
	"testing"
	"time"
)

type prevPriorDailyBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestDailyContextPrevPriorWindowFunctionsParity locks the prev/prior
// window functions inside a daily context, verified against
// ContextInitTermPrevPrior: a 9-to-5 context with a keepall window where
// prev/prevwindow/prevtail/prior read the partition-local history. The
// first day accumulates E1/E2, the window closes at 17:00, and the second
// day restarts with a fresh partition (prev/prior null for E3 again).
func TestDailyContextPrevPriorWindowFunctionsParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[prevPriorDailyBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	source := From[prevPriorDailyBean](env, "SupportBean")
	theString := Field[prevPriorDailyBean, string]("theString")
	intPrimitive := Field[prevPriorDailyBean, int]("intPrimitive")
	nine, _ := NewTimeOfDay(9, 0, 0)
	five, _ := NewTimeOfDay(17, 0, 0)
	if _, err := CreateDailyTimeContext(env, "NineToFive", nine, five); err != nil {
		t.Fatal(err)
	}
	query := source.Window(KeepAll()).Aggregate(
		Alias("col1", Prev[string](1, theString)),
		Alias("col2", PrevWindow[Event](EventValue[Event]())),
		Alias("col3", PrevTail[string](0, theString)),
		Alias("col4", Prior[string](0, theString)),
		Alias("col5", Sum[int](intPrimitive)),
	).Query(StatementName("s0"), WithContext("NineToFive"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	start := time.Date(2002, 5, 1, 8, 0, 0, 0, time.UTC)
	if err := engine.AdvanceTime(context.Background(), start); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var rows []Row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	advance := func(hour int) {
		t.Helper()
		at := time.Date(2002, 5, 1, hour, 0, 0, 0, time.UTC)
		if hour >= 24 {
			at = time.Date(2002, 5, 2, hour-24, 0, 0, 0, time.UTC)
		}
		if err := engine.AdvanceTime(context.Background(), at); err != nil {
			t.Fatal(err)
		}
	}
	send := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), prevPriorDailyBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	// Before 09:00 no partition exists: the event is not analyzed.
	send("", 0)
	if len(rows) != 0 {
		t.Fatalf("pre-start rows = %d, want 0", len(rows))
	}
	advance(9)
	send("E1", 1)
	if len(rows) != 1 {
		t.Fatalf("after E1 rows = %d, want 1", len(rows))
	}
	first := rows[0]
	if !first.Get("col1").IsNull() || first.Get("col3").Any() != "E1" || !first.Get("col4").IsNull() || first.Get("col5").Any() != 1 {
		t.Fatalf("E1 row = %#v", first.AsMap())
	}
	send("E2", 2)
	if len(rows) != 2 {
		t.Fatalf("after E2 rows = %d, want 2", len(rows))
	}
	second := rows[1]
	if second.Get("col1").Any() != "E1" || second.Get("col3").Any() != "E1" || second.Get("col4").Any() != "E1" || second.Get("col5").Any() != 3 {
		t.Fatalf("E2 row = %#v", second.AsMap())
	}
	window, ok := second.Get("col2").Any().([]Event)
	if !ok || len(window) != 2 || window[0].Get("theString").Any() != "E2" || window[1].Get("theString").Any() != "E1" {
		t.Fatalf("E2 prevwindow = %#v", second.Get("col2"))
	}
	// 17:00 closes the daily partition.
	advance(17)
	if got, err := engine.ContextPartitionCount("NineToFive"); err != nil || got != 0 {
		t.Fatalf("after close partition count = %d, err=%v", got, err)
	}
	// Next day 09:00 starts fresh: prev/prior are null again.
	advance(24 + 9)
	send("E3", 9)
	if len(rows) != 3 {
		t.Fatalf("after E3 rows = %d, want 3", len(rows))
	}
	third := rows[2]
	if !third.Get("col1").IsNull() || !third.Get("col4").IsNull() || third.Get("col3").Any() != "E3" || third.Get("col5").Any() != 9 {
		t.Fatalf("E3 row = %#v", third.AsMap())
	}
}
