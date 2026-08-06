package esper

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type triggerActionKind uint8

const (
	triggerInsertTable triggerActionKind = iota
	triggerUpsertTable
	triggerUpdateTable
	triggerDeleteTable
	triggerMergeTable
	triggerSetVariables
	triggerSelectTable
	triggerDeleteAllTable
)

type triggerTargetKind uint8

const (
	triggerTargetTable triggerTargetKind = iota
	triggerTargetNamedWindow
)

// TableAssignment maps an incoming event expression to one target table
// column. It is the Go builder equivalent of an on-trigger column assignment.
type TableAssignment struct {
	Column string
	Expr   Expr
}

func SetColumn(column string, expression Expr) TableAssignment {
	return TableAssignment{Column: strings.TrimSpace(column), Expr: expression}
}

// VariableAssignmentExpr maps an incoming event expression to one registered
// runtime variable. It is the fluent equivalent of an on-set assignment.
type VariableAssignmentExpr struct {
	Name string
	Expr Expr
}

func SetVariableExpr(name string, expression Expr) VariableAssignmentExpr {
	return VariableAssignmentExpr{Name: strings.TrimSpace(name), Expr: expression}
}

// TableMergeClause is one ordered matched or not-matched branch of a table
// merge. Conditions are evaluated against the triggering event; assignments
// are applied to the existing row or the new row respectively.
type TableMergeClause struct {
	Matched     bool
	Condition   Expr
	Assignments []TableAssignment
	Delete      bool
}

// NamedWindowMergeClause uses the same ordered branch contract as table
// merge. The alias keeps the fluent clause constructors reusable while the
// target-specific method makes the resulting plan explicit.
type NamedWindowMergeClause = TableMergeClause

func WhenMatched(condition Expr, assignments ...TableAssignment) TableMergeClause {
	return TableMergeClause{Matched: true, Condition: condition, Assignments: append([]TableAssignment(nil), assignments...)}
}

func WhenNotMatched(condition Expr, assignments ...TableAssignment) TableMergeClause {
	return TableMergeClause{Condition: condition, Assignments: append([]TableAssignment(nil), assignments...)}
}

func WhenMatchedDelete(condition Expr) TableMergeClause {
	return TableMergeClause{Matched: true, Condition: condition, Delete: true}
}

type triggerDefinition struct {
	input               *streamNode
	table               string
	target              triggerTargetKind
	action              triggerActionKind
	assignments         []TableAssignment
	keys                []Expr
	where               Expr
	merge               []TableMergeClause
	variableAssignments []VariableAssignmentExpr
	selections          []Selection
}

type tableMutationResult struct {
	oldRows   []TableRow
	newRows   []TableRow
	oldEvents []Event
	newEvents []Event
}

type TriggerStream[T any] struct {
	env  *Environment
	node *streamNode
}

// OnEvent starts a state-mutation rule from a typed event stream.
func OnEvent[T any](stream Stream[T]) TriggerStream[T] {
	return TriggerStream[T]{env: stream.env, node: stream.node}
}

// OnRecord starts a state-mutation rule from a schema-driven dynamic stream.
func OnRecord(stream RecordStream) TriggerStream[any] {
	return TriggerStream[any]{env: stream.env, node: stream.node}
}

type TriggerQuery struct {
	env        *Environment
	definition *triggerDefinition
}

func (s TriggerStream[T]) InsertIntoTable(table string, assignments ...TableAssignment) TriggerQuery {
	return s.trigger(table, triggerInsertTable, assignments, nil)
}

// UpsertIntoTable is the basic merge form: a row with the same primary key is
// replaced, otherwise a new row is inserted.
func (s TriggerStream[T]) UpsertIntoTable(table string, assignments ...TableAssignment) TriggerQuery {
	return s.trigger(table, triggerUpsertTable, assignments, nil)
}

// MergeIntoTable is an explicit fluent alias for the basic key-based merge
// behavior. Use MergeIntoTableWhen for ordered conditional branches.
func (s TriggerStream[T]) MergeIntoTable(table string, assignments ...TableAssignment) TriggerQuery {
	return s.trigger(table, triggerUpsertTable, assignments, nil)
}

// MergeIntoTableWhen exposes ordered conditional merge branches. Keys must
// correspond to the table primary key columns.
func (s TriggerStream[T]) MergeIntoTableWhen(table string, keys []Expr, clauses ...TableMergeClause) TriggerQuery {
	return TriggerQuery{
		env: s.env,
		definition: &triggerDefinition{
			input:  s.node,
			table:  strings.TrimSpace(table),
			action: triggerMergeTable,
			keys:   append([]Expr(nil), keys...),
			merge:  cloneTableMergeClauses(clauses),
		},
	}
}

// MergeIntoNamedWindowWhen applies ordered matched and not-matched branches
// to events in a named window. The match expression may read the incoming
// event through Field and the candidate window event through NamedWindowField.
// A nil match expression means that no explicit match predicate was supplied:
// every existing target event is treated as matched, while an empty window
// reaches the not-matched branches. Use Literal(false) for an insert-only
// rule that must insert every trigger event.
func (s TriggerStream[T]) MergeIntoNamedWindowWhen(window string, match Expression[bool], clauses ...NamedWindowMergeClause) TriggerQuery {
	return TriggerQuery{
		env: s.env,
		definition: &triggerDefinition{
			input:  s.node,
			table:  strings.TrimSpace(window),
			target: triggerTargetNamedWindow,
			action: triggerMergeTable,
			where:  match,
			merge:  cloneTableMergeClauses(clauses),
		},
	}
}

// MergeIntoNamedWindow is the concise alias for MergeIntoNamedWindowWhen.
// The explicit When form remains useful when documenting ordered branches.
func (s TriggerStream[T]) MergeIntoNamedWindow(window string, match Expression[bool], clauses ...NamedWindowMergeClause) TriggerQuery {
	return s.MergeIntoNamedWindowWhen(window, match, clauses...)
}

func (s TriggerStream[T]) UpdateTable(table string, keys []Expr, assignments ...TableAssignment) TriggerQuery {
	return s.trigger(table, triggerUpdateTable, assignments, append([]Expr(nil), keys...))
}

func (s TriggerStream[T]) DeleteFromTable(table string, keys ...Expr) TriggerQuery {
	return s.trigger(table, triggerDeleteTable, nil, append([]Expr(nil), keys...))
}

// UpdateTableWhere updates every table row whose target-row predicate is true
// for the current trigger event. Use TableField in the predicate or
// assignments to reference the candidate row and Field for the trigger.
func (s TriggerStream[T]) UpdateTableWhere(table string, predicate Expression[bool], assignments ...TableAssignment) TriggerQuery {
	return s.triggerWhere(table, triggerUpdateTable, predicate, assignments)
}

// DeleteFromTableWhere deletes every table row whose target-row predicate is
// true for the current trigger event.
func (s TriggerStream[T]) DeleteFromTableWhere(table string, predicate Expression[bool]) TriggerQuery {
	return s.triggerWhere(table, triggerDeleteTable, predicate, nil)
}

// DeleteAllFromTable removes every row from a table when the trigger arrives.
func (s TriggerStream[T]) DeleteAllFromTable(table string) TriggerQuery {
	return s.trigger(table, triggerDeleteAllTable, nil, nil)
}

