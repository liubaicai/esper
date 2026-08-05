package esper

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"
)

type ContextKind uint8

const (
	ContextKeySegmented ContextKind = iota
	ContextHashSegmented
	ContextCategorySegmented
	ContextInitiatedTerminated
	ContextTimePeriod
	ContextDailyTime
	ContextCronTime
)

// TimeOfDay is a local wall-clock time used by recurring daily contexts.
// The date and location come from the Engine virtual clock; only the clock
// fields participate in the daily window definition.
type TimeOfDay struct {
	Hour       int
	Minute     int
	Second     int
	Nanosecond int
}

// NewTimeOfDay constructs a validated local wall-clock time.
func NewTimeOfDay(hour, minute, second int) (TimeOfDay, error) {
	return NewTimeOfDayN(hour, minute, second, 0)
}

// NewTimeOfDayN is the nanosecond-precision form of NewTimeOfDay.
func NewTimeOfDayN(hour, minute, second, nanosecond int) (TimeOfDay, error) {
	if hour < 0 || hour > 23 {
		return TimeOfDay{}, NewError(ErrorInvalidRule, "time-of-day hour must be in 0..23")
	}
	if minute < 0 || minute > 59 {
		return TimeOfDay{}, NewError(ErrorInvalidRule, "time-of-day minute must be in 0..59")
	}
	if second < 0 || second > 59 {
		return TimeOfDay{}, NewError(ErrorInvalidRule, "time-of-day second must be in 0..59")
	}
	if nanosecond < 0 || nanosecond >= int(time.Second) {
		return TimeOfDay{}, NewError(ErrorInvalidRule, "time-of-day nanosecond must be in 0..999999999")
	}
	return TimeOfDay{Hour: hour, Minute: minute, Second: second, Nanosecond: nanosecond}, nil
}

func (t TimeOfDay) valid() bool {
	return t.Hour >= 0 && t.Hour <= 23 && t.Minute >= 0 && t.Minute <= 59 &&
		t.Second >= 0 && t.Second <= 59 && t.Nanosecond >= 0 && t.Nanosecond < int(time.Second)
}

func (t TimeOfDay) duration() time.Duration {
	return time.Duration(t.Hour)*time.Hour + time.Duration(t.Minute)*time.Minute +
		time.Duration(t.Second)*time.Second + time.Duration(t.Nanosecond)
}

func (t TimeOfDay) String() string {
	return fmt.Sprintf("%02d:%02d:%02d.%09d", t.Hour, t.Minute, t.Second, t.Nanosecond)
}

type ContextCategory struct {
	name      string
	predicate Expression[bool]
}

func NewContextCategory(name string, predicate Expression[bool]) (ContextCategory, error) {
	if strings.TrimSpace(name) == "" {
		return ContextCategory{}, NewError(ErrorInvalidRule, "context category name is required")
	}
	if predicate == nil {
		return ContextCategory{}, NewError(ErrorInvalidRule, "context category predicate is required")
	}
	return ContextCategory{name: name, predicate: predicate}, nil
}

func Category(name string, predicate Expression[bool]) ContextCategory {
	category, _ := NewContextCategory(name, predicate)
	return category
}

func (c ContextCategory) Name() string                { return c.name }
func (c ContextCategory) Predicate() Expression[bool] { return c.predicate }

// ContextDefinition is the compile-time declaration for a key-partitioned
// context. More context initiation/termination modes will use the same
// registry and lifecycle boundary.
type ContextDefinition struct {
	name                 string
	kind                 ContextKind
	key                  Expr
	keys                 []Expr
	partitions           int
	categories           []ContextCategory
	start                Expression[bool]
	end                  Expression[bool]
	initiatedDistinct    bool
	initiatedOverlapping bool
	temporalStartAfter   time.Duration
	temporalActiveFor    time.Duration
	dailyStart           TimeOfDay
	dailyEnd             TimeOfDay
	cronStart            *CronSchedule
	cronEnd              *CronSchedule
	cronStartResolved    *resolvedCronSchedule
	cronEndResolved      *resolvedCronSchedule
	startPattern         *patternDefinition
	endPattern           *patternDefinition
	patternEnvironment   *Environment
	parent               *ContextDefinition
}

const contextVariablePrefix = "\x00esper.context."

func contextVariableName(name string) string {
	return contextVariablePrefix + name
}

func validateContextKeys(keys []Expr) error {
	if len(keys) == 0 {
		return NewError(ErrorInvalidRule, "context key expression is required")
	}
	for index, key := range keys {
		if key == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("context key expression %d is required", index+1))
		}
	}
	return nil
}

func copyContextKeys(keys []Expr) []Expr {
	return append([]Expr(nil), keys...)
}

// NewKeyContext declares a context partitioned by one or more expressions.
// With multiple expressions the complete tuple is used as the partition key,
// matching Esper's multi-key segmented context behavior.
func NewKeyContext(name string, keys ...Expr) (ContextDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "context name is required")
	}
	if err := validateContextKeys(keys); err != nil {
		return ContextDefinition{}, err
	}
	return ContextDefinition{name: name, kind: ContextKeySegmented, key: keys[0], keys: copyContextKeys(keys)}, nil
}

func NewHashContext(name string, key Expr, partitions int) (ContextDefinition, error) {
	return NewHashContextBy(name, partitions, key)
}

