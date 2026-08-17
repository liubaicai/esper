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

const subqueryEngineVariable = "\x00esper.engine"
const subqueryRuntimeVariable = "\x00esper.subqueries"
const subqueryContextNameVariable = "\x00esper.context.name"
const subqueryContextPartitionVariable = "\x00esper.context.partition"

type subqueryEngineRef struct {
	engine *Engine
	locked bool
}

type subqueryRuntimeRef struct {
	registry *subqueryRuntimeRegistry
}

type subqueryRuntimeRegistry struct {
	mu     sync.Mutex
	env    *Environment
	engine *Engine
	states map[*subqueryDefinition]*subqueryRuntimeState
}

type subqueryRuntimeState struct {
	definition *subqueryDefinition
	runtime    *statementRuntime
	events     []Event
}

func newSubqueryRuntimeRegistry(env *Environment, engine *Engine, query Query) *subqueryRuntimeRegistry {
	definitions := querySubqueryDefinitions(query)
	if len(definitions) == 0 {
		return nil
	}
	registry := &subqueryRuntimeRegistry{
		env:    env,
		engine: engine,
		states: make(map[*subqueryDefinition]*subqueryRuntimeState),
	}
	for _, definition := range definitions {
		base, err := subqueryRootSource(definition.source)
		if err != nil || base.kind != streamSource {
			continue
		}
		query := Query{env: env, input: definition.source}
		runtime := newStatementRuntime(query)
		runtime.engine = engine
		registry.states[definition] = &subqueryRuntimeState{definition: definition, runtime: &runtime}
	}
	if len(registry.states) == 0 {
		return nil
	}
	return registry
}

// subqueryRootSource unwraps the fluent operators that transform the rows
// visible to a subquery. sourceNode intentionally stops at a contained node
// because that node owns the child schema; subqueries also need the logical
// parent source so they can snapshot a named window or register an event
// stream runtime before applying the contained expansion.
func subqueryRootSource(node *streamNode) (*streamNode, error) {
	for current := node; current != nil; current = current.input {
		switch current.kind {
		case streamFilter, streamWindow, streamContained:
			continue
		default:
			return current, nil
		}
	}
	return nil, fmt.Errorf("esper: subquery has no root source")
}

func querySubqueryDefinitions(query Query) []*subqueryDefinition {
	seen := make(map[*subqueryDefinition]struct{})
	definitions := make([]*subqueryDefinition, 0)
	var visitNode func(*exprNode)
	visitNode = func(node *exprNode) {
		if node == nil {
			return
		}
		if node.subquery != nil {
			if _, exists := seen[node.subquery]; !exists {
				seen[node.subquery] = struct{}{}
				definitions = append(definitions, node.subquery)
			}
			if node.subquery.predicate != nil {
				visitNode(node.subquery.predicate.node())
			}
			if node.subquery.projection != nil {
				visitNode(node.subquery.projection.node())
			}
			for _, selection := range node.subquery.columns {
				if selection.Expr != nil {
					visitNode(selection.Expr.node())
				}
			}
			if node.subquery.groupBy != nil {
				visitNode(node.subquery.groupBy.node())
			}
			if node.subquery.having != nil {
				visitNode(node.subquery.having.node())
			}
			for _, order := range node.subquery.orderBy {
				if order.Expression != nil {
					visitNode(order.Expression.node())
				}
			}
		}
		for _, child := range node.children {
			visitNode(child)
		}
	}
	_ = visitQueryExpressions(query.env, query, func(expression Expr) error {
		if expression != nil {
			visitNode(expression.node())
		}
		return nil
	})
	return definitions
}

func (r *subqueryRuntimeRegistry) attachVariables(variables map[string]Value) map[string]Value {
	if r == nil {
		return variables
	}
	result := cloneValues(variables)
	if result == nil {
		result = make(map[string]Value)
	}
	result[subqueryRuntimeVariable] = Present(&subqueryRuntimeRef{registry: r})
	return result
}

func (r *subqueryRuntimeRegistry) accept(event Event, now time.Time, variables map[string]Value) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, state := range r.states {
		if state == nil || state.definition == nil || state.runtime == nil || !sourceNodeAcceptsEvent(r.env, state.definition.source, event) {
			continue
		}
		state.runtime.variables = variablesWithEngine(variables, r.engine)
		delta, err := state.runtime.insert(state.definition.source, event, now)
		if err != nil {
			return err
		}
		if subquerySourceContainsWindow(state.definition.source) {
			state.events = append([]Event(nil), delta.history...)
		} else {
			state.events = append(state.events, delta.newEvents...)
		}
	}
	return nil
}

// acceptsEvent lets Context statements avoid constructing a partition-local
// variable map when none of their event-stream subqueries can consume the
// incoming event. This matters for sparse contexts with many lazy partitions:
// an unrelated event must not turn a linear dispatch into an O(partitions)
// clone loop before accept can reject it.
func (r *subqueryRuntimeRegistry) acceptsEvent(event Event) bool {
	if r == nil {
		return false
	}
	for _, state := range r.states {
		if state == nil || state.definition == nil || state.runtime == nil {
			continue
		}
		if sourceNodeAcceptsEvent(r.env, state.definition.source, event) {
			return true
		}
	}
	return false
}

func (r *subqueryRuntimeRegistry) expire(now time.Time) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, state := range r.states {
		if state == nil || state.runtime == nil || !subquerySourceContainsWindow(state.definition.source) {
			continue
		}
		delta := state.runtime.expire(now)
		state.events = append([]Event(nil), delta.history...)
	}
}

func (r *subqueryRuntimeRegistry) snapshot(definition *subqueryDefinition) ([]Event, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	state, ok := r.states[definition]
	if !ok || state == nil {
		return nil, false
	}
	return append([]Event(nil), state.events...), true
}

func subqueryRuntimeFromVariables(variables map[string]Value) *subqueryRuntimeRegistry {
	if variables == nil {
		return nil
	}
	value, ok := variables[subqueryRuntimeVariable]
	if !ok || !value.IsPresent() {
		return nil
	}
	ref, ok := value.Any().(*subqueryRuntimeRef)
	if !ok || ref == nil {
		return nil
	}
	return ref.registry
}

// subqueryIndexSelection returns a physical access path only when the
// subquery has a declared lookup index that can satisfy a complete equality
// or IN key, or an equality-prefix-plus-range B-tree probe. The normal FAF
// planner exposes index selections for top-level sources; subqueries are
// evaluated as expressions, so their access path is derived here from the
// same analyzable predicate vocabulary.
//
// A retention summary such as a Named Window's unique retention is not a
// declared lookup index.  It remains a logical planner candidate for other
// purposes, but this physical subquery path deliberately falls back unless a
// real index was declared.
func subqueryIndexSelection(e *Environment, base *streamNode, definition *subqueryDefinition) (IndexSelection, bool) {
	if e == nil || base == nil || definition == nil || definition.predicate == nil || definition.source != base {
		return IndexSelection{}, false
	}
	if definition.noIndex {
		return IndexSelection{}, false
	}
	if base.kind != streamNamedWindow && base.kind != streamTable {
		return IndexSelection{}, false
	}
	predicates := indexPredicatesFromExpression(definition.predicate)
	if len(predicates) == 0 {
		return IndexSelection{}, false
	}
	var hint *indexHint
	if definition.indexName != "" {
		hint = &indexHint{name: definition.indexName}
	} else if base.kind == streamNamedWindow {
		// A user-declared index takes precedence over the logical retention
		// summary. This mirrors Esper's preference for a concrete backing index
		// in the subquery plan while still allowing automatic sharing when no
		// declared index can satisfy the predicate.
		if declared, ok := subqueryNamedWindowDeclaredIndexSelection(e, base, predicates); ok {
			return declared, true
		}
	}
	selection, err := chooseIndexSelection(e, base, 0, predicates, hint)
	if err != nil {
		return IndexSelection{}, false
	}
	if base.kind == streamNamedWindow {
		if selection.IndexName != "" && !strings.HasPrefix(selection.IndexName, "<") {
			return selection, true
		}
		if selection.IndexName == "" && definition.indexName != "" {
			return IndexSelection{}, false
		}
		// A retention-unique summary is not a declared subquery index. When
		// sharing is enabled, replace that logical candidate with the stable
		// internal equality index for this predicate. Consumer-side disabling
		// deliberately leaves the old snapshot path intact.
		window, ok := e.NamedWindowInModule(base.moduleName, base.sourceName)
		if !ok || !window.SubqueryIndexSharing() || definition.disableIndexSharing || definition.indexName != "" {
			return IndexSelection{}, false
		}
		columns, kind := subquerySharedIndexColumns(predicates)
		if len(columns) == 0 {
			return IndexSelection{}, false
		}
		name := subquerySharedIndexName(columns, kind)
		access := IndexAccessEquality
		backing := IndexBackingHash
		matched := append([]string(nil), columns...)
		if kind == IndexBTree {
			access = IndexAccessRange
			backing = IndexBackingBTree
		}
		return IndexSelection{
			Source:         0,
			Module:         base.moduleName,
			Object:         base.sourceName,
			IndexName:      name,
			Columns:        columns,
			MatchedColumns: matched,
			Access:         access,
			Backing:        backing,
		}, true
	}
	if selection.IndexName == "" {
		return IndexSelection{}, false
	}
	if selection.Access == IndexAccessRange {
		if selection.Backing != IndexBackingBTree && selection.Backing != IndexBackingUniqueBTree {
			return IndexSelection{}, false
		}
		if len(selection.Columns) == 0 || len(selection.MatchedColumns) == 0 || len(selection.MatchedColumns) > len(selection.Columns) {
			return IndexSelection{}, false
		}
		return selection, true
	}
	if selection.Access != IndexAccessEquality && selection.Access != IndexAccessIn {
		return IndexSelection{}, false
	}
	if len(selection.Columns) == 0 || len(selection.MatchedColumns) != len(selection.Columns) {
		return IndexSelection{}, false
	}
	return selection, true
}

