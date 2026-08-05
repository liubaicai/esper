package esper

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type DataflowState uint8

const (
	DataflowInstantiated DataflowState = iota
	DataflowRunning
	DataflowComplete
	DataflowCanceled
)

// DataflowSignal is the control-plane value that travels through a dataflow
// independently of event rows.  The public method keeps the contract open to
// application-defined signals while FinalMarker and WindowMarker mirror
// Esper's built-in signal types.
type DataflowSignal interface {
	SignalType() string
}

// FinalMarker marks the end of a finite source batch.  Receiving a final
// marker does not implicitly cancel a running dataflow; operators decide how
// to flush their state, matching Esper's signal semantics.
type FinalMarker struct{}

func (FinalMarker) SignalType() string { return "final" }

func isDataflowFinalMarker(signal DataflowSignal) bool {
	switch signal.(type) {
	case FinalMarker, *FinalMarker:
		return true
	default:
		return false
	}
}

// WindowMarker marks a logical window boundary without ending the flow.
type WindowMarker struct{}

func (WindowMarker) SignalType() string { return "window" }

// CustomSignal carries an application-defined signal kind and optional
// payload.  It is useful for Go operators that need a typed control-plane
// message without inventing a new marker type.
type CustomSignal struct {
	Type    string
	Payload any
}

func (s CustomSignal) SignalType() string { return s.Type }

// DataflowSignalHandler receives control-plane signals in operator order.
// Returning an error aborts the current dataflow submission.
type DataflowSignalHandler func(context.Context, DataflowSignal) error

// DataflowRecord is the closed union of values emitted by Esper's built-in
// event-oriented operators: Event for an event stream and Row for a
// projection. It gives Filter and Select one typed contract while retaining
// the concrete value shape at runtime.
type DataflowRecord interface {
	dataflowRecord()
}

func (Event) dataflowRecord() {}
func (Row) dataflowRecord()   {}

// DataflowOperatorContext describes the graph and operator instance supplied
// to a custom operator factory.  A fresh runtime is created for every
// DataflowInstance, matching Esper's factory/operator split.
type DataflowOperatorContext struct {
	DataflowName string
	InstanceID   string
	OperatorName string
	OperatorNum  int
	Properties   map[string]any
}

// DataflowEmission is one value emitted from a custom operator. Port defaults
// to "out" when empty, which keeps the common case concise while allowing
// explicit multi-port routing.
type DataflowEmission struct {
	Port  string
	Value any
}

// Emit creates a value on the conventional output port.
func Emit(value any) DataflowEmission { return DataflowEmission{Port: "out", Value: value} }

// EmitPort creates a value on a named output port.
func EmitPort(port string, value any) DataflowEmission {
	return DataflowEmission{Port: port, Value: value}
}

// DataflowInput describes a value and the named input port that received it.
type DataflowInput struct {
	Port  string
	Value any
}

// DataflowEmitter is a handle to a captive Emitter source.  It is returned by
// DataflowInstance.CaptiveEmitter and lets a caller feed a running graph one
// value at a time without routing the value through the engine's ordinary
// event bus.
type DataflowEmitter struct {
	instance *DataflowInstance
	name     string
	allowRaw bool
}

// Submit injects one event value into the captive emitter's outgoing graph.
// Registered Go event values and already materialized Event values are both
// accepted.
func (e *DataflowEmitter) Submit(ctx context.Context, value any) error {
	if e == nil || e.instance == nil {
		return NewError(ErrorState, "nil dataflow emitter")
	}
	if e.allowRaw {
		return e.instance.submitSourcePort(ctx, e.name, "out", value)
	}
	return e.instance.submitEmitter(ctx, e.name, value)
}

// SubmitPort emits a raw value on a named custom-source output port. It is
// useful for fan-out sources; ordinary captive Emitters use Submit because
// their conventional output is the single "out" port.
func (e *DataflowEmitter) SubmitPort(ctx context.Context, port string, value any) error {
	if e == nil || e.instance == nil {
		return NewError(ErrorState, "nil dataflow emitter")
	}
	if !e.allowRaw {
		return NewError(ErrorInvalidRule, "named output ports are only available to custom sources")
	}
	return e.instance.submitSourcePort(ctx, e.name, port, value)
}

// SubmitSignal injects a control-plane signal through the captive emitter.
func (e *DataflowEmitter) SubmitSignal(ctx context.Context, signal DataflowSignal) error {
	if e == nil || e.instance == nil {
		return NewError(ErrorState, "nil dataflow emitter")
	}
	if signal == nil {
		return NewError(ErrorInvalidRule, "dataflow signal is nil")
	}
	if e.allowRaw {
		return e.instance.submitSourcePort(ctx, e.name, "out", signal)
	}
	return e.instance.submitEmitter(ctx, e.name, signal)
}

// DataflowOperatorRuntime is the minimum contract for a custom transform or
// sink. Returning no values consumes the input; returning values forwards them
// to the matching connected output ports.
type DataflowOperatorRuntime interface {
	Process(context.Context, DataflowInput) ([]DataflowEmission, error)
}

// DataflowOperatorSignalRuntime is optional. Operators that implement it can
// flush or otherwise handle a signal. Operators without it pass the signal
// through unchanged.
type DataflowOperatorSignalRuntime interface {
	OnSignal(context.Context, DataflowSignal) ([]DataflowEmission, error)
}

// DataflowOperatorOpener is an optional per-instance open hook. Open and
// Close are intentionally independent: a source or operator may need only
// one side of the lifecycle contract.
type DataflowOperatorOpener interface {
	Open(context.Context) error
}

// DataflowOperatorCloser is an optional per-instance close hook.
type DataflowOperatorCloser interface {
	Close(context.Context) error
}

// DataflowOperatorLifecycle is the convenience contract for runtimes that
// implement both lifecycle hooks.
type DataflowOperatorLifecycle interface {
	DataflowOperatorOpener
	DataflowOperatorCloser
}

// DataflowSourceRuntime produces values into a graph. A source receives its
// per-instance emitter at run time, so it can emit both registered Event
// values and raw values declared by typed output ports.
type DataflowSourceRuntime interface {
	Run(context.Context, *DataflowEmitter) error
}

// DataflowSourceFactory creates one source runtime for every dataflow
// instance. Source runtimes may implement either lifecycle hook.
type DataflowSourceFactory func(DataflowOperatorContext) (DataflowSourceRuntime, error)

// DataflowParameterContext identifies one operator parameter while an
// instance is being created. A provider may return a value with true to
// override the definition value, or false to retain the definition value.
type DataflowParameterContext struct {
	DataflowName  string
	InstanceID    string
	OperatorName  string
	OperatorNum   int
	ParameterName string
	DefaultValue  any
}

// DataflowParameterProvider supplies per-instance operator properties without
// embedding deployment-specific values in a reusable graph definition.
type DataflowParameterProvider func(DataflowParameterContext) (any, bool)

// DataflowOperatorProvider can replace a custom operator runtime at
// instantiation time. It is useful for dependency injection and test doubles.
type DataflowOperatorProvider func(DataflowOperatorContext) (DataflowOperatorRuntime, error)

// DataflowSourceProvider is the source counterpart to DataflowOperatorProvider.
type DataflowSourceProvider func(DataflowOperatorContext) (DataflowSourceRuntime, error)

// DataflowOperatorFactory creates one runtime operator instance.
type DataflowOperatorFactory func(DataflowOperatorContext) (DataflowOperatorRuntime, error)

// DataflowErrorPolicy controls what happens after an operator reports an
// error. Fail is the safe default and returns the error to the caller;
// Continue invokes the optional exception handler and drops the failed work
// item when the handler returns nil. A handler can always force failure by
// returning its own error.
type DataflowErrorPolicy uint8

const (
	DataflowErrorFail DataflowErrorPolicy = iota
	DataflowErrorContinue
)

// DataflowError is the structured error delivered to an exception handler.
// It keeps graph/operator identity without requiring callers to parse a
// formatted error string.
type DataflowError struct {
	DataflowName string
	InstanceID   string
	OperatorName string
	Err          error
}

func (e DataflowError) Error() string {
	if e.OperatorName == "" {
		return fmt.Sprintf("dataflow %q instance %q: %v", e.DataflowName, e.InstanceID, e.Err)
	}
	return fmt.Sprintf("dataflow %q instance %q operator %q: %v", e.DataflowName, e.InstanceID, e.OperatorName, e.Err)
}

func (e DataflowError) Unwrap() error { return e.Err }

// DataflowExceptionHandler observes operator failures. Returning nil is a
// recovery decision and is only effective with DataflowErrorContinue;
// returning an error always aborts the current submission.
type DataflowExceptionHandler func(context.Context, DataflowError) error

// DataflowOptions configures one instantiated dataflow. Definitions remain
// immutable and reusable; these options belong to the instance lifecycle.
type DataflowOptions struct {
	InstanceID        string
	UserObject        any
	ErrorPolicy       DataflowErrorPolicy
	ExceptionHandler  DataflowExceptionHandler
	ParameterProvider DataflowParameterProvider
	OperatorProvider  DataflowOperatorProvider
	SourceProvider    DataflowSourceProvider
}

func (o DataflowOptions) validate() error {
	if o.ErrorPolicy != DataflowErrorFail && o.ErrorPolicy != DataflowErrorContinue {
		return NewError(ErrorInvalidRule, "unknown dataflow error policy")
	}
	return nil
}

type DataflowOperatorKind string

const (
	BeaconSourceKind      DataflowOperatorKind = "BeaconSource"
	EmitterKind           DataflowOperatorKind = "Emitter"
	EPStatementSourceKind DataflowOperatorKind = "EPStatementSource"
	EventBusSourceKind    DataflowOperatorKind = "EventBusSource"
	EventBusSinkKind      DataflowOperatorKind = "EventBusSink"
	FilterKind            DataflowOperatorKind = "Filter"
	SelectKind            DataflowOperatorKind = "Select"
	LogSinkKind           DataflowOperatorKind = "LogSink"
	CustomKind            DataflowOperatorKind = "Custom"
	CustomSourceKind      DataflowOperatorKind = "CustomSource"
)

// DataflowSelectOptions controls the state and trigger policy of a built-in
// Select operator. A zero value preserves the original per-input emission
// behavior. TimeWindow retains Event inputs for the supplied duration, while
// OutputSnapshotEvery emits the current projection on each virtual-clock
// interval instead of on every input. IterateOnFinalMarker retains a finite
// Event batch and emits one row per GroupBy key when FinalMarker arrives.
// PreserveInput returns the input DataflowRecord unchanged when no output
// event type is configured, or uses it as the base for an event projection
// when OutputEventType is set. This models Esper's select-star and wrapper
// output without exposing EPL or Java EventBean internals.
type DataflowSelectOptions struct {
	TimeWindow           time.Duration
	OutputSnapshotEvery  time.Duration
	IterateOnFinalMarker bool
	GroupBy              []Expr
	OrderBy              []SortKey
	OutputEventType      string
	PreserveInput        bool
}

func (o DataflowSelectOptions) validate() error {
	if o.TimeWindow < 0 {
		return NewError(ErrorInvalidRule, "dataflow select time window cannot be negative")
	}
	if o.OutputSnapshotEvery < 0 {
		return NewError(ErrorInvalidRule, "dataflow select snapshot interval cannot be negative")
	}
	if !o.IterateOnFinalMarker && (len(o.GroupBy) > 0 || len(o.OrderBy) > 0) {
		return NewError(ErrorInvalidRule, "dataflow select group/order options require iterate-on-final-marker")
	}
	for index, expression := range o.GroupBy {
		if expression == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("dataflow select group-by expression %d is nil", index))
		}
	}
	for index, key := range o.OrderBy {
		if key.Expr == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("dataflow select order-by expression %d is nil", index))
		}
	}
	if o.IterateOnFinalMarker && (o.TimeWindow > 0 || o.OutputSnapshotEvery > 0) {
		return NewError(ErrorInvalidRule, "dataflow select iterate-on-final-marker cannot combine with time or output snapshots")
	}
	if (o.OutputEventType != "" || o.PreserveInput) && (o.TimeWindow > 0 || o.OutputSnapshotEvery > 0 || o.IterateOnFinalMarker) {
		return NewError(ErrorInvalidRule, "dataflow event output cannot combine with time, snapshot, or final-marker state")
	}
	if o.TimeWindow == 0 && o.OutputSnapshotEvery == 0 {
		return nil
	}
	if o.TimeWindow > 0 && o.TimeWindow < time.Nanosecond {
		return NewError(ErrorInvalidRule, "dataflow select time window must have nanosecond precision")
	}
	if o.OutputSnapshotEvery > 0 && o.OutputSnapshotEvery < time.Nanosecond {
		return NewError(ErrorInvalidRule, "dataflow select snapshot interval must have nanosecond precision")
	}
	return nil
}

func cloneDataflowSelectOptions(options DataflowSelectOptions) DataflowSelectOptions {
	options.GroupBy = append([]Expr(nil), options.GroupBy...)
	options.OrderBy = append([]SortKey(nil), options.OrderBy...)
	return options
}

// DataflowJoinKind selects the row-availability policy for a multi-input
// SelectJoin. Inner joins wait until every input has a value; the outer forms
// retain a Null-valued tuple for the selected edge when the other inputs are
// not available.
type DataflowJoinKind uint8

const (
	DataflowJoinInner DataflowJoinKind = iota
	DataflowJoinFullOuter
	DataflowJoinLeftOuter
	DataflowJoinRightOuter
)

// DataflowJoinRetention controls whether each input keeps only its latest
// event or all events for Cartesian combinations.
type DataflowJoinRetention uint8

const (
	DataflowJoinLastEvent DataflowJoinRetention = iota
	DataflowJoinKeepAll
)

// DataflowJoinOptions describes the named input ports of SelectJoin. Input
// port i is named "in<i>" and is connected with ConnectInput or ConnectPorts.
type DataflowJoinOptions struct {
	Inputs     int
	Kind       DataflowJoinKind
	Retention  DataflowJoinRetention
	Conditions []JoinCondition
}

// On appends analyzable join predicates to a SelectJoin. Multiple predicates
// are combined with AND, matching MultiJoinStream.On while keeping the
// dataflow definition free of EPL strings.
func (o DataflowJoinOptions) On(conditions ...JoinCondition) DataflowJoinOptions {
	o.Conditions = append(cloneJoinConditions(o.Conditions), conditions...)
	return o
}

func (o DataflowJoinOptions) validate() error {
	if o.Inputs < 2 {
		return NewError(ErrorInvalidRule, "dataflow select join requires at least two inputs")
	}
	if o.Kind != DataflowJoinInner && o.Kind != DataflowJoinFullOuter && o.Kind != DataflowJoinLeftOuter && o.Kind != DataflowJoinRightOuter {
		return NewError(ErrorInvalidRule, "unknown dataflow select join kind")
	}
	if o.Retention != DataflowJoinLastEvent && o.Retention != DataflowJoinKeepAll {
		return NewError(ErrorInvalidRule, "unknown dataflow select join retention")
	}
	for index, condition := range o.Conditions {
		if err := validateDataflowJoinCondition(condition, o.Inputs); err != nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("dataflow select join condition %d: %v", index, err))
		}
	}
	return nil
}

// DataflowOperatorOptions contains definition-time properties and the names
// that may be resolved by a DataflowParameterProvider at instantiation.
// Properties and ParameterNames are copied when the graph is built.
type DataflowOperatorOptions struct {
	Properties     map[string]any
	ParameterNames []string
}