// InsertIntoNamedWindow appends a projected event to a named window. The
// assignments describe the target-window fields and are evaluated against
// the incoming trigger event.
func (s TriggerStream[T]) InsertIntoNamedWindow(window string, assignments ...TableAssignment) TriggerQuery {
	return s.namedWindowTrigger(window, triggerInsertTable, nil, assignments, nil)
}

// UpdateNamedWindow updates every named-window event matching predicate.
// NamedWindowField reads the candidate event; Field reads the trigger event.
func (s TriggerStream[T]) UpdateNamedWindow(window string, predicate Expression[bool], assignments ...TableAssignment) TriggerQuery {
	return s.namedWindowTrigger(window, triggerUpdateTable, predicate, assignments, nil)
}

// DeleteFromNamedWindow deletes matching named-window events.
func (s TriggerStream[T]) DeleteFromNamedWindow(window string, predicate Expression[bool]) TriggerQuery {
	return s.namedWindowTrigger(window, triggerDeleteTable, predicate, nil, nil)
}

// DeleteAllFromNamedWindow removes every event from a named window.
func (s TriggerStream[T]) DeleteAllFromNamedWindow(window string) TriggerQuery {
	return s.namedWindowTrigger(window, triggerDeleteAllTable, nil, nil, nil)
}

// SelectFromNamedWindow reads matching named-window events. With no
// selections it emits the original events; selections produce Row results and
// may combine NamedWindowField with trigger Field expressions.
func (s TriggerStream[T]) SelectFromNamedWindow(window string, predicate Expression[bool], selections ...Selection) TriggerQuery {
	return s.namedWindowTrigger(window, triggerSelectTable, predicate, nil, selections)
}

// SetVariable updates one registered variable when the trigger event arrives.
func (s TriggerStream[T]) SetVariable(name string, expression Expr) TriggerQuery {
	return s.SetVariables(SetVariableExpr(name, expression))
}

// SetVariables updates a batch of registered variables as one trigger action.
func (s TriggerStream[T]) SetVariables(assignments ...VariableAssignmentExpr) TriggerQuery {
	return TriggerQuery{
		env: s.env,
		definition: &triggerDefinition{
			input:               s.node,
			action:              triggerSetVariables,
			variableAssignments: append([]VariableAssignmentExpr(nil), assignments...),
		},
	}
}

// SelectFromTable performs a primary-key lookup when the trigger arrives and
// emits the requested table projection as a Row result.
func (s TriggerStream[T]) SelectFromTable(table string, keys []Expr, selections ...Selection) TriggerQuery {
	return TriggerQuery{
		env: s.env,
		definition: &triggerDefinition{
			input:      s.node,
			table:      strings.TrimSpace(table),
			action:     triggerSelectTable,
			keys:       append([]Expr(nil), keys...),
			selections: append([]Selection(nil), selections...),
		},
	}
}

// SelectFromTableWhere projects every table row matching a target-row
// predicate. TableField addresses the row and Field addresses the trigger.
func (s TriggerStream[T]) SelectFromTableWhere(table string, predicate Expression[bool], selections ...Selection) TriggerQuery {
	return TriggerQuery{
		env: s.env,
		definition: &triggerDefinition{
			input:      s.node,
			table:      strings.TrimSpace(table),
			action:     triggerSelectTable,
			where:      predicate,
			selections: append([]Selection(nil), selections...),
		},
	}
}

func (s TriggerStream[T]) trigger(table string, action triggerActionKind, assignments []TableAssignment, keys []Expr) TriggerQuery {
	return TriggerQuery{
		env: s.env,
		definition: &triggerDefinition{
			input:       s.node,
			table:       strings.TrimSpace(table),
			action:      action,
			assignments: append([]TableAssignment(nil), assignments...),
			keys:        append([]Expr(nil), keys...),
		},
	}
}

func (s TriggerStream[T]) triggerWhere(table string, action triggerActionKind, predicate Expression[bool], assignments []TableAssignment) TriggerQuery {
	return TriggerQuery{
		env: s.env,
		definition: &triggerDefinition{
			input:       s.node,
			table:       strings.TrimSpace(table),
			action:      action,
			where:       predicate,
			assignments: append([]TableAssignment(nil), assignments...),
		},
	}
}

func (s TriggerStream[T]) namedWindowTrigger(window string, action triggerActionKind, predicate Expression[bool], assignments []TableAssignment, selections []Selection) TriggerQuery {
	return TriggerQuery{
		env: s.env,
		definition: &triggerDefinition{
			input:       s.node,
			table:       strings.TrimSpace(window),
			target:      triggerTargetNamedWindow,
			action:      action,
			where:       predicate,
			assignments: append([]TableAssignment(nil), assignments...),
			selections:  append([]Selection(nil), selections...),
		},
	}
}

func (q TriggerQuery) Query(options ...QueryOption) Query {
	spec := querySpec{selector: SelectIStream, output: OutputAll()}
	for _, option := range options {
		if option != nil {
			option(&spec)
		}
	}
	return Query{
		env:                        q.env,
		input:                      q.definitionInput(),
		trigger:                    q.definition,
		selections:                 append([]Selection(nil), q.definition.selections...),
		name:                       spec.name,
		statementUserObject:        spec.statementUserObject,
		selector:                   spec.selector,
		sink:                       spec.sink,
		contextName:                spec.contextName,
		output:                     spec.output,
		distinct:                   spec.distinct,
		discardPartialsOnMatch:     spec.discardPartialsOnMatch,
		suppressOverlappingMatches: spec.suppressOverlappingMatches,
		orderBy:                    append([]SortKey(nil), spec.orderBy...),
		limit:                      spec.limit,
		offset:                     spec.offset,
	}
}

func (q TriggerQuery) definitionInput() *streamNode {
	if q.definition == nil {
		return nil
	}
	return q.definition.input
}

