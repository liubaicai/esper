package esper

import (
	"context"
	"testing"
	"time"
)

func TestCronScheduleSupportsMicrosecondPrecision(t *testing.T) {
	schedule := NewCronScheduleWithMicroseconds(
		CronValues(200),
		CronWildcard(),
		CronWildcard(),
		CronWildcard(),
		CronWildcard(),
		CronWildcard(),
		CronWildcard(),
		CronWildcard(),
	)
	resolved, err := schedule.resolve(EvalContext{})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2013, time.August, 23, 8, 5, 0, 0, time.UTC)
	next, err := resolved.nextAfter(start)
	if err != nil {
		t.Fatal(err)
	}
	want := start.Add(200 * time.Microsecond)
	if !next.Equal(want) {
		t.Fatalf("microsecond cron occurrence = %s, want %s", next, want)
	}
	next, err = resolved.nextAfter(next)
	if err != nil {
		t.Fatal(err)
	}
	want = start.Add(1200 * time.Microsecond)
	if !next.Equal(want) {
		t.Fatalf("next microsecond cron occurrence = %s, want %s", next, want)
	}
	if got := schedule.description(); got != "*,*,*,*,*,*,*,200" {
		t.Fatalf("microsecond cron description = %q", got)
	}
	if _, err := NewCronScheduleWithMicroseconds(
		CronValues(1000), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(),
	).resolve(EvalContext{}); err == nil {
		t.Fatal("microsecond value 1000 was accepted")
	}
}

func TestCronScheduleMicrosecondPrecisionFlowsThroughTimerCron(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2013, time.August, 23, 8, 5, 0, 0, time.UTC)
	schedule := NewCronScheduleWithMicroseconds(
		CronValues(200), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(),
	)
	plan, err := env.Build(TimerCron(From[runtimeTestTrade](env, "Trade"), schedule).Select(
		Alias("scheduled", CurrentTime()),
	).Query(StatementName("timer-cron-microsecond")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(start))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	first := start.Add(200 * time.Microsecond)
	second := start.Add(1200 * time.Microsecond)
	if err := engine.AdvanceTime(context.Background(), first.Add(-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("microsecond TimerCron fired early: %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Get("scheduled").Any() != first || rows[1].Get("scheduled").Any() != second {
		t.Fatalf("microsecond TimerCron rows = %#v, want %s and %s", rows, first, second)
	}
}

func TestCronScheduleMicrosecondPrecisionFlowsThroughOutputAt(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2013, time.August, 23, 8, 5, 0, 0, time.UTC)
	schedule := NewCronScheduleWithMicroseconds(
		CronValues(200), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(),
	)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LastEvent()).Query(
		StatementName("output-at-microsecond"), WithOutput(OutputAt(schedule)),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(start))
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
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "micro"}); err != nil {
		t.Fatal(err)
	}
	first := start.Add(200 * time.Microsecond)
	if err := engine.AdvanceTime(context.Background(), first.Add(-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatalf("microsecond OutputAt fired early: %#v", batches)
	}
	if err := engine.AdvanceTime(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("microsecond OutputAt batches = %#v", batches)
	}
}
