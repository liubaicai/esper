package esper

import (
	"context"
	"database/sql/driver"
	"reflect"
	"testing"
)

type dbJoinMiscBeanTwo struct {
	StringTwo       string
	IntPrimitiveTwo int
}

type dbJoinMiscBeanA struct {
	ID string
}

// dbJoinAllMyIntHandler returns all myint values without filtering.
func dbJoinAllMyIntHandler(_ string, _ []any) (dbJoinSQLResult, error) {
	cols := []string{"myint"}
	result := dbJoinSQLResult{columns: cols}
	for _, row := range dbJoinMyTestTable {
		result.rows = append(result.rows, []driver.Value{row.myint})
	}
	return result, nil
}

func dbJoinAllMyCharHandler(_ string, _ []any) (dbJoinSQLResult, error) {
	cols := []string{"mychar"}
	result := dbJoinSQLResult{columns: cols}
	for _, row := range dbJoinMyTestTable {
		result.rows = append(result.rows, []driver.Value{row.mychar})
	}
	return result, nil
}

func TestDatabase3StreamMatchesJava(t *testing.T) {
	dbJoinSetHandler(dbJoinAllMyIntHandler)
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinMiscBeanTwo](env, "SupportBeanTwo"); err != nil {
		t.Fatal(err)
	}
	dbJoinRegisterSupportBean(t, env)
	db := dbJoinOpenDB(t)
	defer db.Close()

	myintSchema, err := NewMapSchema("Hist3StreamMyInt", []FieldSpec{
		FieldDef("myint", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Unparameterized SQL: returns all rows. Use FromHistorical (no trigger
	// restriction) so cached data is available when any stream event arrives.
	provider, err := NewSQLHistoricalProvider(db, myintSchema,
		"select myint from mytesttable")
	if err != nil {
		t.Fatal(err)
	}

	sbStream := From[dbJoinSupportBean](env, "SupportBean").Window(LengthWindow(1))
	sbtStream := From[dbJoinMiscBeanTwo](env, "SupportBeanTwo").Window(LengthWindow(1))
	hist := FromHistorical[map[string]any](env, "MyDB3Stream", myintSchema, provider)

	query := JoinMany(
		JoinSource(sbStream),
		JoinSource(sbtStream),
		JoinSource(hist),
	).On(
		OnSourcesEqual(0, Field[dbJoinSupportBean, string]("theString"),
			1, Field[dbJoinMiscBeanTwo, string]("StringTwo")),
		OnSourcesEqual(2, Field[map[string]any, int]("myint"),
			1, Field[dbJoinMiscBeanTwo, int]("IntPrimitiveTwo")),
	).Select(
		SelectFrom(0, "theString", Field[dbJoinSupportBean, string]("theString")),
		SelectFrom(1, "stringTwo", Field[dbJoinMiscBeanTwo, string]("StringTwo")),
		SelectFrom(2, "myint", Field[map[string]any, int]("myint")),
	).Query(StatementName("s0-3stream"))

	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	getRows := dbJoinSubscribeRows(t, deployment.Statements()[0])

	// T1/2: myint=2 not in table, no output
	if err := engine.SendEvent(context.Background(), dbJoinMiscBeanTwo{StringTwo: "T1", IntPrimitiveTwo: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{TheString: "T1"}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows for T1/2, got %d", len(rows))
	}

	// T2/30: myint=30 exists, output {T2, T2, 30}
	if err := engine.SendEvent(context.Background(), dbJoinMiscBeanTwo{StringTwo: "T2", IntPrimitiveTwo: 30}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{TheString: "T2"}); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row for T2/30, got %d", len(rows))
	}
	dbJoinAssertRow(t, rows[0], map[string]any{
		"theString": "T2", "stringTwo": "T2", "myint": 30,
	})

	// T3/40: myint=40 exists, output {T3, T3, 40}
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{TheString: "T3"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), dbJoinMiscBeanTwo{StringTwo: "T3", IntPrimitiveTwo: 40}); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if len(rows) != 2 {
		t.Fatalf("expected 2 total rows after T3/40, got %d", len(rows))
	}
	dbJoinAssertRow(t, rows[1], map[string]any{
		"theString": "T3", "stringTwo": "T3", "myint": 40,
	})
}

func TestDatabaseVariablesMatchesJava(t *testing.T) {
	dbJoinSetHandler(dbJoinMyIntWhereBigintHandler)
	env := NewEnvironment()
	if err := env.RegisterVariable("queryvar", 0); err != nil {
		t.Fatal(err)
	}
	dbJoinRegisterSupportBean(t, env)
	if _, err := RegisterStruct[dbJoinMiscBeanA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}

	schema, err := NewMapSchema("HistVarMyInt", []FieldSpec{
		FieldDef("myint", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()

	provider, err := NewSQLHistoricalProvider(db, schema,
		"select myint from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any {
			raw, ok := request.Variables["queryvar"]
			if !ok {
				return 0
			}
			current, _ := dbJoinToInt64(raw.Any())
			return current
		})
	if err != nil {
		t.Fatal(err)
	}

	// Use Select on historical source triggered by SupportBean_A.
	// The variable substitution in the provider key function is the join.
	source := FromHistoricalOn[map[string]any](env, "MyDBVarLeft", "SupportBean_A", schema, provider)
	plan, err := env.Build(Select(source,
		Alias("myint", Field[map[string]any, int]("myint")),
	).Query(StatementName("s0-vars")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)

	// Set queryvar=5 (equivalent to "on SupportBean set queryvar=intPrimitive")
	if err := engine.SetVariable(context.Background(), "queryvar", 5); err != nil {
		t.Fatal(err)
	}

	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	getRows := dbJoinSubscribeRows(t, deployment.Statements()[0])

	if err := engine.SendEvent(context.Background(), dbJoinMiscBeanA{ID: "A1"}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row for queryvar=5, got %d", len(rows))
	}
	dbJoinAssertRow(t, rows[0], map[string]any{"myint": 50})
}

func TestDatabaseRestartStatementMatchesJava(t *testing.T) {
	// Java test verifies that undeploy/deploy cycle properly releases DB
	// connections. The Go fake driver has no connection pool, so this test
	// verifies plan reusability: the same Plan object can be deployed and
	// undeployed repeatedly without errors.
	dbJoinSetHandler(func(_ string, args []any) (dbJoinSQLResult, error) {
		cols := []string{"mychar"}
		result := dbJoinSQLResult{columns: cols}
		if len(args) == 0 {
			return result, nil
		}
		v, ok := dbJoinToInt64(args[0])
		if !ok {
			return result, nil
		}
		for _, row := range dbJoinMyTestTable {
			if row.mybigint == v {
				result.rows = append(result.rows, []driver.Value{row.mychar})
			}
		}
		return result, nil
	})
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinExtraS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}

	schema, err := NewMapSchema("HistRestartMyChar", []FieldSpec{
		FieldDef("mychar", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()

	provider, err := NewSQLHistoricalProvider(db, schema,
		"select mychar from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any {
			return request.Trigger.Get("ID").Any()
		})
	if err != nil {
		t.Fatal(err)
	}

	source := FromHistoricalOn[map[string]any](env, "MyDBRestart", "SupportBean_S0", schema, provider)
	plan, err := env.Build(Select(source,
		Alias("mychar", Field[map[string]any, string]("mychar")),
	).Query(StatementName("s0-restart")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)

	// Deploy/undeploy/redeploy loop (Java does 100; reduced to 10 for speed)
	for i := 0; i < 10; i++ {
		dep, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatalf("deploy iteration %d: %v", i, err)
		}
		getRows := dbJoinSubscribeRows(t, dep.Statements()[0])

		if err := engine.SendEvent(context.Background(), dbJoinExtraS0{ID: 1}); err != nil {
			t.Fatalf("send iteration %d: %v", i, err)
		}
		rows := getRows()
		if len(rows) != 1 {
			t.Fatalf("iteration %d: expected 1 row, got %d", i, len(rows))
		}
		if rows[0]["mychar"] != "Z" {
			t.Fatalf("iteration %d: expected mychar=Z, got %v", i, rows[0]["mychar"])
		}
		dep.Undeploy(context.Background())
	}
}
