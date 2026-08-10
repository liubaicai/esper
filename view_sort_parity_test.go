package esper

import (
	"context"
	"reflect"
	"testing"
)

type viewSortBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type viewSortEnumBean struct {
	TheString   string `esper:"theString"`
	SupportEnum string `esper:"supportEnum"`
}

func newViewSortEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[viewSortBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	return env, engine
}

func sendViewSortBean(t *testing.T, engine *Engine, theString string, intPrim int, longPrim int64) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), viewSortBean{TheString: theString, IntPrimitive: intPrim, LongPrimitive: longPrim}); err != nil {
		t.Fatal(err)
	}
}

func assertSortIterator(t *testing.T, stmt *Statement, field string, want ...any) {
	t.Helper()
	result, err := stmt.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != len(want) {
		t.Fatalf("iterator size = %d, want %d (%#v)", len(result.Results()), len(want), result.Results())
	}
	for i, row := range result.Results() {
		if got := row.Get(field).Any(); got != want[i] {
			t.Fatalf("iterator[%d] %s = %v, want %v", i, field, got, want[i])
		}
	}
}

// TestViewSortSceneOneMatchesEsper covers ViewSortSceneOne:
// select irstream * from SupportBean#sort(3, intPrimitive desc, longPrimitive).
// The sort window keeps the top 3 events by (intPrimitive desc, longPrimitive
// asc); arrivals that rank below the retained set are immediately evicted
// (reported in both new and old streams).
func TestViewSortSceneOneMatchesEsper(t *testing.T) {
	env, engine := newViewSortEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	intPrim := Field[viewSortBean, int]("intPrimitive")
	longPrim := Field[viewSortBean, int64]("longPrimitive")
	source := From[viewSortBean](env, "SupportBean").
		Window(SortWindow(3, Descending(intPrim), Ascending(longPrim)))
	stmt, batches := deployViewSort(t, env, engine, source, "s0")

	assertIR := func(index int, newSym string, newInt int, newLong int64, old *viewSortBean) {
		t.Helper()
		if len(*batches) != index+1 {
			t.Fatalf("batches = %d, want %d", len(*batches), index+1)
		}
		batch := (*batches)[index]
		if len(batch.New) != 1 {
			t.Fatalf("event %s: new rows = %d", newSym, len(batch.New))
		}
		if got := batch.New[0].Get("theString").Any(); got != newSym {
			t.Fatalf("event %s: new theString = %v", newSym, got)
		}
		if old == nil {
			if len(batch.Old) != 0 {
				t.Fatalf("event %s: old rows = %d, want 0", newSym, len(batch.Old))
			}
			return
		}
		if len(batch.Old) != 1 {
			t.Fatalf("event %s: old rows = %d, want 1", newSym, len(batch.Old))
		}
		if got := batch.Old[0].Get("theString").Any(); got != old.TheString {
			t.Fatalf("event %s: old theString = %v, want %s", newSym, got, old.TheString)
		}
		if got := batch.Old[0].Get("intPrimitive").Any(); got != old.IntPrimitive {
			t.Fatalf("event %s: old intPrimitive = %v, want %d", newSym, got, old.IntPrimitive)
		}
	}

	sendViewSortBean(t, engine, "E1", 100, 0)
	assertIR(0, "E1", 100, 0, nil)
	assertSortIterator(t, stmt, "theString", "E1")

	sendViewSortBean(t, engine, "E2", 99, 5)
	assertIR(1, "E2", 99, 5, nil)
	assertSortIterator(t, stmt, "theString", "E1", "E2")

	sendViewSortBean(t, engine, "E3", 100, -1)
	assertIR(2, "E3", 100, -1, nil)
	assertSortIterator(t, stmt, "theString", "E3", "E1", "E2")

	sendViewSortBean(t, engine, "E4", 100, 1)
	assertIR(3, "E4", 100, 1, &viewSortBean{TheString: "E2", IntPrimitive: 99, LongPrimitive: 5})
	assertSortIterator(t, stmt, "theString", "E3", "E1", "E4")

	sendViewSortBean(t, engine, "E5", 101, 10)
	assertIR(4, "E5", 101, 10, &viewSortBean{TheString: "E4", IntPrimitive: 100, LongPrimitive: 1})
	assertSortIterator(t, stmt, "theString", "E5", "E3", "E1")

	sendViewSortBean(t, engine, "E6", 101, 11)
	assertIR(5, "E6", 101, 11, &viewSortBean{TheString: "E1", IntPrimitive: 100, LongPrimitive: 0})
	assertSortIterator(t, stmt, "theString", "E5", "E6", "E3")

	// Arrival ranks below the retained set: reported as both new and old.
	sendViewSortBean(t, engine, "E6", 100, 0)
	assertIR(6, "E6", 100, 0, &viewSortBean{TheString: "E6", IntPrimitive: 100, LongPrimitive: 0})
	assertSortIterator(t, stmt, "theString", "E5", "E6", "E3")
}

