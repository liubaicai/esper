package esper

import (
	"context"
	"testing"
)

// TestKeyContextJoinMultitypeMultifieldParity locks the multi-type
// multi-field segmented context join verified against
// ContextKeySegmentedJoinMultitypeMultifield: `partition by theString and
// intPrimitive from SupportBean, p00 and id from SupportBean_S0` with a
// cross join of the per-partition lastevent windows projecting both key
// fields through context.key1/context.key2. A partition joins only once
// both sides have fed it; each completed partition pairs its own lastevent
// windows and exposes its own keys.
func TestKeyContextJoinMultitypeMultifieldParity(t *testing.T) {
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
	intPrimitive := Field[keySegJoinBean, int]("intPrimitive")
	p00 := Field[keySegJoinS0, string]("p00")
	id := Field[keySegJoinS0, int]("id")
	if _, err := CreateKeyContextByStreams(env, "SegmentedBy2Fields",
		KeyContextStream{Type: "SupportBean", Keys: []Expr{theString, intPrimitive}},
		KeyContextStream{Type: "SupportBean_S0", Keys: []Expr{p00, id}},
	); err != nil {
		t.Fatal(err)
	}
	query := Join(
		sb.Window(LastEvent()),
		s0.Window(LastEvent()),
	).Select(
		SelectFrom(0, "c1", JoinField[string](0, "theString")),
		SelectFrom(0, "c2", JoinField[int](0, "intPrimitive")),
		SelectFrom(1, "c3", JoinField[int](1, "id")),
		SelectFrom(1, "c4", JoinField[string](1, "p00")),
		SelectFrom(0, "c5", ContextField[string]("key1")),
		SelectFrom(0, "c6", ContextField[int]("key2")),
	).Query(StatementName("s0"), WithContext("SegmentedBy2Fields"))
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
		c1, c4, c5 string
		c2, c3, c6 int
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				rows = append(rows, row{
					c1: rowValue.Get("c1").Any().(string),
					c2: rowValue.Get("c2").Any().(int),
					c3: rowValue.Get("c3").Any().(int),
					c4: rowValue.Get("c4").Any().(string),
					c5: rowValue.Get("c5").Any().(string),
					c6: rowValue.Get("c6").Any().(int),
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
	sendS0 := func(p00 string, id int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegJoinS0{ID: id, P00: p00}); err != nil {
			t.Fatal(err)
		}
	}
	sendSB("G1", 1)
	sendS0("G1", 2)
	sendSB("G2", 2)
	sendS0("G2", 1)
	if len(rows) != 0 {
		t.Fatalf("single-sided partitions rows = %#v, want none", rows)
	}
	sendSB("G2", 1)
	sendS0("G2", 2)
	sendS0("G1", 1)
	sendSB("G1", 2)
	want := []row{
		{c1: "G2", c2: 1, c3: 1, c4: "G2", c5: "G2", c6: 1},
		{c1: "G2", c2: 2, c3: 2, c4: "G2", c5: "G2", c6: 2},
		{c1: "G1", c2: 1, c3: 1, c4: "G1", c5: "G1", c6: 1},
		{c1: "G1", c2: 2, c3: 2, c4: "G1", c5: "G1", c6: 2},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %#v, want %#v", rows, want)
	}
	for index := range want {
		if rows[index] != want[index] {
			t.Fatalf("rows[%d] = %#v, want %#v", index, rows[index], want[index])
		}
	}
}
