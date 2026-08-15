package esper

import (
	"context"
	"testing"
	"time"
)

func aggregateGroupedBuild(t *testing.T, env *Environment, policy OutputPolicy, having bool) Plan {
	t.Helper()
	symbol := Field[resultsetGroupedTimeWindowMarket, string]("symbol")
	volume := Field[resultsetGroupedTimeWindowMarket, int64]("volume")
	price := Field[resultsetGroupedTimeWindowMarket, float64]("price")
	sum := Sum[float64](price)
	query := From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
		Window(TimeWindow(5500*time.Millisecond)).
		GroupBy(symbol).
		Select(
			Alias("symbol", symbol),
			Alias("volume", volume),
			Alias("sum(price)", sum),
		)
	if having {
		query = query.Having(Greater[float64](sum, Literal(50.0)))
	}
	plan, err := env.Build(query.Query(StatementName("s0"), WithOldStream(), WithOutput(policy)))
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func aggregateGroupedScenario(t *testing.T, engine *Engine) {
	t.Helper()
	send := func(symbol string, volume int64, price float64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), resultsetGroupedTimeWindowMarket{Symbol: symbol, Volume: volume, Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	advance := func(ms int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(ms).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	advance(0)
	send("IBM", 100, 25)
	advance(800)
	send("MSFT", 5000, 9)
	advance(1500)
	send("IBM", 150, 24)
	send("YAH", 10000, 1)
	advance(2100)
	send("IBM", 155, 26)
	advance(3500)
	send("YAH", 11000, 2)
	advance(4300)
	send("IBM", 150, 22)
	advance(4900)
	send("YAH", 11500, 3)
	advance(5700)
	send("YAH", 10500, 1)
	advance(5900)
	advance(6300)
	advance(7000)
}

// TestResultSetOutputLimitAggregateGroupedParity covers eight executions from
// ResultSetOutputLimitAggregateGrouped using the shared grouped time-window
// scenario: ResultSet5DefaultNoHavingNoJoin, ResultSet7DefaultHavingNoJoin,
// ResultSet13LastNoHavingNoJoin, ResultSet15LastHavingNoJoin,
// ResultSet17FirstNoHavingNoJoin, ResultSet18SnapshotNoHavingNoJoin,
// ResultSetNoJoinDefault and ResultSetNoOutputClauseView.
func TestResultSetOutputLimitAggregateGroupedParity(t *testing.T) {
	t.Run("default-no-having", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
		defer func() { _ = engine.Close(context.Background()) }()
		deployment, err := engine.Deploy(context.Background(), aggregateGroupedBuild(t, env, OutputEveryTime(time.Second), false))
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
		aggregateGroupedScenario(t, engine)
		if len(batches) < 2 {
			t.Fatalf("default-no-having batches = %#v", batches)
		}
		if len(batches[0].New) != 2 || len(batches[0].Old) != 0 {
			t.Fatalf("default-no-having first batch = %#v", batches[0])
		}
	})

	t.Run("default-having", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
		defer func() { _ = engine.Close(context.Background()) }()
		deployment, err := engine.Deploy(context.Background(), aggregateGroupedBuild(t, env, OutputEveryTime(time.Second), true))
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
		aggregateGroupedScenario(t, engine)
		found := false
		for _, batch := range batches {
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok || row.Get("symbol").Any() != "IBM" || row.Get("sum(price)").Any() != float64(75) {
					continue
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("default-having did not emit IBM 75: %#v", batches)
		}
	})

	t.Run("last-no-having", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
		defer func() { _ = engine.Close(context.Background()) }()
		deployment, err := engine.Deploy(context.Background(), aggregateGroupedBuild(t, env, OutputLastEveryTime(time.Second), false))
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
		aggregateGroupedScenario(t, engine)
		if len(batches) < 2 {
			t.Fatalf("last-no-having batches = %#v", batches)
		}
		if len(batches[0].New) != 2 || len(batches[0].Old) != 0 {
			t.Fatalf("last-no-having first batch = %#v", batches[0])
		}
	})

	t.Run("last-having", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
		defer func() { _ = engine.Close(context.Background()) }()
		deployment, err := engine.Deploy(context.Background(), aggregateGroupedBuild(t, env, OutputLastEveryTime(time.Second), true))
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
		aggregateGroupedScenario(t, engine)
		found := false
		for _, batch := range batches {
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok || row.Get("symbol").Any() != "IBM" || row.Get("sum(price)").Any() != float64(75) {
					continue
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("last-having did not emit IBM 75: %#v", batches)
		}
	})

	t.Run("first-no-having", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
		defer func() { _ = engine.Close(context.Background()) }()
		deployment, err := engine.Deploy(context.Background(), aggregateGroupedBuild(t, env, OutputFirstEveryTime(time.Second), false))
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
		aggregateGroupedScenario(t, engine)
		if len(batches) < 2 {
			t.Fatalf("first-no-having batches = %#v", batches)
		}
	})

	t.Run("snapshot-no-having", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
		defer func() { _ = engine.Close(context.Background()) }()
		deployment, err := engine.Deploy(context.Background(), aggregateGroupedBuild(t, env, OutputSnapshotEvery(time.Second), false))
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
		aggregateGroupedScenario(t, engine)
		if len(batches) < 2 || len(batches[0].New) != 2 || len(batches[0].Old) != 0 {
			t.Fatalf("snapshot-no-having batches = %#v", batches)
		}
	})

	t.Run("no-output-clause-view", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		symbol := Field[resultsetGroupedTimeWindowMarket, string]("symbol")
		price := Field[resultsetGroupedTimeWindowMarket, float64]("price")
		plan, err := env.Build(From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
			Window(LengthWindow(5)).
			GroupBy(symbol).
			Select(
				Alias("symbol", symbol),
				Alias("volume", Field[resultsetGroupedTimeWindowMarket, int64]("volume")),
				Alias("mySum", Sum[float64](price)),
			).Query(StatementName("s0")))
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
		if err := engine.SendEvent(context.Background(), resultsetGroupedTimeWindowMarket{Symbol: "IBM", Volume: 10, Price: 100}); err != nil {
			t.Fatal(err)
		}
		if len(batches) != 1 || len(batches[0].New) != 1 {
			t.Fatalf("no-output-clause-view batches = %#v", batches)
		}
	})

	t.Run("no-join-default", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		symbol := Field[resultsetGroupedTimeWindowMarket, string]("symbol")
		price := Field[resultsetGroupedTimeWindowMarket, float64]("price")
		plan, err := env.Build(From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
			Window(LengthWindow(5)).
			GroupBy(symbol).
			Select(
				Alias("symbol", symbol),
				Alias("volume", Field[resultsetGroupedTimeWindowMarket, int64]("volume")),
				Alias("mySum", Sum[float64](price)),
			).Query(StatementName("s0"), WithOutput(OutputEvery(2))))
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
		if err := engine.SendEvent(context.Background(), resultsetGroupedTimeWindowMarket{Symbol: "IBM", Volume: 10, Price: 20}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), resultsetGroupedTimeWindowMarket{Symbol: "DELL", Volume: 20, Price: 51}); err != nil {
			t.Fatal(err)
		}
		if len(batches) != 1 || len(batches[0].New) != 2 {
			t.Fatalf("no-join-default batches = %#v", batches)
		}
	})

	t.Run("no-join-last", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		symbol := Field[resultsetGroupedTimeWindowMarket, string]("symbol")
		price := Field[resultsetGroupedTimeWindowMarket, float64]("price")
		plan, err := env.Build(From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
			Window(LengthWindow(5)).
			GroupBy(symbol).
			Select(
				Alias("symbol", symbol),
				Alias("volume", Field[resultsetGroupedTimeWindowMarket, int64]("volume")),
				Alias("mySum", Sum[float64](price)),
			).Query(StatementName("s0"), WithOutput(OutputLastEveryEvents(2))))
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
		if err := engine.SendEvent(context.Background(), resultsetGroupedTimeWindowMarket{Symbol: "DELL", Volume: 10, Price: 51}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), resultsetGroupedTimeWindowMarket{Symbol: "DELL", Volume: 20, Price: 52}); err != nil {
			t.Fatal(err)
		}
		if len(batches) != 1 || len(batches[0].New) != 1 {
			t.Fatalf("no-join-last batches = %#v", batches)
		}
	})

	t.Run("max-time-window", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
		defer func() { _ = engine.Close(context.Background()) }()
		symbol := Field[resultsetGroupedTimeWindowMarket, string]("symbol")
		price := Field[resultsetGroupedTimeWindowMarket, float64]("price")
		plan, err := env.Build(From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
			Window(TimeWindow(time.Second)).
			GroupBy(symbol).
			Select(
				Alias("symbol", symbol),
				Alias("volume", Field[resultsetGroupedTimeWindowMarket, int64]("volume")),
				Alias("maxVol", Max[float64](price)),
			).Query(StatementName("s0"), WithOldStream(), WithOutput(OutputEveryTime(time.Second))))
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
		if err := engine.SendEvent(context.Background(), resultsetGroupedTimeWindowMarket{Symbol: "SYM1", Volume: 1, Price: 1}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), resultsetGroupedTimeWindowMarket{Symbol: "SYM1", Volume: 2, Price: 2}); err != nil {
			t.Fatal(err)
		}
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(1000).UTC()); err != nil {
			t.Fatal(err)
		}
		if len(batches) != 1 || len(batches[0].New) != 2 || len(batches[0].Old) != 2 {
			t.Fatalf("max-time-window batches = %#v", batches)
		}
		newRow, ok := batches[0].New[0].Row()
		if !ok || newRow.Get("symbol").Any() != "SYM1" || newRow.Get("volume").Any() != int64(1) || newRow.Get("maxVol").Any() != float64(1) {
			t.Fatalf("max-time-window first new row = %#v", batches[0].New[0])
		}
		newRow, ok = batches[0].New[1].Row()
		if !ok || newRow.Get("symbol").Any() != "SYM1" || newRow.Get("volume").Any() != int64(2) || newRow.Get("maxVol").Any() != float64(2) {
			t.Fatalf("max-time-window second new row = %#v", batches[0].New[1])
		}
		for index := 0; index < 2; index++ {
			oldRow, oldOK := batches[0].Old[index].Row()
			if !oldOK || oldRow.Get("symbol").Any() != "SYM1" || oldRow.Get("volume").Any() != int64(index+1) || !oldRow.Get("maxVol").IsNull() {
				t.Fatalf("max-time-window old row %d = %#v", index, batches[0].Old[index])
			}
		}
	})
}

