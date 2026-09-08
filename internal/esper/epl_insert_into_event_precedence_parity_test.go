package esper

import (
	"context"
	"fmt"
	"reflect"
	"strings"
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

// ---- 4.342: non-constant precedence over contained events and output-rate
// routing (ords 4/9) ----

type epLvlC struct {
	ID         string `esper:"id"`
	Precedence int    `esper:"precedence"`
}

type epLvlB struct {
	Precedence int      `esper:"precedence"`
	C          []epLvlC `esper:"c"`
}

type epLvlA struct {
	B []epLvlB `esper:"b"`
}

// TestEPLInsertIntoEventPrecContainedChainParity covers
// EPLInsertIntoEventPrecNonConstInsertIntoContainedEvent
// (java-runtime-e99c72ba6838bd3b23f2): two chained contained-event routes
// (LvlA[b] into LvlB, LvlB[c] into LvlC), each carrying a non-constant
// event-precedence evaluated against the routed target event. The routed
// LvlB events drive the second route through the shared precedence queue,
// reproducing Java's two-stage ordering. Contract (flattened LvlC id order
// per LvlA send): C,B,D,A / C,A,B,D / A,B,C,D / H,D,A,G,F,B,I,E,C /
// B,G,H,D,F,A,C,I,E.
func TestEPLInsertIntoEventPrecContainedChainParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[epLvlA](env, "LvlA"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[epLvlB](env, "LvlB"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[epLvlC](env, "LvlC"); err != nil {
		t.Fatal(err)
	}

	lvlBRoute, err := env.Build(Select(
		Unnest[epLvlA, epLvlB](From[epLvlA](env, "LvlA"), Property[[]epLvlB](EventValue[epLvlA](), "b")),
	).InsertInto("LvlB", StatementName("r1"), EventPrecedence(Field[epLvlB, int]("precedence"))))
	if err != nil {
		t.Fatal(err)
	}
	lvlCRoute, err := env.Build(Select(
		Unnest[epLvlB, epLvlC](From[epLvlB](env, "LvlB"), Property[[]epLvlC](EventValue[epLvlB](), "c")),
	).InsertInto("LvlC", StatementName("r2"), EventPrecedence(Field[epLvlC, int]("precedence"))))
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := env.Build(Select(From[epLvlC](env, "LvlC"),
		Alias("id", Field[epLvlC, string]("id")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	for _, plan := range []Plan{lvlBRoute, lvlCRoute, consumer} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	defer func() { _ = engine.Close(context.Background()) }()

	var receivedIDs []string
	for _, deployment := range engine.Deployments() {
		for _, stmt := range deployment.Statements() {
			if stmt.Name() == "s0" {
				if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
					for _, result := range batch.New {
						receivedIDs = append(receivedIDs, result.Get("id").Any().(string))
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
		}
	}

	sends := []struct {
		lvls  []epLvlB
		order string
	}{
		{
			[]epLvlB{{Precedence: 10, C: []epLvlC{{ID: "A"}, {ID: "B", Precedence: 1}}}, {Precedence: 11, C: []epLvlC{{ID: "C", Precedence: 1}, {ID: "D"}}}},
			"C,B,D,A",
		},
		{
			[]epLvlB{{Precedence: 10, C: []epLvlC{{ID: "A"}, {ID: "B"}}}, {Precedence: 10, C: []epLvlC{{ID: "C", Precedence: 1}, {ID: "D"}}}},
			"C,A,B,D",
		},
		{
			[]epLvlB{{Precedence: 0, C: []epLvlC{{ID: "A"}, {ID: "B"}}}, {Precedence: 0, C: []epLvlC{{ID: "C"}, {ID: "D"}}}},
			"A,B,C,D",
		},
		{
			[]epLvlB{
				{Precedence: 100, C: []epLvlC{{ID: "A", Precedence: 2}, {ID: "B", Precedence: 1}, {ID: "C"}}},
				{Precedence: 101, C: []epLvlC{{ID: "D", Precedence: 2}, {ID: "E"}, {ID: "F", Precedence: 1}}},
				{Precedence: 103, C: []epLvlC{{ID: "G", Precedence: 1}, {ID: "H", Precedence: 2}, {ID: "I"}}},
			},
			"H,D,A,G,F,B,I,E,C",
		},
		{
			[]epLvlB{
				{Precedence: 103, C: []epLvlC{{ID: "A"}, {ID: "B", Precedence: 1}, {ID: "C"}}},
				{Precedence: 100, C: []epLvlC{{ID: "D", Precedence: 1}, {ID: "E"}, {ID: "F", Precedence: 1}}},
				{Precedence: 102, C: []epLvlC{{ID: "G", Precedence: 1}, {ID: "H", Precedence: 1}, {ID: "I"}}},
			},
			"B,G,H,D,F,A,C,I,E",
		},
	}

	for i, send := range sends {
		before := len(receivedIDs)
		if err := engine.Send(context.Background(), "LvlA", epLvlA{B: send.lvls}); err != nil {
			t.Fatal(err)
		}
		got := strings.Join(receivedIDs[before:], ",")
		if got != send.order {
			t.Fatalf("send %d: id order = %q, want %q", i+1, got, send.order)
		}
	}
}

// TestEPLInsertIntoEventPrecOutputRateParity covers
// EPLInsertIntoEventPrecConstantInsertIntoOutputRate
// (java-runtime-aa3e3052e879188342c6): three insert-into routes with
// precedences 1, 2 and 3 (the suite's computeEventPrecedence(3, *) static
// call reads the routed output event and yields the constant 3 — Go pins
// the constant directly, an approved adaptation), each buffering with
// `output every 2 events`. Two sends produce one batch of six routed rows
// ordered by precedence across statements and FIFO within: 13,23,12,22,11,21.
func TestEPLInsertIntoEventPrecOutputRateParity(t *testing.T) {
	env := NewEnvironment()
	epRegisterSupportBean(t, env)
	if _, err := RegisterMap(env, "Out", []FieldSpec{
		FieldDef("id", reflect.TypeOf(0)),
	}); err != nil {
		t.Fatal(err)
	}

	intPrimitive := Field[epSupportBean, int]("intPrimitive")
	routes := []struct {
		precedence, offset int
	}{
		{1, 1}, {2, 2}, {3, 3},
	}
	var plans []Plan
	for _, route := range routes {
		plan, err := env.Build(Select(From[epSupportBean](env, "SupportBean"),
			Alias("id", Add[int](Literal(route.offset), Multiply[int](Literal(10), intPrimitive))),
		).InsertInto("Out", StatementName(fmt.Sprintf("p%d", route.precedence)),
			EventPrecedence(Literal(route.precedence)),
			WithOutput(OutputEvery(2))))
		if err != nil {
			t.Fatal(err)
		}
		plans = append(plans, plan)
	}
	consumer, err := env.Build(Select(From[map[string]any](env, "Out"),
		Alias("id", Field[map[string]any, int]("id")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	plans = append(plans, consumer)

	engine := NewEngine(env)
	for _, plan := range plans {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	defer func() { _ = engine.Close(context.Background()) }()

	var receivedIDs []int
	for _, deployment := range engine.Deployments() {
		for _, stmt := range deployment.Statements() {
			if stmt.Name() == "s0" {
				if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
					for _, result := range batch.New {
						receivedIDs = append(receivedIDs, result.Get("id").Any().(int))
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
		}
	}

	if err := engine.Send(context.Background(), "SupportBean", epSupportBean{IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SupportBean", epSupportBean{IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	want := []int{13, 23, 12, 22, 11, 21}
	if !reflect.DeepEqual(receivedIDs, want) {
		t.Fatalf("routed ids = %v, want %v", receivedIDs, want)
	}
}

// ---- 4.343: subquery-driven event precedence (ords 5-8) ----

type epSupportBeanNumeric struct {
	IntOne int `esper:"intOne"`
	IntTwo int `esper:"intTwo"`
}

// epSubqueryPrecedence builds the uncorrelated last-event subquery over
// SupportBeanNumeric that Java's event-precedence expressions use.
func epSubqueryPrecedence(t *testing.T, env *Environment, column string) Expr {
	t.Helper()
	inner := Select(From[epSupportBeanNumeric](env, "SupportBeanNumeric")).Window(LastEvent())
	field := Field[epSupportBeanNumeric, int](column)
	return SubqueryValue[int](inner, field)
}

// epRegisterOut registers the Out route target before producers build.
func epRegisterOut(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterMap(env, "Out", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
}

// epOutConsumer deploys a select-all consumer over Out and subscribes it,
// returning the routed id order.
func epOutConsumer(t *testing.T, env *Environment, engine *Engine) *[]string {
	t.Helper()
	consumer, err := env.Build(FromAny(env, "Out").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), consumer)
	if err != nil {
		t.Fatal(err)
	}
	var received []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			received = append(received, result.Get("id").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &received
}

// epAssertIDs asserts the ids accumulated since offset match the pinned
// order for one trigger round.
func epAssertIDs(t *testing.T, received *[]string, offset int, want ...string) {
	t.Helper()
	got := (*received)[offset:]
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("routed ids = [%s], want [%s]", strings.Join(got, ","), strings.Join(want, ","))
	}
}

// TestEPLInsertIntoEventPrecSubqueryOnSplitParity covers
// EPLInsertIntoEventPrecSubqueryOnSplitSODA
// (java-runtime-9e3c8c45438c687465e7): an on-supportBean split-all with two
// Out branches whose event-precedence expressions are uncorrelated
// last-event subqueries over SupportBeanNumeric. Null subquery results act
// as no precedence (FIFO). Contract per trigger: SB → a,b; N(1,2) → b,a;
// N(2,1) → a,b. Java's SODA flag is compile-path only.
func TestEPLInsertIntoEventPrecSubqueryOnSplitParity(t *testing.T) {
	env := NewEnvironment()
	epRegisterSupportBean(t, env)
	if _, err := RegisterStruct[epSupportBeanNumeric](env, "SupportBeanNumeric"); err != nil {
		t.Fatal(err)
	}
	epRegisterOut(t, env)
	engine := NewEngine(env, WithRuntimeURI("java-runtime-9e3c8c45438c687465e7"))
	defer func() { _ = engine.Close(context.Background()) }()

	producer, err := env.Build(OnEvent(From[epSupportBean](env, "SupportBean")).SplitAll(
		SplitIntoWithPrecedence(
			epSubqueryPrecedence(t, env, "intOne"), "Out", Alias("id", Literal("a"))),
		SplitIntoWithPrecedence(
			epSubqueryPrecedence(t, env, "intTwo"), "Out", Alias("id", Literal("b"))),
	).Query())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), producer); err != nil {
		t.Fatal(err)
	}
	received := epOutConsumer(t, env, engine)

	if err := engine.Send(context.Background(), "SupportBean", epSupportBean{}); err != nil {
		t.Fatal(err)
	}
	epAssertIDs(t, received, 0, "a", "b")
	// The numeric sends only prime the subquery; the next SupportBean
	// trigger routes with the primed precedences.
	if err := engine.Send(context.Background(), "SupportBeanNumeric", epSupportBeanNumeric{IntOne: 1, IntTwo: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SupportBean", epSupportBean{}); err != nil {
		t.Fatal(err)
	}
	epAssertIDs(t, received, 2, "b", "a")
	if err := engine.Send(context.Background(), "SupportBeanNumeric", epSupportBeanNumeric{IntOne: 2, IntTwo: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SupportBean", epSupportBean{}); err != nil {
		t.Fatal(err)
	}
	epAssertIDs(t, received, 4, "a", "b")
}

// TestEPLInsertIntoEventPrecSubqueryInsertIntoParity covers
// EPLInsertIntoEventPrecSubqueryInsertIntoSODA
// (java-runtime-03a6d8b8db3ead5f09f1): two insert-into routes whose
// event-precedence expressions are the same uncorrelated subqueries.
// Contract per trigger: SB → a,b; N(-2,-1) → b,a (−1 sorts before −2).
func TestEPLInsertIntoEventPrecSubqueryInsertIntoParity(t *testing.T) {
	env := NewEnvironment()
	epRegisterSupportBean(t, env)
	if _, err := RegisterStruct[epSupportBeanNumeric](env, "SupportBeanNumeric"); err != nil {
		t.Fatal(err)
	}
	epRegisterOut(t, env)
	engine := NewEngine(env, WithRuntimeURI("java-runtime-03a6d8b8db3ead5f09f1"))
	defer func() { _ = engine.Close(context.Background()) }()

	pa, err := env.Build(Select(From[epSupportBean](env, "SupportBean"),
		Alias("id", Literal("a")),
	).InsertInto("Out", StatementName("pa"),
		EventPrecedence(epSubqueryPrecedence(t, env, "intOne"))))
	if err != nil {
		t.Fatal(err)
	}
	pb, err := env.Build(Select(From[epSupportBean](env, "SupportBean"),
		Alias("id", Literal("b")),
	).InsertInto("Out", StatementName("pb"),
		EventPrecedence(epSubqueryPrecedence(t, env, "intTwo"))))
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range []Plan{pa, pb} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	received := epOutConsumer(t, env, engine)

	if err := engine.Send(context.Background(), "SupportBean", epSupportBean{}); err != nil {
		t.Fatal(err)
	}
	epAssertIDs(t, received, 0, "a", "b")
	if err := engine.Send(context.Background(), "SupportBeanNumeric", epSupportBeanNumeric{IntOne: -2, IntTwo: -1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SupportBean", epSupportBean{}); err != nil {
		t.Fatal(err)
	}
	epAssertIDs(t, received, 2, "b", "a")
}

// TestEPLInsertIntoEventPrecSubqueryMergeParity covers
// EPLInsertIntoEventPrecSubqueryMergeSODA
// (java-runtime-e153c3dff68d79fbb048): an on-supportBean merge of an empty
// keepall window whose when-not-matched actions route two Out rows with
// subquery precedences. Contract per trigger: SB → a,b; N(1,2) → b,a;
// N(2,1) → a,b.
func TestEPLInsertIntoEventPrecSubqueryMergeParity(t *testing.T) {
	env := NewEnvironment()
	epRegisterSupportBean(t, env)
	if _, err := RegisterStruct[epSupportBeanNumeric](env, "SupportBeanNumeric"); err != nil {
		t.Fatal(err)
	}
	windowSchema, err := NewMapSchema("MyWindow", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindow", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	epRegisterOut(t, env)
	engine := NewEngine(env, WithRuntimeURI("java-runtime-e153c3dff68d79fbb048"))
	defer func() { _ = engine.Close(context.Background()) }()

	producer, err := env.Build(OnEvent(From[epSupportBean](env, "SupportBean")).
		MergeIntoNamedWindowWhen("MyWindow", nil,
			WhenNotMatchedActions(
				ThenInsertIntoWithPrecedence(
					epSubqueryPrecedence(t, env, "intOne"), "Out", Alias("id", Literal("a"))),
				ThenInsertIntoWithPrecedence(
					epSubqueryPrecedence(t, env, "intTwo"), "Out", Alias("id", Literal("b"))),
			)).Query())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), producer); err != nil {
		t.Fatal(err)
	}
	received := epOutConsumer(t, env, engine)

	if err := engine.Send(context.Background(), "SupportBean", epSupportBean{}); err != nil {
		t.Fatal(err)
	}
	epAssertIDs(t, received, 0, "a", "b")
	if err := engine.Send(context.Background(), "SupportBeanNumeric", epSupportBeanNumeric{IntOne: 1, IntTwo: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SupportBean", epSupportBean{}); err != nil {
		t.Fatal(err)
	}
	epAssertIDs(t, received, 2, "b", "a")
	if err := engine.Send(context.Background(), "SupportBeanNumeric", epSupportBeanNumeric{IntOne: 2, IntTwo: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SupportBean", epSupportBean{}); err != nil {
		t.Fatal(err)
	}
	epAssertIDs(t, received, 4, "a", "b")
}

// TestEPLInsertIntoEventPrecSubqueryOnInsertParity covers
// EPLInsertIntoEventPrecSubqueryOnInsertSODA
// (java-runtime-d021d80d0590fb79c637): two on-supportBean on-select
// statements reading a pre-populated one-row window, each routing one Out
// row with a subquery precedence. Java pre-populates the window with a FAF
// `insert into MyWindow select 'x' as value`; Go uses the OnDemand insert.
// Contract per trigger: SB → a,b; N(1,2) → b,a; N(2,1) → a,b.
func TestEPLInsertIntoEventPrecSubqueryOnInsertParity(t *testing.T) {
	env := NewEnvironment()
	epRegisterSupportBean(t, env)
	if _, err := RegisterStruct[epSupportBeanNumeric](env, "SupportBeanNumeric"); err != nil {
		t.Fatal(err)
	}
	windowSchema, err := NewMapSchema("MyWindow", []FieldSpec{
		FieldDef("value", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindow", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	epRegisterOut(t, env)
	engine := NewEngine(env, WithRuntimeURI("java-runtime-d021d80d0590fb79c637"))
	defer func() { _ = engine.Close(context.Background()) }()

	faf, err := env.Build(FromNamedWindow(env, "MyWindow").OnDemand().InsertRows(
		InsertValues(Literal("x")),
	))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ExecuteFireAndForget(context.Background(), faf); err != nil {
		t.Fatal(err)
	}

	pa, err := env.Build(OnEvent(From[epSupportBean](env, "SupportBean")).
		SelectFromNamedWindow("MyWindow", nil, Alias("id", Literal("a"))).
		Query(StatementName("pa"),
			RouteTo("Out"),
			EventPrecedence(epSubqueryPrecedence(t, env, "intOne"))))
	if err != nil {
		t.Fatal(err)
	}
	pb, err := env.Build(OnEvent(From[epSupportBean](env, "SupportBean")).
		SelectFromNamedWindow("MyWindow", nil, Alias("id", Literal("b"))).
		Query(StatementName("pb"),
			RouteTo("Out"),
			EventPrecedence(epSubqueryPrecedence(t, env, "intTwo"))))
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range []Plan{pa, pb} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	received := epOutConsumer(t, env, engine)

	if err := engine.Send(context.Background(), "SupportBean", epSupportBean{}); err != nil {
		t.Fatal(err)
	}
	epAssertIDs(t, received, 0, "a", "b")
	if err := engine.Send(context.Background(), "SupportBeanNumeric", epSupportBeanNumeric{IntOne: 1, IntTwo: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SupportBean", epSupportBean{}); err != nil {
		t.Fatal(err)
	}
	epAssertIDs(t, received, 2, "b", "a")
	if err := engine.Send(context.Background(), "SupportBeanNumeric", epSupportBeanNumeric{IntOne: 2, IntTwo: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SupportBean", epSupportBean{}); err != nil {
		t.Fatal(err)
	}
	epAssertIDs(t, received, 4, "a", "b")
}
