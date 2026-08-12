package esper

import (
	"context"
	"database/sql/driver"
	"math"
	"reflect"
	"strconv"
	"testing"
)

// dbJoinPerfS0 mirrors Java SupportBean_S0 with p00/p01/p02 for in-keyword tests.
type dbJoinPerfS0 struct {
	ID  int
	P00 string
	P01 string
	P02 string
}

// dbJoinPerfRange mirrors Java SupportBeanRange used for range-index tests.
type dbJoinPerfRange struct {
	Key            string
	RangeStart     int
	RangeEnd       int
	RangeStartLong int64
	RangeEndLong   int64
}

// dbJoinPerfSupportBean mirrors the subset of Java SupportBean used in perf tests.
type dbJoinPerfSupportBean struct {
	TheString     string
	IntPrimitive  int
	DoubleBoxed   float64
	ByteBoxed     int8
	BoolPrimitive bool
	IntBoxed      int
}

// dbJoinLargeRow mirrors mytesttable_large.
type dbJoinLargeRow struct {
	mycol1 string
	mycol2 string
	mycol3 int
	mycol4 int
}

// dbJoinLargeTable generates the 1000-row fixture used by Java perf tests.
var dbJoinLargeTable = func() []dbJoinLargeRow {
	rows := make([]dbJoinLargeRow, 1000)
	for i := 1; i <= 1000; i++ {
		rows[i-1] = dbJoinLargeRow{
			mycol1: strconv.Itoa(i),
			mycol2: strconv.Itoa(int(math.Round(float64(i) / 10.0))),
			mycol3: i,
			mycol4: int(math.Round(float64(i) / 10.0)),
		}
	}
	return rows
}()

func dbJoinLargeRowValues(row dbJoinLargeRow, cols []string) []driver.Value {
	values := make([]driver.Value, len(cols))
	for i, col := range cols {
		switch col {
		case "mycol1", "MYCOL1":
			values[i] = row.mycol1
		case "mycol2", "MYCOL2":
			values[i] = row.mycol2
		case "mycol3", "MYCOL3":
			values[i] = row.mycol3
		case "mycol4", "MYCOL4":
			values[i] = row.mycol4
		}
	}
	return values
}

// dbJoinLargeAllColsHandler returns all rows from mytesttable_large.
func dbJoinLargeAllColsHandler(_ string, _ []any) (dbJoinSQLResult, error) {
	cols := []string{"mycol1", "mycol2", "mycol3", "mycol4"}
	result := dbJoinSQLResult{columns: cols}
	for _, row := range dbJoinLargeTable {
		result.rows = append(result.rows, dbJoinLargeRowValues(row, cols))
	}
	return result, nil
}

// dbJoinLargeMycol3WhereMycol3Handler filters mytesttable_large by mycol3 equality.
func dbJoinLargeMycol3WhereMycol3Handler(_ string, args []any) (dbJoinSQLResult, error) {
	cols := []string{"mycol1", "mycol3"}
	result := dbJoinSQLResult{columns: cols}
	if len(args) == 0 {
		return result, nil
	}
	v, ok := dbJoinToInt64(args[0])
	if !ok {
		return result, nil
	}
	for _, row := range dbJoinLargeTable {
		if int64(row.mycol3) == v {
			result.rows = append(result.rows, dbJoinLargeRowValues(row, cols))
		}
	}
	return result, nil
}

// dbJoinLargeMycol1Mycol3Handler returns mycol1 and mycol3 for all rows.
func dbJoinLargeMycol1Mycol3Handler(_ string, _ []any) (dbJoinSQLResult, error) {
	cols := []string{"mycol1", "mycol3"}
	result := dbJoinSQLResult{columns: cols}
	for _, row := range dbJoinLargeTable {
		result.rows = append(result.rows, dbJoinLargeRowValues(row, cols))
	}
	return result, nil
}

// dbJoinLargeMycol3Mycol2Handler returns mycol3 and mycol2 for all rows.
func dbJoinLargeMycol3Mycol2Handler(_ string, _ []any) (dbJoinSQLResult, error) {
	cols := []string{"mycol3", "mycol2"}
	result := dbJoinSQLResult{columns: cols}
	for _, row := range dbJoinLargeTable {
		result.rows = append(result.rows, dbJoinLargeRowValues(row, cols))
	}
	return result, nil
}