// NewHashContextBy declares a hash-partitioned context over a key tuple.
// The single-key NewHashContext form remains available for the common case.
func NewHashContextBy(name string, partitions int, keys ...Expr) (ContextDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "context name is required")
	}
	if err := validateContextKeys(keys); err != nil {
		return ContextDefinition{}, err
	}
	if partitions <= 0 {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "hash context partitions must be positive")
	}
	return ContextDefinition{name: name, kind: ContextHashSegmented, key: keys[0], keys: copyContextKeys(keys), partitions: partitions}, nil
}

func NewCategoryContext(name string, categories ...ContextCategory) (ContextDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "context name is required")
	}
	if len(categories) == 0 {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "category context requires at least one category")
	}
	seen := make(map[string]struct{}, len(categories))
	copyCategories := make([]ContextCategory, 0, len(categories))
	for _, category := range categories {
		if category.name == "" || category.predicate == nil {
			return ContextDefinition{}, NewError(ErrorInvalidRule, "category context contains an invalid category")
		}
		if _, exists := seen[category.name]; exists {
			return ContextDefinition{}, NewError(ErrorInvalidRule, fmt.Sprintf("category %q is duplicated", category.name))
		}
		seen[category.name] = struct{}{}
		copyCategories = append(copyCategories, category)
	}
	return ContextDefinition{name: name, kind: ContextCategorySegmented, categories: copyCategories}, nil
}

// NewInitiatedTerminatedContext declares a single-key context whose partition
// is created by start and removed after an event satisfying end is processed.
// The explicit key keeps overlapping lifecycles deterministic; distinct start
// keys create independent partitions.
func NewInitiatedTerminatedContext(name string, key Expr, start, end Expression[bool]) (ContextDefinition, error) {
	return NewInitiatedTerminatedContextBy(name, []Expr{key}, start, end)
}

// NewInitiatedTerminatedContextBy declares an initiated-terminated context
// keyed by the complete tuple in keys. It retains the current non-overlapping
// lifecycle contract while allowing a composite initiation identity.
func NewInitiatedTerminatedContextBy(name string, keys []Expr, start, end Expression[bool]) (ContextDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "context name is required")
	}
	if err := validateContextKeys(keys); err != nil {
		return ContextDefinition{}, err
	}
	if start == nil || end == nil {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "initiated-terminated context requires start and end expressions")
	}
	return ContextDefinition{name: name, kind: ContextInitiatedTerminated, key: keys[0], keys: copyContextKeys(keys), start: start, end: end}, nil
}

// NewInitiatedContext declares a non-overlapping initiated context without an
// automatic termination condition. A key can be active once and remains
// active until the statement/context is undeployed.
func NewInitiatedContext(name string, key Expr, start Expression[bool]) (ContextDefinition, error) {
	return NewInitiatedContextBy(name, []Expr{key}, start)
}

// NewInitiatedContextBy is the multi-key form of NewInitiatedContext.
func NewInitiatedContextBy(name string, keys []Expr, start Expression[bool]) (ContextDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "context name is required")
	}
	if err := validateContextKeys(keys); err != nil {
		return ContextDefinition{}, err
	}
	if start == nil {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "initiated context requires a start expression")
	}
	return ContextDefinition{name: name, kind: ContextInitiatedTerminated, key: keys[0], keys: copyContextKeys(keys), start: start}, nil
}

// NewOverlappingInitiatedTerminatedContext declares an initiated context that
// creates a new active instance for every matching start event, including
// repeated events with the same key tuple. Events accepted by a context
// statement are broadcast to all active instances.
func NewOverlappingInitiatedTerminatedContext(name string, key Expr, start, end Expression[bool]) (ContextDefinition, error) {
	return NewOverlappingInitiatedTerminatedContextBy(name, []Expr{key}, start, end)
}

// NewOverlappingInitiatedTerminatedContextBy is the multi-key form of
// NewOverlappingInitiatedTerminatedContext.
func NewOverlappingInitiatedTerminatedContextBy(name string, keys []Expr, start, end Expression[bool]) (ContextDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "context name is required")
	}
	if err := validateContextKeys(keys); err != nil {
		return ContextDefinition{}, err
	}
	if start == nil || end == nil {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "overlapping initiated-terminated context requires start and end expressions")
	}
	return ContextDefinition{
		name:                 name,
		kind:                 ContextInitiatedTerminated,
		key:                  keys[0],
		keys:                 copyContextKeys(keys),
		start:                start,
		end:                  end,
		initiatedOverlapping: true,
	}, nil
}

// NewOverlappingInitiatedContext declares an overlapping initiated context
// without a termination condition.
func NewOverlappingInitiatedContext(name string, key Expr, start Expression[bool]) (ContextDefinition, error) {
	return NewOverlappingInitiatedContextBy(name, []Expr{key}, start)
}

// NewOverlappingInitiatedContextBy is the multi-key form of
// NewOverlappingInitiatedContext.
func NewOverlappingInitiatedContextBy(name string, keys []Expr, start Expression[bool]) (ContextDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "context name is required")
	}
	if err := validateContextKeys(keys); err != nil {
		return ContextDefinition{}, err
	}
	if start == nil {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "overlapping initiated context requires a start expression")
	}
	return ContextDefinition{
		name:                 name,
		kind:                 ContextInitiatedTerminated,
		key:                  keys[0],
		keys:                 copyContextKeys(keys),
		start:                start,
		initiatedOverlapping: true,
	}, nil
}

