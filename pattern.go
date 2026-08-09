package esper

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type patternStep struct {
	tag       string
	predicate Expression[bool]
}

type patternNodeKind uint8

const (
	patternEventNode patternNodeKind = iota
	patternSequenceNode
	patternAndNode
	patternOrNode
	patternNotNode
	patternMatchUntilNode
	patternUntilNode
	patternEveryNode
	patternWithinNode
	patternTimerIntervalNode
	patternTimerAtNode
	patternTimerScheduleNode
	patternTimerCronNode
)

// patternNode is the analyzable CEP expression tree.  A PatternStream keeps
// the source and lifecycle modifiers at the definition level, while this tree
// carries the event operators.  Keeping the tree explicit lets the runtime
// execute and inspect combinators without going through EPL text.
type patternNode struct {
	kind                   patternNodeKind
	tag                    string
	predicate              Expression[bool]
	source                 *streamNode
	consumeLevel           int
	consumeLevelSet        bool
	left                   *patternNode
	right                  *patternNode
	child                  *patternNode
	sequenceMax            int
	sequenceMaxSet         bool
	sequenceMaxExpr        Expr
	minimum                int
	maximum                int
	dynamicBounds          bool
	minimumExpr            Expr
	maximumExpr            Expr
	everyKey               *exprNode
	everyExpr              Expr
	duration               time.Duration
	durationExpr           Expr
	calendar               *OutputCalendarPeriod
	at                     time.Time
	schedule               []time.Time
	schedulePeriod         *PatternTimerPeriod
	scheduleAnchor         time.Time
	scheduleAnchorSet      bool
	scheduleIncludeAnchor  bool
	scheduleRepetitions    int64
	scheduleRepetitionsSet bool
	scheduleExpr           Expr
	cron                   *CronSchedule
	cronOneShot            bool
	distinctExpiry         time.Duration
	distinctExpirySet      bool
}

func patternEvent(tag string, predicate Expression[bool]) *patternNode {
	return &patternNode{kind: patternEventNode, tag: tag, predicate: predicate}
}

// clonePatternNode returns an independent pattern tree. PatternStream
// builders are value-like, so modifiers such as Consume must not mutate a
// branch that the caller may still reuse in another composed pattern.
func clonePatternNode(node *patternNode) *patternNode {
	if node == nil {
		return nil
	}
	copyNode := *node
	copyNode.left = clonePatternNode(node.left)
	copyNode.right = clonePatternNode(node.right)
	copyNode.child = clonePatternNode(node.child)
	copyNode.schedule = append([]time.Time(nil), node.schedule...)
	if node.schedulePeriod != nil {
		period := *node.schedulePeriod
		copyNode.schedulePeriod = &period
	}
	if node.calendar != nil {
		calendar := *node.calendar
		copyNode.calendar = &calendar
	}
	if node.cron != nil {
		cron := *node.cron
		copyNode.cron = &cron
	}
	return &copyNode
}

func lastPatternEvent(node *patternNode) *patternNode {
	if node == nil {
		return nil
	}
	switch node.kind {
	case patternEventNode:
		return node
	case patternSequenceNode, patternAndNode, patternOrNode:
		if event := lastPatternEvent(node.right); event != nil {
			return event
		}
		return lastPatternEvent(node.left)
	case patternUntilNode:
		if event := lastPatternEvent(node.right); event != nil {
			return event
		}
		return lastPatternEvent(node.child)
	case patternMatchUntilNode:
		if event := lastPatternEvent(node.right); event != nil {
			return event
		}
		return lastPatternEvent(node.child)
	case patternNotNode, patternEveryNode, patternWithinNode:
		return lastPatternEvent(node.child)
	default:
		return nil
	}
}

func (n *patternNode) description() string {
	if n == nil {
		return "<nil-pattern>"
	}
	switch n.kind {
	case patternEventNode:
		if n.predicate == nil {
			return n.tag + ":<nil>"
		}
		description := n.tag + ":" + n.predicate.Description()
		if n.consumeLevelSet {
			description += fmt.Sprintf(".consume(%d)", n.consumeLevel)
		}
		return description
	case patternSequenceNode:
		operator := "->"
		if n.sequenceMaxExpr != nil {
			operator = fmt.Sprintf("-[%s]>", n.sequenceMaxExpr.Description())
		} else if n.sequenceMaxSet {
			operator = fmt.Sprintf("-[%d]>", n.sequenceMax)
		}
		return "(" + n.left.description() + operator + n.right.description() + ")"
	case patternAndNode:
		return "(" + n.left.description() + " and " + n.right.description() + ")"
	case patternOrNode:
		return "(" + n.left.description() + " or " + n.right.description() + ")"
	case patternNotNode:
		return "not(" + n.child.description() + ")"
	case patternMatchUntilNode:
		minimum := fmt.Sprintf("%d", n.minimum)
		if n.minimumExpr != nil {
			minimum = n.minimumExpr.Description()
		}
		maximum := "*"
		if n.maximumExpr != nil {
			maximum = n.maximumExpr.Description()
		} else if n.maximum > 0 {
			maximum = fmt.Sprintf("%d", n.maximum)
		}
		child := n.child.description()
		if n.right != nil {
			child += ",until=" + n.right.description()
		}
		return fmt.Sprintf("match-until(%s:%s,%s)", minimum, maximum, child)
	case patternUntilNode:
		return "until(" + n.child.description() + "," + n.right.description() + ")"
	case patternEveryNode:
		if n.everyExpr != nil {
			description := "every-distinct(" + n.everyExpr.Description()
			if n.distinctExpirySet {
				description += "," + n.distinctExpiry.String()
			}
			return description + "," + n.child.description() + ")"
		}
		return "every(" + n.child.description() + ")"
	case patternWithinNode:
		duration := patternWithinDurationDescription(n)
		if n.maximum < 0 {
			return fmt.Sprintf("within(%s,%s)", duration, n.child.description())
		}
		return fmt.Sprintf("within-max(%s,%d,%s)", duration, n.maximum, n.child.description())
	case patternTimerIntervalNode:
		return "timer-interval(" + patternWithinDurationDescription(n) + ")"
	case patternTimerAtNode:
		return "timer-at(" + n.at.UTC().Format(time.RFC3339Nano) + ")"
	case patternTimerScheduleNode:
		if n.scheduleExpr != nil {
			return "timer-schedule-iso(" + n.scheduleExpr.Description() + ")"
		}
		if n.schedulePeriod != nil {
			start := "now"
			if n.scheduleAnchorSet {
				start = n.scheduleAnchor.UTC().Format(time.RFC3339Nano)
			}
			repetitions := "1"
			if n.scheduleRepetitionsSet {
				repetitions = fmt.Sprintf("%d", n.scheduleRepetitions)
			}
			return fmt.Sprintf("timer-schedule-period(%s,%s,%s,%t)", start, patternTimerPeriodDescription(*n.schedulePeriod), repetitions, n.scheduleIncludeAnchor)
		}
		parts := make([]string, 0, len(n.schedule))
		for _, at := range n.schedule {
			parts = append(parts, at.UTC().Format(time.RFC3339Nano))
		}
		return "timer-schedule(" + strings.Join(parts, ",") + ")"
	case patternTimerCronNode:
		if n.cron == nil {
			return "timer-cron(<nil>)"
		}
		if n.cronOneShot {
			return "timer-at-schedule(" + n.cron.description() + ")"
		}
		return "timer-cron(" + n.cron.description() + ")"
	default:
		return "<unknown-pattern>"
	}
}

