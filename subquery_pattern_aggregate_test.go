package esper

import (
	"context"
	"testing"
)

type subqueryAggregationCandidate struct {
	ID int64 `esper:"id"`
}

type subqueryAggregationReference struct {
	ID int64 `esper:"id"`
}

func TestEventStreamSubqueryAggregationWindowMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[subqueryAggregationCandidate](env, "SubqueryAggregationCandidate"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subqueryAggregationReference](env, "SubqueryAggregationReference"); err != nil {
		t.Fatal(err)
	}

	inner := Select(From[subqueryAggregationReference](env, "SubqueryAggregationReference")).Window(LengthWindow(2))
	candidate := From[subqueryAggregationCandidate](env, "SubqueryAggregationCandidate")
	query := Select(
		candidate.Filter(Equal[int64](
			Field[subqueryAggregationCandidate, int64]("id"),
			SubquerySum[int64](inner, Field[any, int64]("id")),
		)),
		Alias("id", Field[subqueryAggregationCandidate, int64]("id")),
	).Query(StatementName("event-stream-subquery-aggregate"))
	plan, err := env.Build(query)
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
				t.Fatalf("event-stream aggregate result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendCandidate := func(id int64, wantRows int) {
		t.Helper()
		before := len(rows)
		if err := engine.SendEvent(context.Background(), subqueryAggregationCandidate{ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(rows)-before != wantRows {
			t.Fatalf("event-stream aggregate candidate %d emitted %d rows, want %d", id, len(rows)-before, wantRows)
		}
	}
	sendReference := func(id int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), subqueryAggregationReference{ID: id}); err != nil {
			t.Fatal(err)
		}
	}

	sendCandidate(1, 0)
	sendReference(1)
	sendCandidate(1, 1)
	sendReference(3)
	sendCandidate(3, 0)
	sendCandidate(5, 0)
	sendCandidate(4, 1)
	sendReference(10)
	sendCandidate(10, 0)
	sendCandidate(3, 0)
	sendCandidate(13, 1)

	if len(rows) != 3 {
		t.Fatalf("event-stream aggregate rows = %#v, want three rows", rows)
	}
	for index, want := range []int64{1, 4, 13} {
		if got := rows[index].Get("id").Any(); got != want {
			t.Fatalf("event-stream aggregate row %d id = %#v, want %d", index, got, want)
		}
	}
}
