package esper

import (
	"context"
	"reflect"
	"testing"
)

type iteratorMarket struct {
	Symbol string  `esper:"symbol"`
	Volume int64   `esper:"volume"`
	Price  float64 `esper:"price"`
}

type iteratorBean struct {
	TheString string `esper:"theString"`
	IntBoxed  int    `esper:"intBoxed"`
}

func newIteratorMarketEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[iteratorMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	return env, engine
}

func sendIteratorMarket(t *testing.T, engine *Engine, symbol string, volume int64) {
	t.Helper()
	if err := engine.Send(context.Background(), "SupportMarketDataBean", iteratorMarket{Symbol: symbol, Volume: volume}); err != nil {
		t.Fatal(err)
	}
}

func sendIteratorMarketFull(t *testing.T, engine *Engine, symbol string, price float64, volume int64) {
	t.Helper()
	if err := engine.Send(context.Background(), "SupportMarketDataBean", iteratorMarket{Symbol: symbol, Price: price, Volume: volume}); err != nil {
		t.Fatal(err)
	}
}

func iteratorRows(t *testing.T, statement *Statement, fields ...string) [][]any {
	t.Helper()
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	rows := make([][]any, 0, len(snapshot.Results()))
	for _, result := range snapshot.Results() {
		row := make([]any, 0, len(fields))
		for _, field := range fields {
			row = append(row, result.Get(field).Any())
		}
		rows = append(rows, row)
	}
	return rows
}

func assertIteratorRows(t *testing.T, statement *Statement, want [][]any, fields ...string) {
	t.Helper()
	got := iteratorRows(t, statement, fields...)
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("iterator rows = %v, want %v", got, want)
	}
}

// TestResultSetQueryTypeOrderByWildcardParity covers
// ResultSetQueryTypeOrderByWildcard.
func TestResultSetQueryTypeOrderByWildcardParity(t *testing.T) {
	env, engine := newIteratorMarketEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	symbol := Field[iteratorMarket, string]("symbol")
	volume := Field[iteratorMarket, int64]("volume")
	plan, err := env.Build(From[iteratorMarket](env, "SupportMarketDataBean").Window(LengthWindow(5)).
		Query(StatementName("s0"), OrderBy(Ascending(symbol), Ascending(volume))))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	assertIteratorRows(t, statement, nil, "symbol", "volume")
	sendIteratorMarket(t, engine, "SYM", 1)
	assertIteratorRows(t, statement, [][]any{{"SYM", int64(1)}}, "symbol", "volume")
	sendIteratorMarket(t, engine, "OCC", 2)
	assertIteratorRows(t, statement, [][]any{{"OCC", int64(2)}, {"SYM", int64(1)}}, "symbol", "volume")
	sendIteratorMarket(t, engine, "TOC", 3)
	assertIteratorRows(t, statement, [][]any{{"OCC", int64(2)}, {"SYM", int64(1)}, {"TOC", int64(3)}}, "symbol", "volume")
	sendIteratorMarket(t, engine, "SYM", 0)
	assertIteratorRows(t, statement, [][]any{{"OCC", int64(2)}, {"SYM", int64(0)}, {"SYM", int64(1)}, {"TOC", int64(3)}}, "symbol", "volume")
	sendIteratorMarket(t, engine, "SYM", 10)
	assertIteratorRows(t, statement, [][]any{{"OCC", int64(2)}, {"SYM", int64(0)}, {"SYM", int64(1)}, {"SYM", int64(10)}, {"TOC", int64(3)}}, "symbol", "volume")
	sendIteratorMarket(t, engine, "SYM", 4)
	assertIteratorRows(t, statement, [][]any{{"OCC", int64(2)}, {"SYM", int64(0)}, {"SYM", int64(4)}, {"SYM", int64(10)}, {"TOC", int64(3)}}, "symbol", "volume")
}

