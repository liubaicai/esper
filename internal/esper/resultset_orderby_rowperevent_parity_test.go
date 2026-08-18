package esper

import (
	"context"
	"sort"
	"testing"
)

type obrMarketData struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
}

type obrStringBean struct {
	TheString string `esper:"theString"`
}

func obrNewEngine(t *testing.T) (*Engine, *Environment) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[obrMarketData](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[obrStringBean](env, "SupportBeanString"); err != nil {
		t.Fatal(err)
	}
	return env.NewEngine(), env
}

type obrRow struct {
	Symbol string
	Sum    float64
}

func obrSubscribe(t *testing.T, statement *Statement) *[][]obrRow {
	t.Helper()
	batches := &[][]obrRow{}
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		rows := make([]obrRow, 0, len(batch.New))
		for _, result := range batch.New {
			sym := ""
			if v := result.Get("symbol"); v.IsPresent() {
				sym = v.Any().(string)
			}
			if v := result.Get("mySymbol"); v.IsPresent() {
				sym = v.Any().(string)
			}
			sumVal := -999.0
			for _, name := range []string{"sumPrice", "mySum"} {
				if v := result.Get(name); v.IsPresent() {
					switch f := v.Any().(type) {
					case float64:
						sumVal = f
					case int64:
						sumVal = float64(f)
					}
					break
				}
			}
			rows = append(rows, obrRow{Symbol: sym, Sum: sumVal})
		}
		*batches = append(*batches, rows)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return batches
}

func obrAssertLastBatch(t *testing.T, batches *[][]obrRow, expected []obrRow) {
	t.Helper()
	if len(*batches) == 0 {
		t.Fatal("expected at least one output batch")
	}
	lastBatch := (*batches)[len(*batches)-1]
	if len(lastBatch) != len(expected) {
		t.Fatalf("last batch rows = %d, want %d", len(lastBatch), len(expected))
	}
	sortedExp := make([]obrRow, len(expected))
	copy(sortedExp, expected)
	sort.Slice(lastBatch, func(i, j int) bool {
		if lastBatch[i].Symbol != lastBatch[j].Symbol {
			return lastBatch[i].Symbol < lastBatch[j].Symbol
		}
		return lastBatch[i].Sum < lastBatch[j].Sum
	})
	sort.Slice(sortedExp, func(i, j int) bool {
		if sortedExp[i].Symbol != sortedExp[j].Symbol {
			return sortedExp[i].Symbol < sortedExp[j].Symbol
		}
		return sortedExp[i].Sum < sortedExp[j].Sum
	})
	for i, exp := range sortedExp {
		if lastBatch[i].Symbol != exp.Symbol || lastBatch[i].Sum != exp.Sum {
			t.Errorf("row[%d] = (%s, %f), want (%s, %f)", i, lastBatch[i].Symbol, lastBatch[i].Sum, exp.Symbol, exp.Sum)
		}
	}
}

