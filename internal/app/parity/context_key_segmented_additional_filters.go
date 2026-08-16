package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedAdditionalFiltersBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type contextKeySegmentedAdditionalFiltersS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

const contextKeySegmentedAdditionalFiltersJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedAdditionalFiltersJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedAdditionalFiltersJavaRuntimeIDs = []string{
	"java-runtime-1bf346cfb0d6a045df91", // ContextKeySegmentedAdditionalFilters
}

var contextKeySegmentedAdditionalFiltersJavaExecutions = []string{
	"ContextKeySegmentedAdditionalFilters",
}

// runContextKeySegmentedAdditionalFiltersScenario replays the multi-stream
// filtered execution of ContextKeySegmented: a multi-stream segmented
// context partitioning SupportBean by theString (intPrimitive > 0) and
// SupportBean_S0 by p00 (id > 0), with an every OR pattern whose matches
// feed per-partition sums. Events failing the per-stream filters create no
// partition; pattern matches accumulate into each partition's aggregates
// with the match's own captured tags.
func runContextKeySegmentedAdditionalFiltersScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedAdditionalFiltersCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-additional-filters case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedAdditionalFiltersCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedAdditionalFiltersBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextKeySegmentedAdditionalFiltersS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	sb := esper.From[contextKeySegmentedAdditionalFiltersBean](env, "SupportBean")
	s0 := esper.From[contextKeySegmentedAdditionalFiltersS0](env, "SupportBean_S0")
	theString := esper.Field[contextKeySegmentedAdditionalFiltersBean, string]("theString")
	p00 := esper.Field[contextKeySegmentedAdditionalFiltersS0, string]("p00")
	if _, err := esper.CreateKeyContextByStreams(env, "SegmentedByAString",
		esper.KeyContextStream{Type: "SupportBean", Keys: []esper.Expr{theString}},
		esper.KeyContextStream{Type: "SupportBean_S0", Keys: []esper.Expr{p00}},
	); err != nil {
		return compat.Trace{}, err
	}
	pattern := esper.PatternFrom(s0, "s0", esper.Greater[int](esper.Field[contextKeySegmentedAdditionalFiltersS0, int]("id"), esper.Literal(0))).
		Or(esper.PatternFrom(sb, "sb", esper.Greater[int](esper.Field[contextKeySegmentedAdditionalFiltersBean, int]("intPrimitive"), esper.Literal(0)))).
		Every()
	query := pattern.Select(
		esper.Alias("col1", esper.Sum[int](esper.TagField[int]("sb", "intPrimitive"))),
		esper.Alias("col2", esper.Sum[int](esper.TagField[int]("s0", "id"))),
	).Query(esper.StatementName("s0"), esper.WithContext("SegmentedByAString"))
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeySegmentedAdditionalFiltersPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-key-segmented-additional-filters statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeySegmentedAdditionalFiltersPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextKeySegmentedAdditionalFiltersBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextKeySegmentedAdditionalFiltersS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unknown context-key-segmented-additional-filters event type %q", step.EventType)
	}
}
