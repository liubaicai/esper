package esper

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type dbJoinSupportBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	BoolPrimitive bool   `esper:"boolPrimitive"`
	IntBoxed      *int   `esper:"intBoxed"`
}

type dbJoinTestRow struct {
	mybigint  int64
	myint     int
	myvarchar string
	mychar    string
	mybool    bool
	mynumeric *int64
	mydecimal int64
	mydouble  float64
	myreal    float64
}

func dbJoinI64Ptr(v int64) *int64 { x := v; return &x }

var dbJoinMyTestTable = []dbJoinTestRow{
	{1, 10, "A", "Z", true, dbJoinI64Ptr(5000), 100, 1.2, 1.3},
	{2, 20, "B", "Y", false, dbJoinI64Ptr(100), 200, 2.2, 2.3},
	{3, 30, "C", "X", false, dbJoinI64Ptr(100), 300, 3.2, 3.3},
	{4, 40, "D", "W", true, dbJoinI64Ptr(500), 400, 4.2, 4.3},
	{5, 50, "E", "V", false, dbJoinI64Ptr(500), 500, 5.2, 5.3},
	{6, 60, "F", "T", false, dbJoinI64Ptr(200), 600, 6.2, 6.3},
	{7, 70, "G", "S", true, nil, 700, 7.2, 7.3},
	{8, 80, "H", "R", true, nil, 800, 8.2, 8.3},
	{9, 90, "I", "Q", true, nil, 900, 9.2, 9.3},
	{10, 100, "J", "P", true, nil, 1000, 10.2, 10.3},
}

var dbJoinAllFieldNames = []string{
	"mybigint", "myint", "myvarchar", "mychar",
	"mybool", "mynumeric", "mydecimal", "mydouble", "myreal",
}

type dbJoinSQLResult struct {
	columns []string
	rows    [][]driver.Value
}

type dbJoinSQLHandler func(statement string, args []any) (dbJoinSQLResult, error)

var dbJoinFixtureMu sync.Mutex
var dbJoinFixtureHandler dbJoinSQLHandler

func dbJoinSetHandler(handler dbJoinSQLHandler) {
	dbJoinFixtureMu.Lock()
	dbJoinFixtureHandler = handler
	dbJoinFixtureMu.Unlock()
}

type dbJoinSQLDriver struct{}

var dbJoinDriverOnce sync.Once

func dbJoinEnsureDriver() {
	dbJoinDriverOnce.Do(func() {
		sql.Register("esper-dbjoin", dbJoinSQLDriver{})
	})
}

func (dbJoinSQLDriver) Open(string) (driver.Conn, error) { return dbJoinSQLConn{}, nil }

type dbJoinSQLConn struct{}

func (dbJoinSQLConn) Prepare(statement string) (driver.Stmt, error) {
	return dbJoinSQLStmt{statement: statement}, nil
}
func (dbJoinSQLConn) Close() error              { return nil }
func (dbJoinSQLConn) Begin() (driver.Tx, error) { return dbJoinSQLTx{}, nil }
func (dbJoinSQLConn) QueryContext(ctx context.Context, statement string, args []driver.NamedValue) (driver.Rows, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	return dbJoinExecuteSQL(statement, dbJoinNamedToArgs(args))
}

type dbJoinSQLStmt struct{ statement string }

func (s dbJoinSQLStmt) Close() error  { return nil }
func (s dbJoinSQLStmt) NumInput() int { return -1 }
func (s dbJoinSQLStmt) Exec([]driver.Value) (driver.Result, error) {
	return driver.RowsAffected(0), nil
}
func (s dbJoinSQLStmt) Query(args []driver.Value) (driver.Rows, error) {
	return dbJoinExecuteSQL(s.statement, dbJoinValuesToArgs(args))
}
func (s dbJoinSQLStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	return dbJoinExecuteSQL(s.statement, dbJoinNamedToArgs(args))
}

type dbJoinSQLTx struct{}

