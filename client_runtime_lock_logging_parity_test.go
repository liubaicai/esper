package esper

import (
	"context"
	"testing"
)

func TestClientRuntimeLockLoggingParity(t *testing.T) {
	env, _ := newRuntimeTest(t)
	engine := NewEngine(env, WithLockActivityTracing())
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("c0", CountAll()),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E1"}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	results := snapshot.Results()
	if len(results) != 1 || results[0].Get("c0").Any() != int64(1) {
		t.Fatalf("safe snapshot = %#v, want c0=1", results)
	}
	assertClientRuntimeLockActivity(t, engine.LockActivity())

	engine.ClearLockActivity()
	if len(engine.LockActivity()) != 0 {
		t.Fatal("lock activity clear retained events")
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertClientRuntimeLockActivity(t, engine.LockActivity())
}

func assertClientRuntimeLockActivity(t *testing.T, events []LockActivityEvent) {
	t.Helper()
	if len(events) == 0 {
		t.Fatal("lock activity trace is empty")
	}
	counts := map[LockActivityPhase]int{}
	for index, event := range events {
		if index > 0 && event.Sequence <= events[index-1].Sequence {
			t.Fatalf("lock sequence is not increasing: %#v", events)
		}
		counts[event.Phase]++
	}
	if counts[LockActivityAttempt] == 0 || counts[LockActivityAcquired] == 0 || counts[LockActivityReleased] == 0 {
		t.Fatalf("lock activity phases = %#v", counts)
	}
	if counts[LockActivityAttempt] != counts[LockActivityAcquired] || counts[LockActivityAcquired] != counts[LockActivityReleased] {
		t.Fatalf("unbalanced lock activity phases = %#v", counts)
	}
}