func (d *triggerDefinition) description() string {
	if d == nil {
		return "trigger(<nil>)"
	}
	action := map[triggerActionKind]string{
		triggerInsertTable:    "insert",
		triggerUpsertTable:    "upsert",
		triggerUpdateTable:    "update",
		triggerDeleteTable:    "delete",
		triggerMergeTable:     "merge",
		triggerSetVariables:   "set-variables",
		triggerSelectTable:    "select",
		triggerDeleteAllTable: "delete-all",
	}[d.action]
	if d.action == triggerMergeTable {
		keys := make([]string, 0, len(d.keys))
		for _, key := range d.keys {
			if key == nil {
				keys = append(keys, "<nil>")
				continue
			}
			keys = append(keys, key.Description())
		}
		clauses := make([]string, 0, len(d.merge))
		for _, clause := range d.merge {
			branch := "not-matched"
			if clause.Matched {
				branch = "matched"
			}
			condition := "<nil>"
			if clause.Condition != nil {
				condition = clause.Condition.Description()
			}
			if clause.Delete {
				clauses = append(clauses, branch+"("+condition+")->delete")
				continue
			}
			clauses = append(clauses, branch+"("+condition+")->"+describeTableAssignments(clause.Assignments))
		}
		target := "table"
		if d.target == triggerTargetNamedWindow {
			target = "named-window"
		}
		match := "<nil>"
		if d.where != nil {
			match = d.where.Description()
		}
		return fmt.Sprintf("on(%s)->%s.merge(match(%s);keys[%s];%s)", d.input.describe(), target, match, strings.Join(keys, ","), strings.Join(clauses, "|"))
	}
	if d.action == triggerSetVariables {
		parts := make([]string, 0, len(d.variableAssignments))
		for _, assignment := range d.variableAssignments {
			expression := "<nil>"
			if assignment.Expr != nil {
				expression = assignment.Expr.Description()
			}
			parts = append(parts, assignment.Name+"="+expression)
		}
		return fmt.Sprintf("on(%s)->variables(%s)", d.input.describe(), strings.Join(parts, ","))
	}
	target := "table"
	if d.target == triggerTargetNamedWindow {
		target = "named-window"
	}
	if d.action == triggerSelectTable {
		keys := make([]string, 0, len(d.keys))
		for _, key := range d.keys {
			if key == nil {
				keys = append(keys, "<nil>")
			} else {
				keys = append(keys, key.Description())
			}
		}
		selections := make([]string, 0, len(d.selections))
		for _, selection := range d.selections {
			expression := "<nil>"
			if selection.Expr != nil {
				expression = selection.Expr.Description()
			}
			selections = append(selections, selection.Name+"="+expression)
		}
		where := ""
		if d.where != nil {
			where = ";where=" + d.where.Description()
		}
		return fmt.Sprintf("on(%s)->%s.select(keys[%s]%s;%s)", d.input.describe(), target, strings.Join(keys, ","), where, strings.Join(selections, ","))
	}
	where := ""
	if d.where != nil {
		where = ",where=" + d.where.Description()
	}
	return fmt.Sprintf("on(%s)->%s.%s(%s%s)", d.input.describe(), target, action, describeTableAssignments(d.assignments), where)
}

func describeTableAssignments(assignments []TableAssignment) string {
	parts := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		expression := "<nil>"
		if assignment.Expr != nil {
			expression = assignment.Expr.Description()
		}
		parts = append(parts, assignment.Column+"="+expression)
	}
	return strings.Join(parts, ",")
}

func (e *Environment) validateTrigger(definition *triggerDefinition) error {
	if definition == nil || definition.input == nil {
		return NewError(ErrorInvalidRule, "trigger requires a source")
	}
	if err := e.validateNode(definition.input); err != nil {
		return err
	}
	if definition.action == triggerSetVariables {
		return e.validateVariableTriggerAssignments(definition)
	}
	if definition.target == triggerTargetNamedWindow {
		return e.validateNamedWindowTrigger(definition)
	}
	table, ok := e.Table(definition.table)
	if !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("trigger references unknown table %q", definition.table))
	}
	if definition.where != nil {
		if definition.action != triggerUpdateTable && definition.action != triggerDeleteTable && definition.action != triggerSelectTable {
			return NewError(ErrorInvalidRule, "table trigger predicate is supported only for select, update or delete")
		}
		if definition.where.Type() != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, "table trigger predicate must return bool")
		}
		if err := e.validateTriggerTargetExpression(definition.input, table.schema, definition.where, "table-field"); err != nil {
			return fmt.Errorf("table trigger predicate: %w", err)
		}
	}
	if definition.action == triggerSelectTable {
		if definition.where != nil {
			if len(definition.keys) != 0 {
				return NewError(ErrorInvalidRule, "table predicate select cannot also provide primary-key expressions")
			}
		} else {
			if len(definition.keys) != len(table.primaryKey) || len(definition.keys) == 0 {
				return fmt.Errorf("table select requires one key expression for each primary-key column")
			}
			for index, expression := range definition.keys {
				if expression == nil {
					return fmt.Errorf("table select key expression %d is nil", index)
				}
				if err := e.validateExprFields(definition.input, expression); err != nil {
					return fmt.Errorf("table select key %d: %w", index, err)
				}
			}
		}
		if len(definition.selections) == 0 {
			return NewError(ErrorInvalidRule, "table select requires at least one projection")
		}
		seenSelections := make(map[string]struct{}, len(definition.selections))
		tableNode := &streamNode{kind: streamTable, sourceName: definition.table, sourceType: typeOf[any]()}
		for _, selection := range definition.selections {
			if strings.TrimSpace(selection.Name) == "" || selection.Expr == nil {
				return NewError(ErrorInvalidRule, "table select projection requires a name and expression")
			}
			if _, exists := seenSelections[selection.Name]; exists {
				return NewError(ErrorInvalidRule, fmt.Sprintf("table select duplicates alias %q", selection.Name))
			}
			seenSelections[selection.Name] = struct{}{}
			var expressionErr error
			if definition.where != nil {
				expressionErr = e.validateTriggerTargetExpression(definition.input, table.schema, selection.Expr, "table-field")
			} else {
				expressionErr = e.validateExprFields(tableNode, selection.Expr)
			}
			if expressionErr != nil {
				return fmt.Errorf("table select projection %q: %w", selection.Name, expressionErr)
			}
		}
		return nil
	}
	if definition.action != triggerDeleteTable && definition.action != triggerDeleteAllTable && definition.action != triggerMergeTable && len(definition.assignments) == 0 {
		return NewError(ErrorInvalidRule, "table trigger requires at least one assignment")
	}
	seen := make(map[string]struct{}, len(definition.assignments))
	for index, assignment := range definition.assignments {
		if assignment.Column == "" || assignment.Expr == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("table trigger assignment %d is invalid", index))
		}
		if _, exists := seen[assignment.Column]; exists {
			return NewError(ErrorInvalidRule, fmt.Sprintf("table trigger duplicates column %q", assignment.Column))
		}
		seen[assignment.Column] = struct{}{}
		if _, exists := table.schema.Field(assignment.Column); !exists {
			return NewError(ErrorUnknownName, fmt.Sprintf("table trigger references unknown column %q", assignment.Column))
		}
		if err := e.validateTriggerTargetExpression(definition.input, table.schema, assignment.Expr, "table-field"); err != nil {
			return fmt.Errorf("assignment %q: %w", assignment.Column, err)
		}
	}
	if definition.action == triggerUpdateTable || definition.action == triggerDeleteTable {
		if definition.where != nil {
			if len(definition.keys) != 0 {
				return NewError(ErrorInvalidRule, "table predicate trigger cannot also provide primary-key expressions")
			}
		} else if len(definition.keys) != len(table.primaryKey) || len(definition.keys) == 0 {
			return fmt.Errorf("table trigger requires one key expression for each primary-key column")
		} else {
			for index, expression := range definition.keys {
				if expression == nil {
					return fmt.Errorf("table trigger key expression %d is nil", index)
				}
				if err := e.validateExprFields(definition.input, expression); err != nil {
					return fmt.Errorf("table key %d: %w", index, err)
				}
			}
		}
	}
	if definition.action == triggerMergeTable {
		if len(definition.keys) != len(table.primaryKey) || len(definition.keys) == 0 {
			return fmt.Errorf("table merge requires one key expression for each primary-key column")
		}
		for index, expression := range definition.keys {
			if expression == nil {
				return fmt.Errorf("table merge key expression %d is nil", index)
			}
			if err := e.validateExprFields(definition.input, expression); err != nil {
				return fmt.Errorf("table merge key %d: %w", index, err)
			}
		}
		if len(definition.merge) == 0 {
			return NewError(ErrorInvalidRule, "table merge requires at least one clause")
		}
		var matched, notMatched bool
		for index, clause := range definition.merge {
			if clause.Condition == nil || clause.Condition.Type() != typeOf[bool]() {
				return fmt.Errorf("table merge clause %d requires a bool condition", index)
			}
			if clause.Matched {
				matched = true
			} else {
				notMatched = true
			}
			if clause.Delete && !clause.Matched {
				return fmt.Errorf("table merge delete clause %d must be matched", index)
			}
			if clause.Delete && len(clause.Assignments) > 0 {
				return fmt.Errorf("table merge delete clause %d cannot assign columns", index)
			}
			if !clause.Delete && len(clause.Assignments) == 0 {
				return fmt.Errorf("table merge clause %d requires assignments", index)
			}
			if err := validateTriggerAssignments(e, definition.input, table, clause.Assignments); err != nil {
				return fmt.Errorf("table merge clause %d: %w", index, err)
			}
			if err := e.validateExprFields(definition.input, clause.Condition); err != nil {
				return fmt.Errorf("table merge clause %d condition: %w", index, err)
			}
		}
		if !matched || !notMatched {
			return NewError(ErrorInvalidRule, "table merge requires matched and not-matched clauses")
		}
	}
	return nil
}

