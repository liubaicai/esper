package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type subselectFilteredBean struct {
	TheString    string  `esper:"theString"`
	IntPrimitive int     `esper:"intPrimitive"`
	IntBoxed     int     `esper:"intBoxed"`
	LongBoxed    int64   `esper:"longBoxed"`
	DoubleBoxed  float64 `esper:"doubleBoxed"`
}

type subselectFilteredS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type subselectFilteredS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 *string `esper:"p11"`
}

type subselectFilteredS2 struct {
	ID  int    `esper:"id"`
	P20 string `esper:"p20"`
}

type subselectFilteredS3 struct {
	ID  int    `esper:"id"`
	P30 string `esper:"p30"`
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

type subselectFilteredMarketData struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
}

type subselectFilteredSensorEvent struct {
	ID          int     `esper:"id"`
	Type        string  `esper:"type"`
	Device      string  `esper:"device"`
	Measurement float64 `esper:"measurement"`
	Confidence  float64 `esper:"confidence"`
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
		"java-runtime-fd19463d0ce39b10f445",
		"java-runtime-66c3d4d4100cb3f923a0",
		"java-runtime-7474133de58ff180f8fa",
		"java-runtime-0a6686ce79af715278f7",
		"java-runtime-78f47a5bcdf6b87c9e68",
		"java-runtime-b1c2169572264f9fd646",
		"java-runtime-87fd9e194fcd61e0aba5",
		"java-runtime-4ec1d23f545746f81f20",
		"java-runtime-0626e0d5b878384ba989",
		"java-runtime-7705828482936493f505",
		"java-runtime-7e0f728f635710446f39",
		"java-runtime-b3ce74a1b603f1dfa644",
		"java-runtime-21f4b30723f3e5256823",
		"java-runtime-71f4714a3f10241083e9",
		"java-runtime-b2be42620bdc328d006e",
		"java-runtime-1fcd5cdcc62015389f98",
		"java-runtime-9161a695d692feb6902d",
		"java-runtime-1072ab9fc42579eed1e5",
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
		"EPLSubselectSelectWhereJoined4Coercion",
		"EPLSubselectSelectWhereJoined4BackCoercion",
		"EPLSubselectJoinFilteredOne",
		"EPLSubselectJoinFilteredTwo",
		"EPLSubselectWherePrevious",
		"EPLSubselectWherePreviousOM",
		"EPLSubselectWherePreviousCompile",
		"EPLSubselectSameEvent",
		"EPLSubselectSameEventOM",
		"EPLSubselectSameEventCompile",
		"EPLSubselectSelectWildcard",
		"EPLSubselectSelectWhereJoined2Streams",
		"EPLSubselectSelectWhereJoined3Streams",
		"EPLSubselectSelectWhereJoined3SceneTwo",
		"EPLSubselectSelectSceneOne",
		"EPLSubselectSelectWithWhere2Subqery",
		"EPLSubselectSubselectMixMax",
		"EPLSubselectSubselectPrior",
	}
)

