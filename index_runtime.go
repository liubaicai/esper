package esper

import (
	"context"
	"reflect"
	"strings"
	"time"
)

type indexRangeBound struct {
	value     any
	inclusive bool
}

// indexRangeSpec describes a lexicographic B-tree probe. Prefix values are
// complete equality components; the next component carries one or both
// ordered bounds. Trailing index columns are intentionally not constrained,
// matching the planner's equality-prefix-plus-range contract.
type indexRangeSpec struct {
	prefix        []any
	rangePosition int
	lower         *indexRangeBound
	upper         *indexRangeBound
	empty         bool
}

type indexProbeBounds struct {
	lower *indexRangeBound
	upper *indexRangeBound
}

// maxIndexProbeKeys bounds expansion of an IN predicate before execution
// falls back to the normal snapshot path. The fallback keeps correctness and
// prevents a prepared query from turning one large parameter into an
// unbounded allocation.
const maxIndexProbeKeys = 4096

// indexProbeConstraints is the runtime counterpart of indexPredicate. Values
// are deliberately kept as ordinary Go values because the storage indexes use
// the same encodeKey representation as Table.Lookup and NamedWindow.Lookup.
type indexProbeConstraints map[string][]any

func sourceIndexFilterExpressions(source *streamNode) []Expr {
	result := make([]Expr, 0)
	for current := source; current != nil; current = current.input {
		if current.kind == streamFilter && current.predicate != nil {
			result = append(result, current.predicate)
		}
	}
	return result
}

func compareIndexValueSlices(left, right []Value) (int, bool) {
	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	for index := 0; index < limit; index++ {
		comparison, comparable := compareValues(left[index], right[index])
		if !comparable || comparison != 0 {
			return comparison, comparable
		}
	}
	switch {
	case len(left) < len(right):
		return -1, true
	case len(left) > len(right):
		return 1, true
	default:
		return 0, true
	}
}

func indexRangeEntryMatches(values []Value, query indexRangeSpec) bool {
	if query.empty || query.rangePosition < 0 || query.rangePosition >= len(values) || len(query.prefix) > query.rangePosition {
		return false
	}
	for position, expected := range query.prefix {
		comparison, comparable := compareValues(values[position], Present(expected))
		if !comparable || comparison != 0 {
			return false
		}
	}
	current := values[query.rangePosition]
	if !current.IsPresent() {
		return false
	}
	if query.lower != nil {
		comparison, comparable := compareValues(current, Present(query.lower.value))
		if !comparable || comparison < 0 || (comparison == 0 && !query.lower.inclusive) {
			return false
		}
	}
	if query.upper != nil {
		comparison, comparable := compareValues(current, Present(query.upper.value))
		if !comparable || comparison > 0 || (comparison == 0 && !query.upper.inclusive) {
			return false
		}
	}
	return true
}

// compareIndexRangeCursorEntry compares one ordered index member with the
// prefix and optional range boundary of a cursor. A nil boundary means the
// comparison stops after the equality prefix, which is useful for finding the
// first/last member of a prefix group. Incomparable or missing values force
// the caller to use the established full ordered scan.
func compareIndexRangeCursorEntry(values []Value, query indexRangeSpec, boundary *indexRangeBound) (int, bool) {
	if query.rangePosition < 0 || query.rangePosition >= len(values) || len(query.prefix) > query.rangePosition {
		return 0, false
	}
	for position, expected := range query.prefix {
		comparison, comparable := compareValues(values[position], Present(expected))
		if !comparable {
			return 0, false
		}
		if comparison != 0 {
			return comparison, true
		}
	}
	if boundary == nil {
		return 0, true
	}
	current := values[query.rangePosition]
	if !current.IsPresent() {
		return 0, false
	}
	return compareValues(current, Present(boundary.value))
}

// indexRangeCursorBounds locates the half-open ordered-entry interval that
// can contain a range query. It intentionally uses only the equality prefix
// and range column; trailing index columns remain unconstrained. Returning
// usable=false preserves correctness for legacy indexes containing values
// that cannot participate in a total ordering.
func indexRangeCursorBounds(entryCount int, valueAt func(int) []Value, query indexRangeSpec) (start, end int, usable bool) {
	if entryCount == 0 || query.empty {
		return 0, 0, true
	}
	start = 0
	end = entryCount
	if query.lower != nil {
		left, right := 0, entryCount
		for left < right {
			middle := left + (right-left)/2
			comparison, comparable := compareIndexRangeCursorEntry(valueAt(middle), query, query.lower)
			if !comparable {
				return 0, 0, false
			}
			if comparison < 0 || (comparison == 0 && !query.lower.inclusive) {
				left = middle + 1
			} else {
				right = middle
			}
		}
		start = left
	} else {
		left, right := 0, entryCount
		for left < right {
			middle := left + (right-left)/2
			comparison, comparable := compareIndexRangeCursorEntry(valueAt(middle), query, nil)
			if !comparable {
				return 0, 0, false
			}
			if comparison < 0 {
				left = middle + 1
			} else {
				right = middle
			}
		}
		start = left
	}

	if query.upper != nil {
		left, right := 0, entryCount
		for left < right {
			middle := left + (right-left)/2
			comparison, comparable := compareIndexRangeCursorEntry(valueAt(middle), query, query.upper)
			if !comparable {
				return 0, 0, false
			}
			if comparison < 0 || (comparison == 0 && query.upper.inclusive) {
				left = middle + 1
			} else {
				right = middle
			}
		}
		end = left
	} else {
		left, right := 0, entryCount
		for left < right {
			middle := left + (right-left)/2
			comparison, comparable := compareIndexRangeCursorEntry(valueAt(middle), query, nil)
			if !comparable {
				return 0, 0, false
			}
			if comparison <= 0 {
				left = middle + 1
			} else {
				right = middle
			}
		}
		end = left
	}
	if start > end {
		return 0, 0, true
	}
	return start, end, true
}

// collectIndexRangeCursorPositions performs one binary-bounded scan per
// query and unions matching ordered-entry positions. It returns usable=false
// when a total ordering cannot be proven, allowing Table and Named Window to
// retain their correctness-first full scan fallback.
func collectIndexRangeCursorPositions(ctx context.Context, entryCount int, valueAt func(int) []Value, queries []indexRangeSpec) (map[int]struct{}, bool, error) {
	wanted := make(map[int]struct{})
	for _, query := range queries {
		start, end, usable := indexRangeCursorBounds(entryCount, valueAt, query)
		if !usable {
			return nil, false, nil
		}
		for position := start; position < end; position++ {
			if err := contextErr(ctx); err != nil {
				return nil, true, err
			}
			if indexRangeEntryMatches(valueAt(position), query) {
				wanted[position] = struct{}{}
			}
		}
	}
	return wanted, true, nil
}

func indexProbeExpressions(source *streamNode, onDemand Expr) []Expr {
	if onDemand != nil {
		return []Expr{onDemand}
	}
	return sourceIndexFilterExpressions(source)
}

