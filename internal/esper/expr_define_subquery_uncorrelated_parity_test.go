package esper

import (
	"context"
	"reflect"
	"testing"
)

type exprDefineSubqueryUncorrelatedBean struct {
	TheString string `esper:"theString"`
}

type exprDefineSubqueryUncorrelatedST0 struct {
	ID string `esper:"id"`
}

// TestExprDefineSubqueryUncorrelatedParity covers Java's
// ExprDefineSubqueryUncorrelated. The zero-parameter declared expression reads
// the current value from an independent SupportBean_ST0 last-event subquery.
func TestExprDefineSubqueryUncorrelatedParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprDefineSubqueryUncorrelatedBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprDefineSubqueryUncorrelatedST0](env, "SupportBean_ST0"); err != nil {
		t.Fatal(err)
	}

	inner := From[exprDefineSubqueryUncorrelatedST0](env, "SupportBean_ST0").Window(LastEvent()).AsRecord()
	if err := env.DefineExpression("subqOne", SubqueryValue[string](inner, Field[any, string]("id"))); err != nil {
		t.Fatal(err)
	}
	definition, ok := env.Expression("subqOne")
	if !ok || len(definition.Parameters) != 0 {
		t.Fatalf("uncorrelated definition = %#v, want zero parameters", definition)
	}

	input := From[exprDefineSubqueryUncorrelatedBean](env, "SupportBean")
	plan, err := env.Build(Select(input,
		Alias("val0", Field[exprDefineSubqueryUncorrelatedBean, string]("theString")),
		Alias("val1", ExpressionRef[string](env, "subqOne")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("uncorrelated subquery result schema is missing")
	}
	for _, name := range []string{"val0", "val1"} {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != reflect.TypeOf("") {
			t.Fatalf("uncorrelated result field %s = %#v, want string", name, field)
		}
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	rows := collectDotRows(t, deployment)

	if err := engine.SendEvent(context.Background(), exprDefineSubqueryUncorrelatedBean{TheString: "E0"}); err != nil {
		t.Fatal(err)
	}
	assertExprDefineUncorrelatedRow(t, *rows, 0, "E0", Null())

	if err := engine.SendEvent(context.Background(), exprDefineSubqueryUncorrelatedST0{ID: "ST0"}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 {
		t.Fatalf("uncorrelated inner update emitted outer row: %#v", *rows)
	}
	if err := engine.SendEvent(context.Background(), exprDefineSubqueryUncorrelatedBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}
	assertExprDefineUncorrelatedRow(t, *rows, 1, "E1", Present("ST0"))

	if err := engine.SendEvent(context.Background(), exprDefineSubqueryUncorrelatedST0{ID: "ST1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), exprDefineSubqueryUncorrelatedBean{TheString: "E2"}); err != nil {
		t.Fatal(err)
	}
	assertExprDefineUncorrelatedRow(t, *rows, 2, "E2", Present("ST1"))
}

func assertExprDefineUncorrelatedRow(t *testing.T, rows []Row, index int, val0 string, val1 Value) {
	t.Helper()
	if len(rows) != index+1 || rows[index].Get("val0").Any() != val0 || !rows[index].Get("val1").Equal(val1) {
		t.Fatalf("uncorrelated rows = %#v, want row %d val0=%q val1=%v", rows, index, val0, val1)
	}
}
