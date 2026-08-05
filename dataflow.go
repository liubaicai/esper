package esper

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
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

// DataflowOperatorContext describes the graph and operator instance supplied
// to a custom operator factory.  A fresh runtime is created for every
// DataflowInstance, matching Esper's factory/operator split.
type DataflowOperatorContext struct {
	DataflowName string
	InstanceID   string
	OperatorName string
	OperatorNum  int
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

// DataflowOperatorLifecycle is optional and is invoked once per instance.
type DataflowOperatorLifecycle interface {
	Open(context.Context) error
	Close(context.Context) error
}

// DataflowSourceRuntime produces values into a graph. A source receives its
// per-instance emitter at run time, so it can emit both registered Event
// values and raw values declared by typed output ports.
type DataflowSourceRuntime interface {
	Run(context.Context, *DataflowEmitter) error
}

// DataflowSourceFactory creates one source runtime for every dataflow
// instance. Source runtimes may also implement DataflowOperatorLifecycle.
type DataflowSourceFactory func(DataflowOperatorContext) (DataflowSourceRuntime, error)

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
	InstanceID       string
	UserObject       any
	ErrorPolicy      DataflowErrorPolicy
	ExceptionHandler DataflowExceptionHandler
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

func (b DataflowBuilder) Select(name string, selections ...Selection) DataflowBuilder {
	return b.add(DataflowOperator{Name: name, Kind: SelectKind, Selections: append([]Selection(nil), selections...)})
}

func (b DataflowBuilder) LogSink(name string, logger func(context.Context, any) error) DataflowBuilder {
	return b.add(DataflowOperator{Name: name, Kind: LogSinkKind, Log: logger})
}

// Custom adds a Go-native operator factory. The factory is called once for
// each instantiated dataflow and its runtime participates in graph processing
// and optional signal/lifecycle callbacks.
func (b DataflowBuilder) Custom(name string, factory DataflowOperatorFactory) DataflowBuilder {
	return b.CustomPorts(name, factory, []string{"in"}, []string{"out"})
}

// CustomPorts adds a custom operator with explicit named input and output
// ports. Runtime emissions are delivered only to edges matching their port.
func (b DataflowBuilder) CustomPorts(name string, factory DataflowOperatorFactory, inputs, outputs []string) DataflowBuilder {
	return b.add(DataflowOperator{
		Name:        name,
		Kind:        CustomKind,
		Factory:     factory,
		InputPorts:  append([]string(nil), inputs...),
		OutputPorts: append([]string(nil), outputs...),
	})
}

// CustomTypedPorts is the typed counterpart to CustomPorts. Port names remain
// explicit for graph readability while the generic descriptors add compile
// time intent and Build/runtime assignability checks.
func (b DataflowBuilder) CustomTypedPorts(name string, factory DataflowOperatorFactory, inputs, outputs []DataflowPort) DataflowBuilder {
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
	})
}

// CustomSource adds an idiomatic Go source operator. The source runs once per
// instantiated dataflow and emits into the graph through the supplied
// DataflowEmitter. Unlike a captive Emitter, a custom source can own its
// lifecycle and complete the graph when Run returns.
func (b DataflowBuilder) CustomSource(name string, factory DataflowSourceFactory) DataflowBuilder {
	return b.CustomTypedSource(name, factory, []DataflowPort{{Name: "out"}})
}

