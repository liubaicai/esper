package esper

import (
	"context"
	"testing"
)

type viewParityBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	LongPrimitive int64 `esper:"longPrimitive"`
}

func newViewParityEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[viewParityBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	return env, engine
}

func sendViewBean(t *testing.T, engine *Engine, theString string, intPrim int) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), viewParityBean{TheString: theString, IntPrimitive: intPrim}); err != nil {
		t.Fatal(err)
	}
}

func deployViewParity(t *testing.T, env *Environment, engine *Engine, stream Stream[viewParityBean], name string) (*Statement, *[]ResultBatch) {
	t.Helper()
	plan, err := env.Build(stream.Query(StatementName(name), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := new([]ResultBatch)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return deployment.Statements()[0], batches
}

// TestViewKeepAllSimpleMatchesEsper covers ViewKeepAllSimple:
// keep-all window retains all events; each new event is reported as insert
// with no removes. Iterator sees all retained events.
func TestViewKeepAllSimpleMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean")
	_, batches := deployViewParity(t, env, engine, source.Window(KeepAll()), "s0")

	// Send events one at a time - each should produce a batch with 1 new row
	events := []string{"E1", "E2", "E3", "E4"}
	for i, e := range events {
		sendViewBean(t, engine, e, 0)
		if len(*batches) != i+1 {
			t.Fatalf("after %s: got %d batches, want %d", e, len(*batches), i+1)
		}
		if len((*batches)[i].New) != 1 {
			t.Fatalf("after %s: new rows = %d, want 1", e, len((*batches)[i].New))
		}
		// KeepAll: no removes
		if len((*batches)[i].Old) != 0 {
			t.Fatalf("after %s: old rows = %d, want 0 (keep-all)", e, len((*batches)[i].Old))
		}
	}
}

// TestViewLengthWindowMatchesEsper covers ViewLengthWindowSceneOne:
// length window retains only the last N events; oldest event is removed
// when a new event arrives and the window is full.
func TestViewLengthWindowMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean")
	_, batches := deployViewParity(t, env, engine, source.Window(LengthWindow(3)), "s0")

	// Send 5 events through a length-3 window
	events := []string{"E1", "E2", "E3", "E4", "E5"}
	for i, e := range events {
		sendViewBean(t, engine, e, 0)
		batch := (*batches)[i]
		if len(batch.New) != 1 {
			t.Fatalf("%s: new rows = %d, want 1", e, len(batch.New))
		}
		if i < 3 {
			// Window not yet full, no removes
			if len(batch.Old) != 0 {
				t.Fatalf("%s: old rows = %d, want 0 (window not full)", e, len(batch.Old))
			}
		} else {
			// Window full, oldest removed
			if len(batch.Old) != 1 {
				t.Fatalf("%s: old rows = %d, want 1 (window full)", e, len(batch.Old))
			}
		}
	}
}

// TestViewFirstEventMatchesEsper covers ViewFirstEventSceneOne:
// first-event window keeps only the first event; subsequent events
// do not produce new insert rows (they go to old stream as removes).
func TestViewFirstEventMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean")
	_, batches := deployViewParity(t, env, engine, source.Window(FirstEvent()), "s0")

	// First event should match
	sendViewBean(t, engine, "E1", 0)
	if len(*batches) != 1 {
		t.Fatalf("after E1: got %d batches, want 1", len(*batches))
	}
	if len((*batches)[0].New) != 1 {
		t.Fatalf("E1: new rows = %d, want 1", len((*batches)[0].New))
	}

	// Subsequent events should NOT produce new matches (first-event window)
	sendViewBean(t, engine, "E2", 0)
	if len(*batches) != 1 {
		t.Fatalf("after E2: got %d batches, want 1 (first-event only)", len(*batches))
	}
}

