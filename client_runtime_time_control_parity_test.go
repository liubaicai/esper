package esper

import (
	"context"
	"slices"
	"testing"
	"time"
)

type clientRuntimeScheduleWant struct {
	name string
	at   time.Duration
}

func TestClientRuntimeSendTimeSpanParity(t *testing.T) {
	env, _ := newRuntimeTest(t)
	origin := time.Unix(0, 0).UTC()
	plan, err := env.Build(TimerInterval(
		From[runtimeTestTrade](env, "Trade"),
		1500*time.Millisecond,
	).Select(
		Alias("ct", CurrentTime()),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(origin))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var callbackTimes []time.Time
	var rowTimes []time.Time
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		callbackTimes = append(callbackTimes, batch.Time)
		for _, result := range batch.New {
			rowTimes = append(rowTimes, result.Get("ct").Any().(time.Time))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	assertAdvanceTimeSpan := func(target time.Duration, resolution time.Duration, want ...time.Duration) {
		t.Helper()
		beforeCallbacks := len(callbackTimes)
		beforeRows := len(rowTimes)
		var err error
		if resolution == 0 {
			err = engine.AdvanceTimeSpan(context.Background(), origin.Add(target))
		} else {
			err = engine.AdvanceTimeSpan(context.Background(), origin.Add(target), resolution)
		}
		if err != nil {
			t.Fatal(err)
		}
		gotCallbacks := offsetsFrom(origin, callbackTimes[beforeCallbacks:])
		gotRows := offsetsFrom(origin, rowTimes[beforeRows:])
		if !slices.Equal(gotCallbacks, want) {
			t.Fatalf("advanceTimeSpan(%s, %s) callback times = %v, want %v", target, resolution, gotCallbacks, want)
		}
		if !slices.Equal(gotRows, want) {
			t.Fatalf("advanceTimeSpan(%s, %s) row times = %v, want %v", target, resolution, gotRows, want)
		}
	}

	assertAdvanceTimeSpan(3500*time.Millisecond, 0, 1500*time.Millisecond, 3*time.Second)
	assertAdvanceTimeSpan(4500*time.Millisecond, 0, 4500*time.Millisecond)
	assertAdvanceTimeSpan(9*time.Second, 0, 6*time.Second, 7500*time.Millisecond, 9*time.Second)
	assertAdvanceTimeSpan(10499*time.Millisecond, 0)
	assertAdvanceTimeSpan(10499*time.Millisecond, 0)
	assertAdvanceTimeSpan(10500*time.Millisecond, 0, 10500*time.Millisecond)
	assertAdvanceTimeSpan(10500*time.Millisecond, 0)
	assertAdvanceTimeSpan(14*time.Second, 200*time.Millisecond, 12100*time.Millisecond, 13700*time.Millisecond)
	if got := engine.Now(); !got.Equal(origin.Add(14 * time.Second)) {
		t.Fatalf("current time = %s, want %s", got, origin.Add(14*time.Second))
	}
}

func TestClientRuntimeNextScheduledTimeParity(t *testing.T) {
	env, _ := newRuntimeTest(t)
	origin := time.Unix(0, 0).UTC()
	engine := NewEngine(env, WithStartTime(origin))
	source := From[runtimeTestTrade](env, "Trade")

	assertSchedules := func(want ...clientRuntimeScheduleWant) {
		t.Helper()
		schedules, err := engine.StatementNearestSchedules(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(schedules) != len(want) {
			t.Fatalf("statement schedules = %#v, want %d entries", schedules, len(want))
		}
		for index := range want {
			if schedules[index].StatementName != want[index].name || !schedules[index].At.Equal(origin.Add(want[index].at)) {
				t.Fatalf("statement schedules[%d] = %#v, want name=%q at=%s", index, schedules[index], want[index].name, origin.Add(want[index].at))
			}
			if schedules[index].DeploymentID == "" {
				t.Fatalf("statement schedules[%d] has empty deployment id", index)
			}
		}
		next, ok, err := engine.NextScheduledTime(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(want) == 0 {
			if ok || !next.IsZero() {
				t.Fatalf("next scheduled time = (%s, %t), want none", next, ok)
			}
			return
		}
		if !ok || !next.Equal(origin.Add(want[0].at)) {
			t.Fatalf("next scheduled time = (%s, %t), want %s", next, ok, origin.Add(want[0].at))
		}
	}

	assertSchedules()
	s0 := deployClientRuntimeTimer(t, engine, env, TimerAt(source, origin.Add(2*time.Second)), "s0")
	assertSchedules(clientRuntimeScheduleWant{"s0", 2 * time.Second})
	s2 := deployClientRuntimeTimer(t, engine, env, TimerAt(source, origin.Add(150*time.Millisecond)), "s2")
	assertSchedules(
		clientRuntimeScheduleWant{"s2", 150 * time.Millisecond},
		clientRuntimeScheduleWant{"s0", 2 * time.Second},
	)
	if err := s2.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertSchedules(clientRuntimeScheduleWant{"s0", 2 * time.Second})
	s3Pattern := TimerAt(source, origin.Add(3*time.Second)).And(TimerAt(source, origin.Add(4*time.Second)))
	_ = deployClientRuntimeTimer(t, engine, env, s3Pattern, "s3")
	assertSchedules(
		clientRuntimeScheduleWant{"s0", 2 * time.Second},
		clientRuntimeScheduleWant{"s3", 3 * time.Second},
	)
	if err := engine.AdvanceTime(context.Background(), origin.Add(2500*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	assertSchedules(clientRuntimeScheduleWant{"s3", 3 * time.Second})
	if err := engine.AdvanceTime(context.Background(), origin.Add(3500*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	assertSchedules(clientRuntimeScheduleWant{"s3", 4 * time.Second})
	if err := engine.AdvanceTime(context.Background(), origin.Add(4500*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	assertSchedules()
	if err := s0.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func deployClientRuntimeTimer(t *testing.T, engine *Engine, env *Environment, pattern PatternStream, name string) *Deployment {
	t.Helper()
	plan, err := env.Build(pattern.Select(Alias("ct", CurrentTime())).Query(StatementName(name)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	return deployment
}

func offsetsFrom(origin time.Time, values []time.Time) []time.Duration {
	result := make([]time.Duration, len(values))
	for index, value := range values {
		result[index] = value.Sub(origin)
	}
	return result
}
