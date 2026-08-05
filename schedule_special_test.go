package esper

import (
	"context"
	"testing"
	"time"
)

func TestCronScheduleSupportsSpecialCalendarOperators(t *testing.T) {
	tests := []struct {
		name     string
		schedule CronSchedule
		start    time.Time
		want     time.Time
	}{
		{
			name: "last day of month",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(17), DayOfMonth: CronLastDay(),
				Month: CronWildcard(), Weekday: CronWildcard(),
			},
			start: time.Date(2013, time.August, 23, 8, 5, 0, 0, time.UTC),
			want:  time.Date(2013, time.August, 31, 17, 0, 0, 0, time.UTC),
		},
		{
			name: "last weekday of month",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(17), DayOfMonth: CronLastWeekday(),
				Month: CronWildcard(), Weekday: CronWildcard(),
			},
			start: time.Date(2013, time.August, 23, 8, 5, 0, 0, time.UTC),
			want:  time.Date(2013, time.August, 30, 17, 0, 0, 0, time.UTC),
		},
		{
			name: "last sunday",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(0), DayOfMonth: CronWildcard(),
				Month: CronWildcard(), Weekday: CronLastDayOfWeek(0),
			},
			start: time.Date(2013, time.August, 20, 8, 0, 0, 0, time.UTC),
			want:  time.Date(2013, time.August, 25, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "last saturday shorthand",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(0), DayOfMonth: CronWildcard(),
				Month: CronWildcard(), Weekday: CronLastDayOfWeek(),
			},
			start: time.Date(2013, time.August, 1, 8, 0, 0, 0, time.UTC),
			want:  time.Date(2013, time.August, 3, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "last friday weekday operator",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(0), DayOfMonth: CronWildcard(),
				Month: CronWildcard(), Weekday: CronLastWeekdayOf(5),
			},
			start: time.Date(2013, time.August, 20, 8, 0, 0, 0, time.UTC),
			want:  time.Date(2013, time.August, 30, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "nearest weekday",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(0), DayOfMonth: CronNearestWeekday(10),
				Month: CronWildcard(), Weekday: CronWildcard(),
			},
			start: time.Date(2013, time.January, 23, 8, 5, 0, 0, time.UTC),
			want:  time.Date(2013, time.February, 11, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "leap-year last day",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(0), DayOfMonth: CronLastDay(),
				Month: CronValues(2), Weekday: CronWildcard(),
			},
			start: time.Date(2007, time.January, 1, 8, 0, 0, 0, time.UTC),
			want:  time.Date(2007, time.February, 28, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := tt.schedule.resolve(EvalContext{})
			if err != nil {
				t.Fatal(err)
			}
			got, err := resolved.nextAfter(tt.start)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("special cron occurrence = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestCronScheduleSpecialOperatorsMatchEsperSequences(t *testing.T) {
	tests := []struct {
		name     string
		schedule CronSchedule
		start    time.Time
		want     []time.Time
	}{
		{
			name: "last day",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(17), DayOfMonth: CronLastDay(),
				Month: CronWildcard(), Weekday: CronWildcard(),
			},
			start: time.Date(2013, time.August, 23, 8, 5, 0, 0, time.UTC),
			want: []time.Time{
				time.Date(2013, time.August, 31, 17, 0, 0, 0, time.UTC),
				time.Date(2013, time.September, 30, 17, 0, 0, 0, time.UTC),
				time.Date(2013, time.October, 31, 17, 0, 0, 0, time.UTC),
				time.Date(2013, time.November, 30, 17, 0, 0, 0, time.UTC),
				time.Date(2013, time.December, 31, 17, 0, 0, 0, time.UTC),
				time.Date(2014, time.January, 31, 17, 0, 0, 0, time.UTC),
				time.Date(2014, time.February, 28, 17, 0, 0, 0, time.UTC),
				time.Date(2014, time.March, 31, 17, 0, 0, 0, time.UTC),
				time.Date(2014, time.April, 30, 17, 0, 0, 0, time.UTC),
				time.Date(2014, time.May, 31, 17, 0, 0, 0, time.UTC),
				time.Date(2014, time.June, 30, 17, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "last sunday",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(17), DayOfMonth: CronWildcard(),
				Month: CronWildcard(), Weekday: CronLastDayOfWeek(0),
			},
			start: time.Date(2013, time.August, 20, 8, 0, 0, 0, time.UTC),
			want: []time.Time{
				time.Date(2013, time.August, 25, 17, 0, 0, 0, time.UTC),
				time.Date(2013, time.September, 29, 17, 0, 0, 0, time.UTC),
				time.Date(2013, time.October, 27, 17, 0, 0, 0, time.UTC),
				time.Date(2013, time.November, 24, 17, 0, 0, 0, time.UTC),
				time.Date(2013, time.December, 29, 17, 0, 0, 0, time.UTC),
				time.Date(2014, time.January, 26, 17, 0, 0, 0, time.UTC),
				time.Date(2014, time.February, 23, 17, 0, 0, 0, time.UTC),
				time.Date(2014, time.March, 30, 17, 0, 0, 0, time.UTC),
				time.Date(2014, time.April, 27, 17, 0, 0, 0, time.UTC),
				time.Date(2014, time.May, 25, 17, 0, 0, 0, time.UTC),
				time.Date(2014, time.June, 29, 17, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "last friday",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(0), DayOfMonth: CronWildcard(),
				Month: CronWildcard(), Weekday: CronLastDayOfWeek(5),
			},
			start: time.Date(2013, time.August, 20, 8, 0, 0, 0, time.UTC),
			want: []time.Time{
				time.Date(2013, time.August, 30, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.September, 27, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.October, 25, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.November, 29, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.December, 27, 0, 0, 0, 0, time.UTC),
				time.Date(2014, time.January, 31, 0, 0, 0, 0, time.UTC),
				time.Date(2014, time.February, 28, 0, 0, 0, 0, time.UTC),
				time.Date(2014, time.March, 28, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "saturday shorthand",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(0), DayOfMonth: CronWildcard(),
				Month: CronWildcard(), Weekday: CronLastDayOfWeek(),
			},
			start: time.Date(2013, time.August, 1, 8, 0, 0, 0, time.UTC),
			want: []time.Time{
				time.Date(2013, time.August, 3, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.August, 10, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.August, 17, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.August, 24, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.August, 31, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.September, 7, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "last day of august",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(0), DayOfMonth: CronLastDay(),
				Month: CronValues(8), Weekday: CronWildcard(),
			},
			start: time.Date(2013, time.January, 1, 8, 0, 0, 0, time.UTC),
			want: []time.Time{
				time.Date(2013, time.August, 31, 0, 0, 0, 0, time.UTC),
				time.Date(2014, time.August, 31, 0, 0, 0, 0, time.UTC),
				time.Date(2015, time.August, 31, 0, 0, 0, 0, time.UTC),
				time.Date(2016, time.August, 31, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "last friday of june",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(0), DayOfMonth: CronWildcard(),
				Month: CronValues(6), Weekday: CronLastDayOfWeek(5),
			},
			start: time.Date(2007, time.January, 1, 8, 0, 0, 0, time.UTC),
			want: []time.Time{
				time.Date(2007, time.June, 29, 0, 0, 0, 0, time.UTC),
				time.Date(2008, time.June, 27, 0, 0, 0, 0, time.UTC),
				time.Date(2009, time.June, 26, 0, 0, 0, 0, time.UTC),
				time.Date(2010, time.June, 25, 0, 0, 0, 0, time.UTC),
				time.Date(2011, time.June, 24, 0, 0, 0, 0, time.UTC),
				time.Date(2012, time.June, 29, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.June, 28, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "last weekday",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(0), DayOfMonth: CronLastWeekday(),
				Month: CronWildcard(), Weekday: CronWildcard(),
			},
			start: time.Date(2013, time.August, 23, 8, 5, 0, 0, time.UTC),
			want: []time.Time{
				time.Date(2013, time.August, 30, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.September, 30, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.October, 31, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.November, 29, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.December, 31, 0, 0, 0, 0, time.UTC),
				time.Date(2014, time.January, 31, 0, 0, 0, 0, time.UTC),
				time.Date(2014, time.February, 28, 0, 0, 0, 0, time.UTC),
				time.Date(2014, time.March, 31, 0, 0, 0, 0, time.UTC),
				time.Date(2014, time.April, 30, 0, 0, 0, 0, time.UTC),
				time.Date(2014, time.May, 30, 0, 0, 0, 0, time.UTC),
				time.Date(2014, time.June, 30, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "last weekday in february",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(0), DayOfMonth: CronLastWeekday(),
				Month: CronValues(2), Weekday: CronWildcard(),
			},
			start: time.Date(2007, time.January, 23, 8, 5, 0, 0, time.UTC),
			want: []time.Time{
				time.Date(2007, time.February, 28, 0, 0, 0, 0, time.UTC),
				time.Date(2008, time.February, 29, 0, 0, 0, 0, time.UTC),
				time.Date(2009, time.February, 27, 0, 0, 0, 0, time.UTC),
				time.Date(2010, time.February, 26, 0, 0, 0, 0, time.UTC),
				time.Date(2011, time.February, 28, 0, 0, 0, 0, time.UTC),
				time.Date(2012, time.February, 29, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "last weekday of september",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(0), DayOfMonth: CronLastWeekday(),
				Month: CronValues(9), Weekday: CronWildcard(),
			},
			start: time.Date(2007, time.August, 23, 8, 5, 0, 0, time.UTC),
			want: []time.Time{
				time.Date(2007, time.September, 28, 0, 0, 0, 0, time.UTC),
				time.Date(2008, time.September, 30, 0, 0, 0, 0, time.UTC),
				time.Date(2009, time.September, 30, 0, 0, 0, 0, time.UTC),
				time.Date(2010, time.September, 30, 0, 0, 0, 0, time.UTC),
				time.Date(2011, time.September, 30, 0, 0, 0, 0, time.UTC),
				time.Date(2012, time.September, 28, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "nearest weekday",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(0), DayOfMonth: CronNearestWeekday(10),
				Month: CronWildcard(), Weekday: CronWildcard(),
			},
			start: time.Date(2013, time.January, 23, 8, 5, 0, 0, time.UTC),
			want: []time.Time{
				time.Date(2013, time.February, 11, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.March, 11, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.April, 10, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.May, 10, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.June, 10, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.July, 10, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.August, 9, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "nearest weekday 30 in september",
			schedule: CronSchedule{
				Minute: CronValues(0), Hour: CronValues(0), DayOfMonth: CronNearestWeekday(30),
				Month: CronValues(9), Weekday: CronWildcard(),
			},
			start: time.Date(2007, time.January, 23, 8, 5, 0, 0, time.UTC),
			want: []time.Time{
				time.Date(2007, time.September, 28, 0, 0, 0, 0, time.UTC),
				time.Date(2008, time.September, 30, 0, 0, 0, 0, time.UTC),
				time.Date(2009, time.September, 30, 0, 0, 0, 0, time.UTC),
				time.Date(2010, time.September, 30, 0, 0, 0, 0, time.UTC),
				time.Date(2011, time.September, 30, 0, 0, 0, 0, time.UTC),
				time.Date(2012, time.September, 28, 0, 0, 0, 0, time.UTC),
				time.Date(2013, time.September, 30, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := tt.schedule.resolve(EvalContext{})
			if err != nil {
				t.Fatal(err)
			}
			at := tt.start
			for index, want := range tt.want {
				got, err := resolved.nextAfter(at)
				if err != nil {
					t.Fatalf("occurrence %d: %v", index, err)
				}
				if !got.Equal(want) {
					t.Fatalf("occurrence %d = %s, want %s", index, got, want)
				}
				at = got
			}
		})
	}
}

func TestCronScheduleSpecialOperatorsFlowThroughTimerCron(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2013, time.August, 23, 8, 5, 0, 0, time.UTC)
	schedule := CronSchedule{
		Minute: CronValues(0), Hour: CronValues(0), DayOfMonth: CronLastDay(),
		Month: CronWildcard(), Weekday: CronWildcard(),
	}
	plan, err := env.Build(TimerCron(From[runtimeTestTrade](env, "Trade"), schedule).Select(
		Alias("scheduled", CurrentTime()),
	).Query(StatementName("timer-cron-special")))
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
	want := time.Date(2013, time.August, 31, 0, 0, 0, 0, time.UTC)
	if err := engine.AdvanceTime(context.Background(), want.Add(-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("special TimerCron fired early: %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("scheduled").Any() != want {
		t.Fatalf("special TimerCron rows = %#v, want %s", rows, want)
	}
}

func TestCronScheduleSpecialOperatorsFlowThroughOutputAt(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2013, time.August, 23, 8, 5, 0, 0, time.UTC)
	schedule := CronSchedule{
		Minute: CronValues(0), Hour: CronValues(0), DayOfMonth: CronLastDay(),
		Month: CronWildcard(), Weekday: CronWildcard(),
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LastEvent()).Query(
		StatementName("output-at-special"), WithOutput(OutputAt(schedule)),
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
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "special"}); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2013, time.August, 31, 0, 0, 0, 0, time.UTC)
	if err := engine.AdvanceTime(context.Background(), want.Add(-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatalf("special OutputAt fired early: %#v", batches)
	}
	if err := engine.AdvanceTime(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("special OutputAt batches = %#v", batches)
	}
}

func TestCronScheduleUsesConfiguredTimeZoneAndDST(t *testing.T) {
	startZone := time.FixedZone("EST", -5*60*60)
	start := time.Date(2008, time.January, 4, 6, 50, 0, 0, startZone)
	schedule := CronSchedule{
		Minute: CronValues(0), Hour: CronValues(5), DayOfMonth: CronValues(4),
		Month: CronValues(1), Weekday: CronWildcard(), Second: CronValues(0),
	}.WithSeconds(CronValues(0)).InTimeZone("PST")
	resolved, err := schedule.resolve(EvalContext{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolved.nextAfter(start)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2008, time.January, 4, 8, 0, 0, 0, startZone)
	if !got.Equal(want) {
		t.Fatalf("configured timezone occurrence = %s, want %s", got, want)
	}

	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	daily := CronSchedule{
		Minute: CronValues(0), Hour: CronValues(9), DayOfMonth: CronWildcard(),
		Month: CronWildcard(), Weekday: CronWildcard(),
	}.InTimeZone("America/New_York")
	dailyResolved, err := daily.resolve(EvalContext{})
	if err != nil {
		t.Fatal(err)
	}
	beforeDST := time.Date(2024, time.March, 9, 9, 0, 0, 0, location)
	got, err = dailyResolved.nextAfter(beforeDST)
	if err != nil {
		t.Fatal(err)
	}
	want = time.Date(2024, time.March, 10, 9, 0, 0, 0, location)
	if !got.Equal(want) || got.Format("-0700") != "-0400" {
		t.Fatalf("DST timezone occurrence = %s, want %s", got, want)
	}
}

func TestCronScheduleConfiguredTimeZoneFlowsThroughTimerCron(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	startZone := time.FixedZone("EST", -5*60*60)
	start := time.Date(2008, time.January, 4, 6, 50, 0, 0, startZone)
	schedule := CronSchedule{
		Minute: CronValues(0), Hour: CronValues(5), DayOfMonth: CronValues(4),
		Month: CronValues(1), Weekday: CronWildcard(),
	}.InTimeZone("PST")
	plan, err := env.Build(TimerCron(From[runtimeTestTrade](env, "Trade"), schedule).Select(
		Alias("scheduled", CurrentTime()),
	).Query(StatementName("timer-cron-timezone")))
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
	want := time.Date(2008, time.January, 4, 8, 0, 0, 0, startZone)
	if err := engine.AdvanceTime(context.Background(), want.Add(-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("timezone TimerCron fired early: %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("timezone TimerCron rows = %#v, want %s", rows, want)
	}
	got, ok := rows[0].Get("scheduled").Any().(time.Time)
	if !ok || !got.Equal(want) {
		t.Fatalf("timezone TimerCron time = %#v, want %s", rows[0].Get("scheduled").Any(), want)
	}
}

func TestCronScheduleRejectsInvalidSpecialCombinations(t *testing.T) {
	invalid := []CronSchedule{
		{DayOfMonth: CronWildcard(), Weekday: CronLastDay(), Month: CronWildcard(), Hour: CronWildcard(), Minute: CronWildcard()},
		{DayOfMonth: CronWildcard(), Weekday: CronLastDayOfWeek(1, 2), Month: CronWildcard(), Hour: CronWildcard(), Minute: CronWildcard()},
		{DayOfMonth: CronNearestWeekday(32), Weekday: CronWildcard(), Month: CronWildcard(), Hour: CronWildcard(), Minute: CronWildcard()},
		{DayOfMonth: CronLastDay(), Weekday: CronValues(1), Month: CronWildcard(), Hour: CronWildcard(), Minute: CronWildcard()},
	}
	for index, schedule := range invalid {
		if _, err := schedule.resolve(EvalContext{}); err == nil {
			t.Fatalf("invalid special schedule %d was accepted", index)
		}
	}
	if _, err := (CronSchedule{TimeZone: "Not/AZone"}).resolve(EvalContext{}); err == nil {
		t.Fatal("invalid timezone was accepted")
	}
}
