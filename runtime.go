package esper

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const maxRoutedEventsPerSend = 1024

// Listener receives one deterministic new/old-stream batch.
type Listener func(context.Context, ResultBatch) error

// Sink is the chain endpoint for asynchronous integration adapters. The
// default Engine path still invokes it synchronously for Esper-like ordering.
type Sink interface {
	Write(context.Context, ResultBatch) error
}

type SinkFunc func(context.Context, ResultBatch) error

func (f SinkFunc) Write(ctx context.Context, batch ResultBatch) error {
	if f == nil {
		return nil
	}
	return f(ctx, batch)
}

// Result is either an underlying Event or an ordered projection Row.
type Result struct {
	event *Event
	row   *Row
}

func resultEvent(event Event) Result { return Result{event: &event} }
func resultRow(row Row) Result       { return Result{row: &row} }

func (r Result) IsEvent() bool { return r.event != nil }
func (r Result) IsRow() bool   { return r.row != nil }

func (r Result) Event() (Event, bool) {
	if r.event == nil {
		return Event{}, false
	}
	return *r.event, true
}

func (r Result) Row() (Row, bool) {
	if r.row == nil {
		return Row{}, false
	}
	return *r.row, true
}

func (r Result) Underlying() any {
	if r.event != nil {
		return r.event.Underlying()
	}
	if r.row != nil {
		return *r.row
	}
	return nil
}

func (r Result) Get(name string) Value {
	if r.event != nil {
		return r.event.Get(name)
	}
	if r.row != nil {
		return r.row.Get(name)
	}
	return Missing()
}

// ResultBatch preserves listener call boundaries, logical time and sequence.
type ResultBatch struct {
	New             []Result
	Old             []Result
	Sequence        uint64
	Time            time.Time
	outputCountsSet bool
	outputInserted  int64
	outputRemoved   int64
}

func (b ResultBatch) empty() bool { return len(b.New) == 0 && len(b.Old) == 0 }

func (b ResultBatch) clone() ResultBatch {
	b.New = append([]Result(nil), b.New...)
	b.Old = append([]Result(nil), b.Old...)
	return b
}

type VirtualClock struct {
	mu  sync.RWMutex
	now time.Time
}

func NewVirtualClock(start time.Time) *VirtualClock {
	if start.IsZero() {
		start = time.Unix(0, 0).UTC()
	}
	return &VirtualClock{now: start}
}

func (c *VirtualClock) Now() time.Time {
	if c == nil {
		return time.Time{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}

func (c *VirtualClock) Advance(at time.Time) error {
	if c == nil {
		return fmt.Errorf("esper: nil clock")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if at.Before(c.now) {
		return fmt.Errorf("esper: clock cannot move backwards from %s to %s", c.now, at)
	}
	c.now = at
	return nil
}

type engineConfig struct {
	clock          *VirtualClock
	matchRecognize MatchRecognizeRuntimeConfig
}

type EngineOption func(*engineConfig)

func WithClock(clock *VirtualClock) EngineOption {
	return func(cfg *engineConfig) { cfg.clock = clock }
}

func WithStartTime(start time.Time) EngineOption {
	return func(cfg *engineConfig) { cfg.clock = NewVirtualClock(start) }
}

// Engine owns deployed statements and the explicit processing clock.
type Engine struct {
	mu                                sync.Mutex
	env                               *Environment
	clock                             *VirtualClock
	matchRecognizeStatePool           *rowRecogStatePool
	matchRecognizeStateLimitListeners []MatchRecognizeStateLimitListener
	pendingMatchRecognizeStateLimits  []MatchRecognizeStateLimitEvent
	variables                         map[string]Value
	contextVariables                  map[string]map[string]map[string]Value
	contextPartitionRefs              map[string]map[string]int
	contextPartitionIDs               map[string]map[string]int
	contextPartitionNextIDs           map[string]int
	contextPartitionInstanceNextIDs   map[string]uint64
	contextPartitionDescriptors       map[string]map[string]ContextPartitionDescriptor
	contextPartitionListeners         map[string][]ContextPartitionStateListener
	contextTemporalOrigins            map[string]time.Time
	contextStateListeners             []ContextStateListener
	contextCreated                    map[string]bool
	contextStatementRefs              map[string]int
	pendingContextEvents              []contextNotification
	variableChangeListeners           map[string][]VariableChangeListener
	pendingVariableChanges            []VariableChangeEvent
	tables                            map[string]*Table
	namedWindows                      map[string]*NamedWindow
	statements                        map[string]*Statement
	deployments                       map[string]*Deployment
	dataflows                         map[*DataflowInstance]struct{}
	savedDataflowInstances            map[string]*DataflowInstance
	pendingStatementDispatches        []statementDispatch
	pendingNamedWindowDispatches      []namedWindowDispatch
	pendingRoutedEvents               []Event
	closed                            bool
	nextID                            uint64
}

func NewEngine(env *Environment, options ...EngineOption) *Engine {
	cfg := engineConfig{
		clock: NewVirtualClock(time.Unix(0, 0).UTC()),
		matchRecognize: MatchRecognizeRuntimeConfig{
			MaxStates:    -1,
			PreventStart: true,
		},
	}
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	if cfg.clock == nil {
		cfg.clock = NewVirtualClock(time.Unix(0, 0).UTC())
	}
	engine := &Engine{
		env:                             env,
		clock:                           cfg.clock,
		matchRecognizeStatePool:         newRowRecogStatePool(cfg.matchRecognize),
		variables:                       make(map[string]Value),
		contextVariables:                make(map[string]map[string]map[string]Value),
		contextPartitionRefs:            make(map[string]map[string]int),
		contextPartitionIDs:             make(map[string]map[string]int),
		contextPartitionNextIDs:         make(map[string]int),
		contextPartitionInstanceNextIDs: make(map[string]uint64),
		contextPartitionDescriptors:     make(map[string]map[string]ContextPartitionDescriptor),
		contextPartitionListeners:       make(map[string][]ContextPartitionStateListener),
		contextTemporalOrigins:          make(map[string]time.Time),
		contextCreated:                  make(map[string]bool),
		contextStatementRefs:            make(map[string]int),
		variableChangeListeners:         make(map[string][]VariableChangeListener),
		tables:                          make(map[string]*Table),
		namedWindows:                    make(map[string]*NamedWindow),
		statements:                      make(map[string]*Statement),
		deployments:                     make(map[string]*Deployment),
		dataflows:                       make(map[*DataflowInstance]struct{}),
		savedDataflowInstances:          make(map[string]*DataflowInstance),
	}
	if env != nil {
		env.mu.RLock()
		for name, definition := range env.contexts {
			engine.contextCreated[name] = true
			if definition.isTemporal() {
				engine.contextTemporalOrigins[name] = engine.clock.Now()
			}
		}
		for name, definition := range env.variables {
			if definition.context == "" {
				engine.variables[name] = definition.initial
			}
		}
		for name, definition := range env.tables {
			engine.tables[name] = newTable(definition)
		}
		for name, definition := range env.namedWindows {
			engine.namedWindows[name] = newNamedWindow(definition, engine)
		}
		env.mu.RUnlock()
	}
	return engine
}

func (e *Environment) NewEngine(options ...EngineOption) *Engine { return NewEngine(e, options...) }

func (e *Engine) Now() time.Time {
	if e == nil || e.clock == nil {
		return time.Time{}
	}
	return e.clock.Now()
}

type VariableAssignment struct {
	Name  string
	Value any
}

// SetVariable atomically replaces a runtime variable value. All statements
// processing the next event or clock transition observe the same snapshot.
func (e *Engine) SetVariable(ctx context.Context, name string, value any) error {
	return e.SetVariables(ctx, VariableAssignment{Name: name, Value: value})
}

// SetVariables validates the complete batch before changing any value. This
// is the all-or-nothing boundary used by Context and trigger integrations.
func (e *Engine) SetVariables(ctx context.Context, assignments ...VariableAssignment) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	e.mu.Lock()
	err := e.setVariablesLocked(ctx, assignments)
	changes := e.takeVariableChangesLocked()
	e.mu.Unlock()
	e.dispatchVariableChanges(changes)
	return err
}

// setVariablesLocked applies a validated variable batch while the engine lock
// is already held. Event-triggered on-set actions use this path so they do not
// re-enter Engine.mu while Send is dispatching statements.
func (e *Engine) setVariablesLocked(ctx context.Context, assignments []VariableAssignment) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if e.closed {
		return NewError(ErrorState, "engine is closed")
	}
	e.refreshVariablesLocked()
	validated := make([]VariableAssignment, 0, len(assignments))
	seen := make(map[string]struct{}, len(assignments))
	for _, assignment := range assignments {
		if _, exists := seen[assignment.Name]; exists {
			return NewError(ErrorInvalidRule, fmt.Sprintf("variable %q is assigned more than once", assignment.Name))
		}
		seen[assignment.Name] = struct{}{}
		definition, ok := e.env.Variable(assignment.Name)
		if !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("variable %q is not registered", assignment.Name))
		}
		if definition.context != "" {
			return NewError(ErrorState, fmt.Sprintf("variable %q is context-partitioned; use SetContextVariable", assignment.Name))
		}
		if definition.constant {
			return NewError(ErrorState, fmt.Sprintf("variable %q is constant", assignment.Name))
		}
		coerced, err := definition.coerce(assignment.Value)
		if err != nil {
			return WrapError(ErrorTypeMismatch, "variable."+assignment.Name, err)
		}
		validated = append(validated, VariableAssignment{Name: assignment.Name, Value: coerced})
	}
	changes := make([]VariableChangeEvent, 0, len(validated))
	for _, assignment := range validated {
		oldValue := e.variables[assignment.Name]
		newValue := Present(assignment.Value)
		if assignment.Value == nil {
			newValue = Null()
		}
		e.variables[assignment.Name] = newValue
		changes = append(changes, VariableChangeEvent{
			Name:        assignment.Name,
			PartitionID: -1,
			Old:         oldValue,
			New:         newValue,
		})
	}
	e.pendingVariableChanges = append(e.pendingVariableChanges, changes...)
	return nil
}

// GetVariable returns the current value without exposing the engine's map.
func (e *Engine) GetVariable(name string) (Value, bool) {
	if e == nil {
		return Missing(), false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.refreshVariablesLocked()
	value, ok := e.variables[name]
	return value, ok
}

// Variables returns a point-in-time copy of all runtime variable values.
func (e *Engine) Variables() map[string]Value {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.refreshVariablesLocked()
	return cloneValues(e.variables)
}

func (e *Engine) refreshVariablesLocked() {
	if e == nil || e.env == nil {
		return
	}
	e.env.mu.RLock()
	defer e.env.mu.RUnlock()
	for name, definition := range e.env.variables {
		if definition.context != "" {
			continue
		}
		if _, exists := e.variables[name]; !exists {
			e.variables[name] = definition.initial
		}
	}
}

func (e *Engine) ensureContextVariablePartitionLocked(contextName, partitionKey string) map[string]Value {
	if e == nil || e.env == nil || contextName == "" || partitionKey == "" {
		return nil
	}
	e.env.mu.RLock()
	definitions := make(map[string]VariableDefinition)
	for name, definition := range e.env.variables {
		if definition.context == contextName {
			definitions[name] = definition
		}
	}
	e.env.mu.RUnlock()
	if len(definitions) == 0 {
		return nil
	}
	byContext := e.contextVariables[contextName]
	if byContext == nil {
		byContext = make(map[string]map[string]Value)
		e.contextVariables[contextName] = byContext
	}
	state := byContext[partitionKey]
	if state == nil {
		state = make(map[string]Value, len(definitions))
		for name, definition := range definitions {
			state[name] = definition.initial
		}
		byContext[partitionKey] = state
		return state
	}
	for name, definition := range definitions {
		if _, exists := state[name]; !exists {
			state[name] = definition.initial
		}
	}
	return state
}

func (e *Engine) allocateContextPartitionIDLocked(contextName, partitionKey string) int {
	if e == nil || contextName == "" || partitionKey == "" {
		return 0
	}
	byContext := e.contextPartitionIDs[contextName]
	if byContext == nil {
		byContext = make(map[string]int)
		e.contextPartitionIDs[contextName] = byContext
	}
	if id, exists := byContext[partitionKey]; exists {
		return id
	}
	id := e.contextPartitionNextIDs[contextName]
	e.contextPartitionNextIDs[contextName] = id + 1
	byContext[partitionKey] = id
	return id
}

func (e *Engine) allocateOverlappingContextPartitionKeyLocked(contextName, baseKey string) string {
	if e == nil || contextName == "" {
		return baseKey
	}
	next := e.contextPartitionInstanceNextIDs[contextName]
	e.contextPartitionInstanceNextIDs[contextName] = next + 1
	return fmt.Sprintf("%s\x1einstance:%d", baseKey, next)
}

func (e *Engine) retainContextPartitionLocked(contextName, partitionKey string, runtime ...*statementRuntime) {
	if e == nil || contextName == "" || partitionKey == "" {
		return
	}
	byContext := e.contextPartitionRefs[contextName]
	if byContext == nil {
		byContext = make(map[string]int)
		e.contextPartitionRefs[contextName] = byContext
	}
	if byContext[partitionKey] == 0 {
		if e.contextPartitionDescriptors[contextName] == nil {
			e.contextPartitionDescriptors[contextName] = make(map[string]ContextPartitionDescriptor)
		}
		descriptor := ContextPartitionDescriptor{ContextName: contextName, Key: partitionKey, BaseKey: contextPartitionBaseKey(partitionKey), ID: e.allocateContextPartitionIDLocked(contextName, partitionKey)}
		if len(runtime) > 0 && runtime[0] != nil {
			descriptor = newContextPartitionDescriptor(contextName, partitionKey, runtime[0])
			descriptor.ID = e.allocateContextPartitionIDLocked(contextName, partitionKey)
		}
		e.contextPartitionDescriptors[contextName][partitionKey] = descriptor
		e.pendingContextEvents = append(e.pendingContextEvents, contextNotification{
			kind: contextNotificationPartitionAllocated,
			partition: ContextPartitionStateEvent{
				ContextName: contextName,
				PartitionID: descriptor.ID,
				Key:         partitionKey,
				BaseKey:     descriptor.BaseKey,
				Descriptor:  descriptor,
				Allocated:   true,
			},
		})
	}
	byContext[partitionKey]++
}

func (e *Engine) releaseContextPartitionLocked(contextName, partitionKey string, runtime ...*statementRuntime) {
	if e == nil || contextName == "" || partitionKey == "" {
		return
	}
	if len(runtime) > 0 && runtime[0] != nil {
		e.releaseRowRecogRuntimeLocked(runtime[0])
	}
	byContext := e.contextPartitionRefs[contextName]
	if byContext == nil {
		return
	}
	byContext[partitionKey]--
	if byContext[partitionKey] > 0 {
		return
	}
	for _, window := range e.namedWindows {
		if window != nil {
			window.releaseContextPartition(contextName, partitionKey)
		}
	}
	descriptor := e.contextPartitionDescriptors[contextName][partitionKey]
	if len(runtime) > 0 && runtime[0] != nil {
		descriptor = newContextPartitionDescriptor(contextName, partitionKey, runtime[0])
		descriptor.ID = e.allocateContextPartitionIDLocked(contextName, partitionKey)
	}
	delete(byContext, partitionKey)
	if len(byContext) == 0 {
		delete(e.contextPartitionRefs, contextName)
	}
	if states := e.contextVariables[contextName]; states != nil {
		delete(states, partitionKey)
		if len(states) == 0 {
			delete(e.contextVariables, contextName)
		}
	}
	if descriptors := e.contextPartitionDescriptors[contextName]; descriptors != nil {
		delete(descriptors, partitionKey)
		if len(descriptors) == 0 {
			delete(e.contextPartitionDescriptors, contextName)
		}
	}
	e.pendingContextEvents = append(e.pendingContextEvents, contextNotification{
		kind: contextNotificationPartitionDeallocated,
		partition: ContextPartitionStateEvent{
			ContextName: contextName,
			PartitionID: descriptor.ID,
			Key:         partitionKey,
			BaseKey:     descriptor.BaseKey,
			Descriptor:  descriptor,
			Allocated:   false,
		},
	})
}

// AddContextPartitionStateListener registers a context partition lifecycle
// listener. Callbacks are delivered outside the engine lock after the
// current event/timer operation commits its state.
func (e *Engine) AddContextPartitionStateListener(contextName string, listener ContextPartitionStateListener) error {
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	if contextName == "" {
		return NewError(ErrorInvalidRule, "context name is required")
	}
	if _, ok := e.env.Context(contextName); !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", contextName))
	}
	if listener == nil {
		return NewError(ErrorInvalidRule, "context partition listener is nil")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.contextPartitionListeners[contextName] = append(e.contextPartitionListeners[contextName], listener)
	return nil
}

// AddContextPartitionListener is the concise alias for
// AddContextPartitionStateListener.
func (e *Engine) AddContextPartitionListener(contextName string, listener ContextPartitionStateListener) error {
	return e.AddContextPartitionStateListener(contextName, listener)
}

// RemoveContextPartitionStateListener removes one previously registered
// listener. Removing an unknown listener is a no-op.
func (e *Engine) RemoveContextPartitionStateListener(contextName string, listener ContextPartitionStateListener) {
	if e == nil || listener == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	listeners := e.contextPartitionListeners[contextName]
	for index, candidate := range listeners {
		if sameContextPartitionListener(candidate, listener) {
			listeners = append(listeners[:index], listeners[index+1:]...)
			break
		}
	}
	if len(listeners) == 0 {
		delete(e.contextPartitionListeners, contextName)
	} else {
		e.contextPartitionListeners[contextName] = listeners
	}
}

// RemoveContextPartitionListener is the concise alias for
// RemoveContextPartitionStateListener.
func (e *Engine) RemoveContextPartitionListener(contextName string, listener ContextPartitionStateListener) {
	e.RemoveContextPartitionStateListener(contextName, listener)
}

// ContextPartitionStateListeners returns a registration-order snapshot.
func (e *Engine) ContextPartitionStateListeners(contextName string) []ContextPartitionStateListener {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]ContextPartitionStateListener(nil), e.contextPartitionListeners[contextName]...)
}

// RemoveContextPartitionStateListeners removes all listeners for one context.
func (e *Engine) RemoveContextPartitionStateListeners(contextName string) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.contextPartitionListeners, contextName)
}

func sameContextPartitionListener(left, right ContextPartitionStateListener) bool {
	if left == nil || right == nil || reflect.TypeOf(left) != reflect.TypeOf(right) {
		return false
	}
	typ := reflect.TypeOf(left)
	if typ.Comparable() {
		return left == right
	}
	leftValue := reflect.ValueOf(left)
	rightValue := reflect.ValueOf(right)
	if leftValue.Kind() == reflect.Pointer && rightValue.Kind() == reflect.Pointer {
		return leftValue.Pointer() == rightValue.Pointer()
	}
	return false
}

// AddContextStateListener registers a listener for context creation and
// destruction. Existing Environment context definitions are replayed to the
// new listener outside the engine lock.
func (e *Engine) AddContextStateListener(listener ContextStateListener) error {
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	if listener == nil {
		return NewError(ErrorInvalidRule, "context state listener is nil")
	}
	e.mu.Lock()
	e.contextStateListeners = append(e.contextStateListeners, listener)
	contextNames := make([]string, 0, len(e.contextCreated))
	for name, created := range e.contextCreated {
		if created {
			contextNames = append(contextNames, name)
		}
	}
	e.mu.Unlock()
	sort.Strings(contextNames)
	for _, contextName := range contextNames {
		listener.OnContextCreated(ContextStateEvent{ContextName: contextName})
	}
	return nil
}

// RemoveContextStateListener removes one global context listener.
func (e *Engine) RemoveContextStateListener(listener ContextStateListener) {
	if e == nil || listener == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for index, candidate := range e.contextStateListeners {
		if sameContextStateListener(candidate, listener) {
			e.contextStateListeners = append(e.contextStateListeners[:index], e.contextStateListeners[index+1:]...)
			break
		}
	}
}

// ContextStateListeners returns a registration-order snapshot.
func (e *Engine) ContextStateListeners() []ContextStateListener {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]ContextStateListener(nil), e.contextStateListeners...)
}

// RemoveContextStateListeners removes all global context listeners.
func (e *Engine) RemoveContextStateListeners() {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.contextStateListeners = nil
}

func sameContextStateListener(left, right ContextStateListener) bool {
	if left == nil || right == nil || reflect.TypeOf(left) != reflect.TypeOf(right) {
		return false
	}
	typ := reflect.TypeOf(left)
	if typ.Comparable() {
		return left == right
	}
	leftValue := reflect.ValueOf(left)
	rightValue := reflect.ValueOf(right)
	if leftValue.Kind() == reflect.Pointer && rightValue.Kind() == reflect.Pointer {
		return leftValue.Pointer() == rightValue.Pointer()
	}
	return false
}

func (e *Engine) takeContextEventsLocked() []contextNotification {
	if e == nil || len(e.pendingContextEvents) == 0 {
		return nil
	}
	events := append([]contextNotification(nil), e.pendingContextEvents...)
	e.pendingContextEvents = nil
	return events
}

func (e *Engine) dispatchContextEvents(events []contextNotification) {
	if e == nil {
		return
	}
	for _, notification := range events {
		switch notification.kind {
		case contextNotificationCreated, contextNotificationDestroyed:
			e.mu.Lock()
			listeners := append([]ContextStateListener(nil), e.contextStateListeners...)
			e.mu.Unlock()
			for _, listener := range listeners {
				if notification.kind == contextNotificationCreated {
					listener.OnContextCreated(notification.state)
				} else {
					listener.OnContextDestroyed(notification.state)
				}
			}
		case contextNotificationStatementAdded, contextNotificationActivated, contextNotificationStatementRemoved, contextNotificationDeactivated:
			e.mu.Lock()
			listeners := append([]ContextPartitionStateListener(nil), e.contextPartitionListeners[notification.state.ContextName]...)
			e.mu.Unlock()
			for _, listener := range listeners {
				lifecycle, ok := listener.(ContextPartitionLifecycleListener)
				if !ok {
					continue
				}
				switch notification.kind {
				case contextNotificationStatementAdded:
					lifecycle.OnContextStatementAdded(notification.state)
				case contextNotificationActivated:
					lifecycle.OnContextActivated(notification.state)
				case contextNotificationStatementRemoved:
					lifecycle.OnContextStatementRemoved(notification.state)
				case contextNotificationDeactivated:
					lifecycle.OnContextDeactivated(notification.state)
				}
			}
		case contextNotificationPartitionAllocated, contextNotificationPartitionDeallocated:
			e.mu.Lock()
			listeners := append([]ContextPartitionStateListener(nil), e.contextPartitionListeners[notification.partition.ContextName]...)
			e.mu.Unlock()
			for _, listener := range listeners {
				if notification.kind == contextNotificationPartitionAllocated {
					listener.OnContextPartitionAllocated(notification.partition)
				} else {
					listener.OnContextPartitionDeallocated(notification.partition)
				}
			}
		}
	}
}

func contextStateEventForStatement(statement *Statement) ContextStateEvent {
	event := ContextStateEvent{}
	if statement == nil {
		return event
	}
	event.ContextName = statement.plan.query.contextName
	event.StatementName = statement.name
	if statement.deployment != nil {
		event.StatementDeploymentID = statement.deployment.id
	}
	return event
}

func (e *Engine) ensureContextCreatedLocked(contextName string) {
	if e == nil || e.env == nil || contextName == "" || e.contextCreated[contextName] {
		return
	}
	if _, ok := e.env.Context(contextName); !ok {
		return
	}
	e.contextCreated[contextName] = true
	e.pendingContextEvents = append(e.pendingContextEvents, contextNotification{
		kind:  contextNotificationCreated,
		state: ContextStateEvent{ContextName: contextName},
	})
}

func (e *Engine) queueContextStatementAddedLocked(statement *Statement) {
	if e == nil || statement == nil || statement.plan.query.contextName == "" {
		return
	}
	contextName := statement.plan.query.contextName
	e.ensureContextCreatedLocked(contextName)
	e.pendingContextEvents = append(e.pendingContextEvents, contextNotification{
		kind:  contextNotificationStatementAdded,
		state: contextStateEventForStatement(statement),
	})
	e.contextStatementRefs[contextName]++
	if e.contextStatementRefs[contextName] == 1 {
		e.pendingContextEvents = append(e.pendingContextEvents, contextNotification{
			kind:  contextNotificationActivated,
			state: ContextStateEvent{ContextName: contextName},
		})
	}
}

// queueContextStatementRemovedLocked returns true when the removed statement
// was the last statement using the context. The caller queues deactivation
// after releasing its partitions to preserve Esper's event order.
func (e *Engine) queueContextStatementRemovedLocked(statement *Statement) bool {
	if e == nil || statement == nil || statement.plan.query.contextName == "" {
		return false
	}
	contextName := statement.plan.query.contextName
	if e.contextStatementRefs[contextName] <= 0 {
		return false
	}
	e.pendingContextEvents = append(e.pendingContextEvents, contextNotification{
		kind:  contextNotificationStatementRemoved,
		state: contextStateEventForStatement(statement),
	})
	e.contextStatementRefs[contextName]--
	if e.contextStatementRefs[contextName] == 0 {
		delete(e.contextStatementRefs, contextName)
		return true
	}
	return false
}

func (e *Engine) queueContextDeactivatedLocked(contextName string) {
	if e == nil || contextName == "" {
		return
	}
	e.pendingContextEvents = append(e.pendingContextEvents, contextNotification{
		kind:  contextNotificationDeactivated,
		state: ContextStateEvent{ContextName: contextName},
	})
}

func (e *Engine) queueContextDestroyedLocked(contextName string) {
	if e == nil || contextName == "" {
		return
	}
	e.pendingContextEvents = append(e.pendingContextEvents, contextNotification{
		kind:  contextNotificationDestroyed,
		state: ContextStateEvent{ContextName: contextName},
	})
}

func (e *Engine) setContextVariableValuesLocked(contextName, partitionKey string, assignments []VariableAssignment) error {
	validated, err := e.validateContextVariableAssignmentsLocked(contextName, assignments)
	if err != nil {
		return err
	}
	state := e.ensureContextVariablePartitionLocked(contextName, partitionKey)
	if state == nil {
		return NewError(ErrorUnknownName, fmt.Sprintf("context %q has no context variables", contextName))
	}
	partitionID := -1
	if descriptors := e.contextPartitionDescriptors[contextName]; descriptors != nil {
		if descriptor, ok := descriptors[partitionKey]; ok {
			partitionID = descriptor.ID
		}
	}
	changes := make([]VariableChangeEvent, 0, len(validated))
	for _, assignment := range validated {
		oldValue := state[assignment.Name]
		newValue := Present(assignment.Value)
		if assignment.Value == nil {
			newValue = Null()
		}
		state[assignment.Name] = newValue
		changes = append(changes, VariableChangeEvent{
			Name:         assignment.Name,
			ContextName:  contextName,
			PartitionID:  partitionID,
			PartitionKey: partitionKey,
			Old:          oldValue,
			New:          newValue,
		})
	}
	e.pendingVariableChanges = append(e.pendingVariableChanges, changes...)
	return nil
}

func (e *Engine) validateContextVariableAssignmentsLocked(contextName string, assignments []VariableAssignment) ([]VariableAssignment, error) {
	validated := make([]VariableAssignment, 0, len(assignments))
	seen := make(map[string]struct{}, len(assignments))
	for _, assignment := range assignments {
		if _, exists := seen[assignment.Name]; exists {
			return nil, NewError(ErrorInvalidRule, fmt.Sprintf("context variable %q is assigned more than once", assignment.Name))
		}
		seen[assignment.Name] = struct{}{}
		definition, ok := e.env.Variable(assignment.Name)
		if !ok || definition.context != contextName {
			return nil, NewError(ErrorUnknownName, fmt.Sprintf("context variable %q is not registered for context %q", assignment.Name, contextName))
		}
		if definition.constant {
			return nil, NewError(ErrorState, fmt.Sprintf("context variable %q is constant", assignment.Name))
		}
		coerced, err := definition.coerce(assignment.Value)
		if err != nil {
			return nil, WrapError(ErrorTypeMismatch, "context-variable."+assignment.Name, err)
		}
		validated = append(validated, VariableAssignment{Name: assignment.Name, Value: coerced})
	}
	return validated, nil
}

// SetContextVariable changes one context-partitioned variable for a public
// partition key. The key uses the same contract as ContextPartitionKeys.
func (e *Engine) SetContextVariable(ctx context.Context, contextName, partitionKey, name string, value any) error {
	return e.SetContextVariables(ctx, contextName, partitionKey, VariableAssignment{Name: name, Value: value})
}

// SetContextVariables atomically updates multiple variables in one context
// partition. Validation and coercion complete before any value is changed.
func (e *Engine) SetContextVariables(ctx context.Context, contextName, partitionKey string, assignments ...VariableAssignment) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return NewError(ErrorState, "engine is closed")
	}
	err := e.setContextVariableValuesLocked(contextName, partitionKey, assignments)
	changes := e.takeVariableChangesLocked()
	e.mu.Unlock()
	e.dispatchVariableChanges(changes)
	return err
}

// SetContextVariableByID updates one active context partition using the
// engine-level partition ID exposed by ContextPartitionDescriptor.
func (e *Engine) SetContextVariableByID(ctx context.Context, contextName string, partitionID int, name string, value any) error {
	return e.SetContextVariablesByID(ctx, contextName, partitionID, VariableAssignment{Name: name, Value: value})
}

// SetContextVariablesByID atomically updates variables for one active context
// partition. Unlike the key-based API it never creates state for an unknown or
// already deallocated partition ID.
func (e *Engine) SetContextVariablesByID(ctx context.Context, contextName string, partitionID int, assignments ...VariableAssignment) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return NewError(ErrorState, "engine is closed")
	}
	descriptors := e.contextPartitionDescriptors[contextName]
	partitionKey := ""
	for key, descriptor := range descriptors {
		if descriptor.ID == partitionID {
			partitionKey = key
			break
		}
	}
	if partitionKey == "" {
		e.mu.Unlock()
		return NewError(ErrorUnknownName, fmt.Sprintf("context %q partition %d is not active", contextName, partitionID))
	}
	err := e.setContextVariableValuesLocked(contextName, partitionKey, assignments)
	changes := e.takeVariableChangesLocked()
	e.mu.Unlock()
	e.dispatchVariableChanges(changes)
	return err
}

// GetContextVariable returns one context-partitioned value.
func (e *Engine) GetContextVariable(ctx context.Context, contextName, partitionKey, name string) (Value, bool) {
	if err := contextErr(ctx); err != nil || e == nil || e.env == nil {
		return Missing(), false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	definition, ok := e.env.Variable(name)
	if !ok || definition.context != contextName {
		return Missing(), false
	}
	state := e.ensureContextVariablePartitionLocked(contextName, partitionKey)
	value, exists := state[name]
	return value, exists
}

// ContextVariableValues returns a point-in-time copy of all context variable
// values for one partition key.
func (e *Engine) ContextVariableValues(ctx context.Context, contextName, partitionKey string) map[string]Value {
	if err := contextErr(ctx); err != nil || e == nil || e.env == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return cloneValues(e.ensureContextVariablePartitionLocked(contextName, partitionKey))
}

func (e *Engine) Table(name string) (*Table, bool) {
	if e == nil {
		return nil, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	table, ok := e.tables[name]
	return table, ok
}

func (e *Engine) NamedWindow(name string) (*NamedWindow, bool) {
	if e == nil {
		return nil, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	window, ok := e.namedWindows[name]
	return window, ok
}

func (e *Engine) InsertNamedWindow(ctx context.Context, name string, underlying any) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if e == nil {
		return NewError(ErrorDependency, "nil engine")
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return NewError(ErrorState, "engine is closed")
	}
	window, ok := e.namedWindows[name]
	if !ok {
		e.mu.Unlock()
		return NewError(ErrorUnknownName, fmt.Sprintf("named window %q is not registered", name))
	}
	now := e.clock.Now()
	e.refreshVariablesLocked()
	variables := cloneValues(e.variables)
	delta, err := window.insertWithVariables(now, underlying, variables)
	if err != nil {
		e.mu.Unlock()
		return err
	}
	statements := e.sortedStatementsLocked()
	e.pendingStatementDispatches = nil
	e.pendingNamedWindowDispatches = nil
	e.pendingRoutedEvents = nil
	e.pendingContextEvents = nil
	e.pendingVariableChanges = nil
	e.pendingMatchRecognizeStateLimits = nil
	dispatches := make([]statementDispatch, 0, len(statements))
	for _, statement := range statements {
		batch, changed, processErr := statement.processNamedWindow(ctx, now, delta, variables)
		if processErr != nil {
			e.mu.Unlock()
			return processErr
		}
		if err := e.applyStatementOutputAssignmentsLocked(ctx, statement, &variables); err != nil {
			e.mu.Unlock()
			return err
		}
		if changed {
			dispatches = append(dispatches, statementDispatch{statement: statement, batch: batch})
			if routeErr := e.queueStatementRoutesLocked(statement, batch, now); routeErr != nil {
				e.mu.Unlock()
				return routeErr
			}
		}
	}
	if err := e.processPendingRoutedEventsLocked(ctx, now, variables, &dispatches); err != nil {
		e.mu.Unlock()
		return err
	}
	dispatches = append(dispatches, e.pendingStatementDispatches...)
	nestedNamedWindowDispatches := append([]namedWindowDispatch(nil), e.pendingNamedWindowDispatches...)
	variableChanges := e.takeVariableChangesLocked()
	contextEvents := e.takeContextEventsLocked()
	e.pendingStatementDispatches = nil
	e.pendingNamedWindowDispatches = nil
	e.pendingRoutedEvents = nil
	e.mu.Unlock()
	e.dispatchVariableChanges(variableChanges)
	e.dispatchContextEvents(contextEvents)
	e.dispatchMatchRecognizeStateLimitEvents()
	if err := dispatchAll(ctx, dispatches); err != nil {
		return err
	}
	for _, dispatch := range nestedNamedWindowDispatches {
		if err := dispatch.window.dispatch(ctx, dispatch.delta); err != nil {
			return err
		}
	}
	return window.dispatch(ctx, delta)
}

func cloneValues(values map[string]Value) map[string]Value {
	if values == nil {
		return nil
	}
	result := make(map[string]Value, len(values))
	for name, value := range values {
		result[name] = value
	}
	return result
}

type StatementState uint8

const (
	StatementStarted StatementState = iota
	StatementStopped
	StatementDestroyed
)

type Statement struct {
	mu                       sync.RWMutex
	engine                   *Engine
	deployment               *Deployment
	plan                     Plan
	parameters               ParameterValues
	id                       string
	name                     string
	runtime                  statementRuntime
	listeners                map[uint64]Listener
	nextSubID                uint64
	state                    StatementState
	closed                   bool
	closeOnce                sync.Once
	pendingOutputAssignments []VariableAssignment
}

func (s *Statement) ID() string {
	if s == nil {
		return ""
	}
	return s.id
}
func (s *Statement) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}
func (s *Statement) Plan() Plan {
	if s == nil {
		return Plan{}
	}
	return s.plan
}

// Snapshot returns the statement's current iterator view without dispatching
// a listener batch or changing runtime state. Match-recognize statements use
// their retained recognition branches; ordinary statements use the current
// source projection snapshot.
func (s *Statement) Snapshot(ctx context.Context) (QueryResult, error) {
	return s.SnapshotWithSelector(ctx, nil)
}

// SnapshotWithSelector returns the statement's current iterator view limited
// to context partitions accepted by selector. It is the Go-style equivalent
// of Esper's statement iterator(selector)/safeIterator(selector) boundary:
// selection is validated before state is read, and callbacks are not
// dispatched or runtime state mutated.
//
// A selector is only valid for a statement that has a context. Use nil to
// snapshot a non-context statement or to select all partitions of a context.
func (s *Statement) SnapshotWithSelector(ctx context.Context, selector ContextPartitionSelector) (QueryResult, error) {
	if err := contextErr(ctx); err != nil {
		return QueryResult{}, err
	}
	if s == nil {
		return QueryResult{}, NewError(ErrorState, "nil statement")
	}
	if selector != nil {
		if s.plan.query.contextName == "" {
			return QueryResult{}, NewError(ErrorInvalidRule, "context partition selector requires a context statement")
		}
		definition, ok := s.plan.query.env.Context(s.plan.query.contextName)
		if !ok {
			return QueryResult{}, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", s.plan.query.contextName))
		}
		if err := validateContextPartitionSelector(definition, selector); err != nil {
			return QueryResult{}, err
		}
	}
	if s.engine != nil {
		s.engine.mu.Lock()
		defer s.engine.mu.Unlock()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.state == StatementDestroyed {
		return QueryResult{}, NewError(ErrorState, "statement is destroyed")
	}
	var now time.Time
	var variables map[string]Value
	if s.engine != nil {
		s.engine.refreshVariablesLocked()
		now = s.engine.clock.Now()
		variables = variablesWithEngineLockState(statementVariables(cloneValues(s.engine.variables), s.parameters), s.engine, true)
	} else {
		now = time.Now()
		variables = variablesWithEngineLockState(statementVariables(cloneValues(s.runtime.variables), s.parameters), s.engine, true)
	}
	var result ResultBatch
	if s.plan.query.contextName != "" {
		result = ResultBatch{Time: now}
		partitionKeys := make([]string, 0, len(s.runtime.partitions))
		for key := range s.runtime.partitions {
			partitionKeys = append(partitionKeys, key)
		}
		sort.Strings(partitionKeys)
		for _, key := range partitionKeys {
			partition := s.runtime.partitions[key]
			if partition == nil {
				continue
			}
			descriptor := newContextPartitionDescriptor(s.plan.query.contextName, key, partition)
			if !contextPartitionSelectedWithDescriptor(selector, descriptor) {
				continue
			}
			partBatch := partition.snapshotQuery(s.plan, now, variables)
			result.New = append(result.New, partBatch.New...)
			result.Old = append(result.Old, partBatch.Old...)
		}
	} else {
		result = s.runtime.snapshotQuery(s.plan, now, variables)
	}
	return QueryResult{Batch: result}, nil
}

func (s *Statement) collectOutputAssignmentsLocked() {
	if s == nil {
		return
	}
	assignments := s.runtime.drainOutputAssignments()
	if len(assignments) > 0 {
		s.pendingOutputAssignments = append(s.pendingOutputAssignments, assignments...)
	}
}

func (s *Statement) takeOutputAssignments() []VariableAssignment {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	assignments := append([]VariableAssignment(nil), s.pendingOutputAssignments...)
	s.pendingOutputAssignments = nil
	return assignments
}

// ContextPartitionCount and ContextPartitionKeys expose the currently
// materialized key partitions for diagnostics and selector-style tooling.
func (s *Statement) ContextPartitionCount() int {
	return len(s.ContextPartitionKeys())
}

func (s *Statement) ContextPartitionCountWith(selector ContextPartitionSelector) int {
	return len(s.ContextPartitionKeysWith(selector))
}

func (s *Statement) ContextPartitionKeysWith(selector ContextPartitionSelector) []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.runtime.partitions))
	for key := range s.runtime.partitions {
		partition := s.runtime.partitions[key]
		partitionID := 0
		if partition != nil {
			partitionID = partition.partitionID
		}
		descriptor := newContextPartitionDescriptor(s.plan.query.contextName, key, partition)
		if descriptor.ID == 0 && partition != nil {
			descriptor.ID = partitionID
		}
		if contextPartitionSelectedWithDescriptor(selector, descriptor) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func (s *Statement) ContextPartitionKeys() []string {
	return s.ContextPartitionKeysWith(nil)
}

// ContextPartitions returns a stable, sorted snapshot of all currently
// materialized context partitions. It is the descriptor-oriented counterpart
// to ContextPartitionKeys and is safe for callers to retain after this method
// returns.
func (s *Statement) ContextPartitions() []ContextPartitionDescriptor {
	return s.ContextPartitionsWith(nil)
}

// ContextPartitionsWith returns descriptors for the partitions accepted by
// selector. Selection uses the same public partition key contract as
// ContextPartitionKeysWith.
func (s *Statement) ContextPartitionsWith(selector ContextPartitionSelector) []ContextPartitionDescriptor {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.runtime.partitions))
	for key := range s.runtime.partitions {
		descriptor := newContextPartitionDescriptor(s.plan.query.contextName, key, s.runtime.partitions[key])
		if contextPartitionSelectedWithDescriptor(selector, descriptor) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	result := make([]ContextPartitionDescriptor, 0, len(keys))
	for _, key := range keys {
		partition := s.runtime.partitions[key]
		if partition == nil {
			continue
		}
		result = append(result, newContextPartitionDescriptor(s.plan.query.contextName, key, partition))
	}
	return result
}