func (e *Environment) validateNamedWindowTrigger(definition *triggerDefinition) error {
	window, ok := e.NamedWindow(definition.table)
	if !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("trigger references unknown named window %q", definition.table))
	}
	targetSchema := window.schema
	if definition.where != nil {
		if definition.action != triggerUpdateTable && definition.action != triggerDeleteTable && definition.action != triggerSelectTable && definition.action != triggerMergeTable {
			return NewError(ErrorInvalidRule, "named-window predicate is supported only for select, update, delete or merge")
		}
		if definition.where.Type() != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, "named-window predicate must return bool")
		}
		if err := e.validateTriggerTargetExpression(definition.input, targetSchema, definition.where, "named-window-field"); err != nil {
			return fmt.Errorf("named-window predicate: %w", err)
		}
	}
	if definition.action == triggerMergeTable {
		if len(definition.merge) == 0 {
			return NewError(ErrorInvalidRule, "named-window merge requires at least one clause")
		}
		for index, clause := range definition.merge {
			if clause.Condition == nil || clause.Condition.Type() != typeOf[bool]() {
				return fmt.Errorf("named-window merge clause %d requires a bool condition", index)
			}
			if !clause.Matched {
				var targetFields []string
				clause.Condition.node().referencedTargetFields("named-window-field", &targetFields)
				if len(targetFields) > 0 {
					return fmt.Errorf("named-window merge clause %d not-matched condition cannot reference named-window fields", index)
				}
			}
			if clause.Delete && !clause.Matched {
				return fmt.Errorf("named-window merge clause %d cannot delete on not-matched", index)
			}
			if !clause.Delete && clause.Matched && len(clause.Assignments) == 0 {
				return fmt.Errorf("named-window merge clause %d requires assignments", index)
			}
			if clause.Delete && len(clause.Assignments) != 0 {
				return fmt.Errorf("named-window merge clause %d delete cannot have assignments", index)
			}
			if err := e.validateTriggerTargetExpression(definition.input, targetSchema, clause.Condition, "named-window-field"); err != nil {
				return fmt.Errorf("named-window merge clause %d condition: %w", index, err)
			}
			seen := make(map[string]struct{}, len(clause.Assignments))
			for assignmentIndex, assignment := range clause.Assignments {
				if assignment.Column == "" || assignment.Expr == nil {
					return fmt.Errorf("named-window merge clause %d assignment %d is invalid", index, assignmentIndex)
				}
				if _, exists := seen[assignment.Column]; exists {
					return fmt.Errorf("named-window merge clause %d duplicates column %q", index, assignment.Column)
				}
				seen[assignment.Column] = struct{}{}
				if _, exists := targetSchema.Field(assignment.Column); !exists {
					return NewError(ErrorUnknownName, fmt.Sprintf("named-window merge references unknown field %q", assignment.Column))
				}
				if !clause.Matched {
					var targetFields []string
					assignment.Expr.node().referencedTargetFields("named-window-field", &targetFields)
					if len(targetFields) > 0 {
						return fmt.Errorf("named-window merge clause %d not-matched assignment cannot reference named-window fields", index)
					}
				}
				if err := e.validateTriggerTargetExpression(definition.input, targetSchema, assignment.Expr, "named-window-field"); err != nil {
					return fmt.Errorf("named-window merge assignment %q: %w", assignment.Column, err)
				}
			}
		}
		return nil
	}
	if definition.action == triggerInsertTable && len(definition.assignments) == 0 {
		return NewError(ErrorInvalidRule, "named-window insert requires at least one assignment")
	}
	if definition.action == triggerUpdateTable && len(definition.assignments) == 0 {
		return NewError(ErrorInvalidRule, "named-window update requires at least one assignment")
	}
	if definition.action == triggerUpdateTable && definition.where == nil {
		return NewError(ErrorInvalidRule, "named-window update requires a predicate")
	}
	if definition.action == triggerSelectTable && len(definition.selections) > 0 {
		seen := make(map[string]struct{}, len(definition.selections))
		for _, selection := range definition.selections {
			if strings.TrimSpace(selection.Name) == "" || selection.Expr == nil {
				return NewError(ErrorInvalidRule, "named-window select projection requires a name and expression")
			}
			if _, exists := seen[selection.Name]; exists {
				return NewError(ErrorInvalidRule, fmt.Sprintf("named-window select duplicates alias %q", selection.Name))
			}
			seen[selection.Name] = struct{}{}
			if err := e.validateTriggerTargetExpression(definition.input, targetSchema, selection.Expr, "named-window-field"); err != nil {
				return fmt.Errorf("named-window select projection %q: %w", selection.Name, err)
			}
		}
	}
	if definition.action == triggerDeleteAllTable || definition.action == triggerDeleteTable || definition.action == triggerSelectTable || definition.action == triggerUpdateTable || definition.action == triggerInsertTable {
		for index, assignment := range definition.assignments {
			if assignment.Column == "" || assignment.Expr == nil {
				return fmt.Errorf("named-window assignment %d is invalid", index)
			}
			if _, exists := targetSchema.Field(assignment.Column); !exists {
				return NewError(ErrorUnknownName, fmt.Sprintf("named-window trigger references unknown field %q", assignment.Column))
			}
			if err := e.validateTriggerTargetExpression(definition.input, targetSchema, assignment.Expr, "named-window-field"); err != nil {
				return fmt.Errorf("named-window assignment %q: %w", assignment.Column, err)
			}
		}
	}
	if definition.action != triggerInsertTable && definition.action != triggerUpdateTable && definition.action != triggerDeleteTable && definition.action != triggerDeleteAllTable && definition.action != triggerSelectTable {
		return NewError(ErrorInvalidRule, "unknown named-window trigger action")
	}
	return nil
}

