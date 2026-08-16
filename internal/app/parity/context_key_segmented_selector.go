package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedSelectorBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const contextKeySegmentedSelectorJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedSelectorJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedSelectorJavaRuntimeIDs = []string{
	"java-runtime-b571b059e04938cfeb02", // ContextKeySegmentedSelector
}

var contextKeySegmentedSelectorJavaExecutions = []string{
	"ContextKeySegmentedSelector",
}

// runContextKeySegmentedSelectorScenario replays the keyed selector
// execution of ContextKeySegmented: a keyed context partitioned by
// theString projecting context.key1 with a per-partition sum over a
// length(5) window. The context key property is visible in every listener
// row and in the cross-partition statement snapshot.
func runContextKeySegmentedSelectorScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedSelectorCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-selector case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedSelectorCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedSelectorBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextKeySegmentedSelectorBean](env, "SupportBean")
	theString := esper.Field[contextKeySegmentedSelectorBean, string]("theString")
	intPrimitive := esper.Field[contextKeySegmentedSelectorBean, int]("intPrimitive")

	if _, err := esper.CreateKeyContext(env, "PartitionedByString", theString); err != nil {
		return compat.Trace{}, err
	}
	query := beanSource.Window(esper.LengthWindow(5)).Aggregate(
		esper.Alias("c0", esper.ContextField[string]("key1")),
		esper.Alias("c1", esper.Sum[int](intPrimitive)),
	).Query(esper.StatementName("s0"), esper.WithContext("PartitionedByString"))
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedSelectorPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-selector statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedSelectorPayload(step compat.Step) (any, error) {
	var value contextKeySegmentedSelectorBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