type patternDefinition struct {
	input                  *streamNode
	inputs                 []*streamNode
	steps                  []patternStep
	root                   *patternNode
	sourceMismatch         bool
	every                  bool
	everyDistinct          Expr
	everyDistinctExpiry    time.Duration
	everyDistinctExpirySet bool
	maxStates              int
	within                 time.Duration
	guard                  Expression[bool]
	consumeInvalid         bool
}

// PatternStream is a fluent single-source CEP pattern builder. It deliberately
// keeps Pattern separate from Stream so the type-changing tag result does not
// turn the normal event stream API into a universal receiver.
type PatternStream struct {
	env *Environment
	def *patternDefinition
}

func PatternFrom[T any](stream Stream[T], tag string, predicate Expression[bool]) PatternStream {
	definition := &patternDefinition{input: stream.node, inputs: []*streamNode{stream.node}}
	if strings.TrimSpace(tag) != "" && predicate != nil {
		definition.steps = append(definition.steps, patternStep{tag: tag, predicate: predicate})
		definition.root = patternEvent(tag, predicate)
		definition.root.source = stream.node
	}
	return PatternStream{env: stream.env, def: definition}
}

// PatternFromRecord is the dynamic-source counterpart to PatternFrom. It is
// useful for Named Window and schema-driven sources whose Go event type is not
// available at the call site; the predicate is still analyzed against the
// source schema during Build.
func PatternFromRecord(stream RecordStream, tag string, predicate Expression[bool]) PatternStream {
	definition := &patternDefinition{input: stream.node, inputs: []*streamNode{stream.node}}
	if strings.TrimSpace(tag) != "" && predicate != nil {
		definition.steps = append(definition.steps, patternStep{tag: tag, predicate: predicate})
		definition.root = patternEvent(tag, predicate)
		definition.root.source = stream.node
	}
	return PatternStream{env: stream.env, def: definition}
}

// TimerInterval creates a source-marked timer observer. The stream supplies
// the environment and deployment source contract; timer ticks are driven by
// Engine.AdvanceTime and do not consume source events.
func TimerInterval[T any](stream Stream[T], interval time.Duration) PatternStream {
	return PatternStream{
		env: stream.env,
		def: &patternDefinition{
			input: stream.node,
			root:  &patternNode{kind: patternTimerIntervalNode, duration: interval},
		},
	}
}

// TimerIntervalExpr creates a timer observer whose interval is resolved when
// the observer branch is armed. The expression may reference captured Pattern
// tags, variables, or deployment substitution parameters.
func TimerIntervalExpr[T any](stream Stream[T], interval Expression[time.Duration]) PatternStream {
	var expression Expr
	if interval != nil {
		expression = interval
	}
	return PatternStream{
		env: stream.env,
		def: &patternDefinition{
			input: stream.node,
			root:  &patternNode{kind: patternTimerIntervalNode, durationExpr: expression},
		},
	}
}

// TimerIntervalCalendar creates a recurring calendar timer observer. Each
// occurrence advances with time.Time.AddDate, preserving month and year
// boundaries instead of approximating them with a fixed duration.
func TimerIntervalCalendar[T any](stream Stream[T], years, months, days int) PatternStream {
	return PatternStream{
		env: stream.env,
		def: &patternDefinition{
			input: stream.node,
			root: &patternNode{
				kind:     patternTimerIntervalNode,
				calendar: &OutputCalendarPeriod{Years: years, Months: months, Days: days},
			},
		},
	}
}

// PatternTimerPeriod describes a timer interval with an optional calendar
// component and a fixed-duration component. The calendar part is applied
// first with time.Time.AddDate and the fixed part is then added at nanosecond
// precision. This is the fluent counterpart of Esper time periods such as
// "1 month 10 msec".
type PatternTimerPeriod struct {
	Years         int
	Months        int
	Days          int
	FixedDuration time.Duration
}

// PatternTimerScheduleSpec describes a timer:schedule period form. StartAt is
// optional: when omitted, the current virtual time is the anchor. A zero
// Repetitions value means one occurrence, a positive value bounds the number
// of occurrences, and -1 means unlimited recurrence. IncludeStart controls
// whether StartAt itself is an occurrence; it is useful for matching the
// Java date/period and R/date/period forms precisely.
type PatternTimerScheduleSpec struct {
	StartAt      *time.Time
	Period       PatternTimerPeriod
	Repetitions  int64
	IncludeStart bool
}

// TimerIntervalPeriod creates a recurring timer observer for a mixed calendar
// and fixed-duration period. A period must contain at least one positive
// component; negative components are rejected during Build.
func TimerIntervalPeriod[T any](stream Stream[T], period PatternTimerPeriod) PatternStream {
	var calendar *OutputCalendarPeriod
	if period.Years != 0 || period.Months != 0 || period.Days != 0 {
		calendar = &OutputCalendarPeriod{Years: period.Years, Months: period.Months, Days: period.Days}
	}
	return PatternStream{
		env: stream.env,
		def: &patternDefinition{
			input: stream.node,
			root: &patternNode{
				kind:     patternTimerIntervalNode,
				duration: period.FixedDuration,
				calendar: calendar,
			},
		},
	}
}

