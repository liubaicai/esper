package esper

import (
	"context"
	"testing"
	"time"
)

func TestClientRuntimeObtainEngineWideWriteLockParity(t *testing.T) {
	env, engine := newRuntimeTest(t)
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

	locked := make(chan struct{})
	release := make(chan struct{})
	lockDone := make(chan error, 1)
	go func() {
		lockDone <- engine.WithRuntimeWriteLock(context.Background(), func() error {
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked

	sendStarted := make(chan struct{})
	sendDone := make(chan error, 1)
	go func() {
		close(sendStarted)
		sendDone <- engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E1"})
	}()
	<-sendStarted
	select {
	case err := <-sendDone:
		t.Fatalf("send completed while runtime write lock was held: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-lockDone; err != nil {
		t.Fatal(err)
	}
	if err := <-sendDone; err != nil {
		t.Fatal(err)
	}

	snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	results := snapshot.Results()
	if len(results) != 1 || results[0].Get("c0").Any() != int64(1) {
		t.Fatalf("snapshot after lock release = %#v, want c0=1", results)
	}
	if err := engine.WithRuntimeWriteLock(context.Background(), nil); err == nil {
		t.Fatal("nil runtime write-lock action was accepted")
	}
}