func (o DataflowOperatorOptions) validate() error {
	for name := range o.Properties {
		if name == "" {
			return NewError(ErrorInvalidRule, "dataflow operator property name cannot be empty")
		}
	}
	seen := make(map[string]struct{}, len(o.ParameterNames))
	for _, name := range o.ParameterNames {
		if name == "" {
			return NewError(ErrorInvalidRule, "dataflow operator parameter name cannot be empty")
		}
		if _, exists := seen[name]; exists {
			return NewError(ErrorInvalidRule, fmt.Sprintf("dataflow operator parameter %q is declared more than once", name))
		}
		seen[name] = struct{}{}
	}
	return nil
}

func cloneDataflowOperatorOptions(options DataflowOperatorOptions) DataflowOperatorOptions {
	options.Properties = maps.Clone(options.Properties)
	options.ParameterNames = append([]string(nil), options.ParameterNames...)
	return options
}

func validateDataflowJoinCondition(condition JoinCondition, inputs int) error {
	if len(condition.all) > 0 && len(condition.any) > 0 {
		return fmt.Errorf("condition cannot combine all and any")
	}
	if len(condition.all) > 0 || len(condition.any) > 0 {
		children := condition.all
		logic := "all"
		if len(condition.any) > 0 {
			children = condition.any
			logic = "any"
		}
		if len(children) == 0 {
			return fmt.Errorf("%s condition requires at least one child", logic)
		}
		for index, child := range children {
			if err := validateDataflowJoinCondition(child, inputs); err != nil {
				return fmt.Errorf("%s child %d: %w", logic, index, err)
			}
		}
		return nil
	}
	if condition.Left == nil || condition.Right == nil {
		return fmt.Errorf("condition requires left and right expressions")
	}
	leftSource, rightSource := joinConditionSources(condition)
	if leftSource < 0 || leftSource >= inputs || rightSource < 0 || rightSource >= inputs {
		return fmt.Errorf("condition source indexes (%d,%d) are outside %d inputs", leftSource, rightSource, inputs)
	}
	if condition.Comparison > JoinGreaterOrEqual {
		return fmt.Errorf("unknown join comparison %d", condition.Comparison)
	}
	return nil
}

func cloneJoinCondition(condition JoinCondition) JoinCondition {
	result := condition
	if len(condition.all) > 0 {
		result.all = cloneJoinConditions(condition.all)
	}
	if len(condition.any) > 0 {
		result.any = cloneJoinConditions(condition.any)
	}
	return result
}

func cloneJoinConditions(conditions []JoinCondition) []JoinCondition {
	if len(conditions) == 0 {
		return nil
	}
	result := make([]JoinCondition, len(conditions))
	for index, condition := range conditions {
		result[index] = cloneJoinCondition(condition)
	}
	return result
}

func cloneDataflowJoinOptions(options DataflowJoinOptions) DataflowJoinOptions {
	options.Conditions = cloneJoinConditions(options.Conditions)
	return options
}

// DataflowPort declares a named operator port and, optionally, the Go value
// type accepted or emitted by that port. A nil Type is a wildcard, which is
// useful for built-in operators whose runtime value is an Event or Row.
type DataflowPort struct {
	Name string
	Type reflect.Type
}

// DataflowPortOf creates a typed port descriptor without exposing reflect in
// ordinary fluent graph definitions.
func DataflowPortOf[T any](name string) DataflowPort {
	return DataflowPort{Name: name, Type: typeOf[T]()}
}

type DataflowOperator struct {
	Name            string
	Kind            DataflowOperatorKind
	Events          []any
	EventType       string
	Predicate       Expr
	Selections      []Selection
	Log             func(context.Context, any) error
	Statement       *Statement
	Signal          DataflowSignalHandler
	Factory         DataflowOperatorFactory
	InputPorts      []string
	OutputPorts     []string
	InputPortTypes  map[string]reflect.Type
	OutputPortTypes map[string]reflect.Type
	SourceFactory   DataflowSourceFactory
	Properties      map[string]any
	ParameterNames  []string
	SelectOptions   DataflowSelectOptions
	JoinOptions     DataflowJoinOptions
	JoinConfigured  bool
}

// DataflowEdge connects an operator output port to an operator input port.
// The default ports are conventionally named "out" and "in"; the names are
// retained in the definition so typed/multi-port operators can be added
// without changing the graph model.
type DataflowEdge struct {
	From     string
	FromPort string
	To       string
	ToPort   string
}

type DataflowDefinition struct {
	name      string
	operators []DataflowOperator
	edges     []DataflowEdge
}

func (d DataflowDefinition) Name() string { return d.name }

func (d DataflowDefinition) Operators() []DataflowOperator {
	result := append([]DataflowOperator(nil), d.operators...)
	for index := range result {
		result[index].Events = append([]any(nil), result[index].Events...)
		result[index].Selections = append([]Selection(nil), result[index].Selections...)
		result[index].InputPorts = append([]string(nil), result[index].InputPorts...)
		result[index].OutputPorts = append([]string(nil), result[index].OutputPorts...)
		result[index].InputPortTypes = cloneDataflowPortTypes(result[index].InputPortTypes)
		result[index].OutputPortTypes = cloneDataflowPortTypes(result[index].OutputPortTypes)
		result[index].Properties = maps.Clone(result[index].Properties)
		result[index].ParameterNames = append([]string(nil), result[index].ParameterNames...)
		result[index].SelectOptions = cloneDataflowSelectOptions(result[index].SelectOptions)
		result[index].JoinOptions = cloneDataflowJoinOptions(result[index].JoinOptions)
	}
	return result
}

func (d DataflowDefinition) Edges() []DataflowEdge {
	return append([]DataflowEdge(nil), d.edges...)
}

type DataflowBuilder struct {
	env       *Environment
	name      string
	operators []DataflowOperator
	edges     []DataflowEdge
	signals   map[string]DataflowSignalHandler
}

func DefineDataflow(env *Environment, name string) DataflowBuilder {
	return DataflowBuilder{env: env, name: name}
}

func (b DataflowBuilder) add(operator DataflowOperator) DataflowBuilder {
	b.operators = append(append([]DataflowOperator(nil), b.operators...), operator)
	return b
}

// Connect links the conventional output port of one operator to the
// conventional input port of another operator.
func (b DataflowBuilder) Connect(from, to string) DataflowBuilder {
	return b.ConnectPorts(from, "out", to, "in")
}

// ConnectPorts links explicitly named ports. Build validates that declared
// ports exist; built-in operators expose the conventional "in"/"out" ports.
func (b DataflowBuilder) ConnectPorts(from, fromPort, to, toPort string) DataflowBuilder {
	result := b
	result.edges = append([]DataflowEdge(nil), b.edges...)
	result.edges = append(result.edges, DataflowEdge{From: from, FromPort: fromPort, To: to, ToPort: toPort})
	return result
}

// ConnectInput connects a source to the indexed SelectJoin input port.
func (b DataflowBuilder) ConnectInput(from, to string, input int) DataflowBuilder {
	return b.ConnectPorts(from, "out", to, dataflowJoinInputPort(input))
}

func dataflowJoinInputPort(input int) string {
	return fmt.Sprintf("in%d", input)
}

func (b DataflowBuilder) BeaconSource(name string, events ...any) DataflowBuilder {
	return b.add(DataflowOperator{Name: name, Kind: BeaconSourceKind, Events: append([]any(nil), events...)})
}

func (b DataflowBuilder) Emitter(name string) DataflowBuilder {
	return b.add(DataflowOperator{Name: name, Kind: EmitterKind})
}

func (b DataflowBuilder) EPStatementSource(name string, statement *Statement) DataflowBuilder {
	return b.add(DataflowOperator{Name: name, Kind: EPStatementSourceKind, Statement: statement})
}

func (b DataflowBuilder) EventBusSource(name, eventType string) DataflowBuilder {
	return b.add(DataflowOperator{Name: name, Kind: EventBusSourceKind, EventType: eventType})
}

func (b DataflowBuilder) EventBusSink(name, eventType string) DataflowBuilder {
	return b.add(DataflowOperator{Name: name, Kind: EventBusSinkKind, EventType: eventType})
}

func (b DataflowBuilder) Filter(name string, predicate Expr) DataflowBuilder {
	return b.add(DataflowOperator{Name: name, Kind: FilterKind, Predicate: predicate})
}

// FilterWithPorts adds the two-output Filter form. Values satisfying
// predicate use passPort; all other values use rejectPort. The graph must use
// ConnectPorts to route both named outputs.
func (b DataflowBuilder) FilterWithPorts(name string, predicate Expr, passPort, rejectPort string) DataflowBuilder {
	return b.add(DataflowOperator{
		Name:        name,
		Kind:        FilterKind,
		Predicate:   predicate,
		OutputPorts: []string{passPort, rejectPort},
	})
}

func (b DataflowBuilder) Select(name string, selections ...Selection) DataflowBuilder {
	return b.SelectWithOptions(name, DataflowSelectOptions{}, selections...)
}

// SelectWithOptions adds a built-in projection with optional stateful
// time-window and virtual-clock snapshot semantics. It keeps selection
// expressions analyzable and avoids embedding EPL strings in a dataflow
// definition.
func (b DataflowBuilder) SelectWithOptions(name string, options DataflowSelectOptions, selections ...Selection) DataflowBuilder {
	return b.add(DataflowOperator{
		Name:          name,
		Kind:          SelectKind,
		Selections:    append([]Selection(nil), selections...),
		SelectOptions: cloneDataflowSelectOptions(options),
	})
}

// SelectPassThrough forwards the input Event or Row unchanged. It is the
// fluent equivalent of a select-star projection when the graph should retain
// the existing DataflowRecord representation.
func (b DataflowBuilder) SelectPassThrough(name string) DataflowBuilder {
	return b.SelectWithOptions(name, DataflowSelectOptions{PreserveInput: true})
}

// SelectEvent emits a registered event schema, preserving fields from the
// input Event/Row and overlaying any named selections. With no selections it
// is a typed select-star projection; selections model additional wrapper
// properties such as `hello` while retaining the source fields.
func (b DataflowBuilder) SelectEvent(name, eventType string, selections ...Selection) DataflowBuilder {
	return b.SelectWithOptions(name, DataflowSelectOptions{
		OutputEventType: eventType,
		PreserveInput:   true,
	}, selections...)
}

// SelectTimeWindow is the concise chain form for a Select retaining Event
// inputs for duration and emitting a projection on input and expiry changes.
func (b DataflowBuilder) SelectTimeWindow(name string, duration time.Duration, selections ...Selection) DataflowBuilder {
	return b.SelectWithOptions(name, DataflowSelectOptions{TimeWindow: duration}, selections...)
}

// SelectSnapshotEvery is the concise chain form for a Select that emits one
// current projection per virtual-clock interval.
func (b DataflowBuilder) SelectSnapshotEvery(name string, interval time.Duration, selections ...Selection) DataflowBuilder {
	return b.SelectWithOptions(name, DataflowSelectOptions{OutputSnapshotEvery: interval}, selections...)
}

// SelectIterate retains the finite input batch and emits grouped projections
// only when a FinalMarker reaches the operator. OrderBy uses the same SortKey,
// Ascending and Descending descriptors as ordinary fluent queries.
func (b DataflowBuilder) SelectIterate(name string, groupBy []Expr, orderBy []SortKey, selections ...Selection) DataflowBuilder {
	return b.SelectWithOptions(name, DataflowSelectOptions{
		IterateOnFinalMarker: true,
		GroupBy:              groupBy,
		OrderBy:              orderBy,
	}, selections...)
}

// SelectIterateOnFinalMarker is the concise ungrouped form of SelectIterate.
func (b DataflowBuilder) SelectIterateOnFinalMarker(name string, selections ...Selection) DataflowBuilder {
	return b.SelectIterate(name, nil, nil, selections...)
}

// SelectJoin adds an analyzable multi-input projection. JoinField and
// JoinEventValue use the same zero-based input index as the generated in<i>
// ports, keeping source scope explicit in Go code.
func (b DataflowBuilder) SelectJoin(name string, options DataflowJoinOptions, selections ...Selection) DataflowBuilder {
	capacity := 0
	if options.Inputs > 0 {
		capacity = options.Inputs
	}
	ports := make([]string, 0, capacity)
	for index := 0; index < options.Inputs; index++ {
		ports = append(ports, dataflowJoinInputPort(index))
	}
	return b.add(DataflowOperator{
		Name:           name,
		Kind:           SelectKind,
		Selections:     append([]Selection(nil), selections...),
		InputPorts:     ports,
		JoinOptions:    cloneDataflowJoinOptions(options),
		JoinConfigured: true,
	})
}

func (b DataflowBuilder) LogSink(name string, logger func(context.Context, any) error) DataflowBuilder {
	return b.add(DataflowOperator{Name: name, Kind: LogSinkKind, Log: logger})
}

// Custom adds a Go-native operator factory. The factory is called once for
// each instantiated dataflow and its runtime participates in graph processing
// and optional signal/lifecycle callbacks.
func (b DataflowBuilder) Custom(name string, factory DataflowOperatorFactory) DataflowBuilder {
	return b.CustomWithOptions(name, factory, DataflowOperatorOptions{})
}

// CustomWithOptions adds a conventional custom operator with definition
// properties and optional parameter names for instance-time injection.
func (b DataflowBuilder) CustomWithOptions(name string, factory DataflowOperatorFactory, options DataflowOperatorOptions) DataflowBuilder {
	return b.CustomPortsWithOptions(name, factory, []string{"in"}, []string{"out"}, options)
}

// CustomPorts adds a custom operator with explicit named input and output
// ports. Runtime emissions are delivered only to edges matching their port.
func (b DataflowBuilder) CustomPorts(name string, factory DataflowOperatorFactory, inputs, outputs []string) DataflowBuilder {
	return b.CustomPortsWithOptions(name, factory, inputs, outputs, DataflowOperatorOptions{})
}

// CustomPortsWithOptions is the property-aware form of CustomPorts.
func (b DataflowBuilder) CustomPortsWithOptions(name string, factory DataflowOperatorFactory, inputs, outputs []string, options DataflowOperatorOptions) DataflowBuilder {
	return b.add(DataflowOperator{
		Name:           name,
		Kind:           CustomKind,
		Factory:        factory,
		InputPorts:     append([]string(nil), inputs...),
		OutputPorts:    append([]string(nil), outputs...),
		Properties:     maps.Clone(options.Properties),
		ParameterNames: append([]string(nil), options.ParameterNames...),
	})
}

// CustomTypedPorts is the typed counterpart to CustomPorts. Port names remain
// explicit for graph readability while the generic descriptors add compile
// time intent and Build/runtime assignability checks.
func (b DataflowBuilder) CustomTypedPorts(name string, factory DataflowOperatorFactory, inputs, outputs []DataflowPort) DataflowBuilder {
	return b.CustomTypedPortsWithOptions(name, factory, inputs, outputs, DataflowOperatorOptions{})
}

// CustomTypedPortsWithOptions is the typed property-aware form of
// CustomTypedPorts.
func (b DataflowBuilder) CustomTypedPortsWithOptions(name string, factory DataflowOperatorFactory, inputs, outputs []DataflowPort, options DataflowOperatorOptions) DataflowBuilder {
	inputNames, inputTypes := dataflowPortSpecs(inputs)
	outputNames, outputTypes := dataflowPortSpecs(outputs)
	return b.add(DataflowOperator{
		Name:            name,
		Kind:            CustomKind,
		Factory:         factory,
		InputPorts:      inputNames,
		OutputPorts:     outputNames,
		InputPortTypes:  inputTypes,
		OutputPortTypes: outputTypes,
		Properties:      maps.Clone(options.Properties),
		ParameterNames:  append([]string(nil), options.ParameterNames...),
	})
}

