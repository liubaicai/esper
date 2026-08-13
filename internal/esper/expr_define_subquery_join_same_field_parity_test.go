package esper

import (
	"context"
	"reflect"
	"testing"
)

type exprDefineSubquerySameBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type exprDefineSubquerySameST0 struct {
	ID      string `esper:"id"`
	P00     int    `esper:"p00"`
	PCommon string `esper:"pcommon"`
}

type exprDefineSubquerySameST1 struct {
	ID      string `esper:"id"`
	P10     int    `esper:"p10"`
	PCommon string `esper:"pcommon"`
}

// TestExprDefineSubqueryJoinSameFieldParity covers Java's
// ExprDefineSubqueryJoinSameField valid declaration. One Event-typed
// parameter lets the same declared expression correlate through pcommon for
// either last-event join source while each call remains source-explicit.
func TestExprDefineSubqueryJoinSameFieldParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprDefineSubquerySameBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprDefineSubquerySameST0](env, "SupportBean_ST0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprDefineSubquerySameST1](env, "SupportBean_ST1"); err != nil {
		t.Fatal(err)
	}

	parameter := ExpressionParam[Event]("x")
	inner := From[exprDefineSubquerySameBean](env, "SupportBean").Window(KeepAll()).AsRecord()
	value := SubqueryValue[int](
		inner,
		Field[any, int]("intPrimitive"),
		Equal[string](Field[any, string]("theString"), Property[string](parameter, "pcommon")),
	)
	if err := env.DefineExpression("subq", value); err != nil {
		t.Fatal(err)
	}
	definition, ok := env.Expression("subq")
	if !ok || len(definition.Parameters) != 1 || definition.Parameters[0].Name != "x" || definition.Parameters[0].Type != reflect.TypeOf(Event{}) {
		t.Fatalf("same-field definition = %#v, want Event parameter x", definition)
	}

	one := From[exprDefineSubquerySameST0](env, "SupportBean_ST0").Window(LastEvent())
	two := From[exprDefineSubquerySameST1](env, "SupportBean_ST1").Window(LastEvent())
	plan, err := env.Build(Join(one, two).Select(
		SelectFrom(0, "val1", ExpressionRef[int](env, "subq", JoinEventValue[Event](0))),
		SelectFrom(1, "val2", ExpressionRef[int](env, "subq", JoinEventValue[Event](1))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("same-field subquery result schema is missing")
	}
	for _, name := range []string{"val1", "val2"} {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != reflect.TypeOf(int(0)) {
			t.Fatalf("same-field result field %s = %#v, want int", name, field)
		}
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	rows := collectDotRows(t, deployment)

	if err := engine.SendEvent(context.Background(), exprDefineSubquerySameST0{ID: "ST0"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), exprDefineSubquerySameST1{ID: "ST1"}); err != nil {
		t.Fatal(err)
	}
	assertExprDefineSameFieldRow(t, *rows, 0, Null(), Null())

	if err := engine.SendEvent(context.Background(), exprDefineSubquerySameBean{TheString: "E0", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), exprDefineSubquerySameST1{ID: "ST1", PCommon: "E0"}); err != nil {
		t.Fatal(err)
	}
	assertExprDefineSameFieldRow(t, *rows, 1, Null(), Present(10))

	if err := engine.SendEvent(context.Background(), exprDefineSubquerySameST0{ID: "ST0", PCommon: "E0"}); err != nil {
		t.Fatal(err)
	}
	assertExprDefineSameFieldRow(t, *rows, 2, Present(10), Present(10))
}

func assertExprDefineSameFieldRow(t *testing.T, rows []Row, index int, val1, val2 Value) {
	t.Helper()
	if len(rows) != index+1 || !rows[index].Get("val1").Equal(val1) || !rows[index].Get("val2").Equal(val2) {
		t.Fatalf("same-field rows = %#v, want row %d val1=%v val2=%v", rows, index, val1, val2)
	}
}
