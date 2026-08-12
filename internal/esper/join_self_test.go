package esper

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

type selfJoinEvent struct {
	ID    string `esper:"id"`
	Group string `esper:"group"`
}

// TestSelfJoinUsesIndependentLogicalSidesMatchesEsper verifies that two
// source nodes for the same registered event type remain distinct join sides.
// A new event is visible to both sides, while prior events stay available for
// the cross-product; this is the Go-chain equivalent of an Esper self join.
func TestSelfJoinUsesIndependentLogicalSidesMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[selfJoinEvent](env, "SelfJoinEvent"); err != nil {
		t.Fatal(err)
	}
	left := From[selfJoinEvent](env, "SelfJoinEvent").Window(KeepAll())
	right := From[selfJoinEvent](env, "SelfJoinEvent").Window(KeepAll())
	plan, err := env.Build(Join(
		left,
		right,
		OnEqual(
			Field[selfJoinEvent, string]("group"),
			Field[selfJoinEvent, string]("group"),
		),
	).Select(
		SelectLeft("left", Field[selfJoinEvent, string]("id")),
		SelectRight("right", Field[selfJoinEvent, string]("id")),
	).Query(StatementName("self-join")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())

	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	send := func(event selfJoinEvent) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	send(selfJoinEvent{ID: "A1", Group: "A"})
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("self join first batch = %#v", batches)
	}
	send(selfJoinEvent{ID: "A2", Group: "A"})
	if len(batches) != 2 || len(batches[1].New) != 3 || len(batches[1].Old) != 0 {
		t.Fatalf("self join second batch = %#v", batches)
	}
	send(selfJoinEvent{ID: "B1", Group: "B"})
	if len(batches) != 3 || len(batches[2].New) != 1 || len(batches[2].Old) != 0 {
		t.Fatalf("self join third batch = %#v", batches)
	}

	rows := make([]string, 0, 5)
	for _, batch := range batches {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("self join result is not a row: %#v", result)
			}
			rows = append(rows, fmt.Sprintf("%s/%s", row.Get("left").Any(), row.Get("right").Any()))
		}
	}
	want := []string{"A1/A1", "A1/A2", "A2/A1", "A2/A2", "B1/B1"}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("self join rows = %#v, want %#v", rows, want)
	}
}
