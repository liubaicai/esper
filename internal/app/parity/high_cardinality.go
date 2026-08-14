package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type highCardinalityBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const highCardinalityJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var highCardinalityJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var (
	highCardinalityJavaRuntimeIDs = []string{
		"java-runtime-bd4672ec90862ee08790",
	}
	highCardinalityJavaExecutions = []string{
		"ContextKeySegmentedLargeNumberPartitions",
	}
)

func runHighCardinalityScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "high-cardinality"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("high cardinality scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[highCardinalityBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateKeyContext(env, "SegmentedByAString",
		esper.Field[highCardinalityBean, string]("theString")); err != nil {
		return compat.Trace{}, err
	}
	query := esper.From[highCardinalityBean](env, "SupportBean").Aggregate(
		esper.Alias("col1", esper.Sum[int](esper.Field[highCardinalityBean, int]("intPrimitive"))),
	).Query(
		esper.StatementName("s0"),
		esper.WithContext("SegmentedByAString"),
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

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeHighCardinalityPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown high cardinality statement %q", name)
		}
		return statement, nil
	})
}

func decodeHighCardinalityPayload(step compat.Step) (any, error) {
	var value highCardinalityBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
