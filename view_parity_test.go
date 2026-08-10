package esper

import (
	"context"
	"testing"
)

type viewParityBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func newViewParityEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[viewParityBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	return env, engine
}

func sendViewBean(t *testing.T, engine *Engine, theString string, intPrim int) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), viewParityBean{TheString: theString, IntPrimitive: intPrim}); err != nil {
		t.Fatal(err)
	}
}

func deployViewParity(t *testing.T, env *Environment, engine *Engine, stream Stream[viewParityBean], name string) (*Statement, *[]ResultBatch) {
	t.Helper()
	plan, err := env.Build(stream.Query(StatementName(name), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := new([]ResultBatch)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return deployment.Statements()[0], batches
}

// TestViewKeepAllSimpleMatchesEsper covers ViewKeepAllSimple:
// keep-all window retains all events; each new event is reported as insert
// with no removes. Iterator sees all retained events.
func TestViewKeepAllSimpleMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean")
	_, batches := deployViewParity(t, env, engine, source.Window(KeepAll()), "s0")

	// Send events one at a time - each should produce a batch with 1 new row
	events := []string{"E1", "E2", "E3", "E4"}
	for i, e := range events {
		sendViewBean(t, engine, e, 0)
		if len(*batches) != i+1 {
			t.Fatalf("after %s: got %d batches, want %d", e, len(*batches), i+1)
		}
		if len((*batches)[i].New) != 1 {
			t.Fatalf("after %s: new rows = %d, want 1", e, len((*batches)[i].New))
		}
		// KeepAll: no removes
		if len((*batches)[i].Old) != 0 {
			t.Fatalf("after %s: old rows = %d, want 0 (keep-all)", e, len((*batches)[i].Old))
		}
	}
}

// TestViewLengthWindowMatchesEsper covers ViewLengthWindowSceneOne:
// length window retains only the last N events; oldest event is removed
// when a new event arrives and the window is full.
func TestViewLengthWindowMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean")
	_, batches := deployViewParity(t, env, engine, source.Window(LengthWindow(3)), "s0")

	// Send 5 events through a length-3 window
	events := []string{"E1", "E2", "E3", "E4", "E5"}
	for i, e := range events {
		sendViewBean(t, engine, e, 0)
		batch := (*batches)[i]
		if len(batch.New) != 1 {
			t.Fatalf("%s: new rows = %d, want 1", e, len(batch.New))
		}
		if i < 3 {
			// Window not yet full, no removes
			if len(batch.Old) != 0 {
				t.Fatalf("%s: old rows = %d, want 0 (window not full)", e, len(batch.Old))
			}
		} else {
			// Window full, oldest removed
			if len(batch.Old) != 1 {
				t.Fatalf("%s: old rows = %d, want 1 (window full)", e, len(batch.Old))
			}
		}
	}
}

// TestViewFirstEventMatchesEsper covers ViewFirstEventSceneOne:
// first-event window keeps only the first event; subsequent events
// do not produce new insert rows (they go to old stream as removes).
func TestViewFirstEventMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean")
	_, batches := deployViewParity(t, env, engine, source.Window(FirstEvent()), "s0")

	// First event should match
	sendViewBean(t, engine, "E1", 0)
	if len(*batches) != 1 {
		t.Fatalf("after E1: got %d batches, want 1", len(*batches))
	}
	if len((*batches)[0].New) != 1 {
		t.Fatalf("E1: new rows = %d, want 1", len((*batches)[0].New))
	}

	// Subsequent events should NOT produce new matches (first-event window)
	sendViewBean(t, engine, "E2", 0)
	if len(*batches) != 1 {
		t.Fatalf("after E2: got %d batches, want 1 (first-event only)", len(*batches))
	}
}

