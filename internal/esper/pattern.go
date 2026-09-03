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
	patternGuardWhileNode
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
	orExclusive            bool
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
	guardExpr              Expr
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
	distinctExpiryCalendar *OutputCalendarPeriod
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
	case patternNotNode, patternEveryNode, patternWithinNode, patternGuardWhileNode:
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
	case patternGuardWhileNode:
		if n.guardExpr == nil {
			return "while-guard(<nil>," + n.child.description() + ")"
		}
		return "while-guard(" + n.guardExpr.Description() + "," + n.child.description() + ")"
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
	everyDistinctCalendar  *OutputCalendarPeriod
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
// Java date/period and R/date/period forms precisely. A zero Period with a
// StartAt is the one-shot date form (Esper timer:schedule(date: X)); a zero
// Period without a StartAt is rejected during Build.
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
	return p.thenWithMaximum(other, 0, false)
}

// ThenMax chains an arbitrary PatternStream branch after the current branch
// while limiting the number of concurrent matches waiting on the right side
// of this followed-by edge. It is the fluent Go counterpart of Esper's
// -[N]> operator with a parenthesized right side and is checked during
// Build just like FollowedByMax.
func (p PatternStream) ThenMax(maximum int, other PatternStream) PatternStream {
	return p.thenWithMaximum(other, maximum, true)
}

func (p PatternStream) thenWithMaximum(other PatternStream, maximum int, maximumSet bool) PatternStream {
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
		kind:           patternSequenceNode,
		left:           patternBranchRoot(p.def),
		right:          patternBranchRoot(other.def),
		sequenceMax:    maximum,
		sequenceMaxSet: maximumSet,
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
	if p.def.everyDistinct == nil && (p.def.every || (p.def.root != nil && p.def.root.kind == patternEveryNode)) {
		// Nested every (every(every(...))): materialize the pending definition
		// level every (or an already materialized every root) into the AST and
		// wrap one more every level around it, so each completion restarts all
		// enclosing every levels, matching Esper's nested-every match
		// multiplicity.
		copyDefinition.steps = nil
		copyDefinition.root = &patternNode{kind: patternEveryNode, child: patternBranchRoot(p.def)}
		copyDefinition.every = false
		p.def = &copyDefinition
		return p
	}
	if p.def.root != nil && !p.def.every && p.def.everyDistinct == nil {
		switch p.def.root.kind {
		case patternAndNode, patternOrNode, patternSequenceNode, patternMatchUntilNode, patternUntilNode, patternNotNode, patternGuardWhileNode, patternEventNode:
			// every <compound> or every <event>: Esper's every state node
			// holds exactly one active child attempt and only spawns the
			// next attempt when the current one completes or fails. A
			// definition-level every would instead start an overlapping
			// fresh root for every event, which double-fires once two
			// events match the same side of a compound (for example every
			// (a and b) receiving two a-events before b), and which cannot
			// be composed as a repeating followed-by left leg: `every a ->
			// b` binds the every to the left event expression, so every
			// a-event must spawn its own concurrent b-wait. Materializing
			// the every into the AST for event roots too keeps the child
			// attempt single and lets FollowedBy/Then build
			// sequence(every(a), b) with one waiting branch per left
			// firing, matching Esper.
			copyDefinition.steps = nil
			copyDefinition.root = &patternNode{kind: patternEveryNode, child: p.def.root}
			p.def = &copyDefinition
			return p
		}
	}
	copyDefinition.every = true
	copyDefinition.steps = append([]patternStep(nil), p.def.steps...)
	copyDefinition.root = p.def.root
	p.def = &copyDefinition
	return p
}

// EveryDistinct restarts the subexpression for every distinct combination of
// the key expressions, mirroring Esper's every-distinct: the first match for
// a key combination is reported and later matches with an already-reported
// combination are swallowed. Keys are tracked per spawned subexpression
// instance; a falsified attempt (for example a not-branch dying) restarts
// with an empty key set, exactly like EvalEveryDistinctStateNode.
func (p PatternStream) EveryDistinct(keys ...Expr) PatternStream {
	if p.def == nil {
		return p
	}
	return p.everyDistinctMaterialize(patternDistinctKeyExpression(keys), 0, false, nil)
}

