package esper

import (
	"context"
	"testing"
	"time"
)

type metricsModuleIsolationEvent struct {
	ID    string `esper:"id"`
	Value int    `esper:"value"`
}

type metricsModuleIsolationObserved struct {
	newIDs []string
	oldIDs []string
}

func TestStatementConsumesNamedWindowScopesModule(t *testing.T) {
	t.Run("insert", func(t *testing.T) {
		env, engine, moduleA, moduleB, source, schema := newMetricsModuleIsolationRuntime(t, KeepAll())
		observedA, observedB := deployMetricsModuleIsolationConsumers(t, env, engine, moduleA, moduleB)
		_ = schema

		ctx := context.Background()
		if err := engine.InsertNamedWindowInModule(ctx, moduleA.Name(), "SharedWindow", metricsModuleIsolationEvent{ID: "A", Value: 1}); err != nil {
			t.Fatal(err)
		}
		if err := engine.InsertNamedWindowInModule(ctx, moduleB.Name(), "SharedWindow", metricsModuleIsolationEvent{ID: "B", Value: 2}); err != nil {
			t.Fatal(err)
		}

		assertMetricsModuleIsolationIDs(t, observedA.newIDs, []string{"A"}, "module A insert new stream")
		assertMetricsModuleIsolationIDs(t, observedB.newIDs, []string{"B"}, "module B insert new stream")
		if len(observedA.oldIDs) != 0 || len(observedB.oldIDs) != 0 {
			t.Fatalf("keep-all insert old streams = A:%v B:%v", observedA.oldIDs, observedB.oldIDs)
		}
		_ = source
	})
	t.Run("advance-time", func(t *testing.T) {
		env, engine, moduleA, moduleB, _, _ := newMetricsModuleIsolationRuntime(t, TimeWindow(time.Second))
		observedA, observedB := deployMetricsModuleIsolationConsumers(t, env, engine, moduleA, moduleB)
		ctx := context.Background()
		if err := engine.InsertNamedWindowInModule(ctx, moduleA.Name(), "SharedWindow", metricsModuleIsolationEvent{ID: "A", Value: 1}); err != nil {
			t.Fatal(err)
		}
		if err := engine.InsertNamedWindowInModule(ctx, moduleB.Name(), "SharedWindow", metricsModuleIsolationEvent{ID: "B", Value: 2}); err != nil {
			t.Fatal(err)
		}

		if err := engine.AdvanceTime(ctx, time.Unix(2, 0).UTC()); err != nil {
			t.Fatal(err)
		}

		assertMetricsModuleIsolationIDs(t, observedA.newIDs, []string{"A"}, "module A expiry new stream")
		assertMetricsModuleIsolationIDs(t, observedB.newIDs, []string{"B"}, "module B expiry new stream")
		assertMetricsModuleIsolationIDs(t, observedA.oldIDs, []string{"A"}, "module A expiry old stream")
		assertMetricsModuleIsolationIDs(t, observedB.oldIDs, []string{"B"}, "module B expiry old stream")
	})

	t.Run("route-dispatch", func(t *testing.T) {
		env, engine, moduleA, moduleB, source, _ := newMetricsModuleIsolationRuntime(t, KeepAll())
		observedA, observedB := deployMetricsModuleIsolationConsumers(t, env, engine, moduleA, moduleB)
		routePlan, err := moduleA.Build(source.InsertInto("SharedWindow", StatementName("route-a")))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Deploy(context.Background(), routePlan); err != nil {
			t.Fatal(err)
		}

		if err := engine.SendEvent(context.Background(), metricsModuleIsolationEvent{ID: "routed-A", Value: 3}); err != nil {
			t.Fatal(err)
		}
		assertMetricsModuleIsolationIDs(t, observedA.newIDs, []string{"routed-A"}, "module A route new stream")
		assertMetricsModuleIsolationIDs(t, observedB.newIDs, nil, "module B route new stream")
		if len(observedA.oldIDs) != 0 || len(observedB.oldIDs) != 0 {
			t.Fatalf("keep-all route old streams = A:%v B:%v", observedA.oldIDs, observedB.oldIDs)
		}
	})
}

func newMetricsModuleIsolationRuntime(t *testing.T, retention WindowSpec) (*Environment, *Engine, Module, Module, Stream[metricsModuleIsolationEvent], Schema) {
	t.Helper()
	env := NewEnvironment()
	schema, err := RegisterStruct[metricsModuleIsolationEvent](env, "MetricsModuleIsolationEvent")
	if err != nil {
		t.Fatal(err)
	}
	moduleA, err := env.RegisterModule("metrics-module-a", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	moduleB, err := env.RegisterModule("metrics-module-b", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := moduleA.RegisterNamedWindow("SharedWindow", schema, NamedWindowRetention(retention)); err != nil {
		t.Fatal(err)
	}
	if _, err := moduleB.RegisterNamedWindow("SharedWindow", schema, NamedWindowRetention(retention)); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env, WithStartTime(time.Unix(0, 0).UTC())), moduleA, moduleB, From[metricsModuleIsolationEvent](env, "MetricsModuleIsolationEvent"), schema
}

func deployMetricsModuleIsolationConsumers(t *testing.T, env *Environment, engine *Engine, moduleA, moduleB Module) (*metricsModuleIsolationObserved, *metricsModuleIsolationObserved) {
	t.Helper()
	observedA := &metricsModuleIsolationObserved{}
	observedB := &metricsModuleIsolationObserved{}
	for _, item := range []struct {
		module   Module
		name     string
		observed *metricsModuleIsolationObserved
	}{{moduleA, "consume-a", observedA}, {moduleB, "consume-b", observedB}} {
		plan, err := item.module.Build(item.module.NamedWindow("SharedWindow").Query(StatementName(item.name), WithOldStream()))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				event, ok := result.Event()
				if !ok {
					return NewError(ErrorTypeMismatch, "named-window new result is not an event")
				}
				if id, ok := event.Get("id").Any().(string); ok {
					item.observed.newIDs = append(item.observed.newIDs, id)
				}
			}
			for _, result := range batch.Old {
				event, ok := result.Event()
				if !ok {
					return NewError(ErrorTypeMismatch, "named-window old result is not an event")
				}
				if id, ok := event.Get("id").Any().(string); ok {
					item.observed.oldIDs = append(item.observed.oldIDs, id)
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	_ = env
	return observedA, observedB
}

func assertMetricsModuleIsolationIDs(t *testing.T, got, want []string, label string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("%s = %v, want %v", label, got, want)
		}
	}
}
