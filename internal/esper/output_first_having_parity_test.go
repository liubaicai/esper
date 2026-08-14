package esper

import (
	"context"
	"testing"
	"time"
)

type outputFirstHavingParityBean struct {
	DoublePrimitive float64 `esper:"doublePrimitive"`
}

// TestOutputFirstHavingEventsParity mirrors ResultSetHavingNoAvgOutputFirstEvents:
// HAVING filters ungrouped rows and output first every 2 events emits only on
// every second passing event.
func TestOutputFirstHavingEventsParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[outputFirstHavingParityBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	value := Field[outputFirstHavingParityBean, float64]("doublePrimitive")
	plan, err := env.Build(From[outputFirstHavingParityBean](env, "SupportBean").Aggregate(
		Alias("doublePrimitive", value),
	).Having(Greater[float64](value, Literal(1.0))).Query(
		StatementName("s0"),
		WithOutput(OutputFirstEveryEvents(2)),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var values []float64
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("output-first-having result is not a row: %#v", result)
			}
			values = append(values, row.Get("doublePrimitive").Any().(float64))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{1, 2, 9, 1, 1, 2, 1, 2, 2, 2} {
		if err := engine.SendEvent(context.Background(), outputFirstHavingParityBean{DoublePrimitive: value}); err != nil {
			t.Fatal(err)
		}
	}
	if len(values) != 4 {
		t.Fatalf("output-first-having values = %#v", values)
	}
	for _, value := range values {
		if value != 2 {
			t.Fatalf("output-first-having values = %#v", values)
		}
	}
}

// TestOutputFirstHavingTimeParity mirrors ResultSetHavingNoAvgOutputFirstMinutes:
// length(5) sum + HAVING + output first every 2 seconds, including virtual-clock
// boundaries where no output occurs.
func TestOutputFirstHavingTimeParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[outputFirstHavingParityBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	value := Field[outputFirstHavingParityBean, float64]("doublePrimitive")
	sum := Sum[float64](value)
	plan, err := env.Build(From[outputFirstHavingParityBean](env, "SupportBean").Window(LengthWindow(5)).Aggregate(
		Alias("val0", sum),
	).Having(Greater[float64](sum, Literal(100.0))).Query(
		StatementName("s0"),
		WithOutput(OutputFirstEveryTime(2*time.Second)),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var values []float64
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("output-first-having-time result is not a row: %#v", result)
			}
			values = append(values, row.Get("val0").Any().(float64))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(value float64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), outputFirstHavingParityBean{DoublePrimitive: value}); err != nil {
			t.Fatal(err)
		}
	}
	advance := func(unix int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.Unix(unix, 0).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	send(10)
	send(80)
	if len(values) != 0 {
		t.Fatalf("pre-threshold output = %#v", values)
	}
	advance(1)
	send(11)
	if len(values) != 1 || values[0] != 101 {
		t.Fatalf("first time output = %#v", values)
	}
	send(1)
	advance(2)
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 999000000).UTC()); err != nil {
		t.Fatal(err)
	}
	send(1)
	if len(values) != 1 {
		t.Fatalf("pre-boundary output = %#v", values)
	}
	advance(3)
	send(1)
	send(100)
	if len(values) != 2 || values[1] != 114 {
		t.Fatalf("second time output = %#v", values)
	}
	advance(4)
	if err := engine.AdvanceTime(context.Background(), time.Unix(4, 999000000).UTC()); err != nil {
		t.Fatal(err)
	}
	send(0)
	if len(values) != 2 {
		t.Fatalf("second pre-boundary output = %#v", values)
	}
	advance(5)
	send(0)
	if len(values) != 3 || values[2] != 102 {
		t.Fatalf("third time output = %#v", values)
	}
}
