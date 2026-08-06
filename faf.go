package esper

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"
)

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

// ExecuteFireAndForgetWithSelectorAndParameters combines context partition
// selection with execution-time parameter binding.
func (e *Engine) ExecuteFireAndForgetWithSelectorAndParameters(ctx context.Context, plan Plan, selector ContextPartitionSelector, parameters ParameterValues) (QueryResult, error) {
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

// ExecuteFireAndForgetAndRouteWithSelectorAndParameters combines context
// partition selection, parameter binding and explicit result routing.
func (e *Engine) ExecuteFireAndForgetAndRouteWithSelectorAndParameters(ctx context.Context, plan Plan, selector ContextPartitionSelector, parameters ParameterValues) (QueryResult, error) {
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
	events, err := e.snapshotFireAndForgetSource(ctx, source, now, variables)
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

func validateParameterValue(name string, value any, expected reflect.Type) error {
	if value == nil || expected == nil || expected == typeOf[any]() {
		return nil
	}
	actual := reflect.TypeOf(value)
	if actual.AssignableTo(expected) {
		return nil
	}
	return NewError(ErrorTypeMismatch, fmt.Sprintf("fire-and-forget parameter %q expects %s, got %s", name, parameterTypeDescription(expected), actual))
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
			return NewError(ErrorInvalidRule, fmt.Sprintf("fire-and-forget parameter %q is not bound", name))
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

func (e *Engine) executeJoinFireAndForget(ctx context.Context, plan Plan, parameters ParameterValues) (QueryResult, error) {
	sources := joinDefinitionSources(plan.query.join)
	if len(sources) < 2 {
		return QueryResult{}, NewError(ErrorInvalidRule, "fire-and-forget join requires at least two sources")
	}
	if _, err := methodJoinEvaluationOrder(plan.query.join); err != nil {
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
	evaluationOrder, err := methodJoinEvaluationOrder(plan.query.join)
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
			continue
		}
		events, err := e.snapshotFireAndForgetSource(ctx, base, now, variables)
		if err != nil {
			return QueryResult{}, err
		}
		if appendErr := appendFireAndForgetJoinSource(&runtime, &runtime.joinState.sides[index], input, events, now, nil); appendErr != nil {
			return QueryResult{}, appendErr
		}
	}
	tuples := joinTuples(plan.query.join, runtime.joinState, now, &runtime)
	batch := runtime.joinBatch(joinDeltaWithPairs(joinDelta{newTuples: tuples}), plan, now)
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
	evaluationOrder, err := methodJoinEvaluationOrder(plan.query.join)
	if err != nil {
		return QueryResult{}, err
	}
	e.mu.Lock()
	now := e.clock.Now()
	variables := bindParameterValues(cloneValues(e.variables), parameters)
	e.mu.Unlock()
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
		batch := runtime.joinBatch(joinDeltaWithPairs(joinDelta{newTuples: tuples}), partitionPlan, now)
		batch = runtime.applyOutput(query.output, batch, false, now, partitionPlan)
		result.New = append(result.New, batch.New...)
		result.Old = append(result.Old, batch.Old...)
	}
	if !result.empty() {
		result.Sequence = 1
	}
	return QueryResult{Batch: result}, nil
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
	events, err := e.snapshotFireAndForgetSource(ctx, source, now, variables)
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
	switch source.kind {
	case streamNamedWindow:
		var window *NamedWindow
		var ok bool
		if engineLocked {
			window, ok = e.namedWindows[source.sourceName]
		} else {
			window, ok = e.NamedWindow(source.sourceName)
		}
		if !ok {
			return nil, NewError(ErrorUnknownName, fmt.Sprintf("named window %q is not registered", source.sourceName))
		}
		return window.Snapshot(ctx)
	case streamTable:
		var table *Table
		var ok bool
		if engineLocked {
			table, ok = e.tables[source.sourceName]
		} else {
			table, ok = e.Table(source.sourceName)
		}
		if !ok {
			return nil, NewError(ErrorUnknownName, fmt.Sprintf("table %q is not registered", source.sourceName))
		}
		rows, err := table.Snapshot(ctx)
		if err != nil {
			return nil, err
		}
		definition := table.Definition()
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
			event.typeName = source.sourceName
			events = append(events, event)
		}
		return events, nil
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
