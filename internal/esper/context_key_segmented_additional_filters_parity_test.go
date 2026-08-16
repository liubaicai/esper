package esper

import (
	"context"
	"testing"
)

// TestKeyContextAdditionalFiltersMultiStreamParity locks the multi-stream
// segmented context with per-stream filters and pattern-fed aggregates
// verified against ContextKeySegmentedAdditionalFilters: `partition by
// theString from SupportBean(intPrimitive>0), p00 from SupportBean_S0(id >
// 0)` with `select sum(sb.intPrimitive) as col1, sum(s0.id) as col2 from
// pattern [every (s0=SupportBean_S0 or sb=SupportBean)]`. Events failing
// their stream filter create no partition; pattern matches accumulate per
// partition with the match's own captured tags.
func TestKeyContextAdditionalFiltersMultiStreamParity(t *testing.T) {
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
	p00 := Field[keySegJoinS0, string]("p00")
	if _, err := CreateKeyContextByStreams(env, "SegmentedByAString",
		KeyContextStream{Type: "SupportBean", Keys: []Expr{theString}},
		KeyContextStream{Type: "SupportBean_S0", Keys: []Expr{p00}},
	); err != nil {
		t.Fatal(err)
	}
	pattern := PatternFrom(s0, "s0", Greater[int](Field[keySegJoinS0, int]("id"), Literal(0))).
		Or(PatternFrom(sb, "sb", Greater[int](Field[keySegJoinBean, int]("intPrimitive"), Literal(0)))).
		Every()
	query := pattern.Select(
		Alias("col1", Sum[int](TagField[int]("sb", "intPrimitive"))),
		Alias("col2", Sum[int](TagField[int]("s0", "id"))),
	).Query(StatementName("s0"), WithContext("SegmentedByAString"))
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
		col1, col2 any
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				rows = append(rows, row{
					col1: rowValue.Get("col1").Any(),
					col2: rowValue.Get("col2").Any(),
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
	sendSB("B1", -1)
	sendS0("S0", -2)
	sendS0("S0", -3)
	sendSB("S0", -1)
	sendSB("S1", -2)
	sendS0("S0", 2)
	if len(rows) != 1 || rows[0].col1 != nil || rows[0].col2 != 2 {
		t.Fatalf("after S0(2) rows = %#v, want [{nil 2}]", rows)
	}
	sendSB("S1", 10)
	if len(rows) != 2 || rows[1].col1 != 10 || rows[1].col2 != nil {
		t.Fatalf("after SB(S1,10) rows = %#v, want [{nil 2} {10 nil}]", rows)
	}
	sendS0("S0", -2)
	sendSB("S1", -10)
	sendS0("S1", 3)
	if len(rows) != 3 || rows[2].col1 != 10 || rows[2].col2 != 3 {
		t.Fatalf("after S0(3,S1) rows = %#v, want third {10 3}", rows)
	}
	sendSB("S0", 9)
	if len(rows) != 4 || rows[3].col1 != 9 || rows[3].col2 != 2 {
		t.Fatalf("after SB(S0,9) rows = %#v, want fourth {9 2}", rows)
	}
}
