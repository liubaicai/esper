package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type viewTimeWinMarket struct {
	Symbol string  `esper:"symbol"`
	Volume int64   `esper:"volume"`
	Price  float64 `esper:"price"`
}

func newViewTimeWinMarketEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[viewTimeWinMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	return env, engine
}

func sendViewTimeWinMarket(t *testing.T, engine *Engine, symbol string, volume int64, price float64) {
	t.Helper()
	if err := engine.Send(context.Background(), "SupportMarketDataBean", viewTimeWinMarket{Symbol: symbol, Volume: volume, Price: price}); err != nil {
		t.Fatal(err)
	}
}

func advanceViewTimeWin(t *testing.T, engine *Engine, origin time.Time, milliseconds int64) {
	t.Helper()
	if err := engine.AdvanceTime(context.Background(), origin.Add(time.Duration(milliseconds)*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
}

func viewTimeWinRStreamStrings(batches *[]ResultBatch) []string {
	if len(*batches) == 0 {
		return nil
	}
	last := (*batches)[len(*batches)-1]
	// rstream reports removed events as new data (Java rstream semantics).
	values := make([]string, 0, len(last.New))
	for _, result := range last.New {
		if value, ok := result.Get("theString").Any().(string); ok {
			values = append(values, value)
		}
	}
	return values
}

// TestViewTimeWinJustSelectStarParity covers ViewTimeJustSelectStar:
// time(1 sec) with irstream select * and iterator order.
func TestViewTimeWinJustSelectStarParity(t *testing.T) {
	env, engine := newViewTimeWinMarketEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	origin := time.Unix(0, 0).UTC()
	engine = NewEngine(env, WithStartTime(origin))
	defer func() { _ = engine.Close(context.Background()) }()
	statement, batches := deployViewTimeWinMarket(t, env, engine,
		From[viewTimeWinMarket](env, "SupportMarketDataBean").Window(TimeWindow(time.Second)),
		"s0")
	advanceViewTimeWin(t, engine, origin, 500)
	sendViewTimeWinMarket(t, engine, "E1", 0, 0)
	if got := viewTimeWinSnapshotSymbols(t, statement); !reflect.DeepEqual(got, []string{"E1"}) {
		t.Fatalf("after E1 snapshot = %v", got)
	}
	advanceViewTimeWin(t, engine, origin, 600)
	sendViewTimeWinMarket(t, engine, "E2", 0, 0)
	if got := viewTimeWinSnapshotSymbols(t, statement); !reflect.DeepEqual(got, []string{"E1", "E2"}) {
		t.Fatalf("after E2 snapshot = %v", got)
	}
	advanceViewTimeWin(t, engine, origin, 1500)
	if got := viewTimeWinOldSymbols(batches); !reflect.DeepEqual(got, []string{"E1"}) {
		t.Fatalf("old at 1500 = %v", got)
	}
	advanceViewTimeWin(t, engine, origin, 1600)
	if got := viewTimeWinOldSymbols(batches); !reflect.DeepEqual(got, []string{"E2"}) {
		t.Fatalf("old at 1600 = %v", got)
	}
	advanceViewTimeWin(t, engine, origin, 2000)
	sendViewTimeWinMarket(t, engine, "E3", 0, 0)
	if got := viewTimeWinSnapshotSymbols(t, statement); !reflect.DeepEqual(got, []string{"E3"}) {
		t.Fatalf("after E3 snapshot = %v", got)
	}
}

// TestViewTimeWinSumParity covers ViewTimeSum: ungrouped sum(price) over
// time(30) with the window fully expiring at 35s.
func TestViewTimeWinSumParity(t *testing.T) {
	env, engine := newViewTimeWinMarketEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	origin := time.Unix(0, 0).UTC()
	engine = NewEngine(env, WithStartTime(origin))
	defer func() { _ = engine.Close(context.Background()) }()
	plan, err := env.Build(From[viewTimeWinMarket](env, "SupportMarketDataBean").Window(TimeWindow(30*time.Second)).Aggregate(
		Alias("symbol", Field[viewTimeWinMarket, string]("symbol")),
		Alias("volume", Field[viewTimeWinMarket, int64]("volume")),
		Alias("mySum", Sum[float64](Field[viewTimeWinMarket, float64]("price"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	assertViewTimeWinResultType(t, plan)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var sums []float64
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if result.Get("mySum").IsPresent() {
				sums = append(sums, result.Get("mySum").Any().(float64))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendViewTimeWinMarket(t, engine, "DELL", 10000, 51)
	sendViewTimeWinMarket(t, engine, "IBM", 20000, 52)
	sendViewTimeWinMarket(t, engine, "DELL", 40000, 45)
	if !reflect.DeepEqual(sums, []float64{51, 103, 148}) {
		t.Fatalf("sums before expiry = %v", sums)
	}
	advanceViewTimeWin(t, engine, origin, 35000)
	sendViewTimeWinMarket(t, engine, "IBM", 30000, 70)
	sendViewTimeWinMarket(t, engine, "DELL", 10000, 20)
	if !reflect.DeepEqual(sums, []float64{51, 103, 148, 70, 90}) {
		t.Fatalf("sums after expiry = %v", sums)
	}
}

// TestViewTimeWinSumGroupByParity covers ViewTimeSumGroupBy.
func TestViewTimeWinSumGroupByParity(t *testing.T) {
	env, engine := newViewTimeWinMarketEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	origin := time.Unix(0, 0).UTC()
	engine = NewEngine(env, WithStartTime(origin))
	defer func() { _ = engine.Close(context.Background()) }()
	symbol := Field[viewTimeWinMarket, string]("symbol")
	plan, err := env.Build(From[viewTimeWinMarket](env, "SupportMarketDataBean").Window(TimeWindow(30*time.Second)).GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("volume", Field[viewTimeWinMarket, int64]("volume")),
		Alias("mySum", Sum[float64](Field[viewTimeWinMarket, float64]("price"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	assertViewTimeWinResultType(t, plan)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []viewTimeWinSumRow
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			rows = append(rows, viewTimeWinSumRow{
				symbol: result.Get("symbol").Any().(string),
				sum:    result.Get("mySum").Any().(float64),
			})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendViewTimeWinMarket(t, engine, "DELL", 10000, 51)
	sendViewTimeWinMarket(t, engine, "IBM", 30000, 70)
	sendViewTimeWinMarket(t, engine, "DELL", 20000, 52)
	sendViewTimeWinMarket(t, engine, "IBM", 30000, 70)
	want := []viewTimeWinSumRow{{"DELL", 51}, {"IBM", 70}, {"DELL", 103}, {"IBM", 140}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("grouped rows = %v, want %v", rows, want)
	}
	advanceViewTimeWin(t, engine, origin, 35000)
	sendViewTimeWinMarket(t, engine, "DELL", 10000, 90)
	sendViewTimeWinMarket(t, engine, "IBM", 30000, 120)
	sendViewTimeWinMarket(t, engine, "DELL", 20000, 90)
	sendViewTimeWinMarket(t, engine, "IBM", 30000, 120)
	want = append(want,
		viewTimeWinSumRow{"DELL", 90},
		viewTimeWinSumRow{"IBM", 120},
		viewTimeWinSumRow{"DELL", 180},
		viewTimeWinSumRow{"IBM", 240},
	)
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("grouped rows after expiry = %v, want %v", rows, want)
	}
}

type viewTimeWinSumRow struct {
	symbol string
	sum    float64
}

// TestViewTimeWinSumWFilterParity covers ViewTimeSumWFilter: the filter runs
// before the time window (from-clause filter).
func TestViewTimeWinSumWFilterParity(t *testing.T) {
	env, engine := newViewTimeWinMarketEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	origin := time.Unix(0, 0).UTC()
	engine = NewEngine(env, WithStartTime(origin))
	defer func() { _ = engine.Close(context.Background()) }()
	symbol := Field[viewTimeWinMarket, string]("symbol")
	plan, err := env.Build(From[viewTimeWinMarket](env, "SupportMarketDataBean").
		Filter(Equal[string](symbol, Literal("IBM"))).
		Window(TimeWindow(30*time.Second)).
		Aggregate(
			Alias("symbol", symbol),
			Alias("volume", Field[viewTimeWinMarket, int64]("volume")),
			Alias("mySum", Sum[float64](Field[viewTimeWinMarket, float64]("price"))),
		).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	assertViewTimeWinResultType(t, plan)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var sums []float64
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if result.Get("mySum").IsPresent() {
				sums = append(sums, result.Get("mySum").Any().(float64))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendViewTimeWinMarket(t, engine, "IBM", 20000, 52)
	sendViewTimeWinMarket(t, engine, "IBM", 20000, 100)
	if !reflect.DeepEqual(sums, []float64{52, 152}) {
		t.Fatalf("filtered sums = %v", sums)
	}
	advanceViewTimeWin(t, engine, origin, 35000)
	sendViewTimeWinMarket(t, engine, "IBM", 20000, 252)
	sendViewTimeWinMarket(t, engine, "IBM", 20000, 100)
	if !reflect.DeepEqual(sums, []float64{52, 152, 252, 352}) {
		t.Fatalf("filtered sums after expiry = %v", sums)
	}
}

func assertViewTimeWinResultType(t *testing.T, plan Plan) {
	t.Helper()
	schema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("result schema missing")
	}
	fields := schema.Fields()
	if len(fields) != 3 {
		t.Fatalf("result fields = %d", len(fields))
	}
	byName := make(map[string]reflect.Type, len(fields))
	for _, field := range fields {
		byName[field.Name] = field.Type
	}
	if byName["symbol"] != reflect.TypeOf("") || byName["volume"] != reflect.TypeOf(int64(0)) || byName["mySum"] != reflect.TypeOf(float64(0)) {
		t.Fatalf("result types = %v", byName)
	}
}

// TestViewTimeWinMonthScopedParity covers ViewTimeWindowMonthScoped:
// calendar-month time window expiry at exact month boundaries.
func TestViewTimeWinMonthScopedParity(t *testing.T) {
	env, initial := newViewUnionEnv(t)
	if err := initial.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	start := mustParseViewTimeWin(t, "2002-02-01T09:00:00.000")
	engine := NewEngine(env, WithStartTime(start))
	defer func() { _ = engine.Close(context.Background()) }()
	statement, batches := deployViewTimeWinBeanRStream(t, env, engine, TimeWindowCalendar(0, 1, 0, 0), "s0")
	send := func(theString string, at string) {
		t.Helper()
		instant := mustParseViewTimeWin(t, at)
		if err := engine.AdvanceTime(context.Background(), instant); err != nil {
			t.Fatal(err)
		}
		sendViewUnionBean(t, engine, viewUnionBean{TheString: theString})
	}
	send("E1", "2002-02-01T09:00:00.000")
	send("E2", "2002-02-15T09:00:00.000")
	if err := engine.AdvanceTime(context.Background(), mustParseViewTimeWin(t, "2002-03-01T09:00:00.000").Add(-time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 0 {
		t.Fatalf("early March output = %#v", *batches)
	}
	if err := engine.AdvanceTime(context.Background(), mustParseViewTimeWin(t, "2002-03-01T09:00:00.000")); err != nil {
		t.Fatal(err)
	}
	if got := viewTimeWinRStreamStrings(batches); !reflect.DeepEqual(got, []string{"E1"}) {
		t.Fatalf("March 1 old = %v", got)
	}
	if err := engine.AdvanceTime(context.Background(), mustParseViewTimeWin(t, "2002-03-15T09:00:00.000").Add(-time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 1 {
		t.Fatalf("early March 15 output = %#v", *batches)
	}
	if err := engine.AdvanceTime(context.Background(), mustParseViewTimeWin(t, "2002-03-15T09:00:00.000")); err != nil {
		t.Fatal(err)
	}
	if got := viewTimeWinRStreamStrings(batches); !reflect.DeepEqual(got, []string{"E2"}) {
		t.Fatalf("March 15 old = %v", got)
	}
	if snapshot, err := statement.Snapshot(context.Background()); err != nil || len(snapshot.Results()) != 0 {
		t.Fatalf("final snapshot = %v, err=%v", snapshot.Results(), err)
	}
}

func mustParseViewTimeWin(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02T15:04:05.000", value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.UTC()
}

// TestViewTimeWinWPrevParity covers ViewTimeWindowWPrev: prev/prevtail/
// prevcount/prevwindow over a one-second time window.
func TestViewTimeWinWPrevParity(t *testing.T) {
	env, engine := newViewTimeWinMarketEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	origin := time.Unix(0, 0).UTC()
	engine = NewEngine(env, WithStartTime(origin))
	defer func() { _ = engine.Close(context.Background()) }()
	symbol := Field[viewTimeWinMarket, string]("symbol")
	plan, err := env.Build(Select(
		From[viewTimeWinMarket](env, "SupportMarketDataBean").Window(TimeWindow(time.Second)),
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
	var rows []map[string]any
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("prev result is not a row: %#v", result)
			}
			rows = append(rows, row.AsMap())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	advanceViewTimeWin(t, engine, origin, 500)
	sendViewTimeWinMarket(t, engine, "E1", 0, 0)
	sendViewTimeWinMarket(t, engine, "E2", 0, 0)
	sendViewTimeWinMarket(t, engine, "E3", 0, 0)
	if len(rows) != 3 {
		t.Fatalf("prev rows = %d", len(rows))
	}
	assertViewTimeWinPrevRow(t, rows[0], "E1", nil, "E1", int64(1), []string{"E1"})
	assertViewTimeWinPrevRow(t, rows[1], "E2", "E1", "E1", int64(2), []string{"E2", "E1"})
	assertViewTimeWinPrevRow(t, rows[2], "E3", "E2", "E1", int64(3), []string{"E3", "E2", "E1"})
	advanceViewTimeWin(t, engine, origin, 1200)
	sendViewTimeWinMarket(t, engine, "E4", 0, 0)
	sendViewTimeWinMarket(t, engine, "E5", 0, 0)
	assertViewTimeWinPrevRow(t, rows[3], "E4", "E3", "E1", int64(4), []string{"E4", "E3", "E2", "E1"})
	assertViewTimeWinPrevRow(t, rows[4], "E5", "E4", "E1", int64(5), []string{"E5", "E4", "E3", "E2", "E1"})
	advanceViewTimeWin(t, engine, origin, 1600)
	if len(batches) < 6 {
		t.Fatalf("expiry batch missing: %d batches", len(batches))
	}
	expiry := batches[len(batches)-1]
	oldSymbols := make([]string, 0, len(expiry.Old))
	for _, result := range expiry.Old {
		oldSymbols = append(oldSymbols, result.Get("symbol").Any().(string))
	}
	if !sameStringSet(oldSymbols, []string{"E1", "E2", "E3"}) {
		t.Fatalf("expired symbols = %v", oldSymbols)
	}
	sendViewTimeWinMarket(t, engine, "E6", 0, 0)
	if len(rows) != 6 || rows[5]["symbol"] != "E6" {
		t.Fatalf("E6 row = %v", rows)
	}
}

func assertViewTimeWinPrevRow(t *testing.T, row map[string]any, symbol string, prev1 any, prevtail string, count int64, window []string) {
	t.Helper()
	if row["symbol"] != symbol || row["prevtail"] != prevtail || row["prevCountSym"] != count {
		t.Fatalf("prev row = %v", row)
	}
	if prev1 == nil {
		if value, ok := row["prev1"]; ok && value != nil {
			t.Fatalf("prev1 = %v, want nil", value)
		}
	} else if row["prev1"] != prev1 {
		t.Fatalf("prev1 = %v, want %v", row["prev1"], prev1)
	}
	if got, ok := row["prevWindowSym"].([]string); !ok || !reflect.DeepEqual(got, window) {
		t.Fatalf("prevWindowSym = %#v, want %v", row["prevWindowSym"], window)
	}
}

// TestViewTimeWinPreparedStmtParity covers ViewTimeWindowPreparedStmt: the
// window duration comes from a deployment-time substitution parameter.
func TestViewTimeWinPreparedStmtParity(t *testing.T) {
	env, initial := newViewUnionEnv(t)
	if err := initial.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	origin := time.Unix(0, 0).UTC()
	engine := NewEngine(env, WithStartTime(origin))
	defer func() { _ = engine.Close(context.Background()) }()
	plan, err := env.Build(From[viewUnionBean](env, "SupportBean").Window(
		TimeWindowSeconds[float64](Parameter[float64]("seconds")),
	).Query(StatementName("s0"), WithRemoveStreamOnly()))
	if err != nil {
		t.Fatal(err)
	}
	s0, err := engine.DeployWithParameters(context.Background(), plan, ParameterValues{"seconds": 4.0})
	if err != nil {
		t.Fatal(err)
	}
	s1, err := engine.DeployWithParameters(context.Background(), plan, ParameterValues{"seconds": 3.0})
	if err != nil {
		t.Fatal(err)
	}
	runViewTimeWinExpiryAssertion(t, engine, origin, s0.Statements()[0], s1.Statements()[0])
}

// TestViewTimeWinVariableStmtParity covers ViewTimeWindowVariableStmt: the
// window duration comes from a variable (default 4 seconds for s0, set to 3
// seconds before s1 deploys).
func TestViewTimeWinVariableStmtParity(t *testing.T) {
	env, initial := newViewUnionEnv(t)
	if err := initial.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("TIME_WIN_ONE", 4.0); err != nil {
		t.Fatal(err)
	}
	origin := time.Unix(0, 0).UTC()
	engine := NewEngine(env, WithStartTime(origin))
	defer func() { _ = engine.Close(context.Background()) }()
	plan, err := env.Build(From[viewUnionBean](env, "SupportBean").Window(
		TimeWindowSeconds[float64](VariableRef[float64]("TIME_WIN_ONE")),
	).Query(StatementName("s0"), WithRemoveStreamOnly()))
	if err != nil {
		t.Fatal(err)
	}
	s0, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SetVariable(context.Background(), "TIME_WIN_ONE", 3.0); err != nil {
		t.Fatal(err)
	}
	s1, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	runViewTimeWinExpiryAssertion(t, engine, origin, s0.Statements()[0], s1.Statements()[0])
}

// TestViewTimeWinTimePeriodParity covers ViewTimeWindowTimePeriod:
// time(4 sec) and time(3000 milliseconds).
func TestViewTimeWinTimePeriodParity(t *testing.T) {
	env, initial := newViewUnionEnv(t)
	if err := initial.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	origin := time.Unix(0, 0).UTC()
	engine := NewEngine(env, WithStartTime(origin))
	defer func() { _ = engine.Close(context.Background()) }()
	s0, err := engine.Deploy(context.Background(), mustBuildViewTimeWinRStream(t, env, TimeWindow(4*time.Second), "s0"))
	if err != nil {
		t.Fatal(err)
	}
	s1, err := engine.Deploy(context.Background(), mustBuildViewTimeWinRStream(t, env, TimeWindow(3000*time.Millisecond), "s1"))
	if err != nil {
		t.Fatal(err)
	}
	runViewTimeWinExpiryAssertion(t, engine, origin, s0.Statements()[0], s1.Statements()[0])
}

// TestViewTimeWinVariableTimePeriodParity covers
// ViewTimeWindowVariableTimePeriodStmt: milliseconds vs minutes units on the
// same variable (s0 uses the default 4000 ms, s1 uses 0.05 minutes = 3000 ms).
func TestViewTimeWinVariableTimePeriodParity(t *testing.T) {
	env, initial := newViewUnionEnv(t)
	if err := initial.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("TIME_WIN_TWO", 4000.0); err != nil {
		t.Fatal(err)
	}
	origin := time.Unix(0, 0).UTC()
	engine := NewEngine(env, WithStartTime(origin))
	defer func() { _ = engine.Close(context.Background()) }()
	s0, err := engine.Deploy(context.Background(), mustBuildViewTimeWinRStream(t, env,
		TimeWindowMilliseconds[float64](VariableRef[float64]("TIME_WIN_TWO")), "s0"))
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SetVariable(context.Background(), "TIME_WIN_TWO", 0.05); err != nil {
		t.Fatal(err)
	}
	s1, err := engine.Deploy(context.Background(), mustBuildViewTimeWinRStream(t, env,
		TimeWindowMinutes[float64](VariableRef[float64]("TIME_WIN_TWO")), "s1"))
	if err != nil {
		t.Fatal(err)
	}
	runViewTimeWinExpiryAssertion(t, engine, origin, s0.Statements()[0], s1.Statements()[0])
}

func runViewTimeWinExpiryAssertion(t *testing.T, engine *Engine, origin time.Time, s0, s1 *Statement) {
	t.Helper()
	var s0Old, s1Old []string
	if _, err := s0.Subscribe(func(_ context.Context, batch ResultBatch) error {
		// rstream reports removed events as new data.
		for _, result := range batch.New {
			s0Old = append(s0Old, result.Get("theString").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s1.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			s1Old = append(s1Old, result.Get("theString").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	advanceViewTimeWin(t, engine, origin, 1000)
	sendViewUnionBean(t, engine, viewUnionBean{TheString: "E1"})
	advanceViewTimeWin(t, engine, origin, 2000)
	sendViewUnionBean(t, engine, viewUnionBean{TheString: "E2"})
	advanceViewTimeWin(t, engine, origin, 3000)
	sendViewUnionBean(t, engine, viewUnionBean{TheString: "E3"})
	if len(s0Old) != 0 || len(s1Old) != 0 {
		t.Fatalf("early expiry s0=%v s1=%v", s0Old, s1Old)
	}
	advanceViewTimeWin(t, engine, origin, 4000)
	if !reflect.DeepEqual(s1Old, []string{"E1"}) {
		t.Fatalf("s1 old at 4000 = %v", s1Old)
	}
	if len(s0Old) != 0 {
		t.Fatalf("s0 old at 4000 = %v", s0Old)
	}
	advanceViewTimeWin(t, engine, origin, 5000)
	if !reflect.DeepEqual(s0Old, []string{"E1"}) {
		t.Fatalf("s0 old at 5000 = %v", s0Old)
	}
}

func mustBuildViewTimeWinRStream(t *testing.T, env *Environment, window WindowSpec, name string) Plan {
	t.Helper()
	plan, err := env.Build(From[viewUnionBean](env, "SupportBean").Window(window).Query(
		StatementName(name), WithRemoveStreamOnly()))
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func deployViewTimeWinBeanRStream(t *testing.T, env *Environment, engine *Engine, window WindowSpec, name string) (*Statement, *[]ResultBatch) {
	t.Helper()
	deployment, err := engine.Deploy(context.Background(), mustBuildViewTimeWinRStream(t, env, window, name))
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

func deployViewTimeWinMarket(t *testing.T, env *Environment, engine *Engine, stream Stream[viewTimeWinMarket], name string) (*Statement, *[]ResultBatch) {
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

func viewTimeWinSnapshotSymbols(t *testing.T, statement *Statement) []string {
	t.Helper()
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	values := make([]string, 0, len(snapshot.Results()))
	for _, result := range snapshot.Results() {
		values = append(values, result.Get("symbol").Any().(string))
	}
	return values
}

func viewTimeWinOldSymbols(batches *[]ResultBatch) []string {
	if len(*batches) == 0 {
		return nil
	}
	last := (*batches)[len(*batches)-1]
	values := make([]string, 0, len(last.Old))
	for _, result := range last.Old {
		values = append(values, result.Get("symbol").Any().(string))
	}
	return values
}

// TestViewTimeWinTimePeriodParamsParity covers ViewTimeWindowTimePeriodParams:
// all textual interval spellings resolve to 30000 seconds; Go expresses each
// as an equivalent time.Duration and verifies the same expiry boundary.
func TestViewTimeWinTimePeriodParamsParity(t *testing.T) {
	durations := []time.Duration{
		30000 * time.Second,
		30e6 * time.Millisecond,
		30000 * time.Second,
		500 * time.Minute,
		8*time.Hour + 20*time.Minute,
		0*time.Second + 30000*time.Second,
		360*time.Second + 29400*time.Second + 240*time.Second,
	}
	for index, duration := range durations {
		t.Run(string(rune('a'+index)), func(t *testing.T) {
			env, initial := newViewUnionEnv(t)
			if err := initial.Close(context.Background()); err != nil {
				t.Fatal(err)
			}
			origin := time.Unix(0, 0).UTC()
			engine := NewEngine(env, WithStartTime(origin))
			defer func() { _ = engine.Close(context.Background()) }()
			_, batches := deployViewTimeWinBeanRStream(t, env, engine, TimeWindow(duration), "s0")
			sendViewUnionBean(t, engine, viewUnionBean{TheString: "E1"})
			advanceViewTimeWin(t, engine, origin, 29999*1000)
			if len(*batches) != 0 {
				t.Fatalf("early expiry = %#v", *batches)
			}
			advanceViewTimeWin(t, engine, origin, 30000*1000)
			if got := viewTimeWinRStreamStrings(batches); !reflect.DeepEqual(got, []string{"E1"}) {
				t.Fatalf("expiry = %v", got)
			}
		})
	}
}

// TestViewTimeWinFlipTimerParity covers the four ViewTimeWindowFlipTimer
// variants: fixed and calendar-period windows flip exactly at the deadline.
func TestViewTimeWinFlipTimerParity(t *testing.T) {
	variants := []struct {
		name      string
		startTime int64
		window    WindowSpec
		flipTime  int64
	}{
		{"one-second", 0, TimeWindow(time.Second), 1000},
		{"ten-seconds", 123456789, TimeWindow(10 * time.Second), 123466789},
		{"month-plus-ten-ms", 0, TimeWindowCalendar(0, 1, 0, 10*time.Millisecond), 2678400010},
		{"month-plus-fifty-ms", 1020211201999, TimeWindowCalendar(0, 1, 0, 50*time.Millisecond), 1022889602049},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			env, initial := newViewUnionEnv(t)
			if err := initial.Close(context.Background()); err != nil {
				t.Fatal(err)
			}
			origin := time.Unix(0, 0).UTC()
			engine := NewEngine(env, WithStartTime(origin))
			defer func() { _ = engine.Close(context.Background()) }()
			statement, _ := deployViewTimeWinBeanRStream(t, env, engine, variant.window, "s0")
			advanceViewTimeWin(t, engine, origin, variant.startTime)
			sendViewUnionBean(t, engine, viewUnionBean{TheString: "E1"})
			advanceViewTimeWin(t, engine, origin, variant.flipTime-1)
			snapshot, err := statement.Snapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(snapshot.Results()) != 1 {
				t.Fatalf("before flip snapshot = %v", snapshot.Results())
			}
			advanceViewTimeWin(t, engine, origin, variant.flipTime)
			snapshot, err = statement.Snapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(snapshot.Results()) != 0 {
				t.Fatalf("after flip snapshot = %v", snapshot.Results())
			}
		})
	}
}
