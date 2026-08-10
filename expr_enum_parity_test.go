package esper

import (
	"testing"
)

type enumParityItem struct {
	ID    string `esper:"id"`
	Score int64  `esper:"score"`
}

func assertEnumEval[T any](t *testing.T, got Expression[T], want T, label string) {
	t.Helper()
	v := got.eval(EvalContext{})
	if !v.IsPresent() {
		t.Fatalf("%s: expected present, got %v", label, v)
	}
	if !v.Equal(Present(want)) {
		t.Fatalf("%s = %v, want %v", label, v, want)
	}
}

// TestExprEnumWhereMatchesEsper covers ExprEnumWhereEvents/ExprEnumWhereScalar/
// ExprEnumWhereScalarBoolean: collection filtering with element, index, and
// size lambda parameters.
func TestExprEnumWhereMatchesEsper(t *testing.T) {
	// Scalar: strvals.where(x => x not like '%1%')
	// Equivalent: filter strings not containing "1"
	strs := Literal([]string{"E1", "E2", "E3"})
	notOne := EnumWhere[string](strs, NotEqual[string](EnumElement[string](), Literal("E1")))
	assertEnumEval(t, notOne, []string{"E2", "E3"}, "where-not-E1")

	// With index: x not like '%1%' and i >= 1
	// Filter strings not containing "1" and index >= 1
	strs2 := Literal([]string{"E4", "E2", "E1"})
	filtered := EnumWhere[string](strs2, And(
		NotEqual[string](EnumElement[string](), Literal("E1")),
		GreaterOrEqual[int64](EnumIndex(), Literal(int64(1))),
	))
	assertEnumEval(t, filtered, []string{"E2"}, "where-index")

	// With size: x not like '%1%' and i >= 1 and s >= 3
	filtered2 := EnumWhere[string](strs2, And(
		NotEqual[string](EnumElement[string](), Literal("E1")),
		And(
			GreaterOrEqual[int64](EnumIndex(), Literal(int64(1))),
			GreaterOrEqual[int64](EnumSize(), Literal(int64(3))),
		),
	))
	assertEnumEval(t, filtered2, []string{"E2"}, "where-index-size")

	// Boolean: boolvals.where(x => x)
	bools := Literal([]bool{true, true, false})
	trueOnly := EnumWhere[bool](bools, Equal[bool](EnumElement[bool](), Literal(true)))
	assertEnumEval(t, trueOnly, []bool{true, true}, "where-bool")

	// Events: contained.where(x => p00 = 9)
	items := Literal([]enumParityItem{{ID: "E1", Score: 1}, {ID: "E2", Score: 9}, {ID: "E3", Score: 1}})
	score := EnumField[enumParityItem, int64]("score")
	events := EnumWhere[enumParityItem](items, Equal[int64](score, Literal(int64(9))))
	assertEnumEval(t, events, []enumParityItem{{ID: "E2", Score: 9}}, "where-events")
}

// TestExprEnumCountOfMatchesEsper covers ExprEnumCountOfEvents/ExprEnumCountOfScalar:
// counting elements matching a predicate.
func TestExprEnumCountOfMatchesEsper(t *testing.T) {
	// Scalar: strvals.countof(x => x like '%E%')
	strs := Literal([]string{"E1", "E2", "E3"})
	countAll := EnumCount[string](strs)
	assertEnumEval(t, countAll, int64(3), "count-all")

	countNonEmpty := EnumCountOf[string](strs, NotEqual[string](EnumElement[string](), Literal("")))
	assertEnumEval(t, countNonEmpty, int64(3), "count-of-non-empty")

	// Events: contained.countof(x => p00 = 9)
	items := Literal([]enumParityItem{{ID: "E1", Score: 1}, {ID: "E2", Score: 9}, {ID: "E3", Score: 9}})
	score := EnumField[enumParityItem, int64]("score")
	countNine := EnumCountOf[enumParityItem](items, Equal[int64](score, Literal(int64(9))))
	assertEnumEval(t, countNine, int64(2), "count-of-9")
}

