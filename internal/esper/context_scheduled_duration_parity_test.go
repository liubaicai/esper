package esper

import (
	"context"
	"testing"
	"time"
)

// TestScheduledTimePeriodContextExcludesEndInstantParity locks the
// scheduled-start time-period context semantics verified against
// ContextStartEndAfterZeroInitiatedNow: with `start after 0 sec end after
// 60 sec` deployed at epoch, E1@0 and E2@59999 are analyzed, E3 at exactly
// 60000 is dropped (the end fires first at the boundary), and the re-armed
// start fires only at the next advance (E4@120000 is analyzed again).
func TestScheduledTimePeriodContextExcludesEndInstantParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextScheduledDurationBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateScheduledTimePeriodContext(env, "CtxPerId", 0, 60*time.Second); err != nil {
		t.Fatal(err)
	}
	source := From[contextScheduledDurationBean](env, "SupportBean")
	plan, err := env.Build(Select(source,
		Alias("c0", Field[contextScheduledDurationBean, string]("theString")),
		Alias("c1", Field[contextScheduledDurationBean, int]("intPrimitive")),
	).Query(StatementName("s0"), WithContext("CtxPerId")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	now := time.Unix(0, 0).UTC()
	if err := engine.AdvanceTime(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(theString string, intPrimitive int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextScheduledDurationBean{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
	}
	advance := func(ms int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), now.Add(time.Duration(ms)*time.Millisecond)); err != nil {
			t.Fatal(err)
		}
	}
	advance(0)
	send("E1", 1)
	if got := len(batches); got != 1 {
		t.Fatalf("after E1: batches = %d, want 1", got)
	}
	advance(59999)
	send("E2", 2)
	if got := len(batches); got != 2 {
		t.Fatalf("after E2: batches = %d, want 2", got)
	}
	advance(60000)
	send("E3", 3)
	if got := len(batches); got != 2 {
		t.Fatalf("after E3 at 60000: batches = %d, want 2 (end instant excluded)", got)
	}
	advance(120000)
	send("E4", 4)
	if got := len(batches); got != 3 {
		t.Fatalf("after E4 at 120000: batches = %d, want 3 (re-armed start fired)", got)
	}
}

// TestOrExclusiveTerminalBranchQuitsOrParity locks the OR-exclusive pattern
// semantics verified against ContextInitTermPatternIntervalZeroInitiatedNow:
// `timer:interval(0) or every timer:interval(1 min)` fires once — the
// terminal interval(0) branch completing quits the OR and kills the
// every-child — so a 60-second-terminated context accepts E1/E2 but drops
// E3 at exactly 180000 even though the every-child would have re-fired a
// partition there under keep-alive OR semantics.
func TestOrExclusiveTerminalBranchQuitsOrParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextScheduledDurationBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	source := From[contextScheduledDurationBean](env, "SupportBean")
	start := TimerInterval(source, 0).OrExclusive(TimerInterval(source, time.Minute).Every())
	end := TimerInterval(source, 60*time.Second)
	if _, err := CreateOverlappingPatternInitiatedTerminatedContext(env, "CtxPerId", start, end); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(source,
		Alias("c0", Field[contextScheduledDurationBean, string]("theString")),
		Alias("c1", Sum[int](Field[contextScheduledDurationBean, int]("intPrimitive"))),
	).Query(StatementName("s0"), WithContext("CtxPerId")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	now := time.Unix(0, 0).UTC()
	if err := engine.AdvanceTime(context.Background(), now.Add(120*time.Second)); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(theString string, intPrimitive int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextScheduledDurationBean{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
	}
	advance := func(ms int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), now.Add(time.Duration(ms)*time.Millisecond)); err != nil {
			t.Fatal(err)
		}
	}
	send("E1", 10)
	if got := len(batches); got != 1 {
		t.Fatalf("after E1: batches = %d, want 1", got)
	}
	advance(179999)
	send("E2", 20)
	if got := len(batches); got != 2 {
		t.Fatalf("after E2: batches = %d, want 2", got)
	}
	advance(180000)
	send("E3", 4)
	if got := len(batches); got != 2 {
		t.Fatalf("after E3 at 180000: batches = %d, want 2 (one-shot OR start must not re-fire)", got)
	}
}

type contextScheduledDurationBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}
