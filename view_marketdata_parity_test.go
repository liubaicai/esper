package esper

import (
	"context"
	"reflect"
	"testing"
)

func newViewMarketDataEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[viewUniqueMarketData](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	return env, engine
}

func sendViewMarketData(t *testing.T, engine *Engine, symbol string, price float64) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), viewUniqueMarketData{Symbol: symbol, Price: price}); err != nil {
		t.Fatal(err)
	}
}

func deployViewMarketData(t *testing.T, env *Environment, engine *Engine, stream Stream[viewUniqueMarketData], name string) (*Statement, *[]ResultBatch) {
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

func snapshotSymbols(t *testing.T, stmt *Statement) []string {
	t.Helper()
	result, err := stmt.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	symbols := make([]string, 0, len(result.Results()))
	for _, row := range result.Results() {
		symbols = append(symbols, row.Get("symbol").Any().(string))
	}
	return symbols
}

// TestViewFirstEventMarketDataMatchesEsper covers ViewFirstEventMarketData:
// select irstream * from SupportMarketDataBean#firstevent() — only the first
// event is inserted; the iterator keeps returning that first event.
func TestViewFirstEventMarketDataMatchesEsper(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(FirstEvent())
	stmt, batches := deployViewMarketData(t, env, engine, source, "s0")

	sendViewMarketData(t, engine, "E1", 0)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 || len((*batches)[0].Old) != 0 {
		t.Fatalf("E1: batches = %#v", *batches)
	}
	if got := (*batches)[0].New[0].Get("symbol").Any(); got != "E1" {
		t.Fatalf("E1 symbol = %v", got)
	}

	sendViewMarketData(t, engine, "E2", 0)
	if len(*batches) != 1 {
		t.Fatalf("E2 invoked listener: %#v", *batches)
	}
	if got := snapshotSymbols(t, stmt); !reflect.DeepEqual(got, []string{"E1"}) {
		t.Fatalf("iterator after E2 = %v, want [E1]", got)
	}

	sendViewMarketData(t, engine, "E3", 0)
	if len(*batches) != 1 {
		t.Fatalf("E3 invoked listener: %#v", *batches)
	}
	if got := snapshotSymbols(t, stmt); !reflect.DeepEqual(got, []string{"E1"}) {
		t.Fatalf("iterator after E3 = %v, want [E1]", got)
	}
}

// TestViewLastEventMarketDataMatchesEsper covers ViewLastEventMarketData:
// select irstream * from SupportMarketDataBean#lastevent() — each new event
// replaces the previous one (new=Ei, old=Ei-1); iterator shows the latest.
func TestViewLastEventMarketDataMatchesEsper(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(LastEvent())
	stmt, batches := deployViewMarketData(t, env, engine, source, "s0")

	sendViewMarketData(t, engine, "E1", 0)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 || len((*batches)[0].Old) != 0 {
		t.Fatalf("E1: batches = %#v", *batches)
	}

	sendViewMarketData(t, engine, "E2", 0)
	if len(*batches) != 2 {
		t.Fatalf("E2: batches = %#v", *batches)
	}
	if got := (*batches)[1].New[0].Get("symbol").Any(); got != "E2" {
		t.Fatalf("E2 new symbol = %v", got)
	}
	if len((*batches)[1].Old) != 1 || (*batches)[1].Old[0].Get("symbol").Any() != "E1" {
		t.Fatalf("E2 old = %#v, want E1", (*batches)[1].Old)
	}
	if got := snapshotSymbols(t, stmt); !reflect.DeepEqual(got, []string{"E2"}) {
		t.Fatalf("iterator after E2 = %v, want [E2]", got)
	}

	for i := 3; i < 10; i++ {
		cur := "E" + string(rune('0'+i))
		prev := "E" + string(rune('0'+i-1))
		sendViewMarketData(t, engine, cur, 0)
		batch := (*batches)[len(*batches)-1]
		if len(batch.New) != 1 || batch.New[0].Get("symbol").Any() != cur {
			t.Fatalf("%s new = %#v", cur, batch.New)
		}
		if len(batch.Old) != 1 || batch.Old[0].Get("symbol").Any() != prev {
			t.Fatalf("%s old = %#v, want %s", cur, batch.Old, prev)
		}
	}
}

// TestViewFirstLengthMarketDataMatchesEsper covers ViewFirstLengthMarketData:
// select irstream * from SupportMarketDataBean#firstlength(3) — the first
// three events are inserted; later events are suppressed; iterator retains
// the first three.
func TestViewFirstLengthMarketDataMatchesEsper(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(FirstLength(3))
	stmt, batches := deployViewMarketData(t, env, engine, source, "s0")

	sendViewMarketData(t, engine, "E1", 0)
	if got := snapshotSymbols(t, stmt); !reflect.DeepEqual(got, []string{"E1"}) {
		t.Fatalf("iterator after E1 = %v, want [E1]", got)
	}
	sendViewMarketData(t, engine, "E2", 0)
	if got := snapshotSymbols(t, stmt); !reflect.DeepEqual(got, []string{"E1", "E2"}) {
		t.Fatalf("iterator after E2 = %v, want [E1 E2]", got)
	}
	sendViewMarketData(t, engine, "E3", 0)
	if len(*batches) != 3 {
		t.Fatalf("after E3: batches = %d, want 3", len(*batches))
	}
	if got := snapshotSymbols(t, stmt); !reflect.DeepEqual(got, []string{"E1", "E2", "E3"}) {
		t.Fatalf("iterator after E3 = %v, want [E1 E2 E3]", got)
	}

	sendViewMarketData(t, engine, "E4", 0)
	if len(*batches) != 3 {
		t.Fatalf("E4 invoked listener: %#v", *batches)
	}
	if got := snapshotSymbols(t, stmt); !reflect.DeepEqual(got, []string{"E1", "E2", "E3"}) {
		t.Fatalf("iterator after E4 = %v, want [E1 E2 E3]", got)
	}
}

// TestViewLengthWindowIteratorMatchesEsper covers ViewLengthWindowIterator:
// select symbol, price from SupportMarketDataBean#length(2) — the iterator
// returns the retained events in arrival order after eviction.
func TestViewLengthWindowIteratorMatchesEsper(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(LengthWindow(2))
	stmt, _ := deployViewMarketData(t, env, engine, source, "s0")

	sendViewMarketData(t, engine, "ABC", 20)
	sendViewMarketData(t, engine, "DEF", 100)

	result, err := stmt.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 2 {
		t.Fatalf("iterator size = %d, want 2", len(result.Results()))
	}
	if got := result.Results()[0].Get("symbol").Any(); got != "ABC" {
		t.Fatalf("iterator[0] symbol = %v, want ABC", got)
	}
	if got := result.Results()[1].Get("price").Any(); got != 100.0 {
		t.Fatalf("iterator[1] price = %v, want 100", got)
	}

	sendViewMarketData(t, engine, "EFG", 50)
	result, err = stmt.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 2 {
		t.Fatalf("iterator size after eviction = %d, want 2", len(result.Results()))
	}
	if got := result.Results()[0].Get("symbol").Any(); got != "DEF" {
		t.Fatalf("iterator[0] symbol = %v, want DEF (ABC evicted)", got)
	}
	if got := result.Results()[1].Get("symbol").Any(); got != "EFG" {
		t.Fatalf("iterator[1] symbol = %v, want EFG", got)
	}
	if got := result.Results()[1].Get("price").Any(); got != 50.0 {
		t.Fatalf("iterator[1] price = %v, want 50", got)
	}
}

// TestViewLengthWindowPrevPriorMatchesEsper covers ViewLengthWindowWPrevPrior:
// select irstream symbol, prev(1, symbol), prior(1, symbol), prevtail(symbol),
// prevcount(symbol), prevwindow(symbol) from SupportMarketDataBean#length(2).
// Window-history functions see the current window contents; old rows report
// the evicted event with its own (now empty) window history.
//
// API mapping note: Java prior(1, x) addresses the immediately preceding
// event (1-based from current); Go Prior is 0-based from the previous event,
// so Java prior(1, x) maps to Go Prior(0, x).
func TestViewLengthWindowPrevPriorMatchesEsper(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	symbol := Field[viewUniqueMarketData, string]("symbol")
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(LengthWindow(2))
	plan, err := env.Build(Select(source,
		Alias("symbol", symbol),
		Alias("prev1", Prev[string](1, symbol)),
		Alias("prio1", Prior[string](0, symbol)), // Java prior(1, symbol)
		Alias("prevtail0", PrevTail[string](0, symbol)),
		Alias("prevCountSym", PrevCount[string](symbol)),
		Alias("prevWindowSym", PrevWindow[string](symbol)),
	).Query(StatementName("s0"), WithOldStream()))
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

	// E1: prev1/prio1 null, prevtail0=E1, prevCountSym=1, prevWindowSym=[E1]
	sendViewMarketData(t, engine, "E1", 0)
	if len(batches) != 1 {
		t.Fatalf("E1: batches = %#v", batches)
	}
	row := batches[0].New[0]
	if got := row.Get("prev1"); got.IsPresent() {
		t.Fatalf("E1 prev1 = %v, want null", got)
	}
	if got := row.Get("prio1"); got.IsPresent() {
		t.Fatalf("E1 prio1 = %v, want null", got)
	}
	if got := row.Get("prevtail0").Any(); got != "E1" {
		t.Fatalf("E1 prevtail0 = %v, want E1", got)
	}
	if got := row.Get("prevCountSym").Any(); got != int64(1) {
		t.Fatalf("E1 prevCountSym = %v, want 1", got)
	}
	if got := row.Get("prevWindowSym").Any(); !reflect.DeepEqual(got, []string{"E1"}) {
		t.Fatalf("E1 prevWindowSym = %#v, want [E1]", got)
	}

	// E2: prev1=prio1=E1, prevtail0=E1, prevCountSym=2, prevWindowSym=[E2 E1]
	sendViewMarketData(t, engine, "E2", 0)
	if len(batches) != 2 {
		t.Fatalf("E2: batches = %#v", batches)
	}
	row = batches[1].New[0]
	if got := row.Get("prev1").Any(); got != "E1" {
		t.Fatalf("E2 prev1 = %v, want E1", got)
	}
	if got := row.Get("prio1").Any(); got != "E1" {
		t.Fatalf("E2 prio1 = %v, want E1", got)
	}
	if got := row.Get("prevtail0").Any(); got != "E1" {
		t.Fatalf("E2 prevtail0 = %v, want E1", got)
	}
	if got := row.Get("prevCountSym").Any(); got != int64(2) {
		t.Fatalf("E2 prevCountSym = %v, want 2", got)
	}
	if got := row.Get("prevWindowSym").Any(); !reflect.DeepEqual(got, []string{"E2", "E1"}) {
		t.Fatalf("E2 prevWindowSym = %#v, want [E2 E1]", got)
	}

	// E3..E9: window slides; new row has prev1=prio1=prevtail0=E(i-1),
	// old row reports the evicted E(i-2) with null window history.
	for i := 3; i < 10; i++ {
		cur := "E" + string(rune('0'+i))
		prev := "E" + string(rune('0'+i-1))
		evicted := "E" + string(rune('0'+i-2))
		sendViewMarketData(t, engine, cur, 0)
		batch := batches[len(batches)-1]
		newRow := batch.New[0]
		if got := newRow.Get("symbol").Any(); got != cur {
			t.Fatalf("%s new symbol = %v", cur, got)
		}
		if got := newRow.Get("prev1").Any(); got != prev {
			t.Fatalf("%s new prev1 = %v, want %s", cur, got, prev)
		}
		if got := newRow.Get("prio1").Any(); got != prev {
			t.Fatalf("%s new prio1 = %v, want %s", cur, got, prev)
		}
		if got := newRow.Get("prevtail0").Any(); got != prev {
			t.Fatalf("%s new prevtail0 = %v, want %s", cur, got, prev)
		}
		if len(batch.Old) != 1 {
			t.Fatalf("%s old rows = %d, want 1", cur, len(batch.Old))
		}
		oldRow := batch.Old[0]
		if got := oldRow.Get("symbol").Any(); got != evicted {
			t.Fatalf("%s old symbol = %v, want %s", cur, got, evicted)
		}
		if got := oldRow.Get("prev1"); got.IsPresent() {
			t.Fatalf("%s old prev1 = %v, want null", cur, got)
		}
		if got := oldRow.Get("prevtail0"); got.IsPresent() {
			t.Fatalf("%s old prevtail0 = %v, want null", cur, got)
		}
	}

	// Iterator after E9: window holds E8, E9 in arrival order.
	stmt := deployment.Statements()[0]
	if got := snapshotSymbols(t, stmt); !reflect.DeepEqual(got, []string{"E8", "E9"}) {
		t.Fatalf("iterator = %v, want [E8 E9]", got)
	}
}
