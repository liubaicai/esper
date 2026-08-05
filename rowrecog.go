package esper

import (
	"fmt"
	"sort"
	"strings"
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
	partition            []Expr
	allMatches           bool
	skip                 RowRecogSkipStrategy
	maxStates            int
	interval             time.Duration
	intervalCalendar     *OutputCalendarPeriod
	intervalOrTerminated bool
}

// MatchRecognize starts a row-pattern query from a typed stream.
func (s Stream[T]) MatchRecognize(pattern RowPattern) RowRecogQuery {
	return RowRecogQuery{
		env: s.env,
		definition: &rowRecogDefinition{
			input:      s.node,
			pattern:    pattern,
			defines:    make(map[string]Expr),
			allMatches: true,
			skip:       RowRecogSkipPastLastRow,
		},
	}
}

// MatchRecognize starts a row-pattern query from a dynamic RecordStream.
func (s RecordStream) MatchRecognize(pattern RowPattern) RowRecogQuery {
	return RowRecogQuery{
		env: s.env,
		definition: &rowRecogDefinition{
			input:      s.node,
			pattern:    pattern,
			defines:    make(map[string]Expr),
			allMatches: true,
			skip:       RowRecogSkipPastLastRow,
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
	q.definition.defines[strings.TrimSpace(name)] = predicate
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
		env:               q.env,
		input:             q.definitionInput(),
		rowRecog:          q.definition,
		patternSelections: append([]Selection(nil), q.measures...),
		name:              spec.name,
		selector:          spec.selector,
		sink:              spec.sink,
		contextName:       spec.contextName,
		output:            spec.output,
		distinct:          spec.distinct,
		orderBy:           append([]SortKey(nil), spec.orderBy...),
		limit:             spec.limit,
		offset:            spec.offset,
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
	captures map[string][]Event
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
		partition.events = append(partition.events, event)
		if !rowRecogHasInterval(definition) {
			r.emitRowRecogMatchesAtEnd(definition, partition, len(partition.events)-1, plan, now, &batch)
		} else if definition.intervalOrTerminated {
			r.emitRowRecogTerminated(definition, partition, len(partition.events)-1, plan, now, &batch)
		}
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
	return batch
}

func newRowRecogPartitionState() *rowRecogPartitionState {
	return &rowRecogPartitionState{
		emitted:        make(map[string]struct{}),
		intervalClosed: make(map[string]struct{}),
		intervalFinal:  make(map[string][]rowRecogMatch),
	}
}

func (r *statementRuntime) emitRowRecogMatchesAtEnd(definition *rowRecogDefinition, partition *rowRecogPartitionState, end int, plan Plan, now time.Time, batch *ResultBatch) {
	if definition == nil || partition == nil || batch == nil || end < 0 {
		return
	}
	// skipStart controls which matches may be emitted, not which branches are
	// retained. Java can keep updating a skipped branch for iterator results.
	for start := 0; start <= end; start++ {
		matches := rowRecogMatches(definition, partition.events, start, end, now, r.variables)
		emittedForStart := false
		for _, match := range matches {
			if match.start < partition.skipStart {
				continue
			}
			matchKey := rowRecogMatchKey(match)
			if _, exists := partition.emitted[matchKey]; exists {
				continue
			}
			partition.emitted[matchKey] = struct{}{}
			if row, visible := evaluateRowRecogMatch(match, partition.events, plan, now, r.variables); visible {
				batch.New = append(batch.New, resultRow(row))
			}
			emittedForStart = true
			advanceRowRecogSkip(definition, partition, match)
			if !definition.allMatches {
				break
			}
		}
		if !definition.allMatches && emittedForStart {
			break
		}
	}
}

func advanceRowRecogSkip(definition *rowRecogDefinition, partition *rowRecogPartitionState, match rowRecogMatch) {
	if definition == nil || partition == nil {
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
		if partition.intervalFinal == nil {
			partition.intervalFinal = make(map[string][]rowRecogMatch)
		}
		for start := 0; start < len(partition.events); start++ {
			startEvent := partition.events[start]
			startKey := rowRecogStartKey(start, startEvent)
			if _, closed := partition.intervalClosed[startKey]; closed {
				continue
			}
			deadline := rowRecogDeadline(definition, startEvent.ReceivedAt())
			if deadline.After(now) {
				continue
			}
			partition.intervalClosed[startKey] = struct{}{}
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
			matches := rowRecogMatchesAtOrBefore(definition, partition.events, start, last, now, r.variables)
			for _, match := range matches {
				if match.start < partition.skipStart {
					continue
				}
				matchKey := rowRecogMatchKey(match)
				if _, exists := partition.emitted[matchKey]; exists {
					continue
				}
				partition.emitted[matchKey] = struct{}{}
				partition.intervalFinal[startKey] = append(partition.intervalFinal[startKey], match)
				if row, visible := evaluateRowRecogMatch(match, partition.events, plan, now, r.variables); visible {
					batch.New = append(batch.New, resultRow(row))
				}
				advanceRowRecogSkip(definition, partition, match)
				if !definition.allMatches {
					break
				}
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
		startKey := rowRecogStartKey(start, partition.events[start])
		if _, closed := partition.intervalClosed[startKey]; closed {
			continue
		}
		current := rowRecogMatches(definition, partition.events, start, end, now, r.variables)
		if len(current) > 0 {
			if rowRecogMatchAtFiniteWidth(definition.pattern, current[0]) {
				r.emitRowRecogIntervalMatches(definition, partition, startKey, current, plan, now, batch)
			}
			continue
		}
		terminated := rowRecogMatchesAtOrBefore(definition, partition.events, start, end-1, now, r.variables)
		if len(terminated) > 0 {
			r.emitRowRecogIntervalMatches(definition, partition, startKey, terminated, plan, now, batch)
		}
	}
}

func (r *statementRuntime) emitRowRecogIntervalMatches(definition *rowRecogDefinition, partition *rowRecogPartitionState, startKey string, matches []rowRecogMatch, plan Plan, now time.Time, batch *ResultBatch) {
	if r == nil || definition == nil || partition == nil || batch == nil || len(matches) == 0 {
		return
	}
	if partition.intervalClosed == nil {
		partition.intervalClosed = make(map[string]struct{})
	}
	if partition.intervalFinal == nil {
		partition.intervalFinal = make(map[string][]rowRecogMatch)
	}
	partition.intervalClosed[startKey] = struct{}{}
	for _, match := range matches {
		if match.start < partition.skipStart {
			continue
		}
		matchKey := rowRecogMatchKey(match)
		if _, exists := partition.emitted[matchKey]; exists {
			continue
		}
		partition.emitted[matchKey] = struct{}{}
		partition.intervalFinal[startKey] = append(partition.intervalFinal[startKey], match)
		if row, visible := evaluateRowRecogMatch(match, partition.events, plan, now, r.variables); visible {
			batch.New = append(batch.New, resultRow(row))
		}
		advanceRowRecogSkip(definition, partition, match)
		if !definition.allMatches {
			break
		}
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
	for end := last; end >= start; end-- {
		matches := rowRecogMatches(definition, history, start, end, now, variables)
		if len(matches) > 0 {
			return matches
		}
	}
	return nil
}

func rowRecogStartKey(index int, event Event) string {
	return fmt.Sprintf("%d:%s:%d", index, eventIdentity(event), event.ReceivedAt().UnixNano())
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
		partition.events = append(partition.events[:index], partition.events[index+1:]...)
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
		// shift later rows, so discard stale keys and let future arrivals be
		// recognized against the current retained view.
		partition.emitted = make(map[string]struct{})
		partition.intervalClosed = make(map[string]struct{})
		partition.intervalFinal = make(map[string][]rowRecogMatch)
		if len(partition.events) == 0 {
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
	if definition == nil || start < 0 || end < start || end >= len(history) {
		return nil
	}
	matcher := &rowPatternMatchContext{
		definition: definition,
		history:    history,
		end:        end,
		now:        now,
		variables:  variables,
		limit:      definition.maxStates,
	}
	paths := matcher.match(definition.pattern, start, nil)
	result := make([]rowRecogMatch, 0, len(paths))
	for _, path := range paths {
		if path.position != end+1 {
			continue
		}
		result = append(result, rowRecogMatch{start: start, end: end, captures: cloneRowRecogCaptures(path.captures)})
		if definition.maxStates > 0 && len(result) >= definition.maxStates {
			break
		}
	}
	return result
}

const maxRowRecogPatternSteps = 1 << 20

type rowPatternMatchPath struct {
	position int
	captures map[string][]Event
}

type rowPatternMatchContext struct {
	definition *rowRecogDefinition
	history    []Event
	end        int
	now        time.Time
	variables  map[string]Value
	limit      int
	steps      int
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
				Event:     event,
				History:   append([]Event(nil), m.history[:position+1]...),
				Tags:      rowRecogLastTags(next),
				TagValues: cloneRowRecogCaptures(next),
				Now:       m.now,
				Variables: m.variables,
			})
			matched, ok := boolValue(value)
			if !ok || !matched {
				return nil
			}
		}
		return []rowPatternMatchPath{{position: position + 1, captures: next}}
	case rowPatternSequence:
		paths := []rowPatternMatchPath{{position: position, captures: cloneRowRecogCaptures(captures)}}
		for _, part := range pattern.parts {
			next := make([]rowPatternMatchPath, 0)
			for _, path := range paths {
				next = m.appendLimited(next, m.match(part, path.position, path.captures))
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
		for _, part := range pattern.parts {
			result = m.appendLimited(result, m.match(part, position, captures))
			if m.exhausted(len(result)) {
				break
			}
		}
		return result
	case rowPatternPermutation:
		result := make([]rowPatternMatchPath, 0)
		for _, order := range rowPatternPermutationOrders(pattern.parts) {
			paths := []rowPatternMatchPath{{position: position, captures: cloneRowRecogCaptures(captures)}}
			for _, part := range order {
				next := make([]rowPatternMatchPath, 0)
				for _, path := range paths {
					next = m.appendLimited(next, m.match(part, path.position, path.captures))
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
	if maximum == 0 {
		maximum = m.end - position + 1
		if maximum < minimum {
			maximum = minimum
		}
	}
	levels := make([][]rowPatternMatchPath, 0, maximum+1)
	active := []rowPatternMatchPath{{position: position, captures: cloneRowRecogCaptures(captures)}}
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
	if len(plan.query.patternSelections) == 0 || match.end < 0 || match.end >= len(history) {
		return Row{}, false
	}
	currentHistory := append([]Event(nil), history[:match.end+1]...)
	ctx := EvalContext{
		Event:     history[match.end],
		Group:     append([]Event(nil), history[match.start:match.end+1]...),
		History:   currentHistory,
		Tags:      rowRecogLastTags(match.captures),
		TagValues: cloneRowRecogCaptures(match.captures),
		Now:       now,
		Variables: variables,
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
	if definition == nil || r.rowRecogState == nil {
		return ResultBatch{Time: now}
	}
	keys := make([]string, 0, len(r.rowRecogState.partitions))
	for key := range r.rowRecogState.partitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := ResultBatch{Time: now}
	seen := make(map[string]struct{})
	for _, key := range keys {
		partition := r.rowRecogState.partitions[key]
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
			if row, visible := evaluateRowRecogMatch(match, partition.events, plan, now, variables); visible {
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
	for start := 0; start < len(partition.events); start++ {
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
				continue
			}
			if _, closed := partition.intervalClosed[startKey]; closed {
				continue
			}
		}
		for end := len(partition.events) - 1; end >= start; end-- {
			matches := rowRecogMatches(definition, partition.events, start, end, now, variables)
			if len(matches) == 0 {
				continue
			}
			for _, match := range matches {
				key := rowRecogMatchKey(match)
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				result = append(result, match)
				if !definition.allMatches {
					break
				}
			}
			break
		}
	}
	return result
}
