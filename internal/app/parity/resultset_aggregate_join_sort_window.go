package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetAggregateJoinSortWindowJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateJoinSortWindowJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAggregateGrouped.java",
}

var (
	resultsetAggregateJoinSortWindowJavaRuntimeIDs = []string{
		"java-runtime-412ca9b80556afdf36e8",
	}
	resultsetAggregateJoinSortWindowJavaExecutions = []string{
		"ResultSetJoinSortWindow",
	}
)

// runResultSetAggregateJoinSortWindowScenario replays the sort(1, volume)
// join with irstream output every 1 seconds, matching ResultSetJoinSortWindow.
func runResultSetAggregateJoinSortWindowScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "join-sort-window"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-join-sort-window scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[unidirectionalSupportBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	symbol := esper.Field[resultsetGroupedTimeWindowMarket, string]("symbol")
	volume := esper.Field[resultsetGroupedTimeWindowMarket, int64]("volume")
	theString := esper.Field[unidirectionalSupportBean, string]("theString")
	query := esper.Join(
		esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
			Window(esper.SortWindow(1, esper.Ascending(volume))),
		esper.From[unidirectionalSupportBean](env, "SupportBean").
			Window(esper.KeepAll()),
		esper.OnEqual(symbol, theString),
	).GroupBy(esper.JoinField[string](0, "symbol")).
		Select(
			esper.Alias("symbol", esper.JoinField[string](0, "symbol")),
			esper.Alias("volume", esper.JoinField[int64](0, "volume")),
			esper.Alias("maxVol", esper.Max[float64](esper.JoinField[float64](0, "price"))),
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

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateJoinPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-join-sort-window statement %q", name)
		}
		return statement, nil
	})
}
