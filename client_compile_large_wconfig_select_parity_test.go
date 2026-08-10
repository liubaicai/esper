package esper

import (
	"context"
	"fmt"
	"testing"
)

type clientCompileLargeSelectEvent struct {
	TheString string `esper:"theString"`
}

func TestClientCompileLargeSelectColMatchesEsper(t *testing.T) {
	for _, representation := range []struct {
		name     string
		kind     SchemaKind
		register func(*Environment, string, []FieldSpec) (Schema, error)
	}{
		{name: "map", kind: SchemaMap, register: func(env *Environment, name string, fields []FieldSpec) (Schema, error) {
			return RegisterMap(env, name, fields)
		}},
		{name: "object-array", kind: SchemaObjectArray, register: func(env *Environment, name string, fields []FieldSpec) (Schema, error) {
			return RegisterObjectArray(env, name, fields)
		}},
	} {
		t.Run(representation.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[clientCompileLargeSelectEvent](env, "ClientCompileLargeSelectEvent"); err != nil {
				t.Fatal(err)
			}
			const columnCount = 5000
			fields := make([]FieldSpec, columnCount)
			selections := make([]Selection, columnCount)
			theString := Field[clientCompileLargeSelectEvent, string]("theString")
			for index := 0; index < columnCount; index++ {
				name := fmt.Sprintf("c%d", index)
				fields[index] = FieldDef(name, typeOf[string]())
				selections[index] = Alias(name, Concat(theString, Literal(fmt.Sprintf("%d", index))))
			}
			targetName := "ClientCompileLargeSelect" + representation.name
			targetSchema, err := representation.register(env, targetName, fields)
			if err != nil {
				t.Fatal(err)
			}
			if targetSchema.Kind() != representation.kind || len(targetSchema.Fields()) != columnCount {
				t.Fatalf("large %s target schema = kind %v fields %d", representation.name, targetSchema.Kind(), len(targetSchema.Fields()))
			}

			producer, err := env.Build(Select(
				From[clientCompileLargeSelectEvent](env, "ClientCompileLargeSelectEvent"),
				selections...,
			).InsertInto(targetName, StatementName("large-select-producer")))
			if err != nil {
				t.Fatal(err)
			}
			consumer, err := env.Build(FromAny(env, targetName).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}

			engine := NewEngine(env)
			consumerDeployment, err := engine.Deploy(context.Background(), consumer)
			if err != nil {
				t.Fatal(err)
			}
			statement, ok := consumerDeployment.Statement("s0")
			if !ok {
				t.Fatal("large representation consumer s0 is missing")
			}
			received := false
			if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
				if len(batch.New) != 1 {
					t.Fatalf("large %s batch = %#v", representation.name, batch)
				}
				event, ok := batch.New[0].Event()
				if !ok {
					t.Fatalf("large %s result is not an event", representation.name)
				}
				if event.Schema().Kind() != representation.kind || len(event.Schema().Fields()) != columnCount {
					t.Fatalf("large %s event schema = kind %v fields %d", representation.name, event.Schema().Kind(), len(event.Schema().Fields()))
				}
				switch underlying := event.Underlying().(type) {
				case map[string]any:
					if representation.kind != SchemaMap || len(underlying) != columnCount {
						t.Fatalf("large %s map underlying size = %d", representation.name, len(underlying))
					}
				case []any:
					if representation.kind != SchemaObjectArray || len(underlying) != columnCount {
						t.Fatalf("large %s object-array underlying size = %d", representation.name, len(underlying))
					}
				default:
					t.Fatalf("large %s underlying type = %T", representation.name, underlying)
				}
				for index := 0; index < columnCount; index++ {
					name := fmt.Sprintf("c%d", index)
					want := fmt.Sprintf("x%d", index)
					if got := event.Get(name).Any(); got != want {
						t.Fatalf("large %s %s = %#v, want %q", representation.name, name, got, want)
					}
				}
				received = true
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Deploy(context.Background(), producer); err != nil {
				t.Fatal(err)
			}
			if err := engine.Send(context.Background(), "ClientCompileLargeSelectEvent", clientCompileLargeSelectEvent{TheString: "x"}); err != nil {
				t.Fatal(err)
			}
			if !received {
				t.Fatalf("large %s projection produced no event", representation.name)
			}
		})
	}
}
