package esper

import (
	"context"
	"reflect"
	"testing"
)

type infraTableFilterS0 struct {
	ID int64 `esper:"id"`
}

type infraTableFilterBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int64  `esper:"intPrimitive"`
}

type infraTableWindowEvent struct {
	C0 int64 `esper:"c0"`
}

type infraTableWindowTrigger struct {
	ID int64 `esper:"id"`
}

type infraTableIndexedBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int64  `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type infraTableIndexedTrigger struct {
	ID int64 `esper:"id"`
}

type infraTableGroupedTrigger struct {
	ID  int64  `esper:"id"`
	P00 string `esper:"p00"`
}

type infraTableGroupedSingleBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int64  `esper:"intPrimitive"`
}

type infraTableGroupedMultiBean struct {
	TheString       string  `esper:"theString"`
	IntPrimitive    int64   `esper:"intPrimitive"`
	LongPrimitive   int64   `esper:"longPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
}

type infraTableSplitTrigger struct {
	ID int64 `esper:"id"`
}

type infraTableArrayKeyEvent struct {
	ID     string `esper:"id"`
	IntOne []int  `esper:"intOne"`
	Value  int64  `esper:"value"`
}

type infraTableArrayKeyTrigger struct{}

type infraTableArrayPairEvent struct {
	ID     string `esper:"id"`
	IntOne []int  `esper:"intOne"`
	IntTwo []int  `esper:"intTwo"`
	Value  int64  `esper:"value"`
}

type infraTableArrayPairTrigger struct{}

type infraTableArrayPairKey struct {
	K1 []int
	K2 []int
}

