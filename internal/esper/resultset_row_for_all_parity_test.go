package esper

import (
	"context"
	"math"
	"testing"
)

// Parity coverage for the ungrouped (row-for-all) aggregate executions of
// ResultSetQueryTypeRowForAll:
//
// - ResultSetQueryTypeRowForAllSumMinMax: sum/min/max over all events
// - ResultSetQueryTypeRowForAllSimple: full aggregate set with irstream
// - ResultSetQueryTypeRowForAllMinMaxWindowed: min/max over length(2) window with irstream

type rowForAllBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type rowForAllMarket struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
}

func newRowForAllEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[rowForAllBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[rowForAllMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func rowForAllSubscribe(t *testing.T, deployment *Deployment) (*[]Result, *[]Result) {
	t.Helper()
	newResults := &[]Result{}
	oldResults := &[]Result{}
	_, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*newResults = append(*newResults, batch.New...)
		*oldResults = append(*oldResults, batch.Old...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return newResults, oldResults
}

// TestResultSetQueryTypeRowForAllSumMinMaxParity covers
// ResultSetQueryTypeRowForAllSumMinMax: select theString, sum(intPrimitive),
// min(intPrimitive), max(intPrimitive) from SupportBean (ungrouped, no window).
// Java runtime: java-runtime-6c590766c3ba03239c04.
func TestResultSetQueryTypeRowForAllSumMinMaxParity(t *testing.T) {
	env := newRowForAllEnv(t)
	engine := NewEngine(env)

	intPrimitive := Field[rowForAllBean, int]("intPrimitive")
	plan, err := env.Build(
		From[rowForAllBean](env, "SupportBean").Aggregate(
			Alias("c0", Field[rowForAllBean, string]("theString")),
			Alias("c1", Sum[int](intPrimitive)),
			Alias("c2", Min[int](intPrimitive)),
			Alias("c3", Max[int](intPrimitive)),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got, _ := rowForAllSubscribe(t, deployment)

	send := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), rowForAllBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}

	send("E1", 10)
	if len(*got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(*got))
	}
	row := (*got)[0]
	if row.Get("c0").Any() != "E1" || row.Get("c1").Any() != 10 || row.Get("c2").Any() != 10 || row.Get("c3").Any() != 10 {
		t.Fatalf("E1: c0=%v c1=%v c2=%v c3=%v", row.Get("c0").Any(), row.Get("c1").Any(), row.Get("c2").Any(), row.Get("c3").Any())
	}

	send("E2", 100)
	if len(*got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(*got))
	}
	row = (*got)[1]
	if row.Get("c0").Any() != "E2" || row.Get("c1").Any() != 110 || row.Get("c2").Any() != 10 || row.Get("c3").Any() != 100 {
		t.Fatalf("E2: c0=%v c1=%v c2=%v c3=%v", row.Get("c0").Any(), row.Get("c1").Any(), row.Get("c2").Any(), row.Get("c3").Any())
	}

	send("E3", 11)
	if len(*got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(*got))
	}
	row = (*got)[2]
	if row.Get("c0").Any() != "E3" || row.Get("c1").Any() != 121 || row.Get("c2").Any() != 10 || row.Get("c3").Any() != 100 {
		t.Fatalf("E3: c0=%v c1=%v c2=%v c3=%v", row.Get("c0").Any(), row.Get("c1").Any(), row.Get("c2").Any(), row.Get("c3").Any())
	}

	send("E4", 9)
	row = (*got)[3]
	if row.Get("c0").Any() != "E4" || row.Get("c1").Any() != 130 || row.Get("c2").Any() != 9 || row.Get("c3").Any() != 100 {
		t.Fatalf("E4: c0=%v c1=%v c2=%v c3=%v", row.Get("c0").Any(), row.Get("c1").Any(), row.Get("c2").Any(), row.Get("c3").Any())
	}

	send("E5", 120)
	row = (*got)[4]
	if row.Get("c0").Any() != "E5" || row.Get("c1").Any() != 250 || row.Get("c2").Any() != 9 || row.Get("c3").Any() != 120 {
		t.Fatalf("E5: c0=%v c1=%v c2=%v c3=%v", row.Get("c0").Any(), row.Get("c1").Any(), row.Get("c2").Any(), row.Get("c3").Any())
	}

	send("E6", 100)
	row = (*got)[5]
	if row.Get("c0").Any() != "E6" || row.Get("c1").Any() != 350 || row.Get("c2").Any() != 9 || row.Get("c3").Any() != 120 {
		t.Fatalf("E6: c0=%v c1=%v c2=%v c3=%v", row.Get("c0").Any(), row.Get("c1").Any(), row.Get("c2").Any(), row.Get("c3").Any())
	}
}

