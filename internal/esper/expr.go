package esper

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/big"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Expr is the analyzable expression contract used by the Builder and runtime.
// Implementations are intentionally created by the package constructors so a
// Plan never has to inspect an arbitrary closure to discover its fields.
type Expr interface {
	Type() reflect.Type
	Description() string
	eval(EvalContext) Value
	node() *exprNode
}

// Expression[T] carries the expected result type through Go's type checker.
// Type-changing expression combinators are top-level functions, not methods,
// because Go does not support method-level type parameters.
type Expression[T any] interface {
	Expr
	expressionMarker()
}

type exprNode struct {
	kind                            string
	typ                             reflect.Type
	description                     string
	fieldName                       string
	fieldSourceType                 reflect.Type
	literalValue                    any
	initialTarget                   bool
	tagName                         string
	variableName                    string
	parameterName                   string
	pluginName                      string
	pluginReady                     bool
	pluginAccess                    bool
	pluginFactory                   aggregatePluginFactory
	pluginEnvironment               *Environment
	aggregateMultiPluginName        string
	aggregateMultiPluginMethod      string
	aggregateMultiPluginReady       bool
	aggregateMultiPluginFactory     aggregateMultiPluginFactory
	aggregateMultiPluginEnvironment *Environment
	aggregateMultiStateKey          string
	aggregateMultiStateShared       bool
	aggregateMultiMethods           map[string]AggregateMultiPluginMethod
	scriptName                      string
	scriptEnvironment               *Environment
	methodName                      string
	containedParentLevels           int
	joinSource                      int
	previousOffset                  int
	enumInputRequired               bool
	enumParameterRequired           bool
	enumInvalidReason               string
	// enumMetadata mirrors Esper's enumeration method footprint and component
	// type metadata. It is derived entirely from the typed builder AST and is
	// never consulted as mutable runtime state.
	enumMetadata              *EnumMethodMetadata
	enumPluginName            string
	enumPluginEnvironment     *Environment
	enumPluginFactory         enumPluginFactory
	enumPluginReady           bool
	enumPluginFootprints      []EnumMethodFootprint
	enumPluginArguments       []enumPluginArgumentMeta
	dateTimePluginName        string
	dateTimePluginEnvironment *Environment
	dateTimePluginFactory     dateTimePluginFactory
	dateTimePluginReady       bool
	dateTimePluginFootprints  []DateTimeMethodFootprint
	dateTimePluginArguments   []dateTimePluginArgumentMeta
	dateTimePluginMetadata    *DateTimePluginMetadata
	configurationError        string
	expressionName            string
	expressionEnvironment     *Environment
	// expressionArguments are kept separate from children because children
	// also contains the referenced definition body after validation.  Keeping
	// the call-site arguments explicit lets the planner validate arity and
	// types without mistaking the body for an argument.
	expressionArguments []*exprNode
	expressionBody      *exprNode
	children            []*exprNode
	subquery            *subqueryDefinition
}

type typedExpr[T any] struct {
	n  *exprNode
	fn func(EvalContext) Value
}

func (e typedExpr[T]) Type() reflect.Type         { return e.n.typ }
func (e typedExpr[T]) Description() string        { return e.n.description }
func (e typedExpr[T]) eval(ctx EvalContext) Value { return e.fn(ctx) }
func (e typedExpr[T]) node() *exprNode            { return e.n }
func (e typedExpr[T]) expressionMarker()          {}

func typeOf[T any]() reflect.Type {
	var zero T
	typ := reflect.TypeOf(zero)
	if typ == nil {
		return reflect.TypeOf((*any)(nil)).Elem()
	}
	return typ
}

func makeExpr[T any](kind, description string, children []*exprNode, fn func(EvalContext) Value) Expression[T] {
	return typedExpr[T]{
		n:  &exprNode{kind: kind, typ: typeOf[T](), description: description, children: children},
		fn: fn,
	}
}

// ContextField reads a stable property of the current context partition.
// Built-in names include name, id, label (category contexts) and key1/key2...
// for segmented or initiated-terminated key expressions. Parent context
// properties in nested contexts are available as parent.<name>.
func ContextField[T any](name string) Expression[T] {
	name = strings.TrimSpace(name)
	description := "context.<invalid>"
	if name != "" {
		description = "context." + name
	}
	node := &exprNode{kind: "context-field", typ: typeOf[T](), description: description}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		if name == "" || ctx.Variables == nil {
			return Missing()
		}
		value, ok := ctx.Variables[contextVariableName(name)]
		if !ok {
			return Missing()
		}
		return value
	}}
}

// ContextName, ContextID and ContextLabel are typed conveniences for the
// built-in context properties exposed by ContextField.
func ContextName() Expression[string]  { return ContextField[string]("name") }
func ContextID() Expression[int]       { return ContextField[int]("id") }
func ContextLabel() Expression[string] { return ContextField[string]("label") }

// ContextStartTime and ContextEndTime expose the active temporal interval.
// They evaluate to Missing for non-temporal context kinds.
func ContextStartTime() Expression[time.Time] { return ContextField[time.Time]("startTime") }
func ContextEndTime() Expression[time.Time]   { return ContextField[time.Time]("endTime") }

// ContextInitiatingEvent and ContextTerminatingEvent expose the boundary
// events of an initiated-terminated context. They evaluate to Missing for
// context kinds that do not have the corresponding lifecycle event. Use
// Property or Method to continue with a typed event field/method.
func ContextInitiatingEvent() Expression[Event] {
	return ContextField[Event]("initiating_event")
}

func ContextTerminatingEvent() Expression[Event] {
	return ContextField[Event]("terminating_event")
}

// ContextPatternEvent returns the event captured by a named tag in the
// pattern that initiated or terminated the current context partition. Missing
// optional tags evaluate to Null, matching PatternStream tag semantics.
func ContextPatternEvent(tag string) Expression[Event] {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return makeExpr[Event]("context-pattern-event", "context.<invalid-pattern-tag>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{kind: "context-pattern-event", typ: typeOf[Event](), description: "context.pattern." + tag, tagName: tag}
	return typedExpr[Event]{n: node, fn: func(ctx EvalContext) Value {
		if ctx.Variables == nil {
			return Null()
		}
		value, ok := ctx.Variables[contextVariableName("pattern."+tag)]
		if !ok || !value.IsPresent() {
			return Null()
		}
		return value
	}}
}

// ContextPatternField reads a property from a captured context pattern tag.
// It is the typed chain-API counterpart of a context tag path such as
// context.a.symbol in Esper's object model.
func ContextPatternField[V any](tag, name string) Expression[V] {
	tag = strings.TrimSpace(tag)
	name = strings.TrimSpace(name)
	if tag == "" || name == "" {
		return makeExpr[V]("context-pattern-field", "context.<invalid-pattern-field>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{kind: "context-pattern-field", typ: typeOf[V](), description: "context." + tag + "." + name, tagName: tag, fieldName: name}
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value {
		if ctx.Variables == nil {
			return Null()
		}
		value, ok := ctx.Variables[contextVariableName("pattern."+tag)]
		if !ok || !value.IsPresent() {
			return Null()
		}
		return propertyValue(value.Any(), name)
	}}
}

func ContextKeyValue[T any](index int) Expression[T] {
	if index < 0 {
		return ContextField[T]("")
	}
	return ContextField[T](fmt.Sprintf("key%d", index+1))
}

// EvalContext contains the event and logical time visible to an expression.
type EvalContext struct {
	Event Event
	// JoinEvents contains the complete tuple currently being evaluated. It is
	// populated for Join projections and Join conditions so a correlated
	// subquery can refer to any outer source with JoinField/JoinEventValue.
	// Ordinary stream expressions leave it nil.
	JoinEvents []Event
	// OuterEvent is the event from the enclosing statement while evaluating a
	// correlated subquery. Ordinary expressions leave it empty.
	OuterEvent Event
	// ContainedParentEvent is the immediate parent event while evaluating a
	// child produced by Unnest/UnnestValues. It is deliberately separate from
	// OuterEvent: the latter is the enclosing statement scope used by existing
	// correlated expressions, while this field models Esper's contained-event
	// parent scope.
	ContainedParentEvent Event
	// Engine is populated by deployed statement evaluation. It is intentionally
	// absent from the public builder API; subquery expressions use it to obtain
	// a consistent named-window/table snapshot.
	Engine *Engine
	Group  []Event
	// GroupTags parallels Group with the pattern-match tag map of each group
	// event. Pattern-stream aggregates (for example sum(sb.intPrimitive) over
	// a pattern) evaluate their input per group event with that event's own
	// captured tags, matching Esper's aggregate over the pattern match stream.
	GroupTags []map[string]Event
	// InitialGroup preserves the target row as it existed before an ordered
	// on-trigger assignment list started. TableField follows the working row
	// while InitialTableField deliberately remains bound to this snapshot.
	InitialGroup []Event
	EverGroup    []Event
	// AllGroup and AllEverGroup are the statement-level current and retained
	// event ranges used by local group-by aggregates. They are populated by
	// aggregate statement evaluation; ordinary expressions fall back to Group
	// and EverGroup when the statement-level ranges are absent.
	AllGroup     []Event
	AllEverGroup []Event
	// LeavingEvents contains events that have left the current aggregate
	// window. Aggregate leaving expressions retain this history for the
	// lifetime of the aggregate group, matching Esper's stateful leaving().
	LeavingEvents []Event
	// History is the current data-window view in insertion order. When an
	// expression is evaluated for a newly arriving event, the current event is
	// included as the last item. It is primarily consumed by Prev and Prior.
	History []Event
	// PreviousHistory overrides History for Prev/Prior evaluation when a
	// runtime maintains a separate previous-access stream. Match-recognize
	// uses this to preserve arrival-order PREV values after a data window has
	// evicted the referenced event.
	PreviousHistory []Event
	// PreviousWindowAccess marks PreviousHistory as an absolute window-access
	// sequence (index zero is the access head) rather than the default
	// newest-relative history. Sorted and time-order views use this mode.
	PreviousWindowAccess bool
	// PriorHistory is the arrival-order history used by Prior when a view has
	// a separate previous-access ordering, such as TimeOrder or Sort.
	PriorHistory []Event
	// PriorHistorySet distinguishes an explicitly empty prior history from an
	// absent view-specific history. This matters for old-stream rows after the
	// referenced event has left a sorted/time-order view.
	PriorHistorySet bool
	// PreviousTagEvents supplies the event selected by a tag-aware Prev/Prior
	// expression to nested TagField/TagFieldAt and tag enumeration expressions.
	// It is private-in-practice runtime context: fluent callers normally use
	// PrevTag/PriorTag or compose Prev/Prior with TagField.
	PreviousTagEvents map[string]Event
	IsLeaving         bool
	Tags              map[string]Event
	TagValues         map[string][]Event
	Now               time.Time
	Variables         map[string]Value
	Parameters        map[string]Value
	// Metadata is the statement-level evaluation context exposed by
	// CurrentEvaluationContext. It is set only by statement projection paths;
	// direct expression evaluation keeps the zero value and the expression
	// normalizes its partition id to -1.
	Metadata             ExpressionEvaluationContext
	evaluationContextSet bool

	// Output counters are populated only while an output-when expression is
	// evaluated. They model Esper's count_insert/count_remove and total forms
	// without exposing mutable runtime state to ordinary expressions.
	OutputInsertCount    int64
	OutputRemoveCount    int64
	OutputInsertTotal    int64
	OutputRemoveTotal    int64
	OutputLastOutputTime time.Time

	// Expression-window built-ins are populated while an expression or
	// expression-batch view evaluates its keep/trigger predicate. They are
	// explicit runtime context rather than opaque callbacks, so UDFs can
	// consume them through ViewReference/ExpiredCount and the plan remains
	// analyzable.
	WindowReference    []Event
	WindowExpiredCount int64

	// resultRow is populated only while a projected result is being ordered.
	// It lets analyzable result-field expressions sort Row projections without
	// exposing an untyped callback to the planner.
	resultRow *Row

	// enumValue/enumIndex/enumSize are populated only while an enumerable
	// expression evaluates its analyzable element expression. They are kept
	// private so callers cannot smuggle arbitrary runtime state into a Plan;
	// EnumElement, EnumIndex and EnumSize are the public AST constructors.
	enumValue  Value
	enumIndex  int64
	enumSize   int64
	enumActive bool

	enumAccumulator       Value
	enumAccumulatorActive bool
	// enumPluginStateValue is the current state getter visible while a
	// registered enumeration-plugin lambda evaluates. It models Esper's
	// EnumMethodLambdaParameterTypeStateGetter without exposing the mutable
	// plugin state object to ordinary expressions.
	enumPluginStateValue  Value
	enumPluginStateActive bool
	// groupingValues and groupingPresent are populated only while a
	// dimensional aggregate result is evaluated. They let a grouped key
	// evaluate to Null at subtotal levels without exposing runtime state to
	// ordinary expressions.
	groupingValues  map[string]Value
	groupingPresent map[string]bool

	// aggregatePluginStates is private runtime state for registered aggregate
	// factories. It is set only while evaluating a result row and is keyed by
	// expression node so each aggregate group owns an independent state.
	aggregatePluginStates map[*exprNode]aggregatePluginState
	// aggregateMultiPluginStates is private runtime state for multi-function
	// aggregate extensions. The key is the provider's explicit state key plus
	// local aggregate scope; an unshared expression uses its node identity.
	aggregateMultiPluginStates map[string]aggregateMultiPluginState
	aggregateMultiScope        string
	// aggregateEvaluation distinguishes an explicitly empty aggregate group
	// from an ordinary projection that has only a current Event. Access
	// aggregates such as First use the latter as a one-row group, but an empty
	// aggregate group must remain empty after removals.
	aggregateEvaluation bool
}

// groupEventContext builds the per-event evaluation context for one member
// of ctx.Group. When the group carries parallel pattern-match tags
// (ctx.GroupTags), the member's own tags are exposed so tag-based aggregate
// inputs such as sum(sb.intPrimitive) evaluate per match row like Esper's
// aggregate over the pattern match stream.
func (ctx EvalContext) groupEventContext(event Event, index int) EvalContext {
	result := EvalContext{
		Event:      event,
		OuterEvent: ctx.OuterEvent,
		Engine:     ctx.Engine,
		Now:        ctx.Now,
		Variables:  ctx.Variables,
		Parameters: ctx.Parameters,
	}
	if index >= 0 && index < len(ctx.GroupTags) {
		result.Tags = ctx.GroupTags[index]
	}
	return result
}

const parameterValuesVariable = "\x00esper.parameters"

// positionalParameterPrefix is deliberately outside the user-visible name
// space.  Positional substitution parameters still use the same immutable
// expression node and runtime value channel as named parameters, but their
// binding key cannot collide with a caller's named Parameter value.
const positionalParameterPrefix = "\x00esper.positional:"

func positionalParameterName(position int) string {
	return positionalParameterPrefix + strconv.Itoa(position)
}

func positionalParameterPosition(name string) (int, bool) {
	if !strings.HasPrefix(name, positionalParameterPrefix) {
		return 0, false
	}
	position, err := strconv.Atoi(strings.TrimPrefix(name, positionalParameterPrefix))
	if err != nil || position < 1 {
		return 0, false
	}
	return position, true
}

// Leaving reports whether the current result is being emitted on the remove
// stream. It is false for insert-stream evaluation and for contexts that do
// not have a stream transition (such as a direct expression check).
func Leaving(predicate ...Expression[bool]) Expression[bool] {
	children := make([]*exprNode, 0, len(predicate))
	description := "leaving()"
	if len(predicate) == 1 && predicate[0] != nil {
		description = "leaving(filter:" + predicate[0].Description() + ")"
	}
	for _, item := range predicate {
		if item == nil {
			children = append(children, nil)
			continue
		}
		children = append(children, item.node())
	}
	return makeExpr[bool]("leaving", description, children, func(ctx EvalContext) Value {
		if len(predicate) > 1 || (len(predicate) == 1 && predicate[0] == nil) {
			return Missing()
		}
		if len(predicate) == 0 {
			return Present(ctx.IsLeaving)
		}
		for _, event := range ctx.LeavingEvents {
			value := predicate[0].eval(EvalContext{Event: event, Now: ctx.Now, Variables: ctx.Variables, Parameters: ctx.Parameters})
			matched, ok := boolValue(value)
			if ok && matched {
				return Present(true)
			}
		}
		return Present(false)
	})
}

// IStream reports whether the current result is being emitted on the insert
// stream. It is the inverse of Leaving and the Go-style counterpart of
// Esper's istream() built-in function: true for insert-stream events and
// false for remove-stream events.
func IStream() Expression[bool] {
	return makeExpr[bool]("istream", "istream()", nil, func(ctx EvalContext) Value {
		return Present(!ctx.IsLeaving)
	})
}

// Field creates an analyzable property expression. The source event type T is
// a compile-time marker; V is the expected property result type.
func Field[T, V any](name string) Expression[V] {
	if strings.TrimSpace(name) == "" {
		return makeExpr[V]("field", "<invalid-field>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{kind: "field", typ: typeOf[V](), description: name, fieldName: name, fieldSourceType: typeOf[T]()}
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value {
		if ctx.groupingValues != nil {
			if value, ok := ctx.groupingValues[groupingNodeKey(node, name)]; ok {
				return value
			}
		}
		return ctx.Event.Get(name)
	}}
}

// JoinField reads a field from one source of a join tuple. It is intentionally
// source-indexed instead of relying on aliases or an implicit current side so
// the same expression remains analyzable when a two-way join is expanded to a
// multi-way join aggregate.
func JoinField[V any](source int, name string) Expression[V] {
	name = strings.TrimSpace(name)
	description := fmt.Sprintf("join[%d].<invalid>", source)
	if source >= 0 && name != "" {
		description = fmt.Sprintf("join[%d].%s", source, name)
	}
	node := &exprNode{kind: "join-field", typ: typeOf[V](), description: description, fieldName: name, joinSource: source}
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value {
		if source < 0 || name == "" {
			return Missing()
		}
		events := ctx.JoinEvents
		if events == nil {
			tuple, ok := ctx.Event.Underlying().(joinTuple)
			if !ok {
				return Missing()
			}
			events = tuple.events
		}
		if source >= len(events) {
			return Missing()
		}
		event := events[source]
		if !event.Schema().valid() {
			return Null()
		}
		return event.Get(name)
	}}
}

// JoinEventValue returns the typed event value from one source of a join
// tuple. It is useful for access aggregates such as First/Last/Window and for
// continuing with Property or Method after an aggregate access operation.
func JoinEventValue[T any](source int) Expression[T] {
	description := fmt.Sprintf("join[%d].event()", source)
	node := &exprNode{kind: "join-event", typ: typeOf[T](), description: description, joinSource: source}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		if source < 0 {
			return Missing()
		}
		events := ctx.JoinEvents
		if events == nil {
			tuple, ok := ctx.Event.Underlying().(joinTuple)
			if !ok {
				return Missing()
			}
			events = tuple.events
		}
		if source >= len(events) {
			return Missing()
		}
		event := events[source]
		if !event.Schema().valid() {
			return Null()
		}
		if typeOf[T]() == reflect.TypeOf(Event{}) {
			var value T
			reflect.ValueOf(&value).Elem().Set(reflect.ValueOf(event))
			return Present(value)
		}
		underlying := event.Underlying()
		if underlying == nil {
			return Null()
		}
		value, ok := underlying.(T)
		if !ok {
			return Missing()
		}
		return Present(value)
	}}
}

