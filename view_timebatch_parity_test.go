package esper

import (
	"context"
	"testing"
	"time"
)

// TestViewTimeBatch10SecMatchesEsper covers ViewTimeBatch10Sec:
// select irstream * from SupportBean#time_batch(10 sec). Without a reference
// point the batch anchors at the first event arrival; each flush delivers the
// current batch as new and the previous batch as old; a flush with an empty
// current batch still delivers the previous batch as old data once.
func TestViewTimeBatch10SecMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 0)
	source := From[viewParityBean](env, "SupportBean").Window(TimeBatch(10 * time.Second))
	stmt, batches := deployViewParity(t, env, engine, source, "s0")

	assertIterator := func(want ...string) {
		t.Helper()
		result, err := stmt.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Results()) != len(want) {
			t.Fatalf("iterator size = %d, want %v", len(result.Results()), want)
		}
		for i, row := range result.Results() {
			if got := row.Get("theString").Any(); got != want[i] {
				t.Fatalf("iterator[%d] = %v, want %s", i, got, want[i])
			}
		}
	}

	assertIterator()

	// t=1000: E1 anchors the batch (first event sets the reference point).
	advanceViewTime(t, engine, 1000)
	sendViewBean(t, engine, "E1", 0)
	if len(*batches) != 0 {
		t.Fatalf("time batch emitted before boundary: %#v", *batches)
	}
	assertIterator("E1")

	advanceViewTime(t, engine, 2000)
	sendViewBean(t, engine, "E2", 0)
	if len(*batches) != 0 {
		t.Fatalf("time batch emitted before boundary: %#v", *batches)
	}
	assertIterator("E1", "E2")

	// t=10999: just before the boundary (anchor 1000 + 10s).
	advanceViewTime(t, engine, 10999)
	if len(*batches) != 0 {
		t.Fatalf("time batch emitted at t=10999: %#v", *batches)
	}

	// t=11000: first flush, new=[E1,E2], no old.
	advanceViewTime(t, engine, 11000)
	if len(*batches) != 1 {
		t.Fatalf("t=11000: batches = %d, want 1", len(*batches))
	}
	if len((*batches)[0].New) != 2 || (*batches)[0].New[0].Get("theString").Any() != "E1" || (*batches)[0].New[1].Get("theString").Any() != "E2" {
		t.Fatalf("t=11000 new = %#v, want [E1 E2]", (*batches)[0].New)
	}
	if len((*batches)[0].Old) != 0 {
		t.Fatalf("t=11000 old = %#v, want none", (*batches)[0].Old)
	}
	assertIterator()

	// E3 buffers into the next batch.
	sendViewBean(t, engine, "E3", 0)
	if len(*batches) != 1 {
		t.Fatalf("E3 emitted before boundary: %#v", *batches)
	}
	assertIterator("E3")

	// t=21000: new=[E3], old=[E1,E2].
	advanceViewTime(t, engine, 21000)
	if len(*batches) != 2 {
		t.Fatalf("t=21000: batches = %d, want 2", len(*batches))
	}
	if len((*batches)[1].New) != 1 || (*batches)[1].New[0].Get("theString").Any() != "E3" {
		t.Fatalf("t=21000 new = %#v, want [E3]", (*batches)[1].New)
	}
	if len((*batches)[1].Old) != 2 || (*batches)[1].Old[0].Get("theString").Any() != "E1" || (*batches)[1].Old[1].Get("theString").Any() != "E2" {
		t.Fatalf("t=21000 old = %#v, want [E1 E2]", (*batches)[1].Old)
	}

	// t=31000: empty current batch still flushes previous batch as old.
	advanceViewTime(t, engine, 31000)
	if len(*batches) != 3 {
		t.Fatalf("t=31000: batches = %d, want 3 (old-only flush)", len(*batches))
	}
	if len((*batches)[2].New) != 0 {
		t.Fatalf("t=31000 new = %#v, want none", (*batches)[2].New)
	}
	if len((*batches)[2].Old) != 1 || (*batches)[2].Old[0].Get("theString").Any() != "E3" {
		t.Fatalf("t=31000 old = %#v, want [E3]", (*batches)[2].Old)
	}

	// t=41000: both batches empty, listener not invoked.
	advanceViewTime(t, engine, 41000)
	if len(*batches) != 3 {
		t.Fatalf("t=41000 invoked listener: %#v", *batches)
	}
}

