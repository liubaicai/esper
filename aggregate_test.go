package esper

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func (trade runtimeTestTrade) PriceLabel() string {
	return fmt.Sprintf("%s:%.0f", trade.Symbol, trade.Price)
}

type runtimeRateEvent struct {
	Symbol    string  `esper:"symbol"`
	Timestamp int64   `esper:"timestamp"`
	Quantity  float64 `esper:"quantity"`
}

type runtimeDeleteSignal struct {
	ID string `esper:"id"`
}

func TestGroupedAggregateAndHaving(t *testing.T) {
	env, engine := newRuntimeTest(t)
	trade := From[runtimeTestTrade](env, "Trade")
	grouped := trade.GroupBy(Field[runtimeTestTrade, string]("symbol")).Select(
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
		Alias("count", CountAll()),
		Alias("sum", Sum[float64](Field[runtimeTestTrade, float64]("price"))),
		Alias("avg", Avg[float64](Field[runtimeTestTrade, float64]("price"))),
	).Having(Greater[int64](CountAll(), Literal(int64(0))))
	plan, err := env.Build(grouped.Query(StatementName("trade-aggregate"), WithOldStream()))
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
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 2}, {Symbol: "A", Price: 4}, {Symbol: "B", Price: 10}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 3 {
		t.Fatalf("aggregate batches = %#v", batches)
	}
	first, ok := batches[0].New[0].Row()
	if !ok || first.Get("symbol").Any() != "A" || first.Get("count").Any() != int64(1) || first.Get("sum").Any() != float64(2) {
		t.Fatalf("first aggregate row = %#v", first)
	}
	second, ok := batches[1].New[0].Row()
	if !ok || second.Get("count").Any() != int64(2) || second.Get("sum").Any() != float64(6) || second.Get("avg").Any() != float64(3) {
		t.Fatalf("second aggregate row = %#v", second)
	}
	old, ok := batches[1].Old[0].Row()
	if !ok || old.Get("count").Any() != int64(1) || old.Get("sum").Any() != float64(2) {
		t.Fatalf("aggregate old row = %#v", batches[1].Old)
	}
}

func TestAggregateWhereFiltersOrdinaryEventsBeforeGrouping(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("count", CountAll()),
		Alias("sum", Sum[float64](price)),
	).Where(GreaterOrEqual[float64](price, Literal(10.0))).Query(
		StatementName("ordinary-aggregate-where"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 2}, {Symbol: "A", Price: 10}, {Symbol: "A", Price: 20}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 2 || len(batches[0].Old) != 0 || len(batches[1].Old) != 1 || len(batches[1].New) != 1 {
		t.Fatalf("ordinary aggregate Where batches = %#v", batches)
	}
	first, ok := batches[0].New[0].Row()
	if !ok || first.Get("count").Any() != int64(1) || first.Get("sum").Any() != float64(10) {
		t.Fatalf("ordinary aggregate Where first row = %#v", batches[0].New)
	}
	current, ok := batches[1].New[0].Row()
	if !ok || current.Get("count").Any() != int64(2) || current.Get("sum").Any() != float64(30) {
		t.Fatalf("ordinary aggregate Where current row = %#v", batches[1].New)
	}
}

func TestExtendedAggregateFunctions(t *testing.T) {
	env, engine := newRuntimeTest(t)
	trade := From[runtimeTestTrade](env, "Trade")
	plan, err := env.Build(trade.GroupBy(
		Field[runtimeTestTrade, string]("symbol"),
	).Select(
		Alias("first", First[float64](Field[runtimeTestTrade, float64]("price"))),
		Alias("last", Last[float64](Field[runtimeTestTrade, float64]("price"))),
		Alias("nth", Nth[float64](Field[runtimeTestTrade, float64]("price"), 1)),
		Alias("distinct", CountDistinct[float64](Field[runtimeTestTrade, float64]("price"))),
		Alias("median", Median[float64](Field[runtimeTestTrade, float64]("price"))),
		Alias("stddev", StdDev[float64](Field[runtimeTestTrade, float64]("price"))),
	).Query(StatementName("extended-aggregate")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var last Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) == 0 {
			return nil
		}
		var ok bool
		last, ok = batch.New[len(batch.New)-1].Row()
		if !ok {
			t.Fatal("aggregate result is not a row")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, price := range []float64{2, 8, 4, 8} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "X", Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	if last.Get("first").Any() != float64(2) || last.Get("last").Any() != float64(8) || last.Get("nth").Any() != float64(8) {
		t.Fatalf("position aggregates = %#v", last.AsMap())
	}
	if last.Get("distinct").Any() != int64(3) || last.Get("median").Any() != float64(6) {
		t.Fatalf("distinct/median aggregates = %#v", last.AsMap())
	}
	if last.Get("stddev").Any() != 3.0 {
		t.Fatalf("stddev aggregate = %#v", last.AsMap())
	}
}

func TestAggregateLeavingRetainsWindowEvictionStateAndFilter(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(2)).Aggregate(
		Alias("leaving", Leaving()),
		Alias("leftOne", Leaving(Equal[float64](price, Literal(1.0)))),
		Alias("leftTwo", Leaving(Equal[float64](price, Literal(2.0)))),
	).Query(StatementName("aggregate-leaving")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 4)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("aggregate leaving result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{2, 1, 3, 4} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: value}); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 4 {
		t.Fatalf("aggregate leaving rows = %d", len(rows))
	}
	expected := [][3]bool{{false, false, false}, {false, false, false}, {true, false, true}, {true, true, true}}
	for index, row := range rows {
		for column, name := range []string{"leaving", "leftOne", "leftTwo"} {
			if row.Get(name).Any() != expected[index][column] {
				t.Fatalf("aggregate leaving row %d %s = %#v, expected %v", index, name, row.Get(name).Any(), expected[index][column])
			}
		}
	}
}

func TestIndexedFirstLastAndWindowEvents(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	window := WindowAccessBy[runtimeTestTrade](EventValue[runtimeTestTrade]())
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(3)).Aggregate(
		Alias("first0", First[float64](price)),
		Alias("first1", First[float64](price, 1)),
		Alias("last0", Last[float64](price)),
		Alias("last1", Last[float64](price, 1)),
		Alias("nth1", Nth[float64](price, 1)),
		Alias("firstSymbol", Property[string](FirstEventValue(), "symbol")),
		Alias("lastPrice", Property[float64](LastEventValue(), "price")),
		Alias("lastLabel", Method[string](LastEventValue(), "PriceLabel")),
		Alias("events", WindowEvents()),
		Alias("windowFirst", window.First()),
		Alias("windowLast", window.Last()),
		Alias("windowValues", window.Values()),
		Alias("windowCount", window.CountEvents()),
	).Query(StatementName("indexed-first-last")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("indexed aggregate result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{10, 20, 30, 40} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: value}); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 4 {
		t.Fatalf("indexed aggregate rows = %d", len(rows))
	}
	last := rows[len(rows)-1]
	for name, expected := range map[string]float64{
		"first0": 20,
		"first1": 30,
		"last0":  40,
		"last1":  30,
		"nth1":   30,
	} {
		if got := last.Get(name).Any(); got != expected {
			t.Fatalf("%s = %#v, want %v", name, got, expected)
		}
	}
	if last.Get("firstSymbol").Any() != "A" || last.Get("lastPrice").Any() != float64(40) || last.Get("lastLabel").Any() != "A:40" {
		t.Fatalf("no-argument event access = %#v", last.AsMap())
	}
	events, ok := last.Get("events").Any().([]Event)
	if !ok || len(events) != 3 || events[0].Get("price").Any() != float64(20) || events[2].Get("price").Any() != float64(40) {
		t.Fatalf("window events = %#v", last.Get("events").Any())
	}
	firstEvent, ok := last.Get("windowFirst").Any().(runtimeTestTrade)
	if !ok || firstEvent.Price != 20 {
		t.Fatalf("window access first = %#v", last.Get("windowFirst").Any())
	}
	lastEvent, ok := last.Get("windowLast").Any().(runtimeTestTrade)
	if !ok || lastEvent.Price != 40 {
		t.Fatalf("window access last = %#v", last.Get("windowLast").Any())
	}
	if values, ok := last.Get("windowValues").Any().([]runtimeTestTrade); !ok || len(values) != 3 || values[0].Price != 20 || values[2].Price != 40 || last.Get("windowCount").Any() != int64(3) {
		t.Fatalf("window access values = %#v", last.AsMap())
	}
}

func TestAggregateResultOrderByAndHavingUseProjectedRows(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(TimeBatch(time.Second)).GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("sum", Sum[float64](price)),
	).Having(Greater[float64](Sum[float64](price), Literal(0.0))).Query(
		StatementName("aggregate-order-by"),
		OrderBy(Descending(ResultField[float64]("sum"))),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("ordered aggregate result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "B", Price: 2}, {Symbol: "A", Price: 8}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Get("symbol").Any() != "A" || rows[0].Get("sum").Any() != float64(8) || rows[1].Get("symbol").Any() != "B" {
		t.Fatalf("ordered aggregate rows = %#v", rows)
	}

	invalid := From[runtimeTestTrade](env, "Trade").GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("sum", Sum[float64](price)),
	).Query(OrderBy(Ascending(ResultField[float64]("missing"))))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("unknown aggregate result order field unexpectedly built")
	}
}

