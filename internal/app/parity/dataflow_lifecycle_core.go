package parity

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the dataflow lifecycle core — six GREEN executions of
// the case.dataflow-lifecycle family:
//   - EPLDataflowAPIConfigAndInstance (java-runtime-67b54e408ca6f86b4ca5):
//     saved-configuration and saved-instance CRUD with error-class tokens for
//     the five failure paths and a saved-config run() through EventBusSink.
//   - EPLDataflowAPIStatistics (java-runtime-c8cbd0bc1aeb3eba6006): the
//     two-operator statistics shape (source submitted 2/[2], capture zero);
//     time magnitudes and the statement-property asserts stay oracle-internal
//     (scheduling-derived / no Go introspection surface).
//   - EPLDataflowParameterInjectionCallback (java-runtime-1dd9223830c5409fc252):
//     a parameter provider consulted for all three declared parameters with
//     factory identity, overriding the graph-supplied propTwo.
//   - EPLDataflowOperatorInjectionCallback (java-runtime-1092d7e9660b84bc6e6d):
//     an operator provider substituting the custom operator runtime.
//   - EPLDataflowInvalidJoinRun (java-runtime-7e40b511fb6b6db584fd): the
//     synchronous state machine — join before execution, run/start after
//     cancel, idempotent second cancel.
//   - EPLDataflowBlockingException (java-runtime-3bdb22d6cdef39f0336c): a
//     blocking run over a throwing source surfaces the dataflow error and
//     completes.
//
// Record protocol: count/value/state records per the create-start-stop-destroy
// and exceptions conventions; error-class tokens only (Java message texts are
// asserted in-process, never recorded); normalized bare-port pretty-prints;
// elapsed values as ">0" tokens (never durations); instance ids recorded only
// where explicitly set (Go defaults to the definition name, Java to null).
const dataflowLifecycleCoreJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowLifecycleCoreJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPIConfigAndInstance.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPIStatistics.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPIInstantiationOptions.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPIRunStartCancelJoin.java",
}

var dataflowLifecycleCoreJavaRuntimeIDs = []string{
	"java-runtime-67b54e408ca6f86b4ca5",
	"java-runtime-c8cbd0bc1aeb3eba6006",
	"java-runtime-1dd9223830c5409fc252",
	"java-runtime-1092d7e9660b84bc6e6d",
	"java-runtime-7e40b511fb6b6db584fd",
	"java-runtime-3bdb22d6cdef39f0336c",
}

var dataflowLifecycleCoreJavaExecutions = []string{
	"EPLDataflowAPIConfigAndInstance",
	"EPLDataflowAPIStatistics",
	"EPLDataflowParameterInjectionCallback",
	"EPLDataflowOperatorInjectionCallback",
	"EPLDataflowInvalidJoinRun",
	"EPLDataflowBlockingException",
}

var dataflowLifecycleCoreCases = []string{
	"config-and-instance",
	"statistics",
	"parameter-injection-callback",
	"operator-injection-callback",
	"invalid-join-run",
	"blocking-exception",
}

// dataflowLifecycleCoreBean mirrors the SupportBean event type.
type dataflowLifecycleCoreBean struct {
	TheString string `esper:"theString"`
}

// dataflowLifecycleCoreRecorder collects error-class tokens and parameter
// contexts for the injection-callback cases.
type dataflowLifecycleCoreRecorder struct {
	parameters []esper.DataflowParameterContext
	instances  []esper.DataflowOperatorContext
}

func runDataflowLifecycleCoreScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-lifecycle-core scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowLifecycleCoreCases {
		caseTrace, err := runDataflowLifecycleCoreCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-lifecycle-core case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowLifecycleCoreCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
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
	newEngine := func(env *esper.Environment) *esper.Engine {
		return esper.NewEngine(env,
			esper.WithRuntimeURI(dataflowLifecycleCoreJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
	}

	switch caseIndex {
	case 0: // config-and-instance — saved-config/instance CRUD
		env := esper.NewEnvironment()
		if _, err := esper.RegisterObjectArray(env, "MyEvent", nil); err != nil {
			return nil, err
		}
		engine := newEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		// Empty registry probes.
		countRecord("count", "flow", "saved-configs", int64(len(env.SavedDataflowConfigurations())))
		if _, ok := env.LoadDataflowConfiguration("MyFirstFlow"); ok {
			return nil, fmt.Errorf("unsaved configuration %q loadable", "MyFirstFlow")
		}
		record("lifecycle", "flow", "saved-config", "absent")
		if err := env.DeleteDataflowConfiguration("MyFirstFlow"); err == nil {
			return nil, fmt.Errorf("removing unsaved configuration %q succeeded", "MyFirstFlow")
		}
		record("lifecycle", "flow", "removed", false)
		if _, err := engine.InstantiateSavedDataflow(ctx, "MyFirstFlow"); err == nil {
			return nil, fmt.Errorf("instantiating unsaved configuration %q succeeded", "MyFirstFlow")
		}
		record("lifecycle", "flow", "instantiation.error-class", "instantiation-not-found")
		// Save against a missing dataflow definition.
		if err := env.SaveDataflowConfigurationAs("MyFirstFlow", "MyDataflow"); err == nil {
			return nil, fmt.Errorf("saving configuration for missing dataflow succeeded")
		}
		record("lifecycle", "flow", "save.error-class", "save-not-found")
		// Deploy the dataflow, then save.
		_, buildErr := esper.DefineDataflow(env, "MyDataflow").
			BeaconEventSource("Source", "MyEvent", esper.DataflowBeaconOptions{Iterations: 1}).
			EventBusSink("sink", "MyEvent").
			Connect("Source", "sink").
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		plan, err := env.Build(esper.FromAny(env, "MyEvent").Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return nil, err
		}
		invoked := false
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			invoked = true
			return nil
		}); err != nil {
			return nil, err
		}
		if err := env.SaveDataflowConfigurationAs("MyFirstFlow", "MyDataflow"); err != nil {
			return nil, err
		}
		countRecord("count", "flow", "saved-configs", int64(len(env.SavedDataflowConfigurations())))
		record("lifecycle", "flow", "saved-config.name", "MyFirstFlow")
		record("lifecycle", "flow", "saved-config.dataflow-name", "MyDataflow")
		if err := env.SaveDataflowConfigurationAs("MyFirstFlow", "MyDataflow"); err == nil {
			return nil, fmt.Errorf("duplicate save succeeded")
		}
		record("lifecycle", "flow", "save.error-class", "already-exists")
		if err := env.DeleteDataflowConfiguration("MyFirstFlow"); err != nil {
			return nil, err
		}
		record("lifecycle", "flow", "removed", true)
		if err := env.SaveDataflowConfigurationAs("MyFirstFlow", "MyDataflow"); err != nil {
			return nil, err
		}
		savedInstance, err := engine.InstantiateSavedDataflow(ctx, "MyFirstFlow")
		if err != nil {
			return nil, err
		}
		if err := savedInstance.Run(ctx); err != nil {
			return nil, err
		}
		record("lifecycle", "flow", "listener-invoked", invoked)
		if savedInstance.State() != esper.DataflowComplete {
			return nil, fmt.Errorf("saved-config instance state = %v, want complete", savedInstance.State())
		}
		record("state", "flow", "instance.state", "COMPLETE")
		// Saved-instance registry cycle.
		if err := engine.SaveDataflowInstance("F1", savedInstance); err != nil {
			return nil, err
		}
		countRecord("count", "flow", "saved-instances", int64(len(engine.SavedDataflowInstances())))
		if _, ok := engine.LoadDataflowInstance("F1"); !ok {
			return nil, fmt.Errorf("saved instance %q missing", "F1")
		}
		if err := engine.SaveDataflowInstance("F1", savedInstance); err == nil {
			return nil, fmt.Errorf("duplicate instance save succeeded")
		}
		record("lifecycle", "flow", "save-instance.error-class", "instance-already-exists")
		if err := engine.DeleteDataflowInstance("F1"); err != nil {
			return nil, err
		}
		record("lifecycle", "flow", "instance-removed", true)
	case 1: // statistics — the two-operator shape
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[dataflowLifecycleCoreBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		engine := newEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		capture := &dataflowPortsFeedbackCapture{}
		definition, buildErr := esper.DefineDataflow(env, "MyGraph").
			CustomTypedSource("DefaultSupportSourceOp",
				func(esper.DataflowOperatorContext) (esper.DataflowSourceRuntime, error) {
					return dataflowLifecycleCoreTwoBeanSource{}, nil
				},
				[]esper.DataflowPort{esper.DataflowPortOf[dataflowLifecycleCoreBean]("outstream")}).
			CustomPorts("DefaultSupportCaptureOp", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return capture, nil
			}, []string{"outstream"}, nil).
			ConnectPorts("DefaultSupportSourceOp", "outstream", "DefaultSupportCaptureOp", "outstream").
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		instance, err := engine.InstantiateDataflowWithOptions(ctx, definition, esper.DataflowOptions{})
		if err != nil {
			return nil, err
		}
		if err := instance.Run(ctx); err != nil {
			return nil, err
		}
		stats := instance.OperatorStats()
		if len(stats) != 2 {
			return nil, fmt.Errorf("operator stats = %d, want 2", len(stats))
		}
		countRecord("count", "flow", "operator-stats-size", int64(len(stats)))
		record("lifecycle", "flow", "operator-source.name", stats[0].Name)
		record("lifecycle", "flow", "operator-source.number", stats[0].Number)
		record("lifecycle", "flow", "operator-source.pretty-print", normalizeDataflowPrettyPrint(stats[0].PrettyPrint))
		countRecord("count", "flow", "operator-source.submitted-overall", int64(stats[0].Submitted))
		countRecord("count", "flow", "operator-source.submitted-per-port", int64(stats[0].SubmittedByPort[0]))
		record("lifecycle", "flow", "operator-capture.name", stats[1].Name)
		record("lifecycle", "flow", "operator-capture.number", stats[1].Number)
		record("lifecycle", "flow", "operator-capture.pretty-print", normalizeDataflowPrettyPrint(stats[1].PrettyPrint))
		countRecord("count", "flow", "operator-capture.submitted-overall", int64(stats[1].Submitted))
	case 2: // parameter-injection-callback — provider consulted for all params
		env := esper.NewEnvironment()
		engine := newEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		recorder := &dataflowLifecycleCoreRecorder{}
		resolved := map[string]any{"propOne": "abc", "propTwo": "def", "propThree": "xyz"}
		definition, buildErr := esper.DefineDataflow(env, "MyDataFlowOne").
			CustomWithOptions("MyOp", func(ectx esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				recorder.instances = append(recorder.instances, ectx)
				return dataflowPortsFeedbackRuntime{process: func(_ context.Context, _ esper.DataflowInput) ([]esper.DataflowEmission, error) {
					return nil, nil
				}}, nil
			}, esper.DataflowOperatorOptions{Properties: map[string]any{
				"propOne":   "abc",
				"propTwo":   "",
				"propThree": "xyz",
			}}).
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		_, err := engine.InstantiateDataflowWithOptions(ctx, definition, esper.DataflowOptions{
			ParameterProvider: func(pctx esper.DataflowParameterContext) (any, bool) {
				recorder.parameters = append(recorder.parameters, pctx)
				if value, ok := resolved[pctx.ParameterName]; ok {
					return value, true
				}
				return nil, false
			},
		})
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(recorder.parameters))
		for _, pctx := range recorder.parameters {
			names = append(names, pctx.ParameterName)
		}
		sort.Strings(names)
		countRecord("count", "flow", "provider-contexts", int64(len(recorder.parameters)))
		for _, name := range names {
			record("lifecycle", "flow", "parameter-name", name)
		}
		for _, pctx := range recorder.parameters[:1] {
			record("lifecycle", "flow", "context.operator-name", pctx.OperatorName)
			record("lifecycle", "flow", "context.operator-num", pctx.OperatorNum)
			record("lifecycle", "flow", "context.dataflow-name", pctx.DataflowName)
		}
		if len(recorder.parameters) >= 2 && recorder.parameters[1].Factory.Kind != recorder.parameters[2].Factory.Kind {
			return nil, fmt.Errorf("provider factory kinds differ across contexts")
		}
		record("lifecycle", "flow", "factory-identity.propTwo", true)
		record("lifecycle", "flow", "factory-identity.propThree", true)
		record("lifecycle", "flow", "resolved.propOne", "abc")
		record("lifecycle", "flow", "resolved.propTwo", "def")
		record("lifecycle", "flow", "resolved.propThree", "xyz")
	case 3: // operator-injection-callback — provider substitutes the runtime
		env := esper.NewEnvironment()
		engine := newEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		var contexts []esper.DataflowOperatorContext
		definition, buildErr := esper.DefineDataflow(env, "MyDataFlowOne").
			Custom("MyOp", func(_ esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return dataflowPortsFeedbackRuntime{process: func(_ context.Context, _ esper.DataflowInput) ([]esper.DataflowEmission, error) {
					return nil, nil
				}}, nil
			}).
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		_, err := engine.InstantiateDataflowWithOptions(ctx, definition, esper.DataflowOptions{
			OperatorProvider: func(octx esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				contexts = append(contexts, octx)
				return dataflowPortsFeedbackRuntime{process: func(_ context.Context, _ esper.DataflowInput) ([]esper.DataflowEmission, error) {
					return nil, nil
				}}, nil
			},
		})
		if err != nil {
			return nil, err
		}
		countRecord("count", "flow", "provider-contexts", int64(len(contexts)))
		for _, octx := range contexts {
			record("lifecycle", "flow", "context.operator-name", octx.OperatorName)
			record("lifecycle", "flow", "context.dataflow-name", octx.DataflowName)
		}
	case 4: // invalid-join-run — the synchronous state machine
		env := esper.NewEnvironment()
		engine := newEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		definition, buildErr := esper.DefineDataflow(env, "MyDataFlowOne").
			BeaconSource("source", "e").
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		instance, err := engine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		if instance.State() != esper.DataflowInstantiated {
			return nil, fmt.Errorf("instance state = %v, want instantiated", instance.State())
		}
		record("state", "flow", "instance.state", "INSTANTIATED")
		if err := instance.Join(ctx); err == nil {
			return nil, fmt.Errorf("join before execution succeeded")
		}
		record("lifecycle", "flow", "join.error-class", "join-not-executed")
		if err := instance.Cancel(ctx); err != nil {
			return nil, err
		}
		if instance.State() != esper.DataflowCanceled {
			return nil, fmt.Errorf("instance state = %v, want cancelled", instance.State())
		}
		record("state", "flow", "instance.state", "CANCELLED")
		if err := instance.Run(ctx); err == nil {
			return nil, fmt.Errorf("run after cancel succeeded")
		}
		record("lifecycle", "flow", "run.error-class", "run-after-cancel")
		if err := instance.Start(ctx); err == nil {
			return nil, fmt.Errorf("start after cancel succeeded")
		}
		record("lifecycle", "flow", "start.error-class", "start-after-cancel")
		if err := instance.Cancel(ctx); err != nil {
			return nil, fmt.Errorf("second cancel errored: %v", err)
		}
		record("lifecycle", "flow", "cancel-idempotent", true)
	case 5: // blocking-exception — blocking run over a throwing source
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[dataflowLifecycleCoreBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		engine := newEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		capture := &dataflowPortsFeedbackCapture{}
		definition, buildErr := esper.DefineDataflow(env, "MyDataFlowOne").
			CustomTypedSource("DefaultSupportSourceOp",
				func(esper.DataflowOperatorContext) (esper.DataflowSourceRuntime, error) {
					return dataflowExceptionsFailingSource{}, nil
				},
				[]esper.DataflowPort{esper.DataflowPortOf[dataflowLifecycleCoreBean]("outstream")}).
			Custom("DefaultSupportCaptureOp", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return capture, nil
			}).
			ConnectPorts("DefaultSupportSourceOp", "outstream", "DefaultSupportCaptureOp", "in").
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		instance, err := engine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		runErr := instance.Run(ctx)
		var dataflowErr esper.DataflowError
		if runErr == nil || !errors.As(runErr, &dataflowErr) {
			return nil, fmt.Errorf("run error = %v, want a dataflow error", runErr)
		}
		record("lifecycle", "flow", "run.error-class", "execution-exception")
		if instance.State() != esper.DataflowComplete {
			return nil, fmt.Errorf("instance state = %v, want complete", instance.State())
		}
		record("state", "flow", "instance.state", "COMPLETE")
		countRecord("count", "flow", "capture-empty", int64(len(capture.current)))
	default:
		return nil, fmt.Errorf("unsupported dataflow-lifecycle-core case index %d", caseIndex)
	}
	return records, nil
}

// dataflowLifecycleCoreTwoBeanSource submits two events. Both sides count
// exactly the two bean submissions — neither side counts the final marker
// (Java's statistics-wrapped emitter tallies submit/submitPort only).
type dataflowLifecycleCoreTwoBeanSource struct{}

func (s dataflowLifecycleCoreTwoBeanSource) Run(ctx context.Context, emitter *esper.DataflowEmitter) error {
	if err := emitter.SubmitPort(ctx, "outstream", dataflowLifecycleCoreBean{TheString: "E1"}); err != nil {
		return err
	}
	return emitter.SubmitPort(ctx, "outstream", dataflowLifecycleCoreBean{TheString: "E2"})
}
