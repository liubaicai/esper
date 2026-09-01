package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetQueryTypeRowPerEventMarketData struct {
	Symbol *string `esper:"symbol"`
	ID     *string `esper:"id"`
	Price  float64 `esper:"price"`
	Volume *int64  `esper:"volume"`
	Feed   *string `esper:"feed"`
}

type resultsetQueryTypeRowPerEventBean struct {
	TheString       string   `esper:"theString"`
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

type resultsetQueryTypeRowPerEventBeanString struct {
	TheString string `esper:"theString"`
}

type resultsetQueryTypeRowPerEventS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

const resultsetQueryTypeRowPerEventID = "resultset-querytype-row-per-event"

const resultsetQueryTypeRowPerEventDescription = "Row-per-event result sets replaying ResultSetQueryTypeRowPerEvent executions over SupportBean/SupportBeanString/SupportMarketDataBean/SupportBean_S0: irstream sum with evicted-row old columns carrying post-eviction sums (view and join twins), window(s0.*)+trigger-bean join rows, ESPER-571 ungrouped max having against an unaggregated current-event property, sum/avg with a pre-view where filter, sum(distinct) over a length(3) window and avg/count(distinct) over an unbounded stream with null old-side rendering"

const resultsetQueryTypeRowPerEventJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetQueryTypeRowPerEventJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRowPerEvent.java",
}

// Case order fixes the runtime-ID index mapping below. The seven executions
// share one Java class, one scenario, one oracle and one runner surface, so
// the unit ports them together (the ECSM multi-runtime precedent).
var resultsetQueryTypeRowPerEventJavaRuntimeIDs = []string{
	"java-runtime-5111b05c6bc620b88e15",
	"java-runtime-50601d6f0cc0411a9f90",
	"java-runtime-06c962063c57e3a3adee",
	"java-runtime-14d3e2b22e8c3ef00657",
	"java-runtime-1f1dae3e5953610a77e3",
	"java-runtime-9160c96fccf23486d782",
	"java-runtime-cce782d69a46b20b8609",
}

var resultsetQueryTypeRowPerEventJavaExecutions = []string{
	"ResultSetQueryTypeRowPerEventSumOneView",
	"ResultSetQueryTypeRowPerEventSumJoin",
	"ResultSetQueryTypeAggregatedSelectTriggerEvent",
	"ResultSetQueryTypeAggregatedSelectUnaggregatedHaving",
	"ResultSetQueryTypeSumAvgWithWhere",
	"ResultSetQueryTypeRowPerEventDistinct",
	"ResultSetQueryTypeRowPerEventDistinctNullable",
}

var resultsetQueryTypeRowPerEventCases = []string{
	"sum-one-view",
	"sum-join",
	"trigger-event",
	"unagg-having",
	"sum-avg-where",
	"distinct-sum",
	"distinct-nullable",
}