// runSubselectFilteredScenario replays the scalar-filter, multikey-wArray,
// joined numeric-coercion, join-filtered, where-previous, wildcard
// event-subquery, multi-stream join and suite-finale slices of
// EPLSubselectFiltered (26 executions across 31 scenario cases;
// WhereConstant contributes three single-deployment cases, each Joined4
// coercion statement is its own predicate-ordering case, the WherePrevious
// triple replays one shared sequence per compilation path, the SameEvent
// triple does the same for the self-stream wildcard column, and the two
// three-stream join cases contrast partial versus full correlation over
// inputs differing only in the final scene-two round): non-aggregated having
// row filters, constant and correlated where predicates,
// null-on-empty/null-on-multiple scalar subselect boundaries, int[]
// content-equality correlation keys, cross-stream boxed numeric coercion
// over a three-way keepall join, two-stream joins gated by scalar/boolean
// subqueries with prior/prev projections, prev over a gated subquery's
// unfiltered window, single-event wildcard columns observing trigger and
// cross-stream event references, multi-stream keepall joins driving an S0
// correlated subquery, irstream dual projection with per-row subquery
// re-evaluation on eviction, OR-gated dual correlated subqueries, twin sort
// windows projecting high/low event rows, and a three-statement insert-into
// chain with coalesce dedupe gates.
func runSubselectFilteredScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		"having-no-filter-no-where", "having-w-where", "having-w-filter-w-where",
		"where-constant-single-column", "where-constant-two-column", "where-constant-range",
		"select-with-where-joined",
		"multikey-array-primitive", "multikey-array-two-field", "multikey-array-composite",
		"joined-4-coercion-p1", "joined-4-coercion-p2", "joined-4-coercion-p3",
		"joined-4-back-coercion-p1", "joined-4-back-coercion-p2",
		"join-filtered-one", "join-filtered-two",
		"where-previous", "where-previous-om", "where-previous-compile",
		"same-event", "same-event-om", "same-event-compile", "select-wildcard",
		"joined-2-streams", "joined-3-streams", "joined-3-scene-two",
		"select-scene-one", "where-2-subquery", "subselect-mix-max", "subselect-prior",
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
		func() error { _, err := esper.RegisterStruct[subselectFilteredS2](env, "SupportBean_S2"); return err },
		func() error { _, err := esper.RegisterStruct[subselectFilteredS3](env, "SupportBean_S3"); return err },
		func() error {
			_, err := esper.RegisterStruct[subselectFilteredMarketData](env, "SupportMarketDataBean")
			return err
		},
		func() error {
			_, err := esper.RegisterStruct[subselectFilteredSensorEvent](env, "SupportSensorEvent")
			return err
		},
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
	case "joined-4-coercion-p1", "joined-4-coercion-p2", "joined-4-coercion-p3",
		"joined-4-back-coercion-p1", "joined-4-back-coercion-p2":
		coercionInner := esper.From[subselectFilteredBean](env, "SupportBean").
			Filter(esper.Equal[string](esper.Field[subselectFilteredBean, string]("theString"), esper.Literal("S"))).
			Window(esper.LengthWindow(1000)).
			AsRecord()
		filtered := func(name string) esper.Stream[subselectFilteredBean] {
			return esper.From[subselectFilteredBean](env, "SupportBean").
				Filter(esper.Equal[string](esper.Field[subselectFilteredBean, string]("theString"), esper.Literal(name)))
		}
		intBoxed := esper.Field[any, any]("intBoxed")
		longBoxed := esper.Field[any, any]("longBoxed")
		doubleBoxed := esper.Field[any, any]("doubleBoxed")
		var predicate esper.Expression[bool]
		switch caseName {
		case "joined-4-coercion-p1":
			predicate = esper.And(esper.EqualOf(intBoxed, esper.JoinField[any](0, "longBoxed")),
				esper.And(esper.EqualOf(intBoxed, esper.JoinField[any](1, "doubleBoxed")),
					esper.EqualOf(doubleBoxed, esper.JoinField[any](2, "intBoxed"))))
		case "joined-4-coercion-p2":
			predicate = esper.And(esper.EqualOf(doubleBoxed, esper.JoinField[any](2, "intBoxed")),
				esper.And(esper.EqualOf(intBoxed, esper.JoinField[any](1, "doubleBoxed")),
					esper.EqualOf(intBoxed, esper.JoinField[any](0, "longBoxed"))))
		case "joined-4-coercion-p3":
			predicate = esper.And(esper.EqualOf(doubleBoxed, esper.JoinField[any](2, "intBoxed")),
				esper.And(esper.EqualOf(intBoxed, esper.JoinField[any](0, "longBoxed")),
					esper.EqualOf(intBoxed, esper.JoinField[any](1, "doubleBoxed"))))
		case "joined-4-back-coercion-p1":
			predicate = esper.And(esper.EqualOf(longBoxed, esper.JoinField[any](0, "intBoxed")),
				esper.And(esper.EqualOf(longBoxed, esper.JoinField[any](1, "doubleBoxed")),
					esper.EqualOf(intBoxed, esper.JoinField[any](2, "longBoxed"))))
		default:
			predicate = esper.And(esper.EqualOf(longBoxed, esper.JoinField[any](1, "doubleBoxed")),
				esper.And(esper.EqualOf(intBoxed, esper.JoinField[any](2, "longBoxed")),
					esper.EqualOf(longBoxed, esper.JoinField[any](0, "intBoxed"))))
		}
		query = esper.JoinMany(
			esper.JoinSource(filtered("A").Window(esper.KeepAll())),
			esper.JoinSource(filtered("B").Window(esper.KeepAll())),
			esper.JoinSource(filtered("C").Window(esper.KeepAll())),
		).On(
			esper.OnSourcesEqual(0, esper.Field[subselectFilteredBean, int]("intPrimitive"), 1, esper.Field[subselectFilteredBean, int]("intPrimitive")),
			esper.OnSourcesEqual(1, esper.Field[subselectFilteredBean, int]("intPrimitive"), 2, esper.Field[subselectFilteredBean, int]("intPrimitive")),
		).Select(
			esper.SelectLeft("ids0", esper.SubqueryValue[int](coercionInner, esper.Field[any, int]("intPrimitive"), predicate)),
		).Query(esper.StatementName("s0"))
	case "join-filtered-one", "join-filtered-two":
		innerLong := esper.From[subselectFilteredS2](env, "SupportBean_S2").Window(esper.LengthWindow(1000)).AsRecord()
		innerShort := func() esper.RecordStream {
			return esper.From[subselectFilteredS2](env, "SupportBean_S2").Window(esper.LengthWindow(10)).AsRecord()
		}
		correlated := esper.Equal[int](esper.Field[any, int]("id"), esper.JoinField[int](0, "id"))
		build := func(where esper.Expression[bool]) esper.Query {
			return esper.Join(
				esper.From[subselectFilteredS0](env, "SupportBean_S0").Window(esper.KeepAll()),
				esper.From[subselectFilteredS1](env, "SupportBean_S1").Window(esper.KeepAll()),
				esper.OnEqual(
					esper.Field[subselectFilteredS0, int]("id"),
					esper.Field[subselectFilteredS1, int]("id"),
				),
			).Select(
				esper.SelectLeft("s0id", esper.JoinField[int](0, "id")),
				esper.SelectRight("s1id", esper.JoinField[int](1, "id")),
				esper.SelectLeft("s2p20", esper.SubqueryValue[string](innerLong, esper.Field[any, string]("p20"), correlated)),
				esper.SelectLeft("s2p20Prior", esper.SubqueryValue[string](innerLong, esper.Prior[string](0, esper.Field[any, string]("p20")), correlated)),
				esper.SelectLeft("s2p20Prev", esper.SubqueryValue[string](innerShort(),
					esper.Prev[string](1, esper.Field[any, string]("p20")), correlated)),
			).Where(where).Query(esper.StatementName("s0"))
		}
		if caseName == "join-filtered-one" {
			query = build(esper.EqualOf(
				esper.ConcatOf(esper.JoinField[any](0, "p00"), esper.JoinField[any](1, "p10")),
				esper.SubqueryValue[string](innerLong, esper.Field[any, string]("p20"), correlated),
			))
		} else {
			query = build(esper.SubqueryValue[bool](innerLong,
				esper.EqualOf(
					esper.ConcatOf(esper.JoinField[any](0, "p00"), esper.JoinField[any](1, "p10")),
					esper.Field[any, string]("p20"),
				),
				correlated,
			))
		}
	case "where-previous", "where-previous-om", "where-previous-compile":
		query = esper.Select(outer,
			esper.Alias("value", esper.SubqueryValue[int](s1Inner(),
				esper.Prev[int](1, esper.Field[any, int]("id")),
				esper.Equal[int](esper.Field[any, int]("id"), esper.OuterField[int]("id"))),
			)).Query(esper.StatementName("s0"))
	case "same-event", "same-event-om", "same-event-compile":
		query = esper.Select(
			esper.From[subselectFilteredS1](env, "SupportBean_S1"),
			esper.Alias("events1", esper.SubqueryValue[esper.Event](s1Inner(), esper.EventValue[esper.Event]())),
		).Query(esper.StatementName("s0"))
	case "select-wildcard":
		query = esper.Select(outer,
			esper.Alias("events1", esper.SubqueryValue[esper.Event](s1Inner(), esper.EventValue[esper.Event]())),
		).Query(esper.StatementName("s0"))
	case "joined-2-streams":
		s0Inner := func() esper.RecordStream {
			return esper.From[subselectFilteredS0](env, "SupportBean_S0").Window(esper.LengthWindow(1000)).AsRecord()
		}
		query = esper.Join(
			esper.From[subselectFilteredS1](env, "SupportBean_S1").Window(esper.KeepAll()),
			esper.From[subselectFilteredS2](env, "SupportBean_S2").Window(esper.KeepAll()),
			esper.OnEqual(
				esper.Field[subselectFilteredS1, int]("id"),
				esper.Field[subselectFilteredS2, int]("id"),
			),
		).Select(
			esper.SelectLeft("ids0", esper.SubqueryValue[int](s0Inner(),
				esper.Field[any, int]("id"),
				esper.And(
					esper.EqualOf(esper.Field[any, string]("p00"), esper.JoinField[any](0, "p10")),
					esper.EqualOf(esper.Field[any, string]("p00"), esper.JoinField[any](1, "p20")),
				),
			)),
		).Query(esper.StatementName("s0"))
	case "joined-3-streams", "joined-3-scene-two":
		s0Inner := func() esper.RecordStream {
			return esper.From[subselectFilteredS0](env, "SupportBean_S0").Window(esper.LengthWindow(1000)).AsRecord()
		}
		p00 := esper.Field[any, string]("p00")
		var predicate esper.Expression[bool]
		if caseName == "joined-3-streams" {
			predicate = esper.And(esper.EqualOf(p00, esper.JoinField[any](0, "p10")),
				esper.EqualOf(p00, esper.JoinField[any](2, "p30")))
		} else {
			predicate = esper.And(esper.EqualOf(p00, esper.JoinField[any](0, "p10")),
				esper.And(esper.EqualOf(p00, esper.JoinField[any](2, "p30")),
					esper.EqualOf(p00, esper.JoinField[any](1, "p20"))))
		}
		query = esper.JoinMany(
			esper.JoinSource(esper.From[subselectFilteredS1](env, "SupportBean_S1").Window(esper.KeepAll())),
			esper.JoinSource(esper.From[subselectFilteredS2](env, "SupportBean_S2").Window(esper.KeepAll())),
			esper.JoinSource(esper.From[subselectFilteredS3](env, "SupportBean_S3").Window(esper.KeepAll())),
		).On(
			esper.OnSourcesEqual(0, esper.Field[subselectFilteredS1, int]("id"), 1, esper.Field[subselectFilteredS2, int]("id")),
			esper.OnSourcesEqual(1, esper.Field[subselectFilteredS2, int]("id"), 2, esper.Field[subselectFilteredS3, int]("id")),
		).Select(
			esper.SelectLeft("ids0", esper.SubqueryValue[int](s0Inner(),
				esper.Field[any, int]("id"), predicate)),
		).Query(esper.StatementName("s0"))
	case "select-scene-one":
		mdbOuter := esper.From[subselectFilteredMarketData](env, "SupportMarketDataBean").
			Filter(esper.Equal[string](esper.Field[subselectFilteredMarketData, string]("symbol"), esper.Literal("S0"))).
			Window(esper.LengthWindow(2))
		mdbInner := esper.From[subselectFilteredMarketData](env, "SupportMarketDataBean").
			Filter(esper.Equal[string](esper.Field[subselectFilteredMarketData, string]("symbol"), esper.Literal("S1"))).
			Window(esper.LengthWindow(10)).
			AsRecord()
		query = esper.Select(mdbOuter,
			esper.Alias("s0price", esper.Field[subselectFilteredMarketData, float64]("price")),
			esper.Alias("s1price", esper.SubqueryValue[float64](mdbInner,
				esper.Field[any, float64]("price"),
				esper.Equal[float64](
					esper.Field[subselectFilteredMarketData, float64]("volume"),
					esper.OuterField[float64]("volume")))),
		).Query(esper.StatementName("s0"), esper.WithOldStream())
	case "where-2-subquery":
		s0ID := esper.Field[subselectFilteredS0, int]("id")
		s1Inner := func() esper.RecordStream {
			return esper.From[subselectFilteredS1](env, "SupportBean_S1").Window(esper.LengthWindow(1000)).AsRecord()
		}
		s2Inner := func() esper.RecordStream {
			return esper.From[subselectFilteredS2](env, "SupportBean_S2").Window(esper.LengthWindow(1000)).AsRecord()
		}
		query = esper.Select(
			esper.From[subselectFilteredS0](env, "SupportBean_S0").Filter(esper.Or(
				esper.Equal[int](s0ID, esper.SubqueryValue[int](s1Inner(),
					esper.Field[any, int]("id"),
					esper.Equal[int](esper.Field[any, int]("id"), esper.OuterField[int]("id")))),
				esper.Equal[int](s0ID, esper.SubqueryValue[int](s2Inner(),
					esper.Field[any, int]("id"),
					esper.Equal[int](esper.Field[any, int]("id"), esper.OuterField[int]("id")))),
			)),
			esper.Alias("id", s0ID),
		).Query(esper.StatementName("s0"))
	case "subselect-mix-max":
		highInner := esper.From[subselectFilteredSensorEvent](env, "SupportSensorEvent").
			Window(esper.SortWindow(1, esper.Descending(esper.Field[subselectFilteredSensorEvent, float64]("measurement")))).AsRecord()
		lowInner := esper.From[subselectFilteredSensorEvent](env, "SupportSensorEvent").
			Window(esper.SortWindow(1, esper.Ascending(esper.Field[subselectFilteredSensorEvent, float64]("measurement")))).AsRecord()
		query = esper.Select(
			esper.From[subselectFilteredSensorEvent](env, "SupportSensorEvent"),
			esper.Alias("high", esper.SubqueryValue[esper.Event](highInner, esper.EventValue[esper.Event]())),
			esper.Alias("low", esper.SubqueryValue[esper.Event](lowInner, esper.EventValue[esper.Event]())),
		).Query(esper.StatementName("s0"))
	case "subselect-prior":
		pairFields := []esper.FieldSpec{
			{Name: "a", Type: reflect.TypeOf(esper.Event{})},
			{Name: "b", Type: reflect.TypeOf(esper.Event{})},
		}
		if _, err := esper.RegisterMap(env, "Pair", pairFields); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.RegisterMap(env, "PairDuplicatesRemoved", pairFields); err != nil {
			return compat.Trace{}, err
		}
		device := esper.Field[subselectFilteredSensorEvent, string]("device")
		sensorType := esper.Field[subselectFilteredSensorEvent, string]("type")
		pairQuery := esper.Join(
			esper.From[subselectFilteredSensorEvent](env, "SupportSensorEvent").Filter(esper.Equal[string](device, esper.Literal("A"))).Window(esper.LastEvent()),
			esper.From[subselectFilteredSensorEvent](env, "SupportSensorEvent").Filter(esper.Equal[string](device, esper.Literal("B"))).Window(esper.LastEvent()),
			esper.OnEqual(sensorType, sensorType),
		).Select(
			esper.SelectLeft("a", esper.JoinEventValue[esper.Event](0)),
			esper.SelectRight("b", esper.JoinEventValue[esper.Event](1)),
		).InsertInto("Pair", esper.StatementName("pair"))
		seedQuery := esper.FromAny(env, "Pair").Filter(esper.Literal(false)).Select(
			esper.Alias("a", esper.Field[any, esper.Event]("a")),
			esper.Alias("b", esper.Field[any, esper.Event]("b")),
		).InsertInto("PairDuplicatesRemoved", esper.StatementName("pair-duplicates-removed-seed"))
		pdrLast := esper.FromAny(env, "PairDuplicatesRemoved").Window(esper.LastEvent())
		aID := esper.NestedField[int](esper.Field[any, esper.Event]("a"), "id")
		bID := esper.NestedField[int](esper.Field[any, esper.Event]("b"), "id")
		s0Query := esper.FromAny(env, "Pair").Filter(esper.And(
			esper.NotEqual[int](aID, esper.Coalesce[int](esper.SubqueryValue[int](pdrLast, aID), esper.Literal(-1))),
			esper.NotEqual[int](bID, esper.Coalesce[int](esper.SubqueryValue[int](pdrLast, bID), esper.Literal(-1))),
		)).Select(
			esper.Alias("a", esper.Field[any, esper.Event]("a")),
			esper.Alias("b", esper.Field[any, esper.Event]("b")),
		).InsertInto("PairDuplicatesRemoved", esper.StatementName("s0"))
		var plans []esper.Plan
		for _, q := range []esper.Query{pairQuery, seedQuery, s0Query} {
			plan, err := env.Build(q)
			if err != nil {
				return compat.Trace{}, err
			}
			plans = append(plans, plan)
		}
		engine := esper.NewEngine(env)
		var s0Statement *esper.Statement
		for index, plan := range plans {
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return compat.Trace{}, err
			}
			if index == len(plans)-1 {
				s0Statement = deployment.Statements()[0]
			}
		}
		return compat.ReplayWithStatements(ctx, engine, s0Statement, caseScenario, decodeSubselectFilteredPayload, func(name string) (*esper.Statement, error) {
			if name != s0Statement.Name() {
				return nil, fmt.Errorf("unknown subselect-filtered statement %q", name)
			}
			return s0Statement, nil
		})
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
	case "SupportBean_S2":
		var event subselectFilteredS2
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("subselect-filtered SupportBean_S2: %w", err)
		}
		return event, nil
	case "SupportBean_S3":
		var event subselectFilteredS3
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("subselect-filtered SupportBean_S3: %w", err)
		}
		return event, nil
	case "SupportMarketDataBean":
		var event subselectFilteredMarketData
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("subselect-filtered SupportMarketDataBean: %w", err)
		}
		return event, nil
	case "SupportSensorEvent":
		var event subselectFilteredSensorEvent
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("subselect-filtered SupportSensorEvent: %w", err)
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
