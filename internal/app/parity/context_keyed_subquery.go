package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeyedSubqueryBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type contextKeyedSubqueryS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

const contextKeyedSubqueryCase = "keyed-subquery"

const contextKeyedSubqueryJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeyedSubqueryJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var (
	contextKeyedSubqueryJavaRuntimeIDs = []string{
		"java-runtime-9aa54450b89fbc94ab73",
	}
	contextKeyedSubqueryJavaExecutions = []string{
		"ContextKeySegmentedSubqueryFiltered",
	}
)

// runContextKeyedSubqueryScenario replays a keyed context whose correlated
// #lastevent subquery starts empty for each newly created partition.
func runContextKeyedSubqueryScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if !scenarioHasCase(scenario, contextKeyedSubqueryCase) {
		return compat.Trace{}, fmt.Errorf("context keyed subquery scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, contextKeyedSubqueryCase)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeyedSubqueryBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextKeyedSubqueryS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateKeyContext(env, "SegmentedByString", esper.Field[contextKeyedSubqueryBean, string]("theString")); err != nil {
		return compat.Trace{}, err
	}
	inner := esper.Select(esper.From[contextKeyedSubqueryS0](env, "SupportBean_S0")).Window(esper.LastEvent())
	query := esper.Select(
		esper.From[contextKeyedSubqueryBean](env, "SupportBean"),
		esper.Alias("theString", esper.Field[contextKeyedSubqueryBean, string]("theString")),
		esper.Alias("intPrimitive", esper.Field[contextKeyedSubqueryBean, int]("intPrimitive")),
		esper.Alias("val0", esper.SubqueryValue[string](
			inner,
			esper.Field[contextKeyedSubqueryS0, string]("p00"),
			esper.Equal[int](
				esper.Field[contextKeyedSubqueryS0, int]("id"),
				esper.OuterField[int]("intPrimitive"),
			),
		)),
	).Query(
		esper.StatementName("s0"),
		esper.WithContext("SegmentedByString"),
	)
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextKeyedSubqueryPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context keyed subquery statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextKeyedSubqueryPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextKeyedSubqueryBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextKeyedSubqueryS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context keyed subquery event type %q", step.EventType)
	}
}