// TestResultSetQueryTypeOrderByPropsParity covers
// ResultSetQueryTypeOrderByProps.
func TestResultSetQueryTypeOrderByPropsParity(t *testing.T) {
	env, engine := newIteratorMarketEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	symbol := Field[iteratorMarket, string]("symbol")
	volume := Field[iteratorMarket, int64]("volume")
	plan, err := env.Build(Select(
		From[iteratorMarket](env, "SupportMarketDataBean").Window(LengthWindow(3)),
		Alias("symbol", symbol),
		Alias("volume", volume),
	).Query(StatementName("s0"), OrderBy(Ascending(symbol), Ascending(volume))))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	sendIteratorMarket(t, engine, "SYM", 1)
	assertIteratorRows(t, statement, [][]any{{"SYM", int64(1)}}, "symbol", "volume")
	sendIteratorMarket(t, engine, "OCC", 2)
	assertIteratorRows(t, statement, [][]any{{"OCC", int64(2)}, {"SYM", int64(1)}}, "symbol", "volume")
	sendIteratorMarket(t, engine, "SYM", 0)
	assertIteratorRows(t, statement, [][]any{{"OCC", int64(2)}, {"SYM", int64(0)}, {"SYM", int64(1)}}, "symbol", "volume")
	sendIteratorMarket(t, engine, "OCC", 3)
	assertIteratorRows(t, statement, [][]any{{"OCC", int64(2)}, {"OCC", int64(3)}, {"SYM", int64(0)}}, "symbol", "volume")
}

