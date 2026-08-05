package esper

import (
	"context"
	"testing"
)

type dataflowStatsSource struct{}

func (dataflowStatsSource) Run(ctx context.Context, emitter *DataflowEmitter) error {
	for _, value := range []int{1, 2} {
		if err := emitter.Submit(ctx, value); err != nil {
			return err
		}
	}
	return nil
}

func TestDataflowOperatorStatsMatchEsperSubmissionShape(t *testing.T) {
	env := NewEnvironment()
	definition, err := DefineDataflow(env, "operator-stats-flow").
		CustomTypedSource("source", func(DataflowOperatorContext) (DataflowSourceRuntime, error) {
			return dataflowStatsSource{}, nil
		}, []DataflowPort{DataflowPortOf[int]("out")}).
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	stats := instance.OperatorStats()
	if len(stats) != 2 {
		t.Fatalf("operator stats length = %d, want 2: %#v", len(stats), stats)
	}
	if stats[0].Name != "source" || stats[0].Number != 0 || stats[0].PrettyPrint != "source#0" {
		t.Fatalf("source metadata = %#v", stats[0])
	}
	if stats[0].Submitted != 2 || len(stats[0].SubmittedByPort) != 1 || stats[0].SubmittedByPort[0] != 2 {
		t.Fatalf("source submission stats = %#v", stats[0])
	}
	if stats[1].Name != "sink" || stats[1].Number != 1 || stats[1].Submitted != 0 || len(stats[1].SubmittedByPort) != 0 {
		t.Fatalf("terminal operator stats = %#v", stats[1])
	}
	if stats[0].Elapsed < 0 || stats[1].Elapsed < 0 || len(stats[0].ElapsedByPort) != 1 {
		t.Fatalf("operator elapsed stats = %#v", stats)
	}

	stats[0].SubmittedByPort[0] = 99
	if fresh := instance.OperatorStats(); fresh[0].SubmittedByPort[0] != 2 {
		t.Fatalf("operator stats returned aliased port slice = %#v", fresh[0])
	}
	if outputs := instance.Outputs(); len(outputs) != 2 || outputs[0] != 1 || outputs[1] != 2 {
		t.Fatalf("operator stats flow outputs = %#v", outputs)
	}
}
