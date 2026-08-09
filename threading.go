package esper

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

type asyncPoolConfig struct {
	workers  int
	capacity int
}

func normalizeAsyncPoolConfig(workers, capacity int) asyncPoolConfig {
	if workers <= 0 {
		return asyncPoolConfig{}
	}
	if capacity <= 0 {
		capacity = workers * 64
		if capacity < 64 {
			capacity = 64
		}
	}
	return asyncPoolConfig{workers: workers, capacity: capacity}
}

// WithInboundWorkers enables SendAsync and its representation-specific
// variants. Each task still enters the Engine's serialized transaction
// boundary; workers decouple producer latency and provide bounded admission.
func WithInboundWorkers(workers, queueCapacity int) EngineOption {
	return func(config *engineConfig) {
		config.inboundPool = normalizeAsyncPoolConfig(workers, queueCapacity)
	}
}

// WithOutboundWorkers delivers statement subscriber/listener/sink batches on
// a bounded worker pool. Synchronous rule evaluation remains serialized.
func WithOutboundWorkers(workers, queueCapacity int) EngineOption {
	return func(config *engineConfig) {
		config.outboundPool = normalizeAsyncPoolConfig(workers, queueCapacity)
	}
}

// WithRouteWorkers enables RouteAsync.
func WithRouteWorkers(workers, queueCapacity int) EngineOption {
	return func(config *engineConfig) {
		config.routePool = normalizeAsyncPoolConfig(workers, queueCapacity)
	}
}

// WithTimerWorkers enables AdvanceTimeAsync.
func WithTimerWorkers(workers, queueCapacity int) EngineOption {
	return func(config *engineConfig) {
		config.timerPool = normalizeAsyncPoolConfig(workers, queueCapacity)
	}
}

// ThreadingTaskKind identifies one Engine worker-pool category.
type ThreadingTaskKind string

const (
	// ThreadingInbound handles asynchronous event admission.
	ThreadingInbound ThreadingTaskKind = "inbound"
	// ThreadingOutbound handles asynchronous statement observer delivery.
	ThreadingOutbound ThreadingTaskKind = "outbound"
	// ThreadingRoute handles asynchronous routed event cycles.
	ThreadingRoute ThreadingTaskKind = "route"
	// ThreadingTimer handles asynchronous virtual-clock advances.
	ThreadingTimer ThreadingTaskKind = "timer"
)

// ThreadingError reports a failed or panicked asynchronous task.
type ThreadingError struct {
	Kind  ThreadingTaskKind
	Cause error
}

func (e ThreadingError) Error() string {
	return fmt.Sprintf("esper: %s worker: %v", e.Kind, e.Cause)
}

func (e ThreadingError) Unwrap() error { return e.Cause }

// ThreadingErrorHandler observes asynchronous task failures. Handler panics
// are isolated from the worker pool.
type ThreadingErrorHandler func(context.Context, ThreadingError)

// AsyncTask is one admitted worker task. Wait may be called multiple times.
type AsyncTask struct {
	result *asyncTaskResult
}

func (t AsyncTask) Wait(ctx context.Context) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if t.result == nil {
		return NewError(ErrorState, "async task is empty")
	}
	select {
	case <-t.result.done:
		t.result.mu.RLock()
		err := t.result.err
		t.result.mu.RUnlock()
		return err
	case <-ctx.Done():
		return contextErr(ctx)
	}
}

type asyncTaskResult struct {
	mu   sync.RWMutex
	done chan struct{}
	err  error
}

type asyncTask struct {
	ctx      context.Context
	cancel   context.CancelFunc
	stopRoot func() bool
	run      func(context.Context) error
	result   *asyncTaskResult
	once     sync.Once
}

// ThreadPoolStats is a point-in-time snapshot for one bounded worker pool.
type ThreadPoolStats struct {
	Enabled       bool
	Workers       int
	QueueCapacity int
	Queued        int
	InFlight      int
	Pending       int
	Accepted      uint64
	Completed     uint64
	Dropped       uint64
}