// TimerScheduleWithPeriod creates the typed, recurring/one-shot counterpart
// of Esper timer:schedule(period/date/repetitions). It keeps schedule data in
// the Go AST instead of requiring an EPL fragment.
func TimerScheduleWithPeriod[T any](stream Stream[T], spec PatternTimerScheduleSpec) PatternStream {
	period := spec.Period
	node := &patternNode{
		kind:                   patternTimerScheduleNode,
		schedulePeriod:         &period,
		scheduleIncludeAnchor:  spec.IncludeStart,
		scheduleRepetitions:    spec.Repetitions,
		scheduleRepetitionsSet: true,
	}
	if node.scheduleRepetitions == 0 {
		node.scheduleRepetitions = 1
	}
	if spec.StartAt != nil {
		node.scheduleAnchor = *spec.StartAt
		node.scheduleAnchorSet = true
	}
	return PatternStream{
		env: stream.env,
		def: &patternDefinition{input: stream.node, root: node},
	}
}

// TimerScheduleISO creates a schedule observer from an ISO-8601 schedule
// value. The typed TimerScheduleWithPeriod API is preferred for new Go rules;
// this adapter preserves Java timer:schedule ISO forms for migration and
// comparison tests.
func TimerScheduleISO[T any](stream Stream[T], iso string) PatternStream {
	return TimerScheduleISOExpr(stream, Literal[string](iso))
}

// TimerScheduleISOExpr evaluates and parses an ISO-8601 schedule when the
// observer branch is armed, allowing a captured Pattern tag to calculate the
// schedule just like Esper's timer:schedule(iso: expression).
func TimerScheduleISOExpr[T any](stream Stream[T], iso Expression[string]) PatternStream {
	var expression Expr
	if iso != nil {
		expression = iso
	}
	return PatternStream{
		env: stream.env,
		def: &patternDefinition{
			input: stream.node,
			root:  &patternNode{kind: patternTimerScheduleNode, scheduleExpr: expression},
		},
	}
}

// TimerAt creates a one-shot timer observer driven by Engine.AdvanceTime.
func TimerAt[T any](stream Stream[T], at time.Time) PatternStream {
	return PatternStream{
		env: stream.env,
		def: &patternDefinition{
			input: stream.node,
			root:  &patternNode{kind: patternTimerAtNode, at: at},
		},
	}
}

// TimerSchedule creates a deterministic schedule observer. Each supplied
// instant emits once when the engine's virtual clock reaches it; a single
// clock jump may therefore produce multiple result rows in schedule order.
// Cron/calendar recurrence is intentionally a separate capability and is not
// implied by this explicit-instant API.
func TimerSchedule[T any](stream Stream[T], times ...time.Time) PatternStream {
	schedule := append([]time.Time(nil), times...)
	sort.Slice(schedule, func(left, right int) bool { return schedule[left].Before(schedule[right]) })
	return PatternStream{
		env: stream.env,
		def: &patternDefinition{
			input: stream.node,
			root:  &patternNode{kind: patternTimerScheduleNode, schedule: schedule},
		},
	}
}

// TimerCron creates a recurring calendar observer.  The schedule is evaluated
// when the statement is instantiated and all due occurrences are emitted in
// order when Engine.AdvanceTime reaches or passes them.
func TimerCron[T any](stream Stream[T], schedule CronSchedule) PatternStream {
	copySchedule := schedule
	return PatternStream{
		env: stream.env,
		def: &patternDefinition{
			input: stream.node,
			root:  &patternNode{kind: patternTimerCronNode, cron: &copySchedule},
		},
	}
}

// TimerAtSchedule creates a one-shot calendar observer. It resolves the
// first CronSchedule occurrence strictly after the statement/context start
// time, then stops after that occurrence. This is the chainable counterpart
// of Esper's timer:at calendar form; use TimerCron for recurring occurrences.
func TimerAtSchedule[T any](stream Stream[T], schedule CronSchedule) PatternStream {
	copySchedule := schedule
	return PatternStream{
		env: stream.env,
		def: &patternDefinition{
			input: stream.node,
			root:  &patternNode{kind: patternTimerCronNode, cron: &copySchedule, cronOneShot: true},
		},
	}
}

func (p PatternStream) FollowedBy(tag string, predicate Expression[bool]) PatternStream {
	return p.followedBy(0, false, tag, predicate)
}

// FollowedByMax limits the number of concurrent matches waiting on the right
// side of this followed-by edge. It is the fluent counterpart of Esper's
// `-[N]>` operator and is scoped to this edge, rather than to the whole
// pattern. A positive maximum is required and is checked during Build.
func (p PatternStream) FollowedByMax(maximum int, tag string, predicate Expression[bool]) PatternStream {
	return p.followedBy(maximum, true, tag, predicate)
}

// FollowedByMaxExpr applies a deployment/variable expression as the maximum
// number of active matches waiting on this followed-by edge. The expression
// is evaluated when the left side completes and must resolve to a positive
// integer. This is the Go-style counterpart of Esper's variable-backed
// followed-by maximum.
func (p PatternStream) FollowedByMaxExpr(maximum Expression[int], tag string, predicate Expression[bool]) PatternStream {
	return p.followedByExpression(maximum, tag, predicate)
}

// Consume marks the most recently appended event filter with an event-level
// consumption priority. When multiple filters in the same Pattern query match
// one input Event, only consuming filters at the highest level receive that
// Event; unannotated filters are excluded whenever a consuming filter matches.
// If no level is supplied, the level is zero, matching Esper's bare @consume
// annotation. Explicit Consume(0) is also a consuming annotation and is not
// equivalent to leaving a filter unannotated.
func (p PatternStream) Consume(levels ...int) PatternStream {
	if p.def == nil || p.def.root == nil {
		return p
	}
	copyDefinition := *p.def
	copyDefinition.steps = append([]patternStep(nil), p.def.steps...)
	copyDefinition.root = clonePatternNode(p.def.root)
	if len(levels) > 1 {
		copyDefinition.consumeInvalid = true
		return PatternStream{env: p.env, def: &copyDefinition}
	}
	level := 0
	if len(levels) == 1 {
		level = levels[0]
	}
	if event := lastPatternEvent(copyDefinition.root); event != nil {
		event.consumeLevel = level
		event.consumeLevelSet = true
	} else {
		copyDefinition.consumeInvalid = true
	}
	return PatternStream{env: p.env, def: &copyDefinition}
}

