package esper

import (
	"context"
	"fmt"
	"testing"
)

// The dispatch borrow contract (perf doc §4.9.3): the last consumer of a
// dispatch borrows the batch without a clone while every earlier consumer
// receives a private clone. These tests pin the two halves of that contract:
// one consumer's in-place slice mutation never reaches another consumer, and
// retained batches stay valid because the engine never reuses batch arrays.

// dispatchBorrowStatement deploys one plain filter statement with the given
// query options and returns the engine and its single statement.
func dispatchBorrowStatement(t *testing.T, options ...QueryOption) (*Engine, *Statement) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[statelessFilterEvent](env, "StatelessFilterEvent"); err != nil {
		t.Fatal(err)
	}
	stream := From[statelessFilterEvent](env, "StatelessFilterEvent").
		Filter(Equal[string](
			Field[statelessFilterEvent, string]("category"),
			Literal("keep"),
		))
	options = append([]QueryOption{StatementName("dispatch-borrow")}, options...)
	plan, err := env.Build(stream.Query(options...))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })
	return engine, deployment.Statements()[0]
}

// dispatchBorrowSnapshot records what one consumer observed from a batch:
// row count and the category values in delivery order.
func dispatchBorrowSnapshot(batch ResultBatch) (int, []string) {
	values := make([]string, 0, len(batch.New))
	for _, result := range batch.New {
		value, _ := result.Get("category").Any().(string)
		values = append(values, value)
	}
	return len(batch.New), values
}