func (e *Environment) validateVariableTriggerAssignments(definition *triggerDefinition) error {
	if len(definition.variableAssignments) == 0 {
		return NewError(ErrorInvalidRule, "variable trigger requires at least one assignment")
	}
	for index, assignment := range definition.variableAssignments {
		if assignment.Name == "" || assignment.Expr == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("variable trigger assignment %d is invalid", index))
		}
		variableDefinition, ok := e.Variable(assignment.Name)
		if !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("variable trigger references unknown variable %q", assignment.Name))
		}
		if variableDefinition.constant {
			return NewError(ErrorState, fmt.Sprintf("variable trigger cannot assign constant variable %q", assignment.Name))
		}
		if err := e.validateExprFields(definition.input, assignment.Expr); err != nil {
			return fmt.Errorf("variable assignment %q: %w", assignment.Name, err)
		}
		if expressionType := assignment.Expr.Type(); variableDefinition.typ != nil && variableDefinition.typ != typeOf[any]() && expressionType != nil && expressionType != typeOf[any]() {
			if !variableDefinition.typ.AssignableTo(expressionType) && !expressionType.AssignableTo(variableDefinition.typ) && !numericTypes(variableDefinition.typ, expressionType) {
				return NewError(ErrorTypeMismatch, fmt.Sprintf("variable %q has type %s, assignment expression has type %s", assignment.Name, variableDefinition.typ, expressionType))
			}
		}
	}
	return nil
}

func (s *Statement) processTrigger(ctx context.Context, now time.Time, event Event, variables map[string]Value) error {
	_, err := s.processTriggerRuntime(ctx, &s.runtime, now, event, variables)
	return err
}

// triggerInputSupportsContainedTraversal reports whether a trigger input can
// be evaluated one contained child at a time without changing the state
// contract of an intermediate operator. Filters and contained expansions are
// stateless for this purpose; windows, patterns and derived streams need the
// existing batch insert path because they own state across the whole delta.
func triggerInputSupportsContainedTraversal(node *streamNode) bool {
	if node == nil {
		return false
	}
	switch node.kind {
	case streamSource, streamNamedWindow, streamTable, streamHistorical, streamMethod:
		return true
	case streamContained, streamFilter:
		return triggerInputSupportsContainedTraversal(node.input)
	default:
		return false
	}
}

func triggerInputContainsContained(node *streamNode) bool {
	for current := node; current != nil; current = current.input {
		if current.kind == streamContained {
			return true
		}
	}
	return false
}

// forEachTriggerCandidate visits new events produced by a contained/filter
// trigger input in evaluation order. Esper's contained-event operator is
// preemptive: an action caused by one child is visible before the next child
// is evaluated. Keeping the traversal callback-based lets the action run at
// the exact point where the child is produced while preserving parent-array
// order and nested contained expansion order.
func (r *statementRuntime) forEachTriggerCandidate(node *streamNode, event Event, now time.Time, visit func(Event) error) error {
	if r == nil || node == nil || visit == nil {
		return NewError(ErrorDependency, "contained trigger traversal is incomplete")
	}
	switch node.kind {
	case streamContained:
		if node.contained == nil || node.contained.property == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("unnest source %q has no contained property", node.sourceName))
		}
		return r.forEachTriggerCandidate(node.input, event, now, func(parent Event) error {
			childSchema, err := r.query.env.sourceSchema(node)
			if err != nil {
				return err
			}
			children, err := expandContainedEvents(r.query.env, childSchema, node.contained, []Event{parent}, now, r.variables)
			if err != nil {
				return err
			}
			for _, child := range children {
				if err := contextErr(r.context()); err != nil {
					return err
				}
				if err := visit(child); err != nil {
					return err
				}
			}
			return nil
		})
	case streamFilter:
		if node.predicate == nil {
			return NewError(ErrorInvalidRule, "contained trigger filter has no predicate")
		}
		return r.forEachTriggerCandidate(node.input, event, now, func(candidate Event) error {
			value := node.predicate.eval(EvalContext{
				Event:                candidate,
				OuterEvent:           candidate,
				ContainedParentEvent: containedParentEvent(candidate),
				Engine:               r.engine,
				Now:                  now,
				Variables:            r.variables,
			})
			if ok, isBool := boolValue(value); !isBool || !ok {
				return nil
			}
			return visit(candidate)
		})
	default:
		delta, err := r.insert(node, event, now)
		if err != nil {
			return err
		}
		for _, candidate := range delta.newEvents {
			if err := contextErr(r.context()); err != nil {
				return err
			}
			if err := visit(candidate); err != nil {
				return err
			}
		}
		return nil
	}
}

func (s *Statement) processTriggerRuntime(ctx context.Context, runtime *statementRuntime, now time.Time, event Event, variables map[string]Value) (ResultBatch, error) {
	definition := s.plan.query.trigger
	if definition == nil || s.engine == nil || runtime == nil {
		return ResultBatch{}, NewError(ErrorDependency, "trigger has no engine or definition")
	}
	runtime.ctx = ctx
	runtime.variables = runtime.withContextVariables(variablesWithEngine(variables, runtime.engine))
	runtime.variables = runtime.withContextProperties(runtime.variables)
	variables = runtime.variables
	result := ResultBatch{Time: now}
	processCandidate := func(candidate Event) error {
		if definition.action == triggerSelectTable {
			var batch ResultBatch
			var selectErr error
			if definition.target == triggerTargetNamedWindow {
				batch, selectErr = executeSelectNamedWindowAction(ctx, s.engine, definition, candidate, now, variables, s.plan.resultSchema)
			} else {
				batch, selectErr = executeSelectTableAction(ctx, s.engine, definition, candidate, now, variables, s.plan.resultSchema)
			}
			if selectErr != nil {
				return selectErr
			}
			result.New = append(result.New, batch.New...)
			result.Old = append(result.Old, batch.Old...)
			return nil
		}
		mutation, err := executeTriggerAction(ctx, s.engine, definition, candidate, now, variables, s, runtime)
		if err != nil {
			return err
		}
		result.Old = append(result.Old, eventsToResults(mutation.oldEvents)...)
		result.New = append(result.New, eventsToResults(mutation.newEvents)...)
		if len(mutation.oldRows) > 0 || len(mutation.newRows) > 0 {
			oldResults, convertErr := tableRowsToResults(s.engine.tables[definition.table], definition.table, mutation.oldRows, now)
			if convertErr != nil {
				return convertErr
			}
			newResults, convertErr := tableRowsToResults(s.engine.tables[definition.table], definition.table, mutation.newRows, now)
			if convertErr != nil {
				return convertErr
			}
			result.Old = append(result.Old, oldResults...)
			result.New = append(result.New, newResults...)
		}
		return nil
	}
	var err error
	if triggerInputContainsContained(definition.input) && triggerInputSupportsContainedTraversal(definition.input) {
		err = runtime.forEachTriggerCandidate(definition.input, event, now, processCandidate)
	} else {
		var delta eventDelta
		delta, err = runtime.insert(definition.input, event, now)
		if err == nil {
			for _, candidate := range delta.newEvents {
				if err = processCandidate(candidate); err != nil {
					break
				}
			}
		}
	}
	if err != nil {
		return ResultBatch{}, err
	}
	result = runtime.applyOutput(s.plan.query.output, result, false, now, s.plan)
	if !result.empty() {
		result.Sequence = runtime.seq.Add(1)
	}
	return result, nil
}

func tableRowsToResults(table *Table, tableName string, rows []TableRow, now time.Time) ([]Result, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	if table == nil {
		return nil, NewError(ErrorUnknownName, fmt.Sprintf("trigger table %q is not available", tableName))
	}
	results := make([]Result, 0, len(rows))
	for _, row := range rows {
		event, err := tableRowEvent(table, tableName, row, now)
		if err != nil {
			return nil, err
		}
		results = append(results, resultEvent(event))
	}
	return results, nil
}

