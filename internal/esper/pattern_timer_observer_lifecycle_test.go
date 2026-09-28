package esper

import (
	"context"
	"testing"
	"time"
)

// Timer observer lifecycle regressions: a satisfied one-shot timer branch
// must quit permanently (Esper's EvalObserverStateNode quitInternal), an
// and-state's completed-but-unquitted side must stay resident as a cached
// match, and a timer armed by an event that is already due fires inside
// that same send.
func TestPatternTimerIntervalAndCompletedStopsReplaying(t *testing.T) {
	env, _ := newRuntimeTest(t)
	type beanB struct {
		ID string `esper:"id"`
	}
	RegisterStruct[beanB](env, "B")
	b := From[beanB](env, "B")
	plan, err := env.Build(
		PatternFrom(b, "b", Literal(true)).And(TimerInterval(b, 4*time.Second)).
			Select(Alias("b", PatternEvent("b"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine(WithStartTime(time.Unix(0, 0).UTC()))
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, r := range batch.New {
			if row, ok := r.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	})
	engine.AdvanceTime(context.Background(), time.Unix(0, 0).UTC())
	engine.SendEvent(context.Background(), beanB{ID: "B1"})
	for ms := 1000; ms <= 13000; ms += 1000 {
		engine.AdvanceTime(context.Background(), time.Unix(0, 0).UTC().Add(time.Duration(ms)*time.Millisecond))
		if ms == 5000 {
			engine.SendEvent(context.Background(), beanB{ID: "B2"})
		}
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1: the and-state completed at B1+4000 must stop permanently, and B2 must not seed a fresh instance", len(rows))
	}
}

// D1 full: `b and timer(4s)` — one-shot statement must emit a single row
// at B1+4000 and then stop permanently.
func TestPatternTimerIntervalAndOneShotStopsAfterFire(t *testing.T) {
	env, _ := newRuntimeTest(t)
	type beanB struct {
		ID string `esper:"id"`
	}
	RegisterStruct[beanB](env, "B")
	b := From[beanB](env, "B")
	plan, err := env.Build(
		PatternFrom(b, "b", Literal(true)).And(TimerInterval(b, 4*time.Second)).
			Select(Alias("b", PatternEvent("b"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine(WithStartTime(time.Unix(0, 0).UTC()))
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		count += len(batch.New)
		return nil
	})
	engine.AdvanceTime(context.Background(), time.Unix(0, 0).UTC())
	engine.SendEvent(context.Background(), beanB{ID: "B1"})
	engine.AdvanceTime(context.Background(), time.Unix(0, 0).UTC().Add(4000*time.Millisecond))
	engine.AdvanceTime(context.Background(), time.Unix(0, 0).UTC().Add(8000*time.Millisecond))
	engine.AdvanceTime(context.Background(), time.Unix(0, 0).UTC().Add(9000*time.Millisecond))
	if count != 1 {
		t.Fatalf("fires=%d, want 1", count)
	}
}

// D2: `b or timer(4s)` — the one-shot timer wins the or at t=4000; the
// statement is then permanently done and B2 must not re-fire.
func TestPatternTimerIntervalOrWinnerQuitsSibling(t *testing.T) {
	env, _ := newRuntimeTest(t)
	type beanB struct {
		ID string `esper:"id"`
	}
	RegisterStruct[beanB](env, "B")
	b := From[beanB](env, "B")
	plan, err := env.Build(
		PatternFrom(b, "b", Literal(true)).Or(TimerInterval(b, 4*time.Second)).
			Select(Alias("b", PatternEvent("b"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine(WithStartTime(time.Unix(0, 0).UTC()))
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		count += len(batch.New)
		return nil
	})
	engine.AdvanceTime(context.Background(), time.Unix(0, 0).UTC())
	engine.SendEvent(context.Background(), beanB{ID: "B1"})
	engine.AdvanceTime(context.Background(), time.Unix(0, 0).UTC().Add(4000*time.Millisecond))
	engine.SendEvent(context.Background(), beanB{ID: "B2"})
	engine.AdvanceTime(context.Background(), time.Unix(0, 0).UTC().Add(9000*time.Millisecond))
	if count != 1 {
		t.Fatalf("fires=%d, want 1 (timer only)", count)
	}
}

// D3: `b -> timer:interval(0)` — the observer armed by B1's send is due at
// that same instant and fires inside the send, the way Esper evaluates due
// callbacks during event dispatch.
func TestPatternTimerIntervalZeroFiresInSameSend(t *testing.T) {
	env, _ := newRuntimeTest(t)
	type beanB struct {
		ID string `esper:"id"`
	}
	RegisterStruct[beanB](env, "B")
	b := From[beanB](env, "B")
	plan, err := env.Build(
		PatternFrom(b, "b", Literal(true)).Then(TimerInterval(b, 0)).
			Select(Alias("b", PatternEvent("b"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine(WithStartTime(time.Unix(0, 0).UTC()))
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		count += len(batch.New)
		return nil
	})
	engine.AdvanceTime(context.Background(), time.Unix(0, 0).UTC())
	engine.SendEvent(context.Background(), beanB{ID: "B1"})
	if count != 1 {
		t.Fatalf("fires=%d after send, want 1 (same-send timer)", count)
	}
}