// CustomSource adds an idiomatic Go source operator. The source runs once per
// instantiated dataflow and emits into the graph through the supplied
// DataflowEmitter. Unlike a captive Emitter, a custom source can own its
// lifecycle and complete the graph when Run returns.
func (b DataflowBuilder) CustomSource(name string, factory DataflowSourceFactory) DataflowBuilder {
	return b.CustomSourceWithOptions(name, factory, DataflowOperatorOptions{})
}

// CustomSourceWithOptions adds a conventional custom source with
// instance-time properties and parameter resolution.
func (b DataflowBuilder) CustomSourceWithOptions(name string, factory DataflowSourceFactory, options DataflowOperatorOptions) DataflowBuilder {
	return b.CustomTypedSourceWithOptions(name, factory, []DataflowPort{{Name: "out"}}, options)
}

// CustomTypedSource is the typed counterpart to CustomSource. Source outputs
// are validated against downstream input ports at Build time and against
// actual values while the graph is running.
func (b DataflowBuilder) CustomTypedSource(name string, factory DataflowSourceFactory, outputs []DataflowPort) DataflowBuilder {
	return b.CustomTypedSourceWithOptions(name, factory, outputs, DataflowOperatorOptions{})
}

// CustomTypedSourceWithOptions is the typed property-aware form of
// CustomTypedSource.
func (b DataflowBuilder) CustomTypedSourceWithOptions(name string, factory DataflowSourceFactory, outputs []DataflowPort, options DataflowOperatorOptions) DataflowBuilder {
	outputNames, outputTypes := dataflowPortSpecs(outputs)
	return b.add(DataflowOperator{
		Name:            name,
		Kind:            CustomSourceKind,
		OutputPorts:     outputNames,
		OutputPortTypes: outputTypes,
		SourceFactory:   factory,
		Properties:      maps.Clone(options.Properties),
		ParameterNames:  append([]string(nil), options.ParameterNames...),
	})
}

// OnSignal registers a control-plane handler for a named operator.  Signals
// still continue along the operator's outgoing edges after the handler runs,
// allowing one operator to observe a marker while downstream operators flush
// their own state as well.
func (b DataflowBuilder) OnSignal(operator string, handler DataflowSignalHandler) DataflowBuilder {
	result := b
	result.signals = make(map[string]DataflowSignalHandler, len(b.signals)+1)
	for name, existing := range b.signals {
		result.signals[name] = existing
	}
	result.signals[operator] = handler
	return result
}

func (b DataflowBuilder) Build() (DataflowDefinition, error) {
	if b.env == nil {
		return DataflowDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	if b.name == "" {
		return DataflowDefinition{}, NewError(ErrorInvalidRule, "dataflow name is required")
	}
	if len(b.operators) == 0 {
		return DataflowDefinition{}, NewError(ErrorInvalidRule, "dataflow requires an operator")
	}
	seen := make(map[string]struct{}, len(b.operators))
	operatorsByName := make(map[string]DataflowOperator, len(b.operators))
	operators := make([]DataflowOperator, 0, len(b.operators))
	for _, original := range b.operators {
		operator := inferDataflowBuiltinPorts(original)
		if operator.Name == "" || operator.Kind == "" {
			return DataflowDefinition{}, NewError(ErrorInvalidRule, "dataflow operator requires a name and kind")
		}
		if _, exists := seen[operator.Name]; exists {
			return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow duplicates operator %q", operator.Name))
		}
		seen[operator.Name] = struct{}{}
		operatorsByName[operator.Name] = operator
		operators = append(operators, operator)
		if err := validateDataflowPorts(operator); err != nil {
			return DataflowDefinition{}, err
		}
		if operator.Kind == CustomKind || operator.Kind == CustomSourceKind {
			if err := (DataflowOperatorOptions{Properties: operator.Properties, ParameterNames: operator.ParameterNames}).validate(); err != nil {
				return DataflowDefinition{}, WrapError(ErrorInvalidRule, "dataflow operator "+operator.Name, err)
			}
		}
		switch operator.Kind {
		case FilterKind:
			if operator.Predicate == nil || operator.Predicate.Type() != typeOf[bool]() {
				return DataflowDefinition{}, NewError(ErrorTypeMismatch, fmt.Sprintf("dataflow filter %q requires bool predicate", operator.Name))
			}
			if len(operator.OutputPorts) > 2 {
				return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow filter %q supports at most two output ports", operator.Name))
			}
			if len(operator.OutputPorts) == 2 && len(b.edges) == 0 {
				return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow filter %q with two output ports requires graph edges", operator.Name))
			}
		case SelectKind:
			if err := operator.SelectOptions.validate(); err != nil {
				return DataflowDefinition{}, WrapError(ErrorInvalidRule, "dataflow select "+operator.Name, err)
			}
			if operator.SelectOptions.OutputEventType != "" {
				schema, ok := b.env.Schema(operator.SelectOptions.OutputEventType)
				if !ok {
					return DataflowDefinition{}, NewError(ErrorUnknownName, fmt.Sprintf("dataflow select %q references unknown output event type %q", operator.Name, operator.SelectOptions.OutputEventType))
				}
				for index, selection := range operator.Selections {
					if _, exists := schema.Field(selection.Name); !exists && !schema.AllowsDynamicProperties() {
						return DataflowDefinition{}, NewError(ErrorUnknownName, fmt.Sprintf("dataflow select %q projection %d references unknown output event field %q", operator.Name, index, selection.Name))
					}
				}
			}
			if operator.JoinConfigured {
				if err := operator.JoinOptions.validate(); err != nil {
					return DataflowDefinition{}, WrapError(ErrorInvalidRule, "dataflow select join "+operator.Name, err)
				}
				if len(b.edges) == 0 {
					return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow select join %q requires graph input edges", operator.Name))
				}
			}
			if len(operator.Selections) == 0 && !operator.SelectOptions.PreserveInput {
				return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow select %q requires projections", operator.Name))
			}
			if operator.SelectOptions.PreserveInput && operator.SelectOptions.OutputEventType == "" && len(operator.Selections) > 0 {
				return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow select %q pass-through cannot combine selections without an output event type", operator.Name))
			}
			for index, selection := range operator.Selections {
				if selection.Name == "" || selection.Expr == nil {
					return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow select %q projection %d requires name and expression", operator.Name, index))
				}
			}
		case EventBusSourceKind:
			if operator.EventType == "" {
				return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow event-bus source %q requires event type", operator.Name))
			}
			if _, ok := b.env.Schema(operator.EventType); !ok {
				return DataflowDefinition{}, NewError(ErrorUnknownName, fmt.Sprintf("dataflow event-bus source %q references unknown event type %q", operator.Name, operator.EventType))
			}
		case EventBusSinkKind:
			if operator.EventType == "" {
				return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow event-bus sink %q requires event type", operator.Name))
			}
			if _, ok := b.env.Schema(operator.EventType); !ok {
				return DataflowDefinition{}, NewError(ErrorUnknownName, fmt.Sprintf("dataflow event-bus sink %q references unknown event type %q", operator.Name, operator.EventType))
			}
		case EPStatementSourceKind:
			if operator.Statement == nil {
				return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow statement source %q requires statement", operator.Name))
			}
		case CustomKind:
			if operator.Factory == nil {
				return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow custom operator %q requires a factory", operator.Name))
			}
		case CustomSourceKind:
			if operator.SourceFactory == nil {
				return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow custom source %q requires a factory", operator.Name))
			}
		}
	}
	inferDataflowConnectedBuiltinPorts(operators, operatorsByName, b.edges)
	for name, handler := range b.signals {
		found := false
		for index := range operators {
			if operators[index].Name != name {
				continue
			}
			operators[index].Signal = handler
			found = true
			break
		}
		if !found {
			return DataflowDefinition{}, NewError(ErrorUnknownName, fmt.Sprintf("dataflow signal handler references unknown operator %q", name))
		}
	}
	if len(b.edges) > 0 {
		adjacency := make(map[string][]string, len(b.operators))
		indegree := make(map[string]int, len(b.operators))
		seenEdges := make(map[DataflowEdge]struct{}, len(b.edges))
		seenJoinInputs := make(map[string]map[string]struct{})
		for _, operator := range operators {
			if operator.Kind == SelectKind && operator.JoinConfigured {
				seenJoinInputs[operator.Name] = make(map[string]struct{}, len(operator.InputPorts))
			}
		}
		for _, edge := range b.edges {
			if edge.From == "" || edge.To == "" || edge.FromPort == "" || edge.ToPort == "" {
				return DataflowDefinition{}, NewError(ErrorInvalidRule, "dataflow edge requires source, source port, target and target port")
			}
			if edge.From == edge.To {
				return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow edge cannot loop operator %q to itself", edge.From))
			}
			if _, ok := seen[edge.From]; !ok {
				return DataflowDefinition{}, NewError(ErrorUnknownName, fmt.Sprintf("dataflow edge references unknown source operator %q", edge.From))
			}
			if _, ok := seen[edge.To]; !ok {
				return DataflowDefinition{}, NewError(ErrorUnknownName, fmt.Sprintf("dataflow edge references unknown target operator %q", edge.To))
			}
			if !dataflowPortAllowed(operatorsByName[edge.From], true, edge.FromPort) {
				return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow edge references unknown output port %q on operator %q", edge.FromPort, edge.From))
			}
			if !dataflowPortAllowed(operatorsByName[edge.To], false, edge.ToPort) {
				return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow edge references unknown input port %q on operator %q", edge.ToPort, edge.To))
			}
			if outputType := dataflowPortType(operatorsByName[edge.From], true, edge.FromPort); outputType != nil {
				if inputType := dataflowPortType(operatorsByName[edge.To], false, edge.ToPort); inputType != nil && !dataflowTypesAssignable(outputType, inputType) {
					return DataflowDefinition{}, NewError(ErrorTypeMismatch, fmt.Sprintf("dataflow edge %q:%q -> %q:%q cannot assign %s to %s", edge.From, edge.FromPort, edge.To, edge.ToPort, outputType, inputType))
				}
			}
			if _, duplicate := seenEdges[edge]; duplicate {
				return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow duplicates edge %q:%q -> %q:%q", edge.From, edge.FromPort, edge.To, edge.ToPort))
			}
			seenEdges[edge] = struct{}{}
			if _, join := seenJoinInputs[edge.To]; join {
				if _, duplicateInput := seenJoinInputs[edge.To][edge.ToPort]; duplicateInput {
					return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow select join %q has multiple edges for input port %q", edge.To, edge.ToPort))
				}
				seenJoinInputs[edge.To][edge.ToPort] = struct{}{}
			}
			adjacency[edge.From] = append(adjacency[edge.From], edge.To)
			indegree[edge.To]++
		}
		queue := make([]string, 0, len(b.operators))
		for name := range seen {
			if indegree[name] == 0 {
				queue = append(queue, name)
			}
		}
		visited := 0
		for len(queue) > 0 {
			name := queue[0]
			queue = queue[1:]
			visited++
			for _, next := range adjacency[name] {
				indegree[next]--
				if indegree[next] == 0 {
					queue = append(queue, next)
				}
			}
		}
		if visited != len(b.operators) {
			return DataflowDefinition{}, NewError(ErrorInvalidRule, "dataflow graph contains a cycle")
		}
		for _, operator := range operators {
			if operator.Kind != SelectKind || !operator.JoinConfigured {
				continue
			}
			for _, port := range operator.InputPorts {
				if _, connected := seenJoinInputs[operator.Name][port]; !connected {
					return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow select join %q requires an edge for input port %q", operator.Name, port))
				}
			}
		}
	}
	definition := DataflowDefinition{
		name:      b.name,
		operators: operators,
		edges:     append([]DataflowEdge(nil), b.edges...),
	}
	b.env.mu.Lock()
	defer b.env.mu.Unlock()
	if _, exists := b.env.dataflows[b.name]; exists {
		return DataflowDefinition{}, NewError(ErrorDependency, fmt.Sprintf("dataflow %q is already registered", b.name))
	}
	b.env.dataflows[b.name] = definition
	return definition, nil
}

func (e *Environment) Dataflow(name string) (DataflowDefinition, bool) {
	if e == nil {
		return DataflowDefinition{}, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	definition, ok := e.dataflows[name]
	return definition, ok
}

func (e *Environment) Dataflows() []DataflowDefinition {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]DataflowDefinition, 0, len(e.dataflows))
	for _, definition := range e.dataflows {
		result = append(result, definition)
	}
	for left := 0; left < len(result); left++ {
		for right := left + 1; right < len(result); right++ {
			if result[right].name < result[left].name {
				result[left], result[right] = result[right], result[left]
			}
		}
	}
	return result
}

// SaveDataflowConfiguration records a registered definition under the same
// name so it can be looked up and instantiated later. This is an in-process
// saved configuration: operator factories remain Go values and are
// intentionally not serialized as executable code.
func (e *Environment) SaveDataflowConfiguration(name string) error {
	return e.SaveDataflowConfigurationAs(name, name)
}

// SaveDataflowConfigurationAs records dataflowName under an independent
// configuration name. The separation mirrors Esper's runtime service, while
// the Go API omits Java deployment ids because an Environment owns the
// registered dataflow catalog directly.
func (e *Environment) SaveDataflowConfigurationAs(configurationName, dataflowName string) error {
	if e == nil {
		return NewError(ErrorDependency, "nil environment")
	}
	if configurationName == "" || dataflowName == "" {
		return NewError(ErrorInvalidRule, "dataflow configuration name is required")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	definition, ok := e.dataflows[dataflowName]
	if !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("dataflow %q is not registered", dataflowName))
	}
	if e.savedDataflows == nil {
		e.savedDataflows = make(map[string]DataflowDefinition)
	}
	if _, exists := e.savedDataflows[configurationName]; exists {
		return NewError(ErrorDependency, fmt.Sprintf("dataflow configuration %q is already saved", configurationName))
	}
	e.savedDataflows[configurationName] = cloneDataflowDefinition(definition)
	return nil
}

// LoadDataflowConfiguration returns a defensive copy of a saved in-process
// definition. The definition must still be registered in this Environment so
// dependency and schema validation continue to use the same catalog.
func (e *Environment) LoadDataflowConfiguration(name string) (DataflowDefinition, bool) {
	if e == nil {
		return DataflowDefinition{}, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	definition, ok := e.savedDataflows[name]
	if !ok {
		return DataflowDefinition{}, false
	}
	return cloneDataflowDefinition(definition), true
}

func (e *Environment) SavedDataflowConfigurations() []string {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]string, 0, len(e.savedDataflows))
	for name := range e.savedDataflows {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func (e *Environment) DeleteDataflowConfiguration(name string) error {
	if e == nil {
		return NewError(ErrorDependency, "nil environment")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.savedDataflows[name]; !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("dataflow configuration %q is not saved", name))
	}
	delete(e.savedDataflows, name)
	return nil
}

func cloneDataflowDefinition(definition DataflowDefinition) DataflowDefinition {
	result := definition
	result.operators = append([]DataflowOperator(nil), definition.operators...)
	result.edges = append([]DataflowEdge(nil), definition.edges...)
	for index := range result.operators {
		result.operators[index].Events = append([]any(nil), result.operators[index].Events...)
		result.operators[index].Selections = append([]Selection(nil), result.operators[index].Selections...)
		result.operators[index].InputPorts = append([]string(nil), result.operators[index].InputPorts...)
		result.operators[index].OutputPorts = append([]string(nil), result.operators[index].OutputPorts...)
		result.operators[index].InputPortTypes = cloneDataflowPortTypes(result.operators[index].InputPortTypes)
		result.operators[index].OutputPortTypes = cloneDataflowPortTypes(result.operators[index].OutputPortTypes)
		result.operators[index].Properties = maps.Clone(result.operators[index].Properties)
		result.operators[index].ParameterNames = append([]string(nil), result.operators[index].ParameterNames...)
		result.operators[index].SelectOptions = cloneDataflowSelectOptions(result.operators[index].SelectOptions)
		result.operators[index].JoinOptions = cloneDataflowJoinOptions(result.operators[index].JoinOptions)
	}
	return result
}

