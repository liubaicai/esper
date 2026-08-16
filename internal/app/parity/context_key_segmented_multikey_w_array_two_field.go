package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedMultikeyWArrayTwoFieldBean struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
	Value int    `esper:"value"`
}

const contextKeySegmentedMultikeyWArrayTwoFieldJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedMultikeyWArrayTwoFieldJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedMultikeyWArrayTwoFieldJavaRuntimeIDs = []string{
	"java-runtime-c6bd2f3b2b6f99ff6b3e", // ContextKeySegmentedMultikeyWArrayTwoField
}

var contextKeySegmentedMultikeyWArrayTwoFieldJavaExecutions = []string{
	"ContextKeySegmentedMultikeyWArrayOfPrimitive",
}

// runContextKeySegmentedMultikeyWArrayTwoFieldScenario replays the
// two-field array-keyed context execution of ContextKeySegmented: a keyed
// context partitioned by the id string and the int[] array property of
// SupportEventWithIntArray with a per-partition sum. Both key fields
// participate: G1 and G2 with equal arrays are separate partitions, and
// G1/[1,2] and G1/[1] accumulate independently.
func runContextKeySegmentedMultikeyWArrayTwoFieldScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedMultikeyWArrayTwoFieldCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-multikey-w-array-two-field case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedMultikeyWArrayTwoFieldCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedMultikeyWArrayTwoFieldBean](env, "SupportEventWithIntArray"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextKeySegmentedMultikeyWArrayTwoFieldBean](env, "SupportEventWithIntArray")
	id := esper.Field[contextKeySegmentedMultikeyWArrayTwoFieldBean, string]("id")
	array := esper.Field[contextKeySegmentedMultikeyWArrayTwoFieldBean, []int]("array")
	value := esper.Field[contextKeySegmentedMultikeyWArrayTwoFieldBean, int]("value")
	if _, err := esper.CreateKeyContext(env, "PartitionByArray", id, array); err != nil {
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedMultikeyWArrayTwoFieldPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-multikey-w-array-two-field statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedMultikeyWArrayTwoFieldPayload(step compat.Step) (any, error) {
	var value contextKeySegmentedMultikeyWArrayTwoFieldBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportEventWithIntArray: %w", err)
	}
	return value, nil
}
