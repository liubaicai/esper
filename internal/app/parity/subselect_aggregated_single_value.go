package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type subselectSingleS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
}

type subselectSingleS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
	P11 string `esper:"p11"`
}

type subselectSingleBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type subselectSingleST0 struct {
	ID      string `esper:"id"`
	P01Long *int64 `esper:"p01Long"`
}

type subselectSingleST1 struct {
	ID      string `esper:"id"`
	P11Long *int64 `esper:"p11Long"`
}

type subselectSingleST2 struct {
	ID   string  `esper:"id"`
	Key2 *string `esper:"key2"`
	P20  int     `esper:"p20"`
}

const subselectSingleJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var subselectSingleJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectAggregatedSingleValue.java",
}

var (
	subselectSingleJavaRuntimeIDs = []string{
		"java-runtime-1697def4792eef9e455f",
		"java-runtime-bee3648a1cb51568a329",
		"java-runtime-ddfc4da686e2140599f1",
		"java-runtime-af1a6e27be2a4587fcac",
		"java-runtime-da3530feea7227f22475",
		"java-runtime-999af3370fde9daf0484",
		"java-runtime-8e56c1b7043f63e081e6",
		"java-runtime-5aa8d16a080bba1bf9d9",
		"java-runtime-35be7861ebe96a48817a",
		"java-runtime-58bc2a73c88692723791",
		"java-runtime-494ca71cfcaf96e0696b",
		"java-runtime-b49d7c217bee63a5aab4",
		"java-runtime-fc5e32b56ceb8866146f",
		"java-runtime-73e90c44540e5ba6b1fd",
		"java-runtime-7d4d91983868b27f14b1",
	}
	subselectSingleJavaExecutions = []string{
		"EPLSubselectUngroupedUncorrelatedNoDataWindow",
		"EPLSubselectUngroupedUncorrelatedWHaving",
		"EPLSubselectUngroupedUncorrelatedInSelectClause",
		"EPLSubselectUngroupedUncorrelatedWWhereClause",
		"EPLSubselectUngroupedCorrelatedSceneTwo",
		"EPLSubselectUngroupedCorrelatedInWhereClause",
		"EPLSubselectUngroupedCorrelatedWHaving",
		"EPLSubselectGroupedUncorrelatedWHaving",
		"EPLSubselectGroupedCorrelatedWHaving",
		"EPLSubselectGroupedCorrelationInsideHaving",
		"EPLSubselectUngroupedCorrelationInsideHaving",
		"EPLSubselectUngroupedTableWHaving",
		"EPLSubselectGroupedTableWHaving",
		"EPLSubselectUngroupedJoin3StreamKeyRangeCoercion",
		"EPLSubselectUngroupedJoin2StreamRangeCoercion",
	}
)

// derefOuterString converts a nullable outer property into a plain string
// expression so correlated equality can use the string comparison path.
func derefOuterString(expression esper.Expression[*string]) esper.Expression[string] {
	return esper.Func1[*string, string]("deref", func(value *string) string {
		if value == nil {
			return ""
		}
		return *value
	}, expression)
}

