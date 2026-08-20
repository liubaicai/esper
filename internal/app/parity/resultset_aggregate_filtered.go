package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetAggregateFilteredBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	IntBoxed      *int   `esper:"intBoxed"`
	BoolPrimitive bool   `esper:"boolPrimitive"`
}

const resultsetAggregateFilteredJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateFilteredJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFiltered.java",
}

var (
	resultsetAggregateFilteredJavaRuntimeIDs = []string{
		"java-runtime-7528fc808c92f7364d11",
		"java-runtime-658dc7c3a941e98e2f37",
	}
	resultsetAggregateFilteredJavaExecutions = []string{
		"ResultSetAggregateBlackWhitePercent",
		"ResultSetAggregateCountVariations",
	}
)

// runResultSetAggregateFilteredScenario replays the two observable SupportBean
// length-window executions from ResultSetAggregateFiltered. The Java source's
// remaining executions require separate capabilities (or invalid EPL), so they
// are intentionally outside this scenario.
func runResultSetAggregateFilteredScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"black-white-percent", "count-variations"}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetAggregateFilteredCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-filtered case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-filtered scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateFilteredCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateFilteredBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	bean := esper.From[resultsetAggregateFilteredBean](env, "SupportBean").Window(esper.LengthWindow(3))
	boolPrimitive := esper.Field[resultsetAggregateFilteredBean, bool]("boolPrimitive")
	var query esper.Query
	switch caseName {
	case "black-white-percent":
		trueCount := esper.CountIf(boolPrimitive)
		query = bean.Aggregate(
			esper.Alias("cb", trueCount),
			esper.Alias("cnb", esper.CountIf(esper.Not(boolPrimitive))),
			esper.Alias("c", esper.CountAll()),
			esper.Alias("pct", esper.Divide[float64](
				esper.Cast[int64, float64](trueCount),
				esper.Cast[int64, float64](esper.CountAll()),
			)),
		).Query(esper.StatementName("s0"))
	case "count-variations":
		intBoxed := esper.Field[resultsetAggregateFilteredBean, *int]("intBoxed")
		query = bean.Aggregate(
			esper.Alias("c1", esper.FilterAggregate[int64](esper.Count[*int](intBoxed), boolPrimitive)),
			esper.Alias("c2", esper.FilterAggregate[int64](esper.CountDistinct[*int](intBoxed), boolPrimitive)),
		).Query(esper.StatementName("s0"))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset-aggregate-filtered case %q", caseName)
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

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateFilteredPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-filtered statement %q", name)
		}
		return statement, nil
	})
}

func decodeResultSetAggregateFilteredPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("unsupported resultset-aggregate-filtered event type %q", step.EventType)
	}
	var value resultsetAggregateFilteredBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