func (p PatternStream) followedBy(maximum int, maximumSet bool, tag string, predicate Expression[bool]) PatternStream {
	var maximumExpr Expr
	return p.followedByWithMaximum(maximum, maximumSet, maximumExpr, tag, predicate)
}

func (p PatternStream) followedByExpression(maximum Expression[int], tag string, predicate Expression[bool]) PatternStream {
	var maximumExpr Expr
	if maximum != nil {
		maximumExpr = maximum
	}
	return p.followedByWithMaximum(0, true, maximumExpr, tag, predicate)
}

func (p PatternStream) followedByWithMaximum(maximum int, maximumSet bool, maximumExpr Expr, tag string, predicate Expression[bool]) PatternStream {
	if p.def == nil {
		return p
	}
	copyDefinition := *p.def
	copyDefinition.steps = append(append([]patternStep(nil), p.def.steps...), patternStep{tag: tag, predicate: predicate})
	event := patternEvent(tag, predicate)
	event.source = copyDefinition.input
	copyDefinition.root = &patternNode{
		kind:            patternSequenceNode,
		left:            p.def.root,
		right:           event,
		sequenceMax:     maximum,
		sequenceMaxSet:  maximumSet,
		sequenceMaxExpr: maximumExpr,
	}
	p.def = &copyDefinition
	return p
}

// Then chains an arbitrary PatternStream branch after the current branch.
// It is the fluent Go equivalent of an observer-capable followed-by: unlike
// FollowedBy, the right side may be a timer, event, or another combinator.
func (p PatternStream) Then(other PatternStream) PatternStream {
	if p.def == nil {
		return p
	}
	copyDefinition := *p.def
	copyDefinition.steps = nil
	copyDefinition.every = false
	copyDefinition.everyDistinct = nil
	copyDefinition.everyDistinctExpiry = 0
	copyDefinition.everyDistinctExpirySet = false
	copyDefinition.root = &patternNode{
		kind:  patternSequenceNode,
		left:  patternBranchRoot(p.def),
		right: patternBranchRoot(other.def),
	}
	if other.def == nil || p.env != other.env {
		copyDefinition.sourceMismatch = true
	}
	copyDefinition.inputs = mergePatternInputs(p.def, other.def)
	return PatternStream{env: p.env, def: &copyDefinition}
}

func (p PatternStream) Every() PatternStream {
	if p.def == nil {
		return p
	}
	copyDefinition := *p.def
	if p.def.root != nil && p.def.root.kind == patternWithinNode && !p.def.every && p.def.everyDistinct == nil {
		copyDefinition.steps = nil
		copyDefinition.root = &patternNode{kind: patternEveryNode, child: p.def.root}
		return PatternStream{env: p.env, def: &copyDefinition}
	}
	copyDefinition.every = true
	copyDefinition.steps = append([]patternStep(nil), p.def.steps...)
	copyDefinition.root = p.def.root
	p.def = &copyDefinition
	return p
}

func (p PatternStream) EveryDistinct(key Expr) PatternStream {
	if p.def == nil {
		return p
	}
	copyDefinition := *p.def
	if p.def.root != nil && p.def.root.kind == patternWithinNode && !p.def.every && p.def.everyDistinct == nil {
		copyDefinition.steps = nil
		copyDefinition.root = &patternNode{kind: patternEveryNode, child: p.def.root, everyExpr: key}
		return PatternStream{env: p.env, def: &copyDefinition}
	}
	copyDefinition.every = true
	copyDefinition.everyDistinct = key
	copyDefinition.steps = append([]patternStep(nil), p.def.steps...)
	copyDefinition.root = p.def.root
	return PatternStream{env: p.env, def: &copyDefinition}
}

// EveryDistinctFor limits the lifetime of a distinct key. Once expiry has
// elapsed on the engine's virtual clock, a later event with the same key may
// start a new match. This mirrors Esper's optional expiry interval while
// keeping the primary API expression-oriented.
func (p PatternStream) EveryDistinctFor(key Expr, expiry time.Duration) PatternStream {
	if p.def == nil {
		return p
	}
	if p.def.root != nil && p.def.root.kind == patternWithinNode && !p.def.every && p.def.everyDistinct == nil {
		copyDefinition := *p.def
		copyDefinition.steps = nil
		copyDefinition.within = 0
		copyDefinition.every = false
		copyDefinition.everyDistinct = nil
		copyDefinition.everyDistinctExpiry = 0
		copyDefinition.everyDistinctExpirySet = false
		copyDefinition.root = &patternNode{
			kind:              patternEveryNode,
			child:             p.def.root,
			everyExpr:         key,
			distinctExpiry:    expiry,
			distinctExpirySet: true,
		}
		return PatternStream{env: p.env, def: &copyDefinition}
	}
	copyDefinition := *p.def
	copyDefinition.every = true
	copyDefinition.everyDistinct = key
	copyDefinition.everyDistinctExpiry = expiry
	copyDefinition.everyDistinctExpirySet = true
	copyDefinition.steps = append([]patternStep(nil), p.def.steps...)
	copyDefinition.root = p.def.root
	return PatternStream{env: p.env, def: &copyDefinition}
}

// MaxStates bounds the number of active followed-by matches. It is both a
// correctness guard for malformed/high-cardinality patterns and an explicit
// resource contract for untrusted event streams.
func (p PatternStream) MaxStates(max int) PatternStream {
	if p.def == nil {
		return p
	}
	copyDefinition := *p.def
	copyDefinition.maxStates = max
	copyDefinition.steps = append([]patternStep(nil), p.def.steps...)
	copyDefinition.root = p.def.root
	return PatternStream{env: p.env, def: &copyDefinition}
}

func (p PatternStream) Within(duration time.Duration) PatternStream {
	return p.withinNode(duration, nil, nil, -1)
}

// WithinExpr applies a timer guard whose fixed duration expression is
// resolved when the guard branch is armed. Use DurationSeconds or
// DurationMilliseconds to convert an event/tag/parameter numeric value into
// time.Duration while keeping the rule analyzable.
func (p PatternStream) WithinExpr(duration Expression[time.Duration]) PatternStream {
	var expression Expr
	if duration != nil {
		expression = duration
	}
	return p.withinNode(0, expression, nil, -1)
}