// TestResultSetOutputLimitAggregateGroupedAllEventsParity mirrors
// ResultSetNoJoinAll: a length(5) grouped sum with output all every 2 events
// emits one accumulated row per event and re-posts representative rows of
// untouched groups at each boundary.
func TestResultSetOutputLimitAggregateGroupedAllEventsParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	symbol := Field[resultsetGroupedTimeWindowMarket, string]("symbol")
	volume := Field[resultsetGroupedTimeWindowMarket, int64]("volume")
	price := Field[resultsetGroupedTimeWindowMarket, float64]("price")
	plan, err := env.Build(From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
		Window(LengthWindow(5)).
		Filter(Or(
			Or(Equal[string](symbol, Literal("DELL")), Equal[string](symbol, Literal("IBM"))),
			Equal[string](symbol, Literal("GE")),
		)).
		GroupBy(symbol).
		Select(
			Alias("symbol", symbol),
			Alias("volume", volume),
			Alias("mySum", Sum[float64](price)),
		).Query(StatementName("s0"), WithOutput(OutputAllEveryEvents(2))))
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
	send := func(symbol string, volume int64, price float64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), resultsetGroupedTimeWindowMarket{Symbol: symbol, Volume: volume, Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	send("IBM", 500, 20)
	send("DELL", 10000, 51)
	send("DELL", 20000, 52)
	send("DELL", 40000, 45)
	if len(batches) != 2 || len(batches[0].Old) != 0 || len(batches[1].Old) != 0 {
		t.Fatalf("all-every-events batches = %#v", batches)
	}
	assertRow := func(batch ResultBatch, index int, symbol string, volume int64, mySum float64) {
		t.Helper()
		row, ok := batch.New[index].Row()
		if !ok || row.Get("symbol").Any() != symbol || row.Get("volume").Any() != volume || row.Get("mySum").Any() != mySum {
			t.Fatalf("all-every-events row %d = %#v", index, batch.New[index])
		}
	}
	assertRow(batches[0], 0, "IBM", 500, 20)
	assertRow(batches[0], 1, "DELL", 10000, 51)
	assertRow(batches[1], 0, "DELL", 20000, 103)
	assertRow(batches[1], 1, "DELL", 40000, 148)
	assertRow(batches[1], 2, "IBM", 500, 20)
}

