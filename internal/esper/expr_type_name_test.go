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

func TestTypeNameExpressionsUseDeclaredFragmentMetadata(t *testing.T) {
	t.Run("map", func(t *testing.T) {
		env := NewEnvironment()
		inner, err := NewMapSchema("TypeNameInner", []FieldSpec{FieldDef("key", reflect.TypeOf(""))})
		if err != nil {
			t.Fatal(err)
		}
		root, err := NewMapSchema("TypeNameFragmentMap", []FieldSpec{
			FieldDef("inside", reflect.TypeOf(map[string]any{})),
			FieldDef("insidearr", reflect.TypeOf([]map[string]any{})),
		}, WithNestedPropertySchema("inside", inner), WithNestedPropertySchema("insidearr", inner))
		if err != nil {
			t.Fatal(err)
		}
		if err := env.RegisterSchema(inner); err != nil {
			t.Fatal(err)
		}
		if err := env.RegisterSchema(root); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(Select(From[map[string]any](env, root.Name()),
			Alias("t0", TypeName(Field[map[string]any, any]("inside"))),
			Alias("t1", TypeName(Field[map[string]any, any]("insidearr"))),
		).Query(StatementName("type-name-fragment-map")))
		if err != nil {
			t.Fatal(err)
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
					return NewError(ErrorTypeMismatch, "fragment type-name result is not a row")
				}
				rows = append(rows, row)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		for _, payload := range []map[string]any{
			{},
			{"inside": map[string]any{}},
			{"insidearr": []map[string]any{}},
		} {
			if err := engine.Send(context.Background(), root.Name(), payload); err != nil {
				t.Fatal(err)
			}
		}
		want := [][2]Value{
			{Null(), Null()},
			{Present("TypeNameInner"), Null()},
			{Null(), Present("TypeNameInner[]")},
		}
		if len(rows) != len(want) {
			t.Fatalf("fragment rows = %d, want %d", len(rows), len(want))
		}
		for index, row := range rows {
			if !row.Get("t0").Equal(want[index][0]) || !row.Get("t1").Equal(want[index][1]) {
				t.Fatalf("fragment row %d = %#v, want %#v", index, row, want[index])
			}
		}
	})

	t.Run("avro", func(t *testing.T) {
		env := NewEnvironment()
		inner, err := NewAvroSchema("TypeNameAvroInner", []FieldSpec{FieldDef("key", reflect.TypeOf(""))})
		if err != nil {
			t.Fatal(err)
		}
		root, err := NewAvroSchema("TypeNameFragmentAvro", []FieldSpec{
			FieldDef("inside", reflect.TypeOf(map[string]any{})),
			FieldDef("insidearr", reflect.TypeOf([]map[string]any{})),
		}, WithNestedPropertySchema("inside", inner), WithNestedPropertySchema("insidearr", inner))
		if err != nil {
			t.Fatal(err)
		}
		if err := env.RegisterSchema(inner); err != nil {
			t.Fatal(err)
		}
		if err := env.RegisterSchema(root); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(Select(From[*AvroRecord](env, root.Name()),
			Alias("t0", TypeName(Field[*AvroRecord, any]("inside"))),
			Alias("t1", TypeName(Field[*AvroRecord, any]("insidearr"))),
		).Query(StatementName("type-name-fragment-avro")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := make([]Row, 0, 1)
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok {
					return NewError(ErrorTypeMismatch, "Avro fragment type-name result is not a row")
				}
				rows = append(rows, row)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		record, err := NewAvroRecordFromMap(root, map[string]any{
			"inside":    map[string]any{"key": "k"},
			"insidearr": []map[string]any{},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := engine.SendAvro(context.Background(), root.Name(), record); err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || !rows[0].Get("t0").Equal(Present("TypeNameAvroInner")) || !rows[0].Get("t1").Equal(Present("TypeNameAvroInner[]")) {
			t.Fatalf("Avro fragment row = %#v", rows)
		}
	})
}
