package esper

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestDataflowBeaconOptionsIterationsAndFinalMarkerMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	definition, err := DefineDataflow(env, "beacon-options-flow").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{
			Iterations:   3,
			InitialDelay: time.Millisecond,
			Interval:     time.Millisecond,
		}, "A", "B").
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if instance.State() != DataflowComplete {
		t.Fatalf("beacon finite state = %v, want complete", instance.State())
	}
	if got, want := instance.Outputs(), []any{"A", "B", "A"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("beacon finite outputs = %#v, want %#v", got, want)
	}
	if stats := instance.Stats(); stats.Processed != 3 || stats.Emitted != 3 {
		t.Fatalf("beacon finite stats = %#v", stats)
	}
	if signals := instance.Signals(); len(signals) != 1 || !isDataflowFinalMarker(signals[0]) {
		t.Fatalf("beacon final marker signals = %#v", signals)
	}
}

func TestDataflowBeaconFactoryReceivesStableIterationContext(t *testing.T) {
	env := NewEnvironment()
	contexts := make([]DataflowBeaconContext, 0, 3)
	definition, err := DefineDataflow(env, "beacon-factory-flow").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{
			Iterations: 3,
			Factory: func(_ context.Context, beacon DataflowBeaconContext) (any, error) {
				contexts = append(contexts, beacon)
				return fmt.Sprintf("%s-%d", beacon.OperatorName, beacon.Iteration), nil
			},
		}).
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflowWithOptions(context.Background(), definition, DataflowOptions{InstanceID: "beacon-instance"})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(contexts) != 3 {
		t.Fatalf("beacon factory contexts = %#v, want three calls", contexts)
	}
	for index, beacon := range contexts {
		if beacon.DataflowName != "beacon-factory-flow" || beacon.InstanceID != "beacon-instance" || beacon.OperatorName != "source" || beacon.Iteration != index || beacon.Now.IsZero() {
			t.Fatalf("beacon factory context[%d] = %#v", index, beacon)
		}
	}
	if got, want := instance.Outputs(), []any{"source-0", "source-1", "source-2"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("beacon factory outputs = %#v, want %#v", got, want)
	}
}

func TestDataflowBeaconWithoutIterationLimitRunsUntilCanceled(t *testing.T) {
	env := NewEnvironment()
	var calls atomic.Int32
	ready := make(chan struct{})
	var readyOnce atomic.Bool
	definition, err := DefineDataflow(env, "beacon-unlimited-flow").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{
			Interval: time.Millisecond,
			Factory: func(_ context.Context, beacon DataflowBeaconContext) (any, error) {
				count := calls.Add(1)
				if count >= 3 && readyOnce.CompareAndSwap(false, true) {
					close(ready)
				}
				return beacon.Iteration, nil
			},
		}).
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("unlimited beacon did not emit three values")
	}
	if instance.State() != DataflowRunning {
		t.Fatalf("unlimited beacon state = %v, want running", instance.State())
	}
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := instance.Join(context.Background()); err == nil || !errors.Is(err, ErrorCanceled) {
		t.Fatalf("unlimited beacon join error = %v, want cancellation", err)
	}
	if instance.State() != DataflowCanceled {
		t.Fatalf("unlimited beacon canceled state = %v", instance.State())
	}
}

func TestDataflowBeaconOptionsRejectNegativeTiming(t *testing.T) {
	env := NewEnvironment()
	if _, err := DefineDataflow(env, "invalid-beacon-options").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{Iterations: -1}, "value").
		Build(); err == nil {
		t.Fatal("negative beacon iterations were accepted")
	}
	if _, err := DefineDataflow(env, "invalid-beacon-delay").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{InitialDelay: -time.Second}, "value").
		Build(); err == nil {
		t.Fatal("negative beacon initial delay was accepted")
	}
}
