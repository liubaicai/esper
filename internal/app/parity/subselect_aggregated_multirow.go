package parity

import (
	"context"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type subselectMultirowS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
}

type subselectMultirowS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
}

type subselectMultirowBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type subselectMultirowIntArray struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
	Value int    `esper:"value"`
}

type subselectMultirowManyArray struct {
	ID     string `esper:"id"`
	IntOne []int  `esper:"intOne"`
	Value  int    `esper:"value"`
}

var subselectMultirowJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectAggregatedMultirowAndColumn.java",
}

var subselectMultirowJavaRuntimeIDs = []string{
	"java-runtime-7439294a50e35300e1f5",
	"java-runtime-8b69ed212525b9fe6858",
	"java-runtime-3588e48d4e4c6ce21ebb",
	"java-runtime-565c9b30d531a846f0fc",
	"java-runtime-4443d12f747068acf95b",
	"java-runtime-3b7148f2961227ed1393",
	"java-runtime-fe88aa9f7b78478355ae",
	"java-runtime-43ca7182e3e88cd6cbd3",
	"java-runtime-03633b6922f9aa57bd0e",
	"java-runtime-5143812b917f2fdb46a2",
	"java-runtime-2b6ae595427b46dd84fa",
	"java-runtime-ab74e2ffba8b7d8188d3",
}

var subselectMultirowJavaExecutions = []string{
	"EPLSubselectMultirowGroupedNoDataWindowUncorrelated",
	"EPLSubselectMultirowGroupedCorrelatedWithEnumMethod",
	"EPLSubselectMultirowGroupedUncorrelatedWithEnumerationMethod",
	"EPLSubselectMultirowGroupedCorrelatedWHaving",
	"EPLSubselectMultirowGroupedNamedWindowSubqueryIndexShared",
	"EPLSubselectMulticolumnGroupedUncorrelatedUnfiltered",
	"EPLSubselectMulticolumnGroupedUncorrelatedIteratorAndExpressionDef",
	"EPLSubselectMulticolumnGroupedContextPartitioned",
	"EPLSubselectMulticolumnGroupedWHaving",
	"EPLSubselectMulticolumnGroupBy",
	"EPLSubselectMultirowGroupedMultikeyWArray",
	"EPLSubselectMultirowGroupedIndexSharedMultikeyWArray",
}

