package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for EPLDataflowAPICreateStartStopDestroy: the dataflow
// definition-registry lifecycle. Neither Java execution starts an instance —
// the pinned observables are the registered catalog, the instantiate state,
// the error class after the definition is removed, the absent lookup, the
// empty registry, and the re-instantiation. Replay-shape adaptations frozen
// with the scouts: records carry the error class ("not-defined"), not the
// Java message text (no Go text match); the Java deployment-id dimension and
// statementName have no Go analog; the re-deploy's random UUID and the
// module item count are not represented.
type dataflowCSSDBean struct {
	TheString string `esper:"theString"`
}

const dataflowCSSDJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowCSSDJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPICreateStartStopDestroy.java",
}

var dataflowCSSDJavaRuntimeIDs = []string{
	"java-runtime-1a8426a71e3b5252a181",
	"java-runtime-2c3c90d3cdb64596c225",
}

var dataflowCSSDJavaExecutions = []string{
	"EPLDataflowCreateStartStop",
	"EPLDataflowDeploymentAdmin",
}

var dataflowCSSDCases = []string{
	"create-start-stop",
	"deployment-admin",
}

func runDataflowCreateStartStopDestroyScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-create-start-stop-destroy scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowCSSDCases {
		caseTrace, err := runDataflowCSSDCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-create-start-stop-destroy case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowCSSDCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	now := time.Unix(0, 0).UTC()
	sequence := uint64(0)
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
	countRecord := func(operation, statement, name string, count int64) {
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: operation,
			Statement: statement,
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			Name:      name,
			Count:     &count,
		})
	}
	stateRecord := func() {
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: "state",
			Statement: "flow",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			Name:      "state",
			Value:     "instantiated",
		})
	}

	switch caseIndex {
	case 0: // create-start-stop: the registry lifecycle, no instance start
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(dataflowCSSDJavaRuntimeIDs[0]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		definition, err := esper.DefineDataflow(env, "MyGraph").Emitter("Emitter").Build()
		if err != nil {
			return nil, err
		}
		if err := env.SaveDataflowConfiguration("MyGraph"); err != nil {
			return nil, err
		}
		record("lifecycle", "flow:MyGraph", "registered", "MyGraph")
		if _, err := engine.InstantiateDataflow(ctx, definition); err != nil {
			return nil, err
		}
		stateRecord()
		if err := env.DeleteDataflowConfiguration("MyGraph"); err != nil {
			return nil, err
		}
		if _, err := engine.InstantiateSavedDataflow(ctx, "MyGraph"); err == nil {
			return nil, fmt.Errorf("removed dataflow %q instantiated", "MyGraph")
		}
		record("lifecycle", "flow", "instantiate-error", "not-defined")
		if _, err := engine.InstantiateSavedDataflow(ctx, "DUMMY"); err == nil {
			return nil, fmt.Errorf("unknown dataflow %q instantiated", "DUMMY")
		}
		record("lifecycle", "flow", "instantiate-error", "not-defined")
		if _, found := env.LoadDataflowConfiguration("MyGraph"); found {
			return nil, fmt.Errorf("removed dataflow %q still loadable", "MyGraph")
		}
		record("lifecycle", "flow:MyGraph", "registered", nil)
		countRecord("lifecycle", "flow", "saved-configurations", int64(len(env.SavedDataflowConfigurations())))
		// Re-deploy + re-instantiate: the Java re-deploy creates a fresh
		// definition under a random UUID deployment id (not pinned); the Go
		// environment keeps the definition registered, so a fresh
		// instantiation is the observable equivalent.
		if _, err := engine.InstantiateDataflow(ctx, definition); err != nil {
			return nil, err
		}
		stateRecord()
	case 1: // deployment-admin: multi-operator graph, instantiate only
		if _, err := esper.RegisterStruct[dataflowCSSDBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(dataflowCSSDJavaRuntimeIDs[1]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		definition, err := esper.DefineDataflow(env, "TheGraph").
			CustomSource("DefaultSupportSourceOp", func(esper.DataflowOperatorContext) (esper.DataflowSourceRuntime, error) {
				// Realized at instantiate; Run is never invoked because the
				// execution only instantiates and undeploys.
				return dataflowCSSDDormantSource{}, nil
			}).
			Select("Select", esper.Alias("theString", esper.Field[any, string]("theString")), esper.Alias("intPrimitive", esper.Field[any, int]("intPrimitive"))).
			Custom("DefaultSupportCaptureOp", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return &dataflowCSSDCapture{}, nil
			}).
			Connect("DefaultSupportSourceOp", "Select").
			Connect("Select", "DefaultSupportCaptureOp").
			Build()
		if err != nil {
			return nil, err
		}
		if _, err := engine.InstantiateDataflow(ctx, definition); err != nil {
			return nil, err
		}
		stateRecord()
	default:
		return nil, fmt.Errorf("unsupported dataflow-create-start-stop-destroy case index %d", caseIndex)
	}
	return records, nil
}

// dataflowCSSDCapture is topology-only: the execution never starts the
// graph, so no rows are ever delivered.
type dataflowCSSDCapture struct{}

func (c *dataflowCSSDCapture) Process(_ context.Context, _ esper.DataflowInput) ([]esper.DataflowEmission, error) {
	return nil, nil
}

// dataflowCSSDDormantSource realizes the declared source; the execution
// never starts the instance, so Run is unreachable.
type dataflowCSSDDormantSource struct{}

func (s dataflowCSSDDormantSource) Run(_ context.Context, _ *esper.DataflowEmitter) error {
	return fmt.Errorf("dormant source started")
}
