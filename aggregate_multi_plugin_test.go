package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type testAggregateMultiState struct {
	entries []([]Value)
}

func (s *testAggregateMultiState) Enter(values []Value) {
	s.entries = append(s.entries, append([]Value(nil), values...))
}

func (s *testAggregateMultiState) Leave(values []Value) {
	for index, current := range s.entries {
		if aggregateMultiValuesEqual(current, values) {
			s.entries = append(s.entries[:index], s.entries[index+1:]...)
			return
		}
	}
}

func (s *testAggregateMultiState) Value(method string) (any, bool) {
	if len(s.entries) == 0 {
		return nil, false
	}
	switch method {
	case "ss":
		return s.entries[len(s.entries)-1][0].Any(), true
	case "sa", "sc":
		values := make([]string, 0, len(s.entries))
		for _, entry := range s.entries {
			if len(entry) == 0 {
				continue
			}
			value, ok := entry[0].Any().(string)
			if ok {
				values = append(values, value)
			}
		}
		return values, true
	case "se1", "se2":
		event, err := As[Event](Present(s.entries[len(s.entries)-1][0].Any()))
		return event, err == nil
	case "ee":
		events := make([]Event, 0, len(s.entries))
		for _, entry := range s.entries {
			if len(entry) == 0 {
				continue
			}
			event, ok := As[Event](entry[0])
			if ok == nil {
				events = append(events, event)
			}
		}
		return events, true
	default:
		return nil, false
	}
}

func (s *testAggregateMultiState) Clear() { s.entries = nil }

func aggregateMultiValuesEqual(left, right []Value) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].State() != right[index].State() {
			return false
		}
		if left[index].IsPresent() && !reflect.DeepEqual(left[index].Any(), right[index].Any()) {
			return false
		}
	}
	return true
}

func TestAggregateMultiPluginMatchesJavaSharedAccessorAndTypeFamilies(t *testing.T) {
	env, engine := newRuntimeTest(t)
	methods := []AggregateMultiPluginMethod{
		AggregateMultiMethod[string]("ss"),
		AggregateMultiMethod[[]string]("sa"),
		AggregateMultiMethod[[]string]("sc"),
		AggregateMultiMethod[Event]("se1", "single-event"),
		AggregateMultiMethod[Event]("se2", "single-event"),
		AggregateMultiMethod[[]Event]("ee"),
	}
	instances := make(map[string]int)
	if err := RegisterAggregateMultiPlugin(env, "registered-multi", methods, func(ctx AggregateMultiPluginFactoryContext) AggregateMultiPluginState {
		if ctx.Provider != "registered-multi" || len(ctx.Methods) != len(methods) {
			t.Fatalf("multi factory context = %#v", ctx)
		}
		instances[ctx.Provider]++
		return &testAggregateMultiState{}
	}); err != nil {
		t.Fatal(err)
	}
	symbol := Field[runtimeTestTrade, string]("symbol")
	query := From[runtimeTestTrade](env, "Trade").GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("scalar", PluginAggregateMultiRef[string](env, "registered-multi", "ss", symbol)),
		Alias("array", PluginAggregateMultiRef[[]string](env, "registered-multi", "sa", symbol)),
		Alias("collection", PluginAggregateMultiRef[[]string](env, "registered-multi", "sc", symbol)),
		Alias("singleOne", PluginAggregateMultiRef[Event](env, "registered-multi", "se1", nil)),
		Alias("singleTwo", PluginAggregateMultiRef[Event](env, "registered-multi", "se2", nil)),
		Alias("events", PluginAggregateMultiRef[[]Event](env, "registered-multi", "ee", nil)),
	).Query(StatementName("aggregate-multi-shared"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plan.Canonical()), "aggregate-multi-plugin(registered-multi:") {
		t.Fatalf("multi plugin missing from canonical plan: %s", plan.Canonical())
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 3)
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
		{Symbol: "A", Price: 2},
		{Symbol: "B", Price: 3},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 3 {
		t.Fatalf("multi plugin rows = %d", len(rows))
	}
	var sawA, sawB bool
	for _, row := range rows {
		symbolValue, _ := row.Get("symbol").Any().(string)
		array, arrayOK := row.Get("array").Any().([]string)
		collection, collectionOK := row.Get("collection").Any().([]string)
		events, eventsOK := row.Get("events").Any().([]Event)
		if !arrayOK || !collectionOK || !eventsOK {
			t.Fatalf("multi plugin output types = %#v", row.AsMap())
		}
		singleOne, oneErr := As[Event](row.Get("singleOne"))
		singleTwo, twoErr := As[Event](row.Get("singleTwo"))
		if oneErr != nil || twoErr != nil || eventIdentity(singleOne) != eventIdentity(singleTwo) {
			t.Fatalf("shared se1/se2 value differs: %#v", row.AsMap())
		}
		switch symbolValue {
		case "A":
			if reflect.DeepEqual(array, []string{"A", "A"}) && reflect.DeepEqual(collection, []string{"A", "A"}) {
				if len(events) != 2 {
					t.Fatalf("A event collection = %d", len(events))
				}
				sawA = true
			}
		case "B":
			if !reflect.DeepEqual(array, []string{"B"}) || !reflect.DeepEqual(collection, []string{"B"}) || len(events) != 1 {
				t.Fatalf("B multi output = %#v", row.AsMap())
			}
			sawB = true
		}
	}
	if !sawA || !sawB {
		t.Fatalf("multi plugin did not preserve grouped state: %#v", rows)
	}
	if instances["registered-multi"] != 10 {
		t.Fatalf("multi factory instances = %d, want five shared state keys for two groups", instances["registered-multi"])
	}
	if err := RegisterAggregateMultiPlugin(env, "registered-multi", methods, func(AggregateMultiPluginFactoryContext) AggregateMultiPluginState {
		return &testAggregateMultiState{}
	}); err == nil {
		t.Fatal("duplicate multi plugin registration succeeded")
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("bad", PluginAggregateMultiRef[int64](env, "registered-multi", "missing", symbol)),
	).Query(StatementName("aggregate-multi-invalid-method"))); err == nil {
		t.Fatal("unknown multi plugin method unexpectedly built")
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("bad", PluginAggregateMultiRef[int64](env, "registered-multi", "ss", symbol)),
	).Query(StatementName("aggregate-multi-invalid-type"))); err == nil {
		t.Fatal("wrong multi plugin result type unexpectedly built")
	}
}