func subqueryNamedWindowDeclaredIndexSelection(e *Environment, base *streamNode, predicates []indexPredicate) (IndexSelection, bool) {
	if e == nil || base == nil || base.kind != streamNamedWindow {
		return IndexSelection{}, false
	}
	definition, ok := e.NamedWindowInModule(base.moduleName, base.sourceName)
	if !ok {
		return IndexSelection{}, false
	}
	bestScore := -1
	var best NamedWindowIndexDefinition
	var bestAccess IndexAccessKind
	var bestMatched []string
	for _, index := range definition.Indexes() {
		candidate := indexCandidate{name: index.Name, columns: append([]string(nil), index.Columns...), unique: index.Unique, kind: index.Kind}
		access, matched, usable := matchIndex(candidate, predicates)
		if !usable {
			continue
		}
		score := len(matched) * 10
		if access == IndexAccessRange {
			score += 2
		}
		if index.Unique {
			score++
		}
		if score > bestScore || (score == bestScore && index.Name < best.Name) {
			bestScore = score
			best = index
			bestAccess = access
			bestMatched = append([]string(nil), matched...)
		}
	}
	if bestScore < 0 {
		return IndexSelection{}, false
	}
	return IndexSelection{
		Source:         0,
		Module:         base.moduleName,
		Object:         base.sourceName,
		IndexName:      best.Name,
		Columns:        append([]string(nil), best.Columns...),
		MatchedColumns: bestMatched,
		Access:         bestAccess,
		Backing:        indexBacking(best.Kind, best.Unique),
	}, true
}

// subquerySharedIndexColumns produces a deterministic shared-index shape.
// Equality columns form the prefix (sorted for canonical identity); when a
// range predicate is available, one non-equality range column follows that
// prefix. A B-tree can then satisfy the complete correlated probe without
// making an ordering assumption about the expression's written operand order.
// IN remains on the hash path because the current range probe accepts one
// fixed value per equality-prefix component.
func subquerySharedIndexColumns(predicates []indexPredicate) ([]string, IndexKind) {
	equality := make([]string, 0, len(predicates))
	equalitySeen := make(map[string]struct{}, len(predicates))
	ranges := make([]string, 0, len(predicates))
	rangeSeen := make(map[string]struct{}, len(predicates))
	hasIn := false
	for _, predicate := range predicates {
		for _, column := range predicate.columns {
			column = strings.TrimSpace(column)
			if column == "" {
				continue
			}
			switch predicate.access {
			case IndexAccessEquality, IndexAccessIn:
				if predicate.access == IndexAccessIn {
					hasIn = true
				}
				if _, exists := equalitySeen[column]; !exists {
					equalitySeen[column] = struct{}{}
					equality = append(equality, column)
				}
			case IndexAccessRange:
				if _, exists := rangeSeen[column]; !exists {
					rangeSeen[column] = struct{}{}
					ranges = append(ranges, column)
				}
			}
		}
	}
	sort.Strings(equality)
	sort.Strings(ranges)
	if len(equality) == 0 && len(ranges) == 0 {
		return nil, IndexHash
	}
	for _, column := range ranges {
		// The current correlated range probe accepts one fixed value per
		// equality-prefix component. Keep IN + range on the hash candidate
		// path until the B-tree probe can expand one range query per IN value.
		if hasIn {
			break
		}
		if _, exact := equalitySeen[column]; exact {
			continue
		}
		return append(append([]string(nil), equality...), column), IndexBTree
	}
	return equality, IndexHash
}

func subquerySharedIndexName(columns []string, kind IndexKind) string {
	prefix := subquerySharedHashIndexPrefix
	if kind == IndexBTree {
		prefix = subquerySharedBTreeIndexPrefix
	}
	return prefix + strings.Join(columns, ",") + ">"
}

func subqueryIndexOperandValue(node *exprNode, outer EvalContext) (Value, bool) {
	if node == nil {
		return Missing(), false
	}
	if node.kind == "outer-field" {
		event := subqueryEnclosingEvent(outer)
		if !event.Schema().valid() {
			return Missing(), false
		}
		value := event.Get(node.fieldName)
		return value, value.IsPresent()
	}
	return indexOperandValue(node, EvalContext{
		Now:        outer.Now,
		Variables:  outer.Variables,
		Parameters: outer.Parameters,
	})
}

// appendSubqueryIndexProbeValues is the correlated counterpart of
// appendIndexProbeValues.  OuterField is safe here because its value is
// fixed for the current outer row; arbitrary expressions are still rejected
// so physical pruning can never evaluate a dynamic function speculatively.
func appendSubqueryIndexProbeValues(destination *[]any, node *exprNode, outer EvalContext, expandCollection bool) bool {
	if node == nil {
		return false
	}
	value, ok := subqueryIndexOperandValue(node, outer)
	if !ok || !value.IsPresent() {
		return false
	}
	if !expandCollection {
		*destination = append(*destination, value.Any())
		return true
	}
	raw := reflect.ValueOf(value.Any())
	for raw.IsValid() && raw.Kind() == reflect.Interface {
		if raw.IsNil() {
			return false
		}
		raw = raw.Elem()
	}
	if !raw.IsValid() {
		return false
	}
	switch raw.Kind() {
	case reflect.Array, reflect.Slice:
		for index := 0; index < raw.Len(); index++ {
			item := raw.Index(index)
			if item.Kind() == reflect.Interface && !item.IsNil() {
				item = item.Elem()
			}
			if !item.IsValid() || ((item.Kind() == reflect.Pointer || item.Kind() == reflect.Interface) && item.IsNil()) {
				continue
			}
			*destination = append(*destination, item.Interface())
		}
		return true
	case reflect.Map:
		for _, key := range raw.MapKeys() {
			*destination = append(*destination, key.Interface())
		}
		return true
	default:
		*destination = append(*destination, value.Any())
		return true
	}
}

func collectSubqueryIndexProbeConstraints(node *exprNode, outer EvalContext, constraints indexProbeConstraints) {
	if node == nil {
		return
	}
	if node.kind == "and" {
		for _, child := range node.children {
			collectSubqueryIndexProbeConstraints(child, outer, constraints)
		}
		return
	}
	switch node.kind {
	case "eq", "equal-of", "is":
		if len(node.children) < 2 {
			return
		}
		leftColumn := fieldColumn(node.children[0])
		rightColumn := fieldColumn(node.children[1])
		if leftColumn != "" && rightColumn == "" {
			values := make([]any, 0, 1)
			if appendSubqueryIndexProbeValues(&values, node.children[1], outer, false) {
				addIndexProbeConstraint(constraints, leftColumn, values)
			}
			return
		}
		if rightColumn != "" && leftColumn == "" {
			values := make([]any, 0, 1)
			if appendSubqueryIndexProbeValues(&values, node.children[0], outer, false) {
				addIndexProbeConstraint(constraints, rightColumn, values)
			}
		}
	case "in", "in-of":
		if len(node.children) < 2 {
			return
		}
		column := fieldColumn(node.children[0])
		if column == "" {
			return
		}
		values := make([]any, 0, len(node.children)-1)
		for _, candidate := range node.children[1:] {
			if !appendSubqueryIndexProbeValues(&values, candidate, outer, node.kind == "in-of") {
				return
			}
		}
		addIndexProbeConstraint(constraints, column, values)
	case "in-slice":
		if len(node.children) != 2 {
			return
		}
		column := fieldColumn(node.children[0])
		if column == "" {
			return
		}
		values := make([]any, 0)
		if appendSubqueryIndexProbeValues(&values, node.children[1], outer, true) {
			addIndexProbeConstraint(constraints, column, values)
		}
	}
}

func (e *Engine) subqueryIndexProbeKeys(base *streamNode, selection IndexSelection, predicate Expr, outer EvalContext) ([][]any, bool) {
	if e == nil || base == nil || predicate == nil || selection.Access != IndexAccessEquality && selection.Access != IndexAccessIn {
		return nil, false
	}
	schema, err := e.env.sourceSchema(base)
	if err != nil {
		return nil, false
	}
	constraints := make(indexProbeConstraints)
	collectSubqueryIndexProbeConstraints(predicate.node(), outer, constraints)
	keys := [][]any{{}}
	for _, column := range selection.Columns {
		values, exists := constraints[column]
		if !exists || len(values) == 0 {
			return nil, false
		}
		field, fieldExists := schema.Field(column)
		fieldType := reflect.Type(nil)
		if fieldExists {
			fieldType = field.Type
		}
		normalized := make([]any, 0, len(values))
		for _, value := range values {
			converted, ok := normalizeIndexProbeValue(value, fieldType)
			if !ok || converted == nil || isNilReflectValue(reflect.ValueOf(converted)) {
				return nil, false
			}
			normalized = append(normalized, converted)
		}
		next := make([][]any, 0, len(keys)*len(normalized))
		for _, prefix := range keys {
			for _, value := range normalized {
				key := append(append([]any(nil), prefix...), value)
				next = append(next, key)
				if len(next) > maxIndexProbeKeys {
					return nil, false
				}
			}
		}
		keys = next
	}
	seen := make(map[string]struct{}, len(keys))
	result := make([][]any, 0, len(keys))
	for _, key := range keys {
		encoded := encodeKey(key)
		if _, exists := seen[encoded]; exists {
			continue
		}
		seen[encoded] = struct{}{}
		result = append(result, key)
	}
	return result, len(result) > 0
}

func subqueryContextScope(variables map[string]Value) (string, string) {
	if variables == nil {
		return "", ""
	}
	contextName := ""
	partition := ""
	if value, ok := variables[subqueryContextNameVariable]; ok && value.IsPresent() {
		contextName, _ = value.Any().(string)
	}
	if value, ok := variables[subqueryContextPartitionVariable]; ok && value.IsPresent() {
		partition, _ = value.Any().(string)
	}
	return contextName, partition
}

