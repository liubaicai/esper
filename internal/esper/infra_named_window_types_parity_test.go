package esper

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

type infraNWTSupportBean struct {
	TheString     string `esper:"theString"`
	LongPrimitive int64  `esper:"longPrimitive"`
	LongBoxed     int64  `esper:"longBoxed"`
}

type infraNWTMarket struct {
	Symbol string `esper:"symbol"`
	Volume int64  `esper:"volume"`
}

type infraNWTMapBean struct {
	Key       string `esper:"key"`
	Primitive int64  `esper:"primitive"`
	Boxed     int64  `esper:"boxed"`
}

type infraNWTBase struct {
	ID string `esper:"id"`
}

type infraNWTBeanA struct {
	infraNWTBase
}

type infraNWTBeanB struct {
	infraNWTBase
}

type infraNWTSchemaOne struct {
	Col1 int `esper:"col1"`
	Col2 int `esper:"col2"`
}

type infraNWTSchemaWindow struct {
	S1 infraNWTSchemaOne `esper:"s1"`
}

type infraNWTInnerOne struct {
	I1 int `esper:"i1"`
}

type infraNWTInnerTwo struct {
	I2 int `esper:"i2"`
}

type infraNWTOuter struct {
	One infraNWTInnerOne `esper:"one"`
	Two infraNWTInnerTwo `esper:"two"`
}

func newInfraNWTEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[infraNWTSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraNWTMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "MyMapWithKeyPrimitiveBoxed", []FieldSpec{
		FieldDef("key", reflect.TypeOf("")),
		FieldDef("primitive", reflect.TypeOf(int64(0))),
		FieldDef("boxed", reflect.TypeOf(int64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	return env, engine
}

func deployInfraNWTPlans(t *testing.T, engine *Engine, plans []Plan) *Deployment {
	t.Helper()
	deployment, err := engine.DeployPlans(context.Background(), plans)
	if err != nil {
		t.Fatal(err)
	}
	return deployment
}

func subscribeInfraNWTStatement(t *testing.T, statement *Statement) (*[]Result, *[]ResultBatch) {
	t.Helper()
	results := new([]Result)
	batches := new([]ResultBatch)
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		*results = append(*results, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return results, batches
}

func infraNWTStatementByDeploymentName(t *testing.T, deployment *Deployment, name string) *Statement {
	t.Helper()
	for _, statement := range deployment.Statements() {
		if statement.Name() == name {
			return statement
		}
	}
	t.Fatalf("statement %q not found", name)
	return nil
}

// TestInfraNWTMapTransposeParity covers InfraMapTranspose across Map and
// ObjectArray representations: a window projected from nested fragments keeps
// the fragment properties readable.
func TestInfraNWTMapTransposeParity(t *testing.T) {
	for _, kind := range []SchemaKind{SchemaMap, SchemaObjectArray} {
		t.Run(fmt.Sprintf("rep-%d", kind), func(t *testing.T) {
			env, engine := newInfraNWTEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()
			innerOne, err := NewSchema("T1", FieldDef("i1", reflect.TypeOf(0)))
			if err != nil {
				t.Fatal(err)
			}
			innerTwo, err := NewSchema("T2", FieldDef("i2", reflect.TypeOf(0)))
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case SchemaMap:
				_, err = RegisterMap(env, "OuterType", []FieldSpec{
					FieldDef("one", reflect.TypeOf(map[string]any{})),
					FieldDef("two", reflect.TypeOf(map[string]any{})),
				}, WithNestedPropertySchema("one", innerOne), WithNestedPropertySchema("two", innerTwo))
			case SchemaObjectArray:
				_, err = RegisterObjectArray(env, "OuterType", []FieldSpec{
					FieldDef("one", reflect.TypeOf(map[string]any{})),
					FieldDef("two", reflect.TypeOf(map[string]any{})),
				}, WithNestedPropertySchema("one", innerOne), WithNestedPropertySchema("two", innerTwo))
			}
			if err != nil {
				t.Fatal(err)
			}
			windowSchema, err := NewSchema("MyWindowMT", FieldDef("one", reflect.TypeOf(map[string]any{})), FieldDef("two", reflect.TypeOf(map[string]any{})))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := CreateNamedWindow(env, "MyWindowMT", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
				t.Fatal(err)
			}
			insertPlan, err := env.Build(OnRecord(FromAny(env, "OuterType")).InsertIntoNamedWindow(
				"MyWindowMT",
				SetColumn("one", Field[any, any]("one")),
				SetColumn("two", Field[any, any]("two")),
			).Query(StatementName("insert")))
			if err != nil {
				t.Fatal(err)
			}
			consumerPlan, err := env.Build(FromNamedWindow(env, "MyWindowMT").Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			deployment := deployInfraNWTPlans(t, engine, []Plan{insertPlan, consumerPlan})
			results, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "s0"))
			var sendErr error
			if kind == SchemaMap {
				sendErr = engine.Send(context.Background(), "OuterType", map[string]any{
					"one": map[string]any{"i1": 1},
					"two": map[string]any{"i2": 2},
				})
			} else {
				sendErr = engine.SendObjectArray(context.Background(), "OuterType", []any{
					map[string]any{"i1": 1},
					map[string]any{"i2": 2},
				})
			}
			if sendErr != nil {
				t.Fatal(sendErr)
			}
			if len(*results) != 1 {
				t.Fatalf("results = %#v", *results)
			}
			if (*results)[0].Get("one.i1").Any() != 1 || (*results)[0].Get("two.i2").Any() != 2 {
				t.Fatalf("fragment props = %#v", (*results)[0].Get("one.i1"))
			}
		})
	}
}

// TestInfraNWTNoWildcardWithAsParity covers InfraNoWildcardWithAs: explicit
// renamed window columns fed by three source types and an on-delete trigger.
func TestInfraNWTNoWildcardWithAsParity(t *testing.T) {
	env, engine := newInfraNWTEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	windowSchema, err := NewSchema("MyWindowNW",
		FieldDef("a", reflect.TypeOf("")),
		FieldDef("b", reflect.TypeOf(int64(0))),
		FieldDef("c", reflect.TypeOf(int64(0))),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowNW", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	beanInsert, err := env.Build(OnEvent(From[infraNWTSupportBean](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindowNW",
		SetColumn("a", Field[infraNWTSupportBean, string]("theString")),
		SetColumn("b", Field[infraNWTSupportBean, int64]("longPrimitive")),
		SetColumn("c", Field[infraNWTSupportBean, int64]("longBoxed")),
	).Query(StatementName("insert-bean")))
	if err != nil {
		t.Fatal(err)
	}
	marketInsert, err := env.Build(OnEvent(From[infraNWTMarket](env, "SupportMarketDataBean")).InsertIntoNamedWindow(
		"MyWindowNW",
		SetColumn("a", Field[infraNWTMarket, string]("symbol")),
		SetColumn("b", Field[infraNWTMarket, int64]("volume")),
		SetColumn("c", Field[infraNWTMarket, int64]("volume")),
	).Query(StatementName("insert-market")))
	if err != nil {
		t.Fatal(err)
	}
	mapInsert, err := env.Build(OnRecord(FromAny(env, "MyMapWithKeyPrimitiveBoxed")).InsertIntoNamedWindow(
		"MyWindowNW",
		SetColumn("a", Field[any, string]("key")),
		SetColumn("b", Field[any, int64]("boxed")),
		SetColumn("c", Field[any, int64]("primitive")),
	).Query(StatementName("insert-map")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromNamedWindow(env, "MyWindowNW").Query(StatementName("s1")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[infraNWTMarket](env, "SupportMarketDataBean")).DeleteFromNamedWindow(
		"MyWindowNW",
		Equal[string](NamedWindowField[string]("a"), Field[infraNWTMarket, string]("symbol")),
	).Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}
	deployment := deployInfraNWTPlans(t, engine, []Plan{beanInsert, marketInsert, mapInsert, consumerPlan, deletePlan})
	beanResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "insert-bean"))
	marketResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "insert-market"))
	mapResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "insert-map"))
	selectResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "s1"))
	sendBean := func(theString string, longPrimitive, longBoxed int64) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean", infraNWTSupportBean{TheString: theString, LongPrimitive: longPrimitive, LongBoxed: longBoxed}); err != nil {
			t.Fatal(err)
		}
	}
	sendMarket := func(symbol string, volume int64) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportMarketDataBean", infraNWTMarket{Symbol: symbol, Volume: volume}); err != nil {
			t.Fatal(err)
		}
	}
	sendMap := func(key string, primitive, boxed int64) {
		t.Helper()
		if err := engine.Send(context.Background(), "MyMapWithKeyPrimitiveBoxed", map[string]any{"key": key, "primitive": primitive, "boxed": boxed}); err != nil {
			t.Fatal(err)
		}
	}
	sendBean("E1", 1, 10)
	sendMarket("S1", 99)
	sendMap("M1", 100, 101)
	want := [][]any{{"E1", int64(1), int64(10)}, {"S1", int64(99), int64(99)}, {"M1", int64(101), int64(100)}}
	allCreate := make([]Result, 0, 3)
	allCreate = append(allCreate, (*beanResults)...)
	allCreate = append(allCreate, (*marketResults)...)
	allCreate = append(allCreate, (*mapResults)...)
	if len(allCreate) != 3 || len(*selectResults) != 3 {
		t.Fatalf("create/select results = %d/%d", len(allCreate), len(*selectResults))
	}
	for index, row := range want {
		createRow := allCreate[index]
		selectRow := (*selectResults)[index]
		got := []any{createRow.Get("a").Any(), createRow.Get("b").Any(), createRow.Get("c").Any()}
		if !reflect.DeepEqual(got, row) {
			t.Fatalf("create row %d = %v, want %v", index, got, row)
		}
		if !reflect.DeepEqual([]any{selectRow.Get("a").Any(), selectRow.Get("b").Any(), selectRow.Get("c").Any()}, row) {
			t.Fatalf("select row %d = %v", index, []any{selectRow.Get("a").Any(), selectRow.Get("b").Any(), selectRow.Get("c").Any()})
		}
	}
}