// NestedField reads a property of an Event-typed expression result. It is the
// chainable counterpart of Esper's fragment property navigation ("a.id") on
// streams whose events carry joined or inserted event fragments.
func NestedField[V any](host Expression[Event], name string) Expression[V] {
	if host == nil || strings.TrimSpace(name) == "" {
		return makeExpr[V]("nested-field", "<invalid-nested-field>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{kind: "nested-field", typ: typeOf[V](), description: host.Description() + "." + name, fieldName: name, children: []*exprNode{host.node()}}
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value {
		value := host.eval(ctx)
		if !value.IsPresent() {
			return Null()
		}
		event, ok := value.Any().(Event)
		if !ok {
			return Missing()
		}
		if !event.Schema().valid() {
			return Null()
		}
		return event.Get(name)
	}}
}

// EventValue exposes the current event as a typed expression. It is useful for
// Go callers that need access aggregates to return the selected event itself,
// for example MinBy[Trade, float64](EventValue[Trade](), price).
func EventValue[T any]() Expression[T] {
	return makeExpr[T]("event-value", "event()", nil, func(ctx EvalContext) Value {
		if ctx.Event.Schema().Name() == "" {
			return Missing()
		}
		if reflect.TypeOf(Event{}) == typeOf[T]() {
			var value T
			reflect.ValueOf(&value).Elem().Set(reflect.ValueOf(ctx.Event))
			return Present(value)
		}
		underlying := ctx.Event.Underlying()
		if underlying == nil {
			return Null()
		}
		value, ok := underlying.(T)
		if ok {
			return Present(value)
		}
		if ctx.Event.Schema().goType == nil {
			if materialized, err := materializeEventValueAs[T](ctx.Event); err == nil {
				return Present(materialized)
			}
		}
		return Missing()
	})
}

// transposeValue is the evaluation payload produced by a Transpose selection.
// The transpose route materializes the marked value into the registered route
// target schema during projection; the marker itself carries no environment or
// schema reference so eval stays pure.
type transposeValue struct {
	value any
}

// Transpose marks a single expression result as the underlying event of an
// insert-into route. It is the Go fluent counterpart of Esper's transpose()
// select-clause function: the wrapped value becomes the routed event's
// underlying object instead of being projected into named columns.
//
// The expression is valid only as the sole projection of an insert-into route
// (or one transpose alongside non-transpose properties when the target event
// type was auto-created as a Wrapper/Pair, which the Go API models as a
// pre-registered Map target). These restrictions are enforced by Build; a
// bare non-route Transpose evaluates to the wrapped value with no side effect,
// mirroring Esper's treatment of transpose in a where-clause or as a non-top
// level expression.
func Transpose[T any](expression Expression[T]) Expression[Event] {
	if expression == nil {
		return makeExpr[Event]("transpose", "transpose(<nil>)", nil, func(EvalContext) Value {
			return Present(transposeValue{value: nil})
		})
	}
	return makeExpr[Event]("transpose", "transpose("+expression.Description()+")",
		[]*exprNode{expression.node()},
		func(ctx EvalContext) Value {
			return Present(transposeValue{value: expression.eval(ctx).Any()})
		})
}

// materializeEventValueAs maps a map-backed event onto T. This is the
// window(*) counterpart of SendRecord's typed materialization: an aggregate
// may retain map-represented events (matching Esper map event types) while a
// typed WindowAccessBy projection still expects the corresponding Go struct.
func materializeEventValueAs[T any](event Event) (T, error) {
	var zero T
	targetType := typeOf[T]()
	target := targetType
	pointer := false
	if target != nil && target.Kind() == reflect.Pointer {
		pointer = true
		target = target.Elem()
	}
	if target == nil || target.Kind() != reflect.Struct {
		return zero, fmt.Errorf("esper: EventValue target %s is not a struct or struct pointer", targetType)
	}
	schema := event.Schema()
	if schema.goType != nil {
		return zero, fmt.Errorf("esper: EventValue cannot materialize %s from typed event %s", typeOf[T](), schema.Name())
	}
	targetValue := reflect.New(target).Elem()
	value, ok := event.Underlying().(map[string]any)
	if !ok {
		return zero, fmt.Errorf("esper: EventValue map materialization expects map[string]any, got %T", event.Underlying())
	}
	for _, field := range schema.Fields() {
		if field.Type == nil {
			continue
		}
		item, exists := value[field.Name]
		if !exists || item == nil {
			continue
		}
		if err := setStructField(targetValue, field.Name, item, schema.resolution); err != nil {
			return zero, err
		}
	}
	materialized := targetValue
	if pointer {
		materialized = reflect.New(target)
		materialized.Elem().Set(targetValue)
	}
	return materialized.Interface().(T), nil
}

// OuterField reads a property from the event of the enclosing statement while
// a correlated subquery evaluates. It is deliberately explicit so a rule's
// inner and outer scopes remain visible in Go code.
func OuterField[V any](name string) Expression[V] {
	if strings.TrimSpace(name) == "" {
		return makeExpr[V]("outer-field", "<invalid-outer-field>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{kind: "outer-field", typ: typeOf[V](), description: "outer." + name, fieldName: name}
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value { return ctx.OuterEvent.Get(name) }}
}

// ContainedParentField reads a property from the immediate parent event of a
// contained child. It is the explicit Go counterpart of a contained-event
// where clause referring to a field of the enclosing event.
func ContainedParentField[V any](name string) Expression[V] {
	if strings.TrimSpace(name) == "" {
		return makeExpr[V]("contained-parent-field", "<invalid-contained-parent-field>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{kind: "contained-parent-field", typ: typeOf[V](), description: "contained.parent." + name, fieldName: name, containedParentLevels: 1}
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value {
		parent := ctx.ContainedParentEvent
		if !parent.Schema().valid() {
			parent, _ = ctx.Event.Ancestor(1)
		}
		if !parent.Schema().valid() {
			return Missing()
		}
		return parent.Get(name)
	}}
}

// ContainedParentEvent returns the immediate parent event of an Unnest child.
// It is useful when a projection needs the parent fragment itself rather
// than one of its fields.
func ContainedParentEvent() Expression[Event] {
	node := &exprNode{kind: "contained-parent-event", typ: typeOf[Event](), description: "contained.parent()", containedParentLevels: 1}
	return typedExpr[Event]{n: node, fn: func(ctx EvalContext) Value {
		parent := ctx.ContainedParentEvent
		if !parent.Schema().valid() {
			parent, _ = ctx.Event.Ancestor(1)
		}
		if !parent.Schema().valid() {
			return Null()
		}
		return Present(parent)
	}}
}

// ContainedAncestorEvent returns a parent at an explicit nesting level. Level
// 1 is equivalent to ContainedParentEvent; non-positive levels are invalid.
func ContainedAncestorEvent(level int) Expression[Event] {
	if level <= 0 {
		return makeExpr[Event]("contained-ancestor-event", "<invalid-contained-ancestor>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{kind: "contained-ancestor-event", typ: typeOf[Event](), description: fmt.Sprintf("contained.ancestor(%d)", level), containedParentLevels: level}
	return typedExpr[Event]{n: node, fn: func(ctx EvalContext) Value {
		ancestor, ok := ctx.Event.Ancestor(level)
		if !ok {
			return Null()
		}
		return Present(ancestor)
	}}
}

// ContainedAncestorField reads a property from a nested contained ancestor.
// It is the chain-API counterpart of selecting a parent/grandparent fragment
// while traversing nested arrays.
func ContainedAncestorField[V any](level int, name string) Expression[V] {
	if level <= 0 || strings.TrimSpace(name) == "" {
		return makeExpr[V]("contained-ancestor-field", "<invalid-contained-ancestor-field>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{kind: "contained-ancestor-field", typ: typeOf[V](), description: fmt.Sprintf("contained.ancestor(%d).%s", level, name), fieldName: name, containedParentLevels: level}
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value {
		ancestor, ok := ctx.Event.Ancestor(level)
		if !ok {
			return Missing()
		}
		return ancestor.Get(name)
	}}
}

// ResultField reads a named column from a projected Row. It is primarily
// useful for Match Recognize order-by clauses, where Esper orders by measure
// aliases after pattern evaluation rather than by the input event.
func ResultField[V any](name string) Expression[V] {
	if strings.TrimSpace(name) == "" {
		return makeExpr[V]("result-field", "<invalid-result-field>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{kind: "result-field", typ: typeOf[V](), description: name, fieldName: name}
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value {
		if ctx.resultRow == nil {
			return Missing()
		}
		return ctx.resultRow.Get(name)
	}}
}

// Property resolves one named property from a value expression. It is the
// Go-style equivalent of a nested event/map/bean property path and is useful
// after ArrayAt when an object-array field contains nested values.
func Property[T any](object Expr, name string) Expression[T] {
	if object == nil || strings.TrimSpace(name) == "" {
		return makeExpr[T]("property", "property(<invalid>)", nil, func(EvalContext) Value { return Missing() })
	}
	description := "property(" + object.Description() + "." + name + ")"
	return makeExpr[T]("property", description, []*exprNode{object.node()}, func(ctx EvalContext) Value {
		value := object.eval(ctx)
		if !value.IsPresent() {
			return value
		}
		return castPropertyValue[T](propertyValue(value.Any(), name))
	})
}

// Method invokes a zero- or multi-argument exported Go method on a value
// expression. It is the explicit Go counterpart of chained event-method
// access such as first().myMethod(), while keeping the receiver and argument
// expressions visible in the AST. A missing method, incompatible argument or
// non-nil error return evaluates to Missing. Go accessors returning
// (value, bool) use the bool as an optional-value flag: false evaluates to
// Null instead of exposing the value type's zero value.
func Method[T any](object Expr, name string, arguments ...Expr) Expression[T] {
	return methodExpression[T]("method", object, name, arguments...)
}

// DuckMethod invokes an exported Go method on a dynamically typed receiver.
// Missing methods, missing receivers and incompatible dynamic results become
// Null, matching Esper's duck-typed dot-method behavior. Use Method when a
// missing method should remain distinguishable as Missing.
func DuckMethod[T any](object Expr, name string, arguments ...Expr) Expression[T] {
	return methodExpression[T]("duck-method", object, name, arguments...)
}

func methodExpression[T any](kind string, object Expr, name string, arguments ...Expr) Expression[T] {
	name = strings.TrimSpace(name)
	children := make([]*exprNode, 0, 1+len(arguments))
	if object != nil {
		children = append(children, object.node())
	}
	for _, argument := range arguments {
		if argument != nil {
			children = append(children, argument.node())
		}
	}
	description := kind + "(<invalid>)"
	if object != nil && name != "" {
		parts := make([]string, 0, len(arguments))
		for _, argument := range arguments {
			parts = append(parts, expressionDescription(argument))
		}
		description = kind + "(" + object.Description() + "." + name + "(" + strings.Join(parts, ",") + "))"
	}
	node := &exprNode{kind: kind, typ: typeOf[T](), description: description, methodName: name, children: children}
	if object == nil || object.node() == nil {
		node.configurationError = "method receiver is required"
	} else if name == "" {
		node.configurationError = "method name is required"
	}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		if object == nil || name == "" {
			if kind == "duck-method" {
				return Null()
			}
			return Missing()
		}
		target := object.eval(ctx)
		if !target.IsPresent() {
			if kind == "duck-method" {
				return Null()
			}
			return target
		}
		values := make([]Value, 0, len(arguments))
		for _, argument := range arguments {
			if argument == nil {
				if kind == "duck-method" {
					return Null()
				}
				return Missing()
			}
			values = append(values, argument.eval(ctx))
		}
		result := invokeMethod[T](target.Any(), name, values)
		if kind == "duck-method" && result.IsMissing() {
			return Null()
		}
		return result
	}}
}

type reflectMethodStatus uint8

const (
	reflectMethodMissing reflectMethodStatus = iota
	reflectMethodValue
	reflectMethodNull
)

func invokeMethod[T any](underlying any, name string, arguments []Value) (result Value) {
	defer func() {
		if recover() != nil {
			result = Missing()
		}
	}()
	targets := []any{underlying}
	if event, ok := underlying.(Event); ok {
		targets = append([]any{event.Underlying()}, targets...)
	}
	for _, target := range targets {
		value, status := invokeReflectMethod(target, name, arguments)
		switch status {
		case reflectMethodNull:
			return Null()
		case reflectMethodMissing:
			continue
		}
		return reflectMethodResult[T](value)
	}
	return Missing()
}

func invokeReflectMethod(underlying any, name string, arguments []Value) (reflect.Value, reflectMethodStatus) {
	if underlying == nil {
		return reflect.Value{}, reflectMethodMissing
	}
	target := reflect.ValueOf(underlying)
	method := target.MethodByName(name)
	if !method.IsValid() && target.Kind() != reflect.Pointer {
		address := reflect.New(target.Type())
		address.Elem().Set(target)
		method = address.MethodByName(name)
	}
	if !method.IsValid() {
		return reflect.Value{}, reflectMethodMissing
	}
	methodType := method.Type()
	if !methodType.IsVariadic() && methodType.NumIn() != len(arguments) {
		return reflect.Value{}, reflectMethodMissing
	}
	callArguments := make([]reflect.Value, 0, len(arguments))
	for index, argument := range arguments {
		parameterIndex := index
		if methodType.IsVariadic() && index >= methodType.NumIn()-1 {
			parameterIndex = methodType.NumIn() - 1
		}
		parameterType := methodType.In(parameterIndex)
		if methodType.IsVariadic() && parameterIndex == methodType.NumIn()-1 {
			parameterType = parameterType.Elem()
		}
		if !argument.IsPresent() {
			if isNilableType(parameterType) {
				callArguments = append(callArguments, reflect.Zero(parameterType))
				continue
			}
			return reflect.Value{}, reflectMethodMissing
		}
		value := reflect.ValueOf(argument.Any())
		if value.Type().AssignableTo(parameterType) {
			callArguments = append(callArguments, value)
			continue
		}
		if value.Type().ConvertibleTo(parameterType) && (numericTypes(value.Type(), parameterType) || parameterType.Kind() == reflect.Interface) {
			callArguments = append(callArguments, value.Convert(parameterType))
			continue
		}
		return reflect.Value{}, reflectMethodMissing
	}
	results := method.Call(callArguments)
	if len(results) == 0 {
		return reflect.Value{}, reflectMethodMissing
	}
	if len(results) > 1 {
		errorType := reflect.TypeOf((*error)(nil)).Elem()
		if results[1].Type().Implements(errorType) {
			if isNilableType(results[1].Type()) {
				if !results[1].IsNil() {
					return reflect.Value{}, reflectMethodMissing
				}
			} else if !results[1].IsZero() {
				return reflect.Value{}, reflectMethodMissing
			}
		} else if results[1].Kind() == reflect.Bool && !results[1].Bool() {
			// Go accessors commonly use (value, bool) to distinguish an absent
			// value from the type's zero value. Preserve that distinction in a
			// rule projection as Null, matching Esper access-aggregate behavior.
			return reflect.Value{}, reflectMethodNull
		}
	}
	return results[0], reflectMethodValue
}

func reflectMethodResult[T any](value reflect.Value) Value {
	if !value.IsValid() {
		return Missing()
	}
	if isNilableType(value.Type()) && value.IsNil() {
		return Null()
	}
	expected := typeOf[T]()
	if value.Type().AssignableTo(expected) {
		return Present(value.Interface())
	}
	if value.Type().ConvertibleTo(expected) && (numericTypes(value.Type(), expected) || expected.Kind() == reflect.Interface) {
		return Present(value.Convert(expected).Interface())
	}
	return Missing()
}

func isNilableType(typ reflect.Type) bool {
	if typ == nil {
		return false
	}
	switch typ.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return true
	default:
		return false
	}
}

func propertyValue(underlying any, name string) Value {
	if underlying == nil {
		return Null()
	}
	if value, ok := underlying.(Value); ok {
		return value
	}
	if event, ok := underlying.(Event); ok {
		return event.Get(name)
	}
	if row, ok := underlying.(Row); ok {
		return row.Get(name)
	}
	return getPropertyPath(underlying, name, func(value any, property string) Value {
		return (Schema{resolution: PropertyCaseSensitive}).getOne(value, property)
	})
}

func castPropertyValue[T any](value Value) Value {
	if !value.IsPresent() {
		return value
	}
	target := typeOf[T]()
	source := reflect.ValueOf(value.Any())
	if !source.IsValid() {
		return Null()
	}
	if source.Type().AssignableTo(target) {
		return Present(source.Interface())
	}
	for source.IsValid() && (source.Kind() == reflect.Pointer || source.Kind() == reflect.Interface) {
		if source.IsNil() {
			return Null()
		}
		source = source.Elem()
	}
	if !source.IsValid() {
		return Null()
	}
	if source.Type().AssignableTo(target) {
		return Present(source.Interface())
	}
	if source.Type().ConvertibleTo(target) && (numericTypes(source.Type(), target) || target.Kind() == reflect.Interface) {
		return Present(source.Convert(target).Interface())
	}
	return castValue[T](Present(source.Interface()))
}

// TableField reads a property from the target table row during a table
// on-trigger predicate or assignment. The current trigger event remains
// available through Field, so expressions can compare both sides without a
// closure that would be invisible to the planner.
func TableField[V any](name string) Expression[V] {
	return targetField[V]("table-field", "table."+name, name)
}

// InitialTableField reads the target table row before the current ordered
// assignment list began. It is the Go-native counterpart of Esper's
// initial.property access and is useful when a later assignment needs both
// the working value and the original value.
func InitialTableField[V any](name string) Expression[V] {
	return initialTargetField[V]("table-field", "initial.table."+name, name)
}

// NamedWindowField is the equivalent target-row expression for a named-window
// on-trigger operation. It is kept distinct from TableField so validation can
// reject accidentally mixing state targets.
func NamedWindowField[V any](name string) Expression[V] {
	return targetField[V]("named-window-field", "named-window."+name, name)
}

// InitialNamedWindowField is the named-window counterpart of
// InitialTableField.
func InitialNamedWindowField[V any](name string) Expression[V] {
	return initialTargetField[V]("named-window-field", "initial.named-window."+name, name)
}

func targetField[V any](kind, description, name string) Expression[V] {
	return targetFieldWithScope[V](kind, description, name, false)
}

func initialTargetField[V any](kind, description, name string) Expression[V] {
	return targetFieldWithScope[V](kind, description, name, true)
}

func targetFieldWithScope[V any](kind, description, name string, initial bool) Expression[V] {
	if strings.TrimSpace(name) == "" {
		return makeExpr[V](kind, "<invalid-target-field>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{kind: kind, typ: typeOf[V](), description: description, fieldName: name, initialTarget: initial}
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value {
		// A target-row expression may be evaluated while the active row scope
		// has been switched to the triggering event (for example a no-key
		// table's matched merge assignments). The InitialGroup retains the
		// actual target row in that case and must win over the working Group.
		if len(ctx.InitialGroup) > 0 && (node.initialTarget || len(ctx.Group) == 0) {
			return ctx.InitialGroup[0].Get(name)
		}
		if len(ctx.Group) > 0 {
			return ctx.Group[0].Get(name)
		}
		if len(ctx.InitialGroup) > 0 {
			return ctx.InitialGroup[0].Get(name)
		}
		return Missing()
	}}
}

// Prev evaluates expression against an event at a relative position in the
// current data window. Offset zero addresses the newest event (normally the
// current event), offset one addresses the preceding event, and so on. When
// the requested position is not retained, Prev returns Null.
//
// This mirrors Esper's view-relative previous-value family while keeping the
// input expression fully analyzable by the Go planner.
func Prev[V any](offset int, expression Expression[V]) Expression[V] {
	return previousExpression[V]("prev", offset, expression, false)
}

// Prior evaluates expression against an event before the current event. A
// zero offset is the immediately preceding event, a one offset is two events
// back, and so on. Prior returns Null when no such event is retained.
func Prior[V any](offset int, expression Expression[V]) Expression[V] {
	return previousExpression[V]("prior", offset, expression, true)
}

// PrevTag evaluates a property of a named Match Recognize tag against an
// arrival-order previous event. It is the explicit Go counterpart of
// PREV(A.property, offset) while retaining a typed, chainable expression.
func PrevTag[V any](offset int, tag, name string) Expression[V] {
	return Prev[V](offset, TagField[V](tag, name))
}

// PriorTag evaluates a property of a named Match Recognize tag before the
// current event. Offset zero addresses the immediately preceding event.
func PriorTag[V any](offset int, tag, name string) Expression[V] {
	return Prior[V](offset, TagField[V](tag, name))
}

// PrevTail evaluates expression from the oldest end of the current window.
// Offset zero addresses the tail event, offset one the next event, and so on.
// For sorted/time-order windows the tail follows the view's sorted access
// order; for ordinary windows it follows insertion order.
func PrevTail[V any](offset int, expression Expression[V]) Expression[V] {
	if expression == nil {
		return makeExpr[V]("prev-tail", fmt.Sprintf("prev-tail(%d,<nil>)", offset), nil, func(EvalContext) Value { return Null() })
	}
	description := fmt.Sprintf("prev-tail(%d,%s)", offset, expression.Description())
	return makeExpr[V]("prev-tail", description, []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		history := previousWindowHistory(ctx)
		if offset < 0 || len(history) == 0 {
			return Null()
		}
		index := offset
		if ctx.PreviousWindowAccess {
			index = len(history) - 1 - offset
		}
		if ctx.PreviousWindowAccess {
			nested := ctx
			nested.History = history
			nested.PreviousHistory = history
			return evaluatePreviousAt[V](expression, nested, index)
		}
		return evaluatePreviousAt[V](expression, ctx, index)
	})
}

// PrevCount returns the number of events currently visible to previous-value
// access. A zero-sized active window returns zero; a leaving row without a
// retained history returns Null.
func PrevCount[V any](expression Expression[V]) Expression[int64] {
	children := []*exprNode(nil)
	description := "prev-count(<nil>)"
	if expression != nil {
		children = []*exprNode{expression.node()}
		description = "prev-count(" + expression.Description() + ")"
	}
	return makeExpr[int64]("prev-count", description, children, func(ctx EvalContext) Value {
		history := previousWindowHistory(ctx)
		if expression == nil || (ctx.IsLeaving && history == nil) {
			return Null()
		}
		return Present(int64(len(history)))
	})
}

// PrevWindow evaluates expression for every event visible to previous-value
// access. Ordinary windows return newest-to-oldest; sorted/time-order windows
// retain their view access order.
func PrevWindow[V any](expression Expression[V]) Expression[[]V] {
	children := []*exprNode(nil)
	description := "prev-window(<nil>)"
	if expression != nil {
		children = []*exprNode{expression.node()}
		description = "prev-window(" + expression.Description() + ")"
	}
	return makeExpr[[]V]("prev-window", description, children, func(ctx EvalContext) Value {
		history := previousWindowHistory(ctx)
		if expression == nil || len(history) == 0 || (ctx.IsLeaving && history == nil) {
			return Null()
		}
		events := append([]Event(nil), history...)
		if !ctx.PreviousWindowAccess {
			reverseEvents(events)
		}
		values := make([]V, len(events))
		for index, event := range events {
			nested := ctx
			nested.Event = event
			if ctx.PreviousWindowAccess {
				nested.History = append([]Event(nil), events...)
				nested.PreviousHistory = append([]Event(nil), events...)
				nested.PreviousWindowAccess = true
			} else {
				nested.History = append([]Event(nil), events[:index+1]...)
				nested.PreviousHistory = nil
				nested.PreviousWindowAccess = false
			}
			nested.PriorHistory = nil
			nested.PriorHistorySet = false
			value := expression.eval(nested)
			if !value.IsPresent() {
				continue
			}
			converted, err := As[V](value)
			if err != nil {
				return Missing()
			}
			values[index] = converted
		}
		return Present(values)
	})
}

func evaluatePreviousAt[V any](expression Expression[V], ctx EvalContext, index int) Value {
	if index < 0 || index >= len(ctx.History) {
		return Null()
	}
	nested := ctx
	nested.Event = ctx.History[index]
	if ctx.PreviousWindowAccess {
		nested.History = append([]Event(nil), ctx.History...)
	} else {
		nested.History = append([]Event(nil), ctx.History[:index+1]...)
	}
	nested.PreviousHistory = append([]Event(nil), ctx.PreviousHistory...)
	nested.PreviousWindowAccess = ctx.PreviousWindowAccess
	nested.PriorHistory = append([]Event(nil), ctx.PriorHistory...)
	nested.PriorHistorySet = ctx.PriorHistorySet
	return expression.eval(nested)
}

func reverseEvents(events []Event) {
	for left, right := 0, len(events)-1; left < right; left, right = left+1, right-1 {
		events[left], events[right] = events[right], events[left]
	}
}

func previousWindowHistory(ctx EvalContext) []Event {
	if ctx.PreviousWindowAccess {
		return ctx.PreviousHistory
	}
	return ctx.History
}

func previousExpression[V any](kind string, offset int, expression Expression[V], prior bool) Expression[V] {
	if expression == nil {
		return makeExpr[V](kind, fmt.Sprintf("%s(%d,<nil>)", kind, offset), nil, func(EvalContext) Value { return Null() })
	}
	description := fmt.Sprintf("%s(%d,%s)", kind, offset, expression.Description())
	node := &exprNode{kind: kind, typ: typeOf[V](), description: description, previousOffset: offset, children: []*exprNode{expression.node()}}
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value {
		return evaluatePreviousOffset[V](ctx, offset, expression, prior)
	}}
}

// TagField reads a property from a named Pattern or Match Recognize tag. A
// valid tag that is absent from an optional match evaluates to Null, matching
// Esper's measure semantics; an unknown event property remains Missing.
func TagField[V any](tag, name string) Expression[V] {
	if strings.TrimSpace(tag) == "" || strings.TrimSpace(name) == "" {
		return makeExpr[V]("tag-field", "<invalid-tag-field>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{kind: "tag-field", typ: typeOf[V](), description: tag + "." + name, fieldName: name, tagName: tag}
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value {
		if ctx.PreviousTagEvents != nil {
			if event, ok := ctx.PreviousTagEvents[tag]; ok {
				return event.Get(name)
			}
		}
		if ctx.Tags != nil {
			if event, ok := ctx.Tags[tag]; ok {
				return event.Get(name)
			}
		}
		if ctx.TagValues != nil {
			events := ctx.TagValues[tag]
			if len(events) > 0 {
				return events[len(events)-1].Get(name)
			}
		}
		return Null()
	}}
}

// PatternEvent returns the Event captured by a named event-pattern tag. It is
// useful when a PatternStream is used as a Join source, where the tag itself
// becomes a property of the materialized pattern result event.
func PatternEvent(tag string) Expression[Event] {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return makeExpr[Event]("pattern-event", "<invalid-pattern-event>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{kind: "pattern-event", typ: typeOf[Event](), description: "pattern." + tag, tagName: tag}
	return typedExpr[Event]{n: node, fn: func(ctx EvalContext) Value {
		if ctx.PreviousTagEvents != nil {
			if event, ok := ctx.PreviousTagEvents[tag]; ok {
				return Present(event)
			}
		}
		if ctx.Tags != nil {
			if event, ok := ctx.Tags[tag]; ok {
				return Present(event)
			}
		}
		if ctx.TagValues != nil {
			if events := ctx.TagValues[tag]; len(events) > 0 {
				return Present(events[len(events)-1])
			}
		}
		return Null()
	}}
}

// TagFieldAt reads a zero-based event from a repeated pattern tag and then
// resolves one of its properties. A negative or out-of-range index evaluates
// to Null, matching optional/repeated Match Recognize measures.
func TagFieldAt[V any](tag string, index int, name string) Expression[V] {
	if strings.TrimSpace(tag) == "" || strings.TrimSpace(name) == "" || index < 0 {
		return makeExpr[V]("tag-field-at", "<invalid-tag-field-at>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{kind: "tag-field-at", typ: typeOf[V](), description: fmt.Sprintf("%s[%d].%s", tag, index, name), fieldName: name, tagName: tag}
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value {
		if event, ok := ctx.PreviousTagEvents[tag]; ok {
			if index == 0 {
				return event.Get(name)
			}
			return Null()
		}
		if ctx.TagValues == nil || index >= len(ctx.TagValues[tag]) {
			return Null()
		}
		return ctx.TagValues[tag][index].Get(name)
	}}
}

// TagCount returns the number of events captured by a repeated pattern tag.
// For non-repeating tags the count is either zero or one. It is intentionally
// a normal expression so it can be used in PatternStream projections and
// downstream result expressions without introducing EPL syntax.
func TagCount(tag string) Expression[int64] {
	if strings.TrimSpace(tag) == "" {
		return makeExpr[int64]("tag-count", "<invalid-tag-count>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{kind: "tag-count", typ: typeOf[int64](), description: "count(" + tag + ")", fieldName: tag, tagName: tag}
	return typedExpr[int64]{n: node, fn: func(ctx EvalContext) Value {
		if _, ok := ctx.PreviousTagEvents[tag]; ok {
			return Present(int64(1))
		}
		if ctx.TagValues != nil {
			return Present(int64(len(ctx.TagValues[tag])))
		}
		if ctx.Tags != nil {
			if _, ok := ctx.Tags[tag]; ok {
				return Present(int64(1))
			}
		}
		return Present(int64(0))
	}}
}

// TagSize is the expressive alias for TagCount used by Match Recognize
// enumeration-style measures. It keeps the API about the captured tag rather
// than exposing a string aggregate spelling.
func TagSize(tag string) Expression[int64] { return TagCount(tag) }

// TagEvents exposes a defensive copy of all events captured by a repeated
// row-recognition tag. Optional tags evaluate to Null; a present tag with no
// events is represented by an empty slice. Callers can combine it with
// ArrayAt, Property and other typed Go expressions without mutating runtime
// recognition state.
func TagEvents(tag string) Expression[[]Event] {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return makeExpr[[]Event]("tag-events", "<invalid-tag-events>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{kind: "tag-events", typ: typeOf[[]Event](), description: "events(" + tag + ")", tagName: tag}
	return typedExpr[[]Event]{n: node, fn: func(ctx EvalContext) Value {
		events := rowRecogTagEvents(ctx, tag)
		if events == nil {
			return Null()
		}
		return Present(append([]Event(nil), events...))
	}}
}

// TagSum evaluates a numeric expression over the events captured by a named
// tag. It is the Go-style counterpart of Esper's repeated-tag sumOf
// enumeration and is useful in DEFINE predicates as well as measures.
func TagSum[T Numeric](tag string, expression Expression[T]) AggregateExpression[T] {
	return tagNumericAggregate[T]("tag-sum", "sum", tag, expression, func(values []float64) float64 {
		var total float64
		for _, value := range values {
			total += value
		}
		return total
	})
}

// TagAvg evaluates a numeric expression over a named repeated tag.
func TagAvg[T Numeric](tag string, expression Expression[T]) AggregateExpression[float64] {
	return tagNumericAggregateFloat("tag-avg", "avg", tag, expression, func(values []float64) float64 {
		if len(values) == 0 {
			return 0
		}
		var total float64
		for _, value := range values {
			total += value
		}
		return total / float64(len(values))
	})
}

// TagMin and TagMax evaluate an ordered expression over a named repeated
// tag. Null and Missing element values are ignored, matching the ordinary
// aggregate contract.
func TagMin[T Ordered](tag string, expression Expression[T]) AggregateExpression[T] {
	return tagExtremeAggregate[T]("tag-min", "min", tag, expression, true)
}

func TagMax[T Ordered](tag string, expression Expression[T]) AggregateExpression[T] {
	return tagExtremeAggregate[T]("tag-max", "max", tag, expression, false)
}

// TagFirst and TagLast return the first/last present value captured by a tag.
func TagFirst[T any](tag string, expression Expression[T]) AggregateExpression[T] {
	return tagPositionAggregate[T]("tag-first", "first", tag, expression, false)
}

func TagLast[T any](tag string, expression Expression[T]) AggregateExpression[T] {
	return tagPositionAggregate[T]("tag-last", "last", tag, expression, true)
}

// TagAny and TagAll are enumeration predicates evaluated against each event
// captured by a tag. TagAll follows Go's vacuous-truth convention and returns
// true for an absent/empty tag, while TagAny returns false.
func TagAny(tag string, predicate Expression[bool]) Expression[bool] {
	return tagEnumerationPredicate("tag-any", "any", tag, predicate, false)
}

func TagAll(tag string, predicate Expression[bool]) Expression[bool] {
	return tagEnumerationPredicate("tag-all", "all", tag, predicate, true)
}

func rowRecogTagEvents(ctx EvalContext, tag string) []Event {
	if ctx.PreviousTagEvents != nil {
		if event, ok := ctx.PreviousTagEvents[tag]; ok {
			return []Event{event}
		}
	}
	if ctx.TagValues != nil {
		if events, ok := ctx.TagValues[tag]; ok {
			return events
		}
	}
	if ctx.Tags != nil {
		if event, ok := ctx.Tags[tag]; ok {
			return []Event{event}
		}
	}
	return nil
}

func rowRecogTagElementContext(ctx EvalContext, events []Event, index int) EvalContext {
	nested := ctx
	if index < 0 || index >= len(events) {
		return nested
	}
	nested.Event = events[index]
	nested.Group = append([]Event(nil), events...)
	nested.History = append([]Event(nil), events[:index+1]...)
	return nested
}

func tagAggregateNode(kind, name, tag string, expression Expr) (*exprNode, string, bool) {
	tag = strings.TrimSpace(tag)
	if tag == "" || expression == nil || expression.node() == nil {
		return &exprNode{kind: kind, typ: typeOf[any](), description: kind + "(<invalid>)", tagName: tag}, kind + "(<invalid>)", false
	}
	description := name + "(" + tag + "." + expression.Description() + ")"
	return &exprNode{kind: kind, typ: expression.Type(), description: description, tagName: tag, children: []*exprNode{expression.node()}}, description, true
}

func tagNumericAggregate[T Numeric](kind, name, tag string, expression Expression[T], combine func([]float64) float64) AggregateExpression[T] {
	node, _, valid := tagAggregateNode(kind, name, tag, expression)
	if !valid {
		node.typ = typeOf[T]()
		return aggregateExpr[T]{typedExpr: typedExpr[T]{n: node, fn: func(EvalContext) Value { return Missing() }}}
	}
	return aggregateExpr[T]{typedExpr: typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		events := rowRecogTagEvents(ctx, tag)
		values := make([]float64, 0, len(events))
		for index := range events {
			value, ok := numericValue(expression.eval(rowRecogTagElementContext(ctx, events, index)))
			if ok {
				values = append(values, value)
			}
		}
		if len(values) == 0 {
			return Null()
		}
		return Present(convertNumeric[T](combine(values)))
	}}}
}

func tagNumericAggregateFloat(kind, name, tag string, expression Expr, combine func([]float64) float64) AggregateExpression[float64] {
	node, _, valid := tagAggregateNode(kind, name, tag, expression)
	if !valid {
		node.typ = typeOf[float64]()
		return aggregateExpr[float64]{typedExpr: typedExpr[float64]{n: node, fn: func(EvalContext) Value { return Missing() }}}
	}
	return aggregateExpr[float64]{typedExpr: typedExpr[float64]{n: node, fn: func(ctx EvalContext) Value {
		events := rowRecogTagEvents(ctx, tag)
		values := make([]float64, 0, len(events))
		for index := range events {
			value, ok := numericValue(expression.eval(rowRecogTagElementContext(ctx, events, index)))
			if ok {
				values = append(values, value)
			}
		}
		if len(values) == 0 {
			return Null()
		}
		return Present(combine(values))
	}}}
}

func tagExtremeAggregate[T Ordered](kind, name, tag string, expression Expression[T], minimum bool) AggregateExpression[T] {
	node, _, valid := tagAggregateNode(kind, name, tag, expression)
	if !valid {
		node.typ = typeOf[T]()
		return aggregateExpr[T]{typedExpr: typedExpr[T]{n: node, fn: func(EvalContext) Value { return Missing() }}}
	}
	return aggregateExpr[T]{typedExpr: typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		events := rowRecogTagEvents(ctx, tag)
		var result Value
		for index := range events {
			value := expression.eval(rowRecogTagElementContext(ctx, events, index))
			if !value.IsPresent() {
				continue
			}
			if !result.IsPresent() {
				result = value
				continue
			}
			comparison, ok := compareValues(value, result)
			if ok && ((minimum && comparison < 0) || (!minimum && comparison > 0)) {
				result = value
			}
		}
		if !result.IsPresent() {
			return Null()
		}
		return result
	}}}
}

func tagPositionAggregate[T any](kind, name, tag string, expression Expression[T], last bool) AggregateExpression[T] {
	node, _, valid := tagAggregateNode(kind, name, tag, expression)
	if !valid {
		node.typ = typeOf[T]()
		return aggregateExpr[T]{typedExpr: typedExpr[T]{n: node, fn: func(EvalContext) Value { return Missing() }}}
	}
	return aggregateExpr[T]{typedExpr: typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		events := rowRecogTagEvents(ctx, tag)
		if last {
			for index := len(events) - 1; index >= 0; index-- {
				value := expression.eval(rowRecogTagElementContext(ctx, events, index))
				if value.IsPresent() {
					return value
				}
			}
		} else {
			for index := range events {
				value := expression.eval(rowRecogTagElementContext(ctx, events, index))
				if value.IsPresent() {
					return value
				}
			}
		}
		return Null()
	}}}
}

func tagEnumerationPredicate(kind, name, tag string, predicate Expression[bool], all bool) Expression[bool] {
	tag = strings.TrimSpace(tag)
	if tag == "" || predicate == nil || predicate.node() == nil {
		return makeExpr[bool](kind, kind+"(<invalid>)", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{kind: kind, typ: typeOf[bool](), description: name + "(" + tag + ")", tagName: tag, children: []*exprNode{predicate.node()}}
	return typedExpr[bool]{n: node, fn: func(ctx EvalContext) Value {
		events := rowRecogTagEvents(ctx, tag)
		if len(events) == 0 {
			return Present(all)
		}
		for index := range events {
			value, ok := boolValue(predicate.eval(rowRecogTagElementContext(ctx, events, index)))
			if !ok {
				return Present(false)
			}
			if all && !value {
				return Present(false)
			}
			if !all && value {
				return Present(true)
			}
		}
		return Present(all)
	}}
}

// VariableRef creates an analyzable reference to a registered runtime
// variable. The reference is deliberately named in the expression tree so a
// Plan can validate and serialize the dependency without inspecting a
// closure.
func VariableRef[T any](name string) Expression[T] {
	if strings.TrimSpace(name) == "" {
		return makeExpr[T]("variable", "<invalid-variable>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{
		kind:         "variable",
		typ:          typeOf[T](),
		description:  name,
		variableName: name,
	}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		if ctx.Variables == nil {
			return Missing()
		}
		value, ok := ctx.Variables[name]
		if !ok {
			return Missing()
		}
		return value
	}}
}

// Parameter creates a named, type-carrying substitution parameter. Parameters
// are resolved only at execution time, which keeps a Plan immutable and lets a
// PreparedQuery be executed repeatedly with different values.
func Parameter[T any](name string) Expression[T] {
	name = strings.TrimSpace(name)
	if name == "" {
		return makeExpr[T]("parameter", "<invalid-parameter>", nil, func(EvalContext) Value { return Missing() })
	}
	if strings.HasPrefix(name, positionalParameterPrefix) {
		expression := makeExpr[T]("parameter", "<invalid-parameter>", nil, func(EvalContext) Value { return Missing() })
		expression.node().configurationError = "named parameter name uses a reserved positional-parameter prefix"
		return expression
	}
	node := &exprNode{
		kind:          "parameter",
		typ:           typeOf[T](),
		description:   "param(" + name + ":" + typeOf[T]().String() + ")",
		parameterName: name,
	}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		if ctx.Parameters != nil {
			if value, ok := ctx.Parameters[name]; ok {
				return value
			}
		}
		// Runtime evaluation historically carries only Variables. The reserved
		// entry is an internal bridge so old evaluator paths and nested
		// aggregate/window helpers retain parameter bindings without a second
		// mutable state channel.
		if ctx.Variables != nil {
			if bound, ok := ctx.Variables[parameterValuesVariable]; ok && bound.IsPresent() {
				if values, ok := bound.Any().(map[string]Value); ok {
					if value, exists := values[name]; exists {
						return value
					}
				}
			}
		}
		return Missing()
	}}
}

// Param is a concise alias for Parameter for fluent rules that prefer the
// shorter spelling.
func Param[T any](name string) Expression[T] { return Parameter[T](name) }

// ParameterAt creates a 1-based positional substitution parameter.  The
// explicit index mirrors Esper's PreparedQuery setObject(index, value)
// contract while keeping the rule itself typed and free of EPL text.  Use
// ExecuteFireAndForgetWithPositionalParameters or the corresponding
// PreparedQuery method to supply values in declaration order.
func ParameterAt[T any](position int) Expression[T] {
	if position < 1 {
		expression := makeExpr[T]("parameter", "<invalid-positional-parameter>", nil, func(EvalContext) Value { return Missing() })
		expression.node().configurationError = "positional parameter index must be at least 1"
		return expression
	}
	name := positionalParameterName(position)
	node := &exprNode{
		kind:          "parameter",
		typ:           typeOf[T](),
		description:   fmt.Sprintf("param(?%d:%s)", position, typeOf[T]()),
		parameterName: name,
	}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		if ctx.Parameters != nil {
			if value, ok := ctx.Parameters[name]; ok {
				return value
			}
		}
		if ctx.Variables != nil {
			if bound, ok := ctx.Variables[parameterValuesVariable]; ok && bound.IsPresent() {
				if values, ok := bound.Any().(map[string]Value); ok {
					if value, exists := values[name]; exists {
						return value
					}
				}
			}
		}
		return Missing()
	}}
}