func (dbJoinSQLTx) Commit() error   { return nil }
func (dbJoinSQLTx) Rollback() error { return nil }

func dbJoinExecuteSQL(statement string, args []any) (driver.Rows, error) {
	dbJoinFixtureMu.Lock()
	handler := dbJoinFixtureHandler
	dbJoinFixtureMu.Unlock()
	if handler == nil {
		return nil, fmt.Errorf("dbJoin: no SQL handler set")
	}
	result, err := handler(statement, args)
	if err != nil {
		return nil, err
	}
	return &dbJoinSQLRows{result: result}, nil
}

type dbJoinSQLRows struct {
	result dbJoinSQLResult
	pos    int
}

func (r *dbJoinSQLRows) Columns() []string { return r.result.columns }
func (r *dbJoinSQLRows) Close() error      { return nil }
func (r *dbJoinSQLRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.result.rows) {
		return io.EOF
	}
	copy(dest, r.result.rows[r.pos])
	r.pos++
	return nil
}

func dbJoinNamedToArgs(nvs []driver.NamedValue) []any {
	args := make([]any, len(nvs))
	for i, nv := range nvs {
		args[i] = nv.Value
	}
	return args
}

func dbJoinValuesToArgs(vs []driver.Value) []any {
	args := make([]any, len(vs))
	for i, v := range vs {
		args[i] = v
	}
	return args
}

func dbJoinRowToValues(row dbJoinTestRow, cols []string) []driver.Value {
	values := make([]driver.Value, len(cols))
	for i, col := range cols {
		switch col {
		case "mybigint":
			values[i] = row.mybigint
		case "myint":
			values[i] = row.myint
		case "myvarchar", "MyVarChar":
			values[i] = row.myvarchar
		case "mychar":
			values[i] = row.mychar
		case "mybool":
			values[i] = row.mybool
		case "mynumeric":
			if row.mynumeric != nil {
				values[i] = *row.mynumeric
			}
		case "mydecimal":
			values[i] = row.mydecimal
		case "mydouble":
			values[i] = row.mydouble
		case "myreal":
			values[i] = row.myreal
		case "a":
			values[i] = nil
		}
	}
	return values
}

func dbJoinToInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case int32:
		return int64(n), true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}

func dbJoinToBool(v any) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case int64:
		return b != 0, true
	case int:
		return b != 0, true
	default:
		return false, false
	}
}

func dbJoinAllFieldsWhereBigintHandler(_ string, args []any) (dbJoinSQLResult, error) {
	result := dbJoinSQLResult{columns: dbJoinAllFieldNames}
	var filter *int64
	if len(args) > 0 {
		if v, ok := dbJoinToInt64(args[0]); ok {
			filter = &v
		}
	}
	for _, row := range dbJoinMyTestTable {
		if filter != nil && row.mybigint != *filter {
			continue
		}
		result.rows = append(result.rows, dbJoinRowToValues(row, dbJoinAllFieldNames))
	}
	return result, nil
}

func dbJoinAllRowsMyVarCharHandler(_ string, _ []any) (dbJoinSQLResult, error) {
	cols := []string{"MyVarChar"}
	result := dbJoinSQLResult{columns: cols}
	for _, row := range dbJoinMyTestTable {
		result.rows = append(result.rows, []driver.Value{row.myvarchar})
	}
	return result, nil
}

func dbJoinNullSelectHandler(_ string, _ []any) (dbJoinSQLResult, error) {
	return dbJoinSQLResult{columns: []string{"a"}}, nil
}

func dbJoinMyIntWhereBigintBetweenHandler(_ string, args []any) (dbJoinSQLResult, error) {
	cols := []string{"myint"}
	result := dbJoinSQLResult{columns: cols}
	if len(args) < 2 {
		return result, nil
	}
	lower, lok := dbJoinToInt64(args[0])
	upper, uok := dbJoinToInt64(args[1])
	if !lok || !uok {
		return result, nil
	}
	for _, row := range dbJoinMyTestTable {
		if row.mybigint >= lower && row.mybigint <= upper {
			result.rows = append(result.rows, []driver.Value{row.myint})
		}
	}
	return result, nil
}

