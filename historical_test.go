package esper

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type fixtureSQLDriver struct{}

var fixtureSQLQueryCalls atomic.Int64
var fixtureSQLPrepareCalls atomic.Int64

func (fixtureSQLDriver) Open(string) (driver.Conn, error) { return fixtureSQLConn{}, nil }

type fixtureSQLConn struct{}

func (fixtureSQLConn) Prepare(string) (driver.Stmt, error) {
	fixtureSQLPrepareCalls.Add(1)
	return fixtureSQLStmt{}, nil
}
func (fixtureSQLConn) Close() error              { return nil }
func (fixtureSQLConn) Begin() (driver.Tx, error) { return fixtureSQLTx{}, nil }
func (fixtureSQLConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	fixtureSQLQueryCalls.Add(1)
	return &fixtureSQLRows{}, nil
}

type fixtureSQLStmt struct{}

func (fixtureSQLStmt) Close() error                               { return nil }
func (fixtureSQLStmt) NumInput() int                              { return -1 }
func (fixtureSQLStmt) Exec([]driver.Value) (driver.Result, error) { return driver.RowsAffected(0), nil }
func (fixtureSQLStmt) Query([]driver.Value) (driver.Rows, error)  { return &fixtureSQLRows{}, nil }

type fixtureSQLTx struct{}

func (fixtureSQLTx) Commit() error   { return nil }
func (fixtureSQLTx) Rollback() error { return nil }

type fixtureSQLRows struct{ emitted bool }

func (r *fixtureSQLRows) Columns() []string { return []string{"symbol", "value", "enabled"} }
func (r *fixtureSQLRows) Close() error      { return nil }
func (r *fixtureSQLRows) Next(dest []driver.Value) error {
	if r.emitted {
		return io.EOF
	}
	r.emitted = true
	dest[0] = []byte("A")
	dest[1] = []byte("7")
	dest[2] = []byte("1")
	return nil
}

type metadataFixtureDriver struct{}

var metadataFixtureQueriesMu sync.Mutex
var metadataFixtureQueries []string

func (metadataFixtureDriver) Open(string) (driver.Conn, error) { return metadataFixtureConn{}, nil }

type metadataFixtureConn struct{}

func (metadataFixtureConn) Prepare(statement string) (driver.Stmt, error) {
	return metadataFixtureStmt{statement: statement}, nil
}
func (metadataFixtureConn) Close() error              { return nil }
func (metadataFixtureConn) Begin() (driver.Tx, error) { return fixtureSQLTx{}, nil }
func (metadataFixtureConn) QueryContext(_ context.Context, statement string, _ []driver.NamedValue) (driver.Rows, error) {
	metadataFixtureQueriesMu.Lock()
	metadataFixtureQueries = append(metadataFixtureQueries, statement)
	metadataFixtureQueriesMu.Unlock()
	return &metadataFixtureRows{metadata: strings.Contains(statement, "metadata")}, nil
}

type metadataFixtureStmt struct{ statement string }

func (s metadataFixtureStmt) Close() error  { return nil }
func (s metadataFixtureStmt) NumInput() int { return -1 }
func (s metadataFixtureStmt) Exec([]driver.Value) (driver.Result, error) {
	return driver.RowsAffected(0), nil
}
func (s metadataFixtureStmt) Query([]driver.Value) (driver.Rows, error) {
	metadataFixtureQueriesMu.Lock()
	metadataFixtureQueries = append(metadataFixtureQueries, s.statement)
	metadataFixtureQueriesMu.Unlock()
	return &metadataFixtureRows{metadata: strings.Contains(s.statement, "metadata")}, nil
}

type metadataFixtureRows struct {
	metadata bool
	emitted  bool
}