// PositionalParameter is a descriptive alias for ParameterAt.  ParameterAt
// is the shorter spelling used by the rest of the fluent API.
func PositionalParameter[T any](position int) Expression[T] {
	return ParameterAt[T](position)
}

// ExpressionParam creates a parameter that is local to a named expression
// definition.  It is deliberately distinct from Parameter: Parameter binds a
// statement's prepared-query value, while ExpressionParam is substituted by
// ExpressionRef at each named-expression call site.
//
// Example:
//
//	left := ExpressionParam[string]("left")
//	right := ExpressionParam[string]("right")
//	_ = DefineExpression(env, "join", Concat(left, right))
//	call := ExpressionRef[string](env, "join", Field[Event, string]("a"), Literal("!"))
func ExpressionParam[T any](name string) Expression[T] {
	name = strings.TrimSpace(name)
	if name == "" {
		return makeExpr[T]("expression-parameter", "<invalid-expression-parameter>", nil, func(EvalContext) Value { return Missing() })
	}
	node := &exprNode{
		kind:          "expression-parameter",
		typ:           typeOf[T](),
		description:   "expr-param(" + name + ":" + typeOf[T]().String() + ")",
		parameterName: name,
	}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		if ctx.Parameters == nil {
			return Missing()
		}
		value, ok := ctx.Parameters[name]
		if !ok {
			return Missing()
		}
		return evaluateExpressionParameterBinding(value, ctx)
	}}
}

// DeclaredExpressionParam is a descriptive alias for ExpressionParam.  It
// is useful in larger rule modules where ordinary prepared-query parameters
// and named-expression parameters appear in the same scope.
func DeclaredExpressionParam[T any](name string) Expression[T] {
	return ExpressionParam[T](name)
}

// Literal creates a constant expression.
func Literal[T any](value T) Expression[T] {
	description := fmt.Sprintf("%v", value)
	reflected := reflect.ValueOf(value)
	primitive := reflected.IsValid() && (reflected.Kind() == reflect.Bool || reflected.Kind() == reflect.Int || reflected.Kind() == reflect.Int8 || reflected.Kind() == reflect.Int16 || reflected.Kind() == reflect.Int32 || reflected.Kind() == reflect.Int64 || reflected.Kind() == reflect.Uint || reflected.Kind() == reflect.Uint8 || reflected.Kind() == reflect.Uint16 || reflected.Kind() == reflect.Uint32 || reflected.Kind() == reflect.Uint64 || reflected.Kind() == reflect.Uintptr || reflected.Kind() == reflect.Float32 || reflected.Kind() == reflect.Float64 || reflected.Kind() == reflect.Complex64 || reflected.Kind() == reflect.Complex128 || reflected.Kind() == reflect.String)
	if typeOf[T]().Kind() == reflect.Interface && primitive {
		// Interface-typed literals retain their concrete value type in plan
		// identity so dynamic values such as int(1) and string("1") cannot
		// collapse to the same canonical expression.
		description = fmt.Sprintf("literal(%T:%v)", value, value)
	} else if !primitive {
		description = "literal(" + canonicalDataflowPropertyValue(reflected, make(map[canonicalDataflowReference]bool)) + ")"
	}
	node := &exprNode{kind: "literal", typ: typeOf[T](), description: description, literalValue: value}
	return typedExpr[T]{n: node, fn: func(EvalContext) Value { return Present(value) }}
}

// DurationSeconds converts an analyzable numeric expression to a duration.
// It is useful for timer guards whose duration is carried by an event tag or
// substitution parameter, for example:
//
//	pattern.WithinExpr(DurationSeconds(TagField[int64]("a", "seconds")))
//
// Non-positive, non-finite, and overflowing values evaluate to Null so the
// owning timer guard can terminate that branch deterministically.
func DurationSeconds[T Numeric](value Expression[T]) Expression[time.Duration] {
	return durationExpression[T]("duration-seconds", value, float64(time.Second))
}

// DurationDays converts an analyzable numeric expression to a duration in
// 24-hour days. Calendar months and years deliberately use DurationSum only
// for fixed-duration components; use WithinCalendar or TimerIntervalCalendar
// when month/year boundaries must be preserved.
func DurationDays[T Numeric](value Expression[T]) Expression[time.Duration] {
	return durationExpression[T]("duration-days", value, float64(24*time.Hour))
}

// DurationHours converts an analyzable numeric expression to a duration in
// hours.
func DurationHours[T Numeric](value Expression[T]) Expression[time.Duration] {
	return durationExpression[T]("duration-hours", value, float64(time.Hour))
}

// DurationMinutes converts an analyzable numeric expression to a duration in
// minutes.
func DurationMinutes[T Numeric](value Expression[T]) Expression[time.Duration] {
	return durationExpression[T]("duration-minutes", value, float64(time.Minute))
}

// DurationMilliseconds is the millisecond counterpart of DurationSeconds.
func DurationMilliseconds[T Numeric](value Expression[T]) Expression[time.Duration] {
	return durationExpression[T]("duration-milliseconds", value, float64(time.Millisecond))
}

// DurationMicroseconds converts an analyzable numeric expression to a
// duration in microseconds.
func DurationMicroseconds[T Numeric](value Expression[T]) Expression[time.Duration] {
	return durationExpression[T]("duration-microseconds", value, float64(time.Microsecond))
}

// DurationNanoseconds converts an analyzable numeric expression to a duration
// in nanoseconds.
func DurationNanoseconds[T Numeric](value Expression[T]) Expression[time.Duration] {
	return durationExpression[T]("duration-nanoseconds", value, 1)
}

// DurationSum combines component durations while retaining an analyzable
// expression tree. It is the Go-style counterpart of Esper's component-wise
// duration forms such as "D days H hours M minutes S seconds MS milliseconds".
func DurationSum(parts ...Expression[time.Duration]) Expression[time.Duration] {
	children := make([]*exprNode, 0, len(parts))
	for _, part := range parts {
		if part == nil {
			return makeExpr[time.Duration]("duration-sum", "<invalid-duration-sum>", nil, func(EvalContext) Value { return Null() })
		}
		children = append(children, part.node())
	}
	if len(parts) == 0 {
		return makeExpr[time.Duration]("duration-sum", "<invalid-duration-sum>", nil, func(EvalContext) Value { return Null() })
	}
	descriptions := make([]string, 0, len(parts))
	for _, part := range parts {
		descriptions = append(descriptions, part.Description())
	}
	return makeExpr[time.Duration]("duration-sum", "duration-sum("+strings.Join(descriptions, ",")+")", children, func(ctx EvalContext) Value {
		var total int64
		for _, part := range parts {
			value := part.eval(ctx)
			duration, ok := value.Any().(time.Duration)
			if !value.IsPresent() || !ok {
				return Null()
			}
			next := int64(duration)
			if next > 0 && total > int64(^uint64(0)>>1)-next {
				return Null()
			}
			if next < 0 && total < -int64(^uint64(0)>>1)-1-next {
				return Null()
			}
			total += next
		}
		return Present(time.Duration(total))
	})
}

func durationExpression[T Numeric](kind string, value Expression[T], multiplier float64) Expression[time.Duration] {
	if value == nil {
		return makeExpr[time.Duration](kind, "<invalid-duration>", nil, func(EvalContext) Value { return Null() })
	}
	return makeExpr[time.Duration](kind, kind+"("+value.Description()+")", []*exprNode{value.node()}, func(ctx EvalContext) Value {
		number, ok := numericValue(value.eval(ctx))
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number <= 0 {
			return Null()
		}
		nanos := number * multiplier
		if math.IsInf(nanos, 0) || nanos > float64((1<<63)-1) {
			return Null()
		}
		rounded := math.Round(nanos)
		if rounded <= 0 {
			return Null()
		}
		return Present(time.Duration(rounded))
	})
}

func NullLiteral[T any]() Expression[T] {
	return makeExpr[T]("null", "null", nil, func(EvalContext) Value { return Null() })
}

func Equal[T comparable](left, right Expression[T]) Expression[bool] {
	return makeBinaryBool("eq", "("+left.Description()+" = "+right.Description()+")", left, right, EqualValues)
}

func NotEqual[T comparable](left, right Expression[T]) Expression[bool] {
	return makeBinaryBool("neq", "("+left.Description()+" != "+right.Description()+")", left, right, func(l, r Value) Value {
		result := EqualValues(l, r)
		if !result.IsPresent() {
			return result
		}
		return Present(!result.Any().(bool))
	})
}

// SortedMultiKey is an immutable lexicographic key for multi-criteria sorted
// access aggregates. Each component keeps its Value state so a caller can
// distinguish a present component from Null/Missing when inspecting a key;
// sorted access itself only indexes rows whose complete key is present.
type SortedMultiKey struct {
	parts []Value
}

// NewSortedMultiKey constructs a detached multi-criteria key. Value arguments
// retain their state; all other arguments are wrapped with Present (nil is
// therefore represented as Null).
func NewSortedMultiKey(parts ...any) SortedMultiKey {
	result := SortedMultiKey{parts: make([]Value, len(parts))}
	for index, part := range parts {
		if value, ok := part.(Value); ok {
			result.parts[index] = value
			continue
		}
		result.parts[index] = Present(part)
	}
	return result
}

// Parts returns the underlying components as detached Go values. Null and
// Missing components are both exposed as nil; use PartValues when the state
// distinction is needed.
func (key SortedMultiKey) Parts() []any {
	parts := make([]any, len(key.parts))
	for index, value := range key.parts {
		parts[index] = value.Any()
	}
	return parts
}

// PartValues returns a defensive copy retaining Missing/Null/Present state.
func (key SortedMultiKey) PartValues() []Value { return append([]Value(nil), key.parts...) }

type Ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 | ~string | SortedMultiKey
}

type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// ExactNumeric is the arbitrary-precision numeric set supported by the
// exact aggregate family. big.Int corresponds to Java BigInteger and
// big.Rat is the Go representation used for exact decimal/rational values
// corresponding to Java BigDecimal without routing arithmetic through
// float64.
type ExactNumeric interface {
	big.Int | big.Rat
}

func Greater[T Ordered](left, right Expression[T]) Expression[bool] {
	return compareExpression[T]("gt", ">", left, right, func(c int) bool { return c > 0 })
}

func GreaterOrEqual[T Ordered](left, right Expression[T]) Expression[bool] {
	return compareExpression[T]("gte", ">=", left, right, func(c int) bool { return c >= 0 })
}

func Less[T Ordered](left, right Expression[T]) Expression[bool] {
	return compareExpression[T]("lt", "<", left, right, func(c int) bool { return c < 0 })
}

func LessOrEqual[T Ordered](left, right Expression[T]) Expression[bool] {
	return compareExpression[T]("lte", "<=", left, right, func(c int) bool { return c <= 0 })
}

// LessExact compares arbitrary-precision numeric expressions without a
// float64 conversion. It is the explicit Go counterpart for BigInteger and
// BigDecimal predicates used by exact aggregate filters.
func LessExact[T ExactNumeric](left, right Expression[T]) Expression[bool] {
	return exactCompareExpression[T]("exact-lt", "<", left, right, func(comparison int) bool { return comparison < 0 })
}

// LessOrEqualExact compares arbitrary-precision numeric expressions without
// losing precision.
func LessOrEqualExact[T ExactNumeric](left, right Expression[T]) Expression[bool] {
	return exactCompareExpression[T]("exact-lte", "<=", left, right, func(comparison int) bool { return comparison <= 0 })
}

// GreaterExact compares arbitrary-precision numeric expressions without a
// float64 conversion.
func GreaterExact[T ExactNumeric](left, right Expression[T]) Expression[bool] {
	return exactCompareExpression[T]("exact-gt", ">", left, right, func(comparison int) bool { return comparison > 0 })
}

// GreaterOrEqualExact compares arbitrary-precision numeric expressions
// without losing precision.
func GreaterOrEqualExact[T ExactNumeric](left, right Expression[T]) Expression[bool] {
	return exactCompareExpression[T]("exact-gte", ">=", left, right, func(comparison int) bool { return comparison >= 0 })
}

func exactCompareExpression[T ExactNumeric](kind, symbol string, left, right Expression[T], matches func(int) bool) Expression[bool] {
	if left == nil || right == nil {
		return makeExpr[bool](kind, "<invalid-exact-comparison>", nil, func(EvalContext) Value { return Null() })
	}
	description := "(" + left.Description() + " " + symbol + " " + right.Description() + ")"
	return makeExpr[bool](kind, description, []*exprNode{left.node(), right.node()}, func(ctx EvalContext) Value {
		comparison, ok := exactNumericCompare(left.eval(ctx), right.eval(ctx))
		if !ok {
			return Null()
		}
		return Present(matches(comparison))
	})
}

func exactNumericCompare(left, right Value) (int, bool) {
	leftNumber, leftOK := enumRatFromValue(left)
	rightNumber, rightOK := enumRatFromValue(right)
	if !leftOK || !rightOK {
		return 0, false
	}
	return leftNumber.Cmp(rightNumber), true
}

func Between[T Ordered](value, lower, upper Expression[T]) Expression[bool] {
	return BetweenOf(value, lower, upper)
}

func In[T comparable](value Expression[T], candidates ...Expression[T]) Expression[bool] {
	descriptionParts := make([]string, 0, len(candidates))
	children := []*exprNode{value.node()}
	for _, candidate := range candidates {
		descriptionParts = append(descriptionParts, candidate.Description())
		children = append(children, candidate.node())
	}
	description := value.Description() + " in (" + strings.Join(descriptionParts, ",") + ")"
	return makeExpr[bool]("in", description, children, func(ctx EvalContext) Value {
		current := value.eval(ctx)
		if !current.IsPresent() {
			return Null()
		}
		hasNull := false
		for _, candidate := range candidates {
			other := candidate.eval(ctx)
			if !other.IsPresent() {
				hasNull = true
				continue
			}
			if current.Equal(other) {
				return Present(true)
			}
		}
		if hasNull {
			return Null()
		}
		return Present(false)
	})
}

// InSlice checks whether a scalar expression is contained in a slice-valued
// expression. It is the typed Go equivalent of an Esper substitution
// parameter such as "value in (?::string[])" and keeps the slice immutable
// for the duration of one evaluation.
func InSlice[T comparable](value Expression[T], candidates Expression[[]T]) Expression[bool] {
	description := value.Description() + " in " + candidates.Description()
	return makeExpr[bool]("in-slice", description, []*exprNode{value.node(), candidates.node()}, func(ctx EvalContext) Value {
		current := value.eval(ctx)
		if !current.IsPresent() {
			return Null()
		}
		candidateValue := candidates.eval(ctx)
		if !candidateValue.IsPresent() {
			return Null()
		}
		values, err := As[[]T](candidateValue)
		if err != nil {
			return Null()
		}
		for _, candidate := range values {
			if current.Equal(Present(candidate)) {
				return Present(true)
			}
		}
		return Present(false)
	})
}

func Coalesce[T any](values ...Expression[T]) Expression[T] {
	operands := make([]Expr, len(values))
	for index, value := range values {
		if value != nil {
			operands[index] = value
		}
	}
	return coalesceExpressionOf[T](operands...)
}

// CoalesceOf is the explicit-result-type form of Coalesce. It accepts
// compatible expressions of different concrete Go types, including nullable
// numeric pointers, and converts the first present value to T. This models
// Esper's numeric coalesce promotion while keeping the target type visible in
// Go source.
func CoalesceOf[T any](values ...Expr) Expression[T] {
	return coalesceExpressionOf[T](values...)
}

func coalesceExpressionOf[T any](values ...Expr) Expression[T] {
	descriptionParts := make([]string, 0, len(values))
	children := make([]*exprNode, 0, len(values))
	for _, value := range values {
		if value == nil {
			descriptionParts = append(descriptionParts, "<nil>")
			children = append(children, nil)
			continue
		}
		descriptionParts = append(descriptionParts, value.Description())
		children = append(children, value.node())
	}
	node := &exprNode{
		kind:        "coalesce",
		typ:         typeOf[T](),
		description: "coalesce(" + strings.Join(descriptionParts, ",") + ")",
		children:    children,
	}
	if len(values) < 2 {
		node.configurationError = "coalesce requires at least two operands"
	} else {
		for _, value := range values {
			if value == nil || value.node() == nil {
				node.configurationError = "coalesce operands are required"
				break
			}
		}
	}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		for _, value := range values {
			if value == nil {
				continue
			}
			result := value.eval(ctx)
			if !result.IsPresent() {
				continue
			}
			if isNilReflectValue(reflect.ValueOf(result.Any())) {
				continue
			}
			return coalesceCastValue[T](result)
		}
		return Null()
	}}
}

func coalesceCastValue[T any](value Value) Value {
	if !value.IsPresent() {
		return value
	}
	target := typeOf[T]()
	source := reflect.ValueOf(value.Any())
	if isNilReflectValue(source) {
		return Null()
	}
	if !source.IsValid() {
		return Null()
	}
	if source.Type().AssignableTo(target) || source.Type().ConvertibleTo(target) || target == typeOf[any]() {
		return castValue[T](value)
	}
	for source.IsValid() && (source.Kind() == reflect.Pointer || source.Kind() == reflect.Interface) {
		if source.IsNil() {
			return Null()
		}
		source = source.Elem()
	}
	if !source.IsValid() {
		return Null()
	}
	return castValue[T](Present(source.Interface()))
}
func Like(value, pattern Expression[string]) Expression[bool] {
	return likeExpression("like", value, pattern, nil)
}

func RegexpMatch(value, pattern Expression[string]) Expression[bool] {
	return regexpExpression(value, pattern)
}

func Lower(value Expression[string]) Expression[string] {
	return Func1[string, string]("lower", strings.ToLower, value)
}

func Upper(value Expression[string]) Expression[string] {
	return Func1[string, string]("upper", strings.ToUpper, value)
}

func Trim(value Expression[string]) Expression[string] {
	return Func1[string, string]("trim", strings.TrimSpace, value)
}

func StringLength(value Expression[string]) Expression[int64] {
	return Func1[string, int64]("length", func(input string) int64 { return int64(len([]rune(input))) }, value)
}

// Split returns the ordered string elements produced by splitting value on
// separator. It is the Go-style, analyzable counterpart of a string split
// function used as a contained-event source expression; the resulting slice
// can be passed directly to UnnestValues.
func Split(value, separator Expression[string]) Expression[[]string] {
	return Func2[string, string, []string]("split", func(input, delimiter string) []string {
		return strings.Split(input, delimiter)
	}, value, separator)
}

func Contains(value, fragment Expression[string]) Expression[bool] {
	return makeBinaryBool("contains", "contains("+value.Description()+","+fragment.Description()+")", value, fragment, func(left, right Value) Value {
		l, lok := As[string](left)
		r, rok := As[string](right)
		if lok != nil || rok != nil {
			return Null()
		}
		return Present(strings.Contains(l, r))
	})
}

func StartsWith(value, prefix Expression[string]) Expression[bool] {
	return makeBinaryBool("starts-with", "starts-with("+value.Description()+","+prefix.Description()+")", value, prefix, func(left, right Value) Value {
		l, lok := As[string](left)
		p, pok := As[string](right)
		if lok != nil || pok != nil {
			return Null()
		}
		return Present(strings.HasPrefix(l, p))
	})
}

func EndsWith(value, suffix Expression[string]) Expression[bool] {
	return makeBinaryBool("ends-with", "ends-with("+value.Description()+","+suffix.Description()+")", value, suffix, func(left, right Value) Value {
		l, lok := As[string](left)
		s, sok := As[string](right)
		if lok != nil || sok != nil {
			return Null()
		}
		return Present(strings.HasSuffix(l, s))
	})
}

func patternToRegexp(pattern string) string {
	var builder strings.Builder
	for _, runeValue := range pattern {
		switch runeValue {
		case '%':
			builder.WriteString(".*")
		case '_':
			builder.WriteByte('.')
		default:
			builder.WriteString(regexp.QuoteMeta(string(runeValue)))
		}
	}
	return builder.String()
}

func likePattern(pattern string) string { return "^(?:" + pattern + ")$" }

func compareExpression[T any](kind, symbol string, left, right Expression[T], predicate func(int) bool) Expression[bool] {
	description := "(" + left.Description() + " " + symbol + " " + right.Description() + ")"
	return makeBinaryBool(kind, description, left, right, func(l, r Value) Value {
		comparison, ok := compareValues(l, r)
		if !ok {
			return Null()
		}
		return Present(predicate(comparison))
	})
}

func And(left, right Expression[bool]) Expression[bool] {
	return makeExpr[bool]("and", "("+left.Description()+" and "+right.Description()+")", []*exprNode{left.node(), right.node()}, func(ctx EvalContext) Value {
		leftValue := left.eval(ctx)
		leftBool, leftOK := boolValue(leftValue)
		if leftOK && !leftBool {
			return Present(false)
		}
		rightValue := right.eval(ctx)
		rightBool, rightOK := boolValue(rightValue)
		if rightOK && !rightBool {
			return Present(false)
		}
		if leftOK && rightOK {
			return Present(leftBool && rightBool)
		}
		return Null()
	})
}

func Or(left, right Expression[bool]) Expression[bool] {
	return makeExpr[bool]("or", "("+left.Description()+" or "+right.Description()+")", []*exprNode{left.node(), right.node()}, func(ctx EvalContext) Value {
		leftValue := left.eval(ctx)
		leftBool, leftOK := boolValue(leftValue)
		if leftOK && leftBool {
			return Present(true)
		}
		rightValue := right.eval(ctx)
		rightBool, rightOK := boolValue(rightValue)
		if rightOK && rightBool {
			return Present(true)
		}
		if leftOK && rightOK {
			return Present(leftBool || rightBool)
		}
		return Null()
	})
}

func Not(value Expression[bool]) Expression[bool] {
	return makeExpr[bool]("not", "(not "+value.Description()+")", []*exprNode{value.node()}, func(ctx EvalContext) Value {
		result := value.eval(ctx)
		b, ok := boolValue(result)
		if !ok {
			return Null()
		}
		return Present(!b)
	})
}

