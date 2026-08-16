package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedMultikeyWArrayOfPrimitiveBean struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
	Value int    `esper:"value"`
}

const contextKeySegmentedMultikeyWArrayOfPrimitiveJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedMultikeyWArrayOfPrimitiveJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedMultikeyWArrayOfPrimitiveJavaRuntimeIDs = []string{
	"java-runtime-3e54e161e47d29e5c382", // ContextKeySegmentedMultikeyWArrayOfPrimitive
}

var contextKeySegmentedMultikeyWArrayOfPrimitiveJavaExecutions = []string{
	"ContextKeySegmentedMultikeyWArrayOfPrimitive",
}

// runContextKeySegmentedMultikeyWArrayOfPrimitiveScenario replays the
// array-keyed context execution of ContextKeySegmented: a keyed context
// partitioned by the int[] array property of SupportEventWithIntArray with
// a per-partition sum. Equal array content shares a partition, while an
// empty array and a null array are distinct keys (each with their own
// partition), matching Java's value-based array key semantics.
func runContextKeySegmentedMultikeyWArrayOfPrimitiveScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedMultikeyWArrayOfPrimitiveCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-multikey-w-array-of-primitive case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedMultikeyWArrayOfPrimitiveCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedMultikeyWArrayOfPrimitiveBean](env, "SupportEventWithIntArray"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextKeySegmentedMultikeyWArrayOfPrimitiveBean](env, "SupportEventWithIntArray")
	array := esper.Field[contextKeySegmentedMultikeyWArrayOfPrimitiveBean, []int]("array")
	value := esper.Field[contextKeySegmentedMultikeyWArrayOfPrimitiveBean, int]("value")
	if _, err := esper.CreateKeyContext(env, "PartitionByArray", array); err != nil {
		return compat.Trace{}, err
	}
	query := beanSource.Aggregate(
		esper.Alias("thesum", esper.Sum[int](value)),
	).Query(esper.StatementName("s0"), esper.WithContext("PartitionByArray"))
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedMultikeyWArrayOfPrimitivePayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-multikey-w-array-of-primitive statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedMultikeyWArrayOfPrimitivePayload(step compat.Step) (any, error) {
	var value contextKeySegmentedMultikeyWArrayOfPrimitiveBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportEventWithIntArray: %w", err)
	}
	return value, nil
}