// TestViewSortSceneTwoMatchesEsper covers ViewSortSceneTwo:
// select irstream * from SupportBean#sort(3, theString). Equal sort keys
// order most-recent-first in the iterator.
func TestViewSortSceneTwoMatchesEsper(t *testing.T) {
	env, engine := newViewSortEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	theString := Field[viewSortBean, string]("theString")
	source := From[viewSortBean](env, "SupportBean").
		Window(SortWindow(3, Ascending(theString)))
	stmt, batches := deployViewSort(t, env, engine, source, "s0")

	send := func(sym string, intPrim int) {
		t.Helper()
		sendViewSortBean(t, engine, sym, intPrim, 0)
	}
	assertIRPair := func(index int, newSym string, newInt int, oldSym string, oldInt int) {
		t.Helper()
		batch := (*batches)[index]
		if len(batch.New) != 1 || batch.New[0].Get("theString").Any() != newSym || batch.New[0].Get("intPrimitive").Any() != newInt {
			t.Fatalf("batch %d new = %#v, want %s/%d", index, batch.New, newSym, newInt)
		}
		if len(batch.Old) != 1 || batch.Old[0].Get("theString").Any() != oldSym || batch.Old[0].Get("intPrimitive").Any() != oldInt {
			t.Fatalf("batch %d old = %#v, want %s/%d", index, batch.Old, oldSym, oldInt)
		}
	}

	send("G", 1)
	send("E", 2)
	send("H", 3)
	if len(*batches) != 3 {
		t.Fatalf("batches = %d, want 3", len(*batches))
	}
	assertSortIterator(t, stmt, "theString", "E", "G", "H")

	// I ranks last: self-evicted.
	send("I", 4)
	assertIRPair(3, "I", 4, "I", 4)
	assertSortIterator(t, stmt, "theString", "E", "G", "H")

	send("A", 5)
	assertIRPair(4, "A", 5, "H", 3)
	assertSortIterator(t, stmt, "theString", "A", "E", "G")

	send("C", 6)
	assertIRPair(5, "C", 6, "G", 1)
	assertSortIterator(t, stmt, "theString", "A", "C", "E")

	// Equal keys: newest first.
	send("C", 7)
	assertIRPair(6, "C", 7, "E", 2)
	result, err := stmt.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 3 {
		t.Fatalf("iterator size = %d, want 3", len(result.Results()))
	}
	if got := result.Results()[1].Get("intPrimitive").Any(); got != 7 {
		t.Fatalf("iterator[1] intPrimitive = %v, want 7 (newest C first)", got)
	}
	if got := result.Results()[2].Get("intPrimitive").Any(); got != 6 {
		t.Fatalf("iterator[2] intPrimitive = %v, want 6", got)
	}

	send("C", 8)
	assertIRPair(7, "C", 8, "C", 6)
	result, err = stmt.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Results()[1].Get("intPrimitive").Any(); got != 8 {
		t.Fatalf("iterator[1] intPrimitive = %v, want 8", got)
	}
	if got := result.Results()[2].Get("intPrimitive").Any(); got != 7 {
		t.Fatalf("iterator[2] intPrimitive = %v, want 7", got)
	}
}

