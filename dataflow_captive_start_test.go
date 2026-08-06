package esper

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type dataflowCaptiveCapture struct {
	mu      sync.Mutex
	current []runtimeTestTrade
	batches [][]runtimeTestTrade
	opened  atomic.Int32
	closed  atomic.Int32
}

func (c *dataflowCaptiveCapture) Open(context.Context) error {
	c.opened.Add(1)
	return nil
}

func (c *dataflowCaptiveCapture) Close(context.Context) error {
	c.closed.Add(1)
	return nil
}

func (c *dataflowCaptiveCapture) Process(_ context.Context, input DataflowInput) ([]DataflowEmission, error) {
	event, ok := input.Value.(Event)
	if !ok {
		return nil, NewError(ErrorTypeMismatch, "captive capture requires Event")
	}
	trade, ok := event.Underlying().(runtimeTestTrade)
	if !ok {
		return nil, NewError(ErrorTypeMismatch, "captive capture requires runtimeTestTrade")
	}
	c.mu.Lock()
	c.current = append(c.current, trade)
	c.mu.Unlock()
	return nil, nil
}

func (c *dataflowCaptiveCapture) OnSignal(_ context.Context, signal DataflowSignal) ([]DataflowEmission, error) {
	if !isDataflowFinalMarker(signal) {
		return nil, nil
	}
	c.mu.Lock()
	c.batches = append(c.batches, append([]runtimeTestTrade(nil), c.current...))
	c.current = nil
	c.mu.Unlock()
	return nil, nil
}

func (c *dataflowCaptiveCapture) snapshot() ([]runtimeTestTrade, [][]runtimeTestTrade) {
	c.mu.Lock()
	defer c.mu.Unlock()
	current := append([]runtimeTestTrade(nil), c.current...)
	batches := make([][]runtimeTestTrade, len(c.batches))
	for index := range c.batches {
		batches[index] = append([]runtimeTestTrade(nil), c.batches[index]...)
	}
	return current, batches
}

type dataflowCaptiveSource struct {
	opened atomic.Int32
	closed atomic.Int32
}

type dataflowCaptiveBlockingSource struct {
	started chan struct{}
	once    sync.Once
}

func (s *dataflowCaptiveBlockingSource) Run(ctx context.Context, _ *DataflowEmitter) error {
	s.once.Do(func() { close(s.started) })
	<-ctx.Done()
	return ctx.Err()
}

func (s *dataflowCaptiveSource) Open(context.Context) error {
	s.opened.Add(1)
	return nil
}

func (s *dataflowCaptiveSource) Close(context.Context) error {
	s.closed.Add(1)
	return nil
}

func (*dataflowCaptiveSource) Run(ctx context.Context, emitter *DataflowEmitter) error {
	if err := emitter.Submit(ctx, "one"); err != nil {
		return err
	}
	return emitter.Submit(ctx, "two")
}

func TestDataflowStartCaptiveEmitterMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	capture := &dataflowCaptiveCapture{}
	definition, err := DefineDataflow(env, "captive-emitter-flow").
		Emitter("src1").
		Custom("capture", func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			return capture, nil
		}).
		Connect("src1", "capture").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	captive, err := instance.StartCaptive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if instance.State() != DataflowRunning || capture.opened.Load() != 1 {
		t.Fatalf("captive state/open = %v/%d", instance.State(), capture.opened.Load())
	}
	if len(captive.Runnables()) != 0 || len(captive.Emitters()) != 1 {
		t.Fatalf("captive handles = emitters %#v runnables %#v", captive.Emitters(), captive.Runnables())
	}
	emitter, ok := captive.Emitter("src1")
	if !ok {
		t.Fatal("named captive emitter is missing")
	}
	for _, trade := range []runtimeTestTrade{{Symbol: "E1", Price: 10}, {Symbol: "E2", Price: 20}} {
		if err := emitter.Submit(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	current, batches := capture.snapshot()
	if len(current) != 2 || len(batches) != 0 {
		t.Fatalf("capture before final marker = current %#v batches %#v", current, batches)
	}
	if err := emitter.SubmitSignal(context.Background(), FinalMarker{}); err != nil {
		t.Fatal(err)
	}
	current, batches = capture.snapshot()
	if len(current) != 0 || len(batches) != 1 || len(batches[0]) != 2 {
		t.Fatalf("capture after final marker = current %#v batches %#v", current, batches)
	}
	if err := emitter.Submit(context.Background(), runtimeTestTrade{Symbol: "E3", Price: 30}); err != nil {
		t.Fatal(err)
	}
	current, _ = capture.snapshot()
	if len(current) != 1 || current[0].Symbol != "E3" || instance.State() != DataflowRunning {
		t.Fatalf("capture after restart = %#v, state %v", current, instance.State())
	}
	if _, err := instance.StartCaptive(context.Background()); err == nil || !errors.Is(err, ErrorState) {
		t.Fatalf("second captive start error = %v", err)
	}
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if capture.closed.Load() != 1 || instance.State() != DataflowCanceled {
		t.Fatalf("captive close/state = %d/%v", capture.closed.Load(), instance.State())
	}
	if err := emitter.Submit(context.Background(), runtimeTestTrade{}); err == nil || !errors.Is(err, ErrorState) {
		t.Fatalf("canceled captive emitter error = %v", err)
	}
}

func TestDataflowStartCaptiveReturnsCallerRunSources(t *testing.T) {
	env := NewEnvironment()
	source := &dataflowCaptiveSource{}
	definition, err := DefineDataflow(env, "captive-source-flow").
		CustomSource("source", func(DataflowOperatorContext) (DataflowSourceRuntime, error) {
			return source, nil
		}).
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
	captive, err := instance.StartCaptive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(captive.Emitters()) != 0 || len(captive.Runnables()) != 1 || captive.Runnables()[0].Name() != "source" {
		t.Fatalf("captive source handles = emitters %#v runnables %#v", captive.Emitters(), captive.Runnables())
	}
	if outputs := instance.Outputs(); len(outputs) != 0 {
		t.Fatalf("captive source ran automatically: %#v", outputs)
	}
	runnable := captive.Runnables()[0]
	var completions []DataflowCaptiveCompletion
	if err := runnable.AddCompletionListener(func(completion DataflowCaptiveCompletion) {
		completions = append(completions, completion)
	}); err != nil {
		t.Fatal(err)
	}
	if err := runnable.AddCompletionListener(nil); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("nil captive completion listener error = %v", err)
	}
	if err := runnable.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if outputs := instance.Outputs(); len(outputs) != 2 || outputs[0] != "one" || outputs[1] != "two" {
		t.Fatalf("captive source outputs = %#v", outputs)
	}
	if instance.State() != DataflowRunning {
		t.Fatalf("completed captive source state = %v", instance.State())
	}
	select {
	case <-runnable.Done():
	default:
		t.Fatal("captive source done channel is still open")
	}
	if err := runnable.Wait(context.Background()); err != nil {
		t.Fatalf("completed captive source wait error = %v", err)
	}
	if len(completions) != 1 || completions[0].Name != "source" || completions[0].Err != nil || completions[0].Canceled {
		t.Fatalf("captive source completion = %#v", completions)
	}
	if err := runnable.AddCompletionListener(func(completion DataflowCaptiveCompletion) {
		completions = append(completions, completion)
	}); err != nil {
		t.Fatal(err)
	}
	if len(completions) != 2 || completions[1].Name != "source" {
		t.Fatalf("late captive completion = %#v", completions)
	}
	if err := runnable.Run(context.Background()); err == nil || !errors.Is(err, ErrorState) {
		t.Fatalf("second captive source run error = %v", err)
	}
	if source.opened.Load() != 1 {
		t.Fatalf("captive source open count = %d", source.opened.Load())
	}
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if source.closed.Load() != 1 {
		t.Fatalf("captive source close count = %d", source.closed.Load())
	}
}

