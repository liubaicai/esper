package esper

import (
	"context"
	"testing"
)

// filterOrRewriteBean extends SupportBean with double fields needed by
// FourOr and EightOr tests. Java SupportBean has doublePrimitive and
// doubleBoxed which the base filterOptimizableBean struct lacks.
type filterOrRewriteBean struct {
	TheString       string   `esper:"theString"`
	IntPrimitive    int      `esper:"intPrimitive"`
	IntBoxed        *int     `esper:"intBoxed"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	LongBoxed       *int64   `esper:"longBoxed"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
}

// filterOptIntAlphaBean mirrors Java SupportBean_IntAlphabetic for
// ExprFilterOptimizableOrRewrite. The five integer fields (a–e) are
// used by OR-rewrite, NOT-IN, and context-partitioned filter tests.
type filterOptIntAlphaBean struct {
	A int `esper:"a"`
	B int `esper:"b"`
	C int `esper:"c"`
	D int `esper:"d"`
	E int `esper:"e"`
}

// filterOptStringAlphaBean mirrors Java SupportBean_StringAlphabetic for
// ExprFilterOptimizableOrRewrite boolean-expression tests.
type filterOptStringAlphaBean struct {
	A string `esper:"a"`
	B string `esper:"b"`
	C string `esper:"c"`
}

func newFilterOptOrRewriteEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[filterOrRewriteBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[filterOptIntAlphaBean](env, "SupportBean_IntAlphabetic"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[filterOptStringAlphaBean](env, "SupportBean_StringAlphabetic"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	t.Cleanup(func() { _ = engine.Close(context.Background()) })
	return env, engine
}

func sendOptOrRewriteEvent[T any](t *testing.T, engine *Engine, event T) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
}

// TestExprFilterOrRewriteTwoOrParity covers
// ExprFilterOrRewriteTwoOr: the most basic OR rewrite — a filter with
// two equality arms on different fields. Go's filter AST decomposes
// the OR into two independent filter entries, each matching its arm.
func TestExprFilterOrRewriteTwoOrParity(t *testing.T) {
	env, engine := newFilterOptOrRewriteEnv(t)

	// theString = 'a' or intPrimitive = 1
	plan, err := env.Build(
		From[filterOrRewriteBean](env, "SupportBean").Filter(
			Or(
				Equal[string](Field[filterOrRewriteBean, string]("theString"), Literal("a")),
				Equal[int](Field[filterOrRewriteBean, int]("intPrimitive"), Literal(1)),
			),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })

	var invoked bool
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

	// Match: theString='a'
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "a"})
	if !invoked {
		t.Fatal("expected listener to fire for theString='a'")
	}

	// Match: intPrimitive=1
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "b", IntPrimitive: 1})
	if !invoked {
		t.Fatal("expected listener to fire for intPrimitive=1")
	}

	// No match
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "c"})
	if invoked {
		t.Fatal("expected listener NOT to fire for theString='c', intPrimitive=0")
	}
}

// TestExprFilterOrRewriteThreeOrParity covers
// ExprFilterOrRewriteThreeOr: a three-way OR across string, int, and
// long fields. The filter is decomposed into three independent entries.
func TestExprFilterOrRewriteThreeOrParity(t *testing.T) {
	env, engine := newFilterOptOrRewriteEnv(t)

	// theString = 'a' or intPrimitive = 1 or longPrimitive = 2
	plan, err := env.Build(
		From[filterOrRewriteBean](env, "SupportBean").Filter(
			Or(
				Or(
					Equal[string](Field[filterOrRewriteBean, string]("theString"), Literal("a")),
					Equal[int](Field[filterOrRewriteBean, int]("intPrimitive"), Literal(1)),
				),
				Equal[int64](Field[filterOrRewriteBean, int64]("longPrimitive"), Literal[int64](2)),
			),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })

	var invoked bool
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

	// Match: theString='a'
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "a"})
	if !invoked {
		t.Fatal("expected listener to fire for theString='a'")
	}

	// Match: intPrimitive=1
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "b", IntPrimitive: 1})
	if !invoked {
		t.Fatal("expected listener to fire for intPrimitive=1")
	}

	// Match: longPrimitive=2
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "c", LongPrimitive: 2})
	if !invoked {
		t.Fatal("expected listener to fire for longPrimitive=2")
	}

	// No match
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "v"})
	if invoked {
		t.Fatal("expected listener NOT to fire for non-matching event")
	}
}

