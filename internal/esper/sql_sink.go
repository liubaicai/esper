package esper

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
)

// SQLExecContext is implemented by *sql.DB and *sql.Tx. A caller-owned
// transaction can therefore receive a ResultBatch without the sink deciding
// its commit or rollback policy.
type SQLExecContext interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type sqlSinkPreparer interface {
	PrepareContext(context.Context, string) (*sql.Stmt, error)
}

// SQLSinkOptions configures a parameterized database/sql result sink.
type SQLSinkOptions struct {
	Arguments        []func(Result) any
	PrepareStatement bool
}

// SQLSink writes new-stream results followed by old-stream results to a
// parameterized database/sql statement. It is a normal Sink and can be
// attached with Stream.To(NewSQLSink(...)). The caller owns the executor.
type SQLSink struct {
	DB        *sql.DB
	Statement string
	Arguments []func(Result) any

	executor SQLExecContext
	prepare  bool
	stmt     *sql.Stmt
	mu       sync.Mutex
	closed   bool
}

// NewSQLSink constructs a SQL sink over a database connection pool.
func NewSQLSink(db *sql.DB, statement string, arguments ...func(Result) any) (*SQLSink, error) {
	return NewSQLSinkWithOptions(db, statement, SQLSinkOptions{Arguments: arguments})
}

// NewSQLSinkWithOptions constructs a SQL sink with optional lazy prepared
// statement reuse.
func NewSQLSinkWithOptions(db *sql.DB, statement string, options SQLSinkOptions) (*SQLSink, error) {
	return NewSQLSinkWithExecutor(db, statement, options)
}

// NewSQLSinkWithExecutor constructs a SQL sink over *sql.DB or *sql.Tx. The
// sink never commits, rolls back, or closes the executor.
func NewSQLSinkWithExecutor(executor SQLExecContext, statement string, options SQLSinkOptions) (*SQLSink, error) {
	if executor == nil {
		return nil, NewError(ErrorDependency, "SQL sink requires an executor")
	}
	if strings.TrimSpace(statement) == "" {
		return nil, NewError(ErrorInvalidRule, "SQL sink statement is required")
	}
	if options.PrepareStatement {
		if _, ok := executor.(sqlSinkPreparer); !ok {
			return nil, NewError(ErrorDependency, "SQL sink executor does not support prepared statements")
		}
	}
	var db *sql.DB
	if typedDB, ok := executor.(*sql.DB); ok {
		db = typedDB
	}
	return &SQLSink{
		DB:        db,
		Statement: statement,
		Arguments: append([]func(Result) any(nil), options.Arguments...),
		executor:  executor,
		prepare:   options.PrepareStatement,
	}, nil
}

// SQLResultField binds a named result field to a SQL argument. Missing and
// null fields are passed as nil, matching database/sql NULL input behavior.
func SQLResultField(name string) func(Result) any {
	return func(result Result) any {
		value := result.Get(name)
		if !value.IsPresent() || value.IsNull() {
			return nil
		}
		return value.Any()
	}
}

// Write executes the statement once for each new result and then each old
// result in the batch. An error identifies the stream side and row index.
func (s *SQLSink) Write(ctx context.Context, batch ResultBatch) error {
	if s == nil {
		return NewError(ErrorDependency, "SQL sink is nil")
	}
	if err := contextErr(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return NewError(ErrorDependency, "SQL sink is closed")
	}
	stmt, err := s.statementLocked(ctx)
	executor := s.executor
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if executor == nil && stmt == nil {
		return NewError(ErrorDependency, "SQL sink has no executor")
	}
	write := func(side string, index int, result Result) error {
		args := make([]any, 0, len(s.Arguments))
		for _, argument := range s.Arguments {
			if argument == nil {
				args = append(args, nil)
				continue
			}
			args = append(args, argument(result))
		}
		if stmt != nil {
			if _, err := stmt.ExecContext(ctx, args...); err != nil {
				return fmt.Errorf("esper: SQL sink %s result %d: %w", side, index, err)
			}
			return nil
		}
		if _, err := executor.ExecContext(ctx, s.Statement, args...); err != nil {
			return fmt.Errorf("esper: SQL sink %s result %d: %w", side, index, err)
		}
		return nil
	}
	for index, result := range batch.New {
		if err := write("new", index, result); err != nil {
			return err
		}
	}
	for index, result := range batch.Old {
		if err := write("old", index, result); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLSink) statementLocked(ctx context.Context) (*sql.Stmt, error) {
	if !s.prepare {
		return nil, nil
	}
	if s.stmt != nil {
		return s.stmt, nil
	}
	preparer, ok := s.executor.(sqlSinkPreparer)
	if !ok {
		return nil, NewError(ErrorDependency, "SQL sink executor does not support prepared statements")
	}
	statement, err := preparer.PrepareContext(ctx, s.Statement)
	if err != nil {
		return nil, err
	}
	s.stmt = statement
	return statement, nil
}

// Close releases a lazily prepared statement. The caller still owns DB/Tx.
func (s *SQLSink) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.closed = true
	stmt := s.stmt
	s.stmt = nil
	s.mu.Unlock()
	if stmt == nil {
		return nil
	}
	return stmt.Close()
}
