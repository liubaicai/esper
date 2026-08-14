package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type tableMutationBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int64  `esper:"intPrimitive"`
}

type tableMutationS0 struct {
	ID  int64  `esper:"id"`
	P00 string `esper:"p00"`
}

type tableMutationTwoKey struct {
	K1       string `esper:"k1"`
	K2       int64  `esper:"k2"`
	NewValue int64  `esper:"newValue"`
}

const tableMutationJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var tableMutationJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableOnUpdate.java",
}

var (
	tableMutationJavaRuntimeIDs = []string{
		"java-runtime-3707b3fa08171bbd791c",
	}
	tableMutationJavaExecutions = []string{
		"InfraTableOnUpdateTwoKey",
	}
)

func runTableMutationScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "mutation"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("table mutation scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[tableMutationBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[tableMutationS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[tableMutationTwoKey](env, "SupportTwoKeyEvent"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateTable(env, "varagg", []esper.TableColumn{
		esper.PrimaryKeyColumn[string]("keyOne"),
		esper.PrimaryKeyColumn[int64]("keyTwo"),
		esper.TableColumnOf[int64]("p0"),
	}); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[tableMutationBean](env, "SupportBean")
	mergePlan, err := env.Build(esper.OnEvent(beanSource).MergeIntoTableWhen("varagg", []esper.Expr{
		esper.Field[tableMutationBean, string]("theString"),
		esper.Field[tableMutationBean, int64]("intPrimitive"),
	}, esper.WhenNotMatchedAny(
		esper.SetColumn("keyOne", esper.Field[tableMutationBean, string]("theString")),
		esper.SetColumn("keyTwo", esper.Field[tableMutationBean, int64]("intPrimitive")),
		esper.SetColumn("p0", esper.Literal[int64](1)),
	)).Query(esper.StatementName("merge")))
	if err != nil {
		return compat.Trace{}, err
	}
	readPlan, err := env.Build(esper.OnEvent(esper.From[tableMutationS0](env, "SupportBean_S0")).SelectFromTable(
		"varagg",
		[]esper.Expr{
			esper.Field[tableMutationS0, string]("p00"),
			esper.Field[tableMutationS0, int64]("id"),
		},
		esper.Alias("value", esper.TableField[int64]("p0")),
	).Query(esper.StatementName("s0")))
	if err != nil {
		return compat.Trace{}, err
	}
	twoKeySource := esper.From[tableMutationTwoKey](env, "SupportTwoKeyEvent")
	updatePlan, err := env.Build(esper.OnEvent(twoKeySource).UpdateTable(
		"varagg",
		[]esper.Expr{
			esper.Field[tableMutationTwoKey, string]("k1"),
			esper.Field[tableMutationTwoKey, int64]("k2"),
		},
		esper.SetColumn("p0", esper.Field[tableMutationTwoKey, int64]("newValue")),
	).Query(esper.StatementName("update")))
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
	if _, err := deploy(mergePlan); err != nil {
		return compat.Trace{}, err
	}
	readStatement, err := deploy(readPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	updateStatement, err := deploy(updatePlan)
	if err != nil {
		return compat.Trace{}, err
	}

	return compat.ReplayWithStatements(ctx, engine, readStatement, caseScenario, decodeTableMutationPayload, func(name string) (*esper.Statement, error) {
		switch name {
		case readStatement.Name():
			return readStatement, nil
		case updateStatement.Name():
			return updateStatement, nil
		default:
			return nil, fmt.Errorf("unknown table mutation statement %q", name)
		}
	}, updateStatement)
}

func decodeTableMutationPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value tableMutationBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value tableMutationS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportTwoKeyEvent":
		var value tableMutationTwoKey
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportTwoKeyEvent: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported table mutation event type %q", step.EventType)
	}
}
