package esper

import (
	"context"
	"testing"
)

func TestDataflowEventBusSourceCollectorTransformsAndDuplicatesEvents(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "event-bus-source-collector").
		EventBusSourceWithCollector("source", "Trade", func(_ context.Context, event Event) ([]any, error) {
			trade := event.Underlying().(runtimeTestTrade)
			first, err := newEvent(event.Schema(), runtimeTestTrade{Symbol: trade.Symbol + "-1", Price: trade.Price}, event.ReceivedAt())
			if err != nil {
				return nil, err
			}
			return []any{
				first,
				runtimeTestTrade{Symbol: trade.Symbol + "-2", Price: trade.Price},
			}, nil
		}).
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Trade", runtimeTestTrade{Symbol: "E", Price: 7}); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 2 {
		t.Fatalf("source collector outputs = %#v, want two events", outputs)
	}
	for index, want := range []string{"E-1", "E-2"} {
		event, ok := outputs[index].(Event)
		if !ok || event.Underlying().(runtimeTestTrade).Symbol != want {
			t.Fatalf("source collector output[%d] = %#v, want %s", index, outputs[index], want)
		}
	}
	if stats := instance.Stats(); stats.Processed != 1 || stats.Emitted != 2 {
		t.Fatalf("source collector stats = %#v", stats)
	}
}

func TestDataflowEventBusSinkCollectorSendsDynamicEventTypes(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[runtimeTestTrade](env, "TradeCopy"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "event-bus-sink-collector").
		EventBusSource("source", "Trade").
		EventBusSinkWithCollector("sink", func(_ context.Context, value any) ([]DataflowEventBusSinkEmission, error) {
			event := value.(Event)
			trade := event.Underlying().(runtimeTestTrade)
			return []DataflowEventBusSinkEmission{{
				EventType: "TradeCopy",
				Value:     runtimeTestTrade{Symbol: "copy-" + trade.Symbol, Price: trade.Price + 1},
			}}, nil
		}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "TradeCopy").Query(StatementName("collector-copy-observer")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var received []runtimeTestTrade
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if ok {
				received = append(received, event.Underlying().(runtimeTestTrade))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Trade", runtimeTestTrade{Symbol: "E", Price: 7}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || received[0].Symbol != "copy-E" || received[0].Price != 8 {
		t.Fatalf("dynamic sink collector received = %#v", received)
	}
}

func TestDataflowEventBusSourceFilterRunsBeforeCollector(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	collectorCalls := 0
	definition, err := DefineDataflow(env, "event-bus-source-filter-collector").
		EventBusSourceWithFilterAndCollector(
			"source",
			"Trade",
			Greater[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0)),
			func(_ context.Context, event Event) ([]any, error) {
				collectorCalls++
				trade := event.Underlying().(runtimeTestTrade)
				return []any{runtimeTestTrade{Symbol: "accepted-" + trade.Symbol, Price: trade.Price}}, nil
			},
		).
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer instance.Cancel(context.Background())
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "low", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "high", Price: 11}); err != nil {
		t.Fatal(err)
	}
	if collectorCalls != 1 {
		t.Fatalf("source collector calls = %d, want one accepted event", collectorCalls)
	}
	outputs := instance.Outputs()
	if len(outputs) != 1 {
		t.Fatalf("filtered source collector outputs = %#v, want one event", outputs)
	}
	output, ok := outputs[0].(Event)
	if !ok || output.Underlying().(runtimeTestTrade).Symbol != "accepted-high" {
		t.Fatalf("filtered source collector output = %#v", outputs[0])
	}
}
