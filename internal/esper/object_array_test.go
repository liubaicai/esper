package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type objectArrayNestedTrade struct {
	Price float64 `esper:"price"`
}

func objectArrayTestSchema(t *testing.T, env *Environment, name string) Schema {
	t.Helper()
	schema, err := RegisterObjectArray(env, name, []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("price", reflect.TypeOf(float64(0))),
		FieldDef("quantity", reflect.TypeOf(int64(0))),
		FieldDef("metadata", reflect.TypeOf(map[string]any{})),
	})
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func TestObjectArraySchemaAccessCoercionAndBounds(t *testing.T) {
	env := NewEnvironment()
	schema := objectArrayTestSchema(t, env, "ObjectArrayTrade")
	event, err := ParseObjectArray(schema, []any{
		"A",
		12,
		int(3),
		map[string]any{"desk": "D1"},
	}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	values, ok := event.Underlying().([]any)
	if !ok || len(values) != 4 {
		t.Fatalf("normalized object-array underlying = %#v", event.Underlying())
	}
	if got := event.Get("symbol").Any(); got != "A" {
		t.Fatalf("symbol = %#v", got)
	}
	if got, err := As[float64](event.Get("price")); err != nil || got != 12 {
		t.Fatalf("coerced price = %v, %v", got, err)
	}
	if got, err := As[int64](event.Get("quantity")); err != nil || got != 3 {
		t.Fatalf("coerced quantity = %v, %v", got, err)
	}
	if got := event.Get("metadata.desk").Any(); got != "D1" {
		t.Fatalf("nested metadata = %#v", got)
	}
	if _, err := ParseObjectArray(schema, []any{"A", 1}, time.Unix(0, 0).UTC()); err == nil {
		t.Fatal("short object-array event was accepted")
	}
	if _, err := ParseObjectArray(schema, []any{"A", "not-a-number", 3, nil}, time.Unix(0, 0).UTC()); err == nil {
		t.Fatal("object-array field type mismatch was accepted")
	}
	if _, err := ParseObjectArray(mapSchemaForObjectArrayTest(t), []any{"A"}, time.Unix(0, 0).UTC()); err == nil {
		t.Fatal("non-object-array schema was accepted by ParseObjectArray")
	}
}

func mapSchemaForObjectArrayTest(t *testing.T) Schema {
	t.Helper()
	schema, err := NewMapSchema("MapForObjectArrayTest", []FieldSpec{FieldDef("symbol", reflect.TypeOf(""))})
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func TestObjectArrayEngineSendAndFluentFilter(t *testing.T) {
	env := NewEnvironment()
	objectSchema := objectArrayTestSchema(t, env, "ObjectArrayQuote")
	engine := NewEngine(env)
	stream := FromAny(env, "ObjectArrayQuote").Filter(
		Greater[float64](Field[any, float64]("price"), Literal(10.0)),
	)
	plan, err := env.Build(stream.Query(StatementName("object-array-filter")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendObjectArray(context.Background(), "ObjectArrayQuote", []any{"A", 11.5, 2, map[string]any{"desk": "D1"}}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendObjectArray(context.Background(), "ObjectArrayQuote", []any{"B", 9.5, 1, nil}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("object-array filter batches = %#v", batches)
	}
	event, ok := batches[0].New[0].Event()
	if !ok || event.Schema().Name() != objectSchema.Name() || event.Get("symbol").Any() != "A" || event.Get("price").Any() != 11.5 {
		t.Fatalf("object-array filtered event = %#v", batches[0].New[0])
	}
	if err := engine.SendObjectArray(context.Background(), "ObjectArrayQuote", []any{"too-short"}); err == nil {
		t.Fatal("SendObjectArray accepted an invalid field count")
	}
	if err := engine.SendObjectArray(context.Background(), "ObjectArrayQuote", []any{"A", "bad", 2, nil}); err == nil {
		t.Fatal("SendObjectArray accepted an invalid field type")
	}
}

func TestObjectArrayNamedWindowMutationPreservesRepresentation(t *testing.T) {
	env := NewEnvironment()
	schema := objectArrayTestSchema(t, env, "ObjectArrayStateEvent")
	if _, err := env.RegisterNamedWindow("object-array-window", schema); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	source := FromAny(env, "ObjectArrayStateEvent")
	plan, err := env.Build(OnRecord(source).UpdateNamedWindow(
		"object-array-window",
		Equal[string](NamedWindowField[string]("symbol"), Field[any, string]("symbol")),
		SetColumn("price", Field[any, float64]("price")),
	).Query(StatementName("object-array-window-update")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	if err := engine.InsertNamedWindow(context.Background(), "object-array-window", []any{"A", 1.0, int64(1), nil}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendObjectArray(context.Background(), "ObjectArrayStateEvent", []any{"A", 9.0, 1, nil}); err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("object-array-window")
	if !ok {
		t.Fatal("object-array-window is missing")
	}
	events, err := window.Snapshot(context.Background())
	if err != nil || len(events) != 1 {
		t.Fatalf("object-array window snapshot = %#v, err=%v", events, err)
	}
	values, ok := events[0].Underlying().([]any)
	if !ok || len(values) != 4 || values[0] != "A" || values[1] != 9.0 {
		t.Fatalf("object-array window representation = %#v", events[0].Underlying())
	}
}

func TestObjectArrayNestedArrayAndPropertyExpressions(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterObjectArray(env, "ObjectArrayNested", []FieldSpec{
		FieldDef("numbers", reflect.TypeOf([]int{})),
		FieldDef("trades", reflect.TypeOf([]objectArrayNestedTrade{})),
		FieldDef("metadata", reflect.TypeOf(map[string]any{})),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	source := From[any](env, "ObjectArrayNested")
	numbers := Field[any, []int]("numbers")
	trades := Field[any, []objectArrayNestedTrade]("trades")
	metadata := Field[any, map[string]any]("metadata")
	projected := Select(source,
		Alias("first-number", ArrayAt[int](numbers, Literal[int64](0))),
		Alias("second-price", Property[float64](ArrayAt[objectArrayNestedTrade](trades, Literal[int64](1)), "price")),
		Alias("desk", Property[string](metadata, "desk")),
		Alias("missing-number", ArrayAt[int](numbers, Literal[int64](8))),
	)
	plan, err := env.Build(projected.Query(StatementName("object-array-nested")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var row Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) != 1 {
			return nil
		}
		var ok bool
		row, ok = batch.New[0].Row()
		if !ok {
			t.Fatalf("object-array nested result is not a row: %#v", batch.New)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendObjectArray(context.Background(), "ObjectArrayNested", []any{
		[]int{3, 5},
		[]objectArrayNestedTrade{{Price: 1}, {Price: 7.5}},
		map[string]any{"desk": "D1"},
	}); err != nil {
		t.Fatal(err)
	}
	if row.Get("first-number").Any() != 3 || row.Get("second-price").Any() != 7.5 || row.Get("desk").Any() != "D1" || !row.Get("missing-number").IsNull() {
		t.Fatalf("object-array nested projection = %#v", row.AsMap())
	}
}
