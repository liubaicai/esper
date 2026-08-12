package esper

import (
	"context"
	"testing"
	"time"
)

type patternCronPropertyEvent struct {
	IntPrimitive int `esper:"intPrimitive"`
}

func TestPatternTimerAtScheduleUsesCapturedTagExpression(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternCronPropertyEvent](env, "CronPropertyEvent"); err != nil {
		t.Fatal(err)
	}
	base := From[patternCronPropertyEvent](env, "CronPropertyEvent")
	pattern := PatternFrom(base, "a", Literal[bool](true)).Then(
		TimerAtSchedule(base, CronSchedule{
			Minute:     CronEveryExpr(Multiply[int](Literal(2), TagField[int]("a", "intPrimitive"))),
			Hour:       CronWildcard(),
			DayOfMonth: CronWildcard(),
			Month:      CronWildcard(),
			Weekday:    CronWildcard(),
		}),
	)
	plan, err := env.Build(pattern.Select(
		Alias("value", TagField[int]("a", "intPrimitive")),
		Alias("scheduled", CurrentTime()),
	).Query(StatementName("pattern-timer-at-captured-property")))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2008, time.August, 3, 6, 0, 0, 0, time.UTC)
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
	if err := engine.SendEvent(context.Background(), patternCronPropertyEvent{IntPrimitive: 20}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), start.Add(39*time.Minute+59*time.Second+999999999*time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("captured-property timer fired early: %#v", rows)
	}
	want := start.Add(40 * time.Minute)
	if err := engine.AdvanceTime(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("captured-property timer rows = %#v, want one row", rows)
	}
	if got := rows[0].Get("value").Any(); got != 20 {
		t.Fatalf("captured-property value = %#v, want 20", got)
	}
	if got := rows[0].Get("scheduled").Any(); got != want {
		t.Fatalf("captured-property scheduled time = %#v, want %s", got, want)
	}
}

func TestPatternTimerAtScheduleRejectsUnboundEventField(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternCronPropertyEvent](env, "CronPropertyEvent"); err != nil {
		t.Fatal(err)
	}
	base := From[patternCronPropertyEvent](env, "CronPropertyEvent")
	pattern := PatternFrom(base, "a", Literal[bool](true)).Then(
		TimerAtSchedule(base, CronSchedule{
			Minute:     CronEveryExpr(Field[patternCronPropertyEvent, int]("intPrimitive")),
			Hour:       CronWildcard(),
			DayOfMonth: CronWildcard(),
			Month:      CronWildcard(),
			Weekday:    CronWildcard(),
		}),
	)
	if _, err := env.Build(pattern.Query(StatementName("pattern-timer-at-unbound-field"))); err == nil {
		t.Fatal("pattern timer accepted an unbound event field")
	}
}
