package esper

import (
	"context"
	"testing"
)

type contextKeyedSubqueryParityBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type contextKeyedSubqueryParityS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// TestContextKeyedSubqueryParity mirrors ContextKeySegmentedSubqueryFiltered:
// each keyed partition owns a #lastevent subquery registry, so inner events
// that arrived before a partition was created are not visible to it.
func TestContextKeyedSubqueryParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextKeyedSubqueryParityBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[contextKeyedSubqueryParityS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "SegmentedByString", Field[contextKeyedSubqueryParityBean, string]("theString")); err != nil {
		t.Fatal(err)
	}
	inner := Select(From[contextKeyedSubqueryParityS0](env, "SupportBean_S0")).Window(LastEvent())
	query := Select(
		From[contextKeyedSubqueryParityBean](env, "SupportBean"),
		Alias("theString", Field[contextKeyedSubqueryParityBean, string]("theString")),
		Alias("intPrimitive", Field[contextKeyedSubqueryParityBean, int]("intPrimitive")),
		Alias("val0", SubqueryValue[string](
			inner,
			Field[contextKeyedSubqueryParityS0, string]("p00"),
			Equal[int](
				Field[contextKeyedSubqueryParityS0, int]("id"),
				OuterField[int]("intPrimitive"),
			),
		)),
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
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("context keyed subquery result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendS0 := func(id int, p00 string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextKeyedSubqueryParityS0{ID: id, P00: p00}); err != nil {
			t.Fatal(err)
		}
	}
	sendBean := func(theString string, value int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextKeyedSubqueryParityBean{TheString: theString, IntPrimitive: value}); err != nil {
			t.Fatal(err)
		}
	}
	sendS0(10, "s1")
	sendBean("G1", 10)
	assertContextKeyedSubqueryRow(t, rows, 0, "G1", 10, "", true)
	sendS0(10, "s2")
	sendBean("G1", 10)
	assertContextKeyedSubqueryRow(t, rows, 1, "G1", 10, "s2", false)
	sendBean("G2", 10)
	assertContextKeyedSubqueryRow(t, rows, 2, "G2", 10, "", true)
	sendS0(10, "s3")
	sendBean("G2", 10)
	assertContextKeyedSubqueryRow(t, rows, 3, "G2", 10, "s3", false)
	sendBean("G3", 10)
	assertContextKeyedSubqueryRow(t, rows, 4, "G3", 10, "", true)
	sendBean("G1", 10)
	assertContextKeyedSubqueryRow(t, rows, 5, "G1", 10, "s3", false)
}

func assertContextKeyedSubqueryRow(t *testing.T, rows []Row, index int, theString string, value int, want string, wantNull bool) {
	t.Helper()
	if len(rows) <= index {
		t.Fatalf("context keyed subquery rows = %#v", rows)
	}
	row := rows[index]
	if row.Get("theString").Any() != theString || row.Get("intPrimitive").Any() != value {
		t.Fatalf("context keyed subquery row %d = %#v", index, row.AsMap())
	}
	got := row.Get("val0")
	if got.IsNull() != wantNull || (!wantNull && got.Any() != want) {
		t.Fatalf("context keyed subquery val0 row %d = %#v, want %q (null=%v)", index, got.Any(), want, wantNull)
	}
}