// TestInfraNWTNoWildcardNoAsParity covers InfraNoWildcardNoAs.
func TestInfraNWTNoWildcardNoAsParity(t *testing.T) {
	env, engine := newInfraNWTEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	windowSchema, err := NewSchema("MyWindowNWNA",
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("longPrimitive", reflect.TypeOf(int64(0))),
		FieldDef("longBoxed", reflect.TypeOf(int64(0))),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowNWNA", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	beanInsert, err := env.Build(OnEvent(From[infraNWTSupportBean](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindowNWNA",
		SetColumn("theString", Field[infraNWTSupportBean, string]("theString")),
		SetColumn("longPrimitive", Field[infraNWTSupportBean, int64]("longPrimitive")),
		SetColumn("longBoxed", Field[infraNWTSupportBean, int64]("longBoxed")),
	).Query(StatementName("insert-bean")))
	if err != nil {
		t.Fatal(err)
	}
	marketInsert, err := env.Build(OnEvent(From[infraNWTMarket](env, "SupportMarketDataBean")).InsertIntoNamedWindow(
		"MyWindowNWNA",
		SetColumn("theString", Field[infraNWTMarket, string]("symbol")),
		SetColumn("longPrimitive", Field[infraNWTMarket, int64]("volume")),
		SetColumn("longBoxed", Field[infraNWTMarket, int64]("volume")),
	).Query(StatementName("insert-market")))
	if err != nil {
		t.Fatal(err)
	}
	mapInsert, err := env.Build(OnRecord(FromAny(env, "MyMapWithKeyPrimitiveBoxed")).InsertIntoNamedWindow(
		"MyWindowNWNA",
		SetColumn("theString", Field[any, string]("key")),
		SetColumn("longPrimitive", Field[any, int64]("boxed")),
		SetColumn("longBoxed", Field[any, int64]("primitive")),
	).Query(StatementName("insert-map")))
	if err != nil {
		t.Fatal(err)
	}
	selectPlan, err := env.Build(FromNamedWindow(env, "MyWindowNWNA").Query(StatementName("select")))
	if err != nil {
		t.Fatal(err)
	}
	deployment := deployInfraNWTPlans(t, engine, []Plan{beanInsert, marketInsert, mapInsert, selectPlan})
	beanResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "insert-bean"))
	marketResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "insert-market"))
	mapResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "insert-map"))
	selectResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "select"))
	sendBean := func(theString string, longPrimitive, longBoxed int64) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean", infraNWTSupportBean{TheString: theString, LongPrimitive: longPrimitive, LongBoxed: longBoxed}); err != nil {
			t.Fatal(err)
		}
	}
	sendMarket := func(symbol string, volume int64) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportMarketDataBean", infraNWTMarket{Symbol: symbol, Volume: volume}); err != nil {
			t.Fatal(err)
		}
	}
	sendMap := func(key string, primitive, boxed int64) {
		t.Helper()
		if err := engine.Send(context.Background(), "MyMapWithKeyPrimitiveBoxed", map[string]any{"key": key, "primitive": primitive, "boxed": boxed}); err != nil {
			t.Fatal(err)
		}
	}
	sendBean("E1", 1, 10)
	sendMarket("S1", 99)
	sendMap("M1", 100, 101)
	want := [][]any{{"E1", int64(1), int64(10)}, {"S1", int64(99), int64(99)}, {"M1", int64(101), int64(100)}}
	allCreate := make([]Result, 0, 3)
	allCreate = append(allCreate, (*beanResults)...)
	allCreate = append(allCreate, (*marketResults)...)
	allCreate = append(allCreate, (*mapResults)...)
	if len(allCreate) != 3 || len(*selectResults) != 3 {
		t.Fatalf("create/select results = %d/%d", len(allCreate), len(*selectResults))
	}
	for index, row := range want {
		createRow := allCreate[index]
		got := []any{createRow.Get("theString").Any(), createRow.Get("longPrimitive").Any(), createRow.Get("longBoxed").Any()}
		if !reflect.DeepEqual(got, row) {
			t.Fatalf("create row %d = %v, want %v", index, got, row)
		}
		selectRow := (*selectResults)[index]
		if !reflect.DeepEqual([]any{selectRow.Get("theString").Any(), selectRow.Get("longPrimitive").Any(), selectRow.Get("longBoxed").Any()}, row) {
			t.Fatalf("select row %d = %v", index, []any{selectRow.Get("theString").Any(), selectRow.Get("longPrimitive").Any(), selectRow.Get("longBoxed").Any()})
		}
	}
}

