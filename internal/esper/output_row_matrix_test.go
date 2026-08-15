package esper

import (
	"context"
	"testing"
)

type outputRowJoinLeft struct {
	Key string `esper:"key"`
}

type outputRowJoinRight struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

// TestOutputRowPerEventJoinAllAndLastMatchesEsper covers the two join forms
// in ResultSetOutputLimitRowPerEvent. OutputEvery retains every aggregate
// update in the interval, while OutputLastEveryEvents keeps the final global
// aggregate row for the same interval.
func TestOutputRowPerEventJoinAllAndLastMatchesEsper(t *testing.T) {
	run := func(t *testing.T, policy OutputPolicy, wantValues []int64, wantSums []int64) {
		t.Helper()
		env := NewEnvironment()
		if _, err := RegisterStruct[outputRowJoinLeft](env, "OutputRowJoinLeft"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[outputRowJoinRight](env, "OutputRowJoinRight"); err != nil {
			t.Fatal(err)
		}
		value := JoinField[int64](1, "value")
		plan, err := env.Build(Join(
			From[outputRowJoinLeft](env, "OutputRowJoinLeft").Window(LengthWindow(3)),
			From[outputRowJoinRight](env, "OutputRowJoinRight").Window(LengthWindow(3)),
			OnEqual(
				Field[outputRowJoinLeft, string]("key"),
				Field[outputRowJoinRight, string]("key"),
			),
		).Aggregate(
			Alias("value", value),
			Alias("sum", Sum[int64](value)),
		).Query(
			StatementName("output-row-per-event"),
			WithOutput(policy),
		))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer deployment.Undeploy(context.Background())
		var rows []Row
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok {
					t.Fatalf("join output result is not a row: %#v", result)
				}
				rows = append(rows, row)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), outputRowJoinLeft{Key: "A"}); err != nil {
			t.Fatal(err)
		}
		for _, value := range []int64{1, 2} {
			if err := engine.SendEvent(context.Background(), outputRowJoinRight{Key: "A", Value: value}); err != nil {
				t.Fatal(err)
			}
		}
		if len(rows) != len(wantValues) {
			t.Fatalf("join output rows = %#v, want %d rows", rows, len(wantValues))
		}
		for index, want := range wantValues {
			if got := rows[index].Get("value").Any(); got != want {
				t.Fatalf("join output value[%d] = %#v, want %d", index, got, want)
			}
			if got := rows[index].Get("sum").Any(); got != wantSums[index] {
				t.Fatalf("join output sum[%d] = %#v, want %d", index, got, wantSums[index])
			}
		}
	}

	run(t, OutputEvery(2), []int64{1, 2}, []int64{1, 3})
	run(t, OutputLastEveryEvents(2), []int64{2}, []int64{3})
}

type outputRowGroupEvent struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
}

// TestOutputRowPerGroupAllAndLastMatchesEsper keeps the group key in the
// output identity. A single interval contains two updates for A and one for
// B; all emits all three updates, while last emits only A's final row and B's
// row.
func TestOutputRowPerGroupAllAndLastMatchesEsper(t *testing.T) {
	run := func(t *testing.T, policy OutputPolicy, want map[string]float64, wantRows int) {
		t.Helper()
		env := NewEnvironment()
		if _, err := RegisterStruct[outputRowGroupEvent](env, "OutputRowGroupEvent"); err != nil {
			t.Fatal(err)
		}
		symbol := Field[outputRowGroupEvent, string]("symbol")
		price := Field[outputRowGroupEvent, float64]("price")
		plan, err := env.Build(From[outputRowGroupEvent](env, "OutputRowGroupEvent").
			Window(KeepAll()).
			GroupBy(symbol).
			Select(
				Alias("symbol", symbol),
				Alias("sum", Sum[float64](price)),
			).
			Query(StatementName("output-row-per-group"), WithOutput(policy)))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer deployment.Undeploy(context.Background())
		rows := make([]Row, 0, wantRows)
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok {
					t.Fatalf("group output result is not a row: %#v", result)
				}
				rows = append(rows, row)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		for _, event := range []outputRowGroupEvent{
			{Symbol: "A", Price: 10},
			{Symbol: "B", Price: 11},
			{Symbol: "A", Price: 100},
		} {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		if len(rows) != wantRows {
			t.Fatalf("group output rows = %#v, want %d", rows, wantRows)
		}
		if wantRows == 3 {
			wantSequence := []struct {
				symbol string
				sum    float64
			}{{"A", 10}, {"B", 11}, {"A", 110}}
			for index, expected := range wantSequence {
				if gotSymbol := rows[index].Get("symbol").Any(); gotSymbol != expected.symbol || rows[index].Get("sum").Any() != expected.sum {
					t.Fatalf("group all row[%d] = %#v, want %#v", index, rows[index].AsMap(), expected)
				}
			}
			return
		}
		seen := make(map[string]float64, len(rows))
		for _, row := range rows {
			seen[row.Get("symbol").Any().(string)] = row.Get("sum").Any().(float64)
		}
		if len(seen) != len(want) {
			t.Fatalf("group output keys = %#v, want %#v", seen, want)
		}
		for symbol, expected := range want {
			if seen[symbol] != expected {
				t.Fatalf("group output %s = %#v, want %v", symbol, seen[symbol], expected)
			}
		}
	}

	run(t, OutputEvery(3), map[string]float64{"A": 10, "B": 11}, 3)
	run(t, OutputLastEveryEvents(3), map[string]float64{"A": 110, "B": 11}, 2)
}

// TestOutputRowForAllAggregateOldNewBatchMatchesEsper covers an ungrouped
// aggregate over a length window. Java's OutputConditionCount satisfies on
// input view events (new >= N or old >= N), so the third input closes the
// interval even though the length(2) window also emits its first removal;
// the buffered deltas deliver new [10,30,50] and old [null,10,30]. The
// fourth input starts a fresh count and does not flush.
func TestOutputRowForAllAggregateOldNewBatchMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(2)).Aggregate(
		Alias("sum", Sum[float64](price)),
	).Query(
		StatementName("output-row-for-all"),
		WithOldStream(),
		WithOutput(OutputEvery(3)),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, price := range []float64{10, 20, 30} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 1 || len(batches[0].New) != 3 || len(batches[0].Old) != 3 {
		t.Fatalf("row-for-all initial output = %#v", batches)
	}
	for index, want := range []float64{10, 30, 50} {
		row, ok := batches[0].New[index].Row()
		if !ok || row.Get("sum").Any() != want {
			t.Fatalf("row-for-all initial new row %d = %#v", index, batches[0].New[index])
		}
	}
	oldNull, oldNullOK := batches[0].Old[0].Row()
	if !oldNullOK || !oldNull.Get("sum").IsNull() {
		t.Fatalf("row-for-all initial null old row = %#v", batches[0].Old[0])
	}
	for index, want := range []float64{10, 30} {
		row, ok := batches[0].Old[index+1].Row()
		if !ok || row.Get("sum").Any() != want {
			t.Fatalf("row-for-all initial old row %d = %#v", index+1, batches[0].Old[index+1])
		}
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 40}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("row-for-all old/new output flushed too early = %#v", batches)
	}
}
