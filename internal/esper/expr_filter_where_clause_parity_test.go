package esper

import (
	"context"
	"testing"
)

type whereClauseBean struct {
	TheString       string  `esper:"theString"`
	IntPrimitive    int     `esper:"intPrimitive"`
	LongPrimitive   int64   `esper:"longPrimitive"`
	FloatPrimitive  float32 `esper:"floatPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
	Symbol          string  `esper:"symbol"`
}

func newWhereClauseEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[whereClauseBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[whereClauseBean](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	return env, engine
}

// TestExprFilterWhereClauseSimpleMatchesEsper covers ExprFilterWhereClauseSimple:
// select * from SupportMarketDataBean#length(3) where symbol='CSCO'.
// The WHERE clause is equivalent to a post-window Filter in Go.
func TestExprFilterWhereClauseSimpleMatchesEsper(t *testing.T) {
	env, engine := newWhereClauseEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[whereClauseBean](env, "SupportMarketDataBean")
	plan, err := env.Build(source.
		Window(LengthWindow(3)).
		Filter(Equal[string](Field[whereClauseBean, string]("symbol"), Literal("CSCO"))).
		Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}

	invoked := false
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, _ ResultBatch) error {
		invoked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Send IBM - should not match WHERE
	if err := engine.Send(context.Background(), "SupportMarketDataBean", whereClauseBean{Symbol: "IBM"}); err != nil {
		t.Fatal(err)
	}
	if invoked {
		t.Fatal("IBM should not match symbol='CSCO'")
	}

	// Send CSCO - should match
	invoked = false
	if err := engine.Send(context.Background(), "SupportMarketDataBean", whereClauseBean{Symbol: "CSCO"}); err != nil {
		t.Fatal(err)
	}
	if !invoked {
		t.Fatal("CSCO should match symbol='CSCO'")
	}

	// Send IBM again - should not match
	invoked = false
	if err := engine.Send(context.Background(), "SupportMarketDataBean", whereClauseBean{Symbol: "IBM"}); err != nil {
		t.Fatal(err)
	}
	if invoked {
		t.Fatal("IBM should not match again")
	}

	// Send CSCO again - should match
	invoked = false
	if err := engine.Send(context.Background(), "SupportMarketDataBean", whereClauseBean{Symbol: "CSCO"}); err != nil {
		t.Fatal(err)
	}
	if !invoked {
		t.Fatal("CSCO should match again")
	}
}

// TestExprFilterWhereClauseNumericTypeMatchesEsper covers
// ExprFilterWhereClauseNumericType: numeric type coercion in select
// expressions and WHERE clause with mixed numeric types.
func TestExprFilterWhereClauseNumericTypeMatchesEsper(t *testing.T) {
	env, engine := newWhereClauseEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[whereClauseBean](env, "SupportBean").
		Filter(And(
			And(
				EqualOf(Field[whereClauseBean, int]("intPrimitive"), Field[whereClauseBean, int64]("longPrimitive")),
				EqualOf(Field[whereClauseBean, int]("intPrimitive"), Field[whereClauseBean, float64]("doublePrimitive")),
			),
			EqualOf(Field[whereClauseBean, float32]("floatPrimitive"), Field[whereClauseBean, float64]("doublePrimitive")),
		))
	plan, err := env.Build(Select(source,
		Alias("p1", Add[int64](Cast[int, int64](Field[whereClauseBean, int]("intPrimitive")), Field[whereClauseBean, int64]("longPrimitive"))),
		Alias("p2", Multiply[float64](Cast[int, float64](Field[whereClauseBean, int]("intPrimitive")), Field[whereClauseBean, float64]("doublePrimitive"))),
		Alias("p3", Divide[float64](Cast[float32, float64](Field[whereClauseBean, float32]("floatPrimitive")), Field[whereClauseBean, float64]("doublePrimitive"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}

	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Send non-matching values: int=1, long=2, float=3, double=4 -> no match
	if err := engine.Send(context.Background(), "SupportBean", whereClauseBean{
		IntPrimitive: 1, LongPrimitive: 2, FloatPrimitive: 3, DoublePrimitive: 4,
	}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatal("non-matching values should not produce output")
	}

	// Send matching values: all equal to 2
	if err := engine.Send(context.Background(), "SupportBean", whereClauseBean{
		IntPrimitive: 2, LongPrimitive: 2, FloatPrimitive: 2, DoublePrimitive: 2,
	}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("matching values should produce output, got %d batches", len(batches))
	}
	row := batches[0].New[0]
	// p1 = int + long = 2 + 2 = 4 (int64)
	if got := row.Get("p1").Any(); got != int64(4) {
		t.Fatalf("p1=%v, want int64(4)", got)
	}
	// p2 = int * double = 2 * 2.0 = 4.0 (float64)
	if got := row.Get("p2").Any(); got != float64(4.0) {
		t.Fatalf("p2=%v, want float64(4)", got)
	}
	// p3 = float / double = 2.0 / 2.0 = 1.0 (float64)
	if got := row.Get("p3").Any(); got != float64(1.0) {
		t.Fatalf("p3=%v, want float64(1)", got)
	}
}