// WithinCalendar applies a calendar-aware timer guard. The deadline uses
// time.Time.AddDate, preserving month and year boundaries instead of
// approximating them with a fixed number of hours.
func (p PatternStream) WithinCalendar(years, months, days int) PatternStream {
	return p.withinNode(0, nil, &OutputCalendarPeriod{Years: years, Months: months, Days: days}, -1)
}

func (p PatternStream) withinNode(duration time.Duration, durationExpr Expr, calendar *OutputCalendarPeriod, maximum int) PatternStream {
	if p.def == nil {
		return p
	}
	copyDefinition := *p.def
	copyDefinition.steps = nil
	copyDefinition.within = 0
	copyDefinition.every = false
	copyDefinition.everyDistinct = nil
	copyDefinition.everyDistinctExpiry = 0
	copyDefinition.everyDistinctExpirySet = false
	child := p.def.root
	if p.def.every || p.def.everyDistinct != nil {
		child = &patternNode{kind: patternEveryNode, child: child, everyExpr: p.def.everyDistinct}
	}
	var calendarCopy *OutputCalendarPeriod
	if calendar != nil {
		copy := *calendar
		calendarCopy = &copy
	}
	copyDefinition.root = &patternNode{
		kind:         patternWithinNode,
		child:        child,
		duration:     duration,
		durationExpr: durationExpr,
		calendar:     calendarCopy,
		maximum:      maximum,
	}
	return PatternStream{env: p.env, def: &copyDefinition}
}

// WithinOrMax applies a timer guard to this pattern branch and stops it after
// maximum completions. A maximum of zero suppresses all completions; the
// duration still bounds the guard lifetime. This is the fluent Go equivalent
// of Esper's timer:withinmax guard and remains scoped when the branch is later
// composed with Then, And, Or, Until, or another PatternStream operator.
func (p PatternStream) WithinOrMax(duration time.Duration, maximum int) PatternStream {
	return p.withinNode(duration, nil, nil, maximum)
}

// WithinOrMaxExpr is the dynamic-duration form of WithinOrMax.
func (p PatternStream) WithinOrMaxExpr(duration Expression[time.Duration], maximum int) PatternStream {
	var expression Expr
	if duration != nil {
		expression = duration
	}
	return p.withinNode(0, expression, nil, maximum)
}

// WithinOrMaxCalendar applies a calendar deadline and a completion limit.
func (p PatternStream) WithinOrMaxCalendar(years, months, days, maximum int) PatternStream {
	return p.withinNode(0, nil, &OutputCalendarPeriod{Years: years, Months: months, Days: days}, maximum)
}

func patternWithinDurationDescription(node *patternNode) string {
	if node == nil {
		return "<nil-duration>"
	}
	if node.calendar != nil && node.duration > 0 {
		return fmt.Sprintf("calendar(%dY%dM%dD)+%s", node.calendar.Years, node.calendar.Months, node.calendar.Days, node.duration)
	}
	if node.durationExpr != nil {
		return node.durationExpr.Description()
	}
	if node.calendar != nil {
		return fmt.Sprintf("calendar(%dY%dM%dD)", node.calendar.Years, node.calendar.Months, node.calendar.Days)
	}
	return node.duration.String()
}

// While applies an event-local guard to the pattern. A false or non-boolean
// guard clears active matches before the current event is considered. Custom
// stateful guards and observer scheduling remain separate extension points.
func (p PatternStream) While(guard Expression[bool]) PatternStream {
	if p.def == nil {
		return p
	}
	copyDefinition := *p.def
	copyDefinition.guard = guard
	copyDefinition.steps = append([]patternStep(nil), p.def.steps...)
	copyDefinition.root = p.def.root
	return PatternStream{env: p.env, def: &copyDefinition}
}

// And starts both patterns at the same logical time and completes when both
// branches have matched.  The two branches currently share one source; this
// keeps event routing explicit and leaves multi-source CEP for the dedicated
// join/dataflow APIs.
func (p PatternStream) And(other PatternStream) PatternStream {
	return p.combine(other, patternAndNode)
}

// Or completes when either branch matches.  If both branches match the same
// event, both branch results are retained, matching the event-oriented
// fan-out model used by the runtime.
func (p PatternStream) Or(other PatternStream) PatternStream {
	return p.combine(other, patternOrNode)
}

// Not creates a negative branch.  A negative branch is intended to be used
// with And, for example A.And(B.Not()). A root-only negative pattern has no
// positive completion and is rejected during Build.
func (p PatternStream) Not() PatternStream {
	if p.def == nil {
		return p
	}
	copyDefinition := *p.def
	copyDefinition.steps = nil
	copyDefinition.every = false
	copyDefinition.everyDistinct = nil
	copyDefinition.everyDistinctExpiry = 0
	copyDefinition.everyDistinctExpirySet = false
	copyDefinition.root = &patternNode{kind: patternNotNode, child: patternBranchRoot(p.def)}
	return PatternStream{env: p.env, def: &copyDefinition}
}

// MatchUntil repeats a child pattern until at least minimum matches have
// completed and at most maximum matches are accepted. maximum <= 0 means
// unbounded. The repeated tag resolves to the latest event through TagField;
// TagCount exposes the full repetition count to projections.
func (p PatternStream) MatchUntil(minimum, maximum int) PatternStream {
	if p.def == nil {
		return p
	}
	copyDefinition := *p.def
	copyDefinition.steps = nil
	copyDefinition.every = false
	copyDefinition.everyDistinct = nil
	copyDefinition.everyDistinctExpiry = 0
	copyDefinition.everyDistinctExpirySet = false
	copyDefinition.root = &patternNode{kind: patternMatchUntilNode, child: patternBranchRoot(p.def), minimum: minimum, maximum: maximum}
	return PatternStream{env: p.env, def: &copyDefinition}
}

// MatchUntilExpr applies dynamic lower and upper bounds to a repeated
// pattern branch. A nil minimum means zero and a nil maximum means
// unbounded. Bounds are resolved when the branch is first armed, so they can
// read registered variables, deployment parameters, and captured outer tags.
// Runtime values must be non-negative and maximum must be at least minimum.
func (p PatternStream) MatchUntilExpr(minimum, maximum Expression[int]) PatternStream {
	if p.def == nil {
		return p
	}
	copyDefinition := *p.def
	copyDefinition.steps = nil
	copyDefinition.every = false
	copyDefinition.everyDistinct = nil
	copyDefinition.everyDistinctExpiry = 0
	copyDefinition.everyDistinctExpirySet = false
	var minimumExpr, maximumExpr Expr
	if minimum != nil {
		minimumExpr = minimum
	}
	if maximum != nil {
		maximumExpr = maximum
	}
	copyDefinition.root = &patternNode{
		kind:          patternMatchUntilNode,
		child:         patternBranchRoot(p.def),
		dynamicBounds: true,
		minimumExpr:   minimumExpr,
		maximumExpr:   maximumExpr,
	}
	return PatternStream{env: p.env, def: &copyDefinition}
}

