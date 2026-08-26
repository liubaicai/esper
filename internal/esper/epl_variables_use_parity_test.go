package esper

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

// Parity coverage for the variable-use executions of EPLVariablesUse:
//
// - EPLVariableUseVariableInFilter: on-event set variable + filter with variable equality
// - EPLVariableUseVariableInFilterBoolean: on-event set two variables + filter with OR
// - EPLVariableUseSimpleSameModule: create variable + select it in the same module
// - EPLVariableUseEPRuntime: runtime variable API, coercion matrix, rollback proofs
// - EPLVariableUseConstantVariable: constant variables across filter truth tables

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

// variableUseEPRuntimeBean mirrors the SupportBean shape used by
// EPLVariableUseEPRuntime: theString arrives nullable (Java null) so the
// on-set statement can drive var2 through a null round-trip.
type variableUseEPRuntimeBean struct {
	TheString    *string `esper:"theString"`
	IntPrimitive int     `esper:"intPrimitive"`
}

// variableUseConstantEnum is the named comparable Go representation of
// SupportEnum (ENUM_VALUE_1/2/3).
type variableUseConstantEnum string

const (
	variableUseEnumValueOne   variableUseConstantEnum = "ENUM_VALUE_1"
	variableUseEnumValueTwo   variableUseConstantEnum = "ENUM_VALUE_2"
	variableUseEnumValueThree variableUseConstantEnum = "ENUM_VALUE_3"
)

// variableUseConstantBean mirrors the SupportBean shape used by
// EPLVariableUseConstantVariable: boxed Integer/Short properties are Go
// pointer fields, enumValue keeps the named comparable enum type.
type variableUseConstantBean struct {
	TheString    string                  `esper:"theString"`
	IntPrimitive int                     `esper:"intPrimitive"`
	IntBoxed     *int32                  `esper:"intBoxed"`
	ShortBoxed   *int16                  `esper:"shortBoxed"`
	EnumValue    variableUseConstantEnum `esper:"enumValue"`
}

// variableUseValue maps a plain Go value onto the engine Value space, with
// nil representing Java null.
func variableUseValue(value any) Value {
	if value == nil {
		return Null()
	}
	return Present(value)
}

