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

type onDemandAction uint8

const (
	onDemandInsert onDemandAction = iota
	onDemandUpdate
	onDemandDelete
	onDemandDeleteAll
)

type onDemandDefinition struct {
	action      onDemandAction
	predicate   Expr
	assignments []TableAssignment
	rows        []onDemandInsertRow
}

// onDemandInsertRow is the positional values form of an on-demand insert.
// The fields are deliberately private: callers construct rows through
// InsertValues so the target schema remains the source of column ordering.
type onDemandInsertRow struct {
	values []Expr
}

func (d *onDemandDefinition) description() string {
	if d == nil {
		return "on-demand(<nil>)"
	}
	action := "insert"
	switch d.action {
	case onDemandUpdate:
		action = "update"
	case onDemandDelete:
		action = "delete"
	case onDemandDeleteAll:
		action = "delete-all"
	}
	parts := []string{action}
	if d.predicate != nil {
		parts = append(parts, "where("+d.predicate.Description()+")")
	}
	if len(d.assignments) > 0 {
		assignments := make([]string, 0, len(d.assignments))
		for _, assignment := range d.assignments {
			if assignment.Wildcard {
				assignments = append(assignments, "*")
				continue
			}
			index := ""
			if assignment.Index != nil {
				index = "[" + assignment.Index.Description() + "]"
			}
			expression := "<nil>"
			if assignment.Expr != nil {
				expression = assignment.Expr.Description()
			}
			assignments = append(assignments, assignment.Column+index+"="+expression)
		}
		parts = append(parts, "set("+strings.Join(assignments, ",")+")")
	}
	if len(d.rows) > 0 {
		rows := make([]string, 0, len(d.rows))
		for _, row := range d.rows {
			values := make([]string, 0, len(row.values))
			for _, value := range row.values {
				if value == nil {
					values = append(values, "<nil>")
				} else {
					values = append(values, value.Description())
				}
			}
			rows = append(rows, "["+strings.Join(values, ",")+"]")
		}
		parts = append(parts, "values("+strings.Join(rows, ",")+")")
	}
	return "on-demand(" + strings.Join(parts, ",") + ")"
}

// QueryResult is the read-only result of a Fire-and-Forget execution.
type QueryResult struct {
	Batch ResultBatch
}

func (r QueryResult) Results() []Result { return append([]Result(nil), r.Batch.New...) }

// ParameterValues supplies execution-time values for Parameter expressions.
// It is an alias so callers can pass an ordinary map literal without a cast.
type ParameterValues = map[string]any

// ExecuteFireAndForget evaluates a named-window query against one consistent
// snapshot. It never mutates the named-window state or deploys a statement.
func (e *Engine) ExecuteFireAndForget(ctx context.Context, plan Plan) (QueryResult, error) {
	return e.executeFireAndForget(ctx, plan, nil, nil)
}

// ExecuteFireAndForgetWithSelector evaluates a context-aware named-window
// query against only the selected materialized partitions. A nil selector
// selects all partitions.
func (e *Engine) ExecuteFireAndForgetWithSelector(ctx context.Context, plan Plan, selector ContextPartitionSelector) (QueryResult, error) {
	return e.executeFireAndForget(ctx, plan, selector, nil)
}

// ExecuteFireAndForgetWithParameters executes a plan with one immutable
// parameter binding snapshot. It is useful for one-shot parameterized FAF
// queries without creating a PreparedQuery handle.
func (e *Engine) ExecuteFireAndForgetWithParameters(ctx context.Context, plan Plan, parameters ParameterValues) (QueryResult, error) {
	return e.executeFireAndForget(ctx, plan, nil, parameters)
}

// ExecuteFireAndForgetWithPositionalParameters executes a plan whose
// expressions use ParameterAt. Values are supplied in 1-based parameter
// order, matching Esper's prepared-query contract while keeping the Go call
// site allocation-free and immutable for the duration of one execution.
func (e *Engine) ExecuteFireAndForgetWithPositionalParameters(ctx context.Context, plan Plan, values ...any) (QueryResult, error) {
	parameters, err := positionalParameterBindings(plan, values)
	if err != nil {
		return QueryResult{}, err
	}
	return e.executeFireAndForget(ctx, plan, nil, parameters)
}

// ExecuteFireAndForgetWithSelectorAndParameters combines context partition
// selection with execution-time parameter binding.
func (e *Engine) ExecuteFireAndForgetWithSelectorAndParameters(ctx context.Context, plan Plan, selector ContextPartitionSelector, parameters ParameterValues) (QueryResult, error) {
	return e.executeFireAndForget(ctx, plan, selector, parameters)
}

// ExecuteFireAndForgetWithSelectorAndPositionalParameters combines a
// positional binding snapshot with context-partition selection.
func (e *Engine) ExecuteFireAndForgetWithSelectorAndPositionalParameters(ctx context.Context, plan Plan, selector ContextPartitionSelector, values ...any) (QueryResult, error) {
	parameters, err := positionalParameterBindings(plan, values)
	if err != nil {
		return QueryResult{}, err
	}
	return e.executeFireAndForget(ctx, plan, selector, parameters)
}

// ExecuteFireAndForgetAndRoute evaluates a fire-and-forget plan and feeds its
// selected result stream into the plan's RouteTo/InsertInto target. The plain
// ExecuteFireAndForget methods remain read-only; callers opt into the event
// pipeline explicitly with this method when a FAF result should become an
// input event for deployed statements.
func (e *Engine) ExecuteFireAndForgetAndRoute(ctx context.Context, plan Plan) (QueryResult, error) {
	return e.executeFireAndForgetAndRoute(ctx, plan, nil, nil)
}

// ExecuteFireAndForgetAndRouteWithSelector combines context partition
// selection with explicit result routing.
func (e *Engine) ExecuteFireAndForgetAndRouteWithSelector(ctx context.Context, plan Plan, selector ContextPartitionSelector) (QueryResult, error) {
	return e.executeFireAndForgetAndRoute(ctx, plan, selector, nil)
}

// ExecuteFireAndForgetAndRouteWithParameters combines parameter binding with
// explicit result routing.
func (e *Engine) ExecuteFireAndForgetAndRouteWithParameters(ctx context.Context, plan Plan, parameters ParameterValues) (QueryResult, error) {
	return e.executeFireAndForgetAndRoute(ctx, plan, nil, parameters)
}

// ExecuteFireAndForgetAndRouteWithPositionalParameters evaluates and routes
// a positional-parameter plan in one explicit side-effecting operation.
func (e *Engine) ExecuteFireAndForgetAndRouteWithPositionalParameters(ctx context.Context, plan Plan, values ...any) (QueryResult, error) {
	parameters, err := positionalParameterBindings(plan, values)
	if err != nil {
		return QueryResult{}, err
	}
	return e.executeFireAndForgetAndRoute(ctx, plan, nil, parameters)
}

// ExecuteFireAndForgetAndRouteWithSelectorAndParameters combines context
// partition selection, parameter binding and explicit result routing.
func (e *Engine) ExecuteFireAndForgetAndRouteWithSelectorAndParameters(ctx context.Context, plan Plan, selector ContextPartitionSelector, parameters ParameterValues) (QueryResult, error) {
	return e.executeFireAndForgetAndRoute(ctx, plan, selector, parameters)
}

// ExecuteFireAndForgetAndRouteWithSelectorAndPositionalParameters combines
// positional bindings, context selection and explicit result routing.
func (e *Engine) ExecuteFireAndForgetAndRouteWithSelectorAndPositionalParameters(ctx context.Context, plan Plan, selector ContextPartitionSelector, values ...any) (QueryResult, error) {
	parameters, err := positionalParameterBindings(plan, values)
	if err != nil {
		return QueryResult{}, err
	}
	return e.executeFireAndForgetAndRoute(ctx, plan, selector, parameters)
}

func (e *Engine) executeFireAndForgetAndRoute(ctx context.Context, plan Plan, selector ContextPartitionSelector, parameters ParameterValues) (QueryResult, error) {
	result, err := e.executeFireAndForget(ctx, plan, selector, parameters)
	if err != nil {
		return QueryResult{}, err
	}
	if err := e.RouteFireAndForget(ctx, plan, result); err != nil {
		return QueryResult{}, err
	}
	return result, nil
}

// RouteFireAndForget routes a previously evaluated FAF result according to
// the plan's selector and route target. It is useful when the caller needs to
// inspect, persist or compare a snapshot before opting into the side effect.
// Result conversion is completed before the first event is sent, so a bad
// runtime projection cannot partially route a batch.
func (e *Engine) RouteFireAndForget(ctx context.Context, plan Plan, result QueryResult) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	if plan.query.env != e.env || plan.schemaVersion == "" {
		return NewError(ErrorDependency, "plan does not belong to this engine")
	}
	if plan.query.routeTarget == "" {
		return NewError(ErrorInvalidRule, "fire-and-forget route target is not configured")
	}

	results := routeResults(plan.query.selector, result.Batch)
	if len(results) == 0 {
		return nil
	}
	now := result.Batch.Time
	if now.IsZero() {
		now = e.Now()
	}
	// routeResultLocked is shared with live statements and intentionally runs
	// under the engine lock because it reads the environment/schema registry.
	e.mu.Lock()
	routeStatement := &Statement{engine: e, plan: plan}
	routed := make([]Event, 0, len(results))
	for _, item := range results {
		event, routeErr := e.routeResultLocked(routeStatement, item, now)
		if routeErr != nil {
			e.mu.Unlock()
			return routeErr
		}
		routed = append(routed, event)
	}
	e.mu.Unlock()

	target, ok := e.env.Schema(plan.query.routeTarget)
	if !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("route target %q is not registered", plan.query.routeTarget))
	}
	for _, event := range routed {
		if err := contextErr(ctx); err != nil {
			return err
		}
		var underlying any = event.Underlying()
		if target.kind == SchemaVariant {
			underlying = event
		}
		if err := e.Route(ctx, plan.query.routeTarget, underlying); err != nil {
			return err
		}
	}
	return nil
}