// EveryDistinctFor limits the lifetime of a distinct key. Once expiry has
// elapsed on the engine's virtual clock, a later event with the same key may
// start a new match. This mirrors Esper's optional expiry interval while
// keeping the primary API expression-oriented.
func (p PatternStream) EveryDistinctFor(expiry time.Duration, keys ...Expr) PatternStream {
	if p.def == nil {
		return p
	}
	return p.everyDistinctMaterialize(patternDistinctKeyExpression(keys), expiry, true, nil)
}

// EveryDistinctForCalendar limits the lifetime of a distinct key with a
// calendar period, mirroring Esper's month-scoped every-distinct expiry
// (every-distinct(key, 1 month)): a key expires once the virtual clock
// reaches the first-seen time shifted by the calendar period.
func (p PatternStream) EveryDistinctForCalendar(years, months, days int, keys ...Expr) PatternStream {
	if p.def == nil {
		return p
	}
	return p.everyDistinctMaterialize(patternDistinctKeyExpression(keys), 0, true, &OutputCalendarPeriod{Years: years, Months: months, Days: days})
}

// everyDistinctMaterialize applies an every-distinct declaration to the
// current pattern. Filter roots keep the definition-level form (one
// persistent filter child, matching EvalFilterStateNode's non-quitting
// behavior under every); compound roots materialize the every-distinct into
// the AST so the child attempt stays single, exactly like Every does.
func (p PatternStream) everyDistinctMaterialize(key Expr, expiry time.Duration, expirySet bool, calendar *OutputCalendarPeriod) PatternStream {
	copyDefinition := *p.def
	if p.def.root != nil && !p.def.every && p.def.everyDistinct == nil {
		switch p.def.root.kind {
		case patternAndNode, patternOrNode, patternSequenceNode, patternMatchUntilNode, patternUntilNode, patternNotNode, patternWithinNode, patternGuardWhileNode:
			copyDefinition.steps = nil
			copyDefinition.root = &patternNode{
				kind:                   patternEveryNode,
				child:                  p.def.root,
				everyExpr:              key,
				distinctExpiry:         expiry,
				distinctExpirySet:      expirySet,
				distinctExpiryCalendar: calendar,
			}
			return PatternStream{env: p.env, def: &copyDefinition}
		}
	}
	copyDefinition.every = true
	copyDefinition.everyDistinct = key
	copyDefinition.everyDistinctExpiry = expiry
	copyDefinition.everyDistinctExpirySet = expirySet
	copyDefinition.everyDistinctCalendar = calendar
	copyDefinition.steps = append([]patternStep(nil), p.def.steps...)
	copyDefinition.root = p.def.root
	return PatternStream{env: p.env, def: &copyDefinition}
}

