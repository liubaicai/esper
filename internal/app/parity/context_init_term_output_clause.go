package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextInitTermOutputBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     *int64 `esper:"intBoxed"`
	ShortBoxed   *int64 `esper:"shortBoxed"`
	LongBoxed    *int64 `esper:"longBoxed"`
}

type contextInitTermOutputS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 string  `esper:"p01"`
}

const contextInitTermOutputJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextInitTermOutputJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTerm.java",
}

var contextInitTermOutputJavaRuntimeIDs = []string{
	"java-runtime-81ca9d4f0d34ba460d2d",
	"java-runtime-2e991ab4c62f16b80a30",
	"java-runtime-67c0d47278e45cc8407a",
	"java-runtime-61d4f2f4e683b9fbac02",
	"java-runtime-d8420feb762d4f29563a",
}

var contextInitTermOutputJavaExecutions = []string{
	"ContextInitTermOutputAllEvery2AndTerminated",
	"ContextInitTermOutputWhenExprWhenTerminatedCondition",
	"ContextInitTermOutputOnlyWhenTerminatedCondition",
	"ContextInitTermOutputOnlyWhenSetAndWhenTerminatedSet",
	"ContextInitTermOutputOnlyWhenTerminatedThenSet",
}

// runContextInitTermOutputClauseScenario replays the five output-clause
// executions of ContextInitTerm, all driven by the every-minute context
// (initiated by pattern [every timer:at(*, *, *, *, *)] terminated after
// 1 min): output all every 2 events and when terminated with group-by and
// order-by, output when count_insert with a when-terminated condition,
// output only when terminated with count_insert, output when true with
// then-set and when-terminated then-set variable assignments, and output
// only when terminated with a then-set assignment.
func runContextInitTermOutputClauseScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextInitTermOutputClauseCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-init-term-output-clause case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextInitTermOutputClauseCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextInitTermOutputBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermOutputS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextInitTermOutputBean](env, "SupportBean")
	isS0 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
	theString := esper.Field[contextInitTermOutputBean, string]("theString")
	intPrimitive := esper.Field[contextInitTermOutputBean, int]("intPrimitive")

	everyMinute := esper.NewCronScheduleWithSeconds(
		esper.CronValues(0),
		esper.CronWildcard(),
		esper.CronWildcard(),
		esper.CronWildcard(),
		esper.CronWildcard(),
		esper.CronWildcard(),
	)

	var plan esper.Plan
	var variableName string
	switch caseName {
	case "op-all-every2-terminated":
		if _, err := esper.CreateOverlappingPatternInitiatedTerminatedContext(env, "EveryMinute",
			esper.TimerCron(beanSource, everyMinute).Every(), esper.TimerInterval(beanSource, time.Minute)); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.GroupBy(theString).Select(
			esper.Alias("c1", theString),
			esper.Alias("c2", esper.Sum[int](intPrimitive)),
		).Query(
			esper.StatementName("s0"),
			esper.WithContext("EveryMinute"),
			esper.WithOutput(esper.OutputAndWhenTerminated(esper.OutputAllEveryEvents(2))),
			esper.OrderBy(esper.Ascending(esper.ResultField[string]("c1"))),
		)
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "op-when-expr-when-terminated":
		if _, err := esper.CreateOverlappingPatternInitiatedTerminatedContext(env, "EveryMinute",
			esper.TimerCron(beanSource, everyMinute).Every(), esper.TimerInterval(beanSource, time.Minute)); err != nil {
			return compat.Trace{}, err
		}
		countInsert := esper.OutputCountInsert()
		policy := esper.OutputAndWhenTerminatedIf(
			esper.OutputWhen(esper.GreaterOf(countInsert, esper.Literal(1))),
			esper.GreaterOf(countInsert, esper.Literal(0)),
		)
		query := beanSource.Aggregate(esper.Alias("c0", theString)).Query(
			esper.StatementName("s0"),
			esper.WithContext("EveryMinute"),
			esper.WithOutput(policy),
		)
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "op-only-when-terminated":
		if _, err := esper.CreateOverlappingPatternInitiatedTerminatedContext(env, "EveryMinute",
			esper.TimerCron(beanSource, everyMinute).Every(), esper.TimerInterval(beanSource, time.Minute)); err != nil {
			return compat.Trace{}, err
		}
		policy := esper.OutputWhenTerminatedIf(esper.GreaterOf(esper.OutputCountInsert(), esper.Literal(0)))
		query := esper.Select(beanSource, esper.Alias("c0", theString)).Query(
			esper.StatementName("s0"),
			esper.WithContext("EveryMinute"),
			esper.WithOutput(policy),
		)
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "op-when-set-variable":
		variableName = "myvar"
		if err := env.RegisterVariable("myvar", 0); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateOverlappingPatternInitiatedTerminatedContext(env, "EveryMinute",
			esper.TimerCron(beanSource, everyMinute).Every(), esper.TimerInterval(beanSource, time.Minute)); err != nil {
			return compat.Trace{}, err
		}
		base := esper.OutputWhenWith(esper.OutputAll(), esper.Literal(true), esper.SetOutputVariable("myvar", esper.Literal(1)))
		policy := esper.OutputAndWhenTerminatedIf(base, nil, esper.SetOutputVariable("myvar", esper.Literal(2)))
		query := esper.Select(beanSource, esper.Alias("c0", theString)).Query(
			esper.StatementName("s0"),
			esper.WithContext("EveryMinute"),
			esper.WithOutput(policy),
		)
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "op-only-terminated-set":
		variableName = "myvar"
		if err := env.RegisterVariable("myvar", 0); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateOverlappingPatternTerminatedContext(env, "EverySupportBeanS0", esper.Literal("global"), isS0,
			esper.TimerInterval(beanSource, time.Minute)); err != nil {
			return compat.Trace{}, err
		}
		policy := esper.OutputWhenTerminatedIf(nil, esper.SetOutputVariable("myvar", esper.Literal(10)))
		query := esper.Select(beanSource, esper.Alias("c0", theString)).Query(
			esper.StatementName("s0"),
			esper.WithContext("EverySupportBeanS0"),
			esper.WithOutput(policy),
		)
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported context-init-term-output-clause case %q", caseName)
	}

	engine := esper.NewEngine(env)
	if initial, ok := initialAdvanceTime(scenario, caseName); ok {
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
	handlers := map[string]compat.StepHandler{}
	if variableName != "" {
		name := variableName
		handlers["read-variable"] = func(step compat.Step, _ func(*esper.Statement) error) ([]compat.TraceRecord, error) {
			value, ok := engine.GetVariable(name)
			if !ok {
				return nil, fmt.Errorf("variable %q not found", name)
			}
			record := compat.TraceRecord{
				Case:      step.Case,
				Operation: "variable",
				Name:      name,
				Value:     int64(0),
			}
			if number, ok := value.Any().(json.Number); ok {
				parsed, err := number.Int64()
				if err == nil {
					record.Value = parsed
				}
			} else if number, ok := value.Any().(int64); ok {
				record.Value = number
			} else if number, ok := value.Any().(int); ok {
				record.Value = int64(number)
			}
			return []compat.TraceRecord{record}, nil
		}
	}
	return compat.ReplayWithStatementsAndHandlers(ctx, engine, statement, caseScenario, decodeContextInitTermOutputPayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown context-init-term-output-clause statement %q", name)
			}
			return statement, nil
		}, handlers)
}

func decodeContextInitTermOutputPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextInitTermOutputBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextInitTermOutputS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context-init-term-output-clause event type %q", step.EventType)
	}
}
