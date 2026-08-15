package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetAggregateTimeWindowJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateTimeWindowJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAggregateGrouped.java",
}

var (
	resultsetAggregateTimeWindowJavaRuntimeIDs = []string{
		"java-runtime-a7dd740df8a4fa0a88e6",
		"java-runtime-a217721770427537aa2c",
	}
	resultsetAggregateTimeWindowJavaExecutions = []string{
		"ResultSet5DefaultNoHavingNoJoin",
		"ResultSet7DefaultHavingNoJoin",
	}
)

// runResultSetAggregateTimeWindowScenario replays the grouped time-window sum
// with output every 1 seconds, including irstream old-row emission at window
// expiry, matching ResultSet5DefaultNoHavingNoJoin (default-output) and
// ResultSet7DefaultHavingNoJoin (default-having).
func runResultSetAggregateTimeWindowScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"default-output", "default-having"}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-time-window scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runResultSetAggregateTimeWindowCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-time-window case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateTimeWindowCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
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
	sum := esper.Sum[float64](price)
	grouped := esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
		Window(esper.TimeWindow(5500 * time.Millisecond)).
		GroupBy(symbol)
	var query esper.Query
	if caseName == "default-having" {
		query = grouped.Having(esper.Greater[float64](sum, esper.Literal(50.0))).Select(
			esper.Alias("symbol", symbol),
			esper.Alias("volume", volume),
			esper.Alias("sum(price)", sum),
		).Query(
			esper.StatementName("s0"),
			esper.WithOldStream(),
			esper.WithOutput(esper.OutputEveryTime(time.Second)),
		)
	} else {
		query = grouped.Select(
			esper.Alias("symbol", symbol),
			esper.Alias("volume", volume),
			esper.Alias("sum(price)", sum),
		).Query(
			esper.StatementName("s0"),
			esper.WithOldStream(),
			esper.WithOutput(esper.OutputEveryTime(time.Second)),
		)
	}
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
			return nil, fmt.Errorf("unknown resultset-aggregate-time-window statement %q", name)
		}
		return statement, nil
	})
}
