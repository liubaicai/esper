package esper

import (
	"context"
	"testing"
)

type viewUniqueMarketData struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
}

// TestViewFirstUniqueSimpleMatchesEsper covers ViewFirstUniqueSimple:
// select irstream theString as c0, intPrimitive as c1 from
// SupportBean#firstunique(theString). Only the first event per unique key
// is inserted; later events with the same key never invoke the listener,
// and the iterator exposes one retained event per key.
func TestViewFirstUniqueSimpleMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean").
		Window(FirstUnique(Field[viewParityBean, string]("theString")))
	plan, err := env.Build(Select(source,
		Alias("c0", Field[viewParityBean, string]("theString")),
		Alias("c1", Field[viewParityBean, int]("intPrimitive")),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	stmt := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	snapshotStrings := func() []string {
		t.Helper()
		result, err := stmt.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		values := make([]string, 0, len(result.Results()))
		for _, row := range result.Results() {
			values = append(values, row.Get("c0").Any().(string))
		}
		return values
	}
	assertSnapshot := func(want ...string) {
		t.Helper()
		got := snapshotStrings()
		if len(got) != len(want) {
			t.Fatalf("snapshot size = %v, want %v", got, want)
		}
		seen := make(map[string]int, len(got))
		for _, value := range got {
			seen[value]++
		}
		for _, value := range want {
			if seen[value] == 0 {
				t.Fatalf("snapshot %v missing %q", got, value)
			}
			seen[value]--
		}
	}

	// Empty iterator before any event.
	assertSnapshot()

	// E1/1 inserted.
	sendViewBean(t, engine, "E1", 1)
	if len(batches) != 1 || len(batches[0].New) != 1 || len(batches[0].Old) != 0 {
		t.Fatalf("E1: batches = %#v", batches)
	}
	if got := batches[0].New[0].Get("c0").Any(); got != "E1" {
		t.Fatalf("E1 c0 = %v", got)
	}
	if got := batches[0].New[0].Get("c1").Any(); got != 1 {
		t.Fatalf("E1 c1 = %v", got)
	}

	// E2/20 inserted; iterator sees E1+E2.
	sendViewBean(t, engine, "E2", 20)
	if len(batches) != 2 || len(batches[1].New) != 1 {
		t.Fatalf("E2: batches = %#v", batches)
	}
	assertSnapshot("E1", "E2")

	// Duplicate keys E1/E2 are not inserted and do not invoke the listener.
	sendViewBean(t, engine, "E1", 2)
	if len(batches) != 2 {
		t.Fatalf("duplicate E1 invoked listener: %#v", batches)
	}
	sendViewBean(t, engine, "E2", 21)
	if len(batches) != 2 {
		t.Fatalf("duplicate E2 invoked listener: %#v", batches)
	}
	assertSnapshot("E1", "E2")

	// More duplicates still do not invoke the listener.
	sendViewBean(t, engine, "E2", 22)
	sendViewBean(t, engine, "E1", 3)
	if len(batches) != 2 {
		t.Fatalf("later duplicates invoked listener: %#v", batches)
	}
	assertSnapshot("E1", "E2")

	// New key E3/30 is inserted.
	sendViewBean(t, engine, "E3", 30)
	if len(batches) != 3 || len(batches[2].New) != 1 {
		t.Fatalf("E3: batches = %#v", batches)
	}
	if got := batches[2].New[0].Get("c0").Any(); got != "E3" {
		t.Fatalf("E3 c0 = %v", got)
	}
	if got := batches[2].New[0].Get("c1").Any(); got != 30 {
		t.Fatalf("E3 c1 = %v", got)
	}
	assertSnapshot("E1", "E2", "E3")
}

// TestViewFirstUniqueSceneOneMatchesEsper covers ViewFirstUniqueSceneOne:
// select irstream symbol, price from SupportMarketDataBean#firstunique(symbol)
// order by symbol. First event per symbol is inserted; duplicates are
// suppressed; the iterator is ordered by symbol.
func TestViewFirstUniqueSceneOneMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[viewUniqueMarketData](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").
		Window(FirstUnique(Field[viewUniqueMarketData, string]("symbol")))
	plan, err := env.Build(source.Query(
		StatementName("s0"),
		WithOldStream(),
		OrderBy(Ascending(Field[viewUniqueMarketData, string]("symbol"))),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	stmt := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	send := func(symbol string, price float64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), viewUniqueMarketData{Symbol: symbol, Price: price}); err != nil {
			t.Fatal(err)
		}
	}

	// S1/100 and S2/5 inserted.
	send("S1", 100)
	if len(batches) != 1 || len(batches[0].New) != 1 || len(batches[0].Old) != 0 {
		t.Fatalf("S1: batches = %#v", batches)
	}
	if got := batches[0].New[0].Get("symbol").Any(); got != "S1" {
		t.Fatalf("S1 symbol = %v", got)
	}
	if got := batches[0].New[0].Get("price").Any(); got != 100.0 {
		t.Fatalf("S1 price = %v", got)
	}
	send("S2", 5)
	if len(batches) != 2 || len(batches[1].New) != 1 {
		t.Fatalf("S2: batches = %#v", batches)
	}
	if got := batches[1].New[0].Get("price").Any(); got != 5.0 {
		t.Fatalf("S2 price = %v", got)
	}

	// Duplicate S1 events are suppressed.
	send("S1", 101)
	send("S1", 102)
	if len(batches) != 2 {
		t.Fatalf("duplicate S1 invoked listener: %#v", batches)
	}

	// Iterator ordered by symbol: S1 (100.0) then S2 (5.0).
	result, err := stmt.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 2 {
		t.Fatalf("snapshot = %#v", result.Results())
	}
	if got := result.Results()[0].Get("price").Any(); got != 100.0 {
		t.Fatalf("iterator first price = %v, want 100.0 (S1)", got)
	}
	if got := result.Results()[1].Get("price").Any(); got != 5.0 {
		t.Fatalf("iterator second price = %v, want 5.0 (S2)", got)
	}

	// S3/6 inserted.
	send("S3", 6)
	if len(batches) != 3 || len(batches[2].New) != 1 {
		t.Fatalf("S3: batches = %#v", batches)
	}
	if got := batches[2].New[0].Get("symbol").Any(); got != "S3" {
		t.Fatalf("S3 symbol = %v", got)
	}
}
