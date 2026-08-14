package esper

import (
	"context"
	"testing"
	"time"
)

type contextOutputTerminationBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestContextOutputTerminationParity mirrors the shared
// context-output-termination parity scenario (Java
// ContextInitTermOutputSnapshotWhenTerminated): every-minute cron-initiated
// partitions terminate after one minute and emit an aggregate snapshot.
func TestContextOutputTerminationParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextOutputTerminationBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	base := From[contextOutputTerminationBean](env, "SupportBean")
	schedule := CronSchedule{
		Minute:     CronEvery(1),
		Hour:       CronWildcard(),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	}
	start := TimerCron(base, schedule).Every()
	end := TimerInterval(base, time.Minute)
	if _, err := CreatePatternInitiatedTerminatedContext(env, "EveryMinute", start, end); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(base.Aggregate(
		Alias("c1", Sum[int](Field[contextOutputTerminationBean, int]("intPrimitive"))),
	).Query(
		StatementName("s0"),
		WithContext("EveryMinute"),
		WithOutput(OutputSnapshotWhenTerminated()),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Date(2002, 5, 1, 8, 0, 0, 0, time.UTC)))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("termination result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	advance := func(at time.Time) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), at); err != nil {
			t.Fatal(err)
		}
	}
	send := func(id string, primitive int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextOutputTerminationBean{TheString: id, IntPrimitive: primitive}); err != nil {
			t.Fatal(err)
		}
	}
	advance(time.Date(2002, 5, 1, 8, 1, 0, 0, time.UTC))
	send("E1", 1)
	send("E2", 2)
	send("E3", 3)
	advance(time.Date(2002, 5, 1, 8, 1, 59, 999000000, time.UTC))
	advance(time.Date(2002, 5, 1, 8, 2, 0, 0, time.UTC))
	advance(time.Date(2002, 5, 1, 8, 2, 1, 0, time.UTC))
	send("E4", 4)
	send("E5", 5)
	send("E6", 6)
	advance(time.Date(2002, 5, 1, 8, 3, 0, 0, time.UTC))
	if len(rows) != 2 || rows[0].Get("c1").Any() != 6 || rows[1].Get("c1").Any() != 15 {
		t.Fatalf("termination rows = %#v, want c1 6 then 15", rows)
	}
}
