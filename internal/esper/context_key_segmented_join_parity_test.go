package esper

import (
	"context"
	"testing"
)

type keySegJoinBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type keySegJoinS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// TestKeyContextJoinPartnerFanOutParity locks the join-partner fan-out of
// a keyed context verified against ContextKeySegmentedJoin: `partition by
// theString` joining SupportBean#keepall with SupportBean_S0#keepall on
// intPrimitive = id. The partner stream (SupportBean_S0) has no partition
// key, so its events are dispatched to every existing partition's join
// state; a partition created later (G3) never sees earlier partner events.
func TestKeyContextJoinPartnerFanOutParity(t *testing.T) {
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
	if _, err := CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		t.Fatal(err)
	}
	query := Join(
		sb.Window(KeepAll()),
		s0.Window(KeepAll()),
		OnSourcesEqual(
			0, Field[keySegJoinBean, int]("intPrimitive"),
			1, Field[keySegJoinS0, int]("id"),
		),
	).Select(
		SelectFrom(0, "sb.theString", JoinField[string](0, "theString")),
		SelectFrom(0, "sb.intPrimitive", JoinField[int](0, "intPrimitive")),
		SelectFrom(1, "s0.id", JoinField[int](1, "id")),
	).Query(StatementName("s0"), WithContext("SegmentedByString"))
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
		sb string
		sv int
		s0 int
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				rows = append(rows, row{
					sb: rowValue.Get("sb.theString").Any().(string),
					sv: rowValue.Get("sb.intPrimitive").Any().(int),
					s0: rowValue.Get("s0.id").Any().(int),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegJoinBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	sendS0 := func(i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegJoinS0{ID: i, P00: "S0"}); err != nil {
			t.Fatal(err)
		}
	}
	send("G1", 10)
	send("G2", 20)
	sendS0(20)
	if len(rows) != 1 || rows[0] != (row{"G2", 20, 20}) {
		t.Fatalf("after S0(20) rows = %#v, want [{G2 20 20}]", rows)
	}
	sendS0(30)
	send("G3", 30)
	if len(rows) != 1 {
		t.Fatalf("after G3 rows = %#v, want unchanged (late partition sees no early S0)", rows)
	}
	send("G1", 30)
	if len(rows) != 2 || rows[1] != (row{"G1", 30, 30}) {
		t.Fatalf("after G1/30 rows = %#v, want second {G1 30 30}", rows)
	}
	send("G2", 30)
	if len(rows) != 3 || rows[2] != (row{"G2", 30, 30}) {
		t.Fatalf("after G2/30 rows = %#v, want third {G2 30 30}", rows)
	}
}
