package esper

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCronScheduleNextOccurrenceAndDayOrSemantics(t *testing.T) {
	schedule := CronSchedule{
		Minute:     CronEvery(15),
		Hour:       CronRange(8, 17),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	}
	resolved, err := schedule.resolve(EvalContext{})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)
	next, err := resolved.nextAfter(start)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2008, time.February, 1, 17, 15, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("next cron occurrence = %s, want %s", next, want)
	}
	next, err = resolved.nextAfter(time.Date(2008, time.February, 1, 17, 45, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	want = time.Date(2008, time.February, 2, 8, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("next-day cron occurrence = %s, want %s", next, want)
	}

	dayOr, err := CronSchedule{
		Minute:     CronValues(0),
		Hour:       CronValues(0),
		DayOfMonth: CronValues(1, 15),
		Month:      CronWildcard(),
		Weekday:    CronValues(1), // Monday, matching Java Calendar's 1.
	}.resolve(EvalContext{})
	if err != nil {
		t.Fatal(err)
	}
	next, err = dayOr.nextAfter(time.Date(2004, time.December, 5, 9, 50, 59, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	want = time.Date(2004, time.December, 6, 0, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("day-of-month/day-of-week OR occurrence = %s, want %s", next, want)
	}
}

func TestOutputAtUsesVirtualClockAndFlushesAccumulatedRows(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	policy := OutputAt(CronSchedule{
		Minute:     CronEvery(15),
		Hour:       CronRange(8, 17),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	})
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LastEvent()).Query(
		StatementName("output-at"),
		WithOutput(policy),
	))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(plan.Canonical()); !strings.Contains(got, "at(*/15,8:17,*,*,*)->all") {
		t.Fatalf("cron canonical description = %q", got)
	}
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
	send := func(symbol string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	advance := func(at time.Time) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), at); err != nil {
			t.Fatal(err)
		}
	}
	send("S1")
	advance(time.Date(2008, time.February, 1, 17, 14, 59, 0, time.UTC))
	send("S2")
	if len(batches) != 0 {
		t.Fatalf("output-at fired before schedule = %#v", batches)
	}
	advance(time.Date(2008, time.February, 1, 17, 15, 0, 0, time.UTC))
	if len(batches) != 1 || len(batches[0].New) != 2 {
		t.Fatalf("first output-at batch = %#v", batches)
	}
	first, _ := batches[0].New[0].Event()
	second, _ := batches[0].New[1].Event()
	if first.Underlying().(runtimeTestTrade).Symbol != "S1" || second.Underlying().(runtimeTestTrade).Symbol != "S2" {
		t.Fatalf("first output-at rows = %#v", batches[0].New)
	}
	send("S3")
	advance(time.Date(2008, time.February, 1, 17, 30, 0, 0, time.UTC))
	if len(batches) != 2 || len(batches[1].New) != 1 {
		t.Fatalf("second output-at batch = %#v", batches)
	}
	third, _ := batches[1].New[0].Event()
	if third.Underlying().(runtimeTestTrade).Symbol != "S3" {
		t.Fatalf("second output-at row = %#v", batches[1].New[0])
	}
	advance(time.Date(2008, time.February, 1, 17, 45, 0, 0, time.UTC))
	send("S4")
	advance(time.Date(2008, time.February, 1, 18, 0, 0, 0, time.UTC))
	if len(batches) != 2 {
		t.Fatalf("output-at fired outside hour range = %#v", batches)
	}
	advance(time.Date(2008, time.February, 2, 8, 0, 0, 0, time.UTC))
	if len(batches) != 3 || len(batches[2].New) != 1 {
		t.Fatalf("next-day output-at batch = %#v", batches)
	}
}

func TestOutputAtSnapshotReadsCurrentWindowAtCalendarTick(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	policy := OutputAt(CronSchedule{
		Minute:     CronEvery(15),
		Hour:       CronRange(8, 17),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	}, OutputSnapshot())
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(2)).Query(
		StatementName("output-at-snapshot"),
		WithOutput(policy),
	))
	if err != nil {
		t.Fatal(err)
	}
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
	for _, symbol := range []string{"S1", "S2"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 15, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 2 {
		t.Fatalf("first snapshot-at batch = %#v", batches)
	}
	for _, symbol := range []string{"S3", "S4"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 30, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || len(batches[1].New) != 2 {
		t.Fatalf("second snapshot-at batch = %#v", batches)
	}
	first, _ := batches[1].New[0].Event()
	second, _ := batches[1].New[1].Event()
	if first.Underlying().(runtimeTestTrade).Symbol != "S3" || second.Underlying().(runtimeTestTrade).Symbol != "S4" {
		t.Fatalf("second snapshot-at rows = %#v", batches[1].New)
	}
}

func TestOutputAtSupportsInitialVariableSchedule(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("frequency", 15); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("minimum-hour", 8); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("maximum-hour", 17); err != nil {
		t.Fatal(err)
	}
	policy := OutputAt(CronSchedule{
		Minute:     CronEveryExpr(VariableRef[int]("frequency")),
		Hour:       CronRangeExpr(VariableRef[int]("minimum-hour"), VariableRef[int]("maximum-hour")),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	})
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(WithOutput(policy)))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(plan.Canonical()); !strings.Contains(got, "at(*/frequency,minimum-hour:maximum-hour,*,*,*)->all") {
		t.Fatalf("variable cron canonical description = %q", got)
	}
	engine := NewEngine(env, WithStartTime(time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)))
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
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "variable"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 15, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("variable cron output = %#v", batches)
	}
}

