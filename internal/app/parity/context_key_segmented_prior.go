package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedPriorBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const contextKeySegmentedPriorJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedPriorJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedPriorJavaRuntimeIDs = []string{
	"java-runtime-9853fdd35798941954e9", // ContextKeySegmentedPrior
}

var contextKeySegmentedPriorJavaExecutions = []string{
	"ContextKeySegmentedPattern",
}

// runContextKeySegmentedPriorScenario replays the prior-access execution
// of ContextKeySegmented: a keyed context partitioned by theString
// projecting `prior(1, intPrimitive)` (Go Prior offset 0 = Java prior(1)).
// Prior history is per partition: G1's events see only G1's previous event
// and G2's only G2's, so interleaved streams keep isolated histories.
func runContextKeySegmentedPriorScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedPriorCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-prior case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedPriorCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedPriorBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextKeySegmentedPriorBean](env, "SupportBean")
	theString := esper.Field[contextKeySegmentedPriorBean, string]("theString")
	if _, err := esper.CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		return compat.Trace{}, err
	}
	intPrimitive := esper.Field[contextKeySegmentedPriorBean, int]("intPrimitive")
	query := esper.Select(beanSource,
		esper.Alias("val0", intPrimitive),
		esper.Alias("val1", esper.Prior[int](0, intPrimitive)),
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedPriorPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-prior statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedPriorPayload(step compat.Step) (any, error) {
	var value contextKeySegmentedPriorBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
