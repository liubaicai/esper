package esper

import (
	"context"
	"iter"
	"maps"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"
)

type enumExpressionItem struct {
	ID    string `esper:"id"`
	Score int64  `esper:"score"`
}

type enumExpressionContainer struct {
	Values []int64              `esper:"values"`
	Items  []enumExpressionItem `esper:"items"`
}

type enumDecimalItem struct {
	ID     string  `esper:"id"`
	Amount big.Rat `esper:"amount"`
}

type enumExpressionNumbers []int64

type enumExpressionIterator struct {
	values []int64
	index  int
}

func (iterator *enumExpressionIterator) Next() (int64, bool) {
	if iterator.index >= len(iterator.values) {
		return 0, false
	}
	value := iterator.values[iterator.index]
	iterator.index++
	return value, true
}

func TestEnumerableExpressionsUseElementIndexAndSize(t *testing.T) {
	values := Literal([]int64{1, 2, 3})
	element := EnumElement[int64]()

	if got := EnumWhere[int64](values, Greater[int64](element, Literal(int64(1)))).eval(EvalContext{}); !got.Equal(Present([]int64{2, 3})) {
		t.Fatalf("where = %v", got)
	}
	if got := EnumSelect[int64, int64](values, Add[int64](element, EnumIndex())).eval(EvalContext{}); !got.Equal(Present([]int64{1, 3, 5})) {
		t.Fatalf("select with index = %v", got)
	}
	if got := EnumCount[int64](values).eval(EvalContext{}); !got.Equal(Present(int64(3))) {
		t.Fatalf("count = %v", got)
	}
	if got := EnumCountOf[int64](values, Greater[int64](element, Literal(int64(1)))).eval(EvalContext{}); !got.Equal(Present(int64(2))) {
		t.Fatalf("count-of = %v", got)
	}
	if got := EnumAnyOf[int64](values, Equal[int64](element, Literal(int64(2)))).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("any-of = %v", got)
	}
	if got := EnumAllOf[int64](values, Greater[int64](element, Literal(int64(0)))).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("all-of = %v", got)
	}
	if got := EnumAllOf[int64](Literal([]int64{}), Literal(true)); !got.eval(EvalContext{}).Equal(Present(true)) {
		t.Fatalf("all-of empty = %v", got.eval(EvalContext{}))
	}
	if got := EnumAnyOf[int64](Literal([]int64{}), Literal(true)); !got.eval(EvalContext{}).Equal(Present(false)) {
		t.Fatalf("any-of empty = %v", got.eval(EvalContext{}))
	}

	withSize := EnumSelect[int64, int64](values, Add[int64](EnumIndex(), EnumSize()))
	if got := withSize.eval(EvalContext{}); !got.Equal(Present([]int64{3, 4, 5})) {
		t.Fatalf("select with size = %v", got)
	}
}