// dbJoinLargeMycol3Mycol4Handler returns mycol3 and mycol4 for all rows.
func dbJoinLargeMycol3Mycol4Handler(_ string, _ []any) (dbJoinSQLResult, error) {
	cols := []string{"mycol3", "mycol4"}
	result := dbJoinSQLResult{columns: cols}
	for _, row := range dbJoinLargeTable {
		result.rows = append(result.rows, dbJoinLargeRowValues(row, cols))
	}
	return result, nil
}

// dbJoinLargeMycol1Mycol2Mycol3Handler returns mycol1, mycol2 and mycol3 for all rows.
func dbJoinLargeMycol1Mycol2Mycol3Handler(_ string, _ []any) (dbJoinSQLResult, error) {
	cols := []string{"mycol1", "mycol2", "mycol3"}
	result := dbJoinSQLResult{columns: cols}
	for _, row := range dbJoinLargeTable {
		result.rows = append(result.rows, dbJoinLargeRowValues(row, cols))
	}
	return result, nil
}

// dbJoinMyIntWhereMyIntHandler returns myint for rows where myint matches the argument.
func dbJoinMyIntWhereMyIntHandler(_ string, args []any) (dbJoinSQLResult, error) {
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
		if int64(row.myint) == v {
			result.rows = append(result.rows, []driver.Value{row.myint})
		}
	}
	return result, nil
}

// dbJoinMyIntWhereMyVarCharHandler returns myint for rows where myvarchar matches the argument.
func dbJoinMyIntWhereMyVarCharHandler(_ string, args []any) (dbJoinSQLResult, error) {
	cols := []string{"myint"}
	result := dbJoinSQLResult{columns: cols}
	if len(args) == 0 {
		return result, nil
	}
	want, ok := args[0].(string)
	if !ok {
		return result, nil
	}
	for _, row := range dbJoinMyTestTable {
		if row.myvarchar == want {
			result.rows = append(result.rows, []driver.Value{row.myint})
		}
	}
	return result, nil
}

func dbJoinLargeSchema(t *testing.T, name string, fields ...FieldSpec) Schema {
	t.Helper()
	schema, err := NewMapSchema(name, fields)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func TestDatabaseJoinOptionLowercaseMatchesJava(t *testing.T) {
	// Java EPLDatabaseJoinOptionLowercase: MyDBLowerCase lowercases column names
	// and the metadata SQL makes myint come back as a String.
	dbJoinSetHandler(dbJoinMyIntWhereMyIntHandler)
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	db := dbJoinOpenDB(t)
	defer db.Close()

	schema := dbJoinLargeSchema(t, "HistOptLowercase", FieldDef("myint", reflect.TypeOf("")))
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select myint from mytesttable where ? = myint",
		SQLHistoricalProviderOptions{
			Arguments: []func(HistoricalRequest) any{
				func(request HistoricalRequest) any { return request.Trigger.Get("intPrimitive").Any() },
			},
			ColumnCase: SQLColumnCaseLower,
		})
	if err != nil {
		t.Fatal(err)
	}

	hist := FromHistoricalOn[map[string]any](env, "MyDBLowerCase", "SupportBean", schema, provider)
	plan, err := env.Build(Select(hist,
		Alias("myint", Field[map[string]any, string]("myint")),
	).Query(StatementName("s0-opt-lowercase")))
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
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if got, want := rows[0]["myint"], "10"; got != want {
		t.Fatalf("expected myint=%q, got %v (%T)", want, got, got)
	}

	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{IntPrimitive: 80}); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if got, want := rows[len(rows)-1]["myint"], "80"; got != want {
		t.Fatalf("expected myint=%q, got %v (%T)", want, got, got)
	}
}

func TestDatabaseJoinOptionUppercaseMatchesJava(t *testing.T) {
	// Java EPLDatabaseJoinOptionUppercase: MyDBUpperCase uppercases column names
	// and the metadata SQL makes MYINT come back as an Integer.
	dbJoinSetHandler(dbJoinMyIntWhereMyVarCharHandler)
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	db := dbJoinOpenDB(t)
	defer db.Close()

	schema := dbJoinLargeSchema(t, "HistOptUppercase", FieldDef("MYINT", reflect.TypeOf(0)))
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select MYINT from mytesttable where ? = myvarchar",
		SQLHistoricalProviderOptions{
			Arguments: []func(HistoricalRequest) any{
				func(request HistoricalRequest) any { return request.Trigger.Get("TheString").Any() },
			},
			ColumnCase: SQLColumnCaseUpper,
		})
	if err != nil {
		t.Fatal(err)
	}

	hist := FromHistoricalOn[map[string]any](env, "MyDBUpperCase", "SupportBean", schema, provider)
	plan, err := env.Build(Select(hist,
		Alias("MYINT", Field[map[string]any, int]("MYINT")),
	).Query(StatementName("s0-opt-uppercase")))
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

	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{TheString: "A"}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if got, want := rows[0]["MYINT"], 10; got != want {
		t.Fatalf("expected MYINT=%v, got %v (%T)", want, got, got)
	}

	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{TheString: "H"}); err != nil {
		t.Fatal(err)
	}
	rows = getRows()
	if got, want := rows[len(rows)-1]["MYINT"], 80; got != want {
		t.Fatalf("expected MYINT=%v, got %v (%T)", want, got, got)
	}
}