// TestInfraNWTConstantsAsParity covers InfraConstantsAs: window schema
// declared from constants and fed by two source types.
func TestInfraNWTConstantsAsParity(t *testing.T) {
	env, engine := newInfraNWTEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	windowSchema, err := NewSchema("MyWindowCA",
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("longPrimitive", reflect.TypeOf(int64(0))),
		FieldDef("longBoxed", reflect.TypeOf(int64(0))),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowCA", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	beanInsert, err := env.Build(OnEvent(From[infraNWTSupportBean](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindowCA",
		SetColumn("theString", Field[infraNWTSupportBean, string]("theString")),
		SetColumn("longPrimitive", Field[infraNWTSupportBean, int64]("longPrimitive")),
		SetColumn("longBoxed", Field[infraNWTSupportBean, int64]("longBoxed")),
	).Query(StatementName("insert-bean")))
	if err != nil {
		t.Fatal(err)
	}
	marketInsert, err := env.Build(OnEvent(From[infraNWTMarket](env, "SupportMarketDataBean")).InsertIntoNamedWindow(
		"MyWindowCA",
		SetColumn("theString", Field[infraNWTMarket, string]("symbol")),
		SetColumn("longPrimitive", Field[infraNWTMarket, int64]("volume")),
		SetColumn("longBoxed", Field[infraNWTMarket, int64]("volume")),
	).Query(StatementName("insert-market")))
	if err != nil {
		t.Fatal(err)
	}
	selectPlan, err := env.Build(FromNamedWindow(env, "MyWindowCA").Query(StatementName("select")))
	if err != nil {
		t.Fatal(err)
	}
	deployment := deployInfraNWTPlans(t, engine, []Plan{beanInsert, marketInsert, selectPlan})
	beanResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "insert-bean"))
	marketResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "insert-market"))
	selectResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "select"))
	if err := engine.Send(context.Background(), "SupportBean", infraNWTSupportBean{TheString: "E1", LongPrimitive: 1, LongBoxed: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SupportMarketDataBean", infraNWTMarket{Symbol: "S1", Volume: 99}); err != nil {
		t.Fatal(err)
	}
	if len(*beanResults) != 1 || len(*marketResults) != 1 || len(*selectResults) != 2 {
		t.Fatalf("create/select results = %d/%d/%d", len(*beanResults), len(*marketResults), len(*selectResults))
	}
	row := (*beanResults)[0]
	if row.Get("theString").Any() != "E1" || row.Get("longPrimitive").Any() != int64(1) || row.Get("longBoxed").Any() != int64(10) {
		t.Fatalf("bean row = %v", []any{row.Get("theString").Any(), row.Get("longPrimitive").Any(), row.Get("longBoxed").Any()})
	}
	row = (*marketResults)[0]
	if row.Get("theString").Any() != "S1" || row.Get("longPrimitive").Any() != int64(99) || row.Get("longBoxed").Any() != int64(99) {
		t.Fatalf("market row = %v", []any{row.Get("theString").Any(), row.Get("longPrimitive").Any(), row.Get("longBoxed").Any()})
	}
}

