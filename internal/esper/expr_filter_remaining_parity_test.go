package esper

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

type exprFilterPromoteBean struct {
	TheString   string   `esper:"theString"`
	DoubleBoxed *float64 `esper:"doubleBoxed"`
	LongBoxed   *int64   `esper:"longBoxed"`
}

func exprFilterPromoteFloat(value float64) *float64 { return &value }
func exprFilterPromoteLong(value int64) *int64      { return &value }

// TestExprFilterPromoteIndexToSetNotInMatchesEsper covers the two filters in
// ExprFilterPromoteIndexToSetNotIn. The index-promotion implementation is an
// engine optimization; the observable contract is the conjunction of two
// not-equal predicates plus a non-null boxed value.
func TestExprFilterPromoteIndexToSetNotInMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprFilterPromoteBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	source := From[exprFilterPromoteBean](env, "SupportBean")
	s0, err := env.Build(source.Filter(And(
		And(
			NotEqual[string](Field[exprFilterPromoteBean, string]("theString"), Literal("x")),
			NotEqual[string](Field[exprFilterPromoteBean, string]("theString"), Literal("y")),
		),
		IsNot(exprFilterPromoteField[*float64]("doubleBoxed"), NullLiteral[*float64]()),
	)).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	s1, err := env.Build(source.Filter(And(
		And(
			NotEqual[string](Field[exprFilterPromoteBean, string]("theString"), Literal("x")),
			NotEqual[string](Field[exprFilterPromoteBean, string]("theString"), Literal("y")),
		),
		IsNot(exprFilterPromoteField[*int64]("longBoxed"), NullLiteral[*int64]()),
	)).Query(StatementName("s1")))
	if err != nil {
		t.Fatal(err)
	}
	d0, err := engine.Deploy(context.Background(), s0)
	if err != nil {
		t.Fatal(err)
	}
	d1, err := engine.Deploy(context.Background(), s1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d0.Undeploy(context.Background()); _ = d1.Undeploy(context.Background()) }()
	invoked := make([]bool, 2)
	for index, deployment := range []*Deployment{d0, d1} {
		index := index
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			invoked[index] = len(batch.New) > 0
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	send := func(event exprFilterPromoteBean, want0, want1 bool) {
		t.Helper()
		invoked[0], invoked[1] = false, false
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		if invoked[0] != want0 || invoked[1] != want1 {
			t.Fatalf("event %#v invoked=(%v,%v), want=(%v,%v)", event, invoked[0], invoked[1], want0, want1)
		}
	}
	send(exprFilterPromoteBean{TheString: "E1", DoubleBoxed: exprFilterPromoteFloat(1), LongBoxed: exprFilterPromoteLong(1)}, true, true)
	send(exprFilterPromoteBean{TheString: "x", DoubleBoxed: exprFilterPromoteFloat(1), LongBoxed: exprFilterPromoteLong(1)}, false, false)
	send(exprFilterPromoteBean{TheString: "y", DoubleBoxed: exprFilterPromoteFloat(1), LongBoxed: exprFilterPromoteLong(1)}, false, false)
	send(exprFilterPromoteBean{TheString: "E2", LongBoxed: exprFilterPromoteLong(1)}, false, true)
	send(exprFilterPromoteBean{TheString: "E3", DoubleBoxed: exprFilterPromoteFloat(1)}, true, false)
	send(exprFilterPromoteBean{TheString: "E4"}, false, false)
}

func exprFilterPromoteField[T any](name string) Expression[T] {
	return Field[exprFilterPromoteBean, T](name)
}

var exprFilterShortCircuitProperty1Calls int64

type exprFilterShortCircuitBean struct{}

func (exprFilterShortCircuitBean) GetProperty1() string {
	atomic.AddInt64(&exprFilterShortCircuitProperty1Calls, 1)
	panic("property1 must be short-circuited")
}

func (exprFilterShortCircuitBean) GetProperty2() string { return "2" }

// TestExprFilterShortCircuitEvalAndOverspecifiedMatchesEsper verifies both
// short-circuit behavior and the ordinary impossible conjunction. Property1
// deliberately panics when invoked; the false property2 branch must prevent
// that getter from being evaluated.
func TestExprFilterShortCircuitEvalAndOverspecifiedMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprFilterShortCircuitBean](env, "SupportRuntimeExBean", WithAccessorStyle(AccessorJavaBean)); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[filterTestBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	runtimeSource := From[exprFilterShortCircuitBean](env, "SupportRuntimeExBean")
	runtimePlan, err := env.Build(runtimeSource.Filter(And(
		Equal[string](Field[exprFilterShortCircuitBean, string]("property2"), Literal("4")),
		Equal[string](Field[exprFilterShortCircuitBean, string]("property1"), Literal("1")),
	)).Query(StatementName("runtime")))
	if err != nil {
		t.Fatal(err)
	}
	runtimeDeployment, err := engine.Deploy(context.Background(), runtimePlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = runtimeDeployment.Undeploy(context.Background()) }()
	runtimeMatches := false
	if _, err := runtimeDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		runtimeMatches = len(batch.New) > 0
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	atomic.StoreInt64(&exprFilterShortCircuitProperty1Calls, 0)
	if err := engine.SendEvent(context.Background(), exprFilterShortCircuitBean{}); err != nil {
		t.Fatal(err)
	}
	if runtimeMatches || atomic.LoadInt64(&exprFilterShortCircuitProperty1Calls) != 0 {
		t.Fatalf("short-circuit result matches=%v property1Calls=%d, want false/0", runtimeMatches, atomic.LoadInt64(&exprFilterShortCircuitProperty1Calls))
	}
	if err := runtimeDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}

	supportSource := From[filterTestBean](env, "SupportBean")
	supportPlan, err := env.Build(supportSource.Filter(And(
		Equal[string](Field[filterTestBean, string]("theString"), Literal("A")),
		Equal[string](Field[filterTestBean, string]("theString"), Literal("B")),
	)).Query(StatementName("impossible")))
	if err != nil {
		t.Fatal(err)
	}
	supportDeployment, err := engine.Deploy(context.Background(), supportPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = supportDeployment.Undeploy(context.Background()) }()
	supportMatches := false
	if _, err := supportDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		supportMatches = len(batch.New) > 0
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendFilterBean(t, engine, "A", 0, nil, nil)
	if supportMatches {
		t.Fatal("theString='A' and theString='B' must not match")
	}
}

type exprFilterRewriteBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestExprFilterRewriteWhereMatchesEsper covers the ordinary WHERE-to-filter
// rewrite, the typed disable-rewrite hint and the named-window consumer form.
func TestExprFilterRewriteWhereMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[exprFilterRewriteBean](env, "SupportBean")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	buildAndCheck := func(name string, options ...QueryOption) {
		t.Helper()
		plan, err := env.Build(From[exprFilterRewriteBean](env, "SupportBean").Filter(
			Equal[int](Field[exprFilterRewriteBean, int]("intPrimitive"), Literal(3)),
		).Query(append([]QueryOption{StatementName(name)}, options...)...))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		matches := 0
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			matches += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		for _, event := range []exprFilterRewriteBean{{TheString: "E1", IntPrimitive: 3}, {TheString: "E2", IntPrimitive: 4}} {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		if matches != 1 {
			t.Fatalf("%s matches=%d, want 1", name, matches)
		}
	}
	buildAndCheck("normal")
	disable, err := NewStatementHint(HintDisableWhereMoveToFilter)
	if err != nil {
		t.Fatal(err)
	}
	buildAndCheck("disabled", WithStatementHints(disable))

	if _, err := env.RegisterNamedWindow("NamedWindowA", schema, NamedWindowRetention(LengthWindow(1))); err != nil {
		t.Fatal(err)
	}
	trimmed := Func1[string, string]("trim", strings.TrimSpace, Field[exprFilterRewriteBean, string]("theString"))
	namedPlan, err := env.Build(FromNamedWindowAs[exprFilterRewriteBean](env, "NamedWindowA").Filter(
		Equal[string](trimmed, Literal("abc")),
	).Query(StatementName("named-window")))
	if err != nil {
		t.Fatal(err)
	}
	namedDeployment, err := engine.Deploy(context.Background(), namedPlan)
	if err != nil {
		t.Fatal(err)
	}
	if err := namedDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

type exprFilterInvalidBean struct {
	TheString    string   `esper:"theString"`
	IntPrimitive int      `esper:"intPrimitive"`
	LongBoxed    *int64   `esper:"longBoxed"`
	DoubleBoxed  *float64 `esper:"doubleBoxed"`
}

type exprFilterInvalidMarketData struct {
	Volume int64 `esper:"volume"`
}

func assertExprFilterInvalidBuild(t *testing.T, name string, query Query, fragments ...string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		_, err := query.env.Build(query)
		if err == nil {
			t.Fatalf("invalid ExprFilter %s was accepted", name)
		}
		if !errors.Is(err, ErrorInvalidRule) && !errors.Is(err, ErrorTypeMismatch) && !errors.Is(err, ErrorUnknownName) {
			t.Fatalf("invalid ExprFilter %s error=%v, want validation error", name, err)
		}
		for _, fragment := range fragments {
			if !strings.Contains(err.Error(), fragment) {
				t.Fatalf("invalid ExprFilter %s error=%v, want %q", name, err, fragment)
			}
		}
	})
}