// patternDistinctKeyExpression folds one or more every-distinct key
// expressions into a single expression. Multiple keys evaluate to their
// value slice so encodeKey compares the combination content-wise, matching
// Esper's multi-expression every-distinct key.
func patternDistinctKeyExpression(keys []Expr) Expr {
	filtered := make([]Expr, 0, len(keys))
	for _, key := range keys {
		if key != nil {
			filtered = append(filtered, key)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	children := make([]*exprNode, 0, len(filtered))
	descriptions := make([]string, 0, len(filtered))
	for _, key := range filtered {
		children = append(children, key.node())
		descriptions = append(descriptions, key.Description())
	}
	return makeExpr[[]any]("distinct-key-composite", strings.Join(descriptions, ","), children, func(ctx EvalContext) Value {
		values := make([]any, 0, len(filtered))
		for _, key := range filtered {
			value := key.eval(ctx)
			if !value.IsPresent() {
				values = append(values, nil)
				continue
			}
			values = append(values, value.Any())
		}
		return Present(values)
	})
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
		child = &patternNode{
			kind:                   patternEveryNode,
			child:                  child,
			everyExpr:              p.def.everyDistinct,
			distinctExpiry:         p.def.everyDistinctExpiry,
			distinctExpirySet:      p.def.everyDistinctExpirySet,
			distinctExpiryCalendar: p.def.everyDistinctCalendar,
		}
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

// WhileGuard applies an expression guard to the pattern, the fluent
// counterpart of Esper's "pattern while (expression)" guard. The guard
// inspects every match the guarded subexpression reports: a true result
// passes the match through, a false result quits the guarded node
// permanently (Esper's guardQuit, which also reports false to the parent),
// and a null or absent result swallows the match without quitting. Unlike
// While, which pre-filters raw input events at the definition level and
// re-arms afterwards, WhileGuard evaluates against the completed match's
// tags and follows ExpressionGuard semantics exactly. A pending
// definition-level Every/EveryDistinct is materialized into the guarded AST
// child so the guard wraps the repeating expression like the Java text form
// "(every a=A) while (expression)".
func (p PatternStream) WhileGuard(guard Expression[bool]) PatternStream {
	if p.def == nil || p.def.root == nil {
		return p
	}
	copyDefinition := *p.def
	copyDefinition.steps = nil
	copyDefinition.every = false
	copyDefinition.everyDistinct = nil
	copyDefinition.everyDistinctExpiry = 0
	copyDefinition.everyDistinctExpirySet = false
	child := p.def.root
	if p.def.every || p.def.everyDistinct != nil {
		child = &patternNode{
			kind:                   patternEveryNode,
			child:                  child,
			everyExpr:              p.def.everyDistinct,
			distinctExpiry:         p.def.everyDistinctExpiry,
			distinctExpirySet:      p.def.everyDistinctExpirySet,
			distinctExpiryCalendar: p.def.everyDistinctCalendar,
		}
	}
	copyDefinition.root = &patternNode{
		kind:      patternGuardWhileNode,
		child:     child,
		guardExpr: guard,
	}
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

// OrExclusive is Java's true OR: when a branch completes terminally (a
// non-repeating observer such as a one-shot filter or timer), the other
// branches are cancelled. A repeating leg (Every) that fires keeps the OR
// alive, matching EvalOrStateNode's isQuitted handling.
func (p PatternStream) OrExclusive(other PatternStream) PatternStream {
	return p.combineExclusive(other, patternOrNode)
}

// combineExclusive is combine with the orExclusive marker set on the OR node.
func (p PatternStream) combineExclusive(other PatternStream, kind patternNodeKind) PatternStream {
	if p.def == nil {
		return p
	}
	copyDefinition := *p.def
	copyDefinition.steps = nil
	copyDefinition.every = false
	copyDefinition.everyDistinct = nil
	copyDefinition.everyDistinctExpiry = 0
	copyDefinition.everyDistinctExpirySet = false
	root := &patternNode{kind: kind, left: patternBranchRoot(p.def), right: patternBranchRoot(other.def)}
	if kind == patternOrNode {
		root.orExclusive = true
	}
	copyDefinition.root = root
	if other.def == nil || p.env != other.env {
		copyDefinition.sourceMismatch = true
	}
	copyDefinition.inputs = mergePatternInputs(p.def, other.def)
	return PatternStream{env: p.env, def: &copyDefinition}
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
// unbounded and minimum == 0 means no lower bound, matching Esper's [:max]
// and [min:] range forms. With an Until terminator attached and
// minimum == maximum > 0 the repetition fires as soon as the bound is
// reached, exactly like Esper's tightly-bound match-until. The repeated tag
// resolves to the latest event through TagField; TagCount exposes the full
// repetition count to projections.
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
		return &patternNode{
			kind:                   patternEveryNode,
			child:                  root,
			everyExpr:              definition.everyDistinct,
			distinctExpiry:         definition.everyDistinctExpiry,
			distinctExpirySet:      definition.everyDistinctExpirySet,
			distinctExpiryCalendar: definition.everyDistinctCalendar,
		}
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
	where      Expr
}

// Where applies a post-match filter to the completed pattern, the fluent Go
// counterpart of a where-clause following Esper's "from pattern [...]". The
// predicate evaluates against the match's tags (via TagField) and suppresses
// matches it does not accept.
func (p PatternQuery) Where(predicate Expression[bool]) PatternQuery {
	copy := p
	if predicate != nil {
		copy.where = predicate
	}
	return copy
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
		patternWhere:               p.where,
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
		iterableUnbound:            spec.iterableUnbound,
		orderBy:                    append([]SortKey(nil), spec.orderBy...),
		limit:                      spec.limit,
		offset:                     spec.offset,
		limitExpr:                  spec.limitExpr,
		offsetExpr:                 spec.offsetExpr,
		limitExprSet:               spec.limitExprSet,
		offsetExprSet:              spec.offsetExprSet,
		indexHints:                 append([]indexHint(nil), spec.indexHints...),
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
	if definition.everyDistinctExpirySet && definition.everyDistinctExpiry <= 0 && definition.everyDistinctCalendar == nil {
		return NewError(ErrorInvalidRule, "every-distinct expiry must be positive")
	}
	if definition.everyDistinctCalendar != nil {
		if err := validatePatternDistinctCalendar(definition.everyDistinctCalendar); err != nil {
			return err
		}
	}
	if definition.everyDistinct != nil {
		if err := validatePatternDistinctKey(definition.everyDistinct, definition.root); err != nil {
			return err
		}
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

// validatePatternDistinctCalendar checks an every-distinct calendar expiry
// period for negative or all-zero components.
func validatePatternDistinctCalendar(calendar *OutputCalendarPeriod) error {
	if calendar.Years < 0 || calendar.Months < 0 || calendar.Days < 0 {
		return NewError(ErrorInvalidRule, "every-distinct calendar expiry cannot be negative")
	}
	if calendar.Years == 0 && calendar.Months == 0 && calendar.Days == 0 {
		return NewError(ErrorInvalidRule, "every-distinct calendar expiry must be positive")
	}
	return nil
}

// validatePatternDistinctKey enforces Esper's every-distinct key rules: each
// key expression must be non-constant, and every tag a key references must be
// produced by the distinct subexpression itself (Esper reports "Failed to
// validate pattern every-distinct expression" for both).
func validatePatternDistinctKey(key Expr, child *patternNode) error {
	if key == nil {
		return NewError(ErrorInvalidRule, "every-distinct requires at least one key expression")
	}
	if patternDistinctKeyIsConstant(key.node()) {
		return NewError(ErrorInvalidRule, "every-distinct key expressions must each return non-constant result values")
	}
	if child != nil {
		tags := collectPatternNodeTags(child)
		for _, tag := range exprNodeTagReferences(key.node()) {
			if !tags[tag] {
				return NewError(ErrorInvalidRule, fmt.Sprintf("every-distinct key expression references tag %q which is not produced by its subexpression", tag))
			}
		}
	}
	return nil
}

// patternDistinctKeyIsConstant reports whether a key expression (or any
// member of a composite key) is a literal constant.
func patternDistinctKeyIsConstant(node *exprNode) bool {
	if node == nil {
		return false
	}
	if node.kind == "distinct-key-composite" {
		for _, child := range node.children {
			if child != nil && child.kind == "literal" {
				return true
			}
		}
		return false
	}
	return node.kind == "literal"
}

// collectPatternNodeTags gathers every tag the pattern subtree can produce.
func collectPatternNodeTags(node *patternNode) map[string]bool {
	tags := make(map[string]bool)
	var walk func(n *patternNode)
	walk = func(n *patternNode) {
		if n == nil {
			return
		}
		if n.tag != "" {
			tags[n.tag] = true
		}
		walk(n.left)
		walk(n.right)
		walk(n.child)
	}
	walk(node)
	return tags
}

// patternTagOrder returns the tags a pattern can bind in declaration order:
// event tags first (left-to-right through the operator tree), then array
// tags. Esper's @Inclusive match routing evaluates the initiating pattern's
// tagged events in this order (tagged events, then array events), so context
// statements observe the start events in the same sequence the pattern bound
// them.
func patternTagOrder(definition *patternDefinition) []string {
	if definition == nil {
		return nil
	}
	order := make([]string, 0, 4)
	seen := make(map[string]bool)
	var walk func(n *patternNode)
	walk = func(n *patternNode) {
		if n == nil {
			return
		}
		if n.tag != "" && !seen[n.tag] {
			seen[n.tag] = true
			order = append(order, n.tag)
		}
		walk(n.left)
		walk(n.right)
		walk(n.child)
	}
	walk(definition.root)
	return order
}

// exprNodeTagReferences gathers the pattern tags an expression tree reads.
func exprNodeTagReferences(node *exprNode) []string {
	tags := make([]string, 0)
	var walk func(n *exprNode)
	walk = func(n *exprNode) {
		if n == nil {
			return
		}
		switch n.kind {
		case "tag-field", "tag-field-at", "pattern-event":
			if n.tagName != "" {
				tags = append(tags, n.tagName)
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(node)
	return tags
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
		if !node.dynamicBounds && node.minimum < 0 {
			return NewError(ErrorInvalidRule, "match-until minimum must not be negative")
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
		if node.distinctExpirySet && node.distinctExpiry <= 0 && node.distinctExpiryCalendar == nil {
			return NewError(ErrorInvalidRule, "every-distinct expiry must be positive")
		}
		if node.distinctExpiryCalendar != nil {
			if err := validatePatternDistinctCalendar(node.distinctExpiryCalendar); err != nil {
				return err
			}
		}
		if node.distinctExpirySet && node.everyExpr == nil {
			return NewError(ErrorInvalidRule, "every-distinct expiry requires a key expression")
		}
		if node.everyExpr != nil {
			if err := validatePatternDistinctKey(node.everyExpr, node.child); err != nil {
				return err
			}
		}
		return validatePatternNodeScope(node.child, seen, false)
	case patternGuardWhileNode:
		if node.guardExpr == nil {
			return NewError(ErrorInvalidRule, "pattern while guard requires an expression")
		}
		if node.guardExpr.Type() != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, "pattern while guard expression must return bool")
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
		if node.duration == 0 && node.durationExpr == nil && node.calendar == nil {
			// Esper accepts timer:interval(0): the observer is due at the
			// clock the statement started on and fires immediately.
			durationForms++
		}
		if node.durationExpr != nil {
			durationForms++
			if node.durationExpr.Type() != typeOf[time.Duration]() {
				return NewError(ErrorTypeMismatch, "timer interval duration expression must return time.Duration")
			}
			if expressionContainsSubquery(node.durationExpr) {
				return NewError(ErrorInvalidRule, "subselects not allowed within pattern observer parameters, consider using a variable instead")
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
			if expressionContainsSubquery(node.scheduleExpr) {
				return NewError(ErrorInvalidRule, "subselects not allowed within pattern observer parameters, consider using a variable instead")
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
			period := *node.schedulePeriod
			if period.Years == 0 && period.Months == 0 && period.Days == 0 && period.FixedDuration == 0 {
				// Esper timer:schedule(date: X) is a one-shot without a
				// period; mirror the ISO date-only form. A spec with
				// neither date nor period is rejected like Java's
				// "Either the date or period parameter is required".
				if !node.scheduleAnchorSet {
					return NewError(ErrorInvalidRule, "timer schedule requires a date or period parameter")
				}
				return nil
			}
			return validatePatternTimerPeriod(period)
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
		for _, field := range []CronField{node.cron.Minute, node.cron.Hour, node.cron.DayOfMonth, node.cron.Month, node.cron.Weekday, node.cron.Second, node.cron.Millisecond, node.cron.Microsecond} {
			for _, expression := range field.expressions() {
				if expressionContainsSubquery(expression) {
					return NewError(ErrorInvalidRule, "subselects not allowed within pattern observer parameters, consider using a variable instead")
				}
			}
		}
		if err := node.cron.validate(); err != nil {
			return err
		}
	default:
		return NewError(ErrorInvalidRule, "unknown pattern expression kind")
	}
	return nil
}

// expressionContainsSubquery reports whether any descendant of the given
// expression is a subquery evaluation. Java rejects subselects inside
// pattern observer parameters ("Subselects are not allowed within pattern
// observer parameters, please consider using a variable instead") while
// allowing them in filter expressions; the fluent API applies the same
// rule to timer interval, timer schedule and cron observer parameters.
func expressionContainsSubquery(expression Expr) bool {
	if expression == nil || expression.node() == nil {
		return false
	}
	var walk func(node *exprNode) bool
	walk = func(node *exprNode) bool {
		if node == nil {
			return false
		}
		if strings.HasPrefix(node.kind, "subquery-") {
			return true
		}
		for _, child := range node.children {
			if walk(child) {
				return true
			}
		}
		return false
	}
	return walk(expression.node())
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
