package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedViewBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const contextKeySegmentedViewJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedViewJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedViewJavaRuntimeIDs = []string{
	"java-runtime-0b8a6173b8c0956d1c30", // ContextKeySegmentedViewSceneOne
	"java-runtime-2a987a059876811fe2a7", // ContextKeySegmentedViewSceneTwo
}

var contextKeySegmentedViewJavaExecutions = []string{
	"ContextKeySegmentedViewSceneOne",
	"ContextKeySegmentedViewSceneTwo",
}

// runContextKeySegmentedViewScenario replays the per-partition view
// executions of ContextKeySegmented: a keyed context partitioning by
// theString with a per-partition length(2) window projecting prevwindow
// (newest-to-oldest) under irstream, and a per-partition lastevent window
// producing new/old replacement pairs. The evicted row's prevwindow is null.
func runContextKeySegmentedViewScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedViewCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-view case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedViewCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedViewBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextKeySegmentedViewBean](env, "SupportBean")
	theString := esper.Field[contextKeySegmentedViewBean, string]("theString")
	intPrimitive := esper.Field[contextKeySegmentedViewBean, int]("intPrimitive")
	if _, err := esper.CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		return compat.Trace{}, err
	}

	var plan esper.Plan
	switch caseName {
	case "view-scene-one":
		query := esper.Select(beanSource.Window(esper.LengthWindow(2)),
			esper.Alias("intPrimitive", intPrimitive),
			esper.Alias("pw", esper.PrevWindow[esper.Event](esper.EventValue[esper.Event]())),
		).Query(esper.StatementName("s0"), esper.WithContext("SegmentedByString"), esper.WithOldStream())
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "view-scene-two":
		query := esper.Select(beanSource.Window(esper.LastEvent()),
			esper.Alias("theString", theString),
			esper.Alias("intPrimitive", intPrimitive),
		).Query(esper.StatementName("S1"), esper.WithContext("SegmentedByString"), esper.WithOldStream())
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported context-key-segmented-view case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedViewPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-view statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedViewPayload(step compat.Step) (any, error) {
	var value contextKeySegmentedViewBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
