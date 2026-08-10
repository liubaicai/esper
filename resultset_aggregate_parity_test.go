package esper

import (
	"context"
	"testing"
)

type marketDataBean struct {
	Symbol string `esper:"symbol"`
	Volume *int64 `esper:"volume"`
}

func newMarketDataEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[marketDataBean](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	return env, engine
}

func sendMarketData(t *testing.T, engine *Engine, symbol string, volume *int64) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), marketDataBean{Symbol: symbol, Volume: volume}); err != nil {
		t.Fatal(err)
	}
}

func int64P(v int64) *int64 { return &v }

// TestResultSetAggregateMinMaxGroupByMatchesEsper covers
// ResultSetAggregateMinMax: min/max aggregation with group by and
// length window, including distinct variants and null handling.
func TestResultSetAggregateMinMaxGroupByMatchesEsper(t *testing.T) {
	env, engine := newMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	volume := Field[marketDataBean, *int64]("volume")
	symbol := Field[marketDataBean, string]("symbol")

	source := From[marketDataBean](env, "SupportMarketDataBean").
		Filter(Or(
			Or(Equal[string](symbol, Literal("DELL")), Equal[string](symbol, Literal("IBM"))),
			Equal[string](symbol, Literal("GE")),
		)).
		Window(LengthWindow(3))

	grouped := source.GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("minVol", Min[int64](Cast[*int64, int64](volume))),
		Alias("maxVol", Max[int64](Cast[*int64, int64](volume))),
		Alias("minDistVol", DistinctAggregate[int64](Min[int64](Cast[*int64, int64](volume)), Cast[*int64, int64](volume))),
		Alias("maxDistVol", DistinctAggregate[int64](Max[int64](Cast[*int64, int64](volume)), Cast[*int64, int64](volume))),
	)

	plan, err := env.Build(grouped.Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
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

	// Send DELL/50 -> new: min=50, max=50
	sendMarketData(t, engine, "DELL", int64P(50))
	if len(batches) != 1 {
		t.Fatalf("batches=%d, want 1", len(batches))
	}
	row, _ := batches[0].New[0].Row()
	if got := row.Get("minVol").Any(); got != int64(50) {
		t.Fatalf("DELL/50 minVol=%v, want 50", got)
	}
	if got := row.Get("maxVol").Any(); got != int64(50) {
		t.Fatalf("DELL/50 maxVol=%v, want 50", got)
	}

	// Send DELL/30 -> new: min=30, max=50
	sendMarketData(t, engine, "DELL", int64P(30))
	if len(batches) != 2 {
		t.Fatalf("batches=%d, want 2", len(batches))
	}
	row, _ = batches[1].New[0].Row()
	if got := row.Get("minVol").Any(); got != int64(30) {
		t.Fatalf("DELL/30 minVol=%v, want 30", got)
	}
	if got := row.Get("maxVol").Any(); got != int64(50) {
		t.Fatalf("DELL/30 maxVol=%v, want 50", got)
	}
	// Old row should show previous values
	oldRow, _ := batches[1].Old[0].Row()
	if got := oldRow.Get("minVol").Any(); got != int64(50) {
		t.Fatalf("DELL/30 old minVol=%v, want 50", got)
	}

	// Send DELL/30 (duplicate) -> distinct min/max stay same
	sendMarketData(t, engine, "DELL", int64P(30))
	if len(batches) != 3 {
		t.Fatalf("batches=%d, want 3", len(batches))
	}
	row, _ = batches[2].New[0].Row()
	if got := row.Get("minDistVol").Any(); got != int64(30) {
		t.Fatalf("DELL/30dup minDistVol=%v, want 30", got)
	}
	if got := row.Get("maxDistVol").Any(); got != int64(50) {
		t.Fatalf("DELL/30dup maxDistVol=%v, want 50", got)
	}

	// Send DELL/90 -> window is full, oldest (50) evicted
	sendMarketData(t, engine, "DELL", int64P(90))
	if len(batches) != 4 {
		t.Fatalf("batches=%d, want 4", len(batches))
	}
	row, _ = batches[3].New[0].Row()
	if got := row.Get("minVol").Any(); got != int64(30) {
		t.Fatalf("DELL/90 minVol=%v, want 30", got)
	}
	if got := row.Get("maxVol").Any(); got != int64(90) {
		t.Fatalf("DELL/90 maxVol=%v, want 90", got)
	}

	// Send IBM events - separate group
	sendMarketData(t, engine, "IBM", int64P(20))
	sendMarketData(t, engine, "IBM", int64P(5))
	sendMarketData(t, engine, "IBM", int64P(15))
	sendMarketData(t, engine, "IBM", int64P(18))
	// IBM group: after 4 events through length-3 window: {5,15,18}
	// min=5, max=18
	lastBatch := batches[len(batches)-1]
	row, _ = lastBatch.New[0].Row()
	if got := row.Get("symbol").Any(); got != "IBM" {
		t.Fatalf("last batch symbol=%v, want IBM", got)
	}
	if got := row.Get("minVol").Any(); got != int64(5) {
		t.Fatalf("IBM minVol=%v, want 5", got)
	}
	if got := row.Get("maxVol").Any(); got != int64(18) {
		t.Fatalf("IBM maxVol=%v, want 18", got)
	}

	// Send IBM with null volume - null skipped in min/max
	sendMarketData(t, engine, "IBM", nil)
	lastBatch = batches[len(batches)-1]
	row, _ = lastBatch.New[0].Row()
	if got := row.Get("minVol").Any(); got != int64(15) {
		t.Fatalf("IBM null minVol=%v, want 15 (5 evicted)", got)
	}
}

