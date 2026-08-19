package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type infraNWTableSubqueryBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     int    `esper:"intBoxed"`
}

type infraNWTableSubqueryS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type infraNWTableSubqueryA struct {
	ID string `esper:"id"`
}

const infraNWTableSubqueryJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var infraNWTableSubqueryJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubquery.java",
}

var infraNWTableSubqueryJavaRuntimeIDs = []string{
	"java-runtime-cf539c3cb3727c0a0e6e",
	"java-runtime-a88f8ac757f9b7de47a6",
	"java-runtime-5d7bf128b2789f223a55",
	"java-runtime-6134b1502baf01472daf",
}

var infraNWTableSubqueryJavaExecutions = []string{
	"InfraSubquerySceneOne{namedWindow=true}",
	"InfraSubquerySceneOne{namedWindow=false}",
	"InfraSubquerySelfCheck{namedWindow=true}",
	"InfraSubquerySelfCheck{namedWindow=false}",
}

var infraNWTableSubqueryCases = []string{
	"scene-one-named-window",
	"scene-one-table",
	"self-check-named-window",
	"self-check-table",
}

func runInfraNWTableSubqueryScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range infraNWTableSubqueryCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseTrace, err := runInfraNWTableSubqueryCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("infra NW/Table subquery case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("infra NW/Table subquery scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runInfraNWTableSubqueryCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	namedWindow := caseName == "scene-one-named-window" || caseName == "self-check-named-window"
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableSubqueryBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableSubqueryS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableSubqueryA](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}
	if isSceneOneCase(caseName) {
		return runInfraNWTableSubquerySceneOne(ctx, env, caseScenario, namedWindow, infraNWTableSubqueryRuntimeID(caseName))
	}
	return runInfraNWTableSubquerySelfCheck(ctx, env, caseScenario, namedWindow, infraNWTableSubqueryRuntimeID(caseName))
}

func runInfraNWTableSubquerySceneOne(ctx context.Context, env *esper.Environment, scenario compat.Scenario, namedWindow bool, runtimeID string) (compat.Trace, error) {
	name := "MyInfra"
	source := esper.From[infraNWTableSubqueryBean](env, "SupportBean")
	assignments := []esper.TableAssignment{
		esper.SetColumn("theString", esper.Field[infraNWTableSubqueryBean, string]("theString")),
		esper.SetColumn("intPrimitive", esper.Field[infraNWTableSubqueryBean, int]("intPrimitive")),
	}
	var insertPlan esper.Plan
	var err error
	if namedWindow {
		schema, ok := env.Schema("SupportBean")
		if !ok {
			return compat.Trace{}, fmt.Errorf("SupportBean schema is missing")
		}
		if _, err := esper.CreateNamedWindow(env, name, schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		insertPlan, err = env.Build(esper.OnEvent(source).InsertIntoNamedWindow(name, assignments...).Query(esper.StatementName("Insert")))
	} else {
		if _, err := esper.CreateTable(env, name, []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("theString"),
			esper.TableColumnOf[int]("intPrimitive"),
		}); err != nil {
			return compat.Trace{}, err
		}
		insertPlan, err = env.Build(esper.OnEvent(source).InsertIntoTable(name, assignments...).Query(esper.StatementName("Insert")))
	}
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithRuntimeURI(runtimeID))
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := engine.Deploy(ctx, insertPlan); err != nil {
		return compat.Trace{}, err
	}

	// Java deploys Subq only after A1/B2/C3 have been inserted.
	var triggerSteps []compat.Step
	triggerSteps = append(triggerSteps, compat.Step{Op: "case", Case: scenario.Steps[0].Case})
	for _, step := range scenario.Steps[1:] {
		if step.Op == "send" && step.EventType == "SupportBean" {
			payload, decodeErr := decodeInfraNWTableSubqueryPayload(step)
			if decodeErr != nil {
				return compat.Trace{}, decodeErr
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
			continue
		}
		if step.Op == "send" && step.EventType == "SupportBean_S0" {
			triggerSteps = append(triggerSteps, step)
		}
	}
	var inner esper.RecordStream
	if namedWindow {
		inner = esper.FromNamedWindow(env, name)
	} else {
		inner = esper.FromTable(env, name)
	}
	querySource := esper.From[infraNWTableSubqueryS0](env, "SupportBean_S0")
	query := esper.Select(querySource,
		esper.Alias("c0", esper.SubqueryValue[int](inner, esper.Field[any, int]("intPrimitive"),
			esper.Equal[string](esper.Field[any, string]("theString"), esper.OuterField[string]("p00")))),
	).Query(esper.StatementName("Subq"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, fmt.Errorf("build SceneOne query: %w", err)
	}
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, fmt.Errorf("deploy SceneOne query: %w", err)
	}
	if len(deployment.Statements()) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one Subq statement")
	}
	triggerScenario := compat.Scenario{Version: scenario.Version, ID: scenario.ID, Steps: triggerSteps}
	return compat.ReplayWithStatements(ctx, engine, deployment.Statements()[0], triggerScenario, decodeInfraNWTableSubqueryPayload, func(statementName string) (*esper.Statement, error) {
		if statementName != "Subq" {
			return nil, fmt.Errorf("unknown SceneOne statement %q", statementName)
		}
		return deployment.Statements()[0], nil
	})
}

