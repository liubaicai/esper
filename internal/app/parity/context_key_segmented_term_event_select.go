package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedTermEventSelectBean struct {
	UserID *string `esper:"userId"`
	Alert  *string `esper:"alert"`
}

const contextKeySegmentedTermEventSelectJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedTermEventSelectJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedTermEventSelectJavaRuntimeIDs = []string{
	"java-runtime-0fe0eaefe8ed7adcc2bf", // ContextKeySegmentedTermEventSelect
}

var contextKeySegmentedTermEventSelectJavaExecutions = []string{
	"ContextKeySegmentedTermEventSelect",
}

// runContextKeySegmentedTermEventSelectScenario replays the term-event
// projection execution of ContextKeySegmented: a keyed context partitioned
// by userId, initiated by alert='A' and terminated by alert='B' as
// termEvent. The statement projects every UserEvent field plus
// context.termEvent from a firstevent window with output snapshot when
// terminated: the snapshot holds the initiating event (the terminating
// event closes the partition without entering the window) and the term
// column carries the terminating event.
func runContextKeySegmentedTermEventSelectScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedTermEventSelectCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-term-event-select case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedTermEventSelectCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedTermEventSelectBean](env, "UserEvent"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextKeySegmentedTermEventSelectBean](env, "UserEvent")
	userID := esper.Field[contextKeySegmentedTermEventSelectBean, *string]("userId")
	alert := esper.Field[contextKeySegmentedTermEventSelectBean, *string]("alert")
	isA := esper.Equal[*string](alert, esper.Literal("A"))
	isB := esper.Equal[*string](alert, esper.Literal("B"))
	if _, err := esper.CreateInitiatedTerminatedContext(env, "UserSessionContext", userID, isA, isB); err != nil {
		return compat.Trace{}, err
	}
	query := esper.Select(beanSource.Window(esper.FirstEvent()),
		esper.Alias("userId", userID),
		esper.Alias("alert", alert),
		esper.Alias("term", esper.ContextTerminatingEvent()),
	).Query(esper.StatementName("s0"), esper.WithContext("UserSessionContext"), esper.WithOutput(esper.OutputSnapshotWhenTerminated()))
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedTermEventSelectPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-term-event-select statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedTermEventSelectPayload(step compat.Step) (any, error) {
	var value contextKeySegmentedTermEventSelectBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode UserEvent: %w", err)
	}
	return value, nil
}
