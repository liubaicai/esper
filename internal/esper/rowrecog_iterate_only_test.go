package esper

import (
	"context"
	"testing"
)

func TestRowRecogIterateOnlyNoListenerMode(t *testing.T) {
	env, engine := newRowRecogPreviousTest(t)
	stream := From[rowRecogPreviousEvent](env, "RowRecogPreviousEvent").Window(LengthWindow(1))
	query := stream.MatchRecognize(RowVar("A")).
		IterateOnly().
		Define("A", Literal(true)).
		Measures(Alias("a", TagField[string]("A", "name"))).
		Query(StatementName("rowrecog-iterate-only-no-listener"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	listenerCalls := 0
	if _, err := statement.Subscribe(func(context.Context, ResultBatch) error {
		listenerCalls++
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for index := 0; index < 50; index++ {
		sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E" + string(rune('A'+index)), Value: index})
	}
	if listenerCalls != 0 {
		t.Fatalf("iterate-only listener calls = %d, want 0", listenerCalls)
	}
	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E2", Value: 2})
	if listenerCalls != 0 {
		t.Fatalf("iterate-only listener calls after final event = %d, want 0", listenerCalls)
	}
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot, "E2")
}

func TestRowRecogIterateOnlyPrev(t *testing.T) {
	env, engine := newRowRecogPreviousTest(t)
	value := Field[rowRecogPreviousEvent, int]("value")
	stream := From[rowRecogPreviousEvent](env, "RowRecogPreviousEvent").Window(LastEvent())
	query := stream.MatchRecognize(RowVar("A")).
		IterateOnly().
		Define("A", Equal[int](Prev[int](2, value), value)).
		Measures(Alias("a", TagField[string]("A", "name"))).
		Query(StatementName("rowrecog-iterate-only-prev"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	listenerCalls := 0
	if _, err := statement.Subscribe(func(context.Context, ResultBatch) error {
		listenerCalls++
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, event := range []rowRecogPreviousEvent{
		{Name: "E1", Value: 1},
		{Name: "E2", Value: 2},
		{Name: "E3", Value: 3},
		{Name: "E4", Value: 4},
		{Name: "E5", Value: 2},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot)

	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E6", Value: 4})
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot, "E6")

	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E7", Value: 2})
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot, "E7")
	if listenerCalls != 0 {
		t.Fatalf("iterate-only PREV listener calls = %d, want 0", listenerCalls)
	}
}

func TestRowRecogIterateOnlyPrevPartitioned(t *testing.T) {
	env, engine := newRowRecogPreviousTest(t)
	category := Field[rowRecogPreviousEvent, string]("category")
	value := Field[rowRecogPreviousEvent, int]("value")
	stream := From[rowRecogPreviousEvent](env, "RowRecogPreviousEvent").Window(LastEvent())
	query := stream.MatchRecognize(RowVar("A")).
		IterateOnly().
		PartitionBy(category).
		Define("A", Equal[int](Prev[int](2, value), value)).
		Measures(
			Alias("a", TagField[string]("A", "name")),
			Alias("category", TagField[string]("A", "category")),
		).
		Query(StatementName("rowrecog-iterate-only-prev-partitioned"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	listenerCalls := 0
	if _, err := statement.Subscribe(func(context.Context, ResultBatch) error {
		listenerCalls++
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, event := range []rowRecogPreviousEvent{
		{Name: "E1", Category: "A", Value: 1},
		{Name: "E2", Category: "B", Value: 1},
		{Name: "E3", Category: "B", Value: 3},
		{Name: "E4", Category: "A", Value: 4},
		{Name: "E5", Category: "B", Value: 2},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot)

	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E6", Category: "A", Value: 1})
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot, "E6")

	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E7", Category: "B", Value: 3})
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot, "E7")
	if listenerCalls != 0 {
		t.Fatalf("iterate-only partitioned PREV listener calls = %d, want 0", listenerCalls)
	}
}

func TestRowRecogIterateOnlyDoesNotConsumeStatePool(t *testing.T) {
	env, _ := newRowRecogPreviousTest(t)
	engine := NewEngine(env, WithMatchRecognizeStateLimit(1, true))
	stream := From[rowRecogPreviousEvent](env, "RowRecogPreviousEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		IterateOnly().
		Define("A", Literal(true)).
		Define("B", Literal(true)).
		Measures(Alias("a", TagField[string]("A", "name"))).
		Query(StatementName("rowrecog-iterate-only-state-pool"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	var limits []MatchRecognizeStateLimitEvent
	if err := engine.AddMatchRecognizeStateLimitListener(MatchRecognizeStateLimitListenerFunc(func(event MatchRecognizeStateLimitEvent) {
		limits = append(limits, event)
	})); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 10; index++ {
		sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E" + string(rune('A'+index)), Value: index})
	}
	if len(limits) != 0 {
		t.Fatalf("iterate-only state-pool overflow events = %#v, want none", limits)
	}
}
