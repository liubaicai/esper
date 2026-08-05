package esper

import (
	"context"
	"database/sql"
	"reflect"
	"sync"
	"testing"
	"time"
)

type recordingSQLExecutor struct {
	mu         sync.Mutex
	statements []string
	arguments  [][]any
}

func (e *recordingSQLExecutor) ExecContext(ctx context.Context, statement string, arguments ...any) (sql.Result, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	e.mu.Lock()
	e.statements = append(e.statements, statement)
	e.arguments = append(e.arguments, append([]any(nil), arguments...))
	e.mu.Unlock()
	return driverResult(1), nil
}

type driverResult int64

func (r driverResult) LastInsertId() (int64, error) { return 0, nil }
func (r driverResult) RowsAffected() (int64, error) { return int64(r), nil }

func TestSQLSinkWritesNewAndOldResults(t *testing.T) {
	schema, err := NewMapSchema("SQLSinkEvent", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("value", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := newEvent(schema, map[string]any{"symbol": "A", "value": 1}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	second, err := newEvent(schema, map[string]any{"symbol": "B", "value": 2}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	executor := &recordingSQLExecutor{}
	sink, err := NewSQLSinkWithExecutor(executor, "insert into sink_table(symbol, value, optional) values (?, ?, ?)", SQLSinkOptions{
		Arguments: []func(Result) any{
			SQLResultField("symbol"),
			SQLResultField("value"),
			SQLResultField("missing"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(context.Background(), ResultBatch{
		New: []Result{resultEvent(first)},
		Old: []Result{resultEvent(second)},
	}); err != nil {
		t.Fatal(err)
	}
	executor.mu.Lock()
	statements := append([]string(nil), executor.statements...)
	arguments := append([][]any(nil), executor.arguments...)
	executor.mu.Unlock()
	if len(statements) != 2 || statements[0] != statements[1] || len(arguments) != 2 {
		t.Fatalf("SQL sink calls statements=%#v arguments=%#v", statements, arguments)
	}
	if arguments[0][0] != "A" || arguments[0][1] != 1 || arguments[0][2] != nil || arguments[1][0] != "B" || arguments[1][1] != 2 {
		t.Fatalf("SQL sink arguments=%#v", arguments)
	}
}

func TestSQLSinkRejectsClosedAndCanceledWrites(t *testing.T) {
	executor := &recordingSQLExecutor{}
	sink, err := NewSQLSinkWithExecutor(executor, "update sink_table set value = ?", SQLSinkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sink.Write(ctx, ResultBatch{}); err == nil {
		t.Fatal("canceled SQL sink write unexpectedly succeeded")
	}
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(context.Background(), ResultBatch{}); err == nil {
		t.Fatal("closed SQL sink write unexpectedly succeeded")
	}
	if _, err := NewSQLSinkWithExecutor(executor, "select 1", SQLSinkOptions{PrepareStatement: true}); err == nil {
		t.Fatal("non-preparable SQL executor unexpectedly accepted prepared sink")
	}
}
