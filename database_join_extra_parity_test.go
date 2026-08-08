package esper

import (
	"context"
	"database/sql/driver"
	"reflect"
	"strings"
	"testing"
)

type dbJoinExtraS0 struct {
	ID int
}

type dbJoinExtraComplexProps struct {
	ArrayProperty [10]int
}

type dbJoinExtraBeanTwo struct {
	StringTwo       string
	IntPrimitiveTwo int
}

func TestDatabaseJoinSimpleJoinRightMatchesJava(t *testing.T) {
	dbJoinSetHandler(dbJoinAllFieldsWhereBigintHandler)
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinExtraS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	dbSchema := dbJoinMyTestTableSchema(t)
	defer db.Close()

	provider, err := NewSQLHistoricalProvider(db, dbSchema,
		"select mybigint, myint, myvarchar, mychar, mybool, mynumeric, mydecimal, mydouble, myreal from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any {
			return request.Trigger.Get("ID").Any()
		})
	if err != nil {
		t.Fatal(err)
	}
	s0Stream := From[dbJoinExtraS0](env, "SupportBean_S0")
	historical := FromHistoricalOn[map[string]any](env, "MyDBJoinRight", "SupportBean_S0", dbSchema, provider)
	query := Join(historical, s0Stream, OnEqual(
		Field[map[string]any, int64]("mybigint"),
		Field[dbJoinExtraS0, int]("ID"),
	)).Select(
		SelectLeft("mybigint", Field[map[string]any, int64]("mybigint")),
		SelectLeft("myint", Field[map[string]any, int]("myint")),
		SelectLeft("myvarchar", Field[map[string]any, string]("myvarchar")),
		SelectLeft("mychar", Field[map[string]any, string]("mychar")),
		SelectLeft("mybool", Field[map[string]any, bool]("mybool")),
	).Query(StatementName("s0-join-right"))
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
	if err := engine.SendEvent(context.Background(), dbJoinExtraS0{ID: 1}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d: %#v", len(rows), rows)
	}
	dbJoinAssertRow(t, rows[0], map[string]any{
		"mybigint": int64(1), "myint": 10,
		"myvarchar": "A", "mychar": "Z", "mybool": true,
	})
}

func TestDatabaseJoinStreamNamesAndRenameMatchesJava(t *testing.T) {
	dbJoinSetHandler(func(_ string, args []any) (dbJoinSQLResult, error) {
		cols := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
		result := dbJoinSQLResult{columns: cols}
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
			result.rows = append(result.rows, []driver.Value{
				row.mybigint, row.myint, row.myvarchar, row.mychar,
				row.mybool, nil, row.mydecimal, row.mydouble, row.myreal,
			})
		}
		return result, nil
	})
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinExtraS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	aliasSchema, err := NewMapSchema("AliasedSQL", []FieldSpec{
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
	provider, err := NewSQLHistoricalProvider(db, aliasSchema,
		"select mybigint as a, myint as b, myvarchar as c, mychar as d, mybool as e, mynumeric as f, mydecimal as g, mydouble as h, myreal as i from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any {
			return request.Trigger.Get("ID").Any()
		})
	if err != nil {
		t.Fatal(err)
	}
	s0Stream := From[dbJoinExtraS0](env, "SupportBean_S0")
	historical := FromHistoricalOn[map[string]any](env, "MyDBRename", "SupportBean_S0", aliasSchema, provider)
	query := Join(s0Stream, historical, OnEqual(
		Field[dbJoinExtraS0, int]("ID"),
		Field[map[string]any, int64]("a"),
	)).Select(
		SelectRight("mybigint", Field[map[string]any, int64]("a")),
		SelectRight("myint", Field[map[string]any, int]("b")),
		SelectRight("myvarchar", Field[map[string]any, string]("c")),
		SelectRight("mychar", Field[map[string]any, string]("d")),
		SelectRight("mybool", Field[map[string]any, bool]("e")),
	).Query(StatementName("s0-rename"))
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
	if err := engine.SendEvent(context.Background(), dbJoinExtraS0{ID: 1}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d: %#v", len(rows), rows)
	}
	dbJoinAssertRow(t, rows[0], map[string]any{
		"mybigint": int64(1), "myint": 10,
		"myvarchar": "A", "mychar": "Z", "mybool": true,
	})
}

