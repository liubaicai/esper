package db

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/liubaicai/esper/connectors"
)

func TestSameThreadExecutorRunsInlineAndPublishesTaskResult(t *testing.T) {
	executor := NewSameThreadExecutor()
	run := false
	task, err := executor.Submit(context.Background(), func(context.Context) error {
		run = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !run {
		t.Fatal("same-thread executor did not run work inline")
	}
	if err := task.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if task.Err() != nil {
		t.Fatalf("completed task error = %v", task.Err())
	}
}

func TestExecutorServicesResolvesNamedQueueAndUnknownNames(t *testing.T) {
	services, err := NewExecutorServices(map[string]int{"db": 2, "disabled": 0})
	if err != nil {
		t.Fatal(err)
	}
	defer services.Shutdown(context.Background())

	if _, err := services.Executor("missing"); !errors.Is(err, ErrUnknownExecutor) {
		t.Fatalf("unknown executor error = %v", err)
	}
	if _, err := services.Executor("disabled"); !errors.Is(err, ErrUnknownExecutor) {
		t.Fatalf("disabled executor error = %v", err)
	}
	if _, err := services.Executor(""); err != nil {
		t.Fatalf("same-thread fallback = %v", err)
	}
	if depth, err := services.QueueLen("db"); err != nil || depth != 0 {
		t.Fatalf("initial queue depth = %d, %v", depth, err)
	}
}

func TestAsyncExecutorRunsFixedWorkersAndDrainsOnShutdown(t *testing.T) {
	executor, err := NewAsyncExecutor(2)
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var completed atomic.Int32
	work := func(context.Context) error {
		started <- struct{}{}
		<-release
		completed.Add(1)
		return nil
	}
	taskOne, err := executor.Submit(context.Background(), work)
	if err != nil {
		t.Fatal(err)
	}
	taskTwo, err := executor.Submit(context.Background(), work)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("fixed workers did not start concurrently")
		}
	}
	close(release)
	if err := taskOne.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := taskTwo.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if completed.Load() != 2 {
		t.Fatalf("completed work = %d", completed.Load())
	}
	if err := executor.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := executor.Submit(context.Background(), func(context.Context) error { return nil }); !errors.Is(err, ErrExecutorClosed) {
		t.Fatalf("submit after shutdown = %v", err)
	}
}

