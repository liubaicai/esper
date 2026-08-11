package esper

import (
	"reflect"
	"strings"
)

// WithPatternSubexpressionMax configures the runtime-wide pattern
// subexpression pool limit, the fluent counterpart of Esper's
// ConfigurationRuntimePatterns.setMaxSubexpressions. A negative value (the
// default) leaves the pool disabled. When the pool is configured, every
// followed-by edge across all pattern statements shares one admission
// count: starting a non-first followed-by branch consumes one slot and
// releasing that branch returns it.
func WithPatternSubexpressionMax(maximum int64) EngineOption {
	return func(cfg *engineConfig) { cfg.patternSubexpressionMax = maximum }
}

// WithPatternSubexpressionPreventStart controls whether a pool overflow
// rejects the new subexpression (true, the Java default) or only reports a
// PatternRuntimeSubexpressionLimitEvent while admitting the branch (false).
// It mirrors Esper's setMaxSubexpressionPreventStart.
func WithPatternSubexpressionPreventStart(preventStart bool) EngineOption {
	return func(cfg *engineConfig) { cfg.patternSubexpressionPreventStart = preventStart }
}

// PatternRuntimeSubexpressionLimitEvent reports a runtime-wide pattern
// subexpression pool overflow. It corresponds to Esper's
// ConditionPatternRuntimeSubexpressionMax: Maximum is the configured pool
// limit and Counts holds the per-statement live subexpression counts at the
// time of the violation (the attempting statement does not include the
// rejected candidate).
type PatternRuntimeSubexpressionLimitEvent struct {
	RuntimeURI    string
	DeploymentID  string
	StatementName string
	Maximum       int64
	Counts        map[string]int64
}

// PatternRuntimeSubexpressionLimitListener observes runtime pool overflows.
// Callbacks run after the Engine lock is released and may safely call back
// into the Engine.
type PatternRuntimeSubexpressionLimitListener interface {
	OnPatternRuntimeSubexpressionLimit(PatternRuntimeSubexpressionLimitEvent)
}

// PatternRuntimeSubexpressionLimitListenerFunc adapts a function to the
// listener.
type PatternRuntimeSubexpressionLimitListenerFunc func(PatternRuntimeSubexpressionLimitEvent)

func (f PatternRuntimeSubexpressionLimitListenerFunc) OnPatternRuntimeSubexpressionLimit(event PatternRuntimeSubexpressionLimitEvent) {
	if f != nil {
		f(event)
	}
}

// AddPatternRuntimeSubexpressionLimitListener registers a pool-overflow
// listener, the Go counterpart of Esper's condition-handler registration for
// ConditionPatternRuntimeSubexpressionMax.
func (e *Engine) AddPatternRuntimeSubexpressionLimitListener(listener PatternRuntimeSubexpressionLimitListener) error {
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	if isNilPatternRuntimeLimitListener(listener) {
		return NewError(ErrorInvalidRule, "pattern runtime subexpression-limit listener is nil")
	}
	e.mu.Lock()
	e.patternRuntimeLimitListeners = append(e.patternRuntimeLimitListeners, listener)
	e.mu.Unlock()
	return nil
}

func (e *Engine) RemovePatternRuntimeSubexpressionLimitListener(listener PatternRuntimeSubexpressionLimitListener) {
	if e == nil || listener == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for index, candidate := range e.patternRuntimeLimitListeners {
		if samePatternRuntimeLimitListener(candidate, listener) {
			e.patternRuntimeLimitListeners = append(e.patternRuntimeLimitListeners[:index], e.patternRuntimeLimitListeners[index+1:]...)
			break
		}
	}
}

func (e *Engine) PatternRuntimeSubexpressionLimitListeners() []PatternRuntimeSubexpressionLimitListener {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]PatternRuntimeSubexpressionLimitListener(nil), e.patternRuntimeLimitListeners...)
}

func (e *Engine) RemovePatternRuntimeSubexpressionLimitListeners() {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.patternRuntimeLimitListeners = nil
	e.mu.Unlock()
}

