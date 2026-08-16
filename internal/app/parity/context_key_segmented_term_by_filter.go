package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedTermByFilterBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const contextKeySegmentedTermByFilterJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedTermByFilterJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedTermByFilterJavaRuntimeIDs = []string{
	"java-runtime-820bb6f72b84ad070ce4", // ContextKeySegmentedTermByFilter
}

var contextKeySegmentedTermByFilterJavaExecutions = []string{
	"ContextKeySegmentedTermByFilter",
}

// runContextKeySegmentedTermByFilterScenario replays the filter-terminated
// keyed execution of ContextKeySegmented: a keyed context partitioned by
// theString where an intPrimitive<0 event terminates the partition. The
// statement counts non-negative events per partition; a terminated
// partition restarts at 1 on the next non-negative event for the same key,
// and a negative event for an absent key produces no output.
func runContextKeySegmentedTermByFilterScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedTermByFilterCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-term-by-filter case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedTermByFilterCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedTermByFilterBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextKeySegmentedTermByFilterBean](env, "SupportBean")
	theString := esper.Field[contextKeySegmentedTermByFilterBean, string]("theString")
	intPrimitive := esper.Field[contextKeySegmentedTermByFilterBean, int]("intPrimitive")
	isBean := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean"))
	if _, err := esper.CreateInitiatedTerminatedContext(env, "ByP0", theString,
		isBean,
		esper.LessOf(intPrimitive, esper.Literal(0))); err != nil {
		return compat.Trace{}, err
	}
	query := beanSource.Filter(esper.GreaterOrEqual[int](intPrimitive, esper.Literal(0))).Aggregate(
		esper.Alias("theString", theString),
		esper.Alias("cnt", esper.CountAll()),
	).Query(esper.StatementName("s0"), esper.WithContext("ByP0"))
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedTermByFilterPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-term-by-filter statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedTermByFilterPayload(step compat.Step) (any, error) {
	var value contextKeySegmentedTermByFilterBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
