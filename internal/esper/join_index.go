package esper

import (
	"encoding/json"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// This file implements the composition-level equi/IN join index for work unit
// incremental-join-index (performance doc §4.9.1, lower-risk half). The
// keyed-tuple diff contract in updateJoin/expireJoin is unchanged: a
// composition still materializes the complete before/after tuple snapshots.
// Only the per-composition candidate enumeration changes, from a full
// nested-loop scan with joinConditionsMatch re-evaluation on every pair to a
// hash probe over the equi/IN key columns of the opposite side.
//
// Exactness contract (byte-identical output):
//   - The index is a CANDIDATE filter only. Every candidate pair is still
//     re-checked with the exact legacy predicate (lineage check plus the full
//     joinConditionsMatch), so a spurious index hit can never change output.
//   - The index never misses a matching pair: every indexed constraint is a
//     conjunct whose equality is implied by a full match (see
//     joinKeyEqualAlternatives), and key encoding follows EqualValues
//     semantics including numeric coercion (see joinIndexKeyValue).
//   - Candidate positions are emitted in ascending row order, which is the
//     nested-loop iteration order of the legacy scan, so tuple order, old/new
//     classification and lineage keys are preserved exactly.
//
// The index is rebuilt per composition call over the current side rows. All
// side mutations happen between the before/after compositions, so a
// composition-local index has no invalidation hazards; the build is
// O(|side| * columns) against the legacy O(|left| * |right|) condition
// evaluations per composition.

// joinEquiIndexMinWork is the minimum probe-side x build-side row product for
// which the hash index replaces the nested loop. Below it the scan is already
// cheaper than building the per-composition index, and the legacy path keeps
// tiny joins (1x1, 1x2, 2x1) on the battle-tested reference implementation.
const joinEquiIndexMinWork = 4

// joinKeyMaxAlternatives caps the OR-alternative cross product extracted from
// one AllJoin condition. Overflow keeps only the later child's alternatives,
// which stays sound: a full match satisfies at least one alternative of every
// AND child, so probing any single child's alternatives never misses.
const joinKeyMaxAlternatives = 8

// joinIndexSourcesEligible reports whether every join source is a regular
// event-driven stream. Method, table, historical, pattern, derived and
// contained sources keep the reference scan composition: their rows carry
// trigger lineage, per-trigger rebuild semantics or current-state snapshots
// whose retention is cheaper to reason about on the unindexed path.
func joinIndexSourcesEligible(sources []*streamNode) bool {
	for _, source := range sources {
		base, err := sourceNode(source)
		if err != nil || base == nil {
			return false
		}
		if base.kind != streamSource && base.kind != streamNamedWindow {
			return false
		}
	}
	return true
}

// joinIndexScalarType reports whether a static expression type is safe for the
// value encoder's fast paths. Loosely typed map events can still deliver other
// dynamic values; joinIndexKeyValue handles them through the deep encoder.
func joinIndexScalarType(typ reflect.Type) bool {
	if typ == nil {
		return false
	}
	switch typ.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

// joinIndexExprExtractable reports whether expr can serve as an index key
// column for source. Only expressions whose value depends solely on the event
// of their own condition side are safe: evaluating them against a partial
// tuple with the row placed at its source slot yields exactly the value the
// full-tuple condition evaluation would see. Literals and variable references
// are row-independent and therefore always safe.
func joinIndexExprExtractable(node *exprNode, source int) bool {
	if node == nil {
		return false
	}
	switch node.kind {
	case "literal", "variable":
		return true
	case "field":
		return joinIndexScalarType(node.typ)
	case "join-field":
		return node.joinSource == source && joinIndexScalarType(node.typ)
	default:
		return false
	}
}

// joinKeyLeaf is one extracted equality leaf binding the expression of one
// source to the expression of another. Sources differ and both expressions
// are extractable for their sides.
type joinKeyLeaf struct {
	aSource int
	aExpr   Expr
	bSource int
	bExpr   Expr
}

// joinKeyAlternative is a conjunction of equality leaves. It is a NECESSARY
// condition for a matching pair: every leaf is an ANDed equality, so a full
// match satisfies all leaves of the alternative that produced it. Columns is
// the build-side-resolved view filled by joinKeyPlanForBuildSource.
type joinKeyAlternative struct {
	leaves  []joinKeyLeaf
	columns []joinIndexColumn
}

// joinKeyEqualAlternatives returns the OR-alternatives of ANDed equality
// leaves contributed by one JoinCondition, or nil when the condition
// contributes nothing indexable. Inside an AND composite, non-equality leaves
// (ranges, negations) and non-extractable leaves are skipped: they stay in
// the full joinConditionsMatch re-check and only reduce index selectivity.
// Inside an OR composite one non-indexable branch disables the whole
// condition, because pairs matched solely through that branch satisfy none of
// the remaining alternatives.
func joinKeyEqualAlternatives(condition JoinCondition) []joinKeyAlternative {
	// joinConditionMatches gives the `all` composite precedence and ignores
	// `any` when both are set; mirror that exactly.
	if len(condition.all) > 0 {
		// AND of children: cross product of child alternatives. Each product
		// element is a conjunction drawn from every constrained child, so a
		// full match satisfies exactly one product element.
		total := []joinKeyAlternative{{}}
		for _, child := range condition.all {
			childAlternatives := joinKeyEqualAlternatives(child)
			if len(childAlternatives) == 0 {
				// An unconstrained child is AND-true for indexing purposes.
				continue
			}
			if len(total) == 1 && len(total[0].leaves) == 0 {
				total = childAlternatives
				continue
			}
			next := make([]joinKeyAlternative, 0, len(total)*len(childAlternatives))
			for _, left := range total {
				for _, right := range childAlternatives {
					leaves := make([]joinKeyLeaf, 0, len(left.leaves)+len(right.leaves))
					leaves = append(leaves, left.leaves...)
					leaves = append(leaves, right.leaves...)
					next = append(next, joinKeyAlternative{leaves: leaves})
				}
			}
			if len(next) > joinKeyMaxAlternatives {
				// Cap the product. Keeping this child's alternatives alone is
				// still a sound necessary-condition set; earlier children only
				// lose index selectivity and remain in the re-check.
				return childAlternatives
			}
			total = next
		}
		if len(total) == 1 && len(total[0].leaves) == 0 {
			return nil
		}
		return total
	}
	if len(condition.any) > 0 {
		// OR of children: every child's alternatives are independent
		// necessary-condition branches. A full match satisfies at least one
		// branch, so probing each branch's index and unioning the candidates
		// (deduplicated, ascending) never misses — but ONLY while every
		// branch contributes alternatives. A non-indexable branch (range,
		// non-extractable expression) can match pairs that satisfy none of
		// the extracted alternatives, so its presence makes the whole OR
		// unusable as a probe: bail out and leave the condition to the
		// re-check.
		var total []joinKeyAlternative
		for _, child := range condition.any {
			childAlternatives := joinKeyEqualAlternatives(child)
			if len(childAlternatives) == 0 {
				return nil
			}
			total = append(total, childAlternatives...)
		}
		return total
	}
	if condition.Comparison != JoinEqual || condition.Left == nil || condition.Right == nil {
		return nil
	}
	leftSource, rightSource := joinConditionSources(condition)
	if leftSource < 0 || rightSource < 0 || leftSource == rightSource {
		return nil
	}
	if !joinIndexExprExtractable(condition.Left.node(), leftSource) {
		return nil
	}
	if !joinIndexExprExtractable(condition.Right.node(), rightSource) {
		return nil
	}
	return []joinKeyAlternative{{
		leaves: []joinKeyLeaf{{
			aSource: leftSource, aExpr: condition.Left,
			bSource: rightSource, bExpr: condition.Right,
		}},
	}}
}

// joinIndexColumn is one composite-key column resolved for a fixed build
// side: probeExpr evaluates against the probe side's row at probeSource,
// buildExpr against the build side's row.
type joinIndexColumn struct {
	probeSource int
	probeExpr   Expr
	buildExpr   Expr
}

// joinKeyPlan is the extracted index plan for one build side: a list of OR
// alternatives whose columns all bind the build side.
type joinKeyPlan struct {
	alternatives []joinKeyAlternative
}

// empty reports whether the plan carries no constraint for the build side.
func (plan joinKeyPlan) empty() bool { return len(plan.alternatives) == 0 }

// joinKeyPlanForBuildSource extracts the index plan for one build side from
// the composition conditions. Top-level conditions are ANDed, so a condition
// contributes probe branches only when EVERY one of its alternatives is
// level-valid (resolves to at least one build-side column whose probe sources
// are all strictly below the build side); otherwise the whole condition is
// dropped for this level and left to the re-check. Dropping whole conditions
// keeps the probe a sound necessary condition: partial OR groups or probe
// columns reading not-yet-bound tuple slots would either miss pairs matched
// through a dropped branch or evaluate keys against placeholder events and
// silently empty the level.
func joinKeyPlanForBuildSource(conditions []JoinCondition, buildSource int) joinKeyPlan {
	var plan joinKeyPlan
	for _, condition := range conditions {
		raw := joinKeyEqualAlternatives(condition)
		if len(raw) == 0 {
			continue
		}
		resolved := make([]joinKeyAlternative, 0, len(raw))
		usable := true
		for _, alternative := range raw {
			columns, ok := alternative.columnsForBuildSource(buildSource)
			if !ok {
				usable = false
				break
			}
			resolved = append(resolved, joinKeyAlternative{
				leaves:  alternative.leaves,
				columns: columns,
			})
		}
		if !usable {
			continue
		}
		plan.alternatives = append(plan.alternatives, resolved...)
	}
	return plan
}

// columnsForBuildSource resolves one alternative against the build side. It
// reports false unless every leaf binds exactly one side to the build side
// with the other side strictly below it: a leaf that does not touch the build
// side belongs to another level, and a leaf bound to a source at or above the
// build side would probe an unbound tuple slot.
func (alternative joinKeyAlternative) columnsForBuildSource(buildSource int) ([]joinIndexColumn, bool) {
	columns := make([]joinIndexColumn, 0, len(alternative.leaves))
	for _, leaf := range alternative.leaves {
		switch {
		case leaf.bSource == buildSource && leaf.aSource < buildSource:
			columns = append(columns, joinIndexColumn{
				probeSource: leaf.aSource, probeExpr: leaf.aExpr, buildExpr: leaf.bExpr,
			})
		case leaf.aSource == buildSource && leaf.bSource < buildSource:
			columns = append(columns, joinIndexColumn{
				probeSource: leaf.bSource, probeExpr: leaf.bExpr, buildExpr: leaf.aExpr,
			})
		default:
			return nil, false
		}
	}
	return columns, true
}

// joinIndexKeyValue encodes one Value into a canonical key part such that
// EqualValues(left, right) == true implies joinIndexKeyValue(left) ==
// joinIndexKeyValue(right). The converse is intentionally not guaranteed:
// collisions only add candidates that the re-check removes. Present values
// are always encodable; Missing and Null return usable=false because an
// SQL three-valued equality with a null operand can never be satisfied, so
// such rows cannot participate in a pair matching this key column.
func joinIndexKeyValue(value Value) (string, bool) {
	if !value.IsPresent() {
		return "", false
	}
	data := value.Any()
	// Dereference pointer/interface layers like EqualValues does; nil layers
	// fall through to the deep encoder, which keeps nil distinct from values
	// while matching nil to nil.
	reflected := reflect.ValueOf(data)
	for reflected.IsValid() && (reflected.Kind() == reflect.Pointer || reflected.Kind() == reflect.Interface) {
		if reflected.IsNil() {
			break
		}
		reflected = reflected.Elem()
		if reflected.Kind() == reflect.Invalid {
			break
		}
		if reflected.CanInterface() {
			data = reflected.Interface()
		}
	}
	if number, ok := data.(json.Number); ok {
		// json.Number follows EqualValues' numeric path: parse and promote to
		// float64 exactly like numericEqual does. A malformed number falls
		// back to the string identity, which mirrors the DeepEqual fallback.
		if parsed, err := number.Float64(); err == nil {
			return joinIndexNumericKey(parsed), true
		}
		return "j:" + string(number), true
	}
	if reflected.IsValid() {
		switch reflected.Kind() {
		case reflect.String:
			if s, ok := data.(string); ok {
				return "s:" + s, true
			}
		case reflect.Bool:
			if reflected.Bool() {
				return "bt", true
			}
			return "bf", true
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return joinIndexNumericKey(float64(reflected.Int())), true
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return joinIndexNumericKey(float64(reflected.Uint())), true
		case reflect.Float32, reflect.Float64:
			return joinIndexNumericKey(reflected.Float()), true
		}
	}
	// Anything else (structs, maps, slices, unusual string-kinded named
	// types): deep canonical encoding. DeepEqual-equal values always produce
	// identical encodings; unequal values may collide harmlessly.
	var builder strings.Builder
	joinIndexDeepKey(reflect.ValueOf(data), &builder)
	return builder.String(), true
}

// joinIndexNumericKey canonicalizes a numeric value to its float64 bit pattern,
// matching EqualValues' promotion rules. +0 and -0 encode identically because
// EqualValues compares them equal.
func joinIndexNumericKey(f float64) string {
	if f == 0 {
		f = 0
	}
	return "n:" + strconv.FormatUint(math.Float64bits(f), 16)
}

// joinIndexDeepKey appends a canonical, DeepEqual-compatible encoding of an
// arbitrary value. Pointer chains are followed by value so DeepEqual-equal
// values with different pointer identities encode identically; map keys are
// sorted so equal maps encode identically.
func joinIndexDeepKey(value reflect.Value, builder *strings.Builder) {
	switch value.Kind() {
	case reflect.Invalid:
		builder.WriteString("Z~")
		return
	case reflect.Pointer, reflect.Interface:
		if value.IsNil() {
			builder.WriteString("P~")
			return
		}
		builder.WriteString("P")
		joinIndexDeepKey(value.Elem(), builder)
		return
	case reflect.String:
		joinIndexLengthPart("S", value.String(), builder)
		return
	case reflect.Bool:
		if value.Bool() {
			builder.WriteString("B1")
		} else {
			builder.WriteString("B0")
		}
		return
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		builder.WriteString("I")
		builder.WriteString(strconv.FormatInt(value.Int(), 10))
		builder.WriteString("~")
		return
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		builder.WriteString("U")
		builder.WriteString(strconv.FormatUint(value.Uint(), 10))
		builder.WriteString("~")
		return
	case reflect.Float32, reflect.Float64:
		f := value.Float()
		if f == 0 {
			f = 0
		}
		builder.WriteString("F")
		builder.WriteString(strconv.FormatUint(math.Float64bits(f), 16))
		return
	case reflect.Struct:
		builder.WriteString("T{")
		for i := 0; i < value.NumField(); i++ {
			joinIndexDeepKey(value.Field(i), builder)
			builder.WriteString(";")
		}
		builder.WriteString("}")
		return
	case reflect.Slice:
		if value.IsNil() {
			builder.WriteString("A~")
			return
		}
		builder.WriteString("A[")
		for i := 0; i < value.Len(); i++ {
			joinIndexDeepKey(value.Index(i), builder)
			builder.WriteString(",")
		}
		builder.WriteString("]")
		return
	case reflect.Array:
		builder.WriteString("R[")
		for i := 0; i < value.Len(); i++ {
			joinIndexDeepKey(value.Index(i), builder)
			builder.WriteString(",")
		}
		builder.WriteString("]")
		return
	case reflect.Map:
		if value.IsNil() {
			builder.WriteString("M~")
			return
		}
		entries := make([]string, 0, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			var entry strings.Builder
			joinIndexDeepKey(iter.Key(), &entry)
			entry.WriteString(":")
			joinIndexDeepKey(iter.Value(), &entry)
			entries = append(entries, entry.String())
		}
		sort.Strings(entries)
		builder.WriteString("M{")
		for _, entry := range entries {
			builder.WriteString(entry)
			builder.WriteString(";")
		}
		builder.WriteString("}")
		return
	case reflect.Complex64, reflect.Complex128:
		complexValue := value.Complex()
		builder.WriteString("C")
		builder.WriteString(joinIndexNumericKey(real(complexValue)))
		builder.WriteString(joinIndexNumericKey(imag(complexValue)))
		return
	default:
		// Chan/Func/UnsafePointer have identity equality; the pointer value is
		// the canonical form (equal identity implies equal encoding).
		builder.WriteString("X")
		builder.WriteString(strconv.FormatUint(uint64(value.Pointer()), 16))
		return
	}
}

// joinIndexLengthPart writes a length-prefixed part so concatenations of
// parts are unambiguous (netstring property).
func joinIndexLengthPart(tag, part string, builder *strings.Builder) {
	builder.WriteString(tag)
	builder.WriteString(strconv.Itoa(len(part)))
	builder.WriteString(":")
	builder.WriteString(part)
	builder.WriteString("~")
}

// joinIndexExprValue evaluates one key-column expression exactly the way
// joinConditionMatches evaluates it: the row is bound both as the current
// event and as its own slot of the join tuple, so "field" and own-source
// "join-field" nodes see the identical value they would see in the full
// condition evaluation. The result is usable only when present.
func joinIndexExprValue(expr Expr, source int, tuple []Event, now time.Time, variables map[string]Value) (Value, bool) {
	current := Event{}
	if source >= 0 && source < len(tuple) {
		current = tuple[source]
	}
	value := expr.eval(EvalContext{
		Event: current, JoinEvents: tuple, OuterEvent: current, Now: now, Variables: variables,
	})
	if !value.IsPresent() {
		return Value{}, false
	}
	return value, true
}

// joinIndexAlternative is one built OR-branch of the side index: a fixed
// composite key over the branch columns, with buckets holding candidate row
// positions of the build side in ascending order.
type joinIndexAlternative struct {
	columns []joinIndexColumn
	buckets map[string][]int
	key     strings.Builder
}

// buildKey evaluates the build-side composite key for one row. tuple must
// already carry the row at the buildSource slot; every build expression reads
// only its own side's row (own-source field/join-field, literal or variable).
func (a *joinIndexAlternative) buildKey(buildSource int, tuple []Event, now time.Time, variables map[string]Value) (string, bool) {
	a.key.Reset()
	for _, column := range a.columns {
		value, ok := joinIndexExprValue(column.buildExpr, buildSource, tuple, now, variables)
		if !ok {
			return "", false
		}
		part, usable := joinIndexKeyValue(value)
		if !usable {
			return "", false
		}
		joinIndexLengthPart("c", part, &a.key)
	}
	return a.key.String(), true
}

// probeKey evaluates the probe-side composite key for one candidate binding.
func (a *joinIndexAlternative) probeKey(tuple []Event, now time.Time, variables map[string]Value) (string, bool) {
	a.key.Reset()
	for _, column := range a.columns {
		value, ok := joinIndexExprValue(column.probeExpr, column.probeSource, tuple, now, variables)
		if !ok {
			return "", false
		}
		part, usable := joinIndexKeyValue(value)
		if !usable {
			return "", false
		}
		joinIndexLengthPart("c", part, &a.key)
	}
	return a.key.String(), true
}

// joinLevelEquiIndex is the per-composition hash index over one side's rows
// for one OR-alternative set. It is built once per composition and discarded
// afterwards, so it never observes side mutations.
type joinLevelEquiIndex struct {
	alternatives []*joinIndexAlternative
	probeScratch []int
}

// buildJoinLevelEquiIndex indexes rows (candidate positions ascending) for
// one composition. buildSource is the indexed side; tuple is the scratch
// tuple whose buildSource slot receives each row while its key is encoded.
func buildJoinLevelEquiIndex(plan joinKeyPlan, buildSource int, rows []storedEvent, tuple []Event, now time.Time, variables map[string]Value) *joinLevelEquiIndex {
	index := &joinLevelEquiIndex{alternatives: make([]*joinIndexAlternative, 0, len(plan.alternatives))}
	for _, alternative := range plan.alternatives {
		built := &joinIndexAlternative{columns: alternative.columns, buckets: make(map[string][]int, len(rows))}
		for position, row := range rows {
			tuple[buildSource] = row.event
			key, ok := built.buildKey(buildSource, tuple, now, variables)
			if !ok {
				continue
			}
			built.buckets[key] = append(built.buckets[key], position)
		}
		tuple[buildSource] = Event{}
		index.alternatives = append(index.alternatives, built)
	}
	return index
}

// probe returns the candidate build-side row positions for one probe binding,
// in ascending order. A single alternative returns its bucket directly; the
// multi-alternative (IN/OR) union is deduplicated into ascending order using
// a reused scratch buffer.
func (index *joinLevelEquiIndex) probe(tuple []Event, now time.Time, variables map[string]Value) []int {
	if len(index.alternatives) == 1 {
		alternative := index.alternatives[0]
		key, ok := alternative.probeKey(tuple, now, variables)
		if !ok {
			return nil
		}
		return alternative.buckets[key]
	}
	index.probeScratch = index.probeScratch[:0]
	for _, alternative := range index.alternatives {
		key, ok := alternative.probeKey(tuple, now, variables)
		if !ok {
			continue
		}
		index.probeScratch = append(index.probeScratch, alternative.buckets[key]...)
	}
	if len(index.probeScratch) == 0 {
		return nil
	}
	sort.Ints(index.probeScratch)
	output := index.probeScratch[:0]
	previous := -1
	for _, position := range index.probeScratch {
		if position != previous {
			output = append(output, position)
			previous = position
		}
	}
	return output
}

// joinCompositionIndex carries the per-composition index state for the inner
// tuple composition: one lazily built index per level plus the shared scratch
// tuple used for both probe and build key evaluation.
type joinCompositionIndex struct {
	plans   []joinKeyPlan
	indexes []*joinLevelEquiIndex
	tuple   []Event
	vars    map[string]Value
	now     time.Time
}

// newJoinCompositionIndex prepares the per-level plans for an inner join
// composition. It returns nil when indexing cannot apply to any level.
func newJoinCompositionIndex(definition *joinDefinition, state *joinRuntimeState, conditions []JoinCondition, now time.Time, runtime *statementRuntime) *joinCompositionIndex {
	if joinDefinitionHasUnidirectional(definition) {
		return nil
	}
	sources := joinDefinitionSources(definition)
	if !joinIndexSourcesEligible(sources) {
		return nil
	}
	product := 1
	for _, side := range state.sides {
		product *= len(side)
		if product >= joinEquiIndexMinWork {
			break
		}
	}
	if product < joinEquiIndexMinWork {
		return nil
	}
	composition := &joinCompositionIndex{
		plans:   make([]joinKeyPlan, len(state.sides)),
		indexes: make([]*joinLevelEquiIndex, len(state.sides)),
		tuple:   make([]Event, len(state.sides)),
		vars:    runtime.variables,
		now:     now,
	}
	usable := false
	for level := 1; level < len(state.sides); level++ {
		composition.plans[level] = joinKeyPlanForBuildSource(conditions, level)
		if !composition.plans[level].empty() {
			usable = true
		}
	}
	if !usable {
		return nil
	}
	return composition
}

// level returns the index for one level, building it on first use. The probe
// rows of levels < level are already placed in the scratch tuple; build key
// evaluation only reads the build side slot, so the partial probe rows do not
// disturb it.
func (c *joinCompositionIndex) level(level int, rows []storedEvent) *joinLevelEquiIndex {
	if c.indexes[level] == nil {
		c.indexes[level] = buildJoinLevelEquiIndex(c.plans[level], level, rows, c.tuple, c.now, c.vars)
	}
	return c.indexes[level]
}

// joinInnerKeyedTuplesIndexed composes the inner-join tuple snapshot with
// per-level equi/IN probes. It reproduces joinKeyedTuples' legacy visit(0)
// enumeration exactly: levels are walked in source order, rows at each level
// in stored (ascending position) order, and the leaf applies the identical
// lineage-plus-conditions predicate on a fresh event tuple.
func joinInnerKeyedTuplesIndexed(definition *joinDefinition, state *joinRuntimeState, conditions []JoinCondition, now time.Time, runtime *statementRuntime) ([]joinKeyedTuple, bool) {
	composition := newJoinCompositionIndex(definition, state, conditions, now, runtime)
	if composition == nil {
		return nil, false
	}
	result := make([]joinKeyedTuple, 0)
	currentStored := make([]storedEvent, 0, len(state.sides))
	var visit func(level int)
	visit = func(level int) {
		if level == len(state.sides) {
			if !joinStoredTupleMatchesLineage(currentStored) {
				return
			}
			candidate := append([]Event(nil), composition.tuple...)
			if joinConditionsMatch(conditions, candidate, now, runtime) {
				result = append(result, joinKeyedTuple{events: candidate, key: joinStoredTupleLineageKey(currentStored)})
			}
			return
		}
		rows := state.sides[level]
		var index *joinLevelEquiIndex
		if !composition.plans[level].empty() {
			index = composition.level(level, rows)
		}
		if index != nil {
			for _, position := range index.probe(composition.tuple, now, runtime.variables) {
				row := rows[position]
				composition.tuple[level] = row.event
				currentStored = append(currentStored, row)
				visit(level + 1)
				currentStored = currentStored[:len(currentStored)-1]
			}
			composition.tuple[level] = Event{}
			return
		}
		for _, row := range rows {
			composition.tuple[level] = row.event
			currentStored = append(currentStored, row)
			visit(level + 1)
			currentStored = currentStored[:len(currentStored)-1]
		}
		composition.tuple[level] = Event{}
	}
	visit(0)
	return result, true
}
