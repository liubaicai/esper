package esper

import (
	"context"
	"testing"
	"time"
)

type resultsetGroupedTimeWindowMarket struct {
	Symbol string  `esper:"symbol"`
	Volume int64   `esper:"volume"`
	Price  float64 `esper:"price"`
}

// TestResultSetGroupedTimeWindowIStreamParity mirrors the shared
// resultset-grouped-time-window scenario (Java ResultSet1NoneNoHavingNoJoin):
// with the default istream selector, pure time-expiry batches emit no
// listener rows even though the window state changes.
func TestResultSetGroupedTimeWindowIStreamParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	symbol := Field[resultsetGroupedTimeWindowMarket, string]("symbol")
	volume := Field[resultsetGroupedTimeWindowMarket, int64]("volume")
	price := Field[resultsetGroupedTimeWindowMarket, float64]("price")
	plan, err := env.Build(From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
		Window(TimeWindow(5500*time.Millisecond)).
		GroupBy(symbol).
		Select(
			Alias("symbol", symbol),
			Alias("volume", volume),
			Alias("sum(price)", Sum[float64](price)),
		).
		Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
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
	send := func(symbol string, volume int64, price float64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), resultsetGroupedTimeWindowMarket{Symbol: symbol, Volume: volume, Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	advance := func(second int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.Unix(second, 0).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	advance(0)
	send("IBM", 100, 25)
	advance(1)
	send("MSFT", 5000, 9)
	advance(2)
	send("IBM", 150, 24)
	send("YAH", 10000, 1)
	advance(3)
	send("IBM", 155, 26)
	advance(4)
	send("YAH", 11000, 2)
	advance(5)
	send("IBM", 150, 22)
	send("YAH", 11500, 3)
	advance(6)
	send("YAH", 10500, 1)
	advance(7)
	advance(8)
	if len(batches) != 9 {
		t.Fatalf("istream batches = %d, want 9 (pure expiry must not emit)", len(batches))
	}
	for _, batch := range batches {
		if len(batch.Old) != 0 {
			t.Fatalf("istream batch unexpectedly has old rows: %#v", batch)
		}
	}
	row0, ok0 := batches[0].New[0].Row()
	if len(batches[0].New) != 1 || !ok0 || row0.Get("sum(price)").Any() != float64(25) {
		t.Fatalf("first batch = %#v", batches[0])
	}
	row2, ok2 := batches[2].New[0].Row()
	if len(batches[2].New) != 1 || !ok2 || row2.Get("sum(price)").Any() != float64(49) {
		t.Fatalf("third batch = %#v", batches[2])
	}
	row8, ok8 := batches[8].New[0].Row()
	if len(batches[8].New) != 1 || !ok8 || row8.Get("sum(price)").Any() != float64(7) {
		t.Fatalf("last batch = %#v", batches[8])
	}
}
