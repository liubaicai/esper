// Package db implements the database/sql portion of EsperIO DB in Go style.
// It deliberately accepts caller-owned *sql.DB, *sql.Tx, or compatible
// executors: connector lifecycle never commits, rolls back, or closes them.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/connectors"
)

// Executor is implemented by *sql.DB and *sql.Tx.
type Executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// Record is the map-backed value accepted by DB sinks.
type Record map[string]any

type preparer interface {
	PrepareContext(context.Context, string) (*sql.Stmt, error)
}

// Binding maps one SQL parameter to an event property. Position follows the
// Java EsperIO DB convention and is one-based. When all positions are zero,
// bindings are used in declaration order. Value can be supplied for a custom
// conversion and receives the original event value.
type Binding struct {
	Position int
	Property string
	Value    func(any) any
}

// DMLSpec describes a parameterized DML statement. Retry is the total number
// of attempts; zero means one attempt. RetryInterval is context-cancellable.
type DMLSpec struct {
	Executor         Executor
	Statement        string
	Bindings         []Binding
	Prepare          bool
	Retry            int
	RetryInterval    time.Duration
	Placeholder      func(index int) string
	WorkExecutor     WorkExecutor
	ExecutorName     string
	ExecutorServices *ExecutorServices
}

// DMLSink executes a DML statement for each event. It owns prepared
// statements, but never owns the executor.
type DMLSink struct {
	manager *connectors.StateManager
	spec    DMLSpec

	mu          sync.Mutex
	lifecycleMu sync.RWMutex
	stmt        *sql.Stmt
}

func NewDMLSink(spec DMLSpec) (*DMLSink, error) {
	if err := validateDMLSpec(spec); err != nil {
		return nil, err
	}
	workExecutor, err := resolveWorkExecutor(spec.WorkExecutor, spec.ExecutorName, spec.ExecutorServices)
	if err != nil {
		return nil, err
	}
	spec.Bindings = append([]Binding(nil), spec.Bindings...)
	spec.WorkExecutor = workExecutor
	return &DMLSink{manager: connectors.NewStateManager(), spec: spec}, nil
}

func validateDMLSpec(spec DMLSpec) error {
	if spec.Executor == nil {
		return fmt.Errorf("db: DML executor is required")
	}
	if strings.TrimSpace(spec.Statement) == "" {
		return fmt.Errorf("db: DML statement is required")
	}
	if spec.Retry < 0 {
		return fmt.Errorf("db: DML Retry cannot be negative")
	}
	if spec.RetryInterval < 0 {
		return fmt.Errorf("db: DML RetryInterval cannot be negative")
	}
	if spec.WorkExecutor != nil && strings.TrimSpace(spec.ExecutorName) != "" {
		return fmt.Errorf("db: DML WorkExecutor and ExecutorName are mutually exclusive")
	}
	if strings.TrimSpace(spec.ExecutorName) != "" && spec.ExecutorServices == nil {
		return fmt.Errorf("db: DML ExecutorServices is required for ExecutorName %q", spec.ExecutorName)
	}
	if spec.Prepare {
		if _, ok := spec.Executor.(preparer); !ok {
			return fmt.Errorf("db: DML executor does not support prepared statements")
		}
	}
	if _, err := normalizeBindings(spec.Bindings); err != nil {
		return err
	}
	return nil
}

func (s *DMLSink) State() connectors.State {
	if s == nil {
		return connectors.Destroyed
	}
	return s.manager.State()
}

func (s *DMLSink) Start() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Start(); err != nil {
		return err
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.spec.Prepare {
		s.mu.Lock()
		stmt, err := s.spec.Executor.(preparer).PrepareContext(context.Background(), s.spec.Statement)
		if err == nil {
			s.stmt = stmt
		}
		s.mu.Unlock()
		if err != nil {
			_ = s.manager.Stop()
			return fmt.Errorf("db: prepare DML statement: %w", err)
		}
	}
	return nil
}

func (s *DMLSink) Stop() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Stop(); err != nil {
		return err
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	return s.closeStatement()
}

func (s *DMLSink) Pause() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Pause()
}

func (s *DMLSink) Resume() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Resume()
}

func (s *DMLSink) Destroy() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Destroy(); err != nil {
		return err
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	return s.closeStatement()
}