// BitwiseOperand is the set of Go values accepted by the binary bitwise
// operators. Boolean operands use Java/Esper's logical binary semantics:
// &, | and ^ map to AND, OR and XOR respectively. Integer operands preserve
// their declared Go width and signedness in the result.
type BitwiseOperand interface {
	~bool |
		~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

// BitwiseAnd applies a binary & operation to integer or boolean expressions.
// The typed Go API prevents floating-point, string and arbitrary object
// operands at compile time; a Null or Missing operand evaluates to Null.
func BitwiseAnd[T BitwiseOperand](left, right Expression[T]) Expression[T] {
	return bitwiseExpression[T]("bitwise-and", "&", left, right)
}

// BitwiseAndOf is the nullable/mixed-operand form of BitwiseAnd. The result
// type is explicit while each operand may be a compatible value, pointer or
// interface expression; a nil pointer/interface evaluates to Null. This
// models Java's primitive-and-boxed operand combinations without weakening
// the ordinary BitwiseAnd compile-time contract.
func BitwiseAndOf[T BitwiseOperand](left, right Expr) Expression[T] {
	return bitwiseExpressionOf[T]("bitwise-and", "&", left, right)
}

// BitwiseOr applies a binary | operation to integer or boolean expressions.
func BitwiseOr[T BitwiseOperand](left, right Expression[T]) Expression[T] {
	return bitwiseExpression[T]("bitwise-or", "|", left, right)
}

// BitwiseOrOf is the nullable/mixed-operand form of BitwiseOr.
func BitwiseOrOf[T BitwiseOperand](left, right Expr) Expression[T] {
	return bitwiseExpressionOf[T]("bitwise-or", "|", left, right)
}

// BitwiseXor applies a binary ^ operation to integer or boolean expressions.
func BitwiseXor[T BitwiseOperand](left, right Expression[T]) Expression[T] {
	return bitwiseExpression[T]("bitwise-xor", "^", left, right)
}

// BitwiseXorOf is the nullable/mixed-operand form of BitwiseXor.
func BitwiseXorOf[T BitwiseOperand](left, right Expr) Expression[T] {
	return bitwiseExpressionOf[T]("bitwise-xor", "^", left, right)
}

// BinaryAnd, BinaryOr and BinaryXor are descriptive aliases for callers that
// want the operator terminology used by Esper's expression model.
func BinaryAnd[T BitwiseOperand](left, right Expression[T]) Expression[T] {
	return BitwiseAnd[T](left, right)
}

func BinaryAndOf[T BitwiseOperand](left, right Expr) Expression[T] {
	return BitwiseAndOf[T](left, right)
}

func BinaryOr[T BitwiseOperand](left, right Expression[T]) Expression[T] {
	return BitwiseOr[T](left, right)
}

func BinaryOrOf[T BitwiseOperand](left, right Expr) Expression[T] {
	return BitwiseOrOf[T](left, right)
}

func BinaryXor[T BitwiseOperand](left, right Expression[T]) Expression[T] {
	return BitwiseXor[T](left, right)
}

func BinaryXorOf[T BitwiseOperand](left, right Expr) Expression[T] {
	return BitwiseXorOf[T](left, right)
}

func bitwiseExpression[T BitwiseOperand](kind, symbol string, left, right Expression[T]) Expression[T] {
	var leftExpr, rightExpr Expr
	if left != nil {
		leftExpr = left
	}
	if right != nil {
		rightExpr = right
	}
	return bitwiseExpressionOf[T](kind, symbol, leftExpr, rightExpr)
}

func bitwiseExpressionOf[T BitwiseOperand](kind, symbol string, left, right Expr) Expression[T] {
	if left == nil || right == nil {
		node := &exprNode{
			kind:               kind,
			typ:                typeOf[T](),
			description:        "<invalid-" + kind + ">",
			configurationError: "bitwise operator requires two operands",
		}
		return typedExpr[T]{n: node, fn: func(EvalContext) Value { return Null() }}
	}
	description := "(" + left.Description() + " " + symbol + " " + right.Description() + ")"
	return makeExpr[T](kind, description, []*exprNode{left.node(), right.node()}, func(ctx EvalContext) Value {
		return applyBitwise[T](left.eval(ctx), right.eval(ctx), symbol)
	})
}

func applyBitwise[T BitwiseOperand](left, right Value, symbol string) Value {
	var zero T
	target := reflect.TypeOf(zero)
	if target == nil {
		return Null()
	}
	leftValue, leftOK := bitwiseOperandValue(left, target)
	rightValue, rightOK := bitwiseOperandValue(right, target)
	if !leftOK || !rightOK {
		return Null()
	}
	result := reflect.New(target).Elem()
	switch target.Kind() {
	case reflect.Bool:
		leftBool, rightBool := leftValue.Bool(), rightValue.Bool()
		switch symbol {
		case "&":
			result.SetBool(leftBool && rightBool)
		case "|":
			result.SetBool(leftBool || rightBool)
		case "^":
			result.SetBool(leftBool != rightBool)
		default:
			return Null()
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		leftInt, rightInt := leftValue.Int(), rightValue.Int()
		var value int64
		switch symbol {
		case "&":
			value = leftInt & rightInt
		case "|":
			value = leftInt | rightInt
		case "^":
			value = leftInt ^ rightInt
		default:
			return Null()
		}
		result.SetInt(value)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		leftUint, rightUint := leftValue.Uint(), rightValue.Uint()
		var value uint64
		switch symbol {
		case "&":
			value = leftUint & rightUint
		case "|":
			value = leftUint | rightUint
		case "^":
			value = leftUint ^ rightUint
		default:
			return Null()
		}
		result.SetUint(value)
	default:
		return Null()
	}
	return Present(result.Interface())
}

func bitwiseOperandValue(value Value, target reflect.Type) (reflect.Value, bool) {
	if !value.IsPresent() {
		return reflect.Value{}, false
	}
	operand := reflect.ValueOf(value.Any())
	for operand.IsValid() && (operand.Kind() == reflect.Interface || operand.Kind() == reflect.Pointer) {
		if operand.IsNil() {
			return reflect.Value{}, false
		}
		operand = operand.Elem()
	}
	if !operand.IsValid() {
		return reflect.Value{}, false
	}
	if operand.Type() == target {
		return operand, true
	}
	if operand.Type().ConvertibleTo(target) {
		return operand.Convert(target), true
	}
	return reflect.Value{}, false
}

func IsNull[T any](value Expression[T]) Expression[bool] {
	return makeExpr[bool]("is-null", "("+value.Description()+" is null)", []*exprNode{value.node()}, func(ctx EvalContext) Value {
		return Present(value.eval(ctx).IsNull())
	})
}

func IsMissing[T any](value Expression[T]) Expression[bool] {
	return makeExpr[bool]("is-missing", "("+value.Description()+" is missing)", []*exprNode{value.node()}, func(ctx EvalContext) Value {
		return Present(value.eval(ctx).IsMissing())
	})
}

func Add[T Numeric](left, right Expression[T]) Expression[T] {
	return arithmeticExpression[T]("add", "+", left, right, func(l, r float64) float64 { return l + r })
}

func Subtract[T Numeric](left, right Expression[T]) Expression[T] {
	return arithmeticExpression[T]("subtract", "-", left, right, func(l, r float64) float64 { return l - r })
}

func Multiply[T Numeric](left, right Expression[T]) Expression[T] {
	return arithmeticExpression[T]("multiply", "*", left, right, func(l, r float64) float64 { return l * r })
}

func Divide[T Numeric](left, right Expression[T]) Expression[T] {
	return arithmeticExpression[T]("divide", "/", left, right, func(l, r float64) float64 {
		if r == 0 {
			return 0
		}
		return l / r
	})
}

func Modulo[T Numeric](left, right Expression[T]) Expression[T] {
	return arithmeticExpression[T]("modulo", "%", left, right, func(l, r float64) float64 {
		if r == 0 {
			return 0
		}
		return math.Mod(l, r)
	})
}

func Negate[T Numeric](value Expression[T]) Expression[T] {
	return makeExpr[T]("negate", "(-"+value.Description()+")", []*exprNode{value.node()}, func(ctx EvalContext) Value {
		number, ok := numericValue(value.eval(ctx))
		if !ok {
			return Null()
		}
		return Present(convertNumeric[T](-number))
	})
}

// Abs returns the absolute value while preserving the expression's numeric
// type. It is useful in fluent rules that mirror Java's Math.abs without
// introducing an untyped callback into the plan.
func Abs[T Numeric](value Expression[T]) Expression[T] {
	if value == nil {
		return makeExpr[T]("abs", "abs(<nil>)", nil, func(EvalContext) Value { return Null() })
	}
	return makeExpr[T]("abs", "abs("+value.Description()+")", []*exprNode{value.node()}, func(ctx EvalContext) Value {
		number, ok := numericValue(value.eval(ctx))
		if !ok {
			return Null()
		}
		if number < 0 {
			number = -number
		}
		return Present(convertNumeric[T](number))
	})
}

// Signum is the Go-native equivalent of Java Math.signum for double-valued
// expressions. NaN and negative zero are preserved, while finite values are
// normalized to -1, 0, or 1.
func Signum(value Expression[float64]) Expression[float64] {
	if value == nil {
		return makeExpr[float64]("signum", "signum(<nil>)", nil, func(EvalContext) Value { return Null() })
	}
	return makeExpr[float64]("signum", "signum("+value.Description()+")", []*exprNode{value.node()}, func(ctx EvalContext) Value {
		current := value.eval(ctx)
		if !current.IsPresent() {
			return Null()
		}
		input, ok := current.Any().(float64)
		if !ok {
			return Null()
		}
		if math.IsNaN(input) || input == 0 {
			return Present(input)
		}
		if input < 0 {
			return Present(float64(-1))
		}
		return Present(float64(1))
	})
}

func Concat(values ...Expression[string]) Expression[string] {
	operands := make([]Expr, len(values))
	for index, value := range values {
		operands[index] = value
	}
	return concatExpression("concat", operands...)
}

func IfThenElse[T any](condition Expression[bool], whenTrue, whenFalse Expression[T]) Expression[T] {
	return makeExpr[T]("if", "if("+condition.Description()+","+whenTrue.Description()+","+whenFalse.Description()+")", []*exprNode{condition.node(), whenTrue.node(), whenFalse.node()}, func(ctx EvalContext) Value {
		matched, ok := boolValue(condition.eval(ctx))
		if !ok {
			return Null()
		}
		if matched {
			return whenTrue.eval(ctx)
		}
		return whenFalse.eval(ctx)
	})
}

// Cast converts a present expression value to the requested Go type while
// preserving Missing and Null. The explicit type parameters make conversion
// visible in the fluent plan; runtime conversion covers native numerics,
// booleans, arbitrary-precision numbers, recursive arrays and interfaces.
func Cast[A, B any](value Expression[A]) Expression[B] {
	return castExpression[A, B](value, "", nil)
}

// Exists reports whether the expression resolves to an existing property.
// An explicit Null is therefore true while Missing is false.
func Exists(value Expr) Expression[bool] {
	if value == nil {
		return makeExpr[bool]("exists", "exists(<nil>)", nil, func(EvalContext) Value { return Present(false) })
	}
	return makeExpr[bool]("exists", "exists("+value.Description()+")", []*exprNode{value.node()}, func(ctx EvalContext) Value {
		return Present(!value.eval(ctx).IsMissing())
	})
}

// TypeName returns the runtime Go type name for a present value.
func TypeName(value Expr) Expression[string] {
	if value == nil {
		return makeExpr[string]("type-of", "type-of(<nil>)", nil, func(EvalContext) Value { return Null() })
	}
	return makeExpr[string]("type-of", "type-of("+value.Description()+")", []*exprNode{value.node()}, func(ctx EvalContext) Value {
		current := value.eval(ctx)
		if !current.IsPresent() {
			return Null()
		}
		if event, ok := current.Any().(Event); ok {
			return Present(event.TypeName())
		}
		return Present(reflect.TypeOf(current.Any()).String())
	})
}

// InstanceOf reports whether a present value is assignable to T.
func InstanceOf[T any](value Expr) Expression[bool] {
	if value == nil {
		return makeExpr[bool]("instance-of", "instance-of(<nil>)", nil, func(EvalContext) Value { return Present(false) })
	}
	return makeExpr[bool]("instance-of", fmt.Sprintf("instance-of<%s>(%s)", typeOf[T](), value.Description()), []*exprNode{value.node()}, func(ctx EvalContext) Value {
		current := value.eval(ctx)
		if !current.IsPresent() {
			return Present(false)
		}
		candidate := reflect.TypeOf(current.Any())
		target := typeOf[T]()
		matches := candidate.AssignableTo(target)
		if !matches && target.Kind() == reflect.Interface {
			matches = candidate.Implements(target)
		}
		return Present(matches)
	})
}

// ArrayAt safely indexes an array or slice expression. The operands are Expr
// rather than one fixed slice/index instantiation so a fluent rule can use an
// int field, a fixed-size Go array, or a nested dynamic value as Esper's
// indexed-property operation does. Missing, Null, invalid-index and
// out-of-range access return Null rather than panicking.
func ArrayAt[T any](values, index Expr) Expression[T] {
	return arrayElementAt[T](values, index)
}

// MapAt safely reads one key from a map expression. Missing, Null, a
// non-string key and an absent key return Null rather than panicking. The
// expression remains analyzable through its child expression, so callers can
// use it in Join conditions and projections without encoding a closure.
func MapAt[T any](values Expr, key Expression[string]) Expression[T] {
	if values == nil || key == nil {
		return makeExpr[T]("map-at", "map-at(<invalid>)", nil, func(EvalContext) Value { return Null() })
	}
	return makeExpr[T]("map-at", "map-at("+values.Description()+","+key.Description()+")", []*exprNode{values.node(), key.node()}, func(ctx EvalContext) Value {
		base := values.eval(ctx)
		keyValue := key.eval(ctx)
		if !base.IsPresent() || !keyValue.IsPresent() {
			return Null()
		}
		mapKey, ok := keyValue.Any().(string)
		if !ok || mapKey == "" {
			return Null()
		}
		return castValue[T](propertyValue(base.Any(), mapKey))
	})
}

// MapValue is a descriptive alias for MapAt when the expression reads like a
// named map property rather than an array index.
func MapValue[T any](values Expr, key Expression[string]) Expression[T] {
	return MapAt[T](values, key)
}

func castValue[T any](value Value) Value {
	return castValueWithLayout[T](value, "")
}

func CurrentTime() Expression[time.Time] {
	return makeExpr[time.Time]("current-time", "current-time()", nil, func(ctx EvalContext) Value {
		return Present(ctx.Now)
	})
}

func OutputCountInsert() Expression[int64] {
	return makeExpr[int64]("output-count-insert", "count_insert", nil, func(ctx EvalContext) Value {
		return Present(ctx.OutputInsertCount)
	})
}

func OutputCountRemove() Expression[int64] {
	return makeExpr[int64]("output-count-remove", "count_remove", nil, func(ctx EvalContext) Value {
		return Present(ctx.OutputRemoveCount)
	})
}

func OutputCountInsertTotal() Expression[int64] {
	return makeExpr[int64]("output-count-insert-total", "count_insert_total", nil, func(ctx EvalContext) Value {
		return Present(ctx.OutputInsertTotal)
	})
}

func OutputCountRemoveTotal() Expression[int64] {
	return makeExpr[int64]("output-count-remove-total", "count_remove_total", nil, func(ctx EvalContext) Value {
		return Present(ctx.OutputRemoveTotal)
	})
}

func OutputLastOutputTime() Expression[time.Time] {
	return makeExpr[time.Time]("output-last-time", "last_output_timestamp", nil, func(ctx EvalContext) Value {
		if ctx.OutputLastOutputTime.IsZero() {
			return Null()
		}
		return Present(ctx.OutputLastOutputTime)
	})
}

func Year(value Expression[time.Time]) Expression[int64] {
	return Func1[time.Time, int64]("year", func(input time.Time) int64 { return int64(input.Year()) }, value)
}

func Month(value Expression[time.Time]) Expression[int64] {
	return Func1[time.Time, int64]("month", func(input time.Time) int64 { return int64(input.Month()) }, value)
}

func DayOfMonth(value Expression[time.Time]) Expression[int64] {
	return Func1[time.Time, int64]("day-of-month", func(input time.Time) int64 { return int64(input.Day()) }, value)
}

func UnixMillis(value Expression[time.Time]) Expression[int64] {
	return Func1[time.Time, int64]("unix-millis", func(input time.Time) int64 { return input.UnixNano() / int64(time.Millisecond) }, value)
}

func arithmeticExpression[T Numeric](kind, symbol string, left, right Expression[T], operation func(float64, float64) float64) Expression[T] {
	description := "(" + left.Description() + " " + symbol + " " + right.Description() + ")"
	return makeExpr[T](kind, description, []*exprNode{left.node(), right.node()}, func(ctx EvalContext) Value {
		lv, lok := numericValue(left.eval(ctx))
		rv, rok := numericValue(right.eval(ctx))
		if !lok || !rok {
			return Null()
		}
		result := operation(lv, rv)
		if kind == "divide" && rv == 0 {
			return Null()
		}
		return Present(convertNumeric[T](result))
	})
}

func convertNumeric[T Numeric](value float64) T {
	var zero T
	typ := reflect.TypeOf(zero)
	converted := reflect.ValueOf(value).Convert(typ)
	return converted.Interface().(T)
}

func makeBinaryBool(kind, description string, left, right Expr, operation func(Value, Value) Value) Expression[bool] {
	return makeExpr[bool](kind, description, []*exprNode{left.node(), right.node()}, func(ctx EvalContext) Value {
		return operation(left.eval(ctx), right.eval(ctx))
	})
}

// Func0 registers a named zero-argument function. The function is intentionally
// explicit rather than an opaque callback hidden inside a rule, so the Plan
// retains a stable UDF node while the returned collection remains chainable.
func Func0[T any](name string, function func() T) Expression[T] {
	description := strings.TrimSpace(name) + "()"
	configurationError := ""
	if strings.TrimSpace(name) == "" {
		configurationError = "udf function name is required"
	} else if function == nil {
		configurationError = fmt.Sprintf("udf %q requires a function", name)
	}
	return makeUDFExpr[T](description, nil, configurationError, func(EvalContext) (result Value) {
		defer recoverUDF(&result)
		if function == nil {
			return Null()
		}
		return Present(function())
	})
}

// Func1 registers a named unary function. The function is intentionally an
// explicit top-level UDF instead of an opaque field callback, so the Plan can
// retain its stable name and dependency metadata.
func Func1[A, B any](name string, function func(A) B, argument Expression[A]) Expression[B] {
	return makeUDFExpr[B](udfDescription(name, argument), udfChildren(argument), udfConfiguration(name, function == nil, argument == nil), func(ctx EvalContext) (result Value) {
		defer recoverUDF(&result)
		if function == nil || argument == nil {
			return Null()
		}
		input := argument.eval(ctx)
		if !input.IsPresent() {
			return Null()
		}
		value, err := As[A](input)
		if err != nil {
			return Null()
		}
		return Present(function(value))
	})
}

// Func2 registers a named binary function while retaining both argument
// expressions in the analyzable plan. It is useful for collection-producing
// UDFs whose range, limit, or lookup key is supplied by a rule expression.
func Func2[A, B, C any](name string, function func(A, B) C, first Expression[A], second Expression[B]) Expression[C] {
	children := udfChildren(first, second)
	return makeUDFExpr[C](udfDescription(name, first, second), children, udfConfiguration(name, function == nil, first == nil, second == nil), func(ctx EvalContext) (result Value) {
		defer recoverUDF(&result)
		if function == nil || first == nil || second == nil {
			return Null()
		}
		firstValue := first.eval(ctx)
		secondValue := second.eval(ctx)
		if !firstValue.IsPresent() || !secondValue.IsPresent() {
			return Null()
		}
		firstArgument, err := As[A](firstValue)
		if err != nil {
			return Null()
		}
		secondArgument, err := As[B](secondValue)
		if err != nil {
			return Null()
		}
		return Present(function(firstArgument, secondArgument))
	})
}

// Func3 registers a named ternary function. It fills the same explicit UDF
// role as Func0/Func1/Func2 while covering the common Java static-method
// footprint used by range, lookup and collection-producing helpers.
func Func3[A, B, C, D any](name string, function func(A, B, C) D, first Expression[A], second Expression[B], third Expression[C]) Expression[D] {
	children := udfChildren(first, second, third)
	return makeUDFExpr[D](udfDescription(name, first, second, third), children, udfConfiguration(name, function == nil, first == nil, second == nil, third == nil), func(ctx EvalContext) (result Value) {
		defer recoverUDF(&result)
		if function == nil || first == nil || second == nil || third == nil {
			return Null()
		}
		firstValue, secondValue, thirdValue := first.eval(ctx), second.eval(ctx), third.eval(ctx)
		if !firstValue.IsPresent() || !secondValue.IsPresent() || !thirdValue.IsPresent() {
			return Null()
		}
		firstArgument, err := As[A](firstValue)
		if err != nil {
			return Null()
		}
		secondArgument, err := As[B](secondValue)
		if err != nil {
			return Null()
		}
		thirdArgument, err := As[C](thirdValue)
		if err != nil {
			return Null()
		}
		return Present(function(firstArgument, secondArgument, thirdArgument))
	})
}

// Func4 registers a named four-argument function and retains every argument
// in the analyzable expression tree.
func Func4[A, B, C, D, E any](name string, function func(A, B, C, D) E, first Expression[A], second Expression[B], third Expression[C], fourth Expression[D]) Expression[E] {
	children := udfChildren(first, second, third, fourth)
	return makeUDFExpr[E](udfDescription(name, first, second, third, fourth), children, udfConfiguration(name, function == nil, first == nil, second == nil, third == nil, fourth == nil), func(ctx EvalContext) (result Value) {
		defer recoverUDF(&result)
		if function == nil || first == nil || second == nil || third == nil || fourth == nil {
			return Null()
		}
		firstValue, secondValue, thirdValue, fourthValue := first.eval(ctx), second.eval(ctx), third.eval(ctx), fourth.eval(ctx)
		if !firstValue.IsPresent() || !secondValue.IsPresent() || !thirdValue.IsPresent() || !fourthValue.IsPresent() {
			return Null()
		}
		firstArgument, err := As[A](firstValue)
		if err != nil {
			return Null()
		}
		secondArgument, err := As[B](secondValue)
		if err != nil {
			return Null()
		}
		thirdArgument, err := As[C](thirdValue)
		if err != nil {
			return Null()
		}
		fourthArgument, err := As[D](fourthValue)
		if err != nil {
			return Null()
		}
		return Present(function(firstArgument, secondArgument, thirdArgument, fourthArgument))
	})
}

func makeUDFExpr[T any](description string, children []*exprNode, configurationError string, fn func(EvalContext) Value) Expression[T] {
	expression := makeExpr[T]("udf", description, children, fn)
	expression.node().configurationError = configurationError
	return expression
}

func udfChildren(arguments ...Expr) []*exprNode {
	children := make([]*exprNode, len(arguments))
	for index, argument := range arguments {
		if argument != nil {
			children[index] = argument.node()
		}
	}
	return children
}

func udfDescription(name string, arguments ...Expr) string {
	parts := make([]string, len(arguments))
	for index, argument := range arguments {
		if argument == nil {
			parts[index] = "<nil>"
		} else {
			parts[index] = argument.Description()
		}
	}
	return strings.TrimSpace(name) + "(" + strings.Join(parts, ",") + ")"
}

func udfConfiguration(name string, functionMissing bool, argumentsMissing ...bool) string {
	if strings.TrimSpace(name) == "" {
		return "udf function name is required"
	}
	if functionMissing {
		return fmt.Sprintf("udf %q requires a function", name)
	}
	for index, missing := range argumentsMissing {
		if missing {
			return fmt.Sprintf("udf %q argument %d is required", name, index)
		}
	}
	return ""
}

func recoverUDF(result *Value) {
	if recover() != nil {
		*result = Null()
	}
}

// Func1Ctx registers a named unary function that also receives the current
// EvalContext, mirroring Java's EPLMethodInvocationContext parameter. The UDF
// can read the current Event, Group, Engine, statement user object and other
// evaluation-scoped state.
func Func1Ctx[A, B any](name string, function func(A, EvalContext) B, argument Expression[A]) Expression[B] {
	children := udfChildren(argument)
	return makeUDFExpr[B](udfDescription(name, argument), children, udfConfiguration(name, function == nil, argument == nil), func(ctx EvalContext) (result Value) {
		defer recoverUDF(&result)
		if function == nil || argument == nil {
			return Null()
		}
		input := argument.eval(ctx)
		if !input.IsPresent() {
			return Null()
		}
		value, err := As[A](input)
		if err != nil {
			return Null()
		}
		return Present(function(value, ctx))
	})
}

// Func2Ctx registers a named binary function that also receives the current
// EvalContext, mirroring Java's EPLMethodInvocationContext parameter.
func Func2Ctx[A, B, C any](name string, function func(A, B, EvalContext) C, first Expression[A], second Expression[B]) Expression[C] {
	children := udfChildren(first, second)
	return makeUDFExpr[C](udfDescription(name, first, second), children, udfConfiguration(name, function == nil, first == nil, second == nil), func(ctx EvalContext) (result Value) {
		defer recoverUDF(&result)
		if function == nil || first == nil || second == nil {
			return Null()
		}
		firstValue := first.eval(ctx)
		secondValue := second.eval(ctx)
		if !firstValue.IsPresent() || !secondValue.IsPresent() {
			return Null()
		}
		firstArgument, err := As[A](firstValue)
		if err != nil {
			return Null()
		}
		secondArgument, err := As[B](secondValue)
		if err != nil {
			return Null()
		}
		return Present(function(firstArgument, secondArgument, ctx))
	})
}

// Func1Rethrow registers a named unary function whose panics are propagated
// to the caller instead of being caught and turned into Null, mirroring
// Java's @RethrowExceptions annotation on single-row functions.
func Func1Rethrow[A, B any](name string, function func(A) B, argument Expression[A]) Expression[B] {
	children := udfChildren(argument)
	return makeUDFExpr[B](udfDescription(name, argument), children, udfConfiguration(name, function == nil, argument == nil), func(ctx EvalContext) (result Value) {
		if function == nil || argument == nil {
			return Null()
		}
		input := argument.eval(ctx)
		if !input.IsPresent() {
			return Null()
		}
		value, err := As[A](input)
		if err != nil {
			return Null()
		}
		return Present(function(value))
	})
}

// AggregateExpression is evaluated over EvalContext.Group by an aggregate
// stream. It remains an Expr so field and variable dependencies are visible
// to Build validation.
type AggregateExpression[T any] interface {
	Expression[T]
	aggregateMarker()
}

// Grouping reports whether a dimension is rolled up for the current result.
// It returns 1 for an omitted dimension and 0 for a present dimension; for a
// regular GroupBy result all dimensions are present.
func Grouping(expression Expr) Expression[int64] {
	if expression == nil {
		return makeExpr[int64]("grouping", "grouping(<nil>)", nil, func(EvalContext) Value { return Missing() })
	}
	description := "grouping(" + expression.Description() + ")"
	return makeExpr[int64]("grouping", description, []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		if ctx.groupingPresent == nil {
			return Present(int64(0))
		}
		present, ok := ctx.groupingPresent[groupingExpressionKey(expression)]
		if !ok || present {
			return Present(int64(0))
		}
		return Present(int64(1))
	})
}

// GroupingID returns the bit-packed rollup state for the supplied dimensions.
// The first expression is the most significant bit, matching the ordering
// used by Esper's grouping_id expression.
func GroupingID(expressions ...Expr) Expression[int64] {
	children := make([]*exprNode, 0, len(expressions))
	parts := make([]string, 0, len(expressions))
	for _, expression := range expressions {
		if expression == nil {
			parts = append(parts, "<nil>")
			continue
		}
		children = append(children, expression.node())
		parts = append(parts, expression.Description())
	}
	description := "grouping-id(" + strings.Join(parts, ",") + ")"
	return makeExpr[int64]("grouping-id", description, children, func(ctx EvalContext) Value {
		var result int64
		for _, expression := range expressions {
			result <<= 1
			if expression == nil || ctx.groupingPresent == nil {
				continue
			}
			if present, ok := ctx.groupingPresent[groupingExpressionKey(expression)]; ok && !present {
				result |= 1
			}
		}
		return Present(result)
	})
}

// FilterAggregate evaluates any aggregate over only the group rows for which
// predicate is true. Null and non-boolean predicate results are excluded,
// matching Esper's filtered-aggregate behavior while keeping the aggregate
// itself reusable and analyzable.
func FilterAggregate[T any](aggregate AggregateExpression[T], predicate Expression[bool]) AggregateExpression[T] {
	if aggregate == nil || predicate == nil {
		return makeAggregateExpr[T]("aggregate-filter", "aggregate-filter(<invalid>)", nil, func(EvalContext) Value { return Missing() })
	}
	description := "filter(" + aggregate.Description() + "," + predicate.Description() + ")"
	return makeAggregateExpr[T]("aggregate-filter", description, []*exprNode{aggregate.node(), predicate.node()}, func(ctx EvalContext) Value {
		filterEvents := func(events []Event) []Event {
			filtered := make([]Event, 0, len(events))
			for _, event := range events {
				predicateContext := EvalContext{Event: event, Now: ctx.Now, Variables: ctx.Variables, Parameters: ctx.Parameters}
				value := predicate.eval(predicateContext)
				ok, isBool := boolValue(value)
				if isBool && ok {
					filtered = append(filtered, event)
				}
			}
			return filtered
		}
		nested := ctx
		nested.Group = filterEvents(ctx.Group)
		nested.EverGroup = filterEvents(ctx.EverGroup)
		nested.AllGroup = filterEvents(ctx.AllGroup)
		nested.AllEverGroup = filterEvents(ctx.AllEverGroup)
		nested.aggregateMultiScope = aggregateExpressionScope(ctx.aggregateMultiScope, predicate.Description())
		return aggregate.eval(nested)
	})
}

// DistinctAggregate evaluates an aggregate over the first occurrence of each
// input value in the current group. A nil input uses event identity, which is
// the Go fluent equivalent of Esper's stream-wildcard/star parameter. Null and
// Missing are retained as distinct logical values and only collapse with the
// same state on another row.
func DistinctAggregate[T any](aggregate AggregateExpression[T], input Expr) AggregateExpression[T] {
	if aggregate == nil {
		return makeAggregateExpr[T]("aggregate-distinct", "aggregate-distinct(<invalid>)", nil, func(EvalContext) Value { return Missing() })
	}
	children := []*exprNode{aggregate.node()}
	description := "distinct(" + aggregate.Description()
	if input != nil {
		children = append(children, input.node())
		description += "," + input.Description()
	} else {
		description += ",*)"
		return makeAggregateExpr[T]("aggregate-distinct", description, children, func(ctx EvalContext) Value {
			nested := ctx
			nested.Group = distinctAggregateEvents(ctx.Group, nil, ctx)
			nested.EverGroup = distinctAggregateEvents(ctx.EverGroup, nil, ctx)
			nested.AllGroup = distinctAggregateEvents(ctx.AllGroup, nil, ctx)
			nested.AllEverGroup = distinctAggregateEvents(ctx.AllEverGroup, nil, ctx)
			nested.aggregateMultiScope = aggregateExpressionScope(ctx.aggregateMultiScope, description)
			return aggregate.eval(nested)
		})
	}
	description += ")"
	return makeAggregateExpr[T]("aggregate-distinct", description, children, func(ctx EvalContext) Value {
		nested := ctx
		nested.Group = distinctAggregateEvents(ctx.Group, input, ctx)
		nested.EverGroup = distinctAggregateEvents(ctx.EverGroup, input, ctx)
		nested.AllGroup = distinctAggregateEvents(ctx.AllGroup, input, ctx)
		nested.AllEverGroup = distinctAggregateEvents(ctx.AllEverGroup, input, ctx)
		nested.aggregateMultiScope = aggregateExpressionScope(ctx.aggregateMultiScope, description)
		return aggregate.eval(nested)
	})
}

func distinctAggregateEvents(events []Event, input Expr, ctx EvalContext) []Event {
	if len(events) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(events))
	result := make([]Event, 0, len(events))
	for _, event := range events {
		key := eventIdentity(event)
		if input != nil {
			value := input.eval(EvalContext{Event: event, Now: ctx.Now, Variables: ctx.Variables, Parameters: ctx.Parameters})
			key = encodeKey([]any{value})
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, event)
	}
	return result
}

