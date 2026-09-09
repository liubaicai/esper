package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for ViewGroup merge-view semantics: the groupwin parent is
// a merge/union point whose downstream sees the union of all groups' subview
// contents, so an ordinary select-clause aggregate (sum(p2) with no group-by)
// stays one ungrouped aggregate over that union (Java ord 0: 10/21/33/36
// with in-group eviction subtracted), while grouped-view retention delivers
// per-group irstream pairs with in-group eviction (Java ord 14).
var viewGroupMergeViewJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewGroup.java",
}

type viewGroupMergeBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type viewGroupMergeMarket struct {
	Symbol string  `esper:"symbol"`
	Feed   string  `esper:"feed"`
	Volume float64 `esper:"volume"`
	Price  float64 `esper:"price"`
}

type viewGroupMergeTimestamp struct {
	ID        string  `esper:"id"`
	GroupID   *string `esper:"groupId"`
	Timestamp int64   `esper:"timestamp"`
}

type viewGroupMergeCorrelMarket struct {
	Symbol string  `esper:"symbol"`
	Feed   string  `esper:"feed"`
	Volume int64   `esper:"volume"`
	Price  float64 `esper:"price"`
}

var (
	viewGroupMergeViewJavaRuntimeIDs = []string{
		"java-runtime-a3b6bef89e22a122cc7a", // ViewGroupObjectArrayEvent
		"java-runtime-639bc9b69621f3b6a417", // ViewGroupLengthWin
		"java-runtime-03ed11fd1e5c3a3d11c2", // ViewGroupStats
		"java-runtime-74e476ad16bf62d4a9e0", // ViewGroupCorrel
		"java-runtime-942d359f7bb1302684bb", // ViewGroupLinest
		"java-runtime-47877af7a1d114850723", // ViewGroupMultiProperty
		"java-runtime-7bd36b6fe5567b066794", // ViewGroupTimeBatch
		"java-runtime-842bde62118b9b8cae2d", // ViewGroupTimeAccum
		"java-runtime-806120fdd2130ab1f275", // ViewGroupTimeOrder
		"java-runtime-737a5f1ffd4c6a6c8924", // ViewGroupTimeLengthBatch
		"java-runtime-68ef8076bc96595cbfd0", // ViewGroupTimeWin
		"java-runtime-88d7b731431c59d99f3a", // ViewGroupReclaimTimeWindow
		"java-runtime-afc05b1a18402bb17f56", // ViewGroupReclaimAgedHint
		"java-runtime-33b5cb01913d5d23292b", // ViewGroupReclaimWithFlipTime
		"java-runtime-563f2c37fb66e3d067ca", // ViewGroupExpressionGrouped
	}
	viewGroupMergeViewJavaExecutions = []string{
		"ViewGroupObjectArrayEvent",
		"ViewGroupLengthWin",
		"ViewGroupStats",
		"ViewGroupCorrel",
		"ViewGroupLinest",
		"ViewGroupMultiProperty",
		"ViewGroupTimeBatch",
		"ViewGroupTimeAccum",
		"ViewGroupTimeOrder",
		"ViewGroupTimeLengthBatch",
		"ViewGroupTimeWin",
		"ViewGroupReclaimTimeWindow",
		"ViewGroupReclaimAgedHint",
		"ViewGroupReclaimWithFlipTime",
		"ViewGroupExpressionGrouped",
	}
	viewGroupMergeViewCases = []string{
		"merge-view-union-aggregate",
		"length-win-groups",
		"stats-four-views",
		"correl-groups",
		"linest-groups",
		"multi-property-uni",
		"time-batch-groups",
		"time-accum-groups",
		"time-order-groups",
		"time-length-batch-groups",
		"time-win-groups",
		"reclaim-time-window",
		"reclaim-aged-hint",
		"reclaim-flip-time",
		"expression-groupwin",
	}
)