// TestResultSetAggregateCountSumAvgGroupByMatchesEsper covers basic
// count/sum/avg aggregation with group by.
func TestResultSetAggregateCountSumAvgGroupByMatchesEsper(t *testing.T) {
	env, engine := newMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	volume := Field[marketDataBean, *int64]("volume")
	symbol := Field[marketDataBean, string]("symbol")

	source := From[marketDataBean](env, "SupportMarketDataBean").Window(LengthWindow(5))
	grouped := source.GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("cnt", CountAll()),
		Alias("sumVol", Sum[int64](Cast[*int64, int64](volume))),
		Alias("avgVol", Avg[int64](Cast[*int64, int64](volume))),
	)

	plan, err := env.Build(grouped.Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
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

	sendMarketData(t, engine, "DELL", int64P(100))
	sendMarketData(t, engine, "DELL", int64P(200))
	sendMarketData(t, engine, "DELL", int64P(300))
	sendMarketData(t, engine, "IBM", int64P(50))

	// After 3 DELL events
	row, _ := batches[2].New[0].Row()
	if got := row.Get("cnt").Any(); got != int64(3) {
		t.Fatalf("DELL cnt=%v, want 3", got)
	}
	if got := row.Get("sumVol").Any(); got != int64(600) {
		t.Fatalf("DELL sumVol=%v, want 600", got)
	}
	if got := row.Get("avgVol").Any(); got != float64(200.0) {
		t.Fatalf("DELL avgVol=%v, want 200", got)
	}

	// IBM group
	row, _ = batches[3].New[0].Row()
	if got := row.Get("cnt").Any(); got != int64(1) {
		t.Fatalf("IBM cnt=%v, want 1", got)
	}
	if got := row.Get("sumVol").Any(); got != int64(50) {
		t.Fatalf("IBM sumVol=%v, want 50", got)
	}
}

// TestResultSetAggregateHavingMatchesEsper covers Having clause filtering
// on aggregate results.
func TestResultSetAggregateHavingMatchesEsper(t *testing.T) {
	env, engine := newMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	volume := Field[marketDataBean, *int64]("volume")
	symbol := Field[marketDataBean, string]("symbol")

	source := From[marketDataBean](env, "SupportMarketDataBean").Window(LengthWindow(5))
	grouped := source.GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("cnt", CountAll()),
		Alias("sumVol", Sum[int64](Cast[*int64, int64](volume))),
	).Having(Greater[int64](Sum[int64](Cast[*int64, int64](volume)), Literal(int64(150))))

	plan, err := env.Build(grouped.Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
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

	// Send DELL/100 -> sum=100, not > 150, no output
	sendMarketData(t, engine, "DELL", int64P(100))
	if len(batches) != 0 {
		t.Fatalf("DELL/100 should not pass having, got %d batches", len(batches))
	}

	// Send DELL/200 -> sum=300, > 150, should output
	sendMarketData(t, engine, "DELL", int64P(200))
	if len(batches) != 1 {
		t.Fatalf("DELL sum=300 should pass having, got %d batches", len(batches))
	}
	row, _ := batches[0].New[0].Row()
	if got := row.Get("cnt").Any(); got != int64(2) {
		t.Fatalf("DELL cnt=%v, want 2", got)
	}
}
