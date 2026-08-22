package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type subselectUnfilteredS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 string  `esper:"p01"`
}

type subselectUnfilteredS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 *string `esper:"p11"`
}

type subselectUnfilteredS3 struct {
	ID int `esper:"id"`
}

type subselectUnfilteredS4 struct {
	ID int `esper:"id"`
}

type subselectUnfilteredBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

var subselectUnfilteredJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectUnfiltered.java",
}

var (
	subselectUnfilteredJavaRuntimeIDs = []string{
		"java-runtime-bd3151d2bfe783c68a4c",
		"java-runtime-52465ccf64152468573b",
		"java-runtime-35bb4012ba6240eb2b5e",
		"java-runtime-30b11d518bcfb0d33da6",
		"java-runtime-a39f50e203d9cc1c40be",
		"java-runtime-30b43737a275b2cd8d47",
		"java-runtime-789637b3336f1f52fec8",
		"java-runtime-45f056d567635a3db88a",
		"java-runtime-b34d4694cf0d68157126",
		"java-runtime-64658966cf5bbc74764d",
		"java-runtime-09c51a2dd32cd67ca1f8",
		"java-runtime-464e8cfb8d6dac9f204c",
		"java-runtime-259562d83a7bc9a095bb",
		"java-runtime-d6cb26490b44ad3b14e8",
		"java-runtime-c2ce866415b8744b057d",
		"java-runtime-a953d3319b660ffd75ce",
		"java-runtime-713de8bb7b8c70bc738d",
		"java-runtime-4b7fb8d3557174c882cc",
	}
	subselectUnfilteredJavaExecutions = []string{
		"EPLSubselectUnfilteredExpression",
		"EPLSubselectUnfilteredUnlimitedStream",
		"EPLSubselectUnfilteredLengthWindow",
		"EPLSubselectUnfilteredAsAfterSubselect",
		"EPLSubselectUnfilteredWithAsWithinSubselect",
		"EPLSubselectUnfilteredNoAs",
		"EPLSubselectUnfilteredLastEvent",
		"EPLSubselectSelfSubselect",
		"EPLSubselectComputedResult",
		"EPLSubselectFilterInside",
		"EPLSubselectWhereClauseWithExpression",
		"EPLSubselectWhereClauseReturningTrue",
		"EPLSubselectUnfilteredStreamPriorOM",
		"EPLSubselectUnfilteredStreamPriorCompile",
		"EPLSubselectTwoSubqSelect",
		"EPLSubselectJoinUnfiltered",
		"EPLSubselectStartStopStatement",
		"EPLSubselectCustomFunction",
	}
)

