package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetAggregateJoinJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateJoinJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAggregateGrouped.java",
}

var (
	resultsetAggregateJoinJavaRuntimeIDs = []string{
		"java-runtime-00013efc7c322d04c3b3",
		"java-runtime-ecfb75f6916643de1c5f",
		"java-runtime-80532da2ea5ec539be06",
		"java-runtime-d2978828701be89efbdb",
		"java-runtime-b3cb0ef477369111027c",
		"java-runtime-e0866a0c063a6db46942",
		"java-runtime-9a902334e2f0f270f8f5",
	}
	resultsetAggregateJoinJavaExecutions = []string{
		"ResultSet2NoneNoHavingJoin",
		"ResultSet4NoneHavingJoin",
		"ResultSet6DefaultNoHavingJoin",
		"ResultSet8DefaultHavingJoin",
		"ResultSet14LastNoHavingJoin",
		"ResultSet16LastHavingJoin",
		"ResultSet17FirstNoHavingJoin",
	}
)

// runResultSetAggregateJoinScenario replays the grouped time-window join with
// SupportBean#keepall seeds and the 200-7200ms market sequence, covering the
// no-output, having, output-every and output-every-having join variants.
func runResultSetAggregateJoinScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"none-join", "none-having-join", "default-join", "default-having-join", "last-join", "last-having-join", "first-join"}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-join scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runResultSetAggregateJoinCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-join case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateJoinCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
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
	theString := esper.Field[unidirectionalSupportBean, string]("theString")
	join := esper.Join(
		esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
			Window(esper.TimeWindow(5500*time.Millisecond)),
		esper.From[unidirectionalSupportBean](env, "SupportBean").
			Window(esper.KeepAll()),
		esper.OnEqual(symbol, theString),
	)
	sum := esper.Sum[float64](esper.JoinField[float64](0, "price"))
	grouped := join.GroupBy(esper.JoinField[string](0, "symbol"))
	selects := []esper.Selection{
		esper.Alias("symbol", esper.JoinField[string](0, "symbol")),
		esper.Alias("volume", esper.JoinField[int64](0, "volume")),
		esper.Alias("sum(price)", sum),
	}
	options := []esper.QueryOption{esper.StatementName("s0"), esper.WithOldStream()}
	var query esper.Query
	switch caseName {
	case "none-join":
		query = grouped.Select(selects...).Query(options...)
	case "none-having-join":
		query = grouped.Having(esper.Greater[float64](sum, esper.Literal(50.0))).
			Select(selects...).Query(options...)
	case "default-join":
		options = append(options, esper.WithOutput(esper.OutputEveryTime(time.Second)))
		query = grouped.Select(selects...).Query(options...)
	case "default-having-join":
		options = append(options, esper.WithOutput(esper.OutputEveryTime(time.Second)))
		query = grouped.Having(esper.Greater[float64](sum, esper.Literal(50.0))).
			Select(selects...).Query(options...)
	case "last-join":
		options = append(options,
			esper.WithOutput(esper.OutputLastEveryTime(time.Second)),
			esper.OrderBy(esper.Ascending(esper.ResultField[string]("symbol"))),
		)
		query = grouped.Select(selects...).Query(options...)
	case "last-having-join":
		options = append(options, esper.WithOutput(esper.OutputLastEveryTime(time.Second)))
		query = grouped.Having(esper.Greater[float64](sum, esper.Literal(50.0))).
			Select(selects...).Query(options...)
	case "first-join":
		options = append(options, esper.WithOutput(esper.OutputFirstEveryTime(time.Second)))
		query = grouped.Select(selects...).Query(options...)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset-aggregate-join case %q", caseName)
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

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateJoinPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-join statement %q", name)
		}
		return statement, nil
	})
}

func decodeResultSetAggregateJoinPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportMarketDataBean":
		var value resultsetGroupedTimeWindowMarket
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return value, nil
	case "SupportBean":
		var value unidirectionalSupportBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported resultset-aggregate-join event type %q", step.EventType)
	}
}
