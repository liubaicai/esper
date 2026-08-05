package esper

import (
	"context"
	"testing"
)

type patternSubqueryNamedEvent struct {
	ID int64 `esper:"id"`
}

type patternSubqueryUDFTrigger struct {
	ID int64 `esper:"id"`
}

func TestPatternSubqueryNamedWindowFunctionMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[patternSubqueryNamedEvent](env, "PatternSubqueryNamedEvent")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternSubqueryUDFTrigger](env, "PatternSubqueryUDFTrigger"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "PatternSubqueryNamedWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	inner := FromNamedWindow(env, "PatternSubqueryNamedWindow")
	var seenCounts []int64
	function := Func1[int64, bool]("supportSingleRowFunction", func(count int64) bool {
		seenCounts = append(seenCounts, count)
		return count == 1
	}, SubqueryCount(inner))
	pattern := PatternFrom(
		From[patternSubqueryUDFTrigger](env, "PatternSubqueryUDFTrigger"),
		"trigger",
		function,
	)
	plan, err := env.Build(pattern.Select(
		Alias("id", TagField[int64]("trigger", "id")),
	).Query(StatementName("pattern-subquery-named-udf")))
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
				t.Fatalf("pattern named-window function result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.InsertNamedWindow(context.Background(), "PatternSubqueryNamedWindow", patternSubqueryNamedEvent{ID: 7}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternSubqueryUDFTrigger{ID: 11}); err != nil {
		t.Fatal(err)
	}
	if len(seenCounts) != 1 || seenCounts[0] != 1 {
		t.Fatalf("pattern named-window function counts = %#v, want [1]", seenCounts)
	}
	if len(rows) != 1 || rows[0].Get("id").Any() != int64(11) {
		t.Fatalf("pattern named-window function rows = %#v, want trigger 11", rows)
	}
}
