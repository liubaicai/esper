package esper

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const maxRoutedEventsPerSend = 1024

// Listener receives one deterministic new/old-stream batch.
type Listener func(context.Context, ResultBatch) error

// UnmatchedListener receives an event that did not match any active
// statement. Insert-into routes participate in the same boundary, so the
// callback may receive either an externally sent event or an internally
// routed Event while preserving its underlying representation.
type UnmatchedListener func(context.Context, Event) error

// replayListener keeps the initial snapshot ahead of any live batches that
// arrive while SubscribeWithReplay is invoking the caller. Live dispatches
// buffer instead of blocking, which also permits a replay callback to send an
// event back into the same engine without deadlocking.
type replayListener struct {
	mu        sync.Mutex
	listener  Listener
	buffering bool
	queued    []replayDelivery
}

type replayDelivery struct {
	ctx   context.Context
	batch ResultBatch
}

func newReplayListener(listener Listener) *replayListener {
	return &replayListener{listener: listener, buffering: true}
}

func (l *replayListener) deliver(ctx context.Context, batch ResultBatch) error {
	if l == nil || l.listener == nil {
		return nil
	}
	l.mu.Lock()
	if l.buffering {
		l.queued = append(l.queued, replayDelivery{ctx: ctx, batch: batch.clone()})
		l.mu.Unlock()
		return nil
	}
	l.mu.Unlock()
	return l.listener(ctx, batch)
}

func (l *replayListener) finish(ctx context.Context, replay ResultBatch) error {
	if l == nil || l.listener == nil {
		return nil
	}
	if err := l.listener(ctx, replay.clone()); err != nil {
		return err
	}
	for {
		l.mu.Lock()
		if len(l.queued) == 0 {
			l.buffering = false
			l.mu.Unlock()
			return nil
		}
		queued := append([]replayDelivery(nil), l.queued...)
		l.queued = nil
		l.mu.Unlock()
		for _, delivery := range queued {
			if err := l.listener(delivery.ctx, delivery.batch); err != nil {
				return err
			}
		}
	}
}

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
	event      *Event
	row        *Row
	joinEvents []Event
}

func resultEvent(event Event) Result { return Result{event: &event} }
func resultRow(row Row) Result       { return Result{row: &row} }

func resultJoinRow(row Row, tuple []Event) Result {
	return Result{row: &row, joinEvents: append([]Event(nil), tuple...)}
}

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
	outputKeysNew   []string
	outputKeysOld   []string
}

func (b ResultBatch) empty() bool { return len(b.New) == 0 && len(b.Old) == 0 }

func (b ResultBatch) clone() ResultBatch {
	b.New = append([]Result(nil), b.New...)
	b.Old = append([]Result(nil), b.Old...)
	b.outputKeysNew = append([]string(nil), b.outputKeysNew...)
	b.outputKeysOld = append([]string(nil), b.outputKeysOld...)
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
	clock                    *VirtualClock
	matchRecognize           MatchRecognizeRuntimeConfig
	runtimeURI               string
	services                 map[string]any
	lockActivity             bool
	runtimeMetricsConfigured bool
	runtimeMetricsInterval   time.Duration
	statementMetrics         *statementMetricsConfig
	inboundPool              asyncPoolConfig
	outboundPool             asyncPoolConfig
	routePool                asyncPoolConfig
	timerPool                asyncPoolConfig
}

type EngineOption func(*engineConfig)

func WithClock(clock *VirtualClock) EngineOption {
	return func(cfg *engineConfig) { cfg.clock = clock }
}

func WithStartTime(start time.Time) EngineOption {
	return func(cfg *engineConfig) { cfg.clock = NewVirtualClock(start) }
}

// WithRuntimeURI sets the stable runtime identifier exposed by
// CurrentEvaluationContext and lifecycle metadata. The default is "default".
func WithRuntimeURI(uri string) EngineOption {
	return func(cfg *engineConfig) { cfg.runtimeURI = strings.TrimSpace(uri) }
}

// WithRuntimeService installs one immutable process-local dependency in the
// Engine. Listeners and extensions retrieve it by stable name through
// RuntimeService instead of relying on Java-style transient configuration or
// a global service locator.
func WithRuntimeService(name string, service any) EngineOption {
	name = strings.TrimSpace(name)
	return func(cfg *engineConfig) {
		if name == "" {
			return
		}
		if cfg.services == nil {
			cfg.services = make(map[string]any)
		}
		cfg.services[name] = service
	}
}

// WithLockActivityTracing enables an in-memory trace of exact Engine mutex
// attempt/acquire/release activity. Tracing is disabled by default and does
// not write to process-global logs; callers inspect it through LockActivity.
func WithLockActivityTracing() EngineOption {
	return func(cfg *engineConfig) { cfg.lockActivity = true }
}

// Engine owns deployed statements and the explicit processing clock.
type Engine struct {
	mu                                 runtimeMutex
	env                                *Environment
	clock                              *VirtualClock
	runtimeURI                         string
	services                           map[string]any
	lockActivity                       *lockActivityRecorder
	matchRecognizeStatePool            *rowRecogStatePool
	matchRecognizeStateLimitListeners  []MatchRecognizeStateLimitListener
	pendingMatchRecognizeStateLimits   []MatchRecognizeStateLimitEvent
	patternSubexpressionLimitListeners []PatternSubexpressionLimitListener
	pendingPatternSubexpressionLimits  []PatternSubexpressionLimitEvent
	auditListeners                     map[uint64]AuditListener
	nextAuditListenerID                uint64
	pendingAuditRecords                []AuditRecord
	unmatchedListener                  UnmatchedListener
	variables                          map[string]Value
	contextVariables                   map[string]map[string]map[string]Value
	contextPartitionRefs               map[string]map[string]int
	contextPartitionIDs                map[string]map[string]int
	contextPartitionNextIDs            map[string]int
	contextPartitionInstanceNextIDs    map[string]uint64
	contextPartitionDescriptors        map[string]map[string]ContextPartitionDescriptor
	contextPartitionListeners          map[string][]ContextPartitionStateListener
	contextTableOwnership              map[string]map[string]map[uint64]tableContextRowOwnership
	contextTemporalOrigins             map[string]time.Time
	contextStateListeners              []ContextStateListener
	contextCreated                     map[string]bool
	contextStatementRefs               map[string]int
	pendingContextEvents               []contextNotification
	variableChangeListeners            map[string][]VariableChangeListener
	subscriberErrorMu                  sync.RWMutex
	subscriberErrorHandler             SubscriberErrorHandler
	threadingErrorMu                   sync.RWMutex
	threadingErrorHandler              ThreadingErrorHandler
	runtimeMetrics                     *runtimeMetricsState
	statementMetrics                   *statementMetricsState
	pendingVariableChanges             []VariableChangeEvent
	tables                             map[string]*Table
	namedWindows                       map[string]*NamedWindow
	statements                         map[string]*Statement
	deployments                        map[string]*Deployment
	dataflows                          map[*DataflowInstance]struct{}
	savedDataflowInstances             map[string]*DataflowInstance
	pendingStatementDispatches         []statementDispatch
	pendingNamedWindowDispatches       []namedWindowDispatch
	pendingRoutedEvents                []Event
	closed                             bool
	nextID                             uint64
	inboundPool                        *asyncTaskPool
	outboundPool                       *asyncTaskPool
	routePool                          *asyncTaskPool
	timerPool                          *asyncTaskPool
}

