package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// updateIStreamBean mirrors SupportBean (theString, intPrimitive, intBoxed,
// longBoxed) for the update-istream parity suite. Boxed properties are Go
// pointers, matching Esper's nullable Integer/Long.
type updateIStreamBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     *int   `esper:"intBoxed"`
	LongBoxed    *int64 `esper:"longBoxed"`
}

func updateIStreamInt(v int) *int      { return &v }
func updateIStreamLong(v int64) *int64 { return &v }

type updateIStreamCapture struct {
	newEvents  []Event
	oldEvents  []Event
	newResults []Result
	oldResults []Result
}

func (c *updateIStreamCapture) listener() Listener {
	return func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			c.newResults = append(c.newResults, result)
			if event, ok := result.Event(); ok {
				c.newEvents = append(c.newEvents, event)
			}
		}
		for _, result := range batch.Old {
			c.oldResults = append(c.oldResults, result)
			if event, ok := result.Event(); ok {
				c.oldEvents = append(c.oldEvents, event)
			}
		}
		return nil
	}
}

func newUpdateIStreamEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[updateIStreamBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[updateIStreamBean](env, "MyStream"); err != nil {
		t.Fatal(err)
	}
	return env
}

func deployUpdateIStream(t *testing.T, engine *Engine, plans ...Plan) []*Statement {
	t.Helper()
	statements := make([]*Statement, 0, len(plans))
	for _, plan := range plans {
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })
		statements = append(statements, deployment.Statements()...)
	}
	return statements
}

func assertUpdateIStreamEvent(t *testing.T, label string, event Event, theString string, intPrimitive int) {
	t.Helper()
	if got := event.Get("theString").Any(); got != theString {
		t.Fatalf("%s theString = %#v, want %#v", label, got, theString)
	}
	if got := event.Get("intPrimitive").Any(); got != intPrimitive {
		t.Fatalf("%s intPrimitive = %#v, want %#v", label, got, intPrimitive)
	}
}

