package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type subselectFilteredBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type subselectFilteredS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
}

type subselectFilteredS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 *string `esper:"p11"`
}

type subselectFilteredManyArray struct {
	ID     string `esper:"id"`
	IntOne []int  `esper:"intOne"`
	Value  int    `esper:"value"`
}

type subselectFilteredIntArray struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
	Value int    `esper:"value"`
}

var subselectFilteredJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectFiltered.java",
}

var (
	subselectFilteredJavaRuntimeIDs = []string{
		"java-runtime-bc6684a32b1cda80e244",
		"java-runtime-9511e607f74ca4551624",
		"java-runtime-0ef90e75f854b7845de0",
		"java-runtime-57e3956886ac655d387d",
		"java-runtime-6034a5785b901739431e",
		"java-runtime-643236df4a8946ab3c24",
		"java-runtime-9255e3470adb866211bf",
		"java-runtime-7fbee5b6cef2f287ee41",
	}
	subselectFilteredJavaExecutions = []string{
		"EPLSubselectHavingNoAggNoFilterNoWhere",
		"EPLSubselectHavingNoAggWWhere",
		"EPLSubselectHavingNoAggWFilterWWhere",
		"EPLSubselectWhereConstant",
		"EPLSubselectSelectWithWhereJoined",
		"EPLSubselectWhereClauseMultikeyWArrayPrimitive",
		"EPLSubselectWhereClauseMultikeyWArray2Field",
		"EPLSubselectWhereClauseMultikeyWArrayComposite",
	}
)

