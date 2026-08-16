package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedPatternSceneTwoBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type contextKeySegmentedPatternSceneTwoS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

const contextKeySegmentedPatternSceneTwoJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedPatternSceneTwoJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedPatternSceneTwoJavaRuntimeIDs = []string{
	"java-runtime-b85c452789ff77fea9a4", // ContextKeySegmentedPatternSceneTwo
}

var contextKeySegmentedPatternSceneTwoJavaExecutions = []string{
	"ContextKeySegmentedPatternSceneTwo",
}

// runContextKeySegmentedPatternSceneTwoScenario replays the two-stream
// pattern execution of ContextKeySegmented: a multi-stream segmented
// context partitioning SupportBean by theString and SupportBean_S0 by p00
// (shared key space), running `every a=SupportBean ->
// b=SupportBean_S0(id=a.intPrimitive)` per partition. Partner events route
// to the partition whose p00 equals the SB key, so each partition's a-wait
// completes only with its own matching S0 event.
func runContextKeySegmentedPatternSceneTwoScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedPatternSceneTwoCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-pattern-scene-two case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedPatternSceneTwoCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedPatternSceneTwoBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextKeySegmentedPatternSceneTwoS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	theString := esper.Field[contextKeySegmentedPatternSceneTwoBean, string]("theString")
	p00 := esper.Field[contextKeySegmentedPatternSceneTwoS0, string]("p00")
	if _, err := esper.CreateKeyContextByStreams(env, "SegmentedByString",
		esper.KeyContextStream{Type: "SupportBean", Keys: []esper.Expr{theString}},
		esper.KeyContextStream{Type: "SupportBean_S0", Keys: []esper.Expr{p00}},
	); err != nil {
		return compat.Trace{}, err
	}
	pattern := esper.PatternFrom(esper.From[contextKeySegmentedPatternSceneTwoBean](env, "SupportBean"), "a", esper.Literal(true)).
		Every().
		Then(esper.PatternFrom(esper.From[contextKeySegmentedPatternSceneTwoS0](env, "SupportBean_S0"), "b",
			esper.Equal[int](
				esper.TagField[int]("b", "id"),
				esper.TagField[int]("a", "intPrimitive"),
			)))
	query := pattern.Select(
		esper.Alias("c0", esper.TagField[string]("a", "theString")),
		esper.Alias("c1", esper.TagField[int]("a", "intPrimitive")),
		esper.Alias("c2", esper.TagField[int]("b", "id")),
		esper.Alias("c3", esper.TagField[string]("b", "p00")),
	).Query(esper.StatementName("S1"), esper.WithContext("SegmentedByString"))
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedPatternSceneTwoPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-pattern-scene-two statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedPatternSceneTwoPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextKeySegmentedPatternSceneTwoBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextKeySegmentedPatternSceneTwoS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unknown context-key-segmented-pattern-scene-two event type %q", step.EventType)
	}
}
