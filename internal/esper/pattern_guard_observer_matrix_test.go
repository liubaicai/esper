package esper

import (
	"context"
	"testing"
	"time"
)

// TestPatternIndependentWithinGuardsComposeWithEveryAndMatchesEsper mirrors
// the nested guard/And/Every cases in PatternGuardTimerWithin. Each branch
// owns its own deadline; the conjunction must complete only while both
// guarded filters are still alive, and Every must allow a later independent
// attempt after the first completion.
func TestPatternIndependentWithinGuardsComposeWithEveryAndMatchesEsper(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	isSymbol := func(symbol string) Expression[bool] {
		return Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal(symbol))
	}
	left := PatternFrom(base, "b", isSymbol("B")).Within(2001 * time.Millisecond)
	right := PatternFrom(base, "d", isSymbol("D")).Within(6001 * time.Millisecond)
	pattern := left.And(right).Every()
	plan, err := env.Build(pattern.Select(
		Alias("b", TagField[string]("b", "symbol")),
		Alias("d", TagField[string]("d", "symbol")),
		Alias("matchedAt", CurrentTime()),
	).Query(StatementName("pattern-independent-within-and")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("guard/and result = %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendAt := func(at int64, symbol string) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.Unix(at, 0).UTC()); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	// The first B starts the conjunction; D arrives before both guarded
	// branches expire and completes the first Every attempt.
	sendAt(1, "B")
	sendAt(2, "D")
	if len(rows) != 1 || rows[0].Get("b").Any() != "B" || rows[0].Get("d").Any() != "D" || rows[0].Get("matchedAt").Any() != time.Unix(2, 0).UTC() {
		t.Fatalf("first guarded conjunction rows = %#v", rows)
	}

	// Esper's EvalGuardStateNode disarms a within whose child already
	// quit, and EvalEveryStateNode respawns the and at the expiry
	// callback instant (restartable evaluateFalse). The first instance
	// completes at t=2; the respawn armed at t=2 dies when its left
	// within(2001ms) guard expires, and Java's scheduling service runs
	// that callback at the advance target (t=10s), so the instance
	// respawned there holds deadlines 12001/16001. B at t=10 satisfies
	// that left side; D at t=13 completes it, mirroring the S18/S29
	// retained-side pairs in PatternGuardTimerWithin.
	sendAt(10, "B")
	sendAt(13, "D")
	if len(rows) != 2 || rows[1].Get("b").Any() != "B" || rows[1].Get("d").Any() != "D" || rows[1].Get("matchedAt").Any() != time.Unix(13, 0).UTC() {
		t.Fatalf("respawned guarded conjunction rows = %#v", rows)
	}
}