func routeResults(selector StreamSelector, batch ResultBatch) []Result {
	var results []Result
	switch selector {
	case SelectRStream:
		results = append(results, batch.Old...)
	case SelectIRStream:
		results = append(results, batch.New...)
		results = append(results, batch.Old...)
	default:
		results = append(results, batch.New...)
	}
	return results
}

func (e *Engine) executeFireAndForget(ctx context.Context, plan Plan, selector ContextPartitionSelector, parameters ParameterValues) (QueryResult, error) {
	if err := contextErr(ctx); err != nil {
		return QueryResult{}, err
	}
	if e == nil || e.env == nil {
		return QueryResult{}, NewError(ErrorDependency, "engine has no environment")
	}
	if plan.query.env != e.env || plan.schemaVersion == "" {
		return QueryResult{}, NewError(ErrorDependency, "plan does not belong to this engine")
	}
	if plan.query.contextName != "" {
		definition, ok := e.env.Context(plan.query.contextName)
		if !ok {
			return QueryResult{}, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", plan.query.contextName))
		}
		if err := validateContextPartitionSelector(definition, selector); err != nil {
			return QueryResult{}, err
		}
	}
	if plan.query.tableTarget != "" {
		return QueryResult{}, NewError(ErrorInvalidRule, "into-table aggregate statements are live materializations and cannot run as fire-and-forget queries")
	}
	parameterTypes, parameterTypeErr := queryParameterTypes(e.env, plan.query)
	if parameterTypeErr != nil {
		return QueryResult{}, WrapError(ErrorInvalidRule, "parameters", parameterTypeErr)
	}
	if err := validateParameterBindings(parameterTypes, parameters); err != nil {
		return QueryResult{}, err
	}
	// A Plan can also be deployed as a live statement, so the FAF-only
	// subquery source rules are checked at execution time rather than during
	// the general Build step. This mirrors the Java distinction between the
	// ordinary compiler and compileFAF.
	if err := e.env.validateFireAndForgetSubqueries(plan.query); err != nil {
		return QueryResult{}, WrapError(ErrorInvalidRule, "fire-and-forget subquery", err)
	}
	if plan.query.onDemand != nil {
		return e.executeFireAndForgetMutation(ctx, plan, selector, parameters)
	}
	if plan.query.sourceLess {
		e.mu.Lock()
		e.refreshVariablesLocked()
		variables := bindParameterValues(cloneValues(e.variables), parameters)
		now := e.clock.Now()
		e.mu.Unlock()
		runtime := newStatementRuntime(plan.query)
		runtime.engine = e
		runtime.initializeAt(now)
		runtime.ctx = ctx
		runtime.variables = variablesWithEngine(variables, e)
		batch := runtime.batch(eventDelta{newEvents: []Event{{}}}, plan, now)
		batch = runtime.applyOutput(plan.query.output, batch, false, now, plan)
		return QueryResult{Batch: batch}, nil
	}
	if plan.query.join != nil {
		if plan.query.contextName != "" {
			return e.executeContextJoinFireAndForget(ctx, plan, selector, parameters)
		}
		return e.executeJoinFireAndForget(ctx, plan, parameters)
	}
	if !containsNamedWindow(plan.query.input, nil) && !containsTableSource(plan.query.input, nil) && !containsHistoricalSource(plan.query.input) {
		return QueryResult{}, NewError(ErrorInvalidRule, "fire-and-forget currently requires a named-window, table, historical source, or method source")
	}
	if plan.query.contextName != "" {
		return e.executeContextFireAndForget(ctx, plan, selector, parameters)
	}
	source, err := sourceNode(plan.query.input)
	if err != nil {
		return QueryResult{}, err
	}
	e.mu.Lock()
	now := e.clock.Now()
	variables := bindParameterValues(cloneValues(e.variables), parameters)
	e.mu.Unlock()
	selection, _ := plan.indexPlan.ForSource(0)
	events, err := e.snapshotFireAndForgetSourceWithIndex(ctx, source, selection, sourceIndexFilterExpressions(plan.query.input), now, variables)
	if err != nil {
		return QueryResult{}, err
	}
	runtime := newStatementRuntime(plan.query)
	runtime.engine = e
	runtime.initializeAt(now)
	runtime.ctx = ctx
	runtime.variables = variablesWithEngine(variables, e)
	delta := eventDelta{}
	input := plan.query.input
	if source.kind == streamHistorical {
		input = replaceStreamBase(input, source, &streamNode{kind: streamSource, sourceName: source.historical.schema.Name(), sourceType: typeOf[any]()})
	} else if source.kind == streamMethod {
		input = replaceStreamBase(input, source, &streamNode{kind: streamSource, sourceName: source.method.schema.Name(), sourceType: typeOf[any]()})
	} else if source.kind == streamContained {
		childSchema, schemaErr := e.env.sourceSchema(source)
		if schemaErr != nil {
			return QueryResult{}, schemaErr
		}
		// snapshotFireAndForgetSource has already expanded the contained node.
		// Replace that node at the runtime input boundary so the expanded child
		// is not interpreted as a new parent and expanded a second time.
		input = replaceStreamBase(input, source, &streamNode{kind: streamSource, sourceName: childSchema.Name(), sourceType: typeOf[any]()})
	}
	for _, event := range events {
		inserted, insertErr := runtime.insert(input, event, now)
		if insertErr != nil {
			return QueryResult{}, insertErr
		}
		delta = mergeDelta(delta, inserted)
	}
	if plan.query.aggregate != nil {
		batch, aggregateErr := runtime.aggregateBatch(delta, plan, now)
		if aggregateErr != nil {
			return QueryResult{}, aggregateErr
		}
		batch = runtime.applyOutput(plan.query.output, batch, false, now, plan)
		return QueryResult{Batch: batch}, nil
	}
	if plan.query.rowRecog != nil {
		batch := runtime.rowRecogBatch(delta, plan, now)
		batch = runtime.applyOutput(plan.query.output, batch, false, now, plan)
		return QueryResult{Batch: batch}, nil
	}
	batch := runtime.batch(delta, plan, now)
	batch = runtime.applyOutput(plan.query.output, batch, false, now, plan)
	return QueryResult{Batch: batch}, nil
}