func (s *Statement) State() StatementState {
	if s == nil {
		return StatementDestroyed
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *Statement) Stop(ctx context.Context) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if s == nil {
		return NewError(ErrorState, "nil statement")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.state == StatementDestroyed {
		return NewError(ErrorState, "statement is destroyed")
	}
	s.state = StatementStopped
	return nil
}

func (s *Statement) Start(ctx context.Context) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if s == nil {
		return NewError(ErrorState, "nil statement")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.state == StatementDestroyed {
		return NewError(ErrorState, "statement is destroyed")
	}
	s.state = StatementStarted
	return nil
}

func (s *Statement) Destroy(ctx context.Context) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if s == nil {
		return nil
	}
	s.markClosed()
	return nil
}

func (s *Statement) Subscribe(listener Listener) (Subscription, error) {
	if s == nil {
		return Subscription{}, fmt.Errorf("esper: nil statement")
	}
	if listener == nil {
		return Subscription{}, fmt.Errorf("esper: listener is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Subscription{}, fmt.Errorf("esper: statement %q is closed", s.name)
	}
	s.nextSubID++
	id := s.nextSubID
	s.listeners[id] = listener
	return Subscription{statement: s, id: id}, nil
}

func (s *Statement) removeSubscription(id uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.listeners[id]; !exists {
		return nil
	}
	delete(s.listeners, id)
	return nil
}

type Subscription struct {
	statement     *Statement
	id            uint64
	namedWindow   *NamedWindow
	namedWindowID uint64
	once          sync.Once
	err           error
}

func (s *Subscription) Close() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		if s.statement != nil {
			s.err = s.statement.removeSubscription(s.id)
		} else if s.namedWindow != nil {
			s.err = s.namedWindow.removeSubscription(s.namedWindowID)
		}
	})
	return s.err
}

type Deployment struct {
	mu         sync.RWMutex
	engine     *Engine
	id         string
	statements []*Statement
	closed     bool
}

func (d *Deployment) ID() string {
	if d == nil {
		return ""
	}
	return d.id
}

func (d *Deployment) Statements() []*Statement {
	if d == nil {
		return nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return append([]*Statement(nil), d.statements...)
}

func (d *Deployment) Undeploy(ctx context.Context) error {
	if d == nil || d.engine == nil {
		return nil
	}
	return d.engine.Undeploy(ctx, d.id)
}

func (e *Engine) Deploy(ctx context.Context, plan Plan) (*Deployment, error) {
	return e.deploy(ctx, plan, nil, false)
}

// DeployWithParameters deploys a live statement with one immutable binding
// snapshot for its substitution parameters. The values are copied at deploy
// time and are not retained through the caller's map after this call returns.
func (e *Engine) DeployWithParameters(ctx context.Context, plan Plan, parameters ParameterValues) (*Deployment, error) {
	return e.deploy(ctx, plan, parameters, true)
}

func (e *Engine) deploy(ctx context.Context, plan Plan, parameters ParameterValues, parameterized bool) (*Deployment, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if e == nil || e.env == nil {
		return nil, NewError(ErrorDependency, "engine has no environment")
	}
	if plan.query.env != e.env || plan.schemaVersion == "" {
		return nil, NewError(ErrorDependency, "plan does not belong to this engine")
	}
	parameterTypes, parameterErr := queryParameterTypes(e.env, plan.query)
	if parameterErr != nil {
		return nil, WrapError(ErrorInvalidRule, "parameters", parameterErr)
	}
	if len(parameterTypes) > 0 && !parameterized {
		return nil, NewError(ErrorInvalidRule, "statement plan contains substitution parameters; use DeployWithParameters")
	}
	if parameterized {
		if err := validateParameterBindings(parameterTypes, parameters); err != nil {
			return nil, err
		}
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, NewError(ErrorState, "engine is closed")
	}
	if plan.query.contextName != "" {
		definition, ok := e.env.Context(plan.query.contextName)
		if !ok {
			e.mu.Unlock()
			return nil, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", plan.query.contextName))
		}
		if definition.isTemporal() {
			if _, exists := e.contextTemporalOrigins[plan.query.contextName]; !exists {
				e.contextTemporalOrigins[plan.query.contextName] = e.clock.Now()
			}
		}
	}
	e.nextID++
	deploymentID := fmt.Sprintf("deployment-%d", e.nextID)
	name := plan.query.name
	if name == "" {
		name = "statement-" + plan.hash[:12]
	}
	if _, exists := e.statements[name]; exists {
		e.mu.Unlock()
		return nil, NewError(ErrorDeployment, fmt.Sprintf("statement name %q is already deployed", name))
	}
	statement := &Statement{
		engine:     e,
		id:         deploymentID + ":" + name,
		name:       name,
		plan:       plan,
		parameters: cloneParameterValues(parameters),
		listeners:  make(map[uint64]Listener),
		state:      StatementStarted,
		runtime:    newStatementRuntime(plan.query),
	}
	statement.runtime.engine = e
	statement.runtime.rowRecogOwner = statement.id
	if plan.query.rowRecog != nil {
		e.matchRecognizeStatePool.register(statement.id)
	}
	e.refreshVariablesLocked()
	statement.runtime.variables = statementVariables(e.variables, statement.parameters)
	statement.runtime.subqueryRegistry = newSubqueryRuntimeRegistry(e.env, e, plan.query)
	statement.runtime.variables = statement.runtime.subqueryRegistry.attachVariables(statement.runtime.variables)
	statement.runtime.initializeAt(e.clock.Now())
	if plan.query.contextName != "" {
		if definition, ok := e.env.Context(plan.query.contextName); ok && definition.kind == ContextInitiatedTerminated && definition.startPattern != nil {
			// Anchor pure timer-root Context patterns at deployment time, just
			// like a standalone Pattern statement. This preserves interval and
			// calendar occurrences when the first clock advance jumps over more
			// than one due instant.
			initializeContextPatternTimer(&statement.runtime.contextStartPatternState, definition.startPattern, e.clock.Now(), statement.runtime.variables)
		}
	}
	if err := e.seedNamedWindowRowRecogLocked(ctx, statement); err != nil {
		e.matchRecognizeStatePool.removeOwner(statement.id)
		e.mu.Unlock()
		return nil, err
	}
	deployment := &Deployment{engine: e, id: deploymentID, statements: []*Statement{statement}}
	statement.deployment = deployment
	e.statements[name] = statement
	e.deployments[deploymentID] = deployment
	if plan.query.contextName != "" {
		e.ensureContextCreatedLocked(plan.query.contextName)
		e.queueContextStatementAddedLocked(statement)
		if definition, ok := e.env.Context(plan.query.contextName); ok && definition.isTemporal() {
			_, _ = statement.syncTemporalContextLocked(e.clock.Now())
		}
	}
	contextEvents := e.takeContextEventsLocked()
	e.mu.Unlock()
	e.dispatchContextEvents(contextEvents)
	e.notifyDataflowStatementDeployed(statement)
	return deployment, nil
}

// seedNamedWindowRowRecogLocked replays the retained contents of a named
// window into a newly deployed Match Recognize statement. Esper attaches the
// row-recognition consumer to the current named-window state, so an event
// arriving after deployment can complete a sequence that started before the
// consumer was deployed. The replay advances recognition state but deliberately
// discards the resulting listener batch; deployment must not emit historical
// rows as new output.
//
// This is intentionally limited to non-context row-recognition consumers. A
// context statement needs to allocate context partitions through
// Statement.process, which cannot be called while Engine.deploy holds the
// engine lock; context-owned named-window replay remains a separate lifecycle
// path.
func (e *Engine) seedNamedWindowRowRecogLocked(ctx context.Context, statement *Statement) error {
	if e == nil || statement == nil || statement.plan.query.rowRecog == nil || statement.plan.query.trigger != nil || statement.plan.query.contextName != "" {
		return nil
	}
	base, err := sourceNode(statement.plan.query.rowRecog.input)
	if err != nil || base == nil || base.kind != streamNamedWindow {
		return nil
	}
	events, err := e.snapshotFireAndForgetSourceLocked(ctx, base, e.clock.Now(), statement.runtime.variables)
	if err != nil {
		return err
	}
	statement.runtime.ctx = ctx
	for _, event := range events {
		if _, _, err := statement.runtime.process(statement.plan, event, e.clock.Now(), statement.runtime.variables); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) Undeploy(ctx context.Context, deploymentID string) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if e == nil {
		return nil
	}
	e.mu.Lock()
	deployment, exists := e.deployments[deploymentID]
	if !exists {
		e.mu.Unlock()
		return NewError(ErrorDeployment, fmt.Sprintf("deployment %q not found", deploymentID))
	}
	delete(e.deployments, deploymentID)
	removedStatements := append([]*Statement(nil), deployment.statements...)
	for _, statement := range deployment.statements {
		delete(e.statements, statement.name)
		statement.markClosedLocked()
	}
	deployment.mu.Lock()
	deployment.closed = true
	deployment.mu.Unlock()
	contextEvents := e.takeContextEventsLocked()
	e.mu.Unlock()
	e.dispatchContextEvents(contextEvents)
	for _, statement := range removedStatements {
		e.notifyDataflowStatementUndeployed(statement)
	}
	return closeDeploymentSinks(deployment)
}

func closeDeploymentSinks(deployment *Deployment) error {
	for _, statement := range deployment.Statements() {
		if statement.plan.query.sink == nil {
			continue
		}
		if closer, ok := statement.plan.query.sink.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				return fmt.Errorf("esper: close sink for %q: %w", statement.name, err)
			}
		}
	}
	return nil
}

func (s *Statement) markClosed() {
	if s == nil {
		return
	}
	if s.engine != nil {
		s.engine.mu.Lock()
		s.markClosedLocked()
		events := s.engine.takeContextEventsLocked()
		s.engine.mu.Unlock()
		s.engine.dispatchContextEvents(events)
		return
	}
	s.markClosedLocked()
}

func (s *Statement) releaseContextPartitionsLocked() {
	if s == nil || s.engine == nil || s.plan.query.contextName == "" {
		return
	}
	keys := make([]string, 0, len(s.runtime.partitions))
	for key := range s.runtime.partitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		s.engine.releaseContextPartitionLocked(s.plan.query.contextName, key, s.runtime.partitions[key])
	}
	s.runtime.partitions = make(map[string]*statementRuntime)
}

func (s *Statement) markClosedLocked() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		s.mu.Lock()
		lastContextStatement := false
		if s.engine != nil {
			lastContextStatement = s.engine.queueContextStatementRemovedLocked(s)
		}
		s.releaseContextPartitionsLocked()
		if s.engine != nil {
			s.engine.releaseRowRecogRuntimeLocked(&s.runtime)
			s.engine.matchRecognizeStatePool.removeOwner(s.id)
		}
		if lastContextStatement {
			s.engine.queueContextDeactivatedLocked(s.plan.query.contextName)
		}
		s.closed = true
		s.state = StatementDestroyed
		s.listeners = make(map[uint64]Listener)
		s.mu.Unlock()
	})
}

func (e *Engine) Send(ctx context.Context, eventType string, underlying any) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	schema, ok := e.env.Schema(eventType)
	if !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("event type %q is not registered", eventType))
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return NewError(ErrorState, "engine is closed")
	}
	now := e.clock.Now()
	if schema.Kind() == SchemaVariant {
		routed, ok := underlying.(Event)
		if !ok || !routed.Schema().valid() {
			e.mu.Unlock()
			return NewError(ErrorTypeMismatch, fmt.Sprintf("variant event type %q requires a routed member Event", eventType))
		}
		if !e.env.variantAcceptsEventType(schema, routed.TypeName()) {
			e.mu.Unlock()
			return NewError(ErrorTypeMismatch, fmt.Sprintf("event type %q is not a valid member of variant schema %q", routed.TypeName(), eventType))
		}
		if _, registered := e.env.Schema(routed.TypeName()); !registered {
			e.mu.Unlock()
			return NewError(ErrorUnknownName, fmt.Sprintf("variant member event type %q is not registered", routed.TypeName()))
		}
	}
	event, err := newEvent(schema, underlying, now)
	if err != nil {
		e.mu.Unlock()
		return WrapError(ErrorTypeMismatch, "event."+eventType, err)
	}
	e.refreshVariablesLocked()
	variables := cloneValues(e.variables)
	e.pendingStatementDispatches = nil
	e.pendingNamedWindowDispatches = nil
	e.pendingRoutedEvents = nil
	e.pendingContextEvents = nil
	e.pendingVariableChanges = nil
	e.pendingMatchRecognizeStateLimits = nil
	dispatches := make([]statementDispatch, 0)
	routedQueue := []Event{event}
	processedEvents := make([]Event, 0, 1)
	processedRoutes := 0
	for len(routedQueue) > 0 {
		if err := contextErr(ctx); err != nil {
			e.mu.Unlock()
			return err
		}
		current := routedQueue[0]
		routedQueue = routedQueue[1:]
		processedEvents = append(processedEvents, current)
		if processedRoutes >= maxRoutedEventsPerSend {
			e.mu.Unlock()
			return NewError(ErrorState, fmt.Sprintf("route event limit %d exceeded", maxRoutedEventsPerSend))
		}
		processedRoutes++
		statements := e.sortedStatementsLocked()
		for _, statement := range statements {
			batch, changed, processErr := statement.process(ctx, now, current, variables)
			if processErr != nil {
				e.mu.Unlock()
				return processErr
			}
			if err := e.applyStatementOutputAssignmentsLocked(ctx, statement, &variables); err != nil {
				e.mu.Unlock()
				return err
			}
			if changed {
				dispatches = append(dispatches, statementDispatch{statement: statement, batch: batch})
				if routeErr := e.queueStatementRoutesLocked(statement, batch, now); routeErr != nil {
					e.mu.Unlock()
					return routeErr
				}
			}
		}
		dispatches = append(dispatches, e.pendingStatementDispatches...)
		e.pendingStatementDispatches = nil
		routedQueue = append(routedQueue, e.pendingRoutedEvents...)
		e.pendingRoutedEvents = nil
	}
	nestedNamedWindowDispatches := append([]namedWindowDispatch(nil), e.pendingNamedWindowDispatches...)
	variableChanges := e.takeVariableChangesLocked()
	contextEvents := e.takeContextEventsLocked()
	e.pendingStatementDispatches = nil
	e.pendingNamedWindowDispatches = nil
	e.pendingRoutedEvents = nil
	e.mu.Unlock()
	e.dispatchVariableChanges(variableChanges)
	e.dispatchContextEvents(contextEvents)
	e.dispatchMatchRecognizeStateLimitEvents()
	if err := dispatchAll(ctx, dispatches); err != nil {
		return err
	}
	for _, dispatch := range nestedNamedWindowDispatches {
		if err := dispatch.window.dispatch(ctx, dispatch.delta); err != nil {
			return err
		}
	}
	for _, processed := range processedEvents {
		if err := e.dispatchDataflowEvent(ctx, processed); err != nil {
			return err
		}
	}
	return nil
}

// EventSchema returns the immutable schema registered for an event type.
// Connectors use this small inspection hook to choose the correct payload
// bridge without reaching into Engine's runtime services.
func (e *Engine) EventSchema(eventType string) (Schema, bool) {
	if e == nil || e.env == nil {
		return Schema{}, false
	}
	return e.env.Schema(eventType)
}

// SendRecord sends a map-backed record to any registered non-variant event
// schema. Struct-backed schemas are materialized from the named fields before
// entering the normal Send path; map/JSON/Avro schemas retain the map. This is
// the connector-friendly counterpart to SendEvent and keeps adapters from
// depending on private schema internals.
func (e *Engine) SendRecord(ctx context.Context, eventType string, record map[string]any) error {
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	schema, ok := e.env.Schema(eventType)
	if !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("event type %q is not registered", eventType))
	}
	if record == nil {
		return NewError(ErrorTypeMismatch, fmt.Sprintf("event type %q record is nil", eventType))
	}
	if schema.Kind() == SchemaVariant {
		return NewError(ErrorTypeMismatch, fmt.Sprintf("variant event type %q requires a routed member Event", eventType))
	}
	underlying := any(record)
	if schema.goType != nil {
		var err error
		underlying, err = mergeSchemaUnderlying(schema, nil, record)
		if err != nil {
			return WrapError(ErrorTypeMismatch, "event."+eventType, err)
		}
	} else {
		normalized := make(map[string]any, len(record))
		for name, value := range record {
			normalized[name] = value
		}
		for _, field := range schema.fields {
			value, exists := normalized[field.Name]
			if !exists || value == nil || field.Type == nil || field.Type == typeOf[any]() {
				continue
			}
			converted, err := assignReflectValue(field.Type, value)
			if err != nil {
				return WrapError(ErrorTypeMismatch, "event."+eventType+"."+field.Name, err)
			}
			normalized[field.Name] = converted.Interface()
		}
		underlying = normalized
	}
	return e.Send(ctx, eventType, underlying)
}

func (e *Engine) SendEvent(ctx context.Context, underlying any) error {
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	typ := reflect.TypeOf(underlying)
	if typ == nil {
		return NewError(ErrorTypeMismatch, "cannot infer event type from nil")
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	e.env.mu.RLock()
	name, ok := e.env.typeToName[typ]
	e.env.mu.RUnlock()
	if !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("no registered event type for Go type %s", typ))
	}
	return e.Send(ctx, name, underlying)
}

func (e *Engine) SendJSON(ctx context.Context, eventType string, data []byte) error {
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	schema, ok := e.env.Schema(eventType)
	if !ok || schema.Kind() != SchemaJSON {
		return NewError(ErrorUnknownName, fmt.Sprintf("JSON event type %q is not registered", eventType))
	}
	event, err := ParseJSON(schema, data, e.Now())
	if err != nil {
		return err
	}
	return e.Send(ctx, eventType, event.Underlying())
}

func (e *Engine) SendXML(ctx context.Context, eventType string, data []byte) error {
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	schema, ok := e.env.Schema(eventType)
	if !ok || schema.Kind() != SchemaXML {
		return NewError(ErrorUnknownName, fmt.Sprintf("XML event type %q is not registered", eventType))
	}
	event, err := ParseXML(schema, data, e.Now())
	if err != nil {
		return err
	}
	return e.Send(ctx, eventType, event.Underlying())
}

func (e *Engine) SendAvroJSON(ctx context.Context, eventType string, data []byte) error {
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	schema, ok := e.env.Schema(eventType)
	if !ok || schema.Kind() != SchemaAvro {
		return NewError(ErrorUnknownName, fmt.Sprintf("Avro event type %q is not registered", eventType))
	}
	event, err := ParseAvroJSON(schema, data, e.Now())
	if err != nil {
		return err
	}
	return e.Send(ctx, eventType, event.Underlying())
}

// SendObjectArray sends a positional event to an ObjectArray schema. The
// schema validates field count and performs safe numeric coercion before the
// event enters the normal runtime routing path.
func (e *Engine) SendObjectArray(ctx context.Context, eventType string, values []any) error {
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	schema, ok := e.env.Schema(eventType)
	if !ok || schema.Kind() != SchemaObjectArray {
		return NewError(ErrorUnknownName, fmt.Sprintf("object-array event type %q is not registered", eventType))
	}
	return e.Send(ctx, eventType, values)
}

func (e *Engine) Route(ctx context.Context, eventType string, underlying any) error {
	return e.Send(ctx, eventType, underlying)
}

func (e *Engine) AdvanceTime(ctx context.Context, at time.Time) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if e == nil {
		return NewError(ErrorDependency, "nil engine")
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return NewError(ErrorState, "engine is closed")
	}
	if err := e.clock.Advance(at); err != nil {
		e.mu.Unlock()
		return err
	}
	e.refreshVariablesLocked()
	variables := cloneValues(e.variables)
	statements := e.sortedStatementsLocked()
	e.pendingStatementDispatches = nil
	e.pendingNamedWindowDispatches = nil
	e.pendingRoutedEvents = nil
	e.pendingContextEvents = nil
	e.pendingVariableChanges = nil
	e.pendingMatchRecognizeStateLimits = nil
	dispatches := make([]statementDispatch, 0, len(statements))
	namedWindowDispatches := make([]namedWindowDispatch, 0, len(e.namedWindows))
	for _, name := range sortedNamedWindowNames(e.namedWindows) {
		window := e.namedWindows[name]
		if delta := window.expire(at); !delta.empty() {
			namedWindowDispatches = append(namedWindowDispatches, namedWindowDispatch{window: window, delta: delta})
			for _, statement := range statements {
				batch, changed, processErr := statement.processNamedWindow(ctx, at, delta, variables)
				if processErr != nil {
					e.mu.Unlock()
					return processErr
				}
				if err := e.applyStatementOutputAssignmentsLocked(ctx, statement, &variables); err != nil {
					e.mu.Unlock()
					return err
				}
				if changed {
					dispatches = append(dispatches, statementDispatch{statement: statement, batch: batch})
					if routeErr := e.queueStatementRoutesLocked(statement, batch, at); routeErr != nil {
						e.mu.Unlock()
						return routeErr
					}
				}
			}
		}
	}
	for _, statement := range statements {
		batch, changed := statement.expire(at, variables)
		if err := e.applyStatementOutputAssignmentsLocked(ctx, statement, &variables); err != nil {
			e.mu.Unlock()
			return err
		}
		if changed {
			dispatches = append(dispatches, statementDispatch{statement: statement, batch: batch})
			if routeErr := e.queueStatementRoutesLocked(statement, batch, at); routeErr != nil {
				e.mu.Unlock()
				return routeErr
			}
		}
	}
	if err := e.processPendingRoutedEventsLocked(ctx, at, variables, &dispatches); err != nil {
		e.mu.Unlock()
		return err
	}
	dispatches = append(dispatches, e.pendingStatementDispatches...)
	namedWindowDispatches = append(namedWindowDispatches, e.pendingNamedWindowDispatches...)
	variableChanges := e.takeVariableChangesLocked()
	contextEvents := e.takeContextEventsLocked()
	e.pendingStatementDispatches = nil
	e.pendingNamedWindowDispatches = nil
	e.pendingRoutedEvents = nil
	dataflows := make([]*DataflowInstance, 0, len(e.dataflows))
	for instance := range e.dataflows {
		dataflows = append(dataflows, instance)
	}
	e.mu.Unlock()
	e.dispatchVariableChanges(variableChanges)
	e.dispatchContextEvents(contextEvents)
	e.dispatchMatchRecognizeStateLimitEvents()
	if err := dispatchAll(ctx, dispatches); err != nil {
		return err
	}
	for _, dispatch := range namedWindowDispatches {
		if err := dispatch.window.dispatch(ctx, dispatch.delta); err != nil {
			return err
		}
	}
	for _, dataflow := range dataflows {
		if err := dataflow.advanceDataflowTime(ctx, at); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) sortedStatementsLocked() []*Statement {
	statements := make([]*Statement, 0, len(e.statements))
	for _, statement := range e.statements {
		statements = append(statements, statement)
	}
	sort.Slice(statements, func(i, j int) bool { return statements[i].name < statements[j].name })
	return statements
}

func (e *Engine) Close(ctx context.Context) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if e == nil {
		return nil
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	ids := make([]string, 0, len(e.deployments))
	for id := range e.deployments {
		ids = append(ids, id)
	}
	e.mu.Unlock()
	sort.Strings(ids)
	for _, id := range ids {
		if err := e.Undeploy(ctx, id); err != nil {
			return err
		}
	}
	e.mu.Lock()
	e.closed = true
	instances := make([]*DataflowInstance, 0, len(e.dataflows))
	for instance := range e.dataflows {
		instances = append(instances, instance)
	}
	e.dataflows = make(map[*DataflowInstance]struct{})
	e.savedDataflowInstances = make(map[string]*DataflowInstance)
	e.mu.Unlock()
	for _, instance := range instances {
		_ = instance.Cancel(context.Background())
	}
	return nil
}

func (e *Engine) dispatchDataflowEvent(ctx context.Context, event Event) error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	instances := make([]*DataflowInstance, 0, len(e.dataflows))
	for instance := range e.dataflows {
		instances = append(instances, instance)
	}
	e.mu.Unlock()
	for _, instance := range instances {
		instance.mu.Lock()
		active := instance.state == DataflowRunning && len(instance.eventTypes) > 0
		instance.mu.Unlock()
		if !active {
			continue
		}
		if err := instance.process(ctx, event); err != nil {
			return instance.completeDataflowFailure(err)
		}
	}
	return nil
}

type statementDispatch struct {
	statement *Statement
	batch     ResultBatch
}

type namedWindowDispatch struct {
	window *NamedWindow
	delta  NamedWindowDelta
}

// queueNamedWindowDeltaLocked applies a state mutation to all statements that
// consume named windows while the engine's event transaction is still under
// its lock. Listener delivery is deferred until the caller releases the lock,
// matching the ordering used by InsertNamedWindow and AdvanceTime.
func (e *Engine) queueNamedWindowDeltaLocked(ctx context.Context, now time.Time, window *NamedWindow, delta NamedWindowDelta, variables map[string]Value, owner *Statement) error {
	if e == nil || window == nil || delta.empty() {
		return nil
	}
	statements := e.sortedStatementsLocked()
	for _, statement := range statements {
		if statement == owner || !containsNamedWindow(statement.plan.query.input, statement.plan.query.join) {
			continue
		}
		batch, changed, err := statement.processNamedWindow(ctx, now, delta, variables)
		if err != nil {
			return err
		}
		if err := e.applyStatementOutputAssignmentsLocked(ctx, statement, &variables); err != nil {
			return err
		}
		if changed {
			e.pendingStatementDispatches = append(e.pendingStatementDispatches, statementDispatch{statement: statement, batch: batch})
			if err := e.queueStatementRoutesLocked(statement, batch, now); err != nil {
				return err
			}
		}
	}
	e.pendingNamedWindowDispatches = append(e.pendingNamedWindowDispatches, namedWindowDispatch{window: window, delta: delta})
	return nil
}

func (e *Engine) queueStatementRoutesLocked(statement *Statement, batch ResultBatch, now time.Time) error {
	if e == nil || statement == nil || statement.plan.query.routeTarget == "" {
		return nil
	}
	results := routeResults(statement.plan.query.selector, batch)
	for _, result := range results {
		event, err := e.routeResultLocked(statement, result, now)
		if err != nil {
			return err
		}
		e.pendingRoutedEvents = append(e.pendingRoutedEvents, event)
	}
	return nil
}

func (e *Engine) processPendingRoutedEventsLocked(ctx context.Context, now time.Time, variables map[string]Value, dispatches *[]statementDispatch) error {
	if e == nil || dispatches == nil {
		return nil
	}
	routedQueue := append([]Event(nil), e.pendingRoutedEvents...)
	e.pendingRoutedEvents = nil
	processed := 0
	for len(routedQueue) > 0 {
		if err := contextErr(ctx); err != nil {
			return err
		}
		if processed >= maxRoutedEventsPerSend {
			return NewError(ErrorState, fmt.Sprintf("route event limit %d exceeded", maxRoutedEventsPerSend))
		}
		processed++
		current := routedQueue[0]
		routedQueue = routedQueue[1:]
		for _, statement := range e.sortedStatementsLocked() {
			batch, changed, err := statement.process(ctx, now, current, variables)
			if err != nil {
				return err
			}
			if err := e.applyStatementOutputAssignmentsLocked(ctx, statement, &variables); err != nil {
				return err
			}
			if !changed {
				continue
			}
			*dispatches = append(*dispatches, statementDispatch{statement: statement, batch: batch})
			if err := e.queueStatementRoutesLocked(statement, batch, now); err != nil {
				return err
			}
		}
		*dispatches = append(*dispatches, e.pendingStatementDispatches...)
		e.pendingStatementDispatches = nil
		routedQueue = append(routedQueue, e.pendingRoutedEvents...)
		e.pendingRoutedEvents = nil
	}
	return nil
}

func (e *Engine) applyStatementOutputAssignmentsLocked(ctx context.Context, statement *Statement, variables *map[string]Value) error {
	if e == nil || statement == nil {
		return nil
	}
	assignments := statement.takeOutputAssignments()
	if len(assignments) == 0 {
		return nil
	}
	if err := e.setVariablesLocked(ctx, assignments); err != nil {
		return err
	}
	if variables != nil {
		*variables = cloneValues(e.variables)
	}
	return nil
}

func (e *Engine) routeResultLocked(statement *Statement, result Result, now time.Time) (Event, error) {
	if e == nil || e.env == nil || statement == nil {
		return Event{}, NewError(ErrorDependency, "route requires an engine, environment and statement")
	}
	targetName := statement.plan.query.routeTarget
	target, ok := e.env.Schema(targetName)
	if !ok {
		return Event{}, NewError(ErrorUnknownName, fmt.Sprintf("route target %q is not registered", targetName))
	}
	if source, ok := result.Event(); ok {
		if target.kind == SchemaVariant {
			if !e.env.variantAcceptsEventType(target, source.TypeName()) {
				return Event{}, NewError(ErrorTypeMismatch, fmt.Sprintf("event type %q is not a valid member of variant schema %q", source.TypeName(), target.Name()))
			}
			routed, err := newEvent(target, source, now)
			if err != nil {
				return Event{}, WrapError(ErrorTypeMismatch, "route."+targetName, err)
			}
			return routed, nil
		}
		underlying, err := projectEventUnderlying(target, source)
		if err != nil {
			return Event{}, err
		}
		routed, err := newEvent(target, underlying, now)
		if err != nil {
			return Event{}, WrapError(ErrorTypeMismatch, "route."+targetName, err)
		}
		return routed, nil
	}
	row, ok := result.Row()
	if !ok {
		return Event{}, NewError(ErrorTypeMismatch, fmt.Sprintf("route target %q received an empty result", targetName))
	}
	if target.kind == SchemaVariant && target.variantMode == VariantPredefined {
		return Event{}, NewError(ErrorTypeMismatch, fmt.Sprintf("projected row cannot route to predefined variant %q without member identity", targetName))
	}
	if target.kind == SchemaVariant && target.variantMode == VariantAny {
		derivedName := "route:" + targetName
		if len(statement.plan.hash) >= 12 {
			derivedName += ":" + statement.plan.hash[:12]
		}
		derived, err := NewMapSchema(derivedName, row.Schema().Fields(), AllowDynamicFields())
		if err != nil {
			return Event{}, err
		}
		routed, err := newEvent(derived, row.AsMap(), now)
		if err != nil {
			return Event{}, err
		}
		routed.streamType = targetName
		return routed, nil
	}
	underlying, err := projectRowUnderlying(target, row)
	if err != nil {
		return Event{}, err
	}
	routed, err := newEvent(target, underlying, now)
	if err != nil {
		return Event{}, WrapError(ErrorTypeMismatch, "route."+targetName, err)
	}
	return routed, nil
}

func projectEventUnderlying(target Schema, source Event) (any, error) {
	if target.goType != nil {
		underlying := source.Underlying()
		if underlying != nil {
			got := reflect.TypeOf(underlying)
			if got.AssignableTo(target.goType) || (got.Kind() == reflect.Pointer && got.Elem().AssignableTo(target.goType)) {
				return underlying, nil
			}
		}
	}
	values := make(map[string]any, len(target.fields))
	for _, field := range target.fields {
		value := source.Get(field.Name)
		if value.IsPresent() || value.IsNull() {
			values[field.Name] = value.Any()
		}
	}
	return projectMapToSchema(target, values)
}

func projectRowUnderlying(target Schema, row Row) (any, error) {
	values := row.AsMap()
	return projectMapToSchema(target, values)
}

func projectMapToSchema(target Schema, values map[string]any) (any, error) {
	if target.kind == SchemaObjectArray {
		ordered := make([]any, len(target.fields))
		for index, field := range target.fields {
			if value, ok := values[field.Name]; ok {
				ordered[index] = value
			}
		}
		return normalizeObjectArray(target, ordered)
	}
	if target.goType != nil {
		return mergeSchemaUnderlying(target, nil, values)
	}
	return values, nil
}

func dispatchAll(ctx context.Context, dispatches []statementDispatch) error {
	for _, dispatch := range dispatches {
		if err := dispatch.statement.dispatch(ctx, dispatch.batch); err != nil {
			return err
		}
	}
	return nil
}

func (s *Statement) dispatch(ctx context.Context, batch ResultBatch) error {
	s.mu.RLock()
	listenerIDs := make([]uint64, 0, len(s.listeners))
	for id := range s.listeners {
		listenerIDs = append(listenerIDs, id)
	}
	sort.Slice(listenerIDs, func(i, j int) bool { return listenerIDs[i] < listenerIDs[j] })
	listeners := make([]Listener, 0, len(listenerIDs))
	for _, id := range listenerIDs {
		listeners = append(listeners, s.listeners[id])
	}
	sink := s.plan.query.sink
	s.mu.RUnlock()
	for _, listener := range listeners {
		if err := listener(ctx, batch.clone()); err != nil {
			return fmt.Errorf("esper: listener for %q: %w", s.name, err)
		}
	}
	if sink != nil {
		if err := sink.Write(ctx, batch.clone()); err != nil {
			return fmt.Errorf("esper: sink for %q: %w", s.name, err)
		}
	}
	return nil
}

type storedEvent struct {
	event      Event
	receivedAt time.Time
	expiresAt  time.Time
}

var joinTupleSchema = func() Schema {
	schema, _ := NewMapSchema("esper:join-tuple", nil)
	return schema
}()

func newJoinTupleEvent(events []Event, receivedAt time.Time) Event {
	return Event{
		typeName:   "esper:join-tuple",
		schema:     joinTupleSchema,
		underlying: joinTuple{events: append([]Event(nil), events...)},
		receivedAt: receivedAt,
	}
}

