package esper

import (
	"context"
	"testing"
)

// TestResultSetAggregateLastParity mirrors the resultset-aggregate-last
// parity scenario (Java ResultSetNoJoinLast): grouped length-window sum
// with output last every 2 events.
func TestResultSetAggregateLastParity(t *testing.T) {
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
		WithOutput(OutputLastEveryEvents(2)),
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
	send("DELL", 10000, 51)
	send("DELL", 20000, 52)
	send("DELL", 30000, 70)
	send("IBM", 10000, 20)

	if len(batches) != 2 {
		t.Fatalf("batches = %d, want 2", len(batches))
	}
	want := [][][3]any{
		{{"DELL", int64(20000), 103.0}},
		{{"DELL", int64(30000), 173.0}, {"IBM", int64(10000), 20.0}},
	}
	for index, batch := range batches {
		if len(batch.New) != len(want[index]) || len(batch.Old) != 0 {
			t.Fatalf("batch %d new/old = %d/%d, want %d/0", index, len(batch.New), len(batch.Old), len(want[index]))
		}
		for rowIndex, rowWant := range want[index] {
			row, ok := batch.New[rowIndex].Row()
			if !ok || row.Get("symbol").Any() != rowWant[0] || row.Get("volume").Any() != rowWant[1] || row.Get("mySum").Any() != rowWant[2] {
				t.Fatalf("batch %d row %d = %#v, want %#v", index, rowIndex, row, rowWant)
			}
		}
	}
}