// executeFireAndForgetMutation applies the target-side on-demand operation
// under the same engine transaction boundary used by live trigger actions.
// Named Window deltas are fed through consuming statements before listeners
// are dispatched; Table mutations return no rows, matching Esper's FAF table
// result contract.
func (e *Engine) executeFireAndForgetMutation(ctx context.Context, plan Plan, selector ContextPartitionSelector, parameters ParameterValues) (QueryResult, error) {
	if err := contextErr(ctx); err != nil {
		return QueryResult{}, err
	}
	if e == nil || e.env == nil || plan.query.onDemand == nil || plan.query.input == nil {
		return QueryResult{}, NewError(ErrorDependency, "on-demand mutation has no engine, target or definition")
	}
	source := plan.query.input
	if source.kind != streamNamedWindow && source.kind != streamTable {
		return QueryResult{}, NewError(ErrorInvalidRule, "on-demand target must be a root named window or table")
	}

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return QueryResult{}, NewError(ErrorState, "engine is closed")
	}
	now := e.clock.Now()
	e.refreshVariablesLocked()
	variables := variablesWithEngineLockState(bindParameterValues(cloneValues(e.variables), parameters), e, true)
	e.pendingStatementDispatches = nil
	e.pendingNamedWindowDispatches = nil
	e.pendingRoutedEvents = nil
	e.pendingContextEvents = nil
	e.pendingVariableChanges = nil
	e.pendingMatchRecognizeStateLimits = nil
	e.pendingPatternSubexpressionLimits = nil

	if source.kind == streamNamedWindow {
		if _, ok := e.ensureNamedWindowLockedInModule(source.moduleName, source.sourceName); !ok {
			e.mu.Unlock()
			return QueryResult{}, NewError(ErrorUnknownName, fmt.Sprintf("named window %q is not registered", source.sourceName))
		}
	}

	var mutation tableMutationResult
	var err error
	if plan.query.contextName != "" {
		mutation, err = e.executeContextFireAndForgetMutationLocked(ctx, plan, selector, now, variables)
	} else if plan.query.onDemand.action == onDemandInsert && len(plan.query.onDemand.rows) > 0 {
		mutation, err = e.executeFireAndForgetMultirowInsertLocked(ctx, plan, now, variables)
	} else {
		definition := &triggerDefinition{
			input:      source,
			table:      source.sourceName,
			moduleName: source.moduleName,
			onDemand:   true,
			where:      plan.query.onDemand.predicate,
			action:     triggerInsertTable,
		}
		switch plan.query.onDemand.action {
		case onDemandInsert:
			definition.action = triggerInsertTable
		case onDemandUpdate:
			definition.action = triggerUpdateTable
		case onDemandDelete:
			definition.action = triggerDeleteTable
		case onDemandDeleteAll:
			definition.action = triggerDeleteAllTable
		default:
			e.mu.Unlock()
			return QueryResult{}, NewError(ErrorInvalidRule, fmt.Sprintf("unknown on-demand action %d", plan.query.onDemand.action))
		}
		definition.assignments = append([]TableAssignment(nil), plan.query.onDemand.assignments...)
		if source.kind == streamNamedWindow {
			definition.target = triggerTargetNamedWindow
		}

		// A zero Event is intentional: on-demand expressions have no trigger
		// event. Target fields resolve from EvalContext.Group, while literals,
		// variables and bound parameters resolve from EvalContext.Variables.
		mutation, err = executeTriggerAction(ctx, e, definition, Event{}, now, variables, nil, nil)
	}
	if err != nil {
		e.mu.Unlock()
		return QueryResult{}, err
	}

	var batch ResultBatch
	batch.Time = now
	if source.kind == streamNamedWindow {
		switch plan.query.onDemand.action {
		case onDemandDelete, onDemandDeleteAll:
			batch.New = eventsToResults(mutation.oldEvents)
		default:
			batch.New = eventsToResults(mutation.newEvents)
		}
		if !batch.empty() {
			batch.Sequence = 1
		}
	}

	dispatches := make([]statementDispatch, 0, len(e.pendingStatementDispatches))
	if err := e.processPendingRoutedEventsLocked(ctx, now, variables, &dispatches); err != nil {
		e.mu.Unlock()
		return QueryResult{}, err
	}
	dispatches = append(dispatches, e.pendingStatementDispatches...)
	nestedNamedWindowDispatches := append([]namedWindowDispatch(nil), e.pendingNamedWindowDispatches...)
	variableChanges := e.takeVariableChangesLocked()
	contextEvents := e.takeContextEventsLocked()
	e.pendingStatementDispatches = nil
	e.pendingNamedWindowDispatches = nil
	e.pendingRoutedEvents = nil
	e.pendingContextEvents = nil
	e.pendingVariableChanges = nil
	e.mu.Unlock()

	e.dispatchVariableChanges(variableChanges)
	e.dispatchContextEvents(contextEvents)
	e.dispatchMatchRecognizeStateLimitEvents()
	e.dispatchPatternSubexpressionLimitEvents()
	if err := dispatchAll(ctx, dispatches); err != nil {
		return QueryResult{}, err
	}
	for _, dispatch := range nestedNamedWindowDispatches {
		if err := dispatch.window.dispatch(ctx, dispatch.delta); err != nil {
			return QueryResult{}, err
		}
	}
	return QueryResult{Batch: batch}, nil
}

// executeFireAndForgetMultirowInsertLocked evaluates and commits a
// positional multi-row insert while the caller holds e.mu. No consumer is
// processed until every row has been validated and inserted, so one failed
// row cannot expose a partial Named Window delta or a partially populated
// Table.
func (e *Engine) executeFireAndForgetMultirowInsertLocked(ctx context.Context, plan Plan, now time.Time, variables map[string]Value) (tableMutationResult, error) {
	if e == nil || e.env == nil || plan.query.onDemand == nil || plan.query.input == nil {
		return tableMutationResult{}, NewError(ErrorDependency, "multi-row insert has no engine, target or definition")
	}
	target := plan.query.input
	if target.kind != streamNamedWindow && target.kind != streamTable {
		return tableMutationResult{}, NewError(ErrorInvalidRule, "multi-row insert target must be a root named window or table")
	}
	schema, err := e.env.sourceSchema(target)
	if err != nil {
		return tableMutationResult{}, err
	}
	rows := plan.query.onDemand.rows
	if len(rows) == 0 {
		return tableMutationResult{}, NewError(ErrorInvalidRule, "multi-row insert requires at least one row")
	}
	if len(rows) > 1000 {
		return tableMutationResult{}, NewError(ErrorInvalidRule, fmt.Sprintf("on-demand insert number of rows exceeds the maximum of 1000 rows as the query provides %d rows", len(rows)))
	}

	// Evaluate every row before touching target state. This also gives
	// subqueries one engine-backed, lock-aware evaluation context.
	evaluated := make([]map[string]any, 0, len(rows))
	underlyings := make([]any, 0, len(rows))
	evaluation := EvalContext{Engine: e, Now: now, Variables: variables}
	for rowIndex, row := range rows {
		if err := contextErr(ctx); err != nil {
			return tableMutationResult{}, err
		}
		if len(row.values) != len(schema.fields) {
			return tableMutationResult{}, fmt.Errorf("failed to validate multi-row insert at row %d of %d: number of supplied values %d does not match target column count %d", rowIndex+1, len(rows), len(row.values), len(schema.fields))
		}
		assignments := make([]TableAssignment, 0, len(row.values))
		for columnIndex, expression := range row.values {
			if expression == nil {
				return tableMutationResult{}, fmt.Errorf("failed to validate multi-row insert at row %d of %d: value %d is nil", rowIndex+1, len(rows), columnIndex+1)
			}
			assignments = append(assignments, SetColumn(schema.fields[columnIndex].Name, expression))
		}
		values, assignmentErr := evaluateTriggerAssignmentsForTarget(schema, nil, assignments, evaluation, now)
		if assignmentErr != nil {
			return tableMutationResult{}, fmt.Errorf("failed to evaluate multi-row insert row %d of %d: %w", rowIndex+1, len(rows), assignmentErr)
		}
		evaluated = append(evaluated, values)
		if target.kind == streamNamedWindow {
			underlying, mergeErr := mergeSchemaUnderlying(schema, nil, values)
			if mergeErr != nil {
				return tableMutationResult{}, fmt.Errorf("failed to materialize multi-row insert row %d of %d: %w", rowIndex+1, len(rows), mergeErr)
			}
			if _, eventErr := newEvent(schema, underlying, now); eventErr != nil {
				return tableMutationResult{}, fmt.Errorf("failed to materialize multi-row insert row %d of %d: %w", rowIndex+1, len(rows), eventErr)
			}
			underlyings = append(underlyings, underlying)
		}
	}

	if target.kind == streamTable {
		table := e.tables[catalogKey(target.moduleName, target.sourceName)]
		if table == nil {
			return tableMutationResult{}, NewError(ErrorUnknownName, fmt.Sprintf("table %q is not registered", target.sourceName))
		}
		snapshot := table.snapshotMutationState()
		mutation := tableMutationResult{}
		for rowIndex, values := range evaluated {
			if err := contextErr(ctx); err != nil {
				table.restoreMutationState(snapshot)
				return tableMutationResult{}, err
			}
			row, insertErr := table.Insert(ctx, values)
			if insertErr != nil {
				table.restoreMutationState(snapshot)
				return tableMutationResult{}, fmt.Errorf("multi-row insert row %d of %d failed: %w", rowIndex+1, len(rows), insertErr)
			}
			mutation.newRows = append(mutation.newRows, row)
		}
		return mutation, nil
	}

	window := e.namedWindows[catalogKey(target.moduleName, target.sourceName)]
	if window == nil {
		return tableMutationResult{}, NewError(ErrorUnknownName, fmt.Sprintf("named window %q is not registered", target.sourceName))
	}
	snapshot := window.snapshotMutationState()
	delta := NamedWindowDelta{Time: now}
	for rowIndex, underlying := range underlyings {
		if err := contextErr(ctx); err != nil {
			window.restoreMutationState(snapshot)
			return tableMutationResult{}, err
		}
		rowDelta, insertErr := window.insertWithVariables(now, underlying, variables)
		if insertErr != nil {
			window.restoreMutationState(snapshot)
			return tableMutationResult{}, fmt.Errorf("multi-row insert row %d of %d failed: %w", rowIndex+1, len(rows), insertErr)
		}
		delta.New = append(delta.New, rowDelta.New...)
		delta.Old = append(delta.Old, rowDelta.Old...)
	}
	if err := e.queueNamedWindowDeltaLocked(ctx, now, window, delta, variables, nil); err != nil {
		window.restoreMutationState(snapshot)
		return tableMutationResult{}, err
	}
	return tableMutationResult{newEvents: append([]Event(nil), delta.New...)}, nil
}

type contextMutationPartition struct {
	key               string
	representative    Event
	contextProperties map[string]Value
}

func contextMutationProperties(definition ContextDefinition, representative Event, stored map[string]Value, now time.Time, variables map[string]Value, partitionID int) map[string]Value {
	properties := cloneValues(stored)
	if len(properties) == 0 {
		properties = definition.contextPropertyValues(representative, now, variables, partitionID)
	}
	if properties == nil {
		properties = make(map[string]Value)
	}
	properties["name"] = Present(definition.name)
	properties["id"] = Present(partitionID)
	return properties
}

