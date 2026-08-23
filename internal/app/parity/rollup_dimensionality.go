package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type rollupDimensionalityBean struct {
	TheString       string  `esper:"theString"`
	IntPrimitive    int     `esper:"intPrimitive"`
	LongPrimitive   int64   `esper:"longPrimitive"`
	ShortPrimitive  int16   `esper:"shortPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
	IntBoxed        *int    `esper:"intBoxed"`
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
		"java-runtime-b06640d26b3b63075791",
		"java-runtime-14c4aecdca8b446299f1",
		"java-runtime-3b6467afa76b0966c475",
		"java-runtime-274b66386ba625a8b24c",
		"java-runtime-c3795d43550db4779a8c",
		"java-runtime-19844b30add2415c2bcc",
		"java-runtime-effa54ebac66e75bdcb6",
		"java-runtime-3f406a35c51cd03ab734",
		"java-runtime-e17c22ce9356d4acfad9",
		"java-runtime-58abe8e5ebfa57510a04",
		"java-runtime-8178e315c9b40968a218",
		"java-runtime-153edf6a4d78354167f1",
		"java-runtime-23faa930e8e9cbc671c2",
		"java-runtime-a4a78ec3230ca521cf03",
		"java-runtime-0a3b5f198b8a6467078a",
		"java-runtime-3f45c2d30f96ffe3c19f"}
	rollupDimensionalityJavaExecutions = []string{
		"ResultSetQueryTypeUnboundRollup2Dim",
		"ResultSetQueryTypeUnboundRollup1Dim",
		"ResultSetQueryTypeUnboundRollupUnenclosed",
		"ResultSetQueryTypeUnboundRollup3Dim",
		"ResultSetQueryTypeUnboundCubeUnenclosed",
		"ResultSetQueryTypeUnboundCube4Dim",
		"ResultSetQueryTypeBoundRollup2Dim",
		"ResultSetQueryTypeUnboundRollup2DimBatchWindow",
		"ResultSetQueryTypeRollupMultikeyWArray{join=false, unbound=true}",
		"ResultSetQueryTypeRollupMultikeyWArray{join=false, unbound=false}",
		"ResultSetQueryTypeRollupMultikeyWArray{join=true, unbound=false}",
		"ResultSetQueryTypeRollupMultikeyWArrayGroupingSet",
		"ResultSetQueryTypeNamedWindowCube2Dim",
		"ResultSetQueryTypeOnSelect",
		"ResultSetQueryTypeOutputWhenTerminated",
		"ResultSetQueryTypeBoundGroupingSet2LevelNoTopNoDetail",
		"ResultSetQueryTypeBoundGroupingSet2LevelTopAndDetail",
		"ResultSetQueryTypeMixedAccessAggregation",
		"ResultSetQueryTypeNonBoxedTypeWithRollup",
		"ResultSetQueryTypeGroupByWithComputation"}
)

// runRollupDimensionalityScenario replays the unbound rollup, cube and
// bound/batch family of ResultSetQueryTypeRollupDimensionality (8 executions
// across 16 scenario cases; the 1-dim rollup/cube pair, the three unenclosed
// syntax variants, the two 3-dim pairs, the cube-unenclosed trio, the 4-dim
// cube, and the bound/batch pair each replay one shared sequence):
// hierarchical detail-to-overall rows, null-padded aggregated key columns,
// monotonic accumulation on unbounded and windowed streams, rollup/cube/
// grouping-sets syntax equivalence, cube bitmask row ordering, and batch
// flush new/old IR pairs.
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
		"unbound-cube-unenclosed-a", "unbound-cube-unenclosed-b", "unbound-cube-unenclosed-c",
		"unbound-cube-4dim",
		"bound-rollup", "unbound-rollup-2dim-batch",
		// Draft 4.239 completion: the 12 remaining executions. nw-cube-gs
		// replays the named-window cube's grouping-sets spelling (byte-equal
		// to the cube form); the five out-when-term variants share one
		// runtime ID, one per output-limit/hint combination.
		"warray-unbound", "warray-bound", "warray-join", "warray-gs",
		"nw-cube", "nw-cube-gs",
		"onselect-rollup",
		"out-when-term-last", "out-when-term-last-opt", "out-when-term-last-optdis",
		"out-when-term-all", "out-when-term-snapshot",
		"bound-gs-no-top", "bound-gs-top-detail",
		"mixed-access", "non-boxed-types", "groupby-computation",
	}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("rollup-dimensionality scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		var trace compat.Trace
		var err error
		if rollupDimensionalityExtendedCases[caseName] {
			trace, err = runRollupDimensionalityExtendedCase(ctx, scenario, caseName)
		} else {
			trace, err = runRollupDimensionalityCase(ctx, scenario, caseName)
		}
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
	longPrimitive := esper.Field[any, int64]("longPrimitive")
	doublePrimitive := esper.Field[any, float64]("doublePrimitive")

	var query esper.Query
	switch caseName {
	case "unbound-rollup-2dim":
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").
			GroupByRollup(theString, intPrimitive).
			Select(
				esper.Alias("c0", theString),
				esper.Alias("c1", intPrimitive),
				esper.Alias("c2", esper.Sum[int64](longPrimitive)),
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
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").
			GroupByGroupingSets(
				esper.GroupingSet(theString, intPrimitive, longPrimitive),
				esper.GroupingSet(theString, intPrimitive),
				esper.GroupingSet(theString),
			).Select(
			esper.Alias("c0", theString),
			esper.Alias("c1", intPrimitive),
			esper.Alias("c2", longPrimitive),
			esper.Alias("c3", esper.Sum[float64](doublePrimitive)),
		).Query(esper.StatementName("s0"))
	case "unbound-rollup-3dim-rollup", "unbound-rollup-3dim-gs":
		stream := esper.From[rollupDimensionalityBean](env, "SupportBean")
		var agg esper.AggregateStream
		if caseName == "unbound-rollup-3dim-gs" {
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
	case "unbound-rollup-3dim-rollup-join", "unbound-rollup-3dim-gs-join":
		jTheString := esper.JoinField[any](0, "theString")
		jIntPrimitive := esper.JoinField[any](0, "intPrimitive")
		jLongPrimitive := esper.JoinField[any](0, "longPrimitive")
		var agg esper.AggregateStream
		joinAgg := esper.Join(
			esper.From[rollupDimensionalityBean](env, "SupportBean").Window(esper.KeepAll()),
			esper.From[rollupDimensionalityS0](env, "SupportBean_S0").Window(esper.LastEvent()),
		).GroupBy(jTheString, jIntPrimitive, jLongPrimitive)
		if caseName == "unbound-rollup-3dim-gs-join" {
			agg = joinAgg.GroupingSets(
				esper.GroupingSet(jTheString, jIntPrimitive, jLongPrimitive),
				esper.GroupingSet(jTheString, jIntPrimitive),
				esper.GroupingSet(jTheString),
				esper.GroupingSet(),
			)
		} else {
			agg = joinAgg.Rollup(jTheString, jIntPrimitive, jLongPrimitive)
		}
		query = agg.Select(
			esper.Alias("c0", jTheString),
			esper.Alias("c1", jIntPrimitive),
			esper.Alias("c2", jLongPrimitive),
			esper.Alias("c3", esper.CountAll()),
			esper.Alias("c4", esper.Sum[float64](esper.JoinField[float64](0, "doublePrimitive"))),
		).Query(esper.StatementName("s0"))
	case "unbound-cube-unenclosed-a", "unbound-cube-unenclosed-b", "unbound-cube-unenclosed-c":
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").
			GroupByGroupingSets(
				esper.GroupingSet(theString, intPrimitive, longPrimitive),
				esper.GroupingSet(theString, intPrimitive),
				esper.GroupingSet(theString, longPrimitive),
				esper.GroupingSet(theString),
			).Select(
			esper.Alias("c0", theString),
			esper.Alias("c1", intPrimitive),
			esper.Alias("c2", longPrimitive),
			esper.Alias("c3", esper.Sum[float64](doublePrimitive)),
		).Query(esper.StatementName("s0"))
	case "unbound-cube-4dim":
		intBoxed := esper.Field[any, *int]("intBoxed")
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").
			GroupByCube(theString, intPrimitive, longPrimitive, doublePrimitive).
			Select(
				esper.Alias("c0", theString),
				esper.Alias("c1", intPrimitive),
				esper.Alias("c2", longPrimitive),
				esper.Alias("c3", doublePrimitive),
				esper.Alias("c4", esper.Sum[int](intBoxed)),
			).Query(esper.StatementName("s0"))
	case "bound-rollup":
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").
			Window(esper.LengthWindow(3)).
			GroupByRollup(theString, intPrimitive).
			Select(
				esper.Alias("c0", theString),
				esper.Alias("c1", intPrimitive),
				esper.Alias("c2", esper.Sum[int64](longPrimitive)),
			).Query(esper.StatementName("s0"))
	case "unbound-rollup-2dim-batch":
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").
			Window(esper.LengthBatch(4)).
			GroupByRollup(theString, intPrimitive).
			Select(
				esper.Alias("c0", theString),
				esper.Alias("c1", intPrimitive),
				esper.Alias("c2", esper.Sum[int64](longPrimitive)),
			).Query(esper.StatementName("s0"), esper.WithOldStream())
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

// rollupDimensionalityIntArray mirrors the pinned SupportEventWithIntArray
// regression bean: int[] group keys compare by content.
type rollupDimensionalityIntArray struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
	Value int    `esper:"value"`
}

// rollupDimensionalityThreeArray mirrors SupportThreeArrayEvent with three
// differently typed array properties.
type rollupDimensionalityThreeArray struct {
	ID          string    `esper:"id"`
	Value       int       `esper:"value"`
	IntArray    []int     `esper:"intArray"`
	LongArray   []int64   `esper:"longArray"`
	DoubleArray []float64 `esper:"doubleArray"`
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
	case "SupportEventWithIntArray":
		var event rollupDimensionalityIntArray
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("rollup-dimensionality SupportEventWithIntArray: %w", err)
		}
		return event, nil
	case "SupportThreeArrayEvent":
		var event rollupDimensionalityThreeArray
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("rollup-dimensionality SupportThreeArrayEvent: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("rollup-dimensionality: unsupported event type %q", step.EventType)
	}
}

// rollupDimensionalityExtendedCases routes the Draft 4.239 completion cases
// through the multi-statement/handler-capable runner.
var rollupDimensionalityExtendedCases = map[string]bool{
	"warray-unbound": true, "warray-bound": true, "warray-join": true, "warray-gs": true,
	"nw-cube": true, "nw-cube-gs": true,
	"onselect-rollup":    true,
	"out-when-term-last": true, "out-when-term-last-opt": true, "out-when-term-last-optdis": true,
	"out-when-term-all": true, "out-when-term-snapshot": true,
	"bound-gs-no-top": true, "bound-gs-top-detail": true,
	"mixed-access": true, "non-boxed-types": true, "groupby-computation": true,
}

// rollupDimensionalityFullBean mirrors every Java SupportBean property for
// window(*) recursive rendering (MixedAccessAggregation). Nullable boxes stay
// nil when absent; the char primitive defaults to Java's '\u0000'.
type rollupDimensionalityFullBean struct {
	TheString       *string  `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int      `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	CharPrimitive   string   `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	IntBoxed        *int     `esper:"intBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	CharBoxed       *string  `esper:"charBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BigDecimal      *float64 `esper:"bigDecimal"`
	BigInteger      *int64   `esper:"bigInteger"`
	EnumValue       *string  `esper:"enumValue"`
}