func TestLocalGroupByAggregateUsesCurrentEventAndOuterGroup(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("localSum", LocalGroupBy[float64](Sum[float64](price), symbol)),
		Alias("localCount", LocalGroupBy[int64](CountAll(), symbol)),
		Alias("total", Sum[float64](price)),
	).Query(StatementName("local-group-by")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var last Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) == 0 {
			return nil
		}
		var ok bool
		last, ok = batch.New[len(batch.New)-1].Row()
		if !ok {
			return fmt.Errorf("local group result is not a row")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "A", Price: 2},
		{Symbol: "B", Price: 3},
		{Symbol: "A", Price: 4},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if last.Get("localSum").Any() != float64(6) || last.Get("localCount").Any() != int64(2) || last.Get("total").Any() != float64(9) {
		t.Fatalf("local group values = %#v", last.AsMap())
	}

	groupedPlan, err := env.Build(From[runtimeTestTrade](env, "Trade").GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("samePrice", LocalGroupBy[float64](Sum[float64](price), price)),
		Alias("allInSymbol", Sum[float64](price)),
	).Query(StatementName("local-group-by-outer")))
	if err != nil {
		t.Fatal(err)
	}
	groupedDeployment, err := engine.Deploy(context.Background(), groupedPlan)
	if err != nil {
		t.Fatal(err)
	}
	var grouped Row
	if _, err := groupedDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			grouped, _ = batch.New[len(batch.New)-1].Row()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 2}, {Symbol: "A", Price: 4}, {Symbol: "A", Price: 2}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if grouped.Get("samePrice").Any() != float64(4) || grouped.Get("allInSymbol").Any() != float64(8) {
		t.Fatalf("outer local group values = %#v", grouped.AsMap())
	}

	invalid := From[runtimeTestTrade](env, "Trade").GroupByRollup(symbol).Select(
		Alias("value", LocalGroupBy[float64](Sum[float64](price), symbol)),
	).Query()
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("local group aggregate combined with rollup unexpectedly built")
	}
}

func TestStatisticalAndCollectionAggregateFunctions(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").GroupBy(
		Field[runtimeTestTrade, string]("symbol"),
	).Select(
		Alias("avedev", Avedev[float64](price)),
		Alias("variance", Variance[float64](price)),
		Alias("stddevpop", StdDevPop[float64](price)),
		Alias("weighted", WeightedAvg[float64, float64](price, price)),
		Alias("minby", MinBy[float64, float64](price, price)),
		Alias("maxby", MaxBy[float64, float64](price, price)),
		Alias("window", WindowValues[float64](price)),
		Alias("set", SetOfValues[float64](price)),
		Alias("sorted", SortedValues[float64](price, false)),
	).Query(StatementName("statistical-aggregate")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var last Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) == 0 {
			return nil
		}
		var ok bool
		last, ok = batch.New[len(batch.New)-1].Row()
		if !ok {
			t.Fatal("aggregate result is not a row")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, priceValue := range []float64{2, 8, 4, 8} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "X", Price: priceValue}); err != nil {
			t.Fatal(err)
		}
	}
	if got := last.Get("avedev").Any(); got != 2.5 {
		t.Fatalf("avedev = %#v", got)
	}
	if got := last.Get("variance").Any(); got != 9.0 {
		t.Fatalf("variance = %#v", got)
	}
	if got := last.Get("stddevpop").Any(); math.Abs(got.(float64)-math.Sqrt(6.75)) > 1e-12 {
		t.Fatalf("stddevpop = %#v", got)
	}
	if got := last.Get("weighted").Any(); math.Abs(got.(float64)-148.0/22.0) > 1e-12 {
		t.Fatalf("weighted average = %#v", got)
	}
	if last.Get("minby").Any() != 2.0 || last.Get("maxby").Any() != 8.0 {
		t.Fatalf("min/max by = %#v", last.AsMap())
	}
	for name, expected := range map[string][]float64{
		"window": {2, 8, 4, 8},
		"set":    {2, 8, 4},
		"sorted": {2, 4, 8, 8},
	} {
		if got := last.Get(name).Any(); !reflect.DeepEqual(got, expected) {
			t.Fatalf("%s = %#v, want %#v", name, got, expected)
		}
	}
}

type derivedViewTestPoint struct {
	X float64 `esper:"x"`
	Y float64 `esper:"y"`
}

func TestDerivedViewCorrelationAndLinearRegression(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[derivedViewTestPoint](env, "Point"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	x := Field[derivedViewTestPoint, float64]("x")
	y := Field[derivedViewTestPoint, float64]("y")
	regression := LinearRegression[float64, float64](x, y)
	statistics := UnivariateStatistics[float64](x)
	plan, err := env.Build(From[derivedViewTestPoint](env, "Point").Window(LengthWindow(3)).Aggregate(
		Alias("size", CountAll()),
		Alias("correlation", Correlation[float64, float64](x, y)),
		Alias("slope", regression.Slope()),
		Alias("YIntercept", regression.YIntercept()),
		Alias("uniTotal", statistics.Total()),
		Alias("uniDatapoints", statistics.Datapoints()),
		Alias("uniAverage", statistics.Average()),
		Alias("uniVariance", statistics.Variance()),
		Alias("uniStdDev", statistics.StdDev()),
		Alias("uniStdDevPop", statistics.StdDevPop()),
	).Query(StatementName("derived-view-statistics"), WithOldStream()))
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
	points := []derivedViewTestPoint{
		{X: 70, Y: 1000},
		{X: 70.5, Y: 1500},
		{X: 70.1, Y: 1200},
		{X: 70.25, Y: 1000},
	}
	for _, point := range points {
		if err := engine.SendEvent(context.Background(), point); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != len(points) {
		t.Fatalf("got %d derived-view batches, want %d", len(batches), len(points))
	}
	assertDerivedViewRow := func(result Result, wantSize int64, wantCorrelation, wantSlope, wantIntercept float64) {
		row, ok := result.Row()
		if !ok {
			t.Fatalf("derived-view result is not a row: %#v", result)
		}
		if got := row.Get("size").Any(); got != wantSize {
			t.Fatalf("derived-view size = %#v, want %d", got, wantSize)
		}
		gotCorrelation := row.Get("correlation").Any().(float64)
		if math.IsNaN(wantCorrelation) {
			if !math.IsNaN(gotCorrelation) {
				t.Fatalf("derived-view correlation = %v, want NaN", gotCorrelation)
			}
		} else if math.Abs(gotCorrelation-wantCorrelation) > 1e-9 {
			t.Fatalf("derived-view correlation = %.12f, want %.12f", gotCorrelation, wantCorrelation)
		}
		gotSlope := row.Get("slope").Any().(float64)
		gotIntercept := row.Get("YIntercept").Any().(float64)
		if math.IsNaN(wantSlope) {
			if !math.IsNaN(gotSlope) || !math.IsNaN(gotIntercept) {
				t.Fatalf("derived-view regression = (%v,%v), want NaN pair", gotSlope, gotIntercept)
			}
			return
		}
		if math.Abs(gotSlope-wantSlope) > 1e-5 || math.Abs(gotIntercept-wantIntercept) > 1e-5 {
			t.Fatalf("derived-view regression = (%.12f,%.12f), want (%.12f,%.12f)", gotSlope, gotIntercept, wantSlope, wantIntercept)
		}
	}
	assertDerivedViewRow(batches[0].New[0], 1, math.NaN(), math.NaN(), math.NaN())
	assertDerivedViewRow(batches[1].New[0], 2, 1, 1000, -69000)
	assertDerivedViewRow(batches[2].New[0], 3, 0.9762210399358, 928.571428587354, -63952.38095349892)
	assertDerivedViewRow(batches[3].New[0], 3, 0.7046340397673054, 877.5510204634593, -60443.8775549068)
	firstRow, ok := batches[0].New[0].Row()
	if !ok {
		t.Fatal("first derived-view result is not a row")
	}
	if firstRow.Get("uniTotal").Any() != 70.0 || firstRow.Get("uniDatapoints").Any() != int64(1) || firstRow.Get("uniAverage").Any() != 70.0 {
		t.Fatalf("singleton univariate statistics = %#v", firstRow.AsMap())
	}
	if firstRow.Get("uniStdDevPop").Any() != 0.0 || !math.IsNaN(firstRow.Get("uniVariance").Any().(float64)) || !math.IsNaN(firstRow.Get("uniStdDev").Any().(float64)) {
		t.Fatalf("singleton univariate NaN policy = %#v", firstRow.AsMap())
	}
	lastRow, ok := batches[3].New[0].Row()
	if !ok {
		t.Fatal("last derived-view result is not a row")
	}
	if math.Abs(lastRow.Get("uniTotal").Any().(float64)-210.85) > 1e-9 || lastRow.Get("uniDatapoints").Any() != int64(3) || math.Abs(lastRow.Get("uniAverage").Any().(float64)-70.28333333333333) > 1e-9 {
		t.Fatalf("windowed univariate total/count/average = %#v", lastRow.AsMap())
	}
	if math.Abs(lastRow.Get("uniVariance").Any().(float64)-0.04083333333333333) > 1e-9 || math.Abs(lastRow.Get("uniStdDev").Any().(float64)-math.Sqrt(0.04083333333333333)) > 1e-9 || math.Abs(lastRow.Get("uniStdDevPop").Any().(float64)-math.Sqrt(0.02722222222222222)) > 1e-9 {
		t.Fatalf("windowed univariate variance/stddev = %#v", lastRow.AsMap())
	}
	if len(batches[3].Old) != 1 {
		t.Fatalf("length-window derived view old stream = %#v", batches[3].Old)
	}
	assertDerivedViewRow(batches[3].Old[0], 3, 0.9762210399358, 928.571428587354, -63952.38095349892)
}

func TestEverMinMaxByAndSortedEventAccess(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(3)).Aggregate(
		Alias("minCurrent", MinBy[runtimeTestTrade, float64](EventValue[runtimeTestTrade](), price)),
		Alias("maxCurrent", MaxBy[runtimeTestTrade, float64](EventValue[runtimeTestTrade](), price)),
		Alias("minEver", MinByEver[runtimeTestTrade, float64](EventValue[runtimeTestTrade](), price)),
		Alias("maxEver", MaxByEver[runtimeTestTrade, float64](EventValue[runtimeTestTrade](), price)),
		Alias("sorted", SortedEvents(Descending(price))),
	).Query(StatementName("ever-min-max-by")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var last Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) == 0 {
			return nil
		}
		var ok bool
		last, ok = batch.New[len(batch.New)-1].Row()
		if !ok {
			return fmt.Errorf("event access aggregate result is not a row")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{1, 10, 20, 5} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: value}); err != nil {
			t.Fatal(err)
		}
	}
	minCurrent, ok := last.Get("minCurrent").Any().(runtimeTestTrade)
	if !ok || minCurrent.Price != 5 {
		t.Fatalf("min current = %#v", last.Get("minCurrent").Any())
	}
	maxCurrent, ok := last.Get("maxCurrent").Any().(runtimeTestTrade)
	if !ok || maxCurrent.Price != 20 {
		t.Fatalf("max current = %#v", last.Get("maxCurrent").Any())
	}
	minEver, ok := last.Get("minEver").Any().(runtimeTestTrade)
	if !ok || minEver.Price != 1 {
		t.Fatalf("min ever = %#v", last.Get("minEver").Any())
	}
	maxEver, ok := last.Get("maxEver").Any().(runtimeTestTrade)
	if !ok || maxEver.Price != 20 {
		t.Fatalf("max ever = %#v", last.Get("maxEver").Any())
	}
	sorted, ok := last.Get("sorted").Any().([]Event)
	if !ok || len(sorted) != 3 || sorted[0].Get("price").Any() != float64(20) || sorted[2].Get("price").Any() != float64(5) {
		t.Fatalf("sorted events = %#v", last.Get("sorted").Any())
	}
}

