package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// deployAccumPlan deploys a built accum query with the old stream enabled and
// subscribes the standard batch collector.
func deployAccumPlan(t *testing.T, engine *Engine, plan Plan) (*Statement, *[]ResultBatch) {
	t.Helper()
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

// TestViewTimeAccumSceneOneParity covers ViewTimeAccumSceneOne: time_accum
// retains events until the expiry of the most recent event's deadline; the
// whole window then expires as one old batch. New events reschedule the
// window deadline (the newest event owns expiry).
func TestViewTimeAccumSceneOneParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 1000)
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeAccum(10 * time.Second))
	_, batches := deployViewMarketData(t, env, engine, source, "s0")

	advanceViewTime(t, engine, 11000)
	if len(*batches) != 0 {
		t.Fatalf("t=11000 invoked listener: %#v", *batches)
	}

	sendViewMarketData(t, engine, "E1", 1)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 || len((*batches)[0].Old) != 0 {
		t.Fatalf("E1: batches = %#v", *batches)
	}

	advanceViewTime(t, engine, 15000)
	sendViewMarketData(t, engine, "E2", 2)
	advanceViewTime(t, engine, 15000)
	sendViewMarketData(t, engine, "E3", 3)
	advanceViewTime(t, engine, 24000)
	sendViewMarketData(t, engine, "E4", 4)

	// E4 arrived at t=24000, so expiry moves to t=34000.
	advanceViewTime(t, engine, 33999)
	if len(*batches) != 4 {
		t.Fatalf("t=33999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 34000)
	if len(*batches) != 5 || len((*batches)[4].New) != 0 || len((*batches)[4].Old) != 4 {
		t.Fatalf("t=34000: batch = %#v", (*batches)[4])
	}
	want := []string{"E1", "E2", "E3", "E4"}
	for i, sym := range want {
		if got := (*batches)[4].Old[i].Get("symbol").Any(); got != sym {
			t.Fatalf("old[%d] = %v, want %s", i, got, sym)
		}
	}

	// No further expiry without events.
	advanceViewTime(t, engine, 51000)
	if len(*batches) != 5 {
		t.Fatalf("t=51000 invoked listener: %#v", *batches)
	}

	// E5/E6 arrive at 56000; their expiry owns the window at 66000.
	advanceViewTime(t, engine, 56000)
	sendViewMarketData(t, engine, "E5", 5)
	sendViewMarketData(t, engine, "E6", 6)
	advanceViewTime(t, engine, 65999)
	if len(*batches) != 7 {
		t.Fatalf("t=65999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 66000)
	if len(*batches) != 8 || len((*batches)[7].New) != 0 || len((*batches)[7].Old) != 2 {
		t.Fatalf("t=66000: batch = %#v", (*batches)[7])
	}
	if got := (*batches)[7].Old[0].Get("symbol").Any(); got != "E5" {
		t.Fatalf("old[0] = %v, want E5", got)
	}
	if got := (*batches)[7].Old[1].Get("symbol").Any(); got != "E6" {
		t.Fatalf("old[1] = %v, want E6", got)
	}

	// Next window: E7 at 76000 (after a quiet gap), E8 at 85000.
	advanceViewTime(t, engine, 76000)
	sendViewMarketData(t, engine, "E7", 7)
	advanceViewTime(t, engine, 85000)
	sendViewMarketData(t, engine, "E8", 8)
	advanceViewTime(t, engine, 94999)
	if len(*batches) != 10 {
		t.Fatalf("t=94999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 95000)
	if len(*batches) != 11 || len((*batches)[10].New) != 0 || len((*batches)[10].Old) != 2 {
		t.Fatalf("t=95000: batch = %#v", (*batches)[10])
	}
	if got := (*batches)[10].Old[0].Get("symbol").Any(); got != "E7" {
		t.Fatalf("old[0] = %v, want E7", got)
	}
	if got := (*batches)[10].Old[1].Get("symbol").Any(); got != "E8" {
		t.Fatalf("old[1] = %v, want E8", got)
	}
}

// TestViewTimeAccumSceneTwoParity covers ViewTimeAccumSceneTwo: insert stream
// on arrival, iterator sees the whole accumulated window, and the old stream
// carries the entire window on expiry.
func TestViewTimeAccumSceneTwoParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 1000)
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeAccum(10 * time.Second))
	stmt, batches := deployViewMarketData(t, env, engine, source, "s0")

	if got := snapshotSymbols(t, stmt); len(got) != 0 {
		t.Fatalf("initial iterator = %v", got)
	}

	advanceViewTime(t, engine, 1000)
	sendViewMarketData(t, engine, "E1", 1)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 || (*batches)[0].New[0].Get("symbol").Any() != "E1" {
		t.Fatalf("E1: batches = %#v", *batches)
	}

	advanceViewTime(t, engine, 5000)
	sendViewMarketData(t, engine, "E2", 2)
	if len(*batches) != 2 || (*batches)[1].New[0].Get("symbol").Any() != "E2" {
		t.Fatalf("E2: batches = %#v", *batches)
	}

	advanceViewTime(t, engine, 14999)
	if len(*batches) != 2 {
		t.Fatalf("t=14999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 15000)
	if len(*batches) != 3 || len((*batches)[2].New) != 0 || len((*batches)[2].Old) != 2 {
		t.Fatalf("t=15000: batch = %#v", (*batches)[2])
	}
	if got := snapshotSymbols(t, stmt); len(got) != 0 {
		t.Fatalf("iterator after expiry = %v", got)
	}

	advanceViewTime(t, engine, 31000)
	sendViewMarketData(t, engine, "E3", 3)
	sendViewMarketData(t, engine, "E4", 4)
	if len(*batches) != 5 {
		t.Fatalf("E3/E4: batches = %#v", *batches)
	}

	advanceViewTime(t, engine, 40999)
	if len(*batches) != 5 {
		t.Fatalf("t=40999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 41000)
	if len(*batches) != 6 || len((*batches)[5].Old) != 2 {
		t.Fatalf("t=41000: batch = %#v", (*batches)[5])
	}

	sendViewMarketData(t, engine, "E5", 5)
	if len(*batches) != 7 || (*batches)[6].New[0].Get("symbol").Any() != "E5" {
		t.Fatalf("E5: batches = %#v", *batches)
	}
	advanceViewTime(t, engine, 41000)
	sendViewMarketData(t, engine, "E6", 6)
	advanceViewTime(t, engine, 49000)
	sendViewMarketData(t, engine, "E7", 7)
	advanceViewTime(t, engine, 59000)
	if len(*batches) != 10 || len((*batches)[9].New) != 0 || len((*batches)[9].Old) != 3 {
		t.Fatalf("t=59000: batch = %#v", (*batches)[9])
	}
	for i, sym := range []string{"E5", "E6", "E7"} {
		if got := (*batches)[9].Old[i].Get("symbol").Any(); got != sym {
			t.Fatalf("old[%d] = %v, want %s", i, got, sym)
		}
	}
}

// TestViewTimeAccumSceneThreeParity covers ViewTimeAccumSceneThree: the
// support-bean shape with iterator assertions at each milestone and a quiet
// final stretch after the window expires.
func TestViewTimeAccumSceneThreeParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 1000)
	source := From[viewParityBean](env, "SupportBean").Window(TimeAccum(10 * time.Second))
	stmt, batches := deployViewParity(t, env, engine, source, "s0")

	assertIter := func(want ...string) {
		t.Helper()
		got := snapshotStrings(t, stmt)
		if len(got) != len(want) {
			t.Fatalf("iterator = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("iterator[%d] = %v, want %s", i, got[i], want[i])
			}
		}
	}
	assertIter()

	sendViewBean(t, engine, "E1", 1)
	assertIter("E1")

	advanceViewTime(t, engine, 5000)
	sendViewBean(t, engine, "E2", 2)
	assertIter("E1", "E2")

	advanceViewTime(t, engine, 14999)
	if len(*batches) != 2 {
		t.Fatalf("t=14999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 15000)
	if len(*batches) != 3 || len((*batches)[2].New) != 0 || len((*batches)[2].Old) != 2 {
		t.Fatalf("t=15000: batch = %#v", (*batches)[2])
	}
	assertIter()

	advanceViewTime(t, engine, 18000)
	sendViewBean(t, engine, "E3", 3)
	sendViewBean(t, engine, "E4", 4)
	assertIter("E3", "E4")
	advanceViewTime(t, engine, 19000)
	sendViewBean(t, engine, "E5", 5)
	advanceViewTime(t, engine, 28999)
	if len(*batches) != 6 {
		t.Fatalf("t=28999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 29000)
	if len(*batches) != 7 || len((*batches)[6].New) != 0 || len((*batches)[6].Old) != 3 {
		t.Fatalf("t=29000: batch = %#v", (*batches)[6])
	}
	for i, sym := range []string{"E3", "E4", "E5"} {
		if got := (*batches)[6].Old[i].Get("theString").Any(); got != sym {
			t.Fatalf("old[%d] = %v, want %s", i, got, sym)
		}
	}
	assertIter()

	advanceViewTime(t, engine, 39000)
	advanceViewTime(t, engine, 99000)
	if len(*batches) != 7 {
		t.Fatalf("quiet stretch invoked listener: %#v", *batches)
	}
}

func snapshotStrings(t *testing.T, stmt *Statement) []string {
	t.Helper()
	result, err := stmt.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	values := make([]string, 0, len(result.Results()))
	for _, row := range result.Results() {
		values = append(values, row.Get("theString").Any().(string))
	}
	return values
}

// TestViewTimeAccumRStreamParity covers ViewTimeAccumRStream: with only the
// remove stream selected, window expiry still delivers the batch (as new data
// in rstream projection).
func TestViewTimeAccumRStreamParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 1000)
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeAccum(10 * time.Second))
	plan, err := env.Build(source.Query(StatementName("s0"), WithRemoveStreamOnly()))
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

	advanceViewTime(t, engine, 11000)
	if len(*batches) != 0 {
		t.Fatalf("t=11000 invoked listener: %#v", *batches)
	}

	sendViewMarketData(t, engine, "E1", 1)
	sendViewMarketData(t, engine, "E2", 2)
	sendViewMarketData(t, engine, "E3", 3)
	if len(*batches) != 0 {
		t.Fatalf("arrivals invoked listener: %#v", *batches)
	}

	advanceViewTime(t, engine, 21000)
	if len(*batches) != 1 {
		t.Fatalf("t=21000: batches = %d, want 1", len(*batches))
	}
	if len((*batches)[0].New) != 3 {
		t.Fatalf("rstream new = %#v, want 3 events", (*batches)[0].New)
	}
	for i, sym := range []string{"E1", "E2", "E3"} {
		if got := (*batches)[0].New[i].Get("symbol").Any(); got != sym {
			t.Fatalf("new[%d] = %v, want %s", i, got, sym)
		}
	}
}

// TestViewTimeAccumPreviousAndPriorSceneOneParity covers
// ViewTimeAccumPreviousAndPriorSceneOne: prev(1, price) and prior(1, price)
// (Go Prev(1, …) and Prior(0, …)) over time_accum(10 sec). Arriving rows carry
// the window and stream history; the expiry old batch keeps each row's stream
// prior (through the row itself) while window history is null for leaving rows.
func TestViewTimeAccumPreviousAndPriorSceneOneParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 1000)
	price := Field[viewUniqueMarketData, float64]("price")
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeAccum(10 * time.Second))
	plan, err := env.Build(Select(source,
		Alias("price", price),
		Alias("prevPrice", Prev[float64](1, price)),
		Alias("priorPrice", Prior[float64](0, price)), // Java prior(1, price)
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	_, batches := deployAccumPlan(t, engine, plan)

	assertAccumRow := func(row Result, wantPrice float64, wantPrev, wantPrior any, where string) {
		t.Helper()
		if got := row.Get("price").Any(); got != wantPrice {
			t.Fatalf("%s price = %v, want %v", where, got, wantPrice)
		}
		check := func(name string, want any) {
			t.Helper()
			got := row.Get(name)
			if want == nil {
				if got.IsPresent() {
					t.Fatalf("%s %s = %v, want null", where, name, got)
				}
				return
			}
			if !got.IsPresent() || got.Any() != want {
				t.Fatalf("%s %s = %v, want %v", where, name, got, want)
			}
		}
		check("prevPrice", wantPrev)
		check("priorPrice", wantPrior)
	}

	// E5 arrives alone at t=20000: no window or stream history.
	advanceViewTime(t, engine, 20000)
	sendViewMarketData(t, engine, "S5", 5)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 || len((*batches)[0].Old) != 0 {
		t.Fatalf("E5: batches = %#v", *batches)
	}
	assertAccumRow((*batches)[0].New[0], 5, nil, nil, "E5 new")

	// E6 at t=25000: prev and prior both see E5.
	advanceViewTime(t, engine, 25000)
	sendViewMarketData(t, engine, "S6", 6)
	assertAccumRow((*batches)[1].New[0], 6, 5.0, 5.0, "E6 new")

	// E7 at t=34000: prev and prior both see E6.
	advanceViewTime(t, engine, 34000)
	sendViewMarketData(t, engine, "S7", 7)
	assertAccumRow((*batches)[2].New[0], 7, 6.0, 6.0, "E7 new")

	// The accumulation deadline of E7 expires at t=44000, ms-exact.
	advanceViewTime(t, engine, 43999)
	if len(*batches) != 3 {
		t.Fatalf("t=43999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 44000)
	if len(*batches) != 4 || len((*batches)[3].New) != 0 || len((*batches)[3].Old) != 3 {
		t.Fatalf("t=44000: batch = %#v", (*batches)[3])
	}
	assertAccumRow((*batches)[3].Old[0], 5, nil, nil, "E5 old")
	assertAccumRow((*batches)[3].Old[1], 6, nil, 5.0, "E6 old")
	assertAccumRow((*batches)[3].Old[2], 7, nil, 6.0, "E7 old")
}

// TestViewTimeAccumPreviousAndPriorSceneTwoParity covers
// ViewTimeAccumPreviousAndPriorSceneTwo: the full window-history surface
// prevtail/prevcount/prevwindow over time_accum(10 sec). On the expiry old
// batch only the stream prior survives; every window-history output is null.
func TestViewTimeAccumPreviousAndPriorSceneTwoParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 1000)
	symbol := Field[viewUniqueMarketData, string]("symbol")
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeAccum(10 * time.Second))
	priceField := Field[viewUniqueMarketData, float64]("price")
	plan, err := env.Build(Select(source,
		Alias("symbol", symbol),
		Alias("price", priceField),
		Alias("prevPrice", Prev[float64](1, priceField)),
		Alias("priorPrice", Prior[float64](0, priceField)), // Java prior(1, price)
		Alias("prevtailPrice", PrevTail[float64](0, priceField)),
		Alias("prevCountPrice", PrevCount[float64](priceField)), // Java prevcount(price)
		Alias("prevWindowPrice", PrevWindow[float64](priceField)),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	_, batches := deployAccumPlan(t, engine, plan)

	assertAccumRow := func(row Result, wantPrice float64, wantPrev, wantPrior, wantTail, wantCount, wantWindow any, where string) {
		t.Helper()
		if got := row.Get("price").Any(); got != wantPrice {
			t.Fatalf("%s price = %v, want %v", where, got, wantPrice)
		}
		check := func(name string, want any, equal func(got, want any) bool) {
			t.Helper()
			got := row.Get(name)
			if want == nil {
				if got.IsPresent() {
					t.Fatalf("%s %s = %v, want null", where, name, got)
				}
				return
			}
			if !got.IsPresent() || (equal != nil && !equal(got.Any(), want)) {
				t.Fatalf("%s %s = %v, want %v", where, name, got, want)
			}
		}
		check("prevPrice", wantPrev, nil)
		check("priorPrice", wantPrior, nil)
		check("prevtailPrice", wantTail, nil)
		check("prevCountPrice", wantCount, nil)
		check("prevWindowPrice", wantWindow, func(got, want any) bool {
			return reflect.DeepEqual(got, want)
		})
	}

	// E1 arrives alone at t=1000.
	sendViewMarketData(t, engine, "S1", 10)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 {
		t.Fatalf("E1: batches = %#v", *batches)
	}
	assertAccumRow((*batches)[0].New[0], 10, nil, nil, 10.0, int64(1), []float64{10}, "E1 new")

	// E2 at t=5000: prevwindow is newest-first [20 10].
	advanceViewTime(t, engine, 5000)
	sendViewMarketData(t, engine, "S1", 20)
	assertAccumRow((*batches)[1].New[0], 20, 10.0, 10.0, 10.0, int64(2), []float64{20, 10}, "E2 new")

	// E3 at t=10000: the third accumulates; tail stays at the oldest row.
	advanceViewTime(t, engine, 10000)
	sendViewMarketData(t, engine, "S2", 30)
	assertAccumRow((*batches)[2].New[0], 30, 20.0, 20.0, 10.0, int64(3), []float64{30, 20, 10}, "E3 new")

	// The ungrouped accum flushes the whole window at t=20000 (E3 + 10 sec).
	advanceViewTime(t, engine, 19999)
	if len(*batches) != 3 {
		t.Fatalf("t=19999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 20000)
	if len(*batches) != 4 || len((*batches)[3].New) != 0 || len((*batches)[3].Old) != 3 {
		t.Fatalf("t=20000: batch = %#v", (*batches)[3])
	}
	assertAccumRow((*batches)[3].Old[0], 10, nil, nil, nil, nil, nil, "E1 old")
	assertAccumRow((*batches)[3].Old[1], 20, nil, 10.0, nil, nil, nil, "E2 old")
	assertAccumRow((*batches)[3].Old[2], 30, nil, 20.0, nil, nil, nil, "E3 old")
}

// TestViewTimeAccumMonthScopedParity covers ViewTimeAccumMonthScoped:
// time_accum(1 month) expires at the exact calendar-month boundary
// (2002-02-01T09:00 + 1 month = 2002-03-01T09:00, a 28-day Feb gap), silent
// one millisecond before the boundary.
func TestViewTimeAccumMonthScopedParity(t *testing.T) {
	env, initial := newViewUnionEnv(t)
	if err := initial.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(mustParseViewTimeWin(t, "2002-02-01T09:00:00.000")))
	defer func() { _ = engine.Close(context.Background()) }()
	statement, batches := deployViewTimeWinBeanRStream(t, env, engine, TimeAccumCalendar(0, 1, 0), "s0")
	send := func(theString string, at string) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), mustParseViewTimeWin(t, at)); err != nil {
			t.Fatal(err)
		}
		sendViewUnionBean(t, engine, viewUnionBean{TheString: theString})
	}
	send("E1", "2002-02-01T09:00:00.000")
	send("E2", "2002-02-01T09:00:00.000")

	boundary := mustParseViewTimeWin(t, "2002-03-01T09:00:00.000")
	if err := engine.AdvanceTime(context.Background(), boundary.Add(-time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 0 {
		t.Fatalf("pre-boundary output = %#v", *batches)
	}
	if err := engine.AdvanceTime(context.Background(), boundary); err != nil {
		t.Fatal(err)
	}
	if got := viewTimeWinRStreamStrings(batches); !reflect.DeepEqual(got, []string{"E1", "E2"}) {
		t.Fatalf("March 1 rstream = %v, want [E1 E2]", got)
	}
	if snapshot, err := statement.Snapshot(context.Background()); err != nil || len(snapshot.Results()) != 0 {
		t.Fatalf("final snapshot = %v, err = %v", snapshot.Results(), err)
	}
}

// TestViewTimeAccumSumParity covers ViewTimeAccumSum: ungrouped sum(price)
// over time_accum(10 sec) reports the running aggregate per arrival (old
// carries the pre-insert value) and the empty-window null aggregate at expiry
// (old carries the pre-removal value).
func TestViewTimeAccumSumParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 1000)
	price := Field[viewUniqueMarketData, float64]("price")
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeAccum(10 * time.Second))
	plan, err := env.Build(source.Aggregate(
		Alias("sumPrice", Sum[float64](price)),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	_, batches := deployAccumPlan(t, engine, plan)

	assertSumRow := func(row Result, wantSum any, where string) {
		t.Helper()
		got := row.Get("sumPrice")
		if wantSum == nil {
			if got.IsPresent() {
				t.Fatalf("%s sumPrice = %v, want null", where, got)
			}
			return
		}
		if !got.IsPresent() || got.Any() != wantSum {
			t.Fatalf("%s sumPrice = %v, want %v", where, got, wantSum)
		}
	}

	advanceViewTime(t, engine, 20000)
	sendViewMarketData(t, engine, "S5", 5)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 || len((*batches)[0].Old) != 1 {
		t.Fatalf("E5: batches = %#v", *batches)
	}
	assertSumRow((*batches)[0].New[0], 5.0, "E5 new")
	assertSumRow((*batches)[0].Old[0], nil, "E5 old")

	advanceViewTime(t, engine, 25000)
	sendViewMarketData(t, engine, "S6", 6)
	assertSumRow((*batches)[1].New[0], 11.0, "E6 new")
	assertSumRow((*batches)[1].Old[0], 5.0, "E6 old")

	// E6's accum deadline expires at t=35000; the aggregate empties to null.
	advanceViewTime(t, engine, 34999)
	if len(*batches) != 2 {
		t.Fatalf("t=34999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 35000)
	if len(*batches) != 3 || len((*batches)[2].New) != 1 || len((*batches)[2].Old) != 1 {
		t.Fatalf("t=35000: batch = %#v", (*batches)[2])
	}
	assertSumRow((*batches)[2].New[0], nil, "expiry new")
	assertSumRow((*batches)[2].Old[0], 11.0, "expiry old")
}

// TestViewTimeAccumGroupedWindowParity covers ViewTimeAccumGroupedWindow:
// groupwin(symbol) gives each group an independent accum deadline; a new
// event reschedules only its own group, and each group's expiry flushes its
// accumulated events as one old batch in arrival order.
func TestViewTimeAccumGroupedWindowParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 1000)
	key := Field[viewUniqueMarketData, string]("symbol")
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(GroupWindow(key, TimeAccum(10*time.Second)))
	_, batches := deployViewMarketData(t, env, engine, source, "s0")

	// events[1]=S1/p1, [2]=S2/p2, [11]=S1/p11, [12]=S2/p12, [21]=S1/p21,
	// [32]=S2/p32 from the shared get100Events pool (price = index).
	type event struct {
		symbol string
		price  float64
	}
	sendAt := func(millis int64, event event) {
		t.Helper()
		advanceViewTime(t, engine, millis)
		sendViewMarketData(t, engine, event.symbol, event.price)
	}
	assertNew := func(index int, want event, where string) {
		t.Helper()
		batch := (*batches)[index]
		if len(batch.New) != 1 || len(batch.Old) != 0 {
			t.Fatalf("%s: batch = %#v", where, batch)
		}
		if got := batch.New[0].Get("symbol").Any(); got != want.symbol {
			t.Fatalf("%s new symbol = %v, want %s", where, got, want.symbol)
		}
		if got := batch.New[0].Get("price").Any(); got != want.price {
			t.Fatalf("%s new price = %v, want %v", where, got, want.price)
		}
	}
	assertOldBatch := func(index int, want []event, where string) {
		t.Helper()
		batch := (*batches)[index]
		if len(batch.New) != 0 || len(batch.Old) != len(want) {
			t.Fatalf("%s: batch = %#v, want %d old rows", where, batch, len(want))
		}
		for i, row := range batch.Old {
			if got := row.Get("symbol").Any(); got != want[i].symbol {
				t.Fatalf("%s old[%d] symbol = %v, want %s", where, i, got, want[i].symbol)
			}
			if got := row.Get("price").Any(); got != want[i].price {
				t.Fatalf("%s old[%d] price = %v, want %v", where, i, got, want[i].price)
			}
		}
	}

	sendAt(11000, event{"S1", 1})
	assertNew(0, event{"S1", 1}, "E1")
	sendAt(12000, event{"S2", 2})
	assertNew(1, event{"S2", 2}, "E2")
	sendAt(15000, event{"S1", 11})
	assertNew(2, event{"S1", 11}, "E11")
	sendAt(18000, event{"S2", 12})
	assertNew(3, event{"S2", 12}, "E12")
	sendAt(21000, event{"S1", 21})
	assertNew(4, event{"S1", 21}, "E21")

	// S2's deadline (E12 at 18000) expires its two rows at t=28000; S1 keeps
	// accumulating past its earlier individual deadlines.
	advanceViewTime(t, engine, 27999)
	if len(*batches) != 5 {
		t.Fatalf("t=27999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 28000)
	assertOldBatch(5, []event{{"S2", 2}, {"S2", 12}}, "S2 expiry")

	// E32 re-arms only the S2 group.
	sendAt(29000, event{"S2", 32})
	assertNew(6, event{"S2", 32}, "E32")

	// S1's deadline (E21 at 21000) flushes all three S1 rows at t=31000.
	advanceViewTime(t, engine, 31000)
	assertOldBatch(7, []event{{"S1", 1}, {"S1", 11}, {"S1", 21}}, "S1 expiry")

	// E32 expires on its own deadline at t=39000.
	advanceViewTime(t, engine, 38999)
	if len(*batches) != 8 {
		t.Fatalf("t=38999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 39000)
	assertOldBatch(8, []event{{"S2", 32}}, "S2 second expiry")

	advanceViewTime(t, engine, 50000)
	if len(*batches) != 9 {
		t.Fatalf("t=50000 invoked listener: %#v", *batches)
	}
}
