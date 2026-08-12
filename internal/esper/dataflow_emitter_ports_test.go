package esper

import (
	"context"
	"errors"
	"testing"
)

type dataflowEmitterPortText struct {
	Value string `esper:"value"`
}

type dataflowEmitterPortNumber struct {
	Value int `esper:"value"`
}

type dataflowMultiPortSignalSource struct{}

func (dataflowMultiPortSignalSource) Run(ctx context.Context, emitter *DataflowEmitter) error {
	return emitter.SubmitSignal(ctx, WindowMarker{})
}

func registerDataflowEmitterPortSchemas(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterStruct[dataflowEmitterPortText](env, "DataflowEmitterPortText"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dataflowEmitterPortNumber](env, "DataflowEmitterPortNumber"); err != nil {
		t.Fatal(err)
	}
}

func TestDataflowCaptiveEmitterNamedPortsMatchEsperSubmitPort(t *testing.T) {
	env := NewEnvironment()
	registerDataflowEmitterPortSchemas(t, env)
	var signals []string
	definition, err := DefineDataflow(env, "captive-emitter-ports").
		EmitterPorts("source", "text", "number", "unused").
		Emitter("text-sink").
		Emitter("number-sink").
		OnSignal("text-sink", func(context.Context, DataflowSignal) error {
			signals = append(signals, "text")
			return nil
		}).
		OnSignal("number-sink", func(context.Context, DataflowSignal) error {
			signals = append(signals, "number")
			return nil
		}).
		ConnectPorts("source", "text", "text-sink", "in").
		ConnectPorts("source", "number", "number-sink", "in").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	operators := definition.Operators()
	if len(operators) != 3 || len(operators[0].OutputPorts) != 3 || operators[0].OutputPorts[2] != "unused" {
		t.Fatalf("emitter output metadata = %#v", operators)
	}

	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	captive, err := instance.StartCaptive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	emitter, ok := captive.Emitter("source")
	if !ok {
		t.Fatal("multi-port captive emitter is missing")
	}
	if emitter.Name() != "source" {
		t.Fatalf("captive emitter name = %q", emitter.Name())
	}
	var nilEmitter *DataflowEmitter
	if nilEmitter.Name() != "" {
		t.Fatalf("nil captive emitter name = %q", nilEmitter.Name())
	}
	if err := emitter.Submit(context.Background(), dataflowEmitterPortText{Value: "ambiguous"}); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("ambiguous Submit error = %v", err)
	}
	if err := emitter.SubmitPort(context.Background(), "missing", dataflowEmitterPortText{}); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("unknown SubmitPort error = %v", err)
	}
	if err := emitter.SubmitPort(context.Background(), "text", dataflowEmitterPortText{Value: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := emitter.SubmitPort(context.Background(), "number", dataflowEmitterPortNumber{Value: 10}); err != nil {
		t.Fatal(err)
	}
	if err := emitter.SubmitPort(context.Background(), "unused", dataflowEmitterPortText{Value: "ignored"}); err != nil {
		t.Fatalf("unconnected declared emitter port: %v", err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 2 {
		t.Fatalf("named-port outputs = %#v", outputs)
	}
	textEvent, ok := outputs[0].(Event)
	if !ok || textEvent.TypeName() != "DataflowEmitterPortText" || textEvent.Get("value").Any() != "A" {
		t.Fatalf("text-port output = %#v", outputs[0])
	}
	numberEvent, ok := outputs[1].(Event)
	if !ok || numberEvent.TypeName() != "DataflowEmitterPortNumber" || numberEvent.Get("value").Any() != 10 {
		t.Fatalf("number-port output = %#v", outputs[1])
	}
	if err := emitter.SubmitSignal(context.Background(), FinalMarker{}); err != nil {
		t.Fatal(err)
	}
	if len(signals) != 2 || signals[0] != "text" || signals[1] != "number" {
		t.Fatalf("multi-port signal fan-out = %#v", signals)
	}
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDataflowSingleNamedEmitterPortSupportsSubmit(t *testing.T) {
	env := NewEnvironment()
	registerDataflowEmitterPortSchemas(t, env)
	definition, err := DefineDataflow(env, "single-named-emitter-port").
		EmitterPorts("source", "events").
		Emitter("sink").
		ConnectPorts("source", "events", "sink", "in").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	captive, err := instance.StartCaptive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	emitter, _ := captive.Emitter("source")
	if err := emitter.Submit(context.Background(), dataflowEmitterPortText{Value: "single"}); err != nil {
		t.Fatal(err)
	}
	if outputs := instance.Outputs(); len(outputs) != 1 || outputs[0].(Event).Get("value").Any() != "single" {
		t.Fatalf("single named-port outputs = %#v", outputs)
	}
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDataflowCustomSourceSignalFansOutAllNamedPorts(t *testing.T) {
	env := NewEnvironment()
	var signals []string
	definition, err := DefineDataflow(env, "custom-source-signal-ports").
		CustomTypedSource("source", func(DataflowOperatorContext) (DataflowSourceRuntime, error) {
			return dataflowMultiPortSignalSource{}, nil
		}, []DataflowPort{{Name: "left"}, {Name: "right"}}).
		Emitter("left-sink").
		Emitter("right-sink").
		OnSignal("left-sink", func(context.Context, DataflowSignal) error {
			signals = append(signals, "left")
			return nil
		}).
		OnSignal("right-sink", func(context.Context, DataflowSignal) error {
			signals = append(signals, "right")
			return nil
		}).
		ConnectPorts("source", "left", "left-sink", "in").
		ConnectPorts("source", "right", "right-sink", "in").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	captive, err := instance.StartCaptive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if runnables := captive.Runnables(); len(runnables) != 1 {
		t.Fatalf("custom-source runnables = %#v", runnables)
	} else if err := runnables[0].Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(signals) != 2 || signals[0] != "left" || signals[1] != "right" {
		t.Fatalf("custom-source signal fan-out = %#v", signals)
	}
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDataflowEmitterPortDefinitionsEnterPlanIdentity(t *testing.T) {
	build := func(outputs ...string) Plan {
		t.Helper()
		env := NewEnvironment()
		registerDataflowEmitterPortSchemas(t, env)
		if _, err := DefineDataflow(env, "emitter-port-plan").
			EmitterPorts("source", outputs...).
			Emitter("sink").
			ConnectPorts("source", outputs[0], "sink", "in").
			Build(); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(From[dataflowEmitterPortText](env, "DataflowEmitterPortText").Query())
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	one := build("events")
	two := build("events", "unused")
	if one.Hash() == two.Hash() || string(one.Canonical()) == string(two.Canonical()) {
		t.Fatal("different emitter output-port definitions share plan identity")
	}
}
