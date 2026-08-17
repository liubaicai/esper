package esper

import (
	"context"
	"testing"
)

// Parity coverage for the remaining EPLVariablesOnSet executions not covered
// by case.epl-variable-onset-basic. The Compile/ObjectModel pair is one
// semantic scenario (typed on-set + dependent select) since Go has no
// EPL-text or object-model compile path. RUNTIMEOPS variants
// (SubqueryMultikeyWArray, ArrayAtIndex, Expression) and the indexed
// assignment executions (ArrayBoxed, ArrayInvalid) require array-index
// assignment targets not yet in the Go API and are not part of this slice.
//
// - EPLVariableOnSetCompile/ObjectModel: typed double/long on-set + select
// - EPLVariableOnSetSimpleSceneTwo: @public multi-module vars + irstream select
// - EPLVariableOnSetSubquery: subquery on set RHS with empty→null semantics

type onSetS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
	P11 string `esper:"p11"`
}

type onSetMarket struct {
	Symbol string `esper:"symbol"`
}

func registerOnSetS1Type(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterStruct[onSetS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}
}

// TestVariableOnSetTypedSelectParity mirrors the shared semantic scenario of
// EPLVariableOnSetCompile and EPLVariableOnSetObjectModel: preconfigured
// typed variables var1C (double) / var2C (long) are set by an on SupportBean
// trigger and observed by a dependent select on SupportBean_A.
// Java runtimes: java-runtime-2735f44aed732350be1b (Compile),
// java-runtime-60c4c5dc59ffbf4277b3 (ObjectModel).
func TestVariableOnSetTypedSelectParity(t *testing.T) {
	env := NewEnvironment()
	registerOnSetVarTypes(t, env)
	if err := env.RegisterVariable("var1C", 10.0); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var2C", int64(11)); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// s0: select var1C, var2C, id from SupportBean_A
	selectPlan, err := env.Build(
		Select(From[onSetVarA](env, "SupportBean_A"),
			Alias("var1C", VariableRef[float64]("var1C")),
			Alias("var2C", VariableRef[int64]("var2C")),
			Alias("id", Field[onSetVarA, string]("id")),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	selectDeployment := mustDeployOnSetVar(t, engine, selectPlan)
	selectBatches := subscribeOnSetVarCapture(t, selectDeployment.Statements()[0])

	// set: on SupportBean set var1C=intPrimitive, var2C=intBoxed
	setPlan, err := env.Build(
		OnEvent(From[onSetVarBean](env, "SupportBean")).SetVariables(
			SetVariableExpr("var1C", Field[onSetVarBean, int]("intPrimitive")),
			SetVariableExpr("var2C", Field[onSetVarBean, int]("intBoxed")),
		).Query(StatementName("set")),
	)
	if err != nil {
		t.Fatal(err)
	}
	setDeployment := mustDeployOnSetVar(t, engine, setPlan)
	setBatches := subscribeOnSetVarCapture(t, setDeployment.Statements()[0])

	// SB_A(E1) → s0 sees initial values {10.0, 11, "E1"}
	if err := engine.SendEvent(context.Background(), onSetVarA{ID: "E1"}); err != nil {
		t.Fatal(err)
	}
	row := lastOnSetVarRow(t, *selectBatches)
	if row.Get("var1C").Any() != 10.0 || row.Get("var2C").Any() != int64(11) || row.Get("id").Any() != "E1" {
		t.Fatalf("s0 E1 row = %v/%v/%v", row.Get("var1C").Any(), row.Get("var2C").Any(), row.Get("id").Any())
	}

	// SB(S1,3,4) → set trigger fires with {3.0, 4}
	if err := engine.SendEvent(context.Background(), onSetVarBean{TheString: "S1", IntPrimitive: 3, IntBoxed: 4}); err != nil {
		t.Fatal(err)
	}
	setRow := lastOnSetVarRow(t, *setBatches)
	if setRow.Get("var1C").Any() != 3.0 || setRow.Get("var2C").Any() != int64(4) {
		t.Fatalf("set row = %v/%v", setRow.Get("var1C").Any(), setRow.Get("var2C").Any())
	}

	// SB_A(E2) → s0 sees updated values {3.0, 4, "E2"}
	if err := engine.SendEvent(context.Background(), onSetVarA{ID: "E2"}); err != nil {
		t.Fatal(err)
	}
	row = lastOnSetVarRow(t, *selectBatches)
	if row.Get("var1C").Any() != 3.0 || row.Get("var2C").Any() != int64(4) || row.Get("id").Any() != "E2" {
		t.Fatalf("s0 E2 row = %v/%v/%v", row.Get("var1C").Any(), row.Get("var2C").Any(), row.Get("id").Any())
	}
}

// TestVariableOnSetSimpleSceneTwoParity mirrors EPLVariableOnSetSimpleSceneTwo:
// two @public integer variables shared across deployments; an on SupportBean
// trigger sets both from intPrimitive; an irstream select on
// SupportMarketDataBean observes current values.
// Java runtime: java-runtime-91f1de1b06fddf87b2b1.
func TestVariableOnSetSimpleSceneTwoParity(t *testing.T) {
	env := NewEnvironment()
	registerOnSetVarTypes(t, env)
	if _, err := RegisterStruct[onSetMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("resvar", 1); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("durvar", 10); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// s2: select irstream resvar, durvar, symbol from SupportMarketDataBean
	selectPlan, err := env.Build(
		Select(From[onSetMarket](env, "SupportMarketDataBean"),
			Alias("resvar", VariableRef[int]("resvar")),
			Alias("durvar", VariableRef[int]("durvar")),
			Alias("symbol", Field[onSetMarket, string]("symbol")),
		).Query(StatementName("s2"), WithOldStream()),
	)
	if err != nil {
		t.Fatal(err)
	}
	selectDeployment := mustDeployOnSetVar(t, engine, selectPlan)
	selectBatches := subscribeOnSetVarCapture(t, selectDeployment.Statements()[0])

	// s1: on SupportBean set resvar=intPrimitive, durvar=intPrimitive
	setPlan, err := env.Build(
		OnEvent(From[onSetVarBean](env, "SupportBean")).SetVariables(
			SetVariableExpr("resvar", Field[onSetVarBean, int]("intPrimitive")),
			SetVariableExpr("durvar", Field[onSetVarBean, int]("intPrimitive")),
		).Query(StatementName("s1")),
	)
	if err != nil {
		t.Fatal(err)
	}
	setDeployment := mustDeployOnSetVar(t, engine, setPlan)
	setBatches := subscribeOnSetVarCapture(t, setDeployment.Statements()[0])

	// MktData(E1) → s2 new {resvar:1, durvar:10, symbol:E1}
	if err := engine.SendEvent(context.Background(), onSetMarket{Symbol: "E1"}); err != nil {
		t.Fatal(err)
	}
	row := lastOnSetVarRow(t, *selectBatches)
	if row.Get("resvar").Any() != 1 || row.Get("durvar").Any() != 10 || row.Get("symbol").Any() != "E1" {
		t.Fatalf("s2 E1 row = %v/%v/%v", row.Get("resvar").Any(), row.Get("durvar").Any(), row.Get("symbol").Any())
	}

	// SB("",20) → set fires {20,20}
	if err := engine.SendEvent(context.Background(), onSetVarBean{TheString: "", IntPrimitive: 20}); err != nil {
		t.Fatal(err)
	}
	setRow := lastOnSetVarRow(t, *setBatches)
	if setRow.Get("resvar").Any() != 20 || setRow.Get("durvar").Any() != 20 {
		t.Fatalf("set row = %v/%v", setRow.Get("resvar").Any(), setRow.Get("durvar").Any())
	}

	// MktData(E2) → s2 {20,20,E2}
	if err := engine.SendEvent(context.Background(), onSetMarket{Symbol: "E2"}); err != nil {
		t.Fatal(err)
	}
	row = lastOnSetVarRow(t, *selectBatches)
	if row.Get("resvar").Any() != 20 || row.Get("durvar").Any() != 20 || row.Get("symbol").Any() != "E2" {
		t.Fatalf("s2 E2 row = %v/%v/%v", row.Get("resvar").Any(), row.Get("durvar").Any(), row.Get("symbol").Any())
	}

	// SB("",1000) then MktData(E3) → s2 {1000,1000,E3}
	if err := engine.SendEvent(context.Background(), onSetVarBean{TheString: "", IntPrimitive: 1000}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), onSetMarket{Symbol: "E3"}); err != nil {
		t.Fatal(err)
	}
	row = lastOnSetVarRow(t, *selectBatches)
	if row.Get("resvar").Any() != 1000 || row.Get("durvar").Any() != 1000 || row.Get("symbol").Any() != "E3" {
		t.Fatalf("s2 E3 row = %v/%v/%v", row.Get("resvar").Any(), row.Get("durvar").Any(), row.Get("symbol").Any())
	}
}

// TestVariableOnSetSubqueryParity mirrors EPLVariableOnSetSubquery: an
// on-trigger assigns string variables from scalar subqueries over
// SupportBean_S1#lastevent; an empty subquery result assigns null.
// Java runtime: java-runtime-1588aa4d361615645c7e.
func TestVariableOnSetSubqueryParity(t *testing.T) {
	env := NewEnvironment()
	registerOnSetVarTypes(t, env)
	registerOnSetS1Type(t, env)
	if err := env.RegisterVariable("var1SS", "a"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var2SS", "b"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// on SupportBean_S0 set
	//   var1SS = (select p10 from SupportBean_S1#lastevent),
	//   var2SS = (select p11 || s0str.p01 from SupportBean_S1#lastevent)
	// The trigger stream alias correlation (s0str.p01) is expressed via the
	// current trigger event field inside the concatenation expression.
	s1Window := From[onSetS1](env, "SupportBean_S1").Window(LastEvent()).AsRecord()
	var1Sub := MapValue[string](SubqueryRow(s1Window, Alias("p10", Field[onSetS1, string]("p10"))), Literal("p10"))
	var2Sub := MapValue[string](SubqueryRow(s1Window, Alias("p11", Field[onSetS1, string]("p11"))), Literal("p11"))

	setPlan, err := env.Build(
		OnEvent(From[onSetVarS0](env, "SupportBean_S0")).SetVariables(
			SetVariableExpr("var1SS", var1Sub),
			SetVariableExpr("var2SS", Concat(var2Sub, Field[onSetVarS0, string]("p01"))),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	setDeployment := mustDeployOnSetVar(t, engine, setPlan)
	setBatches := subscribeOnSetVarCapture(t, setDeployment.Statements()[0])

	// Initial snapshot: {a, b}
	if v, ok := engine.GetVariable("var1SS"); !ok || v.Any() != "a" {
		t.Fatalf("initial var1SS = %v (ok=%v)", v.Any(), ok)
	}
	if v, ok := engine.GetVariable("var2SS"); !ok || v.Any() != "b" {
		t.Fatalf("initial var2SS = %v (ok=%v)", v.Any(), ok)
	}

	// SB_S0(1) with empty S1 window → subquery null → both become null
	if err := engine.SendEvent(context.Background(), onSetVarS0{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if v, ok := engine.GetVariable("var1SS"); !ok || v.IsPresent() {
		t.Fatalf("var1SS after empty subquery = %v, want null", v.Any())
	}
	if v, ok := engine.GetVariable("var2SS"); !ok || v.IsPresent() {
		t.Fatalf("var2SS after empty subquery = %v, want null", v.Any())
	}

	// SB_S1(0,"x","y") populates the lastevent window
	if err := engine.SendEvent(context.Background(), onSetS1{ID: 0, P10: "x", P11: "y"}); err != nil {
		t.Fatal(err)
	}
	// SB_S0(1,"1","2") → var1SS="x", var2SS="y"||"2"="y2"
	if err := engine.SendEvent(context.Background(), onSetVarS0{ID: 1, P00: "1", P01: "2"}); err != nil {
		t.Fatal(err)
	}
	if v, ok := engine.GetVariable("var1SS"); !ok || v.Any() != "x" {
		t.Fatalf("var1SS = %v, want x", v.Any())
	}
	if v, ok := engine.GetVariable("var2SS"); !ok || v.Any() != "y2" {
		t.Fatalf("var2SS = %v, want y2", v.Any())
	}

	// Set-listener batch carries the new values
	setRow := lastOnSetVarRow(t, *setBatches)
	if setRow.Get("var1SS").Any() != "x" || setRow.Get("var2SS").Any() != "y2" {
		t.Fatalf("set batch row = %v/%v", setRow.Get("var1SS").Any(), setRow.Get("var2SS").Any())
	}
}
