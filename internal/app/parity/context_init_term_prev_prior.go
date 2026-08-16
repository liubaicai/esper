package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextInitTermPrevPriorBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const contextInitTermPrevPriorJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextInitTermPrevPriorJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTerm.java",
}

var contextInitTermPrevPriorJavaRuntimeIDs = []string{
	"java-runtime-962cedd069e3beabeeec", // ContextInitTermPrevPrior
}

var contextInitTermPrevPriorJavaExecutions = []string{
	"ContextInitTermPrevPrior",
}

// runContextInitTermPrevPriorScenario replays the prev/prior execution of
// ContextInitTerm: a daily 9-to-5 context with a keepall window projecting
// prev/prevwindow/prevtail/prior window functions and a running sum. The
// first day accumulates E1/E2, the window closes at 17:00, and the second
// day restarts with a fresh partition where E3 is the first event again.
func runContextInitTermPrevPriorScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextInitTermPrevPriorCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-init-term-prev-prior case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextInitTermPrevPriorCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextInitTermPrevPriorBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextInitTermPrevPriorBean](env, "SupportBean")
	theString := esper.Field[contextInitTermPrevPriorBean, string]("theString")
	intPrimitive := esper.Field[contextInitTermPrevPriorBean, int]("intPrimitive")

	nine, _ := esper.NewTimeOfDay(9, 0, 0)
	five, _ := esper.NewTimeOfDay(17, 0, 0)
	if _, err := esper.CreateDailyTimeContext(env, "NineToFive", nine, five); err != nil {
		return compat.Trace{}, err
	}
	query := beanSource.Window(esper.KeepAll()).Aggregate(
		esper.Alias("col1", esper.Prev[string](1, theString)),
		esper.Alias("col2", esper.PrevWindow[esper.Event](esper.EventValue[esper.Event]())),
		esper.Alias("col3", esper.PrevTail[string](0, theString)),
		esper.Alias("col4", esper.Prior[string](0, theString)),
		esper.Alias("col5", esper.Sum[int](intPrimitive)),
	).Query(esper.StatementName("s0"), esper.WithContext("NineToFive"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermPrevPriorPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-init-term-prev-prior statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextInitTermPrevPriorPayload(step compat.Step) (any, error) {
	var value contextInitTermPrevPriorBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