func TestAggregateMultiPluginReplaysLengthWindowLeaveLifecycle(t *testing.T) {
	env, engine := newRuntimeTest(t)
	methods := []AggregateMultiPluginMethod{
		AggregateMultiMethod[float64]("ss"),
		AggregateMultiMethod[[]string]("sa"),
		AggregateMultiMethod[[]Event]("ee"),
	}
	if err := RegisterAggregateMultiPlugin(env, "window-multi", methods, func(AggregateMultiPluginFactoryContext) AggregateMultiPluginState {
		return &testAggregateMultiState{}
	}); err != nil {
		t.Fatal(err)
	}
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(2)).Aggregate(
		Alias("latest", PluginAggregateMultiRef[float64](env, "window-multi", "ss", price)),
		Alias("array", PluginAggregateMultiRef[[]string](env, "window-multi", "sa", symbol)),
		Alias("events", PluginAggregateMultiRef[[]Event](env, "window-multi", "ee", nil)),
	).Query(StatementName("aggregate-multi-window")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	rows := make([]Row, 0, 3)
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
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "B", Price: 2}, {Symbol: "C", Price: 3}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 3 {
		t.Fatalf("window multi rows = %d", len(rows))
	}
	last := rows[len(rows)-1]
	if last.Get("latest").Any() != float64(3) {
		t.Fatalf("window multi latest = %#v", last.AsMap())
	}
	if values, ok := last.Get("array").Any().([]string); !ok || !reflect.DeepEqual(values, []string{"B", "C"}) {
		t.Fatalf("window multi array = %#v", last.Get("array").Any())
	}
	if values, ok := last.Get("events").Any().([]Event); !ok || len(values) != 2 || values[0].Get("symbol").Any() != "B" || values[1].Get("symbol").Any() != "C" {
		t.Fatalf("window multi events = %#v", last.Get("events").Any())
	}
}
