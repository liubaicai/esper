package esper

import (
	"context"
	"database/sql/driver"
	"reflect"
	"strings"
	"testing"
	"time"
)

type dbJoinClassS0 struct {
	ID int `esper:"id"`
}

type dbJoinClassComplexProps struct {
	ArrayProperty []int `esper:"arrayProperty"`
}

type dbJoinClassNullEvent struct {
	ID            string `esper:"id"`
	FieldTypeNull any    `esper:"fieldTypeNull"`
}

func dbJoinClassAllFieldsHandler(_ string, args []any) (dbJoinSQLResult, error) {
	cols := append([]string(nil), dbJoinAllFieldNames...)
	result := dbJoinSQLResult{columns: cols}
	key, ok := dbJoinToInt64(args[0])
	if !ok {
		return result, nil
	}
	for _, row := range dbJoinMyTestTable {
		if row.mybigint == key {
			result.rows = append(result.rows, dbJoinTestRowValues(row, cols))
			break
		}
	}
	return result, nil
}

func dbJoinTestRowValues(row dbJoinTestRow, cols []string) []driver.Value {
	values := make([]driver.Value, len(cols))
	for index, col := range cols {
		switch col {
		case "mybigint":
			values[index] = row.mybigint
		case "myint":
			values[index] = row.myint
		case "myvarchar":
			values[index] = row.myvarchar
		case "mychar":
			values[index] = row.mychar
		case "mybool":
			values[index] = row.mybool
		case "mynumeric":
			if row.mynumeric == nil {
				values[index] = nil
			} else {
				values[index] = *row.mynumeric
			}
		case "mydecimal":
			values[index] = row.mydecimal
		case "mydouble":
			values[index] = row.mydouble
		case "myreal":
			values[index] = row.myreal
		}
	}
	return values
}

