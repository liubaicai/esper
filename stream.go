package esper

import (
	"fmt"
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
)

type streamNode struct {
	kind       streamNodeKind
	input      *streamNode
	sourceName string
	sourceType reflect.Type
	historical *historicalDefinition
	predicate  Expr
	window     WindowSpec
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
	having       Expression[bool]
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
	env       *Environment
	left      *streamNode
	right     *streamNode
	condition JoinCondition
	kind      JoinKind
}

// JoinInput is the type-erased source handle used by JoinMany. It preserves
// the logical stream node while allowing streams with different Go event
// types to participate in one analyzable join graph.
type JoinInput struct {
	env  *Environment
	node *streamNode
}

func JoinSource[T any](stream Stream[T]) JoinInput {
	return JoinInput{env: stream.env, node: stream.node}
}

func JoinRecordSource(stream RecordStream) JoinInput {
	return JoinInput{env: stream.env, node: stream.node}
}

type MultiJoinStream struct {
	env        *Environment
	sources    []*streamNode
	conditions []JoinCondition
	kind       JoinKind
}

// JoinMany creates a multi-stream join. Conditions should use
// OnSourcesEqual/OnSourcesCompare so each expression is tied to a source
// index. Left/right/full outer variants retain unmatched tuples with zero
// Event placeholders for missing sources.
func JoinMany(inputs ...JoinInput) MultiJoinStream {
	sources := make([]*streamNode, 0, len(inputs))
	var env *Environment
	for _, input := range inputs {
		if env == nil {
			env = input.env
		}
		sources = append(sources, input.node)
	}
	return MultiJoinStream{env: env, sources: sources, kind: JoinInner}
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
			sources:    append([]*streamNode(nil), j.sources...),
			conditions: append([]JoinCondition(nil), j.conditions...),
			kind:       j.kind,
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
}

type joinDefinition struct {
	left       *streamNode
	right      *streamNode
	condition  JoinCondition
	sources    []*streamNode
	conditions []JoinCondition
	kind       JoinKind
}

func Join[L, R any](left Stream[L], right Stream[R], condition JoinCondition) JoinStream[L, R] {
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

// Aggregate starts a tuple-aware aggregate over the two-stream join result.
// Use JoinField or JoinEventValue in aggregate expressions to keep source
// scope explicit in the Go API.
func (j JoinStream[L, R]) Aggregate(selections ...Selection) AggregateStream {
	definition := j.Select().definition
	return AggregateStream{env: j.env, node: j.left, join: definition, selections: append([]Selection(nil), selections...)}
}

func (j JoinStream[L, R]) GroupBy(keys ...Expr) AggregateStream {
	definition := j.Select().definition
	return AggregateStream{env: j.env, node: j.left, join: definition, groupBy: append([]Expr(nil), keys...)}
}

func (j JoinStream[L, R]) Select(selections ...JoinSelection) JoinQuery {
	return JoinQuery{env: j.env, definition: &joinDefinition{left: j.left, right: j.right, condition: j.condition, sources: []*streamNode{j.left, j.right}, conditions: []JoinCondition{j.condition}, kind: j.kind}, selections: append([]JoinSelection(nil), selections...)}
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
		env:            j.env,
		join:           j.definition,
		joinSelections: append([]JoinSelection(nil), j.selections...),
		routeTarget:    spec.routeTarget,
		name:           spec.name,
		selector:       spec.selector,
		sink:           spec.sink,
		contextName:    spec.contextName,
		output:         spec.output,
		distinct:       spec.distinct,
		orderBy:        append([]SortKey(nil), spec.orderBy...),
		limit:          spec.limit,
		offset:         spec.offset,
	}
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
			having:       a.having,
		},
		name:        spec.name,
		selector:    spec.selector,
		sink:        spec.sink,
		contextName: spec.contextName,
		output:      spec.output,
		distinct:    spec.distinct,
		orderBy:     append([]SortKey(nil), spec.orderBy...),
		limit:       spec.limit,
		offset:      spec.offset,
	}
}

