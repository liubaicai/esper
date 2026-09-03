package esper

import (
	"context"
	"errors"
	"testing"
	"time"
)

// ResultSetOutputLimitRowLimit in Esper applies the row limit after the
// output interval has been ordered. Keeping the candidate rows until the
// batch flush is important for offsets: an offset applied to each input event
// would discard every event before the interval can be sorted.
func TestOutputEveryAppliesOrderLimitAfterBatchMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(5)).Query(
		StatementName("output-every-row-limit"),
		WithOutput(OutputEvery(5)),
		OrderBy(Ascending(price)),
		Limit(2),
		Offset(2),
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
	for _, event := range []runtimeTestTrade{
		{Symbol: "E1", Price: 90},
		{Symbol: "E2", Price: 5},
		{Symbol: "E3", Price: 60},
		{Symbol: "E4", Price: 99},
		{Symbol: "E5", Price: 6},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 1 || len(batches[0].New) != 2 {
		t.Fatalf("output-every row limit batches = %#v", batches)
	}
	for index, want := range []struct {
		symbol string
		price  float64
	}{{"E3", 60}, {"E1", 90}} {
		event, ok := batches[0].New[index].Event()
		if !ok {
			t.Fatalf("output-every row %d is not an event: %#v", index, batches[0].New[index])
		}
		trade, ok := event.Underlying().(runtimeTestTrade)
		if !ok || trade.Symbol != want.symbol || trade.Price != want.price {
			t.Fatalf("output-every row %d = %#v, want %s/%v", index, event.Underlying(), want.symbol, want.price)
		}
	}
}

// The same rule applies to aggregate rows. The four aggregate updates are
// ordered as a single time interval, so the top two sums are the last two
// values rather than the first two candidates surviving a per-event limit.
func TestOutputEveryTimeAppliesOrderLimitToAggregateInterval(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(5)).Aggregate(
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
		Alias("sum", Sum[float64](price)),
	).Query(
		StatementName("output-every-time-aggregate-limit"),
		WithOutput(OutputEveryTime(10*time.Second)),
		OrderBy(Descending(ResultField[float64]("sum"))),
		Limit(2),
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
	for _, event := range []runtimeTestTrade{
		{Symbol: "E1", Price: 10},
		{Symbol: "E2", Price: 5},
		{Symbol: "E3", Price: 20},
		{Symbol: "E4", Price: 30},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}

	if err := engine.AdvanceTime(context.Background(), time.Unix(11, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 2 {
		t.Fatalf("output-every-time aggregate batches = %#v", batches)
	}
	want := []struct {
		symbol string
		sum    float64
	}{{"E4", 65}, {"E3", 35}}
	for index, expected := range want {
		row, ok := batches[0].New[index].Row()
		if !ok || row.Get("symbol").Any() != expected.symbol || row.Get("sum").Any() != expected.sum {
			t.Fatalf("output-every-time aggregate row %d = %#v, want %#v", index, row, expected)
		}
	}
}

func TestOutputSnapshotEveryAppliesGroupedOrderLimitMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(5)).GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("sum", Sum[float64](price)),
	).Query(
		StatementName("output-snapshot-grouped-limit"),
		WithOutput(OutputSnapshotEvery(10*time.Second)),
		OrderBy(Descending(ResultField[float64]("sum"))),
		Limit(2),
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
	for _, event := range []runtimeTestTrade{
		{Symbol: "E1", Price: 10},
		{Symbol: "E2", Price: 5},
		{Symbol: "E3", Price: 20},
		{Symbol: "E1", Price: 30},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(11, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 2 {
		t.Fatalf("output snapshot grouped batches = %#v", batches)
	}
	want := []struct {
		symbol string
		sum    float64
	}{{"E1", 40}, {"E3", 20}}
	for index, expected := range want {
		row, ok := batches[0].New[index].Row()
		if !ok || row.Get("symbol").Any() != expected.symbol || row.Get("sum").Any() != expected.sum {
			t.Fatalf("output snapshot grouped row %d = %#v, want %#v", index, row, expected)
		}
	}
}
func TestResultWindowModifiersRejectTypedNilExpressions(t *testing.T) {
	env, engine := newRuntimeTest(t)
	defer func() { _ = engine.Close(context.Background()) }()

	var typedNil *typedExpr[int]
	for _, option := range []QueryOption{
		LimitExpression(typedNil),
		OffsetExpression(typedNil),
	} {
		_, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(option))
		if err == nil || !errors.Is(err, ErrorInvalidRule) {
			t.Fatalf("typed-nil result-window modifier error = %v", err)
		}
	}
}
