package esper

import (
	"context"
	"testing"
)

type viewUniqueMDBean struct {
	Symbol string  `esper:"symbol"`
	Feed   string  `esper:"feed"`
	Price  float64 `esper:"price"`
}

func newViewUniqueMDEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[viewUniqueMDBean](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[viewParityBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func sendViewUniqueMD(t *testing.T, engine *Engine, symbol string, price float64) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), viewUniqueMDBean{Symbol: symbol, Price: price}); err != nil {
		t.Fatal(err)
	}
}

func sendViewUniqueMDFeed(t *testing.T, engine *Engine, symbol, feed string, price float64) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), viewUniqueMDBean{Symbol: symbol, Feed: feed, Price: price}); err != nil {
		t.Fatal(err)
	}
}

func lastUniqueNewPrice(t *testing.T, batches []ResultBatch) float64 {
	t.Helper()
	if len(batches) == 0 {
		t.Fatal("no batches")
	}
	batch := batches[len(batches)-1]
	if len(batch.New) == 0 {
		t.Fatal("no new events")
	}
	return batch.New[0].Get("price").Any().(float64)
}

func lastUniqueOldPrice(t *testing.T, batches []ResultBatch) any {
	t.Helper()
	if len(batches) == 0 {
		return nil
	}
	batch := batches[len(batches)-1]
	if len(batch.Old) == 0 {
		return nil
	}
	return batch.Old[0].Get("price").Any()
}

