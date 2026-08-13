package esper

import (
	"context"
	"reflect"
	"testing"
)

type exprDefineSubqueryCrossBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type exprDefineSubqueryCrossST0 struct {
	ID  string `esper:"id"`
	P00 int    `esper:"p00"`
}

type exprDefineSubqueryCrossST1 struct {
	ID  string `esper:"id"`
	P10 int    `esper:"p10"`
}

// TestExprDefineSubqueryCrossParity covers Java's ExprDefineSubqueryCross.
// A two-parameter declared expression receives complete events from two
// last-event join sources and uses both while correlating a keep-all scalar
// subquery.
func TestExprDefineSubqueryCrossParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprDefineSubqueryCrossBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprDefineSubqueryCrossST0](env, "SupportBean_ST0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprDefineSubqueryCrossST1](env, "SupportBean_ST1"); err != nil {
		t.Fatal(err)
	}

	x := ExpressionParam[exprDefineSubqueryCrossST0]("x")
	y := ExpressionParam[exprDefineSubqueryCrossST1]("y")
	inner := From[exprDefineSubqueryCrossBean](env, "SupportBean").Window(KeepAll()).AsRecord()
	predicate := And(
		Equal[string](Field[any, string]("theString"), Property[string](x, "id")),
		Equal[int](Field[any, int]("intPrimitive"), Property[int](y, "p10")),
	)
	if err := env.DefineExpression("subq", SubqueryValue[string](
		inner,
		Field[any, string]("theString"),
		predicate,
	)); err != nil {
		t.Fatal(err)
	}
	definition, ok := env.Expression("subq")
	if !ok || len(definition.Parameters) != 2 || definition.Parameters[0].Name != "x" || definition.Parameters[1].Name != "y" {
		t.Fatalf("cross-subquery definition = %#v, want parameters x,y", definition)
	}

	one := From[exprDefineSubqueryCrossST0](env, "SupportBean_ST0").Window(LastEvent())
	two := From[exprDefineSubqueryCrossST1](env, "SupportBean_ST1").Window(LastEvent())
	value := ExpressionRef[string](env, "subq",
		JoinEventValue[exprDefineSubqueryCrossST0](0),
		JoinEventValue[exprDefineSubqueryCrossST1](1),
	)
	plan, err := env.Build(Join(one, two).Select(
		SelectFrom(0, "val1", value),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("cross-subquery result schema is missing")
	}
	field, ok := resultSchema.Field("val1")
	if !ok || field.Type != reflect.TypeOf("") {
		t.Fatalf("cross-subquery result field = %#v, want string", field)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	rows := collectDotRows(t, deployment)

	if err := engine.SendEvent(context.Background(), exprDefineSubqueryCrossST0{ID: "ST0", P00: 0}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 0 {
		t.Fatalf("cross-subquery emitted without both outer events: %#v", *rows)
	}
	if err := engine.SendEvent(context.Background(), exprDefineSubqueryCrossST1{ID: "ST1", P10: 20}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 || !(*rows)[0].Get("val1").IsNull() {
		t.Fatalf("cross-subquery initial rows = %#v, want null", *rows)
	}

	if err := engine.SendEvent(context.Background(), exprDefineSubqueryCrossBean{TheString: "ST0", IntPrimitive: 20}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 {
		t.Fatalf("cross-subquery inner update emitted outer row: %#v", *rows)
	}
	if err := engine.SendEvent(context.Background(), exprDefineSubqueryCrossST1{ID: "x", P10: 20}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 2 || !(*rows)[1].Get("val1").Equal(Present("ST0")) {
		t.Fatalf("cross-subquery matched rows = %#v, want ST0", *rows)
	}
}