func TestDatabaseNoJoinIteratePerfMatchesJava(t *testing.T) {
	// Java EPLDatabaseNoJoinIteratePerf: variable-driven SQL query with between.
	// This is a correctness port; the 10000-iteration timing assertion is not
	// meaningful in a fake driver and is intentionally dropped.
	dbJoinSetHandler(dbJoinMyBigIntBoolHandler)
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	db := dbJoinOpenDB(t)
	defer db.Close()

	if err := env.RegisterVariable("queryvar_bool", true); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("lower", 0); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("upper", 0); err != nil {
		t.Fatal(err)
	}

	schema := dbJoinLargeSchema(t, "HistNoJoinPerf",
		FieldDef("mybigint", reflect.TypeOf(int64(0))),
		FieldDef("mybool", reflect.TypeOf(false)),
	)
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select mybigint, mybool from mytesttable where ? = mybool and myint between ? and ? order by mybigint",
		SQLHistoricalProviderOptions{
			Arguments: []func(HistoricalRequest) any{
				func(request HistoricalRequest) any { return request.Variables["queryvar_bool"].Any() },
				func(request HistoricalRequest) any { return request.Variables["lower"].Any() },
				func(request HistoricalRequest) any { return request.Variables["upper"].Any() },
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	hist := FromHistorical[map[string]any](env, "MyDBNoJoinPerf", schema, provider)
	plan, err := env.Build(Select(hist,
		Alias("mybigint", Field[map[string]any, int64]("mybigint")),
		Alias("mybool", Field[map[string]any, bool]("mybool")),
	).Query(StatementName("s0-nojoin-iter")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	intBoxed := 60
	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{BoolPrimitive: true, IntPrimitive: 20, IntBoxed: &intBoxed}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SetVariable(context.Background(), "queryvar_bool", true); err != nil {
		t.Fatal(err)
	}
	if err := engine.SetVariable(context.Background(), "lower", 20); err != nil {
		t.Fatal(err)
	}
	if err := engine.SetVariable(context.Background(), "upper", 60); err != nil {
		t.Fatal(err)
	}

	prepared, err := engine.PrepareFireAndForget(plan)
	if err != nil {
		t.Fatal(err)
	}
	// Execute multiple times to mirror Java's 10000-iteration loop; we only
	// assert correctness, not timing.
	for i := 0; i < 10; i++ {
		result, err := prepared.Execute(context.Background())
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		results := result.Results()
		if len(results) != 1 {
			t.Fatalf("iteration %d: expected 1 result, got %d", i, len(results))
		}
		row, ok := results[0].Row()
		if !ok {
			t.Fatalf("iteration %d: expected row result, got %#v", i, results[0])
		}
		if got, want := row.Get("mybigint").Any(), int64(4); got != want {
			t.Fatalf("iteration %d: expected mybigint=%v, got %v", i, want, got)
		}
		if got, want := row.Get("mybool").Any(), true; got != want {
			t.Fatalf("iteration %d: expected mybool=%v, got %v", i, want, got)
		}
	}
}

func TestDatabaseQueryResultCacheMatchesJava(t *testing.T) {
	// Java EPLDatabaseQueryResultCache: LRU cache for SQL results driven by
	// SupportBean_S0.id. Correctness port (timing assertions dropped).
	dbJoinSetHandler(dbJoinMyIntWhereBigintHandler)
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinExtraS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()

	schema := dbJoinLargeSchema(t, "HistCacheMyInt",
		FieldDef("myint", reflect.TypeOf(0)),
	)
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select myint from mytesttable where ? = mybigint",
		SQLHistoricalProviderOptions{
			Arguments: []func(HistoricalRequest) any{
				func(request HistoricalRequest) any { return request.Trigger.Get("ID").Any() },
			},
			Cache: SQLHistoricalCacheConfig{LRUSize: 100000},
		})
	if err != nil {
		t.Fatal(err)
	}

	s0Stream := From[dbJoinExtraS0](env, "SupportBean_S0")
	hist := FromHistoricalOn[map[string]any](env, "MyDBCache", "SupportBean_S0", schema, provider)
	plan, err := env.Build(Join(s0Stream, hist).Select(
		SelectRight("myint", Field[map[string]any, int]("myint")),
	).Query(StatementName("s0-cache")))
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

	// Sequential id 1..10 -> myint 10..100
	for i := 0; i < 10; i++ {
		id := i%10 + 1
		if err := engine.SendEvent(context.Background(), dbJoinExtraS0{ID: id}); err != nil {
			t.Fatal(err)
		}
		rows := getRows()
		last := rows[len(rows)-1]
		if got, want := last["myint"], id*10; got != want {
			t.Fatalf("event %d: expected myint=%d, got %v", i, want, got)
		}
	}

	// Repeat lookups to exercise cache hits
	for i := 0; i < 10; i++ {
		id := i%10 + 1
		if err := engine.SendEvent(context.Background(), dbJoinExtraS0{ID: id}); err != nil {
			t.Fatal(err)
		}
		rows := getRows()
		last := rows[len(rows)-1]
		if got, want := last["myint"], id*10; got != want {
			t.Fatalf("cache hit %d: expected myint=%d, got %v", i, want, got)
		}
	}
}

func TestDatabaseJoinPerfNoCacheMatchesJava(t *testing.T) {
	// Java EPLDatabaseJoinPerfNoCache: 5 performance sub-assertions.
	// This is a correctness port; timing assertions are dropped.
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinExtraS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()

	schema := dbJoinLargeSchema(t, "HistPerfNoCache",
		FieldDef("myint", reflect.TypeOf(0)),
	)
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select myint from mytesttable where ? = mybigint",
		SQLHistoricalProviderOptions{
			Arguments: []func(HistoricalRequest) any{
				func(request HistoricalRequest) any { return request.Trigger.Get("ID").Any() },
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	s0Stream := From[dbJoinExtraS0](env, "SupportBean_S0")
	hist := FromHistoricalOn[map[string]any](env, "MyDBPerfNoCache", "SupportBean_S0", schema, provider)
	plan, err := env.Build(Join(s0Stream, hist).Select(
		SelectRight("myint", Field[map[string]any, int]("myint")),
	).Query(StatementName("s0-perf-nocache")))
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

	for i := 0; i < 100; i++ {
		id := i%10 + 1
		if err := engine.SendEvent(context.Background(), dbJoinExtraS0{ID: id}); err != nil {
			t.Fatal(err)
		}
		rows := getRows()
		last := rows[len(rows)-1]
		if got, want := last["myint"], id*10; got != want {
			t.Fatalf("event %d: expected myint=%d, got %v", i, want, got)
		}
	}
}

func TestDatabaseJoinPerfWithCacheConstantsMatchesJava(t *testing.T) {
	// Java EPLDatabaseJoinPerfWithCache.EPLDatabaseConstants: constant where
	// filters on mytesttable_large. Correctness port (timing dropped).
	dbJoinSetHandler(dbJoinLargeMycol1Mycol3Handler)
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	db := dbJoinOpenDB(t)
	defer db.Close()

	schema := dbJoinLargeSchema(t, "HistPerfConstants",
		FieldDef("mycol1", reflect.TypeOf("")),
		FieldDef("mycol3", reflect.TypeOf(0)),
	)
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select mycol1, mycol3 from mytesttable_large",
		SQLHistoricalProviderOptions{
			Cache: SQLHistoricalCacheConfig{LRUSize: 100000},
		})
	if err != nil {
		t.Fatal(err)
	}

	sbStream := From[dbJoinSupportBean](env, "SupportBean")
	hist := FromHistorical[map[string]any](env, "MyDBPerfConstants", schema, provider)
	plan, err := env.Build(Join(sbStream, hist).Select(
		SelectRight("mycol1", Field[map[string]any, string]("mycol1")),
		SelectRight("mycol3", Field[map[string]any, int]("mycol3")),
	).Where(Equal[int](JoinField[int](1, "mycol3"), Literal(951))).
		Query(StatementName("s0-perf-constants")))
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

	if err := engine.SendEvent(context.Background(), dbJoinSupportBean{TheString: "E"}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if got, want := rows[0]["mycol1"], "951"; got != want {
		t.Fatalf("expected mycol1=%q, got %v", want, got)
	}
	if got, want := rows[0]["mycol3"], 951; got != want {
		t.Fatalf("expected mycol3=%d, got %v", want, got)
	}
}

func TestDatabaseJoinPerfWithCacheRangeIndexMatchesJava(t *testing.T) {
	// Java EPLDatabaseJoinPerfWithCache.EPLDatabaseRangeIndex: range scan on
	// mytesttable_large.mycol3 between SupportBeanRange.rangeStart and rangeEnd.
	dbJoinSetHandler(dbJoinLargeMycol1Mycol3Handler)
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinPerfRange](env, "SupportBeanRange"); err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()

	schema := dbJoinLargeSchema(t, "HistPerfRange",
		FieldDef("mycol1", reflect.TypeOf("")),
		FieldDef("mycol3", reflect.TypeOf(0)),
	)
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select mycol1, mycol3 from mytesttable_large",
		SQLHistoricalProviderOptions{
			Cache: SQLHistoricalCacheConfig{LRUSize: 100000},
		})
	if err != nil {
		t.Fatal(err)
	}

	rangeStream := From[dbJoinPerfRange](env, "SupportBeanRange")
	hist := FromHistorical[map[string]any](env, "MyDBPerfRange", schema, provider)
	plan, err := env.Build(Join(rangeStream, hist).Select(
		SelectRight("mycol3", Field[map[string]any, int]("mycol3")),
	).Where(
		Between[int](JoinField[int](1, "mycol3"), JoinField[int](0, "RangeStart"), JoinField[int](0, "RangeEnd")),
	).Query(StatementName("s0-perf-range")))
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

	if err := engine.SendEvent(context.Background(), dbJoinPerfRange{RangeStart: 10, RangeEnd: 12}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	for i, want := range []int{10, 11, 12} {
		if got := rows[i]["mycol3"]; got != want {
			t.Fatalf("row %d: expected mycol3=%d, got %v", i, want, got)
		}
	}
}

func TestDatabaseJoinPerfWithCacheKeyRangeIndexMatchesJava(t *testing.T) {
	// Java EPLDatabaseJoinPerfWithCache.EPLDatabaseKeyAndRangeIndex: equality on
	// mycol1 = key plus range scan on mycol3.
	dbJoinSetHandler(dbJoinLargeMycol1Mycol2Mycol3Handler)
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinPerfRange](env, "SupportBeanRange"); err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()

	schema := dbJoinLargeSchema(t, "HistPerfKeyRange",
		FieldDef("mycol1", reflect.TypeOf("")),
		FieldDef("mycol2", reflect.TypeOf("")),
		FieldDef("mycol3", reflect.TypeOf(0)),
	)
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select mycol1, mycol2, mycol3 from mytesttable_large",
		SQLHistoricalProviderOptions{
			Cache: SQLHistoricalCacheConfig{LRUSize: 100000},
		})
	if err != nil {
		t.Fatal(err)
	}

	rangeStream := From[dbJoinPerfRange](env, "SupportBeanRange")
	hist := FromHistorical[map[string]any](env, "MyDBPerfKeyRange", schema, provider)
	plan, err := env.Build(Join(rangeStream, hist).Select(
		SelectRight("mycol3", Field[map[string]any, int]("mycol3")),
	).Where(
		And(
			Equal[string](JoinField[string](1, "mycol1"), JoinField[string](0, "Key")),
			Between[int](JoinField[int](1, "mycol3"), JoinField[int](0, "RangeStart"), JoinField[int](0, "RangeEnd")),
		),
	).Query(StatementName("s0-perf-keyrange")))
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

	if err := engine.SendEvent(context.Background(), dbJoinPerfRange{Key: "11", RangeStart: 10, RangeEnd: 12}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if got, want := rows[0]["mycol3"], 11; got != want {
		t.Fatalf("expected mycol3=%d, got %v", want, got)
	}
}

func TestDatabaseJoinPerfWithCacheLargeResultSetMatchesJava(t *testing.T) {
	// Java EPLDatabaseJoinPerfWithCache.EPLDatabaseSelectLargeResultSet:
	// SupportBean_S0#keepall joined with mytesttable_large on id=mycol3.
	dbJoinSetHandler(dbJoinLargeMycol3Mycol2Handler)
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinExtraS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()

	schema := dbJoinLargeSchema(t, "HistPerfLargeRS",
		FieldDef("mycol3", reflect.TypeOf(0)),
		FieldDef("mycol2", reflect.TypeOf("")),
	)
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select mycol3, mycol2 from mytesttable_large",
		SQLHistoricalProviderOptions{
			Cache: SQLHistoricalCacheConfig{LRUSize: 100000},
		})
	if err != nil {
		t.Fatal(err)
	}

	s0Stream := From[dbJoinExtraS0](env, "SupportBean_S0").Window(KeepAll())
	hist := FromHistorical[map[string]any](env, "MyDBPerfLargeRS", schema, provider)
	plan, err := env.Build(Join(s0Stream, hist, OnEqual(
		JoinField[int](0, "ID"),
		JoinField[int](1, "mycol3"),
	)).Select(
		SelectLeft("id", Field[dbJoinExtraS0, int]("ID")),
		SelectRight("mycol3", Field[map[string]any, int]("mycol3")),
		SelectRight("mycol2", Field[map[string]any, string]("mycol2")),
	).Query(StatementName("s0-perf-largers")))
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

	for i := 0; i < 20; i++ {
		num := i + 1
		col2 := strconv.Itoa(int(math.Round(float64(num) / 10.0)))
		if err := engine.SendEvent(context.Background(), dbJoinExtraS0{ID: num}); err != nil {
			t.Fatal(err)
		}
		rows := getRows()
		last := rows[len(rows)-1]
		if got, want := last["id"], num; got != want {
			t.Fatalf("event %d: expected id=%d, got %v", i, want, got)
		}
		if got, want := last["mycol3"], num; got != want {
			t.Fatalf("event %d: expected mycol3=%d, got %v", i, want, got)
		}
		if got, want := last["mycol2"], col2; got != want {
			t.Fatalf("event %d: expected mycol2=%q, got %v", i, want, got)
		}
	}
}

func TestDatabaseJoinPerfWithCacheLargeResultSetCoercionMatchesJava(t *testing.T) {
	// Java EPLDatabaseJoinPerfWithCache.EPLDatabaseSelectLargeResultSetCoercion:
	// doubleBoxed/byteBoxed match mycol3/mycol4 from mytesttable_large.
	dbJoinSetHandler(dbJoinLargeMycol3Mycol4Handler)
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinPerfSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()

	schema := dbJoinLargeSchema(t, "HistPerfLargeRSCoerce",
		FieldDef("mycol3", reflect.TypeOf(0)),
		FieldDef("mycol4", reflect.TypeOf(0)),
	)
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select mycol3, mycol4 from mytesttable_large",
		SQLHistoricalProviderOptions{
			Cache: SQLHistoricalCacheConfig{LRUSize: 100000},
		})
	if err != nil {
		t.Fatal(err)
	}

	sbStream := From[dbJoinPerfSupportBean](env, "SupportBean").Window(KeepAll())
	hist := FromHistorical[map[string]any](env, "MyDBPerfLargeRSCoerce", schema, provider)
	plan, err := env.Build(Join(hist, sbStream, OnEqual(
		JoinField[int](0, "mycol3"),
		JoinField[float64](1, "DoubleBoxed"),
	), OnEqual(
		JoinField[int](0, "mycol4"),
		JoinField[int](1, "ByteBoxed"),
	)).Select(
		SelectLeft("mycol3", Field[map[string]any, int]("mycol3")),
		SelectLeft("mycol4", Field[map[string]any, int]("mycol4")),
		SelectRight("theString", Field[dbJoinPerfSupportBean, string]("TheString")),
	).Query(StatementName("s0-perf-largers-coerce")))
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

	for i := 0; i < 20; i++ {
		if err := engine.SendEvent(context.Background(), dbJoinPerfSupportBean{
			TheString:   "E" + strconv.Itoa(i),
			DoubleBoxed: 100,
			ByteBoxed:   10,
		}); err != nil {
			t.Fatal(err)
		}
		rows := getRows()
		last := rows[len(rows)-1]
		if got, want := last["theString"], "E"+strconv.Itoa(i); got != want {
			t.Fatalf("event %d: expected theString=%q, got %v", i, want, got)
		}
		if got, want := last["mycol3"], 100; got != want {
			t.Fatalf("event %d: expected mycol3=%d, got %v", i, want, got)
		}
		if got, want := last["mycol4"], 10; got != want {
			t.Fatalf("event %d: expected mycol4=%d, got %v", i, want, got)
		}
	}
}

func TestDatabaseJoinPerfWithCache2StreamOuterJoinMatchesJava(t *testing.T) {
	// Java EPLDatabaseJoinPerfWithCache.EPLDatabase2StreamOuterJoin:
	// sql right outer join SupportBean on theString = mycol1.
	dbJoinSetHandler(dbJoinLargeMycol1Mycol3Handler)
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinPerfSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()

	schema := dbJoinLargeSchema(t, "HistPerfOuter",
		FieldDef("mycol1", reflect.TypeOf("")),
		FieldDef("mycol3", reflect.TypeOf(0)),
	)
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select mycol1, mycol3 from mytesttable_large",
		SQLHistoricalProviderOptions{
			Cache: SQLHistoricalCacheConfig{LRUSize: 100000},
		})
	if err != nil {
		t.Fatal(err)
	}

	sbStream := From[dbJoinPerfSupportBean](env, "SupportBean")
	hist := FromHistorical[map[string]any](env, "MyDBPerfOuter", schema, provider)
	plan, err := env.Build(Join(hist, sbStream, OnEqual(
		JoinField[string](1, "TheString"),
		JoinField[string](0, "mycol1"),
	)).RightOuter().Select(
		SelectRight("theString", Field[dbJoinPerfSupportBean, string]("TheString")),
		SelectLeft("mycol3", Field[map[string]any, int]("mycol3")),
		SelectLeft("mycol1", Field[map[string]any, string]("mycol1")),
	).Query(StatementName("s0-perf-outer")))
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

	for i := 0; i < 20; i++ {
		if err := engine.SendEvent(context.Background(), dbJoinPerfSupportBean{TheString: "50"}); err != nil {
			t.Fatal(err)
		}
		rows := getRows()
		last := rows[len(rows)-1]
		if got, want := last["theString"], "50"; got != want {
			t.Fatalf("event %d: expected theString=%q, got %v", i, want, got)
		}
		if got, want := last["mycol3"], 50; got != want {
			t.Fatalf("event %d: expected mycol3=%d, got %v", i, want, got)
		}
		if got, want := last["mycol1"], "50"; got != want {
			t.Fatalf("event %d: expected mycol1=%q, got %v", i, want, got)
		}
	}

	// no matching on-clause: SQL side null, SupportBean side preserved
	if err := engine.SendEvent(context.Background(), dbJoinPerfSupportBean{TheString: "-1"}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	last := rows[len(rows)-1]
	if got, want := last["theString"], "-1"; got != want {
		t.Fatalf("expected theString=%q, got %v", want, got)
	}
	if last["mycol3"] != nil || last["mycol1"] != nil {
		t.Fatalf("expected null SQL side, got mycol3=%v mycol1=%v", last["mycol3"], last["mycol1"])
	}
}

func TestDatabaseJoinPerfWithCacheOuterJoinPlusWhereMatchesJava(t *testing.T) {
	// Java EPLDatabaseJoinPerfWithCache.EPLDatabaseOuterJoinPlusWhere:
	// right outer join plus where mycol3 = intPrimitive.
	dbJoinSetHandler(dbJoinLargeMycol1Mycol3Handler)
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinPerfSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()

	schema := dbJoinLargeSchema(t, "HistPerfOuterWhere",
		FieldDef("mycol1", reflect.TypeOf("")),
		FieldDef("mycol3", reflect.TypeOf(0)),
	)
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select mycol1, mycol3 from mytesttable_large",
		SQLHistoricalProviderOptions{
			Cache: SQLHistoricalCacheConfig{LRUSize: 100000},
		})
	if err != nil {
		t.Fatal(err)
	}

	sbStream := From[dbJoinPerfSupportBean](env, "SupportBean")
	hist := FromHistorical[map[string]any](env, "MyDBPerfOuterWhere", schema, provider)
	plan, err := env.Build(Join(hist, sbStream, OnEqual(
		JoinField[string](1, "TheString"),
		JoinField[string](0, "mycol1"),
	)).RightOuter().Select(
		SelectRight("theString", Field[dbJoinPerfSupportBean, string]("TheString")),
		SelectLeft("mycol3", Field[map[string]any, int]("mycol3")),
	).Where(Equal[int](JoinField[int](1, "IntPrimitive"), JoinField[int](0, "mycol3"))).
		Query(StatementName("s0-perf-outer-where")))
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

	for i := 0; i < 20; i++ {
		if err := engine.SendEvent(context.Background(), dbJoinPerfSupportBean{TheString: "50", IntPrimitive: 50}); err != nil {
			t.Fatal(err)
		}
		rows := getRows()
		last := rows[len(rows)-1]
		if got, want := last["theString"], "50"; got != want {
			t.Fatalf("event %d: expected theString=%q, got %v", i, want, got)
		}
		if got, want := last["mycol3"], 50; got != want {
			t.Fatalf("event %d: expected mycol3=%d, got %v", i, want, got)
		}
	}

	// matching on-clause but not matching where -> no output
	if err := engine.SendEvent(context.Background(), dbJoinPerfSupportBean{TheString: "50", IntPrimitive: 49}); err != nil {
		t.Fatal(err)
	}
	rows := getRows()
	last := rows[len(rows)-1]
	if got, want := last["theString"], "50"; got != want {
		t.Fatalf("expected previous output retained, got theString=%v", got)
	}
}