// Write executes DML once for value. Values may be map-backed records,
// Esper Event/Row/TableRow, or public-field Go structs.
func (s *DMLSink) Write(ctx context.Context, value any) error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.manager.RequireStarted(); err != nil {
		return err
	}
	args, err := s.dmlArgs(value)
	if err != nil {
		return err
	}
	return s.executeWithLifecycle(ctx, args)
}

// WriteAsync submits one DML action to the configured work executor. With no
// work executor configured, the default same-thread executor runs it inline.
// Submission errors are returned immediately; SQL and retry errors are
// returned by Task.Wait.
func (s *DMLSink) WriteAsync(ctx context.Context, value any) (*Task, error) {
	if s == nil {
		return nil, connectors.ErrDestroyed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.manager.RequireStarted(); err != nil {
		return nil, err
	}
	args, err := s.dmlArgs(value)
	if err != nil {
		return nil, err
	}
	return s.spec.WorkExecutor.Submit(ctx, func(workCtx context.Context) error {
		return s.executeWithLifecycle(workCtx, args)
	})
}

func (s *DMLSink) dmlArgs(value any) ([]any, error) {
	record, err := valueRecord(value)
	if err != nil {
		return nil, err
	}
	bindings, err := normalizeBindings(s.spec.Bindings)
	if err != nil {
		return nil, err
	}
	args := make([]any, len(bindings))
	for index, binding := range bindings {
		if binding.Value != nil {
			args[index] = binding.Value(value)
			continue
		}
		args[index] = recordProperty(record, binding.Property)
	}
	return args, nil
}

func (s *DMLSink) WriteBatch(ctx context.Context, values ...any) error {
	for index, value := range values {
		if err := s.Write(ctx, value); err != nil {
			return fmt.Errorf("db: DML value %d: %w", index, err)
		}
	}
	return nil
}

func (s *DMLSink) executeWithLifecycle(ctx context.Context, args []any) error {
	s.lifecycleMu.RLock()
	defer s.lifecycleMu.RUnlock()
	if err := s.manager.RequireStarted(); err != nil {
		return err
	}
	return s.execute(ctx, args)
}

func (s *DMLSink) execute(ctx context.Context, args []any) error {
	attempts := s.spec.Retry
	if attempts == 0 {
		attempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.mu.Lock()
		stmt := s.stmt
		s.mu.Unlock()
		var err error
		if stmt != nil {
			_, err = stmt.ExecContext(ctx, args...)
		} else {
			_, err = s.spec.Executor.ExecContext(ctx, s.spec.Statement, args...)
		}
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt+1 >= attempts || s.spec.RetryInterval == 0 {
			continue
		}
		timer := time.NewTimer(s.spec.RetryInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("db: DML execution failed after %d attempt(s): %w", attempts, lastErr)
}

func (s *DMLSink) closeStatement() error {
	s.mu.Lock()
	stmt := s.stmt
	s.stmt = nil
	s.mu.Unlock()
	if stmt == nil {
		return nil
	}
	return stmt.Close()
}

// Column describes a key/value mapping for UpsertSink. Type is retained for
// Java configuration parity and diagnostics; database/sql receives the Go
// value directly, letting the driver perform the final conversion.
type Column struct {
	Column   string
	Property string
	Type     string
}

// UpsertSpec describes update-then-insert semantics for a table. Identifiers
// are validated before SQL is generated; values always remain parameters.
type UpsertSpec struct {
	Executor         Executor
	Table            string
	Keys             []Column
	Values           []Column
	Prepare          bool
	Retry            int
	RetryInterval    time.Duration
	Placeholder      func(index int) string
	WorkExecutor     WorkExecutor
	ExecutorName     string
	ExecutorServices *ExecutorServices
}

// UpsertSink implements the same key/value behavior as EsperIO's
// MultiKeyMultiValueTable plus RunnableUpsert.
type UpsertSink struct {
	manager *connectors.StateManager
	spec    UpsertSpec
	insert  string
	update  string

	mu          sync.Mutex
	lifecycleMu sync.RWMutex
	insertStmt  *sql.Stmt
	updateStmt  *sql.Stmt
}

func NewUpsertSink(spec UpsertSpec) (*UpsertSink, error) {
	if err := validateUpsertSpec(spec); err != nil {
		return nil, err
	}
	workExecutor, err := resolveWorkExecutor(spec.WorkExecutor, spec.ExecutorName, spec.ExecutorServices)
	if err != nil {
		return nil, err
	}
	spec.Keys = append([]Column(nil), spec.Keys...)
	spec.Values = append([]Column(nil), spec.Values...)
	spec.WorkExecutor = workExecutor
	placeholder := spec.Placeholder
	if placeholder == nil {
		placeholder = func(int) string { return "?" }
	}
	all := append(append([]Column(nil), spec.Keys...), spec.Values...)
	placeholders := make([]string, len(all))
	for index := range placeholders {
		placeholders[index] = placeholder(index + 1)
	}
	columns := make([]string, len(all))
	for index, column := range all {
		columns[index] = quoteIdentifier(column.Column)
	}
	keyColumns := make([]string, len(spec.Keys))
	for index, column := range spec.Keys {
		keyColumns[index] = quoteIdentifier(column.Column)
	}
	valueColumns := make([]string, len(spec.Values))
	for index, column := range spec.Values {
		valueColumns[index] = quoteIdentifier(column.Column)
	}
	where := make([]string, len(keyColumns))
	for index, column := range keyColumns {
		where[index] = column + "=" + placeholders[len(spec.Values)+index]
	}
	updateParts := make([]string, len(valueColumns))
	for index, column := range valueColumns {
		updateParts[index] = column + "=" + placeholders[index]
	}
	return &UpsertSink{
		manager: connectors.NewStateManager(),
		spec:    spec,
		insert:  "insert into " + quoteIdentifier(spec.Table) + " (" + strings.Join(columns, ", ") + ") values (" + strings.Join(placeholders, ", ") + ")",
		update:  "update " + quoteIdentifier(spec.Table) + " set " + strings.Join(updateParts, ", ") + " where " + strings.Join(where, " and "),
	}, nil
}

func validateUpsertSpec(spec UpsertSpec) error {
	if spec.Executor == nil {
		return fmt.Errorf("db: upsert executor is required")
	}
	if _, err := validateIdentifier(spec.Table); err != nil {
		return fmt.Errorf("db: upsert table: %w", err)
	}
	if len(spec.Keys) == 0 || len(spec.Values) == 0 {
		return fmt.Errorf("db: upsert requires at least one key and one value")
	}
	seen := make(map[string]struct{}, len(spec.Keys)+len(spec.Values))
	for _, column := range append(append([]Column(nil), spec.Keys...), spec.Values...) {
		if _, err := validateIdentifier(column.Column); err != nil {
			return fmt.Errorf("db: upsert column: %w", err)
		}
		if strings.TrimSpace(column.Property) == "" {
			return fmt.Errorf("db: upsert property is required for column %q", column.Column)
		}
		if _, exists := seen[column.Column]; exists {
			return fmt.Errorf("db: duplicate upsert column %q", column.Column)
		}
		seen[column.Column] = struct{}{}
	}
	if spec.Retry < 0 || spec.RetryInterval < 0 {
		return fmt.Errorf("db: upsert retry settings cannot be negative")
	}
	if spec.WorkExecutor != nil && strings.TrimSpace(spec.ExecutorName) != "" {
		return fmt.Errorf("db: upsert WorkExecutor and ExecutorName are mutually exclusive")
	}
	if strings.TrimSpace(spec.ExecutorName) != "" && spec.ExecutorServices == nil {
		return fmt.Errorf("db: upsert ExecutorServices is required for ExecutorName %q", spec.ExecutorName)
	}
	if spec.Prepare {
		if _, ok := spec.Executor.(preparer); !ok {
			return fmt.Errorf("db: upsert executor does not support prepared statements")
		}
	}
	return nil
}

func (s *UpsertSink) State() connectors.State {
	if s == nil {
		return connectors.Destroyed
	}
	return s.manager.State()
}

func (s *UpsertSink) Start() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Start(); err != nil {
		return err
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.spec.Prepare {
		preparer := s.spec.Executor.(preparer)
		insertStmt, err := preparer.PrepareContext(context.Background(), s.insert)
		if err == nil {
			var updateStmt *sql.Stmt
			updateStmt, err = preparer.PrepareContext(context.Background(), s.update)
			if err == nil {
				s.mu.Lock()
				s.insertStmt, s.updateStmt = insertStmt, updateStmt
				s.mu.Unlock()
			} else {
				_ = insertStmt.Close()
			}
		}
		if err != nil {
			_ = s.manager.Stop()
			return fmt.Errorf("db: prepare upsert statements: %w", err)
		}
	}
	return nil
}

func (s *UpsertSink) Stop() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Stop(); err != nil {
		return err
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	return s.closeStatements()
}

func (s *UpsertSink) Pause() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Pause()
}

func (s *UpsertSink) Resume() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Resume()
}

func (s *UpsertSink) Destroy() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Destroy(); err != nil {
		return err
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	return s.closeStatements()
}

func (s *UpsertSink) Write(ctx context.Context, value any) error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.manager.RequireStarted(); err != nil {
		return err
	}
	keys, values, err := s.upsertArgs(value)
	if err != nil {
		return err
	}
	return s.executeWithLifecycle(ctx, keys, values)
}

