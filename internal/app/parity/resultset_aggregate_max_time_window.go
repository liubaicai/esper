package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetAggregateMaxTimeWindowJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateMaxTimeWindowJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAggregateGrouped.java",
}

var (
	resultsetAggregateMaxTimeWindowJavaRuntimeIDs = []string{
		"java-runtime-a137a3b5c9a0cbc904cf",
	}
	resultsetAggregateMaxTimeWindowJavaExecutions = []string{
		"ResultSetMaxTimeWindow",
	}
)

// runResultSetAggregateMaxTimeWindowScenario replays the time(1 sec) grouped
// max with output every 1 seconds, where the first output boundary coincides
// with the expiry of both buffered events, matching ResultSetMaxTimeWindow.
func runResultSetAggregateMaxTimeWindowScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "max-output"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-max-time-window scenario %q has no supported cases", scenario.ID)
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
		Window(esper.TimeWindow(time.Second)).
		GroupBy(symbol).
		Select(
			esper.Alias("symbol", symbol),
			esper.Alias("volume", volume),
			esper.Alias("maxVol", esper.Max[float64](price)),
		).Query(
		esper.StatementName("s0"),
		esper.WithOldStream(),
		esper.WithOutput(esper.OutputEveryTime(time.Second)),
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
			return nil, fmt.Errorf("unknown resultset-aggregate-max-time-window statement %q", name)
		}
		return statement, nil
	})
}
