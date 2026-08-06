package esper

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

func TestSubqueryExistsCorrelatesNamedWindowWithOuterField(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "Prices", schema); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if err := engine.InsertNamedWindow(context.Background(), "Prices", runtimeTestTrade{Symbol: "reference", Price: 20}); err != nil {
		t.Fatal(err)
	}
	query := From[runtimeTestTrade](env, "Trade").Filter(
		SubqueryExists(
			FromNamedWindow(env, "Prices"),
			Greater[float64](Field[any, float64]("price"), OuterField[float64]("price")),
		),
	).Query(StatementName("subquery-exists"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var results []Result
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		results = append(results, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B", Price: 25}); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("correlated exists results = %#v", results)
	}
	matched, ok := results[0].Event()
	if !ok || matched.Underlying().(runtimeTestTrade).Symbol != "A" {
		t.Fatalf("correlated exists event = %#v", results[0])
	}
}

func TestSubqueryValueAndInReadNamedWindowAndTableSnapshots(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "Prices", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "Symbols", []TableColumn{PrimaryKeyColumn[string]("symbol")}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if err := engine.InsertNamedWindow(context.Background(), "Prices", runtimeTestTrade{Symbol: "A", Price: 12.5}); err != nil {
		t.Fatal(err)
	}
	table, ok := engine.Table("Symbols")
	if !ok {
		t.Fatal("Symbols table is missing")
	}
	if _, err := table.Insert(context.Background(), map[string]any{"symbol": "A"}); err != nil {
		t.Fatal(err)
	}
	stream := From[runtimeTestTrade](env, "Trade")
	valueQuery := Select(stream,
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
		Alias("reference", SubqueryValue[float64](
			FromNamedWindow(env, "Prices"),
			Field[any, float64]("price"),
			Equal[string](Field[any, string]("symbol"), OuterField[string]("symbol")),
		)),
	).Query(StatementName("subquery-value"))
	valuePlan, err := env.Build(valueQuery)
	if err != nil {
		t.Fatal(err)
	}
	inQuery := stream.Filter(SubqueryIn[string](
		Field[runtimeTestTrade, string]("symbol"),
		FromTable(env, "Symbols"),
		Field[any, string]("symbol"),
	)).Query(StatementName("subquery-in"))
	inPlan, err := env.Build(inQuery)
	if err != nil {
		t.Fatal(err)
	}
	valueDeployment, err := engine.Deploy(context.Background(), valuePlan)
	if err != nil {
		t.Fatal(err)
	}
	inDeployment, err := engine.Deploy(context.Background(), inPlan)
	if err != nil {
		t.Fatal(err)
	}
	var valueRows []Row
	var inResults []Result
	if _, err := valueDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("subquery value result is not a row: %#v", result)
			}
			valueRows = append(valueRows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := inDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		inResults = append(inResults, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if len(valueRows) != 1 || valueRows[0].Get("reference").Any() != 12.5 || len(inResults) != 1 {
		t.Fatalf("subquery value/in results = %#v/%#v", valueRows, inResults)
	}
}

func TestNestedSubqueryUsesOuterSubqueryCandidate(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "Prices", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "Symbols", []TableColumn{PrimaryKeyColumn[string]("symbol")}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if err := engine.InsertNamedWindow(context.Background(), "Prices", runtimeTestTrade{Symbol: "A", Price: 12.5}); err != nil {
		t.Fatal(err)
	}
	table, ok := engine.Table("Symbols")
	if !ok {
		t.Fatal("Symbols table is missing")
	}
	if _, err := table.Insert(context.Background(), map[string]any{"symbol": "A"}); err != nil {
		t.Fatal(err)
	}

	query := From[runtimeTestTrade](env, "Trade").Filter(
		SubqueryExists(
			FromNamedWindow(env, "Prices"),
			SubqueryExists(
				FromTable(env, "Symbols"),
				Equal[string](Field[any, string]("symbol"), OuterField[string]("symbol")),
			),
		),
	).Query(StatementName("nested-subquery"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var results []Result
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		results = append(results, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("nested subquery results = %#v", results)
	}
}

func TestFireAndForgetSubqueryUsesConsistentSnapshots(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "Prices", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "Positions", []TableColumn{PrimaryKeyColumn[string]("symbol")}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if err := engine.InsertNamedWindow(context.Background(), "Prices", runtimeTestTrade{Symbol: "A", Price: 12.5}); err != nil {
		t.Fatal(err)
	}
	table, ok := engine.Table("Positions")
	if !ok {
		t.Fatal("Positions table is missing")
	}
	for _, symbol := range []string{"A", "B"} {
		if _, err := table.Insert(context.Background(), map[string]any{"symbol": symbol}); err != nil {
			t.Fatal(err)
		}
	}

	plan, err := env.Build(FromTable(env, "Positions").Filter(
		SubqueryExists(
			FromNamedWindow(env, "Prices"),
			Equal[string](Field[any, string]("symbol"), OuterField[string]("symbol")),
		),
	).Query(StatementName("faf-subquery")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil || len(result.Results()) != 1 {
		t.Fatalf("FAF subquery results = %#v, err=%v", result.Results(), err)
	}
	if event, ok := result.Results()[0].Event(); !ok || event.Get("symbol").Any() != "A" {
		t.Fatalf("FAF subquery event = %#v", result.Results()[0])
	}
}

func TestSubqueryParametersAndValidation(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "Prices", schema); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if err := engine.InsertNamedWindow(context.Background(), "Prices", runtimeTestTrade{Symbol: "A", Price: 20}); err != nil {
		t.Fatal(err)
	}
	query := From[runtimeTestTrade](env, "Trade").Filter(SubqueryExists(
		FromNamedWindow(env, "Prices"),
		Greater[float64](Field[any, float64]("price"), Parameter[float64]("minimum")),
	)).Query(StatementName("subquery-parameter"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err == nil {
		t.Fatal("subquery parameter must require DeployWithParameters")
	}
	deployment, err := engine.DeployWithParameters(context.Background(), plan, ParameterValues{"minimum": 10.0})
	if err != nil {
		t.Fatal(err)
	}
	var results []Result
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		results = append(results, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("parameterized subquery results = %#v", results)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Filter(SubqueryExists(
		FromAny(env, "Trade"),
		Literal(true),
	)).Query(StatementName("event-stream-subquery-source"))); err == nil {
		t.Fatal("non-aggregated event source without a window must be rejected as a subquery source")
	}
}

func TestSubqueryAggregatesAndQuantifiedComparisons(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "Prices", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "EmptyPrices", schema); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, price := range []float64{5, 10} {
		if err := engine.InsertNamedWindow(context.Background(), "Prices", runtimeTestTrade{Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	prices := FromNamedWindow(env, "Prices")
	emptyPrices := FromNamedWindow(env, "EmptyPrices")
	query := Select(
		From[runtimeTestTrade](env, "Trade"),
		Alias("count", SubqueryCount(prices)),
		Alias("sum", SubquerySum[float64](prices, Field[any, float64]("price"))),
		Alias("avg", SubqueryAvg[float64](prices, Field[any, float64]("price"))),
		Alias("less_any", SubqueryAny[float64](
			Field[runtimeTestTrade, float64]("price"), prices, Field[any, float64]("price"), SubqueryLess,
		)),
		Alias("less_all", SubqueryAll[float64](
			Field[runtimeTestTrade, float64]("price"), prices, Field[any, float64]("price"), SubqueryLess,
		)),
		Alias("empty_count", SubqueryCount(emptyPrices)),
		Alias("empty_sum", SubquerySum[float64](emptyPrices, Field[any, float64]("price"))),
		Alias("empty_any", SubqueryAny[float64](
			Field[runtimeTestTrade, float64]("price"), emptyPrices, Field[any, float64]("price"), SubqueryLess,
		)),
		Alias("empty_all", SubqueryAll[float64](
			Field[runtimeTestTrade, float64]("price"), emptyPrices, Field[any, float64]("price"), SubqueryLess,
		)),
	).Query(StatementName("subquery-aggregate-quantified"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("subquery aggregate result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 6}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("subquery aggregate rows = %#v", rows)
	}
	row := rows[0]
	if row.Get("count").Any() != int64(2) || row.Get("sum").Any() != 15.0 || row.Get("avg").Any() != 7.5 {
		t.Fatalf("subquery aggregate values = %#v", row)
	}
	if row.Get("less_any").Any() != true || row.Get("less_all").Any() != false {
		t.Fatalf("subquery quantified values = %#v", row)
	}
	if row.Get("empty_count").Any() != int64(0) || !row.Get("empty_sum").IsNull() || !row.Get("empty_any").IsNull() || !row.Get("empty_all").IsNull() {
		t.Fatalf("subquery empty-set values = %#v", row)
	}
	invalid := From[runtimeTestTrade](env, "Trade").Filter(SubqueryAny[float64](
		Field[runtimeTestTrade, float64]("price"), prices, Field[any, float64]("price"), SubqueryComparison(99),
	)).Query(StatementName("invalid-subquery-comparison"))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("invalid quantified subquery comparison must be rejected")
	}

}

func TestSubqueryScalarOptionsOrderLimitOffsetAndCardinality(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "OrderedPrices", schema); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, price := range []float64{5, 10, 20} {
		if err := engine.InsertNamedWindow(context.Background(), "OrderedPrices", runtimeTestTrade{Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	prices := FromNamedWindow(env, "OrderedPrices")
	stream := From[runtimeTestTrade](env, "Trade")
	query := Select(stream,
		Alias("latest", SubqueryValueWithOptions[float64](prices,
			Field[any, float64]("price"),
			SubqueryDescending(Field[any, float64]("price")),
			SubqueryLimit(1),
		)),
		Alias("middle", SubqueryValueWithOptions[float64](prices,
			Field[any, float64]("price"),
			SubqueryAscending(Field[any, float64]("price")),
			SubqueryOffset(1),
			SubqueryLimit(1),
		)),
		Alias("single", SubqueryValueWithOptions[float64](prices,
			Field[any, float64]("price"),
			SubqueryCardinalityMode(SubqueryRequireSingle),
		)),
	).Query(StatementName("subquery-scalar-options"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("subquery scalar result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 1}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("subquery scalar rows = %#v", rows)
	}
	row := rows[0]
	if row.Get("latest").Any() != 20.0 || row.Get("middle").Any() != 10.0 || !row.Get("single").IsNull() {
		t.Fatalf("subquery scalar options = %#v", row)
	}
	invalid := Select(stream, Alias("bad", SubqueryValueWithOptions[float64](
		prices, Field[any, float64]("price"), SubqueryOffset(-1),
	))).Query(StatementName("invalid-subquery-offset"))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("negative subquery offset must be rejected")
	}
	invalid = Select(stream, Alias("bad", SubqueryValueWithOptions[float64](
		prices, Field[any, float64]("price"), SubqueryLimit(-1),
	))).Query(StatementName("invalid-subquery-limit"))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("negative subquery limit must be rejected")
	}
}

func TestSubqueryGroupByAggregateAndHaving(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "Prices", schema); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, trade := range []runtimeTestTrade{
		{Symbol: "A", Price: 5},
		{Symbol: "A", Price: 10},
		{Symbol: "B", Price: 20},
		{Symbol: "C", Price: 2},
	} {
		if err := engine.InsertNamedWindow(context.Background(), "Prices", trade); err != nil {
			t.Fatal(err)
		}
	}

	prices := FromNamedWindow(env, "Prices")
	price := Field[any, float64]("price")
	sum := Sum[float64](price)
	query := Select(From[runtimeTestTrade](env, "Trade"),
		Alias("groups", SubqueryGroupBy[string, float64](
			prices,
			Field[any, string]("symbol"),
			sum,
			SubqueryGroupWhere(Greater[float64](price, Literal(3.0))),
			SubqueryGroupHaving(GreaterOrEqual[float64](sum, Literal(15.0))),
		)),
		Alias("raw_groups", SubqueryGroupBy[string, float64](
			prices,
			Field[any, string]("symbol"),
			price,
			SubqueryGroupWhere(Greater[float64](price, Literal(3.0))),
		)),
	).Query(StatementName("subquery-group-by"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("grouped subquery result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "outer", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("grouped subquery rows = %#v", rows)
	}
	groups, ok := rows[0].Get("groups").Any().(map[string][]float64)
	if !ok {
		t.Fatalf("grouped subquery aggregate type = %#v", rows[0].Get("groups"))
	}
	rawGroups, ok := rows[0].Get("raw_groups").Any().(map[string][]float64)
	if !ok {
		t.Fatalf("grouped subquery scalar type = %#v", rows[0].Get("raw_groups"))
	}
	if !reflect.DeepEqual(groups, map[string][]float64{"A": {15}, "B": {20}}) {
		t.Fatalf("grouped subquery aggregate values = %#v", groups)
	}
	if !reflect.DeepEqual(rawGroups, map[string][]float64{"A": {5, 10}, "B": {20}}) {
		t.Fatalf("grouped subquery scalar values = %#v", rawGroups)
	}
	invalid := Select(From[runtimeTestTrade](env, "Trade"),
		Alias("bad", SubqueryGroupBy[string, float64](
			prices,
			Field[any, string]("missing"),
			price,
		)),
	).Query(StatementName("invalid-subquery-group-key"))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("grouped subquery unknown key must be rejected")
	}
	invalid = Select(From[runtimeTestTrade](env, "Trade"),
		Alias("bad", SubqueryGroupBy[float64, float64](prices, sum, price)),
	).Query(StatementName("invalid-subquery-aggregate-key"))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("grouped subquery aggregate key must be rejected")
	}
}