// TestViewTimeBatchSceneOneMatchesEsper covers ViewTimeBatchSceneOne:
// select irstream * from SupportMarketDataBean#time_batch(1 sec) with
// sub-second advances, iterator visibility of the pending batch and the
// old-only flush lifecycle.
func TestViewTimeBatchSceneOneMatchesEsper(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 0)
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeBatch(time.Second))
	stmt, batches := deployViewMarketData(t, env, engine, source, "s0")

	send := func(symbol string) {
		t.Helper()
		sendViewMarketData(t, engine, symbol, 0)
	}
	assertBatch := func(index int, newSyms []string, oldSyms []string) {
		t.Helper()
		if len(*batches) != index+1 {
			t.Fatalf("batches = %d, want %d", len(*batches), index+1)
		}
		batch := (*batches)[index]
		if len(batch.New) != len(newSyms) {
			t.Fatalf("batch %d new size = %d, want %v", index, len(batch.New), newSyms)
		}
		for i, sym := range newSyms {
			if got := batch.New[i].Get("symbol").Any(); got != sym {
				t.Fatalf("batch %d new[%d] = %v, want %s", index, i, got, sym)
			}
		}
		if len(batch.Old) != len(oldSyms) {
			t.Fatalf("batch %d old size = %d, want %v", index, len(batch.Old), oldSyms)
		}
		for i, sym := range oldSyms {
			if got := batch.Old[i].Get("symbol").Any(); got != sym {
				t.Fatalf("batch %d old[%d] = %v, want %s", index, i, got, sym)
			}
		}
	}

	advanceViewTime(t, engine, 1500)
	if len(*batches) != 0 {
		t.Fatalf("t=1500 invoked listener: %#v", *batches)
	}

	// E1 at t=1500 anchors the batch window.
	send("E1")
	advanceViewTime(t, engine, 1700)
	send("E2")
	advanceViewTime(t, engine, 2499)
	if len(*batches) != 0 {
		t.Fatalf("t=2499 invoked listener: %#v", *batches)
	}

	// t=2500: first flush new=[E1,E2], old=null.
	advanceViewTime(t, engine, 2500)
	assertBatch(0, []string{"E1", "E2"}, nil)

	send("E3")
	send("E4")
	advanceViewTime(t, engine, 2600)
	send("E5")

	// Iterator sees the pending batch.
	if got := snapshotSymbols(t, stmt); len(got) != 3 || got[0] != "E3" || got[1] != "E4" || got[2] != "E5" {
		t.Fatalf("iterator = %v, want [E3 E4 E5]", got)
	}

	// t=3500: new=[E3,E4,E5], old=[E1,E2].
	advanceViewTime(t, engine, 3500)
	assertBatch(1, []string{"E3", "E4", "E5"}, []string{"E1", "E2"})

	// t=4500: empty current batch flushes previous batch as old.
	advanceViewTime(t, engine, 4500)
	assertBatch(2, nil, []string{"E3", "E4", "E5"})

	// t=5500: both empty, not invoked.
	advanceViewTime(t, engine, 5500)
	if len(*batches) != 3 {
		t.Fatalf("t=5500 invoked listener: %#v", *batches)
	}

	// E6 starts a fresh batch.
	send("E6")
	advanceViewTime(t, engine, 6500)
	assertBatch(3, []string{"E6"}, nil)

	// t=7500: old=[E6] only.
	advanceViewTime(t, engine, 7500)
	assertBatch(4, nil, []string{"E6"})
}
