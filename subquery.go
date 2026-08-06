package esper

import (
	"context"
	"fmt"
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
	Predicate   Expression[bool]
	Cardinality SubqueryCardinality
	OrderBy     []SubqueryOrderKey
	Offset      int
	Limit       int
	LimitSet    bool
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

// SubqueryExists evaluates whether at least one event from a named-window or
// table source satisfies predicate. Field expressions in predicate address
// the inner source; OuterField expressions address the enclosing event.
func SubqueryExists(source RecordStream, predicate Expression[bool]) Expression[bool] {
	definition := &subqueryDefinition{source: source.node}
	if predicate != nil {
		definition.predicate = predicate
	}
	return makeSubqueryExpr[bool]("subquery-exists", "exists("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		values := evaluateSubqueryValues(definition, ctx)
		return Present(len(values) > 0)
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
		projection:          projection,
		aggregateProjection: isAggregateExpression(projection),
		cardinality:         config.Cardinality,
		orderBy:             append([]SubqueryOrderKey(nil), config.OrderBy...),
		offset:              config.Offset,
		limit:               config.Limit,
		limitSet:            config.LimitSet,
	}
	return makeSubqueryExpr[T]("subquery-value", "value("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		values := evaluateSubqueryValues(definition, ctx)
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
		projection:          projection,
		aggregateProjection: isAggregateExpression(projection),
		orderBy:             append([]SubqueryOrderKey(nil), config.OrderBy...),
		offset:              config.Offset,
		limit:               config.Limit,
		limitSet:            config.LimitSet,
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
		columns:             columns,
		multiColumn:         true,
		aggregateProjection: subqueryColumnsHaveAggregate(columns),
		cardinality:         config.Cardinality,
		orderBy:             append([]SubqueryOrderKey(nil), config.OrderBy...),
		offset:              config.Offset,
		limit:               config.Limit,
		limitSet:            config.LimitSet,
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
	definition := &subqueryDefinition{source: source.node, projection: projection, aggregateProjection: isAggregateExpression(projection)}
	if len(predicate) > 0 && predicate[0] != nil {
		definition.predicate = predicate[0]
	}
	return makeSubqueryExprWithChildren[bool]("subquery-in", "("+value.Description()+" in "+subqueryDescription(definition)+")", definition, []*exprNode{value.node()}, func(ctx EvalContext) Value {
		outer := value.eval(ctx)
		if !outer.IsPresent() {
			return Null()
		}
		values := evaluateSubqueryValues(definition, ctx)
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
// follows Esper aggregate null behavior: no numeric input yields Null.
func SubquerySum[T Numeric](source RecordStream, projection Expression[T], predicate ...Expression[bool]) Expression[T] {
	definition := subqueryWithProjection(source, projection, predicate...)
	return makeSubqueryExpr[T]("subquery-sum", "sum("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		var total float64
		found := false
		for _, candidate := range evaluateSubqueryValues(definition, ctx) {
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
// returns a float64 average, or Null when no numeric row remains.
func SubqueryAvg[T Numeric](source RecordStream, projection Expression[T], predicate ...Expression[bool]) Expression[float64] {
	definition := subqueryWithProjection(source, projection, predicate...)
	return makeSubqueryExpr[float64]("subquery-avg", "avg("+subqueryDescription(definition)+")", definition, func(ctx EvalContext) Value {
		var total float64
		var count int64
		for _, candidate := range evaluateSubqueryValues(definition, ctx) {
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
	definition := subqueryWithProjection(source, projection, predicate...)
	definition.quantified = true
	definition.comparison = comparison
	description := fmt.Sprintf("%s %s any (%s)", value.Description(), comparison.symbol(), subqueryDescription(definition))
	return makeSubqueryExprWithChildren[bool]("subquery-any", description, definition, []*exprNode{value.node()}, func(ctx EvalContext) Value {
		return evaluateQuantifiedSubquery(value.eval(ctx), evaluateSubqueryValues(definition, ctx), comparison, false)
	})
}

// SubquerySome is the SQL/Esper synonym for SubqueryAny.
func SubquerySome[T any](value Expression[T], source RecordStream, projection Expression[T], comparison SubqueryComparison, predicate ...Expression[bool]) Expression[bool] {
	return SubqueryAny[T](value, source, projection, comparison, predicate...)
}

// SubqueryAll applies a scalar comparison to every projected inner row.
func SubqueryAll[T any](value Expression[T], source RecordStream, projection Expression[T], comparison SubqueryComparison, predicate ...Expression[bool]) Expression[bool] {
	definition := subqueryWithProjection(source, projection, predicate...)
	definition.quantified = true
	definition.comparison = comparison
	description := fmt.Sprintf("%s %s all (%s)", value.Description(), comparison.symbol(), subqueryDescription(definition))
	return makeSubqueryExprWithChildren[bool]("subquery-all", description, definition, []*exprNode{value.node()}, func(ctx EvalContext) Value {
		return evaluateQuantifiedSubquery(value.eval(ctx), evaluateSubqueryValues(definition, ctx), comparison, true)
	})
}

func subqueryWithProjection(source RecordStream, projection Expr, predicate ...Expression[bool]) *subqueryDefinition {
	definition := &subqueryDefinition{source: source.node, projection: projection, aggregateProjection: isAggregateExpression(projection)}
	if len(predicate) > 0 && predicate[0] != nil {
		definition.predicate = predicate[0]
	}
	return definition
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
	return ok
}

func evaluateQuantifiedSubquery(left Value, values []Value, comparison SubqueryComparison, all bool) Value {
	if !left.IsPresent() || len(values) == 0 {
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
	if registry := subqueryRuntimeFromVariables(outer.Variables); registry != nil {
		if snapshot, ok := registry.snapshot(definition); ok {
			events = snapshot
			usingRuntimeSnapshot = true
		}
	}
	if !usingRuntimeSnapshot {
		contextName := ""
		contextPartition := ""
		if outer.Variables != nil {
			if value, ok := outer.Variables[subqueryContextNameVariable]; ok && value.IsPresent() {
				contextName, _ = value.Any().(string)
			}
			if value, ok := outer.Variables[subqueryContextPartitionVariable]; ok && value.IsPresent() {
				contextPartition, _ = value.Any().(string)
			}
		}
		if base.kind == streamNamedWindow && contextName != "" && contextPartition != "" {
			var window *NamedWindow
			if engineLocked {
				window = e.namedWindows[base.sourceName]
			} else {
				window, _ = e.NamedWindow(base.sourceName)
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
				Engine:     e,
				Event:      candidate,
				JoinEvents: append([]Event(nil), outer.JoinEvents...),
				OuterEvent: outer.Event,
				History:    historyForEvent(delta, candidate),
				Now:        now,
				Variables:  outer.Variables,
				Parameters: outer.Parameters,
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
					Engine:     e,
					Event:      event,
					JoinEvents: append([]Event(nil), outer.JoinEvents...),
					OuterEvent: outer.Event,
					Group:      aggregateGroup,
					History:    aggregateGroup,
					Now:        now,
					Variables:  outer.Variables,
					Parameters: outer.Parameters,
				}))
				if ok && matched {
					filtered = append(filtered, event)
				}
			}
			aggregateGroup = filtered
		}
		evaluation := EvalContext{
			Engine:       e,
			JoinEvents:   append([]Event(nil), outer.JoinEvents...),
			OuterEvent:   outer.Event,
			Group:        aggregateGroup,
			EverGroup:    aggregateGroup,
			AllGroup:     aggregateGroup,
			AllEverGroup: aggregateGroup,
			History:      aggregateGroup,
			Now:          now,
			Variables:    outer.Variables,
			Parameters:   outer.Parameters,
		}
		if len(aggregateGroup) > 0 {
			evaluation.Event = aggregateGroup[len(aggregateGroup)-1]
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
			OuterEvent:   outer.Event,
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
