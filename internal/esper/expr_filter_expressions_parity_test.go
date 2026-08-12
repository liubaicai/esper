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

// TestExprFilterExprReversedMatchesEsper covers ExprFilterExprReversed:
// constant on left side of comparison: 5 = intBoxed.
func TestExprFilterExprReversedMatchesEsper(t *testing.T) {
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterTestBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		EqualOf(Literal(5), Field[filterTestBean, *int]("intBoxed")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeFilter(t, deployment)

	sendFilterBean(t, engine, "E", 0, intPtr(5), nil)
	if !wasInvoked() {
		t.Fatal("5 = intBoxed(5) should invoke")
	}
	sendFilterBean(t, engine, "E", 0, intPtr(6), nil)
	if wasInvoked() {
		t.Fatal("5 = intBoxed(6) should not invoke")
	}
}

// TestExprFilterNotEqualsOpMatchesEsper covers ExprFilterNotEqualsOp:
// theString != 'a' filter, null string does not match.
func TestExprFilterNotEqualsOpMatchesEsper(t *testing.T) {
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterTestBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		NotEqual[string](Field[filterTestBean, string]("theString"), Literal("a")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeFilter(t, deployment)

	sendFilterBean(t, engine, "a", 0, nil, nil)
	if wasInvoked() {
		t.Fatal("theString='a' should not match !='a'")
	}
	sendFilterBean(t, engine, "b", 0, nil, nil)
	if !wasInvoked() {
		t.Fatal("theString='b' should match !='a'")
	}
	sendFilterBean(t, engine, "a", 0, nil, nil)
	if wasInvoked() {
		t.Fatal("theString='a' should not match !='a'")
	}
}

// TestExprFilterWithEqualsSameCompareMatchesEsper covers
// ExprFilterWithEqualsSameCompare: cross-type and self-comparison equality,
// IN with field operands, NOT IN with mixed constants/fields, and range
// with field bound.
func TestExprFilterWithEqualsSameCompareMatchesEsper(t *testing.T) {
	// Test data: (intBoxed, doubleBoxed) pairs
	type td struct {
		intBox *int
		dblBox *float64
	}
	data1 := []td{{intPtr(1), floatPtr(1.0)}, {intPtr(1), floatPtr(10.0)}}

	cases1 := []struct {
		name string
		expr Expression[bool]
		want []bool
	}{
		{"int-eq-double", EqualOf(Field[filterTestBean, *int]("intBoxed"), Field[filterTestBean, *float64]("doubleBoxed")), []bool{true, false}},
		{"self-eq", And(EqualOf(Field[filterTestBean, *int]("intBoxed"), Field[filterTestBean, *int]("intBoxed")), EqualOf(Field[filterTestBean, *float64]("doubleBoxed"), Field[filterTestBean, *float64]("doubleBoxed"))), []bool{true, true}},
		{"double-eq-int", EqualOf(Field[filterTestBean, *float64]("doubleBoxed"), Field[filterTestBean, *int]("intBoxed")), []bool{true, false}},
		{"double-in-int", InOf(Field[filterTestBean, *float64]("doubleBoxed"), Field[filterTestBean, *int]("intBoxed")), []bool{true, false}},
		{"int-in-double", InOf(Field[filterTestBean, *int]("intBoxed"), Field[filterTestBean, *float64]("doubleBoxed")), []bool{true, false}},
	}

	for _, tc := range cases1 {
		t.Run(tc.name, func(t *testing.T) {
			e := newFilterTestEnv(t)
			eng := NewEngine(e)
			defer func() { _ = eng.Close(context.Background()) }()
			source := From[filterTestBean](e, "SupportBean")
			plan, err := e.Build(source.Filter(tc.expr).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			dep, err := eng.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			wasInvoked := subscribeFilter(t, dep)
			for i, d := range data1 {
				sendFilterBean(t, eng, "", 0, d.intBox, d.dblBox)
				if got := wasInvoked(); got != tc.want[i] {
					t.Fatalf("(%s) case %d invoked=%v, want %v", tc.name, i, got, tc.want[i])
				}
			}
		})
	}

	// doubleBoxed not in (10, intBoxed) with intBoxed=1, doubleBoxed={1,5,10}
	t.Run("double-not-in-mixed", func(t *testing.T) {
		e := newFilterTestEnv(t)
		eng := NewEngine(e)
		defer func() { _ = eng.Close(context.Background()) }()
		source := From[filterTestBean](e, "SupportBean")
		plan, err := e.Build(source.Filter(
			NotInOf(Field[filterTestBean, *float64]("doubleBoxed"), Literal(float64(10)), Field[filterTestBean, *int]("intBoxed")),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		dep, err := eng.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		wasInvoked := subscribeFilter(t, dep)
		want := []bool{false, true, false}
		dblVals := []float64{1.0, 5.0, 10.0}
		for i, dv := range dblVals {
			sendFilterBean(t, eng, "", 0, intPtr(1), floatPtr(dv))
			if got := wasInvoked(); got != want[i] {
				t.Fatalf("doubleBoxed=%g invoked=%v, want %v", dv, got, want[i])
			}
		}
	})

	// doubleBoxed in (intBoxed:20) with intBoxed={0,1,2}, doubleBoxed=1
	// range (intBoxed:20] means > intBoxed AND <= 20
	t.Run("double-in-field-range", func(t *testing.T) {
		e := newFilterTestEnv(t)
		eng := NewEngine(e)
		defer func() { _ = eng.Close(context.Background()) }()
		source := From[filterTestBean](e, "SupportBean")
		plan, err := e.Build(source.Filter(
			And(
				GreaterOf(Field[filterTestBean, *float64]("doubleBoxed"), Field[filterTestBean, *int]("intBoxed")),
				LessOrEqualOf(Field[filterTestBean, *float64]("doubleBoxed"), Literal(20)),
			),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		dep, err := eng.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		wasInvoked := subscribeFilter(t, dep)
		want := []bool{true, false, false}
		intVals := []int{0, 1, 2}
		for i, iv := range intVals {
			sendFilterBean(t, eng, "", 0, intPtr(iv), floatPtr(1.0))
			if got := wasInvoked(); got != want[i] {
				t.Fatalf("intBoxed=%d doubleBoxed=1 invoked=%v, want %v", iv, got, want[i])
			}
		}
	})
}

// ---- ExprFilterOverInClause ----

type filterTradeEventBean struct {
	ID     int64  `esper:"id"`
	UserID string `esper:"userId"`
	Amount int64  `esper:"amount"`
}

// TestExprFilterOverInClauseMatchesEsper covers ExprFilterOverInClause:
// a pattern filter with IN and relational predicates fires only for matching
// events.
func TestExprFilterOverInClauseMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[filterTradeEventBean](env, "SupportTradeEvent"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	stream := From[filterTradeEventBean](env, "SupportTradeEvent")
	pattern := PatternFrom(stream, "event1", And(
		InOf(Field[filterTradeEventBean, string]("userId"), Literal("100"), Literal("101")),
		GreaterOrEqualOf(Field[filterTradeEventBean, int64]("amount"), Literal(int64(1000))),
	)).Every()
	plan, err := env.Build(pattern.
		Select(Alias("event1_id", TagField[int64]("event1", "id"))).
		Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	invoked := subscribeFilter(t, dep)

	if err := engine.SendEvent(context.Background(), filterTradeEventBean{ID: 1, UserID: "100", Amount: 1001}); err != nil {
		t.Fatal(err)
	}
	if !invoked() {
		t.Fatal("userId=100 amount=1001 should fire")
	}
	if err := engine.SendEvent(context.Background(), filterTradeEventBean{ID: 2, UserID: "102", Amount: 1001}); err != nil {
		t.Fatal(err)
	}
	if invoked() {
		t.Fatal("userId=102 should not fire")
	}
}

// ---- ExprFilterStaticFunc ----

// TestExprFilterStaticFuncMatchesEsper covers ExprFilterStaticFunc: a filter
// with a UDF (isStringEquals) combined with equality predicates.
func TestExprFilterStaticFuncMatchesEsper(t *testing.T) {
	cases := []struct {
		name string
		expr Expression[bool]
		want []bool
	}{
		{"udf-only", Func2[string, string, bool](
			"isStringEquals",
			func(prefix, value string) bool { return value == prefix },
			Literal("b"),
			Field[filterTestBean, string]("theString"),
		), []bool{false, true, false}},
		{"eq+udf-concat", And(
			Equal[string](Literal("b"), Field[filterTestBean, string]("theString")),
			Func2[string, string, bool]("isStringEquals",
				func(prefix, value string) bool { return value == prefix },
				Literal("bx"),
				Concat(Field[filterTestBean, string]("theString"), Literal("x"))),
		), []bool{false, true, false}},
		{"ne-consolidate", And(
			NotEqual[string](Field[filterTestBean, string]("theString"), Literal("a")),
			NotEqual[string](Field[filterTestBean, string]("theString"), Literal("c")),
		), []bool{false, true, false}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newFilterTestEnv(t)
			eng := NewEngine(e)
			defer func() { _ = eng.Close(context.Background()) }()
			plan, err := e.Build(From[filterTestBean](e, "SupportBean").
				Filter(tc.expr).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			d, err := eng.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			inv := subscribeFilter(t, d)
			for i, s := range []string{"a", "b", "c"} {
				sendFilterBean(t, eng, s, 0, nil, nil)
				if got := inv(); got != tc.want[i] {
					t.Fatalf("theString=%s invoked=%v, want %v", s, got, tc.want[i])
				}
			}
		})
	}
}

// ---- ExprFilterInstanceMethodWWildcard ----

type filterInstanceMethodBean struct {
	X int `esper:"x"`
}

func (b filterInstanceMethodBean) MyInstanceMethodAlwaysTrue() bool { return true }
func (b filterInstanceMethodBean) MyInstanceMethodEventBean(propertyName string, expected int) bool {
	return b.X == expected
}

// TestExprFilterInstanceMethodMatchesEsper covers
// ExprFilterInstanceMethodWWildcard: instance methods on the current event
// used as filter predicates.
func TestExprFilterInstanceMethodMatchesEsper(t *testing.T) {
	cases := []struct {
		name string
		expr Expression[bool]
		want []bool
	}{
		{"always-true", Method[bool](
			EventValue[filterInstanceMethodBean](),
			"MyInstanceMethodAlwaysTrue",
		), []bool{true, true, true}},
		{"event-bean-x-eq-1", Method[bool](
			EventValue[filterInstanceMethodBean](),
			"MyInstanceMethodEventBean",
			Literal("x"), Literal(1),
		), []bool{false, true, false}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewEnvironment()
			if _, err := RegisterStruct[filterInstanceMethodBean](e, "SupportInstanceMethodBean"); err != nil {
				t.Fatal(err)
			}
			eng := NewEngine(e)
			defer func() { _ = eng.Close(context.Background()) }()
			plan, err := e.Build(From[filterInstanceMethodBean](e, "SupportInstanceMethodBean").
				Filter(tc.expr).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			d, err := eng.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			inv := subscribeFilter(t, d)
			for i := 0; i < 3; i++ {
				if err := eng.SendEvent(context.Background(), filterInstanceMethodBean{X: i}); err != nil {
					t.Fatal(err)
				}
				if got := inv(); got != tc.want[i] {
					t.Fatalf("x=%d invoked=%v, want %v", i, got, tc.want[i])
				}
			}
		})
	}
}

// ---- ExprFilterNotEqualsConsolidate ----

// TestExprFilterNotEqualsConsolidateMatchesEsper covers
// ExprFilterNotEqualsConsolidate: two equivalent forms of not-in/not-equals
// produce the same match set for values 0..4.
func TestExprFilterNotEqualsConsolidateMatchesEsper(t *testing.T) {
	forms := []struct {
		name string
		expr Expression[bool]
	}{
		{"not-in", NotInOf(Field[filterTestBean, int]("intPrimitive"), Literal(1), Literal(2))},
		{"neq-and-neq", And(
			NotEqual[int](Field[filterTestBean, int]("intPrimitive"), Literal(1)),
			NotEqual[int](Field[filterTestBean, int]("intPrimitive"), Literal(2)),
		)},
	}
	want := []bool{true, false, false, true, true}

	for _, form := range forms {
		t.Run(form.name, func(t *testing.T) {
			e := newFilterTestEnv(t)
			eng := NewEngine(e)
			defer func() { _ = eng.Close(context.Background()) }()
			plan, err := e.Build(From[filterTestBean](e, "SupportBean").
				Filter(form.expr).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			d, err := eng.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			inv := subscribeFilter(t, d)
			for i := 0; i < 5; i++ {
				sendFilterBean(t, eng, "", i, nil, nil)
				if got := inv(); got != want[i] {
					t.Fatalf("intPrimitive=%d invoked=%v, want %v", i, got, want[i])
				}
			}
		})
	}
}
