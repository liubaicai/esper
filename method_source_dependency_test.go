package esper

import (
	"context"
	"fmt"
	"testing"
)

type methodDependencyTrigger struct {
	ID    string `esper:"id"`
	Count int    `esper:"count"`
}

type methodDependencyRow struct {
	Value string `esper:"value"`
	Index int    `esper:"index"`
}

func TestMethodSourceDependentJoinFollowsTopologicalOrderAndLineage(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[methodDependencyTrigger](env, "MethodDependencyTrigger"); err != nil {
		t.Fatal(err)
	}
	rowSchema, err := StructSchema[methodDependencyRow]("MethodDependencyRow")
	if err != nil {
		t.Fatal(err)
	}
	base := From[methodDependencyTrigger](env, "MethodDependencyTrigger").Window(LengthWindow(1))
	h0Calls, h1Calls := 0, 0
	h0 := FromMethodOn[methodDependencyRow](env, "h0", "MethodDependencyTrigger", rowSchema, MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		h0Calls++
		trigger, ok := request.Dependency("MethodDependencyTrigger")
		if !ok {
			return nil, fmt.Errorf("h0 dependency is missing")
		}
		input := trigger.Underlying().(methodDependencyTrigger)
		rows := make([]Event, 0, input.Count)
		for index := 0; index < input.Count; index++ {
			event, eventErr := newEvent(rowSchema, methodDependencyRow{
				Value: fmt.Sprintf("%s-h0-%d", input.ID, index), Index: input.Count,
			}, request.Now)
			if eventErr != nil {
				return nil, eventErr
			}
			rows = append(rows, event)
		}
		return rows, nil
	})).DependingOn("MethodDependencyTrigger")
	h1 := FromMethodOn[methodDependencyRow](env, "h1", "MethodDependencyTrigger", rowSchema, MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		h1Calls++
		previous, ok := request.Dependency("h0")
		if !ok {
			return nil, fmt.Errorf("h1 dependency is missing")
		}
		input := previous.Underlying().(methodDependencyRow)
		event, eventErr := newEvent(rowSchema, methodDependencyRow{Value: input.Value + "-h1", Index: input.Index}, request.Now)
		return []Event{event}, eventErr
	})).DependingOn("h0")

	// Reverse declaration order proves that dependency evaluation is
	// topological while JoinMany source indexes remain stable.
	query := JoinMany(JoinSource(h1), JoinSource(h0), JoinSource(base)).On(
		OnSourcesEqual(0, Field[methodDependencyRow, int]("index"), 1, Field[methodDependencyRow, int]("index")),
		OnSourcesEqual(1, Field[methodDependencyRow, int]("index"), 2, Field[methodDependencyTrigger, int]("count")),
	).Select(
		SelectFrom(2, "id", Field[methodDependencyTrigger, string]("id")),
		SelectFrom(1, "h0", Field[methodDependencyRow, string]("value")),
		SelectFrom(0, "h1", Field[methodDependencyRow, string]("value")),
	).Query(StatementName("method-dependent-event-chain"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	if _, err := deployment.Statements()[0].Subscribe(func(context.Context, ResultBatch) error { return nil }); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), methodDependencyTrigger{ID: "E1", Count: 2}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 2 || h0Calls != 1 || h1Calls != 2 {
		t.Fatalf("first dependent snapshot/calls = %#v h0:%d h1:%d", snapshot.Results(), h0Calls, h1Calls)
	}
	seen := map[string]bool{}
	for _, result := range snapshot.Results() {
		row, ok := result.Row()
		if !ok || row.Get("id").Any() != "E1" {
			t.Fatalf("first dependent result = %#v", result)
		}
		seen[fmt.Sprintf("%v/%v", row.Get("h0").Any(), row.Get("h1").Any())] = true
	}
	if !seen["E1-h0-0/E1-h0-0-h1"] || !seen["E1-h0-1/E1-h0-1-h1"] || len(seen) != 2 {
		t.Fatalf("first dependent rows = %#v", seen)
	}

	// Method sides are replaced per trigger and the ordinary dependency is a
	// last-event window, so no E1 lineage may leak into this cycle.
	if err := engine.SendEvent(context.Background(), methodDependencyTrigger{ID: "E2", Count: 1}); err != nil {
		t.Fatal(err)
	}
	snapshot, err = deployment.Statements()[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 1 || h0Calls != 2 || h1Calls != 3 {
		t.Fatalf("replacement snapshot/calls = %#v h0:%d h1:%d", snapshot.Results(), h0Calls, h1Calls)
	}
	row, ok := snapshot.Results()[0].Row()
	if !ok || row.Get("id").Any() != "E2" || row.Get("h0").Any() != "E2-h0-0" || row.Get("h1").Any() != "E2-h0-0-h1" {
		t.Fatalf("replacement dependent row = %#v", snapshot.Results()[0])
	}
}