func TestDatabaseJoinPerfWithCacheInKeywordSingleIndexMatchesJava(t *testing.T) {
	// Java EPLDatabaseJoinPerfWithCache.EPLDatabaseInKeywordSingleIndex:
	// mycol1 in (p00, p01, p02) with p02 = "815".
	dbJoinSetHandler(dbJoinLargeMycol1Mycol3Handler)
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinPerfS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()

	schema := dbJoinLargeSchema(t, "HistPerfInSingle",
		FieldDef("mycol1", reflect.TypeOf("")),
		FieldDef("mycol3", reflect.TypeOf(0)),
	)
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select mycol1, mycol3 from mytesttable_large",
		SQLHistoricalProviderOptions{
			Cache: SQLHistoricalCacheConfig{LRUSize: 100000},
		})
	if err != nil {
		t.Fatal(err)
	}

	s0Stream := From[dbJoinPerfS0](env, "SupportBean_S0")
	hist := FromHistorical[map[string]any](env, "MyDBPerfInSingle", schema, provider)
	plan, err := env.Build(Join(s0Stream, hist).Select(
		SelectRight("mycol3", Field[map[string]any, int]("mycol3")),
	).Where(
		In[string](JoinField[string](1, "mycol1"), JoinField[string](0, "P00"), JoinField[string](0, "P01"), JoinField[string](0, "P02")),
	).Query(StatementName("s0-perf-insingle")))
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

	for i := 0; i < 20; i++ {
		if err := engine.SendEvent(context.Background(), dbJoinPerfS0{P00: "x", P01: "y", P02: "815"}); err != nil {
			t.Fatal(err)
		}
		rows := getRows()
		last := rows[len(rows)-1]
		if got, want := last["mycol3"], 815; got != want {
			t.Fatalf("event %d: expected mycol3=%d, got %v", i, want, got)
		}
	}
}

