package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedJoinBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type contextKeySegmentedJoinS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

const contextKeySegmentedJoinJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedJoinJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedJoinJavaRuntimeIDs = []string{
	"java-runtime-3418bdab4165eb2024ab", // ContextKeySegmentedJoin
}

var contextKeySegmentedJoinJavaExecutions = []string{
	"ContextKeySegmentedJoin",
}

// runContextKeySegmentedJoinScenario replays the join execution of
// ContextKeySegmented: a keyed context partitioned by theString joining
// SupportBean#keepall with SupportBean_S0#keepall on intPrimitive = id.
// The partner stream (SupportBean_S0) has no partition key, so its events
// fan out to every existing partition's join state: G2 joins S0(20) into
// {G2,20,20}, a partition created later (G3) never sees earlier S0 events,
// and both G1 and G2 join S0(30).
func runContextKeySegmentedJoinScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedJoinCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-join case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedJoinCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedJoinBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextKeySegmentedJoinS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	sb := esper.From[contextKeySegmentedJoinBean](env, "SupportBean")
	s0 := esper.From[contextKeySegmentedJoinS0](env, "SupportBean_S0")
	theString := esper.Field[contextKeySegmentedJoinBean, string]("theString")
	if _, err := esper.CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		return compat.Trace{}, err
	}
	query := esper.Join(
		sb.Window(esper.KeepAll()),
		s0.Window(esper.KeepAll()),
		esper.OnSourcesEqual(
			0, esper.Field[contextKeySegmentedJoinBean, int]("intPrimitive"),
			1, esper.Field[contextKeySegmentedJoinS0, int]("id"),
		),
	).Select(
		esper.SelectFrom(0, "sb.theString", esper.JoinField[string](0, "theString")),
		esper.SelectFrom(0, "sb.intPrimitive", esper.JoinField[int](0, "intPrimitive")),
		esper.SelectFrom(1, "s0.id", esper.JoinField[int](1, "id")),
	).Query(esper.StatementName("s0"), esper.WithContext("SegmentedByString"))
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedJoinPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-join statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedJoinPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextKeySegmentedJoinBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextKeySegmentedJoinS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unknown context-key-segmented-join event type %q", step.EventType)
	}
}