func TestSortedAccessNavigationAndDuplicateKeyBuckets(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	sorted := SortedAccessBy[runtimeTestTrade, float64](EventValue[runtimeTestTrade](), price)
	var missingKey Expression[float64]
	invalid := From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("bad", SortedAccessBy[runtimeTestTrade, float64](EventValue[runtimeTestTrade](), missingKey).FirstKey()),
	).Query(StatementName("invalid-sorted-access"))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("sorted access without a key unexpectedly built")
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(4)).Aggregate(
		Alias("firstKey", sorted.FirstKey()),
		Alias("lastKey", sorted.LastKey()),
		Alias("lowerKey", sorted.LowerKey(Literal(15.0))),
		Alias("floorKey", sorted.FloorKey(Literal(15.0))),
		Alias("higherKey", sorted.HigherKey(Literal(15.0))),
		Alias("ceilingKey", sorted.CeilingKey(Literal(15.0))),
		Alias("firstEvent", sorted.FirstEvent()),
		Alias("lastEvent", sorted.LastEvent()),
		Alias("firstEvents", sorted.FirstEvents()),
		Alias("lastEvents", sorted.LastEvents()),
		Alias("getEvent", sorted.GetEvent(Literal(10.0))),
		Alias("getEvents", sorted.GetEvents(Literal(10.0))),
		Alias("contains", sorted.Contains(Literal(20.0))),
		Alias("eventCount", sorted.CountEvents()),
		Alias("keyCount", sorted.CountKeys()),
		Alias("between", sorted.EventsBetween(Literal(10.0), true, Literal(20.0), true)),
		Alias("submap", sorted.SubMap(Literal(10.0), true, Literal(20.0), true)),
	).Query(StatementName("sorted-access-navigation")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var last Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) == 0 {
			return nil
		}
		var ok bool
		last, ok = batch.New[len(batch.New)-1].Row()
		if !ok {
			return fmt.Errorf("sorted access result is not a row")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "A", Price: 10},
		{Symbol: "B", Price: 10},
		{Symbol: "C", Price: 20},
		{Symbol: "D", Price: 30},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if last.Get("firstKey").Any() != float64(10) || last.Get("lastKey").Any() != float64(30) ||
		last.Get("lowerKey").Any() != float64(10) || last.Get("floorKey").Any() != float64(10) ||
		last.Get("higherKey").Any() != float64(20) || last.Get("ceilingKey").Any() != float64(20) {
		t.Fatalf("sorted navigation keys = %#v", last.AsMap())
	}
	first, ok := last.Get("firstEvent").Any().(runtimeTestTrade)
	if !ok || first.Symbol != "A" {
		t.Fatalf("first sorted event = %#v", last.Get("firstEvent").Any())
	}
	lastEvent, ok := last.Get("lastEvent").Any().(runtimeTestTrade)
	if !ok || lastEvent.Symbol != "D" {
		t.Fatalf("last sorted event = %#v", last.Get("lastEvent").Any())
	}
	if got, ok := last.Get("firstEvents").Any().([]runtimeTestTrade); !ok || len(got) != 2 || got[0].Symbol != "A" || got[1].Symbol != "B" {
		t.Fatalf("first duplicate bucket = %#v", last.Get("firstEvents").Any())
	}
	if got, ok := last.Get("getEvents").Any().([]runtimeTestTrade); !ok || len(got) != 2 || got[0].Symbol != "A" || got[1].Symbol != "B" {
		t.Fatalf("lookup duplicate bucket = %#v", last.Get("getEvents").Any())
	}
	if last.Get("contains").Any() != true || last.Get("eventCount").Any() != int64(4) || last.Get("keyCount").Any() != int64(3) {
		t.Fatalf("sorted access counts = %#v", last.AsMap())
	}
	if got, ok := last.Get("between").Any().([]runtimeTestTrade); !ok || len(got) != 3 || got[0].Symbol != "A" || got[2].Symbol != "C" {
		t.Fatalf("sorted events between = %#v", last.Get("between").Any())
	}
	access, ok := last.Get("submap").Any().(SortedAccessValue[float64, runtimeTestTrade])
	if !ok || access.CountKeys() != 2 || access.CountEvents() != 3 {
		t.Fatalf("sorted submap = %#v", last.Get("submap").Any())
	}

	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E", Price: 40}); err != nil {
		t.Fatal(err)
	}
	if last.Get("firstKey").Any() != float64(10) || last.Get("lastKey").Any() != float64(40) {
		t.Fatalf("sorted access after eviction = %#v", last.AsMap())
	}
}

func TestAggregateToTableSinkPersistsSortedAccessState(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "SortedPrices", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[SortedAccessValue[float64, runtimeTestTrade]]("sortcol"),
		TableColumnOf[int64]("eventCount"),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	sink, err := NewTableSink(engine, "SortedPrices")
	if err != nil {
		t.Fatal(err)
	}
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	sorted := SortedAccessBy[runtimeTestTrade, float64](EventValue[runtimeTestTrade](), price)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(3)).GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("sortcol", sorted),
		Alias("eventCount", CountAll()),
	).To(sink, StatementName("aggregate-to-table")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "A", Price: 10},
		{Symbol: "A", Price: 20},
		{Symbol: "B", Price: 5},
		{Symbol: "A", Price: 30},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	table, ok := engine.Table("SortedPrices")
	if !ok {
		t.Fatal("sorted prices table is missing")
	}
	row, ok, err := table.Get(context.Background(), "A")
	if err != nil || !ok {
		t.Fatalf("table row A = %#v, ok=%v, err=%v", row, ok, err)
	}
	if row.Get("eventCount").Any() != int64(2) {
		t.Fatalf("table aggregate count = %#v", row.Values())
	}
	access, ok := row.Get("sortcol").Any().(SortedAccessValue[float64, runtimeTestTrade])
	if !ok {
		t.Fatalf("table sorted access type = %#v", row.Get("sortcol").Any())
	}
	first, firstOK := access.FirstEvent()
	last, lastOK := access.LastEvent()
	if !firstOK || !lastOK || first.Price != 20 || last.Price != 30 {
		t.Fatalf("table sorted access values = %#v", access.Entries())
	}
}

