package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type subselectQuantifiedBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

var subselectQuantifiedJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectAllAnySomeExpr.java",
}

var (
	subselectQuantifiedJavaRuntimeIDs = []string{
		"java-runtime-b79aed44a6781acea60e",
		"java-runtime-f59ce457c593e1f327d6",
		"java-runtime-34a01c2e4da4d05de0cd",
		"java-runtime-0d291fddb8228d4ec1cf",
	}
	subselectQuantifiedJavaExecutions = []string{
		"EPLSubselectRelationalOpAll",
		"EPLSubselectRelationalOpSome",
		"EPLSubselectEqualsNotEqualsAll",
		"EPLSubselectEqualsAnyOrSome",
	}
)

// runSubselectQuantifiedScenario replays 4 executions of
// EPLSubselectAllAnySomeExpr (5 scenario cases; relational-all-om is the
// fresh-statement OM re-deploy round of RelationalOpAll): quantified
// comparisons value <op> ALL/ANY/SOME (subselect) over a keepall inner
// stream filtered by like 'S%', outer stream filtered by like 'E%'.
func runSubselectQuantifiedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		"relational-all", "relational-all-om", "relational-some",
		"equals-not-equals-all", "equals-any-or-some",
	}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("subselect-quantified scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runSubselectQuantifiedCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("subselect-quantified case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runSubselectQuantifiedCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[subselectQuantifiedBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	inner := func() esper.RecordStream {
		return esper.From[subselectQuantifiedBean](env, "SupportBean").
			Filter(esper.Like(esper.Field[subselectQuantifiedBean, string]("theString"), esper.Literal("S%"))).
			Window(esper.KeepAll()).AsRecord()
	}
	outer := esper.From[subselectQuantifiedBean](env, "SupportBean").
		Filter(esper.Like(esper.Field[subselectQuantifiedBean, string]("theString"), esper.Literal("E%")))
	intField := esper.Field[any, int]("intPrimitive")

	var query esper.Query
	switch caseName {
	case "relational-all", "relational-all-om":
		query = esper.Select(outer,
			esper.Alias("g", esper.SubqueryAll[int](intField, inner(), intField, esper.SubqueryGreater)),
			esper.Alias("ge", esper.SubqueryAll[int](intField, inner(), intField, esper.SubqueryGreaterOrEqual)),
			esper.Alias("l", esper.SubqueryAll[int](intField, inner(), intField, esper.SubqueryLess)),
			esper.Alias("le", esper.SubqueryAll[int](intField, inner(), intField, esper.SubqueryLessOrEqual)),
		).Query(esper.StatementName("s0"))
	case "relational-some":
		query = esper.Select(outer,
			esper.Alias("g", esper.SubqueryAny[int](intField, inner(), intField, esper.SubqueryGreater)),
			esper.Alias("ge", esper.SubqueryAny[int](intField, inner(), intField, esper.SubqueryGreaterOrEqual)),
			esper.Alias("l", esper.SubqueryAny[int](intField, inner(), intField, esper.SubqueryLess)),
			esper.Alias("le", esper.SubqueryAny[int](intField, inner(), intField, esper.SubqueryLessOrEqual)),
		).Query(esper.StatementName("s0"))
	case "equals-not-equals-all":
		query = esper.Select(outer,
			esper.Alias("eq", esper.SubqueryAll[int](intField, inner(), intField, esper.SubqueryEqual)),
			esper.Alias("neq", esper.SubqueryAll[int](intField, inner(), intField, esper.SubqueryNotEqual)),
			esper.Alias("sqlneq", esper.SubqueryAll[int](intField, inner(), intField, esper.SubqueryNotEqual)),
			esper.Alias("nneq", esper.Not(esper.SubqueryAll[int](intField, inner(), intField, esper.SubqueryEqual))),
		).Query(esper.StatementName("s0"))
	case "equals-any-or-some":
		query = esper.Select(outer,
			esper.Alias("r1", esper.SubquerySome[int](intField, inner(), intField, esper.SubqueryEqual)),
			esper.Alias("r2", esper.SubqueryAny[int](intField, inner(), intField, esper.SubqueryEqual)),
			esper.Alias("r3", esper.SubquerySome[int](intField, inner(), intField, esper.SubqueryNotEqual)),
			esper.Alias("r4", esper.SubqueryAny[int](intField, inner(), intField, esper.SubqueryNotEqual)),
		).Query(esper.StatementName("s0"))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported subselect-quantified case %q", caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeSubselectQuantifiedPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown subselect-quantified statement %q", name)
		}
		return statement, nil
	})
}

func decodeSubselectQuantifiedPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var bean subselectQuantifiedBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("subselect-quantified SupportBean: %w", err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("subselect-quantified: unsupported event type %q", step.EventType)
	}
}