// TestEPLVariableUseEPRuntimeParity covers EPLVariableUseEPRuntime: the
// runtime variable API against the preconfigured fixture (var1 int = -1,
// var2 string = "abc"), including introspection, single and bulk sets,
// byte/short coercion, verbatim failure messages, create-on-the-fly
// variables and both rollback proofs.
// Java runtime: java-runtime-826b551e883c9398df67.
func TestEPLVariableUseEPRuntimeParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[variableUseEPRuntimeBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	// Preconfigured variables mirror TestSuiteEPLVariable.configure.
	if err := env.RegisterVariable("var1", -1); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var2", "abc"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer engine.Close(context.Background())
	ctx := context.Background()

	// Step 1 - introspection: getVariableTypeAll and getVariableType.
	definition, ok := env.Variable("var1")
	if !ok || definition.Type() != typeOf[int]() {
		t.Fatalf("var1 type = %#v, %v", definition.Type(), ok)
	}
	if _, ok = env.Variable("var2"); !ok {
		t.Fatal("var2 was not registered")
	}
	variableTypes := make(map[string]reflect.Type)
	for _, entry := range env.Variables() {
		variableTypes[entry.Name()] = entry.Type()
	}
	if variableTypes["var1"] != typeOf[int]() || variableTypes["var2"] != typeOf[string]() {
		t.Fatalf("variable type all = %#v", variableTypes)
	}

	// Step 2 - deploy "on SupportBean set var1 = intPrimitive, var2 = theString".
	setterPlan, err := env.Build(
		OnEvent(From[variableUseEPRuntimeBean](env, "SupportBean")).SetVariables(
			SetVariableExpr("var1", Field[variableUseEPRuntimeBean, int]("intPrimitive")),
			SetVariableExpr("var2", Cast[*string, string](Field[variableUseEPRuntimeBean, *string]("theString"))),
		).Query(StatementName("set")),
	)
	if err != nil {
		t.Fatal(err)
	}
	setterDeployment, err := engine.Deploy(ctx, setterPlan)
	if err != nil {
		t.Fatal(err)
	}

	// Step 3 - initial values observed through three channels: single reads,
	// the full snapshot map, and the by-request value set.
	assertPreconfigured := func(channel string, wantVar1 any, wantVar2 any) {
		t.Helper()
		expected := map[string]Value{"var1": variableUseValue(wantVar1), "var2": variableUseValue(wantVar2)}
		for name, want := range expected {
			got, ok := engine.GetVariable(name)
			if !ok || !got.Equal(want) {
				t.Fatalf("%s: %s = %#v, want %#v", channel, name, got, want)
			}
		}
		all := engine.Variables()
		requested, err := engine.VariableValues(ctx, "var1", "var2")
		if err != nil {
			t.Fatalf("%s: %v", channel, err)
		}
		for name, want := range expected {
			if !all[name].Equal(want) {
				t.Fatalf("%s: snapshot %s = %#v, want %#v", channel, name, all[name], want)
			}
			if !requested[name].Equal(want) {
				t.Fatalf("%s: request %s = %#v, want %#v", channel, name, requested[name], want)
			}
		}
	}
	assertPreconfigured("initial", -1, "abc")

	// Step 4 - bean(null, 99) fires the on-set set clause.
	if err := engine.SendEvent(ctx, variableUseEPRuntimeBean{IntPrimitive: 99}); err != nil {
		t.Fatal(err)
	}
	assertPreconfigured("on-set", 99, nil)

	// Step 5 - runtimeSetVariable with milestone boundaries in between
	// (milestones have no observable Go counterpart).
	if err := engine.SetVariable(ctx, "var2", "def"); err != nil {
		t.Fatal(err)
	}
	assertPreconfigured("after def", 99, "def")
	if err := engine.SetVariable(ctx, "var1", 123); err != nil {
		t.Fatal(err)
	}
	assertPreconfigured("after 123", 123, "def")

	// Step 6 - bulk sets: plain int, byte coerced to Integer, then nulls.
	if err := engine.SetVariables(ctx, VariableAssignment{Name: "var1", Value: 20}); err != nil {
		t.Fatal(err)
	}
	assertPreconfigured("bulk int", 20, "def")
	if err := engine.SetVariables(ctx,
		VariableAssignment{Name: "var1", Value: byte(21)},
		VariableAssignment{Name: "var2", Value: "test"},
	); err != nil {
		t.Fatal(err)
	}
	assertPreconfigured("bulk byte coercion", 21, "test")
	if err := engine.SetVariables(ctx,
		VariableAssignment{Name: "var1", Value: nil},
		VariableAssignment{Name: "var2", Value: nil},
	); err != nil {
		t.Fatal(err)
	}
	assertPreconfigured("bulk null round-trip", nil, nil)

	// Step 7 - unknown-name failures carry the Java message verbatim.
	assertVariableRuntimeError(t,
		engine.SetVariable(ctx, "dummy", nil),
		ErrorUnknownName, "Variable by name 'dummy' has not been declared")
	assertVariableRuntimeError(t,
		engine.SetVariables(ctx, VariableAssignment{Name: "dummy2", Value: 20}),
		ErrorUnknownName, "Variable by name 'dummy2' has not been declared")

	// Step 8 - create-on-the-fly: "@name('create') create variable int dummy
	// = 20 + 20". Variables are environment-scoped in Go, so creation
	// evaluates the initializer through the same expression evaluator and
	// registers the result; the deployment-scoped read collapses to a direct
	// getVariableValue equivalent.
	initPlan, err := env.Build(SelectOnce(env, Alias("initializer", Add[int](Literal(20), Literal(20)))))
	if err != nil {
		t.Fatal(err)
	}
	initResult, err := engine.ExecuteFireAndForget(ctx, initPlan)
	if err != nil {
		t.Fatal(err)
	}
	initRow, ok := initResult.Results()[0].Row()
	if !ok || initRow.Get("initializer").Any() != 40 {
		t.Fatalf("initializer row = %#v", initResult.Results()[0])
	}
	if err := env.RegisterVariable("dummy", initRow.Get("initializer").Any()); err != nil {
		t.Fatal(err)
	}
	dummy, ok := engine.GetVariable("dummy")
	if !ok || !dummy.Equal(Present(40)) {
		t.Fatalf("dummy = %#v, %v", dummy, ok)
	}

	// Step 9 - declared-type mismatches carry the Java messages verbatim.
	assertVariableRuntimeError(t,
		engine.SetVariable(ctx, "dummy", "abc"),
		ErrorTypeMismatch, "Variable 'dummy' of declared type Integer cannot be assigned a value of type String")
	assertVariableRuntimeError(t,
		engine.SetVariable(ctx, "dummy", int64(100)),
		ErrorTypeMismatch, "Variable 'dummy' of declared type Integer cannot be assigned a value of type Long")
	assertVariableRuntimeError(t,
		engine.SetVariable(ctx, "var2", 0),
		ErrorTypeMismatch, "Variable 'var2' of declared type String cannot be assigned a value of type Integer")

	// Step 10 - short coercion accepted into the Integer variable.
	if err := engine.SetVariable(ctx, "var1", int16(-1)); err != nil {
		t.Fatal(err)
	}
	assertPreconfigured("short coercion", -1, nil)

	// Step 11 - rollback on failed coercion in an ordered batch.
	assertVariableRuntimeError(t,
		engine.SetVariables(ctx,
			VariableAssignment{Name: "var2", Value: "xyz"},
			VariableAssignment{Name: "var1", Value: 4.4},
		),
		ErrorTypeMismatch, "Variable 'var1' of declared type Integer cannot be assigned a value of type Double")
	assertPreconfigured("rollback coercion", -1, nil)

	// Step 12 - rollback on unknown name in an ordered batch.
	assertVariableRuntimeError(t,
		engine.SetVariables(ctx,
			VariableAssignment{Name: "var2", Value: "xyz"},
			VariableAssignment{Name: "var1", Value: 1},
			VariableAssignment{Name: "notfoundvariable", Value: nil},
		),
		ErrorUnknownName, "Variable by name 'notfoundvariable' has not been declared")
	assertPreconfigured("rollback unknown", -1, nil)

	// Step 13 - undeployAll.
	if err := setterDeployment.Undeploy(ctx); err != nil {
		t.Fatal(err)
	}
}