func joinDeltaEvents(delta joinDelta, receivedAt time.Time) eventDelta {
	newTuples := append([][]Event(nil), delta.newTuples...)
	oldTuples := append([][]Event(nil), delta.oldTuples...)
	if len(newTuples) == 0 {
		for _, pair := range delta.newPairs {
			newTuples = append(newTuples, []Event{pair.left, pair.right})
		}
	}
	if len(oldTuples) == 0 {
		for _, pair := range delta.oldPairs {
			oldTuples = append(oldTuples, []Event{pair.left, pair.right})
		}
	}
	result := eventDelta{}
	for _, tuple := range newTuples {
		result.newEvents = append(result.newEvents, newJoinTupleEvent(tuple, receivedAt))
	}
	for _, tuple := range oldTuples {
		result.oldEvents = append(result.oldEvents, newJoinTupleEvent(tuple, receivedAt))
	}
	return result
}

type windowRuntimeState struct {
	entries    []storedEvent
	pendingNew []storedEvent
	arrival    []Event
	started    bool
	start      time.Time
	externalAt time.Time
	keyed      map[string]storedEvent
	keyOrder   []string
	groups     map[string]*windowRuntimeState
	children   []*windowRuntimeState
}

type statementRuntime struct {
	query                    Query
	engine                   *Engine
	rowRecogOwner            string
	ctx                      context.Context
	windows                  map[*streamNode]*windowRuntimeState
	joinState                *joinRuntimeState
	aggregateState           *aggregateRuntimeState
	patternState             *patternRuntimeState
	contextStartPatternState *patternRuntimeState
	contextEndPatternState   *patternRuntimeState
	contextPatternTags       map[string]Event
	contextPatternTagValues  map[string][]Event
	rowRecogState            *rowRecogRuntimeState
	outputState              *outputRuntimeState
	distinctCounts           map[string]int
	partitions               map[string]*statementRuntime
	partitionContextName     string
	partitionKey             string
	partitionID              int
	nextPartitionID          int
	contextProperties        map[string]Value
	variables                map[string]Value
	subqueryRegistry         *subqueryRuntimeRegistry
	seq                      *atomic.Uint64
	pendingOutputAssignments []VariableAssignment
}

type eventPair struct {
	left  Event
	right Event
}

type joinDelta struct {
	newPairs  []eventPair
	oldPairs  []eventPair
	newTuples [][]Event
	oldTuples [][]Event
}

type joinRuntimeState struct {
	sides [][]storedEvent
}

type aggregateRuntimeState struct {
	groups        map[string]*aggregateGroup
	allEvents     []Event
	allEverEvents []Event
}

type aggregateGroup struct {
	events         []Event
	everEvents     []Event
	leavingEvents  []Event
	pluginStates   map[*exprNode]aggregatePluginState
	representative Event
	current        Event
	groupingSet    []int
	leaving        bool
	previous       []Value
	emitted        bool
}

type aggregateResultEntry struct {
	result Result
	group  *aggregateGroup
}

type patternRuntimeState struct {
	active         []patternMatch
	patternStopped bool
	emittedEvents  []Event
	distinct       map[string]struct{}
	distinctAt     map[string]time.Time
	timerStarted   bool
	timerNext      time.Time
	timerEmitted   bool
	scheduleIndex  int
	cronSchedule   resolvedCronSchedule
	cronNext       time.Time
}

func patternDistinctExpiry(definition *patternDefinition) time.Duration {
	if definition == nil {
		return 0
	}
	if definition.everyDistinctExpirySet {
		return definition.everyDistinctExpiry
	}
	return definition.within
}

func expirePatternDistinct(state *patternRuntimeState, definition *patternDefinition, now time.Time) {
	if state == nil || len(state.distinctAt) == 0 {
		return
	}
	expiry := patternDistinctExpiry(definition)
	if expiry <= 0 {
		return
	}
	for key, startedAt := range state.distinctAt {
		if !startedAt.Add(expiry).After(now) {
			delete(state.distinctAt, key)
			delete(state.distinct, key)
		}
	}
}

func recordPatternDistinct(state *patternRuntimeState, definition *patternDefinition, key string, now time.Time) {
	if state == nil {
		return
	}
	if state.distinct == nil {
		state.distinct = make(map[string]struct{})
	}
	state.distinct[key] = struct{}{}
	if patternDistinctExpiry(definition) > 0 {
		if state.distinctAt == nil {
			state.distinctAt = make(map[string]time.Time)
		}
		state.distinctAt[key] = now
	}
}

func patternMatchEvents(match patternMatch) []Event {
	if len(match.tagValues) > 0 {
		events := make([]Event, 0)
		for _, values := range match.tagValues {
			events = append(events, values...)
		}
		return events
	}
	events := make([]Event, 0, len(match.tags))
	for _, event := range match.tags {
		events = append(events, event)
	}
	return events
}

func patternEventsOverlap(left, right []Event) bool {
	for _, leftEvent := range left {
		for _, rightEvent := range right {
			if sameEvent(leftEvent, rightEvent) {
				return true
			}
		}
	}
	return false
}

// acceptPatternMatch applies the query-level overlap policy and records the
// captured events only when the result is visible. A suppressed completion
// still terminates its ordinary branch, while a reusable Every branch remains
// governed by the normal continuation logic.
func (r *patternRuntimeState) acceptPatternMatch(query Query, match patternMatch) bool {
	if r == nil || !query.suppressOverlappingMatches {
		return true
	}
	events := patternMatchEvents(match)
	if patternEventsOverlap(events, r.emittedEvents) {
		return false
	}
	r.emittedEvents = append(r.emittedEvents, events...)
	return true
}

type rowRecogRuntimeState struct {
	partitions map[string]*rowRecogPartitionState
}

type rowRecogPartitionState struct {
	events             []Event
	previousByEvent    map[string][]Event
	previousRolling    []Event
	skipStart          int
	emitted            map[string]struct{}
	emittedMatches     map[string]rowRecogMatch
	intervalClosed     map[string]struct{}
	closedBranches     map[string]struct{}
	alternateNotified  map[string]struct{}
	intervalNotified   map[string]struct{}
	intervalFinal      map[string][]rowRecogMatch
	activeStarts       map[string]struct{}
	activeStateCounts  map[string]int64
	activePaths        map[string][]rowRecogNFAPath
	allowedMatchStarts map[string]struct{}
	allowedMatchKeys   map[string]struct{}
	blockedStarts      map[string]struct{}
	fastABStarC        []rowRecogFastABStarCPath
}

// patternProgress is the runtime state of one pattern expression tree. It is
// deliberately separate from the public builder AST: the same pattern tree
// can have many independent in-flight matches when Every is enabled.
type patternProgress struct {
	node                    *patternNode
	phase                   uint8
	count                   int
	done                    bool
	blocked                 bool
	started                 bool
	expired                 bool
	minimum                 int
	maximum                 int
	boundsResolved          bool
	sequenceMaximum         int
	sequenceMaximumResolved bool
	timerStarted            bool
	timerNext               time.Time
	timerEmitted            bool
	scheduleIndex           int
	cronSchedule            resolvedCronSchedule
	cronNext                time.Time
	left                    *patternProgress
	right                   *patternProgress
	child                   *patternProgress
	tags                    map[string]Event
	tagValues               map[string][]Event
	distinct                map[string]struct{}
	distinctAt              map[string]time.Time
}

type patternTransition struct {
	state            *patternProgress
	complete         bool
	consumed         bool
	consumptionLevel int
}

// patternTrigger distinguishes an incoming event from a virtual-clock
// callback. Timer observers are part of the pattern tree, therefore each
// active match needs its own timer state instead of sharing the statement
// level timer used by a timer-root pattern.
type patternTrigger struct {
	event            Event
	now              time.Time
	isTimer          bool
	consumptionLevel int
}

type patternMatch struct {
	state     *patternProgress
	tags      map[string]Event
	tagValues map[string][]Event
	current   Event
	startedAt time.Time
}

type outputRuntimeState struct {
	firstEmitted int
	pending      *ResultBatch
	pendingCount int
	whenPending  *ResultBatch
	afterSeen    int
	afterActive  bool
	afterStarted time.Time
	nextOutputAt time.Time
	cronNext     time.Time
	cronSchedule resolvedCronSchedule
	cronPending  *ResultBatch
	insertCount  int64
	removeCount  int64
	insertTotal  int64
	removeTotal  int64
	lastOutputAt time.Time
}

func newStatementRuntime(query Query) statementRuntime {
	runtime := statementRuntime{query: query, ctx: context.Background(), windows: make(map[*streamNode]*windowRuntimeState), partitions: make(map[string]*statementRuntime), variables: make(map[string]Value), seq: &atomic.Uint64{}}
	if query.join != nil {
		runtime.joinState = &joinRuntimeState{}
	}
	if query.aggregate != nil {
		runtime.aggregateState = &aggregateRuntimeState{groups: make(map[string]*aggregateGroup)}
	}
	if query.pattern != nil {
		runtime.patternState = &patternRuntimeState{distinct: make(map[string]struct{})}
	}
	if query.rowRecog != nil {
		runtime.rowRecogState = &rowRecogRuntimeState{partitions: make(map[string]*rowRecogPartitionState)}
	}
	if query.output.Kind != OutputAllPolicy || query.output.After != OutputAfterNone || query.output.Cron != nil || query.output.When != nil || len(query.output.Then) > 0 || query.output.Termination != OutputNoTermination || query.output.TerminationWhen != nil || len(query.output.TerminationThen) > 0 {
		runtime.outputState = &outputRuntimeState{}
	}
	if query.distinct {
		runtime.distinctCounts = make(map[string]int)
	}
	return runtime
}

func (r *statementRuntime) drainOutputAssignments() []VariableAssignment {
	if r == nil {
		return nil
	}
	assignments := append([]VariableAssignment(nil), r.pendingOutputAssignments...)
	r.pendingOutputAssignments = nil
	keys := make([]string, 0, len(r.partitions))
	for key := range r.partitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		assignments = append(assignments, r.partitions[key].drainOutputAssignments()...)
	}
	return assignments
}

func (r *statementRuntime) initializeAt(at time.Time) {
	if r == nil {
		return
	}
	if r.outputState != nil {
		r.outputState.afterStarted = at
		r.outputState.lastOutputAt = at
		switch r.query.output.After {
		case OutputAfterEventCount:
			r.outputState.afterActive = r.query.output.AfterCount == 0
		case OutputAfterDuration:
			r.outputState.afterActive = r.query.output.AfterDuration == 0
		case OutputAfterCalendarKind:
			period := r.query.output.AfterCalendar
			r.outputState.afterActive = period.Years == 0 && period.Months == 0 && period.Days == 0
		default:
			r.outputState.afterActive = true
		}
		if r.outputState.afterActive {
			r.scheduleOutput(r.query.output, at)
			r.scheduleCron(r.query.output, at)
		}
	}
	if r.query.pattern == nil || r.patternState == nil || r.query.pattern.root == nil {
		return
	}
	if patternContainsTimer(r.query.pattern.root) && !isPatternTimerRoot(r.query.pattern) && patternCanStartWithoutEvent(r.query.pattern.root) {
		progress := newPatternProgress(r.query.pattern.root)
		armPatternProgressTimers(progress, at, r.variables)
		if patternProgressActive(progress) {
			r.patternState.active = []patternMatch{{state: progress, startedAt: at}}
		}
	}
	switch root := r.query.pattern.root; root.kind {
	case patternTimerIntervalNode:
		r.patternState.timerStarted = true
		deadline, ok := patternDurationDeadline(root, nil, at, r.variables)
		if !ok {
			r.patternState.patternStopped = true
			return
		}
		r.patternState.timerNext = deadline
	case patternTimerAtNode:
		r.patternState.timerStarted = true
		r.patternState.timerNext = root.at
	case patternTimerScheduleNode:
		r.patternState.timerStarted = true
		r.patternState.scheduleIndex = 0
	case patternTimerCronNode:
		r.patternState.timerStarted = true
		if root.cron != nil {
			if resolved, err := root.cron.resolve(EvalContext{Now: at, Variables: r.variables}); err == nil {
				r.patternState.cronSchedule = resolved
				r.patternState.cronNext, _ = resolved.nextAfter(at)
			}
		}
	}
}

type eventDelta struct {
	newEvents       []Event
	oldEvents       []Event
	history         []Event
	historyByEvent  map[string][]Event
	previousByEvent map[string][]Event
	priorByEvent    map[string][]Event
}

func (s *Statement) process(ctx context.Context, now time.Time, event Event, variables map[string]Value) (ResultBatch, bool, error) {
	if err := contextErr(ctx); err != nil {
		return ResultBatch{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.collectOutputAssignmentsLocked()
	if s.closed || s.state != StatementStarted {
		return ResultBatch{}, false, nil
	}
	variables = variablesWithEngineLockState(statementVariables(variables, s.parameters), s.engine, true)
	if s.plan.query.contextName == "" && s.runtime.subqueryRegistry != nil {
		if err := s.runtime.subqueryRegistry.accept(event, now, variables); err != nil {
			return ResultBatch{}, false, err
		}
		variables = s.runtime.subqueryRegistry.attachVariables(variables)
	} else if s.plan.query.contextName != "" {
		if err := s.acceptContextSubqueryEventLocked(event, now, variables); err != nil {
			return ResultBatch{}, false, err
		}
	}
	s.runtime.ctx = ctx
	if s.plan.query.contextName != "" {
		definition, definitionOK := s.engine.env.Context(s.plan.query.contextName)
		if definitionOK && definition.kind == ContextInitiatedTerminated {
			if definition.startPattern != nil {
				return s.processPatternInitiatedTerminated(definition, event, now, variables)
			}
			return s.processInitiatedTerminated(definition, event, now, variables)
		}
		if !statementAcceptsEvent(s.plan.query, event) {
			return ResultBatch{}, false, nil
		}
		if definitionOK && definition.isTemporal() {
			_, _ = s.syncTemporalContextLocked(now)
		}
		partition, err := s.partitionRuntime(event, now, variables)
		if err != nil {
			return ResultBatch{}, false, err
		}
		if partition == nil {
			return ResultBatch{}, false, nil
		}
		if s.plan.query.trigger != nil {
			batch, err := s.processTriggerRuntime(ctx, partition, now, event, s.contextPartitionVariables(partition, variables))
			if err != nil {
				return ResultBatch{}, false, err
			}
			return batch, !batch.empty(), nil
		}
		batch, changed, err := partition.process(s.plan, event, now, s.contextPartitionVariables(partition, variables))
		if changed {
			batch.Sequence = s.runtime.seq.Add(1)
		}
		return batch, changed, err
	}
	if s.plan.query.trigger != nil {
		batch, err := s.processTriggerRuntime(ctx, &s.runtime, now, event, variables)
		if err != nil {
			return ResultBatch{}, false, err
		}
		return batch, !batch.empty(), nil
	}
	return s.runtime.process(s.plan, event, now, variables)
}

// acceptContextSubqueryEventLocked advances every currently active context
// partition's raw event-stream subqueries. Context subqueries are partition
// local in Esper, and they observe inner-stream events even when the event is
// not an outer-stream event for the statement itself.
func (s *Statement) acceptContextSubqueryEventLocked(event Event, now time.Time, variables map[string]Value) error {
	if s == nil || s.engine == nil || len(s.runtime.partitions) == 0 {
		return nil
	}
	for _, partition := range s.runtime.partitions {
		if partition == nil {
			continue
		}
		partition.ensureSubqueryRegistry(s.engine)
		if partition.subqueryRegistry == nil {
			continue
		}
		partitionVariables := s.contextPartitionVariables(partition, variables)
		if err := partition.subqueryRegistry.accept(event, now, partitionVariables); err != nil {
			return err
		}
	}
	return nil
}

func (s *Statement) contextPartitionVariables(partition *statementRuntime, variables map[string]Value) map[string]Value {
	result := cloneValues(variables)
	result = variablesWithEngine(result, s.engine)
	if partition != nil {
		result = partition.withContextVariables(result)
		result = partition.withContextProperties(result)
		partition.ensureSubqueryRegistry(s.engine)
		if partition.subqueryRegistry != nil {
			result = partition.subqueryRegistry.attachVariables(result)
		}
	}
	return result
}

func statementAcceptsEvent(query Query, event Event) bool {
	if query.join != nil {
		for _, source := range joinDefinitionSources(query.join) {
			if sourceNodeAcceptsEvent(query.env, source, event) {
				return true
			}
		}
		return false
	}
	input := query.input
	if query.pattern != nil {
		input = query.pattern.input
	}
	if query.aggregate != nil {
		input = query.aggregate.input
	}
	if query.rowRecog != nil {
		input = query.rowRecog.input
	}
	return sourceNodeAcceptsEvent(query.env, input, event)
}

func sourceNodeAcceptsEvent(env *Environment, node *streamNode, event Event) bool {
	source, err := sourceNode(node)
	if err != nil || source == nil {
		return false
	}
	if source.kind == streamHistorical {
		return true
	}
	if env != nil && source.kind == streamSource {
		if schema, err := env.sourceSchema(source); err == nil && schema.kind == SchemaVariant {
			return event.StreamType() == source.sourceName
		}
	}
	if event.StreamType() != event.TypeName() {
		return source.sourceName == event.StreamType()
	}
	if source.sourceName == event.TypeName() {
		return true
	}
	if env == nil {
		return false
	}
	schema, err := env.sourceSchema(source)
	if err != nil {
		return false
	}
	if schema.kind == SchemaVariant {
		return env.variantAcceptsEventType(schema, event.TypeName())
	}
	if source.kind != streamSource {
		return false
	}
	return env.acceptsEventType(schema.Name(), event.TypeName())
}

// processPatternInitiatedTerminated is the event-driven Context lifecycle
// bridge for PatternStream. It reuses the immutable pattern AST and
// transition engine used by pattern queries, while keeping one end-pattern
// state per context partition so end predicates can refer to start tags.
func (s *Statement) processPatternInitiatedTerminated(definition ContextDefinition, event Event, now time.Time, variables map[string]Value) (ResultBatch, bool, error) {
	if s == nil || s.engine == nil {
		return ResultBatch{}, false, NewError(ErrorDependency, "pattern context has no engine")
	}
	if definition.startPattern == nil {
		return ResultBatch{}, false, NewError(ErrorInvalidRule, "pattern context has no start pattern")
	}
	if s.runtime.partitions == nil {
		s.runtime.partitions = make(map[string]*statementRuntime)
	}

	startMatches := advanceContextPattern(&s.runtime.contextStartPatternState, definition.startPattern, event, now, variables, nil, nil, s.engine.env)
	for _, match := range startMatches {
		if !definition.initiatedOverlapping && len(s.runtime.partitions) > 0 {
			continue
		}
		allocationKey := "initiated:pattern"
		if definition.initiatedOverlapping {
			allocationKey = s.engine.allocateOverlappingContextPartitionKeyLocked(s.plan.query.contextName, allocationKey)
		}
		if _, exists := s.runtime.partitions[allocationKey]; exists {
			continue
		}
		query := s.plan.query
		query.contextName = ""
		partitionRuntime := newStatementRuntime(query)
		partitionRuntime.engine = s.engine
		partitionRuntime.rowRecogOwner = s.runtime.rowRecogOwner
		partitionRuntime.partitionContextName = s.plan.query.contextName
		partitionRuntime.partitionKey = allocationKey
		partitionRuntime.partitionID = s.allocateContextPartitionID(allocationKey)
		partitionRuntime.contextPatternTags = clonePatternTags(match.tags)
		partitionRuntime.contextPatternTagValues = clonePatternTagValues(match.tagValues)
		partitionRuntime.contextProperties = definition.contextPropertyValues(event, now, variables, partitionRuntime.partitionID)
		partitionRuntime.contextProperties["initiating_event"] = Present(event)
		applyContextPatternProperties(&partitionRuntime, match.tags)
		partitionRuntime.contextEndPatternState = &patternRuntimeState{distinct: make(map[string]struct{})}
		partitionRuntime.variables = partitionRuntime.withContextProperties(variables)
		initializeContextPatternTimer(&partitionRuntime.contextEndPatternState, definition.endPattern, now, partitionRuntime.variables)
		partitionRuntime.initializeAt(now)
		partition := ptrStatementRuntime(partitionRuntime)
		s.runtime.partitions[allocationKey] = partition
		s.engine.retainContextPartitionLocked(s.plan.query.contextName, allocationKey, partition)
	}

	keys := make([]string, 0, len(s.runtime.partitions))
	for key := range s.runtime.partitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ResultBatch{}, false, nil
	}

	terminating := make(map[string]bool, len(keys))
	if definition.endPattern != nil {
		for _, partitionKey := range keys {
			partition := s.runtime.partitions[partitionKey]
			if partition == nil {
				continue
			}
			partitionVariables := partition.withContextVariables(variablesWithEngine(variables, s.engine))
			partitionVariables = partition.withContextProperties(partitionVariables)
			matches := advanceContextPattern(
				&partition.contextEndPatternState,
				definition.endPattern,
				event,
				now,
				partitionVariables,
				partition.contextPatternTags,
				partition.contextPatternTagValues,
				s.engine.env,
			)
			if len(matches) == 0 {
				continue
			}
			match := matches[0]
			partition.contextPatternTags = clonePatternTags(match.tags)
			partition.contextPatternTagValues = clonePatternTagValues(match.tagValues)
			applyContextPatternProperties(partition, match.tags)
			if partition.contextProperties == nil {
				partition.contextProperties = make(map[string]Value)
			}
			partition.contextProperties["terminating_event"] = Present(event)
			terminating[partitionKey] = true
		}
	}

	accepts := statementAcceptsEvent(s.plan.query, event)
	var result ResultBatch
	var changed bool
	if accepts {
		for _, partitionKey := range keys {
			if terminating[partitionKey] && s.plan.query.output.Termination != OutputNoTermination {
				// A terminating event is available through ContextTerminatingEvent
				// but does not enter a termination snapshot.
				continue
			}
			partition := s.runtime.partitions[partitionKey]
			if partition == nil {
				continue
			}
			var batch ResultBatch
			var partitionChanged bool
			var err error
			if s.plan.query.trigger != nil {
				batch, err = s.processTriggerRuntime(s.runtime.ctx, partition, now, event, s.contextPartitionVariables(partition, variables))
				partitionChanged = !batch.empty()
			} else {
				batch, partitionChanged, err = partition.process(s.plan, event, now, s.contextPartitionVariables(partition, variables))
			}
			if err != nil {
				return ResultBatch{}, false, err
			}
			result.New = append(result.New, batch.New...)
			result.Old = append(result.Old, batch.Old...)
			changed = changed || partitionChanged
		}
	}
	for _, partitionKey := range keys {
		if !terminating[partitionKey] {
			continue
		}
		partition := s.runtime.partitions[partitionKey]
		if partition == nil {
			continue
		}
		partition.variables = s.contextPartitionVariables(partition, variables)
		if s.plan.query.output.Termination != OutputNoTermination {
			terminationBatch := partition.outputAtTermination(s.plan, now)
			result.New = append(result.New, terminationBatch.New...)
			result.Old = append(result.Old, terminationBatch.Old...)
			changed = changed || !terminationBatch.empty()
		}
		s.runtime.pendingOutputAssignments = append(s.runtime.pendingOutputAssignments, partition.drainOutputAssignments()...)
		delete(s.runtime.partitions, partitionKey)
		s.engine.releaseContextPartitionLocked(s.plan.query.contextName, partitionKey, partition)
	}
	if changed {
		result.Sequence = s.runtime.seq.Add(1)
	}
	return result, changed, nil
}

func applyContextPatternProperties(runtime *statementRuntime, tags map[string]Event) {
	if runtime == nil || len(tags) == 0 {
		return
	}
	if runtime.contextProperties == nil {
		runtime.contextProperties = make(map[string]Value, len(tags))
	}
	for tag, event := range tags {
		runtime.contextProperties["pattern."+tag] = Present(event)
	}
}

func advanceContextPattern(state **patternRuntimeState, definition *patternDefinition, event Event, now time.Time, variables map[string]Value, seedTags map[string]Event, seedTagValues map[string][]Event, env *Environment) []patternMatch {
	if state == nil || definition == nil || definition.root == nil {
		return nil
	}
	if definition.input != nil && !sourceNodeAcceptsEvent(env, definition.input, event) {
		return nil
	}
	if !patternGuardAllows(definition, event, now, variables) {
		if *state != nil {
			(*state).active = nil
		}
		return nil
	}
	if *state == nil {
		*state = &patternRuntimeState{distinct: make(map[string]struct{})}
	}
	runtimeState := *state
	if runtimeState.patternStopped {
		return nil
	}
	expirePatternDistinct(runtimeState, definition, now)
	if definition.within > 0 {
		kept := runtimeState.active[:0]
		for _, match := range runtimeState.active {
			if match.startedAt.Add(definition.within).After(now) {
				kept = append(kept, match)
			}
		}
		runtimeState.active = kept
	}
	matches := runtimeState.active
	startAllowed := definition.every || len(matches) == 0
	var startProgress *patternProgress
	if startAllowed {
		startProgress = newPatternProgress(definition.root)
		startProgress.tags = clonePatternTags(seedTags)
		startProgress.tagValues = clonePatternTagValues(seedTagValues)
		armPatternProgressTimers(startProgress, now, variables)
	}
	consumptionLevel := -1
	if patternHasConsumption(definition.root) {
		probe := patternTrigger{event: event, now: now, consumptionLevel: -1}
		for _, match := range matches {
			for _, transition := range advancePatternNodeTrigger(match.state, probe, variables) {
				if transition.consumed && transition.consumptionLevel > consumptionLevel {
					consumptionLevel = transition.consumptionLevel
				}
			}
		}
		if startProgress != nil {
			for _, transition := range advancePatternNodeTrigger(startProgress, probe, variables) {
				if transition.consumed && transition.consumptionLevel > consumptionLevel {
					consumptionLevel = transition.consumptionLevel
				}
			}
		}
	}
	trigger := patternTrigger{event: event, now: now, consumptionLevel: consumptionLevel}
	completed := make([]patternMatch, 0)
	nextActive := make([]patternMatch, 0, len(matches)+1)
	terminal := false
	completedAny := false
	for _, match := range matches {
		transitions := advancePatternNodeTrigger(match.state, trigger, variables)
		for _, transition := range transitions {
			if transition.state == nil {
				continue
			}
			candidate := patternMatch{
				state:     transition.state,
				tags:      clonePatternTags(transition.state.tags),
				tagValues: clonePatternTagValues(transition.state.tagValues),
				current:   event,
				startedAt: match.startedAt,
			}
			if transition.complete {
				completedAny = true
				completed = append(completed, candidate)
				if patternCanContinueAfterMatch(transition.state) && patternMatchWithinLimits(nextActive, candidate, definition) {
					nextActive = append(nextActive, candidate)
				}
				if patternProgressTerminal(transition.state) {
					terminal = true
				}
				continue
			}
			if patternProgressTerminal(transition.state) {
				terminal = true
			}
			if patternProgressActive(transition.state) && patternMatchWithinLimits(nextActive, candidate, definition) {
				nextActive = append(nextActive, candidate)
			}
		}
	}
	if startProgress != nil {
		starts := advancePatternNodeTrigger(startProgress, trigger, variables)
		for _, transition := range starts {
			if transition.state == nil || (!transition.complete && !patternProgressActive(transition.state)) {
				continue
			}
			if definition.everyDistinct != nil {
				keyValue := definition.everyDistinct.eval(EvalContext{Event: event, Tags: transition.state.tags, TagValues: transition.state.tagValues, Now: now, Variables: variables})
				key := encodeKey([]any{keyValue.State(), keyValue.Any()})
				if _, exists := runtimeState.distinct[key]; exists {
					continue
				}
				recordPatternDistinct(runtimeState, definition, key, now)
			}
			started := patternMatch{
				state:     transition.state,
				tags:      clonePatternTags(transition.state.tags),
				tagValues: clonePatternTagValues(transition.state.tagValues),
				current:   event,
				startedAt: now,
			}
			if transition.complete {
				completedAny = true
				completed = append(completed, started)
				if patternCanContinueAfterMatch(transition.state) && patternMatchWithinLimits(nextActive, started, definition) {
					nextActive = append(nextActive, started)
				}
				if patternProgressTerminal(transition.state) {
					terminal = true
				}
			} else if patternMatchWithinLimits(nextActive, started, definition) {
				nextActive = append(nextActive, started)
				if patternProgressTerminal(transition.state) {
					terminal = true
				}
			}
		}
	}
	runtimeState.active = nextActive
	if terminal || (definition.root.kind == patternWithinNode && len(nextActive) == 0 && completedAny) {
		runtimeState.patternStopped = true
	}
	return completed
}

func initializeContextPatternTimer(state **patternRuntimeState, definition *patternDefinition, at time.Time, variables map[string]Value) bool {
	if state == nil || definition == nil || definition.root == nil || (!patternContainsTimer(definition.root) && definition.within <= 0 && patternDistinctExpiry(definition) <= 0) {
		return false
	}
	if *state == nil {
		*state = &patternRuntimeState{distinct: make(map[string]struct{})}
	}
	runtimeState := *state
	if runtimeState.patternStopped {
		return true
	}
	if !isPatternTimerRoot(definition) {
		if !patternContainsTimer(definition.root) {
			expirePatternDistinct(runtimeState, definition, at)
			if definition.within > 0 {
				kept := runtimeState.active[:0]
				for _, match := range runtimeState.active {
					if match.startedAt.Add(definition.within).After(at) {
						kept = append(kept, match)
					}
				}
				runtimeState.active = kept
			}
			return true
		}
		if len(runtimeState.active) == 0 && !runtimeState.patternStopped && patternCanStartWithoutEvent(definition.root) {
			progress := newPatternProgress(definition.root)
			armPatternProgressTimers(progress, at, variables)
			if patternProgressActive(progress) {
				runtimeState.active = []patternMatch{{state: progress, startedAt: at}}
			}
		}
		return true
	}
	if runtimeState.timerStarted {
		return true
	}
	runtimeState.timerStarted = true
	root := definition.root
	switch root.kind {
	case patternTimerIntervalNode:
		deadline, ok := patternDurationDeadline(root, nil, at, variables)
		if !ok {
			runtimeState.patternStopped = true
			return false
		}
		runtimeState.timerNext = deadline
	case patternTimerAtNode:
		runtimeState.timerNext = root.at
	case patternTimerScheduleNode:
		runtimeState.scheduleIndex = 0
	case patternTimerCronNode:
		if root.cron == nil {
			return false
		}
		resolved, err := root.cron.resolve(EvalContext{Now: at, Variables: variables})
		if err != nil {
			return false
		}
		runtimeState.cronSchedule = resolved
		runtimeState.cronNext, _ = resolved.nextAfter(at)
	}
	return true
}

func advanceContextPatternCompositeTime(state **patternRuntimeState, definition *patternDefinition, now time.Time, variables map[string]Value, seedTags map[string]Event, seedTagValues map[string][]Event) []patternMatch {
	runtimeState := *state
	if runtimeState.patternStopped {
		return nil
	}
	expirePatternDistinct(runtimeState, definition, now)
	completed := make([]patternMatch, 0)
	nextActive := make([]patternMatch, 0, len(runtimeState.active))
	terminal := false
	for _, match := range runtimeState.active {
		if definition.within > 0 && !match.startedAt.Add(definition.within).After(now) {
			continue
		}
		transitions := advancePatternNodeTime(match.state, now, variables)
		for _, transition := range transitions {
			if transition.state == nil {
				continue
			}
			candidate := patternMatch{
				state:     transition.state,
				tags:      mergePatternTags(seedTags, transition.state.tags),
				tagValues: mergePatternTagValues(seedTagValues, transition.state.tagValues),
				current:   Event{},
				startedAt: match.startedAt,
			}
			if transition.complete {
				completed = append(completed, candidate)
				if patternCanContinueAfterMatch(transition.state) && patternMatchWithinLimits(nextActive, candidate, definition) {
					nextActive = append(nextActive, candidate)
				}
				if patternProgressTerminal(transition.state) {
					terminal = true
				}
				if definition.every {
					progress := newPatternProgress(definition.root)
					progress.tags = clonePatternTags(seedTags)
					progress.tagValues = clonePatternTagValues(seedTagValues)
					armPatternProgressTimers(progress, now, variables)
					candidate := patternMatch{state: progress, startedAt: now}
					if patternProgressActive(progress) && patternMatchWithinLimits(nextActive, candidate, definition) {
						nextActive = append(nextActive, candidate)
					}
				}
				continue
			}
			if patternProgressTerminal(transition.state) {
				terminal = true
			}
			if patternProgressActive(transition.state) && patternMatchWithinLimits(nextActive, candidate, definition) {
				nextActive = append(nextActive, candidate)
			}
		}
	}
	runtimeState.active = nextActive
	if terminal {
		runtimeState.patternStopped = true
	}
	return completed
}

func advanceContextPatternTime(state **patternRuntimeState, definition *patternDefinition, now time.Time, variables map[string]Value, seedTags map[string]Event, seedTagValues map[string][]Event) []patternMatch {
	if state == nil || definition == nil || definition.root == nil || (!patternContainsTimer(definition.root) && definition.within <= 0 && patternDistinctExpiry(definition) <= 0) {
		return nil
	}
	if !initializeContextPatternTimer(state, definition, now, variables) {
		return nil
	}
	if (*state).patternStopped {
		return nil
	}
	if !patternGuardAllows(definition, Event{}, now, variables) {
		return nil
	}
	if !patternContainsTimer(definition.root) {
		runtimeState := *state
		expirePatternDistinct(runtimeState, definition, now)
		if definition.within > 0 {
			kept := runtimeState.active[:0]
			for _, match := range runtimeState.active {
				if match.startedAt.Add(definition.within).After(now) {
					kept = append(kept, match)
				}
			}
			runtimeState.active = kept
		}
		return nil
	}
	if !isPatternTimerRoot(definition) {
		return advanceContextPatternCompositeTime(state, definition, now, variables, seedTags, seedTagValues)
	}
	runtimeState := *state
	root := definition.root
	completed := make([]patternMatch, 0)
	const maxTimerCatchUp = 100000
	switch root.kind {
	case patternTimerIntervalNode:
		for emitted := 0; emitted < maxTimerCatchUp && !runtimeState.timerNext.IsZero() && !runtimeState.timerNext.After(now); emitted++ {
			dueAt := runtimeState.timerNext
			completed = append(completed, patternMatch{
				current:   Event{},
				startedAt: dueAt,
				tags:      clonePatternTags(seedTags),
				tagValues: clonePatternTagValues(seedTagValues),
			})
			next, ok := patternDurationDeadline(root, nil, dueAt, variables)
			if !ok {
				runtimeState.patternStopped = true
				runtimeState.timerNext = time.Time{}
				break
			}
			runtimeState.timerNext = next
		}
	case patternTimerAtNode:
		if !runtimeState.timerEmitted && !now.Before(runtimeState.timerNext) {
			completed = append(completed, patternMatch{
				current:   Event{},
				startedAt: runtimeState.timerNext,
				tags:      clonePatternTags(seedTags),
				tagValues: clonePatternTagValues(seedTagValues),
			})
			runtimeState.timerEmitted = true
		}
	case patternTimerScheduleNode:
		for emitted := 0; emitted < maxTimerCatchUp && runtimeState.scheduleIndex < len(root.schedule) && !root.schedule[runtimeState.scheduleIndex].After(now); emitted++ {
			dueAt := root.schedule[runtimeState.scheduleIndex]
			completed = append(completed, patternMatch{
				current:   Event{},
				startedAt: dueAt,
				tags:      clonePatternTags(seedTags),
				tagValues: clonePatternTagValues(seedTagValues),
			})
			runtimeState.scheduleIndex++
		}
	case patternTimerCronNode:
		for emitted := 0; emitted < maxTimerCatchUp && !runtimeState.cronNext.IsZero() && !runtimeState.cronNext.After(now); emitted++ {
			dueAt := runtimeState.cronNext
			completed = append(completed, patternMatch{
				current:   Event{},
				startedAt: dueAt,
				tags:      clonePatternTags(seedTags),
				tagValues: clonePatternTagValues(seedTagValues),
			})
			if root.cronOneShot {
				runtimeState.cronNext = time.Time{}
				runtimeState.timerEmitted = true
				break
			}
			runtimeState.cronNext, _ = runtimeState.cronSchedule.nextAfter(dueAt)
		}
	}
	return completed
}

func (s *Statement) processPatternContextTime(definition ContextDefinition, now time.Time, variables map[string]Value) (ResultBatch, bool) {
	if s == nil || s.engine == nil || definition.startPattern == nil {
		return ResultBatch{}, false
	}
	if s.runtime.partitions == nil {
		s.runtime.partitions = make(map[string]*statementRuntime)
	}
	for _, match := range advanceContextPatternTime(&s.runtime.contextStartPatternState, definition.startPattern, now, variables, nil, nil) {
		if !definition.initiatedOverlapping && len(s.runtime.partitions) > 0 {
			continue
		}
		allocationKey := "initiated:pattern"
		if definition.initiatedOverlapping {
			allocationKey = s.engine.allocateOverlappingContextPartitionKeyLocked(s.plan.query.contextName, allocationKey)
		}
		if _, exists := s.runtime.partitions[allocationKey]; exists {
			continue
		}
		query := s.plan.query
		query.contextName = ""
		partitionRuntime := newStatementRuntime(query)
		partitionRuntime.engine = s.engine
		partitionRuntime.rowRecogOwner = s.runtime.rowRecogOwner
		partitionRuntime.partitionContextName = s.plan.query.contextName
		partitionRuntime.partitionKey = allocationKey
		partitionRuntime.partitionID = s.allocateContextPartitionID(allocationKey)
		partitionRuntime.contextPatternTags = clonePatternTags(match.tags)
		partitionRuntime.contextPatternTagValues = clonePatternTagValues(match.tagValues)
		partitionRuntime.contextProperties = definition.contextPropertyValues(Event{}, now, variables, partitionRuntime.partitionID)
		applyContextPatternProperties(&partitionRuntime, match.tags)
		partitionRuntime.contextEndPatternState = &patternRuntimeState{distinct: make(map[string]struct{})}
		partitionRuntime.variables = partitionRuntime.withContextProperties(variables)
		initializeContextPatternTimer(&partitionRuntime.contextEndPatternState, definition.endPattern, match.startedAt, partitionRuntime.variables)
		partitionRuntime.initializeAt(now)
		partition := ptrStatementRuntime(partitionRuntime)
		s.runtime.partitions[allocationKey] = partition
		s.engine.retainContextPartitionLocked(s.plan.query.contextName, allocationKey, partition)
	}

	keys := make([]string, 0, len(s.runtime.partitions))
	for key := range s.runtime.partitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	terminating := make(map[string]bool, len(keys))
	for _, partitionKey := range keys {
		partition := s.runtime.partitions[partitionKey]
		if partition == nil || definition.endPattern == nil {
			continue
		}
		partitionVariables := partition.withContextVariables(variablesWithEngine(variables, s.engine))
		partitionVariables = partition.withContextProperties(partitionVariables)
		matches := advanceContextPatternTime(&partition.contextEndPatternState, definition.endPattern, now, partitionVariables, partition.contextPatternTags, partition.contextPatternTagValues)
		if len(matches) == 0 {
			continue
		}
		match := matches[0]
		partition.contextPatternTags = clonePatternTags(match.tags)
		partition.contextPatternTagValues = clonePatternTagValues(match.tagValues)
		applyContextPatternProperties(partition, match.tags)
		terminating[partitionKey] = true
	}

	result := ResultBatch{Time: now}
	changed := false
	for _, partitionKey := range keys {
		if !terminating[partitionKey] {
			continue
		}
		partition := s.runtime.partitions[partitionKey]
		if partition == nil {
			continue
		}
		partition.variables = s.contextPartitionVariables(partition, variables)
		if s.plan.query.output.Termination != OutputNoTermination {
			terminationBatch := partition.outputAtTermination(s.plan, now)
			result.New = append(result.New, terminationBatch.New...)
			result.Old = append(result.Old, terminationBatch.Old...)
			changed = changed || !terminationBatch.empty()
		}
		s.runtime.pendingOutputAssignments = append(s.runtime.pendingOutputAssignments, partition.drainOutputAssignments()...)
		delete(s.runtime.partitions, partitionKey)
		s.engine.releaseContextPartitionLocked(s.plan.query.contextName, partitionKey, partition)
	}
	if changed {
		result.Sequence = s.runtime.seq.Add(1)
	}
	return result, changed
}

func (s *Statement) processInitiatedTerminated(definition ContextDefinition, event Event, now time.Time, variables map[string]Value) (ResultBatch, bool, error) {
	if s == nil || s.engine == nil {
		return ResultBatch{}, false, NewError(ErrorDependency, "initiated-terminated context has no engine")
	}
	if s.runtime.partitions == nil {
		s.runtime.partitions = make(map[string]*statementRuntime)
	}
	key, _, err := definition.partition(event, now, variables)
	if err != nil {
		return ResultBatch{}, false, err
	}
	startValue := definition.start.eval(EvalContext{Event: event, Now: now, Variables: variables})
	start, startOK := boolValue(startValue)
	keyAvailable := initiatedContextKeyAvailable(definition, event, now, variables)
	if startOK && start && (definition.initiatedOverlapping || s.runtime.partitions[key] == nil) {
		allocationKey := key
		if definition.initiatedOverlapping {
			allocationKey = s.engine.allocateOverlappingContextPartitionKeyLocked(s.plan.query.contextName, key)
		}
		query := s.plan.query
		query.contextName = ""
		partitionRuntime := newStatementRuntime(query)
		partitionRuntime.engine = s.engine
		partitionRuntime.rowRecogOwner = s.runtime.rowRecogOwner
		partitionRuntime.partitionContextName = s.plan.query.contextName
		partitionRuntime.partitionKey = allocationKey
		partitionRuntime.partitionID = s.allocateContextPartitionID(allocationKey)
		partitionRuntime.contextProperties = definition.contextPropertyValues(event, now, variables, partitionRuntime.partitionID)
		partitionRuntime.contextProperties["initiating_event"] = Present(event)
		partitionRuntime.variables = partitionRuntime.withContextProperties(variables)
		partitionRuntime.initializeAt(now)
		partitionValue := ptrStatementRuntime(partitionRuntime)
		s.runtime.partitions[allocationKey] = partitionValue
		s.engine.retainContextPartitionLocked(s.plan.query.contextName, allocationKey, partitionValue)
	}

	keys := make([]string, 0, len(s.runtime.partitions))
	for partitionKey := range s.runtime.partitions {
		keys = append(keys, partitionKey)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ResultBatch{}, false, nil
	}

	// Distinct and overlapping initiation contexts evaluate termination
	// independently for every active context instance. For the existing keyed
	// initiated form, a lifecycle event carrying the key terminates only that
	// partition; an event from another schema has no key and falls back to all
	// instances.
	terminationKeys := keys
	broadcast := definition.initiatedDistinct || definition.initiatedOverlapping
	if !broadcast && keyAvailable {
		terminationKeys = nil
		if _, exists := s.runtime.partitions[key]; exists {
			terminationKeys = []string{key}
		}
	}
	terminating := make(map[string]bool, len(terminationKeys))
	for _, partitionKey := range terminationKeys {
		partition := s.runtime.partitions[partitionKey]
		if partition == nil {
			continue
		}
		if definition.end == nil {
			continue
		}
		partitionVariables := partition.withContextVariables(variablesWithEngine(variables, s.engine))
		partitionVariables = partition.withContextProperties(partitionVariables)
		endValue := definition.end.eval(EvalContext{Event: event, Now: now, Variables: partitionVariables})
		if end, ok := boolValue(endValue); ok && end {
			if partition.contextProperties == nil {
				partition.contextProperties = make(map[string]Value)
			}
			partition.contextProperties["terminating_event"] = Present(event)
			terminating[partitionKey] = true
		}
	}

	accepts := statementAcceptsEvent(s.plan.query, event)
	processKeys := keys
	if !broadcast && keyAvailable {
		processKeys = nil
		if _, exists := s.runtime.partitions[key]; exists {
			processKeys = []string{key}
		}
	}
	var result ResultBatch
	var changed bool
	if accepts {
		for _, partitionKey := range processKeys {
			if terminating[partitionKey] && s.plan.query.output.Termination != OutputNoTermination {
				// An end event closes the context before it enters the
				// statement's data window. A termination snapshot therefore
				// observes the state before this event, matching Esper's
				// same-event termination behavior.
				continue
			}
			partition := s.runtime.partitions[partitionKey]
			if partition == nil {
				continue
			}
			var batch ResultBatch
			var partitionChanged bool
			if s.plan.query.trigger != nil {
				batch, err = s.processTriggerRuntime(s.runtime.ctx, partition, now, event, s.contextPartitionVariables(partition, variables))
				partitionChanged = !batch.empty()
			} else {
				batch, partitionChanged, err = partition.process(s.plan, event, now, s.contextPartitionVariables(partition, variables))
			}
			if err != nil {
				return ResultBatch{}, false, err
			}
			result.New = append(result.New, batch.New...)
			result.Old = append(result.Old, batch.Old...)
			changed = changed || partitionChanged
		}
	}
	for _, partitionKey := range keys {
		if !terminating[partitionKey] {
			continue
		}
		partition := s.runtime.partitions[partitionKey]
		if partition == nil {
			continue
		}
		partition.variables = s.contextPartitionVariables(partition, variables)
		if definition.end != nil && s.plan.query.output.Termination != OutputNoTermination {
			terminationBatch := partition.outputAtTermination(s.plan, now)
			result.New = append(result.New, terminationBatch.New...)
			result.Old = append(result.Old, terminationBatch.Old...)
			changed = changed || !terminationBatch.empty()
		}
		// The parent statement collects assignments from live partitions after
		// processing. Drain this partition before releasing it, including
		// regular output-when assignments when no termination output is set.
		s.runtime.pendingOutputAssignments = append(s.runtime.pendingOutputAssignments, partition.drainOutputAssignments()...)
		delete(s.runtime.partitions, partitionKey)
		s.engine.releaseContextPartitionLocked(s.plan.query.contextName, partitionKey, partition)
	}
	if changed {
		result.Sequence = s.runtime.seq.Add(1)
	}
	return result, changed, nil
}

func initiatedContextKeyAvailable(definition ContextDefinition, event Event, now time.Time, variables map[string]Value) bool {
	for _, value := range definition.evaluatedKeyResults(event, now, variables) {
		if value.IsMissing() {
			return false
		}
	}
	return true
}

// syncTemporalContextLocked materializes the single active partition of a
// recurring temporal context and tears down a prior window when the virtual
// clock crosses its boundary. The caller holds engine.mu and statement.mu.
func (s *Statement) syncTemporalContextLocked(now time.Time) (ResultBatch, bool) {
	if s == nil || s.engine == nil || s.plan.query.contextName == "" {
		return ResultBatch{}, false
	}
	definition, ok := s.engine.env.Context(s.plan.query.contextName)
	if !ok || !definition.isTemporal() {
		return ResultBatch{}, false
	}
	origin, ok := s.engine.contextTemporalOrigins[s.plan.query.contextName]
	if !ok || origin.IsZero() {
		origin = now
		s.engine.contextTemporalOrigins[s.plan.query.contextName] = origin
	}
	start, end, active := definition.temporalWindow(origin, now)
	activeKey := ""
	if active {
		activeKey = temporalPartitionKey(start)
	}
	batch := ResultBatch{Time: now}
	for key, partition := range s.runtime.partitions {
		if active && key == activeKey {
			continue
		}
		if partition != nil {
			partition.variables = s.contextPartitionVariables(partition, s.runtime.variables)
			if s.plan.query.output.Termination != OutputNoTermination {
				terminationBatch := partition.outputAtTermination(s.plan, now)
				batch.New = append(batch.New, terminationBatch.New...)
				batch.Old = append(batch.Old, terminationBatch.Old...)
			} else {
				partBatch, _ := partition.expireBatch(s.plan, now, s.runtime.variables)
				batch.New = append(batch.New, partBatch.New...)
				batch.Old = append(batch.Old, partBatch.Old...)
			}
			s.runtime.pendingOutputAssignments = append(s.runtime.pendingOutputAssignments, partition.drainOutputAssignments()...)
		}
		delete(s.runtime.partitions, key)
		s.engine.releaseContextPartitionLocked(s.plan.query.contextName, key, partition)
	}
	if active {
		if _, exists := s.runtime.partitions[activeKey]; !exists {
			query := s.plan.query
			query.contextName = ""
			partitionRuntime := newStatementRuntime(query)
			partitionRuntime.engine = s.engine
			partitionRuntime.rowRecogOwner = s.runtime.rowRecogOwner
			partitionRuntime.partitionContextName = s.plan.query.contextName
			partitionRuntime.partitionKey = activeKey
			partitionRuntime.partitionID = s.allocateContextPartitionID(activeKey)
			partitionRuntime.contextProperties = definition.contextPropertyValues(Event{}, now, s.runtime.variables, partitionRuntime.partitionID)
			partitionRuntime.contextProperties["startTime"] = Present(start)
			partitionRuntime.contextProperties["endTime"] = Present(end)
			partitionRuntime.variables = partitionRuntime.withContextProperties(s.runtime.variables)
			partitionRuntime.initializeAt(now)
			partition := ptrStatementRuntime(partitionRuntime)
			s.runtime.partitions[activeKey] = partition
			s.engine.retainContextPartitionLocked(s.plan.query.contextName, activeKey, partition)
		}
	}
	if batch.empty() {
		return ResultBatch{}, false
	}
	batch.Sequence = s.runtime.seq.Add(1)
	return batch, true
}

func (s *Statement) partitionRuntime(event Event, now time.Time, variables map[string]Value) (*statementRuntime, error) {
	definition, ok := s.engine.env.Context(s.plan.query.contextName)
	if !ok {
		return nil, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", s.plan.query.contextName))
	}
	if definition.isTemporal() {
		_, _ = s.syncTemporalContextLocked(now)
		origin := s.engine.contextTemporalOrigins[s.plan.query.contextName]
		start, _, active := definition.temporalWindow(origin, now)
		if !active {
			return nil, nil
		}
		key := temporalPartitionKey(start)
		if partition := s.runtime.partitions[key]; partition != nil {
			return partition, nil
		}
		return nil, nil
	}
	key, active, err := definition.partition(event, now, variables)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, nil
	}
	if s.runtime.partitions == nil {
		s.runtime.partitions = make(map[string]*statementRuntime)
	}
	partition := s.runtime.partitions[key]
	if partition == nil {
		query := s.plan.query
		query.contextName = ""
		partitionRuntime := newStatementRuntime(query)
		partitionRuntime.engine = s.engine
		partitionRuntime.rowRecogOwner = s.runtime.rowRecogOwner
		partitionRuntime.partitionContextName = s.plan.query.contextName
		partitionRuntime.partitionKey = key
		partitionRuntime.partitionID = s.allocateContextPartitionID(key)
		partitionRuntime.contextProperties = definition.contextPropertyValues(event, now, variables, partitionRuntime.partitionID)
		partitionRuntime.variables = partitionRuntime.withContextProperties(variables)
		partitionRuntime.initializeAt(now)
		partition = ptrStatementRuntime(partitionRuntime)
		s.runtime.partitions[key] = partition
		s.engine.retainContextPartitionLocked(s.plan.query.contextName, key, partition)
	}
	return partition, nil
}