// WriteAsync submits one update-then-insert action to the configured work
// executor. SQL and retry errors are available from Task.Wait.
func (s *UpsertSink) WriteAsync(ctx context.Context, value any) (*Task, error) {
	if s == nil {
		return nil, connectors.ErrDestroyed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.manager.RequireStarted(); err != nil {
		return nil, err
	}
	keys, values, err := s.upsertArgs(value)
	if err != nil {
		return nil, err
	}
	return s.spec.WorkExecutor.Submit(ctx, func(workCtx context.Context) error {
		return s.executeWithLifecycle(workCtx, keys, values)
	})
}

func (s *UpsertSink) upsertArgs(value any) ([]any, []any, error) {
	record, err := valueRecord(value)
	if err != nil {
		return nil, nil, err
	}
	keys := make([]any, len(s.spec.Keys))
	values := make([]any, len(s.spec.Values))
	for index, column := range s.spec.Keys {
		keys[index] = recordProperty(record, column.Property)
	}
	for index, column := range s.spec.Values {
		values[index] = recordProperty(record, column.Property)
	}
	return keys, values, nil
}

func (s *UpsertSink) WriteBatch(ctx context.Context, values ...any) error {
	for index, value := range values {
		if err := s.Write(ctx, value); err != nil {
			return fmt.Errorf("db: upsert value %d: %w", index, err)
		}
	}
	return nil
}

func (s *UpsertSink) executeWithLifecycle(ctx context.Context, keys, values []any) error {
	s.lifecycleMu.RLock()
	defer s.lifecycleMu.RUnlock()
	if err := s.manager.RequireStarted(); err != nil {
		return err
	}
	args := append(append([]any(nil), values...), keys...)
	return s.execute(ctx, args, keys, values)
}

func (s *UpsertSink) execute(ctx context.Context, updateArgs, keys, values []any) error {
	attempts := s.spec.Retry
	if attempts == 0 {
		attempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		updated, err := s.execOne(ctx, s.update, updateArgs, true)
		if err == nil && updated {
			return nil
		}
		if err == nil {
			insertArgs := append(append([]any(nil), keys...), values...)
			_, err = s.execOne(ctx, s.insert, insertArgs, false)
		}
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt+1 >= attempts || s.spec.RetryInterval == 0 {
			continue
		}
		timer := time.NewTimer(s.spec.RetryInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("db: upsert failed after %d attempt(s): %w", attempts, lastErr)
}

func (s *UpsertSink) execOne(ctx context.Context, statement string, args []any, update bool) (bool, error) {
	s.mu.Lock()
	stmt := s.insertStmt
	if update {
		stmt = s.updateStmt
	}
	s.mu.Unlock()
	var (
		result sql.Result
		err    error
	)
	if stmt != nil {
		result, err = stmt.ExecContext(ctx, args...)
	} else {
		result, err = s.spec.Executor.ExecContext(ctx, statement, args...)
	}
	if err != nil {
		return false, err
	}
	if !update {
		return true, nil
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (s *UpsertSink) closeStatements() error {
	s.mu.Lock()
	insertStmt, updateStmt := s.insertStmt, s.updateStmt
	s.insertStmt, s.updateStmt = nil, nil
	s.mu.Unlock()
	var firstErr error
	if insertStmt != nil {
		firstErr = insertStmt.Close()
	}
	if updateStmt != nil {
		if err := updateStmt.Close(); firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func normalizeBindings(bindings []Binding) ([]Binding, error) {
	result := append([]Binding(nil), bindings...)
	positions := false
	for _, binding := range result {
		if binding.Position != 0 {
			positions = true
			break
		}
	}
	if !positions {
		for index, binding := range result {
			if binding.Value == nil && strings.TrimSpace(binding.Property) == "" {
				return nil, fmt.Errorf("db: binding %d requires Property or Value", index)
			}
		}
		return result, nil
	}
	sort.SliceStable(result, func(left, right int) bool { return result[left].Position < result[right].Position })
	for index, binding := range result {
		if binding.Position != index+1 {
			return nil, fmt.Errorf("db: binding positions must be contiguous starting at 1")
		}
		if binding.Value == nil && strings.TrimSpace(binding.Property) == "" {
			return nil, fmt.Errorf("db: binding %d requires Property or Value", index)
		}
	}
	return result, nil
}

func validateIdentifier(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("identifier is required")
	}
	for index, part := range strings.Split(value, ".") {
		if part == "" {
			return "", fmt.Errorf("identifier %q contains an empty segment", value)
		}
		for charIndex, character := range part {
			if charIndex == 0 && !(character == '_' || unicode.IsLetter(character)) {
				return "", fmt.Errorf("identifier %q is not safe", value)
			}
			if charIndex > 0 && !(character == '_' || unicode.IsLetter(character) || unicode.IsDigit(character)) {
				return "", fmt.Errorf("identifier %q is not safe", value)
			}
		}
		_ = index
	}
	return value, nil
}

func quoteIdentifier(value string) string { return value }

func recordProperty(record map[string]any, property string) any {
	parts := strings.Split(property, ".")
	var current any = record
	for _, part := range parts {
		switch typed := current.(type) {
		case map[string]any:
			current = typed[part]
		default:
			value := reflect.ValueOf(current)
			for value.IsValid() && value.Kind() == reflect.Pointer {
				if value.IsNil() {
					return nil
				}
				value = value.Elem()
			}
			if !value.IsValid() || value.Kind() != reflect.Struct {
				return nil
			}
			field := value.FieldByName(part)
			if !field.IsValid() || !field.CanInterface() {
				return nil
			}
			current = field.Interface()
		}
	}
	return current
}

func valueRecord(value any) (map[string]any, error) {
	if value == nil {
		return nil, fmt.Errorf("db: event value is nil")
	}
	switch typed := value.(type) {
	case Record:
		return cloneRecord(map[string]any(typed)), nil
	case map[string]any:
		return cloneRecord(typed), nil
	case esper.Event:
		record := make(map[string]any, len(typed.Schema().Fields()))
		for _, field := range typed.Schema().Fields() {
			property := typed.Get(field.Name)
			if property.IsPresent() || property.IsNull() {
				record[field.Name] = property.Any()
			}
		}
		return record, nil
	case esper.Row:
		return typed.AsMap(), nil
	case esper.TableRow:
		record := make(map[string]any)
		for name, property := range typed.Values() {
			record[name] = property.Any()
		}
		return record, nil
	}
	reflected := reflect.ValueOf(value)
	for reflected.Kind() == reflect.Pointer {
		if reflected.IsNil() {
			return nil, fmt.Errorf("db: event value is a nil pointer")
		}
		reflected = reflected.Elem()
	}
	if reflected.Kind() != reflect.Struct {
		return nil, fmt.Errorf("db: unsupported event value %T", value)
	}
	result := make(map[string]any)
	typ := reflected.Type()
	for index := 0; index < reflected.NumField(); index++ {
		field := typ.Field(index)
		if field.PkgPath != "" || !reflected.Field(index).CanInterface() {
			continue
		}
		name := field.Tag.Get("esper")
		if name == "" {
			name = field.Tag.Get("json")
		}
		if comma := strings.IndexByte(name, ','); comma >= 0 {
			name = name[:comma]
		}
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		result[name] = reflected.Field(index).Interface()
	}
	return result, nil
}

func cloneRecord(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for name, item := range value {
		result[name] = item
	}
	return result
}
