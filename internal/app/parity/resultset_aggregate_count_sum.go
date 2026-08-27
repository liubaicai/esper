package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type countSumMarket struct {
	Symbol string  `esper:"symbol"`
	Volume *int64  `esper:"volume"`
	Price  float64 `esper:"price"`
	Feed   string  `esper:"feed"`
}

type countSumStringBean struct {
	TheString string `esper:"theString"`
}

type countSumBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongBoxed     int64  `esper:"longBoxed"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type countSumTriggerA struct {
	ID string `esper:"id"`
}

type countSumTriggerB struct {
	ID string `esper:"id"`
}

type countSumNamedWindowRow struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongBoxed     int64  `esper:"longBoxed"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

const resultsetAggregateCountSumJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateCountSumJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateCountSum.java",
}

var (
	resultsetAggregateCountSumJavaRuntimeIDs = []string{
		"java-runtime-d915c88b8d910c953420", // ResultSetAggregateCountSimple
		"java-runtime-7887bb0df9aaf7e99c89", // ResultSetAggregateCountPlusStar
		"java-runtime-5f8ee37abdc1302b4598", // ResultSetAggregateCountHaving
		"java-runtime-6fe8e1bd138b1ee81944", // ResultSetAggregateSumHaving
		"java-runtime-f24aa8d884bd556d2023", // ResultSetAggregateCountOneViewOM
		"java-runtime-9ca872971349f850d32e", // ResultSetAggregateGroupByCountNestedAggregationAvg
		"java-runtime-ef1afacfc1fb429a8bef", // ResultSetAggregateCountOneView
		"java-runtime-5fd3d359929d94deba32", // ResultSetAggregateCountJoin
		"java-runtime-d2a92c1b89214be184e4", // ResultSetAggregateSumNamedWindowRemoveGroup
	}
	resultsetAggregateCountSumJavaExecutions = []string{
		"ResultSetAggregateCountSimple",
		"ResultSetAggregateCountPlusStar",
		"ResultSetAggregateCountHaving",
		"ResultSetAggregateSumHaving",
		"ResultSetAggregateCountOneViewOM",
		"ResultSetAggregateGroupByCountNestedAggregationAvg",
		"ResultSetAggregateCountOneView",
		"ResultSetAggregateCountJoin",
		"ResultSetAggregateSumNamedWindowRemoveGroup",
	}
)