func indexOperandValue(node *exprNode, ctx EvalContext) (Value, bool) {
	if node == nil {
		return Missing(), false
	}
	switch node.kind {
	case "literal":
		return Present(node.literalValue), true
	case "null":
		return Null(), true
	case "variable":
		if ctx.Variables == nil {
			return Missing(), false
		}
		value, ok := ctx.Variables[node.variableName]
		return value, ok
	case "parameter":
		if ctx.Parameters != nil {
			if value, ok := ctx.Parameters[node.parameterName]; ok {
				return value, true
			}
		}
		if ctx.Variables != nil {
			if bound, ok := ctx.Variables[parameterValuesVariable]; ok && bound.IsPresent() {
				if values, ok := bound.Any().(map[string]Value); ok {
					value, exists := values[node.parameterName]
					return value, exists
				}
			}
		}
		return Missing(), false
	case "outer-field":
		if !ctx.OuterEvent.Schema().valid() {
			return Missing(), false
		}
		value := ctx.OuterEvent.Get(node.fieldName)
		return value, value.IsPresent()
	default:
		// Expressions such as arithmetic, UDF and nested property access need
		// their original Expr closure to evaluate. Their child nodes carry
		// metadata but intentionally do not expose arbitrary closures, so the
		// safe choice is to use the full snapshot path.
		return Missing(), false
	}
}