// runResultsetQueryTypeRowPerEventScenario replays the seven row-per-event
// executions. Java semantics pinned by the oracle: old rows in the irstream
// length(3) sum executions carry the EVICTED row's own columns with the
// POST-eviction aggregate (Java computes old select rows after applyLeave on
// the same update), the where clause filters events before window retention
// (filtered events never evict), the ungrouped having binds the current
// event for its unaggregated property (ESPER-571), and unbounded-istream
// aggregate old rows render the previous-state projection (avg null / count 0
// on the first pair).
func runResultsetQueryTypeRowPerEventScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultsetQueryTypeRowPerEventCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultsetQueryTypeRowPerEventCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset querytype row-per-event case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("resultset querytype row-per-event scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runResultsetQueryTypeRowPerEventCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetQueryTypeRowPerEventBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetQueryTypeRowPerEventMarketData](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	needsString := caseName == "sum-join"
	needsS0 := caseName == "trigger-event"
	if needsString {
		if _, err := esper.RegisterStruct[resultsetQueryTypeRowPerEventBeanString](env, "SupportBeanString"); err != nil {
			return compat.Trace{}, err
		}
	}
	if needsS0 {
		if _, err := esper.RegisterStruct[resultsetQueryTypeRowPerEventS0](env, "SupportBean_S0"); err != nil {
			return compat.Trace{}, err
		}
	}

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(ctx) }()

	longBoxed := esper.Cast[*int64, int64](esper.Field[resultsetQueryTypeRowPerEventBean, *int64]("longBoxed"))
	longPrimitive := esper.Field[resultsetQueryTypeRowPerEventBean, int64]("longPrimitive")
	volume := esper.Cast[*int64, int64](esper.Field[resultsetQueryTypeRowPerEventMarketData, *int64]("volume"))
	symbol := esper.Field[resultsetQueryTypeRowPerEventMarketData, *string]("symbol")
	symbolValue := esper.Cast[*string, string](symbol)

	var plan esper.Plan
	var planErr error
	switch caseName {
	case "sum-one-view", "sum-join":
		// Java: select irstream longPrimitive, sum(longBoxed) as mySum from
		// SupportBean#length(3) [joined with SupportBeanString#length(3) on
		// theString]. Old rows carry the evicted row's own longPrimitive with
		// the post-eviction sum.
		if caseName == "sum-one-view" {
			plan, planErr = env.Build(esper.From[resultsetQueryTypeRowPerEventBean](env, "SupportBean").
				Window(esper.LengthWindow(3)).
				Aggregate(
					esper.Alias("longPrimitive", longPrimitive),
					esper.Alias("mySum", esper.Sum[int64](longBoxed)),
				).
				Query(esper.StatementName("s0"), esper.WithOldStream()))
		} else {
			plan, planErr = env.Build(esper.Join(
				esper.From[resultsetQueryTypeRowPerEventBeanString](env, "SupportBeanString").Window(esper.LengthWindow(3)),
				esper.From[resultsetQueryTypeRowPerEventBean](env, "SupportBean").Window(esper.LengthWindow(3)),
				esper.OnEqual(
					esper.JoinField[string](0, "theString"),
					esper.JoinField[string](1, "theString"),
				),
			).Aggregate(
				esper.Alias("longPrimitive", esper.JoinField[int64](1, "longPrimitive")),
				esper.Alias("mySum", esper.Sum[int64](esper.Cast[*int64, int64](esper.JoinField[*int64](1, "longBoxed")))),
			).Query(esper.StatementName("s0"), esper.WithOldStream()))
		}
	case "trigger-event":
		// Java: select window(s0.*) as rows, sb from SupportBean#keepall as
		// sb, SupportBean_S0#keepall as s0 where sb.theString = s0.p00.
		// window() is an access aggregate: one row per join-matching trigger
		// event, rows carrying the full s0 window snapshot, sb the trigger
		// bean (window-latest on s0-direction arrivals).
		plan, planErr = env.Build(esper.Join(
			esper.From[resultsetQueryTypeRowPerEventBean](env, "SupportBean").Window(esper.KeepAll()),
			esper.From[resultsetQueryTypeRowPerEventS0](env, "SupportBean_S0").Window(esper.KeepAll()),
			esper.OnEqual(
				esper.JoinField[string](0, "theString"),
				esper.JoinField[string](1, "p00"),
			),
		).Aggregate(
			// The sb column is the trigger event itself; the Event-typed
			// value normalizes as a row with the full schema field map
			// (Java renders the underlying bean's EventBean view).
			esper.Alias("sb", esper.JoinEventValue[esper.Event](0)),
			// window(s0.*) renders as the typed event-array snapshot;
			// Materialize the WindowAccessValue through its Values() method
			// so the column normalizes as []esper.Event (Java EventBean[]).
			esper.Alias("rows", esper.Method[[]esper.Event](
				esper.WindowAccessBy[esper.Event](esper.JoinEventValue[esper.Event](1)), "Values")),
		).Query(esper.StatementName("s0")))
	case "unagg-having":
		// Java ESPER-571: select max(intPrimitive) as val from
		// SupportBean#time(1) having max(intPrimitive) > intBoxed. No clock
		// advancement, so the window never expires; the having binds its
		// unaggregated property to the current event. Ungrouped statements
		// are exempt from the having-containment rule (Java validateHaving
		// fires only when the group-by property set is non-empty).
		intPrimitive := esper.Field[resultsetQueryTypeRowPerEventBean, int]("intPrimitive")
		maxVal := esper.Max[int](intPrimitive)
		plan, planErr = env.Build(esper.From[resultsetQueryTypeRowPerEventBean](env, "SupportBean").
			Window(esper.TimeWindow(365 * 24 * time.Hour)).
			Aggregate(esper.Alias("val", maxVal)).
			Having(esper.Greater[int](maxVal, esper.Cast[*int, int](esper.Field[resultsetQueryTypeRowPerEventBean, *int]("intBoxed")))).
			Query(esper.StatementName("s0")))
	case "sum-avg-where":
		// Java: select 'IBM stats' as title, volume, avg(volume) as myAvg,
		// sum(volume) as mySum from SupportMarketDataBean#length(3) where
		// symbol='IBM'. The where filters before retention: filtered events
		// never enter the window, never evict, never output.
		avgVolume := esper.Avg[int64](volume)
		plan, planErr = env.Build(esper.From[resultsetQueryTypeRowPerEventMarketData](env, "SupportMarketDataBean").
			Filter(esper.Equal[string](symbolValue, esper.Literal("IBM"))).
			Window(esper.LengthWindow(3)).
			Aggregate(
				esper.Alias("title", esper.Literal("IBM stats")),
				esper.Alias("volume", volume),
				esper.Alias("myAvg", avgVolume),
				esper.Alias("mySum", esper.Sum[int64](volume)),
			).
			Query(esper.StatementName("s0")))
	case "distinct-sum":
		// Java: select irstream symbol, sum(distinct volume) as volSum from
		// SupportMarketDataBean#length(3). Old rows carry the evicted DELL
		// row's symbol with the post-eviction distinct sum.
		volSum := esper.DistinctAggregate[int64](esper.Sum[int64](volume), volume)
		plan, planErr = env.Build(esper.From[resultsetQueryTypeRowPerEventMarketData](env, "SupportMarketDataBean").
			Window(esper.LengthWindow(3)).
			Aggregate(
				esper.Alias("symbol", symbolValue),
				esper.Alias("volSum", volSum),
			).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "distinct-nullable":
		// Java: select irstream avg(distinct volume) as avgVolume,
		// count(distinct symbol) as countDistinctSymbol from
		// SupportMarketDataBean (unbounded). Each send pairs the new row with
		// the previous-state old row (avg null / count 0 on the first pair);
		// null volume/symbol contribute nothing.
		avgVolume := esper.DistinctAggregate[float64](esper.Avg[int64](volume), volume)
		countSymbol := esper.CountDistinct[string](symbol)
		plan, planErr = env.Build(esper.From[resultsetQueryTypeRowPerEventMarketData](env, "SupportMarketDataBean").
			Aggregate(
				esper.Alias("avgVolume", avgVolume),
				esper.Alias("countDistinctSymbol", countSymbol),
			).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset querytype row-per-event case %q", caseName)
	}
	if planErr != nil {
		return compat.Trace{}, planErr
	}

	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one statement, got %d", len(statements))
	}
	statement := statements[0]
	seq := uint64(0)
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		newRows := compat.NormalizeResults(batch.New)
		oldRows := compat.NormalizeResults(batch.Old)
		if len(newRows) == 0 && len(oldRows) == 0 {
			return nil
		}
		seq++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "listener", Statement: statement.Name(),
			Sequence: seq, Time: batch.Time.UTC().Format(time.RFC3339), New: newRows, Old: oldRows,
		})
		return nil
	}); err != nil {
		return compat.Trace{}, err
	}

	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			if err := sendResultsetQueryTypeRowPerEventEvent(ctx, engine, step); err != nil {
				return trace, err
			}
		default:
			return trace, fmt.Errorf("unsupported step %q", step.Op)
		}
	}
	return trace, nil
}