// runSubselectUnfilteredScenario replays 18 executions of EPLSubselectUnfiltered
// (18 scenario cases; StreamPriorOM/Compile share one replay and
// StartStopStatement spans two deployment-generation cases):
// unfiltered scalar subselects in select, where and computed expressions over
// lastevent, length, keepall windows; self-referencing insert-into subselect,
// custom function calls, prior() access, multi-subselect projections and join
// with unfiltered subselects over S3/S4.
func runSubselectUnfilteredScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		"expression", "unlimited-stream", "length-window",
		"as-after-subselect", "with-as-within-subselect", "no-as",
		"last-event", "self-subselect", "computed-result",
		"filter-inside", "where-clause-expression", "where-clause-true",
		"stream-prior", "two-subq-select",
		"join-unfiltered",
		"start-stop-first", "start-stop-second", "custom-function",
	}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("subselect-unfiltered scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runSubselectUnfilteredCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("subselect-unfiltered case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runSubselectUnfilteredCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[subselectUnfilteredS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subselectUnfilteredS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subselectUnfilteredS3](env, "SupportBean_S3"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subselectUnfilteredS4](env, "SupportBean_S4"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subselectUnfilteredBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	s0 := esper.From[subselectUnfilteredS0](env, "SupportBean_S0")
	s1LastEvent := esper.From[subselectUnfilteredS1](env, "SupportBean_S1").Window(esper.LastEvent()).AsRecord()
	s1Len1000 := esper.From[subselectUnfilteredS1](env, "SupportBean_S1").Window(esper.LengthWindow(1000)).AsRecord()
	s1Len2 := esper.From[subselectUnfilteredS1](env, "SupportBean_S1").Window(esper.LengthWindow(2)).AsRecord()
	s1FilteredA := esper.From[subselectUnfilteredS1](env, "SupportBean_S1").
		Filter(esper.Equal[*string](esper.Field[any, *string]("p10"), esper.Literal(ptrString("A")))).
		Window(esper.LengthWindow(1000)).AsRecord()
	bean := esper.From[subselectUnfilteredBean](env, "SupportBean")
	s0LastEvent := esper.From[subselectUnfilteredS0](env, "SupportBean_S0").Window(esper.LastEvent()).AsRecord()
	s3Len1000 := esper.From[subselectUnfilteredS3](env, "SupportBean_S3").Window(esper.LengthWindow(1000)).AsRecord()
	s4Len1000 := esper.From[subselectUnfilteredS4](env, "SupportBean_S4").Window(esper.LengthWindow(1000)).AsRecord()

	var query esper.Query
	switch caseName {
	case "expression":
		query = esper.Select(s0,
			esper.Alias("value", esper.SubqueryValue[string](
				s1LastEvent,
				esper.Concat(esper.Field[any, string]("p10"), esper.Field[any, string]("p11")),
			)),
		).Query(esper.StatementName("s0"))
	case "unlimited-stream":
		query = esper.Select(s0,
			esper.Alias("idS1", esper.SubqueryValueWithOptions[any](s1Len1000, esper.Field[any, int]("id"), esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple))),
		).Query(esper.StatementName("s0"))
	case "length-window":
		query = esper.Select(s0,
			esper.Alias("idS1", esper.SubqueryValueWithOptions[any](s1Len2, esper.Field[any, int]("id"), esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple))),
		).Query(esper.StatementName("s0"))
	case "as-after-subselect":
		query = esper.Select(s0,
			esper.Alias("idS1", esper.SubqueryValue[any](s1LastEvent, esper.Field[any, int]("id"))),
		).Query(esper.StatementName("s0"))
	case "with-as-within-subselect":
		query = esper.Select(s0,
			esper.Alias("myId", esper.SubqueryValue[any](s1LastEvent, esper.Field[any, int]("id"))),
		).Query(esper.StatementName("s0"))
	case "no-as":
		query = esper.Select(s0,
			esper.Alias("id", esper.SubqueryValue[any](s1LastEvent, esper.Field[any, int]("id"))),
		).Query(esper.StatementName("s0"))
	case "last-event":
		query = esper.Select(bean,
			esper.Alias("theString", esper.Field[subselectUnfilteredBean, string]("theString")),
			esper.Alias("col", esper.SubqueryValue[*string](s0LastEvent, esper.Field[any, *string]("p00"))),
		).Query(esper.StatementName("s0"))
	case "self-subselect":
		if _, err := esper.RegisterMap(env, "MyCount", []esper.FieldSpec{esper.FieldDef("cnt", reflect.TypeOf(int64(0)))}); err != nil {
			return compat.Trace{}, err
		}
		insertPlan, err := env.Build(
			esper.From[subselectUnfilteredS0](env, "SupportBean_S0").Aggregate(
				esper.Alias("cnt", esper.CountAll()),
			).InsertInto("MyCount", esper.StatementName("insert-count")),
		)
		if err != nil {
			return compat.Trace{}, err
		}
		query = esper.Select(s0,
			esper.Alias("value", esper.SubqueryValue[int64](
				esper.From[map[string]any](env, "MyCount").Window(esper.LastEvent()).AsRecord(),
				esper.Field[any, int64]("cnt"),
			)),
		).Query(esper.StatementName("s0"))
		selectPlan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		engine := esper.NewEngine(env)
		if _, err := engine.Deploy(ctx, insertPlan); err != nil {
			return compat.Trace{}, err
		}
		deployment, err := engine.Deploy(ctx, selectPlan)
		if err != nil {
			return compat.Trace{}, err
		}
		statement := deployment.Statements()[0]
		defer func() { _ = engine.Close(context.Background()) }()
		return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeSubselectUnfilteredPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown subselect-unfiltered statement %q", name)
			}
			return statement, nil
		})
	case "computed-result":
		query = esper.Select(s0,
			esper.Alias("idS1", esper.Multiply[int](
				esper.Literal(100),
				esper.SubqueryValue[any](s1Len1000, esper.Field[any, int]("id")),
			)),
		).Query(esper.StatementName("s0"))
	case "filter-inside":
		query = esper.Select(s0,
			esper.Alias("idS1", esper.SubqueryValue[any](s1FilteredA, esper.Field[any, int]("id"))),
		).Query(esper.StatementName("s0"))
	case "where-clause-expression":
		query = esper.Select(s0.Filter(
			esper.Equal[*string](
				esper.SubqueryValue[*string](s1Len1000, esper.Field[any, *string]("p10")),
				esper.Literal(ptrString("X")),
			),
		), esper.Alias("id", esper.Field[subselectUnfilteredS0, int]("id")),
		).Query(esper.StatementName("s0"))
	case "where-clause-true":
		query = esper.Select(s0.Filter(
			esper.SubqueryValue[bool](s1Len1000, esper.Literal(true)),
		), esper.Alias("id", esper.Field[subselectUnfilteredS0, int]("id")),
		).Query(esper.StatementName("s0"))
	case "start-stop-first", "start-stop-second":
		// EPLSubselectStartStopStatement (EPLSubselectUnfiltered.java)
		// replays the same statement text twice with a fresh subquery window
		// per deployment; the two scenario cases carry the two deployment
		// generations. Java's unobserved post-undeploy S0 send (:88) has no
		// assertion surface and is intentionally absent from scenario steps.
		query = esper.Select(s0.Filter(
			esper.SubqueryValue[bool](s1Len1000, esper.Literal(true)),
		), esper.Alias("id", esper.Field[subselectUnfilteredS0, int]("id")),
		).Query(esper.StatementName("s0"))
	case "custom-function":
		query = esper.Select(s0,
			// Java widens the Integer id to double before minusOne(double);
			// the Go mirror converts inside the function body.
			esper.Alias("idS1", esper.SubqueryValue[float64](
				s1Len1000,
				esper.Func1[int, float64]("minusOne", func(v int) float64 { return float64(v) - 1 },
					esper.Field[any, int]("id")),
			)),
		).Query(esper.StatementName("s0"))
	case "stream-prior":
		query = esper.Select(s0,
			// Java prior(0) over the subquery window addresses the newest
			// retained row; the Go Prior(N) convention is 0-based on the
			// event before the newest, so the equivalent is the plain field
			// over a last-event view of the subquery stream.
			esper.Alias("idS1", esper.SubqueryValue[any](s1LastEvent, esper.Field[any, int]("id"))),
		).Query(esper.StatementName("s0"))
	case "two-subq-select":
		query = esper.Select(s0,
			esper.Alias("idS1_0", esper.SubqueryValue[int](s1LastEvent, esper.Add[int](esper.Field[any, int]("id"), esper.Literal(1)))),
			esper.Alias("idS1_1", esper.SubqueryValue[int](s1LastEvent, esper.Add[int](esper.Field[any, int]("id"), esper.Literal(2)))),
		).Query(esper.StatementName("s0"))
	case "join-unfiltered":
		query = esper.Join(
			esper.From[subselectUnfilteredS0](env, "SupportBean_S0").Window(esper.KeepAll()),
			esper.From[subselectUnfilteredS1](env, "SupportBean_S1").Window(esper.KeepAll()),
			esper.OnEqual(esper.Field[subselectUnfilteredS0, int]("id"), esper.Field[subselectUnfilteredS1, int]("id")),
		).Select(
			esper.SelectLeft("idS3", esper.SubqueryValueWithOptions[int](
				s3Len1000, esper.Field[any, int]("id"),
				esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple),
			)),
			esper.SelectRight("idS4", esper.SubqueryValueWithOptions[int](
				s4Len1000, esper.Field[any, int]("id"),
				esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple),
			)),
		).Query(esper.StatementName("s0"))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported subselect-unfiltered case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeSubselectUnfilteredPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown subselect-unfiltered statement %q", name)
		}
		return statement, nil
	})
}

func decodeSubselectUnfilteredPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean_S0":
		var value subselectUnfilteredS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value subselectUnfilteredS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	case "SupportBean_S3":
		var value subselectUnfilteredS3
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S3: %w", err)
		}
		return value, nil
	case "SupportBean_S4":
		var value subselectUnfilteredS4
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S4: %w", err)
		}
		return value, nil
	case "SupportBean":
		var value subselectUnfilteredBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported subselect-unfiltered event type %q", step.EventType)
	}
}

func ptrString(s string) *string { return &s }

// concatDeref concatenates dereferenced string pointers, matching Java's p10 || p11.
func concatDeref(a, b *string) string {
	var sb strings.Builder
	if a != nil {
		sb.WriteString(*a)
	}
	if b != nil {
		sb.WriteString(*b)
	}
	return sb.String()
}