// snapshotFireAndForgetSubquerySourceWithIndex performs one complete-key or
// equality-prefix-plus-range lookup for a subquery expression. It returns
// used=false whenever the predicate cannot be proven to provide a complete
// candidate key/range; callers then use the existing snapshot evaluator as
// the correctness path.
func (e *Engine) snapshotFireAndForgetSubquerySourceWithIndex(
	ctx context.Context,
	definition *subqueryDefinition,
	outer EvalContext,
	base *streamNode,
	now time.Time,
	variables map[string]Value,
	engineLocked bool,
) ([]Event, bool, error) {
	selection, usable := subqueryIndexSelection(e.env, base, definition)
	if !usable {
		return nil, false, nil
	}
	var keys [][]any
	var rangeQuery indexRangeSpec
	if selection.Access == IndexAccessRange {
		rangeQuery, usable = e.indexProbeRangeSpecWithOuter(
			base,
			selection,
			[]Expr{definition.predicate},
			now,
			variables,
			subqueryEnclosingEvent(outer),
		)
		if !usable {
			return nil, false, nil
		}
		if rangeQuery.empty {
			return nil, true, nil
		}
	} else {
		keys, usable = e.subqueryIndexProbeKeys(base, selection, definition.predicate, outer)
		if !usable {
			return nil, false, nil
		}
	}
	var events []Event
	switch base.kind {
	case streamNamedWindow:
		var window *NamedWindow
		if engineLocked {
			window = e.namedWindows[catalogKey(base.moduleName, base.sourceName)]
		} else {
			window, _ = e.NamedWindowInModule(base.moduleName, base.sourceName)
		}
		if window == nil {
			return nil, true, NewError(ErrorUnknownName, "named window "+base.sourceName+" is not registered")
		}
		if isSubquerySharedIndex(selection.IndexName) {
			if err := window.ensureSubquerySharedIndex(selection.IndexName, selection.Columns); err != nil {
				return nil, true, err
			}
		}
		contextName, partition := subqueryContextScope(variables)
		windowContext := strings.TrimSpace(window.Definition().Context())
		if windowContext != "" {
			if contextName != windowContext || partition == "" {
				return nil, false, nil
			}
			window.state.mu.RLock()
			partitionState := window.state.partitions[partition]
			window.state.mu.RUnlock()
			if partitionState == nil {
				return nil, true, nil
			}
			if err := contextErr(ctx); err != nil {
				return nil, true, err
			}
			if selection.Access == IndexAccessRange {
				var err error
				events, err = lookupNamedWindowRangeStateMany(partitionState, selection.IndexName, []indexRangeSpec{rangeQuery}, ctx)
				if err != nil {
					return nil, true, err
				}
			} else {
				keys, keyUsable := e.subqueryIndexProbeKeys(base, selection, definition.predicate, outer)
				if !keyUsable {
					return nil, false, nil
				}
				events = lookupNamedWindowStateMany(partitionState, selection.IndexName, keys)
			}
		} else {
			var err error
			if selection.Access == IndexAccessRange {
				events, err = window.lookupRange(ctx, selection.IndexName, rangeQuery)
			} else {
				events, err = window.lookupMany(ctx, selection.IndexName, keys)
			}
			if err != nil {
				return nil, true, err
			}
		}
	case streamTable:
		var table *Table
		var err error
		if engineLocked {
			table = e.tables[catalogKey(base.moduleName, base.sourceName)]
		} else {
			table, _ = e.TableInModule(base.moduleName, base.sourceName)
		}
		if table == nil {
			return nil, true, NewError(ErrorUnknownName, "table "+base.sourceName+" is not registered")
		}
		// A context Table owns an independent index per partition. A correlated
		// subquery has the current context pair in its variables, so it can use
		// that physical state directly when the root state is empty. Root rows
		// may be legacy writes with persistent ownership metadata; seeing any of
		// them requires the complete snapshot path to preserve that compatibility
		// behavior instead of returning only the scoped candidate rows.
		lookupScope := ""
		if table.hasScopedState() {
			contextName, partitionKey, scoped := contextTableScopeValues(variables)
			if scoped {
				rootRows, rootErr := table.hasRowsInScope(ctx, "")
				if rootErr != nil {
					return nil, true, rootErr
				}
				if rootRows {
					return nil, false, nil
				}
				lookupScope = tableContextScope(contextName, partitionKey)
			}
		}
		var rows []TableRow
		if selection.IndexName == "<primary-key>" {
			if selection.Access == IndexAccessRange {
				return nil, false, nil
			}
			keys, keyUsable := e.subqueryIndexProbeKeys(base, selection, definition.predicate, outer)
			if !keyUsable {
				return nil, false, nil
			}
			rows, err = table.lookupPrimaryManyInScope(ctx, lookupScope, keys)
		} else if selection.Access == IndexAccessRange {
			rows, err = table.lookupRangeInScope(ctx, lookupScope, selection.IndexName, rangeQuery)
		} else {
			rows, err = table.lookupManyInScope(ctx, lookupScope, selection.IndexName, keys)
		}
		if err != nil {
			return nil, true, err
		}
		events, err = tableRowsAsEvents(base, table.Definition(), rows, now)
		if err != nil {
			return nil, true, err
		}
	default:
		return nil, false, nil
	}
	return events, true, nil
}

type subqueryDefinition struct {
	source               *streamNode
	predicate            Expr
	projection           Expr
	columns              []Selection
	multiColumn          bool
	groupBy              Expr
	having               Expr
	grouped              bool
	groupedRowProjection bool
	aggregateProjection  bool
	quantified           bool
	comparison           SubqueryComparison
	cardinality          SubqueryCardinality
	orderBy              []SubqueryOrderKey
	offset               int
	limit                int
	limitSet             bool
	indexName            string
	noIndex              bool
	disableIndexSharing  bool
}

// SubqueryColumnMetadata describes one named column in a multi-column
// subquery result. Type is the statically declared Go result type of the
// column expression. Fragment is set when the column itself is another
// multi-column subquery result, preserving the nested fragment shape without
// requiring reflection over a runtime map value.
type SubqueryColumnMetadata struct {
	Name     string
	Type     reflect.Type
	Optional bool
	Fragment *SubqueryResultMetadata
}

// SubqueryResultMetadata is the Go-native counterpart of Esper's fragment
// event type metadata for SubqueryRow/SubqueryRows results. Native indicates
// that the runtime representation is a Go map or slice of maps; Indexed is
// true for a multi-row result. The value returned by SubqueryMetadata is a
// defensive copy and can be retained by callers.
type SubqueryResultMetadata struct {
	ResultType reflect.Type
	Columns    []SubqueryColumnMetadata
	Indexed    bool
	Native     bool
}

// Column returns one column descriptor by alias. Nested metadata is copied so
// callers cannot mutate the metadata retained by the expression node.
func (metadata SubqueryResultMetadata) Column(name string) (SubqueryColumnMetadata, bool) {
	for _, column := range metadata.Columns {
		if column.Name == name {
			return cloneSubqueryColumnMetadata(column), true
		}
	}
	return SubqueryColumnMetadata{}, false
}

// SubqueryMetadata returns static fragment-shaped metadata for a
// multi-column subquery expression. Scalar, aggregate, quantified and
// single-column subqueries return false. The function never evaluates the
// subquery or reads runtime state.
func SubqueryMetadata(expression Expr) (SubqueryResultMetadata, bool) {
	if expression == nil || expression.node() == nil {
		return SubqueryResultMetadata{}, false
	}
	metadata, ok := subqueryMetadataForNode(expression.node())
	if !ok {
		return SubqueryResultMetadata{}, false
	}
	return cloneSubqueryResultMetadata(metadata), true
}

func subqueryMetadataForNode(node *exprNode) (SubqueryResultMetadata, bool) {
	if node == nil || node.subquery == nil || !node.subquery.multiColumn {
		return SubqueryResultMetadata{}, false
	}
	metadata := SubqueryResultMetadata{
		ResultType: node.typ,
		Indexed:    node.kind == "subquery-rows" || node.kind == "subquery-group-rows",
		Native:     true,
	}
	metadata.Columns = make([]SubqueryColumnMetadata, 0, len(node.subquery.columns))
	for _, selection := range node.subquery.columns {
		column := SubqueryColumnMetadata{Name: selection.Name}
		if selection.Expr != nil {
			column.Type = selection.Expr.Type()
			// A subquery can legally project a null value, an empty aggregate,
			// or a missing dynamic property. Keep this fact in metadata while
			// retaining the expression's concrete result type.
			column.Optional = true
			if nested, ok := subqueryMetadataForNode(selection.Expr.node()); ok {
				nestedCopy := cloneSubqueryResultMetadata(nested)
				column.Fragment = &nestedCopy
			}
		}
		metadata.Columns = append(metadata.Columns, column)
	}
	return metadata, true
}

func cloneSubqueryColumnMetadata(column SubqueryColumnMetadata) SubqueryColumnMetadata {
	clone := column
	if column.Fragment != nil {
		fragment := cloneSubqueryResultMetadata(*column.Fragment)
		clone.Fragment = &fragment
	}
	return clone
}

func cloneSubqueryResultMetadata(metadata SubqueryResultMetadata) SubqueryResultMetadata {
	clone := metadata
	if len(metadata.Columns) == 0 {
		clone.Columns = nil
		return clone
	}
	clone.Columns = make([]SubqueryColumnMetadata, len(metadata.Columns))
	for index, column := range metadata.Columns {
		clone.Columns[index] = cloneSubqueryColumnMetadata(column)
	}
	return clone
}

// subquerySchemaOptions converts static multi-column metadata into nested
// Schema options for a projected result. The runtime representation remains
// the Go-native map or []map value, while the result schema can now materialize
// the same value as a fragment when callers need typed Event navigation.
func subquerySchemaOptions(selections []Selection) ([]SchemaOption, error) {
	options := make([]SchemaOption, 0)
	for _, selection := range selections {
		if selection.Expr == nil {
			continue
		}
		metadata, ok := SubqueryMetadata(selection.Expr)
		if !ok {
			continue
		}
		nested, err := subqueryMetadataSchema(metadata, selection.Name)
		if err != nil {
			return nil, err
		}
		options = append(options, WithNestedPropertySchema(selection.Name, nested))
	}
	return options, nil
}

func subqueryMetadataSchema(metadata SubqueryResultMetadata, name string) (Schema, error) {
	fields := make([]FieldSpec, 0, len(metadata.Columns))
	options := make([]SchemaOption, 0)
	for _, column := range metadata.Columns {
		typ := column.Type
		if typ == nil {
			typ = typeOf[any]()
		}
		fields = append(fields, FieldSpec{Name: column.Name, Type: typ, Optional: column.Optional})
		if column.Fragment != nil {
			nested, err := subqueryMetadataSchema(*column.Fragment, name+"."+column.Name)
			if err != nil {
				return Schema{}, err
			}
			options = append(options, WithNestedPropertySchema(column.Name, nested))
		}
	}
	return NewMapSchema("subquery-fragment:"+name, fields, options...)
}

type subqueryGroupValue struct {
	key   Value
	value Value
}

type subqueryCandidate struct {
	value      Value
	evaluation EvalContext
}

// SubqueryComparison selects the scalar comparison used by a quantified
// subquery. It is explicit in the Go API so rules remain analyzable instead
// of embedding an opaque comparator closure.
type SubqueryComparison uint8

const (
	SubqueryEqual SubqueryComparison = iota
	SubqueryNotEqual
	SubqueryGreater
	SubqueryGreaterOrEqual
	SubqueryLess
	SubqueryLessOrEqual
)

func (comparison SubqueryComparison) symbol() string {
	switch comparison {
	case SubqueryEqual:
		return "="
	case SubqueryNotEqual:
		return "!="
	case SubqueryGreater:
		return ">"
	case SubqueryGreaterOrEqual:
		return ">="
	case SubqueryLess:
		return "<"
	case SubqueryLessOrEqual:
		return "<="
	default:
		return "?"
	}
}

