package esper

import (
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
