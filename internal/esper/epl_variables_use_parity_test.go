package esper

import (
	"context"
	"testing"
)

// Parity coverage for the variable-use executions of EPLVariablesUse:
//
// - EPLVariableUseVariableInFilter: on-event set variable + filter with variable equality
// - EPLVariableUseVariableInFilterBoolean: on-event set two variables + filter with OR
// - EPLVariableUseSimpleSameModule: create variable + select it in the same module

type variableUseBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type variableUseS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

func newVariableUseEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[variableUseBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[variableUseS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	return env
}

func variableUseSubscribe(t *testing.T, deployment *Deployment) *[]Result {
	t.Helper()
	got := &[]Result{}
	_, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*got = append(*got, batch.New...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestEPLVariableUseVariableInFilterParity covers EPLVariableUseVariableInFilter:
// on SupportBean_S0 set var1IF = p00; select theString, intPrimitive from
// SupportBean(theString = var1IF). The variable starts with a sentinel value
// (Java starts null); after S0 arrives with p00="a", the filter matches
// SupportBean(theString="a") but not others.
// Java runtime: java-runtime-5a38cfa84dadd7dd6f61.
func TestEPLVariableUseVariableInFilterParity(t *testing.T) {
	env := newVariableUseEnv(t)
	engine := NewEngine(env)

	// Use a sentinel initial value to simulate Java's null variable state.
	if err := env.RegisterVariable("var1IF", "\x00UNSET"); err != nil {
		t.Fatal(err)
	}

	// Variable setter: on SupportBean_S0 set var1IF = p00
	setterPlan, err := env.Build(
		OnEvent(From[variableUseS0](env, "SupportBean_S0")).SetVariables(
			SetVariableExpr("var1IF", Field[variableUseS0, string]("p00")),
		).Query(StatementName("set")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), setterPlan); err != nil {
		t.Fatal(err)
	}

	// Filter: select theString, intPrimitive from SupportBean(theString = var1IF)
	filterPlan, err := env.Build(
		Select(
			From[variableUseBean](env, "SupportBean").Filter(
				Equal[string](Field[variableUseBean, string]("theString"), VariableRef[string]("var1IF")),
			),
			Alias("theString", Field[variableUseBean, string]("theString")),
			Alias("intPrimitive", Field[variableUseBean, int]("intPrimitive")),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	filterDeployment, err := engine.Deploy(context.Background(), filterPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got := variableUseSubscribe(t, filterDeployment)

	// Variable is sentinel; no match
	if err := engine.SendEvent(context.Background(), variableUseBean{TheString: "", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 0 {
		t.Fatalf("expected 0 results before variable set, got %d", len(*got))
	}

	// Set variable to "a"
	if err := engine.SendEvent(context.Background(), variableUseS0{ID: 100, P00: "a", P01: "b"}); err != nil {
		t.Fatal(err)
	}
	// Now theString="a" matches
	if err := engine.SendEvent(context.Background(), variableUseBean{TheString: "a", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result after variable set, got %d", len(*got))
	}
	row := (*got)[0]
	if row.Get("theString").Any() != "a" || row.Get("intPrimitive").Any() != 2 {
		t.Fatalf("row = %#v", row)
	}

	// theString="b" doesn't match (variable is "a")
	if err := engine.SendEvent(context.Background(), variableUseBean{TheString: "b", IntPrimitive: 3}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected still 1 result, got %d", len(*got))
	}

	// Update variable to "e"
	if err := engine.SendEvent(context.Background(), variableUseS0{ID: 100, P00: "e", P01: "c"}); err != nil {
		t.Fatal(err)
	}
	// theString="e" now matches
	if err := engine.SendEvent(context.Background(), variableUseBean{TheString: "e", IntPrimitive: 6}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 2 {
		t.Fatalf("expected 2 results after variable update, got %d", len(*got))
	}
	row = (*got)[1]
	if row.Get("theString").Any() != "e" || row.Get("intPrimitive").Any() != 6 {
		t.Fatalf("row = %#v", row)
	}

	// theString="c" doesn't match (variable is "e")
	if err := engine.SendEvent(context.Background(), variableUseBean{TheString: "c", IntPrimitive: 5}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 2 {
		t.Fatalf("expected still 2 results, got %d", len(*got))
	}
}

// TestEPLVariableUseVariableInFilterBooleanParity covers EPLVariableUseVariableInFilterBoolean:
// on SupportBean_S0 set var1IFB = p00, var2IFB = p01; select from
// SupportBean(theString = var1IFB or theString = var2IFB).
// Java runtime: java-runtime-849ebec4996c28823d57.
func TestEPLVariableUseVariableInFilterBooleanParity(t *testing.T) {
	env := newVariableUseEnv(t)
	engine := NewEngine(env)

	if err := env.RegisterVariable("var1IFB", "\x00UNSET"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var2IFB", "\x00UNSET"); err != nil {
		t.Fatal(err)
	}

	setterPlan, err := env.Build(
		OnEvent(From[variableUseS0](env, "SupportBean_S0")).SetVariables(
			SetVariableExpr("var1IFB", Field[variableUseS0, string]("p00")),
			SetVariableExpr("var2IFB", Field[variableUseS0, string]("p01")),
		).Query(StatementName("set")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), setterPlan); err != nil {
		t.Fatal(err)
	}

	filterPlan, err := env.Build(
		Select(
			From[variableUseBean](env, "SupportBean").Filter(
				Or(
					Equal[string](Field[variableUseBean, string]("theString"), VariableRef[string]("var1IFB")),
					Equal[string](Field[variableUseBean, string]("theString"), VariableRef[string]("var2IFB")),
				),
			),
			Alias("theString", Field[variableUseBean, string]("theString")),
			Alias("intPrimitive", Field[variableUseBean, int]("intPrimitive")),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	filterDeployment, err := engine.Deploy(context.Background(), filterPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got := variableUseSubscribe(t, filterDeployment)

	// No match before variables set
	if err := engine.SendEvent(context.Background(), variableUseBean{TheString: "x", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 0 {
		t.Fatalf("expected 0 results before variables set, got %d", len(*got))
	}

	// Set var1IFB="a", var2IFB="b"
	if err := engine.SendEvent(context.Background(), variableUseS0{ID: 100, P00: "a", P01: "b"}); err != nil {
		t.Fatal(err)
	}
	// theString="a" matches var1IFB
	if err := engine.SendEvent(context.Background(), variableUseBean{TheString: "a", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result for 'a', got %d", len(*got))
	}
	// theString="b" matches var2IFB
	if err := engine.SendEvent(context.Background(), variableUseBean{TheString: "b", IntPrimitive: 3}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 2 {
		t.Fatalf("expected 2 results for 'b', got %d", len(*got))
	}
	// theString="c" doesn't match either
	if err := engine.SendEvent(context.Background(), variableUseBean{TheString: "c", IntPrimitive: 4}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 2 {
		t.Fatalf("expected still 2 results for 'c', got %d", len(*got))
	}

	// Update variables to "e","c"
	if err := engine.SendEvent(context.Background(), variableUseS0{ID: 100, P00: "e", P01: "c"}); err != nil {
		t.Fatal(err)
	}
	// theString="c" now matches var2IFB
	if err := engine.SendEvent(context.Background(), variableUseBean{TheString: "c", IntPrimitive: 5}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 3 {
		t.Fatalf("expected 3 results for 'c' after update, got %d", len(*got))
	}
	// theString="e" matches var1IFB
	if err := engine.SendEvent(context.Background(), variableUseBean{TheString: "e", IntPrimitive: 6}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 4 {
		t.Fatalf("expected 4 results for 'e' after update, got %d", len(*got))
	}
}

// TestEPLVariableUseSimpleSameModuleParity covers EPLVariableUseSimpleSameModule:
// create variable boolean var_simple_module_const = true; select
// var_simple_module_const as c0 from SupportBean.
// Java runtime: java-runtime-5a91cdbc149502c6fb7a.
func TestEPLVariableUseSimpleSameModuleParity(t *testing.T) {
	env := newVariableUseEnv(t)
	engine := NewEngine(env)

	if err := env.RegisterVariable("var_simple_module_const", true); err != nil {
		t.Fatal(err)
	}

	plan, err := env.Build(
		Select(
			From[variableUseBean](env, "SupportBean"),
			Alias("c0", VariableRef[bool]("var_simple_module_const")),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got := variableUseSubscribe(t, deployment)

	if err := engine.SendEvent(context.Background(), variableUseBean{TheString: "E1", IntPrimitive: 0}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(*got))
	}
	row := (*got)[0]
	if row.Get("c0").Any() != true {
		t.Fatalf("c0 = %#v, want true", row.Get("c0").Any())
	}
}