func TestEnumerableEventElementsAndOrdering(t *testing.T) {
	items := Literal([]enumExpressionItem{
		{ID: "E1", Score: 2},
		{ID: "E2", Score: 9},
		{ID: "E3", Score: 2},
	})
	score := EnumField[enumExpressionItem, int64]("score")

	filtered := EnumWhere[enumExpressionItem](items, Greater[int64](score, Literal(int64(2))))
	if got := filtered.eval(EvalContext{}); !got.Equal(Present([]enumExpressionItem{{ID: "E2", Score: 9}})) {
		t.Fatalf("event where = %v", got)
	}
	first := EnumFirstOf[enumExpressionItem](items, Equal[int64](score, Literal(int64(2))))
	last := EnumLastOf[enumExpressionItem](items, Equal[int64](score, Literal(int64(2))))
	if got := first.eval(EvalContext{}); !got.Equal(Present(enumExpressionItem{ID: "E1", Score: 2})) {
		t.Fatalf("first-of = %v", got)
	}
	if got := last.eval(EvalContext{}); !got.Equal(Present(enumExpressionItem{ID: "E3", Score: 2})) {
		t.Fatalf("last-of = %v", got)
	}
	if got := EnumMinBy[enumExpressionItem, int64](items, score).eval(EvalContext{}); !got.Equal(Present(enumExpressionItem{ID: "E1", Score: 2})) {
		t.Fatalf("min-by keeps first tie = %v", got)
	}
	if got := EnumMaxBy[enumExpressionItem, int64](items, score).eval(EvalContext{}); !got.Equal(Present(enumExpressionItem{ID: "E2", Score: 9})) {
		t.Fatalf("max-by = %v", got)
	}
	if got := EnumOrderBy[enumExpressionItem, int64](items, score, false).eval(EvalContext{}); !got.Equal(Present([]enumExpressionItem{{ID: "E1", Score: 2}, {ID: "E3", Score: 2}, {ID: "E2", Score: 9}})) {
		t.Fatalf("order-by = %v", got)
	}
	if got := EnumSelect[enumExpressionItem, string](items, EnumField[enumExpressionItem, string]("id")).eval(EvalContext{}); !got.Equal(Present([]string{"E1", "E2", "E3"})) {
		t.Fatalf("event select = %v", got)
	}
	if got := EnumArrayOf[int64](Literal([]int64{1, 2})).eval(EvalContext{}); !got.Equal(Present([]int64{1, 2})) {
		t.Fatalf("array-of = %v", got)
	}
}

func TestEnumerableCollectionNullEmptyAndSetSemantics(t *testing.T) {
	nullValues := NullLiteral[[]int64]()
	predicate := Greater[int64](EnumElement[int64](), Literal(int64(0)))
	checks := []struct {
		name string
		got  Value
	}{
		{"where", EnumWhere[int64](nullValues, predicate).eval(EvalContext{})},
		{"select", EnumSelect[int64, int64](nullValues, EnumElement[int64]()).eval(EvalContext{})},
		{"any-of", EnumAnyOf[int64](nullValues, predicate).eval(EvalContext{})},
		{"all-of", EnumAllOf[int64](nullValues, predicate).eval(EvalContext{})},
		{"first-of", EnumFirstOf[int64](nullValues).eval(EvalContext{})},
		{"take", EnumTake[int64](nullValues, 1).eval(EvalContext{})},
		{"average", EnumAverage[int64](nullValues).eval(EvalContext{})},
	}
	for _, check := range checks {
		if !check.got.IsNull() {
			t.Errorf("%s on null collection = %v, want null", check.name, check.got)
		}
	}

	empty := Literal([]int64{})
	if got := EnumDistinct[int64](empty).eval(EvalContext{}); !got.Equal(Present([]int64{})) {
		t.Fatalf("distinct empty = %v", got)
	}
	if got := EnumReverse[int64](empty).eval(EvalContext{}); !got.Equal(Present([]int64{})) {
		t.Fatalf("reverse empty = %v", got)
	}
	if got := EnumTakeLast[int64](empty, 2).eval(EvalContext{}); !got.Equal(Present([]int64{})) {
		t.Fatalf("take-last empty = %v", got)
	}
	if got := EnumMin[int64](empty).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("min empty = %v", got)
	}

	duplicates := Literal([]int64{2, 1, 2, 1, 3})
	if got := EnumDistinct[int64](duplicates).eval(EvalContext{}); !got.Equal(Present([]int64{2, 1, 3})) {
		t.Fatalf("distinct = %v", got)
	}
	if got := EnumTake[int64](duplicates, -1).eval(EvalContext{}); !got.Equal(Present([]int64{})) {
		t.Fatalf("negative take = %v", got)
	}
	if got := EnumTakeWhile[int64](duplicates, Less[int64](EnumElement[int64](), Literal(int64(3)))).eval(EvalContext{}); !got.Equal(Present([]int64{2, 1, 2, 1})) {
		t.Fatalf("take-while = %v", got)
	}
	suffix := Literal([]int64{3, 1, 2})
	if got := EnumTakeWhileLast[int64](suffix, Less[int64](EnumElement[int64](), Literal(int64(3)))).eval(EvalContext{}); !got.Equal(Present([]int64{1, 2})) {
		t.Fatalf("take-while-last = %v", got)
	}
}

