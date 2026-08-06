package esper

import (
	"context"
	"errors"
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

type testAggregateMultiStateExtended struct {
	id   int
	rows [][]Value
}

func (state *testAggregateMultiStateExtended) Enter(values []Value) {
	state.rows = append(state.rows, append([]Value(nil), values...))
}

func (state *testAggregateMultiStateExtended) Leave(values []Value) {
	for index, current := range state.rows {
		if reflect.DeepEqual(current, values) {
			state.rows = append(state.rows[:index], state.rows[index+1:]...)
			return
		}
	}
}

func (state *testAggregateMultiStateExtended) Value(method string) (any, bool) {
	switch method {
	case "count":
		return int64(len(state.rows)), true
	case "sum":
		var sum float64
		for _, row := range state.rows {
			if len(row) == 0 {
				continue
			}
			if value, ok := numericValue(row[0]); ok {
				sum += value
			}
		}
		return sum, true
	case "vectorWidth":
		if len(state.rows) == 0 {
			return int64(0), true
		}
		return int64(len(state.rows[len(state.rows)-1])), true
	case "instance":
		return int64(state.id), true
	case "se1", "se2":
		if len(state.rows) == 0 || len(state.rows[len(state.rows)-1]) == 0 {
			return nil, false
		}
		return state.rows[len(state.rows)-1][0].Any(), true
	default:
		return nil, false
	}
}

func (state *testAggregateMultiStateExtended) Clear() {
	state.rows = nil
}

func TestAggregateMultiPluginRegisteredLifecycleAndSharing(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	input := AggregatePluginInputs(price, symbol)
	methods := []AggregateMultiPluginMethod{
		AggregateMultiMethod[int64]("count", "numbers"),
		AggregateMultiMethod[float64]("sum", "numbers"),
		AggregateMultiMethod[int64]("vectorWidth"),
		AggregateMultiMethod[Event]("se1", "single"),
		AggregateMultiMethod[Event]("se2", "single"),
	}
	factoryCount := 0
	factory := func(ctx AggregateMultiPluginFactoryContext) AggregateMultiPluginState {
		factoryCount++
		if ctx.Engine == nil {
			t.Fatal("multi plugin factory context lost its engine")
		}
		for _, method := range []string{"count", "sum", "vectorWidth", "se1", "se2"} {
			if _, ok := ctx.Methods[method]; !ok {
				t.Fatalf("multi plugin factory did not receive method %q", method)
			}
		}
		return &testAggregateMultiStateExtended{id: factoryCount}
	}
	if err := RegisterAggregateMultiPlugin(env, "registered-multi", methods, factory); err != nil {
		t.Fatal(err)
	}
	filtered := FilterAggregate[int64](
		PluginAggregateMultiRef[int64](env, "registered-multi", "count", input),
		StartsWith(symbol, Literal("A")),
	)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(2)).GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("count", PluginAggregateMultiRef[int64](env, "registered-multi", "count", input)),
		Alias("sum", PluginAggregateMultiRef[float64](env, "registered-multi", "sum", input)),
		Alias("width", PluginAggregateMultiRef[int64](env, "registered-multi", "vectorWidth", input)),
		Alias("filtered", filtered),
		Alias("event1", PluginAggregateMultiRef[Event](env, "registered-multi", "se1", nil)),
		Alias("event2", PluginAggregateMultiRef[Event](env, "registered-multi", "se2", nil)),
	).Query(StatementName("registered-multi-lifecycle")))
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
		{Symbol: "A", Price: 1},
		{Symbol: "A", Price: 4},
		{Symbol: "B", Price: 10},
		{Symbol: "A", Price: 7},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 5 {
		t.Fatalf("multi plugin rows = %d", len(rows))
	}
	for _, row := range rows {
		if row.Get("width").Any() != int64(2) {
			t.Fatalf("multi plugin vector width = %#v", row.AsMap())
		}
		event1, ok := row.Get("event1").Any().(Event)
		if !ok {
			t.Fatalf("multi plugin event1 type = %T", row.Get("event1").Any())
		}
		event2, ok := row.Get("event2").Any().(Event)
		if !ok || !sameEvent(event1, event2) {
			t.Fatalf("multi plugin shared event state = %#v", row.AsMap())
		}
		switch row.Get("symbol").Any() {
		case "A":
			if row.Get("filtered").Any() != row.Get("count").Any() {
				t.Fatalf("filtered A row = %#v", row.AsMap())
			}
			count, sum := row.Get("count").Any(), row.Get("sum").Any()
			switch sum {
			case float64(1), float64(4), float64(7):
				if count != int64(1) {
					t.Fatalf("A single-row count = %#v", row.AsMap())
				}
			case float64(5):
				if count != int64(2) {
					t.Fatalf("A two-row count = %#v", row.AsMap())
				}
			default:
				t.Fatalf("A sum = %#v", row.AsMap())
			}
		case "B":
			if row.Get("count").Any() != int64(1) || row.Get("sum").Any() != float64(10) || row.Get("filtered").Any() != int64(0) {
				t.Fatalf("B row = %#v", row.AsMap())
			}
		default:
			t.Fatalf("unexpected multi plugin group = %#v", row.AsMap())
		}
	}
	// numbers and single share one state per group; vectorWidth and the
	// filtered scope each have their own state per group.
	if factoryCount != 8 {
		t.Fatalf("multi plugin factory instances = %d, want 8", factoryCount)
	}
	if err := RegisterAggregateMultiPlugin(env, "registered-multi", methods, factory); err == nil {
		t.Fatal("duplicate multi plugin registration succeeded")
	}
}

