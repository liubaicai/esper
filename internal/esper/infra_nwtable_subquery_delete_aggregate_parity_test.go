package esper

import (
	"context"
	"reflect"
	"sort"
	"testing"
)

type infraNWTableSubqueryDeleteAggregateBean struct {
	TheString     string `esper:"theString"`
	IntBoxed      int    `esper:"intBoxed"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type infraNWTableSubqueryDeleteAggregateMarket struct {
	Symbol string `esper:"symbol"`
}

func infraNWTableSubqueryDeleteAggregateEnvironment(t *testing.T, namedWindow bool) (*Environment, RecordStream) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[infraNWTableSubqueryDeleteAggregateBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if namedWindow {
		schema, err := NewMapSchema("InfraDeleteAggregateSchema", []FieldSpec{
			FieldDef("key", reflect.TypeOf("")),
			FieldDef("value", reflect.TypeOf(0)),
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := env.RegisterSchema(schema); err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNamedWindow(env, "MyInfra", schema, NamedWindowRetention(KeepAll())); err != nil {
			t.Fatal(err)
		}
		return env, FromNamedWindow(env, "MyInfra")
	}
	if _, err := CreateTable(env, "MyInfra", []TableColumn{
		PrimaryKeyColumn[string]("key"),
		PrimaryKeyColumn[int]("value"),
	}); err != nil {
		t.Fatal(err)
	}
	return env, FromTable(env, "MyInfra")
}

func TestInfraNWTableSubqueryDeleteInsertReplaceParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		name := "table"
		if namedWindow {
			name = "named-window"
		}
		t.Run(name, func(t *testing.T) {
			env, inner := infraNWTableSubqueryDeleteAggregateEnvironment(t, namedWindow)
			source := From[infraNWTableSubqueryDeleteAggregateBean](env, "SupportBean")
			assignments := []TableAssignment{
				SetColumn("key", Field[infraNWTableSubqueryDeleteAggregateBean, string]("theString")),
				SetColumn("value", Field[infraNWTableSubqueryDeleteAggregateBean, int]("intBoxed")),
			}
			var insertPlan Plan
			var err error
			if namedWindow {
				insertPlan, err = env.Build(OnEvent(source).InsertIntoNamedWindow("MyInfra", assignments...).Query(StatementName("insert")))
			} else {
				insertPlan, err = env.Build(OnEvent(source).InsertIntoTable("MyInfra", assignments...).Query(StatementName("insert")))
			}
			if err != nil {
				t.Fatal(err)
			}
			deleteSource := From[infraNWTableSubqueryDeleteAggregateBean](env, "SupportBean")
			var deletePlan Plan
			if namedWindow {
				deletePlan, err = env.Build(OnEvent(deleteSource).DeleteFromNamedWindow("MyInfra",
					Equal[string](NamedWindowField[string]("key"), Field[infraNWTableSubqueryDeleteAggregateBean, string]("theString"))).Query(StatementName("delete")))
			} else {
				deletePlan, err = env.Build(OnEvent(deleteSource).DeleteFromTableWhere("MyInfra",
					Equal[string](TableField[string]("key"), Field[infraNWTableSubqueryDeleteAggregateBean, string]("theString"))).Query(StatementName("delete")))
			}
			if err != nil {
				t.Fatal(err)
			}
			createPlan, err := env.Build(inner.Query(StatementName("create"), WithOldStream()))
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			defer func() { _ = engine.Close(context.Background()) }()
			createDeployment, err := engine.Deploy(context.Background(), createPlan)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Deploy(context.Background(), deletePlan); err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
				t.Fatal(err)
			}
			create, ok := createDeployment.Statement("create")
			if !ok {
				t.Fatal("create statement missing")
			}
			type batch struct {
				newRows []map[string]any
				oldRows []map[string]any
			}
			var batches []batch
			rows := func(results []Result) []map[string]any {
				if len(results) == 0 {
					return nil
				}
				out := make([]map[string]any, 0, len(results))
				for _, result := range results {
					if event, ok := result.Event(); ok {
						out = append(out, map[string]any{"key": event.Get("key").Any(), "value": event.Get("value").Any()})
						continue
					}
					row, ok := result.Row()
					if !ok {
						t.Fatalf("result is neither event nor row: %#v", result)
					}
					out = append(out, map[string]any{"key": row.Get("key").Any(), "value": row.Get("value").Any()})
				}
				return out
			}
			if _, err := create.Subscribe(func(_ context.Context, result ResultBatch) error {
				batches = append(batches, batch{newRows: rows(result.New), oldRows: rows(result.Old)})
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			send := func(key string, value int) {
				t.Helper()
				if err := engine.SendEvent(context.Background(), infraNWTableSubqueryDeleteAggregateBean{TheString: key, IntBoxed: value}); err != nil {
					t.Fatal(err)
				}
			}
			send("E1", 1)
			send("E2", 2)
			send("E1", 3)
			if namedWindow {
				want := []batch{
					{newRows: []map[string]any{{"key": "E1", "value": 1}}},
					{newRows: []map[string]any{{"key": "E2", "value": 2}}},
					{newRows: []map[string]any{{"key": "E1", "value": 3}}, oldRows: []map[string]any{{"key": "E1", "value": 1}}},
				}
				if !reflect.DeepEqual(batches, want) {
					t.Fatalf("named-window batches = %#v, want %#v", batches, want)
				}
			} else if len(batches) != 0 {
				t.Fatalf("table create listener batches = %#v, want none", batches)
			}
			snapshot, err := innerSnapshot(context.Background(), engine, namedWindow, "MyInfra")
			if err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				if snapshot[0]["key"] != "E2" || snapshot[1]["key"] != "E1" {
					t.Fatalf("named-window snapshot = %#v, want E2/E1", snapshot)
				}
			} else {
				sort.Slice(snapshot, func(i, j int) bool { return snapshot[i]["key"].(string) < snapshot[j]["key"].(string) })
				want := []map[string]any{{"key": "E1", "value": 3}, {"key": "E2", "value": 2}}
				if !reflect.DeepEqual(snapshot, want) {
					t.Fatalf("table snapshot = %#v, want %#v", snapshot, want)
				}
			}
		})
	}
}

func innerSnapshot(ctx context.Context, engine *Engine, namedWindow bool, name string) ([]map[string]any, error) {
	if namedWindow {
		window, ok := engine.NamedWindow(name)
		if !ok {
			return nil, NewError(ErrorUnknownName, "named window is missing")
		}
		events, err := window.Snapshot(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]map[string]any, 0, len(events))
		for _, event := range events {
			result = append(result, map[string]any{"key": event.Get("key").Any(), "value": event.Get("value").Any()})
		}
		return result, nil
	}
	table, ok := engine.Table(name)
	if !ok {
		return nil, NewError(ErrorUnknownName, "table is missing")
	}
	rows, err := table.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, map[string]any{"key": row.Get("key").Any(), "value": row.Get("value").Any()})
	}
	return result, nil
}

func TestInfraNWTableSubqueryUncorrelatedAggregationParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		name := "table"
		if namedWindow {
			name = "named-window"
		}
		t.Run(name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[infraNWTableSubqueryDeleteAggregateBean](env, "SupportBean"); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[infraNWTableSubqueryDeleteAggregateMarket](env, "SupportMarketDataBean"); err != nil {
				t.Fatal(err)
			}
			var inner RecordStream
			if namedWindow {
				schema, err := NewMapSchema("InfraAggregateParitySchema", []FieldSpec{
					FieldDef("a", reflect.TypeOf("")),
					FieldDef("b", reflect.TypeOf(int64(0))),
				})
				if err != nil {
					t.Fatal(err)
				}
				if err := env.RegisterSchema(schema); err != nil {
					t.Fatal(err)
				}
				if _, err := CreateNamedWindow(env, "MyInfraUCS", schema, NamedWindowRetention(KeepAll())); err != nil {
					t.Fatal(err)
				}
				inner = FromNamedWindow(env, "MyInfraUCS")
			} else {
				if _, err := CreateTable(env, "MyInfraUCS", []TableColumn{
					PrimaryKeyColumn[string]("a"),
					TableColumnOf[int64]("b"),
				}); err != nil {
					t.Fatal(err)
				}
				inner = FromTable(env, "MyInfraUCS")
			}
			source := From[infraNWTableSubqueryDeleteAggregateBean](env, "SupportBean")
			assignments := []TableAssignment{
				SetColumn("a", Field[infraNWTableSubqueryDeleteAggregateBean, string]("theString")),
				SetColumn("b", Field[infraNWTableSubqueryDeleteAggregateBean, int64]("longPrimitive")),
			}
			var insertPlan Plan
			var err error
			if namedWindow {
				insertPlan, err = env.Build(OnEvent(source).InsertIntoNamedWindow("MyInfraUCS", assignments...).Query(StatementName("insert")))
			} else {
				insertPlan, err = env.Build(OnEvent(source).InsertIntoTable("MyInfraUCS", assignments...).Query(StatementName("insert")))
			}
			if err != nil {
				t.Fatal(err)
			}
			build := func(statementName string) Plan {
				plan, buildErr := env.Build(Select(From[infraNWTableSubqueryDeleteAggregateMarket](env, "SupportMarketDataBean"),
					Alias("value", SubquerySum[int64](inner, Field[any, int64]("b"))),
					Alias("symbol", Field[infraNWTableSubqueryDeleteAggregateMarket, string]("symbol")),
				).Query(StatementName(statementName)))
				if buildErr != nil {
					t.Fatal(buildErr)
				}
				return plan
			}
			selectOnePlan := build("selectOne")
			selectTwoPlan := build("selectTwo")
			engine := NewEngine(env)
			defer func() { _ = engine.Close(context.Background()) }()
			if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
				t.Fatal(err)
			}
			oneDeployment, err := engine.Deploy(context.Background(), selectOnePlan)
			if err != nil {
				t.Fatal(err)
			}
			one, ok := oneDeployment.Statement("selectOne")
			if !ok {
				t.Fatal("selectOne statement missing")
			}
			type observed struct {
				statement string
				value     any
				symbol    string
			}
			var observedRows []observed
			subscribe := func(statement *Statement) {
				t.Helper()
				if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
					for _, result := range batch.New {
						row, ok := result.Row()
						if !ok {
							t.Fatalf("aggregate result is not a row: %#v", result)
						}
						value := row.Get("value")
						var anyValue any
						if !value.IsNull() {
							anyValue = value.Any()
						}
						observedRows = append(observedRows, observed{statement: statement.Name(), value: anyValue, symbol: row.Get("symbol").Any().(string)})
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			subscribe(one)
			sendMarket := func(symbol string) {
				t.Helper()
				if err := engine.SendEvent(context.Background(), infraNWTableSubqueryDeleteAggregateMarket{Symbol: symbol}); err != nil {
					t.Fatal(err)
				}
			}
			sendBean := func(symbol string, value int64) {
				t.Helper()
				if err := engine.SendEvent(context.Background(), infraNWTableSubqueryDeleteAggregateBean{TheString: symbol, LongPrimitive: value}); err != nil {
					t.Fatal(err)
				}
			}
			sendMarket("M1")
			sendBean("S1", 5)
			sendMarket("M2")
			sendBean("S2", 10)
			sendMarket("M3")
			twoDeployment, err := engine.Deploy(context.Background(), selectTwoPlan)
			if err != nil {
				t.Fatal(err)
			}
			two, ok := twoDeployment.Statement("selectTwo")
			if !ok {
				t.Fatal("selectTwo statement missing")
			}
			subscribe(two)
			sendBean("S3", 8)
			sendMarket("M4")
			want := []observed{
				{statement: "selectOne", value: nil, symbol: "M1"},
				{statement: "selectOne", value: int64(5), symbol: "M2"},
				{statement: "selectOne", value: int64(15), symbol: "M3"},
				{statement: "selectOne", value: int64(23), symbol: "M4"},
				{statement: "selectTwo", value: int64(23), symbol: "M4"},
			}
			if !reflect.DeepEqual(observedRows, want) {
				t.Fatalf("aggregate rows = %#v, want %#v", observedRows, want)
			}
		})
	}
}
func TestInfraNWTableSubqueryTypedDirectNamedWindowParity(t *testing.T) {
	env, engine := newRuntimeTest(t)
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "TypedDirectWindow", schema, NamedWindowRetention(LastEvent())); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromNamedWindowAs[runtimeTestTrade](env, "TypedDirectWindow").AsRecord().CreateNamedWindowQuery(
		StatementName("typed-direct"),
		WithOldStream(),
	))
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Query().namedWindowDirect {
		t.Fatal("typed direct query lost its direct named-window role")
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "TypedDirectWindow", runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "TypedDirectWindow", runtimeTestTrade{Symbol: "B", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 {
		t.Fatalf("typed direct batches = %d, want 2", len(batches))
	}
	if len(batches[0].New) != 1 || len(batches[0].Old) != 0 || batches[0].New[0].Get("symbol").Any() != "A" {
		t.Fatalf("typed direct first batch = %#v, want new A only", batches[0])
	}
	if len(batches[1].New) != 1 || len(batches[1].Old) != 1 || batches[1].Old[0].Get("symbol").Any() != "A" || batches[1].New[0].Get("symbol").Any() != "B" {
		t.Fatalf("typed direct replacement batch = %#v, want old A/new B", batches[1])
	}
}