func TestDatabaseJoinPropertyResolutionMatchesJava(t *testing.T) {
	dbJoinSetHandler(dbJoinAllFieldsWhereBigintHandler)
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinExtraComplexProps](env, "SupportBeanComplexProps"); err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	dbSchema := dbJoinMyTestTableSchema(t)
	defer db.Close()
	provider, err := NewSQLHistoricalProvider(db, dbSchema,
		"select mybigint, myint, myvarchar, mychar, mybool, mynumeric, mydecimal, mydouble, myreal from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any {
			arr := request.Trigger.Get("ArrayProperty").Any().([10]int)
			if len(arr) > 0 {
				return arr[0]
			}
			return 0
		})
	if err != nil {
		t.Fatal(err)
	}
	complexStream := From[dbJoinExtraComplexProps](env, "SupportBeanComplexProps")
	historical := FromHistoricalOn[map[string]any](env, "MyDBPropRes", "SupportBeanComplexProps", dbSchema, provider)
	query := Join(historical, complexStream, OnEqual(
		Field[map[string]any, int64]("mybigint"),
		ArrayAt[int](Field[dbJoinExtraComplexProps, [10]int]("ArrayProperty"), Literal(0)),
	)).Select(
		SelectLeft("mybigint", Field[map[string]any, int64]("mybigint")),
		SelectLeft("myint", Field[map[string]any, int]("myint")),
		SelectLeft("myvarchar", Field[map[string]any, string]("myvarchar")),
	).Query(StatementName("s0-prop-resolution"))
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
	complexProps := dbJoinExtraComplexProps{ArrayProperty: [10]int{10, 20, 30}}
	if err := engine.SendEvent(context.Background(), complexProps); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d: %#v", len(rows), rows)
	}
	dbJoinAssertRow(t, rows[0], map[string]any{
		"mybigint": int64(10), "myint": 100, "myvarchar": "J",
	})
}

func TestDatabaseJoin2HistoricalStarMatchesJava(t *testing.T) {
	dbJoinSetHandler(dbJoinColumnsWhereBigintHandler)
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	db := dbJoinOpenDB(t)
	defer db.Close()
	myintSchema, err := NewMapSchema("HistMyInt", []FieldSpec{
		FieldDef("mybigint", reflect.TypeOf(int64(0))),
		FieldDef("myint", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	myvarcharSchema, err := NewMapSchema("HistMyVarchar", []FieldSpec{
		FieldDef("mybigint", reflect.TypeOf(int64(0))),
		FieldDef("myvarchar", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	prov1, err := NewSQLHistoricalProvider(db, myintSchema,
		"select mybigint, myint from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any {
			return request.Trigger.Get("intPrimitive").Any()
		})
	if err != nil {
		t.Fatal(err)
	}
	prov2, err := NewSQLHistoricalProvider(db, myvarcharSchema,
		"select mybigint, myvarchar from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any {
			return request.Trigger.Get("intPrimitive").Any()
		})
	if err != nil {
		t.Fatal(err)
	}
	supportStream := From[dbJoinSupportBean](env, "SupportBean").Window(KeepAll())
	hist1 := FromHistoricalOn[map[string]any](env, "MyDBHist1", "SupportBean", myintSchema, prov1)
	hist2 := FromHistoricalOn[map[string]any](env, "MyDBHist2", "SupportBean", myvarcharSchema, prov2)
	query := JoinMany(
		JoinSource(supportStream),
		JoinSource(hist1),
		JoinSource(hist2),
	).On(
		OnSourcesEqual(0, Field[dbJoinSupportBean, int]("intPrimitive"),
			1, Field[map[string]any, int64]("mybigint")),
		OnSourcesEqual(0, Field[dbJoinSupportBean, int]("intPrimitive"),
			2, Field[map[string]any, int64]("mybigint")),
	).Select(
		SelectFrom(0, "intPrimitive", Field[dbJoinSupportBean, int]("intPrimitive")),
		SelectFrom(1, "myint", Field[map[string]any, int]("myint")),
		SelectFrom(2, "myvarchar", Field[map[string]any, string]("myvarchar")),
	).Query(StatementName("s0-2hist-star"))
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
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{IntPrimitive: 6}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after int=6, got %d: %#v", len(rows), rows)
	}
	dbJoinAssertRow(t, rows[0], map[string]any{
		"intPrimitive": 6, "myint": 60, "myvarchar": "F",
	})
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{IntPrimitive: 9}); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows total, got %d", len(rows))
	}
	dbJoinAssertRow(t, rows[1], map[string]any{
		"intPrimitive": 9, "myint": 90, "myvarchar": "I",
	})
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{IntPrimitive: 20}); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if len(rows) != 2 {
		t.Fatalf("expected still 2 rows (no match for 20), got %d", len(rows))
	}
}

// dbJoinExtractColumns parses the column names from a SELECT statement's
// column list (between SELECT and FROM). It handles "col as alias" by
// returning the alias.
func dbJoinExtractColumns(stmt string) []string {
	lower := strings.ToLower(stmt)
	start := strings.Index(lower, "select")
	end := strings.Index(lower, "from")
	if start < 0 || end < 0 || start+6 >= end {
		return dbJoinAllFieldNames
	}
	part := stmt[start+6 : end]
	rawCols := strings.Split(part, ",")
	var cols []string
	for _, c := range rawCols {
		c = strings.TrimSpace(c)
		if asIdx := strings.Index(strings.ToLower(c), " as "); asIdx >= 0 {
			c = strings.TrimSpace(c[asIdx+4:])
		}
		cols = append(cols, c)
	}
	return cols
}

// dbJoinColumnsWhereBigintHandler returns only the columns requested in the
// SQL statement, filtered by the mybigint parameter. This mirrors the real
// MySQL behaviour where the result set only contains selected columns.
func dbJoinColumnsWhereBigintHandler(stmt string, args []any) (dbJoinSQLResult, error) {
	cols := dbJoinExtractColumns(stmt)
	result := dbJoinSQLResult{columns: cols}
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
		result.rows = append(result.rows, dbJoinRowToValues(row, cols))
	}
	return result, nil
}