// SubqueryCardinality controls how a scalar subquery treats multiple rows.
// Expression evaluation cannot return an execution error, so the strict and
// null modes both produce Null for a multi-row result; Build still validates
// the mode and its options explicitly.
type SubqueryCardinality uint8

const (
	SubqueryFirst SubqueryCardinality = iota
	SubqueryNullOnMultiple
	SubqueryRequireSingle
)

// SubqueryOrderKey describes an analyzable inner-row sort key.
type SubqueryOrderKey struct {
	Expression Expr
	Descending bool
}

// SubqueryConfig is the option surface for scalar subqueries. It is public so
// callers can inspect or wrap option builders without exposing runtime state.
type SubqueryConfig struct {
	Predicate           Expression[bool]
	Having              Expression[bool]
	GroupBy             Expr
	Cardinality         SubqueryCardinality
	OrderBy             []SubqueryOrderKey
	Offset              int
	Limit               int
	LimitSet            bool
	IndexName           string
	NoIndex             bool
	DisableIndexSharing bool
}

// SubqueryGroupConfig controls the filter and post-group predicate for a
// grouped subquery. The key and projection remain explicit typed arguments so
// a rule does not need a row-shape closure hidden from Build validation.
type SubqueryGroupConfig struct {
	Where  Expression[bool]
	Having Expression[bool]
}

type SubqueryGroupOption func(*SubqueryGroupConfig)

func SubqueryGroupWhere(predicate Expression[bool]) SubqueryGroupOption {
	return func(config *SubqueryGroupConfig) { config.Where = predicate }
}

func SubqueryGroupHaving(predicate Expression[bool]) SubqueryGroupOption {
	return func(config *SubqueryGroupConfig) { config.Having = predicate }
}

type SubqueryOption func(*SubqueryConfig)

func SubqueryWhere(predicate Expression[bool]) SubqueryOption {
	return func(config *SubqueryConfig) { config.Predicate = predicate }
}

// SubqueryHaving applies a post-aggregation predicate to an ungrouped
// aggregate subquery. The predicate is evaluated once against the complete
// inner aggregate group and may explicitly reference the outer event through
// OuterField, matching Esper's correlated having clause.
func SubqueryHaving(predicate Expression[bool]) SubqueryOption {
	return func(config *SubqueryConfig) { config.Having = predicate }
}

// SubqueryGroupKey turns an IN/ANY/ALL/SOME/EXISTS/value subquery into the
// grouped form `select <projection> from <source> group by <key>`: the
// projection is evaluated once per group (aggregate projections produce one
// value per group) and a SubqueryHaving option then filters groups. An empty
// group set follows SQL empty-set semantics: IN/ANY/SOME are false, ALL is
// true and EXISTS is false, matching Esper's grouped subselects.
func SubqueryGroupKey(expression Expr) SubqueryOption {
	return func(config *SubqueryConfig) { config.GroupBy = expression }
}

func SubqueryOrderBy(expression Expr, descending bool) SubqueryOption {
	return func(config *SubqueryConfig) {
		config.OrderBy = append(config.OrderBy, SubqueryOrderKey{Expression: expression, Descending: descending})
	}
}

func SubqueryAscending(expression Expr) SubqueryOption {
	return SubqueryOrderBy(expression, false)
}

func SubqueryDescending(expression Expr) SubqueryOption {
	return SubqueryOrderBy(expression, true)
}

func SubqueryOffset(offset int) SubqueryOption {
	return func(config *SubqueryConfig) { config.Offset = offset }
}

func SubqueryLimit(limit int) SubqueryOption {
	return func(config *SubqueryConfig) {
		config.Limit = limit
		config.LimitSet = true
	}
}

func SubqueryCardinalityMode(mode SubqueryCardinality) SubqueryOption {
	return func(config *SubqueryConfig) { config.Cardinality = mode }
}

// SubqueryUseIndex binds a correlated subquery to one declared hash/B-tree
// index on its root Named Window or Table source. It is the structured Go
// counterpart of Esper's subquery index hint.
func SubqueryUseIndex(name string) SubqueryOption {
	return func(config *SubqueryConfig) { config.IndexName = strings.TrimSpace(name) }
}

// SubqueryNoIndex forces the complete source snapshot path for this subquery,
// even when a declared or shared index could otherwise satisfy its predicate.
func SubqueryNoIndex() SubqueryOption {
	return func(config *SubqueryConfig) { config.NoIndex = true }
}

// SubqueryDisableIndexSharing disables only automatic Named Window shared
// indexes for this consumer. Explicitly declared indexes remain eligible.
func SubqueryDisableIndexSharing() SubqueryOption {
	return func(config *SubqueryConfig) { config.DisableIndexSharing = true }
}

// SubqueryExists evaluates whether at least one event from a named-window or
// table source satisfies predicate. Field expressions in predicate address
// the inner source; OuterField expressions address the enclosing event.
func SubqueryExists(source RecordStream, predicate Expression[bool]) Expression[bool] {
	return SubqueryExistsWithOptions(source, predicate)
}

// SubqueryExistsWithOptions is the option-bearing form of SubqueryExists. It
// supports the same structured index controls as projected subqueries.
func SubqueryExistsWithOptions(source RecordStream, predicate Expression[bool], options ...SubqueryOption) Expression[bool] {
	config := SubqueryConfig{Cardinality: SubqueryFirst, Predicate: predicate}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	definition := &subqueryDefinition{
		source:              source.node,
		predicate:           config.Predicate,
		indexName:           config.IndexName,
		noIndex:             config.NoIndex,
		disableIndexSharing: config.DisableIndexSharing,
	}
	return makeSubqueryExpr[bool]("subquery-exists", "exists("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		values := evaluateSubqueryValues(definition, ctx)
		return Present(len(values) > 0)
	})
}

// SubqueryExistsValue evaluates whether a projected subquery produces at
// least one result row. Unlike Exists(SubqueryValue(...)), this keeps a
// present aggregate row distinct from an aggregate value that happens to be
// null and therefore supports correlated aggregate having clauses.
func SubqueryExistsValue[T any](source RecordStream, projection Expression[T], options ...SubqueryOption) Expression[bool] {
	definition := subqueryWithProjectionOptions(source, projection, options...)
	return makeSubqueryExpr[bool]("subquery-exists-value", "exists("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		return Present(len(unwrapSubqueryGroupValues(evaluateSubqueryValues(definition, ctx))) > 0)
	})
}

// SubqueryValue returns the first matching projected value. The first-value
// rule is deterministic because snapshots preserve named-window/table order;
// a future scalar-cardinality option can reject or aggregate multiple rows
// without changing the source and correlation contracts.
func SubqueryValue[T any](source RecordStream, projection Expression[T], predicate ...Expression[bool]) Expression[T] {
	options := make([]SubqueryOption, 0, 1)
	if len(predicate) > 0 && predicate[0] != nil {
		options = append(options, SubqueryWhere(predicate[0]))
	}
	return SubqueryValueWithOptions[T](source, projection, options...)
}

// SubqueryValueWithOptions adds explicit scalar cardinality, inner ordering,
// offset/limit and predicate options while keeping the ordinary SubqueryValue
// shorthand source-compatible.
func SubqueryValueWithOptions[T any](source RecordStream, projection Expression[T], options ...SubqueryOption) Expression[T] {
	config := SubqueryConfig{Cardinality: SubqueryFirst}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	definition := &subqueryDefinition{
		source:              source.node,
		predicate:           config.Predicate,
		having:              config.Having,
		projection:          projection,
		aggregateProjection: isAggregateExpression(projection),
		groupBy:             config.GroupBy,
		grouped:             config.GroupBy != nil,
		cardinality:         config.Cardinality,
		orderBy:             append([]SubqueryOrderKey(nil), config.OrderBy...),
		offset:              config.Offset,
		limit:               config.Limit,
		limitSet:            config.LimitSet,
		indexName:           config.IndexName,
		noIndex:             config.NoIndex,
		disableIndexSharing: config.DisableIndexSharing,
	}
	return makeSubqueryExpr[T]("subquery-value", "value("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		values := unwrapSubqueryGroupValues(evaluateSubqueryValues(definition, ctx))
		if len(values) == 0 {
			return Null()
		}
		if definition.cardinality != SubqueryFirst && len(values) > 1 {
			return Null()
		}
		return values[0]
	})
}

// SubqueryValues returns every projected row from a subquery as a typed Go
// slice. A nil projection selects the inner Event envelope, which is the
// chain-friendly counterpart of Esper's `(select * from ... )` collection
// source. Predicate, ordering, offset and limit options apply before the
// slice is materialized.
func SubqueryValues[T any](source RecordStream, projection Expression[T], options ...SubqueryOption) Expression[[]T] {
	config := SubqueryConfig{Cardinality: SubqueryFirst}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	definition := &subqueryDefinition{
		source:              source.node,
		predicate:           config.Predicate,
		having:              config.Having,
		projection:          projection,
		aggregateProjection: isAggregateExpression(projection),
		orderBy:             append([]SubqueryOrderKey(nil), config.OrderBy...),
		offset:              config.Offset,
		limit:               config.Limit,
		limitSet:            config.LimitSet,
		indexName:           config.IndexName,
		noIndex:             config.NoIndex,
		disableIndexSharing: config.DisableIndexSharing,
	}
	return makeSubqueryExpr[[]T]("subquery-values", "values("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		values := evaluateSubqueryValues(definition, ctx)
		result := make([]T, 0, len(values))
		for _, value := range values {
			if !value.IsPresent() {
				var zero T
				result = append(result, zero)
				continue
			}
			converted, err := As[T](value)
			if err != nil {
				return Null()
			}
			result = append(result, converted)
		}
		return Present(result)
	})
}

// SubqueryEvents is the no-projection convenience for a collection of inner
// Event envelopes. Property and Method expressions can continue from each
// item through EnumField/Property in the normal enumerable lambda context.
func SubqueryEvents(source RecordStream, options ...SubqueryOption) Expression[[]Event] {
	return SubqueryValues[Event](source, nil, options...)
}

// SubqueryRow returns the first projected row from a multi-column subquery.
// The row is a Go-native map keyed by the explicit Selection aliases; null or
// missing inner values are represented by nil map values. Use SubqueryRows
// when the inner source may return more than one row.
func SubqueryRow(source RecordStream, selections ...Selection) Expression[map[string]any] {
	return SubqueryRowWithOptions(source, selections)
}

