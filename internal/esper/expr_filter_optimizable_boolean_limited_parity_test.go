package esper

import (
	"context"
	"testing"
)

// Parity coverage for the boolean-limited (REBOOL) filter executions of
// ExprFilterOptimizableBooleanLimitedExpr:
//
// - ExprFilterOptReboolWithEquals: pattern followed-by with a cross-tag
//   arithmetic equality combined with a constant equality.
// - ExprFilterOptReboolMultiple: two statements with the same two-regexp
//   conjunction in either order.
// - ExprFilterOptReboolNoValueConcat: p00||p01 = p02||p03.
// - ExprFilterOptReboolConstValueRegexpLHS: 'abc' regexp theString.
// - ExprFilterOptReboolConstValueRegexpRHS: theString regexp '.*a.*' (s0/s1)
//   and '.*b.*' (s2).
//
// The Java executions additionally assert internal filter-index plan shape
// (REBOOL operator text, index/value sharing between statements via
// assertFilterSvcSingle/assertSameFilterEntry, SupportFilterPlanHook triplets).
// Those are engine-internal planning assertions with no observable Go
// equivalent; this slice covers the observable event selection behavior,
// which must be identical whether or not the filter is rewritten.

type reboolSupportBeanS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
	P02 string `esper:"p02"`
	P03 string `esper:"p03"`
}

func newReboolEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[filterOptimizableBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[reboolSupportBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	return env
}

func subscribeRebool(t *testing.T, deployment *Deployment, name string) func() bool {
	return subscribeOptimizable(t, deployment, name)
}

// TestExprFilterOptReboolWithEqualsParity covers ExprFilterOptReboolWithEquals
// (java-runtime-90af560a2): pattern [s0=SupportBean_S0 -> SupportBean(
// intPrimitive+5=s0.id and theString='a')]. Observable contract: after
// S0(id=10), SB("a",10) false, SB("b",5) false, SB("a",5) true.
func TestExprFilterOptReboolWithEqualsParity(t *testing.T) {
	env := newReboolEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	s0Step := PatternFrom(From[reboolSupportBeanS0](env, "SupportBean_S0"), "s0", Literal[bool](true))
	sbStep := PatternFrom(From[filterOptimizableBean](env, "SupportBean"), "sb", And(
		Equal[int](Add[int](Field[filterOptimizableBean, int]("intPrimitive"), Literal[int](5)), TagField[int]("s0", "id")),
		Equal[string](Field[filterOptimizableBean, string]("theString"), Literal[string]("a")),
	))
	pattern := s0Step.Then(sbStep)
	plan, err := env.Build(pattern.Select(Alias("sb", PatternEvent("sb"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	fired := subscribeRebool(t, deployment, "s0")

	if err := engine.SendEvent(context.Background(), reboolSupportBeanS0{ID: 10}); err != nil {
		t.Fatal(err)
	}
	// Java assertion sequence: a/10 false, b/5 false, a/5 true.
	for _, tc := range []struct {
		theString    string
		intPrimitive int
		want         bool
	}{
		{"a", 10, false},
		{"b", 5, false},
		{"a", 5, true},
	} {
		sendOptimizableBean(t, engine, filterOptimizableBean{TheString: tc.theString, IntPrimitive: tc.intPrimitive})
		if got := fired(); got != tc.want {
			t.Fatalf("theString=%q intPrimitive=%d: fired=%v want %v", tc.theString, tc.intPrimitive, got, tc.want)
		}
	}
}

// TestExprFilterOptReboolMultipleParity covers ExprFilterOptReboolMultiple
// (java-runtime-31bec6993): statements s0/s1 carry the same conjunction
// p00 regexp '.*X' and p01 regexp '.*Y' in either operand order. Observable
// contract: (AX,AZ) false, (AY,AX) false, (AX,BY) true on both statements.
func TestExprFilterOptReboolMultipleParity(t *testing.T) {
	env := newReboolEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	p00 := Field[reboolSupportBeanS0, string]("p00")
	p01 := Field[reboolSupportBeanS0, string]("p01")
	predicates := []Expression[bool]{
		And(RegexpMatch(p00, Literal(".*X")), RegexpMatch(p01, Literal(".*Y"))),
		And(RegexpMatch(p01, Literal(".*Y")), RegexpMatch(p00, Literal(".*X"))),
	}

	var fireds []func() bool
	for i, pred := range predicates {
		plan, err := env.Build(
			From[reboolSupportBeanS0](env, "SupportBean_S0").Filter(pred).Query(StatementName("s" + string(rune('0'+i)))),
		)
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer deployment.Undeploy(context.Background())
		fireds = append(fireds, subscribeRebool(t, deployment, "s"+string(rune('0'+i))))
	}

	for _, tc := range []struct {
		p00, p01 string
		want     bool
	}{
		{"AX", "AZ", false},
		{"AY", "AX", false},
		{"AX", "BY", true},
	} {
		if err := engine.SendEvent(context.Background(), reboolSupportBeanS0{P00: tc.p00, P01: tc.p01}); err != nil {
			t.Fatal(err)
		}
		for i, fired := range fireds {
			if got := fired(); got != tc.want {
				t.Fatalf("stmt%d p00=%q p01=%q: fired=%v want %v", i, tc.p00, tc.p01, got, tc.want)
			}
		}
	}
}

// TestExprFilterOptReboolNoValueConcatParity covers ExprFilterOptReboolNoValueConcat
// (java-runtime-0810493a): p00||p01 = p02||p03 across two structurally
// identical statements. Observable contract: S0(a,b,a,b) true on both,
// S0(a,b,a,c) false on both.
func TestExprFilterOptReboolNoValueConcatParity(t *testing.T) {
	env := newReboolEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	pred := Equal[string](
		Concat(Field[reboolSupportBeanS0, string]("p00"), Field[reboolSupportBeanS0, string]("p01")),
		Concat(Field[reboolSupportBeanS0, string]("p02"), Field[reboolSupportBeanS0, string]("p03")),
	)

	var fireds []func() bool
	for i := 0; i < 2; i++ {
		plan, err := env.Build(
			From[reboolSupportBeanS0](env, "SupportBean_S0").Filter(pred).Query(StatementName("s" + string(rune('0'+i)))),
		)
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer deployment.Undeploy(context.Background())
		fireds = append(fireds, subscribeRebool(t, deployment, "s"+string(rune('0'+i))))
	}

	for _, tc := range []struct {
		p00, p01, p02, p03 string
		want               bool
	}{
		{"a", "b", "a", "b", true},
		{"a", "b", "a", "c", false},
	} {
		if err := engine.SendEvent(context.Background(), reboolSupportBeanS0{
			P00: tc.p00, P01: tc.p01, P02: tc.p02, P03: tc.p03,
		}); err != nil {
			t.Fatal(err)
		}
		for i, fired := range fireds {
			if got := fired(); got != tc.want {
				t.Fatalf("stmt%d concat %s||%s vs %s||%s: fired=%v want %v", i, tc.p00, tc.p01, tc.p02, tc.p03, got, tc.want)
			}
		}
	}
}

// TestExprFilterOptReboolConstValueRegexpLHSParity covers
// ExprFilterOptReboolConstValueRegexpLHS (java-runtime-72b50080e):
// 'abc' regexp theString across two statements. Observable contract:
// SB(".*bc") true on both, SB(".*d") false on both.
func TestExprFilterOptReboolConstValueRegexpLHSParity(t *testing.T) {
	env := newReboolEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	pred := RegexpMatch(Literal("abc"), Field[filterOptimizableBean, string]("theString"))

	var fireds []func() bool
	for i := 0; i < 2; i++ {
		plan, err := env.Build(
			From[filterOptimizableBean](env, "SupportBean").Filter(pred).Query(StatementName("s" + string(rune('0'+i)))),
		)
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer deployment.Undeploy(context.Background())
		fireds = append(fireds, subscribeRebool(t, deployment, "s"+string(rune('0'+i))))
	}

	for _, tc := range []struct {
		theString string
		want      bool
	}{
		{".*bc", true},
		{".*d", false},
	} {
		sendOptimizableBean(t, engine, filterOptimizableBean{TheString: tc.theString})
		for i, fired := range fireds {
			if got := fired(); got != tc.want {
				t.Fatalf("stmt%d theString=%q: fired=%v want %v", i, tc.theString, got, tc.want)
			}
		}
	}
}

// TestExprFilterOptReboolConstValueRegexpRHSParity covers
// ExprFilterOptReboolConstValueRegexpRHS (java-runtime-8941f4303):
// s0/s1 use theString regexp '.*a.*' and s2 uses '.*b.*'. Observable
// contract: garden -> (T,T,F), house -> (F,F,F), grub -> (F,F,T).
func TestExprFilterOptReboolConstValueRegexpRHSParity(t *testing.T) {
	env := newReboolEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	patterns := []string{".*a.*", ".*a.*", ".*b.*"}
	var fireds []func() bool
	for i, pattern := range patterns {
		pred := RegexpMatch(Field[filterOptimizableBean, string]("theString"), Literal(pattern))
		plan, err := env.Build(
			From[filterOptimizableBean](env, "SupportBean").Filter(pred).Query(StatementName("s" + string(rune('0'+i)))),
		)
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer deployment.Undeploy(context.Background())
		fireds = append(fireds, subscribeRebool(t, deployment, "s"+string(rune('0'+i))))
	}

	for _, tc := range []struct {
		theString string
		want      [3]bool
	}{
		{"garden", [3]bool{true, true, false}},
		{"house", [3]bool{false, false, false}},
		{"grub", [3]bool{false, false, true}},
	} {
		sendOptimizableBean(t, engine, filterOptimizableBean{TheString: tc.theString})
		for i, fired := range fireds {
			if got := fired(); got != tc.want[i] {
				t.Fatalf("stmt%d theString=%q: fired=%v want %v", i, tc.theString, got, tc.want[i])
			}
		}
	}
}
