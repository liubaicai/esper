package esper

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

// TableColumn describes a table column. Primary-key columns are populated
// from the row values and participate in Get/Delete/Upsert identity.
type TableColumn struct {
	Name       string
	Type       reflect.Type
	Optional   bool
	PrimaryKey bool
}

func TableColumnOf[T any](name string) TableColumn {
	return TableColumn{Name: name, Type: typeOf[T]()}
}

// OptionalTableColumnOf declares a nullable table column. It is useful for
// dimensional aggregate materializations where subtotal rows intentionally
// carry Null for dimensions that are not present in that grouping set.
func OptionalTableColumnOf[T any](name string) TableColumn {
	return TableColumn{Name: name, Type: typeOf[T](), Optional: true}
}

func PrimaryKeyColumn[T any](name string) TableColumn {
	return TableColumn{Name: name, Type: typeOf[T](), PrimaryKey: true}
}

type TableIndexDefinition struct {
	Name    string
	Columns []string
	Unique  bool
}

type TableOption func(*tableConfig)

func SecondaryIndex(name string, columns ...string) TableOption {
	return func(config *tableConfig) {
		config.indexes = append(config.indexes, TableIndexDefinition{Name: name, Columns: append([]string(nil), columns...)})
	}
}

func UniqueIndex(name string, columns ...string) TableOption {
	return func(config *tableConfig) {
		config.indexes = append(config.indexes, TableIndexDefinition{Name: name, Columns: append([]string(nil), columns...), Unique: true})
	}
}

type tableConfig struct {
	indexes []TableIndexDefinition
}

// TableDefinition is an immutable compile-time table declaration.
type TableDefinition struct {
	name       string
	columns    []TableColumn
	schema     Schema
	primaryKey []string
	indexes    []TableIndexDefinition
}

func NewTableDefinition(name string, columns []TableColumn, options ...TableOption) (TableDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return TableDefinition{}, NewError(ErrorInvalidRule, "table name is required")
	}
	if len(columns) == 0 {
		return TableDefinition{}, NewError(ErrorInvalidRule, "table requires at least one column")
	}
	fields := make([]FieldSpec, 0, len(columns))
	copyColumns := make([]TableColumn, len(columns))
	seen := make(map[string]struct{}, len(columns))
	primaryKey := make([]string, 0)
	for index, column := range columns {
		column.Name = strings.TrimSpace(column.Name)
		if column.Name == "" {
			return TableDefinition{}, NewError(ErrorInvalidRule, "table column name is required")
		}
		if _, exists := seen[column.Name]; exists {
			return TableDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("table duplicates column %q", column.Name))
		}
		seen[column.Name] = struct{}{}
		if column.Type == nil {
			column.Type = typeOf[any]()
		}
		copyColumns[index] = column
		fields = append(fields, FieldSpec{Name: column.Name, Type: column.Type, Optional: column.Optional})
		if column.PrimaryKey {
			primaryKey = append(primaryKey, column.Name)
		}
	}
	config := tableConfig{}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	for _, index := range config.indexes {
		if strings.TrimSpace(index.Name) == "" || len(index.Columns) == 0 {
			return TableDefinition{}, NewError(ErrorInvalidRule, "table index requires a name and columns")
		}
		for _, column := range index.Columns {
			if _, exists := seen[column]; !exists {
				return TableDefinition{}, NewError(ErrorUnknownName, fmt.Sprintf("table index %q references unknown column %q", index.Name, column))
			}
		}
	}
	schema, err := NewMapSchema("table:"+name, fields)
	if err != nil {
		return TableDefinition{}, err
	}
	return TableDefinition{
		name:       name,
		columns:    copyColumns,
		schema:     schema,
		primaryKey: primaryKey,
		indexes:    append([]TableIndexDefinition(nil), config.indexes...),
	}, nil
}

func (d TableDefinition) Name() string           { return d.name }
func (d TableDefinition) Schema() Schema         { return d.schema }
func (d TableDefinition) Columns() []TableColumn { return append([]TableColumn(nil), d.columns...) }
func (d TableDefinition) PrimaryKey() []string   { return append([]string(nil), d.primaryKey...) }
func (d TableDefinition) Indexes() []TableIndexDefinition {
	result := append([]TableIndexDefinition(nil), d.indexes...)
	for index := range result {
		result[index].Columns = append([]string(nil), result[index].Columns...)
	}
	return result
}

