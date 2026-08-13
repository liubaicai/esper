package esper

import (
	"context"
	"reflect"
	"testing"
)

type exprDefineNamedWindowUncorrelatedBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type exprDefineNamedWindowUncorrelatedST0 struct {
	ID string `esper:"id"`
}

// TestExprDefineSubqueryNamedWindowUncorrelatedParity covers Java's
// ExprDefineSubqueryNamedWindowUncorrelated. A zero-parameter declaration
// filters and orders a live named-window collection, and callers can continue
// applying enumeration methods to the returned events.
func TestExprDefineSubqueryNamedWindowUncorrelatedParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprDefineNamedWindowUncorrelatedBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprDefineNamedWindowUncorrelatedST0](env, "SupportBean_ST0"); err != nil {
		t.Fatal(err)
	}
	windowSchema, err := RegisterMap(env, "ExprDefineNamedWindowUncorrelatedRow", []FieldSpec{
		FieldDef("val0", reflect.TypeOf("")),
		FieldDef("val1", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindow", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	insertPlan, err := env.Build(OnEvent(From[exprDefineNamedWindowUncorrelatedBean](env, "SupportBean")).
		InsertIntoNamedWindow("MyWindow",
			SetColumn("val0", Field[exprDefineNamedWindowUncorrelatedBean, string]("theString")),
			SetColumn("val1", Field[exprDefineNamedWindowUncorrelatedBean, int]("intPrimitive")),
		).Query(StatementName("insert-window")))
	if err != nil {
		t.Fatal(err)
	}

	windowEvents := SubqueryEvents(FromNamedWindow(env, "MyWindow"))
	qualified := EnumWhere[Event](windowEvents,
		Greater[int](EnumField[Event, int]("val1"), Literal(10)),
	)
	ordered := EnumOrderBy[Event, string](qualified, EnumField[Event, string]("val0"), false)
	if err := env.DefineExpression("subqnamedwin", ordered); err != nil {
		t.Fatal(err)
	}
	definition, ok := env.Expression("subqnamedwin")
	if !ok || len(definition.Parameters) != 0 {
		t.Fatalf("named-window definition = %#v, want zero parameters", definition)
	}

	declared := ExpressionRef[[]Event](env, "subqnamedwin")
	underHundred := EnumWhere[Event](declared,
		Less[int](EnumField[Event, int]("val1"), Literal(100)),
	)
	mainPlan, err := env.Build(Select(From[exprDefineNamedWindowUncorrelatedST0](env, "SupportBean_ST0"),
		Alias("c0", declared),
		Alias("c1", underHundred),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := mainPlan.ResultSchema()
	if !ok {
		t.Fatal("named-window subquery result schema is missing")
	}
	for _, name := range []string{"c0", "c1"} {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != reflect.TypeOf([]Event{}) {
			t.Fatalf("named-window result field %s = %#v, want []Event", name, field)
		}
	}

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
		bean exprDefineNamedWindowUncorrelatedBean
		id   string
		c0   []string
		c1   []string
	}{
		{bean: exprDefineNamedWindowUncorrelatedBean{TheString: "E0"}, id: "ID0", c0: []string{}, c1: []string{}},
		{bean: exprDefineNamedWindowUncorrelatedBean{TheString: "E1", IntPrimitive: 11}, id: "ID1", c0: []string{"E1"}, c1: []string{"E1"}},
		{bean: exprDefineNamedWindowUncorrelatedBean{TheString: "E2", IntPrimitive: 500}, id: "ID2", c0: []string{"E1", "E2"}, c1: []string{"E1"}},
	}
	for index, step := range steps {
		if err := engine.SendEvent(context.Background(), step.bean); err != nil {
			t.Fatal(err)
		}
		if len(*rows) != index {
			t.Fatalf("named-window insert emitted outer row: %#v", *rows)
		}
		if err := engine.SendEvent(context.Background(), exprDefineNamedWindowUncorrelatedST0{ID: step.id}); err != nil {
			t.Fatal(err)
		}
		assertExprDefineNamedWindowEvents(t, *rows, index, "c0", step.c0)
		assertExprDefineNamedWindowEvents(t, *rows, index, "c1", step.c1)
	}
}

func assertExprDefineNamedWindowEvents(t *testing.T, rows []Row, index int, field string, expected []string) {
	t.Helper()
	if len(rows) != index+1 {
		t.Fatalf("named-window rows = %#v, want %d rows", rows, index+1)
	}
	events, ok := rows[index].Get(field).Any().([]Event)
	if !ok || len(events) != len(expected) {
		t.Fatalf("named-window row %d field %s = %#v, want val0=%v", index, field, rows[index].Get(field), expected)
	}
	for eventIndex, event := range events {
		if event.Get("val0").Any() != expected[eventIndex] {
			t.Fatalf("named-window row %d field %s event %d = %#v, want val0=%q", index, field, eventIndex, event, expected[eventIndex])
		}
	}
}
