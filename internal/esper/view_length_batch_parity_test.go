package esper

import (
	"context"
	"testing"
)

// TestViewLengthBatchSceneOneMatchesEsper covers ViewLengthBatchSceneOne:
// length_batch(3) accumulates 3 events then flushes them all as new data
// (old=null). The next batch accumulates independently. Iterator shows the
// current accumulating batch.
func TestViewLengthBatchSceneOneMatchesEsper(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").
		Window(LengthBatch(3))
	plan, err := env.Build(Select(source,
		Alias("symbol", Field[viewUniqueMarketData, string]("symbol")),
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

	sendMD := func(s string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), viewUniqueMarketData{Symbol: s}); err != nil {
			t.Fatal(err)
		}
	}
	syms := func(batch ResultBatch) []string {
		result := make([]string, 0, len(batch.New))
		for _, r := range batch.New {
			result = append(result, r.Get("symbol").Any().(string))
		}
		return result
	}
	oldSyms := func(batch ResultBatch) []string {
		result := make([]string, 0, len(batch.Old))
		for _, r := range batch.Old {
			result = append(result, r.Get("symbol").Any().(string))
		}
		return result
	}

	sendMD("E1")
	sendMD("E2")
	if len(batches) != 0 {
		t.Fatal("should not flush before batch size")
	}

	sendMD("E3")
	if len(batches) != 1 || len(batches[0].New) != 3 || len(batches[0].Old) != 0 {
		t.Fatalf("batch 1 = %d new/%d old", len(batches[0].New), len(batches[0].Old))
	}
	got := syms(batches[0])
	if got[0] != "E1" || got[1] != "E2" || got[2] != "E3" {
		t.Fatalf("batch 1 new syms = %v", got)
	}

	sendMD("E4")
	sendMD("E5")
	if len(batches) != 1 {
		t.Fatal("should not flush before second batch fills")
	}

	// Iterator shows current accumulating batch
	snap, err := stmt.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Results()) != 2 {
		t.Fatalf("iterator size = %d, want 2", len(snap.Results()))
	}

	sendMD("E6")
	if len(batches) != 2 || len(batches[1].New) != 3 || len(batches[1].Old) != 3 {
		t.Fatalf("batch 2 = %d new/%d old", len(batches[1].New), len(batches[1].Old))
	}
	got = syms(batches[1])
	if got[0] != "E4" || got[1] != "E5" || got[2] != "E6" {
		t.Fatalf("batch 2 new syms = %v", got)
	}
	old := oldSyms(batches[1])
	if old[0] != "E1" || old[1] != "E2" || old[2] != "E3" {
		t.Fatalf("batch 2 old syms = %v", old)
	}
}

// TestViewLengthBatchSize2MatchesEsper covers ViewLengthBatchSize2:
// length_batch(2) with alternating flush and previous batch as old data.
func TestViewLengthBatchSize2MatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean").
		Window(LengthBatch(2))
	plan, err := env.Build(source.Query(StatementName("s0"), WithOldStream()))
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

	sendViewBean(t, engine, "E1", 1)
	if len(batches) != 0 {
		t.Fatal("E1 should not flush")
	}
	sendViewBean(t, engine, "E2", 2)
	if len(batches) != 1 || len(batches[0].New) != 2 || len(batches[0].Old) != 0 {
		t.Fatalf("batch 1 = %d new/%d old", len(batches[0].New), len(batches[0].Old))
	}

	sendViewBean(t, engine, "E3", 3)
	if len(batches) != 1 {
		t.Fatal("E3 should not flush")
	}
	sendViewBean(t, engine, "E4", 4)
	if len(batches) != 2 || len(batches[2-1].New) != 2 || len(batches[1].Old) != 2 {
		t.Fatalf("batch 2 = %d new/%d old", len(batches[1].New), len(batches[1].Old))
	}
}

// TestViewLengthBatchSize1MatchesEsper covers ViewLengthBatchSize1:
// length_batch(1) flushes every event immediately, with the previous event
// as old data.
func TestViewLengthBatchSize1MatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean").
		Window(LengthBatch(1))
	plan, err := env.Build(source.Query(StatementName("s0"), WithOldStream()))
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

	sendViewBean(t, engine, "E1", 1)
	if len(batches) != 1 || len(batches[0].New) != 1 || len(batches[0].Old) != 0 {
		t.Fatalf("batch 1 = %d new/%d old", len(batches[0].New), len(batches[0].Old))
	}
	sendViewBean(t, engine, "E2", 2)
	if len(batches) != 2 || len(batches[1].New) != 1 || len(batches[1].Old) != 1 {
		t.Fatalf("batch 2 = %d new/%d old", len(batches[1].New), len(batches[1].Old))
	}
}
