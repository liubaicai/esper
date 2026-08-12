package esper

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Parity coverage for PatternObserverTimerInterval's
// PatternIntervalSpecExpressionWithProperty (see docs
// esper-go-port-implementation-plan.md): every a=SupportBean ->
// timer:interval(intPrimitive seconds) arms one dynamic timer per captured
// event, reading the interval from the event's own property.

// TestPatternObserverIntervalSpecExpressionWithPropertyMatchesEsper replays
// the Java sequence: two events at t=10000 arm 3-second and 2-second timers
// that fire at t=13000 and t=12000 respectively, each carrying its own
// captured event.
func TestPatternObserverIntervalSpecExpressionWithPropertyMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	source := From[patternOpBean](env, "SupportBean")
	pattern := PatternFrom(source, "a", Literal[bool](true)).Every().Then(
		TimerIntervalExpr(source, DurationSeconds[int](TagField[int]("a", "intPrimitive"))),
	)
	plan, err := env.Build(pattern.Select(
		Alias("id", TagField[string]("a", "theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("pattern result is not a row: %#v", result)
			}
			if value := row.Get("id"); value.State() == ValuePresent {
				ids = append(ids, value.Any().(string))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	advance := func(ms int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(ms).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	advance(10000)
	sendPatternOpBean(t, engine, "E1", 3)
	sendPatternOpBean(t, engine, "E2", 2)
	advance(11999)
	if len(ids) != 0 {
		t.Fatalf("t=11999: ids = %v, want no fires", ids)
	}
	advance(12000)
	if len(ids) != 1 || ids[0] != "E2" {
		t.Fatalf("t=12000: ids = %v, want [E2]", ids)
	}
	advance(12999)
	if len(ids) != 1 {
		t.Fatalf("t=12999: ids = %v, want [E2]", ids)
	}
	advance(13000)
	if len(ids) != 2 || ids[1] != "E1" {
		t.Fatalf("t=13000: ids = %v, want [E2 E1]", ids)
	}
}
