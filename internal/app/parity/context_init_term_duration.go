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

type contextInitTermDurationBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type contextInitTermDurationS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 string  `esper:"p01"`
}

type contextInitTermDurationS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 string  `esper:"p11"`
}

const contextInitTermDurationJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextInitTermDurationJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTerm.java",
}

var contextInitTermDurationJavaRuntimeIDs = []string{
	"java-runtime-e9c2321d3689d73c463c", // ContextStartEndAfterZeroInitiatedNow
	"java-runtime-1faa1af2a6dab9b91347", // ContextStartEndEndSameEventAsAnalyzed
	"java-runtime-67e4f12d8dc646537774", // ContextInitTermFilterInitiatedFilterAllTerminated
	"java-runtime-bdc0590b7c930594e815", // ContextInitTermFilterAndAfter1Min
	"java-runtime-54ff8b2b9b95588a0465", // ContextInitTermPatternIntervalZeroInitiatedNow
	"java-runtime-45fe11b7a30cf8fc8c79", // ContextStartEndStartNowCalMonthScoped
}

var contextInitTermDurationJavaExecutions = []string{
	"ContextStartEndAfterZeroInitiatedNow",
	"ContextStartEndEndSameEventAsAnalyzed",
	"ContextInitTermFilterInitiatedFilterAllTerminated",
	"ContextInitTermFilterAndAfter1Min",
	"ContextInitTermPatternIntervalZeroInitiatedNow",
	"ContextStartEndStartNowCalMonthScoped",
}