// TestResultSetQueryTypeFilterParity covers ResultSetQueryTypeFilter: the
// iterator applies the where predicate to the window contents.
func TestResultSetQueryTypeFilterParity(t *testing.T) {
	env, engine := newIteratorMarketEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	symbol := Field[iteratorMarket, string]("symbol")
	volume := Field[iteratorMarket, int64]("volume")
	plan, err := env.Build(Select(
		From[iteratorMarket](env, "SupportMarketDataBean").Window(LengthWindow(5)).
			Filter(Less[int64](volume, Literal(int64(0)))),
		Alias("symbol", symbol),
		Alias("vol", Multiply[int64](volume, Literal(int64(10)))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	sendIteratorMarket(t, engine, "SYM", 100)
	assertIteratorRows(t, statement, nil, "symbol", "vol")
	sendIteratorMarket(t, engine, "SYM", -1)
	assertIteratorRows(t, statement, [][]any{{"SYM", int64(-10)}}, "symbol", "vol")
	sendIteratorMarket(t, engine, "SYM", -6)
	assertIteratorRows(t, statement, [][]any{{"SYM", int64(-10)}, {"SYM", int64(-60)}}, "symbol", "vol")
	sendIteratorMarket(t, engine, "SYM", 1)
	sendIteratorMarket(t, engine, "SYM", 16)
	assertIteratorRows(t, statement, [][]any{{"SYM", int64(-10)}, {"SYM", int64(-60)}}, "symbol", "vol")
	sendIteratorMarket(t, engine, "SYM", -9)
	assertIteratorRows(t, statement, [][]any{{"SYM", int64(-10)}, {"SYM", int64(-60)}, {"SYM", int64(-90)}}, "symbol", "vol")
	sendIteratorMarket(t, engine, "SYM", 2)
	assertIteratorRows(t, statement, [][]any{{"SYM", int64(-60)}, {"SYM", int64(-90)}}, "symbol", "vol")
	sendIteratorMarket(t, engine, "SYM", 3)
	assertIteratorRows(t, statement, [][]any{{"SYM", int64(-90)}}, "symbol", "vol")
	sendIteratorMarket(t, engine, "SYM", 4)
	sendIteratorMarket(t, engine, "SYM", 5)
	assertIteratorRows(t, statement, [][]any{{"SYM", int64(-90)}}, "symbol", "vol")
	sendIteratorMarket(t, engine, "SYM", 6)
	assertIteratorRows(t, statement, nil, "symbol", "vol")
}

// TestResultSetQueryTypeRowPerGroupIteratorParity covers the four row-per-group
// executions (ordered, plain, having, complex) via statement iterators.
func TestResultSetQueryTypeRowPerGroupIteratorParity(t *testing.T) {
	t.Run("ordered", func(t *testing.T) {
		env, engine := newIteratorMarketEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		symbol := Field[iteratorMarket, string]("symbol")
		volume := Field[iteratorMarket, int64]("volume")
		plan, err := env.Build(From[iteratorMarket](env, "SupportMarketDataBean").Window(LengthWindow(5)).
			GroupBy(symbol).Select(
			Alias("symbol", symbol),
			Alias("sumVol", Sum[int64](volume)),
		).Query(StatementName("s0"), OrderBy(Ascending(symbol))))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		sendIteratorMarket(t, engine, "SYM", 100)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(100)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "OCC", 5)
		assertIteratorRows(t, statement, [][]any{{"OCC", int64(5)}, {"SYM", int64(100)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "SYM", 10)
		assertIteratorRows(t, statement, [][]any{{"OCC", int64(5)}, {"SYM", int64(110)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "OCC", 6)
		assertIteratorRows(t, statement, [][]any{{"OCC", int64(11)}, {"SYM", int64(110)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "ATB", 8)
		assertIteratorRows(t, statement, [][]any{{"ATB", int64(8)}, {"OCC", int64(11)}, {"SYM", int64(110)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "ATB", 7)
		assertIteratorRows(t, statement, [][]any{{"ATB", int64(15)}, {"OCC", int64(11)}, {"SYM", int64(10)}}, "symbol", "sumVol")
	})

	t.Run("plain", func(t *testing.T) {
		env, engine := newIteratorMarketEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		symbol := Field[iteratorMarket, string]("symbol")
		volume := Field[iteratorMarket, int64]("volume")
		plan, err := env.Build(From[iteratorMarket](env, "SupportMarketDataBean").Window(LengthWindow(5)).
			GroupBy(symbol).Select(
			Alias("symbol", symbol),
			Alias("sumVol", Sum[int64](volume)),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		assertIteratorRows(t, statement, nil, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "SYM", 100)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(100)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "SYM", 10)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(110)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "TAC", 1)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(110)}, {"TAC", int64(1)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "SYM", 11)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(121)}, {"TAC", int64(1)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "TAC", 2)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(121)}, {"TAC", int64(3)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "OCC", 55)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(21)}, {"TAC", int64(3)}, {"OCC", int64(55)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "OCC", 4)
		assertIteratorRows(t, statement, [][]any{{"TAC", int64(3)}, {"SYM", int64(11)}, {"OCC", int64(59)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "OCC", 3)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(11)}, {"TAC", int64(2)}, {"OCC", int64(62)}}, "symbol", "sumVol")
	})

	t.Run("having", func(t *testing.T) {
		env, engine := newIteratorMarketEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		symbol := Field[iteratorMarket, string]("symbol")
		volume := Field[iteratorMarket, int64]("volume")
		sumVol := Sum[int64](volume)
		plan, err := env.Build(From[iteratorMarket](env, "SupportMarketDataBean").Window(LengthWindow(5)).
			GroupBy(symbol).Select(
			Alias("symbol", symbol),
			Alias("sumVol", sumVol),
		).Having(Greater[int64](sumVol, Literal(int64(10)))).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		assertIteratorRows(t, statement, nil, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "SYM", 100)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(100)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "SYM", 5)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(105)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "TAC", 1)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(105)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "SYM", 3)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(108)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "TAC", 12)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(108)}, {"TAC", int64(13)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "OCC", 55)
		assertIteratorRows(t, statement, [][]any{{"TAC", int64(13)}, {"OCC", int64(55)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "OCC", 4)
		assertIteratorRows(t, statement, [][]any{{"TAC", int64(13)}, {"OCC", int64(59)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "OCC", 3)
		assertIteratorRows(t, statement, [][]any{{"TAC", int64(12)}, {"OCC", int64(62)}}, "symbol", "sumVol")
	})

	t.Run("complex", func(t *testing.T) {
		env, engine := newIteratorMarketEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		symbol := Field[iteratorMarket, string]("symbol")
		plan, err := env.Build(From[iteratorMarket](env, "SupportMarketDataBean").
			Window(GroupWindow(symbol, LengthWindow(1))).
			Filter(GreaterOrEqual[float64](Subtract[float64](Field[iteratorMarket, float64]("price"), Cast[int64, float64](Field[iteratorMarket, int64]("volume"))), Literal(1000.0))).
			GroupBy(symbol).Select(
			Alias("symbol", symbol),
			Alias("msg", Concat(Cast[int64, string](CountAll()), Literal("x1000.0"))),
		).Having(Equal[int64](CountAll(), Literal(int64(1)))).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		assertIteratorRows(t, statement, nil, "symbol", "msg")
		sendIteratorMarketFull(t, engine, "SYM", -1, -1)
		assertIteratorRows(t, statement, nil, "symbol", "msg")
		sendIteratorMarketFull(t, engine, "SYM", 100000, 0)
		assertIteratorRows(t, statement, [][]any{{"SYM", "1x1000.0"}}, "symbol", "msg")
		sendIteratorMarketFull(t, engine, "SYM", 1, 1)
		assertIteratorRows(t, statement, nil, "symbol", "msg")
	})
}

// TestResultSetQueryTypeAggregateGroupedIteratorParity covers the three
// aggregate-grouped executions (ordered, plain, having): one iterator row per
// retained event carrying the group aggregate.
func TestResultSetQueryTypeAggregateGroupedIteratorParity(t *testing.T) {
	t.Run("ordered", func(t *testing.T) {
		env, engine := newIteratorMarketEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		symbol := Field[iteratorMarket, string]("symbol")
		price := Field[iteratorMarket, float64]("price")
		volume := Field[iteratorMarket, int64]("volume")
		plan, err := env.Build(From[iteratorMarket](env, "SupportMarketDataBean").Window(LengthWindow(5)).
			GroupBy(symbol).Select(
			Alias("symbol", symbol),
			Alias("price", price),
			Alias("sumVol", Sum[int64](volume)),
		).Query(StatementName("s0"), OrderBy(Ascending(ResultField[string]("symbol")))))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		assertIteratorRows(t, statement, nil, "symbol", "price", "sumVol")
		sendIteratorMarketFull(t, engine, "SYM", -1, 100)
		assertIteratorRows(t, statement, [][]any{{"SYM", -1.0, int64(100)}}, "symbol", "price", "sumVol")
		sendIteratorMarketFull(t, engine, "TAC", -2, 12)
		assertIteratorRows(t, statement, [][]any{{"SYM", -1.0, int64(100)}, {"TAC", -2.0, int64(12)}}, "symbol", "price", "sumVol")
		sendIteratorMarketFull(t, engine, "TAC", -3, 13)
		assertIteratorRows(t, statement, [][]any{{"SYM", -1.0, int64(100)}, {"TAC", -2.0, int64(25)}, {"TAC", -3.0, int64(25)}}, "symbol", "price", "sumVol")
		sendIteratorMarketFull(t, engine, "SYM", -4, 1)
		assertIteratorRows(t, statement, [][]any{{"SYM", -1.0, int64(101)}, {"SYM", -4.0, int64(101)}, {"TAC", -2.0, int64(25)}, {"TAC", -3.0, int64(25)}}, "symbol", "price", "sumVol")
		sendIteratorMarketFull(t, engine, "OCC", -5, 99)
		assertIteratorRows(t, statement, [][]any{{"OCC", -5.0, int64(99)}, {"SYM", -1.0, int64(101)}, {"SYM", -4.0, int64(101)}, {"TAC", -2.0, int64(25)}, {"TAC", -3.0, int64(25)}}, "symbol", "price", "sumVol")
		sendIteratorMarketFull(t, engine, "TAC", -6, 2)
		assertIteratorRows(t, statement, [][]any{{"OCC", -5.0, int64(99)}, {"SYM", -4.0, int64(1)}, {"TAC", -2.0, int64(27)}, {"TAC", -3.0, int64(27)}, {"TAC", -6.0, int64(27)}}, "symbol", "price", "sumVol")
	})

	t.Run("plain", func(t *testing.T) {
		env, engine := newIteratorMarketEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		symbol := Field[iteratorMarket, string]("symbol")
		price := Field[iteratorMarket, float64]("price")
		volume := Field[iteratorMarket, int64]("volume")
		plan, err := env.Build(From[iteratorMarket](env, "SupportMarketDataBean").Window(LengthWindow(5)).
			GroupBy(symbol).Select(
			Alias("symbol", symbol),
			Alias("price", price),
			Alias("sumVol", Sum[int64](volume)),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		sendIteratorMarketFull(t, engine, "SYM", -1, 100)
		sendIteratorMarketFull(t, engine, "TAC", -2, 12)
		sendIteratorMarketFull(t, engine, "TAC", -3, 13)
		sendIteratorMarketFull(t, engine, "SYM", -4, 1)
		sendIteratorMarketFull(t, engine, "OCC", -5, 99)
		assertIteratorRows(t, statement, [][]any{{"SYM", -1.0, int64(101)}, {"TAC", -2.0, int64(25)}, {"TAC", -3.0, int64(25)}, {"SYM", -4.0, int64(101)}, {"OCC", -5.0, int64(99)}}, "symbol", "price", "sumVol")
		sendIteratorMarketFull(t, engine, "TAC", -6, 2)
		assertIteratorRows(t, statement, [][]any{{"TAC", -2.0, int64(27)}, {"TAC", -3.0, int64(27)}, {"SYM", -4.0, int64(1)}, {"OCC", -5.0, int64(99)}, {"TAC", -6.0, int64(27)}}, "symbol", "price", "sumVol")
	})

	t.Run("having", func(t *testing.T) {
		env, engine := newIteratorMarketEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		symbol := Field[iteratorMarket, string]("symbol")
		price := Field[iteratorMarket, float64]("price")
		volume := Field[iteratorMarket, int64]("volume")
		sumVol := Sum[int64](volume)
		plan, err := env.Build(From[iteratorMarket](env, "SupportMarketDataBean").Window(LengthWindow(5)).
			GroupBy(symbol).Select(
			Alias("symbol", symbol),
			Alias("price", price),
			Alias("sumVol", sumVol),
		).Having(Greater[int64](sumVol, Literal(int64(20)))).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		sendIteratorMarketFull(t, engine, "SYM", -1, 100)
		assertIteratorRows(t, statement, [][]any{{"SYM", -1.0, int64(100)}}, "symbol", "price", "sumVol")
		sendIteratorMarketFull(t, engine, "TAC", -2, 12)
		assertIteratorRows(t, statement, [][]any{{"SYM", -1.0, int64(100)}}, "symbol", "price", "sumVol")
		sendIteratorMarketFull(t, engine, "TAC", -3, 13)
		assertIteratorRows(t, statement, [][]any{{"SYM", -1.0, int64(100)}, {"TAC", -2.0, int64(25)}, {"TAC", -3.0, int64(25)}}, "symbol", "price", "sumVol")
		sendIteratorMarketFull(t, engine, "SYM", -4, 1)
		assertIteratorRows(t, statement, [][]any{{"SYM", -1.0, int64(101)}, {"TAC", -2.0, int64(25)}, {"TAC", -3.0, int64(25)}, {"SYM", -4.0, int64(101)}}, "symbol", "price", "sumVol")
		sendIteratorMarketFull(t, engine, "OCC", -5, 99)
		assertIteratorRows(t, statement, [][]any{{"SYM", -1.0, int64(101)}, {"TAC", -2.0, int64(25)}, {"TAC", -3.0, int64(25)}, {"SYM", -4.0, int64(101)}, {"OCC", -5.0, int64(99)}}, "symbol", "price", "sumVol")
		sendIteratorMarketFull(t, engine, "TAC", -6, 2)
		assertIteratorRows(t, statement, [][]any{{"TAC", -2.0, int64(27)}, {"TAC", -3.0, int64(27)}, {"OCC", -5.0, int64(99)}, {"TAC", -6.0, int64(27)}}, "symbol", "price", "sumVol")
	})
}

// TestResultSetQueryTypeRowPerEventIteratorParity covers the three
// row-per-event executions: ungrouped aggregate with one iterator row per
// retained event.
func TestResultSetQueryTypeRowPerEventIteratorParity(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		env, engine := newIteratorMarketEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		symbol := Field[iteratorMarket, string]("symbol")
		volume := Field[iteratorMarket, int64]("volume")
		plan, err := env.Build(From[iteratorMarket](env, "SupportMarketDataBean").Window(LengthWindow(3)).
			Aggregate(
				Alias("symbol", symbol),
				Alias("sumVol", Sum[int64](volume)),
			).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		assertIteratorRows(t, statement, nil, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "SYM", 100)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(100)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "TAC", 1)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(101)}, {"TAC", int64(101)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "MOV", 3)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(104)}, {"TAC", int64(104)}, {"MOV", int64(104)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "SYM", 10)
		assertIteratorRows(t, statement, [][]any{{"TAC", int64(14)}, {"MOV", int64(14)}, {"SYM", int64(14)}}, "symbol", "sumVol")
	})

	t.Run("ordered", func(t *testing.T) {
		env, engine := newIteratorMarketEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		symbol := Field[iteratorMarket, string]("symbol")
		volume := Field[iteratorMarket, int64]("volume")
		plan, err := env.Build(From[iteratorMarket](env, "SupportMarketDataBean").Window(LengthWindow(3)).
			Aggregate(
				Alias("symbol", symbol),
				Alias("sumVol", Sum[int64](volume)),
			).Query(StatementName("s0"), WithOldStream(), OrderBy(Ascending(ResultField[string]("symbol")))))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		sendIteratorMarket(t, engine, "SYM", 100)
		sendIteratorMarket(t, engine, "TAC", 1)
		sendIteratorMarket(t, engine, "MOV", 3)
		assertIteratorRows(t, statement, [][]any{{"MOV", int64(104)}, {"SYM", int64(104)}, {"TAC", int64(104)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "SYM", 10)
		assertIteratorRows(t, statement, [][]any{{"MOV", int64(14)}, {"SYM", int64(14)}, {"TAC", int64(14)}}, "symbol", "sumVol")
	})

	t.Run("having", func(t *testing.T) {
		env, engine := newIteratorMarketEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		symbol := Field[iteratorMarket, string]("symbol")
		volume := Field[iteratorMarket, int64]("volume")
		sumVol := Sum[int64](volume)
		plan, err := env.Build(From[iteratorMarket](env, "SupportMarketDataBean").Window(LengthWindow(3)).
			Aggregate(
				Alias("symbol", symbol),
				Alias("sumVol", sumVol),
			).Having(Greater[int64](sumVol, Literal(int64(100)))).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		assertIteratorRows(t, statement, nil, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "SYM", 100)
		assertIteratorRows(t, statement, nil, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "TAC", 1)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(101)}, {"TAC", int64(101)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "MOV", 3)
		assertIteratorRows(t, statement, [][]any{{"SYM", int64(104)}, {"TAC", int64(104)}, {"MOV", int64(104)}}, "symbol", "sumVol")
		sendIteratorMarket(t, engine, "SYM", 10)
		assertIteratorRows(t, statement, nil, "symbol", "sumVol")
	})
}

// TestResultSetQueryTypeRowForAllIteratorParity covers the two row-for-all
// executions: ungrouped aggregate iterator with and without having.
func TestResultSetQueryTypeRowForAllIteratorParity(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		env, engine := newIteratorMarketEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		volume := Field[iteratorMarket, int64]("volume")
		plan, err := env.Build(From[iteratorMarket](env, "SupportMarketDataBean").Window(LengthWindow(3)).
			Aggregate(Alias("sumVol", Sum[int64](volume))).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		assertIteratorRows(t, statement, [][]any{{nil}}, "sumVol")
		sendIteratorMarket(t, engine, "SYM", 100)
		assertIteratorRows(t, statement, [][]any{{int64(100)}}, "sumVol")
		sendIteratorMarket(t, engine, "SYM", 50)
		assertIteratorRows(t, statement, [][]any{{int64(150)}}, "sumVol")
		sendIteratorMarket(t, engine, "SYM", 25)
		assertIteratorRows(t, statement, [][]any{{int64(175)}}, "sumVol")
		sendIteratorMarket(t, engine, "SYM", 10)
		assertIteratorRows(t, statement, [][]any{{int64(85)}}, "sumVol")
	})

	t.Run("having", func(t *testing.T) {
		env, engine := newIteratorMarketEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		volume := Field[iteratorMarket, int64]("volume")
		sumVol := Sum[int64](volume)
		plan, err := env.Build(From[iteratorMarket](env, "SupportMarketDataBean").Window(LengthWindow(3)).
			Aggregate(Alias("sumVol", sumVol)).
			Having(Greater[int64](sumVol, Literal(int64(100)))).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		assertIteratorRows(t, statement, nil, "sumVol")
		sendIteratorMarket(t, engine, "SYM", 100)
		assertIteratorRows(t, statement, nil, "sumVol")
		sendIteratorMarket(t, engine, "SYM", 50)
		assertIteratorRows(t, statement, [][]any{{int64(150)}}, "sumVol")
		sendIteratorMarket(t, engine, "SYM", 25)
		assertIteratorRows(t, statement, [][]any{{int64(175)}}, "sumVol")
		sendIteratorMarket(t, engine, "SYM", 10)
		assertIteratorRows(t, statement, nil, "sumVol")
	})
}

// TestResultSetQueryTypePatternIteratorParity covers the two pattern iterator
// executions: an unbound iterable pattern and a lastevent-windowed pattern.
func TestResultSetQueryTypePatternIteratorParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[iteratorBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	addressStream := From[iteratorBean](env, "SupportBean")
	theString := Field[iteratorBean, string]("theString")
	intBoxed := Field[iteratorBean, int]("intBoxed")
	pattern := PatternFrom(addressStream, "address", Equal[string](theString, Literal("address"))).
		Then(PatternFrom(addressStream, "txnWD", And(
			Equal[string](theString, Literal("txn")),
			Equal[int](intBoxed, TagField[int]("address", "intBoxed")),
		))).Every()
	query := pattern.Select(
		Alias("addressInfo", PatternEvent("address")),
		Alias("txnWD", PatternEvent("txnWD")),
	).Query(StatementName("s0"), WithIterableUnbound())
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	sendBean := func(theStringValue string, intBoxedValue int) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean", iteratorBean{TheString: theStringValue, IntBoxed: intBoxedValue}); err != nil {
			t.Fatal(err)
		}
	}
	sendBean("address", 9001)
	if snapshot, err := statement.Snapshot(context.Background()); err != nil || len(snapshot.Results()) != 0 {
		t.Fatalf("partial pattern snapshot = %v, err=%v", snapshot.Results(), err)
	}
	sendBean("txn", 9001)
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 1 {
		t.Fatalf("completed pattern snapshot = %v", snapshot.Results())
	}
	row := snapshot.Results()[0]
	addressEvent, ok := row.Get("addressInfo").Any().(Event)
	if !ok {
		t.Fatalf("addressInfo = %#v", row.Get("addressInfo"))
	}
	if addressEvent.Underlying() != (iteratorBean{TheString: "address", IntBoxed: 9001}) {
		t.Fatalf("address underlying = %#v", addressEvent.Underlying())
	}
	txnEvent, ok := row.Get("txnWD").Any().(Event)
	if !ok {
		t.Fatalf("txnWD = %#v", row.Get("txnWD"))
	}
	if txnEvent.Underlying() != (iteratorBean{TheString: "txn", IntBoxed: 9001}) {
		t.Fatalf("txn underlying = %#v", txnEvent.Underlying())
	}
}

// TestResultSetQueryTypePatternWithWindowIteratorParity covers the lastevent
// windowed pattern via the typed insert-into route equivalent.
func TestResultSetQueryTypePatternWithWindowIteratorParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[iteratorBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "PatternMatches", []FieldSpec{
		FieldDef("addressInfo", reflect.TypeOf((*any)(nil)).Elem()),
		FieldDef("txnWD", reflect.TypeOf((*any)(nil)).Elem()),
	}); err != nil {
		t.Fatal(err)
	}
	addressStream := From[iteratorBean](env, "SupportBean")
	theString := Field[iteratorBean, string]("theString")
	intBoxed := Field[iteratorBean, int]("intBoxed")
	pattern := PatternFrom(addressStream, "address", Equal[string](theString, Literal("address"))).
		Then(PatternFrom(addressStream, "txnWD", And(
			Equal[string](theString, Literal("txn")),
			Equal[int](intBoxed, TagField[int]("address", "intBoxed")),
		))).Every()
	producerPlan, err := env.Build(pattern.Select(
		Alias("addressInfo", PatternEvent("address")),
		Alias("txnWD", PatternEvent("txnWD")),
	).InsertInto("PatternMatches", StatementName("producer")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromAny(env, "PatternMatches").Window(LastEvent()).
		Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment := deployInfraNWTPlans(t, engine, []Plan{producerPlan, consumerPlan})
	statement := infraNWTStatementByDeploymentName(t, deployment, "s0")
	sendBean := func(theStringValue string, intBoxedValue int) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean", iteratorBean{TheString: theStringValue, IntBoxed: intBoxedValue}); err != nil {
			t.Fatal(err)
		}
	}
	sendBean("address", 9001)
	sendBean("txn", 9001)
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 1 {
		t.Fatalf("pattern window snapshot = %v", snapshot.Results())
	}
}
