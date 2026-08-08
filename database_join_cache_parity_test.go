package esper

import (
	"context"
	"database/sql/driver"
	"reflect"
	"testing"
)

// dbJoinAllMyIntMatchingHandler returns myint values that match the given
// myint parameter. Used for SQL where substitution parameter equals myint.
func dbJoinAllMyIntMatchingHandler(_ string, args []any) (dbJoinSQLResult, error) {
	cols := []string{"myint"}
	result := dbJoinSQLResult{columns: cols}
	if len(args) == 0 {
		for _, row := range dbJoinMyTestTable {
			result.rows = append(result.rows, []driver.Value{row.myint})
		}
		return result, nil
	}
	v, ok := dbJoinToInt64(args[0])
	if !ok {
		return result, nil
	}
	for _, row := range dbJoinMyTestTable {
		if int64(row.myint) == v {
			result.rows = append(result.rows, []driver.Value{row.myint})
		}
	}
	return result, nil
}

// dbJoinMyDoubleWhereMyIntHandler returns mydouble for rows matching myint.
func dbJoinMyDoubleWhereMyIntHandler(_ string, args []any) (dbJoinSQLResult, error) {
	cols := []string{"mydouble"}
	result := dbJoinSQLResult{columns: cols}
	if len(args) == 0 {
		return result, nil
	}
	v, ok := dbJoinToInt64(args[0])
	if !ok {
		return result, nil
	}
	for _, row := range dbJoinMyTestTable {
		if int64(row.myint) == v {
			result.rows = append(result.rows, []driver.Value{row.mydouble})
		}
	}
	return result, nil
}

func TestDatabaseOuterJoinWCacheMatchesJava(t *testing.T) {
	// Java EPLDatabaseOuterJoinWCache: SupportBean LEFT OUTER JOIN
	// sql:...[select myint from mytesttable] ON intPrimitive = myint
	// WHERE myint IS NULL (only output unmatched events)
	dbJoinSetHandler(dbJoinAllMyIntHandler)
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	db := dbJoinOpenDB(t)
	defer db.Close()

	myintSchema, err := NewMapSchema("HistCacheMyInt", []FieldSpec{
		FieldDef("myint", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}

	provider, err := NewSQLHistoricalProvider(db, myintSchema,
		"select myint from mytesttable")
	if err != nil {
		t.Fatal(err)
	}

	sbStream := From[dbJoinSupportBean](env, "SupportBean").Window(LengthWindow(1))
	hist := FromHistorical[map[string]any](env, "MyDBCache", myintSchema, provider)

	query := Join(sbStream, hist, OnEqual(
		Field[dbJoinSupportBean, int]("intPrimitive"),
		Field[map[string]any, int]("myint"),
	)).LeftOuter().Select(
		SelectLeft("intPrimitive", Field[dbJoinSupportBean, int]("intPrimitive")),
		SelectRight("myint", Field[map[string]any, int]("myint")),
	).Where(IsNull[int](JoinField[int](1, "myint"))).Query(StatementName("s0-cache-outer"))

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

	// intPrimitive=-1: no match in myint table -> myint null -> output
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{TheString: "E1", IntPrimitive: -1}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after E1/-1, got %d", len(rows))
	}

	// intPrimitive=10: myint=10 exists in table -> match -> filtered by where
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{TheString: "E2", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if len(rows) != 1 {
		t.Fatalf("expected still 1 row after E2/10 (filtered), got %d", len(rows))
	}

	// intPrimitive=1: myint=1 not in table -> no match -> output
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows after E1/1, got %d", len(rows))
	}
}

func TestDatabaseJoinOptionsNoMetaLexMatchesJava(t *testing.T) {
	// Java EPLDatabaseNoMetaLexAnalysis: SQL with substitution parameter
	// where intPrimitive = myint, returns mydouble.
	// The Go API uses the chainable builder; metadata SQL lex analysis is
	// an Esper compile-time concern not applicable to Go's fluent API.
	dbJoinSetHandler(dbJoinMyDoubleWhereMyIntHandler)
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	db := dbJoinOpenDB(t)
	defer db.Close()

	doubleSchema, err := NewMapSchema("HistOptDouble", []FieldSpec{
		FieldDef("mydouble", reflect.TypeOf(0.0)),
	})
	if err != nil {
		t.Fatal(err)
	}

	provider, err := NewSQLHistoricalProvider(db, doubleSchema,
		"select mydouble from mytesttable where ? = myint",
		func(request HistoricalRequest) any {
			return request.Trigger.Get("intPrimitive").Any()
		})
	if err != nil {
		t.Fatal(err)
	}

	hist := FromHistoricalOn[map[string]any](env, "MyDBOpt", "SupportBean", doubleSchema, provider)
	plan, err := env.Build(Select(hist,
		Alias("mydouble", Field[map[string]any, float64]("mydouble")),
	).Query(StatementName("s0-opt-nometa")))
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

	// intPrimitive=10 -> myint=10 -> mydouble=1.2
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after int=10, got %d", len(rows))
	}
	if rows[0]["mydouble"] != 1.2 {
		t.Fatalf("expected mydouble=1.2, got %v", rows[0]["mydouble"])
	}

	// intPrimitive=80 -> myint=80 -> mydouble=8.2
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{IntPrimitive: 80}); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if v, ok := rows[len(rows)-1]["mydouble"].(float64); !ok || v != 8.2 {
		t.Fatalf("expected mydouble=8.2, got %v", rows[len(rows)-1]["mydouble"])
	}
}

func TestDatabaseJoinOptionsPlaceholderMatchesJava(t *testing.T) {
	// Java EPLDatabasePlaceholderWhere: SQL with EPL sample-where placeholder
	// token that should be stripped before execution. The Go chainable API
	// passes SQL text to the provider without lex analysis, so the placeholder
	// test is equivalent to NoMetaLexAnalysis in terms of behavior.
	dbJoinSetHandler(dbJoinMyDoubleWhereMyIntHandler)
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	db := dbJoinOpenDB(t)
	defer db.Close()

	doubleSchema, err := NewMapSchema("HistPlaceholder", []FieldSpec{
		FieldDef("mydouble", reflect.TypeOf(0.0)),
	})
	if err != nil {
		t.Fatal(err)
	}

	provider, err := NewSQLHistoricalProvider(db, doubleSchema,
		"select mydouble from mytesttable where ? = myint",
		func(request HistoricalRequest) any {
			return request.Trigger.Get("intPrimitive").Any()
		})
	if err != nil {
		t.Fatal(err)
	}

	hist := FromHistoricalOn[map[string]any](env, "MyDBPlaceholder", "SupportBean", doubleSchema, provider)
	plan, err := env.Build(Select(hist,
		Alias("mydouble", Field[map[string]any, float64]("mydouble")),
	).Query(StatementName("s0-opt-placeholder")))
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

	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if v, ok := rows[0]["mydouble"].(float64); !ok || v != 1.2 {
		t.Fatalf("expected mydouble=1.2, got %v", rows[0]["mydouble"])
	}

	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{IntPrimitive: 80}); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if v, ok := rows[len(rows)-1]["mydouble"].(float64); !ok || v != 8.2 {
		t.Fatalf("expected mydouble=8.2, got %v", rows[len(rows)-1]["mydouble"])
	}
}