// Until repeats the left pattern while independently watching the right
// pattern as a terminator. A completed terminator emits one match containing
// the repeated left-tag values and the terminator tags.
func (p PatternStream) Until(terminator PatternStream) PatternStream {
	if p.def == nil {
		return p
	}
	copyDefinition := *p.def
	copyDefinition.steps = nil
	copyDefinition.every = false
	copyDefinition.everyDistinct = nil
	copyDefinition.everyDistinctExpiry = 0
	copyDefinition.everyDistinctExpirySet = false
	if p.def.root != nil && p.def.root.kind == patternMatchUntilNode && p.def.root.right == nil {
		copyDefinition.root = clonePatternNode(p.def.root)
		copyDefinition.root.right = patternBranchRoot(terminator.def)
		if terminator.def == nil || p.env != terminator.env {
			copyDefinition.sourceMismatch = true
		}
		copyDefinition.inputs = mergePatternInputs(p.def, terminator.def)
		return PatternStream{env: p.env, def: &copyDefinition}
	}
	copyDefinition.root = &patternNode{kind: patternUntilNode, child: patternBranchRoot(p.def), right: patternBranchRoot(terminator.def)}
	if terminator.def == nil || p.env != terminator.env {
		copyDefinition.sourceMismatch = true
	}
	copyDefinition.inputs = mergePatternInputs(p.def, terminator.def)
	return PatternStream{env: p.env, def: &copyDefinition}
}

// Repeat is an expressive alias for MatchUntil.
func (p PatternStream) Repeat(minimum, maximum int) PatternStream {
	return p.MatchUntil(minimum, maximum)
}

func (p PatternStream) combine(other PatternStream, kind patternNodeKind) PatternStream {
	if p.def == nil {
		return p
	}
	copyDefinition := *p.def
	copyDefinition.steps = nil
	copyDefinition.every = false
	copyDefinition.everyDistinct = nil
	copyDefinition.everyDistinctExpiry = 0
	copyDefinition.everyDistinctExpirySet = false
	copyDefinition.root = &patternNode{kind: kind, left: patternBranchRoot(p.def), right: patternBranchRoot(other.def)}
	if other.def == nil || p.env != other.env {
		copyDefinition.sourceMismatch = true
	}
	copyDefinition.inputs = mergePatternInputs(p.def, other.def)
	return PatternStream{env: p.env, def: &copyDefinition}
}

func mergePatternInputs(left, right *patternDefinition) []*streamNode {
	result := make([]*streamNode, 0)
	appendInputs := func(definition *patternDefinition) {
		if definition == nil {
			return
		}
		inputs := definition.inputs
		if len(inputs) == 0 && definition.input != nil {
			inputs = []*streamNode{definition.input}
		}
		for _, input := range inputs {
			if input == nil {
				continue
			}
			seen := false
			for _, existing := range result {
				if existing == input {
					seen = true
					break
				}
			}
			if !seen {
				result = append(result, input)
			}
		}
	}
	appendInputs(left)
	appendInputs(right)
	return result
}

func patternDefinitionInputs(definition *patternDefinition) []*streamNode {
	if definition == nil {
		return nil
	}
	if len(definition.inputs) > 0 {
		return append([]*streamNode(nil), definition.inputs...)
	}
	if definition.input != nil {
		return []*streamNode{definition.input}
	}
	return nil
}

func patternNodeTagNames(node *patternNode, result *[]string, seen map[string]struct{}) {
	if node == nil {
		return
	}
	if node.kind == patternEventNode {
		if _, exists := seen[node.tag]; !exists && strings.TrimSpace(node.tag) != "" {
			seen[node.tag] = struct{}{}
			*result = append(*result, node.tag)
		}
	}
	patternNodeTagNames(node.left, result, seen)
	patternNodeTagNames(node.right, result, seen)
	patternNodeTagNames(node.child, result, seen)
}

func patternDefinitionTagNames(definition *patternDefinition) []string {
	if definition == nil {
		return nil
	}
	result := make([]string, 0)
	seen := make(map[string]struct{})
	patternNodeTagNames(definition.root, &result, seen)
	for _, step := range definition.steps {
		if strings.TrimSpace(step.tag) != "" {
			if _, exists := seen[step.tag]; !exists {
				seen[step.tag] = struct{}{}
				result = append(result, step.tag)
			}
		}
	}
	return result
}

func patternBranchRoot(definition *patternDefinition) *patternNode {
	if definition == nil {
		return nil
	}
	root := definition.root
	if definition.every || definition.everyDistinct != nil {
		return &patternNode{kind: patternEveryNode, child: root, everyExpr: definition.everyDistinct}
	}
	return root
}

func (p PatternStream) root() *patternNode {
	if p.def == nil {
		return nil
	}
	return p.def.root
}

func (p PatternStream) Select(selections ...Selection) PatternQuery {
	return PatternQuery{env: p.env, definition: p.def, selections: append([]Selection(nil), selections...)}
}

func (p PatternStream) Query(options ...QueryOption) Query {
	return p.Select().Query(options...)
}

type PatternQuery struct {
	env        *Environment
	definition *patternDefinition
	selections []Selection
}

func (p PatternQuery) Query(options ...QueryOption) Query {
	spec := querySpec{selector: SelectIStream}
	for _, option := range options {
		if option != nil {
			option(&spec)
		}
	}
	return Query{
		env:                        p.env,
		input:                      p.definitionInput(),
		pattern:                    p.definition,
		patternSelections:          append([]Selection(nil), p.selections...),
		routeTarget:                spec.routeTarget,
		name:                       spec.name,
		statementUserObject:        spec.statementUserObject,
		statementMetadata:          cloneStatementMetadata(spec.statementMetadata),
		selector:                   spec.selector,
		sink:                       spec.sink,
		contextName:                spec.contextName,
		output:                     spec.output,
		distinct:                   spec.distinct,
		discardPartialsOnMatch:     spec.discardPartialsOnMatch,
		suppressOverlappingMatches: spec.suppressOverlappingMatches,
		orderBy:                    append([]SortKey(nil), spec.orderBy...),
		limit:                      spec.limit,
		offset:                     spec.offset,
		statementPriority:          spec.statementPriority,
		statementPrioritySet:       spec.statementPrioritySet,
		statementDrop:              spec.statementDrop,
		subscriberDisallowed:       spec.subscriberDisallowed,
	}
}

