package esper

import (
	"context"
	"testing"
	"time"
)

// TestDataflowSelectSnapshotEveryMatchesEsper mirrors
// EPLDataflowOpSelect.EPLDataflowOutputRateLimit: input events accumulate in
// a Select-owned aggregate and only virtual-clock snapshot ticks reach the
// terminal Emitter.
func TestDataflowSelectSnapshotEveryMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	definition, err := DefineDataflow(env, "dataflow-select-snapshot").
		EventBusSource("source", "Trade").
		SelectSnapshotEvery(
			"aggregate",
			time.Minute,
			Alias("sum", Sum[float64](Field[runtimeTestTrade, float64]("price"))),
		).
		Emitter("sink").
		Connect("source", "aggregate").
		Connect("aggregate", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer instance.Cancel(context.Background())

	if err := engine.AdvanceTime(context.Background(), time.Unix(5, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	for _, price := range []float64{5, 3, 6} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	if got := instance.Outputs(); len(got) != 0 {
		t.Fatalf("snapshot output before interval = %#v", got)
	}

	if err := engine.AdvanceTime(context.Background(), time.Unix(65, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	assertDataflowSelectSum(t, instance.Outputs(), 0, 14)

	for _, price := range []float64{3, 6} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(125, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 2 {
		t.Fatalf("snapshot outputs = %#v", outputs)
	}
	assertDataflowSelectSum(t, outputs, 1, 23)
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 100}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(245, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if got := instance.Outputs(); len(got) != 2 {
		t.Fatalf("canceled snapshot dataflow emitted output = %#v", got)
	}
}

// TestDataflowSelectTimeWindowExpiresAtBoundaryMatchesEsper mirrors
// EPLDataflowOpSelect.EPLDataflowTimeWindowTriggered and fixes the exact
// expiry ordering: each removed event recalculates the current aggregate.
func TestDataflowSelectTimeWindowExpiresAtBoundaryMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	definition, err := DefineDataflow(env, "dataflow-select-time-window").
		EventBusSource("source", "Trade").
		SelectTimeWindow(
			"aggregate",
			time.Minute,
			Alias("sum", Sum[float64](Field[runtimeTestTrade, float64]("price"))),
		).
		Emitter("sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer instance.Cancel(context.Background())

	if err := engine.AdvanceTime(context.Background(), time.Unix(5, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(15, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 5}); err != nil {
		t.Fatal(err)
	}
	assertDataflowSelectSum(t, instance.Outputs(), 0, 2)
	assertDataflowSelectSum(t, instance.Outputs(), 1, 7)

	if err := engine.AdvanceTime(context.Background(), time.Unix(65, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	assertDataflowSelectSum(t, instance.Outputs(), 2, 5)
	if err := engine.AdvanceTime(context.Background(), time.Unix(75, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 4 {
		t.Fatalf("time-window outputs = %#v", outputs)
	}
	row, ok := outputs[3].(Row)
	if !ok || !row.Get("sum").IsNull() {
		t.Fatalf("time-window empty aggregate = %#v", outputs[3])
	}
}

func assertDataflowSelectSum(t *testing.T, outputs []any, index int, want float64) {
	t.Helper()
	if index < 0 || index >= len(outputs) {
		t.Fatalf("dataflow select output index %d missing in %#v", index, outputs)
	}
	row, ok := outputs[index].(Row)
	if !ok {
		t.Fatalf("dataflow select output %d = %#v, want Row", index, outputs[index])
	}
	value, ok := row.Get("sum").Any().(float64)
	if !ok || value != want {
		t.Fatalf("dataflow select sum[%d] = %#v, want %v", index, row.Get("sum").Any(), want)
	}
}
