package esper

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// RowPattern is the analyzable pattern tree used by MatchRecognize.  It is
// intentionally separate from PatternStream: row recognition consumes a
// contiguous event row sequence and binds named variables, while
// PatternStream models event-time boolean patterns.
type RowPattern struct {
	kind       rowPatternKind
	name       string
	parts      []RowPattern
	minimum    int
	maximum    int // zero means unbounded for an atom with a quantifier
	greedy     bool
	quantified bool
}

type rowPatternKind uint8

const (
	rowPatternVariable rowPatternKind = iota
	rowPatternSequence
	rowPatternAlternation
	rowPatternPermutation
)

// RowVar creates a named row-recognition variable. Its DEFINE predicate is
// supplied on RowRecogQuery.Define, keeping the pattern shape and predicates
// independently reusable.
func RowVar(name string) RowPattern {
	return RowPattern{kind: rowPatternVariable, name: strings.TrimSpace(name), minimum: 1, maximum: 1, greedy: true, quantified: true}
}

// Recog is a concise alias for RowVar.
func Recog(name string) RowPattern { return RowVar(name) }

// RowSequence creates a left-to-right contiguous row pattern.
func RowSequence(parts ...RowPattern) RowPattern {
	return RowPattern{kind: rowPatternSequence, parts: append([]RowPattern(nil), parts...), minimum: 1, maximum: 1, greedy: true}
}

// MatchSequence is a descriptive alias for RowSequence.
func MatchSequence(parts ...RowPattern) RowPattern { return RowSequence(parts...) }

// RowAlternation creates an ordered alternative pattern. Alternatives are
// tried in declaration order, which makes ambiguous matches deterministic.
func RowAlternation(parts ...RowPattern) RowPattern {
	return RowPattern{kind: rowPatternAlternation, parts: append([]RowPattern(nil), parts...), minimum: 1, maximum: 1, greedy: true}
}

// OneOf is a concise alias for RowAlternation.
func OneOf(parts ...RowPattern) RowPattern { return RowAlternation(parts...) }

// RowPermute creates the finite permutation of its child patterns. It is a
// structural builder for Esper's permute operator; Build rejects permutations
// above the engine's bounded expansion limit instead of silently changing
// their meaning.
func RowPermute(parts ...RowPattern) RowPattern {
	return RowPattern{kind: rowPatternPermutation, parts: append([]RowPattern(nil), parts...), minimum: 1, maximum: 1, greedy: true}
}

// Permute is an alternate spelling for RowPermute.
func Permute(parts ...RowPattern) RowPattern { return RowPermute(parts...) }

// Or appends an alternative without introducing an EPL-style string grammar.
func (p RowPattern) Or(other RowPattern) RowPattern {
	if p.kind == rowPatternAlternation {
		p.parts = append(p.parts, other)
		return p
	}
	return RowAlternation(p, other)
}

// Optional matches zero or one row for the variable/pattern.
func (p RowPattern) Optional() RowPattern { return p.Repeat(0, 1) }

// ZeroOrMore matches zero or more contiguous rows. The default is greedy.
func (p RowPattern) ZeroOrMore() RowPattern { return p.Repeat(0, 0) }

// OneOrMore matches one or more contiguous rows. The default is greedy.
func (p RowPattern) OneOrMore() RowPattern { return p.Repeat(1, 0) }

// Repeat applies an inclusive quantifier. A maximum of zero means unbounded.
func (p RowPattern) Repeat(minimum, maximum int) RowPattern {
	p.minimum = minimum
	p.maximum = maximum
	p.quantified = true
	return p
}

// Reluctant changes a repeated pattern to try the shortest match first.
func (p RowPattern) Reluctant() RowPattern {
	p.greedy = false
	return p
}

type RowRecogSkipStrategy uint8

const (
	// RowRecogSkipPastLastRow is Esper's default skip strategy.
	RowRecogSkipPastLastRow RowRecogSkipStrategy = iota
	RowRecogSkipToNextRow
	RowRecogSkipToCurrentRow
)

// RowRecogQuery is the fluent builder for a MATCH_RECOGNIZE statement.
type RowRecogQuery struct {
	env        *Environment
	definition *rowRecogDefinition
	measures   []Selection
}

type rowRecogDefinition struct {
	input                *streamNode
	pattern              RowPattern
	defines              map[string]Expr
	duplicateDefines     map[string]struct{}
	partition            []Expr
	allMatches           bool
	iterateOnly          bool
	skip                 RowRecogSkipStrategy
	maxStates            int
	interval             time.Duration
	intervalCalendar     *OutputCalendarPeriod
	intervalOrTerminated bool
	statePoolNFAOnce     sync.Once
	statePoolNFA         *rowRecogStatePoolNFA
}

// MatchRecognize starts a row-pattern query from a typed stream.
func (s Stream[T]) MatchRecognize(pattern RowPattern) RowRecogQuery {
	return RowRecogQuery{
		env: s.env,
		definition: &rowRecogDefinition{
			input:            s.node,
			pattern:          pattern,
			defines:          make(map[string]Expr),
			duplicateDefines: make(map[string]struct{}),
			allMatches:       true,
			skip:             RowRecogSkipPastLastRow,
		},
	}
}

// MatchRecognize starts a row-pattern query from a dynamic RecordStream.
func (s RecordStream) MatchRecognize(pattern RowPattern) RowRecogQuery {
	return RowRecogQuery{
		env: s.env,
		definition: &rowRecogDefinition{
			input:            s.node,
			pattern:          pattern,
			defines:          make(map[string]Expr),
			duplicateDefines: make(map[string]struct{}),
			allMatches:       true,
			skip:             RowRecogSkipPastLastRow,
		},
		measures: append([]Selection(nil), s.selections...),
	}
}

// Define attaches the predicate for one named variable. An omitted variable
// predicate is equivalent to true, matching MATCH_RECOGNIZE's default for a
// variable that has no DEFINE entry.
func (q RowRecogQuery) Define(name string, predicate Expr) RowRecogQuery {
	if q.definition == nil {
		return q
	}
	if q.definition.defines == nil {
		q.definition.defines = make(map[string]Expr)
	}
	name = strings.TrimSpace(name)
	if existing, exists := q.definition.defines[name]; exists && (existing == nil || predicate == nil || existing.Description() != predicate.Description()) {
		if q.definition.duplicateDefines == nil {
			q.definition.duplicateDefines = make(map[string]struct{})
		}
		q.definition.duplicateDefines[name] = struct{}{}
	}
	q.definition.defines[name] = predicate
	return q
}

// Measures adds named result expressions. TagField, TagFieldAt and TagCount
// are the usual measure expressions; they resolve against the row variables
// captured by the match.
func (q RowRecogQuery) Measures(selections ...Selection) RowRecogQuery {
	q.measures = append(q.measures, selections...)
	return q
}

// Select is a fluent alias for Measures.
func (q RowRecogQuery) Select(selections ...Selection) RowRecogQuery {
	return q.Measures(selections...)
}

// PartitionBy creates independent recognition state for each key.
func (q RowRecogQuery) PartitionBy(keys ...Expr) RowRecogQuery {
	if q.definition != nil {
		q.definition.partition = append(q.definition.partition, keys...)
	}
	return q
}

// AllMatches retains every match permitted by the skip strategy.
func (q RowRecogQuery) AllMatches() RowRecogQuery {
	if q.definition != nil {
		q.definition.allMatches = true
	}
	return q
}

// FirstMatch limits each input event to the first match found in pattern
// start order. It is the explicit Go counterpart of omitting ALL MATCHES.
func (q RowRecogQuery) FirstMatch() RowRecogQuery {
	if q.definition != nil {
		q.definition.allMatches = false
	}
	return q
}

// IterateOnly keeps the source/window and PREV history up to date but defers
// row-pattern evaluation until Statement.Snapshot.  This is the fluent Go
// counterpart of Esper's @Hint('iterate_only') for MATCH_RECOGNIZE and is
// useful for high-rate streams whose consumers poll the iterator instead of
// receiving incremental listener batches.
func (q RowRecogQuery) IterateOnly() RowRecogQuery {
	if q.definition != nil {
		q.definition.iterateOnly = true
	}
	return q
}

func (q RowRecogQuery) Skip(strategy RowRecogSkipStrategy) RowRecogQuery {
	if q.definition != nil {
		q.definition.skip = strategy
	}
	return q
}

func (q RowRecogQuery) SkipPastLastRow() RowRecogQuery {
	return q.Skip(RowRecogSkipPastLastRow)
}

func (q RowRecogQuery) SkipToNextRow() RowRecogQuery {
	return q.Skip(RowRecogSkipToNextRow)
}

func (q RowRecogQuery) SkipToCurrentRow() RowRecogQuery {
	return q.Skip(RowRecogSkipToCurrentRow)
}

// MaxStates bounds active recognition branches. Zero leaves the runtime
// unbounded, subject to the engine's normal resource policy.
func (q RowRecogQuery) MaxStates(maximum int) RowRecogQuery {
	if q.definition != nil {
		q.definition.maxStates = maximum
	}
	return q
}

// Interval delays match notification until the recognition branch has been
// open for the supplied duration. The retained match state remains visible
// through Statement.Snapshot while the interval is pending.
func (q RowRecogQuery) Interval(duration time.Duration) RowRecogQuery {
	if q.definition != nil {
		q.definition.interval = duration
		q.definition.intervalCalendar = nil
		q.definition.intervalOrTerminated = false
	}
	return q
}

// IntervalOrTerminated delays an open match until the supplied duration, but
// emits it earlier when the incoming row terminates the active branch. A
// fixed-width pattern can therefore complete immediately, while a repeated
// tail remains pending until a mismatch or the deadline.
func (q RowRecogQuery) IntervalOrTerminated(duration time.Duration) RowRecogQuery {
	if q.definition != nil {
		q.definition.interval = duration
		q.definition.intervalCalendar = nil
		q.definition.intervalOrTerminated = true
	}
	return q
}

// IntervalCalendar uses calendar-aware year/month/day arithmetic for the
// match deadline. It is the row-recognition counterpart of output calendar
// gates and preserves month boundaries instead of converting them to hours.
func (q RowRecogQuery) IntervalCalendar(years, months, days int) RowRecogQuery {
	if q.definition != nil {
		q.definition.interval = 0
		q.definition.intervalCalendar = &OutputCalendarPeriod{Years: years, Months: months, Days: days}
	}
	return q
}

func (q RowRecogQuery) Query(options ...QueryOption) Query {
	spec := querySpec{selector: SelectIStream, output: OutputAll()}
	for _, option := range options {
		if option != nil {
			option(&spec)
		}
	}
	return Query{
		env:                        q.env,
		input:                      q.definitionInput(),
		rowRecog:                   q.definition,
		patternSelections:          append([]Selection(nil), q.measures...),
		name:                       spec.name,
		statementUserObject:        spec.statementUserObject,
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
	}
}

func (q RowRecogQuery) definitionInput() *streamNode {
	if q.definition == nil {
		return nil
	}
	return q.definition.input
}

func (definition *rowRecogDefinition) description() string {
	if definition == nil {
		return "match-recognize(<nil>)"
	}
	parts := []string{"match-recognize(pattern(" + definition.pattern.description() + ")"}
	names := make([]string, 0, len(definition.defines))
	for name := range definition.defines {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) > 0 {
		defines := make([]string, 0, len(names))
		for _, name := range names {
			predicate := "true"
			if definition.defines[name] != nil {
				predicate = definition.defines[name].Description()
			}
			defines = append(defines, name+"="+predicate)
		}
		parts = append(parts, "define("+strings.Join(defines, ",")+")")
	}
	if len(definition.partition) > 0 {
		keys := make([]string, 0, len(definition.partition))
		for _, key := range definition.partition {
			if key == nil {
				keys = append(keys, "<nil>")
			} else {
				keys = append(keys, key.Description())
			}
		}
		parts = append(parts, "partition("+strings.Join(keys, ",")+")")
	}
	if definition.allMatches {
		parts = append(parts, "all-matches")
	} else {
		parts = append(parts, "first-match")
	}
	if definition.iterateOnly {
		parts = append(parts, "iterate-only")
	}
	switch definition.skip {
	case RowRecogSkipToNextRow:
		parts = append(parts, "skip-next-row")
	case RowRecogSkipToCurrentRow:
		parts = append(parts, "skip-current-row")
	default:
		parts = append(parts, "skip-past-last-row")
	}
	if definition.maxStates > 0 {
		parts = append(parts, fmt.Sprintf("max-states(%d)", definition.maxStates))
	}
	if definition.interval > 0 {
		intervalName := "interval"
		if definition.intervalOrTerminated {
			intervalName = "interval-or-terminated"
		}
		parts = append(parts, intervalName+"("+definition.interval.String()+")")
	}
	if definition.intervalCalendar != nil {
		period := definition.intervalCalendar
		parts = append(parts, fmt.Sprintf("interval-calendar(%dY%dM%dD)", period.Years, period.Months, period.Days))
	}
	return strings.Join(parts, ", ") + ")"
}

