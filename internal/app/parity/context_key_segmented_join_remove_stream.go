package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedJoinRemoveStreamBean struct {
	PageName  string `esper:"pageName"`
	SessionID string `esper:"sessionId"`
}

const contextKeySegmentedJoinRemoveStreamJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedJoinRemoveStreamJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedJoinRemoveStreamJavaRuntimeIDs = []string{
	"java-runtime-28299eb40969c972fd64", // ContextKeySegmentedJoinRemoveStream
}

var contextKeySegmentedJoinRemoveStreamJavaExecutions = []string{
	"ContextKeySegmentedJoinRemoveStream",
}

// runContextKeySegmentedJoinRemoveStreamScenario replays the remove-stream
// execution of ContextKeySegmented: a keyed context partitioned by
// sessionId joining three per-partition time(30) windows (Start, Middle,
// End) with full outer semantics and a where clause admitting only
// incomplete sessions. The rstream statement emits exactly one row when
// session 3's Start event expires: {Start, 3, null, End}.
func runContextKeySegmentedJoinRemoveStreamScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedJoinRemoveStreamCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-join-remove-stream case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedJoinRemoveStreamCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedJoinRemoveStreamBean](env, "SupportWebEvent"); err != nil {
		return compat.Trace{}, err
	}
	source := esper.From[contextKeySegmentedJoinRemoveStreamBean](env, "SupportWebEvent")
	sessionID := esper.Field[contextKeySegmentedJoinRemoveStreamBean, string]("sessionId")
	pageName := esper.Field[contextKeySegmentedJoinRemoveStreamBean, string]("pageName")
	if _, err := esper.CreateKeyContext(env, "SegmentedBySession", sessionID); err != nil {
		return compat.Trace{}, err
	}
	start := source.Filter(esper.Equal[string](pageName, esper.Literal("Start"))).Window(esper.TimeWindow(30 * time.Second))
	middle := source.Filter(esper.Equal[string](pageName, esper.Literal("Middle"))).Window(esper.TimeWindow(30 * time.Second))
	end := source.Filter(esper.Equal[string](pageName, esper.Literal("End"))).Window(esper.TimeWindow(30 * time.Second))
	chain := esper.JoinChain(esper.JoinSource(start)).
		FullOuterJoin(esper.JoinSource(middle), esper.OnSourcesEqual(
			0, esper.Field[contextKeySegmentedJoinRemoveStreamBean, string]("sessionId"),
			1, esper.Field[contextKeySegmentedJoinRemoveStreamBean, string]("sessionId"),
		)).
		FullOuterJoin(esper.JoinSource(end), esper.OnSourcesEqual(
			0, esper.Field[contextKeySegmentedJoinRemoveStreamBean, string]("sessionId"),
			2, esper.Field[contextKeySegmentedJoinRemoveStreamBean, string]("sessionId"),
		))
	query := chain.Select(
		esper.SelectFrom(0, "pageNameA", esper.JoinField[string](0, "pageName")),
		esper.SelectFrom(0, "sessionIdA", esper.JoinField[string](0, "sessionId")),
		esper.SelectFrom(1, "pageNameB", esper.JoinField[string](1, "pageName")),
		esper.SelectFrom(2, "pageNameC", esper.JoinField[string](2, "pageName")),
	).Where(esper.And(
		esper.Not(esper.IsNull[string](esper.JoinField[string](0, "pageName"))),
		esper.Or(
			esper.IsNull[string](esper.JoinField[string](1, "pageName")),
			esper.IsNull[string](esper.JoinField[string](2, "pageName")),
		),
	)).Query(esper.StatementName("s0"), esper.WithContext("SegmentedBySession"), esper.WithRemoveStreamOnly())
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env)
	initialTime, err := time.Parse(time.RFC3339Nano, "1970-01-01T00:00:00Z")
	if err != nil {
		_ = engine.Close(context.Background())
		return compat.Trace{}, err
	}
	if err := engine.AdvanceTime(ctx, initialTime); err != nil {
		_ = engine.Close(context.Background())
		return compat.Trace{}, err
	}
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedJoinRemoveStreamPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-join-remove-stream statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedJoinRemoveStreamPayload(step compat.Step) (any, error) {
	var value contextKeySegmentedJoinRemoveStreamBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportWebEvent: %w", err)
	}
	return value, nil
}