// TestViewLastEventMatchesEsper covers ViewLastEventSceneOne:
// last-event window keeps only the most recent event; each new event
// replaces the previous one (old stream shows the removed event).
func TestViewLastEventMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean")
	_, batches := deployViewParity(t, env, engine, source.Window(LastEvent()), "s0")

	// Each event should produce a batch
	events := []string{"E1", "E2", "E3"}
	for i, e := range events {
		sendViewBean(t, engine, e, 0)
		if len(*batches) != i+1 {
			t.Fatalf("after %s: got %d batches, want %d", e, len(*batches), i+1)
		}
		if len((*batches)[i].New) != 1 {
			t.Fatalf("%s: new rows = %d, want 1", e, len((*batches)[i].New))
		}
		if i > 0 {
			// Previous event removed
			if len((*batches)[i].Old) != 1 {
				t.Fatalf("%s: old rows = %d, want 1 (previous removed)", e, len((*batches)[i].Old))
			}
		}
	}
}

// TestViewFirstLengthMatchesEsper covers ViewFirstLengthSceneOne:
// first-length window keeps only the first N events; subsequent events
// do not produce insert rows.
func TestViewFirstLengthMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean")
	_, batches := deployViewParity(t, env, engine, source.Window(FirstLength(2)), "s0")

	// First two events should match
	sendViewBean(t, engine, "E1", 0)
	sendViewBean(t, engine, "E2", 0)
	if len(*batches) != 2 {
		t.Fatalf("after E1,E2: got %d batches, want 2", len(*batches))
	}

	// Third event should NOT match (first-length window full)
	sendViewBean(t, engine, "E3", 0)
	if len(*batches) != 2 {
		t.Fatalf("after E3: got %d batches, want 2 (first-length full)", len(*batches))
	}
}

