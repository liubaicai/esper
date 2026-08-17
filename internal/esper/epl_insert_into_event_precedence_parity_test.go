package esper

import (
	"context"
	"reflect"
	"testing"
)

// Parity coverage for EPLInsertIntoEventPrecedence constant-precedence
// executions: ConstantInsertInto, ConstantOnSplit, and
// ConstantInfraMergeInsertInto (table + named window variants).
//
// Java source: EPLInsertIntoEventPrecedence (11 executions).
// These tests verify that the Go runtime's event-precedence expression
// controls the processing order of insert-into routed events, matching
// Java Esper's WorkQueueWPrecedenceMayLatch behavior: higher precedence
// values process first; same-precedence events maintain FIFO order.

type epSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func epRegisterSupportBean(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterStruct[epSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
}

// TestEPLInsertIntoEventPrecConstantInsertIntoParity covers the
// EPLInsertIntoEventPrecConstantInsertInto runtime: multiple insert-into
// statements with different constant precedence values route events into a
// named window. The window iteration order follows descending precedence,
// with no-precedence events last.
func TestEPLInsertIntoEventPrecConstantInsertIntoParity(t *testing.T) {
	env := NewEnvironment()
	epRegisterSupportBean(t, env)

	// Register the target event type for routed events.
	if _, err := RegisterMap(env, "Out", []FieldSpec{
		FieldDef("id", reflect.TypeOf(0)),
	}); err != nil {
		t.Fatal(err)
	}

	// insert into Out event-precedence(4) select 4 as id from SupportBean
	p4, err := env.Build(
		Select(From[epSupportBean](env, "SupportBean"),
			Alias("id", Literal(4)),
		).InsertInto("Out", StatementName("p4"), EventPrecedence(Literal(4))),
	)
	if err != nil {
		t.Fatal(err)
	}

	// insert into Out event-precedence(2) select 2 as id from SupportBean
	p2, err := env.Build(
		Select(From[epSupportBean](env, "SupportBean"),
			Alias("id", Literal(2)),
		).InsertInto("Out", StatementName("p2"), EventPrecedence(Literal(2))),
	)
	if err != nil {
		t.Fatal(err)
	}

	// insert into Out select 0 as id from SupportBean (no precedence)
	p0, err := env.Build(
		Select(From[epSupportBean](env, "SupportBean"),
			Alias("id", Literal(0)),
		).InsertInto("Out", StatementName("p0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	// insert into Out event-precedence(5) select 5 as id from SupportBean
	p5, err := env.Build(
		Select(From[epSupportBean](env, "SupportBean"),
			Alias("id", Literal(5)),
		).InsertInto("Out", StatementName("p5"), EventPrecedence(Literal(5))),
	)
	if err != nil {
		t.Fatal(err)
	}

	// insert into Out event-precedence(1) select 1 as id from SupportBean
	p1, err := env.Build(
		Select(From[epSupportBean](env, "SupportBean"),
			Alias("id", Literal(1)),
		).InsertInto("Out", StatementName("p1"), EventPrecedence(Literal(1))),
	)
	if err != nil {
		t.Fatal(err)
	}

	// insert into Out event-precedence(3) select 3 as id from SupportBean
	p3, err := env.Build(
		Select(From[epSupportBean](env, "SupportBean"),
			Alias("id", Literal(3)),
		).InsertInto("Out", StatementName("p3"), EventPrecedence(Literal(3))),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Consumer: select * from Out — this receives the routed events in
	// precedence order and lets us assert the observable delivery sequence.
	consumer, err := env.Build(
		Select(From[map[string]any](env, "Out"),
			Alias("id", Field[map[string]any, int]("id")),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	for _, plan := range []Plan{p4, p2, p0, p5, p1, p3, consumer} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	defer engine.Close(context.Background())

	// Subscribe to the consumer to capture delivery order.
	var receivedIDs []int
	for _, deployment := range engine.Deployments() {
		for _, stmt := range deployment.Statements() {
			if stmt.Name() == "s0" {
				if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
					for _, r := range batch.New {
						receivedIDs = append(receivedIDs, r.Get("id").Any().(int))
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
		}
	}

	if err := engine.SendEvent(context.Background(), epSupportBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}

	// Java expects iteration order: 5, 4, 3, 2, 1, 0 (descending precedence,
	// no-precedence last).
	expectedOrder := []int{5, 4, 3, 2, 1, 0}
	if len(receivedIDs) != len(expectedOrder) {
		t.Fatalf("expected %d events, got %d: %v", len(expectedOrder), len(receivedIDs), receivedIDs)
	}
	for i, id := range receivedIDs {
		if id != expectedOrder[i] {
			t.Fatalf("event[%d]: expected id=%d, got id=%d (full order: %v)", i, expectedOrder[i], id, receivedIDs)
		}
	}
}

// TestEPLInsertIntoEventPrecConstantInfraMergeInsertIntoTableParity covers
// the EPLInsertIntoEventPrecConstantInfraMergeInsertInto(namedWindow=false)
// runtime: on-merge insert-into actions with different constant precedence
// values route into an event type. The consumer receives events in
// precedence order: c(1), a(0), b(no precedence).
func TestEPLInsertIntoEventPrecConstantInfraMergeInsertIntoTableParity(t *testing.T) {
	env := NewEnvironment()
	epRegisterSupportBean(t, env)

	// create table InfraMerge(mergeid string primary key)
	if _, err := CreateTable(env, "InfraMerge", []TableColumn{
		PrimaryKeyColumn[string]("mergeid"),
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := RegisterMap(env, "WindowOut", []FieldSpec{
		FieldDef("outid", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}

	// on SupportBean sb merge InfraMerge mw where sb.theString = mw.mergeid
	// when not matched
	// then insert into WindowOut event-precedence(0) select 'a' as outid
	// then insert into WindowOut select 'b' as outid
	// then insert into WindowOut event-precedence(1) select 'c' as outid
	mergePlan, err := env.Build(
		OnEvent(From[epSupportBean](env, "SupportBean")).
			MergeIntoTableWhen("InfraMerge",
				[]Expr{Field[epSupportBean, string]("theString")},
				WhenNotMatchedActions(
					ThenInsertIntoWithPrecedence(Literal(0), "WindowOut",
						Alias("outid", Literal("a")),
					),
					ThenInsertInto("WindowOut",
						Alias("outid", Literal("b")),
					),
					ThenInsertIntoWithPrecedence(Literal(1), "WindowOut",
						Alias("outid", Literal("c")),
					),
				),
			).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Consumer: select * from WindowOut
	consumer, err := env.Build(
		Select(From[map[string]any](env, "WindowOut"),
			Alias("outid", Field[map[string]any, string]("outid")),
		).Query(StatementName("c0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	for _, plan := range []Plan{mergePlan, consumer} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	defer engine.Close(context.Background())

	var receivedIDs []string
	for _, stmt := range engine.Deployments()[1].Statements() {
		if stmt.Name() == "c0" {
			if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, r := range batch.New {
					receivedIDs = append(receivedIDs, r.Get("outid").Any().(string))
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}
	}

	if err := engine.SendEvent(context.Background(), epSupportBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}

	// Java expects: c(1), a(0), b(no precedence)
	expectedOrder := []string{"c", "a", "b"}
	if len(receivedIDs) != len(expectedOrder) {
		t.Fatalf("expected %d events, got %d: %v", len(expectedOrder), len(receivedIDs), receivedIDs)
	}
	for i, id := range receivedIDs {
		if id != expectedOrder[i] {
			t.Fatalf("event[%d]: expected outid=%s, got outid=%s (full order: %v)", i, expectedOrder[i], id, receivedIDs)
		}
	}
}

// TestEPLInsertIntoEventPrecConstantInfraMergeInsertIntoWindowParity covers
// the EPLInsertIntoEventPrecConstantInfraMergeInsertInto(namedWindow=true)
// runtime: same as the table variant but merging into a named window.
func TestEPLInsertIntoEventPrecConstantInfraMergeInsertIntoWindowParity(t *testing.T) {
	env := NewEnvironment()
	epRegisterSupportBean(t, env)

	// create window InfraMerge#keepall as (mergeid string)
	mergeSchema, err := NewMapSchema("InfraMerge", []FieldSpec{
		FieldDef("mergeid", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "InfraMerge", mergeSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	if _, err := RegisterMap(env, "WindowOut", []FieldSpec{
		FieldDef("outid", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}

	// on SupportBean sb merge InfraMerge mw where sb.theString = mw.mergeid
	// when not matched
	// then insert into WindowOut event-precedence(0) select 'a' as outid
	// then insert into WindowOut select 'b' as outid
	// then insert into WindowOut event-precedence(1) select 'c' as outid
	mergePlan, err := env.Build(
		OnEvent(From[epSupportBean](env, "SupportBean")).
			MergeIntoNamedWindowWhen("InfraMerge",
				Equal[string](Field[epSupportBean, string]("theString"), NamedWindowField[string]("mergeid")),
				WhenNotMatchedActions(
					ThenInsertIntoWithPrecedence(Literal(0), "WindowOut",
						Alias("outid", Literal("a")),
					),
					ThenInsertInto("WindowOut",
						Alias("outid", Literal("b")),
					),
					ThenInsertIntoWithPrecedence(Literal(1), "WindowOut",
						Alias("outid", Literal("c")),
					),
				),
			).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Consumer: select * from WindowOut
	consumer, err := env.Build(
		Select(From[map[string]any](env, "WindowOut"),
			Alias("outid", Field[map[string]any, string]("outid")),
		).Query(StatementName("c0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	for _, plan := range []Plan{mergePlan, consumer} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	defer engine.Close(context.Background())

	var receivedIDs []string
	for _, stmt := range engine.Deployments()[1].Statements() {
		if stmt.Name() == "c0" {
			if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, r := range batch.New {
					receivedIDs = append(receivedIDs, r.Get("outid").Any().(string))
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}
	}

	if err := engine.SendEvent(context.Background(), epSupportBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}

	// Java expects: c(1), a(0), b(no precedence)
	expectedOrder := []string{"c", "a", "b"}
	if len(receivedIDs) != len(expectedOrder) {
		t.Fatalf("expected %d events, got %d: %v", len(expectedOrder), len(receivedIDs), receivedIDs)
	}
	for i, id := range receivedIDs {
		if id != expectedOrder[i] {
			t.Fatalf("event[%d]: expected outid=%s, got outid=%s (full order: %v)", i, expectedOrder[i], id, receivedIDs)
		}
	}
}

// TestEPLInsertIntoEventPrecConstantOnSplitParity covers the
// EPLInsertIntoEventPrecConstantOnSplit runtime: an on-split statement with
// multiple insert-into branches, each with a different constant precedence.
// The consumer receives events in descending precedence order: 3, 2, 1.
func TestEPLInsertIntoEventPrecConstantOnSplitParity(t *testing.T) {
	env := NewEnvironment()
	epRegisterSupportBean(t, env)

	if _, err := RegisterMap(env, "Out", []FieldSpec{
		FieldDef("id", reflect.TypeOf(0)),
	}); err != nil {
		t.Fatal(err)
	}

	// on SupportBean
	// insert into Out event-precedence(1) select 1 as id
	// insert into Out event-precedence(2) select 2 as id
	// insert into Out event-precedence(3) select 3 as id
	// output all
	splitPlan, err := env.Build(
		OnEvent(From[epSupportBean](env, "SupportBean")).
			SplitAll(
				SplitIntoWithPrecedence(Literal(1), "Out",
					Alias("id", Literal(1)),
				),
				SplitIntoWithPrecedence(Literal(2), "Out",
					Alias("id", Literal(2)),
				),
				SplitIntoWithPrecedence(Literal(3), "Out",
					Alias("id", Literal(3)),
				),
			).
			Query(StatementName("split")),
	)
	if err != nil {
		t.Fatal(err)
	}

	consumer, err := env.Build(
		Select(From[map[string]any](env, "Out"),
			Alias("id", Field[map[string]any, int]("id")),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	for _, plan := range []Plan{splitPlan, consumer} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	defer engine.Close(context.Background())

	var receivedIDs []int
	for _, stmt := range engine.Deployments()[1].Statements() {
		if stmt.Name() == "s0" {
			if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, r := range batch.New {
					receivedIDs = append(receivedIDs, r.Get("id").Any().(int))
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}
	}

	if err := engine.SendEvent(context.Background(), epSupportBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}

	// Java expects: 3, 2, 1 (descending precedence)
	expectedOrder := []int{3, 2, 1}
	if len(receivedIDs) != len(expectedOrder) {
		t.Fatalf("expected %d events, got %d: %v", len(expectedOrder), len(receivedIDs), receivedIDs)
	}
	for i, id := range receivedIDs {
		if id != expectedOrder[i] {
			t.Fatalf("event[%d]: expected id=%d, got id=%d (full order: %v)", i, expectedOrder[i], id, receivedIDs)
		}
	}
}
