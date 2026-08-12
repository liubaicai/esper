package esper

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type dataflowInvalidGraphRuntime struct{}

func (dataflowInvalidGraphRuntime) Process(_ context.Context, input DataflowInput) ([]DataflowEmission, error) {
	return []DataflowEmission{Emit(input.Value)}, nil
}

type dataflowInvalidGraphSourceRuntime struct{}

func (dataflowInvalidGraphSourceRuntime) Run(context.Context, *DataflowEmitter) error {
	return nil
}

type dataflowInvalidGraphEvent struct {
	Value string `esper:"value"`
}

func dataflowInvalidGraphFactory(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
	return dataflowInvalidGraphRuntime{}, nil
}

func dataflowInvalidGraphSourceFactory(DataflowOperatorContext) (DataflowSourceRuntime, error) {
	return dataflowInvalidGraphSourceRuntime{}, nil
}

func TestDataflowInvalidGraphTopologyMatchesEsper(t *testing.T) {
	tests := []struct {
		name  string
		build func(*Environment) (DataflowDefinition, error)
	}{
		{
			name: "dataflow name surrounding whitespace",
			build: func(env *Environment) (DataflowDefinition, error) {
				return DefineDataflow(env, " invalid").Emitter("sink").Build()
			},
		},
		{
			name: "operator name surrounding whitespace",
			build: func(env *Environment) (DataflowDefinition, error) {
				return DefineDataflow(env, "invalid-operator-name").Emitter("sink ").Build()
			},
		},
		{
			name: "input port surrounding whitespace",
			build: func(env *Environment) (DataflowDefinition, error) {
				return DefineDataflow(env, "invalid-input-port").
					CustomPorts("operator", dataflowInvalidGraphFactory, []string{" in"}, []string{"out"}).
					Build()
			},
		},
		{
			name: "output port surrounding whitespace",
			build: func(env *Environment) (DataflowDefinition, error) {
				return DefineDataflow(env, "invalid-output-port").
					CustomPorts("operator", dataflowInvalidGraphFactory, []string{"in"}, []string{"out "}).
					Build()
			},
		},
		{
			name: "input edge into beacon source",
			build: func(env *Environment) (DataflowDefinition, error) {
				return DefineDataflow(env, "invalid-beacon-input").
					Emitter("upstream").
					BeaconSource("source", 1).
					Connect("upstream", "source").
					Build()
			},
		},
		{
			name: "input edge into custom source",
			build: func(env *Environment) (DataflowDefinition, error) {
				return DefineDataflow(env, "invalid-custom-source-input").
					Emitter("upstream").
					CustomSource("source", dataflowInvalidGraphSourceFactory).
					Connect("upstream", "source").
					Build()
			},
		},
		{
			name: "same route ordinary and feedback",
			build: func(env *Environment) (DataflowDefinition, error) {
				return DefineDataflow(env, "invalid-duplicate-route").
					Emitter("first").
					Emitter("second").
					Connect("first", "second").
					ConnectFeedback("first", "second").
					Build()
			},
		},
		{
			name: "forward edge marked as feedback",
			build: func(env *Environment) (DataflowDefinition, error) {
				return DefineDataflow(env, "invalid-forward-feedback").
					Emitter("first").
					Emitter("second").
					ConnectFeedback("first", "second").
					Build()
			},
		},
		{
			name: "cycle made only of feedback edges",
			build: func(env *Environment) (DataflowDefinition, error) {
				return DefineDataflow(env, "invalid-all-feedback-cycle").
					Emitter("first").
					Emitter("second").
					ConnectFeedback("first", "second").
					ConnectFeedback("second", "first").
					Build()
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.build(NewEnvironment())
			if err == nil {
				t.Fatal("invalid graph was accepted")
			}
			if !errors.Is(err, ErrorInvalidRule) {
				t.Fatalf("invalid graph error = %v, want %v", err, ErrorInvalidRule)
			}
		})
	}
}

func TestDataflowFeedbackMustCloseOrdinaryPathAndEntersPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	definition, err := DefineDataflow(env, "valid-feedback-cycle").
		Emitter("entry").
		Custom("first", dataflowInvalidGraphFactory).
		Custom("second", dataflowInvalidGraphFactory).
		Connect("entry", "first").
		Connect("first", "second").
		ConnectFeedback("second", "first").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if edges := definition.Edges(); len(edges) != 3 || !edges[2].Feedback {
		t.Fatalf("feedback graph edges = %#v", edges)
	}

	if _, err := RegisterStruct[dataflowInvalidGraphEvent](env, "DataflowInvalidGraphEvent"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[dataflowInvalidGraphEvent](env, "DataflowInvalidGraphEvent").Query())
	if err != nil {
		t.Fatal(err)
	}
	canonical := string(plan.Canonical())
	if !strings.Contains(canonical, "second:out>first:in:feedback=true") {
		t.Fatalf("plan canonical omitted feedback semantics: %s", canonical)
	}
}
