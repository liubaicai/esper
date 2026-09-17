package esper

import (
	"context"
	"testing"
)

// TestFAFAggregateHavingOverNonEmptySource pins the Java FAF contract: a
// fire-and-forget aggregate over a non-empty source whose having rejects the
// current state returns zero rows - the forced empty-group fallback must only
// fire when no group was touched (empty or where-filtered-to-empty source).
func TestFAFAggregateHavingOverNonEmptySource(t *testing.T) {
	env, _ := newRuntimeTest(t)
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "faf-having-window", schema); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, event := range []runtimeTestTrade{
		{Symbol: "A", Price: 10}, {Symbol: "B", Price: 20}, {Symbol: "C", Price: 30},
		{Symbol: "D", Price: 40}, {Symbol: "E", Price: 50},
	} {
		if err := engine.InsertNamedWindow(context.Background(), "faf-having-window", event); err != nil {
			t.Fatal(err)
		}
	}
	// having count(*)=0 over 5 rows: Java returns 0 rows (was 1 spurious {c0:0})
	reject, err := env.Build(FromNamedWindow(env, "faf-having-window").Aggregate(
		Alias("c0", CountAll()),
	).Having(Equal[int64](CountAll(), Literal(int64(0)))).Query())
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.ExecuteFireAndForget(context.Background(), reject)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 0 {
		t.Fatalf("having count(*)=0 over 5 rows returned %#v, want 0 rows", result.Results())
	}
	// having count(*)=5: 1 row
	accept, err := env.Build(FromNamedWindow(env, "faf-having-window").Aggregate(
		Alias("c0", CountAll()),
	).Having(Equal[int64](CountAll(), Literal(int64(5)))).Query())
	if err != nil {
		t.Fatal(err)
	}
	result, err = engine.ExecuteFireAndForget(context.Background(), accept)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 1 {
		t.Fatalf("having count(*)=5 returned %d rows, want 1", len(result.Results()))
	}
	// empty window + having count(*)=0: 1 row {c0:0}
	if _, err := CreateNamedWindow(env, "faf-having-empty", schema); err != nil {
		t.Fatal(err)
	}
	empty, err := env.Build(FromNamedWindow(env, "faf-having-empty").Aggregate(
		Alias("c0", CountAll()),
	).Having(Equal[int64](CountAll(), Literal(int64(0)))).Query())
	if err != nil {
		t.Fatal(err)
	}
	result, err = engine.ExecuteFireAndForget(context.Background(), empty)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 1 {
		t.Fatalf("empty window having count(*)=0 returned %d rows, want 1", len(result.Results()))
	}
	row, _ := result.Results()[0].Row()
	if row.Get("c0").Any() != int64(0) {
		t.Fatalf("empty window c0 = %#v", row.Get("c0").Any())
	}
}
