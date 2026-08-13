package esper

import (
	"context"
	"reflect"
	"testing"
)

type exprDefineNamedWindowCorrelatedBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type exprDefineNamedWindowCorrelatedST0 struct {
	ID   string `esper:"id"`
	Key0 string `esper:"key0"`
	P00  int    `esper:"p00"`
}

// TestExprDefineSubqueryNamedWindowCorrelatedParity covers Java's
// ExprDefineSubqueryNamedWindowCorrelated. The declared expression binds the
// complete ST0 trigger, correlates a live named-window collection by key0 and
// applies a second collection filter to the matching events.
func TestExprDefineSubqueryNamedWindowCorrelatedParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprDefineNamedWindowCorrelatedBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprDefineNamedWindowCorrelatedST0](env, "SupportBean_ST0"); err != nil {
		t.Fatal(err)
	}
	windowSchema, err := RegisterMap(env, "ExprDefineNamedWindowCorrelatedRow", []FieldSpec{
		FieldDef("val0", reflect.TypeOf("")),
		FieldDef("val1", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindow", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	insertPlan, err := env.Build(OnEvent(From[exprDefineNamedWindowCorrelatedBean](env, "SupportBean")).
		InsertIntoNamedWindow("MyWindow",
			SetColumn("val0", Field[exprDefineNamedWindowCorrelatedBean, string]("theString")),
			SetColumn("val1", Field[exprDefineNamedWindowCorrelatedBean, int]("intPrimitive")),
		).Query(StatementName("insert-window")))
	if err != nil {
		t.Fatal(err)
	}

	parameter := ExpressionParam[exprDefineNamedWindowCorrelatedST0]("x")
	matching := SubqueryEvents(FromNamedWindow(env, "MyWindow"), SubqueryWhere(
		Equal[string](Field[any, string]("val0"), Property[string](parameter, "key0")),
	))
	qualified := EnumWhere[Event](matching,
		Greater[int](EnumField[Event, int]("val1"), Literal(10)),
	)
	if err := env.DefineExpression("subqnamedwin", qualified); err != nil {
		t.Fatal(err)
	}
	definition, ok := env.Expression("subqnamedwin")
	if !ok || len(definition.Parameters) != 1 || definition.Parameters[0].Name != "x" || definition.Parameters[0].Type != reflect.TypeOf(exprDefineNamedWindowCorrelatedST0{}) {
		t.Fatalf("correlated named-window definition = %#v, want SupportBean_ST0 parameter x", definition)
	}

	mainPlan, err := env.Build(Select(From[exprDefineNamedWindowCorrelatedST0](env, "SupportBean_ST0"),
		Alias("c0", ExpressionRef[[]Event](env, "subqnamedwin", EventValue[exprDefineNamedWindowCorrelatedST0]())),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := mainPlan.ResultSchema()
	if !ok {
		t.Fatal("correlated named-window result schema is missing")
	}
	field, ok := resultSchema.Field("c0")
	if !ok || field.Type != reflect.TypeOf([]Event{}) {
		t.Fatalf("correlated named-window result field = %#v, want []Event", field)
	}

	assertExprDefineNamedWindowCorrelatedSameNameBuild(t, env)

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	mainDeployment, err := engine.Deploy(context.Background(), mainPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	rows := collectDotRows(t, mainDeployment)

	steps := []struct {
		bean     exprDefineNamedWindowCorrelatedBean
		trigger  exprDefineNamedWindowCorrelatedST0
		expected []string
	}{
		{bean: exprDefineNamedWindowCorrelatedBean{TheString: "E0"}, trigger: exprDefineNamedWindowCorrelatedST0{ID: "ID0", Key0: "x"}, expected: []string{}},
		{bean: exprDefineNamedWindowCorrelatedBean{TheString: "E1", IntPrimitive: 11}, trigger: exprDefineNamedWindowCorrelatedST0{ID: "ID1", Key0: "x"}, expected: []string{}},
		{bean: exprDefineNamedWindowCorrelatedBean{TheString: "E2", IntPrimitive: 12}, trigger: exprDefineNamedWindowCorrelatedST0{ID: "ID2", Key0: "E2"}, expected: []string{"E2"}},
		{bean: exprDefineNamedWindowCorrelatedBean{TheString: "E3", IntPrimitive: 13}, trigger: exprDefineNamedWindowCorrelatedST0{ID: "E3", Key0: "E3"}, expected: []string{"E3"}},
	}
	for index, step := range steps {
		if err := engine.SendEvent(context.Background(), step.bean); err != nil {
			t.Fatal(err)
		}
		if len(*rows) != index {
			t.Fatalf("correlated named-window insert emitted outer row: %#v", *rows)
		}
		if err := engine.SendEvent(context.Background(), step.trigger); err != nil {
			t.Fatal(err)
		}
		assertExprDefineNamedWindowEvents(t, *rows, index, "c0", step.expected)
	}
}

func assertExprDefineNamedWindowCorrelatedSameNameBuild(t *testing.T, env *Environment) {
	t.Helper()
	schema, err := RegisterMap(env, "ExprDefineNamedWindowCorrelatedSameNameRow", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
		FieldDef("p00", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowTwo", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	parameter := ExpressionParam[exprDefineNamedWindowCorrelatedST0]("x")
	matching := SubqueryEvents(FromNamedWindow(env, "MyWindowTwo"), SubqueryWhere(
		Equal[string](Field[any, string]("id"), Property[string](parameter, "id")),
	))
	if err := env.DefineExpression("subqnamedwinSameName", EnumWhere[Event](matching,
		Greater[int](EnumField[Event, int]("p00"), Literal(10)),
	)); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Build(Select(From[exprDefineNamedWindowCorrelatedST0](env, "SupportBean_ST0"),
		Alias("c0", ExpressionRef[[]Event](env, "subqnamedwinSameName", EventValue[exprDefineNamedWindowCorrelatedST0]())),
	).Query(StatementName("same-name-build"))); err != nil {
		t.Fatal(err)
	}
}