// TestViewSortedSingleKeyBuiltinMatchesEsper covers ViewSortedSingleKeyBuiltin:
// select irstream * from SupportMarketDataBean#sort(3, symbol).
func TestViewSortedSingleKeyBuiltinMatchesEsper(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	symbol := Field[viewUniqueMarketData, string]("symbol")
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").
		Window(SortWindow(3, Ascending(symbol)))
	stmt, batches := deployViewMarketData(t, env, engine, source, "s0")

	assertIR := func(index int, newSym string, oldSym string) {
		t.Helper()
		batch := (*batches)[index]
		if len(batch.New) != 1 || batch.New[0].Get("symbol").Any() != newSym {
			t.Fatalf("batch %d new = %#v, want %s", index, batch.New, newSym)
		}
		if oldSym == "" {
			if len(batch.Old) != 0 {
				t.Fatalf("batch %d old = %#v, want none", index, batch.Old)
			}
			return
		}
		if len(batch.Old) != 1 || batch.Old[0].Get("symbol").Any() != oldSym {
			t.Fatalf("batch %d old = %#v, want %s", index, batch.Old, oldSym)
		}
	}

	sendViewMarketData(t, engine, "B1", 0)
	assertIR(0, "B1", "")
	sendViewMarketData(t, engine, "D1", 0)
	assertIR(1, "D1", "")
	sendViewMarketData(t, engine, "C1", 0)
	assertIR(2, "C1", "")
	sendViewMarketData(t, engine, "A1", 0)
	assertIR(3, "A1", "D1")
	sendViewMarketData(t, engine, "F1", 0)
	assertIR(4, "F1", "F1")
	sendViewMarketData(t, engine, "B2", 0)
	assertIR(5, "B2", "C1")

	assertSortIterator(t, stmt, "symbol", "A1", "B1", "B2")
}

// TestViewSortedMultikeyMatchesEsper covers ViewSortedMultikey:
// select irstream * from SupportBeanWithEnum#sort(1, theString, supportEnum).
// Java SupportEnum orders by enum ordinal; the Go model uses the enum name
// string, whose lexicographic order matches the ordinal order for
// ENUM_VALUE_1 < ENUM_VALUE_2.
func TestViewSortedMultikeyMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[viewSortEnumBean](env, "SupportBeanWithEnum"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	theString := Field[viewSortEnumBean, string]("theString")
	supportEnum := Field[viewSortEnumBean, string]("supportEnum")
	source := From[viewSortEnumBean](env, "SupportBeanWithEnum").
		Window(SortWindow(1, Ascending(theString), Ascending(supportEnum)))
	_, batches := deployViewSortEnum(t, env, engine, source, "s0")

	send := func(sym, enumValue string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), viewSortEnumBean{TheString: sym, SupportEnum: enumValue}); err != nil {
			t.Fatal(err)
		}
	}

	send("E1", "ENUM_VALUE_1")
	if len(*batches) != 1 || len((*batches)[0].New) != 1 || len((*batches)[0].Old) != 0 {
		t.Fatalf("E1: batches = %#v", *batches)
	}

	// E2 ranks after E1: self-evicted.
	send("E2", "ENUM_VALUE_2")
	if len(*batches) != 2 {
		t.Fatalf("E2: batches = %#v", *batches)
	}
	if got := (*batches)[1].New[0].Get("theString").Any(); got != "E2" {
		t.Fatalf("E2 new = %v", got)
	}
	if len((*batches)[1].Old) != 1 || (*batches)[1].Old[0].Get("theString").Any() != "E2" {
		t.Fatalf("E2 old = %#v, want E2 (self-evicted)", (*batches)[1].Old)
	}

	// E0 ranks before E1: E1 evicted.
	send("E0", "ENUM_VALUE_1")
	if len(*batches) != 3 {
		t.Fatalf("E0: batches = %#v", *batches)
	}
	if got := (*batches)[2].New[0].Get("theString").Any(); got != "E0" {
		t.Fatalf("E0 new = %v", got)
	}
	if len((*batches)[2].Old) != 1 || (*batches)[2].Old[0].Get("theString").Any() != "E1" {
		t.Fatalf("E0 old = %#v, want E1", (*batches)[2].Old)
	}
}