func (e *Environment) RegisterTable(name string, columns []TableColumn, options ...TableOption) (TableDefinition, error) {
	definition, err := NewTableDefinition(name, columns, options...)
	if err != nil {
		return TableDefinition{}, err
	}
	if e == nil {
		return TableDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.tables[name]; exists {
		return TableDefinition{}, NewError(ErrorDependency, fmt.Sprintf("table %q is already registered", name))
	}
	e.tables[name] = definition
	return definition, nil
}

func CreateTable(env *Environment, name string, columns []TableColumn, options ...TableOption) (TableDefinition, error) {
	if env == nil {
		return TableDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	return env.RegisterTable(name, columns, options...)
}

func (e *Environment) Table(name string) (TableDefinition, bool) {
	if e == nil {
		return TableDefinition{}, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	definition, ok := e.tables[name]
	return definition, ok
}

type TableRow struct {
	values  map[string]Value
	version uint64
}

func (r TableRow) Get(name string) Value {
	if r.values == nil {
		return Missing()
	}
	value, ok := r.values[name]
	if !ok {
		return Missing()
	}
	return value
}

func (r TableRow) Values() map[string]Value {
	result := make(map[string]Value, len(r.values))
	for name, value := range r.values {
		result[name] = value
	}
	return result
}

func (r TableRow) Version() uint64 { return r.version }

type tableState struct {
	mu      sync.RWMutex
	def     TableDefinition
	rows    map[string]TableRow
	order   []string
	indexes map[string]map[string][]string
	version uint64
}

// Table is a concurrency-safe in-memory table owned by an Engine. Reads use
// a consistent snapshot and writes update all configured indexes atomically.
type Table struct {
	state *tableState
}

func newTable(definition TableDefinition) *Table {
	indexes := make(map[string]map[string][]string, len(definition.indexes))
	for _, index := range definition.indexes {
		indexes[index.Name] = make(map[string][]string)
	}
	return &Table{state: &tableState{def: definition, rows: make(map[string]TableRow), indexes: indexes}}
}

func (t *Table) Definition() TableDefinition {
	if t == nil || t.state == nil {
		return TableDefinition{}
	}
	return t.state.def
}

func (t *Table) Upsert(ctx context.Context, values map[string]any) (TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return TableRow{}, err
	}
	if t == nil || t.state == nil {
		return TableRow{}, NewError(ErrorState, "nil table")
	}
	state := t.state
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.upsert(values, false)
}

// Replace atomically replaces the complete table snapshot. It is used by
// materialized aggregate statements so a group removal cannot leave stale
// rows behind, and a failed conversion or cancelled context cannot partially
// update the table.
func (t *Table) Replace(ctx context.Context, rows []map[string]any) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if t == nil || t.state == nil {
		return NewError(ErrorState, "nil table")
	}
	state := t.state
	state.mu.Lock()
	defer state.mu.Unlock()

	replacement := &tableState{
		def:     state.def,
		rows:    make(map[string]TableRow, len(rows)),
		indexes: make(map[string]map[string][]string, len(state.def.indexes)),
		version: state.version,
	}
	for _, definition := range state.def.indexes {
		replacement.indexes[definition.Name] = make(map[string][]string)
	}
	for index, values := range rows {
		if err := contextErr(ctx); err != nil {
			return err
		}
		if _, err := replacement.upsert(values, false); err != nil {
			return WrapError(ErrorState, fmt.Sprintf("table replacement row %d", index), err)
		}
	}
	state.rows = replacement.rows
	state.order = replacement.order
	state.indexes = replacement.indexes
	state.version = replacement.version
	return nil
}

func (t *Table) Insert(ctx context.Context, values map[string]any) (TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return TableRow{}, err
	}
	if t == nil || t.state == nil {
		return TableRow{}, NewError(ErrorState, "nil table")
	}
	state := t.state
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.upsert(values, true)
}

func (t *Table) Update(ctx context.Context, key []any, values map[string]any) (TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return TableRow{}, err
	}
	if t == nil || t.state == nil {
		return TableRow{}, NewError(ErrorState, "nil table")
	}
	state := t.state
	state.mu.Lock()
	defer state.mu.Unlock()
	rowKey, err := state.keyFromValues(key)
	if err != nil {
		return TableRow{}, err
	}
	existing, ok := state.rows[rowKey]
	if !ok {
		return TableRow{}, NewError(ErrorUnknownName, "table row does not exist")
	}
	for _, primaryKey := range state.def.primaryKey {
		if value, changes := values[primaryKey]; changes {
			converted, convertErr := coerceTableValue(value, state.column(primaryKey).Type)
			if convertErr != nil {
				return TableRow{}, WrapError(ErrorTypeMismatch, "table."+primaryKey, convertErr)
			}
			if !existing.Get(primaryKey).Equal(converted) {
				return TableRow{}, NewError(ErrorState, "table primary-key update is not allowed")
			}
		}
	}
	merged := make(map[string]any, len(existing.values))
	for name, value := range existing.values {
		merged[name] = value.Any()
	}
	for name, value := range values {
		merged[name] = value
	}
	return state.upsert(merged, false)
}

func (t *Table) Delete(ctx context.Context, key ...any) (TableRow, bool, error) {
	if err := contextErr(ctx); err != nil {
		return TableRow{}, false, err
	}
	if t == nil || t.state == nil {
		return TableRow{}, false, NewError(ErrorState, "nil table")
	}
	state := t.state
	state.mu.Lock()
	defer state.mu.Unlock()
	rowKey, err := state.keyFromValues(key)
	if err != nil {
		return TableRow{}, false, err
	}
	row, ok := state.rows[rowKey]
	if !ok {
		return TableRow{}, false, nil
	}
	state.removeLocked(rowKey, row)
	return row, true, nil
}

func (t *Table) Get(ctx context.Context, key ...any) (TableRow, bool, error) {
	if err := contextErr(ctx); err != nil {
		return TableRow{}, false, err
	}
	if t == nil || t.state == nil {
		return TableRow{}, false, NewError(ErrorState, "nil table")
	}
	state := t.state
	state.mu.RLock()
	defer state.mu.RUnlock()
	rowKey, err := state.keyFromValues(key)
	if err != nil {
		return TableRow{}, false, err
	}
	row, ok := state.rows[rowKey]
	return cloneTableRow(row), ok, nil
}

