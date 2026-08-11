package esper

import (
	"context"
	"testing"
	"time"
)

// TestViewFirstTimeSimpleParity covers ViewFirstTimeSimple: firsttime(1 month)
// retains events until the deployment-time deadline; events arriving after
// the deadline are silently ignored and the iterator keeps the retained rows.
func TestViewFirstTimeSimpleParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	anchor := time.Date(2002, 2, 1, 9, 0, 0, 0, time.UTC)
	advanceViewTime(t, engine, anchor.UnixMilli())
	source := From[viewParityBean](env, "SupportBean").Window(FirstTimeCalendar(0, 1, 0))
	stmt, batches := deployViewParity(t, env, engine, source, "s0")

	// Deployment at 2002-02-01 anchors the 1-month deadline.
	sendViewBean(t, engine, "E1", 1)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 {
		t.Fatalf("E1: batches = %#v", *batches)
	}
	if got := snapshotStrings(t, stmt); len(got) != 1 || got[0] != "E1" {
		t.Fatalf("iterator after E1 = %v", got)
	}

	// Before the deadline: E2 still accepted.
	advanceViewTime(t, engine, time.Date(2002, 2, 15, 9, 0, 0, 0, time.UTC).UnixMilli())
	sendViewBean(t, engine, "E2", 2)
	if len(*batches) != 2 || len((*batches)[1].New) != 1 {
		t.Fatalf("E2: batches = %#v", *batches)
	}

	// One millisecond before the deadline: E3 still accepted.
	deadline := time.Date(2002, 3, 1, 9, 0, 0, 0, time.UTC)
	advanceViewTime(t, engine, deadline.UnixMilli()-1)
	sendViewBean(t, engine, "E3", 3)
	if len(*batches) != 3 || len((*batches)[2].New) != 1 {
		t.Fatalf("E3: batches = %#v", *batches)
	}

	// At the deadline: the window closes; later events are ignored.
	advanceViewTime(t, engine, deadline.UnixMilli())
	sendViewBean(t, engine, "E4", 4)
	if len(*batches) != 3 {
		t.Fatalf("E4 after deadline invoked listener: %#v", *batches)
	}
	got := snapshotStrings(t, stmt)
	if len(got) != 3 || got[0] != "E1" || got[1] != "E2" || got[2] != "E3" {
		t.Fatalf("iterator after close = %v", got)
	}
}

// TestViewFirstTimeSceneOneParity covers ViewFirstTimeSceneOne:
// firsttime(10 sec) with projection; events before the deployment-anchored
// deadline are retained and reported, later events never invoke the listener.
func TestViewFirstTimeSceneOneParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 0)
	source := From[viewParityBean](env, "SupportBean").Window(FirstTime(10 * time.Second))
	stmt, batches := deployViewParity(t, env, engine, source, "s0")

	if got := snapshotStrings(t, stmt); len(got) != 0 {
		t.Fatalf("initial iterator = %v", got)
	}

	sendViewBean(t, engine, "E1", 1)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 {
		t.Fatalf("E1: batches = %#v", *batches)
	}
	if got := snapshotStrings(t, stmt); len(got) != 1 || got[0] != "E1" {
		t.Fatalf("iterator after E1 = %v", got)
	}

	advanceViewTime(t, engine, 2000)
	sendViewBean(t, engine, "E2", 20)
	if len(*batches) != 2 || len((*batches)[1].New) != 1 {
		t.Fatalf("E2: batches = %#v", *batches)
	}

	advanceViewTime(t, engine, 9999)
	if len(*batches) != 2 {
		t.Fatalf("t=9999 invoked listener: %#v", *batches)
	}

	// t=10000 closes the window; the iterator still holds E1/E2.
	advanceViewTime(t, engine, 10000)
	sendViewBean(t, engine, "E3", 30)
	sendViewBean(t, engine, "E4", 40)
	if len(*batches) != 2 {
		t.Fatalf("E3/E4 after close invoked listener: %#v", *batches)
	}
	got := snapshotStrings(t, stmt)
	if len(got) != 2 || got[0] != "E1" || got[1] != "E2" {
		t.Fatalf("iterator after close = %v", got)
	}
}

// TestViewFirstTimeSceneTwoParity covers ViewFirstTimeSceneTwo:
// firsttime(1 sec) over the market-data shape with sub-second timing and
// iterator retention after the deadline.
func TestViewFirstTimeSceneTwoParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 0)
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(FirstTime(time.Second))
	stmt, batches := deployViewMarketData(t, env, engine, source, "s0")

	advanceViewTime(t, engine, 500)
	sendViewMarketData(t, engine, "E1", 1)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 {
		t.Fatalf("E1: batches = %#v", *batches)
	}
	if got := snapshotSymbols(t, stmt); len(got) != 1 || got[0] != "E1" {
		t.Fatalf("iterator after E1 = %v", got)
	}

	advanceViewTime(t, engine, 600)
	sendViewMarketData(t, engine, "E2", 2)
	if len(*batches) != 2 || len((*batches)[1].New) != 1 {
		t.Fatalf("E2: batches = %#v", *batches)
	}
	if got := snapshotSymbols(t, stmt); len(got) != 2 || got[0] != "E1" || got[1] != "E2" {
		t.Fatalf("iterator after E2 = %v", got)
	}

	advanceViewTime(t, engine, 1500)
	if len(*batches) != 2 {
		t.Fatalf("t=1500 invoked listener: %#v", *batches)
	}

	advanceViewTime(t, engine, 1600)
	sendViewMarketData(t, engine, "E3", 3)
	if len(*batches) != 2 {
		t.Fatalf("E3 after close invoked listener: %#v", *batches)
	}

	advanceViewTime(t, engine, 2000)
	sendViewMarketData(t, engine, "E4", 4)
	if len(*batches) != 2 {
		t.Fatalf("E4 after close invoked listener: %#v", *batches)
	}
	if got := snapshotSymbols(t, stmt); len(got) != 2 || got[0] != "E1" || got[1] != "E2" {
		t.Fatalf("iterator after close = %v", got)
	}
}