// runSubselectAggregatedSingleValueScenario replays 15 executions of
// EPLSubselectAggregatedSingleValue: single-value aggregate subselects over
// SupportBean (unbound, keepall or length(3), uncorrelated or correlated
// through s0.p00/s0.id, grouped or ungrouped having, including correlated
// expressions inside the subselect projection and having), plus
// table-backed subselects with a having clause.
func runSubselectAggregatedSingleValueScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"no-data-window", "having", "in-select-clause", "where-clause",
		"correlated-scene-two", "correlated-in-where-a", "correlated-in-where-b",
		"correlated-having", "grouped-uncorrelated-having", "grouped-correlated-having",
		"grouped-correlation-inside-having", "ungrouped-correlation-inside-having",
		"ungrouped-table-having", "grouped-table-having",
		"join-3stream-key-range-between", "join-3stream-key-range-no-reversal",
		"join-3stream-key-range-greater-than", "join-3stream-key-range-less-than",
		"join-2stream-range-between-s0-s1", "join-2stream-range-between-s1-s0",
		"join-2stream-range-no-reversal"}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("subselect-aggregated-single-value scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runSubselectSingleCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("subselect-aggregated-single-value case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runSubselectSingleCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[subselectSingleS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subselectSingleS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subselectSingleBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subselectSingleST0](env, "SupportBean_ST0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subselectSingleST1](env, "SupportBean_ST1"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subselectSingleST2](env, "SupportBean_ST2"); err != nil {
		return compat.Trace{}, err
	}

	inner := esper.From[subselectSingleBean](env, "SupportBean").Window(esper.KeepAll()).AsRecord()
	unbound := esper.From[subselectSingleBean](env, "SupportBean").AsRecord()
	theString := esper.Field[any, string]("theString")
	intPrimitive := esper.Field[any, int]("intPrimitive")
	sum := esper.Sum[int](intPrimitive)
	id := esper.Field[subselectSingleS0, int]("id")
	p00 := esper.Field[subselectSingleS0, *string]("p00")
	s1Window := esper.From[subselectSingleS1](env, "SupportBean_S1").Window(esper.LengthWindow(3)).AsRecord()
	s1ID := esper.Field[any, int]("id")

	var query esper.Query
	switch caseName {
	case "no-data-window":
		query = esper.Select(esper.From[subselectSingleS0](env, "SupportBean_S0"),
			esper.Alias("c0", p00),
			esper.Alias("c1", esper.SubquerySum[int](unbound, intPrimitive)),
		).Query(esper.StatementName("s0"))
	case "having":
		having := esper.SubqueryHaving(esper.Greater[int](sum, esper.Literal(100)))
		query = esper.Select(esper.From[subselectSingleS0](env, "SupportBean_S0"),
			esper.Alias("id", id),
			esper.Alias("p00", p00),
			esper.Alias("c0", esper.SubqueryValueWithOptions[int](inner, sum, having)),
			esper.Alias("c1", esper.SubqueryExistsValue[int](inner, sum, having)),
		).Query(esper.StatementName("s0"))
	case "in-select-clause":
		query = esper.Select(esper.From[subselectSingleS0](env, "SupportBean_S0"),
			esper.Alias("value", esper.SubqueryValueWithOptions[int](s1Window,
				esper.Add[int](esper.OuterField[int]("id"), esper.Max[int](s1ID)))),
		).Query(esper.StatementName("s0"))
	case "where-clause":
		query = esper.Select(esper.From[subselectSingleS0](env, "SupportBean_S0"),
			esper.Alias("value", esper.SubquerySum[int](s1Window, s1ID,
				esper.Less[int](s1ID, esper.Literal(0)))),
		).Query(esper.StatementName("s0"))
	case "correlated-scene-two":
		query = esper.Select(esper.From[subselectSingleS0](env, "SupportBean_S0"),
			esper.Alias("id", id),
			esper.Alias("mycount", esper.SubqueryCount(s1Window,
				esper.Equal[string](esper.Field[any, string]("p10"), derefOuterString(esper.OuterField[*string]("p00"))))),
		).Query(esper.StatementName("s0"))
	case "correlated-in-where-a":
		filtered := esper.From[subselectSingleS0](env, "SupportBean_S0").Filter(
			esper.Greater[int](id, esper.SubquerySum[int](inner, intPrimitive,
				esper.Equal[string](theString, derefOuterString(esper.OuterField[*string]("p00"))))))
		query = esper.Select(filtered, esper.Alias("p00", p00)).Query(esper.StatementName("s0"))
	case "correlated-in-where-b":
		filtered := esper.From[subselectSingleS0](env, "SupportBean_S0").Filter(
			esper.Greater[int](id, esper.SubquerySum[int](inner, intPrimitive,
				esper.Equal[string](
					esper.Concat(theString, esper.Literal("X")),
					esper.Concat(derefOuterString(esper.OuterField[*string]("p00")), esper.Literal("X"))))))
		query = esper.Select(filtered, esper.Alias("p00", p00)).Query(esper.StatementName("s0"))
	case "correlated-having":
		query = esper.Select(esper.From[subselectSingleS0](env, "SupportBean_S0"),
			esper.Alias("c0", esper.SubqueryValueWithOptions[int](inner, sum,
				esper.SubqueryWhere(esper.Equal[string](theString, derefOuterString(esper.OuterField[*string]("p00")))),
				esper.SubqueryHaving(esper.Greater[int](sum, esper.Literal(10))))),
		).Query(esper.StatementName("s0"))
	case "grouped-uncorrelated-having":
		query = esper.Select(esper.From[subselectSingleS0](env, "SupportBean_S0"),
			esper.Alias("c0", esper.SubqueryGroupScalar[string, int](inner, theString, sum,
				esper.SubqueryGroupHaving(esper.Greater[int](sum, esper.Literal(10))))),
		).Query(esper.StatementName("s0"))
	case "grouped-correlated-having":
		query = esper.Select(esper.From[subselectSingleS0](env, "SupportBean_S0"),
			esper.Alias("c0", esper.SubqueryGroupScalar[string, int](inner, theString, sum,
				esper.SubqueryGroupWhere(esper.Equal[int](intPrimitive, esper.OuterField[int]("id"))),
				esper.SubqueryGroupHaving(esper.Greater[int](sum, esper.Literal(10))))),
		).Query(esper.StatementName("s0"))
	case "grouped-correlation-inside-having":
		query = esper.Select(esper.From[subselectSingleS0](env, "SupportBean_S0"),
			esper.Alias("c0", esper.SubqueryGroupScalar[string, string](inner, theString, theString,
				esper.SubqueryGroupHaving(esper.Equal[int](sum, esper.OuterField[int]("id"))))),
		).Query(esper.StatementName("s0"))
	case "ungrouped-correlation-inside-having":
		query = esper.Select(esper.From[subselectSingleS0](env, "SupportBean_S0"),
			esper.Alias("c0", esper.SubqueryValueWithOptions[string](inner, esper.Last[string](theString),
				esper.SubqueryHaving(esper.Equal[int](sum, esper.OuterField[int]("id"))))),
		).Query(esper.StatementName("s0"))
	case "join-3stream-key-range-between", "join-3stream-key-range-no-reversal",
		"join-3stream-key-range-greater-than", "join-3stream-key-range-less-than",
		"join-2stream-range-between-s0-s1", "join-2stream-range-between-s1-s0",
		"join-2stream-range-no-reversal":
		// The Java executions correlate an aggregated subselect against
		// properties of every outer joined stream: st2.key2 key equality,
		// s0.p01Long/s1.p11Long as range bounds. JoinField addresses each
		// stream by its FROM-clause position; source order differs between
		// the two between orientations.
		key2Join := esper.JoinField[*string](0, "key2")
		p01Join := esper.JoinField[*int64](1, "p01Long")
		p11Join := esper.JoinField[*int64](2, "p11Long")
		st2Join := esper.JoinSource(esper.From[subselectSingleST2](env, "SupportBean_ST2").Window(esper.LastEvent()))
		st0Join := esper.JoinSource(esper.From[subselectSingleST0](env, "SupportBean_ST0").Window(esper.LastEvent()))
		st1Join := esper.JoinSource(esper.From[subselectSingleST1](env, "SupportBean_ST1").Window(esper.LastEvent()))
		var filter esper.Expression[bool]
		var joinQuery esper.MultiJoinStream
		switch caseName {
		case "join-3stream-key-range-between":
			filter = esper.And(
				esper.Equal[string](theString, derefOuterString(key2Join)),
				esper.BetweenOf(intPrimitive, p01Join, p11Join))
			joinQuery = esper.JoinMany(st2Join, st0Join, st1Join)
		case "join-3stream-key-range-no-reversal":
			filter = esper.And(
				esper.Equal[string](theString, derefOuterString(key2Join)),
				esper.And(
					esper.GreaterOrEqualOf(p11Join, intPrimitive),
					esper.LessOrEqualOf(p01Join, intPrimitive)))
			joinQuery = esper.JoinMany(st2Join, st0Join, st1Join)
		case "join-3stream-key-range-greater-than":
			filter = esper.And(
				esper.Equal[string](theString, derefOuterString(key2Join)),
				esper.GreaterOf(p11Join, intPrimitive))
			joinQuery = esper.JoinMany(st2Join, st0Join, st1Join)
		case "join-3stream-key-range-less-than":
			filter = esper.And(
				esper.Equal[string](theString, derefOuterString(key2Join)),
				esper.LessOf(p11Join, intPrimitive))
			joinQuery = esper.JoinMany(st2Join, st0Join, st1Join)
		case "join-2stream-range-between-s0-s1":
			// FROM ST0, ST1: source 0 is s0, source 1 is s1.
			filter = esper.BetweenOf(intPrimitive,
				esper.JoinField[*int64](0, "p01Long"),
				esper.JoinField[*int64](1, "p11Long"))
			joinQuery = esper.JoinMany(st0Join, st1Join)
		case "join-2stream-range-between-s1-s0":
			// FROM ST1, ST0: source 0 is s1, source 1 is s0; the between
			// operands swap with the FROM order.
			filter = esper.BetweenOf(intPrimitive,
				esper.JoinField[*int64](0, "p11Long"),
				esper.JoinField[*int64](1, "p01Long"))
			joinQuery = esper.JoinMany(st1Join, st0Join)
		case "join-2stream-range-no-reversal":
			// FROM ST0, ST1 with explicit >=/<= bounds; no range reversal.
			filter = esper.And(
				esper.GreaterOrEqualOf(intPrimitive, esper.JoinField[*int64](0, "p01Long")),
				esper.LessOrEqualOf(intPrimitive, esper.JoinField[*int64](1, "p11Long")))
			joinQuery = esper.JoinMany(st0Join, st1Join)
		}
		query = joinQuery.Select(
			esper.SelectLeft("sumi", esper.SubquerySum[int](inner, intPrimitive, filter)),
		).Query(esper.StatementName("s0"))
	case "ungrouped-table-having", "grouped-table-having":
		var intoPlan esper.Plan
		if caseName == "ungrouped-table-having" {
			if _, err := esper.CreateTable(env, "MyTable", []esper.TableColumn{
				esper.TableColumnOf[int64]("total"),
			}); err != nil {
				return compat.Trace{}, err
			}
			intoPlan, err = env.Build(esper.From[subselectSingleBean](env, "SupportBean").Aggregate(
				esper.Alias("total", esper.Sum[int](intPrimitive)),
			).IntoTable("MyTable", esper.StatementName("into")))
			if err != nil {
				return compat.Trace{}, err
			}
		} else {
			if _, err := esper.CreateTable(env, "MyTableWith2Keys", []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("k1"),
				esper.PrimaryKeyColumn[string]("k2"),
				esper.TableColumnOf[int64]("total"),
			}); err != nil {
				return compat.Trace{}, err
			}
			p10 := esper.Field[subselectSingleS1, string]("p10")
			p11 := esper.Field[subselectSingleS1, string]("p11")
			intoPlan, err = env.Build(esper.From[subselectSingleS1](env, "SupportBean_S1").GroupBy(p10, p11).Select(
				esper.Alias("k1", p10),
				esper.Alias("k2", p11),
				esper.Alias("total", esper.Sum[int](esper.Field[subselectSingleS1, int]("id"))),
			).IntoTable("MyTableWith2Keys", esper.StatementName("into")))
			if err != nil {
				return compat.Trace{}, err
			}
		}
		tableTotal := esper.Sum[int64](esper.Field[any, int64]("total"))
		engine := esper.NewEngine(env)
		cleanup := true
		defer func() {
			if cleanup {
				_ = engine.Close(context.Background())
			}
		}()
		deploy := func(plan esper.Plan) (*esper.Statement, error) {
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return nil, err
			}
			if len(deployment.Statements()) != 1 {
				return nil, fmt.Errorf("expected one statement, got %d", len(deployment.Statements()))
			}
			return deployment.Statements()[0], nil
		}
		if _, err := deploy(intoPlan); err != nil {
			return compat.Trace{}, err
		}
		var query esper.Query
		if caseName == "ungrouped-table-having" {
			query = esper.Select(esper.From[subselectSingleS0](env, "SupportBean_S0"),
				esper.Alias("c0", esper.SubqueryValueWithOptions[int64](esper.FromTable(env, "MyTable"), tableTotal,
					esper.SubqueryHaving(esper.Greater[int64](tableTotal, esper.Literal(int64(100)))))),
			).Query(esper.StatementName("s0"))
		} else {
			query = esper.Select(esper.From[subselectSingleS0](env, "SupportBean_S0"),
				esper.Alias("c0", esper.SubqueryGroupScalar[string, int64](esper.FromTable(env, "MyTableWith2Keys"),
					esper.Field[any, string]("k1"), tableTotal,
					esper.SubqueryGroupHaving(esper.Greater[int64](tableTotal, esper.Literal(int64(100)))))),
			).Query(esper.StatementName("s0"))
		}
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		statement, err := deploy(plan)
		if err != nil {
			return compat.Trace{}, err
		}
		cleanup = false
		defer func() { _ = engine.Close(context.Background()) }()
		return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeSubselectSinglePayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown subselect-aggregated-single-value statement %q", name)
			}
			return statement, nil
		})
	default:
		return compat.Trace{}, fmt.Errorf("unsupported subselect-aggregated-single-value case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeSubselectSinglePayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown subselect-aggregated-single-value statement %q", name)
		}
		return statement, nil
	})
}

func decodeSubselectSinglePayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean_S0":
		var value subselectSingleS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value subselectSingleS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	case "SupportBean":
		var value subselectSingleBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_ST0":
		var value subselectSingleST0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_ST0: %w", err)
		}
		return value, nil
	case "SupportBean_ST1":
		var value subselectSingleST1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_ST1: %w", err)
		}
		return value, nil
	case "SupportBean_ST2":
		var value subselectSingleST2
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_ST2: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported subselect-aggregated-single-value event type %q", step.EventType)
	}
}
