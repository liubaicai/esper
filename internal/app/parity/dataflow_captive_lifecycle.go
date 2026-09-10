package parity

import (
	"context"
	"fmt"
	"sync"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the dataflow captive lifecycle:
//   - EPLDataflowAPIStartCaptive (java-runtime-407e42478546e709d774, the
//     whole class is one direct execution): the captive start/emitter/signal
//     contract — startCaptive returns zero runnables and one emitter keyed by
//     the Emitter NAME parameter 'src1'; submits accumulate in the capture's
//     current batch; a FinalMarker signal flushes current into received
//     without producing a row and without completing the instance (it stays
//     RUNNING); a fresh submit lands in a new current; cancel is the only
//     exit and yields CANCELLED. The HelloWorld doc-sample flow replays as
//     instantiate-only (state INSTANTIATED).
//
// Record protocol (create-start-stop-destroy conventions): count records,
// state records, and capture reads over flow:DefaultSupportCaptureOp with
// case-local sequences and fixed epoch time. The captured row is the
// emitter-materialized event of a registered struct — Java passes the raw
// Object[] through uncoerced; Go materializes via the registered type
// (documented adaptation, rows normalize identically).
const dataflowCaptiveLifecycleJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowCaptiveLifecycleJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPIStartCaptive.java",
}

var dataflowCaptiveLifecycleJavaRuntimeIDs = []string{
	"java-runtime-407e42478546e709d774",
}

var dataflowCaptiveLifecycleJavaExecutions = []string{
	"EPLDataflowAPIStartCaptive",
}

var dataflowCaptiveLifecycleCases = []string{
	"captive-emitter",
}

// dataflowCaptiveLifecycleBean mirrors the MyOAEventType object-array schema.
type dataflowCaptiveLifecycleBean struct {
	P0 string `esper:"p0"`
	P1 int    `esper:"p1"`
}

// dataflowCaptiveLifecycleCapture mirrors DefaultSupportCaptureOp: submits
// accumulate in current; a signal flushes current into received.
type dataflowCaptiveLifecycleCapture struct {
	mu       sync.Mutex
	current  []esper.Event
	received [][]esper.Event
}

func (c *dataflowCaptiveLifecycleCapture) Process(_ context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = append(c.current, input.Value.(esper.Event))
	return nil, nil
}

func (c *dataflowCaptiveLifecycleCapture) OnSignal(_ context.Context, _ esper.DataflowSignal) ([]esper.DataflowEmission, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.received = append(c.received, c.current)
	c.current = nil
	return nil, nil
}

func (c *dataflowCaptiveLifecycleCapture) currentSnapshot() []esper.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]esper.Event(nil), c.current...)
}

// takeReceived returns the first received batch and drops it — Java's
// getAndReset returns all batches; identical for this single-signal trace.
func (c *dataflowCaptiveLifecycleCapture) takeReceived() []esper.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.received) == 0 {
		return nil
	}
	batch := c.received[0]
	c.received = c.received[1:]
	return batch
}

func runDataflowCaptiveLifecycleScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-captive-lifecycle scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowCaptiveLifecycleCases {
		caseTrace, err := runDataflowCaptiveLifecycleCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-captive-lifecycle case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowCaptiveLifecycleCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	now := time.Unix(0, 0).UTC()
	sequence := uint64(0)
	var records []compat.TraceRecord
	emit := func(statement string, events []esper.Event) {
		sequence++
		rows := make([]compat.ResultRecord, 0, len(events))
		for _, event := range events {
			rows = append(rows, compat.NormalizeEvents([]esper.Event{event})...)
		}
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: "capture",
			Statement: statement,
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			New:       rows,
		})
	}
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

	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[dataflowCaptiveLifecycleBean](env, "MyOAEventType"); err != nil {
		return nil, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(dataflowCaptiveLifecycleJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(now),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	// Flow A — captive emitter/signal lifecycle over a capture sink.
	capture := &dataflowCaptiveLifecycleCapture{}
	definition, buildErr := esper.DefineDataflow(env, "MyDataFlow").
		Emitter("src1").
		Custom("DefaultSupportCaptureOp", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
			return capture, nil
		}).
		Connect("src1", "DefaultSupportCaptureOp").
		Build()
	if buildErr != nil {
		return nil, buildErr
	}
	instance, err := engine.InstantiateDataflow(ctx, definition)
	if err != nil {
		return nil, err
	}
	captive, err := instance.StartCaptive(ctx)
	if err != nil {
		return nil, err
	}
	if len(captive.Runnables()) != 0 {
		return nil, fmt.Errorf("runnables = %d, want 0", len(captive.Runnables()))
	}
	if len(captive.Emitters()) != 1 {
		return nil, fmt.Errorf("emitters = %d, want 1", len(captive.Emitters()))
	}
	emitter, ok := captive.Emitters()["src1"]
	if !ok || emitter == nil {
		return nil, fmt.Errorf("emitter %q missing from the captive map", "src1")
	}
	countRecord("count", "flow", "runnables", int64(len(captive.Runnables())))
	countRecord("count", "flow", "emitters", int64(len(captive.Emitters())))
	if instance.State() != esper.DataflowRunning {
		return nil, fmt.Errorf("instance state = %v, want running", instance.State())
	}
	record("state", "flow", "instance.state", "RUNNING")
	if err := emitter.Submit(ctx, dataflowCaptiveLifecycleBean{P0: "E1", P1: 10}); err != nil {
		return nil, err
	}
	emit("flow:DefaultSupportCaptureOp", capture.currentSnapshot())
	if err := emitter.Submit(ctx, dataflowCaptiveLifecycleBean{P0: "E2", P1: 20}); err != nil {
		return nil, err
	}
	emit("flow:DefaultSupportCaptureOp", capture.currentSnapshot())
	if err := emitter.SubmitSignal(ctx, esper.FinalMarker{}); err != nil {
		return nil, err
	}
	// The signal is not a row: current is empty after the flush.
	emit("flow:DefaultSupportCaptureOp", capture.currentSnapshot())
	// getAndReset: the received batch carries both rows.
	emit("flow:DefaultSupportCaptureOp", capture.takeReceived())
	if err := emitter.Submit(ctx, dataflowCaptiveLifecycleBean{P0: "E3", P1: 30}); err != nil {
		return nil, err
	}
	emit("flow:DefaultSupportCaptureOp", capture.currentSnapshot())
	// The captive instance has no completion path: still running.
	if instance.State() != esper.DataflowRunning {
		return nil, fmt.Errorf("instance state = %v, want running", instance.State())
	}
	record("state", "flow", "instance.state", "RUNNING")
	if err := instance.Cancel(ctx); err != nil {
		return nil, err
	}
	if instance.State() != esper.DataflowCanceled {
		return nil, fmt.Errorf("instance state = %v, want cancelled", instance.State())
	}
	record("state", "flow", "instance.state", "CANCELLED")

	// Flow B — the doc-sample flow, instantiate-only (real LogSink operator,
	// default providers, never started).
	docDefinition, buildErr := esper.DefineDataflow(env, "HelloWorldDataFlow").
		Emitter("myemitter").
		LogSink("sink", func(_ context.Context, _ any) error { return nil }).
		Connect("myemitter", "sink").
		Build()
	if buildErr != nil {
		return nil, buildErr
	}
	if _, err := engine.InstantiateDataflow(ctx, docDefinition); err != nil {
		return nil, err
	}
	record("state", "flow", "instance.state", "INSTANTIATED")
	return records, nil
}