// TestExprEnumReverseMatchesEsper covers ExprEnumReverseEvents/ExprEnumReverseScalar:
// reversing collection order.
func TestExprEnumReverseMatchesEsper(t *testing.T) {
	// Scalar
	strs := Literal([]string{"A", "B", "C"})
	reversed := EnumReverse[string](strs)
	assertEnumEval(t, reversed, []string{"C", "B", "A"}, "reverse-scalar")

	// Int
	ints := Literal([]int64{1, 2, 3})
	reversedInts := EnumReverse[int64](ints)
	assertEnumEval(t, reversedInts, []int64{3, 2, 1}, "reverse-int")

	// Events
	items := Literal([]enumParityItem{{ID: "E1", Score: 1}, {ID: "E2", Score: 2}})
	reversedItems := EnumReverse[enumParityItem](items)
	assertEnumEval(t, reversedItems, []enumParityItem{{ID: "E2", Score: 2}, {ID: "E1", Score: 1}}, "reverse-events")
}

// TestExprEnumAllOfAnyOfMatchesEsper covers ExprEnumAllOfAnyOfEvents/
// ExprEnumAllOfAnyOfScalar: quantified predicates over collections.
func TestExprEnumAllOfAnyOfMatchesEsper(t *testing.T) {
	// Scalar anyOf
	ints := Literal([]int64{1, 2, 3})
	hasTwo := EnumAnyOf[int64](ints, Equal[int64](EnumElement[int64](), Literal(int64(2))))
	assertEnumEval(t, hasTwo, true, "any-of-2")

	hasFour := EnumAnyOf[int64](ints, Equal[int64](EnumElement[int64](), Literal(int64(4))))
	assertEnumEval(t, hasFour, false, "any-of-4")

	// Scalar allOf
	allPositive := EnumAllOf[int64](ints, Greater[int64](EnumElement[int64](), Literal(int64(0))))
	assertEnumEval(t, allPositive, true, "all-of-positive")

	allBig := EnumAllOf[int64](ints, Greater[int64](EnumElement[int64](), Literal(int64(2))))
	assertEnumEval(t, allBig, false, "all-of-big")

	// Empty collection
	empty := Literal([]int64{})
	allOfEmpty := EnumAllOf[int64](empty, Literal(true))
	assertEnumEval(t, allOfEmpty, true, "all-of-empty")

	anyOfEmpty := EnumAnyOf[int64](empty, Literal(true))
	assertEnumEval(t, anyOfEmpty, false, "any-of-empty")

	// Events
	items := Literal([]enumParityItem{{ID: "E1", Score: 5}, {ID: "E2", Score: 10}})
	score := EnumField[enumParityItem, int64]("score")
	anyHigh := EnumAnyOf[enumParityItem](items, Greater[int64](score, Literal(int64(8))))
	assertEnumEval(t, anyHigh, true, "events-any-high")

	allHigh := EnumAllOf[enumParityItem](items, Greater[int64](score, Literal(int64(8))))
	assertEnumEval(t, allHigh, false, "events-all-high")
}

// TestExprEnumDistinctMatchesEsper covers ExprEnumDistinctEvents/
// ExprEnumDistinctScalar: deduplicating collection elements.
func TestExprEnumDistinctMatchesEsper(t *testing.T) {
	// Scalar
	ints := Literal([]int64{1, 2, 2, 3, 3, 3})
	distinct := EnumDistinct[int64](ints)
	assertEnumEval(t, distinct, []int64{1, 2, 3}, "distinct-int")

	strs := Literal([]string{"A", "B", "A", "C", "B"})
	distinctStrs := EnumDistinct[string](strs)
	assertEnumEval(t, distinctStrs, []string{"A", "B", "C"}, "distinct-string")

	// DistinctBy (with key selector)
	items := Literal([]enumParityItem{{ID: "E1", Score: 1}, {ID: "E2", Score: 2}, {ID: "E3", Score: 1}})
	distinctByScore := EnumDistinctBy[enumParityItem, int64](items, EnumField[enumParityItem, int64]("score"))
	result := distinctByScore.eval(EvalContext{})
	if !result.IsPresent() {
		t.Fatal("distinct-by should be present")
	}
	if len(result.Any().([]enumParityItem)) != 2 {
		t.Fatalf("distinct-by = %d items, want 2", len(result.Any().([]enumParityItem)))
	}
}

