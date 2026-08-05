package esper

import (
	"context"
	"testing"
)

type contextNamedMutationEvent struct {
	Group string `esper:"group"`
	ID    int64  `esper:"id"`
	Value int64  `esper:"value"`
}

func TestContextScopedNamedWindowOnTriggerMutationsStayPartitionLocal(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[contextNamedMutationEvent](env, "ContextNamedMutationEvent")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "context-named-mutation", Field[contextNamedMutationEvent, string]("group")); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "context-named-mutation-window", schema,
		NamedWindowContext("context-named-mutation"),
		NamedWindowRetention(Unique(Field[contextNamedMutationEvent, int64]("id"))),
	); err != nil {
		t.Fatal(err)
	}

	source := From[contextNamedMutationEvent](env, "ContextNamedMutationEvent")
	group := Field[contextNamedMutationEvent, string]("group")
	id := Field[contextNamedMutationEvent, int64]("id")
	value := Field[contextNamedMutationEvent, int64]("value")
	matchID := Equal[int64](NamedWindowField[int64]("id"), id)

	consumerPlan, err := env.Build(FromNamedWindow(env, "context-named-mutation-window").Query(
		StatementName("context-named-mutation-consumer"),
		WithOldStream(),
		WithContext("context-named-mutation"),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = consumerDeployment.Undeploy(context.Background()) }()
	var batches []ResultBatch
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow("context-named-mutation-window",
		SetColumn("group", group),
		SetColumn("id", id),
		SetColumn("value", value),
	).Query(StatementName("context-named-mutation-insert"), WithContext("context-named-mutation")))
	if err != nil {
		t.Fatal(err)
	}
	insertDeployment, err := engine.Deploy(context.Background(), insertPlan)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []contextNamedMutationEvent{
		{Group: "A", ID: 1, Value: 10},
		{Group: "B", ID: 1, Value: 20},
		{Group: "A", ID: 2, Value: 30},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := insertDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}

	updatePlan, err := env.Build(OnEvent(source).UpdateNamedWindow("context-named-mutation-window", matchID,
		SetColumn("value", value),
	).Query(StatementName("context-named-mutation-update"), WithContext("context-named-mutation")))
	if err != nil {
		t.Fatal(err)
	}
	updateDeployment, err := engine.Deploy(context.Background(), updatePlan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), contextNamedMutationEvent{Group: "B", ID: 1, Value: 25}); err != nil {
		t.Fatal(err)
	}
	if err := updateDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}

	deletePlan, err := env.Build(OnEvent(source).DeleteFromNamedWindow("context-named-mutation-window", matchID).
		Query(StatementName("context-named-mutation-delete"), WithContext("context-named-mutation")))
	if err != nil {
		t.Fatal(err)
	}
	deleteDeployment, err := engine.Deploy(context.Background(), deletePlan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), contextNamedMutationEvent{Group: "A", ID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := deleteDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}

	mergePlan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("context-named-mutation-window", matchID,
		WhenMatched(Literal(true), SetColumn("value", value)),
		WhenNotMatched(Literal(true), SetColumn("group", group), SetColumn("id", id), SetColumn("value", value)),
	).Query(StatementName("context-named-mutation-merge"), WithContext("context-named-mutation")))
	if err != nil {
		t.Fatal(err)
	}
	mergeDeployment, err := engine.Deploy(context.Background(), mergePlan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), contextNamedMutationEvent{Group: "B", ID: 1, Value: 35}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), contextNamedMutationEvent{Group: "A", ID: 3, Value: 40}); err != nil {
		t.Fatal(err)
	}
	if err := mergeDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}

	window, ok := engine.NamedWindow("context-named-mutation-window")
	if !ok {
		t.Fatal("context-scoped mutation window is missing")
	}
	keyA := encodeKey([]any{ValuePresent, "A"})
	keyB := encodeKey([]any{ValuePresent, "B"})
	partitionA, err := window.SnapshotContext(context.Background(), keyA)
	if err != nil {
		t.Fatal(err)
	}
	partitionB, err := window.SnapshotContext(context.Background(), keyB)
	if err != nil {
		t.Fatal(err)
	}
	if len(partitionA) != 2 || partitionA[0].Underlying().(contextNamedMutationEvent).ID != 2 || partitionA[1].Underlying().(contextNamedMutationEvent).ID != 3 {
		t.Fatalf("context mutation partition A = %#v", partitionA)
	}
	if len(partitionB) != 1 || partitionB[0].Underlying().(contextNamedMutationEvent).Value != 35 {
		t.Fatalf("context mutation partition B = %#v", partitionB)
	}

	if len(batches) != 7 {
		t.Fatalf("context mutation consumer batches = %#v, want seven", batches)
	}
	if len(batches[3].Old) != 1 || len(batches[3].New) != 1 {
		t.Fatalf("context mutation update batch = %#v", batches[3])
	}
	if len(batches[4].Old) != 1 || len(batches[4].New) != 0 {
		t.Fatalf("context mutation delete batch = %#v", batches[4])
	}
	if len(batches[5].Old) != 1 || len(batches[5].New) != 1 {
		t.Fatalf("context mutation merge update batch = %#v", batches[5])
	}
	if len(batches[6].Old) != 0 || len(batches[6].New) != 1 {
		t.Fatalf("context mutation merge insert batch = %#v", batches[6])
	}
}
