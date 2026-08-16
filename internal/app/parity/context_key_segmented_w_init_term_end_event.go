package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedWInitTermEndEventBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const contextKeySegmentedWInitTermEndEventJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedWInitTermEndEventJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedWInitTermEndEventJavaRuntimeIDs = []string{
	"java-runtime-e5577cf37b78380920d3", // ContextKeySegmentedWInitTermEndEvent
}

var contextKeySegmentedWInitTermEndEventJavaExecutions = []string{
	"ContextKeySegmentedWInitTermEndEvent",
}

// runContextKeySegmentedWInitTermEndEventScenario replays the
// initiated-terminated keyed execution of ContextKeySegmented: a keyed
// context partitioned by theString, initiated by intPrimitive=1 as
// startevent and terminated by intPrimitive=0 as endevent. The statement
// projects context.startevent as c0 and context.endevent as c1 with
// output all when terminated, emitting one row per partition at
// termination carrying both boundary events.
func runContextKeySegmentedWInitTermEndEventScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedWInitTermEndEventCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-w-init-term-end-event case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedWInitTermEndEventCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedWInitTermEndEventBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextKeySegmentedWInitTermEndEventBean](env, "SupportBean")
	theString := esper.Field[contextKeySegmentedWInitTermEndEventBean, string]("theString")
	intPrimitive := esper.Field[contextKeySegmentedWInitTermEndEventBean, int]("intPrimitive")
	if _, err := esper.CreateInitiatedTerminatedContext(env, "MyContext", theString,
		esper.Equal[int](intPrimitive, esper.Literal(1)),
		esper.Equal[int](intPrimitive, esper.Literal(0))); err != nil {
		return compat.Trace{}, err
	}
	query := esper.Select(beanSource,
		esper.Alias("c0", esper.ContextInitiatingEvent()),
		esper.Alias("c1", esper.ContextTerminatingEvent()),
	).Query(esper.StatementName("s0"), esper.WithContext("MyContext"), esper.WithOutput(esper.OutputWhenTerminated(esper.OutputAll())))
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedWInitTermEndEventPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-w-init-term-end-event statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedWInitTermEndEventPayload(step compat.Step) (any, error) {
	var value contextKeySegmentedWInitTermEndEventBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
