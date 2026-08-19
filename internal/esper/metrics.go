package esper

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// WithRuntimeMetrics configures runtime metrics on explicit virtual-time
// advances. A positive interval schedules reports; a non-positive interval
// retains CurrentRuntimeMetric accounting without periodic delivery. The
// first AdvanceTime call establishes a positive interval's reporting epoch.
func WithRuntimeMetrics(interval time.Duration) EngineOption {
	return func(config *engineConfig) {
		config.runtimeMetricsConfigured = true
		config.runtimeMetricsInterval = interval
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

func newRuntimeMetricsState(interval time.Duration, configured bool) *runtimeMetricsState {
	if !configured {
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
	if !state.enabled || state.interval <= 0 {
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
	if enabled && state.interval > 0 {
		state.initialized = true
		state.next = e.clock.Now().Add(state.interval)
	} else {
		state.initialized = false
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

// StatementMetricGroupConfig assigns statements to a reporting interval.
// Groups are evaluated in declaration order and the first matching group
// wins; statements that match none belong to group-default.
type StatementMetricGroupConfig struct {
	Name           string
	Interval       time.Duration
	ReportInactive bool
	DefaultInclude bool
	patterns       []statementMetricPattern
}

// StatementMetricGroupOption configures one statement metric group.
type StatementMetricGroupOption func(*StatementMetricGroupConfig)

// StatementMetricsGroup creates a named statement-metric group. SQL LIKE
// patterns use '%' for any sequence and '_' for any single character.
func StatementMetricsGroup(name string, interval time.Duration, options ...StatementMetricGroupOption) StatementMetricGroupConfig {
	config := StatementMetricGroupConfig{Name: strings.TrimSpace(name), Interval: interval}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	return config
}

// StatementMetricReportInactive emits a zero-valued record when a group
// interval expires without activity for a deployed statement.
func StatementMetricReportInactive() StatementMetricGroupOption {
	return func(config *StatementMetricGroupConfig) { config.ReportInactive = true }
}

// StatementMetricDefaultInclude starts group matching in the included state;
// later ordered exclude/include patterns can change that state.
func StatementMetricDefaultInclude() StatementMetricGroupOption {
	return func(config *StatementMetricGroupConfig) { config.DefaultInclude = true }
}

// StatementMetricIncludeLike includes names matching one SQL LIKE pattern.
func StatementMetricIncludeLike(pattern string) StatementMetricGroupOption {
	return statementMetricLikeOption(pattern, true)
}

// StatementMetricExcludeLike excludes names matching one SQL LIKE pattern.
func StatementMetricExcludeLike(pattern string) StatementMetricGroupOption {
	return statementMetricLikeOption(pattern, false)
}

// StatementMetricIncludeRegexp includes names matching a compiled regexp.
func StatementMetricIncludeRegexp(pattern *regexp.Regexp) StatementMetricGroupOption {
	return statementMetricRegexpOption(pattern, true)
}

// StatementMetricExcludeRegexp excludes names matching a compiled regexp.
func StatementMetricExcludeRegexp(pattern *regexp.Regexp) StatementMetricGroupOption {
	return statementMetricRegexpOption(pattern, false)
}

func statementMetricLikeOption(pattern string, include bool) StatementMetricGroupOption {
	return func(config *StatementMetricGroupConfig) {
		config.patterns = append(config.patterns, statementMetricPattern{include: include, match: compileStatementMetricLike(pattern)})
	}
}

func statementMetricRegexpOption(pattern *regexp.Regexp, include bool) StatementMetricGroupOption {
	return func(config *StatementMetricGroupConfig) {
		if pattern == nil {
			return
		}
		config.patterns = append(config.patterns, statementMetricPattern{include: include, match: pattern})
	}
}

func compileStatementMetricLike(pattern string) *regexp.Regexp {
	var builder strings.Builder
	builder.WriteString("^")
	escaped := false
	for _, char := range pattern {
		if escaped {
			builder.WriteString(regexp.QuoteMeta(string(char)))
			escaped = false
			continue
		}
		switch char {
		case '\\':
			escaped = true
		case '%':
			builder.WriteString(".*")
		case '_':
			builder.WriteString(".")
		default:
			builder.WriteString(regexp.QuoteMeta(string(char)))
		}
	}
	if escaped {
		builder.WriteString("\\\\")
	}
	builder.WriteString("$")
	return regexp.MustCompile(builder.String())
}

// WithStatementMetrics enables statement-level metrics. Non-positive
// intervals disable scheduling while retaining accounting and allowing the
// interval to be enabled later through SetStatementMetricGroupInterval.
func WithStatementMetrics(defaultInterval time.Duration, groups ...StatementMetricGroupConfig) EngineOption {
	configured := statementMetricsConfig{defaultInterval: defaultInterval, groups: append([]StatementMetricGroupConfig(nil), groups...)}
	return func(config *engineConfig) { config.statementMetrics = &configured }
}

// StatementMetric is one reporting-period record. CPUTime is sampled from
// process CPU time on supported Go platforms; WallTime is monotonic elapsed
// time. Both use nanosecond-resolution time.Duration values.
type StatementMetric struct {
	RuntimeURI       string
	DeploymentID     string
	StatementName    string
	Timestamp        time.Time
	CPUTime          time.Duration
	WallTime         time.Duration
	NumInput         uint64
	NumOutputIStream uint64
	NumOutputRStream uint64
}

// StatementMetricListener receives one periodic statement metric record.
type StatementMetricListener func(context.Context, StatementMetric) error

// StatementMetricGroupSnapshot is a detached view of one configured group.
// Name is the stable repository name (group-default, group-1, ...), while
// ConfigName is the caller-provided name and is empty for the default group.
type StatementMetricGroupSnapshot struct {
	Name           string
	ConfigName     string
	Interval       time.Duration
	ReportInactive bool
	Metrics        []StatementMetric
}

type statementMetricsConfig struct {
	defaultInterval time.Duration
	groups          []StatementMetricGroupConfig
}

type statementMetricPattern struct {
	include bool
	match   *regexp.Regexp
}

func (p statementMetricPattern) matches(name string) bool {
	return p.match != nil && p.match.MatchString(name)
}

func statementMetricGroupMatches(config StatementMetricGroupConfig, name string) bool {
	result := config.DefaultInclude
	for _, pattern := range config.patterns {
		if result {
			if !pattern.include && pattern.matches(name) {
				result = false
			}
		} else if pattern.include && pattern.matches(name) {
			result = true
		}
	}
	return result
}

type statementMetricKey struct {
	deploymentID  string
	statementName string
}

type statementMetricAccumulator struct {
	cpuTime          time.Duration
	wallTime         time.Duration
	numInput         uint64
	numOutputIStream uint64
	numOutputRStream uint64
	active           bool
}

type statementMetricEntry struct {
	key     statementMetricKey
	order   uint64
	group   *statementMetricGroupState
	enabled bool
	removed bool
	metric  statementMetricAccumulator
}

type statementMetricGroupState struct {
	name           string
	configName     string
	interval       time.Duration
	reportInactive bool
	initialized    bool
	next           time.Time
	entries        []*statementMetricEntry
}

type statementMetricsState struct {
	runtimeURI  string
	configured  bool
	enabled     bool
	config      statementMetricsConfig
	groups      []*statementMetricGroupState
	groupByName map[string]*statementMetricGroupState
	entries     map[statementMetricKey]*statementMetricEntry
	listeners   map[uint64]StatementMetricListener
	nextID      uint64
}

func newStatementMetricsState(config *statementMetricsConfig, runtimeURI string) *statementMetricsState {
	if config == nil {
		return nil
	}
	state := &statementMetricsState{
		runtimeURI:  runtimeURI,
		configured:  true,
		enabled:     true,
		config:      statementMetricsConfig{defaultInterval: config.defaultInterval, groups: append([]StatementMetricGroupConfig(nil), config.groups...)},
		groupByName: make(map[string]*statementMetricGroupState),
		entries:     make(map[statementMetricKey]*statementMetricEntry),
		listeners:   make(map[uint64]StatementMetricListener),
	}
	defaultGroup := &statementMetricGroupState{name: "group-default", interval: config.defaultInterval}
	state.groups = append(state.groups, defaultGroup)
	for index, groupConfig := range config.groups {
		group := &statementMetricGroupState{
			name:           fmt.Sprintf("group-%d", index+1),
			configName:     groupConfig.Name,
			interval:       groupConfig.Interval,
			reportInactive: groupConfig.ReportInactive,
		}
		state.groups = append(state.groups, group)
		if group.configName != "" {
			state.groupByName[group.configName] = group
		}
	}
	return state
}

func (e *Engine) registerStatementMetricsLocked(statement *Statement) {
	if e == nil || e.statementMetrics == nil || statement == nil || statement.deployment == nil {
		return
	}
	key := statementMetricKey{deploymentID: statement.deployment.id, statementName: statement.name}
	e.registerStatementMetricEntryLocked(key, statement.deploymentOrder)
}

func (e *Engine) registerNamedWindowMetricsLocked(name string, order uint64) {
	if e == nil || e.statementMetrics == nil || strings.TrimSpace(name) == "" {
		return
	}
	e.registerStatementMetricEntryLocked(statementMetricKey{deploymentID: "environment", statementName: name}, order)
}

func namedWindowMetricName(definition NamedWindowDefinition) string {
	return catalogKey(definition.moduleName, definition.name)
}

func (e *Engine) removeNamedWindowMetricsLocked(name string) {
	if e == nil || e.statementMetrics == nil || strings.TrimSpace(name) == "" {
		return
	}
	key := statementMetricKey{deploymentID: "environment", statementName: name}
	entry := e.statementMetrics.entries[key]
	if entry == nil {
		return
	}
	entry.removed = true
	delete(e.statementMetrics.entries, key)
	if entry.group != nil {
		for index, candidate := range entry.group.entries {
			if candidate == entry {
				entry.group.entries = append(entry.group.entries[:index], entry.group.entries[index+1:]...)
				break
			}
		}
	}
}

func (e *Engine) registerStatementMetricEntryLocked(key statementMetricKey, order uint64) {
	if e == nil || e.statementMetrics == nil {
		return
	}
	state := e.statementMetrics
	if _, exists := state.entries[key]; exists {
		return
	}
	group := state.groups[0]
	config := e.statementMetricsConfigLocked()
	for index, groupConfig := range config.groups {
		if statementMetricGroupMatches(groupConfig, key.statementName) {
			group = state.groups[index+1]
			break
		}
	}
	entry := &statementMetricEntry{key: key, order: order, group: group, enabled: true}
	state.entries[key] = entry
	group.entries = append(group.entries, entry)
}

// statementMetricsConfigLocked returns immutable matching configuration.
func (e *Engine) statementMetricsConfigLocked() statementMetricsConfig {
	if e == nil || e.statementMetrics == nil {
		return statementMetricsConfig{}
	}
	return e.statementMetrics.config
}

func (e *Engine) removeStatementMetricsLocked(statement *Statement) {
	if e == nil || e.statementMetrics == nil || statement == nil || statement.deployment == nil {
		return
	}
	key := statementMetricKey{deploymentID: statement.deployment.id, statementName: statement.name}
	entry := e.statementMetrics.entries[key]
	if entry == nil {
		return
	}
	entry.removed = true
	delete(e.statementMetrics.entries, key)
	if entry.group != nil {
		for index, candidate := range entry.group.entries {
			if candidate == entry {
				entry.group.entries = append(entry.group.entries[:index], entry.group.entries[index+1:]...)
				break
			}
		}
	}
}

type statementMetricSample struct {
	active bool
	wall   time.Time
	cpu    time.Duration
}

func (e *Engine) startStatementMetricSampleLocked(statement *Statement) statementMetricSample {
	entry := e.statementMetricEntryLocked(statement)
	if entry == nil || !entry.enabled {
		return statementMetricSample{}
	}
	return statementMetricSample{active: true, wall: time.Now(), cpu: statementMetricsCPUTime()}
}

func (e *Engine) finishStatementMetricSampleLocked(statement *Statement, sample statementMetricSample, input int) {
	if !sample.active || input <= 0 {
		return
	}
	entry := e.statementMetricEntryLocked(statement)
	if entry == nil || !entry.enabled {
		return
	}
	cpu := statementMetricsCPUTime() - sample.cpu
	wall := time.Since(sample.wall)
	if cpu < 0 {
		cpu = 0
	}
	if wall < 0 {
		wall = 0
	}
	entry.metric.cpuTime += cpu
	entry.metric.wallTime += wall
	entry.metric.numInput += uint64(input)
	entry.metric.active = true
}

func (e *Engine) recordStatementMetricOutputLocked(statement *Statement, batch ResultBatch) {
	entry := e.statementMetricEntryLocked(statement)
	if entry == nil || !entry.enabled || !statementHasMetricOutputConsumer(statement) {
		return
	}
	entry.metric.numOutputIStream += uint64(len(batch.New))
	entry.metric.numOutputRStream += uint64(len(batch.Old))
	entry.metric.active = true
}

func (e *Engine) recordNamedWindowMetricInputLocked(window *NamedWindow, delta NamedWindowDelta) {
	if e == nil || e.statementMetrics == nil || window == nil || window.state == nil {
		return
	}
	entry := e.statementMetrics.entries[statementMetricKey{deploymentID: "environment", statementName: namedWindowMetricName(window.state.def)}]
	if entry == nil || !entry.enabled {
		return
	}
	input := len(delta.New)
	if len(delta.Old) > input {
		input = len(delta.Old)
	}
	if input == 0 {
		return
	}
	entry.metric.numInput += uint64(input)
	entry.metric.active = true
}

func statementHasMetricOutputConsumer(statement *Statement) bool {
	if statement == nil {
		return false
	}
	statement.mu.RLock()
	defer statement.mu.RUnlock()
	return len(statement.listeners) > 0 || statement.subscriber != nil || statement.plan.query.sink != nil
}

func (e *Engine) processStatementWithMetricsLocked(ctx context.Context, statement *Statement, now time.Time, event Event, variables map[string]Value, accepted bool) (batch ResultBatch, changed bool, err error) {
	if statement == nil {
		return ResultBatch{}, false, nil
	}
	defer func() {
		if r := recover(); r != nil {
			err = NewError(ErrorInternal, fmt.Sprintf("panic evaluating statement %q: %v", statement.name, r))
			batch = ResultBatch{}
			changed = false
		}
	}()
	sample := statementMetricSample{}
	if accepted {
		sample = e.startStatementMetricSampleLocked(statement)
	}
	batch, changed, err = statement.process(ctx, now, event, variables)
	if err == nil {
		e.auditStatementProcessLocked(statement, event, now, accepted, batch, changed)
	}
	if err == nil && accepted {
		e.finishStatementMetricSampleLocked(statement, sample, 1)
	}
	if err == nil && changed {
		e.recordStatementMetricOutputLocked(statement, batch)
	}
	return batch, changed, err
}

func (e *Engine) processNamedWindowWithMetricsLocked(ctx context.Context, statement *Statement, now time.Time, window *NamedWindow, delta NamedWindowDelta, variables map[string]Value) (ResultBatch, bool, error) {
	if statement == nil {
		return ResultBatch{}, false, nil
	}
	matchedWindow := statementConsumesNamedWindow(statement.plan.query, window)
	if !containsNamedWindow(statement.plan.query.input, statement.plan.query.join) {
		return statement.processNamedWindow(ctx, now, delta, variables)
	}
	input := len(delta.New)
	if len(delta.Old) > input {
		input = len(delta.Old)
	}
	sample := statementMetricSample{}
	if matchedWindow {
		sample = e.startStatementMetricSampleLocked(statement)
	}
	batch, changed, err := statement.processNamedWindow(ctx, now, delta, variables)
	if err == nil && matchedWindow {
		e.auditNamedWindowProcessLocked(statement, window, delta, now, changed, batch)
	}
	if err == nil && matchedWindow {
		e.finishStatementMetricSampleLocked(statement, sample, input)
		if changed {
			e.recordStatementMetricOutputLocked(statement, batch)
		}
	}
	return batch, changed, err
}

func statementConsumesNamedWindow(query Query, window *NamedWindow) bool {
	if window == nil || window.state == nil {
		return false
	}
	name := window.state.def.Name()
	matches := func(node *streamNode) bool {
		for current := node; current != nil; current = current.input {
			if current.kind == streamNamedWindow && current.sourceName == name {
				return true
			}
		}
		return false
	}
	if query.join != nil {
		for _, source := range joinDefinitionSources(query.join) {
			if matches(source) {
				return true
			}
		}
		return false
	}
	if query.aggregate != nil && query.aggregate.join != nil {
		for _, source := range joinDefinitionSources(query.aggregate.join) {
			if matches(source) {
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
	return matches(input)
}

func (e *Engine) expireStatementWithMetricsLocked(statement *Statement, now time.Time, variables map[string]Value) (ResultBatch, bool) {
	sample := e.startStatementMetricSampleLocked(statement)
	batch, changed := statement.expire(now, variables)
	e.auditStatementScheduleFireLocked(statement, now, batch, changed)
	if changed {
		e.finishStatementMetricSampleLocked(statement, sample, 1)
		e.recordStatementMetricOutputLocked(statement, batch)
	}
	return batch, changed
}

func (e *Engine) statementMetricEntryLocked(statement *Statement) *statementMetricEntry {
	if e == nil || e.statementMetrics == nil || statement == nil || statement.deployment == nil {
		return nil
	}
	return e.statementMetrics.entries[statementMetricKey{deploymentID: statement.deployment.id, statementName: statement.name}]
}

func (e *Engine) statementMetricsDueLocked(at time.Time) ([]StatementMetric, []StatementMetricListener) {
	if e == nil || e.statementMetrics == nil || !e.statementMetrics.enabled {
		return nil, nil
	}
	state := e.statementMetrics
	var metrics []StatementMetric
	for _, group := range state.groups {
		if group.interval <= 0 {
			continue
		}
		if !group.initialized {
			group.initialized = true
			group.next = at.Add(group.interval)
			continue
		}
		if at.Before(group.next) {
			continue
		}
		sort.SliceStable(group.entries, func(i, j int) bool { return group.entries[i].order < group.entries[j].order })
		for _, entry := range group.entries {
			if entry == nil || entry.removed {
				continue
			}
			if entry.metric.active || group.reportInactive {
				metrics = append(metrics, state.metric(entry, at))
			}
			entry.metric = statementMetricAccumulator{}
		}
		group.next = at.Add(group.interval)
	}
	if len(metrics) == 0 {
		return nil, nil
	}
	listeners := make([]StatementMetricListener, 0, len(state.listeners))
	for id := uint64(1); id <= state.nextID; id++ {
		if listener := state.listeners[id]; listener != nil {
			listeners = append(listeners, listener)
		}
	}
	return metrics, listeners
}

func (s *statementMetricsState) metric(entry *statementMetricEntry, at time.Time) StatementMetric {
	return StatementMetric{
		RuntimeURI:       s.runtimeURI,
		DeploymentID:     entry.key.deploymentID,
		StatementName:    entry.key.statementName,
		Timestamp:        at,
		CPUTime:          entry.metric.cpuTime,
		WallTime:         entry.metric.wallTime,
		NumInput:         entry.metric.numInput,
		NumOutputIStream: entry.metric.numOutputIStream,
		NumOutputRStream: entry.metric.numOutputRStream,
	}
}

func dispatchStatementMetrics(ctx context.Context, metrics []StatementMetric, listeners []StatementMetricListener) error {
	for _, metric := range metrics {
		for _, listener := range listeners {
			if listener == nil {
				continue
			}
			if err := listener(ctx, metric); err != nil {
				return fmt.Errorf("esper: statement metric listener for %q: %w", metric.StatementName, err)
			}
		}
	}
	return nil
}

// SubscribeStatementMetrics observes periodic statement metric records.
func (e *Engine) SubscribeStatementMetrics(listener StatementMetricListener) (*StatementMetricSubscription, error) {
	if e == nil {
		return nil, NewError(ErrorDependency, "nil engine")
	}
	if listener == nil {
		return nil, NewError(ErrorInvalidRule, "statement metric listener is nil")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, NewError(ErrorState, "engine is closed")
	}
	if e.statementMetrics == nil || !e.statementMetrics.configured {
		return nil, NewError(ErrorState, "statement metrics are not configured")
	}
	e.statementMetrics.nextID++
	id := e.statementMetrics.nextID
	e.statementMetrics.listeners[id] = listener
	return &StatementMetricSubscription{engine: e, id: id}, nil
}

// CurrentStatementMetricGroups returns current, unflushed counters organized
// by the same repository groups used for periodic reporting.
func (e *Engine) CurrentStatementMetricGroups(ctx context.Context) ([]StatementMetricGroupSnapshot, error) {
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
	if e.statementMetrics == nil || !e.statementMetrics.configured {
		return nil, NewError(ErrorState, "statement metrics are not configured")
	}
	now := e.clock.Now()
	result := make([]StatementMetricGroupSnapshot, 0, len(e.statementMetrics.groups))
	for _, group := range e.statementMetrics.groups {
		snapshot := StatementMetricGroupSnapshot{Name: group.name, ConfigName: group.configName, Interval: group.interval, ReportInactive: group.reportInactive}
		for _, entry := range group.entries {
			if entry == nil || entry.removed || (!entry.metric.active && !group.reportInactive) {
				continue
			}
			snapshot.Metrics = append(snapshot.Metrics, e.statementMetrics.metric(entry, now))
		}
		result = append(result, snapshot)
	}
	return result, nil
}

// SetStatementMetricsEnabled enables or disables accounting for one deployed
// statement without changing its group or reporting schedule.
func (e *Engine) SetStatementMetricsEnabled(deploymentID, statementName string, enabled bool) error {
	if e == nil {
		return NewError(ErrorDependency, "nil engine")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return NewError(ErrorState, "engine is closed")
	}
	if e.statementMetrics == nil || !e.statementMetrics.configured {
		return NewError(ErrorState, "statement metrics are not configured")
	}
	entry := e.statementMetrics.entries[statementMetricKey{deploymentID: deploymentID, statementName: statementName}]
	if entry == nil {
		return NewError(ErrorUnknownName, fmt.Sprintf("statement %q in deployment %q is not in metrics collection", statementName, deploymentID))
	}
	entry.enabled = enabled
	return nil
}

// SetStatementMetricGroupInterval changes one group interval. An empty name
// selects group-default; non-positive intervals disable the group. Enabling a
// group schedules its next report relative to current Engine time.
func (e *Engine) SetStatementMetricGroupInterval(name string, interval time.Duration) error {
	if e == nil {
		return NewError(ErrorDependency, "nil engine")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return NewError(ErrorState, "engine is closed")
	}
	if e.statementMetrics == nil || !e.statementMetrics.configured {
		return NewError(ErrorState, "statement metrics are not configured")
	}
	var group *statementMetricGroupState
	if strings.TrimSpace(name) == "" {
		group = e.statementMetrics.groups[0]
	} else {
		group = e.statementMetrics.groupByName[name]
	}
	if group == nil {
		return NewError(ErrorUnknownName, fmt.Sprintf("statement metric group %q not found", name))
	}
	group.interval = interval
	if interval > 0 && e.statementMetrics.enabled {
		group.initialized = true
		group.next = e.clock.Now().Add(interval)
	} else {
		group.initialized = false
		group.next = time.Time{}
	}
	return nil
}

// SetMetricsReportingEnabled controls all configured runtime and statement
// reporting schedules. Accounting continues while schedules are disabled,
// matching Esper's runtime metrics service.
func (e *Engine) SetMetricsReportingEnabled(enabled bool) error {
	if e == nil {
		return NewError(ErrorDependency, "nil engine")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return NewError(ErrorState, "engine is closed")
	}
	if e.runtimeMetrics == nil && e.statementMetrics == nil {
		return NewError(ErrorState, "metrics reporting is not configured")
	}
	now := e.clock.Now()
	if e.runtimeMetrics != nil {
		e.runtimeMetrics.enabled = enabled
		if enabled && e.runtimeMetrics.interval > 0 {
			e.runtimeMetrics.initialized = true
			e.runtimeMetrics.next = now.Add(e.runtimeMetrics.interval)
		} else {
			e.runtimeMetrics.initialized = false
			e.runtimeMetrics.next = time.Time{}
		}
	}
	if e.statementMetrics != nil {
		e.statementMetrics.enabled = enabled
		for _, group := range e.statementMetrics.groups {
			if enabled && group.interval > 0 {
				group.initialized = true
				group.next = now.Add(group.interval)
			} else {
				group.initialized = false
				group.next = time.Time{}
			}
		}
	}
	return nil
}

// StatementMetricSubscription removes one statement metric listener.
type StatementMetricSubscription struct {
	engine *Engine
	id     uint64
	once   sync.Once
	err    error
}

// Close removes the listener. It is safe to call more than once.
func (s *StatementMetricSubscription) Close() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		if s.engine == nil {
			return
		}
		s.engine.mu.Lock()
		if s.engine.statementMetrics != nil {
			delete(s.engine.statementMetrics.listeners, s.id)
		}
		s.engine.mu.Unlock()
	})
	return s.err
}