func TestAggregateMultiPluginInlineIsolationAndPlanIdentity(t *testing.T) {
	env, engine := newRuntimeTest(t)
	methods := []AggregateMultiPluginMethod{AggregateMultiMethod[int64]("instance")}
	factoryCount := 0
	factory := func(AggregateMultiPluginFactoryContext) AggregateMultiPluginState {
		factoryCount++
		return &testAggregateMultiStateExtended{id: factoryCount}
	}
	left := PluginAggregateMulti[int64]("inline-multi", "instance", nil, factory, methods...)
	right := PluginAggregateMulti[int64]("inline-multi", "instance", nil, factory, methods...)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(2)).Aggregate(
		Alias("left", left),
		Alias("right", right),
	).Query(StatementName("inline-multi-isolation")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plan.Canonical()), "aggregate-multi-plugin(inline-multi.instance") {
		t.Fatalf("inline multi plugin missing from canonical plan: %s", plan.Canonical())
	}
	second, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(2)).Aggregate(
		Alias("left", PluginAggregateMulti[int64]("inline-multi", "instance", nil, factory, methods...)),
		Alias("right", PluginAggregateMulti[int64]("inline-multi", "instance", nil, factory, methods...)),
	).Query(StatementName("inline-multi-isolation")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != second.Hash() || !reflect.DeepEqual(plan.Canonical(), second.Canonical()) {
		t.Fatal("inline multi plugin plan identity is unstable")
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var last Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			var ok bool
			last, ok = batch.New[len(batch.New)-1].Row()
			if !ok {
				return fmt.Errorf("inline multi result is not a row")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if last.Get("left").Any() != int64(1) || last.Get("right").Any() != int64(2) {
		t.Fatalf("inline multi default state sharing = %#v", last.AsMap())
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if last.Get("left").Any() != int64(1) || last.Get("right").Any() != int64(2) || factoryCount != 2 {
		t.Fatalf("inline multi state lifecycle = %#v, factories=%d", last.AsMap(), factoryCount)
	}
}

func TestAggregateMultiPluginValidationBoundaries(t *testing.T) {
	env, _ := newRuntimeTest(t)
	methods := []AggregateMultiPluginMethod{
		AggregateMultiMethod[int64]("count", "shared"),
	}
	factory := func(AggregateMultiPluginFactoryContext) AggregateMultiPluginState {
		return &testAggregateMultiStateExtended{}
	}
	if err := RegisterAggregateMultiPlugin(nil, "multi", methods, factory); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("nil environment registration error = %v", err)
	}
	if err := RegisterAggregateMultiPlugin(env, "", methods, factory); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("blank provider registration error = %v", err)
	}
	if err := RegisterAggregateMultiPlugin(env, "no-factory", methods, nil); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("nil factory registration error = %v", err)
	}
	if err := RegisterAggregateMultiPlugin(env, "no-methods", nil, factory); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("empty method registration error = %v", err)
	}
	if err := RegisterAggregateMultiPlugin(env, "duplicate-method", []AggregateMultiPluginMethod{
		AggregateMultiMethod[int64]("same"), AggregateMultiMethod[int64]("same"),
	}, factory); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("duplicate method registration error = %v", err)
	}
	if err := RegisterAggregateMultiPlugin(env, "missing-result", []AggregateMultiPluginMethod{{Name: "value"}}, factory); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("missing method result type error = %v", err)
	}
	if err := RegisterAggregateMultiPlugin(env, "registered-multi-validation", methods, factory); err != nil {
		t.Fatal(err)
	}
	if err := RegisterAggregatePlugin[int64](env, "registered-multi-validation", func(EvalContext) (int64, bool) { return 1, true }); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("cross-category registration error = %v", err)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("unknown-provider", PluginAggregateMultiRef[int64](env, "missing", "count", nil)),
	).Query(StatementName("invalid-multi-provider"))); err == nil || !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("unknown multi provider Build error = %v", err)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("unknown-method", PluginAggregateMultiRef[int64](env, "registered-multi-validation", "missing", nil)),
	).Query(StatementName("invalid-multi-method"))); err == nil || !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("unknown multi method Build error = %v", err)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("wrong-type", PluginAggregateMultiRef[string](env, "registered-multi-validation", "count", nil)),
	).Query(StatementName("invalid-multi-type"))); err == nil || !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("multi result type Build error = %v", err)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("nil-factory", PluginAggregateMulti[int64]("inline", "count", nil, nil, methods...)),
	).Query(StatementName("invalid-multi-factory"))); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("inline nil factory Build error = %v", err)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("duplicate-inline", PluginAggregateMulti[int64]("inline", "count", nil, factory,
			AggregateMultiMethod[int64]("count"), AggregateMultiMethod[int64]("count"))),
	).Query(StatementName("invalid-inline-methods"))); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("inline duplicate method Build error = %v", err)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("nested", PluginAggregateMultiRef[int64](env, "registered-multi-validation", "count", AggregatePluginInputs(CountAll()))),
	).Query(StatementName("invalid-multi-nested-aggregate"))); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("nested multi aggregate Build error = %v", err)
	}
	env2, _ := newRuntimeTest(t)
	if err := RegisterAggregateMultiPlugin(env2, "registered-multi-validation", methods, factory); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("foreign", PluginAggregateMultiRef[int64](env2, "registered-multi-validation", "count", nil)),
	).Query(StatementName("invalid-multi-environment"))); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("foreign multi environment Build error = %v", err)
	}
}

