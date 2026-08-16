package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextInitTermOverlapBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     *int64 `esper:"intBoxed"`
	LongBoxed    *int64 `esper:"longBoxed"`
}

type contextInitTermOverlapS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 string  `esper:"p01"`
}

const contextInitTermOverlapJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextInitTermOverlapJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTerm.java",
}

var contextInitTermOverlapJavaRuntimeIDs = []string{
	"java-runtime-b8e68ca293fe524f9584",
	"java-runtime-23719b9e9c27c4c863d4",
}

var contextInitTermOverlapJavaExecutions = []string{
	"ContextInitTermCrontab",
	"ContextInitTermTerminateTwoContextSameTime",
}

// runContextInitTermOverlapDurationScenario replays the overlapping
// duration-terminated executions of ContextInitTerm: a context initiated by
// an every-minute cron pattern with three-minute duration termination
// (every event aggregates independently per active partition), and a
// context initiated by a filter with one-minute duration termination where
// two partitions end at the same instant.
func runContextInitTermOverlapDurationScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range []string{
		"crontab-minute",
		"two-contexts-same-time",
	} {
		caseTrace, err := runContextInitTermOverlapDurationCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-init-term-overlap-duration case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextInitTermOverlapDurationCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextInitTermOverlapBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermOverlapS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextInitTermOverlapBean](env, "SupportBean")

	var plan esper.Plan
	switch caseName {
	case "crontab-minute":
		everyMinute := esper.NewCronSchedule(esper.CronWildcard(), esper.CronWildcard(),
			esper.CronWildcard(), esper.CronWildcard(), esper.CronWildcard())
		start := esper.TimerCron(beanSource, everyMinute).Every()
		end := esper.TimerInterval(beanSource, 3*time.Minute)
		if _, err := esper.CreateOverlappingPatternInitiatedTerminatedContext(env, "EveryMinute", start, end); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Aggregate(
			esper.Alias("c1", esper.Field[contextInitTermOverlapBean, string]("theString")),
			esper.Alias("c2", esper.Sum[int](esper.Field[contextInitTermOverlapBean, int]("intPrimitive"))),
		).Query(esper.StatementName("s0"), esper.WithContext("EveryMinute"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "two-contexts-same-time":
		isS0 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
		end := esper.TimerInterval(beanSource, time.Minute)
		if _, err := esper.CreateOverlappingPatternTerminatedContext(env, "CtxInitiated", esper.Literal("global"), isS0, end); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Aggregate(
			esper.Alias("c1", esper.Field[contextInitTermOverlapBean, string]("theString")),
			esper.Alias("c2", esper.Sum[int](esper.Field[contextInitTermOverlapBean, int]("intPrimitive"))),
			esper.Alias("c3", esper.Property[*string](esper.ContextInitiatingEvent(), "p00")),
		).Query(esper.StatementName("s0"), esper.WithContext("CtxInitiated"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported context-init-term-overlap-duration case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermOverlapPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-init-term-overlap-duration statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextInitTermOverlapPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextInitTermOverlapBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextInitTermOverlapS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context-init-term-overlap-duration event type %q", step.EventType)
	}
}
