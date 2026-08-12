package esper

import (
	"context"
	"reflect"
	"testing"
)

// TestSubqueryRowsContinueIntoEnumerableChain covers the Java multi-row
// subselect-plus-enum footprint while using Go-native row maps and typed
// chain expressions instead of an EPL lambda string.
func TestSubqueryRowsContinueIntoEnumerableChain(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "SubqueryEnumTrade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SubqueryEnumTrade")
	if !ok {
		t.Fatal("subquery enum trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "SubqueryEnumPrices", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	prices := FromNamedWindow(env, "SubqueryEnumPrices")
	innerRows := SubqueryRows(prices,
		Alias("symbol", Field[any, string]("symbol")),
		Alias("price", Field[any, float64]("price")),
	)
	rowPrice := Property[float64](EnumElement[map[string]any](), "price")
	filtered := EnumWhere[map[string]any](innerRows, Greater[float64](rowPrice, Literal(9.0)))
	projected := EnumSelectMap[map[string]any](filtered,
		Alias("symbol", Property[string](EnumElement[map[string]any](), "symbol")),
		Alias("position", EnumIndex()),
	)
	plan, err := env.Build(Select(
		From[runtimeTestTrade](env, "SubqueryEnumTrade"),
		Alias("rows", projected),
	).Query(StatementName("subquery-enum-chain")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var output []map[string]any
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("subquery enum result is not a row: %#v", result)
			}
			value, ok := row.Get("rows").Any().([]map[string]any)
			if !ok {
				t.Fatalf("subquery enum rows = %#v", row.Get("rows"))
			}
			output = append(output, value...)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, trade := range []runtimeTestTrade{
		{Symbol: "A", Price: 5},
		{Symbol: "A", Price: 10},
		{Symbol: "B", Price: 20},
		{Symbol: "C", Price: 2},
	} {
		if err := engine.InsertNamedWindow(context.Background(), "SubqueryEnumPrices", trade); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "outer"}); err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{
		{"symbol": "A", "position": int64(0)},
		{"symbol": "B", "position": int64(1)},
	}
	if !reflect.DeepEqual(output, want) {
		t.Fatalf("subquery enum chain = %#v, want %#v", output, want)
	}
}
