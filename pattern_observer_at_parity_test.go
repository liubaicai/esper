package esper

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Parity coverage for PatternObserverTimerAt (see docs
// esper-go-port-implementation-plan.md): cron-form timer:at observers over
// weekday lists, substitution parameters, variables and arithmetic field
// expressions, replaying the Java weekday sweep starting Sunday 2008-08-03
// 10:00 and expecting one fire at 08:00 on each of Mon Aug 4 .. Fri Aug 8.

// timerAtWeekdaySchedule returns the cron equivalent of
// timer:at(0,8,*,*,[1,2,3,4,5]); Esper cron weekdays number Sunday=0 through
// Saturday=6, so [1,2,3,4,5] is Monday..Friday.
func timerAtWeekdaySchedule() CronSchedule {
	return CronSchedule{
		Minute:     CronValues(0),
		Hour:       CronValues(8),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronValues(1, 2, 3, 4, 5),
	}
}

// assertTimerAtWeekdaySweep replays the Java tryAssertion loop: advance the
// virtual clock across one week and assert fires land exactly on each weekday
// at 08:00. The sweep steps in 10-minute increments, which always includes
// the exact 08:00 occurrences.
func assertTimerAtWeekdaySweep(t *testing.T, engine *Engine, fires *[]time.Time) {
	t.Helper()
	start := time.Date(2008, 8, 3, 10, 0, 0, 0, time.UTC)
	if err := engine.AdvanceTime(context.Background(), start); err != nil {
		t.Fatal(err)
	}
	for step := 1; step <= 7*24*6; step++ {
		at := start.Add(time.Duration(step) * 10 * time.Minute)
		if err := engine.AdvanceTime(context.Background(), at); err != nil {
			t.Fatal(err)
		}
	}
	want := []time.Time{
		time.Date(2008, 8, 4, 8, 0, 0, 0, time.UTC),
		time.Date(2008, 8, 5, 8, 0, 0, 0, time.UTC),
		time.Date(2008, 8, 6, 8, 0, 0, 0, time.UTC),
		time.Date(2008, 8, 7, 8, 0, 0, 0, time.UTC),
		time.Date(2008, 8, 8, 8, 0, 0, 0, time.UTC),
	}
	if fmt.Sprintf("%v", *fires) != fmt.Sprintf("%v", want) {
		t.Fatalf("weekday fires = %v, want %v", *fires, want)
	}
}

// deployTimerAtPattern deploys the cron pattern and subscribes a fire-time
// collector.
func deployTimerAtPattern(t *testing.T, env *Environment, engine *Engine, stream PatternStream) *[]time.Time {
	t.Helper()
	plan, err := env.Build(stream.Select(Alias("firedAt", CurrentTime())).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	return deployTimerAtPlan(t, engine, plan)
}

func deployTimerAtPlan(t *testing.T, engine *Engine, plan Plan) *[]time.Time {
	t.Helper()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	fires := []time.Time{}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("pattern result is not a row: %#v", result)
			}
			if value := row.Get("firedAt"); value.State() == ValuePresent {
				fires = append(fires, value.Any().(time.Time))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &fires
}

// TestPatternObserverTimerAtWeekdaysMatchesEsper mirrors PatternAtWeekdays:
// every timer:at(0,8,*,*,[1,2,3,4,5]) fires once at 08:00 on every weekday.
func TestPatternObserverTimerAtWeekdaysMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(time.Date(2008, 8, 3, 10, 0, 0, 0, time.UTC)))
	defer func() { _ = engine.Close(context.Background()) }()
	source := From[patternOpBean](env, "SupportBean")
	fires := deployTimerAtPattern(t, env, engine, TimerCron(source, timerAtWeekdaySchedule()).Every())
	assertTimerAtWeekdaySweep(t, engine, fires)
}

