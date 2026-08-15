package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetAggregateAllHavingJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateAllHavingJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAggregateGrouped.java",
}

var (
	resultsetAggregateAllHavingJavaRuntimeIDs = []string{
		"java-runtime-72011b9d57605c76f1b0",
		"java-runtime-598cef9d8eebd4dfd1a2",
	}
	resultsetAggregateAllHavingJavaExecutions = []string{
		"ResultSet11AllHavingNoJoin",
		"ResultSet12AllHavingJoin",
	}
)

// runResultSetAggregateAllHavingScenario replays the grouped time-window sum
// with having sum(price) > 50 and output all every 1 seconds, covering both
// the view and the SupportBean#keepall join forms, matching ResultSet11
// AllHavingNoJoin and ResultSet12AllHavingJoin.
func runResultSetAggregateAllHavingScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"all-having", "all-having-join"}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-all-having scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runResultSetAggregateAllHavingCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-all-having case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateAllHavingCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
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
	selects := []esper.Selection{
		esper.Alias("symbol", symbol),
		esper.Alias("volume", volume),
		esper.Alias("sum(price)", sum),
	}
	options := []esper.QueryOption{esper.StatementName("s0"), esper.WithOldStream()}
	var query esper.Query
	if caseName == "all-having-join" {
		if _, err := esper.RegisterStruct[unidirectionalSupportBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		theString := esper.Field[unidirectionalSupportBean, string]("theString")
		grouped := esper.Join(
			esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
				Window(esper.TimeWindow(5500*time.Millisecond)),
			esper.From[unidirectionalSupportBean](env, "SupportBean").
				Window(esper.KeepAll()),
			esper.OnEqual(symbol, theString),
		).GroupBy(esper.JoinField[string](0, "symbol"))
		joinSum := esper.Sum[float64](esper.JoinField[float64](0, "price"))
		selects = []esper.Selection{
			esper.Alias("symbol", esper.JoinField[string](0, "symbol")),
			esper.Alias("volume", esper.JoinField[int64](0, "volume")),
			esper.Alias("sum(price)", joinSum),
		}
		query = grouped.Having(esper.Greater[float64](joinSum, esper.Literal(50.0))).
			Select(selects...).Query(append(options, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))...)
	} else {
		grouped := esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
			Window(esper.TimeWindow(5500 * time.Millisecond)).
			GroupBy(symbol)
		query = grouped.Having(esper.Greater[float64](sum, esper.Literal(50.0))).
			Select(selects...).Query(append(options, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))...)
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

	decode := func(step compat.Step) (any, error) {
		if caseName == "all-having-join" {
			return decodeResultSetAggregateJoinPayload(step)
		}
		return decodeResultSetGroupedTimeWindowPayload(step)
	}
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decode, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-all-having statement %q", name)
		}
		return statement, nil
	})
}