func validateDataflowPorts(operator DataflowOperator) error {
	for direction, ports := range map[string][]string{"input": operator.InputPorts, "output": operator.OutputPorts} {
		seen := make(map[string]struct{}, len(ports))
		for _, port := range ports {
			if port == "" {
				return NewError(ErrorInvalidRule, fmt.Sprintf("dataflow %s port on operator %q cannot be empty", direction, operator.Name))
			}
			if _, exists := seen[port]; exists {
				return NewError(ErrorInvalidRule, fmt.Sprintf("dataflow operator %q duplicates %s port %q", operator.Name, direction, port))
			}
			seen[port] = struct{}{}
		}
	}
	for direction, types := range map[string]map[string]reflect.Type{
		"input":  operator.InputPortTypes,
		"output": operator.OutputPortTypes,
	} {
		for port, typ := range types {
			if !dataflowPortAllowed(operator, direction == "output", port) {
				return NewError(ErrorInvalidRule, fmt.Sprintf("dataflow %s port type references undeclared port %q on operator %q", direction, port, operator.Name))
			}
			if typ == nil {
				return NewError(ErrorInvalidRule, fmt.Sprintf("dataflow %s port type on operator %q cannot be nil", direction, operator.Name))
			}
		}
	}
	return nil
}

func dataflowPortSpecs(specs []DataflowPort) ([]string, map[string]reflect.Type) {
	names := make([]string, 0, len(specs))
	types := make(map[string]reflect.Type, len(specs))
	for _, spec := range specs {
		names = append(names, spec.Name)
		if spec.Type != nil {
			types[spec.Name] = spec.Type
		}
	}
	if len(types) == 0 {
		types = nil
	}
	return names, types
}

func cloneDataflowPortTypes(types map[string]reflect.Type) map[string]reflect.Type {
	if len(types) == 0 {
		return nil
	}
	return maps.Clone(types)
}

// inferDataflowBuiltinPorts fills the Go value contract for built-ins whose
// runtime representation is stable. Custom operators keep their explicit
// declarations. A nil type remains a wildcard when a source is empty or can
// legitimately emit heterogeneous values.
func inferDataflowBuiltinPorts(operator DataflowOperator) DataflowOperator {
	operator.InputPortTypes = cloneDataflowPortTypes(operator.InputPortTypes)
	operator.OutputPortTypes = cloneDataflowPortTypes(operator.OutputPortTypes)
	eventType := reflect.TypeOf(Event{})
	rowType := reflect.TypeOf(Row{})
	switch operator.Kind {
	case EventBusSourceKind:
		setDataflowBuiltinPortType(&operator.OutputPortTypes, "out", eventType)
	case EventBusSinkKind:
		setDataflowBuiltinPortType(&operator.InputPortTypes, "in", eventType)
	case FilterKind:
		// Filter preserves its input representation. The closed union keeps
		// Event and Row chains valid without pretending that one is fixed.
		setDataflowBuiltinPortType(&operator.InputPortTypes, "in", dataflowRecordType())
		ports := operator.OutputPorts
		if len(ports) == 0 {
			ports = []string{"out"}
		}
		for _, port := range ports {
			setDataflowBuiltinPortType(&operator.OutputPortTypes, port, dataflowRecordType())
		}
	case SelectKind:
		// Select accepts either an Event or a projection Row. Ordinary Select
		// emits a new Row; SelectEvent emits Event and SelectPassThrough keeps
		// the input DataflowRecord representation.
		if operator.JoinConfigured {
			for _, port := range operator.InputPorts {
				setDataflowBuiltinPortType(&operator.InputPortTypes, port, dataflowRecordType())
			}
		} else {
			setDataflowBuiltinPortType(&operator.InputPortTypes, "in", dataflowRecordType())
		}
		if operator.SelectOptions.OutputEventType != "" {
			setDataflowBuiltinPortType(&operator.OutputPortTypes, "out", eventType)
		} else if operator.SelectOptions.PreserveInput {
			setDataflowBuiltinPortType(&operator.OutputPortTypes, "out", dataflowRecordType())
		} else {
			setDataflowBuiltinPortType(&operator.OutputPortTypes, "out", rowType)
		}
	case EPStatementSourceKind:
		if operator.Statement != nil {
			if _, projected := operator.Statement.Plan().ResultSchema(); projected {
				setDataflowBuiltinPortType(&operator.OutputPortTypes, "out", rowType)
			} else {
				setDataflowBuiltinPortType(&operator.OutputPortTypes, "out", eventType)
			}
		}
	case BeaconSourceKind:
		setDataflowBuiltinPortType(&operator.OutputPortTypes, "out", dataflowBeaconValueType(operator.Events))
	}
	return operator
}

// inferDataflowConnectedBuiltinPorts narrows the Event/Row union for a
// Filter or Select when its upstream port has an exact built-in type. This
// keeps EventBusSource -> Filter -> EventBusSink statically valid while also
// allowing EPStatementSource(Row) -> Filter(ResultField) -> Select graphs.
func inferDataflowConnectedBuiltinPorts(operators []DataflowOperator, byName map[string]DataflowOperator, edges []DataflowEdge) {
	indexes := make(map[string]int, len(operators))
	for index, operator := range operators {
		indexes[operator.Name] = index
	}
	for pass := 0; pass < len(operators)+1; pass++ {
		changed := false
		for _, edge := range edges {
			from, fromOK := byName[edge.From]
			to, toOK := byName[edge.To]
			if !fromOK || !toOK || edge.FromPort != "out" {
				continue
			}
			if edge.ToPort != "in" && !(to.Kind == SelectKind && to.JoinConfigured) {
				continue
			}
			output := dataflowPortType(from, true, edge.FromPort)
			if output == nil || (output != reflect.TypeOf(Event{}) && output != reflect.TypeOf(Row{})) {
				continue
			}
			edgeChanged := false
			switch to.Kind {
			case FilterKind:
				edgeChanged = refineDataflowBuiltinPortType(&to.InputPortTypes, "in", output) || edgeChanged
				ports := to.OutputPorts
				if len(ports) == 0 {
					ports = []string{"out"}
				}
				for _, port := range ports {
					edgeChanged = refineDataflowBuiltinPortType(&to.OutputPortTypes, port, output) || edgeChanged
				}
			case SelectKind:
				port := edge.ToPort
				if !to.JoinConfigured && port != "in" {
					continue
				}
				edgeChanged = refineDataflowBuiltinPortType(&to.InputPortTypes, port, output) || edgeChanged
			}
			if edgeChanged {
				changed = true
				byName[to.Name] = to
				operators[indexes[to.Name]] = to
			}
		}
		if !changed {
			return
		}
	}
}

func setDataflowBuiltinPortType(types *map[string]reflect.Type, port string, typ reflect.Type) {
	if typ == nil || types == nil {
		return
	}
	if *types == nil {
		*types = make(map[string]reflect.Type)
	}
	if _, exists := (*types)[port]; !exists {
		(*types)[port] = typ
	}
}

func refineDataflowBuiltinPortType(types *map[string]reflect.Type, port string, typ reflect.Type) bool {
	if typ == nil || types == nil {
		return false
	}
	if *types == nil {
		*types = make(map[string]reflect.Type)
	}
	current, exists := (*types)[port]
	if exists && current != nil && current != dataflowRecordType() {
		return false
	}
	if exists && current == typ {
		return false
	}
	(*types)[port] = typ
	return true
}

func dataflowRecordType() reflect.Type {
	return reflect.TypeOf((*DataflowRecord)(nil)).Elem()
}

func dataflowBeaconValueType(values []any) reflect.Type {
	var result reflect.Type
	for _, value := range values {
		if _, signal := value.(DataflowSignal); signal {
			continue
		}
		typ := reflect.TypeOf(value)
		if typ == nil {
			return nil
		}
		if result == nil {
			result = typ
			continue
		}
		if result != typ {
			return nil
		}
	}
	return result
}

func dataflowFilterOutputPorts(operator DataflowOperator) (string, string) {
	if len(operator.OutputPorts) == 0 {
		return "out", ""
	}
	if len(operator.OutputPorts) == 1 {
		return operator.OutputPorts[0], ""
	}
	return operator.OutputPorts[0], operator.OutputPorts[1]
}

func dataflowPortAllowed(operator DataflowOperator, output bool, port string) bool {
	ports := operator.InputPorts
	if output {
		ports = operator.OutputPorts
	}
	if len(ports) == 0 {
		if output {
			return port == "out"
		}
		return port == "in"
	}
	for _, declared := range ports {
		if declared == port {
			return true
		}
	}
	return false
}

func dataflowPortType(operator DataflowOperator, output bool, port string) reflect.Type {
	if output {
		return operator.OutputPortTypes[port]
	}
	return operator.InputPortTypes[port]
}

func dataflowTypesAssignable(output, input reflect.Type) bool {
	if output == nil || input == nil {
		return true
	}
	return output.AssignableTo(input)
}

func dataflowValueAssignable(value any, expected reflect.Type) bool {
	if expected == nil || value == nil {
		return true
	}
	return reflect.TypeOf(value).AssignableTo(expected)
}

type DataflowStats struct {
	Processed uint64
	Emitted   uint64
	Errors    uint64
	Dropped   uint64
}

// DataflowOperatorStat is a point-in-time snapshot of one operator's graph
// activity. Submitted counts values emitted to at least one connected
// downstream port; terminal operators therefore report zero submissions.
// Elapsed values are monotonic wall-clock durations and are intended for
// diagnostics rather than latency SLAs.
type DataflowOperatorStat struct {
	Name            string
	Number          int
	PrettyPrint     string
	Submitted       uint64
	SubmittedByPort []uint64
	Elapsed         time.Duration
	ElapsedByPort   []time.Duration
}

type dataflowOperatorStatState struct {
	submitted       atomic.Uint64
	submittedByPort []atomic.Uint64
	elapsed         atomic.Int64
	elapsedByPort   []atomic.Int64
	portIndexes     map[string]int
}

func newDataflowOperatorStatState(operator DataflowOperator) *dataflowOperatorStatState {
	state := &dataflowOperatorStatState{
		portIndexes: make(map[string]int, len(operator.OutputPorts)),
	}
	if len(operator.OutputPorts) == 0 {
		return state
	}
	state.submittedByPort = make([]atomic.Uint64, len(operator.OutputPorts))
	state.elapsedByPort = make([]atomic.Int64, len(operator.OutputPorts))
	for index, port := range operator.OutputPorts {
		state.portIndexes[port] = index
	}
	return state
}

type dataflowSelectEvent struct {
	event Event
	at    time.Time
}

type dataflowSelectGroup struct {
	events   []Event
	ever     []Event
	current  Event
	sequence uint64
}

type dataflowSelectState struct {
	mu             sync.Mutex
	options        DataflowSelectOptions
	join           DataflowJoinOptions
	events         []dataflowSelectEvent
	ever           []Event
	lastEvent      Event
	nextOutput     time.Time
	started        bool
	iterateGroups  map[string]*dataflowSelectGroup
	nextGroupOrder uint64
	joinLatest     []*Event
	joinAll        [][]Event
}

type DataflowInstance struct {
	mu sync.Mutex
	// dispatchMu serializes graph delivery for one instance. Custom sources
	// run concurrently, while Esper operators observe one input at a time;
	// keeping the queue synchronous also makes a slow downstream operator
	// provide natural backpressure to the submitting source.
	dispatchMu             sync.Mutex
	engine                 *Engine
	definition             DataflowDefinition
	options                DataflowOptions
	state                  DataflowState
	outputs                []any
	signals                []DataflowSignal
	eventTypes             map[string]struct{}
	operators              map[string]DataflowOperator
	operatorStats          map[string]*dataflowOperatorStatState
	outgoing               map[string][]DataflowEdge
	graph                  bool
	statementSubscriptions []*Subscription
	runtimes               map[string]DataflowOperatorRuntime
	sources                map[string]DataflowSourceRuntime
	subqueryRegistries     map[string]*subqueryRuntimeRegistry
	selectStates           map[string]*dataflowSelectState
	runtimesClosed         bool
	done                   chan struct{}
	doneOnce               sync.Once
	runCancel              context.CancelFunc
	sourceWG               sync.WaitGroup
	persistentSource       bool
	runErr                 error
	processed              atomic.Uint64
	emitted                atomic.Uint64
	errors                 atomic.Uint64
	dropped                atomic.Uint64
	lastError              error
}

