package esper

import (
	"context"
	"testing"
)

// TestResultSetAggregateNoOutputParity mirrors the resultset-aggregate-no-output
// parity scenario (Java ResultSetNoOutputClauseView): grouped length-window sum
// with no output policy.
func TestResultSetAggregateNoOutputParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	symbol := Field[resultsetGroupedTimeWindowMarket, string]("symbol")
	volume := Field[resultsetGroupedTimeWindowMarket, int64]("volume")
	price := Field[resultsetGroupedTimeWindowMarket, float64]("price")
	plan, err := env.Build(From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
		Window(LengthWindow(5)).
		Filter(Or(
			Or(
				Equal[string](symbol, Literal("DELL")),
				Equal[string](symbol, Literal("IBM")),
			),
			Equal[string](symbol, Literal("GE")),
		)).
		GroupBy(symbol).
		Select(
			Alias("symbol", symbol),
			Alias("volume", volume),
			Alias("mySum", Sum[float64](price)),
		).Query(
		StatementName("s0"),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	send := func(symbol string, volume int64, price float64) {
		t.Helper()
		if err := engine.SendEvent(ctx, resultsetGroupedTimeWindowMarket{Symbol: symbol, Volume: volume, Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	send("DELL", 10, 100)
	send("IBM", 15, 50)

	if len(batches) != 2 {
		t.Fatalf("batches = %d, want 2", len(batches))
	}
	want := [][3]any{
		{"DELL", int64(10), 100.0},
		{"IBM", int64(15), 50.0},
	}
	for index, batch := range batches {
		if len(batch.New) != 1 || len(batch.Old) != 0 {
			t.Fatalf("batch %d new/old = %d/%d, want 1/0", index, len(batch.New), len(batch.Old))
		}
		row, ok := batch.New[0].Row()
		if !ok || row.Get("symbol").Any() != want[index][0] || row.Get("volume").Any() != want[index][1] || row.Get("mySum").Any() != want[index][2] {
			t.Fatalf("batch %d row = %#v, want %#v", index, row, want[index])
		}
	}
}
