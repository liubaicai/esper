package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedWInitTermPatternAsNameBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     int    `esper:"intBoxed"`
}

const contextKeySegmentedWInitTermPatternAsNameJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedWInitTermPatternAsNameJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedWInitTermPatternAsNameJavaRuntimeIDs = []string{
	"java-runtime-589238c9bf774001323b", // ContextKeySegmentedWInitTermPatternAsName
}

var contextKeySegmentedWInitTermPatternAsNameJavaExecutions = []string{
	"ContextKeySegmentedWInitTermPatternAsName",
}

// runContextKeySegmentedWInitTermPatternAsNameScenario replays the
// pattern-terminated keyed execution of ContextKeySegmented: a keyed
// context partitioned by theString, initiated by intPrimitive=1 as
// startevent and terminated by the single-tag pattern
// s=SupportBean(intPrimitive=2) as endpattern. The statement projects the
// initiating event's intBoxed through context.startevent and the pattern
// match's intBoxed through context.endpattern.s with a firstevent window
// and output snapshot when terminated.
func runContextKeySegmentedWInitTermPatternAsNameScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedWInitTermPatternAsNameCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-w-init-term-pattern-as-name case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedWInitTermPatternAsNameCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedWInitTermPatternAsNameBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextKeySegmentedWInitTermPatternAsNameBean](env, "SupportBean")
	theString := esper.Field[contextKeySegmentedWInitTermPatternAsNameBean, string]("theString")
	intPrimitive := esper.Field[contextKeySegmentedWInitTermPatternAsNameBean, int]("intPrimitive")
	end := esper.PatternFrom(beanSource, "s",
		esper.Equal[int](intPrimitive, esper.Literal(2)))
	if _, err := esper.CreatePatternTerminatedContext(env, "MyContext", theString,
		esper.Equal[int](intPrimitive, esper.Literal(1)), end); err != nil {
		return compat.Trace{}, err
	}
	query := esper.Select(beanSource.Window(esper.FirstEvent()),
		esper.Alias("c0", esper.Property[int](esper.ContextInitiatingEvent(), "intBoxed")),
		esper.Alias("c1", esper.ContextPatternField[int]("s", "intBoxed")),
	).Query(esper.StatementName("s0"), esper.WithContext("MyContext"), esper.WithOutput(esper.OutputSnapshotWhenTerminated()))
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedWInitTermPatternAsNamePayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-w-init-term-pattern-as-name statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedWInitTermPatternAsNamePayload(step compat.Step) (any, error) {
	var value contextKeySegmentedWInitTermPatternAsNameBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