// executeContextFireAndForgetMutationLocked applies one on-demand mutation to
// each selected context partition. The caller holds e.mu, matching the normal
// FAF mutation path and the live trigger transaction boundary.
func (e *Engine) executeContextFireAndForgetMutationLocked(ctx context.Context, plan Plan, selector ContextPartitionSelector, now time.Time, variables map[string]Value) (tableMutationResult, error) {
	if e == nil || e.env == nil || plan.query.onDemand == nil || plan.query.input == nil {
		return tableMutationResult{}, NewError(ErrorDependency, "context on-demand mutation has no engine, target or definition")
	}
	definition, ok := e.env.Context(plan.query.contextName)
	if !ok {
		return tableMutationResult{}, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", plan.query.contextName))
	}
	if definition.kind == ContextInitiatedTerminated {
		return tableMutationResult{}, NewError(ErrorInvalidRule, "on-demand context mutation does not support initiated-terminated lifecycle")
	}
	if plan.query.onDemand.action == onDemandInsert {
		return tableMutationResult{}, NewError(ErrorInvalidRule, "context on-demand insert is not supported without an incoming partition event")
	}

	source := plan.query.input
	partitions := make(map[string]contextMutationPartition)
	switch source.kind {
	case streamNamedWindow:
		window := e.namedWindows[catalogKey(source.moduleName, source.sourceName)]
		if window == nil {
			return tableMutationResult{}, NewError(ErrorUnknownName, fmt.Sprintf("named window %q is not registered", source.sourceName))
		}
		if window.Definition().Context() != definition.name {
			return tableMutationResult{}, NewError(ErrorInvalidRule, fmt.Sprintf("named window %q is not bound to context %q", source.sourceName, definition.name))
		}
		for _, state := range window.contextPartitionStates() {
			if state == nil || state.contextKey == "" {
				continue
			}
			events := snapshotNamedWindowState(state)
			if len(events) == 0 {
				continue
			}
			if _, exists := partitions[state.contextKey]; !exists {
				partitions[state.contextKey] = contextMutationPartition{
					key:               state.contextKey,
					representative:    events[0],
					contextProperties: state.contextPropertiesSnapshot(),
				}
			}
		}
	case streamTable:
		table := e.tables[catalogKey(source.moduleName, source.sourceName)]
		if table == nil {
			return tableMutationResult{}, NewError(ErrorUnknownName, fmt.Sprintf("table %q is not registered", source.sourceName))
		}
		rows, err := table.Snapshot(ctx)
		if err != nil {
			return tableMutationResult{}, err
		}
		for _, row := range rows {
			if err := contextErr(ctx); err != nil {
				return tableMutationResult{}, err
			}
			targetEvent, eventErr := tableRowEvent(table, catalogKey(source.moduleName, source.sourceName), row, now)
			if eventErr != nil {
				return tableMutationResult{}, eventErr
			}
			key, active, partitionErr := definition.partition(targetEvent, now, variables)
			if partitionErr != nil {
				return tableMutationResult{}, partitionErr
			}
			if active {
				if _, exists := partitions[key]; !exists {
					partitions[key] = contextMutationPartition{key: key, representative: targetEvent}
				}
			}
		}
	default:
		return tableMutationResult{}, NewError(ErrorInvalidRule, "context on-demand target must be a root named window or table")
	}

	keys := make([]string, 0, len(partitions))
	for key := range partitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	mutation := tableMutationResult{}
	for index, key := range keys {
		if err := contextErr(ctx); err != nil {
			return tableMutationResult{}, err
		}
		partition := partitions[key]
		partitionID := index
		if ids := e.contextPartitionIDs[definition.name]; ids != nil {
			if existing, exists := ids[key]; exists {
				partitionID = existing
			} else {
				partitionID = e.allocateContextPartitionIDLocked(definition.name, key)
			}
		} else {
			partitionID = e.allocateContextPartitionIDLocked(definition.name, key)
		}
		properties := contextMutationProperties(definition, partition.representative, partition.contextProperties, now, variables, partitionID)
		descriptor := ContextPartitionDescriptor{ID: partitionID, Key: key, BaseKey: contextPartitionBaseKey(key), ContextName: definition.name, properties: cloneValues(properties)}
		if !contextPartitionSelectedWithDescriptor(selector, descriptor) {
			continue
		}

		partitionRuntime := newStatementRuntime(Query{})
		partitionRuntime.engine = e
		partitionRuntime.partitionContextName = definition.name
		partitionRuntime.partitionKey = key
		partitionRuntime.partitionID = partitionID
		partitionRuntime.contextProperties = properties
		partitionVariables := partitionRuntime.withContextVariables(cloneValues(variables))
		partitionVariables = partitionRuntime.withContextProperties(partitionVariables)

		trigger := &triggerDefinition{
			input:               source,
			table:               source.sourceName,
			moduleName:          source.moduleName,
			onDemand:            true,
			where:               plan.query.onDemand.predicate,
			action:              triggerDeleteTable,
			assignments:         append([]TableAssignment(nil), plan.query.onDemand.assignments...),
			contextDefinition:   &definition,
			contextPartitionKey: key,
		}
		switch plan.query.onDemand.action {
		case onDemandUpdate:
			trigger.action = triggerUpdateTable
		case onDemandDelete:
			trigger.action = triggerDeleteTable
		case onDemandDeleteAll:
			if source.kind == streamTable {
				// Table.Clear is necessarily global. Turn delete-all into the
				// same target-row scan used by delete-where so the context
				// partition predicate remains in force.
				trigger.action = triggerDeleteTable
				trigger.where = Literal(true)
			} else {
				trigger.action = triggerDeleteAllTable
			}
		default:
			return tableMutationResult{}, NewError(ErrorInvalidRule, fmt.Sprintf("unknown on-demand action %d", plan.query.onDemand.action))
		}
		if source.kind == streamNamedWindow {
			trigger.target = triggerTargetNamedWindow
		}
		partMutation, err := executeTriggerAction(ctx, e, trigger, Event{}, now, partitionVariables, nil, nil)
		if err != nil {
			return tableMutationResult{}, err
		}
		mutation.oldRows = append(mutation.oldRows, partMutation.oldRows...)
		mutation.newRows = append(mutation.newRows, partMutation.newRows...)
		mutation.oldEvents = append(mutation.oldEvents, partMutation.oldEvents...)
		mutation.newEvents = append(mutation.newEvents, partMutation.newEvents...)
	}
	return mutation, nil
}

func parameterDisplayName(name string) string {
	if position, positional := positionalParameterPosition(name); positional {
		return fmt.Sprintf("positional parameter %d", position)
	}
	return name
}

func validateParameterValue(name string, value any, expected reflect.Type) error {
	if value == nil || expected == nil || expected == typeOf[any]() {
		return nil
	}
	actual := reflect.TypeOf(value)
	if actual.AssignableTo(expected) {
		return nil
	}
	return NewError(ErrorTypeMismatch, fmt.Sprintf("fire-and-forget parameter %q expects %s, got %s", parameterDisplayName(name), parameterTypeDescription(expected), actual))
}

func validateParameterBindings(parameterTypes map[string]reflect.Type, parameters ParameterValues) error {
	required := make([]string, 0, len(parameterTypes))
	for name := range parameterTypes {
		required = append(required, name)
	}
	sort.Strings(required)
	declared := make(map[string]struct{}, len(required))
	for _, name := range required {
		declared[name] = struct{}{}
		value, ok := parameters[name]
		if !ok {
			return NewError(ErrorInvalidRule, fmt.Sprintf("fire-and-forget parameter %q is not bound", parameterDisplayName(name)))
		}
		if err := validateParameterValue(name, value, parameterTypes[name]); err != nil {
			return err
		}
	}
	for name := range parameters {
		if _, ok := declared[name]; !ok {
			return NewError(ErrorInvalidRule, fmt.Sprintf("fire-and-forget parameter %q is not declared", name))
		}
	}
	return nil
}

// fireAndForgetJoinEvaluationOrder loads the preserved side first for a
// two-stream right outer query. This makes the optional left side eligible for
// a safe index candidate probe while keeping source indexes and result
// projection order unchanged. Dependency-driven method joins retain the
// topological order returned by methodJoinEvaluationOrder.
func fireAndForgetJoinEvaluationOrder(definition *joinDefinition) ([]int, error) {
	order, err := methodJoinEvaluationOrder(definition)
	if err != nil {
		return nil, err
	}
	sources := joinDefinitionSources(definition)
	if definition != nil && definition.kind == JoinRightOuter && len(sources) == 2 && len(order) == 2 && order[0] == 0 && order[1] == 1 {
		return []int{1, 0}, nil
	}
	return order, nil
}