func TestAggregateIntoTableMaterializesUpdatesAndRemovesGroups(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "MaterializedPrices", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[SortedAccessValue[float64, runtimeTestTrade]]("sortcol"),
		TableColumnOf[int64]("eventCount"),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	sorted := SortedAccessBy[runtimeTestTrade, float64](EventValue[runtimeTestTrade](), price)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(3)).GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("sortcol", sorted),
		Alias("eventCount", CountAll()),
	).IntoTable("MaterializedPrices", StatementName("aggregate-into-table")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "A", Price: 10},
		{Symbol: "A", Price: 20},
		{Symbol: "B", Price: 5},
		{Symbol: "A", Price: 30},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	table, ok := engine.Table("MaterializedPrices")
	if !ok {
		t.Fatal("materialized table is missing")
	}
	row, ok, err := table.Get(context.Background(), "A")
	if err != nil || !ok {
		t.Fatalf("materialized row A = %#v, ok=%v, err=%v", row, ok, err)
	}
	if row.Get("eventCount").Any() != int64(2) {
		t.Fatalf("materialized update count = %#v", row.Values())
	}
	access, ok := row.Get("sortcol").Any().(SortedAccessValue[float64, runtimeTestTrade])
	if !ok {
		t.Fatalf("materialized sorted access type = %#v", row.Get("sortcol").Any())
	}
	first, firstOK := access.FirstEvent()
	last, lastOK := access.LastEvent()
	if !firstOK || !lastOK || first.Price != 20 || last.Price != 30 {
		t.Fatalf("materialized sorted access = %#v", access.Entries())
	}

	// The fourth event evicts A's oldest event; three further events evict the
	// remaining A event and must remove the table row rather than leave stale
	// aggregate state behind.
	for _, event := range []runtimeTestTrade{
		{Symbol: "C", Price: 6},
		{Symbol: "D", Price: 7},
		{Symbol: "E", Price: 8},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if _, found, err := table.Get(context.Background(), "A"); err != nil || found {
		t.Fatalf("evicted aggregate row A = found=%v, err=%v", found, err)
	}
	rows, err := table.Snapshot(context.Background())
	if err != nil || len(rows) != 3 {
		t.Fatalf("materialized table snapshot = %#v, err=%v", rows, err)
	}
}

func TestAggregateIntoTableValidatesTargetProjection(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "IncompleteMaterialized", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[int64]("requiredCount"),
	}); err != nil {
		t.Fatal(err)
	}
	query := From[runtimeTestTrade](env, "Trade").GroupBy(
		Field[runtimeTestTrade, string]("symbol"),
	).Select(Alias("symbol", Field[runtimeTestTrade, string]("symbol"))).IntoTable("IncompleteMaterialized")
	if _, err := env.Build(query); err == nil {
		t.Fatal("into-table accepted a missing required projection")
	}
	missing := From[runtimeTestTrade](env, "Trade").GroupBy(
		Field[runtimeTestTrade, string]("symbol"),
	).Select(
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
		Alias("requiredCount", CountAll()),
	).IntoTable("does-not-exist")
	if _, err := env.Build(missing); err == nil {
		t.Fatal("into-table accepted an unknown target")
	}
}

func TestAggregateIntoTableMaterializesRollupSubtotals(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "RollupMaterialized", []TableColumn{
		PrimaryKeyColumn[string]("groupKey"),
		OptionalTableColumnOf[string]("symbol"),
		OptionalTableColumnOf[float64]("price"),
		TableColumnOf[int64]("eventCount"),
	}); err != nil {
		t.Fatal(err)
	}
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	groupKey := Concat(
		Literal("symbol="),
		IfThenElse[string](IsNull[string](symbol), Literal("<all>"), symbol),
		Literal(";price="),
		IfThenElse[string](IsNull[float64](price), Literal("<all>"), Cast[float64, string](price)),
	)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").GroupByRollup(symbol, price).Select(
		Alias("groupKey", groupKey),
		Alias("symbol", symbol),
		Alias("price", price),
		Alias("eventCount", CountAll()),
	).IntoTable("RollupMaterialized", StatementName("aggregate-into-rollup-table")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "A", Price: 2},
		{Symbol: "A", Price: 3},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	table, ok := engine.Table("RollupMaterialized")
	if !ok {
		t.Fatal("rollup materialized table is missing")
	}
	rows, err := table.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("rollup materialized rows = %#v", rows)
	}
	byKey := make(map[string]TableRow, len(rows))
	for _, row := range rows {
		byKey[row.Get("groupKey").Any().(string)] = row
	}
	if got := byKey["symbol=A;price=2"]; got.Get("eventCount").Any() != int64(1) {
		t.Fatalf("rollup detail row = %#v", got.Values())
	}
	if got := byKey["symbol=A;price=3"]; got.Get("eventCount").Any() != int64(1) {
		t.Fatalf("rollup second detail row = %#v", got.Values())
	}
	if got := byKey["symbol=A;price=<all>"]; got.Get("price").State() != ValueNull || got.Get("eventCount").Any() != int64(2) {
		t.Fatalf("rollup symbol subtotal row = %#v", got.Values())
	}
	if got := byKey["symbol=<all>;price=<all>"]; got.Get("symbol").State() != ValueNull || got.Get("price").State() != ValueNull || got.Get("eventCount").Any() != int64(2) {
		t.Fatalf("rollup overall row = %#v", got.Values())
	}
}

func TestAggregateIntoTablePersistsFilteredAccessColumns(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "FilteredAccessMaterialized", []TableColumn{
		OptionalTableColumnOf[float64]("totalA"),
		OptionalTableColumnOf[WindowAccessValue[runtimeTestTrade]]("windowA"),
		OptionalTableColumnOf[SortedAccessValue[float64, runtimeTestTrade]]("sortedA"),
	}); err != nil {
		t.Fatal(err)
	}
	price := Field[runtimeTestTrade, float64]("price")
	symbol := Field[runtimeTestTrade, string]("symbol")
	aOnly := Like(symbol, Literal("A%"))
	window := WindowAccessBy[runtimeTestTrade](EventValue[runtimeTestTrade]())
	sorted := SortedAccessBy[runtimeTestTrade, float64](EventValue[runtimeTestTrade](), price)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(3)).Aggregate(
		Alias("totalA", FilterAggregate[float64](Sum[float64](price), aOnly)),
		Alias("windowA", FilterAggregate[WindowAccessValue[runtimeTestTrade]](window, aOnly)),
		Alias("sortedA", FilterAggregate[SortedAccessValue[float64, runtimeTestTrade]](sorted, aOnly)),
	).IntoTable("FilteredAccessMaterialized", StatementName("aggregate-into-filtered-access")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "X1", Price: 1},
		{Symbol: "A2", Price: 2},
		{Symbol: "A3", Price: 3},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	table, ok := engine.Table("FilteredAccessMaterialized")
	if !ok {
		t.Fatal("filtered access table is missing")
	}
	rows, err := table.Snapshot(context.Background())
	if err != nil || len(rows) != 1 {
		t.Fatalf("filtered access table rows = %#v, err=%v", rows, err)
	}
	row := rows[0]
	if row.Get("totalA").Any() != float64(5) {
		t.Fatalf("filtered access total = %#v", row.Values())
	}
	windowValue, ok := row.Get("windowA").Any().(WindowAccessValue[runtimeTestTrade])
	if !ok || !reflect.DeepEqual(windowValue.Values(), []runtimeTestTrade{{Symbol: "A2", Price: 2}, {Symbol: "A3", Price: 3}}) {
		t.Fatalf("filtered access window = %#v", row.Get("windowA").Any())
	}
	sortedValue, ok := row.Get("sortedA").Any().(SortedAccessValue[float64, runtimeTestTrade])
	if !ok || !reflect.DeepEqual(sortedValue.Values(), []runtimeTestTrade{{Symbol: "A2", Price: 2}, {Symbol: "A3", Price: 3}}) {
		t.Fatalf("filtered access sorted = %#v", row.Get("sortedA").Any())
	}
}

func TestRateAggregateUsesVirtualTime(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").GroupBy(
		Field[runtimeTestTrade, string]("symbol"),
	).Select(Alias("rate", Rate(time.Second))).Query(StatementName("rate")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rates []any
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				rates = append(rates, row.Get("rate").Any())
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Time{time.Unix(0, 0).UTC(), time.Unix(0, int64(500*time.Millisecond)).UTC(), time.Unix(2, 0).UTC()} {
		if !at.Equal(engine.Now()) {
			if err := engine.AdvanceTime(context.Background(), at); err != nil {
				t.Fatal(err)
			}
		}
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "X"}); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(rates, []any{nil, nil, 1.0}) {
		t.Fatalf("rates = %#v", rates)
	}
}

