package esper

import (
	"context"
	"testing"
)

// Parity coverage for PatternGuardWhile (see docs
// esper-go-port-implementation-plan.md): the expression guard
// ("pattern while (expression)") inspects every match the guarded
// subexpression reports - a true result passes the match, a false result
// quits the guarded node permanently (Esper guardQuit), and a null result
// swallows the match without quitting. The fluent counterpart is
// PatternStream.WhileGuard; the definition-level While pre-filter remains a
// separate Go extension with clear-and-continue semantics.

// TestPatternGuardWhileSimpleMatchesEsper mirrors PatternGuardWhileSimple:
// (every a=SupportBean) while (a.theString like 'E%') fires for E1/E2, and
// the first non-matching event falsifies the guard so E3 never fires.
func TestPatternGuardWhileSimpleMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	pattern := PatternFrom(base, "a", Literal(true)).Every().
		WhileGuard(LikeOf(TagField[string]("a", "theString"), Literal("E%")))
	plan, err := env.Build(pattern.Select(Alias("c0", TagField[string]("a", "theString"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var fired []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatal("expected row")
			}
			fired = append(fired, row.Get("c0").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendPatternOpBean(t, engine, "E1", 0)
	sendPatternOpBean(t, engine, "E2", 0)
	sendPatternOpBean(t, engine, "X", 0)
	sendPatternOpBean(t, engine, "E3", 0)
	sendPatternOpBean(t, engine, "X", 0)

	if len(fired) != 2 || fired[0] != "E1" || fired[1] != "E2" {
		t.Fatalf("fired = %v, want [E1 E2]", fired)
	}
}

// TestPatternGuardWhileOpMatchesEsper mirrors the PatternOp W-harness cases:
// the guard only inspects matches the guarded child reports, and a false
// result kills the guarded branch for the rest of the event set.
func TestPatternGuardWhileOpMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)
	neqB := func(id string) Expression[bool] {
		return NotEqual[string](TagField[string]("b", "id"), Literal(id))
	}
	cases := []patternMuCase{
		{
			name:       "a=A -> (every b=B) while(b.id != B2)",
			singleTags: []string{"a", "b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.a, "a", trueExpr).
					Then(PatternFrom(s.b, "b", trueExpr).Every().WhileGuard(neqB("B2")))
			},
			want: map[string][]string{"B1": {"a=A1 b=B1"}},
		},
		{
			name:       "a=A -> (every b=B) while(b.id != B3)",
			singleTags: []string{"a", "b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.a, "a", trueExpr).
					Then(PatternFrom(s.b, "b", trueExpr).Every().WhileGuard(neqB("B3")))
			},
			want: map[string][]string{"B1": {"a=A1 b=B1"}, "B2": {"a=A1 b=B2"}},
		},
		{
			name:       "(every b=B) while(b.id != B3)",
			singleTags: []string{"b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).Every().WhileGuard(neqB("B3"))
			},
			want: map[string][]string{"B1": {"b=B1"}, "B2": {"b=B2"}},
		},
		{
			// The Java suite builds this form twice, once from EPL text and
			// once from the SODA object model; both compile to the same
			// guard, so the fluent form replays the expectations as well.
			name:       "soda (every b=B) while(b.id != B3)",
			singleTags: []string{"b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).Every().WhileGuard(neqB("B3"))
			},
			want: map[string][]string{"B1": {"b=B1"}, "B2": {"b=B2"}},
		},
		{
			name:       "(every b=B) while(b.id != B1)",
			singleTags: []string{"b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).Every().WhileGuard(neqB("B1"))
			},
			want: map[string][]string{},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			runPatternMuCase(t, testCase)
		})
	}
}

// TestPatternGuardWhileVariableMatchesEsper mirrors PatternVariable: a
// variable guard passes while the variable is true and falsifies every live
// branch once it flips to false.
func TestPatternGuardWhileVariableMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	if err := env.RegisterVariable("myVariable", true); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	pattern := PatternFrom(base, "a", LikeOf(Field[patternOpBean, string]("theString"), Literal("A%"))).Every().
		Then(PatternFrom(base, "b", LikeOf(Field[patternOpBean, string]("theString"), Literal("B%"))).Every().
			WhileGuard(VariableRef[bool]("myVariable")))
	plan, err := env.Build(pattern.Select(
		Alias("a", TagField[string]("a", "theString")),
		Alias("b", TagField[string]("b", "theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatal("expected row")
			}
			a, _ := row.Get("a").Any().(string)
			b, _ := row.Get("b").Any().(string)
			rows = append(rows, []string{a, b})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendPatternOpBean(t, engine, "A1", 1)
	sendPatternOpBean(t, engine, "A2", 2)
	sendPatternOpBean(t, engine, "B1", 100)
	if len(rows) != 2 {
		t.Fatalf("B1 rows = %v, want two rows (a=A1 and a=A2)", rows)
	}
	if rows[0][0] != "A1" || rows[1][0] != "A2" || rows[0][1] != "B1" || rows[1][1] != "B1" {
		t.Fatalf("rows = %v, want [[A1 B1] [A2 B1]]", rows)
	}

	if err := engine.SetVariable(context.Background(), "myVariable", false); err != nil {
		t.Fatal(err)
	}
	sendPatternOpBean(t, engine, "A3", 3)
	sendPatternOpBean(t, engine, "A4", 4)
	sendPatternOpBean(t, engine, "B2", 200)
	if len(rows) != 2 {
		t.Fatalf("rows after variable flip = %v, want no new rows", rows)
	}
}

// TestPatternGuardWhileInvalidMatchesEsper mirrors PatternInvalid: guards
// whose expression cannot resolve are rejected at Build. A non-boolean
// literal guard is a Go compile-time type error through Expression[bool]
// and therefore not expressible, the typed-API counterpart of Esper's
// "requires a single expression as a parameter returning a true or false
// (boolean) value" parse error.
func TestPatternGuardWhileInvalidMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	base := From[patternOpBean](env, "SupportBean")
	selections := []Selection{Alias("c0", TagField[string]("a", "theString"))}
	if _, err := env.Build(PatternFrom(base, "a", Literal(true)).Every().
		WhileGuard(VariableRef[bool]("undefinedVariable")).
		Select(selections...).Query(StatementName("s0"))); err == nil {
		t.Error("unbound variable guard: expected a build error")
	}
	if _, err := env.Build(PatternFrom(base, "a", Literal(true)).Every().
		WhileGuard(Field[patternOpBean, bool]("noSuchField")).
		Select(selections...).Query(StatementName("s0"))); err == nil {
		t.Error("unbound field guard: expected a build error")
	}
}