// TestExprEnumFirstLastOfMatchesEsper covers ExprEnumFirstLastScalar/
// ExprEnumFirstLastEventProperty/ExprEnumFirstLastEvent.
func TestExprEnumFirstLastOfMatchesEsper(t *testing.T) {
	// Scalar
	ints := Literal([]int64{3, 1, 4, 1, 5})
	first := EnumFirstOf[int64](ints, Equal[int64](EnumElement[int64](), Literal(int64(1))))
	assertEnumEval(t, first, int64(1), "first-of-1")

	last := EnumLastOf[int64](ints, Equal[int64](EnumElement[int64](), Literal(int64(1))))
	assertEnumEval(t, last, int64(1), "last-of-1")

	// FirstOf without predicate returns first element
	firstNoPred := EnumFirstOf[int64](ints)
	assertEnumEval(t, firstNoPred, int64(3), "first-no-pred")

	lastNoPred := EnumLastOf[int64](ints)
	assertEnumEval(t, lastNoPred, int64(5), "last-no-pred")

	// Events
	items := Literal([]enumParityItem{{ID: "E1", Score: 2}, {ID: "E2", Score: 9}, {ID: "E3", Score: 2}})
	score := EnumField[enumParityItem, int64]("score")
	firstScore2 := EnumFirstOf[enumParityItem](items, Equal[int64](score, Literal(int64(2))))
	assertEnumEval(t, firstScore2, enumParityItem{ID: "E1", Score: 2}, "events-first-2")

	lastScore2 := EnumLastOf[enumParityItem](items, Equal[int64](score, Literal(int64(2))))
	assertEnumEval(t, lastScore2, enumParityItem{ID: "E3", Score: 2}, "events-last-2")
}

// TestExprEnumMinMaxMatchesEsper covers ExprEnumMinMaxScalar/
// ExprEnumMinMaxScalarChain: minimum and maximum of collections.
func TestExprEnumMinMaxMatchesEsper(t *testing.T) {
	ints := Literal([]int64{3, 1, 4, 1, 5})
	assertEnumEval(t, EnumMin[int64](ints), int64(1), "min")
	assertEnumEval(t, EnumMax[int64](ints), int64(5), "max")

	strs := Literal([]string{"banana", "apple", "cherry"})
	assertEnumEval(t, EnumMin[string](strs), "apple", "min-string")
	assertEnumEval(t, EnumMax[string](strs), "cherry", "max-string")

	// MinOf/MaxOf with selector
	items := Literal([]enumParityItem{{ID: "E1", Score: 2}, {ID: "E2", Score: 9}, {ID: "E3", Score: 5}})
	score := EnumField[enumParityItem, int64]("score")
	assertEnumEval(t, EnumMinOf[enumParityItem, int64](items, score), int64(2), "min-of")
	assertEnumEval(t, EnumMaxOf[enumParityItem, int64](items, score), int64(9), "max-of")
}

// TestExprEnumSumOfMatchesEsper covers ExprEnumSumEvents/ExprEnumSumScalar:
// sum of collection values.
func TestExprEnumSumOfMatchesEsper(t *testing.T) {
	ints := Literal([]int64{1, 2, 3})
	assertEnumEval(t, EnumSum[int64](ints), int64(6), "sum-int")

	// SumOf with selector
	items := Literal([]enumParityItem{{ID: "E1", Score: 10}, {ID: "E2", Score: 20}})
	score := EnumField[enumParityItem, int64]("score")
	assertEnumEval(t, EnumSumOf[enumParityItem, int64](items, score), int64(30), "sum-of")
}

// TestExprEnumAverageOfMatchesEsper covers ExprEnumAverageEvents/
// ExprEnumAverageScalar: average of collection values.
func TestExprEnumAverageOfMatchesEsper(t *testing.T) {
	ints := Literal([]int64{1, 2, 3})
	assertEnumEval(t, EnumAverage[int64](ints), float64(2.0), "average")

	// AverageOf with selector
	items := Literal([]enumParityItem{{ID: "E1", Score: 10}, {ID: "E2", Score: 20}})
	score := EnumField[enumParityItem, int64]("score")
	assertEnumEval(t, EnumAverageOf[enumParityItem, int64](items, score), float64(15.0), "average-of")
}

