package esper

import (
	"context"
	"testing"
)

type exprFilterConfirmS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

type exprFilterConfirmS1 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
	P10 string `esper:"p10"`
	P11 string `esper:"p11"`
	P12 string `esper:"p12"`
	P13 string `esper:"p13"`
}

type exprFilterConfirmS2 struct {
	ID int `esper:"id"`
}

type exprFilterConfirmBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func newExprFilterConfirmEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[exprFilterConfirmS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprFilterConfirmS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprFilterConfirmS2](env, "SupportBean_S2"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprFilterConfirmBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func exprFilterConfirmInvoked(t *testing.T, stmt *Statement) func() bool {
	t.Helper()
	var invoked bool
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			invoked = true
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return func() bool {
		current := invoked
		invoked = false
		return current
	}
}

func exprFilterConfirmContext(t *testing.T, env *Environment, filter Expression[bool], endType string) (*Engine, func() bool) {
	t.Helper()
	start := Equal[string](TypeName(EventValue[Event]()), Literal("SupportBean_S0"))
	end := Equal[string](TypeName(EventValue[Event]()), Literal(endType))
	if _, err := CreateInitiatedTerminatedContext(env, "MyContext", Literal("global"), start, end); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	plan, err := env.Build(From[exprFilterConfirmS1](env, "SupportBean_S1").Filter(filter).Query(
		StatementName("s0"), WithContext("MyContext")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })
	return engine, exprFilterConfirmInvoked(t, deployment.Statements()[0])
}

func exprFilterConfirmBeanContext(t *testing.T, env *Environment, filter Expression[bool]) (*Engine, func() bool) {
	t.Helper()
	start := Equal[string](TypeName(EventValue[Event]()), Literal("SupportBean_S0"))
	end := Equal[string](TypeName(EventValue[Event]()), Literal("SupportBean_S1"))
	if _, err := CreateInitiatedTerminatedContext(env, "MyContext", Literal("global"), start, end); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	plan, err := env.Build(From[exprFilterConfirmBean](env, "SupportBean").Filter(filter).Query(
		StatementName("s0"), WithContext("MyContext")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })
	return engine, exprFilterConfirmInvoked(t, deployment.Statements()[0])
}

func exprFilterConfirmPattern(t *testing.T, env *Environment, filter Expression[bool]) (*Engine, func() bool) {
	t.Helper()
	left := PatternFrom(From[exprFilterConfirmS0](env, "SupportBean_S0"), "s0", Literal(true)).Every()
	right := PatternFrom(From[exprFilterConfirmS1](env, "SupportBean_S1"), "e", filter)
	pattern := left.Then(right)
	plan, err := env.Build(pattern.Select(Alias("x", Literal(1))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })
	return engine, exprFilterConfirmInvoked(t, deployment.Statements()[0])
}

func exprFilterConfirmSendS0(t *testing.T, engine *Engine, p00, p01 string) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), exprFilterConfirmS0{ID: 1, P00: p00, P01: p01}); err != nil {
		t.Fatal(err)
	}
}

func exprFilterConfirmSendS1(t *testing.T, engine *Engine, p10, p11, p12, p13 string) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), exprFilterConfirmS1{ID: 1, P10: p10, P11: p11, P12: p12, P13: p13}); err != nil {
		t.Fatal(err)
	}
}

func exprFilterConfirmSendS2(t *testing.T, engine *Engine) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), exprFilterConfirmS2{ID: 1}); err != nil {
		t.Fatal(err)
	}
}

func exprFilterConfirmSendS1End(t *testing.T, engine *Engine) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), exprFilterConfirmS1{ID: 1}); err != nil {
		t.Fatal(err)
	}
}

func exprFilterConfirmSendBean(t *testing.T, engine *Engine, theString string) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), exprFilterConfirmBean{TheString: theString}); err != nil {
		t.Fatal(err)
	}
}

func exprFilterConfirmAssert(t *testing.T, invoked func() bool, want bool, label string) {
	t.Helper()
	if got := invoked(); got != want {
		t.Fatalf("%s: invoked=%v, want %v", label, got, want)
	}
}

