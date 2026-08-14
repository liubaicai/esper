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
		if len(batches[0].New) != 2 || len(batches[0].Old) != 2 {
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
		if len(batches[0].New) != 2 || len(batches[0].Old) != 2 {
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
		if len(batches) != 1 || len(batches[0].New) != 2 || len(batches[0].Old) < 2 {
			t.Fatalf("max-time-window batches = %#v", batches)
		}
	})
}
