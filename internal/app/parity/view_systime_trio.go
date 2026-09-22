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

// Parity coverage for the view system-time trio: ViewTimeBatchRefPoint
// (ViewTimeBatch ord 9, time_batch with an absolute reference point),
// ViewTimeBatchWSystemTime (time_batch + #uni derived view), and
// ViewTimeWinWSystemTime (#time + #weighted_avg with passthrough props).
// The two WSystemTime executions are wall-clock Java tests; the scenario
// replays them under the virtual clock at the cumulative sleep instants.
// Approved differences: Go has no EPL text; each pinned deploy EPL maps to
// the fluent builder; Thread.sleep maps to advance-time steps; the
// statement-type assertion maps to the types op.
var viewSystimeTrioJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewTimeBatch.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewTimeBatchWSystemTime.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewTimeWinWSystemTime.java",
}

var viewSystimeTrioJavaRuntimeIDs = []string{
	"java-runtime-7d3c38fa4d2477a0be87", // ViewTimeBatchRefPoint
	"java-runtime-90902912cde58a38fafd", // ViewTimeBatchWSystemTime
	"java-runtime-486b77a63e1a3518f362", // ViewTimeWinWSystemTime
}

var viewSystimeTrioJavaExecutions = []string{
	"ViewTimeBatchRefPoint",
	"ViewTimeBatchWSystemTime",
	"ViewTimeWinWSystemTime",
}

const viewSystimeTrioID = "view-systime-trio"

func viewSystimeTrioRuntimeURI(caseName string) string {
	return viewSystimeTrioID + "-" + caseName
}

func runViewSystimeTrioScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	var traces []compat.Trace
	for _, caseName := range []string{
		"timebatch-refpoint",
		"timebatch-uni-systime",
		"timewin-weightedavg-systime",
	} {
		caseTrace, err := runViewSystimeTrioCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("view-systime-trio case %q: %w", caseName, err)
		}
		traces = append(traces, caseTrace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runViewSystimeTrioCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[viewTimeBatchBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	str := reflect.TypeOf("")
	f64 := reflect.TypeOf(float64(0))
	i64 := reflect.TypeOf(int64(0))
	if _, err := esper.RegisterMap(env, "SupportMarketDataBean", []esper.FieldSpec{
		esper.FieldDef("symbol", str),
		esper.FieldDef("price", f64),
		esper.FieldDef("volume", i64),
		esper.FieldDef("feed", str),
	}); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(viewSystimeTrioRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: viewSystimeTrioID}
	var sequence uint64
	var deployment *esper.Deployment
	var resultSchema esper.Schema
	deploy := func(epl string) error {
		query, err := viewSystimeTrioQueryForEPL(env, caseName, epl)
		if err != nil {
			return err
		}
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		if schema, ok := plan.ResultSchema(); ok {
			resultSchema = schema
		}
		deployment, err = engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		sequence = 0
		for _, st := range deployment.Statements() {
			st := st
			if _, subErr := st.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				if len(batch.New) == 0 && len(batch.Old) == 0 {
					return nil
				}
				sequence++
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: st.Name(),
					Sequence:  sequence,
					Time:      compat.FormatTraceTime(batch.Time),
					New:       compat.NormalizeResults(batch.New),
					Old:       compat.NormalizeResults(batch.Old),
				})
				return nil
			}); subErr != nil {
				return subErr
			}
		}
		return nil
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
		case "deploy":
			if err := deploy(step.Epl); err != nil {
				return trace, err
			}
		case "undeploy-all":
			if deployment != nil {
				if err := deployment.Undeploy(ctx); err != nil {
					return trace, fmt.Errorf("%s undeploy %q: %w", viewSystimeTrioID, caseName, err)
				}
				deployment = nil
			}
		case "send":
			if err := viewSystimeTrioSend(ctx, engine, step); err != nil {
				return trace, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return trace, err
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return trace, err
			}
		case "snapshot":
			if deployment == nil {
				return trace, fmt.Errorf("%s snapshot without deployment", viewSystimeTrioID)
			}
			result, err := deployment.Statements()[0].Snapshot(ctx)
			if err != nil {
				return trace, err
			}
			rows := compat.NormalizeResults(result.Batch.New)
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: "s0",
				Sequence:  0,
				Time:      compat.FormatTraceTime(engine.Now()),
			}
			if rows != nil {
				record.New = rows
			}
			trace.Records = append(trace.Records, record)
		case "types":
			if deployment == nil {
				return trace, fmt.Errorf("%s types without deployment", viewSystimeTrioID)
			}
			// The oracle records only the pinned asserted surface (Java
			// env.assertStatement checks property average is Double).
			properties := make(map[string]any)
			for _, field := range resultSchema.Fields() {
				if field.Name == "average" {
					properties[field.Name] = javaTypeName(field.Type)
				}
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "types",
				Statement: "s0",
				Sequence:  0,
				Time:      compat.FormatTraceTime(engine.Now()),
				Value:     map[string]any{"properties": properties},
			})
		default:
			return trace, fmt.Errorf("%s unsupported step op %q", viewSystimeTrioID, step.Op)
		}
	}
	return trace, nil
}