// TestExprFilterOptimizableConditionNegateConfirmParity mirrors ten
// listener-observable executions from ExprFilterOptimizableConditionNegateConfirm:
// OnePathAndLeftLRightV, OnePathOrLeftLRightV, TwoPathAndLeftOrLLRightV,
// TwoPathAndLeftOrLVRightOrLL, FourPathAndWithOrLLOrLL,
// OnePathAndWithOrLVVOrLVOrLV, OnePathAndLeftLOrVRightLOrV,
// TwoPathOrLeftLRightAndLWithV, TwoPathOrWithLLV and OnePathOrWithLVV.
func TestExprFilterOptimizableConditionNegateConfirmParity(t *testing.T) {
	t.Run("one-path-and-left-l-right-v", func(t *testing.T) {
		env := newExprFilterConfirmEnvironment(t)
		ctxP00 := Equal[string](Property[string](ContextInitiatingEvent(), "p00"), Literal("x"))
		filter := And(
			Equal[string](Field[exprFilterConfirmBean, string]("theString"), Literal("abc")),
			ctxP00,
		)
		engine, invoked := exprFilterConfirmBeanContext(t, env, filter)
		exprFilterConfirmSendS0(t, engine, "x", "")
		exprFilterConfirmSendBean(t, engine, "abc")
		exprFilterConfirmAssert(t, invoked, true, "abc/x")
		exprFilterConfirmSendBean(t, engine, "def")
		exprFilterConfirmAssert(t, invoked, false, "def/x")
		exprFilterConfirmSendS1End(t, engine)
		exprFilterConfirmSendBean(t, engine, "abc")
		exprFilterConfirmAssert(t, invoked, false, "abc-after-end")
		exprFilterConfirmSendS0(t, engine, "-", "")
		exprFilterConfirmSendBean(t, engine, "abc")
		exprFilterConfirmAssert(t, invoked, false, "abc/dash")
	})

	t.Run("one-path-or-left-l-right-v", func(t *testing.T) {
		env := newExprFilterConfirmEnvironment(t)
		ctxP00 := Equal[string](Property[string](ContextInitiatingEvent(), "p00"), Literal("x"))
		filter := Or(
			Equal[string](Field[exprFilterConfirmBean, string]("theString"), Literal("abc")),
			ctxP00,
		)
		engine, invoked := exprFilterConfirmBeanContext(t, env, filter)
		exprFilterConfirmSendS0(t, engine, "x", "")
		exprFilterConfirmSendBean(t, engine, "abc")
		exprFilterConfirmAssert(t, invoked, true, "abc/x")
		exprFilterConfirmSendBean(t, engine, "def")
		exprFilterConfirmAssert(t, invoked, true, "def/x")
		exprFilterConfirmSendS1End(t, engine)
		exprFilterConfirmSendS0(t, engine, "-", "")
		exprFilterConfirmSendBean(t, engine, "abc")
		exprFilterConfirmAssert(t, invoked, true, "abc/dash")
		exprFilterConfirmSendBean(t, engine, "def")
		exprFilterConfirmAssert(t, invoked, false, "def/dash")
	})

	t.Run("two-path-and-left-or-ll-right-v", func(t *testing.T) {
		env := newExprFilterConfirmEnvironment(t)
		ctxP00 := Equal[string](Property[string](ContextInitiatingEvent(), "p00"), Literal("x"))
		filter := And(
			Or(
				Equal[string](Field[exprFilterConfirmS1, string]("p10"), Literal("a")),
				Equal[string](Field[exprFilterConfirmS1, string]("p11"), Literal("b")),
			),
			ctxP00,
		)
		engine, invoked := exprFilterConfirmContext(t, env, filter, "SupportBean_S2")
		exprFilterConfirmSendS0(t, engine, "-", "")
		exprFilterConfirmSendS1(t, engine, "a", "b", "", "")
		exprFilterConfirmAssert(t, invoked, false, "dash-a-b")
		exprFilterConfirmSendS2(t, engine)
		exprFilterConfirmSendS0(t, engine, "x", "")
		exprFilterConfirmSendS1(t, engine, "-", "-", "", "")
		exprFilterConfirmAssert(t, invoked, false, "x-none")
		exprFilterConfirmSendS1(t, engine, "a", "-", "", "")
		exprFilterConfirmAssert(t, invoked, true, "x-a")
		exprFilterConfirmSendS1(t, engine, "-", "b", "", "")
		exprFilterConfirmAssert(t, invoked, true, "x-b")
	})

	t.Run("two-path-and-left-or-lv-right-or-ll", func(t *testing.T) {
		env := newExprFilterConfirmEnvironment(t)
		ctxP00 := Equal[string](Property[string](ContextInitiatingEvent(), "p00"), Literal("x"))
		filter := And(
			Or(
				Equal[string](Field[exprFilterConfirmS1, string]("p10"), Literal("a")),
				ctxP00,
			),
			Or(
				Equal[string](Field[exprFilterConfirmS1, string]("p11"), Literal("c")),
				Equal[string](Field[exprFilterConfirmS1, string]("p12"), Literal("d")),
			),
		)
		engine, invoked := exprFilterConfirmContext(t, env, filter, "SupportBean_S2")
		exprFilterConfirmSendS0(t, engine, "-", "")
		exprFilterConfirmSendS1(t, engine, "a", "c", "-", "")
		exprFilterConfirmAssert(t, invoked, true, "dash-a-c")
		exprFilterConfirmSendS1(t, engine, "a", "-", "d", "")
		exprFilterConfirmAssert(t, invoked, true, "dash-a-d")
		exprFilterConfirmSendS1(t, engine, "a", "c", "d", "")
		exprFilterConfirmAssert(t, invoked, true, "dash-a-c-d")
		exprFilterConfirmSendS1(t, engine, "-", "c", "d", "")
		exprFilterConfirmAssert(t, invoked, false, "dash-none-c-d")
		exprFilterConfirmSendS2(t, engine)
		exprFilterConfirmSendS0(t, engine, "x", "")
		exprFilterConfirmSendS1(t, engine, "-", "c", "-", "")
		exprFilterConfirmAssert(t, invoked, true, "x-c")
		exprFilterConfirmSendS1(t, engine, "-", "-", "d", "")
		exprFilterConfirmAssert(t, invoked, true, "x-d")
		exprFilterConfirmSendS1(t, engine, "-", "-", "-", "")
		exprFilterConfirmAssert(t, invoked, false, "x-none")
	})

	t.Run("four-path-and-with-or-ll-or-ll", func(t *testing.T) {
		env := newExprFilterConfirmEnvironment(t)
		filter := And(
			Or(
				Equal[string](Field[exprFilterConfirmS1, string]("p10"), Literal("a")),
				Equal[string](Field[exprFilterConfirmS1, string]("p11"), Literal("b")),
			),
			Or(
				Equal[string](Field[exprFilterConfirmS1, string]("p12"), Literal("c")),
				Equal[string](Field[exprFilterConfirmS1, string]("p13"), Literal("d")),
			),
		)
		engine := NewEngine(env)
		plan, err := env.Build(From[exprFilterConfirmS1](env, "SupportBean_S1").Filter(filter).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		invoked := exprFilterConfirmInvoked(t, deployment.Statements()[0])
		for _, tc := range []struct {
			p10, p11, p12, p13 string
			want               bool
		}{
			{"-", "-", "-", "-", false},
			{"a", "-", "c", "-", true},
			{"a", "-", "-", "d", true},
			{"-", "b", "c", "-", true},
			{"-", "b", "-", "d", true},
			{"a", "b", "-", "-", false},
			{"-", "-", "c", "d", false},
		} {
			exprFilterConfirmSendS1(t, engine, tc.p10, tc.p11, tc.p12, tc.p13)
			exprFilterConfirmAssert(t, invoked, tc.want, "four-path")
		}
	})

	t.Run("one-path-and-with-or-lvv-or-lv-or-lv", func(t *testing.T) {
		env := newExprFilterConfirmEnvironment(t)
		s0P00 := TagField[string]("s0", "p00")
		filter := And(
			And(
				Or(
					Equal[string](Field[exprFilterConfirmS1, string]("p10"), Literal("a")),
					Or(
						Like(s0P00, Literal("%1%")),
						Like(s0P00, Literal("%2%")),
					),
				),
				Or(
					Equal[string](Field[exprFilterConfirmS1, string]("p11"), Literal("b")),
					Like(s0P00, Literal("%3%")),
				),
			),
			Or(
				Equal[string](Field[exprFilterConfirmS1, string]("p12"), Literal("c")),
				Like(s0P00, Literal("%4%")),
			),
		)
		engine, invoked := exprFilterConfirmPattern(t, env, filter)
		exprFilterConfirmSendS0(t, engine, "-", "")
		exprFilterConfirmSendS1(t, engine, "a", "b", "-", "")
		exprFilterConfirmAssert(t, invoked, false, "dash-a-b")
		exprFilterConfirmSendS1(t, engine, "-", "b", "c", "")
		exprFilterConfirmAssert(t, invoked, false, "dash-b-c")
		exprFilterConfirmSendS1(t, engine, "a", "b", "c", "")
		exprFilterConfirmAssert(t, invoked, true, "dash-abc")
		exprFilterConfirmSendS0(t, engine, "1", "")
		exprFilterConfirmSendS1(t, engine, "-", "b", "-", "")
		exprFilterConfirmAssert(t, invoked, false, "one-b")
		exprFilterConfirmSendS1(t, engine, "-", "-", "c", "")
		exprFilterConfirmAssert(t, invoked, false, "one-c")
		exprFilterConfirmSendS1(t, engine, "-", "b", "c", "")
		exprFilterConfirmAssert(t, invoked, true, "one-b-c")
		exprFilterConfirmSendS0(t, engine, "3", "")
		exprFilterConfirmSendS1(t, engine, "a", "-", "-", "")
		exprFilterConfirmAssert(t, invoked, false, "three-a")
		exprFilterConfirmSendS1(t, engine, "-", "-", "c", "")
		exprFilterConfirmAssert(t, invoked, false, "three-c")
		exprFilterConfirmSendS1(t, engine, "a", "-", "c", "")
		exprFilterConfirmAssert(t, invoked, true, "three-a-c")
		exprFilterConfirmSendS0(t, engine, "4", "")
		exprFilterConfirmSendS1(t, engine, "a", "-", "-", "")
		exprFilterConfirmAssert(t, invoked, false, "four-a")
		exprFilterConfirmSendS1(t, engine, "-", "b", "-", "")
		exprFilterConfirmAssert(t, invoked, false, "four-b")
		exprFilterConfirmSendS1(t, engine, "a", "b", "-", "")
		exprFilterConfirmAssert(t, invoked, true, "four-a-b")
		exprFilterConfirmSendS0(t, engine, "1234", "")
		exprFilterConfirmSendS1(t, engine, "-", "-", "-", "")
		exprFilterConfirmAssert(t, invoked, true, "all")
	})

	t.Run("one-path-and-left-l-or-v-right-l-or-v", func(t *testing.T) {
		env := newExprFilterConfirmEnvironment(t)
		filter := And(
			Or(
				Equal[string](Field[exprFilterConfirmS1, string]("p10"), Literal("a")),
				Equal[string](TagField[string]("s0", "p00"), Literal("x")),
			),
			Or(
				Equal[string](Field[exprFilterConfirmS1, string]("p11"), Literal("b")),
				Equal[string](TagField[string]("s0", "p01"), Literal("y")),
			),
		)
		engine, invoked := exprFilterConfirmPattern(t, env, filter)
		exprFilterConfirmSendS0(t, engine, "-", "-")
		exprFilterConfirmSendS1(t, engine, "-", "b", "", "")
		exprFilterConfirmAssert(t, invoked, false, "dash-b")
		exprFilterConfirmSendS1(t, engine, "a", "-", "", "")
		exprFilterConfirmAssert(t, invoked, false, "dash-a")
		exprFilterConfirmSendS1(t, engine, "a", "b", "", "")
		exprFilterConfirmAssert(t, invoked, true, "dash-a-b")
		exprFilterConfirmSendS0(t, engine, "x", "-")
		exprFilterConfirmSendS1(t, engine, "a", "-", "", "")
		exprFilterConfirmAssert(t, invoked, false, "x-a")
		exprFilterConfirmSendS1(t, engine, "-", "b", "", "")
		exprFilterConfirmAssert(t, invoked, true, "x-b")
		exprFilterConfirmSendS0(t, engine, "-", "y")
		exprFilterConfirmSendS1(t, engine, "-", "b", "", "")
		exprFilterConfirmAssert(t, invoked, false, "y-b")
		exprFilterConfirmSendS1(t, engine, "a", "-", "", "")
		exprFilterConfirmAssert(t, invoked, true, "y-a")
		exprFilterConfirmSendS0(t, engine, "x", "y")
		exprFilterConfirmSendS1(t, engine, "-", "-", "", "")
		exprFilterConfirmAssert(t, invoked, true, "xy-none")
	})

	t.Run("two-path-or-left-l-right-and-l-with-v", func(t *testing.T) {
		env := newExprFilterConfirmEnvironment(t)
		filter := Or(
			Equal[string](Field[exprFilterConfirmS1, string]("p10"), Literal("a")),
			And(
				Equal[string](Field[exprFilterConfirmS1, string]("p11"), Literal("b")),
				Equal[string](TagField[string]("s0", "p00"), Literal("x")),
			),
		)
		engine, invoked := exprFilterConfirmPattern(t, env, filter)
		exprFilterConfirmSendS0(t, engine, "x", "")
		exprFilterConfirmSendS1(t, engine, "-", "-", "", "")
		exprFilterConfirmAssert(t, invoked, false, "x-none")
		exprFilterConfirmSendS1(t, engine, "-", "b", "", "")
		exprFilterConfirmAssert(t, invoked, true, "x-b")
		exprFilterConfirmSendS0(t, engine, "x", "")
		exprFilterConfirmSendS1(t, engine, "a", "-", "", "")
		exprFilterConfirmAssert(t, invoked, true, "x-a")
		exprFilterConfirmSendS0(t, engine, "-", "")
		exprFilterConfirmSendS1(t, engine, "-", "b", "", "")
		exprFilterConfirmAssert(t, invoked, false, "dash-b")
		exprFilterConfirmSendS1(t, engine, "a", "b", "", "")
		exprFilterConfirmAssert(t, invoked, true, "dash-a-b")
	})

	t.Run("two-path-or-with-ll-v", func(t *testing.T) {
		env := newExprFilterConfirmEnvironment(t)
		filter := Or(
			Equal[string](Field[exprFilterConfirmS1, string]("p10"), Literal("a")),
			Or(
				Equal[string](Field[exprFilterConfirmS1, string]("p11"), Literal("b")),
				Equal[string](TagField[string]("s0", "p00"), Literal("x")),
			),
		)
		engine, invoked := exprFilterConfirmPattern(t, env, filter)
		exprFilterConfirmSendS0(t, engine, "x", "")
		exprFilterConfirmSendS1(t, engine, "-", "-", "", "")
		exprFilterConfirmAssert(t, invoked, true, "x-none")
		exprFilterConfirmSendS0(t, engine, "y", "")
		exprFilterConfirmSendS1(t, engine, "-", "-", "", "")
		exprFilterConfirmAssert(t, invoked, false, "y-none")
		exprFilterConfirmSendS1(t, engine, "a", "-", "", "")
		exprFilterConfirmAssert(t, invoked, true, "y-a")
		exprFilterConfirmSendS0(t, engine, "y", "")
		exprFilterConfirmSendS1(t, engine, "-", "-", "", "")
		exprFilterConfirmAssert(t, invoked, false, "y2-none")
		exprFilterConfirmSendS1(t, engine, "-", "b", "", "")
		exprFilterConfirmAssert(t, invoked, true, "y-b")
	})

	t.Run("one-path-or-with-l-v-v", func(t *testing.T) {
		env := newExprFilterConfirmEnvironment(t)
		filter := Or(
			Equal[string](Field[exprFilterConfirmS1, string]("p10"), Literal("a")),
			Or(
				Equal[string](TagField[string]("s0", "p00"), Literal("x")),
				Equal[string](TagField[string]("s0", "p01"), Literal("y")),
			),
		)
		engine, invoked := exprFilterConfirmPattern(t, env, filter)
		exprFilterConfirmSendS0(t, engine, "x", "-")
		exprFilterConfirmSendS1(t, engine, "-", "", "", "")
		exprFilterConfirmAssert(t, invoked, true, "x-none")
		exprFilterConfirmSendS0(t, engine, "-", "y")
		exprFilterConfirmSendS1(t, engine, "-", "", "", "")
		exprFilterConfirmAssert(t, invoked, true, "y-none")
		exprFilterConfirmSendS0(t, engine, "-", "-")
		exprFilterConfirmSendS1(t, engine, "-", "", "", "")
		exprFilterConfirmAssert(t, invoked, false, "dash-none")
		exprFilterConfirmSendS1(t, engine, "a", "", "", "")
		exprFilterConfirmAssert(t, invoked, true, "dash-a")
	})
}
