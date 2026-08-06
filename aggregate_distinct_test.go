package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type testAggregatePluginStarState struct {
	events []Event
}

func (state *testAggregatePluginStarState) Enter(value Value) {
	event, err := As[Event](value)
	if err == nil {
		state.events = append(state.events, event)
	}
}

func (state *testAggregatePluginStarState) Leave(value Value) {
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

func (state *testAggregatePluginStarState) Value() (string, bool) {
	values := make([]string, 0, len(state.events))
	for _, event := range state.events {
		if symbol, ok := event.Get("symbol").Any().(string); ok {
			values = append(values, symbol)
		}
	}
	if len(values) == 0 {
		return "", false
	}
	return strings.Join(values, " "), true
}

func (state *testAggregatePluginStarState) Clear() { state.events = nil }

func TestDistinctAggregatePluginMatchesJavaDistinctAndStarSemantics(t *testing.T) {
	env, engine := newRuntimeTest(t)
	label := Method[string](EventValue[runtimeTestTrade](), "PriceLabel")
	factory := func(AggregatePluginFactoryContext) AggregatePluginState[string] {
		return &testAggregatePluginConcatState{}
	}
	distinct := DistinctAggregate[string](
		PluginAggregateWithFactory[string]("distinct-concat", label, factory),
		label,
	)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(4)).Aggregate(
		Alias("value", distinct),
	).Query(StatementName("distinct-plugin")))
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
		{Symbol: "B", Price: 2},
		{Symbol: "B", Price: 2},
		{Symbol: "A", Price: 1},
		{Symbol: "C", Price: 3},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"A:1", "A:1 B:2", "A:1 B:2", "A:1 B:2", "B:2 A:1 C:3"}
	if len(rows) != len(want) {
		t.Fatalf("distinct plugin rows = %d", len(rows))
	}
	for index, expected := range want {
		if got := rows[index].Get("value").Any(); got != expected {
			t.Fatalf("distinct plugin row %d = %#v, want %q", index, got, expected)
		}
	}

	star := DistinctAggregate[string](
		PluginAggregateWithFactory[string]("star-concat", nil, func(AggregatePluginFactoryContext) AggregatePluginState[string] {
			return &testAggregatePluginStarState{}
		}),
		nil,
	)
	starPlan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(3)).Aggregate(
		Alias("value", star),
	).Query(StatementName("star-plugin")))
	if err != nil {
		t.Fatal(err)
	}
	starDeployment, err := engine.Deploy(context.Background(), starPlan)
	if err != nil {
		t.Fatal(err)
	}
	var last Row
	if _, err := starDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			var ok bool
			last, ok = batch.New[len(batch.New)-1].Row()
			if !ok {
				t.Fatal("star plugin result is not a row")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	event := runtimeTestTrade{Symbol: "S", Price: 1}
	if err := engine.SendEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if last.Get("value").Any() != "S S" {
		t.Fatalf("star distinct identity result = %#v", last.AsMap())
	}

	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("bad", DistinctAggregate[int64](nil, label)),
	).Query(StatementName("invalid-distinct-plugin"))); err == nil {
		t.Fatal("nil aggregate distinct wrapper unexpectedly built")
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("bad", DistinctAggregate[int64](CountAll(), CountAll())),
	).Query(StatementName("invalid-distinct-input"))); err == nil {
		t.Fatal("aggregate distinct input unexpectedly accepted an aggregate")
	}
	if !reflect.DeepEqual([]string{"A:1", "A:1 B:2"}, []string{rows[0].Get("value").Any().(string), rows[1].Get("value").Any().(string)}) {
		t.Fatal("distinct plugin output changed unexpectedly")
	}
}