// RuntimeThreadingStats contains all Engine worker-pool snapshots.
type RuntimeThreadingStats struct {
	Inbound  ThreadPoolStats
	Outbound ThreadPoolStats
	Route    ThreadPoolStats
	Timer    ThreadPoolStats
}

type asyncTaskPool struct {
	kind     ThreadingTaskKind
	workers  int
	queue    chan *asyncTask
	admit    chan struct{}
	root     context.Context
	cancel   context.CancelFunc
	submitMu sync.RWMutex
	mu       sync.Mutex
	closed   bool
	pending  int
	inflight int
	accepted uint64
	complete uint64
	dropped  uint64
	idle     chan struct{}
	workerWG sync.WaitGroup
	stopped  chan struct{}
	close    sync.Once
}

func newAsyncTaskPool(kind ThreadingTaskKind, config asyncPoolConfig) *asyncTaskPool {
	if config.workers <= 0 {
		return nil
	}
	root, cancel := context.WithCancel(context.Background())
	idle := make(chan struct{})
	close(idle)
	pool := &asyncTaskPool{
		kind:    kind,
		workers: config.workers,
		queue:   make(chan *asyncTask, config.capacity),
		admit:   make(chan struct{}, config.capacity+config.workers),
		root:    root,
		cancel:  cancel,
		idle:    idle,
		stopped: make(chan struct{}),
	}
	pool.workerWG.Add(config.workers)
	for index := 0; index < config.workers; index++ {
		go pool.worker()
	}
	go func() {
		pool.workerWG.Wait()
		close(pool.stopped)
	}()
	return pool
}

func (p *asyncTaskPool) submit(ctx context.Context, run func(context.Context) error) (AsyncTask, error) {
	if err := contextErr(ctx); err != nil {
		return AsyncTask{}, err
	}
	if p == nil {
		return AsyncTask{}, NewError(ErrorState, "worker pool is disabled")
	}
	if run == nil {
		return AsyncTask{}, NewError(ErrorInvalidRule, "async task callback is nil")
	}
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return AsyncTask{}, NewError(ErrorState, fmt.Sprintf("%s worker pool is closed", p.kind))
	}
	taskContext, cancel := context.WithCancel(ctx)
	result := &asyncTaskResult{done: make(chan struct{})}
	task := &asyncTask{ctx: taskContext, cancel: cancel, run: run, result: result}
	task.stopRoot = context.AfterFunc(p.root, cancel)
	select {
	case p.admit <- struct{}{}:
	case <-ctx.Done():
		task.stopRoot()
		cancel()
		return AsyncTask{}, contextErr(ctx)
	case <-p.root.Done():
		task.stopRoot()
		cancel()
		return AsyncTask{}, NewError(ErrorCanceled, fmt.Sprintf("%s worker pool is shutting down", p.kind))
	}
	p.submitMu.RLock()
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		p.submitMu.RUnlock()
		<-p.admit
		task.stopRoot()
		cancel()
		return AsyncTask{}, NewError(ErrorState, fmt.Sprintf("%s worker pool is closed", p.kind))
	}
	if p.pending == 0 {
		p.idle = make(chan struct{})
	}
	p.pending++
	p.accepted++
	p.mu.Unlock()
	p.queue <- task
	p.submitMu.RUnlock()
	return AsyncTask{result: result}, nil
}

func (p *asyncTaskPool) worker() {
	defer p.workerWG.Done()
	for {
		select {
		case <-p.root.Done():
			p.drainCanceled()
			return
		case task := <-p.queue:
			if task == nil {
				continue
			}
			p.mu.Lock()
			p.inflight++
			p.mu.Unlock()
			err := task.run(task.ctx)
			p.mu.Lock()
			p.inflight--
			p.mu.Unlock()
			p.finish(task, err, false)
		}
	}
}

func (p *asyncTaskPool) drainCanceled() {
	for {
		select {
		case task := <-p.queue:
			if task != nil {
				p.finish(task, NewError(ErrorCanceled, fmt.Sprintf("%s worker pool shut down before execution", p.kind)), true)
			}
		default:
			return
		}
	}
}

