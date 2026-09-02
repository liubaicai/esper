package esper

import (
	"context"
	"testing"
)

// TestResultSetOutputLimitRowLimitLengthBatchParity locks the row-limit
// boundary for a batched wildcard stream. The limit is applied to each
// completed batch (rather than each input event), while old data reports the
// previously selected batch.
func TestResultSetOutputLimitRowLimitLengthBatchParity(t *testing.T) {
	env, engine := newRuntimeTest(t)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").
		Window(LengthBatch(3)).
		Query(StatementName("row-limit-length-batch"), WithOldStream(), Limit(1)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	send := func(symbol string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	resultSymbol := func(result Result) string {
		t.Helper()
		event, ok := result.Event()
		if !ok {
			t.Fatalf("result is not an event: %#v", result)
		}
		trade, ok := event.Underlying().(runtimeTestTrade)
		if !ok {
			t.Fatalf("result event has type %T", event.Underlying())
		}
		return trade.Symbol
	}
	assertSnapshot := func(want []string) {
		t.Helper()
		snapshot, err := statement.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(snapshot.Results()) != len(want) {
			t.Fatalf("iterator size = %d, want %d", len(snapshot.Results()), len(want))
		}
		for i, symbol := range want {
			if got := resultSymbol(snapshot.Results()[i]); got != symbol {
				t.Fatalf("iterator result %d = %q, want %q", i, got, symbol)
			}
		}
	}

	send("E1")
	send("E2")
	if len(batches) != 0 {
		t.Fatalf("partial batch produced %d deliveries", len(batches))
	}
	assertSnapshot([]string{"E1"})

	send("E3")
	if len(batches) != 1 || len(batches[0].New) != 1 || len(batches[0].Old) != 0 {
		t.Fatalf("first flush = %#v, want one new and no old", batches)
	}
	if got := resultSymbol(batches[0].New[0]); got != "E1" {
		t.Fatalf("first flush new = %q, want E1", got)
	}
	assertSnapshot(nil)

	send("E4")
	send("E5")
	if len(batches) != 1 {
		t.Fatalf("second partial batch produced %d deliveries", len(batches))
	}
	assertSnapshot([]string{"E4"})

	send("E6")
	if len(batches) != 2 || len(batches[1].New) != 1 || len(batches[1].Old) != 1 {
		t.Fatalf("second flush = %#v, want one new and one old", batches)
	}
	if got := resultSymbol(batches[1].New[0]); got != "E4" {
		t.Fatalf("second flush new = %q, want E4", got)
	}
	if got := resultSymbol(batches[1].Old[0]); got != "E1" {
		t.Fatalf("second flush old = %q, want E1", got)
	}
	assertSnapshot(nil)
}
