package esper

import (
	"context"
	"testing"
)

// TestKeyContextMultiStatementPerStreamSumsParity locks the multi-statement
// sharing of a multi-stream segmented context verified against
// ContextKeySegmentedMultiStatementFilterCount: `partition by theString
// from SupportBean, p00 from SupportBean_S0` with s0 summing SupportBean_S0
// ids per p00 partition and s1 summing SupportBean intPrimitive per
// theString partition. Each statement receives only its own stream's
// events, keeps per-partition state, and the two partition spaces are
// independent.
func TestKeyContextMultiStatementPerStreamSumsParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegJoinBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[keySegJoinS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	theString := Field[keySegJoinBean, string]("theString")
	p00 := Field[keySegJoinS0, string]("p00")
	if _, err := CreateKeyContextByStreams(env, "SegmentedByAString",
		KeyContextStream{Type: "SupportBean", Keys: []Expr{theString}},
		KeyContextStream{Type: "SupportBean_S0", Keys: []Expr{p00}},
	); err != nil {
		t.Fatal(err)
	}
	planS0, err := env.Build(From[keySegJoinS0](env, "SupportBean_S0").Aggregate(
		Alias("col1", Sum[int](Field[keySegJoinS0, int]("id"))),
	).Query(StatementName("s0"), WithContext("SegmentedByAString")))
	if err != nil {
		t.Fatal(err)
	}
	planSB, err := env.Build(From[keySegJoinBean](env, "SupportBean").Aggregate(
		Alias("col1", Sum[int](Field[keySegJoinBean, int]("intPrimitive"))),
	).Query(StatementName("s1"), WithContext("SegmentedByAString")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deploymentS0, err := engine.Deploy(context.Background(), planS0)
	if err != nil {
		t.Fatal(err)
	}
	deploymentSB, err := engine.Deploy(context.Background(), planSB)
	if err != nil {
		t.Fatal(err)
	}
	statementS0 := deploymentS0.Statements()[0]
	statementSB := deploymentSB.Statements()[0]
	type record struct {
		statement string
		sum       int
	}
	var records []record
	subscribe := func(statement *Statement, name string) {
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				if row, ok := result.Row(); ok {
					records = append(records, record{statement: name, sum: row.Get("col1").Any().(int)})
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	subscribe(statementS0, "s0")
	subscribe(statementSB, "s1")
	sendS0 := func(p00 string, id int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegJoinS0{ID: id, P00: p00}); err != nil {
			t.Fatal(err)
		}
	}
	sendSB := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegJoinBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	sendS0("S0", 10)
	sendS0("S1", 8)
	sendS0("S0", 4)
	sendSB("S0", 5)
	sendSB("S2", 6)
	sendS0("S0", 7)
	sendSB("S0", 9)
	want := []record{
		{statement: "s0", sum: 10},
		{statement: "s0", sum: 8},
		{statement: "s0", sum: 14},
		{statement: "s1", sum: 5},
		{statement: "s1", sum: 6},
		{statement: "s0", sum: 21},
		{statement: "s1", sum: 14},
	}
	if len(records) != len(want) {
		t.Fatalf("records = %#v, want %#v", records, want)
	}
	for index := range want {
		if records[index] != want[index] {
			t.Fatalf("records[%d] = %#v, want %#v", index, records[index], want[index])
		}
	}
}
