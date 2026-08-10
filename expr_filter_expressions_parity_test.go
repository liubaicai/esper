package esper

import (
	"context"
	"testing"
)

type filterTestBean struct {
	TheString    string   `esper:"theString"`
	IntPrimitive int      `esper:"intPrimitive"`
	IntBoxed     *int     `esper:"intBoxed"`
	DoubleBoxed  *float64 `esper:"doubleBoxed"`
}

func newFilterTestEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[filterTestBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func sendFilterBean(t *testing.T, engine *Engine, theString string, intPrim int, intBoxed *int, doubleBoxed *float64) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), filterTestBean{
		TheString: theString, IntPrimitive: intPrim, IntBoxed: intBoxed, DoubleBoxed: doubleBoxed,
	}); err != nil {
		t.Fatal(err)
	}
}

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

func subscribeFilter(t *testing.T, deployment *Deployment) func() bool {
	t.Helper()
	invoked := false
	statement, ok := deployment.Statement("s0")
	if !ok {
		t.Fatal("statement s0 not found")
	}
	if _, err := statement.Subscribe(func(_ context.Context, _ ResultBatch) error {
		invoked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return func() bool {
		result := invoked
		invoked = false
		return result
	}
}

// TestExprFilterRelationalOpRangeMatchesEsper covers
// ExprFilterRelationalOpRange: between with inclusive/exclusive bounds.
func TestExprFilterRelationalOpRangeMatchesEsper(t *testing.T) {
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterTestBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		And(
			GreaterOrEqualOf(Field[filterTestBean, *int]("intBoxed"), Literal(2)),
			LessOrEqualOf(Field[filterTestBean, *int]("intBoxed"), Literal(3)),
		),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeFilter(t, deployment)

	tests := []struct {
		value int
		want  bool
	}{
		{1, false}, {2, true}, {3, true}, {4, false},
	}
	for _, tc := range tests {
		sendFilterBean(t, engine, "E", 0, intPtr(tc.value), floatPtr(0))
		if got := wasInvoked(); got != tc.want {
			t.Fatalf("intBoxed=%d invoked=%v, want %v", tc.value, got, tc.want)
		}
	}
}

// TestExprFilterMathExpressionMatchesEsper covers
// ExprFilterMathExpression: intBoxed*doubleBoxed > 20 in the filter.
func TestExprFilterMathExpressionMatchesEsper(t *testing.T) {
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterTestBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		GreaterOf(
			Multiply[float64](
				Cast[*int, float64](Field[filterTestBean, *int]("intBoxed")),
				Cast[*float64, float64](Field[filterTestBean, *float64]("doubleBoxed")),
			),
			Literal(float64(20)),
		),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeFilter(t, deployment)

	sendFilterBean(t, engine, "E", 0, intPtr(5), floatPtr(5.0))
	if !wasInvoked() {
		t.Fatal("5*5=25 > 20 should invoke")
	}
	sendFilterBean(t, engine, "E", 0, intPtr(5), floatPtr(4.0))
	if wasInvoked() {
		t.Fatal("5*4=20 not > 20 should not invoke")
	}
	sendFilterBean(t, engine, "E", 0, intPtr(5), floatPtr(4.001))
	if !wasInvoked() {
		t.Fatal("5*4.001=20.005 > 20 should invoke")
	}
}

// TestExprFilterBooleanExprMatchesEsper covers
// ExprFilterBooleanExpr: 2*intBoxed=doubleBoxed.
func TestExprFilterBooleanExprMatchesEsper(t *testing.T) {
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterTestBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		EqualOf(
			Cast[int, float64](Multiply[int](Literal(2), Cast[*int, int](Field[filterTestBean, *int]("intBoxed")))),
			Field[filterTestBean, *float64]("doubleBoxed"),
		),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeFilter(t, deployment)

	sendFilterBean(t, engine, "E", 0, intPtr(20), floatPtr(50.0))
	if wasInvoked() {
		t.Fatal("2*20=40 != 50 should not invoke")
	}
	sendFilterBean(t, engine, "E", 0, intPtr(25), floatPtr(50.0))
	if !wasInvoked() {
		t.Fatal("2*25=50 == 50 should invoke")
	}
}

// TestExprFilterInSetMatchesEsper covers ExprFilterInSet: intPrimitive in
// (1, 2, 3).
func TestExprFilterInSetMatchesEsper(t *testing.T) {
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterTestBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		In[int](Field[filterTestBean, int]("intPrimitive"),
			Literal(1), Literal(2), Literal(3)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeFilter(t, deployment)

	for _, v := range []int{1, 2, 3} {
		sendFilterBean(t, engine, "E", v, nil, nil)
		if !wasInvoked() {
			t.Fatalf("intPrimitive=%d should be in set", v)
		}
	}
	sendFilterBean(t, engine, "E", 4, nil, nil)
	if wasInvoked() {
		t.Fatal("intPrimitive=4 should not be in set")
	}
}

// TestExprFilterNotEqualsNullMatchesEsper covers ExprFilterNotEqualsNull.
func TestExprFilterNotEqualsNullMatchesEsper(t *testing.T) {
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterTestBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		NotEqual[string](Field[filterTestBean, string]("theString"), Literal("")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeFilter(t, deployment)

	sendFilterBean(t, engine, "abc", 1, nil, nil)
	if !wasInvoked() {
		t.Fatal("non-empty theString should invoke")
	}
	sendFilterBean(t, engine, "", 1, nil, nil)
	if wasInvoked() {
		t.Fatal("empty theString should not invoke")
	}
}

// TestExprFilterConstantMatchesEsper covers ExprFilterConstant.
func TestExprFilterConstantMatchesEsper(t *testing.T) {
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterTestBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(Literal(true)).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeFilter(t, deployment)

	sendFilterBean(t, engine, "E1", 1, nil, nil)
	if !wasInvoked() {
		t.Fatal("constant true should always invoke")
	}
	sendFilterBean(t, engine, "E2", 2, nil, nil)
	if !wasInvoked() {
		t.Fatal("constant true should always invoke again")
	}
}

// TestExprFilterRelationalOpConstantFirstMatchesEsper covers
// ExprFilterRelationalOpConstantFirst: 3 > intBoxed.
func TestExprFilterRelationalOpConstantFirstMatchesEsper(t *testing.T) {
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterTestBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		GreaterOf(Literal(3), Field[filterTestBean, *int]("intBoxed")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeFilter(t, deployment)

	sendFilterBean(t, engine, "E", 0, intPtr(2), nil)
	if !wasInvoked() {
		t.Fatal("3 > 2 should invoke")
	}
	sendFilterBean(t, engine, "E", 0, intPtr(5), nil)
	if wasInvoked() {
		t.Fatal("3 > 5 should not invoke")
	}
}

// TestExprFilterAndOrCombinationMatchesEsper covers combinations of AND/OR.
func TestExprFilterAndOrCombinationMatchesEsper(t *testing.T) {
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterTestBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		Or(
			Equal[int](Field[filterTestBean, int]("intPrimitive"), Literal(1)),
			Equal[int](Field[filterTestBean, int]("intPrimitive"), Literal(2)),
		),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeFilter(t, deployment)

	sendFilterBean(t, engine, "E", 1, nil, nil)
	if !wasInvoked() {
		t.Fatal("intPrimitive=1 should match OR")
	}
	sendFilterBean(t, engine, "E", 2, nil, nil)
	if !wasInvoked() {
		t.Fatal("intPrimitive=2 should match OR")
	}
	sendFilterBean(t, engine, "E", 3, nil, nil)
	if wasInvoked() {
		t.Fatal("intPrimitive=3 should not match OR")
	}
}

// TestExprFilterIn3ValuesAndNullMatchesEsper covers
// ExprFilterIn3ValuesAndNull: intPrimitive in (intBoxed, doubleBoxed).
func TestExprFilterIn3ValuesAndNullMatchesEsper(t *testing.T) {
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterTestBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		InOf(Field[filterTestBean, int]("intPrimitive"),
			Field[filterTestBean, *int]("intBoxed"),
			Field[filterTestBean, *float64]("doubleBoxed")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeFilter(t, deployment)

	sendFilterBean(t, engine, "E", 1, intPtr(0), floatPtr(2.0))
	if wasInvoked() {
		t.Fatal("1 not in (0, 2.0) should not invoke")
	}
	sendFilterBean(t, engine, "E", 1, intPtr(1), floatPtr(2.0))
	if !wasInvoked() {
		t.Fatal("1 in (1, 2.0) should invoke")
	}
	sendFilterBean(t, engine, "E", 1, intPtr(0), floatPtr(1.0))
	if !wasInvoked() {
		t.Fatal("1 in (0, 1.0) should invoke")
	}
}

// TestExprFilterNotInMatchesEsper covers ExprFilterNotIn.
func TestExprFilterNotInMatchesEsper(t *testing.T) {
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterTestBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		Not(In[int](Field[filterTestBean, int]("intPrimitive"),
			Literal(1), Literal(2))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeFilter(t, deployment)

	sendFilterBean(t, engine, "E", 1, nil, nil)
	if wasInvoked() {
		t.Fatal("intPrimitive=1 is in set, NOT should not invoke")
	}
	sendFilterBean(t, engine, "E", 3, nil, nil)
	if !wasInvoked() {
		t.Fatal("intPrimitive=3 is not in set, NOT should invoke")
	}
}

// TestExprFilterBetweenMatchesEsper covers Between expression directly.
func TestExprFilterBetweenMatchesEsper(t *testing.T) {
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterTestBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		BetweenOf(Field[filterTestBean, *int]("intBoxed"), Literal(2), Literal(4)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeFilter(t, deployment)

	for _, tc := range []struct {
		value int
		want  bool
	}{
		{1, false}, {2, true}, {3, true}, {4, true}, {5, false},
	} {
		sendFilterBean(t, engine, "E", 0, intPtr(tc.value), nil)
		if got := wasInvoked(); got != tc.want {
			t.Fatalf("Between(%d, 2, 4) invoked=%v, want %v", tc.value, got, tc.want)
		}
	}
}

// TestExprFilterEqualsSemanticMatchesEsper covers ExprFilterEqualsSemanticFilter.
func TestExprFilterEqualsSemanticMatchesEsper(t *testing.T) {
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterTestBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		Equal[string](Field[filterTestBean, string]("theString"), Literal("s")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeFilter(t, deployment)

	sendFilterBean(t, engine, "s", 1, nil, nil)
	if !wasInvoked() {
		t.Fatal("theString='s' should invoke")
	}
	sendFilterBean(t, engine, "x", 1, nil, nil)
	if wasInvoked() {
		t.Fatal("theString='x' should not invoke")
	}
}

// TestExprFilterNullBooleanExprMatchesEsper covers ExprFilterNullBooleanExpr.
func TestExprFilterNullBooleanExprMatchesEsper(t *testing.T) {
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterTestBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		GreaterOf(Field[filterTestBean, *int]("intBoxed"), Literal(5)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeFilter(t, deployment)

	sendFilterBean(t, engine, "E", 1, nil, nil)
	if wasInvoked() {
		t.Fatal("null intBoxed comparison should not invoke")
	}
	sendFilterBean(t, engine, "E", 1, intPtr(10), nil)
	if !wasInvoked() {
		t.Fatal("intBoxed=10 > 5 should invoke")
	}
}