func (t *Table) Snapshot(ctx context.Context) ([]TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if t == nil || t.state == nil {
		return nil, NewError(ErrorState, "nil table")
	}
	state := t.state
	state.mu.RLock()
	defer state.mu.RUnlock()
	rows := make([]TableRow, 0, len(state.order))
	for _, key := range state.order {
		if row, ok := state.rows[key]; ok {
			rows = append(rows, cloneTableRow(row))
		}
	}
	return rows, nil
}

func (t *Table) Clear(ctx context.Context) ([]TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if t == nil || t.state == nil {
		return nil, NewError(ErrorState, "nil table")
	}
	state := t.state
	state.mu.Lock()
	defer state.mu.Unlock()
	rows := make([]TableRow, 0, len(state.order))
	for _, key := range state.order {
		if row, ok := state.rows[key]; ok {
			rows = append(rows, cloneTableRow(row))
		}
	}
	state.rows = make(map[string]TableRow)
	state.order = nil
	for name := range state.indexes {
		state.indexes[name] = make(map[string][]string)
	}
	state.version++
	return rows, nil
}

func (t *Table) Lookup(ctx context.Context, indexName string, key ...any) ([]TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if t == nil || t.state == nil {
		return nil, NewError(ErrorState, "nil table")
	}
	state := t.state
	state.mu.RLock()
	defer state.mu.RUnlock()
	index, ok := state.indexes[indexName]
	if !ok {
		return nil, NewError(ErrorUnknownName, fmt.Sprintf("table index %q does not exist", indexName))
	}
	indexKey := encodeKey(key)
	rowKeys := index[indexKey]
	rows := make([]TableRow, 0, len(rowKeys))
	for _, rowKey := range rowKeys {
		if row, exists := state.rows[rowKey]; exists {
			rows = append(rows, cloneTableRow(row))
		}
	}
	return rows, nil
}

func (s *tableState) upsert(values map[string]any, insertOnly bool) (TableRow, error) {
	if values == nil {
		values = map[string]any{}
	}
	converted := make(map[string]Value, len(s.def.columns))
	known := make(map[string]struct{}, len(s.def.columns))
	for _, column := range s.def.columns {
		known[column.Name] = struct{}{}
		value, exists := values[column.Name]
		if !exists {
			if column.PrimaryKey || !column.Optional {
				return TableRow{}, NewError(ErrorTypeMismatch, fmt.Sprintf("table column %q is required", column.Name))
			}
			converted[column.Name] = Null()
			continue
		}
		convertedValue, err := coerceTableValue(value, column.Type)
		if err != nil {
			return TableRow{}, WrapError(ErrorTypeMismatch, "table."+column.Name, err)
		}
		converted[column.Name] = convertedValue
	}
	for name := range values {
		if _, exists := known[name]; !exists {
			return TableRow{}, NewError(ErrorUnknownName, fmt.Sprintf("table column %q is not defined", name))
		}
	}
	keyValues := make([]any, 0, len(s.def.primaryKey))
	for _, name := range s.def.primaryKey {
		value := converted[name]
		if !value.IsPresent() {
			return TableRow{}, NewError(ErrorTypeMismatch, fmt.Sprintf("primary-key column %q cannot be null", name))
		}
		keyValues = append(keyValues, value.Any())
	}
	rowKey := encodeKey(keyValues)
	if _, exists := s.rows[rowKey]; exists && insertOnly {
		return TableRow{}, NewError(ErrorState, "table row already exists")
	}
	row := TableRow{values: converted, version: s.version + 1}
	if err := s.validateIndexesLocked(rowKey, row); err != nil {
		return TableRow{}, err
	}
	if old, exists := s.rows[rowKey]; exists {
		s.removeIndexesLocked(rowKey, old)
	} else {
		s.order = append(s.order, rowKey)
	}
	s.version++
	row.version = s.version
	s.rows[rowKey] = row
	s.addIndexesLocked(rowKey, row)
	return cloneTableRow(row), nil
}

func (s *tableState) keyFromValues(key []any) (string, error) {
	if len(key) != len(s.def.primaryKey) {
		return "", NewError(ErrorTypeMismatch, fmt.Sprintf("table key expects %d values, got %d", len(s.def.primaryKey), len(key)))
	}
	convertedKey := make([]any, 0, len(key))
	for index, value := range key {
		column := s.column(s.def.primaryKey[index])
		converted, err := coerceTableValue(value, column.Type)
		if err != nil {
			return "", WrapError(ErrorTypeMismatch, "table.key."+column.Name, err)
		}
		if !converted.IsPresent() {
			return "", NewError(ErrorTypeMismatch, "table key cannot be null")
		}
		convertedKey = append(convertedKey, converted.Any())
	}
	return encodeKey(convertedKey), nil
}

func (s *tableState) column(name string) TableColumn {
	for _, column := range s.def.columns {
		if column.Name == name {
			return column
		}
	}
	return TableColumn{Name: name, Type: typeOf[any]()}
}

func (s *tableState) removeLocked(rowKey string, row TableRow) {
	delete(s.rows, rowKey)
	for index, key := range s.order {
		if key == rowKey {
			s.order = append(s.order[:index], s.order[index+1:]...)
			break
		}
	}
	s.removeIndexesLocked(rowKey, row)
	s.version++
}

