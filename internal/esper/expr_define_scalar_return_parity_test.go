package esper

import (
	"context"
	"reflect"
	"testing"
)

type exprDefineScalarReturnCollection struct {
	Strvals []string `esper:"strvals"`
}

type exprDefineScalarReturnSupportBean struct {
	IntPrimitive int `esper:"intPrimitive"`
}

type exprDefineScalarReturnObject struct {
	One any `esper:"one"`
}

// TestExprDefineScalarReturnParity covers Java's ExprDefineScalarReturn. It
// exercises both a declared expression returning a typed collection and the
// same declaration style in an on-select predicate with CASE and CAST.
func TestExprDefineScalarReturnParity(t *testing.T) {
	t.Run("nestedCollectionFilters", testExprDefineScalarReturnNestedCollectionFilters)
	t.Run("onSelectCaseCast", testExprDefineScalarReturnOnSelectCaseCast)
}

func testExprDefineScalarReturnNestedCollectionFilters(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprDefineScalarReturnCollection](env, "SupportCollection"); err != nil {
		t.Fatal(err)
	}

	collection := ExpressionParam[exprDefineScalarReturnCollection]("collection")
	strvals := Property[[]string](collection, "strvals")
	withoutE1 := EnumWhere[string](strvals,
		NotEqual[string](EnumElement[string](), Literal("E1")))
	if err := env.DefineExpression("scalarfilter", withoutE1); err != nil {
		t.Fatal(err)
	}

	filtered := EnumWhere[string](
		ExpressionRef[[]string](env, "scalarfilter", EventValue[exprDefineScalarReturnCollection]()),
		NotEqual[string](EnumElement[string](), Literal("E2")),
	)
	plan, err := env.Build(Select(From[exprDefineScalarReturnCollection](env, "SupportCollection"),
		Alias("val1", filtered),
	).Query(StatementName("expr-define-scalar-return")))
	if err != nil {
		t.Fatal(err)
	}

	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("scalar-return result schema is missing")
	}
	field, ok := resultSchema.Field("val1")
	if !ok || field.Type != reflect.TypeOf([]string{}) {
		t.Fatalf("scalar-return result field = %#v, want []string", field)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	rows := collectDotRows(t, deployment)
	if err := engine.SendEvent(context.Background(), exprDefineScalarReturnCollection{
		Strvals: []string{"E1", "E2", "E3", "E4"},
	}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 || !(*rows)[0].Get("val1").Equal(Present([]string{"E3", "E4"})) {
		t.Fatalf("scalar-return nested filters = %#v, want [E3 E4]", *rows)
	}
}

func testExprDefineScalarReturnOnSelectCaseCast(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprDefineScalarReturnSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprDefineScalarReturnObject](env, "SupportBeanObject"); err != nil {
		t.Fatal(err)
	}
	windowSchema, err := RegisterMap(env, "ScalarReturnWindow", []FieldSpec{
		FieldDef("myObject", reflect.TypeOf(int64(0))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "ScalarReturnWindow", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	input := From[exprDefineScalarReturnSupportBean](env, "SupportBean")
	insertPlan, err := env.Build(OnEvent(input).InsertIntoNamedWindow("ScalarReturnWindow",
		SetColumn("myObject", Cast[int, int64](Field[exprDefineScalarReturnSupportBean, int]("intPrimitive"))),
	).Query(StatementName("scalar-return-window-insert")))
	if err != nil {
		t.Fatal(err)
	}

	object := ExpressionParam[exprDefineScalarReturnObject]("myEvent")
	one := Property[any](object, "one")
	theExpression := CaseWhen[int64](
		Equal[string](Property[string](object, "one"), Literal("X")),
		Literal(int64(0)),
	).Else(Cast[any, int64](one))
	if err := env.DefineExpression("theExpression", theExpression); err != nil {
		t.Fatal(err)
	}

	trigger := From[exprDefineScalarReturnObject](env, "SupportBeanObject")
	predicate := Equal[int64](
		NamedWindowField[int64]("myObject"),
		ExpressionRef[int64](env, "theExpression", EventValue[exprDefineScalarReturnObject]()),
	)
	selectPlan, err := env.Build(OnEvent(trigger).SelectFromNamedWindow("ScalarReturnWindow", predicate,
		Alias("myObject", NamedWindowField[int64]("myObject")),
	).Query(StatementName("scalar-return-on-select")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	insertDeployment, err := engine.Deploy(context.Background(), insertPlan)
	if err != nil {
		t.Fatal(err)
	}
	selectDeployment, err := engine.Deploy(context.Background(), selectPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	rows := collectDotRows(t, selectDeployment)

	for _, value := range []int{0, 1} {
		if err := engine.SendEvent(context.Background(), exprDefineScalarReturnSupportBean{IntPrimitive: value}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SendEvent(context.Background(), exprDefineScalarReturnObject{One: 2}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 0 {
		t.Fatalf("numeric non-match emitted rows = %#v", *rows)
	}
	if err := engine.SendEvent(context.Background(), exprDefineScalarReturnObject{One: "X"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), exprDefineScalarReturnObject{One: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 2 || (*rows)[0].Get("myObject").Any() != int64(0) || (*rows)[1].Get("myObject").Any() != int64(1) {
		t.Fatalf("on-select CASE/CAST rows = %#v, want [0 1]", *rows)
	}
	_ = insertDeployment
}