func (pattern RowPattern) description() string {
	if pattern.kind == rowPatternSequence {
		parts := make([]string, 0, len(pattern.parts))
		for _, part := range pattern.parts {
			parts = append(parts, part.description())
		}
		base := strings.Join(parts, " ")
		if !pattern.quantified || (pattern.minimum == 1 && pattern.maximum == 1) {
			return base
		}
		return "(" + base + ")" + rowPatternQuantifier(pattern.minimum, pattern.maximum, pattern.greedy)
	}
	if pattern.kind == rowPatternAlternation {
		parts := make([]string, 0, len(pattern.parts))
		for _, part := range pattern.parts {
			parts = append(parts, part.description())
		}
		base := "(" + strings.Join(parts, "|") + ")"
		if !pattern.quantified || (pattern.minimum == 1 && pattern.maximum == 1) {
			return base
		}
		return base + rowPatternQuantifier(pattern.minimum, pattern.maximum, pattern.greedy)
	}
	if pattern.kind == rowPatternPermutation {
		parts := make([]string, 0, len(pattern.parts))
		for _, part := range pattern.parts {
			parts = append(parts, part.description())
		}
		base := "permute(" + strings.Join(parts, ",") + ")"
		if !pattern.quantified || (pattern.minimum == 1 && pattern.maximum == 1) {
			return base
		}
		return "(" + base + ")" + rowPatternQuantifier(pattern.minimum, pattern.maximum, pattern.greedy)
	}
	name := pattern.name
	if name == "" {
		name = "<invalid>"
	}
	if pattern.minimum == 1 && pattern.maximum == 1 {
		return name
	}
	return name + rowPatternQuantifier(pattern.minimum, pattern.maximum, pattern.greedy)
}

func rowPatternQuantifier(minimum, maximum int, greedy bool) string {
	quantifier := "*"
	switch {
	case minimum == 0 && maximum == 1:
		quantifier = "?"
	case minimum == 1 && maximum == 0:
		quantifier = "+"
	case maximum == 0:
		quantifier = fmt.Sprintf("{%d,}", minimum)
	case minimum == maximum:
		quantifier = fmt.Sprintf("{%d}", minimum)
	default:
		quantifier = fmt.Sprintf("{%d,%d}", minimum, maximum)
	}
	if !greedy {
		quantifier += "?"
	}
	return quantifier
}

