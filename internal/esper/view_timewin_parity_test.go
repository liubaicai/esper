package esper

import (
	"context"
	"testing"
	"time"
)

func advanceViewTime(t *testing.T, engine *Engine, millis int64) {
	t.Helper()
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(millis).UTC()); err != nil {
		t.Fatal(err)
	}
}

// TestViewTimeWindowSceneOneMatchesEsper covers ViewTimeWindowSceneOne:
// select irstream * from SupportBean#time(10 sec). Events expire exactly
// 10 seconds after arrival; expiry at the boundary timestamp delivers the
// removed events via the old stream, and multiple same-instant arrivals
// expire in one old batch.
func TestViewTimeWindowSceneOneMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 0)
	source := From[viewParityBean](env, "SupportBean").Window(TimeWindow(10 * time.Second))
	stmt, batches := deployViewParity(t, env, engine, source, "s0")

	assertIterator := func(want ...string) {
		t.Helper()
		result, err := stmt.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Results()) != len(want) {
			t.Fatalf("iterator size = %d, want %v (%#v)", len(result.Results()), want, result.Results())
		}
		for i, row := range result.Results() {
			if got := row.Get("theString").Any(); got != want[i] {
				t.Fatalf("iterator[%d] = %v, want %s", i, got, want[i])
			}
		}
	}

	assertIterator()

	// t=1000: E1 inserted.
	advanceViewTime(t, engine, 1000)
	sendViewBean(t, engine, "E1", 0)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 || len((*batches)[0].Old) != 0 {
		t.Fatalf("E1: batches = %#v", *batches)
	}
	assertIterator("E1")

	// t=2000: E2 inserted.
	advanceViewTime(t, engine, 2000)
	sendViewBean(t, engine, "E2", 0)
	assertIterator("E1", "E2")

	// t=3000: E3 inserted.
	advanceViewTime(t, engine, 3000)
	sendViewBean(t, engine, "E3", 0)
	assertIterator("E1", "E2", "E3")

	// t=10999: no expiry yet.
	advanceViewTime(t, engine, 10999)
	if len(*batches) != 3 {
		t.Fatalf("t=10999 invoked listener: %#v", *batches)
	}
	assertIterator("E1", "E2", "E3")

	// t=11000: E1 (sent at 1000) expires.
	advanceViewTime(t, engine, 11000)
	if len(*batches) != 4 {
		t.Fatalf("t=11000: batches = %d, want 4", len(*batches))
	}
	if len((*batches)[3].Old) != 1 || (*batches)[3].Old[0].Get("theString").Any() != "E1" {
		t.Fatalf("t=11000 old = %#v, want E1", (*batches)[3].Old)
	}
	assertIterator("E2", "E3")

	// t=12000: E2 expires; E4/E5 insert at the same instant.
	advanceViewTime(t, engine, 12000)
	if len(*batches) != 5 || (*batches)[4].Old[0].Get("theString").Any() != "E2" {
		t.Fatalf("t=12000: batches = %#v", *batches)
	}
	assertIterator("E3")
	sendViewBean(t, engine, "E4", 0)
	sendViewBean(t, engine, "E5", 0)
	assertIterator("E3", "E4", "E5")

	// t=13000: E3 expires.
	advanceViewTime(t, engine, 13000)
	if got := (*batches)[len(*batches)-1].Old[0].Get("theString").Any(); got != "E3" {
		t.Fatalf("t=13000 old = %v, want E3", got)
	}
	assertIterator("E4", "E5")

	// t=22000: E4 and E5 (both sent at 12000) expire in one old batch.
	advanceViewTime(t, engine, 22000)
	last := (*batches)[len(*batches)-1]
	if len(last.New) != 0 {
		t.Fatalf("t=22000 new = %#v, want none", last.New)
	}
	if len(last.Old) != 2 || last.Old[0].Get("theString").Any() != "E4" || last.Old[1].Get("theString").Any() != "E5" {
		t.Fatalf("t=22000 old = %#v, want [E4 E5]", last.Old)
	}
	assertIterator()
}

// TestViewTimeWindowSceneTwoMatchesEsper covers ViewTimeWindowSceneTwo:
// same-instant arrivals share one expiry instant; advancing past it leaves
// only the later arrival.
func TestViewTimeWindowSceneTwoMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 1000)
	source := From[viewParityBean](env, "SupportBean").Window(TimeWindow(10 * time.Second))
	stmt, _ := deployViewParity(t, env, engine, source, "s0")

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

	sendViewBean(t, engine, "E1", 0)
	sendViewBean(t, engine, "E2", 0)
	assertIterator("E1", "E2")

	sendViewBean(t, engine, "E3", 0)
	sendViewBean(t, engine, "E4", 0)
	assertIterator("E1", "E2", "E3", "E4")

	advanceViewTime(t, engine, 2000)
	sendViewBean(t, engine, "E5", 0)
	assertIterator("E1", "E2", "E3", "E4", "E5")

	// t=10999: nothing expired.
	advanceViewTime(t, engine, 10999)
	assertIterator("E1", "E2", "E3", "E4", "E5")

	// t=11000: E1..E4 (sent at 1000) expire together; only E5 remains.
	advanceViewTime(t, engine, 11000)
	assertIterator("E5")
	assertIterator("E5")
}