// variableUseOperatorRow is one tryOperator truth-table row: the bean to
// send and whether the deployed filter statement must fire.
type variableUseOperatorRow struct {
	bean variableUseConstantBean
	want bool
}

func variableUseIntRow(value int32, want bool) variableUseOperatorRow {
	return variableUseOperatorRow{bean: variableUseConstantBean{TheString: "S", IntPrimitive: 1, IntBoxed: &value}, want: want}
}

func variableUseNullIntRow(want bool) variableUseOperatorRow {
	return variableUseOperatorRow{bean: variableUseConstantBean{TheString: "S", IntPrimitive: 1}, want: want}
}

func variableUseShortRow(value int16, want bool) variableUseOperatorRow {
	return variableUseOperatorRow{bean: variableUseConstantBean{TheString: "S", IntPrimitive: 1, ShortBoxed: &value}, want: want}
}

func variableUseNullShortRow(want bool) variableUseOperatorRow {
	return variableUseOperatorRow{bean: variableUseConstantBean{TheString: "S", IntPrimitive: 1}, want: want}
}

func variableUseEnumRow(value variableUseConstantEnum, want bool) variableUseOperatorRow {
	return variableUseOperatorRow{bean: variableUseConstantBean{TheString: "S", IntPrimitive: 1, EnumValue: value}, want: want}
}

