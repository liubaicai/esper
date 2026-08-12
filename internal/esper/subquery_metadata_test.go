package esper

import (
	"context"
	"reflect"
	"testing"
)

func TestSubqueryMultiColumnMetadataMatchesEsperFragmentShape(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "SubqueryMetadataTrade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SubqueryMetadataTrade")
	if !ok {
		t.Fatal("subquery metadata schema is missing")
	}
	if _, err := CreateNamedWindow(env, "SubqueryMetadataPrices", schema); err != nil {
		t.Fatal(err)
	}

	prices := FromNamedWindow(env, "SubqueryMetadataPrices")
	price := Field[any, float64]("price")
	scalar := SubqueryRow(prices,
		Alias("count", Count[float64](price)),
		Alias("sum", Sum[float64](price)),
	)
	metadata, ok := SubqueryMetadata(scalar)
	if !ok {
		t.Fatal("multi-column scalar subquery metadata is missing")
	}
	if metadata.ResultType != reflect.TypeOf(map[string]any{}) || metadata.Indexed || !metadata.Native {
		t.Fatalf("scalar subquery metadata shape = %#v", metadata)
	}
	if len(metadata.Columns) != 2 {
		t.Fatalf("scalar subquery columns = %#v", metadata.Columns)
	}
	if metadata.Columns[0].Name != "count" || metadata.Columns[0].Type != reflect.TypeOf(int64(0)) || !metadata.Columns[0].Optional {
		t.Fatalf("count column metadata = %#v", metadata.Columns[0])
	}
	if column, ok := metadata.Column("sum"); !ok || column.Type != reflect.TypeOf(float64(0)) {
		t.Fatalf("sum column metadata = %#v, found=%v", column, ok)
	}

	rows := SubqueryRows(prices,
		Alias("symbol", Field[any, string]("symbol")),
		Alias("price", price),
	)
	rowsMetadata, ok := SubqueryMetadata(rows)
	if !ok {
		t.Fatal("multi-row subquery metadata is missing")
	}
	if rowsMetadata.ResultType != reflect.TypeOf([]map[string]any{}) || !rowsMetadata.Indexed || !rowsMetadata.Native {
		t.Fatalf("multi-row subquery metadata shape = %#v", rowsMetadata)
	}
	if len(rowsMetadata.Columns) != 2 || rowsMetadata.Columns[0].Name != "symbol" || rowsMetadata.Columns[1].Name != "price" {
		t.Fatalf("multi-row subquery columns = %#v", rowsMetadata.Columns)
	}

	groupRows := SubqueryGroupRows(
		prices,
		Field[any, string]("symbol"),
		[]Selection{
			Alias("symbol", Field[any, string]("symbol")),
			Alias("total", Sum[float64](price)),
		},
	)
	groupMetadata, ok := SubqueryMetadata(groupRows)
	if !ok || !groupMetadata.Indexed || groupMetadata.ResultType != reflect.TypeOf([]map[string]any{}) || len(groupMetadata.Columns) != 2 {
		t.Fatalf("grouped subquery metadata = %#v, found=%v", groupMetadata, ok)
	}

	nested := SubqueryRow(prices, Alias("history", rows))
	nestedMetadata, ok := SubqueryMetadata(nested)
	if !ok {
		t.Fatal("nested subquery metadata is missing")
	}
	history, ok := nestedMetadata.Column("history")
	if !ok || history.Fragment == nil || !history.Fragment.Indexed || history.Fragment.ResultType != reflect.TypeOf([]map[string]any{}) {
		t.Fatalf("nested subquery fragment metadata = %#v, found=%v", history, ok)
	}
	if len(history.Fragment.Columns) != 2 || history.Fragment.Columns[0].Name != "symbol" {
		t.Fatalf("nested fragment columns = %#v", history.Fragment.Columns)
	}

	// Metadata is a snapshot. Mutating one returned value must not alter a
	// later query or the nested fragment retained by the expression node.
	metadata.Columns[0].Name = "changed"
	metadata.Columns[0].Fragment = &SubqueryResultMetadata{Indexed: true}
	refreshed, ok := SubqueryMetadata(scalar)
	if !ok || refreshed.Columns[0].Name != "count" || refreshed.Columns[0].Fragment != nil {
		t.Fatalf("subquery metadata was not defensive = %#v", refreshed)
	}

	if _, ok := SubqueryMetadata(SubqueryValue[float64](prices, price)); ok {
		t.Fatal("single-column subquery unexpectedly exposed fragment metadata")
	}
	if _, err := env.Build(Select(
		From[runtimeTestTrade](env, "SubqueryMetadataTrade"),
		Alias("result", nested),
	).Query(StatementName("subquery-metadata"))); err != nil {
		t.Fatalf("nested metadata query failed to build: %v", err)
	}
}

func TestSubqueryMultiColumnRowsMaterializeScalarAndIndexedFragments(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "SubqueryFragmentTrade"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subqueryMultirowTrigger](env, "SubqueryFragmentTrigger"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SubqueryFragmentTrade")
	if !ok {
		t.Fatal("subquery fragment schema is missing")
	}
	if _, err := CreateNamedWindow(env, "SubqueryFragmentPrices", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	prices := FromNamedWindow(env, "SubqueryFragmentPrices")
	symbol := Field[any, string]("symbol")
	price := Field[any, float64]("price")
	rows := SubqueryRows(prices, Alias("symbol", symbol), Alias("price", price))
	scalar := SubqueryRow(prices, Alias("symbol", symbol), Alias("history", rows))
	query := Select(
		From[subqueryMultirowTrigger](env, "SubqueryFragmentTrigger"),
		Alias("subrow", scalar),
		Alias("rows", rows),
	).Query(StatementName("subquery-fragment-runtime"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	for _, trade := range []runtimeTestTrade{{Symbol: "A", Price: 10}, {Symbol: "B", Price: 20}} {
		if err := engine.InsertNamedWindow(context.Background(), "SubqueryFragmentPrices", trade); err != nil {
			t.Fatal(err)
		}
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var row Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) != 1 {
			return nil
		}
		var ok bool
		row, ok = batch.New[0].Row()
		if !ok {
			t.Fatalf("subquery fragment result is not a row: %#v", batch.New)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subqueryMultirowTrigger{}); err != nil {
		t.Fatal(err)
	}

	subrow, ok := row.GetFragment("subrow")
	if !ok || subrow.Get("symbol").Any() != "A" {
		t.Fatalf("scalar subquery fragment = %#v, found=%v", row.Get("subrow"), ok)
	}
	history, ok := subrow.GetFragments("history")
	if !ok || len(history) != 2 || history[0].Get("symbol").Any() != "A" || history[1].Get("symbol").Any() != "B" {
		t.Fatalf("nested indexed subquery fragments = %#v, found=%v", history, ok)
	}
	indexed, ok := row.GetFragments("rows")
	if !ok || len(indexed) != 2 || indexed[0].Get("price").Any() != 10.0 || indexed[1].Get("price").Any() != 20.0 {
		t.Fatalf("top-level indexed subquery fragments = %#v, found=%v", indexed, ok)
	}
}