// TestInfraNWTCreateTableSyntaxParity covers InfraCreateTableSyntax: column
// list window declarations, casts, composite retention shapes and a
// field-list schema.
func TestInfraNWTCreateTableSyntaxParity(t *testing.T) {
	env, engine := newInfraNWTEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	windowSchema, err := NewSchema("MyWindowCTS",
		FieldDef("stringValOne", reflect.TypeOf("")),
		FieldDef("stringValTwo", reflect.TypeOf("")),
		FieldDef("intVal", reflect.TypeOf(0)),
		FieldDef("longVal", reflect.TypeOf(int64(0))),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowCTS", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnEvent(From[infraNWTSupportBean](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindowCTS",
		SetColumn("stringValOne", Field[infraNWTSupportBean, string]("theString")),
		SetColumn("stringValTwo", Field[infraNWTSupportBean, string]("theString")),
		SetColumn("intVal", Cast[int64, int](Field[infraNWTSupportBean, int64]("longPrimitive"))),
		SetColumn("longVal", Field[infraNWTSupportBean, int64]("longBoxed")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	selectPlan, err := env.Build(FromNamedWindow(env, "MyWindowCTS").Query(StatementName("select")))
	if err != nil {
		t.Fatal(err)
	}
	deployment := deployInfraNWTPlans(t, engine, []Plan{insertPlan, selectPlan})
	createResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "insert"))
	selectResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "select"))
	if err := engine.Send(context.Background(), "SupportBean", infraNWTSupportBean{TheString: "E1", LongPrimitive: 1, LongBoxed: 10}); err != nil {
		t.Fatal(err)
	}
	for _, results := range [][]Result{*createResults, *selectResults} {
		if len(results) != 1 {
			t.Fatalf("results = %#v", results)
		}
		row := results[0]
		if row.Get("stringValOne").Any() != "E1" || row.Get("stringValTwo").Any() != "E1" ||
			row.Get("intVal").Any() != 1 || row.Get("longVal").Any() != int64(10) {
			t.Fatalf("row = %v", []any{row.Get("stringValOne").Any(), row.Get("stringValTwo").Any(), row.Get("intVal").Any(), row.Get("longVal").Any()})
		}
	}

	// composite retention shapes from the same execution.
	if _, err := CreateNamedWindow(env, "MyWindowCTSTwo", windowSchema, NamedWindowRetention(
		IntersectWindows(Unique(Field[any, string]("stringValOne")), KeepAll()),
	)); err != nil {
		t.Fatal(err)
	}
	fieldListSchema, err := NewSchema("MyWindowCTSThree",
		FieldDef("a", reflect.TypeOf("")),
		FieldDef("b", reflect.TypeOf(0)),
		FieldDef("c", reflect.TypeOf(0)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowCTSThree", fieldListSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowCTSFour", fieldListSchema, NamedWindowRetention(
		UnionWindows(Unique(Field[any, string]("a")), Unique(Field[any, int]("b"))),
	)); err != nil {
		t.Fatal(err)
	}
}

