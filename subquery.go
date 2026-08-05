package esper

import (
	"context"
	"fmt"
	"sort"
)

const subqueryEngineVariable = "\x00esper.engine"

type subqueryEngineRef struct {
	engine *Engine
	locked bool
}

type subqueryDefinition struct {
	source              *streamNode
	predicate           Expr
	projection          Expr
	aggregateProjection bool
	quantified          bool
	comparison          SubqueryComparison
	cardinality         SubqueryCardinality
	orderBy             []SubqueryOrderKey
	offset              int
	limit               int
	limitSet            bool
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
	base, err := sourceNode(definition.source)
	if err != nil {
		return nil
	}
	now := outer.Now
	if now.IsZero() {
		now = e.Now()
	}
	var events []Event
	if engineLocked {
		events, err = e.snapshotFireAndForgetSourceLocked(context.Background(), base, now, outer.Variables)
	} else {
		events, err = e.snapshotFireAndForgetSource(context.Background(), base, now, outer.Variables)
	}
	if err != nil {
		return nil
	}
	query := Query{env: e.env, input: definition.source}
	runtime := newStatementRuntime(query)
	runtime.engine = e
	runtime.variables = variablesWithEngine(outer.Variables, e)
	type subqueryCandidate struct {
		value      Value
		evaluation EvalContext
	}
	candidates := make([]subqueryCandidate, 0, len(events))
	containsWindow := subquerySourceContainsWindow(definition.source)
	var aggregateGroup []Event
	if containsWindow {
		aggregateGroup = append([]Event(nil), events...)
	}
	lastAggregateDeltaHasHistory := false
	for _, event := range events {
		delta, insertErr := runtime.insert(definition.source, event, now)
		if insertErr != nil {
			return nil
		}
		if definition.aggregateProjection && containsWindow {
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
				OuterEvent: outer.Event,
				History:    historyForEvent(delta, candidate),
				Now:        now,
				Variables:  outer.Variables,
				Parameters: outer.Parameters,
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
			if definition.projection == nil {
				candidates = append(candidates, subqueryCandidate{value: Present(candidate), evaluation: evaluation})
				continue
			}
			candidates = append(candidates, subqueryCandidate{value: definition.projection.eval(evaluation), evaluation: evaluation})
		}
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
			value:      definition.projection.eval(evaluation),
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