func tableRowEvent(table *Table, tableName string, row TableRow, now time.Time) (Event, error) {
	values := make(map[string]any, len(row.values))
	for name, value := range row.values {
		values[name] = value.Any()
	}
	event, err := newEvent(table.Definition().schema, values, now)
	if err != nil {
		return Event{}, err
	}
	event.typeName = tableName
	return event, nil
}

func eventsToResults(events []Event) []Result {
	if len(events) == 0 {
		return nil
	}
	results := make([]Result, 0, len(events))
	for _, event := range events {
		results = append(results, resultEvent(event))
	}
	return results
}

func executeSelectTableAction(ctx context.Context, engine *Engine, definition *triggerDefinition, event Event, now time.Time, variables map[string]Value, resultSchema Schema) (ResultBatch, error) {
	if engine == nil || definition == nil {
		return ResultBatch{}, NewError(ErrorDependency, "nil table select trigger")
	}
	table := engine.tables[definition.table]
	if table == nil {
		return ResultBatch{}, NewError(ErrorUnknownName, fmt.Sprintf("trigger table %q is not available", definition.table))
	}
	if definition.where != nil {
		rows, err := table.Snapshot(ctx)
		if err != nil {
			return ResultBatch{}, err
		}
		result := ResultBatch{Time: now}
		for _, row := range rows {
			if err := contextErr(ctx); err != nil {
				return ResultBatch{}, err
			}
			tableEvent, err := tableRowEvent(table, definition.table, row, now)
			if err != nil {
				return ResultBatch{}, err
			}
			evaluation := EvalContext{Event: event, Group: []Event{tableEvent}, Now: now, Variables: variables}
			matched, ok := boolValue(definition.where.eval(evaluation))
			if !ok || !matched {
				continue
			}
			values := make([]Value, 0, len(definition.selections))
			for _, selection := range definition.selections {
				values = append(values, selection.Expr.eval(evaluation))
			}
			result.New = append(result.New, resultRow(newRow(resultSchema, values)))
		}
		return result, nil
	}
	evaluation := EvalContext{Event: event, Now: now, Variables: variables}
	keys, err := evaluateTriggerKeys(definition.keys, evaluation)
	if err != nil {
		return ResultBatch{}, err
	}
	row, found, err := table.Get(ctx, keys...)
	if err != nil || !found {
		return ResultBatch{Time: now}, err
	}
	values := make(map[string]any, len(row.values))
	for name, value := range row.values {
		values[name] = value.Any()
	}
	tableEvent, err := newEvent(table.Definition().schema, values, now)
	if err != nil {
		return ResultBatch{}, err
	}
	tableEvent.typeName = definition.table
	projected := make([]Value, 0, len(definition.selections))
	for _, selection := range definition.selections {
		projected = append(projected, selection.Expr.eval(EvalContext{Event: tableEvent, Now: now, Variables: variables}))
	}
	return ResultBatch{Time: now, New: []Result{resultRow(newRow(resultSchema, projected))}}, nil
}

func executeSelectNamedWindowAction(ctx context.Context, engine *Engine, definition *triggerDefinition, event Event, now time.Time, variables map[string]Value, resultSchema Schema) (ResultBatch, error) {
	if engine == nil || definition == nil {
		return ResultBatch{}, NewError(ErrorDependency, "nil named-window select trigger")
	}
	window, ok := engine.namedWindows[definition.table]
	if !ok {
		return ResultBatch{}, NewError(ErrorUnknownName, fmt.Sprintf("trigger named window %q is not available", definition.table))
	}
	target, exists, err := window.scopedForVariables(variables, false)
	if err != nil {
		return ResultBatch{}, err
	}
	if !exists {
		return ResultBatch{Time: now}, nil
	}
	events := snapshotNamedWindowState(target.state)
	result := ResultBatch{Time: now}
	for _, candidate := range events {
		if err := contextErr(ctx); err != nil {
			return ResultBatch{}, err
		}
		evaluation := EvalContext{Event: event, Group: []Event{candidate}, Now: now, Variables: variables}
		if definition.where != nil {
			matched, isBool := boolValue(definition.where.eval(evaluation))
			if !isBool || !matched {
				continue
			}
		}
		if len(definition.selections) == 0 {
			result.New = append(result.New, resultEvent(candidate))
			continue
		}
		values := make([]Value, 0, len(definition.selections))
		for _, selection := range definition.selections {
			values = append(values, selection.Expr.eval(evaluation))
		}
		result.New = append(result.New, resultRow(newRow(resultSchema, values)))
	}
	return result, nil
}

