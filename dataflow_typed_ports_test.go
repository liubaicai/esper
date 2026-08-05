package esper

import (
	"context"
	"fmt"
	"testing"
)

type dataflowTypedText struct {
	Value string
}

type dataflowTypedNumber struct {
	Value int
}

type dataflowFanInOutRuntime struct {
	seen *[]string
}

type dataflowWrongOutputRuntime struct{}

func (dataflowWrongOutputRuntime) Process(context.Context, DataflowInput) ([]DataflowEmission, error) {
	return []DataflowEmission{EmitPort("out", 17)}, nil
}

func (r *dataflowFanInOutRuntime) Process(_ context.Context, input DataflowInput) ([]DataflowEmission, error) {
	switch value := input.Value.(type) {
	case dataflowTypedText:
		*r.seen = append(*r.seen, input.Port+":"+value.Value)
		return []DataflowEmission{EmitPort("text", "text:"+value.Value)}, nil
	case dataflowTypedNumber:
		*r.seen = append(*r.seen, input.Port+":"+fmt.Sprint(value.Value))
		return []DataflowEmission{EmitPort("number", fmt.Sprintf("number:%d", value.Value))}, nil
	default:
		return nil, fmt.Errorf("unexpected typed fan-in value %T", input.Value)
	}
}

// TestDataflowTypedPortsFanInAndFanOutMatchesEsper mirrors
// EPLDataflowInputOutputVariations.EPLDataflowFanInOut: two typed streams are
// fanned into separate input ports and the custom operator emits each type to
// its own output port. The Go port contract is checked both at Build time and
// when graph values cross an edge.
func TestDataflowTypedPortsFanInAndFanOutMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	var seen []string
	definition, err := DefineDataflow(env, "typed-fan-in-out").
		BeaconSource("text-a", dataflowTypedText{Value: "A1"}).
		BeaconSource("text-b", dataflowTypedText{Value: "A2"}).
		BeaconSource("number-a", dataflowTypedNumber{Value: 10}).
		BeaconSource("number-b", dataflowTypedNumber{Value: 20}).
		CustomTypedPorts("fan", func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			return &dataflowFanInOutRuntime{seen: &seen}, nil
		}, []DataflowPort{
			DataflowPortOf[dataflowTypedText]("text-a"),
			DataflowPortOf[dataflowTypedText]("text-b"),
			DataflowPortOf[dataflowTypedNumber]("number-a"),
			DataflowPortOf[dataflowTypedNumber]("number-b"),
		}, []DataflowPort{
			DataflowPortOf[string]("text"),
			DataflowPortOf[string]("number"),
		}).
		Emitter("text-sink").
		Emitter("number-sink").
		ConnectPorts("text-a", "out", "fan", "text-a").
		ConnectPorts("text-b", "out", "fan", "text-b").
		ConnectPorts("number-a", "out", "fan", "number-a").
		ConnectPorts("number-b", "out", "fan", "number-b").
		ConnectPorts("fan", "text", "text-sink", "in").
		ConnectPorts("fan", "number", "number-sink", "in").
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
	if instance.State() != DataflowComplete {
		t.Fatalf("typed fan-in/out state = %v", instance.State())
	}
	if stats := instance.Stats(); stats.Processed != 4 || stats.Emitted != 4 {
		t.Fatalf("typed fan-in/out stats = %#v", stats)
	}

	outputs := instance.Outputs()
	if len(outputs) != 4 {
		t.Fatalf("typed fan-in/out outputs = %#v", outputs)
	}
	wantOutputs := map[string]int{
		"text:A1": 1, "text:A2": 1,
		"number:10": 1, "number:20": 1,
	}
	for _, output := range outputs {
		value, ok := output.(string)
		if !ok {
			t.Fatalf("typed fan-in/out output type = %T, want string", output)
		}
		wantOutputs[value]--
	}
	for value, count := range wantOutputs {
		if count != 0 {
			t.Fatalf("typed fan-in/out missing %q, remaining=%d outputs=%#v", value, count, outputs)
		}
	}
	wantSeen := map[string]int{"text-a:A1": 1, "text-b:A2": 1, "number-a:10": 1, "number-b:20": 1}
	for _, value := range seen {
		wantSeen[value]--
	}
	for value, count := range wantSeen {
		if count != 0 {
			t.Fatalf("typed fan-in/out missing input %q, remaining=%d seen=%#v", value, count, seen)
		}
	}

	_, err = DefineDataflow(env, "typed-port-mismatch").
		CustomTypedPorts("source", func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			return &dataflowFanInOutRuntime{}, nil
		}, nil, []DataflowPort{DataflowPortOf[int]("out")}).
		CustomTypedPorts("sink", func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			return &dataflowFanInOutRuntime{}, nil
		}, []DataflowPort{DataflowPortOf[string]("in")}, nil).
		Connect("source", "sink").
		Build()
	if err == nil {
		t.Fatal("typed dataflow edge mismatch was accepted")
	}

	runtimeDefinition, err := DefineDataflow(env, "typed-runtime-mismatch").
		BeaconSource("source", dataflowTypedText{Value: "wrong"}).
		CustomTypedPorts("bad", func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			return dataflowWrongOutputRuntime{}, nil
		}, []DataflowPort{DataflowPortOf[dataflowTypedText]("in")}, []DataflowPort{DataflowPortOf[string]("out")}).
		CustomTypedPorts("typed-sink", func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			return &dataflowFanInOutRuntime{}, nil
		}, []DataflowPort{DataflowPortOf[string]("in")}, nil).
		Connect("source", "bad").
		Connect("bad", "typed-sink").
		Build()
	if err != nil {
		t.Fatal("typed runtime mismatch graph failed to build", err)
	}
	runtimeMismatch, err := NewEngine(env).InstantiateDataflow(context.Background(), runtimeDefinition)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtimeMismatch.Start(context.Background()); err == nil {
		t.Fatal("typed runtime mismatch was not rejected")
	}
}
