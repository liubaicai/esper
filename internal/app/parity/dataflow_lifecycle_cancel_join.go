package parity

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the dataflow lifecycle cancel/join family — the seven
// start-mode executions of EPLDataflowAPIRunStartCancelJoin:
//   - NonBlockingJoinCancel (java-runtime-893c8283ee90019f37fa): join returns
//     only via cancel; state CANCELLED; capture never filled.
//   - NonBlockingJoinException (java-runtime-e1bdb19779e28fa49bc2): the
//     source exception is swallowed in start mode; the instance completes.
//     Go surfaces the same failure from Join — documented semantic
//     difference; the shared observables are COMPLETE and the empty capture.
//   - NonBlockingException (java-runtime-69dbb4e0d879421b52f7): an
//     immediately-throwing source still completes the instance.
//   - NonBlockingCancel (java-runtime-f053fbdc01fad03ba5a6): RUNNING is
//     observable right after start; cancel is synchronous.
//   - NonBlockingJoinMultipleRunnable (java-runtime-827b0af4ea14c59da2bc):
//     the instance stays RUNNING while either source remains gated, and
//     join waits for both.
//   - NonBlockingJoinSingleRunnable (java-runtime-a9a21d69ccb78f9fd152):
//     the single-gated-source completion shape (the Java getCurrentCount==2
//     and cancel-after-COMPLETE assertions are in-process; Go's
//     Cancel-on-Complete errors and is never recorded).
//   - FastCompleteNonBlocking (java-runtime-de4fc2179a2cb72a97e6): a finite
//     beacon flow completing on its own, with the tryAssertionAfterExec
//     state machine (join no-op, run/start-after-complete rejected) asserted
//     in-process.
//
// Java's sleep/spin waits are replaced by bounded deterministic waits that
// cannot change the outcome; delay values are never recorded. Error message
// texts stay in-process (invalidity policy).
const dataflowLifecycleCancelJoinJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowLifecycleCancelJoinJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPIRunStartCancelJoin.java",
}

var dataflowLifecycleCancelJoinJavaRuntimeIDs = []string{
	"java-runtime-893c8283ee90019f37fa",
	"java-runtime-e1bdb19779e28fa49bc2",
	"java-runtime-69dbb4e0d879421b52f7",
	"java-runtime-f053fbdc01fad03ba5a6",
	"java-runtime-827b0af4ea14c59da2bc",
	"java-runtime-a9a21d69ccb78f9fd152",
	"java-runtime-de4fc2179a2cb72a97e6",
}

var dataflowLifecycleCancelJoinJavaExecutions = []string{
	"EPLDataflowNonBlockingJoinCancel",
	"EPLDataflowNonBlockingJoinException",
	"EPLDataflowNonBlockingException",
	"EPLDataflowNonBlockingCancel",
	"EPLDataflowNonBlockingJoinMultipleRunnable",
	"EPLDataflowNonBlockingJoinSingleRunnable",
	"EPLDataflowFastCompleteNonBlocking",
}

var dataflowLifecycleCancelJoinCases = []string{
	"nonblocking-join-cancel",
	"nonblocking-join-exception",
	"nonblocking-exception",
	"nonblocking-cancel",
	"nonblocking-join-multiple-runnable",
	"nonblocking-join-single-runnable",
	"fast-complete-nonblocking",
}

// dataflowCancelJoinBean mirrors the SupportBean event type.
type dataflowCancelJoinBean struct {
	TheString string `esper:"theString"`
}

// dataflowCancelJoinSource mirrors a DefaultSupportSourceOp gated on its
// latch: after release it optionally submits one bean and always submits the
// final marker; it may instead fail (mirroring the RuntimeException
// instruction). submissions counts its own submit/submitSignal invocations
// (the Java getCurrentCount analog).
type dataflowCancelJoinSource struct {
	gate       chan struct{}
	throw      bool
	submitBean bool
	mu         sync.Mutex
	submitted  int
}

