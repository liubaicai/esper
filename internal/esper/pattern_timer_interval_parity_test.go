package esper

import (
	"context"
	"testing"
	"time"
)

type patternTimerIntervalBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestPatternTimerIntervalPropertyParity mirrors the shared
// pattern-timer-interval parity scenario. The Java oracle (Esper 9.0.0 commit
// 9e1b9f1cc9117fea4bf33ab043762c045d73839c) arms one dynamic timer per
// captured SupportBean using timer:interval(intPrimitive seconds): E2/2 fires
// at t=12000 and E1/3 fires at t=13000.
func TestPatternTimerIntervalPropertyParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternTimerIntervalBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	source := From[patternTimerIntervalBean](env, "SupportBean")
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
	var ids []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("pattern result is not a row: %#v", result)
			}
			ids = append(ids, row.Get("id").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	advance := func(at time.Time) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), at); err != nil {
			t.Fatal(err)
		}
	}
	send := func(id string, seconds int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), patternTimerIntervalBean{TheString: id, IntPrimitive: seconds}); err != nil {
			t.Fatal(err)
		}
	}
	advance(time.UnixMilli(10000).UTC())
	send("E1", 3)
	send("E2", 2)
	advance(time.UnixMilli(11999).UTC())
	if len(ids) != 0 {
		t.Fatalf("t=11999 ids = %v, want no fires", ids)
	}
	advance(time.UnixMilli(12000).UTC())
	if len(ids) != 1 || ids[0] != "E2" {
		t.Fatalf("t=12000 ids = %v, want [E2]", ids)
	}
	advance(time.UnixMilli(12999).UTC())
	if len(ids) != 1 {
		t.Fatalf("t=12999 ids = %v, want [E2]", ids)
	}
	advance(time.UnixMilli(13000).UTC())
	if len(ids) != 2 || ids[1] != "E1" {
		t.Fatalf("t=13000 ids = %v, want [E2 E1]", ids)
	}
}
