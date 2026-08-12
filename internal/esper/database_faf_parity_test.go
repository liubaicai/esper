package esper

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// databaseFAFOutputRow is the target representation used by the SQLROW
// conversion case. The SQL column names deliberately do not match the target
// event fields, just as they do not in Esper's SupportSQLOutputRowConversion.
type databaseFAFOutputRow struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type databaseFAFSubqueryEvent struct {
	IntPrimitive int `esper:"intPrimitive"`
}

type sqlFAFColumn struct {
	name     string
	typeName string
	scanType reflect.Type
}

type sqlFAFQueryResult struct {
	columns []sqlFAFColumn
	rows    [][]driver.Value
}

type sqlFAFHandler func(string, []any) (sqlFAFQueryResult, error)

var sqlFAFFixture = struct {
	sync.RWMutex
	handler      sqlFAFHandler
	prepareCalls int
	queryCalls   int
}{}

var sqlFAFDriverOnce sync.Once

type sqlFAFDriver struct{}

func (sqlFAFDriver) Open(string) (driver.Conn, error) { return sqlFAFConn{}, nil }

type sqlFAFConn struct{}

func (sqlFAFConn) Prepare(statement string) (driver.Stmt, error) {
	sqlFAFFixture.Lock()
	sqlFAFFixture.prepareCalls++
	sqlFAFFixture.Unlock()
	return sqlFAFStmt{statement: statement}, nil
}

func (sqlFAFConn) Close() error              { return nil }
func (sqlFAFConn) Begin() (driver.Tx, error) { return sqlFAFTx{}, nil }

func (sqlFAFConn) QueryContext(ctx context.Context, statement string, args []driver.NamedValue) (driver.Rows, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	return sqlFAFQuery(statement, sqlFAFNamedValues(args))
}

type sqlFAFStmt struct {
	statement string
}

func (s sqlFAFStmt) Close() error  { return nil }
func (s sqlFAFStmt) NumInput() int { return -1 }
func (s sqlFAFStmt) Exec([]driver.Value) (driver.Result, error) {
	return driver.RowsAffected(0), nil
}

func (s sqlFAFStmt) Query(args []driver.Value) (driver.Rows, error) {
	values := make([]any, len(args))
	for index, value := range args {
		values[index] = value
	}
	return sqlFAFQuery(s.statement, values)
}

func (s sqlFAFStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	return sqlFAFQuery(s.statement, sqlFAFNamedValues(args))
}

type sqlFAFTx struct{}

func (sqlFAFTx) Commit() error   { return nil }
func (sqlFAFTx) Rollback() error { return nil }

type sqlFAFRows struct {
	result sqlFAFQueryResult
	index  int
}

func (r *sqlFAFRows) Columns() []string {
	columns := make([]string, len(r.result.columns))
	for index, column := range r.result.columns {
		columns[index] = column.name
	}
	return columns
}

func (r *sqlFAFRows) Close() error { return nil }

func (r *sqlFAFRows) ColumnTypeDatabaseTypeName(index int) string {
	return r.result.columns[index].typeName
}

func (r *sqlFAFRows) ColumnTypeScanType(index int) reflect.Type {
	return r.result.columns[index].scanType
}

func (r *sqlFAFRows) Next(dest []driver.Value) error {
	if r.index >= len(r.result.rows) {
		return io.EOF
	}
	row := r.result.rows[r.index]
	r.index++
	if len(row) != len(dest) {
		return fmt.Errorf("fixture row has %d values, want %d", len(row), len(dest))
	}
	copy(dest, row)
	return nil
}

func sqlFAFQuery(statement string, args []any) (driver.Rows, error) {
	sqlFAFFixture.Lock()
	sqlFAFFixture.queryCalls++
	handler := sqlFAFFixture.handler
	sqlFAFFixture.Unlock()
	if handler == nil {
		return nil, errors.New("SQL FAF fixture handler is not configured")
	}
	result, err := handler(statement, args)
	if err != nil {
		return nil, err
	}
	return &sqlFAFRows{result: result}, nil
}

func sqlFAFNamedValues(values []driver.NamedValue) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value.Value
	}
	return result
}

func openSQLFAFDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlFAFDriverOnce.Do(func() {
		sql.Register("esper-fixture-database-faf", sqlFAFDriver{})
	})
	db, err := sql.Open("esper-fixture-database-faf", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func configureSQLFAF(t *testing.T, handler sqlFAFHandler) {
	t.Helper()
	sqlFAFFixture.Lock()
	sqlFAFFixture.handler = handler
	sqlFAFFixture.prepareCalls = 0
	sqlFAFFixture.queryCalls = 0
	sqlFAFFixture.Unlock()
	t.Cleanup(func() {
		sqlFAFFixture.Lock()
		sqlFAFFixture.handler = nil
		sqlFAFFixture.Unlock()
	})
}

func sqlFAFCounts() (int, int) {
	sqlFAFFixture.RLock()
	defer sqlFAFFixture.RUnlock()
	return sqlFAFFixture.prepareCalls, sqlFAFFixture.queryCalls
}

func sqlFAFColumns(names ...string) []sqlFAFColumn {
	columns := make([]sqlFAFColumn, len(names))
	for index, name := range names {
		column := sqlFAFColumn{name: name, typeName: "VARCHAR", scanType: reflect.TypeOf("")}
		switch strings.ToLower(name) {
		case "myint", "myintturnedboolean", "intprimitive":
			column.typeName = "BIGINT"
			column.scanType = reflect.TypeOf(int64(0))
		case "enabled", "mybool":
			column.typeName = "BOOLEAN"
			column.scanType = reflect.TypeOf(false)
		}
		columns[index] = column
	}
	return columns
}

func sqlFAFMapSchema(t *testing.T, name string, fields ...FieldSpec) Schema {
	t.Helper()
	schema, err := NewMapSchema(name, fields)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func sqlFAFInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case uint:
		return int(typed)
	case uint64:
		return int(typed)
	default:
		return -1
	}
}