// NewDistinctInitiatedTerminatedContext declares an initiated-terminated
// context that creates at most one active partition for each initiation key
// tuple. Events accepted by a context statement are broadcast to every active
// distinct partition, matching Esper's initiated-by-distinct lifecycle rather
// than treating the tuple as a routing key for the statement stream.
func NewDistinctInitiatedTerminatedContext(name string, key Expr, start, end Expression[bool]) (ContextDefinition, error) {
	return NewDistinctInitiatedTerminatedContextBy(name, []Expr{key}, start, end)
}

// NewDistinctInitiatedTerminatedContextBy is the multi-key form of
// NewDistinctInitiatedTerminatedContext. Null and array components participate
// in the stable tuple identity used to suppress duplicate initiations.
func NewDistinctInitiatedTerminatedContextBy(name string, keys []Expr, start, end Expression[bool]) (ContextDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "context name is required")
	}
	if err := validateContextKeys(keys); err != nil {
		return ContextDefinition{}, err
	}
	if start == nil || end == nil {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "initiated-terminated context requires start and end expressions")
	}
	return ContextDefinition{
		name:              name,
		kind:              ContextInitiatedTerminated,
		key:               keys[0],
		keys:              copyContextKeys(keys),
		start:             start,
		end:               end,
		initiatedDistinct: true,
	}, nil
}

// NewPatternInitiatedTerminatedContext declares an initiated context whose
// lifecycle is driven by two fluent PatternStream definitions. Captured start
// tags are retained by each context partition and are available through
// ContextPatternEvent/ContextPatternField while the partition is active.
// Pure timer-root and supported timer-plus-event observers are driven by
// Engine.AdvanceTime. Each active pattern match owns its observer state;
// advanced guard/consumption combinations remain separate parity work.
func NewPatternInitiatedTerminatedContext(name string, start, end PatternStream) (ContextDefinition, error) {
	return newPatternInitiatedTerminatedContext(name, start, end, false, true)
}

// NewPatternInitiatedContext declares an event-pattern initiated context with
// no automatic termination condition. The context remains active until the
// statement is undeployed.
func NewPatternInitiatedContext(name string, start PatternStream) (ContextDefinition, error) {
	return newPatternInitiatedTerminatedContext(name, start, PatternStream{}, false, false)
}

// NewOverlappingPatternInitiatedTerminatedContext declares a pattern context
// that allocates one partition for every completed start pattern, even when
// the start pattern itself does not use Every.
func NewOverlappingPatternInitiatedTerminatedContext(name string, start, end PatternStream) (ContextDefinition, error) {
	return newPatternInitiatedTerminatedContext(name, start, end, true, true)
}

// NewOverlappingPatternInitiatedContext is the no-termination form of the
// explicit overlapping pattern context constructor.
func NewOverlappingPatternInitiatedContext(name string, start PatternStream) (ContextDefinition, error) {
	return newPatternInitiatedTerminatedContext(name, start, PatternStream{}, true, false)
}

func newPatternInitiatedTerminatedContext(name string, start, end PatternStream, overlapping, terminated bool) (ContextDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "context name is required")
	}
	if err := validateContextPatternStream(start, "start"); err != nil {
		return ContextDefinition{}, err
	}
	if terminated {
		if err := validateContextPatternStream(end, "end"); err != nil {
			return ContextDefinition{}, err
		}
		if start.env != end.env {
			return ContextDefinition{}, NewError(ErrorDependency, "pattern context start and end require the same environment")
		}
	}
	if patternDefinitionRepeats(start.def) || contextPatternIsRecurringTimer(start.def) {
		overlapping = true
	}
	return ContextDefinition{
		name:                 name,
		kind:                 ContextInitiatedTerminated,
		startPattern:         start.def,
		endPattern:           end.def,
		patternEnvironment:   start.env,
		initiatedOverlapping: overlapping,
	}, nil
}

func patternDefinitionRepeats(definition *patternDefinition) bool {
	if definition == nil {
		return false
	}
	if definition.every || definition.everyDistinct != nil {
		return true
	}
	return patternRootRepeats(definition.root)
}

func patternRootRepeats(node *patternNode) bool {
	if node == nil {
		return false
	}
	if node.kind == patternEveryNode {
		return true
	}
	return node.kind == patternWithinNode && node.child != nil && node.child.kind == patternEveryNode
}

func validateContextPatternStream(pattern PatternStream, label string) error {
	if pattern.env == nil || pattern.def == nil {
		return NewError(ErrorInvalidRule, fmt.Sprintf("pattern context %s pattern is required", label))
	}
	if err := validatePattern(pattern.def); err != nil {
		return WrapError(ErrorInvalidRule, "context "+label+" pattern", err)
	}
	return nil
}

func contextPatternIsRecurringTimer(definition *patternDefinition) bool {
	if definition == nil || definition.root == nil {
		return false
	}
	switch definition.root.kind {
	case patternTimerIntervalNode, patternTimerScheduleNode, patternTimerCronNode:
		return true
	default:
		return false
	}
}

// NewTimePeriodContext declares a recurring temporal context. The first
// active interval starts startAfter after the context is created; each active
// interval lasts activeFor and the next interval starts after the same
// startAfter gap. This is the chain-API equivalent of Esper's
// "start after ... end after ..." temporal context form.
func NewTimePeriodContext(name string, startAfter, activeFor time.Duration) (ContextDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "context name is required")
	}
	if startAfter < 0 {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "temporal context start delay cannot be negative")
	}
	if activeFor <= 0 {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "temporal context active duration must be positive")
	}
	const maxDuration = time.Duration(1<<63 - 1)
	if startAfter > maxDuration-activeFor {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "temporal context period overflows time.Duration")
	}
	return ContextDefinition{
		name:               name,
		kind:               ContextTimePeriod,
		temporalStartAfter: startAfter,
		temporalActiveFor:  activeFor,
	}, nil
}

