package esper

import "reflect"

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
		if pending.DeploymentID != event.DeploymentID || pending.StatementName != event.StatementName || pending.Edge != event.Edge || pending.Maximum != event.Maximum {
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
	e.mu.Unlock()
	for _, event := range events {
		for _, listener := range listeners {
			listener.OnPatternSubexpressionLimit(event)
		}
	}
}
