package esper

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// WithRuntimeMetrics enables periodic runtime metrics on explicit virtual
// time advances. The first AdvanceTime call establishes the reporting epoch;
// subsequent calls at or beyond the interval publish one metric and re-anchor
// the next interval at that time.
func WithRuntimeMetrics(interval time.Duration) EngineOption {
	return func(config *engineConfig) {
		if interval > 0 {
			config.runtimeMetricsInterval = interval
		}
	}
}

// RuntimeMetric is a point-in-time Engine instrumentation record.
type RuntimeMetric struct {
	RuntimeURI      string
	Timestamp       time.Time
	InputCount      uint64
	InputCountDelta uint64
	ScheduleDepth   int
}

// RuntimeMetricListener receives periodic runtime metric records. Listener
// errors are returned from the AdvanceTime call that produced the metric.
type RuntimeMetricListener func(context.Context, RuntimeMetric) error

type runtimeMetricsState struct {
	interval     time.Duration
	configured   bool
	enabled      bool
	initialized  bool
	next         time.Time
	inputCount   uint64
	lastReported uint64
	last         RuntimeMetric
	listeners    map[uint64]RuntimeMetricListener
	nextID       uint64
}

func newRuntimeMetricsState(interval time.Duration) *runtimeMetricsState {
	if interval <= 0 {
		return nil
	}
	return &runtimeMetricsState{
		interval:   interval,
		configured: true,
		enabled:    true,
		listeners:  make(map[uint64]RuntimeMetricListener),
	}
}

func (e *Engine) recordRuntimeInputLocked() {
	if e == nil || e.runtimeMetrics == nil {
		return
	}
	e.runtimeMetrics.inputCount++
}

func (e *Engine) runtimeMetricDueLocked(at time.Time) (RuntimeMetric, []RuntimeMetricListener, bool) {
	if e == nil || e.runtimeMetrics == nil {
		return RuntimeMetric{}, nil, false
	}
	state := e.runtimeMetrics
	if !state.enabled {
		return RuntimeMetric{}, nil, false
	}
	if !state.initialized {
		state.initialized = true
		state.next = at.Add(state.interval)
		return RuntimeMetric{}, nil, false
	}
	if at.Before(state.next) {
		return RuntimeMetric{}, nil, false
	}
	metric := RuntimeMetric{
		RuntimeURI:      e.runtimeURI,
		Timestamp:       at,
		InputCount:      state.inputCount,
		InputCountDelta: state.inputCount - state.lastReported,
		ScheduleDepth:   e.runtimeScheduleDepthLocked(),
	}
	state.last = metric
	state.lastReported = state.inputCount
	state.next = at.Add(state.interval)
	// Esper routes RuntimeMetric as an internal event after capturing the
	// counters, so that event becomes part of the following period's delta.
	state.inputCount++
	listeners := make([]RuntimeMetricListener, 0, len(state.listeners))
	for id := uint64(1); id <= state.nextID; id++ {
		if listener := state.listeners[id]; listener != nil {
			listeners = append(listeners, listener)
		}
	}
	return metric, listeners, true
}

func (e *Engine) runtimeScheduleDepthLocked() int {
	if e == nil {
		return 0
	}
	depth := 0
	for _, statement := range e.statements {
		statement.mu.RLock()
		active := !statement.closed && statement.state == StatementStarted
		if active {
			_, active = statementRuntimeNearestSchedule(&statement.runtime, statement.plan.query)
		}
		statement.mu.RUnlock()
		if active {
			depth++
		}
	}
	return depth
}

func dispatchRuntimeMetric(ctx context.Context, metric RuntimeMetric, listeners []RuntimeMetricListener) error {
	for _, listener := range listeners {
		if listener == nil {
			continue
		}
		if err := listener(ctx, metric); err != nil {
			return fmt.Errorf("esper: runtime metric listener: %w", err)
		}
	}
	return nil
}

// SubscribeRuntimeMetrics adds a periodic runtime metric observer.
func (e *Engine) SubscribeRuntimeMetrics(listener RuntimeMetricListener) (*RuntimeMetricSubscription, error) {
	if e == nil {
		return nil, NewError(ErrorDependency, "nil engine")
	}
	if listener == nil {
		return nil, NewError(ErrorInvalidRule, "runtime metric listener is nil")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, NewError(ErrorState, "engine is closed")
	}
	if e.runtimeMetrics == nil || !e.runtimeMetrics.configured {
		return nil, NewError(ErrorState, "runtime metrics are not configured")
	}
	e.runtimeMetrics.nextID++
	id := e.runtimeMetrics.nextID
	e.runtimeMetrics.listeners[id] = listener
	return &RuntimeMetricSubscription{engine: e, id: id}, nil
}

// CurrentRuntimeMetric returns a snapshot without advancing the reporting
// interval or incrementing the internal metric-event input count.
func (e *Engine) CurrentRuntimeMetric(ctx context.Context) (RuntimeMetric, error) {
	if err := contextErr(ctx); err != nil {
		return RuntimeMetric{}, err
	}
	if e == nil {
		return RuntimeMetric{}, NewError(ErrorDependency, "nil engine")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return RuntimeMetric{}, NewError(ErrorState, "engine is closed")
	}
	if e.runtimeMetrics == nil || !e.runtimeMetrics.configured {
		return RuntimeMetric{}, NewError(ErrorState, "runtime metrics are not configured")
	}
	state := e.runtimeMetrics
	return RuntimeMetric{
		RuntimeURI:      e.runtimeURI,
		Timestamp:       e.clock.Now(),
		InputCount:      state.inputCount,
		InputCountDelta: state.inputCount - state.lastReported,
		ScheduleDepth:   e.runtimeScheduleDepthLocked(),
	}, nil
}

// SetRuntimeMetricsEnabled enables or disables configured periodic reporting.
// Re-enabling schedules the next report relative to the current Engine time.
func (e *Engine) SetRuntimeMetricsEnabled(enabled bool) error {
	if e == nil {
		return NewError(ErrorDependency, "nil engine")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return NewError(ErrorState, "engine is closed")
	}
	if e.runtimeMetrics == nil || !e.runtimeMetrics.configured {
		return NewError(ErrorState, "runtime metrics are not configured")
	}
	state := e.runtimeMetrics
	state.enabled = enabled
	if enabled {
		state.initialized = true
		state.next = e.clock.Now().Add(state.interval)
	} else {
		state.next = time.Time{}
	}
	return nil
}

// RuntimeMetricSubscription removes one runtime metric listener.
type RuntimeMetricSubscription struct {
	engine *Engine
	id     uint64
	once   sync.Once
	err    error
}

// Close removes the listener. It is safe to call more than once.
func (s *RuntimeMetricSubscription) Close() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		if s.engine == nil {
			return
		}
		s.engine.mu.Lock()
		if s.engine.runtimeMetrics != nil {
			delete(s.engine.runtimeMetrics.listeners, s.id)
		}
		s.engine.mu.Unlock()
	})
	return s.err
}
