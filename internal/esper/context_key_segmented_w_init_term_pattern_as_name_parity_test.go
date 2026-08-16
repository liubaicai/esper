package esper

import (
	"context"
	"testing"
)

type keySegWInitTermPatternBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     int    `esper:"intBoxed"`
}

// TestKeyContextWInitTermPatternAsNameProjectionParity locks the named
// pattern-end projection of a keyed initiated-terminated context verified
// against ContextKeySegmentedWInitTermPatternAsName: `partition by
// theString initiated by SupportBean(intPrimitive = 1) as startevent
// terminated by pattern[s=SupportBean(intPrimitive = 2)] as endpattern`
// with `select context.startevent.intBoxed as c0, context.endpattern.s.
// intBoxed as c1 from SupportBean#firstevent output snapshot when
// terminated`. The snapshot reads the initiating event (firstevent window
// holds it) and the end-pattern match's tagged event; mid events produce
// no output.
func TestKeyContextWInitTermPatternAsNameProjectionParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegWInitTermPatternBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	source := From[keySegWInitTermPatternBean](env, "SupportBean")
	theString := Field[keySegWInitTermPatternBean, string]("theString")
	intPrimitive := Field[keySegWInitTermPatternBean, int]("intPrimitive")
	end := PatternFrom(source, "s", Equal[int](intPrimitive, Literal(2)))
	if _, err := CreatePatternTerminatedContext(env, "MyContext", theString,
		Equal[int](intPrimitive, Literal(1)), end); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(source.Window(FirstEvent()),
		Alias("c0", Property[int](ContextInitiatingEvent(), "intBoxed")),
		Alias("c1", ContextPatternField[int]("s", "intBoxed")),
	).Query(StatementName("s0"), WithContext("MyContext"), WithOutput(OutputSnapshotWhenTerminated())))
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
	send := func(s string, intPrimitive, intBoxed int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegWInitTermPatternBean{TheString: s, IntPrimitive: intPrimitive, IntBoxed: intBoxed}); err != nil {
			t.Fatal(err)
		}
	}
	send("A", 1, 10)
	send("A", 0, 99) // mid event: no output
	if len(rows) != 0 {
		t.Fatalf("before termination rows = %d, want 0", len(rows))
	}
	send("A", 2, 20)
	if len(rows) != 1 {
		t.Fatalf("after A termination rows = %d, want 1", len(rows))
	}
	if c0 := rows[0].Get("c0").Any(); c0 != 10 {
		t.Fatalf("c0 = %#v, want 10 (initiating event's intBoxed)", c0)
	}
	if c1 := rows[0].Get("c1").Any(); c1 != 20 {
		t.Fatalf("c1 = %#v, want 20 (end-pattern match's intBoxed)", c1)
	}
	send("B", 1, 10)
	send("B", 2, 20)
	if len(rows) != 2 {
		t.Fatalf("after B termination rows = %d, want 2", len(rows))
	}
	if c0 := rows[1].Get("c0").Any(); c0 != 10 {
		t.Fatalf("B c0 = %#v, want 10", c0)
	}
	if c1 := rows[1].Get("c1").Any(); c1 != 20 {
		t.Fatalf("B c1 = %#v, want 20", c1)
	}
}