// positionalParameterBindings translates the public variadic Go binding form
// into the internal immutable map used by every expression evaluator. It is
// intentionally plan-driven: the number, order and static type of values are
// taken from the built query rather than inferred from the supplied slice.
func positionalParameterBindings(plan Plan, values []any) (ParameterValues, error) {
	parameterTypes, err := queryParameterTypes(plan.query.env, plan.query)
	if err != nil {
		return nil, WrapError(ErrorInvalidRule, "parameters", err)
	}
	positions := make(map[int]reflect.Type)
	hasNamed := false
	for name, typ := range parameterTypes {
		if position, positional := positionalParameterPosition(name); positional {
			positions[position] = typ
		} else {
			hasNamed = true
		}
	}
	if hasNamed {
		return nil, NewError(ErrorInvalidRule, "query uses named substitution parameters; use ExecuteFireAndForgetWithParameters")
	}
	if len(positions) == 0 {
		if len(values) != 0 {
			return nil, NewError(ErrorInvalidRule, "query has no positional substitution parameters")
		}
		return ParameterValues{}, nil
	}
	if len(values) != len(positions) {
		if len(values) < len(positions) {
			return nil, NewError(ErrorInvalidRule, fmt.Sprintf("missing value for positional parameter %d", len(values)+1))
		}
		return nil, NewError(ErrorInvalidRule, fmt.Sprintf("received %d positional parameter values, expected %d", len(values), len(positions)))
	}
	parameters := make(ParameterValues, len(values))
	for index, value := range values {
		position := index + 1
		expected, exists := positions[position]
		if !exists {
			return nil, NewError(ErrorInvalidRule, fmt.Sprintf("positional parameter index %d is not declared", position))
		}
		if err := validateParameterValue(positionalParameterName(position), value, expected); err != nil {
			return nil, err
		}
		parameters[positionalParameterName(position)] = value
	}
	return parameters, nil
}

func (e *Engine) executeJoinFireAndForget(ctx context.Context, plan Plan, parameters ParameterValues) (QueryResult, error) {
	sources := joinDefinitionSources(plan.query.join)
	if len(sources) < 2 {
		return QueryResult{}, NewError(ErrorInvalidRule, "fire-and-forget join requires at least two sources")
	}
	if _, err := fireAndForgetJoinEvaluationOrder(plan.query.join); err != nil {
		return QueryResult{}, err
	}
	e.mu.Lock()
	now := e.clock.Now()
	variables := bindParameterValues(cloneValues(e.variables), parameters)
	e.mu.Unlock()
	runtime := newStatementRuntime(plan.query)
	runtime.engine = e
	runtime.initializeAt(now)
	runtime.ctx = ctx
	runtime.variables = variablesWithEngine(variables, e)
	runtime.joinState = &joinRuntimeState{sides: make([][]storedEvent, len(sources))}
	loaded := make([]bool, len(sources))
	evaluationOrder, err := fireAndForgetJoinEvaluationOrder(plan.query.join)
	if err != nil {
		return QueryResult{}, err
	}
	for _, index := range evaluationOrder {
		source := sources[index]
		base, err := sourceNode(source)
		if err != nil {
			return QueryResult{}, err
		}
		if base.kind != streamNamedWindow && base.kind != streamTable && base.kind != streamHistorical && base.kind != streamMethod {
			return QueryResult{}, NewError(ErrorInvalidRule, "fire-and-forget join sources must be named windows, tables, historical sources, or method sources")
		}
		input := source
		if base.kind == streamHistorical {
			input = replaceStreamBase(source, base, &streamNode{kind: streamSource, sourceName: base.historical.schema.Name(), sourceType: typeOf[any]()})
		} else if base.kind == streamMethod {
			input = replaceStreamBase(source, base, &streamNode{kind: streamSource, sourceName: base.method.schema.Name(), sourceType: typeOf[any]()})
		}
		if base.kind == streamMethod && base.method != nil && len(base.method.dependencies) > 0 {
			invocations, invocationErr := methodDependencyInvocations(base.method.dependencies, sources, runtime.joinState)
			if invocationErr != nil {
				return QueryResult{}, invocationErr
			}
			for _, invocation := range invocations {
				previous := runtime.methodDependencies
				runtime.methodDependencies = invocation.events
				events, pollErr := base.method.provider.Poll(ctx, MethodRequest{
					Now: now, Variables: visibleVariableValues(variables), Parameters: parameterValuesFromVariables(variables),
					Dependencies: cloneMethodDependencies(invocation.events),
					Invocation:   MethodInvocationContext{SourceName: base.sourceName, ContextPartitionID: -1},
				})
				runtime.methodDependencies = previous
				if pollErr != nil {
					return QueryResult{}, pollErr
				}
				if appendErr := appendFireAndForgetJoinSource(&runtime, &runtime.joinState.sides[index], input, events, now, invocation.lineage); appendErr != nil {
					return QueryResult{}, appendErr
				}
			}
			loaded[index] = true
			continue
		}
		selection, _ := plan.indexPlan.ForSource(index)
		events, usedIndex, err := e.snapshotFireAndForgetJoinSourceWithIndex(
			ctx, source, selection, index, plan.query.join, plan.query.joinWhere,
			runtime.joinState.sides, loaded, sourceIndexFilterExpressions(source), now, variables,
		)
		if err != nil {
			return QueryResult{}, err
		}
		if !usedIndex {
			events, err = e.snapshotFireAndForgetSource(ctx, base, now, variables)
		}
		if err != nil {
			return QueryResult{}, err
		}
		if appendErr := appendFireAndForgetJoinSource(&runtime, &runtime.joinState.sides[index], input, events, now, nil); appendErr != nil {
			return QueryResult{}, appendErr
		}
		loaded[index] = true
	}
	tuples := joinTuples(plan.query.join, runtime.joinState, now, &runtime)
	var batch ResultBatch
	if plan.query.aggregate != nil {
		batch, err = runtime.aggregateBatch(joinDeltaEvents(joinDelta{newTuples: tuples}, now), plan, now)
		if err != nil {
			return QueryResult{}, err
		}
	} else {
		batch = runtime.joinBatch(joinDeltaWithPairs(joinDelta{newTuples: tuples}), plan, now)
	}
	batch = runtime.applyOutput(plan.query.output, batch, false, now, plan)
	return QueryResult{Batch: batch}, nil
}

func appendFireAndForgetJoinSource(runtime *statementRuntime, side *[]storedEvent, input *streamNode, events []Event, now time.Time, lineage map[int]uint64) error {
	if runtime == nil || side == nil {
		return NewError(ErrorDependency, "fire-and-forget join runtime is nil")
	}
	for _, event := range events {
		delta, err := runtime.insert(input, event, now)
		if err != nil {
			return err
		}
		removeStoredEvents(side, delta.oldEvents)
		for _, inserted := range delta.newEvents {
			*side = append(*side, storedEvent{
				event: inserted, receivedAt: now, lineageID: runtime.nextJoinLineageID(), lineage: cloneMethodLineage(lineage),
			})
		}
	}
	return nil
}