func executeNamedWindowAction(ctx context.Context, engine *Engine, definition *triggerDefinition, event Event, now time.Time, variables map[string]Value, owner *Statement) (tableMutationResult, error) {
	if engine == nil || definition == nil {
		return tableMutationResult{}, NewError(ErrorDependency, "nil named-window trigger")
	}
	window, ok := engine.namedWindows[definition.table]
	if !ok {
		return tableMutationResult{}, NewError(ErrorUnknownName, fmt.Sprintf("trigger named window %q is not available", definition.table))
	}
	createPartition := definition.action == triggerInsertTable || definition.action == triggerMergeTable
	target, exists, err := window.scopedForVariables(variables, createPartition)
	if err != nil {
		return tableMutationResult{}, err
	}
	if !exists {
		return tableMutationResult{}, nil
	}
	schema := target.Definition().schema
	switch definition.action {
	case triggerInsertTable:
		values := evaluateTriggerAssignments(definition.assignments, EvalContext{Event: event, Now: now, Variables: variables})
		underlying, err := mergeSchemaUnderlying(schema, nil, values)
		if err != nil {
			return tableMutationResult{}, err
		}
		delta, err := target.insertWithVariables(now, underlying, variables)
		if err != nil {
			return tableMutationResult{}, err
		}
		if err := engine.queueNamedWindowDeltaLocked(ctx, now, window, delta, variables, owner); err != nil {
			return tableMutationResult{}, err
		}
		return tableMutationResult{newEvents: append([]Event(nil), delta.New...)}, nil
	case triggerMergeTable:
		delta, err := target.mergeWhere(ctx, func(candidate Event) (namedWindowMergeDecision, error) {
			evaluation := EvalContext{Event: event, Group: []Event{candidate}, Now: now, Variables: variables}
			if definition.where != nil {
				matched, ok := boolValue(definition.where.eval(evaluation))
				if !ok || !matched {
					return namedWindowMergeDecision{}, nil
				}
			}
			for _, clause := range definition.merge {
				if !clause.Matched {
					continue
				}
				condition, ok := boolValue(clause.Condition.eval(evaluation))
				if !ok || !condition {
					continue
				}
				if clause.Delete {
					return namedWindowMergeDecision{matched: true, action: namedWindowMergeDelete}, nil
				}
				values := evaluateTriggerAssignments(clause.Assignments, evaluation)
				underlying, mergeErr := mergeSchemaUnderlying(schema, candidate.Underlying(), values)
				if mergeErr != nil {
					return namedWindowMergeDecision{}, mergeErr
				}
				return namedWindowMergeDecision{matched: true, action: namedWindowMergeUpdate, underlying: underlying}, nil
			}
			return namedWindowMergeDecision{matched: true}, nil
		}, func() (any, bool, error) {
			evaluation := EvalContext{Event: event, Now: now, Variables: variables}
			for _, clause := range definition.merge {
				if clause.Matched {
					continue
				}
				condition, ok := boolValue(clause.Condition.eval(evaluation))
				if !ok || !condition {
					continue
				}
				values := evaluateTriggerAssignments(clause.Assignments, evaluation)
				original := any(nil)
				if len(clause.Assignments) == 0 {
					original = event.Underlying()
				}
				underlying, mergeErr := mergeSchemaUnderlying(schema, original, values)
				if mergeErr != nil {
					return nil, false, mergeErr
				}
				return underlying, true, nil
			}
			return nil, false, nil
		})
		if err != nil {
			return tableMutationResult{}, err
		}
		if err := engine.queueNamedWindowDeltaLocked(ctx, now, window, delta, variables, owner); err != nil {
			return tableMutationResult{}, err
		}
		return tableMutationResult{oldEvents: append([]Event(nil), delta.Old...), newEvents: append([]Event(nil), delta.New...)}, nil
	case triggerDeleteTable, triggerDeleteAllTable:
		predicate := definition.where
		if definition.action == triggerDeleteAllTable {
			predicate = Literal(true)
		}
		delta, err := target.deleteWhere(ctx, func(candidate Event) bool {
			if predicate == nil {
				return true
			}
			value := predicate.eval(EvalContext{Event: event, Group: []Event{candidate}, Now: now, Variables: variables})
			matched, ok := boolValue(value)
			return ok && matched
		})
		if err != nil {
			return tableMutationResult{}, err
		}
		if err := engine.queueNamedWindowDeltaLocked(ctx, now, window, delta, variables, owner); err != nil {
			return tableMutationResult{}, err
		}
		return tableMutationResult{oldEvents: append([]Event(nil), delta.Old...)}, nil
	case triggerUpdateTable:
		if definition.where == nil {
			return tableMutationResult{}, NewError(ErrorInvalidRule, "named-window update requires a predicate")
		}
		delta, err := target.updateWhere(ctx, func(candidate Event) bool {
			value := definition.where.eval(EvalContext{Event: event, Group: []Event{candidate}, Now: now, Variables: variables})
			matched, ok := boolValue(value)
			return ok && matched
		}, func(candidate Event) (any, error) {
			evaluation := EvalContext{Event: event, Group: []Event{candidate}, Now: now, Variables: variables}
			values := evaluateTriggerAssignments(definition.assignments, evaluation)
			return mergeSchemaUnderlying(schema, candidate.Underlying(), values)
		})
		if err != nil {
			return tableMutationResult{}, err
		}
		if err := engine.queueNamedWindowDeltaLocked(ctx, now, window, delta, variables, owner); err != nil {
			return tableMutationResult{}, err
		}
		return tableMutationResult{oldEvents: append([]Event(nil), delta.Old...), newEvents: append([]Event(nil), delta.New...)}, nil
	default:
		return tableMutationResult{}, NewError(ErrorInvalidRule, "unknown named-window trigger action")
	}
}

func executeTriggerAction(ctx context.Context, engine *Engine, definition *triggerDefinition, event Event, now time.Time, variables map[string]Value, owner *Statement, runtime *statementRuntime) (tableMutationResult, error) {
	if engine == nil || definition == nil {
		return tableMutationResult{}, NewError(ErrorDependency, "nil table trigger")
	}
	evaluation := EvalContext{Event: event, Now: now, Variables: variables}
	if definition.action == triggerSetVariables {
		return tableMutationResult{}, executeVariableTriggerAction(ctx, engine, definition, evaluation, variables, runtime)
	}
	if definition.target == triggerTargetNamedWindow {
		return executeNamedWindowAction(ctx, engine, definition, event, now, variables, owner)
	}
	table := engine.tables[definition.table]
	if table == nil {
		return tableMutationResult{}, NewError(ErrorUnknownName, fmt.Sprintf("trigger table %q is not available", definition.table))
	}
	if definition.where != nil {
		return executeTableWhereAction(ctx, table, definition, event, now, variables)
	}
	values := make(map[string]any, len(definition.assignments))
	for _, assignment := range definition.assignments {
		value := assignment.Expr.eval(evaluation)
		if value.IsMissing() {
			continue
		}
		values[assignment.Column] = value.Any()
	}
	mutation := tableMutationResult{}
	switch definition.action {
	case triggerInsertTable:
		row, err := table.Insert(ctx, values)
		if err != nil {
			return tableMutationResult{}, err
		}
		mutation.newRows = append(mutation.newRows, row)
		return mutation, nil
	case triggerUpsertTable:
		old, found, err := tableRowForValues(ctx, table, values)
		if err != nil {
			return tableMutationResult{}, err
		}
		row, err := table.Upsert(ctx, values)
		if err != nil {
			return tableMutationResult{}, err
		}
		if found {
			mutation.oldRows = append(mutation.oldRows, old)
		}
		mutation.newRows = append(mutation.newRows, row)
		return mutation, nil
	case triggerUpdateTable:
		keys, err := evaluateTriggerKeys(definition.keys, evaluation)
		if err != nil {
			return tableMutationResult{}, err
		}
		old, found, err := table.Get(ctx, keys...)
		if err != nil {
			return tableMutationResult{}, err
		}
		if !found {
			return tableMutationResult{}, nil
		}
		row, err := table.Update(ctx, keys, values)
		if err != nil {
			return tableMutationResult{}, err
		}
		mutation.oldRows = append(mutation.oldRows, old)
		mutation.newRows = append(mutation.newRows, row)
		return mutation, nil
	case triggerDeleteTable:
		keys, err := evaluateTriggerKeys(definition.keys, evaluation)
		if err != nil {
			return tableMutationResult{}, err
		}
		row, found, err := table.Delete(ctx, keys...)
		if err != nil {
			return tableMutationResult{}, err
		}
		if found {
			mutation.oldRows = append(mutation.oldRows, row)
		}
		return mutation, nil
	case triggerDeleteAllTable:
		rows, err := table.Clear(ctx)
		if err != nil {
			return tableMutationResult{}, err
		}
		mutation.oldRows = append(mutation.oldRows, rows...)
		return mutation, nil
	case triggerMergeTable:
		keys, err := evaluateTriggerKeys(definition.keys, evaluation)
		if err != nil {
			return tableMutationResult{}, err
		}
		old, found, err := table.Get(ctx, keys...)
		if err != nil {
			return tableMutationResult{}, err
		}
		for _, clause := range definition.merge {
			if clause.Matched != found {
				continue
			}
			condition, ok := boolValue(clause.Condition.eval(evaluation))
			if !ok || !condition {
				continue
			}
			if clause.Delete {
				row, deleted, err := table.Delete(ctx, keys...)
				if err != nil {
					return tableMutationResult{}, err
				}
				if deleted {
					mutation.oldRows = append(mutation.oldRows, row)
				}
				return mutation, nil
			}
			values := evaluateTriggerAssignments(clause.Assignments, evaluation)
			if found {
				row, updateErr := table.Update(ctx, keys, values)
				if updateErr != nil {
					return tableMutationResult{}, updateErr
				}
				mutation.oldRows = append(mutation.oldRows, old)
				mutation.newRows = append(mutation.newRows, row)
			} else {
				row, insertErr := table.Insert(ctx, values)
				if insertErr != nil {
					return tableMutationResult{}, insertErr
				}
				mutation.newRows = append(mutation.newRows, row)
			}
			return mutation, nil
		}
		return mutation, nil
	default:
		return tableMutationResult{}, NewError(ErrorInvalidRule, "unknown trigger action")
	}
}