// To attaches a Go sink to aggregate results. It mirrors Stream.To while
// retaining the aggregate builder's grouping, having and output semantics.
func (a AggregateStream) To(sink Sink, options ...QueryOption) Query {
	options = append(options, WithSink(sink))
	return a.Query(options...)
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
	name        string
	selector    StreamSelector
	selections  []Selection
	routeTarget string
	tableTarget string
	sink        Sink
	contextName string
	output      OutputPolicy
	distinct    bool
	orderBy     []SortKey
	limit       int
	offset      int
	allowNoSink bool
}

// QueryOption configures statement metadata and output policy. Options are
// applied during Build so a fluent chain remains easy to compose.
type QueryOption func(*querySpec)

func StatementName(name string) QueryOption {
	return func(spec *querySpec) { spec.name = name }
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
	return Query{env: env, input: node, selections: spec.selections, routeTarget: spec.routeTarget, tableTarget: spec.tableTarget, name: spec.name, selector: spec.selector, sink: spec.sink, contextName: spec.contextName, output: spec.output, distinct: spec.distinct, orderBy: append([]SortKey(nil), spec.orderBy...), limit: spec.limit, offset: spec.offset}
}

func SelectOnce(env *Environment, selections ...Selection) Query {
	return Query{env: env, selections: append([]Selection(nil), selections...), sourceLess: true}
}

// Query is an immutable logical statement definition.
type Query struct {
	env               *Environment
	input             *streamNode
	aggregate         *aggregateDefinition
	join              *joinDefinition
	pattern           *patternDefinition
	rowRecog          *rowRecogDefinition
	trigger           *triggerDefinition
	selections        []Selection
	joinSelections    []JoinSelection
	patternSelections []Selection
	routeTarget       string
	tableTarget       string
	name              string
	selector          StreamSelector
	sink              Sink
	contextName       string
	sourceLess        bool
	output            OutputPolicy
	distinct          bool
	orderBy           []SortKey
	limit             int
	offset            int
}

func (q Query) Name() string { return q.name }

func (q Query) description() string {
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
		joinName := "inner"
		switch q.join.kind {
		case JoinLeftOuter:
			joinName = "left-outer"
		case JoinRightOuter:
			joinName = "right-outer"
		case JoinFullOuter:
			joinName = "full-outer"
		}
		sources := joinDefinitionSources(q.join)
		sourceDescriptions := make([]string, 0, len(sources))
		for _, source := range sources {
			sourceDescriptions = append(sourceDescriptions, source.describe())
		}
		parts := []string{joinName + "-join(" + strings.Join(sourceDescriptions, ",") + ")"}
		conditions := joinDefinitionConditions(q.join)
		conditionDescriptions := make([]string, 0, len(conditions))
		for _, condition := range conditions {
			conditionDescriptions = append(conditionDescriptions, joinConditionDescription(condition))
		}
		parts = append(parts, "on("+strings.Join(conditionDescriptions, " and ")+")")
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

func describeJoinDefinition(definition *joinDefinition) string {
	if definition == nil {
		return "<nil-join>"
	}
	joinName := "inner"
	switch definition.kind {
	case JoinLeftOuter:
		joinName = "left-outer"
	case JoinRightOuter:
		joinName = "right-outer"
	case JoinFullOuter:
		joinName = "full-outer"
	}
	sources := joinDefinitionSources(definition)
	sourceDescriptions := make([]string, 0, len(sources))
	for _, source := range sources {
		sourceDescriptions = append(sourceDescriptions, source.describe())
	}
	conditions := joinDefinitionConditions(definition)
	conditionDescriptions := make([]string, 0, len(conditions))
	for _, condition := range conditions {
		conditionDescriptions = append(conditionDescriptions, joinConditionDescription(condition))
	}
	return joinName + "-join(" + strings.Join(sourceDescriptions, ",") + ") -> on(" + strings.Join(conditionDescriptions, " and ") + ")"
}

func joinDefinitionConditions(definition *joinDefinition) []JoinCondition {
	if definition == nil {
		return nil
	}
	if len(definition.conditions) > 0 {
		return definition.conditions
	}
	return []JoinCondition{definition.condition}
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
