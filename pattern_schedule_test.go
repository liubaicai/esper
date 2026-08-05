package esper

import (
	"context"
	"testing"
	"time"
)

type patternScheduleEvent struct {
	ISO string `esper:"iso"`
}

func TestPatternTimerScheduleWithPeriodFiniteAndCalendarAnchor(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := time.Date(2002, time.May, 1, 9, 0, 0, 0, time.UTC)
	plan, err := env.Build(TimerScheduleWithPeriod(base, PatternTimerScheduleSpec{
		Period:      PatternTimerPeriod{Days: 1},
		Repetitions: 3,
	}).Select(Alias("scheduled", CurrentTime())).Query(StatementName("pattern-timer-schedule-period")))
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
	first := start.AddDate(0, 0, 1)
	if err := engine.AdvanceTime(context.Background(), first.Add(-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("period schedule fired before first occurrence: %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), start.AddDate(0, 0, 3)); err != nil {
		t.Fatal(err)
	}
	want := []time.Time{first, start.AddDate(0, 0, 2), start.AddDate(0, 0, 3)}
	if len(rows) != len(want) {
		t.Fatalf("period schedule rows = %#v, want %d rows", rows, len(want))
	}
	for index, expected := range want {
		if got := rows[index].Get("scheduled").Any(); got != expected {
			t.Fatalf("period schedule row %d = %#v, want %s", index, got, expected)
		}
	}
	if err := engine.AdvanceTime(context.Background(), start.AddDate(0, 0, 10)); err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(want) {
		t.Fatalf("finite period schedule fired after repetition limit: %#v", rows)
	}
	if _, err := env.Build(TimerScheduleWithPeriod(base, PatternTimerScheduleSpec{
		Period: PatternTimerPeriod{},
	}).Select(Alias("scheduled", CurrentTime())).Query(StatementName("invalid-empty-period-schedule"))); err == nil {
		t.Fatal("empty period schedule was accepted")
	}
}

func TestPatternTimerScheduleISOForms(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := time.Date(2012, time.October, 1, 5, 51, 0, 0, time.UTC)
	plan, err := env.Build(TimerScheduleISO(base, "R3/2012-10-01T05:52:00Z/PT2S").Select(
		Alias("scheduled", CurrentTime()),
	).Query(StatementName("pattern-timer-schedule-iso")))
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
	if err := engine.AdvanceTime(context.Background(), time.Date(2012, time.October, 1, 5, 52, 5, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	want := []time.Time{
		time.Date(2012, time.October, 1, 5, 52, 0, 0, time.UTC),
		time.Date(2012, time.October, 1, 5, 52, 2, 0, time.UTC),
		time.Date(2012, time.October, 1, 5, 52, 4, 0, time.UTC),
	}
	if len(rows) != len(want) {
		t.Fatalf("ISO schedule rows = %#v, want %d rows", rows, len(want))
	}
	for index, expected := range want {
		if got := rows[index].Get("scheduled").Any(); got != expected {
			t.Fatalf("ISO schedule row %d = %#v, want %s", index, got, expected)
		}
	}
	if _, err := env.Build(TimerScheduleISO(base, "P").Select(Alias("scheduled", CurrentTime())).Query(StatementName("invalid-iso-schedule"))); err == nil {
		t.Fatal("invalid ISO schedule was accepted")
	}
}

func TestPatternTimerScheduleISOExpressionUsesCapturedTag(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternScheduleEvent](env, "ScheduleEvent"); err != nil {
		t.Fatal(err)
	}
	base := From[patternScheduleEvent](env, "ScheduleEvent")
	pattern := PatternFrom(base, "event", Literal[bool](true)).Then(
		TimerScheduleISOExpr(base, TagField[string]("event", "iso")),
	).Every()
	plan, err := env.Build(pattern.Select(
		Alias("iso", TagField[string]("event", "iso")),
		Alias("scheduled", CurrentTime()),
	).Query(StatementName("pattern-timer-schedule-iso-expression")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
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
	iso := "R/1970-01-01T00:00:00Z/PT2S"
	if err := engine.SendEvent(context.Background(), patternScheduleEvent{ISO: iso}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 999999999).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("dynamic ISO schedule fired early: %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("iso").Any() != iso || rows[0].Get("scheduled").Any() != time.Unix(2, 0).UTC() {
		t.Fatalf("dynamic ISO schedule rows = %#v", rows)
	}
}

func TestPatternTimerSchedulePeriodContextCreatesRecurringPartitions(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	if _, err := CreateOverlappingPatternInitiatedContext(env, "pattern-schedule-period-context", TimerScheduleWithPeriod(base, PatternTimerScheduleSpec{
		Period:      PatternTimerPeriod{FixedDuration: time.Second},
		Repetitions: -1,
	})); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "Trade").Select(
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
	).Query(StatementName("pattern-schedule-period-context-statement"), WithContext("pattern-schedule-period-context")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(3, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-schedule-period-context"); err != nil || count != 3 {
		t.Fatalf("period schedule context count = %d, err=%v, want 3", count, err)
	}
}

func TestPatternTimerScheduleTypedAnchorAndIncludeStart(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	start := time.Unix(1, 500000000).UTC()
	plan, err := env.Build(TimerScheduleWithPeriod(base, PatternTimerScheduleSpec{
		StartAt:      &start,
		Period:       PatternTimerPeriod{FixedDuration: 2 * time.Second},
		Repetitions:  3,
		IncludeStart: true,
	}).Select(Alias("scheduled", CurrentTime())).Query(StatementName("pattern-timer-schedule-typed-anchor")))
	if err != nil {
		t.Fatal(err)
	}
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
	if err := engine.AdvanceTime(context.Background(), start.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	want := []time.Time{start, start.Add(2 * time.Second), start.Add(4 * time.Second)}
	if len(rows) != len(want) {
		t.Fatalf("typed anchored schedule rows = %#v, want %d rows", rows, len(want))
	}
	for index, expected := range want {
		if got := rows[index].Get("scheduled").Any(); got != expected {
			t.Fatalf("typed anchored schedule row %d = %#v, want %s", index, got, expected)
		}
	}

	env2, _ := newRuntimeTest(t)
	base2 := From[runtimeTestTrade](env2, "Trade")
	engine2 := NewEngine(env2, WithStartTime(time.Unix(0, 0).UTC()))
	date := time.Unix(1, 0).UTC()
	plan2, err := env2.Build(TimerScheduleWithPeriod(base2, PatternTimerScheduleSpec{
		StartAt:     &date,
		Period:      PatternTimerPeriod{FixedDuration: time.Second},
		Repetitions: 1,
	}).Select(Alias("scheduled", CurrentTime())).Query(StatementName("pattern-timer-schedule-date-period")))
	if err != nil {
		t.Fatal(err)
	}
	deployment2, err := engine2.Deploy(context.Background(), plan2)
	if err != nil {
		t.Fatal(err)
	}
	var dateRows []Row
	if _, err := deployment2.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				dateRows = append(dateRows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine2.AdvanceTime(context.Background(), date.Add(500*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if len(dateRows) != 0 {
		t.Fatalf("typed date/period schedule fired at anchor: %#v", dateRows)
	}
	if err := engine2.AdvanceTime(context.Background(), date.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(dateRows) != 1 || dateRows[0].Get("scheduled").Any() != date.Add(time.Second) {
		t.Fatalf("typed date/period schedule rows = %#v", dateRows)
	}
}

func TestPatternTimerScheduleISO8601ParserMatchesEsperForms(t *testing.T) {
	date := time.Date(2012, time.October, 1, 5, 52, 0, 0, time.UTC)
	cases := []struct {
		name         string
		text         string
		repetitions  int64
		start        time.Time
		hasStart     bool
		period       PatternTimerPeriod
		hasPeriod    bool
		includeStart bool
	}{
		{name: "full", text: "R3/2012-10-01T05:52:00Z/PT2S", repetitions: 3, start: date, hasStart: true, period: PatternTimerPeriod{FixedDuration: 2 * time.Second}, hasPeriod: true, includeStart: true},
		{name: "date", text: "2012-10-01T05:52:00Z", repetitions: 1, start: date, hasStart: true, includeStart: true},
		{name: "timezone-fraction", text: "1997-07-16T19:20:30.12+01:00", repetitions: 1, start: time.Date(1997, time.July, 16, 19, 20, 30, 120000000, time.FixedZone("+01:00", 3600)), hasStart: true, includeStart: true},
		{name: "recurring-period", text: "R3/PT2S", repetitions: 3, period: PatternTimerPeriod{FixedDuration: 2 * time.Second}, hasPeriod: true},
		{name: "date-period", text: "2012-10-01T05:52:00Z/P1Y2M10DT2H30M", repetitions: 1, start: date, hasStart: true, period: PatternTimerPeriod{Years: 1, Months: 2, Days: 10, FixedDuration: 2*time.Hour + 30*time.Minute}, hasPeriod: true},
		{name: "period", text: "P1Y2M10DT2H30M", repetitions: 1, period: PatternTimerPeriod{Years: 1, Months: 2, Days: 10, FixedDuration: 2*time.Hour + 30*time.Minute}, hasPeriod: true},
		{name: "weeks", text: "R/2012-10-01T05:52:00Z/P6W", repetitions: -1, start: date, hasStart: true, period: PatternTimerPeriod{Days: 42}, hasPeriod: true, includeStart: true},
		{name: "unlimited-past", text: "R/1980-01-01T00:00:00Z/PT10S", repetitions: -1, start: time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC), hasStart: true, period: PatternTimerPeriod{FixedDuration: 10 * time.Second}, hasPeriod: true, includeStart: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			spec, err := parsePatternTimerScheduleISO(testCase.text)
			if err != nil {
				t.Fatal(err)
			}
			if spec.repetitions != testCase.repetitions || spec.hasStart != testCase.hasStart || spec.hasPeriod != testCase.hasPeriod || spec.includeStart != testCase.includeStart {
				t.Fatalf("parsed metadata = %#v", spec)
			}
			if testCase.hasStart && !spec.start.Equal(testCase.start) {
				t.Fatalf("parsed start = %s, want %s", spec.start, testCase.start)
			}
			if spec.period != testCase.period {
				t.Fatalf("parsed period = %#v, want %#v", spec.period, testCase.period)
			}
		})
	}
}

func TestPatternTimerScheduleISO8601ParserRejectsEsperInvalidForms(t *testing.T) {
	invalid := []string{
		"", "/", "5", "P", "P1", "P1D1D", "PT1D", "PD", "P0.1D", "P-10D", "P0D",
		"1997-07-16T19:20:30.12Z/x", "1997-07-16T19:20:30.12Z/PT1D", "dum-07-16T19:20:30.12Z/P1D",
		"/P1D", "1997-07-16T19:20:30.12Z/", "Ra/P1D", "R0.1/P1D",
		"R100000000000000000000000000000/P1D", "R/dummy/PT1M", "Rx/1997-07-16T19:20:30.12Z/PT1M",
		"R1/1997-07-16T19:20:30.12Z/PT1D",
	}
	for _, text := range invalid {
		t.Run(text, func(t *testing.T) {
			if _, err := parsePatternTimerScheduleISO(text); err == nil {
				t.Fatalf("invalid ISO schedule %q was accepted", text)
			}
		})
	}
}
