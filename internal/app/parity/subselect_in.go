package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type subselectInS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 string  `esper:"p01"`
}

type subselectInS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 *string `esper:"p11"`
}

type subselectInBean struct {
	TheString string `esper:"theString"`
	IntBoxed  *int64 `esper:"intBoxed"`
	LongBoxed *int64 `esper:"longBoxed"`
}

type subselectInWildcardS2 struct {
	ID  int     `esper:"id"`
	P20 *string `esper:"p20"`
	P21 *string `esper:"p21"`
}

type subselectInWildcardMap struct {
	AnyObject any `esper:"anyObject"`
}

const subselectInJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var subselectInJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectIn.java",
}

var (
	subselectInJavaRuntimeIDs = []string{
		"java-runtime-89105b3cad99d9063f41",
		"java-runtime-fdbcd79e160d0517e820",
		"java-runtime-b12caa00c143fc43c04e",
		"java-runtime-f7e01f372cd72fdf1be5",
		"java-runtime-e291b5da908b66ede9c3",
		"java-runtime-c187889237cc3556dc2c",
		"java-runtime-d52b12c5372d3922a9b2",
		"java-runtime-ca0cf0245938a62a8afc",
		"java-runtime-4ae92db4360046f4c27c",
		"java-runtime-57815ee2825665c40f31",
		"java-runtime-af6f36fa0a98caf23fbe",
		"java-runtime-7ca64412e62473c7d2d7",
		"java-runtime-65e715bf6d8eebf884ee",
		"java-runtime-ad48b4b457ce24456800",
		"java-runtime-462a485c982dc5360d48",
	}
	subselectInJavaExecutions = []string{
		"EPLSubselectInSelect",
		"EPLSubselectInSelectOM",
		"EPLSubselectInSelectCompile",
		"EPLSubselectInSelectWhere",
		"EPLSubselectInSelectWhereExpressions",
		"EPLSubselectInFilterCriteria",
		"EPLSubselectInWildcard",
		"EPLSubselectInNullable",
		"EPLSubselectInNullableCoercion",
		"EPLSubselectInNullRow",
		"EPLSubselectInSingleIndex",
		"EPLSubselectInMultiIndex",
		"EPLSubselectNotInNullRow",
		"EPLSubselectNotInSelect",
		"EPLSubselectNotInNullableCoercion",
	}
)

