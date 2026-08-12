package esper

import (
	"context"
	"reflect"
	"testing"
)

func TestExistsExpressionsMatchDynamicPropertyPresenceAndNullSemantics(t *testing.T) {
	env := NewEnvironment()
	schema, err := NewMapSchema("ExistsDynamic", []FieldSpec{
		FieldDef("key", reflect.TypeOf("")),
	}, AllowDynamicFields())
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(schema); err != nil {
		t.Fatal(err)
	}
	input := From[map[string]any](env, "ExistsDynamic")
	prop := Field[map[string]any, any]("prop")
	item := Field[map[string]any, any]("item")
	plan, err := env.Build(Select(input,
		Alias("key", Exists(Field[map[string]any, string]("key"))),
		Alias("prop", Exists(prop)),
		Alias("missing", Exists(Field[map[string]any, any]("missing"))),
		Alias("explicit_null", Exists(Field[map[string]any, any]("null"))),
		Alias("nested_id", Exists(Property[any](item, "id"))),
		Alias("direct_null", Exists(NullLiteral[any]())),
	).Query(StatementName("exists-dynamic")))
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(plan.Query())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent exists plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("exists result schema is missing")
	}
	for _, name := range []string{"key", "prop", "missing", "explicit_null", "nested_id", "direct_null"} {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != typeOf[bool]() {
			t.Fatalf("exists field %q = %#v, want bool", name, field)
		}
	}
	if got := Exists(ContextField[any]("not-present")).eval(EvalContext{}); !got.Equal(Present(false)) {
		t.Fatalf("direct missing exists = %v, want false", got)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 3)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "exists result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	events := []map[string]any{
		{"key": "E1", "prop": 1, "null": nil, "item": map[string]any{"id": "I1"}},
		{"key": "E2", "prop": "value", "item": map[string]any{}},
		{"key": "E3", "prop": nil, "null": nil, "item": nil},
	}
	for _, event := range events {
		if err := engine.Send(context.Background(), "ExistsDynamic", event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != len(events) {
		t.Fatalf("exists rows = %d, want %d", len(rows), len(events))
	}
	wantNested := []bool{true, false, true}
	wantExplicitNull := []bool{true, false, true}
	for index, row := range rows {
		want := map[string]bool{
			"key":           true,
			"prop":          true,
			"missing":       false,
			"explicit_null": wantExplicitNull[index],
			"nested_id":     wantNested[index],
			"direct_null":   true,
		}
		for name, expected := range want {
			if got := row.Get(name); !got.Equal(Present(expected)) {
				t.Fatalf("row %d %s = %v, want %t", index, name, got, expected)
			}
		}
	}
}
