package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type outputAfterBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const outputAfterJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var outputAfterJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAfter.java",
}

var (
	outputAfterJavaRuntimeIDs = []string{
		"java-runtime-523f306d58fe926463bf",
	}
	outputAfterJavaExecutions = []string{
		"ResultSetAfterWithOutputLast",
	}
)

func runOutputAfterScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "after-last"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("output after scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[outputAfterBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	query := esper.From[outputAfterBean](env, "SupportBean").Aggregate(
		esper.Alias("thesum", esper.Sum[int](esper.Field[outputAfterBean, int]("intPrimitive"))),
	).Query(
		esper.StatementName("s0"),
		esper.WithOutput(esper.OutputAfterEvents(4, esper.OutputLastEveryEvents(2))),
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

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeOutputAfterPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown output after statement %q", name)
		}
		return statement, nil
	})
}

func decodeOutputAfterPayload(step compat.Step) (any, error) {
	var value outputAfterBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