func (r *statementRuntime) ensureSubqueryRegistry(engine *Engine) {
	if r == nil || engine == nil || r.subqueryRegistry != nil {
		return
	}
	r.engine = engine
	r.subqueryRegistry = newSubqueryRuntimeRegistry(engine.env, engine, r.query)
}

func ptrStatementRuntime(runtime statementRuntime) *statementRuntime {
	runtime.ensureSubqueryRegistry(runtime.engine)
	return &runtime
}

func (r *statementRuntime) withContextProperties(variables map[string]Value) map[string]Value {
	if r == nil || (len(r.contextProperties) == 0 && (r.partitionContextName == "" || r.partitionKey == "")) {
		return variables
	}
	result := cloneValues(variables)
	if result == nil {
		result = make(map[string]Value, len(r.contextProperties)+2)
	}
	for name, value := range r.contextProperties {
		result[contextVariableName(name)] = value
	}
	if r.partitionContextName != "" && r.partitionKey != "" {
		result[subqueryContextNameVariable] = Present(r.partitionContextName)
		result[subqueryContextPartitionVariable] = Present(r.partitionKey)
	}
	return result
}

func (r *statementRuntime) withContextVariables(variables map[string]Value) map[string]Value {
	if r == nil || r.engine == nil || r.partitionContextName == "" || r.partitionKey == "" {
		return variables
	}
	state := r.engine.ensureContextVariablePartitionLocked(r.partitionContextName, r.partitionKey)
	if len(state) == 0 {
		return variables
	}
	result := cloneValues(variables)
	if result == nil {
		result = make(map[string]Value, len(state))
	}
	for name, value := range state {
		result[name] = value
	}
	return result
}

func (s *Statement) allocateContextPartitionID(key string) int {
	if s == nil {
		return 0
	}
	if s.engine != nil {
		return s.engine.allocateContextPartitionIDLocked(s.plan.query.contextName, key)
	}
	id := s.runtime.nextPartitionID
	s.runtime.nextPartitionID++
	return id
}

func (r *statementRuntime) process(plan Plan, event Event, now time.Time, variables map[string]Value) (ResultBatch, bool, error) {
	r.variables = variablesWithEngine(variables, r.engine)
	r.variables = r.withContextVariables(r.variables)
	r.variables = r.withContextProperties(r.variables)
	variables = r.variables
	var batch ResultBatch
	var err error
	if plan.query.aggregate != nil {
		if plan.query.aggregate.join != nil {
			joinDelta, joinErr := r.insertJoin(plan.query.aggregate.join, event, now)
			err = joinErr
			if err == nil {
				batch, err = r.aggregateBatch(joinDeltaEvents(joinDelta, now), plan, now)
			}
		} else {
			delta, insertErr := r.insert(plan.query.aggregate.input, event, now)
			err = insertErr
			if err == nil {
				batch, err = r.aggregateBatch(delta, plan, now)
			}
		}
	} else if plan.query.join != nil {
		joinDelta, joinErr := r.insertJoin(plan.query.join, event, now)
		err = joinErr
		if err == nil {
			batch = r.joinBatch(joinDelta, plan, now)
		}
	} else if plan.query.rowRecog != nil {
		delta, insertErr := r.insert(plan.query.rowRecog.input, event, now)
		err = insertErr
		if err == nil {
			batch = r.rowRecogBatch(delta, plan, now)
		}
	} else if plan.query.pattern != nil {
		delta, insertErr := r.insert(plan.query.pattern.input, event, now)
		err = insertErr
		if err == nil {
			batch = r.patternBatch(delta, plan, now)
		}
	} else {
		delta, insertErr := r.insert(plan.query.input, event, now)
		err = insertErr
		if err == nil {
			batch = r.batch(delta, plan, now)
		}
	}
	if err != nil {
		return ResultBatch{}, false, err
	}
	batch = r.applyOutput(plan.query.output, batch, false, now, plan)
	return batch, !batch.empty(), nil
}

func (r *statementRuntime) context() context.Context {
	if r == nil || r.ctx == nil {
		return context.Background()
	}
	return r.ctx
}

func (s *Statement) expire(now time.Time, variables map[string]Value) (ResultBatch, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.collectOutputAssignmentsLocked()
	if s.closed || s.state != StatementStarted {
		return ResultBatch{}, false
	}
	variables = variablesWithEngineLockState(statementVariables(variables, s.parameters), s.engine, true)
	if s.plan.query.contextName == "" && s.runtime.subqueryRegistry != nil {
		s.runtime.subqueryRegistry.expire(now)
		variables = s.runtime.subqueryRegistry.attachVariables(variables)
	} else if s.plan.query.contextName != "" {
		s.expireContextSubqueriesLocked(now)
	}
	if s.plan.query.contextName != "" {
		if definition, ok := s.engine.env.Context(s.plan.query.contextName); ok && definition.kind == ContextInitiatedTerminated && definition.startPattern != nil {
			patternBatch, patternChanged := s.processPatternContextTime(definition, now, variables)
			batch, changed := s.expireContext(now, variables)
			if patternChanged {
				patternBatch.New = append(patternBatch.New, batch.New...)
				patternBatch.Old = append(patternBatch.Old, batch.Old...)
				if !batch.empty() {
					patternBatch.Time = batch.Time
				}
				return patternBatch, true
			}
			return batch, changed
		}
		var temporalBatch ResultBatch
		var temporalChanged bool
		if definition, ok := s.engine.env.Context(s.plan.query.contextName); ok && definition.isTemporal() {
			temporalBatch, temporalChanged = s.syncTemporalContextLocked(now)
		}
		batch, changed := s.expireContext(now, variables)
		if temporalChanged {
			temporalBatch.New = append(temporalBatch.New, batch.New...)
			temporalBatch.Old = append(temporalBatch.Old, batch.Old...)
			if !batch.empty() {
				temporalBatch.Time = batch.Time
			}
			return temporalBatch, true
		}
		return batch, changed
	}
	batch, _ := s.runtime.expireBatch(s.plan, now, variables)
	return batch, !batch.empty()
}

func (s *Statement) expireContextSubqueriesLocked(now time.Time) {
	if s == nil {
		return
	}
	for _, partition := range s.runtime.partitions {
		if partition == nil {
			continue
		}
		partition.ensureSubqueryRegistry(s.engine)
		if partition.subqueryRegistry != nil {
			partition.subqueryRegistry.expire(now)
		}
	}
}

func (s *Statement) expireContext(now time.Time, variables map[string]Value) (ResultBatch, bool) {
	keys := make([]string, 0, len(s.runtime.partitions))
	for key := range s.runtime.partitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	batch := ResultBatch{Time: now}
	for _, key := range keys {
		partition := s.runtime.partitions[key]
		partBatch, _ := partition.expireBatch(s.plan, now, s.contextPartitionVariables(partition, variables))
		batch.New = append(batch.New, partBatch.New...)
		batch.Old = append(batch.Old, partBatch.Old...)
	}
	if batch.empty() {
		return ResultBatch{}, false
	}
	batch.Sequence = s.runtime.seq.Add(1)
	return batch, true
}

func (r *statementRuntime) expireBatch(plan Plan, now time.Time, variables map[string]Value) (ResultBatch, error) {
	r.variables = variablesWithEngine(variables, r.engine)
	r.variables = r.withContextVariables(r.variables)
	r.variables = r.withContextProperties(r.variables)
	variables = r.variables
	var batch ResultBatch
	if plan.query.aggregate != nil {
		if plan.query.aggregate.join != nil {
			delta := r.expireJoin(now)
			batch, _ = r.aggregateBatch(joinDeltaEvents(delta, now), plan, now)
		} else {
			delta := r.expire(now)
			batch, _ = r.aggregateBatch(delta, plan, now)
		}
	} else if plan.query.join != nil {
		delta := r.expireJoin(now)
		batch = r.joinBatch(delta, plan, now)
	} else if plan.query.rowRecog != nil {
		delta := r.expire(now)
		batch = r.rowRecogBatch(delta, plan, now)
	} else if plan.query.pattern != nil {
		if isPatternTimerRoot(plan.query.pattern) || patternContainsTimer(plan.query.pattern.root) {
			batch = r.patternTimeBatch(plan, now)
		} else {
			r.patternExpire(plan.query.pattern, now)
		}
	} else {
		delta := r.expire(now)
		batch = r.batch(delta, plan, now)
	}
	batch = r.applyOutput(plan.query.output, batch, true, now, plan)
	return batch, nil
}

func (r *statementRuntime) applyOutput(policy OutputPolicy, batch ResultBatch, flush bool, now time.Time, plans ...Plan) ResultBatch {
	if r == nil {
		return ResultBatch{}
	}
	if now.IsZero() {
		now = batch.Time
	}
	if r.outputState == nil {
		return batch
	}
	var active bool
	batch, active = r.activateOutput(policy, batch, now)
	if !active {
		return ResultBatch{}
	}
	r.recordOutputCounts(batch)
	if policy.Termination == OutputOnlyOnTermination {
		if !batch.empty() && policy.Kind != OutputSnapshotPolicy {
			if policy.When != nil || policy.TerminationWhen != nil {
				r.appendWhenPending(batch)
			} else {
				r.appendPending(batch)
			}
		}
		return ResultBatch{}
	}
	if policy.When != nil {
		if policy.Kind == OutputSnapshotPolicy && len(plans) > 0 {
			snapshot := r.snapshotBatch(plans[0], now)
			if !snapshot.empty() {
				r.outputState.whenPending = &snapshot
			} else {
				r.outputState.whenPending = nil
			}
		} else if !batch.empty() {
			r.appendWhenPending(batch)
		}
		if !r.outputWhenMatches(policy.When, now) {
			return ResultBatch{}
		}
		batch = r.takeWhenPending(batch)
	}
	if policy.When != nil && policy.Kind == OutputSnapshotPolicy {
		return r.finishOutput(policy, batch, now, plans...)
	}
	if policy.Cron != nil {
		return r.applyCronOutput(policy, batch, flush, now, plans...)
	}
	if policy.Kind == OutputAllPolicy {
		return r.finishOutput(policy, batch, now, plans...)
	}
	switch policy.Kind {
	case OutputFirstPolicy:
		remaining := policy.Count - r.outputState.firstEmitted
		if remaining <= 0 {
			return ResultBatch{}
		}
		result := batch.clone()
		if len(result.New) > remaining {
			result.New = result.New[:remaining]
		}
		remaining -= len(result.New)
		if remaining < len(result.Old) {
			result.Old = result.Old[:remaining]
		}
		r.outputState.firstEmitted += len(result.New) + len(result.Old)
		return r.finishOutput(policy, result, now, plans...)
	case OutputLastPolicy, OutputSnapshotPolicy:
		if !batch.empty() {
			copyBatch := batch.clone()
			r.outputState.pending = &copyBatch
		}
		if !flush || r.outputState.pending == nil {
			return ResultBatch{}
		}
		result := r.outputState.pending.clone()
		r.outputState.pending = nil
		return r.finishOutput(policy, result, now, plans...)
	case OutputEveryPolicy:
		if policy.Snapshot {
			if !batch.empty() {
				r.outputState.pendingCount += len(batch.New) + len(batch.Old)
			}
			if r.outputState.pendingCount < policy.Count {
				return ResultBatch{}
			}
			r.outputState.pendingCount = 0
			result := ResultBatch{}
			if len(plans) > 0 {
				result = r.snapshotBatch(plans[0], now)
			}
			if result.empty() {
				return ResultBatch{}
			}
			return r.finishOutput(policy, result, now, plans...)
		}
		if !batch.empty() {
			if r.outputState.pending == nil {
				copyBatch := batch.clone()
				r.outputState.pending = &copyBatch
			} else {
				r.outputState.pending.New = append(r.outputState.pending.New, batch.New...)
				r.outputState.pending.Old = append(r.outputState.pending.Old, batch.Old...)
				r.outputState.pending.Time = batch.Time
			}
			r.outputState.pendingCount += len(batch.New) + len(batch.Old)
		}
		if r.outputState.pending == nil || r.outputState.pendingCount < policy.Count {
			return ResultBatch{}
		}
		result := r.outputState.pending.clone()
		r.outputState.pending = nil
		r.outputState.pendingCount = 0
		return r.finishOutput(policy, result, now, plans...)
	case OutputEveryTimePolicy:
		if policy.Snapshot {
			if !flush || r.outputState.nextOutputAt.IsZero() || now.Before(r.outputState.nextOutputAt) {
				return ResultBatch{}
			}
			result := ResultBatch{}
			if len(plans) > 0 {
				result = r.snapshotBatch(plans[0], now)
			}
			r.outputState.pending = nil
			r.outputState.pendingCount = 0
			r.advanceOutputSchedule(policy, now)
			if result.empty() {
				return ResultBatch{}
			}
			result.Time = now
			return r.finishOutput(policy, result, now, plans...)
		}
		if !batch.empty() {
			r.appendPending(batch)
		}
		if !flush || r.outputState.pending == nil || r.outputState.nextOutputAt.IsZero() || now.Before(r.outputState.nextOutputAt) {
			return ResultBatch{}
		}
		result := r.outputState.pending.clone()
		result.Time = now
		r.outputState.pending = nil
		r.outputState.pendingCount = 0
		r.advanceOutputSchedule(policy, now)
		return r.finishOutput(policy, result, now, plans...)
	default:
		return r.finishOutput(policy, batch, now, plans...)
	}
}

// outputAtTermination produces the result associated with an initiated
// context partition ending. It runs before the terminating event is inserted
// into the statement state. Explicit snapshot policies and count/time-based
// policies rebuild the current state; when policies and other policies with
// pending deltas flush those deltas accumulated since the previous output.
func (r *statementRuntime) outputAtTermination(plan Plan, now time.Time) ResultBatch {
	if r == nil || r.outputState == nil {
		return ResultBatch{}
	}
	policy := plan.query.output
	condition := policy.TerminationWhen
	if condition == nil {
		condition = policy.When
	}
	if condition != nil && !r.outputWhenMatches(condition, now) {
		r.outputState.pending = nil
		r.outputState.whenPending = nil
		r.outputState.pendingCount = 0
		return ResultBatch{}
	}

	var result ResultBatch
	snapshot := policy.Kind == OutputSnapshotPolicy || policy.Kind == OutputEveryPolicy || policy.Kind == OutputEveryTimePolicy ||
		(policy.Kind == OutputAllPolicy && policy.When == nil)
	if snapshot {
		result = r.snapshotBatch(plan, now)
	} else if policy.When != nil || policy.TerminationWhen != nil {
		result = r.takeWhenPending(ResultBatch{})
	} else if r.outputState.pending != nil {
		result = r.outputState.pending.clone()
		r.outputState.pending = nil
	}
	r.outputState.pending = nil
	r.outputState.whenPending = nil
	r.outputState.pendingCount = 0
	if result.empty() {
		return ResultBatch{}
	}

	terminationPolicy := policy
	if policy.TerminationWhen != nil {
		terminationPolicy.Then = append([]OutputVariableAssignment(nil), policy.TerminationThen...)
	} else if len(policy.TerminationThen) > 0 {
		terminationPolicy.Then = append([]OutputVariableAssignment(nil), policy.TerminationThen...)
	}
	return r.finishOutput(terminationPolicy, result, now, plan)
}

func (r *statementRuntime) snapshotBatch(plan Plan, now time.Time) ResultBatch {
	if r == nil {
		return ResultBatch{}
	}
	if plan.query.aggregate != nil {
		return r.snapshotAggregateBatch(plan, now)
	}
	if plan.query.join != nil {
		return r.snapshotJoinBatch(plan, now)
	}
	if plan.query.rowRecog != nil {
		return r.snapshotRowRecog(plan, now, r.variables)
	}
	events := r.currentStreamEvents(plan.query.input, now)
	if len(events) == 0 {
		return ResultBatch{}
	}
	result := ResultBatch{Time: now}
	history := append([]Event(nil), events...)
	previousByEvent := r.currentPreviousAccess(plan.query.input)
	priorByEvent := r.currentPriorAccess(plan.query.input)
	result.New = projectResults(events, plan.query, plan.resultSchema, now, r.variables, history, nil, previousByEvent, priorByEvent, false)
	result.New = applyResultWindow(result.New, plan.query)
	if !result.empty() {
		result.Sequence = r.seq.Add(1)
	}
	return result
}

func (r *statementRuntime) snapshotAggregateBatch(plan Plan, now time.Time) ResultBatch {
	if r != nil && plan.query.aggregate != nil && plan.query.aggregate.join == nil && containsTableSource(plan.query.input, nil) {
		return r.snapshotAggregateFromTable(plan, now)
	}
	return r.snapshotAggregateStateBatch(plan, now)
}

func (r *statementRuntime) snapshotAggregateFromTable(plan Plan, now time.Time) ResultBatch {
	events := r.currentStreamEvents(plan.query.input, now)
	if len(events) == 0 {
		return ResultBatch{Time: now}
	}
	temporaryPlan := plan
	temporaryPlan.query.tableTarget = ""
	temporary := newStatementRuntime(temporaryPlan.query)
	temporary.engine = r.engine
	temporary.ctx = r.ctx
	temporary.variables = r.variables
	for _, event := range events {
		delta, err := temporary.insert(temporaryPlan.query.input, event, now)
		if err != nil {
			return ResultBatch{Time: now}
		}
		if _, err := temporary.aggregateBatch(delta, temporaryPlan, now); err != nil {
			return ResultBatch{Time: now}
		}
	}
	return temporary.snapshotAggregateStateBatch(temporaryPlan, now)
}

func (r *statementRuntime) snapshotAggregateStateBatch(plan Plan, now time.Time) ResultBatch {
	result := ResultBatch{Time: now}
	definition := plan.query.aggregate
	if r == nil || definition == nil || r.aggregateState == nil {
		return result
	}
	keys := make([]string, 0, len(r.aggregateState.groups))
	for key := range r.aggregateState.groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	entries := make([]aggregateResultEntry, 0, len(keys))
	for _, key := range keys {
		group := r.aggregateState.groups[key]
		if group == nil || (len(group.events) == 0 && !aggregateDefinitionUsesEver(definition)) {
			continue
		}
		values, visible := evaluateAggregateGroup(
			definition,
			group.events,
			group.everEvents,
			nil,
			false,
			group.groupingSet,
			group.current,
			r.aggregateState.allEvents,
			r.aggregateState.allEverEvents,
			now,
			r.variables,
			group.pluginStates,
		)
		if !visible {
			continue
		}
		entries = append(entries, aggregateResultEntry{result: resultRow(newRow(plan.resultSchema, values)), group: group})
	}
	if len(plan.query.orderBy) > 0 {
		orderAggregateResults(entries, plan.query.orderBy, definition, r.aggregateState.allEvents, r.aggregateState.allEverEvents, now, r.variables, false)
	}
	for _, entry := range entries {
		result.New = append(result.New, entry.result)
	}
	if plan.query.distinct {
		result.New = distinctSnapshotResults(result.New)
	}
	result.New = applyResultWindow(result.New, plan.query)
	return result
}

func (r *statementRuntime) snapshotJoinBatch(plan Plan, now time.Time) ResultBatch {
	result := ResultBatch{Time: now}
	if r == nil || plan.query.join == nil || r.joinState == nil {
		return result
	}
	state := cloneJoinRuntimeState(r.joinState)
	for index, source := range joinDefinitionSources(plan.query.join) {
		base, err := sourceNode(source)
		if err != nil || base == nil || (base.kind != streamNamedWindow && base.kind != streamTable) {
			continue
		}
		side, snapshotErr := r.snapshotCurrentSourceSide(source, base, now)
		if snapshotErr != nil {
			return result
		}
		if index >= len(state.sides) {
			state.sides = append(state.sides, make([][]storedEvent, index-len(state.sides)+1)...)
		}
		state.sides[index] = side
	}
	tuples := joinTuples(plan.query.join, state, now, r)
	result.New = projectJoinTuples(tuples, plan.query, plan.resultSchema, now, r.variables, false)
	if plan.query.distinct {
		result.New = distinctSnapshotResults(result.New)
	}
	result.New = applyResultWindow(result.New, plan.query)
	return result
}

func cloneJoinRuntimeState(state *joinRuntimeState) *joinRuntimeState {
	if state == nil {
		return &joinRuntimeState{}
	}
	copyState := &joinRuntimeState{sides: make([][]storedEvent, len(state.sides))}
	for index, side := range state.sides {
		copyState.sides[index] = append([]storedEvent(nil), side...)
	}
	return copyState
}

func distinctSnapshotResults(results []Result) []Result {
	if len(results) < 2 {
		return results
	}
	seen := make(map[string]struct{}, len(results))
	unique := make([]Result, 0, len(results))
	for _, result := range results {
		key := resultKey(result)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, result)
	}
	return unique
}

func (r *statementRuntime) currentStreamEvents(node *streamNode, now time.Time) []Event {
	if r == nil || node == nil {
		return nil
	}
	switch node.kind {
	case streamNamedWindow, streamTable:
		if r.engine == nil {
			return nil
		}
		events, err := r.engine.snapshotFireAndForgetSourceLocked(r.context(), node, now, r.variables)
		if err != nil {
			return nil
		}
		return r.filterPartitionEvents(events, now)
	case streamWindow:
		return windowHistory(node.window, r.windows[node])
	case streamFilter:
		events := r.currentStreamEvents(node.input, now)
		filtered := make([]Event, 0, len(events))
		for _, event := range events {
			value := node.predicate.eval(EvalContext{Event: event, History: events, Now: now, Variables: r.variables})
			if matched, ok := boolValue(value); ok && matched {
				filtered = append(filtered, event)
			}
		}
		return filtered
	default:
		return nil
	}
}

func (r *statementRuntime) currentPreviousAccess(node *streamNode) map[string][]Event {
	if r == nil || node == nil {
		return nil
	}
	switch node.kind {
	case streamWindow:
		if !windowUsesPreviousAccess(node.window) {
			return nil
		}
		return windowPreviousAccessByEvent(node.window, r.windows[node])
	case streamFilter:
		return r.currentPreviousAccess(node.input)
	default:
		return nil
	}
}

func (r *statementRuntime) currentPriorAccess(node *streamNode) map[string][]Event {
	if r == nil || node == nil {
		return nil
	}
	switch node.kind {
	case streamWindow:
		if !windowUsesPreviousAccess(node.window) {
			return nil
		}
		return windowPriorAccessByEvent(node.window, r.windows[node])
	case streamFilter:
		return r.currentPriorAccess(node.input)
	default:
		return nil
	}
}

