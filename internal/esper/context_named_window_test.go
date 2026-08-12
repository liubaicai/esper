package esper

import (
	"context"
	"testing"
)

type contextBoundNamedEvent struct {
	Group string `esper:"group"`
	ID    int64  `esper:"id"`
	Value string `esper:"value"`
}

type contextBoundNamedOuter struct {
	Group string `esper:"group"`
	ID    int64  `esper:"id"`
}

func TestContextScopedNamedWindowKeepsPartitionLocalRetentionAndSubqueryState(t *testing.T) {
	env := NewEnvironment()
	windowSchema, err := RegisterStruct[contextBoundNamedEvent](env, "ContextBoundNamedEvent")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[contextBoundNamedOuter](env, "ContextBoundNamedOuter"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "context-bound-named", Field[contextBoundNamedOuter, string]("group")); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "context-bound-window", windowSchema,
		NamedWindowContext("context-bound-named"),
		NamedWindowRetention(Unique(Field[contextBoundNamedEvent, int64]("id"))),
	); err != nil {
		t.Fatal(err)
	}

	consumerPlan, err := env.Build(FromNamedWindow(env, "context-bound-window").Select(
		Alias("group", Field[contextBoundNamedEvent, string]("group")),
		Alias("id", Field[contextBoundNamedEvent, int64]("id")),
	).Query(StatementName("context-bound-window-consumer"), WithContext("context-bound-named")))
	if err != nil {
		t.Fatal(err)
	}
	patternPlan, err := env.Build(PatternFromRecord(
		FromNamedWindow(env, "context-bound-window"),
		"a",
		Literal[bool](true),
	).Every().Select(
		Alias("group", Field[contextBoundNamedEvent, string]("group")),
		Alias("value", Field[contextBoundNamedEvent, string]("value")),
	).Query(StatementName("context-bound-window-pattern"), WithContext("context-bound-named")))
	if err != nil {
		t.Fatal(err)
	}
	inner := FromNamedWindow(env, "context-bound-window")
	subqueryPlan, err := env.Build(Select(
		From[contextBoundNamedOuter](env, "ContextBoundNamedOuter"),
		Alias("group", Field[contextBoundNamedOuter, string]("group")),
		Alias("id", Field[contextBoundNamedOuter, int64]("id")),
		Alias("value", SubqueryValue[string](
			inner,
			Field[contextBoundNamedEvent, string]("value"),
			Equal[int64](Field[contextBoundNamedEvent, int64]("id"), OuterField[int64]("id")),
		)),
	).Query(StatementName("context-bound-window-subquery"), WithContext("context-bound-named")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	subqueryDeployment, err := engine.Deploy(context.Background(), subqueryPlan)
	if err != nil {
		t.Fatal(err)
	}
	patternDeployment, err := engine.Deploy(context.Background(), patternPlan)
	if err != nil {
		t.Fatal(err)
	}
	var consumerRows []Row
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("context named-window consumer result = %#v", result)
			}
			consumerRows = append(consumerRows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var subqueryRows []Row
	if _, err := subqueryDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("context named-window subquery result = %#v", result)
			}
			subqueryRows = append(subqueryRows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var patternRows []Row
	if _, err := patternDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("context named-window pattern result = %#v", result)
			}
			patternRows = append(patternRows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	insert := func(event contextBoundNamedEvent) {
		t.Helper()
		if err := engine.InsertNamedWindow(context.Background(), "context-bound-window", event); err != nil {
			t.Fatal(err)
		}
	}
	insert(contextBoundNamedEvent{Group: "A", ID: 1, Value: "A-1"})
	insert(contextBoundNamedEvent{Group: "B", ID: 1, Value: "B-1"})
	insert(contextBoundNamedEvent{Group: "A", ID: 1, Value: "A-1-replaced"})

	window, ok := engine.NamedWindow("context-bound-window")
	if !ok {
		t.Fatal("context-bound named window is missing")
	}
	all, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("context-bound all-partition snapshot = %#v, want two rows", all)
	}
	keyA := encodeKey([]any{ValuePresent, "A"})
	partitionA, err := window.SnapshotContext(context.Background(), keyA)
	if err != nil {
		t.Fatal(err)
	}
	if len(partitionA) != 1 || partitionA[0].Underlying().(contextBoundNamedEvent).Value != "A-1-replaced" {
		t.Fatalf("context-bound partition A snapshot = %#v", partitionA)
	}

	if len(consumerRows) != 3 {
		t.Fatalf("context-bound consumer rows = %#v, want three new rows", consumerRows)
	}
	if len(patternRows) != 3 {
		t.Fatalf("context-bound pattern rows = %#v, want three matches", patternRows)
	}
	if got := consumerRows[0].Get("group").Any(); got != "A" {
		t.Fatalf("context-bound consumer first group = %#v", got)
	}
	if got := consumerRows[2].Get("id").Any(); got != int64(1) {
		t.Fatalf("context-bound consumer replacement row = %#v", consumerRows[2].AsMap())
	}
	if got := patternRows[2].Get("value").Any(); got != "A-1-replaced" {
		t.Fatalf("context-bound pattern replacement row = %#v", patternRows[2].AsMap())
	}

	fafPlan, err := env.Build(FromNamedWindow(env, "context-bound-window").Select(
		Alias("group", Field[contextBoundNamedEvent, string]("group")),
		Alias("value", Field[contextBoundNamedEvent, string]("value")),
	).Query(StatementName("context-bound-window-faf")))
	if err != nil {
		t.Fatal(err)
	}
	fafResult, err := engine.ExecuteFireAndForget(context.Background(), fafPlan)
	if err != nil {
		t.Fatal(err)
	}
	if len(fafResult.Results()) != 2 {
		t.Fatalf("context-bound FAF rows = %#v, want two rows", fafResult.Results())
	}
	contextFAFPlan, err := env.Build(FromNamedWindow(env, "context-bound-window").Select(
		Alias("group", Field[contextBoundNamedEvent, string]("group")),
		Alias("value", Field[contextBoundNamedEvent, string]("value")),
	).Query(StatementName("context-bound-window-context-faf"), WithContext("context-bound-named")))
	if err != nil {
		t.Fatal(err)
	}
	contextFAFResult, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), contextFAFPlan, ContextPartitionSelectorAll{})
	if err != nil {
		t.Fatal(err)
	}
	if len(contextFAFResult.Results()) != 2 {
		t.Fatalf("context-bound context FAF rows = %#v, want two rows", contextFAFResult.Results())
	}

	sendOuter := func(group string, id int64, want string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextBoundNamedOuter{Group: group, ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(subqueryRows) == 0 {
			t.Fatal("context-bound subquery did not produce a row")
		}
		value := subqueryRows[len(subqueryRows)-1].Get("value")
		if value.IsNull() || value.Any() != want {
			t.Fatalf("context-bound subquery %s/%d value = %#v, want %q", group, id, value.Any(), want)
		}
	}
	sendOuter("A", 1, "A-1-replaced")
	sendOuter("B", 1, "B-1")
}
