package esper

import (
	"context"
	"testing"
)

type filterInBetweenBean struct {
	TheString    string  `esper:"theString"`
	IntPrimitive int     `esper:"intPrimitive"`
	IntBoxed     *int    `esper:"intBoxed"`
	LongBoxed    *int64  `esper:"longBoxed"`
	BoolPrimitive bool   `esper:"boolPrimitive"`
}

func newInBetweenEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[filterInBetweenBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func sendInBetweenBean(t *testing.T, engine *Engine, theString string, intPrim int, intBoxed *int, longBoxed *int64, boolPrim bool) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), filterInBetweenBean{
		TheString: theString, IntPrimitive: intPrim, IntBoxed: intBoxed, LongBoxed: longBoxed, BoolPrimitive: boolPrim,
	}); err != nil {
		t.Fatal(err)
	}
}

func int64Ptr(v int64) *int64 { return &v }

func subscribeInBetween(t *testing.T, deployment *Deployment, stmtName string) func() bool {
	t.Helper()
	invoked := false
	statement, ok := deployment.Statement(stmtName)
	if !ok {
		t.Fatalf("statement %s not found", stmtName)
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


// TestExprFilterInExprStringRelationalMatchesEsper covers string relational
// comparisons in the filter, matching ExprFilterInExpr's string comparison
// assertions (theString > 'b', < 'b', >= 'b', <= 'b').
func TestExprFilterInExprStringRelationalMatchesEsper(t *testing.T) {
	env := newInBetweenEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	values := []string{"a", "b", "c", "d"}
	cases := []struct {
		name string
		expr Expression[bool]
		want []bool
	}{
		{"gt", GreaterOf(Field[filterInBetweenBean, string]("theString"), Literal("b")), []bool{false, false, true, true}},
		{"lt", LessOf(Field[filterInBetweenBean, string]("theString"), Literal("b")), []bool{true, false, false, false}},
		{"ge", GreaterOrEqualOf(Field[filterInBetweenBean, string]("theString"), Literal("b")), []bool{false, true, true, true}},
		{"le", LessOrEqualOf(Field[filterInBetweenBean, string]("theString"), Literal("b")), []bool{true, true, false, false}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewEnvironment()
			if _, err := RegisterStruct[filterInBetweenBean](e, "SupportBean"); err != nil {
				t.Fatal(err)
			}
			eng := NewEngine(e)
			defer func() { _ = eng.Close(context.Background()) }()
			source := From[filterInBetweenBean](e, "SupportBean")
			plan, err := e.Build(source.Filter(tc.expr).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			dep, err := eng.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			wasInvoked := subscribeInBetween(t, dep, "s0")
			for i, v := range values {
				sendInBetweenBean(t, eng, v, 0, nil, nil, false)
				if got := wasInvoked(); got != tc.want[i] {
					t.Fatalf("theString=%s (%s) invoked=%v, want %v", v, tc.name, got, tc.want[i])
				}
			}
		})
	}
}

// TestExprFilterInExprStringRangeMatchesEsper covers string range expressions
// in the filter, matching ExprFilterInExpr's string range assertions:
// theString in ['b':'d'], in ('b':'d'], in ['b':'d'), in ('b':'d').
func TestExprFilterInExprStringRangeMatchesEsper(t *testing.T) {
	values := []string{"a", "b", "c", "d", "e"}
	cases := []struct {
		name string
		expr Expression[bool]
		want []bool
	}{
		{"inclusive", BetweenOf(Field[filterInBetweenBean, string]("theString"), Literal("b"), Literal("d")), []bool{false, true, true, true, false}},
		{"lower-excl", And(GreaterOf(Field[filterInBetweenBean, string]("theString"), Literal("b")), LessOrEqualOf(Field[filterInBetweenBean, string]("theString"), Literal("d"))), []bool{false, false, true, true, false}},
		{"upper-excl", And(GreaterOrEqualOf(Field[filterInBetweenBean, string]("theString"), Literal("b")), LessOf(Field[filterInBetweenBean, string]("theString"), Literal("d"))), []bool{false, true, true, false, false}},
		{"both-excl", And(GreaterOf(Field[filterInBetweenBean, string]("theString"), Literal("b")), LessOf(Field[filterInBetweenBean, string]("theString"), Literal("d"))), []bool{false, false, true, false, false}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewEnvironment()
			if _, err := RegisterStruct[filterInBetweenBean](e, "SupportBean"); err != nil {
				t.Fatal(err)
			}
			eng := NewEngine(e)
			defer func() { _ = eng.Close(context.Background()) }()
			source := From[filterInBetweenBean](e, "SupportBean")
			plan, err := e.Build(source.Filter(tc.expr).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			dep, err := eng.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			wasInvoked := subscribeInBetween(t, dep, "s0")
			for i, v := range values {
				sendInBetweenBean(t, eng, v, 0, nil, nil, false)
				if got := wasInvoked(); got != tc.want[i] {
					t.Fatalf("theString=%s (%s) invoked=%v, want %v", v, tc.name, got, tc.want[i])
				}
			}
		})
	}
}

// TestExprFilterInExprBoolInMatchesEsper covers boolPrimitive IN filters
// matching ExprFilterInExpr's boolean assertions.
func TestExprFilterInExprBoolInMatchesEsper(t *testing.T) {
	env := newInBetweenEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// boolPrimitive in (false) -> only false matches
	source := From[filterInBetweenBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		InOf(Field[filterInBetweenBean, bool]("boolPrimitive"), Literal(false)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeInBetween(t, dep, "s0")

	sendInBetweenBean(t, engine, "", 0, nil, nil, true)
	if wasInvoked() {
		t.Fatal("boolPrimitive=true not in (false) should not invoke")
	}
	sendInBetweenBean(t, engine, "", 0, nil, nil, false)
	if !wasInvoked() {
		t.Fatal("boolPrimitive=false in (false) should invoke")
	}
}

// TestExprFilterInExprIntInMatchesEsper covers int IN and Between expressions
// matching ExprFilterInExpr's int assertions.
func TestExprFilterInExprIntInMatchesEsper(t *testing.T) {
	values := []int{0, 1, 2, 3, 4, 5, 6}
	intValPtrs := make([]*int, len(values))
	for i, v := range values {
		intValPtrs[i] = intPtr(v)
	}

	cases := []struct {
		name string
		expr Expression[bool]
		want []bool
	}{
		{"in-set", InOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(4), Literal(6), Literal(1)), []bool{false, true, false, false, true, false, true}},
		{"in-single", InOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(3)), []bool{false, false, false, true, false, false, false}},
		{"between", BetweenOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(4), Literal(6)), []bool{false, false, false, false, true, true, true}},
		{"between-reversed", BetweenOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(2), Literal(1)), []bool{false, true, true, false, false, false, false}},
		{"between-reversed-neg", BetweenOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(4), Literal(-1)), []bool{true, true, true, true, true, false, false}},
		{"range-inclusive", BetweenOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(2), Literal(4)), []bool{false, false, true, true, true, false, false}},
		{"range-lower-excl", And(GreaterOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(2)), LessOrEqualOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(4))), []bool{false, false, false, true, true, false, false}},
		{"range-upper-excl", And(GreaterOrEqualOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(2)), LessOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(4))), []bool{false, false, true, true, false, false, false}},
		{"range-both-excl", And(GreaterOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(2)), LessOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(4))), []bool{false, false, false, true, false, false, false}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewEnvironment()
			if _, err := RegisterStruct[filterInBetweenBean](e, "SupportBean"); err != nil {
				t.Fatal(err)
			}
			eng := NewEngine(e)
			defer func() { _ = eng.Close(context.Background()) }()
			source := From[filterInBetweenBean](e, "SupportBean")
			plan, err := e.Build(source.Filter(tc.expr).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			dep, err := eng.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			wasInvoked := subscribeInBetween(t, dep, "s0")
			for i := range values {
				sendInBetweenBean(t, eng, "", 0, intValPtrs[i], nil, false)
				if got := wasInvoked(); got != tc.want[i] {
					t.Fatalf("intBoxed=%d (%s) invoked=%v, want %v", values[i], tc.name, got, tc.want[i])
				}
			}
		})
	}
}

