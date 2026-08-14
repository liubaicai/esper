package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextOutputBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const contextOutputJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextOutputJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTerm.java",
}

var (
	contextOutputJavaRuntimeIDs = []string{
		"java-runtime-4b19317271be7df8b569",
	}
	contextOutputJavaExecutions = []string{
		"ContextInitTermOutputSnapshotWhenTerminated",
	}
)

func runContextOutputScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "termination"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("context output scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextOutputBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	base := esper.From[contextOutputBean](env, "SupportBean")
	schedule := esper.CronSchedule{
		Minute:     esper.CronEvery(1),
		Hour:       esper.CronWildcard(),
		DayOfMonth: esper.CronWildcard(),
		Month:      esper.CronWildcard(),
		Weekday:    esper.CronWildcard(),
	}
	start := esper.TimerCron(base, schedule).Every()
	end := esper.TimerInterval(base, time.Minute)
	if _, err := esper.CreatePatternInitiatedTerminatedContext(env, "EveryMinute", start, end); err != nil {
		return compat.Trace{}, err
	}
	query := base.Aggregate(
		esper.Alias("c1", esper.Sum[int](esper.Field[contextOutputBean, int]("intPrimitive"))),
	).Query(
		esper.StatementName("s0"),
		esper.WithContext("EveryMinute"),
		esper.WithOutput(esper.OutputSnapshotWhenTerminated()),
	)
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithStartTime(time.Date(2002, 5, 1, 8, 0, 0, 0, time.UTC)))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	if len(deployment.Statements()) != 1 {
		_ = engine.Close(context.Background())
		return compat.Trace{}, fmt.Errorf("expected one statement, got %d", len(deployment.Statements()))
	}
	statement := deployment.Statements()[0]

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextOutputPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context output statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextOutputPayload(step compat.Step) (any, error) {
	var value contextOutputBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