// TestResultSetQueryTypeRowForAllSimpleParity covers
// ResultSetQueryTypeRowForAllSimple: select irstream avg/sum/min/max/median/
// stddev/avedev/count/countDistinct over SupportMarketDataBean (ungrouped, no window).
// Java runtime: java-runtime-803e092256cda66d2f0e.
func TestResultSetQueryTypeRowForAllSimpleParity(t *testing.T) {
	env := newRowForAllEnv(t)
	engine := NewEngine(env)

	price := Field[rowForAllMarket, float64]("price")
	plan, err := env.Build(
		From[rowForAllMarket](env, "SupportMarketDataBean").Aggregate(
			Alias("avgPrice", Avg[float64](price)),
			Alias("sumPrice", Sum[float64](price)),
			Alias("minPrice", Min[float64](price)),
			Alias("maxPrice", Max[float64](price)),
			Alias("medianPrice", Median[float64](price)),
			Alias("stddevPrice", StdDev[float64](price)),
			Alias("avedevPrice", Avedev[float64](price)),
			Alias("datacount", Count[any](EventValue[rowForAllMarket]())),
			Alias("countDistinctPrice", CountDistinct[float64](price)),
		).Query(StatementName("s0"), WithOldStream()),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got, oldGot := rowForAllSubscribe(t, deployment)

	send := func(price float64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), rowForAllMarket{Symbol: "M", Price: price}); err != nil {
			t.Fatal(err)
		}
	}

	// First event: price=100
	send(100)
	if len(*got) != 1 {
		t.Fatalf("expected 1 new result, got %d", len(*got))
	}
	row := (*got)[0]
	assertFloat(t, row, "avgPrice", 100.0)
	assertFloat(t, row, "sumPrice", 100.0)
	assertFloat(t, row, "minPrice", 100.0)
	assertFloat(t, row, "maxPrice", 100.0)
	assertFloat(t, row, "medianPrice", 100.0)
	// stddev of 1 value is null in Java; Go returns NaN or 0 — check based on impl
	if row.Get("datacount").Any() != int64(1) {
		t.Fatalf("datacount = %v", row.Get("datacount").Any())
	}
	if row.Get("countDistinctPrice").Any() != int64(1) {
		t.Fatalf("countDistinctPrice = %v", row.Get("countDistinctPrice").Any())
	}
	// Old data: initial null state
	if len(*oldGot) != 1 {
		t.Fatalf("expected 1 old result, got %d", len(*oldGot))
	}
	oldRow := (*oldGot)[0]
	if oldRow.Get("datacount").Any() != int64(0) {
		t.Fatalf("old datacount = %v", oldRow.Get("datacount").Any())
	}

	// Second event: price=200
	send(200)
	if len(*got) != 2 {
		t.Fatalf("expected 2 new results, got %d", len(*got))
	}
	row = (*got)[1]
	assertFloat(t, row, "avgPrice", 150.0)
	assertFloat(t, row, "sumPrice", 300.0)
	assertFloat(t, row, "minPrice", 100.0)
	assertFloat(t, row, "maxPrice", 200.0)
	assertFloat(t, row, "medianPrice", 150.0)
	assertFloat(t, row, "avedevPrice", 50.0)
	if row.Get("datacount").Any() != int64(2) {
		t.Fatalf("datacount = %v", row.Get("datacount").Any())
	}
	// stddev of {100,200} = sqrt(((100-150)^2+(200-150)^2)/1) = sqrt(5000) ≈ 70.7107
	stddev := row.Get("stddevPrice").Any().(float64)
	if math.Abs(stddev-70.71067811865476) > 0.001 {
		t.Fatalf("stddevPrice = %v, want ~70.7107", stddev)
	}

	// Old row 2: previous state (after first event)
	if len(*oldGot) < 2 {
		t.Fatalf("expected at least 2 old results, got %d", len(*oldGot))
	}
	oldRow = (*oldGot)[1]
	assertFloat(t, oldRow, "avgPrice", 100.0)
	assertFloat(t, oldRow, "sumPrice", 100.0)
	assertFloat(t, oldRow, "minPrice", 100.0)
	assertFloat(t, oldRow, "maxPrice", 100.0)
	assertFloat(t, oldRow, "medianPrice", 100.0)
	assertFloat(t, oldRow, "avedevPrice", 0.0)
	if oldRow.Get("datacount").Any() != int64(1) {
		t.Fatalf("old row 2 datacount = %v", oldRow.Get("datacount").Any())
	}
	if oldRow.Get("countDistinctPrice").Any() != int64(1) {
		t.Fatalf("old row 2 countDistinctPrice = %v", oldRow.Get("countDistinctPrice").Any())
	}

	// Third event: price=150
	send(150)
	if len(*got) != 3 {
		t.Fatalf("expected 3 new results, got %d", len(*got))
	}
	row = (*got)[2]
	assertFloat(t, row, "avgPrice", 150.0)
	assertFloat(t, row, "sumPrice", 450.0)
	assertFloat(t, row, "minPrice", 100.0)
	assertFloat(t, row, "maxPrice", 200.0)
	assertFloat(t, row, "medianPrice", 150.0)
	assertFloat(t, row, "avedevPrice", 33.333333333333336)
	if row.Get("datacount").Any() != int64(3) {
		t.Fatalf("datacount = %v", row.Get("datacount").Any())
	}
	// stddev of {100,150,200} = sqrt(((100-150)^2+(150-150)^2+(200-150)^2)/2) = sqrt(2500) = 50
	stddev = row.Get("stddevPrice").Any().(float64)
	if math.Abs(stddev-50.0) > 0.001 {
		t.Fatalf("stddevPrice = %v, want ~50.0", stddev)
	}

	// Old row 3: previous state (after second event)
	if len(*oldGot) < 3 {
		t.Fatalf("expected at least 3 old results, got %d", len(*oldGot))
	}
	oldRow = (*oldGot)[2]
	assertFloat(t, oldRow, "avgPrice", 150.0)
	assertFloat(t, oldRow, "sumPrice", 300.0)
	assertFloat(t, oldRow, "minPrice", 100.0)
	assertFloat(t, oldRow, "maxPrice", 200.0)
	assertFloat(t, oldRow, "medianPrice", 150.0)
	assertFloat(t, oldRow, "avedevPrice", 50.0)
	oldStddev := oldRow.Get("stddevPrice").Any().(float64)
	if math.Abs(oldStddev-70.71067811865476) > 0.001 {
		t.Fatalf("old row 3 stddevPrice = %v, want ~70.7107", oldStddev)
	}
	if oldRow.Get("datacount").Any() != int64(2) {
		t.Fatalf("old row 3 datacount = %v", oldRow.Get("datacount").Any())
	}
	if oldRow.Get("countDistinctPrice").Any() != int64(2) {
		t.Fatalf("old row 3 countDistinctPrice = %v", oldRow.Get("countDistinctPrice").Any())
	}
}

