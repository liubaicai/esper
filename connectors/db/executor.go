package db

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	// ErrExecutorClosed indicates that a work executor no longer accepts tasks.
	ErrExecutorClosed = errors.New("db: work executor is closed")
	// ErrUnknownExecutor indicates that a configured work-queue name is absent.
	ErrUnknownExecutor = errors.New("db: work executor is not defined")
	// ErrNilWork indicates that Submit was called without a work function.
	ErrNilWork = errors.New("db: work function is nil")
	// ErrNilTask indicates that a method was called on a nil Task.
	ErrNilTask = errors.New("db: task is nil")
)

// WorkExecutor schedules one database action. The function receives the
// submission context so queued work can observe cancellation and deadlines.
// A WorkExecutor does not own the SQL executor used by a sink.
type WorkExecutor interface {
	Submit(context.Context, func(context.Context) error) (*Task, error)
	Shutdown(context.Context) error
}

// Task represents one submitted database action. Submission failures are
// returned by Submit; execution failures are returned by Wait.
type Task struct {
	done chan struct{}
	once sync.Once

	mu  sync.RWMutex
	err error
}

func newTask() *Task { return &Task{done: make(chan struct{})} }

func (t *Task) complete(err error) {
	if t == nil {
		return
	}
	t.once.Do(func() {
		t.mu.Lock()
		t.err = err
		t.mu.Unlock()
		close(t.done)
	})
}

// Done returns a channel closed when the action has completed.
func (t *Task) Done() <-chan struct{} {
	if t == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return t.done
}

// Wait waits for completion or for the caller's wait context to expire. A
// canceled wait does not cancel the submitted action; cancel the submission
// context passed to WriteAsync when cancellation of queued/running work is
// desired.
func (t *Task) Wait(ctx context.Context) error {
	if t == nil {
		return ErrNilTask
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-t.done:
		t.mu.RLock()
		err := t.err
		t.mu.RUnlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Err returns the execution error after completion, or nil while the task is
// still running. It never waits.
func (t *Task) Err() error {
	if t == nil {
		return ErrNilTask
	}
	select {
	case <-t.done:
		t.mu.RLock()
		err := t.err
		t.mu.RUnlock()
		return err
	default:
		return nil
	}
}

// SameThreadExecutor runs work inline, matching EsperIO DB's default
// ExecutorSameThread behavior.
type SameThreadExecutor struct{}

// NewSameThreadExecutor returns an inline work executor.
func NewSameThreadExecutor() *SameThreadExecutor { return &SameThreadExecutor{} }

var defaultSameThreadExecutor = NewSameThreadExecutor()

func (e SameThreadExecutor) Submit(ctx context.Context, work func(context.Context) error) (*Task, error) {
	if work == nil {
		return nil, ErrNilWork
	}
	if ctx == nil {
		ctx = context.Background()
	}
	task := newTask()
	task.complete(runWork(ctx, work))
	return task, nil
}

func (e SameThreadExecutor) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

type queuedWork struct {
	ctx  context.Context
	work func(context.Context) error
	task *Task
}

// AsyncExecutor is a fixed-size, FIFO work queue. Shutdown stops accepting
// new work and drains already accepted work before returning.
type AsyncExecutor struct {
	mu      sync.Mutex
	cond    *sync.Cond
	queue   []queuedWork
	closed  bool
	workers sync.WaitGroup
	done    chan struct{}
}

// NewAsyncExecutor creates a fixed worker-count queue. A non-positive worker
// count is invalid; ExecutorServices intentionally skips such Java-style
// configuration entries before calling this constructor.
func NewAsyncExecutor(workerCount int) (*AsyncExecutor, error) {
	if workerCount <= 0 {
		return nil, fmt.Errorf("db: worker count must be positive, got %d", workerCount)
	}
	e := &AsyncExecutor{done: make(chan struct{})}
	e.cond = sync.NewCond(&e.mu)
	e.workers.Add(workerCount)
	for index := 0; index < workerCount; index++ {
		go e.worker()
	}
	go func() {
		e.workers.Wait()
		close(e.done)
	}()
	return e, nil
}

func (e *AsyncExecutor) Submit(ctx context.Context, work func(context.Context) error) (*Task, error) {
	if e == nil {
		return nil, ErrExecutorClosed
	}
	if work == nil {
		return nil, ErrNilWork
	}
	if ctx == nil {
		ctx = context.Background()
	}
	task := newTask()
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, ErrExecutorClosed
	}
	if err := ctx.Err(); err != nil {
		e.mu.Unlock()
		task.complete(err)
		return task, nil
	}
	e.queue = append(e.queue, queuedWork{ctx: ctx, work: work, task: task})
	e.cond.Signal()
	e.mu.Unlock()
	return task, nil
}

func (e *AsyncExecutor) worker() {
	defer e.workers.Done()
	for {
		e.mu.Lock()
		for len(e.queue) == 0 && !e.closed {
			e.cond.Wait()
		}
		if len(e.queue) == 0 && e.closed {
			e.mu.Unlock()
			return
		}
		item := e.queue[0]
		e.queue[0] = queuedWork{}
		e.queue = e.queue[1:]
		e.mu.Unlock()
		item.task.complete(runWork(item.ctx, item.work))
	}
}

// QueueLen reports accepted but not yet started work.
func (e *AsyncExecutor) QueueLen() int {
	if e == nil {
		return 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.queue)
}

// Shutdown gracefully closes the queue and waits for workers to drain. If
// ctx expires, workers continue draining and the call returns ctx.Err(). A
// later Shutdown call can be used to wait again.
func (e *AsyncExecutor) Shutdown(ctx context.Context) error {
	if e == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	e.mu.Lock()
	e.closed = true
	e.cond.Broadcast()
	e.mu.Unlock()
	select {
	case <-e.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close is an alias for Shutdown with no deadline.
func (e *AsyncExecutor) Close() error { return e.Shutdown(context.Background()) }

func runWork(ctx context.Context, work func(context.Context) error) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("db: work panicked: %v", recovered)
		}
	}()
	return work(ctx)
}