func TestDatabaseJoinPerfWithCacheInKeywordMultiIndexMatchesJava(t *testing.T) {
	// Java EPLDatabaseJoinPerfWithCache.EPLDatabaseInKeywordMultiIndex:
	// p00 in (mycol2, mycol1) with p00 = "815".
	dbJoinSetHandler(dbJoinLargeMycol1Mycol2Mycol3Handler)
	env := NewEnvironment()
	if _, err := RegisterStruct[dbJoinPerfS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()

	schema := dbJoinLargeSchema(t, "HistPerfInMulti",
		FieldDef("mycol1", reflect.TypeOf("")),
		FieldDef("mycol2", reflect.TypeOf("")),
		FieldDef("mycol3", reflect.TypeOf(0)),
	)
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select mycol1, mycol2, mycol3 from mytesttable_large",
		SQLHistoricalProviderOptions{
			Cache: SQLHistoricalCacheConfig{LRUSize: 100000},
		})
	if err != nil {
		t.Fatal(err)
	}

	s0Stream := From[dbJoinPerfS0](env, "SupportBean_S0")
	hist := FromHistorical[map[string]any](env, "MyDBPerfInMulti", schema, provider)
	plan, err := env.Build(Join(s0Stream, hist).Select(
		SelectRight("mycol3", Field[map[string]any, int]("mycol3")),
	).Where(
		In[string](JoinField[string](0, "P00"), JoinField[string](1, "mycol2"), JoinField[string](1, "mycol1")),
	).Query(StatementName("s0-perf-inmulti")))
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

	for i := 0; i < 20; i++ {
		if err := engine.SendEvent(context.Background(), dbJoinPerfS0{P00: "815"}); err != nil {
			t.Fatal(err)
		}
		rows := getRows()
		last := rows[len(rows)-1]
		if got, want := last["mycol3"], 815; got != want {
			t.Fatalf("event %d: expected mycol3=%d, got %v", i, want, got)
		}
	}
}