func dataflowOperatorContext(dataflowName, instanceID string, operator DataflowOperator, number int, options DataflowOptions) DataflowOperatorContext {
	properties := maps.Clone(operator.Properties)
	if properties == nil {
		properties = make(map[string]any)
	}
	names := make([]string, 0, len(operator.Properties)+len(operator.ParameterNames))
	seen := make(map[string]struct{}, len(operator.Properties)+len(operator.ParameterNames))
	for name := range operator.Properties {
		seen[name] = struct{}{}
		names = append(names, name)
	}
	for _, name := range operator.ParameterNames {
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	sort.Strings(names)
	if options.ParameterProvider != nil {
		for _, name := range names {
			defaultValue := properties[name]
			value, provided := options.ParameterProvider(DataflowParameterContext{
				DataflowName:  dataflowName,
				InstanceID:    instanceID,
				OperatorName:  operator.Name,
				OperatorNum:   number,
				ParameterName: name,
				DefaultValue:  defaultValue,
			})
			if provided {
				properties[name] = value
			}
		}
	}
	return DataflowOperatorContext{
		DataflowName: dataflowName,
		InstanceID:   instanceID,
		OperatorName: operator.Name,
		OperatorNum:  number,
		Properties:   properties,
	}
}

func (e *Engine) InstantiateDataflow(ctx context.Context, definition DataflowDefinition) (*DataflowInstance, error) {
	return e.InstantiateDataflowWithOptions(ctx, definition, DataflowOptions{})
}

// InstantiateDataflowWithOptions creates an independent runtime instance
// from a registered definition. Factories are invoked once per instance and
// receive the configured instance identity.
func (e *Engine) InstantiateDataflowWithOptions(ctx context.Context, definition DataflowDefinition, options DataflowOptions) (*DataflowInstance, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if err := options.validate(); err != nil {
		return nil, err
	}
	if e == nil || e.env == nil {
		return nil, NewError(ErrorDependency, "engine has no environment")
	}
	registered, ok := e.env.Dataflow(definition.name)
	if !ok || registered.name != definition.name {
		return nil, NewError(ErrorDependency, "dataflow is not registered in engine environment")
	}
	if options.InstanceID == "" {
		options.InstanceID = registered.name
	}
	instance := &DataflowInstance{
		engine:             e,
		definition:         registered,
		options:            options,
		state:              DataflowInstantiated,
		eventTypes:         make(map[string]struct{}),
		operators:          make(map[string]DataflowOperator, len(registered.operators)),
		operatorStats:      make(map[string]*dataflowOperatorStatState, len(registered.operators)),
		outgoing:           make(map[string][]DataflowEdge),
		graph:              len(registered.edges) > 0,
		runtimes:           make(map[string]DataflowOperatorRuntime),
		sources:            make(map[string]DataflowSourceRuntime),
		subqueryRegistries: make(map[string]*subqueryRuntimeRegistry),
		selectStates:       make(map[string]*dataflowSelectState),
		done:               make(chan struct{}),
	}
	for operatorNumber, operator := range registered.operators {
		instance.operators[operator.Name] = operator
		instance.operatorStats[operator.Name] = newDataflowOperatorStatState(operator)
		if operator.Kind == EventBusSourceKind {
			instance.eventTypes[operator.EventType] = struct{}{}
		}
		operatorContext := dataflowOperatorContext(registered.name, options.InstanceID, operator, operatorNumber, options)
		if operator.Kind == CustomKind {
			var runtime DataflowOperatorRuntime
			var err error
			if options.OperatorProvider != nil {
				runtime, err = options.OperatorProvider(operatorContext)
			} else {
				runtime, err = operator.Factory(operatorContext)
			}
			if err != nil {
				for _, created := range instance.runtimes {
					if lifecycle, ok := created.(DataflowOperatorCloser); ok {
						_ = lifecycle.Close(context.Background())
					}
				}
				return nil, err
			}
			if runtime == nil {
				return nil, NewError(ErrorDependency, fmt.Sprintf("dataflow custom operator %q factory returned nil runtime", operator.Name))
			}
			instance.runtimes[operator.Name] = runtime
		}
		if operator.Kind == CustomSourceKind {
			var source DataflowSourceRuntime
			var err error
			if options.SourceProvider != nil {
				source, err = options.SourceProvider(operatorContext)
			} else {
				source, err = operator.SourceFactory(operatorContext)
			}
			if err != nil {
				for _, created := range instance.runtimes {
					if lifecycle, ok := created.(DataflowOperatorCloser); ok {
						_ = lifecycle.Close(context.Background())
					}
				}
				for _, created := range instance.sources {
					if lifecycle, ok := created.(DataflowOperatorCloser); ok {
						_ = lifecycle.Close(context.Background())
					}
				}
				return nil, err
			}
			if source == nil {
				for _, created := range instance.runtimes {
					if lifecycle, ok := created.(DataflowOperatorCloser); ok {
						_ = lifecycle.Close(context.Background())
					}
				}
				for _, created := range instance.sources {
					if lifecycle, ok := created.(DataflowOperatorCloser); ok {
						_ = lifecycle.Close(context.Background())
					}
				}
				return nil, NewError(ErrorDependency, fmt.Sprintf("dataflow custom source %q factory returned nil runtime", operator.Name))
			}
			instance.sources[operator.Name] = source
		}
		var expressions []Selection
		switch operator.Kind {
		case FilterKind:
			if operator.Predicate != nil {
				expressions = []Selection{{Name: operator.Name, Expr: operator.Predicate}}
			}
		case SelectKind:
			expressions = operator.Selections
		}
		if len(expressions) > 0 {
			registry := newSubqueryRuntimeRegistry(e.env, e, Query{env: e.env, selections: expressions})
			if registry != nil {
				instance.subqueryRegistries[operator.Name] = registry
			}
		}
		if operator.Kind == SelectKind {
			state := &dataflowSelectState{
				options:       cloneDataflowSelectOptions(operator.SelectOptions),
				join:          operator.JoinOptions,
				iterateGroups: make(map[string]*dataflowSelectGroup),
			}
			if operator.JoinConfigured {
				state.joinLatest = make([]*Event, operator.JoinOptions.Inputs)
				state.joinAll = make([][]Event, operator.JoinOptions.Inputs)
			}
			instance.selectStates[operator.Name] = state
		}
	}
	for _, edge := range registered.edges {
		instance.outgoing[edge.From] = append(instance.outgoing[edge.From], edge)
	}
	for _, operator := range registered.operators {
		if operator.Kind == EventBusSourceKind || operator.Kind == EPStatementSourceKind {
			instance.persistentSource = true
			break
		}
		if operator.Kind == EmitterKind && len(instance.outgoing[operator.Name]) > 0 {
			instance.persistentSource = true
			break
		}
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, NewError(ErrorState, "engine is closed")
	}
	e.dataflows[instance] = struct{}{}
	e.mu.Unlock()
	return instance, nil
}

// CaptiveEmitter returns a handle for an Emitter operator that has outgoing
// graph edges.  The handle can be used after Start while the dataflow remains
// in its running state, matching Esper's captive start mode.
func (d *DataflowInstance) CaptiveEmitter(name string) (*DataflowEmitter, error) {
	if d == nil {
		return nil, NewError(ErrorState, "nil dataflow instance")
	}
	d.mu.Lock()
	operator, ok := d.operators[name]
	hasOutgoing := len(d.outgoing[name]) > 0
	d.mu.Unlock()
	if !ok {
		return nil, NewError(ErrorUnknownName, fmt.Sprintf("dataflow emitter %q is not defined", name))
	}
	if operator.Kind != EmitterKind || !hasOutgoing {
		return nil, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow operator %q is not a captive emitter source", name))
	}
	return &DataflowEmitter{instance: d, name: name}, nil
}

// InstantiateSavedDataflow creates an instance from a saved in-process
// configuration using default instance options.
func (e *Engine) InstantiateSavedDataflow(ctx context.Context, name string) (*DataflowInstance, error) {
	return e.InstantiateSavedDataflowWithOptions(ctx, name, DataflowOptions{})
}

func (e *Engine) InstantiateSavedDataflowWithOptions(ctx context.Context, name string, options DataflowOptions) (*DataflowInstance, error) {
	if e == nil || e.env == nil {
		return nil, NewError(ErrorDependency, "engine has no environment")
	}
	definition, ok := e.env.LoadDataflowConfiguration(name)
	if !ok {
		return nil, NewError(ErrorUnknownName, fmt.Sprintf("dataflow configuration %q is not saved", name))
	}
	return e.InstantiateDataflowWithOptions(ctx, definition, options)
}

// SaveDataflowInstance stores an already-instantiated runtime under a stable
// name. The saved value is the same in-memory instance, matching Esper's
// saveInstance/getSavedInstance contract; it is not a process boundary or a
// serialization mechanism.
func (e *Engine) SaveDataflowInstance(name string, instance *DataflowInstance) error {
	if e == nil {
		return NewError(ErrorDependency, "nil engine")
	}
	if name == "" {
		return NewError(ErrorInvalidRule, "dataflow instance name is required")
	}
	if instance == nil {
		return NewError(ErrorState, "nil dataflow instance")
	}
	if instance.engine != e {
		return NewError(ErrorDependency, "dataflow instance belongs to a different engine")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return NewError(ErrorState, "engine is closed")
	}
	if e.savedDataflowInstances == nil {
		e.savedDataflowInstances = make(map[string]*DataflowInstance)
	}
	if _, exists := e.savedDataflowInstances[name]; exists {
		return NewError(ErrorDependency, fmt.Sprintf("dataflow instance %q is already saved", name))
	}
	e.savedDataflowInstances[name] = instance
	return nil
}

// LoadDataflowInstance returns a previously saved in-process instance.
func (e *Engine) LoadDataflowInstance(name string) (*DataflowInstance, bool) {
	if e == nil {
		return nil, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	instance, ok := e.savedDataflowInstances[name]
	return instance, ok
}

// SavedDataflowInstances returns saved instance names in deterministic order.
func (e *Engine) SavedDataflowInstances() []string {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	result := make([]string, 0, len(e.savedDataflowInstances))
	for name := range e.savedDataflowInstances {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

// DeleteDataflowInstance removes a previously saved in-process instance.
func (e *Engine) DeleteDataflowInstance(name string) error {
	if e == nil {
		return NewError(ErrorDependency, "nil engine")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.savedDataflowInstances[name]; !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("dataflow instance %q is not saved", name))
	}
	delete(e.savedDataflowInstances, name)
	return nil
}

// DataflowName identifies the registered definition used by this instance.
func (d *DataflowInstance) DataflowName() string {
	if d == nil {
		return ""
	}
	return d.definition.name
}

func (d *DataflowInstance) State() DataflowState {
	if d == nil {
		return DataflowCanceled
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state
}

func (d *DataflowInstance) Stats() DataflowStats {
	if d == nil {
		return DataflowStats{}
	}
	return DataflowStats{
		Processed: d.processed.Load(),
		Emitted:   d.emitted.Load(),
		Errors:    d.errors.Load(),
		Dropped:   d.dropped.Load(),
	}
}

// OperatorStats returns deterministic per-operator activity snapshots in
// definition order. The returned slices are independent copies and can be
// retained by callers while the dataflow continues running.
func (d *DataflowInstance) OperatorStats() []DataflowOperatorStat {
	if d == nil {
		return nil
	}
	result := make([]DataflowOperatorStat, 0, len(d.definition.operators))
	for number, operator := range d.definition.operators {
		state := d.operatorStats[operator.Name]
		stat := DataflowOperatorStat{
			Name:        operator.Name,
			Number:      number,
			PrettyPrint: fmt.Sprintf("%s#%d", operator.Name, number),
		}
		if state != nil {
			stat.Submitted = state.submitted.Load()
			stat.Elapsed = time.Duration(state.elapsed.Load())
			if len(state.submittedByPort) > 0 {
				stat.SubmittedByPort = make([]uint64, len(state.submittedByPort))
				stat.ElapsedByPort = make([]time.Duration, len(state.elapsedByPort))
				for index := range state.submittedByPort {
					stat.SubmittedByPort[index] = state.submittedByPort[index].Load()
					stat.ElapsedByPort[index] = time.Duration(state.elapsedByPort[index].Load())
				}
			}
		}
		result = append(result, stat)
	}
	return result
}

func (d *DataflowInstance) recordOperatorElapsed(name string, elapsed time.Duration) {
	if d == nil || elapsed < 0 {
		return
	}
	if state := d.operatorStats[name]; state != nil {
		state.elapsed.Add(int64(elapsed))
	}
}

func (d *DataflowInstance) recordOperatorSubmission(name, port string, elapsed time.Duration) {
	if d == nil {
		return
	}
	state := d.operatorStats[name]
	if state == nil {
		return
	}
	state.submitted.Add(1)
	if index, ok := state.portIndexes[port]; ok {
		state.submittedByPort[index].Add(1)
		if elapsed > 0 {
			state.elapsedByPort[index].Add(int64(elapsed))
		}
	}
}

func (d *DataflowInstance) InstanceID() string {
	if d == nil {
		return ""
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.options.InstanceID
}

func (d *DataflowInstance) UserObject() any {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.options.UserObject
}

// LastError returns the most recently observed operator error. The boolean
// distinguishes no failure from a stored nil value.
func (d *DataflowInstance) LastError() (error, bool) {
	if d == nil {
		return nil, false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.lastError == nil {
		return nil, false
	}
	return d.lastError, true
}

func (d *DataflowInstance) Outputs() []any {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]any(nil), d.outputs...)
}

// Signals returns control-plane signals observed by Emitter operators in
// arrival order.  It is intended for finite/captive flows and test harnesses;
// production operators should normally consume signals through OnSignal.
func (d *DataflowInstance) Signals() []DataflowSignal {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]DataflowSignal(nil), d.signals...)
}

// SubmitSignal injects a signal into a running dataflow.  For graph flows it
// enters at every source operator, which mirrors an emitter submitting a
// marker into a graph with multiple source ports.
func (d *DataflowInstance) SubmitSignal(ctx context.Context, signal DataflowSignal) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if signal == nil {
		return NewError(ErrorInvalidRule, "dataflow signal is nil")
	}
	if d == nil {
		return NewError(ErrorState, "nil dataflow instance")
	}
	d.mu.Lock()
	running := d.state == DataflowRunning
	d.mu.Unlock()
	if !running {
		return NewError(ErrorState, "dataflow is not running")
	}
	return d.process(ctx, signal)
}

func (d *DataflowInstance) submitEmitter(ctx context.Context, name string, value any) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if d == nil {
		return NewError(ErrorState, "nil dataflow instance")
	}
	d.mu.Lock()
	running := d.state == DataflowRunning
	operator, ok := d.operators[name]
	hasOutgoing := len(d.outgoing[name]) > 0
	d.mu.Unlock()
	if !running {
		return NewError(ErrorState, "dataflow is not running")
	}
	if !ok || operator.Kind != EmitterKind || !hasOutgoing {
		return NewError(ErrorInvalidRule, fmt.Sprintf("dataflow operator %q is not a captive emitter source", name))
	}
	if signal, ok := value.(DataflowSignal); ok {
		return d.processGraphFrom(ctx, signal, name)
	}
	event, err := d.materializeDataflowEvent(value)
	if err != nil {
		return err
	}
	d.processed.Add(1)
	return d.processGraphFrom(ctx, event, name)
}

func (d *DataflowInstance) submitSourceValue(ctx context.Context, name string, value any) error {
	return d.submitSourcePort(ctx, name, "out", value)
}

func (d *DataflowInstance) submitSourcePort(ctx context.Context, name, port string, value any) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if d == nil {
		return NewError(ErrorState, "nil dataflow instance")
	}
	d.mu.Lock()
	running := d.state == DataflowRunning
	operator, ok := d.operators[name]
	hasOutgoing := false
	for _, edge := range d.outgoing[name] {
		if edge.FromPort == port {
			hasOutgoing = true
			break
		}
	}
	d.mu.Unlock()
	if !running {
		return NewError(ErrorState, "dataflow is not running")
	}
	if !ok || operator.Kind != CustomSourceKind || !hasOutgoing {
		return NewError(ErrorInvalidRule, fmt.Sprintf("dataflow operator %q is not a custom source", name))
	}
	if !dataflowPortAllowed(operator, true, port) {
		return NewError(ErrorInvalidRule, fmt.Sprintf("dataflow source %q references unknown output port %q", name, port))
	}
	if _, signal := value.(DataflowSignal); !signal {
		if expected := dataflowPortType(operator, true, port); expected != nil && !dataflowValueAssignable(value, expected) {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("dataflow source %q output port %q emitted %T, want %s", name, port, value, expected))
		}
	}
	d.processed.Add(1)
	started := time.Now()
	err := d.processSourceEmission(ctx, name, port, value)
	if err == nil {
		if _, signal := value.(DataflowSignal); !signal {
			d.recordOperatorSubmission(name, port, time.Since(started))
		}
	}
	return err
}

