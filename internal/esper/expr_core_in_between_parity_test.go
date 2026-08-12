package esper

import (
	"context"
	"testing"
)

// inBetweenBean mirrors SupportBean with nullable fields for IN/Between tests.
type inBetweenBean struct {
	TheString       string   `esper:"theString"`
	IntPrimitive    int      `esper:"intPrimitive"`
	IntBoxed        *int     `esper:"intBoxed"`
	DoubleBoxed     float64  `esper:"doubleBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
}

func inBetweenEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[inBetweenBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

func sendInBetween(t *testing.T, engine *Engine, bean inBetweenBean) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), bean); err != nil {
		t.Fatal(err)
	}
}

func subscribeBoolPtr(dep *Deployment, field string, collect *[]*bool) {
	_, _ = dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, r := range batch.New {
			v, err := As[bool](r.Get(field))
			if err != nil {
				*collect = append(*collect, nil)
			} else {
				*collect = append(*collect, &v)
			}
		}
		return nil
	})
}

// ExprCoreInNumeric (ordinal 0). Java:
//
//	doubleBoxed in (1.1d, 7/3.5, 2*6/3, 0)
//
// Tests numeric IN with arithmetic-expression candidates and null propagation.
func TestExprCoreInNumericParity(t *testing.T) {
	env, engine := inBetweenEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	input := From[inBetweenBean](env, "SupportBean")
	// 7/3.5 = 2.0, 2*6/3 = 4.0
	expr := InOf(
		Field[inBetweenBean, float64]("doubleBoxed"),
		Literal(1.1),
		Divide[float64](Literal(7.0), Literal(3.5)),
		Multiply[float64](Literal(2.0), Divide[float64](Literal(6.0), Literal(3.0))),
		Literal(0.0),
	)
	plan, err := env.Build(Select(input, Alias("result", expr)).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var results []*bool
	subscribeBoolPtr(dep, "result", &results)
	// input: 1d(null=false), null, 1.1d(true), 1.0d(false), 1.0999999999(false), 2d(true), 4d(true)
	cases := []struct {
		val  float64
		want *bool
	}{
		{1.0, boolPtr(false)},
		{1.1, boolPtr(true)},
		{1.0, boolPtr(false)},
		{1.0999999999, boolPtr(false)},
		{2.0, boolPtr(true)},
		{4.0, boolPtr(true)},
	}
	for _, tc := range cases {
		sendInBetween(t, engine, inBetweenBean{DoubleBoxed: tc.val})
	}
	if len(results) != len(cases) {
		t.Fatalf("InNumeric: result count = %d, want %d", len(results), len(cases))
	}
	for i, tc := range cases {
		got, want := results[i], tc.want
		if (got == nil) != (want == nil) {
			t.Fatalf("InNumeric[%d]: got %v, want %v", i, got, want)
		}
		if got != nil && *got != *want {
			t.Fatalf("InNumeric[%d]: got %v, want %v", i, *got, *want)
		}
	}
}

// ExprCoreInStringExpr (ordinal 11). Java:
//
//	theString in ('a', 'b', 'c')
//	theString in ('a', null)
//	theString not in ('a', 'b', 'c')
func TestExprCoreInStringExprParity(t *testing.T) {
	env, engine := inBetweenEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	input := From[inBetweenBean](env, "SupportBean")
	expr := InOf(
		Field[inBetweenBean, string]("theString"),
		Literal("a"), Literal("b"), Literal("c"),
	)
	plan, err := env.Build(Select(input, Alias("result", expr)).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var results []*bool
	subscribeBoolPtr(dep, "result", &results)
	cases := []struct {
		val  string
		want *bool
	}{
		{"0", boolPtr(false)},
		{"a", boolPtr(true)},
		{"b", boolPtr(true)},
		{"c", boolPtr(true)},
		{"d", boolPtr(false)},
	}
	for _, tc := range cases {
		sendInBetween(t, engine, inBetweenBean{TheString: tc.val})
	}
	for i, tc := range cases {
		got, want := results[i], tc.want
		if (got == nil) != (want == nil) || (got != nil && *got != *want) {
			t.Fatalf("InString[%d]: got %v, want %v (all: %v)", i, got, want, results)
		}
	}
}

// ExprCoreBetweenNumericExpr (ordinal 14). Java:
//
//	doubleBoxed between 1.1 and 15
//	doubleBoxed not between 1.1 and 15
//
// Null value or null bound yields false.
func TestExprCoreBetweenNumericExprParity(t *testing.T) {
	env, engine := inBetweenEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	input := From[inBetweenBean](env, "SupportBean")
	plan, err := env.Build(Select(input,
		Alias("btwn", BetweenOf(
			Field[inBetweenBean, float64]("doubleBoxed"),
			Literal(1.1), Literal(15.0),
		)),
		Alias("notBtwn", NotBetweenOf(
			Field[inBetweenBean, float64]("doubleBoxed"),
			Literal(1.1), Literal(15.0),
		)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	type pair struct{ btwn, notBtwn bool }
	var results []pair
	_, _ = dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, r := range batch.New {
			bv, _ := As[bool](r.Get("btwn"))
			nv, _ := As[bool](r.Get("notBtwn"))
			results = append(results, pair{bv, nv})
		}
		return nil
	})
	cases := []struct {
		val         float64
		wantBtwn    bool
		wantNotBtwn bool
	}{
		{1.0, false, true},
		{1.1, true, false},
		{2.0, true, false},
		{4.0, true, false},
		{15.0, true, false},
		{15.00001, false, true},
	}
	for _, tc := range cases {
		sendInBetween(t, engine, inBetweenBean{DoubleBoxed: tc.val})
	}
	if len(results) != len(cases) {
		t.Fatalf("BetweenNumeric: result count = %d, want %d", len(results), len(cases))
	}
	for i, tc := range cases {
		if results[i].btwn != tc.wantBtwn || results[i].notBtwn != tc.wantNotBtwn {
			t.Fatalf("BetweenNumeric[%d]: got %v, want btwn=%v notBtwn=%v", i, results[i], tc.wantBtwn, tc.wantNotBtwn)
		}
	}
}

// ExprCoreBetweenStringExpr (ordinal 13). Java:
//
//	theString between 'a0' and 'b9'
//	theString not between 'a0' and 'b9'
func TestExprCoreBetweenStringExprParity(t *testing.T) {
	env, engine := inBetweenEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	input := From[inBetweenBean](env, "SupportBean")
	plan, err := env.Build(Select(input,
		Alias("btwn", BetweenOf(
			Field[inBetweenBean, string]("theString"),
			Literal("a0"), Literal("b9"),
		)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var results []bool
	_, _ = dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, r := range batch.New {
			v, _ := As[bool](r.Get("btwn"))
			results = append(results, v)
		}
		return nil
	})
	cases := []struct {
		val  string
		want bool
	}{
		{"0", false},
		{"a1", true},
		{"a10", true},
		{"c", false},
		{"d", false},
		{"a0", true},
		{"b9", true},
		{"b90", false},
	}
	for _, tc := range cases {
		sendInBetween(t, engine, inBetweenBean{TheString: tc.val})
	}
	for i, tc := range cases {
		if results[i] != tc.want {
			t.Fatalf("BetweenString[%d]: val=%q got %v, want %v", i, tc.val, results[i], tc.want)
		}
	}
}

// ExprCoreInRange (ordinal 19). Java:
//
//	intPrimitive in [2:4]   -- closed range
//	intPrimitive in (2:4)   -- open range
//	intPrimitive in [2:4)   -- half-open
//	intPrimitive in (2:4]   -- half-closed
func TestExprCoreInRangeParity(t *testing.T) {
	env, engine := inBetweenEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	input := From[inBetweenBean](env, "SupportBean")
	plan, err := env.Build(Select(input,
		Alias("ro", BetweenRangeOf(Field[inBetweenBean, int]("intPrimitive"), Literal(2), Literal(4), false, false)),
		Alias("rc", BetweenRangeOf(Field[inBetweenBean, int]("intPrimitive"), Literal(2), Literal(4), true, true)),
		Alias("rho", BetweenRangeOf(Field[inBetweenBean, int]("intPrimitive"), Literal(2), Literal(4), true, false)),
		Alias("rhc", BetweenRangeOf(Field[inBetweenBean, int]("intPrimitive"), Literal(2), Literal(4), false, true)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	type quad struct{ ro, rc, rho, rhc bool }
	var results []quad
	_, _ = dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, r := range batch.New {
			ro, _ := As[bool](r.Get("ro"))
			rc, _ := As[bool](r.Get("rc"))
			rho, _ := As[bool](r.Get("rho"))
			rhc, _ := As[bool](r.Get("rhc"))
			results = append(results, quad{ro, rc, rho, rhc})
		}
		return nil
	})
	cases := []struct {
		val  int
		want quad
	}{
		{1, quad{false, false, false, false}},
		{2, quad{false, true, true, false}},
		{3, quad{true, true, true, true}},
		{4, quad{false, true, false, true}},
		{5, quad{false, false, false, false}},
	}
	for _, tc := range cases {
		sendInBetween(t, engine, inBetweenBean{IntPrimitive: tc.val})
	}
	for i, tc := range cases {
		if results[i] != tc.want {
			t.Fatalf("InRange[%d]: val=%d got %v, want %v", i, tc.val, results[i], tc.want)
		}
	}
}

func boolPtr(v bool) *bool { return &v }
