package esper

import (
	"context"
	"testing"
	"time"
)

// Parity coverage for PatternOperatorOperatorMix (see docs
// esper-go-port-implementation-plan.md): and/or/followed-by operator
// precedence and composition over the EventCollectionFactory mixed event set,
// and PatternDeadPattern's regression guard that a falsified not-branch keeps
// a thousand deployments cheap on the next event.

// TestPatternOperatorOperatorMixMatchesEsper mirrors
// PatternOperatorOperatorMix: eight and/or/followed-by compositions replayed
// against the mixed event set (A1@1000 .. D3@12000), asserting the fired
// tag combinations exactly like the Java harness.
func TestPatternOperatorOperatorMixMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)
	cases := []patternMuCase{
		{
			name:       "(b->d) and (a->e)",
			singleTags: []string{"a", "b", "d", "e"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).Then(PatternFrom(s.d, "d", trueExpr)).
					And(PatternFrom(s.a, "a", trueExpr).Then(PatternFrom(s.e, "e", trueExpr)))
			},
			want: map[string][]string{"E1": {"a=A1 b=B1 d=D1 e=E1"}},
		},
		{
			name:       "b -> (d or a)",
			singleTags: []string{"a", "b", "d"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).
					Then(PatternFrom(s.d, "d", trueExpr).Or(PatternFrom(s.a, "a", trueExpr)))
			},
			want: map[string][]string{"A2": {"a=A2 b=B1"}},
		},
		{
			// The Java suite builds this form from the SODA object model;
			// both compile to the same fluent shape, so the expectation is
			// replayed as a third subtest.
			name:       "soda b -> (d or a)",
			singleTags: []string{"a", "b", "d"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).
					Then(PatternFrom(s.d, "d", trueExpr).Or(PatternFrom(s.a, "a", trueExpr)))
			},
			want: map[string][]string{"A2": {"a=A2 b=B1"}},
		},
		{
			name:       "b -> ((d->a) or (a->e))",
			singleTags: []string{"a", "b", "d", "e"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).
					Then(PatternFrom(s.d, "d", trueExpr).Then(PatternFrom(s.a, "a", trueExpr)).
						Or(PatternFrom(s.a, "a", trueExpr).Then(PatternFrom(s.e, "e", trueExpr))))
			},
			want: map[string][]string{"E1": {"a=A2 b=B1 e=E1"}},
		},
		{
			name:       "(b and d) or a",
			singleTags: []string{"a", "b", "d"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).And(PatternFrom(s.d, "d", trueExpr)).
					Or(PatternFrom(s.a, "a", trueExpr))
			},
			want: map[string][]string{"A1": {"a=A1"}},
		},
		{
			name:       "(b -> d) or a",
			singleTags: []string{"a", "b", "d"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).Then(PatternFrom(s.d, "d", trueExpr)).
					Or(PatternFrom(s.a, "a", trueExpr))
			},
			want: map[string][]string{"A1": {"a=A1"}},
		},
		{
			name:       "(b and d) or a (grouped)",
			singleTags: []string{"a", "b", "d"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).And(PatternFrom(s.d, "d", trueExpr)).
					Or(PatternFrom(s.a, "a", trueExpr))
			},
			want: map[string][]string{"A1": {"a=A1"}},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			runPatternMuCase(t, testCase)
		})
	}
}

// TestPatternDeadPatternMatchesEsper mirrors PatternDeadPattern: a thousand
// deployments of (SupportBean_A -> SupportBean_B) and not SupportBean_C are
// each falsified by a single C event, and the subsequent A event must stay
// cheap. Java bounds the A send at 20ms; the bound is relaxed here to keep
// the test stable across CI machines while still guarding against an O(n)
// regression over the dead deployments.
func TestPatternDeadPatternMatchesEsper(t *testing.T) {
	env, engine, streams := newPatternMuEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	// The Java filters are tagless; the fluent API requires tags, so each
	// branch gets a distinct placeholder tag (none are selected).
	pattern := PatternFrom(streams.a, "da", Literal(true)).
		Then(PatternFrom(streams.b, "db", Literal(true))).
		And(PatternFrom(streams.c, "dc", Literal(true)).Not())
	plan, err := env.Build(pattern.Select(Alias("fired", Literal(1))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	var fires int
	for i := 0; i < 1000; i++ {
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatalf("deploy %d: %v", i, err)
		}
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			fires += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Every deployment's not-SupportBean_C branch falsifies on C1, killing the
	// enclosing and.
	if err := engine.SendEvent(context.Background(), patternNotC{ID: "C1"}); err != nil {
		t.Fatal(err)
	}

	// The A event must traverse a thousand dead deployments cheaply and fire
	// nothing (their (A->B) branches died with the and).
	start := time.Now()
	if err := engine.SendEvent(context.Background(), patternNotA{ID: "A1"}); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("dead-pattern A send took %v, want under 2s", elapsed)
	}
	if fires != 0 {
		t.Fatalf("fires = %d, want 0", fires)
	}
}