func appendIndexProbeValues(destination *[]any, node *exprNode, ctx EvalContext, expandCollection bool) bool {
	value, ok := indexOperandValue(node, ctx)
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

func addIndexProbeConstraint(constraints indexProbeConstraints, column string, values []any) {
	if strings.TrimSpace(column) == "" || len(values) == 0 {
		return
	}
	column = strings.TrimSpace(column)
	seen := make(map[string]struct{}, len(constraints[column])+len(values))
	for _, value := range constraints[column] {
		seen[encodeKey([]any{value})] = struct{}{}
	}
	for _, value := range values {
		key := encodeKey([]any{value})
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		constraints[column] = append(constraints[column], value)
	}
}

func collectIndexProbeConstraints(node *exprNode, ctx EvalContext, constraints indexProbeConstraints) {
	if node == nil {
		return
	}
	if node.kind == "and" {
		for _, child := range node.children {
			collectIndexProbeConstraints(child, ctx, constraints)
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
			if appendIndexProbeValues(&values, node.children[1], ctx, false) {
				addIndexProbeConstraint(constraints, leftColumn, values)
			}
			return
		}
		if rightColumn != "" && leftColumn == "" {
			values := make([]any, 0, 1)
			if appendIndexProbeValues(&values, node.children[0], ctx, false) {
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
			if !appendIndexProbeValues(&values, candidate, ctx, node.kind == "in-of") {
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
		if appendIndexProbeValues(&values, node.children[1], ctx, true) {
			addIndexProbeConstraint(constraints, column, values)
		}
	}
}

func markIndexProbeBoundsInvalid(bounds map[string]*indexProbeBounds, column string) *indexProbeBounds {
	current := bounds[column]
	if current == nil {
		current = &indexProbeBounds{}
		bounds[column] = current
	}
	return current
}

func mergeIndexLowerBound(bounds *indexProbeBounds, bound indexRangeBound) {
	if bounds == nil {
		return
	}
	if bounds.lower == nil {
		copy := bound
		bounds.lower = &copy
		return
	}
	comparison, comparable := compareValues(Present(bounds.lower.value), Present(bound.value))
	if !comparable {
		return
	}
	if comparison < 0 {
		copy := bound
		bounds.lower = &copy
		return
	}
	if comparison == 0 {
		bounds.lower.inclusive = bounds.lower.inclusive && bound.inclusive
	}
}

func mergeIndexUpperBound(bounds *indexProbeBounds, bound indexRangeBound) {
	if bounds == nil {
		return
	}
	if bounds.upper == nil {
		copy := bound
		bounds.upper = &copy
		return
	}
	comparison, comparable := compareValues(Present(bounds.upper.value), Present(bound.value))
	if !comparable {
		return
	}
	if comparison > 0 {
		copy := bound
		bounds.upper = &copy
		return
	}
	if comparison == 0 {
		bounds.upper.inclusive = bounds.upper.inclusive && bound.inclusive
	}
}

func addIndexProbeComparison(bounds map[string]*indexProbeBounds, column, kind string, value any, reversed bool) {
	if strings.TrimSpace(column) == "" {
		return
	}
	if reversed {
		switch kind {
		case "gt":
			kind = "lt"
		case "gte":
			kind = "lte"
		case "lt":
			kind = "gt"
		case "lte":
			kind = "gte"
		}
	}
	current := markIndexProbeBoundsInvalid(bounds, column)
	switch kind {
	case "gt":
		mergeIndexLowerBound(current, indexRangeBound{value: value, inclusive: false})
	case "gte":
		mergeIndexLowerBound(current, indexRangeBound{value: value, inclusive: true})
	case "lt":
		mergeIndexUpperBound(current, indexRangeBound{value: value, inclusive: false})
	case "lte":
		mergeIndexUpperBound(current, indexRangeBound{value: value, inclusive: true})
	}
}

func collectIndexProbeRangeConstraints(node *exprNode, ctx EvalContext, exact indexProbeConstraints, ranges map[string]*indexProbeBounds) {
	if node == nil {
		return
	}
	if node.kind == "and" {
		for _, child := range node.children {
			collectIndexProbeRangeConstraints(child, ctx, exact, ranges)
		}
		return
	}
	switch node.kind {
	case "eq", "equal-of", "is", "in", "in-of", "in-slice":
		collectIndexProbeConstraints(node, ctx, exact)
	case "between", "between-of":
		if len(node.children) != 3 {
			return
		}
		column := fieldColumn(node.children[0])
		if column == "" {
			return
		}
		lower := make([]any, 0, 1)
		upper := make([]any, 0, 1)
		if !appendIndexProbeValues(&lower, node.children[1], ctx, false) || !appendIndexProbeValues(&upper, node.children[2], ctx, false) {
			markIndexProbeBoundsInvalid(ranges, column)
			return
		}
		current := markIndexProbeBoundsInvalid(ranges, column)
		mergeIndexLowerBound(current, indexRangeBound{value: lower[0], inclusive: true})
		mergeIndexUpperBound(current, indexRangeBound{value: upper[0], inclusive: true})
	case "gt", "gte", "lt", "lte", "greater-of", "greater-equal-of", "less-of", "less-equal-of":
		if len(node.children) < 2 {
			return
		}
		kind := node.kind
		switch kind {
		case "greater-of":
			kind = "gt"
		case "greater-equal-of":
			kind = "gte"
		case "less-of":
			kind = "lt"
		case "less-equal-of":
			kind = "lte"
		}
		leftColumn := fieldColumn(node.children[0])
		rightColumn := fieldColumn(node.children[1])
		if leftColumn != "" && rightColumn == "" {
			value := make([]any, 0, 1)
			if appendIndexProbeValues(&value, node.children[1], ctx, false) {
				addIndexProbeComparison(ranges, leftColumn, kind, value[0], false)
			} else {
				markIndexProbeBoundsInvalid(ranges, leftColumn)
			}
			return
		}
		if rightColumn != "" && leftColumn == "" {
			value := make([]any, 0, 1)
			if appendIndexProbeValues(&value, node.children[0], ctx, false) {
				addIndexProbeComparison(ranges, rightColumn, kind, value[0], true)
			} else {
				markIndexProbeBoundsInvalid(ranges, rightColumn)
			}
		}
	}
}

func normalizeIndexProbeValue(value any, fieldType reflect.Type) (any, bool) {
	if fieldType == nil || fieldType == typeOf[any]() || value == nil {
		return value, true
	}
	coerced, err := coerceTableValue(value, fieldType)
	if err != nil {
		return nil, false
	}
	return coerced.Any(), coerced.IsPresent()
}

func schemaFieldType(schema Schema, column string) reflect.Type {
	field, exists := schema.Field(column)
	if !exists {
		return nil
	}
	return field.Type
}

func normalizeIndexRangeBound(bound *indexRangeBound, fieldType reflect.Type) (*indexRangeBound, bool) {
	if bound == nil {
		return nil, true
	}
	value, ok := normalizeIndexProbeValue(bound.value, fieldType)
	if !ok || value == nil || isNilReflectValue(reflect.ValueOf(value)) {
		return nil, false
	}
	return &indexRangeBound{value: value, inclusive: bound.inclusive}, true
}

func (e *Engine) indexProbeRangeSpec(source *streamNode, selection IndexSelection, expressions []Expr, now time.Time, variables map[string]Value) (indexRangeSpec, bool) {
	return e.indexProbeRangeSpecWithOuter(source, selection, expressions, now, variables, Event{})
}

// indexProbeRangeSpecWithOuter is the range-probe counterpart used by
// correlated subqueries.  OuterField is a safe probe operand because the
// enclosing event is fixed for the current subquery evaluation; all other
// dynamic expression forms retain the ordinary snapshot fallback.
func (e *Engine) indexProbeRangeSpecWithOuter(source *streamNode, selection IndexSelection, expressions []Expr, now time.Time, variables map[string]Value, outer Event) (indexRangeSpec, bool) {
	if e == nil || selection.Access != IndexAccessRange || (selection.Backing != IndexBackingBTree && selection.Backing != IndexBackingUniqueBTree) || len(selection.Columns) == 0 || len(selection.MatchedColumns) == 0 {
		return indexRangeSpec{}, false
	}
	base, err := sourceNode(source)
	if err != nil {
		return indexRangeSpec{}, false
	}
	schema, err := e.env.sourceSchema(base)
	if err != nil {
		return indexRangeSpec{}, false
	}
	rangePosition := len(selection.MatchedColumns) - 1
	if rangePosition < 0 || rangePosition >= len(selection.Columns) {
		return indexRangeSpec{}, false
	}
	ctx := EvalContext{Now: now, Variables: variables, Parameters: parameterValuesFromVariables(variables), OuterEvent: outer}
	exact := make(indexProbeConstraints)
	ranges := make(map[string]*indexProbeBounds)
	for _, expression := range expressions {
		if expression != nil {
			collectIndexProbeRangeConstraints(expression.node(), ctx, exact, ranges)
		}
	}
	query := indexRangeSpec{rangePosition: rangePosition, prefix: make([]any, 0, rangePosition)}
	for position := 0; position < rangePosition; position++ {
		column := selection.Columns[position]
		values, exists := exact[column]
		if !exists || len(values) != 1 {
			return indexRangeSpec{}, false
		}
		field, fieldExists := schema.Field(column)
		fieldType := reflect.Type(nil)
		if fieldExists {
			fieldType = field.Type
		}
		value, ok := normalizeIndexProbeValue(values[0], fieldType)
		if !ok || value == nil || isNilReflectValue(reflect.ValueOf(value)) {
			return indexRangeSpec{}, false
		}
		query.prefix = append(query.prefix, value)
	}
	rangeColumn := selection.Columns[rangePosition]
	bounds, exists := ranges[rangeColumn]
	if !exists || bounds == nil || (bounds.lower == nil && bounds.upper == nil) {
		return indexRangeSpec{}, false
	}
	if bounds.lower == nil && bounds.upper == nil {
		return indexRangeSpec{}, false
	}
	if bounds.lower != nil {
		field, fieldExists := schema.Field(rangeColumn)
		fieldType := reflect.Type(nil)
		if fieldExists {
			fieldType = field.Type
		}
		var ok bool
		query.lower, ok = normalizeIndexRangeBound(bounds.lower, fieldType)
		if !ok {
			return indexRangeSpec{}, false
		}
	}
	if bounds.upper != nil {
		field, fieldExists := schema.Field(rangeColumn)
		fieldType := reflect.Type(nil)
		if fieldExists {
			fieldType = field.Type
		}
		var ok bool
		query.upper, ok = normalizeIndexRangeBound(bounds.upper, fieldType)
		if !ok {
			return indexRangeSpec{}, false
		}
	}
	if query.lower != nil && query.upper != nil {
		comparison, comparable := compareValues(Present(query.lower.value), Present(query.upper.value))
		if !comparable {
			return indexRangeSpec{}, false
		}
		if comparison > 0 {
			query.lower, query.upper = query.upper, query.lower
		} else if comparison == 0 && (!query.lower.inclusive || !query.upper.inclusive) {
			query.empty = true
		}
	}
	return query, true
}

func (e *Engine) indexProbeKeys(source *streamNode, selection IndexSelection, expressions []Expr, now time.Time, variables map[string]Value) ([][]any, bool) {
	if e == nil || len(selection.Columns) == 0 || len(selection.MatchedColumns) != len(selection.Columns) {
		return nil, false
	}
	if selection.Access != IndexAccessEquality && selection.Access != IndexAccessIn {
		return nil, false
	}
	base, err := sourceNode(source)
	if err != nil {
		return nil, false
	}
	schema, err := e.env.sourceSchema(base)
	if err != nil {
		return nil, false
	}
	constraints := make(indexProbeConstraints)
	ctx := EvalContext{Now: now, Variables: variables, Parameters: parameterValuesFromVariables(variables)}
	for _, expression := range expressions {
		if expression != nil {
			collectIndexProbeConstraints(expression.node(), ctx, constraints)
		}
	}
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
			if !ok {
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

// joinIndexConstraint is one equality value for a target-side index column.
// source identifies the tuple side used as the current Event when value is a
// plain Field expression; JoinField expressions use their own source metadata.
// A negative source means that value is independent of the join tuple (for
// example a literal, variable or prepared parameter).
type joinIndexConstraint struct {
	value  Expr
	node   *exprNode
	source int
}

type joinIndexRangeBound struct {
	value     Expr
	node      *exprNode
	source    int
	inclusive bool
}

type joinIndexRangeConstraint struct {
	lower []joinIndexRangeBound
	upper []joinIndexRangeBound
}

func expressionReferencesJoinSource(node *exprNode, source int) bool {
	if node == nil {
		return false
	}
	if (node.kind == "join-field" || node.kind == "join-event") && node.joinSource == source {
		return true
	}
	for _, child := range node.children {
		if expressionReferencesJoinSource(child, source) {
			return true
		}
	}
	return false
}

func joinIndexTargetColumn(expression Expr, targetSource, declaredSource int) (string, bool) {
	if expression == nil || expression.node() == nil {
		return "", false
	}
	return joinIndexTargetNode(expression.node(), targetSource, declaredSource)
}

func joinIndexTargetNode(node *exprNode, targetSource, declaredSource int) (string, bool) {
	if node == nil {
		return "", false
	}
	if column := fieldColumn(node); column != "" {
		return column, declaredSource == targetSource
	}
	joinSource, column, ok := joinFieldColumn(node)
	return column, ok && joinSource == targetSource
}

func safeJoinIndexProbeValueNode(node *exprNode) bool {
	if node == nil {
		return false
	}
	switch node.kind {
	case "literal", "null", "field", "join-field", "variable", "parameter":
		return true
	default:
		// Do not evaluate UDFs, arithmetic, property chains or other dynamic
		// expressions once per loaded tuple and use the result to prune the
		// target side. The ordinary join evaluator remains the source of truth.
		return false
	}
}

func addJoinIndexConstraint(constraints map[string]joinIndexConstraint, expression Expr, value Expr, targetSource, declaredTargetSource, valueSource int) bool {
	column, ok := joinIndexTargetColumn(expression, targetSource, declaredTargetSource)
	if !ok || value == nil || value.node() == nil || !safeJoinIndexProbeValueNode(value.node()) || expressionReferencesJoinSource(value.node(), targetSource) {
		return false
	}
	if _, exists := constraints[column]; !exists {
		constraints[column] = joinIndexConstraint{value: value, source: valueSource}
	}
	return true
}

// joinIndexChainedOuterShapeAllowed identifies the left-deep chain subset for
// which a target-side candidate lookup cannot remove a preserved row. Every
// edge must introduce an inner or left-optional source, and its conditions
// must use only the newly introduced source and the immediately preceding
// source. The adjacency restriction matters when an earlier optional side is
// empty: a later edge cannot recover a match through a missing intermediate
// tuple, so an empty candidate set remains semantically safe.
func joinIndexChainedOuterShapeAllowed(definition *joinDefinition) bool {
	if definition == nil || len(definition.edges) == 0 || joinDefinitionHasUnidirectional(definition) {
		return false
	}
	sources := joinDefinitionSources(definition)
	if len(sources) != len(definition.edges)+1 {
		return false
	}
	for edgeIndex, edge := range definition.edges {
		if edge.kind != JoinInner && edge.kind != JoinLeftOuter || len(edge.conditions) == 0 {
			return false
		}
		allowed := map[int]struct{}{edgeIndex: {}, edgeIndex + 1: {}}
		for _, condition := range edge.conditions {
			if !joinConditionUsesOnlySources(condition, allowed) {
				return false
			}
		}
	}
	return true
}

func joinConditionUsesOnlySources(condition JoinCondition, allowed map[int]struct{}) bool {
	if len(condition.all) > 0 && len(condition.any) > 0 {
		return false
	}
	if len(condition.any) > 0 {
		// AnyJoin is an OR. A candidate from one branch can exclude a row
		// matched by another branch, so it is never a safe pruning basis.
		return false
	}
	if len(condition.all) > 0 {
		for _, child := range condition.all {
			if !joinConditionUsesOnlySources(child, allowed) {
				return false
			}
		}
		return true
	}
	if condition.Left == nil || condition.Right == nil {
		return false
	}
	leftSource, rightSource := joinConditionSources(condition)
	_, leftAllowed := allowed[leftSource]
	_, rightAllowed := allowed[rightSource]
	return leftAllowed && rightAllowed && leftSource != rightSource
}

// joinIndexCandidateAllowed limits physical pruning to join shapes for which
// omitting non-matching rows from the target side cannot remove a preserved
// outer row. For a two-stream left outer join the right side is optional; for
// a right outer join the left side is optional. A left-deep chain can use the
// same path only for the adjacency-constrained inner/left-outer subset above.
// Full outer joins, right-preserving chain edges and unidirectional joins use
// the complete snapshot path.
func joinIndexCandidateAllowed(definition *joinDefinition, targetSource int) bool {
	if definition == nil || joinDefinitionHasUnidirectional(definition) {
		return false
	}
	sources := joinDefinitionSources(definition)
	if len(definition.edges) > 0 {
		allInner := true
		for _, edge := range definition.edges {
			if edge.kind != JoinInner {
				allInner = false
				break
			}
		}
		if allInner {
			return true
		}
		if !joinIndexChainedOuterShapeAllowed(definition) || targetSource <= 0 || targetSource >= len(sources) {
			return false
		}
		return definition.edges[targetSource-1].kind == JoinInner || definition.edges[targetSource-1].kind == JoinLeftOuter
	}
	switch definition.kind {
	case JoinInner:
		return true
	case JoinLeftOuter:
		return len(sources) == 2 && targetSource == 1
	case JoinRightOuter:
		return len(sources) == 2 && targetSource == 0
	default:
		return false
	}
}

func addJoinIndexNodeConstraint(constraints map[string]joinIndexConstraint, target, value *exprNode, targetSource, declaredTargetSource, valueSource int) bool {
	column, ok := joinIndexTargetNode(target, targetSource, declaredTargetSource)
	if !ok || value == nil || expressionReferencesJoinSource(value, targetSource) {
		return false
	}
	if _, exists := constraints[column]; !exists {
		constraints[column] = joinIndexConstraint{node: value, source: valueSource}
	}
	return true
}

// collectJoinConditionIndexConstraints extracts only safe equality leaves
// from a JoinCondition. AnyJoin is deliberately rejected: using one branch's
// key as the sole candidate can miss rows matching another branch. Non-equal
// leaves are left to the final Join predicate and do not make an equality
// candidate unsafe.
func collectJoinConditionIndexConstraints(condition JoinCondition, targetSource int, constraints map[string]joinIndexConstraint) bool {
	if len(condition.all) > 0 {
		for _, child := range condition.all {
			if !collectJoinConditionIndexConstraints(child, targetSource, constraints) {
				return false
			}
		}
		return true
	}
	if len(condition.any) > 0 {
		return false
	}
	if condition.Comparison != JoinEqual || condition.Left == nil || condition.Right == nil {
		return true
	}
	leftSource, rightSource := joinConditionSources(condition)
	switch {
	case leftSource == targetSource && rightSource != targetSource:
		return addJoinIndexConstraint(constraints, condition.Left, condition.Right, targetSource, leftSource, rightSource)
	case rightSource == targetSource && leftSource != targetSource:
		return addJoinIndexConstraint(constraints, condition.Right, condition.Left, targetSource, rightSource, leftSource)
	case leftSource == targetSource || rightSource == targetSource:
		return false
	default:
		return true
	}
}

func invertJoinRangeComparison(comparison JoinComparison) (JoinComparison, bool) {
	switch comparison {
	case JoinLess:
		return JoinGreater, true
	case JoinLessOrEqual:
		return JoinGreaterOrEqual, true
	case JoinGreater:
		return JoinLess, true
	case JoinGreaterOrEqual:
		return JoinLessOrEqual, true
	default:
		return JoinEqual, false
	}
}

func addJoinIndexRangeConstraint(constraints map[string]joinIndexRangeConstraint, expression Expr, value Expr, targetSource, declaredTargetSource, valueSource int, comparison JoinComparison) bool {
	column, ok := joinIndexTargetColumn(expression, targetSource, declaredTargetSource)
	if !ok || value == nil || value.node() == nil || !safeJoinIndexProbeValueNode(value.node()) || expressionReferencesJoinSource(value.node(), targetSource) {
		return false
	}
	bound := joinIndexRangeBound{value: value, source: valueSource, inclusive: comparison == JoinGreaterOrEqual || comparison == JoinLessOrEqual}
	if comparison == JoinGreater || comparison == JoinGreaterOrEqual {
		current := constraints[column]
		current.lower = append(current.lower, bound)
		constraints[column] = current
		return true
	}
	if comparison == JoinLess || comparison == JoinLessOrEqual {
		current := constraints[column]
		current.upper = append(current.upper, bound)
		constraints[column] = current
		return true
	}
	return false
}

// collectJoinConditionRangeIndexConstraints extracts range leaves whose
// target side is one indexed source and whose bound is independent of that
// target. Unsupported OR/complex target expressions force snapshot fallback.
func collectJoinConditionRangeIndexConstraints(condition JoinCondition, targetSource int, constraints map[string]joinIndexRangeConstraint) bool {
	if len(condition.all) > 0 {
		for _, child := range condition.all {
			if !collectJoinConditionRangeIndexConstraints(child, targetSource, constraints) {
				return false
			}
		}
		return true
	}
	if len(condition.any) > 0 {
		return false
	}
	if condition.Left == nil || condition.Right == nil {
		return true
	}
	comparison, rangeComparison := invertJoinRangeComparison(condition.Comparison)
	if !rangeComparison {
		return true
	}
	leftSource, rightSource := joinConditionSources(condition)
	switch {
	case leftSource == targetSource && rightSource != targetSource:
		return addJoinIndexRangeConstraint(constraints, condition.Left, condition.Right, targetSource, leftSource, rightSource, condition.Comparison)
	case rightSource == targetSource && leftSource != targetSource:
		return addJoinIndexRangeConstraint(constraints, condition.Right, condition.Left, targetSource, rightSource, leftSource, comparison)
	case leftSource == targetSource || rightSource == targetSource:
		return false
	default:
		return true
	}
}

// collectJoinWhereIndexConstraints handles the analyzable JoinField form of
// a post-join Where. Ordinary non-index predicates are intentionally ignored
// because the runtime evaluates them after candidate retrieval; OR is unsafe
// and therefore forces snapshot fallback.
func collectJoinWhereIndexConstraints(node *exprNode, targetSource int, constraints map[string]joinIndexConstraint) bool {
	if node == nil {
		return true
	}
	if node.kind == "and" {
		for _, child := range node.children {
			if !collectJoinWhereIndexConstraints(child, targetSource, constraints) {
				return false
			}
		}
		return true
	}
	if node.kind == "or" {
		return false
	}
	if node.kind != "eq" && node.kind != "equal-of" && node.kind != "is" || len(node.children) < 2 {
		return true
	}
	leftSource, _, leftOK := joinFieldColumn(node.children[0])
	rightSource, _, rightOK := joinFieldColumn(node.children[1])
	if leftOK && leftSource == targetSource && (!rightOK || rightSource != targetSource) {
		if rightOK {
			return addJoinIndexNodeConstraint(constraints, node.children[0], node.children[1], targetSource, leftSource, rightSource)
		}
		return addJoinIndexNodeConstraint(constraints, node.children[0], node.children[1], targetSource, leftSource, -1)
	}
	if rightOK && rightSource == targetSource && (!leftOK || leftSource != targetSource) {
		if leftOK {
			return addJoinIndexNodeConstraint(constraints, node.children[1], node.children[0], targetSource, rightSource, leftSource)
		}
		return addJoinIndexNodeConstraint(constraints, node.children[1], node.children[0], targetSource, rightSource, -1)
	}
	return true
}

func joinWhereComparisonKind(kind string) (JoinComparison, bool) {
	switch kind {
	case "gt", "exact-gt", "greater-of":
		return JoinGreater, true
	case "gte", "exact-gte", "greater-equal-of":
		return JoinGreaterOrEqual, true
	case "lt", "exact-lt", "less-of":
		return JoinLess, true
	case "lte", "exact-lte", "less-equal-of":
		return JoinLessOrEqual, true
	default:
		return JoinEqual, false
	}
}

func collectJoinWhereRangeIndexConstraints(node *exprNode, targetSource int, constraints map[string]joinIndexRangeConstraint) bool {
	if node == nil {
		return true
	}
	if node.kind == "and" {
		for _, child := range node.children {
			if !collectJoinWhereRangeIndexConstraints(child, targetSource, constraints) {
				return false
			}
		}
		return true
	}
	if node.kind == "or" {
		return false
	}
	if comparison, ok := joinWhereComparisonKind(node.kind); ok {
		if len(node.children) < 2 {
			return true
		}
		leftSource, _, leftOK := joinFieldColumn(node.children[0])
		rightSource, _, rightOK := joinFieldColumn(node.children[1])
		switch {
		case leftOK && leftSource == targetSource && (!rightOK || rightSource != targetSource):
			return addJoinIndexRangeNodeConstraint(constraints, node.children[0], node.children[1], targetSource, leftSource, -1, comparison, rightOK, rightSource)
		case rightOK && rightSource == targetSource && (!leftOK || leftSource != targetSource):
			inverted, invertedOK := invertJoinRangeComparison(comparison)
			if !invertedOK {
				return false
			}
			return addJoinIndexRangeNodeConstraint(constraints, node.children[1], node.children[0], targetSource, rightSource, -1, inverted, leftOK, leftSource)
		case leftOK && leftSource == targetSource || rightOK && rightSource == targetSource:
			return false
		default:
			return true
		}
	}
	if node.kind != "between" && node.kind != "between-of" || len(node.children) != 3 {
		return true
	}
	targetSourceIndex, column, targetOK := joinFieldColumn(node.children[0])
	if !targetOK || targetSourceIndex != targetSource {
		return true
	}
	if !safeJoinIndexProbeValueNode(node.children[1]) || !safeJoinIndexProbeValueNode(node.children[2]) || expressionReferencesJoinSource(node.children[1], targetSource) || expressionReferencesJoinSource(node.children[2], targetSource) {
		return false
	}
	current := constraints[column]
	current.lower = append(current.lower, joinIndexRangeBound{node: node.children[1], source: -1, inclusive: true})
	current.upper = append(current.upper, joinIndexRangeBound{node: node.children[2], source: -1, inclusive: true})
	constraints[column] = current
	return true
}

func addJoinIndexRangeNodeConstraint(constraints map[string]joinIndexRangeConstraint, target, value *exprNode, targetSource, declaredTargetSource, valueSource int, comparison JoinComparison, valueIsJoinField bool, actualValueSource int) bool {
	column, ok := joinIndexTargetNode(target, targetSource, declaredTargetSource)
	if !ok || value == nil || !safeJoinIndexProbeValueNode(value) || expressionReferencesJoinSource(value, targetSource) {
		return false
	}
	if valueIsJoinField {
		valueSource = actualValueSource
	}
	bound := joinIndexRangeBound{node: value, source: valueSource, inclusive: comparison == JoinGreaterOrEqual || comparison == JoinLessOrEqual}
	current := constraints[column]
	if comparison == JoinGreater || comparison == JoinGreaterOrEqual {
		current.lower = append(current.lower, bound)
	} else if comparison == JoinLess || comparison == JoinLessOrEqual {
		current.upper = append(current.upper, bound)
	} else {
		return false
	}
	constraints[column] = current
	return true
}

func joinIndexProbeTuples(sides [][]storedEvent, loaded []bool, targetSource int) ([][]Event, bool) {
	if len(sides) == 0 || targetSource < 0 || targetSource >= len(sides) {
		return nil, false
	}
	for source := range sides {
		if source != targetSource && source < len(loaded) && loaded[source] && len(sides[source]) == 0 {
			// A loaded empty source makes the inner join empty. This is a
			// usable result, not a reason to scan the target source.
			return nil, true
		}
	}
	current := make([]Event, len(sides))
	result := make([][]Event, 0, 1)
	var visit func(int) bool
	visit = func(source int) bool {
		if source == len(sides) {
			if len(result) >= maxIndexProbeKeys {
				return false
			}
			result = append(result, append([]Event(nil), current...))
			return true
		}
		if source == targetSource || source >= len(loaded) || !loaded[source] {
			return visit(source + 1)
		}
		if len(sides[source]) == 0 {
			return false
		}
		for _, stored := range sides[source] {
			current[source] = stored.event
			if !visit(source + 1) {
				return false
			}
		}
		current[source] = Event{}
		return true
	}
	if !visit(0) {
		return nil, false
	}
	return result, true
}

func evaluateJoinIndexConstraint(constraint joinIndexConstraint, tuple []Event, now time.Time, variables map[string]Value) (any, bool) {
	node := constraint.node
	if constraint.value != nil {
		node = constraint.value.node()
	}
	if node == nil {
		return nil, false
	}
	current := Event{}
	if constraint.source >= 0 {
		if constraint.source >= len(tuple) || !tuple[constraint.source].Schema().valid() {
			return nil, false
		}
		current = tuple[constraint.source]
	}
	if constraint.value == nil {
		return evaluateSimpleJoinIndexNode(node, current, tuple, now, variables)
	}
	value := constraint.value.eval(EvalContext{
		Event: current, JoinEvents: tuple, OuterEvent: current, Now: now,
		Variables: variables, Parameters: parameterValuesFromVariables(variables),
	})
	if !value.IsPresent() {
		return nil, false
	}
	return value.Any(), true
}

func evaluateSimpleJoinIndexNode(node *exprNode, current Event, tuple []Event, now time.Time, variables map[string]Value) (any, bool) {
	if node == nil {
		return nil, false
	}
	switch node.kind {
	case "literal":
		return node.literalValue, true
	case "null":
		return nil, false
	case "field":
		value := current.Get(node.fieldName)
		return value.Any(), value.IsPresent()
	case "join-field":
		if node.joinSource < 0 || node.joinSource >= len(tuple) || !tuple[node.joinSource].Schema().valid() {
			return nil, false
		}
		value := tuple[node.joinSource].Get(node.fieldName)
		return value.Any(), value.IsPresent()
	case "variable":
		value, ok := variables[node.variableName]
		return value.Any(), ok && value.IsPresent()
	case "parameter":
		if parameters := parameterValuesFromVariables(variables); parameters != nil {
			if value, ok := parameters[node.parameterName]; ok {
				return value.Any(), value.IsPresent()
			}
		}
		return nil, false
	default:
		return nil, false
	}
}

// joinIndexProbeKeys extracts complete equality keys for one source from the
// already loaded join sides. It returns usable=false for any shape that could
// produce an incomplete candidate set; callers must then use the ordinary
// snapshot path. A usable empty key set is a valid empty result when a loaded
// source has no rows.
func (e *Engine) joinIndexProbeKeys(source *streamNode, selection IndexSelection, targetSource int, definition *joinDefinition, joinWhere Expr, sides [][]storedEvent, loaded []bool, filterExpressions []Expr, now time.Time, variables map[string]Value) ([][]any, bool) {
	if e == nil || source == nil || selection.Access != IndexAccessEquality || len(selection.Columns) == 0 || len(selection.MatchedColumns) != len(selection.Columns) || (strings.HasPrefix(selection.IndexName, "<") && selection.IndexName != "<primary-key>") {
		return nil, false
	}
	if !joinIndexCandidateAllowed(definition, targetSource) {
		return nil, false
	}
	base, err := sourceNode(source)
	if err != nil || (base.kind != streamNamedWindow && base.kind != streamTable) {
		return nil, false
	}
	schema, err := e.env.sourceSchema(base)
	if err != nil {
		return nil, false
	}
	ctx := EvalContext{Now: now, Variables: variables, Parameters: parameterValuesFromVariables(variables)}
	fixed := make(indexProbeConstraints)
	for _, expression := range filterExpressions {
		if expression != nil {
			collectIndexProbeConstraints(expression.node(), ctx, fixed)
		}
	}
	constraints := make(map[string]joinIndexConstraint)
	if definition != nil {
		for _, condition := range joinDefinitionConditions(definition) {
			if !collectJoinConditionIndexConstraints(condition, targetSource, constraints) {
				return nil, false
			}
		}
	}
	if joinWhere != nil && !collectJoinWhereIndexConstraints(joinWhere.node(), targetSource, constraints) {
		return nil, false
	}
	for _, column := range selection.Columns {
		if values := fixed[column]; len(values) > 1 {
			return nil, false
		}
		if len(fixed[column]) == 0 {
			if _, exists := constraints[column]; !exists {
				return nil, false
			}
		}
	}
	tupleProbes, probesUsable := joinIndexProbeTuples(sides, loaded, targetSource)
	if !probesUsable {
		return nil, false
	}
	if tupleProbes == nil {
		return nil, true
	}
	keys := make([][]any, 0, len(tupleProbes))
	seen := make(map[string]struct{}, len(tupleProbes))
	for _, tuple := range tupleProbes {
		key := make([]any, 0, len(selection.Columns))
		for _, column := range selection.Columns {
			var value any
			if fixedValues := fixed[column]; len(fixedValues) == 1 {
				value = fixedValues[0]
			} else {
				var ok bool
				value, ok = evaluateJoinIndexConstraint(constraints[column], tuple, now, variables)
				if !ok {
					return nil, false
				}
			}
			field, exists := schema.Field(column)
			fieldType := reflect.Type(nil)
			if exists {
				fieldType = field.Type
			}
			normalized, ok := normalizeIndexProbeValue(value, fieldType)
			if !ok || normalized == nil || isNilReflectValue(reflect.ValueOf(normalized)) {
				return nil, false
			}
			key = append(key, normalized)
		}
		encoded := encodeKey(key)
		if _, exists := seen[encoded]; exists {
			continue
		}
		seen[encoded] = struct{}{}
		keys = append(keys, key)
		if len(keys) > maxIndexProbeKeys {
			return nil, false
		}
	}
	return keys, true
}

func evaluateJoinIndexRangeBound(bound joinIndexRangeBound, tuple []Event, now time.Time, variables map[string]Value) (*indexRangeBound, bool) {
	value, ok := evaluateJoinIndexConstraint(joinIndexConstraint{value: bound.value, node: bound.node, source: bound.source}, tuple, now, variables)
	if !ok {
		return nil, false
	}
	return &indexRangeBound{value: value, inclusive: bound.inclusive}, true
}

func mergeJoinIndexRangeBound(bounds *indexProbeBounds, bound *indexRangeBound, lower bool) {
	if bounds == nil || bound == nil {
		return
	}
	if lower {
		mergeIndexLowerBound(bounds, *bound)
		return
	}
	mergeIndexUpperBound(bounds, *bound)
}

func (e *Engine) joinIndexProbeRangeSpecs(source *streamNode, selection IndexSelection, targetSource int, definition *joinDefinition, joinWhere Expr, sides [][]storedEvent, loaded []bool, filterExpressions []Expr, now time.Time, variables map[string]Value) ([]indexRangeSpec, bool) {
	if e == nil || source == nil || selection.Access != IndexAccessRange || (selection.Backing != IndexBackingBTree && selection.Backing != IndexBackingUniqueBTree) || len(selection.Columns) == 0 || len(selection.MatchedColumns) == 0 || len(selection.MatchedColumns) > len(selection.Columns) || strings.HasPrefix(selection.IndexName, "<") {
		return nil, false
	}
	if !joinIndexCandidateAllowed(definition, targetSource) {
		return nil, false
	}
	base, err := sourceNode(source)
	if err != nil || (base.kind != streamNamedWindow && base.kind != streamTable) {
		return nil, false
	}
	schema, err := e.env.sourceSchema(base)
	if err != nil {
		return nil, false
	}
	rangePosition := len(selection.MatchedColumns) - 1
	if rangePosition < 0 || rangePosition >= len(selection.Columns) {
		return nil, false
	}
	ctx := EvalContext{Now: now, Variables: variables, Parameters: parameterValuesFromVariables(variables)}
	fixed := make(indexProbeConstraints)
	filterRanges := make(map[string]*indexProbeBounds)
	for _, expression := range filterExpressions {
		if expression != nil {
			collectIndexProbeRangeConstraints(expression.node(), ctx, fixed, filterRanges)
		}
	}
	exact := make(map[string]joinIndexConstraint)
	joinRanges := make(map[string]joinIndexRangeConstraint)
	for _, condition := range joinDefinitionConditions(definition) {
		if !collectJoinConditionIndexConstraints(condition, targetSource, exact) || !collectJoinConditionRangeIndexConstraints(condition, targetSource, joinRanges) {
			return nil, false
		}
	}
	if joinWhere != nil {
		if !collectJoinWhereIndexConstraints(joinWhere.node(), targetSource, exact) || !collectJoinWhereRangeIndexConstraints(joinWhere.node(), targetSource, joinRanges) {
			return nil, false
		}
	}
	for _, column := range selection.Columns[:rangePosition] {
		if len(fixed[column]) > 1 {
			return nil, false
		}
		if len(fixed[column]) == 0 {
			if _, exists := exact[column]; !exists {
				return nil, false
			}
		}
	}
	rangeColumn := selection.Columns[rangePosition]
	if len(fixed[rangeColumn]) > 1 {
		return nil, false
	}
	tupleProbes, probesUsable := joinIndexProbeTuples(sides, loaded, targetSource)
	if !probesUsable {
		return nil, false
	}
	if tupleProbes == nil {
		return nil, true
	}
	queries := make([]indexRangeSpec, 0, len(tupleProbes))
	seen := make(map[string]struct{}, len(tupleProbes))
	for _, tuple := range tupleProbes {
		query := indexRangeSpec{rangePosition: rangePosition, prefix: make([]any, 0, rangePosition)}
		for _, column := range selection.Columns[:rangePosition] {
			var value any
			if values := fixed[column]; len(values) == 1 {
				value = values[0]
			} else {
				var ok bool
				value, ok = evaluateJoinIndexConstraint(exact[column], tuple, now, variables)
				if !ok {
					return nil, false
				}
			}
			field, exists := schema.Field(column)
			fieldType := reflect.Type(nil)
			if exists {
				fieldType = field.Type
			}
			normalized, ok := normalizeIndexProbeValue(value, fieldType)
			if !ok || normalized == nil || isNilReflectValue(reflect.ValueOf(normalized)) {
				return nil, false
			}
			query.prefix = append(query.prefix, normalized)
		}

		rangeBounds := &indexProbeBounds{}
		if bounds := filterRanges[rangeColumn]; bounds != nil {
			if bounds.lower != nil {
				normalized, ok := normalizeIndexRangeBound(bounds.lower, schemaFieldType(schema, rangeColumn))
				if !ok {
					return nil, false
				}
				mergeJoinIndexRangeBound(rangeBounds, normalized, true)
			}
			if bounds.upper != nil {
				normalized, ok := normalizeIndexRangeBound(bounds.upper, schemaFieldType(schema, rangeColumn))
				if !ok {
					return nil, false
				}
				mergeJoinIndexRangeBound(rangeBounds, normalized, false)
			}
		}
		if values := fixed[rangeColumn]; len(values) == 1 {
			value, ok := normalizeIndexProbeValue(values[0], schemaFieldType(schema, rangeColumn))
			if !ok || value == nil || isNilReflectValue(reflect.ValueOf(value)) {
				return nil, false
			}
			point := &indexRangeBound{value: value, inclusive: true}
			mergeJoinIndexRangeBound(rangeBounds, point, true)
			mergeJoinIndexRangeBound(rangeBounds, point, false)
		} else if constraint, exists := exact[rangeColumn]; exists {
			value, ok := evaluateJoinIndexConstraint(constraint, tuple, now, variables)
			if !ok || value == nil || isNilReflectValue(reflect.ValueOf(value)) {
				return nil, false
			}
			normalized, ok := normalizeIndexProbeValue(value, schemaFieldType(schema, rangeColumn))
			if !ok || normalized == nil || isNilReflectValue(reflect.ValueOf(normalized)) {
				return nil, false
			}
			point := &indexRangeBound{value: normalized, inclusive: true}
			mergeJoinIndexRangeBound(rangeBounds, point, true)
			mergeJoinIndexRangeBound(rangeBounds, point, false)
		}
		if constraints := joinRanges[rangeColumn]; len(constraints.lower) > 0 || len(constraints.upper) > 0 {
			for _, bound := range constraints.lower {
				normalized, ok := evaluateJoinIndexRangeBound(bound, tuple, now, variables)
				if !ok {
					return nil, false
				}
				normalized, ok = normalizeIndexRangeBound(normalized, schemaFieldType(schema, rangeColumn))
				if !ok {
					return nil, false
				}
				mergeJoinIndexRangeBound(rangeBounds, normalized, true)
			}
			for _, bound := range constraints.upper {
				normalized, ok := evaluateJoinIndexRangeBound(bound, tuple, now, variables)
				if !ok {
					return nil, false
				}
				normalized, ok = normalizeIndexRangeBound(normalized, schemaFieldType(schema, rangeColumn))
				if !ok {
					return nil, false
				}
				mergeJoinIndexRangeBound(rangeBounds, normalized, false)
			}
		}
		if rangeBounds.lower == nil && rangeBounds.upper == nil {
			return nil, false
		}
		query.lower = rangeBounds.lower
		query.upper = rangeBounds.upper
		if query.lower != nil && query.upper != nil {
			comparison, comparable := compareValues(Present(query.lower.value), Present(query.upper.value))
			if !comparable {
				return nil, false
			}
			if comparison > 0 || (comparison == 0 && (!query.lower.inclusive || !query.upper.inclusive)) {
				query.empty = true
			}
		}
		if query.empty {
			continue
		}
		encoded := encodeKey([]any{query.prefix, query.rangePosition, query.lower, query.upper})
		if _, exists := seen[encoded]; exists {
			continue
		}
		seen[encoded] = struct{}{}
		queries = append(queries, query)
		if len(queries) > maxIndexProbeKeys {
			return nil, false
		}
	}
	return queries, true
}

func (e *Engine) snapshotFireAndForgetJoinSourceWithIndex(ctx context.Context, source *streamNode, selection IndexSelection, targetSource int, definition *joinDefinition, joinWhere Expr, sides [][]storedEvent, loaded []bool, filterExpressions []Expr, now time.Time, variables map[string]Value) ([]Event, bool, error) {
	if selection.Access == IndexAccessRange {
		queries, usable := e.joinIndexProbeRangeSpecs(source, selection, targetSource, definition, joinWhere, sides, loaded, filterExpressions, now, variables)
		if !usable {
			return nil, false, nil
		}
		if len(queries) == 0 {
			return nil, true, nil
		}
		events, err := e.lookupIndexedFireAndForgetRangeSource(ctx, source, selection, queries, now)
		if err != nil {
			return nil, true, err
		}
		return events, true, nil
	}
	keys, usable := e.joinIndexProbeKeys(source, selection, targetSource, definition, joinWhere, sides, loaded, filterExpressions, now, variables)
	if !usable {
		return nil, false, nil
	}
	if len(keys) == 0 {
		return nil, true, nil
	}
	events, err := e.lookupIndexedFireAndForgetSource(ctx, source, selection, keys, now)
	if err != nil {
		return nil, true, err
	}
	return events, true, nil
}

func tableRowsAsEvents(source *streamNode, definition TableDefinition, rows []TableRow, now time.Time) ([]Event, error) {
	events := make([]Event, 0, len(rows))
	for _, row := range rows {
		values := make(map[string]any, len(row.values))
		for name, value := range row.values {
			values[name] = value.Any()
		}
		event, eventErr := newEvent(definition.schema, values, now)
		if eventErr != nil {
			return nil, eventErr
		}
		// The module remains in the source/catalog identity; the runtime event
		// type is the logical table name so normal source acceptance is retained.
		event.typeName = source.sourceName
		events = append(events, event)
	}
	return events, nil
}

// lookupIndexedFireAndForgetSource resolves complete equality keys against a
// declared Table/Named Window index. It is shared by single-source FAF and
// the Join candidate path so both paths have identical source identity and
// Table primary-key handling.
func (e *Engine) lookupIndexedFireAndForgetSource(ctx context.Context, source *streamNode, selection IndexSelection, keys [][]any, now time.Time) ([]Event, error) {
	base, err := sourceNode(source)
	if err != nil {
		return nil, err
	}
	switch base.kind {
	case streamNamedWindow:
		if strings.HasPrefix(selection.IndexName, "<") {
			return nil, NewError(ErrorUnknownName, "named window retention index is not a declared lookup index")
		}
		window, ok := e.NamedWindowInModule(base.moduleName, base.sourceName)
		if !ok {
			return nil, NewError(ErrorUnknownName, "named window "+base.sourceName+" is not registered")
		}
		return window.lookupMany(ctx, selection.IndexName, keys)
	case streamTable:
		table, ok := e.TableInModule(base.moduleName, base.sourceName)
		if !ok {
			return nil, NewError(ErrorUnknownName, "table "+base.sourceName+" is not registered")
		}
		definition := table.Definition()
		var rows []TableRow
		if selection.IndexName == "<primary-key>" {
			rows, err = table.lookupPrimaryMany(ctx, keys)
		} else {
			rows, err = table.lookupMany(ctx, selection.IndexName, keys)
		}
		if err != nil {
			return nil, err
		}
		return tableRowsAsEvents(base, definition, rows, now)
	default:
		return nil, NewError(ErrorInvalidRule, "indexed FAF source must be a named window or table")
	}
}

func (e *Engine) lookupIndexedFireAndForgetRangeSource(ctx context.Context, source *streamNode, selection IndexSelection, queries []indexRangeSpec, now time.Time) ([]Event, error) {
	base, err := sourceNode(source)
	if err != nil {
		return nil, err
	}
	switch base.kind {
	case streamNamedWindow:
		window, ok := e.NamedWindowInModule(base.moduleName, base.sourceName)
		if !ok {
			return nil, NewError(ErrorUnknownName, "named window "+base.sourceName+" is not registered")
		}
		return window.lookupRangeMany(ctx, selection.IndexName, queries)
	case streamTable:
		table, ok := e.TableInModule(base.moduleName, base.sourceName)
		if !ok {
			return nil, NewError(ErrorUnknownName, "table "+base.sourceName+" is not registered")
		}
		rows, err := table.lookupRangeMany(ctx, selection.IndexName, queries)
		if err != nil {
			return nil, err
		}
		return tableRowsAsEvents(base, table.Definition(), rows, now)
	default:
		return nil, NewError(ErrorInvalidRule, "range-indexed FAF source must be a named window or table")
	}
}

func (e *Engine) snapshotFireAndForgetSourceWithIndex(ctx context.Context, source *streamNode, selection IndexSelection, expressions []Expr, now time.Time, variables map[string]Value) ([]Event, error) {
	if base, baseErr := sourceNode(source); baseErr == nil && base != nil && base.kind == streamTable {
		if table := e.tables[catalogKey(base.moduleName, base.sourceName)]; table != nil && table.hasScopedState() {
			// A scoped Table has one independent primary/secondary index per
			// context partition. The context planner may not yet have selected a
			// partition (or may be evaluating a root/unscoped query), so preserve
			// complete snapshot semantics until a scope-aware candidate path is
			// available.
			return e.snapshotFireAndForgetSource(ctx, source, now, variables)
		}
	}
	if selection.Access == IndexAccessRange {
		query, usable := e.indexProbeRangeSpec(source, selection, expressions, now, variables)
		if !usable {
			return e.snapshotFireAndForgetSource(ctx, source, now, variables)
		}
		base, err := sourceNode(source)
		if err != nil {
			return nil, err
		}
		switch base.kind {
		case streamNamedWindow:
			if strings.HasPrefix(selection.IndexName, "<") {
				return e.snapshotFireAndForgetSource(ctx, source, now, variables)
			}
			window, ok := e.NamedWindowInModule(base.moduleName, base.sourceName)
			if !ok {
				return nil, NewError(ErrorUnknownName, "named window "+base.sourceName+" is not registered")
			}
			return window.lookupRange(ctx, selection.IndexName, query)
		case streamTable:
			table, ok := e.TableInModule(base.moduleName, base.sourceName)
			if !ok {
				return nil, NewError(ErrorUnknownName, "table "+base.sourceName+" is not registered")
			}
			rows, err := table.lookupRange(ctx, selection.IndexName, query)
			if err != nil {
				return nil, err
			}
			return tableRowsAsEvents(base, table.Definition(), rows, now)
		default:
			return e.snapshotFireAndForgetSource(ctx, source, now, variables)
		}
	}
	keys, usable := e.indexProbeKeys(source, selection, expressions, now, variables)
	if !usable {
		return e.snapshotFireAndForgetSource(ctx, source, now, variables)
	}
	base, err := sourceNode(source)
	if err != nil {
		return nil, err
	}
	if base.kind == streamNamedWindow && strings.HasPrefix(selection.IndexName, "<") {
		// Retention indexes are planner-visible summaries, not declared
		// Named Window lookup indexes. Preserve the ordinary snapshot path.
		return e.snapshotFireAndForgetSource(ctx, source, now, variables)
	}
	return e.lookupIndexedFireAndForgetSource(ctx, source, selection, keys, now)
}