// TestExprFilterOrRewriteThreeWithOverlapParity covers
// ExprFilterOrRewriteThreeWithOverlap: OR with two arms on the same
// field (theString) and one on a different field. The filter service
// creates separate entries for each theString equality.
func TestExprFilterOrRewriteThreeWithOverlapParity(t *testing.T) {
	env, engine := newFilterOptOrRewriteEnv(t)

	// theString = 'a' or theString = 'b' or intPrimitive=1
	plan, err := env.Build(
		From[filterOrRewriteBean](env, "SupportBean").Filter(
			Or(
				Or(
					Equal[string](Field[filterOrRewriteBean, string]("theString"), Literal("a")),
					Equal[string](Field[filterOrRewriteBean, string]("theString"), Literal("b")),
				),
				Equal[int](Field[filterOrRewriteBean, int]("intPrimitive"), Literal(1)),
			),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })

	var invoked bool
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

	// Match: theString='a', intPrimitive=1
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "a", IntPrimitive: 1})
	if !invoked {
		t.Fatal("expected listener to fire for theString='a'")
	}

	// Match: theString='b'
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "b"})
	if !invoked {
		t.Fatal("expected listener to fire for theString='b'")
	}

	// Match: intPrimitive=1
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "x", IntPrimitive: 1})
	if !invoked {
		t.Fatal("expected listener to fire for intPrimitive=1")
	}

	// No match
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "x"})
	if invoked {
		t.Fatal("expected listener NOT to fire for non-matching event")
	}
}

// TestExprFilterOrRewriteFourOrParity covers
// ExprFilterOrRewriteFourOr: four-way OR across string, int, long,
// and double fields. All four filter entries must independently match.
func TestExprFilterOrRewriteFourOrParity(t *testing.T) {
	env, engine := newFilterOptOrRewriteEnv(t)

	// theString = 'a' or intPrimitive=1 or longPrimitive=10 or doublePrimitive=100
	plan, err := env.Build(
		From[filterOrRewriteBean](env, "SupportBean").Filter(
			Or(
				Or(
					Equal[string](Field[filterOrRewriteBean, string]("theString"), Literal("a")),
					Equal[int](Field[filterOrRewriteBean, int]("intPrimitive"), Literal(1)),
				),
				Or(
					Equal[int64](Field[filterOrRewriteBean, int64]("longPrimitive"), Literal[int64](10)),
					Equal[float64](Field[filterOrRewriteBean, float64]("doublePrimitive"), Literal[float64](100)),
				),
			),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })

	var invoked bool
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

	// Match: theString='a', intPrimitive=1, longPrimitive=10, doublePrimitive=100
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "a", IntPrimitive: 1, LongPrimitive: 10, DoublePrimitive: 100})
	if !invoked {
		t.Fatal("expected listener to fire for all-match event")
	}

	// Match: only theString='a'
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "a"})
	if !invoked {
		t.Fatal("expected listener to fire for theString='a'")
	}

	// Match: only intPrimitive=1
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "x", IntPrimitive: 1})
	if !invoked {
		t.Fatal("expected listener to fire for intPrimitive=1")
	}

	// Match: only longPrimitive=10
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "x", LongPrimitive: 10})
	if !invoked {
		t.Fatal("expected listener to fire for longPrimitive=10")
	}

	// Match: only doublePrimitive=100
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "x", DoublePrimitive: 100})
	if !invoked {
		t.Fatal("expected listener to fire for doublePrimitive=100")
	}

	// No match
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "x"})
	if invoked {
		t.Fatal("expected listener NOT to fire for non-matching event")
	}
}