func TestAsyncExecutorShutdownDrainsQueuedWork(t *testing.T) {
	executor, err := NewAsyncExecutor(1)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	first, err := executor.Submit(context.Background(), func(context.Context) error {
		close(started)
		<-release
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	secondRan := make(chan struct{})
	second, err := executor.Submit(context.Background(), func(context.Context) error {
		close(secondRan)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	deadline := time.Now().Add(time.Second)
	for executor.QueueLen() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if executor.QueueLen() == 0 {
		t.Fatal("second task was not queued")
	}
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- executor.Shutdown(context.Background()) }()
	select {
	case <-shutdownDone:
		t.Fatal("shutdown returned before queued work could drain")
	case <-time.After(10 * time.Millisecond):
	}
	close(release)
	if err := first.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := second.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-secondRan:
	case <-time.After(time.Second):
		t.Fatal("queued task was not drained")
	}
	if err := <-shutdownDone; err != nil {
		t.Fatal(err)
	}
}

func TestAsyncExecutorQueuedContextCancellationAndPanicAreObservable(t *testing.T) {
	executor, err := NewAsyncExecutor(1)
	if err != nil {
		t.Fatal(err)
	}
	defer executor.Shutdown(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	blocker, err := executor.Submit(context.Background(), func(context.Context) error {
		close(started)
		<-release
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	runCanceled := atomic.Bool{}
	canceledTask, err := executor.Submit(ctx, func(context.Context) error {
		runCanceled.Store(true)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	panicTask, err := executor.Submit(context.Background(), func(context.Context) error {
		panic("boom")
	})
	if err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := blocker.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := canceledTask.Wait(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled queued task error = %v", err)
	}
	if runCanceled.Load() {
		t.Fatal("canceled queued work ran")
	}
	if err := panicTask.Wait(context.Background()); err == nil || !strings.Contains(err.Error(), "work panicked") {
		t.Fatalf("panic task error = %v", err)
	}
}

func TestDMLSinkAsyncRetryAndFinalErrorAreObservable(t *testing.T) {
	t.Run("retry succeeds", func(t *testing.T) {
		executor, err := NewAsyncExecutor(1)
		if err != nil {
			t.Fatal(err)
		}
		defer executor.Shutdown(context.Background())
		fake := &fakeExecutor{errors: []error{errors.New("temporary"), nil}}
		sink, err := NewDMLSink(DMLSpec{
			Executor:     fake,
			Statement:    "insert into events(value) values (?)",
			Bindings:     []Binding{{Property: "value"}},
			Retry:        2,
			WorkExecutor: executor,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := sink.Start(); err != nil {
			t.Fatal(err)
		}
		task, err := sink.WriteAsync(context.Background(), Record{"value": 7})
		if err != nil {
			t.Fatal(err)
		}
		if err := task.Wait(context.Background()); err != nil {
			t.Fatal(err)
		}
		fake.mu.Lock()
		calls := len(fake.statements)
		fake.mu.Unlock()
		if calls != 2 {
			t.Fatalf("retry calls = %d", calls)
		}
		if err := sink.Destroy(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("final error", func(t *testing.T) {
		executor, err := NewAsyncExecutor(1)
		if err != nil {
			t.Fatal(err)
		}
		defer executor.Shutdown(context.Background())
		fake := &fakeExecutor{errors: []error{errors.New("one"), errors.New("two")}}
		sink, err := NewDMLSink(DMLSpec{
			Executor:     fake,
			Statement:    "insert into events(value) values (?)",
			Bindings:     []Binding{{Property: "value"}},
			Retry:        2,
			WorkExecutor: executor,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := sink.Start(); err != nil {
			t.Fatal(err)
		}
		task, err := sink.WriteAsync(context.Background(), Record{"value": 7})
		if err != nil {
			t.Fatal(err)
		}
		if err := task.Wait(context.Background()); err == nil || !strings.Contains(err.Error(), "after 2 attempt") {
			t.Fatalf("final task error = %v", err)
		}
		if err := sink.Destroy(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestDMLSinkWriteAsyncDefaultsToSameThread(t *testing.T) {
	fake := &fakeExecutor{}
	sink, err := NewDMLSink(DMLSpec{
		Executor:  fake,
		Statement: "insert into events(value) values (?)",
		Bindings:  []Binding{{Property: "value"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	task, err := sink.WriteAsync(context.Background(), Record{"value": 9})
	if err != nil {
		t.Fatal(err)
	}
	if err := task.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	calls := len(fake.statements)
	fake.mu.Unlock()
	if calls != 1 {
		t.Fatalf("same-thread async calls = %d", calls)
	}
	if err := sink.Destroy(); err != nil {
		t.Fatal(err)
	}
}

func TestUpsertSinkAsyncUpdatesThenInserts(t *testing.T) {
	executor, err := NewAsyncExecutor(1)
	if err != nil {
		t.Fatal(err)
	}
	defer executor.Shutdown(context.Background())
	fake := &fakeExecutor{}
	sink, err := NewUpsertSink(UpsertSpec{
		Executor:     fake,
		Table:        "events",
		Keys:         []Column{{Column: "key", Property: "key"}},
		Values:       []Column{{Column: "value", Property: "value"}},
		WorkExecutor: executor,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	task, err := sink.WriteAsync(context.Background(), Record{"key": "a", "value": 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := task.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	if len(fake.statements) != 2 || !strings.HasPrefix(strings.ToLower(fake.statements[0]), "update") || !strings.HasPrefix(strings.ToLower(fake.statements[1]), "insert") {
		t.Fatalf("async upsert statements = %#v", fake.statements)
	}
	fake.mu.Unlock()
	if err := sink.Destroy(); err != nil {
		t.Fatal(err)
	}
}

func TestAsyncSinkDestroyRejectsQueuedWorkBeforeSQLExecution(t *testing.T) {
	executor, err := NewAsyncExecutor(1)
	if err != nil {
		t.Fatal(err)
	}
	defer executor.Shutdown(context.Background())
	blockStarted := make(chan struct{})
	release := make(chan struct{})
	blocker, err := executor.Submit(context.Background(), func(context.Context) error {
		close(blockStarted)
		<-release
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	<-blockStarted
	fake := &fakeExecutor{}
	sink, err := NewDMLSink(DMLSpec{
		Executor:     fake,
		Statement:    "insert into events(value) values (?)",
		Bindings:     []Binding{{Property: "value"}},
		WorkExecutor: executor,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	task, err := sink.WriteAsync(context.Background(), Record{"value": 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Destroy(); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := blocker.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := task.Wait(context.Background()); !errors.Is(err, connectors.ErrDestroyed) {
		t.Fatalf("queued task after destroy = %v", err)
	}
	fake.mu.Lock()
	calls := len(fake.statements)
	fake.mu.Unlock()
	if calls != 0 {
		t.Fatalf("destroyed sink executed %d SQL calls", calls)
	}
}

func TestExecutorNameResolvesForDMLAndUpsert(t *testing.T) {
	services, err := NewExecutorServices(map[string]int{"db": 1})
	if err != nil {
		t.Fatal(err)
	}
	defer services.Shutdown(context.Background())

	fake := &fakeExecutor{}
	dml, err := NewDMLSink(DMLSpec{
		Executor:         fake,
		Statement:        "insert into events(value) values (?)",
		Bindings:         []Binding{{Property: "value"}},
		ExecutorName:     "db",
		ExecutorServices: services,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := dml.Start(); err != nil {
		t.Fatal(err)
	}
	task, err := dml.WriteAsync(context.Background(), Record{"value": 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := task.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := dml.Destroy(); err != nil {
		t.Fatal(err)
	}

	if _, err := NewUpsertSink(UpsertSpec{
		Executor:         fake,
		Table:            "events",
		Keys:             []Column{{Column: "key", Property: "key"}},
		Values:           []Column{{Column: "value", Property: "value"}},
		ExecutorName:     "missing",
		ExecutorServices: services,
	}); !errors.Is(err, ErrUnknownExecutor) {
		t.Fatalf("unknown upsert executor = %v", err)
	}
}

func TestAsyncExecutorSubmitIsSafeWithConcurrentShutdown(t *testing.T) {
	executor, err := NewAsyncExecutor(2)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for index := 0; index < 16; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = executor.Submit(context.Background(), func(context.Context) error { return nil })
		}()
	}
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- executor.Shutdown(context.Background()) }()
	wg.Wait()
	if err := <-shutdownDone; err != nil {
		t.Fatal(err)
	}
}
