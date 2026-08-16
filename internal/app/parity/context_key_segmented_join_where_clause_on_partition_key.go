package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedJoinWhereClauseOnPartitionKeyBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type contextKeySegmentedJoinWhereClauseOnPartitionKeyS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

const contextKeySegmentedJoinWhereClauseOnPartitionKeyJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedJoinWhereClauseOnPartitionKeyJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedJoinWhereClauseOnPartitionKeyJavaRuntimeIDs = []string{
	"java-runtime-10cb439d3d2aebe1c32b", // ContextKeySegmentedJoinWhereClauseOnPartitionKey
}

var contextKeySegmentedJoinWhereClauseOnPartitionKeyJavaExecutions = []string{
	"ContextKeySegmentedJoinWhereClauseOnPartitionKey",
}

// runContextKeySegmentedJoinWhereClauseOnPartitionKeyScenario replays the
// join-where execution of ContextKeySegmented: a keyed context partitioned
// by theString joining SupportBean#lastevent with SupportBean_S0#lastevent
// filtered by `theString is 'Test'`. The partner event (SupportBean_S0) has
// no partition key, so it fans out to every partition's join state; the
// where clause admits only the Test partition's join result.
func runContextKeySegmentedJoinWhereClauseOnPartitionKeyScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedJoinWhereClauseOnPartitionKeyCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-join-where-clause-on-partition-key case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedJoinWhereClauseOnPartitionKeyCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedJoinWhereClauseOnPartitionKeyBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextKeySegmentedJoinWhereClauseOnPartitionKeyS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	sb := esper.From[contextKeySegmentedJoinWhereClauseOnPartitionKeyBean](env, "SupportBean")
	s0 := esper.From[contextKeySegmentedJoinWhereClauseOnPartitionKeyS0](env, "SupportBean_S0")
	theString := esper.Field[contextKeySegmentedJoinWhereClauseOnPartitionKeyBean, string]("theString")
	if _, err := esper.CreateKeyContext(env, "MyCtx", theString); err != nil {
		return compat.Trace{}, err
	}
	query := esper.Join(
		sb.Window(esper.LastEvent()),
		s0.Window(esper.LastEvent()),
	).Select(
		esper.SelectFrom(0, "sb.theString", esper.JoinField[string](0, "theString")),
		esper.SelectFrom(0, "sb.intPrimitive", esper.JoinField[int](0, "intPrimitive")),
		esper.SelectFrom(1, "s0.id", esper.JoinField[int](1, "id")),
		esper.SelectFrom(1, "s0.p00", esper.JoinField[string](1, "p00")),
	).Where(esper.Equal[string](esper.JoinField[string](0, "theString"), esper.Literal("Test"))).
		Query(esper.StatementName("select"), esper.WithContext("MyCtx"))
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedJoinWhereClauseOnPartitionKeyPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-join-where-clause-on-partition-key statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedJoinWhereClauseOnPartitionKeyPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextKeySegmentedJoinWhereClauseOnPartitionKeyBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextKeySegmentedJoinWhereClauseOnPartitionKeyS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unknown context-key-segmented-join-where-clause-on-partition-key event type %q", step.EventType)
	}
}
