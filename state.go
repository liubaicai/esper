package esper

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// TableColumn describes a table column. Primary-key columns are populated
// from the row values and participate in Get/Delete/Upsert identity.
type TableColumn struct {
	Name       string
	Type       reflect.Type
	Optional   bool
	PrimaryKey bool
	// Nested describes the schema of a composite column, or of each element
	// when Type is an array/slice. It preserves nested property metadata when
	// a table row is materialized as an Event for fluent expressions.
	Nested Schema
}

// TableColumnOption changes the metadata of a table column declaration.
type TableColumnOption func(*TableColumn)

// WithTableColumnNestedSchema associates a composite column with its nested
// event schema. The same schema is used for a scalar composite value and for
// every element of an array/slice composite value.
func WithTableColumnNestedSchema(nested Schema) TableColumnOption {
	return func(column *TableColumn) { column.Nested = nested }
}

func TableColumnOf[T any](name string, options ...TableColumnOption) TableColumn {
	column := TableColumn{Name: name, Type: typeOf[T]()}
	for _, option := range options {
		if option != nil {
			option(&column)
		}
	}
	return column
}

// OptionalTableColumnOf declares a nullable table column. It is useful for
// dimensional aggregate materializations where subtotal rows intentionally
// carry Null for dimensions that are not present in that grouping set.
func OptionalTableColumnOf[T any](name string, options ...TableColumnOption) TableColumn {
	column := TableColumn{Name: name, Type: typeOf[T](), Optional: true}
	for _, option := range options {
		if option != nil {
			option(&column)
		}
	}
	return column
}

func PrimaryKeyColumn[T any](name string, options ...TableColumnOption) TableColumn {
	column := TableColumn{Name: name, Type: typeOf[T](), PrimaryKey: true}
	for _, option := range options {
		if option != nil {
			option(&column)
		}
	}
	return column
}

type TableIndexDefinition struct {
	Name    string
	Columns []string
	Unique  bool
	Kind    IndexKind
}

type TableOption func(*tableConfig)

func SecondaryIndex(name string, columns ...string) TableOption {
	return func(config *tableConfig) {
		config.indexes = append(config.indexes, TableIndexDefinition{Name: name, Columns: append([]string(nil), columns...), Kind: IndexHash})
	}
}

func UniqueIndex(name string, columns ...string) TableOption {
	return func(config *tableConfig) {
		config.indexes = append(config.indexes, TableIndexDefinition{Name: name, Columns: append([]string(nil), columns...), Unique: true, Kind: IndexHash})
	}
}

// SecondaryBTreeIndex declares a non-unique ordered index. It is useful for
// FAF range predicates and for equality-plus-range composite access paths.
// The declaration is explicit so callers never need to encode an EPL index
// backing keyword in a rule string.
func SecondaryBTreeIndex(name string, columns ...string) TableOption {
	return func(config *tableConfig) {
		config.indexes = append(config.indexes, TableIndexDefinition{Name: name, Columns: append([]string(nil), columns...), Kind: IndexBTree})
	}
}

// UniqueBTreeIndex declares a unique ordered index.
func UniqueBTreeIndex(name string, columns ...string) TableOption {
	return func(config *tableConfig) {
		config.indexes = append(config.indexes, TableIndexDefinition{Name: name, Columns: append([]string(nil), columns...), Unique: true, Kind: IndexBTree})
	}
}

type tableConfig struct {
	indexes []TableIndexDefinition
}

// TableDefinition is an immutable compile-time table declaration.
type TableDefinition struct {
	name       string
	moduleName string
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
	nested := make([]SchemaOption, 0)
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
		if column.Nested.valid() {
			nested = append(nested, WithNestedPropertySchema(column.Name, column.Nested))
		}
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
	seenIndexes := make(map[string]struct{}, len(config.indexes))
	for indexPosition := range config.indexes {
		index := &config.indexes[indexPosition]
		index.Name = strings.TrimSpace(index.Name)
		if strings.TrimSpace(index.Name) == "" || len(index.Columns) == 0 {
			return TableDefinition{}, NewError(ErrorInvalidRule, "table index requires a name and columns")
		}
		if _, exists := seenIndexes[index.Name]; exists {
			return TableDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("table duplicates index %q", index.Name))
		}
		seenIndexes[index.Name] = struct{}{}
		if !index.Kind.valid() {
			return TableDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("table index %q has unknown kind %d", index.Name, index.Kind))
		}
		seenColumns := make(map[string]struct{}, len(index.Columns))
		for columnPosition, column := range index.Columns {
			column = strings.TrimSpace(column)
			if _, exists := seen[column]; !exists {
				return TableDefinition{}, NewError(ErrorUnknownName, fmt.Sprintf("table index %q references unknown column %q", index.Name, column))
			}
			if _, duplicate := seenColumns[column]; duplicate {
				return TableDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("table index %q duplicates column %q", index.Name, column))
			}
			seenColumns[column] = struct{}{}
			index.Columns[columnPosition] = column
		}
	}
	schema, err := NewMapSchema("table:"+name, fields, nested...)
	if err != nil {
		return TableDefinition{}, err
	}
	return TableDefinition{
		name:       name,
		columns:    copyColumns,
		schema:     schema,
		primaryKey: primaryKey,
		indexes:    cloneTableIndexDefinitions(config.indexes),
	}, nil
}

func (d TableDefinition) Name() string           { return d.name }
func (d TableDefinition) Module() string         { return d.moduleName }
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
	return e.RegisterTableInModule("", name, columns, options...)
}