func sendResultsetQueryTypeRowPerEventEvent(ctx context.Context, engine *esper.Engine, step compat.Step) error {
	switch step.EventType {
	case "SupportBean":
		var payload struct {
			TheString     string `json:"theString"`
			IntPrimitive  int    `json:"intPrimitive"`
			IntBoxed      *int   `json:"intBoxed"`
			LongBoxed     *int64 `json:"longBoxed"`
			LongPrimitive int64  `json:"longPrimitive"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		// The Java char default renders as "\u0000" in the trace; the Go
		// string zero would render as "".
		return engine.Send(ctx, "SupportBean", resultsetQueryTypeRowPerEventBean{
			TheString: payload.TheString, IntPrimitive: payload.IntPrimitive,
			IntBoxed: payload.IntBoxed, LongBoxed: payload.LongBoxed,
			LongPrimitive: payload.LongPrimitive,
			CharPrimitive: "\u0000",
		})
	case "SupportBeanString":
		var payload struct {
			TheString string `json:"theString"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		return engine.Send(ctx, "SupportBeanString", resultsetQueryTypeRowPerEventBeanString{TheString: payload.TheString})
	case "SupportMarketDataBean":
		var payload struct {
			Symbol *string `json:"symbol"`
			Price  float64 `json:"price"`
			Volume *int64  `json:"volume"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		return engine.Send(ctx, "SupportMarketDataBean", resultsetQueryTypeRowPerEventMarketData{
			Symbol: payload.Symbol, Price: payload.Price, Volume: payload.Volume,
		})
	case "SupportBean_S0":
		var payload struct {
			ID  int    `json:"id"`
			P00 string `json:"p00"`
			P01 string `json:"p01"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		return engine.Send(ctx, "SupportBean_S0", resultsetQueryTypeRowPerEventS0{
			ID: payload.ID, P00: payload.P00, P01: payload.P01,
		})
	}
	return fmt.Errorf("unknown event type %q", step.EventType)
}