// TestViewUniqueSceneOneMatchesEsper covers ViewLastUniqueSceneOne:
// unique(symbol) keeps only the latest event per symbol key.
func TestViewUniqueSceneOneMatchesEsper(t *testing.T) {
	env := newViewUniqueMDEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewUniqueMDBean](env, "SupportMarketDataBean").
		Window(Unique(Field[viewUniqueMDBean, string]("symbol")))
	plan, err := env.Build(Select(source,
		Alias("symbol", Field[viewUniqueMDBean, string]("symbol")),
		Alias("price", Field[viewUniqueMDBean, float64]("price")),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	stmt := dep.Statements()[0]
	var batches []ResultBatch
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendViewUniqueMD(t, engine, "S1", 100)
	if got := lastUniqueNewPrice(t, batches); got != 100 {
		t.Fatalf("S1 new price = %v, want 100", got)
	}
	if lastUniqueOldPrice(t, batches) != nil {
		t.Fatal("S1 should have no old")
	}

	sendViewUniqueMD(t, engine, "S2", 5)
	if got := lastUniqueNewPrice(t, batches); got != 5 {
		t.Fatalf("S2 new price = %v, want 5", got)
	}
	if lastUniqueOldPrice(t, batches) != nil {
		t.Fatal("S2 should have no old")
	}

	sendViewUniqueMD(t, engine, "S1", 101)
	if got := lastUniqueNewPrice(t, batches); got != 101 {
		t.Fatalf("S1@101 new price = %v, want 101", got)
	}
	if got := lastUniqueOldPrice(t, batches); got != 100.0 {
		t.Fatalf("S1@101 old price = %v, want 100", got)
	}

	sendViewUniqueMD(t, engine, "S1", 102)
	if got := lastUniqueNewPrice(t, batches); got != 102 {
		t.Fatalf("S1@102 new price = %v, want 102", got)
	}
	if got := lastUniqueOldPrice(t, batches); got != 101.0 {
		t.Fatalf("S1@102 old price = %v, want 101", got)
	}

	sendViewUniqueMD(t, engine, "S2", 6)
	if got := lastUniqueNewPrice(t, batches); got != 6 {
		t.Fatalf("S2@6 new price = %v, want 6", got)
	}
	if got := lastUniqueOldPrice(t, batches); got != 5.0 {
		t.Fatalf("S2@6 old price = %v, want 5", got)
	}
}

// TestViewUniqueSceneTwoMatchesEsper covers ViewLastUniqueSceneTwo:
// unique(symbol, feed) uses a composite key.
func TestViewUniqueSceneTwoMatchesEsper(t *testing.T) {
	env := newViewUniqueMDEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	compositeKey := Concat(
		Field[viewUniqueMDBean, string]("symbol"),
		Concat(Literal("|"), Field[viewUniqueMDBean, string]("feed")))
	source := From[viewUniqueMDBean](env, "SupportMarketDataBean").
		Window(Unique(compositeKey))
	plan, err := env.Build(Select(source,
		Alias("symbol", Field[viewUniqueMDBean, string]("symbol")),
		Alias("feed", Field[viewUniqueMDBean, string]("feed")),
		Alias("price", Field[viewUniqueMDBean, float64]("price")),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	stmt := dep.Statements()[0]
	var batches []ResultBatch
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	hasOld := func() bool {
		return len(batches) > 0 && len(batches[len(batches)-1].Old) > 0
	}

	sendViewUniqueMDFeed(t, engine, "S1", "F1", 100)
	if hasOld() {
		t.Fatal("S1/F1 should have no old")
	}
	sendViewUniqueMDFeed(t, engine, "S2", "F1", 5)
	if hasOld() {
		t.Fatal("S2/F1 should have no old")
	}
	sendViewUniqueMDFeed(t, engine, "S1", "F1", 101)
	if !hasOld() {
		t.Fatal("S1/F1@101 should have old")
	}
	sendViewUniqueMDFeed(t, engine, "S1", "F2", 6)
	if hasOld() {
		t.Fatal("S1/F2 should have no old (new composite key)")
	}
}

// TestViewUniqueExpressionParameterMatchesEsper covers
// ViewUniqueExpressionParameter: unique(abs(intPrimitive)).
func TestViewUniqueExpressionParameterMatchesEsper(t *testing.T) {
	env := newViewUniqueMDEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	absKey := Func1[int, int]("abs", func(v int) int {
		if v < 0 {
			return -v
		}
		return v
	}, Field[viewParityBean, int]("intPrimitive"))
	source := From[viewParityBean](env, "SupportBean").
		Window(Unique(absKey))
	plan, err := env.Build(Select(source,
		Alias("theString", Field[viewParityBean, string]("theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	stmt := dep.Statements()[0]
	if _, err := stmt.Subscribe(func(_ context.Context, _ ResultBatch) error { return nil }); err != nil {
		t.Fatal(err)
	}

	sendViewBean(t, engine, "E1", 10)
	sendViewBean(t, engine, "E2", -10)
	sendViewBean(t, engine, "E3", -5)
	sendViewBean(t, engine, "E4", 5)

	snap, err := stmt.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	strs := map[string]bool{}
	for _, row := range snap.Results() {
		strs[row.Get("theString").Any().(string)] = true
	}
	if !strs["E2"] || !strs["E4"] || len(strs) != 2 {
		t.Fatalf("snapshot = %v, want {E2, E4}", strs)
	}
}

// TestViewUniqueTwoWindowsMatchesEsper covers ViewUniqueTwoWindows:
// two statements with independent unique windows.
func TestViewUniqueTwoWindowsMatchesEsper(t *testing.T) {
	env := newViewUniqueMDEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source0 := From[viewParityBean](env, "SupportBean").
		Window(Unique(Literal(1)))
	plan0, err := env.Build(source0.Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	dep0, err := engine.Deploy(context.Background(), plan0)
	if err != nil {
		t.Fatal(err)
	}
	stmt0 := dep0.Statements()[0]
	var batch0 []ResultBatch
	if _, err := stmt0.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batch0 = append(batch0, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendViewBean(t, engine, "E1", 1)

	source1 := From[viewParityBean](env, "SupportBean").
		Window(Unique(Literal(1)))
	plan1, err := env.Build(source1.Query(StatementName("s1"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	dep1, err := engine.Deploy(context.Background(), plan1)
	if err != nil {
		t.Fatal(err)
	}
	stmt1 := dep1.Statements()[0]
	var batch1 []ResultBatch
	if _, err := stmt1.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batch1 = append(batch1, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendViewBean(t, engine, "E2", 2)

	if len(batch0) < 2 || len(batch0[len(batch0)-1].Old) == 0 {
		t.Fatal("s0 should have old event when E2 replaces E1")
	}
	if len(batch1) == 0 || len(batch1[len(batch1)-1].New) == 0 {
		t.Fatal("s1 should have new event for E2")
	}
	if len(batch1[len(batch1)-1].Old) > 0 {
		t.Fatal("s1 should have no old event for E2")
	}
}