func (s *tableState) validateIndexesLocked(rowKey string, row TableRow) error {
	for _, definition := range s.def.indexes {
		if !definition.Unique {
			continue
		}
		key := s.indexKey(row, definition.Columns)
		for _, existing := range s.indexes[definition.Name][key] {
			if existing != rowKey {
				return NewError(ErrorState, fmt.Sprintf("unique table index %q rejected duplicate key", definition.Name))
			}
		}
	}
	return nil
}

func (s *tableState) addIndexesLocked(rowKey string, row TableRow) {
	for _, definition := range s.def.indexes {
		key := s.indexKey(row, definition.Columns)
		index := s.indexes[definition.Name]
		if !definition.Unique || len(index[key]) == 0 {
			index[key] = append(index[key], rowKey)
		}
	}
}

func (s *tableState) removeIndexesLocked(rowKey string, row TableRow) {
	for _, definition := range s.def.indexes {
		key := s.indexKey(row, definition.Columns)
		index := s.indexes[definition.Name]
		rowKeys := index[key]
		for position, existing := range rowKeys {
			if existing == rowKey {
				rowKeys = append(rowKeys[:position], rowKeys[position+1:]...)
				break
			}
		}
		if len(rowKeys) == 0 {
			delete(index, key)
		} else {
			index[key] = rowKeys
		}
	}
}

func (s *tableState) indexKey(row TableRow, columns []string) string {
	values := make([]any, 0, len(columns))
	for _, column := range columns {
		values = append(values, row.Get(column).Any())
	}
	return encodeKey(values)
}

func cloneTableRow(row TableRow) TableRow {
	return TableRow{values: row.Values(), version: row.version}
}

func coerceTableValue(value any, target reflect.Type) (Value, error) {
	if value == nil {
		return Null(), nil
	}
	if target == nil || target == typeOf[any]() {
		return Present(value), nil
	}
	got := reflect.TypeOf(value)
	if got.AssignableTo(target) {
		return Present(value), nil
	}
	if numericTypes(got, target) {
		converted := reflect.New(target).Elem()
		converted.Set(reflect.ValueOf(value).Convert(target))
		return Present(converted.Interface()), nil
	}
	return Value{}, fmt.Errorf("expects %s, got %s", target, got)
}

func encodeKey(values []any) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprintf("%T:%#v", value, value))
	}
	return strings.Join(parts, "\x1f")
}

// NamedWindowDefinition describes a named event store and its optional
// retention policy. A named window has one immutable event schema and can have
// multiple consumers.
type NamedWindowDefinition struct {
	name      string
	schema    Schema
	retention WindowSpec
}

type NamedWindowOption func(*namedWindowConfig)

func NamedWindowRetention(window WindowSpec) NamedWindowOption {
	return func(config *namedWindowConfig) { config.retention = window }
}

type namedWindowConfig struct {
	retention WindowSpec
}

func NewNamedWindowDefinition(name string, schema Schema, options ...NamedWindowOption) (NamedWindowDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return NamedWindowDefinition{}, NewError(ErrorInvalidRule, "named-window name is required")
	}
	if !schema.valid() {
		return NamedWindowDefinition{}, NewError(ErrorDependency, "named-window schema is invalid")
	}
	config := namedWindowConfig{retention: KeepAll()}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	if config.retention == nil {
		return NamedWindowDefinition{}, NewError(ErrorInvalidRule, "named-window retention is required")
	}
	if err := config.retention.validate(); err != nil {
		return NamedWindowDefinition{}, err
	}
	return NamedWindowDefinition{name: name, schema: schema, retention: config.retention}, nil
}

func (d NamedWindowDefinition) Name() string          { return d.name }
func (d NamedWindowDefinition) Schema() Schema        { return d.schema }
func (d NamedWindowDefinition) Retention() WindowSpec { return d.retention }