func (e *Engine) executeContextJoinFireAndForget(ctx context.Context, plan Plan, selector ContextPartitionSelector, parameters ParameterValues) (QueryResult, error) {
	definition, ok := e.env.Context(plan.query.contextName)
	if !ok {
		return QueryResult{}, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", plan.query.contextName))
	}
	if definition.kind == ContextInitiatedTerminated {
		return QueryResult{}, NewError(ErrorInvalidRule, "fire-and-forget context selection does not support initiated-terminated lifecycle")
	}
	sources := joinDefinitionSources(plan.query.join)
	if len(sources) < 2 {
		return QueryResult{}, NewError(ErrorInvalidRule, "fire-and-forget context join requires at least two sources")
	}
	evaluationOrder, err := fireAndForgetJoinEvaluationOrder(plan.query.join)
	if err != nil {
		return QueryResult{}, err
	}
	e.mu.Lock()
	now := e.clock.Now()
	variables := bindParameterValues(cloneValues(e.variables), parameters)
	e.mu.Unlock()
	if result, used, indexedErr := e.executeContextJoinFireAndForgetWithIndex(
		ctx, plan, selector, definition, sources, evaluationOrder, now, variables,
	); used {
		return result, indexedErr
	}
	grouped := make([]map[string][]Event, len(sources))
	allKeys := make(map[string]struct{})
	for index, source := range sources {
		base, err := sourceNode(source)
		if err != nil {
			return QueryResult{}, err
		}
		if base.kind != streamNamedWindow && base.kind != streamTable && base.kind != streamHistorical && base.kind != streamMethod {
			return QueryResult{}, NewError(ErrorInvalidRule, "fire-and-forget context join sources must be named windows, tables, historical sources, or method sources")
		}
		if base.kind == streamMethod && base.method != nil && len(base.method.dependencies) > 0 {
			continue
		}
		events, err := e.snapshotFireAndForgetSource(ctx, base, now, variables)
		if err != nil {
			return QueryResult{}, err
		}
		byPartition := make(map[string][]Event)
		for _, event := range events {
			key, active, partitionErr := definition.partition(event, now, variables)
			if partitionErr != nil {
				return QueryResult{}, partitionErr
			}
			if !active {
				continue
			}
			byPartition[key] = append(byPartition[key], event)
			allKeys[key] = struct{}{}
		}
		grouped[index] = byPartition
	}
	keys := make([]string, 0, len(allKeys))
	for key := range allKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := ResultBatch{Time: now}
	for snapshotIndex, key := range keys {
		partitionID := e.contextPartitionSnapshotID(definition.name, key, snapshotIndex)
		representative := Event{}
		for index := range sources {
			if events := grouped[index][key]; len(events) > 0 {
				representative = events[0]
				break
			}
		}
		contextProperties := definition.contextPropertyValues(representative, now, variables, partitionID)
		descriptor := ContextPartitionDescriptor{ID: partitionID, Key: key, ContextName: definition.name, properties: cloneValues(contextProperties)}
		if !contextPartitionSelectedWithDescriptor(selector, descriptor) {
			continue
		}
		query := plan.query
		query.contextName = ""
		partitionPlan := plan
		partitionPlan.query = query
		runtime := newStatementRuntime(query)
		runtime.engine = e
		runtime.partitionContextName = definition.name
		runtime.partitionKey = key
		runtime.initializeAt(now)
		runtime.ctx = ctx
		runtime.partitionID = partitionID
		runtime.contextProperties = contextProperties
		runtime.variables = runtime.withContextVariables(variablesWithEngine(cloneValues(variables), e))
		runtime.variables = runtime.withContextProperties(runtime.variables)
		runtime.joinState = &joinRuntimeState{sides: make([][]storedEvent, len(sources))}
		for _, index := range evaluationOrder {
			source := sources[index]
			base, baseErr := sourceNode(source)
			if baseErr != nil {
				return QueryResult{}, baseErr
			}
			input := source
			if base.kind == streamHistorical {
				input = replaceStreamBase(source, base, &streamNode{kind: streamSource, sourceName: base.historical.schema.Name(), sourceType: typeOf[any]()})
			} else if base.kind == streamMethod {
				input = replaceStreamBase(source, base, &streamNode{kind: streamSource, sourceName: base.method.schema.Name(), sourceType: typeOf[any]()})
			}
			if base.kind == streamMethod && base.method != nil && len(base.method.dependencies) > 0 {
				invocations, invocationErr := methodDependencyInvocations(base.method.dependencies, sources, runtime.joinState)
				if invocationErr != nil {
					return QueryResult{}, invocationErr
				}
				for _, invocation := range invocations {
					previous := runtime.methodDependencies
					runtime.methodDependencies = invocation.events
					events, pollErr := base.method.provider.Poll(ctx, MethodRequest{
						Now: now, Variables: visibleVariableValues(runtime.variables), Parameters: parameterValuesFromVariables(runtime.variables),
						Dependencies: cloneMethodDependencies(invocation.events),
						Invocation:   runtime.methodInvocationContext(base.sourceName),
					})
					runtime.methodDependencies = previous
					if pollErr != nil {
						return QueryResult{}, pollErr
					}
					if appendErr := appendFireAndForgetJoinSource(&runtime, &runtime.joinState.sides[index], input, events, now, invocation.lineage); appendErr != nil {
						return QueryResult{}, appendErr
					}
				}
				continue
			}
			if appendErr := appendFireAndForgetJoinSource(&runtime, &runtime.joinState.sides[index], input, grouped[index][key], now, nil); appendErr != nil {
				return QueryResult{}, appendErr
			}
		}
		tuples := joinTuples(query.join, runtime.joinState, now, &runtime)
		var batch ResultBatch
		if query.aggregate != nil {
			batch, err = runtime.aggregateBatch(joinDeltaEvents(joinDelta{newTuples: tuples}, now), partitionPlan, now)
			if err != nil {
				return QueryResult{}, err
			}
		} else {
			batch = runtime.joinBatch(joinDeltaWithPairs(joinDelta{newTuples: tuples}), partitionPlan, now)
		}
		batch = runtime.applyOutput(query.output, batch, false, now, partitionPlan)
		result.New = append(result.New, batch.New...)
		result.Old = append(result.Old, batch.Old...)
	}
	if !result.empty() {
		result.Sequence = 1
	}
	return QueryResult{Batch: result}, nil
}

// executeContextJoinFireAndForgetWithIndex evaluates the safe physical
// candidate path for a context-scoped join. Context partitioning is a logical
// boundary, not a separate storage index: the first loaded source enumerates
// the possible partition keys and every indexed candidate is filtered back to
// the current key after lookup. This preserves selector and partition
// semantics while avoiding a full scan of the optional/target source.
//
// The helper handles inner joins (including an all-inner left-deep chain) and
// two-stream left/right outer joins without unidirectional sources. For an
// outer join, the evaluation order is required to load the preserved side
// first; the optional side can then be safely reduced to index candidates,
// including an empty candidate set which lets joinTuples emit the unmatched
// preserved row. FullOuter, mixed-edge, chained outer and unidirectional
// shapes return used=false so the established complete-snapshot implementation
// remains the source of truth.
func (e *Engine) executeContextJoinFireAndForgetWithIndex(
	ctx context.Context,
	plan Plan,
	selector ContextPartitionSelector,
	definition ContextDefinition,
	sources []*streamNode,
	evaluationOrder []int,
	now time.Time,
	variables map[string]Value,
) (QueryResult, bool, error) {
	if e == nil || plan.query.join == nil || len(sources) < 2 || len(evaluationOrder) == 0 {
		return QueryResult{}, false, nil
	}
	if !contextJoinIndexShapeAllowed(plan.query.join) {
		return QueryResult{}, false, nil
	}
	driverIndex := evaluationOrder[0]
	if driverIndex < 0 || driverIndex >= len(sources) {
		return QueryResult{}, false, nil
	}
	if !contextJoinIndexDriverAllowed(plan.query.join, driverIndex) {
		return QueryResult{}, false, nil
	}
	if base, err := sourceNode(sources[driverIndex]); err != nil || base.kind == streamMethod && base.method != nil && len(base.method.dependencies) > 0 {
		return QueryResult{}, false, nil
	}

	indexed := make(map[int]IndexSelection)
	for _, index := range evaluationOrder {
		if index == driverIndex {
			continue
		}
		selection, ok := plan.indexPlan.ForSource(index)
		if !ok || !contextJoinIndexSelectionUsable(selection) || !joinIndexCandidateAllowed(plan.query.join, index) {
			continue
		}
		base, err := sourceNode(sources[index])
		if err != nil || (base.kind != streamNamedWindow && base.kind != streamTable) {
			continue
		}
		indexed[index] = selection
	}
	if len(indexed) == 0 {
		return QueryResult{}, false, nil
	}

	driver, err := sourceNode(sources[driverIndex])
	if err != nil {
		return QueryResult{}, false, nil
	}
	driverEvents, err := e.snapshotFireAndForgetSource(ctx, driver, now, variables)
	if err != nil {
		return QueryResult{}, true, err
	}
	grouped := make([]map[string][]Event, len(sources))
	grouped[driverIndex] = make(map[string][]Event)
	allKeys := make(map[string]struct{})
	for _, event := range driverEvents {
		key, active, partitionErr := definition.partition(event, now, variables)
		if partitionErr != nil {
			return QueryResult{}, true, partitionErr
		}
		if !active {
			continue
		}
		grouped[driverIndex][key] = append(grouped[driverIndex][key], event)
		allKeys[key] = struct{}{}
	}
	groupedReady := make([]bool, len(sources))
	groupedReady[driverIndex] = true

	ensureGrouped := func(index int) (map[string][]Event, error) {
		if groupedReady[index] {
			return grouped[index], nil
		}
		base, baseErr := sourceNode(sources[index])
		if baseErr != nil {
			return nil, baseErr
		}
		events, snapshotErr := e.snapshotFireAndForgetSource(ctx, base, now, variables)
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		byPartition := make(map[string][]Event)
		for _, event := range events {
			key, active, partitionErr := definition.partition(event, now, variables)
			if partitionErr != nil {
				return nil, partitionErr
			}
			if active {
				byPartition[key] = append(byPartition[key], event)
			}
		}
		grouped[index] = byPartition
		groupedReady[index] = true
		return byPartition, nil
	}

	keys := make([]string, 0, len(allKeys))
	for key := range allKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := ResultBatch{Time: now}
	for snapshotIndex, key := range keys {
		partitionID := e.contextPartitionSnapshotID(definition.name, key, snapshotIndex)
		representative := grouped[driverIndex][key][0]
		contextProperties := definition.contextPropertyValues(representative, now, variables, partitionID)
		descriptor := ContextPartitionDescriptor{ID: partitionID, Key: key, ContextName: definition.name, properties: cloneValues(contextProperties)}
		if !contextPartitionSelectedWithDescriptor(selector, descriptor) {
			continue
		}

		query := plan.query
		query.contextName = ""
		partitionPlan := plan
		partitionPlan.query = query
		runtime := newStatementRuntime(query)
		runtime.engine = e
		runtime.partitionContextName = definition.name
		runtime.partitionKey = key
		runtime.initializeAt(now)
		runtime.ctx = ctx
		runtime.partitionID = partitionID
		runtime.contextProperties = contextProperties
		runtime.variables = runtime.withContextVariables(variablesWithEngine(cloneValues(variables), e))
		runtime.variables = runtime.withContextProperties(runtime.variables)
		runtime.joinState = &joinRuntimeState{sides: make([][]storedEvent, len(sources))}
		loaded := make([]bool, len(sources))
		for _, index := range evaluationOrder {
			if index < 0 || index >= len(sources) {
				return QueryResult{}, true, NewError(ErrorInvalidRule, "context fire-and-forget join evaluation order contains an invalid source")
			}
			source := sources[index]
			base, baseErr := sourceNode(source)
			if baseErr != nil {
				return QueryResult{}, true, baseErr
			}
			input := source
			if base.kind == streamHistorical {
				input = replaceStreamBase(source, base, &streamNode{kind: streamSource, sourceName: base.historical.schema.Name(), sourceType: typeOf[any]()})
			} else if base.kind == streamMethod {
				input = replaceStreamBase(source, base, &streamNode{kind: streamSource, sourceName: base.method.schema.Name(), sourceType: typeOf[any]()})
			}
			if base.kind == streamMethod && base.method != nil && len(base.method.dependencies) > 0 {
				invocations, invocationErr := methodDependencyInvocations(base.method.dependencies, sources, runtime.joinState)
				if invocationErr != nil {
					return QueryResult{}, true, invocationErr
				}
				for _, invocation := range invocations {
					previous := runtime.methodDependencies
					runtime.methodDependencies = invocation.events
					events, pollErr := base.method.provider.Poll(ctx, MethodRequest{
						Now: now, Variables: visibleVariableValues(runtime.variables), Parameters: parameterValuesFromVariables(runtime.variables),
						Dependencies: cloneMethodDependencies(invocation.events),
						Invocation:   runtime.methodInvocationContext(base.sourceName),
					})
					runtime.methodDependencies = previous
					if pollErr != nil {
						return QueryResult{}, true, pollErr
					}
					if appendErr := appendFireAndForgetJoinSource(&runtime, &runtime.joinState.sides[index], input, events, now, invocation.lineage); appendErr != nil {
						return QueryResult{}, true, appendErr
					}
				}
				loaded[index] = true
				continue
			}

			var events []Event
			if index == driverIndex {
				events = grouped[driverIndex][key]
			} else if selection, ok := indexed[index]; ok {
				candidate, usedIndex, lookupErr := e.snapshotFireAndForgetJoinSourceWithIndex(
					ctx, source, selection, index, plan.query.join, plan.query.joinWhere,
					runtime.joinState.sides, loaded, sourceIndexFilterExpressions(source), now, runtime.variables,
				)
				if lookupErr != nil {
					return QueryResult{}, true, lookupErr
				}
				if usedIndex {
					events, lookupErr = contextJoinPartitionEvents(definition, candidate, key, now, runtime.variables)
					if lookupErr != nil {
						return QueryResult{}, true, lookupErr
					}
				} else {
					fallback, fallbackErr := ensureGrouped(index)
					if fallbackErr != nil {
						return QueryResult{}, true, fallbackErr
					}
					events = fallback[key]
				}
			} else {
				fallback, fallbackErr := ensureGrouped(index)
				if fallbackErr != nil {
					return QueryResult{}, true, fallbackErr
				}
				events = fallback[key]
			}
			if appendErr := appendFireAndForgetJoinSource(&runtime, &runtime.joinState.sides[index], input, events, now, nil); appendErr != nil {
				return QueryResult{}, true, appendErr
			}
			loaded[index] = true
		}

		tuples := joinTuples(query.join, runtime.joinState, now, &runtime)
		var batch ResultBatch
		if query.aggregate != nil {
			batch, err = runtime.aggregateBatch(joinDeltaEvents(joinDelta{newTuples: tuples}, now), partitionPlan, now)
		} else {
			batch = runtime.joinBatch(joinDeltaWithPairs(joinDelta{newTuples: tuples}), partitionPlan, now)
		}
		if err != nil {
			return QueryResult{}, true, err
		}
		batch = runtime.applyOutput(query.output, batch, false, now, partitionPlan)
		result.New = append(result.New, batch.New...)
		result.Old = append(result.Old, batch.Old...)
	}
	if !result.empty() {
		result.Sequence = 1
	}
	return QueryResult{Batch: result}, true, nil
}

