package esper

import (
	"context"
	"testing"
	"time"
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
	volumeLong := Cast[*int64, int64](volume)

	source := From[marketDataBean](env, "SupportMarketDataBean").
		Filter(Or(
			Or(Equal[string](symbol, Literal("DELL")), Equal[string](symbol, Literal("IBM"))),
			Equal[string](symbol, Literal("GE")),
		)).
		Window(LengthWindow(3))

	grouped := source.GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("minVol", Min[int64](volumeLong)),
		Alias("maxVol", Max[int64](volumeLong)),
		Alias("minDistVol", DistinctAggregate[int64](Min[int64](volumeLong), volumeLong)),
		Alias("maxDistVol", DistinctAggregate[int64](Max[int64](volumeLong), volumeLong)),
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

	assertRow := func(label string, row Row, wantSymbol string, minVol, maxVol, minDistVol, maxDistVol *int64) {
		t.Helper()
		if got := row.Get("symbol").Any(); got != wantSymbol {
			t.Fatalf("%s symbol=%v, want %q", label, got, wantSymbol)
		}
		assertLong := func(field string, want *int64) {
			t.Helper()
			value := row.Get(field)
			if want == nil {
				if !value.IsNull() {
					t.Fatalf("%s %s=%v, want null", label, field, value.Any())
				}
				return
			}
			got, ok := value.Any().(int64)
			if !ok || got != *want {
				t.Fatalf("%s %s=%v, want %d", label, field, value.Any(), *want)
			}
		}
		assertLong("minVol", minVol)
		assertLong("maxVol", maxVol)
		assertLong("minDistVol", minDistVol)
		assertLong("maxDistVol", maxDistVol)
	}
	assertBatch := func(label string, batch ResultBatch, oldSymbol string, oldMin, oldMax, oldMinDist, oldMaxDist *int64, newSymbol string, newMin, newMax, newMinDist, newMaxDist *int64) {
		t.Helper()
		if len(batch.Old) != 1 || len(batch.New) != 1 {
			t.Fatalf("%s old/new lengths = %d/%d, want 1/1", label, len(batch.Old), len(batch.New))
		}
		oldRow, ok := batch.Old[0].Row()
		if !ok {
			t.Fatalf("%s old result is not a row: %#v", label, batch.Old[0])
		}
		newRow, ok := batch.New[0].Row()
		if !ok {
			t.Fatalf("%s new result is not a row: %#v", label, batch.New[0])
		}
		assertRow(label+" old", oldRow, oldSymbol, oldMin, oldMax, oldMinDist, oldMaxDist)
		assertRow(label+" new", newRow, newSymbol, newMin, newMax, newMinDist, newMaxDist)
	}
	type expectedAggregateRow struct {
		symbol                                 string
		minVol, maxVol, minDistVol, maxDistVol *int64
	}
	assertGroupedBatch := func(label string, batch ResultBatch, oldRows, newRows []expectedAggregateRow) {
		t.Helper()
		if len(batch.Old) != len(oldRows) || len(batch.New) != len(newRows) {
			t.Fatalf("%s old/new lengths = %d/%d, want %d/%d", label, len(batch.Old), len(batch.New), len(oldRows), len(newRows))
		}
		assertResults := func(results []Result, wants []expectedAggregateRow) {
			for index, want := range wants {
				row, ok := results[index].Row()
				if !ok {
					t.Fatalf("%s row %d is not a row: %#v", label, index, results[index])
				}
				assertRow(label, row, want.symbol, want.minVol, want.maxVol, want.minDistVol, want.maxDistVol)
			}
		}
		assertResults(batch.Old, oldRows)
		assertResults(batch.New, newRows)
	}
	sendGroupedAndAssert := func(label, eventSymbol string, eventVolume *int64, oldRows, newRows []expectedAggregateRow) {
		t.Helper()
		before := len(batches)
		sendMarketData(t, engine, eventSymbol, eventVolume)
		if len(batches) != before+1 {
			t.Fatalf("%s batches=%d, want %d", label, len(batches), before+1)
		}
		assertGroupedBatch(label, batches[len(batches)-1], oldRows, newRows)
	}

	sendAndAssert := func(label, eventSymbol string, eventVolume *int64, oldMin, oldMax, oldMinDist, oldMaxDist *int64, newMin, newMax, newMinDist, newMaxDist *int64) {
		t.Helper()
		before := len(batches)
		sendMarketData(t, engine, eventSymbol, eventVolume)
		if len(batches) != before+1 {
			t.Fatalf("%s batches=%d, want %d", label, len(batches), before+1)
		}
		assertBatch(label, batches[len(batches)-1], eventSymbol, oldMin, oldMax, oldMinDist, oldMaxDist, eventSymbol, newMin, newMax, newMinDist, newMaxDist)
	}

	// The first grouped update pairs the new row with an all-null old row.
	sendAndAssert("DELL/50", "DELL", int64P(50), nil, nil, nil, nil, int64P(50), int64P(50), int64P(50), int64P(50))
	sendAndAssert("DELL/30", "DELL", int64P(30), int64P(50), int64P(50), int64P(50), int64P(50), int64P(30), int64P(50), int64P(30), int64P(50))
	sendAndAssert("DELL/30 duplicate", "DELL", int64P(30), int64P(30), int64P(50), int64P(30), int64P(50), int64P(30), int64P(50), int64P(30), int64P(50))
	sendAndAssert("DELL/90", "DELL", int64P(90), int64P(30), int64P(50), int64P(30), int64P(50), int64P(30), int64P(90), int64P(30), int64P(90))
	// The length window is global: DELL/100 evicts the oldest retained DELL row.
	sendAndAssert("DELL/100", "DELL", int64P(100), int64P(30), int64P(90), int64P(30), int64P(90), int64P(30), int64P(100), int64P(30), int64P(100))

	// These IBM events evict the remaining DELL rows from the same global
	// length(3) window. Each callback carries both changed group rows in
	// IBM-then-DELL order.
	sendGroupedAndAssert("IBM/20", "IBM", int64P(20),
		[]expectedAggregateRow{
			{symbol: "IBM"},
			{symbol: "DELL", minVol: int64P(30), maxVol: int64P(100), minDistVol: int64P(30), maxDistVol: int64P(100)},
		},
		[]expectedAggregateRow{
			{symbol: "IBM", minVol: int64P(20), maxVol: int64P(20), minDistVol: int64P(20), maxDistVol: int64P(20)},
			{symbol: "DELL", minVol: int64P(90), maxVol: int64P(100), minDistVol: int64P(90), maxDistVol: int64P(100)},
		})
	sendGroupedAndAssert("IBM/5", "IBM", int64P(5),
		[]expectedAggregateRow{
			{symbol: "IBM", minVol: int64P(20), maxVol: int64P(20), minDistVol: int64P(20), maxDistVol: int64P(20)},
			{symbol: "DELL", minVol: int64P(90), maxVol: int64P(100), minDistVol: int64P(90), maxDistVol: int64P(100)},
		},
		[]expectedAggregateRow{
			{symbol: "IBM", minVol: int64P(5), maxVol: int64P(20), minDistVol: int64P(5), maxDistVol: int64P(20)},
			{symbol: "DELL", minVol: int64P(100), maxVol: int64P(100), minDistVol: int64P(100), maxDistVol: int64P(100)},
		})
	sendGroupedAndAssert("IBM/15", "IBM", int64P(15),
		[]expectedAggregateRow{
			{symbol: "IBM", minVol: int64P(5), maxVol: int64P(20), minDistVol: int64P(5), maxDistVol: int64P(20)},
			{symbol: "DELL", minVol: int64P(100), maxVol: int64P(100), minDistVol: int64P(100), maxDistVol: int64P(100)},
		},
		[]expectedAggregateRow{
			{symbol: "IBM", minVol: int64P(5), maxVol: int64P(20), minDistVol: int64P(5), maxDistVol: int64P(20)},
			{symbol: "DELL"},
		})
	sendAndAssert("IBM/18", "IBM", int64P(18), int64P(5), int64P(20), int64P(5), int64P(20), int64P(5), int64P(18), int64P(5), int64P(18))

	// Null volumes are skipped by both ordinary and distinct min/max. Each
	// null still enters the length window and therefore evicts the oldest row.
	sendAndAssert("IBM/null #1", "IBM", nil, int64P(5), int64P(18), int64P(5), int64P(18), int64P(15), int64P(18), int64P(15), int64P(18))
	sendAndAssert("IBM/null #2", "IBM", nil, int64P(15), int64P(18), int64P(15), int64P(18), int64P(18), int64P(18), int64P(18), int64P(18))
	sendAndAssert("IBM/null #3", "IBM", nil, int64P(18), int64P(18), int64P(18), int64P(18), nil, nil, nil, nil)
}

