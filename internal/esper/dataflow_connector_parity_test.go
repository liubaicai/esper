package esper

import (
	"context"
	"testing"
	"time"
)

type dataflowConnectorParityBean struct {
	P0 string `esper:"p0"`
	P1 int64  `esper:"p1"`
}

// TestDataflowConnectorOutputParity mirrors the shared
// dataflow-connector-output parity scenario (Java EPLDataflowBeacon):
// BeaconSource emits three typed events, EventBusSink routes them onto the
// event bus, and a statement listener observes all three.
func TestDataflowConnectorOutputParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dataflowConnectorParityBean](env, "MyEventBeacon"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "MyDataFlowOne").
		BeaconEventSource("source", "MyEventBeacon", DataflowBeaconOptions{Iterations: 3},
			Alias("p0", Literal("abc")),
			Alias("p1", Literal(int64(1))),
		).
		EventBusSink("sink", "MyEventBeacon").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(
		From[dataflowConnectorParityBean](env, "MyEventBeacon"),
		Alias("p0", Field[dataflowConnectorParityBean, string]("p0")),
		Alias("p1", Field[dataflowConnectorParityBean, int64]("p1")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	batchChannel := make(chan ResultBatch, 8)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batchChannel <- batch
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for len(batches) < 3 {
		select {
		case batch := <-batchChannel:
			batches = append(batches, batch)
		case <-timer.C:
			timer.Stop()
			t.Fatalf("dataflow connector batches = %d, want 3", len(batches))
		}
	}
	if len(batches) != 3 {
		t.Fatalf("dataflow connector batches = %d, want 3", len(batches))
	}
	for _, batch := range batches {
		if len(batch.New) != 1 {
			t.Fatalf("batch = %#v, want one row", batch)
		}
		row, ok := batch.New[0].Row()
		if !ok || row.Get("p0").Any() != "abc" || row.Get("p1").Any() != int64(1) {
			t.Fatalf("batch row = %#v, want abc/1", batch.New[0])
		}
	}
}
