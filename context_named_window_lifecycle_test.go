package esper

import (
	"context"
	"testing"
	"time"
)

type contextLifecycleNamedEvent struct {
	Group string `esper:"group"`
	ID    int64  `esper:"id"`
	Start bool   `esper:"start"`
	End   bool   `esper:"end"`
}

func TestTemporalContextNamedWindowReleasesPartitionAtBoundary(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[contextLifecycleNamedEvent](env, "TemporalContextNamedEvent")
	if err != nil {
		t.Fatal(err)
	}
	start, err := NewTimeOfDay(9, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	end, err := NewTimeOfDay(17, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateDailyTimeContext(env, "temporal-named-window", start, end); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "temporal-named-window-store", schema,
		NamedWindowContext("temporal-named-window"),
	); err != nil {
		t.Fatal(err)
	}

	source := From[contextLifecycleNamedEvent](env, "TemporalContextNamedEvent")
	group := Field[contextLifecycleNamedEvent, string]("group")
	id := Field[contextLifecycleNamedEvent, int64]("id")
	startFlag := Field[contextLifecycleNamedEvent, bool]("start")
	endFlag := Field[contextLifecycleNamedEvent, bool]("end")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow("temporal-named-window-store",
		SetColumn("group", group),
		SetColumn("id", id),
		SetColumn("start", startFlag),
		SetColumn("end", endFlag),
	).Query(StatementName("temporal-named-window-insert"), WithContext("temporal-named-window")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromNamedWindow(env, "temporal-named-window-store").Query(
		StatementName("temporal-named-window-consumer"),
		WithContext("temporal-named-window"),
		WithOldStream(),
	))
	if err != nil {
		t.Fatal(err)
	}

	initial := time.Date(2002, time.May, 1, 8, 0, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(initial))
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = consumerDeployment.Undeploy(context.Background()) }()
	insertDeployment, err := engine.Deploy(context.Background(), insertPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = insertDeployment.Undeploy(context.Background()) }()
	var batches []ResultBatch
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), contextLifecycleNamedEvent{Group: "A", ID: 1}); err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("temporal-named-window-store")
	if !ok {
		t.Fatal("temporal named window is missing")
	}
	if events, err := window.Snapshot(context.Background()); err != nil || len(events) != 0 {
		t.Fatalf("temporal named window accepted event before start: %#v, err=%v", events, err)
	}

	activeStart := time.Date(2002, time.May, 1, 9, 0, 0, 0, time.UTC)
	if err := engine.AdvanceTime(context.Background(), activeStart); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), contextLifecycleNamedEvent{Group: "A", ID: 2}); err != nil {
		t.Fatal(err)
	}
	activeKey := temporalPartitionKey(activeStart)
	if events, err := window.SnapshotContext(context.Background(), activeKey); err != nil || len(events) != 1 || events[0].Underlying().(contextLifecycleNamedEvent).ID != 2 {
		t.Fatalf("temporal named window active partition = %#v, err=%v", events, err)
	}

	endTime := time.Date(2002, time.May, 1, 17, 0, 0, 0, time.UTC)
	if err := engine.AdvanceTime(context.Background(), endTime); err != nil {
		t.Fatal(err)
	}
	if events, err := window.Snapshot(context.Background()); err != nil || len(events) != 0 {
		t.Fatalf("temporal named window retained ended partition: %#v, err=%v", events, err)
	}
	if _, err := window.SnapshotContext(context.Background(), activeKey); err == nil {
		t.Fatal("temporal named window retained an ended partition key")
	}

	nextStart := time.Date(2002, time.May, 2, 9, 0, 0, 0, time.UTC)
	if err := engine.AdvanceTime(context.Background(), nextStart); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), contextLifecycleNamedEvent{Group: "A", ID: 3}); err != nil {
		t.Fatal(err)
	}
	nextKey := temporalPartitionKey(nextStart)
	if events, err := window.SnapshotContext(context.Background(), nextKey); err != nil || len(events) != 1 || events[0].Underlying().(contextLifecycleNamedEvent).ID != 3 {
		t.Fatalf("temporal named window next partition = %#v, err=%v", events, err)
	}
	if len(batches) != 2 || len(batches[0].New) != 1 || len(batches[1].New) != 1 {
		t.Fatalf("temporal named window consumer batches = %#v", batches)
	}
}

