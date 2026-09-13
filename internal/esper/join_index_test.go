package esper

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Key encoding equivalence with EqualValues
// ---------------------------------------------------------------------------

func TestJoinIndexKeyValueMatchesEqualValues(t *testing.T) {
	negativeZero := math.Copysign(0, -1)
	typedNil := (*string)(nil)
	otherTypedNil := (*int)(nil)
	base := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	sameTime := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	monotonicTime := base.Add(0).Round(0)
	cases := []struct {
		name        string
		left        Value
		right       Value
		expectEqual bool
	}{
		{name: "int-int", left: Present(1), right: Present(1), expectEqual: true},
		{name: "int-int64", left: Present(1), right: Present(int64(1)), expectEqual: true},
		{name: "int64-float", left: Present(int64(5)), right: Present(5.0), expectEqual: true},
		{name: "uint-int-negative", left: Present(uint64(3)), right: Present(int64(-3)), expectEqual: false},
		{name: "json-number-int", left: Present(json.Number("1")), right: Present(1), expectEqual: true},
		{name: "json-number-float", left: Present(json.Number("1.0")), right: Present(1.0), expectEqual: true},
		{name: "json-number-malformed", left: Present(json.Number("abc")), right: Present(json.Number("abc")), expectEqual: true},
		{name: "json-number-malformed-vs-string", left: Present(json.Number("abc")), right: Present("abc"), expectEqual: false},
		{name: "zero-vs-negative-zero", left: Present(0.0), right: Present(negativeZero), expectEqual: true},
		{name: "string-vs-number", left: Present("1"), right: Present(1), expectEqual: false},
		{name: "string-string", left: Present("x"), right: Present("x"), expectEqual: true},
		{name: "bool-bool", left: Present(true), right: Present(true), expectEqual: true},
		{name: "bool-int", left: Present(true), right: Present(1), expectEqual: false},
		{name: "typed-nil-pointers", left: Present(typedNil), right: Present(typedNil), expectEqual: true},
		{name: "typed-nil-vs-value", left: Present(typedNil), right: Present("x"), expectEqual: false},
		{name: "different-typed-nils", left: Present(typedNil), right: Present(otherTypedNil), expectEqual: false},
		{name: "equal-times", left: Present(base), right: Present(sameTime), expectEqual: true},
		{name: "slices", left: Present([]any{int64(1), "a"}), right: Present([]any{int64(1), "a"}), expectEqual: true},
		{name: "maps", left: Present(map[string]any{"a": int64(1)}), right: Present(map[string]any{"a": int64(1)}), expectEqual: true},
		// Monotonic-clock readings make DeepEqual false even for the same
		// instant, so the encoded keys must also differ is NOT required; only
		// DeepEqual-equal values must share a key. The reverse direction is
		// asserted through expectEqual=false tolerance.
		{name: "monotonic-time", left: Present(base), right: Present(monotonicTime), expectEqual: reflect.DeepEqual(base, monotonicTime)},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			leftKey, leftUsable := joinIndexKeyValue(testCase.left)
			rightKey, rightUsable := joinIndexKeyValue(testCase.right)
			// The condition is satisfied only when EqualValues yields the
			// boolean true; null (ok=false) and false both reject the pair.
			matched, matchedOK := boolValue(EqualValues(testCase.left, testCase.right))
			satisfied := matchedOK && matched
			if satisfied != testCase.expectEqual {
				t.Fatalf("EqualValues(%v, %v) satisfied=%v, want %v", testCase.left, testCase.right, satisfied, testCase.expectEqual)
			}
			if !testCase.expectEqual {
				return
			}
			if !leftUsable || !rightUsable {
				t.Fatalf("present values must be encodable, got usable %v/%v", leftUsable, rightUsable)
			}
			if leftKey != rightKey {
				t.Fatalf("EqualValues-true values encoded to different keys %q vs %q", leftKey, rightKey)
			}
		})
	}
	t.Run("missing-and-null-are-unusable", func(t *testing.T) {
		for _, value := range []Value{Missing(), Null()} {
			if _, usable := joinIndexKeyValue(value); usable {
				t.Fatalf("value %v must not be encodable", value)
			}
			// Three-valued equality with null never matches, so excluding
			// these rows from the index cannot lose a pair.
			if _, ok := boolValue(EqualValues(value, Present(1))); ok {
				t.Fatalf("EqualValues with null-ish %v must be undefined", value)
			}
		}
	})
	t.Run("composite-parts-are-unambiguous", func(t *testing.T) {
		encode := func(parts ...string) string {
			var builder strings.Builder
			for _, part := range parts {
				joinIndexLengthPart("c", part, &builder)
			}
			return builder.String()
		}
		if encode("a", "bc") == encode("ab", "c") {
			t.Fatal("composite encoding must be injective across part boundaries")
		}
	})
}

// ---------------------------------------------------------------------------
// Indexed composition vs the legacy nested-loop reference
// ---------------------------------------------------------------------------

type jixRow struct {
	Key   string  `esper:"key"`
	Num   int64   `esper:"num"`
	Ratio float64 `esper:"ratio"`
}

type jixNullableRow struct {
	Key *string `esper:"key"`
}

