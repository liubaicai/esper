package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedNullKeysBean struct {
	TheString    *string `esper:"theString"`
	IntBoxed     *int    `esper:"intBoxed"`
	IntPrimitive int     `esper:"intPrimitive"`
}

const contextKeySegmentedNullKeysJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedNullKeysJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedNullKeysJavaRuntimeIDs = []string{
	"java-runtime-66e2c18a214c52bc1506", // ContextKeySegmentedNullSingleKey
	"java-runtime-4449ff62ddd405a75c89", // ContextKeySegmentedNullKeyMultiKey
}

var contextKeySegmentedNullKeysJavaExecutions = []string{
	"ContextKeySegmentedNullSingleKey",
	"ContextKeySegmentedNullKeyMultiKey",
}

// runContextKeySegmentedNullKeysScenario replays the null-key executions of
// ContextKeySegmented: a keyed context partitioned by theString (nullable)
// and a three-field key (theString, intBoxed, intPrimitive) where intBoxed
// is nullable. Null key values share one partition across events, while a
// present value starts a separate partition.
func runContextKeySegmentedNullKeysScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedNullKeysCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-null-keys case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedNullKeysCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedNullKeysBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextKeySegmentedNullKeysBean](env, "SupportBean")
	theString := esper.Field[contextKeySegmentedNullKeysBean, *string]("theString")
	var plan esper.Plan
	switch caseName {
	case "null-single-key":
		if _, err := esper.CreateKeyContext(env, "MyContext", theString); err != nil {
			return compat.Trace{}, err
		}
		plan, err = env.Build(beanSource.Aggregate(
			esper.Alias("cnt", esper.CountAll()),
		).Query(esper.StatementName("s0"), esper.WithContext("MyContext")))
		if err != nil {
			return compat.Trace{}, err
		}
	case "null-key-multi-key":
		intBoxed := esper.Field[contextKeySegmentedNullKeysBean, *int]("intBoxed")
		intPrimitive := esper.Field[contextKeySegmentedNullKeysBean, int]("intPrimitive")
		if _, err := esper.CreateKeyContext(env, "MyContext", theString, intBoxed, intPrimitive); err != nil {
			return compat.Trace{}, err
		}
		plan, err = env.Build(beanSource.Aggregate(
			esper.Alias("cnt", esper.CountAll()),
		).Query(esper.StatementName("s0"), esper.WithContext("MyContext")))
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported context-key-segmented-null-keys case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedNullKeysPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-null-keys statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedNullKeysPayload(step compat.Step) (any, error) {
	var value contextKeySegmentedNullKeysBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