func contextJoinIndexShapeAllowed(definition *joinDefinition) bool {
	if definition == nil || joinDefinitionHasUnidirectional(definition) {
		return false
	}
	if len(definition.edges) > 0 {
		for _, edge := range definition.edges {
			if edge.kind != JoinInner {
				return false
			}
		}
		return true
	}
	sources := joinDefinitionSources(definition)
	switch definition.kind {
	case JoinInner:
		return true
	case JoinLeftOuter, JoinRightOuter:
		return len(sources) == 2
	default:
		return false
	}
}

func contextJoinIndexDriverAllowed(definition *joinDefinition, driverIndex int) bool {
	if definition == nil || len(definition.edges) > 0 {
		return true
	}
	switch definition.kind {
	case JoinLeftOuter:
		return driverIndex == 0
	case JoinRightOuter:
		return driverIndex == 1
	default:
		return true
	}
}

func contextJoinIndexSelectionUsable(selection IndexSelection) bool {
	if selection.IndexName == "" || strings.HasPrefix(selection.IndexName, "<") || len(selection.Columns) == 0 || len(selection.MatchedColumns) == 0 {
		return false
	}
	switch selection.Access {
	case IndexAccessEquality:
		return len(selection.MatchedColumns) == len(selection.Columns)
	case IndexAccessRange:
		return (selection.Backing == IndexBackingBTree || selection.Backing == IndexBackingUniqueBTree) && len(selection.MatchedColumns) <= len(selection.Columns)
	default:
		return false
	}
}

func contextJoinPartitionEvents(definition ContextDefinition, events []Event, key string, now time.Time, variables map[string]Value) ([]Event, error) {
	if len(events) == 0 {
		return nil, nil
	}
	result := make([]Event, 0, len(events))
	for _, event := range events {
		partitionKey, active, err := definition.partition(event, now, variables)
		if err != nil {
			return nil, err
		}
		if active && partitionKey == key {
			result = append(result, event)
		}
	}
	return result, nil
}