func (p *asyncTaskPool) finish(task *asyncTask, err error, dropped bool) {
	if p == nil || task == nil {
		return
	}
	task.once.Do(func() {
		if task.stopRoot != nil {
			task.stopRoot()
		}
		task.cancel()
		task.result.mu.Lock()
		task.result.err = err
		task.result.mu.Unlock()
		close(task.result.done)
		p.mu.Lock()
		p.pending--
		p.complete++
		if dropped {
			p.dropped++
		}
		if p.pending == 0 {
			close(p.idle)
		}
		p.mu.Unlock()
		<-p.admit
	})
}

func (p *asyncTaskPool) waitIdle(ctx context.Context) error {
	if p == nil {
		return nil
	}
	for {
		p.mu.Lock()
		if p.pending == 0 {
			p.mu.Unlock()
			return nil
		}
		idle := p.idle
		p.mu.Unlock()
		select {
		case <-idle:
		case <-ctx.Done():
			return contextErr(ctx)
		}
	}
}

func (p *asyncTaskPool) shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.close.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()
		p.submitMu.Lock()
		p.cancel()
		p.submitMu.Unlock()
	})
	select {
	case <-p.stopped:
		return nil
	case <-ctx.Done():
		return contextErr(ctx)
	}
}

func (p *asyncTaskPool) stats() ThreadPoolStats {
	if p == nil {
		return ThreadPoolStats{}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return ThreadPoolStats{
		Enabled:       true,
		Workers:       p.workers,
		QueueCapacity: cap(p.queue),
		Queued:        len(p.queue),
		InFlight:      p.inflight,
		Pending:       p.pending,
		Accepted:      p.accepted,
		Completed:     p.complete,
		Dropped:       p.dropped,
	}
}

// SetThreadingErrorHandler replaces the Engine-wide asynchronous failure
// observer. Passing nil disables failure observation.
func (e *Engine) SetThreadingErrorHandler(handler ThreadingErrorHandler) {
	if e == nil {
		return
	}
	e.threadingErrorMu.Lock()
	e.threadingErrorHandler = handler
	e.threadingErrorMu.Unlock()
}

func (e *Engine) reportThreadingError(ctx context.Context, kind ThreadingTaskKind, err error) {
	if e == nil || err == nil {
		return
	}
	e.threadingErrorMu.RLock()
	handler := e.threadingErrorHandler
	e.threadingErrorMu.RUnlock()
	if handler == nil {
		return
	}
	func() {
		defer func() { _ = recover() }()
		handler(ctx, ThreadingError{Kind: kind, Cause: err})
	}()
}

func (e *Engine) runThreadingTask(ctx context.Context, kind ThreadingTaskKind, run func(context.Context) error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %v\n%s", recovered, debug.Stack())
		}
		if err != nil {
			e.reportThreadingError(ctx, kind, err)
		}
	}()
	return run(ctx)
}

// SendAsync admits a named event to the inbound worker pool.
func (e *Engine) SendAsync(ctx context.Context, eventType string, underlying any) (AsyncTask, error) {
	if e == nil {
		return AsyncTask{}, NewError(ErrorDependency, "nil engine")
	}
	return e.inboundPool.submit(ctx, func(taskContext context.Context) error {
		return e.runThreadingTask(taskContext, ThreadingInbound, func(runContext context.Context) error {
			return e.Send(runContext, eventType, underlying)
		})
	})
}

// SendEventAsync admits a registered struct event to the inbound worker pool.
func (e *Engine) SendEventAsync(ctx context.Context, underlying any) (AsyncTask, error) {
	if e == nil {
		return AsyncTask{}, NewError(ErrorDependency, "nil engine")
	}
	return e.inboundPool.submit(ctx, func(taskContext context.Context) error {
		return e.runThreadingTask(taskContext, ThreadingInbound, func(runContext context.Context) error {
			return e.SendEvent(runContext, underlying)
		})
	})
}

