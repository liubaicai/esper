package esper

import (
	"fmt"
	"iter"
	"reflect"
	"strings"
	"time"
)

type streamNodeKind uint8

const (
	streamSource streamNodeKind = iota
	streamFilter
	streamWindow
	streamNamedWindow
	streamTable
	streamHistorical
	streamMethod
	streamPattern
	streamDerived
	streamContained
)

type streamNode struct {
	kind               streamNodeKind
	input              *streamNode
	sourceName         string
	sourceType         reflect.Type
	configurationError string
	historical         *historicalDefinition
	method             *methodDefinition
	pattern            *patternDefinition
	derived            *derivedStreamDefinition
	patternWindow      WindowSpec
	predicate          Expr
	window             WindowSpec
	contained          *containedDefinition
}

type containedDefinition struct {
	property         Expr
	childType        reflect.Type
	elementType      reflect.Type
	targetSchemaName string
	sequence         bool
	wrap             func(reflect.Value) (any, error)
}

func (n *streamNode) describe() string {
	if n == nil {
		return "<nil-stream>"
	}
	switch n.kind {
	case streamSource:
		return "from(" + n.sourceName + ")"
	case streamFilter:
		return n.input.describe() + ".filter(" + n.predicate.Description() + ")"
	case streamWindow:
		return n.input.describe() + ".window(" + n.window.description() + ")"
	case streamNamedWindow:
		return "named-window(" + n.sourceName + ")"
	case streamTable:
		return "table-source(" + n.sourceName + ")"
	case streamHistorical:
		schemaName := "<nil>"
		triggerName := ""
		if n.historical != nil && n.historical.schema.valid() {
			schemaName = n.historical.schema.Name()
		}
		if n.historical != nil {
			triggerName = n.historical.trigger
		}
		return "historical(" + n.sourceName + ":" + schemaName + ":trigger=" + triggerName + ")"
	case streamMethod:
		schemaName := "<nil>"
		triggerName := ""
		dependencies := ""
		if n.method != nil && n.method.schema.valid() {
			schemaName = n.method.schema.Name()
		}
		if n.method != nil {
			triggerName = n.method.trigger
			dependencies = strings.Join(n.method.dependencies, ",")
		}
		return "method(" + n.sourceName + ":" + schemaName + ":trigger=" + triggerName + ":depends=" + dependencies + ")"
	case streamPattern:
		description := "pattern-source(" + n.sourceName
		if n.pattern != nil {
			description += ":" + n.pattern.description()
		}
		if n.patternWindow != nil {
			description += ":window=" + n.patternWindow.description()
		}
		return description + ")"
	case streamDerived:
		description := "derived-source(<nil>)"
		if n.derived != nil && n.derived.aggregate != nil {
			parts := make([]string, 0, len(n.derived.aggregate.selections))
			for _, selection := range n.derived.aggregate.selections {
				parts = append(parts, selection.description())
			}
			input := "<nil>"
			if n.input != nil {
				input = n.input.describe()
			}
			description = "derived-source(" + input + ":group=" + describeExprList(n.derived.aggregate.groupBy) + ":select=" + strings.Join(parts, ",") + ")"
		}
		return description
	case streamContained:
		property := "<nil>"
		if n.contained != nil && n.contained.property != nil {
			property = n.contained.property.Description()
		}
		target := ""
		if n.contained != nil {
			target = n.contained.targetSchemaName
		}
		input := "<nil>"
		if n.input != nil {
			input = n.input.describe()
		}
		if target != "" {
			kind := "type=" + target
			if n.contained != nil && n.contained.sequence {
				kind += ":sequence"
			}
			return "unnest(" + input + ":" + property + ":" + kind + ")"
		}
		return "unnest(" + input + ":" + property + ")"
	default:
		return "<unknown-stream>"
	}
}

// Stream[T] is the typed, type-preserving part of the fluent API.
type Stream[T any] struct {
	env  *Environment
	node *streamNode
}

// RecordStream is the dynamic Row transition used by projections whose result
// type is not known to a Go method receiver.
type RecordStream struct {
	env        *Environment
	node       *streamNode
	selections []Selection
}

type AggregateStream struct {
	env          *Environment
	node         *streamNode
	join         *joinDefinition
	groupBy      []Expr
	grouping     aggregateGroupingMode
	groupingSets [][]Expr
	selections   []Selection
	where        Expression[bool]
	having       Expression[bool]
}

type derivedStreamDefinition struct {
	aggregate *aggregateDefinition
	schema    Schema
}

func describeExprList(expressions []Expr) string {
	parts := make([]string, 0, len(expressions))
	for _, expression := range expressions {
		if expression == nil {
			parts = append(parts, "<nil>")
			continue
		}
		parts = append(parts, expression.Description())
	}
	return strings.Join(parts, ",")
}

type aggregateGroupingMode uint8

const (
	aggregateGroupingPlain aggregateGroupingMode = iota
	aggregateGroupingRollup
	aggregateGroupingCube
	aggregateGroupingSets
)

type aggregateDefinition struct {
	input        *streamNode
	join         *joinDefinition
	groupBy      []Expr
	grouping     aggregateGroupingMode
	groupingSets [][]int
	selections   []Selection
	where        Expr
	having       Expression[bool]
}

type JoinSide uint8

const (
	JoinLeft JoinSide = iota
	JoinRight
)

type JoinCondition struct {
	Left        Expr
	Right       Expr
	Comparison  JoinComparison
	LeftSource  int
	RightSource int
	all         []JoinCondition
	any         []JoinCondition
	sourceSet   bool
}

// JoinComparison is the comparison applied to the two sides of a join
// condition. The zero value is equality so the public Left/Right struct form
// remains source-compatible with the initial two-stream API.
type JoinComparison uint8

const (
	JoinEqual JoinComparison = iota
	JoinNotEqual
	JoinLess
	JoinLessOrEqual
	JoinGreater
	JoinGreaterOrEqual
)

func OnEqual(left, right Expr) JoinCondition {
	return OnSourcesCompare(0, left, 1, right, JoinEqual)
}

func OnCompare(left, right Expr, comparison JoinComparison) JoinCondition {
	return OnSourcesCompare(0, left, 1, right, comparison)
}

func OnLess(left, right Expr) JoinCondition {
	return OnCompare(left, right, JoinLess)
}

func OnLessOrEqual(left, right Expr) JoinCondition {
	return OnCompare(left, right, JoinLessOrEqual)
}

func OnGreater(left, right Expr) JoinCondition {
	return OnCompare(left, right, JoinGreater)
}

func OnGreaterOrEqual(left, right Expr) JoinCondition {
	return OnCompare(left, right, JoinGreaterOrEqual)
}

// OnSourcesCompare creates a condition for a multi-stream join. Source
// indices are zero-based and refer to the order passed to JoinMany.
func OnSourcesCompare(leftSource int, left Expr, rightSource int, right Expr, comparison JoinComparison) JoinCondition {
	return JoinCondition{
		Left:        left,
		Right:       right,
		Comparison:  comparison,
		LeftSource:  leftSource,
		RightSource: rightSource,
		sourceSet:   true,
	}
}

func OnSourcesEqual(leftSource int, left Expr, rightSource int, right Expr) JoinCondition {
	return OnSourcesCompare(leftSource, left, rightSource, right, JoinEqual)
}

// AllJoin and AnyJoin compose analyzable join predicates without resorting to
// an opaque callback. Empty composites are rejected during Build.
func AllJoin(conditions ...JoinCondition) JoinCondition {
	return JoinCondition{all: append([]JoinCondition(nil), conditions...)}
}

func AnyJoin(conditions ...JoinCondition) JoinCondition {
	return JoinCondition{any: append([]JoinCondition(nil), conditions...)}
}

func joinConditionSources(condition JoinCondition) (int, int) {
	if condition.sourceSet {
		return condition.LeftSource, condition.RightSource
	}
	// Preserve the original two-stream struct literal contract where the
	// absence of source metadata meant left expression versus right expression.
	return 0, 1
}

type JoinKind uint8

const (
	JoinInner JoinKind = iota
	JoinLeftOuter
	JoinRightOuter
	JoinFullOuter
)

// joinTuple is the internal event envelope used when a Join stream transitions
// into an Aggregate stream. Aggregate expressions still operate on []Event,
// while JoinField/JoinEventValue make the tuple's source scope explicit.
type joinTuple struct {
	events []Event
}

type JoinStream[L, R any] struct {
	env            *Environment
	left           *streamNode
	right          *streamNode
	condition      JoinCondition
	kind           JoinKind
	unidirectional [2]bool
}

// JoinInput is the type-erased source handle used by JoinMany. It preserves
// the logical stream node while allowing streams with different Go event
// types to participate in one analyzable join graph.
type JoinInput struct {
	env            *Environment
	node           *streamNode
	unidirectional bool
}

func JoinSource[T any](stream Stream[T]) JoinInput {
	return JoinInput{env: stream.env, node: stream.node}
}

func JoinRecordSource(stream RecordStream) JoinInput {
	return JoinInput{env: stream.env, node: stream.node}
}

// JoinPatternSource adapts a fluent PatternStream into a Join source. Pattern
// matches are materialized as events whose properties are the captured tags;
// use JoinPatternField or Property(JoinField[Event](...)) to address a tag
// property from the tuple.
func JoinPatternSource(pattern PatternStream) JoinInput {
	input := JoinInput{env: pattern.env}
	if pattern.def == nil {
		input.node = &streamNode{kind: streamPattern, sourceName: "<nil-pattern>", sourceType: typeOf[any](), configurationError: "join pattern source requires a pattern"}
		return input
	}
	sourceName := "pattern"
	if pattern.def.input != nil {
		sourceName = "pattern:" + pattern.def.input.describe()
	}
	input.node = &streamNode{
		kind:       streamPattern,
		sourceName: sourceName,
		sourceType: typeOf[any](),
		input:      pattern.def.input,
		pattern:    pattern.def,
	}
	return input
}

// JoinPattern is a concise alias for JoinPatternSource.
func JoinPattern(pattern PatternStream) JoinInput { return JoinPatternSource(pattern) }

// Unidirectional marks this source as a transient join driver. A single
// driver probes retained passive sources; marking every source is valid only
// for a full-outer join and emits one transient tuple per arriving event.
func (input JoinInput) Unidirectional() JoinInput {
	input.unidirectional = true
	return input
}

// Window retains completed Pattern matches on the Join side. It is kept on
// JoinInput rather than PatternStream because a standalone Pattern query
// emits matches directly and has no result-side data window.
func (input JoinInput) Window(window WindowSpec) JoinInput {
	if input.node == nil {
		input.node = &streamNode{kind: streamPattern, sourceName: "<nil-pattern>", configurationError: "pattern join window requires a pattern source"}
		return input
	}
	if input.node.kind == streamPattern {
		cloned := cloneStreamNode(input.node)
		cloned.patternWindow = window
		input.node = cloned
		return input
	}
	input.node = &streamNode{kind: streamWindow, input: input.node, window: window}
	return input
}

type MultiJoinStream struct {
	env            *Environment
	sources        []*streamNode
	conditions     []JoinCondition
	kind           JoinKind
	unidirectional []bool
}

// ChainedJoinStream builds a left-deep join tree one edge at a time. Unlike
// JoinMany's convenient single-kind form, each edge carries its own join kind
// and ON conditions, allowing fluent mixed outer joins without EPL text.
type ChainedJoinStream struct {
	env            *Environment
	sources        []*streamNode
	edges          []joinEdgeDefinition
	unidirectional []bool
}

// JoinChain starts a left-deep multi-stream join from one source.
func JoinChain(first JoinInput) ChainedJoinStream {
	return ChainedJoinStream{
		env: first.env, sources: []*streamNode{first.node}, unidirectional: []bool{first.unidirectional},
	}
}

func (j ChainedJoinStream) join(input JoinInput, kind JoinKind, conditions []JoinCondition) ChainedJoinStream {
	j.sources = append(append([]*streamNode(nil), j.sources...), input.node)
	j.unidirectional = append(append([]bool(nil), j.unidirectional...), input.unidirectional)
	j.edges = append(append([]joinEdgeDefinition(nil), j.edges...), joinEdgeDefinition{
		kind: kind, conditions: append([]JoinCondition(nil), conditions...),
	})
	return j
}

func (j ChainedJoinStream) InnerJoin(input JoinInput, conditions ...JoinCondition) ChainedJoinStream {
	return j.join(input, JoinInner, conditions)
}

func (j ChainedJoinStream) LeftOuterJoin(input JoinInput, conditions ...JoinCondition) ChainedJoinStream {
	return j.join(input, JoinLeftOuter, conditions)
}

func (j ChainedJoinStream) RightOuterJoin(input JoinInput, conditions ...JoinCondition) ChainedJoinStream {
	return j.join(input, JoinRightOuter, conditions)
}

func (j ChainedJoinStream) FullOuterJoin(input JoinInput, conditions ...JoinCondition) ChainedJoinStream {
	return j.join(input, JoinFullOuter, conditions)
}