func TestSQLHistoricalFireAndForgetSimpleMatchesJava(t *testing.T) {
	db := openSQLFAFDB(t)
	configureSQLFAF(t, func(statement string, args []any) (sqlFAFQueryResult, error) {
		if !strings.Contains(statement, "mytesttable") || len(args) != 0 {
			return sqlFAFQueryResult{}, fmt.Errorf("unexpected SQL=%q args=%#v", statement, args)
		}
		return sqlFAFQueryResult{
			columns: sqlFAFColumns("myint"),
			rows:    [][]driver.Value{{int64(10)}},
		}, nil
	})
	env := NewEnvironment()
	schema := sqlFAFMapSchema(t, "SQLFAFSimple", FieldDef("myint", reflect.TypeOf(0)))
	provider, err := NewSQLHistoricalProvider(db, schema, "select myint from mytesttable where myint between 5 and 15")
	if err != nil {
		t.Fatal(err)
	}
	source := FromHistorical[map[string]any](env, "MyDBPlain", schema, provider)
	plan, err := env.Build(Select(source,
		Alias("myint", Field[map[string]any, int]("myint")),
	).Query(StatementName("sql-faf-simple")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewEngine(env).ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 1 || result.Results()[0].Get("myint").Any() != 10 {
		t.Fatalf("SQL FAF simple result = %#v", result.Results())
	}
}

func TestSQLHistoricalFireAndForgetHooksMatchesJava(t *testing.T) {
	db := openSQLFAFDB(t)
	env := NewEnvironment()
	booleanSchema := sqlFAFMapSchema(t, "SQLFAFBoolean", FieldDef("myintTurnedBoolean", reflect.TypeOf(false)))
	configureSQLFAF(t, func(statement string, args []any) (sqlFAFQueryResult, error) {
		return sqlFAFQueryResult{columns: sqlFAFColumns("myintTurnedBoolean"), rows: [][]driver.Value{{int64(50)}}}, nil
	})
	booleanProvider, err := NewSQLHistoricalProviderWithOptions(db, booleanSchema,
		"select myint as myintTurnedBoolean from mytesttable where myint = 50",
		SQLHistoricalProviderOptions{
			ValueConverter: func(column SQLHistoricalColumnMetadata, value any, target reflect.Type) (any, error) {
				if column.Name != "myintTurnedBoolean" || target.Kind() != reflect.Bool {
					return nil, fmt.Errorf("unexpected SQL column hook metadata=%#v target=%v", column, target)
				}
				return sqlFAFInt(value) >= 50, nil
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	booleanPlan, err := env.Build(Select(FromHistorical[map[string]any](env, "MyDBPooled", booleanSchema, booleanProvider),
		Alias("myintTurnedBoolean", Field[map[string]any, bool]("myintTurnedBoolean")),
	).Query(StatementName("sql-faf-column-hook")))
	if err != nil {
		t.Fatal(err)
	}
	booleanResult, err := NewEngine(env).ExecuteFireAndForget(context.Background(), booleanPlan)
	if err != nil || len(booleanResult.Results()) != 1 || booleanResult.Results()[0].Get("myintTurnedBoolean").Any() != true {
		t.Fatalf("SQL FAF column hook result = %#v, err=%v", booleanResult.Results(), err)
	}

	outputSchema, err := RegisterStruct[databaseFAFOutputRow](env, "SQLFAFOutputRow")
	if err != nil {
		t.Fatal(err)
	}
	configureSQLFAF(t, func(statement string, args []any) (sqlFAFQueryResult, error) {
		return sqlFAFQueryResult{
			columns: sqlFAFColumns("myint", "myvarchar"),
			rows:    [][]driver.Value{{int64(10), []byte("A")}, {int64(90), []byte("I")}},
		}, nil
	})
	rowProvider, err := NewSQLHistoricalProviderWithOptions(db, outputSchema,
		"select myint, myvarchar from mytesttable",
		SQLHistoricalProviderOptions{
			RowConverter: func(metadata SQLHistoricalRowMetadata, row map[string]any) (any, error) {
				if metadata.Schema.Name() != "SQLFAFOutputRow" || len(metadata.Columns) != 2 {
					return nil, fmt.Errorf("unexpected SQL row hook metadata=%#v", metadata)
				}
				value := sqlFAFInt(row["myint"])
				if value == 90 {
					return nil, nil
				}
				return databaseFAFOutputRow{TheString: fmt.Sprintf(">%d<", value), IntPrimitive: 99000 + value}, nil
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	rowPlan, err := env.Build(Select(FromHistorical[databaseFAFOutputRow](env, "MyDBPooledRows", outputSchema, rowProvider),
		Alias("theString", Field[databaseFAFOutputRow, string]("theString")),
		Alias("intPrimitive", Field[databaseFAFOutputRow, int]("intPrimitive")),
	).Query(StatementName("sql-faf-row-hook")))
	if err != nil {
		t.Fatal(err)
	}
	rowResult, err := NewEngine(env).ExecuteFireAndForget(context.Background(), rowPlan)
	if err != nil || len(rowResult.Results()) != 1 {
		t.Fatalf("SQL FAF row hook result = %#v, err=%v", rowResult.Results(), err)
	}
	if rowResult.Results()[0].Get("theString").Any() != ">10<" || rowResult.Results()[0].Get("intPrimitive").Any() != 99010 {
		t.Fatalf("SQL FAF row hook values = %#v", rowResult.Results()[0])
	}
}

func TestSQLHistoricalFireAndForgetPreparedQueryMatchesJava(t *testing.T) {
	db := openSQLFAFDB(t)
	configureSQLFAF(t, func(statement string, args []any) (sqlFAFQueryResult, error) {
		return sqlFAFQueryResult{columns: sqlFAFColumns("myint"), rows: [][]driver.Value{{int64(10)}}}, nil
	})
	env := NewEnvironment()
	schema := sqlFAFMapSchema(t, "SQLFAFPrepared", FieldDef("myint", reflect.TypeOf(0)))
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema, "select myint from mytesttable where myint = 10", SQLHistoricalProviderOptions{PrepareStatement: true})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(FromHistorical[map[string]any](env, "MyDBPooled", schema, provider),
		Alias("myint", Field[map[string]any, int]("myint")),
	).Query(StatementName("sql-faf-prepared")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	prepared, err := engine.PrepareFireAndForget(plan)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 50; index++ {
		result, executeErr := prepared.Execute(context.Background())
		if executeErr != nil || len(result.Results()) != 1 {
			t.Fatalf("prepared execution %d result=%#v err=%v", index, result.Results(), executeErr)
		}
	}
	prepareCalls, queryCalls := sqlFAFCounts()
	if prepareCalls != 1 || queryCalls != 50 {
		t.Fatalf("prepared SQL calls prepare=%d query=%d", prepareCalls, queryCalls)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Execute(context.Background()); !errors.Is(err, ErrorState) {
		t.Fatalf("closed prepared SQL query error = %v", err)
	}
	if err := provider.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLHistoricalFireAndForgetSubstitutionParametersMatchesJava(t *testing.T) {
	db := openSQLFAFDB(t)
	configureSQLFAF(t, func(statement string, args []any) (sqlFAFQueryResult, error) {
		if len(args) != 1 {
			return sqlFAFQueryResult{}, fmt.Errorf("SQL FAF substitution args=%#v", args)
		}
		values := map[int]string{10: "A", 60: "F"}
		return sqlFAFQueryResult{columns: sqlFAFColumns("myvarchar"), rows: [][]driver.Value{{[]byte(values[sqlFAFInt(args[0])])}}}, nil
	})
	env := NewEnvironment()
	schema := sqlFAFMapSchema(t, "SQLFAFSubstitution", FieldDef("myvarchar", reflect.TypeOf("")))
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema, "select myvarchar from mytesttable where myint = ?", SQLHistoricalProviderOptions{
		Arguments: []func(HistoricalRequest) any{func(request HistoricalRequest) any {
			return request.Parameters["filterValue"].Any()
		}},
		ParameterTypes: map[string]reflect.Type{"filterValue": reflect.TypeOf(0)},
	})
	if err != nil {
		t.Fatal(err)
	}
	source := FromHistorical[map[string]any](env, "MyDBPooled", schema, provider)
	plan, err := env.Build(Select(source,
		Alias("c0", Field[map[string]any, string]("myvarchar")),
		Alias("c1", Parameter[int]("selectValue")),
	).Query(StatementName("sql-faf-substitution")))
	if err != nil {
		t.Fatal(err)
	}
	if canonical := string(plan.Canonical()); !strings.Contains(canonical, "parameters(filterValue:int,selectValue:int)") {
		t.Fatalf("SQL FAF parameter declaration missing from plan canonical: %s", canonical)
	}
	prepared, err := NewEngine(env).PrepareFireAndForget(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		selectValue int
		filterValue int
		want        string
	}{
		{1, 10, "A"},
		{2, 60, "F"},
	} {
		result, executeErr := prepared.ExecuteWithParameters(context.Background(), ParameterValues{
			"selectValue": test.selectValue,
			"filterValue": test.filterValue,
		})
		if executeErr != nil || len(result.Results()) != 1 {
			t.Fatalf("SQL FAF substitution result=%#v err=%v", result.Results(), executeErr)
		}
		row := result.Results()[0]
		if row.Get("c0").Any() != test.want || row.Get("c1").Any() != test.selectValue {
			t.Fatalf("SQL FAF substitution row=%#v want=%#v", row, test)
		}
	}
	if _, err := prepared.ExecuteWithParameters(context.Background(), ParameterValues{"selectValue": 1}); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("SQL FAF missing provider parameter error = %v", err)
	}
	if _, err := prepared.ExecuteWithParameters(context.Background(), ParameterValues{"selectValue": 1, "filterValue": "wrong"}); err == nil || !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("SQL FAF provider parameter type error = %v", err)
	}
	if _, err := prepared.ExecuteWithParameters(context.Background(), ParameterValues{"selectValue": 1, "filterValue": 10, "extra": true}); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("SQL FAF extra provider parameter error = %v", err)
	}
}

func TestSQLHistoricalProviderRejectsInvalidParameterTypeDeclarations(t *testing.T) {
	db := openSQLFAFDB(t)
	schema := sqlFAFMapSchema(t, "SQLFAFInvalidParameterTypes", FieldDef("myint", reflect.TypeOf(0)))
	cases := []struct {
		name  string
		types map[string]reflect.Type
		code  ErrorCode
	}{
		{name: "blank name", types: map[string]reflect.Type{" ": reflect.TypeOf(0)}, code: ErrorInvalidRule},
		{name: "nil type", types: map[string]reflect.Type{"wanted": nil}, code: ErrorInvalidRule},
		{name: "trimmed conflict", types: map[string]reflect.Type{
			"wanted":   reflect.TypeOf(0),
			" wanted ": reflect.TypeOf(""),
		}, code: ErrorTypeMismatch},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewSQLHistoricalProviderWithOptions(db, schema, "select myint from mytesttable", SQLHistoricalProviderOptions{
				ParameterTypes: test.types,
			})
			if err == nil || !errors.Is(err, test.code) {
				t.Fatalf("invalid SQL parameter types error = %v, want %s", err, test.code)
			}
		})
	}
}

func TestSQLHistoricalFireAndForgetDistinctMatchesJava(t *testing.T) {
	db := openSQLFAFDB(t)
	configureSQLFAF(t, func(_ string, args []any) (sqlFAFQueryResult, error) {
		return sqlFAFQueryResult{columns: sqlFAFColumns("myint"), rows: [][]driver.Value{{int64(10)}, {int64(10)}}}, nil
	})
	env := NewEnvironment()
	schema := sqlFAFMapSchema(t, "SQLFAFDistinct", FieldDef("myint", reflect.TypeOf(0)))
	provider, err := NewSQLHistoricalProvider(db, schema, "select myint from mytesttable union all select myint from mytesttable")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(FromHistorical[map[string]any](env, "MyDBPooled", schema, provider),
		Alias("myint", Field[map[string]any, int]("myint")),
	).Query(StatementName("sql-faf-distinct"), WithDistinct()))
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewEngine(env).ExecuteFireAndForget(context.Background(), plan)
	if err != nil || len(result.Results()) != 1 || result.Results()[0].Get("myint").Any() != 10 {
		t.Fatalf("SQL FAF distinct result=%#v err=%v", result.Results(), err)
	}
}

func TestSQLHistoricalFireAndForgetWhereMatchesJava(t *testing.T) {
	db := openSQLFAFDB(t)
	configureSQLFAF(t, func(_ string, args []any) (sqlFAFQueryResult, error) {
		return sqlFAFQueryResult{
			columns: sqlFAFColumns("myint", "myvarchar"),
			rows:    [][]driver.Value{{int64(10), []byte("A")}, {int64(50), []byte("E")}, {int64(60), []byte("F")}},
		}, nil
	})
	env := NewEnvironment()
	schema := sqlFAFMapSchema(t, "SQLFAFWhere", FieldDef("myint", reflect.TypeOf(0)), FieldDef("myvarchar", reflect.TypeOf("")))
	provider, err := NewSQLHistoricalProvider(db, schema, "select myint, myvarchar from mytesttable")
	if err != nil {
		t.Fatal(err)
	}
	source := FromHistorical[map[string]any](env, "MyDBPooled", schema, provider).Filter(In[string](
		Field[map[string]any, string]("myvarchar"), Literal("A"), Literal("E"),
	))
	plan, err := env.Build(Select(source,
		Alias("myint", Field[map[string]any, int]("myint")),
		Alias("myvarchar", Field[map[string]any, string]("myvarchar")),
	).Query(StatementName("sql-faf-where")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewEngine(env).ExecuteFireAndForget(context.Background(), plan)
	if err != nil || len(result.Results()) != 2 {
		t.Fatalf("SQL FAF where result=%#v err=%v", result.Results(), err)
	}
	if result.Results()[0].Get("myint").Any() != 10 || result.Results()[1].Get("myint").Any() != 50 {
		t.Fatalf("SQL FAF where rows=%#v", result.Results())
	}
}

func TestSQLHistoricalFireAndForgetVariableMatchesJava(t *testing.T) {
	db := openSQLFAFDB(t)
	configureSQLFAF(t, func(_ string, args []any) (sqlFAFQueryResult, error) {
		values := map[int]string{20: "B", 30: "C", 50: "E"}
		if len(args) != 1 {
			return sqlFAFQueryResult{}, fmt.Errorf("variable query args=%#v", args)
		}
		return sqlFAFQueryResult{columns: sqlFAFColumns("myvarchar"), rows: [][]driver.Value{{[]byte(values[sqlFAFInt(args[0])])}}}, nil
	})
	env := NewEnvironment()
	if err := env.RegisterVariable("myvar", 20); err != nil {
		t.Fatal(err)
	}
	schema := sqlFAFMapSchema(t, "SQLFAFVariable", FieldDef("myvarchar", reflect.TypeOf("")))
	provider, err := NewSQLHistoricalProvider(db, schema, "select myvarchar from mytesttable where myint = ?", func(request HistoricalRequest) any {
		return request.Variables["myvar"].Any()
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(FromHistorical[map[string]any](env, "MyDBPooled", schema, provider),
		Alias("myvarchar", Field[map[string]any, string]("myvarchar")),
	).Query(StatementName("sql-faf-variable")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	prepared, err := engine.PrepareFireAndForget(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		value int
		want  string
	}{
		{20, "B"},
		{50, "E"},
		{30, "C"},
	} {
		if test.value != 20 {
			if err := engine.SetVariable(context.Background(), "myvar", test.value); err != nil {
				t.Fatal(err)
			}
		}
		result, executeErr := prepared.Execute(context.Background())
		if executeErr != nil || len(result.Results()) != 1 || result.Results()[0].Get("myvarchar").Any() != test.want {
			t.Fatalf("SQL FAF variable value=%d result=%#v err=%v", test.value, result.Results(), executeErr)
		}
	}
}

func TestSQLHistoricalFireAndForgetFluentPlanMatchesJava(t *testing.T) {
	db := openSQLFAFDB(t)
	configureSQLFAF(t, func(string, []any) (sqlFAFQueryResult, error) {
		return sqlFAFQueryResult{columns: sqlFAFColumns("myvarchar"), rows: [][]driver.Value{{[]byte("B")}, {[]byte("C")}}}, nil
	})
	env := NewEnvironment()
	schema := sqlFAFMapSchema(t, "SQLFAFFluent", FieldDef("myvarchar", reflect.TypeOf("")))
	provider, err := NewSQLHistoricalProvider(db, schema, "select myvarchar from mytesttable where myint between 20 and 30")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(FromHistorical[map[string]any](env, "MyDBPooled", schema, provider),
		Alias("c0", Field[map[string]any, string]("myvarchar")),
	).Query(StatementName("sql-faf-fluent-plan"), OrderBy(Ascending(ResultField[string]("c0")))))
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewEngine(env).ExecuteFireAndForget(context.Background(), plan)
	if err != nil || len(result.Results()) != 2 || result.Results()[0].Get("c0").Any() != "B" || result.Results()[1].Get("c0").Any() != "C" {
		t.Fatalf("SQL FAF fluent plan result=%#v err=%v", result.Results(), err)
	}
}

func TestSQLHistoricalFireAndForgetSQLTextSubqueryMatchesJava(t *testing.T) {
	db := openSQLFAFDB(t)
	configureSQLFAF(t, func(_ string, args []any) (sqlFAFQueryResult, error) {
		if len(args) != 1 {
			return sqlFAFQueryResult{}, fmt.Errorf("SQL subquery parameter args=%#v", args)
		}
		values := map[int]string{10: "A", 30: "C"}
		return sqlFAFQueryResult{columns: sqlFAFColumns("myvarchar"), rows: [][]driver.Value{{[]byte(values[sqlFAFInt(args[0])])}}}, nil
	})
	env := NewEnvironment()
	inputSchema, err := RegisterStruct[databaseFAFSubqueryEvent](env, "DatabaseFAFInput")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "DatabaseFAFInputWindow", inputSchema, NamedWindowRetention(LastEvent())); err != nil {
		t.Fatal(err)
	}
	historySchema := sqlFAFMapSchema(t, "SQLFAFSubqueryHistory", FieldDef("myvarchar", reflect.TypeOf("")))
	engine := NewEngine(env)
	provider, err := NewSQLHistoricalProvider(db, historySchema, "select myvarchar from mytesttable where myint = ?", func(HistoricalRequest) any {
		window, ok := engine.NamedWindow("DatabaseFAFInputWindow")
		if !ok {
			return nil
		}
		events, snapshotErr := window.Snapshot(context.Background())
		if snapshotErr != nil || len(events) == 0 {
			return nil
		}
		return events[len(events)-1].Get("intPrimitive").Any()
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(FromHistorical[map[string]any](env, "MyDBPlain", historySchema, provider),
		Alias("myvarchar", Field[map[string]any, string]("myvarchar")),
	).Query(StatementName("sql-faf-sql-text-subquery")))
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := engine.PrepareFireAndForget(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		value int
		want  string
	}{
		{30, "C"},
		{10, "A"},
	} {
		if err := engine.InsertNamedWindow(context.Background(), "DatabaseFAFInputWindow", databaseFAFSubqueryEvent{IntPrimitive: test.value}); err != nil {
			t.Fatal(err)
		}
		result, executeErr := prepared.Execute(context.Background())
		if executeErr != nil || len(result.Results()) != 1 || result.Results()[0].Get("myvarchar").Any() != test.want {
			t.Fatalf("SQL text subquery value=%d result=%#v err=%v", test.value, result.Results(), executeErr)
		}
	}
}

func TestSQLHistoricalFireAndForgetInvalidPathsMatchJavaBoundary(t *testing.T) {
	db := openSQLFAFDB(t)
	configureSQLFAF(t, func(string, []any) (sqlFAFQueryResult, error) {
		return sqlFAFQueryResult{}, errors.New("invalid SQL: no tables used")
	})
	env := NewEnvironment()
	schema := sqlFAFMapSchema(t, "SQLFAFInvalid", FieldDef("myvarchar", reflect.TypeOf("")))
	provider, err := NewSQLHistoricalProvider(db, schema, "select *")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(FromHistorical[map[string]any](env, "MyDBPooled", schema, provider),
		Alias("myvarchar", Field[map[string]any, string]("myvarchar")),
	).Query(StatementName("sql-faf-invalid-sql")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.ExecuteFireAndForget(context.Background(), plan); err == nil || !strings.Contains(err.Error(), "invalid SQL") {
		t.Fatalf("invalid SQL execution error = %v", err)
	}
	prepared, err := engine.PrepareFireAndForget(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Execute(context.Background()); !errors.Is(err, ErrorState) {
		t.Fatalf("closed SQL FAF query error = %v", err)
	}
	// Go's historical source is deliberately composable in Join and Context
	// FAF. The Java suite rejects SQL+SQL Join and Context SQL FAF, so those
	// Java invalid cases are tracked as an explicit API difference rather than
	// making a valid Go fluent rule fail at Build time.
}

func TestSQLHistoricalFireAndForgetMySQLDocker(t *testing.T) {
	dsn := os.Getenv("ESPER_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set ESPER_MYSQL_DSN to run the SQL FAF MySQL integration fixture")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	env := NewEnvironment()
	schema := sqlFAFMapSchema(t, "SQLFAFMySQL", FieldDef("myint", reflect.TypeOf(0)), FieldDef("myvarchar", reflect.TypeOf("")))
	provider, err := NewSQLHistoricalProvider(db, schema, "select myint, myvarchar from mytesttable where myint in (10, 50)")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(FromHistorical[map[string]any](env, "MyDBPooled", schema, provider),
		Alias("myint", Field[map[string]any, int]("myint")),
		Alias("myvarchar", Field[map[string]any, string]("myvarchar")),
	).Query(StatementName("sql-faf-mysql"), OrderBy(Ascending(ResultField[int]("myint")))))
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewEngine(env).ExecuteFireAndForget(ctx, plan)
	if err != nil || len(result.Results()) != 2 {
		t.Fatalf("SQL FAF MySQL result=%#v err=%v", result.Results(), err)
	}
	if result.Results()[0].Get("myint").Any() != 10 || result.Results()[0].Get("myvarchar").Any() != "A" || result.Results()[1].Get("myint").Any() != 50 || result.Results()[1].Get("myvarchar").Any() != "E" {
		t.Fatalf("SQL FAF MySQL rows=%#v", result.Results())
	}

	parameterProvider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select myint, myvarchar from mytesttable where myint = ?",
		SQLHistoricalProviderOptions{
			Arguments: []func(HistoricalRequest) any{func(request HistoricalRequest) any {
				return request.Parameters["wanted"].Any()
			}},
			ParameterTypes:   map[string]reflect.Type{"wanted": reflect.TypeOf(0)},
			PrepareStatement: true,
		})
	if err != nil {
		t.Fatal(err)
	}
	parameterPlan, err := env.Build(Select(FromHistorical[map[string]any](env, "MyDBPooledParameterized", schema, parameterProvider),
		Alias("myint", Field[map[string]any, int]("myint")),
		Alias("myvarchar", Field[map[string]any, string]("myvarchar")),
	).Query(StatementName("sql-faf-mysql-parameterized")))
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := NewEngine(env).PrepareFireAndForget(parameterPlan)
	if err != nil {
		t.Fatal(err)
	}
	parameterResult, err := prepared.ExecuteWithParameters(ctx, ParameterValues{"wanted": 60})
	if err != nil || len(parameterResult.Results()) != 1 || parameterResult.Results()[0].Get("myvarchar").Any() != "F" {
		t.Fatalf("SQL FAF MySQL parameterized result=%#v err=%v", parameterResult.Results(), err)
	}
}
