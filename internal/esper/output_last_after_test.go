package esper

import (
	"context"
	"testing"
	"time"
)

func TestOutputLastEveryEventsAfterActivationMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	sum := Sum[float64](price)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(KeepAll()).Aggregate(
		Alias("sum", sum),
	).Query(
		StatementName("output-after-last-every-events"),
		WithOutput(OutputAfterEvents(4, OutputLastEveryEvents(2))),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var sums []float64
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("last-every-events result is not a row: %#v", result)
			}
			sums = append(sums, row.Get("sum").Any().(float64))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{10, 20, 30, 40, 50, 60, 70, 80} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: value}); err != nil {
			t.Fatal(err)
		}
	}
	if len(sums) != 2 || sums[0] != 210 || sums[1] != 360 {
		t.Fatalf("last-every-events after output = %#v", sums)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		WithOutput(OutputLastEveryEvents(0)),
	)); err == nil {
		t.Fatal("zero last-every-events count was accepted")
	}
}

func TestOutputLastEveryTimeFlushesLatestPendingRow(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	sum := Sum[float64](price)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(KeepAll()).Aggregate(
		Alias("sum", sum),
	).Query(
		StatementName("output-last-every-time"),
		WithOutput(OutputLastEveryTime(2*time.Second)),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var sums []float64
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("last-every-time result is not a row: %#v", result)
			}
			sums = append(sums, row.Get("sum").Any().(float64))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{10, 20} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: value}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 999000000).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(sums) != 0 {
		t.Fatalf("last-every-time flushed early = %#v", sums)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(sums) != 1 || sums[0] != 30 {
		t.Fatalf("last-every-time first flush = %#v", sums)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 40}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(4, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(sums) != 2 || sums[1] != 70 {
		t.Fatalf("last-every-time second flush = %#v", sums)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		WithOutput(OutputLastEveryTime(0)),
	)); err == nil {
		t.Fatal("zero last-every-time interval was accepted")
	}
}