func (r *statementRuntime) filterPartitionEvents(events []Event, now time.Time) []Event {
	if r == nil || r.partitionContextName == "" {
		return events
	}
	if r.engine == nil || r.engine.env == nil {
		return nil
	}
	definition, ok := r.engine.env.Context(r.partitionContextName)
	if !ok {
		return nil
	}
	filtered := make([]Event, 0, len(events))
	for _, event := range events {
		key, active, partitionErr := definition.partition(event, now, r.variables)
		if partitionErr == nil && active && key == r.partitionKey {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func (r *statementRuntime) applyCronOutput(policy OutputPolicy, batch ResultBatch, flush bool, now time.Time, plans ...Plan) ResultBatch {
	if r == nil || r.outputState == nil || policy.Cron == nil {
		return batch
	}
	if !r.ensureCronSchedule(policy, now) {
		return ResultBatch{}
	}
	state := r.outputState
	if !batch.empty() {
		switch policy.Kind {
		case OutputLastPolicy:
			copyBatch := batch.clone()
			state.pending = &copyBatch
		case OutputSnapshotPolicy:
			// A snapshot is reconstructed from the current source state at the
			// calendar tick below.  Do not reduce it to only this event batch.
		default:
			r.appendCronPending(batch)
		}
	}
	if !flush || state.cronNext.IsZero() || now.Before(state.cronNext) {
		return ResultBatch{}
	}

	// A clock jump can pass more than one calendar tick.  The first due tick
	// owns the accumulated rows; later empty ticks only advance the schedule.
	due := state.cronNext
	state.cronNext, _ = state.cronSchedule.nextAfter(now)
	var result ResultBatch
	switch policy.Kind {
	case OutputLastPolicy:
		if state.pending != nil {
			result = state.pending.clone()
			state.pending = nil
		}
	case OutputSnapshotPolicy:
		if len(plans) > 0 {
			result = r.snapshotBatch(plans[0], due)
		} else if state.pending != nil {
			// Keep a safe fallback for internal callers that do not carry a
			// Plan. Public statement paths always provide one.
			result = state.pending.clone()
			state.pending = nil
		}
	default:
		if state.cronPending != nil {
			result = state.cronPending.clone()
			state.cronPending = nil
		}
	}
	if result.empty() {
		return ResultBatch{}
	}
	result.Time = due
	if policy.Kind == OutputFirstPolicy && policy.Count > 0 {
		remaining := policy.Count - state.firstEmitted
		if remaining <= 0 {
			return ResultBatch{}
		}
		if len(result.New) > remaining {
			result.New = result.New[:remaining]
		}
		remaining -= len(result.New)
		if remaining < len(result.Old) {
			result.Old = result.Old[:remaining]
		}
		state.firstEmitted += len(result.New) + len(result.Old)
	}
	return r.finishOutput(policy, result, due, plans...)
}

func (r *statementRuntime) outputWhenMatches(condition Expr, now time.Time) bool {
	if condition == nil {
		return true
	}
	state := r.outputState
	value := condition.eval(EvalContext{
		Now:                  now,
		Variables:            r.variables,
		OutputInsertCount:    state.insertCount,
		OutputRemoveCount:    state.removeCount,
		OutputInsertTotal:    state.insertTotal,
		OutputRemoveTotal:    state.removeTotal,
		OutputLastOutputTime: state.lastOutputAt,
	})
	matched, ok := boolValue(value)
	return ok && matched
}

func (r *statementRuntime) finishOutput(policy OutputPolicy, batch ResultBatch, now time.Time, plans ...Plan) ResultBatch {
	if r == nil || batch.empty() {
		return batch
	}
	if len(plans) > 0 && len(plans[0].query.orderBy) > 0 {
		batch.New = orderRowRecogResults(batch.New, plans[0].query.orderBy, now, r.variables)
		batch.Old = orderRowRecogResults(batch.Old, plans[0].query.orderBy, now, r.variables)
	}
	r.outputState.insertCount = 0
	r.outputState.removeCount = 0
	r.outputState.lastOutputAt = now
	if len(policy.Then) == 0 {
		return batch
	}
	working := cloneValues(r.variables)
	assignments := make([]VariableAssignment, 0, len(policy.Then))
	for _, assignment := range policy.Then {
		value := assignment.Expr.eval(EvalContext{Now: now, Variables: working})
		assignments = append(assignments, VariableAssignment{Name: assignment.Name, Value: value.Any()})
		if value.IsPresent() {
			working[assignment.Name] = value
		} else if value.IsNull() {
			working[assignment.Name] = Null()
		} else {
			working[assignment.Name] = Missing()
		}
	}
	r.pendingOutputAssignments = append(r.pendingOutputAssignments, assignments...)
	return batch
}

func (r *statementRuntime) recordOutputCounts(batch ResultBatch) {
	if r == nil || r.outputState == nil {
		return
	}
	inserted := int64(len(batch.New))
	removed := int64(len(batch.Old))
	if batch.outputCountsSet {
		inserted = batch.outputInserted
		removed = batch.outputRemoved
	}
	r.outputState.insertCount += inserted
	r.outputState.removeCount += removed
	r.outputState.insertTotal += inserted
	r.outputState.removeTotal += removed
}

func (r *statementRuntime) appendWhenPending(batch ResultBatch) {
	if r == nil || r.outputState == nil || batch.empty() {
		return
	}
	if r.outputState.whenPending == nil {
		copyBatch := batch.clone()
		r.outputState.whenPending = &copyBatch
		return
	}
	r.outputState.whenPending.New = append(r.outputState.whenPending.New, batch.New...)
	r.outputState.whenPending.Old = append(r.outputState.whenPending.Old, batch.Old...)
	r.outputState.whenPending.Time = batch.Time
}

func (r *statementRuntime) takeWhenPending(current ResultBatch) ResultBatch {
	if r == nil || r.outputState == nil || r.outputState.whenPending == nil {
		return current
	}
	result := r.outputState.whenPending.clone()
	r.outputState.whenPending = nil
	return result
}

func (r *statementRuntime) activateOutput(policy OutputPolicy, batch ResultBatch, now time.Time) (ResultBatch, bool) {
	if r.outputState == nil {
		return batch, true
	}
	state := r.outputState
	if state.afterActive {
		return batch, true
	}
	switch policy.After {
	case OutputAfterEventCount:
		count := len(batch.New) + len(batch.Old)
		if state.afterSeen+count <= policy.AfterCount {
			state.afterSeen += count
			return ResultBatch{}, false
		}
		skip := policy.AfterCount - state.afterSeen
		state.afterSeen = policy.AfterCount
		state.afterActive = true
		r.scheduleOutput(policy, now)
		return dropResultPrefix(batch, skip), true
	case OutputAfterDuration:
		if now.Before(state.afterStarted.Add(policy.AfterDuration)) {
			return ResultBatch{}, false
		}
		state.afterActive = true
		r.scheduleOutput(policy, now)
		return batch, true
	case OutputAfterCalendarKind:
		period := policy.AfterCalendar
		activationAt := state.afterStarted.AddDate(period.Years, period.Months, period.Days)
		if now.Before(activationAt) {
			return ResultBatch{}, false
		}
		state.afterActive = true
		r.scheduleOutput(policy, now)
		return batch, true
	default:
		state.afterActive = true
		r.scheduleOutput(policy, now)
		return batch, true
	}
}

func dropResultPrefix(batch ResultBatch, count int) ResultBatch {
	if count <= 0 || batch.empty() {
		return batch
	}
	result := batch.clone()
	if count < len(result.New) {
		result.New = result.New[count:]
		return result
	}
	count -= len(result.New)
	result.New = nil
	if count < len(result.Old) {
		result.Old = result.Old[count:]
		return result
	}
	result.Old = nil
	return result
}

func (r *statementRuntime) appendPending(batch ResultBatch) {
	if r.outputState.pending == nil {
		copyBatch := batch.clone()
		r.outputState.pending = &copyBatch
	} else {
		r.outputState.pending.New = append(r.outputState.pending.New, batch.New...)
		r.outputState.pending.Old = append(r.outputState.pending.Old, batch.Old...)
		r.outputState.pending.Time = batch.Time
	}
	r.outputState.pendingCount += len(batch.New) + len(batch.Old)
}

func (r *statementRuntime) appendCronPending(batch ResultBatch) {
	if r == nil || r.outputState == nil || batch.empty() {
		return
	}
	if r.outputState.cronPending == nil {
		copyBatch := batch.clone()
		r.outputState.cronPending = &copyBatch
		return
	}
	r.outputState.cronPending.New = append(r.outputState.cronPending.New, batch.New...)
	r.outputState.cronPending.Old = append(r.outputState.cronPending.Old, batch.Old...)
	r.outputState.cronPending.Time = batch.Time
}

func (r *statementRuntime) scheduleOutput(policy OutputPolicy, at time.Time) {
	if r == nil || r.outputState == nil || policy.Kind != OutputEveryTimePolicy || policy.Interval <= 0 {
		return
	}
	if r.outputState.nextOutputAt.IsZero() {
		r.outputState.nextOutputAt = at.Add(policy.Interval)
	}
}

func (r *statementRuntime) scheduleCron(policy OutputPolicy, at time.Time) {
	if r == nil || r.outputState == nil || policy.Cron == nil || !r.outputState.cronNext.IsZero() {
		return
	}
	resolved, err := policy.Cron.resolve(EvalContext{Now: at, Variables: r.variables})
	if err != nil {
		return
	}
	next, err := resolved.nextAfter(at)
	if err != nil {
		return
	}
	r.outputState.cronSchedule = resolved
	r.outputState.cronNext = next
}

func (r *statementRuntime) ensureCronSchedule(policy OutputPolicy, now time.Time) bool {
	if r == nil || r.outputState == nil || policy.Cron == nil {
		return false
	}
	if !r.outputState.cronNext.IsZero() {
		return true
	}
	r.scheduleCron(policy, now)
	return !r.outputState.cronNext.IsZero()
}

func (r *statementRuntime) advanceOutputSchedule(policy OutputPolicy, now time.Time) {
	if r == nil || r.outputState == nil || policy.Kind != OutputEveryTimePolicy || policy.Interval <= 0 {
		return
	}
	if r.outputState.nextOutputAt.IsZero() {
		r.outputState.nextOutputAt = now.Add(policy.Interval)
		return
	}
	const maxOutputCatchUp = 100000
	for steps := 0; steps < maxOutputCatchUp && !r.outputState.nextOutputAt.After(now); steps++ {
		r.outputState.nextOutputAt = r.outputState.nextOutputAt.Add(policy.Interval)
	}
	if !r.outputState.nextOutputAt.After(now) {
		r.outputState.nextOutputAt = now.Add(policy.Interval)
	}
}

func (s *Statement) processNamedWindow(ctx context.Context, now time.Time, delta NamedWindowDelta, variables map[string]Value) (ResultBatch, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.collectOutputAssignmentsLocked()
	if s.closed || s.state != StatementStarted || !containsNamedWindow(s.plan.query.input, s.plan.query.join) {
		return ResultBatch{}, false, nil
	}
	variables = variablesWithEngineLockState(statementVariables(variables, s.parameters), s.engine, true)
	s.runtime.ctx = ctx
	if s.plan.query.contextName != "" {
		return s.processNamedWindowContextLocked(ctx, now, delta, variables)
	}
	s.runtime.variables = variables
	if s.plan.query.trigger != nil {
		result := ResultBatch{Time: now}
		for _, event := range delta.New {
			batch, err := s.processTriggerRuntime(ctx, &s.runtime, now, event, variables)
			if err != nil {
				return ResultBatch{}, false, err
			}
			result.New = append(result.New, batch.New...)
			result.Old = append(result.Old, batch.Old...)
		}
		if !result.empty() {
			result.Sequence = s.runtime.seq.Add(1)
		}
		return result, !result.empty(), nil
	}
	batch, err := s.runtime.processNamedWindowDelta(s.plan, now, delta)
	if err != nil {
		return ResultBatch{}, false, err
	}
	return batch, !batch.empty(), nil
}

func (r *statementRuntime) processNamedWindowDelta(plan Plan, now time.Time, delta NamedWindowDelta) (ResultBatch, error) {
	if r == nil {
		return ResultBatch{}, NewError(ErrorDependency, "nil statement runtime")
	}
	r.variables = r.withContextVariables(r.variables)
	r.variables = r.withContextProperties(r.variables)
	if plan.query.aggregate != nil && plan.query.aggregate.join != nil {
		joinDelta, err := r.updateJoin(plan.query.aggregate.join, now, delta.New, delta.Old)
		if err != nil {
			return ResultBatch{}, err
		}
		batch, err := r.aggregateBatch(joinDeltaEvents(joinDelta, now), plan, now)
		if err != nil {
			return ResultBatch{}, err
		}
		return r.applyOutput(plan.query.output, batch, false, now, plan), nil
	}
	if plan.query.join != nil {
		joinDelta, err := r.updateJoin(plan.query.join, now, delta.New, delta.Old)
		if err != nil {
			return ResultBatch{}, err
		}
		batch := r.joinBatch(joinDelta, plan, now)
		return r.applyOutput(plan.query.output, batch, false, now, plan), nil
	}
	if plan.query.rowRecog != nil {
		result := eventDelta{}
		for _, event := range delta.Old {
			removed, err := r.remove(plan.query.rowRecog.input, event, now)
			if err != nil {
				return ResultBatch{}, err
			}
			result = mergeDelta(result, removed)
		}
		for _, event := range delta.New {
			inserted, err := r.insert(plan.query.rowRecog.input, event, now)
			if err != nil {
				return ResultBatch{}, err
			}
			result = mergeDelta(result, inserted)
		}
		batch := r.rowRecogBatch(result, plan, now)
		return r.applyOutput(plan.query.output, batch, false, now, plan), nil
	}
	result := eventDelta{}
	for _, event := range delta.New {
		inserted, err := r.insert(plan.query.input, event, now)
		if err != nil {
			return ResultBatch{}, err
		}
		result = mergeDelta(result, inserted)
	}
	for _, event := range delta.Old {
		removed, err := r.remove(plan.query.input, event, now)
		if err != nil {
			return ResultBatch{}, err
		}
		result = mergeDelta(result, removed)
	}
	var batch ResultBatch
	var err error
	if plan.query.aggregate != nil {
		batch, err = r.aggregateBatch(result, plan, now)
	} else if plan.query.pattern != nil {
		batch = r.patternBatch(result, plan, now)
	} else {
		batch = r.batch(result, plan, now)
	}
	if err != nil {
		return ResultBatch{}, err
	}
	return r.applyOutput(plan.query.output, batch, false, now, plan), nil
}

func (s *Statement) processNamedWindowContextLocked(ctx context.Context, now time.Time, delta NamedWindowDelta, variables map[string]Value) (ResultBatch, bool, error) {
	definition, ok := s.engine.env.Context(s.plan.query.contextName)
	if !ok {
		return ResultBatch{}, false, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", s.plan.query.contextName))
	}
	type partitionDelta struct {
		newEvents []Event
		oldEvents []Event
	}
	grouped := make(map[string]*partitionDelta)
	add := func(event Event, newEvent bool) error {
		if !statementAcceptsEvent(s.plan.query, event) {
			return nil
		}
		key := ""
		if definition.isTemporal() {
			key = activeTemporalContextPartitionKey(s.engine, definition, now)
			if key == "" {
				return nil
			}
		} else {
			partitionKey, active, err := definition.partition(event, now, variables)
			if err != nil {
				return err
			}
			if !active {
				return nil
			}
			key = partitionKey
		}
		group := grouped[key]
		if group == nil {
			group = &partitionDelta{}
			grouped[key] = group
		}
		if newEvent {
			group.newEvents = append(group.newEvents, event)
		} else {
			group.oldEvents = append(group.oldEvents, event)
		}
		return nil
	}
	for _, event := range delta.New {
		if err := add(event, true); err != nil {
			return ResultBatch{}, false, err
		}
	}
	for _, event := range delta.Old {
		if err := add(event, false); err != nil {
			return ResultBatch{}, false, err
		}
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := ResultBatch{Time: now}
	for _, key := range keys {
		group := grouped[key]
		partition := s.runtime.partitions[key]
		if partition == nil {
			if definition.isTemporal() || definition.kind == ContextInitiatedTerminated {
				continue
			}
			if len(group.newEvents) == 0 {
				continue
			}
			query := s.plan.query
			query.contextName = ""
			partitionRuntime := newStatementRuntime(query)
			partitionRuntime.engine = s.engine
			partitionRuntime.rowRecogOwner = s.runtime.rowRecogOwner
			partitionRuntime.partitionContextName = s.plan.query.contextName
			partitionRuntime.partitionKey = key
			partitionRuntime.partitionID = s.allocateContextPartitionID(key)
			partitionRuntime.contextProperties = definition.contextPropertyValues(group.newEvents[0], now, variables, partitionRuntime.partitionID)
			partitionRuntime.variables = partitionRuntime.withContextProperties(variables)
			partitionRuntime.initializeAt(now)
			partition = ptrStatementRuntime(partitionRuntime)
			s.runtime.partitions[key] = partition
			s.engine.retainContextPartitionLocked(s.plan.query.contextName, key, partition)
		}
		partition.variables = partition.withContextProperties(variablesWithEngine(variables, partition.engine))
		partition.ctx = ctx
		if s.plan.query.trigger != nil {
			triggerBatch := ResultBatch{Time: now}
			for _, event := range group.newEvents {
				batch, err := s.processTriggerRuntime(ctx, partition, now, event, s.contextPartitionVariables(partition, variables))
				if err != nil {
					return ResultBatch{}, false, err
				}
				triggerBatch.New = append(triggerBatch.New, batch.New...)
				triggerBatch.Old = append(triggerBatch.Old, batch.Old...)
			}
			result.New = append(result.New, triggerBatch.New...)
			result.Old = append(result.Old, triggerBatch.Old...)
			continue
		}
		query := s.plan.query
		query.contextName = ""
		partitionPlan := s.plan
		partitionPlan.query = query
		partitionBatch, err := partition.processNamedWindowDelta(partitionPlan, now, NamedWindowDelta{New: group.newEvents, Old: group.oldEvents, Time: now})
		if err != nil {
			return ResultBatch{}, false, err
		}
		result.New = append(result.New, partitionBatch.New...)
		result.Old = append(result.Old, partitionBatch.Old...)
	}
	if result.empty() {
		return ResultBatch{}, false, nil
	}
	result.Sequence = s.runtime.seq.Add(1)
	return result, true, nil
}

func containsNamedWindow(node *streamNode, join *joinDefinition) bool {
	if join != nil {
		for _, source := range joinDefinitionSources(join) {
			if containsNamedWindow(source, nil) {
				return true
			}
		}
		return false
	}
	for current := node; current != nil; current = current.input {
		if current.kind == streamNamedWindow {
			return true
		}
	}
	return false
}

func containsTableSource(node *streamNode, join *joinDefinition) bool {
	if join != nil {
		for _, source := range joinDefinitionSources(join) {
			if containsTableSource(source, nil) {
				return true
			}
		}
		return false
	}
	for current := node; current != nil; current = current.input {
		if current.kind == streamTable {
			return true
		}
	}
	return false
}

func containsHistoricalSource(node *streamNode) bool {
	for current := node; current != nil; current = current.input {
		if current.kind == streamHistorical {
			return true
		}
	}
	return false
}

func (r *statementRuntime) insertJoin(definition *joinDefinition, event Event, now time.Time) (joinDelta, error) {
	return r.updateJoin(definition, now, []Event{event}, nil)
}

func (r *statementRuntime) updateJoin(definition *joinDefinition, now time.Time, newEvents, oldEvents []Event) (joinDelta, error) {
	if r.joinState == nil {
		r.joinState = &joinRuntimeState{}
	}
	sources := joinDefinitionSources(definition)
	if len(sources) < 2 {
		return joinDelta{}, fmt.Errorf("esper: join requires at least two sources")
	}
	if len(r.joinState.sides) != len(sources) {
		r.joinState.sides = make([][]storedEvent, len(sources))
	}
	before := joinTuples(definition, r.joinState, now, r)
	for _, event := range newEvents {
		for index, source := range sources {
			base, baseErr := sourceNode(source)
			if baseErr != nil {
				return joinDelta{}, baseErr
			}
			if base.kind == streamTable {
				// A table is a current-state source rather than an event stream.
				// Refresh its side for every trigger and let the tuple diff emit
				// the exact old/new pairs caused by table changes. SendEvent holds
				// the engine lock here, hence the locked snapshot path.
				side, snapshotErr := r.snapshotTableJoinSide(source, base, now)
				if snapshotErr != nil {
					return joinDelta{}, snapshotErr
				}
				r.joinState.sides[index] = side
				continue
			}
			if containsHistoricalSource(source) {
				// A historical poll is a per-trigger result set, not a retained
				// event window. Replace the previous poll before evaluating the
				// current join tuple.
				r.joinState.sides[index] = nil
			}
			delta, err := r.insert(source, event, now)
			if err != nil {
				return joinDelta{}, err
			}
			removeStoredEvents(&r.joinState.sides[index], delta.oldEvents)
			for _, newEvent := range delta.newEvents {
				r.joinState.sides[index] = append(r.joinState.sides[index], storedEvent{event: newEvent, receivedAt: now})
			}
		}
	}
	for _, event := range oldEvents {
		for index, source := range sources {
			delta, err := r.remove(source, event, now)
			if err != nil {
				return joinDelta{}, err
			}
			removeStoredEvents(&r.joinState.sides[index], delta.oldEvents)
		}
	}
	after := joinTuples(definition, r.joinState, now, r)
	delta := diffJoinTuples(before, after)
	return joinDeltaWithPairs(delta), nil
}

func (r *statementRuntime) snapshotTableJoinSide(source, base *streamNode, now time.Time) ([]storedEvent, error) {
	if r == nil || r.engine == nil {
		return nil, NewError(ErrorDependency, "table join requires an engine")
	}
	if base == nil || base.kind != streamTable {
		return nil, NewError(ErrorInvalidRule, "table join source is not a table")
	}
	return r.snapshotCurrentSourceSide(source, base, now)
}

func (r *statementRuntime) snapshotCurrentSourceSide(source, base *streamNode, now time.Time) ([]storedEvent, error) {
	if r == nil || r.engine == nil {
		return nil, NewError(ErrorDependency, "current-state source snapshot requires an engine")
	}
	if base == nil || (base.kind != streamNamedWindow && base.kind != streamTable) {
		return nil, NewError(ErrorInvalidRule, "current-state join source is not a named window or table")
	}
	raw, err := r.engine.snapshotFireAndForgetSourceLocked(r.context(), base, now, r.variables)
	if err != nil {
		return nil, err
	}
	raw = r.filterPartitionEvents(raw, now)
	// Evaluate consumer-local filter/window nodes against a fresh temporary
	// runtime. This prevents a current-state snapshot from accumulating duplicate
	// rows in a live join while preserving the same analyzable source chain.
	temporary := newStatementRuntime(r.query)
	temporary.engine = r.engine
	temporary.ctx = r.ctx
	temporary.variables = r.variables
	result := make([]storedEvent, 0, len(raw))
	for _, candidate := range raw {
		delta, insertErr := temporary.insert(source, candidate, now)
		if insertErr != nil {
			return nil, insertErr
		}
		for _, event := range delta.newEvents {
			result = append(result, storedEvent{event: event, receivedAt: now})
		}
	}
	return result, nil
}

func (r *statementRuntime) joinMatches(condition JoinCondition, events []Event, now time.Time) bool {
	if r == nil {
		return false
	}
	return joinConditionMatches(condition, events, now, r.variables)
}

func joinConditionMatches(condition JoinCondition, events []Event, now time.Time, variables map[string]Value) bool {
	if len(condition.all) > 0 {
		for _, child := range condition.all {
			if !joinConditionMatches(child, events, now, variables) {
				return false
			}
		}
		return true
	}
	if len(condition.any) > 0 {
		for _, child := range condition.any {
			if joinConditionMatches(child, events, now, variables) {
				return true
			}
		}
		return false
	}
	leftSource, rightSource := joinConditionSources(condition)
	if condition.Left == nil || condition.Right == nil || leftSource < 0 || rightSource < 0 || leftSource >= len(events) || rightSource >= len(events) {
		return false
	}
	leftValue := condition.Left.eval(EvalContext{Event: events[leftSource], Now: now, Variables: variables})
	rightValue := condition.Right.eval(EvalContext{Event: events[rightSource], Now: now, Variables: variables})
	switch condition.Comparison {
	case JoinEqual:
		matched, ok := boolValue(EqualValues(leftValue, rightValue))
		return ok && matched
	case JoinNotEqual:
		matched, ok := boolValue(EqualValues(leftValue, rightValue))
		return ok && !matched
	default:
		comparison, ok := compareValues(leftValue, rightValue)
		if !ok {
			return false
		}
		switch condition.Comparison {
		case JoinLess:
			return comparison < 0
		case JoinLessOrEqual:
			return comparison <= 0
		case JoinGreater:
			return comparison > 0
		case JoinGreaterOrEqual:
			return comparison >= 0
		default:
			return false
		}
	}
}

func removeStoredEvents(active *[]storedEvent, removed []Event) {
	for _, event := range removed {
		for index, stored := range *active {
			if reflect.DeepEqual(stored.event.Underlying(), event.Underlying()) && stored.event.TypeName() == event.TypeName() {
				*active = append((*active)[:index], (*active)[index+1:]...)
				break
			}
		}
	}
}

func (r *statementRuntime) expireJoin(now time.Time) joinDelta {
	if r.joinState == nil {
		return joinDelta{}
	}
	definition := r.query.join
	if definition == nil {
		return joinDelta{}
	}
	before := joinTuples(definition, r.joinState, now, r)
	delta := r.expire(now)
	for index := range r.joinState.sides {
		removeStoredEvents(&r.joinState.sides[index], delta.oldEvents)
	}
	after := joinTuples(definition, r.joinState, now, r)
	return joinDeltaWithPairs(diffJoinTuples(before, after))
}

func joinPairs(definition *joinDefinition, state *joinRuntimeState, now time.Time, runtime *statementRuntime) []eventPair {
	tuples := joinTuples(definition, state, now, runtime)
	if len(tuples) == 0 {
		return nil
	}
	pairs := make([]eventPair, 0, len(tuples))
	for _, tuple := range tuples {
		if len(tuple) == 2 {
			pairs = append(pairs, eventPair{left: tuple[0], right: tuple[1]})
		}
	}
	return pairs
}

func joinTuples(definition *joinDefinition, state *joinRuntimeState, now time.Time, runtime *statementRuntime) [][]Event {
	if definition == nil || state == nil {
		return nil
	}
	sources := joinDefinitionSources(definition)
	if len(sources) < 2 || len(state.sides) != len(sources) {
		return nil
	}
	conditions := joinDefinitionConditions(definition)
	if len(sources) == 2 && definition.kind != JoinInner {
		result := make([][]Event, 0)
		matchedRight := make(map[int]bool)
		for _, left := range state.sides[0] {
			matched := false
			for rightIndex, right := range state.sides[1] {
				tuple := []Event{left.event, right.event}
				if joinConditionsMatch(conditions, tuple, now, runtime) {
					matched = true
					matchedRight[rightIndex] = true
					result = append(result, tuple)
				}
			}
			if !matched && (definition.kind == JoinLeftOuter || definition.kind == JoinFullOuter) {
				result = append(result, []Event{left.event, Event{}})
			}
		}
		if definition.kind == JoinRightOuter || definition.kind == JoinFullOuter {
			for rightIndex, right := range state.sides[1] {
				if !matchedRight[rightIndex] {
					result = append(result, []Event{Event{}, right.event})
				}
			}
		}
		return result
	}
	if len(sources) > 2 && definition.kind != JoinInner {
		return joinOuterTuples(definition, state, now, runtime)
	}
	for _, side := range state.sides {
		if len(side) == 0 {
			return nil
		}
	}
	result := make([][]Event, 0)
	current := make([]Event, 0, len(sources))
	var visit func(int)
	visit = func(index int) {
		if index == len(state.sides) {
			candidate := append([]Event(nil), current...)
			if joinConditionsMatch(conditions, candidate, now, runtime) {
				result = append(result, candidate)
			}
			return
		}
		for _, stored := range state.sides[index] {
			current = append(current, stored.event)
			visit(index + 1)
			current = current[:len(current)-1]
		}
	}
	visit(0)
	return result
}

// joinOuterTuples enumerates matching N-way tuples first, then emits one
// placeholder tuple for every unmatched event on the selected outer edge.
// This is intentionally a semantic reference implementation; optimized
// indexed joins can replace it without changing the public contract.
func joinOuterTuples(definition *joinDefinition, state *joinRuntimeState, now time.Time, runtime *statementRuntime) [][]Event {
	sources := joinDefinitionSources(definition)
	conditions := joinDefinitionConditions(definition)
	result := make([][]Event, 0)
	matched := make([][]bool, len(state.sides))
	for index := range state.sides {
		matched[index] = make([]bool, len(state.sides[index]))
	}
	current := make([]Event, 0, len(sources))
	currentIndexes := make([]int, 0, len(sources))
	var visit func(int)
	visit = func(index int) {
		if index == len(state.sides) {
			candidate := append([]Event(nil), current...)
			if !joinConditionsMatch(conditions, candidate, now, runtime) {
				return
			}
			result = append(result, candidate)
			for sourceIndex, sideIndex := range currentIndexes {
				matched[sourceIndex][sideIndex] = true
			}
			return
		}
		for sideIndex, stored := range state.sides[index] {
			current = append(current, stored.event)
			currentIndexes = append(currentIndexes, sideIndex)
			visit(index + 1)
			current = current[:len(current)-1]
			currentIndexes = currentIndexes[:len(currentIndexes)-1]
		}
	}
	visit(0)

	if definition.kind == JoinLeftOuter || definition.kind == JoinFullOuter {
		for index, stored := range state.sides[0] {
			if matched[0][index] {
				continue
			}
			tuple := make([]Event, len(sources))
			tuple[0] = stored.event
			result = append(result, tuple)
		}
	}
	if definition.kind == JoinRightOuter || definition.kind == JoinFullOuter {
		last := len(sources) - 1
		for index, stored := range state.sides[last] {
			if matched[last][index] {
				continue
			}
			tuple := make([]Event, len(sources))
			tuple[last] = stored.event
			result = append(result, tuple)
		}
	}
	return result
}

func joinConditionsMatch(conditions []JoinCondition, events []Event, now time.Time, runtime *statementRuntime) bool {
	if runtime == nil {
		return false
	}
	return joinConditionsMatchWithVariables(conditions, events, now, runtime.variables)
}

func joinConditionsMatchWithVariables(conditions []JoinCondition, events []Event, now time.Time, variables map[string]Value) bool {
	for _, condition := range conditions {
		if !joinConditionMatches(condition, events, now, variables) {
			return false
		}
	}
	return true
}

func joinDeltaWithPairs(delta joinDelta) joinDelta {
	for _, tuple := range delta.newTuples {
		if len(tuple) == 2 {
			delta.newPairs = append(delta.newPairs, eventPair{left: tuple[0], right: tuple[1]})
		}
	}
	for _, tuple := range delta.oldTuples {
		if len(tuple) == 2 {
			delta.oldPairs = append(delta.oldPairs, eventPair{left: tuple[0], right: tuple[1]})
		}
	}
	return delta
}

func diffJoinPairs(before, after []eventPair) joinDelta {
	result := joinDelta{}
	used := make([]bool, len(after))
	for _, oldPair := range before {
		found := -1
		for index, newPair := range after {
			if !used[index] && equalEventPair(oldPair, newPair) {
				found = index
				break
			}
		}
		if found >= 0 {
			used[found] = true
		} else {
			result.oldPairs = append(result.oldPairs, oldPair)
		}
	}
	for index, newPair := range after {
		if !used[index] {
			result.newPairs = append(result.newPairs, newPair)
		}
	}
	return result
}

func diffJoinTuples(before, after [][]Event) joinDelta {
	result := joinDelta{}
	used := make([]bool, len(after))
	for _, oldTuple := range before {
		found := -1
		for index, newTuple := range after {
			if !used[index] && equalEventTuple(oldTuple, newTuple) {
				found = index
				break
			}
		}
		if found >= 0 {
			used[found] = true
		} else {
			result.oldTuples = append(result.oldTuples, oldTuple)
		}
	}
	for index, newTuple := range after {
		if !used[index] {
			result.newTuples = append(result.newTuples, newTuple)
		}
	}
	return result
}

func equalEventTuple(left, right []Event) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !sameEvent(left[index], right[index]) {
			return false
		}
	}
	return true
}

func equalEventPair(left, right eventPair) bool {
	return sameEvent(left.left, right.left) && sameEvent(left.right, right.right)
}

func (r *statementRuntime) insert(node *streamNode, event Event, now time.Time) (eventDelta, error) {
	if node == nil {
		return eventDelta{}, fmt.Errorf("esper: runtime encountered nil stream node")
	}
	switch node.kind {
	case streamSource, streamNamedWindow, streamTable:
		if !sourceNodeAcceptsEvent(r.query.env, node, event) {
			return eventDelta{}, nil
		}
		return eventDelta{newEvents: []Event{event}}, nil
	case streamHistorical:
		if node.historical == nil || node.historical.provider == nil {
			return eventDelta{}, NewError(ErrorDependency, fmt.Sprintf("historical source %q has no provider", node.sourceName))
		}
		if node.historical.trigger != "" && node.historical.trigger != event.TypeName() {
			return eventDelta{}, nil
		}
		events, err := node.historical.provider.Poll(r.context(), HistoricalRequest{Trigger: event, Now: now, Variables: visibleVariableValues(r.variables), Parameters: parameterValuesFromVariables(r.variables)})
		if err != nil {
			return eventDelta{}, err
		}
		return eventDelta{newEvents: append([]Event(nil), events...)}, nil
	case streamFilter:
		inputDelta, err := r.insert(node.input, event, now)
		if err != nil {
			return eventDelta{}, err
		}
		filtered := eventDelta{
			history:         append([]Event(nil), inputDelta.history...),
			historyByEvent:  cloneEventHistories(inputDelta.historyByEvent),
			previousByEvent: cloneEventHistories(inputDelta.previousByEvent),
			priorByEvent:    cloneEventHistories(inputDelta.priorByEvent),
		}
		for _, candidate := range inputDelta.newEvents {
			value := node.predicate.eval(EvalContext{Event: candidate, History: historyForEvent(inputDelta, candidate), Now: now, Variables: r.variables})
			if ok, isBool := boolValue(value); isBool && ok {
				filtered.newEvents = append(filtered.newEvents, candidate)
			}
		}
		for _, candidate := range inputDelta.oldEvents {
			value := node.predicate.eval(EvalContext{Event: candidate, History: historyForEvent(inputDelta, candidate), Now: now, Variables: r.variables})
			if ok, isBool := boolValue(value); isBool && ok {
				filtered.oldEvents = append(filtered.oldEvents, candidate)
			}
		}
		return filtered, nil
	case streamWindow:
		inputDelta, err := r.insert(node.input, event, now)
		if err != nil {
			return eventDelta{}, err
		}
		state := r.windows[node]
		if state == nil {
			state = &windowRuntimeState{}
		}
		result := eventDelta{
			oldEvents:       append([]Event(nil), inputDelta.oldEvents...),
			history:         append([]Event(nil), inputDelta.history...),
			historyByEvent:  cloneEventHistories(inputDelta.historyByEvent),
			previousByEvent: cloneEventHistories(inputDelta.previousByEvent),
			priorByEvent:    cloneEventHistories(inputDelta.priorByEvent),
		}
		for _, candidate := range inputDelta.newEvents {
			delta, addErr := r.addToWindow(node.window, state, candidate, now)
			if addErr != nil {
				return eventDelta{}, addErr
			}
			result = mergeDelta(result, delta)
		}
		r.windows[node] = state
		result.history = windowHistory(node.window, state)
		result.historyByEvent = windowHistoryByEvent(node.window, state)
		if windowUsesPreviousAccess(node.window) {
			result.previousByEvent = windowPreviousAccessByEvent(node.window, state)
			if result.previousByEvent == nil {
				result.previousByEvent = make(map[string][]Event)
			}
			newIdentities := make(map[string]struct{}, len(result.newEvents))
			for _, current := range result.newEvents {
				identity := eventIdentity(current)
				newIdentities[identity] = struct{}{}
				if _, exists := result.previousByEvent[identity]; exists {
					continue
				}
				result.previousByEvent[identity] = append([]Event(nil), windowPreviousAccessHistoryForEvent(node.window, state, current, now, r.variables)...)
			}
			for _, old := range result.oldEvents {
				if _, isNew := newIdentities[eventIdentity(old)]; isNew {
					continue
				}
				if result.historyByEvent == nil {
					result.historyByEvent = make(map[string][]Event)
				}
				result.historyByEvent[eventIdentity(old)] = nil
			}
		}
		return result, nil
	default:
		return eventDelta{}, fmt.Errorf("esper: runtime encountered unknown stream node kind %d", node.kind)
	}
}

func (r *statementRuntime) remove(node *streamNode, event Event, now time.Time) (eventDelta, error) {
	if node == nil {
		return eventDelta{}, fmt.Errorf("esper: runtime encountered nil stream node")
	}
	switch node.kind {
	case streamSource, streamNamedWindow, streamTable:
		if !sourceNodeAcceptsEvent(r.query.env, node, event) {
			return eventDelta{}, nil
		}
		return eventDelta{oldEvents: []Event{event}}, nil
	case streamHistorical:
		return eventDelta{}, nil
	case streamFilter:
		inputDelta, err := r.remove(node.input, event, now)
		if err != nil {
			return eventDelta{}, err
		}
		filtered := eventDelta{
			history:         append([]Event(nil), inputDelta.history...),
			historyByEvent:  cloneEventHistories(inputDelta.historyByEvent),
			previousByEvent: cloneEventHistories(inputDelta.previousByEvent),
			priorByEvent:    cloneEventHistories(inputDelta.priorByEvent),
		}
		for _, candidate := range inputDelta.oldEvents {
			value := node.predicate.eval(EvalContext{Event: candidate, History: historyForEvent(inputDelta, candidate), Now: now, Variables: r.variables})
			if ok, isBool := boolValue(value); isBool && ok {
				filtered.oldEvents = append(filtered.oldEvents, candidate)
			}
		}
		filtered.history = append([]Event(nil), inputDelta.history...)
		return filtered, nil
	case streamWindow:
		inputDelta, err := r.remove(node.input, event, now)
		if err != nil {
			return eventDelta{}, err
		}
		state := r.windows[node]
		if state == nil {
			return eventDelta{}, nil
		}
		result := eventDelta{}
		for _, candidate := range inputDelta.oldEvents {
			if windowUsesPreviousAccess(node.window) {
				if history := windowPriorHistoryForEvent(node.window, state, candidate); history != nil {
					if result.priorByEvent == nil {
						result.priorByEvent = make(map[string][]Event)
					}
					result.priorByEvent[eventIdentity(candidate)] = history
				}
			}
			if removeFromWindowState(node.window, state, candidate, now, r.variables) {
				result.oldEvents = append(result.oldEvents, candidate)
			}
		}
		result.history = windowHistory(node.window, state)
		result.historyByEvent = windowHistoryByEvent(node.window, state)
		result.previousByEvent = windowPreviousAccessByEvent(node.window, state)
		if windowUsesPreviousAccess(node.window) {
			if result.previousByEvent == nil {
				result.previousByEvent = make(map[string][]Event)
			}
			for _, old := range result.oldEvents {
				result.previousByEvent[eventIdentity(old)] = nil
				if result.historyByEvent == nil {
					result.historyByEvent = make(map[string][]Event)
				}
				result.historyByEvent[eventIdentity(old)] = nil
			}
		}
		if windowStateEmpty(state) {
			delete(r.windows, node)
		}
		return result, nil
	default:
		return eventDelta{}, fmt.Errorf("esper: runtime encountered unknown stream node kind %d", node.kind)
	}
}

func removeFromWindowState(spec WindowSpec, state *windowRuntimeState, event Event, now time.Time, variables map[string]Value) bool {
	if window, ok := spec.(CompositeWindowSpec); ok {
		wasActive := false
		for _, stored := range state.entries {
			if sameEvent(stored.event, event) {
				wasActive = true
				break
			}
		}
		for index, child := range window.Windows {
			if index < len(state.children) {
				removeFromWindowState(child, state.children[index], event, now, variables)
			}
		}
		reconcileCompositeWindow(state, window, now)
		return wasActive && !containsEvent(state.entries, event)
	}
	if window, ok := spec.(GroupWindowSpec); ok {
		if state.groups == nil {
			return false
		}
		key := groupWindowKey(window.Key, event, now, variables)
		child := state.groups[key]
		if child == nil {
			return false
		}
		removed := removeFromWindowState(window.Inner, child, event, now, variables)
		if windowStateEmpty(child) {
			delete(state.groups, key)
		}
		return removed
	}
	for index, stored := range state.entries {
		if sameEvent(stored.event, event) {
			state.entries = append(state.entries[:index], state.entries[index+1:]...)
			if windowUsesArrivalPrior(spec) {
				removeArrivalEvents(&state.arrival, []Event{event})
			}
			if sorted, ok := spec.(SortedWindowSpec); ok && sorted.Rank && len(sorted.UniqueKeys) > 0 && state.keyed != nil {
				key := sortedWindowKey(sorted, event, now, variables)
				if current, exists := state.keyed[key]; exists && sameEvent(current.event, event) {
					delete(state.keyed, key)
				}
			}
			return true
		}
	}
	for index, stored := range state.pendingNew {
		if sameEvent(stored.event, event) {
			state.pendingNew = append(state.pendingNew[:index], state.pendingNew[index+1:]...)
			return false
		}
	}
	if window, ok := spec.(UniqueWindowSpec); ok && state.keyed != nil {
		key := uniqueWindowKey(window, event, now, variables)
		if stored, exists := state.keyed[key]; exists && sameEvent(stored.event, event) {
			delete(state.keyed, key)
			removeWindowKeyOrder(state, key)
			return true
		}
	}
	return false
}

func sameEvent(left, right Event) bool {
	return left.TypeName() == right.TypeName() && reflect.DeepEqual(left.Underlying(), right.Underlying())
}

func (r *statementRuntime) addToWindow(spec WindowSpec, state *windowRuntimeState, event Event, now time.Time) (eventDelta, error) {
	stored := storedEvent{event: event, receivedAt: now}
	switch window := spec.(type) {
	case GroupWindowSpec:
		if state.groups == nil {
			state.groups = make(map[string]*windowRuntimeState)
		}
		key := groupWindowKey(window.Key, event, now, r.variables)
		child := state.groups[key]
		if child == nil {
			child = &windowRuntimeState{}
		}
		result, err := r.addToWindow(window.Inner, child, event, now)
		if err != nil {
			return eventDelta{}, err
		}
		state.groups[key] = child
		return result, nil
	case CompositeWindowSpec:
		if len(state.children) == 0 {
			state.children = make([]*windowRuntimeState, len(window.Windows))
		}
		for index, childSpec := range window.Windows {
			child := state.children[index]
			if child == nil {
				child = &windowRuntimeState{}
				state.children[index] = child
			}
			if _, err := r.addToWindow(childSpec, child, event, now); err != nil {
				return eventDelta{}, err
			}
		}
		return reconcileCompositeWindow(state, window, now), nil
	case KeepAllWindowSpec:
		state.entries = append(state.entries, stored)
		return eventDelta{newEvents: []Event{event}}, nil
	case LengthWindowSpec:
		state.entries = append(state.entries, stored)
		result := eventDelta{newEvents: []Event{event}}
		for len(state.entries) > window.Size {
			result.oldEvents = append(result.oldEvents, state.entries[0].event)
			state.entries = state.entries[1:]
		}
		return result, nil
	case TimeWindowSpec:
		// Time advancement, not Send, owns expiry. This makes tests fully
		// deterministic and avoids wall-clock sleeps.
		state.entries = append(state.entries, stored)
		return eventDelta{newEvents: []Event{event}}, nil
	case TimeToLiveWindowSpec:
		state.entries = append(state.entries, stored)
		return eventDelta{newEvents: []Event{event}}, nil
	case TimeToLiveAtWindowSpec:
		expiresAt, err := eventTimestamp(window.Timestamp, event, now, r.variables)
		if err != nil {
			return eventDelta{}, err
		}
		stored.expiresAt = expiresAt
		if !expiresAt.After(now) {
			return eventDelta{newEvents: []Event{event}, oldEvents: []Event{event}}, nil
		}
		state.entries = append(state.entries, stored)
		return eventDelta{newEvents: []Event{event}}, nil
	case TimeOrderWindowSpec:
		externalAt, err := eventTimestamp(window.Timestamp, event, now, r.variables)
		if err != nil {
			return eventDelta{}, err
		}
		priorHistory := append(append([]Event(nil), state.arrival...), event)
		state.arrival = append(state.arrival, event)
		if len(state.entries) == 0 || externalAt.After(state.externalAt) {
			state.externalAt = externalAt
		}
		state.entries = append(state.entries, storedEvent{event: event, receivedAt: externalAt})
		sort.SliceStable(state.entries, func(i, j int) bool {
			return state.entries[i].receivedAt.Before(state.entries[j].receivedAt)
		})
		result := eventDelta{newEvents: []Event{event}}
		cutoff := state.externalAt
		if now.After(cutoff) {
			cutoff = now
		}
		kept := state.entries[:0]
		for _, existing := range state.entries {
			if timeOrderExpiry(window, existing.receivedAt).After(cutoff) {
				kept = append(kept, existing)
			} else {
				result.oldEvents = append(result.oldEvents, existing.event)
			}
		}
		state.entries = kept
		if len(result.oldEvents) > 0 {
			if result.priorByEvent == nil {
				result.priorByEvent = make(map[string][]Event)
			}
			addPriorHistories(result.priorByEvent, state.arrival, result.oldEvents)
		}
		removeArrivalEvents(&state.arrival, result.oldEvents)
		if len(state.entries) == 0 {
			state.externalAt = time.Time{}
		}
		if result.priorByEvent == nil {
			result.priorByEvent = make(map[string][]Event)
		}
		result.priorByEvent[eventIdentity(event)] = priorHistory
		return result, nil
	case LengthBatchWindowSpec:
		state.pendingNew = append(state.pendingNew, stored)
		return flushLengthBatch(state, window.Size), nil
	case TimeBatchWindowSpec:
		if !state.started {
			state.started = true
			state.start = now
		}
		state.pendingNew = append(state.pendingNew, stored)
		return eventDelta{}, nil
	case TimeLengthBatchWindowSpec:
		if !state.started {
			state.started = true
			state.start = now
		}
		state.pendingNew = append(state.pendingNew, stored)
		if len(state.pendingNew) >= window.Size {
			result := flushPendingBatch(state)
			state.start = now
			return result, nil
		}
		return eventDelta{}, nil
	case ExternallyTimedWindowSpec:
		externalAt, err := eventTimestamp(window.Timestamp, event, now, r.variables)
		if err != nil {
			return eventDelta{}, err
		}
		result := eventDelta{}
		if window.Batch {
			if !state.started {
				state.started = true
				state.start = externalAt
			}
			state.pendingNew = append(state.pendingNew, storedEvent{event: event, receivedAt: externalAt})
			if !externalAt.Before(state.start.Add(window.Duration)) {
				result = flushPendingBatch(state)
				state.start = externalAt
			}
			state.externalAt = externalAt
			return result, nil
		}
		kept := state.entries[:0]
		for _, existing := range state.entries {
			if existing.receivedAt.Add(window.Duration).After(externalAt) {
				kept = append(kept, existing)
			} else {
				result.oldEvents = append(result.oldEvents, existing.event)
			}
		}
		state.entries = append(kept, storedEvent{event: event, receivedAt: externalAt})
		state.externalAt = externalAt
		result.newEvents = append(result.newEvents, event)
		return result, nil
	case FirstEventWindowSpec:
		if state.started {
			return eventDelta{}, nil
		}
		state.started = true
		state.entries = append(state.entries, stored)
		return eventDelta{newEvents: []Event{event}}, nil
	case LastEventWindowSpec:
		result := eventDelta{newEvents: []Event{event}}
		if len(state.entries) > 0 {
			result.oldEvents = append(result.oldEvents, state.entries[len(state.entries)-1].event)
		}
		state.entries = []storedEvent{stored}
		return result, nil
	case FirstLengthWindowSpec:
		if len(state.entries) >= window.Size {
			return eventDelta{}, nil
		}
		state.entries = append(state.entries, stored)
		return eventDelta{newEvents: []Event{event}}, nil
	case FirstTimeWindowSpec:
		if !state.started {
			state.started = true
			state.start = now
		}
		if !now.Before(state.start.Add(window.Duration)) {
			return eventDelta{}, nil
		}
		state.entries = append(state.entries, stored)
		return eventDelta{newEvents: []Event{event}}, nil
	case TimeAccumWindowSpec:
		if !state.started {
			state.started = true
			state.start = now
		}
		state.entries = append(state.entries, stored)
		return eventDelta{newEvents: []Event{event}}, nil
	case ExpressionWindowSpec:
		state.entries = append(state.entries, stored)
		result := eventDelta{newEvents: []Event{event}}
		for len(state.entries) > 0 && !windowPredicate(window.Keep, state.entries, now, r.variables) {
			result.oldEvents = append(result.oldEvents, state.entries[0].event)
			state.entries = state.entries[1:]
		}
		return result, nil
	case ExpressionBatchWindowSpec:
		candidate := append(append([]storedEvent(nil), state.pendingNew...), stored)
		if !windowPredicate(window.Trigger, candidate, now, r.variables) {
			state.pendingNew = candidate
			return eventDelta{}, nil
		}
		if window.IncludeTrigger {
			state.pendingNew = candidate
			return flushPendingBatch(state), nil
		}
		// The trigger event starts the next batch. The preceding pending
		// events are emitted now, while the trigger remains pending.
		pending := state.pendingNew
		state.pendingNew = nil
		result := eventDelta{}
		if len(pending) > 0 {
			state.pendingNew = pending
			result = flushPendingBatch(state)
		}
		state.pendingNew = []storedEvent{stored}
		return result, nil
	case UniqueWindowSpec:
		if state.keyed == nil {
			state.keyed = make(map[string]storedEvent)
		}
		key := uniqueWindowKey(window, event, now, r.variables)
		if previous, exists := state.keyed[key]; exists && window.First {
			return eventDelta{}, nil
		} else if exists {
			state.keyed[key] = stored
			return eventDelta{newEvents: []Event{event}, oldEvents: []Event{previous.event}}, nil
		}
		state.keyed[key] = stored
		state.keyOrder = append(state.keyOrder, key)
		return eventDelta{newEvents: []Event{event}}, nil
	case SortedWindowSpec:
		result := eventDelta{newEvents: []Event{event}}
		priorHistory := append(append([]Event(nil), state.arrival...), event)
		state.arrival = append(state.arrival, event)
		var uniqueKey string
		if window.Rank && len(window.UniqueKeys) > 0 {
			if state.keyed == nil {
				state.keyed = make(map[string]storedEvent)
			}
			uniqueKey = sortedWindowKey(window, event, now, r.variables)
			if previous, exists := state.keyed[uniqueKey]; exists {
				for index, candidate := range state.entries {
					if sameEvent(candidate.event, previous.event) {
						state.entries = append(state.entries[:index], state.entries[index+1:]...)
						result.oldEvents = append(result.oldEvents, previous.event)
						break
					}
				}
				delete(state.keyed, uniqueKey)
			}
		}
		state.entries = append(state.entries, stored)
		sort.SliceStable(state.entries, func(i, j int) bool {
			return compareStoredEvents(state.entries[i].event, state.entries[j].event, window.Keys, now, r.variables) < 0
		})
		if !window.Rank {
			reverseSortedEqualRuns(state.entries, window.Keys, now, r.variables)
		}
		for len(state.entries) > window.Size {
			removeIndex := len(state.entries) - 1
			if window.Rank {
				removeIndex = rankEvictionIndex(state.entries, window.Keys, now, r.variables)
			}
			removed := state.entries[removeIndex]
			result.oldEvents = append(result.oldEvents, removed.event)
			if window.Rank && len(window.UniqueKeys) > 0 && state.keyed != nil {
				removedKey := sortedWindowKey(window, removed.event, now, r.variables)
				delete(state.keyed, removedKey)
			}
			state.entries = append(state.entries[:removeIndex], state.entries[removeIndex+1:]...)
		}
		if window.Rank && len(window.UniqueKeys) > 0 {
			for _, retained := range state.entries {
				if sameEvent(retained.event, event) {
					state.keyed[uniqueKey] = retained
					break
				}
			}
		}
		if len(result.oldEvents) > 0 {
			if result.priorByEvent == nil {
				result.priorByEvent = make(map[string][]Event)
			}
			addPriorHistories(result.priorByEvent, state.arrival, result.oldEvents)
		}
		removeArrivalEvents(&state.arrival, result.oldEvents)
		if result.priorByEvent == nil {
			result.priorByEvent = make(map[string][]Event)
		}
		result.priorByEvent[eventIdentity(event)] = priorHistory
		return result, nil
	default:
		return eventDelta{}, fmt.Errorf("esper: unsupported window %T", spec)
	}
}

func uniqueWindowKey(window UniqueWindowSpec, event Event, now time.Time, variables map[string]Value) string {
	keys := window.keyExpressions()
	values := make([]any, 0, len(keys)*2)
	for _, expression := range keys {
		if expression == nil {
			values = append(values, ValueMissing, nil)
			continue
		}
		value := expression.eval(EvalContext{Event: event, Now: now, Variables: variables})
		values = append(values, value.State(), value.Any())
	}
	return encodeKey(values)
}

func groupWindowKey(expression Expr, event Event, now time.Time, variables map[string]Value) string {
	if expression == nil {
		return encodeKey([]any{ValueMissing, nil})
	}
	value := expression.eval(EvalContext{Event: event, Now: now, Variables: variables})
	return encodeKey([]any{value.State(), value.Any()})
}

func sortedWindowKey(window SortedWindowSpec, event Event, now time.Time, variables map[string]Value) string {
	values := make([]any, 0, len(window.UniqueKeys)*2)
	for _, expression := range window.UniqueKeys {
		if expression == nil {
			values = append(values, ValueMissing, nil)
			continue
		}
		value := expression.eval(EvalContext{Event: event, Now: now, Variables: variables})
		values = append(values, value.State(), value.Any())
	}
	return encodeKey(values)
}

func reverseSortedEqualRuns(entries []storedEvent, keys []SortKey, now time.Time, variables map[string]Value) {
	for start := 0; start < len(entries); {
		end := start + 1
		for end < len(entries) && compareStoredEvents(entries[start].event, entries[end].event, keys, now, variables) == 0 {
			end++
		}
		for left, right := start, end-1; left < right; left, right = left+1, right-1 {
			entries[left], entries[right] = entries[right], entries[left]
		}
		start = end
	}
}

func rankEvictionIndex(entries []storedEvent, keys []SortKey, now time.Time, variables map[string]Value) int {
	if len(entries) == 0 {
		return 0
	}
	last := len(entries) - 1
	firstEqual := last
	for firstEqual > 0 && compareStoredEvents(entries[firstEqual-1].event, entries[last].event, keys, now, variables) == 0 {
		firstEqual--
	}
	return firstEqual
}

func flushLengthBatch(state *windowRuntimeState, size int) eventDelta {
	if len(state.pendingNew) < size {
		return eventDelta{}
	}
	return flushPendingBatch(state)
}

func flushPendingBatch(state *windowRuntimeState) eventDelta {
	result := eventDelta{}
	for _, old := range state.entries {
		result.oldEvents = append(result.oldEvents, old.event)
	}
	for _, newEvent := range state.pendingNew {
		result.newEvents = append(result.newEvents, newEvent.event)
	}
	state.entries = append([]storedEvent(nil), state.pendingNew...)
	state.pendingNew = nil
	return result
}

func (r *statementRuntime) expire(now time.Time) eventDelta {
	var nodes []*streamNode
	for node := range r.windows {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].describe() < nodes[j].describe() })
	result := eventDelta{}
	for _, node := range nodes {
		state := r.windows[node]
		delta := r.expireWindowState(node.window, state, now)
		delta.history = windowHistory(node.window, state)
		delta.historyByEvent = windowHistoryByEvent(node.window, state)
		if windowUsesPreviousAccess(node.window) {
			delta.previousByEvent = windowPreviousAccessByEvent(node.window, state)
			if delta.previousByEvent == nil {
				delta.previousByEvent = make(map[string][]Event)
			}
			for _, old := range delta.oldEvents {
				delta.previousByEvent[eventIdentity(old)] = nil
				if delta.historyByEvent == nil {
					delta.historyByEvent = make(map[string][]Event)
				}
				delta.historyByEvent[eventIdentity(old)] = nil
			}
		}
		result = mergeDelta(result, delta)
		if windowStateEmpty(state) {
			delete(r.windows, node)
		}
	}
	return result
}

