package esper

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type clientCompileLargeSchemaTrigger struct{}

func TestClientCompileLargeCreateSchemaAndInsertMatchesEsper(t *testing.T) {
	tests := []struct {
		name     string
		kind     SchemaKind
		widening bool
		register func(*Environment, string, []FieldSpec) (Schema, error)
	}{
		{name: "map-current-timestamp", kind: SchemaMap, register: func(env *Environment, name string, fields []FieldSpec) (Schema, error) {
			return RegisterMap(env, name, fields)
		}},
		{name: "map-widening", kind: SchemaMap, widening: true, register: func(env *Environment, name string, fields []FieldSpec) (Schema, error) {
			return RegisterMap(env, name, fields)
		}},
		{name: "object-array-current-timestamp", kind: SchemaObjectArray, register: func(env *Environment, name string, fields []FieldSpec) (Schema, error) {
			return RegisterObjectArray(env, name, fields)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			const columnCount = 5000
			env := NewEnvironment()
			if _, err := RegisterStruct[clientCompileLargeSchemaTrigger](env, "ClientCompileLargeSchemaTrigger"); err != nil {
				t.Fatal(err)
			}
			fields := make([]FieldSpec, columnCount)
			for index := range fields {
				fields[index] = FieldDef(fmt.Sprintf("p%d", columnCount-index-1), typeOf[int64]())
			}
			targetName := "ClientCompileLargeSchema" + test.name
			targetSchema, err := test.register(env, targetName, fields)
			if err != nil {
				t.Fatal(err)
			}
			registeredFields := targetSchema.Fields()
			if targetSchema.Kind() != test.kind || len(registeredFields) != columnCount || registeredFields[0].Name != "p4999" || registeredFields[columnCount-1].Name != "p0" {
				t.Fatalf("large schema = kind %v fields %d first %q last %q", targetSchema.Kind(), len(registeredFields), registeredFields[0].Name, registeredFields[len(registeredFields)-1].Name)
			}
			for index := 0; index < columnCount; index++ {
				field, ok := targetSchema.Field(fmt.Sprintf("p%d", index))
				if !ok || field.Type != typeOf[int64]() {
					t.Fatalf("large schema property p%d = %#v, ok=%t", index, field, ok)
				}
			}

			selections := make([]Selection, columnCount)
			for index := range selections {
				var value Expr = CurrentTimestamp()
				if test.widening {
					value = Literal[int64](1_000_000)
				}
				selections[index] = Alias(
					fmt.Sprintf("p%d", index),
					AddOf[int64](Literal(index), value),
				)
			}
			producer, err := env.Build(Select(
				From[clientCompileLargeSchemaTrigger](env, "ClientCompileLargeSchemaTrigger"),
				selections...,
			).InsertInto(targetName, StatementName("large-schema-producer")))
			if err != nil {
				t.Fatal(err)
			}
			consumer, err := env.Build(FromAny(env, targetName).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}

			engine := NewEngine(env)
			if err := engine.AdvanceTime(context.Background(), time.UnixMilli(1_000_000)); err != nil {
				t.Fatal(err)
			}
			consumerDeployment, err := engine.Deploy(context.Background(), consumer)
			if err != nil {
				t.Fatal(err)
			}
			statement, ok := consumerDeployment.Statement("s0")
			if !ok {
				t.Fatal("large schema consumer s0 is missing")
			}
			received := false
			if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
				if len(batch.New) != 1 {
					t.Fatalf("large schema batch = %#v", batch)
				}
				event, ok := batch.New[0].Event()
				if !ok || event.Schema().Kind() != test.kind {
					t.Fatalf("large schema event = %#v, ok=%t", batch.New[0], ok)
				}
				for index := 0; index < columnCount; index++ {
					want := int64(index + 1_000_000)
					if got := event.Get(fmt.Sprintf("p%d", index)).Any(); got != want {
						t.Fatalf("large schema p%d = %#v, want %d", index, got, want)
					}
				}
				switch underlying := event.Underlying().(type) {
				case map[string]any:
					if test.kind != SchemaMap || len(underlying) != columnCount {
						t.Fatalf("large schema map underlying = %d", len(underlying))
					}
				case []any:
					if test.kind != SchemaObjectArray || len(underlying) != columnCount || underlying[0] != int64(1_004_999) || underlying[columnCount-1] != int64(1_000_000) {
						t.Fatalf("large schema object-array boundaries = len %d first %#v last %#v", len(underlying), underlying[0], underlying[len(underlying)-1])
					}
				default:
					t.Fatalf("large schema underlying type = %T", underlying)
				}
				received = true
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Deploy(context.Background(), producer); err != nil {
				t.Fatal(err)
			}
			if err := engine.Send(context.Background(), "ClientCompileLargeSchemaTrigger", clientCompileLargeSchemaTrigger{}); err != nil {
				t.Fatal(err)
			}
			if !received {
				t.Fatal("large schema insert produced no event")
			}
		})
	}
}