// TestViewLastEventMatchesEsper covers ViewLastEventSceneOne:
// last-event window keeps only the most recent event; each new event
// replaces the previous one (old stream shows the removed event).
func TestViewLastEventMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean")
	_, batches := deployViewParity(t, env, engine, source.Window(LastEvent()), "s0")

	// Each event should produce a batch
	events := []string{"E1", "E2", "E3"}
	for i, e := range events {
		sendViewBean(t, engine, e, 0)
		if len(*batches) != i+1 {
			t.Fatalf("after %s: got %d batches, want %d", e, len(*batches), i+1)
		}
		if len((*batches)[i].New) != 1 {
			t.Fatalf("%s: new rows = %d, want 1", e, len((*batches)[i].New))
		}
		if i > 0 {
			// Previous event removed
			if len((*batches)[i].Old) != 1 {
				t.Fatalf("%s: old rows = %d, want 1 (previous removed)", e, len((*batches)[i].Old))
			}
		}
	}
}

// TestViewFirstLengthMatchesEsper covers ViewFirstLengthSceneOne:
// first-length window keeps only the first N events; subsequent events
// do not produce insert rows.
func TestViewFirstLengthMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean")
	_, batches := deployViewParity(t, env, engine, source.Window(FirstLength(2)), "s0")

	// First two events should match
	sendViewBean(t, engine, "E1", 0)
	sendViewBean(t, engine, "E2", 0)
	if len(*batches) != 2 {
		t.Fatalf("after E1,E2: got %d batches, want 2", len(*batches))
	}

	// Third event should NOT match (first-length window full)
	sendViewBean(t, engine, "E3", 0)
	if len(*batches) != 2 {
		t.Fatalf("after E3: got %d batches, want 2 (first-length full)", len(*batches))
	}
}

// TestEPLOtherIRStreamSelectorMatchesEsper covers
// EPLOtherIStreamRStreamConfigSelectorIRStream: default irstream behavior
// with length window shows both new (inserted) and old (evicted) events.
func TestEPLOtherIRStreamSelectorMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean")
	_, batches := deployViewParity(t, env, engine, source.Window(LengthWindow(3)), "s0")

	// Fill the window (3 events)
	sendViewBean(t, engine, "a", 0)
	sendViewBean(t, engine, "b", 0)
	sendViewBean(t, engine, "c", 0)

	// Reset: send 4th event - 'a' evicted
	sendViewBean(t, engine, "d", 0)

	// Last batch should show new=d, old=a
	lastBatch := (*batches)[len(*batches)-1]
	if len(lastBatch.New) != 1 {
		t.Fatalf("new rows = %d, want 1", len(lastBatch.New))
	}
	if got := lastBatch.New[0].Get("theString").Any(); got != "d" {
		t.Fatalf("new theString=%v, want d", got)
	}

	if len(lastBatch.Old) != 1 {
		t.Fatalf("old rows = %d, want 1", len(lastBatch.Old))
	}
	if got := lastBatch.Old[0].Get("theString").Any(); got != "a" {
		t.Fatalf("old theString=%v, want a (evicted)", got)
	}
}

// TestEPLOtherRStreamSelectorMatchesEsper covers
// EPLOtherIStreamRStreamConfigSelectorRStream: rstream selector reports
// only removed events as new data; inserts produce no output.
func TestEPLOtherRStreamSelectorMatchesEsper(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean")
	plan, err := env.Build(source.Window(LengthWindow(3)).Query(
		StatementName("s0"), WithRemoveStreamOnly(),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}

	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Send a,b,c - rstream does not report inserts, so no output
	sendViewBean(t, engine, "a", 0)
	sendViewBean(t, engine, "b", 0)
	sendViewBean(t, engine, "c", 0)
	if len(batches) != 0 {
		t.Fatalf("rstream should not report inserts, got %d batches", len(batches))
	}

	// Send d - 'a' evicted, rstream reports 'a' via Old (Go models rstream
	// removed events in batch.Old rather than listener newData)
	sendViewBean(t, engine, "d", 0)
	if len(batches) != 1 {
		t.Fatalf("rstream should report eviction, got %d batches", len(batches))
	}
	if len(batches[0].Old) != 1 {
		t.Fatalf("rstream old rows = %d, want 1", len(batches[0].Old))
	}
	if got := batches[0].Old[0].Get("theString").Any(); got != "a" {
		t.Fatalf("rstream old theString=%v, want a (evicted)", got)
	}
}