// TestInfraNWTWildcardShapesParity covers InfraWildcardNoFieldsNoAs,
// InfraNoSpecificationBean, InfraWildcardWithFields and InfraWildcardInheritance.
func TestInfraNWTWildcardShapesParity(t *testing.T) {
	t.Run("wildcard-no-fields", func(t *testing.T) {
		env := NewEnvironment()
		aSchema, err := RegisterStruct[infraNWTBeanA](env, "SupportBean_A")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNamedWindow(env, "MyWindowWNF", aSchema, NamedWindowRetention(KeepAll())); err != nil {
			t.Fatal(err)
		}
		insertPlan, err := env.Build(OnEvent(From[infraNWTBeanA](env, "SupportBean_A")).InsertIntoNamedWindow(
			"MyWindowWNF",
			SetColumn("id", Field[infraNWTBeanA, string]("id")),
		).Query(StatementName("insert")))
		if err != nil {
			t.Fatal(err)
		}
		selectPlan, err := env.Build(FromNamedWindow(env, "MyWindowWNF").Query(StatementName("select")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		deployment := deployInfraNWTPlans(t, engine, []Plan{insertPlan, selectPlan})
		createResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "insert"))
		selectResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "select"))
		if err := engine.Send(context.Background(), "SupportBean_A", infraNWTBeanA{infraNWTBase: infraNWTBase{ID: "E1"}}); err != nil {
			t.Fatal(err)
		}
		for _, results := range [][]Result{*createResults, *selectResults} {
			if len(results) != 1 || results[0].Get("id").Any() != "E1" {
				t.Fatalf("wildcard results = %#v", results)
			}
		}
	})

	t.Run("no-specification-bean", func(t *testing.T) {
		env := NewEnvironment()
		aSchema, err := RegisterStruct[infraNWTBeanA](env, "SupportBean_A")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNamedWindow(env, "MyWindowNSB", aSchema, NamedWindowRetention(KeepAll())); err != nil {
			t.Fatal(err)
		}
		insertPlan, err := env.Build(OnEvent(From[infraNWTBeanA](env, "SupportBean_A")).InsertIntoNamedWindow(
			"MyWindowNSB",
			SetColumn("id", Field[infraNWTBeanA, string]("id")),
		).Query(StatementName("insert")))
		if err != nil {
			t.Fatal(err)
		}
		selectPlan, err := env.Build(FromNamedWindow(env, "MyWindowNSB").Query(StatementName("select")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		deployment := deployInfraNWTPlans(t, engine, []Plan{insertPlan, selectPlan})
		results, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "select"))
		if err := engine.Send(context.Background(), "SupportBean_A", infraNWTBeanA{infraNWTBase: infraNWTBase{ID: "E1"}}); err != nil {
			t.Fatal(err)
		}
		if len(*results) != 1 || (*results)[0].Get("id").Any() != "E1" {
			t.Fatalf("bean results = %#v", *results)
		}
	})

	t.Run("wildcard-with-fields", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[infraNWTBeanA](env, "SupportBean_A"); err != nil {
			t.Fatal(err)
		}
		windowSchema, err := NewSchema("MyWindowWWF",
			FieldDef("id", reflect.TypeOf("")),
			FieldDef("myid", reflect.TypeOf("")),
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNamedWindow(env, "MyWindowWWF", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
			t.Fatal(err)
		}
		id := Field[infraNWTBeanA, string]("id")
		insertPlan, err := env.Build(OnEvent(From[infraNWTBeanA](env, "SupportBean_A")).InsertIntoNamedWindow(
			"MyWindowWWF",
			SetColumn("id", id),
			SetColumn("myid", Concat(id, Literal("A"))),
		).Query(StatementName("insert")))
		if err != nil {
			t.Fatal(err)
		}
		selectPlan, err := env.Build(FromNamedWindow(env, "MyWindowWWF").Query(StatementName("select")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		deployment := deployInfraNWTPlans(t, engine, []Plan{insertPlan, selectPlan})
		createResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "insert"))
		selectResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "select"))
		if err := engine.Send(context.Background(), "SupportBean_A", infraNWTBeanA{infraNWTBase: infraNWTBase{ID: "E1"}}); err != nil {
			t.Fatal(err)
		}
		for _, results := range [][]Result{*createResults, *selectResults} {
			if len(results) != 1 || results[0].Get("id").Any() != "E1" || results[0].Get("myid").Any() != "E1A" {
				t.Fatalf("wildcard-with-fields results = %#v", results)
			}
		}
	})

	t.Run("wildcard-inheritance", func(t *testing.T) {
		env := NewEnvironment()
		baseSchema, err := RegisterStruct[infraNWTBase](env, "SupportBeanAtoFBase")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[infraNWTBeanA](env, "SupportBean_A", WithSchemaParent(baseSchema)); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[infraNWTBeanB](env, "SupportBean_B", WithSchemaParent(baseSchema)); err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNamedWindow(env, "MyWindowWI", baseSchema, NamedWindowRetention(KeepAll())); err != nil {
			t.Fatal(err)
		}
		insertA, err := env.Build(OnEvent(From[infraNWTBeanA](env, "SupportBean_A")).InsertIntoNamedWindow(
			"MyWindowWI",
			SetColumn("id", Field[infraNWTBeanA, string]("id")),
		).Query(StatementName("insert-a")))
		if err != nil {
			t.Fatal(err)
		}
		insertB, err := env.Build(OnEvent(From[infraNWTBeanB](env, "SupportBean_B")).InsertIntoNamedWindow(
			"MyWindowWI",
			SetColumn("id", Field[infraNWTBeanB, string]("id")),
		).Query(StatementName("insert-b")))
		if err != nil {
			t.Fatal(err)
		}
		selectPlan, err := env.Build(FromNamedWindow(env, "MyWindowWI").Query(StatementName("select")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		deployment := deployInfraNWTPlans(t, engine, []Plan{insertA, insertB, selectPlan})
		createResultsA, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "insert-a"))
		createResultsB, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "insert-b"))
		selectResults, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "select"))
		if err := engine.Send(context.Background(), "SupportBean_A", infraNWTBeanA{infraNWTBase: infraNWTBase{ID: "E1"}}); err != nil {
			t.Fatal(err)
		}
		if err := engine.Send(context.Background(), "SupportBean_B", infraNWTBeanB{infraNWTBase: infraNWTBase{ID: "E2"}}); err != nil {
			t.Fatal(err)
		}
		if len(*createResultsA) != 1 || len(*createResultsB) != 1 || len(*selectResults) != 2 {
			t.Fatalf("inheritance results = %d/%d/%d", len(*createResultsA), len(*createResultsB), len(*selectResults))
		}
		if (*createResultsA)[0].Get("id").Any() != "E1" || (*createResultsB)[0].Get("id").Any() != "E2" {
			t.Fatalf("create results = %#v / %#v", *createResultsA, *createResultsB)
		}
		if (*selectResults)[0].Get("id").Any() != "E1" || (*selectResults)[1].Get("id").Any() != "E2" {
			t.Fatalf("select results = %#v", *selectResults)
		}
	})
}

