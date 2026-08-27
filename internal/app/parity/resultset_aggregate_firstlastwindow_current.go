package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetAggregateFirstLastWindowCurrentBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const resultsetAggregateFirstLastWindowCurrentJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateFirstLastWindowCurrentJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFirstLastWindow.java",
}

var (
	// Inventory order is authoritative: NoGroup is ordinal 15 and Group is
	// ordinal 16 in ResultSetAggregateFirstLastWindow.executions().
	resultsetAggregateFirstLastWindowCurrentJavaRuntimeIDs = []string{
		"java-runtime-7e5e6b41dd7cb8e65879", // ResultSetAggregateFirstLastWindowNoGroup
		"java-runtime-e48086834b488cb35e3c", // ResultSetAggregateFirstLastWindowGroup
	}
	resultsetAggregateFirstLastWindowCurrentJavaExecutions = []string{
		"ResultSetAggregateFirstLastWindowNoGroup",
		"ResultSetAggregateFirstLastWindowGroup",
	}
)

const (
	resultsetAggregateFirstLastWindowCurrentNoGroupCase = "no-group"
	resultsetAggregateFirstLastWindowCurrentGroupCase   = "group"
)

// runResultSetAggregateFirstLastWindowCurrentScenario replays the two
// current-window first/last/window executions from
// ResultSetAggregateFirstLastWindow. Each case receives a fresh environment
// and runtime, matching the isolated Java execution lifecycle.
func runResultSetAggregateFirstLastWindowCurrentScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		resultsetAggregateFirstLastWindowCurrentNoGroupCase,
		resultsetAggregateFirstLastWindowCurrentGroupCase,
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetAggregateFirstLastWindowCurrentCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-firstlastwindow-current case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-firstlastwindow-current scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateFirstLastWindowCurrentCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateFirstLastWindowCurrentBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	theString := esper.Field[resultsetAggregateFirstLastWindowCurrentBean, string]("theString")
	intPrimitive := esper.Field[resultsetAggregateFirstLastWindowCurrentBean, int]("intPrimitive")
	var query esper.Query
	switch caseName {
	case resultsetAggregateFirstLastWindowCurrentNoGroupCase:
		query = esper.From[resultsetAggregateFirstLastWindowCurrentBean](env, "SupportBean").
			Window(esper.LengthWindow(2)).
			Aggregate(
				esper.Alias("firststring", esper.First[string](theString)),
				esper.Alias("laststring", esper.Last[string](theString)),
				esper.Alias("firstint", esper.First[int](intPrimitive)),
				esper.Alias("lastint", esper.Last[int](intPrimitive)),
				esper.Alias("allint", esper.WindowValues[int](intPrimitive)),
			).
			Query(esper.StatementName("s0"))
	case resultsetAggregateFirstLastWindowCurrentGroupCase:
		query = esper.From[resultsetAggregateFirstLastWindowCurrentBean](env, "SupportBean").
			Window(esper.LengthWindow(5)).
			GroupBy(theString).
			Select(
				esper.Alias("theString", theString),
				esper.Alias("firststring", esper.First[string](theString)),
				esper.Alias("laststring", esper.Last[string](theString)),
				esper.Alias("firstint", esper.First[int](intPrimitive)),
				esper.Alias("lastint", esper.Last[int](intPrimitive)),
				esper.Alias("allint", esper.WindowValues[int](intPrimitive)),
			).
			Query(esper.StatementName("s0"), esper.OrderBy(esper.Ascending(theString)))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset-aggregate-firstlastwindow-current case %q", caseName)
	}

	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateFirstLastWindowCurrentRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one resultset aggregate statement, got %d", len(statements))
	}
	statement := statements[0]
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateFirstLastWindowCurrentPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-firstlastwindow-current statement %q", name)
		}
		return statement, nil
	})
}

func resultsetAggregateFirstLastWindowCurrentRuntimeURI(caseName string) string {
	switch caseName {
	case resultsetAggregateFirstLastWindowCurrentNoGroupCase:
		return resultsetAggregateFirstLastWindowCurrentJavaRuntimeIDs[0]
	case resultsetAggregateFirstLastWindowCurrentGroupCase:
		return resultsetAggregateFirstLastWindowCurrentJavaRuntimeIDs[1]
	default:
		return "parity-resultset-aggregate-firstlastwindow-current-" + caseName
	}
}

func decodeResultSetAggregateFirstLastWindowCurrentPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("unsupported resultset-aggregate-firstlastwindow-current event type %q", step.EventType)
	}
	var value resultsetAggregateFirstLastWindowCurrentBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
