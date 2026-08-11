package esper

import (
	"context"
	"testing"
	"time"
)

// Parity coverage for PatternObserverTimerSchedule and
// PatternObserverTimerScheduleTimeZoneEST (see docs
// esper-go-port-implementation-plan.md): ISO-8601 and named-parameter
// timer:schedule observers - one-shot dates, periods, limited and unlimited
// recurrences, past/future anchoring, calendar periods, equivalent
// formulations, followed-by dynamic ISO and the named-parameter validation
// the fluent API can express.

// timerScheduleProbe collects schedule fires and steps the virtual clock,
// mirroring the Java suite's assertReceivedAtTime/assertSendNoMoreCallback
// helpers.
type timerScheduleProbe struct {
	t      *testing.T
	engine *Engine
	fires  *int
	last   int
}

func deployScheduleProbe(t *testing.T, env *Environment, engine *Engine, stream PatternStream, selections ...Selection) *timerScheduleProbe {
	t.Helper()
	if len(selections) == 0 {
		selections = []Selection{Alias("firedAt", CurrentTime())}
	}
	plan, err := env.Build(stream.Select(selections...).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		count += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &timerScheduleProbe{t: t, engine: engine, fires: &count}
}

func (probe *timerScheduleProbe) advance(at time.Time) {
	probe.t.Helper()
	if err := probe.engine.AdvanceTime(context.Background(), at.UTC()); err != nil {
		probe.t.Fatal(err)
	}
}

// fireAt asserts no fire happens strictly before ts and exactly one (or n)
// fires at ts.
func (probe *timerScheduleProbe) fireAt(ts time.Time, n ...int) {
	probe.t.Helper()
	want := 1
	if len(n) > 0 {
		want = n[0]
	}
	probe.advance(ts.Add(-time.Millisecond))
	if *probe.fires != probe.last {
		probe.t.Fatalf("before %v: fires = %d, want %d", ts, *probe.fires, probe.last)
	}
	probe.advance(ts)
	if *probe.fires != probe.last+want {
		probe.t.Fatalf("at %v: fires = %d, want %d", ts, *probe.fires, probe.last+want)
	}
	probe.last += want
}

// noneAt asserts advancing to ts produces no fire.
func (probe *timerScheduleProbe) noneAt(ts time.Time) {
	probe.t.Helper()
	probe.advance(ts)
	if *probe.fires != probe.last {
		probe.t.Fatalf("at %v: fires = %d, want %d", ts, *probe.fires, probe.last)
	}
}

// noMore asserts the schedule is exhausted even far in the future.
func (probe *timerScheduleProbe) noMore() {
	probe.t.Helper()
	probe.noneAt(time.Date(2999, 1, 1, 0, 0, 0, 0, time.UTC))
}

func scheduleDate(year int, month time.Month, day, hour, min, sec, ms int) time.Time {
	return time.Date(year, month, day, hour, min, sec, ms*int(time.Millisecond), time.UTC)
}

// TestPatternTimerScheduleSimpleMatchesEsper mirrors PatternTimerScheduleSimple:
// timer:schedule(period:1 day, repetitions: 3) anchored at the deployment
// clock fires three times at +1/+2/+3 days and is then exhausted.
func TestPatternTimerScheduleSimpleMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(scheduleDate(2002, 5, 1, 9, 0, 0, 0)))
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	probe := deployScheduleProbe(t, env, engine, TimerScheduleWithPeriod(base, PatternTimerScheduleSpec{
		Period:      PatternTimerPeriod{Days: 1},
		Repetitions: 3,
	}).Every())
	probe.noneAt(scheduleDate(2002, 5, 2, 8, 59, 59, 999))
	probe.fireAt(scheduleDate(2002, 5, 2, 9, 0, 0, 0))
	probe.noneAt(scheduleDate(2002, 5, 3, 8, 59, 59, 999))
	probe.fireAt(scheduleDate(2002, 5, 3, 9, 0, 0, 0))
	probe.noneAt(scheduleDate(2002, 5, 4, 8, 59, 59, 999))
	probe.fireAt(scheduleDate(2002, 5, 4, 9, 0, 0, 0))
	probe.noneAt(scheduleDate(2002, 5, 30, 9, 0, 0, 0))
}

