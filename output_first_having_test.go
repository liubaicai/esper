package esper

import (
	"context"
	"testing"
	"time"
)

func TestOutputFirstEveryEventsWithHavingMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(KeepAll()).Aggregate(
		Alias("value", price),
	).Having(Greater[float64](price, Literal(1.0))).Query(
		StatementName("output-first-every-events-having"),
		WithOutput(OutputFirstEveryEvents(2)),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var values []float64
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("first-every-events result is not a row: %#v", result)
			}
			values = append(values, row.Get("value").Any().(float64))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{1, 2, 9, 1, 1, 2, 1, 2} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: value}); err != nil {
			t.Fatal(err)
		}
	}
	if len(values) != 3 || values[0] != 2 || values[1] != 2 || values[2] != 2 {
		t.Fatalf("first-every-events values = %#v", values)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		WithOutput(OutputFirstEveryEvents(0)),
	)); err == nil {
		t.Fatal("zero first-every-events count was accepted")
	}
}

func TestOutputFirstEveryTimeWithHavingMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	price := Field[runtimeTestTrade, float64]("price")
	sum := Sum[float64](price)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(5)).Aggregate(
		Alias("sum", sum),
	).Having(Greater[float64](sum, Literal(100.0))).Query(
		StatementName("output-first-every-time-having"),
		WithOutput(OutputFirstEveryTime(2*time.Second)),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var values []float64
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("first-every-time result is not a row: %#v", result)
			}
			values = append(values, row.Get("sum").Any().(float64))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{10, 80} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: value}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 11}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 999000000).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(3, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{1, 100} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: value}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(4, 999000000).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 0}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(5, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 0}); err != nil {
		t.Fatal(err)
	}
	if len(values) != 3 || values[0] != 101 || values[1] != 114 || values[2] != 102 {
		t.Fatalf("first-every-time values = %#v", values)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		WithOutput(OutputFirstEveryTime(0)),
	)); err == nil {
		t.Fatal("zero first-every-time interval was accepted")
	}
}
