package esper

import (
	"testing"
)

func docSampleItoa(v int64) string { return string(rune('0' + v)) }

// TestExprEnumDocSamplesScalarArrayParity mirrors Java ExprEnumScalarArray:
// comprehensive scalar-array enum-method validation with element, index and
// size lambda variants.
func TestExprEnumDocSamplesScalarArrayParity(t *testing.T) {
	three := Literal([]int64{1, 2, 3})
	five := Literal([]int64{1, 2, 3, 2, 1})

	// aggregate with init, index and size
	assertEnumEval(t, EnumAggregate[int64, int64](three, Literal(int64(0)), Add[int64](EnumAccumulator[int64](), EnumElement[int64]())), int64(6), "aggregate-init")
	idx10 := Multiply[int64](Literal(int64(10)), EnumIndex())
	sz100 := Multiply[int64](Literal(int64(100)), EnumSize())
	assertEnumEval(t, EnumAggregate[int64, int64](three, Literal(int64(0)), Add[int64](EnumAccumulator[int64](), Add[int64](EnumElement[int64](), idx10))), int64(36), "aggregate-init-index")
	assertEnumEval(t, EnumAggregate[int64, int64](three, Literal(int64(0)), Add[int64](EnumAccumulator[int64](), Add[int64](EnumElement[int64](), Add[int64](idx10, sz100)))), int64(936), "aggregate-init-index-size")

	// average with selector, index and size
	assertEnumEval(t, EnumAverage[int64](three), 2.0, "average")
	assertEnumEval(t, EnumAverageOf[int64, int64](three, Add[int64](EnumElement[int64](), Literal(int64(1)))), 3.0, "average-selector")
	assertEnumEval(t, EnumAverageOf[int64, int64](three, Add[int64](EnumElement[int64](), idx10)), 12.0, "average-index")
	assertEnumEval(t, EnumAverageOf[int64, int64](three, Add[int64](Add[int64](EnumElement[int64](), idx10), sz100)), 312.0, "average-index-size")

	// sumOf with selector, index and size
	assertEnumEval(t, EnumSum[int64](three), int64(6), "sum")
	assertEnumEval(t, EnumSumOf[int64, int64](three, Add[int64](EnumElement[int64](), Literal(int64(1)))), int64(9), "sum-selector")
	assertEnumEval(t, EnumSumOf[int64, int64](three, Add[int64](EnumElement[int64](), EnumIndex())), int64(9), "sum-index")
	assertEnumEval(t, EnumSumOf[int64, int64](three, Add[int64](Add[int64](EnumElement[int64](), EnumIndex()), EnumSize())), int64(18), "sum-index-size")

	// countOf with element, index and size
	assertEnumEval(t, EnumCountOf[int64](three, nil), int64(3), "count")
	assertEnumEval(t, EnumCountOf[int64](three, Less[int64](EnumElement[int64](), Literal(int64(2)))), int64(1), "count-selector")
	assertEnumEval(t, EnumCountOf[int64](three, Greater[int64](EnumElement[int64](), EnumIndex())), int64(3), "count-index")
	assertEnumEval(t, EnumCountOf[int64](three, GreaterOrEqual[int64](EnumElement[int64](), EnumSize())), int64(1), "count-index-size")

	// min with selector, index and size
	assertEnumEval(t, EnumMinOf[int64, int64](five, Add[int64](EnumElement[int64](), Literal(int64(1)))), int64(2), "min-selector")
	assertEnumEval(t, EnumMinOf[int64, int64](five, Subtract[int64](EnumElement[int64](), EnumIndex())), int64(-3), "min-index")
	assertEnumEval(t, EnumMinOf[int64, int64](five, Subtract[int64](EnumElement[int64](), EnumSize())), int64(-4), "min-index-size")

	// maxBy / minBy with selector and index
	assertEnumEval(t, EnumMaxBy[int64, int64](five, EnumElement[int64]()), int64(3), "maxby")
	assertEnumEval(t, EnumMinBy[int64, int64](five, EnumElement[int64]()), int64(1), "minby")
	assertEnumEval(t, EnumMinBy[int64, int64](five, Case[int64]().When(Less[int64](EnumIndex(), Literal(int64(3))), Literal(int64(-1))).Else(Literal(int64(0)))), int64(1), "minby-index")

	// mostFrequent / leastFrequent
	assertEnumEval(t, EnumMostFrequent[int64](Literal([]int64{1, 2, 3, 2, 1, 2})), int64(2), "most-frequent")
	assertEnumEval(t, EnumLeastFrequent[int64](Literal([]int64{1, 2, 3, 2, 1})), int64(3), "least-frequent")

	// orderBy ascending and descending
	if v := EnumOrderBy[int64, int64](Literal([]int64{2, 3, 2, 1}), EnumElement[int64](), false).eval(EvalContext{}); !v.Equal(Present([]int64{1, 2, 2, 3})) {
		t.Fatalf("orderBy asc = %v", v)
	}
	if v := EnumOrderBy[int64, int64](Literal([]int64{2, 3, 2, 1}), Negate[int64](EnumElement[int64]()), false).eval(EvalContext{}); !v.Equal(Present([]int64{3, 2, 2, 1})) {
		t.Fatalf("orderBy selector = %v", v)
	}

	// distinctOf and reverse
	if v := EnumDistinct[int64](Literal([]int64{2, 3, 2, 1})).eval(EvalContext{}); !v.Equal(Present([]int64{2, 3, 1})) {
		t.Fatalf("distinct = %v", v)
	}
	if v := EnumReverse[int64](Literal([]int64{2, 3, 2, 1})).eval(EvalContext{}); !v.Equal(Present([]int64{1, 2, 3, 2})) {
		t.Fatalf("reverse = %v", v)
	}

	// selectFrom with index
	if v := EnumSelect[string, string](Literal([]string{"A", "B", "C"}), Concat(EnumElement[string](), Concat(Literal("_"), Func1[int64, string]("itoa", docSampleItoa, EnumIndex())))).eval(EvalContext{}); !v.Equal(Present([]string{"A_0", "B_1", "C_2"})) {
		t.Fatalf("selectFrom index = %v", v)
	}

	// takeWhile with selector
	if v := EnumTakeWhile[int64](Literal([]int64{1, 2, 3}), Less[int64](EnumElement[int64](), Literal(int64(3)))).eval(EvalContext{}); !v.Equal(Present([]int64{1, 2})) {
		t.Fatalf("takeWhile = %v", v)
	}

	// groupby with element key
	gb := EnumGroupBy[int64, string](three, Concat(Literal("K"), Func1[int64, string]("itoa", docSampleItoa, EnumElement[int64]())))
	if v := gb.eval(EvalContext{}); !v.Equal(Present(map[string][]int64{"K1": {1}, "K2": {2}, "K3": {3}})) {
		t.Fatalf("groupby = %v", v)
	}

	// toMap with element key and value
	kv := Func1[int64, string]("kv", docSampleItoa, EnumElement[int64]())
	tm := EnumToMap[int64, string, string](three, Concat(Literal("K"), kv), Concat(Literal("V"), kv))
	if v := tm.eval(EvalContext{}); !v.Equal(Present(map[string]string{"K1": "V1", "K2": "V2", "K3": "V3"})) {
		t.Fatalf("toMap = %v", v)
	}

	// arrayOf and arrayOfSelect
	if v := EnumArrayOf[int64](three).eval(EvalContext{}); !v.Equal(Present([]int64{1, 2, 3})) {
		t.Fatalf("arrayOf = %v", v)
	}
	if v := EnumArrayOfSelect[int64, int64](three, Add[int64](EnumElement[int64](), Literal(int64(1)))).eval(EvalContext{}); !v.Equal(Present([]int64{2, 3, 4})) {
		t.Fatalf("arrayOfSelect = %v", v)
	}

	// sequenceEqual, union, except, intersect
	assertEnumEval(t, EnumSequenceEqual[int64](Literal([]int64{1, 2, 3}), Literal([]int64{1, 2, 3})), true, "sequenceEqual-true")
	assertEnumEval(t, EnumUnion[int64](Literal([]int64{1, 2, 3}), Literal([]int64{4, 5})), []int64{1, 2, 3, 4, 5}, "union")
	assertEnumEval(t, EnumExcept[int64](Literal([]int64{1, 2, 3}), Literal([]int64{1})), []int64{2, 3}, "except")
	assertEnumEval(t, EnumIntersect[int64](Literal([]int64{1, 2, 3}), Literal([]int64{2, 3})), []int64{2, 3}, "intersect")

	// where with element and index
	assertEnumEval(t, EnumWhere[int64](Literal([]int64{1, 2, 3}), NotEqual[int64](EnumElement[int64](), Literal(int64(2)))), []int64{1, 3}, "where-element")
	assertEnumEval(t, EnumWhere[int64](Literal([]int64{1, 2, 3}), And(NotEqual[int64](EnumElement[int64](), Literal(int64(2))), Less[int64](EnumIndex(), Literal(int64(2))))), []int64{1}, "where-index")
}

