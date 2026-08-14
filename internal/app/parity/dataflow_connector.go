package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type dataflowConnectorBean struct {
	P0 string `esper:"p0"`
	P1 int64  `esper:"p1"`
}

const dataflowConnectorJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowConnectorJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpEventBusSink.java",
}

var (
	dataflowConnectorJavaRuntimeIDs = []string{
		"java-runtime-1ecb769e10b4a818772c",
	}
	dataflowConnectorJavaExecutions = []string{
		"EPLDataflowBeacon",
	}
)

func runDataflowConnectorScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if !scenarioHasCase(scenario, "connector") {
		return compat.Trace{}, fmt.Errorf("dataflow connector scenario %q has no supported cases", scenario.ID)
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[dataflowConnectorBean](env, "MyEventBeacon"); err != nil {
		return compat.Trace{}, err
	}
	definition, err := esper.DefineDataflow(env, "MyDataFlowOne").
		BeaconEventSource("source", "MyEventBeacon", esper.DataflowBeaconOptions{Iterations: 3},
			esper.Alias("p0", esper.Literal("abc")),
			esper.Alias("p1", esper.Literal(int64(1))),
		).
		EventBusSink("sink", "MyEventBeacon").
		Build()
	if err != nil {
		return compat.Trace{}, err
	}
	plan, err := env.Build(esper.Select(
		esper.From[dataflowConnectorBean](env, "MyEventBeacon"),
		esper.Alias("p0", esper.Field[dataflowConnectorBean, string]("p0")),
		esper.Alias("p1", esper.Field[dataflowConnectorBean, int64]("p1")),
	).Query(esper.StatementName("s0")))
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	if len(deployment.Statements()) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one statement, got %d", len(deployment.Statements()))
	}
	statement := deployment.Statements()[0]
	batches := make([]esper.ResultBatch, 0, 3)
	batchChannel := make(chan esper.ResultBatch, 8)
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		batchChannel <- batch
		return nil
	}); err != nil {
		return compat.Trace{}, err
	}
	instance, err := engine.InstantiateDataflow(ctx, definition)
	if err != nil {
		return compat.Trace{}, err
	}
	if err := instance.Start(ctx); err != nil {
		return compat.Trace{}, err
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for len(batches) < 3 {
		select {
		case batch := <-batchChannel:
			batches = append(batches, batch)
		case <-timer.C:
			timer.Stop()
			return compat.Trace{}, fmt.Errorf("dataflow connector produced %d batches, want 3", len(batches))
		}
	}
	if len(batches) != 3 {
		return compat.Trace{}, fmt.Errorf("dataflow connector produced %d batches, want 3", len(batches))
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for index, batch := range batches {
		record := compat.TraceRecord{
			Case:      "connector",
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  uint64(index + 1),
			Time:      batch.Time.UTC().Format(time.RFC3339Nano),
			New:       compat.NormalizeResults(batch.New),
		}
		trace.Records = append(trace.Records, record)
	}
	return trace, nil
}
