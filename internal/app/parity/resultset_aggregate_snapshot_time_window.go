package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetAggregateSnapshotTimeWindowJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateSnapshotTimeWindowJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAggregateGrouped.java",
}

var (
	resultsetAggregateSnapshotTimeWindowJavaRuntimeIDs = []string{
		"java-runtime-e5b7968f1c5225f9cce2",
	}
	resultsetAggregateSnapshotTimeWindowJavaExecutions = []string{
		"ResultSet18SnapshotNoHavingNoJoin",
	}
)

// runResultSetAggregateSnapshotTimeWindowScenario replays the grouped
// time-window sum with output snapshot every 1 seconds, emitting one row per
// retained event carrying the current group aggregate, matching
// ResultSet18SnapshotNoHavingNoJoin.
func runResultSetAggregateSnapshotTimeWindowScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "snapshot-output"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-snapshot-time-window scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	symbol := esper.Field[resultsetGroupedTimeWindowMarket, string]("symbol")
	volume := esper.Field[resultsetGroupedTimeWindowMarket, int64]("volume")
	price := esper.Field[resultsetGroupedTimeWindowMarket, float64]("price")
	query := esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
		Window(esper.TimeWindow(5500*time.Millisecond)).
		GroupBy(symbol).
		Select(
			esper.Alias("symbol", symbol),
			esper.Alias("volume", volume),
			esper.Alias("sum(price)", esper.Sum[float64](price)),
		).Query(
		esper.StatementName("s0"),
		esper.WithOutput(esper.OutputSnapshotEvery(time.Second)),
	)
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetGroupedTimeWindowPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-snapshot-time-window statement %q", name)
		}
		return statement, nil
	})
}
