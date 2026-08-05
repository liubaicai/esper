package esper

import (
	"reflect"
)

// MatchRecognizeRuntimeConfig controls the runtime-wide resource policy for
// fluent MatchRecognize statements. A negative MaxStates disables the pool,
// matching Esper's null/default configuration. PreventStart defaults to true.
type MatchRecognizeRuntimeConfig struct {
	MaxStates    int64
	PreventStart bool
}

// WithMatchRecognizeRuntime applies the complete runtime-wide recognition
// policy. Use MaxStates < 0 to leave the pool disabled.
func WithMatchRecognizeRuntime(config MatchRecognizeRuntimeConfig) EngineOption {
	return func(cfg *engineConfig) {
		cfg.matchRecognize = config
	}
}

// WithMatchRecognizeMaxStates configures the runtime-wide active-state limit.
// A negative value disables the limit; zero is a valid limit that prevents
// every new recognition start when PreventStart is enabled.
func WithMatchRecognizeMaxStates(maxStates int64) EngineOption {
	return func(cfg *engineConfig) {
		cfg.matchRecognize.MaxStates = maxStates
	}
}

// WithMatchRecognizePreventStart controls whether a state-pool overflow
// rejects the new start (true) or records the overflow and keeps the state
// (false), matching ConfigurationRuntimeMatchRecognize.
func WithMatchRecognizePreventStart(preventStart bool) EngineOption {
	return func(cfg *engineConfig) {
		cfg.matchRecognize.PreventStart = preventStart
	}
}

// WithMatchRecognizeStateLimit is the concise option for the two related
// runtime settings used by Esper's row-recognition configuration tests.
func WithMatchRecognizeStateLimit(maxStates int64, preventStart bool) EngineOption {
	return func(cfg *engineConfig) {
		cfg.matchRecognize = MatchRecognizeRuntimeConfig{
			MaxStates:    maxStates,
			PreventStart: preventStart,
		}
	}
}

// MatchRecognizeStateLimitEvent is delivered when the runtime-wide
// recognition state pool reaches its configured limit. Counts is keyed by
// Statement.ID(), which combines deployment id and statement name in the
// same way as Esper's DeploymentIdNamePair.
type MatchRecognizeStateLimitEvent struct {
	MaxStates int64
	Counts    map[string]int64
}

func (e MatchRecognizeStateLimitEvent) clone() MatchRecognizeStateLimitEvent {
	e.Counts = cloneInt64Map(e.Counts)
	return e
}

// MatchRecognizeStateLimitListener observes runtime-wide row-recognition
// pool overflows. Callbacks run after the engine lock is released.
type MatchRecognizeStateLimitListener interface {
	OnMatchRecognizeStateLimit(MatchRecognizeStateLimitEvent)
}

// MatchRecognizeStateLimitListenerFunc adapts a function to the listener
// interface, keeping small Go integrations allocation-free and idiomatic.
type MatchRecognizeStateLimitListenerFunc func(MatchRecognizeStateLimitEvent)

func (f MatchRecognizeStateLimitListenerFunc) OnMatchRecognizeStateLimit(event MatchRecognizeStateLimitEvent) {
	if f != nil {
		f(event)
	}
}

func (e *Engine) AddMatchRecognizeStateLimitListener(listener MatchRecognizeStateLimitListener) error {
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	if listener == nil {
		return NewError(ErrorInvalidRule, "match-recognize state-limit listener is nil")
	}
	e.mu.Lock()
	e.matchRecognizeStateLimitListeners = append(e.matchRecognizeStateLimitListeners, listener)
	e.mu.Unlock()
	return nil
}

// AddMatchRecognizeStateLimitHandler is a concise alias for
// AddMatchRecognizeStateLimitListener.
func (e *Engine) AddMatchRecognizeStateLimitHandler(listener MatchRecognizeStateLimitListener) error {
	return e.AddMatchRecognizeStateLimitListener(listener)
}

func (e *Engine) RemoveMatchRecognizeStateLimitListener(listener MatchRecognizeStateLimitListener) {
	if e == nil || listener == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for index, candidate := range e.matchRecognizeStateLimitListeners {
		if sameMatchRecognizeStateLimitListener(candidate, listener) {
			e.matchRecognizeStateLimitListeners = append(e.matchRecognizeStateLimitListeners[:index], e.matchRecognizeStateLimitListeners[index+1:]...)
			break
		}
	}
}

// RemoveMatchRecognizeStateLimitHandler is a concise alias for
// RemoveMatchRecognizeStateLimitListener.
func (e *Engine) RemoveMatchRecognizeStateLimitHandler(listener MatchRecognizeStateLimitListener) {
	e.RemoveMatchRecognizeStateLimitListener(listener)
}

func (e *Engine) MatchRecognizeStateLimitListeners() []MatchRecognizeStateLimitListener {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]MatchRecognizeStateLimitListener(nil), e.matchRecognizeStateLimitListeners...)
}

