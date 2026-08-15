package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const rollupOutputFirstMarketJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var rollupOutputFirstMarketJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowPerGroupRollup.java",
}

var (
	rollupOutputFirstMarketJavaRuntimeIDs = []string{
		"java-runtime-1e0ad81982656cd82bcd",
	}
	rollupOutputFirstMarketJavaExecutions = []string{
		"ResultSet5OutputLimitFirst",
	}
)

func runRollupOutputFirstMarketScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "first-market"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("rollup-output-first-market scenario %q has no supported cases", scenario.ID)
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
	price := esper.Field[resultsetGroupedTimeWindowMarket, float64]("price")
	query := esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
		Window(esper.TimeWindow(5500*time.Millisecond)).
		GroupByRollup(symbol).
		Select(
			esper.Alias("symbol", symbol),
			esper.Alias("sum(price)", esper.Sum[float64](price)),
		).Query(
		esper.StatementName("s0"),
		esper.WithOldStream(),
		esper.WithOutput(esper.OutputFirstEveryTime(time.Second)),
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
			return nil, fmt.Errorf("unknown rollup-output-first-market statement %q", name)
		}
		return statement, nil
	})
}
