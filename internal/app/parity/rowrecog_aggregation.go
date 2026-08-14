package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type rowRecogAggregationBean struct {
	TheString string `esper:"theString"`
	Cat       string `esper:"cat"`
	Value     int    `esper:"value"`
}

const (
	rowRecogAggregationCaseUnpartitioned = "unpartitioned"
	rowRecogAggregationCasePartitioned   = "partitioned"
)

const rowRecogAggregationJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var rowRecogAggregationJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogAggregation.java",
}

var (
	rowRecogAggregationJavaRuntimeIDs = []string{
		"java-runtime-254b2ace6ea437689d15",
		"java-runtime-0de4e4485dc851b8c4a5",
	}
	rowRecogAggregationJavaExecutions = []string{
		"RowRecogMeasureAggregation",
		"RowRecogMeasureAggregationPartitioned",
	}
)

// runRowRecogAggregationScenario replays both measure-aggregation executions
// in their own runtime and concatenates listener/snapshot traces.
func runRowRecogAggregationScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{rowRecogAggregationCaseUnpartitioned, rowRecogAggregationCasePartitioned}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runRowRecogAggregationCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("rowrecog aggregation case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("rowrecog aggregation scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runRowRecogAggregationCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[rowRecogAggregationBean](env, "SupportRecogBean"); err != nil {
		return compat.Trace{}, err
	}
	stream := esper.From[rowRecogAggregationBean](env, "SupportRecogBean").Window(esper.KeepAll())
	value := esper.Field[rowRecogAggregationBean, int]("value")
	var query esper.Query
	switch caseName {
	case rowRecogAggregationCaseUnpartitioned:
		query = stream.MatchRecognize(esper.RowSequence(
			esper.RowVar("A"),
			esper.RowVar("B").ZeroOrMore(),
			esper.RowVar("C"),
		)).
			Define("A", esper.Equal[int](value, esper.Literal(0))).
			Define("B", esper.NotEqual[int](value, esper.Literal(1))).
			Define("C", esper.Equal[int](value, esper.Literal(1))).
			AllMatches().
			Measures(
				esper.Alias("a_string", esper.TagField[string]("A", "theString")),
				esper.Alias("c_string", esper.TagField[string]("C", "theString")),
				esper.Alias("maxb", esper.TagMax[int]("B", value)),
				esper.Alias("minb", esper.TagMin[int]("B", value)),
				esper.Alias("minb2x", esper.Multiply[int](esper.Literal(2), esper.TagMin[int]("B", value))),
				esper.Alias("lastb", esper.TagLast[int]("B", value)),
				esper.Alias("firstb", esper.TagFirst[int]("B", value)),
				esper.Alias("countb", esper.TagCount("B")),
			).
			Query(
				esper.StatementName("s0"),
				esper.OrderBy(esper.Ascending(esper.ResultField[string]("a_string"))),
			)
	case rowRecogAggregationCasePartitioned:
		cat := esper.Field[rowRecogAggregationBean, string]("cat")
		plusA := esper.Add[int](value, esper.TagField[int]("A", "value"))
		query = stream.MatchRecognize(esper.RowSequence(
			esper.RowVar("A"),
			esper.RowVar("B"),
			esper.RowVar("B"),
			esper.RowVar("C"),
			esper.RowVar("C"),
			esper.RowVar("D"),
		)).
			PartitionBy(cat).
			Define("A", esper.GreaterOrEqual[int](value, esper.Literal(10))).
			Define("B", esper.Greater[int](value, esper.Literal(1))).
			Define("C", esper.Less[int](value, esper.Literal(-1))).
			Define("D", esper.Equal[int](value, esper.Literal(999))).
			Measures(
				esper.Alias("cat", esper.TagField[string]("A", "cat")),
				esper.Alias("a_string", esper.TagField[string]("A", "theString")),
				esper.Alias("d_string", esper.TagField[string]("D", "theString")),
				esper.Alias("sumb", esper.TagSum[int]("B", value)),
				esper.Alias("sumc", esper.TagSum[int]("C", value)),
				esper.Alias("sumaplusb", esper.TagSum[int]("B", plusA)),
				esper.Alias("sumaplusc", esper.TagSum[int]("C", plusA)),
			).
			Query(
				esper.StatementName("s0"),
				esper.OrderBy(esper.Ascending(esper.ResultField[string]("cat"))),
			)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported rowrecog aggregation case %q", caseName)
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

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeRowRecogAggregationPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown rowrecog aggregation statement %q", name)
		}
		return statement, nil
	})
}

func decodeRowRecogAggregationPayload(step compat.Step) (any, error) {
	var value rowRecogAggregationBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportRecogBean: %w", err)
	}
	return value, nil
}