// NewPeriodicContext is an expressive alias for NewTimePeriodContext.
func NewPeriodicContext(name string, startAfter, activeFor time.Duration) (ContextDefinition, error) {
	return NewTimePeriodContext(name, startAfter, activeFor)
}

// NewDailyTimeContext declares a recurring local-time window. The start is
// inclusive and the end is exclusive. If end is earlier than start, the
// window crosses midnight and ends on the following local date.
func NewDailyTimeContext(name string, start, end TimeOfDay) (ContextDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "context name is required")
	}
	if !start.valid() || !end.valid() {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "daily temporal context contains an invalid time-of-day")
	}
	if start.duration() == end.duration() {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "daily temporal context start and end cannot be equal")
	}
	return ContextDefinition{name: name, kind: ContextDailyTime, dailyStart: start, dailyEnd: end}, nil
}

// NewCronTimeContext declares a recurring context whose active window starts
// at one fixed CronSchedule occurrence and ends at the next occurrence of a
// second fixed schedule. Schedule expressions are intentionally rejected in
// this first slice; dynamic variable schedules need a runtime re-resolution
// contract and are tracked separately from static calendar semantics.
func NewCronTimeContext(name string, start, end CronSchedule) (ContextDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "context name is required")
	}
	startResolved, err := resolveStaticContextCron(start, "start")
	if err != nil {
		return ContextDefinition{}, err
	}
	endResolved, err := resolveStaticContextCron(end, "end")
	if err != nil {
		return ContextDefinition{}, err
	}
	startCopy := start
	endCopy := end
	return ContextDefinition{
		name:              name,
		kind:              ContextCronTime,
		cronStart:         &startCopy,
		cronEnd:           &endCopy,
		cronStartResolved: &startResolved,
		cronEndResolved:   &endResolved,
	}, nil
}

func resolveStaticContextCron(schedule CronSchedule, label string) (resolvedCronSchedule, error) {
	if err := schedule.validate(); err != nil {
		return resolvedCronSchedule{}, WrapError(ErrorInvalidRule, "context "+label+" schedule", err)
	}
	fields := []CronField{schedule.Minute, schedule.Hour, schedule.DayOfMonth, schedule.Month, schedule.Weekday}
	if schedule.hasSeconds() {
		fields = append(fields, schedule.Second)
	}
	if schedule.hasMilliseconds() {
		fields = append(fields, schedule.Millisecond)
	}
	if schedule.hasMicroseconds() {
		fields = append(fields, schedule.Microsecond)
	}
	for _, field := range fields {
		if len(field.expressions()) > 0 {
			return resolvedCronSchedule{}, NewError(ErrorInvalidRule, "cron temporal context requires constant schedule fields")
		}
	}
	resolved, err := schedule.resolve(EvalContext{})
	if err != nil {
		return resolvedCronSchedule{}, WrapError(ErrorInvalidRule, "context "+label+" schedule", err)
	}
	return resolved, nil
}

// NewNestedContext composes a child context under an existing parent. The
// resulting runtime partition key contains both levels and keeps the child
// state isolated inside its parent partition. Keyed initiated children are
// supported below non-initiated parents; pattern children and initiated
// parents remain explicit follow-up capabilities.
func NewNestedContext(name string, parent ContextDefinition, child ContextDefinition) (ContextDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "nested context name is required")
	}
	if parent.name == "" {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "nested context parent is required")
	}
	if parent.kind == ContextInitiatedTerminated {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "initiated-terminated parent contexts cannot be nested")
	}
	if parent.isTemporal() || child.isTemporal() {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "temporal contexts cannot be nested")
	}
	if child.name == "" {
		return ContextDefinition{}, NewError(ErrorInvalidRule, "nested context child is required")
	}
	switch child.kind {
	case ContextKeySegmented:
		if len(child.contextKeys()) == 0 {
			return ContextDefinition{}, NewError(ErrorInvalidRule, "nested key context requires a key expression")
		}
	case ContextHashSegmented:
		if len(child.contextKeys()) == 0 || child.partitions <= 0 {
			return ContextDefinition{}, NewError(ErrorInvalidRule, "nested hash context requires a key and positive partitions")
		}
	case ContextCategorySegmented:
		if len(child.categories) == 0 {
			return ContextDefinition{}, NewError(ErrorInvalidRule, "nested category context requires categories")
		}
	case ContextInitiatedTerminated:
		if child.startPattern != nil {
			return ContextDefinition{}, NewError(ErrorInvalidRule, "pattern initiated children are not supported in nested contexts")
		}
		if len(child.contextKeys()) == 0 {
			return ContextDefinition{}, NewError(ErrorInvalidRule, "nested initiated context requires a key expression")
		}
	default:
		return ContextDefinition{}, NewError(ErrorInvalidRule, "unknown nested context kind")
	}
	parentCopy := parent
	childCopy := child
	childCopy.name = name
	childCopy.keys = copyContextKeys(child.keys)
	childCopy.categories = append([]ContextCategory(nil), child.categories...)
	childCopy.parent = &parentCopy
	if childCopy.kind != ContextInitiatedTerminated {
		childCopy.start = nil
		childCopy.end = nil
	}
	return childCopy, nil
}

