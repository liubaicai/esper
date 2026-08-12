package esper

import (
	"context"
	"fmt"
	"testing"
)

type dataflowFactorialStep struct {
	Current int
	Product uint64
}

type dataflowFactorialRuntime struct{}

func (dataflowFactorialRuntime) Process(_ context.Context, input DataflowInput) ([]DataflowEmission, error) {
	switch input.Port {
	case "input":
		n := input.Value.(int)
		return []DataflowEmission{EmitPort("temp", dataflowFactorialStep{Current: n, Product: uint64(n)})}, nil
	case "temp":
		step := input.Value.(dataflowFactorialStep)
		if step.Current <= 1 {
			return []DataflowEmission{EmitPort("final", step.Product)}, nil
		}
		next := step.Current - 1
		return []DataflowEmission{EmitPort("temp", dataflowFactorialStep{Current: next, Product: step.Product * uint64(next)})}, nil
	default:
		return nil, nil
	}
}

func TestDataflowFeedbackEdgeFactorialMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	definition, err := DefineDataflow(env, "feedback-factorial").
		BeaconSource("source", 5).
		CustomTypedPorts("factorial", func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			return dataflowFactorialRuntime{}, nil
		}, []DataflowPort{
			DataflowPortOf[int]("input"),
			DataflowPortOf[dataflowFactorialStep]("temp"),
		}, []DataflowPort{
			DataflowPortOf[dataflowFactorialStep]("temp"),
			DataflowPortOf[uint64]("final"),
		}).
		Emitter("sink").
		ConnectPorts("source", "out", "factorial", "input").
		ConnectFeedbackPorts("factorial", "temp", "factorial", "temp").
		ConnectPorts("factorial", "final", "sink", "in").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	edges := definition.Edges()
	if len(edges) != 3 || !edges[1].Feedback {
		t.Fatalf("feedback graph edges = %#v, want marked self-loop", edges)
	}

	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if instance.State() != DataflowComplete {
		t.Fatalf("feedback factorial state = %v, want complete", instance.State())
	}
	outputs := instance.Outputs()
	if len(outputs) != 1 || outputs[0] != uint64(120) {
		t.Fatalf("feedback factorial outputs = %#v, want [120]", outputs)
	}
}

type dataflowIdentityRuntime struct{}

func (dataflowIdentityRuntime) Process(_ context.Context, input DataflowInput) ([]DataflowEmission, error) {
	return []DataflowEmission{Emit(input.Value)}, nil
}

func TestDataflowDeepLinearGraphMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	builder := DefineDataflow(env, "deep-linear-dataflow").BeaconSource("source", "A1")
	previous := "source"
	for index := 1; index <= 17; index++ {
		name := fmt.Sprintf("select-%02d", index)
		builder = builder.Custom(name, func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			return dataflowIdentityRuntime{}, nil
		}).Connect(previous, name)
		previous = name
	}
	definition, err := builder.Emitter("sink").Connect(previous, "sink").Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(definition.Operators()) != 19 || len(definition.Edges()) != 18 {
		t.Fatalf("deep graph shape = operators %d edges %d", len(definition.Operators()), len(definition.Edges()))
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if outputs := instance.Outputs(); len(outputs) != 1 || outputs[0] != "A1" {
		t.Fatalf("deep graph outputs = %#v, want [A1]", outputs)
	}
}

func TestDataflowUnmarkedCycleStillRejected(t *testing.T) {
	env := NewEnvironment()
	if _, err := DefineDataflow(env, "unmarked-feedback-cycle").
		BeaconSource("source").
		CustomPorts("middle", func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			return dataflowFactorialRuntime{}, nil
		}, []string{"in", "temp"}, []string{"out", "temp"}).
		Connect("source", "middle").
		ConnectPorts("middle", "temp", "middle", "temp").
		Build(); err == nil {
		t.Fatal("unmarked feedback self-loop was accepted")
	}
}