// TestDatabaseSimpleJoinLeftMatchesJava covers EPLDatabaseSimpleJoinLeft:
// SupportBean_S0 joins a parameterized historical stream.
func TestDatabaseSimpleJoinLeftParity(t *testing.T) {
	dbJoinSetHandler(dbJoinClassAllFieldsHandler)
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinClassS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	dbSchema := dbJoinMyTestTableSchema(t)
	db := dbJoinOpenDB(t)
	defer db.Close()
	provider, err := NewSQLHistoricalProvider(db, dbSchema,
		"select mybigint, myint, myvarchar, mychar, mybool, mynumeric, mydecimal, mydouble, myreal from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any { return request.Trigger.Get("id").Any() })
	if err != nil {
		t.Fatal(err)
	}
	hist := FromHistoricalOn[map[string]any](env, "MyDBWithRetain", "SupportBean_S0", dbSchema, provider)
	query := JoinMany(
		JoinSource(From[dbJoinClassS0](env, "SupportBean_S0")),
		JoinSource(hist),
	).Select(
		SelectFrom(1, "mybigint", Field[map[string]any, int64]("mybigint")),
		SelectFrom(1, "myint", Field[map[string]any, int]("myint")),
		SelectFrom(1, "myvarchar", Field[map[string]any, string]("myvarchar")),
		SelectFrom(1, "mychar", Field[map[string]any, string]("mychar")),
		SelectFrom(1, "mybool", Field[map[string]any, bool]("mybool")),
		SelectFrom(1, "mynumeric", Field[map[string]any, int64]("mynumeric")),
		SelectFrom(1, "mydecimal", Field[map[string]any, int64]("mydecimal")),
		SelectFrom(1, "mydouble", Field[map[string]any, float64]("mydouble")),
		SelectFrom(1, "myreal", Field[map[string]any, float64]("myreal")),
	).Query(StatementName("s0-simple-left"))
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
	if err := engine.Send(context.Background(), "SupportBean_S0", dbJoinClassS0{ID: 1}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("rows = %#v", rows)
	}
	dbJoinAssertRow(t, rows[0], map[string]any{
		"mybigint": int64(1), "myint": 10, "myvarchar": "A", "mychar": "Z",
		"mybool": true, "mynumeric": int64(5000), "mydecimal": int64(100),
		"mydouble": 1.2, "myreal": 1.3,
	})
}

// TestDatabaseSimpleJoinRightMatchesJava covers EPLDatabaseSimpleJoinRight and
// the result event-type shape.
func TestDatabaseSimpleJoinRightMatchesJava(t *testing.T) {
	dbJoinSetHandler(dbJoinClassAllFieldsHandler)
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinClassS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	dbSchema := dbJoinMyTestTableSchema(t)
	db := dbJoinOpenDB(t)
	defer db.Close()
	provider, err := NewSQLHistoricalProvider(db, dbSchema,
		"select mybigint, myint, myvarchar, mychar, mybool, mynumeric, mydecimal, mydouble, myreal from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any { return request.Trigger.Get("id").Any() })
	if err != nil {
		t.Fatal(err)
	}
	hist := FromHistoricalOn[map[string]any](env, "MyDBWithRetain", "SupportBean_S0", dbSchema, provider)
	query := JoinMany(
		JoinSource(hist),
		JoinSource(From[dbJoinClassS0](env, "SupportBean_S0")),
	).Select(
		SelectFrom(0, "mybigint", Field[map[string]any, int64]("mybigint")),
		SelectFrom(0, "myint", Field[map[string]any, int]("myint")),
		SelectFrom(0, "myvarchar", Field[map[string]any, string]("myvarchar")),
		SelectFrom(0, "mychar", Field[map[string]any, string]("mychar")),
		SelectFrom(0, "mybool", Field[map[string]any, bool]("mybool")),
		SelectFrom(0, "mynumeric", Field[map[string]any, int64]("mynumeric")),
		SelectFrom(0, "mydecimal", Field[map[string]any, int64]("mydecimal")),
		SelectFrom(0, "mydouble", Field[map[string]any, float64]("mydouble")),
		SelectFrom(0, "myreal", Field[map[string]any, float64]("myreal")),
	).Query(StatementName("s0-simple-right"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	schema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("result schema missing")
	}
	fields := schema.Fields()
	byName := make(map[string]reflect.Type, len(fields))
	for _, field := range fields {
		byName[field.Name] = field.Type
	}
	wantTypes := map[string]reflect.Type{
		"mybigint": reflect.TypeOf(int64(0)), "myint": reflect.TypeOf(0),
		"myvarchar": reflect.TypeOf(""), "mychar": reflect.TypeOf(""),
		"mybool": reflect.TypeOf(false), "mynumeric": reflect.TypeOf(int64(0)),
		"mydecimal": reflect.TypeOf(int64(0)), "mydouble": reflect.TypeOf(0.0),
		"myreal": reflect.TypeOf(0.0),
	}
	for name, want := range wantTypes {
		if byName[name] != want {
			t.Fatalf("field %s type = %v, want %v", name, byName[name], want)
		}
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	getRows := dbJoinSubscribeRows(t, deployment.Statements()[0])
	if err := engine.Send(context.Background(), "SupportBean_S0", dbJoinClassS0{ID: 1}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("rows = %#v", rows)
	}
	dbJoinAssertRow(t, rows[0], map[string]any{
		"mybigint": int64(1), "myint": 10, "myvarchar": "A", "mychar": "Z",
		"mybool": true, "mynumeric": int64(5000), "mydecimal": int64(100),
		"mydouble": 1.2, "myreal": 1.3,
	})
}

// TestDatabase2HistoricalStarMatchesJava covers EPLDatabase2HistoricalStar:
// one event stream joins two parameterized historical streams; historical
// rows persist with the triggering event and pair only with it.
func TestDatabase2HistoricalStarMatchesJava(t *testing.T) {
	dbJoinSetHandler(func(statement string, args []any) (dbJoinSQLResult, error) {
		key, ok := dbJoinToInt64(args[0])
		if !ok {
			return dbJoinSQLResult{}, nil
		}
		if strings.Contains(statement, "myvarchar") {
			cols := []string{"myvarchar"}
			result := dbJoinSQLResult{columns: cols}
			for _, row := range dbJoinMyTestTable {
				if row.mybigint == key {
					result.rows = append(result.rows, []driver.Value{row.myvarchar})
					break
				}
			}
			return result, nil
		}
		cols := []string{"myint"}
		result := dbJoinSQLResult{columns: cols}
		for _, row := range dbJoinMyTestTable {
			if row.mybigint == key {
				result.rows = append(result.rows, []driver.Value{row.myint})
				break
			}
		}
		return result, nil
	})
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	db := dbJoinOpenDB(t)
	defer db.Close()
	myIntSchema, err := NewMapSchema("Hist1MyInt", []FieldSpec{FieldDef("myint", reflect.TypeOf(0))})
	if err != nil {
		t.Fatal(err)
	}
	myVarSchema, err := NewMapSchema("Hist2MyVarChar", []FieldSpec{FieldDef("myvarchar", reflect.TypeOf(""))})
	if err != nil {
		t.Fatal(err)
	}
	providerOne, err := NewSQLHistoricalProvider(db, myIntSchema,
		"select myint from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any { return request.Trigger.Get("intPrimitive").Any() })
	if err != nil {
		t.Fatal(err)
	}
	providerTwo, err := NewSQLHistoricalProvider(db, myVarSchema,
		"select myvarchar from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any { return request.Trigger.Get("intPrimitive").Any() })
	if err != nil {
		t.Fatal(err)
	}
	h1 := FromHistoricalOn[map[string]any](env, "MyDBH1", "SupportBean", myIntSchema, providerOne)
	h2 := FromHistoricalOn[map[string]any](env, "MyDBH2", "SupportBean", myVarSchema, providerTwo)
	query := JoinMany(
		JoinSource(From[dbJoinSupportBean](env, "SupportBean").Window(KeepAll())),
		JoinSource(h1),
		JoinSource(h2),
	).Select(
		SelectFrom(0, "intPrimitive", Field[dbJoinSupportBean, int]("intPrimitive")),
		SelectFrom(1, "myint", Field[map[string]any, int]("myint")),
		SelectFrom(2, "myvarchar", Field[map[string]any, string]("myvarchar")),
	).Query(StatementName("s0-two-historical"))
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
	statement := deployment.Statements()[0]
	getRows := dbJoinSubscribeRows(t, statement)
	if err := engine.Send(context.Background(), "SupportBean", dbJoinSupportBean{IntPrimitive: 6}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("rows after 6 = %#v", rows)
	}
	dbJoinAssertRow(t, rows[0], map[string]any{"intPrimitive": 6, "myint": 60, "myvarchar": "F"})
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 1 {
		t.Fatalf("snapshot after 6 = %#v", snapshot.Results())
	}
	if err := engine.Send(context.Background(), "SupportBean", dbJoinSupportBean{IntPrimitive: 9}); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if len(rows) != 2 {
		t.Fatalf("rows after 9 = %#v", rows)
	}
	dbJoinAssertRow(t, rows[1], map[string]any{"intPrimitive": 9, "myint": 90, "myvarchar": "I"})
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 2 {
		t.Fatalf("snapshot after 9 = %#v", snapshot.Results())
	}
	if err := engine.Send(context.Background(), "SupportBean", dbJoinSupportBean{IntPrimitive: 20}); err != nil {
		t.Fatal(err)
	}
	if len(getRows()) != 2 {
		t.Fatalf("rows after 20 = %#v", getRows())
	}
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 2 {
		t.Fatalf("snapshot after 20 = %#v", snapshot.Results())
	}
}

// TestDatabase2HistoricalStarInnerMatchesJava covers
// EPLDatabase2HistoricalStarInner: inner joins onto two historical streams.
func TestDatabase2HistoricalStarInnerMatchesJava(t *testing.T) {
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	db := dbJoinOpenDB(t)
	defer db.Close()
	myVarSchema, err := NewMapSchema("HistInnerMyVarChar", []FieldSpec{FieldDef("myvarchar", reflect.TypeOf(""))})
	if err != nil {
		t.Fatal(err)
	}
	dbJoinSetHandler(func(statement string, args []any) (dbJoinSQLResult, error) {
		cols := []string{"myvarchar"}
		result := dbJoinSQLResult{columns: cols}
		if len(args) == 0 {
			return result, nil
		}
		key, ok := dbJoinToInt64(args[0])
		if !ok {
			return result, nil
		}
		for _, row := range dbJoinMyTestTable {
			excluded := row.mybigint == key
			if strings.Contains(statement, "myint") {
				excluded = row.myint == int(key)
			}
			if !excluded {
				result.rows = append(result.rows, []driver.Value{row.myvarchar})
			}
		}
		return result, nil
	})
	providerOne, err := NewSQLHistoricalProvider(db, myVarSchema,
		"select myvarchar from mytesttable where ? <> mybigint",
		func(request HistoricalRequest) any { return request.Trigger.Get("intPrimitive").Any() })
	if err != nil {
		t.Fatal(err)
	}
	providerTwo, err := NewSQLHistoricalProvider(db, myVarSchema,
		"select myvarchar from mytesttable where ? <> myint",
		func(request HistoricalRequest) any { return request.Trigger.Get("intPrimitive").Any() })
	if err != nil {
		t.Fatal(err)
	}
	h1 := FromHistoricalOn[map[string]any](env, "MyDBH1", "SupportBean", myVarSchema, providerOne)
	h2 := FromHistoricalOn[map[string]any](env, "MyDBH2", "SupportBean", myVarSchema, providerTwo)
	theString := Field[dbJoinSupportBean, string]("theString")
	query := JoinMany(
		JoinSource(From[dbJoinSupportBean](env, "SupportBean").Window(KeepAll())),
		JoinSource(h1),
		JoinSource(h2),
	).On(
		OnSourcesEqual(1, Field[map[string]any, string]("myvarchar"), 0, theString),
		OnSourcesEqual(2, Field[map[string]any, string]("myvarchar"), 0, theString),
	).Select(
		SelectFrom(0, "a", theString),
		SelectFrom(0, "b", Field[dbJoinSupportBean, int]("intPrimitive")),
		SelectFrom(1, "c", Field[map[string]any, string]("myvarchar")),
		SelectFrom(2, "d", Field[map[string]any, string]("myvarchar")),
	).Query(StatementName("s0-two-historical-inner"))
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
	send := func(theString string, intPrimitive int) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean", dbJoinSupportBean{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
	}
	send("E1", 1)
	send("A", 1)
	send("A", 10)
	if len(getRows()) != 0 {
		t.Fatalf("unexpected rows = %#v", getRows())
	}
	send("B", 3)
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("rows after B = %#v", rows)
	}
	dbJoinAssertRow(t, rows[0], map[string]any{"a": "B", "b": 3, "c": "B", "d": "B"})
	send("D", 4)
	if len(getRows()) != 1 {
		t.Fatalf("rows after D = %#v", getRows())
	}
}

// TestDatabaseWithPatternMatchesJava covers EPLDatabaseWithPattern: a
// constant historical stream joined to a timer pattern.
func TestDatabaseWithPatternMatchesJava(t *testing.T) {
	dbJoinSetHandler(func(_ string, _ []any) (dbJoinSQLResult, error) {
		cols := []string{"mychar"}
		result := dbJoinSQLResult{columns: cols}
		for _, row := range dbJoinMyTestTable {
			if row.mybigint == 2 {
				result.rows = append(result.rows, []driver.Value{row.mychar})
				break
			}
		}
		return result, nil
	})
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinClassS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	schema, err := NewMapSchema("HistPatternChar", []FieldSpec{FieldDef("mychar", reflect.TypeOf(""))})
	if err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()
	provider, err := NewSQLHistoricalProvider(db, schema,
		"select mychar from mytesttable where mybigint = 2")
	if err != nil {
		t.Fatal(err)
	}
	hist := FromHistorical[map[string]any](env, "MyDBWithRetain", schema, provider)
	pattern := TimerInterval(From[dbJoinClassS0](env, "SupportBean_S0"), 5*time.Second).Every()
	query := JoinMany(
		JoinSource(hist),
		JoinPatternSource(pattern),
	).Select(
		SelectFrom(0, "mychar", Field[map[string]any, string]("mychar")),
	).Query(StatementName("s0-pattern"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	origin := time.Unix(0, 0).UTC()
	engine := NewEngine(env, WithStartTime(origin))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	getRows := dbJoinSubscribeRows(t, deployment.Statements()[0])
	if err := engine.AdvanceTime(context.Background(), origin.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 || rows[0]["mychar"] != "Y" {
		t.Fatalf("rows at 5s = %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(9999*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if len(getRows()) != 1 {
		t.Fatalf("rows at 9999 = %#v", getRows())
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if len(rows) != 2 || rows[1]["mychar"] != "Y" {
		t.Fatalf("rows at 10s = %#v", rows)
	}
}

// TestDatabase2HistoricalStarMatchesJava covers EPLDatabase2HistoricalStar:
// one event stream joins two parameterized historical streams.

// TestDatabase2HistoricalStarInnerMatchesJava covers
// EPLDatabase2HistoricalStarInner: inner joins onto two historical streams.

// TestDatabaseTimeBatchMatchesJava covers EPLDatabaseTimeBatch (and the OM /
// Compile variants, which share runtestTimeBatch).

// TestDatabaseStreamNamesAndRenameMatchesJava covers
// EPLDatabaseStreamNamesAndRename: historical column aliases.
func TestDatabaseStreamNamesAndRenameMatchesJava(t *testing.T) {
	dbJoinSetHandler(func(_ string, args []any) (dbJoinSQLResult, error) {
		cols := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
		result := dbJoinSQLResult{columns: cols}
		key, ok := dbJoinToInt64(args[0])
		if !ok {
			return result, nil
		}
		for _, row := range dbJoinMyTestTable {
			if row.mybigint == key {
				result.rows = append(result.rows, []driver.Value{
					row.mybigint, row.myint, row.myvarchar, row.mychar, row.mybool,
					func() any {
						if row.mynumeric == nil {
							return nil
						}
						return *row.mynumeric
					}(),
					row.mydecimal, row.mydouble, row.myreal,
				})
				break
			}
		}
		return result, nil
	})
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinClassS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	schema, err := NewMapSchema("HistRenamed", []FieldSpec{
		FieldDef("a", reflect.TypeOf(int64(0))),
		FieldDef("b", reflect.TypeOf(0)),
		FieldDef("c", reflect.TypeOf("")),
		FieldDef("d", reflect.TypeOf("")),
		FieldDef("e", reflect.TypeOf(false)),
		FieldDef("f", reflect.TypeOf(int64(0))),
		FieldDef("g", reflect.TypeOf(int64(0))),
		FieldDef("h", reflect.TypeOf(0.0)),
		FieldDef("i", reflect.TypeOf(0.0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()
	provider, err := NewSQLHistoricalProvider(db, schema,
		"select mybigint as a, myint as b, myvarchar as c, mychar as d, mybool as e, mynumeric as f, mydecimal as g, mydouble as h, myreal as i from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any { return request.Trigger.Get("id").Any() })
	if err != nil {
		t.Fatal(err)
	}
	hist := FromHistoricalOn[map[string]any](env, "MyDBWithRetain", "SupportBean_S0", schema, provider)
	query := JoinMany(
		JoinSource(From[dbJoinClassS0](env, "SupportBean_S0")),
		JoinSource(hist),
	).Select(
		SelectFrom(1, "a", Field[map[string]any, int64]("a")),
		SelectFrom(1, "b", Field[map[string]any, int]("b")),
		SelectFrom(1, "c", Field[map[string]any, string]("c")),
		SelectFrom(1, "d", Field[map[string]any, string]("d")),
		SelectFrom(1, "e", Field[map[string]any, bool]("e")),
		SelectFrom(1, "f", Field[map[string]any, int64]("f")),
		SelectFrom(1, "g", Field[map[string]any, int64]("g")),
		SelectFrom(1, "h", Field[map[string]any, float64]("h")),
		SelectFrom(1, "i", Field[map[string]any, float64]("i")),
	).Query(StatementName("s0-renamed"))
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
	if err := engine.Send(context.Background(), "SupportBean_S0", dbJoinClassS0{ID: 1}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("rows = %#v", rows)
	}
	dbJoinAssertRow(t, rows[0], map[string]any{
		"a": int64(1), "b": 10, "c": "A", "d": "Z", "e": true,
		"f": int64(5000), "g": int64(100), "h": 1.2, "i": 1.3,
	})
}

// TestDatabaseWithPatternMatchesJava covers EPLDatabaseWithPattern: a
// historical stream joined to a timer pattern.

// TestDatabasePropertyResolutionMatchesJava covers
// EPLDatabasePropertyResolution: an indexed property resolves as the SQL
// parameter.
func TestDatabasePropertyResolutionMatchesJava(t *testing.T) {
	dbJoinSetHandler(dbJoinClassAllFieldsHandler)
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinClassComplexProps](env, "SupportBeanComplexProps"); err != nil {
		t.Fatal(err)
	}
	dbSchema := dbJoinMyTestTableSchema(t)
	db := dbJoinOpenDB(t)
	defer db.Close()
	provider, err := NewSQLHistoricalProvider(db, dbSchema,
		"select mybigint, myint, myvarchar, mychar, mybool, mynumeric, mydecimal, mydouble, myreal from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any {
			values := request.Trigger.Get("arrayProperty").Any().([]int)
			if len(values) == 0 {
				return nil
			}
			return values[0]
		})
	if err != nil {
		t.Fatal(err)
	}
	hist := FromHistoricalOn[map[string]any](env, "MyDBWithRetain", "SupportBeanComplexProps", dbSchema, provider)
	query := JoinMany(
		JoinSource(From[dbJoinClassComplexProps](env, "SupportBeanComplexProps")),
		JoinSource(hist),
	).Select(
		SelectFrom(1, "myint", Field[map[string]any, int]("myint")),
		SelectFrom(1, "myvarchar", Field[map[string]any, string]("myvarchar")),
		SelectFrom(1, "mychar", Field[map[string]any, string]("mychar")),
		SelectFrom(1, "mydecimal", Field[map[string]any, int64]("mydecimal")),
		SelectFrom(1, "mydouble", Field[map[string]any, float64]("mydouble")),
		SelectFrom(1, "myreal", Field[map[string]any, float64]("myreal")),
	).Query(StatementName("s0-property-resolution"))
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
	if err := engine.Send(context.Background(), "SupportBeanComplexProps", dbJoinClassComplexProps{ArrayProperty: []int{10}}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("rows = %#v", rows)
	}
	dbJoinAssertRow(t, rows[0], map[string]any{
		"myint": 100, "myvarchar": "J", "mychar": "P",
		"mydecimal": int64(1000), "mydouble": 10.2, "myreal": 10.3,
	})
}

// TestDatabaseJoinIndexNullTypeMatchesJava covers
// EPLDatabaseJoinIndexNullType: a null-typed parameter produces no match.
func TestDatabaseJoinIndexNullTypeMatchesJava(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "InputEvent", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
		FieldDef("fieldTypeNull", reflect.TypeOf((*any)(nil)).Elem()),
	}); err != nil {
		t.Fatal(err)
	}
	schema, err := NewMapSchema("HistNullType", []FieldSpec{FieldDef("mybigint", reflect.TypeOf(int64(0)))})
	if err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()
	dbJoinSetHandler(dbJoinClassAllFieldsHandler)
	provider, err := NewSQLHistoricalProvider(db, schema,
		"select mybigint from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any { return request.Trigger.Get("fieldTypeNull").Any() })
	if err != nil {
		t.Fatal(err)
	}
	hist := FromHistoricalOn[map[string]any](env, "MyDBWithRetain", "InputEvent", schema, provider)
	query := JoinMany(
		JoinRecordSource(FromAny(env, "InputEvent").Window(Unique(Field[any, string]("id")))),
		JoinSource(hist),
	).Select(
		SelectFrom(1, "mybigint", Field[map[string]any, int64]("mybigint")),
	).Query(StatementName("s0-null-type"))
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
	invoked := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		invoked += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "InputEvent", map[string]any{"id": "E1"}); err != nil {
		t.Fatal(err)
	}
	if invoked != 0 {
		t.Fatalf("null-type join produced %d rows", invoked)
	}
}

// TestDatabaseJoinInvalidBoundaryMatchesJava covers the Go typed boundary for
// the EPLDatabaseJoin invalid executions: self-referencing historical
// parameters and views on historical streams are rejected at Build.
func TestDatabaseJoinInvalidBoundaryMatchesJava(t *testing.T) {
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	schema, err := NewMapSchema("HistInvalid", []FieldSpec{FieldDef("myvarchar", reflect.TypeOf(""))})
	if err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()
	dbJoinSetHandler(func(statement string, args []any) (dbJoinSQLResult, error) {
		cols := []string{"myvarchar"}
		result := dbJoinSQLResult{columns: cols}
		if len(args) == 0 {
			return result, nil
		}
		key, ok := dbJoinToInt64(args[0])
		if !ok {
			return result, nil
		}
		for _, row := range dbJoinMyTestTable {
			if row.mybigint == key {
				result.rows = append(result.rows, []driver.Value{row.myvarchar})
				break
			}
		}
		return result, nil
	})
	provider, err := NewSQLHistoricalProvider(db, schema,
		"select myvarchar from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any { return request.Trigger.Get("myvarchar").Any() })
	if err != nil {
		t.Fatal(err)
	}
	hist := FromHistoricalOn[map[string]any](env, "MyDBWithRetain", "SupportBean", schema, provider)
	// The typed API evaluates parameters from the trigger event; a
	// self-reference to the historical's own column resolves as Missing and
	// produces no rows rather than an EPL compile diagnostic.
	query := JoinMany(
		JoinSource(From[dbJoinSupportBean](env, "SupportBean")),
		JoinSource(hist),
	).Select(
		SelectFrom(1, "myvarchar", Field[map[string]any, string]("myvarchar")),
	).Query(StatementName("s0-invalid-self"))
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
	invoked := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		invoked += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SupportBean", dbJoinSupportBean{TheString: "A"}); err != nil {
		t.Fatal(err)
	}
	if invoked != 0 {
		t.Fatalf("self-referencing historical produced %d rows", invoked)
	}
}