func (e *Environment) RegisterNamedWindow(name string, schema Schema, options ...NamedWindowOption) (NamedWindowDefinition, error) {
	definition, err := NewNamedWindowDefinition(name, schema, options...)
	if err != nil {
		return NamedWindowDefinition{}, err
	}
	if e == nil {
		return NamedWindowDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.namedWindows[name]; exists {
		return NamedWindowDefinition{}, NewError(ErrorDependency, fmt.Sprintf("named window %q is already registered", name))
	}
	e.namedWindows[name] = definition
	return definition, nil
}

func CreateNamedWindow(env *Environment, name string, schema Schema, options ...NamedWindowOption) (NamedWindowDefinition, error) {
	if env == nil {
		return NamedWindowDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	return env.RegisterNamedWindow(name, schema, options...)
}

func (e *Environment) NamedWindow(name string) (NamedWindowDefinition, bool) {
	if e == nil {
		return NamedWindowDefinition{}, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	definition, ok := e.namedWindows[name]
	return definition, ok
}

type NamedWindowDelta struct {
	New  []Event
	Old  []Event
	Time time.Time
}

func (d NamedWindowDelta) empty() bool { return len(d.New) == 0 && len(d.Old) == 0 }

type NamedWindowListener func(context.Context, NamedWindowDelta) error

type namedWindowRuntime struct {
	mu        sync.RWMutex
	def       NamedWindowDefinition
	entries   []storedEvent
	keyed     map[string]storedEvent
	keyOrder  []string
	listeners map[uint64]NamedWindowListener
	nextID    uint64
}

type NamedWindow struct {
	state  *namedWindowRuntime
	engine *Engine
}

func newNamedWindow(definition NamedWindowDefinition, engine *Engine) *NamedWindow {
	state := &namedWindowRuntime{def: definition, listeners: make(map[uint64]NamedWindowListener)}
	if _, unique := definition.retention.(UniqueWindowSpec); unique {
		state.keyed = make(map[string]storedEvent)
	}
	if retention, sorted := definition.retention.(SortedWindowSpec); sorted && retention.Rank && len(retention.UniqueKeys) > 0 {
		state.keyed = make(map[string]storedEvent)
	}
	return &NamedWindow{state: state, engine: engine}
}

func normalizeSortedNamedWindowEntries(entries []storedEvent, retention SortedWindowSpec, now time.Time) ([]storedEvent, map[string]storedEvent) {
	normalized := append([]storedEvent(nil), entries...)
	var keyed map[string]storedEvent
	if retention.Rank && len(retention.UniqueKeys) > 0 {
		keyed = make(map[string]storedEvent, len(normalized))
		positions := make(map[string]int, len(normalized))
		deduplicated := make([]storedEvent, 0, len(normalized))
		for _, entry := range normalized {
			key := sortedWindowKey(retention, entry.event, now, nil)
			if position, exists := positions[key]; exists {
				// A direct update can change one event's key to another event's
				// key. Keep the later entry, matching rank replacement semantics.
				deduplicated[position] = entry
				continue
			}
			positions[key] = len(deduplicated)
			deduplicated = append(deduplicated, entry)
		}
		normalized = deduplicated
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		return compareStoredEvents(normalized[i].event, normalized[j].event, retention.Keys, now, nil) < 0
	})
	if !retention.Rank {
		reverseSortedEqualRuns(normalized, retention.Keys, now, nil)
	}
	if retention.Rank && len(retention.UniqueKeys) > 0 {
		keyed = make(map[string]storedEvent, len(normalized))
		for _, entry := range normalized {
			keyed[sortedWindowKey(retention, entry.event, now, nil)] = entry
		}
	}
	return normalized, keyed
}

func applySortedNamedWindowInsertLocked(state *namedWindowRuntime, retention SortedWindowSpec, entry storedEvent, now time.Time, delta *NamedWindowDelta) {
	entries, keyed := normalizeSortedNamedWindowEntries(state.entries, retention, now)
	var uniqueKey string
	if retention.Rank && len(retention.UniqueKeys) > 0 {
		uniqueKey = sortedWindowKey(retention, entry.event, now, nil)
		if previous, exists := keyed[uniqueKey]; exists {
			for index, candidate := range entries {
				if sameEvent(candidate.event, previous.event) {
					entries = append(entries[:index], entries[index+1:]...)
					delta.Old = append(delta.Old, previous.event)
					break
				}
			}
			delete(keyed, uniqueKey)
		}
	}
	entries = append(entries, entry)
	sort.SliceStable(entries, func(i, j int) bool {
		return compareStoredEvents(entries[i].event, entries[j].event, retention.Keys, now, nil) < 0
	})
	if !retention.Rank {
		reverseSortedEqualRuns(entries, retention.Keys, now, nil)
	}
	for len(entries) > retention.Size {
		removeIndex := len(entries) - 1
		if retention.Rank {
			removeIndex = rankEvictionIndex(entries, retention.Keys, now, nil)
		}
		removed := entries[removeIndex]
		delta.Old = append(delta.Old, removed.event)
		if retention.Rank && len(retention.UniqueKeys) > 0 {
			delete(keyed, sortedWindowKey(retention, removed.event, now, nil))
		}
		entries = append(entries[:removeIndex], entries[removeIndex+1:]...)
	}
	if retention.Rank && len(retention.UniqueKeys) > 0 {
		keyed = make(map[string]storedEvent, len(entries))
		for _, retained := range entries {
			keyed[sortedWindowKey(retention, retained.event, now, nil)] = retained
		}
	}
	state.entries = entries
	state.keyed = keyed
	state.keyOrder = nil
}

// rebuildUniqueStateLocked normalizes keyed and sorted state after a direct
// named-window delete or update. Unique retention keeps the first-seen key
// order, matching the ordinary Unique window's snapshot order while moving
// the current event value into the existing key slot.
func (w *NamedWindow) rebuildUniqueStateLocked() {
	if w == nil || w.state == nil {
		return
	}
	if retention, sorted := w.state.def.retention.(SortedWindowSpec); sorted {
		w.state.entries, w.state.keyed = normalizeSortedNamedWindowEntries(w.state.entries, retention, w.now())
		w.state.keyOrder = nil
		return
	}
	retention, unique := w.state.def.retention.(UniqueWindowSpec)
	if !unique {
		w.state.keyed = nil
		w.state.keyOrder = nil
		return
	}
	keyed := make(map[string]storedEvent, len(w.state.entries))
	keyOrder := make([]string, 0, len(w.state.entries))
	positions := make(map[string]int, len(w.state.entries))
	entries := make([]storedEvent, 0, len(w.state.entries))
	for _, entry := range w.state.entries {
		key := uniqueWindowKey(retention, entry.event, entry.receivedAt, nil)
		if position, exists := positions[key]; exists {
			entries[position] = entry
			keyed[key] = entry
			continue
		}
		positions[key] = len(entries)
		entries = append(entries, entry)
		keyOrder = append(keyOrder, key)
		keyed[key] = entry
	}
	w.state.entries = entries
	w.state.keyed = keyed
	w.state.keyOrder = keyOrder
}

func (w *NamedWindow) Definition() NamedWindowDefinition {
	if w == nil || w.state == nil {
		return NamedWindowDefinition{}
	}
	return w.state.def
}

func (w *NamedWindow) Subscribe(listener NamedWindowListener) (Subscription, error) {
	if w == nil || w.state == nil {
		return Subscription{}, NewError(ErrorState, "nil named window")
	}
	if listener == nil {
		return Subscription{}, NewError(ErrorState, "named-window listener is nil")
	}
	state := w.state
	state.mu.Lock()
	defer state.mu.Unlock()
	state.nextID++
	id := state.nextID
	state.listeners[id] = listener
	return Subscription{namedWindow: w, namedWindowID: id}, nil
}

func (w *NamedWindow) removeSubscription(id uint64) error {
	if w == nil || w.state == nil {
		return nil
	}
	w.state.mu.Lock()
	defer w.state.mu.Unlock()
	delete(w.state.listeners, id)
	return nil
}

func (w *NamedWindow) Snapshot(ctx context.Context) ([]Event, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if w == nil || w.state == nil {
		return nil, NewError(ErrorState, "nil named window")
	}
	w.state.mu.RLock()
	defer w.state.mu.RUnlock()
	result := make([]Event, 0, len(w.state.entries))
	for _, entry := range w.state.entries {
		result = append(result, entry.event)
	}
	return result, nil
}

func (w *NamedWindow) DeleteWhere(ctx context.Context, predicate func(Event) bool) (NamedWindowDelta, error) {
	delta, err := w.deleteWhere(ctx, predicate)
	if err != nil {
		return NamedWindowDelta{}, err
	}
	if err := w.dispatch(ctx, delta); err != nil {
		return NamedWindowDelta{}, err
	}
	return delta, nil
}

func (w *NamedWindow) deleteWhere(ctx context.Context, predicate func(Event) bool) (NamedWindowDelta, error) {
	if err := contextErr(ctx); err != nil {
		return NamedWindowDelta{}, err
	}
	if w == nil || w.state == nil {
		return NamedWindowDelta{}, NewError(ErrorState, "nil named window")
	}
	if predicate == nil {
		return NamedWindowDelta{}, NewError(ErrorInvalidRule, "named-window delete predicate is required")
	}
	now := w.now()
	state := w.state
	state.mu.Lock()
	kept := state.entries[:0]
	delta := NamedWindowDelta{Time: now}
	for _, entry := range state.entries {
		if predicate(entry.event) {
			delta.Old = append(delta.Old, entry.event)
		} else {
			kept = append(kept, entry)
		}
	}
	state.entries = kept
	w.rebuildUniqueStateLocked()
	state.mu.Unlock()
	return delta, nil
}

func (w *NamedWindow) UpdateWhere(ctx context.Context, predicate func(Event) bool, update func(Event) (any, error)) (NamedWindowDelta, error) {
	delta, err := w.updateWhere(ctx, predicate, update)
	if err != nil {
		return NamedWindowDelta{}, err
	}
	if err := w.dispatch(ctx, delta); err != nil {
		return NamedWindowDelta{}, err
	}
	return delta, nil
}

func (w *NamedWindow) updateWhere(ctx context.Context, predicate func(Event) bool, update func(Event) (any, error)) (NamedWindowDelta, error) {
	if err := contextErr(ctx); err != nil {
		return NamedWindowDelta{}, err
	}
	if w == nil || w.state == nil {
		return NamedWindowDelta{}, NewError(ErrorState, "nil named window")
	}
	if predicate == nil || update == nil {
		return NamedWindowDelta{}, NewError(ErrorInvalidRule, "named-window update requires predicate and updater")
	}
	now := w.now()
	state := w.state
	state.mu.Lock()
	delta := NamedWindowDelta{Time: now}
	for index, entry := range state.entries {
		if !predicate(entry.event) {
			continue
		}
		underlying, err := update(entry.event)
		if err != nil {
			state.mu.Unlock()
			return NamedWindowDelta{}, err
		}
		updated, err := newEvent(state.def.schema, underlying, now)
		if err != nil {
			state.mu.Unlock()
			return NamedWindowDelta{}, err
		}
		updated.typeName = state.def.name
		updated.streamType = state.def.name
		delta.Old = append(delta.Old, entry.event)
		delta.New = append(delta.New, updated)
		state.entries[index] = storedEvent{event: updated, receivedAt: entry.receivedAt, expiresAt: entry.expiresAt}
	}
	w.rebuildUniqueStateLocked()
	state.mu.Unlock()
	return delta, nil
}

type namedWindowMergeAction uint8

const (
	namedWindowMergeNoop namedWindowMergeAction = iota
	namedWindowMergeUpdate
	namedWindowMergeDelete
)

type namedWindowMergeDecision struct {
	matched    bool
	action     namedWindowMergeAction
	underlying any
}

// mergeWhere evaluates all match decisions against one named-window state
// snapshot and applies the resulting update/delete/insert operations as one
// delta. Callers dispatch the returned delta after releasing the engine lock.
// The decision callback must not mutate the window itself.
func (w *NamedWindow) mergeWhere(ctx context.Context, decide func(Event) (namedWindowMergeDecision, error), insert func() (any, bool, error)) (NamedWindowDelta, error) {
	if err := contextErr(ctx); err != nil {
		return NamedWindowDelta{}, err
	}
	if w == nil || w.state == nil {
		return NamedWindowDelta{}, NewError(ErrorState, "nil named window")
	}
	if decide == nil || insert == nil {
		return NamedWindowDelta{}, NewError(ErrorInvalidRule, "named-window merge requires decision and insert callbacks")
	}
	now := w.now()
	state := w.state
	state.mu.Lock()
	defer state.mu.Unlock()

	decisions := make([]namedWindowMergeDecision, len(state.entries))
	matched := false
	for index, entry := range state.entries {
		if err := contextErr(ctx); err != nil {
			return NamedWindowDelta{}, err
		}
		decision, err := decide(entry.event)
		if err != nil {
			return NamedWindowDelta{}, err
		}
		if decision.action != namedWindowMergeNoop && decision.action != namedWindowMergeUpdate && decision.action != namedWindowMergeDelete {
			return NamedWindowDelta{}, NewError(ErrorInvalidRule, "named-window merge returned an unknown action")
		}
		if decision.matched {
			matched = true
		}
		decisions[index] = decision
	}

	var insertUnderlying any
	insertEvent := false
	if !matched {
		underlying, shouldInsert, err := insert()
		if err != nil {
			return NamedWindowDelta{}, err
		}
		insertUnderlying = underlying
		insertEvent = shouldInsert
	}
	if insertEvent {
		switch state.def.retention.(type) {
		case KeepAllWindowSpec, LengthWindowSpec, TimeWindowSpec, TimeToLiveWindowSpec, TimeToLiveAtWindowSpec, UniqueWindowSpec, SortedWindowSpec:
		default:
			return NamedWindowDelta{}, NewError(ErrorInvalidRule, fmt.Sprintf("unsupported named-window retention %T", state.def.retention))
		}
	}

	preparedUpdates := make([]Event, len(decisions))
	for index, decision := range decisions {
		if decision.action != namedWindowMergeUpdate {
			continue
		}
		updated, err := newEvent(state.def.schema, decision.underlying, now)
		if err != nil {
			return NamedWindowDelta{}, err
		}
		updated.typeName = state.def.name
		updated.streamType = state.def.name
		preparedUpdates[index] = updated
	}
	var preparedInsert Event
	if insertEvent {
		inserted, err := newEvent(state.def.schema, insertUnderlying, now)
		if err != nil {
			return NamedWindowDelta{}, err
		}
		inserted.typeName = state.def.name
		inserted.streamType = state.def.name
		preparedInsert = inserted
	}

	delta := NamedWindowDelta{Time: now}
	entries := make([]storedEvent, 0, len(state.entries)+1)
	for index, entry := range state.entries {
		decision := decisions[index]
		switch decision.action {
		case namedWindowMergeNoop:
			entries = append(entries, entry)
		case namedWindowMergeDelete:
			delta.Old = append(delta.Old, entry.event)
		case namedWindowMergeUpdate:
			updated := preparedUpdates[index]
			delta.Old = append(delta.Old, entry.event)
			delta.New = append(delta.New, updated)
			entries = append(entries, storedEvent{event: updated, receivedAt: entry.receivedAt, expiresAt: entry.expiresAt})
		}
	}
	if insertEvent {
		switch retention := state.def.retention.(type) {
		case UniqueWindowSpec:
			entry := storedEvent{event: preparedInsert, receivedAt: now}
			duplicate := -1
			for index, candidate := range entries {
				if uniqueWindowKey(retention, candidate.event, now, nil) == uniqueWindowKey(retention, preparedInsert, now, nil) {
					duplicate = index
					break
				}
			}
			if duplicate < 0 {
				entries = append(entries, entry)
				delta.New = append(delta.New, preparedInsert)
			} else if !retention.First {
				delta.Old = append(delta.Old, entries[duplicate].event)
				entries[duplicate] = entry
				delta.New = append(delta.New, preparedInsert)
			}
		default:
			entry := storedEvent{event: preparedInsert, receivedAt: now}
			if retention, ok := state.def.retention.(TimeToLiveAtWindowSpec); ok {
				expiresAt, err := eventTimestamp(retention.Timestamp, preparedInsert, now, nil)
				if err != nil {
					return NamedWindowDelta{}, err
				}
				entry.expiresAt = expiresAt
			}
			entries = append(entries, entry)
			delta.New = append(delta.New, preparedInsert)
			if !entry.expiresAt.IsZero() && !entry.expiresAt.After(now) {
				delta.Old = append(delta.Old, preparedInsert)
				entries = entries[:len(entries)-1]
			}
			if retention, ok := state.def.retention.(LengthWindowSpec); ok {
				for len(entries) > retention.Size {
					delta.Old = append(delta.Old, entries[0].event)
					entries = entries[1:]
				}
			}
		case SortedWindowSpec:
			state.entries = entries
			w.rebuildUniqueStateLocked()
			delta.New = append(delta.New, preparedInsert)
			applySortedNamedWindowInsertLocked(state, retention, storedEvent{event: preparedInsert, receivedAt: now}, now, &delta)
			entries = state.entries
		}
	}
	state.entries = entries
	w.rebuildUniqueStateLocked()
	return delta, nil
}

func (w *NamedWindow) now() time.Time {
	if w != nil && w.engine != nil {
		return w.engine.Now()
	}
	return time.Unix(0, 0).UTC()
}

func (w *NamedWindow) insert(now time.Time, underlying any) (NamedWindowDelta, error) {
	if w == nil || w.state == nil {
		return NamedWindowDelta{}, NewError(ErrorState, "nil named window")
	}
	event, err := newEvent(w.state.def.schema, underlying, now)
	if err != nil {
		return NamedWindowDelta{}, err
	}
	event.typeName = w.state.def.name
	event.streamType = w.state.def.name
	state := w.state
	state.mu.Lock()
	defer state.mu.Unlock()
	entry := storedEvent{event: event, receivedAt: now}
	delta := NamedWindowDelta{New: []Event{event}, Time: now}
	switch retention := state.def.retention.(type) {
	case KeepAllWindowSpec:
		state.entries = append(state.entries, entry)
	case LengthWindowSpec:
		state.entries = append(state.entries, entry)
		for len(state.entries) > retention.Size {
			delta.Old = append(delta.Old, state.entries[0].event)
			state.entries = state.entries[1:]
		}
	case TimeWindowSpec, TimeToLiveWindowSpec:
		state.entries = append(state.entries, entry)
	case TimeToLiveAtWindowSpec:
		expiresAt, err := eventTimestamp(retention.Timestamp, event, now, nil)
		if err != nil {
			return NamedWindowDelta{}, err
		}
		entry.expiresAt = expiresAt
		if !expiresAt.After(now) {
			delta.Old = append(delta.Old, event)
			return delta, nil
		}
		state.entries = append(state.entries, entry)
	case UniqueWindowSpec:
		if state.keyed == nil {
			state.keyed = make(map[string]storedEvent)
		}
		key := uniqueWindowKey(retention, event, now, nil)
		if previous, exists := state.keyed[key]; exists {
			if retention.First {
				return NamedWindowDelta{Time: now}, nil
			}
			for index, candidate := range state.entries {
				if sameEvent(candidate.event, previous.event) {
					state.entries[index] = entry
					state.keyed[key] = entry
					delta.Old = append(delta.Old, previous.event)
					return delta, nil
				}
			}
			state.keyed[key] = entry
			state.entries = append(state.entries, entry)
			return delta, nil
		}
		state.keyed[key] = entry
		state.keyOrder = append(state.keyOrder, key)
		state.entries = append(state.entries, entry)
	case SortedWindowSpec:
		applySortedNamedWindowInsertLocked(state, retention, entry, now, &delta)
	default:
		return NamedWindowDelta{}, NewError(ErrorInvalidRule, fmt.Sprintf("unsupported named-window retention %T", retention))
	}
	return delta, nil
}

func (w *NamedWindow) expire(at time.Time) NamedWindowDelta {
	if w == nil || w.state == nil {
		return NamedWindowDelta{}
	}
	state := w.state
	state.mu.Lock()
	defer state.mu.Unlock()
	var duration time.Duration
	switch retention := state.def.retention.(type) {
	case TimeWindowSpec:
		duration = retention.Duration
	case TimeToLiveWindowSpec:
		duration = retention.Duration
	case TimeToLiveAtWindowSpec:
		kept := state.entries[:0]
		delta := NamedWindowDelta{Time: at}
		for _, entry := range state.entries {
			if !entry.expiresAt.After(at) {
				delta.Old = append(delta.Old, entry.event)
			} else {
				kept = append(kept, entry)
			}
		}
		state.entries = kept
		return delta
	default:
		return NamedWindowDelta{}
	}
	kept := state.entries[:0]
	delta := NamedWindowDelta{Time: at}
	for _, entry := range state.entries {
		if !entry.receivedAt.Add(duration).After(at) {
			delta.Old = append(delta.Old, entry.event)
		} else {
			kept = append(kept, entry)
		}
	}
	state.entries = kept
	return delta
}

func (w *NamedWindow) dispatch(ctx context.Context, delta NamedWindowDelta) error {
	if delta.empty() || w == nil || w.state == nil {
		return nil
	}
	w.state.mu.RLock()
	listenerIDs := make([]uint64, 0, len(w.state.listeners))
	for id := range w.state.listeners {
		listenerIDs = append(listenerIDs, id)
	}
	sort.Slice(listenerIDs, func(i, j int) bool { return listenerIDs[i] < listenerIDs[j] })
	listeners := make([]NamedWindowListener, 0, len(listenerIDs))
	for _, id := range listenerIDs {
		listeners = append(listeners, w.state.listeners[id])
	}
	w.state.mu.RUnlock()
	for _, listener := range listeners {
		if err := listener(ctx, delta); err != nil {
			return fmt.Errorf("esper: named-window listener for %q: %w", w.state.def.name, err)
		}
	}
	return nil
}

func sortedNamedWindowNames(windows map[string]*NamedWindow) []string {
	names := make([]string, 0, len(windows))
	for name := range windows {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
