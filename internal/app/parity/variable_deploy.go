package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type variableDeployBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const variableDeployJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var variableDeployJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/variable/EPLVariablesOnSet.java",
}

var (
	variableDeployJavaRuntimeIDs = []string{
		"java-runtime-78f23a1b470f6bd625c4",
	}
	variableDeployJavaExecutions = []string{
		"EPLVariableOnSetWDeploy",
	}
)

func runVariableDeployScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "deploy-variable"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("variable deploy scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[variableDeployBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if err := env.RegisterVariable("var1RTC", 10); err != nil {
		return compat.Trace{}, err
	}
	source := esper.From[variableDeployBean](env, "SupportBean")
	selectPlan, err := env.Build(esper.Select(source.Filter(
		esper.StartsWith(esper.Field[variableDeployBean, string]("theString"), esper.Literal("E")),
	),
		esper.Alias("var1RTC", esper.VariableRef[int]("var1RTC")),
		esper.Alias("theString", esper.Field[variableDeployBean, string]("theString")),
	).Query(esper.StatementName("s0")))
	if err != nil {
		return compat.Trace{}, err
	}
	setPlan, err := env.Build(esper.OnEvent(source.Filter(
		esper.StartsWith(esper.Field[variableDeployBean, string]("theString"), esper.Literal("S")),
	)).SetVariable("var1RTC", esper.Field[variableDeployBean, int]("intPrimitive")).Query(esper.StatementName("set")))
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deploy := func(plan esper.Plan) (*esper.Statement, error) {
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return nil, err
		}
		if len(deployment.Statements()) != 1 {
			return nil, fmt.Errorf("expected one statement, got %d", len(deployment.Statements()))
		}
		return deployment.Statements()[0], nil
	}
	selectStatement, err := deploy(selectPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	setStatement, err := deploy(setPlan)
	if err != nil {
		return compat.Trace{}, err
	}

	return compat.ReplayWithStatements(ctx, engine, selectStatement, caseScenario, decodeVariableDeployPayload, func(name string) (*esper.Statement, error) {
		switch name {
		case selectStatement.Name():
			return selectStatement, nil
		case setStatement.Name():
			return setStatement, nil
		default:
			return nil, fmt.Errorf("unknown variable deploy statement %q", name)
		}
	}, setStatement)
}

func decodeVariableDeployPayload(step compat.Step) (any, error) {
	var value variableDeployBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
