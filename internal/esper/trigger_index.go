package esper

import (
	"reflect"
)

// trigger_index.go infers the implicit named-window indexes that Java
// creates for on-delete and on-select trigger where clauses. Java builds
// one index per distinct IndexMultiKey — an ordered list of hash props
// (equality conjuncts) plus an ordered list of range props (between
// conjuncts), each keyed by property name and coercion type — and drops it
// when the last owning statement undeploys. The Go engine keeps the same
// observable lifecycle: the index metadata is registered at statement
// deploy and ref-counted until undeploy, while the mutation itself still
// evaluates the predicate per candidate row.

// inferTriggerImplicitIndex extracts the implicit index spec from a
// named-window trigger where clause. It returns nil when no conjunct is
// indexable (Java creates no index for <=, not-between, disjunctions and
// non-window comparisons either).
func inferTriggerImplicitIndex(where Expr) *namedWindowImplicitIndex {
	if where == nil {
		return nil
	}
	spec := &namedWindowImplicitIndex{}
	collectTriggerImplicitIndexProps(where.node(), spec)
	if len(spec.hashProps) == 0 && len(spec.rangeProps) == 0 {
		return nil
	}
	return spec
}

func collectTriggerImplicitIndexProps(node *exprNode, spec *namedWindowImplicitIndex) {
	if node == nil {
		return
	}
	switch node.kind {
	case "and":
		for _, child := range node.children {
			collectTriggerImplicitIndexProps(child, spec)
		}
	case "equal-of", "eq":
		// "equal-of" is EqualOf's mixed-type node; "eq" is the typed
		// Equal[T] node. Java indexes both spellings of `=`.
		if len(node.children) != 2 {
			return
		}
		windowSide, otherSide := triggerIndexWindowOperand(node.children[0], node.children[1])
		if windowSide == nil {
			return
		}
		spec.hashProps = append(spec.hashProps, namedWindowImplicitProp{
			name:     windowSide.fieldName,
			coercion: triggerIndexCoercionKey(windowSide.typ, otherSide.typ),
		})
	case "between-of":
		// Only the inclusive non-negated between is indexable; not-between
		// and the explicit-bound variants reuse-or-scan like Java.
		if len(node.children) != 3 || node.children[0] == nil || node.children[0].kind != "named-window-field" {
			return
		}
		windowSide := node.children[0]
		var boundType reflect.Type
		for _, bound := range node.children[1:] {
			if bound != nil {
				if bound.kind == "named-window-field" {
					// A window-field bound is not a lookup value; Java
					// indexes nothing for it.
					return
				}
				boundType = triggerIndexCommonReflectType(boundType, bound.typ)
			}
		}
		spec.rangeProps = append(spec.rangeProps, namedWindowImplicitProp{
			name:     windowSide.fieldName,
			coercion: triggerIndexCoercionKey(windowSide.typ, boundType),
		})
	}
}

// triggerIndexWindowOperand returns the named-window-field operand and its
// counterpart, or nil when neither side reads the candidate window row or
// both sides do. Java's SubordPropAnalyzer only forms a lookup key when the
// other side is a lookup value (trigger-stream property or constant); a
// window-field-vs-window-field comparison indexes nothing.
func triggerIndexWindowOperand(left, right *exprNode) (windowSide, otherSide *exprNode) {
	if left != nil && left.kind == "named-window-field" {
		if right != nil && right.kind == "named-window-field" {
			return nil, nil
		}
		return left, right
	}
	if right != nil && right.kind == "named-window-field" {
		return right, left
	}
	return nil, nil
}

// triggerIndexCoercionKey names the common coercion type Java derives for
// an indexed comparison: the wider numeric type when both operands are
// numeric, otherwise the operand type they share. Only equality of keys
// matters — the names distinguish the same pairs Java's boxed coercion
// types distinguish (Integer vs Long vs Double).
func triggerIndexCoercionKey(windowType, otherType reflect.Type) string {
	return triggerIndexCommonType(windowType, otherType)
}

func triggerIndexCommonType(left, right reflect.Type) string {
	return triggerIndexTypeName(triggerIndexCommonReflectType(left, right))
}