func samePatternRuntimeLimitListener(left, right PatternRuntimeSubexpressionLimitListener) bool {
	if isNilPatternRuntimeLimitListener(left) || isNilPatternRuntimeLimitListener(right) || reflect.TypeOf(left) != reflect.TypeOf(right) {
		return false
	}
	typ := reflect.TypeOf(left)
	if typ.Comparable() {
		return left == right
	}
	leftValue := reflect.ValueOf(left)
	rightValue := reflect.ValueOf(right)
	return leftValue.Kind() == reflect.Pointer && rightValue.Kind() == reflect.Pointer && leftValue.Pointer() == rightValue.Pointer()
}

func isNilPatternRuntimeLimitListener(listener PatternRuntimeSubexpressionLimitListener) bool {
	if listener == nil {
		return true
	}
	value := reflect.ValueOf(listener)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (e *Engine) queuePatternRuntimeLimitLocked(event PatternRuntimeSubexpressionLimitEvent) {
	if e == nil {
		return
	}
	e.pendingPatternRuntimeLimits = append(e.pendingPatternRuntimeLimits, event)
}

// countPatternPoolSubexpressions counts the followed-by branches Java would
// charge to the runtime-wide subexpression pool: every sequence progress
// that has started its right side (phase >= 1) holds one pool slot,
// matching EvalFollowedByWithMaxStateNodeManaged tracking active non-first
// children.
func countPatternPoolSubexpressions(progress *patternProgress) int64 {
	if progress == nil {
		return 0
	}
	total := countPatternPoolSubexpressions(progress.left) +
		countPatternPoolSubexpressions(progress.right) +
		countPatternPoolSubexpressions(progress.child)
	if progress.node != nil && progress.node.kind == patternSequenceNode && progress.phase >= 1 {
		total++
	}
	return total
}

// patternPoolTracker carries the runtime-wide subexpression pool admission
// state for one in-flight event (or timer occurrence) within one pattern
// runtime. Java increments the pool the moment a followed-by starts a
// non-first child and decrements it the moment that child quits; the Go
// engine instead rebuilds the active match set functionally per trigger, so
// the tracker replays the same bookkeeping as a delta against the committed
// pre-event counts: start records the source match's charged count as a
// replacement credit, admit charges only the candidate's net new count, and
// settle releases the credit of any source match that produced no admitted
// successor (completion, guard quit, within expiry).
type patternPoolTracker struct {
	engine   *Engine
	runtime  *statementRuntime
	state    *patternRuntimeState
	others   map[string]int64
	otherSum int64
	base     int64
	computed bool
	pending  int64
	credit   int64
}

// newPatternPoolTracker returns nil when the runtime-wide pool is disabled,
// keeping pattern admission allocation-free in the default configuration.
func newPatternPoolTracker(runtime *statementRuntime, state *patternRuntimeState) *patternPoolTracker {
	if runtime == nil || runtime.engine == nil || state == nil || runtime.engine.patternSubexpressionMax < 0 {
		return nil
	}
	return &patternPoolTracker{engine: runtime.engine, runtime: runtime, state: state}
}

// start marks the pool charge of the source match whose transitions are
// about to be processed.
func (t *patternPoolTracker) start(match patternMatch) {
	if t == nil {
		return
	}
	t.credit = countPatternPoolSubexpressions(match.state)
}

// settle folds the source match's fate into the pending delta: a match with
// no admitted successor releases its charge, exactly like Java decrementing
// the pool when the last active child of a followed-by quits.
func (t *patternPoolTracker) settle() {
	if t == nil {
		return
	}
	t.pending -= t.credit
	t.credit = 0
}

func (t *patternPoolTracker) compute() {
	if t.computed {
		return
	}
	t.computed = true
	for _, match := range t.state.active {
		t.base += countPatternPoolSubexpressions(match.state)
	}
	t.others, t.otherSum = t.engine.patternPoolCountsExcludingLocked(t.state)
}

// admit applies PatternSubexpressionPoolRuntimeSvcImpl.tryIncreaseCount
// semantics to one candidate branch: only the candidate's net new charge
// (beyond the source match it replaces) is tested against the pool limit.
// Overflow queues a PatternRuntimeSubexpressionLimitEvent carrying the
// per-statement counts without the rejected candidate; the candidate is
// rejected only when preventStart is set.
func (t *patternPoolTracker) admit(candidate patternMatch) bool {
	if t == nil {
		return true
	}
	fresh := countPatternPoolSubexpressions(candidate.state) - t.credit
	if fresh > 0 {
		t.compute()
		if t.otherSum+t.base+t.pending+fresh > t.engine.patternSubexpressionMax {
			counts := make(map[string]int64, len(t.others)+1)
			for name, count := range t.others {
				counts[name] = count
			}
			_, statementName, _ := strings.Cut(t.runtime.rowRecogOwner, ":")
			counts[statementName] += t.base + t.pending
			deploymentID, _, _ := strings.Cut(t.runtime.rowRecogOwner, ":")
			t.engine.queuePatternRuntimeLimitLocked(PatternRuntimeSubexpressionLimitEvent{
				RuntimeURI:    t.engine.runtimeURI,
				DeploymentID:  deploymentID,
				StatementName: statementName,
				Maximum:       t.engine.patternSubexpressionMax,
				Counts:        counts,
			})
			if t.engine.patternSubexpressionPreventStart {
				return false
			}
		}
		t.pending += fresh
	}
	// The first admitted successor replaces the source match's charge.
	t.credit = 0
	return true
}

// patternPoolCountsExcludingLocked scans the committed pattern state of
// every deployed pattern statement except the excluded runtime state (the
// admitting statement's in-flight set is accounted by the tracker instead).
// Context partitions roll up under their statement name, matching Java's
// per-statement PatternSubexpressionPoolStmtHandler aggregation.
func (e *Engine) patternPoolCountsExcludingLocked(excluded *patternRuntimeState) (map[string]int64, int64) {
	counts := make(map[string]int64)
	var total int64
	var walk func(runtime *statementRuntime) int64
	walk = func(runtime *statementRuntime) int64 {
		if runtime == nil {
			return 0
		}
		var sum int64
		if runtime.patternState != nil && runtime.patternState != excluded {
			for _, match := range runtime.patternState.active {
				sum += countPatternPoolSubexpressions(match.state)
			}
		}
		for _, partition := range runtime.partitions {
			sum += walk(partition)
		}
		return sum
	}
	for _, statement := range e.statements {
		if statement == nil || statement.plan.query.pattern == nil {
			continue
		}
		count := walk(&statement.runtime)
		counts[statement.name] = count
		total += count
	}
	return counts, total
}

// PatternSubexpressionLimitEvent reports a FollowedByMax rejection. Edge is
// the canonical description of the bounded followed-by expression, Maximum
// is the resolved constant/variable limit, and Attempted is the number of
// waiting branches that would have existed if the candidate were admitted.
type PatternSubexpressionLimitEvent struct {
	DeploymentID  string
	StatementName string
	Edge          string
	Maximum       int
	Attempted     int
	ContextName   string
	ContextPhase  string
	PartitionKey  string
	PartitionID   int
}

// PatternSubexpressionLimitListener observes FollowedByMax rejections.
// Callbacks run after the Engine lock is released and may safely call back
// into the Engine.
type PatternSubexpressionLimitListener interface {
	OnPatternSubexpressionLimit(PatternSubexpressionLimitEvent)
}

// PatternSubexpressionLimitListenerFunc adapts a function to the listener.
type PatternSubexpressionLimitListenerFunc func(PatternSubexpressionLimitEvent)

func (f PatternSubexpressionLimitListenerFunc) OnPatternSubexpressionLimit(event PatternSubexpressionLimitEvent) {
	if f != nil {
		f(event)
	}
}

func (e *Engine) AddPatternSubexpressionLimitListener(listener PatternSubexpressionLimitListener) error {
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	if isNilPatternSubexpressionLimitListener(listener) {
		return NewError(ErrorInvalidRule, "pattern subexpression-limit listener is nil")
	}
	e.mu.Lock()
	e.patternSubexpressionLimitListeners = append(e.patternSubexpressionLimitListeners, listener)
	e.mu.Unlock()
	return nil
}

// AddPatternSubexpressionLimitHandler is the concise handler alias.
func (e *Engine) AddPatternSubexpressionLimitHandler(listener PatternSubexpressionLimitListener) error {
	return e.AddPatternSubexpressionLimitListener(listener)
}

func (e *Engine) RemovePatternSubexpressionLimitListener(listener PatternSubexpressionLimitListener) {
	if e == nil || listener == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for index, candidate := range e.patternSubexpressionLimitListeners {
		if samePatternSubexpressionLimitListener(candidate, listener) {
			e.patternSubexpressionLimitListeners = append(e.patternSubexpressionLimitListeners[:index], e.patternSubexpressionLimitListeners[index+1:]...)
			break
		}
	}
}

func (e *Engine) RemovePatternSubexpressionLimitHandler(listener PatternSubexpressionLimitListener) {
	e.RemovePatternSubexpressionLimitListener(listener)
}

func (e *Engine) PatternSubexpressionLimitListeners() []PatternSubexpressionLimitListener {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]PatternSubexpressionLimitListener(nil), e.patternSubexpressionLimitListeners...)
}