// RegisterTableInModule registers a table under a module-local logical name.
// The default module is the original unqualified catalog.
func (e *Environment) RegisterTableInModule(moduleName, name string, columns []TableColumn, options ...TableOption) (TableDefinition, error) {
	definition, err := NewTableDefinition(name, columns, options...)
	if err != nil {
		return TableDefinition{}, err
	}
	if e == nil {
		return TableDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	moduleName = normalizeModuleName(moduleName)
	e.mu.Lock()
	defer e.mu.Unlock()
	if moduleName != "" {
		if _, exists := e.modules[moduleName]; !exists {
			return TableDefinition{}, NewError(ErrorUnknownName, fmt.Sprintf("module %q is not registered", moduleName))
		}
	}
	key := catalogKey(moduleName, name)
	if _, exists := e.tables[key]; exists {
		return TableDefinition{}, NewError(ErrorDependency, fmt.Sprintf("table %q is already registered in module %q", name, moduleName))
	}
	definition.moduleName = moduleName
	definition.schema.name = "table:" + key
	e.tables[key] = definition
	return definition, nil
}

func CreateTable(env *Environment, name string, columns []TableColumn, options ...TableOption) (TableDefinition, error) {
	if env == nil {
		return TableDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	return env.RegisterTable(name, columns, options...)
}

func CreateTableInModule(env *Environment, moduleName, name string, columns []TableColumn, options ...TableOption) (TableDefinition, error) {
	if env == nil {
		return TableDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	return env.RegisterTableInModule(moduleName, name, columns, options...)
}

func (e *Environment) Table(name string) (TableDefinition, bool) {
	return e.TableInModule("", name)
}

func (e *Environment) TableInModule(moduleName, name string) (TableDefinition, bool) {
	if e == nil {
		return TableDefinition{}, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	definition, ok := e.tables[catalogKey(moduleName, name)]
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
	mu           sync.RWMutex
	def          TableDefinition
	rows         map[string]TableRow
	order        []string
	indexes      map[string]map[string][]string
	indexEntries map[string][]tableIndexEntry
	version      uint64
	indexLookups atomic.Uint64
}

// tableIndexEntry is the ordered representation of one B-tree index member.
// The hash map above remains the fast complete-key path; entries retain the
// typed values so range probes do not need to decode encodeKey strings.
type tableIndexEntry struct {
	rowKey string
	values []Value
}

// Table is a concurrency-safe in-memory table owned by an Engine. Reads use
// a consistent snapshot and writes update all configured indexes atomically.
type Table struct {
	state *tableState
}

type tableMutationSnapshot struct {
	rows         map[string]TableRow
	order        []string
	indexes      map[string]map[string][]string
	indexEntries map[string][]tableIndexEntry
	version      uint64
}

func (t *Table) snapshotMutationState() tableMutationSnapshot {
	if t == nil || t.state == nil {
		return tableMutationSnapshot{}
	}
	state := t.state
	state.mu.RLock()
	defer state.mu.RUnlock()
	snapshot := tableMutationSnapshot{
		rows:         make(map[string]TableRow, len(state.rows)),
		order:        append([]string(nil), state.order...),
		indexes:      make(map[string]map[string][]string, len(state.indexes)),
		indexEntries: make(map[string][]tableIndexEntry, len(state.indexEntries)),
		version:      state.version,
	}
	for key, row := range state.rows {
		snapshot.rows[key] = cloneTableRow(row)
	}
	for name, index := range state.indexes {
		copied := make(map[string][]string, len(index))
		for key, rowKeys := range index {
			copied[key] = append([]string(nil), rowKeys...)
		}
		snapshot.indexes[name] = copied
	}
	for name, entries := range state.indexEntries {
		copied := make([]tableIndexEntry, len(entries))
		for index, entry := range entries {
			copied[index] = tableIndexEntry{rowKey: entry.rowKey, values: append([]Value(nil), entry.values...)}
		}
		snapshot.indexEntries[name] = copied
	}
	return snapshot
}

func (t *Table) restoreMutationState(snapshot tableMutationSnapshot) {
	if t == nil || t.state == nil {
		return
	}
	state := t.state
	state.mu.Lock()
	defer state.mu.Unlock()
	state.rows = make(map[string]TableRow, len(snapshot.rows))
	for key, row := range snapshot.rows {
		state.rows[key] = cloneTableRow(row)
	}
	state.order = append([]string(nil), snapshot.order...)
	state.indexes = make(map[string]map[string][]string, len(snapshot.indexes))
	for name, index := range snapshot.indexes {
		copied := make(map[string][]string, len(index))
		for key, rowKeys := range index {
			copied[key] = append([]string(nil), rowKeys...)
		}
		state.indexes[name] = copied
	}
	state.indexEntries = make(map[string][]tableIndexEntry, len(snapshot.indexEntries))
	for name, entries := range snapshot.indexEntries {
		copied := make([]tableIndexEntry, len(entries))
		for index, entry := range entries {
			copied[index] = tableIndexEntry{rowKey: entry.rowKey, values: append([]Value(nil), entry.values...)}
		}
		state.indexEntries[name] = copied
	}
	state.version = snapshot.version
}

func newTable(definition TableDefinition) *Table {
	indexes := make(map[string]map[string][]string, len(definition.indexes))
	indexEntries := make(map[string][]tableIndexEntry, len(definition.indexes))
	for _, index := range definition.indexes {
		indexes[index.Name] = make(map[string][]string)
		indexEntries[index.Name] = nil
	}
	return &Table{state: &tableState{def: definition, rows: make(map[string]TableRow), indexes: indexes, indexEntries: indexEntries}}
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
		def:          state.def,
		rows:         make(map[string]TableRow, len(rows)),
		indexes:      make(map[string]map[string][]string, len(state.def.indexes)),
		indexEntries: make(map[string][]tableIndexEntry, len(state.def.indexes)),
		version:      state.version,
	}
	for _, definition := range state.def.indexes {
		replacement.indexes[definition.Name] = make(map[string][]string)
		replacement.indexEntries[definition.Name] = nil
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
	state.indexEntries = replacement.indexEntries
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
	merged := make(map[string]any, len(existing.values))
	for name, value := range existing.values {
		merged[name] = value.Any()
	}
	for name, value := range values {
		merged[name] = value
	}
	converted, newRowKey, err := state.convertValues(merged)
	if err != nil {
		return TableRow{}, err
	}
	if newRowKey != rowKey {
		if _, exists := state.rows[newRowKey]; exists {
			return TableRow{}, NewError(ErrorState, "table row already exists")
		}
	}
	updated := TableRow{values: converted}
	if err := state.validateIndexesLockedIgnoring(newRowKey, updated, rowKey); err != nil {
		return TableRow{}, err
	}

	// A table primary key is the row identity, but Esper permits an update to
	// change that identity. Re-key in place so insertion order remains stable;
	// removing and appending would make FAF/update iteration order observable.
	state.removeIndexesLocked(rowKey, existing)
	if newRowKey != rowKey {
		delete(state.rows, rowKey)
		for index, key := range state.order {
			if key == rowKey {
				state.order[index] = newRowKey
				break
			}
		}
	}
	state.version++
	updated.version = state.version
	state.rows[newRowKey] = updated
	state.addIndexesLocked(newRowKey, updated)
	return cloneTableRow(updated), nil
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
		state.indexEntries[name] = nil
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
	state.indexLookups.Add(1)
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

// lookupMany returns indexed rows in table insertion order, independent of
// the order in which IN probe keys were supplied. It is internal so the
// observable public Lookup API stays a single complete-key operation.
func (t *Table) lookupMany(ctx context.Context, indexName string, keys [][]any) ([]TableRow, error) {
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
	state.indexLookups.Add(1)
	wanted := make(map[string]struct{})
	for _, key := range keys {
		for _, rowKey := range index[encodeKey(key)] {
			wanted[rowKey] = struct{}{}
		}
	}
	rows := make([]TableRow, 0, len(wanted))
	for _, rowKey := range state.order {
		if _, exists := wanted[rowKey]; !exists {
			continue
		}
		if row, exists := state.rows[rowKey]; exists {
			rows = append(rows, cloneTableRow(row))
		}
	}
	return rows, nil
}

// lookupPrimaryMany is the complete-key counterpart for a table primary key.
// Primary keys are row identity rather than secondary index buckets, but FAF
// candidate execution still needs one consistent snapshot and insertion-order
// result assembly when probing several keys.
func (t *Table) lookupPrimaryMany(ctx context.Context, keys [][]any) ([]TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if t == nil || t.state == nil {
		return nil, NewError(ErrorState, "nil table")
	}
	state := t.state
	state.mu.RLock()
	defer state.mu.RUnlock()
	state.indexLookups.Add(1)
	wanted := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		rowKey, err := state.keyFromValues(key)
		if err != nil {
			return nil, err
		}
		wanted[rowKey] = struct{}{}
	}
	rows := make([]TableRow, 0, len(wanted))
	for _, rowKey := range state.order {
		if _, exists := wanted[rowKey]; !exists {
			continue
		}
		if row, exists := state.rows[rowKey]; exists {
			rows = append(rows, cloneTableRow(row))
		}
	}
	return rows, nil
}

// lookupRange walks the ordered members of a B-tree index and returns matched
// rows in table insertion order. The ordered members are kept separately from
// the hash buckets so the runtime never has to reverse-engineer encodeKey.
func (t *Table) lookupRange(ctx context.Context, indexName string, query indexRangeSpec) ([]TableRow, error) {
	return t.lookupRangeMany(ctx, indexName, []indexRangeSpec{query})
}

// lookupRangeMany unions several complete range probes in one locked
// insertion-order snapshot. FAF Join range candidates can be driven by more
// than one loaded-side tuple; probing them as a batch avoids duplicate rows
// and keeps result order independent of probe order.
func (t *Table) lookupRangeMany(ctx context.Context, indexName string, queries []indexRangeSpec) ([]TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if t == nil || t.state == nil {
		return nil, NewError(ErrorState, "nil table")
	}
	state := t.state
	state.mu.RLock()
	defer state.mu.RUnlock()
	if _, ok := state.indexes[indexName]; !ok {
		return nil, NewError(ErrorUnknownName, fmt.Sprintf("table index %q does not exist", indexName))
	}
	state.indexLookups.Add(1)
	entries := state.indexEntries[indexName]
	positions, cursorUsable, err := collectIndexRangeCursorPositions(ctx, len(entries), func(index int) []Value {
		return entries[index].values
	}, queries)
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]struct{})
	if cursorUsable {
		for position := range positions {
			if position >= 0 && position < len(entries) {
				wanted[entries[position].rowKey] = struct{}{}
			}
		}
	} else {
		for _, entry := range entries {
			if err := contextErr(ctx); err != nil {
				return nil, err
			}
			for _, query := range queries {
				if indexRangeEntryMatches(entry.values, query) {
					wanted[entry.rowKey] = struct{}{}
					break
				}
			}
		}
	}
	rows := make([]TableRow, 0, len(wanted))
	for _, rowKey := range state.order {
		if _, exists := wanted[rowKey]; !exists {
			continue
		}
		if row, exists := state.rows[rowKey]; exists {
			rows = append(rows, cloneTableRow(row))
		}
	}
	return rows, nil
}

func (s *tableState) upsert(values map[string]any, insertOnly bool) (TableRow, error) {
	converted, rowKey, err := s.convertValues(values)
	if err != nil {
		return TableRow{}, err
	}
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

func (s *tableState) convertValues(values map[string]any) (map[string]Value, string, error) {
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
				return nil, "", NewError(ErrorTypeMismatch, fmt.Sprintf("table column %q is required", column.Name))
			}
			converted[column.Name] = Null()
			continue
		}
		convertedValue, err := coerceTableValue(value, column.Type)
		if err != nil {
			return nil, "", WrapError(ErrorTypeMismatch, "table."+column.Name, err)
		}
		converted[column.Name] = convertedValue
	}
	for name := range values {
		if _, exists := known[name]; !exists {
			return nil, "", NewError(ErrorUnknownName, fmt.Sprintf("table column %q is not defined", name))
		}
	}
	keyValues := make([]any, 0, len(s.def.primaryKey))
	for _, name := range s.def.primaryKey {
		value := converted[name]
		if !value.IsPresent() {
			return nil, "", NewError(ErrorTypeMismatch, fmt.Sprintf("primary-key column %q cannot be null", name))
		}
		keyValues = append(keyValues, value.Any())
	}
	return converted, encodeKey(keyValues), nil
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
	return s.validateIndexesLockedIgnoring(rowKey, row, "")
}

