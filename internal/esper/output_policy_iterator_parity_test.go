package esper

import (
	"context"
	"reflect"
	"testing"
)

type outputPolicyIteratorTrade struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
}

// TestOutputPolicyIteratorViewsMatchJava mirrors the shared
// output-policy-iterator parity scenario. The Java oracle (Esper 9.0.0 commit
// 9e1b9f1cc9117fea4bf33ab043762c045d73839c) exposes live aggregate state in
// current filtered-window order through statement.iterator() for output all,
// output first and output last every N events; listener batches keep their own
// delivery order. First-every-events is per-group: a group emits its first
// row, then re-emits its current row after every N events for that group.
func TestOutputPolicyIteratorViewsMatchJava(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[outputPolicyIteratorTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	defer func() { _ = engine.Close(context.Background()) }()
	symbol := Field[outputPolicyIteratorTrade, string]("symbol")
	price := Field[outputPolicyIteratorTrade, float64]("price")

	build := func(t *testing.T, policy OutputPolicy) *Statement {
		t.Helper()
		query := From[outputPolicyIteratorTrade](env, "Trade").
			Window(LengthWindow(2)).
			Filter(Greater[float64](price, Literal(10.0))).
			GroupBy(symbol).
			Select(
				Alias("symbol", symbol),
				Alias("total", Sum[float64](price)),
			)
		plan, err := env.Build(query.Query(StatementName("s0"), WithOutput(policy)))
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

	row := func(symbol string, total any) map[string]any {
		return map[string]any{"symbol": symbol, "total": total}
	}
	rows := func(t *testing.T, statement *Statement) []map[string]any {
		t.Helper()
		snapshot, err := statement.SnapshotWithSelector(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]map[string]any, 0, len(snapshot.Batch.New))
		for _, result := range snapshot.Batch.New {
			r, ok := result.Row()
			if !ok {
				t.Fatalf("snapshot result is not a row: %#v", result)
			}
			value := r.Get("total").Any()
			if r.Get("total").IsNull() {
				value = map[string]any{"state": "null"}
			}
			out = append(out, row(r.Get("symbol").Any().(string), value))
		}
		return out
	}
	assertRows := func(t *testing.T, got []map[string]any, want ...map[string]any) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("rows = %#v, want %#v", got, want)
		}
	}
	send := func(t *testing.T, symbol string, price float64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), outputPolicyIteratorTrade{Symbol: symbol, Price: price}); err != nil {
			t.Fatal(err)
		}
	}

	runCase := func(t *testing.T, policy OutputPolicy, wantBatches [][]map[string]any) {
		t.Helper()
		statement := build(t, policy)
		var batches [][]map[string]any
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			out := make([]map[string]any, 0, len(batch.New))
			for _, result := range batch.New {
				r, ok := result.Row()
				if !ok {
					t.Fatalf("listener result is not a row: %#v", result)
				}
				out = append(out, row(r.Get("symbol").Any().(string), r.Get("total").Any()))
			}
			batches = append(batches, out)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		events := []struct {
			symbol string
			price  float64
		}{
			{"A", 11}, {"B", 12}, {"A", 13}, {"B", 14}, {"A", 20}, {"B", 15},
		}
		wantSnapshots := [][]map[string]any{
			{row("A", float64(11))},
			{row("A", float64(11)), row("B", float64(12))},
			{row("B", float64(12)), row("A", float64(13))},
			{row("A", float64(13)), row("B", float64(14))},
			{row("B", float64(14)), row("A", float64(20))},
			{row("A", float64(20)), row("B", float64(15))},
		}
		for index, event := range events {
			send(t, event.symbol, event.price)
			assertRows(t, rows(t, statement), wantSnapshots[index]...)
		}
		if !reflect.DeepEqual(batches, wantBatches) {
			t.Fatalf("batches = %#v, want %#v", batches, wantBatches)
		}
	}

	runCase(t, OutputSnapshotEveryEvents(3), [][]map[string]any{
		{row("A", float64(13)), row("B", float64(12))},
		{row("A", float64(20)), row("B", float64(15))},
	})
	runCase(t, OutputFirstEveryEvents(3), [][]map[string]any{
		{row("A", float64(11))},
		{row("B", float64(12))},
	})
	runCase(t, OutputLastEveryEvents(3), [][]map[string]any{
		{row("A", float64(13)), row("B", float64(12))},
		{row("B", float64(15)), row("A", float64(20))},
	})
}