func TestCronScheduleRejectsInvalidFieldsAndEventReferences(t *testing.T) {
	env, _ := newRuntimeTest(t)
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(WithOutput(OutputAt(CronSchedule{
		Minute: CronEvery(0),
	})))); err == nil {
		t.Fatal("zero cron step was accepted")
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(WithOutput(OutputAt(CronSchedule{
		Minute: CronEveryExpr(Field[runtimeTestTrade, int]("price")),
	})))); err == nil {
		t.Fatal("event-field cron expression was accepted")
	}
}

func TestCronScheduleSupportsSecondAndMillisecondPrecision(t *testing.T) {
	schedule := NewCronScheduleWithMilliseconds(
		CronValues(200),
		CronValues(5),
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
	start := time.Date(2008, time.February, 1, 17, 10, 5, 199000000, time.UTC)
	next, err := resolved.nextAfter(start)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2008, time.February, 1, 17, 10, 5, 200000000, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("sub-second cron occurrence = %s, want %s", next, want)
	}
	if got := schedule.description(); got != "*,*,*,*,*,5,200" {
		t.Fatalf("precision cron description = %q", got)
	}

	secondWildcard := CronSchedule{}.WithSeconds(CronWildcard())
	resolved, err = secondWildcard.resolve(EvalContext{})
	if err != nil {
		t.Fatal(err)
	}
	next, err = resolved.nextAfter(time.Date(2008, time.February, 1, 17, 10, 0, 100000000, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	want = time.Date(2008, time.February, 1, 17, 10, 1, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("second wildcard occurrence = %s, want %s", next, want)
	}
}

func TestCronSchedulePrecisionFlowsThroughOutputAtAndTimerCron(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schedule := NewCronScheduleWithMilliseconds(
		CronValues(200), CronValues(5), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(),
	)
	policy := OutputAt(schedule)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LastEvent()).Query(
		StatementName("output-at-precision"), WithOutput(policy),
	))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var outputBatches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		outputBatches = append(outputBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "output"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 10, 5, 199000000, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if len(outputBatches) != 0 {
		t.Fatalf("precision OutputAt fired early = %#v", outputBatches)
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 10, 5, 200000000, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if len(outputBatches) != 1 || len(outputBatches[0].New) != 1 {
		t.Fatalf("precision OutputAt batches = %#v", outputBatches)
	}

	pattern := TimerCron(From[runtimeTestTrade](env, "Trade"), schedule).Select(Alias("tick", Literal("timer"))).Query(StatementName("timer-cron-precision"))
	patternPlan, err := env.Build(pattern)
	if err != nil {
		t.Fatal(err)
	}
	timerEngine := NewEngine(env, WithStartTime(start))
	patternDeployment, err := timerEngine.Deploy(context.Background(), patternPlan)
	if err != nil {
		t.Fatal(err)
	}
	var timerBatches []ResultBatch
	if _, err := patternDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		timerBatches = append(timerBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := timerEngine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 10, 5, 199000000, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if len(timerBatches) != 0 {
		t.Fatalf("precision TimerCron fired early = %#v", timerBatches)
	}
	if err := timerEngine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 10, 5, 200000000, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if len(timerBatches) != 1 || len(timerBatches[0].New) != 1 {
		t.Fatalf("precision TimerCron batches = %#v", timerBatches)
	}
}

func TestCronScheduleRejectsInvalidPrecision(t *testing.T) {
	if _, err := NewCronScheduleWithSeconds(CronValues(60), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard()).resolve(EvalContext{}); err == nil {
		t.Fatal("second 60 was accepted")
	}
	if _, err := NewCronScheduleWithMilliseconds(CronValues(1000), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard()).resolve(EvalContext{}); err == nil {
		t.Fatal("millisecond 1000 was accepted")
	}
}

func TestCronEverySecondNextAfterIsExactAndBounded(t *testing.T) {
	// The every-second cron schedule must resolve the next whole second
	// immediately. A clock advanced years before deploy used to anchor the
	// temporal origin at the Unix epoch, forcing cronWindow to step one
	// cycle per second from 1970 to the deploy instant; the deploy-time
	// origin anchoring fixed the hang, and this test pins the schedule
	// arithmetic that the context loop relies on.
	everySecond := NewCronScheduleWithSeconds(CronWildcard(), CronWildcard(), CronWildcard(),
		CronWildcard(), CronWildcard(), CronWildcard())
	resolved, err := everySecond.resolve(EvalContext{})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2002, time.May, 1, 8, 0, 0, 0, time.UTC)
	start := time.Now()
	next, err := resolved.nextAfter(at)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("nextAfter on a wildcard schedule took %s", time.Since(start))
	}
	want := at.Add(time.Second)
	if !next.Equal(want) {
		t.Fatalf("nextAfter(%s) = %s, want %s", at, next, want)
	}
	// Sub-second references advance to the next whole second boundary.
	subSecond := at.Add(500 * time.Millisecond)
	next, err = resolved.nextAfter(subSecond)
	if err != nil {
		t.Fatal(err)
	}
	if !next.Equal(want) {
		t.Fatalf("nextAfter(%s) = %s, want %s", subSecond, next, want)
	}
	// previousOrAt returns the reference itself at a whole second.
	previous, err := resolved.previousOrAt(at)
	if err != nil {
		t.Fatal(err)
	}
	if !previous.Equal(at) {
		t.Fatalf("previousOrAt(%s) = %s, want %s", at, previous, at)
	}
}
