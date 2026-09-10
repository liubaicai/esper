package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the dataflow documentation-sample executions:
//   - EPLDataflowDocSamplesRun (java-runtime-b41da4e41f347dcd49a8): the
//     hello-world BeaconSource -> LogSink graph drained by the blocking run,
//     pinning the INSTANTIATED -> COMPLETE state transitions. The suite's 14
//     parse-only EPL grammar fragments are asserted internally and produce no
//     trace rows (no Go parser surface — documented).
//   - EPLDataflowDocSamples (java-runtime-aaf36f532374aca5a232, from
//     EPLDataflowOpSelect.java): a five-Select graph instantiated only — the
//     instance stays INSTANTIATED and is never started (two sources would make
//     run() throw) — then undeployed.
const dataflowDocSamplesJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowDocSamplesJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowDocSamples.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpSelect.java",
}

var dataflowDocSamplesJavaRuntimeIDs = []string{
	"java-runtime-b41da4e41f347dcd49a8",
	"java-runtime-aaf36f532374aca5a232",
}

var dataflowDocSamplesJavaExecutions = []string{
	"EPLDataflowDocSamplesRun",
	"EPLDataflowDocSamples",
}

var dataflowDocSamplesCases = []string{
	"doc-run",
	"doc-flow",
}

// dataflowDocSamplesCapture records delivered events in arrival order.
type dataflowDocSamplesCapture struct {
	values []any
}

func (c *dataflowDocSamplesCapture) Process(_ context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
	c.values = append(c.values, input.Value)
	return nil, nil
}

func stateName(instance *esper.DataflowInstance) string {
	switch instance.State() {
	case esper.DataflowInstantiated:
		return "INSTANTIATED"
	case esper.DataflowComplete:
		return "COMPLETE"
	case esper.DataflowRunning:
		return "RUNNING"
	case esper.DataflowCanceled:
		return "CANCELLED"
	}
	return "UNKNOWN"
}

func runDataflowDocSamplesScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-doc-samples scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowDocSamplesCases {
		caseTrace, err := runDataflowDocSamplesCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-doc-samples case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowDocSamplesCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(dataflowDocSamplesJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := uint64(0)
	now := time.Unix(0, 0).UTC()
	var records []compat.TraceRecord
	record := func(operation, statement, name string, value any) {
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: operation,
			Statement: statement,
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			Name:      name,
			Value:     value,
		})
	}

	switch caseIndex {
	case 0: // doc-run — hello-world graph, blocking run
		definition, buildErr := esper.DefineDataflow(env, "HelloWorldDataFlow").
			BeaconSourceWithOptions("BeaconSource", esper.DataflowBeaconOptions{Iterations: 1},
				map[string]any{"text": "hello world"}).
			LogSink("LogSink", func(_ context.Context, _ any) error { return nil }).
			Connect("BeaconSource", "LogSink").
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		record("deploy", "flow", "statementCount", 1)
		record("deploy", "flow", "statementName", "flow")
		instance, err := engine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		record("state", "flow:HelloWorldDataFlow", "instance.state", stateName(instance))
		if err := instance.Run(ctx); err != nil {
			return nil, err
		}
		record("state", "flow:HelloWorldDataFlow", "instance.state", stateName(instance))
	case 1: // doc-flow — five-Select graph, instantiate-only (never started:
		// two sources would make run() throw), then undeployed.
		definition, buildErr := esper.DefineDataflow(env, "MyDataFlow").
			Emitter("s0").Emitter("s1").
			Select("select-0", esper.Alias("tagId", esper.Field[any, string]("tagId"))).
			Select("select-1", esper.Alias("locX", esper.Field[any, float64]("locX"))).
			SelectIterate("select-2", nil, nil,
				esper.Alias("tagId", esper.Field[any, string]("tagId"))).
			SelectJoin("select-3", esper.DataflowJoinOptions{Inputs: 2},
				esper.Alias("tagId", esper.Field[any, string]("tagId")),
				esper.Alias("locX", esper.Field[any, float64]("locX"))).
			Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return &dataflowDocSamplesCapture{}, nil
			}).
			Connect("s0", "select-0").Connect("s0", "select-2").
			ConnectInput("s0", "select-3", 0).ConnectInput("s1", "select-3", 1).
			Connect("s1", "select-1").
			Connect("select-0", "capture").Connect("select-1", "capture").
			Connect("select-2", "capture").Connect("select-3", "capture").
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		record("deploy", "flow", "statementCount", 1)
		record("deploy", "flow", "statementName", "flow")
		instance, err := engine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		record("state", "flow:MyDataFlow", "instance.state", stateName(instance))
		if err := instance.Cancel(ctx); err != nil {
			return nil, err
		}
		record("undeploy", "flow", "deploymentCount", 0)
	default:
		return nil, fmt.Errorf("unsupported dataflow-doc-samples case index %d", caseIndex)
	}
	return records, nil
}
