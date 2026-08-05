package esper

import (
	"errors"
	"testing"
)

// TestPatternSubqueryInvalidDefinitionsMatchesEsper mirrors
// EPLSubselectWithinPattern.EPLSubselectInvalid. The Go fluent API reports
// structural/type errors at Build time instead of reproducing EPL parser
// text, while keeping the same three invalid-rule boundaries.
func TestPatternSubqueryInvalidDefinitionsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternSubqueryCandidate](env, "PatternSubqueryInvalidCandidate"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternSubqueryReference](env, "PatternSubqueryInvalidReference"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("PatternSubqueryInvalidReference")
	if !ok {
		t.Fatal("invalid subquery reference schema is missing")
	}
	if _, err := CreateNamedWindow(env, "PatternSubqueryInvalidWindow", schema, NamedWindowRetention(LastEvent())); err != nil {
		t.Fatal(err)
	}

	assertInvalid := func(name string, query Query, code ErrorCode) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			_, err := env.Build(query)
			if err == nil {
				t.Fatal("invalid pattern subquery definition was accepted")
			}
			if !errors.Is(err, code) {
				t.Fatalf("invalid pattern subquery error = %v, want %s", err, code)
			}
		})
	}

	assertInvalid("unbounded-event-stream", From[patternSubqueryCandidate](env, "PatternSubqueryInvalidCandidate").Filter(
		SubqueryExists(
			Select(From[patternSubqueryReference](env, "PatternSubqueryInvalidReference")),
			Literal(true),
		),
	).Query(StatementName("invalid-unbounded-pattern-subquery")), ErrorInvalidRule)

	assertInvalid("named-window-data-window", From[patternSubqueryCandidate](env, "PatternSubqueryInvalidCandidate").Filter(
		SubqueryExists(
			FromNamedWindow(env, "PatternSubqueryInvalidWindow").Window(LastEvent()),
			Literal(true),
		),
	).Query(StatementName("invalid-named-window-pattern-subquery")), ErrorInvalidRule)

	assertInvalid("in-type-mismatch", From[patternSubqueryCandidate](env, "PatternSubqueryInvalidCandidate").Filter(
		SubqueryIn[int64](
			Field[patternSubqueryCandidate, int64]("id"),
			FromNamedWindow(env, "PatternSubqueryInvalidWindow"),
			Field[any, string]("symbol"),
		),
	).Query(StatementName("invalid-in-type-pattern-subquery")), ErrorTypeMismatch)
}
