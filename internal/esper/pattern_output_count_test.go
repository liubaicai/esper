package esper

import (
	"context"
	"encoding/json"
	"testing"
)

type patternOutputCountEvent struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// Java's output-limit condition counts the match rows a pattern posts into
// the output process view, not the statement's input events: A-events that
// complete no match never reach the output view, so "output every 3 events"
// must fire on the B-event that delivers three match rows at once
// (ResultSetOrderByMultiDelivery part 2, ESPER-409).
func TestPatternOutputLimitCountsMatchRows(t *testing.T) {
	ctx := context.Background()
	env := NewEnvironment()
	if _, err := RegisterStruct[patternOutputCountEvent](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	pattern := PatternFrom(
		From[patternOutputCountEvent](env, "SupportBean"),
		"a",
		LikeOf(Field[patternOutputCountEvent, string]("theString"), Literal("A%")),
	).Every().FollowedBy(
		"b",
		LikeOf(Field[patternOutputCountEvent, string]("theString"), Literal("B%")),
	)
	query := pattern.Select(
		Alias("theString", TagField[string]("a", "theString")),
	).Query(
		StatementName("s0"),
		WithOutput(OutputAllEveryEvents(3)),
		OrderBy(Descending(ResultField[string]("theString"))),
	)
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(ctx) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches [][]string
	_, err = statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		rows := make([]string, 0, len(batch.New))
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				data, _ := json.Marshal(row.Get("theString").Any())
				rows = append(rows, string(data))
			}
		}
		batches = append(batches, rows)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []patternOutputCountEvent{
		{"A1", 1}, {"A2", 2}, {"A3", 3},
	} {
		if err := engine.Send(ctx, "SupportBean", event); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 0 {
		t.Fatalf("input-only events produced %d batches, want 0", len(batches))
	}
	if err := engine.Send(ctx, "SupportBean", patternOutputCountEvent{"B", 3}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("B produced %d batches, want 1", len(batches))
	}
	want := []string{`"A3"`, `"A2"`, `"A1"`}
	if len(batches[0]) != 3 || batches[0][0] != want[0] || batches[0][1] != want[1] || batches[0][2] != want[2] {
		t.Fatalf("rows = %v, want %v", batches[0], want)
	}
}

// Without an output policy a pattern statement delivers each match batch
// immediately, and the order-by sorts the delivered rows within the batch
// (ESPER-409): two pending every-branches completed by one B-event arrive as
// one callback of two rows.
func TestPatternImmediateDeliverySortsBatch(t *testing.T) {
	ctx := context.Background()
	env := NewEnvironment()
	if _, err := RegisterStruct[patternOutputCountEvent](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	pattern := PatternFrom(
		From[patternOutputCountEvent](env, "SupportBean"),
		"a",
		LikeOf(Field[patternOutputCountEvent, string]("theString"), Literal("A%")),
	).Every().FollowedBy(
		"b",
		LikeOf(Field[patternOutputCountEvent, string]("theString"), Literal("B%")),
	)
	query := pattern.Select(
		Alias("theString", TagField[string]("a", "theString")),
	).Query(
		StatementName("s0"),
		OrderBy(Descending(ResultField[string]("theString"))),
	)
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(ctx) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches [][]string
	_, err = statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		rows := make([]string, 0, len(batch.New))
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				data, _ := json.Marshal(row.Get("theString").Any())
				rows = append(rows, string(data))
			}
		}
		batches = append(batches, rows)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []patternOutputCountEvent{{"A1", 1}, {"A2", 2}} {
		if err := engine.Send(ctx, "SupportBean", event); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 0 {
		t.Fatalf("A-events produced %d batches, want 0", len(batches))
	}
	if err := engine.Send(ctx, "SupportBean", patternOutputCountEvent{"B", 3}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("B produced %d batches, want 1", len(batches))
	}
	want := []string{`"A2"`, `"A1"`}
	if len(batches[0]) != 2 || batches[0][0] != want[0] || batches[0][1] != want[1] {
		t.Fatalf("rows = %v, want %v", batches[0], want)
	}
}
