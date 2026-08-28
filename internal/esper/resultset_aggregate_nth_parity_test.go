package esper

import (
	"context"
	"reflect"
	"testing"
)

type resultSetAggregateNthBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestResultSetAggregateNthParity mirrors ResultSetAggregateNTh using both
// grouped last-output batching and ordering, including post-mutation nth values.
func TestResultSetAggregateNthParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[resultSetAggregateNthBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	key := Field[resultSetAggregateNthBean, string]("theString")
	value := Field[resultSetAggregateNthBean, int]("intPrimitive")
	query := From[resultSetAggregateNthBean](env, "SupportBean").Window(KeepAll()).GroupBy(key).Select(
		Alias("theString", key), Alias("int1", Nth[int](value, 0)), Alias("int2", Nth[int](value, 1)),
	)
	plan, err := env.Build(query.Query(StatementName("s0"), WithOutput(OutputLastEveryEvents(3)), OrderBy(Ascending(ResultField[string]("theString")))))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches [][]map[string]any
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		var rows []map[string]any
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("result is not a row: %#v", result)
			}
			rows = append(rows, map[string]any{"theString": row.Get("theString").Any(), "int1": row.Get("int1").Any(), "int2": row.Get("int2").Any()})
		}
		batches = append(batches, rows)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []resultSetAggregateNthBean{{"G1", 10}, {"G2", 11}, {"G1", 12}, {"G2", 30}, {"G2", 20}, {"G2", 25}, {"G1", -1}, {"G1", -2}, {"G2", 8}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	want := [][]map[string]any{
		{{"theString": "G1", "int1": 12, "int2": 10}, {"theString": "G2", "int1": 11, "int2": nil}},
		{{"theString": "G2", "int1": 25, "int2": 20}},
		{{"theString": "G1", "int1": -2, "int2": -1}, {"theString": "G2", "int1": 8, "int2": 25}},
	}
	if !reflect.DeepEqual(batches, want) {
		t.Fatalf("batches = %#v, want %#v", batches, want)
	}
}