func (j ChainedJoinStream) Select(selections ...JoinSelection) JoinQuery {
	return JoinQuery{
		env: j.env,
		definition: &joinDefinition{
			sources:        append([]*streamNode(nil), j.sources...),
			edges:          cloneJoinEdges(j.edges),
			unidirectional: cloneJoinUnidirectional(j.unidirectional),
		},
		selections: append([]JoinSelection(nil), selections...),
	}
}

func (j ChainedJoinStream) Query(options ...QueryOption) Query {
	return j.Select().Query(options...)
}

func (j ChainedJoinStream) Aggregate(selections ...Selection) AggregateStream {
	definition := j.Select().definition
	var node *streamNode
	if len(definition.sources) > 0 {
		node = definition.sources[0]
	}
	return AggregateStream{env: j.env, node: node, join: definition, selections: append([]Selection(nil), selections...)}
}

func (j ChainedJoinStream) GroupBy(keys ...Expr) AggregateStream {
	definition := j.Select().definition
	var node *streamNode
	if len(definition.sources) > 0 {
		node = definition.sources[0]
	}
	return AggregateStream{env: j.env, node: node, join: definition, groupBy: append([]Expr(nil), keys...)}
}

// JoinMany creates a multi-stream join. Conditions should use
// OnSourcesEqual/OnSourcesCompare so each expression is tied to a source
// index. Left/right/full outer variants retain unmatched tuples with zero
// Event placeholders for missing sources.
func JoinMany(inputs ...JoinInput) MultiJoinStream {
	sources := make([]*streamNode, 0, len(inputs))
	unidirectional := make([]bool, 0, len(inputs))
	var env *Environment
	for _, input := range inputs {
		if env == nil {
			env = input.env
		}
		sources = append(sources, input.node)
		unidirectional = append(unidirectional, input.unidirectional)
	}
	return MultiJoinStream{env: env, sources: sources, kind: JoinInner, unidirectional: unidirectional}
}

func (j MultiJoinStream) On(conditions ...JoinCondition) MultiJoinStream {
	j.conditions = append(append([]JoinCondition(nil), j.conditions...), conditions...)
	return j
}

func (j MultiJoinStream) LeftOuter() MultiJoinStream {
	j.kind = JoinLeftOuter
	return j
}

func (j MultiJoinStream) RightOuter() MultiJoinStream {
	j.kind = JoinRightOuter
	return j
}

func (j MultiJoinStream) FullOuter() MultiJoinStream {
	j.kind = JoinFullOuter
	return j
}

// Aggregate starts a tuple-aware aggregate over the join result. Aggregate
// expressions should use JoinField or JoinEventValue to identify which source
// in the tuple they read.
func (j MultiJoinStream) Aggregate(selections ...Selection) AggregateStream {
	definition := j.Select().definition
	var node *streamNode
	if len(definition.sources) > 0 {
		node = definition.sources[0]
	}
	return AggregateStream{env: j.env, node: node, join: definition, selections: append([]Selection(nil), selections...)}
}

func (j MultiJoinStream) GroupBy(keys ...Expr) AggregateStream {
	definition := j.Select().definition
	var node *streamNode
	if len(definition.sources) > 0 {
		node = definition.sources[0]
	}
	return AggregateStream{env: j.env, node: node, join: definition, groupBy: append([]Expr(nil), keys...)}
}

func (j MultiJoinStream) Select(selections ...JoinSelection) JoinQuery {
	return JoinQuery{
		env: j.env,
		definition: &joinDefinition{
			sources:        append([]*streamNode(nil), j.sources...),
			conditions:     append([]JoinCondition(nil), j.conditions...),
			kind:           j.kind,
			unidirectional: cloneJoinUnidirectional(j.unidirectional),
		},
		selections: append([]JoinSelection(nil), selections...),
	}
}

func (j MultiJoinStream) Query(options ...QueryOption) Query {
	return j.Select().Query(options...)
}

type JoinSelection struct {
	Name      string
	Side      JoinSide
	Source    int
	Expr      Expr
	sourceSet bool
}

func SelectLeft(name string, expression Expr) JoinSelection {
	return JoinSelection{Name: name, Side: JoinLeft, Source: 0, sourceSet: true, Expr: expression}
}

func SelectRight(name string, expression Expr) JoinSelection {
	return JoinSelection{Name: name, Side: JoinRight, Source: 1, sourceSet: true, Expr: expression}
}

func SelectFrom(source int, name string, expression Expr) JoinSelection {
	return JoinSelection{Name: name, Source: source, sourceSet: true, Expr: expression}
}

// SelectSourceEvent projects the complete Event envelope from one join source.
// It is the chainable equivalent of selecting a source event under an alias
// (the source portion of Esper's join wildcard projection).
func SelectSourceEvent(source int, name string) JoinSelection {
	return SelectFrom(source, name, EventValue[Event]())
}

func (selection JoinSelection) sourceIndex() int {
	if selection.sourceSet {
		return selection.Source
	}
	if selection.Side == JoinRight {
		return 1
	}
	return 0
}

type JoinQuery struct {
	env        *Environment
	definition *joinDefinition
	selections []JoinSelection
	where      Expr
}

type joinDefinition struct {
	left           *streamNode
	right          *streamNode
	condition      JoinCondition
	sources        []*streamNode
	conditions     []JoinCondition
	kind           JoinKind
	edges          []joinEdgeDefinition
	unidirectional []bool
}

type joinEdgeDefinition struct {
	kind       JoinKind
	conditions []JoinCondition
}

func cloneJoinEdges(edges []joinEdgeDefinition) []joinEdgeDefinition {
	if len(edges) == 0 {
		return nil
	}
	cloned := make([]joinEdgeDefinition, len(edges))
	for index, edge := range edges {
		cloned[index] = joinEdgeDefinition{kind: edge.kind, conditions: append([]JoinCondition(nil), edge.conditions...)}
	}
	return cloned
}

func cloneJoinUnidirectional(flags []bool) []bool {
	for _, flag := range flags {
		if flag {
			return append([]bool(nil), flags...)
		}
	}
	return nil
}

// Join constructs a two-stream join. With no condition it is a Cartesian join;
// multiple conditions are combined with logical AND for a compact fluent form.
func Join[L, R any](left Stream[L], right Stream[R], conditions ...JoinCondition) JoinStream[L, R] {
	var condition JoinCondition
	switch len(conditions) {
	case 0:
		// An omitted condition is an explicit Cartesian join.
	case 1:
		condition = conditions[0]
	default:
		condition = AllJoin(conditions...)
	}
	return JoinStream[L, R]{env: left.env, left: left.node, right: right.node, condition: condition, kind: JoinInner}
}

func (j JoinStream[L, R]) LeftOuter() JoinStream[L, R] {
	j.kind = JoinLeftOuter
	return j
}

func (j JoinStream[L, R]) RightOuter() JoinStream[L, R] {
	j.kind = JoinRightOuter
	return j
}

func (j JoinStream[L, R]) FullOuter() JoinStream[L, R] {
	j.kind = JoinFullOuter
	return j
}

// Unidirectional marks one side of a two-stream join as transient. Calling it
// for both sides is valid only together with FullOuter.
func (j JoinStream[L, R]) Unidirectional(side JoinSide) JoinStream[L, R] {
	if side == JoinRight {
		j.unidirectional[1] = true
	} else {
		j.unidirectional[0] = true
	}
	return j
}

// Aggregate starts a tuple-aware aggregate over the two-stream join result.
// Use JoinField or JoinEventValue in aggregate expressions to keep source
// scope explicit in the Go API.
func (j JoinStream[L, R]) Aggregate(selections ...Selection) AggregateStream {
	definition := j.Select().definition
	return AggregateStream{env: j.env, node: j.left, join: definition, selections: append([]Selection(nil), selections...)}
}

// JoinPatternField reads a property from a captured tag on a Pattern join
// source. The source index addresses the Join tuple; tag and property keep
// the nested Pattern scope explicit in the Go API.
func JoinPatternField[V any](source int, tag, property string) Expression[V] {
	return Property[V](JoinField[Event](source, tag), property)
}

func (j JoinStream[L, R]) GroupBy(keys ...Expr) AggregateStream {
	definition := j.Select().definition
	return AggregateStream{env: j.env, node: j.left, join: definition, groupBy: append([]Expr(nil), keys...)}
}

func (j JoinStream[L, R]) Select(selections ...JoinSelection) JoinQuery {
	definition := &joinDefinition{left: j.left, right: j.right, condition: j.condition, sources: []*streamNode{j.left, j.right}, kind: j.kind, unidirectional: cloneJoinUnidirectional(j.unidirectional[:])}
	if joinConditionPresent(j.condition) {
		definition.conditions = []JoinCondition{j.condition}
	}
	return JoinQuery{env: j.env, definition: definition, selections: append([]JoinSelection(nil), selections...)}
}

func (j JoinStream[L, R]) Query(options ...QueryOption) Query {
	return j.Select().Query(options...)
}