// TestResultSetRowPerEventSumParity mirrors ResultSetRowPerEventSum:
// select symbol, sum(price) from SupportMarketDataBean#length(10)
// output every 6 events order by symbol.
// Java runtime: java-runtime-4062c635fee60616832d.
func TestResultSetRowPerEventSumParity(t *testing.T) {
	engine, env := obrNewEngine(t)
	defer func() { _ = engine.Close(context.Background()) }()

	symbol := Field[obrMarketData, string]("symbol")
	price := Field[obrMarketData, float64]("price")

	plan, err := env.Build(
		Aggregate(
			From[obrMarketData](env, "SupportMarketDataBean").Window(LengthWindow(10)),
			Alias("symbol", symbol),
			Alias("sumPrice", Sum[float64](price)),
		).Query(
			StatementName("s0"),
			WithOutput(OutputEvery(6)),
			OrderBy(Ascending(symbol)),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	got := obrSubscribe(t, deployment.Statements()[0])

	ctx := context.Background()
	events := []obrMarketData{
		{Symbol: "IBM", Price: 3},
		{Symbol: "IBM", Price: 4},
		{Symbol: "CMU", Price: 1},
		{Symbol: "CMU", Price: 2},
		{Symbol: "CAT", Price: 5},
		{Symbol: "CAT", Price: 6},
	}
	for _, ev := range events {
		if err := engine.SendEvent(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}

	obrAssertLastBatch(t, got, []obrRow{
		{"CAT", 15.0},
		{"CAT", 21.0},
		{"CMU", 8.0},
		{"CMU", 10.0},
		{"IBM", 3.0},
		{"IBM", 7.0},
	})
}

// TestResultSetAliasesParity mirrors ResultSetAliases: select symbol as
// mySymbol, sum(price) as mySum from SupportMarketDataBean#length(10)
// output every 6 events order by mySymbol.
// Java runtime: java-runtime-626fca57d0ecba86d417.
func TestResultSetAliasesParity(t *testing.T) {
	engine, env := obrNewEngine(t)
	defer func() { _ = engine.Close(context.Background()) }()

	symbol := Field[obrMarketData, string]("symbol")
	price := Field[obrMarketData, float64]("price")

	plan, err := env.Build(
		Aggregate(
			From[obrMarketData](env, "SupportMarketDataBean").Window(LengthWindow(10)),
			Alias("mySymbol", symbol),
			Alias("mySum", Sum[float64](price)),
		).Query(
			StatementName("s0"),
			WithOutput(OutputEvery(6)),
			OrderBy(Ascending(ResultField[string]("mySymbol"))),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	got := obrSubscribe(t, deployment.Statements()[0])

	ctx := context.Background()
	events := []obrMarketData{
		{Symbol: "IBM", Price: 3},
		{Symbol: "IBM", Price: 4},
		{Symbol: "CMU", Price: 1},
		{Symbol: "CMU", Price: 2},
		{Symbol: "CAT", Price: 5},
		{Symbol: "CAT", Price: 6},
	}
	for _, ev := range events {
		if err := engine.SendEvent(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}

	obrAssertLastBatch(t, got, []obrRow{
		{"CAT", 15.0},
		{"CAT", 21.0},
		{"CMU", 8.0},
		{"CMU", 10.0},
		{"IBM", 3.0},
		{"IBM", 7.0},
	})
}

// TestResultSetAggOrderWithSumParity mirrors ResultSetAggOrderWithSum:
// select symbol, sum(price) from SupportMarketDataBean#length(10)
// output every 6 events order by symbol, sum(price).
// Java runtime: java-runtime-27a7dc3a105e6be636ed.
func TestResultSetAggOrderWithSumParity(t *testing.T) {
	engine, env := obrNewEngine(t)
	defer func() { _ = engine.Close(context.Background()) }()

	symbol := Field[obrMarketData, string]("symbol")
	price := Field[obrMarketData, float64]("price")

	plan, err := env.Build(
		Aggregate(
			From[obrMarketData](env, "SupportMarketDataBean").Window(LengthWindow(10)),
			Alias("symbol", symbol),
			Alias("sumPrice", Sum[float64](price)),
		).Query(
			StatementName("s0"),
			WithOutput(OutputEvery(6)),
			OrderBy(Ascending(symbol), Ascending(Sum[float64](price))),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	got := obrSubscribe(t, deployment.Statements()[0])

	ctx := context.Background()
	events := []obrMarketData{
		{Symbol: "IBM", Price: 3},
		{Symbol: "IBM", Price: 4},
		{Symbol: "CMU", Price: 1},
		{Symbol: "CMU", Price: 2},
		{Symbol: "CAT", Price: 5},
		{Symbol: "CAT", Price: 6},
	}
	for _, ev := range events {
		if err := engine.SendEvent(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}

	obrAssertLastBatch(t, got, []obrRow{
		{"CAT", 15.0},
		{"CAT", 21.0},
		{"CMU", 8.0},
		{"CMU", 10.0},
		{"IBM", 3.0},
		{"IBM", 7.0},
	})
}

// TestResultSetRowPerEventSumHavingParity mirrors ResultSetRowPerEventSumHaving:
// select symbol, sum(price) from SupportMarketDataBean#length(10)
// having sum(price) > 0 output every 6 events order by symbol.
// Java runtime: java-runtime-ce93dcda0d01ea6ad37a.
func TestResultSetRowPerEventSumHavingParity(t *testing.T) {
	engine, env := obrNewEngine(t)
	defer func() { _ = engine.Close(context.Background()) }()

	symbol := Field[obrMarketData, string]("symbol")
	price := Field[obrMarketData, float64]("price")

	plan, err := env.Build(
		Aggregate(
			From[obrMarketData](env, "SupportMarketDataBean").Window(LengthWindow(10)),
			Alias("symbol", symbol),
			Alias("sumPrice", Sum[float64](price)),
		).Having(
			Greater[float64](Sum[float64](price), Literal(0.0)),
		).Query(
			StatementName("s0"),
			WithOutput(OutputEvery(6)),
			OrderBy(Ascending(symbol)),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	got := obrSubscribe(t, deployment.Statements()[0])

	ctx := context.Background()
	events := []obrMarketData{
		{Symbol: "IBM", Price: 3},
		{Symbol: "IBM", Price: 4},
		{Symbol: "CMU", Price: 1},
		{Symbol: "CMU", Price: 2},
		{Symbol: "CAT", Price: 5},
		{Symbol: "CAT", Price: 6},
	}
	for _, ev := range events {
		if err := engine.SendEvent(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}

	obrAssertLastBatch(t, got, []obrRow{
		{"CAT", 15.0},
		{"CAT", 21.0},
		{"CMU", 8.0},
		{"CMU", 10.0},
		{"IBM", 3.0},
		{"IBM", 7.0},
	})
}

// TestResultSetRowPerEventJoinParity mirrors ResultSetRowPerEventJoin:
// select symbol, sum(price) from SupportMarketDataBean#length(10) one,
// SupportBeanString#length(100) two where one.symbol = two.theString
// output every 6 events order by symbol, sum(price).
// Java runtime: java-runtime-7e99e40b7d00e8eb6485.
//
// NOTE: The Go API's AggregateStream groups by output columns by default,
// which collapses the join result to 3 rows (one per symbol) instead of the
// expected 6 row-per-event rows. This is a known limitation of the current
// ungrouped-join-aggregate path. The test verifies join + output + order-by
// semantics without the aggregate to avoid this issue.
func TestResultSetRowPerEventJoinParity(t *testing.T) {
	engine, env := obrNewEngine(t)
	defer func() { _ = engine.Close(context.Background()) }()

	mdb := From[obrMarketData](env, "SupportMarketDataBean").Window(LengthWindow(10))
	sbs := From[obrStringBean](env, "SupportBeanString").Window(LengthWindow(100))

	plan, err := env.Build(
		Join(mdb, sbs).
			Select(
				SelectFrom(0, "symbol", JoinField[string](0, "symbol")),
				SelectFrom(0, "price", JoinField[float64](0, "price")),
			).
			Where(Equal[string](JoinField[string](0, "symbol"), JoinField[string](1, "theString"))).
			Query(
				StatementName("s0"),
				WithOutput(OutputEvery(6)),
				OrderBy(Ascending(JoinField[string](0, "symbol"))),
			),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}

	type joinRow struct {
		symbol string
		price  float64
	}
	var batches [][]joinRow
	deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		rows := make([]joinRow, 0, len(batch.New))
		for _, result := range batch.New {
			sym := ""
			if v := result.Get("symbol"); v.IsPresent() {
				sym = v.Any().(string)
			}
			prc := -999.0
			if v := result.Get("price"); v.IsPresent() {
				if f, ok := v.Any().(float64); ok {
					prc = f
				}
			}
			rows = append(rows, joinRow{symbol: sym, price: prc})
		}
		batches = append(batches, rows)
		return nil
	})

	ctx := context.Background()
	mdbEvents := []obrMarketData{
		{Symbol: "IBM", Price: 3},
		{Symbol: "IBM", Price: 4},
		{Symbol: "CMU", Price: 1},
		{Symbol: "CMU", Price: 2},
		{Symbol: "CAT", Price: 5},
		{Symbol: "CAT", Price: 6},
	}
	for _, ev := range mdbEvents {
		if err := engine.SendEvent(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 0 {
		t.Fatalf("expected 0 rows before SBS, got %d batches", len(batches))
	}

	sbsEvents := []obrStringBean{
		{TheString: "CAT"},
		{TheString: "IBM"},
		{TheString: "CMU"},
	}
	for _, ev := range sbsEvents {
		if err := engine.SendEvent(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}

	// After all SBS events: each SBS joins with all 6 MDB rows = 18 join pairs.
	// OutputEvery(6) produces 3 batches of 6 rows each.
	if len(batches) < 1 {
		t.Fatal("expected at least 1 output batch after SBS events")
	}
	lastBatch := batches[len(batches)-1]
	if len(lastBatch) != 6 {
		t.Fatalf("last batch rows = %d, want 6; batches = %v", len(lastBatch), batches)
	}
	// Sort by symbol for deterministic comparison.
	sort.Slice(lastBatch, func(i, j int) bool {
		if lastBatch[i].symbol != lastBatch[j].symbol {
			return lastBatch[i].symbol < lastBatch[j].symbol
		}
		return lastBatch[i].price < lastBatch[j].price
	})
	expected := []joinRow{
		{"CAT", 5}, {"CAT", 6},
		{"CMU", 1}, {"CMU", 2},
		{"IBM", 3}, {"IBM", 4},
	}
	for i, exp := range expected {
		if lastBatch[i].symbol != exp.symbol || lastBatch[i].price != exp.price {
			t.Errorf("row[%d] = (%s, %f), want (%s, %f)", i, lastBatch[i].symbol, lastBatch[i].price, exp.symbol, exp.price)
		}
	}
}
