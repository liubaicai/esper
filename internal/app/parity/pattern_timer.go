package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type patternTimerBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const patternTimerJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var patternTimerJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternObserverTimerInterval.java",
}

var (
	patternTimerJavaRuntimeIDs = []string{
		"java-runtime-36d18111e663f599bc14",
	}
	patternTimerJavaExecutions = []string{
		"PatternIntervalSpecExpressionWithProperty",
	}
)

func runPatternTimerScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "interval-property"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("pattern timer scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternTimerBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	source := esper.From[patternTimerBean](env, "SupportBean")
	pattern := esper.PatternFrom(source, "a", esper.Literal(true)).Every().Then(
		esper.TimerIntervalExpr(source, esper.DurationSeconds[int](esper.TagField[int]("a", "intPrimitive"))),
	)
	plan, err := env.Build(pattern.Select(
		esper.Alias("id", esper.TagField[string]("a", "theString")),
	).Query(esper.StatementName("s0")))
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodePatternTimerPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown pattern timer statement %q", name)
		}
		return statement, nil
	})
}

func decodePatternTimerPayload(step compat.Step) (any, error) {
	var value patternTimerBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