// TestViewKeepAllIteratorMatchesEsper covers ViewKeepAllIterator:
// select symbol, price from SupportMarketDataBean#keepall — the iterator
// returns every retained event in arrival order.
func TestViewKeepAllIteratorMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[viewUniqueMarketData](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(KeepAll())
	plan, err := env.Build(source.Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	stmt := deployment.Statements()[0]

	send := func(symbol string, price float64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), viewUniqueMarketData{Symbol: symbol, Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	assertIterator := func(want ...viewUniqueMarketData) {
		t.Helper()
		result, err := stmt.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Results()) != len(want) {
			t.Fatalf("iterator size = %d, want %d (%#v)", len(result.Results()), len(want), result.Results())
		}
		for i, row := range result.Results() {
			if got := row.Get("symbol").Any(); got != want[i].Symbol {
				t.Fatalf("iterator[%d] symbol = %v, want %s", i, got, want[i].Symbol)
			}
			if got := row.Get("price").Any(); got != want[i].Price {
				t.Fatalf("iterator[%d] price = %v, want %v", i, got, want[i].Price)
			}
		}
	}

	send("ABC", 20)
	send("DEF", 100)
	assertIterator(viewUniqueMarketData{Symbol: "ABC", Price: 20}, viewUniqueMarketData{Symbol: "DEF", Price: 100})

	send("EFG", 50)
	assertIterator(
		viewUniqueMarketData{Symbol: "ABC", Price: 20},
		viewUniqueMarketData{Symbol: "DEF", Price: 100},
		viewUniqueMarketData{Symbol: "EFG", Price: 50},
	)
}

// TestViewKeepAllWindowStatsMatchesEsper covers ViewKeepAllWindowStats:
// select irstream symbol, count(*) as cnt, sum(price) as mysum from
// SupportMarketDataBean#keepall group by symbol — grouped aggregates over a
// keep-all window report the new state in the insert stream and the previous
// state in the remove stream (first event per group reports cnt=0/sum=null).
func TestViewKeepAllWindowStatsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[viewUniqueMarketData](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	symbol := Field[viewUniqueMarketData, string]("symbol")
	price := Field[viewUniqueMarketData, float64]("price")
	grouped := From[viewUniqueMarketData](env, "SupportMarketDataBean").
		Window(KeepAll()).
		GroupBy(symbol).
		Select(
			Alias("symbol", symbol),
			Alias("cnt", CountAll()),
			Alias("mysum", Sum[float64](price)),
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

	send := func(sym string, price float64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), viewUniqueMarketData{Symbol: sym, Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	assertPair := func(index int, sym string, newCnt int64, newSum float64, oldCnt int64, oldSum any) {
		t.Helper()
		if len(batches) != index+1 {
			t.Fatalf("batches = %d, want %d", len(batches), index+1)
		}
		newRow, ok := batches[index].New[0].Row()
		if !ok {
			t.Fatalf("batch %d new row is not a projection row", index)
		}
		if got := newRow.Get("symbol").Any(); got != sym {
			t.Fatalf("batch %d new symbol = %v, want %s", index, got, sym)
		}
		if got := newRow.Get("cnt").Any(); got != newCnt {
			t.Fatalf("batch %d new cnt = %v, want %d", index, got, newCnt)
		}
		if got := newRow.Get("mysum").Any(); got != newSum {
			t.Fatalf("batch %d new mysum = %v, want %v", index, got, newSum)
		}
		oldRow, ok := batches[index].Old[0].Row()
		if !ok {
			t.Fatalf("batch %d old row is not a projection row", index)
		}
		if got := oldRow.Get("symbol").Any(); got != sym {
			t.Fatalf("batch %d old symbol = %v, want %s", index, got, sym)
		}
		if got := oldRow.Get("cnt").Any(); got != oldCnt {
			t.Fatalf("batch %d old cnt = %v, want %d", index, got, oldCnt)
		}
		if got := oldRow.Get("mysum").Any(); got != oldSum {
			t.Fatalf("batch %d old mysum = %v, want %v", index, got, oldSum)
		}
	}

	send("S1", 100)
	assertPair(0, "S1", 1, 100.0, 0, nil)

	send("S2", 50)
	assertPair(1, "S2", 1, 50.0, 0, nil)

	send("S1", 5)
	assertPair(2, "S1", 2, 105.0, 1, 100.0)

	send("S2", -1)
	assertPair(3, "S2", 2, 49.0, 1, 50.0)
}

// TestEPLOtherIRStreamSelectorMatchesEsper covers
// EPLOtherIStreamRStreamConfigSelectorIRStream: default irstream behavior
// with length window shows both new (inserted) and old (evicted) events.
func TestEPLOtherIRStreamSelectorMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean")
	_, batches := deployViewParity(t, env, engine, source.Window(LengthWindow(3)), "s0")

	// Fill the window (3 events)
	sendViewBean(t, engine, "a", 0)
	sendViewBean(t, engine, "b", 0)
	sendViewBean(t, engine, "c", 0)

	// Reset: send 4th event - 'a' evicted
	sendViewBean(t, engine, "d", 0)

	// Last batch should show new=d, old=a
	lastBatch := (*batches)[len(*batches)-1]
	if len(lastBatch.New) != 1 {
		t.Fatalf("new rows = %d, want 1", len(lastBatch.New))
	}
	if got := lastBatch.New[0].Get("theString").Any(); got != "d" {
		t.Fatalf("new theString=%v, want d", got)
	}

	if len(lastBatch.Old) != 1 {
		t.Fatalf("old rows = %d, want 1", len(lastBatch.Old))
	}
	if got := lastBatch.Old[0].Get("theString").Any(); got != "a" {
		t.Fatalf("old theString=%v, want a (evicted)", got)
	}
}

// TestEPLOtherRStreamSelectorMatchesEsper covers
// EPLOtherIStreamRStreamConfigSelectorRStream: rstream selector reports
// only removed events as new data; inserts produce no output.
func TestEPLOtherRStreamSelectorMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean")
	plan, err := env.Build(source.Window(LengthWindow(3)).Query(
		StatementName("s0"), WithRemoveStreamOnly(),
	))
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

	// Send a,b,c - rstream does not report inserts, so no output
	sendViewBean(t, engine, "a", 0)
	sendViewBean(t, engine, "b", 0)
	sendViewBean(t, engine, "c", 0)
	if len(batches) != 0 {
		t.Fatalf("rstream should not report inserts, got %d batches", len(batches))
	}

	// Send d - 'a' evicted, rstream reports 'a' via Old (Go models rstream
	// removed events in batch.Old rather than listener newData)
	sendViewBean(t, engine, "d", 0)
	if len(batches) != 1 {
		t.Fatalf("rstream should report eviction, got %d batches", len(batches))
	}
	if len(batches[0].Old) != 1 {
		t.Fatalf("rstream old rows = %d, want 1", len(batches[0].Old))
	}
	if got := batches[0].Old[0].Get("theString").Any(); got != "a" {
		t.Fatalf("rstream old theString=%v, want a (evicted)", got)
	}
}
