package esper

import (
	"context"
	"testing"
)

// Parity coverage for the lookupable-limited filter executions of
// ExprFilterOptimizableLookupableLimitedExpr:
//
// - ExprFilterOptLkupConstantEqualsNull: null = 'a' never matches.
// - ExprFilterOptLkupEqualsCoercion: doublePrimitive + doubleBoxed =
//   Integer.parseInt('10') with numeric coercion to the property type.
// - ExprFilterOptLkupEqualsOneStmt: pattern followed-by filter
//   p10 || p11 = 'ax' over a concatenated lookupable.
// - ExprFilterOptLkupInSetOfValue: pattern filter
//   longPrimitive + longBoxed in (a.id, b.id, c.id).
// - ExprFilterOptLkupInRangeWCoercion: pattern filter
//   longPrimitive + longBoxed in [a.id - 2 : b.id + 2] and its not-in form.
//
// The Java executions additionally assert internal filter-index plan shape
// (assertFilterSvcSingle with EQUAL / IN_LIST_OF_VALUES / RANGE_CLOSED /
// NOT_RANGE_CLOSED on the concatenated lookupable). Those are engine-internal
// planning assertions with no observable Go equivalent; this slice covers the
// observable event selection behavior, which must be identical whether or not
// the filter is index-optimized. The remaining executions of the class
// (CurrentTimestampWEquals, CurrentTimestampCompare, Disqualify,
// EqualsOneStmtWPatternSharingIndex, EqualsMultiStmtSharingIndex) assert
// filter-service index internals and/or infrastructure (hooks, variables,
// tables, declared expressions, scripts) outside this slice's scope.

// lkupS0 mirrors Java SupportBean_S0 (id).
type lkupS0 struct {
	ID int `esper:"id"`
}

// lkupS1 mirrors Java SupportBean_S1 (id, p10, p11).
type lkupS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
	P11 string `esper:"p11"`
}

// lkupS2 mirrors Java SupportBean_S2 (id).
type lkupS2 struct {
	ID int `esper:"id"`
}

func newFilterLkupEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[filterOptimizableBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[lkupS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[lkupS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[lkupS2](env, "SupportBean_S2"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	return env, engine
}

// TestExprFilterOptLkupConstantEqualsNullParity covers
// ExprFilterOptLkupConstantEqualsNull: the filter null = 'a' evaluates to
// null and never delivers an event.
// Java runtime: java-runtime-61170f8386522145cdeb.
func TestExprFilterOptLkupConstantEqualsNullParity(t *testing.T) {
	env, engine := newFilterLkupEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(
		From[filterOptimizableBean](env, "SupportBean").
			Filter(Equal[string](NullLiteral[string](), Literal("a"))).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	fired := subscribeOptimizable(t, deployment, "s0")
	sendOptimizableBean(t, engine, filterOptimizableBean{TheString: "E1"})
	if fired() {
		t.Fatal("null = 'a' must never match")
	}
}

// TestExprFilterOptLkupEqualsCoercionParity covers ExprFilterOptLkupEqualsCoercion:
// doublePrimitive + doubleBoxed = Integer.parseInt('10') with the constant
// folded and coerced to the double lookupable.
// Java runtime: java-runtime-94a2be557857d83a0b1b13a463ab267.
func TestExprFilterOptLkupEqualsCoercionParity(t *testing.T) {
	env, engine := newFilterLkupEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(
		From[filterOptimizableBean](env, "SupportBean").
			Filter(Equal[float64](
				Add[float64](Field[filterOptimizableBean, float64]("doublePrimitive"), Field[filterOptimizableBean, float64]("doubleBoxed")),
				Literal(10.0),
			)).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	fired := subscribeOptimizable(t, deployment, "s0")
	for _, tc := range []struct {
		doublePrimitive, doubleBoxed float64
		want                         bool
	}{
		{5, 5, true},
		{10, 0, true},
		{0, 10, true},
		{0, 9, false},
	} {
		doubleBoxed := tc.doubleBoxed
		sendOptimizableBean(t, engine, filterOptimizableBean{DoublePrimitive: tc.doublePrimitive, DoubleBoxed: &doubleBoxed})
		if got := fired(); got != tc.want {
			t.Fatalf("coercion (%v,%v): fired=%v want %v", tc.doublePrimitive, tc.doubleBoxed, got, tc.want)
		}
	}
}

// TestExprFilterOptLkupEqualsOneStmtParity covers ExprFilterOptLkupEqualsOneStmt:
// pattern[s0=SupportBean_S0 -> every SupportBean_S1(p10 || p11 = 'ax')] with
// the concatenated lookupable compared against a constant.
// Java runtime: java-runtime-85175d99c086015482b2.
func TestExprFilterOptLkupEqualsOneStmtParity(t *testing.T) {
	env, engine := newFilterLkupEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	concat := Concat(Field[lkupS1, string]("p10"), Field[lkupS1, string]("p11"))
	pattern := PatternFrom(From[lkupS0](env, "SupportBean_S0"), "s0", Literal(true)).
		Then(PatternFrom(From[lkupS1](env, "SupportBean_S1"), "s", Equal[string](concat, Literal("ax"))).Every())
	plan, err := env.Build(pattern.Select(Alias("s_theString", TagField[string]("s", "p11"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	fired := subscribeOptimizable(t, deployment, "s0")

	if err := engine.SendEvent(context.Background(), lkupS0{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if fired() {
		t.Fatal("S0 alone must not fire")
	}
	for _, tc := range []struct {
		p10, p11 string
		want     bool
	}{
		{"a", "x", true},
		{"a", "y", false},
		{"b", "x", false},
		{"a", "x", true},
	} {
		if err := engine.SendEvent(context.Background(), lkupS1{P10: tc.p10, P11: tc.p11}); err != nil {
			t.Fatal(err)
		}
		if got := fired(); got != tc.want {
			t.Fatalf("one-stmt (p10=%q,p11=%q): fired=%v want %v", tc.p10, tc.p11, got, tc.want)
		}
	}
}

// TestExprFilterOptLkupInSetOfValueParity covers ExprFilterOptLkupInSetOfValue:
// pattern[a=S0 -> b=S1 -> c=S2 -> every SupportBean(longPrimitive + longBoxed
// in (a.id, b.id, c.id))] — IN set over pattern-tagged values.
// Java runtime: java-runtime-93515ff2a2f83c9dc747.
func TestExprFilterOptLkupInSetOfValueParity(t *testing.T) {
	env, engine := newFilterLkupEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	sum := Add[int64](Field[filterOptimizableBean, int64]("longPrimitive"), Field[filterOptimizableBean, int64]("longBoxed"))
	pred := InOf(sum,
		TagField[int64]("a", "id"),
		TagField[int64]("b", "id"),
		TagField[int64]("c", "id"),
	)
	pattern := PatternFrom(From[lkupS0](env, "SupportBean_S0"), "a", Literal(true)).
		Then(PatternFrom(From[lkupS1](env, "SupportBean_S1"), "b", Literal(true))).
		Then(PatternFrom(From[lkupS2](env, "SupportBean_S2"), "c", Literal(true))).
		Then(PatternFrom(From[filterOptimizableBean](env, "SupportBean"), "s", pred).Every())
	plan, err := env.Build(pattern.Select(Alias("s_long", TagField[int64]("s", "longPrimitive"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	fired := subscribeOptimizable(t, deployment, "s0")

	if err := engine.SendEvent(context.Background(), lkupS0{ID: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), lkupS1{ID: 200}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), lkupS2{ID: 3000}); err != nil {
		t.Fatal(err)
	}
	if fired() {
		t.Fatal("a/b/c alone must not fire")
	}
	for _, tc := range []struct {
		longPrimitive, longBoxed int64
		want                     bool
	}{
		{0, 9, false},
		{9, 1, true},
		{199, 1, true},
		{2090, 910, true},
	} {
		sendOptimizableBean(t, engine, filterOptimizableBean{LongPrimitive: tc.longPrimitive, LongBoxed: &tc.longBoxed})
		if got := fired(); got != tc.want {
			t.Fatalf("in-set (%d,%d): fired=%v want %v", tc.longPrimitive, tc.longBoxed, got, tc.want)
		}
	}
}

// TestExprFilterOptLkupInRangeWCoercionParity covers
// ExprFilterOptLkupInRangeWCoercion: pattern[a=S0 -> b=S1 -> every
// SupportBean(longPrimitive + longBoxed in [a.id - 2 : b.id + 2])] plus its
// not-in variant. Bounds are computed from pattern tags.
// Java runtimes: java-runtime-94a2be557857d83a0b1b (in), java-runtime-530153bd9 covers
// Disqualify and is not part of this slice; the not-in variant shares the
// InRangeWCoercion execution (same runtime, second epl form).
func TestExprFilterOptLkupInRangeWCoercionParity(t *testing.T) {
	for _, form := range []struct {
		name string
		not  bool
	}{
		{"in", false},
		{"not-in", true},
	} {
		t.Run(form.name, func(t *testing.T) {
			env, engine := newFilterLkupEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()

			sum := Add[int64](Field[filterOptimizableBean, int64]("longPrimitive"), Field[filterOptimizableBean, int64]("longBoxed"))
			lower := Subtract[int64](TagField[int64]("a", "id"), Literal[int64](2))
			upper := Add[int64](TagField[int64]("b", "id"), Literal[int64](2))
			var pred Expression[bool]
			if form.not {
				pred = NotBetweenRangeOf(sum, lower, upper, true, true)
			} else {
				pred = BetweenRangeOf(sum, lower, upper, true, true)
			}
			pattern := PatternFrom(From[lkupS0](env, "SupportBean_S0"), "a", Literal(true)).
				Then(PatternFrom(From[lkupS1](env, "SupportBean_S1"), "b", Literal(true))).
				Then(PatternFrom(From[filterOptimizableBean](env, "SupportBean"), "s", pred).Every())
			plan, err := env.Build(pattern.Select(Alias("s_long", TagField[int64]("s", "longPrimitive"))).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			fired := subscribeOptimizable(t, deployment, "s0")

			if err := engine.SendEvent(context.Background(), lkupS0{ID: 10}); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(context.Background(), lkupS1{ID: 200}); err != nil {
				t.Fatal(err)
			}
			if fired() {
				t.Fatal("a/b alone must not fire")
			}
			for _, tc := range []struct {
				longPrimitive, longBoxed int64
				want                     bool
			}{
				{3, 4, false},
				{5, 3, true},
				{1, 99, true},
				{101, 101, true},
				{200, 3, false},
			} {
				expected := tc.want
				if form.not {
					expected = !expected
				}
				sendOptimizableBean(t, engine, filterOptimizableBean{LongPrimitive: tc.longPrimitive, LongBoxed: &tc.longBoxed})
				if got := fired(); got != expected {
					t.Fatalf("%s (%d,%d): fired=%v want %v", form.name, tc.longPrimitive, tc.longBoxed, got, expected)
				}
			}
		})
	}
}