// InsertInto routes each completed pattern match's new-stream projection into
// a registered event type.
func (p PatternQuery) InsertInto(eventType string, options ...QueryOption) Query {
	options = append(append([]QueryOption(nil), options...), RouteTo(eventType))
	return p.Query(options...)
}

func (p PatternQuery) definitionInput() *streamNode {
	if p.definition == nil {
		return nil
	}
	return p.definition.input
}

func (p patternDefinition) description() string {
	name := "pattern(" + p.root.description() + ")"
	if p.root == nil {
		parts := make([]string, 0, len(p.steps))
		for _, step := range p.steps {
			parts = append(parts, step.tag+":"+step.predicate.Description())
		}
		name = "pattern(" + strings.Join(parts, "->") + ")"
	}
	if p.every {
		name += ".every()"
	}
	if p.within > 0 {
		name += ".within(" + p.within.String() + ")"
	}
	if p.guard != nil {
		name += ".while(" + p.guard.Description() + ")"
	}
	if p.everyDistinct != nil {
		name += ".every-distinct(" + p.everyDistinct.Description()
		if p.everyDistinctExpirySet {
			name += "," + p.everyDistinctExpiry.String()
		}
		name += ")"
	}
	if p.maxStates > 0 {
		name += fmt.Sprintf(".max-states(%d)", p.maxStates)
	}
	return name
}

func validatePattern(definition *patternDefinition) error {
	if definition == nil || definition.input == nil {
		return NewError(ErrorInvalidRule, "pattern requires a source")
	}
	if definition.root == nil && len(definition.steps) == 0 {
		return NewError(ErrorInvalidRule, "pattern requires at least one step")
	}
	if definition.within < 0 {
		return NewError(ErrorInvalidRule, "pattern within duration cannot be negative")
	}
	if definition.everyDistinct != nil && !definition.every {
		return NewError(ErrorInvalidRule, "every-distinct requires every")
	}
	if definition.everyDistinctExpirySet && definition.everyDistinctExpiry <= 0 {
		return NewError(ErrorInvalidRule, "every-distinct expiry must be positive")
	}
	if definition.maxStates < 0 {
		return NewError(ErrorInvalidRule, "pattern max states cannot be negative")
	}
	if definition.guard != nil && definition.guard.Type() != typeOf[bool]() {
		return NewError(ErrorTypeMismatch, "pattern while guard must return bool")
	}
	if definition.sourceMismatch {
		return NewError(ErrorDependency, "pattern combinators require the same environment and source")
	}
	if definition.consumeInvalid {
		return NewError(ErrorInvalidRule, "pattern consume requires an event filter")
	}
	if definition.root == nil {
		return NewError(ErrorInvalidRule, "pattern requires a pattern expression")
	}
	if definition.root.kind == patternNotNode {
		return NewError(ErrorInvalidRule, "root negative pattern has no positive completion")
	}
	seen := make(map[string]struct{})
	if err := validatePatternNode(definition.root, seen); err != nil {
		return err
	}
	return nil
}

func validatePatternNode(node *patternNode, seen map[string]struct{}) error {
	return validatePatternNodeScope(node, seen, false)
}

