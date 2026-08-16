package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedJoinMultitypeMultifieldBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type contextKeySegmentedJoinMultitypeMultifieldS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

const contextKeySegmentedJoinMultitypeMultifieldJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedJoinMultitypeMultifieldJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedJoinMultitypeMultifieldJavaRuntimeIDs = []string{
	"java-runtime-75cacdf00909ff5fb04e", // ContextKeySegmentedJoinMultitypeMultifield
}

var contextKeySegmentedJoinMultitypeMultifieldJavaExecutions = []string{
	"ContextKeySegmentedJoinMultitypeMultifield",
}

// runContextKeySegmentedJoinMultitypeMultifieldScenario replays the
// multi-type multi-field execution of ContextKeySegmented: a multi-stream
// segmented context partitioning SupportBean by (theString, intPrimitive)
// and SupportBean_S0 by (p00, id), with a cross join of the per-partition
// lastevent windows projecting both key fields through context.key1 and
// context.key2. A partition exists only once both sides have fed it, so the
// first four events produce nothing and each completed partition joins its
// own lastevent pair.
func runContextKeySegmentedJoinMultitypeMultifieldScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedJoinMultitypeMultifieldCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-join-multitype-multifield case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedJoinMultitypeMultifieldCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedJoinMultitypeMultifieldBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextKeySegmentedJoinMultitypeMultifieldS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	sb := esper.From[contextKeySegmentedJoinMultitypeMultifieldBean](env, "SupportBean")
	s0 := esper.From[contextKeySegmentedJoinMultitypeMultifieldS0](env, "SupportBean_S0")
	theString := esper.Field[contextKeySegmentedJoinMultitypeMultifieldBean, string]("theString")
	intPrimitive := esper.Field[contextKeySegmentedJoinMultitypeMultifieldBean, int]("intPrimitive")
	p00 := esper.Field[contextKeySegmentedJoinMultitypeMultifieldS0, string]("p00")
	id := esper.Field[contextKeySegmentedJoinMultitypeMultifieldS0, int]("id")
	if _, err := esper.CreateKeyContextByStreams(env, "SegmentedBy2Fields",
		esper.KeyContextStream{Type: "SupportBean", Keys: []esper.Expr{theString, intPrimitive}},
		esper.KeyContextStream{Type: "SupportBean_S0", Keys: []esper.Expr{p00, id}},
	); err != nil {
		return compat.Trace{}, err
	}
	query := esper.Join(
		sb.Window(esper.LastEvent()),
		s0.Window(esper.LastEvent()),
	).Select(
		esper.SelectFrom(0, "c1", esper.JoinField[string](0, "theString")),
		esper.SelectFrom(0, "c2", esper.JoinField[int](0, "intPrimitive")),
		esper.SelectFrom(1, "c3", esper.JoinField[int](1, "id")),
		esper.SelectFrom(1, "c4", esper.JoinField[string](1, "p00")),
		esper.SelectFrom(0, "c5", esper.ContextField[string]("key1")),
		esper.SelectFrom(0, "c6", esper.ContextField[int]("key2")),
	).Query(esper.StatementName("s0"), esper.WithContext("SegmentedBy2Fields"))
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedJoinMultitypeMultifieldPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-join-multitype-multifield statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedJoinMultitypeMultifieldPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextKeySegmentedJoinMultitypeMultifieldBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextKeySegmentedJoinMultitypeMultifieldS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unknown context-key-segmented-join-multitype-multifield event type %q", step.EventType)
	}
}