func (r *statementRuntime) expireWindowState(spec WindowSpec, state *windowRuntimeState, now time.Time) eventDelta {
	if state == nil {
		return eventDelta{}
	}
	if window, ok := spec.(GroupWindowSpec); ok {
		result := eventDelta{}
		keys := make([]string, 0, len(state.groups))
		for key := range state.groups {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := state.groups[key]
			result = mergeDelta(result, r.expireWindowState(window.Inner, child, now))
			if windowStateEmpty(child) {
				delete(state.groups, key)
			}
		}
		return result
	}
	if window, ok := spec.(CompositeWindowSpec); ok {
		for index, childSpec := range window.Windows {
			if index < len(state.children) {
				r.expireWindowState(childSpec, state.children[index], now)
			}
		}
		return reconcileCompositeWindow(state, window, now)
	}

	var result eventDelta
	switch window := spec.(type) {
	case TimeWindowSpec:
		kept := state.entries[:0]
		for _, stored := range state.entries {
			if !stored.receivedAt.Add(window.Duration).After(now) {
				result.oldEvents = append(result.oldEvents, stored.event)
				continue
			}
			kept = append(kept, stored)
		}
		state.entries = kept
	case TimeToLiveWindowSpec:
		kept := state.entries[:0]
		for _, stored := range state.entries {
			if !stored.receivedAt.Add(window.Duration).After(now) {
				result.oldEvents = append(result.oldEvents, stored.event)
				continue
			}
			kept = append(kept, stored)
		}
		state.entries = kept
	case TimeToLiveAtWindowSpec:
		kept := state.entries[:0]
		for _, stored := range state.entries {
			if !stored.expiresAt.After(now) {
				result.oldEvents = append(result.oldEvents, stored.event)
				continue
			}
			kept = append(kept, stored)
		}
		state.entries = kept
	case TimeOrderWindowSpec:
		kept := state.entries[:0]
		for _, stored := range state.entries {
			if !timeOrderExpiry(window, stored.receivedAt).After(now) {
				result.oldEvents = append(result.oldEvents, stored.event)
				continue
			}
			kept = append(kept, stored)
		}
		state.entries = kept
		if len(result.oldEvents) > 0 {
			if result.priorByEvent == nil {
				result.priorByEvent = make(map[string][]Event)
			}
			addPriorHistories(result.priorByEvent, state.arrival, result.oldEvents)
		}
		removeArrivalEvents(&state.arrival, result.oldEvents)
		if len(state.entries) == 0 {
			state.externalAt = time.Time{}
		} else {
			state.externalAt = state.entries[len(state.entries)-1].receivedAt
		}
	case TimeBatchWindowSpec:
		if state.started && !now.Before(state.start.Add(window.Duration)) && len(state.pendingNew) > 0 {
			result = mergeDelta(result, flushPendingBatch(state))
			state.start = now
		}
	case TimeLengthBatchWindowSpec:
		if state.started && !now.Before(state.start.Add(window.Duration)) && len(state.pendingNew) > 0 {
			result = mergeDelta(result, flushPendingBatch(state))
			state.start = now
		}
	case FirstTimeWindowSpec:
		if state.started && !now.Before(state.start.Add(window.Duration)) {
			for _, stored := range state.entries {
				result.oldEvents = append(result.oldEvents, stored.event)
			}
			state.entries = nil
		}
	case TimeAccumWindowSpec:
		if state.started && !now.Before(state.start.Add(window.Duration)) {
			for _, stored := range state.entries {
				result.oldEvents = append(result.oldEvents, stored.event)
			}
			state.entries = nil
			state.started = false
		}
	case ExpressionWindowSpec:
		for len(state.entries) > 0 && !windowPredicate(window.Keep, state.entries, now, r.variables) {
			result.oldEvents = append(result.oldEvents, state.entries[0].event)
			state.entries = state.entries[1:]
		}
	}
	return result
}

func windowStateEmpty(state *windowRuntimeState) bool {
	if state == nil {
		return true
	}
	if len(state.entries) != 0 || len(state.pendingNew) != 0 || len(state.arrival) != 0 || len(state.keyed) != 0 || !state.externalAt.IsZero() {
		return false
	}
	for _, child := range state.groups {
		if !windowStateEmpty(child) {
			return false
		}
	}
	for _, child := range state.children {
		if !windowStateEmpty(child) {
			return false
		}
	}
	return true
}

func reconcileCompositeWindow(state *windowRuntimeState, spec CompositeWindowSpec, now time.Time) eventDelta {
	previous := make(map[string]storedEvent, len(state.entries))
	for _, stored := range state.entries {
		previous[eventIdentity(stored.event)] = stored
	}

	candidates := make(map[string]storedEvent)
	candidateOrder := make([]string, 0)
	presence := make(map[string]int)
	for index, child := range state.children {
		if index >= len(spec.Windows) {
			continue
		}
		seenInChild := make(map[string]struct{})
		for _, stored := range storedWindowHistory(spec.Windows[index], child) {
			key := eventIdentity(stored.event)
			if _, exists := candidates[key]; !exists {
				candidates[key] = stored
				candidateOrder = append(candidateOrder, key)
			}
			if _, exists := seenInChild[key]; !exists {
				presence[key]++
				seenInChild[key] = struct{}{}
			}
		}
	}

	eligible := make(map[string]storedEvent, len(candidates))
	for index, child := range state.children {
		if index >= len(spec.Windows) {
			continue
		}
		for _, stored := range storedWindowHistory(spec.Windows[index], child) {
			key := eventIdentity(stored.event)
			if spec.Mode == UnionWindowMode || presence[key] == len(state.children) {
				eligible[key] = stored
			}
		}
	}

	// A composite view keeps the existing active order when a child replaces
	// an event. Newly active events are appended in child traversal order. This
	// matches the stable iterator order of Java's union/intersect views and
	// prevents a replacement in one Unique child from reordering the whole
	// composite snapshot.
	active := make([]storedEvent, 0, len(eligible))
	seen := make(map[string]struct{}, len(eligible))
	for _, stored := range state.entries {
		key := eventIdentity(stored.event)
		if retained, ok := eligible[key]; ok {
			active = append(active, retained)
			seen[key] = struct{}{}
		}
	}
	for _, key := range candidateOrder {
		if _, exists := seen[key]; exists {
			continue
		}
		if stored, ok := eligible[key]; ok {
			active = append(active, stored)
			seen[key] = struct{}{}
		}
	}

	result := eventDelta{}
	for _, stored := range state.entries {
		if _, exists := seen[eventIdentity(stored.event)]; !exists {
			result.oldEvents = append(result.oldEvents, stored.event)
		}
	}
	for _, stored := range active {
		if _, exists := previous[eventIdentity(stored.event)]; !exists {
			result.newEvents = append(result.newEvents, stored.event)
		}
	}
	state.entries = active
	_ = now
	return result
}