func (e *Engine) RemovePatternSubexpressionLimitListeners() {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.patternSubexpressionLimitListeners = nil
	e.mu.Unlock()
}

func samePatternSubexpressionLimitListener(left, right PatternSubexpressionLimitListener) bool {
	if isNilPatternSubexpressionLimitListener(left) || isNilPatternSubexpressionLimitListener(right) || reflect.TypeOf(left) != reflect.TypeOf(right) {
		return false
	}
	typ := reflect.TypeOf(left)
	if typ.Comparable() {
		return left == right
	}
	leftValue := reflect.ValueOf(left)
	rightValue := reflect.ValueOf(right)
	return leftValue.Kind() == reflect.Pointer && rightValue.Kind() == reflect.Pointer && leftValue.Pointer() == rightValue.Pointer()
}

func isNilPatternSubexpressionLimitListener(listener PatternSubexpressionLimitListener) bool {
	if listener == nil {
		return true
	}
	value := reflect.ValueOf(listener)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (e *Engine) queuePatternSubexpressionLimitLocked(event PatternSubexpressionLimitEvent) {
	if e == nil {
		return
	}
	for index, pending := range e.pendingPatternSubexpressionLimits {
		if pending.DeploymentID != event.DeploymentID || pending.StatementName != event.StatementName || pending.Edge != event.Edge || pending.Maximum != event.Maximum ||
			pending.ContextName != event.ContextName || pending.ContextPhase != event.ContextPhase || pending.PartitionKey != event.PartitionKey || pending.PartitionID != event.PartitionID {
			continue
		}
		if event.Attempted > pending.Attempted {
			e.pendingPatternSubexpressionLimits[index].Attempted = event.Attempted
		}
		return
	}
	e.pendingPatternSubexpressionLimits = append(e.pendingPatternSubexpressionLimits, event)
}

func (e *Engine) dispatchPatternSubexpressionLimitEvents() {
	if e == nil {
		return
	}
	e.mu.Lock()
	events := append([]PatternSubexpressionLimitEvent(nil), e.pendingPatternSubexpressionLimits...)
	e.pendingPatternSubexpressionLimits = nil
	listeners := append([]PatternSubexpressionLimitListener(nil), e.patternSubexpressionLimitListeners...)
	poolEvents := append([]PatternRuntimeSubexpressionLimitEvent(nil), e.pendingPatternRuntimeLimits...)
	e.pendingPatternRuntimeLimits = nil
	poolListeners := append([]PatternRuntimeSubexpressionLimitListener(nil), e.patternRuntimeLimitListeners...)
	e.mu.Unlock()
	for _, event := range events {
		for _, listener := range listeners {
			listener.OnPatternSubexpressionLimit(event)
		}
	}
	for _, event := range poolEvents {
		for _, listener := range poolListeners {
			listener.OnPatternRuntimeSubexpressionLimit(event)
		}
	}
}