// SubqueryRowWithOptions adds the ordinary subquery predicate, ordering,
// offset/limit and scalar-cardinality options to a multi-column row.
func SubqueryRowWithOptions(source RecordStream, selections []Selection, options ...SubqueryOption) Expression[map[string]any] {
	definition := newSubqueryColumnsDefinition(source, selections, options...)
	return makeSubqueryExpr[map[string]any]("subquery-row", "row("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		values := evaluateSubqueryValues(definition, ctx)
		if len(values) == 0 || (definition.cardinality != SubqueryFirst && len(values) > 1) {
			return Null()
		}
		row, ok := values[0].Any().(map[string]any)
		if !ok {
			return Null()
		}
		return Present(row)
	})
}

// subqueryRowToUnderlying converts a map-based subquery projection row into
// the underlying value shape expected by the given schema. Object-array
// schemas need positional []any; map and bean schemas accept map[string]any.
func subqueryRowToUnderlying(schema Schema, row map[string]any) (any, error) {
	if schema.kind == SchemaObjectArray {
		fields := schema.Fields()
		underlying := make([]any, len(fields))
		for index, field := range fields {
			underlying[index] = row[field.Name]
		}
		return underlying, nil
	}
	return row, nil
}

// SubqueryRowAsEvent projects named columns from a single-row subquery and
// materializes the result as an Event of the named schema type.  This bridges
// the gap between SubqueryRow (which returns Expression[map[string]any]) and
// insert-into routes whose target column type is Event.  When the inner
// source returns zero rows the result is null; one row is materialized; more
// than one row yields null (strict single-row cardinality).
func SubqueryRowAsEvent(env *Environment, schemaName string, source RecordStream, selections ...Selection) Expression[Event] {
	return SubqueryRowAsEventWithOptions(env, schemaName, source, selections, SubqueryCardinalityMode(SubqueryNullOnMultiple))
}

// SubqueryRowAsEventWithOptions applies predicate, ordering and cardinality
// options before materializing the single-row subquery result as an Event.
func SubqueryRowAsEventWithOptions(env *Environment, schemaName string, source RecordStream, selections []Selection, options ...SubqueryOption) Expression[Event] {
	definition := newSubqueryColumnsDefinition(source, selections, options...)
	return makeSubqueryExpr[Event]("subquery-row-as-event", "rowAsEvent("+schemaName+","+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		values := evaluateSubqueryValues(definition, ctx)
		if len(values) == 0 {
			return Null()
		}
		row, ok := values[0].Any().(map[string]any)
		if !ok {
			return Null()
		}
		schema, found := env.Schema(schemaName)
		if !found {
			return Null()
		}
		underlying, err := subqueryRowToUnderlying(schema, row)
		if err != nil {
			return Null()
		}
		now := time.Time{}
		if ctx.Engine != nil {
			now = ctx.Engine.Now()
		}
		event, err := newEvent(schema, underlying, now)
		if err != nil {
			return Null()
		}
		return Present(event)
	})
}

// SubqueryRowsAsEvent projects named columns from a multi-row subquery and
// materializes every row as an Event of the named schema type.  The result
// is Expression[[]Event] which can be routed into an array-typed event column.
func SubqueryRowsAsEvent(env *Environment, schemaName string, source RecordStream, selections ...Selection) Expression[[]Event] {
	return SubqueryRowsAsEventWithOptions(env, schemaName, source, selections)
}

// SubqueryRowsAsEventWithOptions applies predicate, ordering, offset and
// limit options before materializing the multi-row subquery result as Events.
func SubqueryRowsAsEventWithOptions(env *Environment, schemaName string, source RecordStream, selections []Selection, options ...SubqueryOption) Expression[[]Event] {
	definition := newSubqueryColumnsDefinition(source, selections, options...)
	return makeSubqueryExpr[[]Event]("subquery-rows-as-event", "rowsAsEvent("+schemaName+","+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		values := evaluateSubqueryValues(definition, ctx)
		schema, found := env.Schema(schemaName)
		if !found {
			return Null()
		}
		now := time.Time{}
		if ctx.Engine != nil {
			now = ctx.Engine.Now()
		}
		events := make([]Event, 0, len(values))
		for _, value := range values {
			row, ok := value.Any().(map[string]any)
			if !ok {
				return Null()
			}
			underlying, err := subqueryRowToUnderlying(schema, row)
			if err != nil {
				return Null()
			}
			event, err := newEvent(schema, underlying, now)
			if err != nil {
				return Null()
			}
			events = append(events, event)
		}
		return Present(events)
	})
}

// SubqueryRows returns every projected row from a multi-column subquery. Each
// Selection alias becomes one map key, preserving the declared selection order
// only in the AST; callers that need deterministic presentation can keep the
// returned slice order and use the aliases for lookup.
func SubqueryRows(source RecordStream, selections ...Selection) Expression[[]map[string]any] {
	return SubqueryRowsWithOptions(source, selections)
}

// SubqueryRowsWithOptions applies predicate, ordering, offset and limit before
// materializing the multi-column rows.
func SubqueryRowsWithOptions(source RecordStream, selections []Selection, options ...SubqueryOption) Expression[[]map[string]any] {
	definition := newSubqueryColumnsDefinition(source, selections, options...)
	return makeSubqueryExpr[[]map[string]any]("subquery-rows", "rows("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		values := evaluateSubqueryValues(definition, ctx)
		rows := make([]map[string]any, 0, len(values))
		for _, value := range values {
			row, ok := value.Any().(map[string]any)
			if !ok {
				return Null()
			}
			rows = append(rows, row)
		}
		return Present(rows)
	})
}

// SubqueryGroupRows returns one multi-column map row per accepted group. The
// key is kept in the group evaluation context, while each Selection is
// evaluated against that group so scalar key columns and aggregate columns
// can be mixed in the same row.
func SubqueryGroupRows(source RecordStream, key Expr, selections []Selection, options ...SubqueryGroupOption) Expression[[]map[string]any] {
	config := SubqueryGroupConfig{}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	definition := newSubqueryColumnsDefinition(source, selections)
	definition.predicate = config.Where
	definition.groupBy = key
	definition.having = config.Having
	definition.grouped = true
	definition.groupedRowProjection = true
	return makeSubqueryExpr[[]map[string]any]("subquery-group-rows", "group-rows("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		values := evaluateSubqueryValues(definition, ctx)
		rows := make([]map[string]any, 0, len(values))
		for _, value := range values {
			group, ok := value.Any().(subqueryGroupValue)
			if !ok {
				return Null()
			}
			row, ok := group.value.Any().(map[string]any)
			if !ok {
				return Null()
			}
			rows = append(rows, row)
		}
		return Present(rows)
	})
}

// SubqueryGroupBy groups the current inner snapshot by key and returns one
// typed value bucket per key. A scalar projection contributes one value per
// accepted inner event; an aggregate projection such as Sum contributes one
// value per group. SubqueryGroupHaving is evaluated with that group's
// EvalContext, so aggregate expressions can be used without an opaque
// callback. The map representation is intentionally Go-native; callers can
// continue with EnumValues/Func expressions when a flattened collection is
// desired.
func SubqueryGroupBy[K comparable, V any](source RecordStream, key Expression[K], projection Expression[V], options ...SubqueryGroupOption) Expression[map[K][]V] {
	config := SubqueryGroupConfig{}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	definition := &subqueryDefinition{
		source:              source.node,
		predicate:           config.Where,
		projection:          projection,
		groupBy:             key,
		having:              config.Having,
		grouped:             true,
		aggregateProjection: isAggregateExpression(projection),
	}
	return makeSubqueryExpr[map[K][]V]("subquery-group-by", "group-by("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		values := evaluateSubqueryValues(definition, ctx)
		result := make(map[K][]V)
		for _, value := range values {
			group, ok := value.Any().(subqueryGroupValue)
			if !ok {
				return Null()
			}
			var keyValue K
			if group.key.IsPresent() {
				converted, err := As[K](group.key)
				if err != nil {
					return Null()
				}
				keyValue = converted
			}
			var projected V
			if group.value.IsPresent() {
				converted, err := As[V](group.value)
				if err != nil {
					return Null()
				}
				projected = converted
			}
			result[keyValue] = append(result[keyValue], projected)
		}
		return Present(result)
	})
}

// SubqueryGroupBucket is one ordered result bucket from SubqueryGroupByAny.
// Key is the typed Go value when the group key is present; KeyValue preserves
// Missing/Null and supports non-comparable keys such as slices and arrays.
// Values contains one projected value per accepted inner row, or one value
// for an aggregate projection.
type SubqueryGroupBucket[K any, V any] struct {
	Key      K
	KeyValue Value
	Values   []V
}

// SubqueryGroupByAny groups a subquery by any expression result, including
// slice, array, map and struct keys that cannot satisfy Go's comparable map
// key constraint. It is the typed bucket counterpart of Esper's multi-row
// grouped subselect; bucket order follows first group appearance in the
// inner snapshot.
func SubqueryGroupByAny[K any, V any](source RecordStream, key Expression[K], projection Expression[V], options ...SubqueryGroupOption) Expression[[]SubqueryGroupBucket[K, V]] {
	config := SubqueryGroupConfig{}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	definition := &subqueryDefinition{
		source:              source.node,
		predicate:           config.Where,
		projection:          projection,
		groupBy:             key,
		having:              config.Having,
		grouped:             true,
		aggregateProjection: isAggregateExpression(projection),
	}
	return makeSubqueryExpr[[]SubqueryGroupBucket[K, V]]("subquery-group-by-any", "group-by-any("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		values := evaluateSubqueryValues(definition, ctx)
		buckets := make([]SubqueryGroupBucket[K, V], 0, len(values))
		for _, value := range values {
			group, ok := value.Any().(subqueryGroupValue)
			if !ok {
				return Null()
			}
			bucketIndex := -1
			for index := range buckets {
				if subqueryValuesEqual(buckets[index].KeyValue, group.key) {
					bucketIndex = index
					break
				}
			}
			if bucketIndex < 0 {
				var typedKey K
				if group.key.IsPresent() {
					converted, err := As[K](group.key)
					if err != nil {
						return Null()
					}
					typedKey = converted
				}
				buckets = append(buckets, SubqueryGroupBucket[K, V]{Key: typedKey, KeyValue: group.key})
				bucketIndex = len(buckets) - 1
			}
			var projected V
			if group.value.IsPresent() {
				converted, err := As[V](group.value)
				if err != nil {
					return Null()
				}
				projected = converted
			}
			buckets[bucketIndex].Values = append(buckets[bucketIndex].Values, projected)
		}
		return Present(buckets)
	})
}

