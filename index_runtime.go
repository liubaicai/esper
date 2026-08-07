package esper

import (
	"context"
	"reflect"
	"strings"
	"time"
)

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

func (e *Engine) snapshotFireAndForgetSourceWithIndex(ctx context.Context, source *streamNode, selection IndexSelection, expressions []Expr, now time.Time, variables map[string]Value) ([]Event, error) {
	keys, usable := e.indexProbeKeys(source, selection, expressions, now, variables)
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
		return window.lookupMany(ctx, selection.IndexName, keys)
	case streamTable:
		table, ok := e.TableInModule(base.moduleName, base.sourceName)
		if !ok {
			return nil, NewError(ErrorUnknownName, "table "+base.sourceName+" is not registered")
		}
		definition := table.Definition()
		var rows []TableRow
		if selection.IndexName == "<primary-key>" {
			// Primary keys are already the table's row identity and are not
			// duplicated into the secondary-index map. Probe each complete key;
			// this path is used only for a complete equality key.
			for _, key := range keys {
				row, found, getErr := table.Get(ctx, key...)
				if getErr != nil {
					return nil, getErr
				}
				if found {
					rows = append(rows, row)
				}
			}
		} else {
			rows, err = table.lookupMany(ctx, selection.IndexName, keys)
			if err != nil {
				return nil, err
			}
		}
		return tableRowsAsEvents(base, definition, rows, now)
	default:
		return e.snapshotFireAndForgetSource(ctx, source, now, variables)
	}
}