// TestExprFilterOrRewriteWithAndParity covers
// ExprFilterOrRewriteOrRewriteWithAnd: OR of AND groups — each arm is
// a conjunction of two equality conditions. The filter service creates
// separate entries for each AND group.
func TestExprFilterOrRewriteWithAndParity(t *testing.T) {
	env, engine := newFilterOptOrRewriteEnv(t)

	// (theString = 'a' and intPrimitive = 1) or (theString = 'b' and intPrimitive = 2)
	plan, err := env.Build(
		From[filterOrRewriteBean](env, "SupportBean").Filter(
			Or(
				And(
					Equal[string](Field[filterOrRewriteBean, string]("theString"), Literal("a")),
					Equal[int](Field[filterOrRewriteBean, int]("intPrimitive"), Literal(1)),
				),
				And(
					Equal[string](Field[filterOrRewriteBean, string]("theString"), Literal("b")),
					Equal[int](Field[filterOrRewriteBean, int]("intPrimitive"), Literal(2)),
				),
			),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })

	var invoked bool
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

	// Match: theString='a', intPrimitive=1
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "a", IntPrimitive: 1})
	if !invoked {
		t.Fatal("expected listener to fire for (a,1)")
	}

	// Match: theString='b', intPrimitive=2
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "b", IntPrimitive: 2})
	if !invoked {
		t.Fatal("expected listener to fire for (b,2)")
	}

	// No match: theString='a', intPrimitive=0
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "a", IntPrimitive: 0})
	if invoked {
		t.Fatal("expected listener NOT to fire for (a,0)")
	}

	// No match: theString='x', intPrimitive=1
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "x", IntPrimitive: 1})
	if invoked {
		t.Fatal("expected listener NOT to fire for (x,1)")
	}
}

// TestExprFilterOrRewriteAndOrMultiParity covers
// ExprFilterOrRewriteOrRewriteAndOrMulti: nested AND-OR structure
// where multiple OR groups are AND'ed together. The filter service
// decomposes this into a cross-product of filter entries.
func TestExprFilterOrRewriteAndOrMultiParity(t *testing.T) {
	env, engine := newFilterOptOrRewriteEnv(t)

	// a=1 and (b=1 or c=1) and (d=1 or e=1)
	plan, err := env.Build(
		From[filterOptIntAlphaBean](env, "SupportBean_IntAlphabetic").Filter(
			And(
				And(
					Equal[int](Field[filterOptIntAlphaBean, int]("a"), Literal(1)),
					Or(
						Equal[int](Field[filterOptIntAlphaBean, int]("b"), Literal(1)),
						Equal[int](Field[filterOptIntAlphaBean, int]("c"), Literal(1)),
					),
				),
				Or(
					Equal[int](Field[filterOptIntAlphaBean, int]("d"), Literal(1)),
					Equal[int](Field[filterOptIntAlphaBean, int]("e"), Literal(1)),
				),
			),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })

	var invoked bool
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

	// Match: a=1, b=1, d=1
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{A: 1, B: 1, D: 1})
	if !invoked {
		t.Fatal("expected listener to fire for (1,1,0,1,0)")
	}

	// Match: a=1, c=1, e=1
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{A: 1, C: 1, E: 1})
	if !invoked {
		t.Fatal("expected listener to fire for (1,0,1,0,1)")
	}

	// Match: a=1, b=1, e=1
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{A: 1, B: 1, E: 1})
	if !invoked {
		t.Fatal("expected listener to fire for (1,1,0,0,1)")
	}

	// Match: a=1, c=1, d=1
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{A: 1, C: 1, D: 1})
	if !invoked {
		t.Fatal("expected listener to fire for (1,0,1,1,0)")
	}

	// No match: a=1, b=0, c=0, d=1 (b=0 and c=0 fails middle OR)
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{A: 1, D: 1})
	if invoked {
		t.Fatal("expected listener NOT to fire for (1,0,0,1,0)")
	}

	// No match: a=0, b=1, d=1 (a=0 fails first AND)
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{B: 1, D: 1})
	if invoked {
		t.Fatal("expected listener NOT to fire for (0,1,0,1,0)")
	}
}