func triggerIndexCommonReflectType(left, right reflect.Type) reflect.Type {
	left = unwrapTriggerIndexType(left)
	right = unwrapTriggerIndexType(right)
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	if isNumericType(left) && isNumericType(right) {
		if triggerIndexNumericRank(right) > triggerIndexNumericRank(left) {
			return right
		}
		return left
	}
	if left == right {
		return left
	}
	return left
}

func unwrapTriggerIndexType(typ reflect.Type) reflect.Type {
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}

// triggerIndexNumericRank orders numeric kinds by Java widening so the
// wider operand's boxed type names the coercion (int64→Long, float64→Double).
func triggerIndexNumericRank(typ reflect.Type) int {
	switch typ.Kind() {
	case reflect.Int8, reflect.Uint8:
		return 1 // Byte
	case reflect.Int16, reflect.Uint16:
		return 2 // Short
	case reflect.Int32:
		return 3 // Integer
	case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint32, reflect.Uint64:
		return 4 // Long
	case reflect.Float32:
		return 5 // Float
	case reflect.Float64:
		return 6 // Double
	}
	return 0
}

func triggerIndexTypeName(typ reflect.Type) string {
	if typ == nil {
		return "any"
	}
	switch typ.Kind() {
	case reflect.Int8, reflect.Uint8:
		return "Byte"
	case reflect.Int16, reflect.Uint16:
		return "Short"
	case reflect.Int32:
		return "Integer"
	case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint32, reflect.Uint64:
		return "Long"
	case reflect.Float32:
		return "Float"
	case reflect.Float64:
		return "Double"
	case reflect.String:
		return "String"
	case reflect.Bool:
		return "Boolean"
	}
	return typ.String()
}

// registerTriggerImplicitIndexLocked infers and registers the implicit
// index for a named-window delete/select trigger at statement deploy. The
// caller must hold e.mu. It returns the spec actually registered so the
// statement can release exactly that index on undeploy.
func (e *Engine) registerTriggerImplicitIndexLocked(statement *Statement) *namedWindowImplicitIndex {
	if e == nil || statement == nil {
		return nil
	}
	trigger := statement.plan.query.trigger
	if trigger == nil || trigger.target != triggerTargetNamedWindow || trigger.where == nil {
		return nil
	}
	// Java plans an implicit index for every OnTriggerWindowDesc action that
	// looks rows up in the window: delete, select, update and merge all run
	// SubordinateQueryPlanner.planOnExpression. Insert-only triggers have no
	// window lookup and stay excluded.
	switch trigger.action {
	case triggerDeleteTable, triggerDeleteAllTable, triggerSelectTable, triggerUpdateTable, triggerMergeTable:
	default:
		return nil
	}
	spec := inferTriggerImplicitIndex(trigger.where)
	if spec == nil {
		return nil
	}
	// ensureNamedWindowLockedInModule materializes the runtime window
	// lazily, so the index lands on the same instance the trigger resolves
	// at send time even when the window was registered after engine
	// creation.
	window, ok := e.ensureNamedWindowLockedInModule(trigger.moduleName, trigger.table)
	if !ok || !window.registerImplicitIndex(statement.id, spec) {
		return nil
	}
	return spec
}

// releaseTriggerImplicitIndexLocked drops the statement's implicit index
// reference at undeploy. The caller must hold e.mu.
func (e *Engine) releaseTriggerImplicitIndexLocked(statement *Statement) {
	if e == nil || statement == nil {
		return
	}
	trigger := statement.plan.query.trigger
	if trigger == nil || trigger.target != triggerTargetNamedWindow {
		return
	}
	spec := statement.runtime.triggerImplicitIndex
	if spec == nil {
		return
	}
	statement.runtime.triggerImplicitIndex = nil
	if window, ok := e.ensureNamedWindowLockedInModule(trigger.moduleName, trigger.table); ok {
		window.unregisterImplicitIndex(statement.id, spec)
	}
}