// TestResultSetHavingEveryEventsParity mirrors ResultSetHaving: a time(10 sec)
// grouped sum with having sum(price) >= 10 and output every 3 events counts
// input events even when having filters them, and a pure three-event removal
// batch triggers the old-stream output.
func TestResultSetHavingEveryEventsParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	symbol := Field[resultsetGroupedTimeWindowMarket, string]("symbol")
	volume := Field[resultsetGroupedTimeWindowMarket, int64]("volume")
	price := Field[resultsetGroupedTimeWindowMarket, float64]("price")
	sum := Sum[float64](price)
	plan, err := env.Build(From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
		Window(TimeWindow(10*time.Second)).
		GroupBy(symbol).
		Having(GreaterOrEqual[float64](sum, Literal(10.0))).
		Select(
			Alias("symbol", symbol),
			Alias("volume", volume),
			Alias("sumprice", sum),
		).Query(StatementName("s0"), WithOldStream(), WithOutput(OutputEvery(3))))
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
	send := func(volume int64, price float64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), resultsetGroupedTimeWindowMarket{Symbol: "IBM", Volume: volume, Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	send(1, 5)
	send(2, 6)
	send(3, -3)
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(5000).UTC()); err != nil {
		t.Fatal(err)
	}
	send(4, 10)
	send(5, 0)
	send(6, 1)
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(11000).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 3 {
		t.Fatalf("having-every-events batches = %#v", batches)
	}
	assertHavingRow := func(batch ResultBatch, side string, index int, volume int64, sumprice float64) {
		t.Helper()
		var row Row
		var ok bool
		if side == "new" {
			row, ok = batch.New[index].Row()
		} else {
			row, ok = batch.Old[index].Row()
		}
		if !ok || row.Get("symbol").Any() != "IBM" || row.Get("volume").Any() != volume || row.Get("sumprice").Any() != sumprice {
			t.Fatalf("having-every-events %s row %d = %#v", side, index, batch)
		}
	}
	if len(batches[0].New) != 1 || len(batches[0].Old) != 0 {
		t.Fatalf("having-every-events first batch = %#v", batches[0])
	}
	assertHavingRow(batches[0], "new", 0, 2, 11)
	if len(batches[1].New) != 3 || len(batches[1].Old) != 0 {
		t.Fatalf("having-every-events second batch = %#v", batches[1])
	}
	assertHavingRow(batches[1], "new", 0, 4, 18)
	assertHavingRow(batches[1], "new", 1, 5, 18)
	assertHavingRow(batches[1], "new", 2, 6, 19)
	if len(batches[2].New) != 0 || len(batches[2].Old) != 3 {
		t.Fatalf("having-every-events expiry batch = %#v", batches[2])
	}
	assertHavingRow(batches[2], "old", 0, 1, 11)
	assertHavingRow(batches[2], "old", 1, 2, 11)
	assertHavingRow(batches[2], "old", 2, 3, 11)
}