// runResultSetAggregateCountSumScenario replays the grouped irstream
// count/count-distinct/count sequence over SupportMarketDataBean#length(3)
// (view and SupportBeanString-seeded join variants), the ungrouped count(*)
// over a time(1) window, and the named-window grouped sum with on-delete
// group removal, matching ResultSetAggregateCountOneView,
// ResultSetAggregateCountJoin, ResultSetAggregateCountSimple and
// ResultSetAggregateSumNamedWindowRemoveGroup.
func runResultSetAggregateCountSumScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		"count-one-view",
		"count-join",
		"count-simple",
		"sum-named-window-remove-group",
		"count-plus-star",
		"count-having",
		"sum-having",
		"count-one-view-om",
		"nested-avg",
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetAggregateCountSumCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-count-sum case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-count-sum scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateCountSumCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[countSumMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[countSumStringBean](env, "SupportBeanString"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[countSumBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[countSumTriggerA](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[countSumTriggerB](env, "SupportBean_B"); err != nil {
		return compat.Trace{}, err
	}

	var engine *esper.Engine
	var statement *esper.Statement
	defer func() {
		if engine != nil {
			_ = engine.Close(context.Background())
		}
	}()
	deploy := func(plan esper.Plan) (*esper.Statement, error) {
		if engine == nil {
			return nil, fmt.Errorf("resultset-aggregate-count-sum engine is not initialized")
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return nil, err
		}
		statements := deployment.Statements()
		if len(statements) != 1 {
			return nil, fmt.Errorf("expected one statement, got %d", len(statements))
		}
		return statements[0], nil
	}
	buildAndDeploy := func(query esper.Query) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		engine, statement, err = deployParityStatement(ctx, env, plan)
		return err
	}

	switch caseName {
	case "count-one-view", "count-one-view-om":
		symbol := esper.Field[countSumMarket, string]("symbol")
		volume := esper.Field[countSumMarket, *int64]("volume")
		grouped := esper.From[countSumMarket](env, "SupportMarketDataBean").
			Window(esper.LengthWindow(3)).
			Filter(esper.Or(esper.Or(
				esper.Equal[string](symbol, esper.Literal("DELL")),
				esper.Equal[string](symbol, esper.Literal("IBM")),
			), esper.Equal[string](symbol, esper.Literal("GE")))).
			GroupBy(symbol)
		query := grouped.Select(
			esper.Alias("symbol", symbol),
			esper.Alias("countAll", esper.CountAll()),
			esper.Alias("countDistVol", esper.CountDistinct[any](volume)),
			esper.Alias("countVol", esper.Count[*int64](volume)),
		).Query(esper.StatementName("s0"), esper.WithOldStream())
		if err := buildAndDeploy(query); err != nil {
			return compat.Trace{}, err
		}
	case "count-join":
		theString := esper.Field[countSumStringBean, string]("theString")
		symbol := esper.Field[countSumMarket, string]("symbol")
		left := esper.From[countSumStringBean](env, "SupportBeanString").
			Window(esper.LengthWindow(100))
		right := esper.From[countSumMarket](env, "SupportMarketDataBean").
			Window(esper.LengthWindow(3)).
			Filter(esper.Or(esper.Or(
				esper.Equal[string](symbol, esper.Literal("DELL")),
				esper.Equal[string](symbol, esper.Literal("IBM")),
			), esper.Equal[string](symbol, esper.Literal("GE"))))
		joined := esper.Join(left, right, esper.OnEqual(theString, symbol))
		grouped := joined.GroupBy(esper.JoinField[string](1, "symbol"))
		query := grouped.Select(
			esper.Alias("symbol", esper.JoinField[string](1, "symbol")),
			esper.Alias("countAll", esper.CountAll()),
			esper.Alias("countDistVol", esper.CountDistinct[any](esper.JoinField[*int64](1, "volume"))),
			esper.Alias("countVol", esper.Count[*int64](esper.JoinField[*int64](1, "volume"))),
		).Query(esper.StatementName("s0"), esper.WithOldStream())
		if err := buildAndDeploy(query); err != nil {
			return compat.Trace{}, err
		}
	case "count-simple":
		query := esper.From[countSumMarket](env, "SupportMarketDataBean").
			Window(esper.TimeWindow(time.Second)).
			Aggregate(esper.Alias("cnt", esper.CountAll())).
			Query(esper.StatementName("s0"))
		if err := buildAndDeploy(query); err != nil {
			return compat.Trace{}, err
		}
	case "sum-named-window-remove-group":
		rowSchema, err := esper.RegisterStruct[countSumNamedWindowRow](env, "MyWindowRow")
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindow", rowSchema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		bean := esper.From[countSumBean](env, "SupportBean")
		insertPlan, err := env.Build(esper.OnEvent(bean).InsertIntoNamedWindow(
			"MyWindow",
			esper.SetColumn("theString", esper.Field[countSumBean, string]("theString")),
			esper.SetColumn("intPrimitive", esper.Field[countSumBean, int]("intPrimitive")),
			esper.SetColumn("longBoxed", esper.Field[countSumBean, int64]("longBoxed")),
			esper.SetColumn("longPrimitive", esper.Field[countSumBean, int64]("longPrimitive")),
		).Query(esper.StatementName("insert")))
		if err != nil {
			return compat.Trace{}, err
		}
		deleteAPlan, err := env.Build(esper.OnEvent(
			esper.From[countSumTriggerA](env, "SupportBean_A"),
		).DeleteFromNamedWindow(
			"MyWindow",
			esper.Equal[string](esper.NamedWindowField[string]("theString"), esper.Field[countSumTriggerA, string]("id")),
		).Query(esper.StatementName("delete1")))
		if err != nil {
			return compat.Trace{}, err
		}
		deleteBPlan, err := env.Build(esper.OnEvent(
			esper.From[countSumTriggerB](env, "SupportBean_B"),
		).DeleteAllFromNamedWindow("MyWindow").Query(esper.StatementName("delete2")))
		if err != nil {
			return compat.Trace{}, err
		}
		theString := esper.Field[any, string]("theString")
		consumerPlan, err := env.Build(esper.FromNamedWindow(env, "MyWindow").
			GroupBy(theString).
			Select(
				esper.Alias("theString", theString),
				esper.Alias("mysum", esper.Sum[int](esper.Field[any, int]("intPrimitive"))),
			).
			Query(esper.StatementName("s0"),
				esper.OrderBy(esper.Ascending(esper.ResultField[string]("theString")))))
		if err != nil {
			return compat.Trace{}, err
		}
		engine = esper.NewEngine(env)
		for _, plan := range []esper.Plan{insertPlan, deleteAPlan, deleteBPlan} {
			if _, err := deploy(plan); err != nil {
				return compat.Trace{}, err
			}
		}
		statement, err = deploy(consumerPlan)
		if err != nil {
			return compat.Trace{}, err
		}
	case "count-plus-star":
		symbol := esper.Field[countSumMarket, string]("symbol")
		volume := esper.Field[countSumMarket, *int64]("volume")
		price := esper.Field[countSumMarket, float64]("price")
		feed := esper.Field[countSumMarket, string]("feed")
		query := esper.From[countSumMarket](env, "SupportMarketDataBean").
			Aggregate(
				esper.Alias("symbol", symbol),
				esper.Alias("volume", volume),
				esper.Alias("price", price),
				esper.Alias("feed", feed),
				esper.Alias("cnt", esper.CountAll()),
			).
			Query(esper.StatementName("s0"))
		if err := buildAndDeploy(query); err != nil {
			return compat.Trace{}, err
		}
	case "count-having":
		intPrimitive := esper.Field[countSumBean, int]("intPrimitive")
		sumExpr := esper.Sum[int](intPrimitive)
		query := esper.From[countSumBean](env, "SupportBean").
			Aggregate(esper.Alias("mysum", sumExpr)).
			Having(esper.Equal[int](sumExpr, esper.Literal(2))).
			Query(esper.StatementName("s0"), esper.WithOldStream())
		if err := buildAndDeploy(query); err != nil {
			return compat.Trace{}, err
		}
	case "sum-having":
		countExpr := esper.CountAll()
		query := esper.From[countSumBean](env, "SupportBean").
			Aggregate(esper.Alias("mysum", countExpr)).
			Having(esper.Equal[int64](countExpr, esper.Literal(int64(2)))).
			Query(esper.StatementName("s0"), esper.WithOldStream())
		if err := buildAndDeploy(query); err != nil {
			return compat.Trace{}, err
		}
	case "nested-avg":
		symbol := esper.Field[countSumMarket, string]("symbol")
		countExpr := esper.CountAll()
		query := esper.From[countSumMarket](env, "SupportMarketDataBean").
			Window(esper.LengthWindow(3)).
			GroupBy(symbol).
			Select(
				esper.Alias("symbol", symbol),
				esper.Alias("cnt", countExpr),
				esper.Alias("val", esper.Avg[int64](countExpr)),
			).
			Query(esper.StatementName("s0"),
				esper.OrderBy(esper.Ascending(esper.ResultField[string]("symbol"))))
		if err := buildAndDeploy(query); err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset-aggregate-count-sum case %q", caseName)
	}
	if engine == nil || statement == nil {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-count-sum case %q did not deploy a listener statement", caseName)
	}

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateCountSumPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-count-sum statement %q", name)
		}
		return statement, nil
	})
}

func decodeResultSetAggregateCountSumPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportMarketDataBean":
		var value countSumMarket
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		value.Feed = "f1"
		return value, nil
	case "SupportBeanString":
		var value countSumStringBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBeanString: %w", err)
		}
		return value, nil
	case "SupportBean":
		var value countSumBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_A":
		var value countSumTriggerA
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return value, nil
	case "SupportBean_B":
		var value countSumTriggerB
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_B: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported resultset-aggregate-count-sum event type %q", step.EventType)
	}
}