func TestEnumerableNumericMethodsAndPlanIntegration(t *testing.T) {
	values := Literal([]int64{1, 2, 3, 4})
	if got := EnumSum[int64](values).eval(EvalContext{}); !got.Equal(Present(int64(10))) {
		t.Fatalf("sum = %v", got)
	}
	if got := EnumAverage[int64](values).eval(EvalContext{}); !got.Equal(Present(float64(2.5))) {
		t.Fatalf("average = %v", got)
	}
	if got := EnumMin[int64](values).eval(EvalContext{}); !got.Equal(Present(int64(1))) {
		t.Fatalf("min = %v", got)
	}
	if got := EnumMax[int64](values).eval(EvalContext{}); !got.Equal(Present(int64(4))) {
		t.Fatalf("max = %v", got)
	}
	if got := EnumSumOf[enumExpressionItem, int64](Literal([]enumExpressionItem{{Score: 2}, {Score: 5}}), EnumField[enumExpressionItem, int64]("score")).eval(EvalContext{}); !got.Equal(Present(int64(7))) {
		t.Fatalf("sum-of = %v", got)
	}

	env := NewEnvironment()
	if _, err := RegisterStruct[enumExpressionContainer](env, "EnumContainer"); err != nil {
		t.Fatal(err)
	}
	stream := From[enumExpressionContainer](env, "EnumContainer")
	field := Field[enumExpressionContainer, []int64]("values")
	query := Select(stream, Alias("filtered", EnumWhere[int64](field, Greater[int64](EnumElement[int64](), Literal(int64(1))))))
	plan, err := env.Build(query.Query(StatementName("enum")))
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(string(plan.Canonical()), "where(values") {
		t.Fatalf("canonical plan does not include enum operator: %s", plan.Canonical())
	}

	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batch ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, received ResultBatch) error {
		batch = received
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), enumExpressionContainer{Values: []int64{1, 2, 3}}); err != nil {
		t.Fatal(err)
	}
	if len(batch.New) != 1 {
		t.Fatalf("new result count = %d", len(batch.New))
	}
	row, ok := batch.New[0].Row()
	if !ok {
		t.Fatalf("expected projection row, got %#v", batch.New[0])
	}
	filtered, err := As[[]int64](row.Get("filtered"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(filtered, []int64{2, 3}) {
		t.Fatalf("runtime filtered = %#v", filtered)
	}
}

func TestEnumerableSupportsExactBigNumbersAndArrayCollections(t *testing.T) {
	largeOne := *big.NewInt(9007199254740993001)
	largeTwo := *big.NewInt(9007199254740993002)
	largeValues := Literal([]big.Int{largeOne, largeTwo})
	largeSum, err := As[big.Int](EnumSum[big.Int](largeValues).eval(EvalContext{}))
	if err != nil {
		t.Fatal(err)
	}
	wantSum := new(big.Int).Add(&largeOne, &largeTwo)
	if largeSum.Cmp(wantSum) != 0 {
		t.Fatalf("big integer sum = %s, want %s", largeSum.String(), wantSum.String())
	}
	largeMin, err := As[big.Int](EnumMin[big.Int](largeValues).eval(EvalContext{}))
	if err != nil {
		t.Fatal(err)
	}
	if largeMin.Cmp(&largeOne) != 0 {
		t.Fatalf("big integer min = %s, want %s", largeMin.String(), largeOne.String())
	}

	third := *big.NewRat(1, 3)
	twoThirds := *big.NewRat(2, 3)
	rationalValues := Literal([]big.Rat{third, twoThirds})
	exactAverage, err := As[big.Rat](EnumAverageExact[big.Rat](rationalValues).eval(EvalContext{}))
	if err != nil {
		t.Fatal(err)
	}
	wantAverage := *big.NewRat(1, 2)
	if exactAverage.Cmp(&wantAverage) != 0 {
		t.Fatalf("exact rational average = %s, want %s", exactAverage.RatString(), wantAverage.RatString())
	}
	if got := EnumSum[big.Rat](rationalValues).eval(EvalContext{}); !got.IsPresent() {
		t.Fatalf("rational sum = %v", got)
	}

	items := Literal([]enumDecimalItem{
		{ID: "A", Amount: *big.NewRat(7, 10)},
		{ID: "B", Amount: *big.NewRat(1, 10)},
	})
	amount := EnumField[enumDecimalItem, big.Rat]("amount")
	selectedSum, err := As[big.Rat](EnumSumOf[enumDecimalItem, big.Rat](items, amount).eval(EvalContext{}))
	if err != nil {
		t.Fatal(err)
	}
	wantSelectedSum := *big.NewRat(4, 5)
	if selectedSum.Cmp(&wantSelectedSum) != 0 {
		t.Fatalf("selected rational sum = %s, want %s", selectedSum.RatString(), wantSelectedSum.RatString())
	}
	minimum, err := As[enumDecimalItem](EnumMinBy[enumDecimalItem, big.Rat](items, amount).eval(EvalContext{}))
	if err != nil {
		t.Fatal(err)
	}
	if minimum.ID != "B" {
		t.Fatalf("selected rational minimum = %#v", minimum)
	}

	array := Literal([3]int64{4, 5, 6})
	collected := EnumCollect[int64](array)
	if got := EnumSum[int64](collected).eval(EvalContext{}); !got.Equal(Present(int64(15))) {
		t.Fatalf("array collection sum = %v", got)
	}
	namedSlice := EnumCollect[int64](Literal(enumExpressionNumbers{7, 8, 9}))
	if got := EnumCount[int64](namedSlice).eval(EvalContext{}); !got.Equal(Present(int64(3))) {
		t.Fatalf("named slice collection count = %v", got)
	}
}

func TestEnumerableSelectorExtremesNaturalOrderAndDynamicTake(t *testing.T) {
	items := Literal([]enumExpressionItem{
		{ID: "E1", Score: 12},
		{ID: "E2", Score: 11},
		{ID: "E3", Score: 2},
	})
	score := EnumField[enumExpressionItem, int64]("score")
	if got := EnumMinOf[enumExpressionItem, int64](items, score).eval(EvalContext{}); !got.Equal(Present(int64(2))) {
		t.Fatalf("min selector = %v", got)
	}
	if got := EnumMaxOf[enumExpressionItem, int64](items, score).eval(EvalContext{}); !got.Equal(Present(int64(12))) {
		t.Fatalf("max selector = %v", got)
	}

	words := Literal([]string{"E2", "E1", "E5", "E4"})
	if got := EnumOrderByNatural[string](words, false).eval(EvalContext{}); !got.Equal(Present([]string{"E1", "E2", "E4", "E5"})) {
		t.Fatalf("natural ascending order = %v", got)
	}
	if got := EnumOrderByNatural[string](words, true).eval(EvalContext{}); !got.Equal(Present([]string{"E5", "E4", "E2", "E1"})) {
		t.Fatalf("natural descending order = %v", got)
	}

	values := Literal([]int64{1, 2, 3, 4})
	if got := EnumTakeExpr[int64](values, Literal(int64(2))).eval(EvalContext{}); !got.Equal(Present([]int64{1, 2})) {
		t.Fatalf("dynamic take = %v", got)
	}
	if got := EnumTakeLastExpr[int64](values, Literal(int64(2))).eval(EvalContext{}); !got.Equal(Present([]int64{3, 4})) {
		t.Fatalf("dynamic take-last = %v", got)
	}
	if got := EnumTakeExpr[int64](values, Literal(int64(-1))).eval(EvalContext{}); !got.Equal(Present([]int64{})) {
		t.Fatalf("negative dynamic take = %v", got)
	}

	env := NewEnvironment()
	if _, err := RegisterStruct[enumExpressionContainer](env, "EnumDynamicContainer"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("enum-limit", int64(2)); err != nil {
		t.Fatal(err)
	}
	stream := From[enumExpressionContainer](env, "EnumDynamicContainer")
	field := Field[enumExpressionContainer, []int64]("values")
	query := Select(stream, Alias("head", EnumTakeExpr[int64](field, VariableRef[int64]("enum-limit"))))
	plan, err := env.Build(query.Query(StatementName("enum-dynamic")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, received ResultBatch) error {
		batches = append(batches, received)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), enumExpressionContainer{Values: []int64{1, 2, 3}}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SetVariable(context.Background(), "enum-limit", int64(1)); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), enumExpressionContainer{Values: []int64{4, 5, 6}}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 {
		t.Fatalf("dynamic take batch count = %d", len(batches))
	}
	firstRow, ok := batches[0].New[0].Row()
	if !ok {
		t.Fatal("first dynamic take result is not a row")
	}
	first, err := As[[]int64](firstRow.Get("head"))
	if err != nil || !reflect.DeepEqual(first, []int64{1, 2}) {
		t.Fatalf("first dynamic take = %#v, err=%v", first, err)
	}
	secondRow, ok := batches[1].New[0].Row()
	if !ok {
		t.Fatal("updated dynamic take result is not a row")
	}
	second, err := As[[]int64](secondRow.Get("head"))
	if err != nil || !reflect.DeepEqual(second, []int64{4}) {
		t.Fatalf("updated dynamic take = %#v, err=%v", second, err)
	}
}

func TestEnumerableCollectsMapValuesAndIterators(t *testing.T) {
	mapValues := map[string]int64{"a": 4, "b": 5, "c": 6}
	collectedMap := EnumCollect[int64](Literal(maps.Values(mapValues)))
	if got := EnumSum[int64](collectedMap).eval(EvalContext{}); !got.Equal(Present(int64(15))) {
		t.Fatalf("map-backed collection sum = %v", got)
	}
	if got := EnumCount[int64](collectedMap).eval(EvalContext{}); !got.Equal(Present(int64(3))) {
		t.Fatalf("map-backed collection count = %v", got)
	}

	sequence := Literal(iter.Seq[int64](func(yield func(int64) bool) {
		for _, value := range []int64{2, 4, 6} {
			if !yield(value) {
				return
			}
		}
	}))
	collectedSequence := EnumCollect[int64](sequence)
	if got := EnumSelect[int64, int64](collectedSequence, Add[int64](EnumElement[int64](), EnumIndex())).eval(EvalContext{}); !got.Equal(Present([]int64{2, 5, 8})) {
		t.Fatalf("iter.Seq collection = %v", got)
	}

	pullIterator := &enumExpressionIterator{values: []int64{3, 1, 2}}
	collectedIterator := EnumCollect[int64](Literal(pullIterator))
	if got := EnumReverse[int64](collectedIterator).eval(EvalContext{}); !got.Equal(Present([]int64{2, 1, 3})) {
		t.Fatalf("pull iterator collection = %v", got)
	}
}

func TestEnumerableBuildRejectsMissingRequiredExpressions(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumExpressionContainer](env, "EnumInvalidContainer"); err != nil {
		t.Fatal(err)
	}
	stream := From[enumExpressionContainer](env, "EnumInvalidContainer")
	values := Field[enumExpressionContainer, []int64]("values")
	invalid := []struct {
		name string
		expr Expr
	}{
		{name: "where-predicate", expr: EnumWhere[int64](values, nil)},
		{name: "select-selector", expr: EnumSelect[int64, int64](values, nil)},
		{name: "union-right", expr: EnumUnion[int64](values, nil)},
		{name: "min-by-selector", expr: EnumMinBy[int64, int64](values, nil)},
		{name: "to-map-value", expr: EnumToMap[int64, string, int64](values, Literal("key"), nil)},
	}
	for _, testCase := range invalid {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := env.Build(Select(stream, Alias("invalid", testCase.expr)).Query(StatementName("enum-invalid-" + testCase.name))); err == nil {
				t.Fatal("Build accepted an enumeration expression with a missing required expression")
			}
		})
	}
}