func (d ContextDefinition) Name() string               { return d.name }
func (d ContextDefinition) Key() Expr                  { return d.key }
func (d ContextDefinition) Keys() []Expr               { return copyContextKeys(d.contextKeys()) }
func (d ContextDefinition) Kind() ContextKind          { return d.kind }
func (d ContextDefinition) Partitions() int            { return d.partitions }
func (d ContextDefinition) InitiatedDistinct() bool    { return d.initiatedDistinct }
func (d ContextDefinition) InitiatedOverlapping() bool { return d.initiatedOverlapping }
func (d ContextDefinition) HasTermination() bool       { return d.end != nil || d.endPattern != nil }
func (d ContextDefinition) Parent() (ContextDefinition, bool) {
	if d.parent == nil {
		return ContextDefinition{}, false
	}
	return *d.parent, true
}
func (d ContextDefinition) Categories() []ContextCategory {
	return append([]ContextCategory(nil), d.categories...)
}

func (d ContextDefinition) description() string {
	local := d.localDescription()
	if d.parent != nil {
		return "nested(parent=" + d.parent.name + "," + local + ")"
	}
	return local
}

func (d ContextDefinition) localDescription() string {
	keyDescription := d.keyDescription()
	switch d.kind {
	case ContextHashSegmented:
		return fmt.Sprintf("hash(%s,%d)", keyDescription, d.partitions)
	case ContextCategorySegmented:
		parts := make([]string, 0, len(d.categories))
		for _, category := range d.categories {
			parts = append(parts, category.name+":"+category.predicate.Description())
		}
		return "category(" + strings.Join(parts, ",") + ")"
	case ContextInitiatedTerminated:
		prefix := "initiated-terminated"
		if d.initiatedDistinct {
			prefix = "initiated-terminated-distinct"
		} else if d.initiatedOverlapping {
			prefix = "initiated-overlapping"
		}
		if d.startPattern != nil {
			endDescription := "<none>"
			if d.endPattern != nil {
				endDescription = d.endPattern.description()
			}
			return prefix + "(pattern-start=" + d.startPattern.description() + ",pattern-end=" + endDescription + ")"
		}
		endDescription := "<none>"
		if d.end != nil {
			endDescription = d.end.Description()
		}
		return prefix + "(" + keyDescription + ",start=" + d.start.Description() + ",end=" + endDescription + ")"
	case ContextTimePeriod:
		return fmt.Sprintf("time-period(start-after=%s,active-for=%s)", d.temporalStartAfter, d.temporalActiveFor)
	case ContextDailyTime:
		return fmt.Sprintf("daily-time(start=%s,end=%s)", d.dailyStart, d.dailyEnd)
	case ContextCronTime:
		if d.cronStart == nil || d.cronEnd == nil {
			return "cron-time(<invalid>)"
		}
		return fmt.Sprintf("cron-time(start=%s,end=%s)", d.cronStart.description(), d.cronEnd.description())
	default:
		return "key(" + keyDescription + ")"
	}
}

func (d ContextDefinition) contextKeys() []Expr {
	if len(d.keys) != 0 {
		return d.keys
	}
	if d.key != nil {
		return []Expr{d.key}
	}
	return nil
}

func (d ContextDefinition) keyDescription() string {
	keys := d.contextKeys()
	descriptions := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == nil {
			continue
		}
		descriptions = append(descriptions, key.Description())
	}
	return strings.Join(descriptions, ",")
}

func (d ContextDefinition) evaluatedKeyResults(event Event, now time.Time, variables map[string]Value) []Value {
	keys := d.contextKeys()
	values := make([]Value, 0, len(keys))
	for _, key := range keys {
		values = append(values, key.eval(EvalContext{Event: event, Now: now, Variables: variables}))
	}
	return values
}

func (d ContextDefinition) evaluatedKeyValues(event Event, now time.Time, variables map[string]Value) []any {
	results := d.evaluatedKeyResults(event, now, variables)
	values := make([]any, 0, len(results)*2)
	for _, value := range results {
		values = append(values, value.State(), value.Any())
	}
	return values
}

func (d ContextDefinition) isTemporal() bool {
	return d.kind == ContextTimePeriod || d.kind == ContextDailyTime || d.kind == ContextCronTime
}

// temporalWindow returns the active interval containing now. The origin is
// shared by all statements using one Environment/Engine context so a late
// statement joins the same temporal cycle instead of starting a private one.
func (d ContextDefinition) temporalWindow(origin, now time.Time) (time.Time, time.Time, bool) {
	if !d.isTemporal() || origin.IsZero() || now.Before(origin) {
		return time.Time{}, time.Time{}, false
	}
	if d.kind == ContextDailyTime {
		return d.dailyWindow(origin, now)
	}
	if d.kind == ContextCronTime {
		return d.cronWindow(origin, now)
	}
	period := d.temporalStartAfter + d.temporalActiveFor
	elapsed := now.Sub(origin)
	if elapsed < d.temporalStartAfter {
		return time.Time{}, time.Time{}, false
	}
	cycle := (elapsed - d.temporalStartAfter) / period
	start := origin.Add(d.temporalStartAfter + cycle*period)
	end := start.Add(d.temporalActiveFor)
	if now.Before(end) {
		return start, end, true
	}
	return time.Time{}, time.Time{}, false
}