func referenceInnerKeyedTuples(state *joinRuntimeState, conditions []JoinCondition, now time.Time, runtime *statementRuntime) []joinKeyedTuple {
	result := make([]joinKeyedTuple, 0)
	current := make([]Event, 0, len(state.sides))
	currentStored := make([]storedEvent, 0, len(state.sides))
	var visit func(int)
	visit = func(index int) {
		if index == len(state.sides) {
			if !joinStoredTupleMatchesLineage(currentStored) {
				return
			}
			candidate := append([]Event(nil), current...)
			if joinConditionsMatch(conditions, candidate, now, runtime) {
				result = append(result, joinKeyedTuple{events: candidate, key: joinStoredTupleLineageKey(currentStored)})
			}
			return
		}
		for _, stored := range state.sides[index] {
			current = append(current, stored.event)
			currentStored = append(currentStored, stored)
			visit(index + 1)
			current = current[:len(current)-1]
			currentStored = currentStored[:len(currentStored)-1]
		}
	}
	visit(0)
	return result
}

func referenceOuterTwoStreamKeyedTuples(state *joinRuntimeState, conditions []JoinCondition, kind JoinKind, now time.Time, runtime *statementRuntime) []joinKeyedTuple {
	result := make([]joinKeyedTuple, 0)
	matchedRight := make(map[int]bool)
	for _, left := range state.sides[0] {
		matched := false
		for rightIndex, right := range state.sides[1] {
			storedTuple := []storedEvent{left, right}
			if !joinStoredTupleMatchesLineage(storedTuple) {
				continue
			}
			tuple := []Event{left.event, right.event}
			if joinConditionsMatch(conditions, tuple, now, runtime) {
				matched = true
				matchedRight[rightIndex] = true
				result = append(result, joinKeyedTuple{events: tuple, key: joinStoredTupleLineageKey(storedTuple)})
			}
		}
		if !matched && (kind == JoinLeftOuter || kind == JoinFullOuter) {
			storedTuple := []storedEvent{left, {}}
			result = append(result, joinKeyedTuple{events: []Event{left.event, Event{}}, key: joinStoredTupleLineageKey(storedTuple)})
		}
	}
	if kind == JoinRightOuter || kind == JoinFullOuter {
		for rightIndex, right := range state.sides[1] {
			if !matchedRight[rightIndex] {
				storedTuple := []storedEvent{{}, right}
				result = append(result, joinKeyedTuple{events: []Event{Event{}, right.event}, key: joinStoredTupleLineageKey(storedTuple)})
			}
		}
	}
	return result
}

func referenceChainedKeyedTuples(definition *joinDefinition, state *joinRuntimeState, now time.Time, runtime *statementRuntime) []joinKeyedTuple {
	rows := make([]chainedJoinTuple, 0, len(state.sides[0]))
	for _, stored := range state.sides[0] {
		rows = append(rows, chainedJoinTuple{events: []Event{stored.event}, stored: []storedEvent{stored}})
	}
	for edgeIndex, edge := range definition.edges {
		rightSide := state.sides[edgeIndex+1]
		matchedRight := make([]bool, len(rightSide))
		next := make([]chainedJoinTuple, 0)
		for _, left := range rows {
			if edge.kind != JoinInner && !joinChainedEdgeAnchored(edge, edgeIndex, left.events) {
				events := append(append([]Event(nil), left.events...), Event{})
				stored := append(append([]storedEvent(nil), left.stored...), storedEvent{})
				next = append(next, chainedJoinTuple{events: events, stored: stored})
				continue
			}
			matchedLeft := false
			for rightSlot, right := range rightSide {
				events := append(append([]Event(nil), left.events...), right.event)
				stored := append(append([]storedEvent(nil), left.stored...), right)
				if !joinStoredTupleMatchesLineageWithMissing(stored) || !joinConditionsMatch(edge.conditions, events, now, runtime) {
					continue
				}
				matchedLeft = true
				matchedRight[rightSlot] = true
				next = append(next, chainedJoinTuple{events: events, stored: stored})
			}
			if !matchedLeft && (edge.kind == JoinLeftOuter || edge.kind == JoinFullOuter) && joinStoredTupleHasAnchor(left.stored) {
				events := append(append([]Event(nil), left.events...), Event{})
				stored := append(append([]storedEvent(nil), left.stored...), storedEvent{})
				next = append(next, chainedJoinTuple{events: events, stored: stored})
			}
		}
		rows = next
	}
	result := make([]joinKeyedTuple, len(rows))
	for index, row := range rows {
		result[index] = joinKeyedTuple{events: row.events, key: joinStoredTupleLineageKey(row.stored)}
	}
	return result
}

// jixFixture builds a join definition over registered struct sources plus a
// runtime and per-side stored rows for direct composition comparisons.
type jixFixture struct {
	env      *Environment
	schema   Schema
	runtime  *statementRuntime
	now      time.Time
	sequence uint64
}

func newJixFixture(t *testing.T, names []string) *jixFixture {
	t.Helper()
	env := NewEnvironment()
	for _, name := range names {
		if _, err := RegisterStruct[jixRow](env, name); err != nil {
			t.Fatal(err)
		}
	}
	return &jixFixture{env: env, runtime: nil, now: time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)}
}