// TestExprFilterInExprLongInMatchesEsper covers longBoxed IN expression
// matching ExprFilterInExpr's long assertion: longBoxed in (3).
func TestExprFilterInExprLongInMatchesEsper(t *testing.T) {
	env := newInBetweenEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterInBetweenBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		InOf(Field[filterInBetweenBean, *int64]("longBoxed"), Literal(int64(3))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeInBetween(t, dep, "s0")

	for _, tc := range []struct {
		value int64
		want  bool
	}{
		{0, false}, {1, false}, {2, false}, {3, true}, {4, false}, {5, false}, {6, false},
	} {
		sendInBetweenBean(t, engine, "", 0, nil, int64Ptr(tc.value), false)
		if got := wasInvoked(); got != tc.want {
			t.Fatalf("longBoxed=%d invoked=%v, want %v", tc.value, got, tc.want)
		}
	}
}

// TestExprFilterNotBetweenMatchesEsper covers NotBetween and NOT IN range
// expressions matching ExprFilterNotIn assertions.
func TestExprFilterNotBetweenMatchesEsper(t *testing.T) {
	values := []int{0, 1, 2, 3, 4, 5, 6}
	intValPtrs := make([]*int, len(values))
	for i, v := range values {
		intValPtrs[i] = intPtr(v)
	}

	cases := []struct {
		name string
		expr Expression[bool]
		want []bool
	}{
		{"not-between", NotBetweenOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(4), Literal(6)), []bool{true, true, true, true, false, false, false}},
		{"not-between-reversed", NotBetweenOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(2), Literal(1)), []bool{true, false, false, true, true, true, true}},
		{"not-between-reversed-neg", NotBetweenOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(4), Literal(-1)), []bool{false, false, false, false, false, true, true}},
		{"not-range-inclusive", Not(BetweenOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(2), Literal(4))), []bool{true, true, false, false, false, true, true}},
		{"not-in-set", NotInOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(4), Literal(6), Literal(1)), []bool{true, false, true, true, false, true, false}},
		{"not-in-single", NotInOf(Field[filterInBetweenBean, *int]("intBoxed"), Literal(3)), []bool{true, true, true, false, true, true, true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewEnvironment()
			if _, err := RegisterStruct[filterInBetweenBean](e, "SupportBean"); err != nil {
				t.Fatal(err)
			}
			eng := NewEngine(e)
			defer func() { _ = eng.Close(context.Background()) }()
			source := From[filterInBetweenBean](e, "SupportBean")
			plan, err := e.Build(source.Filter(tc.expr).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			dep, err := eng.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			wasInvoked := subscribeInBetween(t, dep, "s0")
			for i := range values {
				sendInBetweenBean(t, eng, "", 0, intValPtrs[i], nil, false)
				if got := wasInvoked(); got != tc.want[i] {
					t.Fatalf("intBoxed=%d (%s) invoked=%v, want %v", values[i], tc.name, got, tc.want[i])
				}
			}
		})
	}
}

