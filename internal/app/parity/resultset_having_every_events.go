package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetHavingEveryEventsJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetHavingEveryEventsJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAggregateGrouped.java",
}

var (
	resultsetHavingEveryEventsJavaRuntimeIDs = []string{
		"java-runtime-f209dbfcc4be7dcfa536",
		"java-runtime-1b23cf3fcfceb0d7eb23",
	}
	resultsetHavingEveryEventsJavaExecutions = []string{
		"ResultSetHaving",
		"ResultSetHavingJoin",
	}
)

// runResultSetHavingEveryEventsScenario replays the time(10 sec) grouped sum
// with having sum(price) >= 10 and output every 3 events, including irstream
// expiry old rows, matching ResultSetHaving.
func runResultSetHavingEveryEventsScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"having-output", "having-join"}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("resultset-having-every-events scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runResultSetHavingEveryEventsCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-having-every-events case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetHavingEveryEventsCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
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
	var query esper.Query
	if caseName == "having-join" {
		if _, err := esper.RegisterStruct[unidirectionalSupportBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		theString := esper.Field[unidirectionalSupportBean, string]("theString")
		joinSum := esper.Sum[float64](esper.JoinField[float64](0, "price"))
		query = esper.Join(
			esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
				Window(esper.TimeWindow(10*time.Second)),
			esper.From[unidirectionalSupportBean](env, "SupportBean").
				Window(esper.KeepAll()),
			esper.OnEqual(symbol, theString),
		).GroupBy(esper.JoinField[string](0, "symbol")).
			Having(esper.GreaterOrEqual[float64](joinSum, esper.Literal(10.0))).
			Select(
				esper.Alias("symbol", esper.JoinField[string](0, "symbol")),
				esper.Alias("volume", esper.JoinField[int64](0, "volume")),
				esper.Alias("sumprice", joinSum),
			).Query(
			esper.StatementName("s0"),
			esper.WithOldStream(),
			esper.WithOutput(esper.OutputEvery(3)),
		)
	} else {
		query = esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
			Window(esper.TimeWindow(10*time.Second)).
			GroupBy(symbol).
			Having(esper.GreaterOrEqual[float64](sum, esper.Literal(10.0))).
			Select(
				esper.Alias("symbol", symbol),
				esper.Alias("volume", volume),
				esper.Alias("sumprice", sum),
			).Query(
			esper.StatementName("s0"),
			esper.WithOldStream(),
			esper.WithOutput(esper.OutputEvery(3)),
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

	decode := decodeResultSetGroupedTimeWindowPayload
	if caseName == "having-join" {
		decode = decodeResultSetAggregateJoinPayload
	}
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decode, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-having-every-events statement %q", name)
		}
		return statement, nil
	})
}