func dbJoinMyBigIntBoolHandler(_ string, args []any) (dbJoinSQLResult, error) {
	cols := []string{"mybigint", "mybool"}
	result := dbJoinSQLResult{columns: cols}
	if len(args) < 3 {
		return result, nil
	}
	bv, bok := dbJoinToBool(args[0])
	lower, lok := dbJoinToInt64(args[1])
	upper, uok := dbJoinToInt64(args[2])
	if !bok || !lok || !uok {
		return result, nil
	}
	for _, row := range dbJoinMyTestTable {
		if row.mybool == bv && int64(row.myint) >= lower && int64(row.myint) <= upper {
			result.rows = append(result.rows, []driver.Value{row.mybigint, row.mybool})
		}
	}
	return result, nil
}

func dbJoinMyIntWhereBigintHandler(_ string, args []any) (dbJoinSQLResult, error) {
	cols := []string{"myint"}
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
			result.rows = append(result.rows, []driver.Value{row.myint})
		}
	}
	return result, nil
}

func dbJoinAllFieldsWhereMyintHandler(_ string, args []any) (dbJoinSQLResult, error) {
	result := dbJoinSQLResult{columns: dbJoinAllFieldNames}
	if len(args) == 0 {
		return result, nil
	}
	v, ok := dbJoinToInt64(args[0])
	if !ok {
		return result, nil
	}
	for _, row := range dbJoinMyTestTable {
		if int64(row.myint) == v {
			result.rows = append(result.rows, dbJoinRowToValues(row, dbJoinAllFieldNames))
		}
	}
	return result, nil
}
func dbJoinMyTestTableSchema(t *testing.T) Schema {
	t.Helper()
	schema, err := NewMapSchema("mytesttable", []FieldSpec{
		FieldDef("mybigint", reflect.TypeOf(int64(0))),
		FieldDef("myint", reflect.TypeOf(0)),
		FieldDef("myvarchar", reflect.TypeOf("")),
		FieldDef("mychar", reflect.TypeOf("")),
		FieldDef("mybool", reflect.TypeOf(false)),
		FieldDef("mynumeric", reflect.TypeOf(int64(0))),
		FieldDef("mydecimal", reflect.TypeOf(int64(0))),
		FieldDef("mydouble", reflect.TypeOf(0.0)),
		FieldDef("myreal", reflect.TypeOf(0.0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func dbJoinOpenDB(t *testing.T) *sql.DB {
	t.Helper()
	dbJoinEnsureDriver()
	db, err := sql.Open("esper-dbjoin", "")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func dbJoinRegisterSupportBean(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterStruct[dbJoinSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
}

func dbJoinSubscribeRows(t *testing.T, stmt *Statement) func() []map[string]any {
	t.Helper()
	var mu sync.Mutex
	var collected []map[string]any
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		mu.Lock()
		defer mu.Unlock()
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				collected = append(collected, row.AsMap())
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return func() []map[string]any {
		mu.Lock()
		defer mu.Unlock()
		return append([]map[string]any(nil), collected...)
	}
}

func dbJoinAssertRow(t *testing.T, row map[string]any, expected map[string]any) {
	t.Helper()
	for key, want := range expected {
		got := row[key]
		if want == nil {
			if got != nil {
				t.Errorf("field %q = %#v, want nil", key, got)
			}
			continue
		}
		if !reflect.DeepEqual(got, want) {
			if gv, gok := dbJoinToInt64(got); gok {
				if wv, wok := dbJoinToInt64(want); wok && gv == wv {
					continue
				}
			}
			t.Errorf("field %q = %#v (%T), want %#v (%T)", key, got, got, want, want)
		}
	}
}
func TestDatabaseJoinLeftOuterMatchesJava(t *testing.T) {
	dbJoinSetHandler(dbJoinAllFieldsWhereBigintHandler)
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	dbSchema := dbJoinMyTestTableSchema(t)
	db := dbJoinOpenDB(t)
	defer db.Close()

	provider, err := NewSQLHistoricalProvider(db, dbSchema,
		"select mybigint, myint, myvarchar, mychar, mybool, mynumeric, mydecimal, mydouble, myreal from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any {
			return request.Trigger.Get("intPrimitive").Any()
		})
	if err != nil {
		t.Fatal(err)
	}

	supportStream := From[dbJoinSupportBean](env, "SupportBean").Window(LengthWindow(1))
	historical := FromHistoricalOn[map[string]any](env, "MyDBJoinLeftOuter", "SupportBean", dbSchema, provider)

	// Build a left-outer join: SupportBean LEFT OUTER JOIN sql.
	// This covers Java EPLDatabaseOuterJoinLeftS0, EPLDatabaseOuterJoinRightS1,
	// EPLDatabaseOuterJoinFullS0, EPLDatabaseOuterJoinFullS1 — all four produce
	// the same listener output: matched row for int=1, null-padded row for int=11.
	query := Join(supportStream, historical, OnEqual(
		Field[dbJoinSupportBean, int]("intPrimitive"),
		Field[map[string]any, int64]("mybigint"),
	)).LeftOuter().Select(
		SelectLeft("MyInt", Field[dbJoinSupportBean, int]("intPrimitive")),
		SelectRight("mybigint", Field[map[string]any, int64]("mybigint")),
		SelectRight("myint", Field[map[string]any, int]("myint")),
		SelectRight("myvarchar", Field[map[string]any, string]("myvarchar")),
		SelectRight("mychar", Field[map[string]any, string]("mychar")),
		SelectRight("mybool", Field[map[string]any, bool]("mybool")),
		SelectRight("mynumeric", Field[map[string]any, int64]("mynumeric")),
		SelectRight("mydecimal", Field[map[string]any, int64]("mydecimal")),
		SelectRight("mydouble", Field[map[string]any, float64]("mydouble")),
		SelectRight("myreal", Field[map[string]any, float64]("myreal")),
	).Query(StatementName("s0-left-outer"))
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

	// int=1 -> matches mybigint=1: full row data
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after int=1, got %d", len(rows))
	}
	dbJoinAssertRow(t, rows[0], map[string]any{
		"MyInt": 1, "mybigint": int64(1), "myint": 10,
		"myvarchar": "A", "mychar": "Z", "mybool": true,
		"mynumeric": int64(5000), "mydecimal": int64(100),
		"mydouble": 1.2, "myreal": 1.3,
	})

	// int=11 -> no SQL match -> null SQL fields (left outer preserves stream)
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{IntPrimitive: 11}); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows total, got %d", len(rows))
	}
	last := rows[1]
	if last["MyInt"] != 11 {
		t.Fatalf("second row MyInt = %#v, want 11", last["MyInt"])
	}
	dbJoinAssertRow(t, last, map[string]any{
		"mybigint": nil, "myint": nil, "myvarchar": nil,
		"mychar": nil, "mybool": nil, "mynumeric": nil,
		"mydecimal": nil, "mydouble": nil, "myreal": nil,
	})
}

func TestDatabaseJoinFullOuterMatchesJava(t *testing.T) {
	dbJoinSetHandler(dbJoinAllFieldsWhereBigintHandler)
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	dbSchema := dbJoinMyTestTableSchema(t)
	db := dbJoinOpenDB(t)
	defer db.Close()

	provider, err := NewSQLHistoricalProvider(db, dbSchema,
		"select mybigint, myint, myvarchar, mychar, mybool, mynumeric, mydecimal, mydouble, myreal from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any {
			return request.Trigger.Get("intPrimitive").Any()
		})
	if err != nil {
		t.Fatal(err)
	}

	supportStream := From[dbJoinSupportBean](env, "SupportBean").Window(LengthWindow(1))
	historical := FromHistoricalOn[map[string]any](env, "MyDBJoinFullOuter", "SupportBean", dbSchema, provider)

	query := Join(supportStream, historical, OnEqual(
		Field[dbJoinSupportBean, int]("intPrimitive"),
		Field[map[string]any, int64]("mybigint"),
	)).FullOuter().Select(
		SelectLeft("MyInt", Field[dbJoinSupportBean, int]("intPrimitive")),
		SelectRight("mybigint", Field[map[string]any, int64]("mybigint")),
		SelectRight("myvarchar", Field[map[string]any, string]("myvarchar")),
	).Query(StatementName("s0-full-outer"))
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

	// int=1 -> matched
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after int=1, got %d", len(rows))
	}
	dbJoinAssertRow(t, rows[0], map[string]any{
		"MyInt": 1, "mybigint": int64(1), "myvarchar": "A",
	})

	// int=11 -> unmatched -> null SQL fields (full outer preserves both sides)
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{IntPrimitive: 11}); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	dbJoinAssertRow(t, rows[1], map[string]any{
		"MyInt": 11, "mybigint": nil, "myvarchar": nil,
	})
}
func TestDatabaseJoinHistoricalPreservedNoUnmatchedOutputMatchesJava(t *testing.T) {
	dbJoinSetHandler(dbJoinAllFieldsWhereBigintHandler)
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	dbSchema := dbJoinMyTestTableSchema(t)
	db := dbJoinOpenDB(t)
	defer db.Close()

	provider, err := NewSQLHistoricalProvider(db, dbSchema,
		"select mybigint, myint, myvarchar, mychar, mybool, mynumeric, mydecimal, mydouble, myreal from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any {
			return request.Trigger.Get("intPrimitive").Any()
		})
	if err != nil {
		t.Fatal(err)
	}

	supportStream := From[dbJoinSupportBean](env, "SupportBean").Window(LengthWindow(1))
	historical := FromHistoricalOn[map[string]any](env, "MyDBJoinHistPreserved", "SupportBean", dbSchema, provider)

	query := Join(supportStream, historical, OnEqual(
		Field[dbJoinSupportBean, int]("intPrimitive"),
		Field[map[string]any, int64]("mybigint"),
	)).RightOuter().Select(
		SelectLeft("MyInt", Field[dbJoinSupportBean, int]("intPrimitive")),
		SelectRight("mybigint", Field[map[string]any, int64]("mybigint")),
		SelectRight("myvarchar", Field[map[string]any, string]("myvarchar")),
	).Query(StatementName("s0-hist-preserved"))
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

	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after int=2, got %d", len(rows))
	}
	dbJoinAssertRow(t, rows[0], map[string]any{
		"MyInt": 2, "mybigint": int64(2), "myvarchar": "B",
	})

	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{IntPrimitive: 11}); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if len(rows) != 1 {
		t.Fatalf("expected still 1 row after unmatched int=11, got %d", len(rows))
	}
}