func TestEnumerableSetFoldGroupMapAndFrequencyMethods(t *testing.T) {
	left := Literal([]int64{1, 2, 2, 3})
	right := Literal([]int64{2, 3})
	if got := EnumExcept[int64](left, right).eval(EvalContext{}); !got.Equal(Present([]int64{1})) {
		t.Fatalf("except = %v", got)
	}
	if got := EnumIntersect[int64](left, right).eval(EvalContext{}); !got.Equal(Present([]int64{2, 2, 3})) {
		t.Fatalf("intersect = %v", got)
	}
	if got := EnumUnion[int64](left, right).eval(EvalContext{}); !got.Equal(Present([]int64{1, 2, 2, 3, 2, 3})) {
		t.Fatalf("union = %v", got)
	}
	if got := EnumSequenceEqual[int64](left, right).eval(EvalContext{}); !got.Equal(Present(false)) {
		t.Fatalf("sequence-equal = %v", got)
	}
	if got := EnumSequenceEqual[int64](Literal([]int64{1, 2}), Literal([]int64{1, 2})).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("sequence-equal true = %v", got)
	}
	if got := EnumSequenceEqual[int64](NullLiteral[[]int64](), NullLiteral[[]int64]()).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("sequence-equal null = %v", got)
	}

	accumulator := Add[int64](EnumAccumulator[int64](), EnumElement[int64]())
	if got := EnumAggregate[int64, int64](Literal([]int64{1, 2, 3}), int64(0), accumulator).eval(EvalContext{}); !got.Equal(Present(int64(6))) {
		t.Fatalf("aggregate = %v", got)
	}

	words := Literal([]string{"a", "b", "a"})
	groups := EnumGroupBy[string, string](words, EnumElement[string]()).eval(EvalContext{})
	if !groups.Equal(Present(map[string][]string{"a": {"a", "a"}, "b": {"b"}})) {
		t.Fatalf("group-by = %v", groups)
	}
	toMap := EnumToMap[string, string, int64](words, EnumElement[string](), EnumIndex()).eval(EvalContext{})
	if !toMap.Equal(Present(map[string]int64{"a": 2, "b": 1})) {
		t.Fatalf("to-map = %v", toMap)
	}

	frequent := Literal([]string{"a", "b", "a", "c", "b", "a"})
	if got := EnumMostFrequent[string](frequent).eval(EvalContext{}); !got.Equal(Present("a")) {
		t.Fatalf("most-frequent = %v", got)
	}
	if got := EnumLeastFrequent[string](frequent).eval(EvalContext{}); !got.Equal(Present("c")) {
		t.Fatalf("least-frequent = %v", got)
	}
	items := Literal([]enumExpressionItem{{ID: "A", Score: 1}, {ID: "B", Score: 2}, {ID: "C", Score: 1}})
	score := EnumField[enumExpressionItem, int64]("score")
	if got := EnumMostFrequentBy[enumExpressionItem, int64](items, score).eval(EvalContext{}); !got.Equal(Present(enumExpressionItem{ID: "A", Score: 1})) {
		t.Fatalf("most-frequent-by = %v", got)
	}
}

func containsString(value, fragment string) bool {
	return strings.Contains(value, fragment)
}