func TestResultSetAggregateMinNoGroupHavingMatchesEsper(t *testing.T) {
	env, engine := newMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	volume := Field[marketDataBean, *int64]("volume")
	volumeFloat := Cast[*int64, float64](volume)
	symbol := Field[marketDataBean, string]("symbol")
	minimum := Min[float64](volumeFloat)
	query := From[marketDataBean](env, "SupportMarketDataBean").
		Window(TimeWindow(5 * time.Second)).
		Aggregate(Alias("symbol", symbol)).
		Having(Greater[float64](volumeFloat, Multiply[float64](minimum, Literal(1.3)))).
		Query(StatementName("s0"))

	plan, err := env.Build(query)
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

	assertSilent := func(label string, eventVolume *int64) {
		t.Helper()
		before := len(batches)
		sendMarketData(t, engine, "DELL", eventVolume)
		if len(batches) != before {
			t.Fatalf("%s unexpectedly emitted %d new batches", label, len(batches)-before)
		}
	}
	assertNew := func(label string, eventVolume *int64) {
		t.Helper()
		before := len(batches)
		sendMarketData(t, engine, "DELL", eventVolume)
		if len(batches) != before+1 {
			t.Fatalf("%s batches=%d, want %d", label, len(batches), before+1)
		}
		batch := batches[len(batches)-1]
		if len(batch.Old) != 0 || len(batch.New) != 1 {
			t.Fatalf("%s old/new lengths = %d/%d, want 0/1", label, len(batch.Old), len(batch.New))
		}
		row, ok := batch.New[0].Row()
		if !ok || row.Get("symbol").Any() != "DELL" {
			t.Fatalf("%s new row = %#v, want symbol DELL", label, batch.New)
		}
	}

	// The aggregate minimum starts at 100, so 100, 105 and 100 all fail
	// volume > min(volume) * 1.3 (130). The raw event remains the left operand
	// while min(volume) is evaluated over the complete ungrouped time window.
	assertSilent("DELL/100", int64P(100))
	assertSilent("DELL/105", int64P(105))
	assertSilent("DELL/100 duplicate", int64P(100))
	assertNew("DELL/131", int64P(131))
	assertNew("DELL/132", int64P(132))
	assertSilent("DELL/129", int64P(129))
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