// runSubselectInScenario replays 15 executions of EPLSubselectIn: IN/NOT IN
// subselects in the select clause, filter criteria and where clause over
// SupportBean_S1 length windows with eviction, expression forms on both
// sides, nullable string and boxed numeric coercions, null rows, the
// correlated keepall index shapes of EPLSubselectInSingleIndex/MultiIndex,
// and the whole-event wildcard subselect of EPLSubselectInWildcard.
func runSubselectInScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"in-select", "in-select-om", "in-select-compile", "in-filter-criteria",
		"in-select-where", "in-select-where-expressions", "in-nullable", "in-nullable-coercion",
		"in-null-row", "in-single-index", "in-multi-index", "not-in-null-row", "not-in-select",
		"not-in-nullable-coercion", "in-wildcard"}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("subselect-in scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runSubselectInCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("subselect-in case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runSubselectInCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[subselectInS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subselectInS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subselectInBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	s0 := esper.From[subselectInS0](env, "SupportBean_S0")
	s1Window := esper.From[subselectInS1](env, "SupportBean_S1").Window(esper.LengthWindow(1000)).AsRecord()
	s1Len2 := esper.From[subselectInS1](env, "SupportBean_S1").Window(esper.LengthWindow(2)).AsRecord()
	s1ID := esper.Field[any, int]("id")
	id := esper.Field[subselectInS0, int]("id")
	p00 := esper.Field[subselectInS0, *string]("p00")
	bean := esper.From[subselectInBean](env, "SupportBean")
	theString := esper.Field[subselectInBean, string]("theString")
	intBoxed := esper.Field[subselectInBean, *int64]("intBoxed")
	longBoxed := esper.Field[subselectInBean, *int64]("longBoxed")
	innerB := esper.From[subselectInBean](env, "SupportBean").
		Filter(esper.Equal[string](esper.Field[any, string]("theString"), esper.Literal("B"))).
		Window(esper.LengthWindow(1000)).AsRecord()
	s0KeepAll := esper.From[subselectInS0](env, "SupportBean_S0").Window(esper.KeepAll()).AsRecord()

	var query esper.Query
	switch caseName {
	case "in-select", "in-select-om", "in-select-compile":
		query = esper.Select(s0,
			esper.Alias("value", esper.SubqueryIn[int](id, s1Window, s1ID)),
		).Query(esper.StatementName("s0"))
	case "in-filter-criteria":
		query = esper.Select(s0.Filter(esper.SubqueryIn[int](id, s1Len2, s1ID)),
			esper.Alias("id", id),
		).Query(esper.StatementName("s0"))
	case "in-select-where":
		query = esper.Select(s0,
			esper.Alias("value", esper.SubqueryInWithOptions[int](id, s1Window, s1ID,
				esper.SubqueryWhere(esper.Greater[int](s1ID, esper.Literal(0))))),
		).Query(esper.StatementName("s0"))
	case "in-select-where-expressions":
		query = esper.Select(s0,
			esper.Alias("value", esper.SubqueryIn[int](
				esper.Multiply[int](id, esper.Literal(3)),
				s1Window,
				esper.Multiply[int](s1ID, esper.Literal(2)))),
		).Query(esper.StatementName("s0"))
	case "in-nullable":
		query = esper.Select(s0.Filter(esper.SubqueryIn[*string](p00, s1Window, esper.Field[any, *string]("p10"))),
			esper.Alias("id", id),
		).Query(esper.StatementName("s0"))
	case "in-nullable-coercion":
		query = esper.Select(bean.Filter(esper.Equal[string](theString, esper.Literal("A"))).
			Filter(esper.SubqueryIn[*int64](longBoxed, innerB, esper.Field[any, *int64]("intBoxed"))),
			esper.Alias("longBoxed", longBoxed),
		).Query(esper.StatementName("s0"))
	case "in-null-row":
		query = esper.Select(bean.Filter(esper.Equal[string](theString, esper.Literal("A"))).
			Filter(esper.SubqueryIn[*int64](intBoxed, innerB, esper.Field[any, *int64]("longBoxed"))),
			esper.Alias("intBoxed", intBoxed),
		).Query(esper.StatementName("s0"))
	case "in-single-index":
		query = esper.Select(esper.From[subselectInS1](env, "SupportBean_S1"),
			esper.Alias("c0", esper.SubqueryValueWithOptions[string](s0KeepAll, esper.Field[any, string]("p00"),
				esper.SubqueryWhere(esper.In[string](
					esper.Field[any, string]("p01"),
					derefOuterString(esper.OuterField[*string]("p10")),
					derefOuterString(esper.OuterField[*string]("p11")))))),
		).Query(esper.StatementName("s0"))
	case "in-multi-index":
		query = esper.Select(esper.From[subselectInS1](env, "SupportBean_S1"),
			esper.Alias("c0", esper.SubqueryValueWithOptions[string](s0KeepAll, esper.Field[any, string]("p00"),
				esper.SubqueryWhere(esper.In[string](
					derefOuterString(esper.OuterField[*string]("p11")),
					esper.Field[any, string]("p00"),
					esper.Field[any, string]("p01"))))),
		).Query(esper.StatementName("s0"))
	case "not-in-null-row":
		query = esper.Select(bean.Filter(esper.Equal[string](theString, esper.Literal("A"))).
			Filter(esper.Not(esper.SubqueryIn[*int64](intBoxed, innerB, esper.Field[any, *int64]("longBoxed")))),
			esper.Alias("intBoxed", intBoxed),
		).Query(esper.StatementName("s0"))
	case "not-in-select":
		query = esper.Select(s0,
			esper.Alias("value", esper.Not(esper.SubqueryIn[int](id, s1Window, s1ID))),
		).Query(esper.StatementName("s0"))
	case "in-wildcard":
		// EPLSubselectInWildcard: anyObject in (select * from S1#length(1000)).
		// SubqueryIn compares the anyObject payload against each whole-event
		// window row; the S1 window rows and the anyObject field decode to
		// the same struct type, while S2 is a distinct struct, reproducing
		// Java's class-checked equals for the pinned sends. The two event
		// types below exist only for this case, mirroring the oracle's
		// case-scoped registration branch.
		if _, err := esper.RegisterStruct[subselectInWildcardS2](env, "SupportBean_S2"); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.RegisterStruct[subselectInWildcardMap](env, "SupportBeanArrayCollMap"); err != nil {
			return compat.Trace{}, err
		}
		wildcardMap := esper.From[subselectInWildcardMap](env, "SupportBeanArrayCollMap")
		anyObject := esper.Field[subselectInWildcardMap, any]("anyObject")
		query = esper.Select(wildcardMap,
			esper.Alias("value", esper.SubqueryIn[any](anyObject, s1Window, esper.EventValue[any]())),
		).Query(esper.StatementName("s0"))
	case "not-in-nullable-coercion":
		query = esper.Select(bean.Filter(esper.Equal[string](theString, esper.Literal("A"))).
			Filter(esper.Not(esper.SubqueryIn[*int64](longBoxed, innerB, esper.Field[any, *int64]("intBoxed")))),
			esper.Alias("longBoxed", longBoxed),
		).Query(esper.StatementName("s0"))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported subselect-in case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeSubselectInPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown subselect-in statement %q", name)
		}
		return statement, nil
	})
}

func decodeSubselectInPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean_S0":
		var value subselectInS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value subselectInS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	case "SupportBean":
		var value subselectInBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S2":
		var value subselectInWildcardS2
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S2: %w", err)
		}
		return value, nil
	case "SupportBeanArrayCollMap":
		// The nested anyObject reference is encoded as
		// {"type":"SupportBean_S1"|"SupportBean_S2","id":N,"pXX":...}; the
		// type field selects the concrete struct so the decoded value
		// compares structurally equal to the window rows of that type.
		var payload struct {
			AnyObject struct {
				Type string  `json:"type"`
				ID   int     `json:"id"`
				P10  *string `json:"p10"`
				P11  *string `json:"p11"`
				P20  *string `json:"p20"`
				P21  *string `json:"p21"`
			} `json:"anyObject"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBeanArrayCollMap: %w", err)
		}
		var resolved any
		switch payload.AnyObject.Type {
		case "SupportBean_S1":
			resolved = subselectInS1{ID: payload.AnyObject.ID, P10: payload.AnyObject.P10, P11: payload.AnyObject.P11}
		case "SupportBean_S2":
			resolved = subselectInWildcardS2{ID: payload.AnyObject.ID, P20: payload.AnyObject.P20, P21: payload.AnyObject.P21}
		default:
			return nil, fmt.Errorf("unsupported anyObject reference type %q", payload.AnyObject.Type)
		}
		return subselectInWildcardMap{AnyObject: resolved}, nil
	default:
		return nil, fmt.Errorf("unsupported subselect-in event type %q", step.EventType)
	}
}
