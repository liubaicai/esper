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

type infraNWTableSubqueryDeleteAggregateBean struct {
	TheString     string `esper:"theString"`
	IntBoxed      int    `esper:"intBoxed"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type infraNWTableSubqueryDeleteAggregateMarket struct {
	Symbol string `esper:"symbol"`
}

const infraNWTableSubqueryDeleteAggregateJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var infraNWTableSubqueryDeleteAggregateJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubquery.java",
}

var infraNWTableSubqueryDeleteAggregateJavaRuntimeIDs = []string{
	"java-runtime-0d6d178d950b8f02c75a",
	"java-runtime-b3cb688e8cb105fa1cea",
	"java-runtime-44eff67d7f2520f31f05",
	"java-runtime-1d5e53c021e683fb3a59",
}

var infraNWTableSubqueryDeleteAggregateJavaExecutions = []string{
	"InfraSubqueryDeleteInsertReplace{namedWindow=true}",
	"InfraSubqueryDeleteInsertReplace{namedWindow=false}",
	"InfraUncorrelatedSubqueryAggregation{namedWindow=true}",
	"InfraUncorrelatedSubqueryAggregation{namedWindow=false}",
}

var infraNWTableSubqueryDeleteAggregateCases = []string{
	"delete-replace-named-window",
	"delete-replace-table",
	"aggregate-named-window",
	"aggregate-table",
}

func runInfraNWTableSubqueryDeleteAggregateScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range infraNWTableSubqueryDeleteAggregateCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runInfraNWTableSubqueryDeleteAggregateCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("infra NW/Table subquery delete/aggregate case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("infra NW/Table subquery delete/aggregate scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runInfraNWTableSubqueryDeleteAggregateCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableSubqueryDeleteAggregateBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if caseName == "aggregate-named-window" || caseName == "aggregate-table" {
		if _, err := esper.RegisterStruct[infraNWTableSubqueryDeleteAggregateMarket](env, "SupportMarketDataBean"); err != nil {
			return compat.Trace{}, err
		}
	}
	if caseName == "delete-replace-named-window" || caseName == "delete-replace-table" {
		return runInfraNWTableSubqueryDeleteAggregateDelete(ctx, env, scenario, caseName)
	}
	return runInfraNWTableSubqueryDeleteAggregateAggregation(ctx, env, scenario, caseName)
}

func runInfraNWTableSubqueryDeleteAggregateDelete(ctx context.Context, env *esper.Environment, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	const name = "MyInfra"
	namedWindow := caseName == "delete-replace-named-window"
	if namedWindow {
		schema, err := esper.NewMapSchema("InfraDeleteReplaceSchema", []esper.FieldSpec{
			esper.FieldDef("key", reflect.TypeOf("")),
			esper.FieldDef("value", reflect.TypeOf(0)),
		})
		if err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterSchema(schema); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, name, schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	} else {
		if _, err := esper.CreateTable(env, name, []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("key"),
			esper.PrimaryKeyColumn[int]("value"),
		}); err != nil {
			return compat.Trace{}, err
		}
	}

	source := esper.From[infraNWTableSubqueryDeleteAggregateBean](env, "SupportBean")
	assignments := []esper.TableAssignment{
		esper.SetColumn("key", esper.Field[infraNWTableSubqueryDeleteAggregateBean, string]("theString")),
		esper.SetColumn("value", esper.Field[infraNWTableSubqueryDeleteAggregateBean, int]("intBoxed")),
	}
	var insertPlan esper.Plan
	var err error
	if namedWindow {
		insertPlan, err = env.Build(esper.OnEvent(source).InsertIntoNamedWindow(name, assignments...).Query(esper.StatementName("insert")))
	} else {
		insertPlan, err = env.Build(esper.OnEvent(source).InsertIntoTable(name, assignments...).Query(esper.StatementName("insert")))
	}
	if err != nil {
		return compat.Trace{}, err
	}
	deleteSource := esper.From[infraNWTableSubqueryDeleteAggregateBean](env, "SupportBean")
	var deletePlan esper.Plan
	if namedWindow {
		deletePlan, err = env.Build(esper.OnEvent(deleteSource).DeleteFromNamedWindow(name,
			esper.Equal[string](esper.NamedWindowField[string]("key"), esper.Field[infraNWTableSubqueryDeleteAggregateBean, string]("theString"))).Query(esper.StatementName("delete")))
	} else {
		deletePlan, err = env.Build(esper.OnEvent(deleteSource).DeleteFromTableWhere(name,
			esper.Equal[string](esper.TableField[string]("key"), esper.Field[infraNWTableSubqueryDeleteAggregateBean, string]("theString"))).Query(esper.StatementName("delete")))
	}
	if err != nil {
		return compat.Trace{}, err
	}
	var inner esper.RecordStream
	if namedWindow {
		inner = esper.FromNamedWindow(env, name)
	} else {
		inner = esper.FromTable(env, name)
	}
	var createQuery esper.Query
	if namedWindow {
		createQuery = inner.CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream())
	} else {
		createQuery = inner.Query(esper.StatementName("create"), esper.WithOldStream())
	}
	createPlan, err := env.Build(createQuery)
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env, esper.WithRuntimeURI(infraNWTableSubqueryDeleteAggregateRuntimeID(caseName)))
	defer func() { _ = engine.Close(context.Background()) }()
	createDeployment, err := engine.Deploy(ctx, createPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := engine.Deploy(ctx, deletePlan); err != nil {
		return compat.Trace{}, err
	}
	if _, err := engine.Deploy(ctx, insertPlan); err != nil {
		return compat.Trace{}, err
	}
	createStatement, ok := createDeployment.Statement("create")
	if !ok {
		return compat.Trace{}, fmt.Errorf("create statement is missing")
	}
	trace, err := compat.ReplayWithStatements(ctx, engine, createStatement, scenario,
		decodeInfraNWTableSubqueryDeleteAggregatePayload,
		func(name string) (*esper.Statement, error) {
			if name != "create" {
				return nil, fmt.Errorf("unknown delete/replace statement %q", name)
			}
			return createStatement, nil
		})
	if err != nil {
		return compat.Trace{}, err
	}
	if !namedWindow {
		for index := range trace.Records {
			if trace.Records[index].Operation == "snapshot" {
				sortInfraNWTableSubqueryDeleteAggregateRows(trace.Records[index].New)
				sortInfraNWTableSubqueryDeleteAggregateRows(trace.Records[index].Old)
			}
		}
	}
	return trace, nil
}

func runInfraNWTableSubqueryDeleteAggregateAggregation(ctx context.Context, env *esper.Environment, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	const name = "MyInfraUCS"
	namedWindow := caseName == "aggregate-named-window"
	if namedWindow {
		schema, err := esper.NewMapSchema("InfraAggregateSchema", []esper.FieldSpec{
			esper.FieldDef("a", reflect.TypeOf("")),
			esper.FieldDef("b", reflect.TypeOf(int64(0))),
		})
		if err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterSchema(schema); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, name, schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	} else {
		if _, err := esper.CreateTable(env, name, []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("a"),
			esper.TableColumnOf[int64]("b"),
		}); err != nil {
			return compat.Trace{}, err
		}
	}

	source := esper.From[infraNWTableSubqueryDeleteAggregateBean](env, "SupportBean")
	assignments := []esper.TableAssignment{
		esper.SetColumn("a", esper.Field[infraNWTableSubqueryDeleteAggregateBean, string]("theString")),
		esper.SetColumn("b", esper.Field[infraNWTableSubqueryDeleteAggregateBean, int64]("longPrimitive")),
	}
	var insertPlan esper.Plan
	var err error
	if namedWindow {
		insertPlan, err = env.Build(esper.OnEvent(source).InsertIntoNamedWindow(name, assignments...).Query(esper.StatementName("insert")))
	} else {
		insertPlan, err = env.Build(esper.OnEvent(source).InsertIntoTable(name, assignments...).Query(esper.StatementName("insert")))
	}
	if err != nil {
		return compat.Trace{}, err
	}
	inner := esper.FromNamedWindow(env, name)
	if !namedWindow {
		inner = esper.FromTable(env, name)
	}
	value := esper.SubquerySum[int64](inner, esper.Field[any, int64]("b"))
	buildConsumer := func(statementName string) (esper.Plan, error) {
		return env.Build(esper.Select(esper.From[infraNWTableSubqueryDeleteAggregateMarket](env, "SupportMarketDataBean"),
			esper.Alias("value", value),
			esper.Alias("symbol", esper.Field[infraNWTableSubqueryDeleteAggregateMarket, string]("symbol")),
		).Query(esper.StatementName(statementName)))
	}
	selectOnePlan, err := buildConsumer("selectOne")
	if err != nil {
		return compat.Trace{}, err
	}
	selectTwoPlan, err := buildConsumer("selectTwo")
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env, esper.WithRuntimeURI(infraNWTableSubqueryDeleteAggregateRuntimeID(caseName)))
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := engine.Deploy(ctx, insertPlan); err != nil {
		return compat.Trace{}, err
	}
	selectOneDeployment, err := engine.Deploy(ctx, selectOnePlan)
	if err != nil {
		return compat.Trace{}, err
	}
	selectOne, ok := selectOneDeployment.Statement("selectOne")
	if !ok {
		return compat.Trace{}, fmt.Errorf("selectOne statement is missing")
	}
	return compat.ReplayWithStatementsAndHandlers(ctx, engine, selectOne, scenario,
		decodeInfraNWTableSubqueryDeleteAggregatePayload,
		func(name string) (*esper.Statement, error) {
			if name == "selectOne" {
				return selectOne, nil
			}
			return nil, fmt.Errorf("unknown aggregate statement %q", name)
		},
		map[string]compat.StepHandler{
			"deploy": func(step compat.Step, attach func(*esper.Statement) error) ([]compat.TraceRecord, error) {
				if step.Statement != "selectTwo" {
					return nil, fmt.Errorf("unknown aggregate deployment %q", step.Statement)
				}
				deployment, err := engine.Deploy(ctx, selectTwoPlan)
				if err != nil {
					return nil, err
				}
				selectTwo, ok := deployment.Statement("selectTwo")
				if !ok {
					return nil, fmt.Errorf("selectTwo statement is missing")
				}
				if err := attach(selectTwo); err != nil {
					return nil, err
				}
				return nil, nil
			},
		})
}

func decodeInfraNWTableSubqueryDeleteAggregatePayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value infraNWTableSubqueryDeleteAggregateBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportMarketDataBean":
		var value infraNWTableSubqueryDeleteAggregateMarket
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported delete/aggregate event type %q", step.EventType)
	}
}

func infraNWTableSubqueryDeleteAggregateRuntimeID(caseName string) string {
	switch caseName {
	case "delete-replace-named-window":
		return infraNWTableSubqueryDeleteAggregateJavaRuntimeIDs[0]
	case "delete-replace-table":
		return infraNWTableSubqueryDeleteAggregateJavaRuntimeIDs[1]
	case "aggregate-named-window":
		return infraNWTableSubqueryDeleteAggregateJavaRuntimeIDs[2]
	case "aggregate-table":
		return infraNWTableSubqueryDeleteAggregateJavaRuntimeIDs[3]
	default:
		return "infra-nwtable-subquery-delete-aggregate-unknown"
	}
}

func sortInfraNWTableSubqueryDeleteAggregateRows(rows []compat.ResultRecord) {
	sort.SliceStable(rows, func(i, j int) bool {
		left := fmt.Sprint(rows[i].Fields["key"])
		right := fmt.Sprint(rows[j].Fields["key"])
		if left != right {
			return left < right
		}
		return fmt.Sprint(rows[i].Fields["value"]) < fmt.Sprint(rows[j].Fields["value"])
	})
}