// SubqueryGroupScalar groups the current inner snapshot by key and returns
// the single accepted group's projected value as a scalar, mirroring Esper's
// scalar grouped subselect: zero groups produce null and multiple groups
// produce null (Esper assigns null when a scalar grouped subselect returns
// more than one row, as observed in the update-istream multikey regression).
// An aggregate projection such as Sum contributes one value per group; a
// scalar projection contributes the first accepted row of the single group.
// Slice, array, map and struct keys are supported because the key value is
// never used as a Go map key.
func SubqueryGroupScalar[K any, V any](source RecordStream, key Expression[K], projection Expression[V], options ...SubqueryGroupOption) Expression[V] {
	config := SubqueryGroupConfig{}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	definition := &subqueryDefinition{
		source:              source.node,
		predicate:           config.Where,
		projection:          projection,
		groupBy:             key,
		having:              config.Having,
		grouped:             true,
		aggregateProjection: isAggregateExpression(projection),
	}
	return makeSubqueryExpr[V]("subquery-group-scalar", "group-scalar("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		values := evaluateSubqueryValues(definition, ctx)
		if len(values) == 0 {
			return Null()
		}
		first, ok := values[0].Any().(subqueryGroupValue)
		if !ok {
			return Null()
		}
		// A grouped subquery collapses to one row per group: the scalar form
		// requires exactly one distinct group and returns its first projected
		// value. Multiple events inside the single group still produce one
		// group row (the group-key projection is constant for the group),
		// matching Esper's scalar grouped subselect.
		for _, value := range values[1:] {
			group, ok := value.Any().(subqueryGroupValue)
			if !ok || !subqueryValuesEqual(group.key, first.key) {
				return Null()
			}
		}
		return first.value
	})
}

func newSubqueryColumnsDefinition(source RecordStream, selections []Selection, options ...SubqueryOption) *subqueryDefinition {
	config := SubqueryConfig{Cardinality: SubqueryFirst}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	columns := append([]Selection(nil), selections...)
	return &subqueryDefinition{
		source:              source.node,
		predicate:           config.Predicate,
		having:              config.Having,
		columns:             columns,
		multiColumn:         true,
		aggregateProjection: subqueryColumnsHaveAggregate(columns),
		cardinality:         config.Cardinality,
		orderBy:             append([]SubqueryOrderKey(nil), config.OrderBy...),
		offset:              config.Offset,
		limit:               config.Limit,
		limitSet:            config.LimitSet,
		indexName:           config.IndexName,
		noIndex:             config.NoIndex,
		disableIndexSharing: config.DisableIndexSharing,
	}
}

func subqueryColumnsHaveAggregate(columns []Selection) bool {
	for _, selection := range columns {
		if isAggregateExpression(selection.Expr) {
			return true
		}
	}
	return false
}

// SubqueryIn compares value with the projected values of a named-window or
// table subquery. It follows the engine's three-valued comparison contract:
// a null/missing outer value yields Null, a matching value yields true, and a
// null inner value yields Null only when no present value matches.
func SubqueryIn[T comparable](value Expression[T], source RecordStream, projection Expression[T], predicate ...Expression[bool]) Expression[bool] {
	options := make([]SubqueryOption, 0, 1)
	if len(predicate) > 0 && predicate[0] != nil {
		options = append(options, SubqueryWhere(predicate[0]))
	}
	return SubqueryInWithOptions[T](value, source, projection, options...)
}

// SubqueryInWithOptions adds post-aggregation having, ordering and other
// analyzable options to an IN subquery while keeping SubqueryIn concise for
// the common row-predicate form.
func SubqueryInWithOptions[T comparable](value Expression[T], source RecordStream, projection Expression[T], options ...SubqueryOption) Expression[bool] {
	definition := subqueryWithProjectionOptions(source, projection, options...)
	return makeSubqueryExprWithChildren[bool]("subquery-in", "("+value.Description()+" in "+subqueryDescription(definition)+")", definition, []*exprNode{value.node()}, func(ctx EvalContext) Value {
		values := unwrapSubqueryGroupValues(evaluateSubqueryValues(definition, ctx))
		// Esper follows the SQL empty-set rule for IN: there is no matching
		// candidate, even when the outer value itself is null.
		if len(values) == 0 {
			if definition.aggregateProjection && !definition.grouped {
				// An aggregate subquery with a HAVING clause can produce no
				// aggregate result row. Esper exposes that missing aggregate row
				// as Null rather than as a scalar empty collection.
				return Null()
			}
			return Present(false)
		}
		outer := value.eval(ctx)
		if !outer.IsPresent() {
			return Null()
		}
		hasNull := false
		for _, candidate := range values {
			if !candidate.IsPresent() {
				hasNull = true
				continue
			}
			if equal, ok := boolValue(EqualValues(outer, candidate)); ok && equal {
				return Present(true)
			}
		}
		if hasNull {
			return Null()
		}
		return Present(false)
	})
}

// SubqueryCount counts matching inner rows. A zero count is present and
// distinguishes an empty result from a null projected value.
func SubqueryCount(source RecordStream, predicate ...Expression[bool]) Expression[int64] {
	definition := &subqueryDefinition{source: source.node}
	if len(predicate) > 0 && predicate[0] != nil {
		definition.predicate = predicate[0]
	}
	return makeSubqueryExpr[int64]("subquery-count", "count("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		return Present(int64(len(evaluateSubqueryValues(definition, ctx))))
	})
}

// SubquerySum evaluates a numeric projection over matching inner rows. It
// follows Esper aggregate null behavior: no numeric input yields Null. The
// definition carries the wrapped Sum projection so unbound event-stream
// sources are accepted (an aggregate subselect over all events ever) and the
// aggregate evaluation path applies to window, named-window and table
// sources alike.
func SubquerySum[T Numeric](source RecordStream, projection Expression[T], predicate ...Expression[bool]) Expression[T] {
	options := make([]SubqueryOption, 0, 1)
	if len(predicate) > 0 && predicate[0] != nil {
		options = append(options, SubqueryWhere(predicate[0]))
	}
	definition := subqueryWithProjectionOptions(source, Sum[T](projection), options...)
	return makeSubqueryExpr[T]("subquery-sum", "sum("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		var total float64
		found := false
		for _, candidate := range unwrapSubqueryGroupValues(evaluateSubqueryValues(definition, ctx)) {
			value, ok := numericValue(candidate)
			if !ok {
				continue
			}
			total += value
			found = true
		}
		if !found {
			return Null()
		}
		return Present(convertNumeric[T](total))
	})
}

// SubqueryAvg evaluates a numeric projection over matching inner rows and
// returns a float64 average, or Null when no numeric row remains. The
// definition carries the wrapped Avg projection for the same reasons as
// SubquerySum.
func SubqueryAvg[T Numeric](source RecordStream, projection Expression[T], predicate ...Expression[bool]) Expression[float64] {
	options := make([]SubqueryOption, 0, 1)
	if len(predicate) > 0 && predicate[0] != nil {
		options = append(options, SubqueryWhere(predicate[0]))
	}
	definition := subqueryWithProjectionOptions(source, Avg[T](projection), options...)
	return makeSubqueryExpr[float64]("subquery-avg", "avg("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		var total float64
		var count int64
		for _, candidate := range unwrapSubqueryGroupValues(evaluateSubqueryValues(definition, ctx)) {
			value, ok := numericValue(candidate)
			if !ok {
				continue
			}
			total += value
			count++
		}
		if count == 0 {
			return Null()
		}
		return Present(total / float64(count))
	})
}

// SubqueryAny applies a scalar comparison to each projected inner row and
// returns true when at least one comparison is true. Null candidates preserve
// three-valued semantics when no definite true result exists.
func SubqueryAny[T any](value Expression[T], source RecordStream, projection Expression[T], comparison SubqueryComparison, predicate ...Expression[bool]) Expression[bool] {
	options := make([]SubqueryOption, 0, 1)
	if len(predicate) > 0 && predicate[0] != nil {
		options = append(options, SubqueryWhere(predicate[0]))
	}
	return SubqueryAnyWithOptions[T](value, source, projection, comparison, options...)
}

// SubqueryAnyWithOptions is the option-bearing form of SubqueryAny. It is
// useful for Esper-style aggregate subqueries with a having clause.
func SubqueryAnyWithOptions[T any](value Expression[T], source RecordStream, projection Expression[T], comparison SubqueryComparison, options ...SubqueryOption) Expression[bool] {
	definition := subqueryWithProjectionOptions(source, projection, options...)
	definition.quantified = true
	definition.comparison = comparison
	description := fmt.Sprintf("%s %s any (%s)", value.Description(), comparison.symbol(), subqueryDescription(definition))
	return makeSubqueryExprWithChildren[bool]("subquery-any", description, definition, []*exprNode{value.node()}, func(ctx EvalContext) Value {
		values := unwrapSubqueryGroupValues(evaluateSubqueryValues(definition, ctx))
		return evaluateQuantifiedSubquery(value.eval(ctx), values, comparison, false, definition.aggregateProjection && !definition.grouped)
	})
}

// SubquerySome is the SQL/Esper synonym for SubqueryAny.
func SubquerySome[T any](value Expression[T], source RecordStream, projection Expression[T], comparison SubqueryComparison, predicate ...Expression[bool]) Expression[bool] {
	return SubqueryAny[T](value, source, projection, comparison, predicate...)
}

// SubquerySomeWithOptions is the option-bearing synonym of
// SubqueryAnyWithOptions.
func SubquerySomeWithOptions[T any](value Expression[T], source RecordStream, projection Expression[T], comparison SubqueryComparison, options ...SubqueryOption) Expression[bool] {
	return SubqueryAnyWithOptions[T](value, source, projection, comparison, options...)
}

// SubqueryAll applies a scalar comparison to every projected inner row.
func SubqueryAll[T any](value Expression[T], source RecordStream, projection Expression[T], comparison SubqueryComparison, predicate ...Expression[bool]) Expression[bool] {
	options := make([]SubqueryOption, 0, 1)
	if len(predicate) > 0 && predicate[0] != nil {
		options = append(options, SubqueryWhere(predicate[0]))
	}
	return SubqueryAllWithOptions[T](value, source, projection, comparison, options...)
}