// TestPatternObserverTimerAtWeekdaysPreparedMatchesEsper mirrors
// PatternAtWeekdaysPrepared: cron fields bound through 1-based positional
// substitution parameters, deployed with the values 0 (minute) and 8 (hour).
func TestPatternObserverTimerAtWeekdaysPreparedMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(time.Date(2008, 8, 3, 10, 0, 0, 0, time.UTC)))
	defer func() { _ = engine.Close(context.Background()) }()
	source := From[patternOpBean](env, "SupportBean")
	schedule := CronSchedule{
		Minute:     CronValuesExpr(ParameterAt[int](1)),
		Hour:       CronValuesExpr(ParameterAt[int](2)),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronValues(1, 2, 3, 4, 5),
	}
	plan, err := env.Build(TimerCron(source, schedule).Every().Select(Alias("firedAt", CurrentTime())).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.DeployWithPositionalParameters(context.Background(), plan, 0, 8)
	if err != nil {
		t.Fatal(err)
	}
	fires := []time.Time{}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("pattern result is not a row: %#v", result)
			}
			if value := row.Get("firedAt"); value.State() == ValuePresent {
				fires = append(fires, value.Any().(time.Time))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	assertTimerAtWeekdaySweep(t, engine, &fires)
}

// TestPatternObserverTimerAtWeekdaysVariableMatchesEsper mirrors
// PatternAtWeekdaysVariable: cron fields read from registered variables VMIN
// and VHOUR.
func TestPatternObserverTimerAtWeekdaysVariableMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	if err := env.RegisterVariable("VMIN", 0); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("VHOUR", 8); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Date(2008, 8, 3, 10, 0, 0, 0, time.UTC)))
	defer func() { _ = engine.Close(context.Background()) }()
	source := From[patternOpBean](env, "SupportBean")
	schedule := CronSchedule{
		Minute:     CronValuesExpr(VariableRef[int]("VMIN")),
		Hour:       CronValuesExpr(VariableRef[int]("VHOUR")),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronValues(1, 2, 3, 4, 5),
	}
	fires := deployTimerAtPattern(t, env, engine, TimerCron(source, schedule).Every())
	assertTimerAtWeekdaySweep(t, engine, fires)
}

// TestPatternObserverTimerAtExpressionMatchesEsper mirrors PatternExpression:
// arithmetic inside cron fields (7+1-8 and 4+4 evaluate to the weekday
// schedule 0 and 8).
func TestPatternObserverTimerAtExpressionMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(time.Date(2008, 8, 3, 10, 0, 0, 0, time.UTC)))
	defer func() { _ = engine.Close(context.Background()) }()
	source := From[patternOpBean](env, "SupportBean")
	schedule := CronSchedule{
		Minute:     CronValuesExpr(Subtract[int](Add[int](Literal(7), Literal(1)), Literal(8))),
		Hour:       CronValuesExpr(Add[int](Literal(4), Literal(4))),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronValues(1, 2, 3, 4, 5),
	}
	fires := deployTimerAtPattern(t, env, engine, TimerCron(source, schedule).Every())
	assertTimerAtWeekdaySweep(t, engine, fires)
}

// TestPatternObserverTimerAtEvery15thMonthMatchesEsper mirrors
// PatternEvery15thMonth: timer:at(*,*,*,*/15,*) compiles, deploys and
// undeploys cleanly (a month step beyond the range simply never advances).
func TestPatternObserverTimerAtEvery15thMonthMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(time.Date(2008, 8, 3, 10, 0, 0, 0, time.UTC)))
	defer func() { _ = engine.Close(context.Background()) }()
	source := From[patternOpBean](env, "SupportBean")
	schedule := CronSchedule{
		Minute:     CronWildcard(),
		Hour:       CronWildcard(),
		DayOfMonth: CronWildcard(),
		Month:      CronEvery(15),
		Weekday:    CronWildcard(),
	}
	plan, err := env.Build(TimerCron(source, schedule).Every().Select(Alias("firedAt", CurrentTime())).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}