type infraTableExpressionBean struct {
	TheString       string  `esper:"theString"`
	IntPrimitive    int     `esper:"intPrimitive"`
	LongPrimitive   int64   `esper:"longPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
	FloatPrimitive  float32 `esper:"floatPrimitive"`
}

type infraTableExpressionTrigger struct {
	ID  int64  `esper:"id"`
	P00 string `esper:"p00"`
}

type infraTableExpressionS1 struct {
	ID int64 `esper:"id"`
}

// TestInfraTableAccessUngroupedContextParity mirrors Java
// InfraTableAccessCore.InfraUngroupedWContext. The same context is entered by
// two event types with different key properties; the table has one unkeyed
// aggregate row per context partition.
func TestInfraTableAccessUngroupedContextParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableGroupedSingleBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableGroupedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}

	current := EventValue[Event]()
	contextKey := CaseWhen[string](
		Equal[string](TypeName(current), Literal("SupportBean")),
		Property[string](current, "theString"),
	).When(
		Equal[string](TypeName(current), Literal("SupportBean_S0")),
		Property[string](current, "p00"),
	).Else(NullLiteral[string]())
	if _, err := CreateKeyContext(env, "partitioned-by-string", contextKey); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varTotalUG", []TableColumn{
		TableColumnOf[int64]("total"),
	}); err != nil {
		t.Fatal(err)
	}

	aggregatePlan, err := env.Build(
		From[infraTableGroupedSingleBean](env, "SupportBean").
			Aggregate(Alias("total", Sum[int64](Field[infraTableGroupedSingleBean, int64]("intPrimitive")))).
			IntoTable("varTotalUG", StatementName("table-context-aggregate"), WithContext("partitioned-by-string")),
	)
	if err != nil {
		t.Fatal(err)
	}
	readPlan, err := env.Build(
		OnEvent(From[infraTableGroupedTrigger](env, "SupportBean_S0")).
			SelectFromTableWhere("varTotalUG", Literal(true),
				Alias("c0", Field[infraTableGroupedTrigger, string]("p00")),
				Alias("c1", TableField[int64]("total")),
			).
			Query(StatementName("table-context-read"), WithContext("partitioned-by-string")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), readPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()

	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("context table read result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	assertPartitionTotal := func(key string, total int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), infraTableGroupedTrigger{P00: key}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatalf("context table read for %q produced no row", key)
		}
		row := rows[len(rows)-1]
		if row.Get("c0").Any() != key || row.Get("c1").Any() != total {
			t.Fatalf("context table partition %q = %#v, want total %d", key, row.AsMap(), total)
		}
	}

	sendBean := func(key string, value int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), infraTableGroupedSingleBean{TheString: key, IntPrimitive: value}); err != nil {
			t.Fatal(err)
		}
	}
	sendBean("A", 10)
	assertPartitionTotal("A", 10)
	sendBean("A", 11)
	assertPartitionTotal("A", 21)
	sendBean("B", 20)
	assertPartitionTotal("A", 21)
	sendBean("B", 21)
	assertPartitionTotal("B", 41)
	sendBean("C", 30)
	assertPartitionTotal("A", 21)
	sendBean("D", 40)
	assertPartitionTotal("C", 30)

	for _, expected := range []struct {
		key   string
		total int64
	}{
		{key: "A", total: 21},
		{key: "B", total: 41},
		{key: "C", total: 30},
		{key: "D", total: 40},
		{key: "A", total: 21},
	} {
		assertPartitionTotal(expected.key, expected.total)
	}

	if got := deployment.Statements()[0].ContextPartitionCount(); got != 4 {
		t.Fatalf("context table read partitions = %d, want 4", got)
	}
}

// TestInfraTableAccessExpressionAliasAndDeclParity mirrors Java
// InfraTableAccessCore.InfraExpressionAliasAndDecl. The Java execution uses
// declared expressions in three positions: aggregate inputs for IntoTable,
// direct table lookup, and a table lookup guarded by a lastevent subquery.
// Go keeps each dependency in the analyzable fluent expression graph.
func TestInfraTableAccessExpressionAliasAndDeclParity(t *testing.T) {
	t.Run("aggregate-input", testInfraTableAccessExpressionAggregateInput)
	t.Run("direct-table-access", testInfraTableAccessExpressionDirectTableAccess)
	t.Run("lastevent-table-access", testInfraTableAccessExpressionLastEventTableAccess)
}

func testInfraTableAccessExpressionAggregateInput(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableExpressionBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableExpressionTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varaggITFE", []TableColumn{
		TableColumnOf[int]("sumi"),
		TableColumnOf[float64]("sumd"),
		TableColumnOf[float32]("sumf"),
		TableColumnOf[int64]("suml"),
	}); err != nil {
		t.Fatal(err)
	}

	aggregateParam := ExpressionParam[Event]("a")
	if err := DefineExpression[int](env, "sumi", Sum[int](Property[int](aggregateParam, "intPrimitive"))); err != nil {
		t.Fatal(err)
	}
	if err := DefineExpression[float64](env, "sumd", Sum[float64](Field[infraTableExpressionBean, float64]("doublePrimitive"))); err != nil {
		t.Fatal(err)
	}
	if err := DefineExpression[int64](env, "suml", Sum[int64](Field[infraTableExpressionBean, int64]("longPrimitive"))); err != nil {
		t.Fatal(err)
	}

	source := From[infraTableExpressionBean](env, "SupportBean")
	aggregatePlan, err := env.Build(source.Aggregate(
		Alias("sumi", ExpressionRef[int](env, "sumi", EventValue[Event]())),
		Alias("sumd", ExpressionRef[float64](env, "sumd")),
		Alias("sumf", Sum[float32](Field[infraTableExpressionBean, float32]("floatPrimitive"))),
		Alias("suml", ExpressionRef[int64](env, "suml")),
	).IntoTable("varaggITFE", StatementName("table-expression-aggregate-input")))
	if err != nil {
		t.Fatal(err)
	}

	readPlan, err := env.Build(
		OnEvent(From[infraTableExpressionTrigger](env, "SupportBean_S0")).
			SelectFromTableWhere("varaggITFE", Literal(true),
				Alias("sumi", TableField[int]("sumi")),
				Alias("sumd", TableField[float64]("sumd")),
				Alias("sumf", TableField[float32]("sumf")),
				Alias("suml", TableField[int64]("suml")),
			).
			Query(StatementName("table-expression-aggregate-read")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), readPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()

	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("declared table aggregate result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	for _, event := range []infraTableExpressionBean{{
		TheString: "E1", IntPrimitive: 10, LongPrimitive: 100,
		DoublePrimitive: 1000, FloatPrimitive: 10000,
	}, {
		TheString: "E1", IntPrimitive: 11, LongPrimitive: 101,
		DoublePrimitive: 1001, FloatPrimitive: 10001,
	}} {
		if err := engine.SendEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(ctx, infraTableExpressionTrigger{ID: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("declared table aggregate rows = %d, want 2", len(rows))
	}
	want := [][4]any{
		{10, float64(1000), float32(10000), int64(100)},
		{21, float64(2001), float32(20001), int64(201)},
	}
	for index, expected := range want {
		for column, name := range []string{"sumi", "sumd", "sumf", "suml"} {
			if got := rows[index].Get(name).Any(); !reflect.DeepEqual(got, expected[column]) {
				t.Fatalf("aggregate row %d %s = %#v, want %#v", index, name, got, expected[column])
			}
		}
	}
}

func testInfraTableAccessExpressionDirectTableAccess(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableExpressionBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableExpressionTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "MyTableOne", []TableColumn{
		PrimaryKeyColumn[string]("theString"),
		TableColumnOf[int]("intPrimitive"),
	}); err != nil {
		t.Fatal(err)
	}

	key := ExpressionParam[string]("key")
	lookup := SubqueryValue[int](FromTable(env, "MyTableOne"),
		Field[any, int]("intPrimitive"),
		Equal[string](Field[any, string]("theString"), key),
	)
	if err := DefineExpression[int](env, "getMyValue", lookup); err != nil {
		t.Fatal(err)
	}

	insertPlan, err := env.Build(OnEvent(From[infraTableExpressionBean](env, "SupportBean")).InsertIntoTable(
		"MyTableOne",
		SetColumn("theString", Field[infraTableExpressionBean, string]("theString")),
		SetColumn("intPrimitive", Field[infraTableExpressionBean, int]("intPrimitive")),
	).Query(StatementName("table-expression-direct-populate")))
	if err != nil {
		t.Fatal(err)
	}
	queryPlan, err := env.Build(Select(
		From[infraTableExpressionTrigger](env, "SupportBean_S0"),
		Alias("c0", ExpressionRef[int](env, "getMyValue", Field[infraTableExpressionTrigger, string]("p00"))),
	).Query(StatementName("table-expression-direct-read")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), queryPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	var values []Value
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("direct declared table result is not a row: %#v", result)
			}
			values = append(values, row.Get("c0"))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, event := range []infraTableExpressionBean{
		{TheString: "E1", IntPrimitive: 1},
		{TheString: "E2", IntPrimitive: 2},
	} {
		if err := engine.SendEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SendEvent(ctx, infraTableExpressionTrigger{ID: 0, P00: "E2"}); err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].Any() != 2 {
		t.Fatalf("direct declared table value = %#v, want 2", values)
	}
}

func testInfraTableAccessExpressionLastEventTableAccess(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableExpressionBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableExpressionTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableExpressionS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "MyTableTwo", []TableColumn{
		PrimaryKeyColumn[string]("theString"),
		TableColumnOf[int]("intPrimitive"),
	}); err != nil {
		t.Fatal(err)
	}

	key := ExpressionParam[string]("key")
	lastEvent := From[infraTableExpressionS1](env, "SupportBean_S1").Window(LastEvent()).AsRecord()
	lookup := SubqueryValue[int](FromTable(env, "MyTableTwo"),
		Field[any, int]("intPrimitive"),
		Equal[string](Field[any, string]("theString"), key),
	)
	declared := IfThenElse[int](
		SubqueryExists(lastEvent, Literal(true)),
		lookup,
		NullLiteral[int](),
	)
	if err := DefineExpression[int](env, "getMyValue", declared); err != nil {
		t.Fatal(err)
	}

	insertPlan, err := env.Build(OnEvent(From[infraTableExpressionBean](env, "SupportBean")).InsertIntoTable(
		"MyTableTwo",
		SetColumn("theString", Field[infraTableExpressionBean, string]("theString")),
		SetColumn("intPrimitive", Field[infraTableExpressionBean, int]("intPrimitive")),
	).Query(StatementName("table-expression-last-event-populate")))
	if err != nil {
		t.Fatal(err)
	}
	queryPlan, err := env.Build(Select(
		From[infraTableExpressionTrigger](env, "SupportBean_S0"),
		Alias("c0", ExpressionRef[int](env, "getMyValue", Field[infraTableExpressionTrigger, string]("p00"))),
	).Query(StatementName("table-expression-last-event-read")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), queryPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	var values []Value
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("lastevent declared table result is not a row: %#v", result)
			}
			values = append(values, row.Get("c0"))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := engine.SendEvent(ctx, infraTableExpressionTrigger{ID: 0, P00: "E2"}); err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || !values[0].IsNull() {
		t.Fatalf("lastevent declared value before S1 = %#v, want null", values)
	}
	if err := engine.SendEvent(ctx, infraTableExpressionS1{ID: 1000}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []infraTableExpressionBean{
		{TheString: "E1", IntPrimitive: 1},
		{TheString: "E2", IntPrimitive: 2},
	} {
		if err := engine.SendEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SendEvent(ctx, infraTableExpressionTrigger{ID: 0, P00: "E2"}); err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[1].Any() != 2 {
		t.Fatalf("lastevent declared table values = %#v, want [null 2]", values)
	}
}

// TestInfraTableAccessFilterBehaviorParity mirrors Java
// InfraTableAccessCore.InfraFilterBehavior:
//
//	create table varaggFB (total count(*))
//	into table varaggFB select count(*) from SupportBean_S0
//	select * from SupportBean(varaggFB.total = intPrimitive)
//
// The Go form keeps the same dataflow explicit: an aggregate statement
// materializes the table, and a typed scalar subquery reads its current row in
// the ordinary event filter.
func TestInfraTableAccessFilterBehaviorParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableFilterS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableFilterBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varaggFB", []TableColumn{
		TableColumnOf[int64]("total"),
	}); err != nil {
		t.Fatal(err)
	}

	countPlan, err := env.Build(
		From[infraTableFilterS0](env, "SupportBean_S0").
			Aggregate(Alias("total", CountAll())).
			IntoTable("varaggFB", StatementName("varaggFB-count")),
	)
	if err != nil {
		t.Fatal(err)
	}
	countDeployment, err := NewEngine(env).Deploy(context.Background(), countPlan)
	if err != nil {
		t.Fatal(err)
	}
	engine := countDeployment.engine
	if engine == nil {
		t.Fatal("count deployment has no engine")
	}

	total := SubqueryValue[int64](
		FromTable(env, "varaggFB"),
		Field[any, int64]("total"),
	)
	filterPlan, err := env.Build(
		From[infraTableFilterBean](env, "SupportBean").
			Filter(Equal[int64](total, Field[infraTableFilterBean, int64]("intPrimitive"))).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	filterDeployment, err := engine.Deploy(context.Background(), filterPlan)
	if err != nil {
		t.Fatal(err)
	}

	var matched []int64
	if _, err := filterDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("filter result is not an event: %#v", result)
			}
			matched = append(matched, event.Get("intPrimitive").Any().(int64))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := engine.SendEvent(ctx, infraTableFilterS0{ID: 0}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableFilterBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableFilterS0{ID: 0}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableFilterBean{TheString: "E1", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableFilterBean{TheString: "E1", IntPrimitive: 3}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableFilterBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}

	want := []int64{1, 2}
	if len(matched) != len(want) {
		t.Fatalf("table filter matches = %v, want %v", matched, want)
	}
	for index, value := range want {
		if matched[index] != value {
			t.Fatalf("table filter match[%d] = %d, want %d (all=%v)", index, matched[index], value, matched)
		}
	}

	table, ok := engine.Table("varaggFB")
	if !ok {
		t.Fatal("varaggFB table is missing")
	}
	rows, err := table.Snapshot(ctx)
	if err != nil || len(rows) != 1 || rows[0].Get("total").Any() != int64(2) {
		t.Fatalf("varaggFB snapshot = %#v, err=%v", rows, err)
	}
}

// TestInfraTableAccessExprSelectClauseRenderingUnnamedColParity mirrors Java
// InfraTableAccessCore.InfraExprSelectClauseRenderingUnnamedCol. The Java
// statement intentionally relies on expression-rendered column names; the Go
// fluent form gives each projection an explicit alias while preserving the
// same five result types and table-access operations.
func TestInfraTableAccessExprSelectClauseRenderingUnnamedColParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableGroupedSingleBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableGroupedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varaggESC", []TableColumn{
		PrimaryKeyColumn[string]("key"),
		TableColumnOf[WindowAccessValue[infraTableGroupedSingleBean]]("theEvents"),
	}); err != nil {
		t.Fatal(err)
	}

	groupKey := Field[infraTableGroupedSingleBean, string]("theString")
	aggregatePlan, err := env.Build(
		From[infraTableGroupedSingleBean](env, "SupportBean").
			Window(KeepAll()).
			GroupBy(groupKey).
			Select(
				Alias("key", groupKey),
				Alias("theEvents", WindowAccessBy[infraTableGroupedSingleBean](EventValue[infraTableGroupedSingleBean]())),
			).
			IntoTable("varaggESC", StatementName("table-access-expression-rendering-populate")),
	)
	if err != nil {
		t.Fatal(err)
	}

	tableEvents := TableField[WindowAccessValue[infraTableGroupedSingleBean]]("theEvents")
	events := Method[[]infraTableGroupedSingleBean](tableEvents, "Values")
	row := StructOf(
		Alias("key", TableField[string]("key")),
		Alias("theEvents", events),
	)
	keys := SubqueryValues[string](FromTable(env, "varaggESC"), Field[any, string]("key"))
	plan, err := env.Build(
		OnEvent(From[infraTableGroupedTrigger](env, "SupportBean_S0")).
			SelectFromTable("varaggESC", []Expr{Field[infraTableGroupedTrigger, string]("p00")},
				Alias("keys", keys),
				Alias("theEvents", events),
				Alias("row", row),
				Alias("last", Method[infraTableGroupedSingleBean](tableEvents, "Last")),
				Alias("takeOne", EnumTake[infraTableGroupedSingleBean](events, 1)),
			).
			Query(StatementName("table-access-expression-rendering")),
	)
	if err != nil {
		t.Fatal(err)
	}

	schema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("table expression-rendering plan has no result schema")
	}
	wantTypes := map[string]reflect.Type{
		"keys":      reflect.TypeOf([]string{}),
		"theEvents": reflect.TypeOf([]infraTableGroupedSingleBean{}),
		"row":       reflect.TypeOf(map[string]any{}),
		"last":      reflect.TypeOf(infraTableGroupedSingleBean{}),
		"takeOne":   reflect.TypeOf([]infraTableGroupedSingleBean{}),
	}
	for name, want := range wantTypes {
		got, exists := schema.PropertyType(name)
		if !exists || got != want {
			t.Fatalf("table expression-rendering schema %s = %v, want %v", name, got, want)
		}
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
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
				t.Fatalf("table expression-rendering result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	input := infraTableGroupedSingleBean{TheString: "E1", IntPrimitive: 10}
	if err := engine.SendEvent(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), infraTableGroupedTrigger{P00: "E1"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("table expression-rendering rows = %d, want 1", len(rows))
	}
	if got := rows[0].Get("keys").Any(); !reflect.DeepEqual(got, []string{"E1"}) {
		t.Fatalf("table expression-rendering keys = %#v, want [E1]", got)
	}
	if got := rows[0].Get("theEvents").Any(); !reflect.DeepEqual(got, []infraTableGroupedSingleBean{input}) {
		t.Fatalf("table expression-rendering events = %#v, want [%#v]", got, input)
	}
	rowValue, ok := rows[0].Get("row").Any().(map[string]any)
	if !ok || !reflect.DeepEqual(rowValue["key"], "E1") || !reflect.DeepEqual(rowValue["theEvents"], []infraTableGroupedSingleBean{input}) {
		t.Fatalf("table expression-rendering row = %#v", rows[0].Get("row").Any())
	}
	if got := rows[0].Get("last").Any(); !reflect.DeepEqual(got, input) {
		t.Fatalf("table expression-rendering last = %#v, want %#v", got, input)
	}
	if got := rows[0].Get("takeOne").Any(); !reflect.DeepEqual(got, []infraTableGroupedSingleBean{input}) {
		t.Fatalf("table expression-rendering take-one = %#v, want [%#v]", got, input)
	}
}

// TestInfraTableAccessUngroupedWindowAndSumParity mirrors Java
// InfraTableAccessCoreUnGroupedWindowAndSum. The table has one unkeyed row
// containing both a length-window access value and a running sum; a trigger
// stream reads that row after each insertion.
func TestInfraTableAccessUngroupedWindowAndSumParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableWindowEvent](env, "MyEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableWindowTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "windowAndTotal", []TableColumn{
		TableColumnOf[WindowAccessValue[infraTableWindowEvent]]("thewindow"),
		TableColumnOf[int64]("thetotal"),
	}); err != nil {
		t.Fatal(err)
	}

	c0 := Field[infraTableWindowEvent, int64]("c0")
	aggregatePlan, err := env.Build(
		From[infraTableWindowEvent](env, "MyEvent").
			Window(LengthWindow(2)).
			Aggregate(
				Alias("thewindow", WindowAccessBy[infraTableWindowEvent](EventValue[infraTableWindowEvent]())),
				Alias("thetotal", Sum[int64](c0)),
			).
			IntoTable("windowAndTotal", StatementName("window-and-total")),
	)
	if err != nil {
		t.Fatal(err)
	}

	triggerPlan, err := env.Build(
		OnEvent(From[infraTableWindowTrigger](env, "SupportBean_S0")).
			SelectFromTableWhere("windowAndTotal", Literal(true),
				Alias("thewindow", TableField[WindowAccessValue[infraTableWindowEvent]]("thewindow")),
				Alias("thetotal", TableField[int64]("thetotal")),
			).
			Query(StatementName("read-window-and-total")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 3)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("table access result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendAndRead := func(value, triggerID int64, wantValues []int64, wantTotal int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), infraTableWindowEvent{C0: value}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), infraTableWindowTrigger{ID: triggerID}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("table access trigger produced no row")
		}
		row := rows[len(rows)-1]
		if row.Get("thetotal").Any() != wantTotal {
			t.Fatalf("table total after %d = %v, want %d", value, row.Get("thetotal"), wantTotal)
		}
		access, ok := row.Get("thewindow").Any().(WindowAccessValue[infraTableWindowEvent])
		if !ok {
			t.Fatalf("table window type = %T", row.Get("thewindow").Any())
		}
		values := access.Values()
		if len(values) != len(wantValues) {
			t.Fatalf("table window after %d = %#v, want %#v", value, values, wantValues)
		}
		for index, want := range wantValues {
			if values[index].C0 != want {
				t.Fatalf("table window after %d index %d = %d, want %d", value, index, values[index].C0, want)
			}
		}
	}

	sendAndRead(10, 0, []int64{10}, 10)
	sendAndRead(20, 1, []int64{10, 20}, 30)
	sendAndRead(30, 2, []int64{20, 30}, 50)
}

// TestInfraTableAccessIntegerIndexedPropertyLookAlikeParity mirrors Java
// InfraTableAccessCore.InfraIntegerIndexedPropertyLookAlike. The Java
// varaggIIP[1] table access is expressed as an explicit typed primary-key
// lookup in the Go trigger chain; the remaining projections keep the row,
// access value and indexed window navigation visible in the plan.
func TestInfraTableAccessIntegerIndexedPropertyLookAlikeParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableIndexedBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableIndexedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varaggIIP", []TableColumn{
		PrimaryKeyColumn[int64]("key"),
		TableColumnOf[WindowAccessValue[infraTableIndexedBean]]("myevents"),
	}); err != nil {
		t.Fatal(err)
	}

	key := Field[infraTableIndexedBean, int64]("intPrimitive")
	window := WindowAccessBy[infraTableIndexedBean](EventValue[infraTableIndexedBean]())
	aggregatePlan, err := env.Build(
		From[infraTableIndexedBean](env, "SupportBean").
			Window(LengthWindow(3)).
			GroupBy(key).
			Select(
				Alias("key", key),
				Alias("myevents", window),
			).
			IntoTable("varaggIIP", StatementName("varagg-iip")),
	)
	if err != nil {
		t.Fatal(err)
	}

	tableField := TableField[WindowAccessValue[infraTableIndexedBean]]("myevents")
	values := Method[[]infraTableIndexedBean](tableField, "Values")
	triggerPlan, err := env.Build(
		OnEvent(From[infraTableIndexedTrigger](env, "SupportBean_S0")).
			SelectFromTable("varaggIIP", []Expr{Literal[int64](1)},
				Alias("c0", EventValue[Event]()),
				Alias("c1", tableField),
				Alias("c2", Method[infraTableIndexedBean](tableField, "Last")),
				Alias("c3", ArrayAt[infraTableIndexedBean](values, Subtract[int64](ArraySize(values), Literal[int64](2)))),
			).
			Query(StatementName("read-varagg-iip")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("integer-indexed table result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	e1 := infraTableIndexedBean{TheString: "E1", IntPrimitive: 1, LongPrimitive: 10}
	e2 := infraTableIndexedBean{TheString: "E2", IntPrimitive: 1, LongPrimitive: 20}
	ctx := context.Background()
	if err := engine.SendEvent(ctx, e1); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, e2); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableIndexedTrigger{ID: 0}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("integer-indexed table result count = %d, want 1", len(rows))
	}

	row := rows[0]
	c0, ok := row.Get("c0").Any().(Event)
	if !ok {
		t.Fatalf("c0 type = %T, want Event", row.Get("c0").Any())
	}
	c0Access, ok := c0.Get("myevents").Any().(WindowAccessValue[infraTableIndexedBean])
	if !ok {
		t.Fatalf("c0.myevents type = %T", c0.Get("myevents").Any())
	}
	c1, ok := row.Get("c1").Any().(WindowAccessValue[infraTableIndexedBean])
	if !ok {
		t.Fatalf("c1 type = %T", row.Get("c1").Any())
	}
	for name, access := range map[string]WindowAccessValue[infraTableIndexedBean]{"c0.myevents": c0Access, "c1": c1} {
		got := access.Values()
		if len(got) != 2 || got[0] != e1 || got[1] != e2 {
			t.Fatalf("%s = %#v, want [%#v %#v]", name, got, e1, e2)
		}
	}
	if got, ok := row.Get("c2").Any().(infraTableIndexedBean); !ok || got != e2 {
		t.Fatalf("c2 = %#v, want %#v", row.Get("c2").Any(), e2)
	}
	if got, ok := row.Get("c3").Any().(infraTableIndexedBean); !ok || got != e1 {
		t.Fatalf("c3 = %#v, want %#v", row.Get("c3").Any(), e1)
	}
}

// TestInfraTableAccessTopLevelReadUngroupedParity mirrors Java
// InfraTableAccessCore.InfraTopLevelReadUnGrouped. The top-level table row is
// represented as an explicit Go map projection, while the object-array event
// remains a schema-bound Event inside the window access value.
func TestInfraTableAccessTopLevelReadUngroupedParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterObjectArray(env, "MyEventOATLRU", []FieldSpec{
		FieldDef("c0", reflect.TypeOf(int64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableWindowTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "windowAndTotalTLRUG", []TableColumn{
		TableColumnOf[WindowAccessValue[Event]]("thewindow"),
		TableColumnOf[int64]("thetotal"),
	}); err != nil {
		t.Fatal(err)
	}

	window := WindowAccessBy[Event](EventValue[Event]())
	aggregatePlan, err := env.Build(
		FromAny(env, "MyEventOATLRU").
			Window(LengthWindow(2)).
			Aggregate(
				Alias("thewindow", window),
				Alias("thetotal", Sum[int64](Field[any, int64]("c0"))),
			).
			IntoTable("windowAndTotalTLRUG", StatementName("tlrug-aggregate")),
	)
	if err != nil {
		t.Fatal(err)
	}

	tableWindow := TableField[WindowAccessValue[Event]]("thewindow")
	rowProjection := StructOf(
		Alias("thewindow", Method[[]Event](tableWindow, "Values")),
		Alias("thetotal", TableField[int64]("thetotal")),
	)
	triggerPlan, err := env.Build(
		OnEvent(From[infraTableWindowTrigger](env, "SupportBean_S0")).
			SelectFromTableWhere("windowAndTotalTLRUG", Literal(true), Alias("val0", rowProjection)).
			Query(StatementName("tlrug-read")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("top-level table result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	sendAndRead := func(value, triggerID int64, wantValues []int64, wantTotal int64) {
		t.Helper()
		if err := engine.SendObjectArray(ctx, "MyEventOATLRU", []any{value}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(ctx, infraTableWindowTrigger{ID: triggerID}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("top-level table trigger produced no row")
		}
		row := rows[len(rows)-1]
		val0, ok := row.Get("val0").Any().(map[string]any)
		if !ok {
			t.Fatalf("val0 type = %T", row.Get("val0").Any())
		}
		if got := val0["thetotal"]; got != wantTotal {
			t.Fatalf("top-level table total after %d = %#v, want %d", value, got, wantTotal)
		}
		events, ok := val0["thewindow"].([]Event)
		if !ok {
			t.Fatalf("top-level table window type = %T", val0["thewindow"])
		}
		if len(events) != len(wantValues) {
			t.Fatalf("top-level table window after %d = %d events, want %d", value, len(events), len(wantValues))
		}
		for index, want := range wantValues {
			if got := events[index].Get("c0").Any(); got != want {
				t.Fatalf("top-level table window after %d index %d = %#v, want %d", value, index, got, want)
			}
		}
	}

	sendAndRead(10, 0, []int64{10}, 10)
	sendAndRead(20, 1, []int64{10, 20}, 30)
	sendAndRead(30, 2, []int64{20, 30}, 50)
}

// TestInfraTableAccessTopLevelReadGroupedTwoKeysParity mirrors Java
// InfraTableAccessCore.InfraTopLevelReadGrouped2Keys. The two primary-key
// columns are looked up explicitly and a missing group is represented by the
// same present-but-null map fields used by the ungrouped table projection.
func TestInfraTableAccessTopLevelReadGroupedTwoKeysParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterObjectArray(env, "MyEventOA", []FieldSpec{
		FieldDef("c0", reflect.TypeOf(int64(0))),
		FieldDef("c1", reflect.TypeOf("")),
		FieldDef("c2", reflect.TypeOf(int64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableGroupedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "windowAndTotalTLP2K", []TableColumn{
		PrimaryKeyColumn[int64]("keyi"),
		PrimaryKeyColumn[string]("keys"),
		TableColumnOf[WindowAccessValue[Event]]("thewindow"),
		TableColumnOf[int64]("thetotal"),
	}); err != nil {
		t.Fatal(err)
	}

	keyInt := Field[any, int64]("c0")
	keyString := Field[any, string]("c1")
	window := WindowAccessBy[Event](EventValue[Event]())
	aggregatePlan, err := env.Build(
		FromAny(env, "MyEventOA").
			Window(LengthWindow(2)).
			GroupBy(keyInt, keyString).
			Select(
				Alias("keyi", keyInt),
				Alias("keys", keyString),
				Alias("thewindow", window),
				Alias("thetotal", Sum[int64](Field[any, int64]("c2"))),
			).
			IntoTable("windowAndTotalTLP2K", StatementName("tlp2k-aggregate")),
	)
	if err != nil {
		t.Fatal(err)
	}

	tableWindow := TableField[WindowAccessValue[Event]]("thewindow")
	rowProjection := StructOf(
		Alias("thewindow", Method[[]Event](tableWindow, "Values")),
		Alias("thetotal", TableField[int64]("thetotal")),
	)
	triggerPlan, err := env.Build(
		OnEvent(From[infraTableGroupedTrigger](env, "SupportBean_S0")).
			SelectFromTable("windowAndTotalTLP2K", []Expr{
				Field[infraTableGroupedTrigger, int64]("id"),
				Field[infraTableGroupedTrigger, string]("p00"),
			}, Alias("val0", rowProjection)).
			Query(StatementName("tlp2k-read")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("grouped top-level table result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	read := func(event []any, id int64, key string) map[string]any {
		t.Helper()
		if event != nil {
			if err := engine.SendObjectArray(ctx, "MyEventOA", event); err != nil {
				t.Fatal(err)
			}
		}
		if err := engine.SendEvent(ctx, infraTableGroupedTrigger{ID: id, P00: key}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("grouped top-level table trigger produced no row")
		}
		value := rows[len(rows)-1].Get("val0").Any()
		projected, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("grouped val0 type = %T", value)
		}
		return projected
	}
	assertGroup := func(projected map[string]any, wantValues []int64, wantTotal any) {
		t.Helper()
		if projected["thetotal"] != wantTotal {
			t.Fatalf("grouped table total = %#v, want %#v", projected["thetotal"], wantTotal)
		}
		events, ok := projected["thewindow"].([]Event)
		if wantValues == nil {
			if projected["thewindow"] != nil {
				t.Fatalf("missing grouped table window = %#v, want nil", projected["thewindow"])
			}
			return
		}
		if !ok || len(events) != len(wantValues) {
			t.Fatalf("grouped table window = %#v, want %d events", projected["thewindow"], len(wantValues))
		}
		for index, want := range wantValues {
			if got := events[index].Get("c0").Any(); got != want {
				t.Fatalf("grouped table window index %d = %#v, want %d", index, got, want)
			}
		}
	}

	assertGroup(read([]any{int64(10), "G1", int64(100)}, 10, "G1"), []int64{10}, int64(100))
	assertGroup(read([]any{int64(20), "G2", int64(200)}, 20, "G2"), []int64{20}, int64(200))
	missing := read([]any{int64(20), "G2", int64(300)}, 10, "G1")
	assertGroup(missing, nil, nil)
	assertGroup(read(nil, 20, "G2"), []int64{20, 20}, int64(500))
}

// TestInfraTableAccessGroupedSingleKeyNoContextParity mirrors Java
// InfraTableAccessCore.InfraGroupedSingleKeyNoContext. The table's grouped
// aggregate is addressed by one typed primary-key expression. The final Z
// lookup also fixes the present-but-null behavior for a missing key.
func TestInfraTableAccessGroupedSingleKeyNoContextParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableGroupedSingleBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableGroupedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varTotalG1K", []TableColumn{
		PrimaryKeyColumn[string]("key"),
		TableColumnOf[int64]("total"),
	}); err != nil {
		t.Fatal(err)
	}

	key := Field[infraTableGroupedSingleBean, string]("theString")
	aggregatePlan, err := env.Build(
		From[infraTableGroupedSingleBean](env, "SupportBean").
			GroupBy(key).
			Select(
				Alias("key", key),
				Alias("total", Sum[int64](Field[infraTableGroupedSingleBean, int64]("intPrimitive"))),
			).
			IntoTable("varTotalG1K", StatementName("grouped-single-aggregate")),
	)
	if err != nil {
		t.Fatal(err)
	}

	triggerKey := Field[infraTableGroupedTrigger, string]("p00")
	triggerPlan, err := env.Build(
		OnEvent(From[infraTableGroupedTrigger](env, "SupportBean_S0")).
			SelectFromTable("varTotalG1K", []Expr{triggerKey},
				Alias("c0", OuterField[string]("p00")),
				Alias("c1", TableField[int64]("total")),
			).
			Query(StatementName("grouped-single-read")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("grouped single-key result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	sendAndAssert := func(name string, value int64, wantTotal any) {
		t.Helper()
		if err := engine.SendEvent(ctx, infraTableGroupedSingleBean{TheString: name, IntPrimitive: value}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(ctx, infraTableGroupedTrigger{P00: name}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("grouped single-key trigger produced no row")
		}
		row := rows[len(rows)-1]
		if row.Get("c0").Any() != name || row.Get("c1").Any() != wantTotal {
			t.Fatalf("grouped single-key result for %s = %#v, want total %#v", name, row.AsMap(), wantTotal)
		}
	}
	sendAndRead := func(name string, wantTotal any) {
		t.Helper()
		if err := engine.SendEvent(ctx, infraTableGroupedTrigger{P00: name}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("grouped single-key lookup produced no row")
		}
		row := rows[len(rows)-1]
		if row.Get("c0").Any() != name || row.Get("c1").Any() != wantTotal {
			t.Fatalf("grouped single-key lookup for %s = %#v, want total %#v", name, row.AsMap(), wantTotal)
		}
	}

	sendAndAssert("A", 10, int64(10))
	sendAndAssert("A", 11, int64(21))
	if err := engine.SendEvent(ctx, infraTableGroupedSingleBean{TheString: "B", IntPrimitive: 20}); err != nil {
		t.Fatal(err)
	}
	sendAndRead("A", int64(21))
	if err := engine.SendEvent(ctx, infraTableGroupedSingleBean{TheString: "B", IntPrimitive: 21}); err != nil {
		t.Fatal(err)
	}
	sendAndRead("B", int64(41))
	if err := engine.SendEvent(ctx, infraTableGroupedSingleBean{TheString: "C", IntPrimitive: 30}); err != nil {
		t.Fatal(err)
	}
	sendAndRead("A", int64(21))
	if err := engine.SendEvent(ctx, infraTableGroupedSingleBean{TheString: "D", IntPrimitive: 40}); err != nil {
		t.Fatal(err)
	}
	sendAndRead("C", int64(30))
	sendAndRead("D", int64(40))
	sendAndRead("Z", nil)
}

// TestInfraTableAccessGroupedTwoKeyNoContextParity mirrors Java
// InfraTableAccessCore.InfraGroupedTwoKeyNoContext. Both primary-key
// expressions are evaluated from the trigger event and missing combinations
// retain the table selector's present-null projection semantics.
func TestInfraTableAccessGroupedTwoKeyNoContextParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableGroupedMultiBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableGroupedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varTotalG2K", []TableColumn{
		PrimaryKeyColumn[string]("key0"),
		PrimaryKeyColumn[int64]("key1"),
		TableColumnOf[int64]("total"),
		TableColumnOf[int64]("cnt"),
	}); err != nil {
		t.Fatal(err)
	}

	groupString := Field[infraTableGroupedMultiBean, string]("theString")
	groupInt := Field[infraTableGroupedMultiBean, int64]("intPrimitive")
	aggregatePlan, err := env.Build(
		From[infraTableGroupedMultiBean](env, "SupportBean").
			GroupBy(groupString, groupInt).
			Select(
				Alias("key0", groupString),
				Alias("key1", groupInt),
				Alias("total", Sum[int64](Field[infraTableGroupedMultiBean, int64]("longPrimitive"))),
				Alias("cnt", CountAll()),
			).
			IntoTable("varTotalG2K", StatementName("grouped-two-aggregate")),
	)
	if err != nil {
		t.Fatal(err)
	}

	triggerString := Field[infraTableGroupedTrigger, string]("p00")
	triggerInt := Field[infraTableGroupedTrigger, int64]("id")
	triggerPlan, err := env.Build(
		OnEvent(From[infraTableGroupedTrigger](env, "SupportBean_S0")).
			SelectFromTable("varTotalG2K", []Expr{triggerString, triggerInt},
				Alias("c0", TableField[int64]("total")),
				Alias("c1", TableField[int64]("cnt")),
			).
			Query(StatementName("grouped-two-read")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("grouped two-key result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	read := func(key string, id int64, wantTotal, wantCount any) {
		t.Helper()
		if err := engine.SendEvent(ctx, infraTableGroupedTrigger{P00: key, ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("grouped two-key lookup produced no row")
		}
		row := rows[len(rows)-1]
		if row.Get("c0").Any() != wantTotal || row.Get("c1").Any() != wantCount {
			t.Fatalf("grouped two-key lookup %s/%d = %#v, want %v/%v", key, id, row.AsMap(), wantTotal, wantCount)
		}
	}

	if err := engine.SendEvent(ctx, infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100}); err != nil {
		t.Fatal(err)
	}
	read("E1", 10, int64(100), int64(1))
	read("E1", 0, nil, nil)
	read("E2", 10, nil, nil)
}

// TestInfraTableAccessGroupedThreeKeyNoContextParity mirrors Java
// InfraTableAccessCore.InfraGroupedThreeKeyNoContext. The third key is a
// literal in the trigger lookup, matching the Java 100L access expression.
func TestInfraTableAccessGroupedThreeKeyNoContextParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableGroupedMultiBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableGroupedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varTotalG3K", []TableColumn{
		PrimaryKeyColumn[string]("key0"),
		PrimaryKeyColumn[int64]("key1"),
		PrimaryKeyColumn[int64]("key2"),
		TableColumnOf[float64]("total"),
		TableColumnOf[int64]("cnt"),
	}); err != nil {
		t.Fatal(err)
	}

	groupString := Field[infraTableGroupedMultiBean, string]("theString")
	groupInt := Field[infraTableGroupedMultiBean, int64]("intPrimitive")
	groupLong := Field[infraTableGroupedMultiBean, int64]("longPrimitive")
	aggregatePlan, err := env.Build(
		From[infraTableGroupedMultiBean](env, "SupportBean").
			GroupBy(groupString, groupInt, groupLong).
			Select(
				Alias("key0", groupString),
				Alias("key1", groupInt),
				Alias("key2", groupLong),
				Alias("total", Sum[float64](Field[infraTableGroupedMultiBean, float64]("doublePrimitive"))),
				Alias("cnt", CountAll()),
			).
			IntoTable("varTotalG3K", StatementName("grouped-three-aggregate")),
	)
	if err != nil {
		t.Fatal(err)
	}

	triggerString := Field[infraTableGroupedTrigger, string]("p00")
	triggerInt := Field[infraTableGroupedTrigger, int64]("id")
	triggerPlan, err := env.Build(
		OnEvent(From[infraTableGroupedTrigger](env, "SupportBean_S0")).
			SelectFromTable("varTotalG3K", []Expr{triggerString, triggerInt, Literal[int64](100)},
				Alias("c0", TableField[float64]("total")),
				Alias("c1", TableField[int64]("cnt")),
			).
			Query(StatementName("grouped-three-read")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("grouped three-key result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	read := func(key string, id int64, wantTotal float64, wantCount int64) {
		t.Helper()
		if err := engine.SendEvent(ctx, infraTableGroupedTrigger{P00: key, ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("grouped three-key lookup produced no row")
		}
		row := rows[len(rows)-1]
		if row.Get("c0").Any() != wantTotal || row.Get("c1").Any() != wantCount {
			t.Fatalf("grouped three-key lookup %s/%d = %#v, want %v/%v", key, id, row.AsMap(), wantTotal, wantCount)
		}
	}

	if err := engine.SendEvent(ctx, infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100, DoublePrimitive: 1000}); err != nil {
		t.Fatal(err)
	}
	read("E1", 10, 1000, 1)
	if err := engine.SendEvent(ctx, infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100, DoublePrimitive: 1001}); err != nil {
		t.Fatal(err)
	}
	read("E1", 10, 2001, 2)
}

// TestInfraTableAccessGroupedMixedMethodAndAccessParity mirrors Java
// InfraTableAccessCore.InfraGroupedMixedMethodAndAccess. The grouped table
// keeps scalar aggregates and an insertion-ordered window access value in the
// same row, allowing a typed table lookup to read both ordinary aggregate
// columns and access-aggregate state.
func TestInfraTableAccessGroupedMixedMethodAndAccessParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableGroupedMultiBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableGroupedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varMyAgg", []TableColumn{
		PrimaryKeyColumn[string]("key"),
		TableColumnOf[int64]("c0"),
		TableColumnOf[int64]("c1"),
		TableColumnOf[WindowAccessValue[infraTableGroupedMultiBean]]("c2"),
		TableColumnOf[int64]("c3"),
	}); err != nil {
		t.Fatal(err)
	}

	groupKey := Field[infraTableGroupedMultiBean, string]("theString")
	intValue := Field[infraTableGroupedMultiBean, int64]("intPrimitive")
	longValue := Field[infraTableGroupedMultiBean, int64]("longPrimitive")
	window := WindowAccessBy[infraTableGroupedMultiBean](EventValue[infraTableGroupedMultiBean]())
	aggregatePlan, err := env.Build(
		From[infraTableGroupedMultiBean](env, "SupportBean").
			Window(LengthWindow(3)).
			GroupBy(groupKey).
			Select(
				Alias("key", groupKey),
				Alias("c0", CountAll()),
				Alias("c1", CountDistinct[int64](intValue)),
				Alias("c2", window),
				Alias("c3", Sum[int64](longValue)),
			).
			IntoTable("varMyAgg", StatementName("grouped-mixed-aggregate")),
	)
	if err != nil {
		t.Fatal(err)
	}

	triggerKey := Field[infraTableGroupedTrigger, string]("p00")
	triggerPlan, err := env.Build(
		OnEvent(From[infraTableGroupedTrigger](env, "SupportBean_S0")).
			SelectFromTable("varMyAgg", []Expr{triggerKey},
				Alias("c0", TableField[int64]("c0")),
				Alias("c1", TableField[int64]("c1")),
				Alias("c2", TableField[WindowAccessValue[infraTableGroupedMultiBean]]("c2")),
				Alias("c3", TableField[int64]("c3")),
			).
			Query(StatementName("grouped-mixed-read")),
	)
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := triggerPlan.ResultSchema()
	if !ok {
		t.Fatal("grouped mixed table read has no result schema")
	}
	for name, want := range map[string]reflect.Type{
		"c0": reflect.TypeOf(int64(0)),
		"c1": reflect.TypeOf(int64(0)),
		"c2": reflect.TypeOf(WindowAccessValue[infraTableGroupedMultiBean]{}),
		"c3": reflect.TypeOf(int64(0)),
	} {
		got, exists := resultSchema.PropertyType(name)
		if !exists || got != want {
			t.Fatalf("grouped mixed result schema %s = %v, want %v", name, got, want)
		}
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("grouped mixed result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	read := func(key string, wantCount, wantDistinct, wantSum any, wantValues []infraTableGroupedMultiBean) {
		t.Helper()
		if err := engine.SendEvent(ctx, infraTableGroupedTrigger{P00: key}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("grouped mixed lookup produced no row")
		}
		row := rows[len(rows)-1]
		if row.Get("c0").Any() != wantCount || row.Get("c1").Any() != wantDistinct || row.Get("c3").Any() != wantSum {
			t.Fatalf("grouped mixed lookup %s = %#v, want count/distinct/sum %v/%v/%v", key, row.AsMap(), wantCount, wantDistinct, wantSum)
		}
		if wantValues == nil {
			if row.Get("c2").State() != ValueNull {
				t.Fatalf("grouped mixed missing access value %s = %#v, want null", key, row.Get("c2"))
			}
			return
		}
		access, ok := row.Get("c2").Any().(WindowAccessValue[infraTableGroupedMultiBean])
		if !ok || !reflect.DeepEqual(access.Values(), wantValues) {
			t.Fatalf("grouped mixed window %s = %#v, want %#v", key, row.Get("c2").Any(), wantValues)
		}
	}

	e1 := infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100}
	e2 := infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 11, LongPrimitive: 101}
	e3 := infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 10, LongPrimitive: 102}
	for _, event := range []infraTableGroupedMultiBean{e1, e2, e3} {
		if err := engine.SendEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	read("E1", int64(3), int64(2), int64(303), []infraTableGroupedMultiBean{e1, e2, e3})
	read("E2", nil, nil, nil, nil)

	e4 := infraTableGroupedMultiBean{TheString: "E2", IntPrimitive: 20, LongPrimitive: 200}
	if err := engine.SendEvent(ctx, e4); err != nil {
		t.Fatal(err)
	}
	read("E2", int64(1), int64(1), int64(200), []infraTableGroupedMultiBean{e4})
}

// TestInfraTableAccessSplitStreamParity mirrors Java
// InfraTableAccessCore.InfraTableAccessCoreSplitStream. The table is populated
// by a typed on-event mutation, then each split branch performs a typed table
// subquery and routes its projection to the same output event stream.
func TestInfraTableAccessSplitStreamParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableGroupedSingleBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableSplitTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "MyTable", []TableColumn{
		PrimaryKeyColumn[string]("k1"),
		TableColumnOf[int64]("c1"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "AStream", []FieldSpec{
		FieldDef("c0", reflect.TypeOf(int64(0))),
	}); err != nil {
		t.Fatal(err)
	}

	support := From[infraTableGroupedSingleBean](env, "SupportBean")
	insertPlan, err := env.Build(OnEvent(support).InsertIntoTable("MyTable",
		SetColumn("k1", Field[infraTableGroupedSingleBean, string]("theString")),
		SetColumn("c1", Field[infraTableGroupedSingleBean, int64]("intPrimitive")),
	).Query(StatementName("table-split-populate")))
	if err != nil {
		t.Fatal(err)
	}

	lookup := func(key string) Expression[int64] {
		return SubqueryValue[int64](
			FromTable(env, "MyTable"),
			Field[any, int64]("c1"),
			Equal[string](Field[any, string]("k1"), Literal(key)),
		)
	}
	trigger := From[infraTableSplitTrigger](env, "SupportBean_S0")
	triggerID := Field[infraTableSplitTrigger, int64]("id")
	splitPlan, err := env.Build(OnEvent(trigger).SplitAll(
		SplitIntoWhen(Equal[int64](triggerID, Literal[int64](1)), "AStream", Alias("c0", lookup("A"))),
		SplitIntoWhen(Equal[int64](triggerID, Literal[int64](2)), "AStream", Alias("c0", lookup("B"))),
	).Query(StatementName("table-access-split-stream")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromAny(env, "AStream").Query(StatementName("table-access-split-consumer")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), splitPlan); err != nil {
		t.Fatal(err)
	}
	var values []int64
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("split-stream result is not an event: %#v", result)
			}
			values = append(values, event.Get("c0").Any().(int64))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	for _, event := range []infraTableGroupedSingleBean{
		{TheString: "A", IntPrimitive: 10},
		{TheString: "B", IntPrimitive: 20},
	} {
		if err := engine.SendEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []int64{1, 2, 3} {
		if err := engine.SendEvent(ctx, infraTableSplitTrigger{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(values, []int64{10, 20}) {
		t.Fatalf("table split-stream values = %#v, want [10 20]", values)
	}
}

// TestInfraTableAccessMultikeyArrayOneArrayKeyParity mirrors Java
// InfraTableAccessCore.InfraTableAccessMultikeyWArrayOneArrayKey. A slice
// primary key is evaluated by contents, so distinct array values occupy
// independent rows and a later lookup can retrieve the matching value.
func TestInfraTableAccessMultikeyArrayOneArrayKeyParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableArrayKeyEvent](env, "SupportEventWithManyArray"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableArrayKeyTrigger](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "MyTable", []TableColumn{
		PrimaryKeyColumn[[]int]("k"),
		TableColumnOf[int64]("value"),
	}); err != nil {
		t.Fatal(err)
	}

	source := From[infraTableArrayKeyEvent](env, "SupportEventWithManyArray")
	insertPlan, err := env.Build(
		OnEvent(source.Filter(Equal[string](
			Field[infraTableArrayKeyEvent, string]("id"),
			Literal("I"),
		))).InsertIntoTable("MyTable",
			SetColumn("k", Field[infraTableArrayKeyEvent, []int]("intOne")),
			SetColumn("value", Field[infraTableArrayKeyEvent, int64]("value")),
		).Query(StatementName("table-array-key-populate")),
	)
	if err != nil {
		t.Fatal(err)
	}

	queryPlan, err := env.Build(
		OnEvent(source.Filter(Equal[string](
			Field[infraTableArrayKeyEvent, string]("id"),
			Literal("Q"),
		))).SelectFromTable("MyTable", []Expr{
			Field[infraTableArrayKeyEvent, []int]("intOne"),
		}, Alias("c0", TableField[int64]("value"))).
			Query(StatementName("table-array-key-read")),
	)
	if err != nil {
		t.Fatal(err)
	}

	keys := SubqueryValues[[]int](
		FromTable(env, "MyTable"),
		Field[any, []int]("k"),
	)
	keysPlan, err := env.Build(
		Select(From[infraTableArrayKeyTrigger](env, "SupportBean"), Alias("keys", keys)).
			Query(StatementName("table-array-key-keys")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	queryDeployment, err := engine.Deploy(context.Background(), queryPlan)
	if err != nil {
		t.Fatal(err)
	}
	keysDeployment, err := engine.Deploy(context.Background(), keysPlan)
	if err != nil {
		t.Fatal(err)
	}

	var values []Value
	if _, err := queryDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("single-array table lookup result is not a row: %#v", result)
			}
			values = append(values, row.Get("c0"))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var keySnapshots [][][]int
	if _, err := keysDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("single-array keys result is not a row: %#v", result)
			}
			got, ok := row.Get("keys").Any().([][]int)
			if !ok {
				t.Fatalf("single-array keys type = %T, want [][]int", row.Get("keys").Any())
			}
			keySnapshots = append(keySnapshots, got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	for _, event := range []infraTableArrayKeyEvent{
		{ID: "I", IntOne: []int{1, 2}, Value: 10},
		{ID: "I", IntOne: []int{2, 1}, Value: 20},
		{ID: "I", IntOne: []int{1, 2, 1}, Value: 30},
	} {
		if err := engine.SendEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}

	query := func(key []int, want any) {
		t.Helper()
		if err := engine.SendEvent(ctx, infraTableArrayKeyEvent{ID: "Q", IntOne: key}); err != nil {
			t.Fatal(err)
		}
		if len(values) == 0 {
			t.Fatal("single-array table lookup produced no row")
		}
		got := values[len(values)-1]
		if want == nil {
			if !got.IsNull() {
				t.Fatalf("single-array lookup %v = %#v, want null", key, got)
			}
			return
		}
		if got.Any() != want {
			t.Fatalf("single-array lookup %v = %#v, want %v", key, got.Any(), want)
		}
	}
	query([]int{1, 2}, int64(10))
	query([]int{1, 2, 1}, int64(30))
	query([]int{2, 1}, int64(20))
	query([]int{1, 2, 2}, nil)

	if err := engine.SendEvent(ctx, infraTableArrayKeyTrigger{}); err != nil {
		t.Fatal(err)
	}
	if len(keySnapshots) != 1 {
		t.Fatalf("single-array keys snapshots = %d, want 1", len(keySnapshots))
	}
	wantKeys := [][]int{{1, 2}, {2, 1}, {1, 2, 1}}
	gotKeys := keySnapshots[0]
	if len(gotKeys) != len(wantKeys) {
		t.Fatalf("single-array keys = %#v, want %#v", gotKeys, wantKeys)
	}
	used := make([]bool, len(wantKeys))
	for _, got := range gotKeys {
		found := false
		for index, want := range wantKeys {
			if !used[index] && reflect.DeepEqual(got, want) {
				used[index] = true
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("single-array keys contains unexpected key %v: %#v", got, gotKeys)
		}
	}
}

// TestInfraTableAccessMultikeyArrayTwoArrayKeyParity mirrors Java
// InfraTableAccessCore.InfraTableAccessMultikeyWArrayTwoArrayKey. Two slice
// primary-key columns form one composite identity: changing either array
// selects a different table row, while a later insert with the same pair
// replaces only that pair.
func TestInfraTableAccessMultikeyArrayTwoArrayKeyParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableArrayPairEvent](env, "SupportEventWithManyArray"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableArrayPairTrigger](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "MyTable", []TableColumn{
		PrimaryKeyColumn[[]int]("k1"),
		PrimaryKeyColumn[[]int]("k2"),
		TableColumnOf[int64]("value"),
	}); err != nil {
		t.Fatal(err)
	}

	source := From[infraTableArrayPairEvent](env, "SupportEventWithManyArray")
	insertPlan, err := env.Build(
		OnEvent(source.Filter(Equal[string](
			Field[infraTableArrayPairEvent, string]("id"),
			Literal("I"),
		))).InsertIntoTable("MyTable",
			SetColumn("k1", Field[infraTableArrayPairEvent, []int]("intOne")),
			SetColumn("k2", Field[infraTableArrayPairEvent, []int]("intTwo")),
			SetColumn("value", Field[infraTableArrayPairEvent, int64]("value")),
		).Query(StatementName("table-array-pair-populate")),
	)
	if err != nil {
		t.Fatal(err)
	}

	queryPlan, err := env.Build(
		OnEvent(source.Filter(Equal[string](
			Field[infraTableArrayPairEvent, string]("id"),
			Literal("Q"),
		))).SelectFromTable("MyTable", []Expr{
			Field[infraTableArrayPairEvent, []int]("intOne"),
			Field[infraTableArrayPairEvent, []int]("intTwo"),
		}, Alias("c0", TableField[int64]("value"))).
			Query(StatementName("table-array-pair-read")),
	)
	if err != nil {
		t.Fatal(err)
	}

	keys := SubqueryValues[infraTableArrayPairKey](
		FromTable(env, "MyTable"),
		Func2[[]int, []int, infraTableArrayPairKey]("table-key-pair", func(first, second []int) infraTableArrayPairKey {
			return infraTableArrayPairKey{K1: first, K2: second}
		}, Field[any, []int]("k1"), Field[any, []int]("k2")),
	)
	keysPlan, err := env.Build(
		Select(From[infraTableArrayPairTrigger](env, "SupportBean"), Alias("keys", keys)).
			Query(StatementName("table-array-pair-keys")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	queryDeployment, err := engine.Deploy(context.Background(), queryPlan)
	if err != nil {
		t.Fatal(err)
	}
	keysDeployment, err := engine.Deploy(context.Background(), keysPlan)
	if err != nil {
		t.Fatal(err)
	}

	var values []Value
	if _, err := queryDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("two-array table lookup result is not a row: %#v", result)
			}
			values = append(values, row.Get("c0"))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var keySnapshots [][]infraTableArrayPairKey
	if _, err := keysDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("two-array keys result is not a row: %#v", result)
			}
			got, ok := row.Get("keys").Any().([]infraTableArrayPairKey)
			if !ok {
				t.Fatalf("two-array keys type = %T, want []infraTableArrayPairKey", row.Get("keys").Any())
			}
			keySnapshots = append(keySnapshots, got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	for _, event := range []infraTableArrayPairEvent{
		{ID: "I", IntOne: []int{1, 2}, IntTwo: []int{1, 2}, Value: 10},
		{ID: "I", IntOne: []int{1, 3}, IntTwo: []int{1, 1}, Value: 20},
		{ID: "I", IntOne: []int{1, 2}, IntTwo: []int{1, 1}, Value: 30},
	} {
		if err := engine.SendEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}

	query := func(first, second []int, want any) {
		t.Helper()
		if err := engine.SendEvent(ctx, infraTableArrayPairEvent{ID: "Q", IntOne: first, IntTwo: second}); err != nil {
			t.Fatal(err)
		}
		if len(values) == 0 {
			t.Fatal("two-array table lookup produced no row")
		}
		got := values[len(values)-1]
		if want == nil {
			if !got.IsNull() {
				t.Fatalf("two-array lookup %v/%v = %#v, want null", first, second, got)
			}
			return
		}
		if got.Any() != want {
			t.Fatalf("two-array lookup %v/%v = %#v, want %v", first, second, got.Any(), want)
		}
	}
	query([]int{1, 2}, []int{1, 2}, int64(10))
	query([]int{1, 2}, []int{1, 1}, int64(30))
	query([]int{1, 3}, []int{1, 1}, int64(20))
	query([]int{1, 2}, []int{1, 2, 2}, nil)

	if err := engine.SendEvent(ctx, infraTableArrayPairTrigger{}); err != nil {
		t.Fatal(err)
	}
	if len(keySnapshots) != 1 {
		t.Fatalf("two-array keys snapshots = %d, want 1", len(keySnapshots))
	}
	wantKeys := []infraTableArrayPairKey{
		{K1: []int{1, 2}, K2: []int{1, 2}},
		{K1: []int{1, 3}, K2: []int{1, 1}},
		{K1: []int{1, 2}, K2: []int{1, 1}},
	}
	gotKeys := keySnapshots[0]
	if len(gotKeys) != len(wantKeys) {
		t.Fatalf("two-array keys = %#v, want %#v", gotKeys, wantKeys)
	}
	used := make([]bool, len(wantKeys))
	for _, got := range gotKeys {
		found := false
		for index, want := range wantKeys {
			if !used[index] && reflect.DeepEqual(got, want) {
				used[index] = true
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("two-array keys contains unexpected pair %v: %#v", got, gotKeys)
		}
	}
}