func variableUseStringRow(value string, want bool) variableUseOperatorRow {
	return variableUseOperatorRow{bean: variableUseConstantBean{TheString: value, IntPrimitive: 1}, want: want}
}

// assertResultSchemaType observes a statement output property type, the Go
// counterpart of Java's statement.getPropertyType(name) assertions.
func assertResultSchemaType(t *testing.T, plan Plan, fieldName string, want reflect.Type) {
	t.Helper()
	schema, ok := plan.ResultSchema()
	if !ok {
		t.Fatalf("plan has no result schema for %q", fieldName)
	}
	for _, field := range schema.Fields() {
		if field.Name == fieldName {
			if field.Type != want {
				t.Fatalf("%s property type = %s, want %s", fieldName, field.Type, want)
			}
			return
		}
	}
	t.Fatalf("result schema has no property %q", fieldName)
}

// TestEPLVariableUseConstantVariableParity covers EPLVariableUseConstantVariable:
// a constant Integer variable across the full filter operator truth table,
// short coercion rows, literal-list and array-constant membership filters,
// enum constants, compile-time and API constant protection with verbatim
// messages, the ESPER-653 date presence marker and a final non-constant
// enum variable set by an on-set statement that never fires.
// Java runtime: java-runtime-d273a38f6415e6c3ee62.
func TestEPLVariableUseConstantVariableParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[variableUseConstantBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer engine.Close(context.Background())
	ctx := context.Background()

	// "@public create const variable int MYCONST = 10".
	if err := env.RegisterVariable("MYCONST", int32(10), ConstantVariable()); err != nil {
		t.Fatal(err)
	}

	myconst := VariableRef[int32]("MYCONST")
	intBoxedNullable := Field[variableUseConstantBean, *int32]("intBoxed")
	intBoxed := Cast[*int32, int32](intBoxedNullable)
	shortBoxed := Cast[*int16, int32](Field[variableUseConstantBean, *int16]("shortBoxed"))

	// tryOperator deploys one "select c0,c1 from SupportBean(<op>)"
	// statement per operator; each row sends exactly one SupportBean and the
	// listener must fire (true) or stay silent (false). Java's warm-up
	// SupportBean_S0 event belongs to a different event type and has no
	// effect here. The FilterItem op != BOOLEAN_EXPRESSION assertion is an
	// engine-internal filter-plan shape and is registered
	// intentionally-different for this port (InspectFilter precedent).
	tryOperator := func(description string, filter Expression[bool], rows []variableUseOperatorRow) {
		t.Helper()
		plan, err := env.Build(Select(
			From[variableUseConstantBean](env, "SupportBean").Filter(filter),
			Alias("c0", Field[variableUseConstantBean, string]("theString")),
			Alias("c1", Field[variableUseConstantBean, int]("intPrimitive")),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatalf("%s: build: %v", description, err)
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			t.Fatalf("%s: deploy: %v", description, err)
		}
		results := variableUseSubscribe(t, deployment)
		fired := 0
		for index, row := range rows {
			if err := engine.SendEvent(ctx, row.bean); err != nil {
				t.Fatalf("%s row %d: %v", description, index, err)
			}
			if row.want {
				fired++
			}
			if len(*results) != fired {
				t.Fatalf("%s row %d (%#v): results = %d, want fired = %#v",
					description, index, row.bean, len(*results), rows[index].want)
			}
		}
		if err := deployment.Undeploy(ctx); err != nil {
			t.Fatalf("%s: undeploy: %v", description, err)
		}
	}

	tryOperator("MYCONST = intBoxed", Equal[int32](myconst, intBoxed), []variableUseOperatorRow{
		variableUseIntRow(10, true), variableUseIntRow(9, false), variableUseNullIntRow(false),
	})
	tryOperator("MYCONST > intBoxed", Greater[int32](myconst, intBoxed), []variableUseOperatorRow{
		variableUseIntRow(11, false), variableUseIntRow(10, false), variableUseIntRow(9, true), variableUseIntRow(8, true),
	})
	tryOperator("MYCONST >= intBoxed", GreaterOrEqual[int32](myconst, intBoxed), []variableUseOperatorRow{
		variableUseIntRow(11, false), variableUseIntRow(10, true), variableUseIntRow(9, true), variableUseIntRow(8, true),
	})
	tryOperator("MYCONST < intBoxed", Less[int32](myconst, intBoxed), []variableUseOperatorRow{
		variableUseIntRow(11, true), variableUseIntRow(10, false), variableUseIntRow(9, false), variableUseIntRow(8, false),
	})
	tryOperator("MYCONST <= intBoxed", LessOrEqual[int32](myconst, intBoxed), []variableUseOperatorRow{
		variableUseIntRow(11, true), variableUseIntRow(10, true), variableUseIntRow(9, false), variableUseIntRow(8, false),
	})
	tryOperator("intBoxed < MYCONST", Less[int32](intBoxed, myconst), []variableUseOperatorRow{
		variableUseIntRow(11, false), variableUseIntRow(10, false), variableUseIntRow(9, true), variableUseIntRow(8, true),
	})
	tryOperator("intBoxed <= MYCONST", LessOrEqual[int32](intBoxed, myconst), []variableUseOperatorRow{
		variableUseIntRow(11, false), variableUseIntRow(10, true), variableUseIntRow(9, true), variableUseIntRow(8, true),
	})
	tryOperator("intBoxed > MYCONST", Greater[int32](intBoxed, myconst), []variableUseOperatorRow{
		variableUseIntRow(11, true), variableUseIntRow(10, false), variableUseIntRow(9, false), variableUseIntRow(8, false),
	})
	tryOperator("intBoxed >= MYCONST", GreaterOrEqual[int32](intBoxed, myconst), []variableUseOperatorRow{
		variableUseIntRow(11, true), variableUseIntRow(10, true), variableUseIntRow(9, false), variableUseIntRow(8, false),
	})
	tryOperator("intBoxed in (MYCONST)", In[int32](intBoxed, myconst), []variableUseOperatorRow{
		variableUseIntRow(11, false), variableUseIntRow(10, true), variableUseIntRow(9, false), variableUseIntRow(8, false),
	})
	tryOperator("intBoxed between MYCONST and MYCONST", Between[int32](intBoxed, myconst, myconst), []variableUseOperatorRow{
		variableUseIntRow(11, false), variableUseIntRow(10, true), variableUseIntRow(9, false), variableUseIntRow(8, false),
	})
	tryOperator("MYCONST != intBoxed", NotEqual[int32](myconst, intBoxed), []variableUseOperatorRow{
		variableUseIntRow(10, false), variableUseIntRow(9, true), variableUseNullIntRow(false),
	})
	tryOperator("intBoxed != MYCONST", NotEqual[int32](intBoxed, myconst), []variableUseOperatorRow{
		variableUseIntRow(10, false), variableUseIntRow(9, true), variableUseNullIntRow(false),
	})
	tryOperator("intBoxed not in (MYCONST)", NotInOf(intBoxed, myconst), []variableUseOperatorRow{
		variableUseIntRow(11, true), variableUseIntRow(10, false), variableUseIntRow(9, true), variableUseIntRow(8, true),
	})
	tryOperator("intBoxed not between MYCONST and MYCONST", NotBetweenOf(intBoxed, myconst, myconst), []variableUseOperatorRow{
		variableUseIntRow(11, true), variableUseIntRow(10, false), variableUseIntRow(9, true), variableUseIntRow(8, true),
	})
	// `is`/`is not` are null-aware equality forms. MYCONST is a non-null
	// constant, so `A is B` reduces to both-non-null equality and
	// `A is not B` to null-or-unequal.
	boxedNonNull := Not(IsNull[*int32](intBoxedNullable))
	tryOperator("MYCONST is intBoxed", And(boxedNonNull, Equal[int32](myconst, intBoxed)), []variableUseOperatorRow{
		variableUseIntRow(10, true), variableUseIntRow(9, false), variableUseNullIntRow(false),
	})
	tryOperator("intBoxed is MYCONST", And(boxedNonNull, Equal[int32](intBoxed, myconst)), []variableUseOperatorRow{
		variableUseIntRow(10, true), variableUseIntRow(9, false), variableUseNullIntRow(false),
	})
	tryOperator("MYCONST is not intBoxed", Or(IsNull[*int32](intBoxedNullable), NotEqual[int32](myconst, intBoxed)), []variableUseOperatorRow{
		variableUseIntRow(10, false), variableUseIntRow(9, true), variableUseNullIntRow(true),
	})
	tryOperator("intBoxed is not MYCONST", Or(IsNull[*int32](intBoxedNullable), NotEqual[int32](intBoxed, myconst)), []variableUseOperatorRow{
		variableUseIntRow(10, false), variableUseIntRow(9, true), variableUseNullIntRow(true),
	})

	// Short coercion rows S21-S25.
	tryOperator("MYCONST = shortBoxed", Equal[int32](myconst, shortBoxed), []variableUseOperatorRow{
		variableUseShortRow(10, true), variableUseShortRow(9, false), variableUseNullShortRow(false),
	})
	tryOperator("shortBoxed = MYCONST", Equal[int32](shortBoxed, myconst), []variableUseOperatorRow{
		variableUseShortRow(10, true), variableUseShortRow(9, false), variableUseNullShortRow(false),
	})
	tryOperator("MYCONST > shortBoxed", Greater[int32](myconst, shortBoxed), []variableUseOperatorRow{
		variableUseShortRow(11, false), variableUseShortRow(10, false), variableUseShortRow(9, true), variableUseShortRow(8, true),
	})
	tryOperator("shortBoxed < MYCONST", Less[int32](shortBoxed, myconst), []variableUseOperatorRow{
		variableUseShortRow(11, false), variableUseShortRow(10, false), variableUseShortRow(9, true), variableUseShortRow(8, true),
	})
	tryOperator("shortBoxed in (MYCONST)", In[int32](shortBoxed, myconst), []variableUseOperatorRow{
		variableUseShortRow(11, false), variableUseShortRow(10, true), variableUseShortRow(9, false), variableUseShortRow(8, false),
	})

	// SODA eplToModelCompileDeploy("create constant variable int MYCONST = 10")
	// exercises only Java's second compile path; the Go port keeps a single
	// typed compile path, recorded as a representation note.

	// Compile-time constant rejections carry the bare sentence verbatim; the
	// outer wrapper ("Failed to validate ...") is statement-level text the
	// typed builders do not reproduce.
	_, err := env.Build(
		OnEvent(From[variableUseConstantBean](env, "SupportBean")).SetVariables(
			SetVariableExpr("MYCONST", Literal(10)),
		).Query(StatementName("on-set-const")),
	)
	assertVariableBuildConstRejection(t, err, "Variable by name 'MYCONST' is declared constant and may not be set")
	_, err = env.Build(Select(
		From[variableUseConstantBean](env, "SupportBean"),
	).Query(
		StatementName("output-rate-const"),
		WithOutput(OutputWhen(Literal(true), SetOutputVariable("MYCONST", Literal(1)))),
	))
	assertVariableBuildConstRejection(t, err, "Variable by name 'MYCONST' is declared constant and may not be set")

	// API write protection for all three constant targets, single and bulk:
	// the deployed MYCONST plus preconfigured MYCONST_TWO/MYCONST_THREE.
	if err := env.RegisterVariable("MYCONST_TWO", nil, VariableType(typeOf[string]()), ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("MYCONST_THREE", true, ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	constantTargets := []struct {
		name  string
		value any
	}{
		{"MYCONST", 1},
		{"MYCONST_TWO", "dummy"},
		{"MYCONST_THREE", false},
	}
	for _, target := range constantTargets {
		message := fmt.Sprintf("Variable by name '%s' is declared as constant and may not be assigned a new value", target.name)
		assertVariableRuntimeError(t, engine.SetVariable(ctx, target.name, target.value), ErrorState, message)
		assertVariableRuntimeError(t,
			engine.SetVariables(ctx, VariableAssignment{Name: target.name, Value: target.value}),
			ErrorState, message)
	}

	// ESPER-653: START_TIME asserts presence only - Calendar.getInstance()
	// wall clocks are nondeterministic and excluded on both sides by design;
	// both emit a <date> marker instead of a timestamp.
	if err := env.RegisterVariable("START_TIME", time.Unix(0, 0).UTC(), ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	startTime, ok := engine.GetVariable("START_TIME")
	if !ok || !startTime.IsPresent() || startTime.IsNull() {
		t.Fatalf("START_TIME = %#v, %v", startTime, ok)
	}

	// Array constants: property-type observation through a deployed select.
	if err := env.RegisterVariable("var_strings", []string{"E1", "E2"}, ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	stringsPlan, err := env.Build(Select(
		From[variableUseConstantBean](env, "SupportBean"),
		Alias("var_strings", VariableRef[[]string]("var_strings")),
	).Query(StatementName("strings-type")))
	if err != nil {
		t.Fatal(err)
	}
	assertResultSchemaType(t, stringsPlan, "var_strings", reflect.TypeOf([]string{}))

	tryOperator("theString in (var_strings)",
		InSlice[string](Field[variableUseConstantBean, string]("theString"), VariableRef[[]string]("var_strings")),
		[]variableUseOperatorRow{
			variableUseStringRow("E1", true), variableUseStringRow("E2", true), variableUseStringRow("E3", false),
		})

	tryOperator("intBoxed in (10, 8)",
		In[int32](intBoxed, Literal(int32(10)), Literal(int32(8))),
		[]variableUseOperatorRow{
			variableUseIntRow(11, false), variableUseIntRow(10, true), variableUseIntRow(9, false), variableUseIntRow(8, true),
		})

	if err := env.RegisterVariable("var_ints", []int32{8, 10}, ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	varInts := VariableRef[[]int32]("var_ints")
	tryOperator("intBoxed in (var_ints)", InSlice[int32](intBoxed, varInts), []variableUseOperatorRow{
		variableUseIntRow(11, false), variableUseIntRow(10, true), variableUseIntRow(9, false), variableUseIntRow(8, true),
	})

	if err := env.RegisterVariable("var_intstwo", []int32{9}, ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	// Or-of-memberships is observationally equivalent here (distinct values
	// match exactly one slot); delivery-count fidelity for the flattened
	// Java shape is pinned by TestMembershipMixedArrayAndScalarCounting.
	varIntsTwo := VariableRef[[]int32]("var_intstwo")
	tryOperator("intBoxed in (var_ints, var_intstwo)",
		Or(InSlice[int32](intBoxed, varInts), InSlice[int32](intBoxed, varIntsTwo)),
		[]variableUseOperatorRow{
			variableUseIntRow(11, false), variableUseIntRow(10, true), variableUseIntRow(9, true), variableUseIntRow(8, true),
		})

	// "create constant variable SupportBean[] var_beans" rejects event-type
	// arrays in Java; the Go builder surface has no equivalent declaration
	// path, registered as a remaining note.

	// Byte[] boxed vs byte[primitive] collapse to one Go array kind; the
	// distinction is unrepresentable and registered as a representation
	// difference. The uninitialized declarations map onto a typed null.
	if err := env.RegisterVariable("myBytes", nil, VariableType(typeOf[[]byte]()), ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	bytesPlan, err := env.Build(Select(
		From[variableUseConstantBean](env, "SupportBean"),
		Alias("myBytes", VariableRef[[]byte]("myBytes")),
	).Query(StatementName("bytes-type")))
	if err != nil {
		t.Fatal(err)
	}
	assertResultSchemaType(t, bytesPlan, "myBytes", reflect.TypeOf([]byte{}))

	// Enum constants over the named comparable representation of SupportEnum.
	if err := env.RegisterVariable("var_enumone", variableUseEnumValueTwo, ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	enumOne := VariableRef[variableUseConstantEnum]("var_enumone")
	enumValue := Field[variableUseConstantBean, variableUseConstantEnum]("enumValue")
	tryOperator("var_enumone = enumValue", Equal[variableUseConstantEnum](enumOne, enumValue), []variableUseOperatorRow{
		variableUseEnumRow(variableUseEnumValueThree, false),
		variableUseEnumRow(variableUseEnumValueTwo, true),
		variableUseEnumRow(variableUseEnumValueOne, false),
	})

	if err := env.RegisterVariable("var_enumarr", []variableUseConstantEnum{variableUseEnumValueTwo, variableUseEnumValueOne}, ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	// Boolean-outcome pin only: Java compiles this membership to flattened
	// filter slots [V2,V1,V2], so ENUM_VALUE_2 DELIVERS TWICE at runtime.
	// Per-slot delivery-count fidelity is pinned by
	// TestMembershipMultiMatchPerCandidateSlot and the scenario evidence
	// (final listener rows seq 51,52,53 = V2,V2,V1).
	enumArr := VariableRef[[]variableUseConstantEnum]("var_enumarr")
	tryOperator("enumValue in (var_enumarr, var_enumone)",
		Or(InSlice[variableUseConstantEnum](enumValue, enumArr), In[variableUseConstantEnum](enumValue, enumOne)),
		[]variableUseOperatorRow{
			variableUseEnumRow(variableUseEnumValueThree, false),
			variableUseEnumRow(variableUseEnumValueTwo, true),
			variableUseEnumRow(variableUseEnumValueOne, true),
		})

	// Final: the non-constant enum variable set by an on-set statement that
	// is deployed but never fired.
	if err := env.RegisterVariable("var_enumtwo", variableUseEnumValueTwo); err != nil {
		t.Fatal(err)
	}
	enumSetterPlan, err := env.Build(
		OnEvent(From[variableUseConstantBean](env, "SupportBean")).SetVariables(
			SetVariableExpr("var_enumtwo", enumValue),
		).Query(StatementName("set-enumtwo")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(ctx, enumSetterPlan); err != nil {
		t.Fatal(err)
	}
}

// assertVariableBuildConstRejection requires a Build failure to carry the
// State kind through its wrapper chain and to hold the bare Java sentence as
// the innermost message. The outer "trigger"/"output" wrappers are
// statement-level clause context the typed builders keep as Error paths,
// matching the contract's inner-sentence alignment.
func assertVariableBuildConstRejection(t *testing.T, err error, message string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected build rejection %q, got nil", message)
	}
	if !errors.Is(err, ErrorState) {
		t.Fatalf("error %v does not carry the State kind", err)
	}
	var esperErr *Error
	if !errors.As(err, &esperErr) {
		t.Fatalf("error %v is not an *Error", err)
	}
	for {
		var inner *Error
		if !errors.As(esperErr.Cause, &inner) {
			break
		}
		esperErr = inner
	}
	if esperErr.Message != message {
		t.Fatalf("inner message = %q, want %q", esperErr.Message, message)
	}
}