// TestExprFilterOrRewriteInnerOrParity covers
// ExprFilterOrRewriteAndRewriteInnerOr: an AND where the right side
// is an OR of two different field equalities. The filter service
// creates two entries: one for each OR arm combined with the AND.
func TestExprFilterOrRewriteInnerOrParity(t *testing.T) {
	env, engine := newFilterOptOrRewriteEnv(t)

	// theString='a' and (intPrimitive=1 or longPrimitive=10)
	plan, err := env.Build(
		From[filterOrRewriteBean](env, "SupportBean").Filter(
			And(
				Equal[string](Field[filterOrRewriteBean, string]("theString"), Literal("a")),
				Or(
					Equal[int](Field[filterOrRewriteBean, int]("intPrimitive"), Literal(1)),
					Equal[int64](Field[filterOrRewriteBean, int64]("longPrimitive"), Literal[int64](10)),
				),
			),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })

	var invoked bool
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

	// Match: theString='a', intPrimitive=1
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "a", IntPrimitive: 1})
	if !invoked {
		t.Fatal("expected listener to fire for (a, 1)")
	}

	// Match: theString='a', longPrimitive=10
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "a", LongPrimitive: 10})
	if !invoked {
		t.Fatal("expected listener to fire for (a, long=10)")
	}

	// Match: theString='a', both
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "a", IntPrimitive: 1, LongPrimitive: 10})
	if !invoked {
		t.Fatal("expected listener to fire for (a, 1, long=10)")
	}

	// No match: theString='x', intPrimitive=1
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "x", IntPrimitive: 1})
	if invoked {
		t.Fatal("expected listener NOT to fire for (x, 1)")
	}

	// No match: theString='a', intPrimitive=2, longPrimitive=20
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "a", IntPrimitive: 2, LongPrimitive: 20})
	if invoked {
		t.Fatal("expected listener NOT to fire for (a, 2, long=20)")
	}
}

// TestExprFilterOrRewriteNotEqualsOrParity covers
// ExprFilterOrRewriteAndRewriteNotEqualsOr: NOT EQUALS combined with
// OR. The filter rewrites a!=1 and a!=2 and (b=1 or c=1) into
// NOT_IN_LIST for the consolidated NOT EQUALS, plus separate entries
// for each OR arm.
func TestExprFilterOrRewriteNotEqualsOrParity(t *testing.T) {
	env, engine := newFilterOptOrRewriteEnv(t)

	// a!=1 and a!=2 and (b=1 or c=1)
	plan, err := env.Build(
		From[filterOptIntAlphaBean](env, "SupportBean_IntAlphabetic").Filter(
			And(
				And(
					NotEqual[int](Field[filterOptIntAlphaBean, int]("a"), Literal(1)),
					NotEqual[int](Field[filterOptIntAlphaBean, int]("a"), Literal(2)),
				),
				Or(
					Equal[int](Field[filterOptIntAlphaBean, int]("b"), Literal(1)),
					Equal[int](Field[filterOptIntAlphaBean, int]("c"), Literal(1)),
				),
			),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })

	var invoked bool
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

	// Match: a=3, b=1 (a!=1 and a!=2 pass, b=1 matches OR)
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{A: 3, B: 1})
	if !invoked {
		t.Fatal("expected listener to fire for (3,1,0)")
	}

	// Match: a=3, c=1 (a!=1 and a!=2 pass, c=1 matches OR)
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{A: 3, C: 1})
	if !invoked {
		t.Fatal("expected listener to fire for (3,0,1)")
	}

	// Match: a=0, b=1
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{A: 0, B: 1})
	if !invoked {
		t.Fatal("expected listener to fire for (0,1,0)")
	}

	// No match: a=1 (fails a!=1)
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{A: 1, B: 1})
	if invoked {
		t.Fatal("expected listener NOT to fire for a=1")
	}

	// No match: a=2 (fails a!=2)
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{A: 2, B: 1})
	if invoked {
		t.Fatal("expected listener NOT to fire for a=2")
	}

	// No match: a=3, b=0, c=0 (OR not satisfied)
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{A: 3})
	if invoked {
		t.Fatal("expected listener NOT to fire for (3,0,0)")
	}
}

