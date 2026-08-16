package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedMatchRecognizeBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

const contextKeySegmentedMatchRecognizeJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedMatchRecognizeJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedMatchRecognizeJavaRuntimeIDs = []string{
	"java-runtime-3e24f25a32226e390917", // ContextKeySegmentedMatchRecognize
}

var contextKeySegmentedMatchRecognizeJavaExecutions = []string{
	"ContextKeySegmentedMatchRecognize",
}

// runContextKeySegmentedMatchRecognizeScenario replays the match-recognize
// execution of ContextKeySegmented: a keyed context partitioned by
// theString with a per-partition row-recognition pattern (A B) where A has
// intPrimitive=1 and B has intPrimitive=2. Each partition keeps its own
// recognition state: A's events complete an A-B match within their own
// partition, never crossing into B's.
func runContextKeySegmentedMatchRecognizeScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedMatchRecognizeCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-match-recognize case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedMatchRecognizeCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedMatchRecognizeBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextKeySegmentedMatchRecognizeBean](env, "SupportBean")
	theString := esper.Field[contextKeySegmentedMatchRecognizeBean, string]("theString")
	if _, err := esper.CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		return compat.Trace{}, err
	}
	query := beanSource.MatchRecognize(esper.RowSequence(esper.RowVar("A"), esper.RowVar("B"))).
		Define("A", esper.Equal[int](esper.TagField[int]("A", "intPrimitive"), esper.Literal(1))).
		Define("B", esper.Equal[int](esper.TagField[int]("B", "intPrimitive"), esper.Literal(2))).
		Measures(
			esper.Alias("a", esper.TagField[int64]("A", "longPrimitive")),
			esper.Alias("b", esper.TagField[int64]("B", "longPrimitive")),
		).
		Query(esper.StatementName("s0"), esper.WithContext("SegmentedByString"))
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedMatchRecognizePayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-match-recognize statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedMatchRecognizePayload(step compat.Step) (any, error) {
	var value contextKeySegmentedMatchRecognizeBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
