package esper

import (
	"context"
	"testing"
	"time"
)

// TestRollupOutputLastMarketParity mirrors the rollup-output-last-market
// parity scenario (Java ResultSet4OutputLimitLast): a 5.5s SupportMarketDataBean
// window with rollup(symbol) and output last every second.
func TestRollupOutputLastMarketParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	symbol := Field[resultsetGroupedTimeWindowMarket, string]("symbol")
	price := Field[resultsetGroupedTimeWindowMarket, float64]("price")
	plan, err := env.Build(From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
		Window(TimeWindow(5500*time.Millisecond)).
		GroupByRollup(symbol).
		Select(
			Alias("symbol", symbol),
			Alias("sum(price)", Sum[float64](price)),
		).Query(
		StatementName("s0"),
		WithOldStream(),
		WithOutput(OutputLastEveryTime(time.Second)),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
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
	advance := func(ms int64) {
		t.Helper()
		if err := engine.AdvanceTime(ctx, time.UnixMilli(ms).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	send := func(symbol string, volume int64, price float64) {
		t.Helper()
		if err := engine.SendEvent(ctx, resultsetGroupedTimeWindowMarket{Symbol: symbol, Volume: volume, Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	advance(200)
	send("IBM", 100, 25)
	advance(800)
	send("MSFT", 5000, 9)
	advance(1000)
	advance(1200)
	advance(1500)
	send("IBM", 150, 24)
	send("YAH", 10000, 1)
	advance(2000)
	advance(2100)
	send("IBM", 155, 26)
	advance(2200)
	advance(2500)
	advance(3000)
	advance(3200)
	advance(3500)
	send("YAH", 11000, 2)
	advance(4000)
	advance(4200)
	advance(4300)
	send("IBM", 150, 22)
	advance(4900)
	send("YAH", 11500, 3)
	advance(5000)
	advance(5200)
	advance(5700)
	advance(5900)
	send("YAH", 10500, 1)
	advance(6000)
	advance(6200)
	advance(6300)
	advance(7000)
	advance(7200)

	if len(batches) != 6 {
		t.Fatalf("batches = %d, want 6", len(batches))
	}
	want := [][][][2]any{
		{
			{{"IBM", 25.0}, {"MSFT", 9.0}, {nil, 34.0}},
			{{"IBM", nil}, {"MSFT", nil}, {nil, nil}},
		},
		{
			{{"IBM", 75.0}, {"YAH", 1.0}, {nil, 85.0}},
			{{"IBM", 25.0}, {"YAH", nil}, {nil, 34.0}},
		},
		{
			{{"YAH", 3.0}, {nil, 87.0}},
			{{"YAH", 1.0}, {nil, 85.0}},
		},
		{
			{{"IBM", 97.0}, {"YAH", 6.0}, {nil, 112.0}},
			{{"IBM", 75.0}, {"YAH", 3.0}, {nil, 87.0}},
		},
		{
			{{"IBM", 72.0}, {"YAH", 7.0}, {nil, 88.0}},
			{{"IBM", 97.0}, {"YAH", 6.0}, {nil, 112.0}},
		},
		{
			{{"MSFT", nil}, {"IBM", 48.0}, {"YAH", 6.0}, {nil, 54.0}},
			{{"MSFT", 9.0}, {"IBM", 72.0}, {"YAH", 7.0}, {nil, 88.0}},
		},
	}
	check := func(side []Result, expected [][2]any) {
		t.Helper()
		if len(side) != len(expected) {
			t.Fatalf("side length = %d, want %d", len(side), len(expected))
		}
		for index, rowWant := range expected {
			row, ok := side[index].Row()
			if !ok {
				t.Fatalf("row %d is not a row", index)
			}
			gotSymbol := row.Get("symbol")
			gotSum := row.Get("sum(price)")
			if (rowWant[0] == nil && !gotSymbol.IsNull()) || (rowWant[0] != nil && (gotSymbol.IsNull() || gotSymbol.Any() != rowWant[0])) {
				t.Fatalf("row %d symbol = %#v, want %#v", index, gotSymbol, rowWant[0])
			}
			if (rowWant[1] == nil && !gotSum.IsNull()) || (rowWant[1] != nil && (gotSum.IsNull() || gotSum.Any() != rowWant[1])) {
				t.Fatalf("row %d sum = %#v, want %#v", index, gotSum, rowWant[1])
			}
		}
	}
	for index, batch := range batches {
		check(batch.New, want[index][0])
		check(batch.Old, want[index][1])
	}
}