// runSubselectFilteredScenario replays the scalar-filter and multikey-wArray
// slices of EPLSubselectFiltered (8 executions across 10 scenario cases;
// WhereConstant contributes three single-deployment cases): non-aggregated
// having row filters, constant and correlated where predicates, and the
// null-on-empty/null-on-multiple scalar subselect boundaries, plus int[]
// content-equality correlation keys.
func runSubselectFilteredScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		"having-no-filter-no-where", "having-w-where", "having-w-filter-w-where",
		"where-constant-single-column", "where-constant-two-column", "where-constant-range",
		"select-with-where-joined",
		"multikey-array-primitive", "multikey-array-two-field", "multikey-array-composite",
	}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("subselect-filtered scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runSubselectFilteredCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("subselect-filtered case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runSubselectFilteredCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	for _, register := range []func() error{
		func() error { _, err := esper.RegisterStruct[subselectFilteredBean](env, "SupportBean"); return err },
		func() error { _, err := esper.RegisterStruct[subselectFilteredS0](env, "SupportBean_S0"); return err },
		func() error { _, err := esper.RegisterStruct[subselectFilteredS1](env, "SupportBean_S1"); return err },
		func() error {
			_, err := esper.RegisterStruct[subselectFilteredManyArray](env, "SupportEventWithManyArray")
			return err
		},
		func() error {
			_, err := esper.RegisterStruct[subselectFilteredIntArray](env, "SupportEventWithIntArray")
			return err
		},
	} {
		if err := register(); err != nil {
			return compat.Trace{}, err
		}
	}

	beanInner := func() esper.RecordStream {
		return esper.From[subselectFilteredBean](env, "SupportBean").Window(esper.KeepAll()).AsRecord()
	}
	s1Inner := func() esper.RecordStream {
		return esper.From[subselectFilteredS1](env, "SupportBean_S1").Window(esper.LengthWindow(1000)).AsRecord()
	}
	outer := esper.From[subselectFilteredS0](env, "SupportBean_S0")

	var query esper.Query
	switch caseName {
	case "having-no-filter-no-where":
		query = esper.Select(outer,
			esper.Alias("c0", esper.SubqueryValueWithOptions[int](beanInner(),
				esper.Field[any, int]("intPrimitive"),
				esper.SubqueryHaving(esper.Equal[string](esper.Field[any, string]("theString"), esper.Literal("ID1"))),
			)),
		).Query(esper.StatementName("s0"))
	case "having-w-where":
		query = esper.Select(outer,
			esper.Alias("c0", esper.SubqueryValueWithOptions[int](beanInner(),
				esper.Field[any, int]("intPrimitive"),
				esper.SubqueryWhere(esper.Greater[int](esper.Field[any, int]("intPrimitive"), esper.Literal(15))),
				esper.SubqueryHaving(esper.Equal[string](esper.Field[any, string]("theString"), esper.Literal("ID1"))),
			)),
		).Query(esper.StatementName("s0"))
	case "having-w-filter-w-where":
		filteredInner := esper.From[subselectFilteredBean](env, "SupportBean").
			Filter(esper.Less[int](esper.Field[subselectFilteredBean, int]("intPrimitive"), esper.Literal(20))).
			Window(esper.KeepAll()).AsRecord()
		query = esper.Select(outer,
			esper.Alias("c0", esper.SubqueryValueWithOptions[int](filteredInner,
				esper.Field[any, int]("intPrimitive"),
				esper.SubqueryWhere(esper.Greater[int](esper.Field[any, int]("intPrimitive"), esper.Literal(15))),
				esper.SubqueryHaving(esper.Equal[string](esper.Field[any, string]("theString"), esper.Literal("ID1"))),
			)),
		).Query(esper.StatementName("s0"))
	case "where-constant-single-column":
		query = esper.Select(outer,
			esper.Alias("ids1", esper.SubqueryValueWithOptions[int](s1Inner(),
				esper.Field[any, int]("id"),
				esper.SubqueryWhere(esper.Equal[string](esper.Field[any, string]("p10"), esper.Literal("X"))),
				esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple),
			)),
		).Query(esper.StatementName("s0"))
	case "where-constant-two-column":
		query = esper.Select(outer,
			esper.Alias("ids1", esper.SubqueryValue[int](s1Inner(),
				esper.Field[any, int]("id"),
				esper.And(
					esper.Equal[string](esper.Field[any, string]("p10"), esper.Literal("X")),
					esper.Equal[string](esper.Field[any, string]("p11"), esper.Literal("Y")),
				),
			)),
		).Query(esper.StatementName("s0"))
	case "where-constant-range":
		lastEventInner := esper.From[subselectFilteredBean](env, "SupportBean").Window(esper.LastEvent()).AsRecord()
		query = esper.Select(outer,
			esper.Alias("ids1", esper.SubqueryValue[string](lastEventInner,
				esper.Field[any, string]("theString"),
				esper.Between[int](esper.Field[any, int]("intPrimitive"), esper.Literal(10), esper.Literal(20)),
			)),
		).Query(esper.StatementName("s0"))
	case "select-with-where-joined":
		query = esper.Select(outer,
			esper.Alias("ids1", esper.SubqueryValue[int](s1Inner(),
				esper.Field[any, int]("id"),
				esper.Equal[string](esper.Field[any, string]("p10"), esper.OuterField[string]("p00")),
			)),
		).Query(esper.StatementName("s0"))
	case "multikey-array-primitive", "multikey-array-two-field", "multikey-array-composite":
		manyArrayInner := func() esper.RecordStream {
			return esper.From[subselectFilteredManyArray](env, "SupportEventWithManyArray").Window(esper.KeepAll()).AsRecord()
		}
		intArrayOuter := esper.From[subselectFilteredIntArray](env, "SupportEventWithIntArray")
		arrayEqual := esper.Is(esper.Field[any, any]("intOne"), esper.OuterField[any]("array"))
		var predicate esper.Expression[bool]
		switch caseName {
		case "multikey-array-primitive":
			predicate = arrayEqual
		case "multikey-array-two-field":
			predicate = esper.And(arrayEqual,
				esper.Equal[int](esper.Field[any, int]("value"), esper.OuterField[int]("value")))
		default:
			predicate = esper.And(arrayEqual,
				esper.Greater[int](esper.Field[any, int]("value"), esper.OuterField[int]("value")))
		}
		query = esper.Select(intArrayOuter,
			esper.Alias("value", esper.SubqueryValueWithOptions[string](manyArrayInner(),
				esper.Field[any, string]("id"),
				esper.SubqueryWhere(predicate),
				esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple),
			)),
		).Query(esper.StatementName("s0"))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported subselect-filtered case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeSubselectFilteredPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown subselect-filtered statement %q", name)
		}
		return statement, nil
	})
}

func decodeSubselectFilteredPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var bean subselectFilteredBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("subselect-filtered SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_S0":
		var event subselectFilteredS0
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("subselect-filtered SupportBean_S0: %w", err)
		}
		return event, nil
	case "SupportBean_S1":
		var event subselectFilteredS1
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("subselect-filtered SupportBean_S1: %w", err)
		}
		return event, nil
	case "SupportEventWithManyArray":
		var event subselectFilteredManyArray
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("subselect-filtered SupportEventWithManyArray: %w", err)
		}
		return event, nil
	case "SupportEventWithIntArray":
		var event subselectFilteredIntArray
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("subselect-filtered SupportEventWithIntArray: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("subselect-filtered: unsupported event type %q", step.EventType)
	}
}
