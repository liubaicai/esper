package esper

import (
	"context"
	"reflect"
	"testing"
)

type filterWindowAggregateOutputTrade struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
}

// TestFilterWindowAggregateOutputListenerOrderMatchesJava mirrors the shared
// filter-window-aggregate-output parity scenario. The Java oracle (Esper
// 9.0.0 commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c) applies the where
// filter after the length window, so a filtered event still evicts an old
// event and produces the removal row. For istream grouped aggregation the
// incoming event's group is listed before the eviction-affected group.
func TestFilterWindowAggregateOutputListenerOrderMatchesJava(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[filterWindowAggregateOutputTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	defer func() { _ = engine.Close(context.Background()) }()
	symbol := Field[filterWindowAggregateOutputTrade, string]("symbol")
	price := Field[filterWindowAggregateOutputTrade, float64]("price")

	build := func(t *testing.T, withOutput bool) *Statement {
		t.Helper()
		query := From[filterWindowAggregateOutputTrade](env, "Trade").
			Window(LengthWindow(2)).
			Filter(Greater[float64](price, Literal(10.0))).
			GroupBy(symbol).
			Select(
				Alias("symbol", symbol),
				Alias("total", Sum[float64](price)),
			)
		options := []QueryOption{StatementName("s0")}
		if withOutput {
			options = append(options, WithOutput(OutputEvery(3)))
		}
		plan, err := env.Build(query.Query(options...))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })
		return deployment.Statements()[0]
	}

	collect := func(t *testing.T, statement *Statement) *[][]map[string]any {
		t.Helper()
		batches := &[][]map[string]any{}
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			rows := make([]map[string]any, 0, len(batch.New))
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok {
					t.Fatalf("aggregate output is not a row: %#v", result)
				}
				fields := map[string]any{"symbol": row.Get("symbol").Any()}
				if value := row.Get("total"); value.IsNull() {
					fields["total"] = map[string]any{"state": "null"}
				} else {
					fields["total"] = value.Any()
				}
				rows = append(rows, fields)
			}
			*batches = append(*batches, rows)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return batches
	}

	plain := build(t, false)
	plainBatches := collect(t, plain)
	for _, event := range []filterWindowAggregateOutputTrade{
		{Symbol: "A", Price: 11},
		{Symbol: "A", Price: 12},
		{Symbol: "B", Price: 13},
		{Symbol: "A", Price: 20},
		{Symbol: "B", Price: 5},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*plainBatches) != 5 {
		t.Fatalf("plain batches = %#v", *plainBatches)
	}
	assertBatch := func(t *testing.T, got []map[string]any, want ...map[string]any) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("batch = %#v, want %#v", got, want)
		}
		for index := range want {
			for key, expected := range want[index] {
				actual, exists := got[index][key]
				if !exists || !reflect.DeepEqual(actual, expected) {
					t.Fatalf("batch[%d][%s] = %#v, want %#v (batch %#v)", index, key, actual, expected, got)
				}
			}
		}
	}
	assertBatch(t, (*plainBatches)[0], map[string]any{"symbol": "A", "total": float64(11)})
	assertBatch(t, (*plainBatches)[1], map[string]any{"symbol": "A", "total": float64(23)})
	assertBatch(t, (*plainBatches)[2],
		map[string]any{"symbol": "B", "total": float64(13)},
		map[string]any{"symbol": "A", "total": float64(12)})
	assertBatch(t, (*plainBatches)[3], map[string]any{"symbol": "A", "total": float64(20)})
	assertBatch(t, (*plainBatches)[4], map[string]any{"symbol": "B", "total": map[string]any{"state": "null"}})

	snapshot, err := plain.SnapshotWithSelector(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Batch.New) != 1 {
		t.Fatalf("plain snapshot = %#v", snapshot.Batch)
	}
	row, ok := snapshot.Batch.New[0].Row()
	if !ok || row.Get("symbol").Any() != "A" || row.Get("total").Any() != float64(20) {
		t.Fatalf("plain snapshot row = %#v", snapshot.Batch.New[0])
	}

	every3 := build(t, true)
	every3Batches := collect(t, every3)
	snapshotRows := func(t *testing.T) []map[string]any {
		t.Helper()
		snapshot, err := every3.SnapshotWithSelector(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		rows := make([]map[string]any, 0, len(snapshot.Batch.New))
		for _, result := range snapshot.Batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("every-3 snapshot is not a row: %#v", result)
			}
			fields := map[string]any{"symbol": row.Get("symbol").Any()}
			if value := row.Get("total"); value.IsNull() {
				fields["total"] = map[string]any{"state": "null"}
			} else {
				fields["total"] = value.Any()
			}
			rows = append(rows, fields)
		}
		return rows
	}
	every3Events := []filterWindowAggregateOutputTrade{
		{Symbol: "A", Price: 11},
		{Symbol: "B", Price: 12},
		{Symbol: "A", Price: 13},
		{Symbol: "B", Price: 14},
	}
	// Esper's iterator for `output every N events` grouped aggregates reads
	// the last emitted per-group rows and walks the current filtered window
	// for presence and order: before the first output every group projects
	// null aggregates, after an output the values freeze at that output until
	// the next one, and pending deltas stay invisible.
	every3Snapshots := [][]map[string]any{
		{{"symbol": "A", "total": map[string]any{"state": "null"}}},
		{
			{"symbol": "A", "total": map[string]any{"state": "null"}},
			{"symbol": "B", "total": map[string]any{"state": "null"}},
		},
		{{"symbol": "B", "total": float64(12)}, {"symbol": "A", "total": float64(13)}},
		{{"symbol": "A", "total": float64(13)}, {"symbol": "B", "total": float64(12)}},
	}
	for index, event := range every3Events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		assertBatch(t, snapshotRows(t), every3Snapshots[index]...)
	}
	if len(*every3Batches) != 1 {
		t.Fatalf("every-3 batches = %#v", *every3Batches)
	}
	assertBatch(t, (*every3Batches)[0],
		map[string]any{"symbol": "A", "total": float64(11)},
		map[string]any{"symbol": "B", "total": float64(12)},
		map[string]any{"symbol": "A", "total": float64(13)})
}
