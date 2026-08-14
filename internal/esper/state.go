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
		return TableDefinition{}, duplicateModuleObjectError(DeploymentResourceTable, key)
	}
	definition.moduleName = moduleName
	definition.schema.name = "table:" + key
	e.tables[key] = definition
	if moduleName != "" {
		e.moduleObjects[key] = moduleName
	}
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
	values   map[string]Value
	version  uint64
	identity uint64
	// scope is empty for the ordinary/root table state. Context-bound
	// runtime views keep their logical context scope here so the Engine can
	// route a legacy root row and a partition-local row through the correct
	// mutation path without exposing storage details in the public row API.
	scope string
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
	scope        string
	identity     *tableIdentitySource
	rows         map[string]TableRow
	order        []string
	indexes      map[string]map[string][]string
	indexEntries map[string][]tableIndexEntry
	version      uint64
	nextIdentity uint64
	indexLookups atomic.Uint64
}

type tableIdentitySource struct {
	mu   sync.Mutex
	next uint64
}

func (source *tableIdentitySource) allocate() uint64 {
	if source == nil {
		return 0
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	source.next++
	return source.next
}

func (source *tableIdentitySource) value() uint64 {
	if source == nil {
		return 0
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	return source.next
}

func (source *tableIdentitySource) restore(next uint64) {
	if source == nil {
		return
	}
	source.mu.Lock()
	source.next = next
	source.mu.Unlock()
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
	state       *tableState
	identity    *tableIdentitySource
	scopesMu    sync.RWMutex
	scopedState map[string]*tableState
}

type tableMutationSnapshot struct {
	rows          map[string]TableRow
	order         []string
	indexes       map[string]map[string][]string
	indexEntries  map[string][]tableIndexEntry
	version       uint64
	nextIdentity  uint64
	scopes        map[string]tableStateMutationSnapshot
	allocatorNext uint64
}

type tableStateMutationSnapshot struct {
	rows         map[string]TableRow
	order        []string
	indexes      map[string]map[string][]string
	indexEntries map[string][]tableIndexEntry
	version      uint64
	nextIdentity uint64
}

func snapshotTableState(state *tableState) tableStateMutationSnapshot {
	if state == nil {
		return tableStateMutationSnapshot{}
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	snapshot := tableStateMutationSnapshot{
		rows:         make(map[string]TableRow, len(state.rows)),
		order:        append([]string(nil), state.order...),
		indexes:      make(map[string]map[string][]string, len(state.indexes)),
		indexEntries: make(map[string][]tableIndexEntry, len(state.indexEntries)),
		version:      state.version,
		nextIdentity: state.nextIdentity,
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

func restoreTableState(state *tableState, snapshot tableStateMutationSnapshot) {
	if state == nil {
		return
	}
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
	state.nextIdentity = snapshot.nextIdentity
}

func (t *Table) snapshotMutationState() tableMutationSnapshot {
	if t == nil || t.state == nil {
		return tableMutationSnapshot{}
	}
	snapshot := tableMutationSnapshot{
		allocatorNext: t.identity.value(),
	}
	root := snapshotTableState(t.state)
	snapshot.rows = root.rows
	snapshot.order = root.order
	snapshot.indexes = root.indexes
	snapshot.indexEntries = root.indexEntries
	snapshot.version = root.version
	snapshot.nextIdentity = root.nextIdentity
	t.scopesMu.RLock()
	states := make(map[string]*tableState, len(t.scopedState))
	for scope, state := range t.scopedState {
		states[scope] = state
	}
	t.scopesMu.RUnlock()
	if len(states) > 0 {
		snapshot.scopes = make(map[string]tableStateMutationSnapshot, len(states))
		for scope, state := range states {
			snapshot.scopes[scope] = snapshotTableState(state)
		}
	}
	return snapshot
}

func (t *Table) restoreMutationState(snapshot tableMutationSnapshot) {
	if t == nil || t.state == nil {
		return
	}
	root := tableStateMutationSnapshot{
		rows: snapshot.rows, order: snapshot.order, indexes: snapshot.indexes,
		indexEntries: snapshot.indexEntries, version: snapshot.version, nextIdentity: snapshot.nextIdentity,
	}
	restoreTableState(t.state, root)
	t.scopesMu.Lock()
	current := t.scopedState
	if current == nil {
		current = make(map[string]*tableState)
	}
	restored := make(map[string]*tableState, len(snapshot.scopes))
	for scope := range snapshot.scopes {
		state := current[scope]
		if state == nil {
			state = newTableState(t.state.def, scope, t.identity)
		}
		restored[scope] = state
	}
	t.scopedState = restored
	t.scopesMu.Unlock()
	for scope, state := range restored {
		restoreTableState(state, snapshot.scopes[scope])
	}
	t.identity.restore(snapshot.allocatorNext)
}

func newTableState(definition TableDefinition, scope string, identity *tableIdentitySource) *tableState {
	indexes := make(map[string]map[string][]string, len(definition.indexes))
	indexEntries := make(map[string][]tableIndexEntry, len(definition.indexes))
	for _, index := range definition.indexes {
		indexes[index.Name] = make(map[string][]string)
		indexEntries[index.Name] = nil
	}
	return &tableState{def: definition, scope: scope, identity: identity, rows: make(map[string]TableRow), indexes: indexes, indexEntries: indexEntries}
}

func newTable(definition TableDefinition) *Table {
	identity := &tableIdentitySource{}
	return &Table{state: newTableState(definition, "", identity), identity: identity, scopedState: make(map[string]*tableState)}
}

func (t *Table) Definition() TableDefinition {
	if t == nil || t.state == nil {
		return TableDefinition{}
	}
	return t.state.def
}

// tableContextScope is the internal storage key for a logical context
// partition. It is deliberately not part of TableRow values or the public
// Table API: callers still use ordinary typed rows while the runtime selects
// the appropriate state view from the statement context.
func tableContextScope(contextName, partitionKey string) string {
	if contextName == "" || partitionKey == "" {
		return ""
	}
	return contextName + "\x00" + partitionKey
}

func contextTableScopeFromVariables(variables map[string]Value) string {
	contextName, partitionKey, ok := contextTableScopeValues(variables)
	if !ok {
		return ""
	}
	return tableContextScope(contextName, partitionKey)
}

func contextTableScopeValues(variables map[string]Value) (string, string, bool) {
	if len(variables) == 0 {
		return "", "", false
	}
	nameValue, nameOK := variables[subqueryContextNameVariable]
	keyValue, keyOK := variables[subqueryContextPartitionVariable]
	if !nameOK || !keyOK || !nameValue.IsPresent() || !keyValue.IsPresent() {
		return "", "", false
	}
	contextName, nameOK := nameValue.Any().(string)
	partitionKey, keyOK := keyValue.Any().(string)
	if !nameOK || !keyOK {
		return "", "", false
	}
	return contextName, partitionKey, contextName != "" && partitionKey != ""
}

func (t *Table) stateForScope(scope string, create bool) (*tableState, error) {
	if t == nil || t.state == nil {
		return nil, NewError(ErrorState, "nil table")
	}
	if scope == "" {
		return t.state, nil
	}
	t.scopesMu.RLock()
	state := t.scopedState[scope]
	t.scopesMu.RUnlock()
	if state != nil || !create {
		if state == nil {
			return nil, nil
		}
		return state, nil
	}
	t.scopesMu.Lock()
	defer t.scopesMu.Unlock()
	if state = t.scopedState[scope]; state == nil {
		state = newTableState(t.state.def, scope, t.identity)
		t.scopedState[scope] = state
	}
	return state, nil
}

func (t *Table) statesSnapshot() []*tableState {
	if t == nil || t.state == nil {
		return nil
	}
	states := []*tableState{t.state}
	t.scopesMu.RLock()
	keys := make([]string, 0, len(t.scopedState))
	for key := range t.scopedState {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		states = append(states, t.scopedState[key])
	}
	t.scopesMu.RUnlock()
	return states
}

// lookupScopesSnapshot returns the root state followed by every currently
// materialized Context state in deterministic scope order. The root state is
// always present for a valid Table; a missing scoped state is intentionally
// omitted because an index probe against it cannot produce a row.
func (t *Table) lookupScopesSnapshot() []string {
	if t == nil || t.state == nil {
		return nil
	}
	scopes := []string{""}
	t.scopesMu.RLock()
	keys := make([]string, 0, len(t.scopedState))
	for key := range t.scopedState {
		keys = append(keys, key)
	}
	t.scopesMu.RUnlock()
	sort.Strings(keys)
	return append(scopes, keys...)
}

func (t *Table) hasScopedState() bool {
	if t == nil {
		return false
	}
	t.scopesMu.RLock()
	defer t.scopesMu.RUnlock()
	return len(t.scopedState) > 0
}

// releaseContextPartition drops the physical state for one logical context
// partition. The state object is not reused: a later partition with the same
// key must receive a fresh table view, just as Esper creates a fresh
// partition-local table instance after a lifecycle-managed Context partition
// is destroyed.
func (t *Table) releaseContextPartition(contextName, partitionKey string) {
	if t == nil || contextName == "" || partitionKey == "" {
		return
	}
	scope := tableContextScope(contextName, partitionKey)
	if scope == "" {
		return
	}
	t.scopesMu.Lock()
	delete(t.scopedState, scope)
	t.scopesMu.Unlock()
}

func (t *Table) upsertInScope(ctx context.Context, scope string, values map[string]any, insertOnly bool) (TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return TableRow{}, err
	}
	state, err := t.stateForScope(scope, true)
	if err != nil {
		return TableRow{}, err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.upsert(values, insertOnly, t.identity.allocate)
}

func (t *Table) insertInScope(ctx context.Context, scope string, values map[string]any) (TableRow, error) {
	return t.upsertInScope(ctx, scope, values, true)
}

func (t *Table) upsertExistingInScope(ctx context.Context, scope string, values map[string]any) (TableRow, error) {
	return t.upsertInScope(ctx, scope, values, false)
}

func (t *Table) Upsert(ctx context.Context, values map[string]any) (TableRow, error) {
	return t.upsertExistingInScope(ctx, "", values)
}

// Replace atomically replaces the complete table snapshot. It is used by
// materialized aggregate statements so a group removal cannot leave stale
// rows behind, and a failed conversion or cancelled context cannot partially
// update the table.
func (t *Table) Replace(ctx context.Context, rows []map[string]any) error {
	return t.replaceInScope(ctx, "", rows)
}

func (t *Table) replaceInScope(ctx context.Context, scope string, rows []map[string]any) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if t == nil || t.state == nil {
		return NewError(ErrorState, "nil table")
	}
	state, err := t.stateForScope(scope, true)
	if err != nil {
		return err
	}
	state.mu.Lock()
	defer state.mu.Unlock()

	replacement := newTableState(state.def, state.scope, t.identity)
	replacement.version = state.version
	replacement.nextIdentity = state.nextIdentity
	for index, values := range rows {
		if err := contextErr(ctx); err != nil {
			return err
		}
		if _, err := replacement.upsert(values, false, t.identity.allocate); err != nil {
			return WrapError(ErrorState, fmt.Sprintf("table replacement row %d", index), err)
		}
	}
	for key, row := range replacement.rows {
		if previous, exists := state.rows[key]; exists {
			row.identity = previous.identity
			row.scope = previous.scope
			replacement.rows[key] = row
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
	return t.insertInScope(ctx, "", values)
}

func (t *Table) Update(ctx context.Context, key []any, values map[string]any) (TableRow, error) {
	return t.updateInScope(ctx, "", key, values)
}

func (t *Table) updateInScope(ctx context.Context, scope string, key []any, values map[string]any) (TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return TableRow{}, err
	}
	state, err := t.stateForScope(scope, false)
	if err != nil {
		return TableRow{}, err
	}
	if state == nil {
		return TableRow{}, NewError(ErrorUnknownName, "table row does not exist")
	}
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
	updated := TableRow{values: converted, identity: existing.identity, scope: state.scope}
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
	return t.deleteInScope(ctx, "", key...)
}

func (t *Table) deleteInScope(ctx context.Context, scope string, key ...any) (TableRow, bool, error) {
	if err := contextErr(ctx); err != nil {
		return TableRow{}, false, err
	}
	state, err := t.stateForScope(scope, false)
	if err != nil {
		return TableRow{}, false, err
	}
	if state == nil {
		return TableRow{}, false, nil
	}
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
	return t.getInScope(ctx, "", key...)
}

func (t *Table) getInScope(ctx context.Context, scope string, key ...any) (TableRow, bool, error) {
	if err := contextErr(ctx); err != nil {
		return TableRow{}, false, err
	}
	state, err := t.stateForScope(scope, false)
	if err != nil {
		return TableRow{}, false, err
	}
	if state == nil {
		return TableRow{}, false, nil
	}
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
	rows := make([]TableRow, 0)
	for _, state := range t.statesSnapshot() {
		state.mu.RLock()
		for _, key := range state.order {
			if row, ok := state.rows[key]; ok {
				rows = append(rows, cloneTableRow(row))
			}
		}
		state.mu.RUnlock()
	}
	sort.SliceStable(rows, func(left, right int) bool { return rows[left].identity < rows[right].identity })
	return rows, nil
}

func (t *Table) snapshotInScope(ctx context.Context, scope string) ([]TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	state, err := t.stateForScope(scope, false)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
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

func (t *Table) hasRowsInScope(ctx context.Context, scope string) (bool, error) {
	if err := contextErr(ctx); err != nil {
		return false, err
	}
	state, err := t.stateForScope(scope, false)
	if err != nil {
		return false, err
	}
	if state == nil {
		return false, nil
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	return len(state.rows) > 0, nil
}

func (t *Table) Clear(ctx context.Context) ([]TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if t == nil || t.state == nil {
		return nil, NewError(ErrorState, "nil table")
	}
	rows := make([]TableRow, 0)
	for _, state := range t.statesSnapshot() {
		cleared, err := clearTableState(ctx, state)
		if err != nil {
			return nil, err
		}
		rows = append(rows, cleared...)
	}
	sort.SliceStable(rows, func(left, right int) bool { return rows[left].identity < rows[right].identity })
	return rows, nil
}

func clearTableState(ctx context.Context, state *tableState) ([]TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
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

func (t *Table) clearInScope(ctx context.Context, scope string) ([]TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	state, err := t.stateForScope(scope, false)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
	return clearTableState(ctx, state)
}

func (t *Table) Lookup(ctx context.Context, indexName string, key ...any) ([]TableRow, error) {
	return t.lookupInScope(ctx, "", indexName, key...)
}

func (t *Table) lookupInScope(ctx context.Context, scope, indexName string, key ...any) ([]TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	state, err := t.stateForScope(scope, false)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
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
	return t.lookupManyInScope(ctx, "", indexName, keys)
}

func (t *Table) lookupManyInScope(ctx context.Context, scope, indexName string, keys [][]any) ([]TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	state, err := t.stateForScope(scope, false)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
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

// lookupManyAllScopes is the complete-state counterpart used by ordinary FAF
// and Join candidate execution. A Context table is physically represented by
// one root state plus zero or more partition states; probing every state keeps
// the candidate set equivalent to Table.Snapshot while retaining index
// pruning.
func (t *Table) lookupManyAllScopes(ctx context.Context, indexName string, keys [][]any) ([]TableRow, error) {
	rows := make([]TableRow, 0)
	for _, scope := range t.lookupScopesSnapshot() {
		part, err := t.lookupManyInScope(ctx, scope, indexName, keys)
		if err != nil {
			return nil, err
		}
		rows = append(rows, part...)
	}
	sort.SliceStable(rows, func(left, right int) bool { return rows[left].identity < rows[right].identity })
	return rows, nil
}

// lookupPrimaryMany is the complete-key counterpart for a table primary key.
// Primary keys are row identity rather than secondary index buckets, but FAF
// candidate execution still needs one consistent snapshot and insertion-order
// result assembly when probing several keys.
func (t *Table) lookupPrimaryMany(ctx context.Context, keys [][]any) ([]TableRow, error) {
	return t.lookupPrimaryManyInScope(ctx, "", keys)
}

func (t *Table) lookupPrimaryManyInScope(ctx context.Context, scope string, keys [][]any) ([]TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	state, err := t.stateForScope(scope, false)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
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

func (t *Table) lookupPrimaryManyAllScopes(ctx context.Context, keys [][]any) ([]TableRow, error) {
	rows := make([]TableRow, 0)
	for _, scope := range t.lookupScopesSnapshot() {
		part, err := t.lookupPrimaryManyInScope(ctx, scope, keys)
		if err != nil {
			return nil, err
		}
		rows = append(rows, part...)
	}
	sort.SliceStable(rows, func(left, right int) bool { return rows[left].identity < rows[right].identity })
	return rows, nil
}

// lookupRange walks the ordered members of a B-tree index and returns matched
// rows in table insertion order. The ordered members are kept separately from
// the hash buckets so the runtime never has to reverse-engineer encodeKey.
func (t *Table) lookupRange(ctx context.Context, indexName string, query indexRangeSpec) ([]TableRow, error) {
	return t.lookupRangeMany(ctx, indexName, []indexRangeSpec{query})
}

func (t *Table) lookupRangeInScope(ctx context.Context, scope, indexName string, query indexRangeSpec) ([]TableRow, error) {
	return t.lookupRangeManyInScope(ctx, scope, indexName, []indexRangeSpec{query})
}

// lookupRangeMany unions several complete range probes in one locked
// insertion-order snapshot. FAF Join range candidates can be driven by more
// than one loaded-side tuple; probing them as a batch avoids duplicate rows
// and keeps result order independent of probe order.
func (t *Table) lookupRangeMany(ctx context.Context, indexName string, queries []indexRangeSpec) ([]TableRow, error) {
	return t.lookupRangeManyInScope(ctx, "", indexName, queries)
}

func (t *Table) lookupRangeManyInScope(ctx context.Context, scope, indexName string, queries []indexRangeSpec) ([]TableRow, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	state, err := t.stateForScope(scope, false)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
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

func (t *Table) lookupRangeManyAllScopes(ctx context.Context, indexName string, queries []indexRangeSpec) ([]TableRow, error) {
	rows := make([]TableRow, 0)
	for _, scope := range t.lookupScopesSnapshot() {
		part, err := t.lookupRangeManyInScope(ctx, scope, indexName, queries)
		if err != nil {
			return nil, err
		}
		rows = append(rows, part...)
	}
	sort.SliceStable(rows, func(left, right int) bool { return rows[left].identity < rows[right].identity })
	return rows, nil
}

func (s *tableState) upsert(values map[string]any, insertOnly bool, allocateIdentity func() uint64) (TableRow, error) {
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
		row.identity = old.identity
		row.scope = old.scope
		s.removeIndexesLocked(rowKey, old)
	} else {
		if allocateIdentity != nil {
			row.identity = allocateIdentity()
		} else {
			s.nextIdentity++
			row.identity = s.nextIdentity
		}
		if row.identity > s.nextIdentity {
			s.nextIdentity = row.identity
		}
		row.scope = s.scope
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
	return TableRow{values: row.Values(), version: row.version, identity: row.identity, scope: row.scope}
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
	name                 string
	moduleName           string
	schema               Schema
	retention            WindowSpec
	contextName          string
	subqueryIndexSharing bool
	indexes              []NamedWindowIndexDefinition
	uniqueIndexes        []NamedWindowIndexDefinition
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

// NamedWindowSubqueryIndexSharing allows correlated subqueries to create and
// reuse an internal equality/hash access path when no declared index matches.
// The index is still only a candidate-pruning mechanism: the complete
// subquery predicate is evaluated after lookup, preserving ordinary subquery
// cardinality and null semantics.
func NamedWindowSubqueryIndexSharing() NamedWindowOption {
	return func(config *namedWindowConfig) { config.subqueryIndexSharing = true }
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
	retention            WindowSpec
	contextName          string
	subqueryIndexSharing bool
	indexes              []NamedWindowIndexDefinition
	uniqueIndexes        []NamedWindowIndexDefinition
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
	if grouped, ok := config.retention.(GroupWindowSpec); ok {
		switch grouped.Inner.(type) {
		case LengthWindowSpec, TimeBatchWindowSpec:
		default:
			// Esper rejects named-window view chains whose groupwin child is
			// not a data window view at compile time (for example
			// #groupwin(value)#uni(value)); Go mirrors that boundary for the
			// grouped retentions the named-window runtime honors.
			return NamedWindowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("named-window grouped retention %T is not a supported data window view", grouped.Inner))
		}
	}
	if composite, ok := config.retention.(CompositeWindowSpec); ok {
		if len(composite.Windows) < 2 {
			return NamedWindowDefinition{}, NewError(ErrorInvalidRule, "named-window composite retention requires at least two child views")
		}
		for _, child := range composite.Windows {
			switch child.(type) {
			case KeepAllWindowSpec, LengthWindowSpec, UniqueWindowSpec, TimeWindowSpec, FirstLengthWindowSpec:
			default:
				return NamedWindowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("named-window composite retention %T is not a supported data window view", child))
			}
		}
	}
	seenIndexes := make(map[string]struct{}, len(config.indexes))
	indexes := make([]NamedWindowIndexDefinition, 0, len(config.indexes))
	uniqueIndexes := make([]NamedWindowIndexDefinition, 0, len(config.indexes))
	for index, definition := range config.indexes {
		definition.Name = strings.TrimSpace(definition.Name)
		if definition.Name == "" || len(definition.Columns) == 0 {
			return NamedWindowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("named-window index %d requires a name and columns", index+1))
		}
		if isSubquerySharedIndex(definition.Name) {
			return NamedWindowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("named-window index %q uses a reserved internal index name", definition.Name))
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
	return NamedWindowDefinition{
		name:                 name,
		schema:               schema,
		retention:            config.retention,
		contextName:          config.contextName,
		subqueryIndexSharing: config.subqueryIndexSharing,
		indexes:              indexes,
		uniqueIndexes:        uniqueIndexes,
	}, nil
}

func (d NamedWindowDefinition) Name() string               { return d.name }
func (d NamedWindowDefinition) Module() string             { return d.moduleName }
func (d NamedWindowDefinition) Schema() Schema             { return d.schema }
func (d NamedWindowDefinition) Retention() WindowSpec      { return d.retention }
func (d NamedWindowDefinition) Context() string            { return d.contextName }
func (d NamedWindowDefinition) SubqueryIndexSharing() bool { return d.subqueryIndexSharing }
func (d NamedWindowDefinition) Indexes() []NamedWindowIndexDefinition {
	public := make([]NamedWindowIndexDefinition, 0, len(d.indexes))
	for _, index := range d.indexes {
		if isSubquerySharedIndex(index.Name) {
			continue
		}
		public = append(public, index)
	}
	return cloneNamedWindowIndexDefinitions(public)
}

// UniqueIndexes returns only strict unique constraints. Indexes returns both
// unique and non-unique declarations.
func (d NamedWindowDefinition) UniqueIndexes() []NamedWindowIndexDefinition {
	public := make([]NamedWindowIndexDefinition, 0, len(d.uniqueIndexes))
	for _, index := range d.uniqueIndexes {
		if isSubquerySharedIndex(index.Name) {
			continue
		}
		public = append(public, index)
	}
	return cloneNamedWindowIndexDefinitions(public)
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
		return NamedWindowDefinition{}, duplicateModuleObjectError(DeploymentResourceNamedWindow, key)
	}
	definition.moduleName = moduleName
	e.namedWindows[key] = definition
	if moduleName != "" {
		e.moduleObjects[key] = moduleName
	}
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
	// External marks a delta caused by an explicit insert/update/delete/merge
	// mutation rather than an internal time-based expiry. Consumers use it to
	// distinguish external old-only batches (which produce new aggregate
	// rows) from pure expiry batches (which are silent under istream).
	External bool
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
	timeBatchBoundary time.Time
	extBatchBoundary  time.Time
	maxExtTimestamp   time.Time
	batchLast         []Event
	indexes           map[string]map[string][]int
	indexEntries      map[string][]namedWindowIndexEntry
	keyed             map[string]storedEvent
	keyOrder          []string
	listeners         map[uint64]NamedWindowListener
	nextID            uint64
	entrySeq          uint64
	indexLookups      atomic.Uint64
	// compositeChildren holds one child runtime per child view of an
	// intersecting composite retention (#length(2)#unique(x)); the window
	// contents are the intersection of the child contents.
	compositeChildren []*namedWindowRuntime
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
	if composite, ok := definition.retention.(CompositeWindowSpec); ok {
		children := make([]*namedWindowRuntime, len(composite.Windows))
		for index, childSpec := range composite.Windows {
			childDefinition := definition
			childDefinition.retention = childSpec
			childDefinition.indexes = nil
			children[index] = newNamedWindowRuntime(childDefinition, contextKey)
		}
		state.compositeChildren = children
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
		return sortedWindowEntryLess(normalized[i], normalized[j], retention.Keys, retention.Rank, now, nil)
	})
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
	if entry.lineageID == 0 {
		state.entrySeq++
		entry.lineageID = state.entrySeq
	}
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
		return sortedWindowEntryLess(entries[i], entries[j], retention.Keys, retention.Rank, now, nil)
	})
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

// applyExpressionNamedWindowRetentionLocked re-evaluates an expression-window
// keep predicate over the current named-window entries after a delete. The
// predicate keeps popping the oldest rows until it holds, exactly like the
// ordinary expression-window expiry loop, and returns the reduced entries
// together with the rows it expelled.
func applyExpressionNamedWindowRetentionLocked(state *namedWindowRuntime, retention ExpressionWindowSpec, now time.Time) ([]storedEvent, []Event) {
	if state == nil || len(state.entries) == 0 {
		return state.entries, nil
	}
	entries := append([]storedEvent(nil), state.entries...)
	var removed []Event
	for len(entries) > 0 && !windowPredicate(retention.Keep, entries, now, nil, 0) {
		removed = append(removed, entries[0].event)
		entries = entries[1:]
	}
	return entries, removed
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

const (
	subquerySharedHashIndexPrefix  = "<subquery-shared-hash:"
	subquerySharedBTreeIndexPrefix = "<subquery-shared-btree:"
)

func isSubquerySharedIndex(name string) bool {
	name = strings.TrimSpace(name)
	return strings.HasPrefix(name, subquerySharedHashIndexPrefix) || strings.HasPrefix(name, subquerySharedBTreeIndexPrefix)
}

func subquerySharedIndexKind(name string) (IndexKind, bool) {
	name = strings.TrimSpace(name)
	switch {
	case strings.HasPrefix(name, subquerySharedHashIndexPrefix):
		return IndexHash, true
	case strings.HasPrefix(name, subquerySharedBTreeIndexPrefix):
		return IndexBTree, true
	default:
		return IndexHash, false
	}
}

// ensureSubquerySharedIndex installs one internal hash or B-tree index on the
// root Named Window runtime and propagates it to every already-materialized
// Context partition. Future partitions inherit the root definition through
// partitionState. The environment definition remains unchanged: shared
// indexes are runtime access paths, not user-declared infrastructure indexes.
func (w *NamedWindow) ensureSubquerySharedIndex(name string, columns []string) error {
	if w == nil || w.state == nil {
		return NewError(ErrorState, "nil named window")
	}
	name = strings.TrimSpace(name)
	kind, kindOK := subquerySharedIndexKind(name)
	if !kindOK || len(columns) == 0 {
		return NewError(ErrorInvalidRule, "invalid subquery shared index definition")
	}
	if !w.state.def.subqueryIndexSharing {
		return NewError(ErrorInvalidRule, fmt.Sprintf("named window %q does not enable subquery index sharing", w.state.def.name))
	}
	definition := NamedWindowIndexDefinition{
		Name:    name,
		Columns: append([]string(nil), columns...),
		Kind:    kind,
	}
	for _, column := range definition.Columns {
		if _, ok := w.state.def.schema.Field(column); !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("subquery shared index %q references unknown column %q", name, column))
		}
	}

	root := w.state
	root.mu.Lock()
	for _, existing := range root.def.indexes {
		if existing.Name != name {
			continue
		}
		if !isSubquerySharedIndex(existing.Name) || !reflect.DeepEqual(existing.Columns, definition.Columns) || existing.Kind != definition.Kind {
			root.mu.Unlock()
			return NewError(ErrorDependency, fmt.Sprintf("subquery shared index %q conflicts with an existing named-window index", name))
		}
		root.mu.Unlock()
		return nil
	}
	root.def.indexes = append(root.def.indexes, definition)
	if root.indexes == nil {
		root.indexes = make(map[string]map[string][]int)
	}
	if root.indexEntries == nil {
		root.indexEntries = make(map[string][]namedWindowIndexEntry)
	}
	// Snapshot pointers while holding the root lock. The definition is added
	// before release, so a concurrently-created partition will inherit it.
	partitions := make([]*namedWindowRuntime, 0, len(root.partitions))
	for _, partition := range root.partitions {
		partitions = append(partitions, partition)
	}
	rebuildNamedWindowIndexesLocked(root)
	root.mu.Unlock()

	for _, partition := range partitions {
		if partition == nil {
			continue
		}
		partition.mu.Lock()
		alreadyPresent := false
		for _, existing := range partition.def.indexes {
			if existing.Name == name {
				alreadyPresent = true
				break
			}
		}
		if !alreadyPresent {
			partition.def.indexes = append(partition.def.indexes, definition)
			rebuildNamedWindowIndexesLocked(partition)
		}
		partition.mu.Unlock()
	}
	return nil
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
		result := NamedWindowDelta{Time: now, External: true}
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
	delta := NamedWindowDelta{Time: now, External: true}
	for _, entry := range state.entries {
		if predicate(entry.event) {
			delta.Old = append(delta.Old, entry.event)
		} else {
			kept = append(kept, entry)
		}
	}
	state.entries = kept
	switch retention := state.def.retention.(type) {
	case ExpressionWindowSpec:
		// #expr(keep) named-window retention: an on-delete that leaves
		// rows behind lets the keep predicate re-evaluate over the
		// reduced window and expels rows that no longer qualify. This
		// mirrors Java ViewExpressionWindowAggregationWOnDelete where
		// deleting the heavy row makes previously accumulated rows pass
		// or fail the aggregate threshold again.
		reduced, removed := applyExpressionNamedWindowRetentionLocked(state, retention, now)
		if len(removed) > 0 {
			delta.Old = append(delta.Old, removed...)
		}
		state.entries = reduced
	case ExpressionBatchWindowSpec:
		// #expr_batch(trigger) named-window retention: deletes can only
		// remove rows from the accumulating batch; the trigger predicate
		// is not re-evaluated by a delete. Pending batch delivery remains
		// driven by the next insert boundary.
		reduced := state.entries[:0]
		for _, entry := range state.entries {
			if containsEvent(reduced, entry.event) || containsEvent(kept, entry.event) {
				reduced = append(reduced, entry)
			}
		}
		state.entries = reduced
		delta.Old = nil
	case TimeBatchWindowSpec, LengthBatchWindowSpec, TimeLengthBatchWindowSpec:
		// Events accumulated in the current batch were never delivered
		// as new data, so deleting them produces no remove stream either.
		delta.Old = nil
	case GroupWindowSpec:
		// The same silent-delete rule applies when the grouped inner view
		// accumulates silently (#groupwin(key)#time_batch).
		if _, isBatch := retention.Inner.(TimeBatchWindowSpec); isBatch {
			delta.Old = nil
		}
	case CompositeWindowSpec:
		// Esper forwards intersection removes to every child view, so the
		// child states stay consistent with the window contents.
		removeFromNamedWindowCompositeChildrenLocked(state, delta.Old)
	}
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
		result := NamedWindowDelta{Time: now, External: true}
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
	if composite, ok := state.def.retention.(CompositeWindowSpec); ok {
		return w.updateCompositeWhereState(state, predicate, update, now, composite)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	delta := NamedWindowDelta{Time: now, External: true}
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

// updateCompositeWhereState applies on-update to an intersecting view stack.
// Each matched event leaves every child view and its replacement enters every
// child view (Esper pushes insert+remove through the intersection); the
// window contents are then recomputed as the intersection of the child
// contents, so a replacement expelled by one child (or an update that expels
// an untouched row through a child view) leaves the window as old data.
func (w *NamedWindow) updateCompositeWhereState(state *namedWindowRuntime, predicate func(Event) bool, update func(Event) (any, error), now time.Time, composite CompositeWindowSpec) (NamedWindowDelta, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.compositeChildren) != len(composite.Windows) {
		return NamedWindowDelta{}, NewError(ErrorState, "named-window composite retention is not initialized")
	}
	delta := NamedWindowDelta{Time: now, External: true}
	previous := append([]storedEvent(nil), state.entries...)
	replacements := make(map[int]Event)
	for index, entry := range previous {
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
		removeFromNamedWindowCompositeChildrenLocked(state, []Event{entry.event})
		stored := storedEvent{event: updated, receivedAt: entry.receivedAt, expiresAt: entry.expiresAt}
		for childIndex, child := range state.compositeChildren {
			if err := insertNamedWindowCompositeChildLocked(child, composite.Windows[childIndex], stored, now); err != nil {
				return NamedWindowDelta{}, err
			}
		}
		delta.Old = append(delta.Old, entry.event)
		replacements[index] = updated
	}
	retains := func(event Event) bool {
		return namedWindowCompositeRetains(composite.Mode, state.compositeChildren, event)
	}
	kept := make([]storedEvent, 0, len(previous))
	for index, stored := range previous {
		if updated, replaced := replacements[index]; replaced {
			if retains(updated) {
				kept = append(kept, storedEvent{event: updated, receivedAt: stored.receivedAt, expiresAt: stored.expiresAt})
				delta.New = append(delta.New, updated)
			}
			continue
		}
		if retains(stored.event) {
			kept = append(kept, stored)
			continue
		}
		delta.Old = append(delta.Old, stored.event)
	}
	state.entries = kept
	rebuildNamedWindowIndexesLocked(state)
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
		case KeepAllWindowSpec, LengthWindowSpec, LengthBatchWindowSpec, FirstLengthWindowSpec, LastEventWindowSpec, FirstEventWindowSpec, TimeWindowSpec, FirstTimeWindowSpec, TimeBatchWindowSpec, TimeLengthBatchWindowSpec, TimeAccumWindowSpec, ExternallyTimedWindowSpec, TimeOrderWindowSpec, TimeToLiveWindowSpec, TimeToLiveAtWindowSpec, UniqueWindowSpec, SortedWindowSpec, GroupWindowSpec, ExpressionWindowSpec, ExpressionBatchWindowSpec:
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

	delta := NamedWindowDelta{Time: now, External: true}
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
		case FirstEventWindowSpec:
			if len(entries) == 0 {
				entries = append(entries, storedEvent{event: preparedInsert, receivedAt: now})
				delta.New = append(delta.New, preparedInsert)
			}
		case FirstTimeWindowSpec:
			// A firsttime window admits events only while the window is empty
			// or the current time is before the oldest retained event plus the
			// time period; later events are dropped silently (Esper firsttime).
			if len(entries) == 0 || now.Before(entries[0].receivedAt.Add(retention.Duration)) {
				entries = append(entries, storedEvent{event: preparedInsert, receivedAt: now})
				delta.New = append(delta.New, preparedInsert)
			}
		case TimeBatchWindowSpec:
			// A time_batch window accumulates the current batch silently; the
			// completed batch is delivered by expire at the batch boundary.
			if state.timeBatchBoundary.IsZero() {
				state.timeBatchBoundary = now.Add(retention.Duration)
			}
			entries = append(entries, storedEvent{event: preparedInsert, receivedAt: now})
		case LengthBatchWindowSpec:
			// A length_batch window accumulates the current batch silently and
			// completes it as one new-data delivery at the size boundary; the
			// previously completed batch leaves as old data in the same delta.
			entries = append(entries, storedEvent{event: preparedInsert, receivedAt: now})
			if len(entries) >= retention.Size {
				delta.Old = append(delta.Old, state.batchLast...)
				state.batchLast = make([]Event, 0, len(entries))
				for _, entry := range entries {
					state.batchLast = append(state.batchLast, entry.event)
				}
				delta.New = append(delta.New, state.batchLast...)
				entries = nil
			}
		case FirstLengthWindowSpec:
			// A firstlength window admits only the first size events; later
			// inserts are dropped silently until deletes free a slot.
			if len(entries) < retention.Size {
				entries = append(entries, storedEvent{event: preparedInsert, receivedAt: now})
				delta.New = append(delta.New, preparedInsert)
			}
		case TimeLengthBatchWindowSpec:
			if state.timeBatchBoundary.IsZero() {
				state.timeBatchBoundary = now.Add(retention.Duration)
			}
			entries = append(entries, storedEvent{event: preparedInsert, receivedAt: now})
			if len(entries) >= retention.Size {
				delta.Old = append(delta.Old, state.batchLast...)
				state.batchLast = make([]Event, 0, len(entries))
				for _, entry := range entries {
					state.batchLast = append(state.batchLast, entry.event)
				}
				delta.New = append(delta.New, state.batchLast...)
				entries = nil
			}
		case ExternallyTimedWindowSpec:
			timestamp, err := eventTimestamp(retention.Timestamp, preparedInsert, now, nil)
			if err != nil {
				return NamedWindowDelta{}, err
			}
			if retention.Batch {
				if state.extBatchBoundary.IsZero() {
					state.extBatchBoundary = time.Unix(0, 0).UTC().Add(retention.Duration)
				}
				if !timestamp.Before(state.extBatchBoundary) {
					delta.Old = append(delta.Old, state.batchLast...)
					state.batchLast = make([]Event, 0, len(entries))
					for _, entry := range entries {
						state.batchLast = append(state.batchLast, entry.event)
					}
					delta.New = append(delta.New, state.batchLast...)
					entries = nil
					for !timestamp.Before(state.extBatchBoundary) {
						state.extBatchBoundary = state.extBatchBoundary.Add(retention.Duration)
					}
				}
				entries = append(entries, storedEvent{event: preparedInsert, receivedAt: now})
				break
			}
			entry := storedEvent{event: preparedInsert, receivedAt: now, expiresAt: timestamp.Add(retention.Duration)}
			if timestamp.After(state.maxExtTimestamp) {
				state.maxExtTimestamp = timestamp
			}
			entries = append(entries, entry)
			delta.New = append(delta.New, preparedInsert)
			kept := entries[:0]
			for _, retained := range entries {
				if !retained.expiresAt.After(state.maxExtTimestamp) {
					delta.Old = append(delta.Old, retained.event)
				} else {
					kept = append(kept, retained)
				}
			}
			entries = kept
		case TimeOrderWindowSpec:
			timestamp, err := eventTimestamp(retention.Timestamp, preparedInsert, now, nil)
			if err != nil {
				return NamedWindowDelta{}, err
			}
			entry := storedEvent{event: preparedInsert, receivedAt: now, expiresAt: timeOrderExpiry(retention, timestamp)}
			if !entry.expiresAt.After(now) {
				delta.New = append(delta.New, preparedInsert)
				delta.Old = append(delta.Old, preparedInsert)
				break
			}
			position := len(entries)
			for index, retained := range entries {
				if retained.expiresAt.After(entry.expiresAt) {
					position = index
					break
				}
			}
			entries = append(entries, storedEvent{})
			copy(entries[position+1:], entries[position:])
			entries[position] = entry
			delta.New = append(delta.New, preparedInsert)
		case GroupWindowSpec:
			switch inner := retention.Inner.(type) {
			case LengthWindowSpec:
				// #groupwin(key)#length(size) retains the newest size events
				// per group; the oldest events of an over-full group leave
				// as old data in the same delta as the triggering insert.
				entries = append(entries, storedEvent{event: preparedInsert, receivedAt: now})
				delta.New = append(delta.New, preparedInsert)
				kept, expelled := expelGroupedLengthOverflow(entries, retention.effectiveKeys(), inner.Size, now, nil)
				entries = kept
				delta.Old = append(delta.Old, expelled...)
			case TimeBatchWindowSpec:
				// #groupwin(key)#time_batch(duration) accumulates silently
				// against the anchored boundary schedule; the rollover
				// delivers each completed batch with events grouped by key
				// (see expireNamedWindowGroupedBatchRollover).
				if state.timeBatchBoundary.IsZero() {
					state.timeBatchBoundary = now.Add(inner.Duration)
				}
				entries = append(entries, storedEvent{event: preparedInsert, receivedAt: now})
			default:
				return NamedWindowDelta{}, NewError(ErrorInvalidRule, fmt.Sprintf("unsupported named-window grouped retention %T", retention.Inner))
			}
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
	return w.insertWithVariables(context.Background(), now, underlying, nil)
}

// newInsertEvent builds the event offered to the window for one insert. The
// event carries the window name as its type identity so update-istream
// statements targeting the window and window subscribers observe the same
// representation as Esper's named-window insert path.
func (w *NamedWindow) newInsertEvent(now time.Time, underlying any) (Event, error) {
	event, err := newEvent(w.state.def.schema, underlying, now)
	if err != nil {
		return Event{}, err
	}
	event.typeName = w.state.def.name
	event.streamType = w.state.def.name
	return event, nil
}

func (w *NamedWindow) insertWithVariables(ctx context.Context, now time.Time, underlying any, variables map[string]Value) (NamedWindowDelta, error) {
	if w == nil || w.state == nil {
		return NamedWindowDelta{}, NewError(ErrorState, "nil named window")
	}
	event, err := w.newInsertEvent(now, underlying)
	if err != nil {
		return NamedWindowDelta{}, err
	}
	if w.engine != nil {
		// Esper's update istream on a named window attaches an update
		// strategy to the window insert path: every offered event is
		// preprocessed copy-on-write (or removed by a matching drop)
		// before the window contents, subscribers and on-triggers observe
		// it. The original underlying stays untouched for the feeding
		// statement's own listeners.
		updated, dropped, updateErr := w.engine.applyNamedWindowUpdatesLocked(ctx, w, event, now, variables)
		if updateErr != nil {
			return NamedWindowDelta{}, updateErr
		}
		if dropped {
			return NamedWindowDelta{Time: now}, nil
		}
		event = updated
	}
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
	delta := NamedWindowDelta{New: []Event{event}, Time: now, External: true}
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
	case FirstEventWindowSpec:
		// A first-event window drops every insert once occupied; the dropped
		// event produces no listener callback at all (Esper firstevent).
		if len(state.entries) > 0 {
			return NamedWindowDelta{Time: now}, nil
		}
		state.entries = append(state.entries, entry)
	case FirstTimeWindowSpec:
		// A firsttime window admits events only while the window is empty or
		// the current time is before the oldest retained event plus the time
		// period; later events are dropped silently (Esper firsttime).
		if len(state.entries) > 0 && !now.Before(state.entries[0].receivedAt.Add(retention.Duration)) {
			return NamedWindowDelta{Time: now}, nil
		}
		state.entries = append(state.entries, entry)
	case TimeBatchWindowSpec:
		// A time_batch window accumulates the current batch without a
		// listener callback; expire delivers the completed batch as new data
		// at the batch boundary and as old data at the following boundary.
		if state.timeBatchBoundary.IsZero() {
			state.timeBatchBoundary = now.Add(retention.Duration)
		}
		state.entries = append(state.entries, entry)
		delta.New = nil
	case LengthBatchWindowSpec:
		// A length_batch window accumulates the current batch without a
		// listener callback and completes it as one new-data delivery at the
		// size boundary; the previously completed batch leaves as old data in
		// the same delta (Esper length_batch second insert stream).
		state.entries = append(state.entries, entry)
		delta.New = nil
		if len(state.entries) >= retention.Size {
			delta.Old = append(delta.Old, state.batchLast...)
			state.batchLast = make([]Event, 0, len(state.entries))
			for _, retained := range state.entries {
				state.batchLast = append(state.batchLast, retained.event)
			}
			delta.New = append(delta.New, state.batchLast...)
			state.entries = nil
		}
	case FirstLengthWindowSpec:
		// A firstlength window admits only the first size events; later
		// inserts are dropped silently until deletes free a slot.
		if len(state.entries) >= retention.Size {
			return NamedWindowDelta{Time: now}, nil
		}
		state.entries = append(state.entries, entry)
	case TimeAccumWindowSpec:
		// A time_accum window delivers inserts immediately; all retained
		// events expire together at the newest retained event plus the
		// period (see expireNamedWindowState).
		state.entries = append(state.entries, entry)
	case TimeLengthBatchWindowSpec:
		// time_length_batch accumulates silently like length_batch and
		// completes the batch either at the size boundary here or at the
		// time boundary in expireNamedWindowBatchRollover.
		if state.timeBatchBoundary.IsZero() {
			state.timeBatchBoundary = now.Add(retention.Duration)
		}
		state.entries = append(state.entries, entry)
		delta.New = nil
		if len(state.entries) >= retention.Size {
			delta.Old = append(delta.Old, state.batchLast...)
			state.batchLast = make([]Event, 0, len(state.entries))
			for _, retained := range state.entries {
				state.batchLast = append(state.batchLast, retained.event)
			}
			delta.New = append(delta.New, state.batchLast...)
			state.entries = nil
		}
	case ExternallyTimedWindowSpec:
		timestamp, err := eventTimestamp(retention.Timestamp, event, now, nil)
		if err != nil {
			return NamedWindowDelta{}, err
		}
		if retention.Batch {
			// ext_timed_batch accumulates silently; an arriving event whose
			// timestamp crosses the next boundary (anchored at the epoch
			// reference) flushes the accumulated batch as new data and the
			// previously completed batch as old data, then starts the next
			// batch with the arriving event.
			delta.New = nil
			if state.extBatchBoundary.IsZero() {
				state.extBatchBoundary = time.Unix(0, 0).UTC().Add(retention.Duration)
			}
			if !timestamp.Before(state.extBatchBoundary) {
				delta.Old = append(delta.Old, state.batchLast...)
				state.batchLast = make([]Event, 0, len(state.entries))
				for _, retained := range state.entries {
					state.batchLast = append(state.batchLast, retained.event)
				}
				delta.New = append(delta.New, state.batchLast...)
				state.entries = nil
				for !timestamp.Before(state.extBatchBoundary) {
					state.extBatchBoundary = state.extBatchBoundary.Add(retention.Duration)
				}
			}
			state.entries = append(state.entries, entry)
			break
		}
		// ext_timed slides with arriving event timestamps: events expire
		// when the newest timestamp seen reaches their timestamp plus the
		// period, delivered as old data in the same delta as the insert.
		entry.expiresAt = timestamp.Add(retention.Duration)
		if timestamp.After(state.maxExtTimestamp) {
			state.maxExtTimestamp = timestamp
		}
		state.entries = append(state.entries, entry)
		kept := state.entries[:0]
		for _, retained := range state.entries {
			if !retained.expiresAt.After(state.maxExtTimestamp) {
				delta.Old = append(delta.Old, retained.event)
			} else {
				kept = append(kept, retained)
			}
		}
		state.entries = kept
	case TimeOrderWindowSpec:
		// time_order keeps events sorted by their external timestamp and
		// expires them under the engine clock (see expireNamedWindowState).
		timestamp, err := eventTimestamp(retention.Timestamp, event, now, nil)
		if err != nil {
			return NamedWindowDelta{}, err
		}
		entry.expiresAt = timeOrderExpiry(retention, timestamp)
		if !entry.expiresAt.After(now) {
			// An event whose external timestamp is already expired under the
			// current engine time passes straight through: delivered as new
			// and removed again in the same delta (Esper time_order).
			delta.Old = append(delta.Old, event)
			break
		}
		position := len(state.entries)
		for index, retained := range state.entries {
			if retained.expiresAt.After(entry.expiresAt) {
				position = index
				break
			}
		}
		state.entries = append(state.entries, storedEvent{})
		copy(state.entries[position+1:], state.entries[position:])
		state.entries[position] = entry
	case GroupWindowSpec:
		switch inner := retention.Inner.(type) {
		case LengthWindowSpec:
			// #groupwin(key)#length(size) retains the newest size events per
			// group; the oldest events of an over-full group leave as old
			// data in the same delta as the triggering insert.
			state.entries = append(state.entries, entry)
			kept, expelled := expelGroupedLengthOverflow(state.entries, retention.effectiveKeys(), inner.Size, now, variables)
			state.entries = kept
			delta.Old = append(delta.Old, expelled...)
		case TimeBatchWindowSpec:
			// #groupwin(key)#time_batch(duration) accumulates silently
			// against the anchored boundary schedule; the rollover delivers
			// each completed batch with events grouped by key (see
			// expireNamedWindowGroupedBatchRollover).
			if state.timeBatchBoundary.IsZero() {
				state.timeBatchBoundary = now.Add(inner.Duration)
			}
			state.entries = append(state.entries, entry)
			delta.New = nil
		default:
			return NamedWindowDelta{}, NewError(ErrorInvalidRule, fmt.Sprintf("unsupported named-window grouped retention %T", retention.Inner))
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
	case CompositeWindowSpec:
		// Composite view stacks (#length(2)#unique(intPrimitive)): each child
		// view retains independently; the window contents are the
		// intersection (every child) or union (any child) of the child
		// contents. An event expelled by all children leaves the window as
		// old data in the triggering insert delta (Esper IntersectionView /
		// UnionView).
		if len(state.compositeChildren) != len(retention.Windows) {
			return NamedWindowDelta{}, NewError(ErrorState, "named-window composite retention is not initialized")
		}
		previous := append([]storedEvent(nil), state.entries...)
		for index, child := range state.compositeChildren {
			if err := insertNamedWindowCompositeChildLocked(child, retention.Windows[index], entry, now); err != nil {
				return NamedWindowDelta{}, err
			}
		}
		retains := func(candidate Event) bool {
			return namedWindowCompositeRetains(retention.Mode, state.compositeChildren, candidate)
		}
		admitted := retains(event)
		kept := make([]storedEvent, 0, len(previous)+1)
		for _, stored := range previous {
			if retains(stored.event) {
				kept = append(kept, stored)
				continue
			}
			delta.Old = append(delta.Old, stored.event)
		}
		if admitted {
			kept = append(kept, entry)
		} else {
			delta.New = nil
		}
		state.entries = kept
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
	case ExpressionWindowSpec:
		// #expr(keep) named-window retention applies the keep predicate
		// at insert time exactly like the stream expression window: the
		// newest row is added first and the predicate pops the oldest
		// rows until it holds. Expelled rows leave as old data.
		state.entries = append(state.entries, entry)
		kept, expelled := applyExpressionNamedWindowRetentionLocked(state, retention, now)
		state.entries = kept
		delta.Old = append(delta.Old, expelled...)
	case ExpressionBatchWindowSpec:
		// #expr_batch(trigger) named-window retention accumulates the
		// current batch silently; when the trigger predicate holds over
		// the accumulated rows the batch is delivered as new data and
		// the previous delivered batch leaves as old data. The trigger
		// event is included by default (Java default true).
		candidate := append(append([]storedEvent(nil), state.entries...), entry)
		if !windowPredicate(retention.Trigger, candidate, now, nil, 0) {
			state.entries = candidate
			return NamedWindowDelta{Time: now}, nil
		}
		if !retention.IncludeTrigger {
			// The trigger event starts the next batch; the preceding
			// pending events are delivered now and the trigger remains
			// pending. Java explicit false form.
			pending := state.entries
			state.entries = nil
			delta.Old = append(delta.Old, state.batchLast...)
			delta.New = nil
			state.batchLast = make([]Event, 0, len(pending))
			for _, retained := range pending {
				state.batchLast = append(state.batchLast, retained.event)
			}
			if len(pending) == 0 {
				state.entries = []storedEvent{entry}
				return delta, nil
			}
			for _, retained := range pending {
				delta.New = append(delta.New, retained.event)
			}
			state.entries = []storedEvent{entry}
			return delta, nil
		}
		state.entries = candidate
		delta.New = nil
		delta.Old = append(delta.Old, state.batchLast...)
		state.batchLast = make([]Event, 0, len(state.entries))
		for _, retained := range state.entries {
			delta.New = append(delta.New, retained.event)
			state.batchLast = append(state.batchLast, retained.event)
		}
		state.entries = nil
	default:
		return NamedWindowDelta{}, NewError(ErrorInvalidRule, fmt.Sprintf("unsupported named-window retention %T", retention))
	}
	rebuildNamedWindowIndexesLocked(state)
	return delta, nil
}

// insertNamedWindowCompositeChildLocked applies one child view's retention
// rule to a composite child runtime. Only the event-driven data window views
// Esper commonly stacks on named windows are supported; validation rejects
// every other child at definition time.
func insertNamedWindowCompositeChildLocked(child *namedWindowRuntime, spec WindowSpec, entry storedEvent, now time.Time) error {
	switch childSpec := spec.(type) {
	case KeepAllWindowSpec:
		child.entries = append(child.entries, entry)
	case TimeWindowSpec:
		child.entries = append(child.entries, entry)
	case FirstLengthWindowSpec:
		if len(child.entries) >= childSpec.Size {
			return nil
		}
		child.entries = append(child.entries, entry)
	case LengthWindowSpec:
		child.entries = append(child.entries, entry)
		for len(child.entries) > childSpec.Size {
			child.entries = child.entries[1:]
		}
	case UniqueWindowSpec:
		if child.keyed == nil {
			child.keyed = make(map[string]storedEvent)
		}
		key := uniqueWindowKey(childSpec, entry.event, now, nil)
		if previous, exists := child.keyed[key]; exists {
			if childSpec.First {
				return nil
			}
			for index, candidate := range child.entries {
				if sameEvent(candidate.event, previous.event) {
					child.entries[index] = entry
					child.keyed[key] = entry
					return nil
				}
			}
			child.keyed[key] = entry
			child.entries = append(child.entries, entry)
			return nil
		}
		child.keyed[key] = entry
		child.keyOrder = append(child.keyOrder, key)
		child.entries = append(child.entries, entry)
	default:
		return NewError(ErrorInvalidRule, fmt.Sprintf("unsupported named-window composite child %T", spec))
	}
	return nil
}

// namedWindowCompositeRetains reports whether the event is retained by the
// composite window: every child for intersection, any child for union.
func namedWindowCompositeRetains(mode CompositeWindowMode, children []*namedWindowRuntime, event Event) bool {
	retained := 0
	for _, child := range children {
		if containsEvent(child.entries, event) {
			retained++
		}
	}
	if mode == UnionWindowMode {
		return retained > 0
	}
	return retained == len(children)
}

// namedWindowCompositeContainsAll reports whether every composite child
// currently retains the event. It is kept for callers that explicitly need
// the intersection test.
func namedWindowCompositeContainsAll(children []*namedWindowRuntime, event Event) bool {
	return namedWindowCompositeRetains(IntersectWindowMode, children, event)
}

// removeFromNamedWindowCompositeChildrenLocked drops events from every
// composite child so a window-level delete stays consistent with the child
// view states (Esper forwards intersection removes to all children).
func removeFromNamedWindowCompositeChildrenLocked(state *namedWindowRuntime, removed []Event) {
	if len(removed) == 0 || len(state.compositeChildren) == 0 {
		return
	}
	removedSet := make([]storedEvent, len(removed))
	for index, event := range removed {
		removedSet[index] = storedEvent{event: event}
	}
	for _, child := range state.compositeChildren {
		kept := child.entries[:0]
		for _, entry := range child.entries {
			if !containsEvent(removedSet, entry.event) {
				kept = append(kept, entry)
			}
		}
		child.entries = kept
		if child.keyed != nil {
			for key, stored := range child.keyed {
				if containsEvent(removedSet, stored.event) {
					delete(child.keyed, key)
				}
			}
			keyOrder := child.keyOrder[:0]
			for _, key := range child.keyOrder {
				if _, exists := child.keyed[key]; exists {
					keyOrder = append(keyOrder, key)
				}
			}
			child.keyOrder = keyOrder
		}
	}
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
	case TimeAccumWindowSpec:
		// All retained events share one expiry time anchored at the newest
		// retained event; deleting the newest re-anchors to the next newest
		// (Esper time_accum). The whole window leaves together as old data.
		if len(state.entries) == 0 {
			return NamedWindowDelta{}
		}
		if state.entries[len(state.entries)-1].receivedAt.Add(retention.Duration).After(at) {
			return NamedWindowDelta{}
		}
		delta := NamedWindowDelta{Time: at}
		for _, entry := range state.entries {
			delta.Old = append(delta.Old, entry.event)
		}
		state.entries = nil
		rebuildNamedWindowIndexesLocked(state)
		return delta
	case TimeBatchWindowSpec:
		// Batch rollover: the previously completed batch leaves as old data,
		// the just-completed batch is delivered as new data and the window
		// starts accumulating the next batch. The boundary schedule anchors
		// at the first insert and slides by the period even when batches are
		// empty (Esper time_batch named window semantics).
		if state.timeBatchBoundary.IsZero() {
			return NamedWindowDelta{}
		}
		return expireNamedWindowBatchRollover(state, at, retention.Duration, false)
	case TimeLengthBatchWindowSpec:
		// time_length_batch rolls over on the same anchored boundary
		// schedule as time_batch; the size trigger lives in the insert path.
		// Unlike time_batch an all-empty rollover disarms the schedule and
		// the next insert re-arms it.
		if state.timeBatchBoundary.IsZero() {
			return NamedWindowDelta{}
		}
		return expireNamedWindowBatchRollover(state, at, retention.Duration, true)
	case GroupWindowSpec:
		// #groupwin(key)#time_batch(duration) rolls over on the same
		// anchored boundary schedule as time_batch; the completed batch is
		// delivered with events grouped by key.
		inner, ok := retention.Inner.(TimeBatchWindowSpec)
		if !ok || state.timeBatchBoundary.IsZero() {
			return NamedWindowDelta{}
		}
		return expireNamedWindowGroupedBatchRollover(state, at, inner.Duration, retention.effectiveKeys())
	case TimeOrderWindowSpec:
		// time_order keeps events sorted by their external timestamp and
		// expires them under the engine clock at timestamp plus the period.
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
	case CompositeWindowSpec:
		// Composite view stacks expire each child independently; an event
		// leaves the window when it is no longer retained by the composite
		// (every child for intersection, any child for union).
		if len(state.compositeChildren) != len(retention.Windows) {
			return NamedWindowDelta{}
		}
		for _, child := range state.compositeChildren {
			expireNamedWindowState(child, at)
		}
		delta := NamedWindowDelta{Time: at}
		kept := state.entries[:0]
		for _, entry := range state.entries {
			if namedWindowCompositeRetains(retention.Mode, state.compositeChildren, entry.event) {
				kept = append(kept, entry)
				continue
			}
			delta.Old = append(delta.Old, entry.event)
		}
		state.entries = kept
		rebuildNamedWindowIndexesLocked(state)
		return delta
	case TimeWindowSpec:
		kept := state.entries[:0]
		delta := NamedWindowDelta{Time: at}
		for _, entry := range state.entries {
			if !timeWindowEventDeadline(retention, entry.event, entry.receivedAt, at, nil, nil).After(at) {
				delta.Old = append(delta.Old, entry.event)
			} else {
				kept = append(kept, entry)
			}
		}
		state.entries = kept
		rebuildNamedWindowIndexesLocked(state)
		return delta
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

// expireNamedWindowBatchRollover runs the time_batch/time_length_batch
// boundary schedule: at every crossed boundary the previously completed
// batch leaves as old data and the just-completed batch is delivered as new
// data. The schedule anchors at the first insert and slides by the period
// even while batches are empty (Esper time_batch named window semantics).
func expireNamedWindowBatchRollover(state *namedWindowRuntime, at time.Time, duration time.Duration, disarmWhenEmpty bool) NamedWindowDelta {
	delta := NamedWindowDelta{Time: at}
	changed := false
	for !state.timeBatchBoundary.After(at) {
		if disarmWhenEmpty && len(state.batchLast) == 0 && len(state.entries) == 0 {
			// An all-empty rollover disarms the time_length_batch schedule;
			// the next insert re-arms it (Esper re-anchoring semantics).
			state.timeBatchBoundary = time.Time{}
			break
		}
		if len(state.batchLast) > 0 {
			delta.Old = append(delta.Old, state.batchLast...)
		}
		if len(state.entries) > 0 {
			state.batchLast = make([]Event, 0, len(state.entries))
			for _, entry := range state.entries {
				state.batchLast = append(state.batchLast, entry.event)
			}
			delta.New = append(delta.New, state.batchLast...)
			state.entries = nil
			changed = true
		} else {
			state.batchLast = nil
		}
		state.timeBatchBoundary = state.timeBatchBoundary.Add(duration)
	}
	if changed {
		rebuildNamedWindowIndexesLocked(state)
	}
	return delta
}

// expireNamedWindowGroupedBatchRollover runs the #groupwin(key)#time_batch
// boundary schedule: identical to time_batch except the just-completed
// batch is delivered with events grouped by the group key in first-seen
// group order (Esper concatenates the per-group completed batches into one
// new-data delivery).
func expireNamedWindowGroupedBatchRollover(state *namedWindowRuntime, at time.Time, duration time.Duration, keys []Expr) NamedWindowDelta {
	delta := NamedWindowDelta{Time: at}
	changed := false
	for !state.timeBatchBoundary.After(at) {
		if len(state.batchLast) > 0 {
			delta.Old = append(delta.Old, state.batchLast...)
		}
		if len(state.entries) > 0 {
			state.batchLast = make([]Event, 0, len(state.entries))
			groupOrder := make([]string, 0)
			grouped := make(map[string][]Event)
			for _, entry := range state.entries {
				groupKey := groupWindowKeys(keys, entry.event, at, nil)
				if _, seen := grouped[groupKey]; !seen {
					groupOrder = append(groupOrder, groupKey)
				}
				grouped[groupKey] = append(grouped[groupKey], entry.event)
			}
			for _, groupKey := range groupOrder {
				state.batchLast = append(state.batchLast, grouped[groupKey]...)
			}
			delta.New = append(delta.New, state.batchLast...)
			state.entries = nil
			changed = true
		} else {
			state.batchLast = nil
		}
		state.timeBatchBoundary = state.timeBatchBoundary.Add(duration)
	}
	if changed {
		rebuildNamedWindowIndexesLocked(state)
	}
	return delta
}

// expelGroupedLengthOverflow removes the oldest events of every group whose
// retained count exceeds the per-group size, returning the kept entries and
// the expelled events oldest-first (Esper #groupwin(key)#length(size)).
func expelGroupedLengthOverflow(entries []storedEvent, keys []Expr, size int, now time.Time, variables map[string]Value) ([]storedEvent, []Event) {
	counts := make(map[string]int)
	for _, entry := range entries {
		counts[groupWindowKeys(keys, entry.event, now, variables)]++
	}
	excess := make(map[string]int)
	for groupKey, count := range counts {
		if count > size {
			excess[groupKey] = count - size
		}
	}
	if len(excess) == 0 {
		return entries, nil
	}
	kept := entries[:0]
	var expelled []Event
	for _, entry := range entries {
		groupKey := groupWindowKeys(keys, entry.event, now, variables)
		if excess[groupKey] > 0 {
			excess[groupKey]--
			expelled = append(expelled, entry.event)
			continue
		}
		kept = append(kept, entry)
	}
	return kept, expelled
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