func (d ContextDefinition) cronWindow(origin, now time.Time) (time.Time, time.Time, bool) {
	if d.cronStartResolved == nil || d.cronEndResolved == nil {
		return time.Time{}, time.Time{}, false
	}
	start, err := d.cronStartResolved.previousOrAt(now)
	if err != nil || start.Before(origin) {
		return time.Time{}, time.Time{}, false
	}
	end, err := d.cronEndResolved.nextAfter(start)
	if err != nil || !now.Before(end) {
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

func (d ContextDefinition) dailyWindow(origin, now time.Time) (time.Time, time.Time, bool) {
	location := origin.Location()
	if location == nil {
		location = time.UTC
	}
	localNow := now.In(location)
	localOrigin := origin.In(location)
	for offset := -1; offset <= 1; offset++ {
		date := localNow.AddDate(0, 0, offset)
		start := time.Date(date.Year(), date.Month(), date.Day(), d.dailyStart.Hour, d.dailyStart.Minute, d.dailyStart.Second, d.dailyStart.Nanosecond, location)
		endDate := date
		if d.dailyEnd.duration() < d.dailyStart.duration() {
			endDate = date.AddDate(0, 0, 1)
		}
		end := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), d.dailyEnd.Hour, d.dailyEnd.Minute, d.dailyEnd.Second, d.dailyEnd.Nanosecond, location)
		if start.Before(localOrigin) {
			continue
		}
		if !localNow.Before(start) && localNow.Before(end) {
			return start, end, true
		}
	}
	return time.Time{}, time.Time{}, false
}

func temporalPartitionKey(start time.Time) string {
	return fmt.Sprintf("temporal:%d", start.UnixNano())
}

func activeTemporalContextPartitionKey(engine *Engine, definition ContextDefinition, now time.Time) string {
	if engine == nil || !definition.isTemporal() {
		return ""
	}
	origin, ok := engine.contextTemporalOrigins[definition.name]
	if !ok || origin.IsZero() {
		origin = now
		engine.contextTemporalOrigins[definition.name] = origin
	}
	start, _, active := definition.temporalWindow(origin, now)
	if !active {
		return ""
	}
	return temporalPartitionKey(start)
}

// contextPropertyValues materializes the stable properties visible to a
// statement running inside one context partition. The internal map is merged
// into EvalContext variables by statementRuntime; callers use ContextField and
// do not need to know the reserved variable names.
func (d ContextDefinition) contextPropertyValues(event Event, now time.Time, variables map[string]Value, partitionID int) map[string]Value {
	properties := make(map[string]Value)
	if d.parent != nil {
		for name, value := range d.parent.contextPropertyValues(event, now, variables, partitionID) {
			properties["parent."+name] = value
		}
	}
	properties["name"] = Present(d.name)
	properties["id"] = Present(partitionID)
	keyResults := d.evaluatedKeyResults(event, now, variables)
	for index, value := range keyResults {
		properties[fmt.Sprintf("key%d", index+1)] = value
	}
	switch d.kind {
	case ContextCategorySegmented:
		for _, category := range d.categories {
			value := category.predicate.eval(EvalContext{Event: event, Now: now, Variables: variables})
			matched, ok := boolValue(value)
			if ok && matched {
				properties["label"] = Present(category.name)
				break
			}
		}
	case ContextHashSegmented:
		hasher := fnv.New32a()
		_, _ = hasher.Write([]byte(encodeKey(d.evaluatedKeyValues(event, now, variables))))
		properties["hash"] = Present(int64(int32(hasher.Sum32())))
	}
	return properties
}

func (d ContextDefinition) partition(event Event, now time.Time, variables map[string]Value) (string, bool, error) {
	if d.parent != nil {
		parentKey, active, err := d.parent.partition(event, now, variables)
		if err != nil || !active {
			return "", active, err
		}
		childKey, childActive, err := d.partitionLocal(event, now, variables)
		if err != nil || !childActive {
			return "", childActive, err
		}
		return encodeKey([]any{"nested", parentKey, childKey}), true, nil
	}
	return d.partitionLocal(event, now, variables)
}

func (d ContextDefinition) partitionLocal(event Event, now time.Time, variables map[string]Value) (string, bool, error) {
	switch d.kind {
	case ContextCategorySegmented:
		for _, category := range d.categories {
			value := category.predicate.eval(EvalContext{Event: event, Now: now, Variables: variables})
			matched, ok := boolValue(value)
			if ok && matched {
				return "category:" + category.name, true, nil
			}
		}
		return "", false, nil
	case ContextHashSegmented:
		hasher := fnv.New32a()
		_, _ = hasher.Write([]byte(encodeKey(d.evaluatedKeyValues(event, now, variables))))
		return fmt.Sprintf("hash:%d", hasher.Sum32()%uint32(d.partitions)), true, nil
	case ContextKeySegmented:
		return encodeKey(d.evaluatedKeyValues(event, now, variables)), true, nil
	case ContextInitiatedTerminated:
		return "initiated:" + encodeKey(d.evaluatedKeyValues(event, now, variables)), true, nil
	case ContextTimePeriod, ContextDailyTime, ContextCronTime:
		return "", false, nil
	default:
		return "", false, NewError(ErrorInvalidRule, "unknown context kind")
	}
}

func (e *Environment) RegisterContext(name string, keys ...Expr) (ContextDefinition, error) {
	definition, err := NewKeyContext(name, keys...)
	if err != nil {
		return ContextDefinition{}, err
	}
	if e == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	return e.registerContextDefinition(definition)
}

