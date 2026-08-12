package esper

import (
	"context"
	"testing"
)

type patternOpBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func newPatternOpEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[patternOpBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func sendPatternOpBean(t *testing.T, engine *Engine, theString string, intPrim int) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), patternOpBean{TheString: theString, IntPrimitive: intPrim}); err != nil {
		t.Fatal(err)
	}
}

// TestPatternOperatorAndSimpleMatchesEsper covers PatternOperatorAndSimple:
// a=SupportBean(intPrimitive=0) and b=SupportBean(intPrimitive=1).
// Both events must arrive; B arriving first does not match, but when A
// arrives the pattern fires with a=A and b=B (the first B already arrived).
func TestPatternOperatorAndSimpleMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[patternOpBean](env, "SupportBean")
	patternA := PatternFrom(source, "a", Equal[int](Field[patternOpBean, int]("intPrimitive"), Literal(0)))
	patternB := PatternFrom(source, "b", Equal[int](Field[patternOpBean, int]("intPrimitive"), Literal(1)))
	pattern := patternA.And(patternB)

	query := pattern.Select(
		Alias("c0", TagField[string]("a", "theString")),
		Alias("c1", TagField[string]("b", "theString")),
	).Query(StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
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

	// B arrives first - no match (A hasn't arrived yet)
	sendPatternOpBean(t, engine, "EB", 1)
	if len(batches) != 0 {
		t.Fatal("B alone should not match AND pattern")
	}

	// A arrives - now both have arrived, match fires
	sendPatternOpBean(t, engine, "EA", 0)
	if len(batches) != 1 {
		t.Fatalf("A+B should match AND pattern, got %d batches", len(batches))
	}
	row, ok := batches[0].New[0].Row()
	if !ok {
		t.Fatal("expected row")
	}
	if got := row.Get("c0").Any(); got != "EA" {
		t.Fatalf("c0=%v, want EA", got)
	}
	if got := row.Get("c1").Any(); got != "EB" {
		t.Fatalf("c1=%v, want EB", got)
	}

	// After match, sending B again then A again should not match again
	// (the pattern is not 'every', so it's consumed)
	sendPatternOpBean(t, engine, "EB", 1)
	sendPatternOpBean(t, engine, "EA", 0)
	if len(batches) != 1 {
		t.Fatalf("AND pattern should not re-match without every, got %d batches", len(batches))
	}
}

// TestPatternOperatorOrSimpleMatchesEsper covers PatternOrSimple:
// a=SupportBean(intPrimitive=0) or b=SupportBean(intPrimitive=1).
// Either event arriving matches; when B arrives, a is null and b is set.
func TestPatternOperatorOrSimpleMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[patternOpBean](env, "SupportBean")
	patternA := PatternFrom(source, "a", Equal[int](Field[patternOpBean, int]("intPrimitive"), Literal(0)))
	patternB := PatternFrom(source, "b", Equal[int](Field[patternOpBean, int]("intPrimitive"), Literal(1)))
	pattern := patternA.Or(patternB)

	query := pattern.Select(
		Alias("c0", TagField[string]("a", "theString")),
		Alias("c1", TagField[string]("b", "theString")),
	).Query(StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
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

	// B arrives first - OR matches immediately with only b set
	sendPatternOpBean(t, engine, "EB", 1)
	if len(batches) != 1 {
		t.Fatalf("B should match OR pattern, got %d batches", len(batches))
	}
	row, ok := batches[0].New[0].Row()
	if !ok {
		t.Fatal("expected row")
	}
	if got := row.Get("c0").Any(); got != nil {
		t.Fatalf("c0=%v, want nil (a not matched)", got)
	}
	if got := row.Get("c1").Any(); got != "EB" {
		t.Fatalf("c1=%v, want EB", got)
	}

	// After match, sending more events should not fire again (consumed)
	sendPatternOpBean(t, engine, "EA", 0)
	sendPatternOpBean(t, engine, "EB", 1)
	if len(batches) != 1 {
		t.Fatalf("OR pattern should not re-match, got %d batches", len(batches))
	}
}

// TestPatternOperatorAndWithEveryMatchesEsper covers
// PatternOperatorAndWithEveryAndTerminationOptimization:
// a=SupportBean_A and every b=SupportBean_B. Once A arrives, every
// subsequent B matches.
func TestPatternOperatorAndWithEveryMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// Pattern: a=SupportBean(theString='A') and every b=SupportBean(theString='B')
	// Java: once A arrives, every B fires with a=A, b=B.
	// Go: patternAndNode wraps right in patternEveryNode via patternBranchRoot.
	sourceA := From[patternOpBean](env, "SupportBean")
	patternA := PatternFrom(sourceA, "a", Equal[string](Field[patternOpBean, string]("theString"), Literal("A")))
	sourceB := From[patternOpBean](env, "SupportBean")
	patternB := PatternFrom(sourceB, "b", Equal[string](Field[patternOpBean, string]("theString"), Literal("B")))
	pattern := patternA.And(patternB.Every())

	query := pattern.Select(
		Alias("c0", TagField[string]("a", "theString")),
		Alias("c1", TagField[string]("b", "theString")),
	).Query(StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
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

	// Send A first - no match yet (need B too)
	sendPatternOpBean(t, engine, "A", 0)
	if len(batches) != 0 {
		t.Fatal("A alone should not match AND pattern")
	}

	// Send B - should match with a=A, b=B
	sendPatternOpBean(t, engine, "B", 1)
	if len(batches) != 1 {
		t.Fatalf("A+B should match AND+Every pattern, got %d batches", len(batches))
	}
	row, ok := batches[0].New[0].Row()
	if !ok {
		t.Fatal("expected row")
	}
	if got := row.Get("c0").Any(); got != "A" {
		t.Fatalf("c0=%v, want A", got)
	}
	if got := row.Get("c1").Any(); got != "B" {
		t.Fatalf("c1=%v, want B", got)
	}
}