type runtimeRateEvent struct {
	Symbol    string  `esper:"symbol"`
	Timestamp int64   `esper:"timestamp"`
	Quantity  float64 `esper:"quantity"`
}

type runtimeDeleteSignal struct {
	ID string `esper:"id"`
}

type sortedAccessTableTrigger struct {
	Price float64 `esper:"price"`
}

type sortedGroupedTableTrigger struct {
	Symbol string `esper:"symbol"`
}

func sortedTableMethod[T any](field Expr, name string, arguments ...Expr) Expression[T] {
	return Method[T](field, name, arguments...)
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

func TestSortedAccessMultiCriteriaMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	sorted := SortedAccessByMulti[runtimeTestTrade, string, float64](EventValue[runtimeTestTrade](), symbol, price)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(8)).Aggregate(
		Alias("firstKey", sorted.FirstKey()),
		Alias("lastKey", sorted.LastKey()),
		Alias("lowerKey", sorted.LowerKey(Literal(NewSortedMultiKey("E4", 1.0)))),
		Alias("higherKey", sorted.HigherKey(Literal(NewSortedMultiKey("E4b", -1.0)))),
	).Query(StatementName("sorted-access-multi-criteria")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var last Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			var ok bool
			last, ok = batch.New[len(batch.New)-1].Row()
			if !ok {
				return fmt.Errorf("multi-criteria sorted result is not a row")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "E1a", Price: 1},
		{Symbol: "E1b", Price: 1},
		{Symbol: "E4b", Price: 4},
		{Symbol: "E6a", Price: 6},
		{Symbol: "E6b", Price: 6},
		{Symbol: "E8", Price: 8},
		{Symbol: "E9", Price: 9},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	assertKey := func(name, wantSymbol string, wantPrice float64) {
		value := last.Get(name)
		key, ok := value.Any().(SortedMultiKey)
		if !ok {
			t.Fatalf("%s = %#v, want SortedMultiKey", name, value.Any())
		}
		parts := key.Parts()
		if len(parts) != 2 || parts[0] != wantSymbol || parts[1] != wantPrice {
			t.Fatalf("%s parts = %#v, want [%q %v]", name, parts, wantSymbol, wantPrice)
		}
	}
	assertKey("firstKey", "E1a", 1)
	assertKey("lastKey", "E9", 9)
	assertKey("lowerKey", "E1b", 1)
	assertKey("higherKey", "E4b", 4)
	if !reflect.DeepEqual(NewSortedMultiKey("E4b", 4.0).Parts(), []any{"E4b", 4.0}) {
		t.Fatalf("multi-key defensive parts = %#v", NewSortedMultiKey("E4b", 4.0).Parts())
	}
	var missingPrice Expression[float64]
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("invalid", SortedAccessByMulti[runtimeTestTrade, string, float64](EventValue[runtimeTestTrade](), symbol, missingPrice).FirstKey()),
	).Query(StatementName("invalid-sorted-access-multi"))); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("invalid sorted multi-key Build error = %v", err)
	}
}

func TestSortedAccessEventBucketNavigationMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	sorted := SortedAccessBy[runtimeTestTrade, float64](EventValue[runtimeTestTrade](), price)
	lookup := Literal(15.0)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(4)).Aggregate(
		Alias("lowerEvent", sorted.LowerEvent(lookup)),
		Alias("floorEvent", sorted.FloorEvent(lookup)),
		Alias("higherEvent", sorted.HigherEvent(lookup)),
		Alias("ceilingEvent", sorted.CeilingEvent(lookup)),
		Alias("lowerEvents", sorted.LowerEvents(lookup)),
		Alias("floorEvents", sorted.FloorEvents(lookup)),
		Alias("higherEvents", sorted.HigherEvents(lookup)),
		Alias("ceilingEvents", sorted.CeilingEvents(lookup)),
		Alias("lowerLast", EnumLastOf[runtimeTestTrade](sorted.LowerEvents(lookup))),
		Alias("floorLast", EnumLastOf[runtimeTestTrade](sorted.FloorEvents(lookup))),
		Alias("higherLast", EnumLastOf[runtimeTestTrade](sorted.HigherEvents(lookup))),
		Alias("ceilingLast", EnumLastOf[runtimeTestTrade](sorted.CeilingEvents(lookup))),
	).Query(StatementName("sorted-access-event-buckets")))
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
		{Symbol: "A", Price: 10},
		{Symbol: "B", Price: 10},
		{Symbol: "C", Price: 20},
		{Symbol: "D", Price: 30},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	assertEvent := func(name, want string) {
		value := last.Get(name)
		if want == "" {
			if !value.IsNull() {
				t.Fatalf("%s = %#v, want null", name, value.Any())
			}
			return
		}
		got, ok := value.Any().(runtimeTestTrade)
		if !ok || got.Symbol != want {
			t.Fatalf("%s = %#v, want symbol %q", name, value.Any(), want)
		}
	}
	assertBucket := func(name string, want ...string) {
		value := last.Get(name)
		if len(want) == 0 {
			if !value.IsNull() {
				t.Fatalf("%s = %#v, want null", name, value.Any())
			}
			return
		}
		values, ok := value.Any().([]runtimeTestTrade)
		if !ok || len(values) != len(want) {
			t.Fatalf("%s = %#v, want %v", name, value.Any(), want)
		}
		for index, symbol := range want {
			if values[index].Symbol != symbol {
				t.Fatalf("%s[%d] = %#v, want symbol %q", name, index, values[index], symbol)
			}
		}
	}
	assertEvent("lowerEvent", "A")
	assertEvent("floorEvent", "A")
	assertEvent("higherEvent", "C")
	assertEvent("ceilingEvent", "C")
	assertBucket("lowerEvents", "A", "B")
	assertBucket("floorEvents", "A", "B")
	assertBucket("higherEvents", "C")
	assertBucket("ceilingEvents", "C")
	assertEvent("lowerLast", "B")
	assertEvent("floorLast", "B")
	assertEvent("higherLast", "C")
	assertEvent("ceilingLast", "C")

	for _, test := range []struct {
		name                          string
		key                           float64
		lower, floor, higher, ceiling []string
	}{
		{name: "below-first", key: 5, higher: []string{"A", "B"}, ceiling: []string{"A", "B"}},
		{name: "exact-first", key: 10, floor: []string{"A", "B"}, ceiling: []string{"A", "B"}, higher: []string{"C"}},
		{name: "between", key: 15, lower: []string{"A", "B"}, floor: []string{"A", "B"}, higher: []string{"C"}, ceiling: []string{"C"}},
		{name: "exact-second", key: 20, lower: []string{"A", "B"}, floor: []string{"C"}, higher: []string{"D"}, ceiling: []string{"C"}},
		{name: "between-last", key: 25, lower: []string{"C"}, floor: []string{"C"}, higher: []string{"D"}, ceiling: []string{"D"}},
		{name: "exact-last", key: 30, lower: []string{"C"}, floor: []string{"D"}, ceiling: []string{"D"}},
		{name: "above-last", key: 35, lower: []string{"D"}, floor: []string{"D"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			boundaryEnv, boundaryEngine := newRuntimeTest(t)
			boundaryPrice := Field[runtimeTestTrade, float64]("price")
			boundarySorted := SortedAccessBy[runtimeTestTrade, float64](EventValue[runtimeTestTrade](), boundaryPrice)
			boundaryKey := Literal(test.key)
			boundaryPlan, err := boundaryEnv.Build(From[runtimeTestTrade](boundaryEnv, "Trade").Window(LengthWindow(4)).Aggregate(
				Alias("lower", boundarySorted.LowerEvents(boundaryKey)),
				Alias("floor", boundarySorted.FloorEvents(boundaryKey)),
				Alias("higher", boundarySorted.HigherEvents(boundaryKey)),
				Alias("ceiling", boundarySorted.CeilingEvents(boundaryKey)),
			).Query(StatementName("sorted-access-boundary-" + test.name)))
			if err != nil {
				t.Fatal(err)
			}
			boundaryDeployment, err := boundaryEngine.Deploy(context.Background(), boundaryPlan)
			if err != nil {
				t.Fatal(err)
			}
			var boundaryRow Row
			if _, err := boundaryDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				if len(batch.New) != 0 {
					boundaryRow, _ = batch.New[len(batch.New)-1].Row()
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
				if err := boundaryEngine.SendEvent(context.Background(), event); err != nil {
					t.Fatal(err)
				}
			}
			assertBoundaryBucket := func(name string, want []string) {
				value := boundaryRow.Get(name)
				if len(want) == 0 {
					if !value.IsNull() {
						t.Fatalf("%s = %#v, want null", name, value.Any())
					}
					return
				}
				values, ok := value.Any().([]runtimeTestTrade)
				if !ok || len(values) != len(want) {
					t.Fatalf("%s = %#v, want %v", name, value.Any(), want)
				}
				for index, symbol := range want {
					if values[index].Symbol != symbol {
						t.Fatalf("%s[%d] = %#v, want symbol %q", name, index, values[index], symbol)
					}
				}
			}
			assertBoundaryBucket("lower", test.lower)
			assertBoundaryBucket("floor", test.floor)
			assertBoundaryBucket("higher", test.higher)
			assertBoundaryBucket("ceiling", test.ceiling)
		})
	}
}

