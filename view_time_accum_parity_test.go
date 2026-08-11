package esper

import (
	"context"
	"testing"
	"time"
)

// TestViewTimeAccumSceneOneParity covers ViewTimeAccumSceneOne: time_accum
// retains events until the expiry of the most recent event's deadline; the
// whole window then expires as one old batch. New events reschedule the
// window deadline (the newest event owns expiry).
func TestViewTimeAccumSceneOneParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 1000)
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeAccum(10 * time.Second))
	_, batches := deployViewMarketData(t, env, engine, source, "s0")

	advanceViewTime(t, engine, 11000)
	if len(*batches) != 0 {
		t.Fatalf("t=11000 invoked listener: %#v", *batches)
	}

	sendViewMarketData(t, engine, "E1", 1)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 || len((*batches)[0].Old) != 0 {
		t.Fatalf("E1: batches = %#v", *batches)
	}

	advanceViewTime(t, engine, 15000)
	sendViewMarketData(t, engine, "E2", 2)
	advanceViewTime(t, engine, 15000)
	sendViewMarketData(t, engine, "E3", 3)
	advanceViewTime(t, engine, 24000)
	sendViewMarketData(t, engine, "E4", 4)

	// E4 arrived at t=24000, so expiry moves to t=34000.
	advanceViewTime(t, engine, 33999)
	if len(*batches) != 4 {
		t.Fatalf("t=33999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 34000)
	if len(*batches) != 5 || len((*batches)[4].New) != 0 || len((*batches)[4].Old) != 4 {
		t.Fatalf("t=34000: batch = %#v", (*batches)[4])
	}
	want := []string{"E1", "E2", "E3", "E4"}
	for i, sym := range want {
		if got := (*batches)[4].Old[i].Get("symbol").Any(); got != sym {
			t.Fatalf("old[%d] = %v, want %s", i, got, sym)
		}
	}

	// No further expiry without events.
	advanceViewTime(t, engine, 51000)
	if len(*batches) != 5 {
		t.Fatalf("t=51000 invoked listener: %#v", *batches)
	}

	// E5/E6 arrive at 56000; their expiry owns the window at 66000.
	advanceViewTime(t, engine, 56000)
	sendViewMarketData(t, engine, "E5", 5)
	sendViewMarketData(t, engine, "E6", 6)
	advanceViewTime(t, engine, 65999)
	if len(*batches) != 7 {
		t.Fatalf("t=65999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 66000)
	if len(*batches) != 8 || len((*batches)[7].New) != 0 || len((*batches)[7].Old) != 2 {
		t.Fatalf("t=66000: batch = %#v", (*batches)[7])
	}
	if got := (*batches)[7].Old[0].Get("symbol").Any(); got != "E5" {
		t.Fatalf("old[0] = %v, want E5", got)
	}
	if got := (*batches)[7].Old[1].Get("symbol").Any(); got != "E6" {
		t.Fatalf("old[1] = %v, want E6", got)
	}

	// Next window: E7 at 76000 (after a quiet gap), E8 at 85000.
	advanceViewTime(t, engine, 76000)
	sendViewMarketData(t, engine, "E7", 7)
	advanceViewTime(t, engine, 85000)
	sendViewMarketData(t, engine, "E8", 8)
	advanceViewTime(t, engine, 94999)
	if len(*batches) != 10 {
		t.Fatalf("t=94999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 95000)
	if len(*batches) != 11 || len((*batches)[10].New) != 0 || len((*batches)[10].Old) != 2 {
		t.Fatalf("t=95000: batch = %#v", (*batches)[10])
	}
	if got := (*batches)[10].Old[0].Get("symbol").Any(); got != "E7" {
		t.Fatalf("old[0] = %v, want E7", got)
	}
	if got := (*batches)[10].Old[1].Get("symbol").Any(); got != "E8" {
		t.Fatalf("old[1] = %v, want E8", got)
	}
}

// TestViewTimeAccumSceneTwoParity covers ViewTimeAccumSceneTwo: insert stream
// on arrival, iterator sees the whole accumulated window, and the old stream
// carries the entire window on expiry.
func TestViewTimeAccumSceneTwoParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 1000)
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeAccum(10 * time.Second))
	stmt, batches := deployViewMarketData(t, env, engine, source, "s0")

	if got := snapshotSymbols(t, stmt); len(got) != 0 {
		t.Fatalf("initial iterator = %v", got)
	}

	advanceViewTime(t, engine, 1000)
	sendViewMarketData(t, engine, "E1", 1)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 || (*batches)[0].New[0].Get("symbol").Any() != "E1" {
		t.Fatalf("E1: batches = %#v", *batches)
	}

	advanceViewTime(t, engine, 5000)
	sendViewMarketData(t, engine, "E2", 2)
	if len(*batches) != 2 || (*batches)[1].New[0].Get("symbol").Any() != "E2" {
		t.Fatalf("E2: batches = %#v", *batches)
	}

	advanceViewTime(t, engine, 14999)
	if len(*batches) != 2 {
		t.Fatalf("t=14999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 15000)
	if len(*batches) != 3 || len((*batches)[2].New) != 0 || len((*batches)[2].Old) != 2 {
		t.Fatalf("t=15000: batch = %#v", (*batches)[2])
	}
	if got := snapshotSymbols(t, stmt); len(got) != 0 {
		t.Fatalf("iterator after expiry = %v", got)
	}

	advanceViewTime(t, engine, 31000)
	sendViewMarketData(t, engine, "E3", 3)
	sendViewMarketData(t, engine, "E4", 4)
	if len(*batches) != 5 {
		t.Fatalf("E3/E4: batches = %#v", *batches)
	}

	advanceViewTime(t, engine, 40999)
	if len(*batches) != 5 {
		t.Fatalf("t=40999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 41000)
	if len(*batches) != 6 || len((*batches)[5].Old) != 2 {
		t.Fatalf("t=41000: batch = %#v", (*batches)[5])
	}

	sendViewMarketData(t, engine, "E5", 5)
	if len(*batches) != 7 || (*batches)[6].New[0].Get("symbol").Any() != "E5" {
		t.Fatalf("E5: batches = %#v", *batches)
	}
	advanceViewTime(t, engine, 41000)
	sendViewMarketData(t, engine, "E6", 6)
	advanceViewTime(t, engine, 49000)
	sendViewMarketData(t, engine, "E7", 7)
	advanceViewTime(t, engine, 59000)
	if len(*batches) != 10 || len((*batches)[9].New) != 0 || len((*batches)[9].Old) != 3 {
		t.Fatalf("t=59000: batch = %#v", (*batches)[9])
	}
	for i, sym := range []string{"E5", "E6", "E7"} {
		if got := (*batches)[9].Old[i].Get("symbol").Any(); got != sym {
			t.Fatalf("old[%d] = %v, want %s", i, got, sym)
		}
	}
}

// TestViewTimeAccumSceneThreeParity covers ViewTimeAccumSceneThree: the
// support-bean shape with iterator assertions at each milestone and a quiet
// final stretch after the window expires.
func TestViewTimeAccumSceneThreeParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 1000)
	source := From[viewParityBean](env, "SupportBean").Window(TimeAccum(10 * time.Second))
	stmt, batches := deployViewParity(t, env, engine, source, "s0")

	assertIter := func(want ...string) {
		t.Helper()
		got := snapshotStrings(t, stmt)
		if len(got) != len(want) {
			t.Fatalf("iterator = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("iterator[%d] = %v, want %s", i, got[i], want[i])
			}
		}
	}
	assertIter()

	sendViewBean(t, engine, "E1", 1)
	assertIter("E1")

	advanceViewTime(t, engine, 5000)
	sendViewBean(t, engine, "E2", 2)
	assertIter("E1", "E2")

	advanceViewTime(t, engine, 14999)
	if len(*batches) != 2 {
		t.Fatalf("t=14999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 15000)
	if len(*batches) != 3 || len((*batches)[2].New) != 0 || len((*batches)[2].Old) != 2 {
		t.Fatalf("t=15000: batch = %#v", (*batches)[2])
	}
	assertIter()

	advanceViewTime(t, engine, 18000)
	sendViewBean(t, engine, "E3", 3)
	sendViewBean(t, engine, "E4", 4)
	assertIter("E3", "E4")
	advanceViewTime(t, engine, 19000)
	sendViewBean(t, engine, "E5", 5)
	advanceViewTime(t, engine, 28999)
	if len(*batches) != 6 {
		t.Fatalf("t=28999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 29000)
	if len(*batches) != 7 || len((*batches)[6].New) != 0 || len((*batches)[6].Old) != 3 {
		t.Fatalf("t=29000: batch = %#v", (*batches)[6])
	}
	for i, sym := range []string{"E3", "E4", "E5"} {
		if got := (*batches)[6].Old[i].Get("theString").Any(); got != sym {
			t.Fatalf("old[%d] = %v, want %s", i, got, sym)
		}
	}
	assertIter()

	advanceViewTime(t, engine, 39000)
	advanceViewTime(t, engine, 99000)
	if len(*batches) != 7 {
		t.Fatalf("quiet stretch invoked listener: %#v", *batches)
	}
}

func snapshotStrings(t *testing.T, stmt *Statement) []string {
	t.Helper()
	result, err := stmt.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	values := make([]string, 0, len(result.Results()))
	for _, row := range result.Results() {
		values = append(values, row.Get("theString").Any().(string))
	}
	return values
}

// TestViewTimeAccumRStreamParity covers ViewTimeAccumRStream: with only the
// remove stream selected, window expiry still delivers the batch (as new data
// in rstream projection).
func TestViewTimeAccumRStreamParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 1000)
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeAccum(10 * time.Second))
	plan, err := env.Build(source.Query(StatementName("s0"), WithRemoveStreamOnly()))
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

	advanceViewTime(t, engine, 11000)
	if len(*batches) != 0 {
		t.Fatalf("t=11000 invoked listener: %#v", *batches)
	}

	sendViewMarketData(t, engine, "E1", 1)
	sendViewMarketData(t, engine, "E2", 2)
	sendViewMarketData(t, engine, "E3", 3)
	if len(*batches) != 0 {
		t.Fatalf("arrivals invoked listener: %#v", *batches)
	}

	advanceViewTime(t, engine, 21000)
	if len(*batches) != 1 {
		t.Fatalf("t=21000: batches = %d, want 1", len(*batches))
	}
	if len((*batches)[0].New) != 3 {
		t.Fatalf("rstream new = %#v, want 3 events", (*batches)[0].New)
	}
	for i, sym := range []string{"E1", "E2", "E3"} {
		if got := (*batches)[0].New[i].Get("symbol").Any(); got != sym {
			t.Fatalf("new[%d] = %v, want %s", i, got, sym)
		}
	}
}