func (e *Engine) executeContextFireAndForget(ctx context.Context, plan Plan, selector ContextPartitionSelector, parameters ParameterValues) (QueryResult, error) {
	definition, ok := e.env.Context(plan.query.contextName)
	if !ok {
		return QueryResult{}, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", plan.query.contextName))
	}
	if definition.kind == ContextInitiatedTerminated {
		return QueryResult{}, NewError(ErrorInvalidRule, "fire-and-forget context selection does not support initiated-terminated lifecycle")
	}
	source, err := sourceNode(plan.query.input)
	if err != nil {
		return QueryResult{}, err
	}
	e.mu.Lock()
	now := e.clock.Now()
	variables := bindParameterValues(cloneValues(e.variables), parameters)
	e.mu.Unlock()
	selection, _ := plan.indexPlan.ForSource(0)
	events, err := e.snapshotFireAndForgetSourceWithIndex(ctx, source, selection, sourceIndexFilterExpressions(plan.query.input), now, variables)
	if err != nil {
		return QueryResult{}, err
	}
	grouped := make(map[string][]Event)
	for _, event := range events {
		key, active, partitionErr := definition.partition(event, now, variables)
		if partitionErr != nil {
			return QueryResult{}, partitionErr
		}
		if !active {
			continue
		}
		grouped[key] = append(grouped[key], event)
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := ResultBatch{Time: now}
	for snapshotIndex, key := range keys {
		partitionID := e.contextPartitionSnapshotID(definition.name, key, snapshotIndex)
		contextProperties := definition.contextPropertyValues(grouped[key][0], now, variables, partitionID)
		descriptor := ContextPartitionDescriptor{ID: partitionID, Key: key, ContextName: definition.name, properties: cloneValues(contextProperties)}
		if !contextPartitionSelectedWithDescriptor(selector, descriptor) {
			continue
		}
		query := plan.query
		query.contextName = ""
		partitionPlan := plan
		partitionPlan.query = query
		runtime := newStatementRuntime(query)
		runtime.engine = e
		runtime.partitionContextName = definition.name
		runtime.partitionKey = key
		runtime.initializeAt(now)
		runtime.ctx = ctx
		runtime.partitionID = partitionID
		runtime.contextProperties = contextProperties
		runtime.variables = runtime.withContextVariables(variablesWithEngine(cloneValues(variables), e))
		runtime.variables = runtime.withContextProperties(runtime.variables)
		delta := eventDelta{}
		input := query.input
		if source.kind == streamHistorical {
			input = replaceStreamBase(input, source, &streamNode{kind: streamSource, sourceName: source.historical.schema.Name(), sourceType: typeOf[any]()})
		} else if source.kind == streamMethod {
			input = replaceStreamBase(input, source, &streamNode{kind: streamSource, sourceName: source.method.schema.Name(), sourceType: typeOf[any]()})
		} else if source.kind == streamContained {
			childSchema, schemaErr := e.env.sourceSchema(source)
			if schemaErr != nil {
				return QueryResult{}, schemaErr
			}
			input = replaceStreamBase(input, source, &streamNode{kind: streamSource, sourceName: childSchema.Name(), sourceType: typeOf[any]()})
		}
		for _, event := range grouped[key] {
			inserted, insertErr := runtime.insert(input, event, now)
			if insertErr != nil {
				return QueryResult{}, insertErr
			}
			delta = mergeDelta(delta, inserted)
		}
		var batch ResultBatch
		if query.aggregate != nil {
			batch, err = runtime.aggregateBatch(delta, partitionPlan, now)
		} else if query.rowRecog != nil {
			batch = runtime.rowRecogBatch(delta, partitionPlan, now)
		} else {
			batch = runtime.batch(delta, partitionPlan, now)
		}
		if err != nil {
			return QueryResult{}, err
		}
		batch = runtime.applyOutput(query.output, batch, false, now, partitionPlan)
		result.New = append(result.New, batch.New...)
		result.Old = append(result.Old, batch.Old...)
	}
	if !result.empty() {
		result.Sequence = 1
	}
	return QueryResult{Batch: result}, nil
}

func (e *Engine) contextPartitionSnapshotID(contextName, key string, fallback int) int {
	if e == nil {
		return fallback
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if ids := e.contextPartitionIDs[contextName]; ids != nil {
		if id, ok := ids[key]; ok {
			return id
		}
	}
	return fallback
}

func (e *Engine) snapshotFireAndForgetSource(ctx context.Context, source *streamNode, now time.Time, variables map[string]Value) ([]Event, error) {
	return e.snapshotFireAndForgetSourceInternal(ctx, source, now, variables, false)
}

func (e *Engine) snapshotFireAndForgetSourceLocked(ctx context.Context, source *streamNode, now time.Time, variables map[string]Value) ([]Event, error) {
	return e.snapshotFireAndForgetSourceInternal(ctx, source, now, variables, true)
}

func (e *Engine) snapshotFireAndForgetSourceInternal(ctx context.Context, source *streamNode, now time.Time, variables map[string]Value, engineLocked bool) ([]Event, error) {
	if source == nil {
		return nil, NewError(ErrorDependency, "fire-and-forget source is nil")
	}
	if source.kind == streamContained {
		parents, err := e.snapshotFireAndForgetSourceInternal(ctx, source.input, now, variables, engineLocked)
		if err != nil {
			return nil, err
		}
		childSchema, err := e.env.sourceSchema(source)
		if err != nil {
			return nil, err
		}
		return expandContainedEvents(e.env, childSchema, source.contained, parents, now, variables)
	}
	switch source.kind {
	case streamNamedWindow:
		var window *NamedWindow
		var ok bool
		if engineLocked {
			window, ok = e.namedWindows[catalogKey(source.moduleName, source.sourceName)]
		} else {
			window, ok = e.NamedWindowInModule(source.moduleName, source.sourceName)
		}
		if !ok {
			return nil, NewError(ErrorUnknownName, fmt.Sprintf("named window %q is not registered", source.sourceName))
		}
		return window.Snapshot(ctx)
	case streamTable:
		var table *Table
		var ok bool
		if engineLocked {
			table, ok = e.tables[catalogKey(source.moduleName, source.sourceName)]
		} else {
			table, ok = e.TableInModule(source.moduleName, source.sourceName)
		}
		if !ok {
			return nil, NewError(ErrorUnknownName, fmt.Sprintf("table %q is not registered", source.sourceName))
		}
		rows, err := table.Snapshot(ctx)
		if err != nil {
			return nil, err
		}
		return tableRowsAsEvents(source, table.Definition(), rows, now)
	case streamHistorical:
		if source.historical == nil || source.historical.provider == nil {
			return nil, NewError(ErrorDependency, fmt.Sprintf("historical source %q has no provider", source.sourceName))
		}
		return source.historical.provider.Poll(ctx, HistoricalRequest{
			Now:        now,
			Variables:  visibleVariableValues(variables),
			Parameters: parameterValuesFromVariables(variables),
		})
	case streamMethod:
		if source.method == nil || source.method.provider == nil {
			return nil, NewError(ErrorDependency, fmt.Sprintf("method source %q has no provider", source.sourceName))
		}
		return source.method.provider.Poll(ctx, MethodRequest{
			Now:        now,
			Variables:  visibleVariableValues(variables),
			Parameters: parameterValuesFromVariables(variables),
			Invocation: MethodInvocationContext{
				SourceName: source.sourceName, ContextPartitionID: -1,
			},
		})
	default:
		return nil, NewError(ErrorInvalidRule, fmt.Sprintf("fire-and-forget source %q is not a named window, table, historical source, or method source", source.sourceName))
	}
}

func replaceStreamBase(node, target, replacement *streamNode) *streamNode {
	if node == nil {
		return nil
	}
	if node == target {
		return replacement
	}
	copyNode := *node
	copyNode.input = replaceStreamBase(node.input, target, replacement)
	return &copyNode
}

func bindParameterValues(variables map[string]Value, parameters ParameterValues) map[string]Value {
	if len(parameters) == 0 {
		return variables
	}
	result := cloneValues(variables)
	bound := make(map[string]Value, len(parameters))
	for name, value := range parameters {
		if value == nil {
			bound[name] = Null()
		} else {
			bound[name] = Present(value)
		}
	}
	result[parameterValuesVariable] = Present(bound)
	return result
}

func statementVariables(variables map[string]Value, parameters ParameterValues) map[string]Value {
	if len(parameters) == 0 {
		if _, inherited := variables[parameterValuesVariable]; !inherited {
			return variables
		}
		return visibleVariableValues(variables)
	}
	return bindParameterValues(visibleVariableValues(variables), parameters)
}

func cloneParameterValues(parameters ParameterValues) ParameterValues {
	if parameters == nil {
		return nil
	}
	result := make(ParameterValues, len(parameters))
	for name, value := range parameters {
		result[name] = value
	}
	return result
}

func parameterValuesFromVariables(variables map[string]Value) map[string]Value {
	if variables == nil {
		return nil
	}
	bound, ok := variables[parameterValuesVariable]
	if !ok || !bound.IsPresent() {
		return nil
	}
	values, ok := bound.Any().(map[string]Value)
	if !ok {
		return nil
	}
	return cloneValues(values)
}

func visibleVariableValues(variables map[string]Value) map[string]Value {
	if variables == nil {
		return nil
	}
	result := make(map[string]Value, len(variables))
	for name, value := range variables {
		if name == parameterValuesVariable || name == subqueryEngineVariable || name == subqueryRuntimeVariable {
			continue
		}
		result[name] = value
	}
	return result
}

// PreparedQuery captures an immutable plan and can be executed repeatedly.
// Closing is idempotent and prevents further execution.
type PreparedQuery struct {
	mu     sync.Mutex
	engine *Engine
	plan   Plan
	closed bool
}

func (e *Engine) PrepareFireAndForget(plan Plan) (*PreparedQuery, error) {
	if e == nil || e.env == nil {
		return nil, NewError(ErrorDependency, "engine has no environment")
	}
	if plan.query.env != e.env || plan.schemaVersion == "" {
		return nil, NewError(ErrorDependency, "plan does not belong to this engine")
	}
	return &PreparedQuery{engine: e, plan: plan}, nil
}

func (p *PreparedQuery) Execute(ctx context.Context) (QueryResult, error) {
	if p == nil {
		return QueryResult{}, NewError(ErrorState, "nil prepared query")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return QueryResult{}, NewError(ErrorState, "prepared query is closed")
	}
	return p.engine.ExecuteFireAndForget(ctx, p.plan)
}

// ExecuteWithParameters reuses the prepared plan with a fresh parameter
// binding snapshot. The handle remains serializable and safe for sequential
// reuse; parameter values are not retained after this call returns.
func (p *PreparedQuery) ExecuteWithParameters(ctx context.Context, parameters ParameterValues) (QueryResult, error) {
	if p == nil {
		return QueryResult{}, NewError(ErrorState, "nil prepared query")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return QueryResult{}, NewError(ErrorState, "prepared query is closed")
	}
	return p.engine.ExecuteFireAndForgetWithParameters(ctx, p.plan, parameters)
}

// ExecuteWithPositionalParameters reuses a prepared plan built with
// ParameterAt expressions. Values are supplied in 1-based parameter order;
// the prepared handle does not retain the slice after execution returns.
func (p *PreparedQuery) ExecuteWithPositionalParameters(ctx context.Context, values ...any) (QueryResult, error) {
	if p == nil {
		return QueryResult{}, NewError(ErrorState, "nil prepared query")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return QueryResult{}, NewError(ErrorState, "prepared query is closed")
	}
	return p.engine.ExecuteFireAndForgetWithPositionalParameters(ctx, p.plan, values...)
}

// ExecuteAndRoute evaluates the prepared plan and explicitly routes its
// selected result stream into the plan target.
func (p *PreparedQuery) ExecuteAndRoute(ctx context.Context) (QueryResult, error) {
	if p == nil {
		return QueryResult{}, NewError(ErrorState, "nil prepared query")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return QueryResult{}, NewError(ErrorState, "prepared query is closed")
	}
	return p.engine.ExecuteFireAndForgetAndRoute(ctx, p.plan)
}

// ExecuteAndRouteWithParameters reuses the prepared plan with a fresh
// parameter binding snapshot and explicitly routes the result stream.
func (p *PreparedQuery) ExecuteAndRouteWithParameters(ctx context.Context, parameters ParameterValues) (QueryResult, error) {
	if p == nil {
		return QueryResult{}, NewError(ErrorState, "nil prepared query")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return QueryResult{}, NewError(ErrorState, "prepared query is closed")
	}
	return p.engine.ExecuteFireAndForgetAndRouteWithParameters(ctx, p.plan, parameters)
}

func (p *PreparedQuery) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	return nil
}

func (p *PreparedQuery) Closed() bool {
	if p == nil {
		return true
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}