func (s *tableState) validateIndexesLockedIgnoring(rowKey string, row TableRow, ignoredRowKey string) error {
	for _, definition := range s.def.indexes {
		if !definition.Unique {
			continue
		}
		key := s.indexKey(row, definition.Columns)
		for _, existing := range s.indexes[definition.Name][key] {
			if existing != rowKey && existing != ignoredRowKey {
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
		entries := s.indexEntries[definition.Name]
		entries = append(entries, tableIndexEntry{rowKey: rowKey, values: tableIndexValues(row, definition.Columns)})
		sort.SliceStable(entries, func(left, right int) bool {
			comparison, comparable := compareIndexValueSlices(entries[left].values, entries[right].values)
			if !comparable || comparison == 0 {
				return false
			}
			return comparison < 0
		})
		s.indexEntries[definition.Name] = entries
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
		entries := s.indexEntries[definition.Name]
		for position, entry := range entries {
			if entry.rowKey != rowKey {
				continue
			}
			entries = append(entries[:position], entries[position+1:]...)
			break
		}
		s.indexEntries[definition.Name] = entries
	}
}

func (s *tableState) indexKey(row TableRow, columns []string) string {
	values := make([]any, 0, len(columns))
	for _, column := range columns {
		values = append(values, row.Get(column).Any())
	}
	return encodeKey(values)
}

func tableIndexValues(row TableRow, columns []string) []Value {
	values := make([]Value, 0, len(columns))
	for _, column := range columns {
		values = append(values, row.Get(column))
	}
	return values
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
	name          string
	moduleName    string
	schema        Schema
	retention     WindowSpec
	contextName   string
	indexes       []NamedWindowIndexDefinition
	uniqueIndexes []NamedWindowIndexDefinition
}

// NamedWindowIndexDefinition describes an index declared directly on a
// Named Window. A non-unique index is a reusable lookup/access path, while a
// strict unique index also rejects duplicate keys. It remains distinct from
// a Unique retention view, whose duplicate-key behavior is replacement rather
// than a constraint violation.
type NamedWindowIndexDefinition struct {
	Name    string
	Columns []string
	Unique  bool
	Kind    IndexKind
}

type NamedWindowOption func(*namedWindowConfig)

func NamedWindowRetention(window WindowSpec) NamedWindowOption {
	return func(config *namedWindowConfig) { config.retention = window }
}

// NamedWindowContext binds storage to one partition of a registered key or
// category context. The root NamedWindow still exposes an all-partition
// snapshot for fire-and-forget reads; statements running inside the same
// context use their partition-local snapshot.
func NamedWindowContext(contextName string) NamedWindowOption {
	return func(config *namedWindowConfig) { config.contextName = strings.TrimSpace(contextName) }
}

// NamedWindowIndex adds a non-unique hash index to a named window.
func NamedWindowIndex(name string, columns ...string) NamedWindowOption {
	return func(config *namedWindowConfig) {
		config.indexes = append(config.indexes, NamedWindowIndexDefinition{
			Name: strings.TrimSpace(name), Columns: append([]string(nil), columns...), Kind: IndexHash,
		})
	}
}

// NamedWindowBTreeIndex adds a non-unique ordered index to a named window.
func NamedWindowBTreeIndex(name string, columns ...string) NamedWindowOption {
	return func(config *namedWindowConfig) {
		config.indexes = append(config.indexes, NamedWindowIndexDefinition{
			Name: strings.TrimSpace(name), Columns: append([]string(nil), columns...), Kind: IndexBTree,
		})
	}
}

// NamedWindowUniqueIndex adds a strict unique index constraint to a Named
// Window. Unlike NamedWindowRetention(Unique(...)), a duplicate key is an
// error and does not replace the retained event.
func NamedWindowUniqueIndex(name string, columns ...string) NamedWindowOption {
	return func(config *namedWindowConfig) {
		config.indexes = append(config.indexes, NamedWindowIndexDefinition{
			Name: strings.TrimSpace(name), Columns: append([]string(nil), columns...), Unique: true, Kind: IndexHash,
		})
	}
}

// NamedWindowUniqueBTreeIndex adds a strict unique ordered index.
func NamedWindowUniqueBTreeIndex(name string, columns ...string) NamedWindowOption {
	return func(config *namedWindowConfig) {
		config.indexes = append(config.indexes, NamedWindowIndexDefinition{
			Name: strings.TrimSpace(name), Columns: append([]string(nil), columns...), Unique: true, Kind: IndexBTree,
		})
	}
}

type namedWindowConfig struct {
	retention     WindowSpec
	contextName   string
	indexes       []NamedWindowIndexDefinition
	uniqueIndexes []NamedWindowIndexDefinition
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
	seenIndexes := make(map[string]struct{}, len(config.indexes))
	indexes := make([]NamedWindowIndexDefinition, 0, len(config.indexes))
	uniqueIndexes := make([]NamedWindowIndexDefinition, 0, len(config.indexes))
	for index, definition := range config.indexes {
		definition.Name = strings.TrimSpace(definition.Name)
		if definition.Name == "" || len(definition.Columns) == 0 {
			return NamedWindowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("named-window index %d requires a name and columns", index+1))
		}
		if _, exists := seenIndexes[definition.Name]; exists {
			return NamedWindowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("named-window duplicates index %q", definition.Name))
		}
		seenIndexes[definition.Name] = struct{}{}
		if !definition.Kind.valid() {
			return NamedWindowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("named-window index %q has unknown kind %d", definition.Name, definition.Kind))
		}
		seenColumns := make(map[string]struct{}, len(definition.Columns))
		for columnIndex, column := range definition.Columns {
			column = strings.TrimSpace(column)
			if column == "" {
				return NamedWindowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("named-window index %q column %d is blank", definition.Name, columnIndex+1))
			}
			if _, duplicate := seenColumns[column]; duplicate {
				return NamedWindowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("named-window index %q duplicates column %q", definition.Name, column))
			}
			if _, exists := schema.Field(column); !exists {
				return NamedWindowDefinition{}, NewError(ErrorUnknownName, fmt.Sprintf("named-window index %q references unknown column %q", definition.Name, column))
			}
			seenColumns[column] = struct{}{}
			definition.Columns[columnIndex] = column
		}
		indexes = append(indexes, definition)
		if definition.Unique {
			uniqueIndexes = append(uniqueIndexes, definition)
		}
	}
	return NamedWindowDefinition{name: name, schema: schema, retention: config.retention, contextName: config.contextName, indexes: indexes, uniqueIndexes: uniqueIndexes}, nil
}

