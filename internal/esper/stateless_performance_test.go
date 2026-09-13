package esper

import (
	"context"
	"reflect"
	"testing"
)

type statelessFilterEvent struct {
	Category string `esper:"category"`
	Value    int    `esper:"value"`
}

// statelessFilterEngine deploys one bare filter statement and returns the
// engine with a listener that records batches.
func statelessFilterEngine(t *testing.T, predicate Expression[bool]) (*Engine, *Deployment, *[]ResultBatch, func()) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[statelessFilterEvent](env, "StatelessFilterEvent"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[statelessFilterEvent](env, "StatelessFilterEvent").
		Filter(predicate).
		Query(StatementName("stateless-filter")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	ctx := context.Background()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := &[]ResultBatch{}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return engine, deployment, batches, func() { _ = engine.Close(ctx) }
}

// TestStatelessFilterListenerSemantics pins the observable result of the
// stateless execution path: exactly one wildcard row per accepted event with
// the event values and the send time, nothing for a rejected event, one
// sequence step per delivered row, and no delivery while the statement is
// stopped or undeployed.
func TestStatelessFilterListenerSemantics(t *testing.T) {
	ctx := context.Background()
	predicate := Equal[string](
		Field[statelessFilterEvent, string]("category"),
		Literal("keep"),
	)
	engine, deployment, batches, closeEngine := statelessFilterEngine(t, predicate)
	defer closeEngine()
	statement := deployment.Statements()[0]
	send := func(event statelessFilterEvent) {
		t.Helper()
		if err := engine.SendEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}

	send(statelessFilterEvent{Category: "drop", Value: 1})
	if len(*batches) != 0 {
		t.Fatalf("rejected event delivered %d batches", len(*batches))
	}

	send(statelessFilterEvent{Category: "keep", Value: 2})
	if len(*batches) != 1 {
		t.Fatalf("accepted event delivered %d batches, want 1", len(*batches))
	}
	first := (*batches)[0]
	if len(first.New) != 1 || len(first.Old) != 0 {
		t.Fatalf("batch = new %d old %d, want new 1 old 0", len(first.New), len(first.Old))
	}
	if !first.Time.Equal(engine.Now()) {
		t.Fatalf("batch time = %s, want %s", first.Time, engine.Now())
	}
	if got := first.New[0].Get("value").Any(); got != 2 {
		t.Fatalf("row value = %v, want 2", got)
	}
	if got := first.New[0].Get("category").Any(); got != "keep" {
		t.Fatalf("row category = %v, want keep", got)
	}
	if first.Sequence == 0 {
		t.Fatal("delivered batch has no sequence")
	}

	send(statelessFilterEvent{Category: "drop", Value: 3})
	send(statelessFilterEvent{Category: "keep", Value: 4})
	if len(*batches) != 2 {
		t.Fatalf("batches = %d, want 2", len(*batches))
	}
	second := (*batches)[1]
	if second.Sequence != first.Sequence+1 {
		t.Fatalf("sequence %d, want %d (rejected events must not consume a sequence)", second.Sequence, first.Sequence+1)
	}
	if got := second.New[0].Get("value").Any(); got != 4 {
		t.Fatalf("row value = %v, want 4", got)
	}

	if err := statement.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	send(statelessFilterEvent{Category: "keep", Value: 5})
	if len(*batches) != 2 {
		t.Fatalf("stopped statement delivered %d batches", len(*batches))
	}
	if err := statement.Start(ctx); err != nil {
		t.Fatal(err)
	}
	send(statelessFilterEvent{Category: "keep", Value: 6})
	if len(*batches) != 3 {
		t.Fatalf("restarted statement delivered %d batches, want 3", len(*batches))
	}
	if err := deployment.Undeploy(ctx); err != nil {
		t.Fatal(err)
	}
	send(statelessFilterEvent{Category: "keep", Value: 7})
	if len(*batches) != 3 {
		t.Fatalf("undeployed statement delivered %d batches", len(*batches))
	}
}

// TestStatelessFilterKeepsUserCodePredicateSemantics checks that a predicate
// containing a user function still decides acceptance through the generic
// path with the same rows a plain comparison produces.
func TestStatelessFilterKeepsUserCodePredicateSemantics(t *testing.T) {
	ctx := context.Background()
	predicate := Func1[statelessFilterEvent, bool]("statelessFilterKeep", func(event statelessFilterEvent) bool {
		return event.Value%2 == 0
	}, EventValue[statelessFilterEvent]())
	engine, _, batches, closeEngine := statelessFilterEngine(t, predicate)
	defer closeEngine()
	for _, value := range []int{1, 2, 3, 4} {
		if err := engine.SendEvent(ctx, statelessFilterEvent{Category: "any", Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	if len(*batches) != 2 {
		t.Fatalf("batches = %d, want 2 even values", len(*batches))
	}
	if got := (*batches)[0].New[0].Get("value").Any(); got != 2 {
		t.Fatalf("first row value = %v, want 2", got)
	}
	if got := (*batches)[1].New[0].Get("value").Any(); got != 4 {
		t.Fatalf("second row value = %v, want 4", got)
	}
}

// TestStatelessFilterKeepsGetterBackedPropertySemantics pins the purity
// boundary for property reads: a schema that resolves a property through a
// registered getter runs user code, so the statement must keep the generic
// pipeline's evaluation behavior instead of the single-evaluation shortcut.
// The generic pipeline evaluates the predicate exactly once per send: the
// dispatch loop's filter result is handed to the statement's own processing
// (genericFilterNodeLocked / takeDispatchFilterVerdict), so a getter — or any
// user function — inside a filter is invoked once per event and statement,
// matching Java's single filter-service evaluation. The count is deliberate:
// docs/esper-go-performance.md section 4.9.2 records the eliminated duplicate
// evaluation with differential evidence.
func TestStatelessFilterKeepsGetterBackedPropertySemantics(t *testing.T) {
	ctx := context.Background()
	env := NewEnvironment()
	getterCalls := 0
	if _, err := RegisterStruct[statelessFilterEvent](env, "StatelessFilterGetterEvent",
		WithPropertyGetter("category", reflect.TypeOf(""), func(any) (Value, error) {
			getterCalls++
			return Present("keep"), nil
		}),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[statelessFilterEvent](env, "StatelessFilterGetterEvent").
		Filter(Equal[string](
			Field[statelessFilterEvent, string]("category"),
			Literal("keep"),
		)).
		Query(StatementName("stateless-filter-getter")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(ctx) }()
	rows := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		rows += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, statelessFilterEvent{Category: "ignored", Value: 1}); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("rows = %d, want 1 (the getter returns the matching value)", rows)
	}
	if getterCalls != 1 {
		t.Fatalf("getter invocations = %d, want the single evaluation per send", getterCalls)
	}
}

// BenchmarkStatelessFilterSend measures the plain filter statement that the
// stateless execution path serves: a rejected event costs one predicate
// evaluation and no row, an accepted event adds one wildcard row.
func BenchmarkStatelessFilterSend(b *testing.B) {
	for _, test := range []struct {
		name  string
		value string
	}{{name: "rejected", value: "drop"}, {name: "accepted", value: "keep"}} {
		b.Run(test.name, func(b *testing.B) {
			env := NewEnvironment()
			if _, err := RegisterStruct[statelessFilterEvent](env, "StatelessFilterEvent"); err != nil {
				b.Fatal(err)
			}
			plan, err := env.Build(From[statelessFilterEvent](env, "StatelessFilterEvent").
				Filter(Equal[string](
					Field[statelessFilterEvent, string]("category"),
					Literal("keep"),
				)).
				Query(StatementName("stateless-filter")))
			if err != nil {
				b.Fatal(err)
			}
			engine := NewEngine(env)
			ctx := context.Background()
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = deployment.Undeploy(ctx) }()
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, _ ResultBatch) error { return nil }); err != nil {
				b.Fatal(err)
			}
			event := statelessFilterEvent{Category: test.value, Value: 1}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if err := engine.SendEvent(ctx, event); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkGenericFilterSend measures non-fast-path statements: a
// getter-backed property keeps the statement on the generic pipeline (the
// stateless plan refuses getter-backed reads), and a data window keeps even a
// pure predicate on the generic pipeline. Both shapes used to evaluate the
// stream filter predicate twice per send — once for the dispatch loop's
// accepted result and once inside the statement's own insert walk — so the
// numbers also track the eliminated duplicate evaluation.
func BenchmarkGenericFilterSend(b *testing.B) {
	for _, test := range []struct {
		name     string
		window   bool
		getter   bool
		category string
	}{
		{name: "getter-accepted", getter: true, category: "keep"},
		{name: "getter-rejected", getter: true, category: "drop"},
		{name: "window-accepted", window: true, category: "keep"},
		{name: "window-rejected", window: true, category: "drop"},
	} {
		b.Run(test.name, func(b *testing.B) {
			env := NewEnvironment()
			register := func(name string) error {
				if test.getter {
					_, err := RegisterStruct[statelessFilterEvent](env, name,
						WithPropertyGetter("category", reflect.TypeOf(""), func(any) (Value, error) {
							return Present(test.category), nil
						}))
					return err
				}
				_, err := RegisterStruct[statelessFilterEvent](env, name)
				return err
			}
			eventType := "GenericFilterGetterEvent"
			if !test.getter {
				eventType = "GenericFilterWindowEvent"
			}
			if err := register(eventType); err != nil {
				b.Fatal(err)
			}
			stream := From[statelessFilterEvent](env, eventType).
				Filter(Equal[string](
					Field[statelessFilterEvent, string]("category"),
					Literal("keep"),
				))
			if test.window {
				stream = stream.Window(LengthWindow(8))
			}
			plan, err := env.Build(stream.Query(StatementName("generic-filter")))
			if err != nil {
				b.Fatal(err)
			}
			engine := NewEngine(env)
			ctx := context.Background()
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = deployment.Undeploy(ctx) }()
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, _ ResultBatch) error { return nil }); err != nil {
				b.Fatal(err)
			}
			event := statelessFilterEvent{Category: test.category, Value: 1}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if err := engine.SendEvent(ctx, event); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
