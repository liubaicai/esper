package esper

import (
	"context"
	"testing"
)

// Extended parity coverage for ResultSetAggregateFirstLastWindow executions
// that exercise subquery, output-rate-limiting, late-initialize, and
// named-window mutation semantics. All use existing Go API.
//
// - Subquery: window aggregate in subquery over SupportBean#length(2)
// - OutputRateLimiting: output every 2 events with keepall window aggregates
// - LateInitialize: named window consumer deployed after inserts
// - MixedNamedWindow: named window + on-delete + grouped aggregate null rows

type flwxBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type flwxBeanA struct {
	ID string `esper:"id"`
}

type flwxBeanS0 struct {
	ID int `esper:"id"`
}

func registerFlwxTypes(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterStruct[flwxBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[flwxBeanA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[flwxBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
}

func flwxSubscribe(t *testing.T, stmt *Statement) *[]ResultBatch {
	t.Helper()
	batches := make([]ResultBatch, 0, 8)
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &batches
}

func flwxLastRow(t *testing.T, batches []ResultBatch) Row {
	t.Helper()
	if len(batches) == 0 {
		t.Fatal("no result batch")
	}
	last := batches[len(batches)-1]
	if len(last.New) != 1 {
		t.Fatalf("expected one new row, got %d", len(last.New))
	}
	row, ok := last.New[0].Row()
	if !ok {
		t.Fatalf("result is not a row: %#v", last.New[0])
	}
	return row
}

// TestResultSetAggregateSubqueryParity mirrors ResultSetAggregateSubquery:
// a subquery projects window(sb.*) from SupportBean#length(2) for each
// SupportBean_A event; the subquery evaluates the window at trigger time.
// Java runtime: java-runtime-20f27b9b18b7ea3b5180.
func TestResultSetAggregateSubqueryParity(t *testing.T) {
	env := NewEnvironment()
	registerFlwxTypes(t, env)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	inner := From[flwxBean](env, "SupportBean").Window(LengthWindow(2)).AsRecord()
	sub := MapValue[[]Event](SubqueryRow(inner, Alias("w", WindowEvents())), Literal("w"))

	plan, err := env.Build(
		Select(From[flwxBeanA](env, "SupportBean_A"),
			Alias("id", Field[flwxBeanA, string]("id")),
			Alias("w", sub),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := flwxSubscribe(t, deployment.Statements()[0])
	ctx := context.Background()

	// A1 → w = null (empty window)
	if err := engine.SendEvent(ctx, flwxBeanA{ID: "A1"}); err != nil {
		t.Fatal(err)
	}
	row := flwxLastRow(t, *batches)
	if row.Get("w").IsPresent() {
		t.Fatalf("A1 w should be null, got %v", row.Get("w").Any())
	}

	// SB(E1) → no output (s0 is on SupportBean_A)
	if err := engine.SendEvent(ctx, flwxBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}

	// A2 → w = [E1]
	if err := engine.SendEvent(ctx, flwxBeanA{ID: "A2"}); err != nil {
		t.Fatal(err)
	}
	row = flwxLastRow(t, *batches)
	w := row.Get("w").Any()
	if w == nil {
		t.Fatal("A2 w should not be null")
	}
	wSlice, ok := w.([]Event)
	if !ok {
		t.Fatalf("A2 w type = %T, want []Event", w)
	}
	if len(wSlice) != 1 {
		t.Fatalf("A2 w len = %d, want 1", len(wSlice))
	}
	if got := wSlice[0].Get("theString").Any(); got != "E1" {
		t.Fatalf("A2 w[0].theString = %v, want E1", got)
	}

	// SB(E2) → no output
	if err := engine.SendEvent(ctx, flwxBean{TheString: "E2", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}

	// A3 → w = [E1, E2]
	if err := engine.SendEvent(ctx, flwxBeanA{ID: "A3"}); err != nil {
		t.Fatal(err)
	}
	row = flwxLastRow(t, *batches)
	wSlice = row.Get("w").Any().([]Event)
	if len(wSlice) != 2 {
		t.Fatalf("A3 w len = %d, want 2", len(wSlice))
	}
	if got := wSlice[0].Get("theString").Any(); got != "E1" {
		t.Fatalf("A3 w[0].theString = %v, want E1", got)
	}
	if got := wSlice[1].Get("theString").Any(); got != "E2" {
		t.Fatalf("A3 w[1].theString = %v, want E2", got)
	}

	// SB(E3) → window rolls (length(2)), E1 pushed out
	if err := engine.SendEvent(ctx, flwxBean{TheString: "E3", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}

	// A4 → w = [E2, E3]
	if err := engine.SendEvent(ctx, flwxBeanA{ID: "A4"}); err != nil {
		t.Fatal(err)
	}
	row = flwxLastRow(t, *batches)
	wSlice = row.Get("w").Any().([]Event)
	if len(wSlice) != 2 {
		t.Fatalf("A4 w len = %d, want 2", len(wSlice))
	}
	if got := wSlice[0].Get("theString").Any(); got != "E2" {
		t.Fatalf("A4 w[0].theString = %v, want E2", got)
	}
	if got := wSlice[1].Get("theString").Any(); got != "E3" {
		t.Fatalf("A4 w[1].theString = %v, want E3", got)
	}
}

// TestResultSetAggregateOutputRateLimitingParity mirrors
// ResultSetAggregateOutputRateLimiting: output every 2 events with keepall
// window aggregates (sum + window). Java runtime: java-runtime-6ae9bc8636941d90dd91.
func TestResultSetAggregateOutputRateLimitingParity(t *testing.T) {
	env := NewEnvironment()
	registerFlwxTypes(t, env)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(
		From[flwxBean](env, "SupportBean").Window(KeepAll()).
			Aggregate(
				Alias("si", Sum[int](Field[flwxBean, int]("intPrimitive"))),
				Alias("wi", WindowValues[int](Field[flwxBean, int]("intPrimitive"))),
			).
			Query(StatementName("s0"), WithOutput(OutputEvery(2))),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := flwxSubscribe(t, deployment.Statements()[0])
	ctx := context.Background()

	send := func(v int) {
		t.Helper()
		if err := engine.SendEvent(ctx, flwxBean{TheString: "E", IntPrimitive: v}); err != nil {
			t.Fatal(err)
		}
	}

	// E(1), E(2) → output after 2 events: per-event snapshots
	send(1)
	send(2)
	if len(*batches) != 1 {
		t.Fatalf("expected 1 batch after 2 events, got %d", len(*batches))
	}
	last := (*batches)[len(*batches)-1]
	if len(last.New) != 2 {
		t.Fatalf("expected 2 rows in output-every-2 batch, got %d", len(last.New))
	}
	r0, _ := last.New[0].Row()
	r1, _ := last.New[1].Row()
	// Row 1: sum=1, window=[1]
	if got := r0.Get("si").Any(); got != 1 {
		t.Fatalf("row0 si = %v, want 1", got)
	}
	// Row 2: sum=3, window=[1,2]
	if got := r1.Get("si").Any(); got != 3 {
		t.Fatalf("row1 si = %v, want 3", got)
	}

	// E(3), E(4) → second output batch
	send(3)
	send(4)
	if len(*batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(*batches))
	}
	last = (*batches)[len(*batches)-1]
	if len(last.New) != 2 {
		t.Fatalf("expected 2 rows in second batch, got %d", len(last.New))
	}
	r0, _ = last.New[0].Row()
	r1, _ = last.New[1].Row()
	// Row 1: sum=6, window=[1,2,3]
	if got := r0.Get("si").Any(); got != 6 {
		t.Fatalf("batch2 row0 si = %v, want 6", got)
	}
	// Row 2: sum=10, window=[1,2,3,4]
	if got := r1.Get("si").Any(); got != 10 {
		t.Fatalf("batch2 row1 si = %v, want 10", got)
	}
}

// TestResultSetAggregateLateInitializeParity mirrors
// ResultSetAggregateLateInitialize: named window created and populated before
// the aggregate consumer is deployed; the consumer sees pre-existing rows.
// Java runtime: java-runtime-3d1a2b9deed08981b0f9.
func TestResultSetAggregateLateInitializeParity(t *testing.T) {
	env := NewEnvironment()
	registerFlwxTypes(t, env)

	// Create named window and build all plans before creating the engine
	// so the engine sees the named window registration.
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema missing")
	}
	if _, err := CreateNamedWindow(env, "MyWindowTwo", schema); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(
		OnEvent(From[flwxBean](env, "SupportBean")).InsertIntoNamedWindow("MyWindowTwo",
			SetColumn("theString", Field[flwxBean, string]("theString")),
			SetColumn("intPrimitive", Field[flwxBean, int]("intPrimitive")),
			SetColumn("longPrimitive", Field[flwxBean, int64]("longPrimitive")),
		).Query(StatementName("insert")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}

	// Insert E1, E2 before consumer exists
	ctx := context.Background()
	if err := engine.SendEvent(ctx, flwxBean{TheString: "E1", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, flwxBean{TheString: "E2", IntPrimitive: 20}); err != nil {
		t.Fatal(err)
	}

	// Deployment 2: aggregate consumer deployed after inserts
	theStr := Field[any, string]("theString")
	consumerPlan, err := env.Build(
		FromNamedWindow(env, "MyWindowTwo").
			Aggregate(
				Alias("firststring", First[string](theStr)),
				Alias("windowstring", WindowValues[string](theStr)),
				Alias("laststring", Last[string](theStr)),
			).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	batches := flwxSubscribe(t, consumerDeployment.Statements()[0])

	// Send E3 → consumer sees all three: first=E1, window=[E1,E2,E3], last=E3
	if err := engine.SendEvent(ctx, flwxBean{TheString: "E3", IntPrimitive: 30}); err != nil {
		t.Fatal(err)
	}
	row := flwxLastRow(t, *batches)
	if got := row.Get("firststring").Any(); got != "E1" {
		t.Fatalf("firststring = %v, want E1", got)
	}
	if got := row.Get("laststring").Any(); got != "E3" {
		t.Fatalf("laststring = %v, want E3", got)
	}
	ws := row.Get("windowstring").Any()
	wsSlice, ok := ws.([]string)
	if !ok {
		t.Fatalf("windowstring type = %T, want []string", ws)
	}
	if len(wsSlice) != 3 || wsSlice[0] != "E1" || wsSlice[1] != "E2" || wsSlice[2] != "E3" {
		t.Fatalf("windowstring = %v, want [E1 E2 E3]", wsSlice)
	}
}

// TestResultSetAggregateMixedNamedWindowParity mirrors
// ResultSetAggregateMixedNamedWindow: named window + on-delete + grouped
// aggregate with null rows for emptied groups.
// Java runtime: java-runtime-2aed2cacb4747c8fcbe6.
func TestResultSetAggregateMixedNamedWindowParity(t *testing.T) {
	env := NewEnvironment()
	registerFlwxTypes(t, env)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// Create named window
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema missing")
	}
	if _, err := CreateNamedWindow(env, "ABCWin", schema); err != nil {
		t.Fatal(err)
	}
	// Insert trigger
	insertPlan, err := env.Build(
		OnEvent(From[flwxBean](env, "SupportBean")).InsertIntoNamedWindow("ABCWin",
			SetColumn("theString", Field[flwxBean, string]("theString")),
			SetColumn("intPrimitive", Field[flwxBean, int]("intPrimitive")),
			SetColumn("longPrimitive", Field[flwxBean, int64]("longPrimitive")),
		).Query(StatementName("insert")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	// On-delete trigger
	deletePlan, err := env.Build(
		OnEvent(From[flwxBeanS0](env, "SupportBean_S0")).DeleteFromNamedWindow(
			"ABCWin",
			Equal[int](NamedWindowField[int]("intPrimitive"), Field[flwxBeanS0, int]("id")),
		).Query(StatementName("delete")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), deletePlan); err != nil {
		t.Fatal(err)
	}

	// Grouped aggregate consumer
	theStr := Field[any, string]("theString")
	intPrim := Field[any, int]("intPrimitive")
	consumerPlan, err := env.Build(
		FromNamedWindow(env, "ABCWin").
			GroupBy(theStr).
			Select(
				Alias("c0", theStr),
				Alias("c1", Sum[int](intPrim)),
				Alias("c2", WindowValues[int](intPrim)),
			).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	batches := flwxSubscribe(t, consumerDeployment.Statements()[0])
	ctx := context.Background()

	// SB(E1,10) → {E1,10,[10]}
	if err := engine.SendEvent(ctx, flwxBean{TheString: "E1", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	row := flwxLastRow(t, *batches)
	if got := row.Get("c0").Any(); got != "E1" {
		t.Fatalf("c0 = %v, want E1", got)
	}
	if got := row.Get("c1").Any(); got != 10 {
		t.Fatalf("c1 = %v, want 10", got)
	}

	// SB(E2,100) → {E2,100,[100]}
	if err := engine.SendEvent(ctx, flwxBean{TheString: "E2", IntPrimitive: 100}); err != nil {
		t.Fatal(err)
	}
	row = flwxLastRow(t, *batches)
	if got := row.Get("c0").Any(); got != "E2" {
		t.Fatalf("c0 = %v, want E2", got)
	}

	// S0(id=100) → delete E2/100 → {E2,null,null}
	if err := engine.SendEvent(ctx, flwxBeanS0{ID: 100}); err != nil {
		t.Fatal(err)
	}
	row = flwxLastRow(t, *batches)
	if got := row.Get("c0").Any(); got != "E2" {
		t.Fatalf("after delete c0 = %v, want E2", got)
	}
	if row.Get("c1").IsPresent() {
		t.Fatalf("after delete c1 should be null, got %v", row.Get("c1").Any())
	}

	// SB(E1,11) → {E1,21,[10,11]}
	if err := engine.SendEvent(ctx, flwxBean{TheString: "E1", IntPrimitive: 11}); err != nil {
		t.Fatal(err)
	}
	row = flwxLastRow(t, *batches)
	if got := row.Get("c0").Any(); got != "E1" {
		t.Fatalf("c0 = %v, want E1", got)
	}
	if got := row.Get("c1").Any(); got != 21 {
		t.Fatalf("c1 = %v, want 21", got)
	}

	// S0(id=10) → delete E1/10 → {E1,11,[11]}
	if err := engine.SendEvent(ctx, flwxBeanS0{ID: 10}); err != nil {
		t.Fatal(err)
	}
	row = flwxLastRow(t, *batches)
	if got := row.Get("c0").Any(); got != "E1" {
		t.Fatalf("after delete c0 = %v, want E1", got)
	}
	if got := row.Get("c1").Any(); got != 11 {
		t.Fatalf("after delete c1 = %v, want 11", got)
	}
}