func (e *Engine) RemoveMatchRecognizeStateLimitListeners() {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.matchRecognizeStateLimitListeners = nil
	e.mu.Unlock()
}

func sameMatchRecognizeStateLimitListener(left, right MatchRecognizeStateLimitListener) bool {
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

func (e *Engine) takeMatchRecognizeStateLimitEventsLocked() []MatchRecognizeStateLimitEvent {
	if e == nil || len(e.pendingMatchRecognizeStateLimits) == 0 {
		return nil
	}
	events := append([]MatchRecognizeStateLimitEvent(nil), e.pendingMatchRecognizeStateLimits...)
	e.pendingMatchRecognizeStateLimits = nil
	return events
}

func (e *Engine) dispatchMatchRecognizeStateLimitEvents() {
	if e == nil {
		return
	}
	e.mu.Lock()
	events := e.takeMatchRecognizeStateLimitEventsLocked()
	listeners := append([]MatchRecognizeStateLimitListener(nil), e.matchRecognizeStateLimitListeners...)
	e.mu.Unlock()
	for _, event := range events {
		for _, listener := range listeners {
			listener.OnMatchRecognizeStateLimit(event.clone())
		}
	}
}

func (e *Engine) queueMatchRecognizeStateLimitLocked(maxStates int64, counts map[string]int64) {
	if e == nil {
		return
	}
	e.pendingMatchRecognizeStateLimits = append(e.pendingMatchRecognizeStateLimits, MatchRecognizeStateLimitEvent{
		MaxStates: maxStates,
		Counts:    cloneInt64Map(counts),
	})
}

type rowRecogStatePool struct {
	maxStates    int64
	preventStart bool
	total        int64
	counts       map[string]int64
	registered   map[string]struct{}
}

func newRowRecogStatePool(config MatchRecognizeRuntimeConfig) *rowRecogStatePool {
	return &rowRecogStatePool{
		maxStates:    config.MaxStates,
		preventStart: config.PreventStart,
		counts:       make(map[string]int64),
		registered:   make(map[string]struct{}),
	}
}

func (p *rowRecogStatePool) register(owner string) {
	if p == nil || p.maxStates < 0 || owner == "" {
		return
	}
	p.registered[owner] = struct{}{}
	if _, exists := p.counts[owner]; !exists {
		p.counts[owner] = 0
	}
}

func (p *rowRecogStatePool) tryIncrease(e *Engine, owner string) bool {
	if p == nil || p.maxStates < 0 {
		return true
	}
	if p.total+1 > p.maxStates {
		p.emitLimit(e)
		if p.preventStart {
			return false
		}
	}
	p.total++
	p.counts[owner]++
	return true
}

func (p *rowRecogStatePool) decrease(owner string, amount int64) {
	if p == nil || p.maxStates < 0 || amount <= 0 {
		return
	}
	p.total -= amount
	if p.total < 0 {
		p.total = 0
	}
	if p.counts[owner] <= amount {
		p.counts[owner] = 0
		return
	}
	p.counts[owner] -= amount
}

func (p *rowRecogStatePool) removeOwner(owner string) {
	if p == nil || owner == "" {
		return
	}
	if p.maxStates >= 0 {
		p.total -= p.counts[owner]
		if p.total < 0 {
			p.total = 0
		}
	}
	delete(p.counts, owner)
	delete(p.registered, owner)
}

func (p *rowRecogStatePool) emitLimit(e *Engine) {
	if p == nil || e == nil {
		return
	}
	counts := make(map[string]int64, len(p.registered))
	for owner := range p.registered {
		counts[owner] = p.counts[owner]
	}
	e.queueMatchRecognizeStateLimitLocked(p.maxStates, counts)
}

func cloneInt64Map(source map[string]int64) map[string]int64 {
	if len(source) == 0 {
		return map[string]int64{}
	}
	result := make(map[string]int64, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

// releaseRowRecogRuntimeLocked returns all active starts owned by a runtime
// and its context partitions to the engine-wide pool. The caller holds
// Engine.mu; context partition teardown and statement undeploy both use this
// boundary.
func (e *Engine) releaseRowRecogRuntimeLocked(runtime *statementRuntime) {
	if e == nil || runtime == nil {
		return
	}
	if state := runtime.rowRecogState; state != nil {
		for _, partition := range state.partitions {
			if partition == nil {
				continue
			}
			amount := int64(0)
			if len(partition.activeStateCounts) > 0 {
				for _, count := range partition.activeStateCounts {
					amount += count
				}
			} else {
				amount = int64(len(partition.activeStarts))
			}
			if amount > 0 {
				e.matchRecognizeStatePool.decrease(runtime.rowRecogOwner, amount)
			}
			partition.activeStarts = make(map[string]struct{})
			partition.activeStateCounts = make(map[string]int64)
			partition.activePaths = make(map[string][]rowRecogNFAPath)
			partition.allowedMatchStarts = make(map[string]struct{})
		}
	}
	for _, partition := range runtime.partitions {
		e.releaseRowRecogRuntimeLocked(partition)
	}
}
