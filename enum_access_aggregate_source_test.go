package esper

import (
	"context"
	"testing"
)

// TestEnumerableWindowAccessAggregateSourceMatchesJava covers the
// ExprEnumDataSources window(*)/window(property) source footprint: the access
// aggregate is materialized as an ordered []T before EnumAllOf evaluates its
// element predicate.
func TestEnumerableWindowAccessAggregateSourceMatchesJava(t *testing.T) {
	env, engine := newRuntimeTest(t)
	windowValues := WindowValues[runtimeTestTrade](EventValue[runtimeTestTrade]())
	allBelowFive := EnumAllOf[runtimeTestTrade](windowValues, Less[float64](
		EnumField[runtimeTestTrade, float64]("price"),
		Literal(5.0),
	))
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").
		Window(LengthWindow(2)).
		Aggregate(Alias("allBelowFive", allBelowFive)).
		Query(StatementName("enum-window-access")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var values []bool
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("window access enum result is not a row: %#v", result)
			}
			value, ok := row.Get("allBelowFive").Any().(bool)
			if !ok {
				t.Fatalf("window access enum value = %#v", row.Get("allBelowFive"))
			}
			values = append(values, value)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, event := range []runtimeTestTrade{
		{Symbol: "E1", Price: 1},
		{Symbol: "E2", Price: 10},
		{Symbol: "E3", Price: 2},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(values) != 3 || values[0] != true || values[1] != false || values[2] != false {
		t.Fatalf("window access enum trace = %#v", values)
	}
}
