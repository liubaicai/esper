package esper

import (
	"context"
	"testing"
)

type keySegWInitTermEndBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestKeyContextWInitTermEndEventProjectionParity locks the boundary-event
// projection of a keyed initiated-terminated context verified against
// ContextKeySegmentedWInitTermEndEvent: `partition by theString initiated by
// SupportBean(intPrimitive = 1) as startevent terminated by
// SupportBean(intPrimitive = 0) as endevent` with `select
// context.startevent as c0, context.endevent as c1 from SupportBean output
// all when terminated`. The terminating event closes the partition without
// entering the statement, and the buffered rows are projected at termination
// so every row sees the terminating event in c1.
func TestKeyContextWInitTermEndEventProjectionParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegWInitTermEndBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	source := From[keySegWInitTermEndBean](env, "SupportBean")
	theString := Field[keySegWInitTermEndBean, string]("theString")
	intPrimitive := Field[keySegWInitTermEndBean, int]("intPrimitive")
	if _, err := CreateInitiatedTerminatedContext(env, "MyContext", theString,
		Equal[int](intPrimitive, Literal(1)),
		Equal[int](intPrimitive, Literal(0))); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(source,
		Alias("c0", ContextInitiatingEvent()),
		Alias("c1", ContextTerminatingEvent()),
	).Query(StatementName("s0"), WithContext("MyContext"), WithOutput(OutputWhenTerminated(OutputAll()))))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var rows []Row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegWInitTermEndBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	startValue := func(row Row) int {
		event, ok := row.Get("c0").Any().(Event)
		if !ok {
			t.Fatalf("c0 not an event: %#v", row.Get("c0").Any())
		}
		return event.Get("intPrimitive").Any().(int)
	}
	endValue := func(row Row) int {
		event, ok := row.Get("c1").Any().(Event)
		if !ok {
			t.Fatalf("c1 not an event: %#v", row.Get("c1").Any())
		}
		return event.Get("intPrimitive").Any().(int)
	}

	send("A", 1)
	if len(rows) != 0 {
		t.Fatalf("after A/1 rows = %#v, want none", rows)
	}
	send("A", 2) // mid event: buffered, not emitted
	if len(rows) != 0 {
		t.Fatalf("after A/2 rows = %#v, want none", rows)
	}
	send("A", 0) // terminate: flush both buffered rows, both see c1
	if len(rows) != 2 {
		t.Fatalf("after A/0 rows = %d, want 2", len(rows))
	}
	// c0/c1 are partition properties: every buffered row sees the same
	// initiating event (A/1) and the terminating event (A/0). Java
	// projects the batch at termination, so c1 is populated on both rows.
	for index, row := range rows {
		if startValue(row) != 1 || endValue(row) != 0 {
			t.Fatalf("row%d c0/c1 = %d/%d, want 1/0", index, startValue(row), endValue(row))
		}
	}
	send("B", 1)
	send("B", 0)
	if len(rows) != 3 {
		t.Fatalf("after B cycle rows = %d, want 3", len(rows))
	}
	if startValue(rows[2]) != 1 || endValue(rows[2]) != 0 {
		t.Fatalf("row2 c0/c1 = %d/%d, want 1/0", startValue(rows[2]), endValue(rows[2]))
	}
}