func viewSystimeTrioSend(ctx context.Context, engine *esper.Engine, step compat.Step) error {
	var payload map[string]any
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return fmt.Errorf("%s decode %s: %w", viewSystimeTrioID, step.EventType, err)
	}
	switch step.EventType {
	case "SupportBean":
		var bean viewTimeBatchBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return fmt.Errorf("%s decode SupportBean: %w", viewSystimeTrioID, err)
		}
		return engine.Send(ctx, step.EventType, bean)
	case "SupportMarketDataBean":
		for _, field := range []struct {
			name   string
			defVal any
		}{{"price", float64(0)}, {"volume", int64(0)}, {"feed", nil}} {
			if _, ok := payload[field.name]; !ok {
				payload[field.name] = field.defVal
			}
		}
		return engine.SendRecord(ctx, step.EventType, payload)
	default:
		return fmt.Errorf("%s unsupported event type %q", viewSystimeTrioID, step.EventType)
	}
}

// viewSystimeTrioQueryForEPL maps each pinned deploy EPL to the fluent
// builder. The EPL text is pinned verbatim from the Java source; the
// builder mirrors the same semantics.
func viewSystimeTrioQueryForEPL(env *esper.Environment, caseName, epl string) (esper.Query, error) {
	switch caseName {
	case "timebatch-refpoint":
		// #time_batch(10 minutes, 10L): absolute epoch-ms reference point.
		if epl == "@name('s0') select * from SupportBean#time_batch(10 minutes, 10L)" {
			return esper.From[viewTimeBatchBean](env, "SupportBean").
				Window(esper.TimeBatchRefPoint(10*time.Minute,
					time.Unix(0, 10*int64(time.Millisecond)).UTC())).
				Query(esper.StatementName("s0")), nil
		}
	case "timebatch-uni-systime":
		// #time_batch(2)#uni(volume): select * exposes the six uni stats
		// fields under the Java ViewFieldEnum names.
		if epl == "@name('s0') select * from SupportMarketDataBean(symbol='CSCO.O')#time_batch(2)#uni(volume)" {
			volume := esper.Field[any, int64]("volume")
			stats := esper.UnivariateStatistics[int64](volume)
			return esper.FromAny(env, "SupportMarketDataBean").
				Filter(esper.Equal[string](esper.Field[any, string]("symbol"), esper.Literal("CSCO.O"))).
				Window(esper.TimeBatch(2*time.Second)).
				Aggregate(
					esper.Alias("datapoints", stats.Datapoints()),
					esper.Alias("total", stats.Total()),
					esper.Alias("average", stats.Average()),
					esper.Alias("stddevpa", stats.StdDevPop()),
					esper.Alias("stddev", stats.StdDev()),
					esper.Alias("variance", stats.Variance()),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "timewin-weightedavg-systime":
		// #time(3.0)#weighted_avg(price, volume, symbol, feed): params ≥3
		// are passthrough props evaluated on the last new event and
		// retained on the NaN row — LastEver mirrors that retention (bare
		// aliases would suppress the expiry emission).
		if epl == "@name('s0') select * from SupportMarketDataBean(symbol='CSCO.O')#time(3.0)#weighted_avg(price, volume, symbol, feed)" {
			price := esper.Field[any, float64]("price")
			volume := esper.Field[any, int64]("volume")
			symbol := esper.Field[any, string]("symbol")
			feed := esper.Field[any, string]("feed")
			return esper.FromAny(env, "SupportMarketDataBean").
				Filter(esper.Equal[string](symbol, esper.Literal("CSCO.O"))).
				Window(esper.TimeWindow(3*time.Second)).
				Aggregate(
					esper.Alias("average", esper.WeightedAvg[float64, int64](price, volume)),
					esper.Alias("symbol", esper.LastEver[string](symbol)),
					esper.Alias("feed", esper.LastEver[string](feed)),
				).
				Query(esper.StatementName("s0")), nil
		}
	}
	return esper.Query{}, fmt.Errorf("%s case %q: unpinned deploy epl %q", viewSystimeTrioID, caseName, epl)
}