// LocalGroupBy evaluates an aggregate against the subset of the outer
// aggregate group matching the current event's local key values. It is the
// analyzable Go equivalent of Esper's aggregate(..., group_by: (...))
// parameter and can be combined with ordinary outer GroupBy.
func LocalGroupBy[T any](aggregate AggregateExpression[T], keys ...Expr) AggregateExpression[T] {
	if aggregate == nil {
		return invalidAggregate[T]("aggregate-local-group", nil, "local-group(<invalid>)")
	}
	children := make([]*exprNode, 0, 1+len(keys))
	children = append(children, aggregate.node())
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == nil {
			children = append(children, nil)
			parts = append(parts, "<nil>")
			continue
		}
		children = append(children, key.node())
		parts = append(parts, key.Description())
	}
	description := aggregate.Description() + ",group_by:(" + strings.Join(parts, ",") + ")"
	return makeAggregateExpr[T]("aggregate-local-group", description, children, func(ctx EvalContext) Value {
		if len(keys) == 0 {
			nested := ctx
			if ctx.AllGroup != nil {
				nested.Group = ctx.AllGroup
			}
			if ctx.AllEverGroup != nil {
				nested.EverGroup = ctx.AllEverGroup
			}
			nested.AllGroup = nested.Group
			nested.AllEverGroup = nested.EverGroup
			nested.aggregateMultiScope = aggregateExpressionScope(ctx.aggregateMultiScope, description)
			return aggregate.eval(nested)
		}
		current := ctx.Event
		if current.Schema().Name() == "" {
			if len(ctx.Group) > 0 {
				current = ctx.Group[len(ctx.Group)-1]
			} else if len(ctx.EverGroup) > 0 {
				current = ctx.EverGroup[len(ctx.EverGroup)-1]
			}
		}
		target := evaluateLocalGroupKeys(keys, current, ctx)
		scope := ctx.AllGroup
		if scope == nil {
			scope = ctx.Group
		}
		everScope := ctx.AllEverGroup
		if everScope == nil {
			everScope = ctx.EverGroup
		}
		filter := func(events []Event) []Event {
			filtered := make([]Event, 0, len(events))
			for _, event := range events {
				if localGroupKeysEqual(keys, target, event, ctx) {
					filtered = append(filtered, event)
				}
			}
			return filtered
		}
		nested := ctx
		nested.Group = filter(scope)
		nested.EverGroup = filter(everScope)
		// A nested filtered/local aggregate must see the subset selected by
		// this local group as its own statement scope.
		nested.AllGroup = nested.Group
		nested.AllEverGroup = nested.EverGroup
		nested.aggregateMultiScope = aggregateExpressionScope(ctx.aggregateMultiScope, description)
		return aggregate.eval(nested)
	})
}

func evaluateLocalGroupKeys(keys []Expr, event Event, ctx EvalContext) []Value {
	values := make([]Value, 0, len(keys))
	for _, key := range keys {
		if key == nil {
			values = append(values, Missing())
			continue
		}
		values = append(values, key.eval(localGroupContext(ctx, event)))
	}
	return values
}

func localGroupKeysEqual(keys []Expr, target []Value, event Event, ctx EvalContext) bool {
	if len(keys) != len(target) {
		return false
	}
	current := localGroupContext(ctx, event)
	for index, key := range keys {
		if key == nil {
			return false
		}
		value := key.eval(current)
		if !localGroupValueEqual(value, target[index]) {
			return false
		}
	}
	return true
}

func localGroupContext(ctx EvalContext, event Event) EvalContext {
	return EvalContext{Event: event, Now: ctx.Now, Variables: ctx.Variables, Parameters: ctx.Parameters}
}

func localGroupValueEqual(left, right Value) bool {
	if !left.IsPresent() || !right.IsPresent() {
		return left.State() == right.State()
	}
	if comparison, ok := compareValues(left, right); ok {
		return comparison == 0
	}
	return left.Equal(right)
}

func CountIf(predicate Expression[bool]) AggregateExpression[int64] {
	return FilterAggregate[int64](CountAll(), predicate)
}

func SumIf[T Numeric](expression Expression[T], predicate Expression[bool]) AggregateExpression[T] {
	return FilterAggregate[T](Sum[T](expression), predicate)
}

func AvgIf[T Numeric](expression Expression[T], predicate Expression[bool]) AggregateExpression[float64] {
	return FilterAggregate[float64](Avg[T](expression), predicate)
}

func MinIf[T Ordered](expression Expression[T], predicate Expression[bool]) AggregateExpression[T] {
	return FilterAggregate[T](Min[T](expression), predicate)
}

func MaxIf[T Ordered](expression Expression[T], predicate Expression[bool]) AggregateExpression[T] {
	return FilterAggregate[T](Max[T](expression), predicate)
}

type aggregateExpr[T any] struct {
	typedExpr[T]
}

type aggregatePluginDefinition struct {
	resultType reflect.Type
	evaluate   func(EvalContext) (Value, bool)
	factory    aggregatePluginFactory
	access     bool
}

// AggregatePluginState is the Go lifecycle contract for a stateful aggregate
// extension. Values are passed as Value so a plugin can distinguish Missing,
// Null and Present without relying on a Java-style nullable interface.
type AggregatePluginState[T any] interface {
	Enter(value Value)
	Leave(value Value)
	Value() (T, bool)
	Clear()
}

// AggregatePluginFactoryContext contains immutable information available when
// a new per-group plugin state is created. The factory itself is deliberately
// an explicit Go registration point; it is not loaded by class name.
type AggregatePluginFactoryContext struct {
	Name       string
	Event      Event
	Engine     *Engine
	Now        time.Time
	Variables  map[string]Value
	Parameters map[string]Value
}

// AggregatePluginFactory creates an independent state holder for one
// aggregate group. The runtime replays the current group into that state on
// each result transition, which keeps the state deterministic for nested
// filtered/local-group expressions while preserving group isolation.
type AggregatePluginFactory[T any] func(AggregatePluginFactoryContext) AggregatePluginState[T]

type aggregatePluginState interface {
	enter(Value)
	leave(Value)
	value() (Value, bool)
	clear()
	sync([]Value)
}

type aggregatePluginFactory func(AggregatePluginFactoryContext) aggregatePluginState

// aggregatePluginRuntimePanic marks a panic raised while invoking user-owned
// aggregate extension code. The statement runtime converts only this typed
// boundary into an error; unrelated internal panics remain visible to tests
// and the process instead of being silently swallowed.
type aggregatePluginRuntimePanic struct {
	plugin string
	cause  any
}

type aggregatePluginStateAdapter[T any] struct {
	state       AggregatePluginState[T]
	inputs      []Value
	initialized bool
}

func (a *aggregatePluginStateAdapter[T]) enter(value Value) { a.state.Enter(value) }
func (a *aggregatePluginStateAdapter[T]) leave(value Value) { a.state.Leave(value) }
func (a *aggregatePluginStateAdapter[T]) clear() {
	a.state.Clear()
	a.inputs = nil
	a.initialized = false
}
func (a *aggregatePluginStateAdapter[T]) sync(inputs []Value) {
	if a.initialized {
		for _, value := range a.inputs {
			a.state.Leave(value)
		}
	} else {
		a.state.Clear()
	}
	for _, value := range inputs {
		a.state.Enter(value)
	}
	a.inputs = append(a.inputs[:0], inputs...)
	a.initialized = true
}
func (a *aggregatePluginStateAdapter[T]) value() (Value, bool) {
	if a.state == nil {
		return Missing(), false
	}
	value, present := a.state.Value()
	if !present {
		return Null(), false
	}
	return Present(value), true
}

func adaptAggregatePluginFactory[T any](factory AggregatePluginFactory[T]) aggregatePluginFactory {
	if factory == nil {
		return nil
	}
	return func(ctx AggregatePluginFactoryContext) aggregatePluginState {
		state := factory(ctx)
		if state == nil {
			return nil
		}
		return &aggregatePluginStateAdapter[T]{state: state}
	}
}

func (aggregateExpr[T]) aggregateMarker() {}

func makeAggregateExpr[T any](kind, description string, children []*exprNode, fn func(EvalContext) Value) AggregateExpression[T] {
	return aggregateExpr[T]{typedExpr: typedExpr[T]{
		n:  &exprNode{kind: kind, typ: typeOf[T](), description: description, children: children},
		fn: fn,
	}}
}

// AggregatePluginInputs evaluates a typed argument vector once per retained
// event. It is the Go-native counterpart of a plug-in aggregation function's
// multiple parameters: constants, event fields, EventValue, and array values
// can be combined without hiding their expression nodes behind reflection.
// The resulting []Value preserves Missing and Null for each argument.
func AggregatePluginInputs(inputs ...Expr) Expression[[]Value] {
	children := make([]*exprNode, len(inputs))
	valid := true
	for index, input := range inputs {
		if input == nil {
			valid = false
			continue
		}
		children[index] = input.node()
	}
	return typedExpr[[]Value]{
		n: &exprNode{
			kind:        "aggregate-plugin-inputs",
			typ:         typeOf[[]Value](),
			description: "aggregate-plugin-inputs()",
			children:    children,
		},
		fn: func(ctx EvalContext) Value {
			if !valid {
				return Missing()
			}
			values := make([]Value, len(inputs))
			for index, input := range inputs {
				values[index] = input.eval(ctx)
			}
			return Present(values)
		},
	}
}

// PluginAggregate exposes a named Go aggregate extension without hiding its
// position in the analyzable expression tree. The evaluator receives the
// current and ever-retained group in EvalContext and returns (value, true) for
// a present result or (zero, false) for an aggregate null result.
//
// The runtime evaluates the plugin against the current group on every state
// transition. This keeps the extension deterministic and makes it work for
// both insert and remove-stream updates without requiring an unsafe mutable
// callback object in a Plan.
func PluginAggregate[T any](name string, evaluate func(EvalContext) (T, bool)) AggregateExpression[T] {
	name = strings.TrimSpace(name)
	description := "plugin-aggregate(<invalid>)"
	if name != "" {
		description = "plugin-aggregate(" + name + ")"
	}
	return aggregateExpr[T]{typedExpr: typedExpr[T]{
		n: &exprNode{kind: "aggregate-plugin", typ: typeOf[T](), description: description, pluginName: name, pluginReady: evaluate != nil},
		fn: func(ctx EvalContext) Value {
			if evaluate == nil {
				return Missing()
			}
			value, present := evaluate(ctx)
			if !present {
				return Null()
			}
			return Present(value)
		},
	}}
}

// PluginAggregateWithFactory creates a stateful aggregate extension. The
// optional input expression is evaluated once for every event in the current
// aggregate group and passed to AggregatePluginState.Enter. A nil expression
// passes the event itself as a Present Value, which is useful for event-aware
// plugins. FilterAggregate can be composed with this constructor for the
// named-filter behavior exposed by Esper's plug-in aggregate API.
func PluginAggregateWithFactory[T any](name string, input Expr, factory AggregatePluginFactory[T]) AggregateExpression[T] {
	name = strings.TrimSpace(name)
	description := "plugin-aggregate-factory(<invalid>)"
	if name != "" {
		description = "plugin-aggregate-factory(" + name + ")"
	}
	var children []*exprNode
	if input != nil {
		children = []*exprNode{input.node()}
	}
	node := &exprNode{
		kind:          "aggregate-plugin-factory",
		typ:           typeOf[T](),
		description:   description,
		pluginName:    name,
		pluginReady:   factory != nil,
		pluginFactory: adaptAggregatePluginFactory(factory),
		children:      children,
	}
	return aggregateExpr[T]{typedExpr: typedExpr[T]{
		n: node,
		fn: func(ctx EvalContext) Value {
			return evaluateAggregatePluginFactory(ctx, node, input, node.pluginFactory)
		},
	}}
}

// PluginAggregateAccess is the chainable Go form for an access-style
// aggregation plug-in. The factory still owns the typed state, while the
// optional filter is expressed as a normal analyzable predicate instead of a
// string named parameter. A nil input makes the factory receive the retained
// Event value, which is useful for plug-ins such as events-as-list that keep
// the source event rather than the scalar argument.
func PluginAggregateAccess[T any](name string, input Expr, factory AggregatePluginFactory[T], filter ...Expression[bool]) AggregateExpression[T] {
	aggregate := PluginAggregateWithFactory[T](name, input, factory)
	if len(filter) == 0 {
		return aggregate
	}
	if len(filter) != 1 || filter[0] == nil {
		// Reuse the regular filtered-aggregate validator so malformed
		// variadic filter input fails during Build instead of becoming a
		// silently unevaluable custom node.
		return FilterAggregate[T](aggregate, nil)
	}
	return FilterAggregate[T](aggregate, filter[0])
}

// RegisterAggregateAccessPlugin registers a named access-style aggregation
// extension. Access plugins use the same per-group state lifecycle as a
// stateful aggregate factory, but the registration is deliberately marked so
// PluginAggregateAccessRef cannot accidentally bind a method-style plugin to
// an access expression (or vice versa).
func RegisterAggregateAccessPlugin[T any](env *Environment, name string, factory AggregatePluginFactory[T]) error {
	return registerAggregatePluginFactory[T](env, name, factory, true)
}

// RegisterAggregatePlugin registers a named, typed aggregate extension in an
// Environment. It is the configuration-backed counterpart to PluginAggregate
// and lets multiple plans refer to the same extension by stable name.
func RegisterAggregatePlugin[T any](env *Environment, name string, evaluate func(EvalContext) (T, bool)) error {
	if env == nil {
		return NewError(ErrorDependency, "nil environment")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return NewError(ErrorInvalidRule, "aggregate plugin name is required")
	}
	if evaluate == nil {
		return NewError(ErrorInvalidRule, fmt.Sprintf("aggregate plugin %q has no evaluator", name))
	}
	env.mu.Lock()
	defer env.mu.Unlock()
	if _, exists := env.aggregatePlugins[name]; exists {
		return NewError(ErrorDependency, fmt.Sprintf("aggregate plugin %q is already registered", name))
	}
	if _, exists := env.aggregateMultiPlugins[name]; exists {
		return NewError(ErrorDependency, fmt.Sprintf("aggregate plugin %q conflicts with aggregate multi plugin", name))
	}
	env.aggregatePlugins[name] = aggregatePluginDefinition{
		resultType: typeOf[T](),
		evaluate: func(ctx EvalContext) (Value, bool) {
			value, present := evaluate(ctx)
			if !present {
				return Null(), false
			}
			return Present(value), true
		},
	}
	return nil
}

// RegisterAggregatePluginFactory registers a stateful aggregate factory in an
// Environment. Each aggregate group receives a separate state instance.
func RegisterAggregatePluginFactory[T any](env *Environment, name string, factory AggregatePluginFactory[T]) error {
	return registerAggregatePluginFactory[T](env, name, factory, false)
}

func registerAggregatePluginFactory[T any](env *Environment, name string, factory AggregatePluginFactory[T], access bool) error {
	if env == nil {
		return NewError(ErrorDependency, "nil environment")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return NewError(ErrorInvalidRule, "aggregate plugin factory name is required")
	}
	if factory == nil {
		return NewError(ErrorInvalidRule, fmt.Sprintf("aggregate plugin factory %q has no factory", name))
	}
	env.mu.Lock()
	defer env.mu.Unlock()
	if _, exists := env.aggregatePlugins[name]; exists {
		return NewError(ErrorDependency, fmt.Sprintf("aggregate plugin %q is already registered", name))
	}
	if _, exists := env.aggregateMultiPlugins[name]; exists {
		return NewError(ErrorDependency, fmt.Sprintf("aggregate plugin %q conflicts with aggregate multi plugin", name))
	}
	env.aggregatePlugins[name] = aggregatePluginDefinition{
		resultType: typeOf[T](),
		factory:    adaptAggregatePluginFactory(factory),
		access:     access,
	}
	return nil
}

// PluginAggregateRef creates a typed aggregate expression backed by a plugin
// registered in env. Build validates that the name exists and that its result
// type is compatible with T before the plan can be deployed.
func PluginAggregateRef[T any](env *Environment, name string) AggregateExpression[T] {
	name = strings.TrimSpace(name)
	description := "plugin-aggregate(<invalid>)"
	if name != "" {
		description = "plugin-aggregate(" + name + ")"
	}
	return aggregateExpr[T]{typedExpr: typedExpr[T]{
		n: &exprNode{kind: "aggregate-plugin-ref", typ: typeOf[T](), description: description, pluginName: name, pluginEnvironment: env},
		fn: func(ctx EvalContext) Value {
			if env == nil {
				return Missing()
			}
			env.mu.RLock()
			definition, ok := env.aggregatePlugins[name]
			env.mu.RUnlock()
			if !ok || definition.evaluate == nil {
				return Missing()
			}
			value, present := definition.evaluate(ctx)
			if !present {
				return Null()
			}
			return value
		},
	}}
}

// PluginAggregateFactoryRef creates a typed aggregate expression backed by a
// registered stateful factory. The input expression may be nil, matching
// PluginAggregateWithFactory's event-aware mode.
func PluginAggregateFactoryRef[T any](env *Environment, name string, input Expr) AggregateExpression[T] {
	return pluginAggregateFactoryRef[T](env, name, input, false)
}

// PluginAggregateAccessRef creates a typed access-aggregation expression
// backed by an access plugin registered in env. The optional filter is the
// Go-native equivalent of Esper's named `filter:` parameter.
func PluginAggregateAccessRef[T any](env *Environment, name string, input Expr, filter ...Expression[bool]) AggregateExpression[T] {
	aggregate := pluginAggregateFactoryRef[T](env, name, input, true)
	if len(filter) == 0 {
		return aggregate
	}
	if len(filter) != 1 || filter[0] == nil {
		return FilterAggregate[T](aggregate, nil)
	}
	return FilterAggregate[T](aggregate, filter[0])
}

func pluginAggregateFactoryRef[T any](env *Environment, name string, input Expr, access bool) AggregateExpression[T] {
	name = strings.TrimSpace(name)
	description := "plugin-aggregate-factory(<invalid>)"
	if name != "" {
		description = "plugin-aggregate-factory(" + name + ")"
	}
	kind := "aggregate-plugin-factory-ref"
	if access {
		kind = "aggregate-plugin-access-ref"
		description = "plugin-aggregate-access-ref(<invalid>)"
		if name != "" {
			description = "plugin-aggregate-access-ref(" + name + ")"
		}
	}
	var children []*exprNode
	if input != nil {
		children = []*exprNode{input.node()}
	}
	node := &exprNode{
		kind:              kind,
		typ:               typeOf[T](),
		description:       description,
		pluginName:        name,
		pluginEnvironment: env,
		pluginAccess:      access,
		children:          children,
	}
	return aggregateExpr[T]{typedExpr: typedExpr[T]{
		n: node,
		fn: func(ctx EvalContext) Value {
			if env == nil {
				return Missing()
			}
			env.mu.RLock()
			definition, ok := env.aggregatePlugins[name]
			env.mu.RUnlock()
			if !ok || definition.factory == nil {
				return Missing()
			}
			return evaluateAggregatePluginFactory(ctx, node, input, definition.factory)
		},
	}}
}

func evaluateAggregatePluginFactory(ctx EvalContext, node *exprNode, input Expr, factory aggregatePluginFactory) (result Value) {
	if node == nil || factory == nil {
		return Missing()
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			if _, alreadyMarked := recovered.(aggregatePluginRuntimePanic); alreadyMarked {
				panic(recovered)
			}
			panic(aggregatePluginRuntimePanic{plugin: node.pluginName, cause: recovered})
		}
	}()
	state := aggregatePluginState(nil)
	if ctx.aggregatePluginStates != nil {
		state = ctx.aggregatePluginStates[node]
	}
	if state == nil {
		state = factory(AggregatePluginFactoryContext{
			Name:       node.pluginName,
			Event:      ctx.Event,
			Engine:     ctx.Engine,
			Now:        ctx.Now,
			Variables:  ctx.Variables,
			Parameters: ctx.Parameters,
		})
		if state == nil {
			return Missing()
		}
		if ctx.aggregatePluginStates != nil {
			ctx.aggregatePluginStates[node] = state
		}
	}
	inputs := make([]Value, 0, len(ctx.Group))
	for index, event := range ctx.Group {
		value := Present(event)
		if input != nil {
			value = input.eval(ctx.groupEventContext(event, index))
		}
		inputs = append(inputs, value)
	}
	state.sync(inputs)
	value, present := state.value()
	if !present {
		return Null()
	}
	return value
}

// CountMinSketchValue is a compact frequency estimator. The implementation
// uses deterministic FNV-1a rows and preserves the Count-Min Sketch contract:
// estimates never under-count, while collisions may over-count.
type CountMinSketchValue[T comparable] struct {
	width    uint32
	depth    uint32
	counters []uint64
	total    int64
}

const (
	defaultCountMinSketchWidth = 256
	defaultCountMinSketchDepth = 5
)

func newCountMinSketch[T comparable]() CountMinSketchValue[T] {
	return CountMinSketchValue[T]{
		width:    defaultCountMinSketchWidth,
		depth:    defaultCountMinSketchDepth,
		counters: make([]uint64, defaultCountMinSketchWidth*defaultCountMinSketchDepth),
	}
}

func (s *CountMinSketchValue[T]) add(key T) {
	if s == nil || s.width == 0 || s.depth == 0 {
		return
	}
	for row := uint32(0); row < s.depth; row++ {
		index := row*s.width + uint32(countMinSketchHash(key, uint64(row))%uint64(s.width))
		s.counters[index]++
	}
	s.total++
}

func (s CountMinSketchValue[T]) Frequency(key T) int64 {
	if s.width == 0 || s.depth == 0 || len(s.counters) == 0 {
		return 0
	}
	minimum := ^uint64(0)
	for row := uint32(0); row < s.depth; row++ {
		index := row*s.width + uint32(countMinSketchHash(key, uint64(row))%uint64(s.width))
		if s.counters[index] < minimum {
			minimum = s.counters[index]
		}
	}
	if minimum > uint64(^uint64(0)>>1) {
		return int64(^uint64(0) >> 1)
	}
	return int64(minimum)
}

func (s CountMinSketchValue[T]) Total() int64 { return s.total }

func countMinSketchHash[T comparable](key T, seed uint64) uint64 {
	hasher := fnv.New64a()
	_, _ = fmt.Fprintf(hasher, "%d:%#v", seed, key)
	return hasher.Sum64()
}

// CountMinSketchExpression is a chainable aggregate that adds each present
// expression value, optionally gated by one boolean predicate.
type CountMinSketchExpression[T comparable] struct {
	aggregateExpr[CountMinSketchValue[T]]
	expression Expression[T]
	predicate  Expression[bool]
}

func CountMinSketchAdd[T comparable](expression Expression[T], predicate ...Expression[bool]) CountMinSketchExpression[T] {
	var filter Expression[bool]
	if len(predicate) == 1 {
		filter = predicate[0]
	}
	children := make([]*exprNode, 0, 1+len(predicate))
	if expression != nil {
		children = append(children, expression.node())
	}
	for _, item := range predicate {
		if item != nil {
			children = append(children, item.node())
		}
	}
	description := "count-min-sketch(<invalid>)"
	if expression != nil && len(predicate) <= 1 {
		description = "count-min-sketch(" + expression.Description() + ")"
	}
	return CountMinSketchExpression[T]{
		aggregateExpr: aggregateExpr[CountMinSketchValue[T]]{typedExpr: typedExpr[CountMinSketchValue[T]]{
			n: &exprNode{kind: "count-min-sketch", typ: typeOf[CountMinSketchValue[T]](), description: description, children: children},
			fn: func(ctx EvalContext) Value {
				if expression == nil || len(predicate) > 1 || (len(predicate) == 1 && filter == nil) {
					return Missing()
				}
				sketch := newCountMinSketch[T]()
				for index, event := range ctx.Group {
					eventContext := ctx.groupEventContext(event, index)
					if filter != nil {
						allowed, ok := boolValue(filter.eval(eventContext))
						if !ok || !allowed {
							continue
						}
					}
					value := expression.eval(eventContext)
					if !value.IsPresent() {
						continue
					}
					converted, ok := value.Any().(T)
					if ok {
						sketch.add(converted)
					}
				}
				return Present(sketch)
			},
		}},
		expression: expression,
		predicate:  filter,
	}
}