// TestResultSetQueryTypeRowForAllMinMaxWindowedParity covers
// ResultSetQueryTypeRowForAllMinMaxWindowed: select irstream min(price), max(price)
// from SupportMarketDataBean#length(2).
// Java runtime: java-runtime-e54e4c73c8eec446581f.
func TestResultSetQueryTypeRowForAllMinMaxWindowedParity(t *testing.T) {
	env := newRowForAllEnv(t)
	engine := NewEngine(env)

	price := Field[rowForAllMarket, float64]("price")
	plan, err := env.Build(
		From[rowForAllMarket](env, "SupportMarketDataBean").Window(LengthWindow(2)).Aggregate(
			Alias("minPrice", Min[float64](price)),
			Alias("maxPrice", Max[float64](price)),
		).Query(StatementName("s0"), WithOldStream()),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got, oldGot := rowForAllSubscribe(t, deployment)

	send := func(price float64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), rowForAllMarket{Symbol: "M", Price: price}); err != nil {
			t.Fatal(err)
		}
	}

	// price=100 → min=100, max=100
	send(100)
	if len(*got) != 1 {
		t.Fatalf("expected 1 new result, got %d", len(*got))
	}
	row := (*got)[0]
	assertFloat(t, row, "minPrice", 100.0)
	assertFloat(t, row, "maxPrice", 100.0)
	// Old: null state
	if len(*oldGot) != 1 {
		t.Fatalf("expected 1 old result, got %d", len(*oldGot))
	}

	// price=200 → min=100, max=200
	send(200)
	if len(*got) != 2 {
		t.Fatalf("expected 2 new results, got %d", len(*got))
	}
	row = (*got)[1]
	assertFloat(t, row, "minPrice", 100.0)
	assertFloat(t, row, "maxPrice", 200.0)
	// Old: previous state min=100, max=100
	if len(*oldGot) != 2 {
		t.Fatalf("expected 2 old results, got %d", len(*oldGot))
	}
	oldRow := (*oldGot)[1]
	assertFloat(t, oldRow, "minPrice", 100.0)
	assertFloat(t, oldRow, "maxPrice", 100.0)

	// price=150 → window slides, evicting 100 → min=150, max=200
	send(150)
	if len(*got) != 3 {
		t.Fatalf("expected 3 new results, got %d", len(*got))
	}
	row = (*got)[2]
	assertFloat(t, row, "minPrice", 150.0)
	assertFloat(t, row, "maxPrice", 200.0)
	// Old: previous state min=100, max=200
	if len(*oldGot) != 3 {
		t.Fatalf("expected 3 old results, got %d", len(*oldGot))
	}
	oldRow = (*oldGot)[2]
	assertFloat(t, oldRow, "minPrice", 100.0)
	assertFloat(t, oldRow, "maxPrice", 200.0)
}