// SendJSONAsync copies and admits one JSON event to the inbound worker pool.
func (e *Engine) SendJSONAsync(ctx context.Context, eventType string, data []byte) (AsyncTask, error) {
	if e == nil {
		return AsyncTask{}, NewError(ErrorDependency, "nil engine")
	}
	copied := append([]byte(nil), data...)
	return e.inboundPool.submit(ctx, func(taskContext context.Context) error {
		return e.runThreadingTask(taskContext, ThreadingInbound, func(runContext context.Context) error {
			return e.SendJSON(runContext, eventType, copied)
		})
	})
}

// SendXMLAsync copies and admits one XML event to the inbound worker pool.
func (e *Engine) SendXMLAsync(ctx context.Context, eventType string, data []byte) (AsyncTask, error) {
	if e == nil {
		return AsyncTask{}, NewError(ErrorDependency, "nil engine")
	}
	copied := append([]byte(nil), data...)
	return e.inboundPool.submit(ctx, func(taskContext context.Context) error {
		return e.runThreadingTask(taskContext, ThreadingInbound, func(runContext context.Context) error {
			return e.SendXML(runContext, eventType, copied)
		})
	})
}

// SendObjectArrayAsync copies and admits one ObjectArray event to the inbound
// worker pool.
func (e *Engine) SendObjectArrayAsync(ctx context.Context, eventType string, values []any) (AsyncTask, error) {
	if e == nil {
		return AsyncTask{}, NewError(ErrorDependency, "nil engine")
	}
	copied := append([]any(nil), values...)
	return e.inboundPool.submit(ctx, func(taskContext context.Context) error {
		return e.runThreadingTask(taskContext, ThreadingInbound, func(runContext context.Context) error {
			return e.SendObjectArray(runContext, eventType, copied)
		})
	})
}

// RouteAsync runs one routed event cycle on the route worker pool.
func (e *Engine) RouteAsync(ctx context.Context, eventType string, underlying any) (AsyncTask, error) {
	if e == nil {
		return AsyncTask{}, NewError(ErrorDependency, "nil engine")
	}
	return e.routePool.submit(ctx, func(taskContext context.Context) error {
		return e.runThreadingTask(taskContext, ThreadingRoute, func(runContext context.Context) error {
			return e.Route(runContext, eventType, underlying)
		})
	})
}

// AdvanceTimeAsync advances virtual time on the timer worker pool.
func (e *Engine) AdvanceTimeAsync(ctx context.Context, at time.Time) (AsyncTask, error) {
	if e == nil {
		return AsyncTask{}, NewError(ErrorDependency, "nil engine")
	}
	return e.timerPool.submit(ctx, func(taskContext context.Context) error {
		return e.runThreadingTask(taskContext, ThreadingTimer, func(runContext context.Context) error {
			return e.AdvanceTime(runContext, at)
		})
	})
}

// WaitAsync waits until all currently admitted inbound, route, timer and
// outbound tasks (including outbound work produced by the first three pools)
// have completed.
func (e *Engine) WaitAsync(ctx context.Context) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if e == nil {
		return NewError(ErrorDependency, "nil engine")
	}
	for {
		for _, pool := range []*asyncTaskPool{e.inboundPool, e.routePool, e.timerPool, e.outboundPool} {
			if err := pool.waitIdle(ctx); err != nil {
				return err
			}
		}
		stats := e.ThreadingStats()
		if stats.Inbound.Pending+stats.Route.Pending+stats.Timer.Pending+stats.Outbound.Pending == 0 {
			return nil
		}
	}
}

// ThreadingStats returns point-in-time snapshots for all worker pools.
func (e *Engine) ThreadingStats() RuntimeThreadingStats {
	if e == nil {
		return RuntimeThreadingStats{}
	}
	return RuntimeThreadingStats{
		Inbound:  e.inboundPool.stats(),
		Outbound: e.outboundPool.stats(),
		Route:    e.routePool.stats(),
		Timer:    e.timerPool.stats(),
	}
}

func (e *Engine) shutdownThreading(ctx context.Context) error {
	if e == nil {
		return nil
	}
	for _, pool := range []*asyncTaskPool{e.inboundPool, e.routePool, e.timerPool, e.outboundPool} {
		if err := pool.shutdown(ctx); err != nil {
			return err
		}
	}
	return nil
}