// TestExprEnumDocSamplesChainedWhereParity mirrors Java ExprEnumHowToUse: a
// chained .where().where() expression applied to items with nested location
// properties, filtering to items where both location.x=0 and location.y=0.
func TestExprEnumDocSamplesChainedWhereParity(t *testing.T) {
	type docSampleLocation struct {
		X int64 `esper:"x"`
		Y int64 `esper:"y"`
	}
	type docSampleItem struct {
		AssetId  string            `esper:"assetId"`
		Location docSampleLocation `esper:"location"`
	}

	items := Literal([]docSampleItem{
		{AssetId: "P00002", Location: docSampleLocation{X: 10, Y: 10}},
		{AssetId: "P00020", Location: docSampleLocation{X: 0, Y: 0}},
		{AssetId: "L001", Location: docSampleLocation{X: 5, Y: 0}},
	})

	// items.where(x => x.location.x = 0 and x.location.y = 0)
	predicate := And(Equal[int64](EnumField[docSampleItem, int64]("location.x"), Literal(int64(0))), Equal[int64](EnumField[docSampleItem, int64]("location.y"), Literal(int64(0))))
	result := EnumWhere[docSampleItem](items, predicate)
	v := result.eval(EvalContext{})
	if !v.IsPresent() {
		t.Fatalf("chained-where compound = %v, want present", v)
	}
	got, err := As[[]docSampleItem](v)
	if err != nil {
		t.Fatalf("cast error: %v", err)
	}
	if len(got) != 1 || got[0].AssetId != "P00020" {
		t.Fatalf("chained-where compound = %+v, want only P00020", got)
	}

	// Chained .where(x => x.location.x = 0).where(x => x.location.y = 0)
	first := EnumWhere[docSampleItem](items, Equal[int64](EnumField[docSampleItem, int64]("location.x"), Literal(int64(0))))
	chained := EnumWhere[docSampleItem](first, Equal[int64](EnumField[docSampleItem, int64]("location.y"), Literal(int64(0))))
	v2 := chained.eval(EvalContext{})
	if !v2.IsPresent() {
		t.Fatalf("chained-where sequential = %v, want present", v2)
	}
	got2, err := As[[]docSampleItem](v2)
	if err != nil {
		t.Fatalf("cast error: %v", err)
	}
	if len(got2) != 1 || got2[0].AssetId != "P00020" {
		t.Fatalf("chained-where sequential = %+v, want only P00020", got2)
	}
}
