package db

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/liubaicai/esper/connectors"
)

type fakeResult int64

func (r fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeResult) RowsAffected() (int64, error) { return int64(r), nil }

type fakeExecutor struct {
	mu         sync.Mutex
	statements []string
	arguments  [][]any
	errors     []error
	updateRows int64
}

func (e *fakeExecutor) ExecContext(ctx context.Context, statement string, arguments ...any) (sql.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.statements = append(e.statements, statement)
	e.arguments = append(e.arguments, append([]any(nil), arguments...))
	if len(e.errors) > 0 {
		err := e.errors[0]
		e.errors = e.errors[1:]
		return nil, err
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(statement)), "update") {
		return fakeResult(e.updateRows), nil
	}
	return fakeResult(1), nil
}

func TestDMLSinkBindsPositionsAndSupportsRetry(t *testing.T) {
	executor := &fakeExecutor{errors: []error{errors.New("temporary"), nil}}
	sink, err := NewDMLSink(DMLSpec{
		Executor:      executor,
		Statement:     "insert into events(a,b) values (?, ?)",
		Retry:         2,
		RetryInterval: time.Millisecond,
		Bindings: []Binding{
			{Position: 2, Property: "b"},
			{Position: 1, Property: "a"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(context.Background(), Record{"a": 1, "b": "two"}); err != nil {
		t.Fatal(err)
	}
	executor.mu.Lock()
	statements := append([]string(nil), executor.statements...)
	arguments := append([][]any(nil), executor.arguments...)
	executor.mu.Unlock()
	if len(statements) != 2 || len(arguments) != 2 {
		t.Fatalf("calls = %#v args=%#v", statements, arguments)
	}
	if arguments[1][0] != 1 || arguments[1][1] != "two" {
		t.Fatalf("ordered arguments = %#v", arguments[1])
	}
	if err := sink.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Destroy(); err != nil {
		t.Fatalf("destroy after stop = %v", err)
	}
}

func TestDMLSinkLifecycleAndNestedProperty(t *testing.T) {
	executor := &fakeExecutor{}
	sink, err := NewDMLSink(DMLSpec{
		Executor:  executor,
		Statement: "update events set value = ?",
		Bindings:  []Binding{{Property: "nested.value"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(context.Background(), Record{"nested": map[string]any{"value": 7}}); !errors.Is(err, connectors.ErrNotStarted) {
		t.Fatalf("write before start = %v", err)
	}
	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Pause(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(context.Background(), Record{"nested": map[string]any{"value": 7}}); !errors.Is(err, connectors.ErrPaused) {
		t.Fatalf("write while paused = %v", err)
	}
	if err := sink.Resume(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(context.Background(), Record{"nested": map[string]any{"value": 7}}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Destroy(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(context.Background(), Record{"nested": map[string]any{"value": 7}}); !errors.Is(err, connectors.ErrDestroyed) {
		t.Fatalf("write after destroy = %v", err)
	}
	executor.mu.Lock()
	defer executor.mu.Unlock()
	if len(executor.arguments) != 1 || executor.arguments[0][0] != 7 {
		t.Fatalf("nested binding args = %#v", executor.arguments)
	}
}

func TestUpsertSinkUpdatesThenInserts(t *testing.T) {
	executor := &fakeExecutor{updateRows: 0}
	sink, err := NewUpsertSink(UpsertSpec{
		Executor: executor,
		Table:    "mytestupsert",
		Keys:     []Column{{Column: "k1", Property: "key1", Type: "varchar"}, {Column: "k2", Property: "key2", Type: "integer"}},
		Values:   []Column{{Column: "v1", Property: "value1", Type: "varchar"}, {Column: "v2", Property: "value2", Type: "double"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	value := Record{"key1": "myk1", "key2": 10, "value1": "myv1", "value2": 20.2}
	if err := sink.Write(context.Background(), value); err != nil {
		t.Fatal(err)
	}
	executor.mu.Lock()
	if len(executor.statements) != 2 {
		t.Fatalf("upsert calls = %#v", executor.statements)
	}
	if !strings.HasPrefix(strings.ToLower(executor.statements[0]), "update") || !strings.HasPrefix(strings.ToLower(executor.statements[1]), "insert") {
		t.Fatalf("upsert order = %#v", executor.statements)
	}
	if got := executor.arguments[1]; len(got) != 4 || got[0] != "myk1" || got[1] != 10 || got[2] != "myv1" || got[3] != 20.2 {
		t.Fatalf("insert arguments = %#v", got)
	}
	executor.mu.Unlock()

	executor.updateRows = 1
	if err := sink.Write(context.Background(), Record{"key1": "myk1", "key2": 10, "value1": "changed", "value2": 21.3}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Destroy(); err != nil {
		t.Fatal(err)
	}
}

func TestDBConnectorMySQLDocker(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ESPER_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("ESPER_MYSQL_DSN is not set")
	}
	dbConn, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer dbConn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := dbConn.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	const table = "esper_go_db_connector_test"
	if _, err := dbConn.ExecContext(ctx, "create table if not exists "+table+" (key1 varchar(64) primary key, value1 int)"); err != nil {
		t.Fatal(err)
	}
	defer dbConn.ExecContext(context.Background(), "drop table "+table)
	if _, err := dbConn.ExecContext(ctx, "delete from "+table); err != nil {
		t.Fatal(err)
	}
	sink, err := NewUpsertSink(UpsertSpec{
		Executor: dbConn,
		Table:    table,
		Keys:     []Column{{Column: "key1", Property: "key"}},
		Values:   []Column{{Column: "value1", Property: "value"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(ctx, Record{"key": "A", "value": 7}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(ctx, Record{"key": "A", "value": 8}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Destroy(); err != nil {
		t.Fatal(err)
	}
	var value int
	if err := dbConn.QueryRowContext(ctx, "select value1 from "+table+" where key1='A'").Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != 8 {
		t.Fatalf("MySQL upsert value = %d, want 8", value)
	}
}
