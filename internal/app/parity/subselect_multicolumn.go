package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type subselectMulticolumnS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
}

type subselectMulticolumnS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
}

type subselectMulticolumnBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

var subselectMulticolumnJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectMulticolumn.java",
}

var (
	subselectMulticolumnJavaRuntimeIDs = []string{
		"java-runtime-b2e8b1dca51c23129cfb",
		"java-runtime-ddbf36446a70a786217d",
		"java-runtime-584cf85f99af593afb75",
	}
	subselectMulticolumnJavaExecutions = []string{
		"EPLSubselectMulticolumnAgg",
		"EPLSubselectColumnsUncorrelated",
		"EPLSubselectCorrelatedAggregation",
	}
)

// runSubselectMulticolumnScenario replays 3 observable executions of
// EPLSubselectMulticolumn (4 scenario cases; columns-uncorrelated-om is the
// OM re-deploy round of ColumnsUncorrelated): multi-column subselect row
// maps — fully-aggregated uncorrelated (count/sum over #length(3)), plain
// two-column projection over #lastevent, and correlated aggregation with
// sum/sum(+1)/window(intPrimitive)/window(sb.*) over #keepall.
// EPLSubselectInvalid is excluded (compile-time-only, approved difference).
func runSubselectMulticolumnScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		"multicolumn-agg", "columns-uncorrelated", "columns-uncorrelated-om",
		"correlated-aggregation",
	}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("subselect-multicolumn scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runSubselectMulticolumnCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("subselect-multicolumn case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runSubselectMulticolumnCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[subselectMulticolumnS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subselectMulticolumnS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subselectMulticolumnBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	var query esper.Query
	switch caseName {
	case "multicolumn-agg":
		s1 := esper.From[subselectMulticolumnS1](env, "SupportBean_S1").Window(esper.LengthWindow(3)).AsRecord()
		query = esper.Select(esper.From[subselectMulticolumnS0](env, "SupportBean_S0"),
			esper.Alias("id", esper.Field[subselectMulticolumnS0, int]("id")),
			esper.Alias("s1totals", esper.SubqueryRowWithOptions(
				s1,
				[]esper.Selection{
					esper.Alias("v1", esper.CountAll()),
					esper.Alias("v2", esper.Sum[int](esper.Field[any, int]("id"))),
				},
			)),
		).Query(esper.StatementName("s0"))
	case "columns-uncorrelated", "columns-uncorrelated-om":
		last := esper.From[subselectMulticolumnBean](env, "SupportBean").Window(esper.LastEvent()).AsRecord()
		query = esper.Select(esper.From[subselectMulticolumnS0](env, "SupportBean_S0"),
			esper.Alias("subrow", esper.SubqueryRowWithOptions(
				last,
				[]esper.Selection{
					esper.Alias("v1", esper.Field[any, string]("theString")),
					esper.Alias("v2", esper.Field[any, int]("intPrimitive")),
				},
			)),
		).Query(esper.StatementName("s0"))
	case "correlated-aggregation":
		keepall := esper.From[subselectMulticolumnBean](env, "SupportBean").Window(esper.KeepAll()).AsRecord()
		query = esper.Select(esper.From[subselectMulticolumnS0](env, "SupportBean_S0"),
			esper.Alias("p00", esper.Field[subselectMulticolumnS0, *string]("p00")),
			esper.Alias("subrow", esper.SubqueryRowWithOptions(
				keepall,
				[]esper.Selection{
					esper.Alias("v1", esper.Sum[int](esper.Field[any, int]("intPrimitive"))),
					esper.Alias("v2", esper.Sum[int](esper.Add[int](esper.Field[any, int]("intPrimitive"), esper.Literal(1)))),
					esper.Alias("v3", esper.WindowValues[int](esper.Field[any, int]("intPrimitive"))),
					esper.Alias("v4", esper.WindowEvents()),
				},
				esper.SubqueryWhere(esper.Equal[string](
					esper.Field[any, string]("theString"),
					esper.OuterField[string]("p00"),
				)),
			)),
		).Query(esper.StatementName("s0"))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported subselect-multicolumn case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeSubselectMulticolumnPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown subselect-multicolumn statement %q", name)
		}
		return statement, nil
	})
}

func decodeSubselectMulticolumnPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean_S0":
		var value subselectMulticolumnS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("subselect-multicolumn SupportBean_S0: %w", err)
		}
		if value.P00 != nil {
			value.P00 = ptrString(*value.P00)
		}
		return value, nil
	case "SupportBean_S1":
		var value subselectMulticolumnS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("subselect-multicolumn SupportBean_S1: %w", err)
		}
		return value, nil
	case "SupportBean":
		var value subselectMulticolumnBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("subselect-multicolumn SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("subselect-multicolumn: unsupported event type %q", step.EventType)
	}
}