// TestInfraNWTModelAfterMapParity covers InfraModelAfterMap: a window declared
// over a Map event type keeps the map underlying.
func TestInfraNWTModelAfterMapParity(t *testing.T) {
	env, engine := newInfraNWTEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	mapSchema, ok := env.Schema("MyMapWithKeyPrimitiveBoxed")
	if !ok {
		t.Fatal("map schema missing")
	}
	if _, err := CreateNamedWindow(env, "MyWindowMAM", mapSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnRecord(FromAny(env, "MyMapWithKeyPrimitiveBoxed")).InsertIntoNamedWindow(
		"MyWindowMAM",
		SetColumn("key", Field[any, string]("key")),
		SetColumn("primitive", Field[any, int64]("primitive")),
		SetColumn("boxed", Field[any, int64]("boxed")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	selectPlan, err := env.Build(FromNamedWindow(env, "MyWindowMAM").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment := deployInfraNWTPlans(t, engine, []Plan{insertPlan, selectPlan})
	results, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "s0"))
	if err := engine.Send(context.Background(), "MyMapWithKeyPrimitiveBoxed", map[string]any{"key": "k1", "primitive": int64(100), "boxed": int64(200)}); err != nil {
		t.Fatal(err)
	}
	if len(*results) != 1 {
		t.Fatalf("results = %#v", *results)
	}
	event, ok := (*results)[0].Event()
	if !ok {
		t.Fatalf("result is not an event: %#v", (*results)[0])
	}
	if _, ok := event.Underlying().(map[string]any); !ok {
		t.Fatalf("underlying = %T, want map", event.Underlying())
	}
	if (*results)[0].Get("key").Any() != "k1" || (*results)[0].Get("primitive").Any() != int64(100) {
		t.Fatalf("row = %v", []any{(*results)[0].Get("key").Any(), (*results)[0].Get("primitive").Any()})
	}
}

// TestInfraNWTCreateTableArrayParity covers InfraCreateTableArray: array
// columns in a named window.
func TestInfraNWTCreateTableArrayParity(t *testing.T) {
	env, engine := newInfraNWTEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	windowSchema, err := NewSchema("MyWindowCTA", FieldDef("myvalue", reflect.TypeOf([]string{})))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowCTA", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnEvent(From[infraNWTSupportBean](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindowCTA",
		SetColumn("myvalue", Literal([]string{"a", "b"})),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("MyWindowCTA")
	if !ok {
		t.Fatal("window missing")
	}
	if err := engine.Send(context.Background(), "SupportBean", infraNWTSupportBean{TheString: "E1", LongPrimitive: 1, LongBoxed: 10}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != 1 {
		t.Fatalf("snapshot = %d", len(snapshot))
	}
	values, ok := snapshot[0].Get("myvalue").Any().([]string)
	if !ok || !reflect.DeepEqual(values, []string{"a", "b"}) {
		t.Fatalf("myvalue = %#v", snapshot[0].Get("myvalue"))
	}
}

// TestInfraNWTEventTypeColumnDefParity covers the InfraEventTypeColumnDef
// matrix: a window with a nested schema column receives the source event as
// the nested column across Map/ObjectArray/JSON/JSON-provided/Avro and the
// Go DEFAULT equivalent (map).
func TestInfraNWTEventTypeColumnDefParity(t *testing.T) {
	representations := []struct {
		name     string
		kind     SchemaKind
		provided bool
	}{
		{name: "map", kind: SchemaMap},
		{name: "objectarray", kind: SchemaObjectArray},
		{name: "json", kind: SchemaJSON},
		{name: "json-provided", kind: SchemaJSON, provided: true},
		{name: "avro", kind: SchemaAvro},
		{name: "default", kind: SchemaMap},
	}
	oneFields := []FieldSpec{
		FieldDef("col1", reflect.TypeOf(0)),
		FieldDef("col2", reflect.TypeOf(0)),
	}
	windowFields := []FieldSpec{
		FieldDef("s1", reflect.TypeOf(map[string]any{})),
	}
	for _, representation := range representations {
		t.Run(representation.name, func(t *testing.T) {
			env := NewEnvironment()
			var oneSchema Schema
			var err error
			switch representation.kind {
			case SchemaMap:
				oneSchema, err = RegisterMap(env, "SchemaOne", oneFields)
			case SchemaObjectArray:
				oneSchema, err = RegisterObjectArray(env, "SchemaOne", oneFields)
			case SchemaJSON:
				if representation.provided {
					oneSchema, err = RegisterJSONFor[infraNWTSchemaOne](env, "SchemaOne", nil)
				} else {
					oneSchema, err = RegisterJSON(env, "SchemaOne", oneFields)
				}
			case SchemaAvro:
				oneSchema, err = RegisterAvro(env, "SchemaOne", oneFields)
			default:
				oneSchema, err = RegisterMap(env, "SchemaOne", oneFields)
			}
			if err != nil {
				t.Fatal(err)
			}
			var windowSchema Schema
			switch representation.kind {
			case SchemaMap:
				windowSchema, err = RegisterMap(env, "SchemaWindow", windowFields, WithNestedPropertySchema("s1", oneSchema))
			case SchemaObjectArray:
				windowSchema, err = RegisterObjectArray(env, "SchemaWindow", windowFields, WithNestedPropertySchema("s1", oneSchema))
			case SchemaJSON:
				if representation.provided {
					windowSchema, err = RegisterJSONFor[infraNWTSchemaWindow](env, "SchemaWindow", nil)
				} else {
					windowSchema, err = RegisterJSON(env, "SchemaWindow", windowFields, WithNestedPropertySchema("s1", oneSchema))
				}
			case SchemaAvro:
				windowSchema, err = RegisterAvro(env, "SchemaWindow", windowFields, WithNestedPropertySchema("s1", oneSchema))
			default:
				windowSchema, err = RegisterMap(env, "SchemaWindow", windowFields, WithNestedPropertySchema("s1", oneSchema))
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := CreateNamedWindow(env, "SchemaWindow", windowSchema, NamedWindowRetention(LastEvent())); err != nil {
				t.Fatal(err)
			}
			var nested Expr
			if representation.provided {
				nested = EventValue[infraNWTSchemaOne]()
			} else {
				nested = StructOf(
					Alias("col1", Field[any, int]("col1")),
					Alias("col2", Field[any, int]("col2")),
				)
			}
			insertPlan, err := env.Build(OnRecord(FromAny(env, "SchemaOne")).InsertIntoNamedWindow(
				"SchemaWindow",
				SetColumn("s1", nested),
			).Query(StatementName("insert")))
			if err != nil {
				t.Fatal(err)
			}
			consumerPlan, err := env.Build(FromNamedWindow(env, "SchemaWindow").Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			defer func() { _ = engine.Close(context.Background()) }()
			deployment := deployInfraNWTPlans(t, engine, []Plan{insertPlan, consumerPlan})
			results, _ := subscribeInfraNWTStatement(t, infraNWTStatementByDeploymentName(t, deployment, "s0"))
			var sendErr error
			switch representation.kind {
			case SchemaMap:
				sendErr = engine.Send(context.Background(), "SchemaOne", map[string]any{"col1": 10, "col2": 11})
			case SchemaObjectArray:
				sendErr = engine.SendObjectArray(context.Background(), "SchemaOne", []any{10, 11})
			case SchemaJSON:
				sendErr = engine.SendJSON(context.Background(), "SchemaOne", []byte(`{"col1":10,"col2":11}`))
			case SchemaAvro:
				record, recordErr := NewAvroRecordFromMap(oneSchema, map[string]any{"col1": 10, "col2": 11})
				if recordErr != nil {
					t.Fatal(recordErr)
				}
				sendErr = engine.SendAvro(context.Background(), "SchemaOne", record)
			default:
				sendErr = engine.Send(context.Background(), "SchemaOne", map[string]any{"col1": 10, "col2": 11})
			}
			if sendErr != nil {
				t.Fatal(sendErr)
			}
			if len(*results) != 1 {
				t.Fatalf("results = %#v", *results)
			}
			row := (*results)[0]
			if row.Get("s1.col1").Any() != 10 || row.Get("s1.col2").Any() != 11 {
				t.Fatalf("nested row = %v", []any{row.Get("s1.col1").Any(), row.Get("s1.col2").Any()})
			}
			event, ok := row.Event()
			if !ok {
				t.Fatalf("row is not an event: %#v", row)
			}
			switch representation.kind {
			case SchemaObjectArray:
				if _, ok := event.Underlying().([]any); !ok {
					t.Fatalf("underlying = %T, want []any", event.Underlying())
				}
			case SchemaAvro:
				if _, ok := event.Underlying().(*AvroRecord); !ok {
					t.Fatalf("underlying = %T, want *AvroRecord", event.Underlying())
				}
			}
		})
	}
}

// TestInfraNWTCreateSchemaModelAfterParity covers InfraCreateSchemaModelAfter:
// a window declared as a nested schema receives the source event as its
// nested column and supports a unique key over the nested property.
func TestInfraNWTCreateSchemaModelAfterParity(t *testing.T) {
	env := NewEnvironment()
	eventTypeOne, err := RegisterMap(env, "EventTypeOne", []FieldSpec{FieldDef("hsi", reflect.TypeOf(0))})
	if err != nil {
		t.Fatal(err)
	}
	eventTypeTwo, err := RegisterMap(env, "EventTypeTwo", []FieldSpec{
		FieldDef("event", reflect.TypeOf(map[string]any{})),
	}, WithNestedPropertySchema("event", eventTypeOne))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "NamedWindow", eventTypeTwo, NamedWindowRetention(
		Unique(Property[int](Field[any, Event]("event"), "hsi")),
	)); err != nil {
		t.Fatal(err)
	}
	nested := StructOf(Alias("hsi", Field[any, int]("hsi")))
	insertPlan, err := env.Build(OnRecord(FromAny(env, "EventTypeOne")).InsertIntoNamedWindow(
		"NamedWindow",
		SetColumn("event", nested),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("NamedWindow")
	if !ok {
		t.Fatal("window missing")
	}
	if err := engine.Send(context.Background(), "EventTypeOne", map[string]any{"hsi": 10}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != 1 {
		t.Fatalf("snapshot = %d", len(snapshot))
	}
	if snapshot[0].Get("event.hsi").Any() != 10 {
		t.Fatalf("nested hsi = %#v", snapshot[0].Get("event.hsi"))
	}
}
