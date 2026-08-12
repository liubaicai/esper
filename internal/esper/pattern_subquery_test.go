package esper

import (
	"context"
	"testing"
)

type patternSubqueryCandidate struct {
	ID     int64  `esper:"id"`
	Symbol string `esper:"symbol"`
}

type patternSubqueryReference struct {
	ID     int64  `esper:"id"`
	Symbol string `esper:"symbol"`
}

// TestPatternSubqueryCorrelatedExistsMatchesEsper mirrors
// EPLSubselectWithinPattern.EPLSubselectCorrelated's pattern branch:
// every candidate starts a match only when the keep-all reference stream
// already contains the same symbol. The explicit OuterField keeps the
// enclosing pattern event visible in the fluent Go expression.
func TestPatternSubqueryCorrelatedExistsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternSubqueryCandidate](env, "PatternSubqueryCandidate"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternSubqueryReference](env, "PatternSubqueryReference"); err != nil {
		t.Fatal(err)
	}

	inner := Select(From[patternSubqueryReference](env, "PatternSubqueryReference")).Window(KeepAll())
	pattern := PatternFrom(
		From[patternSubqueryCandidate](env, "PatternSubqueryCandidate"),
		"candidate",
		SubqueryExists(inner, Equal[string](
			Field[patternSubqueryReference, string]("symbol"),
			OuterField[string]("symbol"),
		)),
	).Every()
	plan, err := env.Build(pattern.Select(
		Alias("id", TagField[int64]("candidate", "id")),
		Alias("symbol", TagField[string]("candidate", "symbol")),
	).Query(StatementName("pattern-subquery-correlated")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("pattern subquery result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendCandidate := func(id int64, symbol string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), patternSubqueryCandidate{ID: id, Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	sendReference := func(id int64, symbol string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), patternSubqueryReference{ID: id, Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}

	// This is the Java tryAssertionCorrelated sequence. Reference events
	// populate the inner keep-all stream even though they are not pattern
	// candidates; only candidates with an existing matching reference emit.
	sendCandidate(1, "A")
	sendReference(2, "A")
	sendCandidate(3, "B")
	sendReference(4, "C")
	if len(rows) != 0 {
		t.Fatalf("early pattern subquery rows = %#v, want none", rows)
	}

	sendCandidate(5, "C")
	sendCandidate(6, "A")
	sendCandidate(7, "D")
	sendReference(8, "E")
	sendCandidate(9, "C")

	if len(rows) != 3 {
		t.Fatalf("pattern subquery rows = %#v, want three matches", rows)
	}
	wantIDs := []int64{5, 6, 9}
	for index, wantID := range wantIDs {
		if got := rows[index].Get("id").Any(); got != wantID {
			t.Fatalf("pattern subquery row %d id = %#v, want %d", index, got, wantID)
		}
	}
}

// TestPatternSubqueryInLastEventMatchesEsper mirrors
// EPLSubselectWithinPattern.EPLSubselectFilterPatternNamedWindowNoAlias's
// event-stream pattern branch. The reference source is a LastEvent window,
// so each reference replaces the only value visible to the pattern predicate.
func TestPatternSubqueryInLastEventMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternSubqueryCandidate](env, "PatternSubqueryCandidate"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternSubqueryReference](env, "PatternSubqueryReference"); err != nil {
		t.Fatal(err)
	}

	inner := Select(From[patternSubqueryReference](env, "PatternSubqueryReference")).Window(LastEvent())
	pattern := PatternFrom(
		From[patternSubqueryCandidate](env, "PatternSubqueryCandidate"),
		"candidate",
		SubqueryIn[string](
			Field[patternSubqueryCandidate, string]("symbol"),
			inner,
			Field[patternSubqueryReference, string]("symbol"),
		),
	).Every()
	plan, err := env.Build(pattern.Select(
		Alias("id", TagField[int64]("candidate", "id")),
	).Query(StatementName("pattern-subquery-in-lastevent")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("pattern IN subquery result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendCandidate := func(id int64, symbol string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), patternSubqueryCandidate{ID: id, Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	sendReference := func(id int64, symbol string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), patternSubqueryReference{ID: id, Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}

	// This is the Java tryAssertion sequence. The only matches are the
	// candidate immediately following LastEvent(A) and the one following
	// LastEvent(E).
	sendCandidate(1, "A")
	sendReference(2, "A")
	sendCandidate(3, "B")
	sendReference(4, "C")
	sendCandidate(5, "C")
	sendCandidate(6, "A")
	sendCandidate(7, "D")
	sendReference(8, "E")
	sendCandidate(9, "C")
	sendCandidate(10, "E")

	if len(rows) != 2 {
		t.Fatalf("pattern IN subquery rows = %#v, want two matches", rows)
	}
	wantIDs := []int64{5, 10}
	for index, wantID := range wantIDs {
		if got := rows[index].Get("id").Any(); got != wantID {
			t.Fatalf("pattern IN subquery row %d id = %#v, want %d", index, got, wantID)
		}
	}
}