// UnknownExecutorError identifies a missing named executor while preserving a
// stable errors.Is target for callers.
type UnknownExecutorError struct {
	Name string
}

func (e *UnknownExecutorError) Error() string {
	if e == nil {
		return ErrUnknownExecutor.Error()
	}
	return fmt.Sprintf("db: executor by name %q has not been defined", e.Name)
}

func (e *UnknownExecutorError) Unwrap() error { return ErrUnknownExecutor }

// ExecutorServices owns named asynchronous executors and provides the
// same-thread fallback used when no name is configured.
type ExecutorServices struct {
	mu         sync.RWMutex
	services   map[string]*AsyncExecutor
	sameThread *SameThreadExecutor
}

// NewExecutorServices creates named fixed-size queues. Entries with a
// non-positive worker count are ignored, matching EsperIO DB's Java config
// behavior. The caller remains responsible for calling Shutdown.
func NewExecutorServices(workerCounts map[string]int) (*ExecutorServices, error) {
	services := &ExecutorServices{
		services:   make(map[string]*AsyncExecutor),
		sameThread: NewSameThreadExecutor(),
	}
	for name, workerCount := range workerCounts {
		if strings.TrimSpace(name) == "" {
			for _, queue := range services.services {
				_ = queue.Shutdown(context.Background())
			}
			return nil, fmt.Errorf("db: executor name is required")
		}
		if workerCount <= 0 {
			continue
		}
		queue, err := NewAsyncExecutor(workerCount)
		if err != nil {
			return nil, fmt.Errorf("db: create executor %q: %w", name, err)
		}
		services.services[name] = queue
	}
	return services, nil
}

// Executor resolves name to a configured work queue. An empty name selects
// the same-thread fallback; a non-empty unknown name is an error.
func (s *ExecutorServices) Executor(name string) (WorkExecutor, error) {
	if s == nil {
		if name == "" {
			return NewSameThreadExecutor(), nil
		}
		return nil, &UnknownExecutorError{Name: name}
	}
	if name == "" {
		return s.sameThread, nil
	}
	s.mu.RLock()
	queue := s.services[name]
	s.mu.RUnlock()
	if queue == nil {
		return nil, &UnknownExecutorError{Name: name}
	}
	return queue, nil
}

func resolveWorkExecutor(explicit WorkExecutor, name string, services *ExecutorServices) (WorkExecutor, error) {
	if explicit != nil {
		return explicit, nil
	}
	if strings.TrimSpace(name) != "" {
		if services == nil {
			return nil, fmt.Errorf("db: ExecutorServices is required for ExecutorName %q", name)
		}
		return services.Executor(name)
	}
	if services != nil {
		return services.Executor("")
	}
	return defaultSameThreadExecutor, nil
}

// GetExecutor is an explicit alias for callers migrating from the Java
// ExecutorServices naming.
func (s *ExecutorServices) GetExecutor(name string) (WorkExecutor, error) {
	return s.Executor(name)
}

// QueueLen reports the queue depth for a named executor. The same-thread
// fallback always has a zero-length queue.
func (s *ExecutorServices) QueueLen(name string) (int, error) {
	work, err := s.Executor(name)
	if err != nil {
		return 0, err
	}
	queue, ok := work.(*AsyncExecutor)
	if !ok {
		return 0, nil
	}
	return queue.QueueLen(), nil
}

// Shutdown closes all named executors and waits for accepted work to drain.
func (s *ExecutorServices) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.mu.Lock()
	names := make([]string, 0, len(s.services))
	queues := make(map[string]*AsyncExecutor, len(s.services))
	for name, queue := range s.services {
		names = append(names, name)
		queues[name] = queue
	}
	s.mu.Unlock()
	sort.Strings(names)
	var firstErr error
	for _, name := range names {
		if err := queues[name].Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Destroy is a Java terminology alias for graceful Shutdown.
func (s *ExecutorServices) Destroy(ctx context.Context) error { return s.Shutdown(ctx) }
