package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedPatternFilterBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const contextKeySegmentedPatternFilterJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedPatternFilterJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedPatternFilterJavaRuntimeIDs = []string{
	"java-runtime-88b756bf18da9ee916b1", // ContextKeySegmentedPatternFilter
}

var contextKeySegmentedPatternFilterJavaExecutions = []string{
	"ContextKeySegmentedPatternFilter",
}

// runContextKeySegmentedPatternFilterScenario replays the never-match
// pattern execution of ContextKeySegmented: a keyed context partitioned by
// theString running `every (event1=SupportBean(not-like-%X%) ->
// event2=SupportBean(like-%X%))`. Because the partition key IS theString,
// event1 (no X) and event2 (contains X) can never occur in the same
// partition, so the pattern never fires: four events, zero records. The
// Java oracle normalizes the regression's stringContainsX UDF to the
// equivalent like predicate.
func runContextKeySegmentedPatternFilterScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedPatternFilterCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-pattern-filter case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedPatternFilterCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedPatternFilterBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextKeySegmentedPatternFilterBean](env, "SupportBean")
	theString := esper.Field[contextKeySegmentedPatternFilterBean, string]("theString")
	if _, err := esper.CreateKeyContext(env, "IndividualBean", theString); err != nil {
		return compat.Trace{}, err
	}
	pattern := esper.PatternFrom(beanSource, "event1",
		esper.Not(esper.Contains(theString, esper.Literal("X")))).
		Every().
		FollowedBy("event2", esper.Contains(theString, esper.Literal("X")))
	query := pattern.Select(
		esper.Alias("a_theString", esper.TagField[string]("event1", "theString")),
		esper.Alias("b_theString", esper.TagField[string]("event2", "theString")),
	).Query(esper.StatementName("s0"), esper.WithContext("IndividualBean"))
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedPatternFilterPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-pattern-filter statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedPatternFilterPayload(step compat.Step) (any, error) {
	var value contextKeySegmentedPatternFilterBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
