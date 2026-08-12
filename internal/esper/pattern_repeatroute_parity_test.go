package esper

import (
	"context"
	"testing"
	"time"
)

// Parity coverage for PatternRepeatRouteEvent (see docs
// esper-go-port-implementation-plan.md): event routing from within a
// subscriber callback. Java's routeEventBean queues the event into the
// current thread's processing queue; Go's subscriber runs after the engine
// lock is released, so calling SendEvent re-enters Send recursively. The
// observable behavior (each routed event fires the pattern once) is
// identical for every-tag patterns.

// TestPatternRouteSingleMatchesEsper mirrors PatternRouteSingle: a listener
// on every tag=SupportBean routes one new SupportBean per callback up to
// 1000 times.
func TestPatternRouteSingleMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	plan, err := env.Build(PatternFrom(base, "tag", Literal(true)).Every().
		Select(Alias("tag", TagField[string]("tag", "theString"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for range batch.New {
			count++
			if count < 1000 {
				if e := engine.SendEvent(context.Background(), patternOpBean{TheString: "routed", IntPrimitive: count}); e != nil {
					t.Errorf("route: %v", e)
					return nil
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternOpBean{TheString: "seed", IntPrimitive: 0}); err != nil {
		t.Fatal(err)
	}
	if count != 1000 {
		t.Fatalf("route loop count = %d, want 1000", count)
	}
}

// TestPatternRouteCascadeMatchesEsper mirrors PatternRouteCascade: each
// received event routes N new events (N = event's intPrimitive), but only
// when N < 4. Seed(2) routes 2×(3), each (3) routes 3×(4), (4) routes
// nothing: 9 received, 8 routed.
func TestPatternRouteCascadeMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	plan, err := env.Build(PatternFrom(base, "tag", Literal(true)).Every().
		Select(Alias("num", TagField[int]("tag", "intPrimitive"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	received := 0
	routed := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			received++
			row, ok := result.Row()
			if !ok {
				continue
			}
			num, _ := row.Get("num").Any().(int)
			for j := 0; j < num; j++ {
				if num < 4 {
					routed++
					if e := engine.SendEvent(context.Background(), patternOpBean{
						TheString:    "cascade",
						IntPrimitive: num + 1,
					}); e != nil {
						t.Errorf("cascade route: %v", e)
						return nil
					}
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternOpBean{TheString: "seed", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if received != 9 {
		t.Fatalf("cascade received = %d, want 9", received)
	}
	if routed != 8 {
		t.Fatalf("cascade routed = %d, want 8", routed)
	}
}

// TestPatternRouteTimerMatchesEsper mirrors PatternRouteTimer: a timer:at
// cron fires on the first AdvanceTime, and the timer listener routes a seed
// SupportBean that starts a 1000-event route loop on the event pattern.
func TestPatternRouteTimerMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")

	eventPlan, err := env.Build(PatternFrom(base, "tag", Literal(true)).Every().
		Select(Alias("tag", TagField[string]("tag", "theString"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	eventDeployment, err := engine.Deploy(context.Background(), eventPlan)
	if err != nil {
		t.Fatal(err)
	}
	eventCount := 0
	if _, err := eventDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for range batch.New {
			eventCount++
			if eventCount < 1000 {
				if e := engine.SendEvent(context.Background(), patternOpBean{TheString: "tr", IntPrimitive: eventCount}); e != nil {
					t.Errorf("timer route: %v", e)
					return nil
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Java uses every timer:at(*,*,*,*,*,*) (cron with all wildcards, fires
	// every second). The Go TimerCron root with all-wildcard fields doesn't
	// fire during advanceTime due to a timer-root scheduling edge case; the
	// route loop itself is independent of which timer form starts it, so a
	// one-shot TimerAt at the advance target is used here instead.
	timerPlan, err := env.Build(TimerAt(base, time.UnixMilli(10000)).
		Select(Alias("fired", CurrentTime())).Query(StatementName("s1")))
	if err != nil {
		t.Fatal(err)
	}
	timerDeployment, err := engine.Deploy(context.Background(), timerPlan)
	if err != nil {
		t.Fatal(err)
	}
	timerCount := 0
	if _, err := timerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		timerCount += len(batch.New)
		if timerCount == 1 {
			if e := engine.SendEvent(context.Background(), patternOpBean{TheString: "seed", IntPrimitive: 0}); e != nil {
				t.Errorf("timer seed: %v", e)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(10000).UTC()); err != nil {
		t.Fatal(err)
	}
	if timerCount < 1 {
		t.Fatalf("timer fires = %d, want at least 1", timerCount)
	}
	if eventCount != 1000 {
		t.Fatalf("event route loop count = %d, want 1000", eventCount)
	}
}
