package esper

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

type subqueryHavingTrigger struct {
	Threshold float64 `esper:"threshold"`
}

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
	if row.Get("empty_count").Any() != int64(0) || !row.Get("empty_sum").IsNull() || row.Get("empty_any").Any() != false || row.Get("empty_all").Any() != true {
		t.Fatalf("subquery empty-set values = %#v", row)
	}
	invalid := From[runtimeTestTrade](env, "Trade").Filter(SubqueryAny[float64](
		Field[runtimeTestTrade, float64]("price"), prices, Field[any, float64]("price"), SubqueryComparison(99),
	)).Query(StatementName("invalid-subquery-comparison"))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("invalid quantified subquery comparison must be rejected")
	}

}

func TestSubqueryEmptyQuantifiersFollowEsperTruthTable(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "SubqueryQuantifierTrigger", []FieldSpec{
		OptionalFieldDef("value", reflect.TypeOf(float64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	innerSchema, err := RegisterMap(env, "SubqueryQuantifierInner", []FieldSpec{
		OptionalFieldDef("value", reflect.TypeOf(float64(0))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "SubqueryQuantifierWindow", innerSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	outerValue := Field[any, float64]("value")
	innerValue := Field[any, float64]("value")
	window := FromNamedWindow(env, "SubqueryQuantifierWindow")
	query := Select(
		From[map[string]any](env, "SubqueryQuantifierTrigger"),
		Alias("all", SubqueryAll[float64](outerValue, window, innerValue, SubqueryLess)),
		Alias("any", SubqueryAny[float64](outerValue, window, innerValue, SubqueryLess)),
		Alias("in", SubqueryIn[float64](outerValue, window, innerValue)),
	).Query(StatementName("subquery-empty-quantifiers"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("empty quantifier result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(value *float64) Row {
		t.Helper()
		before := len(rows)
		if err := engine.Send(context.Background(), "SubqueryQuantifierTrigger", map[string]any{"value": value}); err != nil {
			t.Fatal(err)
		}
		if len(rows) != before+1 {
			t.Fatalf("empty quantifier rows = %#v", rows)
		}
		return rows[len(rows)-1]
	}

	row := send(nil)
	if row.Get("all").Any() != true || row.Get("any").Any() != false || row.Get("in").Any() != false {
		t.Fatalf("empty quantifier null outer = %#v, want all=true any=false in=false", row)
	}
	one := 1.0
	row = send(&one)
	if row.Get("all").Any() != true || row.Get("any").Any() != false || row.Get("in").Any() != false {
		t.Fatalf("empty quantifier present outer = %#v, want all=true any=false in=false", row)
	}
	if err := engine.InsertNamedWindow(context.Background(), "SubqueryQuantifierWindow", map[string]any{"value": nil}); err != nil {
		t.Fatal(err)
	}
	row = send(&one)
	if !row.Get("all").IsNull() || !row.Get("any").IsNull() || !row.Get("in").IsNull() {
		t.Fatalf("null candidate quantifier values = %#v, want all/any/in null", row)
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

func TestSubqueryUngroupedHavingCorrelatesAndTracksWindowState(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "SubqueryHavingTrade"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subqueryHavingTrigger](env, "SubqueryHavingTrigger"); err != nil {
		t.Fatal(err)
	}

	inner := From[runtimeTestTrade](env, "SubqueryHavingTrade").Window(KeepAll()).AsRecord()
	price := Field[any, float64]("price")
	total := Sum[float64](price)
	threshold := Field[subqueryHavingTrigger, float64]("threshold")
	having := SubqueryHaving(GreaterOrEqual[float64](total, OuterField[float64]("threshold")))
	value := SubqueryValueWithOptions[float64](inner, total,
		having,
	)
	query := Select(
		From[subqueryHavingTrigger](env, "SubqueryHavingTrigger"),
		Alias("value", value),
		Alias("exists", SubqueryExistsValue[float64](inner, total, having)),
		Alias("in", SubqueryInWithOptions[float64](threshold, inner, total, having)),
		Alias("any", SubqueryAnyWithOptions[float64](threshold, inner, total, SubqueryGreater, having)),
		Alias("all", SubqueryAllWithOptions[float64](threshold, inner, total, SubqueryLessOrEqual, having)),
	).Query(StatementName("subquery-ungrouped-having"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("subquery having result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendTrigger := func(threshold float64) Row {
		t.Helper()
		before := len(rows)
		if err := engine.SendEvent(context.Background(), subqueryHavingTrigger{Threshold: threshold}); err != nil {
			t.Fatal(err)
		}
		if len(rows) != before+1 {
			t.Fatalf("subquery having rows = %#v", rows)
		}
		return rows[len(rows)-1]
	}

	row := sendTrigger(15)
	if !row.Get("value").IsNull() || row.Get("exists").Any() != false || !row.Get("in").IsNull() || !row.Get("any").IsNull() || !row.Get("all").IsNull() {
		t.Fatalf("empty aggregate having result = %#v", rows[len(rows)-1])
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 10}); err != nil {
		t.Fatal(err)
	}
	row = sendTrigger(15)
	if !row.Get("value").IsNull() || row.Get("exists").Any() != false || !row.Get("in").IsNull() || !row.Get("any").IsNull() || !row.Get("all").IsNull() {
		t.Fatalf("below-threshold aggregate having result = %#v", rows[len(rows)-1])
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 5}); err != nil {
		t.Fatal(err)
	}
	row = sendTrigger(15)
	if got := row.Get("value").Any(); got != 15.0 || row.Get("exists").Any() != true || row.Get("in").Any() != true || row.Get("any").Any() != false || row.Get("all").Any() != true {
		t.Fatalf("aggregate having result = %#v, want value=15 exists/in=false->true any=false all=true", row)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: -1}); err != nil {
		t.Fatal(err)
	}
	row = sendTrigger(15)
	if !row.Get("value").IsNull() || row.Get("exists").Any() != false || !row.Get("in").IsNull() || !row.Get("any").IsNull() || !row.Get("all").IsNull() {
		t.Fatalf("aggregate having after removal-like update = %#v", rows[len(rows)-1])
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 1}); err != nil {
		t.Fatal(err)
	}
	row = sendTrigger(15)
	if got := row.Get("value").Any(); got != 15.0 || row.Get("exists").Any() != true || row.Get("in").Any() != true || row.Get("any").Any() != false || row.Get("all").Any() != true {
		t.Fatalf("aggregate having recovered result = %#v, want value=15 exists/in=false->true any=false all=true", row)
	}

	// A non-aggregated having is valid: Esper evaluates it row by row after
	// the where clause (covered by the EPLSubselectFiltered parity tests).
	nonAggregateHaving := Select(
		From[subqueryHavingTrigger](env, "SubqueryHavingTrigger"),
		Alias("value", SubqueryValueWithOptions[float64](inner, price, SubqueryHaving(Greater[float64](price, Literal(0.0))))),
	).Query(StatementName("non-aggregate-having"))
	if _, err := env.Build(nonAggregateHaving); err != nil {
		t.Fatalf("having on a non-aggregate subquery must be accepted, got %v", err)
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
	invalid = Select(From[runtimeTestTrade](env, "Trade"),
		Alias("bad", SubqueryGroupBy[float64, float64](
			prices,
			Prev[float64](0, price),
			price,
		)),
	).Query(StatementName("invalid-subquery-previous-key"))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("grouped subquery previous/prior key must be rejected")
	}
}

func TestSubqueryRowsAndGroupedRows(t *testing.T) {
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
	symbol := Field[any, string]("symbol")
	price := Field[any, float64]("price")
	sum := Sum[float64](price)
	columns := []Selection{
		Alias("symbol", symbol),
		Alias("total", sum),
	}
	query := Select(From[runtimeTestTrade](env, "Trade"),
		Alias("first", SubqueryRow(prices,
			Alias("symbol", symbol),
			Alias("price", price),
		)),
		Alias("aggregate", SubqueryRow(prices,
			Alias("count", Count[float64](price)),
			Alias("sum", sum),
		)),
		Alias("rows", SubqueryRows(prices,
			Alias("symbol", symbol),
			Alias("price", price),
		)),
		Alias("group_rows", SubqueryGroupRows(
			prices,
			symbol,
			columns,
			SubqueryGroupWhere(Greater[float64](price, Literal(3.0))),
			SubqueryGroupHaving(GreaterOrEqual[float64](sum, Literal(15.0))),
		)),
	).Query(StatementName("subquery-rows"))
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
				return fmt.Errorf("multi-column subquery result is not a row: %#v", result)
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
		t.Fatalf("multi-column subquery rows = %#v", rows)
	}
	first, ok := rows[0].Get("first").Any().(map[string]any)
	if !ok {
		t.Fatalf("multi-column scalar type = %#v", rows[0].Get("first"))
	}
	if !reflect.DeepEqual(first, map[string]any{"symbol": "A", "price": 5.0}) {
		t.Fatalf("multi-column scalar value = %#v", first)
	}
	aggregate, ok := rows[0].Get("aggregate").Any().(map[string]any)
	if !ok || !reflect.DeepEqual(aggregate, map[string]any{"count": int64(4), "sum": 37.0}) {
		t.Fatalf("multi-column aggregate value = %#v", rows[0].Get("aggregate"))
	}
	many, ok := rows[0].Get("rows").Any().([]map[string]any)
	if !ok || len(many) != 4 {
		t.Fatalf("multi-column rows value = %#v", rows[0].Get("rows"))
	}
	grouped, ok := rows[0].Get("group_rows").Any().([]map[string]any)
	if !ok || !reflect.DeepEqual(grouped, []map[string]any{
		{"symbol": "A", "total": 15.0},
		{"symbol": "B", "total": 20.0},
	}) {
		t.Fatalf("multi-column grouped rows = %#v", rows[0].Get("group_rows"))
	}

	invalid := Select(From[runtimeTestTrade](env, "Trade"),
		Alias("bad", SubqueryRow(prices,
			Alias("duplicate", symbol),
			Alias("duplicate", price),
		)),
	).Query(StatementName("invalid-subquery-column"))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("multi-column duplicate aliases must be rejected")
	}
	invalid = Select(From[runtimeTestTrade](env, "Trade"),
		Alias("bad", SubqueryRow(prices)),
	).Query(StatementName("invalid-subquery-no-columns"))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("multi-column subquery without columns must be rejected")
	}
	invalid = Select(From[runtimeTestTrade](env, "Trade"),
		Alias("bad", SubqueryRow(prices,
			Alias("symbol", symbol),
			Alias("sum", sum),
		)),
	).Query(StatementName("invalid-subquery-mixed-columns"))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("multi-column mixed aggregate columns must be rejected")
	}
	invalid = Select(From[runtimeTestTrade](env, "Trade"),
		Alias("bad", SubqueryGroupRows(prices, OuterField[string]("symbol"), columns)),
	).Query(StatementName("invalid-subquery-correlated-group-key"))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("grouped subquery outer group key must be rejected")
	}
	invalid = Select(From[runtimeTestTrade](env, "Trade"),
		Alias("bad", SubqueryGroupRows(prices, symbol, []Selection{
			Alias("price", price),
			Alias("total", sum),
		})),
	).Query(StatementName("invalid-subquery-group-column"))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("grouped subquery non-key scalar column must be rejected")
	}
}

// TestSubqueryGroupedInAnyAllFollowsEmptySetAndHavingSemantics covers the
// SubqueryGroupKey option family on IN/ANY/ALL/SOME subqueries: an empty
// group set yields SQL empty-set results (IN/ANY/SOME false, ALL true), a
// group-by aggregate produces one candidate per group, and a SubqueryHaving
// filters groups before quantification. Mirrors the
// EPLSubselectAggregatedInExistsAnyAll trajectories exercised by the
// subselect-aggregated-in-exists-any-all differential scenario.
func TestSubqueryGroupedInAnyAllFollowsEmptySetAndHavingSemantics(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "GroupedSubselectTrade"); err != nil {
		t.Fatal(err)
	}
	triggerSchema, err := RegisterMap(env, "GroupedSubselectTrigger", []FieldSpec{
		OptionalFieldDef("value", reflect.TypeOf(int(0))),
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = triggerSchema
	inner := From[runtimeTestTrade](env, "GroupedSubselectTrade").Window(KeepAll()).AsRecord()
	theString := Field[any, string]("symbol")
	sum := Sum[int](Field[any, int]("price"))
	value := Field[any, int]("value")
	grouped := SubqueryGroupKey(theString)
	query := Select(
		From[map[string]any](env, "GroupedSubselectTrigger"),
		Alias("in", SubqueryInWithOptions[int](value, inner, sum, grouped)),
		Alias("any", SubqueryAnyWithOptions[int](value, inner, sum, SubqueryLess, grouped)),
		Alias("all", SubqueryAllWithOptions[int](value, inner, sum, SubqueryLess, grouped)),
		Alias("in_having", SubqueryInWithOptions[int](value, inner, sum, grouped,
			SubqueryHaving(NotEqual[string](Last[string](theString), Literal("B"))))),
	).Query(StatementName("grouped-in-any-all"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 6)
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
	send := func(value int) {
		if err := engine.Send(context.Background(), "GroupedSubselectTrigger", map[string]any{"value": value}); err != nil {
			t.Fatal(err)
		}
	}
	sendTrade := func(symbol string, price int) {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol, Price: float64(price)}); err != nil {
			t.Fatal(err)
		}
	}
	// Empty group set: IN/ANY false, ALL true; having leaves it empty too.
	send(10)
	// Groups {A:5}: 10 < 5 false for every quantifier; in_having keeps A.
	sendTrade("A", 5)
	send(10)
	// Groups {A:5, B:12}: 10 < 12 true -> ANY true, ALL false (10 < 5);
	// in_having drops group B (last(theString) == "B") and keeps A: no match.
	sendTrade("B", 12)
	send(10)
	// Group A grows to sum 8: 10 < 8 false; in_having keeps A only.
	sendTrade("A", 3)
	send(10)
	// Groups {A:8, B:12}: 10 < all false; 10 < any true.
	if len(rows) != 4 {
		t.Fatalf("grouped subquery rows = %#v", rows)
	}
	first := rows[0]
	if first.Get("in").Any() != false || first.Get("any").Any() != false || first.Get("all").Any() != true || first.Get("in_having").Any() != false {
		t.Fatalf("empty grouped subquery = %#v", first.AsMap())
	}
	second := rows[1]
	if second.Get("in").Any() != false || second.Get("any").Any() != false || second.Get("all").Any() != false || second.Get("in_having").Any() != false {
		t.Fatalf("single group subquery = %#v", second.AsMap())
	}
	third := rows[2]
	if third.Get("in").Any() != false || third.Get("any").Any() != true || third.Get("all").Any() != false || third.Get("in_having").Any() != false {
		t.Fatalf("two group subquery = %#v", third.AsMap())
	}
	fourth := rows[3]
	if fourth.Get("in").Any() != false || fourth.Get("any").Any() != true || fourth.Get("all").Any() != false || fourth.Get("in_having").Any() != false {
		t.Fatalf("grown group subquery = %#v", fourth.AsMap())
	}
	// An IN hit: value 12 matches group B's sum 12; in_having drops group B
	// (last(theString) == "B") so the having variant stays false.
	send(12)
	last := rows[len(rows)-1]
	if last.Get("in").Any() != true || last.Get("in_having").Any() != false {
		t.Fatalf("grouped in hit = %#v", last.AsMap())
	}
}
