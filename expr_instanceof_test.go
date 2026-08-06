package esper

import (
	"context"
	"reflect"
	"testing"
)

type instanceMarker interface {
	marker()
}

type instanceMarkerValue struct{}

func (instanceMarkerValue) marker() {}

type instanceOfEvent struct {
	Item any `esper:"item"`
}

func TestInstanceOfExpressionsMatchJavaDynamicAndInterfaceSemantics(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[instanceOfEvent](env, "InstanceOfEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[instanceOfEvent](env, "InstanceOfEvent")
	item := Property[any](EventValue[instanceOfEvent](), "Item")
	text := Literal("abc")
	marker := Literal[any](instanceMarkerValue{})
	plan, err := env.Build(Select(input,
		Alias("text", InstanceOf[string](text)),
		Alias("text_float", InstanceOf[float64](text)),
		Alias("text_any", InstanceOf[any](text)),
		Alias("item_int", InstanceOf[int](item)),
		Alias("item_string", InstanceOf[string](item)),
		Alias("marker", InstanceOf[instanceMarker](marker)),
		Alias("marker_pointer", InstanceOf[instanceMarker](Literal[any](&instanceMarkerValue{}))),
		Alias("null", InstanceOf[string](NullLiteral[string]())),
	).Query(StatementName("instance-of")))
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(plan.Query())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent instance-of plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("instance-of result schema is missing")
	}
	for _, name := range []string{"text", "text_float", "text_any", "item_int", "item_string", "marker", "marker_pointer", "null"} {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != typeOf[bool]() {
			t.Fatalf("instance-of field %q = %#v, want bool", name, field)
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
				return NewError(ErrorTypeMismatch, "instance-of result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []instanceOfEvent{{Item: 100}, {Item: "value"}, {Item: nil}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 3 {
		t.Fatalf("instance-of rows = %d, want 3", len(rows))
	}
	for index, row := range rows {
		wantText := true
		wantItemInt := index == 0
		wantItemString := index == 1
		for name, want := range map[string]bool{
			"text":           wantText,
			"text_float":     false,
			"text_any":       true,
			"item_int":       wantItemInt,
			"item_string":    wantItemString,
			"marker":         true,
			"marker_pointer": true,
			"null":           false,
		} {
			if got := row.Get(name); !got.Equal(Present(want)) {
				t.Fatalf("row %d %s = %v, want %t", index, name, got, want)
			}
		}
	}
}