func TestRateAggregateUsesEverPointsAndFilter(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	symbol := Field[runtimeTestTrade, string]("symbol")
	filter := StartsWith(symbol, Literal("A"))
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(2)).Aggregate(
		Alias("rate", Rate(time.Second, filter)),
	).Query(StatementName("rate-filter")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rates := make([]any, 0, 4)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				rates = append(rates, row.Get("rate").Any())
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "X"}, {Symbol: "A"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "X"}); err != nil {
		t.Fatal(err)
	}
	// The final X arrives after the virtual clock has crossed the one-second
	// boundary. The A point is retained in the ever-state but is now outside
	// the interval, so the filtered rate is zero after a leave.
	if !reflect.DeepEqual(rates, []any{nil, nil, 0.0}) {
		t.Fatalf("filtered rates = %#v", rates)
	}
}

func TestTimestampRateAggregateUsesLeavingWindowAndQuantity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeRateEvent](env, "RateEvent"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	timestamp := Field[runtimeRateEvent, int64]("timestamp")
	quantity := Field[runtimeRateEvent, float64]("quantity")
	plan, err := env.Build(From[runtimeRateEvent](env, "RateEvent").Window(LengthWindow(3)).Aggregate(
		Alias("rate", RateByTimestamp[int64](timestamp)),
		Alias("quantityRate", RateQuantityByTimestamp[int64, float64](timestamp, quantity)),
	).Query(StatementName("timestamp-rate")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 5)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("timestamp rate result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeRateEvent{
		{Timestamp: 1000, Quantity: 10},
		{Timestamp: 1200, Quantity: 0},
		{Timestamp: 1300, Quantity: 0},
		{Timestamp: 1500, Quantity: 14},
		{Timestamp: 2000, Quantity: 11},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 5 {
		t.Fatalf("timestamp rate rows = %d", len(rows))
	}
	if rows[0].Get("rate").Any() != nil || rows[2].Get("rate").Any() != nil {
		t.Fatalf("timestamp rate became available before a window event left: %#v", rows)
	}
	if rows[3].Get("rate").Any() != float64(6) || rows[3].Get("quantityRate").Any() != float64(28) {
		t.Fatalf("timestamp rate at first eviction = %#v", rows[3].AsMap())
	}
	if rows[4].Get("rate").Any() != float64(3.75) || rows[4].Get("quantityRate").Any() != float64(31.25) {
		t.Fatalf("timestamp rate at second eviction = %#v", rows[4].AsMap())
	}
}

func TestTimestampRateAggregateHonorsFilter(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeRateEvent](env, "RateEvent"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	timestamp := Field[runtimeRateEvent, int64]("timestamp")
	quantity := Field[runtimeRateEvent, float64]("quantity")
	symbol := Field[runtimeRateEvent, string]("symbol")
	filter := StartsWith(symbol, Literal("A"))
	plan, err := env.Build(From[runtimeRateEvent](env, "RateEvent").Window(LengthWindow(3)).Aggregate(
		Alias("rate", RateByTimestamp[int64](timestamp, filter)),
		Alias("quantityRate", RateQuantityByTimestamp[int64, float64](timestamp, quantity, filter)),
	).Query(StatementName("timestamp-rate-filter")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 8)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("filtered timestamp rate result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeRateEvent{
		{Symbol: "X1", Timestamp: 1000, Quantity: 10},
		{Symbol: "X2", Timestamp: 1200},
		{Symbol: "X2", Timestamp: 1300},
		{Symbol: "A1", Timestamp: 1000, Quantity: 10},
		{Symbol: "A2", Timestamp: 1200},
		{Symbol: "A3", Timestamp: 1300},
		{Symbol: "A4", Timestamp: 1500, Quantity: 14},
		{Symbol: "A5", Timestamp: 2000, Quantity: 11},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 8 {
		t.Fatalf("filtered timestamp rate rows = %d", len(rows))
	}
	if rows[6].Get("rate").Any() != float64(6) || rows[6].Get("quantityRate").Any() != float64(28) {
		t.Fatalf("filtered timestamp rate at first matching eviction = %#v", rows[6].AsMap())
	}
	if rows[7].Get("rate").Any() != float64(3.75) || rows[7].Get("quantityRate").Any() != float64(31.25) {
		t.Fatalf("filtered timestamp rate at second matching eviction = %#v", rows[7].AsMap())
	}
}

func TestFilteredAggregatesReuseGroupRowsAndIgnoreNullPredicates(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	predicate := Greater[float64](price, Literal(5.0))
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").GroupBy(
		Field[runtimeTestTrade, string]("symbol"),
	).Select(
		Alias("count", CountIf(predicate)),
		Alias("sum", SumIf[float64](price, predicate)),
		Alias("avg", AvgIf[float64](price, predicate)),
		Alias("min", MinIf[float64](price, predicate)),
		Alias("max", MaxIf[float64](price, predicate)),
		Alias("distinct", FilterAggregate[int64](CountDistinct[float64](price), predicate)),
	).Query(StatementName("filtered-aggregate")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var last Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) == 0 {
			return nil
		}
		var ok bool
		last, ok = batch.New[len(batch.New)-1].Row()
		if !ok {
			t.Fatal("filtered aggregate result is not a row")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 2}, {Symbol: "A", Price: 8}, {Symbol: "A", Price: 10}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if last.Get("count").Any() != int64(2) || last.Get("sum").Any() != float64(18) || last.Get("avg").Any() != float64(9) || last.Get("min").Any() != float64(8) || last.Get("max").Any() != float64(10) || last.Get("distinct").Any() != int64(2) {
		t.Fatalf("filtered aggregate row = %#v", last.AsMap())
	}
}

func TestFilteredAggregateRejectsMissingPredicate(t *testing.T) {
	env, _ := newRuntimeTest(t)
	var predicate Expression[bool]
	query := From[runtimeTestTrade](env, "Trade").GroupBy(
		Field[runtimeTestTrade, string]("symbol"),
	).Select(Alias("bad", FilterAggregate[int64](CountAll(), predicate))).Query(StatementName("invalid-filtered-aggregate"))
	if _, err := env.Build(query); err == nil {
		t.Fatal("filtered aggregate with nil predicate unexpectedly built")
	}
}

func TestFilteredAggregateAccessMethodsReuseFilteredGroup(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	filter := StartsWith(symbol, Literal("A"))
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(3)).Aggregate(
		Alias("first", FilterAggregate[float64](First[float64](price), filter)),
		Alias("last", FilterAggregate[float64](Last[float64](price), filter)),
		Alias("window", FilterAggregate[[]float64](WindowValues[float64](price), filter)),
		Alias("sorted", FilterAggregate[[]float64](SortedValues[float64](price, false), filter)),
		Alias("minBy", FilterAggregate[runtimeTestTrade](MinBy[runtimeTestTrade, float64](EventValue[runtimeTestTrade](), price), filter)),
	).Query(StatementName("filtered-access")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 4)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "A1", Price: 10},
		{Symbol: "B1", Price: 1},
		{Symbol: "A2", Price: 5},
		{Symbol: "B2", Price: 2},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 4 {
		t.Fatalf("filtered access rows = %d", len(rows))
	}
	if rows[2].Get("first").Any() != float64(10) || rows[2].Get("last").Any() != float64(5) {
		t.Fatalf("filtered access first/last = %#v", rows[2].AsMap())
	}
	if got, ok := rows[2].Get("window").Any().([]float64); !ok || !reflect.DeepEqual(got, []float64{10, 5}) {
		t.Fatalf("filtered access window = %#v", rows[2].Get("window").Any())
	}
	if got, ok := rows[2].Get("sorted").Any().([]float64); !ok || !reflect.DeepEqual(got, []float64{5, 10}) {
		t.Fatalf("filtered access sorted = %#v", rows[2].Get("sorted").Any())
	}
	if minBy, ok := rows[2].Get("minBy").Any().(runtimeTestTrade); !ok || minBy.Price != 5 {
		t.Fatalf("filtered access min-by = %#v", rows[2].Get("minBy").Any())
	}
	if rows[3].Get("first").Any() != float64(5) || rows[3].Get("last").Any() != float64(5) {
		t.Fatalf("filtered access after eviction = %#v", rows[3].AsMap())
	}
}

func TestPluginAggregateEvaluatesCurrentAndEverGroups(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	rangeAggregate := PluginAggregate[float64]("price-range", func(ctx EvalContext) (float64, bool) {
		if len(ctx.Group) == 0 {
			return 0, false
		}
		minimum, maximum := math.Inf(1), math.Inf(-1)
		for _, event := range ctx.Group {
			price, ok := event.Get("price").Any().(float64)
			if !ok {
				continue
			}
			minimum = math.Min(minimum, price)
			maximum = math.Max(maximum, price)
		}
		return maximum - minimum, minimum != math.Inf(1)
	})
	everCount := PluginAggregate[int64]("ever-count", func(ctx EvalContext) (int64, bool) {
		return int64(len(ctx.EverGroup)), true
	})
	if err := RegisterAggregatePlugin[int64](env, "registered-count", func(ctx EvalContext) (int64, bool) {
		return int64(len(ctx.Group)), true
	}); err != nil {
		t.Fatal(err)
	}
	registeredCount := PluginAggregateRef[int64](env, "registered-count")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(2)).GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("priceRange", rangeAggregate),
		Alias("everCount", everCount),
		Alias("registeredCount", registeredCount),
	).Query(StatementName("plugin-aggregate")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var last Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok && row.Get("symbol").Any() == "A" {
				last = row
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 10}, {Symbol: "A", Price: 4}, {Symbol: "A", Price: 7}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if last.Get("priceRange").Any() != float64(3) || last.Get("everCount").Any() != int64(3) || last.Get("registeredCount").Any() != int64(2) {
		t.Fatalf("plugin aggregate row = %#v", last.AsMap())
	}
	if err := RegisterAggregatePlugin[int64](env, "registered-count", func(ctx EvalContext) (int64, bool) {
		return 0, true
	}); err == nil {
		t.Fatal("duplicate aggregate plugin registration succeeded")
	}

	invalid := From[runtimeTestTrade](env, "Trade").GroupBy(symbol).Select(
		Alias("bad", PluginAggregate[float64]("", nil)),
	).Query(StatementName("invalid-plugin-aggregate"))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("plugin aggregate with missing name/evaluator unexpectedly built")
	}
	unknown := From[runtimeTestTrade](env, "Trade").GroupBy(symbol).Select(
		Alias("unknown", PluginAggregateRef[float64](env, "unknown-plugin")),
	).Query(StatementName("unknown-plugin-aggregate"))
	if _, err := env.Build(unknown); err == nil {
		t.Fatal("unregistered aggregate plugin unexpectedly built")
	}
}

