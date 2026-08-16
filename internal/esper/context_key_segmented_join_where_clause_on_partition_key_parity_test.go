package esper

import (
	"context"
	"testing"
)

// TestKeyContextJoinWhereClauseOnPartitionKeyParity locks the join where
// clause over a partition-key condition verified against
// ContextKeySegmentedJoinWhereClauseOnPartitionKey: `partition by theString
// from SupportBean` joining SupportBean#lastevent with SupportBean_S0#
// lastevent filtered by `theString is 'Test'`. The partner event has no
// partition key and fans out to every partition's join state; only the
// Test partition's join result passes the where clause.
func TestKeyContextJoinWhereClauseOnPartitionKeyParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegJoinBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[keySegJoinS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	sb := From[keySegJoinBean](env, "SupportBean")
	s0 := From[keySegJoinS0](env, "SupportBean_S0")
	theString := Field[keySegJoinBean, string]("theString")
	if _, err := CreateKeyContext(env, "MyCtx", theString); err != nil {
		t.Fatal(err)
	}
	query := Join(
		sb.Window(LastEvent()),
		s0.Window(LastEvent()),
	).Select(
		SelectFrom(0, "sb.theString", JoinField[string](0, "theString")),
		SelectFrom(0, "sb.intPrimitive", JoinField[int](0, "intPrimitive")),
		SelectFrom(1, "s0.id", JoinField[int](1, "id")),
		SelectFrom(1, "s0.p00", JoinField[string](1, "p00")),
	).Where(Equal[string](JoinField[string](0, "theString"), Literal("Test"))).
		Query(StatementName("select"), WithContext("MyCtx"))
	plan, err := env.Build(query)
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
	type row struct {
		sb, p00 string
		sv, id  int
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				rows = append(rows, row{
					sb:  rowValue.Get("sb.theString").Any().(string),
					sv:  rowValue.Get("sb.intPrimitive").Any().(int),
					id:  rowValue.Get("s0.id").Any().(int),
					p00: rowValue.Get("s0.p00").Any().(string),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendSB := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegJoinBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	sendS0 := func(id int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegJoinS0{ID: id, P00: "S0"}); err != nil {
			t.Fatal(err)
		}
	}
	sendSB("Test", 10)
	sendSB("E2", 20)
	if len(rows) != 0 {
		t.Fatalf("before partner rows = %#v, want none", rows)
	}
	sendS0(1)
	want := []row{{sb: "Test", sv: 10, id: 1, p00: "S0"}}
	if len(rows) != len(want) {
		t.Fatalf("rows = %#v, want %#v", rows, want)
	}
	for index := range want {
		if rows[index] != want[index] {
			t.Fatalf("rows[%d] = %#v, want %#v", index, rows[index], want[index])
		}
	}
}