// TestExprFilterOrRewriteNotEqualsConsolidateParity covers
// ExprFilterOrRewriteAndRewriteNotEqualsConsolidate: NOT EQUALS
// consolidation where a!=1 and a!=2 and (a!=3 or a!=4) results in
// NOT_IN_LIST entries. The OR of two NOT EQUALS on the same field
// simplifies because at most one can be false.
func TestExprFilterOrRewriteNotEqualsConsolidateParity(t *testing.T) {
	env, engine := newFilterOptOrRewriteEnv(t)

	// a!=1 and a!=2 and (a!=3 or a!=4)
	plan, err := env.Build(
		From[filterOptIntAlphaBean](env, "SupportBean_IntAlphabetic").Filter(
			And(
				And(
					NotEqual[int](Field[filterOptIntAlphaBean, int]("a"), Literal(1)),
					NotEqual[int](Field[filterOptIntAlphaBean, int]("a"), Literal(2)),
				),
				Or(
					NotEqual[int](Field[filterOptIntAlphaBean, int]("a"), Literal(3)),
					NotEqual[int](Field[filterOptIntAlphaBean, int]("a"), Literal(4)),
				),
			),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })

	var invoked bool
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

	// Match: a=3 (a!=1, a!=2, a!=4 is true → OR satisfied)
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{A: 3})
	if !invoked {
		t.Fatal("expected listener to fire for a=3")
	}

	// Match: a=4 (a!=1, a!=2, a!=3 is true → OR satisfied)
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{A: 4})
	if !invoked {
		t.Fatal("expected listener to fire for a=4")
	}

	// Match: a=0 (all NOT EQUALS pass)
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{A: 0})
	if !invoked {
		t.Fatal("expected listener to fire for a=0")
	}

	// No match: a=1
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{A: 1})
	if invoked {
		t.Fatal("expected listener NOT to fire for a=1")
	}

	// No match: a=2
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptIntAlphaBean{A: 2})
	if invoked {
		t.Fatal("expected listener NOT to fire for a=2")
	}
}

// TestExprFilterOrRewriteBooleanExprSimpleParity covers
// ExprFilterOrRewriteBooleanExprSimple: a filter that mixes LIKE
// with OR, preventing simple index decomposition. The filter service
// falls back to a boolean expression for the non-indexable arm.
func TestExprFilterOrRewriteBooleanExprSimpleParity(t *testing.T) {
	env, engine := newFilterOptOrRewriteEnv(t)

	// a like 'a%' and (b='b' or c='c')
	plan, err := env.Build(
		From[filterOptStringAlphaBean](env, "SupportBean_StringAlphabetic").Filter(
			And(
				Like(
					Field[filterOptStringAlphaBean, string]("a"),
					Literal("a%"),
				),
				Or(
					Equal[string](Field[filterOptStringAlphaBean, string]("b"), Literal("b")),
					Equal[string](Field[filterOptStringAlphaBean, string]("c"), Literal("c")),
				),
			),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })

	var invoked bool
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

	// Match: a='a1', b='b'
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptStringAlphaBean{A: "a1", B: "b"})
	if !invoked {
		t.Fatal("expected listener to fire for (a1, b)")
	}

	// Match: a='a1', c='c'
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptStringAlphaBean{A: "a1", C: "c"})
	if !invoked {
		t.Fatal("expected listener to fire for (a1, c)")
	}

	// No match: a='x', b='b' (LIKE fails)
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptStringAlphaBean{A: "x", B: "b"})
	if invoked {
		t.Fatal("expected listener NOT to fire for (x, b)")
	}

	// No match: a='a1', b=nil, c=nil (OR not satisfied)
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptStringAlphaBean{A: "a1"})
	if invoked {
		t.Fatal("expected listener NOT to fire for (a1, nil, nil)")
	}
}