type testAggregatePluginConcatState struct {
	values []string
	leaves int
}

func (s *testAggregatePluginConcatState) Enter(value Value) {
	if text, ok := value.Any().(string); ok {
		s.values = append(s.values, text)
	}
}

func (s *testAggregatePluginConcatState) Leave(value Value) {
	s.leaves++
	text, ok := value.Any().(string)
	if !ok {
		return
	}
	for index, current := range s.values {
		if current == text {
			s.values = append(s.values[:index], s.values[index+1:]...)
			return
		}
	}
}

func (s *testAggregatePluginConcatState) Value() (string, bool) {
	if len(s.values) == 0 {
		return "", false
	}
	return strings.Join(s.values, " "), true
}

func (s *testAggregatePluginConcatState) Clear() {
	s.values = s.values[:0]
}

func TestPluginAggregateFactoryIsGroupScopedAndComposable(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	factoryCount := 0
	var states []*testAggregatePluginConcatState
	newFactory := func(ctx AggregatePluginFactoryContext) AggregatePluginState[string] {
		if ctx.Name == "" {
			t.Fatal("aggregate plugin factory context lost its name")
		}
		factoryCount++
		state := &testAggregatePluginConcatState{}
		states = append(states, state)
		return state
	}
	if err := RegisterAggregatePluginFactory[string](env, "registered-concat", newFactory); err != nil {
		t.Fatal(err)
	}
	label := Method[string](EventValue[runtimeTestTrade](), "PriceLabel")
	query := From[runtimeTestTrade](env, "Trade").Window(LengthWindow(2)).GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("direct", FilterAggregate[string](PluginAggregateWithFactory[string]("direct-concat", label, newFactory), StartsWith(symbol, Literal("A")))),
		Alias("registered", PluginAggregateFactoryRef[string](env, "registered-concat", label)),
	).Query(StatementName("plugin-aggregate-factory"), WithOldStream())
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "A", Price: 2}, {Symbol: "B", Price: 3}, {Symbol: "A", Price: 4}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 5 {
		t.Fatalf("factory aggregate rows = %d", len(rows))
	}
	var sawA2, sawB, sawA4 bool
	for _, row := range rows {
		switch row.Get("symbol").Any() {
		case "A":
			if row.Get("direct").Any() == "A:1 A:2" && row.Get("registered").Any() == "A:1 A:2" {
				sawA2 = true
			}
			if row.Get("direct").Any() == "A:4" && row.Get("registered").Any() == "A:4" {
				sawA4 = true
			}
		case "B":
			if !row.Get("direct").IsNull() || row.Get("registered").Any() != "B:3" {
				t.Fatalf("factory aggregate B row = %#v", row.AsMap())
			}
			sawB = true
		}
	}
	if !sawA2 || !sawA4 || !sawB {
		t.Fatalf("factory aggregate rows did not cover group/window transitions = %#v", rows)
	}
	if factoryCount != 4 {
		t.Fatalf("factory instances = %d, want one direct and one registered state per group", factoryCount)
	}
	leftTransitions := 0
	for _, state := range states {
		leftTransitions += state.leaves
	}
	if leftTransitions == 0 {
		t.Fatal("aggregate plugin factory never received leave values during window eviction")
	}

	if err := RegisterAggregatePluginFactory[string](env, "registered-concat", newFactory); err == nil {
		t.Fatal("duplicate aggregate plugin factory registration succeeded")
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("bad", PluginAggregateFactoryRef[string](env, "missing-factory", symbol)),
	).Query(StatementName("missing-plugin-factory"))); err == nil {
		t.Fatal("unknown aggregate plugin factory unexpectedly built")
	}
}

func TestPluginAggregateAccessWithNamedFilter(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	factory := func(_ AggregatePluginFactoryContext) AggregatePluginState[[]Event] {
		return &testAggregatePluginEventListState{}
	}
	query := From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("events", PluginAggregateAccess[[]Event]("events-as-list", nil, factory, StartsWith(symbol, Literal("A")))),
	).Query(StatementName("plugin-access-filter"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 4)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "X1", Price: 0},
		{Symbol: "A1", Price: 0},
		{Symbol: "A2", Price: 0},
		{Symbol: "X2", Price: 0},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 4 {
		t.Fatalf("plugin access rows = %d", len(rows))
	}
	wantSymbols := [][]string{{}, {"A1"}, {"A1", "A2"}, {"A1", "A2"}}
	for index, want := range wantSymbols {
		values, ok := rows[index].Get("events").Any().([]Event)
		if !ok {
			t.Fatalf("plugin access row %d type = %T", index, rows[index].Get("events").Any())
		}
		if len(values) != len(want) {
			t.Fatalf("plugin access row %d length = %d, want %d", index, len(values), len(want))
		}
		for valueIndex, event := range values {
			got, ok := event.Get("symbol").Any().(string)
			if !ok || got != want[valueIndex] {
				t.Fatalf("plugin access row %d event %d = %v, want %q", index, valueIndex, got, want[valueIndex])
			}
		}
	}
}

func TestRegisteredAggregateAccessPluginMatchesJavaAndPreservesCategory(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	eventFactory := func(_ AggregatePluginFactoryContext) AggregatePluginState[[]Event] {
		return &testAggregatePluginEventListState{}
	}
	if err := RegisterAggregateAccessPlugin[[]Event](env, "registered-events-as-list", eventFactory); err != nil {
		t.Fatal(err)
	}
	methodFactory := func(_ AggregatePluginFactoryContext) AggregatePluginState[string] {
		return &testAggregatePluginConcatState{}
	}
	if err := RegisterAggregatePluginFactory[string](env, "registered-method-only", methodFactory); err != nil {
		t.Fatal(err)
	}

	access := PluginAggregateAccessRef[[]Event](env, "registered-events-as-list", nil, StartsWith(symbol, Literal("A")))
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("events", access),
	).Query(StatementName("registered-plugin-access")))
	if err != nil {
		t.Fatal(err)
	}
	canonical := string(plan.Canonical())
	if !strings.Contains(canonical, "aggregate-plugin(registered-events-as-list:") || !strings.Contains(canonical, "access=true") || !strings.Contains(canonical, "access=false") {
		t.Fatalf("aggregate plugin category missing from plan canonical: %s", canonical)
	}
	second, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("events", PluginAggregateAccessRef[[]Event](env, "registered-events-as-list", nil, StartsWith(symbol, Literal("A")))),
	).Query(StatementName("registered-plugin-access")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != second.Hash() || !reflect.DeepEqual(plan.Canonical(), second.Canonical()) {
		t.Fatal("registered access plugin plan identity is unstable")
	}

	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("wrong", PluginAggregateFactoryRef[[]Event](env, "registered-events-as-list", nil)),
	).Query(StatementName("access-as-method"))); err == nil {
		t.Fatal("access plugin was accepted by the method-style factory reference")
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("wrong", PluginAggregateAccessRef[string](env, "registered-method-only", nil)),
	).Query(StatementName("method-as-access"))); err == nil {
		t.Fatal("method plugin was accepted by the access-style reference")
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("wrong", PluginAggregateAccessRef[[]Event](env, "registered-events-as-list", nil,
			StartsWith(symbol, Literal("A")), StartsWith(symbol, Literal("B")))),
	).Query(StatementName("access-multiple-filters"))); err == nil {
		t.Fatal("access plugin accepted multiple named-filter predicates")
	}
	if err := RegisterAggregateAccessPlugin[[]Event](env, "registered-events-as-list", eventFactory); err == nil {
		t.Fatal("duplicate access plugin registration succeeded")
	}

	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 4)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "X1"},
		{Symbol: "A1"},
		{Symbol: "A2"},
		{Symbol: "X2"},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	wantSymbols := [][]string{{}, {"A1"}, {"A1", "A2"}, {"A1", "A2"}}
	if len(rows) != len(wantSymbols) {
		t.Fatalf("registered access plugin rows = %d, want %d", len(rows), len(wantSymbols))
	}
	for rowIndex, want := range wantSymbols {
		values, ok := rows[rowIndex].Get("events").Any().([]Event)
		if !ok || len(values) != len(want) {
			t.Fatalf("registered access plugin row %d = %#v, want %v", rowIndex, rows[rowIndex].Get("events").Any(), want)
		}
		for valueIndex, event := range values {
			got, ok := event.Get("symbol").Any().(string)
			if !ok || got != want[valueIndex] {
				t.Fatalf("registered access plugin row %d event %d = %v, want %q", rowIndex, valueIndex, got, want[valueIndex])
			}
		}
	}
}