func (d NamedWindowDefinition) Name() string          { return d.name }
func (d NamedWindowDefinition) Module() string        { return d.moduleName }
func (d NamedWindowDefinition) Schema() Schema        { return d.schema }
func (d NamedWindowDefinition) Retention() WindowSpec { return d.retention }
func (d NamedWindowDefinition) Context() string       { return d.contextName }
func (d NamedWindowDefinition) Indexes() []NamedWindowIndexDefinition {
	return cloneNamedWindowIndexDefinitions(d.indexes)
}

// UniqueIndexes returns only strict unique constraints. Indexes returns both
// unique and non-unique declarations.
func (d NamedWindowDefinition) UniqueIndexes() []NamedWindowIndexDefinition {
	return cloneNamedWindowIndexDefinitions(d.uniqueIndexes)
}

func (e *Environment) RegisterNamedWindow(name string, schema Schema, options ...NamedWindowOption) (NamedWindowDefinition, error) {
	return e.RegisterNamedWindowInModule("", name, schema, options...)
}

// RegisterNamedWindowInModule registers a named window under a module-local
// logical name. A module-local object keeps its Java-style protected/public
// resolution boundary without exposing an EPL module declaration.
func (e *Environment) RegisterNamedWindowInModule(moduleName, name string, schema Schema, options ...NamedWindowOption) (NamedWindowDefinition, error) {
	definition, err := NewNamedWindowDefinition(name, schema, options...)
	if err != nil {
		return NamedWindowDefinition{}, err
	}
	if e == nil {
		return NamedWindowDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	if definition.contextName != "" {
		if _, ok := e.Context(definition.contextName); !ok {
			return NamedWindowDefinition{}, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", definition.contextName))
		}
	}
	moduleName = normalizeModuleName(moduleName)
	e.mu.Lock()
	defer e.mu.Unlock()
	if moduleName != "" {
		if _, exists := e.modules[moduleName]; !exists {
			return NamedWindowDefinition{}, NewError(ErrorUnknownName, fmt.Sprintf("module %q is not registered", moduleName))
		}
	}
	key := catalogKey(moduleName, name)
	if _, exists := e.namedWindows[key]; exists {
		return NamedWindowDefinition{}, NewError(ErrorDependency, fmt.Sprintf("named window %q is already registered in module %q", name, moduleName))
	}
	definition.moduleName = moduleName
	e.namedWindows[key] = definition
	return definition, nil
}

func CreateNamedWindow(env *Environment, name string, schema Schema, options ...NamedWindowOption) (NamedWindowDefinition, error) {
	if env == nil {
		return NamedWindowDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	return env.RegisterNamedWindow(name, schema, options...)
}

func CreateNamedWindowInModule(env *Environment, moduleName, name string, schema Schema, options ...NamedWindowOption) (NamedWindowDefinition, error) {
	if env == nil {
		return NamedWindowDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	return env.RegisterNamedWindowInModule(moduleName, name, schema, options...)
}

func (e *Environment) NamedWindow(name string) (NamedWindowDefinition, bool) {
	return e.NamedWindowInModule("", name)
}

func (e *Environment) NamedWindowInModule(moduleName, name string) (NamedWindowDefinition, bool) {
	if e == nil {
		return NamedWindowDefinition{}, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	definition, ok := e.namedWindows[catalogKey(moduleName, name)]
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
	mu                sync.RWMutex
	def               NamedWindowDefinition
	contextKey        string
	contextProperties map[string]Value
	partitions        map[string]*namedWindowRuntime
	entries           []storedEvent
	indexes           map[string]map[string][]int
	indexEntries      map[string][]namedWindowIndexEntry
	keyed             map[string]storedEvent
	keyOrder          []string
	listeners         map[uint64]NamedWindowListener
	nextID            uint64
	indexLookups      atomic.Uint64
}

// namedWindowIndexEntry is the ordered representation of one B-tree index
// member. Positions are rebuilt together with the Named Window index after
// any mutation that can shift entry positions.
type namedWindowIndexEntry struct {
	position int
	values   []Value
}

type NamedWindow struct {
	state  *namedWindowRuntime
	engine *Engine
}

func newNamedWindow(definition NamedWindowDefinition, engine *Engine) *NamedWindow {
	state := newNamedWindowRuntime(definition, "")
	return &NamedWindow{state: state, engine: engine}
}

func newNamedWindowRuntime(definition NamedWindowDefinition, contextKey string) *namedWindowRuntime {
	indexes := make(map[string]map[string][]int, len(definition.indexes))
	indexEntries := make(map[string][]namedWindowIndexEntry, len(definition.indexes))
	for _, index := range definition.indexes {
		indexes[index.Name] = make(map[string][]int)
		indexEntries[index.Name] = nil
	}
	state := &namedWindowRuntime{def: definition, contextKey: contextKey, indexes: indexes, indexEntries: indexEntries, listeners: make(map[uint64]NamedWindowListener)}
	if definition.contextName != "" && contextKey == "" {
		state.partitions = make(map[string]*namedWindowRuntime)
	}
	if _, unique := definition.retention.(UniqueWindowSpec); unique {
		state.keyed = make(map[string]storedEvent)
	}
	if retention, sorted := definition.retention.(SortedWindowSpec); sorted && retention.Rank && len(retention.UniqueKeys) > 0 {
		state.keyed = make(map[string]storedEvent)
	}
	return state
}

func namedWindowIndexKey(event Event, columns []string) string {
	values := make([]any, 0, len(columns))
	for _, column := range columns {
		values = append(values, event.Get(column).Any())
	}
	return encodeKey(values)
}

func namedWindowIndexValues(event Event, columns []string) []Value {
	values := make([]Value, 0, len(columns))
	for _, column := range columns {
		values = append(values, event.Get(column))
	}
	return values
}

func namedWindowIndexKeyDisplay(event Event, columns []string) any {
	if len(columns) == 1 {
		return event.Get(columns[0]).Any()
	}
	values := make([]any, 0, len(columns))
	for _, column := range columns {
		values = append(values, event.Get(column).Any())
	}
	return values
}

func namedWindowIndexDefinition(definition NamedWindowDefinition, name string) (NamedWindowIndexDefinition, bool) {
	for _, index := range definition.indexes {
		if index.Name == name {
			return index, true
		}
	}
	return NamedWindowIndexDefinition{}, false
}

func rebuildNamedWindowIndexesLocked(state *namedWindowRuntime) {
	if state == nil {
		return
	}
	indexes := make(map[string]map[string][]int, len(state.def.indexes))
	indexEntries := make(map[string][]namedWindowIndexEntry, len(state.def.indexes))
	for _, definition := range state.def.indexes {
		index := make(map[string][]int)
		entries := make([]namedWindowIndexEntry, 0, len(state.entries))
		for position, entry := range state.entries {
			key := namedWindowIndexKey(entry.event, definition.Columns)
			if definition.Unique && len(index[key]) > 0 {
				continue
			}
			index[key] = append(index[key], position)
			entries = append(entries, namedWindowIndexEntry{position: position, values: namedWindowIndexValues(entry.event, definition.Columns)})
		}
		sort.SliceStable(entries, func(left, right int) bool {
			comparison, comparable := compareIndexValueSlices(entries[left].values, entries[right].values)
			if !comparable || comparison == 0 {
				return false
			}
			return comparison < 0
		})
		indexes[definition.Name] = index
		indexEntries[definition.Name] = entries
	}
	state.indexes = indexes
	state.indexEntries = indexEntries
}

func validateNamedWindowUniqueIndexesLocked(state *namedWindowRuntime, event Event) error {
	if state == nil || len(state.def.uniqueIndexes) == 0 {
		return nil
	}
	for _, definition := range state.def.uniqueIndexes {
		key := namedWindowIndexKey(event, definition.Columns)
		for _, existing := range state.entries {
			if namedWindowIndexKey(existing.event, definition.Columns) != key {
				continue
			}
			return NewError(ErrorState, fmt.Sprintf("Unique index violation, index '%s' is a unique index and key '%v' already exists", definition.Name, namedWindowIndexKeyDisplay(event, definition.Columns)))
		}
	}
	return nil
}

func (state *namedWindowRuntime) contextPropertiesSnapshot() map[string]Value {
	if state == nil {
		return nil
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	return cloneValues(state.contextProperties)
}

func (state *namedWindowRuntime) rememberContextProperties(properties map[string]Value) {
	if state == nil || len(properties) == 0 || state.def.contextName == "" {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.contextProperties) == 0 {
		state.contextProperties = cloneValues(properties)
	}
}

func contextPropertiesFromVariables(variables map[string]Value) map[string]Value {
	if len(variables) == 0 {
		return nil
	}
	properties := make(map[string]Value)
	for name, value := range variables {
		if strings.HasPrefix(name, contextVariablePrefix) {
			properties[strings.TrimPrefix(name, contextVariablePrefix)] = value
		}
	}
	return properties
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
	if w == nil {
		return
	}
	w.rebuildUniqueStateForLocked(w.state)
}

func (w *NamedWindow) rebuildUniqueStateForLocked(state *namedWindowRuntime) {
	if w == nil || state == nil {
		return
	}
	if retention, sorted := state.def.retention.(SortedWindowSpec); sorted {
		state.entries, state.keyed = normalizeSortedNamedWindowEntries(state.entries, retention, w.now())
		state.keyOrder = nil
		rebuildNamedWindowIndexesLocked(state)
		return
	}
	retention, unique := state.def.retention.(UniqueWindowSpec)
	if !unique {
		state.keyed = nil
		state.keyOrder = nil
		rebuildNamedWindowIndexesLocked(state)
		return
	}
	keyed := make(map[string]storedEvent, len(state.entries))
	keyOrder := make([]string, 0, len(state.entries))
	positions := make(map[string]int, len(state.entries))
	entries := make([]storedEvent, 0, len(state.entries))
	for _, entry := range state.entries {
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
	state.entries = entries
	state.keyed = keyed
	state.keyOrder = keyOrder
	rebuildNamedWindowIndexesLocked(state)
}

func (w *NamedWindow) Definition() NamedWindowDefinition {
	if w == nil || w.state == nil {
		return NamedWindowDefinition{}
	}
	return w.state.def
}

func (w *NamedWindow) contextPartitionKey(event Event, now time.Time, variables map[string]Value) (string, bool, error) {
	if w == nil || w.state == nil || w.state.def.contextName == "" || w.state.contextKey != "" {
		return "", true, nil
	}
	if w.engine == nil || w.engine.env == nil {
		return "", false, NewError(ErrorDependency, "context-bound named window has no environment")
	}
	definition, ok := w.engine.env.Context(w.state.def.contextName)
	if !ok {
		return "", false, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", w.state.def.contextName))
	}
	// Callers route events while holding the engine mutex, so the variables are
	// passed as an already captured snapshot instead of calling Engine.Variables
	// and attempting to reacquire that mutex.
	key, active, err := definition.partition(event, now, variables)
	if err != nil || !active {
		return key, active, err
	}
	return key, true, nil
}

func (w *NamedWindow) partitionState(key string, create bool) (*namedWindowRuntime, error) {
	if w == nil || w.state == nil {
		return nil, NewError(ErrorState, "nil named window")
	}
	if w.state.def.contextName == "" || w.state.contextKey != "" {
		return w.state, nil
	}
	if key == "" {
		return nil, NewError(ErrorInvalidRule, "context-bound named window requires an active partition key")
	}
	w.state.mu.Lock()
	defer w.state.mu.Unlock()
	partition := w.state.partitions[key]
	if partition == nil && create {
		partition = newNamedWindowRuntime(w.state.def, key)
		w.state.partitions[key] = partition
	}
	if partition == nil {
		return nil, NewError(ErrorUnknownName, fmt.Sprintf("named window context partition %q is not active", key))
	}
	return partition, nil
}

// releaseContextPartition drops the storage owned by one lifecycle-managed
// context partition. Key/hash/category contexts intentionally keep their
// partition state while the Environment remains alive; temporal and
// initiated contexts call this only after their last statement reference is
// released.
func (w *NamedWindow) releaseContextPartition(contextName, partitionKey string) {
	if w == nil || w.state == nil || partitionKey == "" || w.state.contextKey != "" || w.state.def.contextName != contextName {
		return
	}
	if w.engine == nil || w.engine.env == nil {
		return
	}
	definition, ok := w.engine.env.Context(contextName)
	if !ok || (!definition.isTemporal() && definition.kind != ContextInitiatedTerminated) {
		return
	}
	w.state.mu.Lock()
	delete(w.state.partitions, partitionKey)
	w.state.mu.Unlock()
}

// scopedForVariables returns the context partition represented by the
// reserved context variables attached to a statement evaluation.  A
// context-bound named window is still exposed as a root object for public
// snapshots and fire-and-forget queries, but on-trigger operations running
// inside a context must address only the current partition.
//
// When create is false, an inactive/empty partition is reported through the
// boolean result instead of creating state or treating the whole context
// window as the target.  This makes context-local select/update/delete
// operations no-ops until that partition has received an insert.
func (w *NamedWindow) scopedForVariables(variables map[string]Value, create bool) (*NamedWindow, bool, error) {
	if w == nil || w.state == nil {
		return nil, false, NewError(ErrorState, "nil named window")
	}
	if w.state.def.contextName == "" || w.state.contextKey != "" {
		return w, true, nil
	}
	partitionKey, ok := w.contextPartitionFromVariables(variables)
	if !ok {
		return w, true, nil
	}
	if !create {
		w.state.mu.RLock()
		partition := w.state.partitions[partitionKey]
		w.state.mu.RUnlock()
		if partition == nil {
			return nil, false, nil
		}
		return &NamedWindow{state: partition, engine: w.engine}, true, nil
	}
	partition, err := w.partitionState(partitionKey, true)
	if err != nil {
		return nil, false, err
	}
	return &NamedWindow{state: partition, engine: w.engine}, true, nil
}

func (w *NamedWindow) contextPartitionFromVariables(variables map[string]Value) (string, bool) {
	if w == nil || w.state == nil || w.state.def.contextName == "" || w.state.contextKey != "" || variables == nil {
		return "", false
	}
	nameValue, nameOK := variables[subqueryContextNameVariable]
	keyValue, keyOK := variables[subqueryContextPartitionVariable]
	if !nameOK || !keyOK || !nameValue.IsPresent() || !keyValue.IsPresent() {
		return "", false
	}
	contextName, nameTypeOK := nameValue.Any().(string)
	partitionKey, keyTypeOK := keyValue.Any().(string)
	if !nameTypeOK || !keyTypeOK || contextName != w.state.def.contextName || partitionKey == "" {
		return "", false
	}
	return partitionKey, true
}

func (w *NamedWindow) contextPartitionStates() []*namedWindowRuntime {
	if w == nil || w.state == nil {
		return nil
	}
	if w.state.def.contextName == "" || w.state.contextKey != "" {
		return []*namedWindowRuntime{w.state}
	}
	w.state.mu.RLock()
	keys := make([]string, 0, len(w.state.partitions))
	partitions := make(map[string]*namedWindowRuntime, len(w.state.partitions))
	for key, partition := range w.state.partitions {
		keys = append(keys, key)
		partitions[key] = partition
	}
	w.state.mu.RUnlock()
	sort.Strings(keys)
	result := make([]*namedWindowRuntime, 0, len(keys))
	for _, key := range keys {
		result = append(result, partitions[key])
	}
	return result
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
	if w.state.def.contextName == "" || w.state.contextKey != "" {
		return snapshotNamedWindowState(w.state), nil
	}
	w.state.mu.RLock()
	keys := make([]string, 0, len(w.state.partitions))
	partitions := make(map[string]*namedWindowRuntime, len(w.state.partitions))
	for key, partition := range w.state.partitions {
		keys = append(keys, key)
		partitions[key] = partition
	}
	w.state.mu.RUnlock()
	sort.Strings(keys)
	result := make([]Event, 0)
	for _, key := range keys {
		result = append(result, snapshotNamedWindowState(partitions[key])...)
	}
	return result, nil
}

// SnapshotContext returns the rows retained by one context partition. It is
// intentionally separate from Snapshot so fire-and-forget callers can still
// observe the complete context-bound window.
func (w *NamedWindow) SnapshotContext(ctx context.Context, partitionKey string) ([]Event, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if w == nil || w.state == nil {
		return nil, NewError(ErrorState, "nil named window")
	}
	state, err := w.partitionState(partitionKey, false)
	if err != nil {
		return nil, err
	}
	return snapshotNamedWindowState(state), nil
}

// Lookup returns the events matching one complete declared Named Window index
// key. It is an explicit Go-native access-path API for callers that need a
// keyed snapshot; query execution remains free to choose a plan from
// Plan.IndexPlan. Composite keys follow the declaration order. The method
// also works for context-bound windows and searches all active partitions when
// called on the root window.
func (w *NamedWindow) Lookup(ctx context.Context, indexName string, key ...any) ([]Event, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if w == nil || w.state == nil {
		return nil, NewError(ErrorState, "nil named window")
	}
	definition, exists := namedWindowIndexDefinition(w.state.def, indexName)
	if !exists {
		return nil, NewError(ErrorUnknownName, fmt.Sprintf("named window index %q does not exist", indexName))
	}
	if len(key) != len(definition.Columns) {
		return nil, NewError(ErrorTypeMismatch, fmt.Sprintf("named window index %q expects %d values, got %d", indexName, len(definition.Columns), len(key)))
	}
	if w.state.def.contextName == "" || w.state.contextKey != "" {
		return lookupNamedWindowState(w.state, indexName, key), nil
	}
	w.state.mu.RLock()
	partitionKeys := make([]string, 0, len(w.state.partitions))
	partitions := make(map[string]*namedWindowRuntime, len(w.state.partitions))
	for partitionKey, partition := range w.state.partitions {
		partitionKeys = append(partitionKeys, partitionKey)
		partitions[partitionKey] = partition
	}
	w.state.mu.RUnlock()
	sort.Strings(partitionKeys)
	result := make([]Event, 0)
	for _, partitionKey := range partitionKeys {
		result = append(result, lookupNamedWindowState(partitions[partitionKey], indexName, key)...)
	}
	return result, nil
}

func lookupNamedWindowState(state *namedWindowRuntime, indexName string, key []any) []Event {
	return lookupNamedWindowStateMany(state, indexName, [][]any{key})
}

func lookupNamedWindowStateMany(state *namedWindowRuntime, indexName string, keys [][]any) []Event {
	if state == nil {
		return nil
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	state.indexLookups.Add(1)
	index := state.indexes[indexName]
	wanted := make(map[int]struct{})
	for _, key := range keys {
		for _, position := range index[encodeKey(key)] {
			wanted[position] = struct{}{}
		}
	}
	result := make([]Event, 0, len(wanted))
	for position, entry := range state.entries {
		if _, exists := wanted[position]; exists {
			result = append(result, entry.event)
		}
	}
	return result
}

func (w *NamedWindow) lookupMany(ctx context.Context, indexName string, keys [][]any) ([]Event, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if w == nil || w.state == nil {
		return nil, NewError(ErrorState, "nil named window")
	}
	if _, exists := namedWindowIndexDefinition(w.state.def, indexName); !exists {
		return nil, NewError(ErrorUnknownName, fmt.Sprintf("named window index %q does not exist", indexName))
	}
	if w.state.def.contextName == "" || w.state.contextKey != "" {
		return lookupNamedWindowStateMany(w.state, indexName, keys), nil
	}
	w.state.mu.RLock()
	partitionKeys := make([]string, 0, len(w.state.partitions))
	partitions := make(map[string]*namedWindowRuntime, len(w.state.partitions))
	for partitionKey, partition := range w.state.partitions {
		partitionKeys = append(partitionKeys, partitionKey)
		partitions[partitionKey] = partition
	}
	w.state.mu.RUnlock()
	sort.Strings(partitionKeys)
	result := make([]Event, 0)
	for _, partitionKey := range partitionKeys {
		result = append(result, lookupNamedWindowStateMany(partitions[partitionKey], indexName, keys)...)
	}
	return result, nil
}

// lookupRange walks ordered Named Window index members and returns matched
// events in retention/insertion order. Context-bound root windows search
// partitions in the same deterministic order as Lookup.
func (w *NamedWindow) lookupRange(ctx context.Context, indexName string, query indexRangeSpec) ([]Event, error) {
	return w.lookupRangeMany(ctx, indexName, []indexRangeSpec{query})
}

func (w *NamedWindow) lookupRangeMany(ctx context.Context, indexName string, queries []indexRangeSpec) ([]Event, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if w == nil || w.state == nil {
		return nil, NewError(ErrorState, "nil named window")
	}
	if _, exists := namedWindowIndexDefinition(w.state.def, indexName); !exists {
		return nil, NewError(ErrorUnknownName, fmt.Sprintf("named window index %q does not exist", indexName))
	}
	if w.state.def.contextName == "" || w.state.contextKey != "" {
		return lookupNamedWindowRangeStateMany(w.state, indexName, queries, ctx)
	}
	w.state.mu.RLock()
	partitionKeys := make([]string, 0, len(w.state.partitions))
	partitions := make(map[string]*namedWindowRuntime, len(w.state.partitions))
	for partitionKey, partition := range w.state.partitions {
		partitionKeys = append(partitionKeys, partitionKey)
		partitions[partitionKey] = partition
	}
	w.state.mu.RUnlock()
	sort.Strings(partitionKeys)
	result := make([]Event, 0)
	for _, partitionKey := range partitionKeys {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		partitionResult, err := lookupNamedWindowRangeStateMany(partitions[partitionKey], indexName, queries, ctx)
		if err != nil {
			return nil, err
		}
		result = append(result, partitionResult...)
	}
	return result, nil
}

func lookupNamedWindowRangeState(state *namedWindowRuntime, indexName string, query indexRangeSpec, ctx context.Context) ([]Event, error) {
	return lookupNamedWindowRangeStateMany(state, indexName, []indexRangeSpec{query}, ctx)
}

func lookupNamedWindowRangeStateMany(state *namedWindowRuntime, indexName string, queries []indexRangeSpec, ctx context.Context) ([]Event, error) {
	if state == nil {
		return nil, nil
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	state.indexLookups.Add(1)
	entries := state.indexEntries[indexName]
	positions, cursorUsable, err := collectIndexRangeCursorPositions(ctx, len(entries), func(index int) []Value {
		return entries[index].values
	}, queries)
	if err != nil {
		return nil, err
	}
	wanted := make(map[int]struct{})
	if cursorUsable {
		for position := range positions {
			if position >= 0 && position < len(entries) {
				wanted[entries[position].position] = struct{}{}
			}
		}
	} else {
		for _, entry := range entries {
			if err := contextErr(ctx); err != nil {
				return nil, err
			}
			for _, query := range queries {
				if indexRangeEntryMatches(entry.values, query) {
					wanted[entry.position] = struct{}{}
					break
				}
			}
		}
	}
	result := make([]Event, 0, len(wanted))
	for position, entry := range state.entries {
		if _, exists := wanted[position]; exists {
			result = append(result, entry.event)
		}
	}
	return result, nil
}

func snapshotNamedWindowState(state *namedWindowRuntime) []Event {
	if state == nil {
		return nil
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	result := make([]Event, 0, len(state.entries))
	for _, entry := range state.entries {
		result = append(result, entry.event)
	}
	return result
}

type namedWindowMutationSnapshot struct {
	entries           []storedEvent
	keyed             map[string]storedEvent
	keyOrder          []string
	contextProperties map[string]Value
}

func (w *NamedWindow) snapshotMutationState() namedWindowMutationSnapshot {
	if w == nil || w.state == nil {
		return namedWindowMutationSnapshot{}
	}
	state := w.state
	state.mu.RLock()
	defer state.mu.RUnlock()
	snapshot := namedWindowMutationSnapshot{
		entries:           append([]storedEvent(nil), state.entries...),
		keyOrder:          append([]string(nil), state.keyOrder...),
		contextProperties: cloneValues(state.contextProperties),
	}
	if state.keyed != nil {
		snapshot.keyed = make(map[string]storedEvent, len(state.keyed))
		for key, entry := range state.keyed {
			snapshot.keyed[key] = entry
		}
	}
	return snapshot
}

func (w *NamedWindow) restoreMutationState(snapshot namedWindowMutationSnapshot) {
	if w == nil || w.state == nil {
		return
	}
	state := w.state
	state.mu.Lock()
	defer state.mu.Unlock()
	state.entries = append([]storedEvent(nil), snapshot.entries...)
	if snapshot.keyed != nil {
		state.keyed = make(map[string]storedEvent, len(snapshot.keyed))
		for key, entry := range snapshot.keyed {
			state.keyed[key] = entry
		}
	} else {
		state.keyed = nil
	}
	state.keyOrder = append([]string(nil), snapshot.keyOrder...)
	state.contextProperties = cloneValues(snapshot.contextProperties)
	rebuildNamedWindowIndexesLocked(state)
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
	if w.state.def.contextName != "" && w.state.contextKey == "" {
		result := NamedWindowDelta{Time: now}
		for _, state := range w.contextPartitionStates() {
			delta, err := w.deleteWhereState(ctx, state, predicate, now)
			if err != nil {
				return NamedWindowDelta{}, err
			}
			result.New = append(result.New, delta.New...)
			result.Old = append(result.Old, delta.Old...)
		}
		return result, nil
	}
	return w.deleteWhereState(ctx, w.state, predicate, now)
}

func (w *NamedWindow) deleteWherePartition(ctx context.Context, partitionKey string, predicate func(Event) bool) (NamedWindowDelta, error) {
	if err := contextErr(ctx); err != nil {
		return NamedWindowDelta{}, err
	}
	if w == nil || w.state == nil {
		return NamedWindowDelta{}, NewError(ErrorState, "nil named window")
	}
	if predicate == nil {
		return NamedWindowDelta{}, NewError(ErrorInvalidRule, "named-window delete predicate is required")
	}
	state, err := w.partitionState(partitionKey, false)
	if err != nil {
		return NamedWindowDelta{}, err
	}
	return w.deleteWhereState(ctx, state, predicate, w.now())
}

func (w *NamedWindow) deleteWhereState(ctx context.Context, state *namedWindowRuntime, predicate func(Event) bool, now time.Time) (NamedWindowDelta, error) {
	if state == nil {
		return NamedWindowDelta{}, NewError(ErrorState, "nil named-window partition")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
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
	w.rebuildUniqueStateForLocked(state)
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
	if w.state.def.contextName != "" && w.state.contextKey == "" {
		result := NamedWindowDelta{Time: now}
		for _, state := range w.contextPartitionStates() {
			delta, err := w.updateWhereState(ctx, state, predicate, update, now)
			if err != nil {
				return NamedWindowDelta{}, err
			}
			result.New = append(result.New, delta.New...)
			result.Old = append(result.Old, delta.Old...)
		}
		return result, nil
	}
	return w.updateWhereState(ctx, w.state, predicate, update, now)
}

func (w *NamedWindow) updateWherePartition(ctx context.Context, partitionKey string, predicate func(Event) bool, update func(Event) (any, error)) (NamedWindowDelta, error) {
	if err := contextErr(ctx); err != nil {
		return NamedWindowDelta{}, err
	}
	if w == nil || w.state == nil {
		return NamedWindowDelta{}, NewError(ErrorState, "nil named window")
	}
	if predicate == nil || update == nil {
		return NamedWindowDelta{}, NewError(ErrorInvalidRule, "named-window update requires predicate and updater")
	}
	state, err := w.partitionState(partitionKey, false)
	if err != nil {
		return NamedWindowDelta{}, err
	}
	return w.updateWhereState(ctx, state, predicate, update, w.now())
}

func (w *NamedWindow) updateWhereState(ctx context.Context, state *namedWindowRuntime, predicate func(Event) bool, update func(Event) (any, error), now time.Time) (NamedWindowDelta, error) {
	if state == nil {
		return NamedWindowDelta{}, NewError(ErrorState, "nil named-window partition")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	delta := NamedWindowDelta{Time: now}
	for index, entry := range state.entries {
		if !predicate(entry.event) {
			continue
		}
		underlying, err := update(entry.event)
		if err != nil {
			return NamedWindowDelta{}, err
		}
		updated, err := newEvent(state.def.schema, underlying, now)
		if err != nil {
			return NamedWindowDelta{}, err
		}
		updated.typeName = state.def.name
		updated.streamType = state.def.name
		delta.Old = append(delta.Old, entry.event)
		delta.New = append(delta.New, updated)
		state.entries[index] = storedEvent{event: updated, receivedAt: entry.receivedAt, expiresAt: entry.expiresAt}
	}
	w.rebuildUniqueStateForLocked(state)
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
		case KeepAllWindowSpec, LengthWindowSpec, LastEventWindowSpec, TimeWindowSpec, TimeToLiveWindowSpec, TimeToLiveAtWindowSpec, UniqueWindowSpec, SortedWindowSpec:
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
		case LastEventWindowSpec:
			if len(entries) > 0 {
				delta.Old = append(delta.Old, entries[len(entries)-1].event)
			}
			entries = []storedEvent{{event: preparedInsert, receivedAt: now}}
			delta.New = append(delta.New, preparedInsert)
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
			w.rebuildUniqueStateForLocked(state)
			delta.New = append(delta.New, preparedInsert)
			applySortedNamedWindowInsertLocked(state, retention, storedEvent{event: preparedInsert, receivedAt: now}, now, &delta)
			entries = state.entries
		}
	}
	state.entries = entries
	w.rebuildUniqueStateForLocked(state)
	return delta, nil
}

func (w *NamedWindow) now() time.Time {
	if w != nil && w.engine != nil {
		return w.engine.Now()
	}
	return time.Unix(0, 0).UTC()
}

func (w *NamedWindow) insert(now time.Time, underlying any) (NamedWindowDelta, error) {
	return w.insertWithVariables(now, underlying, nil)
}

func (w *NamedWindow) insertWithVariables(now time.Time, underlying any, variables map[string]Value) (NamedWindowDelta, error) {
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
	if state.def.contextName != "" && state.contextKey == "" {
		if partitionKey, fromContext := w.contextPartitionFromVariables(variables); fromContext {
			var err error
			state, err = w.partitionState(partitionKey, true)
			if err != nil {
				return NamedWindowDelta{}, err
			}
		} else {
			key, active, err := w.contextPartitionKey(event, now, variables)
			if err != nil {
				return NamedWindowDelta{}, err
			}
			if !active {
				return NamedWindowDelta{Time: now}, nil
			}
			state, err = w.partitionState(key, true)
			if err != nil {
				return NamedWindowDelta{}, err
			}
		}
	}
	if state.def.contextName != "" {
		if definition, ok := w.engineContextDefinition(); ok {
			properties := definition.contextPropertyValues(event, now, variables, 0)
			for name, value := range contextPropertiesFromVariables(variables) {
				properties[name] = value
			}
			state.rememberContextProperties(properties)
		}
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if err := validateNamedWindowUniqueIndexesLocked(state, event); err != nil {
		return NamedWindowDelta{}, err
	}
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
	case LastEventWindowSpec:
		if len(state.entries) > 0 {
			delta.Old = append(delta.Old, state.entries[len(state.entries)-1].event)
		}
		state.entries = []storedEvent{entry}
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
					rebuildNamedWindowIndexesLocked(state)
					return delta, nil
				}
			}
			state.keyed[key] = entry
			state.entries = append(state.entries, entry)
			rebuildNamedWindowIndexesLocked(state)
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
	rebuildNamedWindowIndexesLocked(state)
	return delta, nil
}

func (w *NamedWindow) expire(at time.Time) NamedWindowDelta {
	if w == nil || w.state == nil {
		return NamedWindowDelta{}
	}
	if w.state.def.contextName != "" && w.state.contextKey == "" {
		w.state.mu.RLock()
		partitions := make([]*namedWindowRuntime, 0, len(w.state.partitions))
		partitionKeys := make([]string, 0, len(w.state.partitions))
		for _, partition := range w.state.partitions {
			partitions = append(partitions, partition)
		}
		for key := range w.state.partitions {
			partitionKeys = append(partitionKeys, key)
		}
		w.state.mu.RUnlock()
		if definition, ok := w.engineContextDefinition(); ok && definition.isTemporal() {
			activeKey := w.activeTemporalPartitionKey(definition, at)
			stale := make(map[string]struct{})
			for _, key := range partitionKeys {
				if key != activeKey {
					stale[key] = struct{}{}
				}
			}
			if len(stale) > 0 {
				w.state.mu.Lock()
				for key := range stale {
					delete(w.state.partitions, key)
				}
				w.state.mu.Unlock()
			}
			result := NamedWindowDelta{Time: at}
			for _, partition := range partitions {
				if partition == nil || partition.contextKey != activeKey {
					continue
				}
				delta := expireNamedWindowState(partition, at)
				result.New = append(result.New, delta.New...)
				result.Old = append(result.Old, delta.Old...)
			}
			return result
		}
		result := NamedWindowDelta{Time: at}
		for _, partition := range partitions {
			delta := expireNamedWindowState(partition, at)
			result.New = append(result.New, delta.New...)
			result.Old = append(result.Old, delta.Old...)
		}
		return result
	}
	return expireNamedWindowState(w.state, at)
}

func (w *NamedWindow) engineContextDefinition() (ContextDefinition, bool) {
	if w == nil || w.engine == nil || w.engine.env == nil || w.state == nil || w.state.def.contextName == "" {
		return ContextDefinition{}, false
	}
	return w.engine.env.Context(w.state.def.contextName)
}

func (w *NamedWindow) activeTemporalPartitionKey(definition ContextDefinition, now time.Time) string {
	if w == nil || w.engine == nil {
		return ""
	}
	return activeTemporalContextPartitionKey(w.engine, definition, now)
}

func expireNamedWindowState(state *namedWindowRuntime, at time.Time) NamedWindowDelta {
	if state == nil {
		return NamedWindowDelta{}
	}
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
		rebuildNamedWindowIndexesLocked(state)
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
	rebuildNamedWindowIndexesLocked(state)
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
