package esper

import (
	"context"
	"testing"
	"time"
)

func TestPreviousExpressionsUseWindowHistory(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	stream := From[runtimeTestTrade](env, "Trade").Window(LengthWindow(3))
	projected := Select(stream,
		Alias("current", price),
		Alias("prev", Prev[float64](1, price)),
		Alias("prior", Prior[float64](0, price)),
	)
	plan, err := env.Build(projected.Query(StatementName("previous")))
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

	for _, priceValue := range []float64{10, 20, 30} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: priceValue}); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 3 {
		t.Fatalf("previous batches = %d, want 3", len(batches))
	}
	want := [][]Value{
		{Present(10.0), Null(), Null()},
		{Present(20.0), Present(10.0), Present(10.0)},
		{Present(30.0), Present(20.0), Present(20.0)},
	}
	for index, batch := range batches {
		if len(batch.New) != 1 {
			t.Fatalf("batch %d new = %#v", index, batch.New)
		}
		row, ok := batch.New[0].Row()
		if !ok {
			t.Fatalf("batch %d result is not a row", index)
		}
		values := row.Values()
		if len(values) != len(want[index]) {
			t.Fatalf("batch %d values = %#v", index, values)
		}
		for valueIndex, expected := range want[index] {
			if !values[valueIndex].Equal(expected) {
				t.Fatalf("batch %d value %d = %v, want %v", index, valueIndex, values[valueIndex], expected)
			}
		}
	}
}

func TestPreviousExpressionsReturnNullWhenHistoryIsUnavailable(t *testing.T) {
	schema, err := StructSchema[runtimeTestTrade]("Trade")
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, runtimeTestTrade{Price: 10}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	price := Field[runtimeTestTrade, float64]("price")
	if got := Prev[float64](-1, price).eval(EvalContext{Event: event}); !got.IsNull() {
		t.Fatalf("negative prev = %v", got)
	}
	if got := Prior[float64](0, price).eval(EvalContext{Event: event, History: []Event{event}}); !got.IsNull() {
		t.Fatalf("missing prior = %v", got)
	}
}

func TestLeavingExpressionMarksRemoveStream(t *testing.T) {
	env, engine := newRuntimeTest(t)
	projected := Select(
		From[runtimeTestTrade](env, "Trade").Window(LengthWindow(1)),
		Alias("leaving", Leaving()),
	)
	plan, err := env.Build(projected.Query(StatementName("leaving"), WithOldStream()))
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
	for _, symbol := range []string{"A", "B"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 2 || len(batches[1].New) != 1 || len(batches[1].Old) != 1 {
		t.Fatalf("leaving stream batches = %#v", batches)
	}
	newRow, _ := batches[1].New[0].Row()
	oldRow, _ := batches[1].Old[0].Row()
	if newRow.Get("leaving").Any() != false || oldRow.Get("leaving").Any() != true {
		t.Fatalf("leaving values = %#v / %#v", newRow.AsMap(), oldRow.AsMap())
	}
}