// TestExprEnumOrderByMatchesEsper covers ExprEnumOrderByEvents/
// ExprEnumOrderByEventsPlus: sorting collections.
func TestExprEnumOrderByMatchesEsper(t *testing.T) {
	// Natural order
	ints := Literal([]int64{3, 1, 2})
	sorted := EnumOrderByNatural[int64](ints, false)
	assertEnumEval(t, sorted, []int64{1, 2, 3}, "order-natural")

	sortedDesc := EnumOrderByNatural[int64](ints, true)
	assertEnumEval(t, sortedDesc, []int64{3, 2, 1}, "order-natural-desc")

	// With key selector
	items := Literal([]enumParityItem{{ID: "C", Score: 3}, {ID: "A", Score: 1}, {ID: "B", Score: 2}})
	score := EnumField[enumParityItem, int64]("score")
	byScore := EnumOrderBy[enumParityItem, int64](items, score, false)
	assertEnumEval(t, byScore, []enumParityItem{{ID: "A", Score: 1}, {ID: "B", Score: 2}, {ID: "C", Score: 3}}, "order-by-score")
}

// TestExprEnumTakeMatchesEsper covers ExprEnumTakeWhileEvents/
// ExprEnumTakeWhileScalar and take/takeLast operations.
func TestExprEnumTakeMatchesEsper(t *testing.T) {
	ints := Literal([]int64{1, 2, 3, 4, 5})

	assertEnumEval(t, EnumTake[int64](ints, 2), []int64{1, 2}, "take-2")
	assertEnumEval(t, EnumTakeLast[int64](ints, 2), []int64{4, 5}, "take-last-2")

	// TakeWhile
	lessThan3 := EnumTakeWhile[int64](ints, Less[int64](EnumElement[int64](), Literal(int64(3))))
	assertEnumEval(t, lessThan3, []int64{1, 2}, "take-while-lt3")

	// TakeWhileLast
	greaterThan3 := EnumTakeWhileLast[int64](ints, Greater[int64](EnumElement[int64](), Literal(int64(3))))
	assertEnumEval(t, greaterThan3, []int64{4, 5}, "take-while-last-gt3")
}

// TestExprEnumMinMaxByMatchesEsper covers ExprEnumMinMaxByEvents/
// ExprEnumMinMaxByScalar: min/max by selector returning the element.
func TestExprEnumMinMaxByMatchesEsper(t *testing.T) {
	items := Literal([]enumParityItem{{ID: "E1", Score: 2}, {ID: "E2", Score: 9}, {ID: "E3", Score: 5}})
	score := EnumField[enumParityItem, int64]("score")

	minItem := EnumMinBy[enumParityItem, int64](items, score)
	assertEnumEval(t, minItem, enumParityItem{ID: "E1", Score: 2}, "min-by")

	maxItem := EnumMaxBy[enumParityItem, int64](items, score)
	assertEnumEval(t, maxItem, enumParityItem{ID: "E2", Score: 9}, "max-by")
}

// TestExprEnumSelectMatchesEsper covers EnumSelect projection and
// EnumSelectMap operations.
func TestExprEnumSelectMatchesEsper(t *testing.T) {
	// Scalar select with element+index
	ints := Literal([]int64{1, 2, 3})
	selected := EnumSelect[int64, int64](ints, Add[int64](EnumElement[int64](), EnumIndex()))
	assertEnumEval(t, selected, []int64{1, 3, 5}, "select-index")

	// Event select - project to ID field
	items := Literal([]enumParityItem{{ID: "E1", Score: 2}, {ID: "E2", Score: 9}})
	ids := EnumSelect[enumParityItem, string](items, EnumField[enumParityItem, string]("id"))
	assertEnumEval(t, ids, []string{"E1", "E2"}, "select-event-id")
}
