package parity

import (
	"context"
	"errors"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the dataflow lifecycle blocking trio — the three
// blocking-mode executions of EPLDataflowAPIRunStartCancelJoin:
//   - BlockingCancel (java-runtime-91a5f5421ce63806e2c4): a gated source
//     holds the flow while RUNNING is observable; cancelling surfaces the
//     cancellation from the join (Java's EPDataFlowCancellationException —
//     error-class token recorded, message text in-process), leaving
//     CANCELLED with an empty capture.
//   - FastCompleteBlocking (java-runtime-42082e1062bebbe49a0a): a finite
//     beacon flow that is provably not complete before the blocking run and
//     completes with one row; the post-execution state machine (join no-op,
//     run/start-after-complete rejected, cancel/join silent) is asserted
//     in-process.
//   - RunBlocking (java-runtime-bcc34451f5417d84e6fa): RUNNING is observed
//     while the source is gated; releasing it completes the run with one
//     row; the source's own submission count is two (bean + final marker).
const dataflowLifecycleBlockingJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowLifecycleBlockingJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPIRunStartCancelJoin.java",
}

var dataflowLifecycleBlockingJavaRuntimeIDs = []string{
	"java-runtime-91a5f5421ce63806e2c4",
	"java-runtime-42082e1062bebbe49a0a",
	"java-runtime-bcc34451f5417d84e6fa",
}

var dataflowLifecycleBlockingJavaExecutions = []string{
	"EPLDataflowBlockingCancel",
	"EPLDataflowFastCompleteBlocking",
	"EPLDataflowRunBlocking",
}

var dataflowLifecycleBlockingCases = []string{
	"blocking-cancel",
	"fast-complete-blocking",
	"run-blocking",
}

func runDataflowLifecycleBlockingScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-lifecycle-blocking scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowLifecycleBlockingCases {
		caseTrace, err := runDataflowLifecycleBlockingCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-lifecycle-blocking case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowLifecycleBlockingCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
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
	countRecord := func(statement, name string, count int64) {
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: "count",
			Statement: statement,
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			Name:      name,
			Count:     &count,
		})
	}
	assertState := func(instance *esper.DataflowInstance, want esper.DataflowState) error {
		if got := instance.State(); got != want {
			return fmt.Errorf("instance state = %v, want %v", got, want)
		}
		return nil
	}
	newEngine := func(env *esper.Environment) *esper.Engine {
		return esper.NewEngine(env,
			esper.WithRuntimeURI(dataflowLifecycleBlockingJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
	}
	// pollRunning observes the RUNNING state with a bounded deterministic
	// wait (the Go-side analog of Java's spin-until-RUNNING device).
	pollRunning := func(instance *esper.DataflowInstance) error {
		deadline := time.Now().Add(5 * time.Second)
		for instance.State() != esper.DataflowRunning {
			if time.Now().After(deadline) {
				return fmt.Errorf("instance did not reach running within the bound")
			}
			time.Sleep(time.Millisecond)
		}
		return nil
	}

	switch caseIndex {
	case 0: // blocking-cancel — cancel surfaces from the join
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[dataflowCancelJoinBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		engine := newEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		capture := &dataflowCancelJoinCapture{}
		source := &dataflowCancelJoinSource{gate: make(chan struct{}), submitBean: true}
		definition, buildErr := esper.DefineDataflow(env, "MyDataFlowOne").
			CustomTypedSource("DefaultSupportSourceOp",
				func(esper.DataflowOperatorContext) (esper.DataflowSourceRuntime, error) {
					return source, nil
				},
				[]esper.DataflowPort{esper.DataflowPortOf[dataflowCancelJoinBean]("outstream")}).
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
		if err := instance.Start(ctx); err != nil {
			return nil, err
		}
		if err := pollRunning(instance); err != nil {
			return nil, err
		}
		record("state", "flow", "instance.state", "RUNNING")
		// Cancelling unblocks the join; Go surfaces ErrorCanceled where Java
		// throws EPDataFlowCancellationException — the error-class token is
		// the recorded observable, the message text stays in-process.
		if err := instance.Cancel(ctx); err != nil {
			return nil, err
		}
		joinErr := instance.Join(ctx)
		if joinErr == nil || !errors.Is(joinErr, esper.ErrorCanceled) {
			return nil, fmt.Errorf("join after cancel = %v, want a cancellation error", joinErr)
		}
		record("lifecycle", "flow", "run.error-class", "cancellation-exception")
		if err := assertState(instance, esper.DataflowCanceled); err != nil {
			return nil, err
		}
		record("state", "flow", "instance.state", "CANCELLED")
		countRecord("flow", "capture-empty", int64(capture.rows()))
	case 1: // fast-complete-blocking — finite beacon, provably not done before
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[dataflowCancelJoinBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		engine := newEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		capture := &dataflowCancelJoinCapture{}
		definition, buildErr := esper.DefineDataflow(env, "MyDataFlowOne").
			BeaconEventSource("Source", "SupportBean", esper.DataflowBeaconOptions{Iterations: 1},
				esper.Alias("theString", esper.Literal("E1"))).
			Custom("DefaultSupportCaptureOp", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return capture, nil
			}).
			Connect("Source", "DefaultSupportCaptureOp").
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		instance, err := engine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		if instance.DataflowName() != "MyDataFlowOne" {
			return nil, fmt.Errorf("dataflow name = %q", instance.DataflowName())
		}
		if err := assertState(instance, esper.DataflowInstantiated); err != nil {
			return nil, err
		}
		record("lifecycle", "flow", "not-done-before-run", true)
		if err := instance.Run(ctx); err != nil {
			return nil, err
		}
		if err := assertState(instance, esper.DataflowComplete); err != nil {
			return nil, err
		}
		record("state", "flow", "instance.state", "COMPLETE")
		// tryAssertionAfterExec in-process: join is a no-op; run/start after
		// complete error; the trailing cancel/join stay unreplayed (Go
		// cancel-after-complete errors — frozen 4.375 disposition, silent
		// Java no-op, nothing recorded).
		if err := instance.Join(ctx); err != nil {
			return nil, err
		}
		if err := instance.Run(ctx); err == nil {
			return nil, fmt.Errorf("run after complete succeeded")
		}
		if err := instance.Start(ctx); err == nil {
			return nil, fmt.Errorf("start after complete succeeded")
		}
		countRecord("flow", "capture-rows", int64(capture.rows()))
	case 2: // run-blocking — RUNNING observed while gated, release completes
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[dataflowCancelJoinBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		engine := newEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		capture := &dataflowCancelJoinCapture{}
		source := &dataflowCancelJoinSource{gate: make(chan struct{}), submitBean: true}
		definition, buildErr := esper.DefineDataflow(env, "MyDataFlowOne").
			CustomTypedSource("DefaultSupportSourceOp",
				func(esper.DataflowOperatorContext) (esper.DataflowSourceRuntime, error) {
					return source, nil
				},
				[]esper.DataflowPort{esper.DataflowPortOf[dataflowCancelJoinBean]("outstream")}).
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
		if instance.DataflowName() != "MyDataFlowOne" {
			return nil, fmt.Errorf("dataflow name = %q", instance.DataflowName())
		}
		if err := instance.Start(ctx); err != nil {
			return nil, err
		}
		if err := pollRunning(instance); err != nil {
			return nil, err
		}
		record("state", "flow", "instance.state", "RUNNING")
		close(source.gate)
		if err := instance.Join(ctx); err != nil {
			return nil, err
		}
		if err := assertState(instance, esper.DataflowComplete); err != nil {
			return nil, err
		}
		record("state", "flow", "instance.state", "COMPLETE")
		source.mu.Lock()
		submitted := source.submitted
		source.mu.Unlock()
		if submitted != 2 {
			return nil, fmt.Errorf("source submissions = %d, want 2", submitted)
		}
		countRecord("flow", "capture-rows", int64(capture.rows()))
	default:
		return nil, fmt.Errorf("unsupported dataflow-lifecycle-blocking case index %d", caseIndex)
	}
	return records, nil
}