// SubqueryAllWithOptions is the option-bearing form of SubqueryAll.
func SubqueryAllWithOptions[T any](value Expression[T], source RecordStream, projection Expression[T], comparison SubqueryComparison, options ...SubqueryOption) Expression[bool] {
	definition := subqueryWithProjectionOptions(source, projection, options...)
	definition.quantified = true
	definition.comparison = comparison
	description := fmt.Sprintf("%s %s all (%s)", value.Description(), comparison.symbol(), subqueryDescription(definition))
	return makeSubqueryExprWithChildren[bool]("subquery-all", description, definition, []*exprNode{value.node()}, func(ctx EvalContext) Value {
		values := unwrapSubqueryGroupValues(evaluateSubqueryValues(definition, ctx))
		return evaluateQuantifiedSubquery(value.eval(ctx), values, comparison, true, definition.aggregateProjection && !definition.grouped)
	})
}

func subqueryWithProjection(source RecordStream, projection Expr, predicate ...Expression[bool]) *subqueryDefinition {
	options := make([]SubqueryOption, 0, 1)
	if len(predicate) > 0 && predicate[0] != nil {
		options = append(options, SubqueryWhere(predicate[0]))
	}
	return subqueryWithProjectionOptions(source, projection, options...)
}

func subqueryWithProjectionOptions(source RecordStream, projection Expr, options ...SubqueryOption) *subqueryDefinition {
	config := SubqueryConfig{Cardinality: SubqueryFirst}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	return &subqueryDefinition{
		source:              source.node,
		predicate:           config.Predicate,
		having:              config.Having,
		projection:          projection,
		aggregateProjection: isAggregateExpression(projection),
		groupBy:             config.GroupBy,
		grouped:             config.GroupBy != nil,
		cardinality:         config.Cardinality,
		orderBy:             append([]SubqueryOrderKey(nil), config.OrderBy...),
		offset:              config.Offset,
		limit:               config.Limit,
		limitSet:            config.LimitSet,
		indexName:           config.IndexName,
		noIndex:             config.NoIndex,
		disableIndexSharing: config.DisableIndexSharing,
	}
}

// isAggregateExpression identifies the expression shape rather than relying
// on a concrete aggregate implementation. Aggregate expressions use this
// private marker so the planner and subquery evaluator can distinguish an
// aggregate projection (one row over the inner group) from a scalar
// projection (one row per inner event) without exposing runtime internals in
// the public API.
func isAggregateExpression(expression Expr) bool {
	if expression == nil {
		return false
	}
	_, ok := expression.(interface{ aggregateMarker() })
	if ok {
		return true
	}
	return expressionNodeContainsAggregate(expression.node())
}

func evaluateQuantifiedSubquery(left Value, values []Value, comparison SubqueryComparison, all, aggregateEmpty bool) Value {
	if len(values) == 0 {
		if aggregateEmpty {
			return Null()
		}
		if all {
			return Present(true)
		}
		return Present(false)
	}
	if !left.IsPresent() {
		return Null()
	}
	hasNull := false
	for _, right := range values {
		result := compareSubqueryValues(left, right, comparison)
		matched, present := boolValue(result)
		if !present {
			hasNull = true
			continue
		}
		if all && !matched {
			return Present(false)
		}
		if !all && matched {
			return Present(true)
		}
	}
	if hasNull {
		return Null()
	}
	if all {
		return Present(true)
	}
	return Present(false)
}

func compareSubqueryValues(left, right Value, comparison SubqueryComparison) Value {
	switch comparison {
	case SubqueryEqual:
		return EqualValues(left, right)
	case SubqueryNotEqual:
		result := EqualValues(left, right)
		if !result.IsPresent() {
			return result
		}
		return Present(!result.Any().(bool))
	case SubqueryGreater, SubqueryGreaterOrEqual, SubqueryLess, SubqueryLessOrEqual:
		ordering, ok := compareValues(left, right)
		if !ok {
			return Null()
		}
		switch comparison {
		case SubqueryGreater:
			return Present(ordering > 0)
		case SubqueryGreaterOrEqual:
			return Present(ordering >= 0)
		case SubqueryLess:
			return Present(ordering < 0)
		default:
			return Present(ordering <= 0)
		}
	default:
		return Null()
	}
}

func makeSubqueryExpr[T any](kind, description string, definition *subqueryDefinition, fn func(EvalContext) Value) Expression[T] {
	return makeSubqueryExprWithChildren[T](kind, description, definition, nil, fn)
}

func makeSubqueryExprWithChildren[T any](kind, description string, definition *subqueryDefinition, children []*exprNode, fn func(EvalContext) Value) Expression[T] {
	expression := makeExpr[T](kind, description, children, fn)
	node := expression.(typedExpr[T]).n
	node.subquery = definition
	return expression
}

func subqueryDescription(definition *subqueryDefinition) string {
	if definition == nil || definition.source == nil {
		return "<invalid>"
	}
	description := definition.source.describe()
	if definition.predicate != nil {
		description += ".where(" + definition.predicate.Description() + ")"
	}
	if definition.projection != nil {
		description += ".select(" + definition.projection.Description() + ")"
	}
	if len(definition.columns) > 0 {
		columns := make([]string, 0, len(definition.columns))
		for _, selection := range definition.columns {
			if selection.Expr == nil {
				columns = append(columns, selection.Name+"=<nil>")
				continue
			}
			columns = append(columns, selection.Name+"="+selection.Expr.Description())
		}
		description += ".select(" + strings.Join(columns, ",") + ")"
	}
	if definition.groupBy != nil {
		description += ".groupBy(" + definition.groupBy.Description() + ")"
	}
	if definition.having != nil {
		description += ".having(" + definition.having.Description() + ")"
	}
	for _, order := range definition.orderBy {
		direction := "asc"
		if order.Descending {
			direction = "desc"
		}
		if order.Expression != nil {
			description += ".order(" + order.Expression.Description() + "," + direction + ")"
		}
	}
	if definition.offset != 0 {
		description += fmt.Sprintf(".offset(%d)", definition.offset)
	}
	if definition.limitSet {
		description += fmt.Sprintf(".limit(%d)", definition.limit)
	}
	if definition.indexName != "" {
		description += ".useIndex(" + definition.indexName + ")"
	}
	if definition.noIndex {
		description += ".noIndex()"
	}
	if definition.disableIndexSharing {
		description += ".disableIndexSharing()"
	}
	return description
}

func evaluateSubqueryValues(definition *subqueryDefinition, outer EvalContext) []Value {
	if definition == nil || definition.source == nil {
		return nil
	}
	e := outer.Engine
	engineLocked := false
	if outer.Variables != nil {
		if value, ok := outer.Variables[subqueryEngineVariable]; ok && value.IsPresent() {
			switch reference := value.Any().(type) {
			case *subqueryEngineRef:
				if e == nil {
					e = reference.engine
				}
				engineLocked = reference.locked
			case *Engine:
				if e == nil {
					e = reference
				}
			}
		}
	}
	if e == nil {
		return nil
	}
	base, err := subqueryRootSource(definition.source)
	if err != nil {
		return nil
	}
	now := outer.Now
	if now.IsZero() {
		now = e.Now()
	}
	var events []Event
	usingRuntimeSnapshot := false
	usingIndexedSnapshot := false
	if registry := subqueryRuntimeFromVariables(outer.Variables); registry != nil {
		if snapshot, ok := registry.snapshot(definition); ok {
			events = snapshot
			usingRuntimeSnapshot = true
		}
	}
	if !usingRuntimeSnapshot {
		// A FAF subquery is evaluated once for each outer row.  When its
		// predicate exposes a complete equality/IN key (including an
		// OuterField value), use the declared Named Window/Table index to
		// produce candidates.  This is intentionally a pruning step only;
		// the ordinary temporary runtime below still evaluates the complete
		// predicate and projection, preserving scalar cardinality and null
		// semantics.
		if indexed, used, indexErr := e.snapshotFireAndForgetSubquerySourceWithIndex(
			context.Background(), definition, outer, base, now, outer.Variables, engineLocked,
		); used {
			if indexErr != nil {
				return nil
			}
			events = indexed
			usingIndexedSnapshot = true
		}
	}
	if !usingRuntimeSnapshot && !usingIndexedSnapshot {
		contextName, contextPartition := subqueryContextScope(outer.Variables)
		if base.kind == streamNamedWindow && contextName != "" && contextPartition != "" {
			var window *NamedWindow
			if engineLocked {
				window = e.namedWindows[catalogKey(base.moduleName, base.sourceName)]
			} else {
				window, _ = e.NamedWindowInModule(base.moduleName, base.sourceName)
			}
			if window != nil && window.Definition().Context() == contextName {
				events, err = window.SnapshotContext(context.Background(), contextPartition)
			} else if engineLocked {
				events, err = e.snapshotFireAndForgetSourceLocked(context.Background(), base, now, outer.Variables)
			} else {
				events, err = e.snapshotFireAndForgetSource(context.Background(), base, now, outer.Variables)
			}
		} else if engineLocked {
			events, err = e.snapshotFireAndForgetSourceLocked(context.Background(), base, now, outer.Variables)
		} else {
			events, err = e.snapshotFireAndForgetSource(context.Background(), base, now, outer.Variables)
		}
	}
	if err != nil {
		return nil
	}
	var runtime *statementRuntime
	if !usingRuntimeSnapshot {
		query := Query{env: e.env, input: definition.source}
		temporary := newStatementRuntime(query)
		temporary.engine = e
		temporary.variables = variablesWithEngine(outer.Variables, e)
		runtime = &temporary
	}
	candidates := make([]subqueryCandidate, 0, len(events))
	groupCandidates := make([]subqueryCandidate, 0, len(events))
	containsWindow := subquerySourceContainsWindow(definition.source)
	var aggregateGroup []Event
	if containsWindow {
		aggregateGroup = append([]Event(nil), events...)
	}
	lastAggregateDeltaHasHistory := false
	for _, event := range events {
		var delta eventDelta
		if usingRuntimeSnapshot {
			delta = eventDelta{newEvents: []Event{event}, history: events}
		} else {
			var insertErr error
			delta, insertErr = runtime.insert(definition.source, event, now)
			if insertErr != nil {
				return nil
			}
		}
		if definition.aggregateProjection && containsWindow && !definition.grouped {
			// A window source owns the final aggregate group. The delta history
			// is updated after every snapshot event, including an empty history
			// after the last event was evicted.
			aggregateGroup = append([]Event(nil), delta.history...)
			lastAggregateDeltaHasHistory = true
		}
		for _, candidate := range delta.newEvents {
			evaluation := EvalContext{
				Engine:               e,
				Event:                candidate,
				JoinEvents:           append([]Event(nil), outer.JoinEvents...),
				OuterEvent:           subqueryEnclosingEvent(outer),
				ContainedParentEvent: containedParentEvent(candidate),
				History:              historyForEvent(delta, candidate),
				Now:                  now,
				Variables:            outer.Variables,
				Parameters:           outer.Parameters,
				aggregateEvaluation:  true,
			}
			if definition.grouped {
				if definition.predicate != nil {
					matched, ok := boolValue(definition.predicate.eval(evaluation))
					if !ok || !matched {
						continue
					}
				}
				groupCandidates = append(groupCandidates, subqueryCandidate{value: Present(candidate), evaluation: evaluation})
				continue
			}
			if definition.aggregateProjection {
				// Aggregate projections are evaluated once below over the final
				// group. Keep the accepted rows here for named-window, table and
				// historical sources, whose base source has no runtime window
				// history of its own.
				if !containsWindow {
					aggregateGroup = append(aggregateGroup, candidate)
				}
				continue
			}
			if definition.predicate != nil {
				matched, ok := boolValue(definition.predicate.eval(evaluation))
				if !ok || !matched {
					continue
				}
			}
			if definition.having != nil {
				// A non-aggregated, non-grouped having filters row-by-row after
				// the where clause (for example "having theString='ID1'").
				matched, ok := boolValue(definition.having.eval(evaluation))
				if !ok || !matched {
					continue
				}
			}
			if definition.projection == nil && len(definition.columns) == 0 {
				candidates = append(candidates, subqueryCandidate{value: Present(candidate), evaluation: evaluation})
				continue
			}
			candidates = append(candidates, subqueryCandidate{value: evaluateSubqueryProjection(definition, evaluation), evaluation: evaluation})
		}
	}
	if definition.grouped {
		return evaluateSubqueryGroups(definition, groupCandidates, outer, e, now)
	}
	if definition.aggregateProjection {
		if containsWindow && !lastAggregateDeltaHasHistory {
			aggregateGroup = nil
		}
		if definition.predicate != nil {
			filtered := make([]Event, 0, len(aggregateGroup))
			for _, event := range aggregateGroup {
				matched, ok := boolValue(definition.predicate.eval(EvalContext{
					Engine:               e,
					Event:                event,
					JoinEvents:           append([]Event(nil), outer.JoinEvents...),
					OuterEvent:           subqueryEnclosingEvent(outer),
					ContainedParentEvent: containedParentEvent(event),
					Group:                aggregateGroup,
					History:              aggregateGroup,
					Now:                  now,
					Variables:            outer.Variables,
					Parameters:           outer.Parameters,
				}))
				if ok && matched {
					filtered = append(filtered, event)
				}
			}
			aggregateGroup = filtered
		}
		evaluation := EvalContext{
			Engine:              e,
			JoinEvents:          append([]Event(nil), outer.JoinEvents...),
			OuterEvent:          subqueryEnclosingEvent(outer),
			Group:               aggregateGroup,
			EverGroup:           aggregateGroup,
			AllGroup:            aggregateGroup,
			AllEverGroup:        aggregateGroup,
			History:             aggregateGroup,
			Now:                 now,
			Variables:           outer.Variables,
			Parameters:          outer.Parameters,
			aggregateEvaluation: true,
		}
		if len(aggregateGroup) > 0 {
			evaluation.Event = aggregateGroup[len(aggregateGroup)-1]
		}
		if definition.having != nil {
			matched, ok := boolValue(definition.having.eval(evaluation))
			if !ok || !matched {
				return nil
			}
		}
		candidates = append(candidates, subqueryCandidate{
			value:      evaluateSubqueryProjection(definition, evaluation),
			evaluation: evaluation,
		})
	}
	if len(definition.orderBy) > 0 {
		sort.SliceStable(candidates, func(left, right int) bool {
			for _, key := range definition.orderBy {
				if key.Expression == nil {
					continue
				}
				leftValue := key.Expression.eval(candidates[left].evaluation)
				rightValue := key.Expression.eval(candidates[right].evaluation)
				less, equal := subqueryOrderLess(leftValue, rightValue)
				if equal {
					continue
				}
				if key.Descending {
					return !less
				}
				return less
			}
			return false
		})
	}
	start := definition.offset
	if start < 0 {
		start = 0
	}
	if start > len(candidates) {
		start = len(candidates)
	}
	end := len(candidates)
	if definition.limitSet && start+definition.limit < end {
		end = start + definition.limit
	}
	if definition.limitSet && definition.limit < 0 {
		end = start
	}
	values := make([]Value, 0, end-start)
	for _, candidate := range candidates[start:end] {
		values = append(values, candidate.value)
	}
	return values
}