type testAggregatePluginEventListState struct {
	events []Event
}

func (state *testAggregatePluginEventListState) Enter(value Value) {
	event, err := As[Event](value)
	if err == nil {
		state.events = append(state.events, event)
	}
}

func (state *testAggregatePluginEventListState) Leave(value Value) {
	event, err := As[Event](value)
	if err != nil {
		return
	}
	identity := eventIdentity(event)
	for index, current := range state.events {
		if eventIdentity(current) == identity {
			state.events = append(state.events[:index], state.events[index+1:]...)
			return
		}
	}
}

func (state *testAggregatePluginEventListState) Value() ([]Event, bool) {
	return append([]Event(nil), state.events...), true
}

func (state *testAggregatePluginEventListState) Clear() {
	state.events = state.events[:0]
}

func TestCountMinSketchAggregateTracksFilteredFrequency(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	sketch := CountMinSketchAdd[string](symbol, Greater[float64](price, Literal(0.0)))
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("frequency", sketch.Frequency(symbol)),
		Alias("total", sketch.Total()),
	).Query(StatementName("count-min-sketch")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var last Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) == 0 {
			return nil
		}
		last, _ = batch.New[len(batch.New)-1].Row()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "hello", Price: 1},
		{Symbol: "hello", Price: 0},
		{Symbol: "hello", Price: 1},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if last.Get("frequency").Any() != int64(2) || last.Get("total").Any() != int64(2) {
		t.Fatalf("count-min-sketch result = %#v", last.AsMap())
	}
	if value := sketch.Frequency(symbol); value == nil {
		t.Fatal("count-min-sketch frequency expression is nil")
	}
}

func TestCountMinSketchTableColumnCanBeReadByTrigger(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "WordCountTable", []TableColumn{
		TableColumnOf[CountMinSketchValue[string]]("wordcms"),
	}); err != nil {
		t.Fatal(err)
	}
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	sketch := CountMinSketchAdd[string](symbol, Greater[float64](price, Literal(0.0)))
	aggregatePlan, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("wordcms", sketch),
	).IntoTable("WordCountTable", StatementName("a-count-min-sketch")))
	if err != nil {
		t.Fatal(err)
	}
	frequency := CountMinSketchFrequency[string](
		TableField[CountMinSketchValue[string]]("wordcms"),
		symbol,
	)
	triggerPlan, err := env.Build(OnEvent(From[runtimeTestTrade](env, "Trade")).SelectFromTableWhere(
		"WordCountTable", Literal(true), Alias("frequency", frequency),
	).Query(StatementName("b-count-min-sketch")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	triggerDeployment, err := engine.Deploy(context.Background(), triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var frequencies []int64
	if _, err := triggerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				frequencies = append(frequencies, row.Get("frequency").Any().(int64))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "hello", Price: 1},
		{Symbol: "hello", Price: 0},
		{Symbol: "name", Price: 1},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(frequencies, []int64{1, 1, 1}) {
		t.Fatalf("count-min-sketch table frequencies = %#v", frequencies)
	}
}

func TestWindowAccessTableMethodChainTracksEviction(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "WindowTable", []TableColumn{
		TableColumnOf[WindowAccessValue[runtimeTestTrade]]("windowcol"),
	}); err != nil {
		t.Fatal(err)
	}
	window := WindowAccessBy[runtimeTestTrade](EventValue[runtimeTestTrade]())
	aggregatePlan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(2)).Aggregate(
		Alias("windowcol", window),
	).IntoTable("WindowTable", StatementName("a-window-access")))
	if err != nil {
		t.Fatal(err)
	}
	windowField := TableField[WindowAccessValue[runtimeTestTrade]]("windowcol")
	triggerPlan, err := env.Build(OnEvent(From[runtimeTestTrade](env, "Trade")).SelectFromTableWhere(
		"WindowTable", Literal(true),
		Alias("first", Method[runtimeTestTrade](windowField, "First")),
		Alias("last", Method[runtimeTestTrade](windowField, "Last")),
		Alias("count", Method[int64](windowField, "CountEvents")),
		Alias("values", Method[[]runtimeTestTrade](windowField, "Values")),
	).Query(StatementName("b-window-access")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 3)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	events := []runtimeTestTrade{
		{Symbol: "A", Price: 1},
		{Symbol: "A", Price: 2},
		{Symbol: "A", Price: 3},
	}
	for _, event := range events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 3 {
		t.Fatalf("window access table rows = %d", len(rows))
	}
	assertWindowAccessRow := func(index int, first, last runtimeTestTrade, values []runtimeTestTrade) {
		if rows[index].Get("first").Any() != first || rows[index].Get("last").Any() != last || rows[index].Get("count").Any() != int64(len(values)) {
			t.Fatalf("window access row %d = %#v", index, rows[index].AsMap())
		}
		if got, ok := rows[index].Get("values").Any().([]runtimeTestTrade); !ok || !reflect.DeepEqual(got, values) {
			t.Fatalf("window access values %d = %#v", index, rows[index].Get("values").Any())
		}
	}
	assertWindowAccessRow(0, events[0], events[0], []runtimeTestTrade{events[0]})
	assertWindowAccessRow(1, events[0], events[1], []runtimeTestTrade{events[0], events[1]})
	assertWindowAccessRow(2, events[1], events[2], []runtimeTestTrade{events[1], events[2]})
}

func TestAggregateFirstLastWindowRecomputesAfterNamedWindowDelete(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[runtimeDeleteSignal](env, "DeleteSignal"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "aggregate-delete-window", schema); err != nil {
		t.Fatal(err)
	}
	source := From[runtimeTestTrade](env, "Trade")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow("aggregate-delete-window",
		SetColumn("symbol", Field[runtimeTestTrade, string]("symbol")),
		SetColumn("price", Field[runtimeTestTrade, float64]("price")),
	).Query(StatementName("a-aggregate-delete-insert")))
	if err != nil {
		t.Fatal(err)
	}
	symbol := Field[any, string]("symbol")
	aggregatePlan, err := env.Build(FromNamedWindow(env, "aggregate-delete-window").Aggregate(
		Alias("first", First[string](symbol)),
		Alias("window", WindowValues[string](symbol)),
		Alias("last", Last[string](symbol)),
	).Query(StatementName("b-aggregate-delete")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[runtimeDeleteSignal](env, "DeleteSignal")).DeleteFromNamedWindow(
		"aggregate-delete-window",
		Equal[string](NamedWindowField[string]("symbol"), Field[runtimeDeleteSignal, string]("id")),
	).Query(StatementName("c-aggregate-delete-remove")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	aggregateDeployment, err := engine.Deploy(context.Background(), aggregatePlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), deletePlan); err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 6)
	if _, err := aggregateDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "E1"}, {Symbol: "E2"}, {Symbol: "E3"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"E2", "E3", "E1"} {
		if err := engine.SendEvent(context.Background(), runtimeDeleteSignal{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 6 {
		t.Fatalf("aggregate delete rows = %d", len(rows))
	}
	if rows[3].Get("first").Any() != "E1" || rows[3].Get("last").Any() != "E3" {
		t.Fatalf("aggregate delete first removal = %#v", rows[3].AsMap())
	}
	if rows[4].Get("first").Any() != "E1" || rows[4].Get("last").Any() != "E1" {
		t.Fatalf("aggregate delete second removal = %#v", rows[4].AsMap())
	}
	if rows[5].Get("first").Any() != nil || rows[5].Get("window").Any() != nil || rows[5].Get("last").Any() != nil {
		t.Fatalf("aggregate delete empty result = %#v", rows[5].AsMap())
	}
}

func TestEverAggregatesRetainHistoryAfterWindowEviction(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(2)).GroupBy(
		Field[runtimeTestTrade, string]("symbol"),
	).Select(
		Alias("currentCount", CountAll()),
		Alias("everCount", CountEver()),
		Alias("everValueCount", CountEver(price)),
		Alias("firstEver", FirstEver[float64](price)),
		Alias("lastEver", LastEver[float64](price)),
	).Query(StatementName("ever-aggregate")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var last Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) == 0 {
			return nil
		}
		var ok bool
		last, ok = batch.New[len(batch.New)-1].Row()
		if !ok {
			t.Fatal("ever aggregate result is not a row")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, priceValue := range []float64{1, 2, 3} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: priceValue}); err != nil {
			t.Fatal(err)
		}
	}
	if last.Get("currentCount").Any() != int64(2) || last.Get("everCount").Any() != int64(3) || last.Get("everValueCount").Any() != int64(3) || last.Get("firstEver").Any() != float64(1) || last.Get("lastEver").Any() != float64(3) {
		t.Fatalf("ever aggregate row = %#v", last.AsMap())
	}
}

