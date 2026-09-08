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
// - ExprFilterOptReboolContextValueDeep / ContextValueWithConst: context
//   partition values read inside filter regexp operands.
// - ExprFilterOptReboolPatternValueWithConst: pattern tag value read inside
//   a followed-by filter regexp operand.
// - ExprFilterOptReboolDuplicateLike / DuplicateRegexp: duplicate like and
//   not-regexp conjunction guards.
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

type reboolSupportBeanS1 struct {
	ID  int     `esper:"id"`
	P10 string  `esper:"p10"`
	P11 *string `esper:"p11"`
	P12 *string `esper:"p12"`
	P13 *string `esper:"p13"`
}

func reboolStringPtr(value string) *string { return &value }

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

func newReboolEnvWithS1(t *testing.T) *Environment {
	t.Helper()
	env := newReboolEnv(t)
	if _, err := RegisterStruct[reboolSupportBeanS1](env, "SupportBean_S1"); err != nil {
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

// TestExprFilterOptReboolContextValueDeepParity covers
// ExprFilterOptReboolContextValueDeep (java-runtime-273669bcae26e6ec054d):
// context MyContext select * from SupportBean_S1(p10 regexp p11 ||
// context.s0.p00). Java starts the context with a SupportBean_S0 filter;
// Go expresses the same partition lifecycle as a pattern-initiated context
// because Go filter-start evaluation observes only the consuming
// statement's stream (approved adaptation, identical observable behavior).
// Contract: S0(1,".*X") starts the partition silently; S1("gardenX","abc")
// false (matches "abc.*X" never), S1("garden","gard") false, S1("gardenX",
// "gard") true.
func TestExprFilterOptReboolContextValueDeepParity(t *testing.T) {
	env := newReboolEnvWithS1(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	if _, err := CreatePatternInitiatedContext(env, "MyContext",
		PatternFrom(From[reboolSupportBeanS0](env, "SupportBean_S0"), "s0", Literal[bool](true))); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(
		From[reboolSupportBeanS1](env, "SupportBean_S1").
			Filter(RegexpMatch(
				Field[reboolSupportBeanS1, string]("p10"),
				Concat(Field[reboolSupportBeanS1, string]("p11"), ContextPatternField[string]("s0", "p00")))).
			Query(StatementName("s0"), WithContext("MyContext")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	fired := subscribeRebool(t, deployment, "s0")

	if err := engine.SendEvent(context.Background(), reboolSupportBeanS0{ID: 1, P00: ".*X"}); err != nil {
		t.Fatal(err)
	}
	if fired() {
		t.Fatal("context start event must not be delivered to the consumer")
	}
	for _, tc := range []struct {
		p10, p11 string
		want     bool
	}{
		{"gardenX", "abc", false},
		{"garden", "gard", false},
		{"gardenX", "gard", true},
	} {
		if err := engine.SendEvent(context.Background(), reboolSupportBeanS1{
			ID: 1, P10: tc.p10, P11: reboolStringPtr(tc.p11),
		}); err != nil {
			t.Fatal(err)
		}
		if got := fired(); got != tc.want {
			t.Fatalf("p10=%q p11=%q: fired=%v want %v", tc.p10, tc.p11, got, tc.want)
		}
	}
}

// TestExprFilterOptReboolContextValueWithConstParity covers
// ExprFilterOptReboolContextValueWithConst (java-runtime-1da34c09fa675957b063):
// SupportBean_S1(p10 || 'abc' regexp context.s0.p00). Contract:
// S0(1,"x.*abc") starts the partition; "ydotabc" false ("ydotabcabc" vs
// "x.*abc"), "xdotabc" true. The Java helper sends p11=null (pointer nil).
func TestExprFilterOptReboolContextValueWithConstParity(t *testing.T) {
	env := newReboolEnvWithS1(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	if _, err := CreatePatternInitiatedContext(env, "MyContext",
		PatternFrom(From[reboolSupportBeanS0](env, "SupportBean_S0"), "s0", Literal[bool](true))); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(
		From[reboolSupportBeanS1](env, "SupportBean_S1").
			Filter(RegexpMatch(
				Concat(Field[reboolSupportBeanS1, string]("p10"), Literal("abc")),
				ContextPatternField[string]("s0", "p00"))).
			Query(StatementName("s0"), WithContext("MyContext")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	fired := subscribeRebool(t, deployment, "s0")

	if err := engine.SendEvent(context.Background(), reboolSupportBeanS0{ID: 1, P00: "x.*abc"}); err != nil {
		t.Fatal(err)
	}
	if fired() {
		t.Fatal("context start event must not be delivered to the consumer")
	}
	for _, tc := range []struct {
		p10  string
		want bool
	}{
		{"ydotabc", false},
		{"xdotabc", true},
	} {
		if err := engine.SendEvent(context.Background(), reboolSupportBeanS1{ID: 1, P10: tc.p10}); err != nil {
			t.Fatal(err)
		}
		if got := fired(); got != tc.want {
			t.Fatalf("p10=%q: fired=%v want %v", tc.p10, got, tc.want)
		}
	}
}

// TestExprFilterOptReboolPatternValueWithConstParity covers
// ExprFilterOptReboolPatternValueWithConst (java-runtime-11f60c98a851b822d97f):
// pattern[s0=SupportBean_S0 -> SupportBean_S1(p10 || 'abc' regexp s0.p00)].
// Contract: S0(1,"x.*abc") arms the pattern without output; "ydotabc" false,
// "xdotabc" true.
func TestExprFilterOptReboolPatternValueWithConstParity(t *testing.T) {
	env := newReboolEnvWithS1(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	s0Step := PatternFrom(From[reboolSupportBeanS0](env, "SupportBean_S0"), "s0", Literal[bool](true))
	sbStep := PatternFrom(From[reboolSupportBeanS1](env, "SupportBean_S1"), "sb",
		RegexpMatch(
			Concat(Field[reboolSupportBeanS1, string]("p10"), Literal("abc")),
			TagField[string]("s0", "p00")))
	plan, err := env.Build(s0Step.Then(sbStep).
		Select(Alias("sb", PatternEvent("sb"))).
		Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	fired := subscribeRebool(t, deployment, "s0")

	if err := engine.SendEvent(context.Background(), reboolSupportBeanS0{ID: 1, P00: "x.*abc"}); err != nil {
		t.Fatal(err)
	}
	if fired() {
		t.Fatal("first pattern leg alone must not deliver")
	}
	for _, tc := range []struct {
		p10  string
		want bool
	}{
		{"ydotabc", false},
		{"xdotabc", true},
	} {
		if err := engine.SendEvent(context.Background(), reboolSupportBeanS1{ID: 1, P10: tc.p10}); err != nil {
			t.Fatal(err)
		}
		if got := fired(); got != tc.want {
			t.Fatalf("p10=%q: fired=%v want %v", tc.p10, got, tc.want)
		}
	}
}

// TestExprFilterOptReboolDuplicateLikeParity covers
// ExprFilterOptReboolDuplicateLike (java-runtime-0f83ba904d5457bbc6d2):
// theString like '%' and theString like '%'. Observable contract: the
// duplicate conjunction still selects SB("x", 0).
func TestExprFilterOptReboolDuplicateLikeParity(t *testing.T) {
	env := newReboolEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	pred := And(
		Like(Field[filterOptimizableBean, string]("theString"), Literal("%")),
		Like(Field[filterOptimizableBean, string]("theString"), Literal("%")))
	plan, err := env.Build(
		From[filterOptimizableBean](env, "SupportBean").Filter(pred).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	fired := subscribeRebool(t, deployment, "s0")

	sendOptimizableBean(t, engine, filterOptimizableBean{TheString: "x", IntPrimitive: 0})
	if got := fired(); !got {
		t.Fatalf("duplicate like: fired=%v want true", got)
	}
}

// TestExprFilterOptReboolDuplicateRegexpParity covers
// ExprFilterOptReboolDuplicateRegexp (java-runtime-5253f42525fa48e8784c):
// theString regexp "test.*" and theString not regexp ".*\.gov" and
// theString not regexp ".*\.org" (Go regexp literals via \\. escapes).
// Contract: test.com true, test.gov false, test.org false, x.com false.
func TestExprFilterOptReboolDuplicateRegexpParity(t *testing.T) {
	env := newReboolEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	pred := And(
		And(
			RegexpMatch(Field[filterOptimizableBean, string]("theString"), Literal("test.*")),
			Not(RegexpMatch(Field[filterOptimizableBean, string]("theString"), Literal(`.*\.gov`)))),
		Not(RegexpMatch(Field[filterOptimizableBean, string]("theString"), Literal(`.*\.org`))))
	plan, err := env.Build(
		From[filterOptimizableBean](env, "SupportBean").Filter(pred).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	fired := subscribeRebool(t, deployment, "s0")

	for _, tc := range []struct {
		theString string
		want      bool
	}{
		{"test.com", true},
		{"test.gov", false},
		{"test.org", false},
		{"x.com", false},
	} {
		sendOptimizableBean(t, engine, filterOptimizableBean{TheString: tc.theString, IntPrimitive: 0})
		if got := fired(); got != tc.want {
			t.Fatalf("theString=%q: fired=%v want %v", tc.theString, got, tc.want)
		}
	}
}

// TestExprFilterOptReboolNoValueExprRegexpSelfParity covers
// ExprFilterOptReboolNoValueExprRegexpSelf (java-runtime-c21cb50c065d46513b04):
// p00 regexp p01 across two statements (Java deploys them via two
// compileDeploy calls with ` as a` / ` as s0` stream aliases; the aliases
// have no filter-side analogue). Contract: S0(1,"abc",".*c") true on both,
// S0(2,"abc",".*d") false on both.
func TestExprFilterOptReboolNoValueExprRegexpSelfParity(t *testing.T) {
	env := newReboolEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	pred := RegexpMatch(
		Field[reboolSupportBeanS0, string]("p00"),
		Field[reboolSupportBeanS0, string]("p01"))
	var fireds []func() bool
	for i := 0; i < 2; i++ {
		plan, err := env.Build(
			From[reboolSupportBeanS0](env, "SupportBean_S0").Filter(pred).Query(StatementName("s" + string(rune('0'+i)))))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		fireds = append(fireds, subscribeRebool(t, deployment, "s"+string(rune('0'+i))))
	}

	for _, tc := range []struct {
		id       int
		p00, p01 string
		want     bool
	}{
		{1, "abc", ".*c", true},
		{2, "abc", ".*d", false},
	} {
		if err := engine.SendEvent(context.Background(), reboolSupportBeanS0{ID: tc.id, P00: tc.p00, P01: tc.p01}); err != nil {
			t.Fatal(err)
		}
		for i, fired := range fireds {
			if got := fired(); got != tc.want {
				t.Fatalf("stmt%d id=%d p00=%q p01=%q: fired=%v want %v", i, tc.id, tc.p00, tc.p01, got, tc.want)
			}
		}
	}
}

// TestExprFilterOptReboolMixedValueRegexpRHSParity covers
// ExprFilterOptReboolMixedValueRegexpRHS (java-runtime-f6199a434eee24910555):
// four monitored statements whose regexp right-hand values come from
// non-literal sources — a constant variable (s0), a context partition value
// (s1), a pattern tag inside an every leg (s2), and a constant concat (s3).
// Java's shared REBOOL index/value plan assertions are engine-internal and
// out of scope. Contract: S0(1,".*abc.*") starts the context and arms the
// pattern silently; SB("xabsx",0) false on all four; SB("xabcx",0) true on
// all four. The context uses the pattern-start adaptation (see
// TestExprFilterOptReboolContextValueDeepParity).
func TestExprFilterOptReboolMixedValueRegexpRHSParity(t *testing.T) {
	env := newReboolEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	if err := env.RegisterVariable("MYVAR", ".*abc.*", ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	if _, err := CreatePatternInitiatedContext(env, "MyContext",
		PatternFrom(From[reboolSupportBeanS0](env, "SupportBean_S0"), "s0", Literal[bool](true))); err != nil {
		t.Fatal(err)
	}

	planDefs := []struct {
		name  string
		build func() (Plan, error)
	}{
		{"s0", func() (Plan, error) {
			return env.Build(From[filterOptimizableBean](env, "SupportBean").
				Filter(RegexpMatch(
					Field[filterOptimizableBean, string]("theString"),
					VariableRef[string]("MYVAR"))).
				Query(StatementName("s0")))
		}},
		{"s1", func() (Plan, error) {
			return env.Build(From[filterOptimizableBean](env, "SupportBean").
				Filter(RegexpMatch(
					Field[filterOptimizableBean, string]("theString"),
					ContextPatternField[string]("s0", "p00"))).
				Query(StatementName("s1"), WithContext("MyContext")))
		}},
		{"s2", func() (Plan, error) {
			s0Step := PatternFrom(From[reboolSupportBeanS0](env, "SupportBean_S0"), "s0", Literal[bool](true))
			sbStep := PatternFrom(From[filterOptimizableBean](env, "SupportBean"), "sb",
				RegexpMatch(
					Field[filterOptimizableBean, string]("theString"),
					TagField[string]("s0", "p00"))).Every()
			return env.Build(s0Step.Then(sbStep).
				Select(Alias("sb", PatternEvent("sb"))).
				Query(StatementName("s2")))
		}},
		{"s3", func() (Plan, error) {
			return env.Build(From[filterOptimizableBean](env, "SupportBean").
				Filter(RegexpMatch(
					Field[filterOptimizableBean, string]("theString"),
					Concat(Literal(".*"), Literal("abc"), Literal(".*")))).
				Query(StatementName("s3")))
		}},
	}

	var fireds []func() bool
	for _, def := range planDefs {
		plan, err := def.build()
		if err != nil {
			t.Fatalf("build %s: %v", def.name, err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatalf("deploy %s: %v", def.name, err)
		}
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		fireds = append(fireds, subscribeRebool(t, deployment, def.name))
	}

	if err := engine.SendEvent(context.Background(), reboolSupportBeanS0{ID: 1, P00: ".*abc.*"}); err != nil {
		t.Fatal(err)
	}
	for i, fired := range fireds {
		if fired() {
			t.Fatalf("stmt%d: context start / first pattern leg must not deliver", i)
		}
	}
	for _, tc := range []struct {
		theString string
		want      bool
	}{
		{"xabsx", false},
		{"xabcx", true},
	} {
		sendOptimizableBean(t, engine, filterOptimizableBean{TheString: tc.theString, IntPrimitive: 0})
		for i, fired := range fireds {
			if got := fired(); got != tc.want {
				t.Fatalf("stmt%d theString=%q: fired=%v want %v", i, tc.theString, got, tc.want)
			}
		}
	}
}