// TestUpdateIStreamBeanParity mirrors EPLOtherUpdateBean: an insert-into chain
// feeding MyStream, an update on MyStream and a plain consumer. The source
// statement observes the pre-update event, the update listener receives the
// insert/remove pair and downstream consumers observe the updated copy; the
// original underlying value is never mutated (Java-probed semantics).
func TestUpdateIStreamBeanParity(t *testing.T) {
	env := newUpdateIStreamEnvironment(t)
	insertPlan, err := env.Build(From[updateIStreamBean](env, "SupportBean").InsertInto("MyStream", StatementName("Insert")))
	if err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(From[updateIStreamBean](env, "MyStream").UpdateStream(
		SetColumn("intPrimitive", Literal(10)),
		SetColumn("theString", Concat(Literal("O_"), Field[updateIStreamBean, string]("theString"))),
	).Where(Equal[int](Field[updateIStreamBean, int]("intPrimitive"), Literal(1))).Query(StatementName("Update")))
	if err != nil {
		t.Fatal(err)
	}
	selectPlan, err := env.Build(Select(From[updateIStreamBean](env, "MyStream")).Query(StatementName("Select")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	statements := deployUpdateIStream(t, engine, insertPlan, updatePlan, selectPlan)
	captures := make([]updateIStreamCapture, len(statements))
	for index, statement := range statements {
		if _, err := statement.Subscribe(captures[index].listener()); err != nil {
			t.Fatal(err)
		}
	}
	insert, update, selectCapture := &captures[0], &captures[1], &captures[2]

	if err := engine.Send(context.Background(), "SupportBean", updateIStreamBean{TheString: "E1", IntPrimitive: 9}); err != nil {
		t.Fatal(err)
	}
	if len(insert.newEvents) != 1 || len(selectCapture.newEvents) != 1 || len(update.newEvents) != 0 {
		t.Fatalf("E1 deliveries: insert=%d update=%d select=%d", len(insert.newEvents), len(update.newEvents), len(selectCapture.newEvents))
	}
	assertUpdateIStreamEvent(t, "insert E1", insert.newEvents[0], "E1", 9)
	assertUpdateIStreamEvent(t, "select E1", selectCapture.newEvents[0], "E1", 9)

	if err := engine.Send(context.Background(), "SupportBean", updateIStreamBean{TheString: "E2", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(insert.newEvents) != 2 || len(selectCapture.newEvents) != 2 || len(update.newEvents) != 1 || len(update.oldEvents) != 1 {
		t.Fatalf("E2 deliveries: insert=%d update=%d/%d select=%d", len(insert.newEvents), len(update.newEvents), len(update.oldEvents), len(selectCapture.newEvents))
	}
	assertUpdateIStreamEvent(t, "insert E2 (pre-update)", insert.newEvents[1], "E2", 1)
	assertUpdateIStreamEvent(t, "update new E2", update.newEvents[0], "O_E2", 10)
	assertUpdateIStreamEvent(t, "update old E2", update.oldEvents[0], "E2", 1)
	assertUpdateIStreamEvent(t, "select E2 (updated copy)", selectCapture.newEvents[1], "O_E2", 10)

	// The updated event is a copy: the original underlying observed by the
	// insert statement is a different instance and keeps the original values.
	if underlying, ok := insert.newEvents[1].Underlying().(updateIStreamBean); !ok || underlying.IntPrimitive != 1 || underlying.TheString != "E2" {
		t.Fatalf("insert underlying mutated or wrong type: %#v", insert.newEvents[1].Underlying())
	}
	if underlying, ok := selectCapture.newEvents[1].Underlying().(updateIStreamBean); !ok || underlying.IntPrimitive != 10 || underlying.TheString != "O_E2" {
		t.Fatalf("select underlying not the updated copy: %#v", selectCapture.newEvents[1].Underlying())
	}
}

// TestUpdateIStreamFieldUpdateOrderParity mirrors EPLOtherUpdateFieldUpdateOrder:
// every assignment expression evaluates against the pre-update event, so
// intBoxed=intPrimitive reads the original value even though intPrimitive
// itself is assigned earlier in the set clause.
func TestUpdateIStreamFieldUpdateOrderParity(t *testing.T) {
	env := newUpdateIStreamEnvironment(t)
	if err := env.RegisterVariable("myvar", 10); err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(From[updateIStreamBean](env, "SupportBean").UpdateStream(
		SetColumn("intPrimitive", VariableRef[int]("myvar")),
		SetColumn("intBoxed", Field[updateIStreamBean, int]("intPrimitive")),
	).Query(StatementName("update")))
	if err != nil {
		t.Fatal(err)
	}
	selectPlan, err := env.Build(Select(From[updateIStreamBean](env, "SupportBean")).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	statements := deployUpdateIStream(t, engine, updatePlan, selectPlan)
	captures := make([]updateIStreamCapture, len(statements))
	for index, statement := range statements {
		if _, err := statement.Subscribe(captures[index].listener()); err != nil {
			t.Fatal(err)
		}
	}

	if err := engine.Send(context.Background(), "SupportBean", updateIStreamBean{TheString: "E1", IntPrimitive: 1, IntBoxed: updateIStreamInt(2)}); err != nil {
		t.Fatal(err)
	}
	if len(captures[0].newEvents) != 1 || len(captures[1].newEvents) != 1 {
		t.Fatalf("deliveries: update=%d select=%d", len(captures[0].newEvents), len(captures[1].newEvents))
	}
	updated := captures[1].newEvents[0]
	assertUpdateIStreamEvent(t, "select E1", updated, "E1", 10)
	boxed, ok := updated.Get("intBoxed").Any().(*int)
	if !ok || boxed == nil || *boxed != 1 {
		t.Fatalf("intBoxed = %#v, want 1 (pre-update value)", updated.Get("intBoxed").Any())
	}
}

// TestUpdateIStreamInvalidParity covers the Go-enforceable slice of
// EPLOtherUpdateInvalid: unknown assignment properties, data windows on the
// update target, aggregate set expressions, duplicate properties and
// missing assignments are rejected at Build time. A non-boolean where clause
// is compile-time impossible in the typed Go builder (Where takes an
// Expression[bool]).
func TestUpdateIStreamInvalidParity(t *testing.T) {
	env := newUpdateIStreamEnvironment(t)
	assertInvalid := func(label string, build func() (Plan, error)) {
		t.Helper()
		if _, err := build(); err == nil {
			t.Fatalf("%s must be rejected", label)
		}
	}

	assertInvalid("unknown-property", func() (Plan, error) {
		return env.Build(From[updateIStreamBean](env, "SupportBean").UpdateStream(
			SetColumn("dummy", Literal(1)),
		).Query(StatementName("s0")))
	})
	assertInvalid("window-on-target", func() (Plan, error) {
		return env.Build(From[updateIStreamBean](env, "SupportBean").Window(LengthWindow(2)).UpdateStream(
			SetColumn("intPrimitive", Literal(1)),
		).Query(StatementName("s0")))
	})
	assertInvalid("aggregate-set", func() (Plan, error) {
		return env.Build(From[updateIStreamBean](env, "SupportBean").UpdateStream(
			SetColumn("intPrimitive", Sum[int](Field[updateIStreamBean, int]("intPrimitive"))),
		).Query(StatementName("s0")))
	})
	assertInvalid("duplicate-property", func() (Plan, error) {
		return env.Build(From[updateIStreamBean](env, "SupportBean").UpdateStream(
			SetColumn("intPrimitive", Literal(1)),
			SetColumn("intPrimitive", Literal(2)),
		).Query(StatementName("s0")))
	})
	assertInvalid("missing-assignments", func() (Plan, error) {
		return env.Build(From[updateIStreamBean](env, "SupportBean").UpdateStream().Query(StatementName("s0")))
	})
}

// updateIStreamDeployOne deploys a single plan and attaches a capture.
func updateIStreamDeployOne(t *testing.T, engine *Engine, plan Plan) (*Deployment, *updateIStreamCapture) {
	t.Helper()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	capture := &updateIStreamCapture{}
	if _, err := deployment.Statements()[0].Subscribe(capture.listener()); err != nil {
		t.Fatal(err)
	}
	return deployment, capture
}

// TestUpdateIStreamFieldsWithPriorityParity mirrors tryAssertionFieldsWithPriority
// (map representation): eight prioritized updates plus a drop entry chained on
// one stream, where clauses observing the running copy, drop removing events
// and undeploy changing the effective priority winner.
func TestUpdateIStreamFieldsWithPriorityParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[updateIStreamBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "MyStream", []FieldSpec{
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(int(0))),
	}); err != nil {
		t.Fatal(err)
	}
	insert1, err := env.Build(Select(
		From[updateIStreamBean](env, "SupportBean").Filter(Not(Like(Field[updateIStreamBean, string]("theString"), Literal("Z%")))),
		Alias("theString", Field[updateIStreamBean, string]("theString")),
		Alias("intPrimitive", Field[updateIStreamBean, int]("intPrimitive")),
	).InsertInto("MyStream", StatementName("insert-1")))
	if err != nil {
		t.Fatal(err)
	}
	insert2, err := env.Build(Select(
		From[updateIStreamBean](env, "SupportBean").Filter(Like(Field[updateIStreamBean, string]("theString"), Literal("Z%"))),
		Alias("theString", Concat(Literal("AX"), Field[updateIStreamBean, string]("theString"))),
		Alias("intPrimitive", Field[updateIStreamBean, int]("intPrimitive")),
	).InsertInto("MyStream", StatementName("insert-2")))
	if err != nil {
		t.Fatal(err)
	}
	like := func(pattern string) Expression[bool] {
		return Like(Field[Event, string]("theString"), Literal(pattern))
	}
	update := func(name string, priority int, value int, where Expression[bool]) Plan {
		t.Helper()
		chain := FromAny(env, "MyStream").UpdateStream(SetColumn("intPrimitive", Literal(value)))
		if where != nil {
			chain = chain.Where(where)
		}
		plan, err := env.Build(chain.Query(StatementName(name), UpdatePriority(priority)))
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	dropPlan, err := env.Build(FromAny(env, "MyStream").UpdateStream(
		SetColumn("intPrimitive", Literal(6)),
	).Where(like("B%")).Query(StatementName("h"), UpdateDrop()))
	if err != nil {
		t.Fatal(err)
	}
	s0FilteredPlan, err := env.Build(FromAny(env, "MyStream").Filter(
		Greater[int](Field[Event, int]("intPrimitive"), Literal(0)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	updateIStreamDeployOne(t, engine, insert1)
	updateIStreamDeployOne(t, engine, insert2)
	updateIStreamDeployOne(t, engine, update("a", 12, -2, Equal[int](Field[Event, int]("intPrimitive"), Literal(-1))))
	updateIStreamDeployOne(t, engine, update("b", 11, -1, like("D%")))
	cDeployment, _ := updateIStreamDeployOne(t, engine, update("c", 9, 9, like("A%")))
	dDeployment, _ := updateIStreamDeployOne(t, engine, update("d", 8, 8, Or(like("A%"), like("C%"))))
	eDeployment, _ := updateIStreamDeployOne(t, engine, update("e", 10, 10, like("A%")))
	fDeployment, _ := updateIStreamDeployOne(t, engine, update("f", 7, 7, Or(like("A%"), like("C%"))))
	gDeployment, _ := updateIStreamDeployOne(t, engine, update("g", 6, 6, like("A%")))
	updateIStreamDeployOne(t, engine, dropPlan)
	s0Deployment, s0 := updateIStreamDeployOne(t, engine, s0FilteredPlan)

	send := func(theString string, intPrimitive int) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean", updateIStreamBean{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
	}
	assertS0 := func(label, theString string, intPrimitive int) {
		t.Helper()
		if len(s0.newEvents) == 0 {
			t.Fatalf("%s: s0 not invoked", label)
		}
		event := s0.newEvents[len(s0.newEvents)-1]
		if got := event.Get("theString").Any(); got != theString {
			t.Fatalf("%s theString = %#v, want %#v", label, got, theString)
		}
		if got := event.Get("intPrimitive").Any(); got != intPrimitive {
			t.Fatalf("%s intPrimitive = %#v, want %d", label, got, intPrimitive)
		}
	}

	send("A1", 0) // g(6), f(7), d(8), c(9), e(10) apply in priority order; e wins
	assertS0("A1", "A1", 10)
	s0Count := len(s0.newEvents)
	send("B1", 0) // h drops the event
	if len(s0.newEvents) != s0Count {
		t.Fatalf("B1: drop did not remove the event from s0")
	}
	send("C1", 0) // f(7) then d(8): d wins
	assertS0("C1", "C1", 8)
	s0Count = len(s0.newEvents)
	send("D1", 100) // b(11) -> -1, then a(12) observes -1 -> -2; s0 filter hides it
	if len(s0.newEvents) != s0Count {
		t.Fatalf("D1: s0 filter must hide intPrimitive=-2")
	}

	// Redeploy s0 without the filter; negative values now arrive.
	if err := s0Deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	s0PlainPlan, err := env.Build(FromAny(env, "MyStream").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	_, s0 = updateIStreamDeployOne(t, engine, s0PlainPlan)
	send("D1", -2)
	assertS0("D1 plain", "D1", -2)
	send("Z1", -3) // insert-2 prefixes AX, then e(10) wins over g/f/d/c
	assertS0("Z1", "AXZ1", 10)

	// Undeploy e: the next-highest matching priority (c=9) wins.
	if err := eDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	send("Z2", 0)
	assertS0("Z2", "AXZ2", 9)

	// Undeploy c, d, f and g: no update matches, values pass through unchanged.
	for _, deployment := range []*Deployment{cDeployment, dDeployment, fDeployment, gDeployment} {
		if err := deployment.Undeploy(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	send("Z3", 0)
	assertS0("Z3", "AXZ3", 0)
}

// TestUpdateIStreamUnprioritizedOrderParity mirrors
// EPLOtherUpdateUnprioritizedOrder: unprioritized updates apply in deployment
// order, so the last deployed assignment wins.
func TestUpdateIStreamUnprioritizedOrderParity(t *testing.T) {
	env := NewEnvironment()
	fields := []FieldSpec{
		FieldDef("s0", reflect.TypeOf("")),
		FieldDef("s1", reflect.TypeOf(int(0))),
	}
	if _, err := RegisterMap(env, "MyMapTypeUO", fields); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "ABCStreamUO", fields); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(FromAny(env, "MyMapTypeUO").InsertInto("ABCStreamUO", StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(FromAny(env, "ABCStreamUO").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	updateIStreamDeployOne(t, engine, insertPlan)
	for _, name := range []string{"A", "B", "C", "D"} {
		plan, err := env.Build(FromAny(env, "ABCStreamUO").UpdateStream(
			SetColumn("s0", Literal(name)),
		).Query(StatementName(name)))
		if err != nil {
			t.Fatal(err)
		}
		updateIStreamDeployOne(t, engine, plan)
	}
	_, s0 := updateIStreamDeployOne(t, engine, s0Plan)

	if err := engine.Send(context.Background(), "MyMapTypeUO", map[string]any{"s0": "", "s1": 1}); err != nil {
		t.Fatal(err)
	}
	if len(s0.newEvents) != 1 {
		t.Fatalf("s0 deliveries = %d", len(s0.newEvents))
	}
	event := s0.newEvents[0]
	if got := event.Get("s0").Any(); got != "D" {
		t.Fatalf("s0 = %#v, want D (last deployed wins)", got)
	}
	if got := event.Get("s1").Any(); got != 1 {
		t.Fatalf("s1 = %#v, want 1", got)
	}
}

// TestUpdateIStreamListenerDeliveryMultiupdateParity mirrors
// EPLOtherUpdateListenerDeliveryMultiupdate: each update's insert/remove pair
// sees the running copy (old = state after the previous update), and the
// consumer observes the final assignment.
func TestUpdateIStreamListenerDeliveryMultiupdateParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[updateIStreamBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "ABCStreamLD", []FieldSpec{
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(int(0))),
		FieldDef("value1", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(Select(
		From[updateIStreamBean](env, "SupportBean"),
		Alias("theString", Field[updateIStreamBean, string]("theString")),
		Alias("intPrimitive", Field[updateIStreamBean, int]("intPrimitive")),
		Alias("value1", Literal("orig")),
	).InsertInto("ABCStreamLD", StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	update := func(name, value string, candidates ...int) Plan {
		t.Helper()
		inCandidates := make([]Expression[int], 0, len(candidates))
		for _, candidate := range candidates {
			inCandidates = append(inCandidates, Literal(candidate))
		}
		plan, err := env.Build(FromAny(env, "ABCStreamLD").UpdateStream(
			SetColumn("theString", Literal(name)),
			SetColumn("value1", Literal(value)),
		).Where(In[int](Field[Event, int]("intPrimitive"), inCandidates...)).Query(StatementName(name)))
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	s0Plan, err := env.Build(FromAny(env, "ABCStreamLD").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	_, insert := updateIStreamDeployOne(t, engine, insertPlan)
	_, listenerA := updateIStreamDeployOne(t, engine, update("A", "a", 1, 2))
	_, listenerB := updateIStreamDeployOne(t, engine, update("B", "b", 1, 3))
	_, listenerC := updateIStreamDeployOne(t, engine, update("C", "c", 2, 3))
	_, s0 := updateIStreamDeployOne(t, engine, s0Plan)

	assertTriple := func(label string, result Result, theString string, intPrimitive int, value1 string) {
		t.Helper()
		if got := result.Get("theString").Any(); got != theString {
			t.Fatalf("%s theString = %#v, want %#v", label, got, theString)
		}
		if got := result.Get("intPrimitive").Any(); got != intPrimitive {
			t.Fatalf("%s intPrimitive = %#v, want %d", label, got, intPrimitive)
		}
		if got := result.Get("value1").Any(); got != value1 {
			t.Fatalf("%s value1 = %#v, want %#v", label, got, value1)
		}
	}
	send := func(theString string, intPrimitive int) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean", updateIStreamBean{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
	}

	// E1: A and B apply; C does not.
	send("E1", 1)
	assertTriple("insert E1", insert.newResults[0], "E1", 1, "orig")
	assertTriple("A old E1", listenerA.oldResults[0], "E1", 1, "orig")
	assertTriple("A new E1", listenerA.newResults[0], "A", 1, "a")
	assertTriple("B old E1 (running copy)", listenerB.oldResults[0], "A", 1, "a")
	assertTriple("B new E1", listenerB.newResults[0], "B", 1, "b")
	if len(listenerC.newResults) != 0 {
		t.Fatalf("C must not fire on E1")
	}
	assertTriple("s0 E1", s0.newResults[0], "B", 1, "b")

	// E2: A and C apply; B does not.
	send("E2", 2)
	assertTriple("A old E2", listenerA.oldResults[1], "E2", 2, "orig")
	assertTriple("A new E2", listenerA.newResults[1], "A", 2, "a")
	if len(listenerB.newResults) != 1 {
		t.Fatalf("B must not fire on E2")
	}
	assertTriple("C old E2 (running copy)", listenerC.oldResults[0], "A", 2, "a")
	assertTriple("C new E2", listenerC.newResults[0], "C", 2, "c")
	assertTriple("s0 E2", s0.newResults[1], "C", 2, "c")

	// E3: B and C apply; A does not.
	send("E3", 3)
	if len(listenerA.newResults) != 2 {
		t.Fatalf("A must not fire on E3")
	}
	assertTriple("B old E3", listenerB.oldResults[1], "E3", 3, "orig")
	assertTriple("B new E3", listenerB.newResults[1], "B", 3, "b")
	assertTriple("C old E3 (running copy)", listenerC.oldResults[1], "B", 3, "b")
	assertTriple("C new E3", listenerC.newResults[1], "C", 3, "c")
	assertTriple("s0 E3", s0.newResults[2], "C", 3, "c")
}

// TestUpdateIStreamListenerDeliveryMultiupdateMixedParity mirrors
// EPLOtherUpdateListenerDeliveryMultiupdateMixed: five unprioritized updates
// apply in deployment order, only the listened ones deliver pairs, and the
// consumer observes the final assignment.
func TestUpdateIStreamListenerDeliveryMultiupdateMixedParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[updateIStreamBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "ABCStreamLDM", []FieldSpec{
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(int(0))),
		FieldDef("value1", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(Select(
		From[updateIStreamBean](env, "SupportBean"),
		Alias("theString", Field[updateIStreamBean, string]("theString")),
		Alias("intPrimitive", Field[updateIStreamBean, int]("intPrimitive")),
		Alias("value1", Literal("orig")),
	).InsertInto("ABCStreamLDM", StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(FromAny(env, "ABCStreamLDM").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	update := func(name, value string) Plan {
		t.Helper()
		plan, err := env.Build(FromAny(env, "ABCStreamLDM").UpdateStream(
			SetColumn("theString", Literal(name)),
			SetColumn("value1", Literal(value)),
		).Query(StatementName(name)))
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}

	engine := NewEngine(env)
	_, insert := updateIStreamDeployOne(t, engine, insertPlan)
	_, s0 := updateIStreamDeployOne(t, engine, s0Plan)
	updateIStreamDeployOne(t, engine, update("A", "a"))
	_, listenerB := updateIStreamDeployOne(t, engine, update("B", "b"))
	updateIStreamDeployOne(t, engine, update("C", "c"))
	_, listenerD := updateIStreamDeployOne(t, engine, update("D", "d"))
	updateIStreamDeployOne(t, engine, update("E", "e"))

	if err := engine.Send(context.Background(), "SupportBean", updateIStreamBean{TheString: "E4", IntPrimitive: 4}); err != nil {
		t.Fatal(err)
	}
	assertTriple := func(label string, result Result, theString string, intPrimitive int, value1 string) {
		t.Helper()
		if got := result.Get("theString").Any(); got != theString {
			t.Fatalf("%s theString = %#v, want %#v", label, got, theString)
		}
		if got := result.Get("intPrimitive").Any(); got != intPrimitive {
			t.Fatalf("%s intPrimitive = %#v, want %d", label, got, intPrimitive)
		}
		if got := result.Get("value1").Any(); got != value1 {
			t.Fatalf("%s value1 = %#v, want %#v", label, got, value1)
		}
	}
	assertTriple("insert E4", insert.newResults[0], "E4", 4, "orig")
	if len(listenerB.newResults) != 1 || len(listenerD.newResults) != 1 {
		t.Fatalf("B/D deliveries: %d/%d", len(listenerB.newResults), len(listenerD.newResults))
	}
	assertTriple("B old E4 (after A)", listenerB.oldResults[0], "A", 4, "a")
	assertTriple("B new E4", listenerB.newResults[0], "B", 4, "b")
	assertTriple("D old E4 (after C)", listenerD.oldResults[0], "C", 4, "c")
	assertTriple("D new E4", listenerD.newResults[0], "D", 4, "d")
	assertTriple("s0 E4 (final assignment)", s0.newResults[0], "E", 4, "e")
}


// TestUpdateIStreamInsertIntoWBeanWhereParity mirrors the full
// EPLOtherUpdateInsertIntoWBeanWhere matrix: two updates with where clauses
// chained on one insert-into stream, an update deployed after the consumer
// (which still preprocesses, matching Esper's router), undeploy/redeploy
// boundaries and a null boxed assignment skipped on a primitive target.
func TestUpdateIStreamInsertIntoWBeanWhereParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[updateIStreamBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[updateIStreamBean](env, "MyStreamBW"); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(From[updateIStreamBean](env, "SupportBean").InsertInto("MyStreamBW", StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	update1Plan, err := env.Build(From[updateIStreamBean](env, "MyStreamBW").UpdateStream(
		SetColumn("intPrimitive", Literal(10)),
		SetColumn("theString", Concat(Literal("O_"), Field[updateIStreamBean, string]("theString"))),
	).Where(Equal[int](Field[updateIStreamBean, int]("intPrimitive"), Literal(1))).Query(StatementName("update_1")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(Select(From[updateIStreamBean](env, "MyStreamBW")).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	update2Plan, err := env.Build(From[updateIStreamBean](env, "MyStreamBW").UpdateStream(
		SetColumn("intPrimitive", Add[int](Field[updateIStreamBean, int]("intPrimitive"), Literal(1000))),
	).Where(Equal[int](Field[updateIStreamBean, int]("intPrimitive"), Literal(2))).Query(StatementName("update_2")))
	if err != nil {
		t.Fatal(err)
	}
	update3Plan, err := env.Build(From[updateIStreamBean](env, "MyStreamBW").UpdateStream(
		SetColumn("intPrimitive", Field[updateIStreamBean, *int]("intBoxed")),
	).Query(StatementName("update_3")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deploy := func(plan Plan) (*Deployment, *updateIStreamCapture) {
		t.Helper()
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		capture := &updateIStreamCapture{}
		if _, err := deployment.Statements()[0].Subscribe(capture.listener()); err != nil {
			t.Fatal(err)
		}
		return deployment, capture
	}
	_, insert := deploy(insertPlan)
	update1Deployment, update1 := deploy(update1Plan)
	_, s0 := deploy(s0Plan)

	send := func(theString string, intPrimitive int) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean", updateIStreamBean{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
	}

	// E1/E3 do not match update_1's where; E2/E4 do.
	send("E1", 9)
	send("E2", 1)
	send("E3", 2)
	send("E4", 1)
	if len(s0.newEvents) != 4 || len(insert.newEvents) != 4 || len(update1.newEvents) != 2 {
		t.Fatalf("E1-E4 deliveries: insert=%d update_1=%d s0=%d", len(insert.newEvents), len(update1.newEvents), len(s0.newEvents))
	}
	assertUpdateIStreamEvent(t, "s0 E1", s0.newEvents[0], "E1", 9)
	assertUpdateIStreamEvent(t, "s0 E2 (updated)", s0.newEvents[1], "O_E2", 10)
	assertUpdateIStreamEvent(t, "s0 E3", s0.newEvents[2], "E3", 2)
	assertUpdateIStreamEvent(t, "s0 E4 (updated)", s0.newEvents[3], "O_E4", 10)
	assertUpdateIStreamEvent(t, "insert E4 (pre-update)", insert.newEvents[3], "E4", 1)
	assertUpdateIStreamEvent(t, "update_1 new E4", update1.newEvents[1], "O_E4", 10)
	assertUpdateIStreamEvent(t, "update_1 old E4", update1.oldEvents[1], "E4", 1)

	// update_2 is deployed after the s0 consumer but still preprocesses the
	// stream, so s0 observes the updated copy.
	update2Deployment, update2 := deploy(update2Plan)
	send("E5", 2)
	if len(update2.newEvents) != 1 || len(s0.newEvents) != 5 {
		t.Fatalf("E5 deliveries: update_2=%d s0=%d", len(update2.newEvents), len(s0.newEvents))
	}
	assertUpdateIStreamEvent(t, "s0 E5 (update_2 applied)", s0.newEvents[4], "E5", 1002)
	assertUpdateIStreamEvent(t, "update_2 new E5", update2.newEvents[0], "E5", 1002)
	assertUpdateIStreamEvent(t, "update_2 old E5", update2.oldEvents[0], "E5", 2)

	// Undeploy update_1: its where no longer fires and update_2 keeps working.
	if err := update1Deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	send("E6", 1)
	assertUpdateIStreamEvent(t, "s0 E6 (update_1 undeployed)", s0.newEvents[5], "E6", 1)
	if len(update2.newEvents) != 1 {
		t.Fatalf("update_2 fired on E6 (intPrimitive=1): %d", len(update2.newEvents))
	}
	send("E7", 2)
	assertUpdateIStreamEvent(t, "s0 E7 (update_2 applied)", s0.newEvents[6], "E7", 1002)
	assertUpdateIStreamEvent(t, "update_2 new E7", update2.newEvents[1], "E7", 1002)

	// Undeploy update_2: E10 passes through unchanged.
	if err := update2Deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	send("E10", 2)
	assertUpdateIStreamEvent(t, "s0 E10 (update_2 undeployed)", s0.newEvents[7], "E10", 2)

	// update_3 sets a primitive property from a null boxed property: Esper
	// skips the write, the pair still fires and the value is preserved.
	_, update3 := deploy(update3Plan)
	send("E11", 2)
	if len(update3.newEvents) != 1 {
		t.Fatalf("update_3 deliveries = %d", len(update3.newEvents))
	}
	assertUpdateIStreamEvent(t, "update_3 new E11 (null write skipped)", update3.newEvents[0], "E11", 2)
	assertUpdateIStreamEvent(t, "s0 E11 (value preserved)", s0.newEvents[8], "E11", 2)
}

// TestUpdateIStreamInsertIntoWMapNoWhereParity mirrors
// EPLOtherUpdateInsertIntoWMapNoWhere: a map-typed insert-into chain whose
// update swaps two properties without a where clause; both set expressions
// evaluate against the pre-update event, and undeploy/redeploy boundaries
// toggle the update.
func TestUpdateIStreamInsertIntoWMapNoWhereParity(t *testing.T) {
	env := NewEnvironment()
	fields := []FieldSpec{
		FieldDef("p0", reflect.TypeOf(int64(0))),
		FieldDef("p1", reflect.TypeOf(int64(0))),
		FieldDef("p2", reflect.TypeOf(int64(0))),
	}
	if _, err := RegisterMap(env, "MyMapTypeII", fields); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "MyStreamII", fields); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(FromAny(env, "MyMapTypeII").InsertInto("MyStreamII", StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(FromAny(env, "MyStreamII").UpdateStream(
		SetColumn("p0", Field[Event, int64]("p1")),
		SetColumn("p1", Field[Event, int64]("p0")),
	).Query(StatementName("update")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(FromAny(env, "MyStreamII").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deploy := func(plan Plan) (*Deployment, *updateIStreamCapture) {
		t.Helper()
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		capture := &updateIStreamCapture{}
		if _, err := deployment.Statements()[0].Subscribe(capture.listener()); err != nil {
			t.Fatal(err)
		}
		return deployment, capture
	}
	_, insert := deploy(insertPlan)
	updateDeployment, _ := deploy(updatePlan)
	_, s0 := deploy(s0Plan)

	send := func(p0, p1, p2 int64) {
		t.Helper()
		if err := engine.Send(context.Background(), "MyMapTypeII", map[string]any{"p0": p0, "p1": p1, "p2": p2}); err != nil {
			t.Fatal(err)
		}
	}
	assertProps := func(label string, event Event, p0, p1, p2 int64) {
		t.Helper()
		if got := event.Get("p0").Any(); got != p0 {
			t.Fatalf("%s p0 = %#v, want %d", label, got, p0)
		}
		if got := event.Get("p1").Any(); got != p1 {
			t.Fatalf("%s p1 = %#v, want %d", label, got, p1)
		}
		if got := event.Get("p2").Any(); got != p2 {
			t.Fatalf("%s p2 = %#v, want %d", label, got, p2)
		}
	}

	send(10, 1, 100)
	assertProps("s0 swapped", s0.newEvents[0], 1, 10, 100)
	assertProps("insert original", insert.newEvents[0], 10, 1, 100)

	// Undeploy and redeploy the update: the swap resumes for later events.
	if err := updateDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	updateDeployment, _ = deploy(updatePlan)
	send(5, 4, 101)
	assertProps("s0 swapped after redeploy", s0.newEvents[1], 4, 5, 101)
	assertProps("insert original after redeploy", insert.newEvents[1], 5, 4, 101)

	// Undeploy again: events pass through unchanged.
	if err := updateDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	send(20, 0, 102)
	assertProps("s0 passthrough after undeploy", s0.newEvents[2], 20, 0, 102)
	assertProps("insert passthrough after undeploy", insert.newEvents[2], 20, 0, 102)
}

// TestUpdateIStreamTypeWidenerParity mirrors EPLOtherUpdateTypeWidener:
// a boxed int widens into a boxed long and a null assignment writes null
// into a nullable property.
func TestUpdateIStreamTypeWidenerParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[updateIStreamBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[updateIStreamBean](env, "AStream"); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(From[updateIStreamBean](env, "SupportBean").InsertInto("AStream", StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(From[updateIStreamBean](env, "AStream").UpdateStream(
		SetColumn("longBoxed", Field[updateIStreamBean, *int]("intBoxed")),
		SetColumn("intBoxed", NullLiteral[*int]()),
	).Query(StatementName("update")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(Select(From[updateIStreamBean](env, "AStream")).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	statements := deployUpdateIStream(t, engine, insertPlan, updatePlan, s0Plan)
	captures := make([]updateIStreamCapture, len(statements))
	for index, statement := range statements {
		if _, err := statement.Subscribe(captures[index].listener()); err != nil {
			t.Fatal(err)
		}
	}

	if err := engine.Send(context.Background(), "SupportBean", updateIStreamBean{TheString: "E1", IntPrimitive: 0, IntBoxed: updateIStreamInt(999), LongBoxed: updateIStreamLong(888)}); err != nil {
		t.Fatal(err)
	}
	if len(captures[2].newEvents) != 1 {
		t.Fatalf("s0 deliveries = %d", len(captures[2].newEvents))
	}
	updated := captures[2].newEvents[0]
	assertUpdateIStreamEvent(t, "s0 E1", updated, "E1", 0)
	longBoxed, ok := updated.Get("longBoxed").Any().(*int64)
	if !ok || longBoxed == nil || *longBoxed != 999 {
		t.Fatalf("longBoxed = %#v, want widened 999", updated.Get("longBoxed").Any())
	}
	if got := updated.Get("intBoxed"); !got.IsNull() {
		t.Fatalf("intBoxed = %#v, want null", got.Any())
	}
	// The pre-update event keeps the original boxed values.
	original := captures[1].oldEvents[0]
	if boxed, ok := original.Get("intBoxed").Any().(*int); !ok || boxed == nil || *boxed != 999 {
		t.Fatalf("original intBoxed = %#v, want 999", original.Get("intBoxed").Any())
	}
}

// updateIStreamMapPropBean mirrors MyMapPropEvent (props Map, array Object[10])
// for the nested map/array write parity tests.
type updateIStreamMapPropBean struct {
	Props map[string]any `esper:"props"`
	Array []any          `esper:"array"`
}

// TestUpdateIStreamMapSetMapPropsBeanParity mirrors EPLOtherUpdateMapSetMapPropsBean:
// an update-istream statement writes a map entry (props('abc')=1) and an array
// element (array[2]=10) into a bean event routed through insert-into; the IR
// pair observes the new values on the copy and nulls on the original, and the
// sent bean's nested containers are never mutated (copy-on-write clones the
// containers before writing).
func TestUpdateIStreamMapSetMapPropsBeanParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[updateIStreamMapPropBean](env, "MyMapPropEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[updateIStreamMapPropBean](env, "MyStream"); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(FromAny(env, "MyMapPropEvent").InsertInto("MyStream", StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(FromAny(env, "MyStream").UpdateStream(
		SetMapEntry("props", Literal("abc"), Literal(1)),
		SetArrayElement("array", Literal(2), Literal(10)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	updateIStreamDeployOne(t, engine, insertPlan)
	_, s0 := updateIStreamDeployOne(t, engine, updatePlan)

	sent := updateIStreamMapPropBean{Props: map[string]any{}, Array: make([]any, 10)}
	if err := engine.Send(context.Background(), "MyMapPropEvent", sent); err != nil {
		t.Fatal(err)
	}
	if len(s0.newResults) != 1 || len(s0.oldResults) != 1 {
		t.Fatalf("s0 deliveries: new=%d old=%d", len(s0.newResults), len(s0.oldResults))
	}
	newProps, ok := s0.newResults[0].Get("props").Any().(map[string]any)
	if !ok || newProps["abc"] != 1 {
		t.Fatalf("s0 new props = %#v, want entry abc=1", s0.newResults[0].Get("props").Any())
	}
	newArray, ok := s0.newResults[0].Get("array").Any().([]any)
	if !ok || len(newArray) != 10 || newArray[2] != 10 {
		t.Fatalf("s0 new array = %#v, want element [2]=10", s0.newResults[0].Get("array").Any())
	}
	oldProps, ok := s0.oldResults[0].Get("props").Any().(map[string]any)
	if !ok {
		t.Fatalf("s0 old props = %#v, want map", s0.oldResults[0].Get("props").Any())
	}
	if _, exists := oldProps["abc"]; exists {
		t.Fatalf("s0 old props must not contain abc: %#v", oldProps)
	}
	oldArray, ok := s0.oldResults[0].Get("array").Any().([]any)
	if !ok || len(oldArray) != 10 || oldArray[2] != nil {
		t.Fatalf("s0 old array = %#v, want element [2]=nil", s0.oldResults[0].Get("array").Any())
	}
	// Copy-on-write: the sent bean's nested containers are never mutated.
	if _, exists := sent.Props["abc"]; exists || sent.Array[2] != nil {
		t.Fatalf("sent bean mutated: props=%#v array[2]=%#v", sent.Props, sent.Array[2])
	}
}

// TestUpdateIStreamMapSetMapPropsRepParity mirrors EPLOtherUpdateMapSetMapPropsRep
// (map representation): an update-istream statement directly on a map event
// type writes a plain column (simple='A'), a map entry (mymap('abc')=1) and an
// array element (myarray[2]=10); the IR pair observes {A, 1, 10} on the copy
// and {null, null, 0} on the original, and the sent map's nested containers
// are never mutated.
func TestUpdateIStreamMapSetMapPropsRepParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "MyInfraTypeWithMapProp", []FieldSpec{
		FieldDef("simple", reflect.TypeOf("")),
		FieldDef("myarray", reflect.TypeOf([]int{})),
		FieldDef("mymap", reflect.TypeOf(map[string]any{})),
	}); err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(FromAny(env, "MyInfraTypeWithMapProp").UpdateStream(
		SetColumn("simple", Literal("A")),
		SetMapEntry("mymap", Literal("abc"), Literal(1)),
		SetArrayElement("myarray", Literal(2), Literal(10)),
	).Query(StatementName("update")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	_, update := updateIStreamDeployOne(t, engine, updatePlan)

	sentMap := map[string]any{}
	sentArray := make([]int, 10)
	if err := engine.Send(context.Background(), "MyInfraTypeWithMapProp", map[string]any{"mymap": sentMap, "myarray": sentArray}); err != nil {
		t.Fatal(err)
	}
	if len(update.newResults) != 1 || len(update.oldResults) != 1 {
		t.Fatalf("update deliveries: new=%d old=%d", len(update.newResults), len(update.oldResults))
	}
	if got := update.newResults[0].Get("simple").Any(); got != "A" {
		t.Fatalf("update new simple = %#v, want A", got)
	}
	newMap, ok := update.newResults[0].Get("mymap").Any().(map[string]any)
	if !ok || newMap["abc"] != 1 {
		t.Fatalf("update new mymap = %#v, want entry abc=1", update.newResults[0].Get("mymap").Any())
	}
	newArray, ok := update.newResults[0].Get("myarray").Any().([]int)
	if !ok || len(newArray) != 10 || newArray[2] != 10 {
		t.Fatalf("update new myarray = %#v, want element [2]=10", update.newResults[0].Get("myarray").Any())
	}
	if got := update.oldResults[0].Get("simple").Any(); got != nil {
		t.Fatalf("update old simple = %#v, want nil", got)
	}
	oldMap, ok := update.oldResults[0].Get("mymap").Any().(map[string]any)
	if !ok {
		t.Fatalf("update old mymap = %#v, want map", update.oldResults[0].Get("mymap").Any())
	}
	if _, exists := oldMap["abc"]; exists {
		t.Fatalf("update old mymap must not contain abc: %#v", oldMap)
	}
	oldArray, ok := update.oldResults[0].Get("myarray").Any().([]int)
	if !ok || len(oldArray) != 10 || oldArray[2] != 0 {
		t.Fatalf("update old myarray = %#v, want element [2]=0", update.oldResults[0].Get("myarray").Any())
	}
	// Copy-on-write: the sent map's nested containers are never mutated.
	if _, exists := sentMap["abc"]; exists || sentArray[2] != 0 {
		t.Fatalf("sent containers mutated: mymap=%#v myarray[2]=%d", sentMap, sentArray[2])
	}
}

// TestUpdateIStreamNestedSetInvalidParity covers the Go-enforceable subset of
// EPLOtherUpdateArrayElementInvalid for nested write targets: non-array index
// targets, non-integer index expressions, non-map entry targets and
// non-string key expressions are all rejected at Build time. The remaining
// invalid matrix (including runtime index-overflow behavior) is tracked with
// the array-element executions.
func TestUpdateIStreamNestedSetInvalidParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "MyInfraTypeWithMapProp", []FieldSpec{
		FieldDef("simple", reflect.TypeOf("")),
		FieldDef("myarray", reflect.TypeOf([]int{})),
		FieldDef("mymap", reflect.TypeOf(map[string]any{})),
	}); err != nil {
		t.Fatal(err)
	}
	assertInvalid := func(label, want string, build func() (Plan, error)) {
		t.Helper()
		if _, err := build(); err == nil {
			t.Fatalf("%s must be rejected", label)
		} else if !strings.Contains(err.Error(), want) {
			t.Fatalf("%s error = %v, want substring %q", label, err, want)
		}
	}
	assertInvalid("index-on-non-array", "is not an array", func() (Plan, error) {
		return env.Build(FromAny(env, "MyInfraTypeWithMapProp").UpdateStream(
			SetArrayElement("simple", Literal(0), Literal(1)),
		).Query(StatementName("s0")))
	})
	assertInvalid("non-integer-index", "must return an integer", func() (Plan, error) {
		return env.Build(FromAny(env, "MyInfraTypeWithMapProp").UpdateStream(
			SetArrayElement("myarray", Literal("k"), Literal(1)),
		).Query(StatementName("s0")))
	})
	assertInvalid("entry-on-non-map", "is not a map", func() (Plan, error) {
		return env.Build(FromAny(env, "MyInfraTypeWithMapProp").UpdateStream(
			SetMapEntry("myarray", Literal("k"), Literal(1)),
		).Query(StatementName("s0")))
	})
	assertInvalid("non-string-key", "must return a string", func() (Plan, error) {
		return env.Build(FromAny(env, "MyInfraTypeWithMapProp").UpdateStream(
			SetMapEntry("mymap", Literal(1), Literal(1)),
		).Query(StatementName("s0")))
	})
}

// TestUpdateIStreamNWSetMapPropsParity mirrors EPLOtherUpdateNWSetMapProps (map
// representation): an on-trigger update on a named window writes a plain
// column, a map entry and an array element per row; a null map or a too-small
// array skips that nested write silently while the plain assignment still
// applies. SERDEREQUIRED (HA serde round-trip) has no Go counterpart.
func TestUpdateIStreamNWSetMapPropsParity(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterMap(env, "MyNWInfraTypeWithMapProp", []FieldSpec{
		FieldDef("simple", reflect.TypeOf("")),
		FieldDef("myarray", reflect.TypeOf([]int{})),
		FieldDef("mymap", reflect.TypeOf(map[string]any{})),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowWithMapProp", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[updateIStreamBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnRecord(FromAny(env, "MyNWInfraTypeWithMapProp")).InsertIntoNamedWindow("MyWindowWithMapProp",
		SetColumn("simple", Field[Event, any]("simple")),
		SetColumn("myarray", Field[Event, []int]("myarray")),
		SetColumn("mymap", Field[Event, map[string]any]("mymap")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(FromNamedWindow(env, "MyWindowWithMapProp").Query(
		StatementName("s0"),
		WithOldStream(),
	))
	if err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(OnEvent(From[updateIStreamBean](env, "SupportBean")).UpdateNamedWindow("MyWindowWithMapProp", Literal(true),
		SetColumn("simple", Literal("A")),
		SetMapEntry("mymap", Literal("abc"), Field[updateIStreamBean, int]("intPrimitive")),
		SetArrayElement("myarray", Literal(2), Field[updateIStreamBean, int]("intPrimitive")),
	).Query(StatementName("update")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	updateIStreamDeployOne(t, engine, insertPlan)
	_, s0 := updateIStreamDeployOne(t, engine, s0Plan)
	updateIStreamDeployOne(t, engine, updatePlan)

	assertRow := func(label string, result Result, simple any, arrayElement2 any, mapEntry any) {
		t.Helper()
		if got := result.Get("simple").Any(); got != simple {
			t.Fatalf("%s simple = %#v, want %#v", label, got, simple)
		}
		array, ok := result.Get("myarray").Any().([]int)
		if !ok {
			t.Fatalf("%s myarray = %#v, want []int", label, result.Get("myarray").Any())
		}
		if arrayElement2 == nil {
			for index, value := range array {
				if value != 0 {
					t.Fatalf("%s myarray[%d] = %d, want untouched zeros", label, index, value)
				}
			}
		} else if len(array) <= 2 || array[2] != arrayElement2 {
			t.Fatalf("%s myarray = %#v, want element [2]=%#v", label, array, arrayElement2)
		}
		if mapEntry == nil {
			if got := result.Get("mymap").Any(); got != nil {
				t.Fatalf("%s mymap = %#v, want nil", label, got)
			}
		} else {
			container, ok := result.Get("mymap").Any().(map[string]any)
			if !ok || container["abc"] != mapEntry {
				t.Fatalf("%s mymap = %#v, want entry abc=%#v", label, result.Get("mymap").Any(), mapEntry)
			}
		}
	}

	// Row 1: full nested writes apply.
	if err := engine.Send(context.Background(), "MyNWInfraTypeWithMapProp", map[string]any{"myarray": make([]int, 10), "mymap": map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), updateIStreamBean{TheString: "E1", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	if len(s0.newResults) != 2 || len(s0.oldResults) != 1 {
		t.Fatalf("E1 deliveries: new=%d old=%d", len(s0.newResults), len(s0.oldResults))
	}
	assertRow("row1 after E1", s0.newResults[1], "A", 10, 10)

	// Row 2 has a too-small array and a null map; row 1 is re-updated with 20.
	if err := engine.Send(context.Background(), "MyNWInfraTypeWithMapProp", map[string]any{"myarray": make([]int, 2)}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), updateIStreamBean{TheString: "E2", IntPrimitive: 20}); err != nil {
		t.Fatal(err)
	}
	if len(s0.newResults) != 5 || len(s0.oldResults) != 3 {
		t.Fatalf("E2 deliveries: new=%d old=%d", len(s0.newResults), len(s0.oldResults))
	}
	assertRow("row1 after E2", s0.newResults[3], "A", 20, 20)
	assertRow("row2 after E2", s0.newResults[4], "A", nil, nil)
}

// TestUpdateIStreamArrayElementParity mirrors EPLOtherUpdateArrayElement: the
// array index expression reads an event property (position), one statement
// writes two array properties, and downstream consumers observe the mutated
// arrays on the updated copy.
func TestUpdateIStreamArrayElementParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "Arriving", []FieldSpec{
		FieldDef("position", reflect.TypeOf(int(0))),
		FieldDef("intarray", reflect.TypeOf([]int{})),
		FieldDef("objectarray", reflect.TypeOf([]any{})),
	}); err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(FromAny(env, "Arriving").UpdateStream(
		SetArrayElement("intarray", Field[Event, int]("position"), Literal(1)),
		SetArrayElement("objectarray", Field[Event, int]("position"), Literal(1)),
	).Query(StatementName("update")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(FromAny(env, "Arriving").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	updateIStreamDeployOne(t, engine, updatePlan)
	_, s0 := updateIStreamDeployOne(t, engine, s0Plan)

	send := func(position int) {
		t.Helper()
		if err := engine.Send(context.Background(), "Arriving", map[string]any{
			"position":    position,
			"intarray":    make([]int, 3),
			"objectarray": make([]any, 3),
		}); err != nil {
			t.Fatal(err)
		}
	}
	send(1)
	send(0)
	send(2)
	wantInts := [][]int{{0, 1, 0}, {1, 0, 0}, {0, 0, 1}}
	wantObjects := [][]any{{nil, 1, nil}, {1, nil, nil}, {nil, nil, 1}}
	if len(s0.newResults) != 3 {
		t.Fatalf("s0 deliveries = %d", len(s0.newResults))
	}
	for index, result := range s0.newResults {
		if got := result.Get("position").Any(); got != []int{1, 0, 2}[index] {
			t.Fatalf("event %d position = %#v", index, got)
		}
		if got := result.Get("intarray").Any(); !reflect.DeepEqual(got, wantInts[index]) {
			t.Fatalf("event %d intarray = %#v, want %#v", index, got, wantInts[index])
		}
		if got := result.Get("objectarray").Any(); !reflect.DeepEqual(got, wantObjects[index]) {
			t.Fatalf("event %d objectarray = %#v, want %#v", index, got, wantObjects[index])
		}
	}
}

// TestUpdateIStreamArrayElementBoxedParity mirrors EPLOtherUpdateArrayElementBoxed:
// the index is an arithmetic expression (3-2) and the value widens and reboxes
// into a boxed double array element ([]*float64).
func TestUpdateIStreamArrayElementBoxedParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "MyEventBoxed", []FieldSpec{
		FieldDef("dbls", reflect.TypeOf([]*float64{})),
	}); err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(FromAny(env, "MyEventBoxed").UpdateStream(
		SetArrayElement("dbls", Subtract[int](Literal(3), Literal(2)), Literal(1)),
	).Query(StatementName("update")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(FromAny(env, "MyEventBoxed").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	updateIStreamDeployOne(t, engine, updatePlan)
	_, s0 := updateIStreamDeployOne(t, engine, s0Plan)

	if err := engine.Send(context.Background(), "MyEventBoxed", map[string]any{"dbls": make([]*float64, 3)}); err != nil {
		t.Fatal(err)
	}
	if len(s0.newResults) != 1 {
		t.Fatalf("s0 deliveries = %d", len(s0.newResults))
	}
	dbls, ok := s0.newResults[0].Get("dbls").Any().([]*float64)
	if !ok || len(dbls) != 3 {
		t.Fatalf("dbls = %#v, want []*float64 length 3", s0.newResults[0].Get("dbls").Any())
	}
	if dbls[0] != nil || dbls[2] != nil || dbls[1] == nil || *dbls[1] != 1.0 {
		t.Fatalf("dbls = %#v, want {nil, 1, nil}", dbls)
	}
}

// TestUpdateIStreamArrayElementInvalidParity mirrors EPLOtherUpdateArrayElementInvalid:
// the build-time invalid matrix (unknown property, null/non-integer index
// expression, incompatible value type, not-an-array target, unknown index
// field) plus the runtime behaviors (index overflow fails the send with
// Esper's diagnostic text, null index and null rhs for a primitive array skip
// the write silently). Esper's exact compile-time message texts differ from
// Go's typed-builder diagnostics; both reject the same statement shapes.
func TestUpdateIStreamArrayElementInvalidParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "MySchemaArr", []FieldSpec{
		FieldDef("doublearray", reflect.TypeOf([]float64{})),
		FieldDef("intarray", reflect.TypeOf([]int{})),
		FieldDef("notAnArray", reflect.TypeOf(int(0))),
	}); err != nil {
		t.Fatal(err)
	}
	assertInvalid := func(label, want string, build func() (Plan, error)) {
		t.Helper()
		if _, err := build(); err == nil {
			t.Fatalf("%s must be rejected", label)
		} else if want != "" && !strings.Contains(err.Error(), want) {
			t.Fatalf("%s error = %v, want substring %q", label, err, want)
		}
	}
	assertInvalid("unknown-property", "", func() (Plan, error) {
		return env.Build(FromAny(env, "MySchemaArr").UpdateStream(
			SetArrayElement("c1", Literal(0), Literal(1)),
		).Query(StatementName("s0")))
	})
	assertInvalid("null-index", "must return an integer", func() (Plan, error) {
		return env.Build(FromAny(env, "MySchemaArr").UpdateStream(
			SetArrayElement("doublearray", NullLiteral[any](), Literal(1)),
		).Query(StatementName("s0")))
	})
	assertInvalid("incompatible-value", "incompatible", func() (Plan, error) {
		return env.Build(FromAny(env, "MySchemaArr").UpdateStream(
			SetArrayElement("intarray", Field[Event, int]("notAnArray"), Literal("x")),
		).Query(StatementName("s0")))
	})
	assertInvalid("not-an-array", "is not an array", func() (Plan, error) {
		return env.Build(FromAny(env, "MySchemaArr").UpdateStream(
			SetArrayElement("notAnArray", Field[Event, int]("notAnArray"), Literal(1)),
		).Query(StatementName("s0")))
	})
	assertInvalid("unknown-index-field", "", func() (Plan, error) {
		return env.Build(FromAny(env, "MySchemaArr").UpdateStream(
			SetArrayElement("doublearray", Field[Event, int]("intPrimitive"), Literal(1)),
		).Query(StatementName("s0")))
	})

	// Runtime behaviors: overflow errors; null index and null rhs skip.
	if _, err := RegisterMap(env, "MySchemaRt", []FieldSpec{
		FieldDef("doublearray", reflect.TypeOf([]float64{})),
		FieldDef("indexvalue", reflect.TypeOf(int(0))),
		FieldDef("rhsvalue", reflect.TypeOf(int(0))),
	}); err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(FromAny(env, "MySchemaRt").UpdateStream(
		SetArrayElement("doublearray", Field[Event, int]("indexvalue"), Field[Event, int]("rhsvalue")),
	).Query(StatementName("update")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	updateIStreamDeployOne(t, engine, updatePlan)

	err = engine.Send(context.Background(), "MySchemaRt", map[string]any{"doublearray": make([]float64, 3), "indexvalue": 10, "rhsvalue": 1})
	if err == nil || !strings.Contains(err.Error(), "Array length 3 less than index 10 for property 'doublearray'") {
		t.Fatalf("index-overflow error = %v, want Esper diagnostic", err)
	}
	if err := engine.Send(context.Background(), "MySchemaRt", map[string]any{"doublearray": make([]float64, 3), "indexvalue": nil, "rhsvalue": 1}); err != nil {
		t.Fatalf("null-index send must succeed: %v", err)
	}
	if err := engine.Send(context.Background(), "MySchemaRt", map[string]any{"doublearray": make([]float64, 3), "indexvalue": 1, "rhsvalue": nil}); err != nil {
		t.Fatalf("null-rhs send must succeed: %v", err)
	}
}

// updateIStreamCopyMethodBean mirrors SupportBeanCopyMethod (valOne, valTwo).
// Esper needs a user copy method or serializer for bean copy-on-write; Go
// structs always copy field-wise so no hook is required.
type updateIStreamCopyMethodBean struct {
	ValOne string `esper:"valOne"`
	ValTwo string `esper:"valTwo"`
}

// TestUpdateIStreamSODAParity covers the runtime shape of EPLOtherUpdateSODA:
// update istream on a map type with a where clause. The SODA object-model
// surface (EPStatementObjectModel, toEPL, eplToModel round-trip) and the
// update's optional stream alias (as mytype) have no Go counterpart in the
// immutable typed chain plans (approved difference).
func TestUpdateIStreamSODAParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "MyMapTypeSODA", []FieldSpec{
		FieldDef("p0", reflect.TypeOf("")),
		FieldDef("p1", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(FromAny(env, "MyMapTypeSODA").UpdateStream(
		SetColumn("p1", Literal("newvalue")),
	).Where(Equal[string](Field[Event, string]("p0"), Literal("E1"))).Query(StatementName("update")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(FromAny(env, "MyMapTypeSODA").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	updateIStreamDeployOne(t, engine, updatePlan)
	_, s0 := updateIStreamDeployOne(t, engine, s0Plan)

	if err := engine.Send(context.Background(), "MyMapTypeSODA", map[string]any{"p0": "E1", "p1": "E1"}); err != nil {
		t.Fatal(err)
	}
	if len(s0.newResults) != 1 {
		t.Fatalf("s0 deliveries = %d", len(s0.newResults))
	}
	if got := s0.newResults[0].Get("p0").Any(); got != "E1" {
		t.Fatalf("s0 p0 = %#v, want E1", got)
	}
	if got := s0.newResults[0].Get("p1").Any(); got != "newvalue" {
		t.Fatalf("s0 p1 = %#v, want newvalue", got)
	}
}

// TestUpdateIStreamXMLEventParity mirrors EPLOtherUpdateXMLEvent on the map
// representation: insert-into adds two literal columns alongside the source
// property, and the update rewrites both literal columns where the source
// property matches. The XML DOM event representation itself has no Go
// counterpart (approved difference).
func TestUpdateIStreamXMLEventParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "MyXMLEvent", []FieldSpec{
		FieldDef("prop1", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "ABCStreamXML", []FieldSpec{
		FieldDef("valOne", reflect.TypeOf(int(0))),
		FieldDef("valTwo", reflect.TypeOf(int(0))),
		FieldDef("prop1", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(FromAny(env, "MyXMLEvent").Select(
		Alias("valOne", Literal(1)),
		Alias("valTwo", Literal(2)),
		Alias("prop1", Field[Event, string]("prop1")),
	).InsertInto("ABCStreamXML", StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(FromAny(env, "ABCStreamXML").UpdateStream(
		SetColumn("valOne", Literal(987)),
		SetColumn("valTwo", Literal(123)),
	).Where(Equal[string](Field[Event, string]("prop1"), Literal("SAMPLE_V1"))).Query(StatementName("update")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(FromAny(env, "ABCStreamXML").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	updateIStreamDeployOne(t, engine, insertPlan)
	updateIStreamDeployOne(t, engine, updatePlan)
	_, s0 := updateIStreamDeployOne(t, engine, s0Plan)

	if err := engine.Send(context.Background(), "MyXMLEvent", map[string]any{"prop1": "SAMPLE_V1"}); err != nil {
		t.Fatal(err)
	}
	if len(s0.newResults) != 1 {
		t.Fatalf("s0 deliveries = %d", len(s0.newResults))
	}
	if got := s0.newResults[0].Get("valOne").Any(); got != 987 {
		t.Fatalf("s0 valOne = %#v, want 987", got)
	}
	if got := s0.newResults[0].Get("valTwo").Any(); got != 123 {
		t.Fatalf("s0 valTwo = %#v, want 123", got)
	}
	if got := s0.newResults[0].Get("prop1").Any(); got != "SAMPLE_V1" {
		t.Fatalf("s0 prop1 = %#v, want SAMPLE_V1", got)
	}
}

// TestUpdateIStreamWrappedObjectParity mirrors EPLOtherUpdateWrappedObject on
// the flat map representation: insert-into projects two literal columns plus
// the source property; three update shapes (extra columns only, source
// property only, mixed) apply across undeploy/redeploy boundaries. Esper's
// wrapped-object representation (the bean as a fragment next to extra
// columns) is represented by the equivalent flat implicit stream in Go
// (approved difference).
func TestUpdateIStreamWrappedObjectParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[updateIStreamBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "ABCStreamWO", []FieldSpec{
		FieldDef("valOne", reflect.TypeOf(int(0))),
		FieldDef("valTwo", reflect.TypeOf(int(0))),
		FieldDef("theString", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(Select(
		From[updateIStreamBean](env, "SupportBean"),
		Alias("valOne", Literal(1)),
		Alias("valTwo", Literal(2)),
		Alias("theString", Field[updateIStreamBean, string]("theString")),
	).InsertInto("ABCStreamWO", StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(FromAny(env, "ABCStreamWO").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	update := func(name string, assignments ...TableAssignment) Plan {
		t.Helper()
		plan, err := env.Build(FromAny(env, "ABCStreamWO").UpdateStream(assignments...).Query(StatementName(name)))
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}

	engine := NewEngine(env)
	updateIStreamDeployOne(t, engine, insertPlan)
	_, s0 := updateIStreamDeployOne(t, engine, s0Plan)

	send := func(theString string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), updateIStreamBean{TheString: theString}); err != nil {
			t.Fatal(err)
		}
	}
	assertRow := func(label string, result Result, valOne, valTwo int, theString string) {
		t.Helper()
		if got := result.Get("valOne").Any(); got != valOne {
			t.Fatalf("%s valOne = %#v, want %d", label, got, valOne)
		}
		if got := result.Get("valTwo").Any(); got != valTwo {
			t.Fatalf("%s valTwo = %#v, want %d", label, got, valTwo)
		}
		if got := result.Get("theString").Any(); got != theString {
			t.Fatalf("%s theString = %#v, want %#v", label, got, theString)
		}
	}

	oneDeployment, _ := updateIStreamDeployOne(t, engine, update("update1", SetColumn("valOne", Literal(987)), SetColumn("valTwo", Literal(123))))
	send("E1")
	assertRow("E1", s0.newResults[0], 987, 123, "E1")

	if err := oneDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	twoDeployment, _ := updateIStreamDeployOne(t, engine, update("update2", SetColumn("theString", Literal("A"))))
	send("E2")
	assertRow("E2", s0.newResults[1], 1, 2, "A")

	if err := twoDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	updateIStreamDeployOne(t, engine, update("update3", SetColumn("theString", Literal("B")), SetColumn("valOne", Literal(555))))
	send("E3")
	assertRow("E3", s0.newResults[2], 555, 2, "B")
}

// TestUpdateIStreamCopyMethodParity covers the runtime shape of
// EPLOtherUpdateCopyMethod: insert-into wildcard route plus an update writing
// both properties. Esper requires a user copy method on the bean for the
// copy-on-write; Go structs always copy field-wise so no hook is needed
// (approved difference).
func TestUpdateIStreamCopyMethodParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[updateIStreamCopyMethodBean](env, "SupportBeanCopyMethod"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[updateIStreamCopyMethodBean](env, "ABCStreamCM"); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(FromAny(env, "SupportBeanCopyMethod").InsertInto("ABCStreamCM", StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(FromAny(env, "ABCStreamCM").UpdateStream(
		SetColumn("valOne", Literal("x")),
		SetColumn("valTwo", Literal("y")),
	).Query(StatementName("update")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(FromAny(env, "ABCStreamCM").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	updateIStreamDeployOne(t, engine, insertPlan)
	updateIStreamDeployOne(t, engine, updatePlan)
	_, s0 := updateIStreamDeployOne(t, engine, s0Plan)

	if err := engine.SendEvent(context.Background(), updateIStreamCopyMethodBean{ValOne: "1", ValTwo: "2"}); err != nil {
		t.Fatal(err)
	}
	if len(s0.newResults) != 1 {
		t.Fatalf("s0 deliveries = %d", len(s0.newResults))
	}
	if got := s0.newResults[0].Get("valOne").Any(); got != "x" {
		t.Fatalf("s0 valOne = %#v, want x", got)
	}
	if got := s0.newResults[0].Get("valTwo").Any(); got != "y" {
		t.Fatalf("s0 valTwo = %#v, want y", got)
	}
}

// TestUpdateIStreamExpressionParity documents the Go-native equivalent of
// EPLOtherUpdateExpression: Esper uses an inlined_class Helper.swap(me) that
// mutates the map event in place; in Go the same observable swap falls out of
// the all-set-expressions-evaluate-pre-update rule with two plain column
// assignments. The inlined_class mechanism and set-with-method-call form have
// no Go counterpart (approved difference).
func TestUpdateIStreamExpressionParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "MyEventSwap", []FieldSpec{
		FieldDef("a", reflect.TypeOf(int(0))),
		FieldDef("b", reflect.TypeOf(int(0))),
	}); err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(FromAny(env, "MyEventSwap").UpdateStream(
		SetColumn("a", Field[Event, int]("b")),
		SetColumn("b", Field[Event, int]("a")),
	).Query(StatementName("update")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(FromAny(env, "MyEventSwap").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	updateIStreamDeployOne(t, engine, updatePlan)
	_, s0 := updateIStreamDeployOne(t, engine, s0Plan)

	if err := engine.Send(context.Background(), "MyEventSwap", map[string]any{"a": 1, "b": 10}); err != nil {
		t.Fatal(err)
	}
	if len(s0.newResults) != 1 {
		t.Fatalf("s0 deliveries = %d", len(s0.newResults))
	}
	if got := s0.newResults[0].Get("a").Any(); got != 10 {
		t.Fatalf("s0 a = %#v, want 10 (swapped)", got)
	}
	if got := s0.newResults[0].Get("b").Any(); got != 1 {
		t.Fatalf("s0 b = %#v, want 1 (swapped)", got)
	}
}

// TestUpdateIStreamSubqueryParity mirrors EPLOtherUpdateSubquery: subqueries
// in update-set and update-where. The uncorrelated part sets theString from a
// last-event scalar subquery gated by an IN-subquery where clause (empty
// subquery assigns null to the nullable property); the correlated part reads
// the pre-update event via OuterField (null subquery result skips the write
// on the non-nullable int property). Esper's optional stream alias
// (as mystream) has no Go counterpart; the correlated redeploy boundary is
// mirrored with a fresh identical plan.
func TestUpdateIStreamSubqueryParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[updateIStreamBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "MyMapTypeSelect", []FieldSpec{
		FieldDef("s0", reflect.TypeOf("")),
		FieldDef("s1", reflect.TypeOf(int(0))),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "MyMapTypeWhere", []FieldSpec{
		FieldDef("w0", reflect.TypeOf(int(0))),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "ABCStreamSQ", []FieldSpec{
		FieldDef("theString", reflect.TypeOf((*any)(nil)).Elem()),
		FieldDef("intPrimitive", reflect.TypeOf(int(0))),
	}); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(Select(
		From[updateIStreamBean](env, "SupportBean"),
		Alias("theString", Field[updateIStreamBean, string]("theString")),
		Alias("intPrimitive", Field[updateIStreamBean, int]("intPrimitive")),
	).InsertInto("ABCStreamSQ", StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(FromAny(env, "ABCStreamSQ").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	update1Plan, err := env.Build(FromAny(env, "ABCStreamSQ").UpdateStream(
		SetColumn("theString", SubqueryValue[string](
			FromAny(env, "MyMapTypeSelect").Window(LastEvent()),
			Field[Event, string]("s0"),
		)),
	).Where(SubqueryIn[int](
		Field[Event, int]("intPrimitive"),
		FromAny(env, "MyMapTypeWhere").Window(KeepAll()),
		Field[Event, int]("w0"),
	)).Query(StatementName("update")))
	if err != nil {
		t.Fatal(err)
	}
	correlated := func(name string) Plan {
		t.Helper()
		plan, err := env.Build(FromAny(env, "ABCStreamSQ").UpdateStream(
			SetColumn("intPrimitive", SubqueryValue[int](
				FromAny(env, "MyMapTypeSelect").Window(KeepAll()),
				Field[Event, int]("s1"),
				Equal[string](Field[Event, string]("s0"), OuterField[string]("theString")),
			)),
		).Query(StatementName(name)))
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}

	engine := NewEngine(env)
	updateIStreamDeployOne(t, engine, insertPlan)
	update1Deployment, _ := updateIStreamDeployOne(t, engine, update1Plan)
	_, s0 := updateIStreamDeployOne(t, engine, s0Plan)

	send := func(theString string, intPrimitive int) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean", updateIStreamBean{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
	}
	sendMap := func(eventType string, event map[string]any) {
		t.Helper()
		if err := engine.Send(context.Background(), eventType, event); err != nil {
			t.Fatal(err)
		}
	}
	assertRow := func(label string, index int, theString any, intPrimitive int) {
		t.Helper()
		if len(s0.newResults) <= index {
			t.Fatalf("%s: s0 deliveries = %d, want at least %d", label, len(s0.newResults), index+1)
		}
		result := s0.newResults[index]
		if got := result.Get("theString").Any(); got != theString {
			t.Fatalf("%s theString = %#v, want %#v", label, got, theString)
		}
		if got := result.Get("intPrimitive").Any(); got != intPrimitive {
			t.Fatalf("%s intPrimitive = %#v, want %d", label, got, intPrimitive)
		}
	}

	send("E1", 0)
	assertRow("E1", 0, "E1", 0)

	sendMap("MyMapTypeWhere", map[string]any{"w0": 1})
	send("E2", 1)
	assertRow("E2 (empty subquery assigns null)", 1, nil, 1)

	send("E3", 2)
	assertRow("E3", 2, "E3", 2)

	sendMap("MyMapTypeSelect", map[string]any{"s0": "newvalue"})
	send("E4", 1)
	assertRow("E4", 3, "newvalue", 1)

	sendMap("MyMapTypeSelect", map[string]any{"s0": "othervalue"})
	send("E5", 1)
	assertRow("E5", 4, "othervalue", 1)

	// Correlated subquery: null result skips the write on the primitive int.
	if err := update1Deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	update2Deployment, _ := updateIStreamDeployOne(t, engine, correlated("update2"))
	send("E6", 8)
	assertRow("E6 (null subquery skips)", 5, "E6", 8)

	sendMap("MyMapTypeSelect", map[string]any{"s0": "E7", "s1": 91})
	send("E7", 0)
	assertRow("E7", 6, "E7", 91)

	// Esper's as-clause variant correlates through the stream alias; Go
	// redeploys an identical plan to mirror the boundary.
	if err := update2Deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	updateIStreamDeployOne(t, engine, correlated("update3"))
	send("E8", 111)
	assertRow("E8 (null subquery skips)", 7, "E8", 111)

	sendMap("MyMapTypeSelect", map[string]any{"s0": "E9", "s1": -1})
	send("E9", 0)
	assertRow("E9", 8, "E9", -1)
}

// TestUpdateIStreamSubqueryMultikeyWArrayParity mirrors
// EPLOtherUpdateSubqueryMultikeyWArray: a scalar grouped subquery with an
// array (multikey) group key assigns the single group's aggregate, and a
// multi-group result assigns null (Esper's scalar grouped subselect
// semantics, surfaced in Go through SubqueryGroupScalar).
func TestUpdateIStreamSubqueryMultikeyWArrayParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "SupportEventWithIntArray", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
		FieldDef("array", reflect.TypeOf([]int{})),
		FieldDef("value", reflect.TypeOf(int(0))),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "Arriving", []FieldSpec{
		FieldDef("value", reflect.TypeOf((*any)(nil)).Elem()),
	}); err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(FromAny(env, "Arriving").UpdateStream(
		SetColumn("value", SubqueryGroupScalar[[]int, int](
			FromAny(env, "SupportEventWithIntArray").Window(KeepAll()),
			Field[Event, []int]("array"),
			Sum[int](Field[Event, int]("value")),
		)),
	).Query(StatementName("update")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(FromAny(env, "Arriving").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	updateIStreamDeployOne(t, engine, updatePlan)
	_, s0 := updateIStreamDeployOne(t, engine, s0Plan)

	sendSWIA := func(id string, array []int, value int) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportEventWithIntArray", map[string]any{"id": id, "array": array, "value": value}); err != nil {
			t.Fatal(err)
		}
	}
	trigger := func(label string, want any) {
		t.Helper()
		before := len(s0.newResults)
		if err := engine.Send(context.Background(), "Arriving", map[string]any{}); err != nil {
			t.Fatal(err)
		}
		if len(s0.newResults) != before+1 {
			t.Fatalf("%s: s0 deliveries = %d, want %d", label, len(s0.newResults), before+1)
		}
		if got := s0.newResults[before].Get("value").Any(); got != want {
			t.Fatalf("%s value = %#v, want %#v", label, got, want)
		}
	}

	sendSWIA("E1", []int{1, 2}, 10)
	sendSWIA("E2", []int{1, 2}, 11)
	trigger("single group {1,2}", 21)

	sendSWIA("E3", []int{1, 2}, 12)
	trigger("single group {1,2} after E3", 33)

	sendSWIA("E4", []int{1}, 13)
	trigger("two groups assign null", nil)
}