// TestExprFilterOrRewriteEightOrParity covers
// ExprFilterOrRewriteOrRewriteEightOr: an eight-way OR across all
// field types (string, int, long, double, bool, boxed variants).
func TestExprFilterOrRewriteEightOrParity(t *testing.T) {
	env, engine := newFilterOptOrRewriteEnv(t)

	// theString='a' or intPrimitive=1 or longPrimitive=10 or doublePrimitive=100
	// or boolPrimitive=true or intBoxed=2 or longBoxed=20 or doubleBoxed=200
	plan, err := env.Build(
		From[filterOrRewriteBean](env, "SupportBean").Filter(
			Or(
				Or(
					Or(
						Equal[string](Field[filterOrRewriteBean, string]("theString"), Literal("a")),
						Equal[int](Field[filterOrRewriteBean, int]("intPrimitive"), Literal(1)),
					),
					Or(
						Equal[int64](Field[filterOrRewriteBean, int64]("longPrimitive"), Literal[int64](10)),
						Equal[float64](Field[filterOrRewriteBean, float64]("doublePrimitive"), Literal[float64](100)),
					),
				),
				Or(
					Or(
						Equal[bool](Field[filterOrRewriteBean, bool]("boolPrimitive"), Literal(true)),
						Equal[int](Field[filterOrRewriteBean, int]("intBoxed"), Literal(2)),
					),
					Or(
						Equal[int64](Field[filterOrRewriteBean, int64]("longBoxed"), Literal[int64](20)),
						Equal[float64](Field[filterOrRewriteBean, float64]("doubleBoxed"), Literal[float64](200)),
					),
				),
			),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })

	var invoked bool
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

	// Match: theString='a'
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{TheString: "a"})
	if !invoked {
		t.Fatal("expected listener to fire for theString='a'")
	}

	// Match: intPrimitive=1
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{IntPrimitive: 1})
	if !invoked {
		t.Fatal("expected listener to fire for intPrimitive=1")
	}

	// Match: boolPrimitive=true
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{BoolPrimitive: true})
	if !invoked {
		t.Fatal("expected listener to fire for boolPrimitive=true")
	}

	// Match: longBoxed=20
	invoked = false
	longBoxed := int64(20)
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{LongBoxed: &longBoxed})
	if !invoked {
		t.Fatal("expected listener to fire for longBoxed=20")
	}

	// Match: doubleBoxed=200
	invoked = false
	doubleBoxed := float64(200)
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{DoubleBoxed: &doubleBoxed})
	if !invoked {
		t.Fatal("expected listener to fire for doubleBoxed=200")
	}

	// No match
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOrRewriteBean{})
	if invoked {
		t.Fatal("expected listener NOT to fire for zero-value event")
	}
}

// TestExprFilterOrRewriteBooleanExprAndParity covers
// ExprFilterOrRewriteBooleanExprAnd: two OR groups AND'ed together
// where both arms use LIKE (non-indexable). The entire filter falls
// back to boolean expression evaluation.
func TestExprFilterOrRewriteBooleanExprAndParity(t *testing.T) {
	env, engine := newFilterOptOrRewriteEnv(t)

	// (a='a' or a like 'A%') and (b='b' or b like 'B%')
	plan, err := env.Build(
		From[filterOptStringAlphaBean](env, "SupportBean_StringAlphabetic").Filter(
			And(
				Or(
					Equal[string](Field[filterOptStringAlphaBean, string]("a"), Literal("a")),
					Like(Field[filterOptStringAlphaBean, string]("a"), Literal("A%")),
				),
				Or(
					Equal[string](Field[filterOptStringAlphaBean, string]("b"), Literal("b")),
					Like(Field[filterOptStringAlphaBean, string]("b"), Literal("B%")),
				),
			),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })

	var invoked bool
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

	// Match: a='a', b='b'
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptStringAlphaBean{A: "a", B: "b"})
	if !invoked {
		t.Fatal("expected listener to fire for (a, b)")
	}

	// Match: a='A1', b='b' (A% matches)
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptStringAlphaBean{A: "A1", B: "b"})
	if !invoked {
		t.Fatal("expected listener to fire for (A1, b)")
	}

	// Match: a='a', b='B1' (B% matches)
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptStringAlphaBean{A: "a", B: "B1"})
	if !invoked {
		t.Fatal("expected listener to fire for (a, B1)")
	}

	// Match: a='A1', b='B1' (both LIKE match)
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptStringAlphaBean{A: "A1", B: "B1"})
	if !invoked {
		t.Fatal("expected listener to fire for (A1, B1)")
	}

	// No match: a='x', b='b'
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptStringAlphaBean{A: "x", B: "b"})
	if invoked {
		t.Fatal("expected listener NOT to fire for (x, b)")
	}

	// No match: a='a', b='x'
	invoked = false
	sendOptOrRewriteEvent(t, engine, filterOptStringAlphaBean{A: "a", B: "x"})
	if invoked {
		t.Fatal("expected listener NOT to fire for (a, x)")
	}
}