func (pattern RowPattern) validate(path string) error {
	if pattern.kind == rowPatternSequence || pattern.kind == rowPatternAlternation || pattern.kind == rowPatternPermutation {
		if len(pattern.parts) == 0 {
			if pattern.kind == rowPatternSequence {
				return fmt.Errorf("%s sequence cannot be empty", path)
			}
			if pattern.kind == rowPatternPermutation {
				return fmt.Errorf("%s permutation cannot be empty", path)
			}
			return fmt.Errorf("%s alternation cannot be empty", path)
		}
		if pattern.kind == rowPatternPermutation && rowPatternPermutationCount(len(pattern.parts)) > maxRowPatternPermutations {
			return fmt.Errorf("%s permutation exceeds %d alternatives", path, maxRowPatternPermutations)
		}
		if pattern.quantified {
			if pattern.minimum < 0 || pattern.maximum < 0 {
				return fmt.Errorf("%s group quantifier cannot be negative", path)
			}
			if pattern.minimum > pattern.maximum && pattern.maximum != 0 {
				return fmt.Errorf("%s group quantifier minimum exceeds maximum", path)
			}
		}
		for index, part := range pattern.parts {
			if err := part.validate(fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
		return nil
	}
	if pattern.kind != rowPatternVariable {
		return fmt.Errorf("%s has unknown pattern kind %d", path, pattern.kind)
	}
	if pattern.name == "" {
		return fmt.Errorf("%s variable name is required", path)
	}
	if pattern.minimum < 0 || pattern.maximum < 0 {
		return fmt.Errorf("%s quantifier cannot be negative", path)
	}
	if pattern.maximum > 0 && pattern.minimum > pattern.maximum {
		return fmt.Errorf("%s quantifier minimum exceeds maximum", path)
	}
	return nil
}

func rowPatternVariables(pattern RowPattern, result map[string]struct{}) {
	if pattern.kind == rowPatternSequence || pattern.kind == rowPatternAlternation || pattern.kind == rowPatternPermutation {
		for _, part := range pattern.parts {
			rowPatternVariables(part, result)
		}
		return
	}
	if pattern.name != "" {
		result[pattern.name] = struct{}{}
	}
}

const maxRowPatternPermutations = 720

func rowPatternPermutationCount(size int) int {
	result := 1
	for index := 2; index <= size; index++ {
		if result > maxRowPatternPermutations/index {
			return maxRowPatternPermutations + 1
		}
		result *= index
	}
	return result
}

func rowPatternPermutationOrders(parts []RowPattern) [][]RowPattern {
	result := make([][]RowPattern, 0, rowPatternPermutationCount(len(parts)))
	var visit func([]RowPattern, []RowPattern)
	visit = func(remaining, prefix []RowPattern) {
		if len(remaining) == 0 {
			result = append(result, append([]RowPattern(nil), prefix...))
			return
		}
		for index, part := range remaining {
			next := append([]RowPattern(nil), remaining[:index]...)
			next = append(next, remaining[index+1:]...)
			visit(next, append(prefix, part))
		}
	}
	visit(append([]RowPattern(nil), parts...), nil)
	return result
}

type rowRecogMatch struct {
	start    int
	end      int
	terminal bool
	branch   string
	captures map[string][]Event
}

type rowRecogFastABStarCDefinition struct {
	a string
	b string
	c string
}

type rowRecogFastABStarCPath struct {
	start int
	a     Event
}

func (r *statementRuntime) rowRecogBatch(delta eventDelta, plan Plan, now time.Time) ResultBatch {
	definition := plan.query.rowRecog
	if definition == nil {
		return ResultBatch{}
	}
	if r.rowRecogState == nil {
		r.rowRecogState = &rowRecogRuntimeState{partitions: make(map[string]*rowRecogPartitionState)}
	}
	if r.rowRecogState.partitions == nil {
		r.rowRecogState.partitions = make(map[string]*rowRecogPartitionState)
	}
	batch := ResultBatch{Time: now, outputCountsSet: true, outputInserted: int64(len(delta.newEvents)), outputRemoved: int64(len(delta.oldEvents))}
	fastDefinition, fastPath := rowRecogFastABStarCDefinitionFor(definition)
	if r.rowRecogStatePoolTracking(definition) {
		// The fast AB* C representation intentionally keeps one path per
		// start. A configured engine-wide pool needs every NFA successor, so
		// use the general result matcher together with the NFA accounting path.
		fastPath = false
	}

	// Retention changes must be applied before new rows are matched. This is
	// observable for length/time windows where one arrival can remove an older
	// row and add a new row in the same engine transaction.
	for _, event := range delta.oldEvents {
		r.removeRowRecogEvent(definition, event, now)
	}
	for _, event := range delta.newEvents {
		key := rowRecogPartitionKey(definition, event, nil, now, r.variables)
		partition := r.rowRecogState.partitions[key]
		if partition == nil {
			partition = newRowRecogPartitionState()
			r.rowRecogState.partitions[key] = partition
		}
		r.recordRowRecogPrevious(definition, partition, event, plan)
		if !definition.iterateOnly {
			r.admitRowRecogStart(definition, partition, event, now)
		}
		partition.events = append(partition.events, event)
		if definition.iterateOnly {
			// Esper's iterate-only mode deliberately does not advance the NFA
			// or produce child output.  The retained events and previous-access
			// history above are sufficient for snapshot-time evaluation.
			continue
		}
		r.reconcileRowRecogStatePool(definition, partition, len(partition.events)-1, now)
		if !rowRecogHasInterval(definition) {
			if fastPath {
				r.emitRowRecogFastABStarC(definition, partition, len(partition.events)-1, plan, now, &batch, fastDefinition)
			} else {
				r.emitRowRecogMatchesAtEnd(definition, partition, len(partition.events)-1, plan, now, &batch)
			}
			r.pruneRowRecogFixedSequenceStarts(definition, partition, len(partition.events)-1, now)
		} else if definition.intervalOrTerminated {
			r.emitRowRecogTerminated(definition, partition, len(partition.events)-1, plan, now, &batch)
		}
	}
	if definition.iterateOnly {
		if rowRecogBatchWindowNode(definition.input) != nil {
			r.resetRowRecogBatchState()
		}
		return ResultBatch{}
	}
	if rowRecogHasInterval(definition) {
		r.flushRowRecogIntervals(definition, plan, now, &batch)
	}
	if !batch.empty() {
		if plan.query.distinct {
			batch.New, batch.Old = r.applyDistinct(plan.query, batch.New, batch.Old)
		}
		batch.New = orderRowRecogResults(batch.New, plan.query.orderBy, now, r.variables)
		batch.Old = orderRowRecogResults(batch.Old, plan.query.orderBy, now, r.variables)
		batch.New = applyResultWindow(batch.New, plan.query)
		batch.Old = applyResultWindow(batch.Old, plan.query)
		batch.Sequence = r.seq.Add(1)
	}
	if rowRecogBatchWindowNode(definition.input) != nil {
		r.resetRowRecogBatchState()
	}
	return batch
}

func newRowRecogPartitionState() *rowRecogPartitionState {
	return &rowRecogPartitionState{
		previousByEvent:    make(map[string][]Event),
		emitted:            make(map[string]struct{}),
		emittedMatches:     make(map[string]rowRecogMatch),
		intervalClosed:     make(map[string]struct{}),
		closedBranches:     make(map[string]struct{}),
		alternateNotified:  make(map[string]struct{}),
		intervalNotified:   make(map[string]struct{}),
		intervalFinal:      make(map[string][]rowRecogMatch),
		activeStarts:       make(map[string]struct{}),
		activeStateCounts:  make(map[string]int64),
		activePaths:        make(map[string][]rowRecogNFAPath),
		allowedMatchStarts: make(map[string]struct{}),
		allowedMatchKeys:   make(map[string]struct{}),
		blockedStarts:      make(map[string]struct{}),
	}
}

func (r *statementRuntime) recordRowRecogPrevious(definition *rowRecogDefinition, partition *rowRecogPartitionState, event Event, plan Plan) {
	if definition == nil || partition == nil {
		return
	}
	if partition.previousByEvent == nil {
		partition.previousByEvent = make(map[string][]Event)
	}
	rolling := append(append([]Event(nil), partition.previousRolling...), event)
	maximum := rowRecogPreviousMaximum(definition, plan)
	if maximum < 0 {
		return
	}
	needed := maximum + 1
	if needed < 1 {
		needed = 1
	}
	if len(rolling) > needed {
		rolling = rolling[len(rolling)-needed:]
	}
	partition.previousRolling = append([]Event(nil), rolling...)
	partition.previousByEvent[rowRecogPreviousEventKey(event)] = append([]Event(nil), rolling...)
}

func rowRecogPreviousEventKey(event Event) string {
	return fmt.Sprintf("%s:%d", eventIdentity(event), event.ReceivedAt().UnixNano())
}

func rowRecogPreviousMaximum(definition *rowRecogDefinition, plan Plan) int {
	maximum := -1
	if definition == nil {
		return maximum
	}
	var walk func(*exprNode)
	walk = func(node *exprNode) {
		if node == nil {
			return
		}
		requested := node.previousOffset
		if node.kind == "prior" {
			requested++
		}
		if (node.kind == "prev" || node.kind == "prior") && requested > maximum {
			maximum = requested
		}
		for _, child := range node.children {
			walk(child)
		}
	}
	for _, expression := range definition.defines {
		if expression != nil {
			walk(expression.node())
		}
	}
	for _, selection := range plan.query.patternSelections {
		if selection.Expr != nil {
			walk(selection.Expr.node())
		}
	}
	return maximum
}

func rowRecogPreviousHistory(partition *rowRecogPartitionState, event Event, fallback []Event) []Event {
	if partition == nil {
		return append([]Event(nil), fallback...)
	}
	return rowRecogPreviousHistoryByMap(partition.previousByEvent, event, fallback)
}

func rowRecogPreviousHistoryByMap(previousByEvent map[string][]Event, event Event, fallback []Event) []Event {
	if previousByEvent != nil {
		if history, ok := previousByEvent[rowRecogPreviousEventKey(event)]; ok {
			return append([]Event(nil), history...)
		}
	}
	return append([]Event(nil), fallback...)
}

func (r *statementRuntime) admitRowRecogStart(definition *rowRecogDefinition, partition *rowRecogPartitionState, event Event, now time.Time) {
	if r == nil || definition == nil || partition == nil || !rowRecogEventCanStart(definition, partition, event, now, r.variables) {
		return
	}
	if partition.activeStarts == nil {
		partition.activeStarts = make(map[string]struct{})
	}
	if partition.blockedStarts == nil {
		partition.blockedStarts = make(map[string]struct{})
	}
	if r.rowRecogStatePoolTracking(definition) {
		// The state-pool NFA admits the start and charges each successor after
		// the event has been consumed. Charging here would count a synthetic
		// one-state approximation before branches are known.
		return
	}
	key := rowRecogStartKey(len(partition.events), event)
	allowed := true
	if r.engine != nil && r.rowRecogOwner != "" && r.engine.matchRecognizeStatePool != nil {
		allowed = r.engine.matchRecognizeStatePool.tryIncrease(r.engine, r.rowRecogOwner)
	}
	if allowed {
		partition.activeStarts[key] = struct{}{}
	} else {
		partition.blockedStarts[key] = struct{}{}
	}
}

func (r *statementRuntime) rowRecogStatePoolTracking(definition *rowRecogDefinition) bool {
	if r == nil || definition == nil || r.engine == nil || r.rowRecogOwner == "" || r.engine.matchRecognizeStatePool == nil {
		return false
	}
	if r.engine.matchRecognizeStatePool.maxStates < 0 {
		return false
	}
	return rowRecogStatePoolNFAFor(definition) != nil
}

// reconcileRowRecogStatePool advances each active NFA entry exactly once for
// the current event. Esper releases the consumed entry before charging every
// successor edge; doing the same here matters when PreventStart is enabled
// and a single repeated/alternating state fans out to more than one node.
func (r *statementRuntime) reconcileRowRecogStatePool(definition *rowRecogDefinition, partition *rowRecogPartitionState, end int, now time.Time) {
	if r == nil || partition == nil || end < 0 || !r.rowRecogStatePoolTracking(definition) {
		return
	}
	nfa := rowRecogStatePoolNFAFor(definition)
	pool := r.engine.matchRecognizeStatePool
	if nfa == nil || pool == nil || end >= len(partition.events) {
		return
	}
	if partition.activePaths == nil {
		partition.activePaths = make(map[string][]rowRecogNFAPath)
	}
	if partition.activeStateCounts == nil {
		partition.activeStateCounts = make(map[string]int64)
	}
	if partition.blockedStarts == nil {
		partition.blockedStarts = make(map[string]struct{})
	}
	oldPaths := partition.activePaths
	oldCounts := partition.activeStateCounts
	nextPaths := make(map[string][]rowRecogNFAPath)
	nextCounts := make(map[string]int64)
	nextActive := make(map[string]struct{})
	nextBlocked := make(map[string]struct{}, len(partition.blockedStarts))
	for key := range partition.blockedStarts {
		nextBlocked[key] = struct{}{}
	}
	allowedMatches := make(map[string]struct{})
	allowedMatchKeys := make(map[string]struct{})

	type activeEntry struct {
		key   string
		start int
		paths []rowRecogNFAPath
	}
	entries := make([]activeEntry, 0, len(oldPaths))
	for key, paths := range oldPaths {
		if len(paths) == 0 {
			continue
		}
		start, found := rowRecogActiveStartIndex(partition, key)
		if !found {
			if count := oldCounts[key]; count > 0 {
				pool.decrease(r.rowRecogOwner, count)
			}
			continue
		}
		entries = append(entries, activeEntry{key: key, start: start, paths: paths})
	}
	sort.SliceStable(entries, func(left, right int) bool { return entries[left].start < entries[right].start })

	for _, entry := range entries {
		accepted := make([]rowRecogNFAPath, 0, len(entry.paths))
		activeCandidate := false
		activeDenied := false
		for _, path := range entry.paths {
			pool.decrease(r.rowRecogOwner, 1)
			captures, matched := rowRecogNFAPathMatches(definition, partition, path, end, now, r.variables)
			if !matched || path.node == nil {
				continue
			}
			if path.node.terminal {
				allowedMatches[entry.key] = struct{}{}
				allowedMatchKeys[rowRecogMatchKey(rowRecogMatch{start: entry.start, end: end, captures: captures})] = struct{}{}
			}
			for _, next := range path.node.next {
				activeCandidate = true
				candidate := rowRecogNFAPath{node: next, captures: captures}
				if pool.tryIncrease(r.engine, r.rowRecogOwner) {
					accepted = append(accepted, candidate)
				} else {
					activeDenied = true
				}
			}
		}
		if len(accepted) > 0 {
			nextPaths[entry.key] = accepted
			nextCounts[entry.key] = int64(len(accepted))
			nextActive[entry.key] = struct{}{}
			delete(nextBlocked, entry.key)
		} else if activeCandidate && activeDenied {
			// No successor survived the pool. Marking the start blocked keeps
			// the result matcher from recreating an NFA branch on a later row.
			nextBlocked[entry.key] = struct{}{}
		}
	}

	// New starts are evaluated after existing states, matching RowRecogNFAView
	// step(): current entries are consumed first, then factory start states are
	// offered for the same event.
	startKey := rowRecogStartKey(end, partition.events[end])
	startAccepted := make([]rowRecogNFAPath, 0, len(nfa.starts))
	startCandidate := false
	startDenied := false
	for _, start := range nfa.starts {
		captures, matched := rowRecogNFAStartMatches(definition, partition, start, end, now, r.variables)
		if !matched || start == nil {
			continue
		}
		if start.terminal {
			allowedMatches[startKey] = struct{}{}
			allowedMatchKeys[rowRecogMatchKey(rowRecogMatch{start: end, end: end, captures: captures})] = struct{}{}
		}
		for _, next := range start.next {
			startCandidate = true
			candidate := rowRecogNFAPath{node: next, captures: captures}
			if pool.tryIncrease(r.engine, r.rowRecogOwner) {
				startAccepted = append(startAccepted, candidate)
			} else {
				startDenied = true
			}
		}
	}
	if len(startAccepted) > 0 {
		nextPaths[startKey] = startAccepted
		nextCounts[startKey] = int64(len(startAccepted))
		nextActive[startKey] = struct{}{}
		delete(nextBlocked, startKey)
	} else if startCandidate && startDenied {
		nextBlocked[startKey] = struct{}{}
	}

	partition.activePaths = nextPaths
	partition.activeStateCounts = nextCounts
	partition.activeStarts = nextActive
	partition.blockedStarts = nextBlocked
	partition.allowedMatchStarts = allowedMatches
	partition.allowedMatchKeys = allowedMatchKeys
}

func rowRecogNFAPathMatches(definition *rowRecogDefinition, partition *rowRecogPartitionState, path rowRecogNFAPath, end int, now time.Time, variables map[string]Value) (map[string][]Event, bool) {
	if path.node == nil {
		return nil, false
	}
	return rowRecogNFANodeMatches(definition, partition, path.node, path.captures, end, now, variables)
}

func rowRecogNFAStartMatches(definition *rowRecogDefinition, partition *rowRecogPartitionState, node *rowRecogStatePoolNFANode, end int, now time.Time, variables map[string]Value) (map[string][]Event, bool) {
	return rowRecogNFANodeMatches(definition, partition, node, nil, end, now, variables)
}

func rowRecogNFANodeMatches(definition *rowRecogDefinition, partition *rowRecogPartitionState, node *rowRecogStatePoolNFANode, captures map[string][]Event, end int, now time.Time, variables map[string]Value) (map[string][]Event, bool) {
	if definition == nil || partition == nil || node == nil || end < 0 || end >= len(partition.events) {
		return nil, false
	}
	event := partition.events[end]
	next := cloneRowRecogCaptures(captures)
	next[node.variable] = append(next[node.variable], event)
	if predicate := definition.defines[node.variable]; predicate != nil {
		history := partition.events[:end+1]
		value := predicate.eval(EvalContext{
			Event:           event,
			History:         append([]Event(nil), history...),
			PreviousHistory: rowRecogPreviousHistoryByMap(partition.previousByEvent, event, history),
			Tags:            rowRecogLastTags(next),
			TagValues:       cloneRowRecogCaptures(next),
			Now:             now,
			Variables:       variables,
		})
		matched, ok := boolValue(value)
		if !ok || !matched {
			return nil, false
		}
	}
	return next, true
}

// pruneRowRecogFixedSequenceStarts releases starts that have no NFA state
// left after the current event. The general row-recognition evaluator is
// intentionally history-based, while the runtime state pool is lifecycle
// based: a fixed sequence can therefore cheaply and safely identify dead
// starts without changing the more involved repeated/alternating matcher.
//
// This narrow structural path is important for engine-wide state limits. In
// Java, a state that cannot transition on the current event is removed before
// the next event; retaining it as an admitted start would make later
// PreventStart decisions observe a larger pool than the actual NFA.
func (r *statementRuntime) pruneRowRecogFixedSequenceStarts(definition *rowRecogDefinition, partition *rowRecogPartitionState, end int, now time.Time) {
	if r == nil || definition == nil || partition == nil || end < 0 || rowRecogHasInterval(definition) {
		return
	}
	parts, ok := rowRecogFixedVariableSequence(definition.pattern)
	if !ok || len(partition.activeStarts) == 0 {
		return
	}
	for key := range partition.activeStarts {
		start, found := rowRecogActiveStartIndex(partition, key)
		if !found || !rowRecogFixedSequenceHasOpenPath(definition, parts, partition, start, end, now, r.variables) {
			r.releaseRowRecogStart(partition, key)
		}
	}
}

func rowRecogFixedVariableSequence(pattern RowPattern) ([]RowPattern, bool) {
	minimum, maximum := rowPatternBounds(pattern)
	if minimum != 1 || maximum != 1 {
		return nil, false
	}
	if pattern.kind == rowPatternVariable {
		return []RowPattern{pattern}, true
	}
	if pattern.kind != rowPatternSequence || len(pattern.parts) == 0 {
		return nil, false
	}
	parts := make([]RowPattern, len(pattern.parts))
	for index, part := range pattern.parts {
		partMinimum, partMaximum := rowPatternBounds(part)
		if partMinimum != 1 || partMaximum != 1 || part.kind != rowPatternVariable {
			return nil, false
		}
		parts[index] = part
	}
	return parts, true
}

func rowRecogActiveStartIndex(partition *rowRecogPartitionState, key string) (int, bool) {
	if partition == nil {
		return 0, false
	}
	for index, event := range partition.events {
		if rowRecogStartKey(index, event) == key {
			return index, true
		}
	}
	return 0, false
}

func rowRecogFixedSequenceHasOpenPath(definition *rowRecogDefinition, parts []RowPattern, partition *rowRecogPartitionState, start, end int, now time.Time, variables map[string]Value) bool {
	if definition == nil || partition == nil || start < 0 || end < start || end >= len(partition.events) || len(parts) == 0 {
		return false
	}
	consumed := end - start + 1
	if consumed >= len(parts) {
		return false
	}
	captures := make(map[string][]Event)
	for offset := 0; offset < consumed; offset++ {
		pattern := parts[offset]
		eventIndex := start + offset
		event := partition.events[eventIndex]
		captures[pattern.name] = append(captures[pattern.name], event)
		predicate := definition.defines[pattern.name]
		if predicate == nil {
			continue
		}
		history := append([]Event(nil), partition.events[:eventIndex+1]...)
		value := predicate.eval(EvalContext{
			Event:           event,
			History:         history,
			PreviousHistory: rowRecogPreviousHistoryByMap(partition.previousByEvent, event, history),
			Tags:            rowRecogLastTags(captures),
			TagValues:       cloneRowRecogCaptures(captures),
			Now:             now,
			Variables:       variables,
		})
		matched, valid := boolValue(value)
		if !valid || !matched {
			return false
		}
	}
	return true
}

func (r *statementRuntime) releaseRowRecogStart(partition *rowRecogPartitionState, key string) {
	if r == nil || partition == nil || key == "" {
		return
	}
	count := int64(0)
	if partition.activeStateCounts != nil {
		count = partition.activeStateCounts[key]
		delete(partition.activeStateCounts, key)
	}
	if _, active := partition.activeStarts[key]; active {
		if count == 0 {
			// Legacy accounting admits one state per start. This fallback also
			// keeps the lifecycle safe for state created before the NFA map was
			// initialized.
			count = 1
		}
		delete(partition.activeStarts, key)
	}
	if partition.activePaths != nil {
		delete(partition.activePaths, key)
	}
	if partition.allowedMatchStarts != nil {
		delete(partition.allowedMatchStarts, key)
	}
	if r.engine != nil && r.engine.matchRecognizeStatePool != nil && count > 0 {
		r.engine.matchRecognizeStatePool.decrease(r.rowRecogOwner, count)
	}
}

func rowRecogEventCanStart(definition *rowRecogDefinition, partition *rowRecogPartitionState, event Event, now time.Time, variables map[string]Value) bool {
	if definition == nil || partition == nil {
		return false
	}
	names := rowPatternStartVariables(definition.pattern)
	if len(names) == 0 {
		return false
	}
	history := partition.events
	needsPrevious := false
	for _, name := range names {
		if predicate := definition.defines[name]; predicate != nil && rowRecogExprUsesPrevious(predicate.node()) {
			needsPrevious = true
			break
		}
	}
	if needsPrevious {
		history = append(append([]Event(nil), partition.events...), event)
	}
	for _, name := range names {
		predicate := definition.defines[name]
		if predicate == nil {
			return true
		}
		previous := []Event(nil)
		if needsPrevious {
			previous = rowRecogPreviousHistory(partition, event, history)
		}
		value := predicate.eval(EvalContext{
			Event:           event,
			History:         history,
			PreviousHistory: previous,
			Now:             now,
			Variables:       variables,
		})
		matched, ok := boolValue(value)
		if ok && matched {
			return true
		}
	}
	return false
}

func rowPatternStartVariables(pattern RowPattern) []string {
	minimum, maximum := rowPatternBounds(pattern)
	base := pattern
	if pattern.quantified && !(minimum == 1 && maximum == 1) {
		base.quantified = false
		base.minimum = 1
		base.maximum = 1
	}
	var names []string
	switch base.kind {
	case rowPatternVariable:
		if base.name != "" {
			names = append(names, base.name)
		}
	case rowPatternSequence:
		for _, part := range base.parts {
			names = append(names, rowPatternStartVariables(part)...)
			if !rowPatternNullable(part) {
				break
			}
		}
	case rowPatternAlternation, rowPatternPermutation:
		for _, part := range base.parts {
			names = append(names, rowPatternStartVariables(part)...)
		}
	}
	seen := make(map[string]struct{}, len(names))
	result := make([]string, 0, len(names))
	for _, name := range names {
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}

func rowPatternNullable(pattern RowPattern) bool {
	minimum, _ := rowPatternBounds(pattern)
	if minimum == 0 {
		return true
	}
	base := pattern
	if pattern.quantified && !(pattern.minimum == 1 && pattern.maximum == 1) {
		base.quantified = false
		base.minimum = 1
		base.maximum = 1
	}
	switch base.kind {
	case rowPatternVariable:
		return false
	case rowPatternSequence, rowPatternPermutation:
		for _, part := range base.parts {
			if !rowPatternNullable(part) {
				return false
			}
		}
		return true
	case rowPatternAlternation:
		for _, part := range base.parts {
			if rowPatternNullable(part) {
				return true
			}
		}
	}
	return false
}

func rowPatternContainsAlternation(pattern RowPattern) bool {
	minimum, maximum := rowPatternBounds(pattern)
	base := pattern
	if pattern.quantified && !(minimum == 1 && maximum == 1) {
		base.quantified = false
		base.minimum = 1
		base.maximum = 1
	}
	if base.kind == rowPatternAlternation {
		return true
	}
	for _, part := range base.parts {
		if rowPatternContainsAlternation(part) {
			return true
		}
	}
	return false
}

func (r *statementRuntime) emitRowRecogMatchesAtEnd(definition *rowRecogDefinition, partition *rowRecogPartitionState, end int, plan Plan, now time.Time, batch *ResultBatch) {
	if definition == nil || partition == nil || batch == nil || end < 0 {
		return
	}
	for start := 0; start <= end; start++ {
		key := rowRecogStartKey(start, partition.events[start])
		_, allowedByStatePool := partition.allowedMatchStarts[key]
		if rowRecogStartBlocked(partition, start) && (!r.rowRecogStatePoolTracking(definition) || !allowedByStatePool) {
			continue
		}
		if r.rowRecogStatePoolTracking(definition) {
			if !allowedByStatePool {
				continue
			}
		}
		matches := rowRecogMatchesWithPrevious(definition, partition.events, partition.previousByEvent, start, end, now, r.variables)
		emittedForStart := false
		for _, match := range matches {
			if r.rowRecogStatePoolTracking(definition) {
				if _, allowed := partition.allowedMatchKeys[rowRecogMatchKey(match)]; !allowed {
					continue
				}
			}
			if !rowRecogMatchAllowed(definition, partition, match) {
				continue
			}
			matchKey := rowRecogMatchKey(match)
			if _, exists := partition.emitted[matchKey]; exists {
				continue
			}
			partition.emitted[matchKey] = struct{}{}
			if partition.emittedMatches == nil {
				partition.emittedMatches = make(map[string]rowRecogMatch)
			}
			partition.emittedMatches[matchKey] = match
			if row, visible := evaluateRowRecogMatchWithPrevious(match, partition.events, partition.previousByEvent, plan, now, r.variables); visible {
				batch.New = append(batch.New, resultRow(row))
			}
			emittedForStart = true
			r.advanceRowRecogSkip(definition, partition, match)
			if !definition.allMatches {
				break
			}
		}
		if !definition.allMatches && emittedForStart {
			break
		}
	}
}

func (r *statementRuntime) advanceRowRecogSkip(definition *rowRecogDefinition, partition *rowRecogPartitionState, match rowRecogMatch) {
	if definition == nil || partition == nil {
		return
	}
	// SKIP TO CURRENT ROW keeps already-admitted recognition branches that
	// started before the current match. They can extend on subsequent events
	// and are required for overlapping ALL MATCHES results such as the
	// financial DataSet pattern. The other skip modes deliberately prune
	// branches before their target start position.
	if definition.skip == RowRecogSkipToCurrentRow {
		return
	}
	switch definition.skip {
	case RowRecogSkipToNextRow:
		partition.skipStart = match.start + 1
	case RowRecogSkipToCurrentRow:
		partition.skipStart = match.end
	default:
		partition.skipStart = match.end + 1
	}
	if partition.skipStart < 0 {
		partition.skipStart = 0
	}
	threshold := partition.skipStart
	for index, event := range partition.events {
		if index >= threshold {
			break
		}
		key := rowRecogStartKey(index, event)
		if _, active := partition.activeStarts[key]; active {
			r.releaseRowRecogStart(partition, key)
		}
	}
}

func rowRecogMatchAllowed(definition *rowRecogDefinition, partition *rowRecogPartitionState, match rowRecogMatch) bool {
	if definition == nil || partition == nil {
		return false
	}
	if definition.skip == RowRecogSkipToCurrentRow {
		return true
	}
	return match.start >= partition.skipStart
}

func rowRecogClosedBranchKey(startKey, branch string) string {
	return startKey + "|" + branch
}

func rowRecogMatchBranchClosed(partition *rowRecogPartitionState, startKey string, match rowRecogMatch) bool {
	if partition == nil {
		return true
	}
	if _, closed := partition.intervalClosed[startKey]; closed {
		return true
	}
	_, closed := partition.closedBranches[rowRecogClosedBranchKey(startKey, match.branch)]
	return closed
}

func rowRecogOpenMatches(partition *rowRecogPartitionState, startKey string, matches []rowRecogMatch) []rowRecogMatch {
	if len(matches) == 0 {
		return nil
	}
	result := make([]rowRecogMatch, 0, len(matches))
	for _, match := range matches {
		if !rowRecogMatchBranchClosed(partition, startKey, match) {
			result = append(result, match)
		}
	}
	return result
}

func rowRecogHasOpenBranchAtEnd(definition *rowRecogDefinition, partition *rowRecogPartitionState, startKey string, start int, now time.Time, variables map[string]Value) bool {
	if definition == nil || partition == nil || start < 0 || start >= len(partition.events) {
		return false
	}
	end := len(partition.events) - 1
	if end < start {
		return false
	}
	for _, match := range rowRecogMatchesWithPrevious(definition, partition.events, partition.previousByEvent, start, end, now, variables) {
		if !rowRecogMatchBranchClosed(partition, startKey, match) {
			return true
		}
	}
	return false
}

func rowRecogStartAllowed(definition *rowRecogDefinition, partition *rowRecogPartitionState, start int) bool {
	if definition == nil || partition == nil {
		return false
	}
	if definition.skip == RowRecogSkipToCurrentRow {
		return true
	}
	return start >= partition.skipStart
}

func (r *statementRuntime) flushRowRecogIntervals(definition *rowRecogDefinition, plan Plan, now time.Time, batch *ResultBatch) {
	if definition == nil || !rowRecogHasInterval(definition) || r == nil || r.rowRecogState == nil || batch == nil {
		return
	}
	keys := make([]string, 0, len(r.rowRecogState.partitions))
	for key := range r.rowRecogState.partitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		partition := r.rowRecogState.partitions[key]
		if partition == nil {
			continue
		}
		if partition.intervalClosed == nil {
			partition.intervalClosed = make(map[string]struct{})
		}
		if partition.closedBranches == nil {
			partition.closedBranches = make(map[string]struct{})
		}
		if partition.intervalNotified == nil {
			partition.intervalNotified = make(map[string]struct{})
		}
		if partition.intervalFinal == nil {
			partition.intervalFinal = make(map[string][]rowRecogMatch)
		}
		for start := 0; start < len(partition.events); start++ {
			if !rowRecogStartAllowed(definition, partition, start) {
				continue
			}
			startEvent := partition.events[start]
			if rowRecogStartBlocked(partition, start) {
				continue
			}
			startKey := rowRecogStartKey(start, startEvent)
			if definition.intervalOrTerminated {
				if _, notified := partition.intervalNotified[startKey]; notified {
					continue
				}
			} else {
				if _, closed := partition.intervalClosed[startKey]; closed {
					continue
				}
			}
			deadline := rowRecogDeadline(definition, startEvent.ReceivedAt())
			if deadline.After(now) {
				continue
			}
			last := -1
			for index := start; index < len(partition.events); index++ {
				if partition.events[index].ReceivedAt().After(deadline) {
					break
				}
				last = index
			}
			if last < start {
				continue
			}
			matches := rowRecogMatchesAtOrBeforeWithPrevious(definition, partition.events, partition.previousByEvent, start, last, now, r.variables)
			if definition.intervalOrTerminated && definition.allMatches {
				matches = rowRecogAllMatchesAtOrBeforeWithPrevious(definition, partition.events, partition.previousByEvent, start, last, now, r.variables)
			}
			if definition.intervalOrTerminated {
				matches = rowRecogOpenMatches(partition, startKey, matches)
				partition.intervalNotified[startKey] = struct{}{}
				r.emitRowRecogIntervalSnapshot(definition, partition, matches, plan, now, batch)
				if !definition.allMatches && len(matches) > 0 {
					// Esper schedules all end states with the same match-begin
					// timestamp together. A first-match timer callback emits the
					// ranked earliest branch and consumes the other callbacks for
					// that same deadline; they must not fire when a later event
					// arrives at the exact boundary.
					for candidate := start + 1; candidate < len(partition.events); candidate++ {
						candidateKey := rowRecogStartKey(candidate, partition.events[candidate])
						if rowRecogDeadline(definition, partition.events[candidate].ReceivedAt()).Equal(deadline) {
							partition.intervalNotified[candidateKey] = struct{}{}
						}
					}
					break
				}
				continue
			}
			partition.intervalClosed[startKey] = struct{}{}
			r.emitRowRecogIntervalMatches(definition, partition, startKey, matches, plan, now, batch, true)
			if !definition.allMatches && len(matches) > 0 {
				break
			}
		}
	}
}

func rowRecogDeadline(definition *rowRecogDefinition, started time.Time) time.Time {
	if definition == nil {
		return started
	}
	if definition.intervalCalendar != nil {
		period := definition.intervalCalendar
		return started.AddDate(period.Years, period.Months, period.Days)
	}
	return started.Add(definition.interval)
}

func rowRecogHasInterval(definition *rowRecogDefinition) bool {
	return definition != nil && (definition.interval > 0 || definition.intervalCalendar != nil)
}

// rowRecogBatchWindowNode locates a batch view in the input chain. Batch
// views expose pending rows to iterator snapshots before the boundary, but
// the recognition state itself is rebuilt for each completed batch.
func rowRecogBatchWindowNode(input *streamNode) *streamNode {
	for node := input; node != nil; node = node.input {
		if node.kind != streamWindow {
			continue
		}
		switch node.window.(type) {
		case LengthBatchWindowSpec, TimeBatchWindowSpec, TimeLengthBatchWindowSpec:
			return node
		}
	}
	return nil
}

func (r *statementRuntime) resetRowRecogBatchState() {
	if r == nil {
		return
	}
	if r.engine != nil {
		r.engine.releaseRowRecogRuntimeLocked(r)
	}
	r.rowRecogState = &rowRecogRuntimeState{partitions: make(map[string]*rowRecogPartitionState)}
}

// emitRowRecogTerminated handles the useful common subset of Esper's
// "interval ... or terminated" lifecycle. A match that reaches the maximum
// finite width is terminal immediately. Otherwise, when the newest row no
// longer produces a match, the latest completed match for the same start row
// is terminal and is emitted before the interval deadline.
func (r *statementRuntime) emitRowRecogTerminated(definition *rowRecogDefinition, partition *rowRecogPartitionState, end int, plan Plan, now time.Time, batch *ResultBatch) {
	if r == nil || definition == nil || partition == nil || batch == nil || end < 0 {
		return
	}
	for start := 0; start <= end; start++ {
		if !rowRecogStartAllowed(definition, partition, start) {
			continue
		}
		startKey := rowRecogStartKey(start, partition.events[start])
		_, allowedByStatePool := partition.allowedMatchStarts[startKey]
		if rowRecogStartBlocked(partition, start) && (!r.rowRecogStatePoolTracking(definition) || !allowedByStatePool) {
			continue
		}
		if !definition.intervalOrTerminated {
			if _, closed := partition.intervalClosed[startKey]; closed {
				continue
			}
		}
		current := rowRecogMatchesWithPrevious(definition, partition.events, partition.previousByEvent, start, end, now, r.variables)
		current = rowRecogOpenMatches(partition, startKey, current)
		if r.rowRecogStatePoolTracking(definition) {
			allowed := make([]rowRecogMatch, 0, len(current))
			for _, match := range current {
				if _, exists := partition.allowedMatchKeys[rowRecogMatchKey(match)]; exists {
					allowed = append(allowed, match)
				}
			}
			current = allowed
		}
		terminalCurrent := make([]rowRecogMatch, 0, len(current))
		for _, match := range current {
			if match.terminal || rowRecogMatchAtFiniteWidth(definition.pattern, match) {
				terminalCurrent = append(terminalCurrent, match)
			}
		}
		if len(terminalCurrent) > 0 {
			r.emitRowRecogIntervalMatches(definition, partition, startKey, terminalCurrent, plan, now, batch, true)
			continue
		}
		if end <= start {
			continue
		}
		terminated := rowRecogMatchesAtOrBeforeWithPrevious(definition, partition.events, partition.previousByEvent, start, end-1, now, r.variables)
		if definition.allMatches {
			terminated = rowRecogAllMatchesAtOrBeforeWithPrevious(definition, partition.events, partition.previousByEvent, start, end-1, now, r.variables)
		}
		if len(current) > 0 {
			terminated = rowRecogMatchesForTerminated(definition, partition, startKey, terminated, current)
		}
		if len(terminated) > 0 {
			r.emitRowRecogIntervalMatches(definition, partition, startKey, terminated, plan, now, batch, true)
		}
	}
}

func (r *statementRuntime) emitRowRecogIntervalSnapshot(definition *rowRecogDefinition, partition *rowRecogPartitionState, matches []rowRecogMatch, plan Plan, now time.Time, batch *ResultBatch) {
	if r == nil || definition == nil || partition == nil || batch == nil || len(matches) == 0 {
		return
	}
	for _, match := range matches {
		if !rowRecogMatchAllowed(definition, partition, match) {
			continue
		}
		startKey := rowRecogStartKey(match.start, partition.events[match.start])
		if rowRecogMatchBranchClosed(partition, startKey, match) {
			continue
		}
		if row, visible := evaluateRowRecogMatchWithPrevious(match, partition.events, partition.previousByEvent, plan, now, r.variables); visible {
			batch.New = append(batch.New, resultRow(row))
		}
		if !definition.allMatches {
			break
		}
	}
}

func (r *statementRuntime) emitRowRecogIntervalMatches(definition *rowRecogDefinition, partition *rowRecogPartitionState, startKey string, matches []rowRecogMatch, plan Plan, now time.Time, batch *ResultBatch, closeBranch bool) {
	if r == nil || definition == nil || partition == nil || batch == nil || len(matches) == 0 {
		return
	}
	if partition.intervalClosed == nil {
		partition.intervalClosed = make(map[string]struct{})
	}
	if partition.intervalFinal == nil {
		partition.intervalFinal = make(map[string][]rowRecogMatch)
	}
	selected := make([]rowRecogMatch, 0, len(matches))
	for _, match := range matches {
		if !rowRecogMatchAllowed(definition, partition, match) {
			continue
		}
		if definition.intervalOrTerminated {
			if rowRecogMatchBranchClosed(partition, startKey, match) {
				continue
			}
		}
		selected = append(selected, match)
		if !definition.allMatches {
			break
		}
	}
	if len(selected) == 0 {
		return
	}
	if closeBranch {
		if definition.intervalOrTerminated {
			if partition.closedBranches == nil {
				partition.closedBranches = make(map[string]struct{})
			}
			endMatches := rowRecogMatchesWithPrevious(definition, partition.events, partition.previousByEvent, selected[0].start, len(partition.events)-1, now, r.variables)
			for _, match := range selected {
				continues := false
				for _, later := range endMatches {
					if rowRecogMatchExtends(match, later) {
						continues = true
						break
					}
				}
				if !continues {
					partition.closedBranches[rowRecogClosedBranchKey(startKey, match.branch)] = struct{}{}
				}
			}
		} else {
			partition.intervalClosed[startKey] = struct{}{}
		}
	}
	for _, match := range selected {
		matchKey := rowRecogMatchKey(match)
		if _, exists := partition.emitted[matchKey]; exists {
			continue
		}
		partition.emitted[matchKey] = struct{}{}
		if partition.emittedMatches == nil {
			partition.emittedMatches = make(map[string]rowRecogMatch)
		}
		partition.emittedMatches[matchKey] = match
		partition.intervalFinal[startKey] = append(partition.intervalFinal[startKey], match)
		if row, visible := evaluateRowRecogMatchWithPrevious(match, partition.events, partition.previousByEvent, plan, now, r.variables); visible {
			batch.New = append(batch.New, resultRow(row))
		}
	}
	if closeBranch {
		last := selected[0]
		for _, match := range selected[1:] {
			if match.end > last.end {
				last = match
			}
		}
		if definition.intervalOrTerminated && rowRecogHasOpenBranchAtEnd(definition, partition, startKey, last.start, now, r.variables) {
			return
		}
		r.advanceRowRecogSkip(definition, partition, last)
	}
}

func rowRecogMatchAtFiniteWidth(pattern RowPattern, match rowRecogMatch) bool {
	maximum, bounded := rowPatternMaxWidth(pattern)
	return bounded && maximum > 0 && match.end-match.start+1 >= maximum
}

func rowPatternMaxWidth(pattern RowPattern) (int, bool) {
	base := pattern
	if pattern.quantified && !(pattern.minimum == 1 && pattern.maximum == 1) {
		base.quantified = false
		base.minimum = 1
		base.maximum = 1
	}
	var width int
	var bounded bool
	switch base.kind {
	case rowPatternVariable:
		width, bounded = 1, true
	case rowPatternSequence, rowPatternPermutation:
		bounded = true
		for _, part := range base.parts {
			partWidth, partBounded := rowPatternMaxWidth(part)
			width += partWidth
			bounded = bounded && partBounded
		}
	case rowPatternAlternation:
		bounded = true
		for _, part := range base.parts {
			partWidth, partBounded := rowPatternMaxWidth(part)
			if partWidth > width {
				width = partWidth
			}
			bounded = bounded && partBounded
		}
	default:
		return 0, false
	}
	if pattern.quantified && !(pattern.minimum == 1 && pattern.maximum == 1) {
		if pattern.maximum == 0 {
			return 0, false
		}
		if !bounded || width > int(^uint(0)>>1)/pattern.maximum {
			return 0, false
		}
		width *= pattern.maximum
	}
	return width, bounded
}

func rowRecogMatchesAtOrBefore(definition *rowRecogDefinition, history []Event, start, last int, now time.Time, variables map[string]Value) []rowRecogMatch {
	return rowRecogMatchesAtOrBeforeWithPrevious(definition, history, nil, start, last, now, variables)
}

func rowRecogMatchesAtOrBeforeWithPrevious(definition *rowRecogDefinition, history []Event, previousByEvent map[string][]Event, start, last int, now time.Time, variables map[string]Value) []rowRecogMatch {
	for end := last; end >= start; end-- {
		matches := rowRecogMatchesWithPrevious(definition, history, previousByEvent, start, end, now, variables)
		if len(matches) > 0 {
			return matches
		}
	}
	return nil
}

func rowRecogAllMatchesAtOrBeforeWithPrevious(definition *rowRecogDefinition, history []Event, previousByEvent map[string][]Event, start, last int, now time.Time, variables map[string]Value) []rowRecogMatch {
	if definition == nil || start < 0 || last < start || start >= len(history) {
		return nil
	}
	result := make([]rowRecogMatch, 0)
	seen := make(map[string]struct{})
	for end := last; end >= start; end-- {
		for _, match := range rowRecogMatchesWithPrevious(definition, history, previousByEvent, start, end, now, variables) {
			key := rowRecogMatchKey(match)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, match)
		}
	}
	return result
}

func rowRecogMatchesNotExtendedByCurrent(previous, current []rowRecogMatch) []rowRecogMatch {
	if len(previous) == 0 || len(current) == 0 {
		return previous
	}
	result := make([]rowRecogMatch, 0, len(previous))
	for _, candidate := range previous {
		extended := false
		for _, later := range current {
			if rowRecogMatchExtends(candidate, later) {
				extended = true
				break
			}
		}
		if !extended {
			result = append(result, candidate)
		}
	}
	return result
}

func rowRecogMatchesForTerminated(definition *rowRecogDefinition, partition *rowRecogPartitionState, startKey string, previous, current []rowRecogMatch) []rowRecogMatch {
	if definition == nil || partition == nil || len(previous) == 0 || len(current) == 0 {
		return previous
	}
	if !rowPatternContainsAlternation(definition.pattern) {
		return rowRecogMatchesNotExtendedByCurrent(previous, current)
	}
	if partition.alternateNotified == nil {
		partition.alternateNotified = make(map[string]struct{})
	}
	result := make([]rowRecogMatch, 0, len(previous))
	for _, candidate := range previous {
		extended := false
		for _, later := range current {
			if rowRecogMatchExtends(candidate, later) {
				extended = true
				break
			}
		}
		if !extended {
			result = append(result, candidate)
			continue
		}
		key := rowRecogClosedBranchKey(startKey, candidate.branch)
		if _, notified := partition.alternateNotified[key]; notified {
			continue
		}
		partition.alternateNotified[key] = struct{}{}
		result = append(result, candidate)
	}
	return result
}

func rowRecogMatchExtends(previous, current rowRecogMatch) bool {
	if previous.start != current.start || current.end <= previous.end || previous.branch != current.branch {
		return false
	}
	for name, events := range previous.captures {
		later := current.captures[name]
		if len(later) < len(events) {
			return false
		}
		for index, event := range events {
			if !sameEvent(event, later[index]) {
				return false
			}
		}
	}
	return true
}

func rowRecogStartKey(index int, event Event) string {
	return fmt.Sprintf("%d:%s:%d", index, eventIdentity(event), event.ReceivedAt().UnixNano())
}

func rowRecogMatchContainsEvent(match rowRecogMatch, event Event) bool {
	for _, captured := range match.captures {
		for _, candidate := range captured {
			if sameEvent(candidate, event) {
				return true
			}
		}
	}
	return false
}

func remapRowRecogEmittedMatches(matches map[string]rowRecogMatch, oldEvents []Event, removed Event, retained []Event) map[string]rowRecogMatch {
	result := make(map[string]rowRecogMatch)
	for _, match := range matches {
		if match.start < 0 || match.end < match.start || match.end >= len(oldEvents) || rowRecogMatchContainsEvent(match, removed) {
			continue
		}
		startEvent := oldEvents[match.start]
		endEvent := oldEvents[match.end]
		newStart, newEnd := -1, -1
		for index, candidate := range retained {
			if newStart < 0 && sameEvent(candidate, startEvent) {
				newStart = index
			}
			if sameEvent(candidate, endEvent) {
				newEnd = index
			}
		}
		if newStart < 0 || newEnd < newStart {
			continue
		}
		match.start = newStart
		match.end = newEnd
		result[rowRecogMatchKey(match)] = match
	}
	return result
}

func rowRecogSkipPastLastSnapshotStartExcluded(partition *rowRecogPartitionState, match rowRecogMatch, strict bool) bool {
	if partition == nil || match.start < 0 || match.start >= len(partition.events) {
		return false
	}
	if strict {
		// The emitted history is added to the iterator before discovering any
		// current matches. New starts before the skip boundary are therefore
		// already consumed by SKIP PAST LAST ROW, including starts between an
		// emitted match's first and last event. Comparing only the start event
		// to the emitted end event would incorrectly expose suffix matches such
		// as (A* B) starting at the second A.
		return match.start < partition.skipStart
	}
	startEvent := partition.events[match.start]
	for _, emitted := range partition.emittedMatches {
		if emitted.end < 0 || emitted.end >= len(partition.events) {
			continue
		}
		if sameEvent(startEvent, partition.events[emitted.end]) {
			return true
		}
	}
	return false
}

func rowRecogStartBlocked(partition *rowRecogPartitionState, index int) bool {
	if partition == nil || index < 0 || index >= len(partition.events) {
		return false
	}
	_, blocked := partition.blockedStarts[rowRecogStartKey(index, partition.events[index])]
	return blocked
}

func (r *statementRuntime) removeRowRecogEvent(definition *rowRecogDefinition, event Event, now time.Time) {
	if r == nil || r.rowRecogState == nil || definition == nil {
		return
	}
	key := rowRecogPartitionKey(definition, event, nil, now, r.variables)
	partition := r.rowRecogState.partitions[key]
	if partition == nil {
		return
	}
	for index, retained := range partition.events {
		if !sameEvent(retained, event) {
			continue
		}
		oldEvents := append([]Event(nil), partition.events...)
		oldEmittedMatches := partition.emittedMatches
		oldActive := partition.activeStarts
		oldActiveCounts := partition.activeStateCounts
		oldActivePaths := partition.activePaths
		oldBlocked := partition.blockedStarts
		removedKey := rowRecogStartKey(index, retained)
		if _, active := oldActive[removedKey]; active {
			amount := int64(1)
			if oldActiveCounts != nil && oldActiveCounts[removedKey] > 0 {
				amount = oldActiveCounts[removedKey]
			}
			if r.engine != nil && r.engine.matchRecognizeStatePool != nil {
				r.engine.matchRecognizeStatePool.decrease(r.rowRecogOwner, amount)
			}
		}
		if partition.previousByEvent != nil {
			delete(partition.previousByEvent, rowRecogPreviousEventKey(retained))
		}
		partition.events = append(partition.events[:index], partition.events[index+1:]...)
		partition.activeStarts = make(map[string]struct{})
		partition.activeStateCounts = make(map[string]int64)
		partition.activePaths = make(map[string][]rowRecogNFAPath)
		partition.allowedMatchStarts = make(map[string]struct{})
		partition.allowedMatchKeys = make(map[string]struct{})
		partition.blockedStarts = make(map[string]struct{})
		for newIndex, candidate := range partition.events {
			oldIndex := newIndex
			if newIndex >= index {
				oldIndex++
			}
			oldKey := rowRecogStartKey(oldIndex, candidate)
			newKey := rowRecogStartKey(newIndex, candidate)
			if _, active := oldActive[oldKey]; active {
				partition.activeStarts[newKey] = struct{}{}
				if oldActiveCounts != nil {
					partition.activeStateCounts[newKey] = oldActiveCounts[oldKey]
				}
				if oldActivePaths != nil {
					partition.activePaths[newKey] = append([]rowRecogNFAPath(nil), oldActivePaths[oldKey]...)
				}
			}
			if _, blocked := oldBlocked[oldKey]; blocked {
				partition.blockedStarts[newKey] = struct{}{}
			}
		}
		if index < partition.skipStart {
			partition.skipStart--
		}
		if partition.skipStart < 0 {
			partition.skipStart = 0
		}
		if partition.skipStart > len(partition.events) {
			partition.skipStart = len(partition.events)
		}
		// Match keys contain positional information. A retention change can
		// shift later rows, so rebuild the emitted-match history against the
		// current retained view. Matches containing the removed event disappear,
		// while already-emitted matches whose events remain stay visible to the
		// SKIP PAST LAST ROW iterator.
		partition.emitted = make(map[string]struct{})
		partition.emittedMatches = remapRowRecogEmittedMatches(oldEmittedMatches, oldEvents, retained, partition.events)
		partition.intervalClosed = make(map[string]struct{})
		partition.closedBranches = make(map[string]struct{})
		partition.alternateNotified = make(map[string]struct{})
		partition.intervalNotified = make(map[string]struct{})
		partition.intervalFinal = make(map[string][]rowRecogMatch)
		if len(partition.events) == 0 && len(partition.previousRolling) == 0 {
			delete(r.rowRecogState.partitions, key)
		}
		return
	}
}

func rowRecogPartitionKey(definition *rowRecogDefinition, event Event, history []Event, now time.Time, variables map[string]Value) string {
	if definition == nil || len(definition.partition) == 0 {
		return "<all>"
	}
	parts := make([]any, 0, len(definition.partition)*2)
	for _, expression := range definition.partition {
		if expression == nil {
			parts = append(parts, "<nil>")
			continue
		}
		value := expression.eval(EvalContext{Event: event, History: history, Now: now, Variables: variables})
		parts = append(parts, value.State(), value.Any())
	}
	return encodeKey(parts)
}

func rowRecogMatches(definition *rowRecogDefinition, history []Event, start, end int, now time.Time, variables map[string]Value) []rowRecogMatch {
	return rowRecogMatchesWithPrevious(definition, history, nil, start, end, now, variables)
}

func rowRecogFastABStarCDefinitionFor(definition *rowRecogDefinition) (rowRecogFastABStarCDefinition, bool) {
	if definition == nil || !definition.allMatches || definition.iterateOnly || definition.skip != RowRecogSkipPastLastRow ||
		rowRecogHasInterval(definition) || rowRecogBatchWindowNode(definition.input) != nil || definition.pattern.kind != rowPatternSequence ||
		len(definition.pattern.parts) != 3 {
		return rowRecogFastABStarCDefinition{}, false
	}
	first, repeated, terminal := definition.pattern.parts[0], definition.pattern.parts[1], definition.pattern.parts[2]
	firstMinimum, firstMaximum := rowPatternBounds(first)
	repeatedMinimum, repeatedMaximum := rowPatternBounds(repeated)
	terminalMinimum, terminalMaximum := rowPatternBounds(terminal)
	if first.kind != rowPatternVariable || repeated.kind != rowPatternVariable || terminal.kind != rowPatternVariable ||
		firstMinimum != 1 || firstMaximum != 1 || repeatedMinimum != 0 || repeatedMaximum != 0 ||
		terminalMinimum != 1 || terminalMaximum != 1 || repeated.greedy || first.name == "" || repeated.name == "" || terminal.name == "" {
		return rowRecogFastABStarCDefinition{}, false
	}
	// A repeated predicate that reads its own tag depends on the complete
	// capture array. The fast path intentionally keeps only the first event
	// and derives the repeated slice at terminal time, so leave such patterns
	// on the general matcher.
	for _, name := range []string{first.name, repeated.name, terminal.name} {
		predicate := definition.defines[name]
		if predicate != nil && (rowRecogExprUsesPrevious(predicate.node()) || (name == repeated.name && rowRecogExprUsesTag(predicate.node(), repeated.name))) {
			return rowRecogFastABStarCDefinition{}, false
		}
	}
	return rowRecogFastABStarCDefinition{a: first.name, b: repeated.name, c: terminal.name}, true
}

func rowRecogExprUsesTag(node *exprNode, tag string) bool {
	if node == nil || tag == "" {
		return false
	}
	if node.tagName == tag {
		return true
	}
	for _, child := range node.children {
		if rowRecogExprUsesTag(child, tag) {
			return true
		}
	}
	return false
}

func rowRecogExprUsesPrevious(node *exprNode) bool {
	if node == nil {
		return false
	}
	if node.kind == "prev" || node.kind == "prior" {
		return true
	}
	for _, child := range node.children {
		if rowRecogExprUsesPrevious(child) {
			return true
		}
	}
	return false
}

func rowRecogFastDefineMatches(definition *rowRecogDefinition, variable string, event Event, partition *rowRecogPartitionState, captures map[string][]Event, now time.Time, variables map[string]Value) bool {
	if definition == nil || partition == nil {
		return false
	}
	predicate := definition.defines[variable]
	if predicate == nil {
		return true
	}
	history := partition.events
	previous := []Event(nil)
	if rowRecogExprUsesPrevious(predicate.node()) {
		previous = rowRecogPreviousHistory(partition, event, history)
	}
	value := predicate.eval(EvalContext{
		Event:           event,
		History:         history,
		PreviousHistory: previous,
		Tags:            rowRecogLastTags(captures),
		TagValues:       cloneRowRecogCaptures(captures),
		Now:             now,
		Variables:       variables,
	})
	matched, ok := boolValue(value)
	return ok && matched
}

func rowRecogFastABStarCCaptures(path rowRecogFastABStarCPath, partition *rowRecogPartitionState, end int, definition rowRecogFastABStarCDefinition) map[string][]Event {
	captures := map[string][]Event{definition.a: []Event{path.a}}
	if path.start+1 < end {
		captures[definition.b] = append([]Event(nil), partition.events[path.start+1:end]...)
	}
	captures[definition.c] = []Event{partition.events[end]}
	return captures
}

func (r *statementRuntime) dropRowRecogFastABStarCPath(partition *rowRecogPartitionState, path rowRecogFastABStarCPath) {
	if r == nil || partition == nil || path.start < 0 || path.start >= len(partition.events) {
		return
	}
	key := rowRecogStartKey(path.start, partition.events[path.start])
	if _, active := partition.activeStarts[key]; !active {
		return
	}
	r.releaseRowRecogStart(partition, key)
}

func (r *statementRuntime) emitRowRecogFastABStarC(definition *rowRecogDefinition, partition *rowRecogPartitionState, end int, plan Plan, now time.Time, batch *ResultBatch, fast rowRecogFastABStarCDefinition) {
	if r == nil || definition == nil || partition == nil || batch == nil || end < 0 || end >= len(partition.events) {
		return
	}
	event := partition.events[end]
	next := make([]rowRecogFastABStarCPath, 0, len(partition.fastABStarC))
	for _, path := range partition.fastABStarC {
		if path.start < 0 || path.start >= end {
			continue
		}
		baseCaptures := map[string][]Event{fast.a: []Event{path.a}}
		cCaptures := baseCaptures
		if predicate := definition.defines[fast.c]; predicate != nil && rowRecogExprUsesTag(predicate.node(), fast.b) {
			cCaptures = rowRecogFastABStarCCaptures(path, partition, end, rowRecogFastABStarCDefinition{a: fast.a, b: fast.b, c: ""})
			delete(cCaptures, "")
		}
		if rowRecogFastDefineMatches(definition, fast.c, event, partition, cCaptures, now, r.variables) {
			captures := rowRecogFastABStarCCaptures(path, partition, end, fast)
			match := rowRecogMatch{
				start:    path.start,
				end:      end,
				terminal: true,
				branch:   rowPatternBranchKey(definition.pattern),
				captures: captures,
			}
			key := rowRecogMatchKey(match)
			if _, emitted := partition.emitted[key]; !emitted {
				partition.emitted[key] = struct{}{}
				if partition.emittedMatches == nil {
					partition.emittedMatches = make(map[string]rowRecogMatch)
				}
				partition.emittedMatches[key] = match
				if row, visible := evaluateRowRecogMatchWithPrevious(match, partition.events, partition.previousByEvent, plan, now, r.variables); visible {
					batch.New = append(batch.New, resultRow(row))
				}
			}
			// The fast path is restricted to SKIP PAST LAST ROW, therefore the
			// first terminal branch closes every earlier in-flight start just as
			// the general matcher does.
			r.advanceRowRecogSkip(definition, partition, match)
			partition.fastABStarC = nil
			return
		}
		bCaptures := baseCaptures
		if rowRecogFastDefineMatches(definition, fast.b, event, partition, bCaptures, now, r.variables) {
			next = append(next, path)
			continue
		}
		r.dropRowRecogFastABStarCPath(partition, path)
	}
	partition.fastABStarC = next
	if key := rowRecogStartKey(end, event); func() bool {
		_, active := partition.activeStarts[key]
		return active
	}() {
		partition.fastABStarC = append(partition.fastABStarC, rowRecogFastABStarCPath{start: end, a: event})
	}
}

func rowRecogMatchesWithPrevious(definition *rowRecogDefinition, history []Event, previousByEvent map[string][]Event, start, end int, now time.Time, variables map[string]Value) []rowRecogMatch {
	if definition == nil || start < 0 || end < start || end >= len(history) {
		return nil
	}
	matcher := &rowPatternMatchContext{
		definition:      definition,
		history:         history,
		previousByEvent: previousByEvent,
		end:             end,
		now:             now,
		variables:       variables,
		limit:           definition.maxStates,
	}
	paths := matcher.match(definition.pattern, start, nil)
	result := make([]rowRecogMatch, 0, len(paths))
	for _, path := range paths {
		if path.position != end+1 {
			continue
		}
		result = append(result, rowRecogMatch{
			start:    start,
			end:      end,
			terminal: path.terminal,
			branch:   path.branch,
			captures: cloneRowRecogCaptures(path.captures),
		})
		if definition.maxStates > 0 && len(result) >= definition.maxStates {
			break
		}
	}
	return result
}

const maxRowRecogPatternSteps = 1 << 20

type rowPatternMatchPath struct {
	position int
	terminal bool
	branch   string
	captures map[string][]Event
}

func rowPatternBranchKey(pattern RowPattern) string {
	minimum, maximum := rowPatternBounds(pattern)
	repeated := pattern.quantified && !(minimum == 1 && maximum == 1)
	base := pattern
	if repeated {
		base.quantified = false
		base.minimum = 1
		base.maximum = 1
	}
	var key string
	switch base.kind {
	case rowPatternVariable:
		key = "var:" + base.name
	case rowPatternSequence:
		parts := make([]string, 0, len(base.parts))
		for _, part := range base.parts {
			parts = append(parts, rowPatternBranchKey(part))
		}
		key = "seq/" + strings.Join(parts, "/")
	case rowPatternAlternation:
		parts := make([]string, 0, len(base.parts))
		for _, part := range base.parts {
			parts = append(parts, rowPatternBranchKey(part))
		}
		key = "alt/" + strings.Join(parts, "|")
	case rowPatternPermutation:
		parts := make([]string, 0, len(base.parts))
		for _, part := range base.parts {
			parts = append(parts, rowPatternBranchKey(part))
		}
		key = "perm/" + strings.Join(parts, "|")
	default:
		key = "unknown"
	}
	if repeated {
		return fmt.Sprintf("repeat[%d,%d]/%s", pattern.minimum, pattern.maximum, key)
	}
	return key
}

func rowPatternJoinBranch(left, right string) string {
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	return left + "/" + right
}

type rowPatternMatchContext struct {
	definition      *rowRecogDefinition
	history         []Event
	previousByEvent map[string][]Event
	end             int
	now             time.Time
	variables       map[string]Value
	limit           int
	steps           int
}

func (m *rowPatternMatchContext) match(pattern RowPattern, position int, captures map[string][]Event) []rowPatternMatchPath {
	if m == nil || m.definition == nil || position > m.end+1 || !m.step() {
		return nil
	}
	minimum, maximum := rowPatternBounds(pattern)
	if pattern.quantified && !(minimum == 1 && maximum == 1) {
		base := pattern
		base.quantified = false
		base.minimum = 1
		base.maximum = 1
		return m.repeat(base, position, captures, minimum, maximum, pattern.greedy)
	}
	if pattern.kind == rowPatternVariable {
		return m.repeat(pattern, position, captures, minimum, maximum, pattern.greedy)
	}
	return m.matchBase(pattern, position, captures)
}

func (m *rowPatternMatchContext) matchBase(pattern RowPattern, position int, captures map[string][]Event) []rowPatternMatchPath {
	if m == nil || !m.step() {
		return nil
	}
	switch pattern.kind {
	case rowPatternVariable:
		if position > m.end || position >= len(m.history) {
			return nil
		}
		event := m.history[position]
		next := cloneRowRecogCaptures(captures)
		next[pattern.name] = append(next[pattern.name], event)
		predicate := m.definition.defines[pattern.name]
		if predicate != nil {
			value := predicate.eval(EvalContext{
				Event:           event,
				History:         append([]Event(nil), m.history[:position+1]...),
				PreviousHistory: rowRecogPreviousHistoryByMap(m.previousByEvent, event, m.history[:position+1]),
				Tags:            rowRecogLastTags(next),
				TagValues:       cloneRowRecogCaptures(next),
				Now:             m.now,
				Variables:       m.variables,
			})
			matched, ok := boolValue(value)
			if !ok || !matched {
				return nil
			}
		}
		return []rowPatternMatchPath{{position: position + 1, terminal: true, branch: rowPatternBranchKey(pattern), captures: next}}
	case rowPatternSequence:
		paths := []rowPatternMatchPath{{position: position, terminal: true, branch: "seq", captures: cloneRowRecogCaptures(captures)}}
		for _, part := range pattern.parts {
			next := make([]rowPatternMatchPath, 0)
			for _, path := range paths {
				for _, candidate := range m.match(part, path.position, path.captures) {
					candidate.terminal = path.terminal && candidate.terminal
					candidate.branch = rowPatternJoinBranch(path.branch, candidate.branch)
					next = m.appendLimited(next, []rowPatternMatchPath{candidate})
					if m.exhausted(len(next)) {
						break
					}
				}
				if m.exhausted(len(next)) {
					break
				}
			}
			paths = next
			if len(paths) == 0 {
				return nil
			}
		}
		return paths
	case rowPatternAlternation:
		result := make([]rowPatternMatchPath, 0)
		for index, part := range pattern.parts {
			for _, candidate := range m.match(part, position, captures) {
				candidate.branch = fmt.Sprintf("alt%d/%s", index, candidate.branch)
				result = m.appendLimited(result, []rowPatternMatchPath{candidate})
				if m.exhausted(len(result)) {
					break
				}
			}
			if m.exhausted(len(result)) {
				break
			}
		}
		return result
	case rowPatternPermutation:
		result := make([]rowPatternMatchPath, 0)
		for _, order := range rowPatternPermutationOrders(pattern.parts) {
			paths := []rowPatternMatchPath{{position: position, terminal: true, branch: "perm", captures: cloneRowRecogCaptures(captures)}}
			for _, part := range order {
				next := make([]rowPatternMatchPath, 0)
				for _, path := range paths {
					for _, candidate := range m.match(part, path.position, path.captures) {
						candidate.terminal = path.terminal && candidate.terminal
						candidate.branch = rowPatternJoinBranch(path.branch, candidate.branch)
						next = m.appendLimited(next, []rowPatternMatchPath{candidate})
						if m.exhausted(len(next)) {
							break
						}
					}
					if m.exhausted(len(next)) {
						break
					}
				}
				paths = next
				if len(paths) == 0 {
					break
				}
			}
			result = m.appendLimited(result, paths)
			if m.exhausted(len(result)) {
				break
			}
		}
		return result
	default:
		return nil
	}
}

func (m *rowPatternMatchContext) repeat(base RowPattern, position int, captures map[string][]Event, minimum, maximum int, greedy bool) []rowPatternMatchPath {
	unbounded := maximum == 0
	branchPrefix := ""
	if !(minimum == 1 && maximum == 1) {
		branchPrefix = fmt.Sprintf("repeat[%d,%d]/", minimum, maximum)
	}
	if maximum == 0 {
		maximum = m.end - position + 1
		if maximum < minimum {
			maximum = minimum
		}
	}
	levels := make([][]rowPatternMatchPath, 0, maximum+1)
	active := []rowPatternMatchPath{{
		position: position,
		terminal: !unbounded && minimum == 0,
		branch:   branchPrefix + rowPatternBranchKey(base),
		captures: cloneRowRecogCaptures(captures),
	}}
	for count := 0; count <= maximum; count++ {
		if count >= minimum {
			levels = append(levels, active)
		}
		if count == maximum || len(active) == 0 {
			break
		}
		next := make([]rowPatternMatchPath, 0)
		for _, path := range active {
			paths := m.matchBase(base, path.position, path.captures)
			for _, candidate := range paths {
				// Repeating a zero-width group forever is not a useful NFA
				// state. Keep the zero-repeat result but require progress for
				// subsequent iterations.
				if candidate.position == path.position {
					continue
				}
				candidate.terminal = candidate.terminal && !unbounded
				candidate.branch = branchPrefix + candidate.branch
				next = m.appendLimited(next, []rowPatternMatchPath{candidate})
				if m.exhausted(len(next)) {
					break
				}
			}
			if m.exhausted(len(next)) {
				break
			}
		}
		active = next
	}
	result := make([]rowPatternMatchPath, 0)
	if greedy {
		for index := len(levels) - 1; index >= 0; index-- {
			result = m.appendLimited(result, levels[index])
			if m.exhausted(len(result)) {
				break
			}
		}
		return result
	}
	for _, level := range levels {
		result = m.appendLimited(result, level)
		if m.exhausted(len(result)) {
			break
		}
	}
	return result
}

func rowPatternBounds(pattern RowPattern) (int, int) {
	if !pattern.quantified && pattern.kind != rowPatternVariable {
		return 1, 1
	}
	if !pattern.quantified && pattern.kind == rowPatternVariable {
		return 1, 1
	}
	return pattern.minimum, pattern.maximum
}

func (m *rowPatternMatchContext) step() bool {
	m.steps++
	return m.steps <= maxRowRecogPatternSteps
}

func (m *rowPatternMatchContext) exhausted(size int) bool {
	return m.limit > 0 && size >= m.limit
}

func (m *rowPatternMatchContext) appendLimited(dst, src []rowPatternMatchPath) []rowPatternMatchPath {
	if len(src) == 0 {
		return dst
	}
	if m.limit <= 0 {
		return append(dst, src...)
	}
	remaining := m.limit - len(dst)
	if remaining <= 0 {
		return dst
	}
	if len(src) > remaining {
		src = src[:remaining]
	}
	return append(dst, src...)
}

func cloneRowRecogCaptures(captures map[string][]Event) map[string][]Event {
	if len(captures) == 0 {
		return make(map[string][]Event)
	}
	result := make(map[string][]Event, len(captures))
	for name, events := range captures {
		result[name] = append([]Event(nil), events...)
	}
	return result
}

func rowRecogLastTags(captures map[string][]Event) map[string]Event {
	if len(captures) == 0 {
		return nil
	}
	result := make(map[string]Event, len(captures))
	for name, events := range captures {
		if len(events) > 0 {
			result[name] = events[len(events)-1]
		}
	}
	return result
}

func rowRecogMatchKey(match rowRecogMatch) string {
	names := make([]string, 0, len(match.captures))
	for name := range match.captures {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := []string{fmt.Sprintf("%d:%d", match.start, match.end)}
	for _, name := range names {
		parts = append(parts, name)
		for _, event := range match.captures[name] {
			parts = append(parts, eventIdentity(event))
		}
	}
	return strings.Join(parts, "|")
}

func evaluateRowRecogMatch(match rowRecogMatch, history []Event, plan Plan, now time.Time, variables map[string]Value) (Row, bool) {
	return evaluateRowRecogMatchWithPrevious(match, history, nil, plan, now, variables)
}

func evaluateRowRecogMatchWithPrevious(match rowRecogMatch, history []Event, previousByEvent map[string][]Event, plan Plan, now time.Time, variables map[string]Value) (Row, bool) {
	if len(plan.query.patternSelections) == 0 || match.end < 0 || match.end >= len(history) {
		return Row{}, false
	}
	currentHistory := append([]Event(nil), history[:match.end+1]...)
	ctx := EvalContext{
		Event:           history[match.end],
		Group:           append([]Event(nil), history[match.start:match.end+1]...),
		History:         currentHistory,
		PreviousHistory: rowRecogPreviousHistoryByMap(previousByEvent, history[match.end], currentHistory),
		Tags:            rowRecogLastTags(match.captures),
		TagValues:       cloneRowRecogCaptures(match.captures),
		Now:             now,
		Variables:       variables,
	}
	values := make([]Value, 0, len(plan.query.patternSelections))
	for _, selection := range plan.query.patternSelections {
		values = append(values, selection.Expr.eval(ctx))
	}
	return newRow(plan.resultSchema, values), true
}

func (r *statementRuntime) snapshotQuery(plan Plan, now time.Time, variables map[string]Value) ResultBatch {
	if r == nil {
		return ResultBatch{}
	}
	r.variables = r.withContextVariables(variables)
	r.variables = r.withContextProperties(r.variables)
	variables = r.variables
	if plan.query.rowRecog != nil {
		return r.snapshotRowRecog(plan, now, variables)
	}
	return r.snapshotBatch(plan, now)
}

func (r *statementRuntime) snapshotRowRecog(plan Plan, now time.Time, variables map[string]Value) ResultBatch {
	definition := plan.query.rowRecog
	if definition == nil {
		return ResultBatch{Time: now}
	}
	// Esper does not expose an iterator for match-recognize directly over an
	// unbound event stream. The runtime still evaluates incoming events and
	// dispatches listener matches, but there is no retained data-window view
	// for Statement.Snapshot to enumerate. Filters may sit between the source
	// and MATCH_RECOGNIZE, so inspect the complete input chain.
	if rowRecogInputIsUnbound(definition.input) {
		return ResultBatch{Time: now}
	}
	partitions := make(map[string]*rowRecogPartitionState)
	if r != nil && r.rowRecogState != nil {
		partitions = r.rowRecogState.partitions
	}
	if batchNode := rowRecogBatchWindowNode(definition.input); batchNode != nil {
		partitions = r.rowRecogBatchPendingPartitions(definition, batchNode, now, variables)
	}
	keys := make([]string, 0, len(partitions))
	for key := range partitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := ResultBatch{Time: now}
	seen := make(map[string]struct{})
	for _, key := range keys {
		partition := partitions[key]
		if partition == nil {
			continue
		}
		matches := rowRecogCurrentMatches(definition, partition, now, variables)
		for _, match := range matches {
			matchKey := rowRecogMatchKey(match)
			if _, exists := seen[matchKey]; exists {
				continue
			}
			seen[matchKey] = struct{}{}
			if row, visible := evaluateRowRecogMatchWithPrevious(match, partition.events, partition.previousByEvent, plan, now, variables); visible {
				result.New = append(result.New, resultRow(row))
			}
		}
	}
	if plan.query.distinct {
		unique := make([]Result, 0, len(result.New))
		keys := make(map[string]struct{}, len(result.New))
		for _, row := range result.New {
			key := resultKey(row)
			if _, exists := keys[key]; exists {
				continue
			}
			keys[key] = struct{}{}
			unique = append(unique, row)
		}
		result.New = unique
	}
	result.New = orderRowRecogResults(result.New, plan.query.orderBy, now, variables)
	result.New = applyResultWindow(result.New, plan.query)
	return result
}

func rowRecogInputIsUnbound(input *streamNode) bool {
	for node := input; node != nil; node = node.input {
		switch node.kind {
		case streamWindow, streamNamedWindow, streamTable, streamHistorical, streamMethod:
			return false
		case streamSource:
			return true
		}
	}
	return true
}

// rowRecogBatchPendingPartitions reconstructs the recognition input visible
// to an iterator while a batch window is collecting its next batch. Esper's
// match-recognize iterator exposes the pending batch before the boundary, but
// after the boundary the recognition state is cleared even though the window
// retains the just-flushed batch for ordinary window consumers.
func (r *statementRuntime) rowRecogBatchPendingPartitions(definition *rowRecogDefinition, batchNode *streamNode, now time.Time, variables map[string]Value) map[string]*rowRecogPartitionState {
	partitions := make(map[string]*rowRecogPartitionState)
	if r == nil || definition == nil || batchNode == nil {
		return partitions
	}
	state := r.windows[batchNode]
	if state == nil || len(state.pendingNew) == 0 {
		return partitions
	}

	// A filter downstream of the batch node has not been applied to pendingNew
	// yet. Walk from the row-recognition input back to the batch node and apply
	// those filters in upstream-to-downstream order.
	var filters []Expr
	for node := definition.input; node != nil && node != batchNode; node = node.input {
		if node.kind == streamFilter && node.predicate != nil {
			filters = append(filters, node.predicate)
		}
	}

	for _, stored := range state.pendingNew {
		event := stored.event
		accepted := true
		for index := len(filters) - 1; index >= 0; index-- {
			value := filters[index].eval(EvalContext{Event: event, Now: now, Variables: variables})
			matched, ok := boolValue(value)
			if !ok || !matched {
				accepted = false
				break
			}
		}
		if !accepted {
			continue
		}
		key := rowRecogPartitionKey(definition, event, nil, now, variables)
		partition := partitions[key]
		if partition == nil {
			partition = newRowRecogPartitionState()
			partitions[key] = partition
		}
		partition.events = append(partition.events, event)
	}
	return partitions
}

func orderRowRecogResults(results []Result, keys []SortKey, now time.Time, variables map[string]Value) []Result {
	if len(keys) == 0 || len(results) < 2 {
		return results
	}
	ordered := append([]Result(nil), results...)
	sort.SliceStable(ordered, func(left, right int) bool {
		for _, key := range keys {
			leftValue := evalResultOrderExpression(key.Expr, ordered[left], now, variables)
			rightValue := evalResultOrderExpression(key.Expr, ordered[right], now, variables)
			comparison, ok := compareValues(leftValue, rightValue)
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

func evalResultOrderExpression(expression Expr, result Result, now time.Time, variables map[string]Value) Value {
	if expression == nil {
		return Missing()
	}
	ctx := EvalContext{Now: now, Variables: variables}
	if row, ok := result.Row(); ok {
		ctx.resultRow = &row
	} else if event, ok := result.Event(); ok {
		ctx.Event = event
	}
	return expression.eval(ctx)
}

func rowRecogCurrentMatches(definition *rowRecogDefinition, partition *rowRecogPartitionState, now time.Time, variables map[string]Value) []rowRecogMatch {
	if definition == nil || partition == nil || len(partition.events) == 0 {
		return nil
	}
	result := make([]rowRecogMatch, 0)
	seen := make(map[string]struct{})
	// SKIP PAST LAST ROW removes a completed match's start branch after the
	// listener notification, but Esper's iterator keeps that already-emitted
	// row visible. Preserve the emitted history first, then only discover new
	// matches from starts that remain eligible under the current skip boundary.
	if definition.skip == RowRecogSkipPastLastRow && len(partition.emittedMatches) > 0 {
		emitted := make([]rowRecogMatch, 0, len(partition.emittedMatches))
		for _, match := range partition.emittedMatches {
			emitted = append(emitted, match)
		}
		sort.SliceStable(emitted, func(left, right int) bool {
			if emitted[left].start != emitted[right].start {
				return emitted[left].start < emitted[right].start
			}
			if emitted[left].end != emitted[right].end {
				return emitted[left].end < emitted[right].end
			}
			return rowRecogMatchKey(emitted[left]) < rowRecogMatchKey(emitted[right])
		})
		for _, match := range emitted {
			key := rowRecogMatchKey(match)
			seen[key] = struct{}{}
			result = append(result, match)
		}
	}
	for start := 0; start < len(partition.events); start++ {
		if rowRecogStartBlocked(partition, start) {
			continue
		}
		startKey := rowRecogStartKey(start, partition.events[start])
		if rowRecogHasInterval(definition) {
			if finals, closed := partition.intervalFinal[startKey]; closed {
				for _, final := range finals {
					key := rowRecogMatchKey(final)
					if _, exists := seen[key]; !exists {
						seen[key] = struct{}{}
						result = append(result, final)
					}
				}
				if !definition.intervalOrTerminated {
					continue
				}
			}
			if !definition.intervalOrTerminated {
				if _, closed := partition.intervalClosed[startKey]; closed {
					continue
				}
			}
		}
		// With ALL MATCHES and SKIP TO CURRENT ROW, Esper's iterator retains
		// every completed end for a start (including shorter optional/repeated
		// paths). Other modes expose the longest current path for that start.
		allCurrentEnds := definition.allMatches && definition.skip == RowRecogSkipToCurrentRow
		endStart := len(partition.events) - 1
		endLimit := start - 1
		step := -1
		if allCurrentEnds {
			endStart = start
			endLimit = len(partition.events)
			step = 1
		}
		for end := endStart; end != endLimit; end += step {
			matches := rowRecogMatchesWithPrevious(definition, partition.events, partition.previousByEvent, start, end, now, variables)
			if len(matches) == 0 {
				continue
			}
			for _, match := range matches {
				key := rowRecogMatchKey(match)
				if _, exists := seen[key]; exists {
					continue
				}
				if definition.intervalOrTerminated && !rowRecogMatchAllowed(definition, partition, match) {
					continue
				}
				if definition.intervalOrTerminated && rowRecogMatchBranchClosed(partition, startKey, match) {
					continue
				}
				if definition.skip == RowRecogSkipPastLastRow && rowRecogSkipPastLastSnapshotStartExcluded(partition, match, !definition.allMatches) {
					continue
				}
				seen[key] = struct{}{}
				result = append(result, match)
			}
			if !allCurrentEnds {
				break
			}
		}
	}
	return result
}