// TestPatternTimerScheduleLimitedWDateAndPeriodMatchesEsper mirrors
// PatternTimerScheduleLimitedWDateAndPeriod: the ISO full form
// R3/2012-10-01T05:52:00Z/PT2S fires three times from its anchor.
func TestPatternTimerScheduleLimitedWDateAndPeriodMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(scheduleDate(2012, 10, 1, 5, 51, 0, 0)))
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	probe := deployScheduleProbe(t, env, engine, TimerScheduleISO(base, "R3/2012-10-01T05:52:00Z/PT2S").Every())
	probe.noneAt(scheduleDate(2012, 10, 1, 5, 51, 59, 999))
	probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 0, 0))
	probe.noneAt(scheduleDate(2012, 10, 1, 5, 52, 1, 999))
	probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 2, 0))
	probe.noneAt(scheduleDate(2012, 10, 1, 5, 52, 3, 999))
	probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 4, 0))
	probe.noneAt(scheduleDate(2012, 10, 1, 5, 53, 0, 0))
}

// TestPatternTimerScheduleJustDateMatchesEsper mirrors
// PatternTimerScheduleJustDate: timer:schedule(date: X) fires once at X.
func TestPatternTimerScheduleJustDateMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(scheduleDate(2012, 10, 1, 5, 51, 0, 0)))
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	start := scheduleDate(2012, 10, 2, 0, 0, 0, 0)
	probe := deployScheduleProbe(t, env, engine, TimerScheduleWithPeriod(base, PatternTimerScheduleSpec{
		StartAt:      &start,
		Repetitions:  1,
		IncludeStart: true,
	}).Every())
	probe.noneAt(scheduleDate(2012, 10, 1, 23, 59, 59, 999))
	probe.fireAt(scheduleDate(2012, 10, 2, 0, 0, 0, 0))
	probe.noneAt(scheduleDate(2012, 10, 10, 0, 0, 0, 0))
}

// TestPatternTimerScheduleJustPeriodMatchesEsper mirrors
// PatternTimerScheduleJustPeriod: timer:schedule(period: 1 minute) recurs
// from the deployment clock.
func TestPatternTimerScheduleJustPeriodMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(scheduleDate(2012, 10, 1, 5, 51, 0, 0)))
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	probe := deployScheduleProbe(t, env, engine, TimerScheduleWithPeriod(base, PatternTimerScheduleSpec{
		Period:      PatternTimerPeriod{FixedDuration: time.Minute},
		Repetitions: -1,
	}).Every())
	probe.noneAt(scheduleDate(2012, 10, 1, 5, 51, 59, 999))
	probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 0, 0))
	probe.fireAt(scheduleDate(2012, 10, 1, 5, 53, 0, 0))
}

// TestPatternTimerScheduleDateWithPeriodMatchesEsper mirrors
// PatternTimerScheduleDateWithPeriod: date+period anchors the recurrence at
// the date and fires one period later.
func TestPatternTimerScheduleDateWithPeriodMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(scheduleDate(2012, 10, 1, 5, 51, 0, 0)))
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	start := scheduleDate(2012, 10, 2, 0, 0, 0, 0)
	probe := deployScheduleProbe(t, env, engine, TimerScheduleWithPeriod(base, PatternTimerScheduleSpec{
		StartAt:     &start,
		Period:      PatternTimerPeriod{Days: 1},
		Repetitions: -1,
	}).Every())
	probe.noneAt(scheduleDate(2012, 10, 2, 23, 59, 59, 999))
	probe.fireAt(scheduleDate(2012, 10, 3, 0, 0, 0, 0))
	probe.fireAt(scheduleDate(2012, 10, 4, 0, 0, 0, 0))
}

// TestPatternTimerScheduleUnlimitedRecurringPeriodMatchesEsper mirrors
// PatternTimerScheduleUnlimitedRecurringPeriod: repetitions:-1 with a period
// fires every period from the deployment clock without exhaustion.
func TestPatternTimerScheduleUnlimitedRecurringPeriodMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(scheduleDate(2012, 10, 1, 5, 51, 0, 0)))
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	probe := deployScheduleProbe(t, env, engine, TimerScheduleWithPeriod(base, PatternTimerScheduleSpec{
		Period:      PatternTimerPeriod{FixedDuration: time.Second},
		Repetitions: -1,
	}).Every())
	probe.noneAt(scheduleDate(2012, 10, 1, 5, 51, 0, 999))
	probe.fireAt(scheduleDate(2012, 10, 1, 5, 51, 1, 0))
	probe.noneAt(scheduleDate(2012, 10, 1, 5, 51, 1, 999))
	probe.fireAt(scheduleDate(2012, 10, 1, 5, 51, 2, 0))
	probe.noneAt(scheduleDate(2012, 10, 1, 5, 51, 2, 999))
	probe.fireAt(scheduleDate(2012, 10, 1, 5, 51, 3, 0))
}
