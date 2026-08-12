package esper

import (
	"context"
	"testing"
)

// TestViewUnionLengthUniqueMatchesEsper covers ViewUnionSceneOne:
// union(length(1), unique(theString)) retains an event while any child
// retains it. The length(1) child keeps only the latest event; the unique
// child keeps the latest per key. When length(1) evicts an event but unique
// still retains it, the event stays.
func TestViewUnionLengthUniqueMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean").
		Window(UnionWindows(
			LengthWindow(1),
			Unique(Field[viewParityBean, string]("theString")),
		))
	plan, err := env.Build(Select(source,
		Alias("theString", Field[viewParityBean, string]("theString")),
		Alias("intPrimitive", Field[viewParityBean, int]("intPrimitive")),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	stmt := dep.Statements()[0]
	var batches []ResultBatch
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	lastNewStrings := func() []string {
		if len(batches) == 0 {
			return nil
		}
		result := make([]string, 0)
		for _, r := range batches[len(batches)-1].New {
			result = append(result, r.Get("theString").Any().(string))
		}
		return result
	}

	// E1 arrives: both children retain it -> new only
	sendViewBean(t, engine, "E1", 1)
	if len(batches) != 1 || len(batches[0].New) != 1 || len(batches[0].Old) != 0 {
		t.Fatalf("batch 1 = %d new/%d old", len(batches[0].New), len(batches[0].Old))
	}
	if lastNewStrings()[0] != "E1" {
		t.Fatalf("batch 1 new = %v", lastNewStrings())
	}

	// E2 arrives: length(1) evicts E1 but unique still retains E1 -> only E2 is new
	sendViewBean(t, engine, "E2", 2)
	if len(batches) != 2 || len(batches[1].New) != 1 {
		t.Fatalf("batch 2 = %d new", len(batches[1].New))
	}
	if lastNewStrings()[0] != "E2" {
		t.Fatalf("batch 2 new = %v", lastNewStrings())
	}
	// E1 still in unique, so no old event
	if len(batches[1].Old) != 0 {
		t.Fatalf("batch 2 old = %d (E1 still retained by unique)", len(batches[1].Old))
	}

	// Snapshot should show both E1 and E2 retained
	snap, err := stmt.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Results()) < 2 {
		t.Fatalf("snapshot size = %d, want >= 2 (union retains both)", len(snap.Results()))
	}
}
