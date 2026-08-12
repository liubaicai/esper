package esper

import (
	"context"
	"math"
	"testing"
)

func TestFilteredAggregateAllFunctionsJavaTrace(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	onlyA := StartsWith(symbol, Literal("A"))
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(3)).Aggregate(
		Alias("count", CountIf(onlyA)),
		Alias("avedev", FilterAggregate[float64](Avedev[float64](price), onlyA)),
		Alias("avg", FilterAggregate[float64](Avg[float64](price), onlyA)),
		Alias("max", FilterAggregate[float64](Max[float64](price), onlyA)),
		Alias("median", FilterAggregate[float64](Median[float64](price), onlyA)),
		Alias("min", FilterAggregate[float64](Min[float64](price), onlyA)),
		Alias("stddev", FilterAggregate[float64](StdDev[float64](price), onlyA)),
		Alias("sum", FilterAggregate[float64](Sum[float64](price), onlyA)),
		Alias("maxEver", FilterAggregate[float64](MaxByEver[float64, float64](price, price), onlyA)),
		Alias("minEver", FilterAggregate[float64](MinByEver[float64, float64](price, price), onlyA)),
	).Query(StatementName("filtered-all-functions-java-trace")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 6)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("filtered all-functions result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, event := range []runtimeTestTrade{
		{Symbol: "B", Price: 100},
		{Symbol: "A", Price: 10},
		{Symbol: "B", Price: 11},
		{Symbol: "A", Price: 20},
		{Symbol: "A", Price: 30},
		{Symbol: "A", Price: 40},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 6 {
		t.Fatalf("filtered all-functions rows = %d", len(rows))
	}
	if rows[0].Get("count").Any() != int64(0) || !rows[0].Get("avg").IsNull() || !rows[0].Get("stddev").IsNull() {
		t.Fatalf("empty filtered aggregate row = %#v", rows[0].AsMap())
	}
	if rows[1].Get("count").Any() != int64(1) || rows[1].Get("avg").Any() != float64(10) || !rows[1].Get("stddev").IsNull() {
		t.Fatalf("singleton filtered aggregate row = %#v", rows[1].AsMap())
	}
	if rows[3].Get("count").Any() != int64(2) || rows[3].Get("avedev").Any() != float64(5) || rows[3].Get("avg").Any() != float64(15) || rows[3].Get("max").Any() != float64(20) || rows[3].Get("median").Any() != float64(15) || rows[3].Get("min").Any() != float64(10) || rows[3].Get("sum").Any() != float64(30) {
		t.Fatalf("two-value filtered aggregate row = %#v", rows[3].AsMap())
	}
	if got := rows[3].Get("stddev").Any().(float64); math.Abs(got-math.Sqrt(50)) > 1e-12 || rows[3].Get("maxEver").Any() != float64(20) || rows[3].Get("minEver").Any() != float64(10) {
		t.Fatalf("two-value filtered aggregate statistics = %#v", rows[3].AsMap())
	}
	if rows[4].Get("count").Any() != int64(2) || rows[4].Get("max").Any() != float64(30) || rows[4].Get("min").Any() != float64(20) || rows[4].Get("maxEver").Any() != float64(30) || rows[4].Get("minEver").Any() != float64(10) {
		t.Fatalf("windowed filtered aggregate row = %#v", rows[4].AsMap())
	}
	if rows[5].Get("count").Any() != int64(3) || rows[5].Get("min").Any() != float64(20) || rows[5].Get("max").Any() != float64(40) || rows[5].Get("minEver").Any() != float64(10) {
		t.Fatalf("filtered ever aggregate after eviction = %#v", rows[5].AsMap())
	}
}

func TestFilteredAggregateDistinctAndNullPredicateJavaTrace(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	onlyA := StartsWith(symbol, Literal("A"))
	nullForN := IfThenElse[bool](Equal[string](symbol, Literal("N")), NullLiteral[bool](), onlyA)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(3)).Aggregate(
		Alias("distinct", FilterAggregate[int64](CountDistinct[float64](price), onlyA)),
		Alias("nullExcluded", CountIf(nullForN)),
	).Query(StatementName("filtered-distinct-null-java-trace")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 6)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("filtered distinct result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "A", Price: 100},
		{Symbol: "A", Price: 100},
		{Symbol: "N", Price: 200},
		{Symbol: "A", Price: 200},
		{Symbol: "A", Price: 200},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 5 {
		t.Fatalf("filtered distinct rows = %d", len(rows))
	}
	expectedDistinct := []int64{1, 1, 1, 2, 1}
	expectedNullExcluded := []int64{1, 2, 2, 2, 2}
	for index, expected := range expectedDistinct {
		if rows[index].Get("distinct").Any() != expected || rows[index].Get("nullExcluded").Any() != expectedNullExcluded[index] {
			t.Fatalf("filtered distinct row %d = %#v", index, rows[index].AsMap())
		}
	}
}
