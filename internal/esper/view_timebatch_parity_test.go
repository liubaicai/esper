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

// TestViewTimeBatchMultirowMatchesEsper covers ViewTimeBatchMultirow:
// multi-event batches accumulate across advances; a boundary with an empty
// current batch flushes the previous batch as old; after two empty batches
// the listener goes quiet, and a late event starts a fresh batch whose old
// stream is empty.
func TestViewTimeBatchMultirowMatchesEsper(t *testing.T) {
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

	advanceViewTime(t, engine, 1000)
	sendViewBean(t, engine, "E1", 0)
	sendViewBean(t, engine, "E2", 0)
	if len(*batches) != 0 {
		t.Fatalf("emitted before boundary: %#v", *batches)
	}
	assertIterator("E1", "E2")

	advanceViewTime(t, engine, 2000)
	sendViewBean(t, engine, "E3", 0)
	assertIterator("E1", "E2", "E3")

	advanceViewTime(t, engine, 3000)
	sendViewBean(t, engine, "E4", 0)
	assertIterator("E1", "E2", "E3", "E4")

	// t=11000: new=[E1..E4], old=null.
	advanceViewTime(t, engine, 11000)
	if len(*batches) != 1 || len((*batches)[0].New) != 4 || len((*batches)[0].Old) != 0 {
		t.Fatalf("t=11000: batch = %#v", (*batches)[0])
	}

	// t=21000: empty current batch, old=[E1..E4].
	advanceViewTime(t, engine, 21000)
	if len(*batches) != 2 || len((*batches)[1].New) != 0 || len((*batches)[1].Old) != 4 {
		t.Fatalf("t=21000: batch = %#v", (*batches)[1])
	}

	// t=31000: quiet.
	advanceViewTime(t, engine, 31000)
	if len(*batches) != 2 {
		t.Fatalf("t=31000 invoked listener: %#v", *batches)
	}

	// Late E5 starts a fresh batch; its flush has no old rows.
	sendViewBean(t, engine, "E5", 0)
	assertIterator("E5")
	advanceViewTime(t, engine, 41000)
	if len(*batches) != 3 || len((*batches)[2].New) != 1 || len((*batches)[2].Old) != 0 {
		t.Fatalf("t=41000: batch = %#v", (*batches)[2])
	}
	if got := (*batches)[2].New[0].Get("theString").Any(); got != "E5" {
		t.Fatalf("t=41000 new = %v, want E5", got)
	}
}

// TestViewTimeBatchMultiBatchMatchesEsper covers ViewTimeBatchMultiBatch:
// consecutive non-empty batches each report the previous batch as old data,
// ending with one old-only flush and then silence.
func TestViewTimeBatchMultiBatchMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 0)
	source := From[viewParityBean](env, "SupportBean").Window(TimeBatch(10 * time.Second))
	_, batches := deployViewParity(t, env, engine, source, "s0")

	assertIR := func(index int, newSyms, oldSyms []string) {
		t.Helper()
		if len(*batches) != index+1 {
			t.Fatalf("batches = %d, want %d", len(*batches), index+1)
		}
		batch := (*batches)[index]
		if len(batch.New) != len(newSyms) || len(batch.Old) != len(oldSyms) {
			t.Fatalf("batch %d = %d new/%d old, want %v/%v", index, len(batch.New), len(batch.Old), newSyms, oldSyms)
		}
		for i, sym := range newSyms {
			if got := batch.New[i].Get("theString").Any(); got != sym {
				t.Fatalf("batch %d new[%d] = %v, want %s", index, i, got, sym)
			}
		}
		for i, sym := range oldSyms {
			if got := batch.Old[i].Get("theString").Any(); got != sym {
				t.Fatalf("batch %d old[%d] = %v, want %s", index, i, got, sym)
			}
		}
	}

	advanceViewTime(t, engine, 1000)
	sendViewBean(t, engine, "E1", 0)
	sendViewBean(t, engine, "E2", 0)
	if len(*batches) != 0 {
		t.Fatalf("emitted before boundary: %#v", *batches)
	}

	advanceViewTime(t, engine, 11000)
	assertIR(0, []string{"E1", "E2"}, nil)

	sendViewBean(t, engine, "E3", 0)
	sendViewBean(t, engine, "E4", 0)
	advanceViewTime(t, engine, 21000)
	assertIR(1, []string{"E3", "E4"}, []string{"E1", "E2"})

	sendViewBean(t, engine, "E5", 0)
	sendViewBean(t, engine, "E6", 0)
	advanceViewTime(t, engine, 31000)
	assertIR(2, []string{"E5", "E6"}, []string{"E3", "E4"})

	sendViewBean(t, engine, "E7", 0)
	sendViewBean(t, engine, "E8", 0)
	advanceViewTime(t, engine, 41000)
	assertIR(3, []string{"E7", "E8"}, []string{"E5", "E6"})

	advanceViewTime(t, engine, 51000)
	assertIR(4, nil, []string{"E7", "E8"})

	advanceViewTime(t, engine, 61000)
	if len(*batches) != 5 {
		t.Fatalf("t=61000 invoked listener: %#v", *batches)
	}
}

// TestViewTimeBatchNoRefPointMatchesEsper covers ViewTimeBatchNoRefPoint:
// without a reference point, a single event at t=0 anchors the batch; the
// flush fires at exactly anchor+period.
func TestViewTimeBatchNoRefPointMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean").Window(TimeBatch(10 * time.Minute))
	_, batches := deployViewParity(t, env, engine, source, "s0")

	advanceViewTime(t, engine, 0)
	sendViewBean(t, engine, "E1", 0)

	advanceViewTime(t, engine, 10*60*1000-1)
	if len(*batches) != 0 {
		t.Fatalf("emitted before anchor+period: %#v", *batches)
	}

	advanceViewTime(t, engine, 10*60*1000)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 {
		t.Fatalf("t=600000: batches = %#v", *batches)
	}
	if got := (*batches)[0].New[0].Get("theString").Any(); got != "E1" {
		t.Fatalf("flush new = %v, want E1", got)
	}
}

// TestViewTimeBatchLongerMatchesEsper covers ViewTimeBatchLonger: a 20-boundary
// stability walk with varying batch sizes (the Java execution uses a random
// per-boundary event count and asserts no values; the Go port drives the same
// lifecycle with a deterministic count pattern).
func TestViewTimeBatchLongerMatchesEsper(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeBatch(time.Second))
	_, batches := deployViewMarketData(t, env, engine, source, "s0")

	count := 0
	for i := 0; i < 20; i++ {
		// Deterministic stand-in for Java's random.nextInt() % 10 (>6 -> 0):
		// exercises empty and non-empty batches across 20 boundaries.
		numEvents := (i*7 + 3) % 10
		if numEvents > 6 {
			numEvents = 0
		}
		advanceViewTime(t, engine, int64(i*1000))
		for j := 0; j < numEvents; j++ {
			sendViewMarketData(t, engine, "E_"+string(rune('A'+count%26)), 0)
			count++
		}
	}

	// Every delivered batch must be consistent: new rows are the events since
	// the previous flush, old rows are the previous batch.
	previousFlushed := 0
	seenTotal := 0
	for _, batch := range *batches {
		if len(batch.Old) != previousFlushed {
			t.Fatalf("old rows = %d, want previous batch size %d", len(batch.Old), previousFlushed)
		}
		seenTotal += len(batch.New)
		previousFlushed = len(batch.New)
	}
	if seenTotal+previousFlushed == 0 && count > 0 {
		t.Fatalf("no batches delivered for %d events", count)
	}
}
