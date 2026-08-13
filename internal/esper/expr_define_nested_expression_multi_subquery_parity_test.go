package esper

import (
	"context"
	"reflect"
	"testing"
)

type exprDefineNestedMultiSubqueryBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestExprDefineNestedExpressionMultiSubqueryParity covers Java's
// ExprDefineNestedExpressionMultiSubquery. F3 composes an uncorrelated
// last-event scalar declaration with a parameterized unique-window scalar
// declaration and forwards its complete event parameter to F2.
func TestExprDefineNestedExpressionMultiSubqueryParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprDefineNestedMultiSubqueryBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}

	last := From[exprDefineNestedMultiSubqueryBean](env, "SupportBean").Window(LastEvent()).AsRecord()
	if err := env.DefineExpression("F1", SubqueryValue[int](last, Field[any, int]("intPrimitive"))); err != nil {
		t.Fatal(err)
	}

	parameter := ExpressionParam[exprDefineNestedMultiSubqueryBean]("param")
	unique := From[exprDefineNestedMultiSubqueryBean](env, "SupportBean").
		Window(Unique(Field[exprDefineNestedMultiSubqueryBean, string]("theString"))).AsRecord()
	f2 := SubqueryValueWithOptions[int](unique,
		Field[any, int]("intPrimitive"),
		SubqueryWhere(Equal[string](Field[any, string]("theString"), Property[string](parameter, "theString"))),
		SubqueryCardinalityMode(SubqueryNullOnMultiple),
	)
	if err := env.DefineExpression("F2", f2); err != nil {
		t.Fatal(err)
	}

	s := ExpressionParam[exprDefineNestedMultiSubqueryBean]("s")
	if err := env.DefineExpression("F3", Add[int](
		ExpressionRef[int](env, "F1"),
		ExpressionRef[int](env, "F2", s),
	)); err != nil {
		t.Fatal(err)
	}
	assertExprDefineNestedMultiSubqueryDefinitions(t, env)

	plan, err := env.Build(Select(From[exprDefineNestedMultiSubqueryBean](env, "SupportBean"),
		Alias("c0", ExpressionRef[int](env, "F3", EventValue[exprDefineNestedMultiSubqueryBean]())),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("nested multi-subquery result schema is missing")
	}
	field, ok := resultSchema.Field("c0")
	if !ok || field.Type != reflect.TypeOf(0) {
		t.Fatalf("nested multi-subquery result field = %#v, want int", field)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	rows := collectDotRows(t, deployment)

	for index, event := range []exprDefineNestedMultiSubqueryBean{
		{TheString: "E1", IntPrimitive: 10},
		{TheString: "E1", IntPrimitive: 11},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		expected := 20 + index*2
		if len(*rows) != index+1 || !(*rows)[index].Get("c0").Equal(Present(expected)) {
			t.Fatalf("nested multi-subquery rows = %#v, want row %d c0=%d", *rows, index, expected)
		}
	}
}

func assertExprDefineNestedMultiSubqueryDefinitions(t *testing.T, env *Environment) {
	t.Helper()
	type expected struct {
		name      string
		parameter string
	}
	for _, item := range []expected{{name: "F1"}, {name: "F2", parameter: "param"}, {name: "F3", parameter: "s"}} {
		definition, ok := env.Expression(item.name)
		if !ok {
			t.Fatalf("nested multi-subquery definition %s is missing", item.name)
		}
		if item.parameter == "" {
			if len(definition.Parameters) != 0 {
				t.Fatalf("nested multi-subquery definition %s = %#v, want zero parameters", item.name, definition)
			}
			continue
		}
		if len(definition.Parameters) != 1 || definition.Parameters[0].Name != item.parameter || definition.Parameters[0].Type != reflect.TypeOf(exprDefineNestedMultiSubqueryBean{}) {
			t.Fatalf("nested multi-subquery definition %s = %#v, want SupportBean parameter %s", item.name, definition, item.parameter)
		}
	}
}