func (s *dataflowCancelJoinSource) Run(ctx context.Context, emitter *esper.DataflowEmitter) error {
	select {
	case <-s.gate:
	case <-ctx.Done():
		return ctx.Err()
	}
	if s.throw {
		return errors.New("TestException")
	}
	if s.submitBean {
		s.mu.Lock()
		s.submitted++
		s.mu.Unlock()
		if err := emitter.SubmitPort(ctx, "outstream", dataflowCancelJoinBean{TheString: "E1"}); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.submitted++
	s.mu.Unlock()
	return emitter.SubmitSignal(ctx, esper.FinalMarker{})
}

// dataflowCancelJoinImmediateSource throws before any gating.
type dataflowCancelJoinImmediateSource struct{}

func (s dataflowCancelJoinImmediateSource) Run(_ context.Context, _ *esper.DataflowEmitter) error {
	return errors.New("TestException")
}

// dataflowCancelJoinCapture mirrors DefaultSupportCaptureOp: rows accumulate
// in the current batch; a forwarded signal flushes it into received.
type dataflowCancelJoinCapture struct {
	mu       sync.Mutex
	current  []any
	received [][]any
}

func (c *dataflowCancelJoinCapture) Process(_ context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// The typed source's raw bean passes through unmaterialized; the records
	// only pin row counts, so no event projection is needed here.
	c.current = append(c.current, input.Value)
	return nil, nil
}

func (c *dataflowCancelJoinCapture) OnSignal(_ context.Context, _ esper.DataflowSignal) ([]esper.DataflowEmission, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.received = append(c.received, append([]any(nil), c.current...))
	c.current = nil
	return nil, nil
}

// rows returns the total received rows plus any unflushed current rows.
func (c *dataflowCancelJoinCapture) rows() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	total := len(c.current)
	for _, batch := range c.received {
		total += len(batch)
	}
	return total
}

func runDataflowLifecycleCancelJoinScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-lifecycle-cancel-join scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowLifecycleCancelJoinCases {
		caseTrace, err := runDataflowLifecycleCancelJoinCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-lifecycle-cancel-join case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowLifecycleCancelJoinCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
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
	buildGraph := func(env *esper.Environment, source esper.DataflowSourceRuntime, capture *dataflowCancelJoinCapture) (esper.DataflowDefinition, error) {
		return esper.DefineDataflow(env, "MyDataFlowOne").
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
	}

	switch caseIndex {
	case 0: // nonblocking-join-cancel — join returns only via cancel
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[dataflowCancelJoinBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(dataflowLifecycleCancelJoinJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		capture := &dataflowCancelJoinCapture{}
		source := &dataflowCancelJoinSource{gate: make(chan struct{})}
		definition, buildErr := buildGraph(env, source, capture)
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
		if err := assertState(instance, esper.DataflowRunning); err != nil {
			return nil, err
		}
		record("state", "flow", "instance.state", "RUNNING")
		if err := instance.Cancel(ctx); err != nil {
			return nil, err
		}
		if err := assertState(instance, esper.DataflowCanceled); err != nil {
			return nil, err
		}
		countRecord("flow", "capture-empty", int64(capture.rows()))
	case 1: // nonblocking-join-exception — start mode swallows the source error
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[dataflowCancelJoinBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(dataflowLifecycleCancelJoinJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		capture := &dataflowCancelJoinCapture{}
		source := &dataflowCancelJoinSource{gate: make(chan struct{}), throw: true}
		definition, buildErr := buildGraph(env, source, capture)
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
		close(source.gate)
		// Go surfaces the swallowed Java failure from Join; the shared
		// observables are the COMPLETE state and the empty capture.
		joinErr := instance.Join(ctx)
		if joinErr == nil {
			return nil, fmt.Errorf("join after source exception returned nil")
		}
		if err := assertState(instance, esper.DataflowComplete); err != nil {
			return nil, err
		}
		record("state", "flow", "instance.state", "COMPLETE")
		countRecord("flow", "capture-empty", int64(capture.rows()))
	case 2: // nonblocking-exception — immediately-throwing source
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[dataflowCancelJoinBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(dataflowLifecycleCancelJoinJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		capture := &dataflowCancelJoinCapture{}
		definition, buildErr := buildGraph(env, dataflowCancelJoinImmediateSource{}, capture)
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
		if err := instance.Join(ctx); err == nil {
			return nil, fmt.Errorf("join after source exception returned nil")
		}
		if err := assertState(instance, esper.DataflowComplete); err != nil {
			return nil, err
		}
		record("state", "flow", "instance.state", "COMPLETE")
		countRecord("flow", "capture-empty", int64(capture.rows()))
	case 3: // nonblocking-cancel — RUNNING observable, synchronous cancel
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[dataflowCancelJoinBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(dataflowLifecycleCancelJoinJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		capture := &dataflowCancelJoinCapture{}
		source := &dataflowCancelJoinSource{gate: make(chan struct{})}
		definition, buildErr := buildGraph(env, source, capture)
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
		if err := assertState(instance, esper.DataflowRunning); err != nil {
			return nil, err
		}
		record("state", "flow", "instance.state", "RUNNING")
		if err := instance.Cancel(ctx); err != nil {
			return nil, err
		}
		if err := assertState(instance, esper.DataflowCanceled); err != nil {
			return nil, err
		}
		record("state", "flow", "instance.state", "CANCELLED")
		close(source.gate) // Java's trailing countDown; no submit can follow
		countRecord("flow", "capture-empty", int64(capture.rows()))
	case 4: // nonblocking-join-multiple-runnable — join waits for both sources
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[dataflowCancelJoinBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(dataflowLifecycleCancelJoinJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		capture := &dataflowCancelJoinCapture{}
		sourceOne := &dataflowCancelJoinSource{gate: make(chan struct{}), submitBean: true}
		sourceTwo := &dataflowCancelJoinSource{gate: make(chan struct{}), submitBean: true}
		sourceFactory := func(source esper.DataflowSourceRuntime) esper.DataflowSourceFactory {
			return func(esper.DataflowOperatorContext) (esper.DataflowSourceRuntime, error) {
				return source, nil
			}
		}
		definition, buildErr := esper.DefineDataflow(env, "MyDataFlowOne").
			CustomTypedSource("SourceOne", sourceFactory(sourceOne),
				[]esper.DataflowPort{esper.DataflowPortOf[dataflowCancelJoinBean]("outstream")}).
			CustomTypedSource("SourceTwo", sourceFactory(sourceTwo),
				[]esper.DataflowPort{esper.DataflowPortOf[dataflowCancelJoinBean]("outstream")}).
			Custom("DefaultSupportCaptureOp", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return capture, nil
			}).
			ConnectPorts("SourceOne", "outstream", "DefaultSupportCaptureOp", "in").
			ConnectPorts("SourceTwo", "outstream", "DefaultSupportCaptureOp", "in").
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
		if err := assertState(instance, esper.DataflowRunning); err != nil {
			return nil, err
		}
		record("state", "flow", "instance.state", "RUNNING")
		close(sourceOne.gate)
		if err := assertState(instance, esper.DataflowRunning); err != nil {
			return nil, err
		}
		record("state", "flow", "instance.state", "RUNNING")
		close(sourceTwo.gate)
		if err := instance.Join(ctx); err != nil {
			return nil, err
		}
		if err := assertState(instance, esper.DataflowComplete); err != nil {
			return nil, err
		}
		record("state", "flow", "instance.state", "COMPLETE")
		countRecord("flow", "capture-rows", int64(capture.rows()))
	case 5: // nonblocking-join-single-runnable — single gated source
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[dataflowCancelJoinBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(dataflowLifecycleCancelJoinJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		capture := &dataflowCancelJoinCapture{}
		source := &dataflowCancelJoinSource{gate: make(chan struct{}), submitBean: true}
		definition, buildErr := buildGraph(env, source, capture)
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
		if err := instance.Start(ctx); err != nil {
			return nil, err
		}
		if err := assertState(instance, esper.DataflowRunning); err != nil {
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
		// The source counted one bean plus one final-marker submission (the
		// Java getCurrentCount()==2 analog); cancel-after-COMPLETE would
		// error Go-side and is frozen out of the records.
		source.mu.Lock()
		submitted := source.submitted
		source.mu.Unlock()
		if submitted != 2 {
			return nil, fmt.Errorf("source submissions = %d, want 2", submitted)
		}
		countRecord("flow", "capture-rows", int64(capture.rows()))
	case 6: // fast-complete-nonblocking — finite beacon, own completion
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[dataflowCancelJoinBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(dataflowLifecycleCancelJoinJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
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
		if err := instance.Start(ctx); err != nil {
			return nil, err
		}
		// Bounded poll replaces Java's busy-wait-with-timeout.
		deadline := time.Now().Add(5 * time.Second)
		for instance.State() != esper.DataflowComplete {
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("instance did not complete within the bound")
			}
			time.Sleep(time.Millisecond)
		}
		record("state", "flow", "instance.state", "COMPLETE")
		// tryAssertionAfterExec in-process: join is a no-op; run/start after
		// complete error; cancel after complete errors Go-side (silent Java
		// no-op — frozen, never recorded).
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
	default:
		return nil, fmt.Errorf("unsupported dataflow-lifecycle-cancel-join case index %d", caseIndex)
	}
	return records, nil
}