func executeTableWhereAction(ctx context.Context, table *Table, definition *triggerDefinition, event Event, now time.Time, variables map[string]Value) (tableMutationResult, error) {
	if table == nil || definition == nil || definition.where == nil {
		return tableMutationResult{}, NewError(ErrorDependency, "nil table predicate trigger")
	}
	rows, err := table.Snapshot(ctx)
	if err != nil {
		return tableMutationResult{}, err
	}
	mutation := tableMutationResult{}
	for _, row := range rows {
		if err := contextErr(ctx); err != nil {
			return tableMutationResult{}, err
		}
		targetEvent, eventErr := tableRowEvent(table, definition.table, row, now)
		if eventErr != nil {
			return tableMutationResult{}, eventErr
		}
		evaluation := EvalContext{Event: event, Group: []Event{targetEvent}, Now: now, Variables: variables}
		matched, ok := boolValue(definition.where.eval(evaluation))
		if !ok || !matched {
			continue
		}
		keys := tableRowKeys(table.Definition(), row)
		switch definition.action {
		case triggerDeleteTable:
			deleted, found, deleteErr := table.Delete(ctx, keys...)
			if deleteErr != nil {
				return tableMutationResult{}, deleteErr
			}
			if found {
				mutation.oldRows = append(mutation.oldRows, deleted)
			}
		case triggerUpdateTable:
			values := evaluateTriggerAssignments(definition.assignments, evaluation)
			updated, updateErr := table.Update(ctx, keys, values)
			if updateErr != nil {
				return tableMutationResult{}, updateErr
			}
			mutation.oldRows = append(mutation.oldRows, row)
			mutation.newRows = append(mutation.newRows, updated)
		default:
			return tableMutationResult{}, NewError(ErrorInvalidRule, "table predicate trigger requires update or delete")
		}
	}
	return mutation, nil
}

func tableRowKeys(definition TableDefinition, row TableRow) []any {
	keys := make([]any, 0, len(definition.primaryKey))
	for _, name := range definition.primaryKey {
		keys = append(keys, row.Get(name).Any())
	}
	return keys
}

func tableRowForValues(ctx context.Context, table *Table, values map[string]any) (TableRow, bool, error) {
	definition := table.Definition()
	keys := make([]any, 0, len(definition.primaryKey))
	for _, name := range definition.primaryKey {
		value, ok := values[name]
		if !ok {
			return TableRow{}, false, nil
		}
		keys = append(keys, value)
	}
	return table.Get(ctx, keys...)
}

func executeVariableTriggerAction(ctx context.Context, engine *Engine, definition *triggerDefinition, evaluation EvalContext, variables map[string]Value, runtime *statementRuntime) error {
	working := cloneValues(variables)
	if working == nil {
		working = make(map[string]Value)
	}
	assignments := make([]VariableAssignment, 0, len(definition.variableAssignments))
	contextAssignments := make([]VariableAssignment, 0, len(definition.variableAssignments))
	seen := make(map[string]struct{}, len(definition.variableAssignments))
	for _, assignment := range definition.variableAssignments {
		stepEvaluation := evaluation
		stepEvaluation.Variables = working
		value := assignment.Expr.eval(stepEvaluation)
		variableDefinition, ok := engine.env.Variable(assignment.Name)
		if !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("variable %q is not registered", assignment.Name))
		}
		if variableDefinition.context != "" && (runtime == nil || runtime.partitionContextName != variableDefinition.context || runtime.partitionKey == "") {
			return NewError(ErrorState, fmt.Sprintf("context variable %q requires a matching context partition", assignment.Name))
		}
		coerced, err := variableDefinition.coerce(value.Any())
		if err != nil {
			return WrapError(ErrorTypeMismatch, "variable."+assignment.Name, err)
		}
		if coerced == nil {
			working[assignment.Name] = Null()
		} else {
			working[assignment.Name] = Present(coerced)
		}
		if _, exists := seen[assignment.Name]; !exists {
			seen[assignment.Name] = struct{}{}
			assignmentValue := VariableAssignment{Name: assignment.Name, Value: coerced}
			if variableDefinition.context != "" {
				contextAssignments = append(contextAssignments, assignmentValue)
			} else {
				assignments = append(assignments, assignmentValue)
			}
		} else {
			target := &assignments
			if variableDefinition.context != "" {
				target = &contextAssignments
			}
			for index := range *target {
				if (*target)[index].Name == assignment.Name {
					(*target)[index].Value = coerced
					break
				}
			}
		}
	}
	if len(assignments) > 0 {
		if err := engine.setVariablesLocked(ctx, assignments); err != nil {
			return err
		}
	}
	if len(contextAssignments) > 0 {
		if err := engine.setContextVariableValuesLocked(runtime.partitionContextName, runtime.partitionKey, contextAssignments); err != nil {
			return err
		}
	}
	for name, value := range working {
		if _, exists := seen[name]; exists {
			variables[name] = value
		}
	}
	return nil
}

func cloneTableMergeClauses(clauses []TableMergeClause) []TableMergeClause {
	result := append([]TableMergeClause(nil), clauses...)
	for index := range result {
		result[index].Assignments = append([]TableAssignment(nil), result[index].Assignments...)
	}
	return result
}

func validateTriggerAssignments(e *Environment, input *streamNode, table TableDefinition, assignments []TableAssignment) error {
	seen := make(map[string]struct{}, len(assignments))
	for index, assignment := range assignments {
		if assignment.Column == "" || assignment.Expr == nil {
			return fmt.Errorf("assignment %d is invalid", index)
		}
		if _, exists := seen[assignment.Column]; exists {
			return fmt.Errorf("assignment column %q is duplicated", assignment.Column)
		}
		seen[assignment.Column] = struct{}{}
		if _, exists := table.schema.Field(assignment.Column); !exists {
			return fmt.Errorf("assignment references unknown column %q", assignment.Column)
		}
		if err := e.validateExprFields(input, assignment.Expr); err != nil {
			return fmt.Errorf("assignment %q: %w", assignment.Column, err)
		}
	}
	return nil
}

func evaluateTriggerAssignments(assignments []TableAssignment, evaluation EvalContext) map[string]any {
	values := make(map[string]any, len(assignments))
	for _, assignment := range assignments {
		value := assignment.Expr.eval(evaluation)
		if value.IsMissing() {
			continue
		}
		values[assignment.Column] = value.Any()
	}
	return values
}

func evaluateTriggerKeys(keys []Expr, evaluation EvalContext) ([]any, error) {
	values := make([]any, 0, len(keys))
	for _, expression := range keys {
		value := expression.eval(evaluation)
		if value.IsMissing() {
			return nil, NewError(ErrorTypeMismatch, "table trigger key resolved to missing")
		}
		values = append(values, value.Any())
	}
	return values, nil
}
