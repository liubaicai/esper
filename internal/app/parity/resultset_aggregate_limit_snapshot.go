package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetAggregateLimitSnapshotJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateLimitSnapshotJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAggregateGrouped.java",
}

var (
	resultsetAggregateLimitSnapshotJavaRuntimeIDs = []string{
		"java-runtime-caa47dd0c613a0fc0ff4",
		"java-runtime-ba55e3db86bd1b3007eb",
	}
	resultsetAggregateLimitSnapshotJavaExecutions = []string{
		"ResultSetLimitSnapshot",
		"ResultSetLimitSnapshotJoin",
	}
)

// runResultSetAggregateLimitSnapshotScenario replays the time(10 seconds)
// grouped sum with output snapshot every 1 seconds, plain and join variants,
// matching ResultSetLimitSnapshot and ResultSetLimitSnapshotJoin.
func runResultSetAggregateLimitSnapshotScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"limit-snapshot", "limit-snapshot-join"}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-limit-snapshot scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runResultSetAggregateLimitSnapshotCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-limit-snapshot case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateLimitSnapshotCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetGroupOutputBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	symbol := esper.Field[resultsetGroupedTimeWindowMarket, string]("symbol")
	volume := esper.Field[resultsetGroupedTimeWindowMarket, int64]("volume")
	price := esper.Field[resultsetGroupedTimeWindowMarket, float64]("price")
	var query esper.Query
	switch caseName {
	case "limit-snapshot":
		query = esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
			Window(esper.TimeWindow(10*time.Second)).
			GroupBy(symbol).
			Select(
				esper.Alias("symbol", symbol),
				esper.Alias("volume", volume),
				esper.Alias("sumprice", esper.Sum[float64](price)),
			).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputSnapshotEvery(time.Second)),
		)
	case "limit-snapshot-join":
		theString := esper.Field[resultsetGroupOutputBean, string]("theString")
		query = esper.Join(
			esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
				Window(esper.TimeWindow(10*time.Second)),
			esper.From[resultsetGroupOutputBean](env, "SupportBean").
				Window(esper.KeepAll()),
			esper.OnEqual(symbol, theString),
		).GroupBy(esper.JoinField[string](0, "symbol")).
			Select(
				esper.Alias("symbol", esper.JoinField[string](0, "symbol")),
				esper.Alias("volume", esper.JoinField[int64](0, "volume")),
				esper.Alias("sumprice", esper.Sum[float64](esper.JoinField[float64](0, "price"))),
			).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputSnapshotEvery(time.Second)),
			esper.OrderBy(
				esper.Ascending(esper.ResultField[string]("symbol")),
				esper.Ascending(esper.ResultField[int64]("volume")),
			),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset-aggregate-limit-snapshot case %q", caseName)
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

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateLimitSnapshotPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-limit-snapshot statement %q", name)
		}
		return statement, nil
	})
}

func decodeResultSetAggregateLimitSnapshotPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportMarketDataBean":
		var value resultsetGroupedTimeWindowMarket
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return value, nil
	case "SupportBean":
		var value resultsetGroupOutputBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported resultset-aggregate-limit-snapshot event type %q", step.EventType)
	}
}