func TestDatabaseJoinLeftOuterOnFilterMatchesJava(t *testing.T) {
	dbJoinSetHandler(dbJoinAllFieldsWhereBigintHandler)
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	dbSchema := dbJoinMyTestTableSchema(t)
	db := dbJoinOpenDB(t)
	defer db.Close()

	provider, err := NewSQLHistoricalProvider(db, dbSchema,
		"select mybigint, myint, myvarchar, mychar, mybool, mynumeric, mydecimal, mydouble, myreal from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any {
			return request.Trigger.Get("intPrimitive").Any()
		})
	if err != nil {
		t.Fatal(err)
	}

	supportStream := From[dbJoinSupportBean](env, "SupportBean").Window(KeepAll())
	historical := FromHistoricalOn[map[string]any](env, "MyDBJoinOnFilter", "SupportBean", dbSchema, provider)

	query := Join(supportStream, historical, OnEqual(
		Field[dbJoinSupportBean, string]("theString"),
		Field[map[string]any, string]("myvarchar"),
	)).LeftOuter().Select(
		SelectLeft("MyInt", Field[dbJoinSupportBean, int]("intPrimitive")),
		SelectRight("myint", Field[map[string]any, int]("myint")),
	).Query(StatementName("s0-on-filter"))
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

	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{TheString: "xxx", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after xxx/1, got %d", len(rows))
	}
	dbJoinAssertRow(t, rows[0], map[string]any{"MyInt": 1, "myint": nil})

	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{TheString: "xxx", IntPrimitive: -1}); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows after xxx/-1, got %d", len(rows))
	}
	dbJoinAssertRow(t, rows[1], map[string]any{"MyInt": -1, "myint": nil})

	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{TheString: "B", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows after B/2, got %d", len(rows))
	}
	dbJoinAssertRow(t, rows[2], map[string]any{"MyInt": 2, "myint": 20})
}
func TestDatabaseNoJoinNullSelectMatchesJava(t *testing.T) {
	dbJoinSetHandler(dbJoinNullSelectHandler)
	env := NewEnvironment()
	schema, err := NewMapSchema("NullSelectSchema", []FieldSpec{
		FieldDef("a", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()

	provider, err := NewSQLHistoricalProvider(db, schema,
		"select null as a from mytesttable where myint = 1")
	if err != nil {
		t.Fatal(err)
	}
	source := FromHistorical[map[string]any](env, "MyDBNullSelect", schema, provider)
	plan, err := env.Build(Select(source,
		Alias("a", Field[map[string]any, string]("a")),
	).Query(StatementName("s0-null-select")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewEngine(env).ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 0 {
		t.Fatalf("expected 0 results for null select, got %d", len(result.Results()))
	}
}

func TestDatabaseNoJoinExpressionPollMatchesJava(t *testing.T) {
	dbJoinSetHandler(dbJoinMyIntWhereBigintHandler)
	env := NewEnvironment()
	if err := env.RegisterVariable("queryvar_int", 0); err != nil {
		t.Fatal(err)
	}
	dbJoinRegisterSupportBean(t, env)
	schema, err := NewMapSchema("ExprPollSchema", []FieldSpec{
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
			raw, ok := request.Variables["queryvar_int"]
			if !ok {
				return 0
			}
			current, _ := dbJoinToInt64(raw.Any())
			return current - 2
		})
	if err != nil {
		t.Fatal(err)
	}
	source := FromHistorical[map[string]any](env, "MyDBExprPoll", schema, provider)
	plan, err := env.Build(Select(source,
		Alias("myint", Field[map[string]any, int]("myint")),
	).Query(StatementName("s0-expr-poll")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)

	result, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 0 {
		t.Fatalf("expected 0 results before variable set, got %d", len(result.Results()))
	}

	if err := engine.SetVariable(context.Background(), "queryvar_int", 5); err != nil {
		t.Fatal(err)
	}
	result, err = engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 1 {
		t.Fatalf("expected 1 result after queryvar_int=5, got %d", len(result.Results()))
	}
	row, ok := result.Results()[0].Row()
	if !ok || row.Get("myint").Any() != 30 {
		t.Fatalf("expected myint=30, got %#v", result.Results()[0])
	}

	dbJoinSetHandler(dbJoinMyIntWhereBigintBetweenHandler)
	multiProvider, err := NewSQLHistoricalProvider(db, schema,
		"select myint from mytesttable where mybigint between ? and ?",
		func(request HistoricalRequest) any {
			raw, ok := request.Variables["queryvar_int"]
			if !ok {
				return 0
			}
			current, _ := dbJoinToInt64(raw.Any())
			return current - 2
		},
		func(request HistoricalRequest) any {
			raw, ok := request.Variables["queryvar_int"]
			if !ok {
				return 0
			}
			current, _ := dbJoinToInt64(raw.Any())
			return current + 2
		})
	if err != nil {
		t.Fatal(err)
	}
	multiSource := FromHistorical[map[string]any](env, "MyDBExprPollMulti", schema, multiProvider)
	multiPlan, err := env.Build(Select(multiSource,
		Alias("myint", Field[map[string]any, int]("myint")),
	).Query(StatementName("s0-expr-poll-multi")))
	if err != nil {
		t.Fatal(err)
	}
	result, err = engine.ExecuteFireAndForget(context.Background(), multiPlan)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 5 {
		t.Fatalf("expected 5 results for between 3 and 7, got %d", len(result.Results()))
	}
	myints := make(map[int]bool)
	for _, item := range result.Results() {
		row, ok := item.Row()
		if !ok {
			t.Fatalf("result is not a row: %#v", item)
		}
		v, _ := dbJoinToInt64(row.Get("myint").Any())
		myints[int(v)] = true
	}
	for _, expected := range []int{30, 40, 50, 60, 70} {
		if !myints[expected] {
			t.Errorf("expected myint=%d in results, got %#v", expected, myints)
		}
	}
}

func TestDatabaseNoJoinVariablesPollMatchesJava(t *testing.T) {
	dbJoinSetHandler(dbJoinMyBigIntBoolHandler)
	env := NewEnvironment()
	if err := env.RegisterVariable("queryvar_bool", false); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("lower", 0); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("upper", 0); err != nil {
		t.Fatal(err)
	}
	schema, err := NewMapSchema("VarPollSchema", []FieldSpec{
		FieldDef("mybigint", reflect.TypeOf(int64(0))),
		FieldDef("mybool", reflect.TypeOf(false)),
	})
	if err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()

	provider, err := NewSQLHistoricalProvider(db, schema,
		"select mybigint, mybool from mytesttable where ? = mybool and myint between ? and ?",
		func(request HistoricalRequest) any {
			raw, _ := request.Variables["queryvar_bool"]
			return raw.Any()
		},
		func(request HistoricalRequest) any {
			raw, _ := request.Variables["lower"]
			v, _ := dbJoinToInt64(raw.Any())
			return v
		},
		func(request HistoricalRequest) any {
			raw, _ := request.Variables["upper"]
			v, _ := dbJoinToInt64(raw.Any())
			return v
		})
	if err != nil {
		t.Fatal(err)
	}
	source := FromHistorical[map[string]any](env, "MyDBVarPoll", schema, provider)
	plan, err := env.Build(Select(source,
		Alias("mybigint", Field[map[string]any, int64]("mybigint")),
		Alias("mybool", Field[map[string]any, bool]("mybool")),
	).Query(StatementName("s0-var-poll")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)

	if err := engine.SetVariables(context.Background(),
		VariableAssignment{Name: "queryvar_bool", Value: true},
		VariableAssignment{Name: "lower", Value: int64(10)},
		VariableAssignment{Name: "upper", Value: int64(40)},
	); err != nil {
		t.Fatal(err)
	}
	result, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	bigints := make(map[int64]bool)
	for _, item := range result.Results() {
		row, ok := item.Row()
		if !ok {
			t.Fatalf("result is not a row: %#v", item)
		}
		bi, _ := dbJoinToInt64(row.Get("mybigint").Any())
		bigints[bi] = true
		if row.Get("mybool").Any() != true {
			t.Errorf("expected mybool=true for mybigint=%d", bi)
		}
	}
	if !bigints[1] || !bigints[4] || len(bigints) != 2 {
		t.Fatalf("expected mybigint {1,4}, got %#v", bigints)
	}

	if err := engine.SetVariables(context.Background(),
		VariableAssignment{Name: "queryvar_bool", Value: false},
		VariableAssignment{Name: "lower", Value: int64(30)},
		VariableAssignment{Name: "upper", Value: int64(80)},
	); err != nil {
		t.Fatal(err)
	}
	result, err = engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	bigints = make(map[int64]bool)
	for _, item := range result.Results() {
		row, ok := item.Row()
		if !ok {
			t.Fatalf("result is not a row: %#v", item)
		}
		bi, _ := dbJoinToInt64(row.Get("mybigint").Any())
		bigints[bi] = true
	}
	if !bigints[3] || !bigints[5] || !bigints[6] || len(bigints) != 3 {
		t.Fatalf("expected mybigint {3,5,6}, got %#v", bigints)
	}
}

func TestDatabaseNoJoinSubstitutionParameterMatchesJava(t *testing.T) {
	dbJoinSetHandler(dbJoinAllFieldsWhereMyintHandler)
	env := NewEnvironment()
	schema := dbJoinMyTestTableSchema(t)
	db := dbJoinOpenDB(t)
	defer db.Close()

	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select mybigint, myint, myvarchar, mychar, mybool, mynumeric, mydecimal, mydouble, myreal from mytesttable where myint = ?",
		SQLHistoricalProviderOptions{
			ParameterTypes: map[string]reflect.Type{
				"myint": reflect.TypeOf(0),
			},
			Arguments: []func(HistoricalRequest) any{
				func(request HistoricalRequest) any {
					if v, ok := request.Parameters["myint"]; ok {
						return v.Any()
					}
					return 0
				},
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	source := FromHistorical[map[string]any](env, "MyDBSubstParam", schema, provider)
	plan, err := env.Build(Select(source,
		Alias("myvarchar", Field[map[string]any, string]("myvarchar")),
	).Query(StatementName("s0-subst-param")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)

	cases := []struct {
		myint    int
		expected string
	}{
		{10, "A"},
		{50, "E"},
		{30, "C"},
	}
	for _, tc := range cases {
		result, err := engine.ExecuteFireAndForgetWithParameters(context.Background(), plan,
			ParameterValues{"myint": tc.myint})
		if err != nil {
			t.Fatalf("FAF with myint=%d: %v", tc.myint, err)
		}
		if len(result.Results()) != 1 {
			t.Fatalf("myint=%d: expected 1 result, got %d", tc.myint, len(result.Results()))
		}
		row, ok := result.Results()[0].Row()
		if !ok || row.Get("myvarchar").Any() != tc.expected {
			t.Fatalf("myint=%d: expected myvarchar=%q, got %#v", tc.myint, tc.expected, result.Results()[0])
		}
	}
}

func TestDatabaseNoJoinSQLTextParamSubqueryMatchesJava(t *testing.T) {
	dbJoinSetHandler(dbJoinAllFieldsWhereMyintHandler)
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	schema := dbJoinMyTestTableSchema(t)
	db := dbJoinOpenDB(t)
	defer db.Close()

	provider, err := NewSQLHistoricalProvider(db, schema,
		"select mybigint, myint, myvarchar, mychar, mybool, mynumeric, mydecimal, mydouble, myreal from mytesttable where myint = ?",
		func(request HistoricalRequest) any {
			return request.Trigger.Get("intPrimitive").Any()
		})
	if err != nil {
		t.Fatal(err)
	}
	source := FromHistoricalOn[map[string]any](env, "MyDBSubquery", "SupportBean", schema, provider)
	plan, err := env.Build(Select(source,
		Alias("myvarchar", Field[map[string]any, string]("myvarchar")),
	).Query(StatementName("s0-sqltext-subquery")))
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

	// Send SupportBean(intPrimitive=30) -> SQL myint=30 -> myvarchar="C"
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{IntPrimitive: 30}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 listener result after int=30, got %d: %#v", len(rows), rows)
	}
	if rows[0]["myvarchar"] != "C" {
		t.Fatalf("expected myvarchar=C, got %#v", rows[0]["myvarchar"])
	}
}

var _ = strings.Contains
var _ = time.Now
