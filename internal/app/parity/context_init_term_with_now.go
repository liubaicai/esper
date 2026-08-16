package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextInitTermWithNowBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     *int64 `esper:"intBoxed"`
	LongBoxed    *int64 `esper:"longBoxed"`
}

const contextInitTermWithNowJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextInitTermWithNowJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermWithNow.java",
}

var contextInitTermWithNowJavaRuntimeIDs = []string{
	"java-runtime-8e440e320bbbb03c15d2",
	"java-runtime-20229f5d0bc028d126fc",
	"java-runtime-8f7f85ae13613f8e5083",
}

var contextInitTermWithNowJavaExecutions = []string{
	"ContextStartStopWNow",
	"ContextInitTermWithPattern",
	"ContextInitTermWNowNoEnd",
}

// runContextInitTermWithNowScenario replays the @now executions of
// ContextInitTermWithNow: a time-period context starting immediately with
// output last when terminated (including an empty cycle emitting a zero
// count), a pattern context initiated by an immediate timer OR an
// every-ten-seconds timer with ten-second duration termination, and a
// never-ending immediate context accumulating counts.
func runContextInitTermWithNowScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range []string{
		"start-stop-now",
		"initiated-now-pattern",
		"now-no-end",
	} {
		caseTrace, err := runContextInitTermWithNowCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-init-term-with-now case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextInitTermWithNowCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextInitTermWithNowBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextInitTermWithNowBean](env, "SupportBean")

	var plan esper.Plan
	switch caseName {
	case "start-stop-now":
		if _, err := esper.CreateTimePeriodContext(env, "MyContext", 0, 10*time.Second); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Aggregate(
			esper.Alias("cnt", esper.CountAll()),
		).Query(esper.StatementName("s0"), esper.WithContext("MyContext"),
			esper.WithOutput(esper.OutputWhenTerminated(esper.OutputLast())))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "initiated-now-pattern":
		start := esper.TimerInterval(beanSource, 0).Or(esper.TimerInterval(beanSource, 10*time.Second).Every())
		end := esper.TimerInterval(beanSource, 10*time.Second)
		if _, err := esper.CreateOverlappingPatternInitiatedTerminatedContext(env, "MyContext", start, end); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Aggregate(
			esper.Alias("cnt", esper.CountAll()),
		).Query(esper.StatementName("s0"), esper.WithContext("MyContext"),
			esper.WithOutput(esper.OutputWhenTerminated(esper.OutputLast())))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "now-no-end":
		if _, err := esper.CreatePatternInitiatedContext(env, "MyContext", esper.TimerInterval(beanSource, 0)); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Aggregate(
			esper.Alias("cnt", esper.CountAll()),
		).Query(esper.StatementName("s0"), esper.WithContext("MyContext"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported context-init-term-with-now case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermWithNowPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-init-term-with-now statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextInitTermWithNowPayload(step compat.Step) (any, error) {
	var value contextInitTermWithNowBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