func CreateKeyContext(env *Environment, name string, keys ...Expr) (ContextDefinition, error) {
	if env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	return env.RegisterContext(name, keys...)
}

func CreateHashContext(env *Environment, name string, key Expr, partitions int) (ContextDefinition, error) {
	return CreateHashContextBy(env, name, partitions, key)
}

func CreateHashContextBy(env *Environment, name string, partitions int, keys ...Expr) (ContextDefinition, error) {
	if env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	definition, err := NewHashContextBy(name, partitions, keys...)
	if err != nil {
		return ContextDefinition{}, err
	}
	return env.registerContextDefinition(definition)
}

func CreateCategoryContext(env *Environment, name string, categories ...ContextCategory) (ContextDefinition, error) {
	if env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	definition, err := NewCategoryContext(name, categories...)
	if err != nil {
		return ContextDefinition{}, err
	}
	return env.registerContextDefinition(definition)
}

func CreateInitiatedTerminatedContext(env *Environment, name string, key Expr, start, end Expression[bool]) (ContextDefinition, error) {
	return CreateInitiatedTerminatedContextBy(env, name, []Expr{key}, start, end)
}

func CreateInitiatedTerminatedContextBy(env *Environment, name string, keys []Expr, start, end Expression[bool]) (ContextDefinition, error) {
	if env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	definition, err := NewInitiatedTerminatedContextBy(name, keys, start, end)
	if err != nil {
		return ContextDefinition{}, err
	}
	return env.registerContextDefinition(definition)
}

// CreateInitiatedContext registers a non-overlapping initiated context with
// no automatic termination condition.
func CreateInitiatedContext(env *Environment, name string, key Expr, start Expression[bool]) (ContextDefinition, error) {
	return CreateInitiatedContextBy(env, name, []Expr{key}, start)
}

// CreateInitiatedContextBy registers the multi-key no-termination form.
func CreateInitiatedContextBy(env *Environment, name string, keys []Expr, start Expression[bool]) (ContextDefinition, error) {
	if env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	definition, err := NewInitiatedContextBy(name, keys, start)
	if err != nil {
		return ContextDefinition{}, err
	}
	return env.registerContextDefinition(definition)
}

// CreatePatternInitiatedTerminatedContext registers an event-pattern
// initiated-terminated context in env.
func CreatePatternInitiatedTerminatedContext(env *Environment, name string, start, end PatternStream) (ContextDefinition, error) {
	if env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	definition, err := NewPatternInitiatedTerminatedContext(name, start, end)
	if err != nil {
		return ContextDefinition{}, err
	}
	if definition.patternEnvironment != env {
		return ContextDefinition{}, NewError(ErrorDependency, "pattern context belongs to a different environment")
	}
	return env.registerContextDefinition(definition)
}

// CreatePatternInitiatedContext registers the no-termination pattern form.
func CreatePatternInitiatedContext(env *Environment, name string, start PatternStream) (ContextDefinition, error) {
	if env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	definition, err := NewPatternInitiatedContext(name, start)
	if err != nil {
		return ContextDefinition{}, err
	}
	if definition.patternEnvironment != env {
		return ContextDefinition{}, NewError(ErrorDependency, "pattern context belongs to a different environment")
	}
	return env.registerContextDefinition(definition)
}

// CreateOverlappingPatternInitiatedTerminatedContext registers an explicit
// overlapping pattern lifecycle.
func CreateOverlappingPatternInitiatedTerminatedContext(env *Environment, name string, start, end PatternStream) (ContextDefinition, error) {
	if env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	definition, err := NewOverlappingPatternInitiatedTerminatedContext(name, start, end)
	if err != nil {
		return ContextDefinition{}, err
	}
	if definition.patternEnvironment != env {
		return ContextDefinition{}, NewError(ErrorDependency, "pattern context belongs to a different environment")
	}
	return env.registerContextDefinition(definition)
}

// CreateOverlappingPatternInitiatedContext registers the explicit overlapping
// no-termination pattern form.
func CreateOverlappingPatternInitiatedContext(env *Environment, name string, start PatternStream) (ContextDefinition, error) {
	if env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	definition, err := NewOverlappingPatternInitiatedContext(name, start)
	if err != nil {
		return ContextDefinition{}, err
	}
	if definition.patternEnvironment != env {
		return ContextDefinition{}, NewError(ErrorDependency, "pattern context belongs to a different environment")
	}
	return env.registerContextDefinition(definition)
}

// CreateOverlappingInitiatedTerminatedContext registers an overlapping
// initiated-terminated context.
func CreateOverlappingInitiatedTerminatedContext(env *Environment, name string, key Expr, start, end Expression[bool]) (ContextDefinition, error) {
	return CreateOverlappingInitiatedTerminatedContextBy(env, name, []Expr{key}, start, end)
}

// CreateOverlappingInitiatedTerminatedContextBy registers the multi-key
// overlapping initiated-terminated form.
func CreateOverlappingInitiatedTerminatedContextBy(env *Environment, name string, keys []Expr, start, end Expression[bool]) (ContextDefinition, error) {
	if env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	definition, err := NewOverlappingInitiatedTerminatedContextBy(name, keys, start, end)
	if err != nil {
		return ContextDefinition{}, err
	}
	return env.registerContextDefinition(definition)
}

