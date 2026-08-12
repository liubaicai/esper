package esper

import (
	"context"
	"testing"
	"time"
)

type outputGroupEvent struct {
	Symbol string `esper:"symbol"`
	Level  int64  `esper:"level"`
	Value  int64  `esper:"value"`
}

func TestOutputFirstEveryTimeIsIndependentPerGroupMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[outputGroupEvent](env, "OutputGroupEvent"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	symbol := Field[outputGroupEvent, string]("symbol")
	value := Field[outputGroupEvent, int64]("value")
	plan, err := env.Build(From[outputGroupEvent](env, "OutputGroupEvent").GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("value", value),
	).Query(
		StatementName("output-first-every-time-grouped"),
		WithOutput(OutputFirstEveryTime(10*time.Second)),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("grouped first-every result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []outputGroupEvent{
		{Symbol: "E1", Value: 1},
		{Symbol: "E1", Value: 2},
		{Symbol: "E2", Value: 3},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 2 || rows[0].Get("symbol").Any() != "E1" || rows[0].Get("value").Any() != int64(1) || rows[1].Get("symbol").Any() != "E2" || rows[1].Get("value").Any() != int64(3) {
		t.Fatalf("grouped first-every initial rows = %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(10, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	for _, event := range []outputGroupEvent{
		{Symbol: "E1", Value: 4},
		{Symbol: "E2", Value: 5},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 4 || rows[2].Get("symbol").Any() != "E1" || rows[3].Get("symbol").Any() != "E2" {
		t.Fatalf("grouped first-every refreshed rows = %#v", rows)
	}
}

func TestOutputLastEveryEventsKeepsLatestRowPerGroupMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[outputGroupEvent](env, "OutputGroupEvent"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	symbol := Field[outputGroupEvent, string]("symbol")
	value := Field[outputGroupEvent, int64]("value")
	plan, err := env.Build(From[outputGroupEvent](env, "OutputGroupEvent").GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("value", value),
	).Query(
		StatementName("output-last-every-events-grouped"),
		WithOutput(OutputLastEveryEvents(3)),
		OrderBy(Ascending(ResultField[string]("symbol"))),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batch ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, received ResultBatch) error {
		batch = received
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []outputGroupEvent{
		{Symbol: "IBM", Value: 10},
		{Symbol: "ATT", Value: 11},
		{Symbol: "IBM", Value: 100},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(batch.New) != 2 {
		t.Fatalf("grouped last-every rows = %#v", batch)
	}
	want := []struct {
		symbol string
		value  int64
	}{{"ATT", 11}, {"IBM", 100}}
	for index, expected := range want {
		row, ok := batch.New[index].Row()
		if !ok || row.Get("symbol").Any() != expected.symbol || row.Get("value").Any() != expected.value {
			t.Fatalf("grouped last-every row %d = %#v, want %#v", index, row, expected)
		}
	}
}

func TestOutputLastEveryTimeKeepsLatestRowsPerGroupAndSupportsMultikey(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[outputGroupEvent](env, "OutputGroupEvent"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	symbol := Field[outputGroupEvent, string]("symbol")
	level := Field[outputGroupEvent, int64]("level")
	value := Field[outputGroupEvent, int64]("value")
	plan, err := env.Build(From[outputGroupEvent](env, "OutputGroupEvent").GroupBy(symbol, level).Select(
		Alias("symbol", symbol),
		Alias("level", level),
		Alias("value", value),
	).Query(
		StatementName("output-last-every-time-multikey"),
		WithOutput(OutputLastEveryTime(time.Second)),
		OrderBy(Ascending(ResultField[string]("symbol")), Ascending(ResultField[int64]("level"))),
	))
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
	for _, event := range []outputGroupEvent{
		{Symbol: "A", Level: 0, Value: 10},
		{Symbol: "B", Level: 1, Value: 11},
		{Symbol: "A", Level: 0, Value: 12},
		{Symbol: "A", Level: 1, Value: 13},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 3 {
		t.Fatalf("grouped last-every-time batches = %#v", batches)
	}
	want := [][3]any{{"A", int64(0), int64(12)}, {"A", int64(1), int64(13)}, {"B", int64(1), int64(11)}}
	for index, expected := range want {
		row, ok := batches[0].New[index].Row()
		if !ok || row.Get("symbol").Any() != expected[0] || row.Get("level").Any() != expected[1] || row.Get("value").Any() != expected[2] {
			t.Fatalf("grouped last-every-time row %d = %#v, want %#v", index, row, expected)
		}
	}
}