func TestSortedAccessValueNavigableSnapshotAndIteratorMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	sorted := SortedAccessBy[runtimeTestTrade, float64](EventValue[runtimeTestTrade](), price)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(5)).Aggregate(
		Alias("map", sorted.NavigableMapReference()),
	).Query(StatementName("sorted-access-navigable-snapshot")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var last Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			last, _ = batch.New[len(batch.New)-1].Row()
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
	access, ok := last.Get("map").Any().(SortedAccessValue[float64, runtimeTestTrade])
	if !ok || access.IsEmpty() {
		t.Fatalf("sorted access snapshot = %#v", last.Get("map").Any())
	}
	if !reflect.DeepEqual(access.Keys(), []float64{10, 20, 30}) {
		t.Fatalf("sorted access keys = %#v", access.Keys())
	}
	if entry, found := access.FirstEntry(); !found || entry.Key != 10 || len(entry.Values) != 2 || entry.Values[0].Symbol != "A" {
		t.Fatalf("first sorted access entry = %#v, found=%t", entry, found)
	}
	if entry, found := access.LastEntry(); !found || entry.Key != 30 || len(entry.Values) != 1 || entry.Values[0].Symbol != "D" {
		t.Fatalf("last sorted access entry = %#v, found=%t", entry, found)
	}
	if entry, found := access.Entry(20); !found || len(entry.Values) != 1 || entry.Values[0].Symbol != "C" {
		t.Fatalf("exact sorted access entry = %#v, found=%t", entry, found)
	}
	buckets := access.Buckets()
	if len(buckets) != 3 || len(buckets[0]) != 2 || buckets[0][0].Symbol != "A" || buckets[0][1].Symbol != "B" {
		t.Fatalf("sorted access buckets = %#v", buckets)
	}
	if entry, found := access.LowerEntry(20); !found || entry.Key != 10 || len(entry.Values) != 2 {
		t.Fatalf("lower entry = %#v, found=%t", entry, found)
	}
	if entry, found := access.FloorEntry(20); !found || entry.Key != 20 || entry.Values[0].Symbol != "C" {
		t.Fatalf("floor entry = %#v, found=%t", entry, found)
	}
	if entry, found := access.HigherEntry(20); !found || entry.Key != 30 || entry.Values[0].Symbol != "D" {
		t.Fatalf("higher entry = %#v, found=%t", entry, found)
	}
	if entry, found := access.CeilingEntry(21); !found || entry.Key != 30 || entry.Values[0].Symbol != "D" {
		t.Fatalf("ceiling entry = %#v, found=%t", entry, found)
	}
	if !reflect.DeepEqual(access.HeadMap(20, true).Keys(), []float64{10, 20}) || !reflect.DeepEqual(access.TailMap(20, false).Keys(), []float64{30}) {
		t.Fatalf("head/tail maps = %#v / %#v", access.HeadMap(20, true).Keys(), access.TailMap(20, false).Keys())
	}
	if !reflect.DeepEqual(access.Descending().Keys(), []float64{30, 20, 10}) {
		t.Fatalf("descending keys = %#v", access.Descending().Keys())
	}
	descending := access.Descending()
	if entry, found := descending.LowerEntry(25); !found || entry.Key != 30 || entry.Values[0].Symbol != "D" {
		t.Fatalf("descending lower entry = %#v, found=%t", entry, found)
	}
	if entry, found := descending.FloorEntry(25); !found || entry.Key != 30 || entry.Values[0].Symbol != "D" {
		t.Fatalf("descending floor entry = %#v, found=%t", entry, found)
	}
	if entry, found := descending.HigherEntry(25); !found || entry.Key != 20 || entry.Values[0].Symbol != "C" {
		t.Fatalf("descending higher entry = %#v, found=%t", entry, found)
	}
	if entry, found := descending.CeilingEntry(25); !found || entry.Key != 20 || entry.Values[0].Symbol != "C" {
		t.Fatalf("descending ceiling entry = %#v, found=%t", entry, found)
	}
	if entry, found := descending.LowerEntry(15); !found || entry.Key != 20 || entry.Values[0].Symbol != "C" {
		t.Fatalf("descending lower multi-candidate entry = %#v, found=%t", entry, found)
	}
	if entry, found := descending.FloorEntry(15); !found || entry.Key != 20 || entry.Values[0].Symbol != "C" {
		t.Fatalf("descending floor multi-candidate entry = %#v, found=%t", entry, found)
	}
	if entry, found := descending.HigherEntry(15); !found || entry.Key != 10 || entry.Values[0].Symbol != "A" {
		t.Fatalf("descending higher multi-candidate entry = %#v, found=%t", entry, found)
	}
	if !reflect.DeepEqual(descending.HeadMap(20, true).Keys(), []float64{30, 20}) || !reflect.DeepEqual(descending.TailMap(20, false).Keys(), []float64{10}) {
		t.Fatalf("descending head/tail maps = %#v / %#v", descending.HeadMap(20, true).Keys(), descending.TailMap(20, false).Keys())
	}
	if !reflect.DeepEqual(descending.SubMap(30, true, 10, true).Keys(), []float64{30, 20, 10}) || !reflect.DeepEqual(descending.Descending().Keys(), []float64{10, 20, 30}) {
		t.Fatalf("descending submap/toggle = %#v / %#v", descending.SubMap(30, true, 10, true).Keys(), descending.Descending().Keys())
	}
	iterator := access.Iterator()
	var iterated []float64
	for {
		entry, found := iterator.Next()
		if !found {
			break
		}
		iterated = append(iterated, entry.Key)
		entry.Values[0].Symbol = "mutated"
	}
	if !reflect.DeepEqual(iterated, []float64{10, 20, 30}) || access.FirstEvents()[0].Symbol != "A" {
		t.Fatalf("iterator or copy isolation = %#v / %#v", iterated, access.FirstEvents())
	}
	var empty SortedAccessValue[float64, runtimeTestTrade]
	if !empty.IsEmpty() || len(empty.Keys()) != 0 {
		t.Fatalf("empty sorted access = %#v", empty)
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

type testAggregatePluginInputsState struct {
	accepted []float64
}

func (state *testAggregatePluginInputsState) Enter(value Value) {
	inputs, err := As[[]Value](value)
	if err != nil || len(inputs) < 5 {
		return
	}
	if _, err := As[runtimeTestTrade](inputs[3]); err != nil {
		return
	}
	if array, err := As[[]int](inputs[4]); err != nil || len(array) != 2 || array[0] != 1 || array[1] != 2 {
		return
	}
	minimum, minimumOK := numericValue(inputs[0])
	maximum, maximumOK := numericValue(inputs[1])
	current, currentOK := numericValue(inputs[2])
	if !minimumOK || !maximumOK || !currentOK || current < minimum || current > maximum {
		return
	}
	state.accepted = append(state.accepted, current)
}

func (state *testAggregatePluginInputsState) Leave(value Value) {
	inputs, err := As[[]Value](value)
	if err != nil || len(inputs) < 5 {
		return
	}
	if _, err := As[runtimeTestTrade](inputs[3]); err != nil {
		return
	}
	if array, err := As[[]int](inputs[4]); err != nil || len(array) != 2 || array[0] != 1 || array[1] != 2 {
		return
	}
	current, currentOK := numericValue(inputs[2])
	if !currentOK {
		return
	}
	for index, accepted := range state.accepted {
		if accepted == current {
			state.accepted = append(state.accepted[:index], state.accepted[index+1:]...)
			return
		}
	}
}

func (state *testAggregatePluginInputsState) Value() (int64, bool) {
	return int64(len(state.accepted)), true
}

func (state *testAggregatePluginInputsState) Clear() {
	state.accepted = state.accepted[:0]
}

type testAggregatePluginNoInputState struct {
	count int64
}

func (state *testAggregatePluginNoInputState) Enter(value Value) {
	inputs, err := As[[]Value](value)
	if err == nil && len(inputs) == 0 {
		state.count++
	}
}

func (state *testAggregatePluginNoInputState) Leave(value Value) {
	inputs, err := As[[]Value](value)
	if err == nil && len(inputs) == 0 {
		state.count--
	}
}

func (state *testAggregatePluginNoInputState) Value() (int64, bool) {
	return state.count, true
}

func (state *testAggregatePluginNoInputState) Clear() { state.count = 0 }

func TestAggregatePluginInputsMatchJavaMultiParameterAndNoParameter(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	inputVector := AggregatePluginInputs(Literal(1.0), Literal(10.0), price, EventValue[runtimeTestTrade](), Literal([]int{1, 2}))
	factory := func(_ AggregatePluginFactoryContext) AggregatePluginState[int64] {
		return &testAggregatePluginInputsState{}
	}
	if err := RegisterAggregatePluginFactory[int64](env, "registered-count-boundary", factory); err != nil {
		t.Fatal(err)
	}
	noInput := AggregatePluginInputs()
	noInputFactory := func(_ AggregatePluginFactoryContext) AggregatePluginState[int64] {
		return &testAggregatePluginNoInputState{}
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(4)).Aggregate(
		Alias("boundary", PluginAggregateWithFactory[int64]("count-boundary", inputVector, factory)),
		Alias("registered", PluginAggregateFactoryRef[int64](env, "registered-count-boundary", inputVector)),
		Alias("noParam", PluginAggregateWithFactory[int64]("count-no-param", noInput, noInputFactory)),
	).Query(StatementName("plugin-aggregate-inputs")))
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
		{Symbol: "A", Price: 5},
		{Symbol: "A", Price: 0},
		{Symbol: "A", Price: 11},
		{Symbol: "A", Price: 1},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 4 {
		t.Fatalf("plugin input rows = %d", len(rows))
	}
	for index, row := range rows {
		wantBoundary := int64(0)
		if index >= 0 {
			wantBoundary = 1
		}
		if index == 3 {
			wantBoundary = 2
		}
		for _, name := range []string{"boundary", "registered"} {
			if row.Get(name).Any() != wantBoundary {
				t.Fatalf("row %d %s = %#v, want %d", index, name, row.Get(name).Any(), wantBoundary)
			}
		}
		if row.Get("noParam").Any() != int64(index+1) {
			t.Fatalf("row %d noParam = %#v, want %d", index, row.Get("noParam").Any(), index+1)
		}
	}

	_, err = env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("bad", PluginAggregateWithFactory[int64]("bad-inputs", AggregatePluginInputs(nil), factory)),
	).Query(StatementName("invalid-plugin-inputs")))
	if err == nil {
		t.Fatal("nil aggregate plugin input unexpectedly built")
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

func TestSortedAccessTableMethodChainMatchesJavaAndPreservesNulls(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[sortedAccessTableTrigger](env, "SortedAccessTrigger"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "SortedTable", []TableColumn{
		TableColumnOf[SortedAccessValue[float64, runtimeTestTrade]]("sortcol"),
	}); err != nil {
		t.Fatal(err)
	}

	price := Field[runtimeTestTrade, float64]("price")
	sorted := SortedAccessBy[runtimeTestTrade, float64](EventValue[runtimeTestTrade](), price)
	aggregatePlan, err := env.Build(From[runtimeTestTrade](env, "Trade").
		Window(LengthWindow(4)).
		Aggregate(Alias("sortcol", sorted)).
		IntoTable("SortedTable", StatementName("sorted-table-access")))
	if err != nil {
		t.Fatal(err)
	}

	triggerPrice := Field[sortedAccessTableTrigger, float64]("price")
	sortedField := TableField[SortedAccessValue[float64, runtimeTestTrade]]("sortcol")
	descending := sortedTableMethod[SortedAccessValue[float64, runtimeTestTrade]](sortedField, "Descending")
	triggerPlan, err := env.Build(OnEvent(From[sortedAccessTableTrigger](env, "SortedAccessTrigger")).
		SelectFromTableWhere("SortedTable", Literal(true),
			Alias("first", sortedTableMethod[runtimeTestTrade](sortedField, "FirstEvent")),
			Alias("last", sortedTableMethod[runtimeTestTrade](sortedField, "LastEvent")),
			Alias("firstKey", sortedTableMethod[float64](sortedField, "FirstKey")),
			Alias("lastKey", sortedTableMethod[float64](sortedField, "LastKey")),
			Alias("get", sortedTableMethod[runtimeTestTrade](sortedField, "GetEvent", Literal(20.0))),
			Alias("getEvents", sortedTableMethod[[]runtimeTestTrade](sortedField, "GetEvents", Literal(20.0))),
			Alias("lower", sortedTableMethod[runtimeTestTrade](sortedField, "LowerEvent", triggerPrice)),
			Alias("higher", sortedTableMethod[runtimeTestTrade](sortedField, "HigherEvent", triggerPrice)),
			Alias("between", sortedTableMethod[[]runtimeTestTrade](sortedField, "EventsBetween", Literal(10.0), Literal(true), Literal(20.0), Literal(true))),
			Alias("submap", sortedTableMethod[SortedAccessValue[float64, runtimeTestTrade]](sortedField, "SubMap", Literal(10.0), Literal(true), Literal(20.0), Literal(true))),
			Alias("descendingKeys", Method[[]float64](descending, "Keys")),
			Alias("countEvents", sortedTableMethod[int64](sortedField, "CountEvents")),
			Alias("countKeys", sortedTableMethod[int64](sortedField, "CountKeys")),
			Alias("values", sortedTableMethod[[]runtimeTestTrade](sortedField, "Values")),
		).Query(StatementName("sorted-table-trigger")))
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
	rows := make([]Row, 0, 2)
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

	if err := engine.SendEvent(context.Background(), sortedAccessTableTrigger{Price: 25}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("empty sorted table rows = %d", len(rows))
	}
	for _, name := range []string{"first", "last", "firstKey", "lastKey", "get", "getEvents", "lower", "higher"} {
		if !rows[0].Get(name).IsNull() {
			t.Fatalf("empty sorted table %s = %#v", name, rows[0].Get(name))
		}
	}
	if rows[0].Get("countEvents").Any() != int64(0) || rows[0].Get("countKeys").Any() != int64(0) {
		t.Fatalf("empty sorted table counts = %#v", rows[0].AsMap())
	}

	for _, event := range []runtimeTestTrade{
		{Symbol: "A", Price: 10},
		{Symbol: "B", Price: 20},
		{Symbol: "C", Price: 20},
		{Symbol: "D", Price: 30},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SendEvent(context.Background(), sortedAccessTableTrigger{Price: 25}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("populated sorted table rows = %d", len(rows))
	}
	row := rows[1]
	if first, ok := row.Get("first").Any().(runtimeTestTrade); !ok || first.Symbol != "A" || first.Price != 10 {
		t.Fatalf("sorted table first = %#v", row.Get("first").Any())
	}
	if last, ok := row.Get("last").Any().(runtimeTestTrade); !ok || last.Symbol != "D" || last.Price != 30 {
		t.Fatalf("sorted table last = %#v", row.Get("last").Any())
	}
	if row.Get("firstKey").Any() != float64(10) || row.Get("lastKey").Any() != float64(30) {
		t.Fatalf("sorted table key range = %#v", row.AsMap())
	}
	if get, ok := row.Get("get").Any().(runtimeTestTrade); !ok || get.Symbol != "B" {
		t.Fatalf("sorted table get = %#v", row.Get("get").Any())
	}
	if lower, ok := row.Get("lower").Any().(runtimeTestTrade); !ok || lower.Symbol != "B" {
		t.Fatalf("sorted table lower = %#v", row.Get("lower").Any())
	}
	if higher, ok := row.Get("higher").Any().(runtimeTestTrade); !ok || higher.Symbol != "D" {
		t.Fatalf("sorted table higher = %#v", row.Get("higher").Any())
	}
	if events, ok := row.Get("getEvents").Any().([]runtimeTestTrade); !ok || len(events) != 2 || events[0].Symbol != "B" || events[1].Symbol != "C" {
		t.Fatalf("sorted table get events = %#v", row.Get("getEvents").Any())
	}
	if events, ok := row.Get("between").Any().([]runtimeTestTrade); !ok || len(events) != 3 || events[0].Symbol != "A" || events[2].Symbol != "C" {
		t.Fatalf("sorted table between = %#v", row.Get("between").Any())
	}
	if keys, ok := row.Get("descendingKeys").Any().([]float64); !ok || !reflect.DeepEqual(keys, []float64{30, 20, 10}) {
		t.Fatalf("sorted table descending keys = %#v", row.Get("descendingKeys").Any())
	}
	if row.Get("countEvents").Any() != int64(4) || row.Get("countKeys").Any() != int64(3) {
		t.Fatalf("sorted table counts = %#v", row.AsMap())
	}
	if values, ok := row.Get("values").Any().([]runtimeTestTrade); !ok || len(values) != 4 || values[1].Symbol != "B" || values[2].Symbol != "C" {
		t.Fatalf("sorted table values = %#v", row.Get("values").Any())
	}
	submap, ok := row.Get("submap").Any().(SortedAccessValue[float64, runtimeTestTrade])
	if !ok || !reflect.DeepEqual(submap.Keys(), []float64{10, 20}) || submap.CountEvents() != 3 {
		t.Fatalf("sorted table submap = %#v", row.Get("submap").Any())
	}
}

func TestSortedAccessGroupedTableSelectorMatchesJavaAndPreservesMissingGroupNulls(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[sortedGroupedTableTrigger](env, "SortedGroupedTableTrigger"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "SortedGroupedTable", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[SortedAccessValue[float64, runtimeTestTrade]]("sortcol"),
	}); err != nil {
		t.Fatal(err)
	}

	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	sorted := SortedAccessBy[runtimeTestTrade, float64](EventValue[runtimeTestTrade](), price)
	aggregatePlan, err := env.Build(From[runtimeTestTrade](env, "Trade").
		GroupBy(symbol).
		Select(
			Alias("symbol", symbol),
			Alias("sortcol", sorted),
		).
		IntoTable("SortedGroupedTable", StatementName("sorted-grouped-table")))
	if err != nil {
		t.Fatal(err)
	}

	triggerSymbol := Field[sortedGroupedTableTrigger, string]("symbol")
	sortedField := TableField[SortedAccessValue[float64, runtimeTestTrade]]("sortcol")
	triggerPlan, err := env.Build(OnEvent(From[sortedGroupedTableTrigger](env, "SortedGroupedTableTrigger")).
		SelectFromTable("SortedGroupedTable", []Expr{triggerSymbol},
			Alias("firstKey", Method[float64](sortedField, "FirstKey")),
			Alias("lastKey", Method[float64](sortedField, "LastKey")),
			Alias("sorted", Method[[]runtimeTestTrade](sortedField, "Sorted")),
		).
		Query(StatementName("sorted-grouped-table-trigger")))
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
	rows := make([]Row, 0, 8)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("grouped sorted table result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	assertNullGroup := func(index int) {
		if rows[index].Get("firstKey").State() != ValueNull || rows[index].Get("lastKey").State() != ValueNull || rows[index].Get("sorted").State() != ValueNull {
			t.Fatalf("missing grouped sorted table row = %#v", rows[index].AsMap())
		}
	}
	assertKeys := func(index int, first, last float64, expectedPrices ...float64) {
		if rows[index].Get("firstKey").Any() != first || rows[index].Get("lastKey").Any() != last {
			t.Fatalf("grouped sorted table keys = %#v", rows[index].AsMap())
		}
		sortedValue, ok := rows[index].Get("sorted").Any().([]runtimeTestTrade)
		if !ok || len(sortedValue) != len(expectedPrices) {
			t.Fatalf("grouped sorted table sorted value = %#v", rows[index].Get("sorted").Any())
		}
		for position, expected := range expectedPrices {
			if sortedValue[position].Price != expected {
				t.Fatalf("grouped sorted table sorted value = %#v", rows[index].Get("sorted").Any())
			}
		}
	}

	if err := engine.SendEvent(context.Background(), sortedGroupedTableTrigger{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("initial grouped selector rows = %d", len(rows))
	}
	assertNullGroup(0)

	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 10}, {Symbol: "A", Price: 20}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SendEvent(context.Background(), sortedGroupedTableTrigger{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("populated grouped selector rows = %d", len(rows))
	}
	assertKeys(1, 10, 20, 10, 20)

	if err := engine.SendEvent(context.Background(), sortedGroupedTableTrigger{Symbol: "B"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("missing second grouped selector rows = %d", len(rows))
	}
	assertNullGroup(2)

	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B", Price: 100}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), sortedGroupedTableTrigger{Symbol: "B"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("second populated grouped selector rows = %d", len(rows))
	}
	assertKeys(3, 100, 100, 100)
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

func TestFireAndForgetSortedAccessSnapshotAndNavigation(t *testing.T) {
	env, _ := newRuntimeTest(t)
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "aggregate-faf-sorted-window", schema); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, event := range []runtimeTestTrade{
		{Symbol: "A", Price: 10},
		{Symbol: "B", Price: 20},
		{Symbol: "C", Price: 20},
		{Symbol: "D", Price: 30},
	} {
		if err := engine.InsertNamedWindow(context.Background(), "aggregate-faf-sorted-window", event); err != nil {
			t.Fatal(err)
		}
	}
	price := Field[any, float64]("price")
	sorted := SortedAccessBy[float64, float64](price, price)
	plan, err := env.Build(FromNamedWindow(env, "aggregate-faf-sorted-window").Aggregate(
		Alias("sorted", sorted),
		Alias("lower", sorted.LowerEvent(Literal(25.0))),
		Alias("between", sorted.EventsBetween(Literal(10.0), true, Literal(20.0), true)),
		Alias("map", sorted.NavigableMapReference()),
	).Query(StatementName("aggregate-faf-sorted-access")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 1 {
		t.Fatalf("sorted aggregate FAF result count = %d", len(result.Results()))
	}
	row, ok := result.Results()[0].Row()
	if !ok {
		t.Fatalf("sorted aggregate FAF result is not a row: %#v", result.Results()[0])
	}
	if values, ok := row.Get("sorted").Any().(SortedAccessValue[float64, float64]); !ok || !reflect.DeepEqual(values.Keys(), []float64{10, 20, 30}) || values.CountEvents() != 4 {
		t.Fatalf("sorted aggregate FAF value = %#v", row.Get("sorted").Any())
	}
	if lower, ok := row.Get("lower").Any().(float64); !ok || lower != 20 {
		t.Fatalf("sorted aggregate FAF lower event = %#v", row.Get("lower").Any())
	}
	if between, ok := row.Get("between").Any().([]float64); !ok || !reflect.DeepEqual(between, []float64{10, 20, 20}) {
		t.Fatalf("sorted aggregate FAF events between = %#v", row.Get("between").Any())
	}
	mapValue, ok := row.Get("map").Any().(SortedAccessValue[float64, float64])
	if !ok || !reflect.DeepEqual(mapValue.Descending().Keys(), []float64{30, 20, 10}) || len(mapValue.Buckets()) != 3 {
		t.Fatalf("sorted aggregate FAF navigable map = %#v", row.Get("map").Any())
	}
}
