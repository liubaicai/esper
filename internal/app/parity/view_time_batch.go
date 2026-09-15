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

// Parity coverage for the deterministic ViewTimeBatch executions:
// time_batch flush anchors with quiet minus-one-millisecond ticks, new/old
// batch pairs across consecutive anchors, START_EAGER/FORCE_UPDATE hints
// whose empty boundary flushes deliver nothing observable, calendar-month
// batching, and partial-batch iterator checkpoints.
var viewTimeBatchJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewTimeBatch.java",
}

type viewTimeBatchBean struct {
	TheString    *string  `esper:"theString"`
	IntPrimitive int32    `esper:"intPrimitive"`
	DoubleBoxed  *float64 `esper:"doubleBoxed"`
}

type viewTimeBatchMarket struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
	Feed   *string `esper:"feed"`
}

var (
	viewTimeBatchJavaRuntimeIDs = []string{
		"java-runtime-5a110975a6ab7750e433", // SceneOne
		"java-runtime-77fc0819a391d361a0c8", // 10Sec
		"java-runtime-cb1e1193f3ace6152a25", // StartEagerForceUpdateSceneTwo
		"java-runtime-1a1b45f9c756465a0191", // MonthScoped
		"java-runtime-736d2f58158461c2c777", // StartEagerForceUpdate
		"java-runtime-9ae5cbb72669d84a7f7f", // Multirow
		"java-runtime-a3c81710a21ac12db682", // MultiBatch
		"java-runtime-5e01b6f92f8d8bc5851e", // NoRefPoint
	}
	viewTimeBatchJavaExecutions = []string{
		"ViewTimeBatchSceneOne",
		"ViewTimeBatch10Sec",
		"ViewTimeBatchStartEagerForceUpdateSceneTwo",
		"ViewTimeBatchMonthScoped",
		"ViewTimeBatchStartEagerForceUpdate",
		"ViewTimeBatchMultirow",
		"ViewTimeBatchMultiBatch",
		"ViewTimeBatchNoRefPoint",
	}
)

var viewTimeBatchCaseOrder = []string{
	"scene-one", "ten-sec", "start-eager-force-update-scene-two", "month-scoped",
	"start-eager-force-update", "multirow", "multi-batch", "no-ref-point",
}

func runViewTimeBatchScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range viewTimeBatchCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		caseTrace, err := runViewTimeBatchCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("view-time-batch case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("view-time-batch scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runViewTimeBatchCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
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

	symbol := esper.Field[viewTimeBatchMarket, string]("symbol")
	switch caseName {
	case "scene-one":
		err = build("s0", true, esper.FromAny(env, "SupportMarketDataBean").
			Window(esper.TimeBatch(time.Second)).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "ten-sec":
		err = build("s0", true, esper.From[viewTimeBatchBean](env, "SupportBean").
			Window(esper.TimeBatch(10*time.Second)).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "start-eager-force-update-scene-two":
		err = build("s0", true, esper.Select(
			esper.From[viewTimeBatchMarket](env, "SupportMarketDataBean").
				Window(esper.TimeBatchForce(time.Second, true, true)),
			esper.Alias("symbol", symbol),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "month-scoped":
		err = build("s0", true, esper.From[viewTimeBatchBean](env, "SupportBean").
			Window(esper.TimeBatchCalendar(0, 1, 0)).
			Query(esper.StatementName("s0")))
	case "start-eager-force-update":
		err = build("s0", true, esper.From[viewTimeBatchBean](env, "SupportBean").
			Window(esper.TimeBatchForce(time.Second, true, true)).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "multirow":
		err = build("s0", true, esper.From[viewTimeBatchBean](env, "SupportBean").
			Window(esper.TimeBatch(10*time.Second)).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "multi-batch":
		err = build("s0", true, esper.From[viewTimeBatchBean](env, "SupportBean").
			Window(esper.TimeBatch(10*time.Second)).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "no-ref-point":
		err = build("s0", true, esper.From[viewTimeBatchBean](env, "SupportBean").
			Window(esper.TimeBatch(10*time.Minute)).
			Query(esper.StatementName("s0")))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported view-time-batch case %q", caseName)
	}
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	seq := uint64(0)
	statements := make(map[string]*esper.Statement)
	for _, item := range plans {
		deployment, err := engine.Deploy(ctx, item.plan)
		if err != nil {
			return trace, err
		}
		for _, st := range deployment.Statements() {
			statements[st.Name()] = st
		}
		if !item.listener {
			continue
		}
		st := deployment.Statements()[0]
		if _, err := st.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			hasNew := len(batch.New) > 0
			hasOld := len(batch.Old) > 0
			if !hasNew && !hasOld {
				return nil
			}
			seq++
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "listener",
				Statement: st.Name(),
				Time:      compat.FormatTraceTime(batch.Time),
				Sequence:  seq,
			}
			record.New = compat.NormalizeResults(batch.New)
			record.Old = compat.NormalizeResults(batch.Old)
			trace.Records = append(trace.Records, record)
			return nil
		}); err != nil {
			return trace, err
		}
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return trace, fmt.Errorf("view-time-batch decode %s: %w", step.EventType, err)
			}
			switch step.EventType {
			case "SupportMarketDataBean":
				for _, field := range []struct {
					name   string
					defVal any
				}{{"price", float64(0)}, {"volume", int64(0)}, {"feed", nil}} {
					if _, ok := payload[field.name]; !ok {
						payload[field.name] = field.defVal
					}
				}
			case "SupportBean":
				for _, field := range []struct {
					name   string
					defVal any
				}{{"intPrimitive", int32(0)}, {"doubleBoxed", nil}} {
					if _, ok := payload[field.name]; !ok {
						payload[field.name] = field.defVal
					}
				}
			}
			if err := engine.SendRecord(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "advance-time":
			at, parseErr := time.Parse(time.RFC3339Nano, step.At)
			if parseErr != nil {
				return trace, fmt.Errorf("view-time-batch advance time %q: %w", step.At, parseErr)
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return trace, err
			}
		case "deployed":
			// Silent on both sides: the oracle deploys without recording.
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
			return trace, fmt.Errorf("unsupported view-time-batch step op %q", step.Op)
		}
	}
	return trace, nil
}