func runInfraNWTableSubquerySelfCheck(ctx context.Context, env *esper.Environment, scenario compat.Scenario, namedWindow bool, runtimeID string) (compat.Trace, error) {
	var inner esper.RecordStream
	if namedWindow {
		schema, err := esper.NewMapSchema("InfraSelfCheckSchema", []esper.FieldSpec{
			esper.FieldDef("key", reflectTypeString),
			esper.FieldDef("value", reflectTypeInt),
		})
		if err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterSchema(schema); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyInfraSSS", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		inner = esper.FromNamedWindow(env, "MyInfraSSS")
	} else {
		if _, err := esper.CreateTable(env, "MyInfraSSS", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("key"),
			esper.TableColumnOf[int]("value"),
		}); err != nil {
			return compat.Trace{}, err
		}
		inner = esper.FromTable(env, "MyInfraSSS")
	}

	source := esper.From[infraNWTableSubqueryBean](env, "SupportBean")
	insertSource := source.Filter(esper.Not(esper.SubqueryExists(inner,
		esper.Equal[string](esper.Field[any, string]("key"), esper.OuterField[string]("theString")),
	)))
	assignments := []esper.TableAssignment{
		esper.SetColumn("key", esper.Field[infraNWTableSubqueryBean, string]("theString")),
		esper.SetColumn("value", esper.Field[infraNWTableSubqueryBean, int]("intBoxed")),
	}
	var insertPlan esper.Plan
	var err error
	if namedWindow {
		insertPlan, err = env.Build(esper.OnEvent(insertSource).InsertIntoNamedWindow("MyInfraSSS", assignments...).Query(esper.StatementName("insert")))
	} else {
		insertPlan, err = env.Build(esper.OnEvent(insertSource).InsertIntoTable("MyInfraSSS", assignments...).Query(esper.StatementName("insert")))
	}
	if err != nil {
		return compat.Trace{}, err
	}
	deleteSource := esper.From[infraNWTableSubqueryA](env, "SupportBean_A")
	var deletePlan esper.Plan
	if namedWindow {
		deletePlan, err = env.Build(esper.OnEvent(deleteSource).DeleteFromNamedWindow("MyInfraSSS", esper.Equal[string](esper.NamedWindowField[string]("key"), esper.Field[infraNWTableSubqueryA, string]("id"))).Query(esper.StatementName("delete")))
	} else {
		deletePlan, err = env.Build(esper.OnEvent(deleteSource).DeleteFromTableWhere("MyInfraSSS", esper.Equal[string](esper.TableField[string]("key"), esper.Field[infraNWTableSubqueryA, string]("id"))).Query(esper.StatementName("delete")))
	}
	if err != nil {
		return compat.Trace{}, err
	}
	createPlan, err := env.Build(inner.Query(esper.StatementName("create"), esper.WithOldStream()))
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithRuntimeURI(runtimeID))
	defer func() { _ = engine.Close(context.Background()) }()
	createDeployment, err := engine.Deploy(ctx, createPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := engine.Deploy(ctx, insertPlan); err != nil {
		return compat.Trace{}, err
	}
	if _, err := engine.Deploy(ctx, deletePlan); err != nil {
		return compat.Trace{}, err
	}
	createStatement, ok := createDeployment.Statement("create")
	if !ok {
		return compat.Trace{}, fmt.Errorf("create statement is missing")
	}
	trace, err := compat.ReplayWithStatements(ctx, engine, createStatement, scenario, decodeInfraNWTableSubqueryPayload, func(name string) (*esper.Statement, error) {
		if name != "create" {
			return nil, fmt.Errorf("unknown SelfCheck statement %q", name)
		}
		return createStatement, nil
	})
	if err != nil {
		return compat.Trace{}, err
	}
	if !namedWindow {
		for index := range trace.Records {
			if trace.Records[index].Operation == "snapshot" {
				sortInfraNWTableSubqueryRows(trace.Records[index].New)
				sortInfraNWTableSubqueryRows(trace.Records[index].Old)
			}
		}
	}
	return trace, nil
}

func decodeInfraNWTableSubqueryPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value infraNWTableSubqueryBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value infraNWTableSubqueryS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_A":
		var value infraNWTableSubqueryA
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported InfraNWTableSubquery event type %q", step.EventType)
	}
}

func infraNWTableSubqueryRuntimeID(caseName string) string {
	switch caseName {
	case "scene-one-named-window":
		return infraNWTableSubqueryJavaRuntimeIDs[0]
	case "scene-one-table":
		return infraNWTableSubqueryJavaRuntimeIDs[1]
	case "self-check-named-window":
		return infraNWTableSubqueryJavaRuntimeIDs[2]
	case "self-check-table":
		return infraNWTableSubqueryJavaRuntimeIDs[3]
	default:
		return "infra-nwtable-subquery-unknown"
	}
}

func isSceneOneCase(caseName string) bool {
	return caseName == "scene-one-named-window" || caseName == "scene-one-table"
}

func sortInfraNWTableSubqueryRows(rows []compat.ResultRecord) {
	sort.SliceStable(rows, func(i, j int) bool {
		left := fmt.Sprint(rows[i].Fields["key"])
		right := fmt.Sprint(rows[j].Fields["key"])
		if left != right {
			return left < right
		}
		return fmt.Sprint(rows[i].Fields["value"]) < fmt.Sprint(rows[j].Fields["value"])
	})
}

var (
	reflectTypeString = reflect.TypeOf("")
	reflectTypeInt    = reflect.TypeOf(0)
)