func NewEngine(env *Environment, options ...EngineOption) *Engine {
	cfg := engineConfig{
		clock:      NewVirtualClock(time.Unix(0, 0).UTC()),
		runtimeURI: "default",
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
	if strings.TrimSpace(cfg.runtimeURI) == "" {
		cfg.runtimeURI = "default"
	}
	engine := &Engine{
		env:                             env,
		clock:                           cfg.clock,
		runtimeURI:                      cfg.runtimeURI,
		services:                        make(map[string]any, len(cfg.services)),
		matchRecognizeStatePool:         newRowRecogStatePool(cfg.matchRecognize),
		variables:                       make(map[string]Value),
		contextVariables:                make(map[string]map[string]map[string]Value),
		contextPartitionRefs:            make(map[string]map[string]int),
		contextPartitionIDs:             make(map[string]map[string]int),
		contextPartitionNextIDs:         make(map[string]int),
		contextPartitionInstanceNextIDs: make(map[string]uint64),
		contextPartitionDescriptors:     make(map[string]map[string]ContextPartitionDescriptor),
		contextPartitionListeners:       make(map[string][]ContextPartitionStateListener),
		auditListeners:                  make(map[uint64]AuditListener),
		contextTableOwnership:           make(map[string]map[string]map[uint64]tableContextRowOwnership),
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
	engine.inboundPool = newAsyncTaskPool(ThreadingInbound, cfg.inboundPool)
	engine.outboundPool = newAsyncTaskPool(ThreadingOutbound, cfg.outboundPool)
	engine.routePool = newAsyncTaskPool(ThreadingRoute, cfg.routePool)
	engine.timerPool = newAsyncTaskPool(ThreadingTimer, cfg.timerPool)
	engine.runtimeMetrics = newRuntimeMetricsState(cfg.runtimeMetricsInterval, cfg.runtimeMetricsConfigured)
	engine.statementMetrics = newStatementMetricsState(cfg.statementMetrics, engine.runtimeURI)
	if cfg.lockActivity {
		engine.lockActivity = newLockActivityRecorder()
		engine.mu.recorder = engine.lockActivity
	}
	for name, service := range cfg.services {
		engine.services[name] = service
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
		namedWindowNames := make([]string, 0, len(env.namedWindows))
		for name := range env.namedWindows {
			namedWindowNames = append(namedWindowNames, name)
		}
		sort.Strings(namedWindowNames)
		for index, name := range namedWindowNames {
			definition := env.namedWindows[name]
			engine.namedWindows[name] = newNamedWindow(definition, engine)
			engine.registerNamedWindowMetricsLocked(definition.Name(), uint64(index+1))
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

// RuntimeURI returns the immutable identifier of this Engine.
func (e *Engine) RuntimeURI() string {
	if e == nil {
		return ""
	}
	return e.runtimeURI
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
		e.auditContextPartitionLocked(contextName, descriptor.ID, true, e.clock.Now())
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
	definition, definitionOK := e.env.Context(contextName)
	lifecycleManaged := definitionOK && (definition.isTemporal() || definition.kind == ContextInitiatedTerminated)
	if lifecycleManaged {
		for _, table := range e.tables {
			if table != nil {
				table.releaseContextPartition(contextName, partitionKey)
			}
		}
		for tableKey, byContext := range e.contextTableOwnership {
			byRow := byContext[contextName]
			for identity, ownership := range byRow {
				if ownership.partitionKey == partitionKey {
					delete(byRow, identity)
				}
			}
			if len(byRow) == 0 {
				delete(byContext, contextName)
			}
			if len(byContext) == 0 {
				delete(e.contextTableOwnership, tableKey)
			}
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
	e.auditContextPartitionLocked(contextName, descriptor.ID, false, e.clock.Now())
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
	return e.TableInModule("", name)
}

func (e *Engine) TableInModule(moduleName, name string) (*Table, bool) {
	if e == nil {
		return nil, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	table, ok := e.tables[catalogKey(moduleName, name)]
	return table, ok
}

func (e *Engine) NamedWindow(name string) (*NamedWindow, bool) {
	return e.NamedWindowInModule("", name)
}

func (e *Engine) NamedWindowInModule(moduleName, name string) (*NamedWindow, bool) {
	if e == nil {
		return nil, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.ensureNamedWindowLockedInModule(moduleName, name)
}

// ensureNamedWindowLocked keeps the runtime catalog in sync with the
// environment catalog.  Environments are intentionally mutable during rule
// assembly, and callers commonly create an Engine before registering a named
// window; the first runtime lookup must still materialize that definition.
// The engine mutex must be held by the caller.
func (e *Engine) ensureNamedWindowLocked(name string) (*NamedWindow, bool) {
	return e.ensureNamedWindowLockedInModule("", name)
}

func (e *Engine) ensureNamedWindowLockedInModule(moduleName, name string) (*NamedWindow, bool) {
	if e == nil {
		return nil, false
	}
	key := catalogKey(moduleName, name)
	if window, ok := e.namedWindows[key]; ok {
		return window, true
	}
	if e.env == nil {
		return nil, false
	}
	e.env.mu.RLock()
	definition, ok := e.env.namedWindows[key]
	e.env.mu.RUnlock()
	if !ok {
		return nil, false
	}
	window := newNamedWindow(definition, e)
	e.namedWindows[key] = window
	e.registerNamedWindowMetricsLocked(definition.Name(), e.nextID+1)
	return window, true
}

func (e *Engine) InsertNamedWindow(ctx context.Context, name string, underlying any) error {
	return e.InsertNamedWindowInModule(ctx, "", name, underlying)
}

func (e *Engine) InsertNamedWindowInModule(ctx context.Context, moduleName, name string, underlying any) error {
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
	window, ok := e.ensureNamedWindowLockedInModule(moduleName, name)
	if !ok {
		e.mu.Unlock()
		return NewError(ErrorUnknownName, fmt.Sprintf("named window %q is not registered", name))
	}
	now := e.clock.Now()
	e.refreshVariablesLocked()
	variables := cloneValues(e.variables)
	e.pendingStatementDispatches = nil
	e.pendingNamedWindowDispatches = nil
	e.pendingRoutedEvents = nil
	e.pendingContextEvents = nil
	e.pendingVariableChanges = nil
	e.pendingMatchRecognizeStateLimits = nil
	e.pendingPatternSubexpressionLimits = nil
	e.pendingAuditRecords = nil
	delta, err := window.insertWithVariables(ctx, now, underlying, variables)
	if err != nil {
		e.mu.Unlock()
		return err
	}
	e.recordNamedWindowMetricInputLocked(window, delta)
	statements := e.dispatchStatementsLocked()
	dispatches := make([]statementDispatch, 0, len(statements))
	for _, statement := range statements {
		batch, changed, processErr := e.processNamedWindowWithMetricsLocked(ctx, statement, now, window, delta, variables)
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
			if statement.plan.query.statementDrop {
				break
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
	auditRecords, auditListeners := e.takeAuditDispatchLocked()
	e.pendingStatementDispatches = nil
	e.pendingNamedWindowDispatches = nil
	e.pendingRoutedEvents = nil
	e.mu.Unlock()
	if err := dispatchAuditRecords(ctx, auditRecords, auditListeners); err != nil {
		return err
	}
	e.dispatchVariableChanges(variableChanges)
	e.dispatchContextEvents(contextEvents)
	e.dispatchMatchRecognizeStateLimitEvents()
	e.dispatchPatternSubexpressionLimitEvents()
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
	deploymentOrder          uint64
	plan                     Plan
	parameters               ParameterValues
	id                       string
	name                     string
	runtime                  statementRuntime
	listeners                map[uint64]Listener
	subscriber               Subscriber
	nextSubID                uint64
	state                    StatementState
	closed                   bool
	closeOnce                sync.Once
	pendingOutputAssignments []VariableAssignment
	// replacedEvent carries the copy-on-write event produced by an
	// update-istream statement so the engine dispatch loop continues the
	// current cycle with the updated event for statements deployed later.
	replacedEvent *Event
	// droppedEvent marks that an update-istream statement with UpdateDrop
	// matched the in-flight event, so the engine drops the rest of the
	// current dispatch cycle for that event.
	droppedEvent bool
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

// Metadata returns a detached snapshot of the deployed statement's built-in
// and custom metadata.
func (s *Statement) Metadata() StatementMetadata {
	if s == nil {
		return StatementMetadata{}
	}
	return statementMetadataSnapshot(s.name, s.plan.query.statementMetadata)
}

// Annotation returns one named application-defined statement annotation.
func (s *Statement) Annotation(name string) (StatementAnnotation, bool) {
	if s == nil {
		return StatementAnnotation{}, false
	}
	return statementAnnotationByName(s.plan.query.statementMetadata.annotations, name)
}

// HasNoLock reports whether the statement carries the built-in NoLock
// instruction.
func (s *Statement) HasNoLock() bool {
	return s != nil && s.plan.query.statementMetadata.noLock
}

// Priority returns the ordinary continuous-statement dispatch priority.
// Higher values run first. The bool is false when no explicit priority was
// supplied and the effective priority is the default zero.
func (s *Statement) Priority() (int, bool) {
	if s == nil {
		return 0, false
	}
	return s.plan.query.statementPriority, s.plan.query.statementPrioritySet
}

// DropsLowerPriority reports whether this statement preempts lower-priority
// statements after it produces a result for a dispatch cycle.
func (s *Statement) DropsLowerPriority() bool {
	return s != nil && s.plan.query.statementDrop
}

// DeploymentID returns the stable deployment that owns this statement.
func (s *Statement) DeploymentID() string {
	if s == nil || s.deployment == nil {
		return ""
	}
	return s.deployment.id
}

// takeReplacedEvent returns and clears the copy-on-write event an
// update-istream statement produced for the in-flight dispatch cycle. The
// engine substitutes it for the remaining statements so downstream consumers
// observe the updated event while earlier statements already saw the
// original, mirroring Esper's InternalEventRouter preprocessing.
func (s *Statement) takeReplacedEvent() (Event, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.replacedEvent == nil {
		return Event{}, false
	}
	event := *s.replacedEvent
	s.replacedEvent = nil
	return event, true
}

// takeDroppedEvent returns and clears the drop signal an update-istream
// statement with UpdateDrop raised for the in-flight dispatch cycle.
func (s *Statement) takeDroppedEvent() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	dropped := s.droppedEvent
	s.droppedEvent = false
	return dropped
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
	if s.engine != nil {
		s.engine.mu.Lock()
		defer s.engine.mu.Unlock()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.state == StatementDestroyed {
		return QueryResult{}, NewError(ErrorState, "statement is destroyed")
	}
	if joinDefinitionHasUnidirectional(s.plan.query.join) || (s.plan.query.aggregate != nil && joinDefinitionHasUnidirectional(s.plan.query.aggregate.join)) {
		return QueryResult{}, fmt.Errorf("esper: iteration over a unidirectional join is not supported")
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
	return QueryResult{Batch: s.snapshotLocked(now, variables, selector)}, nil
}

// snapshotLocked evaluates the statement iterator while the caller owns the
// statement lock and, for live statements, the engine lock. Keeping this
// small boundary shared by Snapshot and SubscribeWithReplay makes listener
// registration and replay one consistent statement-state transition.
func (s *Statement) snapshotLocked(now time.Time, variables map[string]Value, selector ContextPartitionSelector) ResultBatch {
	if s.plan.query.contextName == "" && s.runtime.subqueryRegistry != nil {
		// Snapshot rebuilds the evaluation variables instead of reusing the
		// variables attached during event processing. Keep the statement-owned
		// event-stream subquery state visible so iterator results agree with the
		// listener path for unbounded/grouped subqueries.
		variables = s.runtime.subqueryRegistry.attachVariables(variables)
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
			partVariables := variables
			if s.plan.query.contextName != "" {
				partVariables = s.contextPartitionVariables(partition, variables)
			}
			partBatch := partition.snapshotQuery(s.plan, now, partVariables)
			result.New = append(result.New, partBatch.New...)
			result.Old = append(result.Old, partBatch.Old...)
		}
	} else {
		result = s.runtime.snapshotQuery(s.plan, now, variables)
	}
	return result
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
		return Subscription{}, NewError(ErrorState, "nil statement")
	}
	if listener == nil {
		return Subscription{}, NewError(ErrorInvalidRule, "listener is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.state == StatementDestroyed {
		return Subscription{}, NewError(ErrorState, "statement is destroyed")
	}
	s.nextSubID++
	id := s.nextSubID
	s.listeners[id] = listener
	return Subscription{statement: s, id: id}, nil
}

// SetSubscriber replaces the statement's single subscriber. Passing nil
// removes it. Subscribers coexist with any number of listeners and receive
// the same logical batches through the typed SubscriberUpdate contract.
func (s *Statement) SetSubscriber(subscriber Subscriber) error {
	if s == nil {
		return NewError(ErrorState, "nil statement")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.state == StatementDestroyed {
		return NewError(ErrorState, "statement is destroyed")
	}
	if subscriber != nil && s.plan.query.subscriberDisallowed {
		return NewError(ErrorInvalidRule, "setting a subscriber is not allowed for this statement")
	}
	s.subscriber = subscriber
	return nil
}

func (s *Statement) HasSubscriber() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.subscriber != nil
}

// SubscribeWithReplay registers a listener and synchronously delivers the
// statement's current iterator snapshot as its first batch. An empty
// statement still produces one empty replay batch. Events processed while the
// replay callback runs are queued and delivered afterwards in separate live
// batches, so no update is lost or folded into the snapshot.
//
// Like Esper's addListenerWithReplay, this operation is unavailable for
// context statements; use SnapshotWithSelector followed by Subscribe when a
// caller needs an explicitly selected context view.
func (s *Statement) SubscribeWithReplay(ctx context.Context, listener Listener) (Subscription, error) {
	if err := contextErr(ctx); err != nil {
		return Subscription{}, err
	}
	if s == nil {
		return Subscription{}, NewError(ErrorState, "nil statement")
	}
	if listener == nil {
		return Subscription{}, NewError(ErrorInvalidRule, "listener is nil")
	}
	engineLocked := s.engine != nil
	if engineLocked {
		s.engine.mu.Lock()
	}
	s.mu.Lock()
	if s.closed || s.state == StatementDestroyed {
		s.mu.Unlock()
		if engineLocked {
			s.engine.mu.Unlock()
		}
		return Subscription{}, NewError(ErrorState, "statement is destroyed")
	}
	if s.plan.query.contextName != "" {
		s.mu.Unlock()
		if engineLocked {
			s.engine.mu.Unlock()
		}
		return Subscription{}, NewError(ErrorInvalidRule, "subscribe with replay is not available for context statements")
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
	replay := s.snapshotLocked(now, variables, nil)
	buffer := newReplayListener(listener)
	s.nextSubID++
	id := s.nextSubID
	s.listeners[id] = buffer.deliver
	s.mu.Unlock()

	// Release the engine lock before entering application code. The buffering
	// wrapper preserves replay-first ordering and allows reentrant sends.
	if engineLocked {
		s.engine.mu.Unlock()
	}
	if err := buffer.finish(ctx, replay); err != nil {
		_ = s.removeSubscription(id)
		return Subscription{}, fmt.Errorf("esper: replay listener for %q: %w", s.name, err)
	}
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

// Statement returns the statement with the given name within this deployment.
// Statement names are unique inside one deployment but may be reused by a
// different deployment.
func (d *Deployment) Statement(name string) (*Statement, bool) {
	if d == nil {
		return nil, false
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, statement := range d.statements {
		if statement != nil && statement.name == name {
			return statement, true
		}
	}
	return nil, false
}

func (d *Deployment) Undeploy(ctx context.Context) error {
	if d == nil || d.engine == nil {
		return nil
	}
	return d.engine.Undeploy(ctx, d.id)
}

// DeploymentStatementNameContext describes one plan while a multi-plan
// deployment resolves its runtime statement names.
type DeploymentStatementNameContext struct {
	Index        int
	Plan         Plan
	OriginalName string
}

// DeploymentStatementNameResolver selects the runtime name for one plan.
// Returning an error aborts the deployment before any statement is visible.
type DeploymentStatementNameResolver func(DeploymentStatementNameContext) (string, error)

type deploymentConfig struct {
	nameResolver DeploymentStatementNameResolver
}

// DeploymentOption configures a DeployPlans operation.
type DeploymentOption func(*deploymentConfig)

// WithDeploymentStatementNameResolver overrides plan names at deployment
// time. Resolved names must be non-blank and unique within the deployment.
func WithDeploymentStatementNameResolver(resolver DeploymentStatementNameResolver) DeploymentOption {
	return func(config *deploymentConfig) { config.nameResolver = resolver }
}

type deploymentRequest struct {
	plan          Plan
	parameters    ParameterValues
	parameterized bool
	name          string
}

func (e *Engine) Deploy(ctx context.Context, plan Plan) (*Deployment, error) {
	return e.deployRequests(ctx, []deploymentRequest{{plan: plan}}, nil)
}

// DeployWithParameters deploys a live statement with one immutable binding
// snapshot for its substitution parameters. The values are copied at deploy
// time and are not retained through the caller's map after this call returns.
func (e *Engine) DeployWithParameters(ctx context.Context, plan Plan, parameters ParameterValues) (*Deployment, error) {
	return e.deployRequests(ctx, []deploymentRequest{{plan: plan, parameters: parameters, parameterized: true}}, nil)
}

// DeployPlans deploys multiple immutable plans as one deployment. Plans keep
// declaration order for dispatch and management traversal. Unnamed plans use
// stmt-0, stmt-1 and so on, matching Esper module deployment names.
func (e *Engine) DeployPlans(ctx context.Context, plans []Plan, options ...DeploymentOption) (*Deployment, error) {
	config := deploymentConfig{}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	requests := make([]deploymentRequest, len(plans))
	for index, plan := range plans {
		requests[index] = deploymentRequest{plan: plan}
	}
	return e.deployRequests(ctx, requests, config.nameResolver)
}

func (e *Engine) deployRequests(ctx context.Context, requests []deploymentRequest, resolver DeploymentStatementNameResolver) (*Deployment, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if e == nil || e.env == nil {
		return nil, NewError(ErrorDependency, "engine has no environment")
	}
	if len(requests) == 0 {
		return nil, NewError(ErrorInvalidRule, "deployment requires at least one plan")
	}
	seenNames := make(map[string]struct{}, len(requests))
	for index := range requests {
		request := &requests[index]
		if request.plan.query.env != e.env || request.plan.schemaVersion == "" {
			return nil, NewError(ErrorDependency, fmt.Sprintf("plan %d does not belong to this engine", index))
		}
		parameterTypes, parameterErr := queryParameterTypes(e.env, request.plan.query)
		if parameterErr != nil {
			return nil, WrapError(ErrorInvalidRule, fmt.Sprintf("plan %d parameters", index), parameterErr)
		}
		if len(parameterTypes) > 0 && !request.parameterized {
			return nil, NewError(ErrorInvalidRule, fmt.Sprintf("plan %d contains substitution parameters; use DeployWithParameters", index))
		}
		if request.parameterized {
			if err := validateParameterBindings(parameterTypes, request.parameters); err != nil {
				return nil, err
			}
		}
		name := request.plan.query.name
		if resolver != nil {
			resolved, err := resolver(DeploymentStatementNameContext{Index: index, Plan: request.plan, OriginalName: name})
			if err != nil {
				return nil, WrapError(ErrorDeployment, fmt.Sprintf("resolve statement name for plan %d", index), err)
			}
			name = resolved
		}
		if name == "" && resolver == nil {
			name = fmt.Sprintf("stmt-%d", index)
		}
		if strings.TrimSpace(name) == "" {
			return nil, NewError(ErrorDeployment, fmt.Sprintf("statement name resolver returned a blank name for plan %d", index))
		}
		if _, exists := seenNames[name]; exists {
			return nil, NewError(ErrorDeployment, fmt.Sprintf("duplicate statement name %q within deployment", name))
		}
		seenNames[name] = struct{}{}
		request.name = name
		if request.plan.query.contextName != "" {
			if _, ok := e.env.Context(request.plan.query.contextName); !ok {
				return nil, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", request.plan.query.contextName))
			}
		}
	}
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, NewError(ErrorState, "engine is closed")
	}
	e.nextID++
	deploymentID := fmt.Sprintf("deployment-%d", e.nextID)
	deployment := &Deployment{engine: e, id: deploymentID, statements: make([]*Statement, 0, len(requests))}
	tableSnapshots := make(map[string]tableMutationSnapshot)
	for _, request := range requests {
		if target := request.plan.query.tableTarget; target != "" {
			if table := e.tables[target]; table != nil {
				if _, exists := tableSnapshots[target]; !exists {
					tableSnapshots[target] = table.snapshotMutationState()
				}
			}
		}
	}
	for index, request := range requests {
		if index > 0 {
			e.nextID++
		}
		statement, err := e.prepareStatementLocked(ctx, deployment, request, e.nextID)
		if err != nil {
			for name, snapshot := range tableSnapshots {
				if table := e.tables[name]; table != nil {
					table.restoreMutationState(snapshot)
				}
			}
			for _, prepared := range deployment.statements {
				e.cleanupPreparedStatementLocked(prepared)
			}
			e.mu.Unlock()
			return nil, err
		}
		deployment.statements = append(deployment.statements, statement)
	}
	for _, statement := range deployment.statements {
		e.statements[statement.id] = statement
		e.registerStatementMetricsLocked(statement)
	}
	e.deployments[deploymentID] = deployment
	for _, statement := range deployment.statements {
		contextName := statement.plan.query.contextName
		if contextName == "" {
			continue
		}
		if definition, ok := e.env.Context(contextName); ok && definition.isTemporal() {
			if _, exists := e.contextTemporalOrigins[contextName]; !exists {
				e.contextTemporalOrigins[contextName] = e.clock.Now()
			}
		}
		e.ensureContextCreatedLocked(contextName)
		e.queueContextStatementAddedLocked(statement)
		if definition, ok := e.env.Context(contextName); ok && definition.isTemporal() {
			_, _ = statement.syncTemporalContextLocked(e.clock.Now())
		}
	}
	contextEvents := e.takeContextEventsLocked()
	auditRecords, auditListeners := e.takeAuditDispatchLocked()
	e.mu.Unlock()
	if err := dispatchAuditRecords(ctx, auditRecords, auditListeners); err != nil {
		return nil, err
	}
	e.dispatchContextEvents(contextEvents)
	for _, statement := range deployment.statements {
		e.notifyDataflowStatementDeployed(statement)
	}
	return deployment, nil
}

func (e *Engine) prepareStatementLocked(ctx context.Context, deployment *Deployment, request deploymentRequest, deploymentOrder uint64) (*Statement, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	plan := request.plan
	name := request.name
	runtimeQuery := plan.query
	runtimeQuery.name = name
	statement := &Statement{
		engine:          e,
		deployment:      deployment,
		deploymentOrder: deploymentOrder,
		id:              deployment.id + ":" + name,
		name:            name,
		plan:            plan,
		parameters:      cloneParameterValues(request.parameters),
		listeners:       make(map[uint64]Listener),
		state:           StatementStarted,
		runtime:         newStatementRuntime(runtimeQuery),
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
	if plan.query.tableTarget != "" {
		if err := statement.runtime.persistAggregateTable(plan, e.clock.Now()); err != nil {
			e.cleanupPreparedStatementLocked(statement)
			return nil, err
		}
	}
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
		e.cleanupPreparedStatementLocked(statement)
		return nil, err
	}
	if err := e.seedNamedWindowConsumerLocked(ctx, statement); err != nil {
		e.cleanupPreparedStatementLocked(statement)
		return nil, err
	}
	if err := e.seedEvaluateOnceJoinSidesLocked(ctx, statement); err != nil {
		e.cleanupPreparedStatementLocked(statement)
		return nil, err
	}
	return statement, nil
}

func (e *Engine) cleanupPreparedStatementLocked(statement *Statement) {
	if statement == nil {
		return
	}
	e.releaseRowRecogRuntimeLocked(&statement.runtime)
	e.matchRecognizeStatePool.removeOwner(statement.id)
	statement.closed = true
	statement.state = StatementDestroyed
	statement.listeners = make(map[uint64]Listener)
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

// seedNamedWindowConsumerLocked replays the retained contents of a named
// window into a newly deployed consumer statement. Esper attaches plain,
// aggregate and join consumers to the current named-window state, so a
// late-deployed irstream aggregate reports its seeded value as the prior old
// row and a left outer join iterator sees retained rows with null join
// columns immediately. The replay deliberately discards every resulting
// batch; deployment must not announce historical rows as new output.
//
// Like the row-recognition replay this is limited to non-context statements;
// pattern, trigger and row-recognition plans keep their own lifecycles.
func (e *Engine) seedNamedWindowConsumerLocked(ctx context.Context, statement *Statement) error {
	if e == nil || statement == nil {
		return nil
	}
	query := statement.plan.query
	if query.trigger != nil || query.contextName != "" || query.pattern != nil || query.rowRecog != nil {
		return nil
	}
	var sources []*streamNode
	switch {
	case query.aggregate != nil && query.aggregate.join != nil:
		sources = joinDefinitionSources(query.aggregate.join)
	case query.aggregate != nil:
		sources = []*streamNode{query.aggregate.input}
	case query.join != nil:
		sources = joinDefinitionSources(query.join)
	default:
		sources = []*streamNode{query.input}
	}
	plain := query.aggregate == nil && query.join == nil
	trackPrior := plain && queryUsesPreviousAccess(query)
	now := e.clock.Now()
	replayed := make(map[string]struct{})
	for _, source := range sources {
		base, err := sourceNode(source)
		if err != nil || base == nil || base.kind != streamNamedWindow {
			continue
		}
		if _, duplicate := replayed[base.sourceName]; duplicate {
			continue
		}
		replayed[base.sourceName] = struct{}{}
		if definition, ok := e.env.NamedWindowInModule(base.moduleName, base.sourceName); ok && namedWindowRetentionIsBatch(definition.retention) {
			// Esper skips the named-window consumer preload when the parent
			// window batches (time_batch, length_batch, time_length_batch,
			// firsttime, ext_timed_batch, expression_batch): the late consumer
			// first observes the completed batch as new data at the boundary.
			continue
		}
		events, err := e.snapshotFireAndForgetSourceLocked(ctx, base, now, statement.runtime.variables)
		if err != nil {
			return err
		}
		statement.runtime.ctx = ctx
		for _, event := range events {
			if _, _, err := statement.runtime.process(statement.plan, event, now, statement.runtime.variables); err != nil {
				return err
			}
			if trackPrior {
				statement.runtime.namedWindowArrival = append(statement.runtime.namedWindowArrival, event)
			}
		}
	}
	return nil
}

// namedWindowRetentionIsBatch reports whether a named-window retention spec
// batches deliveries. Esper marks windows whose view chain contains a batching
// data window (time_batch, length_batch, time_length_batch, firsttime,
// ext_timed_batch or expression_batch) and skips the consumer preload for
// them; a grouped retention defers to its inner data window.
func namedWindowRetentionIsBatch(spec WindowSpec) bool {
	if grouped, ok := spec.(GroupWindowSpec); ok {
		return namedWindowRetentionIsBatch(grouped.Inner)
	}
	switch window := spec.(type) {
	case TimeBatchWindowSpec, LengthBatchWindowSpec, TimeLengthBatchWindowSpec, FirstTimeWindowSpec, ExpressionBatchWindowSpec:
		return true
	case ExternallyTimedWindowSpec:
		return window.Batch
	default:
		return false
	}
}

// seedEvaluateOnceJoinSidesLocked polls join method sources marked
// EvaluateOnce exactly once at deployment, mirroring Esper's statement-start
// evaluation of method/historical streams whose arguments reference no other
// stream. The rows are retained in the join state like a read-only data
// window, so a full/right outer join iterator sees unmatched placeholder rows
// immediately after Deploy returns. Seeding deliberately does not emit a
// listener batch; deployment must not announce historical rows as new output.
func (e *Engine) seedEvaluateOnceJoinSidesLocked(ctx context.Context, statement *Statement) error {
	if e == nil || statement == nil {
		return nil
	}
	var definitions []*joinDefinition
	if statement.plan.query.join != nil {
		definitions = append(definitions, statement.plan.query.join)
	}
	if statement.plan.query.aggregate != nil && statement.plan.query.aggregate.join != nil {
		definitions = append(definitions, statement.plan.query.aggregate.join)
	}
	if len(definitions) == 0 {
		return nil
	}
	now := e.clock.Now()
	for _, definition := range definitions {
		sources := joinDefinitionSources(definition)
		order, orderErr := methodJoinEvaluationOrder(definition)
		if orderErr != nil {
			return orderErr
		}
		initialized := false
		seeded := make([]bool, len(sources))
		ensureInitialized := func() {
			if initialized {
				return
			}
			if statement.runtime.joinState == nil {
				statement.runtime.joinState = &joinRuntimeState{}
			}
			if len(statement.runtime.joinState.sides) != len(sources) {
				statement.runtime.joinState.sides = make([][]storedEvent, len(sources))
			}
			initialized = true
		}
		for _, index := range order {
			source := sources[index]
			base, err := sourceNode(source)
			if err != nil {
				return err
			}
			if base == nil || base.kind != streamMethod || base.method == nil {
				continue
			}
			if base.method.evaluateOnce {
				ensureInitialized()
				events, err := base.method.provider.Poll(ctx, MethodRequest{
					Now:        now,
					Variables:  visibleVariableValues(statement.runtime.variables),
					Parameters: parameterValuesFromVariables(statement.runtime.variables),
					Invocation: statement.runtime.methodInvocationContext(base.sourceName),
				})
				if err != nil {
					return err
				}
				side := make([]storedEvent, 0, len(events))
				for _, event := range events {
					side = append(side, storedEvent{event: event, receivedAt: now, lineageID: statement.runtime.nextJoinLineageID()})
				}
				statement.runtime.joinState.sides[index] = side
				seeded[index] = true
				continue
			}
			if base.method.trigger != "" || len(base.method.dependencies) == 0 {
				continue
			}
			// A triggerless subordinate method source whose dependencies were
			// all seeded at deployment is itself evaluated once at deployment:
			// Esper polls the subordinate at statement start per dependency row
			// and retains the rows for the iterator.
			dependenciesSeeded := true
			for _, dependency := range base.method.dependencies {
				dependencyIndex := joinSourceIndexByName(sources, dependency)
				if dependencyIndex < 0 || !seeded[dependencyIndex] {
					dependenciesSeeded = false
					break
				}
			}
			if !dependenciesSeeded {
				continue
			}
			ensureInitialized()
			invocations, err := methodDependencyInvocations(base.method.dependencies, sources, statement.runtime.joinState)
			if err != nil {
				return err
			}
			side := make([]storedEvent, 0, len(invocations))
			for _, invocation := range invocations {
				events, err := base.method.provider.Poll(ctx, MethodRequest{
					Now:          now,
					Variables:    visibleVariableValues(statement.runtime.variables),
					Parameters:   parameterValuesFromVariables(statement.runtime.variables),
					Dependencies: cloneMethodDependencies(invocation.events),
					Invocation:   statement.runtime.methodInvocationContext(base.sourceName),
				})
				if err != nil {
					return err
				}
				for _, event := range events {
					side = append(side, storedEvent{
						event: event, receivedAt: now,
						lineageID: statement.runtime.nextJoinLineageID(),
						lineage:   cloneMethodLineage(invocation.lineage),
					})
				}
			}
			statement.runtime.joinState.sides[index] = side
			seeded[index] = true
		}
	}
	return nil
}

// joinSourceIndexByName resolves a join source's logical name back to its
// declaration index, or -1 when the name is unknown.
func joinSourceIndexByName(sources []*streamNode, name string) int {
	trimmed := strings.TrimSpace(name)
	for index, source := range sources {
		base, err := sourceNode(source)
		if err != nil || base == nil {
			continue
		}
		if base.logicalName() == trimmed {
			return index
		}
	}
	return -1
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
		e.removeStatementMetricsLocked(statement)
		delete(e.statements, statement.id)
		statement.markClosedLocked()
	}
	deployment.mu.Lock()
	deployment.closed = true
	deployment.mu.Unlock()
	contextEvents := e.takeContextEventsLocked()
	auditRecords, auditListeners := e.takeAuditDispatchLocked()
	e.mu.Unlock()
	if err := dispatchAuditRecords(ctx, auditRecords, auditListeners); err != nil {
		return err
	}
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
		s.subscriber = nil
		s.mu.Unlock()
	})
}

// SetSubscriberErrorHandler replaces the engine-wide subscriber failure
// observer. Subscriber callback errors and panics never fail Send/Route or
// prevent listeners and sinks from receiving the same batch.
func (e *Engine) SetSubscriberErrorHandler(handler SubscriberErrorHandler) {
	if e == nil {
		return
	}
	e.subscriberErrorMu.Lock()
	e.subscriberErrorHandler = handler
	e.subscriberErrorMu.Unlock()
}

func (e *Engine) reportSubscriberError(ctx context.Context, failure SubscriberError) {
	if e == nil {
		return
	}
	e.subscriberErrorMu.RLock()
	handler := e.subscriberErrorHandler
	e.subscriberErrorMu.RUnlock()
	if handler == nil {
		return
	}
	func() {
		defer func() { _ = recover() }()
		handler(ctx, failure)
	}()
}

func (e *Engine) Send(ctx context.Context, eventType string, underlying any) error {
	return e.send(ctx, eventType, underlying, nil)
}

// SetUnmatchedListener replaces the engine-wide unmatched-event callback.
// Passing nil disables delivery. Callbacks run after the engine transaction
// lock is released, allowing a callback to deploy a statement that can match
// the next event.
func (e *Engine) SetUnmatchedListener(listener UnmatchedListener) {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.unmatchedListener = listener
	e.mu.Unlock()
}

func (e *Engine) send(ctx context.Context, eventType string, underlying any, jsonRaw any) error {
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
	event.jsonRaw = jsonRaw
	e.refreshVariablesLocked()
	variables := cloneValues(e.variables)
	e.pendingStatementDispatches = nil
	e.pendingNamedWindowDispatches = nil
	e.pendingRoutedEvents = nil
	e.pendingContextEvents = nil
	e.pendingVariableChanges = nil
	e.pendingMatchRecognizeStateLimits = nil
	e.pendingPatternSubexpressionLimits = nil
	e.pendingAuditRecords = nil
	dispatches := make([]statementDispatch, 0)
	routedQueue := []Event{event}
	processedEvents := make([]Event, 0, 1)
	unmatchedEvents := make([]Event, 0, 1)
	processedRoutes := 0
	for len(routedQueue) > 0 {
		if err := contextErr(ctx); err != nil {
			e.mu.Unlock()
			return err
		}
		current := routedQueue[0]
		routedQueue = routedQueue[1:]
		e.recordRuntimeInputLocked()
		processedEvents = append(processedEvents, current)
		if processedRoutes >= maxRoutedEventsPerSend {
			e.mu.Unlock()
			return NewError(ErrorState, fmt.Sprintf("route event limit %d exceeded", maxRoutedEventsPerSend))
		}
		processedRoutes++
		statements := e.dispatchStatementsLocked()
		matched := false
		for _, statement := range orderUpdateStatementsFirst(statements) {
			accepted := statement.matchesEventFilter(current, now, variables)
			batch, changed, processErr := e.processStatementWithMetricsLocked(ctx, statement, now, current, variables, accepted)
			if processErr != nil {
				e.mu.Unlock()
				return processErr
			}
			if accepted {
				matched = true
			}
			if changed {
				matched = true
			}
			if replaced, replacedOK := statement.takeReplacedEvent(); replacedOK {
				current = replaced
			}
			if statement.takeDroppedEvent() {
				break
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
				if statement.plan.query.statementDrop {
					break
				}
			}
		}
		dispatches = append(dispatches, e.pendingStatementDispatches...)
		e.pendingStatementDispatches = nil
		routedQueue = append(routedQueue, e.pendingRoutedEvents...)
		e.pendingRoutedEvents = nil
		if !matched {
			unmatchedEvents = append(unmatchedEvents, current)
		}
	}
	nestedNamedWindowDispatches := append([]namedWindowDispatch(nil), e.pendingNamedWindowDispatches...)
	variableChanges := e.takeVariableChangesLocked()
	contextEvents := e.takeContextEventsLocked()
	auditRecords, auditListeners := e.takeAuditDispatchLocked()
	unmatchedListener := e.unmatchedListener
	e.pendingStatementDispatches = nil
	e.pendingNamedWindowDispatches = nil
	e.pendingRoutedEvents = nil
	e.mu.Unlock()
	if err := dispatchAuditRecords(ctx, auditRecords, auditListeners); err != nil {
		return err
	}
	if unmatchedListener != nil {
		for _, unmatched := range unmatchedEvents {
			if err := unmatchedListener(ctx, unmatched); err != nil {
				return fmt.Errorf("esper: unmatched listener for %q: %w", unmatched.TypeName(), err)
			}
		}
	}
	e.dispatchVariableChanges(variableChanges)
	e.dispatchContextEvents(contextEvents)
	e.dispatchMatchRecognizeStateLimitEvents()
	e.dispatchPatternSubexpressionLimitEvents()
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
// entering the normal Send path; Avro schemas become schema-bound AvroRecord
// values while map/JSON/XML schemas retain the map. This is
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
	sender, err := e.JSONSender(eventType)
	if err != nil {
		return err
	}
	return sender.Send(ctx, data)
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

// SendAvro sends a native schema-bound Avro record.
func (e *Engine) SendAvro(ctx context.Context, eventType string, record *AvroRecord) error {
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	schema, ok := e.env.Schema(eventType)
	if !ok || schema.Kind() != SchemaAvro {
		return NewError(ErrorUnknownName, fmt.Sprintf("Avro event type %q is not registered", eventType))
	}
	if record == nil {
		return NewError(ErrorTypeMismatch, fmt.Sprintf("Avro event type %q record is nil", eventType))
	}
	if !avroSchemasEqual(schema, record.Schema()) {
		return NewError(ErrorTypeMismatch, fmt.Sprintf("Avro event type %q received record for schema %q", eventType, record.Schema().Name()))
	}
	return e.Send(ctx, eventType, record)
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

// StatementSchedule identifies the nearest pending engine-clock callback for
// one deployed statement. At is expressed in the same time domain as Now and
// AdvanceTime; DeploymentID and StatementName form a stable lookup pair.
type StatementSchedule struct {
	DeploymentID  string
	StatementName string
	At            time.Time
}

// StatementNearestSchedules returns one nearest pending callback per active
// statement, ordered by time, statement name and deployment id. Statements
// without a pending engine-clock callback are omitted.
func (e *Engine) StatementNearestSchedules(ctx context.Context) ([]StatementSchedule, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if e == nil {
		return nil, NewError(ErrorDependency, "nil engine")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, NewError(ErrorState, "engine is closed")
	}
	statements := e.sortedStatementsLocked()
	result := make([]StatementSchedule, 0, len(statements))
	for _, statement := range statements {
		statement.mu.RLock()
		if !statement.closed && statement.state == StatementStarted {
			if at, ok := statementRuntimeNearestSchedule(&statement.runtime, statement.plan.query); ok {
				result = append(result, StatementSchedule{
					DeploymentID:  statement.deployment.id,
					StatementName: statement.name,
					At:            at,
				})
			}
		}
		statement.mu.RUnlock()
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].At.Equal(result[j].At) {
			return result[i].At.Before(result[j].At)
		}
		if result[i].StatementName != result[j].StatementName {
			return result[i].StatementName < result[j].StatementName
		}
		return result[i].DeploymentID < result[j].DeploymentID
	})
	return result, nil
}

// NextScheduledTime returns the earliest pending statement callback. The bool
// result is false when no active statement has a pending engine-clock task.
func (e *Engine) NextScheduledTime(ctx context.Context) (time.Time, bool, error) {
	schedules, err := e.StatementNearestSchedules(ctx)
	if err != nil {
		return time.Time{}, false, err
	}
	if len(schedules) == 0 {
		return time.Time{}, false, nil
	}
	return schedules[0].At, true, nil
}

// AdvanceTimeSpan advances to every due statement schedule up to target,
// preserving one listener callback boundary per due time. When resolution is
// supplied, the clock advances in fixed sampling steps instead: overdue
// callbacks fire at the first sampled time at or after their deadline and
// recurring timers re-anchor from that sampled time, matching Esper's
// advanceTimeSpan(target, resolution) behavior.
func (e *Engine) AdvanceTimeSpan(ctx context.Context, target time.Time, resolution ...time.Duration) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if e == nil {
		return NewError(ErrorDependency, "nil engine")
	}
	if len(resolution) > 1 {
		return NewError(ErrorInvalidRule, "advance time span accepts at most one resolution")
	}
	current := e.Now()
	if target.Before(current) {
		return fmt.Errorf("esper: clock cannot move backwards from %s to %s", current, target)
	}
	if len(resolution) == 1 {
		step := resolution[0]
		if step <= 0 {
			return NewError(ErrorInvalidRule, "advance time span resolution must be positive")
		}
		for current.Before(target) {
			next := current.Add(step)
			if next.After(target) {
				next = target
			}
			if err := e.advanceTime(ctx, next, true); err != nil {
				return err
			}
			current = next
		}
		return nil
	}
	for {
		next, ok, err := e.NextScheduledTime(ctx)
		if err != nil {
			return err
		}
		if !ok || next.After(target) {
			break
		}
		current := e.Now()
		if next.Before(current) {
			next = current
		}
		if err := e.advanceTime(ctx, next, false); err != nil {
			return err
		}
	}
	if e.Now().Before(target) {
		return e.advanceTime(ctx, target, false)
	}
	return nil
}

func (e *Engine) AdvanceTime(ctx context.Context, at time.Time) error {
	return e.advanceTime(ctx, at, false)
}

func (e *Engine) advanceTime(ctx context.Context, at time.Time, coalesceSchedules bool) error {
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
	statements := e.dispatchStatementsLocked()
	if coalesceSchedules {
		for _, statement := range statements {
			statement.coalesceSchedulesLocked(at)
		}
	}
	e.pendingStatementDispatches = nil
	e.pendingNamedWindowDispatches = nil
	e.pendingRoutedEvents = nil
	e.pendingContextEvents = nil
	e.pendingVariableChanges = nil
	e.pendingMatchRecognizeStateLimits = nil
	e.pendingPatternSubexpressionLimits = nil
	e.pendingAuditRecords = nil
	dispatches := make([]statementDispatch, 0, len(statements))
	namedWindowDispatches := make([]namedWindowDispatch, 0, len(e.namedWindows))
	for _, name := range sortedNamedWindowNames(e.namedWindows) {
		window := e.namedWindows[name]
		if delta := window.expire(at); !delta.empty() {
			e.recordNamedWindowMetricInputLocked(window, delta)
			namedWindowDispatches = append(namedWindowDispatches, namedWindowDispatch{window: window, delta: delta})
			for _, statement := range statements {
				batch, changed, processErr := e.processNamedWindowWithMetricsLocked(ctx, statement, at, window, delta, variables)
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
					if statement.plan.query.statementDrop {
						break
					}
				}
			}
		}
	}
	for _, statement := range statements {
		batch, changed := e.expireStatementWithMetricsLocked(statement, at, variables)
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
			if statement.plan.query.statementDrop {
				break
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
	auditRecords, auditListeners := e.takeAuditDispatchLocked()
	e.pendingStatementDispatches = nil
	e.pendingNamedWindowDispatches = nil
	e.pendingRoutedEvents = nil
	dataflows := make([]*DataflowInstance, 0, len(e.dataflows))
	for instance := range e.dataflows {
		dataflows = append(dataflows, instance)
	}
	runtimeMetric, runtimeMetricListeners, runtimeMetricDue := e.runtimeMetricDueLocked(at)
	statementMetrics, statementMetricListeners := e.statementMetricsDueLocked(at)
	e.mu.Unlock()
	if err := dispatchAuditRecords(ctx, auditRecords, auditListeners); err != nil {
		return err
	}
	e.dispatchVariableChanges(variableChanges)
	e.dispatchContextEvents(contextEvents)
	e.dispatchMatchRecognizeStateLimitEvents()
	e.dispatchPatternSubexpressionLimitEvents()
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
	if runtimeMetricDue {
		if err := dispatchRuntimeMetric(ctx, runtimeMetric, runtimeMetricListeners); err != nil {
			return err
		}
	}
	if err := dispatchStatementMetrics(ctx, statementMetrics, statementMetricListeners); err != nil {
		return err
	}
	return nil
}

// orderUpdateStatementsFirst moves update-istream statements ahead of all
// other statements for one event dispatch cycle, keeping deployment order
// within each group. Esper applies update-istream as InternalEventRouter
// preprocessing when the event enters the stream, so consumers observe the
// updated event even when the update statement was deployed after them.
func orderUpdateStatementsFirst(statements []*Statement) []*Statement {
	hasUpdate := false
	for _, statement := range statements {
		if statement.plan.query.updateStream != nil {
			hasUpdate = true
			break
		}
	}
	if !hasUpdate {
		return statements
	}
	updates := make([]*Statement, 0, len(statements))
	for _, statement := range statements {
		if statement.plan.query.updateStream != nil {
			updates = append(updates, statement)
		}
	}
	// Esper sorts update-istream entries by ascending priority with drop
	// entries first on ties; deployment order stays the stable fallback.
	sort.SliceStable(updates, func(i, j int) bool {
		left := updates[i].plan.query.updateStream
		right := updates[j].plan.query.updateStream
		if left.priority != right.priority {
			return left.priority < right.priority
		}
		if left.drop != right.drop {
			return left.drop
		}
		return false
	})
	ordered := make([]*Statement, 0, len(statements))
	ordered = append(ordered, updates...)
	for _, statement := range statements {
		if statement.plan.query.updateStream == nil {
			ordered = append(ordered, statement)
		}
	}
	return ordered
}

func (e *Engine) sortedStatementsLocked() []*Statement {
	statements := make([]*Statement, 0, len(e.statements))
	for _, statement := range e.statements {
		statements = append(statements, statement)
	}
	// Catalog traversal follows deployment order. Names remain a deterministic
	// fallback for legacy or externally constructed statements that do not
	// carry an order.
	sort.SliceStable(statements, func(i, j int) bool {
		if statements[i].deploymentOrder != statements[j].deploymentOrder {
			return statements[i].deploymentOrder < statements[j].deploymentOrder
		}
		return statements[i].name < statements[j].name
	})
	return statements
}

// dispatchStatementsLocked returns the continuous-statement order for one
// runtime cycle without changing deployment-ordered management traversal.
// Higher priorities run first. At equal priority a drop statement precedes
// non-drop statements, and stable deployment order breaks all remaining ties.
func (e *Engine) dispatchStatementsLocked() []*Statement {
	statements := e.sortedStatementsLocked()
	sort.SliceStable(statements, func(i, j int) bool {
		left := statements[i].plan.query
		right := statements[j].plan.query
		if left.statementPriority != right.statementPriority {
			return left.statementPriority > right.statementPriority
		}
		if left.statementDrop != right.statementDrop {
			return left.statementDrop
		}
		return false
	})
	return statements
}

func (e *Engine) Close(ctx context.Context) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if e == nil {
		return nil
	}
	if err := e.shutdownThreading(ctx); err != nil {
		return err
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
	e.recordNamedWindowMetricInputLocked(window, delta)
	statements := e.dispatchStatementsLocked()
	for _, statement := range statements {
		if statement == owner || !containsNamedWindow(statement.plan.query.input, statement.plan.query.join) {
			continue
		}
		batch, changed, err := e.processNamedWindowWithMetricsLocked(ctx, statement, now, window, delta, variables)
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
			if statement.plan.query.statementDrop {
				break
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
		e.recordRuntimeInputLocked()
		for _, statement := range orderUpdateStatementsFirst(e.dispatchStatementsLocked()) {
			needsAccepted := e.statementMetrics != nil || len(statement.plan.query.statementMetadata.auditCategories) > 0
			accepted := needsAccepted && statement.matchesEventFilter(current, now, variables)
			batch, changed, err := e.processStatementWithMetricsLocked(ctx, statement, now, current, variables, accepted)
			if err != nil {
				return err
			}
			if replaced, replacedOK := statement.takeReplacedEvent(); replacedOK {
				current = replaced
			}
			if statement.takeDroppedEvent() {
				break
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
			if statement.plan.query.statementDrop {
				break
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
	if statement == nil {
		return Event{}, NewError(ErrorDependency, "route requires a statement")
	}
	return e.routeResultToTargetLocked(statement, result, statement.plan.query.routeTarget, now)
}

func (e *Engine) routeResultToTargetLocked(statement *Statement, result Result, targetName string, now time.Time) (Event, error) {
	if e == nil || e.env == nil || statement == nil {
		return Event{}, NewError(ErrorDependency, "route requires an engine, environment and statement")
	}
	targetName = strings.TrimSpace(targetName)
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
		} else {
			values[field.Name] = nil
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
	if target.kind == SchemaAvro {
		return NewAvroRecordFromMap(target, values)
	}
	if target.goType != nil {
		return mergeSchemaUnderlying(target, nil, values)
	}
	result := make(map[string]any, len(target.fields)+len(values))
	for _, field := range target.fields {
		if value, ok := values[field.Name]; ok {
			result[field.Name] = value
		} else {
			result[field.Name] = nil
		}
	}
	for name, value := range values {
		if _, declared := target.fieldIndex[name]; !declared {
			result[name] = value
		}
	}
	return result, nil
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
	if s != nil && s.engine != nil && s.engine.outboundPool != nil {
		outboundContext := context.WithoutCancel(ctx)
		cloned := batch.clone()
		_, err := s.engine.outboundPool.submit(outboundContext, func(taskContext context.Context) error {
			return s.engine.runThreadingTask(taskContext, ThreadingOutbound, func(runContext context.Context) error {
				return s.dispatchSync(runContext, cloned)
			})
		})
		return err
	}
	return s.dispatchSync(ctx, batch)
}

func (s *Statement) dispatchSync(ctx context.Context, batch ResultBatch) error {
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
	subscriber := s.subscriber
	sink := s.plan.query.sink
	s.mu.RUnlock()
	if subscriber != nil {
		if failure, failed := invokeSubscriber(ctx, subscriber, newSubscriberUpdate(s, batch.clone())); failed {
			if s.engine != nil {
				s.engine.reportSubscriberError(ctx, failure)
			}
		}
	}
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
	lineageID  uint64
	lineage    map[int]uint64
}

var joinTupleSchema = func() Schema {
	schema, _ := NewMapSchema("esper:join-tuple", nil)
	return schema
}()

func joinTupleEvents(event Event) []Event {
	if tuple, ok := event.Underlying().(joinTuple); ok {
		return tuple.events
	}
	return nil
}

func newJoinTupleEvent(events []Event, receivedAt time.Time) Event {
	return Event{
		identity:   &eventIdentityToken{marker: 1},
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
	derivedStates            map[*streamNode]*aggregateRuntimeState
	patternState             *patternRuntimeState
	patternJoinStates        map[*streamNode]*patternJoinRuntime
	contextStartPatternState *patternRuntimeState
	contextEndPatternState   *patternRuntimeState
	contextPatternTags       map[string]Event
	contextPatternTagValues  map[string][]Event
	rowRecogState            *rowRecogRuntimeState
	outputState              *outputRuntimeState
	distinctCounts           map[string]int
	namedWindowArrival       []Event
	partitions               map[string]*statementRuntime
	partitionContextName     string
	partitionKey             string
	partitionID              int
	nextPartitionID          int
	initializedAt            time.Time
	initialized              bool
	contextProperties        map[string]Value
	variables                map[string]Value
	methodDependencies       map[string]Event
	joinLineageSeq           uint64
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
	// unidirectionalTrigger is true when at least one transient driver
	// accepted the current event batch. Aggregate joins use it to reset their
	// contribution set before evaluating the trigger's current probe rows.
	unidirectionalTrigger bool
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
	events            []Event
	everEvents        []Event
	leavingEvents     []Event
	pluginStates      map[*exprNode]aggregatePluginState
	multiPluginStates map[string]aggregateMultiPluginState
	representative    Event
	current           Event
	groupingSet       []int
	leaving           bool
	previous          []Value
	emitted           bool
}

type aggregateResultEntry struct {
	result Result
	group  *aggregateGroup
	key    string
}

type patternRuntimeState struct {
	active                 []patternMatch
	patternStopped         bool
	emittedEvents          []Event
	distinct               map[string]struct{}
	distinctAt             map[string]time.Time
	timerStarted           bool
	timerNext              time.Time
	timerEmitted           bool
	timerIntervalReference time.Time
	timerIntervalVariables map[string]Value
	timerIntervalFired     bool
	scheduleIndex          int
	schedulePeriod         *patternTimerScheduleRuntime
	cronSchedule           resolvedCronSchedule
	cronNext               time.Time
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
	node  *patternNode
	phase uint8
	count int
	done  bool
	// quit marks a permanently completed expression, mirroring Esper's
	// EvalNode isQuitted propagation: a plain filter/sequence/and completion
	// cannot produce further matches, and an or-expression one of whose
	// branches completed permanently quits its sibling branches as well.
	// Every-nodes never quit (they restart their child), and the statement
	// root uses the flag to quiesce the whole pattern instead of starting a
	// fresh match for the next event.
	quit                    bool
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
	schedulePeriod          *patternTimerScheduleRuntime
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
	state    *patternProgress
	complete bool
	// matched reports that this transition accepted the current input event;
	// it is separate from consumed because ordinary filters may match without
	// an event-level @consume annotation.
	matched          bool
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
	env              *Environment
	consumptionLevel int
}

type patternMatch struct {
	state     *patternProgress
	tags      map[string]Event
	tagValues map[string][]Event
	current   Event
	startedAt time.Time
	// prearmed marks a branch installed by a timer/guard clock callback before
	// an input event arrived. The first matching event must advance that branch
	// instead of creating a duplicate Every root for the same event.
	prearmed bool
}

type outputRuntimeState struct {
	firstEmitted      int
	firstEverySeen    int
	firstEveryStarted bool
	firstEveryGroups  map[string]struct{}
	firstEveryNext    map[string]time.Time
	lastEverySeen     int
	pending           *ResultBatch
	pendingCount      int
	whenPending       *ResultBatch
	afterSeen         int
	afterActive       bool
	afterStarted      time.Time
	nextOutputAt      time.Time
	cronNext          time.Time
	cronSchedule      resolvedCronSchedule
	cronPending       *ResultBatch
	insertCount       int64
	removeCount       int64
	insertTotal       int64
	removeTotal       int64
	lastOutputAt      time.Time
}

func earlierSchedule(current time.Time, found bool, candidate time.Time) (time.Time, bool) {
	if candidate.IsZero() {
		return current, found
	}
	if !found || candidate.Before(current) {
		return candidate, true
	}
	return current, found
}

func statementRuntimeNearestSchedule(runtime *statementRuntime, query Query) (time.Time, bool) {
	if runtime == nil {
		return time.Time{}, false
	}
	var nearest time.Time
	found := false
	if query.pattern != nil && runtime.patternState != nil && !runtime.patternState.patternStopped {
		root := query.pattern.root
		if root != nil && isPatternTimerRoot(query.pattern) {
			switch root.kind {
			case patternTimerIntervalNode:
				nearest, found = earlierSchedule(nearest, found, runtime.patternState.timerNext)
			case patternTimerAtNode:
				if !runtime.patternState.timerEmitted {
					nearest, found = earlierSchedule(nearest, found, runtime.patternState.timerNext)
				}
			case patternTimerScheduleNode:
				if schedule := runtime.patternState.schedulePeriod; schedule != nil && schedule.active {
					nearest, found = earlierSchedule(nearest, found, schedule.next)
				} else if runtime.patternState.scheduleIndex < len(root.schedule) {
					nearest, found = earlierSchedule(nearest, found, root.schedule[runtime.patternState.scheduleIndex])
				}
			case patternTimerCronNode:
				nearest, found = earlierSchedule(nearest, found, runtime.patternState.cronNext)
			}
		}
		for _, match := range runtime.patternState.active {
			if at, ok := patternProgressNearestSchedule(match.state); ok {
				nearest, found = earlierSchedule(nearest, found, at)
			}
		}
	}
	if state := runtime.outputState; state != nil {
		nearest, found = earlierSchedule(nearest, found, state.nextOutputAt)
		nearest, found = earlierSchedule(nearest, found, state.cronNext)
		for _, at := range state.firstEveryNext {
			nearest, found = earlierSchedule(nearest, found, at)
		}
		if !state.afterActive && query.output.After == OutputAfterDuration && query.output.AfterDuration > 0 {
			nearest, found = earlierSchedule(nearest, found, state.afterStarted.Add(query.output.AfterDuration))
		}
	}
	for _, state := range runtime.windows {
		if at, ok := windowRuntimeNearestSchedule(state); ok {
			nearest, found = earlierSchedule(nearest, found, at)
		}
	}
	for _, partition := range runtime.partitions {
		if at, ok := statementRuntimeNearestSchedule(partition, query); ok {
			nearest, found = earlierSchedule(nearest, found, at)
		}
	}
	return nearest, found
}

func patternProgressNearestSchedule(progress *patternProgress) (time.Time, bool) {
	if progress == nil || progress.expired || progress.quit {
		return time.Time{}, false
	}
	var nearest time.Time
	found := false
	if progress.timerStarted && !progress.timerEmitted {
		nearest, found = earlierSchedule(nearest, found, progress.timerNext)
	}
	if schedule := progress.schedulePeriod; schedule != nil && schedule.active {
		nearest, found = earlierSchedule(nearest, found, schedule.next)
	} else if progress.node != nil && progress.node.kind == patternTimerScheduleNode && progress.scheduleIndex < len(progress.node.schedule) {
		nearest, found = earlierSchedule(nearest, found, progress.node.schedule[progress.scheduleIndex])
	}
	nearest, found = earlierSchedule(nearest, found, progress.cronNext)
	for _, child := range []*patternProgress{progress.left, progress.right, progress.child} {
		if at, ok := patternProgressNearestSchedule(child); ok {
			nearest, found = earlierSchedule(nearest, found, at)
		}
	}
	return nearest, found
}

func windowRuntimeNearestSchedule(state *windowRuntimeState) (time.Time, bool) {
	if state == nil {
		return time.Time{}, false
	}
	var nearest time.Time
	found := false
	for _, entry := range state.entries {
		nearest, found = earlierSchedule(nearest, found, entry.expiresAt)
	}
	for _, entry := range state.pendingNew {
		nearest, found = earlierSchedule(nearest, found, entry.expiresAt)
	}
	for _, child := range state.children {
		if at, ok := windowRuntimeNearestSchedule(child); ok {
			nearest, found = earlierSchedule(nearest, found, at)
		}
	}
	for _, group := range state.groups {
		if at, ok := windowRuntimeNearestSchedule(group); ok {
			nearest, found = earlierSchedule(nearest, found, at)
		}
	}
	return nearest, found
}

func coalesceTime(candidate *time.Time, at time.Time) {
	if candidate != nil && !candidate.IsZero() && candidate.Before(at) {
		*candidate = at
	}
}

func (s *Statement) coalesceSchedulesLocked(at time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.state != StatementStarted {
		return
	}
	coalesceStatementRuntimeSchedules(&s.runtime, at)
}

func coalesceStatementRuntimeSchedules(runtime *statementRuntime, at time.Time) {
	if runtime == nil {
		return
	}
	if state := runtime.patternState; state != nil {
		coalesceTime(&state.timerNext, at)
		coalesceTime(&state.cronNext, at)
		if state.schedulePeriod != nil {
			coalesceTime(&state.schedulePeriod.next, at)
		}
		for index := range state.active {
			coalescePatternProgressSchedules(state.active[index].state, at)
		}
	}
	if state := runtime.outputState; state != nil {
		coalesceTime(&state.nextOutputAt, at)
		coalesceTime(&state.cronNext, at)
		for key, candidate := range state.firstEveryNext {
			coalesceTime(&candidate, at)
			state.firstEveryNext[key] = candidate
		}
	}
	for _, partition := range runtime.partitions {
		coalesceStatementRuntimeSchedules(partition, at)
	}
}

func coalescePatternProgressSchedules(progress *patternProgress, at time.Time) {
	if progress == nil || progress.expired || progress.quit {
		return
	}
	coalesceTime(&progress.timerNext, at)
	coalesceTime(&progress.cronNext, at)
	if progress.schedulePeriod != nil {
		coalesceTime(&progress.schedulePeriod.next, at)
	}
	coalescePatternProgressSchedules(progress.left, at)
	coalescePatternProgressSchedules(progress.right, at)
	coalescePatternProgressSchedules(progress.child, at)
}

func newStatementRuntime(query Query) statementRuntime {
	runtime := statementRuntime{query: query, ctx: context.Background(), windows: make(map[*streamNode]*windowRuntimeState), derivedStates: make(map[*streamNode]*aggregateRuntimeState), partitions: make(map[string]*statementRuntime), patternJoinStates: make(map[*streamNode]*patternJoinRuntime), variables: make(map[string]Value), seq: &atomic.Uint64{}}
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
	r.initializedAt = at
	r.initialized = true
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
			r.patternState.active = []patternMatch{{state: progress, startedAt: at, prearmed: true}}
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
		r.patternState.timerIntervalFired = false
		recordPatternTimerIntervalSchedule(r.patternState, at, r.variables)
	case patternTimerAtNode:
		r.patternState.timerStarted = true
		r.patternState.timerNext = root.at
	case patternTimerScheduleNode:
		r.patternState.timerStarted = true
		if root.schedulePeriod != nil || root.scheduleExpr != nil {
			schedule, ok := patternTimerScheduleRuntimeFor(root, at, nil, nil, r.variables)
			if !ok || schedule == nil {
				r.patternState.patternStopped = true
				return
			}
			r.patternState.schedulePeriod = schedule
			if !schedule.active {
				r.patternState.patternStopped = true
			}
		} else {
			r.patternState.scheduleIndex = 0
		}
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
		if changed && queryIteratorOnlyMethodSources(s.plan.query) {
			// Esper keeps triggerless historical/method-only statements
			// iterator-only: the cycle refreshed the retained state for the
			// iterator, but no listener batch or route is ever posted.
			return ResultBatch{Time: batch.Time}, false, err
		}
		if changed {
			batch.Sequence = s.runtime.seq.Add(1)
		}
		return batch, changed, err
	}
	if s.plan.query.updateStream != nil {
		if updateStreamTargetsNamedWindow(s.plan.query) {
			// A named-window update attaches to the window insert path
			// only (Esper binds the update strategy to the window, not to
			// the underlying event type), so the plain stream dispatch
			// never fires it. The subquery registry above still accepted
			// the event to keep subquery state current.
			return ResultBatch{}, false, nil
		}
		batch, replaced, dropped, err := s.runtime.processUpdateStream(s.plan, event, now, variables)
		if err != nil {
			return ResultBatch{}, false, err
		}
		if replaced != nil {
			s.replacedEvent = replaced
		}
		if dropped {
			s.droppedEvent = true
		}
		return batch, !batch.empty(), nil
	}
	if s.plan.query.trigger != nil {
		batch, err := s.processTriggerRuntime(ctx, &s.runtime, now, event, variables)
		if err != nil {
			return ResultBatch{}, false, err
		}
		return batch, !batch.empty(), nil
	}
	batch, changed, err := s.runtime.process(s.plan, event, now, variables)
	if err != nil {
		return ResultBatch{}, false, err
	}
	if changed && queryIteratorOnlyMethodSources(s.plan.query) {
		return ResultBatch{Time: batch.Time}, false, nil
	}
	return batch, changed, nil
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

// queryIteratorOnlyMethodSources reports whether every from-clause source of
// the query bottoms out in a triggerless method source. Esper keeps such
// statements iterator-only: an arriving event (including the event that sets
// the variables the sources poll against) re-evaluates the sources and
// refreshes the retained state, but no listener batch or route is ever posted
// for the statement. SQL-historical sources intentionally keep the Go-native
// poll-on-any-event delivery behavior.
func queryIteratorOnlyMethodSources(query Query) bool {
	if query.trigger != nil {
		return false
	}
	definitions := make([]*joinDefinition, 0, 2)
	if query.join != nil {
		definitions = append(definitions, query.join)
	}
	if query.aggregate != nil && query.aggregate.join != nil {
		definitions = append(definitions, query.aggregate.join)
	}
	if len(definitions) > 0 {
		for _, definition := range definitions {
			sources := joinDefinitionSources(definition)
			if len(sources) == 0 {
				return false
			}
			for _, source := range sources {
				if !sourceIteratorOnlyMethod(source) {
					return false
				}
			}
		}
		return true
	}
	if query.aggregate != nil || query.input == nil {
		return false
	}
	return sourceIteratorOnlyMethod(query.input)
}

// sourceIteratorOnlyMethod reports whether the source chain bottoms out in a
// method source without an explicit trigger.
func sourceIteratorOnlyMethod(node *streamNode) bool {
	base, err := sourceNode(node)
	if err != nil || base == nil {
		return false
	}
	return base.kind == streamMethod && base.method != nil && base.method.trigger == ""
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
		for _, source := range patternDefinitionInputs(query.pattern) {
			if sourceNodeAcceptsEvent(query.env, source, event) {
				return true
			}
		}
		return false
	}
	if query.aggregate != nil {
		input = query.aggregate.input
	}
	if query.rowRecog != nil {
		input = query.rowRecog.input
	}
	return sourceNodeAcceptsEvent(query.env, input, event)
}

// matchesEventFilter reports whether an active statement's input filter
// service accepts the event. It deliberately ignores post-input where,
// having and output policies: Esper's unmatched-listener contract is based on
// event-stream filter criteria, not on whether a statement ultimately emits
// a result batch.
func (s *Statement) matchesEventFilter(event Event, now time.Time, variables map[string]Value) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed || s.state != StatementStarted {
		return false
	}
	variables = variablesWithEngineLockState(statementVariables(variables, s.parameters), s.engine, true)
	query := s.plan.query
	if query.join != nil {
		for _, source := range joinDefinitionSources(query.join) {
			if sourceNodeMatchesEventFilter(query.env, source, event, now, variables, s.engine) {
				return true
			}
		}
		return false
	}
	if query.pattern != nil {
		for _, source := range patternDefinitionInputs(query.pattern) {
			if sourceNodeMatchesEventFilter(query.env, source, event, now, variables, s.engine) {
				return true
			}
		}
		return false
	}
	input := query.input
	if query.aggregate != nil {
		input = query.aggregate.input
	}
	if query.rowRecog != nil {
		input = query.rowRecog.input
	}
	return sourceNodeMatchesEventFilter(query.env, input, event, now, variables, s.engine)
}

func sourceNodeMatchesEventFilter(env *Environment, node *streamNode, event Event, now time.Time, variables map[string]Value, engine *Engine) bool {
	if node == nil {
		return false
	}
	switch node.kind {
	case streamFilter:
		if !sourceNodeMatchesEventFilter(env, node.input, event, now, variables, engine) || node.predicate == nil {
			return false
		}
		value := node.predicate.eval(EvalContext{
			Event:      event,
			OuterEvent: event,
			Engine:     engine,
			Now:        now,
			Variables:  variables,
		})
		matched, ok := boolValue(value)
		return ok && matched
	case streamWindow, streamDerived:
		return sourceNodeMatchesEventFilter(env, node.input, event, now, variables, engine)
	case streamPattern:
		for _, input := range patternDefinitionInputs(node.pattern) {
			if sourceNodeMatchesEventFilter(env, input, event, now, variables, engine) {
				return true
			}
		}
		return false
	case streamContained:
		// Contained filters execute after parent expansion and therefore do not
		// have a single child Event at the engine filter-service boundary. The
		// parent event type match is the stable unmatched-listener criterion.
		return sourceNodeAcceptsEvent(env, node, event)
	default:
		return sourceNodeAcceptsEvent(env, node, event)
	}
}

func sourceNodeAcceptsEvent(env *Environment, node *streamNode, event Event) bool {
	source, err := sourceNode(node)
	if err != nil || source == nil {
		return false
	}
	if source.kind == streamDerived {
		if source.input == nil {
			return false
		}
		return sourceNodeAcceptsEvent(env, source.input, event)
	}
	if source.kind == streamPattern {
		for _, input := range patternDefinitionInputs(source.pattern) {
			if sourceNodeAcceptsEvent(env, input, event) {
				return true
			}
		}
		return false
	}
	if source.kind == streamContained {
		// A contained source is fed by its parent event at the statement input
		// boundary, but Pattern/Join branches consume the expanded child events
		// after the source has been evaluated. Accept both representations so a
		// nested contained pattern can arm on the child while the statement still
		// subscribes to the parent event type.
		if env != nil {
			if childSchema, schemaErr := env.sourceSchema(source); schemaErr == nil {
				if event.StreamType() == childSchema.Name() || event.TypeName() == childSchema.Name() || env.acceptsEventType(childSchema.Name(), event.TypeName()) {
					return true
				}
			}
		}
		return sourceNodeAcceptsEvent(env, source.input, event)
	}
	if source.kind == streamHistorical || source.kind == streamMethod {
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

	startMatches := advanceContextPattern(&s.runtime.contextStartPatternState, definition.startPattern, event, now, variables, nil, nil, s.engine.env, &s.runtime, "start")
	for _, match := range startMatches {
		if !definition.initiatedOverlapping && len(s.runtime.partitions) > 0 {
			continue
		}
		allocationKey := "initiated:pattern"
		if definition.initiatedOverlapping {
			// Key the overlapping partition by the initiating event
			// identity plus a per-statement ordinal rather than a global
			// sequence: every statement that observes the same start
			// event resolves the same context partition (context
			// variables are shared across statements), while repeated
			// identical start events within one statement still allocate
			// distinct overlapping partitions.
			baseKey := "initiated:pattern" + overlappingContextPartitionSeparator + encodeKey([]any{event.Underlying()})
			allocationKey = baseKey
			for ordinal := 1; ; ordinal++ {
				if _, exists := s.runtime.partitions[allocationKey]; !exists {
					break
				}
				allocationKey = fmt.Sprintf("%s#%d", baseKey, ordinal)
			}
		} else if _, exists := s.runtime.partitions[allocationKey]; exists {
			continue
		}
		query := s.runtime.query
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
				partition,
				"end",
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

func advanceContextPattern(state **patternRuntimeState, definition *patternDefinition, event Event, now time.Time, variables map[string]Value, seedTags map[string]Event, seedTagValues map[string][]Event, env *Environment, runtime *statementRuntime, phase string) []patternMatch {
	if state == nil || definition == nil || definition.root == nil {
		return nil
	}
	if !patternDefinitionAcceptsEvent(env, definition, event) {
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
	prearmedActive := false
	for _, match := range matches {
		if match.prearmed {
			prearmedActive = true
			break
		}
	}
	startAllowed := !prearmedActive && (definition.every || len(matches) == 0)
	var startProgress *patternProgress
	if startAllowed {
		startProgress = newPatternProgress(definition.root)
		startProgress.tags = clonePatternTags(seedTags)
		startProgress.tagValues = clonePatternTagValues(seedTagValues)
		armPatternProgressTimers(startProgress, now, variables)
	}
	consumptionLevel := -1
	if patternHasConsumption(definition.root) {
		probe := patternTrigger{event: event, now: now, env: env, consumptionLevel: -1}
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
	trigger := patternTrigger{event: event, now: now, env: env, consumptionLevel: consumptionLevel}
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
				prearmed:  match.prearmed && !transition.matched,
			}
			if transition.complete {
				completedAny = true
				completed = append(completed, candidate)
				if patternCanContinueAfterMatch(transition.state) && admitContextPatternMatch(runtime, phase, nextActive, candidate, definition) {
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
			if patternProgressActive(transition.state) && admitContextPatternMatch(runtime, phase, nextActive, candidate, definition) {
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
				if patternCanContinueAfterMatch(transition.state) && admitContextPatternMatch(runtime, phase, nextActive, started, definition) {
					nextActive = append(nextActive, started)
				}
				if patternProgressTerminal(transition.state) {
					terminal = true
				}
			} else if admitContextPatternMatch(runtime, phase, nextActive, started, definition) {
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
				runtimeState.active = []patternMatch{{state: progress, startedAt: at, prearmed: true}}
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
		runtimeState.timerIntervalFired = false
		recordPatternTimerIntervalSchedule(runtimeState, at, variables)
	case patternTimerAtNode:
		runtimeState.timerNext = root.at
	case patternTimerScheduleNode:
		if root.schedulePeriod != nil || root.scheduleExpr != nil {
			schedule, ok := patternTimerScheduleRuntimeFor(root, at, nil, nil, variables)
			if !ok || schedule == nil {
				runtimeState.patternStopped = true
				return false
			}
			runtimeState.schedulePeriod = schedule
			if !schedule.active {
				runtimeState.patternStopped = true
			}
		} else {
			runtimeState.scheduleIndex = 0
		}
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

func advanceContextPatternCompositeTime(state **patternRuntimeState, definition *patternDefinition, now time.Time, variables map[string]Value, seedTags map[string]Event, seedTagValues map[string][]Event, runtime *statementRuntime, phase string) []patternMatch {
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
				prearmed:  match.prearmed,
			}
			if transition.complete {
				completed = append(completed, candidate)
				if patternCanContinueAfterMatch(transition.state) && admitContextPatternMatch(runtime, phase, nextActive, candidate, definition) {
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
					candidate := patternMatch{state: progress, startedAt: now, prearmed: patternCanStartWithoutEvent(definition.root)}
					if patternProgressActive(progress) && admitContextPatternMatch(runtime, phase, nextActive, candidate, definition) {
						nextActive = append(nextActive, candidate)
					}
				}
				continue
			}
			if patternProgressTerminal(transition.state) {
				terminal = true
			}
			if patternProgressActive(transition.state) && admitContextPatternMatch(runtime, phase, nextActive, candidate, definition) {
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

func advanceContextPatternTime(state **patternRuntimeState, definition *patternDefinition, now time.Time, variables map[string]Value, seedTags map[string]Event, seedTagValues map[string][]Event, runtime *statementRuntime, phase string) []patternMatch {
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
		return advanceContextPatternCompositeTime(state, definition, now, variables, seedTags, seedTagValues, runtime, phase)
	}
	runtimeState := *state
	root := definition.root
	completed := make([]patternMatch, 0)
	const maxTimerCatchUp = 100000
	switch root.kind {
	case patternTimerIntervalNode:
		if !rearmPatternTimerIntervalIfVariablesChanged(runtimeState, root, variables) {
			return completed
		}
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
			runtimeState.timerIntervalFired = true
			recordPatternTimerIntervalSchedule(runtimeState, dueAt, variables)
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
		if runtimeState.schedulePeriod != nil {
			for emitted := 0; emitted < maxTimerCatchUp && runtimeState.schedulePeriod.active && !runtimeState.schedulePeriod.next.After(now); emitted++ {
				dueAt := runtimeState.schedulePeriod.next
				completed = append(completed, patternMatch{
					current:   Event{},
					startedAt: dueAt,
					tags:      clonePatternTags(seedTags),
					tagValues: clonePatternTagValues(seedTagValues),
				})
				advancePatternTimerScheduleRuntime(runtimeState.schedulePeriod)
			}
		} else {
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
	for _, match := range advanceContextPatternTime(&s.runtime.contextStartPatternState, definition.startPattern, now, variables, nil, nil, &s.runtime, "start") {
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
		query := s.runtime.query
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
		matches := advanceContextPatternTime(&partition.contextEndPatternState, definition.endPattern, now, partitionVariables, partition.contextPatternTags, partition.contextPatternTagValues, partition, "end")
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
		query := s.runtime.query
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
			query := s.runtime.query
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
		query := s.runtime.query
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
				if joinDelta.unidirectionalTrigger {
					// A unidirectional aggregate represents the current
					// trigger probe, not a retained contribution from prior
					// trigger events. Passive events do not reach aggregateBatch.
					r.aggregateState = nil
				}
				batch, err = r.aggregateBatchSafely(joinDeltaEvents(joinDelta, now), plan, now)
			}
		} else {
			delta, insertErr := r.insert(plan.query.aggregate.input, event, now)
			err = insertErr
			if err == nil {
				batch, err = r.aggregateBatchSafely(delta, plan, now)
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
		delta, insertErr := r.insertPatternInputs(plan.query.pattern, event, now)
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

// processUpdateStream applies an update-istream statement to one accepted
// event. The where clause and every assignment expression evaluate against
// the pre-update event; the assignments are then written into a copy of the
// underlying so the original event value is never mutated. The batch mirrors
// Esper's update delivery (new = updated copy, old = pre-update event), and
// the returned replacement event lets the engine continue the current
// dispatch cycle with the updated copy for statements deployed later.
func (r *statementRuntime) processUpdateStream(plan Plan, event Event, now time.Time, variables map[string]Value) (ResultBatch, *Event, bool, error) {
	r.variables = variablesWithEngine(variables, r.engine)
	r.variables = r.withContextVariables(r.variables)
	r.variables = r.withContextProperties(r.variables)
	delta, err := r.insert(plan.query.input, event, now)
	if err != nil {
		return ResultBatch{}, nil, false, err
	}
	if len(delta.newEvents) == 0 {
		return ResultBatch{}, nil, false, nil
	}
	definition := plan.query.updateStream
	accepted := delta.newEvents[0]
	// Engine lets subquery set/where expressions take their consistent
	// snapshot; OuterEvent binds correlated subqueries to the pre-update
	// event, matching Esper's update-istream correlation scope.
	evalContext := EvalContext{Engine: r.engine, Event: accepted, OuterEvent: accepted, Now: now, Variables: r.variables}
	if definition.where != nil {
		value := definition.where.eval(evalContext)
		if matched, ok := boolValue(value); !ok || !matched {
			return ResultBatch{}, nil, false, nil
		}
	}
	if definition.drop {
		// @Drop removes the matching event from the stream dispatch
		// entirely and delivers no listener batch of its own.
		return ResultBatch{}, nil, true, nil
	}
	updates := make(map[string]any, len(definition.assignments))
	for _, assignment := range definition.assignments {
		if assignment.Index != nil {
			indexValue := assignment.Index.eval(evalContext)
			if indexValue.IsMissing() || indexValue.IsNull() {
				// Esper skips an indexed write whose index expression
				// returns null without failing the statement.
				continue
			}
			value := assignment.Expr.eval(evalContext)
			if err := applyIndexedUpdateSetValue(accepted.Schema(), accepted.Underlying(), updates, assignment.Column, indexValue, value); err != nil {
				return ResultBatch{}, nil, false, err
			}
			continue
		}
		if assignment.Key != nil {
			keyValue := assignment.Key.eval(evalContext)
			if keyValue.IsMissing() || keyValue.IsNull() {
				// Esper skips a map-entry write whose key expression
				// returns null without failing the statement.
				continue
			}
			value := assignment.Expr.eval(evalContext)
			if err := applyKeyedUpdateSetValue(accepted.Schema(), accepted.Underlying(), updates, assignment.Column, keyValue, value); err != nil {
				return ResultBatch{}, nil, false, err
			}
			continue
		}
		value := assignment.Expr.eval(evalContext).Any()
		if field, _, lookupErr := accepted.Schema().lookupField(assignment.Column); lookupErr == nil && field.Type != nil {
			// Coercion normalizes typed-nil pointers to untyped nil before
			// the null check below, and widens numerics towards the target.
			coerced, coerceErr := coerceUpdateSetValue(field.Type, value)
			if coerceErr != nil {
				return ResultBatch{}, nil, false, WrapError(ErrorTypeMismatch, fmt.Sprintf("update-set %q", assignment.Column), coerceErr)
			}
			value = coerced
			if value == nil && !nullableUpdateFieldKind(field.Type) {
				// Esper skips null writes to non-nullable properties: the
				// assignment is dropped and the property keeps its current
				// value, while the update still fires the insert/remove pair.
				continue
			}
		}
		updates[assignment.Column] = value
	}
	updated, err := mergeSchemaUnderlying(accepted.Schema(), accepted.Underlying(), updates)
	if err != nil {
		return ResultBatch{}, nil, false, WrapError(ErrorTypeMismatch, "update-set", err)
	}
	replaced := accepted.withUnderlying(updated)
	batch := ResultBatch{
		Time:            now,
		New:             []Result{resultEvent(replaced)},
		Old:             []Result{resultEvent(accepted)},
		outputCountsSet: true,
		outputInserted:  1,
		outputRemoved:   1,
	}
	batch.Sequence = r.seq.Add(1)
	return batch, &replaced, false, nil
}

// processNamedWindowUpdate runs one update-istream statement against an event
// offered to its target named window. It mirrors the update branch of
// Statement.process: the where clause and assignments evaluate against the
// offered (pre-update) event and the returned replacement carries the
// copy-on-write update for the window insert path.
func (s *Statement) processNamedWindowUpdate(ctx context.Context, event Event, now time.Time, variables map[string]Value) (ResultBatch, *Event, bool, error) {
	if err := contextErr(ctx); err != nil {
		return ResultBatch{}, nil, false, err
	}
	if s == nil {
		return ResultBatch{}, nil, false, NewError(ErrorDependency, "nil statement")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.state != StatementStarted {
		return ResultBatch{}, nil, false, nil
	}
	variables = variablesWithEngineLockState(statementVariables(variables, s.parameters), s.engine, true)
	s.runtime.ctx = ctx
	return s.runtime.processUpdateStream(s.plan, event, now, variables)
}

// windowUpdateStatementsLocked collects the update-istream statements whose
// target is the given named window, ordered like the stream dispatch path
// (ascending priority, drop entries first on ties, deployment order as the
// stable fallback). The engine mutex must be held.
func (e *Engine) windowUpdateStatementsLocked(definition NamedWindowDefinition) []*Statement {
	if e == nil {
		return nil
	}
	key := catalogKey(definition.moduleName, definition.name)
	updates := make([]*Statement, 0, 1)
	for _, statement := range e.sortedStatementsLocked() {
		if statement == nil || statement.plan.query.updateStream == nil {
			continue
		}
		source, err := sourceNode(statement.plan.query.input)
		if err != nil || source == nil || source.kind != streamNamedWindow {
			continue
		}
		if catalogKey(source.moduleName, source.sourceName) != key {
			continue
		}
		updates = append(updates, statement)
	}
	return orderUpdateStatementsFirst(updates)
}

// applyNamedWindowUpdatesLocked preprocesses one event offered to a named
// window through every update-istream statement targeting that window, in
// priority order. Each matching statement replaces the in-flight event with
// its updated copy for the next update and for the window insert; a matching
// drop removes the event from the insert entirely. Update listener batches
// queue onto the engine's pending statement dispatches so they publish in
// the same dispatch cycle as the window delta, mirroring Esper's update
// strategy delivery. The engine mutex must be held.
func (e *Engine) applyNamedWindowUpdatesLocked(ctx context.Context, window *NamedWindow, event Event, now time.Time, variables map[string]Value) (Event, bool, error) {
	if e == nil || window == nil || window.state == nil {
		return event, false, nil
	}
	statements := e.windowUpdateStatementsLocked(window.state.def)
	if len(statements) == 0 {
		return event, false, nil
	}
	current := event
	for _, statement := range statements {
		batch, replaced, dropped, err := statement.processNamedWindowUpdate(ctx, current, now, variables)
		if err != nil {
			return Event{}, false, err
		}
		if !batch.empty() {
			e.pendingStatementDispatches = append(e.pendingStatementDispatches, statementDispatch{statement: statement, batch: batch})
		}
		if dropped {
			return Event{}, true, nil
		}
		if replaced != nil {
			current = *replaced
		}
	}
	return current, false, nil
}

// nullableUpdateFieldKind reports whether a schema field type can hold a
// null value. Esper's update-istream skips null writes to properties whose
// type cannot represent null (primitive ints and similar value kinds).
func nullableUpdateFieldKind(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return true
	}
	return false
}

// coerceUpdateSetValue converts an evaluated update-set value towards the
// target property type, mirroring Esper's update-istream type widener:
// boxed values are unboxed, numeric values widen or narrow across Go numeric
// kinds and the result is re-boxed when the property is a pointer. Values
// that need no coercion pass through unchanged so mergeSchemaUnderlying's
// own assignment validation still applies.
func coerceUpdateSetValue(target reflect.Type, value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	current := reflect.ValueOf(value)
	for current.Kind() == reflect.Pointer {
		if current.IsNil() {
			return nil, nil
		}
		current = current.Elem()
	}
	base := target
	for base.Kind() == reflect.Pointer {
		base = base.Elem()
	}
	if current.Type().AssignableTo(base) {
		if target.Kind() == reflect.Pointer {
			boxed := reflect.New(base)
			boxed.Elem().Set(current)
			return boxed.Interface(), nil
		}
		return current.Interface(), nil
	}
	if numericTypes(current.Type(), base) && current.Type().ConvertibleTo(base) {
		converted := current.Convert(base)
		if target.Kind() == reflect.Pointer {
			boxed := reflect.New(base)
			boxed.Elem().Set(converted)
			return boxed.Interface(), nil
		}
		return converted.Interface(), nil
	}
	return value, nil
}

// applyIndexedUpdateSetValue applies one update-istream array-element write
// (column[index] = value) to the accumulated updates. The target container is
// cloned before the write so the original event value is never mutated,
// matching Esper's copy-on-write preprocessing; repeated writes to one column
// build on the previous clone. A null container skips the write silently. An
// out-of-range index fails the statement with Esper's update-istream
// diagnostic. A null value writes the zero value into nilable element types
// and is skipped otherwise (Esper skips a null rhs for an array of
// primitives).
func applyIndexedUpdateSetValue(schema Schema, original any, updates map[string]any, column string, indexValue, value Value) error {
	workingUnderlying, err := mergeSchemaUnderlying(schema, original, updates)
	if err != nil {
		return err
	}
	current := schema.get(workingUnderlying, column)
	if !current.IsPresent() || current.IsNull() {
		return nil
	}
	index, ok := triggerIndexValue(indexValue.Any())
	if !ok {
		return fmt.Errorf("array index for %q is not an integer", column)
	}
	array := reflect.ValueOf(current.Any())
	if !array.IsValid() {
		return nil
	}
	for array.Kind() == reflect.Interface {
		if array.IsNil() {
			return nil
		}
		array = array.Elem()
	}
	pointer := array.Kind() == reflect.Pointer
	if pointer {
		if array.IsNil() {
			return nil
		}
		array = array.Elem()
	}
	if array.Kind() != reflect.Array && array.Kind() != reflect.Slice {
		return fmt.Errorf("target column %q is not an array or slice", column)
	}
	if index < 0 || index >= array.Len() {
		return fmt.Errorf("Array length %d less than index %d for property '%s'", array.Len(), index, column)
	}
	elementType := array.Type().Elem()
	if value.IsMissing() {
		return nil
	}
	if value.IsNull() || value.Any() == nil {
		if !isTriggerNilableType(elementType) {
			return nil
		}
	}
	var copyValue reflect.Value
	if array.Kind() == reflect.Slice {
		copyValue = reflect.MakeSlice(array.Type(), array.Len(), array.Len())
		reflect.Copy(copyValue, array)
	} else {
		copyValue = reflect.New(array.Type()).Elem()
		copyValue.Set(array)
	}
	element := copyValue.Index(index)
	if value.IsNull() || value.Any() == nil {
		element.Set(reflect.Zero(element.Type()))
	} else {
		coerced, coerceErr := coerceUpdateSetValue(element.Type(), value.Any())
		if coerceErr != nil {
			return fmt.Errorf("array column %q element %d: %w", column, index, coerceErr)
		}
		converted, convertErr := assignReflectValue(element.Type(), coerced)
		if convertErr != nil {
			return fmt.Errorf("array column %q element %d: %w", column, index, convertErr)
		}
		element.Set(converted)
	}
	if pointer {
		result := reflect.New(copyValue.Type())
		result.Elem().Set(copyValue)
		updates[column] = result.Interface()
	} else {
		updates[column] = copyValue.Interface()
	}
	return nil
}

// applyKeyedUpdateSetValue applies one update-istream map-entry write
// (column('key') = value) to the accumulated updates. The target map is
// cloned before the write so the original event value is never mutated,
// matching Esper's copy-on-write preprocessing; repeated writes to one column
// build on the previous clone. A null map or null key skips the write
// silently. A null value writes the zero value into nilable element types and
// is skipped otherwise.
func applyKeyedUpdateSetValue(schema Schema, original any, updates map[string]any, column string, keyValue, value Value) error {
	workingUnderlying, err := mergeSchemaUnderlying(schema, original, updates)
	if err != nil {
		return err
	}
	current := schema.get(workingUnderlying, column)
	if !current.IsPresent() || current.IsNull() {
		return nil
	}
	key, ok := keyValue.Any().(string)
	if !ok {
		return fmt.Errorf("map key for %q is not a string", column)
	}
	container := reflect.ValueOf(current.Any())
	if !container.IsValid() {
		return nil
	}
	for container.Kind() == reflect.Interface {
		if container.IsNil() {
			return nil
		}
		container = container.Elem()
	}
	pointer := container.Kind() == reflect.Pointer
	if pointer {
		if container.IsNil() {
			return nil
		}
		container = container.Elem()
	}
	if container.Kind() != reflect.Map {
		return fmt.Errorf("target column %q is not a map", column)
	}
	elementType := container.Type().Elem()
	if value.IsMissing() {
		return nil
	}
	if value.IsNull() || value.Any() == nil {
		if !isTriggerNilableType(elementType) {
			return nil
		}
	}
	clone := reflect.MakeMapWithSize(container.Type(), container.Len()+1)
	iterator := container.MapRange()
	for iterator.Next() {
		clone.SetMapIndex(iterator.Key(), iterator.Value())
	}
	keyReflected := reflect.ValueOf(key)
	if keyReflected.Type() != container.Type().Key() {
		keyReflected = keyReflected.Convert(container.Type().Key())
	}
	if value.IsNull() || value.Any() == nil {
		clone.SetMapIndex(keyReflected, reflect.Zero(elementType))
	} else {
		coerced, coerceErr := coerceUpdateSetValue(elementType, value.Any())
		if coerceErr != nil {
			return fmt.Errorf("map column %q entry %q: %w", column, key, coerceErr)
		}
		converted, convertErr := assignReflectValue(elementType, coerced)
		if convertErr != nil {
			return fmt.Errorf("map column %q entry %q: %w", column, key, convertErr)
		}
		clone.SetMapIndex(keyReflected, converted)
	}
	if pointer {
		result := reflect.New(clone.Type())
		result.Elem().Set(clone)
		updates[column] = result.Interface()
	} else {
		updates[column] = clone.Interface()
	}
	return nil
}

func (r *statementRuntime) evaluationContext() ExpressionEvaluationContext {
	metadata := ExpressionEvaluationContext{ContextPartitionID: -1}
	if r == nil {
		return metadata
	}
	metadata.StatementName = r.query.name
	metadata.StatementUserObject = r.query.statementUserObject
	if r.engine != nil {
		metadata.RuntimeURI = r.engine.RuntimeURI()
	}
	if strings.TrimSpace(r.partitionContextName) != "" {
		metadata.ContextPartitionID = r.partitionID
	}
	return metadata
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
			delta, expireErr := r.expireJoin(now)
			if expireErr != nil {
				return ResultBatch{}, expireErr
			}
			if delta.unidirectionalTrigger {
				r.aggregateState = nil
			}
			batch, _ = r.aggregateBatch(joinDeltaEvents(delta, now), plan, now)
		} else {
			delta := r.expire(now)
			batch, _ = r.aggregateBatch(delta, plan, now)
		}
	} else if plan.query.join != nil {
		delta, expireErr := r.expireJoin(now)
		if expireErr != nil {
			return ResultBatch{}, expireErr
		}
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
	case OutputFirstEveryEventsPolicy:
		return r.applyFirstEveryEvents(policy, batch, now, plans...)
	case OutputFirstEveryTimePolicy:
		return r.applyFirstEveryTime(policy, batch, now, plans...)
	case OutputLastEveryEventsPolicy:
		return r.applyLastEveryEvents(policy, batch, now, plans...)
	case OutputLastEveryTimePolicy:
		return r.applyLastEveryTime(policy, batch, flush, now, plans...)
	case OutputLastPolicy, OutputSnapshotPolicy:
		if !batch.empty() {
			copyBatch := mergeLastOutputBatch(r.outputState.pending, batch)
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
		if len(plans) > 0 && plans[0].query.distinct {
			result.New = distinctSnapshotResults(result.New)
			result.Old = distinctSnapshotResults(result.Old)
		}
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
		if len(plans) > 0 && plans[0].query.distinct {
			result.New = distinctSnapshotResults(result.New)
			result.Old = distinctSnapshotResults(result.Old)
		}
		result.Time = now
		r.outputState.pending = nil
		r.outputState.pendingCount = 0
		r.advanceOutputSchedule(policy, now)
		return r.finishOutput(policy, result, now, plans...)
	default:
		return r.finishOutput(policy, batch, now, plans...)
	}
}

func (r *statementRuntime) applyFirstEveryEvents(policy OutputPolicy, batch ResultBatch, now time.Time, plans ...Plan) ResultBatch {
	if r == nil || r.outputState == nil {
		return ResultBatch{}
	}
	state := r.outputState
	if state.firstEveryGroups == nil {
		state.firstEveryGroups = make(map[string]struct{})
	}
	if !state.firstEveryStarted {
		if batch.empty() {
			return ResultBatch{}
		}
		state.firstEveryStarted = true
		state.firstEverySeen = 0
		result := selectFirstOutputByGroup(batch, func(key string) bool {
			if _, exists := state.firstEveryGroups[key]; exists {
				return false
			}
			state.firstEveryGroups[key] = struct{}{}
			return true
		})
		return r.finishOutput(policy, result, now, plans...)
	}
	state.firstEverySeen += acceptedOutputEventCount(batch)
	if state.firstEverySeen >= policy.Count {
		state.firstEverySeen = 0
		clearOutputGroupSet(state.firstEveryGroups)
	}
	if batch.empty() {
		return ResultBatch{}
	}
	result := selectFirstOutputByGroup(batch, func(key string) bool {
		if _, exists := state.firstEveryGroups[key]; exists {
			return false
		}
		state.firstEveryGroups[key] = struct{}{}
		return true
	})
	return r.finishOutput(policy, result, now, plans...)
}

func (r *statementRuntime) applyFirstEveryTime(policy OutputPolicy, batch ResultBatch, now time.Time, plans ...Plan) ResultBatch {
	if r == nil || r.outputState == nil || batch.empty() {
		return ResultBatch{}
	}
	state := r.outputState
	if state.firstEveryNext == nil {
		state.firstEveryNext = make(map[string]time.Time)
	}
	result := selectFirstOutputByGroup(batch, func(key string) bool {
		next, exists := state.firstEveryNext[key]
		if exists && now.Before(next) {
			return false
		}
		state.firstEveryNext[key] = now.Add(policy.Interval)
		return true
	})
	return r.finishOutput(policy, result, now, plans...)
}

func (r *statementRuntime) applyLastEveryEvents(policy OutputPolicy, batch ResultBatch, now time.Time, plans ...Plan) ResultBatch {
	if r == nil || r.outputState == nil {
		return ResultBatch{}
	}
	state := r.outputState
	state.lastEverySeen += acceptedOutputEventCount(batch)
	if !batch.empty() {
		copyBatch := mergeLastOutputBatch(state.pending, batch)
		state.pending = &copyBatch
	}
	if state.lastEverySeen < policy.Count {
		return ResultBatch{}
	}
	state.lastEverySeen = 0
	if state.pending == nil {
		return ResultBatch{}
	}
	result := state.pending.clone()
	state.pending = nil
	return r.finishOutput(policy, result, now, plans...)
}

func (r *statementRuntime) applyLastEveryTime(policy OutputPolicy, batch ResultBatch, flush bool, now time.Time, plans ...Plan) ResultBatch {
	if r == nil || r.outputState == nil {
		return ResultBatch{}
	}
	state := r.outputState
	if !batch.empty() {
		copyBatch := mergeLastOutputBatch(state.pending, batch)
		state.pending = &copyBatch
		if state.nextOutputAt.IsZero() {
			state.nextOutputAt = now.Add(policy.Interval)
		}
	}
	if !flush || state.pending == nil || state.nextOutputAt.IsZero() || now.Before(state.nextOutputAt) {
		return ResultBatch{}
	}
	result := state.pending.clone()
	result.Time = now
	state.pending = nil
	r.advanceOutputSchedule(policy, now)
	return r.finishOutput(policy, result, now, plans...)
}

func acceptedOutputEventCount(batch ResultBatch) int {
	if batch.outputCountsSet {
		if batch.outputInserted <= 0 {
			return 0
		}
		return int(batch.outputInserted)
	}
	return len(batch.New)
}

func firstOutputResult(batch ResultBatch) ResultBatch {
	result := batch.clone()
	if len(result.New) > 0 {
		result.New = result.New[:1]
		if len(result.outputKeysNew) > 1 {
			result.outputKeysNew = result.outputKeysNew[:1]
		}
		result.Old = nil
		result.outputKeysOld = nil
		return result
	}
	if len(result.Old) > 1 {
		result.Old = result.Old[:1]
		if len(result.outputKeysOld) > 1 {
			result.outputKeysOld = result.outputKeysOld[:1]
		}
	}
	return result
}

func selectFirstOutputByGroup(batch ResultBatch, allow func(string) bool) ResultBatch {
	result := ResultBatch{Time: batch.Time}
	selected := make(map[string]struct{})
	appendNew := func(index int) {
		key := outputGroupKey(batch.outputKeysNew, index)
		if _, exists := selected[key]; exists || !allow(key) {
			return
		}
		selected[key] = struct{}{}
		result.New = append(result.New, batch.New[index])
		if len(batch.outputKeysNew) > index {
			result.outputKeysNew = append(result.outputKeysNew, key)
		}
	}
	appendOld := func(index int) {
		key := outputGroupKey(batch.outputKeysOld, index)
		if _, exists := selected[key]; exists || !allow(key) {
			return
		}
		selected[key] = struct{}{}
		result.Old = append(result.Old, batch.Old[index])
		if len(batch.outputKeysOld) > index {
			result.outputKeysOld = append(result.outputKeysOld, key)
		}
	}
	for index := range batch.New {
		appendNew(index)
	}
	for index := range batch.Old {
		appendOld(index)
	}
	return result
}

func clearOutputGroupSet(groups map[string]struct{}) {
	for key := range groups {
		delete(groups, key)
	}
}

func outputGroupKey(keys []string, index int) string {
	if index >= 0 && index < len(keys) && keys[index] != "" {
		return keys[index]
	}
	return "\x00esper-output-global"
}

func mergeLastOutputBatch(existing *ResultBatch, incoming ResultBatch) ResultBatch {
	if incoming.empty() {
		if existing == nil {
			return ResultBatch{}
		}
		return existing.clone()
	}
	if existing == nil || (len(existing.outputKeysNew) == 0 && len(existing.outputKeysOld) == 0 && len(incoming.outputKeysNew) == 0 && len(incoming.outputKeysOld) == 0) {
		return incoming.clone()
	}
	result := existing.clone()
	result.New, result.outputKeysNew = mergeLastOutputSide(result.New, incoming.New, result.outputKeysNew, incoming.outputKeysNew)
	result.Old, result.outputKeysOld = mergeLastOutputSide(result.Old, incoming.Old, result.outputKeysOld, incoming.outputKeysOld)
	result.Time = incoming.Time
	return result
}

func mergeLastOutputSide(existing, incoming []Result, existingKeys, incomingKeys []string) ([]Result, []string) {
	if len(incoming) == 0 {
		return existing, existingKeys
	}
	if len(existing) == 0 && len(existingKeys) == 0 {
		return append([]Result(nil), incoming...), append([]string(nil), incomingKeys...)
	}
	result := append([]Result(nil), existing...)
	keys := append([]string(nil), existingKeys...)
	positions := make(map[string]int, len(result))
	for index := range result {
		positions[outputGroupKey(keys, index)] = index
	}
	for index, item := range incoming {
		key := outputGroupKey(incomingKeys, index)
		if position, exists := positions[key]; exists {
			result[position] = item
			continue
		}
		positions[key] = len(result)
		result = append(result, item)
		if len(incomingKeys) > index || len(keys) > 0 {
			keys = append(keys, key)
		}
	}
	return result, keys
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
	result.New = projectResults(events, plan.query, plan.resultSchema, now, r.variables, history, nil, previousByEvent, priorByEvent, false, r.evaluationContext())
	if plan.query.distinct {
		result.New = distinctSnapshotResults(result.New)
	}
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
		if aggregateDefinitionIsRowForEvent(definition) && len(group.events) > 0 {
			for _, current := range group.events {
				values, visible := evaluateAggregateGroup(
					definition,
					group.events,
					group.everEvents,
					nil,
					false,
					group.groupingSet,
					current,
					r.aggregateState.allEvents,
					r.aggregateState.allEverEvents,
					now,
					r.variables,
					group.pluginStates,
					group.multiPluginStates,
				)
				if visible {
					entries = append(entries, aggregateResultEntry{result: resultRow(newRow(plan.resultSchema, values)), group: group, key: key})
				}
			}
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
			group.multiPluginStates,
		)
		if !visible {
			continue
		}
		entries = append(entries, aggregateResultEntry{result: resultRow(newRow(plan.resultSchema, values)), group: group, key: key})
	}
	if len(plan.query.orderBy) > 0 {
		orderAggregateResults(entries, plan.query.orderBy, definition, r.aggregateState.allEvents, r.aggregateState.allEverEvents, now, r.variables, false)
	}
	for _, entry := range entries {
		result.New = append(result.New, entry.result)
		result.outputKeysNew = append(result.outputKeysNew, entry.key)
	}
	if plan.query.distinct {
		result.New = distinctSnapshotResults(result.New)
	}
	result.New = applyResultWindow(result.New, plan.query)
	return result
}

func (r *statementRuntime) snapshotJoinBatch(plan Plan, now time.Time) ResultBatch {
	result := ResultBatch{Time: now}
	if r == nil || plan.query.join == nil {
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
		state.sides[index] = r.assignJoinLineageIDs(side, state.sides[index])
	}
	tuples := joinTuples(plan.query.join, state, now, r)
	tuples = filterJoinTuples(tuples, plan.query, now, r.variables)
	result.New = orderJoinResults(
		projectJoinTuples(tuples, plan.query, plan.resultSchema, now, r.variables, false, r.evaluationContext()),
		tuples, plan.query.orderBy, now, r.variables, false,
	)
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
		copyState.sides[index] = make([]storedEvent, len(side))
		for eventIndex, stored := range side {
			copyState.sides[index][eventIndex] = stored
			copyState.sides[index][eventIndex].lineage = cloneMethodLineage(stored.lineage)
		}
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
	case streamMethod:
		// A triggerless method source is re-polled for the iterator with the
		// current variables, mirroring Esper's lazy historical evaluation of
		// variable-driven method streams.
		if r.engine == nil {
			return nil
		}
		events, err := r.engine.snapshotFireAndForgetSourceLocked(r.context(), node, now, r.variables)
		if err != nil {
			return nil
		}
		return events
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
			copyBatch := mergeLastOutputBatch(state.pending, batch)
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
	if len(plans) > 0 && deferOutputResultWindow(plans[0].query.output) {
		batch.New = orderResults(batch.New, plans[0].query.orderBy, now, r.variables)
		batch.Old = orderResults(batch.Old, plans[0].query.orderBy, now, r.variables)
		batch.New = applyResultWindow(batch.New, plans[0].query)
		batch.Old = applyResultWindow(batch.Old, plans[0].query)
	}
	if len(plans) > 0 && len(plans[0].query.orderBy) > 0 {
		batch.New = orderRowRecogResults(batch.New, plans[0].query.orderBy, now, r.variables)
		batch.Old = orderRowRecogResults(batch.Old, plans[0].query.orderBy, now, r.variables)
	}
	// Capture the counters before resetting the per-output values.  Esper makes
	// the same output context available to both the OUTPUT WHEN predicate and
	// its THEN assignments, so an assignment such as
	// `set observed = count_insert` must see the count that caused this output,
	// rather than the zeroed state for the next output interval.
	outputInsertCount := r.outputState.insertCount
	outputRemoveCount := r.outputState.removeCount
	outputInsertTotal := r.outputState.insertTotal
	outputRemoveTotal := r.outputState.removeTotal
	lastOutputAt := r.outputState.lastOutputAt
	r.outputState.insertCount = 0
	r.outputState.removeCount = 0
	r.outputState.lastOutputAt = now
	if len(policy.Then) == 0 {
		return batch
	}
	working := cloneValues(r.variables)
	assignments := make([]VariableAssignment, 0, len(policy.Then))
	for _, assignment := range policy.Then {
		value := assignment.Expr.eval(EvalContext{
			Now:                  now,
			Variables:            working,
			OutputInsertCount:    outputInsertCount,
			OutputRemoveCount:    outputRemoveCount,
			OutputInsertTotal:    outputInsertTotal,
			OutputRemoveTotal:    outputRemoveTotal,
			OutputLastOutputTime: lastOutputAt,
		})
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
	if r == nil || r.outputState == nil || (policy.Kind != OutputEveryTimePolicy && policy.Kind != OutputLastEveryTimePolicy) || policy.Interval <= 0 {
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
	if r == nil || r.outputState == nil || (policy.Kind != OutputEveryTimePolicy && policy.Kind != OutputLastEveryTimePolicy) || policy.Interval <= 0 {
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
	if updateStreamTargetsNamedWindow(s.plan.query) {
		// A named-window update fires only from the window insert
		// preprocessing (Esper's update strategy on the window); it never
		// consumes the window delta as a subscriber.
		return ResultBatch{}, false, nil
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
		if joinDelta.unidirectionalTrigger {
			r.aggregateState = nil
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
	if queryUsesPreviousAccess(plan.query) {
		r.trackNamedWindowPriorArrival(&result)
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

// queryUsesPreviousAccess reports whether any projection expression reads
// previous or prior event access. Named-window consumers then maintain the
// arrival history those expressions evaluate against.
func queryUsesPreviousAccess(query Query) bool {
	for _, selection := range query.selections {
		if selection.Expr != nil && expressionContainsPreviousAccess(selection.Expr.node()) {
			return true
		}
	}
	return false
}

// trackNamedWindowPriorArrival maintains the per-statement arrival history
// behind prior/prev expressions on named-window consumers. Esper evaluates
// prior access against the consumer stream's retained arrival order: each new
// event carries the arrival prefix ending with itself and each leaving event
// carries the prefix ending with its own retained position.
func (r *statementRuntime) trackNamedWindowPriorArrival(result *eventDelta) {
	if result.priorByEvent == nil {
		result.priorByEvent = make(map[string][]Event)
	}
	for _, event := range result.newEvents {
		r.namedWindowArrival = append(r.namedWindowArrival, event)
		result.priorByEvent[eventIdentity(event)] = append([]Event(nil), r.namedWindowArrival...)
	}
	addPriorHistories(result.priorByEvent, r.namedWindowArrival, result.oldEvents)
	removeArrivalEvents(&r.namedWindowArrival, result.oldEvents)
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
			query := s.runtime.query
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
		query := s.runtime.query
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
		if current.kind == streamMethod {
			return true
		}
	}
	return false
}

// isEvaluateOnceSource reports whether the source chain bottoms out in a
// method source marked EvaluateOnce. Such sides are seeded once at deployment
// and retained like a read-only data window: trigger cycles neither clear nor
// re-poll them.
func isEvaluateOnceSource(node *streamNode) bool {
	base, err := sourceNode(node)
	return err == nil && base != nil && base.kind == streamMethod && base.method != nil && base.method.evaluateOnce
}

// joinCycleOrder returns the per-trigger evaluation order with event-driven
// sides ahead of lookup sides (historical/method), preserving the dependency
// order within each group. Dependency-free method sides re-poll against the
// retained rows of the side that accepted the trigger, so the trigger insert
// must happen first.
func joinCycleOrder(sources []*streamNode, evaluationOrder []int) []int {
	eventDriven := make([]int, 0, len(evaluationOrder))
	lookup := make([]int, 0, len(evaluationOrder))
	for _, index := range evaluationOrder {
		base, err := sourceNode(sources[index])
		if err == nil && base != nil && joinSourceIsEventDriven(base) {
			eventDriven = append(eventDriven, index)
			continue
		}
		lookup = append(lookup, index)
	}
	return append(eventDriven, lookup...)
}

// joinSourceIsEventDriven reports whether the base source produces events of
// its own (event stream, pattern, contained or derived sources), as opposed
// to a current-state or lookup source (named window, table, historical or
// method source).
func joinSourceIsEventDriven(base *streamNode) bool {
	switch base.kind {
	case streamSource, streamPattern, streamContained, streamDerived:
		return true
	default:
		return false
	}
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
	evaluationOrder, err := methodJoinEvaluationOrder(definition)
	if err != nil {
		return joinDelta{}, err
	}
	hasEventDrivenSource := false
	for _, source := range sources {
		if base, baseErr := sourceNode(source); baseErr == nil && base != nil && joinSourceIsEventDriven(base) {
			hasEventDrivenSource = true
			break
		}
	}
	if joinDefinitionHasUnidirectional(definition) {
		return r.updateUnidirectionalJoin(definition, sources, evaluationOrder, now, newEvents, oldEvents)
	}
	before := joinKeyedTuples(definition, r.joinState, now, r)
	for _, event := range newEvents {
		// Historical (SQL) and subordinate method results belong to exactly
		// one trigger cycle. Clear those sides before evaluating the
		// dependency graph so a subordinate source can never observe a
		// previous trigger's rows. Dependency-free method rows instead
		// persist per trigger lineage: Esper polls a method stream once per
		// triggering event and retains the result rows inside the join
		// windows until the owning event expires.
		rebuiltSides := make(map[int][]storedEvent)
		for index, source := range sources {
			if containsHistoricalSource(source) && !isEvaluateOnceSource(source) {
				if base, baseErr := sourceNode(source); baseErr == nil && base != nil &&
					base.kind == streamMethod && base.method != nil && len(base.method.dependencies) == 0 {
					continue
				}
				rebuiltSides[index] = r.joinState.sides[index]
				r.joinState.sides[index] = nil
			}
		}
		// Event-driven sides insert the arriving event ahead of lookup sides
		// so a dependency-free method side polls the newly accepted rows of
		// the side that accepted this trigger.
		triggerSides := make([]int, 0, len(sources))
		triggerNewRows := make(map[int][]storedEvent, len(sources))
		for _, index := range joinCycleOrder(sources, evaluationOrder) {
			source := sources[index]
			base, baseErr := sourceNode(source)
			if baseErr != nil {
				return joinDelta{}, baseErr
			}
			if base.kind == streamMethod && base.method != nil && base.method.evaluateOnce {
				// Evaluate-once rows were seeded at deployment and are retained
				// across triggers; Esper does not re-poll dependency-free method
				// streams once the statement is running.
				continue
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
				r.joinState.sides[index] = r.assignJoinLineageIDs(side, r.joinState.sides[index])
				continue
			}
			if base.kind == streamMethod && base.method != nil && len(base.method.dependencies) > 0 {
				invocations, invocationErr := methodDependencyInvocations(base.method.dependencies, sources, r.joinState)
				if invocationErr != nil {
					return joinDelta{}, invocationErr
				}
				for _, invocation := range invocations {
					previous := r.methodDependencies
					r.methodDependencies = invocation.events
					delta, insertErr := r.insert(source, event, now)
					r.methodDependencies = previous
					if insertErr != nil {
						return joinDelta{}, insertErr
					}
					removeStoredEvents(&r.joinState.sides[index], delta.oldEvents)
					for _, newEvent := range delta.newEvents {
						r.joinState.sides[index] = append(r.joinState.sides[index], storedEvent{
							event: newEvent, receivedAt: now, lineageID: r.nextJoinLineageID(), lineage: cloneMethodLineage(invocation.lineage),
						})
					}
				}
				// Rebuilt subordinate rows keep their previous join lineage
				// when the re-poll returns equal content, so a trigger that
				// changes nothing nets out of the lineage-keyed tuple diff
				// exactly the way Esper's per-event composition produces no
				// rows for it.
				r.joinState.sides[index] = r.assignJoinLineageIDs(r.joinState.sides[index], rebuiltSides[index])
				continue
			}
			if base.kind == streamMethod && base.method != nil {
				if len(triggerSides) > 0 {
					// A dependency-free method side polls once per newly
					// accepted trigger row, mirroring Esper's per-event method
					// poll. The result rows carry the accepting row's lineage
					// so they join strictly inside that row's tuple, persist
					// while it is retained, and vanish when it expires.
					if base.method.trigger != "" && base.method.trigger != event.TypeName() {
						continue
					}
					for _, triggerIndex := range triggerSides {
						for _, triggerRow := range triggerNewRows[triggerIndex] {
							events, pollErr := base.method.provider.Poll(r.context(), MethodRequest{
								Trigger:      triggerRow.event,
								Now:          now,
								Variables:    visibleVariableValues(r.variables),
								Parameters:   parameterValuesFromVariables(r.variables),
								Dependencies: cloneMethodDependencies(r.methodDependencies),
								Invocation:   r.methodInvocationContext(base.sourceName),
							})
							if pollErr != nil {
								return joinDelta{}, pollErr
							}
							for _, newEvent := range events {
								r.joinState.sides[index] = append(r.joinState.sides[index], storedEvent{
									event: newEvent, receivedAt: now,
									lineageID: r.nextJoinLineageID(),
									lineage:   map[int]uint64{triggerIndex: triggerRow.lineageID},
								})
							}
						}
					}
					continue
				}
				if hasEventDrivenSource {
					// The statement subscribes to real event streams, so a
					// cycle without an accepted trigger row carries no driver
					// for the method side: Esper never polls a method stream
					// for an event its sibling streams did not accept.
					continue
				}
				// A triggerless method-only statement instead re-polls a
				// dependency-free method side wholesale through the generic
				// insert below and replaces its previous rows, mirroring
				// Esper's iterator-only refresh of variable-driven method
				// statements.
				rebuiltSides[index] = r.joinState.sides[index]
				r.joinState.sides[index] = nil
			}
			delta, err := r.insert(source, event, now)
			if err != nil {
				return joinDelta{}, err
			}
			removeStoredEventsCascade(r.joinState, index, delta.oldEvents)
			for _, newEvent := range delta.newEvents {
				stored := storedEvent{event: newEvent, receivedAt: now, lineageID: r.nextJoinLineageID()}
				r.joinState.sides[index] = append(r.joinState.sides[index], stored)
				if joinSourceIsEventDriven(base) {
					if len(triggerNewRows[index]) == 0 {
						triggerSides = append(triggerSides, index)
					}
					triggerNewRows[index] = append(triggerNewRows[index], stored)
				}
			}
			if previous, rebuilt := rebuiltSides[index]; rebuilt {
				r.joinState.sides[index] = r.assignJoinLineageIDs(r.joinState.sides[index], previous)
			}
		}
	}
	for _, event := range oldEvents {
		for index, source := range sources {
			delta, err := r.remove(source, event, now)
			if err != nil {
				return joinDelta{}, err
			}
			removeStoredEventsCascade(r.joinState, index, delta.oldEvents)
		}
	}
	after := joinKeyedTuples(definition, r.joinState, now, r)
	delta := diffJoinKeyedTuples(before, after)
	return joinDeltaWithPairs(delta), nil
}

// containedJoinSourceForDriver reports whether source is a contained-event
// expansion of the same logical source as driver. Esper evaluates this shape
// as a per-driver-event probe: the parent event is expanded independently for
// each contained source and the contained rows from an earlier parent must
// not participate in a later unidirectional probe. An explicit view after the
// contained expansion changes that contract and remains stateful.
func containedJoinSourceForDriver(source, driver *streamNode) bool {
	if source == nil || driver == nil {
		return false
	}
	contained := false
	for current := source; current != nil; current = current.input {
		switch current.kind {
		case streamFilter:
			continue
		case streamContained:
			contained = true
		case streamWindow:
			return false
		default:
			if !contained {
				return false
			}
			return equivalentJoinSource(current, driver)
		}
	}
	return false
}

func equivalentJoinSource(left, right *streamNode) bool {
	if left == nil || right == nil {
		return false
	}
	if left == right {
		return true
	}
	leftBase, leftErr := sourceNode(left)
	rightBase, rightErr := sourceNode(right)
	if leftErr != nil || rightErr != nil || leftBase == nil || rightBase == nil {
		return false
	}
	if leftBase.kind != rightBase.kind || leftBase.kind != streamSource {
		return false
	}
	return leftBase.sourceName == rightBase.sourceName
}

func (r *statementRuntime) updateUnidirectionalJoin(definition *joinDefinition, sources []*streamNode, evaluationOrder []int, now time.Time, newEvents, oldEvents []Event) (joinDelta, error) {
	flags := definition.unidirectional
	driverCount := joinDefinitionUnidirectionalCount(definition)
	result := joinDelta{}
	driver := -1
	if driverCount == 1 {
		for index, flag := range flags {
			if flag {
				driver = index
				break
			}
		}
	}
	for _, event := range newEvents {
		working := cloneJoinRuntimeState(r.joinState)
		rebuiltSides := make(map[int][]storedEvent)
		for index, source := range sources {
			if containsHistoricalSource(source) && !flags[index] && !isEvaluateOnceSource(source) {
				rebuiltSides[index] = r.joinState.sides[index]
				r.joinState.sides[index] = nil
				working.sides[index] = nil
			}
		}
		driverRows := make([][]storedEvent, len(sources))
		for _, index := range evaluationOrder {
			source := sources[index]
			base, err := sourceNode(source)
			if err != nil {
				return joinDelta{}, err
			}
			if base.kind == streamMethod && base.method != nil && base.method.evaluateOnce {
				continue
			}
			if base.kind == streamTable {
				side, snapshotErr := r.snapshotTableJoinSide(source, base, now)
				if snapshotErr != nil {
					return joinDelta{}, snapshotErr
				}
				if flags[index] {
					driverRows[index] = r.assignJoinLineageIDs(side, nil)
					working.sides[index] = driverRows[index]
				} else {
					r.joinState.sides[index] = r.assignJoinLineageIDs(side, r.joinState.sides[index])
					working.sides[index] = r.joinState.sides[index]
				}
				continue
			}
			if base.kind == streamMethod && base.method != nil && len(base.method.dependencies) > 0 {
				invocations, invocationErr := methodDependencyInvocations(base.method.dependencies, sources, working)
				if invocationErr != nil {
					return joinDelta{}, invocationErr
				}
				for _, invocation := range invocations {
					previous := r.methodDependencies
					r.methodDependencies = invocation.events
					delta, insertErr := r.insert(source, event, now)
					r.methodDependencies = previous
					if insertErr != nil {
						return joinDelta{}, insertErr
					}
					stored := make([]storedEvent, 0, len(delta.newEvents))
					for _, inserted := range delta.newEvents {
						stored = append(stored, storedEvent{event: inserted, receivedAt: now, lineageID: r.nextJoinLineageID(), lineage: cloneMethodLineage(invocation.lineage)})
					}
					if flags[index] {
						driverRows[index] = append(driverRows[index], stored...)
					} else {
						removeStoredEvents(&r.joinState.sides[index], delta.oldEvents)
						r.joinState.sides[index] = append(r.joinState.sides[index], stored...)
					}
				}
				if flags[index] {
					working.sides[index] = driverRows[index]
				} else {
					// Same-content re-poll rows keep their previous join
					// lineage so unchanged subordinate results net out of
					// the lineage-keyed tuple diff (see updateJoin).
					r.joinState.sides[index] = r.assignJoinLineageIDs(r.joinState.sides[index], rebuiltSides[index])
					working.sides[index] = r.joinState.sides[index]
				}
				continue
			}
			delta, insertErr := r.insert(source, event, now)
			if insertErr != nil {
				return joinDelta{}, insertErr
			}
			stored := make([]storedEvent, 0, len(delta.newEvents))
			for _, inserted := range delta.newEvents {
				stored = append(stored, storedEvent{event: inserted, receivedAt: now, lineageID: r.nextJoinLineageID()})
			}
			if flags[index] {
				driverRows[index] = append(driverRows[index], stored...)
				working.sides[index] = driverRows[index]
				continue
			}
			if driver >= 0 && containedJoinSourceForDriver(source, sources[driver]) {
				// A contained source rooted at the unidirectional parent is a
				// current-event probe, not a retained passive stream. Keep it in
				// the working tuple only; leaving r.joinState untouched prevents
				// rows from a previous parent event from leaking into the next
				// probe.
				working.sides[index] = stored
				continue
			}
			removeStoredEvents(&r.joinState.sides[index], delta.oldEvents)
			r.joinState.sides[index] = append(r.joinState.sides[index], stored...)
			if previous, rebuilt := rebuiltSides[index]; rebuilt {
				r.joinState.sides[index] = r.assignJoinLineageIDs(r.joinState.sides[index], previous)
			}
			working.sides[index] = r.joinState.sides[index]
		}

		if driverCount == len(sources) {
			anyDriverRow := false
			for _, rows := range driverRows {
				if len(rows) > 0 {
					anyDriverRow = true
					break
				}
			}
			if anyDriverRow {
				result.unidirectionalTrigger = true
				for _, tuple := range joinTuples(definition, working, now, r) {
					for index, event := range tuple {
						if index < len(flags) && flags[index] && event.TypeName() != "" {
							result.newTuples = append(result.newTuples, tuple)
							break
						}
					}
				}
			}
			continue
		}
		if driver < 0 || len(driverRows[driver]) == 0 {
			continue
		}
		result.unidirectionalTrigger = true
		for _, tuple := range joinTuples(definition, working, now, r) {
			if driver < len(tuple) && tuple[driver].TypeName() != "" {
				result.newTuples = append(result.newTuples, tuple)
			}
		}
	}
	for _, event := range oldEvents {
		for index, source := range sources {
			if flags[index] {
				continue
			}
			delta, err := r.remove(source, event, now)
			if err != nil {
				return joinDelta{}, err
			}
			removeStoredEvents(&r.joinState.sides[index], delta.oldEvents)
		}
	}
	return joinDeltaWithPairs(result), nil
}

type methodDependencyInvocation struct {
	events  map[string]Event
	lineage map[int]uint64
}

func methodDependencyInvocations(dependencies []string, sources []*streamNode, state *joinRuntimeState) ([]methodDependencyInvocation, error) {
	if len(dependencies) == 0 {
		return []methodDependencyInvocation{{}}, nil
	}
	byName := make(map[string]int, len(sources))
	for index, source := range sources {
		base, err := sourceNode(source)
		if err != nil {
			return nil, err
		}
		byName[base.logicalName()] = index
	}
	invocations := []methodDependencyInvocation{{events: make(map[string]Event), lineage: make(map[int]uint64)}}
	for _, rawName := range dependencies {
		name := strings.TrimSpace(rawName)
		dependencyIndex, ok := byName[name]
		if !ok || dependencyIndex < 0 || dependencyIndex >= len(state.sides) {
			return nil, NewError(ErrorUnknownName, fmt.Sprintf("unknown method dependency %q", name))
		}
		if len(state.sides[dependencyIndex]) == 0 {
			return nil, nil
		}
		next := make([]methodDependencyInvocation, 0, len(invocations)*len(state.sides[dependencyIndex]))
		for _, invocation := range invocations {
			for _, selected := range state.sides[dependencyIndex] {
				lineage := cloneMethodLineage(invocation.lineage)
				if !mergeMethodLineage(lineage, selected.lineage) || !mergeMethodLineage(lineage, map[int]uint64{dependencyIndex: selected.lineageID}) {
					continue
				}
				events := cloneMethodDependencies(invocation.events)
				events[name] = selected.event
				next = append(next, methodDependencyInvocation{events: events, lineage: lineage})
			}
		}
		invocations = next
	}
	return invocations, nil
}

func mergeMethodLineage(target map[int]uint64, additions map[int]uint64) bool {
	for index, lineageID := range additions {
		if existing, exists := target[index]; exists && existing != lineageID {
			return false
		}
		target[index] = lineageID
	}
	return true
}

func cloneMethodLineage(lineage map[int]uint64) map[int]uint64 {
	cloned := make(map[int]uint64, len(lineage))
	for index, lineageID := range lineage {
		cloned[index] = lineageID
	}
	return cloned
}

func cloneMethodDependencies(dependencies map[string]Event) map[string]Event {
	cloned := make(map[string]Event, len(dependencies))
	for name, event := range dependencies {
		cloned[name] = event
	}
	return cloned
}

func (r *statementRuntime) methodInvocationContext(sourceName string) MethodInvocationContext {
	invocation := MethodInvocationContext{SourceName: sourceName, ContextPartitionID: -1}
	if r == nil {
		return invocation
	}
	invocation.DeploymentID, invocation.StatementName, _ = strings.Cut(r.rowRecogOwner, ":")
	if invocation.StatementName == "" {
		invocation.StatementName = r.query.name
	}
	invocation.ContextName = r.partitionContextName
	if invocation.ContextName != "" {
		invocation.ContextPartitionID = r.partitionID
	}
	return invocation
}

func (r *statementRuntime) nextJoinLineageID() uint64 {
	r.joinLineageSeq++
	return r.joinLineageSeq
}

func (r *statementRuntime) assignJoinLineageIDs(events, previous []storedEvent) []storedEvent {
	used := make([]bool, len(previous))
	for index := range events {
		for previousIndex, stored := range previous {
			if !used[previousIndex] && sameEvent(events[index].event, stored.event) {
				events[index].lineageID = stored.lineageID
				used[previousIndex] = true
				break
			}
		}
		if events[index].lineageID == 0 {
			events[index].lineageID = r.nextJoinLineageID()
		}
	}
	return events
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
	leftValue := condition.Left.eval(EvalContext{Event: events[leftSource], JoinEvents: events, OuterEvent: events[leftSource], Now: now, Variables: variables})
	rightValue := condition.Right.eval(EvalContext{Event: events[rightSource], JoinEvents: events, OuterEvent: events[rightSource], Now: now, Variables: variables})
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

// removeStoredEventsCascade removes the matching rows of side index and then
// every row on any other side whose lineage binds one of the removed rows.
// Dependency-free method rows persist across trigger cycles bound to the
// trigger row that produced them, so they must disappear together with that
// row to mirror Esper's per-event method result retention.
func removeStoredEventsCascade(state *joinRuntimeState, index int, removed []Event) {
	if state == nil || index < 0 || index >= len(state.sides) || len(removed) == 0 {
		return
	}
	removedIDs := make(map[uint64]struct{}, len(removed))
	for _, event := range removed {
		for position, stored := range state.sides[index] {
			if reflect.DeepEqual(stored.event.Underlying(), event.Underlying()) && stored.event.TypeName() == event.TypeName() {
				removedIDs[stored.lineageID] = struct{}{}
				state.sides[index] = append(state.sides[index][:position], state.sides[index][position+1:]...)
				break
			}
		}
	}
	if len(removedIDs) == 0 {
		return
	}
	for side := range state.sides {
		if side == index {
			continue
		}
		kept := state.sides[side][:0]
		for _, stored := range state.sides[side] {
			if bound, ok := stored.lineage[index]; ok {
				if _, gone := removedIDs[bound]; gone {
					continue
				}
			}
			kept = append(kept, stored)
		}
		state.sides[side] = kept
	}
}

func (r *statementRuntime) expireJoin(now time.Time) (joinDelta, error) {
	if r.joinState == nil {
		return joinDelta{}, nil
	}
	definition := r.query.join
	if definition == nil {
		return joinDelta{}, nil
	}
	before := joinKeyedTuples(definition, r.joinState, now, r)
	delta := r.expire(now)
	for index := range r.joinState.sides {
		removeStoredEventsCascade(r.joinState, index, delta.oldEvents)
	}
	timedEvents, err := r.advancePatternJoinSources(definition, now)
	if err != nil {
		return joinDelta{}, err
	}
	timedDelta, err := r.applyPatternJoinTime(definition, timedEvents, now)
	if err != nil {
		return joinDelta{}, err
	}
	if joinDefinitionHasUnidirectional(definition) {
		return timedDelta, nil
	}
	after := joinKeyedTuples(definition, r.joinState, now, r)
	return joinDeltaWithPairs(diffJoinKeyedTuples(before, after)), nil
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

// joinKeyedTuple pairs a composed join tuple with its lineage identity, the
// composite of the per-side join lineage IDs. Esper composes join insert and
// remove rows per arriving or departing event, so a batch rollover that
// replaces a row with equal content still produces the full old/new pair:
// the lineage key keeps those rows distinct where a content multiset diff
// would net them out, while refreshed current-state rows keep their lineage
// (assignJoinLineageIDs) and still match.
type joinKeyedTuple struct {
	events []Event
	key    string
}

func joinStoredTupleLineageKey(stored []storedEvent) string {
	parts := make([]string, len(stored))
	for index, entry := range stored {
		parts[index] = strconv.FormatUint(entry.lineageID, 10)
	}
	return strings.Join(parts, "|")
}

func joinTuples(definition *joinDefinition, state *joinRuntimeState, now time.Time, runtime *statementRuntime) [][]Event {
	keyed := joinKeyedTuples(definition, state, now, runtime)
	if len(keyed) == 0 {
		return nil
	}
	result := make([][]Event, len(keyed))
	for index, tuple := range keyed {
		result[index] = tuple.events
	}
	return result
}

func joinKeyedTuples(definition *joinDefinition, state *joinRuntimeState, now time.Time, runtime *statementRuntime) []joinKeyedTuple {
	if definition == nil || state == nil {
		return nil
	}
	sources := joinDefinitionSources(definition)
	if len(sources) < 2 || len(state.sides) != len(sources) {
		return nil
	}
	if len(definition.edges) > 0 {
		return joinChainedKeyedTuples(definition, state, now, runtime)
	}
	conditions := joinDefinitionConditions(definition)
	if len(sources) == 2 && definition.kind != JoinInner {
		result := make([]joinKeyedTuple, 0)
		matchedRight := make(map[int]bool)
		for _, left := range state.sides[0] {
			matched := false
			for rightIndex, right := range state.sides[1] {
				storedTuple := []storedEvent{left, right}
				if !joinStoredTupleMatchesLineage(storedTuple) {
					continue
				}
				tuple := []Event{left.event, right.event}
				if joinConditionsMatch(conditions, tuple, now, runtime) {
					matched = true
					matchedRight[rightIndex] = true
					result = append(result, joinKeyedTuple{events: tuple, key: joinStoredTupleLineageKey(storedTuple)})
				}
			}
			if !matched && (definition.kind == JoinLeftOuter || definition.kind == JoinFullOuter) {
				storedTuple := []storedEvent{left, {}}
				result = append(result, joinKeyedTuple{events: []Event{left.event, Event{}}, key: joinStoredTupleLineageKey(storedTuple)})
			}
		}
		if definition.kind == JoinRightOuter || definition.kind == JoinFullOuter {
			for rightIndex, right := range state.sides[1] {
				if !matchedRight[rightIndex] {
					storedTuple := []storedEvent{{}, right}
					result = append(result, joinKeyedTuple{events: []Event{Event{}, right.event}, key: joinStoredTupleLineageKey(storedTuple)})
				}
			}
		}
		return result
	}
	if len(sources) > 2 && definition.kind != JoinInner {
		return joinOuterKeyedTuples(definition, state, now, runtime)
	}
	for _, side := range state.sides {
		if len(side) == 0 {
			return nil
		}
	}
	result := make([]joinKeyedTuple, 0)
	current := make([]Event, 0, len(sources))
	currentStored := make([]storedEvent, 0, len(sources))
	var visit func(int)
	visit = func(index int) {
		if index == len(state.sides) {
			if !joinStoredTupleMatchesLineage(currentStored) {
				return
			}
			candidate := append([]Event(nil), current...)
			if joinConditionsMatch(conditions, candidate, now, runtime) {
				result = append(result, joinKeyedTuple{events: candidate, key: joinStoredTupleLineageKey(currentStored)})
			}
			return
		}
		for _, stored := range state.sides[index] {
			current = append(current, stored.event)
			currentStored = append(currentStored, stored)
			visit(index + 1)
			current = current[:len(current)-1]
			currentStored = currentStored[:len(currentStored)-1]
		}
	}
	visit(0)
	return result
}

type chainedJoinTuple struct {
	events []Event
	stored []storedEvent
}

// joinChainedTuples evaluates the explicit left-deep join tree edge by edge.
// This preserves intermediate unmatched rows, which cannot be represented by
// the legacy JoinMany single-kind N-way Cartesian implementation.
func joinChainedKeyedTuples(definition *joinDefinition, state *joinRuntimeState, now time.Time, runtime *statementRuntime) []joinKeyedTuple {
	if definition == nil || state == nil || len(definition.edges) != len(state.sides)-1 || len(state.sides) < 2 {
		return nil
	}
	rows := make([]chainedJoinTuple, 0, len(state.sides[0]))
	for _, stored := range state.sides[0] {
		rows = append(rows, chainedJoinTuple{events: []Event{stored.event}, stored: []storedEvent{stored}})
	}
	for edgeIndex, edge := range definition.edges {
		rightSide := state.sides[edgeIndex+1]
		matchedRight := make([]bool, len(rightSide))
		next := make([]chainedJoinTuple, 0)
		for _, left := range rows {
			if edge.kind != JoinInner && !joinChainedEdgeAnchored(edge, edgeIndex, left.events) {
				// The edge cannot be evaluated against this partial tuple: none
				// of the accumulated streams referenced by the edge conditions
				// carries an actual event. Esper's N-way outer assembly does not
				// eliminate such partial results at a later outer edge, so the
				// tuple passes through unchanged with a placeholder appended.
				events := append(append([]Event(nil), left.events...), Event{})
				stored := append(append([]storedEvent(nil), left.stored...), storedEvent{})
				next = append(next, chainedJoinTuple{events: events, stored: stored})
				continue
			}
			matchedLeft := false
			for rightIndex, right := range rightSide {
				events := append(append([]Event(nil), left.events...), right.event)
				stored := append(append([]storedEvent(nil), left.stored...), right)
				if !joinStoredTupleMatchesLineageWithMissing(stored) || !joinConditionsMatch(edge.conditions, events, now, runtime) {
					continue
				}
				matchedLeft = true
				matchedRight[rightIndex] = true
				next = append(next, chainedJoinTuple{events: events, stored: stored})
			}
			if !matchedLeft && (edge.kind == JoinLeftOuter || edge.kind == JoinFullOuter) && joinStoredTupleHasAnchor(left.stored) {
				events := append(append([]Event(nil), left.events...), Event{})
				stored := append(append([]storedEvent(nil), left.stored...), storedEvent{})
				next = append(next, chainedJoinTuple{events: events, stored: stored})
			}
		}
		if edge.kind == JoinRightOuter || edge.kind == JoinFullOuter {
			for rightIndex, right := range rightSide {
				if matchedRight[rightIndex] || len(right.lineage) > 0 {
					continue
				}
				events := make([]Event, edgeIndex+2)
				stored := make([]storedEvent, edgeIndex+2)
				events[edgeIndex+1] = right.event
				stored[edgeIndex+1] = right
				next = append(next, chainedJoinTuple{events: events, stored: stored})
			}
		}
		rows = next
	}
	result := make([]joinKeyedTuple, len(rows))
	for index, row := range rows {
		result[index] = joinKeyedTuple{events: row.events, key: joinStoredTupleLineageKey(row.stored)}
	}
	return result
}

// joinOuterTuples enumerates matching N-way tuples first, then emits one
// placeholder tuple for every unmatched event on the selected outer edge.
// A full outer join emits unmatched events from every source, including
// sources between the first and last positions. This is intentionally a
// semantic reference implementation; optimized indexed joins can replace it
// without changing the public contract.
func joinOuterKeyedTuples(definition *joinDefinition, state *joinRuntimeState, now time.Time, runtime *statementRuntime) []joinKeyedTuple {
	sources := joinDefinitionSources(definition)
	conditions := joinDefinitionConditions(definition)
	result := make([]joinKeyedTuple, 0)
	matched := make([][]bool, len(state.sides))
	for index := range state.sides {
		matched[index] = make([]bool, len(state.sides[index]))
	}
	current := make([]Event, 0, len(sources))
	currentStored := make([]storedEvent, 0, len(sources))
	currentIndexes := make([]int, 0, len(sources))
	var visit func(int)
	visit = func(index int) {
		if index == len(state.sides) {
			if !joinStoredTupleMatchesLineage(currentStored) {
				return
			}
			candidate := append([]Event(nil), current...)
			if !joinConditionsMatch(conditions, candidate, now, runtime) {
				return
			}
			result = append(result, joinKeyedTuple{events: candidate, key: joinStoredTupleLineageKey(currentStored)})
			for sourceIndex, sideIndex := range currentIndexes {
				matched[sourceIndex][sideIndex] = true
			}
			return
		}
		for sideIndex, stored := range state.sides[index] {
			current = append(current, stored.event)
			currentStored = append(currentStored, stored)
			currentIndexes = append(currentIndexes, sideIndex)
			visit(index + 1)
			current = current[:len(current)-1]
			currentStored = currentStored[:len(currentStored)-1]
			currentIndexes = currentIndexes[:len(currentIndexes)-1]
		}
	}
	visit(0)

	if definition.kind == JoinLeftOuter {
		for index, stored := range state.sides[0] {
			if matched[0][index] || len(stored.lineage) > 0 {
				continue
			}
			tuple := make([]Event, len(sources))
			tuple[0] = stored.event
			storedTuple := make([]storedEvent, len(sources))
			storedTuple[0] = stored
			result = append(result, joinKeyedTuple{events: tuple, key: joinStoredTupleLineageKey(storedTuple)})
		}
	}
	if definition.kind == JoinRightOuter {
		last := len(sources) - 1
		for index, stored := range state.sides[last] {
			if matched[last][index] || len(stored.lineage) > 0 {
				continue
			}
			tuple := make([]Event, len(sources))
			tuple[last] = stored.event
			storedTuple := make([]storedEvent, len(sources))
			storedTuple[last] = stored
			result = append(result, joinKeyedTuple{events: tuple, key: joinStoredTupleLineageKey(storedTuple)})
		}
	}
	if definition.kind == JoinFullOuter {
		for sourceIndex, side := range state.sides {
			for index, stored := range side {
				if matched[sourceIndex][index] || len(stored.lineage) > 0 {
					continue
				}
				tuple := make([]Event, len(sources))
				tuple[sourceIndex] = stored.event
				storedTuple := make([]storedEvent, len(sources))
				storedTuple[sourceIndex] = stored
				result = append(result, joinKeyedTuple{events: tuple, key: joinStoredTupleLineageKey(storedTuple)})
			}
		}
	}
	return result
}

func joinStoredTupleMatchesLineage(tuple []storedEvent) bool {
	for _, stored := range tuple {
		for dependencyIndex, dependencyLineageID := range stored.lineage {
			if dependencyIndex < 0 || dependencyIndex >= len(tuple) || tuple[dependencyIndex].lineageID != dependencyLineageID {
				return false
			}
		}
	}
	return true
}

// joinChainedEdgeAnchored reports whether an outer edge can be evaluated
// against the accumulated partial tuple: at least one accumulated stream
// referenced by the edge's conditions must carry an actual (non-placeholder)
// event. Edges without conditions always apply (cross product semantics).
func joinChainedEdgeAnchored(edge joinEdgeDefinition, edgeIndex int, events []Event) bool {
	if len(edge.conditions) == 0 {
		return true
	}
	for source := 0; source <= edgeIndex && source < len(events); source++ {
		if events[source].underlying == nil {
			continue
		}
		for _, condition := range edge.conditions {
			if joinConditionReferencesSource(condition, source) {
				return true
			}
		}
	}
	return false
}

// joinStoredTupleHasAnchor reports whether the tuple holds at least one
// event that is not bound to a method dependency. Esper generates
// subordinate method/historical rows strictly inside their dependency
// tuple's context, so an outer-join placeholder tuple composed solely of
// dependency-bound rows can never occur; such rows simply vanish when their
// owning tuple does not join. Real stream events, table rows and
// evaluate-once seeded rows carry no dependency lineage and anchor
// placeholders normally.
func joinStoredTupleHasAnchor(tuple []storedEvent) bool {
	for _, stored := range tuple {
		if len(stored.lineage) == 0 {
			return true
		}
	}
	return false
}

func joinStoredTupleMatchesLineageWithMissing(tuple []storedEvent) bool {
	for _, stored := range tuple {
		for dependencyIndex, dependencyLineageID := range stored.lineage {
			if dependencyIndex < 0 || dependencyIndex >= len(tuple) {
				return false
			}
			actual := tuple[dependencyIndex].lineageID
			if actual != 0 && actual != dependencyLineageID {
				return false
			}
		}
	}
	return true
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

// diffJoinKeyedTuples matches before/after tuples by lineage identity as a
// multiset. Rows that keep their lineage (unchanged or refreshed
// current-state rows) cancel out; rows that left or arrived — including
// same-content replacements from a batch rollover — surface as old and new
// join rows, matching Esper's per-event join composition.
func diffJoinKeyedTuples(before, after []joinKeyedTuple) joinDelta {
	result := joinDelta{}
	used := make([]bool, len(after))
	for _, oldTuple := range before {
		found := -1
		for index, newTuple := range after {
			if !used[index] && oldTuple.key == newTuple.key {
				found = index
				break
			}
		}
		if found >= 0 {
			used[found] = true
		} else {
			result.oldTuples = append(result.oldTuples, oldTuple.events)
		}
	}
	for index, newTuple := range after {
		if !used[index] {
			result.newTuples = append(result.newTuples, newTuple.events)
		}
	}
	return result
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
	case streamMethod:
		if node.method == nil || node.method.provider == nil {
			return eventDelta{}, NewError(ErrorDependency, fmt.Sprintf("method source %q has no provider", node.sourceName))
		}
		if node.method.trigger != "" && node.method.trigger != event.TypeName() {
			return eventDelta{}, nil
		}
		events, err := node.method.provider.Poll(r.context(), MethodRequest{
			Trigger: event, Now: now,
			Variables: visibleVariableValues(r.variables), Parameters: parameterValuesFromVariables(r.variables),
			Dependencies: cloneMethodDependencies(r.methodDependencies),
			Invocation:   r.methodInvocationContext(node.sourceName),
		})
		if err != nil {
			return eventDelta{}, err
		}
		return eventDelta{newEvents: append([]Event(nil), events...)}, nil
	case streamContained:
		if node.contained == nil || node.contained.property == nil {
			return eventDelta{}, NewError(ErrorInvalidRule, fmt.Sprintf("unnest source %q has no contained property", node.sourceName))
		}
		inputDelta, err := r.insert(node.input, event, now)
		if err != nil {
			return eventDelta{}, err
		}
		childSchema, err := r.query.env.sourceSchema(node)
		if err != nil {
			return eventDelta{}, err
		}
		newEvents, err := expandContainedEvents(r.query.env, childSchema, node.contained, inputDelta.newEvents, now, r.variables)
		if err != nil {
			return eventDelta{}, err
		}
		oldEvents, err := expandContainedEvents(r.query.env, childSchema, node.contained, inputDelta.oldEvents, now, r.variables)
		if err != nil {
			return eventDelta{}, err
		}
		return eventDelta{newEvents: newEvents, oldEvents: oldEvents}, nil
	case streamDerived:
		if node.input == nil || node.derived == nil || node.derived.aggregate == nil {
			return eventDelta{}, NewError(ErrorInvalidRule, fmt.Sprintf("derived source %q has no input or aggregate", node.sourceName))
		}
		inputDelta, err := r.insert(node.input, event, now)
		if err != nil {
			return eventDelta{}, err
		}
		return r.insertDerived(node, inputDelta, now)
	case streamPattern:
		return r.insertPatternSource(node, event, now)
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
			value := node.predicate.eval(EvalContext{
				Event:                candidate,
				OuterEvent:           candidate,
				ContainedParentEvent: containedParentEvent(candidate),
				Engine:               r.engine,
				History:              historyForEvent(inputDelta, candidate),
				Now:                  now,
				Variables:            r.variables,
			})
			if ok, isBool := boolValue(value); isBool && ok {
				filtered.newEvents = append(filtered.newEvents, candidate)
			}
		}
		for _, candidate := range inputDelta.oldEvents {
			value := node.predicate.eval(EvalContext{Event: candidate, OuterEvent: candidate, ContainedParentEvent: containedParentEvent(candidate), History: historyForEvent(inputDelta, candidate), Now: now, Variables: r.variables})
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
	case streamMethod:
		return eventDelta{}, nil
	case streamContained:
		if node.contained == nil || node.contained.property == nil {
			return eventDelta{}, NewError(ErrorInvalidRule, fmt.Sprintf("unnest source %q has no contained property", node.sourceName))
		}
		inputDelta, err := r.remove(node.input, event, now)
		if err != nil {
			return eventDelta{}, err
		}
		childSchema, err := r.query.env.sourceSchema(node)
		if err != nil {
			return eventDelta{}, err
		}
		oldEvents, err := expandContainedEvents(r.query.env, childSchema, node.contained, inputDelta.oldEvents, now, r.variables)
		if err != nil {
			return eventDelta{}, err
		}
		return eventDelta{oldEvents: oldEvents}, nil
	case streamDerived:
		if node.input == nil || node.derived == nil || node.derived.aggregate == nil {
			return eventDelta{}, NewError(ErrorInvalidRule, fmt.Sprintf("derived source %q has no input or aggregate", node.sourceName))
		}
		inputDelta, err := r.remove(node.input, event, now)
		if err != nil {
			return eventDelta{}, err
		}
		return r.insertDerived(node, inputDelta, now)
	case streamPattern:
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
			value := node.predicate.eval(EvalContext{Event: candidate, OuterEvent: candidate, ContainedParentEvent: containedParentEvent(candidate), History: historyForEvent(inputDelta, candidate), Now: now, Variables: r.variables})
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

func expandContainedEvents(env *Environment, schema Schema, definition *containedDefinition, parents []Event, now time.Time, variables map[string]Value) ([]Event, error) {
	if env == nil || !schema.valid() || definition == nil || definition.property == nil {
		return nil, NewError(ErrorInvalidRule, "unnest expansion is incomplete")
	}
	result := make([]Event, 0)
	for _, parent := range parents {
		value := definition.property.eval(EvalContext{Event: parent, ContainedParentEvent: containedParentEvent(parent), Now: now, Variables: variables})
		if value.IsMissing() || value.IsNull() {
			continue
		}
		items, err := containedCollectionItems(value.Any(), definition.sequence)
		if err != nil {
			return nil, err
		}
		for index, item := range items {
			if item.Kind() == reflect.Interface && !item.IsNil() {
				item = item.Elem()
			}
			if !item.IsValid() || (item.Kind() == reflect.Pointer && item.IsNil()) {
				continue
			}
			underlying := any(item.Interface())
			var err error
			if definition.wrap != nil {
				underlying, err = definition.wrap(item)
				if err != nil {
					return nil, fmt.Errorf("unnest child %d: %w", index, err)
				}
			}
			child, err := materializeContainedEvent(env, schema, underlying, parent.ReceivedAt())
			if err != nil {
				return nil, fmt.Errorf("unnest child %d: %w", index, err)
			}
			parentCopy := parent
			child.parent = &parentCopy
			result = append(result, child)
		}
	}
	return result, nil
}

func containedCollectionItems(value any, sequence bool) ([]reflect.Value, error) {
	collection := reflect.ValueOf(value)
	if !collection.IsValid() {
		return nil, fmt.Errorf("unnest property evaluated to %T, expected slice, array or iter.Seq", value)
	}
	if sequence {
		if collection.Kind() != reflect.Func {
			return nil, fmt.Errorf("unnest sequence property evaluated to %T, expected iter.Seq", value)
		}
		if collection.IsNil() {
			return nil, nil
		}
		yieldType := collection.Type().In(0)
		if yieldType.Kind() != reflect.Func || yieldType.NumIn() != 1 || yieldType.NumOut() != 1 || yieldType.Out(0).Kind() != reflect.Bool {
			return nil, fmt.Errorf("unnest sequence property evaluated to %T, expected iter.Seq", value)
		}
		items := make([]reflect.Value, 0)
		yield := reflect.MakeFunc(yieldType, func(args []reflect.Value) []reflect.Value {
			if len(args) == 1 {
				items = append(items, args[0])
			}
			return []reflect.Value{reflect.ValueOf(true)}
		})
		collection.Call([]reflect.Value{yield})
		return items, nil
	}
	if collection.Kind() != reflect.Slice && collection.Kind() != reflect.Array {
		return nil, fmt.Errorf("unnest property evaluated to %T, expected slice or array", value)
	}
	items := make([]reflect.Value, collection.Len())
	for index := 0; index < collection.Len(); index++ {
		items[index] = collection.Index(index)
	}
	return items, nil
}

// materializeContainedEvent applies the target type of an @type-style
// contained expansion. Event values are already adapted and therefore retain
// their concrete schema/identity after the target accepts them. Raw values
// are converted through the registered target schema, including JSON/XML
// textual payloads and map-to-struct materialization.
func materializeContainedEvent(env *Environment, schema Schema, value any, receivedAt time.Time) (Event, error) {
	if event, ok := value.(Event); ok {
		if !event.Schema().valid() {
			return Event{}, NewError(ErrorTypeMismatch, "contained Event has no valid schema")
		}
		if schema.kind == SchemaVariant {
			return newEvent(schema, event, receivedAt)
		}
		if !env.acceptsEventType(schema.Name(), event.TypeName()) {
			return Event{}, NewError(ErrorTypeMismatch, fmt.Sprintf("contained event type %q is not accepted by target %q", event.TypeName(), schema.Name()))
		}
		// Preserve the concrete event type and its underlying identity. The
		// expansion caller installs the immediate parent after this function.
		event.receivedAt = receivedAt
		return event, nil
	}
	if event, ok := value.(*Event); ok {
		if event == nil {
			return Event{}, NewError(ErrorTypeMismatch, "contained Event pointer is nil")
		}
		return materializeContainedEvent(env, schema, *event, receivedAt)
	}

	if schema.kind == SchemaVariant {
		return Event{}, NewError(ErrorTypeMismatch, fmt.Sprintf("target variant %q requires contained Event values", schema.Name()))
	}

	// JSON and XML @type expressions may return their textual wire form. Parse
	// it against the target schema before normal event construction so the
	// target's declared fields and null policy remain authoritative.
	if schema.kind == SchemaJSON || schema.kind == SchemaXML {
		var data []byte
		switch typed := value.(type) {
		case string:
			data = []byte(typed)
		case []byte:
			data = append([]byte(nil), typed...)
		}
		if data != nil {
			if schema.kind == SchemaJSON {
				return ParseJSON(schema, data, receivedAt)
			}
			return ParseXML(schema, data, receivedAt)
		}
	}

	underlying := value
	if schema.kind == SchemaStruct && schema.goType != nil {
		if values, ok := value.(map[string]any); ok {
			var err error
			underlying, err = mergeSchemaUnderlying(schema, nil, values)
			if err != nil {
				return Event{}, err
			}
		}
	}
	return newEvent(schema, underlying, receivedAt)
}

func containedParentEvent(event Event) Event {
	parent, ok := event.Parent()
	if !ok {
		return Event{}
	}
	return parent
}

func (r *statementRuntime) insertDerived(node *streamNode, delta eventDelta, now time.Time) (eventDelta, error) {
	if node == nil || node.derived == nil || node.derived.aggregate == nil || !node.derived.schema.valid() {
		return eventDelta{}, NewError(ErrorInvalidRule, "derived source is incomplete")
	}
	if len(delta.newEvents) == 0 && len(delta.oldEvents) == 0 {
		return eventDelta{}, nil
	}
	if r.derivedStates == nil {
		r.derivedStates = make(map[*streamNode]*aggregateRuntimeState)
	}
	state := r.derivedStates[node]
	if state == nil {
		state = &aggregateRuntimeState{groups: make(map[string]*aggregateGroup)}
		r.derivedStates[node] = state
	}
	definition := node.derived.aggregate
	temporaryQuery := Query{
		env:       r.query.env,
		input:     definition.input,
		aggregate: definition,
		selector:  SelectIRStream,
		name:      node.sourceName,
	}
	temporaryPlan := Plan{
		schemaVersion: planSchemaVersion,
		query:         temporaryQuery,
		resultSchema:  node.derived.schema,
	}
	previous := r.aggregateState
	r.aggregateState = state
	batch, err := r.aggregateBatch(delta, temporaryPlan, now)
	r.aggregateState = previous
	if err != nil {
		return eventDelta{}, err
	}
	result := eventDelta{}
	result.oldEvents, err = derivedResultEvents(node.derived.schema, batch.Old, now, "old", batch.Sequence)
	if err != nil {
		return eventDelta{}, err
	}
	result.newEvents, err = derivedResultEvents(node.derived.schema, batch.New, now, "new", batch.Sequence)
	if err != nil {
		return eventDelta{}, err
	}
	return result, nil
}

func derivedResultEvents(schema Schema, results []Result, receivedAt time.Time, stream string, sequence uint64) ([]Event, error) {
	if len(results) == 0 {
		return nil, nil
	}
	events := make([]Event, 0, len(results))
	for _, result := range results {
		row, ok := result.Row()
		if !ok {
			return nil, fmt.Errorf("derived source %q produced a non-row result", schema.Name())
		}
		values := make(map[string]any, len(schema.fields))
		for _, field := range schema.fields {
			value := row.Get(field.Name)
			if value.IsMissing() {
				continue
			}
			values[field.Name] = value.Any()
		}
		// Aggregate rows can carry identical visible values across adjacent
		// batches. Keep a private identity marker in the dynamic underlying map
		// so Join treats those view updates as distinct events, like Esper's
		// view-generated EventBean instances.
		values["__esper_derived_identity"] = fmt.Sprintf("%s:%d:%d", stream, sequence, len(events))
		event, err := newEvent(schema, values, receivedAt)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
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
		key := groupWindowKeys(window.effectiveKeys(), event, now, variables)
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
		key := groupWindowKeys(window.effectiveKeys(), event, now, r.variables)
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
	return groupWindowKeys([]Expr{expression}, event, now, variables)
}

// groupWindowKeys encodes the multi-key group identity of an event, mirroring
// Esper's #groupwin(propOne, propTwo) partition semantics.
func groupWindowKeys(expressions []Expr, event Event, now time.Time, variables map[string]Value) string {
	if len(expressions) == 0 {
		return encodeKey([]any{ValueMissing, nil})
	}
	values := make([]any, 0, len(expressions)*2)
	for _, expression := range expressions {
		if expression == nil {
			values = append(values, ValueMissing, nil)
			continue
		}
		value := expression.eval(EvalContext{Event: event, Now: now, Variables: variables})
		values = append(values, value.State(), value.Any())
	}
	return encodeKey(values)
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
	if event.identity != nil {
		return fmt.Sprintf("token:%p", event.identity)
	}
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
			if progress.node.schedulePeriod != nil || progress.node.scheduleExpr != nil {
				schedule, ok := patternTimerScheduleRuntimeFor(progress.node, at, progress.tags, progress.tagValues, variables)
				if !ok || schedule == nil || !schedule.active {
					progress.expired = true
					return
				}
				progress.schedulePeriod = schedule
			} else {
				progress.scheduleIndex = 0
			}
		}
	case patternTimerCronNode:
		if !progress.timerStarted {
			progress.timerStarted = true
			if progress.node.cron != nil {
				if resolved, err := progress.node.cron.resolve(EvalContext{
					Now:       at,
					Variables: variables,
					Tags:      progress.tags,
					TagValues: progress.tagValues,
				}); err == nil {
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
	deadline := at
	if node.calendar != nil {
		period := node.calendar
		deadline = deadline.AddDate(period.Years, period.Months, period.Days)
	}
	if node.duration <= 0 {
		if node.calendar == nil {
			return time.Time{}, false
		}
		return deadline, true
	}
	return deadline.Add(node.duration), true
}

func recordPatternTimerIntervalSchedule(state *patternRuntimeState, at time.Time, variables map[string]Value) {
	if state == nil {
		return
	}
	state.timerIntervalReference = at
	state.timerIntervalVariables = visibleVariableValues(variables)
}

func patternTimerIntervalVariablesChanged(state *patternRuntimeState, variables map[string]Value) bool {
	if state == nil || state.timerIntervalVariables == nil {
		return false
	}
	current := visibleVariableValues(variables)
	if len(current) != len(state.timerIntervalVariables) {
		return true
	}
	for name, prior := range state.timerIntervalVariables {
		value, exists := current[name]
		if !exists || !value.Equal(prior) {
			return true
		}
	}
	return false
}

// rearmPatternTimerIntervalIfVariablesChanged preserves the already scheduled
// first callback, then mirrors Esper's every-observer lifecycle for later
// callbacks: a changed variable is read when the next observer is armed from
// the preceding callback timestamp.
func rearmPatternTimerIntervalIfVariablesChanged(state *patternRuntimeState, node *patternNode, variables map[string]Value) bool {
	if state == nil || node == nil || node.durationExpr == nil || !state.timerIntervalFired || !patternTimerIntervalVariablesChanged(state, variables) {
		return true
	}
	deadline, ok := patternDurationDeadline(node, nil, state.timerIntervalReference, variables)
	if !ok {
		state.patternStopped = true
		state.timerNext = time.Time{}
		return false
	}
	state.timerNext = deadline
	state.timerIntervalVariables = visibleVariableValues(variables)
	return true
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
		if progress.schedulePeriod != nil {
			return progress.schedulePeriod.active && !progress.schedulePeriod.next.IsZero() && !progress.schedulePeriod.next.After(now)
		}
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
			merged[tag] = mergePatternEventHistory(merged[tag], events)
		}
	}
	return merged
}

// mergePatternEventHistory combines tag histories from nested progress
// branches. A restarted repetition inherits the parent history so predicates
// can still read outer/repeated tags; that inherited prefix must not be
// appended a second time when the child transition is merged back into the
// parent. Independent branches still append their non-overlapping histories.
func mergePatternEventHistory(existing, incoming []Event) []Event {
	if len(existing) == 0 {
		return append([]Event(nil), incoming...)
	}
	if len(incoming) == 0 {
		return append([]Event(nil), existing...)
	}
	if patternEventHistoryPrefix(existing, incoming) {
		return append([]Event(nil), incoming...)
	}
	if patternEventHistoryPrefix(incoming, existing) {
		return append([]Event(nil), existing...)
	}
	return append(append([]Event(nil), existing...), incoming...)
}

func patternEventHistoryPrefix(prefix, values []Event) bool {
	if len(prefix) > len(values) {
		return false
	}
	for index := range prefix {
		if prefix[index].identity == nil || values[index].identity == nil || prefix[index].identity != values[index].identity {
			return false
		}
	}
	return true
}

func clonePatternProgress(progress *patternProgress) *patternProgress {
	if progress == nil {
		return nil
	}
	copyProgress := *progress
	copyProgress.left = clonePatternProgress(progress.left)
	copyProgress.right = clonePatternProgress(progress.right)
	copyProgress.child = clonePatternProgress(progress.child)
	copyProgress.schedulePeriod = clonePatternTimerScheduleRuntime(progress.schedulePeriod)
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
	if progress.quit {
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

// patternRepeatingLegAlive reports whether a progress tree still holds a
// repeating leg (an Every node, which restarts its child after each firing)
// whose branch can produce further matches. Esper reports such completions
// with isQuitted=false: a followed-by with every-distinct legs keeps pairing
// later distinct events with the tags it already captured.
func patternRepeatingLegAlive(progress *patternProgress) bool {
	if progress == nil || progress.expired || progress.quit {
		return false
	}
	switch progress.node.kind {
	case patternEveryNode:
		return true
	case patternWithinNode:
		if progress.done || progress.expired {
			return false
		}
		return patternRepeatingLegAlive(progress.child)
	case patternSequenceNode, patternAndNode:
		return patternRepeatingLegAlive(progress.left) || patternRepeatingLegAlive(progress.right)
	case patternOrNode:
		return patternRepeatingLegAlive(progress.left) || patternRepeatingLegAlive(progress.right)
	}
	return false
}

// patternCompletionPermanent reports whether a completed progress tree cannot
// produce further matches, mirroring the isQuitted flag Esper propagates from
// a completed EvalNode: plain filters, sequences and and-expressions quit
// permanently when done, while every-expressions report their matches with
// isQuitted=false because they restart their child. An or-expression quits
// permanently exactly when one of its satisfied branches did, which is what
// kills the sibling branches (including an every-branch) of the or. Timer
// observers are excluded: their lifecycle is driven by the virtual-clock
// paths, not by event completions.
func patternCompletionPermanent(progress *patternProgress) bool {
	if progress == nil || progress.expired || !patternSatisfied(progress) {
		return false
	}
	if progress.quit {
		return true
	}
	switch progress.node.kind {
	case patternEveryNode:
		return false
	case patternWithinNode:
		if patternWithinCanContinue(progress) {
			return false
		}
		return !patternRepeatingLegAlive(progress.child)
	case patternSequenceNode, patternAndNode:
		// A completed followed-by/and branch with a repeating leg reports
		// isQuitted=false in Esper: every-distinct legs keep pairing later
		// distinct events with the tags the branch already captured.
		return !patternRepeatingLegAlive(progress.left) && !patternRepeatingLegAlive(progress.right)
	case patternOrNode:
		return (patternSatisfied(progress.left) && patternCompletionPermanent(progress.left)) ||
			(patternSatisfied(progress.right) && patternCompletionPermanent(progress.right))
	case patternTimerIntervalNode, patternTimerAtNode, patternTimerScheduleNode, patternTimerCronNode:
		return false
	default:
		return true
	}
}

// patternMatchWithinLimits applies both the legacy whole-pattern MaxStates
// limit and the per-edge FollowedByMax limit. The latter counts only active
// sequence progress that has consumed its left side and is waiting on the
// right side. Counting the materialized progress tree keeps nested and
// composed followed-by edges independent, matching Esper's subexpression
// scope instead of treating every active root as one shared bucket.
func patternMatchWithinLimits(active []patternMatch, candidate patternMatch, definition *patternDefinition) bool {
	allowed, _ := patternMatchLimit(active, candidate, definition)
	return allowed
}

type patternSequenceLimitViolation struct {
	edge      string
	maximum   int
	attempted int
}

func patternMatchLimit(active []patternMatch, candidate patternMatch, definition *patternDefinition) (bool, *patternSequenceLimitViolation) {
	if definition == nil {
		return true, nil
	}
	if definition.maxStates > 0 && len(active) >= definition.maxStates {
		return false, nil
	}
	counts := make(map[*patternNode]patternSequenceMaxCount)
	for _, match := range active {
		addPatternSequenceMaxCounts(counts, match.state)
	}
	addPatternSequenceMaxCounts(counts, candidate.state)
	violations := make([]patternSequenceLimitViolation, 0)
	for node, count := range counts {
		if count.maximum <= 0 {
			return false, nil
		}
		if count.count > count.maximum {
			violations = append(violations, patternSequenceLimitViolation{
				edge:      node.description(),
				maximum:   count.maximum,
				attempted: count.count,
			})
		}
	}
	if len(violations) == 0 {
		return true, nil
	}
	sort.Slice(violations, func(left, right int) bool {
		if violations[left].edge != violations[right].edge {
			return violations[left].edge < violations[right].edge
		}
		return violations[left].maximum < violations[right].maximum
	})
	return false, &violations[0]
}

func (r *statementRuntime) admitPatternMatch(active []patternMatch, candidate patternMatch, definition *patternDefinition) bool {
	return r.admitPatternMatchForContext(active, candidate, definition, "")
}

func (r *statementRuntime) admitPatternMatchForContext(active []patternMatch, candidate patternMatch, definition *patternDefinition, phase string) bool {
	allowed, violation := patternMatchLimit(active, candidate, definition)
	if allowed || violation == nil || r == nil || r.engine == nil {
		return allowed
	}
	deploymentID, statementName, _ := strings.Cut(r.rowRecogOwner, ":")
	contextName := r.partitionContextName
	partitionID := -1
	if contextName == "" {
		contextName = r.query.contextName
	} else {
		partitionID = r.partitionID
	}
	r.engine.queuePatternSubexpressionLimitLocked(PatternSubexpressionLimitEvent{
		DeploymentID:  deploymentID,
		StatementName: statementName,
		Edge:          violation.edge,
		Maximum:       violation.maximum,
		Attempted:     violation.attempted,
		ContextName:   contextName,
		ContextPhase:  phase,
		PartitionKey:  r.partitionKey,
		PartitionID:   partitionID,
	})
	return false
}

func admitContextPatternMatch(runtime *statementRuntime, phase string, active []patternMatch, candidate patternMatch, definition *patternDefinition) bool {
	if runtime == nil {
		return patternMatchWithinLimits(active, candidate, definition)
	}
	return runtime.admitPatternMatchForContext(active, candidate, definition, phase)
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
		if source.matched {
			transition.matched = true
		}
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
		if !patternEventSourceMatches(next.node, trigger) {
			return []patternTransition{patternTransitionFor(next)}
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
			matched:  true,
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
		if next.schedulePeriod != nil {
			if !trigger.isTimer {
				return []patternTransition{{state: next, complete: patternSatisfied(next)}}
			}
			if !patternTimerProgressDue(next, trigger.now) {
				return []patternTransition{{state: next}}
			}
			next.started = true
			advancePatternTimerScheduleRuntime(next.schedulePeriod)
			// A timer observer occurrence completes its enclosing branch. An
			// outer Every can arm a fresh observer for the next event; keeping
			// this branch non-terminal would make Sequence/And wait forever
			// because their satisfaction checks use the child state.
			next.done = true
			return []patternTransition{{state: next, complete: true}}
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
				leftSatisfied := patternSatisfied(leftTransition.state)
				if progress.node.left != nil && progress.node.left.kind == patternEveryNode {
					// A repeating leg reports done=true from its previous firing;
					// only a fresh completion on this event may advance the
					// followed-by. Treating the retained flag as satisfied would
					// re-advance with stale tags and drop the listening branch.
					leftSatisfied = leftTransition.complete
				}
				if leftSatisfied {
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
				if leftTransition.complete && progress.node.left != nil && progress.node.left.kind == patternEveryNode {
					// An every/every-distinct left leg keeps spawning followed-by
					// branches: Esper's followed-by state holds one waiting branch
					// per left firing while the every node itself stays armed.
					// Keep a phase-0 continuation alongside the advanced branch so
					// later distinct keys start further sequences.
					continuation := clonePatternProgress(progress)
					continuation.left = leftTransition.state
					continuation.tags = clonePatternTags(progress.tags)
					continuation.tagValues = clonePatternTagValues(progress.tagValues)
					continuation.started = true
					result = append(result, patternTransitionFrom(continuation, false, leftTransition))
				}
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
			rightSatisfied := patternSatisfied(rightTransition.state)
			if progress.node.right != nil && progress.node.right.kind == patternEveryNode {
				// Same retained-done guard as the phase-0 path above: a
				// repeating right leg completes the followed-by only on a
				// fresh firing, not on its previously reported done flag.
				rightSatisfied = rightTransition.complete
			}
			if rightSatisfied {
				next.phase = 2
				next.done = true
			}
			result = append(result, patternTransitionFrom(next, patternSatisfied(next), rightTransition))
			if rightTransition.complete && progress.node.right != nil && progress.node.right.kind == patternEveryNode {
				// A repeating right leg (every/every-distinct) keeps the
				// followed-by branch resident after each match: Esper pairs the
				// captured left tags with every later distinct right firing.
				// Reset the fired flag on the retained every node so the
				// continuation waits for the next completion instead of
				// re-reporting the consumed one.
				continuation := clonePatternProgress(progress)
				continuation.right = clonePatternProgress(rightTransition.state)
				continuation.right.done = false
				continuation.right.started = true
				continuation.tags = clonePatternTags(progress.tags)
				continuation.tagValues = clonePatternTagValues(progress.tagValues)
				continuation.started = true
				result = append(result, patternTransitionFrom(continuation, false, rightTransition))
			}
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
		if progress.quit {
			// A permanently completed or-expression is inert: Esper quits all
			// child listeners when one branch completes with isQuitted=true, so
			// the surviving siblings (including an every-branch) can no longer
			// match. The quit state may still sit inside a kept parent match
			// (and/sequence), where it must report its captured truth without
			// producing new completions.
			return []patternTransition{{state: clonePatternProgress(progress)}}
		}
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
				if next.done && patternCompletionPermanent(next) {
					next.quit = true
				}
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
				if next.done && patternCompletionPermanent(next) {
					next.quit = true
				}
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
				if next.done && patternCompletionPermanent(next) {
					next.quit = true
				}
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
							inheritPatternProgressTags(next, next.child)
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
					inheritPatternProgressTags(next, next.child)
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
					inheritPatternProgressTags(next, next.child)
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
				// Every restarts its child after a failed/terminated attempt. This
				// matters for timer-and-not branches: a forbidden event cancels
				// only the current attempt, and the next timer is armed from the
				// cancellation time rather than ending the enclosing repetition.
				next.child = newPatternProgress(next.node.child)
				armPatternProgressTimers(next.child, trigger.now, variables)
				next.started = patternProgressActive(next.child)
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

func patternEventSourceMatches(node *patternNode, trigger patternTrigger) bool {
	if node == nil || node.source == nil || trigger.isTimer {
		return true
	}
	return streamNodeOutputAcceptsEvent(trigger.env, node.source, trigger.event)
}

// streamNodeOutputAcceptsEvent checks the type emitted by a pattern input.
// sourceNodeAcceptsEvent answers the different question of whether an input
// event can enter a source graph; for contained/filter/window chains the
// pattern receives the transformed child/output event instead.
func streamNodeOutputAcceptsEvent(env *Environment, node *streamNode, event Event) bool {
	if node == nil || !event.Schema().valid() {
		return false
	}
	switch node.kind {
	case streamFilter, streamWindow:
		return streamNodeOutputAcceptsEvent(env, node.input, event)
	case streamContained:
		if env == nil {
			return false
		}
		schema, err := env.sourceSchema(node)
		if err != nil {
			return false
		}
		return env.acceptsEventType(schema.Name(), event.TypeName())
	case streamDerived:
		if env == nil || node.derived == nil || !node.derived.schema.valid() {
			return false
		}
		return env.acceptsEventType(node.derived.schema.Name(), event.TypeName())
	case streamSource, streamNamedWindow, streamTable, streamHistorical, streamMethod:
		return sourceNodeAcceptsEvent(env, node, event)
	case streamPattern:
		if env == nil || node.pattern == nil {
			return false
		}
		schema, err := patternJoinSchema(node.pattern)
		if err != nil {
			return false
		}
		return env.acceptsEventType(schema.Name(), event.TypeName())
	default:
		return false
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
		prearmedActive := false
		for _, match := range matches {
			if match.prearmed {
				prearmedActive = true
				break
			}
		}
		startAllowed := definition.every || len(matches) == 0
		consumptionLevel := -1
		if hasConsumption {
			probe := patternTrigger{event: event, now: now, env: r.query.env, consumptionLevel: -1}
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
		trigger := patternTrigger{event: event, now: now, env: r.query.env, consumptionLevel: consumptionLevel}
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
					prearmed:  match.prearmed && !transition.matched,
				}
				if transition.complete {
					completed = true
					if row, visible := evaluatePatternMatch(definition, candidate, plan, now, r.variables); visible && r.patternState.acceptPatternMatch(plan.query, candidate) {
						batch.New = append(batch.New, resultRow(row))
					}
					if patternCanContinueAfterMatch(transition.state) && r.admitPatternMatch(nextActive, candidate, definition) {
						nextActive = append(nextActive, candidate)
					}
					if patternWithinTerminal(transition.state) || patternCompletionPermanent(transition.state) {
						terminal = true
					}
					continue
				}
				if patternProgressTerminal(transition.state) {
					terminal = true
				}
				if patternProgressActive(transition.state) && r.admitPatternMatch(nextActive, candidate, definition) {
					nextActive = append(nextActive, candidate)
				}
			}
		}

		if startAllowed && !prearmedActive && !(definition.every && completed) && !(plan.query.discardPartialsOnMatch && completed) {
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
					if patternCanContinueAfterMatch(transition.state) && r.admitPatternMatch(nextActive, started, definition) {
						nextActive = append(nextActive, started)
					}
					if patternWithinTerminal(transition.state) || patternCompletionPermanent(transition.state) {
						terminal = true
					}
				} else if r.admitPatternMatch(nextActive, started, definition) {
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
		if !deferOutputResultWindow(plan.query.output) {
			batch.New = applyResultWindow(batch.New, plan.query)
			batch.Old = applyResultWindow(batch.Old, plan.query)
		}
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
		if !rearmPatternTimerIntervalIfVariablesChanged(r.patternState, root, r.variables) {
			break
		}
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
			r.patternState.timerIntervalFired = true
			recordPatternTimerIntervalSchedule(r.patternState, dueAt, r.variables)
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
		if r.patternState.schedulePeriod != nil {
			const maxScheduleCatchUp = 100000
			for emitted := 0; emitted < maxScheduleCatchUp && r.patternState.schedulePeriod.active && !r.patternState.schedulePeriod.next.After(now); emitted++ {
				dueAt := r.patternState.schedulePeriod.next
				match := patternMatch{current: Event{}, startedAt: dueAt}
				if patternGuardAllows(plan.query.pattern, Event{}, dueAt, r.variables) {
					if row, visible := evaluatePatternMatch(plan.query.pattern, match, plan, dueAt, r.variables); visible {
						batch.New = append(batch.New, resultRow(row))
					}
				}
				advancePatternTimerScheduleRuntime(r.patternState.schedulePeriod)
			}
		} else {
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
		if !deferOutputResultWindow(plan.query.output) {
			batch.New = applyResultWindow(batch.New, plan.query)
		}
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
			r.patternState.active = []patternMatch{{state: progress, startedAt: now, prearmed: true}}
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
				prearmed:  match.prearmed,
			}
			if transition.complete {
				completed = true
				if row, visible := evaluatePatternMatch(definition, candidate, plan, now, r.variables); visible && r.patternState.acceptPatternMatch(plan.query, candidate) {
					batch.New = append(batch.New, resultRow(row))
				}
				if patternCanContinueAfterMatch(transition.state) && r.admitPatternMatch(nextActive, candidate, definition) {
					nextActive = append(nextActive, candidate)
				}
				if patternWithinTerminal(transition.state) {
					terminal = true
				}
				if definition.every {
					progress := newPatternProgress(definition.root)
					armPatternProgressTimers(progress, now, r.variables)
					candidate := patternMatch{state: progress, startedAt: now, prearmed: patternCanStartWithoutEvent(definition.root)}
					if patternProgressActive(progress) && r.admitPatternMatch(nextActive, candidate, definition) {
						nextActive = append(nextActive, candidate)
					}
				}
				continue
			}
			if patternWithinTerminal(transition.state) {
				terminal = true
			}
			if patternProgressActive(transition.state) && r.admitPatternMatch(nextActive, candidate, definition) {
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
		if !deferOutputResultWindow(plan.query.output) {
			batch.New = applyResultWindow(batch.New, plan.query)
			batch.Old = applyResultWindow(batch.Old, plan.query)
		}
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
		prearmed:  match.prearmed,
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
	if definition.where != nil {
		delta.newEvents = filterAggregateEvents(delta.newEvents, definition.where, now, r.variables, r.engine)
		delta.oldEvents = filterAggregateEvents(delta.oldEvents, definition.where, now, r.variables, r.engine)
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
				groupingSet:       append([]int(nil), groupingSet...),
				representative:    event,
				pluginStates:      make(map[*exprNode]aggregatePluginState),
				multiPluginStates: make(map[string]aggregateMultiPluginState),
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
		emittedBefore := group.emitted
		if group.emitted && (plan.query.selector == SelectRStream || plan.query.selector == SelectIRStream) {
			oldEntries = append(oldEntries, aggregateResultEntry{
				result: resultRow(newRow(plan.resultSchema, group.previous)),
				group:  group,
				key:    key,
			})
		}
		if aggregateDefinitionIsRowForEvent(definition) && len(delta.newEvents) > 0 && len(group.events) > 0 {
			for _, current := range delta.newEvents {
				values, visible := evaluateAggregateGroup(definition, group.events, group.everEvents, group.leavingEvents, group.leaving, group.groupingSet, current, state.allEvents, state.allEverEvents, now, r.variables, group.pluginStates, group.multiPluginStates)
				if visible && (plan.query.selector == SelectIStream || plan.query.selector == SelectIRStream) {
					newEntries = append(newEntries, aggregateResultEntry{
						result: resultRow(newRow(plan.resultSchema, values)),
						group:  group,
						key:    key,
					})
				}
			}
			previous, visible := evaluateAggregateGroup(definition, group.events, group.everEvents, group.leavingEvents, group.leaving, group.groupingSet, group.current, state.allEvents, state.allEverEvents, now, r.variables, group.pluginStates, group.multiPluginStates)
			if visible {
				group.previous = append([]Value(nil), previous...)
				group.emitted = true
			} else {
				group.previous = nil
				group.emitted = false
			}
			if len(group.events) == 0 && !aggregateDefinitionUsesEver(definition) {
				for _, pluginState := range group.multiPluginStates {
					if pluginState != nil {
						pluginState.clear()
					}
				}
				delete(state.groups, key)
			}
			continue
		}
		newValues, visible := evaluateAggregateGroup(definition, group.events, group.everEvents, group.leavingEvents, group.leaving, group.groupingSet, group.current, state.allEvents, state.allEverEvents, now, r.variables, group.pluginStates, group.multiPluginStates)
		if visible && (plan.query.selector == SelectIStream || plan.query.selector == SelectIRStream) {
			newEntries = append(newEntries, aggregateResultEntry{
				result: resultRow(newRow(plan.resultSchema, newValues)),
				group:  group,
				key:    key,
			})
		}
		if visible && !emittedBefore && len(definition.groupBy) > 0 && plan.query.selector == SelectIRStream {
			// Esper group-by irstream semantics: creating a group pairs the
			// first new row with a null-prior old row whose group-by columns
			// are populated and whose aggregate columns are null.
			nullPrior := make([]Value, len(newValues))
			for index, selection := range definition.selections {
				if isAggregateExpression(selection.Expr) {
					nullPrior[index] = Null()
				} else {
					nullPrior[index] = newValues[index]
				}
			}
			oldEntries = append(oldEntries, aggregateResultEntry{
				result: resultRow(newRow(plan.resultSchema, nullPrior)),
				group:  group,
				key:    key,
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
			for _, pluginState := range group.multiPluginStates {
				if pluginState != nil {
					pluginState.clear()
				}
			}
			delete(state.groups, key)
		}
	}
	if len(plan.query.orderBy) > 0 {
		orderAggregateResults(newEntries, plan.query.orderBy, definition, state.allEvents, state.allEverEvents, now, r.variables, false)
		orderAggregateResults(oldEntries, plan.query.orderBy, definition, state.allEvents, state.allEverEvents, now, r.variables, true)
	}
	for _, entry := range newEntries {
		batch.New = append(batch.New, entry.result)
		batch.outputKeysNew = append(batch.outputKeysNew, entry.key)
	}
	for _, entry := range oldEntries {
		batch.Old = append(batch.Old, entry.result)
		batch.outputKeysOld = append(batch.outputKeysOld, entry.key)
	}
	if !batch.empty() {
		if plan.query.distinct {
			batch.New, batch.Old = r.applyDistinct(plan.query, batch.New, batch.Old)
		}
		if !deferOutputResultWindow(plan.query.output) {
			batch.New = applyResultWindow(batch.New, plan.query)
			batch.Old = applyResultWindow(batch.Old, plan.query)
		}
		batch.Sequence = r.seq.Add(1)
	}
	if plan.query.tableTarget != "" {
		if err := r.persistAggregateTable(plan, now); err != nil {
			return ResultBatch{}, err
		}
	}
	return batch, nil
}

func (r *statementRuntime) aggregateBatchSafely(delta eventDelta, plan Plan, now time.Time) (batch ResultBatch, err error) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}
		failure, ok := recovered.(aggregatePluginRuntimePanic)
		if !ok {
			panic(recovered)
		}
		plugin := failure.plugin
		if strings.TrimSpace(plugin) == "" {
			plugin = "<unnamed>"
		}
		batch = ResultBatch{}
		err = NewError(ErrorState, fmt.Sprintf("aggregate plugin %q for statement %q: %v", plugin, plan.query.name, failure.cause))
	}()
	return r.aggregateBatch(delta, plan, now)
}

// filterAggregateEvents applies a pre-aggregate WHERE to the incoming delta.
// For join aggregates the wrapper Event keeps the complete source tuple
// available to JoinField, including when the tuple contains an outer-join null
// side; ordinary aggregates evaluate the same predicate against Event.
func filterAggregateEvents(events []Event, predicate Expr, now time.Time, variables map[string]Value, engine *Engine) []Event {
	if predicate == nil || len(events) == 0 {
		return events
	}
	filtered := make([]Event, 0, len(events))
	for _, event := range events {
		tuple := joinTupleEvents(event)
		value := predicate.eval(EvalContext{
			Event:      event,
			JoinEvents: tuple,
			OuterEvent: event,
			Engine:     engine,
			Now:        now,
			Variables:  variables,
		})
		matched, ok := boolValue(value)
		if ok && matched {
			filtered = append(filtered, event)
		}
	}
	return filtered
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
		values, visible := evaluateAggregateGroup(definition, group.events, group.everEvents, group.leavingEvents, group.leaving, group.groupingSet, group.current, r.aggregateState.allEvents, r.aggregateState.allEverEvents, now, r.variables, group.pluginStates, group.multiPluginStates)
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
	if len(definition.groupBy) == 0 && len(rows) == 0 {
		values, visible := evaluateEmptyAggregateGroup(definition, now, r.variables)
		if visible {
			row := make(map[string]any, len(definition.selections))
			for index, selection := range definition.selections {
				if index < len(values) {
					row[selection.Name] = values[index].Any()
				}
			}
			rows = append(rows, row)
		}
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
	scope := ""
	if r.partitionContextName != "" && r.partitionKey != "" {
		scope = tableContextScope(r.partitionContextName, r.partitionKey)
	}
	if err := table.replaceInScope(r.context(), scope, rows); err != nil {
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
		values = append(values, expression.eval(EvalContext{Event: event, JoinEvents: joinTupleEvents(event), Now: now, Variables: variables}).Any())
	}
	return encodeKey(values)
}

func evaluateAggregateGroup(definition *aggregateDefinition, events []Event, everEvents []Event, leavingEvents []Event, leaving bool, groupingSet []int, current Event, allEvents []Event, allEverEvents []Event, now time.Time, variables map[string]Value, pluginStates map[*exprNode]aggregatePluginState, multiPluginStates map[string]aggregateMultiPluginState) ([]Value, bool) {
	return evaluateAggregateGroupInternal(definition, events, everEvents, leavingEvents, leaving, groupingSet, current, allEvents, allEverEvents, now, variables, pluginStates, multiPluginStates, false)
}

func evaluateEmptyAggregateGroup(definition *aggregateDefinition, now time.Time, variables map[string]Value) ([]Value, bool) {
	return evaluateAggregateGroupInternal(definition, nil, nil, nil, false, nil, Event{}, nil, nil, now, variables, nil, nil, true)
}

func evaluateAggregateGroupInternal(definition *aggregateDefinition, events []Event, everEvents []Event, leavingEvents []Event, leaving bool, groupingSet []int, current Event, allEvents []Event, allEverEvents []Event, now time.Time, variables map[string]Value, pluginStates map[*exprNode]aggregatePluginState, multiPluginStates map[string]aggregateMultiPluginState, allowEmpty bool) ([]Value, bool) {
	if len(events) == 0 && len(everEvents) == 0 && current.Schema().Name() == "" && !allowEmpty {
		return nil, false
	}
	ctx := aggregateGroupContext(definition, events, everEvents, leavingEvents, leaving, groupingSet, current, allEvents, allEverEvents, now, variables, pluginStates, multiPluginStates)
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

func aggregateGroupContext(definition *aggregateDefinition, events []Event, everEvents []Event, leavingEvents []Event, leaving bool, groupingSet []int, current Event, allEvents []Event, allEverEvents []Event, now time.Time, variables map[string]Value, pluginStates map[*exprNode]aggregatePluginState, multiPluginStates map[string]aggregateMultiPluginState) EvalContext {
	if current.Schema().Name() == "" {
		if len(events) > 0 {
			current = events[0]
		} else if len(everEvents) > 0 {
			current = everEvents[0]
		}
	}
	ctx := EvalContext{Event: current, JoinEvents: joinTupleEvents(current), Group: append([]Event(nil), events...), EverGroup: append([]Event(nil), everEvents...), AllGroup: append([]Event(nil), allEvents...), AllEverGroup: append([]Event(nil), allEverEvents...), LeavingEvents: append([]Event(nil), leavingEvents...), IsLeaving: leaving, Engine: aggregateEngineFromVariables(variables), Now: now, Variables: variables, aggregatePluginStates: pluginStates, aggregateMultiPluginStates: multiPluginStates, aggregateEvaluation: true}
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
				ctx.groupingValues[key] = expression.eval(EvalContext{Event: groupingEvent, JoinEvents: joinTupleEvents(groupingEvent), Now: now, Variables: variables})
			} else {
				ctx.groupingValues[key] = Null()
			}
		}
	}
	return ctx
}

func aggregateEngineFromVariables(variables map[string]Value) *Engine {
	if variables == nil {
		return nil
	}
	value, ok := variables[subqueryEngineVariable]
	if !ok || !value.IsPresent() {
		return nil
	}
	reference, ok := value.Any().(*subqueryEngineRef)
	if !ok {
		return nil
	}
	return reference.engine
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
	ctx := aggregateGroupContext(definition, events, group.everEvents, group.leavingEvents, group.leaving, group.groupingSet, group.current, allEvents, allEverEvents, now, variables, group.pluginStates, group.multiPluginStates)
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

// aggregateDefinitionIsRowForEvent identifies Esper's ungrouped
// "row-for-event" result shape: at least one projection reads the current
// event while another expression evaluates over the complete aggregate group.
// Grouped and dimensional aggregates always produce one row per group, even
// when a projection happens to read a representative event.
func aggregateDefinitionIsRowForEvent(definition *aggregateDefinition) bool {
	if definition == nil || definition.join != nil || len(definition.groupBy) != 0 || definition.grouping != aggregateGroupingPlain {
		return false
	}
	hasAggregate := false
	readsCurrentEvent := false
	for _, selection := range definition.selections {
		hasAggregate = hasAggregate || isAggregateExpression(selection.Expr)
		readsCurrentEvent = readsCurrentEvent || expressionTreeReadsCurrentEvent(selection.Expr)
	}
	return hasAggregate && readsCurrentEvent
}

// expressionTreeReadsCurrentEvent identifies scalar projections that depend
// on the ordinary input event.  Context metadata and lifecycle/pattern values
// are represented as scalar expressions too, but they describe the context
// partition rather than the event whose arrival caused the aggregate update;
// they must therefore keep the normal one-row-per-group result shape.
func expressionTreeReadsCurrentEvent(expression Expr) bool {
	if expression == nil || expression.node() == nil {
		return false
	}
	var visit func(*exprNode) bool
	visit = func(node *exprNode) bool {
		if node == nil {
			return false
		}
		if strings.HasPrefix(node.kind, "tag-") {
			return false
		}
		if expressionNodeIsAggregate(node) {
			return false
		}
		if node.kind == "field" {
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
		newResults = projectResults(delta.newEvents, plan.query, plan.resultSchema, now, r.variables, delta.history, delta.historyByEvent, delta.previousByEvent, delta.priorByEvent, false, r.evaluationContext())
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
		oldResults = projectResults(delta.oldEvents, plan.query, plan.resultSchema, now, r.variables, delta.history, oldHistoryByEvent, oldPreviousByEvent, delta.priorByEvent, true, r.evaluationContext())
	}
	if plan.query.distinct {
		newResults, oldResults = r.applyDistinct(plan.query, newResults, oldResults)
	}
	if plan.query.selector == SelectIStream || plan.query.selector == SelectIRStream {
		batch.New = newResults
		if !deferOutputResultWindow(plan.query.output) {
			batch.New = applyResultWindow(batch.New, plan.query)
		}
	}
	if plan.query.selector == SelectRStream || plan.query.selector == SelectIRStream {
		batch.Old = oldResults
		if !deferOutputResultWindow(plan.query.output) {
			batch.Old = applyResultWindow(batch.Old, plan.query)
		}
	}
	if !batch.empty() {
		batch.Sequence = r.seq.Add(1)
	}
	return batch
}

func (r *statementRuntime) joinBatch(delta joinDelta, plan Plan, now time.Time) ResultBatch {
	batch := ResultBatch{Time: now}
	newTuples := append([][]Event(nil), delta.newTuples...)
	if len(newTuples) == 0 {
		for _, pair := range delta.newPairs {
			newTuples = append(newTuples, []Event{pair.left, pair.right})
		}
	}
	oldTuples := append([][]Event(nil), delta.oldTuples...)
	if len(oldTuples) == 0 {
		for _, pair := range delta.oldPairs {
			oldTuples = append(oldTuples, []Event{pair.left, pair.right})
		}
	}
	if plan.query.selector == SelectIStream || plan.query.selector == SelectIRStream {
		filtered := filterJoinTuples(newTuples, plan.query, now, r.variables)
		batch.New = orderJoinResults(
			projectJoinTuples(filtered, plan.query, plan.resultSchema, now, r.variables, false, r.evaluationContext()),
			filtered, plan.query.orderBy, now, r.variables, false,
		)
	}
	if plan.query.selector == SelectRStream || plan.query.selector == SelectIRStream {
		filtered := filterJoinTuples(oldTuples, plan.query, now, r.variables)
		batch.Old = orderJoinResults(
			projectJoinTuples(filtered, plan.query, plan.resultSchema, now, r.variables, true, r.evaluationContext()),
			filtered, plan.query.orderBy, now, r.variables, true,
		)
	}
	if plan.query.distinct {
		batch.New, batch.Old = r.applyDistinct(plan.query, batch.New, batch.Old)
	}
	if !deferOutputResultWindow(plan.query.output) {
		batch.New = applyResultWindow(batch.New, plan.query)
		batch.Old = applyResultWindow(batch.Old, plan.query)
	}
	if !batch.empty() {
		batch.Sequence = r.seq.Add(1)
	}
	return batch
}

func filterJoinTuples(tuples [][]Event, query Query, now time.Time, variables map[string]Value) [][]Event {
	if query.joinWhere == nil || len(tuples) == 0 {
		return tuples
	}
	filtered := make([][]Event, 0, len(tuples))
	for _, tuple := range tuples {
		var event Event
		if len(tuple) > 0 {
			event = tuple[0]
		}
		value := query.joinWhere.eval(EvalContext{
			Event:      event,
			JoinEvents: tuple,
			OuterEvent: event,
			Now:        now,
			Variables:  variables,
		})
		matched, ok := boolValue(value)
		if ok && matched {
			filtered = append(filtered, tuple)
		}
	}
	return filtered
}

func projectResults(events []Event, query Query, resultSchema Schema, now time.Time, variables map[string]Value, history []Event, historyByEvent, previousByEvent, priorByEvent map[string][]Event, leaving bool, evaluation ExpressionEvaluationContext) []Result {
	if len(events) == 0 {
		return nil
	}
	events = orderEvents(events, query.orderBy, now, variables, history, historyByEvent, previousByEvent, priorByEvent, leaving, evaluation)
	results := make([]Result, 0, len(events))
	for _, event := range events {
		if len(query.selections) == 0 {
			results = append(results, resultEvent(event))
			continue
		}
		values := make([]Value, 0, len(query.selections))
		for _, selection := range query.selections {
			values = append(values, selection.Expr.eval(projectionEvalContext(event, now, variables, history, historyByEvent, previousByEvent, priorByEvent, leaving, evaluation)))
		}
		row := newRow(resultSchema, values)
		results = append(results, resultRow(row))
	}
	return results
}

func orderEvents(events []Event, keys []SortKey, now time.Time, variables map[string]Value, history []Event, historyByEvent, previousByEvent, priorByEvent map[string][]Event, leaving bool, evaluation ExpressionEvaluationContext) []Event {
	if len(keys) == 0 || len(events) < 2 {
		return events
	}
	ordered := append([]Event(nil), events...)
	sort.SliceStable(ordered, func(left, right int) bool {
		for _, key := range keys {
			comparison, ok := compareValues(key.Expr.eval(projectionEvalContext(ordered[left], now, variables, history, historyByEvent, previousByEvent, priorByEvent, leaving, evaluation)), key.Expr.eval(projectionEvalContext(ordered[right], now, variables, history, historyByEvent, previousByEvent, priorByEvent, leaving, evaluation)))
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

func projectionEvalContext(event Event, now time.Time, variables map[string]Value, history []Event, historyByEvent, previousByEvent, priorByEvent map[string][]Event, leaving bool, evaluation ExpressionEvaluationContext) EvalContext {
	eventHistory := history
	identity := eventIdentity(event)
	if historyByEvent != nil {
		eventHistory = historyByEvent[identity]
	}
	ctx := EvalContext{Event: event, History: append([]Event(nil), eventHistory...), IsLeaving: leaving, Now: now, Variables: variables, Metadata: evaluation, evaluationContextSet: true}
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
	// New results fire on the first occurrence of each distinct key within the
	// current update (a window event batch). For a continuous view such as
	// keepall each update carries a single event so every event re-emits; for a
	// batching view (length_batch) the whole batch is one update and the key is
	// emitted once. The global reference count still tracks every sharing event
	// so the old stream fires only when the last event with a key leaves.
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
	seen := make(map[string]struct{}, len(newResults))
	for _, result := range newResults {
		key := resultKey(result)
		r.distinctCounts[key]++
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			newOutput = append(newOutput, result)
		}
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

// deferOutputResultWindow keeps row-limit candidates intact while a batched
// output policy is collecting deltas. Esper applies order/limit/offset to the
// complete output interval; applying it to each input event would discard
// candidates before the interval can be sorted. Snapshot policies already
// rebuild and limit the current state directly.
func deferOutputResultWindow(policy OutputPolicy) bool {
	if policy.Snapshot {
		return false
	}
	switch policy.Kind {
	case OutputEveryPolicy, OutputEveryTimePolicy, OutputFirstEveryEventsPolicy, OutputFirstEveryTimePolicy, OutputLastEveryEventsPolicy, OutputLastEveryTimePolicy:
		return true
	default:
		return false
	}
}

func orderResults(results []Result, keys []SortKey, now time.Time, variables map[string]Value) []Result {
	if len(results) < 2 || len(keys) == 0 {
		return results
	}
	ordered := append([]Result(nil), results...)
	sort.SliceStable(ordered, func(left, right int) bool {
		leftContext := resultOrderContext(ordered[left], now, variables)
		rightContext := resultOrderContext(ordered[right], now, variables)
		for _, key := range keys {
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
	return ordered
}

func resultOrderContext(result Result, now time.Time, variables map[string]Value) EvalContext {
	ctx := EvalContext{Now: now, Variables: variables, JoinEvents: result.joinEvents}
	if event, ok := result.Event(); ok {
		ctx.Event = event
	}
	if len(result.joinEvents) > 0 {
		ctx.Event = result.joinEvents[0]
		ctx.OuterEvent = result.joinEvents[0]
	}
	if row, ok := result.Row(); ok {
		rowCopy := row
		ctx.resultRow = &rowCopy
	}
	return ctx
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

func projectJoinResults(pairs []eventPair, query Query, resultSchema Schema, now time.Time, variables map[string]Value, leaving bool, evaluation ExpressionEvaluationContext) []Result {
	tuuples := make([][]Event, 0, len(pairs))
	for _, pair := range pairs {
		tuuples = append(tuuples, []Event{pair.left, pair.right})
	}
	return projectJoinTuples(tuuples, query, resultSchema, now, variables, leaving, evaluation)
}

func projectJoinTuples(tuples [][]Event, query Query, resultSchema Schema, now time.Time, variables map[string]Value, leaving bool, evaluation ExpressionEvaluationContext) []Result {
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
			values = append(values, selection.Expr.eval(EvalContext{
				Event:                event,
				JoinEvents:           tuple,
				OuterEvent:           event,
				IsLeaving:            leaving,
				Now:                  now,
				Variables:            variables,
				Metadata:             evaluation,
				evaluationContextSet: true,
			}))
		}
		results = append(results, resultJoinRow(newRow(resultSchema, values), tuple))
	}
	return results
}

type joinResultOrderEntry struct {
	result Result
	tuple  []Event
}

func orderJoinResults(results []Result, tuples [][]Event, keys []SortKey, now time.Time, variables map[string]Value, leaving bool) []Result {
	if len(results) < 2 || len(keys) == 0 || len(results) != len(tuples) {
		return results
	}
	entries := make([]joinResultOrderEntry, len(results))
	for index := range results {
		entries[index] = joinResultOrderEntry{result: results[index], tuple: tuples[index]}
	}
	sort.SliceStable(entries, func(left, right int) bool {
		leftContext := joinResultOrderContext(entries[left].result, entries[left].tuple, now, variables, leaving)
		rightContext := joinResultOrderContext(entries[right].result, entries[right].tuple, now, variables, leaving)
		for _, key := range keys {
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
	ordered := make([]Result, len(entries))
	for index, entry := range entries {
		ordered[index] = entry.result
	}
	return ordered
}

func joinResultOrderContext(result Result, tuple []Event, now time.Time, variables map[string]Value, leaving bool) EvalContext {
	var event Event
	if len(tuple) > 0 {
		event = tuple[0]
	}
	ctx := EvalContext{
		Event:      event,
		JoinEvents: tuple,
		OuterEvent: event,
		IsLeaving:  leaving,
		Now:        now,
		Variables:  variables,
	}
	if row, ok := result.Row(); ok {
		rowCopy := row
		ctx.resultRow = &rowCopy
	}
	return ctx
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
