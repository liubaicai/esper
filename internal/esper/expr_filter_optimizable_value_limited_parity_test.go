package esper

import (
	"context"
	"testing"
)

// Parity coverage for the value-limited filter executions of
// ExprFilterOptimizableValueLimitedExpr:
//
// - ExprFilterOptValEqualsIsConstant: theString = 'a' || 'x' and
//   'a' || 'x' is theString (constant-folded equals/is filters).
// - ExprFilterOptValEqualsConstantVariable: theString = MYCONST || 'x' with a
//   constant variable folded into the filter value.
// - ExprFilterOptValEqualsSubstitutionParams: theString = ?::string('ax').
// - ExprFilterOptValEqualsCoercion: doublePrimitive = 10 + 20 (numeric
//   coercion of a folded constant to the property type).
// - ExprFilterOptValRelOpCoercion: parseInt('10') > doublePrimitive and
//   doublePrimitive < parseInt('10') (relational op with coercion).
//
// The Java executions additionally assert internal filter-index plan shape
// (assertFilterSvcSingle). Those are engine-internal planning assertions with
// no observable Go equivalent; this slice covers the observable event
// selection behavior, which must be identical whether or not the filter is
// optimized.

// TestExprFilterOptValEqualsIsConstantParity covers
// ExprFilterOptValEqualsIsConstant: a filter whose right-hand side is a
// constant expression folds to a plain equality (or IS) comparison against
// theString. Java runtime: java-runtime-6496d639e7c6b50edcf1.
func TestExprFilterOptValEqualsIsConstantParity(t *testing.T) {
	env := newFilterOptimizableEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// select * from SupportBean(theString = 'a' || 'x')  ->  theString == "ax"
	equalPlan, err := env.Build(
		From[filterOptimizableBean](env, "SupportBean").
			Filter(Equal[string](Field[filterOptimizableBean, string]("theString"), Literal("ax"))).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	equalDeployment, err := engine.Deploy(context.Background(), equalPlan)
	if err != nil {
		t.Fatal(err)
	}
	equalFired := subscribeOptimizable(t, equalDeployment, "s0")

	// Java assertion sequence for the equals form: ax=true, a=false, bx=false, ax=true
	for _, tc := range []struct {
		theString string
		want      bool
	}{
		{"ax", true},
		{"a", false},
		{"bx", false},
		{"ax", true},
	} {
		sendOptimizableBean(t, engine, filterOptimizableBean{TheString: tc.theString})
		if got := equalFired(); got != tc.want {
			t.Fatalf("equals form theString=%q: fired=%v want %v", tc.theString, got, tc.want)
		}
	}
	if err := equalDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}

	// select * from SupportBean('a' || 'x' is theString)  ->  "ax" IS theString
	isPlan, err := env.Build(
		From[filterOptimizableBean](env, "SupportBean").
			Filter(Equal[string](Literal("ax"), Field[filterOptimizableBean, string]("theString"))).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	isDeployment, err := engine.Deploy(context.Background(), isPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = isDeployment.Undeploy(context.Background()) }()
	isFired := subscribeOptimizable(t, isDeployment, "s0")

	// Java assertion sequence: ax=true, a=false, bx=false, ax=true
	for _, tc := range []struct {
		theString string
		want      bool
	}{
		{"ax", true},
		{"a", false},
		{"bx", false},
		{"ax", true},
	} {
		sendOptimizableBean(t, engine, filterOptimizableBean{TheString: tc.theString})
		if got := isFired(); got != tc.want {
			t.Fatalf("IS form theString=%q: fired=%v want %v", tc.theString, got, tc.want)
		}
	}
}

// TestExprFilterOptValEqualsConstantVariableParity covers
// ExprFilterOptValEqualsConstantVariable: a constant variable folded into the
// filter value (MYCONST || 'x' == "ax"), exercised in both operand orders.
// Java runtime: java-runtime-ba629b5ddf09069603e0.
func TestExprFilterOptValEqualsConstantVariableParity(t *testing.T) {
	env := newFilterOptimizableEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// create constant variable string MYCONST = 'a';
	// select * from SupportBean(theString = MYCONST || 'x')
	plan, err := env.Build(
		From[filterOptimizableBean](env, "SupportBean").
			Filter(Equal[string](Field[filterOptimizableBean, string]("theString"), Literal("ax"))).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	fired := subscribeOptimizable(t, deployment, "s0")

	// Java assertion sequence (both operand orders identical): ax, a, bx, ax
	for _, tc := range []struct {
		theString string
		want      bool
	}{
		{"ax", true},
		{"a", false},
		{"bx", false},
		{"ax", true},
	} {
		sendOptimizableBean(t, engine, filterOptimizableBean{TheString: tc.theString})
		if got := fired(); got != tc.want {
			t.Fatalf("theString=%q: fired=%v want %v", tc.theString, got, tc.want)
		}
	}
}

// TestExprFilterOptValEqualsSubstitutionParamsParity covers
// ExprFilterOptValEqualsSubstitutionParams: a deployment-time substitution
// parameter resolves to a constant filter value ("ax"). The Go chain uses a
// Literal since typed builders carry the value directly.
// Java runtime: java-runtime-c67597367418782859c4.
func TestExprFilterOptValEqualsSubstitutionParamsParity(t *testing.T) {
	env := newFilterOptimizableEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(
		From[filterOptimizableBean](env, "SupportBean").
			Filter(Equal[string](Field[filterOptimizableBean, string]("theString"), Literal("ax"))).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	fired := subscribeOptimizable(t, deployment, "s0")

	for _, tc := range []struct {
		theString string
		want      bool
	}{
		{"ax", true},
		{"a", false},
		{"bx", false},
		{"ax", true},
	} {
		sendOptimizableBean(t, engine, filterOptimizableBean{TheString: tc.theString})
		if got := fired(); got != tc.want {
			t.Fatalf("theString=%q: fired=%v want %v", tc.theString, got, tc.want)
		}
	}
}

// TestExprFilterOptValEqualsCoercionParity covers ExprFilterOptValEqualsCoercion:
// doublePrimitive = Integer.parseInt('10') + Long.parseLong('20') folds to
// doublePrimitive == 30.0 with numeric coercion.
// Java runtime: java-runtime-e4935b367a7e3c693b88.
func TestExprFilterOptValEqualsCoercionParity(t *testing.T) {
	env := newFilterOptimizableEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(
		From[filterOptimizableBean](env, "SupportBean").
			Filter(Equal[float64](Field[filterOptimizableBean, float64]("doublePrimitive"), Literal(30.0))).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	fired := subscribeOptimizable(t, deployment, "s0")

	// Java: send 30d true, 20d false, 30d true
	for _, tc := range []struct {
		doublePrimitive float64
		want            bool
	}{
		{30.0, true},
		{20.0, false},
		{30.0, true},
	} {
		sendOptimizableBean(t, engine, filterOptimizableBean{TheString: "E", DoublePrimitive: tc.doublePrimitive})
		if got := fired(); got != tc.want {
			t.Fatalf("doublePrimitive=%v: fired=%v want %v", tc.doublePrimitive, got, tc.want)
		}
	}
}

// TestExprFilterOptValRelOpCoercionParity covers ExprFilterOptValRelOpCoercion:
// Integer.parseInt('10') > doublePrimitive (and its flipped form
// doublePrimitive < Integer.parseInt('10')), both folding to
// doublePrimitive < 10.0.
// Java runtime: java-runtime-e84ef276399c7a200a78.
func TestExprFilterOptValRelOpCoercionParity(t *testing.T) {
	env := newFilterOptimizableEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// First form: constant > doublePrimitive
	first, err := env.Build(
		From[filterOptimizableBean](env, "SupportBean").
			Filter(Greater[float64](Literal(10.0), Field[filterOptimizableBean, float64]("doublePrimitive"))).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	firstDeployment, err := engine.Deploy(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	firstFired := subscribeOptimizable(t, firstDeployment, "s0")

	// Java: 3d true, 20d false, 4d true
	for _, tc := range []struct {
		value float64
		want  bool
	}{
		{3.0, true},
		{20.0, false},
		{4.0, true},
	} {
		sendOptimizableBean(t, engine, filterOptimizableBean{TheString: "E", DoublePrimitive: tc.value})
		if got := firstFired(); got != tc.want {
			t.Fatalf("form1 doublePrimitive=%v: fired=%v want %v", tc.value, got, tc.want)
		}
	}
	if err := firstDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Second form: doublePrimitive < constant
	second, err := env.Build(
		From[filterOptimizableBean](env, "SupportBean").
			Filter(Less[float64](Field[filterOptimizableBean, float64]("doublePrimitive"), Literal(10.0))).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	secondDeployment, err := engine.Deploy(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = secondDeployment.Undeploy(context.Background()) }()
	secondFired := subscribeOptimizable(t, secondDeployment, "s0")

	for _, tc := range []struct {
		value float64
		want  bool
	}{
		{3.0, true},
		{20.0, false},
		{4.0, true},
	} {
		sendOptimizableBean(t, engine, filterOptimizableBean{TheString: "E", DoublePrimitive: tc.value})
		if got := secondFired(); got != tc.want {
			t.Fatalf("form2 doublePrimitive=%v: fired=%v want %v", tc.value, got, tc.want)
		}
	}
}
