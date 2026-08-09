package esper

import (
	"context"
	"testing"
)

// updateIStreamBean mirrors SupportBean (theString, intPrimitive, intBoxed)
// for the update-istream parity suite.
type updateIStreamBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     int    `esper:"intBoxed"`
}

type updateIStreamCapture struct {
	newEvents []Event
	oldEvents []Event
}

func (c *updateIStreamCapture) listener() Listener {
	return func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				c.newEvents = append(c.newEvents, event)
			}
		}
		for _, result := range batch.Old {
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

	if err := engine.Send(context.Background(), "SupportBean", updateIStreamBean{TheString: "E1", IntPrimitive: 1, IntBoxed: 2}); err != nil {
		t.Fatal(err)
	}
	if len(captures[0].newEvents) != 1 || len(captures[1].newEvents) != 1 {
		t.Fatalf("deliveries: update=%d select=%d", len(captures[0].newEvents), len(captures[1].newEvents))
	}
	updated := captures[1].newEvents[0]
	assertUpdateIStreamEvent(t, "select E1", updated, "E1", 10)
	if got := updated.Get("intBoxed").Any(); got != 1 {
		t.Fatalf("intBoxed = %#v, want 1 (pre-update value)", got)
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