func (s CountMinSketchExpression[T]) Frequency(key Expression[T]) AggregateExpression[int64] {
	children := []*exprNode{s.node()}
	if key != nil {
		children = append(children, key.node())
	}
	return makeAggregateExpr[int64]("count-min-frequency", "count-min-frequency("+expressionDescription(key)+")", children, func(ctx EvalContext) Value {
		if key == nil {
			return Missing()
		}
		value := s.eval(ctx)
		if !value.IsPresent() {
			return value
		}
		sketch, ok := value.Any().(CountMinSketchValue[T])
		if !ok {
			return Missing()
		}
		keyValue, ok := key.eval(ctx).Any().(T)
		if !ok {
			return Null()
		}
		return Present(sketch.Frequency(keyValue))
	})
}

func (s CountMinSketchExpression[T]) Total() AggregateExpression[int64] {
	return makeAggregateExpr[int64]("count-min-total", "count-min-total()", []*exprNode{s.node()}, func(ctx EvalContext) Value {
		value := s.eval(ctx)
		if !value.IsPresent() {
			return value
		}
		sketch, ok := value.Any().(CountMinSketchValue[T])
		if !ok {
			return Missing()
		}
		return Present(sketch.Total())
	})
}

// CountMinSketchFrequency reads a sketch-valued expression, including a
// TableField or another target-row expression, for a runtime key.
func CountMinSketchFrequency[T comparable](sketch Expression[CountMinSketchValue[T]], key Expression[T]) Expression[int64] {
	children := make([]*exprNode, 0, 2)
	if sketch != nil {
		children = append(children, sketch.node())
	}
	if key != nil {
		children = append(children, key.node())
	}
	return makeExpr[int64]("count-min-frequency-ref", "count-min-frequency("+expressionDescription(key)+")", children, func(ctx EvalContext) Value {
		if sketch == nil || key == nil {
			return Missing()
		}
		value := sketch.eval(ctx)
		if !value.IsPresent() {
			return value
		}
		countSketch, ok := value.Any().(CountMinSketchValue[T])
		if !ok {
			return Missing()
		}
		keyValue, ok := key.eval(ctx).Any().(T)
		if !ok {
			return Null()
		}
		return Present(countSketch.Frequency(keyValue))
	})
}

func CountAll() AggregateExpression[int64] {
	return makeAggregateExpr[int64]("count", "count(*)", nil, func(ctx EvalContext) Value {
		return Present(int64(len(ctx.Group)))
	})
}

func Count[T any](expression Expression[T]) AggregateExpression[int64] {
	return makeAggregateExpr[int64]("count", "count("+expression.Description()+")", []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		var count int64
		for index, event := range ctx.Group {
			value := expression.eval(ctx.groupEventContext(event, index))
			if value.IsPresent() {
				count++
			}
		}
		return Present(count)
	})
}

func Sum[T Numeric](expression Expression[T]) AggregateExpression[T] {
	return makeAggregateExpr[T]("sum", "sum("+expression.Description()+")", []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		var total float64
		found := false
		for index, event := range ctx.Group {
			value, ok := numericValue(expression.eval(ctx.groupEventContext(event, index)))
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

func Avg[T Numeric](expression Expression[T]) AggregateExpression[float64] {
	return makeAggregateExpr[float64]("avg", "avg("+expression.Description()+")", []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		var total float64
		var count int64
		for index, event := range ctx.Group {
			value, ok := numericValue(expression.eval(ctx.groupEventContext(event, index)))
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

// SumExact accumulates big.Int or big.Rat values with arbitrary precision.
// For big.Int input the result remains a big.Int; for big.Rat input the
// result remains a big.Rat. Missing and null values are ignored, matching the
// ordinary Sum aggregate's input policy.
func SumExact[T ExactNumeric](expression Expression[T]) AggregateExpression[T] {
	if expression == nil {
		return invalidAggregate[T]("sum-exact", expression, "sum-exact(<nil>)")
	}
	return makeAggregateExpr[T]("sum-exact", "sum-exact("+expression.Description()+")", []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		total := new(big.Rat)
		found := false
		for index, event := range ctx.Group {
			number, ok := enumRatFromValue(expression.eval(ctx.groupEventContext(event, index)))
			if !ok {
				continue
			}
			total.Add(total, number)
			found = true
		}
		if !found {
			return Null()
		}
		result, ok := enumRatTo[T](total)
		if !ok {
			return Null()
		}
		return Present(result)
	})
}

// AvgExact computes an arbitrary-precision average and returns a big.Rat.
// Returning a rational keeps recurring decimals and very large operands
// exact; callers can choose a presentation scale at the edge of their
// application with big.Rat.FloatString or conversion to another decimal
// representation.
func AvgExact[T ExactNumeric](expression Expression[T]) AggregateExpression[big.Rat] {
	if expression == nil {
		return invalidAggregate[big.Rat]("avg-exact", expression, "avg-exact(<nil>)")
	}
	return makeAggregateExpr[big.Rat]("avg-exact", "avg-exact("+expression.Description()+")", []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		total := new(big.Rat)
		count := int64(0)
		for index, event := range ctx.Group {
			number, ok := enumRatFromValue(expression.eval(ctx.groupEventContext(event, index)))
			if !ok {
				continue
			}
			total.Add(total, number)
			count++
		}
		if count == 0 {
			return Null()
		}
		average := new(big.Rat).Quo(total, new(big.Rat).SetInt64(count))
		return Present(*average)
	})
}

// MinExact returns the smallest arbitrary-precision numeric value in the
// current group while preserving the input type.
func MinExact[T ExactNumeric](expression Expression[T]) AggregateExpression[T] {
	return exactExtreme[T]("min-exact", expression, true)
}

// MaxExact returns the largest arbitrary-precision numeric value in the
// current group while preserving the input type.
func MaxExact[T ExactNumeric](expression Expression[T]) AggregateExpression[T] {
	return exactExtreme[T]("max-exact", expression, false)
}

func exactExtreme[T ExactNumeric](kind string, expression Expression[T], minimum bool) AggregateExpression[T] {
	if expression == nil {
		return invalidAggregate[T](kind, expression, kind+"(<nil>)")
	}
	return makeAggregateExpr[T](kind, kind+"("+expression.Description()+")", []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		var result T
		var resultNumber *big.Rat
		found := false
		for index, event := range ctx.Group {
			value := expression.eval(ctx.groupEventContext(event, index))
			number, ok := enumRatFromValue(value)
			if !ok {
				continue
			}
			candidate, err := As[T](value)
			if err != nil {
				continue
			}
			if !found || (minimum && number.Cmp(resultNumber) < 0) || (!minimum && number.Cmp(resultNumber) > 0) {
				result = candidate
				resultNumber = number
				found = true
			}
		}
		if !found {
			return Null()
		}
		return Present(result)
	})
}

func Min[T Ordered](expression Expression[T]) AggregateExpression[T] {
	return aggregateExtreme[T]("min", expression, true)
}

func Max[T Ordered](expression Expression[T]) AggregateExpression[T] {
	return aggregateExtreme[T]("max", expression, false)
}

// First returns the first non-null value in insertion order. An optional
// zero-based index mirrors Esper's first(value, index) form while keeping the
// one-argument Go call concise.
func First[T any](expression Expression[T], indexes ...int) AggregateExpression[T] {
	index, valid := aggregateIndex(indexes)
	if !valid {
		return invalidAggregate[T]("first", expression, "first(<invalid>)")
	}
	description := "first(" + expressionDescription(expression) + ")"
	if len(indexes) == 1 {
		description = fmt.Sprintf("first(%s,%d)", expressionDescription(expression), index)
	}
	return aggregatePosition[T]("first", expression, index, false, description)
}

// Last returns the last non-null value in reverse insertion order. An optional
// zero-based index mirrors Esper's last(value, index) form.
func Last[T any](expression Expression[T], indexes ...int) AggregateExpression[T] {
	index, valid := aggregateIndex(indexes)
	if !valid {
		return invalidAggregate[T]("last", expression, "last(<invalid>)")
	}
	description := "last(" + expressionDescription(expression) + ")"
	if len(indexes) == 1 {
		description = fmt.Sprintf("last(%s,%d)", expressionDescription(expression), index)
	}
	return aggregatePosition[T]("last", expression, index, true, description)
}

// FirstEventValue and LastEventValue are the Go names for no-argument
// first()/last() access aggregation. They retain the Event identity, so
// callers can continue with Property or inspect the underlying value.
func FirstEventValue() AggregateExpression[Event] {
	return First[Event](EventValue[Event]())
}

func LastEventValue() AggregateExpression[Event] {
	return Last[Event](EventValue[Event]())
}

// FirstEver returns the first non-null value seen by the aggregate group,
// including events that have since left the current data window.
func FirstEver[T any](expression Expression[T]) AggregateExpression[T] {
	return aggregateEverPosition[T]("first-ever", expression, false)
}

// LastEver returns the latest non-null value ever seen by the aggregate group.
func LastEver[T any](expression Expression[T]) AggregateExpression[T] {
	return aggregateEverPosition[T]("last-ever", expression, true)
}

func aggregateEverPosition[T any](kind string, expression Expression[T], last bool) AggregateExpression[T] {
	if expression == nil {
		return makeAggregateExpr[T](kind, kind+"(<nil>)", nil, func(EvalContext) Value { return Missing() })
	}
	return makeAggregateExpr[T](kind, kind+"("+expression.Description()+")", []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		var result Value
		for _, event := range ctx.EverGroup {
			value := expression.eval(EvalContext{Event: event, Now: ctx.Now, Variables: ctx.Variables, Parameters: ctx.Parameters})
			if !value.IsPresent() {
				continue
			}
			if !last {
				return value
			}
			result = value
		}
		if !result.IsPresent() {
			return Null()
		}
		return result
	})
}

// CountEver counts all events ever accepted by the group. With one
// expression it counts non-null values, mirroring Count; with no expression
// it counts every event.
func CountEver(expressions ...Expr) AggregateExpression[int64] {
	if len(expressions) > 1 {
		return makeAggregateExpr[int64]("count-ever-invalid", "count-ever(<invalid>)", nil, func(EvalContext) Value { return Missing() })
	}
	var expression Expr
	if len(expressions) == 1 {
		expression = expressions[0]
	}
	children := []*exprNode(nil)
	description := "count-ever(*)"
	if expression != nil {
		children = []*exprNode{expression.node()}
		description = "count-ever(" + expression.Description() + ")"
	}
	return makeAggregateExpr[int64]("count-ever", description, children, func(ctx EvalContext) Value {
		if expression == nil {
			return Present(int64(len(ctx.EverGroup)))
		}
		var count int64
		for _, event := range ctx.EverGroup {
			value := expression.eval(EvalContext{Event: event, Now: ctx.Now, Variables: ctx.Variables, Parameters: ctx.Parameters})
			if value.IsPresent() {
				count++
			}
		}
		return Present(count)
	})
}

// Nth returns the zero-based nth non-null value in current group order.
// Negative indexes are invalid at evaluation time and produce Null.
func Nth[T any](expression Expression[T], index int) AggregateExpression[T] {
	return aggregatePosition[T](fmt.Sprintf("nth(%d)", index), expression, index, false, "")
}

func aggregatePosition[T any](kind string, expression Expression[T], index int, last bool, description string) AggregateExpression[T] {
	if expression == nil {
		return invalidAggregate[T](kind, expression, description)
	}
	if description == "" {
		description = kind + "(" + expression.Description() + ")"
	}
	return makeAggregateExpr[T](kind, description, []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		if index < 0 {
			return Null()
		}
		group := ctx.Group
		// A first/last expression can be nested in an ordinary projection in
		// the same way Esper permits an access aggregate on the current
		// event.  Aggregate plans provide Group explicitly; scalar projection
		// paths provide only Event, which is the one-row group for this
		// access operation.
		if len(group) == 0 && !ctx.aggregateEvaluation && ctx.Event.schema.valid() {
			group = []Event{ctx.Event}
		}
		values := make([]Value, 0, len(group))
		for _, event := range group {
			value := expression.eval(EvalContext{Event: event, Now: ctx.Now, Variables: ctx.Variables, Parameters: ctx.Parameters})
			if value.IsPresent() {
				values = append(values, value)
			}
		}
		if len(values) == 0 {
			return Null()
		}
		if last {
			position := len(values) - 1 - index
			if position < 0 {
				return Null()
			}
			return values[position]
		}
		if index >= len(values) {
			return Null()
		}
		return values[index]
	})
}

func aggregateIndex(indexes []int) (int, bool) {
	if len(indexes) > 1 {
		return 0, false
	}
	if len(indexes) == 0 {
		return 0, true
	}
	return indexes[0], true
}

func invalidAggregate[T any](kind string, expression Expr, description string) AggregateExpression[T] {
	if description == "" {
		description = kind + "(<invalid>)"
	}
	var children []*exprNode
	if expression != nil {
		children = []*exprNode{expression.node()}
	}
	return makeAggregateExpr[T](kind, description, children, func(EvalContext) Value { return Missing() })
}

func CountDistinct[T comparable](expression Expression[T]) AggregateExpression[int64] {
	return makeAggregateExpr[int64]("count-distinct", "count-distinct("+expression.Description()+")", []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		seen := make(map[any]struct{})
		fallback := make(map[string]struct{})
		for index, event := range ctx.Group {
			value := expression.eval(ctx.groupEventContext(event, index))
			if !value.IsPresent() {
				continue
			}
			candidate := value.Any()
			// Dereference pointer candidates so that distinctness follows
			// the pointed-to value, matching Esper's equals-based
			// count(distinct ...) on boxed event properties (two Long(25)
			// values are one distinct value regardless of object identity).
			reflected := reflect.ValueOf(candidate)
			for reflected.IsValid() && (reflected.Kind() == reflect.Pointer || reflected.Kind() == reflect.Interface) {
				if reflected.IsNil() {
					break
				}
				reflected = reflected.Elem()
			}
			if reflected.IsValid() && reflected.Kind() != reflect.Pointer && reflected.Kind() != reflect.Interface {
				candidate = reflected.Interface()
			}
			if reflect.ValueOf(candidate).IsValid() && reflect.ValueOf(candidate).Comparable() {
				seen[candidate] = struct{}{}
				continue
			}
			fallback[fmt.Sprintf("%T:%#v", candidate, candidate)] = struct{}{}
		}
		return Present(int64(len(seen) + len(fallback)))
	})
}

func Median[T Numeric](expression Expression[T]) AggregateExpression[float64] {
	return makeAggregateExpr[float64]("median", "median("+expression.Description()+")", []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		values := numericAggregateValues[T](expression, ctx)
		if len(values) == 0 {
			return Null()
		}
		sort.Float64s(values)
		middle := len(values) / 2
		if len(values)%2 == 1 {
			return Present(values[middle])
		}
		return Present((values[middle-1] + values[middle]) / 2)
	})
}

// StdDev uses the sample standard-deviation convention used by Esper's
// stddev aggregate. A singleton group has no defined sample deviation and
// therefore evaluates to Null, matching Esper's aggregate result semantics.
func StdDev[T Numeric](expression Expression[T]) AggregateExpression[float64] {
	return makeAggregateExpr[float64]("stddev", "stddev("+expression.Description()+")", []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		values := numericAggregateValues[T](expression, ctx)
		if len(values) < 2 {
			return Null()
		}
		var total float64
		for _, value := range values {
			total += value
		}
		mean := total / float64(len(values))
		var squared float64
		for _, value := range values {
			delta := value - mean
			squared += delta * delta
		}
		return Present(math.Sqrt(squared / float64(len(values)-1)))
	})
}

// StdDevPop computes population standard deviation. It is kept separate from
// StdDev because Esper exposes both sample and population conventions.
func StdDevPop[T Numeric](expression Expression[T]) AggregateExpression[float64] {
	return makeAggregateExpr[float64]("stddev-pop", "stddev-pop("+expression.Description()+")", []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		values := numericAggregateValues[T](expression, ctx)
		if len(values) == 0 {
			return Null()
		}
		return Present(math.Sqrt(populationVariance(values)))
	})
}

// Variance computes sample variance. A singleton group has variance zero,
// matching the sample convention used by StdDev in this package.
func Variance[T Numeric](expression Expression[T]) AggregateExpression[float64] {
	return makeAggregateExpr[float64]("variance", "variance("+expression.Description()+")", []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		values := numericAggregateValues[T](expression, ctx)
		if len(values) == 0 {
			return Null()
		}
		if len(values) == 1 {
			return Present(float64(0))
		}
		return Present(sampleVariance(values))
	})
}

// Avedev computes the mean absolute deviation from the group mean.
func Avedev[T Numeric](expression Expression[T]) AggregateExpression[float64] {
	return makeAggregateExpr[float64]("avedev", "avedev("+expression.Description()+")", []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		values := numericAggregateValues[T](expression, ctx)
		if len(values) == 0 {
			return Null()
		}
		var total float64
		for _, value := range values {
			total += value
		}
		mean := total / float64(len(values))
		var deviation float64
		for _, value := range values {
			deviation += math.Abs(value - mean)
		}
		return Present(deviation / float64(len(values)))
	})
}

// WeightedAvg computes sum(value*weight)/sum(weight), ignoring rows where
// either expression is not numeric or where the total weight is zero.
func WeightedAvg[V Numeric, W Numeric](value Expression[V], weight Expression[W]) AggregateExpression[float64] {
	return makeAggregateExpr[float64]("weighted-avg", "weighted-avg("+value.Description()+","+weight.Description()+")", []*exprNode{value.node(), weight.node()}, func(ctx EvalContext) Value {
		var weighted, totalWeight float64
		for index, event := range ctx.Group {
			evalContext := ctx.groupEventContext(event, index)
			candidate, valueOK := numericValue(value.eval(evalContext))
			factor, weightOK := numericValue(weight.eval(evalContext))
			if !valueOK || !weightOK {
				continue
			}
			weighted += candidate * factor
			totalWeight += factor
		}
		if totalWeight == 0 {
			return Null()
		}
		return Present(weighted / totalWeight)
	})
}

// Correlation computes Pearson's correlation coefficient over the current
// aggregate group. Rows for which either input is missing, null or nonnumeric
// are ignored. Fewer than two usable pairs, or a zero-variance input, produce
// NaN, matching Esper's correl view contract for an undefined coefficient.
func Correlation[X Numeric, Y Numeric](left Expression[X], right Expression[Y]) AggregateExpression[float64] {
	children := []*exprNode(nil)
	if left != nil {
		children = append(children, left.node())
	}
	if right != nil {
		children = append(children, right.node())
	}
	description := "correlation(<invalid>)"
	if left != nil && right != nil {
		description = "correlation(" + left.Description() + "," + right.Description() + ")"
	}
	return makeAggregateExpr[float64]("correlation", description, children, func(ctx EvalContext) Value {
		if left == nil || right == nil {
			return Missing()
		}
		xs, ys := numericPairs[X, Y](left, right, ctx)
		if len(xs) < 2 {
			return Present(math.NaN())
		}
		var sumX, sumY float64
		for index := range xs {
			sumX += xs[index]
			sumY += ys[index]
		}
		meanX := sumX / float64(len(xs))
		meanY := sumY / float64(len(ys))
		var numerator, sumXX, sumYY float64
		for index := range xs {
			deltaX := xs[index] - meanX
			deltaY := ys[index] - meanY
			numerator += deltaX * deltaY
			sumXX += deltaX * deltaX
			sumYY += deltaY * deltaY
		}
		denominator := math.Sqrt(sumXX * sumYY)
		if denominator == 0 {
			return Present(math.NaN())
		}
		return Present(numerator / denominator)
	})
}

// Correl is the short name used by Esper's correl view. Correlation is the
// more descriptive Go spelling; both constructors produce the same AST.
func Correl[X Numeric, Y Numeric](left Expression[X], right Expression[Y]) AggregateExpression[float64] {
	return Correlation[X, Y](left, right)
}

// LinearRegressionValue is the immutable result of LinearRegression. The
// accessors intentionally expose both the Go spelling Intercept and Esper's
// YIntercept terminology.
type LinearRegressionValue struct {
	slope     float64
	intercept float64
}

func (v LinearRegressionValue) Slope() float64      { return v.slope }
func (v LinearRegressionValue) Intercept() float64  { return v.intercept }
func (v LinearRegressionValue) YIntercept() float64 { return v.intercept }

// LinearRegressionExpression is the chainable Go representation of Esper's
// linest view. Use .Slope() and .YIntercept()/.Intercept() as aggregate
// selections, for example:
//
//	stream.Window(LengthWindow(3)).Aggregate(
//	    Alias("slope", LinearRegression(price, volume).Slope()),
//	    Alias("YIntercept", LinearRegression(price, volume).YIntercept()),
//	)
//
// The calculation is recomputed from the current group on every transition,
// so length/time window removals have the same observable behavior as Java's
// derived view.
type LinearRegressionExpression[X Numeric, Y Numeric] struct {
	aggregateExpr[LinearRegressionValue]
}

func LinearRegression[X Numeric, Y Numeric](x Expression[X], y Expression[Y]) LinearRegressionExpression[X, Y] {
	children := []*exprNode(nil)
	if x != nil {
		children = append(children, x.node())
	}
	if y != nil {
		children = append(children, y.node())
	}
	description := "linear-regression(<invalid>)"
	if x != nil && y != nil {
		description = "linear-regression(" + x.Description() + "," + y.Description() + ")"
	}
	return LinearRegressionExpression[X, Y]{aggregateExpr: aggregateExpr[LinearRegressionValue]{typedExpr: typedExpr[LinearRegressionValue]{
		n: &exprNode{kind: "linear-regression", typ: typeOf[LinearRegressionValue](), description: description, children: children},
		fn: func(ctx EvalContext) Value {
			if x == nil || y == nil {
				return Missing()
			}
			xs, ys := numericPairs[X, Y](x, y, ctx)
			if len(xs) < 2 {
				return Present(LinearRegressionValue{slope: math.NaN(), intercept: math.NaN()})
			}
			var sumX, sumY float64
			for index := range xs {
				sumX += xs[index]
				sumY += ys[index]
			}
			meanX := sumX / float64(len(xs))
			meanY := sumY / float64(len(ys))
			var covariance, varianceX float64
			for index := range xs {
				deltaX := xs[index] - meanX
				covariance += deltaX * (ys[index] - meanY)
				varianceX += deltaX * deltaX
			}
			if varianceX == 0 {
				return Present(LinearRegressionValue{slope: math.NaN(), intercept: math.NaN()})
			}
			slope := covariance / varianceX
			return Present(LinearRegressionValue{slope: slope, intercept: meanY - slope*meanX})
		},
	}}}
}

func (r LinearRegressionExpression[X, Y]) Slope() AggregateExpression[float64] {
	return linearRegressionValueExpression(r, "slope", func(value LinearRegressionValue) float64 { return value.Slope() })
}

func (r LinearRegressionExpression[X, Y]) Intercept() AggregateExpression[float64] {
	return linearRegressionValueExpression(r, "intercept", func(value LinearRegressionValue) float64 { return value.Intercept() })
}

func (r LinearRegressionExpression[X, Y]) YIntercept() AggregateExpression[float64] {
	return linearRegressionValueExpression(r, "YIntercept", func(value LinearRegressionValue) float64 { return value.YIntercept() })
}

// Linest is the short name used by Esper's linest view.
func Linest[X Numeric, Y Numeric](x Expression[X], y Expression[Y]) LinearRegressionExpression[X, Y] {
	return LinearRegression[X, Y](x, y)
}

func linearRegressionValueExpression[X Numeric, Y Numeric](regression LinearRegressionExpression[X, Y], name string, selectValue func(LinearRegressionValue) float64) AggregateExpression[float64] {
	return makeAggregateExpr[float64]("linear-regression-"+strings.ToLower(name), strings.ToLower(name)+"("+regression.Description()+")", []*exprNode{regression.node()}, func(ctx EvalContext) Value {
		value := regression.eval(ctx)
		if !value.IsPresent() {
			return value
		}
		result, ok := value.Any().(LinearRegressionValue)
		if !ok {
			return Missing()
		}
		return Present(selectValue(result))
	})
}

func numericPairs[X Numeric, Y Numeric](left Expression[X], right Expression[Y], ctx EvalContext) ([]float64, []float64) {
	xs := make([]float64, 0, len(ctx.Group))
	ys := make([]float64, 0, len(ctx.Group))
	for index, event := range ctx.Group {
		eventContext := ctx.groupEventContext(event, index)
		x, xOK := numericValue(left.eval(eventContext))
		y, yOK := numericValue(right.eval(eventContext))
		if !xOK || !yOK {
			continue
		}
		xs = append(xs, x)
		ys = append(ys, y)
	}
	return xs, ys
}

// UnivariateStatisticsValue is the immutable value behind the Java #uni
// derived view. Variance and StdDev use the sample convention and therefore
// are NaN until two data points exist; StdDevPop is zero for one data point and
// NaN for an empty group.
type UnivariateStatisticsValue struct {
	total    float64
	count    int64
	average  float64
	variance float64
	stddev   float64
	stddevpa float64
}

func (v UnivariateStatisticsValue) Total() float64     { return v.total }
func (v UnivariateStatisticsValue) Datapoints() int64  { return v.count }
func (v UnivariateStatisticsValue) Count() int64       { return v.count }
func (v UnivariateStatisticsValue) Average() float64   { return v.average }
func (v UnivariateStatisticsValue) Variance() float64  { return v.variance }
func (v UnivariateStatisticsValue) StdDev() float64    { return v.stddev }
func (v UnivariateStatisticsValue) StdDevPop() float64 { return v.stddevpa }
func (v UnivariateStatisticsValue) StdDevPA() float64  { return v.stddevpa }

// UnivariateStatistics is the chainable Go representation of Esper's uni
// derived view. Its field methods are aggregate expressions and can be
// selected individually, which keeps the result schema explicit and idiomatic
// for Go while preserving the Java field semantics.
type UnivariateStatisticsExpression[T Numeric] struct {
	aggregateExpr[UnivariateStatisticsValue]
}

func UnivariateStatistics[T Numeric](expression Expression[T]) UnivariateStatisticsExpression[T] {
	children := []*exprNode(nil)
	description := "univariate-statistics(<invalid>)"
	if expression != nil {
		children = []*exprNode{expression.node()}
		description = "univariate-statistics(" + expression.Description() + ")"
	}
	return UnivariateStatisticsExpression[T]{aggregateExpr: aggregateExpr[UnivariateStatisticsValue]{typedExpr: typedExpr[UnivariateStatisticsValue]{
		n: &exprNode{kind: "univariate-statistics", typ: typeOf[UnivariateStatisticsValue](), description: description, children: children},
		fn: func(ctx EvalContext) Value {
			if expression == nil {
				return Missing()
			}
			values := numericAggregateValues[T](expression, ctx)
			result := UnivariateStatisticsValue{count: int64(len(values)), total: 0, average: math.NaN(), variance: math.NaN(), stddev: math.NaN(), stddevpa: math.NaN()}
			for _, value := range values {
				result.total += value
			}
			if len(values) == 0 {
				return Present(result)
			}
			result.average = result.total / float64(len(values))
			result.stddevpa = math.Sqrt(populationVariance(values))
			if len(values) == 1 {
				return Present(result)
			}
			result.variance = sampleVariance(values)
			result.stddev = math.Sqrt(result.variance)
			return Present(result)
		},
	}}}
}

func (s UnivariateStatisticsExpression[T]) Total() AggregateExpression[float64] {
	return univariateStatisticsValueExpression(s, "total", func(value UnivariateStatisticsValue) float64 { return value.Total() })
}

func (s UnivariateStatisticsExpression[T]) Datapoints() AggregateExpression[int64] {
	return univariateStatisticsIntExpression(s, "datapoints", func(value UnivariateStatisticsValue) int64 { return value.Datapoints() })
}

func (s UnivariateStatisticsExpression[T]) Count() AggregateExpression[int64] {
	return s.Datapoints()
}

func (s UnivariateStatisticsExpression[T]) Average() AggregateExpression[float64] {
	return univariateStatisticsValueExpression(s, "average", func(value UnivariateStatisticsValue) float64 { return value.Average() })
}

func (s UnivariateStatisticsExpression[T]) Variance() AggregateExpression[float64] {
	return univariateStatisticsValueExpression(s, "variance", func(value UnivariateStatisticsValue) float64 { return value.Variance() })
}

