package esper

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

type clientRuntimeThreadingBean struct {
	ID int `esper:"id"`
}

type clientRuntimeThreadingEvent struct{}

func registerClientRuntimeThreadingRepresentations(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterStruct[clientRuntimeThreadingBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[clientRuntimeThreadingEvent](env, "MyEvent"); err != nil {
		t.Fatal(err)
	}
	registrations := []func() error{
		func() error { _, err := RegisterMap(env, "MyMap", nil); return err },
		func() error { _, err := RegisterObjectArray(env, "MyOA", nil); return err },
		func() error { _, err := RegisterXML(env, "XMLType", nil); return err },
		func() error { _, err := RegisterJSON(env, "JsonEvent", nil); return err },
	}
	for _, register := range registrations {
		if err := register(); err != nil {
			t.Fatal(err)
		}
	}
}

func deployClientRuntimeThreadingPlan(t *testing.T, engine *Engine, plan Plan) *Statement {
	t.Helper()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	return deployment.Statements()[0]
}

func cleanupClientRuntimeThreadingEngine(t *testing.T, engine *Engine) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := engine.Close(ctx); err != nil {
			t.Errorf("close threading engine: %v", err)
		}
	})
}

func TestClientRuntimeThreadedConfigInboundParity(t *testing.T) {
	env := NewEnvironment()
	registerClientRuntimeThreadingRepresentations(t, env)
	engine := NewEngine(env, WithInboundWorkers(4, 64))
	cleanupClientRuntimeThreadingEngine(t, engine)
	var delivered atomic.Int64
	for index, eventType := range []string{"MyMap", "SupportBean", "XMLType", "MyOA", "JsonEvent"} {
		sleep := Func1[int, int]("sleep", func(value int) int {
			time.Sleep(2 * time.Millisecond)
			return value
		}, Literal(index))
		plan, err := env.Build(FromAny(env, eventType).Select(Alias("value", sleep)).Query(StatementName("inbound-" + eventType)))
		if err != nil {
			t.Fatal(err)
		}
		statement := deployClientRuntimeThreadingPlan(t, engine, plan)
		if _, err := statement.Subscribe(func(context.Context, ResultBatch) error {
			delivered.Add(1)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}

	var tasks []AsyncTask
	appendTask := func(task AsyncTask, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		tasks = append(tasks, task)
	}
	for index := 0; index < 2; index++ {
		appendTask(engine.SendAsync(context.Background(), "MyMap", map[string]any{}))
		appendTask(engine.SendEventAsync(context.Background(), &clientRuntimeThreadingBean{ID: index}))
		appendTask(engine.SendXMLAsync(context.Background(), "XMLType", []byte(`<myevent/>`)))
		appendTask(engine.SendObjectArrayAsync(context.Background(), "MyOA", []any{}))
		appendTask(engine.SendJSONAsync(context.Background(), "JsonEvent", []byte(`{}`)))
	}
	for _, task := range tasks {
		if err := task.Wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if err := tasks[0].Wait(context.Background()); err != nil {
		t.Fatalf("second AsyncTask.Wait changed result: %v", err)
	}
	if err := engine.WaitAsync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if delivered.Load() != 10 {
		t.Fatalf("inbound delivered = %d, want 10", delivered.Load())
	}
	stats := engine.ThreadingStats().Inbound
	if !stats.Enabled || stats.Workers != 4 || stats.Accepted != 10 || stats.Completed != 10 || stats.Pending != 0 || stats.Queued != 0 {
		t.Fatalf("inbound stats = %#v", stats)
	}

	var failures []ThreadingError
	engine.SetThreadingErrorHandler(func(_ context.Context, failure ThreadingError) {
		failures = append(failures, failure)
	})
	invalid, err := engine.SendAsync(context.Background(), "UnknownType", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if err := invalid.Wait(context.Background()); !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("inbound async invalid error = %v, want %s", err, ErrorUnknownName)
	}
	if len(failures) != 1 || failures[0].Kind != ThreadingInbound || !errors.Is(failures[0], ErrorUnknownName) {
		t.Fatalf("inbound failure handler = %#v", failures)
	}
}

func TestClientRuntimeThreadedConfigInboundFastShutdownParity(t *testing.T) {
	env := NewEnvironment()
	registerClientRuntimeThreadingRepresentations(t, env)
	sleep := Func1[int, int]("sleep-a-little", func(value int) int {
		time.Sleep(5 * time.Millisecond)
		return value
	}, Literal(1))
	plan, err := env.Build(Select(From[clientRuntimeThreadingEvent](env, "MyEvent"),
		Alias("value", sleep)).
		Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithInboundWorkers(2, 1024))
	cleanupClientRuntimeThreadingEngine(t, engine)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := deployment.Statements()[0].SetSubscriber(func(context.Context, SubscriberUpdate) error { return nil }); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 1000; index++ {
		if _, err := engine.SendEventAsync(context.Background(), &clientRuntimeThreadingEvent{}); err != nil {
			t.Fatal(err)
		}
	}
	_ = deployment
	closeContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := engine.Close(closeContext); err != nil {
		t.Fatalf("fast async shutdown: %v", err)
	}
	stats := engine.ThreadingStats().Inbound
	if stats.Accepted != 1000 || stats.Completed != 1000 || stats.Pending != 0 || stats.Dropped == 0 {
		t.Fatalf("fast-shutdown stats = %#v", stats)
	}
	if _, err := engine.SendEventAsync(context.Background(), &clientRuntimeThreadingEvent{}); !errors.Is(err, ErrorState) {
		t.Fatalf("submit after shutdown error = %v, want %s", err, ErrorState)
	}
}

func TestClientRuntimeThreadedConfigOutboundParity(t *testing.T) {
	env := NewEnvironment()
	registerClientRuntimeThreadingRepresentations(t, env)
	plan, err := env.Build(From[clientRuntimeThreadingBean](env, "SupportBean").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithOutboundWorkers(5, 16))
	cleanupClientRuntimeThreadingEngine(t, engine)
	statement := deployClientRuntimeThreadingPlan(t, engine, plan)
	gate := make(chan struct{})
	var delivered atomic.Int64
	if _, err := statement.Subscribe(func(context.Context, ResultBatch) error {
		<-gate
		delivered.Add(1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 5; index++ {
		if err := engine.SendEvent(context.Background(), &clientRuntimeThreadingBean{ID: index}); err != nil {
			t.Fatalf("outbound-enabled SendEvent blocked on listener: %v", err)
		}
	}
	if delivered.Load() != 0 {
		t.Fatalf("blocked outbound listener delivered early = %d", delivered.Load())
	}
	close(gate)
	if err := engine.WaitAsync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if delivered.Load() != 5 {
		t.Fatalf("outbound delivered = %d, want 5", delivered.Load())
	}
	stats := engine.ThreadingStats().Outbound
	if stats.Accepted != 5 || stats.Completed != 5 || stats.Pending != 0 {
		t.Fatalf("outbound stats = %#v", stats)
	}
}

func TestClientRuntimeThreadedConfigRouteParity(t *testing.T) {
	env := NewEnvironment()
	registerClientRuntimeThreadingRepresentations(t, env)
	engine := NewEngine(env, WithRouteWorkers(5, 16))
	cleanupClientRuntimeThreadingEngine(t, engine)
	gate := make(chan struct{})
	entered := make(chan struct{}, 32)
	plans := make([]Plan, 20)
	for index := range plans {
		blocked := Func1[int, int]("route-work", func(value int) int {
			entered <- struct{}{}
			<-gate
			return value
		}, Literal(index))
		plan, err := env.Build(Select(From[clientRuntimeThreadingBean](env, "SupportBean"), Alias("value", blocked)).
			Query(StatementName("route-statement-" + string(rune('a'+index)))))
		if err != nil {
			t.Fatal(err)
		}
		plans[index] = plan
	}
	deployment, err := engine.DeployPlans(context.Background(), plans)
	if err != nil {
		t.Fatal(err)
	}
	var delivered atomic.Int64
	for _, statement := range deployment.Statements() {
		if _, err := statement.Subscribe(func(context.Context, ResultBatch) error {
			delivered.Add(1)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	task, err := engine.RouteAsync(context.Background(), "SupportBean", &clientRuntimeThreadingBean{ID: 1})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("route worker did not begin statement evaluation")
	}
	close(gate)
	if err := task.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if delivered.Load() != 20 {
		t.Fatalf("route delivered = %d, want 20", delivered.Load())
	}
	stats := engine.ThreadingStats().Route
	if stats.Accepted != 1 || stats.Completed != 1 || stats.Pending != 0 {
		t.Fatalf("route stats = %#v", stats)
	}
}

func TestClientRuntimeThreadedConfigTimerParity(t *testing.T) {
	origin := time.Unix(0, 0).UTC()
	env := NewEnvironment()
	registerClientRuntimeThreadingRepresentations(t, env)
	engine := NewEngine(env, WithStartTime(origin), WithTimerWorkers(5, 16))
	cleanupClientRuntimeThreadingEngine(t, engine)
	base := FromAny(env, "MyMap")
	gate := make(chan struct{})
	entered := make(chan struct{}, 32)
	plans := make([]Plan, 20)
	for index := range plans {
		blocked := Func1[int, int]("timer-work", func(value int) int {
			entered <- struct{}{}
			<-gate
			return value
		}, Literal(index))
		pattern := PatternFromRecord(base, "event", Literal(true)).Then(TimerInterval(From[clientRuntimeThreadingBean](env, "SupportBean"), time.Second))
		plan, err := env.Build(pattern.Select(Alias("value", blocked)).Query(StatementName("timer-statement-" + string(rune('a'+index)))))
		if err != nil {
			t.Fatal(err)
		}
		plans[index] = plan
	}
	deployment, err := engine.DeployPlans(context.Background(), plans)
	if err != nil {
		t.Fatal(err)
	}
	var delivered atomic.Int64
	for _, statement := range deployment.Statements() {
		if _, err := statement.Subscribe(func(context.Context, ResultBatch) error {
			delivered.Add(1)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.Send(context.Background(), "MyMap", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	task, err := engine.AdvanceTimeAsync(context.Background(), origin.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("timer worker did not begin scheduled statement evaluation")
	}
	close(gate)
	if err := task.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if delivered.Load() != 20 {
		t.Fatalf("timer delivered = %d, want 20", delivered.Load())
	}
	if got := engine.Now(); !got.Equal(origin.Add(time.Second)) {
		t.Fatalf("timer worker clock = %s, want %s", got, origin.Add(time.Second))
	}
	stats := engine.ThreadingStats().Timer
	if stats.Accepted != 1 || stats.Completed != 1 || stats.Pending != 0 {
		t.Fatalf("timer stats = %#v", stats)
	}
}

func TestAsyncTaskDisabledAndWaitCancellation(t *testing.T) {
	env := NewEnvironment()
	registerClientRuntimeThreadingRepresentations(t, env)
	engine := NewEngine(env)
	cleanupClientRuntimeThreadingEngine(t, engine)
	if _, err := engine.SendEventAsync(context.Background(), &clientRuntimeThreadingBean{}); !errors.Is(err, ErrorState) {
		t.Fatalf("disabled inbound error = %v, want %s", err, ErrorState)
	}
	if stats := engine.ThreadingStats(); !reflect.DeepEqual(stats, RuntimeThreadingStats{}) {
		t.Fatalf("disabled threading stats = %#v", stats)
	}

	gate := make(chan struct{})
	entered := make(chan struct{}, 1)
	blocked := Func1[int, int]("wait-cancellation", func(value int) int {
		entered <- struct{}{}
		<-gate
		return value
	}, Literal(1))
	plan, err := env.Build(Select(From[clientRuntimeThreadingBean](env, "SupportBean"), Alias("value", blocked)).
		Query(StatementName("wait-cancellation")))
	if err != nil {
		t.Fatal(err)
	}
	asyncEngine := NewEngine(env, WithInboundWorkers(1, 1))
	cleanupClientRuntimeThreadingEngine(t, asyncEngine)
	deployClientRuntimeThreadingPlan(t, asyncEngine, plan)
	task, err := asyncEngine.SendEventAsync(context.Background(), &clientRuntimeThreadingBean{})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("inbound worker did not begin blocked task")
	}
	waitContext, cancel := context.WithCancel(context.Background())
	cancel()
	if err := task.Wait(waitContext); !errors.Is(err, ErrorCanceled) {
		t.Fatalf("AsyncTask.Wait canceled error = %v, want %s", err, ErrorCanceled)
	}
	if err := asyncEngine.WaitAsync(waitContext); !errors.Is(err, ErrorCanceled) {
		t.Fatalf("Engine.WaitAsync canceled error = %v, want %s", err, ErrorCanceled)
	}
	close(gate)
	if err := task.Wait(context.Background()); err != nil {
		t.Fatalf("AsyncTask.Wait after canceled waiter: %v", err)
	}
	if err := asyncEngine.WaitAsync(context.Background()); err != nil {
		t.Fatalf("Engine.WaitAsync after canceled waiter: %v", err)
	}
}
