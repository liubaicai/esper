package esper

import (
	"context"
	"fmt"
	"reflect"
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
	// Index is non-nil for an indexed assignment such as thearray[index].
	// The index is evaluated against the same working target row as the value
	// expression, so ordered assignments can feed later indexes.
	Index Expr
	Expr  Expr
	// Wildcard copies every source field whose name is also declared by the
	// target schema. It is the Go-native equivalent of a merge insert/update
	// wildcard projection while keeping the field matching explicit in the
	// logical plan.
	Wildcard bool
}

func SetColumn(column string, expression Expr) TableAssignment {
	return TableAssignment{Column: strings.TrimSpace(column), Expr: expression}
}

// SetArrayElement updates one element of a slice/array-valued target column.
// It is the Go-native fluent equivalent of an on-trigger assignment such as
// target.items[index] = value. The index and value remain analyzable
// expressions instead of being embedded in a string rule.
func SetArrayElement(column string, index, expression Expr) TableAssignment {
	return TableAssignment{Column: strings.TrimSpace(column), Index: index, Expr: expression}
}

// CopyMatchingFields copies source-event fields into target columns with the
// same name. Fields present only on the source are ignored; target columns
// without a source field retain their normal null/default materialization.
func CopyMatchingFields() TableAssignment {
	return TableAssignment{Wildcard: true}
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

// TableMergeAction is one ordered action in a merge branch. Matched update and
// delete actions are evaluated against the working target row. Table deletes
// terminate the branch; Named Window retains Esper's observed simple
// delete-then-unconditional-update behavior while conditional multi-action
// chains remain terminal on delete.
// InsertTarget actions route side-stream projections from either branch and may read the current target on a matched branch;
// InsertIntoTarget is reserved for a not-matched insert into the current
// target. Assignments in a later action see earlier updates, while
// InitialTableField/InitialNamedWindowField remain bound to the action-chain
// snapshot.
type TableMergeAction struct {
	Condition        Expr
	Assignments      []TableAssignment
	Delete           bool
	InsertTarget     string
	InsertSelections []Selection
	InsertIntoTarget bool
}

// ThenUpdate creates one conditional update action for a matched merge
// branch.
func ThenUpdate(condition Expr, assignments ...TableAssignment) TableMergeAction {
	return TableMergeAction{Condition: condition, Assignments: append([]TableAssignment(nil), assignments...)}
}

// ThenDelete creates one conditional delete action for a matched merge
// branch. For Table targets it terminates the action chain. Named Window
// targets also terminate conditional multi-action chains, while preserving
// Esper's observed simple delete-then-unconditional-update behavior.
func ThenDelete(condition Expr) TableMergeAction {
	return TableMergeAction{Condition: condition, Delete: true}
}

// ThenInsertInto creates an unconditional merge action that routes a named
// projection into another registered event type. On a matched branch the
// projection may read the working target row; use ThenInsertIntoWhen when the
// insert has a predicate.
func ThenInsertInto(target string, selections ...Selection) TableMergeAction {
	return ThenInsertIntoWhen(Literal(true), target, selections...)
}

// ThenInsertIntoWhen creates a conditional side-stream route action.
func ThenInsertIntoWhen(condition Expr, target string, selections ...Selection) TableMergeAction {
	return TableMergeAction{
		Condition:        condition,
		InsertTarget:     strings.TrimSpace(target),
		InsertSelections: append([]Selection(nil), selections...),
	}
}

// ThenInsertIntoTarget creates an unconditional not-matched action that
// inserts into the Table or Named Window being merged. It is useful when the
// same branch also emits one or more side-stream events.
func ThenInsertIntoTarget(assignments ...TableAssignment) TableMergeAction {
	return ThenInsertIntoTargetWhen(Literal(true), assignments...)
}

// ThenInsertIntoTargetWhen creates a conditional insert into the current
// merge target.
func ThenInsertIntoTargetWhen(condition Expr, assignments ...TableAssignment) TableMergeAction {
	return TableMergeAction{Condition: condition, Assignments: append([]TableAssignment(nil), assignments...), InsertIntoTarget: true}
}

// TableMergeClause is one ordered matched or not-matched branch of a table
// merge. The legacy Condition/Assignments/Delete fields represent one action.
// Either branch can instead use Actions to express Esper's repeated
// update/delete/insert action chain.
type TableMergeClause struct {
	Matched     bool
	Condition   Expr
	Assignments []TableAssignment
	Delete      bool
	Actions     []TableMergeAction
}

// NamedWindowMergeClause uses the same ordered branch contract as table
// merge. The alias keeps the fluent clause constructors reusable while the
// target-specific method makes the resulting plan explicit.
type NamedWindowMergeClause = TableMergeClause

func WhenMatched(condition Expr, assignments ...TableAssignment) TableMergeClause {
	return TableMergeClause{Matched: true, Condition: condition, Assignments: append([]TableAssignment(nil), assignments...)}
}

// WhenMatchedAny creates an unconditional matched branch. It is the fluent
// equivalent of Java Esper's "when matched then update" form.
func WhenMatchedAny(assignments ...TableAssignment) TableMergeClause {
	return WhenMatched(Literal(true), assignments...)
}

func WhenNotMatched(condition Expr, assignments ...TableAssignment) TableMergeClause {
	return TableMergeClause{Condition: condition, Assignments: append([]TableAssignment(nil), assignments...)}
}

// WhenNotMatchedAny creates an unconditional not-matched branch. It is the
// fluent equivalent of Java Esper's "when not matched then insert" form.
func WhenNotMatchedAny(assignments ...TableAssignment) TableMergeClause {
	return WhenNotMatched(Literal(true), assignments...)
}

func WhenMatchedDelete(condition Expr) TableMergeClause {
	return TableMergeClause{Matched: true, Condition: condition, Delete: true}
}

// WhenMatchedDeleteAny creates an unconditional matched delete branch.
func WhenMatchedDeleteAny() TableMergeClause {
	return WhenMatchedDelete(Literal(true))
}

// WhenMatchedActions creates a matched merge branch with an ordered action
// chain. Keeping the action list explicit makes the evaluation order visible
// in the logical plan and avoids embedding EPL text in the Go API.
func WhenMatchedActions(actions ...TableMergeAction) TableMergeClause {
	copyActions := make([]TableMergeAction, len(actions))
	copy(copyActions, actions)
	return TableMergeClause{Matched: true, Actions: copyActions}
}

// WhenNotMatchedActions creates a not-matched branch with ordered insert
// actions. The actions may route side-stream projections and/or insert the
// final row into the current Table or Named Window.
func WhenNotMatchedActions(actions ...TableMergeAction) TableMergeClause {
	copyActions := make([]TableMergeAction, len(actions))
	copy(copyActions, actions)
	return TableMergeClause{Actions: copyActions}
}

type triggerDefinition struct {
	input      *streamNode
	table      string
	moduleName string
	target     triggerTargetKind
	action     triggerActionKind
	// onDemand distinguishes fire-and-forget target-row evaluation from a
	// live on-trigger.  Live triggers keep the incoming event as the outer
	// scope; FAF mutations use the candidate target row as OuterEvent so a
	// correlated subquery can reference it through OuterField.
	onDemand            bool
	assignments         []TableAssignment
	keys                []Expr
	where               Expr
	merge               []TableMergeClause
	variableAssignments []VariableAssignmentExpr
	selections          []Selection
	// contextDefinition/contextPartitionKey are populated only by an
	// on-demand context mutation. Live trigger definitions use the ordinary
	// statement runtime partition scope instead. Table storage is global in the
	// Go runtime, so the context pair lets a target-row scan apply the same
	// partition predicate that a context-bound Table has in Esper.
	contextDefinition   *ContextDefinition
	contextPartitionKey string
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
// correspond to the table primary-key columns. For a table without primary
// keys, pass a nil or empty key slice to use its single-row no-where state.
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

// MergeInsertIntoTable is the concise insertion-only merge form. Existing
// rows that match the supplied primary-key expressions are left unchanged;
// only the not-matched branch evaluates assignments and inserts a row.
func (s TriggerStream[T]) MergeInsertIntoTable(table string, keys []Expr, assignments ...TableAssignment) TriggerQuery {
	return s.MergeIntoTableWhen(table, keys, WhenNotMatchedAny(assignments...))
}

// MergeIntoNamedWindowWhen applies ordered matched and not-matched branches
// to events in a named window. The match expression may read the incoming
// event through Field and the candidate window event through NamedWindowField.
// A nil match expression means that no explicit match predicate was supplied.
// When matched branches exist, every existing target event is treated as
// matched; when the rule contains only not-matched actions, each trigger event
// reaches those actions. Use Literal(false) when an explicit always-unmatched
// predicate is clearer than the no-matched-branch form.
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

// MergeInsertIntoNamedWindow is the insertion-only named-window merge form.
// A matching target event is retained and produces no mutation; a trigger
// event with no matching target is materialized from assignments.
func (s TriggerStream[T]) MergeInsertIntoNamedWindow(window string, match Expression[bool], assignments ...TableAssignment) TriggerQuery {
	return s.MergeIntoNamedWindowWhen(window, match, WhenNotMatchedAny(assignments...))
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
			if clause.Actions != nil {
				actions := make([]string, 0, len(clause.Actions))
				for _, action := range clause.Actions {
					condition := "<nil>"
					if action.Condition != nil {
						condition = action.Condition.Description()
					}
					if action.InsertTarget != "" {
						selections := make([]string, 0, len(action.InsertSelections))
						for _, selection := range action.InsertSelections {
							selections = append(selections, selection.description())
						}
						actions = append(actions, "insert("+condition+")->"+action.InsertTarget+"["+strings.Join(selections, ",")+"]")
					} else if action.InsertIntoTarget {
						actions = append(actions, "insert-target("+condition+")->"+describeTableAssignments(action.Assignments))
					} else if action.Delete {
						actions = append(actions, "delete("+condition+")")
					} else {
						actions = append(actions, "update("+condition+")->"+describeTableAssignments(action.Assignments))
					}
				}
				clauses = append(clauses, branch+"-actions["+strings.Join(actions, "|")+"]")
				continue
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
		if assignment.Wildcard {
			parts = append(parts, "copy-matching-fields")
			continue
		}
		expression := "<nil>"
		if assignment.Expr != nil {
			expression = assignment.Expr.Description()
		}
		column := assignment.Column
		if assignment.Index != nil {
			column += "[" + assignment.Index.Description() + "]"
		}
		parts = append(parts, column+"="+expression)
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
	table, ok := e.TableInModule(definition.moduleName, definition.table)
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
	for index, assignment := range definition.assignments {
		if err := validateTriggerAssignment(e, definition.input, table.schema, assignment, "table-field"); err != nil {
			return fmt.Errorf("table trigger assignment %d: %w", index, err)
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
		if len(table.primaryKey) == 0 {
			if len(definition.keys) != 0 {
				return NewError(ErrorInvalidRule, "table merge target has no primary-key columns; omit key expressions")
			}
		} else if len(definition.keys) != len(table.primaryKey) {
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
		for index, clause := range definition.merge {
			if clause.Actions != nil && (clause.Condition != nil || len(clause.Assignments) != 0 || clause.Delete) {
				return fmt.Errorf("table merge clause %d action chain cannot be combined with legacy clause fields", index)
			}
			actions := tableMergeClauseActions(clause)
			if len(actions) == 0 {
				return fmt.Errorf("table merge clause %d requires at least one action", index)
			}
			for actionIndex, action := range actions {
				if action.Condition == nil || action.Condition.Type() != typeOf[bool]() {
					return fmt.Errorf("table merge clause %d action %d requires a bool condition", index, actionIndex)
				}
				if !clause.Matched && clause.Actions != nil && action.InsertTarget == "" && !action.InsertIntoTarget && !action.Delete {
					return fmt.Errorf("table merge clause %d action %d not-matched branch requires an insert action", index, actionIndex)
				}
				if action.InsertTarget != "" || action.InsertIntoTarget {
					if action.InsertIntoTarget && clause.Matched {
						return fmt.Errorf("table merge clause %d action %d target insert must be not-matched", index, actionIndex)
					}
					if action.InsertTarget != "" && action.InsertIntoTarget {
						return fmt.Errorf("table merge clause %d action %d cannot target an event type and the merge target", index, actionIndex)
					}
					var targetFields []string
					if !clause.Matched {
						action.Condition.node().referencedTargetFields("table-field", &targetFields)
						if len(targetFields) > 0 {
							return fmt.Errorf("table merge clause %d not-matched condition cannot reference table fields", index)
						}
					}
					if action.InsertTarget != "" {
						if action.Delete || len(action.Assignments) != 0 {
							return fmt.Errorf("table merge clause %d action %d event insert cannot delete or assign target columns", index, actionIndex)
						}
						targetKind := ""
						if clause.Matched {
							targetKind = "table-field"
						}
						if err := validateMergeInsertSelectionsForTarget(e, definition.input, action.InsertTarget, action.InsertSelections, targetKind, table.schema); err != nil {
							return fmt.Errorf("table merge clause %d action %d: %w", index, actionIndex, err)
						}
					} else {
						if action.Delete || len(action.InsertSelections) != 0 {
							return fmt.Errorf("table merge clause %d action %d target insert has an invalid action shape", index, actionIndex)
						}
						if len(action.Assignments) == 0 {
							return fmt.Errorf("table merge clause %d action %d target insert requires assignments", index, actionIndex)
						}
						for _, assignment := range action.Assignments {
							if !assignment.Wildcard {
								targetFields = nil
								assignment.Expr.node().referencedTargetFields("table-field", &targetFields)
								if assignment.Index != nil {
									assignment.Index.node().referencedTargetFields("table-field", &targetFields)
								}
								if len(targetFields) > 0 {
									return fmt.Errorf("table merge clause %d not-matched assignment cannot reference table fields", index)
								}
							}
						}
						if err := validateTriggerAssignmentsWithTarget(e, definition.input, table, action.Assignments, ""); err != nil {
							return fmt.Errorf("table merge clause %d action %d: %w", index, actionIndex, err)
						}
					}
					var conditionErr error
					if clause.Matched {
						conditionErr = e.validateTriggerTargetExpression(definition.input, table.schema, action.Condition, "table-field")
					} else {
						conditionErr = e.validateExprFields(definition.input, action.Condition)
					}
					if conditionErr != nil {
						return fmt.Errorf("table merge clause %d action %d condition: %w", index, actionIndex, conditionErr)
					}
					continue
				}
				if action.Delete && !clause.Matched {
					return fmt.Errorf("table merge delete clause %d must be matched", index)
				}
				if action.Delete && len(action.Assignments) > 0 {
					return fmt.Errorf("table merge clause %d action %d delete cannot assign columns", index, actionIndex)
				}
				if !action.Delete && len(action.Assignments) == 0 {
					return fmt.Errorf("table merge clause %d action %d requires assignments", index, actionIndex)
				}
				if !clause.Matched {
					var targetFields []string
					action.Condition.node().referencedTargetFields("table-field", &targetFields)
					if len(targetFields) > 0 {
						return fmt.Errorf("table merge clause %d not-matched condition cannot reference table fields", index)
					}
					for _, assignment := range action.Assignments {
						if !assignment.Wildcard {
							targetFields = nil
							assignment.Expr.node().referencedTargetFields("table-field", &targetFields)
							if assignment.Index != nil {
								assignment.Index.node().referencedTargetFields("table-field", &targetFields)
							}
							if len(targetFields) > 0 {
								return fmt.Errorf("table merge clause %d not-matched assignment cannot reference table fields", index)
							}
						}
					}
				}
				assignmentTargetKind := ""
				if clause.Matched {
					assignmentTargetKind = "table-field"
				}
				if err := validateTriggerAssignmentsWithTarget(e, definition.input, table, action.Assignments, assignmentTargetKind); err != nil {
					return fmt.Errorf("table merge clause %d action %d: %w", index, actionIndex, err)
				}
				var conditionErr error
				if clause.Matched {
					conditionErr = e.validateTriggerTargetExpression(definition.input, table.schema, action.Condition, "table-field")
				} else {
					conditionErr = e.validateExprFields(definition.input, action.Condition)
				}
				if conditionErr != nil {
					return fmt.Errorf("table merge clause %d action %d condition: %w", index, actionIndex, conditionErr)
				}
			}
		}
	}
	return nil
}

func (e *Environment) validateNamedWindowTrigger(definition *triggerDefinition) error {
	window, ok := e.NamedWindowInModule(definition.moduleName, definition.table)
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
			if clause.Actions != nil && (clause.Condition != nil || len(clause.Assignments) != 0 || clause.Delete) {
				return fmt.Errorf("named-window merge clause %d action chain cannot be combined with legacy clause fields", index)
			}
			actions := tableMergeClauseActions(clause)
			if len(actions) == 0 {
				return fmt.Errorf("named-window merge clause %d requires at least one action", index)
			}
			for actionIndex, action := range actions {
				if action.Condition == nil || action.Condition.Type() != typeOf[bool]() {
					return fmt.Errorf("named-window merge clause %d action %d requires a bool condition", index, actionIndex)
				}
				if !clause.Matched && clause.Actions != nil && action.InsertTarget == "" && !action.InsertIntoTarget && !action.Delete {
					return fmt.Errorf("named-window merge clause %d action %d not-matched branch requires an insert action", index, actionIndex)
				}
				if action.InsertTarget != "" || action.InsertIntoTarget {
					if action.InsertIntoTarget && clause.Matched {
						return fmt.Errorf("named-window merge clause %d action %d target insert must be not-matched", index, actionIndex)
					}
					if action.InsertTarget != "" && action.InsertIntoTarget {
						return fmt.Errorf("named-window merge clause %d action %d cannot target an event type and the merge target", index, actionIndex)
					}
					var targetFields []string
					if !clause.Matched {
						action.Condition.node().referencedTargetFields("named-window-field", &targetFields)
						if len(targetFields) > 0 {
							return fmt.Errorf("named-window merge clause %d not-matched condition cannot reference named-window fields", index)
						}
					}
					if action.InsertTarget != "" {
						if action.Delete || len(action.Assignments) != 0 {
							return fmt.Errorf("named-window merge clause %d action %d event insert cannot delete or assign target columns", index, actionIndex)
						}
						targetKind := ""
						if clause.Matched {
							targetKind = "named-window-field"
						}
						if err := validateMergeInsertSelectionsForTarget(e, definition.input, action.InsertTarget, action.InsertSelections, targetKind, targetSchema); err != nil {
							return fmt.Errorf("named-window merge clause %d action %d: %w", index, actionIndex, err)
						}
					} else {
						if action.Delete || len(action.InsertSelections) != 0 {
							return fmt.Errorf("named-window merge clause %d action %d target insert has an invalid action shape", index, actionIndex)
						}
						for _, assignment := range action.Assignments {
							if !assignment.Wildcard {
								targetFields = nil
								assignment.Expr.node().referencedTargetFields("named-window-field", &targetFields)
								if assignment.Index != nil {
									assignment.Index.node().referencedTargetFields("named-window-field", &targetFields)
								}
								if len(targetFields) > 0 {
									return fmt.Errorf("named-window merge clause %d not-matched assignment cannot reference named-window fields", index)
								}
							}
						}
						for assignmentIndex, assignment := range action.Assignments {
							if err := validateTriggerAssignment(e, definition.input, targetSchema, assignment, ""); err != nil {
								return fmt.Errorf("named-window merge clause %d action %d assignment %d: %w", index, actionIndex, assignmentIndex, err)
							}
						}
					}
					var conditionErr error
					if clause.Matched {
						conditionErr = e.validateTriggerTargetExpression(definition.input, targetSchema, action.Condition, "named-window-field")
					} else {
						conditionErr = e.validateExprFields(definition.input, action.Condition)
					}
					if conditionErr != nil {
						return fmt.Errorf("named-window merge clause %d action %d condition: %w", index, actionIndex, conditionErr)
					}
					continue
				}
				if !clause.Matched {
					var targetFields []string
					action.Condition.node().referencedTargetFields("named-window-field", &targetFields)
					if len(targetFields) > 0 {
						return fmt.Errorf("named-window merge clause %d not-matched condition cannot reference named-window fields", index)
					}
				}
				if action.Delete && !clause.Matched {
					return fmt.Errorf("named-window merge clause %d cannot delete on not-matched", index)
				}
				if !action.Delete && clause.Matched && len(action.Assignments) == 0 {
					return fmt.Errorf("named-window merge clause %d action %d requires assignments", index, actionIndex)
				}
				if action.Delete && len(action.Assignments) != 0 {
					return fmt.Errorf("named-window merge clause %d action %d delete cannot have assignments", index, actionIndex)
				}
				if err := e.validateTriggerTargetExpression(definition.input, targetSchema, action.Condition, "named-window-field"); err != nil {
					return fmt.Errorf("named-window merge clause %d action %d condition: %w", index, actionIndex, err)
				}
				for assignmentIndex, assignment := range action.Assignments {
					if !clause.Matched && !assignment.Wildcard {
						var targetFields []string
						assignment.Expr.node().referencedTargetFields("named-window-field", &targetFields)
						if assignment.Index != nil {
							assignment.Index.node().referencedTargetFields("named-window-field", &targetFields)
						}
						if len(targetFields) > 0 {
							return fmt.Errorf("named-window merge clause %d not-matched assignment cannot reference named-window fields", index)
						}
					}
					if err := validateTriggerAssignment(e, definition.input, targetSchema, assignment, "named-window-field"); err != nil {
						return fmt.Errorf("named-window merge clause %d action %d assignment %d: %w", index, actionIndex, assignmentIndex, err)
					}
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
			if err := validateTriggerAssignment(e, definition.input, targetSchema, assignment, "named-window-field"); err != nil {
				return fmt.Errorf("named-window assignment %d: %w", index, err)
			}
		}
	}
	if definition.action != triggerInsertTable && definition.action != triggerUpdateTable && definition.action != triggerDeleteTable && definition.action != triggerDeleteAllTable && definition.action != triggerSelectTable {
		return NewError(ErrorInvalidRule, "unknown named-window trigger action")
	}
	return nil
}
func validateMergeInsertSelections(e *Environment, input *streamNode, target string, selections []Selection) error {
	return validateMergeInsertSelectionsForTarget(e, input, target, selections, "", Schema{})
}

func validateMergeInsertSelectionsForTarget(e *Environment, input *streamNode, target string, selections []Selection, targetKind string, targetScope Schema) error {
	if e == nil {
		return NewError(ErrorDependency, "merge insert requires an environment")
	}
	targetSchema, ok := e.Schema(target)
	if !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("merge insert references unknown event type %q", target))
	}
	if len(selections) == 0 {
		return NewError(ErrorInvalidRule, "merge insert requires at least one projection")
	}
	seen := make(map[string]struct{}, len(selections))
	for index, selection := range selections {
		name := strings.TrimSpace(selection.Name)
		if name == "" || selection.Expr == nil {
			return fmt.Errorf("merge insert projection %d requires a name and expression", index)
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("merge insert projection duplicates alias %q", name)
		}
		seen[name] = struct{}{}
		field, exists := targetSchema.Field(name)
		if !exists {
			if targetSchema.Kind() != SchemaMap && targetSchema.Kind() != SchemaJSON && targetSchema.Kind() != SchemaXML && !targetSchema.IsVariantAny() {
				return fmt.Errorf("merge insert projection %q is not present in event type %q", name, target)
			}
		} else if field.Type != nil && field.Type != typeOf[any]() && selection.Expr.Type() != nil &&
			!field.Type.AssignableTo(selection.Expr.Type()) && !selection.Expr.Type().AssignableTo(field.Type) && !numericTypes(field.Type, selection.Expr.Type()) {
			return fmt.Errorf("merge insert projection %q has type %s, target expects %s", name, selection.Expr.Type(), field.Type)
		}
		var expressionErr error
		if targetKind == "" {
			expressionErr = e.validateExprFields(input, selection.Expr)
		} else {
			expressionErr = e.validateTriggerTargetExpression(input, targetScope, selection.Expr, targetKind)
		}
		if expressionErr != nil {
			return fmt.Errorf("merge insert projection %q: %w", name, expressionErr)
		}
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
			oldResults, convertErr := tableRowsToResults(s.engine.tables[catalogKey(definition.moduleName, definition.table)], definition.table, mutation.oldRows, now)
			if convertErr != nil {
				return convertErr
			}
			newResults, convertErr := tableRowsToResults(s.engine.tables[catalogKey(definition.moduleName, definition.table)], definition.table, mutation.newRows, now)
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
	return tableEventFromValues(table, tableName, values, now)
}

// tableNullRowEvent creates the target-row scope used by a primary-key lookup
// that has no matching row.  A table selector still emits its projected row in
// this case; every table column must therefore be present-but-null rather than
// missing, so chained methods preserve Esper's null result semantics. The
// triggering event is retained as the explicit outer scope for expressions
// that need both sides of the lookup.
func tableNullRowEvent(table *Table, tableName string, now time.Time) (Event, error) {
	values := make(map[string]any, len(table.Definition().Columns()))
	for _, column := range table.Definition().Columns() {
		values[column.Name] = nil
	}
	return tableEventFromValues(table, tableName, values, now)
}

func tableEventFromValues(table *Table, tableName string, values map[string]any, now time.Time) (Event, error) {
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
	table := engine.tables[catalogKey(definition.moduleName, definition.table)]
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
	if err != nil {
		return ResultBatch{Time: now}, err
	}
	var tableEvent Event
	if found {
		tableEvent, err = tableRowEvent(table, definition.table, row, now)
	} else {
		tableEvent, err = tableNullRowEvent(table, definition.table, now)
	}
	if err != nil {
		return ResultBatch{}, err
	}
	projected := make([]Value, 0, len(definition.selections))
	evaluation = EvalContext{
		Event:      tableEvent,
		OuterEvent: event,
		Group:      []Event{tableEvent},
		Now:        now,
		Variables:  variables,
	}
	for _, selection := range definition.selections {
		projected = append(projected, selection.Expr.eval(evaluation))
	}
	return ResultBatch{Time: now, New: []Result{resultRow(newRow(resultSchema, projected))}}, nil
}

func executeSelectNamedWindowAction(ctx context.Context, engine *Engine, definition *triggerDefinition, event Event, now time.Time, variables map[string]Value, resultSchema Schema) (ResultBatch, error) {
	if engine == nil || definition == nil {
		return ResultBatch{}, NewError(ErrorDependency, "nil named-window select trigger")
	}
	window, ok := engine.namedWindows[catalogKey(definition.moduleName, definition.table)]
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

// evaluateTableMergeActions evaluates a matched action chain against a
// detached working target. The caller commits the returned final value once,
// which keeps the old/new lifecycle equivalent to one Esper merge action even
// when several conditional actions ran internally. Table targets terminate on
// delete. Named Window targets preserve the one observed Java exception where
// a two-action delete followed by an unconditional update updates the row;
// conditional multi-action chains still terminate on delete.
func evaluateTableMergeActions(engine *Engine, schema Schema, original any, evaluation EvalContext, actions []TableMergeAction, now time.Time, deleteTerminates bool) (underlying any, matched, deleted, updated bool, err error) {
	initialGroup := append([]Event(nil), evaluation.InitialGroup...)
	if len(initialGroup) == 0 {
		initialGroup = append([]Event(nil), evaluation.Group...)
	}
	workingUnderlying := original
	var workingEvent Event
	hasWorkingEvent := false
	if len(initialGroup) > 0 {
		workingEvent = initialGroup[0]
		hasWorkingEvent = true
	} else if original != nil {
		workingEvent, err = newEvent(schema, original, now)
		if err != nil {
			return nil, false, false, false, err
		}
		hasWorkingEvent = true
	}

	for actionIndex, action := range actions {
		step := evaluation
		if hasWorkingEvent {
			step.Group = []Event{workingEvent}
		} else {
			step.Group = nil
		}
		step.InitialGroup = append([]Event(nil), initialGroup...)
		condition, ok := boolValue(action.Condition.eval(step))
		if !ok || !condition {
			continue
		}
		matched = true
		if action.InsertTarget != "" {
			if err := queueMergeInsertEvent(engine, action, step, now); err != nil {
				return nil, false, false, false, err
			}
			continue
		}
		if action.InsertIntoTarget {
			return nil, false, false, false, NewError(ErrorInvalidRule, "matched merge action cannot insert into the merge target")
		}
		if action.Delete {
			deleted = true
			updated = false
			if deleteTerminates || !canContinueAfterNamedWindowDelete(actions, actionIndex) {
				return workingUnderlying, true, true, false, nil
			}
			continue
		}
		values, assignmentErr := evaluateTriggerAssignmentsForTarget(schema, workingUnderlying, action.Assignments, step, now)
		if assignmentErr != nil {
			return nil, false, false, false, assignmentErr
		}
		workingUnderlying, err = mergeSchemaUnderlying(schema, workingUnderlying, values)
		if err != nil {
			return nil, false, false, false, err
		}
		workingEvent, err = newEvent(schema, workingUnderlying, now)
		if err != nil {
			return nil, false, false, false, err
		}
		hasWorkingEvent = true
		deleted = false
		updated = true
	}
	return workingUnderlying, matched, false, updated, nil
}

func canContinueAfterNamedWindowDelete(actions []TableMergeAction, deleteIndex int) bool {
	if deleteIndex != 0 || len(actions) != 2 {
		return false
	}
	next := actions[1]
	if next.Delete || next.InsertTarget != "" || next.InsertIntoTarget || len(next.Assignments) == 0 || next.Condition == nil {
		return false
	}
	node := next.Condition.node()
	return node != nil && node.kind == "literal" && node.description == "true"
}

func materializeMergeInsertEvent(engine *Engine, action TableMergeAction, evaluation EvalContext, now time.Time) (Event, error) {
	if engine == nil || engine.env == nil {
		return Event{}, NewError(ErrorDependency, "merge insert has no environment")
	}
	targetSchema, ok := engine.env.Schema(action.InsertTarget)
	if !ok {
		return Event{}, NewError(ErrorUnknownName, fmt.Sprintf("merge insert references unknown event type %q", action.InsertTarget))
	}
	values := make(map[string]any, len(action.InsertSelections))
	for _, selection := range action.InsertSelections {
		name := strings.TrimSpace(selection.Name)
		value := selection.Expr.eval(evaluation)
		if value.IsMissing() {
			values[name] = nil
		} else {
			values[name] = value.Any()
		}
	}
	underlying, err := projectMapToSchema(targetSchema, values)
	if err != nil {
		return Event{}, err
	}
	event, err := newEvent(targetSchema, underlying, now)
	if err != nil {
		return Event{}, WrapError(ErrorTypeMismatch, "merge-insert."+action.InsertTarget, err)
	}
	event.streamType = action.InsertTarget
	return event, nil
}

func queueMergeInsertEvent(engine *Engine, action TableMergeAction, evaluation EvalContext, now time.Time) error {
	event, err := materializeMergeInsertEvent(engine, action, evaluation, now)
	if err != nil {
		return err
	}
	engine.pendingRoutedEvents = append(engine.pendingRoutedEvents, event)
	return nil
}

func executeTableMergeNotMatchedActions(ctx context.Context, engine *Engine, table *Table, definition *triggerDefinition, event Event, evaluation EvalContext, actions []TableMergeAction, now time.Time) (tableMutationResult, bool, error) {
	if engine == nil || table == nil || definition == nil {
		return tableMutationResult{}, false, NewError(ErrorDependency, "nil table merge insert action")
	}
	mutation := tableMutationResult{}
	executed := false
	for _, action := range actions {
		condition, ok := boolValue(action.Condition.eval(evaluation))
		if !ok || !condition {
			continue
		}
		executed = true
		if action.InsertTarget != "" {
			if err := queueMergeInsertEvent(engine, action, evaluation, now); err != nil {
				return tableMutationResult{}, false, err
			}
			continue
		}
		if !action.InsertIntoTarget {
			return tableMutationResult{}, false, NewError(ErrorInvalidRule, "not-matched merge action must insert into an event type or the merge target")
		}
		values, assignmentErr := evaluateTriggerAssignmentsForTarget(table.Definition().schema, nil, action.Assignments, evaluation, now)
		if assignmentErr != nil {
			return tableMutationResult{}, false, assignmentErr
		}
		row, insertErr := table.Insert(ctx, values)
		if insertErr != nil {
			return tableMutationResult{}, false, insertErr
		}
		mutation.newRows = append(mutation.newRows, row)
	}
	return mutation, executed, nil
}

func evaluateNamedWindowMergeNotMatchedActions(engine *Engine, schema Schema, event Event, evaluation EvalContext, actions []TableMergeAction, now time.Time) (any, bool, bool, error) {
	executed := false
	var targetUnderlying any
	shouldInsert := false
	for _, action := range actions {
		condition, ok := boolValue(action.Condition.eval(evaluation))
		if !ok || !condition {
			continue
		}
		executed = true
		if action.InsertTarget != "" {
			if err := queueMergeInsertEvent(engine, action, evaluation, now); err != nil {
				return nil, false, false, err
			}
			continue
		}
		if !action.InsertIntoTarget {
			return nil, false, false, NewError(ErrorInvalidRule, "not-matched merge action must insert into an event type or the merge target")
		}
		values, assignmentErr := evaluateTriggerAssignmentsForTarget(schema, nil, action.Assignments, evaluation, now)
		if assignmentErr != nil {
			return nil, false, false, assignmentErr
		}
		if len(action.Assignments) == 0 {
			targetUnderlying = event.Underlying()
		} else {
			var mergeErr error
			targetUnderlying, mergeErr = mergeSchemaUnderlying(schema, nil, values)
			if mergeErr != nil {
				return nil, false, false, mergeErr
			}
		}
		shouldInsert = true
	}
	return targetUnderlying, shouldInsert, executed, nil
}

func executeNamedWindowAction(ctx context.Context, engine *Engine, definition *triggerDefinition, event Event, now time.Time, variables map[string]Value, owner *Statement) (tableMutationResult, error) {
	if engine == nil || definition == nil {
		return tableMutationResult{}, NewError(ErrorDependency, "nil named-window trigger")
	}
	window, ok := engine.namedWindows[catalogKey(definition.moduleName, definition.table)]
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
		values, assignmentErr := evaluateTriggerAssignmentsForTarget(schema, nil, definition.assignments, EvalContext{Engine: engine, Event: event, Now: now, Variables: variables}, now)
		if assignmentErr != nil {
			return tableMutationResult{}, assignmentErr
		}
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
		hasMatchedClause := false
		for _, clause := range definition.merge {
			if clause.Matched {
				hasMatchedClause = true
				break
			}
		}
		delta, err := target.mergeWhere(ctx, func(candidate Event) (namedWindowMergeDecision, error) {
			// Esper's insert-only "merge Window insert ..." form has no
			// matched branch. With no where clause, every incoming event is
			// therefore an unmatched trigger, including later events after a
			// keep-all target has already received earlier events. Treating the
			// existing rows as matched here would silently drop all but the
			// first child of a contained expansion.
			if definition.where == nil && !hasMatchedClause {
				return namedWindowMergeDecision{}, nil
			}
			evaluation := EvalContext{Engine: engine, Event: event, Group: []Event{candidate}, Now: now, Variables: variables}
			if definition.onDemand {
				evaluation.OuterEvent = candidate
			}
			if definition.where != nil {
				matched, ok := boolValue(definition.where.eval(evaluation))
				if !ok || !matched {
					return namedWindowMergeDecision{}, nil
				}
			}
			evaluation.InitialGroup = []Event{candidate}
			for _, clause := range definition.merge {
				if !clause.Matched {
					continue
				}
				underlying, actionMatched, deleted, updated, actionErr := evaluateTableMergeActions(engine, schema, candidate.Underlying(), evaluation, tableMergeClauseActions(clause), now, false)
				if actionErr != nil {
					return namedWindowMergeDecision{}, actionErr
				}
				if !actionMatched {
					continue
				}
				if deleted {
					return namedWindowMergeDecision{matched: true, action: namedWindowMergeDelete}, nil
				}
				if !updated {
					return namedWindowMergeDecision{matched: true}, nil
				}
				return namedWindowMergeDecision{matched: true, action: namedWindowMergeUpdate, underlying: underlying}, nil
			}
			return namedWindowMergeDecision{matched: true}, nil
		}, func() (any, bool, error) {
			evaluation := EvalContext{Engine: engine, Event: event, Now: now, Variables: variables}
			for _, clause := range definition.merge {
				if clause.Matched {
					continue
				}
				if clause.Actions != nil {
					underlying, shouldInsert, _, actionErr := evaluateNamedWindowMergeNotMatchedActions(engine, schema, event, evaluation, tableMergeClauseActions(clause), now)
					return underlying, shouldInsert, actionErr
				}
				condition, ok := boolValue(clause.Condition.eval(evaluation))
				if !ok || !condition {
					continue
				}
				values, assignmentErr := evaluateTriggerAssignmentsForTarget(schema, nil, clause.Assignments, evaluation, now)
				if assignmentErr != nil {
					return nil, false, assignmentErr
				}
				original := any(nil)
				if len(clause.Assignments) == 0 {
					if schema.kind == SchemaVariant {
						// A predefined Variant keeps the concrete routed member
						// Event as its identity. Passing only the underlying struct
						// would make the target unable to reconstruct the member
						// envelope for a named-window insert.
						original = event
					} else {
						original = event.Underlying()
					}
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
			evaluation := EvalContext{Engine: engine, Event: event, Group: []Event{candidate}, Now: now, Variables: variables}
			if definition.onDemand {
				evaluation.OuterEvent = candidate
			}
			value := predicate.eval(evaluation)
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
			evaluation := EvalContext{Engine: engine, Event: event, Group: []Event{candidate}, Now: now, Variables: variables}
			if definition.onDemand {
				evaluation.OuterEvent = candidate
			}
			value := definition.where.eval(evaluation)
			matched, ok := boolValue(value)
			return ok && matched
		}, func(candidate Event) (any, error) {
			evaluation := EvalContext{Engine: engine, Event: event, Group: []Event{candidate}, Now: now, Variables: variables}
			if definition.onDemand {
				evaluation.OuterEvent = candidate
			}
			values, assignmentErr := evaluateTriggerAssignmentsForTarget(schema, candidate.Underlying(), definition.assignments, evaluation, now)
			if assignmentErr != nil {
				return nil, assignmentErr
			}
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

func executeTriggerAction(ctx context.Context, engine *Engine, definition *triggerDefinition, event Event, now time.Time, variables map[string]Value, owner *Statement, runtime *statementRuntime) (mutation tableMutationResult, err error) {
	if engine == nil || definition == nil {
		return tableMutationResult{}, NewError(ErrorDependency, "nil table trigger")
	}
	defer func() {
		if err != nil || definition.target == triggerTargetNamedWindow || definition.action == triggerSetVariables || runtime == nil {
			return
		}
		if ownershipErr := engine.recordLiveTableContextMutationLocked(runtime, definition, mutation, event, now, variables); ownershipErr != nil {
			mutation = tableMutationResult{}
			err = ownershipErr
		}
	}()
	evaluation := EvalContext{Engine: engine, Event: event, Now: now, Variables: variables}
	if definition.action == triggerSetVariables {
		return tableMutationResult{}, executeVariableTriggerAction(ctx, engine, definition, evaluation, variables, runtime)
	}
	if definition.target == triggerTargetNamedWindow {
		return executeNamedWindowAction(ctx, engine, definition, event, now, variables, owner)
	}
	table := engine.tables[catalogKey(definition.moduleName, definition.table)]
	if table == nil {
		return tableMutationResult{}, NewError(ErrorUnknownName, fmt.Sprintf("trigger table %q is not available", definition.table))
	}
	if definition.where != nil {
		return executeTableWhereAction(ctx, engine, table, definition, event, now, variables)
	}
	mutation = tableMutationResult{}
	switch definition.action {
	case triggerInsertTable:
		values, assignmentErr := evaluateTriggerAssignmentsForTarget(table.Definition().schema, nil, definition.assignments, evaluation, now)
		if assignmentErr != nil {
			return tableMutationResult{}, assignmentErr
		}
		row, err := table.Insert(ctx, values)
		if err != nil {
			return tableMutationResult{}, err
		}
		mutation.newRows = append(mutation.newRows, row)
		return mutation, nil
	case triggerUpsertTable:
		values, assignmentErr := evaluateTriggerAssignmentsForTarget(table.Definition().schema, nil, definition.assignments, evaluation, now)
		if assignmentErr != nil {
			return tableMutationResult{}, assignmentErr
		}
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
		targetEvent, eventErr := tableRowEvent(table, definition.table, old, now)
		if eventErr != nil {
			return tableMutationResult{}, eventErr
		}
		evaluation.Group = []Event{targetEvent}
		values, assignmentErr := evaluateTriggerAssignmentsForTarget(table.Definition().schema, targetEvent.Underlying(), definition.assignments, evaluation, now)
		if assignmentErr != nil {
			return tableMutationResult{}, assignmentErr
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
		var targetUnderlying any
		if found {
			targetEvent, eventErr := tableRowEvent(table, definition.table, old, now)
			if eventErr != nil {
				return tableMutationResult{}, eventErr
			}
			targetUnderlying = targetEvent.Underlying()
			evaluation.Group = []Event{targetEvent}
			evaluation.InitialGroup = []Event{targetEvent}
		}
		for _, clause := range definition.merge {
			if clause.Matched != found {
				continue
			}
			if !found && clause.Actions != nil {
				mutation, executed, actionErr := executeTableMergeNotMatchedActions(ctx, engine, table, definition, event, evaluation, tableMergeClauseActions(clause), now)
				if actionErr != nil {
					return tableMutationResult{}, actionErr
				}
				if executed {
					return mutation, nil
				}
				continue
			}
			if found {
				underlying, actionMatched, deleted, updated, actionErr := evaluateTableMergeActions(engine, table.Definition().schema, targetUnderlying, evaluation, tableMergeClauseActions(clause), now, true)
				if actionErr != nil {
					return tableMutationResult{}, actionErr
				}
				if !actionMatched {
					continue
				}
				if deleted {
					row, deletedRow, deleteErr := table.Delete(ctx, keys...)
					if deleteErr != nil {
						return tableMutationResult{}, deleteErr
					}
					if deletedRow {
						mutation.oldRows = append(mutation.oldRows, row)
					}
					return mutation, nil
				}
				if !updated {
					return mutation, nil
				}
				row, updateErr := table.Update(ctx, keys, valuesFromMergeUnderlying(table.Definition().schema, underlying))
				if updateErr != nil {
					return tableMutationResult{}, updateErr
				}
				mutation.oldRows = append(mutation.oldRows, old)
				mutation.newRows = append(mutation.newRows, row)
				return mutation, nil
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
			var assignmentErr error
			values, assignmentErr := evaluateTriggerAssignmentsForTarget(table.Definition().schema, targetUnderlying, clause.Assignments, evaluation, now)
			if assignmentErr != nil {
				return tableMutationResult{}, assignmentErr
			}
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

func executeTableWhereAction(ctx context.Context, engine *Engine, table *Table, definition *triggerDefinition, event Event, now time.Time, variables map[string]Value) (tableMutationResult, error) {
	if engine == nil || table == nil || definition == nil || definition.where == nil {
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
		if definition.contextDefinition != nil {
			ownership, active, partitionErr := engine.resolveTableContextRowOwnershipLocked(*definition.contextDefinition, catalogKey(definition.moduleName, definition.table), row, targetEvent, now, variables)
			if partitionErr != nil {
				return tableMutationResult{}, partitionErr
			}
			if !active || ownership.partitionKey != definition.contextPartitionKey {
				continue
			}
		}
		evaluation := EvalContext{Event: event, Group: []Event{targetEvent}, Now: now, Variables: variables}
		if definition.onDemand {
			evaluation.OuterEvent = targetEvent
		}
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
				if definition.contextDefinition != nil {
					engine.forgetTableContextRowsLocked(definition.contextDefinition.name, catalogKey(definition.moduleName, definition.table), []TableRow{deleted})
				}
			}
		case triggerUpdateTable:
			values, assignmentErr := evaluateTriggerAssignmentsForTarget(table.Definition().schema, targetEvent.Underlying(), definition.assignments, evaluation, now)
			if assignmentErr != nil {
				return tableMutationResult{}, assignmentErr
			}
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

func valuesFromMergeUnderlying(schema Schema, underlying any) map[string]any {
	values := make(map[string]any, len(schema.fields))
	for _, field := range schema.fields {
		value := schema.get(underlying, field.Name)
		if !value.IsMissing() {
			values[field.Name] = value.Any()
		}
	}
	return values
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
		if result[index].Actions != nil {
			actions := make([]TableMergeAction, len(result[index].Actions))
			for actionIndex, action := range result[index].Actions {
				actions[actionIndex] = action
				actions[actionIndex].Assignments = append([]TableAssignment(nil), action.Assignments...)
				actions[actionIndex].InsertSelections = append([]Selection(nil), action.InsertSelections...)
			}
			result[index].Actions = actions
		}
	}
	return result
}

func tableMergeClauseActions(clause TableMergeClause) []TableMergeAction {
	if clause.Actions != nil {
		return clause.Actions
	}
	return []TableMergeAction{{
		Condition:   clause.Condition,
		Assignments: clause.Assignments,
		Delete:      clause.Delete,
	}}
}

func validateTriggerAssignments(e *Environment, input *streamNode, table TableDefinition, assignments []TableAssignment) error {
	return validateTriggerAssignmentsWithTarget(e, input, table, assignments, "")
}

func validateTriggerAssignmentsWithTarget(e *Environment, input *streamNode, table TableDefinition, assignments []TableAssignment, targetKind string) error {
	for _, assignment := range assignments {
		if err := validateTriggerAssignment(e, input, table.schema, assignment, targetKind); err != nil {
			return fmt.Errorf("assignment %q: %w", assignment.Column, err)
		}
	}
	return nil
}

func validateTriggerAssignment(e *Environment, input *streamNode, targetSchema Schema, assignment TableAssignment, targetKind string) error {
	if assignment.Wildcard {
		if assignment.Column != "" || assignment.Index != nil || assignment.Expr != nil {
			return NewError(ErrorInvalidRule, "wildcard assignment cannot declare a column, index or expression")
		}
		return nil
	}
	if assignment.Column == "" || assignment.Expr == nil {
		return NewError(ErrorInvalidRule, "assignment is invalid")
	}
	field, exists := targetSchema.Field(assignment.Column)
	if !exists {
		return NewError(ErrorUnknownName, fmt.Sprintf("assignment references unknown column %q", assignment.Column))
	}
	validateExpression := func(expression Expr) error {
		if expression == nil {
			return NewError(ErrorInvalidRule, "assignment expression is nil")
		}
		if targetKind == "" {
			return e.validateExprFields(input, expression)
		}
		return e.validateTriggerTargetExpression(input, targetSchema, expression, targetKind)
	}
	if err := validateExpression(assignment.Expr); err != nil {
		return err
	}
	if assignment.Index == nil {
		if err := validateTriggerAssignmentType(field.Type, assignment.Expr); err != nil {
			return err
		}
		return nil
	}
	if err := validateExpression(assignment.Index); err != nil {
		return fmt.Errorf("array index: %w", err)
	}
	if !isTriggerIntegerType(assignment.Index.Type()) {
		return fmt.Errorf("array index expression must return an integer, got %s", triggerTypeDescription(assignment.Index.Type()))
	}
	arrayType := field.Type
	for arrayType != nil && arrayType.Kind() == reflect.Pointer {
		arrayType = arrayType.Elem()
	}
	if arrayType == nil || (arrayType.Kind() != reflect.Array && arrayType.Kind() != reflect.Slice) {
		return fmt.Errorf("target column %q is not an array or slice", assignment.Column)
	}
	elementType := arrayType.Elem()
	if err := validateTriggerAssignmentType(elementType, assignment.Expr); err != nil {
		return fmt.Errorf("array column %q: %w", assignment.Column, err)
	}
	return nil
}

func validateTriggerAssignmentType(target reflect.Type, expression Expr) error {
	if target == nil || expression == nil {
		return nil
	}
	if node := expression.node(); node != nil && node.kind == "null" {
		return nil
	}
	actual := expression.Type()
	if actual == nil || actual == typeOf[any]() || target == typeOf[any]() {
		return nil
	}
	if actual.AssignableTo(target) || numericTypes(actual, target) {
		return nil
	}
	if target.Kind() == reflect.Pointer && actual.AssignableTo(target.Elem()) {
		return nil
	}
	return fmt.Errorf("assignment expression type %s is incompatible with target type %s", actual, target)
}

func isTriggerIntegerType(typ reflect.Type) bool {
	if typ == nil {
		return false
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}

func triggerTypeDescription(typ reflect.Type) string {
	if typ == nil {
		return "<nil>"
	}
	return typ.String()
}

func evaluateTriggerAssignmentsForTarget(schema Schema, original any, assignments []TableAssignment, evaluation EvalContext, now time.Time) (map[string]any, error) {
	values := make(map[string]any, len(assignments))
	initialGroup := append([]Event(nil), evaluation.InitialGroup...)
	if len(initialGroup) == 0 {
		initialGroup = append([]Event(nil), evaluation.Group...)
	}
	for _, assignment := range assignments {
		step := evaluation
		step.InitialGroup = append([]Event(nil), initialGroup...)
		if len(initialGroup) > 0 {
			workingUnderlying, err := mergeSchemaUnderlying(schema, original, values)
			if err != nil {
				return nil, err
			}
			workingEvent, err := newEvent(schema, workingUnderlying, now)
			if err != nil {
				return nil, err
			}
			step.Group = []Event{workingEvent}
		} else {
			step.Group = nil
		}
		if assignment.Wildcard {
			if step.Event.TypeName() == "" {
				return nil, NewError(ErrorInvalidRule, "wildcard assignment requires a source event")
			}
			for _, field := range schema.fields {
				value := step.Event.Get(field.Name)
				if value.IsMissing() {
					if original == nil {
						values[field.Name] = nil
					}
					continue
				}
				values[field.Name] = value.Any()
			}
			continue
		}
		if assignment.Index != nil {
			indexValue := assignment.Index.eval(step)
			if indexValue.IsMissing() || indexValue.IsNull() {
				continue
			}
			value := assignment.Expr.eval(step)
			if value.IsMissing() {
				continue
			}
			if err := applyIndexedTriggerAssignment(schema, original, values, assignment.Column, indexValue, value); err != nil {
				return nil, err
			}
			continue
		}
		value := assignment.Expr.eval(step)
		if value.IsMissing() {
			continue
		}
		values[assignment.Column] = value.Any()
	}
	return values, nil
}

func applyIndexedTriggerAssignment(schema Schema, original any, updates map[string]any, column string, indexValue, value Value) error {
	workingUnderlying, err := mergeSchemaUnderlying(schema, original, updates)
	if err != nil {
		return err
	}
	current := schema.get(workingUnderlying, column)
	if !current.IsPresent() || current.IsNull() {
		// Esper ignores an indexed write when the target array itself is null.
		return nil
	}
	index, ok := triggerIndexValue(indexValue.Any())
	if !ok {
		return fmt.Errorf("array index for %q is not an integer", column)
	}
	array := reflect.ValueOf(current.Any())
	if !array.IsValid() {
		return nil
	}
	for array.Kind() == reflect.Interface {
		if array.IsNil() {
			return nil
		}
		array = array.Elem()
	}
	pointer := array.Kind() == reflect.Pointer
	if pointer {
		if array.IsNil() {
			return nil
		}
		array = array.Elem()
	}
	if array.Kind() != reflect.Array && array.Kind() != reflect.Slice {
		return fmt.Errorf("target column %q is not an array or slice", column)
	}
	if index < 0 || index >= array.Len() {
		return fmt.Errorf("array index %d out of range for target column %q (length %d)", index, column, array.Len())
	}
	var copyValue reflect.Value
	if array.Kind() == reflect.Slice {
		copyValue = reflect.MakeSlice(array.Type(), array.Len(), array.Len())
		reflect.Copy(copyValue, array)
	} else {
		copyValue = reflect.New(array.Type()).Elem()
		copyValue.Set(array)
	}
	element := copyValue.Index(index)
	if value.IsNull() || value.Any() == nil {
		if isTriggerNilableType(element.Type()) {
			element.Set(reflect.Zero(element.Type()))
		}
	} else {
		converted, convertErr := assignReflectValue(element.Type(), value.Any())
		if convertErr != nil {
			return fmt.Errorf("array column %q element %d: %w", column, index, convertErr)
		}
		element.Set(converted)
	}
	if pointer {
		result := reflect.New(copyValue.Type())
		result.Elem().Set(copyValue)
		updates[column] = result.Interface()
	} else {
		updates[column] = copyValue.Interface()
	}
	return nil
}

func triggerIndexValue(value any) (int, bool) {
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return 0, false
	}
	for reflected.Kind() == reflect.Interface || reflected.Kind() == reflect.Pointer {
		if reflected.IsNil() {
			return 0, false
		}
		reflected = reflected.Elem()
	}
	switch reflected.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		index := reflected.Int()
		if int64(int(index)) != index {
			return 0, false
		}
		return int(index), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		index := reflected.Uint()
		if uint64(int(index)) != index || int(index) < 0 {
			return 0, false
		}
		return int(index), true
	default:
		return 0, false
	}
}

func isTriggerNilableType(typ reflect.Type) bool {
	if typ == nil {
		return false
	}
	for typ.Kind() == reflect.Pointer {
		return true
	}
	switch typ.Kind() {
	case reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return true
	default:
		return false
	}
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