// runContextInitTermDurationScenario replays the duration-terminated
// ContextInitTerm executions: a time-period context starting after zero
// seconds (end instant excluded), same-event termination with and without an
// insert-into terminator, a filter-all-terminated overlapping context, a
// one-minute-duration initiated context, a one-shot OR pattern start, and a
// calendar-month end condition.
func runContextInitTermDurationScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextInitTermDurationCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-init-term-duration case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextInitTermDurationCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextInitTermDurationBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermDurationS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermDurationS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextInitTermDurationBean](env, "SupportBean")
	theString := esper.Field[contextInitTermDurationBean, string]("theString")
	intPrimitive := esper.Field[contextInitTermDurationBean, int]("intPrimitive")
	isBean := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean"))
	isS0 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
	isS1 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S1"))

	var plan esper.Plan
	var insertPlan esper.Plan
	hasInsertPlan := false
	switch caseName {
	case "start-after-zero":
		if _, err := esper.CreateScheduledTimePeriodContext(env, "CtxPerId", 0, 60*time.Second); err != nil {
			return compat.Trace{}, err
		}
		query := esper.Select(beanSource,
			esper.Alias("c0", theString),
			esper.Alias("c1", intPrimitive),
		).Query(esper.StatementName("s0"), esper.WithContext("CtxPerId"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "same-event-not-included":
		end := esper.And(isBean, esper.Equal[int](intPrimitive, esper.Literal(11)))
		if _, err := esper.CreateInitiatedTerminatedContext(env, "MyCtx", esper.Literal("global"), isBean, end); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Aggregate(
			esper.Alias("c1", esper.Min[int](intPrimitive)),
			esper.Alias("c2", esper.Max[int](intPrimitive)),
			esper.Alias("c3", esper.Sum[int](intPrimitive)),
			esper.Alias("c4", esper.Avg[int](intPrimitive)),
		).Query(esper.StatementName("s0"), esper.WithContext("MyCtx"),
			esper.WithOutput(esper.OutputSnapshotWhenTerminated()))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "same-event-included":
		if _, err := esper.RegisterMap(env, "MyCtxTerminate", []esper.FieldSpec{esper.FieldDef("theString", reflect.TypeOf(""))}); err != nil {
			return compat.Trace{}, err
		}
		isTerminate := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("MyCtxTerminate"))
		if _, err := esper.CreateInitiatedTerminatedContext(env, "MyCtx", esper.Literal("global"), isBean, isTerminate); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Aggregate(
			esper.Alias("c1", esper.Min[int](intPrimitive)),
			esper.Alias("c2", esper.Max[int](intPrimitive)),
			esper.Alias("c3", esper.Sum[int](intPrimitive)),
			esper.Alias("c4", esper.Avg[int](intPrimitive)),
		).Query(esper.StatementName("s0"), esper.WithContext("MyCtx"),
			esper.WithOutput(esper.OutputSnapshotWhenTerminated()))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		insertPlan, err = env.Build(beanSource.Filter(
			esper.Equal[int](intPrimitive, esper.Literal(11)),
		).AsRecord().Select(
			esper.Alias("theString", theString),
		).InsertInto("MyCtxTerminate", esper.StatementName("i0")))
		if err != nil {
			return compat.Trace{}, err
		}
		hasInsertPlan = true
	case "filter-all-terminated":
		if _, err := esper.CreateOverlappingInitiatedTerminatedContext(env, "MyContext", esper.Literal("global"), isS0, isS1); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Aggregate(
			esper.Alias("c1", esper.Sum[int](intPrimitive)),
		).Query(esper.StatementName("s0"), esper.WithContext("MyContext"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "filter-after-1-min":
		if _, err := esper.CreateOverlappingPatternTerminatedContext(env, "CtxInitiated", esper.Literal("global"), isS0,
			esper.TimerInterval(beanSource, time.Minute)); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Aggregate(
			esper.Alias("c1", theString),
			esper.Alias("c2", esper.Sum[int](intPrimitive)),
			esper.Alias("c3", esper.Property[*string](esper.ContextInitiatingEvent(), "p00")),
		).Query(esper.StatementName("S1"), esper.WithContext("CtxInitiated"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "pattern-interval-zero":
		start := esper.TimerInterval(beanSource, 0).OrExclusive(esper.TimerInterval(beanSource, time.Minute).Every())
		end := esper.TimerInterval(beanSource, 60*time.Second)
		if _, err := esper.CreateOverlappingPatternInitiatedTerminatedContext(env, "CtxPerId", start, end); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Aggregate(
			esper.Alias("c0", theString),
			esper.Alias("c1", esper.Sum[int](intPrimitive)),
		).Query(esper.StatementName("s0"), esper.WithContext("CtxPerId"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "cal-month-scoped":
		if _, err := esper.CreatePatternTerminatedContext(env, "MyCtx", esper.Literal("global"), isS1,
			esper.TimerIntervalCalendar(beanSource, 0, 1, 0)); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Query(esper.StatementName("s0"), esper.WithContext("MyCtx"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported context-init-term-duration case %q", caseName)
	}

	engine := esper.NewEngine(env)
	initial, ok := initialAdvanceTime(scenario, caseName)
	if ok {
		initialTime, err := time.Parse(time.RFC3339Nano, initial)
		if err != nil {
			_ = engine.Close(context.Background())
			return compat.Trace{}, err
		}
		if err := engine.AdvanceTime(ctx, initialTime); err != nil {
			_ = engine.Close(context.Background())
			return compat.Trace{}, err
		}
	}
	if hasInsertPlan {
		// The insert-into terminator must be deployed before the consuming
		// context statement so the routed event reaches the end condition.
		if _, err := engine.Deploy(ctx, insertPlan); err != nil {
			_ = engine.Close(context.Background())
			return compat.Trace{}, err
		}
	}
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		_ = engine.Close(context.Background())
		return compat.Trace{}, err
	}
	if len(deployment.Statements()) != 1 {
		_ = engine.Close(context.Background())
		return compat.Trace{}, fmt.Errorf("expected one statement, got %d", len(deployment.Statements()))
	}
	statement := deployment.Statements()[0]
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermDurationPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-init-term-duration statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextInitTermDurationPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextInitTermDurationBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextInitTermDurationS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value contextInitTermDurationS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context-init-term-duration event type %q", step.EventType)
	}
}