// runSubselectMultirowScenario replays 12 observable executions of
// EPLSubselectAggregatedMultirowAndColumn (17 scenario cases; the enum
// unfiltered/filtered pair, the nodelete/redeploy pair and the indexshare
// uncorrelated/correlated pair each split one execution across two cases):
// grouped multirow subselects (collection and row forms) over plain streams
// and named windows, with correlation, having, enumeration methods, context
// partitioning, expression declarations and multikey-array group keys.
// EPLSubselectMulticolumnInvalid is excluded (compile-time-only, approved
// difference).
func runSubselectMultirowScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		"multirow-nodatawindow",
		"multirow-enum-unfiltered",
		"multirow-enum-filtered",
		"multirow-correlated-enum",
		"multirow-correlated-having",
		"indexshare-uncorrelated",
		"indexshare-correlated",
		"grouped-row-nodelete",
		"grouped-row-nodelete-redeploy",
		"grouped-row-namedwindow-delete",
		"grouped-row-multigroup",
		"grouped-iterator-exprdef",
		"grouped-context-partitioned",
		"grouped-row-whaving",
		"grouped-keepall-take",
		"multikey-array-scalar",
		"indexshare-multikey-array",
	}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("subselect-multirow scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runSubselectMultirowCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("subselect-multirow case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runSubselectMultirowCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	for _, registration := range []struct {
		name string
		fn   func() error
	}{
		{"SupportBean_S0", func() error { _, err := esper.RegisterStruct[subselectMultirowS0](env, "SupportBean_S0"); return err }},
		{"SupportBean_S1", func() error { _, err := esper.RegisterStruct[subselectMultirowS1](env, "SupportBean_S1"); return err }},
		{"SupportBean", func() error { _, err := esper.RegisterStruct[subselectMultirowBean](env, "SupportBean"); return err }},
		{"SupportEventWithIntArray", func() error {
			_, err := esper.RegisterStruct[subselectMultirowIntArray](env, "SupportEventWithIntArray")
			return err
		}},
		{"SupportEventWithManyArray", func() error {
			_, err := esper.RegisterStruct[subselectMultirowManyArray](env, "SupportEventWithManyArray")
			return err
		}},
	} {
		if err := registration.fn(); err != nil {
			return compat.Trace{}, err
		}
	}

	sbKeepall := func() esper.RecordStream {
		return esper.From[subselectMultirowBean](env, "SupportBean").Window(esper.KeepAll()).AsRecord()
	}
	sbPlain := esper.From[subselectMultirowBean](env, "SupportBean")
	s0 := esper.From[subselectMultirowS0](env, "SupportBean_S0")
	intField := esper.Field[any, int]("intPrimitive")
	strField := esper.Field[any, string]("theString")
	groupSelections := func() []esper.Selection {
		return []esper.Selection{
			esper.Alias("c0", strField),
			esper.Alias("c1", esper.Sum[int](intField)),
		}
	}

	var query esper.Query
	switch caseName {
	case "multirow-nodatawindow":
		query = esper.Select(s0,
			esper.Alias("subq", esper.EnumTake[map[string]any](
				esper.SubqueryGroupRows(sbPlain.AsRecord(), strField, groupSelections()), 10)),
		).Query(esper.StatementName("s0"))
	case "multirow-enum-unfiltered":
		query = esper.Select(s0,
			esper.Alias("subq", esper.EnumTake[map[string]any](
				esper.SubqueryGroupRows(sbKeepall(), strField, groupSelections()), 100)),
		).Query(esper.StatementName("s0"))
	case "multirow-enum-filtered":
		query = esper.Select(s0,
			esper.Alias("subq", esper.EnumTake[map[string]any](
				esper.SubqueryGroupRows(sbKeepall(), strField, groupSelections(),
					esper.SubqueryGroupWhere(esper.Greater[int](intField, esper.Literal(100)))), 100)),
		).Query(esper.StatementName("s0"))
	case "multirow-correlated-enum":
		query = esper.Select(s0,
			esper.Alias("subq", esper.EnumTake[map[string]any](
				esper.SubqueryGroupRows(sbKeepall(), strField, groupSelections(),
					esper.SubqueryGroupWhere(esper.Equal[int](intField, esper.OuterField[int]("id")))), 100)),
		).Query(esper.StatementName("s0"))
	case "multirow-correlated-having":
		query = esper.Select(s0,
			esper.Alias("subq", esper.EnumTake[map[string]any](
				esper.SubqueryGroupRows(sbKeepall(), strField, groupSelections(),
					esper.SubqueryGroupWhere(esper.Equal[int](intField, esper.OuterField[int]("id"))),
					esper.SubqueryGroupHaving(esper.Greater[int](
						esper.Sum[int](intField), esper.Literal(10)))), 100)),
		).Query(esper.StatementName("s0"))
	case "grouped-keepall-take":
		query = esper.Select(s0,
			esper.Alias("e1", esper.EnumTake[map[string]any](
				esper.SubqueryGroupRows(sbKeepall(), strField, groupSelections()), 10)),
		).Query(esper.StatementName("s0"))
	case "grouped-row-nodelete", "grouped-row-nodelete-redeploy":
		query = esper.Select(s0,
			esper.Alias("subq", esper.SubqueryGroupRow(sbKeepall(), strField, groupSelections())),
		).Query(esper.StatementName("s0"))
	case "grouped-row-whaving":
		query = esper.Select(s0,
			esper.Alias("subq", esper.SubqueryGroupRow(sbKeepall(), strField, groupSelections(),
				esper.SubqueryGroupHaving(esper.Greater[int](esper.Sum[int](intField), esper.Literal(10))))),
		).Query(esper.StatementName("s0"))
	case "grouped-row-multigroup":
		query = esper.Select(s0,
			esper.Alias("subq", esper.SubqueryGroupRow(sbKeepall(),
				esper.ArrayOf[any](strField, intField),
				[]esper.Selection{
					esper.Alias("c0", strField),
					esper.Alias("c1", intField),
					esper.Alias("c2", esper.Concat(strField, esper.Literal("x"))),
					esper.Alias("c3", esper.Multiply[int](intField, esper.Literal(1000))),
					esper.Alias("c4", esper.Sum[int64](esper.Field[any, int64]("longPrimitive"))),
				})),
		).Query(esper.StatementName("s0"))
	case "multikey-array-scalar":
		swia := esper.From[subselectMultirowIntArray](env, "SupportEventWithIntArray").Window(esper.KeepAll()).AsRecord()
		query = esper.Select(sbPlain,
			esper.Alias("subq", esper.SubqueryGroupScalar[[]int, int](
				swia, esper.Field[any, []int]("array"), esper.Sum[int](esper.Field[any, int]("value")))),
		).Query(esper.StatementName("s0"))
	case "indexshare-uncorrelated", "indexshare-correlated":
		return runSubselectMultirowIndexShareCase(ctx, env, caseScenario, caseName)
	case "grouped-row-namedwindow-delete":
		return runSubselectMultirowNamedWindowDeleteCase(ctx, env, caseScenario)
	case "grouped-iterator-exprdef":
		return runSubselectMultirowIteratorCase(ctx, env, caseScenario)
	case "grouped-context-partitioned":
		return runSubselectMultirowContextCase(ctx, env, caseScenario)
	case "indexshare-multikey-array":
		return runSubselectMultirowIndexShareArrayCase(ctx, env, caseScenario)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported subselect-multirow case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeSubselectMultirowPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown subselect-multirow statement %q", name)
		}
		return statement, nil
	})
}
