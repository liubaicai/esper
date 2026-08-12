package esper

import (
	"context"
	"reflect"
	"testing"
)

// TestExprEnumSelectFromEventsPlainParity covers the Java
// ExprEnumSelectFromEventsPlain execution through a typed event source.
func TestExprEnumSelectFromEventsPlainParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumExpressionContainer](env, "SelectFromEventsContainer"); err != nil {
		t.Fatal(err)
	}

	items := Field[enumExpressionContainer, []enumExpressionItem]("items")
	ids := EnumSelect[enumExpressionItem, string](items, EnumField[enumExpressionItem, string]("id"))
	nulls := EnumSelect[enumExpressionItem, string](items, NullLiteral[string]())
	plan, err := env.Build(Select(From[enumExpressionContainer](env, "SelectFromEventsContainer"),
		Alias("ids", ids),
		Alias("nulls", nulls),
	).Query(StatementName("enum-select-from-events-plain")))
	if err != nil {
		t.Fatal(err)
	}

	schema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("select-from event result schema is missing")
	}
	for _, name := range []string{"ids", "nulls"} {
		field, exists := schema.Field(name)
		if !exists || field.Type != reflect.TypeOf([]string{}) {
			t.Fatalf("select-from event field %q = %#v, want []string", name, field)
		}
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectDotRows(t, deployment)

	input := []enumExpressionItem{{ID: "E1", Score: 12}, {ID: "E2", Score: 11}, {ID: "E3", Score: 2}}
	if err := engine.SendEvent(context.Background(), enumExpressionContainer{Items: input}); err != nil {
		t.Fatal(err)
	}
	row := (*rows)[0]
	if !row.Get("ids").Equal(Present([]string{"E1", "E2", "E3"})) {
		t.Fatalf("select-from event ids = %v", row.Get("ids"))
	}
	if !row.Get("nulls").Equal(Present([]string{})) {
		t.Fatalf("select-from event null selector = %v, want empty collection", row.Get("nulls"))
	}

	if err := engine.SendEvent(context.Background(), enumExpressionContainer{Items: []enumExpressionItem{}}); err != nil {
		t.Fatal(err)
	}
	empty := (*rows)[1]
	if !empty.Get("ids").Equal(Present([]string{})) || !empty.Get("nulls").Equal(Present([]string{})) {
		t.Fatalf("select-from event empty = %#v", empty.AsMap())
	}

	// A map-backed event preserves Java's null collection state. Struct-backed
	// events intentionally keep typed-nil slices as present values in Go's
	// value model, so exercise the Java null case through a dynamic source.
	if _, err := RegisterMap(env, "SelectFromEventsMap", []FieldSpec{
		FieldDef("items", reflect.TypeOf([]enumExpressionItem{})),
	}); err != nil {
		t.Fatal(err)
	}
	mapItems := Field[any, []enumExpressionItem]("items")
	mapPlan, err := env.Build(FromAny(env, "SelectFromEventsMap").Select(
		Alias("ids", EnumSelect[enumExpressionItem, string](mapItems, EnumField[enumExpressionItem, string]("id"))),
		Alias("nulls", EnumSelect[enumExpressionItem, string](mapItems, NullLiteral[string]())),
	).Query(StatementName("enum-select-from-events-plain-map")))
	if err != nil {
		t.Fatal(err)
	}
	mapEngine := NewEngine(env)
	mapDeployment, err := mapEngine.Deploy(context.Background(), mapPlan)
	if err != nil {
		t.Fatal(err)
	}
	mapRows := collectDotRows(t, mapDeployment)
	if err := mapEngine.SendRecord(context.Background(), "SelectFromEventsMap", map[string]any{"items": nil}); err != nil {
		t.Fatal(err)
	}
	mapNull := (*mapRows)[0]
	if !mapNull.Get("ids").IsNull() || !mapNull.Get("nulls").IsNull() {
		t.Fatalf("select-from map null collection = %#v", mapNull.AsMap())
	}
}