func assertFloat(t *testing.T, row Result, name string, expected float64) {
	t.Helper()
	got, ok := row.Get(name).Any().(float64)
	if !ok {
		t.Fatalf("%s is not float64: %T %v", name, row.Get(name).Any(), row.Get(name).Any())
	}
	if math.Abs(got-expected) > 0.001 {
		t.Fatalf("%s = %v, want %v", name, got, expected)
	}
}

// TestResultSetQueryTypeRowForAllWWindowAggParity covers
// ResultSetQueryTypeRowForAllWWindowAgg: select irstream theString,
// sum(intPrimitive), window(*) from SupportBean#length(2). When the window
// overflows, both the new row and the removed (old) row carry the recomputed
// aggregate and the post-eviction window contents.
// Java runtime: java-runtime-287708385b40a014c3fc.
func TestResultSetQueryTypeRowForAllWWindowAggParity(t *testing.T) {
	env := newRowForAllEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(
		From[rowForAllBean](env, "SupportBean").Window(LengthWindow(2)).Aggregate(
			Alias("c0", Field[rowForAllBean, string]("theString")),
			Alias("c1", Sum[int](Field[rowForAllBean, int]("intPrimitive"))),
			Alias("c2", WindowEvents()),
		).Query(StatementName("s0"), WithOldStream()),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	newGot, oldGot := rowForAllSubscribe(t, deployment)

	assertRow := func(label string, row Result, theString string, sum int, window []rowForAllBean) {
		t.Helper()
		if got := row.Get("c0").Any(); got != theString {
			t.Fatalf("%s c0 = %v, want %v", label, got, theString)
		}
		if got, ok := row.Get("c1").Any().(int); !ok || got != sum {
			t.Fatalf("%s c1 = %v, want %v", label, row.Get("c1").Any(), sum)
		}
		events, ok := row.Get("c2").Any().([]Event)
		if !ok {
			t.Fatalf("%s c2 is not []Event: %T", label, row.Get("c2").Any())
		}
		if len(events) != len(window) {
			t.Fatalf("%s c2 length = %d, want %v", label, len(events), window)
		}
		for i := range window {
			got := EventValue[rowForAllBean]().eval(EvalContext{Event: events[i]})
			bean, ok := got.Any().(rowForAllBean)
			if !ok || bean != window[i] {
				t.Fatalf("%s c2[%d] = %v, want %v", label, i, got.Any(), window[i])
			}
		}
	}

	for _, tc := range []struct {
		theString string
		intValue  int
	}{
		{"E1", 10}, {"E2", 100}, {"E3", 11}, {"E4", 9},
	} {
		*newGot = (*newGot)[:0]
		*oldGot = (*oldGot)[:0]
		if err := engine.SendEvent(context.Background(), rowForAllBean{TheString: tc.theString, IntPrimitive: tc.intValue}); err != nil {
			t.Fatal(err)
		}
		switch tc.theString {
		case "E1":
			assertRow("E1 new", (*newGot)[0], "E1", 10, []rowForAllBean{{"E1", 10}})
		case "E2":
			assertRow("E2 new", (*newGot)[0], "E2", 110, []rowForAllBean{{"E1", 10}, {"E2", 100}})
		case "E3":
			assertRow("E3 new", (*newGot)[0], "E3", 111, []rowForAllBean{{"E2", 100}, {"E3", 11}})
			assertRow("E3 old", (*oldGot)[0], "E1", 111, []rowForAllBean{{"E2", 100}, {"E3", 11}})
		case "E4":
			assertRow("E4 new", (*newGot)[0], "E4", 20, []rowForAllBean{{"E3", 11}, {"E4", 9}})
			assertRow("E4 old", (*oldGot)[0], "E2", 20, []rowForAllBean{{"E3", 11}, {"E4", 9}})
		}
	}
}

// TestResultSetQueryTypeRowForAllNamedWindowWindowParity covers
// ResultSetQueryTypeRowForAllNamedWindowWindow: create window ABCWin#keepall
// as SupportBean; insert into ABCWin select * from SupportBean; on
// SupportBean_A delete from ABCWin where theString = id; select irstream
// theString, window(intPrimitive) from ABCWin.
// Java runtime: java-runtime-a86df6b7b7185701f64b.
func TestResultSetQueryTypeRowForAllNamedWindowWindowParity(t *testing.T) {
	env := newRowForAllEnv(t)
	if _, err := RegisterStruct[rowForAllBeanA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema missing")
	}
	if _, err := CreateNamedWindow(env, "ABCWin", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	insertPlan, err := env.Build(OnEvent(From[rowForAllBean](env, "SupportBean")).InsertIntoNamedWindow("ABCWin",
		SetColumn("theString", Field[rowForAllBean, string]("theString")),
		SetColumn("intPrimitive", Field[rowForAllBean, int]("intPrimitive")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	queryPlan, err := env.Build(FromNamedWindow(env, "ABCWin").Aggregate(
		Alias("c0", Field[any, string]("theString")),
		Alias("c1", WindowValues[int](Field[any, int]("intPrimitive"))),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[rowForAllBeanA](env, "SupportBean_A")).DeleteFromNamedWindow("ABCWin",
		Equal[string](NamedWindowField[string]("theString"), Field[rowForAllBeanA, string]("id")),
	).Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	queryDeployment, err := engine.Deploy(context.Background(), queryPlan)
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range []Plan{insertPlan, deletePlan} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	newGot, oldGot := rowForAllSubscribe(t, queryDeployment)

	assertRow := func(label string, row Result, theString string, window []int) {
		t.Helper()
		if got := row.Get("c0").Any(); got != theString {
			t.Fatalf("%s c0 = %v, want %v", label, got, theString)
		}
		gotWindow, ok := row.Get("c1").Any().([]int)
		if !ok || len(gotWindow) != len(window) {
			t.Fatalf("%s c1 = %v, want %v", label, row.Get("c1").Any(), window)
		}
		for i := range window {
			if gotWindow[i] != window[i] {
				t.Fatalf("%s c1 = %v, want %v", label, gotWindow, window)
			}
		}
	}

	steps := []struct {
		action     string
		theString  string
		intValue   int
		expectNew  bool
		expectOld  bool
		wantString string
		wantWindow []int
	}{
		{"send", "E1", 10, true, false, "E1", []int{10}},
		{"send", "E2", 100, true, false, "E2", []int{10, 100}},
		{"delete", "E2", 0, false, true, "E2", []int{10}},
		{"send", "E3", 50, true, false, "E3", []int{10, 50}},
		{"delete", "E1", 0, false, true, "E1", []int{50}},
		{"send", "E4", -1, true, false, "E4", []int{50, -1}},
	}
	for _, step := range steps {
		*newGot = (*newGot)[:0]
		*oldGot = (*oldGot)[:0]
		if step.action == "send" {
			if err := engine.SendEvent(context.Background(), rowForAllBean{TheString: step.theString, IntPrimitive: step.intValue}); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := engine.SendEvent(context.Background(), rowForAllBeanA{ID: step.theString}); err != nil {
				t.Fatal(err)
			}
		}
		if len(*newGot) != boolToInt(step.expectNew) {
			t.Fatalf("%s/%s: new count = %d, want %d", step.action, step.theString, len(*newGot), boolToInt(step.expectNew))
		}
		if len(*oldGot) != boolToInt(step.expectOld) {
			t.Fatalf("%s/%s: old count = %d, want %d", step.action, step.theString, len(*oldGot), boolToInt(step.expectOld))
		}
		if step.expectNew {
			assertRow(step.theString+" new", (*newGot)[0], step.wantString, step.wantWindow)
		}
		if step.expectOld {
			assertRow(step.theString+" old", (*oldGot)[0], step.wantString, step.wantWindow)
		}
	}
}

type rowForAllBeanA struct {
	ID string `esper:"id"`
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
