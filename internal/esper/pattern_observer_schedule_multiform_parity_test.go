package esper

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// Multiform and timezone parity coverage for PatternObserverTimerSchedule
// and PatternObserverTimerScheduleTimeZoneEST. See
// pattern_observer_schedule_parity_test.go for the probe harness and the
// single-form executions.

// scheduleTagProbe deploys a followed-by schedule pattern that selects the
// sb tag's theString and collects every fired row value in order.
type scheduleTagProbe struct {
	t      *testing.T
	engine *Engine
	rows   []string
}

func deployScheduleTagProbe(t *testing.T, env *Environment, engine *Engine, stream PatternStream) *scheduleTagProbe {
	t.Helper()
	plan, err := env.Build(stream.Select(Alias("sb", TagField[string]("sb", "theString"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	probe := &scheduleTagProbe{t: t, engine: engine}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, event := range batch.New {
			row, ok := event.Row()
			if !ok {
				probe.rows = append(probe.rows, "<none>")
				continue
			}
			value, _ := row.Get("sb").Any().(string)
			probe.rows = append(probe.rows, value)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return probe
}

func (probe *scheduleTagProbe) advance(at time.Time) {
	probe.t.Helper()
	if err := probe.engine.AdvanceTime(context.Background(), at.UTC()); err != nil {
		probe.t.Fatal(err)
	}
}

// noneAt asserts advancing to ts fires no row.
func (probe *scheduleTagProbe) noneAt(ts time.Time) {
	probe.t.Helper()
	before := len(probe.rows)
	probe.advance(ts)
	if len(probe.rows) != before {
		probe.t.Fatalf("at %v: rows = %v, want no new rows", ts, probe.rows)
	}
}

// wantRows advances to at and asserts the accumulated rows equal want
// exactly.
func (probe *scheduleTagProbe) wantRows(at time.Time, want ...string) {
	probe.t.Helper()
	probe.advance(at)
	if len(probe.rows) != len(want) {
		probe.t.Fatalf("at %v: rows = %v, want %v", at, probe.rows, want)
	}
	for i := range want {
		if probe.rows[i] != want[i] {
			probe.t.Fatalf("at %v: rows = %v, want %v", at, probe.rows, want)
		}
	}
}

// scheduleBuild names a pattern construction for the multiform variants.
type scheduleBuild struct {
	name  string
	build func(base Stream[patternOpBean]) PatternStream
}

// multiformProbe deploys one multiform variant against a fresh engine whose
// clock starts at start.
func multiformProbe(t *testing.T, start time.Time, build func(base Stream[patternOpBean]) PatternStream) *timerScheduleProbe {
	t.Helper()
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(start))
	t.Cleanup(func() { _ = engine.Close(context.Background()) })
	base := From[patternOpBean](env, "SupportBean")
	return deployScheduleProbe(t, env, engine, build(base))
}

// TestPatternObserverTimerScheduleMultiformMatchesEsper mirrors
// PatternObserverTimerScheduleMultiform: every ISO-8601 form and every
// named-parameter form behaves identically, one-shot dates fire once,
// periods anchor at the deployment clock, recurrences skip past-due
// occurrences, equivalent formulations produce identical schedules, and
// followed-by schedules evaluate dynamic ISO expressions per match.
func TestPatternObserverTimerScheduleMultiformMatchesEsper(t *testing.T) {
	deployEarly := scheduleDate(2012, 10, 1, 5, 51, 0, 0)
	deployLate := scheduleDate(2012, 10, 1, 5, 52, 0, 0)
	deployAnchor := scheduleDate(2012, 1, 1, 0, 0, 0, 0)
	future := scheduleDate(2012, 10, 1, 5, 52, 0, 0)
	anchor1980 := scheduleDate(1980, 1, 1, 0, 0, 0, 0)

	t.Run("just-future-date", func(t *testing.T) {
		for _, variant := range []scheduleBuild{
			{"every-iso", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleISO(base, "2012-10-01T05:52:00Z").Every()
			}},
			{"bare-iso", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleISO(base, "2012-10-01T05:52:00Z")
			}},
			{"named-date", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleWithPeriod(base, PatternTimerScheduleSpec{
					StartAt:      &future,
					Repetitions:  1,
					IncludeStart: true,
				})
			}},
		} {
			t.Run(variant.name, func(t *testing.T) {
				probe := multiformProbe(t, deployEarly, variant.build)
				probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 0, 0))
				probe.noneAt(scheduleDate(2012, 10, 1, 5, 53, 0, 0))
				probe.noMore()
			})
		}
	})

	t.Run("just-past-date", func(t *testing.T) {
		for _, variant := range []scheduleBuild{
			{"every-iso", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleISO(base, "2010-10-01T05:52:00Z").Every()
			}},
			{"bare-iso", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleISO(base, "2010-10-01T05:52:00Z")
			}},
		} {
			t.Run(variant.name, func(t *testing.T) {
				probe := multiformProbe(t, deployEarly, variant.build)
				probe.noneAt(scheduleDate(2012, 10, 1, 5, 53, 0, 0))
				probe.noMore()
			})
		}
	})

	t.Run("just-period", func(t *testing.T) {
		for _, variant := range []scheduleBuild{
			{"iso", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleISO(base, "P1DT2H")
			}},
			{"named", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleWithPeriod(base, PatternTimerScheduleSpec{
					Period:      PatternTimerPeriod{Days: 1, FixedDuration: 2 * time.Hour},
					Repetitions: 1,
				})
			}},
		} {
			t.Run(variant.name, func(t *testing.T) {
				probe := multiformProbe(t, deployEarly, variant.build)
				probe.fireAt(scheduleDate(2012, 10, 2, 7, 51, 0, 0))
				probe.noneAt(scheduleDate(2012, 10, 3, 9, 51, 0, 0))
				probe.noMore()
			})
		}
	})

	t.Run("date-with-period", func(t *testing.T) {
		for _, variant := range []scheduleBuild{
			{"iso", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleISO(base, "2012-10-01T05:52:00Z/PT2S")
			}},
			{"named", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleWithPeriod(base, PatternTimerScheduleSpec{
					StartAt:     &future,
					Period:      PatternTimerPeriod{FixedDuration: 2 * time.Second},
					Repetitions: 1,
				})
			}},
		} {
			t.Run(variant.name, func(t *testing.T) {
				probe := multiformProbe(t, deployEarly, variant.build)
				probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 2, 0))
				probe.noneAt(scheduleDate(2012, 10, 1, 5, 52, 4, 0))
				probe.noMore()
			})
		}
	})

	t.Run("recurring-limited-period", func(t *testing.T) {
		for _, variant := range []scheduleBuild{
			{"iso", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleISO(base, "R3/PT2S").Every()
			}},
			{"named", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleWithPeriod(base, PatternTimerScheduleSpec{
					Period:      PatternTimerPeriod{FixedDuration: 2 * time.Second},
					Repetitions: 3,
				}).Every()
			}},
		} {
			t.Run(variant.name, func(t *testing.T) {
				probe := multiformProbe(t, deployLate, variant.build)
				probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 2, 0))
				probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 4, 0))
				probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 6, 0))
				probe.noneAt(scheduleDate(2012, 10, 1, 5, 52, 8, 0))
				probe.noMore()
			})
		}
	})

	t.Run("recurring-unlimited-period", func(t *testing.T) {
		probe := multiformProbe(t, deployLate, func(base Stream[patternOpBean]) PatternStream {
			return TimerScheduleISO(base, "R/PT1M10S").Every()
		})
		probe.fireAt(scheduleDate(2012, 10, 1, 5, 53, 10, 0))
		probe.fireAt(scheduleDate(2012, 10, 1, 5, 54, 20, 0))
		probe.fireAt(scheduleDate(2012, 10, 1, 5, 55, 30, 0))
		probe.fireAt(scheduleDate(2012, 10, 1, 5, 56, 40, 0))
	})

	t.Run("recurring-anchoring", func(t *testing.T) {
		probe := multiformProbe(t, deployAnchor, func(base Stream[patternOpBean]) PatternStream {
			return TimerScheduleISO(base, "R/PT10S").Every()
		})
		probe.fireAt(scheduleDate(2012, 1, 1, 0, 0, 10, 0))
		probe.fireAt(scheduleDate(2012, 1, 1, 0, 0, 20, 0))
		probe.fireAt(scheduleDate(2012, 1, 1, 0, 0, 30, 0))
		probe.fireAt(scheduleDate(2012, 1, 1, 0, 0, 40, 0))
		probe.fireAt(scheduleDate(2012, 1, 1, 0, 0, 50, 0))
		probe.fireAt(scheduleDate(2012, 1, 1, 0, 1, 0, 0))
	})

	t.Run("fullform-limited-future", func(t *testing.T) {
		for _, variant := range []scheduleBuild{
			{"iso", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleISO(base, "R3/2012-10-01T05:52:00Z/PT2S").Every()
			}},
			{"named", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleWithPeriod(base, PatternTimerScheduleSpec{
					StartAt:      &future,
					Period:       PatternTimerPeriod{FixedDuration: 2 * time.Second},
					Repetitions:  3,
					IncludeStart: true,
				}).Every()
			}},
		} {
			t.Run(variant.name, func(t *testing.T) {
				probe := multiformProbe(t, deployEarly, variant.build)
				probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 0, 0))
				probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 2, 0))
				probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 4, 0))
				probe.noneAt(scheduleDate(2012, 10, 1, 5, 52, 6, 0))
				probe.noMore()
			})
		}
	})

	t.Run("fullform-limited-past", func(t *testing.T) {
		probe := multiformProbe(t, deployLate, func(base Stream[patternOpBean]) PatternStream {
			return TimerScheduleISO(base, "R8/2012-10-01T05:51:00Z/PT10S").Every()
		})
		probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 10, 0))
		probe.noneAt(scheduleDate(2012, 10, 1, 5, 52, 20, 0))
		probe.noMore()
	})

	t.Run("fullform-unlimited-future", func(t *testing.T) {
		probe := multiformProbe(t, deployLate, func(base Stream[patternOpBean]) PatternStream {
			return TimerScheduleISO(base, "R/2013-01-01T02:00:05Z/P1D").Every()
		})
		probe.fireAt(scheduleDate(2013, 1, 1, 2, 0, 5, 0))
		probe.fireAt(scheduleDate(2013, 1, 2, 2, 0, 5, 0))
		probe.fireAt(scheduleDate(2013, 1, 3, 2, 0, 5, 0))
		probe.fireAt(scheduleDate(2013, 1, 4, 2, 0, 5, 0))
	})

	t.Run("fullform-unlimited-past", func(t *testing.T) {
		probe := multiformProbe(t, deployLate, func(base Stream[patternOpBean]) PatternStream {
			return TimerScheduleISO(base, "R/1980-01-01T00:00:00Z/PT1S").Every()
		})
		probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 1, 0))
		probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 2, 0))
		probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 3, 0))
	})

	t.Run("fullform-unlimited-past-anchoring", func(t *testing.T) {
		probe := multiformProbe(t, deployAnchor, func(base Stream[patternOpBean]) PatternStream {
			return TimerScheduleISO(base, "R/1980-01-01T00:00:00Z/PT10S").Every()
		})
		probe.fireAt(scheduleDate(2012, 1, 1, 0, 0, 10, 0))
		probe.fireAt(scheduleDate(2012, 1, 1, 0, 0, 20, 0))
		probe.fireAt(scheduleDate(2012, 1, 1, 0, 0, 30, 0))
		probe.fireAt(scheduleDate(2012, 1, 1, 0, 0, 40, 0))
		probe.fireAt(scheduleDate(2012, 1, 1, 0, 0, 50, 0))
		probe.fireAt(scheduleDate(2012, 1, 1, 0, 1, 0, 0))
	})

	t.Run("equivalent", func(t *testing.T) {
		for _, variant := range []scheduleBuild{
			{"recurrence", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleISO(base, "R2/2008-03-01T13:00:00Z/P1Y2M10DT2H30M").Every()
			}},
			{"or-dates", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleISO(base, "2008-03-01T13:00:00Z").
					Or(TimerScheduleISO(base, "2009-05-11T15:30:00Z")).
					Every()
			}},
			{"or-date-and-period", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleISO(base, "2008-03-01T13:00:00Z").
					Or(TimerScheduleISO(base, "2008-03-01T13:00:00Z/P1Y2M10DT2H30M")).
					Every()
			}},
		} {
			t.Run(variant.name, func(t *testing.T) {
				probe := multiformProbe(t, scheduleDate(2001, 10, 1, 5, 51, 0, 0), variant.build)
				probe.fireAt(scheduleDate(2008, 3, 1, 13, 0, 0, 0))
				probe.fireAt(scheduleDate(2009, 5, 11, 15, 30, 0, 0))
				probe.noneAt(scheduleDate(2012, 10, 1, 5, 52, 4, 0))
				probe.noMore()
			})
		}
	})

	t.Run("followed-by-iso", func(t *testing.T) {
		env := newPatternOpEnv(t)
		engine := NewEngine(env, WithStartTime(scheduleDate(2012, 10, 1, 5, 51, 7, 0)))
		defer func() { _ = engine.Close(context.Background()) }()
		base := From[patternOpBean](env, "SupportBean")
		probe := deployScheduleTagProbe(t, env, engine, PatternFrom(base, "sb", Literal(true)).Every().
			Then(TimerScheduleISO(base, "R/1980-01-01T00:00:00Z/PT15S")))

		sendPatternOpBean(t, engine, "E1", 0)
		probe.noneAt(scheduleDate(2012, 10, 1, 5, 51, 14, 999))
		probe.wantRows(scheduleDate(2012, 10, 1, 5, 51, 15, 0), "E1")

		probe.advance(scheduleDate(2012, 10, 1, 5, 51, 16, 0))
		sendPatternOpBean(t, engine, "E2", 0)
		probe.advance(scheduleDate(2012, 10, 1, 5, 51, 18, 0))
		sendPatternOpBean(t, engine, "E3", 0)
		probe.wantRows(scheduleDate(2012, 10, 1, 5, 51, 30, 0), "E1", "E2", "E3")
	})

	t.Run("followed-by-computed-iso", func(t *testing.T) {
		env := newPatternOpEnv(t)
		engine := NewEngine(env, WithStartTime(scheduleDate(2012, 10, 1, 5, 51, 7, 0)))
		defer func() { _ = engine.Close(context.Background()) }()
		base := From[patternOpBean](env, "SupportBean")
		iso := Concat(
			Literal("R/1980-01-01T00:00:00Z/PT"),
			Func1[int, string]("itoa", strconv.Itoa, TagField[int]("sb", "intPrimitive")),
			Literal("S"),
		)
		probe := deployScheduleTagProbe(t, env, engine, PatternFrom(base, "sb", Literal(true)).Every().
			Then(TimerScheduleISOExpr(base, iso)))

		sendPatternOpBean(t, engine, "E1", 5)
		probe.noneAt(scheduleDate(2012, 10, 1, 5, 51, 9, 999))
		probe.wantRows(scheduleDate(2012, 10, 1, 5, 51, 10, 0), "E1")
	})

	t.Run("named-parameters", func(t *testing.T) {
		for _, variant := range []scheduleBuild{
			{"named", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleWithPeriod(base, PatternTimerScheduleSpec{
					StartAt:      &anchor1980,
					Period:       PatternTimerPeriod{FixedDuration: time.Second},
					Repetitions:  -1,
					IncludeStart: true,
				}).Every()
			}},
			{"iso", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleISO(base, "R/1980-01-01T00:00:00Z/PT1S").Every()
			}},
		} {
			t.Run(variant.name, func(t *testing.T) {
				probe := multiformProbe(t, deployLate, variant.build)
				probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 1, 0))
				probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 2, 0))
				probe.fireAt(scheduleDate(2012, 10, 1, 5, 52, 3, 0))
			})
		}
	})

	t.Run("invalid", func(t *testing.T) {
		env := newPatternOpEnv(t)
		base := From[patternOpBean](env, "SupportBean")
		cases := []scheduleBuild{
			{"bad-iso", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleISO(base, "x")
			}},
			{"no-parameters", func(base Stream[patternOpBean]) PatternStream {
				return TimerScheduleWithPeriod(base, PatternTimerScheduleSpec{})
			}},
		}
		for _, invalid := range cases {
			if _, err := env.Build(invalid.build(base).Select(Alias("firedAt", CurrentTime())).Query(StatementName("s0"))); err == nil {
				t.Errorf("%s: expected a build error", invalid.name)
			}
		}
	})
}

// TestPatternObserverTimerScheduleTimeZoneESTMatchesEsper mirrors
// PatternObserverTimerScheduleTimeZoneEST: a computed date anchored to the
// statement timezone fires at 9am local even though the engine clock
// advances in UTC instants.
func TestPatternObserverTimerScheduleTimeZoneESTMatchesEsper(t *testing.T) {
	est := time.FixedZone("GMT-4", -4*60*60)
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(time.Date(2012, 10, 1, 8, 59, 0, 0, est)))
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	todayAtNine := Func1[int64, string]("est-today-9am", func(ms int64) string {
		local := time.UnixMilli(ms).In(est)
		return time.Date(local.Year(), local.Month(), local.Day(), 9, 0, 0, 0, est).Format(time.RFC3339)
	}, CurrentTimestamp())
	probe := deployScheduleProbe(t, env, engine, TimerScheduleISOExpr(base, todayAtNine))
	probe.noneAt(time.Date(2012, 10, 1, 8, 59, 59, 999*int(time.Millisecond), est))
	probe.fireAt(time.Date(2012, 10, 1, 9, 0, 0, 0, est))
	probe.noneAt(time.Date(2012, 10, 3, 9, 0, 0, 0, est))
	probe.noMore()
}