// TestExprFilterSimpleIntInMatchesEsper covers ExprFilterSimpleIntAndEnumWrite:
// intPrimitive in (1, 10).
func TestExprFilterSimpleIntInMatchesEsper(t *testing.T) {
	env := newInBetweenEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterInBetweenBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		In[int](Field[filterInBetweenBean, int]("intPrimitive"), Literal(1), Literal(10)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeInBetween(t, dep, "s0")

	sendInBetweenBean(t, engine, "", 10, nil, nil, false)
	if !wasInvoked() {
		t.Fatal("intPrimitive=10 in (1,10) should invoke")
	}
	sendInBetweenBean(t, engine, "", 11, nil, nil, false)
	if wasInvoked() {
		t.Fatal("intPrimitive=11 not in (1,10) should not invoke")
	}
	sendInBetweenBean(t, engine, "", 1, nil, nil, false)
	if !wasInvoked() {
		t.Fatal("intPrimitive=1 in (1,10) should invoke")
	}
}

// TestExprFilterNotInStringMatchesEsper covers string NOT IN expressions
// matching ExprFilterNotIn string assertions.
func TestExprFilterNotInStringMatchesEsper(t *testing.T) {
	env := newInBetweenEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	values := []string{"a", "x", "b", "y"}
	want := []bool{false, true, false, true}

	source := From[filterInBetweenBean](env, "SupportBean")
	plan, err := env.Build(source.Filter(
		NotInOf(Field[filterInBetweenBean, string]("theString"), Literal("a"), Literal("b")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	wasInvoked := subscribeInBetween(t, dep, "s0")

	for i, v := range values {
		sendInBetweenBean(t, engine, v, 0, nil, nil, false)
		if got := wasInvoked(); got != want[i] {
			t.Fatalf("theString=%s invoked=%v, want %v", v, got, want[i])
		}
	}
}