func (j JoinQuery) Query(options ...QueryOption) Query {
	spec := querySpec{selector: SelectIStream, selections: nil}
	for _, option := range options {
		if option != nil {
			option(&spec)
		}
	}
	return Query{
		env:                        j.env,
		join:                       j.definition,
		joinSelections:             append([]JoinSelection(nil), j.selections...),
		joinWhere:                  j.where,
		routeTarget:                spec.routeTarget,
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

// Where applies a post-join predicate after ON matching and outer-row
// materialization. Use JoinField/JoinEventValue to make source scope explicit;
// this keeps outer-join null-side behavior analyzable in the fluent API.
func (j JoinQuery) Where(predicate Expression[bool]) JoinQuery {
	j.where = predicate
	return j
}

// InsertInto routes the join's new-stream projection into a registered event
// type.  The projection is materialized as a Row first, then converted by the
// runtime according to the target schema, just like a RecordStream route.
func (j JoinQuery) InsertInto(eventType string, options ...QueryOption) Query {
	options = append(append([]QueryOption(nil), options...), RouteTo(eventType))
	return j.Query(options...)
}

// From creates a typed source. The source name is explicit in normal use; an
// empty name falls back to the Go type name for small examples.
func From[T any](env *Environment, sourceName string) Stream[T] {
	if strings.TrimSpace(sourceName) == "" {
		typ := typeOf[T]()
		sourceName = typ.Name()
	}
	return Stream[T]{
		env:  env,
		node: &streamNode{kind: streamSource, sourceName: sourceName, sourceType: typeOf[T]()},
	}
}

// FromAny creates a schema-driven source for dynamic map/JSON events.
func FromAny(env *Environment, sourceName string) RecordStream {
	return RecordStream{env: env, node: &streamNode{kind: streamSource, sourceName: sourceName, sourceType: typeOf[any]()}}
}

// FromNamedWindow creates a consumer stream for a registered named window.
// The window itself owns retention; additional filter/window nodes are
// consumer-local views.
func FromNamedWindow(env *Environment, windowName string) RecordStream {
	return RecordStream{env: env, node: &streamNode{kind: streamNamedWindow, sourceName: windowName, sourceType: typeOf[any]()}}
}

// FromNamedWindowAs creates a typed consumer stream for a named window. It
// keeps the named-window source in the same generic chain as From, allowing
// contained properties to be expanded with Unnest and then passed through
// typed filters, windows, joins and projections.
func FromNamedWindowAs[T any](env *Environment, windowName string) Stream[T] {
	return Stream[T]{env: env, node: &streamNode{kind: streamNamedWindow, sourceName: windowName, sourceType: typeOf[T]()}}
}

// FromTable creates a read-only record stream over a registered table. Table
// sources are evaluated by Fire-and-Forget execution and are not fed by
// ordinary event delivery.
func FromTable(env *Environment, tableName string) RecordStream {
	return RecordStream{env: env, node: &streamNode{kind: streamTable, sourceName: tableName, sourceType: typeOf[any]()}}
}

// FromHistorical creates a source whose rows are polled when an input event
// enters the statement. The provider receives that trigger event, allowing a
// prepared SQL query or another external lookup to bind event properties
// without embedding EPL text in the Go rule.
func FromHistorical[T any](env *Environment, sourceName string, schema Schema, provider HistoricalProvider) Stream[T] {
	return FromHistoricalOn[T](env, sourceName, "", schema, provider)
}

// FromHistoricalOn restricts polling to one registered trigger event type.
// The unrestricted FromHistorical form is useful for a statement driven by
// several event types or by an application-level scheduler.
func FromHistoricalOn[T any](env *Environment, sourceName, triggerType string, schema Schema, provider HistoricalProvider) Stream[T] {
	return Stream[T]{
		env: env,
		node: &streamNode{
			kind:       streamHistorical,
			sourceName: sourceName,
			sourceType: typeOf[T](),
			historical: &historicalDefinition{name: sourceName, trigger: triggerType, schema: schema, provider: provider},
		},
	}
}

// FromMethod creates a Go method-backed source. The provider is evaluated
// once for each incoming event in a live statement and once for a
// fire-and-forget snapshot. Its returned events can be filtered, windowed,
// joined and projected through the same fluent chain as ordinary sources.
func FromMethod[T any](env *Environment, sourceName string, schema Schema, provider MethodProvider) Stream[T] {
	return FromMethodOn[T](env, sourceName, "", schema, provider)
}

// FromMethodOn restricts a method source to one registered trigger event
// type. An empty triggerType keeps the source available to every event type.
func FromMethodOn[T any](env *Environment, sourceName, triggerType string, schema Schema, provider MethodProvider) Stream[T] {
	return Stream[T]{
		env: env,
		node: &streamNode{
			kind:       streamMethod,
			sourceName: sourceName,
			sourceType: typeOf[T](),
			method:     &methodDefinition{name: sourceName, trigger: triggerType, schema: schema, provider: provider},
		},
	}
}

// Unnest expands one slice/array-valued property of every parent event into
// child events. It is the typed Go counterpart of Esper contained-event
// syntax while keeping the rule chain explicit and composable:
//
//	books := Property[[]Book](EventValue[Order](), "books")
//	children := Unnest(From[Order](env, "Order"), books)
//
// The child Go type must have a registered schema in the environment. The
// returned stream can continue with Filter, Window, Join, Aggregate or Query.
func Unnest[T, V any](input Stream[T], property Expression[[]V]) Stream[V] {
	childType := typeOf[V]()
	return Stream[V]{
		env: input.env,
		node: &streamNode{
			kind:       streamContained,
			input:      input.node,
			sourceName: "unnest:" + childType.String(),
			sourceType: childType,
			contained:  &containedDefinition{property: property, childType: childType, elementType: childType},
		},
	}
}

// UnnestAs expands a slice/array property and materializes each element as
// the named event type. It is the explicit Go counterpart of Esper's
// contained-event @type annotation. V may be a raw representation (for
// example map[string]any, []any, string/[]byte JSON, or *AvroRecord) or Event.
// When an element is already an Event, its concrete event identity and
// underlying value are preserved after the target type is checked; this is
// what allows a BaseEvent target to carry AEvent/BEvent members.
//
// The returned stream intentionally exposes Event rather than pretending that
// a polymorphic @type result has one concrete Go struct type. Callers can use
// EventValue[Event](), Field[Event, V](...), Property or Event.Get in the
// following chain.
func UnnestAs[T, V any](input Stream[T], property Expression[[]V], targetType string) Stream[Event] {
	return newUnnestAsStream(input, property, typeOf[V](), targetType, false)
}

func newUnnestAsStream[T any](input Stream[T], property Expr, elementType reflect.Type, targetType string, sequence bool) Stream[Event] {
	targetType = strings.TrimSpace(targetType)
	definition := &containedDefinition{
		property:         property,
		childType:        typeOf[Event](),
		elementType:      elementType,
		targetSchemaName: targetType,
		sequence:         sequence,
	}
	if targetType == "" {
		definition.targetSchemaName = "<invalid>"
	}
	return Stream[Event]{
		env: input.env,
		node: &streamNode{
			kind:       streamContained,
			input:      input.node,
			sourceName: "unnest-as:" + targetType,
			sourceType: typeOf[Event](),
			contained:  definition,
		},
	}
}

// UnnestSeqAs expands a Go iter.Seq-valued property and materializes each
// yielded element as the named event type. iter.Seq is the Go-native
// equivalent of Esper's Collection/Iterable return shape; yield order is
// preserved and the result remains a normal Event stream for further fluent
// operators.
func UnnestSeqAs[T, V any](input Stream[T], property Expression[iter.Seq[V]], targetType string) Stream[Event] {
	return newUnnestAsStream(input, property, typeOf[V](), targetType, true)
}

// UnnestEvents is the common EventBean[]/Event[] split form. The target type
// is still explicit because one result array may contain different concrete
// member schemas under a common parent or variant schema.
func UnnestEvents[T any](input Stream[T], property Expression[[]Event], targetType string) Stream[Event] {
	return UnnestAs[T, Event](input, property, targetType)
}

// UnnestSeqEvents is the iter.Seq counterpart of UnnestEvents.
func UnnestSeqEvents[T any](input Stream[T], property Expression[iter.Seq[Event]], targetType string) Stream[Event] {
	return UnnestSeqAs[T, Event](input, property, targetType)
}

// ContainedValue is the explicit event shape used by UnnestValues for scalar
// array elements. A caller registers ContainedValue[T] under the desired
// child event name before building the rule.
type ContainedValue[T any] struct {
	Value T `esper:"value"`
}

// UnnestValues expands scalar slice/array elements into ContainedValue events.
// It keeps scalar contained data inside the same typed event-stream model as
// struct children, so the result can continue through Filter, Window,
// Aggregate, Select and Join.
func UnnestValues[T, V any](input Stream[T], property Expression[[]V]) Stream[ContainedValue[V]] {
	childType := typeOf[ContainedValue[V]]()
	elementType := typeOf[V]()
	return Stream[ContainedValue[V]]{
		env: input.env,
		node: &streamNode{
			kind:       streamContained,
			input:      input.node,
			sourceName: "unnest-values:" + elementType.String(),
			sourceType: childType,
			contained: &containedDefinition{
				property:    property,
				childType:   childType,
				elementType: elementType,
				wrap: func(value reflect.Value) (any, error) {
					if value.Kind() == reflect.Interface && !value.IsNil() {
						value = value.Elem()
					}
					wrapped := reflect.New(childType).Elem()
					wrapped.FieldByName("Value").Set(value)
					return wrapped.Interface(), nil
				},
			},
		},
	}
}

// DependingOn declares lateral/subordinate method inputs by join source
// name. During a join the provider is polled once for every compatible
// dependency tuple and obtains those events through MethodRequest.Dependency.
// Build rejects unknown, duplicate, cyclic and outer-join-incompatible
// dependencies. The stream value is cloned, preserving fluent API immutability.
func (s Stream[T]) DependingOn(sourceNames ...string) Stream[T] {
	node := cloneStreamNode(s.node)
	base, err := sourceNode(node)
	if err != nil || base.kind != streamMethod || base.method == nil {
		if node != nil {
			node.configurationError = "DependingOn requires a method source"
		}
		return Stream[T]{env: s.env, node: node}
	}
	base.method.dependencies = make([]string, len(sourceNames))
	for index, name := range sourceNames {
		base.method.dependencies[index] = strings.TrimSpace(name)
	}
	return Stream[T]{env: s.env, node: node}
}

func cloneStreamNode(node *streamNode) *streamNode {
	if node == nil {
		return nil
	}
	cloned := *node
	cloned.input = cloneStreamNode(node.input)
	if node.derived != nil {
		definition := *node.derived
		if node.derived.aggregate != nil {
			aggregate := *node.derived.aggregate
			aggregate.input = cloned.input
			aggregate.groupBy = append([]Expr(nil), node.derived.aggregate.groupBy...)
			aggregate.groupingSets = cloneGroupingExprSetsByIndex(node.derived.aggregate.groupingSets)
			aggregate.selections = append([]Selection(nil), node.derived.aggregate.selections...)
			definition.aggregate = &aggregate
		}
		definition.schema = node.derived.schema
		cloned.derived = &definition
	}
	if node.historical != nil {
		definition := *node.historical
		cloned.historical = &definition
	}
	if node.method != nil {
		definition := *node.method
		definition.dependencies = append([]string(nil), node.method.dependencies...)
		cloned.method = &definition
	}
	if node.contained != nil {
		definition := *node.contained
		cloned.contained = &definition
	}
	return &cloned
}

func cloneGroupingExprSetsByIndex(sets [][]int) [][]int {
	if sets == nil {
		return nil
	}
	cloned := make([][]int, len(sets))
	for index, set := range sets {
		cloned[index] = append([]int(nil), set...)
	}
	return cloned
}

func (s Stream[T]) Filter(predicate Expression[bool]) Stream[T] {
	return Stream[T]{
		env:  s.env,
		node: &streamNode{kind: streamFilter, input: s.node, predicate: predicate},
	}
}

func (s Stream[T]) Window(window WindowSpec) Stream[T] {
	return Stream[T]{
		env:  s.env,
		node: &streamNode{kind: streamWindow, input: s.node, window: window},
	}
}

// AsRecord exposes a typed stream through the dynamic RecordStream view while
// preserving the same source graph. It is useful when a typed contained or
// method source becomes the input of a subquery API whose result shape is
// intentionally dynamic.
func (s Stream[T]) AsRecord() RecordStream {
	return RecordStream{env: s.env, node: s.node}
}

func (s Stream[T]) Query(options ...QueryOption) Query {
	return newQuery(s.env, s.node, nil, options...)
}

func (s Stream[T]) GroupBy(keys ...Expr) AggregateStream {
	return AggregateStream{env: s.env, node: s.node, groupBy: append([]Expr(nil), keys...)}
}

// GroupByRollup creates hierarchical grouping levels from the most specific
// key set to the overall group. It is the Go fluent equivalent of a rollup
// clause and keeps the expressions analyzable by Build.
func (s Stream[T]) GroupByRollup(keys ...Expr) AggregateStream {
	return AggregateStream{env: s.env, node: s.node, groupBy: append([]Expr(nil), keys...), grouping: aggregateGroupingRollup}
}

// GroupByCube creates every grouping combination for the supplied keys.
func (s Stream[T]) GroupByCube(keys ...Expr) AggregateStream {
	return AggregateStream{env: s.env, node: s.node, groupBy: append([]Expr(nil), keys...), grouping: aggregateGroupingCube}
}

// GroupingSet is a small helper for GroupByGroupingSets. An empty set is
// meaningful and represents the overall aggregate.
func GroupingSet(keys ...Expr) []Expr {
	return append([]Expr(nil), keys...)
}

// GroupByGroupingSets creates the explicit grouping sets in the supplied
// order. Expressions are de-duplicated into one dimension list while each
// set retains its own selected dimensions.
func (s Stream[T]) GroupByGroupingSets(sets ...[]Expr) AggregateStream {
	return AggregateStream{env: s.env, node: s.node, grouping: aggregateGroupingSets, groupingSets: cloneGroupingExprSets(sets)}
}

func (s Stream[T]) Aggregate(selections ...Selection) AggregateStream {
	return AggregateStream{env: s.env, node: s.node, selections: append([]Selection(nil), selections...)}
}

func (s Stream[T]) To(sink Sink, options ...QueryOption) Query {
	options = append(options, WithSink(sink))
	return s.Query(options...)
}

// InsertInto builds a statement whose new-stream events are routed to a
// registered event type after this statement processes an input event. With
// no projection the original Event identity/underlying value is preserved;
// RecordStream.InsertInto routes a Row projection through the target schema.
func (s Stream[T]) InsertInto(eventType string, options ...QueryOption) Query {
	options = append(append([]QueryOption(nil), options...), RouteTo(eventType))
	return s.Query(options...)
}

// Select is a type-changing top-level combinator. It yields an ordered Row
// result and keeps the underlying stream graph analyzable.
func Select[T any](s Stream[T], selections ...Selection) RecordStream {
	return RecordStream{env: s.env, node: s.node, selections: append([]Selection(nil), selections...)}
}

func (s RecordStream) Filter(predicate Expression[bool]) RecordStream {
	return RecordStream{env: s.env, node: &streamNode{kind: streamFilter, input: s.node, predicate: predicate}, selections: append([]Selection(nil), s.selections...)}
}

// Select appends a typed projection to an untyped record source such as a
// Named Window or Table. Keeping this as a method makes those sources as
// chainable as a typed Stream while preserving the existing selection list.
func (s RecordStream) Select(selections ...Selection) RecordStream {
	projected := append([]Selection(nil), s.selections...)
	projected = append(projected, selections...)
	return RecordStream{env: s.env, node: s.node, selections: projected}
}

func (s RecordStream) Window(window WindowSpec) RecordStream {
	return RecordStream{env: s.env, node: &streamNode{kind: streamWindow, input: s.node, window: window}, selections: append([]Selection(nil), s.selections...)}
}

func (s RecordStream) Query(options ...QueryOption) Query {
	return newQuery(s.env, s.node, s.selections, options...)
}

// OnDemandStream is the fluent target-side view used to build a one-shot
// mutation against a Table or Named Window.  It deliberately has no incoming
// event stream: expressions are evaluated against the current target row
// through TableField/NamedWindowField, while insert expressions normally use
// literals, variables or parameters.
type OnDemandStream struct {
	env         *Environment
	node        *streamNode
	contextName string
}

// OnDemand starts a Fire-and-Forget state mutation chain for this source.
// Build rejects sources that are not a root Table or Named Window.
func (s RecordStream) OnDemand() OnDemandStream {
	return OnDemandStream{env: s.env, node: s.node}
}

// OnDemand exposes the same target-side builder for callers that keep a
// typed source handle.  A typed event stream is still rejected at Build time
// unless its root is a Table or Named Window.
func (s Stream[T]) OnDemand() OnDemandStream {
	return OnDemandStream{env: s.env, node: s.node}
}

// WithContext scopes the on-demand mutation chain to the named context.
// ExecuteFireAndForgetWithSelector can then select the context partitions
// that receive the mutation. Keeping this on the target-side builder makes
// the scope visible in the same fluent chain as the mutation itself.
func (s OnDemandStream) WithContext(name string) OnDemandStream {
	s.contextName = strings.TrimSpace(name)
	return s
}

func (s OnDemandStream) query(action onDemandAction, predicate Expr, assignments []TableAssignment, options ...QueryOption) Query {
	spec := querySpec{selector: SelectIStream, output: OutputAll(), contextName: s.contextName}
	for _, option := range options {
		if option != nil {
			option(&spec)
		}
	}
	return Query{
		env:         s.env,
		input:       s.node,
		name:        spec.name,
		selector:    spec.selector,
		output:      spec.output,
		contextName: spec.contextName,
		onDemand:    &onDemandDefinition{action: action, predicate: predicate, assignments: append([]TableAssignment(nil), assignments...)},
	}
}

// Insert appends one row/event to the target. Assignments are evaluated once
// with no incoming event, which makes Literal, Variable and Parameter
// expressions the natural Go equivalents of an on-demand values clause.
func (s OnDemandStream) Insert(assignments ...TableAssignment) Query {
	return s.query(onDemandInsert, nil, assignments)
}

// UpdateWhere updates every target row for which predicate is true. The
// predicate and assignment expressions may read the current row with
// TableField or NamedWindowField; assignments are applied in declaration
// order and Initial*Field retains the pre-update value.
func (s OnDemandStream) UpdateWhere(predicate Expression[bool], assignments ...TableAssignment) Query {
	return s.query(onDemandUpdate, predicate, assignments)
}

// DeleteWhere deletes every target row for which predicate is true.
func (s OnDemandStream) DeleteWhere(predicate Expression[bool]) Query {
	return s.query(onDemandDelete, predicate, nil)
}

// DeleteAll deletes every row/event in the target.
func (s OnDemandStream) DeleteAll() Query {
	return s.query(onDemandDeleteAll, nil, nil)
}

func (s RecordStream) GroupBy(keys ...Expr) AggregateStream {
	return AggregateStream{env: s.env, node: s.node, groupBy: append([]Expr(nil), keys...)}
}

func (s RecordStream) GroupByRollup(keys ...Expr) AggregateStream {
	return AggregateStream{env: s.env, node: s.node, groupBy: append([]Expr(nil), keys...), grouping: aggregateGroupingRollup}
}

func (s RecordStream) GroupByCube(keys ...Expr) AggregateStream {
	return AggregateStream{env: s.env, node: s.node, groupBy: append([]Expr(nil), keys...), grouping: aggregateGroupingCube}
}

func (s RecordStream) GroupByGroupingSets(sets ...[]Expr) AggregateStream {
	return AggregateStream{env: s.env, node: s.node, grouping: aggregateGroupingSets, groupingSets: cloneGroupingExprSets(sets)}
}

func (s RecordStream) Aggregate(selections ...Selection) AggregateStream {
	return AggregateStream{env: s.env, node: s.node, selections: append([]Selection(nil), s.selections...)}.Select(selections...)
}

func (s RecordStream) To(sink Sink, options ...QueryOption) Query {
	options = append(options, WithSink(sink))
	return s.Query(options...)
}

// InsertInto routes the projected new stream into a registered event type.
// A map/JSON/Avro target receives the row by field name; an ANY Variant may
// receive an anonymous derived map event. PREDEFINED Variant targets require
// an existing member Event because identity is part of their contract.
func (s RecordStream) InsertInto(eventType string, options ...QueryOption) Query {
	options = append(append([]QueryOption(nil), options...), RouteTo(eventType))
	return s.Query(options...)
}

func Aggregate[T any](s Stream[T], selections ...Selection) AggregateStream {
	return s.Aggregate(selections...)
}

func GroupBy[T any](s Stream[T], keys ...Expr) AggregateStream {
	return s.GroupBy(keys...)
}

func GroupByRollup[T any](s Stream[T], keys ...Expr) AggregateStream {
	return s.GroupByRollup(keys...)
}

func GroupByCube[T any](s Stream[T], keys ...Expr) AggregateStream {
	return s.GroupByCube(keys...)
}

func GroupByGroupingSets[T any](s Stream[T], sets ...[]Expr) AggregateStream {
	return s.GroupByGroupingSets(sets...)
}

func (a AggregateStream) Select(selections ...Selection) AggregateStream {
	a.selections = append(a.selections, selections...)
	return a
}

// Rollup and Cube are convenience modifiers for callers that start with an
// AggregateStream. They replace its grouping dimensions, which makes the
// resulting plan unambiguous and avoids a hidden mixture of grouping modes.
func (a AggregateStream) Rollup(keys ...Expr) AggregateStream {
	a.groupBy = append([]Expr(nil), keys...)
	a.grouping = aggregateGroupingRollup
	a.groupingSets = nil
	return a
}

func (a AggregateStream) Cube(keys ...Expr) AggregateStream {
	a.groupBy = append([]Expr(nil), keys...)
	a.grouping = aggregateGroupingCube
	a.groupingSets = nil
	return a
}

func (a AggregateStream) GroupingSets(sets ...[]Expr) AggregateStream {
	a.groupBy = nil
	a.grouping = aggregateGroupingSets
	a.groupingSets = cloneGroupingExprSets(sets)
	return a
}

func (a AggregateStream) Having(predicate Expression[bool]) AggregateStream {
	a.having = predicate
	return a
}

// Where filters the current stream or join tuples before grouping and
// aggregation. For a Join aggregate the predicate must use JoinField or
// JoinEventValue so the complete tuple scope remains analyzable. On ordinary
// streams, Filter is the more explicit equivalent and this method is kept as
// a compact aggregate-chain form.
func (a AggregateStream) Where(predicate Expression[bool]) AggregateStream {
	a.where = predicate
	return a
}

func (a AggregateStream) Query(options ...QueryOption) Query {
	spec := querySpec{selector: SelectIStream}
	for _, option := range options {
		if option != nil {
			option(&spec)
		}
	}
	groupBy, groupingSets := normalizedAggregateGrouping(a.groupBy, a.grouping, a.groupingSets)
	return Query{
		env:         a.env,
		input:       a.node,
		join:        a.join,
		routeTarget: spec.routeTarget,
		tableTarget: spec.tableTarget,
		aggregate: &aggregateDefinition{
			input:        a.node,
			join:         a.join,
			groupBy:      groupBy,
			grouping:     a.grouping,
			groupingSets: groupingSets,
			selections:   append([]Selection(nil), a.selections...),
			where:        a.where,
			having:       a.having,
		},
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

// To attaches a Go sink to aggregate results. It mirrors Stream.To while
// retaining the aggregate builder's grouping, having and output semantics.
func (a AggregateStream) To(sink Sink, options ...QueryOption) Query {
	options = append(options, WithSink(sink))
	return a.Query(options...)
}

// AsJoinSource turns an ordinary aggregate chain into a reusable derived
// event source. Each aggregate update is materialized as a schema-bound Row
// event, so it can participate in another fluent Join without EPL text.
// Join aggregates and side effects are intentionally rejected at Build time;
// this adapter models view/aggregate output, not a second statement graph.
func (a AggregateStream) AsJoinSource() JoinInput {
	inputNode := cloneStreamNode(a.node)
	node := &streamNode{
		kind:       streamDerived,
		sourceName: "derived-aggregate",
		sourceType: typeOf[any](),
		input:      inputNode,
	}
	if a.env == nil || inputNode == nil {
		node.configurationError = "derived aggregate source requires an environment and input"
		return JoinInput{env: a.env, node: node}
	}
	if a.join != nil {
		node.configurationError = "derived aggregate source cannot wrap a join aggregate"
		return JoinInput{env: a.env, node: node}
	}
	if len(a.selections) == 0 {
		node.configurationError = "derived aggregate source requires at least one projection"
		return JoinInput{env: a.env, node: node}
	}
	groupBy, groupingSets := normalizedAggregateGrouping(a.groupBy, a.grouping, a.groupingSets)
	definition := &aggregateDefinition{
		input:        inputNode,
		groupBy:      groupBy,
		grouping:     a.grouping,
		groupingSets: groupingSets,
		selections:   append([]Selection(nil), a.selections...),
		where:        a.where,
		having:       a.having,
	}
	fields := make([]FieldSpec, 0, len(a.selections))
	for _, selection := range a.selections {
		fieldType := typeOf[any]()
		if selection.Expr != nil && selection.Expr.Type() != nil {
			fieldType = selection.Expr.Type()
		}
		fields = append(fields, FieldSpec{Name: selection.Name, Type: fieldType})
	}
	schema, err := NewSchema("derived:"+a.node.describe(), fields...)
	if err != nil {
		node.configurationError = err.Error()
		return JoinInput{env: a.env, node: node}
	}
	node.derived = &derivedStreamDefinition{aggregate: definition, schema: schema}
	return JoinInput{env: a.env, node: node}
}

// JoinAggregateSource is the function form of AggregateStream.AsJoinSource.
func JoinAggregateSource(aggregate AggregateStream) JoinInput {
	return aggregate.AsJoinSource()
}

func cloneGroupingExprSets(sets [][]Expr) [][]Expr {
	if sets == nil {
		return nil
	}
	cloned := make([][]Expr, len(sets))
	for index, set := range sets {
		cloned[index] = append([]Expr(nil), set...)
	}
	return cloned
}

func groupingExpressionKey(expression Expr) string {
	if expression == nil {
		return "<nil>"
	}
	return groupingNodeKey(expression.node(), expression.Description())
}

func groupingNodeKey(node *exprNode, description string) string {
	if node == nil {
		return "<nil>"
	}
	return node.kind + ":" + description
}

func normalizedAggregateGrouping(groupBy []Expr, mode aggregateGroupingMode, sets [][]Expr) ([]Expr, [][]int) {
	keys := append([]Expr(nil), groupBy...)
	if mode != aggregateGroupingSets {
		return keys, nil
	}
	keys = nil
	indexes := make(map[string]int)
	normalizedSets := make([][]int, len(sets))
	for setIndex, set := range sets {
		for _, expression := range set {
			key := groupingExpressionKey(expression)
			index, exists := indexes[key]
			if !exists {
				index = len(keys)
				indexes[key] = index
				keys = append(keys, expression)
			}
			normalizedSets[setIndex] = append(normalizedSets[setIndex], index)
		}
	}
	return keys, normalizedSets
}

// InsertInto routes each aggregate new-stream row into a registered event
// type.  Old-stream aggregate updates remain listener-only, matching the
// new-stream insert-into contract.
func (a AggregateStream) InsertInto(eventType string, options ...QueryOption) Query {
	options = append(append([]QueryOption(nil), options...), RouteTo(eventType))
	return a.Query(options...)
}

// IntoTable builds a live aggregate statement that materializes its current
// grouped rows into a registered table. Additional options still control the
// statement name, output selector, listener/sink and other query metadata.
func (a AggregateStream) IntoTable(tableName string, options ...QueryOption) Query {
	options = append(append([]QueryOption(nil), options...), IntoTable(tableName))
	return a.Query(options...)
}

// WindowSpec is the common definition for stateful stream windows.
type WindowSpec interface {
	windowSpec()
	description() string
	validate() error
}

type LengthWindowSpec struct{ Size int }

func LengthWindow(size int) LengthWindowSpec   { return LengthWindowSpec{Size: size} }
func (LengthWindowSpec) windowSpec()           {}
func (w LengthWindowSpec) description() string { return fmt.Sprintf("length(%d)", w.Size) }
func (w LengthWindowSpec) validate() error {
	if w.Size <= 0 {
		return fmt.Errorf("esper: length window size must be positive, got %d", w.Size)
	}
	return nil
}

type KeepAllWindowSpec struct{}

func KeepAll() KeepAllWindowSpec              { return KeepAllWindowSpec{} }
func (KeepAllWindowSpec) windowSpec()         {}
func (KeepAllWindowSpec) description() string { return "keep-all" }
func (KeepAllWindowSpec) validate() error     { return nil }

type LengthBatchWindowSpec struct{ Size int }

func LengthBatch(size int) LengthBatchWindowSpec    { return LengthBatchWindowSpec{Size: size} }
func (LengthBatchWindowSpec) windowSpec()           {}
func (w LengthBatchWindowSpec) description() string { return fmt.Sprintf("length-batch(%d)", w.Size) }
func (w LengthBatchWindowSpec) validate() error {
	if w.Size <= 0 {
		return fmt.Errorf("esper: length-batch window size must be positive, got %d", w.Size)
	}
	return nil
}

type TimeWindowSpec struct{ Duration time.Duration }

func TimeWindow(duration time.Duration) TimeWindowSpec { return TimeWindowSpec{Duration: duration} }
func (TimeWindowSpec) windowSpec()                     {}
func (w TimeWindowSpec) description() string           { return "time(" + w.Duration.String() + ")" }
func (w TimeWindowSpec) validate() error {
	if w.Duration <= 0 {
		return fmt.Errorf("esper: time window duration must be positive, got %s", w.Duration)
	}
	return nil
}

type TimeBatchWindowSpec struct{ Duration time.Duration }

func TimeBatch(duration time.Duration) TimeBatchWindowSpec {
	return TimeBatchWindowSpec{Duration: duration}
}
func (TimeBatchWindowSpec) windowSpec()           {}
func (w TimeBatchWindowSpec) description() string { return "time-batch(" + w.Duration.String() + ")" }
func (w TimeBatchWindowSpec) validate() error {
	if w.Duration <= 0 {
		return fmt.Errorf("esper: time-batch window duration must be positive, got %s", w.Duration)
	}
	return nil
}

type TimeLengthBatchWindowSpec struct {
	Duration time.Duration
	Size     int
}

func TimeLengthBatch(duration time.Duration, size int) TimeLengthBatchWindowSpec {
	return TimeLengthBatchWindowSpec{Duration: duration, Size: size}
}
func (TimeLengthBatchWindowSpec) windowSpec() {}
func (w TimeLengthBatchWindowSpec) description() string {
	return fmt.Sprintf("time-length-batch(%s,%d)", w.Duration, w.Size)
}
func (w TimeLengthBatchWindowSpec) validate() error {
	if w.Duration <= 0 || w.Size <= 0 {
		return fmt.Errorf("esper: time-length-batch requires positive duration and size")
	}
	return nil
}

type FirstEventWindowSpec struct{}

func FirstEvent() FirstEventWindowSpec           { return FirstEventWindowSpec{} }
func (FirstEventWindowSpec) windowSpec()         {}
func (FirstEventWindowSpec) description() string { return "first-event" }
func (FirstEventWindowSpec) validate() error     { return nil }

type LastEventWindowSpec struct{}

func LastEvent() LastEventWindowSpec            { return LastEventWindowSpec{} }
func (LastEventWindowSpec) windowSpec()         {}
func (LastEventWindowSpec) description() string { return "last-event" }
func (LastEventWindowSpec) validate() error     { return nil }

type FirstLengthWindowSpec struct{ Size int }

func FirstLength(size int) FirstLengthWindowSpec    { return FirstLengthWindowSpec{Size: size} }
func (FirstLengthWindowSpec) windowSpec()           {}
func (w FirstLengthWindowSpec) description() string { return fmt.Sprintf("first-length(%d)", w.Size) }
func (w FirstLengthWindowSpec) validate() error {
	if w.Size <= 0 {
		return fmt.Errorf("esper: first-length window size must be positive, got %d", w.Size)
	}
	return nil
}

type FirstTimeWindowSpec struct{ Duration time.Duration }

func FirstTime(duration time.Duration) FirstTimeWindowSpec {
	return FirstTimeWindowSpec{Duration: duration}
}
func (FirstTimeWindowSpec) windowSpec()           {}
func (w FirstTimeWindowSpec) description() string { return "first-time(" + w.Duration.String() + ")" }
func (w FirstTimeWindowSpec) validate() error {
	if w.Duration <= 0 {
		return fmt.Errorf("esper: first-time window duration must be positive, got %s", w.Duration)
	}
	return nil
}

type TimeAccumWindowSpec struct{ Duration time.Duration }

func TimeAccum(duration time.Duration) TimeAccumWindowSpec {
	return TimeAccumWindowSpec{Duration: duration}
}
func (TimeAccumWindowSpec) windowSpec()           {}
func (w TimeAccumWindowSpec) description() string { return "time-accum(" + w.Duration.String() + ")" }
func (w TimeAccumWindowSpec) validate() error {
	if w.Duration <= 0 {
		return fmt.Errorf("esper: time-accum window duration must be positive, got %s", w.Duration)
	}
	return nil
}

// ExpressionWindowSpec keeps the newest events while Keep evaluates to true
// over the current window group. The predicate is evaluated after insertion
// and during virtual-time advancement, which makes time-aware predicates
// deterministic without a wall-clock goroutine.
type ExpressionWindowSpec struct {
	Keep Expression[bool]
}

func ExpressionWindow(keep Expression[bool]) ExpressionWindowSpec {
	return ExpressionWindowSpec{Keep: keep}
}

func (ExpressionWindowSpec) windowSpec() {}
func (w ExpressionWindowSpec) description() string {
	if w.Keep == nil {
		return "expression-window(<nil>)"
	}
	return "expression-window(" + w.Keep.Description() + ")"
}
func (w ExpressionWindowSpec) validate() error {
	if w.Keep == nil {
		return fmt.Errorf("esper: expression window predicate is required")
	}
	if w.Keep.Type() != typeOf[bool]() {
		return fmt.Errorf("esper: expression window predicate must return bool, got %s", w.Keep.Type())
	}
	return nil
}

// ExpressionBatchOption configures whether the event that makes the trigger
// true belongs to the batch being emitted. The default is false, matching
// Esper's expr_batch default.
type ExpressionBatchOption func(*ExpressionBatchWindowSpec)

func IncludeTriggerEvent() ExpressionBatchOption {
	return func(spec *ExpressionBatchWindowSpec) { spec.IncludeTrigger = true }
}

type ExpressionBatchWindowSpec struct {
	Trigger        Expression[bool]
	IncludeTrigger bool
}

func ExpressionBatch(trigger Expression[bool], options ...ExpressionBatchOption) ExpressionBatchWindowSpec {
	spec := ExpressionBatchWindowSpec{Trigger: trigger}
	for _, option := range options {
		if option != nil {
			option(&spec)
		}
	}
	return spec
}

func (ExpressionBatchWindowSpec) windowSpec() {}
func (w ExpressionBatchWindowSpec) description() string {
	if w.Trigger == nil {
		return "expression-batch(<nil>)"
	}
	return fmt.Sprintf("expression-batch(%s,%t)", w.Trigger.Description(), w.IncludeTrigger)
}
func (w ExpressionBatchWindowSpec) validate() error {
	if w.Trigger == nil {
		return fmt.Errorf("esper: expression batch trigger is required")
	}
	if w.Trigger.Type() != typeOf[bool]() {
		return fmt.Errorf("esper: expression batch trigger must return bool, got %s", w.Trigger.Type())
	}
	return nil
}

// GroupWindow applies an inner window independently for each key. It is the
// builder-only equivalent of Esper's groupwin view and can be composed with
// length, time, batch, expression and uniqueness windows.
type GroupWindowSpec struct {
	Key   Expr
	Inner WindowSpec
}

func GroupWindow(key Expr, inner WindowSpec) GroupWindowSpec {
	return GroupWindowSpec{Key: key, Inner: inner}
}

func (GroupWindowSpec) windowSpec() {}
func (w GroupWindowSpec) description() string {
	if w.Key == nil || w.Inner == nil {
		return "group-window(<invalid>)"
	}
	return "group-window(" + w.Key.Description() + "," + w.Inner.description() + ")"
}
func (w GroupWindowSpec) validate() error {
	if w.Key == nil {
		return fmt.Errorf("esper: group window key expression is required")
	}
	if w.Inner == nil {
		return fmt.Errorf("esper: group window inner window is required")
	}
	return w.Inner.validate()
}

type CompositeWindowMode uint8

const (
	UnionWindowMode CompositeWindowMode = iota
	IntersectWindowMode
)

// CompositeWindowSpec combines independent child views over the same input.
// Union retains an event while any child retains it; intersect retains it only
// while every child retains it.
type CompositeWindowSpec struct {
	Mode    CompositeWindowMode
	Windows []WindowSpec
}

func UnionWindows(windows ...WindowSpec) CompositeWindowSpec {
	return CompositeWindowSpec{Mode: UnionWindowMode, Windows: append([]WindowSpec(nil), windows...)}
}

func IntersectWindows(windows ...WindowSpec) CompositeWindowSpec {
	return CompositeWindowSpec{Mode: IntersectWindowMode, Windows: append([]WindowSpec(nil), windows...)}
}

func (CompositeWindowSpec) windowSpec() {}
func (w CompositeWindowSpec) description() string {
	name := "union"
	if w.Mode == IntersectWindowMode {
		name = "intersect"
	}
	parts := make([]string, 0, len(w.Windows))
	for _, window := range w.Windows {
		if window == nil {
			parts = append(parts, "<nil>")
			continue
		}
		parts = append(parts, window.description())
	}
	return name + "(" + strings.Join(parts, ",") + ")"
}
func (w CompositeWindowSpec) validate() error {
	if w.Mode != UnionWindowMode && w.Mode != IntersectWindowMode {
		return fmt.Errorf("esper: composite window has unknown mode %d", w.Mode)
	}
	if len(w.Windows) < 2 {
		return fmt.Errorf("esper: composite window requires at least two child windows")
	}
	for _, window := range w.Windows {
		if window == nil {
			return fmt.Errorf("esper: composite window child is required")
		}
		if err := window.validate(); err != nil {
			return err
		}
	}
	return nil
}

type ExternallyTimedWindowSpec struct {
	Timestamp Expr
	Duration  time.Duration
	Batch     bool
}

type TimeOrderWindowSpec struct {
	Timestamp      Expr
	Duration       time.Duration
	CalendarYears  int
	CalendarMonths int
	CalendarDays   int
}

func TimeOrder(timestamp Expr, duration time.Duration) TimeOrderWindowSpec {
	return TimeOrderWindowSpec{Timestamp: timestamp, Duration: duration}
}

// TimeOrderCalendar creates an externally timestamp-ordered window whose
// retention interval is measured with calendar arithmetic. This is the Go
// fluent counterpart of Esper's time_order(timestamp, 1 month/year/day)
// forms; expiry uses time.Time.AddDate rather than a fixed duration.
func TimeOrderCalendar(timestamp Expr, years, months, days int) TimeOrderWindowSpec {
	return TimeOrderWindowSpec{
		Timestamp:      timestamp,
		CalendarYears:  years,
		CalendarMonths: months,
		CalendarDays:   days,
	}
}

func (TimeOrderWindowSpec) windowSpec() {}
func (w TimeOrderWindowSpec) description() string {
	if w.CalendarYears != 0 || w.CalendarMonths != 0 || w.CalendarDays != 0 {
		return fmt.Sprintf("time-order(%s,%dY%dM%dD)", w.Timestamp.Description(), w.CalendarYears, w.CalendarMonths, w.CalendarDays)
	}
	return "time-order(" + w.Timestamp.Description() + "," + w.Duration.String() + ")"
}
func (w TimeOrderWindowSpec) validate() error {
	calendar := w.CalendarYears != 0 || w.CalendarMonths != 0 || w.CalendarDays != 0
	if w.Timestamp == nil || (!calendar && w.Duration <= 0) || (calendar && (w.Duration != 0 || w.CalendarYears < 0 || w.CalendarMonths < 0 || w.CalendarDays < 0 || (w.CalendarYears == 0 && w.CalendarMonths == 0 && w.CalendarDays == 0))) {
		return fmt.Errorf("esper: time-order window requires timestamp and positive duration")
	}
	return nil
}

func ExternallyTimed(timestamp Expr, duration time.Duration) ExternallyTimedWindowSpec {
	return ExternallyTimedWindowSpec{Timestamp: timestamp, Duration: duration}
}
func ExternallyTimedBatch(timestamp Expr, duration time.Duration) ExternallyTimedWindowSpec {
	return ExternallyTimedWindowSpec{Timestamp: timestamp, Duration: duration, Batch: true}
}
func (ExternallyTimedWindowSpec) windowSpec() {}
func (w ExternallyTimedWindowSpec) description() string {
	name := "externally-timed"
	if w.Batch {
		name = "externally-timed-batch"
	}
	return name + "(" + w.Timestamp.Description() + "," + w.Duration.String() + ")"
}
func (w ExternallyTimedWindowSpec) validate() error {
	if w.Timestamp == nil || w.Duration <= 0 {
		return fmt.Errorf("esper: externally-timed window requires timestamp and positive duration")
	}
	return nil
}

type TimeToLiveWindowSpec struct{ Duration time.Duration }

func TimeToLive(duration time.Duration) TimeToLiveWindowSpec {
	return TimeToLiveWindowSpec{Duration: duration}
}
func (TimeToLiveWindowSpec) windowSpec() {}
func (w TimeToLiveWindowSpec) description() string {
	return "time-to-live(" + w.Duration.String() + ")"
}
func (w TimeToLiveWindowSpec) validate() error {
	if w.Duration <= 0 {
		return fmt.Errorf("esper: time-to-live duration must be positive, got %s", w.Duration)
	}
	return nil
}

// TimeToLiveAtWindowSpec retains each event until the absolute timestamp
// supplied by the event expression. The timestamp uses epoch milliseconds,
// matching TimeOrder and Esper's #timetolive(timestamp) view.
//
// TimeToLive is the fixed-duration form; TimeToLiveAt is the dynamic form.
type TimeToLiveAtWindowSpec struct{ Timestamp Expr }

func TimeToLiveAt(timestamp Expr) TimeToLiveAtWindowSpec {
	return TimeToLiveAtWindowSpec{Timestamp: timestamp}
}

func (TimeToLiveAtWindowSpec) windowSpec() {}
func (w TimeToLiveAtWindowSpec) description() string {
	if w.Timestamp == nil {
		return "time-to-live-at(<nil>)"
	}
	return "time-to-live-at(" + w.Timestamp.Description() + ")"
}
func (w TimeToLiveAtWindowSpec) validate() error {
	if w.Timestamp == nil {
		return fmt.Errorf("esper: time-to-live-at window requires a timestamp expression")
	}
	if !isIntegralType(w.Timestamp.Type()) {
		return fmt.Errorf("esper: time-to-live-at window requires an integral timestamp expression")
	}
	return nil
}

type SortKey struct {
	Expr       Expr
	Descending bool
}

func Ascending(expr Expr) SortKey  { return SortKey{Expr: expr} }
func Descending(expr Expr) SortKey { return SortKey{Expr: expr, Descending: true} }

type SortedWindowSpec struct {
	Size              int
	Keys              []SortKey
	Rank              bool
	UniqueKeys        []Expr
	requireUniqueKeys bool
}

func SortWindow(size int, keys ...SortKey) SortedWindowSpec {
	return SortedWindowSpec{Size: size, Keys: append([]SortKey(nil), keys...)}
}
func RankWindow(size int, keys ...SortKey) SortedWindowSpec {
	return SortedWindowSpec{Size: size, Keys: append([]SortKey(nil), keys...), Rank: true}
}

// RankWindowBy is the full Esper rank-view form: one current event is kept for
// each unique-key tuple, and the retained tuples are ordered by sortKeys up to
// size. The explicit slice keeps the unique-key part separate from sort keys,
// avoiding the positional parameter ambiguity of EPL's rank view.
func RankWindowBy(size int, uniqueKeys []Expr, sortKeys ...SortKey) SortedWindowSpec {
	return SortedWindowSpec{
		Size:              size,
		Keys:              append([]SortKey(nil), sortKeys...),
		Rank:              true,
		UniqueKeys:        append([]Expr(nil), uniqueKeys...),
		requireUniqueKeys: true,
	}
}

// RankWindowWithUniqueKeys is a descriptive alias for callers that prefer an
// API name which makes the replacement key behavior explicit.
func RankWindowWithUniqueKeys(size int, uniqueKeys []Expr, sortKeys ...SortKey) SortedWindowSpec {
	return RankWindowBy(size, uniqueKeys, sortKeys...)
}

func (SortedWindowSpec) windowSpec() {}
func (w SortedWindowSpec) description() string {
	name := "sort"
	if w.Rank {
		name = "rank"
	}
	unique := ""
	if len(w.UniqueKeys) > 0 {
		parts := make([]string, 0, len(w.UniqueKeys))
		for _, key := range w.UniqueKeys {
			if key == nil {
				parts = append(parts, "<nil>")
				continue
			}
			parts = append(parts, key.Description())
		}
		unique = strings.Join(parts, ",") + ";"
	}
	parts := make([]string, 0, len(w.Keys))
	for _, key := range w.Keys {
		if key.Expr == nil {
			parts = append(parts, "<nil>")
			continue
		}
		direction := "asc"
		if key.Descending {
			direction = "desc"
		}
		parts = append(parts, key.Expr.Description()+":"+direction)
	}
	return fmt.Sprintf("%s(%s%d,%s)", name, unique, w.Size, strings.Join(parts, ","))
}
func (w SortedWindowSpec) validate() error {
	if w.Size <= 0 || len(w.Keys) == 0 {
		return fmt.Errorf("esper: sort/rank window requires positive size and at least one key")
	}
	if !w.Rank && len(w.UniqueKeys) > 0 {
		return fmt.Errorf("esper: sort window cannot have unique keys")
	}
	if w.Rank {
		if w.requireUniqueKeys && len(w.UniqueKeys) == 0 {
			return fmt.Errorf("esper: rank window requires at least one unique-key expression")
		}
		for _, key := range w.UniqueKeys {
			if key == nil {
				return fmt.Errorf("esper: rank unique-key expression is required")
			}
		}
	}
	for _, key := range w.Keys {
		if key.Expr == nil {
			return fmt.Errorf("esper: sort/rank key expression is required")
		}
	}
	return nil
}

type UniqueWindowSpec struct {
	Key   Expr
	Keys  []Expr
	First bool
}

func Unique(key Expr) UniqueWindowSpec      { return UniqueWindowSpec{Key: key} }
func FirstUnique(key Expr) UniqueWindowSpec { return UniqueWindowSpec{Key: key, First: true} }

// UniqueBy and FirstUniqueBy are the multi-key Go forms of Esper's unique
// view. Keys are evaluated in declaration order and encoded with their value
// state, type and contents, so arrays and mixed numeric/string keys do not
// collapse merely because fmt.Stringer produces the same text.
func UniqueBy(keys ...Expr) UniqueWindowSpec {
	return UniqueWindowSpec{Keys: append([]Expr(nil), keys...)}
}

func FirstUniqueBy(keys ...Expr) UniqueWindowSpec {
	return UniqueWindowSpec{Keys: append([]Expr(nil), keys...), First: true}
}

func (UniqueWindowSpec) windowSpec() {}
func (w UniqueWindowSpec) description() string {
	parts := make([]string, 0, len(w.keyExpressions()))
	for _, key := range w.keyExpressions() {
		if key == nil {
			parts = append(parts, "<nil>")
			continue
		}
		parts = append(parts, key.Description())
	}
	name := "unique"
	if w.First {
		name = "first-unique"
	}
	return name + "(" + strings.Join(parts, ",") + ")"
}
func (w UniqueWindowSpec) validate() error {
	keys := w.keyExpressions()
	if len(keys) == 0 {
		return fmt.Errorf("esper: unique window key expression is required")
	}
	for _, key := range keys {
		if key == nil {
			return fmt.Errorf("esper: unique window key expression is required")
		}
	}
	return nil
}

func (w UniqueWindowSpec) keyExpressions() []Expr {
	if len(w.Keys) > 0 {
		return w.Keys
	}
	if w.Key == nil {
		return nil
	}
	return []Expr{w.Key}
}

type StreamSelector uint8

const (
	SelectIStream StreamSelector = iota
	SelectRStream
	SelectIRStream
)

type OutputPolicyKind uint8

const (
	OutputAllPolicy OutputPolicyKind = iota
	OutputFirstPolicy
	OutputLastPolicy
	OutputSnapshotPolicy
	OutputEveryPolicy
	OutputEveryTimePolicy
	OutputFirstEveryEventsPolicy
	OutputFirstEveryTimePolicy
	OutputLastEveryEventsPolicy
	OutputLastEveryTimePolicy
)

type OutputAfterKind uint8

const (
	OutputAfterNone OutputAfterKind = iota
	OutputAfterEventCount
	OutputAfterDuration
	OutputAfterCalendarKind
)

// OutputTerminationKind controls whether a context statement emits output
// when an initiated context partition is terminated.
type OutputTerminationKind uint8

const (
	OutputNoTermination OutputTerminationKind = iota
	OutputAndOnTermination
	OutputOnlyOnTermination
)

type OutputCalendarPeriod struct {
	Years  int
	Months int
	Days   int
}

type OutputVariableAssignment struct {
	Name string
	Expr Expr
}

func SetOutputVariable(name string, expression Expr) OutputVariableAssignment {
	return OutputVariableAssignment{Name: strings.TrimSpace(name), Expr: expression}
}

type OutputPolicy struct {
	Kind            OutputPolicyKind
	Snapshot        bool
	Count           int
	Interval        time.Duration
	After           OutputAfterKind
	AfterCount      int
	AfterDuration   time.Duration
	AfterCalendar   OutputCalendarPeriod
	Cron            *CronSchedule
	When            Expr
	Then            []OutputVariableAssignment
	Termination     OutputTerminationKind
	TerminationWhen Expr
	TerminationThen []OutputVariableAssignment
}

func OutputAll() OutputPolicy { return OutputPolicy{Kind: OutputAllPolicy} }

func OutputFirst(count int) OutputPolicy {
	return OutputPolicy{Kind: OutputFirstPolicy, Count: count}
}

// OutputFirstEveryEvents emits the first visible result immediately, then
// permits the next first result after count accepted input events. This is
// the chainable Go form of Esper's "output first every N events" policy.
func OutputFirstEveryEvents(count int) OutputPolicy {
	return OutputPolicy{Kind: OutputFirstEveryEventsPolicy, Count: count}
}

// OutputFirstEveryTime emits the first visible result immediately, then
// permits the next first result after interval on the engine's virtual clock.
func OutputFirstEveryTime(interval time.Duration) OutputPolicy {
	return OutputPolicy{Kind: OutputFirstEveryTimePolicy, Interval: interval}
}

// OutputLastEveryEvents emits the latest visible result after each count of
// accepted input events. It can be composed with OutputAfterEvents to model
// Esper's output-after plus last-every policy.
func OutputLastEveryEvents(count int) OutputPolicy {
	return OutputPolicy{Kind: OutputLastEveryEventsPolicy, Count: count}
}

// OutputLastEveryTime emits the latest pending result at each virtual-clock
// interval. It never starts a wall-clock goroutine.
func OutputLastEveryTime(interval time.Duration) OutputPolicy {
	return OutputPolicy{Kind: OutputLastEveryTimePolicy, Interval: interval}
}

func OutputLast() OutputPolicy { return OutputPolicy{Kind: OutputLastPolicy, Count: 1} }

func OutputSnapshot() OutputPolicy { return OutputPolicy{Kind: OutputSnapshotPolicy, Count: 1} }

// OutputWhenTerminated emits the current output policy when a context
// partition terminates and suppresses normal event-time output. For a
// non-snapshot policy, rows accumulated since the last output are flushed;
// OutputSnapshot() rebuilds the complete current statement state instead.
func OutputWhenTerminated(base ...OutputPolicy) OutputPolicy {
	policy := outputBasePolicy(base)
	policy.Termination = OutputOnlyOnTermination
	return policy
}

// OutputAndWhenTerminated composes normal output with an additional output at
// context termination. Count/time policies use a state snapshot at
// termination, while OutputAll/OutputWhen flush their pending deltas.
func OutputAndWhenTerminated(base OutputPolicy) OutputPolicy {
	base.Termination = OutputAndOnTermination
	return base
}

// OutputSnapshotWhenTerminated is the concise form for a context statement
// that emits a state snapshot only when its partition terminates.
func OutputSnapshotWhenTerminated() OutputPolicy {
	return OutputWhenTerminated(OutputSnapshot())
}

// OutputWhenTerminatedIf adds a variable-only termination condition and
// optional variable assignments to a termination-only output policy.
func OutputWhenTerminatedIf(condition Expr, assignments ...OutputVariableAssignment) OutputPolicy {
	policy := OutputWhenTerminated()
	policy.TerminationWhen = condition
	policy.TerminationThen = append([]OutputVariableAssignment(nil), assignments...)
	return policy
}

// OutputAndWhenTerminatedIf adds a variable-only termination condition and
// optional variable assignments to an existing output policy.
func OutputAndWhenTerminatedIf(base OutputPolicy, condition Expr, assignments ...OutputVariableAssignment) OutputPolicy {
	base.Termination = OutputAndOnTermination
	base.TerminationWhen = condition
	base.TerminationThen = append([]OutputVariableAssignment(nil), assignments...)
	return base
}

// OutputEvery buffers result rows/events until count visible results have
// accumulated, then delivers one deterministic batch.
func OutputEvery(count int) OutputPolicy { return OutputPolicy{Kind: OutputEveryPolicy, Count: count} }

// OutputEveryTime buffers result rows/events until the next virtual-clock
// interval. The timer is driven by Engine.AdvanceTime; it never starts a
// goroutine or depends on wall-clock sleeps.
func OutputEveryTime(interval time.Duration) OutputPolicy {
	return OutputPolicy{Kind: OutputEveryTimePolicy, Interval: interval}
}

// OutputSnapshotEvery emits the current statement state at each virtual-clock
// interval. It is the chainable Go form of Esper's "output snapshot every"
// policy; unlike OutputEveryTime it does not accumulate only the deltas seen
// since the previous tick.
func OutputSnapshotEvery(interval time.Duration) OutputPolicy {
	return OutputPolicy{Kind: OutputEveryTimePolicy, Interval: interval, Snapshot: true}
}

// OutputSnapshotEveryEvents emits the current statement state after each
// count accepted result items. It is the event-count form of Esper's
// "output snapshot every N events" policy.
func OutputSnapshotEveryEvents(count int) OutputPolicy {
	return OutputPolicy{Kind: OutputEveryPolicy, Count: count, Snapshot: true}
}

// OutputAfterEvents activates the supplied output policy after count accepted
// result items. With no base policy, OutputAll is used. A zero count activates
// the policy immediately, matching Esper's "after 0 events" form.
func OutputAfterEvents(count int, base ...OutputPolicy) OutputPolicy {
	policy := outputBasePolicy(base)
	policy.After = OutputAfterEventCount
	policy.AfterCount = count
	return policy
}

// OutputAfterTime activates the supplied output policy once duration has
// elapsed on the engine's virtual clock. With no base policy, OutputAll is
// used. Calendar-month semantics are intentionally separate and are not
// represented by time.Duration.
func OutputAfterTime(duration time.Duration, base ...OutputPolicy) OutputPolicy {
	policy := outputBasePolicy(base)
	policy.After = OutputAfterDuration
	policy.AfterDuration = duration
	return policy
}

// OutputAfterCalendar activates the supplied output policy after a calendar
// period measured with time.Time.AddDate. This preserves month/year boundaries
// instead of approximating them with a fixed duration.
func OutputAfterCalendar(years, months, days int, base ...OutputPolicy) OutputPolicy {
	policy := outputBasePolicy(base)
	policy.After = OutputAfterCalendarKind
	policy.AfterCalendar = OutputCalendarPeriod{Years: years, Months: months, Days: days}
	return policy
}

// OutputWhen gates a base output policy on a variable-only condition and can
// update registered variables after a non-empty output batch. Compose it with
// OutputAfterEvents/OutputAfterTime/OutputAfterCalendar when an activation gate
// is also required.
func OutputWhen(condition Expr, assignments ...OutputVariableAssignment) OutputPolicy {
	return OutputWhenWith(OutputAll(), condition, assignments...)
}

// OutputWhenWith composes a when/then gate with any base output policy.  It is
// useful for Esper forms such as snapshot-when, while OutputWhen keeps the
// common output-all form concise.
func OutputWhenWith(base OutputPolicy, condition Expr, assignments ...OutputVariableAssignment) OutputPolicy {
	base.When = condition
	base.Then = append([]OutputVariableAssignment(nil), assignments...)
	return base
}

func outputBasePolicy(base []OutputPolicy) OutputPolicy {
	if len(base) > 0 {
		return base[0]
	}
	return OutputAll()
}

type querySpec struct {
	name                       string
	statementUserObject        any
	selector                   StreamSelector
	selections                 []Selection
	routeTarget                string
	tableTarget                string
	sink                       Sink
	contextName                string
	output                     OutputPolicy
	distinct                   bool
	discardPartialsOnMatch     bool
	suppressOverlappingMatches bool
	orderBy                    []SortKey
	limit                      int
	offset                     int
	allowNoSink                bool
}

// QueryOption configures statement metadata and output policy. Options are
// applied during Build so a fluent chain remains easy to compose.
type QueryOption func(*querySpec)

func StatementName(name string) QueryOption {
	return func(spec *querySpec) { spec.name = name }
}

// WithStatementUserObject attaches an opaque caller-owned value to the
// statement metadata exposed by CurrentEvaluationContext. It is not evaluated
// as part of the rule and does not enter Plan canonical identity.
func WithStatementUserObject(value any) QueryOption {
	return func(spec *querySpec) { spec.statementUserObject = value }
}

func WithOldStream() QueryOption {
	return func(spec *querySpec) { spec.selector = SelectIRStream }
}

func WithRemoveStreamOnly() QueryOption {
	return func(spec *querySpec) { spec.selector = SelectRStream }
}

func WithNewStreamOnly() QueryOption {
	return func(spec *querySpec) { spec.selector = SelectIStream }
}

func WithSink(sink Sink) QueryOption {
	return func(spec *querySpec) { spec.sink = sink }
}

// RouteTo is the option form of InsertInto for callers composing a generic
// query option list.
func RouteTo(eventType string) QueryOption {
	return func(spec *querySpec) { spec.routeTarget = strings.TrimSpace(eventType) }
}

// IntoTable configures an aggregate query to maintain a registered table as
// its materialized current state. It is the fluent-API equivalent of an
// Esper into-table declaration; the table is synchronized even when the
// statement emits no listener batch for a change.
func IntoTable(tableName string) QueryOption {
	return func(spec *querySpec) { spec.tableTarget = strings.TrimSpace(tableName) }
}

func WithContext(name string) QueryOption {
	return func(spec *querySpec) { spec.contextName = name }
}

func WithOutput(policy OutputPolicy) QueryOption {
	return func(spec *querySpec) { spec.output = policy }
}

func WithDistinct() QueryOption {
	return func(spec *querySpec) { spec.distinct = true }
}

// DiscardPartialsOnMatch clears all still-active pattern branches after a
// match completes. It is the fluent counterpart of Esper's
// @DiscardPartialsOnMatch pattern policy.
func DiscardPartialsOnMatch() QueryOption {
	return func(spec *querySpec) { spec.discardPartialsOnMatch = true }
}

// SuppressOverlappingMatches keeps pattern state alive but suppresses a
// completed result whose captured events overlap an already emitted result.
// It is the fluent counterpart of Esper's @SuppressOverlappingMatches policy.
func SuppressOverlappingMatches() QueryOption {
	return func(spec *querySpec) { spec.suppressOverlappingMatches = true }
}

func OrderBy(keys ...SortKey) QueryOption {
	return func(spec *querySpec) { spec.orderBy = append([]SortKey(nil), keys...) }
}

func Limit(count int) QueryOption {
	return func(spec *querySpec) { spec.limit = count }
}

func Offset(count int) QueryOption {
	return func(spec *querySpec) { spec.offset = count }
}

func newQuery(env *Environment, node *streamNode, selections []Selection, options ...QueryOption) Query {
	spec := querySpec{selector: SelectIStream, selections: append([]Selection(nil), selections...), output: OutputAll()}
	for _, option := range options {
		if option != nil {
			option(&spec)
		}
	}
	return Query{env: env, input: node, selections: spec.selections, routeTarget: spec.routeTarget, tableTarget: spec.tableTarget, name: spec.name, statementUserObject: spec.statementUserObject, selector: spec.selector, sink: spec.sink, contextName: spec.contextName, output: spec.output, distinct: spec.distinct, discardPartialsOnMatch: spec.discardPartialsOnMatch, suppressOverlappingMatches: spec.suppressOverlappingMatches, orderBy: append([]SortKey(nil), spec.orderBy...), limit: spec.limit, offset: spec.offset}
}

func SelectOnce(env *Environment, selections ...Selection) Query {
	return Query{env: env, selections: append([]Selection(nil), selections...), sourceLess: true}
}

// Query is an immutable logical statement definition.
type Query struct {
	env                        *Environment
	input                      *streamNode
	aggregate                  *aggregateDefinition
	join                       *joinDefinition
	pattern                    *patternDefinition
	rowRecog                   *rowRecogDefinition
	trigger                    *triggerDefinition
	onDemand                   *onDemandDefinition
	selections                 []Selection
	joinSelections             []JoinSelection
	joinWhere                  Expr
	patternSelections          []Selection
	routeTarget                string
	tableTarget                string
	name                       string
	statementUserObject        any
	selector                   StreamSelector
	sink                       Sink
	contextName                string
	sourceLess                 bool
	output                     OutputPolicy
	distinct                   bool
	discardPartialsOnMatch     bool
	suppressOverlappingMatches bool
	orderBy                    []SortKey
	limit                      int
	offset                     int
}

func (q Query) Name() string { return q.name }

func (q Query) description() string {
	if q.onDemand != nil {
		parts := []string{q.input.describe(), q.onDemand.description()}
		if q.contextName != "" {
			parts = append(parts, "context("+q.contextName+")")
		}
		return strings.Join(parts, " -> ")
	}
	if q.sourceLess {
		parts := []string{"select-once"}
		selections := make([]string, 0, len(q.selections))
		for _, selection := range q.selections {
			selections = append(selections, selection.description())
		}
		parts = append(parts, "select("+strings.Join(selections, ",")+")")
		parts = appendQueryModifiers(parts, q)
		return strings.Join(parts, " -> ")
	}
	if q.trigger != nil {
		parts := []string{q.trigger.description()}
		if q.contextName != "" {
			parts = append(parts, "context("+q.contextName+")")
		}
		return strings.Join(parts, " -> ")
	}
	if q.pattern != nil {
		parts := []string{q.pattern.description()}
		selections := make([]string, 0, len(q.patternSelections))
		for _, selection := range q.patternSelections {
			selections = append(selections, selection.description())
		}
		parts = append(parts, "select("+strings.Join(selections, ",")+")")
		if q.contextName != "" {
			parts = append(parts, "context("+q.contextName+")")
		}
		parts = append(parts, "output("+outputDescription(q.output)+")")
		parts = appendQueryModifiers(parts, q)
		return strings.Join(parts, " -> ")
	}
	if q.rowRecog != nil {
		parts := []string{q.rowRecog.input.describe(), q.rowRecog.description()}
		selections := make([]string, 0, len(q.patternSelections))
		for _, selection := range q.patternSelections {
			selections = append(selections, selection.description())
		}
		parts = append(parts, "measures("+strings.Join(selections, ",")+")")
		if q.contextName != "" {
			parts = append(parts, "context("+q.contextName+")")
		}
		parts = append(parts, "output("+outputDescription(q.output)+")")
		parts = appendQueryModifiers(parts, q)
		return strings.Join(parts, " -> ")
	}
	if q.aggregate != nil {
		inputDescription := "<nil-stream>"
		if q.aggregate.join != nil {
			inputDescription = describeJoinDefinition(q.aggregate.join)
		} else if q.aggregate.input != nil {
			inputDescription = q.aggregate.input.describe()
		}
		parts := []string{inputDescription}
		if len(q.aggregate.groupBy) > 0 {
			keys := make([]string, 0, len(q.aggregate.groupBy))
			for _, key := range q.aggregate.groupBy {
				if key == nil {
					keys = append(keys, "<nil>")
					continue
				}
				keys = append(keys, key.Description())
			}
			groupingName := "group-by"
			switch q.aggregate.grouping {
			case aggregateGroupingRollup:
				groupingName = "group-by-rollup"
			case aggregateGroupingCube:
				groupingName = "group-by-cube"
			}
			if q.aggregate.grouping != aggregateGroupingSets {
				parts = append(parts, groupingName+"("+strings.Join(keys, ",")+")")
			} else {
				sets := make([]string, 0, len(q.aggregate.groupingSets))
				for _, set := range q.aggregate.groupingSets {
					setKeys := make([]string, 0, len(set))
					for _, index := range set {
						if index >= 0 && index < len(keys) {
							setKeys = append(setKeys, keys[index])
						} else {
							setKeys = append(setKeys, fmt.Sprintf("#%d", index))
						}
					}
					sets = append(sets, "["+strings.Join(setKeys, ",")+"]")
				}
				parts = append(parts, "group-by-grouping-sets("+strings.Join(sets, ";")+")")
			}
		} else if q.aggregate.grouping == aggregateGroupingSets {
			parts = append(parts, "group-by-grouping-sets()")
		}
		selections := make([]string, 0, len(q.aggregate.selections))
		for _, selection := range q.aggregate.selections {
			selections = append(selections, selection.description())
		}
		parts = append(parts, "aggregate("+strings.Join(selections, ",")+")")
		if q.tableTarget != "" {
			parts = append(parts, "into-table("+q.tableTarget+")")
		}
		if q.aggregate.where != nil {
			parts = append(parts, "where("+q.aggregate.where.Description()+")")
		}
		if q.aggregate.having != nil {
			parts = append(parts, "having("+q.aggregate.having.Description()+")")
		}
		if q.contextName != "" {
			parts = append(parts, "context("+q.contextName+")")
		}
		parts = append(parts, "output("+outputDescription(q.output)+")")
		parts = appendQueryModifiers(parts, q)
		return strings.Join(parts, " -> ")
	}
	if q.join != nil {
		parts := []string{describeJoinDefinition(q.join)}
		if q.joinWhere != nil {
			parts = append(parts, "where("+q.joinWhere.Description()+")")
		}
		if len(q.joinSelections) == 0 {
			return strings.Join(parts, " -> ")
		}
		selectionDescriptions := make([]string, 0, len(q.joinSelections))
		for _, selection := range q.joinSelections {
			selectionDescriptions = append(selectionDescriptions, selection.Name+"="+selection.Expr.Description())
		}
		if q.contextName != "" {
			parts = append(parts, "context("+q.contextName+")")
		}
		parts = append(parts, "select("+strings.Join(selectionDescriptions, ",")+")")
		parts = appendQueryModifiers(parts, q)
		return strings.Join(parts, " -> ")
	}
	parts := []string{q.input.describe()}
	if len(q.selections) > 0 {
		selectionDescriptions := make([]string, 0, len(q.selections))
		for _, selection := range q.selections {
			selectionDescriptions = append(selectionDescriptions, selection.description())
		}
		parts = append(parts, "select("+strings.Join(selectionDescriptions, ",")+")")
	} else {
		parts = append(parts, "select(*)")
	}
	if q.contextName != "" {
		parts = append(parts, "context("+q.contextName+")")
	}
	parts = append(parts, "output("+outputDescription(q.output)+")")
	parts = appendQueryModifiers(parts, q)
	return strings.Join(parts, " -> ")
}

func appendQueryModifiers(parts []string, query Query) []string {
	if query.distinct {
		parts = append(parts, "distinct")
	}
	if query.discardPartialsOnMatch {
		parts = append(parts, "discard-partials-on-match")
	}
	if query.suppressOverlappingMatches {
		parts = append(parts, "suppress-overlapping-matches")
	}
	if len(query.orderBy) > 0 {
		keys := make([]string, 0, len(query.orderBy))
		for _, key := range query.orderBy {
			direction := "asc"
			if key.Descending {
				direction = "desc"
			}
			if key.Expr == nil {
				keys = append(keys, "<nil>"+":"+direction)
			} else {
				keys = append(keys, key.Expr.Description()+":"+direction)
			}
		}
		parts = append(parts, "order-by("+strings.Join(keys, ",")+")")
	}
	if query.offset > 0 {
		parts = append(parts, fmt.Sprintf("offset(%d)", query.offset))
	}
	if query.limit > 0 {
		parts = append(parts, fmt.Sprintf("limit(%d)", query.limit))
	}
	return parts
}

func joinDefinitionSources(definition *joinDefinition) []*streamNode {
	if definition == nil {
		return nil
	}
	if len(definition.sources) > 0 {
		return definition.sources
	}
	return []*streamNode{definition.left, definition.right}
}

func streamHasMethodDependencies(node *streamNode) bool {
	base, err := sourceNode(node)
	return err == nil && base.kind == streamMethod && base.method != nil && len(base.method.dependencies) > 0
}

// methodJoinEvaluationOrder validates explicit subordinate method
// dependencies and returns a stable topological order. Source declaration
// order remains untouched and therefore continues to define join indexes.
func methodJoinEvaluationOrder(definition *joinDefinition) ([]int, error) {
	sources := joinDefinitionSources(definition)
	bases := make([]*streamNode, len(sources))
	byName := make(map[string][]int, len(sources))
	for index, source := range sources {
		base, err := sourceNode(source)
		if err != nil {
			return nil, err
		}
		bases[index] = base
		name := strings.TrimSpace(base.sourceName)
		byName[name] = append(byName[name], index)
	}

	indegree := make([]int, len(sources))
	edges := make([][]int, len(sources))
	for methodIndex, base := range bases {
		if base.kind != streamMethod || base.method == nil {
			continue
		}
		seen := make(map[string]struct{}, len(base.method.dependencies))
		for _, dependency := range base.method.dependencies {
			name := strings.TrimSpace(dependency)
			if name == "" {
				return nil, NewError(ErrorInvalidRule, fmt.Sprintf("method source %q has a blank dependency", base.sourceName))
			}
			if _, exists := seen[name]; exists {
				return nil, NewError(ErrorInvalidRule, fmt.Sprintf("method source %q duplicates dependency %q", base.sourceName, name))
			}
			seen[name] = struct{}{}
			matches := byName[name]
			if len(matches) == 0 {
				return nil, NewError(ErrorUnknownName, fmt.Sprintf("method source %q references unknown dependency %q", base.sourceName, name))
			}
			if len(matches) > 1 {
				return nil, NewError(ErrorInvalidRule, fmt.Sprintf("method source %q dependency %q is ambiguous", base.sourceName, name))
			}
			dependencyIndex := matches[0]
			if dependencyIndex == methodIndex {
				return nil, NewError(ErrorInvalidRule, fmt.Sprintf("method source %q cannot depend on itself", base.sourceName))
			}
			if len(definition.edges) > 0 {
				if err := validateChainedMethodDependency(methodIndex, dependencyIndex, name, base.sourceName, definition.edges, containsHistoricalSource(sources[dependencyIndex])); err != nil {
					return nil, err
				}
			} else {
				switch definition.kind {
				case JoinLeftOuter:
					if dependencyIndex > methodIndex {
						return nil, NewError(ErrorInvalidRule, fmt.Sprintf("method source %q dependency %q cannot be satisfied by the left outer join", base.sourceName, name))
					}
				case JoinRightOuter:
					if dependencyIndex < methodIndex {
						return nil, NewError(ErrorInvalidRule, fmt.Sprintf("method source %q dependency %q cannot be satisfied by the right outer join", base.sourceName, name))
					}
				}
			}
			edges[dependencyIndex] = append(edges[dependencyIndex], methodIndex)
			indegree[methodIndex]++
		}
	}

	order := make([]int, 0, len(sources))
	used := make([]bool, len(sources))
	for len(order) < len(sources) {
		selected := -1
		for index := range sources {
			if !used[index] && indegree[index] == 0 {
				selected = index
				break
			}
		}
		if selected < 0 {
			return nil, NewError(ErrorInvalidRule, "method source dependencies contain a cycle")
		}
		used[selected] = true
		order = append(order, selected)
		for _, dependent := range edges[selected] {
			indegree[dependent]--
		}
	}
	return order, nil
}

func validateChainedMethodDependency(methodIndex, dependencyIndex int, dependencyName, methodName string, edges []joinEdgeDefinition, dependencyHistorical bool) error {
	if dependencyIndex < methodIndex {
		// A source introduced by a right/full edge can appear independently of
		// the prior left relation. Java rejects making a later left-optional
		// method subordinate to that branch. A source introduced by left outer
		// is different: its descendants may remain on the same optional branch.
		if dependencyHistorical && dependencyIndex > 0 {
			origin := edges[dependencyIndex-1].kind
			if origin == JoinRightOuter || origin == JoinFullOuter {
				for edgeIndex := dependencyIndex; edgeIndex < methodIndex && edgeIndex < len(edges); edgeIndex++ {
					if edges[edgeIndex].kind == JoinLeftOuter {
						return NewError(ErrorInvalidRule, fmt.Sprintf("method source %q dependency %q cannot or may not be satisfied by the join chain", methodName, dependencyName))
					}
				}
			}
		}
		return nil
	}
	// Inner joins are reorderable and right/full outer edges can be driven by
	// the newly introduced right side. A left outer edge requires its left
	// relation first and therefore cannot satisfy a reverse dependency.
	for edgeIndex := methodIndex; edgeIndex < dependencyIndex && edgeIndex < len(edges); edgeIndex++ {
		if edges[edgeIndex].kind == JoinLeftOuter {
			return NewError(ErrorInvalidRule, fmt.Sprintf("method source %q dependency %q cannot or may not be satisfied by join edge %d", methodName, dependencyName, edgeIndex))
		}
	}
	return nil
}

func describeJoinDefinition(definition *joinDefinition) string {
	if definition == nil {
		return "<nil-join>"
	}
	sources := joinDefinitionSources(definition)
	if len(definition.edges) > 0 {
		if len(sources) == 0 {
			return "join-chain(<empty>)"
		}
		parts := []string{describeJoinSource(definition, sources, 0)}
		for index, edge := range definition.edges {
			conditionDescriptions := make([]string, 0, len(edge.conditions))
			for _, condition := range edge.conditions {
				conditionDescriptions = append(conditionDescriptions, joinConditionDescription(condition))
			}
			source := "<missing-source>"
			if index+1 < len(sources) {
				source = describeJoinSource(definition, sources, index+1)
			}
			parts = append(parts, joinKindDescription(edge.kind)+"("+source+",on="+strings.Join(conditionDescriptions, " and ")+")")
		}
		return "join-chain(" + strings.Join(parts, " -> ") + ")"
	}
	joinName := joinKindDescription(definition.kind)
	sourceDescriptions := make([]string, 0, len(sources))
	for index := range sources {
		sourceDescriptions = append(sourceDescriptions, describeJoinSource(definition, sources, index))
	}
	conditions := joinDefinitionConditions(definition)
	conditionDescriptions := make([]string, 0, len(conditions))
	for _, condition := range conditions {
		conditionDescriptions = append(conditionDescriptions, joinConditionDescription(condition))
	}
	return joinName + "-join(" + strings.Join(sourceDescriptions, ",") + ") -> on(" + strings.Join(conditionDescriptions, " and ") + ")"
}

func describeJoinSource(definition *joinDefinition, sources []*streamNode, index int) string {
	if index < 0 || index >= len(sources) || sources[index] == nil {
		return "<missing-source>"
	}
	description := sources[index].describe()
	if index < len(definition.unidirectional) && definition.unidirectional[index] {
		description += ".unidirectional()"
	}
	return description
}

func joinDefinitionHasUnidirectional(definition *joinDefinition) bool {
	if definition == nil {
		return false
	}
	for _, flag := range definition.unidirectional {
		if flag {
			return true
		}
	}
	return false
}

func joinDefinitionUnidirectionalCount(definition *joinDefinition) int {
	if definition == nil {
		return 0
	}
	count := 0
	for _, flag := range definition.unidirectional {
		if flag {
			count++
		}
	}
	return count
}

func joinKindDescription(kind JoinKind) string {
	switch kind {
	case JoinInner:
		return "inner"
	case JoinLeftOuter:
		return "left-outer"
	case JoinRightOuter:
		return "right-outer"
	case JoinFullOuter:
		return "full-outer"
	default:
		return fmt.Sprintf("unknown-%d", kind)
	}
}

func joinDefinitionConditions(definition *joinDefinition) []JoinCondition {
	if definition == nil {
		return nil
	}
	if len(definition.edges) > 0 {
		var conditions []JoinCondition
		for _, edge := range definition.edges {
			conditions = append(conditions, edge.conditions...)
		}
		return conditions
	}
	if len(definition.conditions) > 0 {
		return definition.conditions
	}
	if !joinConditionPresent(definition.condition) {
		return nil
	}
	return []JoinCondition{definition.condition}
}

func joinConditionPresent(condition JoinCondition) bool {
	return condition.Left != nil || condition.Right != nil || condition.sourceSet || len(condition.all) > 0 || len(condition.any) > 0
}

func joinConditionDescription(condition JoinCondition) string {
	if len(condition.all) > 0 {
		parts := make([]string, 0, len(condition.all))
		for _, child := range condition.all {
			parts = append(parts, joinConditionDescription(child))
		}
		return "all(" + strings.Join(parts, ",") + ")"
	}
	if len(condition.any) > 0 {
		parts := make([]string, 0, len(condition.any))
		for _, child := range condition.any {
			parts = append(parts, joinConditionDescription(child))
		}
		return "any(" + strings.Join(parts, ",") + ")"
	}
	left := "<nil>"
	if condition.Left != nil {
		left = condition.Left.Description()
	}
	right := "<nil>"
	if condition.Right != nil {
		right = condition.Right.Description()
	}
	operator := map[JoinComparison]string{
		JoinEqual:          "=",
		JoinNotEqual:       "!=",
		JoinLess:           "<",
		JoinLessOrEqual:    "<=",
		JoinGreater:        ">",
		JoinGreaterOrEqual: ">=",
	}[condition.Comparison]
	if operator == "" {
		operator = "?"
	}
	return left + operator + right
}

func outputDescription(policy OutputPolicy) string {
	base := "all"
	switch policy.Kind {
	case OutputFirstPolicy:
		base = fmt.Sprintf("first(%d)", policy.Count)
	case OutputEveryPolicy:
		if policy.Snapshot {
			base = fmt.Sprintf("snapshot-every-events(%d)", policy.Count)
		} else {
			base = fmt.Sprintf("every(%d)", policy.Count)
		}
	case OutputEveryTimePolicy:
		if policy.Snapshot {
			base = fmt.Sprintf("snapshot-every-time(%s)", policy.Interval)
		} else {
			base = fmt.Sprintf("every-time(%s)", policy.Interval)
		}
	case OutputFirstEveryEventsPolicy:
		base = fmt.Sprintf("first-every-events(%d)", policy.Count)
	case OutputFirstEveryTimePolicy:
		base = fmt.Sprintf("first-every-time(%s)", policy.Interval)
	case OutputLastEveryEventsPolicy:
		base = fmt.Sprintf("last-every-events(%d)", policy.Count)
	case OutputLastEveryTimePolicy:
		base = fmt.Sprintf("last-every-time(%s)", policy.Interval)
	case OutputLastPolicy:
		base = "last"
	case OutputSnapshotPolicy:
		base = "snapshot"
	}
	if policy.Cron != nil {
		base = fmt.Sprintf("at(%s)->%s", policy.Cron.description(), base)
	}
	switch policy.After {
	case OutputAfterEventCount:
		base = fmt.Sprintf("after-events(%d)->%s", policy.AfterCount, base)
	case OutputAfterDuration:
		base = fmt.Sprintf("after-time(%s)->%s", policy.AfterDuration, base)
	case OutputAfterCalendarKind:
		period := policy.AfterCalendar
		base = fmt.Sprintf("after-calendar(%dY%dM%dD)->%s", period.Years, period.Months, period.Days, base)
	default:
	}
	if policy.When != nil {
		parts := []string{policy.When.Description()}
		for _, assignment := range policy.Then {
			expression := "<nil>"
			if assignment.Expr != nil {
				expression = assignment.Expr.Description()
			}
			parts = append(parts, assignment.Name+"="+expression)
		}
		base = fmt.Sprintf("when(%s)", strings.Join(parts, ";")) + "->" + base
	}
	if policy.TerminationWhen != nil {
		parts := []string{policy.TerminationWhen.Description()}
		for _, assignment := range policy.TerminationThen {
			expression := "<nil>"
			if assignment.Expr != nil {
				expression = assignment.Expr.Description()
			}
			parts = append(parts, assignment.Name+"="+expression)
		}
		base = fmt.Sprintf("on-termination-when(%s)", strings.Join(parts, ";")) + "->" + base
	}
	switch policy.Termination {
	case OutputAndOnTermination:
		base += "+termination"
	case OutputOnlyOnTermination:
		base = "termination->" + base
	}
	return base
}
