package esper

import (
	"context"
	"testing"
)

// TestContextScopedNamedWindowTriggerMutatesOnlyCurrentPartition fixes the
// on-trigger side of ContextKeySegmentedNamedWindow.  The same trigger
// definition is evaluated in A and B partitions; update/delete/merge/select
// must never inspect or mutate the sibling partition.
func TestContextScopedNamedWindowTriggerMutatesOnlyCurrentPartition(t *testing.T) {
	env := NewEnvironment()
	windowSchema, err := RegisterStruct[contextBoundNamedEvent](env, "ContextTriggerEvent")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "context-trigger", Field[contextBoundNamedEvent, string]("group")); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "context-trigger-window", windowSchema,
		NamedWindowContext("context-trigger"),
		NamedWindowRetention(Unique(Field[contextBoundNamedEvent, int64]("id"))),
	); err != nil {
		t.Fatal(err)
	}

	source := From[contextBoundNamedEvent](env, "ContextTriggerEvent")
	id := Field[contextBoundNamedEvent, int64]("id")
	value := Field[contextBoundNamedEvent, string]("value")
	group := Field[contextBoundNamedEvent, string]("group")
	matchID := Equal[int64](NamedWindowField[int64]("id"), id)

	updatePlan, err := env.Build(OnEvent(source).UpdateNamedWindow(
		"context-trigger-window",
		matchID,
		SetColumn("value", value),
	).Query(StatementName("context-trigger-update"), WithContext("context-trigger")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(source).DeleteFromNamedWindow(
		"context-trigger-window", matchID,
	).Query(StatementName("context-trigger-delete"), WithContext("context-trigger")))
	if err != nil {
		t.Fatal(err)
	}
	mergePlan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen(
		"context-trigger-window", matchID,
		WhenMatched(Literal[bool](true), SetColumn("value", value)),
		WhenNotMatched(Literal[bool](true),
			SetColumn("group", group),
			SetColumn("id", id),
			SetColumn("value", value),
		),
	).Query(StatementName("context-trigger-merge"), WithContext("context-trigger")))
	if err != nil {
		t.Fatal(err)
	}
	selectPlan, err := env.Build(OnEvent(source).SelectFromNamedWindow(
		"context-trigger-window", Literal[bool](true),
		Alias("group", NamedWindowField[string]("group")),
		Alias("id", NamedWindowField[int64]("id")),
		Alias("value", NamedWindowField[string]("value")),
	).Query(StatementName("context-trigger-select"), WithContext("context-trigger")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deploy := func(plan Plan) *Deployment {
		deployment, deployErr := engine.Deploy(context.Background(), plan)
		if deployErr != nil {
			t.Fatal(deployErr)
		}
		return deployment
	}

	updateDeployment := deploy(updatePlan)
	var updateBatches []ResultBatch
	if _, err := updateDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		updateBatches = append(updateBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	insert := func(event contextBoundNamedEvent) {
		t.Helper()
		if err := engine.InsertNamedWindow(context.Background(), "context-trigger-window", event); err != nil {
			t.Fatal(err)
		}
	}
	insert(contextBoundNamedEvent{Group: "A", ID: 1, Value: "A-before"})
	insert(contextBoundNamedEvent{Group: "B", ID: 1, Value: "B-before"})
	if err := engine.SendEvent(context.Background(), contextBoundNamedEvent{Group: "A", ID: 1, Value: "A-updated"}); err != nil {
		t.Fatal(err)
	}
	updatedEvent := Event{}
	updatedOK := false
	if len(updateBatches) == 1 && len(updateBatches[0].New) == 1 {
		updatedEvent, updatedOK = updateBatches[0].New[0].Event()
	}
	if len(updateBatches) != 1 || len(updateBatches[0].Old) != 1 || len(updateBatches[0].New) != 1 || !updatedOK ||
		updatedEvent.Get("value").Any() != "A-updated" {
		t.Fatalf("context update batch = %#v", updateBatches)
	}
	window, ok := engine.NamedWindow("context-trigger-window")
	if !ok {
		t.Fatal("context trigger named window is missing")
	}
	keyA := encodeKey([]any{ValuePresent, "A"})
	keyB := encodeKey([]any{ValuePresent, "B"})
	assertPartitionValue := func(key, want string) {
		t.Helper()
		events, snapshotErr := window.SnapshotContext(context.Background(), key)
		if snapshotErr != nil {
			t.Fatal(snapshotErr)
		}
		if len(events) != 1 || events[0].Get("value").Any() != want {
			t.Fatalf("partition %q events = %#v, want value %q", key, events, want)
		}
	}
	assertPartitionValue(keyA, "A-updated")
	assertPartitionValue(keyB, "B-before")

	if err := engine.Undeploy(context.Background(), updateDeployment.ID()); err != nil {
		t.Fatal(err)
	}
	deleteDeployment := deploy(deletePlan)
	if err := engine.SendEvent(context.Background(), contextBoundNamedEvent{Group: "A", ID: 1, Value: "ignored"}); err != nil {
		t.Fatal(err)
	}
	if events, snapshotErr := window.SnapshotContext(context.Background(), keyA); snapshotErr != nil {
		t.Fatal(snapshotErr)
	} else if len(events) != 0 {
		t.Fatalf("partition A after delete = %#v", events)
	}
	assertPartitionValue(keyB, "B-before")
	if err := engine.Undeploy(context.Background(), deleteDeployment.ID()); err != nil {
		t.Fatal(err)
	}

	mergeDeployment := deploy(mergePlan)
	if err := engine.SendEvent(context.Background(), contextBoundNamedEvent{Group: "A", ID: 2, Value: "A-inserted"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), contextBoundNamedEvent{Group: "B", ID: 1, Value: "B-merged"}); err != nil {
		t.Fatal(err)
	}
	assertPartitionValue(keyA, "A-inserted")
	assertPartitionValue(keyB, "B-merged")
	if err := engine.Undeploy(context.Background(), mergeDeployment.ID()); err != nil {
		t.Fatal(err)
	}

	selectDeployment := deploy(selectPlan)
	var selected []Row
	if _, err := selectDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("context named-window trigger select result = %#v", result)
			}
			selected = append(selected, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), contextBoundNamedEvent{Group: "A", ID: 99, Value: "select"}); err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0].Get("group").Any() != "A" || selected[0].Get("id").Any() != int64(2) {
		t.Fatalf("context-local trigger select rows = %#v", selected)
	}
}

func TestContextScopedNamedWindowTriggerRequiresMatchingContext(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[contextBoundNamedEvent](env, "ContextTriggerValidationEvent")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "context-trigger-validation", Field[contextBoundNamedEvent, string]("group")); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "other-trigger-context", Field[contextBoundNamedEvent, string]("group")); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "context-trigger-validation-window", schema,
		NamedWindowContext("context-trigger-validation"),
	); err != nil {
		t.Fatal(err)
	}
	source := From[contextBoundNamedEvent](env, "ContextTriggerValidationEvent")
	match := Equal[int64](NamedWindowField[int64]("id"), Field[contextBoundNamedEvent, int64]("id"))
	if _, err := env.Build(OnEvent(source).DeleteFromNamedWindow("context-trigger-validation-window", match).Query()); err == nil {
		t.Fatal("context-bound named-window trigger without context was accepted")
	}
	if _, err := env.Build(OnEvent(source).DeleteFromNamedWindow("context-trigger-validation-window", match).Query(WithContext("other-trigger-context"))); err == nil {
		t.Fatal("context-bound named-window trigger with a different context was accepted")
	}
}