// TestExprFilterInvalidMatchesEsper covers every ExprFilterInvalid case that
// has a typed fluent representation. EPL-only non-boolean syntax and Java
// enum class-literal diagnostics are compile-time/type-system differences.
func TestExprFilterInvalidMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprFilterInvalidBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprFilterInvalidMarketData](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprFilterEnumBean](env, "SupportBeanWithEnum"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	source := From[exprFilterInvalidBean](env, "SupportBean")
	market := From[exprFilterInvalidMarketData](env, "SupportMarketDataBean")
	longTag := Cast[*int64, int64](TagField[*int64]("a", "longBoxed"))
	aggregate := Sum[int64](longTag)
	assertExprFilterInvalidBuild(t, "aggregate-in-pattern-filter", PatternFrom(source, "a", Literal(true)).Every().Then(
		PatternFrom(market, "b", EqualOf(aggregate, Literal(int64(2)))),
	).Select(Alias("matched", Literal(true))).Query(), "aggregation functions not allowed within filters")
	assertExprFilterInvalidBuild(t, "prior-in-pattern-filter", PatternFrom(source, "a", Prior[bool](1, Literal(true))).
		Select(Alias("matched", Literal(true))).Query(), "previous or prior functions cannot be used")
	assertExprFilterInvalidBuild(t, "prev-in-pattern-filter", PatternFrom(source, "a", Prev[bool](1, Literal(true))).
		Select(Alias("matched", Literal(true))).Query(), "previous or prior functions cannot be used")
	assertExprFilterInvalidBuild(t, "enum-string-coercion", From[exprFilterEnumBean](env, "SupportBeanWithEnum").Filter(
		EqualOf(Field[exprFilterEnumBean, string]("theString"), Literal(exprFilterEnumValueOne)),
	).Query(), "equality operands are not compatible")
	assertExprFilterInvalidBuild(t, "enum-unknown-symbol", From[exprFilterEnumBean](env, "SupportBeanWithEnum").Filter(
		Equal[exprFilterEnumValue](Field[exprFilterEnumBean, exprFilterEnumValue]("A.b"), Literal(exprFilterEnumValueOne)),
	).Query(), "unknown field")
	assertExprFilterInvalidBuild(t, "unknown-pattern-tag", PatternFrom(source, "a", Literal(true)).Then(
		PatternFrom(source, "b", EqualOf(Field[exprFilterInvalidBean, int]("intPrimitive"), TagField[int]("x", "intPrimitive"))),
	).Select(Alias("matched", Literal(true))).Query(), "unknown tag")
	assertExprFilterInvalidBuild(t, "unknown-pattern-property", PatternFrom(source, "a", Literal(true)).Then(
		PatternFrom(source, "b", Equal[int](Field[exprFilterInvalidBean, int]("cluedo.intPrimitive"), Literal(1))),
	).Select(Alias("matched", Literal(true))).Query(), "unknown field")

	// Filter(Literal(5)) is intentionally unrepresentable: the Go type
	// checker rejects a non-boolean expression before Build, matching the
	// fluent API's stronger compile-time contract.
	_ = engine
}