func validatePatternNodeScope(node *patternNode, seen map[string]struct{}, allowDuplicate bool) error {
	if node == nil {
		return NewError(ErrorInvalidRule, "pattern expression cannot be nil")
	}
	switch node.kind {
	case patternEventNode:
		if strings.TrimSpace(node.tag) == "" || node.predicate == nil {
			return NewError(ErrorInvalidRule, "pattern event requires a tag and predicate")
		}
		if node.consumeLevelSet && node.consumeLevel < 0 {
			return NewError(ErrorInvalidRule, "pattern consume level cannot be negative")
		}
		if _, exists := seen[node.tag]; exists && !allowDuplicate {
			return NewError(ErrorInvalidRule, fmt.Sprintf("pattern duplicates tag %q", node.tag))
		}
		seen[node.tag] = struct{}{}
	case patternSequenceNode:
		if node.sequenceMaxExpr != nil && node.sequenceMaxExpr.Type() != typeOf[int]() {
			return NewError(ErrorTypeMismatch, "followed-by maximum expression must return int")
		}
		if node.sequenceMaxExpr == nil && node.sequenceMaxSet && node.sequenceMax <= 0 {
			return NewError(ErrorInvalidRule, "followed-by maximum must be positive")
		}
		if err := validatePatternNodeScope(node.left, seen, false); err != nil {
			return err
		}
		if err := validatePatternNodeScope(node.right, seen, false); err != nil {
			return err
		}
	case patternAndNode, patternOrNode:
		// Esper permits sibling branches to reuse a tag name; the latest
		// sibling value wins for scalar lookup while TagValues retains all
		// captures. Duplicates within one branch remain invalid.
		leftSeen := clonePatternTagSet(seen)
		rightSeen := clonePatternTagSet(seen)
		if err := validatePatternNodeScope(node.left, leftSeen, false); err != nil {
			return err
		}
		if err := validatePatternNodeScope(node.right, rightSeen, false); err != nil {
			return err
		}
		for tag := range leftSeen {
			seen[tag] = struct{}{}
		}
		for tag := range rightSeen {
			seen[tag] = struct{}{}
		}
	case patternNotNode:
		return validatePatternNodeScope(node.child, seen, false)
	case patternMatchUntilNode:
		if node.minimumExpr != nil && node.minimumExpr.Type() != typeOf[int]() {
			return NewError(ErrorTypeMismatch, "match-until minimum expression must return int")
		}
		if node.maximumExpr != nil && node.maximumExpr.Type() != typeOf[int]() {
			return NewError(ErrorTypeMismatch, "match-until maximum expression must return int")
		}
		if !node.dynamicBounds && node.minimum <= 0 {
			return NewError(ErrorInvalidRule, "match-until minimum must be positive")
		}
		if !node.dynamicBounds && node.maximum > 0 && node.maximum < node.minimum {
			return NewError(ErrorInvalidRule, "match-until maximum must be at least minimum")
		}
		if err := validatePatternNodeScope(node.child, seen, false); err != nil {
			return err
		}
		if node.right != nil {
			return validatePatternNodeScope(node.right, seen, false)
		}
		return nil
	case patternUntilNode:
		if err := validatePatternNodeScope(node.child, seen, false); err != nil {
			return err
		}
		return validatePatternNodeScope(node.right, seen, false)
	case patternEveryNode:
		if node.everyExpr != nil && node.everyExpr.Type() == nil {
			return NewError(ErrorInvalidRule, "every-distinct key expression is invalid")
		}
		if node.distinctExpirySet && node.distinctExpiry <= 0 {
			return NewError(ErrorInvalidRule, "every-distinct expiry must be positive")
		}
		if node.distinctExpirySet && node.everyExpr == nil {
			return NewError(ErrorInvalidRule, "every-distinct expiry requires a key expression")
		}
		return validatePatternNodeScope(node.child, seen, false)
	case patternWithinNode:
		durationForms := 0
		if node.duration > 0 {
			durationForms++
		}
		if node.durationExpr != nil {
			durationForms++
			if node.durationExpr.Type() != typeOf[time.Duration]() {
				return NewError(ErrorTypeMismatch, "pattern within duration expression must return time.Duration")
			}
		}
		if node.calendar != nil {
			durationForms++
			if node.calendar.Years < 0 || node.calendar.Months < 0 || node.calendar.Days < 0 {
				return NewError(ErrorInvalidRule, "pattern within calendar duration cannot be negative")
			}
			if node.calendar.Years == 0 && node.calendar.Months == 0 && node.calendar.Days == 0 {
				return NewError(ErrorInvalidRule, "pattern within calendar duration must be positive")
			}
		}
		if durationForms != 1 {
			return NewError(ErrorInvalidRule, "pattern within requires exactly one positive duration form")
		}
		if node.maximum < -1 {
			return NewError(ErrorInvalidRule, "pattern within-or-max maximum cannot be negative")
		}
		return validatePatternNodeScope(node.child, seen, false)
	case patternTimerIntervalNode:
		durationForms := 0
		if node.duration < 0 {
			return NewError(ErrorInvalidRule, "timer interval fixed duration cannot be negative")
		}
		if node.duration > 0 {
			durationForms++
		}
		if node.durationExpr != nil {
			durationForms++
			if node.durationExpr.Type() != typeOf[time.Duration]() {
				return NewError(ErrorTypeMismatch, "timer interval duration expression must return time.Duration")
			}
		}
		if node.calendar != nil {
			durationForms++
			if node.calendar.Years < 0 || node.calendar.Months < 0 || node.calendar.Days < 0 {
				return NewError(ErrorInvalidRule, "timer interval calendar duration cannot be negative")
			}
			if node.calendar.Years == 0 && node.calendar.Months == 0 && node.calendar.Days == 0 {
				return NewError(ErrorInvalidRule, "timer interval calendar duration must be positive")
			}
		}
		if durationForms == 2 && node.duration > 0 && node.calendar != nil {
			return nil
		}
		if durationForms != 1 {
			return NewError(ErrorInvalidRule, "timer interval requires exactly one positive duration form")
		}
	case patternTimerAtNode:
		if node.at.IsZero() {
			return NewError(ErrorInvalidRule, "timer-at time is required")
		}
	case patternTimerScheduleNode:
		if node.scheduleExpr != nil {
			if node.scheduleExpr.Type() != typeOf[string]() {
				return NewError(ErrorTypeMismatch, "timer schedule ISO expression must return string")
			}
			if value := node.scheduleExpr.eval(EvalContext{}); value.IsPresent() {
				iso, ok := value.Any().(string)
				if !ok {
					return NewError(ErrorTypeMismatch, "timer schedule ISO expression must return string")
				}
				if _, err := parsePatternTimerScheduleISO(iso); err != nil {
					return NewError(ErrorInvalidRule, "invalid timer schedule ISO expression: "+err.Error())
				}
			}
			break
		}
		if node.schedulePeriod != nil {
			if node.scheduleAnchorSet && node.scheduleAnchor.IsZero() {
				return NewError(ErrorInvalidRule, "timer schedule start time is required")
			}
			if node.scheduleRepetitions < -1 {
				return NewError(ErrorInvalidRule, "timer schedule repetitions must be -1 or non-negative")
			}
			return validatePatternTimerPeriod(*node.schedulePeriod)
		}
		if len(node.schedule) == 0 {
			return NewError(ErrorInvalidRule, "timer schedule requires at least one time")
		}
		for index, at := range node.schedule {
			if at.IsZero() {
				return NewError(ErrorInvalidRule, fmt.Sprintf("timer schedule time %d is required", index))
			}
			if index > 0 && at.Before(node.schedule[index-1]) {
				return NewError(ErrorInvalidRule, "timer schedule times must be sorted")
			}
		}
	case patternTimerCronNode:
		if node.cron == nil {
			return NewError(ErrorInvalidRule, "timer cron schedule is required")
		}
		if err := node.cron.validate(); err != nil {
			return err
		}
	default:
		return NewError(ErrorInvalidRule, "unknown pattern expression kind")
	}
	return nil
}

func clonePatternTagSet(tags map[string]struct{}) map[string]struct{} {
	copyTags := make(map[string]struct{}, len(tags))
	for tag := range tags {
		copyTags[tag] = struct{}{}
	}
	return copyTags
}

func patternContainsTimer(node *patternNode) bool {
	if node == nil {
		return false
	}
	if node.kind == patternWithinNode || node.kind == patternTimerIntervalNode || node.kind == patternTimerAtNode || node.kind == patternTimerScheduleNode || node.kind == patternTimerCronNode {
		return true
	}
	return patternContainsTimer(node.left) || patternContainsTimer(node.right) || patternContainsTimer(node.child)
}

func patternHasConsumption(node *patternNode) bool {
	if node == nil {
		return false
	}
	if node.kind == patternEventNode && node.consumeLevelSet {
		return true
	}
	return patternHasConsumption(node.left) || patternHasConsumption(node.right) || patternHasConsumption(node.child)
}
