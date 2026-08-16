package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedPatternBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const contextKeySegmentedPatternJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedPatternJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedPatternJavaRuntimeIDs = []string{
	"java-runtime-854addf97e729f7ee54d", // ContextKeySegmentedPattern
}

var contextKeySegmentedPatternJavaExecutions = []string{
	"ContextKeySegmentedPattern",
}

// runContextKeySegmentedPatternScenario replays the correlated pattern
// execution of ContextKeySegmented (first statement shape, without the
// @Consume variants): a keyed context partitioned by theString running
// `every a=SupportBean -> b=SupportBean(intPrimitive=a.intPrimitive+1)`.
// The every binds to the left event expression, so every a-event spawns its
// own concurrent b-wait within its partition: G1's 10 waits for 11 and G2's
// 20 waits for 21 independently, and later matches chain (21->22, 22->23).
func runContextKeySegmentedPatternScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedPatternCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-pattern case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedPatternCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedPatternBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextKeySegmentedPatternBean](env, "SupportBean")
	theString := esper.Field[contextKeySegmentedPatternBean, string]("theString")
	if _, err := esper.CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		return compat.Trace{}, err
	}
	pattern := esper.PatternFrom(beanSource, "a", esper.Literal(true)).
		Every().
		FollowedBy("b", esper.Equal[int](
			esper.TagField[int]("b", "intPrimitive"),
			esper.Add[int](esper.TagField[int]("a", "intPrimitive"), esper.Literal(1))))
	query := pattern.Select(
		esper.Alias("a", esper.PatternEvent("a")),
		esper.Alias("b", esper.PatternEvent("b")),
	).Query(esper.StatementName("s0"), esper.WithContext("SegmentedByString"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env)
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		_ = engine.Close(context.Background())
		return compat.Trace{}, err
	}
	if len(deployment.Statements()) != 1 {
		_ = engine.Close(context.Background())
		return compat.Trace{}, fmt.Errorf("expected one statement, got %d", len(deployment.Statements()))
	}
	statement := deployment.Statements()[0]
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedPatternPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-pattern statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedPatternPayload(step compat.Step) (any, error) {
	var value contextKeySegmentedPatternBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
