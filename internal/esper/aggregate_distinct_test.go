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

// countDistinctBoxedEvent is a trade with a boxed nullable volume, the Go
// shape of Java's Long volume property.
type countDistinctBoxedEvent struct {
	Symbol string `esper:"symbol"`
	Volume *int64 `esper:"volume"`
}

// TestCountDistinctBoxedPointerValuesMatchEsperValueEquality covers the
// count(distinct ...) contract on nullable boxed properties: two events
// carrying different Long objects with the same value are one distinct value
// (Java equals semantics, not object identity), and window expiry recomputes
// the distinct set over the retained events. Mirrors the
// ResultSetAggregateCountOneView trajectory exercised by the
// resultset-aggregate-count-sum differential scenario.
func TestCountDistinctBoxedPointerValuesMatchEsperValueEquality(t *testing.T) {
	env, engine := newRuntimeTest(t)
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := RegisterStruct[countDistinctBoxedEvent](env, "BoxedTrade"); err != nil {
		t.Fatal(err)
	}
	volume := Field[countDistinctBoxedEvent, *int64]("volume")
	plan, err := env.Build(From[countDistinctBoxedEvent](env, "BoxedTrade").Window(LengthWindow(3)).Aggregate(
		Alias("countAll", CountAll()),
		Alias("countDistVol", CountDistinct[any](volume)),
		Alias("countVol", Count[*int64](volume)),
	).Query(StatementName("boxed-count-distinct"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 4)
	olds := make([]Row, 0, 4)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatal("count distinct result is not a row")
			}
			rows = append(rows, row)
		}
		for _, result := range batch.Old {
			row, ok := result.Row()
			if !ok {
				t.Fatal("count distinct old result is not a row")
			}
			olds = append(olds, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	boxed := func(value int64) *int64 { return &value }
	send := func(volume *int64) {
		if err := engine.SendEvent(context.Background(), countDistinctBoxedEvent{Symbol: "X", Volume: volume}); err != nil {
			t.Fatal(err)
		}
	}
	send(boxed(25))
	send(boxed(25))
	send(boxed(25))
	send(boxed(50))
	if len(rows) != 4 {
		t.Fatalf("count distinct rows = %#v", rows)
	}
	// Two distinct Long(25) objects are one distinct value.
	if rows[1].Get("countDistVol").Any() != int64(1) || rows[1].Get("countAll").Any() != int64(2) {
		t.Fatalf("second row = %#v", rows[1].AsMap())
	}
	// The fourth event evicts the first 25: distinct values drop to {25, 50}.
	if rows[3].Get("countDistVol").Any() != int64(2) || rows[3].Get("countAll").Any() != int64(3) {
		t.Fatalf("fourth row = %#v", rows[3].AsMap())
	}
	if len(olds) != 4 || olds[3].Get("countDistVol").Any() != int64(1) {
		t.Fatalf("count distinct old rows = %#v", olds)
	}
	// Null volumes do not contribute to count(distinct) or count(volume).
	// The window is now [25, 50, null]: two non-null volumes.
	send(nil)
	last := rows[len(rows)-1]
	if last.Get("countDistVol").Any() != int64(2) || last.Get("countVol").Any() != int64(2) || last.Get("countAll").Any() != int64(3) {
		t.Fatalf("null volume row = %#v", last.AsMap())
	}
}
