package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestDataflowFilterWithPortsRoutesMatchedAndRejectedValuesMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "dataflow-filter-ports").
		EventBusSource("source", "Trade").
		FilterWithPorts("filter", Greater[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0)), "matched", "rejected").
		Emitter("ok").
		Emitter("bad").
		ConnectPorts("source", "out", "filter", "in").
		ConnectPorts("filter", "matched", "ok", "in").
		ConnectPorts("filter", "rejected", "bad", "in").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	assertDataflowPortType(t, definition, "filter", true, "matched", reflect.TypeOf(Event{}))
	assertDataflowPortType(t, definition, "filter", true, "rejected", reflect.TypeOf(Event{}))

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
	outputs := instance.Outputs()
	if len(outputs) != 2 {
		t.Fatalf("filter outputs = %#v, want two terminal values", outputs)
	}
	first, ok := outputs[0].(Event)
	if !ok || first.Get("symbol").Any() != "low" {
		t.Fatalf("rejected output = %#v, want low event", outputs[0])
	}
	second, ok := outputs[1].(Event)
	if !ok || second.Get("symbol").Any() != "high" {
		t.Fatalf("matched output = %#v, want high event", outputs[1])
	}
}

type dataflowFilterSignalSource struct{}

func (dataflowFilterSignalSource) Run(ctx context.Context, emitter *DataflowEmitter) error {
	if err := emitter.SubmitSignal(ctx, WindowMarker{}); err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}

func TestDataflowFilterWithPortsFansOutSignalsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	definition, err := DefineDataflow(env, "dataflow-filter-port-signals").
		CustomSource("source", func(DataflowOperatorContext) (DataflowSourceRuntime, error) {
			return dataflowFilterSignalSource{}, nil
		}).
		FilterWithPorts("filter", Literal(true), "matched", "rejected").
		Emitter("ok").
		Emitter("bad").
		ConnectPorts("source", "out", "filter", "in").
		ConnectPorts("filter", "matched", "ok", "in").
		ConnectPorts("filter", "rejected", "bad", "in").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer instance.Cancel(context.Background())
	var signals []DataflowSignal
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		signals = instance.Signals()
		if len(signals) == 2 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(signals) != 2 || signals[0].SignalType() != "window" || signals[1].SignalType() != "window" {
		t.Fatalf("filter signal fan-out = %#v, want one signal at each downstream branch", signals)
	}
}

func TestDataflowFilterWithPortsRejectsLinearDefinition(t *testing.T) {
	env := NewEnvironment()
	if _, err := DefineDataflow(env, "dataflow-filter-ports-linear").
		BeaconSource("source", 1).
		FilterWithPorts("filter", Literal(true), "matched", "rejected").
		Emitter("sink").
		Build(); err == nil {
		t.Fatal("two-output Filter was accepted without graph edges")
	}
}
