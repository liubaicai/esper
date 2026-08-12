package esper

import (
	"context"
	"testing"
)

type orderByMarketData struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
}

func registerOrderByTypes(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterStruct[orderByMarketData](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
}

func sendOrderByEvents(t *testing.T, engine *Engine) {
	t.Helper()
	ctx := context.Background()
	for _, e := range []orderByMarketData{
		{Symbol: "IBM", Price: 2},
		{Symbol: "KGB", Price: 1},
		{Symbol: "CMU", Price: 3},
		{Symbol: "IBM", Price: 6},
		{Symbol: "CAT", Price: 6},
		{Symbol: "CAT", Price: 5},
	} {
		if err := engine.SendEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
}

// TestResultSetOrderByAscendingParity mirrors the ascending price sort from
// ResultSetSimple/ResultSetDescending. Groups by symbol, sums price, orders by
// sum ascending: KGB(1), IBM(8), CMU(3), CAT(11).
func TestResultSetOrderByAscendingParity(t *testing.T) {
	env := NewEnvironment()
	registerOrderByTypes(t, env)
	symbol := Field[orderByMarketData, string]("symbol")
	price := Field[orderByMarketData, float64]("price")
	plan, err := env.Build(From[orderByMarketData](env, "SupportMarketDataBean").
		Window(LengthBatch(6)).GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("sum", Sum[float64](price)),
	).Query(OrderBy(Ascending(ResultField[float64]("sum"))), StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, r := range batch.New {
			row, _ := r.Row()
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendOrderByEvents(t, engine)
	wantSymbols := []string{"KGB", "CMU", "IBM", "CAT"}
	wantSums := []float64{1, 3, 8, 11}
	if len(rows) != len(wantSymbols) {
		t.Fatalf("ascending rows = %d, want %d", len(rows), len(wantSymbols))
	}
	for i, ws := range wantSymbols {
		if rows[i].Get("symbol").Any() != ws || rows[i].Get("sum").Any() != wantSums[i] {
			t.Fatalf("ascending row %d = {%v, %v}, want {%s, %v}",
				i, rows[i].Get("symbol").Any(), rows[i].Get("sum").Any(), ws, wantSums[i])
		}
	}
}

// TestResultSetOrderByDescendingParity mirrors the descending price sort from
// ResultSetDescending. Same groups, ordered by sum descending.
func TestResultSetOrderByDescendingParity(t *testing.T) {
	env := NewEnvironment()
	registerOrderByTypes(t, env)
	symbol := Field[orderByMarketData, string]("symbol")
	price := Field[orderByMarketData, float64]("price")
	plan, err := env.Build(From[orderByMarketData](env, "SupportMarketDataBean").
		Window(LengthBatch(6)).GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("sum", Sum[float64](price)),
	).Query(OrderBy(Descending(ResultField[float64]("sum"))), StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, r := range batch.New {
			row, _ := r.Row()
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendOrderByEvents(t, engine)
	wantSymbols := []string{"CAT", "IBM", "CMU", "KGB"}
	wantSums := []float64{11, 8, 3, 1}
	if len(rows) != len(wantSymbols) {
		t.Fatalf("descending rows = %d, want %d", len(rows), len(wantSymbols))
	}
	for i, ws := range wantSymbols {
		if rows[i].Get("symbol").Any() != ws || rows[i].Get("sum").Any() != wantSums[i] {
			t.Fatalf("descending row %d = {%v, %v}, want {%s, %v}",
				i, rows[i].Get("symbol").Any(), rows[i].Get("sum").Any(), ws, wantSums[i])
		}
	}
}

// TestResultSetOrderByMultipleKeysParity mirrors ResultSetDescending with
// "order by sum desc, symbol asc": within equal sum groups, sort by symbol asc.
// CAT and IBM both sum high but aren't equal here; this validates multi-key tiebreak.
func TestResultSetOrderByMultipleKeysParity(t *testing.T) {
	env := NewEnvironment()
	registerOrderByTypes(t, env)
	symbol := Field[orderByMarketData, string]("symbol")
	price := Field[orderByMarketData, float64]("price")
	plan, err := env.Build(From[orderByMarketData](env, "SupportMarketDataBean").
		Window(LengthBatch(6)).GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("sum", Sum[float64](price)),
	).Query(OrderBy(
		Descending(ResultField[float64]("sum")),
		Ascending(ResultField[string]("symbol")),
	), StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, r := range batch.New {
			row, _ := r.Row()
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Send events where two symbols have equal sum to test tie-break.
	ctx := context.Background()
	for _, e := range []orderByMarketData{
		{Symbol: "AAA", Price: 5},
		{Symbol: "BBB", Price: 5},
		{Symbol: "CCC", Price: 10},
		{Symbol: "DDD", Price: 10},
		{Symbol: "EEE", Price: 3},
		{Symbol: "FFF", Price: 3},
	} {
		if err := engine.SendEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	// Groups: AAA=5, BBB=5, CCC=10, DDD=10, EEE=3, FFF=3
	// Order by sum desc, symbol asc: CCC(10), DDD(10), AAA(5), BBB(5), EEE(3), FFF(3)
	want := []string{"CCC", "DDD", "AAA", "BBB", "EEE", "FFF"}
	if len(rows) != len(want) {
		t.Fatalf("multi-key rows = %d, want %d", len(rows), len(want))
	}
	for i, ws := range want {
		if rows[i].Get("symbol").Any() != ws {
			t.Fatalf("multi-key row %d = %v, want %s (full: %v)", i, rows[i].Get("symbol").Any(), ws, rows)
		}
	}
}

// TestResultSetOrderByExpressionParity mirrors ResultSetExpressions: order by a
// computed expression applied to the aggregate result. Sum * 2 sorts the same
// direction as sum, confirming expression keys work.
func TestResultSetOrderByExpressionParity(t *testing.T) {
	env := NewEnvironment()
	registerOrderByTypes(t, env)
	symbol := Field[orderByMarketData, string]("symbol")
	price := Field[orderByMarketData, float64]("price")
	plan, err := env.Build(From[orderByMarketData](env, "SupportMarketDataBean").
		Window(LengthBatch(6)).GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("sum", Sum[float64](price)),
	).Query(OrderBy(Ascending(Multiply[float64](ResultField[float64]("sum"), Literal(2.0)))), StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, r := range batch.New {
			row, _ := r.Row()
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendOrderByEvents(t, engine)
	// sum*2 ascending: KGB(2), CMU(6), IBM(16), CAT(22)
	want := []string{"KGB", "CMU", "IBM", "CAT"}
	if len(rows) != len(want) {
		t.Fatalf("expression rows = %d, want %d", len(rows), len(want))
	}
	for i, ws := range want {
		if rows[i].Get("symbol").Any() != ws {
			t.Fatalf("expression row %d = %v, want %s", i, rows[i].Get("symbol").Any(), ws)
		}
	}
}

// TestResultSetOrderByInvalidParity mirrors ResultSetInvalid: ordering by an
// unknown result field is rejected at build time.
func TestResultSetOrderByInvalidParity(t *testing.T) {
	env := NewEnvironment()
	registerOrderByTypes(t, env)
	symbol := Field[orderByMarketData, string]("symbol")
	price := Field[orderByMarketData, float64]("price")
	invalid := From[orderByMarketData](env, "SupportMarketDataBean").
		Window(LengthBatch(6)).GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("sum", Sum[float64](price)),
	).Query(OrderBy(Ascending(ResultField[float64]("missing"))))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("unknown order-by field should fail at build time")
	}
}
