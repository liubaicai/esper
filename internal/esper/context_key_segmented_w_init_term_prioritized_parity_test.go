package esper

import (
	"context"
	"testing"
)

type ctxKeySegBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
	BoolPrimitive bool   `esper:"boolPrimitive"`
}

type ctxKeySegS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

func newCtxKeySegEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[ctxKeySegBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[ctxKeySegS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	return env
}

func ctxKeySegRows(t *testing.T, stmt *Statement) func() []Row {
	t.Helper()
	var rows []Row
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return func() []Row { return append([]Row(nil), rows...) }
}

// TestContextKeySegmentedWInitTermPrioritizedParity covers seven explicit
// initiated executions from ContextKeySegmentedWInitTermPrioritized:
// InitTermNoPartitionFilter, InitNoTerm (two variants), InitWCorrelatedTermFilter,
// FilterExprTermByFilter, FilterExprTermByFilterWExpr and Invalid.
func TestContextKeySegmentedWInitTermPrioritizedParity(t *testing.T) {
	t.Run("init-term-no-partition-filter", func(t *testing.T) {
		env := newCtxKeySegEnvironment(t)
		start := Equal[int](Field[ctxKeySegBean, int]("intPrimitive"), Literal(0))
		end := Equal[int](Field[ctxKeySegBean, int]("intPrimitive"), Literal(1000))
		if _, err := CreateInitiatedTerminatedContext(env, "Ctx", Field[ctxKeySegBean, string]("theString"), start, end); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		plan, err := env.Build(From[ctxKeySegBean](env, "SupportBean").GroupBy(
			Field[ctxKeySegBean, string]("theString"),
		).Select(
			Alias("theString", Field[ctxKeySegBean, string]("theString")),
			Alias("theSum", Sum[int](Field[ctxKeySegBean, int]("intPrimitive"))),
		).Query(StatementName("s0"), WithContext("Ctx"), WithOutput(OutputWhenTerminated())))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := ctxKeySegRows(t, deployment.Statements()[0])
		send := func(s string, i int) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), ctxKeySegBean{TheString: s, IntPrimitive: i}); err != nil {
				t.Fatal(err)
			}
		}
		send("A", 20)
		send("A", 1000)
		send("B", 0)
		send("B", 30)
		send("B", 1000)
		got := rows()
		if len(got) != 1 || got[0].Get("theString").Any() != "B" || got[0].Get("theSum").Any() != 30 {
			t.Fatalf("no-partition-filter rows = %#v", got)
		}
	})

	t.Run("init-no-term", func(t *testing.T) {
		for _, variant := range []string{"a", "b"} {
			t.Run(variant, func(t *testing.T) {
				env := newCtxKeySegEnvironment(t)
				start := Greater[int](Field[ctxKeySegS0, int]("id"), Literal(0))
				if _, err := CreateInitiatedContextBy(env, "Ctx", []Expr{
					Field[ctxKeySegS0, string]("p00"),
					Field[ctxKeySegS0, string]("p01"),
				}, start); err != nil {
					t.Fatal(err)
				}
				engine := NewEngine(env)
				plan, err := env.Build(From[ctxKeySegS0](env, "SupportBean_S0").GroupBy(
					Field[ctxKeySegS0, string]("p00"),
					Field[ctxKeySegS0, string]("p01"),
				).Select(
					Alias("p00", Field[ctxKeySegS0, string]("p00")),
					Alias("p01", Field[ctxKeySegS0, string]("p01")),
					Alias("theSum", Sum[int](Field[ctxKeySegS0, int]("id"))),
				).Query(StatementName("s0"), WithContext("Ctx")))
				if err != nil {
					t.Fatal(err)
				}
				deployment, err := engine.Deploy(context.Background(), plan)
				if err != nil {
					t.Fatal(err)
				}
				rows := ctxKeySegRows(t, deployment.Statements()[0])
				send := func(id int, p00, p01 string) {
					t.Helper()
					if err := engine.SendEvent(context.Background(), ctxKeySegS0{ID: id, P00: p00, P01: p01}); err != nil {
						t.Fatal(err)
					}
				}
				send(0, "A", "G1")
				send(-1, "B", "G1")
				if len(rows()) != 0 {
					t.Fatalf("init-no-term initial rows = %#v", rows())
				}
				send(10, "B", "G1")
				send(-1, "B", "G1")
				send(2, "A", "G1")
				send(3, "A", "G2")
				send(4, "A", "G2")
				send(6, "A", "G1")
				got := rows()
				if len(got) != 6 || got[0].Get("theSum").Any() != 10 || got[1].Get("theSum").Any() != 9 {
					t.Fatalf("init-no-term rows = %#v", got)
				}
			})
		}
	})

	t.Run("init-w-correlated-term-filter", func(t *testing.T) {
		env := newCtxKeySegEnvironment(t)
		start := Equal[bool](Field[ctxKeySegBean, bool]("boolPrimitive"), Literal(true))
		end := And(
			Equal[bool](Field[ctxKeySegBean, bool]("boolPrimitive"), Literal(false)),
			Equal[int](Field[ctxKeySegBean, int]("intPrimitive"),
				Property[int](ContextInitiatingEvent(), "intPrimitive")),
		)
		if _, err := CreateInitiatedTerminatedContext(env, "Ctx", Field[ctxKeySegBean, string]("theString"), start, end); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		plan, err := env.Build(From[ctxKeySegBean](env, "SupportBean").GroupBy(
			Field[ctxKeySegBean, string]("theString"),
		).Select(
			Alias("theString", Field[ctxKeySegBean, string]("theString")),
			Alias("theSum", Sum[int64](Field[ctxKeySegBean, int64]("longPrimitive"))),
		).Query(StatementName("s0"), WithContext("Ctx"), WithOutput(OutputWhenTerminated())))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := ctxKeySegRows(t, deployment.Statements()[0])
		send := func(s string, i int, l int64, b bool) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), ctxKeySegBean{TheString: s, IntPrimitive: i, LongPrimitive: l, BoolPrimitive: b}); err != nil {
				t.Fatal(err)
			}
		}
		send("A", 100, 1, true)
		send("B", 99, 2, false)
		send("B", 200, 3, true)
		send("A", 0, 4, false)
		send("B", 0, 5, false)
		send("A", 0, 6, true)
		send("B", 200, 7, false)
		got := rows()
		if len(got) != 1 || got[0].Get("theString").Any() != "B" || got[0].Get("theSum").Any() != int64(8) {
			t.Fatalf("init correlated B rows = %#v", got)
		}
		send("A", 100, 8, false)
		got = rows()
		if len(got) != 2 || got[1].Get("theString").Any() != "A" || got[1].Get("theSum").Any() != int64(11) {
			t.Fatalf("init correlated A rows = %#v", got)
		}
	})

	t.Run("filter-expr-term-by-filter", func(t *testing.T) {
		env := newCtxKeySegEnvironment(t)
		start := Equal[int](Field[ctxKeySegBean, int]("intPrimitive"), Literal(0))
		end := Equal[bool](Field[ctxKeySegBean, bool]("boolPrimitive"), Literal(false))
		if _, err := CreateInitiatedTerminatedContext(env, "Ctx", Field[ctxKeySegBean, string]("theString"), start, end); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		plan, err := env.Build(From[ctxKeySegBean](env, "SupportBean").GroupBy(
			Field[ctxKeySegBean, string]("theString"),
		).Select(
			Alias("theString", Field[ctxKeySegBean, string]("theString")),
			Alias("cnt", CountAll()),
		).Query(StatementName("s0"), WithContext("Ctx"), WithOutput(OutputWhenTerminated())))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := ctxKeySegRows(t, deployment.Statements()[0])
		send := func(s string, i int, b bool) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), ctxKeySegBean{TheString: s, IntPrimitive: i, BoolPrimitive: b}); err != nil {
				t.Fatal(err)
			}
		}
		send("A", 2, true)
		send("B", 0, true)
		send("B", 30, true)
		send("B", 0, false)
		got := rows()
		if len(got) != 1 || got[0].Get("theString").Any() != "B" || got[0].Get("cnt").Any() != int64(2) {
			t.Fatalf("filter-expr-term rows = %#v", got)
		}
	})

	t.Run("filter-expr-term-by-filter-w-expr", func(t *testing.T) {
		env := newCtxKeySegEnvironment(t)
		start := Equal[int](Field[ctxKeySegBean, int]("intPrimitive"), Literal(0))
		end := Equal[int](Field[ctxKeySegBean, int]("intPrimitive"), Literal(1))
		if _, err := CreateInitiatedTerminatedContext(env, "Ctx", Field[ctxKeySegBean, string]("theString"), start, end); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		plan, err := env.Build(From[ctxKeySegBean](env, "SupportBean").GroupBy(
			Field[ctxKeySegBean, string]("theString"),
		).Select(
			Alias("theString", Field[ctxKeySegBean, string]("theString")),
			Alias("cnt", CountAll()),
		).Query(StatementName("s0"), WithContext("Ctx"), WithOutput(OutputWhenTerminated())))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := ctxKeySegRows(t, deployment.Statements()[0])
		send := func(s string, i int) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), ctxKeySegBean{TheString: s, IntPrimitive: i}); err != nil {
				t.Fatal(err)
			}
		}
		send("A", 2)
		send("B", 0)
		send("B", 2)
		send("B", 1)
		got := rows()
		if len(got) != 1 || got[0].Get("theString").Any() != "B" || got[0].Get("cnt").Any() != int64(2) {
			t.Fatalf("filter-expr-term-expr rows = %#v", got)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		if _, err := CreateInitiatedTerminatedContext(nil, "Ctx", Literal("k"), Literal(true), Literal(true)); err == nil {
			t.Fatal("nil environment context was accepted")
		}
		if _, err := CreateInitiatedTerminatedContext(NewEnvironment(), "", Literal("k"), Literal(true), Literal(true)); err == nil {
			t.Fatal("empty context name was accepted")
		}
		if _, err := CreateInitiatedTerminatedContext(NewEnvironment(), "Ctx", nil, Literal(true), Literal(true)); err == nil {
			t.Fatal("nil context key was accepted")
		}
	})
}
