package esper

import (
	"context"
	"testing"
	"time"
)

// genericFilterSingleEvalEvent is the source event for the single-evaluation
// pins. The statements below all keep the generic pipeline (a data window,
// a user function or a getter keeps the stateless fast path away) so the
// tests observe the dispatch-loop verdict reuse in statementRuntime.process.
type genericFilterSingleEvalEvent struct {
	Category string `esper:"category"`
	Value    int    `esper:"value"`
}

// genericFilterRouterEvent triggers the router statement of the routed
// dispatch pins; its output routes one GenericFilterEvent per trigger into
// the engine's routed dispatch queue.
type genericFilterRouterEvent struct {
	Category string `esper:"category"`
	Value    int    `esper:"value"`
}

func newGenericFilterSingleEvalEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[genericFilterSingleEvalEvent](env, "GenericFilterEvent"); err != nil {
		t.Fatal(err)
	}
	return env
}

// TestGenericFilterSingleEvaluationUserCodeCallCount pins the observability
// contract of the unified single filter evaluation: a user function inside a
// generic-path filter runs exactly once per event and statement — for the
// non-routed send path (dispatch verdict reused by the pipeline), for the
// routed path with the verdict computed (unmatched listener present), and for
// the routed path without any accepted consumer (the pipeline evaluates its
// own predicate once). Delivered rows are identical in all three setups.
func TestGenericFilterSingleEvaluationUserCodeCallCount(t *testing.T) {
	ctx := context.Background()
	events := []genericFilterSingleEvalEvent{
		{Category: "a", Value: 1},
		{Category: "b", Value: 2},
		{Category: "c", Value: 3},
		{Category: "d", Value: 4},
	}

	t.Run("send", func(t *testing.T) {
		calls := 0
		env := NewEnvironment()
		if _, err := RegisterStruct[genericFilterSingleEvalEvent](env, "GenericFilterEvent"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(ctx) }()
		deployment := deployWithCounter(t, env, engine, "send", &calls)
		rows := 0
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			rows += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		for _, event := range events {
			if err := engine.SendEvent(ctx, event); err != nil {
				t.Fatal(err)
			}
		}
		if calls != len(events) {
			t.Fatalf("user function calls = %d, want exactly one per event (%d)", calls, len(events))
		}
		if rows != 2 {
			t.Fatalf("rows = %d, want the two even values", rows)
		}
	})

	t.Run("routed-with-unmatched-listener", func(t *testing.T) {
		calls := 0
		env := NewEnvironment()
		if _, err := RegisterStruct[genericFilterSingleEvalEvent](env, "GenericFilterEvent"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[genericFilterRouterEvent](env, "GenericFilterRouterEvent"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(ctx) }()
		// The unmatched listener is what makes the routed dispatch compute
		// the accepted verdict (needsAccepted) instead of skipping it.
		engine.SetUnmatchedListener(func(_ context.Context, _ Event) error { return nil })
		deployment := deployWithCounter(t, env, engine, "routed-listener", &calls)
		rows := 0
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			rows += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		routerPlan, err := env.Build(From[genericFilterRouterEvent](env, "GenericFilterRouterEvent").
			Query(StatementName("router"), RouteTo("GenericFilterEvent")))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Deploy(ctx, routerPlan); err != nil {
			t.Fatal(err)
		}
		for _, event := range events {
			if err := engine.SendEvent(ctx, genericFilterRouterEvent{Category: event.Category, Value: event.Value}); err != nil {
				t.Fatal(err)
			}
		}
		if calls != len(events) {
			t.Fatalf("user function calls = %d, want exactly one per routed event (%d)", calls, len(events))
		}
		if rows != 2 {
			t.Fatalf("rows = %d, want the two even values", rows)
		}
	})

	t.Run("routed-without-accepted-consumer", func(t *testing.T) {
		calls := 0
		env := NewEnvironment()
		if _, err := RegisterStruct[genericFilterSingleEvalEvent](env, "GenericFilterEvent"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[genericFilterRouterEvent](env, "GenericFilterRouterEvent"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(ctx) }()
		// No unmatched listener, no metrics and no audit categories: the
		// routed dispatch skips the accepted computation entirely and the
		// pipeline evaluates its own predicate exactly once.
		deployment := deployWithCounter(t, env, engine, "routed-plain", &calls)
		rows := 0
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			rows += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		routerPlan, err := env.Build(From[genericFilterRouterEvent](env, "GenericFilterRouterEvent").
			Query(StatementName("router"), RouteTo("GenericFilterEvent")))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Deploy(ctx, routerPlan); err != nil {
			t.Fatal(err)
		}
		for _, event := range events {
			if err := engine.SendEvent(ctx, genericFilterRouterEvent{Category: event.Category, Value: event.Value}); err != nil {
				t.Fatal(err)
			}
		}
		if calls != len(events) {
			t.Fatalf("user function calls = %d, want exactly one per routed event (%d)", calls, len(events))
		}
		if rows != 2 {
			t.Fatalf("rows = %d, want the two even values", rows)
		}
	})
}

// deployWithCounter deploys one windowed generic-path statement whose filter
// predicate counts its own invocations. The window keeps the statement off
// the stateless fast path; the user function would keep it off regardless.
func deployWithCounter(t *testing.T, env *Environment, engine *Engine, name string, calls *int) *Deployment {
	t.Helper()
	predicate := Func1[genericFilterSingleEvalEvent, bool]("genericFilterSingleEvalKeep", func(event genericFilterSingleEvalEvent) bool {
		*calls++
		return event.Value%2 == 0
	}, EventValue[genericFilterSingleEvalEvent]())
	plan, err := env.Build(From[genericFilterSingleEvalEvent](env, "GenericFilterEvent").
		Filter(predicate).
		Window(LengthWindow(8)).
		Query(StatementName(name)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	return deployment
}

// TestGenericFilterSingleEvaluationKeepsMetricsAndAudit pins the statement
// observability surfaces across the unified single filter evaluation:
// metrics sample accepted events only (NumInput/NumOutputIStream), and the
// audit stream-filter record appears once per accepted event and never for a
// rejected one.
func TestGenericFilterSingleEvaluationKeepsMetricsAndAudit(t *testing.T) {
	ctx := context.Background()
	origin := time.Unix(0, 0).UTC()
	env := newGenericFilterSingleEvalEnv(t)
	engine := NewEngine(env, WithStartTime(origin), WithStatementMetrics(10*time.Second))
	defer func() { _ = engine.Close(ctx) }()
	if err := engine.AdvanceTime(ctx, origin.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	auditRecords := make([]AuditRecord, 0)
	auditSubscription, err := engine.SubscribeAudit(func(_ context.Context, record AuditRecord) error {
		auditRecords = append(auditRecords, record)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = auditSubscription.Close() }()

	statementReports := make([]StatementMetric, 0)
	metricsSubscription, err := engine.SubscribeStatementMetrics(func(_ context.Context, metric StatementMetric) error {
		statementReports = append(statementReports, metric)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = metricsSubscription.Close() }()

	plan, err := env.Build(From[genericFilterSingleEvalEvent](env, "GenericFilterEvent").
		Filter(Greater[int](
			Field[genericFilterSingleEvalEvent, int]("value"),
			Literal(2),
		)).
		Window(LengthWindow(8)).
		Query(StatementName("single-eval-metrics"), StatementAudit(AuditStream)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SetStatementMetricsEnabled(deployment.ID(), "single-eval-metrics", true); err != nil {
		t.Fatal(err)
	}
	rows := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		rows += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	send := func(event genericFilterSingleEvalEvent) {
		t.Helper()
		if err := engine.SendEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	flushAt := origin.Add(11 * time.Second)
	flush := func() []StatementMetric {
		t.Helper()
		if err := engine.AdvanceTime(ctx, flushAt); err != nil {
			t.Fatal(err)
		}
		flushAt = flushAt.Add(10 * time.Second)
		reports := statementReports
		statementReports = nil
		return reports
	}

	// Rejected event: no audit record, no metric input, no row. Inactive
	// statements produce no report at all.
	send(genericFilterSingleEvalEvent{Category: "a", Value: 1})
	if len(auditRecords) != 0 {
		t.Fatalf("rejected event audit records = %d, want 0", len(auditRecords))
	}
	reports := flush()
	if len(reports) != 0 {
		t.Fatalf("rejected event metric reports = %#v, want none (nothing sampled)", reports)
	}

	// Accepted event: one audit stream record, one sampled input, one row.
	send(genericFilterSingleEvalEvent{Category: "b", Value: 3})
	if len(auditRecords) != 1 || auditRecords[0].Category() != AuditStream {
		t.Fatalf("accepted event audit records = %#v, want one AuditStream record", auditRecords)
	}
	if rows != 1 {
		t.Fatalf("rows = %d, want 1", rows)
	}
	reports = flush()
	if len(reports) != 1 || reports[0].NumInput != 1 || reports[0].NumOutputIStream != 1 {
		t.Fatalf("accepted event metric reports = %#v, want input 1 output 1", reports)
	}

	// A second rejected event must stay invisible to both surfaces.
	send(genericFilterSingleEvalEvent{Category: "c", Value: 2})
	if len(auditRecords) != 1 {
		t.Fatalf("post-rejection audit records = %d, want still 1", len(auditRecords))
	}
	reports = flush()
	if len(reports) != 0 {
		t.Fatalf("post-rejection metric reports = %#v, want none (nothing sampled)", reports)
	}
}

// TestGenericFilterSingleEvaluationVariableSnapshot pins that the reused
// verdict observes the same variable snapshot the pipeline evaluation would:
// a filter on a variable-bound threshold flips exactly when the variable
// changes, with one evaluation per send.
func TestGenericFilterSingleEvaluationVariableSnapshot(t *testing.T) {
	ctx := context.Background()
	env := newGenericFilterSingleEvalEnv(t)
	if err := env.RegisterVariable("singleEvalThreshold", 10); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[genericFilterSingleEvalEvent](env, "GenericFilterEvent").
		Filter(Greater[int](
			Field[genericFilterSingleEvalEvent, int]("value"),
			VariableRef[int]("singleEvalThreshold"),
		)).
		Window(LengthWindow(8)).
		Query(StatementName("single-eval-variable")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(ctx) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		rows += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	send := func(value int) {
		t.Helper()
		if err := engine.SendEvent(ctx, genericFilterSingleEvalEvent{Category: "x", Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	send(5)
	send(15)
	if rows != 1 {
		t.Fatalf("rows = %d, want 1 (threshold 10)", rows)
	}
	if err := engine.SetVariable(ctx, "singleEvalThreshold", 20); err != nil {
		t.Fatal(err)
	}
	send(15)
	send(25)
	if rows != 2 {
		t.Fatalf("rows = %d, want 2 (threshold 20 admits only 25)", rows)
	}
}

// TestGenericFilterSingleEvaluationMembershipSlots pins the excluded
// multi-slot membership shape: predicates that deliver one pipeline copy per
// matching slice element keep the generic evaluation (and its full slot
// multiplicity) instead of the single-slot verdict shortcut.
func TestGenericFilterSingleEvaluationMembershipSlots(t *testing.T) {
	ctx := context.Background()
	env := newGenericFilterSingleEvalEnv(t)
	plan, err := env.Build(From[genericFilterSingleEvalEvent](env, "GenericFilterEvent").
		Filter(In[int](
			Field[genericFilterSingleEvalEvent, int]("value"),
			Literal([]int{4, 4}),
		)).
		Window(LengthWindow(8)).
		Query(StatementName("single-eval-membership")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(ctx) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		rows += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, genericFilterSingleEvalEvent{Category: "x", Value: 4}); err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Fatalf("rows = %d, want 2 (one slot per matching slice element)", rows)
	}
	if err := engine.SendEvent(ctx, genericFilterSingleEvalEvent{Category: "x", Value: 5}); err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Fatalf("rows = %d, want 2 (the non-member must not add slots)", rows)
	}
}
