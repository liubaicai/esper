package esper

import (
	"context"
	"reflect"
	"testing"
)

type joinInheritAImpl struct {
	ValueA      string `esper:"a"`
	ValueBaseAB string `esper:"baseAB"`
}

type joinInheritBImpl struct {
	ValueB      string `esper:"b"`
	ValueBaseAB string `esper:"baseAB"`
}

func TestJoinInterfaceAndInheritedEventTypesMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	stringType := reflect.TypeOf("")
	aSchema, err := NewSchema("ISupportA", FieldDef("a", stringType), FieldDef("baseAB", stringType))
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(aSchema); err != nil {
		t.Fatal(err)
	}
	bSchema, err := NewSchema("ISupportB", FieldDef("b", stringType), FieldDef("baseAB", stringType))
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(bSchema); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinInheritAImpl](env, "ISupportAImpl", WithAccessorStyle(AccessorExplicit), WithSchemaParent(aSchema)); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinInheritBImpl](env, "ISupportBImpl", WithAccessorStyle(AccessorExplicit), WithSchemaParent(bSchema)); err != nil {
		t.Fatal(err)
	}

	plan, err := env.Build(Join(
		From[any](env, "ISupportA").Window(LengthWindow(10)),
		From[any](env, "ISupportB").Window(LengthWindow(10)),
		OnEqual(Field[any, string]("a"), Field[any, string]("b")),
	).Select(
		SelectFrom(0, "a", JoinField[string](0, "a")),
		SelectFrom(1, "b", JoinField[string](1, "b")),
	).Query(StatementName("join-interface-inheritance")))
	if err != nil {
		t.Fatal(err)
	}

	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())

	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), joinInheritAImpl{ValueA: "1", ValueBaseAB: "ab1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), joinInheritBImpl{ValueB: "2", ValueBaseAB: "ab2"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("non-matching inherited join emitted %#v", rows)
	}
	if err := engine.SendEvent(context.Background(), joinInheritBImpl{ValueB: "1", ValueBaseAB: "ab3"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("matching inherited join rows = %#v, want one row", rows)
	}
	if rows[0].Get("a").Any() != "1" || rows[0].Get("b").Any() != "1" {
		t.Fatalf("inherited interface join projection = %#v", rows[0].AsMap())
	}
}