// TestDispatchBorrowMutationIsolationAcrossListeners pins clone-for-all-but-
// last: a middle listener mutates its batch in place, the borrowing last
// listener mutates the dispatch copy, and neither mutation is observable
// through an earlier listener's retained slice header.
func TestDispatchBorrowMutationIsolationAcrossListeners(t *testing.T) {
	engine, statement := dispatchBorrowStatement(t)

	counts := make([]int, 3)
	categories := make([][]string, 3)
	var retainedFirstNew []Result
	for index := 0; index < 3; index++ {
		index := index
		listener := func(_ context.Context, batch ResultBatch) error {
			counts[index], categories[index] = dispatchBorrowSnapshot(batch)
			if index == 0 {
				// Retain the delivered slice header itself: under the borrow
				// contract this is the listener's private clone array.
				retainedFirstNew = batch.New
			}
			if index == 1 {
				// In-place mutation of this consumer's private clone.
				batch.New[0] = Result{}
				batch.New = batch.New[:0]
				batch.New = append(batch.New, Result{})
				batch.Old = nil
			}
			if index == 2 {
				// The borrower mutates the shared dispatch copy: earlier
				// consumers hold clones, so nothing observable changes.
				batch.New[0] = Result{}
				batch.New = append(batch.New, Result{})
			}
			return nil
		}
		if _, err := statement.Subscribe(listener); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SendEvent(context.Background(), statelessFilterEvent{Category: "keep", Value: 1}); err != nil {
		t.Fatal(err)
	}
	for index := range counts {
		if counts[index] != 1 {
			t.Fatalf("listener %d rows = %d, want 1", index, counts[index])
		}
		if len(categories[index]) != 1 || categories[index][0] != "keep" {
			t.Fatalf("listener %d categories = %v, want [keep]", index, categories[index])
		}
	}
	if len(retainedFirstNew) != 1 {
		t.Fatalf("retained first-listener slice = %d rows, want 1 (mutations by later consumers must not reach it)", len(retainedFirstNew))
	}
	if value, _ := retainedFirstNew[0].Get("category").Any().(string); value != "keep" {
		t.Fatalf("retained first-listener row category = %q, want keep", value)
	}
}

// TestDispatchBorrowSingleListenerRetainedBatch pins the retention half for
// the single-listener borrow path: the engine does not reuse or reset batch
// arrays, so a mutating listener that keeps the borrowed batch cannot corrupt
// the next delivery.
func TestDispatchBorrowSingleListenerRetainedBatch(t *testing.T) {
	engine, statement := dispatchBorrowStatement(t)

	var retained ResultBatch
	seen := 0
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		seen++
		if seen == 1 {
			retained = batch
			// Mutate the borrowed batch in every slice-level way.
			retained.New[0] = Result{}
			retained.New = append(retained.New, Result{})
			retained.Old = nil
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := engine.SendEvent(ctx, statelessFilterEvent{Category: "keep", Value: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, statelessFilterEvent{Category: "keep", Value: 2}); err != nil {
		t.Fatal(err)
	}
	if seen != 2 {
		t.Fatalf("deliveries = %d, want 2", seen)
	}
	if len(retained.New) != 2 {
		t.Fatalf("retained borrowed batch rows = %d, want the 2 rows its own mutation produced", len(retained.New))
	}
}

// TestDispatchBorrowSubscriberDetachedFromListeners pins that the subscriber
// consumes the batch without a clone yet stays isolated: its update copies
// the rows before listeners run, so a mutating listener never changes what
// the subscriber observed, and the retained update survives later sends.
func TestDispatchBorrowSubscriberDetachedFromListeners(t *testing.T) {
	engine, statement := dispatchBorrowStatement(t)

	var retainedUpdate SubscriberUpdate
	listenerRan := false
	if err := statement.SetSubscriber(func(_ context.Context, update SubscriberUpdate) error {
		retainedUpdate = update
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		listenerRan = true
		// This listener is both the only listener and the borrow consumer;
		// mutating the borrowed batch must not touch the subscriber's copies.
		batch.New[0] = Result{}
		batch.New = append(batch.New, Result{})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := engine.SendEvent(ctx, statelessFilterEvent{Category: "keep", Value: 1}); err != nil {
		t.Fatal(err)
	}
	if !listenerRan {
		t.Fatal("listener did not run")
	}
	rows := retainedUpdate.NewRows()
	if len(rows) != 1 {
		t.Fatalf("retained subscriber rows = %d, want 1", len(rows))
	}
	if value, _ := rows[0].Get("category").Any().(string); value != "keep" {
		t.Fatalf("retained subscriber row category = %q, want keep", value)
	}
	if values := rows[0].Values(); len(values) != 1 || values[0].IsMissing() {
		t.Fatalf("retained subscriber row values = %v, want the wildcard event value", values)
	}
}

// TestDispatchBorrowSinkIsolation pins that the sink, as the last consumer,
// borrows the pristine dispatch batch: a mutating listener cannot leak into
// it, and a sink that keeps the delivered batch stays valid across later
// sends because engine batch arrays are never reused.
func TestDispatchBorrowSinkIsolation(t *testing.T) {
	var sinkRetained ResultBatch
	sinkSeen := 0
	engine, statement := dispatchBorrowStatement(t, WithSink(SinkFunc(func(_ context.Context, batch ResultBatch) error {
		sinkSeen++
		if sinkSeen == 1 {
			sinkRetained = batch
		}
		return nil
	})))
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		// The listener runs before the sink and mutates its private clone.
		batch.New = batch.New[:0]
		batch.New = append(batch.New, Result{})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, value := range []int{1, 2} {
		if err := engine.SendEvent(ctx, statelessFilterEvent{Category: "keep", Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	if sinkSeen != 2 {
		t.Fatalf("sink deliveries = %d, want 2", sinkSeen)
	}
	if len(sinkRetained.New) != 1 {
		t.Fatalf("retained sink batch rows = %d, want 1 (listener mutation must not reach the sink)", len(sinkRetained.New))
	}
	if value, _ := sinkRetained.New[0].Get("category").Any().(string); value != "keep" {
		t.Fatalf("retained sink row category = %q, want keep", value)
	}
}

// TestDispatchBorrowReplayBufferIsolation pins the replayListener shape
// under borrowing: a live batch queued while SubscribeWithReplay buffers is
// cloned at queue time and the replay snapshot is cloned too, so mutating
// each delivered batch inside the callback cannot corrupt the other
// deliveries.
func TestDispatchBorrowReplayBufferIsolation(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[statelessFilterEvent](env, "StatelessFilterEvent"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[statelessFilterEvent](env, "StatelessFilterEvent").
		Window(LengthWindow(2)).
		Query(StatementName("dispatch-borrow-replay")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	ctx := context.Background()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deployment.Undeploy(ctx) })
	statement := deployment.Statements()[0]
	for _, value := range []int{1, 2} {
		if err := engine.SendEvent(ctx, statelessFilterEvent{Category: "keep", Value: value}); err != nil {
			t.Fatal(err)
		}
	}

	var replayValues []int
	var liveNewValues []int
	var liveOldValues []int
	liveDeliveries := 0
	// The replay callback sends an event back into the engine; that live
	// dispatch buffers until the replay finishes and is then delivered as its
	// own batch (the buffering path clones at queue time).
	if _, err := statement.SubscribeWithReplay(ctx, func(_ context.Context, batch ResultBatch) error {
		if liveDeliveries == 0 && len(batch.Old) == 0 && len(batch.New) == 2 {
			for _, result := range batch.New {
				value, _ := result.Get("value").Any().(int)
				replayValues = append(replayValues, value)
			}
			// Sending during the replay callback exercises the buffered
			// live-delivery path of the replay listener.
			if err := engine.SendEvent(ctx, statelessFilterEvent{Category: "keep", Value: 3}); err != nil {
				t.Error(err)
				return err
			}
		} else {
			liveDeliveries++
			for _, result := range batch.New {
				value, _ := result.Get("value").Any().(int)
				liveNewValues = append(liveNewValues, value)
			}
			for _, result := range batch.Old {
				value, _ := result.Get("value").Any().(int)
				liveOldValues = append(liveOldValues, value)
			}
		}
		// Mutate every delivery after recording it.
		for index := range batch.New {
			batch.New[index] = Result{}
		}
		batch.New = append(batch.New, Result{})
		batch.Old = nil
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(replayValues) != 2 || replayValues[0] != 1 || replayValues[1] != 2 {
		t.Fatalf("replay rows = %v, want [1 2]", replayValues)
	}
	if liveDeliveries != 1 {
		t.Fatalf("live deliveries = %d, want 1", liveDeliveries)
	}
	if len(liveNewValues) != 1 || liveNewValues[0] != 3 {
		t.Fatalf("live new rows = %v, want [3]", liveNewValues)
	}
	if len(liveOldValues) != 0 {
		// The default istream selector posts no old rows for the eviction.
		t.Fatalf("live old rows = %v, want none", liveOldValues)
	}
}

// TestDispatchBorrowSecondSendClean pins that a listener mutating its batch
// cannot influence the following send's delivery: each dispatch builds fresh
// arrays, so the second delivery still reports the full row.
func TestDispatchBorrowSecondSendClean(t *testing.T) {
	engine, statement := dispatchBorrowStatement(t)

	var deliveries [][]string
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		_, cats := dispatchBorrowSnapshot(batch)
		deliveries = append(deliveries, cats)
		// Mutate the borrowed batch after recording it.
		batch.New = batch.New[:0]
		batch.New = append(batch.New, Result{})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, value := range []int{1, 2} {
		if err := engine.SendEvent(ctx, statelessFilterEvent{Category: "keep", Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	if len(deliveries) != 2 {
		t.Fatalf("deliveries = %d, want 2", len(deliveries))
	}
	for index, cats := range deliveries {
		if len(cats) != 1 || cats[0] != "keep" {
			t.Fatalf("delivery %d categories = %v, want [keep]", index, cats)
		}
	}
}

// BenchmarkDispatchBorrowListeners measures the accepted-event dispatch cost
// of the borrow contract as the listener count grows. Before the borrow
// contract every listener paid one ResultBatch clone; now only all but the
// last listener pay it, so the borrowing consumer removes one clone per dispatch (the per-listener marginal cost was and stays +1).
func BenchmarkDispatchBorrowListeners(b *testing.B) {
	for _, listenerCount := range []int{1, 2, 4} {
		b.Run(fmt.Sprintf("listeners-%d", listenerCount), func(b *testing.B) {
			env := NewEnvironment()
			if _, err := RegisterStruct[statelessFilterEvent](env, "StatelessFilterEvent"); err != nil {
				b.Fatal(err)
			}
			plan, err := env.Build(From[statelessFilterEvent](env, "StatelessFilterEvent").
				Filter(Equal[string](
					Field[statelessFilterEvent, string]("category"),
					Literal("keep"),
				)).
				Query(StatementName("dispatch-borrow-bench")))
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
			for index := 0; index < listenerCount; index++ {
				if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, _ ResultBatch) error {
					return nil
				}); err != nil {
					b.Fatal(err)
				}
			}
			event := statelessFilterEvent{Category: "keep", Value: 1}
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