func (s UnivariateStatisticsExpression[T]) StdDev() AggregateExpression[float64] {
	return univariateStatisticsValueExpression(s, "stddev", func(value UnivariateStatisticsValue) float64 { return value.StdDev() })
}

func (s UnivariateStatisticsExpression[T]) StdDevPop() AggregateExpression[float64] {
	return univariateStatisticsValueExpression(s, "stddev-pop", func(value UnivariateStatisticsValue) float64 { return value.StdDevPop() })
}

func (s UnivariateStatisticsExpression[T]) StdDevPA() AggregateExpression[float64] {
	return s.StdDevPop()
}

func univariateStatisticsValueExpression[T Numeric](statistics UnivariateStatisticsExpression[T], name string, selectValue func(UnivariateStatisticsValue) float64) AggregateExpression[float64] {
	return makeAggregateExpr[float64]("univariate-statistics-"+name, name+"("+statistics.Description()+")", []*exprNode{statistics.node()}, func(ctx EvalContext) Value {
		value := statistics.eval(ctx)
		if !value.IsPresent() {
			return value
		}
		result, ok := value.Any().(UnivariateStatisticsValue)
		if !ok {
			return Missing()
		}
		return Present(selectValue(result))
	})
}

func univariateStatisticsIntExpression[T Numeric](statistics UnivariateStatisticsExpression[T], name string, selectValue func(UnivariateStatisticsValue) int64) AggregateExpression[int64] {
	return makeAggregateExpr[int64]("univariate-statistics-"+name, name+"("+statistics.Description()+")", []*exprNode{statistics.node()}, func(ctx EvalContext) Value {
		value := statistics.eval(ctx)
		if !value.IsPresent() {
			return value
		}
		result, ok := value.Any().(UnivariateStatisticsValue)
		if !ok {
			return Missing()
		}
		return Present(selectValue(result))
	})
}

// Rate returns the number of events per second observed during the supplied
// interval. It uses the aggregate's ever-seen event points, which keeps the
// constant-interval form correct even when the source also has a data window.
// An optional predicate is applied to the event points, matching Esper's
// named-filter form. Event timestamps come from the engine's injected clock,
// so the result is deterministic in virtual-time tests.
func Rate(interval time.Duration, predicate ...Expression[bool]) AggregateExpression[float64] {
	children := make([]*exprNode, 0, len(predicate))
	for _, item := range predicate {
		if item == nil {
			children = append(children, nil)
		} else {
			children = append(children, item.node())
		}
	}
	description := fmt.Sprintf("rate(%s)", interval)
	if len(predicate) == 1 && predicate[0] != nil {
		description += ",filter:" + predicate[0].Description()
	}
	return makeAggregateExpr[float64]("rate", description, children, func(ctx EvalContext) Value {
		if interval <= 0 || len(predicate) > 1 || (len(predicate) == 1 && predicate[0] == nil) {
			return Missing()
		}
		cutoff := ctx.Now.Add(-interval)
		events := ctx.EverGroup
		if events == nil {
			events = ctx.Group
		}
		var count int64
		hasLeave := false
		for _, event := range events {
			if !ratePredicateMatches(predicate, event, ctx) {
				continue
			}
			if event.ReceivedAt().After(cutoff) {
				count++
			} else {
				hasLeave = true
			}
		}
		if !hasLeave {
			return Null()
		}
		return Present(float64(count) / interval.Seconds())
	})
}

func ratePredicateMatches(predicate []Expression[bool], event Event, ctx EvalContext) bool {
	if len(predicate) == 0 {
		return true
	}
	value := predicate[0].eval(EvalContext{Event: event, Now: ctx.Now, Variables: ctx.Variables, Parameters: ctx.Parameters})
	matched, ok := boolValue(value)
	return ok && matched
}

// RateByTimestamp computes an arrival rate from a numeric event timestamp in
// the current data window. The rate becomes available after the first
// matching event leaves the window, which avoids presenting a zero-width
// interval as a meaningful rate. An optional predicate limits both the
// retained quantity and the leaving timestamp, matching Esper's filter form.
func RateByTimestamp[T Numeric](timestamp Expression[T], predicate ...Expression[bool]) AggregateExpression[float64] {
	var filter Expression[bool]
	if len(predicate) == 1 {
		filter = predicate[0]
	}
	return rateByTimestampAggregate("rate-timestamp", timestamp, nil, filter, len(predicate), false)
}

// RateQuantityByTimestamp computes a quantity-per-second rate using the
// supplied numeric quantity expression and event timestamp expression.
func RateQuantityByTimestamp[T Numeric, Q Numeric](timestamp Expression[T], quantity Expression[Q], predicate ...Expression[bool]) AggregateExpression[float64] {
	var filter Expression[bool]
	if len(predicate) == 1 {
		filter = predicate[0]
	}
	return rateByTimestampAggregate("rate-quantity-timestamp", timestamp, quantity, filter, len(predicate), true)
}

func rateByTimestampAggregate(kind string, timestamp, quantity Expr, predicate Expression[bool], predicateCount int, quantityRequired bool) AggregateExpression[float64] {
	children := make([]*exprNode, 0, 3)
	if timestamp != nil {
		children = append(children, timestamp.node())
	} else {
		children = append(children, nil)
	}
	if quantityRequired {
		if quantity != nil {
			children = append(children, quantity.node())
		} else {
			children = append(children, nil)
		}
	}
	for index := 0; index < predicateCount; index++ {
		if index == 0 && predicate != nil {
			children = append(children, predicate.node())
		} else {
			children = append(children, nil)
		}
	}
	description := kind + "(<invalid>)"
	valid := timestamp != nil && (!quantityRequired || quantity != nil) && predicateCount <= 1 && (predicateCount == 0 || predicate != nil)
	if valid {
		description = kind + "(" + timestamp.Description()
		if quantity != nil {
			description += "," + quantity.Description()
		}
		if predicate != nil {
			description += ",filter:" + predicate.Description()
		}
		description += ")"
	}
	return makeAggregateExpr[float64](kind, description, children, func(ctx EvalContext) Value {
		if !valid {
			return Missing()
		}
		matches := func(event Event) bool {
			if predicate == nil {
				return true
			}
			value := predicate.eval(EvalContext{Event: event, Now: ctx.Now, Variables: ctx.Variables, Parameters: ctx.Parameters})
			matched, ok := boolValue(value)
			return ok && matched
		}
		numericTimestamp := func(event Event) (int64, bool) {
			value, ok := numericValue(timestamp.eval(EvalContext{Event: event, Now: ctx.Now, Variables: ctx.Variables, Parameters: ctx.Parameters}))
			return int64(value), ok
		}
		var latest int64
		var total float64
		latestSet := false
		for index, event := range ctx.Group {
			if !matches(event) {
				continue
			}
			timestampValue, ok := numericTimestamp(event)
			if !ok {
				continue
			}
			if quantity == nil {
				total++
			} else {
				value, numeric := numericValue(quantity.eval(ctx.groupEventContext(event, index)))
				if !numeric {
					continue
				}
				total += value
			}
			latest = timestampValue
			latestSet = true
		}
		if !latestSet {
			return Null()
		}
		var oldest int64
		oldestSet := false
		for _, event := range ctx.LeavingEvents {
			if !matches(event) {
				continue
			}
			timestampValue, ok := numericTimestamp(event)
			if !ok {
				continue
			}
			oldest = timestampValue
			oldestSet = true
		}
		if !oldestSet || latest <= oldest {
			return Null()
		}
		return Present(total * 1000 / float64(latest-oldest))
	})
}

// MinBy returns the value expression from the row with the smallest key.
func MinBy[V any, K Ordered](value Expression[V], key Expression[K]) AggregateExpression[V] {
	return aggregateBy[V, K]("min-by", value, key, true)
}

// MaxBy returns the value expression from the row with the largest key.
func MaxBy[V any, K Ordered](value Expression[V], key Expression[K]) AggregateExpression[V] {
	return aggregateBy[V, K]("max-by", value, key, false)
}

// MinByEver and MaxByEver keep the selected value over all events accepted by
// the group, including events that have left the current data window.
func MinByEver[V any, K Ordered](value Expression[V], key Expression[K]) AggregateExpression[V] {
	return aggregateByEver[V, K]("min-by-ever", value, key, true)
}

func MaxByEver[V any, K Ordered](value Expression[V], key Expression[K]) AggregateExpression[V] {
	return aggregateByEver[V, K]("max-by-ever", value, key, false)
}

func aggregateBy[V any, K Ordered](kind string, value Expression[V], key Expression[K], minimum bool) AggregateExpression[V] {
	return aggregateBySource[V, K](kind, value, key, minimum, false)
}

func aggregateByEver[V any, K Ordered](kind string, value Expression[V], key Expression[K], minimum bool) AggregateExpression[V] {
	return aggregateBySource[V, K](kind, value, key, minimum, true)
}

func aggregateBySource[V any, K Ordered](kind string, value Expression[V], key Expression[K], minimum, ever bool) AggregateExpression[V] {
	if value == nil || key == nil {
		return invalidAggregate[V](kind, value, kind+"(<invalid>)")
	}
	return makeAggregateExpr[V](kind, kind+"("+value.Description()+","+key.Description()+")", []*exprNode{value.node(), key.node()}, func(ctx EvalContext) Value {
		var selected Value
		var selectedKey Value
		events := ctx.Group
		if ever {
			events = ctx.EverGroup
		}
		for _, event := range events {
			evalContext := EvalContext{Event: event, Now: ctx.Now, Variables: ctx.Variables, Parameters: ctx.Parameters}
			candidate := value.eval(evalContext)
			candidateKey := key.eval(evalContext)
			if !candidate.IsPresent() || !candidateKey.IsPresent() {
				continue
			}
			if !selected.IsPresent() {
				selected, selectedKey = candidate, candidateKey
				continue
			}
			comparison, ok := compareValues(candidateKey, selectedKey)
			if !ok {
				continue
			}
			if (minimum && comparison < 0) || (!minimum && comparison > 0) {
				selected, selectedKey = candidate, candidateKey
			}
		}
		if !selected.IsPresent() {
			return Null()
		}
		return selected
	})
}

// WindowValues returns the non-null values in current group order.
func WindowValues[T any](expression Expression[T]) AggregateExpression[[]T] {
	return makeAggregateExpr[[]T]("window", "window("+expression.Description()+")", []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		if len(ctx.Group) == 0 {
			return Null()
		}
		values := make([]T, 0, len(ctx.Group))
		for index, event := range ctx.Group {
			value := expression.eval(ctx.groupEventContext(event, index))
			if !value.IsPresent() {
				continue
			}
			converted, err := As[T](value)
			if err == nil {
				values = append(values, converted)
			}
		}
		return Present(values)
	})
}

// WindowEvents returns the retained Event values in insertion order. It is
// the explicit Go form of Esper's window(*) access aggregation.
func WindowEvents() AggregateExpression[[]Event] {
	return makeAggregateExpr[[]Event]("window", "window(*)", nil, func(ctx EvalContext) Value {
		if len(ctx.Group) == 0 {
			return Null()
		}
		return Present(append([]Event(nil), ctx.Group...))
	})
}

// SortedEvents returns current group events ordered by one or more analyzable
// sort keys. It is the Go counterpart of sorted(*) with multi-criteria order.
func SortedEvents(keys ...SortKey) AggregateExpression[[]Event] {
	children := make([]*exprNode, 0, len(keys))
	for _, key := range keys {
		if key.Expr != nil {
			children = append(children, key.Expr.node())
		}
	}
	return makeAggregateExpr[[]Event]("sorted", "sorted(*)", children, func(ctx EvalContext) Value {
		if len(ctx.Group) == 0 {
			return Null()
		}
		events := append([]Event(nil), ctx.Group...)
		if len(events) < 2 || len(keys) == 0 {
			return Present(events)
		}
		sort.SliceStable(events, func(left, right int) bool {
			for _, key := range keys {
				if key.Expr == nil {
					continue
				}
				comparison, ok := compareOrderValues(
					key.Expr.eval(EvalContext{Event: events[left], Now: ctx.Now, Variables: ctx.Variables, Parameters: ctx.Parameters}),
					key.Expr.eval(EvalContext{Event: events[right], Now: ctx.Now, Variables: ctx.Variables, Parameters: ctx.Parameters}),
				)
				if !ok || comparison == 0 {
					continue
				}
				if key.Descending {
					return comparison > 0
				}
				return comparison < 0
			}
			return false
		})
		return Present(events)
	})
}

// SortedAccessEntry is one key bucket in a SortedAccessValue. Values sharing
// a key retain their input insertion order.
type SortedAccessEntry[K Ordered, V any] struct {
	Key    K
	Values []V
}

// SortedAccessValue is the immutable, navigable result of SortedAccessBy.
// It intentionally exposes copies so callers cannot mutate aggregate state
// held by a deployed statement.
type SortedAccessValue[K Ordered, V any] struct {
	entries    []SortedAccessEntry[K, V]
	descending bool
}

func (s SortedAccessValue[K, V]) Entries() []SortedAccessEntry[K, V] {
	return cloneSortedAccessEntries(s.entries)
}

func cloneSortedAccessEntry[K Ordered, V any](entry SortedAccessEntry[K, V]) SortedAccessEntry[K, V] {
	return SortedAccessEntry[K, V]{Key: entry.Key, Values: append([]V(nil), entry.Values...)}
}

func cloneSortedAccessEntries[K Ordered, V any](entries []SortedAccessEntry[K, V]) []SortedAccessEntry[K, V] {
	result := make([]SortedAccessEntry[K, V], len(entries))
	for index, entry := range entries {
		result[index] = cloneSortedAccessEntry(entry)
	}
	return result
}

func (s SortedAccessValue[K, V]) Values() []V {
	values := make([]V, 0, s.CountEvents())
	for _, entry := range s.entries {
		values = append(values, entry.Values...)
	}
	return values
}

func (s SortedAccessValue[K, V]) CountEvents() int64 {
	var count int64
	for _, entry := range s.entries {
		count += int64(len(entry.Values))
	}
	return count
}

func (s SortedAccessValue[K, V]) CountKeys() int64 { return int64(len(s.entries)) }

func (s SortedAccessValue[K, V]) FirstKey() (K, bool) {
	if len(s.entries) == 0 {
		var zero K
		return zero, false
	}
	return s.entries[0].Key, true
}

func (s SortedAccessValue[K, V]) LastKey() (K, bool) {
	if len(s.entries) == 0 {
		var zero K
		return zero, false
	}
	return s.entries[len(s.entries)-1].Key, true
}

func (s SortedAccessValue[K, V]) FirstEvent() (V, bool) {
	if len(s.entries) == 0 || len(s.entries[0].Values) == 0 {
		var zero V
		return zero, false
	}
	return s.entries[0].Values[0], true
}

func (s SortedAccessValue[K, V]) LastEvent() (V, bool) {
	if len(s.entries) == 0 {
		var zero V
		return zero, false
	}
	values := s.entries[len(s.entries)-1].Values
	if len(values) == 0 {
		var zero V
		return zero, false
	}
	return values[len(values)-1], true
}

func (s SortedAccessValue[K, V]) FirstEvents() []V {
	if len(s.entries) == 0 {
		return nil
	}
	return append([]V(nil), s.entries[0].Values...)
}

func (s SortedAccessValue[K, V]) LastEvents() []V {
	if len(s.entries) == 0 {
		return nil
	}
	return append([]V(nil), s.entries[len(s.entries)-1].Values...)
}

func (s SortedAccessValue[K, V]) ValuesForKey(key K) []V {
	index, ok := s.findKey(key, sortedAccessExact)
	if !ok {
		return nil
	}
	return append([]V(nil), s.entries[index].Values...)
}

func (s SortedAccessValue[K, V]) ContainsKey(key K) bool {
	_, ok := s.findKey(key, sortedAccessExact)
	return ok
}

// FirstEntry returns a defensive copy of the first key bucket in view order.
func (s SortedAccessValue[K, V]) FirstEntry() (SortedAccessEntry[K, V], bool) {
	if len(s.entries) == 0 {
		return SortedAccessEntry[K, V]{}, false
	}
	return cloneSortedAccessEntry(s.entries[0]), true
}

// LastEntry returns a defensive copy of the last key bucket in view order.
func (s SortedAccessValue[K, V]) LastEntry() (SortedAccessEntry[K, V], bool) {
	if len(s.entries) == 0 {
		return SortedAccessEntry[K, V]{}, false
	}
	return cloneSortedAccessEntry(s.entries[len(s.entries)-1]), true
}

// Entry returns a defensive copy of the exact key bucket, when present.
func (s SortedAccessValue[K, V]) Entry(key K) (SortedAccessEntry[K, V], bool) {
	index, ok := s.findKey(key, sortedAccessExact)
	if !ok {
		return SortedAccessEntry[K, V]{}, false
	}
	return cloneSortedAccessEntry(s.entries[index]), true
}

// Sorted and ListReference expose the detached value sequence used by the
// corresponding aggregate access methods. They are useful when a sorted
// access value has been materialized in a Table and is read through TableField.
func (s SortedAccessValue[K, V]) Sorted() []V { return s.Values() }

func (s SortedAccessValue[K, V]) ListReference() []V { return s.Values() }

// NavigableMapReference returns another detached typed view of this snapshot.
func (s SortedAccessValue[K, V]) NavigableMapReference() SortedAccessValue[K, V] {
	return SortedAccessValue[K, V]{entries: s.Entries(), descending: s.descending}
}

func (s SortedAccessValue[K, V]) GetEvent(key K) (V, bool) {
	return s.eventAt(key, sortedAccessExact, false)
}

func (s SortedAccessValue[K, V]) GetEvents(key K) ([]V, bool) {
	return s.eventsAt(key, sortedAccessExact)
}

func (s SortedAccessValue[K, V]) LowerEvent(key K) (V, bool) {
	return s.eventAt(key, sortedAccessLower, false)
}

func (s SortedAccessValue[K, V]) FloorEvent(key K) (V, bool) {
	return s.eventAt(key, sortedAccessFloor, false)
}

func (s SortedAccessValue[K, V]) HigherEvent(key K) (V, bool) {
	return s.eventAt(key, sortedAccessHigher, false)
}

func (s SortedAccessValue[K, V]) CeilingEvent(key K) (V, bool) {
	return s.eventAt(key, sortedAccessCeiling, false)
}

func (s SortedAccessValue[K, V]) LowerEvents(key K) ([]V, bool) {
	return s.eventsAt(key, sortedAccessLower)
}

func (s SortedAccessValue[K, V]) FloorEvents(key K) ([]V, bool) {
	return s.eventsAt(key, sortedAccessFloor)
}

func (s SortedAccessValue[K, V]) HigherEvents(key K) ([]V, bool) {
	return s.eventsAt(key, sortedAccessHigher)
}

func (s SortedAccessValue[K, V]) CeilingEvents(key K) ([]V, bool) {
	return s.eventsAt(key, sortedAccessCeiling)
}

func (s SortedAccessValue[K, V]) MinBy() (V, bool) { return s.FirstEvent() }

func (s SortedAccessValue[K, V]) MaxBy() (V, bool) { return s.LastEvent() }

// EventsBetween returns all values in the requested inclusive/exclusive key
// range, retaining this view's key order and duplicate-key insertion order.
func (s SortedAccessValue[K, V]) EventsBetween(from K, fromInclusive bool, to K, toInclusive bool) []V {
	return s.SubMap(from, fromInclusive, to, toInclusive).Values()
}

func (s SortedAccessValue[K, V]) eventAt(key K, mode sortedAccessKeyMode, last bool) (V, bool) {
	index, ok := s.findKey(key, mode)
	if !ok || index < 0 || index >= len(s.entries) || len(s.entries[index].Values) == 0 {
		var zero V
		return zero, false
	}
	values := s.entries[index].Values
	if last {
		return values[len(values)-1], true
	}
	return values[0], true
}

func (s SortedAccessValue[K, V]) eventsAt(key K, mode sortedAccessKeyMode) ([]V, bool) {
	index, ok := s.findKey(key, mode)
	if !ok || index < 0 || index >= len(s.entries) {
		return nil, false
	}
	return append([]V(nil), s.entries[index].Values...), true
}

func (s SortedAccessValue[K, V]) IsEmpty() bool { return len(s.entries) == 0 }

// Keys returns keys in this view's iteration order as a defensive copy.
func (s SortedAccessValue[K, V]) Keys() []K {
	keys := make([]K, len(s.entries))
	for index, entry := range s.entries {
		keys[index] = entry.Key
	}
	return keys
}

// Buckets returns the map-style collection of values, preserving view key
// order and insertion order within each duplicate-key bucket.
func (s SortedAccessValue[K, V]) Buckets() [][]V {
	buckets := make([][]V, len(s.entries))
	for index, entry := range s.entries {
		buckets[index] = append([]V(nil), entry.Values...)
	}
	return buckets
}

// Descending returns a detached view with reverse key order. Values within a
// duplicate-key bucket retain their insertion order, matching a Java
// NavigableMap descendingMap view.
func (s SortedAccessValue[K, V]) Descending() SortedAccessValue[K, V] {
	entries := make([]SortedAccessEntry[K, V], len(s.entries))
	for index := range s.entries {
		entries[len(s.entries)-1-index] = cloneSortedAccessEntry(s.entries[index])
	}
	return SortedAccessValue[K, V]{entries: entries, descending: !s.descending}
}

func (s SortedAccessValue[K, V]) HeadMap(to K, inclusive bool) SortedAccessValue[K, V] {
	entries := make([]SortedAccessEntry[K, V], 0, len(s.entries))
	for _, entry := range s.entries {
		comparison, ok := s.orderCompare(entry.Key, to)
		if !ok || comparison > 0 || (comparison == 0 && !inclusive) {
			break
		}
		entries = append(entries, cloneSortedAccessEntry(entry))
	}
	return SortedAccessValue[K, V]{entries: entries, descending: s.descending}
}

func (s SortedAccessValue[K, V]) TailMap(from K, inclusive bool) SortedAccessValue[K, V] {
	entries := make([]SortedAccessEntry[K, V], 0, len(s.entries))
	for _, entry := range s.entries {
		comparison, ok := s.orderCompare(entry.Key, from)
		if !ok {
			continue
		}
		if comparison < 0 || (comparison == 0 && !inclusive) {
			continue
		}
		entries = append(entries, cloneSortedAccessEntry(entry))
	}
	return SortedAccessValue[K, V]{entries: entries, descending: s.descending}
}

func (s SortedAccessValue[K, V]) LowerEntry(key K) (SortedAccessEntry[K, V], bool) {
	return s.navigationEntry(key, sortedAccessLower)
}

func (s SortedAccessValue[K, V]) FloorEntry(key K) (SortedAccessEntry[K, V], bool) {
	return s.navigationEntry(key, sortedAccessFloor)
}

func (s SortedAccessValue[K, V]) HigherEntry(key K) (SortedAccessEntry[K, V], bool) {
	return s.navigationEntry(key, sortedAccessHigher)
}

func (s SortedAccessValue[K, V]) CeilingEntry(key K) (SortedAccessEntry[K, V], bool) {
	return s.navigationEntry(key, sortedAccessCeiling)
}

func (s SortedAccessValue[K, V]) LowerKey(key K) (K, bool) {
	return s.navigationKey(key, sortedAccessLower)
}

func (s SortedAccessValue[K, V]) FloorKey(key K) (K, bool) {
	return s.navigationKey(key, sortedAccessFloor)
}

func (s SortedAccessValue[K, V]) HigherKey(key K) (K, bool) {
	return s.navigationKey(key, sortedAccessHigher)
}

func (s SortedAccessValue[K, V]) CeilingKey(key K) (K, bool) {
	return s.navigationKey(key, sortedAccessCeiling)
}

func (s SortedAccessValue[K, V]) navigationEntry(key K, mode sortedAccessKeyMode) (SortedAccessEntry[K, V], bool) {
	index, ok := s.findKey(key, mode)
	if !ok || index < 0 || index >= len(s.entries) {
		return SortedAccessEntry[K, V]{}, false
	}
	return cloneSortedAccessEntry(s.entries[index]), true
}

func (s SortedAccessValue[K, V]) navigationKey(key K, mode sortedAccessKeyMode) (K, bool) {
	entry, ok := s.navigationEntry(key, mode)
	return entry.Key, ok
}

// SortedAccessIterator is a detached, read-only iterator over key buckets.
// Next returns copies, so advancing or mutating a returned bucket cannot
// change the aggregate snapshot held by a table or statement.
type SortedAccessIterator[K Ordered, V any] struct {
	entries []SortedAccessEntry[K, V]
	index   int
}

func (s SortedAccessValue[K, V]) Iterator() SortedAccessIterator[K, V] {
	return SortedAccessIterator[K, V]{entries: s.Entries()}
}

func (iterator *SortedAccessIterator[K, V]) Next() (SortedAccessEntry[K, V], bool) {
	if iterator == nil || iterator.index >= len(iterator.entries) {
		return SortedAccessEntry[K, V]{}, false
	}
	entry := cloneSortedAccessEntry(iterator.entries[iterator.index])
	iterator.index++
	return entry, true
}

func (s SortedAccessValue[K, V]) SubMap(from K, fromInclusive bool, to K, toInclusive bool) SortedAccessValue[K, V] {
	result := SortedAccessValue[K, V]{entries: make([]SortedAccessEntry[K, V], 0, len(s.entries))}
	for _, entry := range s.entries {
		lower, lowerOK := s.orderCompare(entry.Key, from)
		upper, upperOK := s.orderCompare(entry.Key, to)
		if !lowerOK || !upperOK {
			continue
		}
		if lower < 0 || (lower == 0 && !fromInclusive) || upper > 0 || (upper == 0 && !toInclusive) {
			continue
		}
		result.entries = append(result.entries, cloneSortedAccessEntry(entry))
	}
	result.descending = s.descending
	return result
}

func (s SortedAccessValue[K, V]) orderCompare(left, right K) (int, bool) {
	comparison, ok := compareValues(Present(left), Present(right))
	if s.descending {
		comparison = -comparison
	}
	return comparison, ok
}

func (s SortedAccessValue[K, V]) findKey(key K, mode sortedAccessKeyMode) (int, bool) {
	best := -1
	for index, entry := range s.entries {
		comparison, ok := s.orderCompare(entry.Key, key)
		if !ok {
			continue
		}
		switch mode {
		case sortedAccessExact:
			if comparison == 0 {
				return index, true
			}
		case sortedAccessLower:
			if comparison < 0 {
				best = s.preferNavigationCandidate(best, index, s.descending)
			}
		case sortedAccessFloor:
			if comparison <= 0 {
				best = s.preferNavigationCandidate(best, index, s.descending)
			}
		case sortedAccessHigher:
			if comparison > 0 {
				best = s.preferNavigationCandidate(best, index, !s.descending)
			}
		case sortedAccessCeiling:
			if comparison >= 0 {
				best = s.preferNavigationCandidate(best, index, !s.descending)
			}
		}
	}
	return best, best >= 0
}

func (s SortedAccessValue[K, V]) preferNavigationCandidate(best, candidate int, preferLowerNatural bool) int {
	if best < 0 {
		return candidate
	}
	comparison, ok := compareValues(Present(s.entries[candidate].Key), Present(s.entries[best].Key))
	if !ok {
		return best
	}
	if preferLowerNatural {
		if comparison < 0 {
			return candidate
		}
		return best
	}
	if comparison > 0 {
		return candidate
	}
	return best
}

type sortedAccessKeyMode uint8

const (
	sortedAccessExact sortedAccessKeyMode = iota
	sortedAccessLower
	sortedAccessFloor
	sortedAccessHigher
	sortedAccessCeiling
)

// SortedAccessExpression is the chainable Go representation of Esper's
// sorted(...) access aggregation. Its methods return analyzable aggregate
// expressions, so access operations can be selected, filtered by HAVING, or
// composed with other result expressions without EPL strings.
type SortedAccessExpression[V any, K Ordered] struct {
	aggregateExpr[SortedAccessValue[K, V]]
}

