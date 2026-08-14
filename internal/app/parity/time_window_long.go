package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type timeWindowBean struct {
	TheString string `esper:"theString"`
	LongBoxed int64  `esper:"longBoxed"`
}

type timeWindowRow struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

type timeWindowMarket struct {
	Symbol string `esper:"symbol"`
}

const timeWindowJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var timeWindowJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java",
}

var (
	timeWindowJavaRuntimeIDs = []string{
		"java-runtime-d00485a2855b2128289f",
	}
	timeWindowJavaExecutions = []string{
		"InfraTimeWindow",
	}
)

func runTimeWindowScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "time-window"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("time window scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[timeWindowBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[timeWindowMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	rowSchema, err := esper.RegisterStruct[timeWindowRow](env, "MyWindowRow")
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateNamedWindow(env, "MyWindowTW", rowSchema,
		esper.NamedWindowRetention(esper.TimeWindow(10*time.Second))); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[timeWindowBean](env, "SupportBean")
	insertPlan, err := env.Build(esper.OnEvent(beanSource).InsertIntoNamedWindow(
		"MyWindowTW",
		esper.SetColumn("key", esper.Field[timeWindowBean, string]("theString")),
		esper.SetColumn("value", esper.Field[timeWindowBean, int64]("longBoxed")),
	).Query(esper.StatementName("insert")))
	if err != nil {
		return compat.Trace{}, err
	}
	deletePlan, err := env.Build(esper.OnEvent(esper.From[timeWindowMarket](env, "SupportMarketDataBean")).DeleteFromNamedWindow(
		"MyWindowTW",
		esper.Equal[string](esper.NamedWindowField[string]("key"), esper.Field[timeWindowMarket, string]("symbol")),
	).Query(esper.StatementName("delete")))
	if err != nil {
		return compat.Trace{}, err
	}
	consumerPlan, err := env.Build(esper.FromNamedWindow(env, "MyWindowTW").Select(
		esper.Alias("key", esper.Field[any, string]("key")),
		esper.Alias("value", esper.Field[any, int64]("value")),
	).Query(esper.StatementName("s0"), esper.WithOldStream()))
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
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
	if _, err := deploy(insertPlan); err != nil {
		return compat.Trace{}, err
	}
	if _, err := deploy(deletePlan); err != nil {
		return compat.Trace{}, err
	}
	statement, err := deploy(consumerPlan)
	if err != nil {
		return compat.Trace{}, err
	}

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeTimeWindowPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown time window statement %q", name)
		}
		return statement, nil
	})
}

func decodeTimeWindowPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value timeWindowBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportMarketDataBean":
		var value timeWindowMarket
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported time window event type %q", step.EventType)
	}
}