func TestRollupAndGroupingIDProduceSubtotalLevels(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").GroupByRollup(symbol, price).Select(
		Alias("symbol", symbol),
		Alias("price", price),
		Alias("groupingSymbol", Grouping(symbol)),
		Alias("groupingPrice", Grouping(price)),
		Alias("groupingID", GroupingID(symbol, price)),
		Alias("count", CountAll()),
	).Query(StatementName("rollup")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("rollup result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rollup rows = %#v", rows)
	}
	byID := make(map[int64]Row, len(rows))
	for _, row := range rows {
		byID[row.Get("groupingID").Any().(int64)] = row
	}
	if got := byID[0]; got.Get("symbol").Any() != "A" || got.Get("price").Any() != float64(2) || got.Get("count").Any() != int64(1) || got.Get("groupingSymbol").Any() != int64(0) || got.Get("groupingPrice").Any() != int64(0) {
		t.Fatalf("rollup detail row = %#v", got.AsMap())
	}
	if got := byID[1]; got.Get("symbol").Any() != "A" || got.Get("price").State() != ValueNull || got.Get("count").Any() != int64(1) || got.Get("groupingPrice").Any() != int64(1) {
		t.Fatalf("rollup subtotal row = %#v", got.AsMap())
	}
	if got := byID[3]; got.Get("symbol").State() != ValueNull || got.Get("price").State() != ValueNull || got.Get("count").Any() != int64(1) || got.Get("groupingSymbol").Any() != int64(1) {
		t.Fatalf("rollup overall row = %#v", got.AsMap())
	}
}

func TestCubeAndExplicitGroupingSetsKeepIndependentGroups(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").GroupByCube(symbol, price).Select(
		Alias("symbol", symbol),
		Alias("price", price),
		Alias("groupingID", GroupingID(symbol, price)),
		Alias("count", CountAll()),
	).Query(StatementName("cube")))
	if err != nil {
		t.Fatal(err)
	}
	setPlan, err := env.Build(From[runtimeTestTrade](env, "Trade").GroupByGroupingSets(
		GroupingSet(symbol),
		GroupingSet(price),
		GroupingSet(),
	).Select(
		Alias("symbol", symbol),
		Alias("price", price),
		Alias("count", CountAll()),
	).Query(StatementName("grouping-sets")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	setDeployment, err := engine.Deploy(context.Background(), setPlan)
	if err != nil {
		t.Fatal(err)
	}
	var cubeRows, setRows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				cubeRows = append(cubeRows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := setDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				setRows = append(setRows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if len(cubeRows) != 4 {
		t.Fatalf("cube rows = %#v", cubeRows)
	}
	if len(setRows) != 3 {
		t.Fatalf("grouping-set rows = %#v", setRows)
	}
	seenSetShapes := make(map[string]struct{}, len(setRows))
	for _, row := range setRows {
		seenSetShapes[fmt.Sprintf("%v/%v", row.Get("symbol").Any(), row.Get("price").Any())] = struct{}{}
	}
	for _, expected := range []string{"A/<nil>", "<nil>/2", "<nil>/<nil>"} {
		if _, ok := seenSetShapes[expected]; !ok {
			t.Fatalf("grouping-set shape %q missing from %#v", expected, seenSetShapes)
		}
	}
}

func TestDimensionalGroupingRejectsDuplicateAndEmptyDefinitions(t *testing.T) {
	env, _ := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	cases := []Query{
		From[runtimeTestTrade](env, "Trade").GroupByRollup(symbol, symbol).Select(Alias("count", CountAll())).Query(),
		From[runtimeTestTrade](env, "Trade").GroupByCube().Select(Alias("count", CountAll())).Query(),
		From[runtimeTestTrade](env, "Trade").GroupByGroupingSets(GroupingSet()).Select(Alias("count", CountAll())).Query(),
		From[runtimeTestTrade](env, "Trade").GroupByGroupingSets(GroupingSet(symbol), GroupingSet(symbol)).Select(Alias("count", CountAll())).Query(),
		From[runtimeTestTrade](env, "Trade").GroupByGroupingSets(GroupingSet(symbol, price, symbol)).Select(Alias("count", CountAll())).Query(),
	}
	for index, query := range cases {
		if _, err := env.Build(query); err == nil {
			t.Fatalf("dimensional grouping case %d unexpectedly built", index)
		}
	}
}

func TestRollupFireAndForgetNamedWindowSnapshot(t *testing.T) {
	env, _ := newRuntimeTest(t)
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "Trades", schema); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if err := engine.InsertNamedWindow(context.Background(), "Trades", runtimeTestTrade{Symbol: "A", Price: 2}); err != nil {
		t.Fatal(err)
	}
	symbol := Field[any, string]("symbol")
	price := Field[any, float64]("price")
	plan, err := env.Build(FromNamedWindow(env, "Trades").GroupByRollup(symbol, price).Select(
		Alias("symbol", symbol),
		Alias("price", price),
		Alias("groupingID", GroupingID(symbol, price)),
		Alias("count", CountAll()),
	).Query(StatementName("rollup-faf")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 3 {
		t.Fatalf("rollup FAF results = %#v", result.Results())
	}
	seen := make(map[int64]struct{}, len(result.Results()))
	for _, item := range result.Results() {
		row, ok := item.Row()
		if !ok {
			t.Fatalf("rollup FAF result is not a row: %#v", item)
		}
		seen[row.Get("groupingID").Any().(int64)] = struct{}{}
	}
	for _, groupingID := range []int64{0, 1, 3} {
		if _, ok := seen[groupingID]; !ok {
			t.Fatalf("rollup FAF grouping id %d missing from %#v", groupingID, seen)
		}
	}
}

func TestFireAndForgetAggregateAccessSnapshotAndGrouping(t *testing.T) {
	env, _ := newRuntimeTest(t)
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "aggregate-faf-window", schema); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, event := range []runtimeTestTrade{
		{Symbol: "E1", Price: 10},
		{Symbol: "E2", Price: 20},
		{Symbol: "E3", Price: 30},
		{Symbol: "E3", Price: 31},
		{Symbol: "E1", Price: 11},
		{Symbol: "E1", Price: 12},
	} {
		if err := engine.InsertNamedWindow(context.Background(), "aggregate-faf-window", event); err != nil {
			t.Fatal(err)
		}
	}
	symbol := Field[any, string]("symbol")
	price := Field[any, float64]("price")
	plan, err := env.Build(FromNamedWindow(env, "aggregate-faf-window").Aggregate(
		Alias("first", First[float64](price)),
		Alias("window", WindowValues[float64](price)),
		Alias("last", Last[float64](price)),
	).Query(StatementName("aggregate-faf-access")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 1 {
		t.Fatalf("aggregate FAF result count = %d", len(result.Results()))
	}
	row, ok := result.Results()[0].Row()
	if !ok || row.Get("first").Any() != float64(10) || row.Get("last").Any() != float64(12) {
		t.Fatalf("aggregate FAF access row = %#v", result.Results())
	}
	if values, ok := row.Get("window").Any().([]float64); !ok || !reflect.DeepEqual(values, []float64{10, 20, 30, 31, 11, 12}) {
		t.Fatalf("aggregate FAF window = %#v", row.Get("window").Any())
	}
	grouped, err := env.Build(FromNamedWindow(env, "aggregate-faf-window").GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("first", First[float64](price)),
		Alias("window", WindowValues[float64](price)),
		Alias("last", Last[float64](price)),
	).Query(OrderBy(Ascending(symbol)), StatementName("aggregate-faf-grouped")))
	if err != nil {
		t.Fatal(err)
	}
	groupedResult, err := engine.ExecuteFireAndForget(context.Background(), grouped)
	if err != nil {
		t.Fatal(err)
	}
	if len(groupedResult.Results()) != 3 {
		t.Fatalf("grouped aggregate FAF result count = %d", len(groupedResult.Results()))
	}
	groupedRows := make([]Row, 0, 3)
	for _, result := range groupedResult.Results() {
		row, ok := result.Row()
		if !ok {
			t.Fatalf("grouped aggregate FAF result is not a row: %#v", result)
		}
		groupedRows = append(groupedRows, row)
	}
	if groupedRows[0].Get("symbol").Any() != "E1" || groupedRows[0].Get("first").Any() != float64(10) || groupedRows[0].Get("last").Any() != float64(12) {
		t.Fatalf("grouped aggregate FAF first row = %#v", groupedRows[0].AsMap())
	}
}