// SortedAccessBy creates a sorted access aggregate over value and key. The
// key is ordered ascending and duplicate keys form insertion-ordered buckets.
func SortedAccessBy[V any, K Ordered](value Expression[V], key Expression[K]) SortedAccessExpression[V, K] {
	children := make([]*exprNode, 0, 2)
	description := "sorted-access(<invalid>)"
	if value != nil {
		children = append(children, value.node())
	}
	if key != nil {
		children = append(children, key.node())
	}
	if value != nil && key != nil {
		description = "sorted-access(" + value.Description() + "," + key.Description() + ")"
	}
	return SortedAccessExpression[V, K]{aggregateExpr: aggregateExpr[SortedAccessValue[K, V]]{typedExpr: typedExpr[SortedAccessValue[K, V]]{
		n: &exprNode{kind: "sorted-access", typ: typeOf[SortedAccessValue[K, V]](), description: description, children: children},
		fn: func(ctx EvalContext) Value {
			if value == nil || key == nil {
				return Missing()
			}
			return Present(buildSortedAccessValue[V, K](ctx, value, key))
		},
	}}}
}

// SortedAccessByMulti creates a sorted access aggregate with two ordered key
// expressions. Keys compare lexicographically, matching Esper's
// HashableMultiKey/TreeMap behavior while keeping the rule fully typed and
// analyzable in Go.
func SortedAccessByMulti[V any, A Ordered, B Ordered](value Expression[V], first Expression[A], second Expression[B]) SortedAccessExpression[V, SortedMultiKey] {
	key := sortedMultiKeyExpression[A, B](first, second)
	return SortedAccessBy[V, SortedMultiKey](value, key)
}

func sortedMultiKeyExpression[A Ordered, B Ordered](first Expression[A], second Expression[B]) Expression[SortedMultiKey] {
	children := make([]*exprNode, 0, 2)
	if first != nil {
		children = append(children, first.node())
	} else {
		children = append(children, nil)
	}
	if second != nil {
		children = append(children, second.node())
	} else {
		children = append(children, nil)
	}
	description := "sorted-multi-key(<invalid>)"
	if first != nil && second != nil {
		description = "sorted-multi-key(" + first.Description() + "," + second.Description() + ")"
	}
	node := &exprNode{kind: "sorted-multi-key", typ: typeOf[SortedMultiKey](), description: description, children: children}
	if first == nil || second == nil {
		node.configurationError = "sorted multi-key requires two key expressions"
	}
	return typedExpr[SortedMultiKey]{n: node, fn: func(ctx EvalContext) Value {
		if first == nil || second == nil {
			return Missing()
		}
		firstValue := first.eval(ctx)
		secondValue := second.eval(ctx)
		if !firstValue.IsPresent() || !secondValue.IsPresent() {
			return Null()
		}
		return Present(NewSortedMultiKey(firstValue, secondValue))
	}}
}

func buildSortedAccessValue[V any, K Ordered](ctx EvalContext, value Expression[V], key Expression[K]) SortedAccessValue[K, V] {
	entries := make([]SortedAccessEntry[K, V], 0)
	for index, event := range ctx.Group {
		evalContext := ctx.groupEventContext(event, index)
		keyValue := key.eval(evalContext)
		valueValue := value.eval(evalContext)
		if !keyValue.IsPresent() || !valueValue.IsPresent() {
			continue
		}
		convertedKey, keyErr := As[K](keyValue)
		convertedValue, valueErr := As[V](valueValue)
		if keyErr != nil || valueErr != nil {
			continue
		}
		found := false
		for index := range entries {
			comparison, ok := compareValues(Present(entries[index].Key), Present(convertedKey))
			if ok && comparison == 0 {
				entries[index].Values = append(entries[index].Values, convertedValue)
				found = true
				break
			}
		}
		if !found {
			entries = append(entries, SortedAccessEntry[K, V]{Key: convertedKey, Values: []V{convertedValue}})
		}
	}
	sort.SliceStable(entries, func(left, right int) bool {
		comparison, ok := compareValues(Present(entries[left].Key), Present(entries[right].Key))
		return ok && comparison < 0
	})
	return SortedAccessValue[K, V]{entries: entries}
}

func (s SortedAccessExpression[V, K]) access(ctx EvalContext) (SortedAccessValue[K, V], bool) {
	value := s.eval(ctx)
	if !value.IsPresent() {
		return SortedAccessValue[K, V]{}, false
	}
	result, ok := value.Any().(SortedAccessValue[K, V])
	return result, ok
}

func sortedAccessMethod[V any, K Ordered, T any](access SortedAccessExpression[V, K], kind, description string, lookups []Expr, fn func(SortedAccessValue[K, V], []Value) Value) AggregateExpression[T] {
	children := []*exprNode{access.node()}
	for _, lookup := range lookups {
		if lookup == nil {
			return invalidAggregate[T](kind, access, description)
		}
		children = append(children, lookup.node())
	}
	return makeAggregateExpr[T](kind, description, children, func(ctx EvalContext) Value {
		value, ok := access.access(ctx)
		if !ok {
			return Null()
		}
		lookupValues := make([]Value, 0, len(lookups))
		for _, lookup := range lookups {
			lookupValues = append(lookupValues, lookup.eval(ctx))
		}
		return fn(value, lookupValues)
	})
}

func sortedAccessKey[T any, K Ordered](values []Value) (K, bool) {
	if len(values) == 0 || !values[0].IsPresent() {
		var zero K
		return zero, false
	}
	key, err := As[K](values[0])
	return key, err == nil
}

func sortedAccessValueAt[V any, K Ordered](access SortedAccessValue[K, V], values []Value, mode sortedAccessKeyMode, last bool) Value {
	key, ok := sortedAccessKey[V, K](values)
	if !ok {
		return Null()
	}
	index, ok := access.findKey(key, mode)
	if !ok || index < 0 || index >= len(access.entries) || len(access.entries[index].Values) == 0 {
		return Null()
	}
	items := access.entries[index].Values
	if last {
		return Present(items[len(items)-1])
	}
	return Present(items[0])
}

func sortedAccessEventsAt[V any, K Ordered](access SortedAccessValue[K, V], values []Value, mode sortedAccessKeyMode) Value {
	key, ok := sortedAccessKey[V, K](values)
	if !ok {
		return Null()
	}
	index, ok := access.findKey(key, mode)
	if !ok || index < 0 || index >= len(access.entries) {
		return Null()
	}
	return Present(append([]V(nil), access.entries[index].Values...))
}

func (s SortedAccessExpression[V, K]) Sorted() AggregateExpression[[]V] {
	return sortedAccessMethod[V, K, []V](s, "sorted-access-values", "sorted-access.values()", nil, func(access SortedAccessValue[K, V], _ []Value) Value {
		return Present(access.Values())
	})
}

func (s SortedAccessExpression[V, K]) ListReference() AggregateExpression[[]V] {
	return s.Sorted()
}

func (s SortedAccessExpression[V, K]) NavigableMapReference() AggregateExpression[SortedAccessValue[K, V]] {
	return sortedAccessMethod[V, K, SortedAccessValue[K, V]](s, "sorted-access-map", "sorted-access.navigable-map()", nil, func(access SortedAccessValue[K, V], _ []Value) Value {
		return Present(SortedAccessValue[K, V]{entries: access.Entries()})
	})
}

func (s SortedAccessExpression[V, K]) FirstKey() AggregateExpression[K] {
	return sortedAccessMethod[V, K, K](s, "sorted-access-first-key", "sorted-access.first-key()", nil, func(access SortedAccessValue[K, V], _ []Value) Value {
		key, ok := access.FirstKey()
		if !ok {
			return Null()
		}
		return Present(key)
	})
}

func (s SortedAccessExpression[V, K]) LastKey() AggregateExpression[K] {
	return sortedAccessMethod[V, K, K](s, "sorted-access-last-key", "sorted-access.last-key()", nil, func(access SortedAccessValue[K, V], _ []Value) Value {
		key, ok := access.LastKey()
		if !ok {
			return Null()
		}
		return Present(key)
	})
}

func (s SortedAccessExpression[V, K]) FirstEvent() AggregateExpression[V] {
	return sortedAccessMethod[V, K, V](s, "sorted-access-first-event", "sorted-access.first-event()", nil, func(access SortedAccessValue[K, V], _ []Value) Value {
		value, ok := access.FirstEvent()
		if !ok {
			return Null()
		}
		return Present(value)
	})
}

func (s SortedAccessExpression[V, K]) LastEvent() AggregateExpression[V] {
	return sortedAccessMethod[V, K, V](s, "sorted-access-last-event", "sorted-access.last-event()", nil, func(access SortedAccessValue[K, V], _ []Value) Value {
		value, ok := access.LastEvent()
		if !ok {
			return Null()
		}
		return Present(value)
	})
}

func (s SortedAccessExpression[V, K]) FirstEvents() AggregateExpression[[]V] {
	return sortedAccessMethod[V, K, []V](s, "sorted-access-first-events", "sorted-access.first-events()", nil, func(access SortedAccessValue[K, V], _ []Value) Value {
		return Present(access.FirstEvents())
	})
}

func (s SortedAccessExpression[V, K]) LastEvents() AggregateExpression[[]V] {
	return sortedAccessMethod[V, K, []V](s, "sorted-access-last-events", "sorted-access.last-events()", nil, func(access SortedAccessValue[K, V], _ []Value) Value {
		return Present(access.LastEvents())
	})
}

func (s SortedAccessExpression[V, K]) MinBy() AggregateExpression[V] { return s.FirstEvent() }
func (s SortedAccessExpression[V, K]) MaxBy() AggregateExpression[V] { return s.LastEvent() }

func (s SortedAccessExpression[V, K]) GetEvent(key Expression[K]) AggregateExpression[V] {
	return sortedAccessMethod[V, K, V](s, "sorted-access-get-event", "sorted-access.get-event()", []Expr{key}, func(access SortedAccessValue[K, V], values []Value) Value {
		return sortedAccessValueAt[V, K](access, values, sortedAccessExact, false)
	})
}

func (s SortedAccessExpression[V, K]) GetEvents(key Expression[K]) AggregateExpression[[]V] {
	return sortedAccessMethod[V, K, []V](s, "sorted-access-get-events", "sorted-access.get-events()", []Expr{key}, func(access SortedAccessValue[K, V], values []Value) Value {
		return sortedAccessEventsAt[V, K](access, values, sortedAccessExact)
	})
}

func (s SortedAccessExpression[V, K]) Contains(key Expression[K]) AggregateExpression[bool] {
	return sortedAccessMethod[V, K, bool](s, "sorted-access-contains-key", "sorted-access.contains-key()", []Expr{key}, func(access SortedAccessValue[K, V], values []Value) Value {
		key, ok := sortedAccessKey[V, K](values)
		return Present(ok && access.ContainsKey(key))
	})
}

func (s SortedAccessExpression[V, K]) CountEvents() AggregateExpression[int64] {
	return sortedAccessMethod[V, K, int64](s, "sorted-access-count-events", "sorted-access.count-events()", nil, func(access SortedAccessValue[K, V], _ []Value) Value {
		return Present(access.CountEvents())
	})
}

func (s SortedAccessExpression[V, K]) CountKeys() AggregateExpression[int64] {
	return sortedAccessMethod[V, K, int64](s, "sorted-access-count-keys", "sorted-access.count-keys()", nil, func(access SortedAccessValue[K, V], _ []Value) Value {
		return Present(access.CountKeys())
	})
}

func (s SortedAccessExpression[V, K]) LowerKey(key Expression[K]) AggregateExpression[K] {
	return sortedAccessMethod[V, K, K](s, "sorted-access-lower-key", "sorted-access.lower-key()", []Expr{key}, func(access SortedAccessValue[K, V], values []Value) Value {
		return sortedAccessNearestKey(access, values, sortedAccessLower)
	})
}

func (s SortedAccessExpression[V, K]) FloorKey(key Expression[K]) AggregateExpression[K] {
	return sortedAccessMethod[V, K, K](s, "sorted-access-floor-key", "sorted-access.floor-key()", []Expr{key}, func(access SortedAccessValue[K, V], values []Value) Value {
		return sortedAccessNearestKey(access, values, sortedAccessFloor)
	})
}

func (s SortedAccessExpression[V, K]) HigherKey(key Expression[K]) AggregateExpression[K] {
	return sortedAccessMethod[V, K, K](s, "sorted-access-higher-key", "sorted-access.higher-key()", []Expr{key}, func(access SortedAccessValue[K, V], values []Value) Value {
		return sortedAccessNearestKey(access, values, sortedAccessHigher)
	})
}

func (s SortedAccessExpression[V, K]) CeilingKey(key Expression[K]) AggregateExpression[K] {
	return sortedAccessMethod[V, K, K](s, "sorted-access-ceiling-key", "sorted-access.ceiling-key()", []Expr{key}, func(access SortedAccessValue[K, V], values []Value) Value {
		return sortedAccessNearestKey(access, values, sortedAccessCeiling)
	})
}

func sortedAccessNearestKey[V any, K Ordered](access SortedAccessValue[K, V], values []Value, mode sortedAccessKeyMode) Value {
	key, ok := sortedAccessKey[V, K](values)
	if !ok {
		return Null()
	}
	index, ok := access.findKey(key, mode)
	if !ok {
		return Null()
	}
	return Present(access.entries[index].Key)
}

func (s SortedAccessExpression[V, K]) LowerEvent(key Expression[K]) AggregateExpression[V] {
	return sortedAccessMethod[V, K, V](s, "sorted-access-lower-event", "sorted-access.lower-event()", []Expr{key}, func(access SortedAccessValue[K, V], values []Value) Value {
		return sortedAccessValueAt[V, K](access, values, sortedAccessLower, false)
	})
}

func (s SortedAccessExpression[V, K]) FloorEvent(key Expression[K]) AggregateExpression[V] {
	return sortedAccessMethod[V, K, V](s, "sorted-access-floor-event", "sorted-access.floor-event()", []Expr{key}, func(access SortedAccessValue[K, V], values []Value) Value {
		return sortedAccessValueAt[V, K](access, values, sortedAccessFloor, false)
	})
}

func (s SortedAccessExpression[V, K]) HigherEvent(key Expression[K]) AggregateExpression[V] {
	return sortedAccessMethod[V, K, V](s, "sorted-access-higher-event", "sorted-access.higher-event()", []Expr{key}, func(access SortedAccessValue[K, V], values []Value) Value {
		return sortedAccessValueAt[V, K](access, values, sortedAccessHigher, false)
	})
}

func (s SortedAccessExpression[V, K]) CeilingEvent(key Expression[K]) AggregateExpression[V] {
	return sortedAccessMethod[V, K, V](s, "sorted-access-ceiling-event", "sorted-access.ceiling-event()", []Expr{key}, func(access SortedAccessValue[K, V], values []Value) Value {
		return sortedAccessValueAt[V, K](access, values, sortedAccessCeiling, false)
	})
}

func (s SortedAccessExpression[V, K]) LowerEvents(key Expression[K]) AggregateExpression[[]V] {
	return sortedAccessMethod[V, K, []V](s, "sorted-access-lower-events", "sorted-access.lower-events()", []Expr{key}, func(access SortedAccessValue[K, V], values []Value) Value {
		return sortedAccessEventsAt[V, K](access, values, sortedAccessLower)
	})
}

func (s SortedAccessExpression[V, K]) FloorEvents(key Expression[K]) AggregateExpression[[]V] {
	return sortedAccessMethod[V, K, []V](s, "sorted-access-floor-events", "sorted-access.floor-events()", []Expr{key}, func(access SortedAccessValue[K, V], values []Value) Value {
		return sortedAccessEventsAt[V, K](access, values, sortedAccessFloor)
	})
}

func (s SortedAccessExpression[V, K]) HigherEvents(key Expression[K]) AggregateExpression[[]V] {
	return sortedAccessMethod[V, K, []V](s, "sorted-access-higher-events", "sorted-access.higher-events()", []Expr{key}, func(access SortedAccessValue[K, V], values []Value) Value {
		return sortedAccessEventsAt[V, K](access, values, sortedAccessHigher)
	})
}

func (s SortedAccessExpression[V, K]) CeilingEvents(key Expression[K]) AggregateExpression[[]V] {
	return sortedAccessMethod[V, K, []V](s, "sorted-access-ceiling-events", "sorted-access.ceiling-events()", []Expr{key}, func(access SortedAccessValue[K, V], values []Value) Value {
		return sortedAccessEventsAt[V, K](access, values, sortedAccessCeiling)
	})
}

func (s SortedAccessExpression[V, K]) EventsBetween(from Expression[K], fromInclusive bool, to Expression[K], toInclusive bool) AggregateExpression[[]V] {
	return sortedAccessMethod[V, K, []V](s, "sorted-access-events-between", "sorted-access.events-between()", []Expr{from, to}, func(access SortedAccessValue[K, V], values []Value) Value {
		fromKey, fromOK := sortedAccessKey[V, K]([]Value{values[0]})
		toKey, toOK := sortedAccessKey[V, K]([]Value{values[1]})
		if !fromOK || !toOK {
			return Present([]V(nil))
		}
		return Present(access.SubMap(fromKey, fromInclusive, toKey, toInclusive).Values())
	})
}

func (s SortedAccessExpression[V, K]) SubMap(from Expression[K], fromInclusive bool, to Expression[K], toInclusive bool) AggregateExpression[SortedAccessValue[K, V]] {
	return sortedAccessMethod[V, K, SortedAccessValue[K, V]](s, "sorted-access-submap", "sorted-access.submap()", []Expr{from, to}, func(access SortedAccessValue[K, V], values []Value) Value {
		fromKey, fromOK := sortedAccessKey[V, K]([]Value{values[0]})
		toKey, toOK := sortedAccessKey[V, K]([]Value{values[1]})
		if !fromOK || !toOK {
			return Present(SortedAccessValue[K, V]{})
		}
		return Present(access.SubMap(fromKey, fromInclusive, toKey, toInclusive))
	})
}

// WindowAccessValue is the immutable insertion-ordered value produced by
// WindowAccessBy. It is useful when a window access aggregate is persisted in
// a Go Table or passed through a sink.
type WindowAccessValue[V any] struct {
	values []V
}

func (w WindowAccessValue[V]) Values() []V {
	// Keep an empty access result as a present, non-nil slice. A typed nil
	// slice would be converted to Null by the expression value model, while
	// Esper's window(*) access returns an empty collection for zero matches.
	values := make([]V, len(w.values))
	copy(values, w.values)
	return values
}
func (w WindowAccessValue[V]) CountEvents() int64 {
	return int64(len(w.values))
}
func (w WindowAccessValue[V]) First() (V, bool) {
	if len(w.values) == 0 {
		var zero V
		return zero, false
	}
	return w.values[0], true
}
func (w WindowAccessValue[V]) Last() (V, bool) {
	if len(w.values) == 0 {
		var zero V
		return zero, false
	}
	return w.values[len(w.values)-1], true
}

// WindowAccessExpression is the chainable Go representation of window(*).
// Its methods return analyzable aggregate expressions and preserve insertion
// order after filter/window eviction.
type WindowAccessExpression[V any] struct {
	aggregateExpr[WindowAccessValue[V]]
}

func WindowAccessBy[V any](expression Expression[V]) WindowAccessExpression[V] {
	children := []*exprNode(nil)
	description := "window-access(<invalid>)"
	if expression != nil {
		children = []*exprNode{expression.node()}
		description = "window-access(" + expression.Description() + ")"
	}
	return WindowAccessExpression[V]{aggregateExpr: aggregateExpr[WindowAccessValue[V]]{typedExpr: typedExpr[WindowAccessValue[V]]{
		n: &exprNode{kind: "window-access", typ: typeOf[WindowAccessValue[V]](), description: description, children: children},
		fn: func(ctx EvalContext) Value {
			if expression == nil {
				return Missing()
			}
			values := make([]V, 0, len(ctx.Group))
			for index, event := range ctx.Group {
				evalContext := ctx.groupEventContext(event, index)
				value := expression.eval(evalContext)
				if !value.IsPresent() {
					continue
				}
				converted, err := As[V](value)
				if err == nil {
					values = append(values, converted)
				}
			}
			return Present(WindowAccessValue[V]{values: values})
		},
	}}}
}

func (w WindowAccessExpression[V]) access(ctx EvalContext) (WindowAccessValue[V], bool) {
	value := w.eval(ctx)
	if !value.IsPresent() {
		return WindowAccessValue[V]{}, false
	}
	result, ok := value.Any().(WindowAccessValue[V])
	return result, ok
}

func windowAccessMethod[V any, T any](access WindowAccessExpression[V], kind, description string, fn func(WindowAccessValue[V]) Value) AggregateExpression[T] {
	return makeAggregateExpr[T](kind, description, []*exprNode{access.node()}, func(ctx EvalContext) Value {
		value, ok := access.access(ctx)
		if !ok {
			return Null()
		}
		return fn(value)
	})
}

func (w WindowAccessExpression[V]) Values() AggregateExpression[[]V] {
	return windowAccessMethod[V, []V](w, "window-access-values", "window-access.values()", func(value WindowAccessValue[V]) Value {
		return Present(value.Values())
	})
}

func (w WindowAccessExpression[V]) ListReference() AggregateExpression[[]V] { return w.Values() }

func (w WindowAccessExpression[V]) First() AggregateExpression[V] {
	return windowAccessMethod[V, V](w, "window-access-first", "window-access.first()", func(value WindowAccessValue[V]) Value {
		item, ok := value.First()
		if !ok {
			return Null()
		}
		return Present(item)
	})
}

func (w WindowAccessExpression[V]) Last() AggregateExpression[V] {
	return windowAccessMethod[V, V](w, "window-access-last", "window-access.last()", func(value WindowAccessValue[V]) Value {
		item, ok := value.Last()
		if !ok {
			return Null()
		}
		return Present(item)
	})
}

func (w WindowAccessExpression[V]) CountEvents() AggregateExpression[int64] {
	return windowAccessMethod[V, int64](w, "window-access-count", "window-access.count-events()", func(value WindowAccessValue[V]) Value {
		return Present(value.CountEvents())
	})
}

// SetOfValues returns distinct non-null values in first-seen order.
func SetOfValues[T comparable](expression Expression[T]) AggregateExpression[[]T] {
	return makeAggregateExpr[[]T]("set", "set("+expression.Description()+")", []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		values := make([]T, 0, len(ctx.Group))
		seen := make(map[T]struct{})
		for index, event := range ctx.Group {
			value := expression.eval(ctx.groupEventContext(event, index))
			if !value.IsPresent() {
				continue
			}
			converted, err := As[T](value)
			if err != nil {
				continue
			}
			if _, exists := seen[converted]; exists {
				continue
			}
			seen[converted] = struct{}{}
			values = append(values, converted)
		}
		return Present(values)
	})
}

// SortedValues returns non-null values ordered by their natural Go ordering.
func SortedValues[T Ordered](expression Expression[T], descending bool) AggregateExpression[[]T] {
	return makeAggregateExpr[[]T]("sorted", fmt.Sprintf("sorted(%s,%t)", expression.Description(), descending), []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		values := make([]T, 0, len(ctx.Group))
		for index, event := range ctx.Group {
			value := expression.eval(ctx.groupEventContext(event, index))
			if !value.IsPresent() {
				continue
			}
			converted, err := As[T](value)
			if err == nil {
				values = append(values, converted)
			}
		}
		sort.SliceStable(values, func(left, right int) bool {
			comparison, ok := compareValues(Present(values[left]), Present(values[right]))
			if !ok {
				return false
			}
			if descending {
				return comparison > 0
			}
			return comparison < 0
		})
		return Present(values)
	})
}

func sampleVariance(values []float64) float64 {
	if len(values) <= 1 {
		return 0
	}
	var total float64
	for _, value := range values {
		total += value
	}
	mean := total / float64(len(values))
	var squared float64
	for _, value := range values {
		delta := value - mean
		squared += delta * delta
	}
	return squared / float64(len(values)-1)
}

func populationVariance(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var total float64
	for _, value := range values {
		total += value
	}
	mean := total / float64(len(values))
	var squared float64
	for _, value := range values {
		delta := value - mean
		squared += delta * delta
	}
	return squared / float64(len(values))
}

func numericAggregateValues[T Numeric](expression Expression[T], ctx EvalContext) []float64 {
	values := make([]float64, 0, len(ctx.Group))
	for index, event := range ctx.Group {
		value, ok := numericValue(expression.eval(ctx.groupEventContext(event, index)))
		if ok {
			values = append(values, value)
		}
	}
	return values
}

func aggregateExtreme[T Ordered](kind string, expression Expression[T], minimum bool) AggregateExpression[T] {
	return makeAggregateExpr[T](kind, kind+"("+expression.Description()+")", []*exprNode{expression.node()}, func(ctx EvalContext) Value {
		var result Value
		for index, event := range ctx.Group {
			value := expression.eval(ctx.groupEventContext(event, index))
			if !value.IsPresent() {
				continue
			}
			if !result.IsPresent() {
				result = value
				continue
			}
			comparison, ok := compareValues(value, result)
			if ok && ((minimum && comparison < 0) || (!minimum && comparison > 0)) {
				result = value
			}
		}
		if !result.IsPresent() {
			return Null()
		}
		return result
	})
}

// Selection is a named projection expression.
type Selection struct {
	Name string
	Expr Expr
}

func Alias(name string, expression Expr) Selection {
	return Selection{Name: name, Expr: expression}
}

func (s Selection) description() string {
	if s.Expr == nil {
		return s.Name + "=<nil>"
	}
	return s.Name + "=" + s.Expr.Description()
}

func (n *exprNode) referencedFields(result *[]string) {
	if n == nil {
		return
	}
	if n.kind == "field" || n.kind == "tag-field" || n.kind == "tag-field-at" {
		*result = append(*result, n.fieldName)
	}
	for _, child := range n.children {
		child.referencedFields(result)
	}
}

func (n *exprNode) referencedLocalFields(result *[]string) {
	if n == nil {
		return
	}
	if n.kind == "field" {
		*result = append(*result, n.fieldName)
	}
	for _, child := range n.children {
		child.referencedLocalFields(result)
	}
}

type tagFieldReference struct {
	tag  string
	name string
	typ  reflect.Type
}

func (n *exprNode) referencedTagFields(result *[]tagFieldReference) {
	if n == nil {
		return
	}
	if n.kind == "tag-field" || n.kind == "tag-field-at" {
		*result = append(*result, tagFieldReference{tag: n.tagName, name: n.fieldName, typ: n.typ})
	}
	for _, child := range n.children {
		child.referencedTagFields(result)
	}
}

type containedParentFieldReference struct {
	level int
	name  string
}

func (n *exprNode) referencedContainedParentFields(result *[]containedParentFieldReference) {
	if n == nil {
		return
	}
	if n.kind == "contained-parent-field" || n.kind == "contained-ancestor-field" {
		*result = append(*result, containedParentFieldReference{level: n.containedParentLevels, name: n.fieldName})
	}
	for _, child := range n.children {
		child.referencedContainedParentFields(result)
	}
}

func (n *exprNode) referencedTags(result *[]string) {
	if n == nil {
		return
	}
	if strings.HasPrefix(n.kind, "tag-") {
		if n.tagName != "" {
			*result = append(*result, n.tagName)
		}
	}
	for _, child := range n.children {
		child.referencedTags(result)
	}
}

func (n *exprNode) referencedTargetFields(kind string, result *[]string) {
	if n == nil {
		return
	}
	if n.kind == kind {
		*result = append(*result, n.fieldName)
	}
	for _, child := range n.children {
		child.referencedTargetFields(kind, result)
	}
}

func (n *exprNode) referencedVariables(result *[]string) {
	if n == nil {
		return
	}
	if n.kind == "variable" {
		*result = append(*result, n.variableName)
	}
	for _, child := range n.children {
		child.referencedVariables(result)
	}
}

func (n *exprNode) referencedParameters(result *[]string) {
	if n == nil {
		return
	}
	if n.kind == "parameter" {
		*result = append(*result, n.parameterName)
	}
	for _, child := range n.children {
		child.referencedParameters(result)
	}
}