func TestDataflowCaptiveRunnableAsyncShutdownAndWait(t *testing.T) {
	env := NewEnvironment()
	source := &dataflowCaptiveBlockingSource{started: make(chan struct{})}
	definition, err := DefineDataflow(env, "captive-async-source-flow").
		CustomSource("source", func(DataflowOperatorContext) (DataflowSourceRuntime, error) {
			return source, nil
		}).
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
	captive, err := instance.StartCaptive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	runnable := captive.Runnables()[0]
	completionCh := make(chan DataflowCaptiveCompletion, 1)
	if err := runnable.AddCompletionListener(func(completion DataflowCaptiveCompletion) {
		completionCh <- completion
	}); err != nil {
		t.Fatal(err)
	}
	if err := runnable.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-source.started:
	case <-time.After(time.Second):
		t.Fatal("asynchronous captive source did not start")
	}
	waitCtx, cancelWait := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancelWait()
	if err := runnable.Wait(waitCtx); err == nil || !errors.Is(err, ErrorTimeout) {
		t.Fatalf("running captive source wait error = %v", err)
	}
	runnable.Shutdown()
	runnable.Shutdown()
	if !runnable.IsShutdown() {
		t.Fatal("captive source did not report shutdown")
	}
	joinCtx, cancelJoin := context.WithTimeout(context.Background(), time.Second)
	defer cancelJoin()
	if err := runnable.Wait(joinCtx); err == nil || !errors.Is(err, ErrorCanceled) {
		t.Fatalf("shutdown captive source wait error = %v", err)
	}
	select {
	case completion := <-completionCh:
		if completion.Name != "source" || !completion.Canceled || completion.Err == nil || !errors.Is(completion.Err, ErrorCanceled) {
			t.Fatalf("shutdown captive completion = %#v", completion)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown captive completion listener was not invoked")
	}
	if instance.State() != DataflowRunning {
		t.Fatalf("source-only shutdown changed instance state to %v", instance.State())
	}
	if err := runnable.Start(context.Background()); err == nil || !errors.Is(err, ErrorState) {
		t.Fatalf("second asynchronous captive start error = %v", err)
	}
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDataflowCaptiveRunnableStopsWhenInstanceIsCanceled(t *testing.T) {
	env := NewEnvironment()
	source := &dataflowCaptiveBlockingSource{started: make(chan struct{})}
	definition, err := DefineDataflow(env, "captive-instance-cancel-flow").
		CustomSource("source", func(DataflowOperatorContext) (DataflowSourceRuntime, error) {
			return source, nil
		}).
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
	captive, err := instance.StartCaptive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	runnable := captive.Runnables()[0]
	if err := runnable.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-source.started:
	case <-time.After(time.Second):
		t.Fatal("instance-cancel captive source did not start")
	}
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitCtx, cancelWait := context.WithTimeout(context.Background(), time.Second)
	defer cancelWait()
	if err := runnable.Wait(waitCtx); err == nil || !errors.Is(err, ErrorCanceled) {
		t.Fatalf("instance-canceled captive wait error = %v", err)
	}
	if instance.State() != DataflowCanceled {
		t.Fatalf("instance-canceled captive state = %v", instance.State())
	}
	if runnable.IsShutdown() {
		t.Fatal("instance cancellation was reported as source-only shutdown")
	}
}

func TestDataflowStartCaptiveReturnsBeaconRunnable(t *testing.T) {
	env := NewEnvironment()
	definition, err := DefineDataflow(env, "captive-beacon-flow").
		BeaconSourceWithOptions("beacon", DataflowBeaconOptions{Iterations: 2}, "A", "B").
		Emitter("sink").
		Connect("beacon", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	captive, err := instance.StartCaptive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	runnables := captive.Runnables()
	if len(runnables) != 1 || runnables[0].Name() != "beacon" {
		t.Fatalf("captive beacon runnables = %#v", runnables)
	}
	if err := runnables[0].Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if outputs := instance.Outputs(); len(outputs) != 2 || outputs[0] != "A" || outputs[1] != "B" {
		t.Fatalf("captive beacon outputs = %#v", outputs)
	}
	if signals := instance.Signals(); len(signals) != 1 || !isDataflowFinalMarker(signals[0]) {
		t.Fatalf("captive beacon signals = %#v", signals)
	}
	if instance.State() != DataflowRunning {
		t.Fatalf("completed captive beacon state = %v", instance.State())
	}
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
}
