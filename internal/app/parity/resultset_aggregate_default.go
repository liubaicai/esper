package parity

import (
	"context"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetAggregateDefaultJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateDefaultJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAggregateGrouped.java",
}

var (
	resultsetAggregateDefaultJavaRuntimeIDs = []string{
		"java-runtime-fd011003ea3fe327b12c",
	}
	resultsetAggregateDefaultJavaExecutions = []string{
		"ResultSetNoJoinDefault",
	}
)

func runResultSetAggregateDefaultScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "default"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-default scenario %q has no supported cases", scenario.ID)
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
		Window(esper.LengthWindow(5)).
		Filter(esper.Or(
			esper.Or(
				esper.Equal[string](symbol, esper.Literal("DELL")),
				esper.Equal[string](symbol, esper.Literal("IBM")),
			),
			esper.Equal[string](symbol, esper.Literal("GE")),
		)).
		GroupBy(symbol).
		Select(
			esper.Alias("symbol", symbol),
			esper.Alias("volume", volume),
			esper.Alias("mySum", esper.Sum[float64](price)),
		).Query(
		esper.StatementName("s0"),
		esper.WithOutput(esper.OutputEvery(2)),
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
			return nil, fmt.Errorf("unknown resultset-aggregate-default statement %q", name)
		}
		return statement, nil
	})
}
