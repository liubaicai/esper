package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextInitTermBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     *int64 `esper:"intBoxed"`
	LongBoxed    *int64 `esper:"longBoxed"`
}

type contextInitTermS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 string  `esper:"p01"`
}

type contextInitTermS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 string  `esper:"p11"`
}

const contextInitTermJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextInitTermJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermTemporalFixed.java",
}

var (
	contextInitTermJavaRuntimeIDs = []string{
		"java-runtime-0b7363d451c2f72c5038",
		"java-runtime-44efa77435e01275da02",
		"java-runtime-521d7b99d221b8a0a5b3",
		"java-runtime-f17d1a63f5dafe5fa895",
		"java-runtime-0462363d89bce1987f95",
		"java-runtime-f3b99373dfd94c1e5d8f",
		"java-runtime-dfb7b1c195efb70fa200",
		"java-runtime-09d7f8c16e7ea336773e",
		"java-runtime-c2ee338ea039c3d6fb7a",
		"java-runtime-d5217f32217bf1114d96",
		"java-runtime-cbfe2ec943d1d33c4f56",
	}
	contextInitTermJavaExecutions = []string{
		"ContextStartEndFilterStartedFilterEndedCorrelatedOutputSnapshot",
		"ContextStartEndFilterStartedPatternEndedCorrelated",
		"ContextStartEndFilterStartedFilterEndedOutputSnapshot",
		"ContextStartEndPatternStartedPatternEnded",
		"ContextStartEndContextCreateDestroy",
		"ContextStartEndJoin",
		"ContextStartEndPatternWithTime",
		"ContextStartEndStartTurnedOff",
		"ContextStartEndStartTurnedOn",
		"ContextStart9End5AggUngrouped",
		"ContextStart9End5AggGrouped",
	}
)

// runContextInitTermTemporalFixedScenario replays 11 executions of
// ContextInitTermTemporalFixed: initiated-terminated contexts with filter and
// pattern boundaries (correlated termination, output snapshot when
// terminated), every-second cron contexts, daily 9-to-5 cron contexts with
// full-outer joins, patterns with timers, multi-statement mid-case
// deployments and grouped/ungrouped aggregation.
func runContextInitTermTemporalFixedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"correlated-output-snapshot", "filter-pattern-correlated", "filter-output-snapshot",
		"pattern-pattern", "every-second", "daily-join", "daily-pattern-time",
		"turned-off", "turned-on", "daily-agg-ungrouped", "daily-agg-grouped"}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("context-init-term-temporal-fixed scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runContextInitTermCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-init-term-temporal-fixed case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

// initialAdvanceTime returns the case's first advance-time step, the clock
// position at which the oracle deploys (cron contexts evaluate their
// schedule from that instant).
func initialAdvanceTime(scenario compat.Scenario, caseName string) (string, bool) {
	active := false
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			active = step.Case == caseName
			continue
		}
		if !active {
			continue
		}
		if step.Op == "advance-time" {
			return step.At, true
		}
		if step.Op != "send" && step.Op != "deploy" {
			return "", false
		}
	}
	return "", false
}

func runContextInitTermCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextInitTermBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}

	bean := esper.From[contextInitTermBean](env, "SupportBean")
	beanKeepAll := bean.Window(esper.KeepAll())
	beanSource := esper.From[contextInitTermBean](env, "SupportBean")
	theString := esper.Field[contextInitTermBean, string]("theString")
	intPrimitive := esper.Field[contextInitTermBean, int]("intPrimitive")
	isS0 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
	isS1 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S1"))
	initiatingP00 := esper.Property[string](esper.ContextInitiatingEvent(), "p00")
	initiatingID := esper.Property[int](esper.ContextInitiatingEvent(), "id")
	terminatingID := esper.Property[int](esper.ContextTerminatingEvent(), "id")

	var engine *esper.Engine
	cleanup := true
	defer func() {
		if cleanup {
			_ = engine.Close(context.Background())
		}
	}()

	switch caseName {
	case "correlated-output-snapshot":
		start := isS0
		end := esper.And(isS1, esper.Equal[*string](esper.Field[any, *string]("p10"),
			esper.Property[*string](esper.ContextInitiatingEvent(), "p00")))
		if _, err := esper.CreateInitiatedTerminatedContext(env, "EveryNowAndThen", esper.Literal("global"), start, end); err != nil {
			return compat.Trace{}, err
		}
		query := beanKeepAll.Aggregate(
			esper.Alias("c1", initiatingID),
			esper.Alias("c2", terminatingID),
			esper.Alias("c3", esper.Sum[int](intPrimitive)),
		).Query(esper.StatementName("s0"), esper.WithContext("EveryNowAndThen"),
			esper.WithOutput(esper.OutputSnapshotWhenTerminated()))
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		engine, statement, err := deployParityStatementAt(ctx, env, plan, scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		cleanup = false
		defer func() { _ = engine.Close(context.Background()) }()
		return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown context-init-term statement %q", name)
			}
			return statement, nil
		})
	case "filter-pattern-correlated":
		s1Base := esper.From[contextInitTermS1](env, "SupportBean_S1")
		end := esper.PatternFrom(s1Base, "s1",
			esper.Equal[*string](esper.Field[any, *string]("p10"),
				esper.Property[*string](esper.ContextInitiatingEvent(), "p00")))
		if _, err := esper.CreatePatternTerminatedContext(env, "EveryNowAndThen", esper.Literal("global"), isS0, end); err != nil {
			return compat.Trace{}, err
		}
		query := beanKeepAll.Aggregate(
			esper.Alias("c1", initiatingP00),
			esper.Alias("c2", esper.Sum[int](intPrimitive)),
		).Query(esper.StatementName("s0"), esper.WithContext("EveryNowAndThen"))
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		engine, statement, err := deployParityStatementAt(ctx, env, plan, scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		cleanup = false
		defer func() { _ = engine.Close(context.Background()) }()
		return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown context-init-term statement %q", name)
			}
			return statement, nil
		})
	case "filter-output-snapshot":
		start := isS0
		end := isS1
		if _, err := esper.CreateInitiatedTerminatedContext(env, "EveryNowAndThen", esper.Literal("global"), start, end); err != nil {
			return compat.Trace{}, err
		}
		query := beanKeepAll.Aggregate(
			esper.Alias("c1", initiatingP00),
			esper.Alias("c2", esper.Sum[int](intPrimitive)),
		).Query(esper.StatementName("s0"), esper.WithContext("EveryNowAndThen"),
			esper.WithOutput(esper.OutputSnapshotWhenTerminated()))
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		engine, statement, err := deployParityStatementAt(ctx, env, plan, scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		cleanup = false
		defer func() { _ = engine.Close(context.Background()) }()
		return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown context-init-term statement %q", name)
			}
			return statement, nil
		})
	case "pattern-pattern":
		s0Base := esper.From[contextInitTermS0](env, "SupportBean_S0")
		s1Base := esper.From[contextInitTermS1](env, "SupportBean_S1")
		start := esper.PatternFrom(s0Base, "s0", esper.Literal(true)).Then(esper.TimerInterval(s0Base, time.Second))
		end := esper.PatternFrom(s1Base, "s1", esper.Literal(true)).Then(esper.TimerInterval(s1Base, time.Second))
		if _, err := esper.CreatePatternInitiatedTerminatedContext(env, "EveryNowAndThen", start, end); err != nil {
			return compat.Trace{}, err
		}
		query := beanKeepAll.Aggregate(
			esper.Alias("c1", esper.ContextPatternField[string]("s0", "p00")),
			esper.Alias("c2", esper.Sum[int](intPrimitive)),
		).Query(esper.StatementName("s0"), esper.WithContext("EveryNowAndThen"))
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		engine, statement, err := deployParityStatementAt(ctx, env, plan, scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		cleanup = false
		defer func() { _ = engine.Close(context.Background()) }()
		return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown context-init-term statement %q", name)
			}
			return statement, nil
		})
	case "every-second":
		everySecond := esper.NewCronScheduleWithSeconds(esper.CronWildcard(), esper.CronWildcard(), esper.CronWildcard(),
			esper.CronWildcard(), esper.CronWildcard(), esper.CronWildcard())
		if _, err := esper.CreateCronTimeContext(env, "EverySecond", everySecond, everySecond); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Query(esper.StatementName("s0"), esper.WithContext("EverySecond"))
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		engine, statement, err := deployParityStatementAt(ctx, env, plan, scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		cleanup = false
		defer func() { _ = engine.Close(context.Background()) }()
		return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown context-init-term statement %q", name)
			}
			return statement, nil
		})
	case "daily-join":
		nine, _ := esper.NewTimeOfDay(9, 0, 0)
		five, _ := esper.NewTimeOfDay(17, 0, 0)
		if _, err := esper.CreateDailyTimeContext(env, "NineToFive", nine, five); err != nil {
			return compat.Trace{}, err
		}
		s0Base := esper.From[contextInitTermS0](env, "SupportBean_S0")
		join := esper.JoinChain(esper.JoinSource(beanKeepAll)).FullOuterJoin(esper.JoinSource(s0Base.Window(esper.KeepAll())),
			esper.OnSourcesEqual(0, esper.Field[any, string]("theString"), 1, esper.Field[any, string]("p00")))
		query := join.Select(
			esper.SelectFrom(0, "col1", esper.Field[any, string]("theString")),
			esper.SelectFrom(0, "col2", esper.Field[any, int]("intPrimitive")),
			esper.SelectFrom(1, "col3", esper.Field[any, int]("id")),
			esper.SelectFrom(1, "col4", esper.Field[any, string]("p00")),
		).Query(esper.StatementName("s0"), esper.WithContext("NineToFive"))
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		engine, statement, err := deployParityStatementAt(ctx, env, plan, scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		cleanup = false
		defer func() { _ = engine.Close(context.Background()) }()
		return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown context-init-term statement %q", name)
			}
			return statement, nil
		})
	case "daily-pattern-time":
		nine, _ := esper.NewTimeOfDay(9, 0, 0)
		five, _ := esper.NewTimeOfDay(17, 0, 0)
		if _, err := esper.CreateDailyTimeContext(env, "NineToFive", nine, five); err != nil {
			return compat.Trace{}, err
		}
		pattern := esper.TimerInterval(beanSource, 10*time.Second).Every()
		query := pattern.Query(esper.StatementName("s0"), esper.WithContext("NineToFive"))
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		engine, statement, err := deployParityStatementAt(ctx, env, plan, scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		cleanup = false
		defer func() { _ = engine.Close(context.Background()) }()
		return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown context-init-term statement %q", name)
			}
			return statement, nil
		})
	case "turned-off", "turned-on":
		nine, _ := esper.NewTimeOfDay(9, 0, 0)
		five, _ := esper.NewTimeOfDay(17, 0, 0)
		if _, err := esper.CreateDailyTimeContext(env, "NineToFive", nine, five); err != nil {
			return compat.Trace{}, err
		}
		buildA := func() (esper.Plan, error) {
			return env.Build(beanSource.Query(esper.StatementName("A"), esper.WithContext("NineToFive")))
		}
		buildB := func() (esper.Plan, error) {
			return env.Build(beanSource.Query(esper.StatementName("B"), esper.WithContext("NineToFive")))
		}
		buildC := func() (esper.Plan, error) {
			return env.Build(beanSource.Query(esper.StatementName("C"), esper.WithContext("NineToFive")))
		}
		initial, ok := initialAdvanceTime(scenario, caseName)
		if !ok {
			return compat.Trace{}, fmt.Errorf("context-init-term case %q has no initial time", caseName)
		}
		initialTime, err := time.Parse(time.RFC3339Nano, initial)
		if err != nil {
			return compat.Trace{}, err
		}
		engine = esper.NewEngine(env)
		if err := engine.AdvanceTime(ctx, initialTime); err != nil {
			return compat.Trace{}, err
		}
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
		planA, err := buildA()
		if err != nil {
			return compat.Trace{}, err
		}
		statementA, err := deploy(planA)
		if err != nil {
			return compat.Trace{}, err
		}
		var pendingB, pendingC bool
		if caseName == "turned-off" {
			pendingB, pendingC = true, true
		} else {
			pendingB = true
		}
		handlers := map[string]compat.StepHandler{
			"deploy": func(step compat.Step, attach func(*esper.Statement) error) ([]compat.TraceRecord, error) {
				switch step.Statement {
				case "B":
					if !pendingB {
						return nil, fmt.Errorf("statement B already deployed")
					}
					pendingB = false
					plan, err := buildB()
					if err != nil {
						return nil, err
					}
					statement, err := deploy(plan)
					if err != nil {
						return nil, err
					}
					return nil, attach(statement)
				case "C":
					if !pendingC {
						return nil, fmt.Errorf("statement C already deployed")
					}
					pendingC = false
					plan, err := buildC()
					if err != nil {
						return nil, err
					}
					statement, err := deploy(plan)
					if err != nil {
						return nil, err
					}
					return nil, attach(statement)
				default:
					return nil, fmt.Errorf("unknown deploy target %q", step.Statement)
				}
			},
		}
		cleanup = false
		defer func() { _ = engine.Close(context.Background()) }()
		return compat.ReplayWithStatementsAndHandlers(ctx, engine, statementA, caseScenario, decodeContextInitTermPayload, func(name string) (*esper.Statement, error) {
			if name != statementA.Name() {
				return nil, fmt.Errorf("unknown context-init-term statement %q", name)
			}
			return statementA, nil
		}, handlers)
	case "daily-agg-ungrouped":
		nine, _ := esper.NewTimeOfDay(9, 0, 0)
		five, _ := esper.NewTimeOfDay(17, 0, 0)
		if _, err := esper.CreateDailyTimeContext(env, "NineToFive", nine, five); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Aggregate(
			esper.Alias("c1", theString),
			esper.Alias("c2", esper.Sum[int](intPrimitive)),
		).Query(esper.StatementName("S1"), esper.WithContext("NineToFive"))
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		engine, statement, err := deployParityStatementAt(ctx, env, plan, scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		cleanup = false
		defer func() { _ = engine.Close(context.Background()) }()
		return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown context-init-term statement %q", name)
			}
			return statement, nil
		})
	case "daily-agg-grouped":
		eight, _ := esper.NewTimeOfDay(8, 0, 0)
		nine, _ := esper.NewTimeOfDay(9, 0, 0)
		if _, err := esper.CreateDailyTimeContext(env, "NestedContext", eight, nine); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.GroupBy(theString).Select(
			esper.Alias("c1", theString),
			esper.Alias("c2", esper.CountAll()),
		).Query(esper.StatementName("s0"), esper.WithContext("NestedContext"))
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		engine, statement, err := deployParityStatementAt(ctx, env, plan, scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		cleanup = false
		defer func() { _ = engine.Close(context.Background()) }()
		return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown context-init-term statement %q", name)
			}
			return statement, nil
		})
	default:
		return compat.Trace{}, fmt.Errorf("unsupported context-init-term-temporal-fixed case %q", caseName)
	}
}

// deployParityStatementAt deploys a statement after advancing the engine to
// the case's initial time, mirroring the Java oracle's deploy-time clock.
func deployParityStatementAt(ctx context.Context, env *esper.Environment, plan esper.Plan, scenario compat.Scenario, caseName string) (*esper.Engine, *esper.Statement, error) {
	initial, ok := initialAdvanceTime(scenario, caseName)
	if !ok {
		return nil, nil, fmt.Errorf("context-init-term case %q has no initial time", caseName)
	}
	initialTime, err := time.Parse(time.RFC3339Nano, initial)
	if err != nil {
		return nil, nil, err
	}
	engine := esper.NewEngine(env)
	if err := engine.AdvanceTime(ctx, initialTime); err != nil {
		_ = engine.Close(context.Background())
		return nil, nil, err
	}
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		_ = engine.Close(context.Background())
		return nil, nil, err
	}
	if len(deployment.Statements()) != 1 {
		_ = engine.Close(context.Background())
		return nil, nil, fmt.Errorf("expected one statement, got %d", len(deployment.Statements()))
	}
	return engine, deployment.Statements()[0], nil
}

func decodeContextInitTermPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextInitTermBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextInitTermS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value contextInitTermS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context-init-term event type %q", step.EventType)
	}
}
