package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type namedWindowMutationBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type namedWindowMutationRow struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type namedWindowMutationS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

const namedWindowMutationJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var namedWindowMutationJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnDelete.java",
}

var (
	namedWindowMutationJavaRuntimeIDs = []string{
		"java-runtime-62b4f6edf3c24229e8ac",
	}
	namedWindowMutationJavaExecutions = []string{
		"InfraFirstUnique",
	}
)

func runNamedWindowMutationScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "mutation"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("named window mutation scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[namedWindowMutationBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[namedWindowMutationS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	rowSchema, err := esper.RegisterStruct[namedWindowMutationRow](env, "MyWindowRow")
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateNamedWindow(env, "MyWindow", rowSchema,
		esper.NamedWindowRetention(esper.FirstUnique(esper.Field[namedWindowMutationRow, string]("theString")))); err != nil {
		return compat.Trace{}, err
	}
	insertPlan, err := env.Build(esper.OnEvent(esper.From[namedWindowMutationBean](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindow",
		esper.SetColumn("theString", esper.Field[namedWindowMutationBean, string]("theString")),
		esper.SetColumn("intPrimitive", esper.Field[namedWindowMutationBean, int]("intPrimitive")),
	).Query(esper.StatementName("insert")))
	if err != nil {
		return compat.Trace{}, err
	}
	deletePlan, err := env.Build(esper.OnEvent(esper.From[namedWindowMutationS0](env, "SupportBean_S0")).DeleteFromNamedWindow(
		"MyWindow",
		esper.Equal[string](esper.NamedWindowField[string]("theString"), esper.Field[namedWindowMutationS0, string]("p00")),
	).Query(esper.StatementName("delete")))
	if err != nil {
		return compat.Trace{}, err
	}
	createPlan, err := env.Build(esper.FromNamedWindow(env, "MyWindow").Select(
		esper.Alias("theString", esper.Field[any, string]("theString")),
		esper.Alias("intPrimitive", esper.Field[any, int]("intPrimitive")),
	).Query(esper.StatementName("create"), esper.WithOldStream()))
	if err != nil {
		return compat.Trace{}, err
	}
	countPlan, err := env.Build(esper.FromNamedWindow(env, "MyWindow").Aggregate(
		esper.Alias("cnt", esper.CountAll()),
	).Query(esper.StatementName("count")))
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env)
	cleanup := true
	defer func() {
		if cleanup {
			_ = engine.Close(context.Background())
		}
	}()
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
	createStatement, err := deploy(createPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	countStatement, err := deploy(countPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	cleanup = false
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, createStatement, caseScenario, decodeNamedWindowMutationPayload, func(name string) (*esper.Statement, error) {
		switch name {
		case createStatement.Name():
			return createStatement, nil
		case countStatement.Name():
			return countStatement, nil
		default:
			return nil, fmt.Errorf("unknown named window mutation statement %q", name)
		}
	}, countStatement)
}

func decodeNamedWindowMutationPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value namedWindowMutationBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value namedWindowMutationS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported named window mutation event type %q", step.EventType)
	}
}
