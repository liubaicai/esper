package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetJoinStringBean struct {
	TheString string `esper:"theString"`
}

const resultsetAggregateJoinEventsJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateJoinEventsJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAggregateGrouped.java",
}

var (
	resultsetAggregateJoinEventsJavaRuntimeIDs = []string{
		"java-runtime-a67ffb3a47f7812b19d3",
		"java-runtime-a54e39addd5960e2ccb4",
		"java-runtime-f0c54da1159588825b5d",
	}
	resultsetAggregateJoinEventsJavaExecutions = []string{
		"ResultSetJoinDefault",
		"ResultSetJoinAll",
		"ResultSetJoinLast",
	}
)

// runResultSetAggregateJoinEventsScenario replays the SupportBeanString-seeded
// length(5) join with output every/all/last every 2 events, matching
// ResultSetJoinDefault, ResultSetJoinAll and ResultSetJoinLast.
func runResultSetAggregateJoinEventsScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"default-join2", "all-join2", "last-join2"}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-join-events scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runResultSetAggregateJoinEventsCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-join-events case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateJoinEventsCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetJoinStringBean](env, "SupportBeanString"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	theString := esper.Field[resultsetJoinStringBean, string]("theString")
	symbol := esper.Field[resultsetGroupedTimeWindowMarket, string]("symbol")
	grouped := esper.Join(
		esper.From[resultsetJoinStringBean](env, "SupportBeanString").
			Window(esper.KeepAll()),
		esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
			Window(esper.LengthWindow(5)),
		esper.OnEqual(theString, symbol),
	).GroupBy(esper.JoinField[string](1, "symbol"))
	selects := []esper.Selection{
		esper.Alias("symbol", esper.JoinField[string](1, "symbol")),
		esper.Alias("volume", esper.JoinField[int64](1, "volume")),
		esper.Alias("mySum", esper.Sum[float64](esper.JoinField[float64](1, "price"))),
	}
	var query esper.Query
	switch caseName {
	case "default-join2":
		query = grouped.Select(selects...).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputEvery(2)),
		)
	case "all-join2":
		query = grouped.Select(selects...).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(2)),
		)
	case "last-join2":
		query = grouped.Select(selects...).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputLastEveryEvents(2)),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset-aggregate-join-events case %q", caseName)
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

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateJoinEventsPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-join-events statement %q", name)
		}
		return statement, nil
	})
}

func decodeResultSetAggregateJoinEventsPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBeanString":
		var value resultsetJoinStringBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBeanString: %w", err)
		}
		return value, nil
	case "SupportMarketDataBean":
		var value resultsetGroupedTimeWindowMarket
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported resultset-aggregate-join-events event type %q", step.EventType)
	}
}