// mergeUpdatedColumns returns the target columns a table merge statement
// assigns in its when-matched update actions. Esper forbids a later
// create-unique-index over any of them while the merge stays deployed.
func mergeUpdatedColumns(definition *triggerDefinition) []string {
	if definition == nil || definition.action != triggerMergeTable {
		return nil
	}
	var columns []string
	for _, clause := range definition.merge {
		if !clause.Matched {
			continue
		}
		for _, action := range tableMergeClauseActions(clause) {
			if action.Delete || action.InsertTarget != "" || action.InsertIntoTarget {
				continue
			}
			for _, assignment := range action.Assignments {
				if assignment.Wildcard || assignment.Column == "" {
					continue
				}
				columns = append(columns, assignment.Column)
			}
		}
	}
	return columns
}

// registerMergeUpdatedColumnsLocked records the merge statement's
// when-matched update columns on its target table at deploy. The caller
// must hold e.mu.
func (e *Engine) registerMergeUpdatedColumnsLocked(statement *Statement) {
	if e == nil || statement == nil {
		return
	}
	trigger := statement.plan.query.trigger
	columns := mergeUpdatedColumns(trigger)
	if len(columns) == 0 {
		return
	}
	table, ok := e.ensureTableLockedInModule(trigger.moduleName, trigger.table)
	if !ok || table == nil {
		return
	}
	table.mergeUpdatedMu.Lock()
	if table.mergeUpdatedColumns == nil {
		table.mergeUpdatedColumns = make(map[string]int)
	}
	for _, column := range columns {
		table.mergeUpdatedColumns[column]++
	}
	table.mergeUpdatedMu.Unlock()
	statement.runtime.mergeUpdatedColumnsRegistered = true
}

// releaseMergeUpdatedColumnsLocked drops the merge statement's column
// references at undeploy. The caller must hold e.mu. It is a no-op unless
// registration actually ran, so a prepare failure before registration
// cannot decrement a count it never added.
func (e *Engine) releaseMergeUpdatedColumnsLocked(statement *Statement) {
	if e == nil || statement == nil || !statement.runtime.mergeUpdatedColumnsRegistered {
		return
	}
	statement.runtime.mergeUpdatedColumnsRegistered = false
	trigger := statement.plan.query.trigger
	columns := mergeUpdatedColumns(trigger)
	if len(columns) == 0 {
		return
	}
	table, ok := e.ensureTableLockedInModule(trigger.moduleName, trigger.table)
	if !ok || table == nil {
		return
	}
	table.mergeUpdatedMu.Lock()
	for _, column := range columns {
		if table.mergeUpdatedColumns[column] <= 1 {
			delete(table.mergeUpdatedColumns, column)
		} else {
			table.mergeUpdatedColumns[column]--
		}
	}
	table.mergeUpdatedMu.Unlock()
}

// validateMergeUniqueColumnsLocked re-checks a merge statement's
// when-matched update columns against the live table's unique keys at
// deploy. env.Build sees only the declared TableDefinition; a unique index
// created later through Table.CreateIndex lands on the engine-side state,
// so Java's compile-time check (which reads shared TableMetaData including
// deployed create-index statements) maps to this deploy-time gate in Go.
// The caller must hold e.mu.
func (e *Engine) validateMergeUniqueColumnsLocked(statement *Statement) error {
	if e == nil || statement == nil {
		return nil
	}
	trigger := statement.plan.query.trigger
	columns := mergeUpdatedColumns(trigger)
	if len(columns) == 0 {
		return nil
	}
	table, ok := e.ensureTableLockedInModule(trigger.moduleName, trigger.table)
	if !ok || table == nil {
		return nil
	}
	root := table.state
	root.mu.Lock()
	unique := make(map[string]struct{}, len(root.def.primaryKey))
	for _, column := range root.def.primaryKey {
		unique[column] = struct{}{}
	}
	for _, index := range root.def.indexes {
		if !index.Unique {
			continue
		}
		for _, column := range index.Columns {
			unique[column] = struct{}{}
		}
	}
	root.mu.Unlock()
	for _, column := range columns {
		if _, hit := unique[column]; hit {
			return NewError(ErrorInvalidRule, "On-merge statements may not update unique keys of tables")
		}
	}
	return nil
}