func (f *jixFixture) runtimeFor(t *testing.T, query Query) *statementRuntime {
	t.Helper()
	value := newStatementRuntime(query)
	value.variables = map[string]Value{}
	f.runtime = &value
	return &value
}

func (f *jixFixture) event(t *testing.T, name string, row jixRow) Event {
	t.Helper()
	schema, ok := f.env.Schema(name)
	if !ok {
		t.Fatalf("schema %q not registered", name)
	}
	event, err := newEvent(schema, &row, f.now)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func (f *jixFixture) rows(t *testing.T, name string, keys []jixRow) []storedEvent {
	t.Helper()
	result := make([]storedEvent, 0, len(keys))
	for _, key := range keys {
		f.sequence++
		result = append(result, storedEvent{event: f.event(t, name, key), receivedAt: f.now, lineageID: f.sequence})
	}
	return result
}

func assertKeyedTuplesEqual(t *testing.T, stage string, indexed, reference []joinKeyedTuple) {
	t.Helper()
	if len(indexed) != len(reference) {
		t.Fatalf("%s: tuple count %d != reference %d\nindexed: %v\nreference: %v", stage, len(indexed), len(reference), describeTuples(indexed), describeTuples(reference))
	}
	for index := range indexed {
		if indexed[index].key != reference[index].key {
			t.Fatalf("%s: tuple %d lineage key %q != reference %q", stage, index, indexed[index].key, reference[index].key)
		}
		if len(indexed[index].events) != len(reference[index].events) {
			t.Fatalf("%s: tuple %d width mismatch", stage, index)
		}
		for slot := range indexed[index].events {
			left := indexed[index].events[slot]
			right := reference[index].events[slot]
			if left.TypeName() != right.TypeName() || !reflect.DeepEqual(left.Underlying(), right.Underlying()) {
				t.Fatalf("%s: tuple %d slot %d event mismatch: %v vs %v", stage, index, slot, left.Underlying(), right.Underlying())
			}
		}
	}
}

func describeTuples(tuples []joinKeyedTuple) []string {
	result := make([]string, 0, len(tuples))
	for _, tuple := range tuples {
		parts := make([]string, 0, len(tuple.events))
		for _, event := range tuple.events {
			parts = append(parts, fmt.Sprintf("%s:%+v", event.TypeName(), event.Underlying()))
		}
		result = append(result, fmt.Sprintf("[%s]|%s", joinStringsConcat(parts, ","), tuple.key))
	}
	return result
}

func joinStringsConcat(parts []string, separator string) string {
	out := ""
	for index, part := range parts {
		if index > 0 {
			out += separator
		}
		out += part
	}
	return out
}

func TestJoinIndexCompositionMatchesLegacyReference(t *testing.T) {
	type scenario struct {
		name       string
		sideCount  int
		conditions func(names []string) []JoinCondition
		chained    bool
		// fallbackOnly marks scenarios whose conditions can never produce an
		// index plan by design; the engagement check below asserts the
		// composition really stays on the reference scan for them.
		fallbackOnly bool
	}
	scenarios := []scenario{
		{
			name:      "single-equi-string",
			sideCount: 2,
			conditions: func(names []string) []JoinCondition {
				return []JoinCondition{OnSourcesEqual(0, Field[jixRow, string]("key"), 1, Field[jixRow, string]("key"))}
			},
		},
		{
			name:      "composite-equi-two-columns",
			sideCount: 2,
			conditions: func(names []string) []JoinCondition {
				return []JoinCondition{
					OnSourcesEqual(0, Field[jixRow, string]("key"), 1, Field[jixRow, string]("key")),
					OnSourcesEqual(0, Field[jixRow, int64]("num"), 1, Field[jixRow, float64]("ratio")),
				}
			},
		},
		{
			name:      "equi-plus-range",
			sideCount: 2,
			conditions: func(names []string) []JoinCondition {
				return []JoinCondition{
					OnSourcesEqual(0, Field[jixRow, string]("key"), 1, Field[jixRow, string]("key")),
					OnSourcesCompare(0, Field[jixRow, int64]("num"), 1, Field[jixRow, int64]("num"), JoinGreater),
				}
			},
		},
		{
			name:      "in-any-join-two-branches",
			sideCount: 2,
			conditions: func(names []string) []JoinCondition {
				left := Field[jixRow, string]("key")
				right := Field[jixRow, string]("key")
				num := Field[jixRow, int64]("num")
				return []JoinCondition{AnyJoin(
					OnSourcesEqual(0, left, 1, right),
					OnSourcesEqual(0, num, 1, num),
				)}
			},
		},
		{
			name:      "all-over-any-composite",
			sideCount: 2,
			conditions: func(names []string) []JoinCondition {
				left := Field[jixRow, string]("key")
				right := Field[jixRow, string]("key")
				return []JoinCondition{AllJoin(
					AnyJoin(
						OnSourcesEqual(0, left, 1, right),
						OnSourcesEqual(0, Field[jixRow, int64]("num"), 1, Field[jixRow, int64]("num")),
					),
					OnSourcesCompare(0, Field[jixRow, int64]("num"), 1, Field[jixRow, float64]("ratio"), JoinLessOrEqual),
				)}
			},
		},
		{
			name:      "coercing-int64-to-float64",
			sideCount: 2,
			conditions: func(names []string) []JoinCondition {
				return []JoinCondition{OnSourcesEqual(0, Field[jixRow, int64]("num"), 1, Field[jixRow, float64]("ratio"))}
			},
		},
		{
			name:      "join-field-conditions",
			sideCount: 2,
			conditions: func(names []string) []JoinCondition {
				return []JoinCondition{
					OnSourcesEqual(0, JoinField[string](0, "key"), 1, JoinField[string](1, "key")),
				}
			},
		},
		{
			name:      "cross-source-join-field-not-extractable",
			sideCount: 2,
			// The right expression reads source 0 from inside side 1's
			// condition slot, so it is not own-source-only and must stay
			// out of the index key columns.
			fallbackOnly: true,
			conditions: func(names []string) []JoinCondition {
				return []JoinCondition{OnSourcesEqual(0, JoinField[string](0, "key"), 1, JoinField[string](0, "ratio"))}
			},
		},
		{
			name:      "any-mixed-equi-and-range",
			sideCount: 2,
			// P1-1 pin: an OR branch that is not equi-indexable disables the
			// whole OR as a probe; pairs matched solely through the range
			// branch satisfy none of the equi alternatives.
			fallbackOnly: true,
			conditions: func(names []string) []JoinCondition {
				return []JoinCondition{AnyJoin(
					OnSourcesEqual(0, Field[jixRow, string]("key"), 1, Field[jixRow, string]("key")),
					OnSourcesCompare(0, Field[jixRow, int64]("num"), 1, Field[jixRow, int64]("num"), JoinGreater),
				)}
			},
		},
		{
			name:      "any-mixed-equi-and-non-extractable",
			sideCount: 2,
			// Same P1-1 class with a non-extractable (cross-source) OR branch.
			fallbackOnly: true,
			conditions: func(names []string) []JoinCondition {
				return []JoinCondition{AnyJoin(
					OnSourcesEqual(0, Field[jixRow, string]("key"), 1, Field[jixRow, string]("key")),
					OnSourcesEqual(0, JoinField[string](0, "key"), 1, JoinField[string](0, "ratio")),
				)}
			},
		},
		{
			name:      "three-stream-two-pairs",
			sideCount: 3,
			conditions: func(names []string) []JoinCondition {
				return []JoinCondition{
					OnSourcesEqual(0, Field[jixRow, string]("key"), 1, Field[jixRow, string]("key")),
					OnSourcesEqual(1, Field[jixRow, string]("key"), 2, Field[jixRow, string]("key")),
				}
			},
		},
		{
			name:      "three-stream-cross-condition",
			sideCount: 3,
			conditions: func(names []string) []JoinCondition {
				return []JoinCondition{
					OnSourcesEqual(0, Field[jixRow, string]("key"), 2, Field[jixRow, string]("key")),
					OnSourcesEqual(0, Field[jixRow, int64]("num"), 1, Field[jixRow, int64]("num")),
				}
			},
		},
		{
			name:      "three-stream-pair-without-lower-level",
			sideCount: 3,
			// P1-2 pin: the only condition binds sources 1 and 2, so level 1
			// has no usable plan and must full-scan instead of probing an
			// unbound slot-2 key (which would empty the result).
			conditions: func(names []string) []JoinCondition {
				return []JoinCondition{
					OnSourcesEqual(1, Field[jixRow, int64]("num"), 2, Field[jixRow, int64]("num")),
				}
			},
		},
		{
			name:      "three-stream-or-cross-pairs",
			sideCount: 3,
			// P1-2 pin: OR branches bind different source pairs. Each level
			// either applies the whole OR (all branches level-valid) or
			// full-scans; per-branch filtering would probe slot 2 at level 1.
			// Branch (0,1) never touches build side 2 and branch (1,2) probes
			// unbound slot 2 at level 1, so this OR never indexes and the
			// composition must stay on the reference scan at every level.
			fallbackOnly: true,
			conditions: func(names []string) []JoinCondition {
				return []JoinCondition{AnyJoin(
					OnSourcesEqual(0, Field[jixRow, string]("key"), 1, Field[jixRow, string]("key")),
					OnSourcesEqual(1, Field[jixRow, int64]("num"), 2, Field[jixRow, int64]("num")),
				)}
			},
		},
		{
			name:      "three-stream-or-same-pair",
			sideCount: 3,
			// The OR branches all bind sources (0,1), so the whole group is
			// level-valid at level 1 and must probe there; level 2 has no
			// plan and full-scans under the re-check.
			conditions: func(names []string) []JoinCondition {
				return []JoinCondition{AnyJoin(
					OnSourcesEqual(0, Field[jixRow, string]("key"), 1, Field[jixRow, string]("key")),
					OnSourcesEqual(0, Field[jixRow, int64]("num"), 1, Field[jixRow, int64]("num")),
				)}
			},
		},
		{
			name:      "chained-inner-two-edges",
			sideCount: 3,
			chained:   true,
			conditions: func(names []string) []JoinCondition {
				return []JoinCondition{
					OnSourcesEqual(0, Field[jixRow, string]("key"), 1, Field[jixRow, string]("key")),
					OnSourcesEqual(1, Field[jixRow, string]("key"), 2, Field[jixRow, string]("key")),
				}
			},
		},
		{
			name:      "chained-inner-cross-edge",
			sideCount: 3,
			chained:   true,
			conditions: func(names []string) []JoinCondition {
				return []JoinCondition{
					OnSourcesEqual(0, Field[jixRow, string]("key"), 1, Field[jixRow, string]("key")),
					OnSourcesEqual(0, Field[jixRow, int64]("num"), 2, Field[jixRow, int64]("num")),
				}
			},
		},
	}
	keyAlphabet := []string{"a", "b", "c", ""}
	random := rand.New(rand.NewSource(20240909))
	for _, testCase := range scenarios {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			names := make([]string, testCase.sideCount)
			for index := range names {
				names[index] = fmt.Sprintf("JixRefS%d", index)
			}
			fixture := newJixFixture(t, names)
			// Engagement pin: with two rows per side the work threshold is
			// always met, so every indexable scenario must report the indexed
			// path (and every fallbackOnly scenario must not). Without this
			// check the random rounds could silently fall below the threshold
			// and compare reference against reference.
			if !testCase.chained {
				conditions := testCase.conditions(names)
				definition := buildJixManyQuery(t, fixture.env, names, conditions)
				runtime := fixture.runtimeFor(t, Query{join: definition})
				gateState := &joinRuntimeState{sides: make([][]storedEvent, testCase.sideCount)}
				for side := range gateState.sides {
					gateState.sides[side] = fixture.rows(t, names[side], []jixRow{{Key: "a", Num: 1}, {Key: "b", Num: 2}})
				}
				_, engaged := joinInnerKeyedTuplesIndexed(definition, gateState, conditions, fixture.now, runtime)
				if engaged == testCase.fallbackOnly {
					t.Fatalf("scenario %s: indexed path engaged=%v, want engaged=%v", testCase.name, engaged, !testCase.fallbackOnly)
				}
			}
			for round := 0; round < 40; round++ {
				state := &joinRuntimeState{sides: make([][]storedEvent, testCase.sideCount)}
				for side := range state.sides {
					rowCount := random.Intn(5)
					rows := make([]jixRow, 0, rowCount)
					for i := 0; i < rowCount; i++ {
						rows = append(rows, jixRow{
							Key:   keyAlphabet[random.Intn(len(keyAlphabet))],
							Num:   int64(random.Intn(3)),
							Ratio: float64(random.Intn(3)),
						})
					}
					state.sides[side] = fixture.rows(t, names[side], rows)
				}
				conditions := testCase.conditions(names)
				var definition *joinDefinition
				if testCase.chained {
					definition = buildJixChainedQuery(t, fixture.env, names, conditions)
				} else {
					definition = buildJixManyQuery(t, fixture.env, names, conditions)
				}
				runtime := fixture.runtimeFor(t, Query{join: definition})
				// Force the index path to be considered: the random rows can
				// fall below the work threshold, in which case the composition
				// returns identical output through the reference scan.
				indexed := joinKeyedTuples(definition, state, fixture.now, runtime)
				if testCase.chained {
					assertKeyedTuplesEqual(t, "chained", indexed, referenceChainedKeyedTuples(definition, state, fixture.now, runtime))
					continue
				}
				if definition.kind == JoinInner {
					assertKeyedTuplesEqual(t, "inner", indexed, referenceInnerKeyedTuples(state, conditions, fixture.now, runtime))
					continue
				}
				assertKeyedTuplesEqual(t, "outer", indexed, referenceOuterTwoStreamKeyedTuples(state, conditions, definition.kind, fixture.now, runtime))
			}
		})
	}
}