// subqueryEnclosingEvent returns the immediate event scope for an inner
// subquery. Ordinary statement evaluation stores that scope in Event. A
// fire-and-forget target mutation has no incoming event, so its target row is
// deliberately carried in OuterEvent instead; falling back here preserves
// correlated OuterField expressions for that path without changing nested
// statement semantics.
func subqueryEnclosingEvent(outer EvalContext) Event {
	if outer.Event.identity != nil {
		return outer.Event
	}
	return outer.OuterEvent
}

// unwrapSubqueryGroupValues collapses grouped subquery result wrappers to
// the projected group value so quantified, IN, EXISTS and scalar subquery
// consumers observe the same raw values as ungrouped aggregate subselects.
// The grouped row-shape consumers (SubqueryGroupBy, SubqueryGroupScalar,
// SubqueryGroupRows) keep the wrapper because they need the group key.
func unwrapSubqueryGroupValues(values []Value) []Value {
	unwrapped := make([]Value, 0, len(values))
	for _, value := range values {
		if group, ok := value.Any().(subqueryGroupValue); ok {
			unwrapped = append(unwrapped, group.value)
			continue
		}
		unwrapped = append(unwrapped, value)
	}
	return unwrapped
}

func evaluateSubqueryGroups(definition *subqueryDefinition, candidates []subqueryCandidate, outer EvalContext, engine *Engine, now time.Time) []Value {
	if definition == nil || definition.groupBy == nil || (definition.projection == nil && len(definition.columns) == 0) {
		return nil
	}
	type groupCandidate struct {
		key         Value
		events      []Event
		evaluations []EvalContext
	}
	groups := make([]groupCandidate, 0)
	for _, candidate := range candidates {
		event, ok := candidate.value.Any().(Event)
		if !ok {
			continue
		}
		key := definition.groupBy.eval(candidate.evaluation)
		groupIndex := -1
		for index := range groups {
			if subqueryValuesEqual(groups[index].key, key) {
				groupIndex = index
				break
			}
		}
		if groupIndex < 0 {
			groups = append(groups, groupCandidate{key: key})
			groupIndex = len(groups) - 1
		}
		groups[groupIndex].events = append(groups[groupIndex].events, event)
		groups[groupIndex].evaluations = append(groups[groupIndex].evaluations, candidate.evaluation)
	}
	values := make([]Value, 0, len(groups))
	for _, group := range groups {
		evaluation := EvalContext{
			Engine:       engine,
			JoinEvents:   append([]Event(nil), outer.JoinEvents...),
			OuterEvent:   subqueryEnclosingEvent(outer),
			Group:        group.events,
			EverGroup:    group.events,
			AllGroup:     group.events,
			AllEverGroup: group.events,
			History:      group.events,
			Now:          now,
			Variables:    outer.Variables,
			Parameters:   outer.Parameters,
		}
		if len(group.events) > 0 {
			evaluation.Event = group.events[len(group.events)-1]
		}
		if definition.having != nil {
			matched, ok := boolValue(definition.having.eval(evaluation))
			if !ok || !matched {
				continue
			}
		}
		if definition.aggregateProjection || definition.groupedRowProjection {
			values = append(values, Present(subqueryGroupValue{key: group.key, value: evaluateSubqueryProjection(definition, evaluation)}))
			continue
		}
		for _, candidateEvaluation := range group.evaluations {
			candidateEvaluation.Group = group.events
			candidateEvaluation.EverGroup = group.events
			candidateEvaluation.AllGroup = group.events
			candidateEvaluation.AllEverGroup = group.events
			if candidateEvaluation.Now.IsZero() {
				candidateEvaluation.Now = now
			}
			values = append(values, Present(subqueryGroupValue{key: group.key, value: evaluateSubqueryProjection(definition, candidateEvaluation)}))
		}
	}
	return values
}

func evaluateSubqueryProjection(definition *subqueryDefinition, evaluation EvalContext) Value {
	if definition == nil {
		return Null()
	}
	if len(definition.columns) == 0 {
		if definition.projection == nil {
			return Null()
		}
		return definition.projection.eval(evaluation)
	}
	row := make(map[string]any, len(definition.columns))
	for _, selection := range definition.columns {
		if selection.Expr == nil {
			return Null()
		}
		value := selection.Expr.eval(evaluation)
		if value.IsPresent() {
			row[selection.Name] = value.Any()
		} else {
			row[selection.Name] = nil
		}
	}
	return Present(row)
}

func subqueryValuesEqual(left, right Value) bool {
	if !left.IsPresent() || !right.IsPresent() {
		return !left.IsPresent() && !right.IsPresent()
	}
	matched, ok := boolValue(EqualValues(left, right))
	return ok && matched
}

func subquerySourceContainsWindow(node *streamNode) bool {
	for node != nil {
		if node.kind == streamWindow {
			return true
		}
		node = node.input
	}
	return false
}

func subqueryOrderLess(left, right Value) (less, equal bool) {
	leftRank := subqueryOrderRank(left)
	rightRank := subqueryOrderRank(right)
	if leftRank != rightRank {
		return leftRank < rightRank, false
	}
	if leftRank != 0 {
		return false, true
	}
	ordering, ok := compareValues(left, right)
	if !ok {
		return false, true
	}
	return ordering < 0, ordering == 0
}

func subqueryOrderRank(value Value) int {
	if value.IsPresent() {
		return 0
	}
	if value.IsNull() {
		return 1
	}
	return 2
}

func variablesWithEngine(variables map[string]Value, engine *Engine) map[string]Value {
	locked := false
	if variables != nil {
		if value, ok := variables[subqueryEngineVariable]; ok && value.IsPresent() {
			if reference, ok := value.Any().(*subqueryEngineRef); ok {
				locked = reference.locked
			}
		}
	}
	return variablesWithEngineLockState(variables, engine, locked)
}

func variablesWithEngineLockState(variables map[string]Value, engine *Engine, locked bool) map[string]Value {
	if engine == nil {
		return variables
	}
	if variables == nil {
		variables = make(map[string]Value)
	}
	variables[subqueryEngineVariable] = Present(&subqueryEngineRef{engine: engine, locked: locked})
	return variables
}