// storedWindowHistory exposes the retained events of a child view in the same
// order used by windowHistory, while preserving received timestamps for
// composite reconciliation. Unique windows keep their current entries in a
// keyed map rather than state.entries; treating every child as a flat entries
// slice would silently break composite unique/group views.
func storedWindowHistory(spec WindowSpec, state *windowRuntimeState) []storedEvent {
	if state == nil {
		return nil
	}
	if window, ok := spec.(GroupWindowSpec); ok {
		keys := make([]string, 0, len(state.groups))
		for key := range state.groups {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var result []storedEvent
		for _, key := range keys {
			result = append(result, storedWindowHistory(window.Inner, state.groups[key])...)
		}
		return result
	}
	if _, ok := spec.(UniqueWindowSpec); ok {
		keys := append([]string(nil), state.keyOrder...)
		if len(keys) == 0 {
			for key := range state.keyed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
		}
		result := make([]storedEvent, 0, len(state.keyed))
		seen := make(map[string]struct{}, len(keys))
		for _, key := range keys {
			stored, ok := state.keyed[key]
			if !ok {
				continue
			}
			result = append(result, stored)
			seen[key] = struct{}{}
		}
		if len(seen) != len(state.keyed) {
			remaining := make([]string, 0, len(state.keyed)-len(seen))
			for key := range state.keyed {
				if _, ok := seen[key]; !ok {
					remaining = append(remaining, key)
				}
			}
			sort.Strings(remaining)
			for _, key := range remaining {
				result = append(result, state.keyed[key])
			}
		}
		return result
	}
	return append([]storedEvent(nil), state.entries...)
}

func containsEvent(events []storedEvent, target Event) bool {
	for _, stored := range events {
		if sameEvent(stored.event, target) {
			return true
		}
	}
	return false
}

func eventIdentity(event Event) string {
	return fmt.Sprintf("%s:%T:%#v", event.TypeName(), event.Underlying(), event.Underlying())
}

func cloneEventHistories(histories map[string][]Event) map[string][]Event {
	if len(histories) == 0 {
		return nil
	}
	result := make(map[string][]Event, len(histories))
	for key, history := range histories {
		result[key] = append([]Event(nil), history...)
	}
	return result
}

func historyForEvent(delta eventDelta, event Event) []Event {
	if delta.historyByEvent != nil {
		if history, ok := delta.historyByEvent[eventIdentity(event)]; ok {
			return append([]Event(nil), history...)
		}
	}
	return append([]Event(nil), delta.history...)
}

func eventsFromStored(entries []storedEvent) []Event {
	if len(entries) == 0 {
		return nil
	}
	result := make([]Event, 0, len(entries))
	for _, stored := range entries {
		result = append(result, stored.event)
	}
	return result
}

// windowHistory returns the retained events in the order used by previous
// value expressions. Group windows are flattened only as a fallback; the
// event-specific map below preserves the correct partition history.
func windowHistory(spec WindowSpec, state *windowRuntimeState) []Event {
	if state == nil {
		return nil
	}
	if window, ok := spec.(GroupWindowSpec); ok {
		keys := make([]string, 0, len(state.groups))
		for key := range state.groups {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var result []Event
		for _, key := range keys {
			result = append(result, windowHistory(window.Inner, state.groups[key])...)
		}
		return result
	}
	if _, ok := spec.(UniqueWindowSpec); ok {
		return windowHistoryFromUniqueState(state)
	}
	return windowHistoryFromState(state)
}

func windowHistoryFromUniqueState(state *windowRuntimeState) []Event {
	if state == nil || len(state.keyed) == 0 {
		return nil
	}
	keys := append([]string(nil), state.keyOrder...)
	if len(keys) == 0 {
		for key := range state.keyed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
	}
	result := make([]Event, 0, len(state.keyed))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		stored, ok := state.keyed[key]
		if !ok {
			continue
		}
		result = append(result, stored.event)
		seen[key] = struct{}{}
	}
	if len(seen) != len(state.keyed) {
		remaining := make([]string, 0, len(state.keyed)-len(seen))
		for key := range state.keyed {
			if _, exists := seen[key]; !exists {
				remaining = append(remaining, key)
			}
		}
		sort.Strings(remaining)
		for _, key := range remaining {
			result = append(result, state.keyed[key].event)
		}
	}
	return result
}

func windowHistoryFromState(state *windowRuntimeState) []Event {
	if state == nil {
		return nil
	}
	return eventsFromStored(state.entries)
}

func windowHistoryByEvent(spec WindowSpec, state *windowRuntimeState) map[string][]Event {
	if state == nil {
		return nil
	}
	if window, ok := spec.(GroupWindowSpec); ok {
		result := make(map[string][]Event)
		keys := make([]string, 0, len(state.groups))
		for key := range state.groups {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := state.groups[key]
			history := windowHistory(window.Inner, child)
			for _, event := range history {
				result[eventIdentity(event)] = append([]Event(nil), history...)
			}
		}
		return result
	}
	history := windowHistory(spec, state)
	if len(history) == 0 {
		return nil
	}
	result := make(map[string][]Event, len(history))
	for _, event := range history {
		result[eventIdentity(event)] = append([]Event(nil), history...)
	}
	return result
}

func removeWindowKeyOrder(state *windowRuntimeState, key string) {
	if state == nil {
		return
	}
	for index, candidate := range state.keyOrder {
		if candidate == key {
			state.keyOrder = append(state.keyOrder[:index], state.keyOrder[index+1:]...)
			return
		}
	}
}

func windowPredicate(expression Expression[bool], entries []storedEvent, now time.Time, variables map[string]Value) bool {
	if expression == nil {
		return false
	}
	group := make([]Event, 0, len(entries))
	for _, stored := range entries {
		group = append(group, stored.event)
	}
	var current Event
	if len(group) > 0 {
		current = group[len(group)-1]
	}
	value := expression.eval(EvalContext{Event: current, Group: group, Now: now, Variables: variables})
	matched, ok := boolValue(value)
	return ok && matched
}

func eventTimestamp(expression Expr, event Event, now time.Time, variables map[string]Value) (time.Time, error) {
	value := expression.eval(EvalContext{Event: event, Now: now, Variables: variables})
	if !value.IsPresent() {
		return time.Time{}, fmt.Errorf("esper: external timestamp is missing or null")
	}
	if timestamp, ok := value.Any().(time.Time); ok {
		return timestamp, nil
	}
	if number, ok := numericValue(value); ok {
		return time.Unix(0, int64(number*float64(time.Millisecond))).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("esper: external timestamp has unsupported type %T", value.Any())
}

func timeOrderExpiry(window TimeOrderWindowSpec, receivedAt time.Time) time.Time {
	if window.CalendarYears != 0 || window.CalendarMonths != 0 || window.CalendarDays != 0 {
		return receivedAt.AddDate(window.CalendarYears, window.CalendarMonths, window.CalendarDays)
	}
	return receivedAt.Add(window.Duration)
}

func compareStoredEvents(left, right Event, keys []SortKey, now time.Time, variables map[string]Value) int {
	for _, key := range keys {
		comparison, ok := compareValues(key.Expr.eval(EvalContext{Event: left, Now: now, Variables: variables}), key.Expr.eval(EvalContext{Event: right, Now: now, Variables: variables}))
		if !ok || comparison == 0 {
			continue
		}
		if key.Descending {
			return -comparison
		}
		return comparison
	}
	return 0
}

func mergeDelta(left, right eventDelta) eventDelta {
	left.newEvents = append(left.newEvents, right.newEvents...)
	left.oldEvents = append(left.oldEvents, right.oldEvents...)
	if right.history != nil {
		left.history = append([]Event(nil), right.history...)
	}
	if len(right.historyByEvent) > 0 {
		if left.historyByEvent == nil {
			left.historyByEvent = make(map[string][]Event)
		}
		for key, history := range right.historyByEvent {
			left.historyByEvent[key] = append([]Event(nil), history...)
		}
	}
	if right.previousByEvent != nil {
		if left.previousByEvent == nil {
			left.previousByEvent = make(map[string][]Event)
		}
		for key, history := range right.previousByEvent {
			left.previousByEvent[key] = append([]Event(nil), history...)
		}
	}
	if right.priorByEvent != nil {
		if left.priorByEvent == nil {
			left.priorByEvent = make(map[string][]Event)
		}
		for key, history := range right.priorByEvent {
			left.priorByEvent[key] = append([]Event(nil), history...)
		}
	}
	return left
}

func newPatternProgress(node *patternNode) *patternProgress {
	if node == nil {
		return nil
	}
	progress := &patternProgress{
		node:                    node,
		minimum:                 node.minimum,
		maximum:                 node.maximum,
		boundsResolved:          !node.dynamicBounds || (node.minimumExpr == nil && node.maximumExpr == nil),
		sequenceMaximum:         node.sequenceMax,
		sequenceMaximumResolved: node.sequenceMaxExpr == nil,
	}
	switch node.kind {
	case patternSequenceNode, patternAndNode, patternOrNode:
		progress.left = newPatternProgress(node.left)
		progress.right = newPatternProgress(node.right)
	case patternNotNode:
		progress.child = newPatternProgress(node.child)
	case patternMatchUntilNode:
		progress.child = newPatternProgress(node.child)
		if node.right != nil {
			progress.right = newPatternProgress(node.right)
		}
	case patternWithinNode:
		progress.child = newPatternProgress(node.child)
	case patternUntilNode:
		progress.child = newPatternProgress(node.child)
		progress.right = newPatternProgress(node.right)
	case patternEveryNode:
		progress.child = newPatternProgress(node.child)
		if node.everyExpr != nil {
			progress.distinct = make(map[string]struct{})
			if node.distinctExpirySet {
				progress.distinctAt = make(map[string]time.Time)
			}
		}
	case patternTimerIntervalNode, patternTimerAtNode, patternTimerScheduleNode, patternTimerCronNode:
		// Timer observers have no event-local progress; the statement runtime
		// drives them from the virtual clock.
	}
	return progress
}

// patternCanStartWithoutEvent reports whether a pattern has an observer that
// can be armed when the statement/context starts. This is intentionally
// structural: an event-only branch must still wait for an event, while a
// timer branch in an And/Or expression can be active independently.
func patternCanStartWithoutEvent(node *patternNode) bool {
	if node == nil {
		return false
	}
	switch node.kind {
	case patternTimerIntervalNode, patternTimerAtNode, patternTimerScheduleNode, patternTimerCronNode:
		return true
	case patternSequenceNode:
		return patternCanStartWithoutEvent(node.left)
	case patternAndNode, patternOrNode:
		return patternCanStartWithoutEvent(node.left) || patternCanStartWithoutEvent(node.right)
	case patternNotNode, patternEveryNode:
		return patternCanStartWithoutEvent(node.child)
	case patternMatchUntilNode:
		return patternCanStartWithoutEvent(node.child) || patternCanStartWithoutEvent(node.right)
	case patternWithinNode:
		return true
	case patternUntilNode:
		return patternCanStartWithoutEvent(node.child) || patternCanStartWithoutEvent(node.right)
	default:
		return false
	}
}

// armPatternProgressTimers arms timer observers on the branch that is
// currently reachable. It is called when a match is created and whenever a
// sequence/repetition advances to a fresh child. Each pattern match owns the
// resulting schedule, which is what makes overlapping event starts safe.
func armPatternProgressTimers(progress *patternProgress, at time.Time, variables map[string]Value) {
	if progress == nil || progress.node == nil {
		return
	}
	switch progress.node.kind {
	case patternTimerIntervalNode:
		if !progress.timerStarted {
			progress.timerStarted = true
			deadline, ok := patternDurationDeadline(progress.node, progress, at, variables)
			if !ok {
				progress.expired = true
				return
			}
			progress.timerNext = deadline
		}
	case patternTimerAtNode:
		if !progress.timerStarted {
			progress.timerStarted = true
			progress.timerNext = progress.node.at
		}
	case patternTimerScheduleNode:
		if !progress.timerStarted {
			progress.timerStarted = true
			progress.scheduleIndex = 0
		}
	case patternTimerCronNode:
		if !progress.timerStarted {
			progress.timerStarted = true
			if progress.node.cron != nil {
				if resolved, err := progress.node.cron.resolve(EvalContext{Now: at, Variables: variables}); err == nil {
					progress.cronSchedule = resolved
					progress.cronNext, _ = resolved.nextAfter(at)
				}
			}
		}
	case patternSequenceNode:
		if progress.phase == 0 {
			inheritPatternProgressTags(progress, progress.left)
			armPatternProgressTimers(progress.left, at, variables)
		} else {
			inheritPatternProgressTags(progress, progress.right)
			armPatternProgressTimers(progress.right, at, variables)
		}
	case patternAndNode, patternOrNode:
		inheritPatternProgressTags(progress, progress.left)
		inheritPatternProgressTags(progress, progress.right)
		armPatternProgressTimers(progress.left, at, variables)
		armPatternProgressTimers(progress.right, at, variables)
	case patternNotNode, patternEveryNode:
		inheritPatternProgressTags(progress, progress.child)
		armPatternProgressTimers(progress.child, at, variables)
	case patternMatchUntilNode:
		inheritPatternProgressTags(progress, progress.child)
		armPatternProgressTimers(progress.child, at, variables)
		if progress.right != nil {
			inheritPatternProgressTags(progress, progress.right)
			armPatternProgressTimers(progress.right, at, variables)
		}
	case patternWithinNode:
		if !progress.timerStarted {
			progress.timerStarted = true
			deadline, ok := patternWithinDeadline(progress.node, progress, at, variables)
			if !ok {
				progress.expired = true
				return
			}
			progress.timerNext = deadline
		}
		inheritPatternProgressTags(progress, progress.child)
		armPatternProgressTimers(progress.child, at, variables)
	case patternUntilNode:
		inheritPatternProgressTags(progress, progress.child)
		inheritPatternProgressTags(progress, progress.right)
		armPatternProgressTimers(progress.child, at, variables)
		armPatternProgressTimers(progress.right, at, variables)
	}
}

func inheritPatternProgressTags(parent, child *patternProgress) {
	if parent == nil || child == nil {
		return
	}
	child.tags = mergePatternTags(parent.tags, child.tags)
	child.tagValues = mergePatternTagValues(parent.tagValues, child.tagValues)
}

func resolvePatternMatchUntilBounds(progress *patternProgress, trigger patternTrigger, variables map[string]Value) bool {
	if progress == nil || progress.node == nil || progress.node.kind != patternMatchUntilNode || progress.boundsResolved {
		return progress != nil && progress.boundsResolved
	}
	minimum := 0
	maximum := 0
	maximumPresent := false
	if progress.node.minimumExpr != nil {
		value := progress.node.minimumExpr.eval(EvalContext{
			Tags:       progress.tags,
			TagValues:  progress.tagValues,
			Now:        trigger.now,
			Variables:  variables,
			Parameters: parameterValuesFromVariables(variables),
		})
		if value.IsMissing() {
			return false
		}
		if value.IsNull() {
			value = Present(0)
		}
		resolved, err := As[int](value)
		if err != nil || !value.IsPresent() {
			return false
		}
		minimum = resolved
	}
	if progress.node.maximumExpr != nil {
		value := progress.node.maximumExpr.eval(EvalContext{
			Tags:       progress.tags,
			TagValues:  progress.tagValues,
			Now:        trigger.now,
			Variables:  variables,
			Parameters: parameterValuesFromVariables(variables),
		})
		if value.IsMissing() {
			return false
		}
		if value.IsNull() {
			value = Present(0)
		} else {
			maximumPresent = true
		}
		resolved, err := As[int](value)
		if err != nil || !value.IsPresent() {
			return false
		}
		maximum = resolved
	}
	if minimum < 0 || (maximumPresent && maximum <= 0) || (maximum > 0 && maximum < minimum) {
		return false
	}
	progress.minimum = minimum
	progress.maximum = maximum
	progress.boundsResolved = true
	return true
}

func resolvePatternSequenceMaximum(progress *patternProgress, trigger patternTrigger, variables map[string]Value) bool {
	if progress == nil || progress.node == nil || progress.node.kind != patternSequenceNode || progress.sequenceMaximumResolved {
		return progress != nil && progress.sequenceMaximumResolved
	}
	if progress.node.sequenceMaxExpr == nil {
		progress.sequenceMaximum = progress.node.sequenceMax
		progress.sequenceMaximumResolved = true
		return progress.sequenceMaximum > 0
	}
	value := progress.node.sequenceMaxExpr.eval(EvalContext{
		Event:      trigger.event,
		Tags:       progress.tags,
		TagValues:  progress.tagValues,
		Now:        trigger.now,
		Variables:  variables,
		Parameters: parameterValuesFromVariables(variables),
	})
	if !value.IsPresent() {
		return false
	}
	maximum, err := As[int](value)
	if err != nil || maximum <= 0 {
		return false
	}
	progress.sequenceMaximum = maximum
	progress.sequenceMaximumResolved = true
	return true
}

func patternDurationDeadline(node *patternNode, progress *patternProgress, at time.Time, variables map[string]Value) (time.Time, bool) {
	if node == nil {
		return time.Time{}, false
	}
	if node.durationExpr != nil {
		var tags map[string]Event
		var tagValues map[string][]Event
		if progress != nil {
			tags = progress.tags
			tagValues = progress.tagValues
		}
		value := node.durationExpr.eval(EvalContext{
			Tags:       tags,
			TagValues:  tagValues,
			Now:        at,
			Variables:  variables,
			Parameters: parameterValuesFromVariables(variables),
		})
		duration, ok := value.Any().(time.Duration)
		if !value.IsPresent() || !ok || duration <= 0 {
			return time.Time{}, false
		}
		return at.Add(duration), true
	}
	if node.calendar != nil {
		period := node.calendar
		return at.AddDate(period.Years, period.Months, period.Days), true
	}
	if node.duration <= 0 {
		return time.Time{}, false
	}
	return at.Add(node.duration), true
}

func patternWithinDeadline(node *patternNode, progress *patternProgress, at time.Time, variables map[string]Value) (time.Time, bool) {
	return patternDurationDeadline(node, progress, at, variables)
}

func patternTimerProgressDue(progress *patternProgress, now time.Time) bool {
	if progress == nil || progress.node == nil {
		return false
	}
	switch progress.node.kind {
	case patternTimerIntervalNode, patternTimerAtNode:
		return !progress.timerNext.IsZero() && !progress.timerNext.After(now)
	case patternTimerScheduleNode:
		return progress.scheduleIndex < len(progress.node.schedule) && !progress.node.schedule[progress.scheduleIndex].After(now)
	case patternTimerCronNode:
		return !progress.cronNext.IsZero() && !progress.cronNext.After(now)
	default:
		return false
	}
}

func clonePatternTags(tags map[string]Event) map[string]Event {
	if len(tags) == 0 {
		return nil
	}
	copyTags := make(map[string]Event, len(tags))
	for tag, event := range tags {
		copyTags[tag] = event
	}
	return copyTags
}

func clonePatternTagValues(values map[string][]Event) map[string][]Event {
	if len(values) == 0 {
		return nil
	}
	copyValues := make(map[string][]Event, len(values))
	for tag, events := range values {
		copyValues[tag] = append([]Event(nil), events...)
	}
	return copyValues
}

func mergePatternTags(parts ...map[string]Event) map[string]Event {
	var count int
	for _, part := range parts {
		count += len(part)
	}
	if count == 0 {
		return nil
	}
	merged := make(map[string]Event, count)
	for _, part := range parts {
		for tag, event := range part {
			merged[tag] = event
		}
	}
	return merged
}

func mergePatternTagValues(parts ...map[string][]Event) map[string][]Event {
	var count int
	for _, part := range parts {
		count += len(part)
	}
	if count == 0 {
		return nil
	}
	merged := make(map[string][]Event, count)
	for _, part := range parts {
		for tag, events := range part {
			merged[tag] = append(merged[tag], events...)
		}
	}
	return merged
}

func clonePatternProgress(progress *patternProgress) *patternProgress {
	if progress == nil {
		return nil
	}
	copyProgress := *progress
	copyProgress.left = clonePatternProgress(progress.left)
	copyProgress.right = clonePatternProgress(progress.right)
	copyProgress.child = clonePatternProgress(progress.child)
	copyProgress.tags = clonePatternTags(progress.tags)
	copyProgress.tagValues = clonePatternTagValues(progress.tagValues)
	if len(progress.distinct) > 0 {
		copyProgress.distinct = make(map[string]struct{}, len(progress.distinct))
		for key := range progress.distinct {
			copyProgress.distinct[key] = struct{}{}
		}
	}
	if len(progress.distinctAt) > 0 {
		copyProgress.distinctAt = make(map[string]time.Time, len(progress.distinctAt))
		for key, startedAt := range progress.distinctAt {
			copyProgress.distinctAt[key] = startedAt
		}
	}
	return &copyProgress
}

func patternProgressActive(progress *patternProgress) bool {
	if progress == nil || progress.expired {
		return false
	}
	switch progress.node.kind {
	case patternEventNode:
		return progress.started
	case patternSequenceNode:
		return progress.phase > 0 || patternProgressActive(progress.left) || patternProgressActive(progress.right)
	case patternAndNode, patternOrNode:
		return patternProgressActive(progress.left) || patternProgressActive(progress.right)
	case patternNotNode:
		// A negative branch is meaningful even before its child has consumed an
		// event. Once the forbidden child matches, however, the negative branch
		// is terminal and must release its enclosing partial match immediately.
		return !progress.blocked
	case patternMatchUntilNode:
		return progress.count > 0 || patternProgressActive(progress.child) || patternProgressActive(progress.right)
	case patternUntilNode:
		return progress.count > 0 || patternProgressActive(progress.child) || patternProgressActive(progress.right)
	case patternEveryNode:
		return progress.done || patternProgressActive(progress.child)
	case patternWithinNode:
		return progress.timerStarted && !progress.expired && !progress.done
	case patternTimerIntervalNode, patternTimerAtNode, patternTimerScheduleNode, patternTimerCronNode:
		return progress.timerStarted && !progress.done && !progress.expired
	default:
		return false
	}
}

func patternSatisfied(progress *patternProgress) bool {
	if progress == nil || progress.expired {
		return false
	}
	if progress.done {
		return true
	}
	switch progress.node.kind {
	case patternEventNode:
		return progress.started
	case patternSequenceNode:
		return progress.phase >= 2
	case patternAndNode:
		return patternSatisfied(progress.left) && patternSatisfied(progress.right)
	case patternOrNode:
		return patternSatisfied(progress.left) || patternSatisfied(progress.right)
	case patternNotNode:
		return !progress.blocked
	case patternMatchUntilNode:
		if progress.node.right != nil {
			return progress.done
		}
		if progress.node.dynamicBounds && progress.count == 0 && !progress.started {
			return false
		}
		return progress.count >= progress.minimum
	case patternUntilNode:
		return progress.done
	case patternEveryNode:
		return progress.done
	case patternWithinNode:
		if progress.expired {
			return false
		}
		return patternSatisfied(progress.child)
	case patternTimerIntervalNode, patternTimerAtNode, patternTimerScheduleNode, patternTimerCronNode:
		return progress.done
	default:
		return false
	}
}

// patternWithinCanContinue identifies a timer guard that emitted one match
// from an Every child but still has capacity for later matches. Most pattern
// completions are terminal to their parent; this is the small exception that
// makes WithinOrMax useful with a repeatable branch without changing the
// existing transition contract.
func patternWithinCanContinue(progress *patternProgress) bool {
	if progress == nil || progress.node == nil || progress.node.kind != patternWithinNode || progress.expired || progress.done {
		return false
	}
	if progress.node.maximum > 0 && progress.count >= progress.node.maximum {
		return false
	}
	return progress.child != nil && progress.child.node != nil && progress.child.node.kind == patternEveryNode
}

// patternEveryCanContinue identifies a materialized Every node that must stay
// resident after emitting a match. Definition-level Every uses the legacy
// definition flag and starts a fresh root on the next event; an AST Every
// node, such as Within().Every(), owns its distinct set and therefore has to
// keep the progress object alive across completions.
func patternEveryCanContinue(progress *patternProgress) bool {
	if progress == nil || progress.node == nil || progress.node.kind != patternEveryNode || progress.expired {
		return false
	}
	return progress.done || patternProgressActive(progress.child)
}

func expirePatternProgressDistinct(progress *patternProgress, now time.Time) {
	if progress == nil || progress.node == nil || !progress.node.distinctExpirySet || progress.node.distinctExpiry <= 0 || len(progress.distinctAt) == 0 {
		return
	}
	for key, startedAt := range progress.distinctAt {
		if !startedAt.Add(progress.node.distinctExpiry).After(now) {
			delete(progress.distinctAt, key)
			delete(progress.distinct, key)
		}
	}
}

func recordPatternProgressDistinct(progress *patternProgress, key string, now time.Time) {
	if progress == nil {
		return
	}
	if progress.distinct == nil {
		progress.distinct = make(map[string]struct{})
	}
	progress.distinct[key] = struct{}{}
	if progress.node != nil && progress.node.distinctExpirySet {
		if progress.distinctAt == nil {
			progress.distinctAt = make(map[string]time.Time)
		}
		progress.distinctAt[key] = now
	}
}

func patternCanContinueAfterMatch(progress *patternProgress) bool {
	return patternWithinCanContinue(progress) || patternEveryCanContinue(progress)
}

// patternMatchWithinLimits applies both the legacy whole-pattern MaxStates
// limit and the per-edge FollowedByMax limit. The latter counts only active
// sequence progress that has consumed its left side and is waiting on the
// right side. Counting the materialized progress tree keeps nested and
// composed followed-by edges independent, matching Esper's subexpression
// scope instead of treating every active root as one shared bucket.
func patternMatchWithinLimits(active []patternMatch, candidate patternMatch, definition *patternDefinition) bool {
	if definition == nil {
		return true
	}
	if definition.maxStates > 0 && len(active) >= definition.maxStates {
		return false
	}
	counts := make(map[*patternNode]patternSequenceMaxCount)
	for _, match := range active {
		addPatternSequenceMaxCounts(counts, match.state)
	}
	addPatternSequenceMaxCounts(counts, candidate.state)
	for _, count := range counts {
		if count.maximum <= 0 || count.count > count.maximum {
			return false
		}
	}
	return true
}

type patternSequenceMaxCount struct {
	count   int
	maximum int
}

func addPatternSequenceMaxCounts(counts map[*patternNode]patternSequenceMaxCount, progress *patternProgress) {
	if progress == nil || progress.node == nil {
		return
	}
	if progress.node.kind == patternSequenceNode && progress.node.sequenceMaxSet && progress.phase == 1 && !progress.done && !progress.expired {
		if !progress.sequenceMaximumResolved {
			counts[progress.node] = patternSequenceMaxCount{maximum: 0}
		} else {
			count := counts[progress.node]
			count.count++
			if count.maximum == 0 || progress.sequenceMaximum < count.maximum {
				count.maximum = progress.sequenceMaximum
			}
			counts[progress.node] = count
		}
	}
	addPatternSequenceMaxCounts(counts, progress.left)
	addPatternSequenceMaxCounts(counts, progress.right)
	addPatternSequenceMaxCounts(counts, progress.child)
}

func patternWithinTerminal(progress *patternProgress) bool {
	if progress == nil || progress.node == nil || progress.node.kind != patternWithinNode {
		return false
	}
	return progress.expired || (progress.done && progress.node.maximum > 0 && progress.count >= progress.node.maximum)
}

func patternProgressTerminal(progress *patternProgress) bool {
	if progress == nil {
		return false
	}
	if progress.node != nil && progress.node.kind == patternNotNode && progress.blocked {
		return true
	}
	return progress.expired || patternWithinTerminal(progress)
}

func patternPredicateMatches(node *patternNode, progress *patternProgress, event Event, now time.Time, variables map[string]Value) bool {
	if node == nil || node.predicate == nil {
		return false
	}
	ctx := EvalContext{Event: event, Tags: progress.tags, TagValues: progress.tagValues, Now: now, Variables: variables}
	value := node.predicate.eval(ctx)
	ok, isBool := boolValue(value)
	return isBool && ok
}

func capturePatternEvent(progress *patternProgress, event Event) {
	progress.started = true
	progress.done = true
	if progress.tags == nil {
		progress.tags = make(map[string]Event)
	}
	progress.tags[progress.node.tag] = event
	if progress.tagValues == nil {
		progress.tagValues = make(map[string][]Event)
	}
	progress.tagValues[progress.node.tag] = append(progress.tagValues[progress.node.tag], event)
}

func patternTransitionFor(progress *patternProgress) patternTransition {
	complete := patternSatisfied(progress)
	if progress != nil && progress.node != nil && progress.node.kind == patternNotNode {
		complete = false
	}
	return patternTransition{state: progress, complete: complete}
}

func patternTransitionFrom(progress *patternProgress, complete bool, sources ...patternTransition) patternTransition {
	transition := patternTransition{state: progress, complete: complete}
	for _, source := range sources {
		if !source.consumed {
			continue
		}
		if !transition.consumed || source.consumptionLevel > transition.consumptionLevel {
			transition.consumptionLevel = source.consumptionLevel
		}
		transition.consumed = true
	}
	return transition
}

func advancePatternNode(progress *patternProgress, event Event, now time.Time, variables map[string]Value) []patternTransition {
	return advancePatternNodeTrigger(progress, patternTrigger{event: event, now: now, consumptionLevel: -1}, variables)
}

func advancePatternNodeTime(progress *patternProgress, now time.Time, variables map[string]Value) []patternTransition {
	return advancePatternNodeTrigger(progress, patternTrigger{now: now, isTimer: true, consumptionLevel: -1}, variables)
}

func advancePatternNodeTrigger(progress *patternProgress, trigger patternTrigger, variables map[string]Value) []patternTransition {
	if progress == nil || progress.node == nil {
		return nil
	}
	switch progress.node.kind {
	case patternEventNode:
		next := clonePatternProgress(progress)
		if trigger.isTimer {
			return []patternTransition{{state: next, complete: patternSatisfied(next)}}
		}
		if next.done {
			return []patternTransition{patternTransitionFor(next)}
		}
		if !patternPredicateMatches(next.node, next, trigger.event, trigger.now, variables) {
			return []patternTransition{patternTransitionFor(next)}
		}
		if trigger.consumptionLevel >= 0 {
			// Once any consuming filter matched this event, Esper dispatches
			// only consuming filters at the highest level. An unannotated
			// filter therefore remains at its pre-event state even though its
			// own predicate may match.
			if !next.node.consumeLevelSet || next.node.consumeLevel < trigger.consumptionLevel {
				return []patternTransition{patternTransitionFor(next)}
			}
		}
		capturePatternEvent(next, trigger.event)
		transition := patternTransition{
			state:    next,
			complete: patternSatisfied(next),
		}
		if next.node.consumeLevelSet {
			transition.consumed = true
			transition.consumptionLevel = next.node.consumeLevel
		}
		return []patternTransition{transition}

	case patternTimerIntervalNode, patternTimerAtNode, patternTimerScheduleNode, patternTimerCronNode:
		next := clonePatternProgress(progress)
		if !next.timerStarted {
			armPatternProgressTimers(next, trigger.now, variables)
		}
		if !trigger.isTimer || next.timerEmitted {
			return []patternTransition{{state: next, complete: patternSatisfied(next)}}
		}
		if !patternTimerProgressDue(next, trigger.now) {
			return []patternTransition{{state: next}}
		}
		next.timerEmitted = true
		next.done = true
		next.started = true
		return []patternTransition{patternTransitionFor(next)}

	case patternSequenceNode:
		if progress.phase >= 2 || progress.done {
			next := clonePatternProgress(progress)
			return []patternTransition{patternTransitionFor(next)}
		}
		if progress.phase == 0 {
			leftTransitions := advancePatternNodeTrigger(progress.left, trigger, variables)
			result := make([]patternTransition, 0, len(leftTransitions))
			for _, leftTransition := range leftTransitions {
				next := clonePatternProgress(progress)
				next.left = leftTransition.state
				next.tags = clonePatternTags(leftTransition.state.tags)
				next.tagValues = clonePatternTagValues(leftTransition.state.tagValues)
				next.started = patternProgressActive(leftTransition.state)
				if patternProgressTerminal(leftTransition.state) && !patternSatisfied(leftTransition.state) {
					next.expired = true
				}
				if patternSatisfied(leftTransition.state) {
					if next.node.sequenceMaxExpr != nil && !resolvePatternSequenceMaximum(next, trigger, variables) {
						next.expired = true
						result = append(result, patternTransitionFrom(next, false, leftTransition))
						continue
					}
					next.phase = 1
					next.right = newPatternProgress(progress.node.right)
					inheritPatternProgressTags(next, next.right)
					armPatternProgressTimers(next.right, trigger.now, variables)
					next.started = true
				}
				result = append(result, patternTransitionFrom(next, patternSatisfied(next), leftTransition))
			}
			return result
		}

		rightTransitions := advancePatternNodeTrigger(progress.right, trigger, variables)
		result := make([]patternTransition, 0, len(rightTransitions))
		for _, rightTransition := range rightTransitions {
			next := clonePatternProgress(progress)
			next.right = rightTransition.state
			next.tags = mergePatternTags(progress.left.tags, rightTransition.state.tags)
			next.tagValues = mergePatternTagValues(progress.left.tagValues, rightTransition.state.tagValues)
			next.started = true
			if patternProgressTerminal(rightTransition.state) && !patternSatisfied(rightTransition.state) {
				next.expired = true
			}
			if patternSatisfied(rightTransition.state) {
				next.phase = 2
				next.done = true
			}
			result = append(result, patternTransitionFrom(next, patternSatisfied(next), rightTransition))
		}
		return result

	case patternAndNode:
		leftTransitions := advancePatternNodeTrigger(progress.left, trigger, variables)
		rightTransitions := advancePatternNodeTrigger(progress.right, trigger, variables)
		result := make([]patternTransition, 0, len(leftTransitions)*len(rightTransitions))
		for _, leftTransition := range leftTransitions {
			for _, rightTransition := range rightTransitions {
				next := clonePatternProgress(progress)
				next.left = leftTransition.state
				next.right = rightTransition.state
				next.tags = mergePatternTags(leftTransition.state.tags, rightTransition.state.tags)
				next.tagValues = mergePatternTagValues(leftTransition.state.tagValues, rightTransition.state.tagValues)
				next.started = patternProgressActive(next.left) || patternProgressActive(next.right)
				if (patternProgressTerminal(leftTransition.state) && !patternSatisfied(leftTransition.state)) || (patternProgressTerminal(rightTransition.state) && !patternSatisfied(rightTransition.state)) {
					next.expired = true
				}
				next.done = patternSatisfied(next.left) && patternSatisfied(next.right)
				result = append(result, patternTransitionFrom(next, patternSatisfied(next), leftTransition, rightTransition))
			}
		}
		return result

	case patternOrNode:
		leftTransitions := advancePatternNodeTrigger(progress.left, trigger, variables)
		rightTransitions := advancePatternNodeTrigger(progress.right, trigger, variables)
		result := make([]patternTransition, 0, len(leftTransitions)+len(rightTransitions))
		leftMatched := false
		rightMatched := false
		for _, transition := range leftTransitions {
			leftMatched = leftMatched || transition.consumed
		}
		for _, transition := range rightTransitions {
			rightMatched = rightMatched || transition.consumed
		}
		if leftMatched || rightMatched {
			// An Or is a fan-out of independent alternatives. Advancing both
			// sides and merging their captures would turn two same-event
			// callbacks into one row and would make @consume(N) unable to
			// suppress the lower-priority alternative. Keep the other side at
			// its pre-event state for each branch that actually matched.
			for _, leftTransition := range leftTransitions {
				if !leftTransition.consumed {
					continue
				}
				next := clonePatternProgress(progress)
				next.left = leftTransition.state
				next.right = clonePatternProgress(progress.right)
				next.tags = mergePatternTags(leftTransition.state.tags, next.right.tags)
				next.tagValues = mergePatternTagValues(leftTransition.state.tagValues, next.right.tagValues)
				next.started = patternProgressActive(next.left) || patternProgressActive(next.right)
				next.done = patternSatisfied(next.left) || patternSatisfied(next.right)
				if !next.done && patternProgressTerminal(leftTransition.state) && patternProgressTerminal(next.right) {
					next.expired = true
				}
				result = append(result, patternTransitionFrom(next, patternSatisfied(next), leftTransition))
			}
			for _, rightTransition := range rightTransitions {
				if !rightTransition.consumed {
					continue
				}
				next := clonePatternProgress(progress)
				next.left = clonePatternProgress(progress.left)
				next.right = rightTransition.state
				next.tags = mergePatternTags(next.left.tags, rightTransition.state.tags)
				next.tagValues = mergePatternTagValues(next.left.tagValues, rightTransition.state.tagValues)
				next.started = patternProgressActive(next.left) || patternProgressActive(next.right)
				next.done = patternSatisfied(next.left) || patternSatisfied(next.right)
				if !next.done && patternProgressTerminal(next.left) && patternProgressTerminal(rightTransition.state) {
					next.expired = true
				}
				result = append(result, patternTransitionFrom(next, patternSatisfied(next), rightTransition))
			}
			return result
		}
		// No branch consumed the event. Preserve the existing product of
		// no-op transitions so nested observers and negative branches keep
		// their current state.
		for _, leftTransition := range leftTransitions {
			for _, rightTransition := range rightTransitions {
				next := clonePatternProgress(progress)
				next.left = leftTransition.state
				next.right = rightTransition.state
				next.tags = mergePatternTags(leftTransition.state.tags, rightTransition.state.tags)
				next.tagValues = mergePatternTagValues(leftTransition.state.tagValues, rightTransition.state.tagValues)
				next.started = patternProgressActive(next.left) || patternProgressActive(next.right)
				next.done = patternSatisfied(next.left) || patternSatisfied(next.right)
				if !next.done && patternProgressTerminal(leftTransition.state) && patternProgressTerminal(rightTransition.state) {
					next.expired = true
				}
				result = append(result, patternTransitionFrom(next, patternSatisfied(next), leftTransition, rightTransition))
			}
		}
		return result

	case patternNotNode:
		if progress.blocked {
			next := clonePatternProgress(progress)
			return []patternTransition{{state: next}}
		}
		childTransitions := advancePatternNodeTrigger(progress.child, trigger, variables)
		result := make([]patternTransition, 0, len(childTransitions))
		for _, childTransition := range childTransitions {
			next := clonePatternProgress(progress)
			next.child = childTransition.state
			next.blocked = patternSatisfied(childTransition.state)
			next.started = patternProgressActive(childTransition.state)
			// A negative branch never contributes positive captures to its parent.
			next.tags = nil
			next.tagValues = nil
			result = append(result, patternTransitionFrom(next, patternTransitionFor(next).complete, childTransition))
		}
		return result

	case patternMatchUntilNode:
		base := clonePatternProgress(progress)
		if !resolvePatternMatchUntilBounds(base, trigger, variables) {
			base.expired = true
			base.child = nil
			return []patternTransition{{state: base, complete: false}}
		}
		if base.node.right != nil {
			childTransitions := make([]patternTransition, 0, 1)
			if base.child == nil || (base.maximum > 0 && base.count >= base.maximum) {
				childTransitions = append(childTransitions, patternTransition{state: base.child})
			} else {
				childTransitions = advancePatternNodeTrigger(base.child, trigger, variables)
			}
			terminatorTransitions := advancePatternNodeTrigger(base.right, trigger, variables)
			terminatorMatched := false
			for _, terminatorTransition := range terminatorTransitions {
				if terminatorTransition.complete {
					terminatorMatched = true
					break
				}
			}
			if terminatorMatched {
				result := make([]patternTransition, 0, len(terminatorTransitions))
				for _, terminatorTransition := range terminatorTransitions {
					if !terminatorTransition.complete {
						continue
					}
					next := clonePatternProgress(base)
					next.right = terminatorTransition.state
					next.tags = mergePatternTags(base.tags, terminatorTransition.state.tags)
					next.tagValues = mergePatternTagValues(base.tagValues, terminatorTransition.state.tagValues)
					next.started = true
					if next.count >= next.minimum {
						next.done = true
						next.child = nil
						result = append(result, patternTransitionFrom(next, true, terminatorTransition))
					} else {
						next.expired = true
						next.child = nil
						result = append(result, patternTransitionFrom(next, false, terminatorTransition))
					}
				}
				return result
			}
			result := make([]patternTransition, 0, len(childTransitions)*len(terminatorTransitions))
			for _, childTransition := range childTransitions {
				for _, terminatorTransition := range terminatorTransitions {
					next := clonePatternProgress(base)
					next.child = childTransition.state
					next.right = terminatorTransition.state
					var childTags map[string]Event
					var childTagValues map[string][]Event
					if childTransition.state != nil {
						childTags = childTransition.state.tags
						childTagValues = childTransition.state.tagValues
					}
					next.tags = mergePatternTags(base.tags, childTags, terminatorTransition.state.tags)
					next.tagValues = mergePatternTagValues(base.tagValues, childTagValues, terminatorTransition.state.tagValues)
					next.count = base.count
					next.started = base.count > 0 || patternProgressActive(childTransition.state) || patternProgressActive(terminatorTransition.state)
					if patternProgressTerminal(childTransition.state) && !childTransition.complete && base.child != nil {
						next.expired = true
					}
					if childTransition.complete {
						next.count++
						if next.maximum > 0 && next.count >= next.maximum {
							next.child = nil
						} else {
							next.child = newPatternProgress(base.node.child)
							armPatternProgressTimers(next.child, trigger.now, variables)
						}
					}
					result = append(result, patternTransitionFrom(next, false, childTransition, terminatorTransition))
				}
			}
			return result
		}
		childTransitions := advancePatternNodeTrigger(base.child, trigger, variables)
		result := make([]patternTransition, 0, len(childTransitions))
		for _, childTransition := range childTransitions {
			next := clonePatternProgress(base)
			next.child = childTransition.state
			next.tags = mergePatternTags(progress.tags, childTransition.state.tags)
			next.tagValues = mergePatternTagValues(progress.tagValues, childTransition.state.tagValues)
			next.count = progress.count
			next.started = progress.count > 0 || patternProgressActive(childTransition.state)
			if patternProgressTerminal(childTransition.state) && !childTransition.complete && next.count < next.minimum {
				next.expired = true
			}
			if childTransition.complete {
				next.count++
				if next.count >= next.minimum || (next.maximum > 0 && next.count >= next.maximum) {
					next.done = true
					next.child = nil
				} else {
					next.child = newPatternProgress(progress.node.child)
					armPatternProgressTimers(next.child, trigger.now, variables)
				}
			}
			result = append(result, patternTransitionFrom(next, patternSatisfied(next), childTransition))
		}
		return result

	case patternUntilNode:
		childTransitions := advancePatternNodeTrigger(progress.child, trigger, variables)
		terminatorTransitions := advancePatternNodeTrigger(progress.right, trigger, variables)
		result := make([]patternTransition, 0, len(childTransitions)*len(terminatorTransitions))
		for _, childTransition := range childTransitions {
			for _, terminatorTransition := range terminatorTransitions {
				next := clonePatternProgress(progress)
				next.child = childTransition.state
				next.right = terminatorTransition.state
				next.tags = mergePatternTags(progress.tags, childTransition.state.tags, terminatorTransition.state.tags)
				next.tagValues = mergePatternTagValues(progress.tagValues, childTransition.state.tagValues, terminatorTransition.state.tagValues)
				next.count = progress.count
				next.started = progress.count > 0 || patternProgressActive(childTransition.state) || patternProgressActive(terminatorTransition.state)
				if childTransition.complete {
					next.count++
					next.child = newPatternProgress(progress.node.child)
					armPatternProgressTimers(next.child, trigger.now, variables)
					next.started = true
				}
				if terminatorTransition.complete {
					next.done = true
				}
				result = append(result, patternTransitionFrom(next, patternSatisfied(next), childTransition, terminatorTransition))
			}
		}
		return result

	case patternEveryNode:
		// A completed Every node already installed its next child when the
		// previous completion was processed. Re-creating it here would reset
		// an inner timer guard on every virtual-clock callback and could keep
		// Within().Every() alive forever.
		base := clonePatternProgress(progress)
		expirePatternProgressDistinct(base, trigger.now)
		child := base.child
		childTransitions := advancePatternNodeTrigger(child, trigger, variables)
		result := make([]patternTransition, 0, len(childTransitions))
		for _, childTransition := range childTransitions {
			next := clonePatternProgress(base)
			next.child = childTransition.state
			next.started = true
			fired := childTransition.complete
			if patternProgressTerminal(childTransition.state) && !childTransition.complete {
				next.expired = true
			}
			if childTransition.complete {
				if next.node.everyExpr != nil {
					keyValue := next.node.everyExpr.eval(EvalContext{Event: trigger.event, Tags: childTransition.state.tags, TagValues: childTransition.state.tagValues, Now: trigger.now, Variables: variables})
					key := encodeKey([]any{keyValue.State(), keyValue.Any()})
					if _, exists := next.distinct[key]; exists {
						fired = false
					} else {
						recordPatternProgressDistinct(next, key, trigger.now)
					}
				}
				if fired {
					next.tags = clonePatternTags(childTransition.state.tags)
					next.tagValues = clonePatternTagValues(childTransition.state.tagValues)
					next.done = true
				}
				next.child = newPatternProgress(next.node.child)
				inheritPatternProgressTags(next, next.child)
				armPatternProgressTimers(next.child, trigger.now, variables)
			}
			result = append(result, patternTransitionFrom(next, fired, childTransition))
		}
		return result

	case patternWithinNode:
		next := clonePatternProgress(progress)
		if next.expired || next.done {
			return []patternTransition{{state: next, complete: false}}
		}
		if !next.timerStarted {
			armPatternProgressTimers(next, trigger.now, variables)
		}
		if next.expired {
			return []patternTransition{{state: next, complete: false}}
		}
		// The guard wins at the exact boundary. An event delivered after the
		// virtual clock has reached the deadline cannot complete the child.
		if !next.timerNext.IsZero() && !next.timerNext.After(trigger.now) {
			next.expired = true
			next.child = nil
			return []patternTransition{{state: next, complete: false}}
		}
		childTransitions := advancePatternNodeTrigger(next.child, trigger, variables)
		result := make([]patternTransition, 0, len(childTransitions))
		for _, childTransition := range childTransitions {
			if childTransition.state == nil {
				continue
			}
			candidate := clonePatternProgress(next)
			candidate.child = childTransition.state
			candidate.tags = clonePatternTags(childTransition.state.tags)
			candidate.tagValues = clonePatternTagValues(childTransition.state.tagValues)
			candidate.started = true
			if childTransition.complete {
				candidate.count++
				if candidate.node.maximum == 0 {
					candidate.expired = true
					candidate.child = nil
					result = append(result, patternTransitionFrom(candidate, false, childTransition))
					continue
				}
				if candidate.node.maximum > 0 && candidate.count >= candidate.node.maximum {
					candidate.done = true
				}
			}
			result = append(result, patternTransitionFrom(candidate, childTransition.complete && patternSatisfied(candidate), childTransition))
		}
		return result
	default:
		return nil
	}
}

func (r *statementRuntime) patternBatch(delta eventDelta, plan Plan, now time.Time) ResultBatch {
	if plan.query.pattern == nil || r.patternState == nil {
		return ResultBatch{}
	}
	definition := plan.query.pattern
	if isPatternTimerRoot(definition) {
		return ResultBatch{}
	}
	if plan.query.suppressOverlappingMatches {
		// Esper's PatternRemoveDispatchView suppresses overlaps within the
		// result batch being dispatched. A later event starts a fresh batch and
		// may therefore reuse an Event that appeared in an earlier batch.
		r.patternState.emittedEvents = nil
	}
	batch := ResultBatch{Time: now}
	batch.outputCountsSet = true
	batch.outputInserted = int64(len(delta.newEvents))
	batch.outputRemoved = int64(len(delta.oldEvents))
	hasConsumption := patternHasConsumption(definition.root)
	for _, event := range delta.newEvents {
		if r.patternState.patternStopped {
			continue
		}
		if !patternGuardAllows(definition, event, now, r.variables) {
			r.patternState.active = nil
			continue
		}
		r.patternExpire(definition, now)
		matches := r.patternState.active
		startAllowed := definition.every || len(matches) == 0
		consumptionLevel := -1
		if hasConsumption {
			probe := patternTrigger{event: event, now: now, consumptionLevel: -1}
			for _, match := range matches {
				for _, transition := range advancePatternNodeTrigger(match.state, probe, r.variables) {
					if transition.consumed && transition.consumptionLevel > consumptionLevel {
						consumptionLevel = transition.consumptionLevel
					}
				}
			}
			if startAllowed && !plan.query.discardPartialsOnMatch {
				progress := newPatternProgress(definition.root)
				armPatternProgressTimers(progress, now, r.variables)
				for _, transition := range advancePatternNodeTrigger(progress, probe, r.variables) {
					if transition.consumed && transition.consumptionLevel > consumptionLevel {
						consumptionLevel = transition.consumptionLevel
					}
				}
			}
		}
		trigger := patternTrigger{event: event, now: now, consumptionLevel: consumptionLevel}
		nextActive := make([]patternMatch, 0, len(matches)+1)
		terminal := false
		completed := false
		for _, match := range matches {
			transitions := advancePatternNodeTrigger(match.state, trigger, r.variables)
			for _, transition := range transitions {
				if transition.state == nil {
					continue
				}
				candidate := patternMatch{
					state:     transition.state,
					tags:      clonePatternTags(transition.state.tags),
					tagValues: clonePatternTagValues(transition.state.tagValues),
					current:   event,
					startedAt: match.startedAt,
				}
				if transition.complete {
					completed = true
					if row, visible := evaluatePatternMatch(definition, candidate, plan, now, r.variables); visible && r.patternState.acceptPatternMatch(plan.query, candidate) {
						batch.New = append(batch.New, resultRow(row))
					}
					if patternCanContinueAfterMatch(transition.state) && patternMatchWithinLimits(nextActive, candidate, definition) {
						nextActive = append(nextActive, candidate)
					}
					if patternWithinTerminal(transition.state) {
						terminal = true
					}
					continue
				}
				if patternProgressTerminal(transition.state) {
					terminal = true
				}
				if patternProgressActive(transition.state) && patternMatchWithinLimits(nextActive, candidate, definition) {
					nextActive = append(nextActive, candidate)
				}
			}
		}

		if startAllowed && !(plan.query.discardPartialsOnMatch && completed) {
			progress := newPatternProgress(definition.root)
			armPatternProgressTimers(progress, now, r.variables)
			starts := advancePatternNodeTrigger(progress, trigger, r.variables)
			for _, transition := range starts {
				if patternProgressTerminal(transition.state) {
					terminal = true
				}
				if transition.state == nil || (!transition.complete && !patternProgressActive(transition.state)) {
					continue
				}
				if definition.everyDistinct != nil {
					keyValue := definition.everyDistinct.eval(EvalContext{Event: event, Now: now, Variables: r.variables})
					key := encodeKey([]any{keyValue.State(), keyValue.Any()})
					if _, exists := r.patternState.distinct[key]; exists {
						continue
					}
					recordPatternDistinct(r.patternState, definition, key, now)
				}
				started := patternMatch{
					state:     transition.state,
					tags:      clonePatternTags(transition.state.tags),
					tagValues: clonePatternTagValues(transition.state.tagValues),
					current:   event,
					startedAt: now,
				}
				if transition.complete {
					completed = true
					if row, visible := evaluatePatternMatch(definition, started, plan, now, r.variables); visible && r.patternState.acceptPatternMatch(plan.query, started) {
						batch.New = append(batch.New, resultRow(row))
					}
					if patternCanContinueAfterMatch(transition.state) && patternMatchWithinLimits(nextActive, started, definition) {
						nextActive = append(nextActive, started)
					}
					if patternWithinTerminal(transition.state) {
						terminal = true
					}
				} else if patternMatchWithinLimits(nextActive, started, definition) {
					nextActive = append(nextActive, started)
					if patternProgressTerminal(transition.state) {
						terminal = true
					}
				}
			}
		}
		if plan.query.discardPartialsOnMatch && completed {
			nextActive = nil
		}
		r.patternState.active = nextActive
		if terminal && !definition.every && len(nextActive) == 0 {
			r.patternState.patternStopped = true
		}
		if definition.root != nil && definition.root.kind == patternWithinNode && (terminal || (len(nextActive) == 0 && completed)) {
			r.patternState.patternStopped = true
		}
	}
	if !batch.empty() {
		if plan.query.distinct {
			batch.New, batch.Old = r.applyDistinct(plan.query, batch.New, batch.Old)
		}
		batch.New = applyResultWindow(batch.New, plan.query)
		batch.Old = applyResultWindow(batch.Old, plan.query)
		batch.Sequence = r.seq.Add(1)
	}
	return batch
}

func isPatternTimerRoot(definition *patternDefinition) bool {
	if definition == nil || definition.root == nil {
		return false
	}
	return definition.root.kind == patternTimerIntervalNode || definition.root.kind == patternTimerAtNode || definition.root.kind == patternTimerScheduleNode || definition.root.kind == patternTimerCronNode
}

func (r *statementRuntime) patternTimeBatch(plan Plan, now time.Time) ResultBatch {
	if plan.query.pattern == nil || r.patternState == nil {
		return ResultBatch{}
	}
	if !isPatternTimerRoot(plan.query.pattern) {
		if !patternContainsTimer(plan.query.pattern.root) {
			return ResultBatch{}
		}
		return r.patternCompositeTimeBatch(plan, now)
	}
	if !r.patternState.timerStarted {
		r.initializeAt(now)
	}
	batch := ResultBatch{Time: now}
	root := plan.query.pattern.root
	switch root.kind {
	case patternTimerIntervalNode:
		// A large clock jump may make multiple interval callbacks due. Emit one
		// result per due callback, but cap a malformed/hostile jump so a timer
		// cannot turn into an unbounded allocation.
		const maxTimerCatchUp = 100000
		for emitted := 0; emitted < maxTimerCatchUp && !r.patternState.timerNext.IsZero() && !r.patternState.timerNext.After(now); emitted++ {
			dueAt := r.patternState.timerNext
			match := patternMatch{current: Event{}, startedAt: dueAt}
			if patternGuardAllows(plan.query.pattern, Event{}, dueAt, r.variables) {
				if row, visible := evaluatePatternMatch(plan.query.pattern, match, plan, dueAt, r.variables); visible {
					batch.New = append(batch.New, resultRow(row))
				}
			}
			next, ok := patternDurationDeadline(root, nil, dueAt, r.variables)
			if !ok {
				r.patternState.patternStopped = true
				r.patternState.timerNext = time.Time{}
				break
			}
			r.patternState.timerNext = next
		}
	case patternTimerAtNode:
		if !r.patternState.timerEmitted && !now.Before(r.patternState.timerNext) {
			match := patternMatch{current: Event{}, startedAt: r.patternState.timerNext}
			if patternGuardAllows(plan.query.pattern, Event{}, now, r.variables) {
				if row, visible := evaluatePatternMatch(plan.query.pattern, match, plan, now, r.variables); visible {
					batch.New = append(batch.New, resultRow(row))
				}
			}
			r.patternState.timerEmitted = true
		}
	case patternTimerScheduleNode:
		for r.patternState.scheduleIndex < len(root.schedule) && !root.schedule[r.patternState.scheduleIndex].After(now) {
			dueAt := root.schedule[r.patternState.scheduleIndex]
			match := patternMatch{current: Event{}, startedAt: dueAt}
			if patternGuardAllows(plan.query.pattern, Event{}, dueAt, r.variables) {
				if row, visible := evaluatePatternMatch(plan.query.pattern, match, plan, dueAt, r.variables); visible {
					batch.New = append(batch.New, resultRow(row))
				}
			}
			r.patternState.scheduleIndex++
		}
	case patternTimerCronNode:
		if r.patternState.cronNext.IsZero() && root.cron != nil {
			if resolved, err := root.cron.resolve(EvalContext{Now: now, Variables: r.variables}); err == nil {
				r.patternState.cronSchedule = resolved
				r.patternState.cronNext, _ = resolved.nextAfter(now)
			}
		}
		const maxCronCatchUp = 100000
		for emitted := 0; emitted < maxCronCatchUp && !r.patternState.cronNext.IsZero() && !r.patternState.cronNext.After(now); emitted++ {
			dueAt := r.patternState.cronNext
			match := patternMatch{current: Event{}, startedAt: dueAt}
			if patternGuardAllows(plan.query.pattern, Event{}, dueAt, r.variables) {
				if row, visible := evaluatePatternMatch(plan.query.pattern, match, plan, dueAt, r.variables); visible {
					batch.New = append(batch.New, resultRow(row))
				}
			}
			if root.cronOneShot {
				r.patternState.cronNext = time.Time{}
				r.patternState.timerEmitted = true
				break
			}
			r.patternState.cronNext, _ = r.patternState.cronSchedule.nextAfter(dueAt)
		}
	}
	if !batch.empty() {
		if plan.query.distinct {
			batch.New, batch.Old = r.applyDistinct(plan.query, batch.New, batch.Old)
		}
		batch.New = applyResultWindow(batch.New, plan.query)
		batch.Sequence = r.seq.Add(1)
	}
	return batch
}

func (r *statementRuntime) patternCompositeTimeBatch(plan Plan, now time.Time) ResultBatch {
	definition := plan.query.pattern
	if definition == nil || definition.root == nil || !patternContainsTimer(definition.root) {
		return ResultBatch{}
	}
	if r.patternState.patternStopped {
		return ResultBatch{}
	}
	if plan.query.suppressOverlappingMatches {
		r.patternState.emittedEvents = nil
	}
	if !patternGuardAllows(definition, Event{}, now, r.variables) {
		r.patternState.active = nil
		return ResultBatch{}
	}
	expirePatternDistinct(r.patternState, definition, now)
	if len(r.patternState.active) == 0 && !r.patternState.patternStopped && patternCanStartWithoutEvent(definition.root) {
		progress := newPatternProgress(definition.root)
		armPatternProgressTimers(progress, now, r.variables)
		if patternProgressActive(progress) {
			r.patternState.active = []patternMatch{{state: progress, startedAt: now}}
		}
	}
	batch := ResultBatch{Time: now}
	batch.outputCountsSet = true
	nextActive := make([]patternMatch, 0, len(r.patternState.active))
	terminal := false
	completed := false
	for _, match := range r.patternState.active {
		if definition.within > 0 && !match.startedAt.Add(definition.within).After(now) {
			continue
		}
		transitions := advancePatternNodeTime(match.state, now, r.variables)
		for _, transition := range transitions {
			if transition.state == nil {
				continue
			}
			candidate := patternMatch{
				state:     transition.state,
				tags:      clonePatternTags(transition.state.tags),
				tagValues: clonePatternTagValues(transition.state.tagValues),
				current:   Event{},
				startedAt: match.startedAt,
			}
			if transition.complete {
				completed = true
				if row, visible := evaluatePatternMatch(definition, candidate, plan, now, r.variables); visible && r.patternState.acceptPatternMatch(plan.query, candidate) {
					batch.New = append(batch.New, resultRow(row))
				}
				if patternCanContinueAfterMatch(transition.state) && patternMatchWithinLimits(nextActive, candidate, definition) {
					nextActive = append(nextActive, candidate)
				}
				if patternWithinTerminal(transition.state) {
					terminal = true
				}
				if definition.every {
					progress := newPatternProgress(definition.root)
					armPatternProgressTimers(progress, now, r.variables)
					candidate := patternMatch{state: progress, startedAt: now}
					if patternProgressActive(progress) && patternMatchWithinLimits(nextActive, candidate, definition) {
						nextActive = append(nextActive, candidate)
					}
				}
				continue
			}
			if patternWithinTerminal(transition.state) {
				terminal = true
			}
			if patternProgressActive(transition.state) && patternMatchWithinLimits(nextActive, candidate, definition) {
				nextActive = append(nextActive, candidate)
			}
		}
	}
	if plan.query.discardPartialsOnMatch && completed {
		nextActive = nil
	}
	r.patternState.active = nextActive
	if definition.root.kind == patternWithinNode && terminal {
		r.patternState.patternStopped = true
	}
	if !batch.empty() {
		if plan.query.distinct {
			batch.New, batch.Old = r.applyDistinct(plan.query, batch.New, batch.Old)
		}
		batch.New = applyResultWindow(batch.New, plan.query)
		batch.Old = applyResultWindow(batch.Old, plan.query)
		batch.Sequence = r.seq.Add(1)
	}
	return batch
}

func patternGuardAllows(definition *patternDefinition, event Event, now time.Time, variables map[string]Value) bool {
	if definition == nil || definition.guard == nil {
		return true
	}
	value := definition.guard.eval(EvalContext{Event: event, Now: now, Variables: variables})
	allowed, ok := boolValue(value)
	return ok && allowed
}

func clonePatternMatch(match patternMatch) patternMatch {
	return patternMatch{
		state:     clonePatternProgress(match.state),
		tags:      clonePatternTags(match.tags),
		tagValues: clonePatternTagValues(match.tagValues),
		current:   match.current,
		startedAt: match.startedAt,
	}
}

func evaluatePatternMatch(definition *patternDefinition, match patternMatch, plan Plan, now time.Time, variables map[string]Value) (Row, bool) {
	if len(plan.query.patternSelections) == 0 {
		return Row{}, false
	}
	ctx := EvalContext{Event: match.current, Tags: match.tags, TagValues: match.tagValues, Now: now, Variables: variables}
	values := make([]Value, 0, len(plan.query.patternSelections))
	for _, selection := range plan.query.patternSelections {
		values = append(values, selection.Expr.eval(ctx))
	}
	return newRow(plan.resultSchema, values), true
}

func (r *statementRuntime) patternExpire(definition *patternDefinition, now time.Time) {
	if definition == nil || r.patternState == nil {
		return
	}
	expirePatternDistinct(r.patternState, definition, now)
	if definition.within > 0 {
		kept := r.patternState.active[:0]
		for _, match := range r.patternState.active {
			if match.startedAt.Add(definition.within).After(now) {
				kept = append(kept, match)
			}
		}
		r.patternState.active = kept
	}
}

func (r *statementRuntime) aggregateBatch(delta eventDelta, plan Plan, now time.Time) (ResultBatch, error) {
	definition := plan.query.aggregate
	if definition == nil {
		return ResultBatch{}, NewError(ErrorInvalidRule, "aggregate runtime has no definition")
	}
	if r.aggregateState == nil {
		r.aggregateState = &aggregateRuntimeState{groups: make(map[string]*aggregateGroup)}
	}
	state := r.aggregateState
	removeAggregateScopeEvents(state, delta.oldEvents)
	state.allEvents = appendAggregateScopeEvents(state.allEvents, delta.newEvents)
	state.allEverEvents = appendAggregateScopeEvents(state.allEverEvents, delta.newEvents)
	affected := make([]string, 0)
	seen := make(map[string]struct{})
	groupingSets := aggregateGroupingSetsForDefinition(definition)
	markAffected := func(event Event, groupingSet []int) string {
		key := aggregateGroupKey(definition.groupBy, groupingSet, event, now, r.variables)
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			affected = append(affected, key)
		}
		group := state.groups[key]
		if group == nil {
			group = &aggregateGroup{
				groupingSet:    append([]int(nil), groupingSet...),
				representative: event,
				pluginStates:   make(map[*exprNode]aggregatePluginState),
			}
			state.groups[key] = group
		}
		group.current = event
		return key
	}
	for _, event := range delta.oldEvents {
		for _, groupingSet := range groupingSets {
			key := markAffected(event, groupingSet)
			group := state.groups[key]
			group.leaving = true
			group.leavingEvents = append(group.leavingEvents, event)
			for index, current := range group.events {
				if sameEvent(current, event) {
					group.events = append(group.events[:index], group.events[index+1:]...)
					break
				}
			}
		}
	}
	for _, event := range delta.newEvents {
		for _, groupingSet := range groupingSets {
			key := markAffected(event, groupingSet)
			state.groups[key].events = append(state.groups[key].events, event)
			state.groups[key].everEvents = append(state.groups[key].everEvents, event)
		}
	}

	batch := ResultBatch{Time: now}
	batch.outputCountsSet = true
	batch.outputInserted = int64(len(delta.newEvents))
	batch.outputRemoved = int64(len(delta.oldEvents))
	newEntries := make([]aggregateResultEntry, 0, len(affected))
	oldEntries := make([]aggregateResultEntry, 0, len(affected))
	for _, key := range affected {
		group := state.groups[key]
		if group == nil {
			continue
		}
		if group.emitted && (plan.query.selector == SelectRStream || plan.query.selector == SelectIRStream) {
			oldEntries = append(oldEntries, aggregateResultEntry{
				result: resultRow(newRow(plan.resultSchema, group.previous)),
				group:  group,
			})
		}
		newValues, visible := evaluateAggregateGroup(definition, group.events, group.everEvents, group.leavingEvents, group.leaving, group.groupingSet, group.current, state.allEvents, state.allEverEvents, now, r.variables, group.pluginStates)
		if visible && (plan.query.selector == SelectIStream || plan.query.selector == SelectIRStream) {
			newEntries = append(newEntries, aggregateResultEntry{
				result: resultRow(newRow(plan.resultSchema, newValues)),
				group:  group,
			})
		}
		if visible {
			group.previous = append([]Value(nil), newValues...)
			group.emitted = true
		} else {
			group.previous = nil
			group.emitted = false
		}
		if len(group.events) == 0 && !aggregateDefinitionUsesEver(definition) {
			delete(state.groups, key)
		}
	}
	if len(plan.query.orderBy) > 0 {
		orderAggregateResults(newEntries, plan.query.orderBy, definition, state.allEvents, state.allEverEvents, now, r.variables, false)
		orderAggregateResults(oldEntries, plan.query.orderBy, definition, state.allEvents, state.allEverEvents, now, r.variables, true)
	}
	for _, entry := range newEntries {
		batch.New = append(batch.New, entry.result)
	}
	for _, entry := range oldEntries {
		batch.Old = append(batch.Old, entry.result)
	}
	if !batch.empty() {
		if plan.query.distinct {
			batch.New, batch.Old = r.applyDistinct(plan.query, batch.New, batch.Old)
		}
		batch.New = applyResultWindow(batch.New, plan.query)
		batch.Old = applyResultWindow(batch.Old, plan.query)
		batch.Sequence = r.seq.Add(1)
	}
	if plan.query.tableTarget != "" {
		if err := r.persistAggregateTable(plan, now); err != nil {
			return ResultBatch{}, err
		}
	}
	return batch, nil
}

func (r *statementRuntime) persistAggregateTable(plan Plan, now time.Time) error {
	if r == nil || r.engine == nil {
		return NewError(ErrorDependency, "into-table aggregate has no engine")
	}
	table, ok := r.engine.tables[plan.query.tableTarget]
	if !ok || table == nil {
		return NewError(ErrorUnknownName, fmt.Sprintf("into-table target %q is not registered", plan.query.tableTarget))
	}
	definition := plan.query.aggregate
	if definition == nil || r.aggregateState == nil {
		return NewError(ErrorInvalidRule, "into-table aggregate state is not initialized")
	}
	keys := make([]string, 0, len(r.aggregateState.groups))
	for key := range r.aggregateState.groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		group := r.aggregateState.groups[key]
		if group == nil || len(group.events) == 0 {
			continue
		}
		values, visible := evaluateAggregateGroup(definition, group.events, group.everEvents, group.leavingEvents, group.leaving, group.groupingSet, group.current, r.aggregateState.allEvents, r.aggregateState.allEverEvents, now, r.variables, group.pluginStates)
		if !visible {
			continue
		}
		row := make(map[string]any, len(definition.selections))
		for index, selection := range definition.selections {
			if index < len(values) {
				row[selection.Name] = values[index].Any()
			}
		}
		rows = append(rows, row)
	}
	primaryKey := table.Definition().PrimaryKey()
	sort.SliceStable(rows, func(left, right int) bool {
		leftValues := make([]any, 0, len(primaryKey))
		rightValues := make([]any, 0, len(primaryKey))
		for _, column := range primaryKey {
			leftValues = append(leftValues, rows[left][column])
			rightValues = append(rightValues, rows[right][column])
		}
		return encodeKey(leftValues) < encodeKey(rightValues)
	})
	if err := table.Replace(r.context(), rows); err != nil {
		return WrapError(ErrorState, "into-table."+plan.query.tableTarget, err)
	}
	return nil
}

