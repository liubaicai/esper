package esper

import (
	"context"
	"reflect"
	"testing"
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

func infraNWTableSubqueryRegister(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[infraNWTableSubqueryBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraNWTableSubqueryS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraNWTableSubqueryA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	return env
}

func infraNWTableSubqueryBuildStore(t *testing.T, env *Environment, namedWindow bool, name string) Plan {
	t.Helper()
	source := From[infraNWTableSubqueryBean](env, "SupportBean")
	assignments := []TableAssignment{
		SetColumn("theString", Field[infraNWTableSubqueryBean, string]("theString")),
		SetColumn("intPrimitive", Field[infraNWTableSubqueryBean, int]("intPrimitive")),
	}
	var plan Plan
	var err error
	if namedWindow {
		schema, ok := env.Schema("SupportBean")
		if !ok {
			t.Fatal("SupportBean schema is missing")
		}
		if _, err := CreateNamedWindow(env, name, schema, NamedWindowRetention(KeepAll())); err != nil {
			t.Fatal(err)
		}
		plan, err = env.Build(OnEvent(source).InsertIntoNamedWindow(name, assignments...).Query(StatementName("insert")))
	} else {
		if _, err := CreateTable(env, name, []TableColumn{
			PrimaryKeyColumn[string]("theString"),
			TableColumnOf[int]("intPrimitive"),
		}); err != nil {
			t.Fatal(err)
		}
		plan, err = env.Build(OnEvent(source).InsertIntoTable(name, assignments...).Query(StatementName("insert")))
	}
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestInfraNWTableSubquerySceneOneParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		name := "table"
		if namedWindow {
			name = "named-window"
		}
		t.Run(name, func(t *testing.T) {
			env := infraNWTableSubqueryRegister(t)
			insertPlan := infraNWTableSubqueryBuildStore(t, env, namedWindow, "MyInfra")
			engine := NewEngine(env)
			if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = engine.Close(context.Background()) }()

			for _, event := range []infraNWTableSubqueryBean{
				{TheString: "A1", IntPrimitive: 1, IntBoxed: 1},
				{TheString: "B2", IntPrimitive: 2, IntBoxed: 2},
				{TheString: "C3", IntPrimitive: 3, IntBoxed: 3},
			} {
				if err := engine.SendEvent(context.Background(), event); err != nil {
					t.Fatal(err)
				}
			}

			var inner RecordStream
			if namedWindow {
				inner = FromNamedWindow(env, "MyInfra")
			} else {
				inner = FromTable(env, "MyInfra")
			}
			query := Select(
				From[infraNWTableSubqueryS0](env, "SupportBean_S0"),
				Alias("c0", SubqueryValue[int](inner, Field[any, int]("intPrimitive"),
					Equal[string](Field[any, string]("theString"), OuterField[string]("p00")))),
			).Query(StatementName("Subq"))
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
						t.Fatalf("scene one result is not a row: %#v", result)
					}
					rows = append(rows, row)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			for _, test := range []struct {
				key  string
				want int
			}{
				{key: "A1", want: 1},
				{key: "B2", want: 2},
			} {
				if err := engine.SendEvent(context.Background(), infraNWTableSubqueryS0{P00: test.key}); err != nil {
					t.Fatal(err)
				}
				if len(rows) == 0 {
					t.Fatalf("scene one query emitted no row for %s", test.key)
				}
				got := rows[len(rows)-1].Get("c0")
				if got.IsNull() || got.Any() != test.want {
					t.Fatalf("scene one %s = %#v, want %d", test.key, got, test.want)
				}
			}
			if len(rows) != 2 {
				t.Fatalf("scene one rows = %#v, want 2", rows)
			}
		})
	}
}

func infraNWTableSubquerySelfState(t *testing.T, engine *Engine, namedWindow bool) (keys []string, values map[string]int) {
	t.Helper()
	values = make(map[string]int)
	if namedWindow {
		window, ok := engine.NamedWindow("MyInfraSSS")
		if !ok {
			t.Fatal("MyInfraSSS named window is missing")
		}
		events, err := window.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range events {
			key, ok := event.Get("key").Any().(string)
			if !ok {
				t.Fatalf("named window key = %#v", event.Get("key").Any())
			}
			value, ok := event.Get("value").Any().(int)
			if !ok {
				t.Fatalf("named window value = %#v", event.Get("value").Any())
			}
			keys = append(keys, key)
			values[key] = value
		}
		return keys, values
	}
	table, ok := engine.Table("MyInfraSSS")
	if !ok {
		t.Fatal("MyInfraSSS table is missing")
	}
	rows, err := table.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		key, ok := row.Get("key").Any().(string)
		if !ok {
			t.Fatalf("table key = %#v", row.Get("key").Any())
		}
		value, ok := row.Get("value").Any().(int)
		if !ok {
			t.Fatalf("table value = %#v", row.Get("value").Any())
		}
		keys = append(keys, key)
		values[key] = value
	}
	return keys, values
}

func TestInfraNWTableSubquerySelfCheckParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		name := "table"
		if namedWindow {
			name = "named-window"
		}
		t.Run(name, func(t *testing.T) {
			env := infraNWTableSubqueryRegister(t)
			storeSource := From[infraNWTableSubqueryBean](env, "SupportBean")
			var inner RecordStream
			if namedWindow {
				selfSchema, err := NewMapSchema("InfraSelfCheckSchema", []FieldSpec{
					FieldDef("key", reflect.TypeOf("")),
					FieldDef("value", reflect.TypeOf(0)),
				})
				if err != nil {
					t.Fatal(err)
				}
				if err := env.RegisterSchema(selfSchema); err != nil {
					t.Fatal(err)
				}
				if _, err := CreateNamedWindow(env, "MyInfraSSS", selfSchema, NamedWindowRetention(KeepAll())); err != nil {
					t.Fatal(err)
				}
				inner = FromNamedWindow(env, "MyInfraSSS")
			} else {
				if _, err := CreateTable(env, "MyInfraSSS", []TableColumn{
					PrimaryKeyColumn[string]("key"),
					TableColumnOf[int]("value"),
				}); err != nil {
					t.Fatal(err)
				}
				inner = FromTable(env, "MyInfraSSS")
			}
			insertSource := storeSource.Filter(Not(SubqueryExists(inner,
				Equal[string](Field[any, string]("key"), OuterField[string]("theString")),
			)))
			assignments := []TableAssignment{
				SetColumn("key", Field[infraNWTableSubqueryBean, string]("theString")),
				SetColumn("value", Field[infraNWTableSubqueryBean, int]("intBoxed")),
			}
			var insertPlan Plan
			var err error
			if namedWindow {
				insertPlan, err = env.Build(OnEvent(insertSource).InsertIntoNamedWindow("MyInfraSSS", assignments...).Query(StatementName("insert")))
			} else {
				insertPlan, err = env.Build(OnEvent(insertSource).InsertIntoTable("MyInfraSSS", assignments...).Query(StatementName("insert")))
			}
			if err != nil {
				t.Fatal(err)
			}
			deleteSource := From[infraNWTableSubqueryA](env, "SupportBean_A")
			var deletePlan Plan
			if namedWindow {
				deletePlan, err = env.Build(OnEvent(deleteSource).DeleteFromNamedWindow("MyInfraSSS", Equal[string](NamedWindowField[string]("key"), Field[infraNWTableSubqueryA, string]("id"))).Query(StatementName("delete")))
			} else {
				deletePlan, err = env.Build(OnEvent(deleteSource).DeleteFromTableWhere("MyInfraSSS", Equal[string](TableField[string]("key"), Field[infraNWTableSubqueryA, string]("id"))).Query(StatementName("delete")))
			}
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Deploy(context.Background(), deletePlan); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = engine.Close(context.Background()) }()

			send := func(key string, value int) {
				t.Helper()
				if err := engine.SendEvent(context.Background(), infraNWTableSubqueryBean{TheString: key, IntBoxed: value}); err != nil {
					t.Fatal(err)
				}
			}
			assertState := func(wantKeys []string, wantValues map[string]int) {
				t.Helper()
				keys, values := infraNWTableSubquerySelfState(t, engine, namedWindow)
				if namedWindow && !reflect.DeepEqual(keys, wantKeys) {
					t.Fatalf("named-window order = %#v, want %#v", keys, wantKeys)
				}
				if !reflect.DeepEqual(values, wantValues) {
					t.Fatalf("self-check state = %#v, want %#v", values, wantValues)
				}
			}

			send("E1", 1)
			assertState([]string{"E1"}, map[string]int{"E1": 1})
			send("E2", 2)
			assertState([]string{"E1", "E2"}, map[string]int{"E1": 1, "E2": 2})
			send("E1", 3)
			assertState([]string{"E1", "E2"}, map[string]int{"E1": 1, "E2": 2})
			send("E3", 4)
			assertState([]string{"E1", "E2", "E3"}, map[string]int{"E1": 1, "E2": 2, "E3": 4})
			if err := engine.SendEvent(context.Background(), infraNWTableSubqueryA{ID: "E2"}); err != nil {
				t.Fatal(err)
			}
			assertState([]string{"E1", "E3"}, map[string]int{"E1": 1, "E3": 4})
			send("E2", 5)
			assertState([]string{"E1", "E3", "E2"}, map[string]int{"E1": 1, "E3": 4, "E2": 5})
		})
	}
}
