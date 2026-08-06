package esper

import (
	"context"
	"reflect"
	"testing"
)

type subqueryMultirowValue struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

type subqueryMultirowTrigger struct {
	P00 string `esper:"p00"`
}

// TestSubqueryMultirowWindowValuesMatchesEsper covers the two Java
// EPLSubselectMultirow shapes: an event-stream keep-all subquery and a late
// named-window consumer whose length retention is already populated.
func TestSubqueryMultirowWindowValuesMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[subqueryMultirowValue](env, "SubqueryMultirowValue"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subqueryMultirowTrigger](env, "SubqueryMultirowTrigger"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SubqueryMultirowValue")
	if !ok {
		t.Fatal("subquery multirow value schema is missing")
	}
	const namedWindowName = "SubqueryMultirowLength"
	if _, err := CreateNamedWindow(env, namedWindowName, schema, NamedWindowRetention(LengthWindow(3))); err != nil {
		t.Fatal(err)
	}

	innerStream := Select(From[subqueryMultirowValue](env, "SubqueryMultirowValue")).Window(KeepAll())
	innerValues := Field[any, int64]("value")
	direct := SubqueryValue[[]int64](innerStream, WindowValues[int64](innerValues))
	named := SubqueryValue[[]int64](FromNamedWindow(env, namedWindowName), WindowValues[int64](innerValues))
	query := Select(
		From[subqueryMultirowTrigger](env, "SubqueryMultirowTrigger"),
		Alias("direct", direct),
		Alias("named", named),
	).Query(StatementName("subquery-multirow-window-values"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	// Populate the named window before the consumer is deployed. This is the
	// Java late-start case: the length(3) snapshot must expose 10, 15 and 6.
	for _, value := range []int64{5, 10, 15, 6} {
		if err := engine.InsertNamedWindow(context.Background(), namedWindowName, subqueryMultirowValue{Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("multirow window result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), subqueryMultirowTrigger{P00: "before"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("empty event-stream subquery rows = %#v", rows)
	}
	if !rows[0].Get("direct").IsNull() {
		t.Fatalf("empty window aggregate = %#v, want null", rows[0].Get("direct"))
	}
	namedValues, ok := rows[0].Get("named").Any().([]int64)
	if !ok || !reflect.DeepEqual(namedValues, []int64{10, 15, 6}) {
		t.Fatalf("late named-window values = %#v, want [10 15 6]", rows[0].Get("named"))
	}

	for _, value := range []int64{5, 10, 15, 6} {
		if err := engine.SendEvent(context.Background(), subqueryMultirowValue{Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SendEvent(context.Background(), subqueryMultirowTrigger{P00: "after"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("populated event-stream subquery rows = %#v", rows)
	}
	directValues, ok := rows[1].Get("direct").Any().([]int64)
	if !ok || !reflect.DeepEqual(directValues, []int64{5, 10, 15, 6}) {
		t.Fatalf("keep-all subquery values = %#v, want [5 10 15 6]", rows[1].Get("direct"))
	}
	if typ, ok := rows[1].Schema().PropertyType("direct"); !ok || typ != reflect.TypeOf([]int64{}) {
		t.Fatalf("multirow value property type = %v, found=%v", typ, ok)
	}
}

// TestSubqueryMultirowUnderlyingCorrelatesAndPreservesEvents covers
// window(sb.*): an empty correlated result is null, while a non-empty result
// remains a typed []Event whose underlying values are not projected maps.
func TestSubqueryMultirowUnderlyingCorrelatesAndPreservesEvents(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[subqueryMultirowValue](env, "SubqueryUnderlyingValue"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subqueryMultirowTrigger](env, "SubqueryUnderlyingTrigger"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SubqueryUnderlyingValue")
	if !ok {
		t.Fatal("subquery underlying schema is missing")
	}
	const windowName = "SubqueryUnderlyingWindow"
	if _, err := CreateNamedWindow(env, windowName, schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	key := Field[any, string]("key")
	underlying := SubqueryValueWithOptions[[]Event](
		FromNamedWindow(env, windowName),
		WindowEvents(),
		SubqueryWhere(Equal[string](key, OuterField[string]("p00"))),
	)
	query := Select(
		From[subqueryMultirowTrigger](env, "SubqueryUnderlyingTrigger"),
		Alias("p00", Field[subqueryMultirowTrigger, string]("p00")),
		Alias("val", underlying),
	).Query(StatementName("subquery-multirow-underlying"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("underlying subquery result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	trigger := func(key string) Row {
		t.Helper()
		before := len(rows)
		if err := engine.SendEvent(context.Background(), subqueryMultirowTrigger{P00: key}); err != nil {
			t.Fatal(err)
		}
		if len(rows) != before+1 {
			t.Fatalf("underlying trigger %q rows = %#v", key, rows)
		}
		return rows[len(rows)-1]
	}
	if row := trigger("T1"); !row.Get("val").IsNull() {
		t.Fatalf("empty correlated underlying = %#v, want null", row.Get("val"))
	}

	first := subqueryMultirowValue{Key: "T1", Value: 10}
	if err := engine.InsertNamedWindow(context.Background(), windowName, first); err != nil {
		t.Fatal(err)
	}
	row := trigger("T1")
	events, ok := row.Get("val").Any().([]Event)
	if !ok || len(events) != 1 || !reflect.DeepEqual(events[0].Underlying(), first) {
		t.Fatalf("single correlated underlying = %#v, want %#v", row.Get("val"), first)
	}

	second := subqueryMultirowValue{Key: "T2", Value: 20}
	third := subqueryMultirowValue{Key: "T2", Value: 30}
	for _, value := range []subqueryMultirowValue{second, third} {
		if err := engine.InsertNamedWindow(context.Background(), windowName, value); err != nil {
			t.Fatal(err)
		}
	}
	row = trigger("T2")
	events, ok = row.Get("val").Any().([]Event)
	if !ok || len(events) != 2 || !reflect.DeepEqual(events[0].Underlying(), second) || !reflect.DeepEqual(events[1].Underlying(), third) {
		t.Fatalf("correlated underlying events = %#v, want [%#v %#v]", row.Get("val"), second, third)
	}
	if typ, ok := row.Schema().PropertyType("val"); !ok || typ != reflect.TypeOf([]Event{}) {
		t.Fatalf("underlying property type = %v, found=%v", typ, ok)
	}
}

// TestSubqueryMultirowEmptyCollectionsRemainEnumerable makes the Go slice
// contract explicit: collection subqueries materialize an empty slice, so
// enum methods can be chained without a null special case. Aggregate window
// access above intentionally keeps the separate Esper null-on-empty behavior.
func TestSubqueryMultirowEmptyCollectionsRemainEnumerable(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[subqueryMultirowValue](env, "SubqueryEnumerableValue"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subqueryMultirowTrigger](env, "SubqueryEnumerableTrigger"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SubqueryEnumerableValue")
	if !ok {
		t.Fatal("subquery enumerable schema is missing")
	}
	const windowName = "SubqueryEnumerableWindow"
	if _, err := CreateNamedWindow(env, windowName, schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	window := FromNamedWindow(env, windowName)
	events := SubqueryEvents(window)
	values := SubqueryValues[int64](window, Field[any, int64]("value"))
	keys := EnumSelect[Event, string](events, EnumField[Event, string]("key"))
	query := Select(
		From[subqueryMultirowTrigger](env, "SubqueryEnumerableTrigger"),
		Alias("values", values),
		Alias("events", events),
		Alias("keys", keys),
		Alias("valueCount", EnumCount[int64](values)),
	).Query(StatementName("subquery-multirow-enumerable"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("enumerable subquery result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	trigger := func() Row {
		t.Helper()
		before := len(rows)
		if err := engine.SendEvent(context.Background(), subqueryMultirowTrigger{}); err != nil {
			t.Fatal(err)
		}
		if len(rows) != before+1 {
			t.Fatalf("enumerable subquery rows = %#v", rows)
		}
		return rows[len(rows)-1]
	}
	row := trigger()
	emptyValues, ok := row.Get("values").Any().([]int64)
	if !ok || !reflect.DeepEqual(emptyValues, []int64{}) {
		t.Fatalf("empty subquery values = %#v, want []int64{}", row.Get("values"))
	}
	emptyEvents, ok := row.Get("events").Any().([]Event)
	if !ok || !reflect.DeepEqual(emptyEvents, []Event{}) {
		t.Fatalf("empty subquery events = %#v, want []Event{}", row.Get("events"))
	}
	emptyKeys, ok := row.Get("keys").Any().([]string)
	if !ok || !reflect.DeepEqual(emptyKeys, []string{}) || row.Get("valueCount").Any() != int64(0) {
		t.Fatalf("empty enumerable chain = %#v", row)
	}

	for _, value := range []subqueryMultirowValue{{Key: "A", Value: 5}, {Key: "B", Value: 10}} {
		if err := engine.InsertNamedWindow(context.Background(), windowName, value); err != nil {
			t.Fatal(err)
		}
	}
	row = trigger()
	gotValues, ok := row.Get("values").Any().([]int64)
	if !ok || !reflect.DeepEqual(gotValues, []int64{5, 10}) {
		t.Fatalf("enumerable subquery values = %#v", row.Get("values"))
	}
	gotKeys, ok := row.Get("keys").Any().([]string)
	if !ok || !reflect.DeepEqual(gotKeys, []string{"A", "B"}) || row.Get("valueCount").Any() != int64(2) {
		t.Fatalf("enumerable subquery chain = %#v", row)
	}
}
