package esper

import (
	"context"
	"reflect"
	"testing"
)

func TestTypeNameExpressionsMatchDynamicValuesAndEventTypeIdentity(t *testing.T) {
	env := NewEnvironment()
	schema, err := NewMapSchema("TypeNameDynamic", []FieldSpec{
		FieldDef("key", reflect.TypeOf("")),
	}, AllowDynamicFields())
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(schema); err != nil {
		t.Fatal(err)
	}
	input := From[map[string]any](env, "TypeNameDynamic")
	property := Field[map[string]any, any]("prop")
	plan, err := env.Build(Select(input,
		Alias("property_type", TypeName(property)),
		Alias("event_type", TypeName(EventValue[Event]())),
		Alias("key_type", TypeName(Field[map[string]any, string]("key"))),
		Alias("null_type", TypeName(NullLiteral[any]())),
	).Query(StatementName("type-name")))
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(plan.Query())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent type-name plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("type-name result schema is missing")
	}
	for _, name := range []string{"property_type", "event_type", "key_type", "null_type"} {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != typeOf[string]() {
			t.Fatalf("type-name field %q = %#v, want string", name, field)
		}
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
				return NewError(ErrorTypeMismatch, "type-name result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	events := []map[string]any{
		{"key": "E1", "prop": 1},
		{"key": "E2", "prop": "value"},
		{"key": "E3", "prop": nil},
	}
	for _, event := range events {
		if err := engine.Send(context.Background(), "TypeNameDynamic", event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != len(events) {
		t.Fatalf("type-name rows = %d, want %d", len(rows), len(events))
	}
	wantPropertyTypes := []Value{Present("int"), Present("string"), Null()}
	for index, row := range rows {
		if !row.Get("property_type").Equal(wantPropertyTypes[index]) ||
			!row.Get("event_type").Equal(Present("TypeNameDynamic")) ||
			!row.Get("key_type").Equal(Present("string")) ||
			!row.Get("null_type").IsNull() {
			t.Fatalf("type-name row %d = %#v, want property=%v event=TypeNameDynamic key=string null", index, row, wantPropertyTypes[index])
		}
	}
}
