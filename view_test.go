package esper

import (
	"context"
	"testing"
	"time"
)

func deployViewTest[T any](t *testing.T, env *Environment, engine *Engine, stream Stream[T], name string) (*Statement, *[]ResultBatch) {
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

func TestLengthBatchWindowEmitsAtBatchBoundary(t *testing.T) {
	env, engine := newRuntimeTest(t)
	_, batches := deployViewTest(t, env, engine, From[runtimeTestTrade](env, "Trade").Window(LengthBatch(2)), "length-batch")
	for _, trade := range []runtimeTestTrade{{Symbol: "A"}, {Symbol: "B"}, {Symbol: "C"}, {Symbol: "D"}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(*batches) != 2 {
		t.Fatalf("got %d batches, want 2", len(*batches))
	}
	if len((*batches)[0].New) != 2 || len((*batches)[0].Old) != 0 {
		t.Fatalf("first batch = %#v", (*batches)[0])
	}
	if len((*batches)[1].New) != 2 || len((*batches)[1].Old) != 2 {
		t.Fatalf("second batch = %#v", (*batches)[1])
	}
}

func TestTimeBatchWindowFlushesOnVirtualTime(t *testing.T) {
	env, engine := newRuntimeTest(t)
	_, batches := deployViewTest(t, env, engine, From[runtimeTestTrade](env, "Trade").Window(TimeBatch(time.Second)), "time-batch")
	for _, trade := range []runtimeTestTrade{{Symbol: "A"}, {Symbol: "B"}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(*batches) != 0 {
		t.Fatalf("time batch emitted before clock advance: %#v", *batches)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 1 || len((*batches)[0].New) != 2 {
		t.Fatalf("first time batch = %#v", *batches)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "C"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 2 || len((*batches)[1].New) != 1 || len((*batches)[1].Old) != 2 {
		t.Fatalf("second time batch = %#v", *batches)
	}
}

func TestLastAndUniqueWindowsProduceReplacementOldStream(t *testing.T) {
	env, engine := newRuntimeTest(t)
	_, lastBatches := deployViewTest(t, env, engine, From[runtimeTestTrade](env, "Trade").Window(LastEvent()), "last-event")
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B"}); err != nil {
		t.Fatal(err)
	}
	if len(*lastBatches) != 2 || len((*lastBatches)[1].Old) != 1 {
		t.Fatalf("last-event batches = %#v", *lastBatches)
	}

	key := Field[runtimeTestTrade, string]("symbol")
	_, uniqueBatches := deployViewTest(t, env, engine, From[runtimeTestTrade](env, "Trade").Window(Unique(key)), "unique")
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if len(*uniqueBatches) != 2 || len((*uniqueBatches)[1].Old) != 1 {
		t.Fatalf("unique batches = %#v", *uniqueBatches)
	}
}

func TestUniqueWindowSnapshotAndNestedHistory(t *testing.T) {
	env, engine := newRuntimeTest(t)
	key := Field[runtimeTestTrade, string]("symbol")
	stream := From[runtimeTestTrade](env, "Trade").Window(Unique(key))
	plan, err := env.Build(stream.Query(StatementName("unique-snapshot")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, trade := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "B", Price: 2}, {Symbol: "A", Price: 3}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	results := snapshot.Results()
	if len(results) != 2 {
		t.Fatalf("unique snapshot length = %d, want 2", len(results))
	}
	first, ok := results[0].Event()
	if !ok || first.Underlying().(runtimeTestTrade).Price != 3 {
		t.Fatalf("unique snapshot first event = %#v", results[0])
	}
	second, ok := results[1].Event()
	if !ok || second.Underlying().(runtimeTestTrade).Price != 2 {
		t.Fatalf("unique snapshot second event = %#v", results[1])
	}

	nestedPlan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(GroupWindow(key, Unique(key))).Query(StatementName("group-unique-snapshot")))
	if err != nil {
		t.Fatal(err)
	}
	nestedDeployment, err := engine.Deploy(context.Background(), nestedPlan)
	if err != nil {
		t.Fatal(err)
	}
	for _, trade := range []runtimeTestTrade{{Symbol: "A", Price: 4}, {Symbol: "B", Price: 5}, {Symbol: "A", Price: 6}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	nestedSnapshot, err := nestedDeployment.Statements()[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(nestedSnapshot.Results()) != 2 {
		t.Fatalf("group unique snapshot = %#v", nestedSnapshot.Results())
	}
}

func TestMultiKeyUniqueWindowsUseAllKeyValues(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	_, batches := deployViewTest(t, env, engine, From[runtimeTestTrade](env, "Trade").Window(UniqueBy(symbol, price)), "multi-key-unique")
	for _, trade := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "A", Price: 2}, {Symbol: "A", Price: 1}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(*batches) != 3 || len((*batches)[0].Old) != 0 || len((*batches)[1].Old) != 0 || len((*batches)[2].Old) != 1 {
		t.Fatalf("multi-key unique batches = %#v", *batches)
	}

	_, firstBatches := deployViewTest(t, env, engine, From[runtimeTestTrade](env, "Trade").Window(FirstUniqueBy(symbol, price)), "multi-key-first-unique")
	for _, trade := range []runtimeTestTrade{{Symbol: "B", Price: 1}, {Symbol: "B", Price: 1}, {Symbol: "B", Price: 2}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(*firstBatches) != 2 || len((*firstBatches)[0].New) != 1 || len((*firstBatches)[1].New) != 1 {
		t.Fatalf("multi-key first-unique batches = %#v", *firstBatches)
	}
}

func TestUniqueWindowUsesArrayKeyContent(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[groupArrayTrade](env, "GroupArrayTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	key := Field[groupArrayTrade, []int64]("coll")
	_, batches := deployViewTest(t, env, engine, From[groupArrayTrade](env, "GroupArrayTrade").Window(Unique(key)), "array-unique")
	for _, trade := range []groupArrayTrade{
		{ID: "A", Coll: []int64{1, 2}},
		{ID: "B", Coll: []int64{1}},
		{ID: "C", Coll: []int64{1, 2}},
	} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(*batches) != 3 || len((*batches)[0].Old) != 0 || len((*batches)[1].Old) != 0 || len((*batches)[2].Old) != 1 {
		t.Fatalf("array unique batches = %#v", *batches)
	}
	old, ok := (*batches)[2].Old[0].Event()
	if !ok || old.Underlying().(groupArrayTrade).ID != "A" {
		t.Fatalf("array unique replacement old = %#v", (*batches)[2].Old)
	}
}

func TestUniqueWindowUsesObjectAndTwoDimensionalArrayContent(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[objectArrayTrade](env, "ObjectArrayTrade"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[twoDimensionalArrayTrade](env, "TwoDimensionalArrayTrade"); err != nil {
		t.Fatal(err)
	}
	objectEngine := NewEngine(env)
	objectKey := Field[objectArrayTrade, []any]("arr")
	_, objectBatches := deployViewTest(t, env, objectEngine, From[objectArrayTrade](env, "ObjectArrayTrade").Window(Unique(objectKey)), "object-array-unique")
	objectEvents := []objectArrayTrade{
		{ID: "O1", Arr: []any{"A", "B"}},
		{ID: "O2", Arr: []any{"A"}},
		{ID: "O3", Arr: []any{"A", "B"}},
	}
	for _, event := range objectEvents {
		if err := objectEngine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*objectBatches) != 3 || len((*objectBatches)[2].Old) != 1 {
		t.Fatalf("object array unique batches = %#v", *objectBatches)
	}
	oldObject, ok := (*objectBatches)[2].Old[0].Event()
	if !ok || oldObject.Underlying().(objectArrayTrade).ID != "O1" {
		t.Fatalf("object array replacement old = %#v", (*objectBatches)[2].Old)
	}

	twoDimKey := Field[twoDimensionalArrayTrade, [][]int64]("matrix")
	twoDimEngine := NewEngine(env)
	_, twoDimBatches := deployViewTest(t, env, twoDimEngine, From[twoDimensionalArrayTrade](env, "TwoDimensionalArrayTrade").Window(Unique(twoDimKey)), "two-dimensional-array-unique")
	twoDimEvents := []twoDimensionalArrayTrade{
		{ID: "D1", Matrix: [][]int64{{1}, {2}}},
		{ID: "D2", Matrix: [][]int64{{1}, {2}}},
		{ID: "D3", Matrix: [][]int64{{2}, {1}}},
		{ID: "D4", Matrix: [][]int64{{1, 2}}},
		{ID: "D5", Matrix: [][]int64{{}, {1, 2}}},
		{ID: "D6", Matrix: [][]int64{{2}, {1}}},
	}
	for _, event := range twoDimEvents {
		if err := twoDimEngine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*twoDimBatches) != len(twoDimEvents) || len((*twoDimBatches)[1].Old) != 1 || len((*twoDimBatches)[5].Old) != 1 {
		t.Fatalf("two-dimensional array unique batches = %#v", *twoDimBatches)
	}
	oldFirst, ok := (*twoDimBatches)[1].Old[0].Event()
	if !ok || oldFirst.Underlying().(twoDimensionalArrayTrade).ID != "D1" {
		t.Fatalf("two-dimensional array first replacement old = %#v", (*twoDimBatches)[1].Old)
	}
	oldSecond, ok := (*twoDimBatches)[5].Old[0].Event()
	if !ok || oldSecond.Underlying().(twoDimensionalArrayTrade).ID != "D3" {
		t.Fatalf("two-dimensional array second replacement old = %#v", (*twoDimBatches)[5].Old)
	}
}

func TestRankWindowUsesArrayUniqueKeyContent(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[rankArrayTrade](env, "RankArrayTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	key := Field[rankArrayTrade, []int64]("coll")
	price := Field[rankArrayTrade, int64]("price")
	_, batches := deployViewTest(t, env, engine, From[rankArrayTrade](env, "RankArrayTrade").Window(RankWindowBy(2, []Expr{key}, Descending(price))), "rank-array-unique")
	for _, event := range []rankArrayTrade{
		{ID: "R1", Coll: []int64{1, 2}, Price: 10},
		{ID: "R2", Coll: []int64{1}, Price: 9},
		{ID: "R3", Coll: []int64{1, 2}, Price: 11},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*batches) != 3 || len((*batches)[2].Old) != 1 {
		t.Fatalf("rank array unique batches = %#v", *batches)
	}
	old, ok := (*batches)[2].Old[0].Event()
	if !ok || old.Underlying().(rankArrayTrade).ID != "R1" {
		t.Fatalf("rank array replacement old = %#v", (*batches)[2].Old)
	}
}

func TestFirstWindowsStopAcceptingAfterTheirLimit(t *testing.T) {
	env, engine := newRuntimeTest(t)
	_, firstBatches := deployViewTest(t, env, engine, From[runtimeTestTrade](env, "Trade").Window(FirstLength(2)), "first-length")
	for _, trade := range []runtimeTestTrade{{Symbol: "A"}, {Symbol: "B"}, {Symbol: "C"}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(*firstBatches) != 2 || len((*firstBatches)[0].New) != 1 || len((*firstBatches)[1].New) != 1 {
		t.Fatalf("first-length batches = %#v", *firstBatches)
	}

	_, firstEventBatches := deployViewTest(t, env, engine, From[runtimeTestTrade](env, "Trade").Window(FirstEvent()), "first-event")
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "D"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E"}); err != nil {
		t.Fatal(err)
	}
	if len(*firstEventBatches) != 1 {
		t.Fatalf("first-event batches = %#v", *firstEventBatches)
	}
}

func TestExternallyTimedAndSortedWindows(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[externalTrade](env, "ExternalTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	timestamp := Field[externalTrade, int64]("timestamp")
	stream := From[externalTrade](env, "ExternalTrade").Window(ExternallyTimed(timestamp, time.Second))
	_, batches := deployViewTest(t, env, engine, stream, "external")
	for _, trade := range []externalTrade{{Symbol: "A", Timestamp: 1000}, {Symbol: "B", Timestamp: 1500}, {Symbol: "C", Timestamp: 2100}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(*batches) != 3 || len((*batches)[2].Old) != 1 {
		t.Fatalf("externally timed batches = %#v", *batches)
	}

	sortStream := From[externalTrade](env, "ExternalTrade").Window(SortWindow(2, Descending(Field[externalTrade, float64]("price"))))
	_, sortedBatches := deployViewTest(t, env, engine, sortStream, "sort")
	for _, trade := range []externalTrade{{Symbol: "L", Price: 1}, {Symbol: "H", Price: 3}, {Symbol: "M", Price: 2}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(*sortedBatches) != 3 || len((*sortedBatches)[2].Old) != 1 {
		t.Fatalf("sorted batches = %#v", *sortedBatches)
	}
}

func TestTimeOrderWindowUsesEventTimestampOrder(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[externalTrade](env, "ExternalTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	timestamp := Field[externalTrade, int64]("timestamp")
	stream := From[externalTrade](env, "ExternalTrade").Window(TimeOrder(timestamp, time.Second))
	_, batches := deployViewTest(t, env, engine, stream, "time-order")
	for _, trade := range []externalTrade{{Symbol: "late", Timestamp: 2000}, {Symbol: "early", Timestamp: 1500}, {Symbol: "new", Timestamp: 3001}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(*batches) != 3 || len((*batches)[2].Old) == 0 {
		t.Fatalf("time-order batches = %#v", *batches)
	}
}

func TestRankWindowByReplacesUniqueKeysAndEvictsWorst(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[externalTrade](env, "ExternalTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	symbol := Field[externalTrade, string]("symbol")
	price := Field[externalTrade, float64]("price")
	stream := From[externalTrade](env, "ExternalTrade").Window(RankWindowBy(3, []Expr{symbol}, Descending(price)))
	statement, batches := deployViewTest(t, env, engine, stream, "rank-unique")
	for _, trade := range []externalTrade{
		{Symbol: "A", Price: 10},
		{Symbol: "B", Price: 30},
		{Symbol: "A", Price: 50},
		{Symbol: "C", Price: 40},
		{Symbol: "B", Price: 45},
	} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(*batches) != 5 || len((*batches)[2].Old) != 1 || eventSymbol((*batches)[2].Old[0]) != "A" || len((*batches)[4].Old) != 1 || eventSymbol((*batches)[4].Old[0]) != "B" {
		t.Fatalf("rank replacement batches = %#v", *batches)
	}
	assertWindowSymbols(t, statement, []string{"A", "B", "C"})

	if err := engine.SendEvent(context.Background(), externalTrade{Symbol: "D", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 6 || len((*batches)[5].New) != 1 || len((*batches)[5].Old) != 1 || eventSymbol((*batches)[5].New[0]) != "D" || eventSymbol((*batches)[5].Old[0]) != "D" {
		t.Fatalf("rank out-of-space batch = %#v", (*batches)[5])
	}
	assertWindowSymbols(t, statement, []string{"A", "B", "C"})

	if err := engine.SendEvent(context.Background(), externalTrade{Symbol: "E", Price: 60}); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 7 || len((*batches)[6].Old) != 1 || eventSymbol((*batches)[6].Old[0]) != "C" {
		t.Fatalf("rank eviction batch = %#v", (*batches)[6])
	}
	assertWindowSymbols(t, statement, []string{"E", "A", "B"})
}

func TestSortAndRankWindowsPreserveEsperTieArrivalOrder(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[externalTrade](env, "ExternalTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	price := Field[externalTrade, float64]("price")
	sortStatement, sortBatches := deployViewTest(t, env, engine, From[externalTrade](env, "ExternalTrade").Window(SortWindow(3, Ascending(price))), "sort-ties")
	for _, trade := range []externalTrade{{Symbol: "A", Price: 1}, {Symbol: "B", Price: 1}, {Symbol: "C", Price: 2}, {Symbol: "D", Price: 1}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(*sortBatches) != 4 || len((*sortBatches)[3].Old) != 1 || eventSymbol((*sortBatches)[3].Old[0]) != "C" {
		t.Fatalf("sort tie eviction = %#v", (*sortBatches)[3])
	}
	assertWindowSymbols(t, sortStatement, []string{"D", "B", "A"})

	key := Field[externalTrade, string]("symbol")
	rankStatement, rankBatches := deployViewTest(t, env, engine, From[externalTrade](env, "ExternalTrade").Window(RankWindowBy(2, []Expr{key}, Ascending(price))), "rank-ties")
	for _, trade := range []externalTrade{{Symbol: "R1", Price: 1}, {Symbol: "R2", Price: 1}, {Symbol: "R3", Price: 1}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rankBatches) != 3 || len((*rankBatches)[2].Old) != 1 || eventSymbol((*rankBatches)[2].Old[0]) != "R1" {
		t.Fatalf("rank tie eviction = %#v", (*rankBatches)[2])
	}
	assertWindowSymbols(t, rankStatement, []string{"R2", "R3"})
}

func assertWindowSymbols(t *testing.T, statement *Statement, want []string) {
	t.Helper()
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	results := snapshot.Results()
	if len(results) != len(want) {
		t.Fatalf("window symbols = %#v, want %#v", results, want)
	}
	for index, result := range results {
		if got := eventSymbol(result); got != want[index] {
			t.Fatalf("window symbol[%d] = %q, want %q", index, got, want[index])
		}
	}
}

func TestTimeOrderWindowExpiresOnVirtualTimeAndRejectsLateEvents(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[externalTrade](env, "ExternalTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	timestamp := Field[externalTrade, int64]("timestamp")
	stream := From[externalTrade](env, "ExternalTrade").Window(TimeOrder(timestamp, 10*time.Second))
	_, batches := deployViewTest(t, env, engine, stream, "time-order-expiry")
	for _, trade := range []externalTrade{
		{Symbol: "E1", Timestamp: 3000},
		{Symbol: "E2", Timestamp: 2000},
		{Symbol: "E3", Timestamp: 3000},
		{Symbol: "E4", Timestamp: 2500},
	} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(12000).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 5 || len((*batches)[4].Old) != 1 || eventSymbol((*batches)[4].Old[0]) != "E2" {
		t.Fatalf("time-order first expiry = %#v", *batches)
	}
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(12500).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 6 || len((*batches)[5].Old) != 1 || eventSymbol((*batches)[5].Old[0]) != "E4" {
		t.Fatalf("time-order second expiry = %#v", *batches)
	}
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(13000).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 7 || len((*batches)[6].Old) != 2 || eventSymbol((*batches)[6].Old[0]) != "E1" || eventSymbol((*batches)[6].Old[1]) != "E3" {
		t.Fatalf("time-order final expiry = %#v", *batches)
	}
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(25000).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), externalTrade{Symbol: "E9", Timestamp: 15000}); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 8 || len((*batches)[7].New) != 1 || len((*batches)[7].Old) != 1 || eventSymbol((*batches)[7].Old[0]) != "E9" {
		t.Fatalf("time-order late event = %#v", *batches)
	}
}

func TestTimeToLiveAtWindowUsesAbsoluteEventExpiry(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[externalTrade](env, "ExternalTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	timestamp := Field[externalTrade, int64]("timestamp")
	stream := From[externalTrade](env, "ExternalTrade").Window(TimeToLiveAt(timestamp))
	_, batches := deployViewTest(t, env, engine, stream, "time-to-live-at")

	send := func(trade externalTrade) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	advance := func(milliseconds int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(milliseconds).UTC()); err != nil {
			t.Fatal(err)
		}
	}

	send(externalTrade{Symbol: "E1", Timestamp: 1000})
	send(externalTrade{Symbol: "E2", Timestamp: 500})
	advance(499)
	if len(*batches) != 2 {
		t.Fatalf("events expired before absolute timestamp: %#v", *batches)
	}
	advance(500)
	if len(*batches) != 3 || len((*batches)[2].Old) != 1 || eventSymbol((*batches)[2].Old[0]) != "E2" {
		t.Fatalf("first absolute expiry = %#v", *batches)
	}

	send(externalTrade{Symbol: "E3", Timestamp: 200})
	if len(*batches) != 4 || len((*batches)[3].New) != 1 || len((*batches)[3].Old) != 1 || eventSymbol((*batches)[3].Old[0]) != "E3" {
		t.Fatalf("already-expired insertion = %#v", *batches)
	}
	send(externalTrade{Symbol: "E4", Timestamp: 1200})
	send(externalTrade{Symbol: "E5", Timestamp: 1000})
	advance(999)
	if len(*batches) != 6 {
		t.Fatalf("events expired before second boundary: %#v", *batches)
	}
	advance(1000)
	if len(*batches) != 7 || len((*batches)[6].Old) != 2 || eventSymbol((*batches)[6].Old[0]) != "E1" || eventSymbol((*batches)[6].Old[1]) != "E5" {
		t.Fatalf("same-timestamp absolute expiry = %#v", *batches)
	}
	advance(1199)
	if len(*batches) != 7 {
		t.Fatalf("future event expired too early: %#v", *batches)
	}
	advance(1200)
	if len(*batches) != 8 || len((*batches)[7].Old) != 1 || eventSymbol((*batches)[7].Old[0]) != "E4" {
		t.Fatalf("final absolute expiry = %#v", *batches)
	}
	send(externalTrade{Symbol: "E6", Timestamp: 1200})
	if len(*batches) != 9 || len((*batches)[8].New) != 1 || len((*batches)[8].Old) != 1 || eventSymbol((*batches)[8].Old[0]) != "E6" {
		t.Fatalf("boundary insertion = %#v", *batches)
	}
}

func TestTimeToLiveAtWindowRejectsNonIntegralTimestamp(t *testing.T) {
	env, _ := newRuntimeTest(t)
	stream := From[runtimeTestTrade](env, "Trade").Window(TimeToLiveAt(Field[runtimeTestTrade, float64]("price")))
	if _, err := env.Build(stream.Query(StatementName("invalid-time-to-live-at"))); err == nil {
		t.Fatal("expected non-integral time-to-live-at timestamp to be rejected")
	}
}

func eventSymbol(result Result) string {
	event, ok := result.Event()
	if !ok {
		return ""
	}
	trade, ok := event.Underlying().(externalTrade)
	if !ok {
		return ""
	}
	return trade.Symbol
}

type externalTrade struct {
	Symbol    string  `esper:"symbol"`
	Price     float64 `esper:"price"`
	Timestamp int64   `esper:"timestamp"`
}

type objectArrayTrade struct {
	ID  string `esper:"id"`
	Arr []any  `esper:"arr"`
}

type twoDimensionalArrayTrade struct {
	ID     string    `esper:"id"`
	Matrix [][]int64 `esper:"matrix"`
}

type rankArrayTrade struct {
	ID    string  `esper:"id"`
	Coll  []int64 `esper:"coll"`
	Price int64   `esper:"price"`
}