// runRollupDimensionalityExtendedCase replays the Draft 4.239 completion
// cases: multi-statement modules (named windows, contexts, on-select), the
// array-keyed rollup family, output-when-terminated variants, and the
// deploy-only types record.
func runRollupDimensionalityExtendedCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	// mixed-access registers a full-fidelity SupportBean mirror in its own
	// branch and must not collide with the short-form registration here.
	if caseName != "mixed-access" {
		if _, err := esper.RegisterStruct[rollupDimensionalityBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
	}
	if _, err := esper.RegisterStruct[rollupDimensionalityS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}

	var plans []esper.Plan
	s0Name := "s0"
	build := func(query esper.Query) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		plans = append(plans, plan)
		return nil
	}

	intPrimitive := func() esper.Expr { return esper.Field[rollupDimensionalityBean, int]("intPrimitive") }
	doublePrimitive := func() esper.Expr { return esper.Field[rollupDimensionalityBean, float64]("doublePrimitive") }

	switch caseName {
	case "warray-unbound", "warray-bound":
		if _, err := esper.RegisterStruct[rollupDimensionalityIntArray](env, "SupportEventWithIntArray"); err != nil {
			return compat.Trace{}, err
		}
		arr := esper.Field[rollupDimensionalityIntArray, []int]("array")
		val := esper.Field[rollupDimensionalityIntArray, int]("value")
		stream := esper.From[rollupDimensionalityIntArray](env, "SupportEventWithIntArray")
		if caseName == "warray-bound" {
			stream = stream.Window(esper.KeepAll())
		}
		err := build(stream.GroupByRollup(arr, val).Select(
			esper.Alias("array", arr),
			esper.Alias("value", val),
			esper.Alias("cnt", esper.CountAll()),
		).Query(esper.StatementName(s0Name)))
		if err != nil {
			return compat.Trace{}, err
		}
	case "warray-join":
		if _, err := esper.RegisterStruct[rollupDimensionalityIntArray](env, "SupportEventWithIntArray"); err != nil {
			return compat.Trace{}, err
		}
		jArr := esper.JoinField[any](0, "array")
		jVal := esper.JoinField[any](0, "value")
		joinAgg := esper.Join(
			esper.From[rollupDimensionalityIntArray](env, "SupportEventWithIntArray").Window(esper.KeepAll()),
			esper.From[rollupDimensionalityBean](env, "SupportBean").Window(esper.KeepAll()),
		).GroupBy().Rollup(jArr, jVal)
		err := build(joinAgg.Select(
			esper.Alias("array", jArr),
			esper.Alias("value", jVal),
			esper.Alias("cnt", esper.CountAll()),
		).Query(esper.StatementName(s0Name)))
		if err != nil {
			return compat.Trace{}, err
		}
	case "warray-gs":
		if _, err := esper.RegisterStruct[rollupDimensionalityThreeArray](env, "SupportThreeArrayEvent"); err != nil {
			return compat.Trace{}, err
		}
		intArray := esper.Field[rollupDimensionalityThreeArray, []int]("intArray")
		longArray := esper.Field[rollupDimensionalityThreeArray, []int64]("longArray")
		doubleArray := esper.Field[rollupDimensionalityThreeArray, []float64]("doubleArray")
		value := esper.Field[rollupDimensionalityThreeArray, int]("value")
		err := build(esper.From[rollupDimensionalityThreeArray](env, "SupportThreeArrayEvent").
			GroupByGroupingSets(
				esper.GroupingSet(intArray),
				esper.GroupingSet(longArray),
				esper.GroupingSet(doubleArray),
			).Select(
			esper.Alias("thesum", esper.Sum[int](value)),
		).Query(esper.StatementName(s0Name)))
		if err != nil {
			return compat.Trace{}, err
		}
	case "nw-cube", "nw-cube-gs":
		schema, err := esper.StructSchema[rollupDimensionalityBean]("SupportBean")
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindow", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		intBoxed := esper.Field[rollupDimensionalityBean, *int]("intBoxed")
		insertPlan, err := env.Build(esper.OnEvent(esper.From[rollupDimensionalityBean](env, "SupportBean").Filter(
			esper.EqualOf(intBoxed, esper.Literal(0)),
		)).InsertIntoNamedWindow("MyWindow",
			esper.CopyMatchingFields(),
		).Query(esper.StatementName("insert")))
		if err != nil {
			return compat.Trace{}, err
		}
		plans = append(plans, insertPlan)
		deletePlan, err := env.Build(esper.OnEvent(esper.From[rollupDimensionalityBean](env, "SupportBean").Filter(
			esper.EqualOf(intBoxed, esper.Literal(3)),
		)).DeleteFromNamedWindow("MyWindow", nil).
			Query(esper.StatementName("delete")))
		if err != nil {
			return compat.Trace{}, err
		}
		plans = append(plans, deletePlan)
		ts := esper.Field[rollupDimensionalityBean, string]("theString")
		ip := esper.Field[rollupDimensionalityBean, int]("intPrimitive")
		lp := esper.Field[rollupDimensionalityBean, int64]("longPrimitive")
		var agg esper.AggregateStream
		if caseName == "nw-cube" {
			agg = esper.FromNamedWindow(env, "MyWindow").GroupByCube(ts, ip)
		} else {
			agg = esper.FromNamedWindow(env, "MyWindow").GroupByGroupingSets(
				esper.GroupingSet(ts, ip),
				esper.GroupingSet(ts),
				esper.GroupingSet(ip),
				esper.GroupingSet(),
			)
		}
		err = build(agg.Select(
			esper.Alias("c0", ts),
			esper.Alias("c1", ip),
			esper.Alias("c2", esper.Sum[int64](lp)),
		).Query(esper.StatementName(s0Name), esper.WithOldStream()))
		if err != nil {
			return compat.Trace{}, err
		}
	case "onselect-rollup":
		schema, err := esper.StructSchema[rollupDimensionalityBean]("SupportBean")
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindow", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		insertPlan, err := env.Build(esper.OnEvent(esper.From[rollupDimensionalityBean](env, "SupportBean")).
			InsertIntoNamedWindow("MyWindow", esper.CopyMatchingFields()).
			Query(esper.StatementName("insert")))
		if err != nil {
			return compat.Trace{}, err
		}
		plans = append(plans, insertPlan)
		ts := esper.Field[rollupDimensionalityBean, string]("theString")
		ip := esper.Field[rollupDimensionalityBean, int]("intPrimitive")
		err = build(esper.OnEvent(esper.From[rollupDimensionalityS0](env, "SupportBean_S0")).
			SelectFromNamedWindowRollup("MyWindow", nil,
				[]esper.Expr{ts},
				esper.Alias("c0", ts),
				esper.Alias("c1", esper.Sum[int](ip)),
				esper.Alias("c2", esper.CountAll()),
			).Query(esper.StatementName(s0Name)))
		if err != nil {
			return compat.Trace{}, err
		}
	case "out-when-term-last", "out-when-term-last-opt", "out-when-term-last-optdis", "out-when-term-all", "out-when-term-snapshot":
		idField := esper.Field[rollupDimensionalityS0, int]("id")
		isStart := esper.Equal[int](idField, esper.Literal(1))
		isEnd := esper.Equal[int](idField, esper.Literal(0))
		if _, err := esper.CreateInitiatedTerminatedContext(env, "MyContext", esper.Literal("global"), isStart, isEnd); err != nil {
			return compat.Trace{}, err
		}
		var base esper.OutputPolicy
		switch caseName {
		case "out-when-term-all":
			base = esper.OutputAll()
		case "out-when-term-snapshot":
			base = esper.OutputSnapshot()
		default:
			base = esper.OutputLast()
		}
		ts := esper.Field[rollupDimensionalityBean, string]("theString")
		ip := esper.Field[rollupDimensionalityBean, int]("intPrimitive")
		err := build(esper.From[rollupDimensionalityBean](env, "SupportBean").
			GroupByRollup(ts).
			Select(
				esper.Alias("c0", ts),
				esper.Alias("c1", esper.Sum[int](ip)),
			).Query(
			esper.StatementName(s0Name),
			esper.WithContext("MyContext"),
			esper.WithOutput(esper.OutputWhenTerminated(base)),
		))
		if err != nil {
			return compat.Trace{}, err
		}
	case "bound-gs-no-top", "bound-gs-top-detail":
		ts := esper.Field[rollupDimensionalityBean, string]("theString")
		ip := esper.Field[rollupDimensionalityBean, int]("intPrimitive")
		lp := esper.Field[rollupDimensionalityBean, int64]("longPrimitive")
		var agg esper.AggregateStream
		if caseName == "bound-gs-no-top" {
			agg = esper.From[rollupDimensionalityBean](env, "SupportBean").Window(esper.LengthWindow(4)).
				GroupByGroupingSets(esper.GroupingSet(ts), esper.GroupingSet(ip))
		} else {
			agg = esper.From[rollupDimensionalityBean](env, "SupportBean").Window(esper.LengthWindow(4)).
				GroupByGroupingSets(esper.GroupingSet(), esper.GroupingSet(ts, ip))
		}
		err := build(agg.Select(
			esper.Alias("c0", ts),
			esper.Alias("c1", ip),
			esper.Alias("c2", esper.Sum[int64](lp)),
		).Query(esper.StatementName(s0Name), esper.WithOldStream()))
		if err != nil {
			return compat.Trace{}, err
		}
	case "mixed-access":
		if _, err := esper.RegisterStruct[rollupDimensionalityFullBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		fullString := esper.Field[rollupDimensionalityFullBean, *string]("theString")
		fullInt := esper.Field[rollupDimensionalityFullBean, int]("intPrimitive")
		err := build(esper.From[rollupDimensionalityFullBean](env, "SupportBean").
			Window(esper.LengthWindow(2)).
			GroupByRollup(fullString).
			Select(
				esper.Alias("c0", esper.Sum[int](fullInt)),
				esper.Alias("c1", fullString),
				esper.Alias("c2", esper.WindowEvents()),
			).Query(
			esper.StatementName(s0Name),
			esper.OrderBy(esper.Ascending(fullString)),
		))
		if err != nil {
			return compat.Trace{}, err
		}
	case "non-boxed-types":
		shortPrimitive := esper.Field[rollupDimensionalityBean, int16]("shortPrimitive")
		iF := intPrimitive()
		dF := doublePrimitive()
		lF := esper.Field[rollupDimensionalityBean, int64]("longPrimitive")
		selectColumns := []esper.Selection{
			esper.Alias("c0", iF),
			esper.Alias("c1", dF),
			esper.Alias("c2", lF),
			esper.Alias("c3", esper.Sum[int16](shortPrimitive)),
		}
		newStream := func() esper.Stream[rollupDimensionalityBean] {
			return esper.From[rollupDimensionalityBean](env, "SupportBean")
		}
		spellings := []struct {
			name string
			agg  esper.AggregateStream
		}{
			// Java s0: group by intPrimitive, rollup(doublePrimitive, longPrimitive)
			{"s0", newStream().GroupByGroupingSets(
				esper.GroupingSet(iF, dF, lF),
				esper.GroupingSet(iF, dF),
				esper.GroupingSet(iF),
			)},
			{"s1", newStream().GroupByGroupingSets(esper.GroupingSet(iF, dF, lF))},
			{"s2", newStream().GroupByGroupingSets(
				esper.GroupingSet(iF, dF, lF),
				esper.GroupingSet(iF, dF),
			)},
			{"s3", newStream().GroupByGroupingSets(
				esper.GroupingSet(dF, iF),
				esper.GroupingSet(lF, iF),
			)},
		}
		for _, spelling := range spellings {
			query := spelling.agg.Select(selectColumns...).
				Query(esper.StatementName(spelling.name))
			if err := build(query); err != nil {
				return compat.Trace{}, err
			}
		}
	case "groupby-computation":
		longF := esper.Field[rollupDimensionalityBean, int64]("longPrimitive")
		computedKey := esper.CaseWhen[int64](
			esper.Greater[int64](longF, esper.Literal[int64](0)),
			esper.Literal[int64](1),
		).Else(esper.Literal[int64](0))
		intF := esper.Field[rollupDimensionalityBean, int]("intPrimitive")
		err := build(esper.From[rollupDimensionalityBean](env, "SupportBean").
			GroupByRollup(computedKey).
			Select(
				esper.Alias("c0", longF),
				esper.Alias("c1", esper.Sum[int](intF)),
			).Query(esper.StatementName(s0Name)))
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported extended rollup-dimensionality case %q", caseName)
	}

	engine := esper.NewEngine(env)
	cleanup := true
	defer func() {
		if cleanup {
			_ = engine.Close(context.Background())
		}
	}()
	type deployedStatement struct {
		statement *esper.Statement
		schema    esper.Schema
		hasSchema bool
	}
	statements := make(map[string]*esper.Statement)
	var order []deployedStatement
	for _, plan := range plans {
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return compat.Trace{}, err
		}
		schema, schemaOK := plan.ResultSchema()
		for _, statement := range deployment.Statements() {
			statements[statement.Name()] = statement
			order = append(order, deployedStatement{statement: statement, schema: schema, hasSchema: schemaOK})
		}
	}
	primary := statements[s0Name]
	if primary == nil {
		return compat.Trace{}, fmt.Errorf("extended case %q has no %q statement", caseName, s0Name)
	}
	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	// Listener records come from the compat replay's own subscription, which
	// mirrors the oracle's per-statement sequence numbering.
	_ = primary

	handlers := map[string]compat.StepHandler{}
	if caseName == "non-boxed-types" {
		handlers["types"] = func(step compat.Step, _ func(*esper.Statement) error) ([]compat.TraceRecord, error) {
			records := make([]compat.TraceRecord, 0, len(order))
			for _, deployed := range order {
				value := map[string]any{}
				if !deployed.hasSchema {
					continue
				}
				for _, name := range []string{"c0", "c1", "c2"} {
					field, ok := deployed.schema.Field(name)
					if !ok {
						continue
					}
					if token := javaTypeName(field.Type); token != "" {
						value[name] = token
					}
				}
				records = append(records, compat.TraceRecord{
					Case:      caseName,
					Operation: "types",
					Statement: deployed.statement.Name(),
					Value:     value,
				})
			}
			return records, nil
		}
	}

	decoder := decodeRollupDimensionalityPayload
	if caseName == "mixed-access" {
		decoder = func(step compat.Step) (any, error) {
			if step.EventType == "SupportBean" {
				var bean rollupDimensionalityFullBean
				if err := json.Unmarshal(step.Payload, &bean); err != nil {
					return nil, fmt.Errorf("rollup-dimensionality SupportBean: %w", err)
				}
				// Java char primitive defaults to '\u0000'; sends never
				// override it in this scenario.
				if bean.CharPrimitive == "" {
					bean.CharPrimitive = "\u0000"
				}
				return bean, nil
			}
			return decodeRollupDimensionalityPayload(step)
		}
	}
	result, err := compat.ReplayWithStatementsAndHandlers(ctx, engine, primary, caseScenario,
		decoder,
		func(name string) (*esper.Statement, error) {
			statement, ok := statements[name]
			if !ok {
				return nil, fmt.Errorf("unknown rollup-dimensionality statement %q", name)
			}
			return statement, nil
		}, handlers)
	cleanup = false
	if err != nil {
		return trace, err
	}
	trace.Records = append(trace.Records, result.Records...)
	return trace, nil
}

// javaTypeName maps a Go output field type to the Java boxed-class simple
// name token used by the types record.
func javaTypeName(t reflect.Type) string {
	if t == nil {
		return ""
	}
	switch t.Kind() {
	case reflect.Bool:
		return "Boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return "Integer"
	case reflect.Int64, reflect.Uint64:
		return "Long"
	case reflect.Float32:
		return "Float"
	case reflect.Float64:
		return "Double"
	case reflect.String:
		return "String"
	default:
		return ""
	}
}
