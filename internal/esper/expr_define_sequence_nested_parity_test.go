package esper

import (
	"context"
	"reflect"
	"testing"
)

type exprDefineSequenceS0 struct {
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

type exprDefineSequenceS1 struct {
	P10 string `esper:"p10"`
	P11 string `esper:"p11"`
}

type exprDefineSequenceBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestExprDefineSequenceAndNestedParity covers Java's
// ExprDefineSequenceAndNested. Two parameterized declarations perform
// correlated named-window collection lookups, take the last two rows, project
// nested fields and compare the resulting sequences in insertion order.
func TestExprDefineSequenceAndNestedParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprDefineSequenceS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprDefineSequenceS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprDefineSequenceBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	windowSchema, err := RegisterMap(env, "ExprDefineSequenceWindowRow", []FieldSpec{
		FieldDef("col1", reflect.TypeOf("")),
		FieldDef("col2", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"WindowOne", "WindowTwo"} {
		if _, err := CreateNamedWindow(env, name, windowSchema, NamedWindowRetention(KeepAll())); err != nil {
			t.Fatal(err)
		}
	}

	insertOne, err := env.Build(OnEvent(From[exprDefineSequenceS0](env, "SupportBean_S0")).
		InsertIntoNamedWindow("WindowOne",
			SetColumn("col1", Field[exprDefineSequenceS0, string]("p00")),
			SetColumn("col2", Field[exprDefineSequenceS0, string]("p01")),
		).Query(StatementName("insert-window-one")))
	if err != nil {
		t.Fatal(err)
	}
	insertTwo, err := env.Build(OnEvent(From[exprDefineSequenceS1](env, "SupportBean_S1")).
		InsertIntoNamedWindow("WindowTwo",
			SetColumn("col1", Field[exprDefineSequenceS1, string]("p10")),
			SetColumn("col2", Field[exprDefineSequenceS1, string]("p11")),
		).Query(StatementName("insert-window-two")))
	if err != nil {
		t.Fatal(err)
	}

	parameterX := ExpressionParam[exprDefineSequenceBean]("p")
	rowsX := SubqueryEvents(FromNamedWindow(env, "WindowOne"), SubqueryWhere(
		Equal[string](Field[any, string]("col1"), Property[string](parameterX, "theString")),
	))
	if err := env.DefineExpression("last2X", EnumTakeLast[Event](rowsX, 2)); err != nil {
		t.Fatal(err)
	}

	parameterY := ExpressionParam[exprDefineSequenceBean]("p")
	rowsY := SubqueryEvents(FromNamedWindow(env, "WindowTwo"), SubqueryWhere(
		Equal[string](Field[any, string]("col1"), Property[string](parameterY, "theString")),
	))
	lastValuesY := EnumSelect[Event, string](
		EnumTakeLast[Event](rowsY, 2),
		EnumField[Event, string]("col2"),
	)
	if err := env.DefineExpression("last2Y", lastValuesY); err != nil {
		t.Fatal(err)
	}

	bean := EventValue[exprDefineSequenceBean]()
	lastValuesX := EnumSelect[Event, string](
		ExpressionRef[[]Event](env, "last2X", bean),
		EnumField[Event, string]("col2"),
	)
	sequencesEqual := EnumSequenceEqual[string](
		lastValuesX,
		ExpressionRef[[]string](env, "last2Y", bean),
	)
	mainPlan, err := env.Build(Select(From[exprDefineSequenceBean](env, "SupportBean"),
		Alias("val", sequencesEqual),
	).Query(StatementName("s0"), StatementAudit(AuditExpressionDefinition)))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := mainPlan.ResultSchema()
	if !ok {
		t.Fatal("sequence/nested result schema is missing")
	}
	field, ok := resultSchema.Field("val")
	if !ok || field.Type != reflect.TypeOf(false) {
		t.Fatalf("sequence/nested result field = %#v, want bool", field)
	}
	metadata := mainPlan.Query().Metadata()
	if len(metadata.AuditCategories) != 1 || metadata.AuditCategories[0] != AuditExpressionDefinition {
		t.Fatalf("sequence/nested audit metadata = %#v, want EXPRDEF", metadata.AuditCategories)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), insertOne); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertTwo); err != nil {
		t.Fatal(err)
	}
	mainDeployment, err := engine.Deploy(context.Background(), mainPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	rows := collectDotRows(t, mainDeployment)
	audits := make([]AuditRecord, 0, 1)
	auditSubscription, err := engine.SubscribeAudit(func(_ context.Context, record AuditRecord) error {
		audits = append(audits, record)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = auditSubscription.Close() }()

	for _, event := range []exprDefineSequenceS0{{P00: "A", P01: "B1"}, {P00: "A", P01: "B2"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	for _, event := range []exprDefineSequenceS1{{P10: "A", P11: "B1"}, {P10: "A", P11: "B2"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SendEvent(context.Background(), exprDefineSequenceBean{TheString: "A", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 || !(*rows)[0].Get("val").Equal(Present(true)) {
		t.Fatalf("sequence/nested rows = %#v, want val=true", *rows)
	}
	if len(audits) != 1 || audits[0].StatementName() != "s0" || audits[0].Category() != AuditExpressionDefinition {
		t.Fatalf("sequence/nested audits = %#v, want one s0 EXPRDEF record", audits)
	}
}
