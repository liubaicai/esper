package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type subqueryS0 struct {
	ID int `esper:"id"`
}

type subqueryS1 struct {
	ID int `esper:"id"`
}

const subqueryJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var subqueryJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectAggregatedSingleValue.java",
}

var (
	subqueryJavaRuntimeIDs = []string{
		"java-runtime-60f4742e4d330725c1e5",
	}
	subqueryJavaExecutions = []string{
		"EPLSubselectUngroupedUncorrelatedInSelect",
	}
)

func runSubqueryScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "subquery"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("subquery scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[subqueryS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subqueryS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	inner := esper.From[subqueryS1](env, "SupportBean_S1").Window(esper.LengthWindow(3)).AsRecord()
	query := esper.Select(
		esper.From[subqueryS0](env, "SupportBean_S0"),
		esper.Alias("value", esper.SubqueryValue[int](inner, esper.Max[int](esper.Field[any, int]("id")))),
	).Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeSubqueryPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown subquery statement %q", name)
		}
		return statement, nil
	})
}

func decodeSubqueryPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean_S0":
		var value subqueryS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value subqueryS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported subquery event type %q", step.EventType)
	}
}