// CreateOverlappingInitiatedContext registers an overlapping initiated
// context with no automatic termination condition.
func CreateOverlappingInitiatedContext(env *Environment, name string, key Expr, start Expression[bool]) (ContextDefinition, error) {
	return CreateOverlappingInitiatedContextBy(env, name, []Expr{key}, start)
}

// CreateOverlappingInitiatedContextBy registers the multi-key overlapping
// no-termination form.
func CreateOverlappingInitiatedContextBy(env *Environment, name string, keys []Expr, start Expression[bool]) (ContextDefinition, error) {
	if env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	definition, err := NewOverlappingInitiatedContextBy(name, keys, start)
	if err != nil {
		return ContextDefinition{}, err
	}
	return env.registerContextDefinition(definition)
}

// CreateDistinctInitiatedTerminatedContext registers a distinct initiation
// context in env.
func CreateDistinctInitiatedTerminatedContext(env *Environment, name string, key Expr, start, end Expression[bool]) (ContextDefinition, error) {
	return CreateDistinctInitiatedTerminatedContextBy(env, name, []Expr{key}, start, end)
}

// CreateDistinctInitiatedTerminatedContextBy registers the multi-key distinct
// initiation form in env.
func CreateDistinctInitiatedTerminatedContextBy(env *Environment, name string, keys []Expr, start, end Expression[bool]) (ContextDefinition, error) {
	if env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	definition, err := NewDistinctInitiatedTerminatedContextBy(name, keys, start, end)
	if err != nil {
		return ContextDefinition{}, err
	}
	return env.registerContextDefinition(definition)
}

// CreateTimePeriodContext registers a recurring temporal context in env.
func CreateTimePeriodContext(env *Environment, name string, startAfter, activeFor time.Duration) (ContextDefinition, error) {
	if env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	definition, err := NewTimePeriodContext(name, startAfter, activeFor)
	if err != nil {
		return ContextDefinition{}, err
	}
	return env.registerContextDefinition(definition)
}

// CreatePeriodicContext is an expressive alias for CreateTimePeriodContext.
func CreatePeriodicContext(env *Environment, name string, startAfter, activeFor time.Duration) (ContextDefinition, error) {
	return CreateTimePeriodContext(env, name, startAfter, activeFor)
}

// CreateDailyTimeContext registers a recurring local-time context in env.
func CreateDailyTimeContext(env *Environment, name string, start, end TimeOfDay) (ContextDefinition, error) {
	if env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	definition, err := NewDailyTimeContext(name, start, end)
	if err != nil {
		return ContextDefinition{}, err
	}
	return env.registerContextDefinition(definition)
}

// CreateCronTimeContext registers a fixed-schedule temporal context in env.
func CreateCronTimeContext(env *Environment, name string, start, end CronSchedule) (ContextDefinition, error) {
	if env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	definition, err := NewCronTimeContext(name, start, end)
	if err != nil {
		return ContextDefinition{}, err
	}
	return env.registerContextDefinition(definition)
}

// CreateNestedContext registers a segmented child context below a registered
// parent context. The child definition supplies the local key/hash/category
// partitioning rules; its name is replaced by name in the environment.
func CreateNestedContext(env *Environment, name, parentName string, child ContextDefinition) (ContextDefinition, error) {
	if env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "nil environment")
	}
	parent, ok := env.Context(parentName)
	if !ok {
		return ContextDefinition{}, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", parentName))
	}
	definition, err := NewNestedContext(name, parent, child)
	if err != nil {
		return ContextDefinition{}, err
	}
	return env.registerContextDefinition(definition)
}

func (e *Environment) registerContextDefinition(definition ContextDefinition) (ContextDefinition, error) {
	if definition.parent != nil {
		if definition.parent.name == "" {
			return ContextDefinition{}, NewError(ErrorInvalidRule, "nested context parent is required")
		}
		if definition.parent.name == definition.name {
			return ContextDefinition{}, NewError(ErrorInvalidRule, "nested context cannot reference itself")
		}
		if _, exists := e.Context(definition.parent.name); !exists {
			return ContextDefinition{}, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", definition.parent.name))
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.contexts[definition.name]; exists {
		return ContextDefinition{}, NewError(ErrorDependency, fmt.Sprintf("context %q is already registered", definition.name))
	}
	e.contexts[definition.name] = definition
	return definition, nil
}

func (e *Environment) removeContextDefinition(name string) error {
	if e == nil {
		return NewError(ErrorDependency, "nil environment")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.contexts[name]; !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", name))
	}
	for childName, child := range e.contexts {
		if childName != name && child.parent != nil && child.parent.name == name {
			return NewError(ErrorDependency, fmt.Sprintf("context %q is still a parent of context %q", name, childName))
		}
	}
	delete(e.contexts, name)
	for variableName, definition := range e.variables {
		if definition.context == name {
			delete(e.variables, variableName)
		}
	}
	return nil
}

func (e *Environment) Context(name string) (ContextDefinition, bool) {
	if e == nil {
		return ContextDefinition{}, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	definition, ok := e.contexts[name]
	return definition, ok
}

func (e *Environment) Contexts() []ContextDefinition {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]ContextDefinition, 0, len(e.contexts))
	for _, definition := range e.contexts {
		result = append(result, definition)
	}
	// Context names are the only stable ordering key; avoid depending on map
	// iteration in plan hashes and diagnostics.
	for left := 0; left < len(result); left++ {
		for right := left + 1; right < len(result); right++ {
			if result[right].name < result[left].name {
				result[left], result[right] = result[right], result[left]
			}
		}
	}
	return result
}
