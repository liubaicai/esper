package esper

import (
	"context"
	"reflect"
	"testing"
)

type exprDefineSplitStreamBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestExprDefineSplitStreamParity covers Java's ExprDefineSplitStream. The
// declared expression keeps its explicit event parameter while evaluating to
// false for every event, so only the branch using its negation routes.
func TestExprDefineSplitStreamParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprDefineSplitStreamBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ABC", "DEF"} {
		if _, err := RegisterMap(env, name, []FieldSpec{
			FieldDef("theString", reflect.TypeOf("")),
			FieldDef("intPrimitive", reflect.TypeOf(0)),
		}); err != nil {
			t.Fatal(err)
		}
	}

	parameter := ExpressionParam[exprDefineSplitStreamBean]("event")
	alwaysFalse := And(
		Literal(false),
		Equal[string](Property[string](parameter, "theString"), Property[string](parameter, "theString")),
	)
	if err := env.DefineExpression("myLittleExpression", alwaysFalse); err != nil {
		t.Fatal(err)
	}
	definition, ok := env.Expression("myLittleExpression")
	if !ok || len(definition.Parameters) != 1 || definition.Parameters[0].Name != "event" || definition.Parameters[0].Type != reflect.TypeOf(exprDefineSplitStreamBean{}) {
		t.Fatalf("split-stream definition = %#v, want SupportBean parameter event", definition)
	}

	argument := EventValue[exprDefineSplitStreamBean]()
	predicate := ExpressionRef[bool](env, "myLittleExpression", argument)
	splitPlan, err := env.Build(OnEvent(From[exprDefineSplitStreamBean](env, "SupportBean")).SplitAll(
		SplitIntoWhen(predicate, "ABC"),
		SplitIntoWhen(Not(predicate), "DEF"),
	).Query(StatementName("split")))
	if err != nil {
		t.Fatal(err)
	}
	abcPlan, err := env.Build(FromAny(env, "ABC").Select(
		Alias("theString", Field[any, string]("theString")),
		Alias("intPrimitive", Field[any, int]("intPrimitive")),
	).Query(StatementName("abc")))
	if err != nil {
		t.Fatal(err)
	}
	defPlan, err := env.Build(FromAny(env, "DEF").Select(
		Alias("theString", Field[any, string]("theString")),
		Alias("intPrimitive", Field[any, int]("intPrimitive")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	abcDeployment, err := engine.Deploy(context.Background(), abcPlan)
	if err != nil {
		t.Fatal(err)
	}
	defDeployment, err := engine.Deploy(context.Background(), defPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), splitPlan); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	abcRows := collectDotRows(t, abcDeployment)
	defRows := collectDotRows(t, defDeployment)

	event := exprDefineSplitStreamBean{TheString: "E1", IntPrimitive: 10}
	if err := engine.SendEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if len(*abcRows) != 0 {
		t.Fatalf("split-stream ABC rows = %#v, want none", *abcRows)
	}
	if len(*defRows) != 1 || (*defRows)[0].Get("theString").Any() != "E1" || (*defRows)[0].Get("intPrimitive").Any() != 10 {
		t.Fatalf("split-stream DEF rows = %#v, want E1/10", *defRows)
	}
}