func (d *DataflowInstance) materializeDataflowEvent(value any) (Event, error) {
	if event, ok := value.(Event); ok {
		return event, nil
	}
	if d == nil || d.engine == nil || d.engine.env == nil {
		return Event{}, NewError(ErrorDependency, "dataflow has no engine environment")
	}
	typ := reflect.TypeOf(value)
	if typ == nil {
		return Event{}, NewError(ErrorTypeMismatch, "cannot infer dataflow event type from nil")
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	d.engine.env.mu.RLock()
	eventType, ok := d.engine.env.typeToName[typ]
	d.engine.env.mu.RUnlock()
	if !ok {
		return Event{}, NewError(ErrorUnknownName, fmt.Sprintf("no registered event type for Go type %s", typ))
	}
	schema, ok := d.engine.env.Schema(eventType)
	if !ok {
		return Event{}, NewError(ErrorUnknownName, fmt.Sprintf("event type %q is not registered", eventType))
	}
	event, err := newEvent(schema, value, d.engine.Now())
	if err != nil {
		return Event{}, WrapError(ErrorTypeMismatch, "dataflow.event", err)
	}
	return event, nil
}

func (d *DataflowInstance) handleDataflowError(ctx context.Context, operator string, err error) error {
	if err == nil {
		return nil
	}
	if d == nil {
		return err
	}
	d.mu.Lock()
	errorValue := DataflowError{
		DataflowName: d.definition.name,
		InstanceID:   d.options.InstanceID,
		OperatorName: operator,
		Err:          err,
	}
	d.lastError = errorValue
	handler := d.options.ExceptionHandler
	policy := d.options.ErrorPolicy
	d.mu.Unlock()
	d.errors.Add(1)
	if handler != nil {
		if handlerErr := handler(ctx, errorValue); handlerErr != nil {
			return fmt.Errorf("dataflow exception handler: %w", handlerErr)
		}
	}
	if policy == DataflowErrorContinue {
		d.dropped.Add(1)
		return nil
	}
	return errorValue
}

func (d *DataflowInstance) openRuntimes(ctx context.Context) error {
	for _, operator := range d.definition.operators {
		runtime := d.runtimes[operator.Name]
		if runtime != nil {
			if lifecycle, ok := runtime.(DataflowOperatorOpener); ok {
				if err := lifecycle.Open(ctx); err != nil {
					return fmt.Errorf("dataflow operator %q open: %w", operator.Name, err)
				}
			}
		}
		source := d.sources[operator.Name]
		if source != nil {
			if lifecycle, ok := source.(DataflowOperatorOpener); ok {
				if err := lifecycle.Open(ctx); err != nil {
					return fmt.Errorf("dataflow source %q open: %w", operator.Name, err)
				}
			}
		}
	}
	return nil
}

func (d *DataflowInstance) closeRuntimes(ctx context.Context) {
	d.mu.Lock()
	if d.runtimesClosed {
		d.mu.Unlock()
		return
	}
	d.runtimesClosed = true
	d.mu.Unlock()
	for _, operator := range d.definition.operators {
		runtime := d.runtimes[operator.Name]
		if runtime != nil {
			if lifecycle, ok := runtime.(DataflowOperatorCloser); ok {
				_ = lifecycle.Close(ctx)
			}
		}
		source := d.sources[operator.Name]
		if source != nil {
			if lifecycle, ok := source.(DataflowOperatorCloser); ok {
				_ = lifecycle.Close(ctx)
			}
		}
	}
}

func (d *DataflowInstance) closeDone() {
	if d == nil || d.done == nil {
		return
	}
	d.doneOnce.Do(func() { close(d.done) })
}

func (d *DataflowInstance) complete(err error) {
	if d == nil {
		return
	}
	d.mu.Lock()
	if d.state == DataflowCanceled {
		d.mu.Unlock()
		return
	}
	if d.state == DataflowComplete {
		if d.runErr == nil && err != nil {
			d.runErr = err
		}
		d.mu.Unlock()
		return
	}
	d.state = DataflowComplete
	if err != nil {
		d.runErr = err
	}
	cancel := d.runCancel
	subscriptions := append([]*Subscription(nil), d.statementSubscriptions...)
	d.statementSubscriptions = nil
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	for _, subscription := range subscriptions {
		if subscription != nil {
			_ = subscription.Close()
		}
	}
	d.closeRuntimes(context.Background())
	d.closeDone()
}

func (d *DataflowInstance) watchRunContext(runCtx context.Context) {
	if runCtx == nil || runCtx.Done() == nil {
		return
	}
	go func() {
		<-runCtx.Done()
		d.mu.Lock()
		running := d.state == DataflowRunning
		d.mu.Unlock()
		if running {
			_ = d.Cancel(context.Background())
		}
	}()
}

func (d *DataflowInstance) runSource(runCtx context.Context, operator DataflowOperator, source DataflowSourceRuntime) {
	defer d.sourceWG.Done()
	emitter := &DataflowEmitter{instance: d, name: operator.Name, allowRaw: true}
	started := time.Now()
	err := source.Run(runCtx, emitter)
	d.recordOperatorElapsed(operator.Name, time.Since(started))
	if err == nil || runCtx.Err() != nil {
		return
	}
	if handled := d.handleDataflowError(context.Background(), operator.Name, err); handled != nil {
		d.complete(handled)
	}
}

func (d *DataflowInstance) waitSources() {
	d.sourceWG.Wait()
	d.mu.Lock()
	complete := d.state == DataflowRunning && !d.persistentSource
	d.mu.Unlock()
	if complete {
		d.complete(nil)
	}
}

func (d *DataflowInstance) Start(ctx context.Context) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if d == nil {
		return NewError(ErrorState, "nil dataflow instance")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, runCancel := context.WithCancel(ctx)
	d.mu.Lock()
	if d.state != DataflowInstantiated {
		d.mu.Unlock()
		runCancel()
		return NewError(ErrorState, "dataflow can only start from instantiated state")
	}
	d.state = DataflowRunning
	d.runCancel = runCancel
	d.mu.Unlock()
	d.watchRunContext(runCtx)
	d.startDataflowSelectStates(d.engine.Now())
	if err := d.openRuntimes(ctx); err != nil {
		_ = d.Cancel(context.Background())
		return err
	}
	hasEventSource := d.persistentSource
	for _, operator := range d.definition.operators {
		if operator.Kind != EPStatementSourceKind {
			continue
		}
		hasEventSource = true
		subscription, err := operator.Statement.Subscribe(func(callbackCtx context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				var value any
				if event, ok := result.Event(); ok {
					value = event
				} else if row, ok := result.Row(); ok {
					value = row
				} else {
					continue
				}
				var processErr error
				if d.graph {
					d.processed.Add(1)
					processErr = d.processGraphFrom(callbackCtx, value, operator.Name)
				} else {
					processErr = d.process(callbackCtx, value)
				}
				if processErr != nil {
					return processErr
				}
				if err := contextErr(callbackCtx); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			_ = d.Cancel(context.Background())
			return err
		}
		d.mu.Lock()
		d.statementSubscriptions = append(d.statementSubscriptions, &subscription)
		d.mu.Unlock()
	}
	for _, operator := range d.definition.operators {
		if operator.Kind != BeaconSourceKind {
			continue
		}
		for _, event := range operator.Events {
			if err := contextErr(ctx); err != nil {
				_ = d.Cancel(context.Background())
				return err
			}
			var processErr error
			_, isSignal := event.(DataflowSignal)
			if d.graph {
				if !isSignal {
					d.processed.Add(1)
				}
				processErr = d.processGraphFrom(ctx, event, operator.Name)
			} else {
				processErr = d.process(ctx, event)
			}
			if processErr != nil {
				_ = d.Cancel(context.Background())
				return processErr
			}
		}
	}
	sourceCount := 0
	for _, operator := range d.definition.operators {
		if operator.Kind != CustomSourceKind {
			continue
		}
		source := d.sources[operator.Name]
		if source == nil {
			continue
		}
		sourceCount++
		d.sourceWG.Add(1)
		go d.runSource(runCtx, operator, source)
	}
	if sourceCount > 0 {
		go d.waitSources()
	}
	d.mu.Lock()
	complete := d.state == DataflowRunning && !hasEventSource && sourceCount == 0
	d.mu.Unlock()
	if complete {
		d.complete(nil)
	}
	return nil
}

func (d *DataflowInstance) startDataflowSelectStates(now time.Time) {
	if d == nil {
		return
	}
	for _, state := range d.selectStates {
		if state == nil {
			continue
		}
		state.mu.Lock()
		if !state.started {
			state.started = true
			if state.options.OutputSnapshotEvery > 0 {
				state.nextOutput = now.Add(state.options.OutputSnapshotEvery)
			}
		}
		state.mu.Unlock()
	}
}

// Run starts a dataflow and waits until it completes or is canceled. It is
// the blocking counterpart to Start, mirroring Esper's run API while keeping
// cancellation and deadlines idiomatic through context.Context.
func (d *DataflowInstance) Run(ctx context.Context) error {
	if d == nil {
		return NewError(ErrorState, "nil dataflow instance")
	}
	if err := d.Start(ctx); err != nil {
		return err
	}
	return d.Join(ctx)
}

// Join waits for a previously started dataflow. A normal finite completion
// returns nil; an external Cancel is reported as ErrorCanceled so callers can
// distinguish it from a completed graph.
func (d *DataflowInstance) Join(ctx context.Context) error {
	if d == nil {
		return NewError(ErrorState, "nil dataflow instance")
	}
	d.mu.Lock()
	state := d.state
	done := d.done
	d.mu.Unlock()
	if state == DataflowInstantiated {
		return NewError(ErrorState, "dataflow has not been started")
	}
	if done == nil {
		return NewError(ErrorState, "dataflow has no completion signal")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-done:
		d.mu.Lock()
		state = d.state
		err := d.runErr
		d.mu.Unlock()
		if err != nil {
			return err
		}
		if state == DataflowCanceled {
			return NewError(ErrorCanceled, "dataflow was canceled")
		}
		return nil
	case <-ctx.Done():
		return contextErr(ctx)
	}
}

func (d *DataflowInstance) Cancel(ctx context.Context) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if d == nil {
		return nil
	}
	d.mu.Lock()
	if d.state == DataflowComplete {
		d.mu.Unlock()
		return NewError(ErrorState, "completed dataflow cannot be canceled")
	}
	if d.state == DataflowCanceled {
		d.mu.Unlock()
		return nil
	}
	d.state = DataflowCanceled
	cancel := d.runCancel
	engine := d.engine
	subscriptions := append([]*Subscription(nil), d.statementSubscriptions...)
	d.statementSubscriptions = nil
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	for _, subscription := range subscriptions {
		if subscription != nil {
			_ = subscription.Close()
		}
	}
	d.closeRuntimes(context.Background())
	d.closeDone()
	if engine != nil {
		engine.mu.Lock()
		delete(engine.dataflows, d)
		engine.mu.Unlock()
	}
	return nil
}

func (d *DataflowInstance) process(ctx context.Context, event any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if d.graph {
		return d.processGraphEvent(ctx, event)
	}
	if owner, ok := ctx.Value(dataflowDispatchOwnerKey{}).(*DataflowInstance); ok && owner == d {
		return d.processLinear(ctx, event)
	}
	d.dispatchMu.Lock()
	defer d.dispatchMu.Unlock()
	owned := context.WithValue(ctx, dataflowDispatchOwnerKey{}, d)
	if err := d.processLinear(owned, event); err != nil {
		return d.handleDataflowError(ctx, "", err)
	}
	return nil
}

func (d *DataflowInstance) advanceDataflowTime(ctx context.Context, at time.Time) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if d == nil {
		return nil
	}
	d.mu.Lock()
	running := d.state == DataflowRunning
	d.mu.Unlock()
	if !running {
		return nil
	}
	d.dispatchMu.Lock()
	defer d.dispatchMu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	owned := context.WithValue(ctx, dataflowDispatchOwnerKey{}, d)
	for index, operator := range d.definition.operators {
		if operator.Kind != SelectKind {
			continue
		}
		rows, err := d.advanceDataflowSelect(operator, at)
		if err != nil {
			if handled := d.handleDataflowError(owned, operator.Name, err); handled != nil {
				return handled
			}
			continue
		}
		for _, row := range rows {
			if d.graph {
				err = d.processSourceEmission(owned, operator.Name, "out", row)
			} else {
				err = d.processLinearValues(owned, []any{row}, index+1, false)
			}
			if err != nil {
				if handled := d.handleDataflowError(owned, operator.Name, err); handled != nil {
					return handled
				}
			}
		}
	}
	return nil
}

func (d *DataflowInstance) dataflowEvaluation(operator DataflowOperator, value any) (EvalContext, error) {
	return d.dataflowEvaluationWithGroups(operator, value, nil, nil, true)
}

func (d *DataflowInstance) dataflowEvaluationWithGroups(operator DataflowOperator, value any, group, everGroup []Event, acceptSubquery bool) (EvalContext, error) {
	if d == nil || d.engine == nil {
		return EvalContext{}, NewError(ErrorDependency, "dataflow operator has no engine")
	}
	now := d.engine.Now()
	variables := d.engine.Variables()
	evaluation := EvalContext{
		Engine:       d.engine,
		Now:          now,
		Variables:    variables,
		Group:        append([]Event(nil), group...),
		EverGroup:    append([]Event(nil), everGroup...),
		AllGroup:     append([]Event(nil), group...),
		AllEverGroup: append([]Event(nil), everGroup...),
	}
	if event, ok := value.(Event); ok {
		evaluation.Event = event
	} else if row, ok := value.(Row); ok {
		// Dataflow Select/Filter can consume projection rows produced by an
		// EPStatementSource. ResultField is the analyzable row-property
		// counterpart to Field and reads this private evaluation scope.
		rowCopy := row
		evaluation.resultRow = &rowCopy
	} else {
		return EvalContext{}, NewError(ErrorTypeMismatch, fmt.Sprintf("dataflow operator %q requires Event or Row input, got %T", operator.Name, value))
	}
	if registry := d.subqueryRegistries[operator.Name]; registry != nil {
		if acceptSubquery {
			if event, ok := value.(Event); ok {
				if err := registry.accept(event, now, variables); err != nil {
					return EvalContext{}, err
				}
			}
		}
		variables = registry.attachVariables(variables)
		evaluation.Variables = variables
	}
	return evaluation, nil
}

func (d *DataflowInstance) evaluateDataflowSelect(operator DataflowOperator, value any) (Row, error) {
	return d.evaluateDataflowSelectWithGroups(operator, value, nil, nil, true)
}

// evaluateDataflowSelectOutput evaluates a projection when present and then
// materializes the configured Dataflow Select output representation. Keeping
// this boundary separate lets select-star preserve an Event without inventing
// an empty Row schema.
func (d *DataflowInstance) evaluateDataflowSelectOutput(operator DataflowOperator, value any, group, everGroup []Event, acceptSubquery bool) (any, error) {
	if len(operator.Selections) == 0 {
		return d.materializeDataflowSelectOutput(operator, value, nil)
	}
	row, err := d.evaluateDataflowSelectWithGroups(operator, value, group, everGroup, acceptSubquery)
	if err != nil {
		return nil, err
	}
	return d.materializeDataflowSelectOutput(operator, value, &row)
}

func (d *DataflowInstance) materializeDataflowSelectOutput(operator DataflowOperator, value any, row *Row) (any, error) {
	options := operator.SelectOptions
	if options.OutputEventType == "" {
		if options.PreserveInput {
			switch value.(type) {
			case Event, Row:
				return value, nil
			default:
				return nil, NewError(ErrorTypeMismatch, fmt.Sprintf("dataflow select %q pass-through requires Event or Row input, got %T", operator.Name, value))
			}
		}
		if row == nil {
			return nil, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow select %q requires a projection row", operator.Name))
		}
		return *row, nil
	}

	if d == nil || d.engine == nil || d.engine.env == nil {
		return nil, NewError(ErrorDependency, "dataflow select event output has no environment")
	}
	schema, ok := d.engine.env.Schema(options.OutputEventType)
	if !ok {
		return nil, NewError(ErrorUnknownName, fmt.Sprintf("dataflow select %q output event type %q is not registered", operator.Name, options.OutputEventType))
	}
	updates := make(map[string]any, len(schema.Fields()))
	if options.PreserveInput {
		for _, field := range schema.Fields() {
			var property Value
			switch input := value.(type) {
			case Event:
				property = input.Get(field.Name)
			case Row:
				property = input.Get(field.Name)
			default:
				return nil, NewError(ErrorTypeMismatch, fmt.Sprintf("dataflow select %q event output requires Event or Row input, got %T", operator.Name, value))
			}
			if property.IsPresent() || property.IsNull() {
				updates[field.Name] = property.Any()
			}
		}
	}
	if row != nil {
		for _, field := range row.Schema().Fields() {
			property := row.Get(field.Name)
			if property.IsPresent() || property.IsNull() {
				updates[field.Name] = property.Any()
			}
		}
	}
	underlying, err := mergeSchemaUnderlying(schema, nil, updates)
	if err != nil {
		return nil, WrapError(ErrorTypeMismatch, fmt.Sprintf("dataflow select %q output event", operator.Name), err)
	}
	receivedAt := d.engine.Now()
	if event, ok := value.(Event); ok {
		receivedAt = event.ReceivedAt()
	}
	event, err := newEvent(schema, underlying, receivedAt)
	if err != nil {
		return nil, WrapError(ErrorTypeMismatch, fmt.Sprintf("dataflow select %q output event", operator.Name), err)
	}
	return event, nil
}

