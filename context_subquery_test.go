package esper

import (
	"context"
	"testing"
)

type contextSubqueryOuter struct {
	Symbol string `esper:"symbol"`
	ID     int64  `esper:"id"`
}

type contextSubqueryReference struct {
	ID    int64  `esper:"id"`
	Value string `esper:"value"`
}

func TestContextEventStreamSubqueryKeepsPartitionLocalLastEvent(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextSubqueryOuter](env, "ContextSubqueryOuter"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[contextSubqueryReference](env, "ContextSubqueryReference"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "context-subquery-by-symbol", Field[contextSubqueryOuter, string]("symbol")); err != nil {
		t.Fatal(err)
	}

	inner := Select(From[contextSubqueryReference](env, "ContextSubqueryReference")).Window(LastEvent())
	plan, err := env.Build(Select(
		From[contextSubqueryOuter](env, "ContextSubqueryOuter"),
		Alias("symbol", Field[contextSubqueryOuter, string]("symbol")),
		Alias("id", Field[contextSubqueryOuter, int64]("id")),
		Alias("value", SubqueryValue[string](
			inner,
			Field[contextSubqueryReference, string]("value"),
			Equal[int64](Field[contextSubqueryReference, int64]("id"), OuterField[int64]("id")),
		)),
	).Query(StatementName("context-subquery"), WithContext("context-subquery-by-symbol")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("context subquery result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendReference := func(id int64, value string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextSubqueryReference{ID: id, Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	sendOuter := func(symbol string, id int64, wantValue string, wantNull bool) {
		t.Helper()
		before := len(rows)
		if err := engine.SendEvent(context.Background(), contextSubqueryOuter{Symbol: symbol, ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(rows) != before+1 {
			t.Fatalf("context subquery rows after %s = %d, want %d", symbol, len(rows), before+1)
		}
		row := rows[len(rows)-1]
		if row.Get("symbol").Any() != symbol || row.Get("id").Any() != id {
			t.Fatalf("context subquery outer row = %#v", row.AsMap())
		}
		value := row.Get("value")
		if value.IsNull() != wantNull || (!wantNull && value.Any() != wantValue) {
			t.Fatalf("context subquery value for %s = %#v, want %q (null=%v)", symbol, value.Any(), wantValue, wantNull)
		}
	}

	// Existing partitions do not retroactively receive an inner event that
	// arrived before their creation, matching Esper's #lastevent behavior.
	sendReference(10, "s1")
	sendOuter("G1", 10, "", true)
	sendReference(10, "s2")
	sendOuter("G1", 10, "s2", false)
	sendOuter("G2", 10, "", true)
	sendReference(10, "s3")
	sendOuter("G2", 10, "s3", false)
	sendOuter("G3", 10, "", true)
	sendOuter("G1", 10, "s3", false)
}