func (r *metadataFixtureRows) Columns() []string { return []string{"symbol", "value"} }
func (r *metadataFixtureRows) Close() error      { return nil }
func (r *metadataFixtureRows) ColumnTypeDatabaseTypeName(index int) string {
	if r.metadata {
		return []string{"VARCHAR", "BIGINT"}[index]
	}
	return []string{"LIVE_TEXT", "LIVE_NUMBER"}[index]
}
func (r *metadataFixtureRows) ColumnTypeScanType(index int) reflect.Type {
	if index == 0 {
		return reflect.TypeOf("")
	}
	return reflect.TypeOf(int64(0))
}
func (r *metadataFixtureRows) Next(dest []driver.Value) error {
	if r.emitted {
		return io.EOF
	}
	r.emitted = true
	dest[0] = []byte("A")
	dest[1] = []byte("7")
	return nil
}

type fixtureHistoricalProvider struct {
	mu     sync.Mutex
	schema Schema
	rows   []map[string]any
	calls  []HistoricalRequest
}

func (p *fixtureHistoricalProvider) Poll(ctx context.Context, request HistoricalRequest) ([]Event, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.calls = append(p.calls, request)
	rows := append([]map[string]any(nil), p.rows...)
	p.mu.Unlock()
	result := make([]Event, 0, len(rows))
	for _, values := range rows {
		event, err := newEvent(p.schema, values, request.Now)
		if err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, nil
}

func (p *fixtureHistoricalProvider) CallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

func TestSQLHistoricalProviderMapsRowsAndBindsArguments(t *testing.T) {
	const driverName = "esper-fixture-historical"
	sql.Register(driverName, fixtureSQLDriver{})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema, err := NewMapSchema("HistorySQL", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("value", reflect.TypeOf(0)),
		FieldDef("enabled", reflect.TypeOf(false)),
	})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewSQLHistoricalProvider(db, schema, "select symbol, value, enabled", func(request HistoricalRequest) any {
		return request.Trigger.TypeName()
	})
	if err != nil {
		t.Fatal(err)
	}
	triggerSchema, err := StructSchema[runtimeTestTrade]("TradeSQLTrigger")
	if err != nil {
		t.Fatal(err)
	}
	trigger, err := newEvent(triggerSchema, runtimeTestTrade{Symbol: "A"}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	events, err := provider.Poll(context.Background(), HistoricalRequest{Trigger: trigger, Now: trigger.ReceivedAt()})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Get("symbol").Any() != "A" || events[0].Get("value").Any() != 7 || events[0].Get("enabled").Any() != true {
		t.Fatalf("SQL historical events = %#v", events)
	}
}

func TestSQLHistoricalProviderSupportsLRUCacheAndClonesResults(t *testing.T) {
	const driverName = "esper-fixture-historical-lru"
	sql.Register(driverName, fixtureSQLDriver{})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	fixtureSQLQueryCalls.Store(0)
	schema, err := NewMapSchema("HistorySQLLRU", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("value", reflect.TypeOf(0)),
		FieldDef("enabled", reflect.TypeOf(false)),
	})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema, "select symbol, value, enabled", SQLHistoricalProviderOptions{
		Arguments: []func(HistoricalRequest) any{func(request HistoricalRequest) any {
			return request.Parameters["id"].Any()
		}},
		Cache: SQLHistoricalCacheConfig{LRUSize: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0).UTC()
	first, err := provider.Poll(context.Background(), HistoricalRequest{Now: now, Parameters: map[string]Value{"id": Present(1)}})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 {
		t.Fatalf("first cached query returned %#v", first)
	}
	first[0].Underlying().(map[string]any)["value"] = 99
	second, err := provider.Poll(context.Background(), HistoricalRequest{Now: now.Add(time.Second), Parameters: map[string]Value{"id": Present(1)}})
	if err != nil {
		t.Fatal(err)
	}
	if fixtureSQLQueryCalls.Load() != 1 || second[0].Get("value").Any() != 7 {
		t.Fatalf("LRU cache calls=%d second=%#v", fixtureSQLQueryCalls.Load(), second)
	}
	if _, err := provider.Poll(context.Background(), HistoricalRequest{Now: now.Add(2 * time.Second), Parameters: map[string]Value{"id": Present(2)}}); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Poll(context.Background(), HistoricalRequest{Now: now.Add(3 * time.Second), Parameters: map[string]Value{"id": Present(1)}}); err != nil {
		t.Fatal(err)
	}
	if fixtureSQLQueryCalls.Load() != 3 {
		t.Fatalf("LRU eviction query calls=%d, want 3", fixtureSQLQueryCalls.Load())
	}
}

func TestSQLHistoricalProviderSupportsExpiryAndColumnCase(t *testing.T) {
	const driverName = "esper-fixture-historical-expiry"
	sql.Register(driverName, fixtureSQLDriver{})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	fixtureSQLQueryCalls.Store(0)
	schema, err := NewMapSchema("HistorySQLUpper", []FieldSpec{
		FieldDef("SYMBOL", reflect.TypeOf("")),
		FieldDef("VALUE", reflect.TypeOf(0)),
		FieldDef("ENABLED", reflect.TypeOf(false)),
	})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema, "select symbol, value, enabled", SQLHistoricalProviderOptions{
		Cache:      SQLHistoricalCacheConfig{MaxAge: time.Second, PurgeInterval: time.Second},
		ColumnCase: SQLColumnCaseUpper,
	})
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(200, 0).UTC()
	if events, pollErr := provider.Poll(context.Background(), HistoricalRequest{Now: base}); pollErr != nil || len(events) != 1 || events[0].Get("VALUE").Any() != 7 {
		t.Fatalf("uppercase SQL column mapping events=%#v err=%v", events, pollErr)
	}
	if _, err := provider.Poll(context.Background(), HistoricalRequest{Now: base.Add(500 * time.Millisecond)}); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Poll(context.Background(), HistoricalRequest{Now: base.Add(2 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	if fixtureSQLQueryCalls.Load() != 2 {
		t.Fatalf("expiry cache query calls=%d, want 2", fixtureSQLQueryCalls.Load())
	}
}

func TestSQLHistoricalProviderRejectsInvalidCacheOptions(t *testing.T) {
	const driverName = "esper-fixture-historical-invalid-cache"
	sql.Register(driverName, fixtureSQLDriver{})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema, err := NewMapSchema("HistorySQLInvalidCache", []FieldSpec{FieldDef("symbol", reflect.TypeOf(""))})
	if err != nil {
		t.Fatal(err)
	}
	for _, cache := range []SQLHistoricalCacheConfig{
		{LRUSize: -1},
		{MaxAge: time.Second},
		{LRUSize: 1, MaxAge: time.Second},
	} {
		if _, err := NewSQLHistoricalProviderWithOptions(db, schema, "select symbol", SQLHistoricalProviderOptions{Cache: cache}); err == nil {
			t.Fatalf("cache config %#v unexpectedly accepted", cache)
		}
	}
}

func TestSQLHistoricalProviderSupportsRewriterConverterAndClose(t *testing.T) {
	const driverName = "esper-fixture-historical-prepared"
	sql.Register(driverName, fixtureSQLDriver{})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	fixtureSQLPrepareCalls.Store(0)
	schema, err := NewMapSchema("HistorySQLPrepared", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("value", reflect.TypeOf(0)),
		FieldDef("enabled", reflect.TypeOf(false)),
	})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema, "select symbol, value from history where id = ? and kind = ? and note = '?'", SQLHistoricalProviderOptions{
		Arguments: []func(HistoricalRequest) any{
			func(HistoricalRequest) any { return 1 },
			func(HistoricalRequest) any { return 2 },
		},
		StatementRewriter: NewSQLPositionalPlaceholderRewriter("$", 1),
		PrepareStatement:  true,
		ValueConverter: func(column SQLHistoricalColumnMetadata, value any, target reflect.Type) (any, error) {
			if column.Name == "value" && target.Kind() == reflect.Int {
				return 42, nil
			}
			return coerceHistoricalSQLValue(value, target)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if provider.Statement != "select symbol, value from history where id = $1 and kind = $2 and note = '?'" {
		t.Fatalf("rewritten SQL = %q", provider.Statement)
	}
	events, err := provider.Poll(context.Background(), HistoricalRequest{Now: time.Unix(300, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Get("value").Any() != 42 {
		t.Fatalf("prepared converter events = %#v", events)
	}
	if fixtureSQLPrepareCalls.Load() != 1 {
		t.Fatalf("prepared statement count = %d, want 1", fixtureSQLPrepareCalls.Load())
	}
	if err := provider.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Poll(context.Background(), HistoricalRequest{}); err == nil {
		t.Fatal("poll after provider close unexpectedly succeeded")
	}
}

func TestSQLHistoricalProviderUsesSampleMetadataStatement(t *testing.T) {
	const driverName = "esper-fixture-historical-metadata"
	sql.Register(driverName, metadataFixtureDriver{})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	metadataFixtureQueriesMu.Lock()
	metadataFixtureQueries = nil
	metadataFixtureQueriesMu.Unlock()
	schema, err := NewMapSchema("HistorySQLSampleMetadata", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("value", reflect.TypeOf(int64(0))),
	})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select symbol, value from live_history where id = ?",
		SQLHistoricalProviderOptions{
			Arguments:         []func(HistoricalRequest) any{func(HistoricalRequest) any { return 6 }},
			MetadataMode:      SQLHistoricalMetadataSample,
			MetadataStatement: "select symbol, value from metadata_history where id = ?",
			PrepareStatement:  true,
			ValueConverter: func(column SQLHistoricalColumnMetadata, value any, target reflect.Type) (any, error) {
				if column.Name == "value" && column.DatabaseTypeName == "BIGINT" && target == reflect.TypeOf(int64(0)) {
					return int64(70), nil
				}
				return coerceHistoricalSQLValue(value, target)
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	events, err := provider.Poll(context.Background(), HistoricalRequest{Now: time.Unix(400, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Get("symbol").Any() != "A" || events[0].Get("value").Any() != int64(70) {
		t.Fatalf("sample metadata events = %#v", events)
	}
	metadataFixtureQueriesMu.Lock()
	queries := append([]string(nil), metadataFixtureQueries...)
	metadataFixtureQueriesMu.Unlock()
	if len(queries) != 2 || !strings.Contains(queries[0], "metadata_history") || !strings.Contains(queries[1], "live_history") {
		t.Fatalf("sample metadata query order = %#v", queries)
	}
	if err := provider.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Poll(context.Background(), HistoricalRequest{}); err == nil {
		t.Fatal("sample metadata provider poll after close unexpectedly succeeded")
	}
	if _, err := NewSQLHistoricalProviderWithOptions(db, schema, "select value", SQLHistoricalProviderOptions{MetadataMode: SQLHistoricalMetadataSample}); err == nil {
		t.Fatal("sample metadata mode accepted without metadata statement")
	}
	if _, err := NewSQLHistoricalProviderWithOptions(db, schema, "select value", SQLHistoricalProviderOptions{
		MetadataStatement: "select value",
	}); err == nil {
		t.Fatal("metadata statement accepted without sample metadata mode")
	}
}

func TestSQLPositionalPlaceholderRewriterRejectsMismatch(t *testing.T) {
	rewriter := NewSQLPositionalPlaceholderRewriter("$", 1)
	if _, err := rewriter("select ?", 2); err == nil {
		t.Fatal("placeholder mismatch unexpectedly accepted")
	}
	if _, err := rewriter("select ?", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSQLPositionalPlaceholderRewriter("", 1)("select ?", 1); err == nil {
		t.Fatal("empty placeholder prefix unexpectedly accepted")
	}
	statement := "select ?, '?', \"?\", `?`, [?] -- ?\n# ?\n/* ? /* ? */ */ ? /* tail */ ?"
	rewritten, err := rewriter(statement, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := "select $1, '?', \"?\", `?`, [?] -- ?\n# ?\n/* ? /* ? */ */ $2 /* tail */ $3"
	if rewritten != want {
		t.Fatalf("quoted/comment SQL rewrite = %q, want %q", rewritten, want)
	}
	dollarQuoted, err := rewriter("select ?, $$?$$, $tag$?$tag$, ?", 2)
	if err != nil {
		t.Fatal(err)
	}
	if dollarQuoted != "select $1, $$?$$, $tag$?$tag$, $2" {
		t.Fatalf("dollar-quoted SQL rewrite = %q", dollarQuoted)
	}
	for _, test := range []struct {
		dialect SQLHistoricalPlaceholderDialect
		want    string
	}{
		{SQLHistoricalPlaceholderQuestion, "select ?"},
		{SQLHistoricalPlaceholderDollarNumbered, "select $1"},
		{SQLHistoricalPlaceholderColonNumbered, "select :1"},
		{SQLHistoricalPlaceholderAtPNumbered, "select @p1"},
	} {
		got, rewriteErr := NewSQLHistoricalPlaceholderRewriter(test.dialect)("select ?", 1)
		if rewriteErr != nil || got != test.want {
			t.Fatalf("dialect %d rewrite=%q err=%v want=%q", test.dialect, got, rewriteErr, test.want)
		}
	}
	if _, err := NewSQLHistoricalPlaceholderRewriter(SQLHistoricalPlaceholderDialect(99))("select ?", 1); err == nil {
		t.Fatal("unknown placeholder dialect unexpectedly accepted")
	}
}

func TestSQLHistoricalProviderMySQLDocker(t *testing.T) {
	dsn := os.Getenv("ESPER_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set ESPER_MYSQL_DSN to run the MySQL integration fixture")
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
	schema, err := NewMapSchema("MySQLHistory", []FieldSpec{
		FieldDef("myint", reflect.TypeOf(0)),
		FieldDef("mybool", reflect.TypeOf(false)),
	})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select myint, mybool from mytesttable where mybigint = ?",
		SQLHistoricalProviderOptions{
			Arguments: []func(HistoricalRequest) any{func(request HistoricalRequest) any {
				return request.Parameters["id"].Any()
			}},
			ColumnCase: SQLColumnCaseLower,
			Cache:      SQLHistoricalCacheConfig{LRUSize: 4},
		})
	if err != nil {
		t.Fatal(err)
	}
	events, err := provider.Poll(ctx, HistoricalRequest{
		Now:        time.Now().UTC(),
		Parameters: map[string]Value{"id": Present(6)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Get("myint").Any() != 60 || events[0].Get("mybool").Any() != false {
		t.Fatalf("MySQL historical result = %#v", events)
	}
	stringSchema, err := NewMapSchema("MySQLHistoryStringInt", []FieldSpec{
		FieldDef("myint", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	stringProvider, err := NewSQLHistoricalProviderWithOptions(db, stringSchema,
		"select myint from mytesttable where mybigint = ?",
		SQLHistoricalProviderOptions{
			Arguments: []func(HistoricalRequest) any{func(request HistoricalRequest) any {
				return request.Parameters["id"].Any()
			}},
			TypeBindings: map[string]reflect.Type{
				"INT":     reflect.TypeOf(""),
				"INTEGER": reflect.TypeOf(""),
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	stringEvents, err := stringProvider.Poll(ctx, HistoricalRequest{
		Now:        time.Now().UTC(),
		Parameters: map[string]Value{"id": Present(6)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(stringEvents) != 1 || stringEvents[0].Get("myint").Any() != "60" {
		t.Fatalf("MySQL SQL type binding result = %#v", stringEvents)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	txProvider, err := NewSQLHistoricalProviderWithQueryer(tx, schema, "select myint, mybool from mytesttable where mybigint = ?", SQLHistoricalProviderOptions{
		Arguments: []func(HistoricalRequest) any{func(request HistoricalRequest) any {
			return request.Parameters["id"].Any()
		}},
		PrepareStatement: true,
	})
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	txEvents, err := txProvider.Poll(ctx, HistoricalRequest{
		Now:        time.Now().UTC(),
		Parameters: map[string]Value{"id": Present(6)},
	})
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if len(txEvents) != 1 || txEvents[0].Get("myint").Any() != 60 {
		_ = tx.Rollback()
		t.Fatalf("transaction historical result = %#v", txEvents)
	}
	if err := txProvider.Close(); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLSinkMySQLDocker(t *testing.T) {
	dsn := os.Getenv("ESPER_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set ESPER_MYSQL_DSN to run the MySQL integration fixture")
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
	const key = "esper-go-sink-test"
	if _, err := db.ExecContext(ctx, "delete from mytestupsert where key1 = ?", key); err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(context.Background(), "delete from mytestupsert where key1 = ?", key)
	schema, err := NewMapSchema("MySQLSinkEvent", []FieldSpec{
		FieldDef("key1", reflect.TypeOf("")),
		FieldDef("key2", reflect.TypeOf(0)),
		FieldDef("value1", reflect.TypeOf("")),
		FieldDef("value2", reflect.TypeOf(float64(0))),
	})
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, map[string]any{"key1": key, "key2": 7, "value1": "from-go", "value2": 1.25}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	sink, err := NewSQLSinkWithOptions(db, "insert into mytestupsert(key1, key2, value1, value2) values (?, ?, ?, ?)", SQLSinkOptions{
		Arguments: []func(Result) any{
			SQLResultField("key1"),
			SQLResultField("key2"),
			SQLResultField("value1"),
			SQLResultField("value2"),
		},
		PrepareStatement: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(ctx, ResultBatch{New: []Result{resultEvent(event)}}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	var key2 int
	var value1 string
	var value2 float64
	if err := db.QueryRowContext(ctx, "select key2, value1, value2 from mytestupsert where key1 = ?", key).Scan(&key2, &value1, &value2); err != nil {
		t.Fatal(err)
	}
	if key2 != 7 || value1 != "from-go" || value2 != 1.25 {
		t.Fatalf("MySQL sink row key2=%d value1=%q value2=%v", key2, value1, value2)
	}
}

func TestHistoricalStreamPollsOnTriggerEvent(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	historySchema, err := NewMapSchema("History", []FieldSpec{FieldDef("symbol", reflect.TypeOf("")), FieldDef("value", reflect.TypeOf(0))})
	if err != nil {
		t.Fatal(err)
	}
	provider := &fixtureHistoricalProvider{
		schema: historySchema,
		rows:   []map[string]any{{"symbol": "A", "value": 7}},
	}
	historical := FromHistoricalOn[map[string]any](env, "history", "Trade", historySchema, provider)
	plan, err := env.Build(Select(historical,
		Alias("symbol", Field[map[string]any, string]("symbol")),
		Alias("value", Field[map[string]any, int]("value")),
	).Query(StatementName("historical-poll")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var observed []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("historical result is not a row: %#v", result)
			}
			observed = append(observed, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if provider.CallCount() != 1 || len(observed) != 1 {
		t.Fatalf("historical polls=%d observed=%#v", provider.CallCount(), observed)
	}
	if observed[0].Get("symbol").Any() != "A" || observed[0].Get("value").Any() != 7 {
		t.Fatalf("historical row=%#v", observed[0].AsMap())
	}
}

func TestHistoricalFireAndForgetBindsParametersAndUsesOneSnapshot(t *testing.T) {
	env := NewEnvironment()
	historySchema, err := NewMapSchema("HistoryFAF", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("value", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	provider := &fixtureHistoricalProvider{
		schema: historySchema,
		rows:   []map[string]any{{"symbol": "A", "value": 7}},
	}
	historical := FromHistorical[map[string]any](env, "history-faf", historySchema, provider)
	wanted := Parameter[int]("wanted")
	plan, err := env.Build(Select(historical.Filter(
		Equal[int](Field[map[string]any, int]("value"), wanted),
	),
		Alias("symbol", Field[map[string]any, string]("symbol")),
		Alias("value", Field[map[string]any, int]("value")),
	).Query(StatementName("historical-faf")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	first, err := engine.ExecuteFireAndForgetWithParameters(context.Background(), plan, ParameterValues{"wanted": 7})
	if err != nil || len(first.Results()) != 1 {
		t.Fatalf("historical FAF first result = %#v, err=%v", first.Results(), err)
	}
	second, err := engine.ExecuteFireAndForgetWithParameters(context.Background(), plan, ParameterValues{"wanted": 9})
	if err != nil || len(second.Results()) != 0 {
		t.Fatalf("historical FAF second result = %#v, err=%v", second.Results(), err)
	}
	provider.mu.Lock()
	calls := append([]HistoricalRequest(nil), provider.calls...)
	provider.mu.Unlock()
	if len(calls) != 2 || calls[0].Parameters["wanted"].Any() != 7 || calls[1].Parameters["wanted"].Any() != 9 {
		t.Fatalf("historical FAF parameter requests = %#v", calls)
	}
}

func TestHistoricalStreamDeployBindsParameters(t *testing.T) {
	env, engine := newRuntimeTest(t)
	historySchema, err := NewMapSchema("HistoryDeployParams", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("value", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	provider := &fixtureHistoricalProvider{
		schema: historySchema,
		rows:   []map[string]any{{"symbol": "A", "value": 7}},
	}
	wanted := Parameter[int]("wanted")
	historical := FromHistorical[map[string]any](env, "history-deploy-params", historySchema, provider)
	plan, err := env.Build(Select(historical.Filter(
		Equal[int](Field[map[string]any, int]("value"), wanted),
	),
		Alias("symbol", Field[map[string]any, string]("symbol")),
	).Query(StatementName("historical-deploy-params")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.DeployWithParameters(context.Background(), plan, ParameterValues{"wanted": 7})
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var observed []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				observed = append(observed, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	provider.mu.Lock()
	calls := append([]HistoricalRequest(nil), provider.calls...)
	provider.mu.Unlock()
	if len(observed) != 1 || observed[0].Get("symbol").Any() != "A" || len(calls) != 1 || calls[0].Parameters["wanted"].Any() != 7 {
		t.Fatalf("deployed historical parameter result=%#v calls=%#v", observed, calls)
	}
}

func TestHistoricalStreamParticipatesInJoin(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	historySchema, err := NewMapSchema("History", []FieldSpec{FieldDef("symbol", reflect.TypeOf("")), FieldDef("value", reflect.TypeOf(0))})
	if err != nil {
		t.Fatal(err)
	}
	provider := &fixtureHistoricalProvider{
		schema: historySchema,
		rows:   []map[string]any{{"symbol": "A", "value": 7}},
	}
	trades := From[runtimeTestTrade](env, "Trade")
	historical := FromHistorical[map[string]any](env, "history", historySchema, provider)
	query := Join(trades, historical, OnEqual(
		Field[runtimeTestTrade, string]("symbol"),
		Field[map[string]any, string]("symbol"),
	)).Select(
		SelectLeft("trade", Field[runtimeTestTrade, string]("symbol")),
		SelectRight("value", Field[map[string]any, int]("value")),
	).Query(StatementName("historical-join"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var observed []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("historical join result is not a row: %#v", result)
			}
			observed = append(observed, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if len(observed) != 1 || observed[0].Get("trade").Any() != "A" || observed[0].Get("value").Any() != 7 {
		t.Fatalf("historical join rows=%#v", observed)
	}
	if provider.CallCount() != 1 {
		t.Fatalf("historical join provider calls=%d", provider.CallCount())
	}
}