// viewGroupMergeFormatTime renders whole seconds without a fraction and
// non-zero millisecond remainders with exactly three fractional digits,
// mirroring the Java oracle's optional-section time pattern.
func viewGroupMergeFormatTime(t time.Time) string {
	t = t.UTC()
	if t.Nanosecond() == 0 {
		return t.Format("2006-01-02T15:04:05Z07:00")
	}
	return t.Format("2006-01-02T15:04:05.000Z07:00")
}

func runViewGroupMergeViewScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	// All cases use variable-length step blocks: derive each span by scanning
	// until the next case marker (the scenario protocol has no deploy ops).
	spans := make(map[string]int, len(viewGroupMergeViewCases))
	current := ""
	stepStart := 0
	for index, step := range scenario.Steps {
		if step.Op == "case" {
			if current != "" {
				spans[current] = index - stepStart
			}
			current = step.Case
			stepStart = index
		}
	}
	if current != "" {
		spans[current] = len(scenario.Steps) - stepStart
	}
	offset := 0
	for _, caseName := range viewGroupMergeViewCases {
		if spans[caseName] <= 0 {
			return compat.Trace{}, fmt.Errorf("viewgroup-merge-view case %q has no steps", caseName)
		}
	}
	for _, caseName := range viewGroupMergeViewCases {
		caseSteps := scenario.Steps[offset : offset+spans[caseName]]
		offset += spans[caseName]
		caseTrace, err := runViewGroupMergeViewCase(ctx, caseScenarioFor(scenario, caseName, caseSteps), caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("viewgroup-merge-view case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("viewgroup-merge-view scenario %q has trailing steps", scenario.ID)
	}
	return trace, nil
}

func caseScenarioFor(scenario compat.Scenario, caseName string, steps []compat.Step) compat.Scenario {
	return compat.Scenario{Version: scenario.Version, ID: scenario.ID, Steps: steps}
}

func runViewGroupMergeViewCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()

	var plans []struct {
		name     string
		plan     esper.Plan
		listener bool
	}
	build := func(name string, listener bool, query esper.Query) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		plans = append(plans, struct {
			name     string
			plan     esper.Plan
			listener bool
		}{name, plan, listener})
		return nil
	}

	switch caseName {
	case "merge-view-union-aggregate":
		// Java ord 0 registers OAEventStringInt as an object-array event type
		// {p1: String, p2: int}.
		if _, err := esper.RegisterObjectArray(env, "OAEventStringInt", []esper.FieldSpec{
			esper.FieldDef("p1", reflect.TypeOf("")),
			esper.FieldDef("p2", reflect.TypeOf(0)),
		}); err != nil {
			return compat.Trace{}, err
		}
		p1 := esper.Field[map[string]any, string]("p1")
		p2 := esper.Field[map[string]any, int]("p2")
		err := build("s0", true, esper.FromAny(env, "OAEventStringInt").Window(
			esper.GroupWindow(p1, esper.LengthWindow(2)),
		).Aggregate(
			esper.Alias("p1", p1),
			esper.Alias("sp2", esper.Sum[int](p2)),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return compat.Trace{}, err
		}
	case "length-win-groups":
		if _, err := esper.RegisterStruct[viewGroupMergeBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		theString := esper.Field[viewGroupMergeBean, string]("theString")
		intPrimitive := esper.Field[viewGroupMergeBean, int]("intPrimitive")
		err := build("s0", true, esper.Select(
			esper.From[viewGroupMergeBean](env, "SupportBean").Window(
				esper.GroupWindow(theString, esper.LengthWindow(3))),
			esper.Alias("c0", theString),
			esper.Alias("c1", intPrimitive),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
		if err != nil {
			return compat.Trace{}, err
		}
	case "stats-four-views":
		if _, err := esper.RegisterStruct[viewGroupMergeMarket](env, "SupportMarketDataBean"); err != nil {
			return compat.Trace{}, err
		}
		symbol := esper.Field[viewGroupMergeMarket, string]("symbol")
		price := esper.Field[viewGroupMergeMarket, float64]("price")
		volume := esper.Field[viewGroupMergeMarket, float64]("volume")
		// Java ord 1: four select-* statements over the uni derived-value
		// views; the Go adaptation pins the full uni column set (average,
		// datapoints, stddev, stddevpa, total, variance + symbol) via the
		// grouped aggregate accessors.
		last3 := esper.GroupWindow(symbol, esper.LengthWindow(3))
		all := esper.GroupWindow(symbol, esper.KeepAll())
		uniColumns := func(statistic esper.Expression[float64]) []esper.Selection {
			stats := esper.UnivariateStatistics[float64](statistic)
			return []esper.Selection{
				esper.Alias("average", stats.Average()),
				esper.Alias("datapoints", stats.Datapoints()),
				esper.Alias("stddev", stats.StdDev()),
				esper.Alias("stddevpa", stats.StdDevPop()),
				esper.Alias("total", stats.Total()),
				esper.Alias("variance", stats.Variance()),
				esper.Alias("symbol", symbol),
			}
		}
		if err := build("priceLast3Stats", true, esper.From[viewGroupMergeMarket](env, "SupportMarketDataBean").
			Window(last3).
			Aggregate(uniColumns(price)...).
			Query(esper.StatementName("priceLast3Stats"), esper.OrderBy(esper.Ascending(symbol)))); err != nil {
			return compat.Trace{}, err
		}
		if err := build("volumeLast3Stats", true, esper.From[viewGroupMergeMarket](env, "SupportMarketDataBean").
			Window(last3).
			Aggregate(uniColumns(volume)...).
			Query(esper.StatementName("volumeLast3Stats"), esper.OrderBy(esper.Ascending(symbol)))); err != nil {
			return compat.Trace{}, err
		}
		if err := build("priceAllStats", true, esper.From[viewGroupMergeMarket](env, "SupportMarketDataBean").
			Window(all).
			Aggregate(uniColumns(price)...).
			Query(esper.StatementName("priceAllStats"), esper.OrderBy(esper.Ascending(symbol)))); err != nil {
			return compat.Trace{}, err
		}
		if err := build("volumeAllStats", true, esper.From[viewGroupMergeMarket](env, "SupportMarketDataBean").
			Window(all).
			Aggregate(uniColumns(volume)...).
			Query(esper.StatementName("volumeAllStats"), esper.OrderBy(esper.Ascending(symbol)))); err != nil {
			return compat.Trace{}, err
		}
	case "correl-groups":
		if _, err := esper.RegisterStruct[viewGroupMergeCorrelMarket](env, "SupportMarketDataBean"); err != nil {
			return compat.Trace{}, err
		}
		symbol := esper.Field[viewGroupMergeCorrelMarket, string]("symbol")
		feed := esper.Field[viewGroupMergeCorrelMarket, string]("feed")
		err := build("s0", true, esper.From[viewGroupMergeCorrelMarket](env, "SupportMarketDataBean").
			Window(esper.GroupWindow(symbol, esper.LengthWindow(1000000))).
			Aggregate(esper.Alias("symbol", symbol),
				esper.Alias("correlation", esper.Correlation[float64, float64](
					esper.Field[viewGroupMergeCorrelMarket, float64]("price"),
					esper.Field[viewGroupMergeCorrelMarket, float64]("volume"))),
				esper.Alias("feed", feed)).
			Query(esper.StatementName("s0")))
		if err != nil {
			return compat.Trace{}, err
		}
	case "linest-groups":
		if _, err := esper.RegisterStruct[viewGroupMergeCorrelMarket](env, "SupportMarketDataBean"); err != nil {
			return compat.Trace{}, err
		}
		symbol := esper.Field[viewGroupMergeCorrelMarket, string]("symbol")
		feed := esper.Field[viewGroupMergeCorrelMarket, string]("feed")
		linest := esper.LinearRegression[float64, float64](
			esper.Field[viewGroupMergeCorrelMarket, float64]("price"),
			esper.Field[viewGroupMergeCorrelMarket, float64]("volume"))
		// Java select-* rows carry the full linest property set plus the
		// group key and the additional prop; the Go accessors compute the
		// same statistics from the group pairs.
		err := build("s0", true, esper.From[viewGroupMergeCorrelMarket](env, "SupportMarketDataBean").
			Window(esper.GroupWindow(symbol, esper.LengthWindow(1000000))).
			Aggregate(esper.Alias("symbol", symbol),
				esper.Alias("slope", linest.Slope()),
				esper.Alias("YIntercept", linest.YIntercept()),
				esper.Alias("XAverage", linest.XAverage()),
				esper.Alias("XStandardDeviationPop", linest.XStandardDeviationPop()),
				esper.Alias("XStandardDeviationSample", linest.XStandardDeviationSample()),
				esper.Alias("XSum", linest.XSum()),
				esper.Alias("XVariance", linest.XVariance()),
				esper.Alias("YAverage", linest.YAverage()),
				esper.Alias("YStandardDeviationPop", linest.YStandardDeviationPop()),
				esper.Alias("YStandardDeviationSample", linest.YStandardDeviationSample()),
				esper.Alias("YSum", linest.YSum()),
				esper.Alias("YVariance", linest.YVariance()),
				esper.Alias("sumX", linest.XSum()),
				esper.Alias("sumXSq", linest.SumXSq()),
				esper.Alias("sumXY", linest.SumXY()),
				esper.Alias("sumY", linest.YSum()),
				esper.Alias("sumYSq", linest.SumYSq()),
				esper.Alias("dataPoints", linest.DataPoints()),
				esper.Alias("n", linest.N()),
				esper.Alias("feed", feed)).
			Query(esper.StatementName("s0")))
		if err != nil {
			return compat.Trace{}, err
		}
	case "multi-property-uni":
		if _, err := esper.RegisterStruct[viewGroupMergeMarket](env, "SupportMarketDataBean"); err != nil {
			return compat.Trace{}, err
		}
		symbol := esper.Field[viewGroupMergeMarket, string]("symbol")
		feed := esper.Field[viewGroupMergeMarket, string]("feed")
		volume := esper.Field[viewGroupMergeMarket, float64]("volume")
		price := esper.Field[viewGroupMergeMarket, float64]("price")
		keys := []esper.Expr{symbol, feed, volume}
		err := build("s0", true, esper.From[viewGroupMergeMarket](env, "SupportMarketDataBean").
			Window(esper.GroupWindowKeys(keys, esper.KeepAll())).
			Aggregate(esper.Alias("size", esper.UnivariateStatistics[float64](price).Datapoints()),
				esper.Alias("symbol", symbol),
				esper.Alias("feed", feed),
				esper.Alias("volume", volume)).
			Query(esper.StatementName("s0"), esper.WithOldStream(), esper.OrderBy(
				esper.Ascending(symbol), esper.Ascending(feed), esper.Ascending(volume))))
		if err != nil {
			return compat.Trace{}, err
		}
	case "time-batch-groups", "time-accum-groups", "time-length-batch-groups", "time-win-groups":
		if _, err := esper.RegisterStruct[viewGroupMergeMarket](env, "SupportMarketDataBean"); err != nil {
			return compat.Trace{}, err
		}
		symbol := esper.Field[viewGroupMergeMarket, string]("symbol")
		var inner esper.WindowSpec
		switch caseName {
		case "time-batch-groups":
			inner = esper.TimeBatch(10 * time.Second)
		case "time-accum-groups":
			inner = esper.TimeAccum(10 * time.Second)
		case "time-length-batch-groups":
			inner = esper.TimeLengthBatch(10*time.Second, 100)
		default:
			inner = esper.TimeWindow(10 * time.Second)
		}
		err := build("s0", true, esper.From[viewGroupMergeMarket](env, "SupportMarketDataBean").
			Window(esper.GroupWindow(symbol, inner)).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
		if err != nil {
			return compat.Trace{}, err
		}
	case "time-order-groups":
		if _, err := esper.RegisterStruct[viewGroupMergeTimestamp](env, "SupportBeanTimestamp"); err != nil {
			return compat.Trace{}, err
		}
		groupID := esper.Field[viewGroupMergeTimestamp, *string]("groupId")
		ts := esper.Field[viewGroupMergeTimestamp, int64]("timestamp")
		err := build("s0", true, esper.From[viewGroupMergeTimestamp](env, "SupportBeanTimestamp").
			Window(esper.GroupWindow(groupID, esper.TimeOrder(ts, 10*time.Second))).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
		if err != nil {
			return compat.Trace{}, err
		}
	case "reclaim-time-window":
		if _, err := esper.RegisterStruct[viewGroupMergeBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		agedHint, hintErr := esper.NewStatementHint(esper.HintReclaimGroupAged, "30")
		if hintErr != nil {
			return compat.Trace{}, hintErr
		}
		freqHint, hintErr := esper.NewStatementHint(esper.HintReclaimGroupFreq, "5")
		if hintErr != nil {
			return compat.Trace{}, hintErr
		}
		hints := []esper.StatementHint{agedHint, freqHint}
		err := build("s0", false, esper.From[viewGroupMergeBean](env, "SupportBean").
			Window(esper.GroupWindow(
				esper.Field[viewGroupMergeBean, string]("theString"),
				esper.TimeWindow(3000000*time.Millisecond))).
			Aggregate(
				esper.Alias("longPrimitive", esper.Field[viewGroupMergeBean, int64]("longPrimitive")),
				esper.Alias("cnt", esper.CountAll()),
			).
			Query(esper.StatementName("s0"), esper.WithStatementHints(hints...)))
		if err != nil {
			return compat.Trace{}, err
		}
	case "expression-groupwin":
		if _, err := esper.RegisterStruct[viewGroupMergeTimestamp](env, "SupportBeanTimestamp"); err != nil {
			return compat.Trace{}, err
		}
		ts := esper.Field[viewGroupMergeTimestamp, int64]("timestamp")
		// Day-of-week over the epoch-milli timestamp — the groupwin key
		// groups all three Tuesday events into one shared length(2) window.
		// The mapping is injective per day (the key value never surfaces in
		// select * rows), so any consistent day-numbering reproduces the
		// observable grouping.
		dow := esper.Func1[int64]("getDayOfWeek", func(ms int64) int64 {
			t := time.Unix(ms/1000, (ms%1000)*int64(time.Millisecond)).UTC()
			return int64((int(t.Weekday())+1)%7 + 1)
		}, ts)
		err := build("s0", true, esper.From[viewGroupMergeTimestamp](env, "SupportBeanTimestamp").
			Window(esper.GroupWindow(dow, esper.LengthWindow(2))).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
		if err != nil {
			return compat.Trace{}, err
		}
	case "reclaim-aged-hint", "reclaim-flip-time":
		if _, err := esper.RegisterStruct[viewGroupMergeBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		aged, freq := "5", "1"
		if caseName == "reclaim-flip-time" {
			aged, freq = "1", "5"
		}
		agedHint, hintErr := esper.NewStatementHint(esper.HintReclaimGroupAged, aged)
		if hintErr != nil {
			return compat.Trace{}, hintErr
		}
		freqHint, hintErr := esper.NewStatementHint(esper.HintReclaimGroupFreq, freq)
		if hintErr != nil {
			return compat.Trace{}, hintErr
		}
		hints := []esper.StatementHint{agedHint, freqHint}
		err := build("s0", false, esper.From[viewGroupMergeBean](env, "SupportBean").
			Window(esper.GroupWindow(
				esper.Field[viewGroupMergeBean, string]("theString"),
				esper.KeepAll())).
			Query(esper.StatementName("s0"), esper.WithStatementHints(hints...)))
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported viewgroup-merge-view case %q", caseName)
	}

	engine := esper.NewEngine(env, esper.WithRuntimeURI(
		viewGroupMergeViewJavaRuntimeIDs[indexOfCase(viewGroupMergeViewCases, caseName)]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	// Per-statement sequence counters mirror the Java oracle: each named
	// statement counts its own delivered batches from 1.
	seqByStmt := make(map[string]uint64)
	var deployments []*esper.Deployment
	statements := make(map[string]*esper.Statement)
	for _, item := range plans {
		deployment, err := engine.Deploy(ctx, item.plan)
		if err != nil {
			return trace, err
		}
		deployments = append(deployments, deployment)
		for _, st := range deployment.Statements() {
			statements[st.Name()] = st
		}
		if !item.listener {
			continue
		}
		st := deployment.Statements()[0]
		if _, err := st.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			if len(batch.New) == 0 && len(batch.Old) == 0 {
				return nil
			}
			seqByStmt[st.Name()]++
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "listener",
				Statement: st.Name(),
				Time:      viewGroupMergeFormatTime(batch.Time),
				Sequence:  seqByStmt[st.Name()],
			}
			record.New = compat.NormalizeResults(batch.New)
			record.Old = compat.NormalizeResults(batch.Old)
			trace.Records = append(trace.Records, record)
			return nil
		}); err != nil {
			return trace, err
		}
	}

	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return trace, fmt.Errorf("viewgroup-merge-view decode %s: %w", step.EventType, err)
			}
			var underlying any = payload
			switch step.EventType {
			case "OAEventStringInt":
				// Object-array events send positional underlyings.
				underlying = []any{
					payload["p1"],
					int(payload["p2"].(float64)),
				}
			case "SupportBeanTimestamp":
				tsEvent := viewGroupMergeTimestamp{ID: payload["id"].(string)}
				if v, ok := payload["groupId"].(string); ok {
					tsEvent.GroupID = &v
				}
				if v, ok := payload["timestamp"].(float64); ok {
					tsEvent.Timestamp = int64(v)
				}
				underlying = tsEvent
			case "SupportBean":
				underlying = viewGroupMergeBean{
					TheString:    payload["theString"].(string),
					IntPrimitive: int(payload["intPrimitive"].(float64)),
				}
			}
			if err := engine.Send(ctx, step.EventType, underlying); err != nil {
				return trace, err
			}
		case "undeploy-all":
			for _, deployment := range deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return trace, err
				}
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return trace, fmt.Errorf("viewgroup-merge-view decode advance-time: %w", err)
			}
			if err := engine.AdvanceTime(ctx, at.UTC()); err != nil {
				return trace, err
			}
		case "schedule-count", "schedule-count-overall", "iterator-count":
			var count int64
			var err error
			if step.Op == "schedule-count" {
				st, ok := statements[step.Statement]
				if !ok {
					return trace, fmt.Errorf("schedule-count statement %q not found", step.Statement)
				}
				var n int
				n, err = st.ScheduleCount(ctx)
				count = int64(n)
			} else if step.Op == "schedule-count-overall" {
				var n int
				n, err = engine.ScheduleCountOverall(ctx)
				count = int64(n)
			} else {
				st, ok := statements[step.Statement]
				if !ok {
					return trace, fmt.Errorf("iterator-count statement %q not found", step.Statement)
				}
				result, snapErr := st.Snapshot(ctx)
				if snapErr != nil {
					return trace, snapErr
				}
				count = int64(len(result.Batch.New))
			}
			if err != nil {
				return trace, err
			}
			pinned := count
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: step.Op,
				Statement: step.Statement,
				Count:     &pinned,
			}
			trace.Records = append(trace.Records, record)
		case "snapshot":
			st, ok := statements[step.Statement]
			if !ok {
				return trace, fmt.Errorf("snapshot statement %q not found", step.Statement)
			}
			result, snapErr := st.Snapshot(ctx)
			if snapErr != nil {
				return trace, snapErr
			}
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: step.Statement,
			}
			record.New = compat.NormalizeResults(result.Batch.New)
			if record.New == nil {
				record.New = []compat.ResultRecord{}
			}
			trace.Records = append(trace.Records, record)
		default:
			return trace, fmt.Errorf("unsupported viewgroup-merge-view step op %q", step.Op)
		}
	}
	return trace, nil
}

func indexOfCase(cases []string, wanted string) int {
	for index, name := range cases {
		if name == wanted {
			return index
		}
	}
	return -1
}
