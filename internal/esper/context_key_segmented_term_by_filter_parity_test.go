package esper

import (
	"context"
	"testing"
)

type keySegTermFilterBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestKeyContextTermByFilterRestartsPartitionParity locks the filter
// termination of a keyed context verified against
// ContextKeySegmentedTermByFilter: `partition by theString terminated by
// SupportBean(intPrimitive<0)` counts non-negative events per partition,
// terminates on the negative event, and restarts at 1 on the next
// non-negative event for the same key. A negative event for an absent key
// produces nothing.
func TestKeyContextTermByFilterRestartsPartitionParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegTermFilterBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	source := From[keySegTermFilterBean](env, "SupportBean")
	theString := Field[keySegTermFilterBean, string]("theString")
	intPrimitive := Field[keySegTermFilterBean, int]("intPrimitive")
	isBean := Equal[string](TypeName(EventValue[Event]()), Literal("SupportBean"))
	if _, err := CreateInitiatedTerminatedContext(env, "ByP0", theString,
		isBean,
		LessOf(intPrimitive, Literal(0))); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(source.Filter(GreaterOrEqual[int](intPrimitive, Literal(0))).Aggregate(
		Alias("theString", theString),
		Alias("cnt", CountAll()),
	).Query(StatementName("s0"), WithContext("ByP0")))
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
		if err := engine.SendEvent(context.Background(), keySegTermFilterBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	send("A", 0)
	if len(rows) != 1 || rows[0].Get("cnt").Any() != int64(1) {
		t.Fatalf("after A/0 rows = %#v", rows)
	}
	send("A", 0)
	if len(rows) != 2 || rows[1].Get("cnt").Any() != int64(2) {
		t.Fatalf("after A/0x2 rows = %#v", rows)
	}
	send("A", -1) // terminates A partition, no output
	if len(rows) != 2 {
		t.Fatalf("after A/-1 rows = %d, want 2 (no output)", len(rows))
	}
	send("A", 0) // restart at 1
	if len(rows) != 3 || rows[2].Get("cnt").Any() != int64(1) {
		t.Fatalf("after A/0 restart rows = %#v", rows)
	}
	send("B", 0)
	if len(rows) != 4 || rows[3].Get("cnt").Any() != int64(1) {
		t.Fatalf("after B/0 rows = %#v", rows)
	}
	send("B", -1)
	send("B", 0)
	send("B", 0)
	if len(rows) != 6 || rows[5].Get("cnt").Any() != int64(2) {
		t.Fatalf("after B cycle rows = %#v", rows)
	}
	send("B", -1)
	send("B", 0)
	if len(rows) != 7 || rows[6].Get("cnt").Any() != int64(1) {
		t.Fatalf("after B restart rows = %#v", rows)
	}
	send("C", -1) // absent key negative: nothing
	if len(rows) != 7 {
		t.Fatalf("after C/-1 rows = %d, want 7", len(rows))
	}
}
