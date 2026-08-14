package esper

import (
	"context"
	"testing"
)

type resultsetRowPerGroupSimpleBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestResultSetRowPerGroupSimpleParity mirrors ResultSetQueryTypeRowPerGroupSimple:
// an unbound grouped aggregate with sum/min/max emits one row per group event.
func TestResultSetRowPerGroupSimpleParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[resultsetRowPerGroupSimpleBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	theString := Field[resultsetRowPerGroupSimpleBean, string]("theString")
	intPrimitive := Field[resultsetRowPerGroupSimpleBean, int]("intPrimitive")
	plan, err := env.Build(From[resultsetRowPerGroupSimpleBean](env, "SupportBean").
		GroupBy(theString).
		Select(
			Alias("c0", theString),
			Alias("c1", Sum[int](intPrimitive)),
			Alias("c2", Min[int](intPrimitive)),
			Alias("c3", Max[int](intPrimitive)),
		).
		Query(StatementName("s0")))
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
				t.Fatalf("row-per-group result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []resultsetRowPerGroupSimpleBean{
		{TheString: "E1", IntPrimitive: 10},
		{TheString: "E2", IntPrimitive: 100},
		{TheString: "E1", IntPrimitive: 11},
		{TheString: "E1", IntPrimitive: 9},
		{TheString: "E2", IntPrimitive: 99},
		{TheString: "E2", IntPrimitive: 97},
		{TheString: "E3", IntPrimitive: 1000},
		{TheString: "E2", IntPrimitive: 96},
		{TheString: "E2", IntPrimitive: 101},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	want := []struct {
		c0         string
		c1, c2, c3 int
	}{
		{"E1", 10, 10, 10},
		{"E2", 100, 100, 100},
		{"E1", 21, 10, 11},
		{"E1", 30, 9, 11},
		{"E2", 199, 99, 100},
		{"E2", 296, 97, 100},
		{"E3", 1000, 1000, 1000},
		{"E2", 392, 96, 100},
		{"E2", 493, 96, 101},
	}
	for index, expected := range want {
		if len(rows) <= index {
			t.Fatalf("rows = %#v", rows)
		}
		row := rows[index]
		if row.Get("c0").Any() != expected.c0 || row.Get("c1").Any() != expected.c1 ||
			row.Get("c2").Any() != expected.c2 || row.Get("c3").Any() != expected.c3 {
			t.Fatalf("row %d = %#v", index, row.AsMap())
		}
	}
	if len(rows) != 9 {
		t.Fatalf("rows = %d, want 9", len(rows))
	}
}
