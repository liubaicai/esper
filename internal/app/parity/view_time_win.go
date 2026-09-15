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

// Parity coverage for ViewTimeWin: sliding time-window expiry lifecycles under
// the virtual clock, windowed sum aggregates (ungrouped/grouped/filtered),
// calendar-month rstream expiry, the prev family over a sliding window,
// prepared-statement and variable window durations deployed twice, seven
// verbatim 30000-second time-period spellings, and the flip-timer boundary
// variants. All executions are advance-time driven.
var viewTimeWinJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewTimeWin.java",
}

type viewTimeWinBean struct {
	TheString    *string  `esper:"theString"`
	IntPrimitive int32    `esper:"intPrimitive"`
	DoubleBoxed  *float64 `esper:"doubleBoxed"`
}

type viewTimeWinMarket struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
	Feed   *string `esper:"feed"`
}

var (
	viewTimeWinJavaRuntimeIDs = []string{
		"java-runtime-25dfb49811c974a34e5d", // SceneOne
		"java-runtime-a8007c80ae5756bb4ddb", // SceneTwo
		"java-runtime-63f096fecc7fa8382cd1", // JustSelectStar
		"java-runtime-9b1ddabf0088cb326211", // Sum
		"java-runtime-6edb156e4a10bcba9784", // SumGroupBy
		"java-runtime-9239909fee9398909be1", // SumWFilter
		"java-runtime-71fe6ccae381fc1a0843", // MonthScoped
		"java-runtime-aa228ddbee38aa48a796", // WPrev
		"java-runtime-fc4ed4ace0c09a9154da", // PreparedStmt
		"java-runtime-323ea34bd1e1c38e3895", // VariableStmt
		"java-runtime-87e95838609b5cc88869", // TimePeriod
		"java-runtime-469f129b7fa2da1d0b14", // VariableTimePeriodStmt
		"java-runtime-19a1a7c856e9567f7aac", // TimePeriodParams (seven per-spec cases)
		"java-runtime-2deb887a05147a8dc59c", // FlipTimer{0, "1", 1000}
		"java-runtime-d563c454a45f40bcc404", // FlipTimer{123456789, "10", 123466789}
		"java-runtime-cc6e1ac37196bc5ae5e8", // FlipTimer{0, "1 months 10 milliseconds", ...}
		"java-runtime-c9c0f3b2ebd23aedce50", // FlipTimer{2002-05-01, "1 months 50 milliseconds", ...}
	}
	viewTimeWinJavaExecutions = []string{
		"ViewTimeWindowSceneOne",
		"ViewTimeWindowSceneTwo",
		"ViewTimeJustSelectStar",
		"ViewTimeSum",
		"ViewTimeSumGroupBy",
		"ViewTimeSumWFilter",
		"ViewTimeWindowMonthScoped",
		"ViewTimeWindowWPrev",
		"ViewTimeWindowPreparedStmt",
		"ViewTimeWindowVariableStmt",
		"ViewTimeWindowTimePeriod",
		"ViewTimeWindowVariableTimePeriodStmt",
		"ViewTimeWindowTimePeriodParams",
		"ViewTimeWindowFlipTimer{startTime=0, size='1', flipTime=1000}",
		"ViewTimeWindowFlipTimer{startTime=123456789, size='10', flipTime=123466789}",
		"ViewTimeWindowFlipTimer{startTime=0, size='1 months 10 milliseconds', flipTime=2678400010}",
		"ViewTimeWindowFlipTimer{startTime=1020211201999, size='1 months 50 milliseconds', flipTime=1022889602049}",
	}
)

var viewTimeWinCaseOrder = []string{
	"scene-one", "scene-two",
	"just-select-star", "sum", "sum-group-by", "sum-w-filter", "month-scoped",
	"w-prev", "prepared-stmt", "variable-stmt", "time-period",
	"variable-time-period", "time-period-params-1", "time-period-params-2",
	"time-period-params-3", "time-period-params-4", "time-period-params-5",
	"time-period-params-6", "time-period-params-7", "flip-timer-1s",
	"flip-timer-10s-large-start", "flip-timer-months-ms-epoch",
	"flip-timer-months-ms-2002",
}

func runViewTimeWinScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range viewTimeWinCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		caseTrace, err := runViewTimeWinCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("view-time-win case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("view-time-win scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

// viewTimeWinPlannedDeployment couples one pending deployment with its
// statement name and optional substitution parameters; deployed steps consume
// the entries in order.
type viewTimeWinPlannedDeployment struct {
	name       string
	plan       esper.Plan
	parameters esper.ParameterValues
}

func runViewTimeWinCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[viewTimeWinBean](env, "SupportBean"); err != nil {
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

	// Pinned harness defaults: TIME_WIN_ONE int 4, TIME_WIN_TWO double 4000.
	// Go durations are float64 seconds/ms/minutes per the unit-test precedent.
	variablesRegistered := map[string]bool{}
	registerVariable := func(name string, value float64) error {
		if variablesRegistered[name] {
			return nil
		}
		if err := env.RegisterVariable(name, value); err != nil {
			return err
		}
		variablesRegistered[name] = true
		return nil
	}

	build := func(name string, parameters esper.ParameterValues, query esper.Query) (viewTimeWinPlannedDeployment, error) {
		plan, err := env.Build(query)
		if err != nil {
			return viewTimeWinPlannedDeployment{}, err
		}
		return viewTimeWinPlannedDeployment{name: name, plan: plan, parameters: parameters}, nil
	}
	ts := esper.Field[viewTimeWinBean, *string]("theString")
	symbol := esper.Field[viewTimeWinMarket, string]("symbol")
	var deployments []viewTimeWinPlannedDeployment
	switch caseName {
	case "scene-one", "scene-two":
		entry, buildErr := build("s0", nil, esper.From[viewTimeWinBean](env, "SupportBean").
			Window(esper.TimeWindow(10*time.Second)).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
		if buildErr != nil {
			return compat.Trace{}, buildErr
		}
		deployments = append(deployments, entry)
	case "just-select-star":
		entry, buildErr := build("s0", nil, esper.FromAny(env, "SupportMarketDataBean").
			Window(esper.TimeWindow(time.Second)).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
		if buildErr != nil {
			return compat.Trace{}, buildErr
		}
		deployments = append(deployments, entry)
	case "sum":
		entry, buildErr := build("s0", nil, esper.From[viewTimeWinMarket](env, "SupportMarketDataBean").
			Window(esper.TimeWindow(30*time.Millisecond)).
			Aggregate(
				esper.Alias("symbol", symbol),
				esper.Alias("volume", esper.Field[viewTimeWinMarket, int64]("volume")),
				esper.Alias("mySum", esper.Sum[float64](esper.Field[viewTimeWinMarket, float64]("price"))),
			).Query(esper.StatementName("s0")))
		if buildErr != nil {
			return compat.Trace{}, buildErr
		}
		deployments = append(deployments, entry)
	case "sum-group-by":
		entry, buildErr := build("s0", nil, esper.From[viewTimeWinMarket](env, "SupportMarketDataBean").
			Window(esper.TimeWindow(30*time.Millisecond)).
			GroupBy(symbol).
			Select(
				esper.Alias("symbol", symbol),
				esper.Alias("volume", esper.Field[viewTimeWinMarket, int64]("volume")),
				esper.Alias("mySum", esper.Sum[float64](esper.Field[viewTimeWinMarket, float64]("price"))),
			).Query(esper.StatementName("s0")))
		if buildErr != nil {
			return compat.Trace{}, buildErr
		}
		deployments = append(deployments, entry)
	case "sum-w-filter":
		entry, buildErr := build("s0", nil, esper.From[viewTimeWinMarket](env, "SupportMarketDataBean").
			Filter(esper.Equal[string](symbol, esper.Literal("IBM"))).
			Window(esper.TimeWindow(30*time.Millisecond)).
			Aggregate(
				esper.Alias("symbol", symbol),
				esper.Alias("volume", esper.Field[viewTimeWinMarket, int64]("volume")),
				esper.Alias("mySum", esper.Sum[float64](esper.Field[viewTimeWinMarket, float64]("price"))),
			).Query(esper.StatementName("s0")))
		if buildErr != nil {
			return compat.Trace{}, buildErr
		}
		deployments = append(deployments, entry)
	case "month-scoped":
		entry, buildErr := build("s0", nil, esper.From[viewTimeWinBean](env, "SupportBean").
			Window(esper.TimeWindowCalendar(0, 1, 0, 0)).
			Query(esper.StatementName("s0"), esper.WithRemoveStreamOnly()))
		if buildErr != nil {
			return compat.Trace{}, buildErr
		}
		deployments = append(deployments, entry)
	case "w-prev":
		entry, buildErr := build("s0", nil, esper.Select(
			esper.From[viewTimeWinMarket](env, "SupportMarketDataBean").Window(esper.TimeWindow(time.Second)),
			esper.Alias("symbol", symbol),
			esper.Alias("prev1", esper.Prev[string](1, symbol)),
			esper.Alias("prevtail", esper.PrevTail[string](0, symbol)),
			esper.Alias("prevCountSym", esper.PrevCount[int64](symbol)),
			esper.Alias("prevWindowSym", esper.PrevWindow[string](symbol)),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
		if buildErr != nil {
			return compat.Trace{}, buildErr
		}
		deployments = append(deployments, entry)
	case "prepared-stmt":
		// One pinned text deployed twice with positional substitution
		// values 4 and 3; the runtime statement names s0/s1 are observable,
		// so Go builds the same plan body twice under both names.
		for _, entry := range []struct {
			name       string
			parameters esper.ParameterValues
		}{
			{"s0", esper.ParameterValues{"seconds": 4.0}},
			{"s1", esper.ParameterValues{"seconds": 3.0}},
		} {
			plan, buildErr := env.Build(esper.Select(
				esper.From[viewTimeWinBean](env, "SupportBean").
					Window(esper.TimeWindowSeconds[float64](esper.Parameter[float64]("seconds"))),
				esper.Alias("theString", ts),
			).Query(esper.StatementName(entry.name), esper.WithRemoveStreamOnly()))
			if buildErr != nil {
				return compat.Trace{}, buildErr
			}
			deployments = append(deployments, viewTimeWinPlannedDeployment{name: entry.name, plan: plan, parameters: entry.parameters})
		}
	case "variable-stmt":
		if err := registerVariable("TIME_WIN_ONE", 4.0); err != nil {
			return compat.Trace{}, err
		}
		for _, name := range []string{"s0", "s1"} {
			plan, buildErr := env.Build(esper.Select(
				esper.From[viewTimeWinBean](env, "SupportBean").
					Window(esper.TimeWindowSeconds[float64](esper.VariableRef[float64]("TIME_WIN_ONE"))),
				esper.Alias("theString", ts),
			).Query(esper.StatementName(name), esper.WithRemoveStreamOnly()))
			if buildErr != nil {
				return compat.Trace{}, buildErr
			}
			deployments = append(deployments, viewTimeWinPlannedDeployment{name: name, plan: plan})
		}
	case "time-period":
		for _, entry := range []struct {
			name   string
			window esper.WindowSpec
		}{
			{"s0", esper.TimeWindow(4 * time.Second)},
			{"s1", esper.TimeWindow(3000 * time.Millisecond)},
		} {
			plan, buildErr := env.Build(esper.Select(
				esper.From[viewTimeWinBean](env, "SupportBean").Window(entry.window),
				esper.Alias("theString", ts),
			).Query(esper.StatementName(entry.name), esper.WithRemoveStreamOnly()))
			if buildErr != nil {
				return compat.Trace{}, buildErr
			}
			deployments = append(deployments, viewTimeWinPlannedDeployment{name: entry.name, plan: plan})
		}
	case "variable-time-period":
		if err := registerVariable("TIME_WIN_TWO", 4000.0); err != nil {
			return compat.Trace{}, err
		}
		for _, entry := range []struct {
			name   string
			window esper.WindowSpec
		}{
			{"s0", esper.TimeWindowMilliseconds[float64](esper.VariableRef[float64]("TIME_WIN_TWO"))},
			{"s1", esper.TimeWindowMinutes[float64](esper.VariableRef[float64]("TIME_WIN_TWO"))},
		} {
			plan, buildErr := env.Build(esper.Select(
				esper.From[viewTimeWinBean](env, "SupportBean").Window(entry.window),
				esper.Alias("theString", ts),
			).Query(esper.StatementName(entry.name), esper.WithRemoveStreamOnly()))
			if buildErr != nil {
				return compat.Trace{}, buildErr
			}
			deployments = append(deployments, viewTimeWinPlannedDeployment{name: entry.name, plan: plan})
		}
	case "time-period-params-1", "time-period-params-2", "time-period-params-3",
		"time-period-params-4", "time-period-params-5", "time-period-params-6",
		"time-period-params-7":
		// Every pinned spelling resolves to exactly 30000 seconds.
		entry, buildErr := build("s0", nil, esper.From[viewTimeWinBean](env, "SupportBean").
			Window(esper.TimeWindow(30000*time.Second)).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
		if buildErr != nil {
			return compat.Trace{}, buildErr
		}
		deployments = append(deployments, entry)
	case "flip-timer-1s":
		entry, buildErr := build("s0", nil, esper.From[viewTimeWinBean](env, "SupportBean").
			Window(esper.TimeWindow(time.Second)).
			Query(esper.StatementName("s0")))
		if buildErr != nil {
			return compat.Trace{}, buildErr
		}
		deployments = append(deployments, entry)
	case "flip-timer-10s-large-start":
		entry, buildErr := build("s0", nil, esper.From[viewTimeWinBean](env, "SupportBean").
			Window(esper.TimeWindow(10*time.Second)).
			Query(esper.StatementName("s0")))
		if buildErr != nil {
			return compat.Trace{}, buildErr
		}
		deployments = append(deployments, entry)
	case "flip-timer-months-ms-epoch":
		entry, buildErr := build("s0", nil, esper.From[viewTimeWinBean](env, "SupportBean").
			Window(esper.TimeWindowCalendar(0, 1, 0, 10*time.Millisecond)).
			Query(esper.StatementName("s0")))
		if buildErr != nil {
			return compat.Trace{}, buildErr
		}
		deployments = append(deployments, entry)
	case "flip-timer-months-ms-2002":
		entry, buildErr := build("s0", nil, esper.From[viewTimeWinBean](env, "SupportBean").
			Window(esper.TimeWindowCalendar(0, 1, 0, 50*time.Millisecond)).
			Query(esper.StatementName("s0")))
		if buildErr != nil {
			return compat.Trace{}, buildErr
		}
		deployments = append(deployments, entry)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported view-time-win case %q", caseName)
	}

	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	seq := uint64(0)
	statements := make(map[string]*esper.Statement)
	liveDeployments := make(map[string]*esper.Deployment)
	deployIndex := 0
	subscribe := func(name string, deployment *esper.Deployment) error {
		statement := deployment.Statements()[0]
		statements[name] = statement
		liveDeployments[name] = deployment
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
				return trace, fmt.Errorf("view-time-win decode %s: %w", step.EventType, err)
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
				return trace, fmt.Errorf("view-time-win advance time %q: %w", step.At, parseErr)
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return trace, err
			}
		case "deployed":
			if deployIndex >= len(deployments) {
				return trace, fmt.Errorf("view-time-win case %q: unexpected deployed step for %q", caseName, step.Statement)
			}
			pending := deployments[deployIndex]
			deployIndex++
			if pending.name != step.Statement {
				return trace, fmt.Errorf("view-time-win case %q: deployed step names %q but the pinned plan expects %q",
					caseName, step.Statement, pending.name)
			}
			var deployment *esper.Deployment
			var deployErr error
			if pending.parameters != nil {
				deployment, deployErr = engine.DeployWithParameters(ctx, pending.plan, pending.parameters)
			} else {
				deployment, deployErr = engine.Deploy(ctx, pending.plan)
			}
			if deployErr != nil {
				return trace, deployErr
			}
			if err := subscribe(pending.name, deployment); err != nil {
				return trace, err
			}
		case "undeploy":
			deployment, ok := liveDeployments[step.Statement]
			if !ok {
				return trace, fmt.Errorf("view-time-win case %q: undeploy references undeployed statement %q", caseName, step.Statement)
			}
			if err := deployment.Undeploy(ctx); err != nil {
				return trace, err
			}
			delete(liveDeployments, step.Statement)
			delete(statements, step.Statement)
		case "set-variable":
			var payload struct {
				Type  string `json:"type"`
				Value any    `json:"value"`
			}
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return trace, fmt.Errorf("view-time-win decode set-variable %s: %w", step.Name, err)
			}
			value, convErr := viewTimeWinVariableValue(step.Name, payload)
			if convErr != nil {
				return trace, convErr
			}
			if err := engine.SetVariable(ctx, step.Name, value); err != nil {
				return trace, err
			}
		case "snapshot":
			statement, ok := statements[step.Statement]
			if !ok {
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
			return trace, fmt.Errorf("unsupported view-time-win step op %q", step.Op)
		}
	}
	return trace, nil
}

// viewTimeWinVariableValue converts the pinned harness variable types: both
// duration variables are registered as float64 in Go regardless of the
// scenario's Java-side type spelling.
func viewTimeWinVariableValue(name string, payload struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}) (float64, error) {
	switch name {
	case "TIME_WIN_ONE", "TIME_WIN_TWO":
	default:
		return 0, fmt.Errorf("view-time-win: unexpected set-variable target %q", name)
	}
	number, ok := payload.Value.(float64)
	if !ok {
		return 0, fmt.Errorf("view-time-win: set-variable %s value %v is not numeric", name, payload.Value)
	}
	return number, nil
}
