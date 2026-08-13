package esper

import (
	"context"
	"reflect"
	"testing"
)

type exprDefineSubqueryCorrelatedBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type exprDefineSubqueryCorrelatedST0 struct {
	ID  string `esper:"id"`
	P00 int    `esper:"p00"`
}

// TestExprDefineSubqueryCorrelatedParity covers Java's
// ExprDefineSubqueryCorrelated. The declared expression binds the complete
// outer SupportBean event and applies Esper's scalar null-on-multiple
// cardinality to the correlated keep-all subquery.
func TestExprDefineSubqueryCorrelatedParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprDefineSubqueryCorrelatedBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprDefineSubqueryCorrelatedST0](env, "SupportBean_ST0"); err != nil {
		t.Fatal(err)
	}

	parameter := ExpressionParam[exprDefineSubqueryCorrelatedBean]("x")
	inner := From[exprDefineSubqueryCorrelatedST0](env, "SupportBean_ST0").Window(KeepAll()).AsRecord()
	value := SubqueryValueWithOptions[string](
		inner,
		Field[any, string]("id"),
		SubqueryWhere(Equal[int](Field[any, int]("p00"), Property[int](parameter, "intPrimitive"))),
		SubqueryCardinalityMode(SubqueryNullOnMultiple),
	)
	if err := env.DefineExpression("subqOne", value); err != nil {
		t.Fatal(err)
	}
	definition, ok := env.Expression("subqOne")
	if !ok || len(definition.Parameters) != 1 || definition.Parameters[0].Name != "x" || definition.Parameters[0].Type != reflect.TypeOf(exprDefineSubqueryCorrelatedBean{}) {
		t.Fatalf("correlated definition = %#v, want SupportBean parameter x", definition)
	}

	input := From[exprDefineSubqueryCorrelatedBean](env, "SupportBean")
	plan, err := env.Build(Select(input,
		Alias("val0", Field[exprDefineSubqueryCorrelatedBean, string]("theString")),
		Alias("val1", ExpressionRef[string](env, "subqOne", EventValue[exprDefineSubqueryCorrelatedBean]())),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("correlated subquery result schema is missing")
	}
	for _, name := range []string{"val0", "val1"} {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != reflect.TypeOf("") {
			t.Fatalf("correlated result field %s = %#v, want string", name, field)
		}
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	rows := collectDotRows(t, deployment)

	if err := engine.SendEvent(context.Background(), exprDefineSubqueryCorrelatedBean{TheString: "E0"}); err != nil {
		t.Fatal(err)
	}
	assertExprDefineCorrelatedRow(t, *rows, 0, "E0", Null())

	if err := engine.SendEvent(context.Background(), exprDefineSubqueryCorrelatedST0{ID: "ST0", P00: 100}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 {
		t.Fatalf("correlated inner update emitted outer row: %#v", *rows)
	}
	if err := engine.SendEvent(context.Background(), exprDefineSubqueryCorrelatedBean{TheString: "E1", IntPrimitive: 99}); err != nil {
		t.Fatal(err)
	}
	assertExprDefineCorrelatedRow(t, *rows, 1, "E1", Null())

	if err := engine.SendEvent(context.Background(), exprDefineSubqueryCorrelatedBean{TheString: "E2", IntPrimitive: 100}); err != nil {
		t.Fatal(err)
	}
	assertExprDefineCorrelatedRow(t, *rows, 2, "E2", Present("ST0"))

	if err := engine.SendEvent(context.Background(), exprDefineSubqueryCorrelatedST0{ID: "ST1", P00: 100}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), exprDefineSubqueryCorrelatedBean{TheString: "E3", IntPrimitive: 100}); err != nil {
		t.Fatal(err)
	}
	assertExprDefineCorrelatedRow(t, *rows, 3, "E3", Null())
}

func assertExprDefineCorrelatedRow(t *testing.T, rows []Row, index int, val0 string, val1 Value) {
	t.Helper()
	if len(rows) != index+1 || rows[index].Get("val0").Any() != val0 || !rows[index].Get("val1").Equal(val1) {
		t.Fatalf("correlated rows = %#v, want row %d val0=%q val1=%v", rows, index, val0, val1)
	}
}
