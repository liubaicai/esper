package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetRowPerGroupSimpleBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const resultsetRowPerGroupSimpleCase = "simple"

const resultsetRowPerGroupSimpleJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetRowPerGroupSimpleJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRowPerGroup.java",
}

var (
	resultsetRowPerGroupSimpleJavaRuntimeIDs = []string{
		"java-runtime-d7060e63eda2baaf87f5",
	}
	resultsetRowPerGroupSimpleJavaExecutions = []string{
		"ResultSetQueryTypeRowPerGroupSimple",
	}
)

func runResultSetRowPerGroupSimpleScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if !scenarioHasCase(scenario, resultsetRowPerGroupSimpleCase) {
		return compat.Trace{}, fmt.Errorf("resultset row per group simple scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, resultsetRowPerGroupSimpleCase)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetRowPerGroupSimpleBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	theString := esper.Field[resultsetRowPerGroupSimpleBean, string]("theString")
	intPrimitive := esper.Field[resultsetRowPerGroupSimpleBean, int]("intPrimitive")
	query := esper.From[resultsetRowPerGroupSimpleBean](env, "SupportBean").
		GroupBy(theString).
		Select(
			esper.Alias("c0", theString),
			esper.Alias("c1", esper.Sum[int](intPrimitive)),
			esper.Alias("c2", esper.Min[int](intPrimitive)),
			esper.Alias("c3", esper.Max[int](intPrimitive)),
		).
		Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetRowPerGroupSimplePayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset row per group simple statement %q", name)
		}
		return statement, nil
	})
}

func decodeResultSetRowPerGroupSimplePayload(step compat.Step) (any, error) {
	var value resultsetRowPerGroupSimpleBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