func TestJoinIndexOuterKindsMatchLegacyReference(t *testing.T) {
	names := []string{"JixOuterS0", "JixOuterS1"}
	fixture := newJixFixture(t, names)
	keyAlphabet := []string{"a", "b", "c", ""}
	random := rand.New(rand.NewSource(99))
	kinds := []JoinKind{JoinInner, JoinLeftOuter, JoinRightOuter, JoinFullOuter}
	for _, kind := range kinds {
		for round := 0; round < 30; round++ {
			state := &joinRuntimeState{sides: make([][]storedEvent, 2)}
			for side := range state.sides {
				rowCount := random.Intn(4)
				rows := make([]jixRow, 0, rowCount)
				for i := 0; i < rowCount; i++ {
					rows = append(rows, jixRow{Key: keyAlphabet[random.Intn(len(keyAlphabet))], Num: int64(random.Intn(2)), Ratio: float64(random.Intn(2))})
				}
				state.sides[side] = fixture.rows(t, names[side], rows)
			}
			conditions := []JoinCondition{OnSourcesEqual(0, Field[jixRow, string]("key"), 1, Field[jixRow, string]("key"))}
			definition := buildJixTwoQuery(t, fixture.env, names, conditions, kind)
			runtime := fixture.runtimeFor(t, Query{join: definition})
			indexed := joinKeyedTuples(definition, state, fixture.now, runtime)
			label := fmt.Sprintf("kind-%d", kind)
			if kind == JoinInner {
				assertKeyedTuplesEqual(t, label, indexed, referenceInnerKeyedTuples(state, conditions, fixture.now, runtime))
				continue
			}
			assertKeyedTuplesEqual(t, label, indexed, referenceOuterTwoStreamKeyedTuples(state, conditions, kind, fixture.now, runtime))
		}
	}
}