func aggregateGroupingSetsForDefinition(definition *aggregateDefinition) [][]int {
	if definition == nil || len(definition.groupBy) == 0 {
		return [][]int{nil}
	}
	if len(definition.groupingSets) > 0 {
		sets := make([][]int, len(definition.groupingSets))
		for index, set := range definition.groupingSets {
			sets[index] = append([]int(nil), set...)
		}
		return sets
	}
	switch definition.grouping {
	case aggregateGroupingRollup:
		return rollupGroupingSets(len(definition.groupBy))
	case aggregateGroupingCube:
		return cubeGroupingSets(len(definition.groupBy))
	default:
		return [][]int{allGroupingSetIndices(len(definition.groupBy))}
	}
}

func allGroupingSetIndices(size int) []int {
	indices := make([]int, size)
	for index := range indices {
		indices[index] = index
	}
	return indices
}

func rollupGroupingSets(size int) [][]int {
	sets := make([][]int, 0, size+1)
	for width := size; width >= 0; width-- {
		set := make([]int, width)
		for index := range set {
			set[index] = index
		}
		sets = append(sets, set)
	}
	return sets
}

func cubeGroupingSets(size int) [][]int {
	if size < 0 || size >= 63 {
		return nil
	}
	sets := make([][]int, 0, 1<<size)
	for mask := (uint64(1) << size) - 1; ; mask-- {
		set := make([]int, 0, size)
		for index := 0; index < size; index++ {
			if mask&(uint64(1)<<uint(index)) != 0 {
				set = append(set, index)
			}
		}
		sets = append(sets, set)
		if mask == 0 {
			break
		}
	}
	return sets
}

func appendAggregateScopeEvents(existing, additions []Event) []Event {
	if len(additions) == 0 {
		return existing
	}
	return append(existing, additions...)
}

func removeAggregateScopeEvents(state *aggregateRuntimeState, removals []Event) {
	if state == nil || len(removals) == 0 || len(state.allEvents) == 0 {
		return
	}
	for _, removal := range removals {
		for index, event := range state.allEvents {
			if sameEvent(event, removal) {
				state.allEvents = append(state.allEvents[:index], state.allEvents[index+1:]...)
				break
			}
		}
	}
}

func aggregateGroupKey(groupBy []Expr, groupingSet []int, event Event, now time.Time, variables map[string]Value) string {
	if len(groupBy) == 0 {
		return "<all>"
	}
	present := make(map[int]struct{}, len(groupingSet))
	for _, index := range groupingSet {
		present[index] = struct{}{}
	}
	values := make([]any, 0, len(groupBy)+1)
	values = append(values, groupingSet)
	for index, expression := range groupBy {
		if _, ok := present[index]; !ok {
			values = append(values, Null())
			continue
		}
		values = append(values, expression.eval(EvalContext{Event: event, Now: now, Variables: variables}).Any())
	}
	return encodeKey(values)
}

func evaluateAggregateGroup(definition *aggregateDefinition, events []Event, everEvents []Event, leavingEvents []Event, leaving bool, groupingSet []int, current Event, allEvents []Event, allEverEvents []Event, now time.Time, variables map[string]Value, pluginStates map[*exprNode]aggregatePluginState) ([]Value, bool) {
	if len(events) == 0 && len(everEvents) == 0 && current.Schema().Name() == "" {
		return nil, false
	}
	ctx := aggregateGroupContext(definition, events, everEvents, leavingEvents, leaving, groupingSet, current, allEvents, allEverEvents, now, variables, pluginStates)
	values := make([]Value, 0, len(definition.selections))
	for _, selection := range definition.selections {
		values = append(values, evaluateAggregateExpression(selection.Expr, ctx))
	}
	if definition.having != nil {
		value := evaluateAggregateExpression(definition.having, ctx)
		ok, isBool := boolValue(value)
		if !isBool || !ok {
			return values, false
		}
	}
	return values, true
}

func aggregateGroupContext(definition *aggregateDefinition, events []Event, everEvents []Event, leavingEvents []Event, leaving bool, groupingSet []int, current Event, allEvents []Event, allEverEvents []Event, now time.Time, variables map[string]Value, pluginStates map[*exprNode]aggregatePluginState) EvalContext {
	if current.Schema().Name() == "" {
		if len(events) > 0 {
			current = events[0]
		} else if len(everEvents) > 0 {
			current = everEvents[0]
		}
	}
	ctx := EvalContext{Event: current, Group: append([]Event(nil), events...), EverGroup: append([]Event(nil), everEvents...), AllGroup: append([]Event(nil), allEvents...), AllEverGroup: append([]Event(nil), allEverEvents...), LeavingEvents: append([]Event(nil), leavingEvents...), IsLeaving: leaving, Now: now, Variables: variables, aggregatePluginStates: pluginStates}
	if len(definition.groupBy) > 0 {
		groupingEvent := current
		if groupingEvent.Schema().Name() == "" {
			if len(events) > 0 {
				groupingEvent = events[0]
			} else if len(everEvents) > 0 {
				groupingEvent = everEvents[0]
			}
		}
		ctx.groupingValues = make(map[string]Value, len(definition.groupBy))
		ctx.groupingPresent = make(map[string]bool, len(definition.groupBy))
		present := make(map[int]struct{}, len(groupingSet))
		for _, index := range groupingSet {
			present[index] = struct{}{}
		}
		for index, expression := range definition.groupBy {
			if expression == nil {
				continue
			}
			key := groupingExpressionKey(expression)
			isPresent := false
			if _, ok := present[index]; ok {
				isPresent = true
			}
			ctx.groupingPresent[key] = isPresent
			if isPresent {
				ctx.groupingValues[key] = expression.eval(EvalContext{Event: groupingEvent, Now: now, Variables: variables})
			} else {
				ctx.groupingValues[key] = Null()
			}
		}
	}
	return ctx
}

func orderAggregateResults(entries []aggregateResultEntry, keys []SortKey, definition *aggregateDefinition, allEvents []Event, allEverEvents []Event, now time.Time, variables map[string]Value, leaving bool) {
	if len(entries) < 2 || len(keys) == 0 {
		return
	}
	sort.SliceStable(entries, func(left, right int) bool {
		leftRow, _ := entries[left].result.Row()
		rightRow, _ := entries[right].result.Row()
		for _, key := range keys {
			leftContext := aggregateResultContext(entries[left].group, definition, allEvents, allEverEvents, now, variables, leaving)
			leftContext.resultRow = &leftRow
			rightContext := aggregateResultContext(entries[right].group, definition, allEvents, allEverEvents, now, variables, leaving)
			rightContext.resultRow = &rightRow
			comparison, ok := compareOrderValues(key.Expr.eval(leftContext), key.Expr.eval(rightContext))
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
}

func aggregateResultContext(group *aggregateGroup, definition *aggregateDefinition, allEvents []Event, allEverEvents []Event, now time.Time, variables map[string]Value, leaving bool) EvalContext {
	if group == nil {
		return EvalContext{Now: now, Variables: variables, IsLeaving: leaving}
	}
	events := group.events
	if len(events) == 0 && group.representative.Schema().Name() != "" {
		events = []Event{group.representative}
	}
	ctx := aggregateGroupContext(definition, events, group.everEvents, group.leavingEvents, group.leaving, group.groupingSet, group.current, allEvents, allEverEvents, now, variables, group.pluginStates)
	ctx.IsLeaving = leaving
	return ctx
}

func compareOrderValues(left, right Value) (int, bool) {
	leftPresent := left.IsPresent()
	rightPresent := right.IsPresent()
	if !leftPresent || !rightPresent {
		switch {
		case !leftPresent && !rightPresent:
			return 0, true
		case !leftPresent:
			return -1, true
		default:
			return 1, true
		}
	}
	return compareValues(left, right)
}

func aggregateDefinitionUsesEver(definition *aggregateDefinition) bool {
	if definition == nil {
		return false
	}
	for _, selection := range definition.selections {
		if expressionTreeContainsEver(selection.Expr) {
			return true
		}
	}
	return expressionTreeContainsEver(definition.having)
}

func expressionTreeContainsEver(expression Expr) bool {
	if expression == nil || expression.node() == nil {
		return false
	}
	var visit func(*exprNode) bool
	visit = func(node *exprNode) bool {
		if node == nil {
			return false
		}
		switch node.kind {
		case "count-ever", "first-ever", "last-ever", "min-by-ever", "max-by-ever":
			return true
		}
		for _, child := range node.children {
			if visit(child) {
				return true
			}
		}
		return false
	}
	return visit(expression.node())
}

func evaluateAggregateExpression(expression Expr, ctx EvalContext) Value {
	if expression == nil {
		return Missing()
	}
	if ctx.groupingValues != nil {
		if value, ok := ctx.groupingValues[groupingExpressionKey(expression)]; ok {
			return value
		}
	}
	return expression.eval(ctx)
}

func (r *statementRuntime) batch(delta eventDelta, plan Plan, now time.Time) ResultBatch {
	batch := ResultBatch{Time: now}
	batch.outputCountsSet = true
	batch.outputInserted = int64(len(delta.newEvents))
	batch.outputRemoved = int64(len(delta.oldEvents))
	newResults := []Result(nil)
	oldResults := []Result(nil)
	if plan.query.selector == SelectIStream || plan.query.selector == SelectIRStream || plan.query.distinct {
		newResults = projectResults(delta.newEvents, plan.query, plan.resultSchema, now, r.variables, delta.history, delta.historyByEvent, delta.previousByEvent, delta.priorByEvent, false)
	}
	if plan.query.selector == SelectRStream || plan.query.selector == SelectIRStream || plan.query.distinct {
		oldPreviousByEvent := cloneEventHistories(delta.previousByEvent)
		if len(delta.oldEvents) > 0 {
			if oldPreviousByEvent == nil {
				oldPreviousByEvent = make(map[string][]Event)
			}
			for _, old := range delta.oldEvents {
				oldPreviousByEvent[eventIdentity(old)] = nil
			}
		}
		oldHistoryByEvent := cloneEventHistories(delta.historyByEvent)
		if len(delta.oldEvents) > 0 {
			if oldHistoryByEvent == nil {
				oldHistoryByEvent = make(map[string][]Event)
			}
			for _, old := range delta.oldEvents {
				oldHistoryByEvent[eventIdentity(old)] = nil
			}
		}
		oldResults = projectResults(delta.oldEvents, plan.query, plan.resultSchema, now, r.variables, delta.history, oldHistoryByEvent, oldPreviousByEvent, delta.priorByEvent, true)
	}
	if plan.query.distinct {
		newResults, oldResults = r.applyDistinct(plan.query, newResults, oldResults)
	}
	if plan.query.selector == SelectIStream || plan.query.selector == SelectIRStream {
		batch.New = applyResultWindow(newResults, plan.query)
	}
	if plan.query.selector == SelectRStream || plan.query.selector == SelectIRStream {
		batch.Old = applyResultWindow(oldResults, plan.query)
	}
	if !batch.empty() {
		batch.Sequence = r.seq.Add(1)
	}
	return batch
}

func (r *statementRuntime) joinBatch(delta joinDelta, plan Plan, now time.Time) ResultBatch {
	batch := ResultBatch{Time: now}
	if len(joinDefinitionSources(plan.query.join)) > 2 {
		if plan.query.selector == SelectIStream || plan.query.selector == SelectIRStream {
			batch.New = projectJoinTuples(delta.newTuples, plan.query, plan.resultSchema, now, r.variables, false)
		}
		if plan.query.selector == SelectRStream || plan.query.selector == SelectIRStream {
			batch.Old = projectJoinTuples(delta.oldTuples, plan.query, plan.resultSchema, now, r.variables, true)
		}
	} else {
		if plan.query.selector == SelectIStream || plan.query.selector == SelectIRStream {
			batch.New = projectJoinResults(delta.newPairs, plan.query, plan.resultSchema, now, r.variables, false)
		}
		if plan.query.selector == SelectRStream || plan.query.selector == SelectIRStream {
			batch.Old = projectJoinResults(delta.oldPairs, plan.query, plan.resultSchema, now, r.variables, true)
		}
	}
	if plan.query.distinct {
		batch.New, batch.Old = r.applyDistinct(plan.query, batch.New, batch.Old)
	}
	batch.New = applyResultWindow(batch.New, plan.query)
	batch.Old = applyResultWindow(batch.Old, plan.query)
	if !batch.empty() {
		batch.Sequence = r.seq.Add(1)
	}
	return batch
}

func projectResults(events []Event, query Query, resultSchema Schema, now time.Time, variables map[string]Value, history []Event, historyByEvent, previousByEvent, priorByEvent map[string][]Event, leaving bool) []Result {
	if len(events) == 0 {
		return nil
	}
	events = orderEvents(events, query.orderBy, now, variables, history, historyByEvent, previousByEvent, priorByEvent, leaving)
	results := make([]Result, 0, len(events))
	for _, event := range events {
		if len(query.selections) == 0 {
			results = append(results, resultEvent(event))
			continue
		}
		values := make([]Value, 0, len(query.selections))
		for _, selection := range query.selections {
			values = append(values, selection.Expr.eval(projectionEvalContext(event, now, variables, history, historyByEvent, previousByEvent, priorByEvent, leaving)))
		}
		row := newRow(resultSchema, values)
		results = append(results, resultRow(row))
	}
	return results
}

func orderEvents(events []Event, keys []SortKey, now time.Time, variables map[string]Value, history []Event, historyByEvent, previousByEvent, priorByEvent map[string][]Event, leaving bool) []Event {
	if len(keys) == 0 || len(events) < 2 {
		return events
	}
	ordered := append([]Event(nil), events...)
	sort.SliceStable(ordered, func(left, right int) bool {
		for _, key := range keys {
			comparison, ok := compareValues(key.Expr.eval(projectionEvalContext(ordered[left], now, variables, history, historyByEvent, previousByEvent, priorByEvent, leaving)), key.Expr.eval(projectionEvalContext(ordered[right], now, variables, history, historyByEvent, previousByEvent, priorByEvent, leaving)))
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
	return ordered
}

func projectionEvalContext(event Event, now time.Time, variables map[string]Value, history []Event, historyByEvent, previousByEvent, priorByEvent map[string][]Event, leaving bool) EvalContext {
	eventHistory := history
	identity := eventIdentity(event)
	if historyByEvent != nil {
		eventHistory = historyByEvent[identity]
	}
	ctx := EvalContext{Event: event, History: append([]Event(nil), eventHistory...), IsLeaving: leaving, Now: now, Variables: variables}
	if previousByEvent != nil {
		if previous, ok := previousByEvent[identity]; ok {
			ctx.PreviousWindowAccess = true
			ctx.PreviousHistory = append([]Event(nil), previous...)
		}
	}
	if priorByEvent != nil {
		if prior, ok := priorByEvent[identity]; ok {
			ctx.PriorHistorySet = true
			ctx.PriorHistory = append([]Event(nil), prior...)
		}
	}
	return ctx
}

func (r *statementRuntime) applyDistinct(query Query, newResults, oldResults []Result) ([]Result, []Result) {
	if r.distinctCounts == nil {
		r.distinctCounts = make(map[string]int)
	}
	oldOutput := make([]Result, 0, len(oldResults))
	for _, result := range oldResults {
		key := resultKey(result)
		count := r.distinctCounts[key]
		if count <= 1 {
			delete(r.distinctCounts, key)
			if count == 1 {
				oldOutput = append(oldOutput, result)
			}
		} else {
			r.distinctCounts[key] = count - 1
		}
	}
	newOutput := make([]Result, 0, len(newResults))
	for _, result := range newResults {
		key := resultKey(result)
		if r.distinctCounts[key] == 0 {
			newOutput = append(newOutput, result)
		}
		r.distinctCounts[key]++
	}
	return newOutput, oldOutput
}

func applyResultWindow(results []Result, query Query) []Result {
	if len(results) == 0 {
		return nil
	}
	start := query.offset
	if start >= len(results) {
		return nil
	}
	if start < 0 {
		start = 0
	}
	end := len(results)
	if query.limit > 0 && start+query.limit < end {
		end = start + query.limit
	}
	return append([]Result(nil), results[start:end]...)
}

func resultKey(result Result) string {
	if row, ok := result.Row(); ok {
		values := make([]any, 0, len(row.Values())*2)
		for _, value := range row.Values() {
			values = append(values, value.State(), value.Any())
		}
		return "row:" + encodeKey(values)
	}
	if event, ok := result.Event(); ok {
		return "event:" + event.TypeName() + ":" + encodeKey([]any{event.Underlying()})
	}
	return "empty"
}

func projectJoinResults(pairs []eventPair, query Query, resultSchema Schema, now time.Time, variables map[string]Value, leaving bool) []Result {
	tuuples := make([][]Event, 0, len(pairs))
	for _, pair := range pairs {
		tuuples = append(tuuples, []Event{pair.left, pair.right})
	}
	return projectJoinTuples(tuuples, query, resultSchema, now, variables, leaving)
}

func projectJoinTuples(tuples [][]Event, query Query, resultSchema Schema, now time.Time, variables map[string]Value, leaving bool) []Result {
	if len(tuples) == 0 {
		return nil
	}
	results := make([]Result, 0, len(tuples))
	for _, tuple := range tuples {
		values := make([]Value, 0, len(query.joinSelections))
		for _, selection := range query.joinSelections {
			source := selection.sourceIndex()
			var event Event
			if source >= 0 && source < len(tuple) {
				event = tuple[source]
			}
			values = append(values, selection.Expr.eval(EvalContext{Event: event, IsLeaving: leaving, Now: now, Variables: variables}))
		}
		results = append(results, resultRow(newRow(resultSchema, values)))
	}
	return results
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		switch ctx.Err() {
		case context.Canceled:
			return NewError(ErrorCanceled, "context canceled")
		case context.DeadlineExceeded:
			return NewError(ErrorTimeout, "context deadline exceeded")
		default:
			return ctx.Err()
		}
	default:
		return nil
	}
}