func TestInitiatedContextNamedWindowReleasesPartitionOnTermination(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[contextLifecycleNamedEvent](env, "InitiatedContextNamedEvent")
	if err != nil {
		t.Fatal(err)
	}
	group := Field[contextLifecycleNamedEvent, string]("group")
	startFlag := Field[contextLifecycleNamedEvent, bool]("start")
	endFlag := Field[contextLifecycleNamedEvent, bool]("end")
	if _, err := CreateInitiatedTerminatedContext(env, "initiated-named-window", group, startFlag, endFlag); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "initiated-named-window-store", schema,
		NamedWindowContext("initiated-named-window"),
	); err != nil {
		t.Fatal(err)
	}
	source := From[contextLifecycleNamedEvent](env, "InitiatedContextNamedEvent")
	id := Field[contextLifecycleNamedEvent, int64]("id")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow("initiated-named-window-store",
		SetColumn("group", group),
		SetColumn("id", id),
		SetColumn("start", startFlag),
		SetColumn("end", endFlag),
	).Query(StatementName("initiated-named-window-insert"), WithContext("initiated-named-window")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromNamedWindow(env, "initiated-named-window-store").Query(
		StatementName("initiated-named-window-consumer"),
		WithContext("initiated-named-window"),
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
	insertDeployment, err := engine.Deploy(context.Background(), insertPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = insertDeployment.Undeploy(context.Background()) }()

	startEvent := contextLifecycleNamedEvent{Group: "A", ID: 1, Start: true}
	if err := engine.SendEvent(context.Background(), startEvent); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), contextLifecycleNamedEvent{Group: "A", ID: 2}); err != nil {
		t.Fatal(err)
	}
	definition, ok := env.Context("initiated-named-window")
	if !ok {
		t.Fatal("initiated context is missing")
	}
	partitionKey, active, err := definition.partition(Event{typeName: "InitiatedContextNamedEvent", streamType: "InitiatedContextNamedEvent", schema: schema, underlying: startEvent}, engine.Now(), nil)
	if err != nil || !active {
		t.Fatalf("initiated context partition = %q active=%v err=%v", partitionKey, active, err)
	}
	window, ok := engine.NamedWindow("initiated-named-window-store")
	if !ok {
		t.Fatal("initiated named window is missing")
	}
	if events, err := window.SnapshotContext(context.Background(), partitionKey); err != nil || len(events) != 2 {
		t.Fatalf("initiated named window active partition = %#v, err=%v", events, err)
	}

	if err := engine.SendEvent(context.Background(), contextLifecycleNamedEvent{Group: "A", ID: 3, End: true}); err != nil {
		t.Fatal(err)
	}
	if events, err := window.Snapshot(context.Background()); err != nil || len(events) != 0 {
		t.Fatalf("initiated named window retained terminated partition: %#v, err=%v", events, err)
	}
	if _, err := window.SnapshotContext(context.Background(), partitionKey); err == nil {
		t.Fatal("initiated named window retained terminated partition key")
	}
	if err := engine.SendEvent(context.Background(), contextLifecycleNamedEvent{Group: "A", ID: 4}); err != nil {
		t.Fatal(err)
	}
	if events, err := window.Snapshot(context.Background()); err != nil || len(events) != 0 {
		t.Fatalf("initiated named window accepted event while inactive: %#v, err=%v", events, err)
	}

	if err := engine.SendEvent(context.Background(), contextLifecycleNamedEvent{Group: "A", ID: 5, Start: true}); err != nil {
		t.Fatal(err)
	}
	if events, err := window.Snapshot(context.Background()); err != nil || len(events) != 1 || events[0].Underlying().(contextLifecycleNamedEvent).ID != 5 {
		t.Fatalf("initiated named window restarted partition = %#v, err=%v", events, err)
	}
}
