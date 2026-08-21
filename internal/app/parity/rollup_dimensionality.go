package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type rollupDimensionalityBean struct {
	TheString       string  `esper:"theString"`
	IntPrimitive    int     `esper:"intPrimitive"`
	LongPrimitive   int64   `esper:"longPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
}

type rollupDimensionalityS0 struct {
	ID int `esper:"id"`
}

var rollupDimensionalityJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRollupDimensionality.java",
}

var (
	rollupDimensionalityJavaRuntimeIDs = []string{
		"java-runtime-30499c2e4ff9aece48b2",
		"java-runtime-b059890b735f776a9e03",
		"java-runtime-e60ea25dc87dcfbdcc08",
		"java-runtime-f5da6be14e939f2b26cc",
	}
	rollupDimensionalityJavaExecutions = []string{
		"ResultSetQueryTypeUnboundRollup2Dim",
		"ResultSetQueryTypeUnboundRollup1Dim",
		"ResultSetQueryTypeUnboundRollupUnenclosed",
		"ResultSetQueryTypeUnboundRollup3Dim",
	}
)

// runRollupDimensionalityScenario replays the unbound rollup family of
// ResultSetQueryTypeRollupDimensionality (4 executions across 10 scenario
// cases; the 1-dim rollup/cube pair, the three unenclosed syntax variants,
// and the 3-dim rollup/grouping-sets pair each replay one shared sequence,
// with join variants priming a cartesian SupportBean_S0#lastevent side):
// hierarchical detail-to-overall rows, null-padded aggregated key columns,
// monotonic accumulation on the unbounded stream, and rollup/grouping-sets
// syntax equivalence.
func runRollupDimensionalityScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		"unbound-rollup-2dim",
		"unbound-rollup-1dim-rollup", "unbound-rollup-1dim-cube",
		"unbound-rollup-unenclosed-a", "unbound-rollup-unenclosed-b", "unbound-rollup-unenclosed-c",
		"unbound-rollup-3dim-rollup", "unbound-rollup-3dim-gs",
		"unbound-rollup-3dim-rollup-join", "unbound-rollup-3dim-gs-join",
	}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("rollup-dimensionality scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runRollupDimensionalityCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("rollup-dimensionality case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runRollupDimensionalityCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[rollupDimensionalityBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[rollupDimensionalityS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}

	theString := esper.Field[any, string]("theString")
	intPrimitive := esper.Field[any, int]("intPrimitive")

	var query esper.Query
	switch caseName {
	case "unbound-rollup-2dim":
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").
			GroupByRollup(theString, intPrimitive).
			Select(
				esper.Alias("c0", theString),
				esper.Alias("c1", intPrimitive),
				esper.Alias("c2", esper.Sum[int64](esper.Field[any, int64]("longPrimitive"))),
			).Query(esper.StatementName("s0"))
	case "unbound-rollup-1dim-rollup", "unbound-rollup-1dim-cube":
		stream := esper.From[rollupDimensionalityBean](env, "SupportBean")
		var agg esper.AggregateStream
		if caseName == "unbound-rollup-1dim-rollup" {
			agg = stream.GroupByRollup(theString)
		} else {
			agg = stream.GroupByCube(theString)
		}
		query = agg.Select(
			esper.Alias("c0", theString),
			esper.Alias("c1", esper.Sum[int](intPrimitive)),
		).Query(esper.StatementName("s0"))
	case "unbound-rollup-unenclosed-a", "unbound-rollup-unenclosed-b", "unbound-rollup-unenclosed-c":
		longPrimitive := esper.Field[any, int64]("longPrimitive")
		// The three Java syntax variants (plain key + rollup, explicit
		// grouping sets with the top key repeated, plain key + inner
		// grouping sets) expand to the same three grouping sets.
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").
			GroupByGroupingSets(
				esper.GroupingSet(theString, intPrimitive, longPrimitive),
				esper.GroupingSet(theString, intPrimitive),
				esper.GroupingSet(theString),
			).Select(
			esper.Alias("c0", theString),
			esper.Alias("c1", intPrimitive),
			esper.Alias("c2", longPrimitive),
			esper.Alias("c3", esper.Sum[float64](esper.Field[any, float64]("doublePrimitive"))),
		).Query(esper.StatementName("s0"))
	case "unbound-rollup-3dim-rollup", "unbound-rollup-3dim-gs",
		"unbound-rollup-3dim-rollup-join", "unbound-rollup-3dim-gs-join":
		join := caseName == "unbound-rollup-3dim-rollup-join" || caseName == "unbound-rollup-3dim-gs-join"
		explicitSets := caseName == "unbound-rollup-3dim-gs" || caseName == "unbound-rollup-3dim-gs-join"
		longPrimitive := esper.Field[any, int64]("longPrimitive")
		doublePrimitive := esper.Field[any, float64]("doublePrimitive")
		var agg esper.AggregateStream
		if !join {
			stream := esper.From[rollupDimensionalityBean](env, "SupportBean")
			if explicitSets {
				agg = stream.GroupByGroupingSets(
					esper.GroupingSet(theString, intPrimitive, longPrimitive),
					esper.GroupingSet(theString, intPrimitive),
					esper.GroupingSet(theString),
					esper.GroupingSet(),
				)
			} else {
				agg = stream.GroupByRollup(theString, intPrimitive, longPrimitive)
			}
			query = agg.Select(
				esper.Alias("c0", theString),
				esper.Alias("c1", intPrimitive),
				esper.Alias("c2", longPrimitive),
				esper.Alias("c3", esper.CountAll()),
				esper.Alias("c4", esper.Sum[float64](doublePrimitive)),
			).Query(esper.StatementName("s0"))
		} else {
			// Cartesian join against the single-row primed side stream.
			jTheString := esper.JoinField[any](0, "theString")
			jIntPrimitive := esper.JoinField[any](0, "intPrimitive")
			jLongPrimitive := esper.JoinField[any](0, "longPrimitive")
			agg = esper.Join(
				esper.From[rollupDimensionalityBean](env, "SupportBean").Window(esper.KeepAll()),
				esper.From[rollupDimensionalityS0](env, "SupportBean_S0").Window(esper.LastEvent()),
			).GroupBy(jTheString, jIntPrimitive, jLongPrimitive)
			if explicitSets {
				agg = agg.GroupingSets(
					esper.GroupingSet(jTheString, jIntPrimitive, jLongPrimitive),
					esper.GroupingSet(jTheString, jIntPrimitive),
					esper.GroupingSet(jTheString),
					esper.GroupingSet(),
				)
			} else {
				agg = agg.Rollup(jTheString, jIntPrimitive, jLongPrimitive)
			}
			query = agg.Select(
				esper.Alias("c0", jTheString),
				esper.Alias("c1", jIntPrimitive),
				esper.Alias("c2", jLongPrimitive),
				esper.Alias("c3", esper.CountAll()),
				esper.Alias("c4", esper.Sum[float64](esper.JoinField[float64](0, "doublePrimitive"))),
			).Query(esper.StatementName("s0"))
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported rollup-dimensionality case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeRollupDimensionalityPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown rollup-dimensionality statement %q", name)
		}
		return statement, nil
	})
}

func longPrimitiveField() esper.Expr {
	return esper.Field[any, int64]("longPrimitive")
}

func decodeRollupDimensionalityPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var bean rollupDimensionalityBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("rollup-dimensionality SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_S0":
		var event rollupDimensionalityS0
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("rollup-dimensionality SupportBean_S0: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("rollup-dimensionality: unsupported event type %q", step.EventType)
	}
}