func (d *DataflowInstance) evaluateDataflowSelectWithGroups(operator DataflowOperator, value any, group, everGroup []Event, acceptSubquery bool) (Row, error) {
	evaluation, err := d.dataflowEvaluationWithGroups(operator, value, group, everGroup, acceptSubquery)
	if err != nil {
		return Row{}, err
	}
	values := make([]Value, 0, len(operator.Selections))
	for _, selection := range operator.Selections {
		if selection.Expr == nil {
			return Row{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow select %q contains nil expression", operator.Name))
		}
		values = append(values, selection.Expr.eval(evaluation))
	}
	schema, err := NewSchema("dataflow:"+operator.Name, selectionFields(operator.Selections)...)
	if err != nil {
		return Row{}, err
	}
	return newRow(schema, values), nil
}

func (d *DataflowInstance) dataflowSelectGroupKey(operator DataflowOperator, event Event) (string, error) {
	if len(operator.SelectOptions.GroupBy) == 0 {
		return "<all>", nil
	}
	evaluation, err := d.dataflowEvaluationWithGroups(operator, event, nil, nil, false)
	if err != nil {
		return "", err
	}
	values := make([]any, 0, len(operator.SelectOptions.GroupBy))
	for _, expression := range operator.SelectOptions.GroupBy {
		if expression == nil {
			return "", NewError(ErrorInvalidRule, fmt.Sprintf("dataflow select %q contains nil group-by expression", operator.Name))
		}
		values = append(values, expression.eval(evaluation).Any())
	}
	return encodeKey(values), nil
}

func (d *DataflowInstance) processDataflowSelectIterate(operator DataflowOperator, value any) ([]any, error) {
	event, ok := value.(Event)
	if !ok {
		return nil, NewError(ErrorTypeMismatch, fmt.Sprintf("dataflow select %q iterate-on-final-marker requires Event input, got %T", operator.Name, value))
	}
	state := d.selectStates[operator.Name]
	if state == nil {
		return nil, NewError(ErrorDependency, fmt.Sprintf("dataflow select %q has no runtime state", operator.Name))
	}
	key, err := d.dataflowSelectGroupKey(operator, event)
	if err != nil {
		return nil, err
	}
	state.mu.Lock()
	group := state.iterateGroups[key]
	if group == nil {
		state.nextGroupOrder++
		group = &dataflowSelectGroup{sequence: state.nextGroupOrder}
		state.iterateGroups[key] = group
	}
	group.events = append(group.events, event)
	group.ever = append(group.ever, event)
	group.current = event
	state.mu.Unlock()
	return nil, nil
}

type dataflowSelectSnapshotEntry struct {
	row         Row
	orderValues []Value
}

func (d *DataflowInstance) processDataflowSelectFinalMarker(operator DataflowOperator) ([]any, error) {
	state := d.selectStates[operator.Name]
	if state == nil {
		return nil, NewError(ErrorDependency, fmt.Sprintf("dataflow select %q has no runtime state", operator.Name))
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	entries := make([]dataflowSelectSnapshotEntry, 0, len(state.iterateGroups))
	groups := make([]*dataflowSelectGroup, 0, len(state.iterateGroups))
	for _, group := range state.iterateGroups {
		groups = append(groups, group)
	}
	sort.Slice(groups, func(left, right int) bool {
		return groups[left].sequence < groups[right].sequence
	})
	for _, group := range groups {
		if group == nil || len(group.events) == 0 {
			continue
		}
		row, err := d.evaluateDataflowSelectWithGroups(operator, group.current, group.events, group.ever, false)
		if err != nil {
			return nil, err
		}
		entry := dataflowSelectSnapshotEntry{row: row}
		for _, key := range operator.SelectOptions.OrderBy {
			evaluation, err := d.dataflowEvaluationWithGroups(operator, group.current, group.events, group.ever, false)
			if err != nil {
				return nil, err
			}
			evaluation.resultRow = &entry.row
			entry.orderValues = append(entry.orderValues, key.Expr.eval(evaluation))
		}
		entries = append(entries, entry)
	}
	if len(operator.SelectOptions.OrderBy) > 0 {
		sort.SliceStable(entries, func(left, right int) bool {
			for index, key := range operator.SelectOptions.OrderBy {
				comparison, comparable := compareOrderValues(entries[left].orderValues[index], entries[right].orderValues[index])
				if !comparable || comparison == 0 {
					continue
				}
				if key.Descending {
					return comparison > 0
				}
				return comparison < 0
			}
			return false
		})
	}
	rows := make([]any, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, entry.row)
	}
	return rows, nil
}

func (d *DataflowInstance) processDataflowSelect(operator DataflowOperator, value any) ([]any, error) {
	state := d.selectStates[operator.Name]
	if state == nil {
		output, err := d.evaluateDataflowSelectOutput(operator, value, nil, nil, true)
		if err != nil {
			return nil, err
		}
		return []any{output}, nil
	}
	if state.options.IterateOnFinalMarker {
		return d.processDataflowSelectIterate(operator, value)
	}
	now := d.engine.Now()
	state.mu.Lock()
	if event, ok := value.(Event); ok {
		if state.options.TimeWindow > 0 {
			state.expireAt(now)
		}
		state.events = append(state.events, dataflowSelectEvent{event: event, at: now})
		state.ever = append(state.ever, event)
		state.lastEvent = event
	}
	group := state.currentEvents()
	everGroup := append([]Event(nil), state.ever...)
	var row *Row
	if len(operator.Selections) > 0 {
		projected, err := d.evaluateDataflowSelectWithGroups(operator, value, group, everGroup, true)
		if err != nil {
			state.mu.Unlock()
			return nil, err
		}
		row = &projected
	}
	state.mu.Unlock()
	output, err := d.materializeDataflowSelectOutput(operator, value, row)
	if err != nil {
		return nil, err
	}
	if state.options.OutputSnapshotEvery > 0 {
		return nil, nil
	}
	return []any{output}, nil
}

func (d *DataflowInstance) processDataflowSelectJoin(operator DataflowOperator, inputPort string, value any) ([]any, error) {
	event, ok := value.(Event)
	if !ok {
		return nil, NewError(ErrorTypeMismatch, fmt.Sprintf("dataflow select join %q requires Event input, got %T", operator.Name, value))
	}
	state := d.selectStates[operator.Name]
	if state == nil {
		return nil, NewError(ErrorDependency, fmt.Sprintf("dataflow select join %q has no runtime state", operator.Name))
	}
	input := -1
	for index, port := range operator.InputPorts {
		if port == inputPort {
			input = index
			break
		}
	}
	if input < 0 || input >= state.join.Inputs {
		return nil, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow select join %q received unknown input port %q", operator.Name, inputPort))
	}
	now := d.engine.Now()
	variables := d.engine.Variables()
	state.mu.Lock()
	tuples := state.joinTuples(input, event)
	if len(state.join.Conditions) > 0 {
		matched := make([][]Event, 0, len(tuples))
		for _, tuple := range tuples {
			if joinConditionsMatchWithVariables(state.join.Conditions, tuple, now, variables) {
				matched = append(matched, tuple)
			}
		}
		// SelectJoin is insert-stream oriented. If an outer-edge input has no
		// matching combination, emit its unmatched tuple once; rows emitted for
		// earlier inputs are not replayed when a later side arrives.
		outerEdge := state.join.Kind == DataflowJoinFullOuter ||
			(state.join.Kind == DataflowJoinLeftOuter && input == 0) ||
			(state.join.Kind == DataflowJoinRightOuter && input == state.join.Inputs-1)
		if len(matched) == 0 && outerEdge {
			unmatched := make([]Event, state.join.Inputs)
			unmatched[input] = event
			matched = append(matched, unmatched)
		}
		tuples = matched
	}
	rows := make([]any, 0, len(tuples))
	for _, tuple := range tuples {
		joinEvent := newJoinTupleEvent(tuple, now)
		output, err := d.evaluateDataflowSelectOutput(operator, joinEvent, nil, nil, true)
		if err != nil {
			state.mu.Unlock()
			return nil, err
		}
		rows = append(rows, output)
	}
	state.mu.Unlock()
	return rows, nil
}

func (s *dataflowSelectState) joinTuples(input int, event Event) [][]Event {
	if s == nil || input < 0 || input >= len(s.joinLatest) {
		return nil
	}
	if s.join.Retention == DataflowJoinKeepAll {
		s.joinAll[input] = append(s.joinAll[input], event)
		complete := true
		for _, events := range s.joinAll {
			if len(events) == 0 {
				complete = false
				break
			}
		}
		if s.join.Kind == DataflowJoinInner && !complete {
			return nil
		}
		if s.join.Kind == DataflowJoinLeftOuter && len(s.joinAll[0]) == 0 {
			return nil
		}
		if s.join.Kind == DataflowJoinRightOuter && len(s.joinAll[len(s.joinAll)-1]) == 0 {
			return nil
		}
		lists := make([][]Event, len(s.joinAll))
		for index, events := range s.joinAll {
			if index == input {
				// Only combinations containing the newly arrived event are
				// new-stream rows. Replaying the full Cartesian product would
				// duplicate rows whenever an existing input receives another
				// event.
				lists[index] = []Event{event}
			} else if len(events) == 0 {
				lists[index] = []Event{{}}
			} else {
				lists[index] = events
			}
		}
		return cartesianDataflowJoinTuples(lists, 0, nil)
	}

	copyEvent := event
	s.joinLatest[input] = &copyEvent
	switch s.join.Kind {
	case DataflowJoinInner:
		for _, latest := range s.joinLatest {
			if latest == nil {
				return nil
			}
		}
	case DataflowJoinLeftOuter:
		if s.joinLatest[0] == nil {
			return nil
		}
	case DataflowJoinRightOuter:
		if s.joinLatest[len(s.joinLatest)-1] == nil {
			return nil
		}
	}
	tuple := make([]Event, len(s.joinLatest))
	for index, latest := range s.joinLatest {
		if latest != nil {
			tuple[index] = *latest
		}
	}
	return [][]Event{tuple}
}

func cartesianDataflowJoinTuples(lists [][]Event, index int, prefix []Event) [][]Event {
	if index == len(lists) {
		return [][]Event{append([]Event(nil), prefix...)}
	}
	result := make([][]Event, 0)
	for _, event := range lists[index] {
		result = append(result, cartesianDataflowJoinTuples(lists, index+1, append(prefix, event))...)
	}
	return result
}

func (s *dataflowSelectState) currentEvents() []Event {
	result := make([]Event, 0, len(s.events))
	for _, item := range s.events {
		result = append(result, item.event)
	}
	return result
}

func (s *dataflowSelectState) expireAt(at time.Time) bool {
	if s == nil || s.options.TimeWindow <= 0 || len(s.events) == 0 {
		return false
	}
	kept := make([]dataflowSelectEvent, 0, len(s.events))
	removed := false
	for _, item := range s.events {
		if !item.at.Add(s.options.TimeWindow).After(at) {
			removed = true
			continue
		}
		kept = append(kept, item)
	}
	if removed {
		s.events = kept
	}
	return removed
}

func (s *dataflowSelectState) nextExpiry() (time.Time, bool) {
	if s == nil || s.options.TimeWindow <= 0 || len(s.events) == 0 {
		return time.Time{}, false
	}
	next := s.events[0].at.Add(s.options.TimeWindow)
	for _, item := range s.events[1:] {
		expires := item.at.Add(s.options.TimeWindow)
		if expires.Before(next) {
			next = expires
		}
	}
	return next, true
}

func (d *DataflowInstance) advanceDataflowSelect(operator DataflowOperator, at time.Time) ([]any, error) {
	state := d.selectStates[operator.Name]
	if state == nil {
		return nil, nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.started {
		state.started = true
		if state.options.OutputSnapshotEvery > 0 {
			state.nextOutput = at.Add(state.options.OutputSnapshotEvery)
		}
	}
	rows := make([]any, 0)
	if state.options.OutputSnapshotEvery > 0 {
		for !state.nextOutput.IsZero() && !state.nextOutput.After(at) {
			dueAt := state.nextOutput
			state.expireAt(dueAt)
			row, err := d.evaluateDataflowSelectWithGroups(operator, state.lastEvent, state.currentEvents(), append([]Event(nil), state.ever...), false)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
			state.nextOutput = state.nextOutput.Add(state.options.OutputSnapshotEvery)
		}
		state.expireAt(at)
		return rows, nil
	}
	if state.options.TimeWindow <= 0 {
		return nil, nil
	}
	for {
		expiresAt, ok := state.nextExpiry()
		if !ok || expiresAt.After(at) {
			break
		}
		state.expireAt(expiresAt)
		row, err := d.evaluateDataflowSelectWithGroups(operator, state.lastEvent, state.currentEvents(), append([]Event(nil), state.ever...), false)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (d *DataflowInstance) processLinear(ctx context.Context, event any) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	d.mu.Lock()
	if d.state != DataflowRunning {
		d.mu.Unlock()
		return nil
	}
	d.mu.Unlock()
	if signal, ok := event.(DataflowSignal); ok {
		current := []any{signal}
		for index, operator := range d.definition.operators {
			next := make([]any, 0, len(current))
			for _, candidate := range current {
				candidateSignal, isSignal := candidate.(DataflowSignal)
				if isSignal && operator.Signal != nil {
					if err := operator.Signal(ctx, candidateSignal); err != nil {
						return err
					}
				}
				if isSignal && operator.Kind == SelectKind && operator.SelectOptions.IterateOnFinalMarker && isDataflowFinalMarker(candidateSignal) {
					rows, err := d.processDataflowSelectFinalMarker(operator)
					if err != nil {
						return err
					}
					for _, row := range rows {
						if err := d.processLinearValues(ctx, []any{row}, index+1, false); err != nil {
							return err
						}
					}
					continue
				}
				if operator.Kind == CustomKind {
					runtime := d.runtimes[operator.Name]
					if isSignal {
						if signalRuntime, ok := runtime.(DataflowOperatorSignalRuntime); ok {
							values, err := signalRuntime.OnSignal(ctx, candidateSignal)
							if err != nil {
								return err
							}
							for _, emission := range normalizeDataflowEmissions(values) {
								next = append(next, emission.Value)
							}
						} else {
							next = append(next, candidate)
						}
					} else {
						values, err := runtime.Process(ctx, DataflowInput{Port: "in", Value: candidate})
						if err != nil {
							return err
						}
						for _, emission := range normalizeDataflowEmissions(values) {
							next = append(next, emission.Value)
						}
					}
				} else {
					next = append(next, candidate)
				}
			}
			if operator.Kind == EmitterKind {
				for _, candidate := range next {
					if candidateSignal, ok := candidate.(DataflowSignal); ok {
						d.mu.Lock()
						d.signals = append(d.signals, candidateSignal)
						d.mu.Unlock()
					}
				}
			}
			current = next
		}
		return nil
	}
	eventValue, isEvent := event.(Event)
	if len(d.eventTypes) > 0 {
		if !isEvent {
			return nil
		}
		accepted := false
		for eventType := range d.eventTypes {
			if dataflowEventTypeAccepts(d.engine, eventType, eventValue) {
				accepted = true
				break
			}
		}
		if !accepted {
			return nil
		}
	}
	return d.processLinearValues(ctx, []any{event}, 0, true)
}

func (d *DataflowInstance) processLinearValues(ctx context.Context, current []any, start int, countProcessed bool) error {
	eventValue, isEvent := Event{}, false
	if len(current) > 0 {
		eventValue, isEvent = current[0].(Event)
	}
	for index := start; index < len(d.definition.operators); index++ {
		operator := d.definition.operators[index]
		started := time.Now()
		switch operator.Kind {
		case BeaconSourceKind, EPStatementSourceKind:
			continue
		case EventBusSourceKind:
			if !isEvent || !dataflowEventTypeAccepts(d.engine, operator.EventType, eventValue) {
				current = nil
			}
		case FilterKind:
			filtered := make([]any, 0, len(current))
			for _, candidate := range current {
				if _, isEvent := candidate.(Event); !isEvent {
					if _, isRow := candidate.(Row); !isRow {
						continue
					}
				}
				evaluation, err := d.dataflowEvaluation(operator, candidate)
				if err != nil {
					return err
				}
				value := operator.Predicate.eval(evaluation)
				if pass, isBool := boolValue(value); isBool && pass {
					filtered = append(filtered, candidate)
				}
			}
			current = filtered
		case SelectKind:
			selected := make([]any, 0, len(current))
			for _, candidate := range current {
				if _, isEvent := candidate.(Event); !isEvent {
					if _, isRow := candidate.(Row); !isRow {
						continue
					}
				}
				rows, err := d.processDataflowSelect(operator, candidate)
				if err != nil {
					return err
				}
				selected = append(selected, rows...)
			}
			current = selected
		case EmitterKind:
			d.mu.Lock()
			d.outputs = append(d.outputs, current...)
			d.mu.Unlock()
			d.emitted.Add(uint64(len(current)))
		case LogSinkKind:
			if operator.Log != nil {
				for _, candidate := range current {
					if err := operator.Log(ctx, candidate); err != nil {
						return err
					}
				}
			}
		case EventBusSinkKind:
			if d.engine == nil {
				return NewError(ErrorDependency, "dataflow event-bus sink has no engine")
			}
			for _, candidate := range current {
				candidateEvent, ok := candidate.(Event)
				if !ok {
					return NewError(ErrorTypeMismatch, fmt.Sprintf("dataflow event-bus sink %q requires Event output", operator.Name))
				}
				underlying := candidateEvent.Underlying()
				if schema, ok := d.engine.env.Schema(operator.EventType); ok && schema.Kind() == SchemaVariant {
					underlying = candidateEvent
				}
				if err := d.engine.Send(ctx, operator.EventType, underlying); err != nil {
					return err
				}
			}
		case CustomKind:
			runtime := d.runtimes[operator.Name]
			processed := make([]any, 0, len(current))
			for _, candidate := range current {
				values, err := runtime.Process(ctx, DataflowInput{Port: "in", Value: candidate})
				if err != nil {
					return err
				}
				for _, emission := range normalizeDataflowEmissions(values) {
					processed = append(processed, emission.Value)
				}
			}
			current = processed
		}
		elapsed := time.Since(started)
		d.recordOperatorElapsed(operator.Name, elapsed)
		if index+1 < len(d.definition.operators) {
			for _, candidate := range current {
				if _, signal := candidate.(DataflowSignal); !signal {
					d.recordOperatorSubmission(operator.Name, "out", elapsed)
				}
			}
		}
	}
	if countProcessed {
		d.processed.Add(1)
	}
	return nil
}

type dataflowWorkItem struct {
	operator string
	port     string
	value    any
}

type dataflowDispatchOwnerKey struct{}

func (d *DataflowInstance) processGraphEvent(ctx context.Context, event any) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if signal, ok := event.(DataflowSignal); ok {
		for _, operator := range d.definition.operators {
			switch operator.Kind {
			case BeaconSourceKind, EventBusSourceKind, EPStatementSourceKind:
				if err := d.processGraphFrom(ctx, signal, operator.Name); err != nil {
					return err
				}
			}
		}
		return nil
	}
	eventValue, ok := event.(Event)
	if !ok {
		return nil
	}
	starts := make([]string, 0)
	for _, operator := range d.definition.operators {
		if operator.Kind == EventBusSourceKind && dataflowEventTypeAccepts(d.engine, operator.EventType, eventValue) {
			starts = append(starts, operator.Name)
		}
	}
	if len(starts) == 0 {
		return nil
	}
	d.processed.Add(1)
	for _, start := range starts {
		if err := d.processGraphFrom(ctx, event, start); err != nil {
			return err
		}
	}
	return nil
}

func dataflowEventTypeAccepts(engine *Engine, eventType string, event Event) bool {
	if event.StreamType() != event.TypeName() {
		return eventType == event.StreamType()
	}
	if engine != nil && engine.env != nil {
		if schema, ok := engine.env.Schema(eventType); ok && schema.kind == SchemaVariant {
			return engine.env.variantAcceptsEventType(schema, event.TypeName())
		}
	}
	if eventType == event.TypeName() {
		return true
	}
	if engine == nil || engine.env == nil {
		return false
	}
	schema, ok := engine.env.Schema(eventType)
	if !ok {
		return false
	}
	if schema.kind == SchemaVariant {
		return engine.env.variantAcceptsEventType(schema, event.TypeName())
	}
	return schema.acceptsEventType(event.TypeName())
}

func (d *DataflowInstance) processGraphFrom(ctx context.Context, event any, start string) error {
	return d.processGraphQueue(ctx, []dataflowWorkItem{{operator: start, port: "in", value: event}})
}

func (d *DataflowInstance) processSourceEmission(ctx context.Context, source, port string, value any) error {
	queue := make([]dataflowWorkItem, 0, len(d.outgoing[source]))
	for _, edge := range d.outgoing[source] {
		if edge.FromPort == port {
			queue = append(queue, dataflowWorkItem{operator: edge.To, port: edge.ToPort, value: value})
		}
	}
	return d.processGraphQueue(ctx, queue)
}

func (d *DataflowInstance) processGraphQueue(ctx context.Context, queue []dataflowWorkItem) error {
	if d == nil {
		return NewError(ErrorState, "nil dataflow instance")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if owner, ok := ctx.Value(dataflowDispatchOwnerKey{}).(*DataflowInstance); ok && owner == d {
		// An EventBusSink can synchronously re-enter Engine.Send, which may
		// dispatch another source in this same instance before the outer graph
		// queue returns. It is already inside this instance's dispatch section,
		// so processing the nested queue directly avoids self-deadlock.
		return d.processGraphQueueLocked(ctx, queue)
	}
	// Source runtimes are started independently and may submit concurrently.
	// Serialize the complete work queue so stateful built-ins (notably the
	// statement-owned subquery registry) and custom operators see the same
	// single-input-at-a-time contract as the Java graph runtime.
	d.dispatchMu.Lock()
	defer d.dispatchMu.Unlock()
	owned := context.WithValue(ctx, dataflowDispatchOwnerKey{}, d)
	return d.processGraphQueueLocked(owned, queue)
}

func (d *DataflowInstance) processGraphQueueLocked(ctx context.Context, queue []dataflowWorkItem) error {
	for len(queue) > 0 {
		if err := contextErr(ctx); err != nil {
			return err
		}
		item := queue[0]
		queue = queue[1:]
		d.mu.Lock()
		running := d.state == DataflowRunning
		d.mu.Unlock()
		if !running {
			return nil
		}
		operator, ok := d.operators[item.operator]
		if !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("dataflow graph references unknown operator %q", item.operator))
		}
		if _, signal := item.value.(DataflowSignal); !signal {
			if expected := dataflowPortType(operator, false, item.port); expected != nil && !dataflowValueAssignable(item.value, expected) {
				return NewError(ErrorTypeMismatch, fmt.Sprintf("dataflow operator %q input port %q received %T, want %s", operator.Name, item.port, item.value, expected))
			}
		}
		started := time.Now()
		emissions, err := d.applyGraphOperator(ctx, operator, item.port, item.value)
		elapsed := time.Since(started)
		d.recordOperatorElapsed(operator.Name, elapsed)
		if err != nil {
			if handled := d.handleDataflowError(ctx, operator.Name, err); handled != nil {
				return handled
			}
			continue
		}
		normalized := normalizeDataflowEmissions(emissions)
		if err := validateDataflowEmissions(operator, normalized); err != nil {
			if handled := d.handleDataflowError(ctx, operator.Name, err); handled != nil {
				return handled
			}
			continue
		}
		for _, edge := range d.outgoing[operator.Name] {
			for _, emission := range normalized {
				if emission.Port == edge.FromPort {
					queue = append(queue, dataflowWorkItem{operator: edge.To, port: edge.ToPort, value: emission.Value})
				}
			}
		}
		for _, emission := range normalized {
			if _, signal := emission.Value.(DataflowSignal); signal {
				continue
			}
			connected := false
			for _, edge := range d.outgoing[operator.Name] {
				if edge.FromPort == emission.Port {
					connected = true
					break
				}
			}
			if connected {
				d.recordOperatorSubmission(operator.Name, emission.Port, elapsed)
			}
		}
	}
	return nil
}

func (d *DataflowInstance) applyGraphOperator(ctx context.Context, operator DataflowOperator, inputPort string, value any) ([]DataflowEmission, error) {
	if signal, ok := value.(DataflowSignal); ok {
		if operator.Signal != nil {
			if err := operator.Signal(ctx, signal); err != nil {
				return nil, err
			}
		}
		if operator.Kind == SelectKind && operator.SelectOptions.IterateOnFinalMarker && isDataflowFinalMarker(signal) {
			rows, err := d.processDataflowSelectFinalMarker(operator)
			if err != nil {
				return nil, err
			}
			emissions := make([]DataflowEmission, 0, len(rows))
			for _, row := range rows {
				emissions = append(emissions, Emit(row))
			}
			return emissions, nil
		}
		if operator.Kind == FilterKind && len(operator.OutputPorts) > 1 {
			emissions := make([]DataflowEmission, 0, len(operator.OutputPorts))
			for _, port := range operator.OutputPorts {
				emissions = append(emissions, EmitPort(port, signal))
			}
			return emissions, nil
		}
		if operator.Kind == CustomKind {
			runtime := d.runtimes[operator.Name]
			if signalRuntime, ok := runtime.(DataflowOperatorSignalRuntime); ok {
				return signalRuntime.OnSignal(ctx, signal)
			}
			return []DataflowEmission{Emit(signal)}, nil
		}
		if operator.Kind == EmitterKind {
			d.mu.Lock()
			d.signals = append(d.signals, signal)
			d.mu.Unlock()
		}
		return []DataflowEmission{Emit(signal)}, nil
	}
	switch operator.Kind {
	case CustomSourceKind:
		// A custom source enters the graph through DataflowEmitter. The source
		// node itself only forwards the submitted value to its outgoing edges.
		return []DataflowEmission{Emit(value)}, nil
	case CustomKind:
		runtime := d.runtimes[operator.Name]
		return runtime.Process(ctx, DataflowInput{Port: inputPort, Value: value})
	case FilterKind:
		if _, isEvent := value.(Event); !isEvent {
			if _, isRow := value.(Row); !isRow {
				return nil, nil
			}
		}
		evaluation, err := d.dataflowEvaluation(operator, value)
		if err != nil {
			return nil, err
		}
		result := operator.Predicate.eval(evaluation)
		pass, ok := boolValue(result)
		if !ok || !pass {
			_, rejectPort := dataflowFilterOutputPorts(operator)
			if rejectPort == "" {
				return nil, nil
			}
			return []DataflowEmission{EmitPort(rejectPort, value)}, nil
		}
		passPort, _ := dataflowFilterOutputPorts(operator)
		return []DataflowEmission{EmitPort(passPort, value)}, nil
	case SelectKind:
		if operator.JoinConfigured {
			rows, err := d.processDataflowSelectJoin(operator, inputPort, value)
			if err != nil {
				return nil, err
			}
			emissions := make([]DataflowEmission, 0, len(rows))
			for _, row := range rows {
				emissions = append(emissions, Emit(row))
			}
			return emissions, nil
		}
		if _, isEvent := value.(Event); !isEvent {
			if _, isRow := value.(Row); !isRow {
				return nil, nil
			}
		}
		rows, err := d.processDataflowSelect(operator, value)
		if err != nil {
			return nil, err
		}
		emissions := make([]DataflowEmission, 0, len(rows))
		for _, row := range rows {
			emissions = append(emissions, Emit(row))
		}
		return emissions, nil
	case EmitterKind:
		// An Emitter with outgoing edges is a captive source/forwarder. Only
		// terminal Emitters are sinks visible through Outputs; this preserves
		// the Java Emitter -> instream -> operator shape without recording the
		// source submission as an extra output row.
		if len(d.outgoing[operator.Name]) == 0 {
			d.mu.Lock()
			d.outputs = append(d.outputs, value)
			d.mu.Unlock()
			d.emitted.Add(1)
		}
		return []DataflowEmission{Emit(value)}, nil
	case LogSinkKind:
		if operator.Log != nil {
			if err := operator.Log(ctx, value); err != nil {
				return nil, err
			}
		}
		return []DataflowEmission{Emit(value)}, nil
	case EventBusSinkKind:
		if d.engine == nil {
			return nil, NewError(ErrorDependency, "dataflow event-bus sink has no engine")
		}
		eventValue, ok := value.(Event)
		if !ok {
			return nil, NewError(ErrorTypeMismatch, fmt.Sprintf("dataflow event-bus sink %q requires Event output", operator.Name))
		}
		underlying := eventValue.Underlying()
		if schema, ok := d.engine.env.Schema(operator.EventType); ok && schema.Kind() == SchemaVariant {
			underlying = eventValue
		}
		if err := d.engine.Send(ctx, operator.EventType, underlying); err != nil {
			return nil, err
		}
		return []DataflowEmission{Emit(value)}, nil
	default:
		return []DataflowEmission{Emit(value)}, nil
	}
}

func normalizeDataflowEmissions(emissions []DataflowEmission) []DataflowEmission {
	result := make([]DataflowEmission, len(emissions))
	copy(result, emissions)
	for index := range result {
		if result[index].Port == "" {
			result[index].Port = "out"
		}
	}
	return result
}

func validateDataflowEmissions(operator DataflowOperator, emissions []DataflowEmission) error {
	for _, emission := range emissions {
		if !dataflowPortAllowed(operator, true, emission.Port) {
			return NewError(ErrorInvalidRule, fmt.Sprintf("dataflow operator %q emitted on undeclared output port %q", operator.Name, emission.Port))
		}
		if _, signal := emission.Value.(DataflowSignal); signal {
			continue
		}
		if expected := dataflowPortType(operator, true, emission.Port); expected != nil && !dataflowValueAssignable(emission.Value, expected) {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("dataflow operator %q output port %q emitted %T, want %s", operator.Name, emission.Port, emission.Value, expected))
		}
	}
	return nil
}

func selectionFields(selections []Selection) []FieldSpec {
	fields := make([]FieldSpec, 0, len(selections))
	for _, selection := range selections {
		fields = append(fields, FieldSpec{Name: selection.Name, Type: selection.Expr.Type()})
	}
	return fields
}