// CustomTypedSource is the typed counterpart to CustomSource. Source outputs
// are validated against downstream input ports at Build time and against
// actual values while the graph is running.
func (b DataflowBuilder) CustomTypedSource(name string, factory DataflowSourceFactory, outputs []DataflowPort) DataflowBuilder {
	outputNames, outputTypes := dataflowPortSpecs(outputs)
	return b.add(DataflowOperator{
		Name:            name,
		Kind:            CustomSourceKind,
		OutputPorts:     outputNames,
		OutputPortTypes: outputTypes,
		SourceFactory:   factory,
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
	for _, operator := range b.operators {
		if operator.Name == "" || operator.Kind == "" {
			return DataflowDefinition{}, NewError(ErrorInvalidRule, "dataflow operator requires a name and kind")
		}
		if _, exists := seen[operator.Name]; exists {
			return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow duplicates operator %q", operator.Name))
		}
		seen[operator.Name] = struct{}{}
		operatorsByName[operator.Name] = operator
		if err := validateDataflowPorts(operator); err != nil {
			return DataflowDefinition{}, err
		}
		switch operator.Kind {
		case FilterKind:
			if operator.Predicate == nil || operator.Predicate.Type() != typeOf[bool]() {
				return DataflowDefinition{}, NewError(ErrorTypeMismatch, fmt.Sprintf("dataflow filter %q requires bool predicate", operator.Name))
			}
		case SelectKind:
			if len(operator.Selections) == 0 {
				return DataflowDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("dataflow select %q requires projections", operator.Name))
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
	operators := append([]DataflowOperator(nil), b.operators...)
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

// SaveDataflowConfiguration records a registered definition under its own
// stable name so it can be looked up and instantiated later. This is an
// in-process saved configuration: operator factories remain Go values and
// are intentionally not serialized as executable code.
func (e *Environment) SaveDataflowConfiguration(name string) error {
	if e == nil {
		return NewError(ErrorDependency, "nil environment")
	}
	if name == "" {
		return NewError(ErrorInvalidRule, "dataflow configuration name is required")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	definition, ok := e.dataflows[name]
	if !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("dataflow %q is not registered", name))
	}
	if e.savedDataflows == nil {
		e.savedDataflows = make(map[string]DataflowDefinition)
	}
	e.savedDataflows[name] = cloneDataflowDefinition(definition)
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

type DataflowInstance struct {
	mu                     sync.Mutex
	engine                 *Engine
	definition             DataflowDefinition
	options                DataflowOptions
	state                  DataflowState
	outputs                []any
	signals                []DataflowSignal
	eventTypes             map[string]struct{}
	operators              map[string]DataflowOperator
	outgoing               map[string][]DataflowEdge
	graph                  bool
	statementSubscriptions []*Subscription
	runtimes               map[string]DataflowOperatorRuntime
	sources                map[string]DataflowSourceRuntime
	subqueryRegistries     map[string]*subqueryRuntimeRegistry
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
		outgoing:           make(map[string][]DataflowEdge),
		graph:              len(registered.edges) > 0,
		runtimes:           make(map[string]DataflowOperatorRuntime),
		sources:            make(map[string]DataflowSourceRuntime),
		subqueryRegistries: make(map[string]*subqueryRuntimeRegistry),
		done:               make(chan struct{}),
	}
	for operatorNumber, operator := range registered.operators {
		instance.operators[operator.Name] = operator
		if operator.Kind == EventBusSourceKind {
			instance.eventTypes[operator.EventType] = struct{}{}
		}
		if operator.Kind == CustomKind {
			runtime, err := operator.Factory(DataflowOperatorContext{
				DataflowName: registered.name,
				InstanceID:   options.InstanceID,
				OperatorName: operator.Name,
				OperatorNum:  operatorNumber,
			})
			if err != nil {
				for _, created := range instance.runtimes {
					if lifecycle, ok := created.(DataflowOperatorLifecycle); ok {
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
			source, err := operator.SourceFactory(DataflowOperatorContext{
				DataflowName: registered.name,
				InstanceID:   options.InstanceID,
				OperatorName: operator.Name,
				OperatorNum:  operatorNumber,
			})
			if err != nil {
				for _, created := range instance.runtimes {
					if lifecycle, ok := created.(DataflowOperatorLifecycle); ok {
						_ = lifecycle.Close(context.Background())
					}
				}
				for _, created := range instance.sources {
					if lifecycle, ok := created.(DataflowOperatorLifecycle); ok {
						_ = lifecycle.Close(context.Background())
					}
				}
				return nil, err
			}
			if source == nil {
				for _, created := range instance.runtimes {
					if lifecycle, ok := created.(DataflowOperatorLifecycle); ok {
						_ = lifecycle.Close(context.Background())
					}
				}
				for _, created := range instance.sources {
					if lifecycle, ok := created.(DataflowOperatorLifecycle); ok {
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
	hasOutgoing := len(d.outgoing[name]) > 0
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
	return d.processSourceEmission(ctx, name, port, value)
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
			if lifecycle, ok := runtime.(DataflowOperatorLifecycle); ok {
				if err := lifecycle.Open(ctx); err != nil {
					return fmt.Errorf("dataflow operator %q open: %w", operator.Name, err)
				}
			}
		}
		source := d.sources[operator.Name]
		if source != nil {
			if lifecycle, ok := source.(DataflowOperatorLifecycle); ok {
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
			if lifecycle, ok := runtime.(DataflowOperatorLifecycle); ok {
				_ = lifecycle.Close(ctx)
			}
		}
		source := d.sources[operator.Name]
		if source != nil {
			if lifecycle, ok := source.(DataflowOperatorLifecycle); ok {
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
	err := source.Run(runCtx, emitter)
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
				event, ok := result.Event()
				if !ok {
					continue
				}
				var processErr error
				if d.graph {
					d.processed.Add(1)
					processErr = d.processGraphFrom(callbackCtx, event, operator.Name)
				} else {
					processErr = d.process(callbackCtx, event)
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
	if d.graph {
		return d.processGraphEvent(ctx, event)
	}
	if err := d.processLinear(ctx, event); err != nil {
		return d.handleDataflowError(ctx, "", err)
	}
	return nil
}

func (d *DataflowInstance) dataflowEvaluation(operator DataflowOperator, event Event) (EvalContext, error) {
	if d == nil || d.engine == nil {
		return EvalContext{}, NewError(ErrorDependency, "dataflow operator has no engine")
	}
	now := d.engine.Now()
	variables := d.engine.Variables()
	if registry := d.subqueryRegistries[operator.Name]; registry != nil {
		if err := registry.accept(event, now, variables); err != nil {
			return EvalContext{}, err
		}
		variables = registry.attachVariables(variables)
	}
	return EvalContext{Event: event, Engine: d.engine, Now: now, Variables: variables}, nil
}

func (d *DataflowInstance) evaluateDataflowSelect(operator DataflowOperator, event Event) (Row, error) {
	evaluation, err := d.dataflowEvaluation(operator, event)
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
		for _, operator := range d.definition.operators {
			next := make([]any, 0, len(current))
			for _, candidate := range current {
				candidateSignal, isSignal := candidate.(DataflowSignal)
				if isSignal && operator.Signal != nil {
					if err := operator.Signal(ctx, candidateSignal); err != nil {
						return err
					}
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
	current := []any{event}
	for _, operator := range d.definition.operators {
		switch operator.Kind {
		case BeaconSourceKind, EPStatementSourceKind:
			continue
		case EventBusSourceKind:
			if !dataflowEventTypeAccepts(d.engine, operator.EventType, eventValue) {
				current = nil
			}
		case FilterKind:
			filtered := make([]any, 0, len(current))
			for _, candidate := range current {
				eventValue, ok := candidate.(Event)
				if !ok {
					continue
				}
				evaluation, err := d.dataflowEvaluation(operator, eventValue)
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
				eventValue, ok := candidate.(Event)
				if !ok {
					continue
				}
				row, err := d.evaluateDataflowSelect(operator, eventValue)
				if err != nil {
					return err
				}
				selected = append(selected, row)
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
	}
	d.processed.Add(1)
	return nil
}

type dataflowWorkItem struct {
	operator string
	port     string
	value    any
}

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
		emissions, err := d.applyGraphOperator(ctx, operator, item.port, item.value)
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
				if emission.Port != edge.FromPort {
					continue
				}
				queue = append(queue, dataflowWorkItem{operator: edge.To, port: edge.ToPort, value: emission.Value})
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
		eventValue, ok := value.(Event)
		if !ok {
			return nil, nil
		}
		evaluation, err := d.dataflowEvaluation(operator, eventValue)
		if err != nil {
			return nil, err
		}
		result := operator.Predicate.eval(evaluation)
		pass, ok := boolValue(result)
		if !ok || !pass {
			return nil, nil
		}
		return []DataflowEmission{Emit(value)}, nil
	case SelectKind:
		eventValue, ok := value.(Event)
		if !ok {
			return nil, nil
		}
		row, err := d.evaluateDataflowSelect(operator, eventValue)
		if err != nil {
			return nil, err
		}
		return []DataflowEmission{Emit(row)}, nil
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
