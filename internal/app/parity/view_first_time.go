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

// Parity coverage for ViewFirstTime: firsttime views admit events only during
// the window anchored at deployment, keep the admitted rows visible through
// the deadline, never admit later events, and expire silently (no old-data
// delivery). All executions are advance-time driven.
var viewFirstTimeJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewFirstTime.java",
}

type viewFirstTimeBean struct {
	TheString    *string  `esper:"theString"`
	IntPrimitive int32    `esper:"intPrimitive"`
	DoubleBoxed  *float64 `esper:"doubleBoxed"`
}

type viewFirstTimeMarket struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
	Feed   *string `esper:"feed"`
}

var (
	viewFirstTimeJavaRuntimeIDs = []string{
		"java-runtime-9733dfdbee02d899bb23", // Simple
		"java-runtime-b1061971825b14664ff2", // SceneOne
		"java-runtime-ac89ffce844dc00114f7", // SceneTwo
	}
	viewFirstTimeJavaExecutions = []string{
		"ViewFirstTimeSimple",
		"ViewFirstTimeSceneOne",
		"ViewFirstTimeSceneTwo",
	}
)

var viewFirstTimeCaseOrder = []string{
	"first-time-simple", "first-time-scene-one", "first-time-scene-two",
}

func runViewFirstTimeScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range viewFirstTimeCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		caseTrace, err := runViewFirstTimeCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("view-first-time case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("view-first-time scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runViewFirstTimeCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[viewFirstTimeBean](env, "SupportBean"); err != nil {
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

	var deployment struct {
		name string
		plan esper.Plan
	}
	ts := esper.Field[viewFirstTimeBean, *string]("theString")
	ip := esper.Field[viewFirstTimeBean, int32]("intPrimitive")
	switch caseName {
	case "first-time-simple":
		plan, buildErr := env.Build(esper.From[viewFirstTimeBean](env, "SupportBean").
			Window(esper.FirstTimeCalendar(0, 1, 0)).
			Query(esper.StatementName("s0")))
		if buildErr != nil {
			return compat.Trace{}, buildErr
		}
		deployment = struct {
			name string
			plan esper.Plan
		}{"s0", plan}
	case "first-time-scene-one":
		plan, buildErr := env.Build(esper.Select(
			esper.From[viewFirstTimeBean](env, "SupportBean").Window(esper.FirstTime(10*time.Second)),
			esper.Alias("c0", ts),
			esper.Alias("c1", ip),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
		if buildErr != nil {
			return compat.Trace{}, buildErr
		}
		deployment = struct {
			name string
			plan esper.Plan
		}{"s0", plan}
	case "first-time-scene-two":
		plan, buildErr := env.Build(esper.FromAny(env, "SupportMarketDataBean").
			Window(esper.FirstTime(time.Second)).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
		if buildErr != nil {
			return compat.Trace{}, buildErr
		}
		deployment = struct {
			name string
			plan esper.Plan
		}{"s0", plan}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported view-first-time case %q", caseName)
	}

	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	seq := uint64(0)
	var statement *esper.Statement
	subscribe := func(deployment *esper.Deployment) error {
		statement = deployment.Statements()[0]
		_, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			hasNew := len(batch.New) > 0
			hasOld := len(batch.Old) > 0
			if !hasNew && !hasOld {
				return nil
			}
			seq++
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "listener",
				Statement: statement.Name(),
				Time:      compat.FormatTraceTime(batch.Time),
				Sequence:  seq,
			}
			record.New = compat.NormalizeResults(batch.New)
			record.Old = compat.NormalizeResults(batch.Old)
			trace.Records = append(trace.Records, record)
			return nil
		})
		return err
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return trace, fmt.Errorf("view-first-time decode %s: %w", step.EventType, err)
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
				return trace, fmt.Errorf("view-first-time advance time %q: %w", step.At, parseErr)
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return trace, err
			}
		case "deployed":
			deployed, deployErr := engine.Deploy(ctx, deployment.plan)
			if deployErr != nil {
				return trace, deployErr
			}
			if err := subscribe(deployed); err != nil {
				return trace, err
			}
		case "snapshot":
			if statement == nil {
				return trace, fmt.Errorf("snapshot statement %q not found", step.Statement)
			}
			result, snapErr := statement.Snapshot(ctx)
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
			return trace, fmt.Errorf("unsupported view-first-time step op %q", step.Op)
		}
	}
	return trace, nil
}