func buildJixTwoQuery(t *testing.T, env *Environment, names []string, conditions []JoinCondition, kind JoinKind) *joinDefinition {
	t.Helper()
	left := From[jixRow](env, names[0]).Window(KeepAll())
	right := From[jixRow](env, names[1]).Window(KeepAll())
	stream := Join(left, right, conditions...)
	switch kind {
	case JoinLeftOuter:
		stream = stream.LeftOuter()
	case JoinRightOuter:
		stream = stream.RightOuter()
	case JoinFullOuter:
		stream = stream.FullOuter()
	}
	query := stream.Select(SelectLeft("key", Field[jixRow, string]("key"))).Query(StatementName("jix-two"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan.query.join
}

func buildJixManyQuery(t *testing.T, env *Environment, names []string, conditions []JoinCondition) *joinDefinition {
	t.Helper()
	inputs := make([]JoinInput, len(names))
	for index, name := range names {
		inputs[index] = JoinSource(From[jixRow](env, name).Window(KeepAll()))
	}
	query := JoinMany(inputs...).On(conditions...).Select(SelectFrom(0, "key", JoinField[string](0, "key"))).Query(StatementName("jix-many"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan.query.join
}

func buildJixChainedQuery(t *testing.T, env *Environment, names []string, conditions []JoinCondition) *joinDefinition {
	t.Helper()
	chain := JoinChain(JoinSource(From[jixRow](env, names[0]).Window(KeepAll())))
	for index := 1; index < len(names); index++ {
		chain = chain.InnerJoin(JoinSource(From[jixRow](env, names[index]).Window(KeepAll())), conditions[index-1])
	}
	query := chain.Select(SelectFrom(0, "key", JoinField[string](0, "key"))).Query(StatementName("jix-chain"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan.query.join
}

type jixHistoricalProvider struct{}

func (jixHistoricalProvider) Poll(context.Context, HistoricalRequest) ([]Event, error) {
	return nil, nil
}

// TestJoinIndexActivationGate pins which shapes actually engage the index:
// regular event sources with extractable equi/IN keys above the tiny-window
// threshold take the indexed path; excluded shapes report the reference scan.
func TestJoinIndexActivationGate(t *testing.T) {
	names := []string{"JixGateS0", "JixGateS1"}
	fixture := newJixFixture(t, names)
	conditions := []JoinCondition{OnSourcesEqual(0, Field[jixRow, string]("key"), 1, Field[jixRow, string]("key"))}
	buildState := func(left, right int) (*joinRuntimeState, *statementRuntime, *joinDefinition) {
		state := &joinRuntimeState{sides: make([][]storedEvent, 2)}
		for side, count := range []int{left, right} {
			rows := make([]jixRow, 0, count)
			for i := 0; i < count; i++ {
				rows = append(rows, jixRow{Key: fmt.Sprintf("k%d", i)})
			}
			state.sides[side] = fixture.rows(t, names[side], rows)
		}
		definition := buildJixTwoQuery(t, fixture.env, names, conditions, JoinInner)
		runtime := fixture.runtimeFor(t, Query{join: definition})
		return state, runtime, definition
	}
	t.Run("engages-above-threshold", func(t *testing.T) {
		state, runtime, definition := buildState(2, 2)
		indexed, ok := joinInnerKeyedTuplesIndexed(definition, state, conditions, fixture.now, runtime)
		if !ok {
			t.Fatal("2x2 equi join must take the indexed path")
		}
		assertKeyedTuplesEqual(t, "gate", indexed, referenceInnerKeyedTuples(state, conditions, fixture.now, runtime))
	})
	t.Run("falls-back-below-threshold", func(t *testing.T) {
		_, runtime, definition := buildState(1, 1)
		state := &joinRuntimeState{sides: [][]storedEvent{
			{{event: fixture.event(t, names[0], jixRow{Key: "a"}), receivedAt: fixture.now, lineageID: 1}},
			{{event: fixture.event(t, names[1], jixRow{Key: "a"}), receivedAt: fixture.now, lineageID: 2}},
		}}
		if _, ok := joinInnerKeyedTuplesIndexed(definition, state, conditions, fixture.now, runtime); ok {
			t.Fatal("1x1 join must stay on the reference scan")
		}
	})
	t.Run("non-extractable-conditions", func(t *testing.T) {
		state, runtime, _ := buildState(3, 3)
		// Right expression reads source 0 from side 1's slot: not
		// own-source-only, so no index plan can be extracted.
		cross := []JoinCondition{OnSourcesEqual(0, JoinField[string](0, "key"), 1, JoinField[string](0, "ratio"))}
		if _, ok := joinInnerKeyedTuplesIndexed(buildJixTwoQuery(t, fixture.env, names, cross, JoinInner), state, cross, fixture.now, runtime); ok {
			t.Fatal("cross-source condition join must stay on the reference scan")
		}
	})
	t.Run("historical-source-excluded", func(t *testing.T) {
		env := fixture.env
		schema, ok := env.Schema(names[1])
		if !ok {
			t.Fatalf("schema %q not registered", names[1])
		}
		query := Join(
			From[jixRow](env, names[0]).Window(KeepAll()),
			FromHistorical[jixRow](env, names[1], schema, jixHistoricalProvider{}).Window(KeepAll()),
			OnEqual(Field[jixRow, string]("key"), Field[jixRow, string]("key")),
		).Select(SelectLeft("key", Field[jixRow, string]("key"))).Query(StatementName("jix-gate-historical"))
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		definition := plan.query.join
		state := &joinRuntimeState{sides: make([][]storedEvent, 2)}
		state.sides[0] = fixture.rows(t, names[0], []jixRow{{Key: "a"}, {Key: "b"}})
		state.sides[1] = fixture.rows(t, names[1], []jixRow{{Key: "a"}, {Key: "b"}})
		runtime := fixture.runtimeFor(t, Query{join: definition})
		if _, ok := joinInnerKeyedTuplesIndexed(definition, state, joinDefinitionConditions(definition), fixture.now, runtime); ok {
			t.Fatal("historical source joins must stay on the reference scan")
		}
	})
	t.Run("unidirectional-excluded", func(t *testing.T) {
		env := fixture.env
		query := JoinMany(
			JoinSource(From[jixRow](env, names[0])).Unidirectional(),
			JoinSource(From[jixRow](env, names[1]).Window(KeepAll())),
		).On(conditions...).Select(SelectFrom(1, "key", JoinField[string](1, "key"))).Query(StatementName("jix-gate-unidirectional"))
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		definition := plan.query.join
		state := &joinRuntimeState{sides: make([][]storedEvent, 2)}
		state.sides[0] = fixture.rows(t, names[0], []jixRow{{Key: "a"}, {Key: "b"}})
		state.sides[1] = fixture.rows(t, names[1], []jixRow{{Key: "a"}, {Key: "b"}})
		runtime := fixture.runtimeFor(t, Query{join: definition})
		if _, ok := joinInnerKeyedTuplesIndexed(definition, state, joinDefinitionConditions(definition), fixture.now, runtime); ok {
			t.Fatal("unidirectional joins must stay on the reference scan")
		}
	})
}

// ---------------------------------------------------------------------------
// Engine-level exact-output tests
// ---------------------------------------------------------------------------

// TestJoinIndexDuplicateCandidatesEmittedOnce pins the multi-alternative
// (IN/OR) probe: a right row matched by more than one alternative must still
// be emitted exactly once, in right-row order, exactly as the nested loop
// would emit it.
func TestJoinIndexDuplicateCandidatesEmittedOnce(t *testing.T) {
	env := NewEnvironment()
	for _, name := range []string{"JixDupLeft", "JixDupRight"} {
		if _, err := RegisterStruct[jixRow](env, name); err != nil {
			t.Fatal(err)
		}
	}
	leftKey := Field[jixRow, string]("key")
	rightKey := Field[jixRow, string]("key")
	leftNum := Field[jixRow, int64]("num")
	rightNum := Field[jixRow, int64]("num")
	query := Join(
		From[jixRow](env, "JixDupLeft").Window(KeepAll()),
		From[jixRow](env, "JixDupRight").Window(KeepAll()),
		// left.key == right.key OR left.num == right.num: the right row
		// {a,1} matches the left row {a,1} through BOTH branches and must be
		// emitted exactly once, like the nested loop emits it.
		AnyJoin(
			OnEqual(leftKey, rightKey),
			OnEqual(leftNum, rightNum),
		),
	).Select(SelectLeft("key", Field[jixRow, string]("key"))).Query(StatementName("jix-dup-once"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var emitted []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				value, _ := row.Get("key").Any().(string)
				emitted = append(emitted, value)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(name string, row jixRow) {
		t.Helper()
		if err := engine.Send(context.Background(), name, row); err != nil {
			t.Fatal(err)
		}
	}
	send("JixDupRight", jixRow{Key: "x", Num: 1})
	send("JixDupRight", jixRow{Key: "a", Num: 9})
	send("JixDupRight", jixRow{Key: "a", Num: 1})
	// The left row {a,1} joins: {x,1} via num, {a,9} via key, {a,1} via both
	// branches. All three right rows are emitted, in right-row order, with
	// the doubly-matching row appearing exactly once.
	send("JixDupLeft", jixRow{Key: "a", Num: 1})
	if len(emitted) != 3 {
		t.Fatalf("emitted %v (%d rows), want exactly 3 joins", emitted, len(emitted))
	}
	// The projection carries the left key for every join; the count is the
	// pin: a naive multi-bucket probe would emit {a,1} twice (4 rows).
	for _, value := range emitted {
		if value != "a" {
			t.Fatalf("unexpected emitted key %q in %v", value, emitted)
		}
	}
}

// TestJoinIndexNumericCoercionEndToEnd pins the numeric key equivalence: an
// int64 key on one side matches float64 keys on the other exactly like
// EqualValues coerces them.
func TestJoinIndexNumericCoercionEndToEnd(t *testing.T) {
	env := NewEnvironment()
	for _, name := range []string{"JixCoerceLeft", "JixCoerceRight"} {
		if _, err := RegisterStruct[jixRow](env, name); err != nil {
			t.Fatal(err)
		}
	}
	query := Join(
		From[jixRow](env, "JixCoerceLeft").Window(KeepAll()),
		From[jixRow](env, "JixCoerceRight").Window(KeepAll()),
		OnEqual(
			Field[jixRow, int64]("num"),
			Field[jixRow, float64]("ratio"),
		),
	).Select(SelectLeft("num", Field[jixRow, int64]("num"))).Query(StatementName("jix-coerce"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var emitted []int64
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				value, _ := row.Get("num").Any().(int64)
				emitted = append(emitted, value)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(name string, row jixRow) {
		t.Helper()
		if err := engine.Send(context.Background(), name, row); err != nil {
			t.Fatal(err)
		}
	}
	send("JixCoerceRight", jixRow{Ratio: 1.0})
	send("JixCoerceRight", jixRow{Ratio: 2.5})
	send("JixCoerceRight", jixRow{Ratio: 3})
	send("JixCoerceLeft", jixRow{Num: 1})
	// 1 matches ratio 1.0 only; 2.5 and 3 stay unmatched.
	if len(emitted) != 1 || emitted[0] != 1 {
		t.Fatalf("emitted %v, want [1]", emitted)
	}
	send("JixCoerceLeft", jixRow{Num: 3})
	if len(emitted) != 2 || emitted[1] != 3 {
		t.Fatalf("emitted %v, want [1 3]", emitted)
	}
}

// TestJoinIndexLeftOuterPaddingUnchanged pins the two-stream outer path: the
// index only replaces the matching loop; unmatched left rows still produce
// the null-padded tuple with the same lineage key and ordering.
func TestJoinIndexLeftOuterPaddingUnchanged(t *testing.T) {
	env := NewEnvironment()
	for _, name := range []string{"JixOuterLeft", "JixOuterRight"} {
		if _, err := RegisterStruct[jixRow](env, name); err != nil {
			t.Fatal(err)
		}
	}
	query := Join(
		From[jixRow](env, "JixOuterLeft").Window(KeepAll()),
		From[jixRow](env, "JixOuterRight").Window(KeepAll()),
		OnEqual(
			Field[jixRow, string]("key"),
			Field[jixRow, string]("key"),
		),
	).LeftOuter().Select(
		SelectLeft("key", Field[jixRow, string]("key")),
	).Query(StatementName("jix-outer-left"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var emitted []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				value, _ := row.Get("key").Any().(string)
				emitted = append(emitted, value)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(name string, row jixRow) {
		t.Helper()
		if err := engine.Send(context.Background(), name, row); err != nil {
			t.Fatal(err)
		}
	}
	// Build the right side beyond the 1x1 threshold so the index path is
	// active, then drive left rows: one unmatched (padded), one matching.
	send("JixOuterRight", jixRow{Key: "a"})
	send("JixOuterRight", jixRow{Key: "a"})
	send("JixOuterLeft", jixRow{Key: "lonely"})
	send("JixOuterLeft", jixRow{Key: "a"})
	if len(emitted) != 3 {
		t.Fatalf("emitted %v, want 3 rows", emitted)
	}
	if emitted[0] != "lonely" {
		t.Fatalf("first row %q, want the padded \"lonely\" row", emitted[0])
	}
	if emitted[1] != "a" || emitted[2] != "a" {
		t.Fatalf("matching rows %v, want [a a]", emitted[1:])
	}
}

// TestJoinIndexNullKeysNeverMatch pins three-valued equality on the indexed
// path: rows whose nullable key property is null (nil pointer) join nothing,
// exactly like the reference scan.
func TestJoinIndexNullKeysNeverMatch(t *testing.T) {
	env := NewEnvironment()
	for _, name := range []string{"JixNullLeft", "JixNullRight"} {
		if _, err := RegisterStruct[jixNullableRow](env, name); err != nil {
			t.Fatal(err)
		}
	}
	query := Join(
		From[jixNullableRow](env, "JixNullLeft").Window(KeepAll()),
		From[jixNullableRow](env, "JixNullRight").Window(KeepAll()),
		OnEqual(
			Field[jixNullableRow, string]("key"),
			Field[jixNullableRow, string]("key"),
		),
	).Select(SelectLeft("key", Field[jixNullableRow, string]("key"))).Query(StatementName("jix-null-keys"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	emitted := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		emitted += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(name string, row jixNullableRow) {
		t.Helper()
		if err := engine.Send(context.Background(), name, row); err != nil {
			t.Fatal(err)
		}
	}
	// Build the right side beyond the tiny-window threshold first.
	send("JixNullRight", jixNullableRow{Key: strPtr("a")})
	send("JixNullRight", jixNullableRow{Key: strPtr("b")})
	// Null keys can never satisfy an equi condition; they must not throw and
	// must not join, including with each other.
	send("JixNullLeft", jixNullableRow{})
	send("JixNullLeft", jixNullableRow{})
	send("JixNullLeft", jixNullableRow{Key: strPtr("a")})
	if emitted != 1 {
		t.Fatalf("emitted %d rows, want exactly the single a/a join", emitted)
	}
}