// TestViewSortedPrimitiveKeyMatchesEsper covers ViewSortedPrimitiveKey:
// select irstream * from SupportMarketDataBean#sort(1, price).
func TestViewSortedPrimitiveKeyMatchesEsper(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	price := Field[viewUniqueMarketData, float64]("price")
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").
		Window(SortWindow(1, Ascending(price)))
	_, batches := deployViewMarketData(t, env, engine, source, "s0")

	sendViewMarketData(t, engine, "P1", 10.5)
	if len(*batches) != 1 || len((*batches)[0].Old) != 0 {
		t.Fatalf("10.5: batches = %#v", *batches)
	}

	sendViewMarketData(t, engine, "P2", 10)
	if len(*batches) != 2 || (*batches)[1].New[0].Get("price").Any() != 10.0 {
		t.Fatalf("10: batches = %#v", *batches)
	}
	if len((*batches)[1].Old) != 1 || (*batches)[1].Old[0].Get("price").Any() != 10.5 {
		t.Fatalf("10 old = %#v, want 10.5", (*batches)[1].Old)
	}

	// 11 ranks last: self-evicted.
	sendViewMarketData(t, engine, "P3", 11)
	if len(*batches) != 3 || (*batches)[2].New[0].Get("price").Any() != 11.0 {
		t.Fatalf("11: batches = %#v", *batches)
	}
	if len((*batches)[2].Old) != 1 || (*batches)[2].Old[0].Get("price").Any() != 11.0 {
		t.Fatalf("11 old = %#v, want 11 (self-evicted)", (*batches)[2].Old)
	}
}

// TestViewSortedPrevMatchesEsper covers ViewSortedPrev:
// select irstream symbol, prev(1, symbol), prevtail(symbol), prevcount(symbol),
// prevwindow(symbol) from SupportMarketDataBean#sort(3, symbol). On sorted
// windows the previous-value functions follow the sorted access order:
// prev(1) is the second-lowest entry, prevtail the highest, prevwindow the
// entries in ascending sort order.
func TestViewSortedPrevMatchesEsper(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	symbol := Field[viewUniqueMarketData, string]("symbol")
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").
		Window(SortWindow(3, Ascending(symbol)))
	plan, err := env.Build(Select(source,
		Alias("symbol", symbol),
		Alias("prev1", Prev[string](1, symbol)),
		Alias("prevtail", PrevTail[string](0, symbol)),
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

	type expectation struct {
		symbol     string
		prev1      any
		prevtail   string
		count      int64
		prevWindow []string
	}
	assertRow := func(index int, want expectation) {
		t.Helper()
		if len(batches) != index+1 {
			t.Fatalf("batches = %d, want %d", len(batches), index+1)
		}
		row := batches[index].New[0]
		if got := row.Get("symbol").Any(); got != want.symbol {
			t.Fatalf("event %s: symbol = %v", want.symbol, got)
		}
		prev1 := row.Get("prev1")
		if want.prev1 == nil {
			if prev1.IsPresent() {
				t.Fatalf("event %s: prev1 = %v, want null", want.symbol, prev1)
			}
		} else if prev1.Any() != want.prev1 {
			t.Fatalf("event %s: prev1 = %v, want %v", want.symbol, prev1.Any(), want.prev1)
		}
		if got := row.Get("prevtail").Any(); got != want.prevtail {
			t.Fatalf("event %s: prevtail = %v, want %s", want.symbol, got, want.prevtail)
		}
		if got := row.Get("prevCountSym").Any(); got != want.count {
			t.Fatalf("event %s: prevCountSym = %v, want %d", want.symbol, got, want.count)
		}
		if got := row.Get("prevWindowSym").Any(); !reflect.DeepEqual(got, want.prevWindow) {
			t.Fatalf("event %s: prevWindowSym = %#v, want %v", want.symbol, got, want.prevWindow)
		}
	}

	sendViewMarketData(t, engine, "B1", 0)
	assertRow(0, expectation{"B1", nil, "B1", 1, []string{"B1"}})

	sendViewMarketData(t, engine, "D1", 0)
	assertRow(1, expectation{"D1", "D1", "D1", 2, []string{"B1", "D1"}})

	sendViewMarketData(t, engine, "C1", 0)
	assertRow(2, expectation{"C1", "C1", "D1", 3, []string{"B1", "C1", "D1"}})

	sendViewMarketData(t, engine, "A1", 0)
	assertRow(3, expectation{"A1", "B1", "C1", 3, []string{"A1", "B1", "C1"}})

	sendViewMarketData(t, engine, "F1", 0)
	assertRow(4, expectation{"F1", "B1", "C1", 3, []string{"A1", "B1", "C1"}})
}

func deployViewSort(t *testing.T, env *Environment, engine *Engine, stream Stream[viewSortBean], name string) (*Statement, *[]ResultBatch) {
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

func deployViewSortEnum(t *testing.T, env *Environment, engine *Engine, stream Stream[viewSortEnumBean], name string) (*Statement, *[]ResultBatch) {
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
