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

func TestEnumerableMetadataMirrorsJavaFootprints(t *testing.T) {
	items := Literal([]enumExpressionItem{{ID: "E1", Score: 1}})
	where := EnumWhere[enumExpressionItem](items, Equal[int64](EnumIndex(), Literal(int64(0))))
	metadata, ok := EnumerationMetadata(where)
	if !ok {
		t.Fatal("where metadata was not attached")
	}
	if metadata.Method != "where" || metadata.CollectionType != reflect.TypeOf([]enumExpressionItem{}) || metadata.ElementType != reflect.TypeOf(enumExpressionItem{}) || metadata.ResultType != reflect.TypeOf([]enumExpressionItem{}) {
		t.Fatalf("where metadata = %#v", metadata)
	}
	if len(metadata.Footprints) != 3 {
		t.Fatalf("where footprints = %#v", metadata.Footprints)
	}
	for index, want := range []int{1, 2, 3} {
		footprint := metadata.Footprints[index]
		if footprint.Input != EnumInputAny || len(footprint.Parameters) != 1 || footprint.Parameters[0].LambdaParameterCount != want || footprint.Parameters[0].Expected != EnumParameterBoolean {
			t.Fatalf("where footprint %d = %#v", index, footprint)
		}
	}

	sumMetadata, ok := EnumerationMetadata(EnumSum[int64](Literal([]int64{1, 2})))
	if !ok || len(sumMetadata.Footprints) != 4 || sumMetadata.Footprints[0].Input != EnumInputScalarNumeric || sumMetadata.Footprints[0].Parameters != nil || sumMetadata.Footprints[1].Input != EnumInputAny || sumMetadata.Footprints[1].Parameters[0].Expected != EnumParameterNumeric {
		t.Fatalf("sum metadata = %#v", sumMetadata)
	}

	aggregateMetadata, ok := EnumerationMetadata(EnumAggregate[int64, int64](Literal([]int64{1}), int64(0), Add[int64](EnumAccumulator[int64](), EnumElement[int64]())))
	if !ok || len(aggregateMetadata.Footprints) != 3 {
		t.Fatalf("aggregate metadata = %#v", aggregateMetadata)
	}
	for index, want := range []int{2, 3, 4} {
		parameters := aggregateMetadata.Footprints[index].Parameters
		if len(parameters) != 2 || parameters[0].LambdaParameterCount != 0 || parameters[1].LambdaParameterCount != want {
			t.Fatalf("aggregate footprint %d = %#v", index, parameters)
		}
	}

	groupMetadata, ok := EnumerationMetadata(EnumGroupBySelect[enumExpressionItem, string, int64](items, EnumField[enumExpressionItem, string]("id"), EnumField[enumExpressionItem, int64]("score")))
	if !ok || len(groupMetadata.Footprints) != 3 {
		t.Fatalf("group-by metadata = %#v", groupMetadata)
	}
	for index, want := range []int{1, 2, 3} {
		parameters := groupMetadata.Footprints[index].Parameters
		if len(parameters) != 2 || parameters[0].LambdaParameterCount != want || parameters[1].LambdaParameterCount != want {
			t.Fatalf("group-by footprint %d = %#v", index, parameters)
		}
	}

	collectMetadata, ok := EnumerationMetadata(EnumCollect[int64](Literal([]int64{1})))
	if !ok || collectMetadata.Method != "collect" || len(collectMetadata.Footprints) != 1 || len(collectMetadata.Footprints[0].Parameters) != 0 {
		t.Fatalf("collect metadata = %#v", collectMetadata)
	}

	metadata.Footprints[0].Parameters[0].Description = "mutated"
	again, ok := EnumerationMetadata(where)
	if !ok || again.Footprints[0].Parameters[0].Description == "mutated" {
		t.Fatal("enumeration metadata exposes mutable footprint storage")
	}
}

func TestEnumerableMetadataCoversAllGoEnumerationConstructors(t *testing.T) {
	items := Literal([]enumExpressionItem{{ID: "E1", Score: 1}, {ID: "E2", Score: 2}})
	numbers := Literal([]int64{1, 2, 3})
	id := EnumField[enumExpressionItem, string]("id")
	score := EnumField[enumExpressionItem, int64]("score")
	predicate := GreaterOrEqual[int64](score, Literal(int64(1)))

	type footprintExpectation struct {
		name       string
		expression Expr
		method     string
		input      EnumInputKind
		arities    []int
		parameters int
	}
	expectations := []footprintExpectation{
		{"collect", EnumCollect[enumExpressionItem](items), "collect", EnumInputAny, []int{0}, 0},
		{"where", EnumWhere[enumExpressionItem](items, predicate), "where", EnumInputAny, []int{1, 2, 3}, 1},
		{"select", EnumSelect[enumExpressionItem, int64](items, score), "select", EnumInputAny, []int{1, 2, 3}, 1},
		{"select-from", EnumSelectMap[enumExpressionItem](items, Alias("id", id)), "select-from", EnumInputAny, []int{1, 2, 3}, 1},
		{"array-of", EnumArrayOf[enumExpressionItem](items), "array-of", EnumInputScalarAny, []int{0, 1, 2, 3}, -1},
		{"array-of-select", EnumArrayOfSelect[enumExpressionItem, int64](items, score), "array-of", EnumInputScalarAny, []int{0, 1, 2, 3}, -1},
		{"count", EnumCount[enumExpressionItem](items), "count", EnumInputAny, []int{0}, 0},
		{"count-of", EnumCountOf[enumExpressionItem](items, predicate), "count-of", EnumInputAny, []int{0, 1, 2, 3}, -1},
		{"any-of", EnumAnyOf[enumExpressionItem](items, predicate), "any-of", EnumInputAny, []int{1, 2, 3}, 1},
		{"all-of", EnumAllOf[enumExpressionItem](items, predicate), "all-of", EnumInputAny, []int{1, 2, 3}, 1},
		{"first-of", EnumFirstOf[enumExpressionItem](items, predicate), "first-of", EnumInputAny, []int{0, 1, 2, 3}, -1},
		{"last-of", EnumLastOf[enumExpressionItem](items, predicate), "last-of", EnumInputAny, []int{0, 1, 2, 3}, -1},
		{"distinct", EnumDistinct[enumExpressionItem](items), "distinct", EnumInputScalarAny, []int{0, 1, 2, 3}, -1},
		{"distinct-by", EnumDistinctBy[enumExpressionItem, string](items, id), "distinct", EnumInputScalarAny, []int{0, 1, 2, 3}, -1},
		{"take", EnumTake[enumExpressionItem](items, 1), "take", EnumInputAny, []int{0}, 1},
		{"take-expr", EnumTakeExpr[enumExpressionItem](items, Literal(int64(1))), "take", EnumInputAny, []int{0}, 1},
		{"take-last", EnumTakeLast[enumExpressionItem](items, 1), "take-last", EnumInputAny, []int{0}, 1},
		{"take-while", EnumTakeWhile[enumExpressionItem](items, predicate), "take-while", EnumInputAny, []int{1, 2, 3}, 1},
		{"take-while-last", EnumTakeWhileLast[enumExpressionItem](items, predicate), "take-while-last", EnumInputAny, []int{1, 2, 3}, 1},
		{"reverse", EnumReverse[enumExpressionItem](items), "reverse", EnumInputAny, []int{0}, 0},
		{"min", EnumMin[int64](numbers), "min", EnumInputScalarAny, []int{0, 1, 2, 3}, -1},
		{"max", EnumMax[int64](numbers), "max", EnumInputScalarAny, []int{0, 1, 2, 3}, -1},
		{"min-by", EnumMinBy[enumExpressionItem, int64](items, score), "min-by", EnumInputAny, []int{1, 2, 3}, 1},
		{"max-by", EnumMaxBy[enumExpressionItem, int64](items, score), "max-by", EnumInputAny, []int{1, 2, 3}, 1},
		{"min-of", EnumMinOf[enumExpressionItem, int64](items, score), "min", EnumInputScalarAny, []int{0, 1, 2, 3}, -1},
		{"max-of", EnumMaxOf[enumExpressionItem, int64](items, score), "max", EnumInputScalarAny, []int{0, 1, 2, 3}, -1},
		{"order-by", EnumOrderBy[enumExpressionItem, int64](items, score, false), "order-by", EnumInputScalarAny, []int{0, 1, 2, 3}, -1},
		{"order-by-natural", EnumOrderByNatural[int64](numbers, false), "order-by", EnumInputScalarAny, []int{0, 1, 2, 3}, -1},
		{"order-by-desc", EnumOrderBy[enumExpressionItem, int64](items, score, true), "order-by-desc", EnumInputScalarAny, []int{0, 1, 2, 3}, -1},
		{"sum", EnumSum[int64](numbers), "sum", EnumInputScalarNumeric, []int{0, 1, 2, 3}, -1},
		{"sum-of", EnumSumOf[enumExpressionItem, int64](items, score), "sum", EnumInputScalarNumeric, []int{0, 1, 2, 3}, -1},
		{"average", EnumAverage[int64](numbers), "average", EnumInputScalarNumeric, []int{0, 1, 2, 3}, -1},
		{"average-of", EnumAverageOf[enumExpressionItem, int64](items, score), "average", EnumInputScalarNumeric, []int{0, 1, 2, 3}, -1},
		{"average-exact", EnumAverageExact[int64](numbers), "average-exact", EnumInputScalarNumeric, []int{0, 1, 2, 3}, -1},
		{"aggregate", EnumAggregate[int64, int64](numbers, int64(0), Add[int64](EnumAccumulator[int64](), EnumElement[int64]())), "aggregate", EnumInputAny, []int{2, 3, 4}, 2},
		{"except", EnumExcept[int64](numbers, Literal([]int64{2})), "except", EnumInputAny, []int{0}, 1},
		{"intersect", EnumIntersect[int64](numbers, Literal([]int64{2})), "intersect", EnumInputAny, []int{0}, 1},
		{"union", EnumUnion[int64](numbers, Literal([]int64{2})), "union", EnumInputAny, []int{0}, 1},
		{"sequence-equal", EnumSequenceEqual[int64](numbers, Literal([]int64{1})), "sequence-equal", EnumInputScalarAny, []int{0}, 1},
		{"group-by", EnumGroupBy[enumExpressionItem, string](items, id), "group-by", EnumInputAny, []int{1, 2, 3, 1, 2, 3}, -1},
		{"group-by-select", EnumGroupBySelect[enumExpressionItem, string, int64](items, id, score), "group-by-select", EnumInputAny, []int{1, 2, 3}, 2},
		{"to-map", EnumToMap[enumExpressionItem, string, int64](items, id, score), "to-map", EnumInputAny, []int{1, 2, 3}, 2},
		{"most-frequent", EnumMostFrequent[int64](numbers), "most-frequent", EnumInputScalarAny, []int{0, 1, 2, 3}, -1},
		{"most-frequent-by", EnumMostFrequentBy[enumExpressionItem, string](items, id), "most-frequent", EnumInputScalarAny, []int{0, 1, 2, 3}, -1},
		{"least-frequent", EnumLeastFrequent[int64](numbers), "least-frequent", EnumInputScalarAny, []int{0, 1, 2, 3}, -1},
		{"least-frequent-by", EnumLeastFrequentBy[enumExpressionItem, string](items, id), "least-frequent", EnumInputScalarAny, []int{0, 1, 2, 3}, -1},
	}

	for _, expectation := range expectations {
		metadata, ok := EnumerationMetadata(expectation.expression)
		if !ok {
			t.Errorf("%s: metadata missing", expectation.name)
			continue
		}
		if metadata.Method != expectation.method {
			t.Errorf("%s: method = %q, want %q", expectation.name, metadata.Method, expectation.method)
		}
		if len(metadata.Footprints) != len(expectation.arities) {
			t.Errorf("%s: footprint count = %d, want %d (%#v)", expectation.name, len(metadata.Footprints), len(expectation.arities), metadata.Footprints)
			continue
		}
		for index, arity := range expectation.arities {
			footprint := metadata.Footprints[index]
			wantInput := expectation.input
			if index > 0 && (wantInput == EnumInputScalarAny || wantInput == EnumInputScalarNumeric) {
				wantInput = EnumInputAny
			}
			if footprint.Input != wantInput {
				t.Errorf("%s footprint %d: input = %s, want %s", expectation.name, index, footprint.Input, wantInput)
			}
			if expectation.parameters >= 0 && len(footprint.Parameters) != expectation.parameters {
				t.Errorf("%s footprint %d: parameter count = %d, want %d", expectation.name, index, len(footprint.Parameters), expectation.parameters)
			}
			if len(footprint.Parameters) > 0 {
				lambdaParameter := footprint.Parameters[0]
				if len(footprint.Parameters) > 1 {
					lambdaParameter = footprint.Parameters[len(footprint.Parameters)-1]
				}
				if lambdaParameter.LambdaParameterCount != arity {
					t.Errorf("%s footprint %d: lambda arity = %d, want %d", expectation.name, index, lambdaParameter.LambdaParameterCount, arity)
				}
			}
		}
	}

	if _, ok := EnumerationMetadata(Literal(int64(1))); ok {
		t.Fatal("ordinary expressions must not expose enumeration metadata")
	}
	sequence := Func0[iter.Seq[int64]]("values", func() iter.Seq[int64] {
		return func(yield func(int64) bool) {
			yield(1)
		}
	})
	sequenceMetadata, ok := EnumerationMetadata(EnumCollect[int64](sequence))
	if !ok || sequenceMetadata.CollectionType != reflect.TypeOf([]int64{}) || sequenceMetadata.ElementType != reflect.TypeOf(int64(0)) || sequenceMetadata.ResultType != reflect.TypeOf([]int64{}) {
		t.Fatalf("iterator collection metadata = source=%v method=%s collection=%v (want normalized %v) element=%v (want %v) result=%v (want %v)", sequence.Type(), sequenceMetadata.Method, sequenceMetadata.CollectionType, reflect.TypeOf([]int64{}), sequenceMetadata.ElementType, reflect.TypeOf(int64(0)), sequenceMetadata.ResultType, reflect.TypeOf([]int64{}))
	}
	if EnumInputAny.String() != "any" || EnumInputScalarAny.String() != "scalar-any" || EnumInputScalarNumeric.String() != "scalar-numeric" || EnumInputEventCollection.String() != "event-collection" || EnumInputKind(255).String() != "unknown" {
		t.Fatal("unexpected enumeration input-kind strings")
	}
	if EnumParameterAny.String() != "any" || EnumParameterBoolean.String() != "boolean" || EnumParameterNumeric.String() != "numeric" || EnumParameterCollection.String() != "collection" || EnumParameterKind(255).String() != "unknown" {
		t.Fatal("unexpected enumeration parameter-kind strings")
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

func TestEnumerableSubqueryAndZeroArgumentUDFFSources(t *testing.T) {
	env := NewEnvironment()
	itemSchema, err := RegisterStruct[enumExpressionItem](env, "EnumSourceItem")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[enumExpressionContainer](env, "EnumSourceTrigger"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "EnumSourceWindow", itemSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	window := FromNamedWindow(env, "EnumSourceWindow")
	filteredEvents := SubqueryEvents(window, SubqueryWhere(
		Greater[int64](Field[any, int64]("score"), Literal(int64(1))),
	))
	scores := SubqueryValues[int64](window, Field[any, int64]("score"))
	udfValues := Func0[[]int64]("enum-values", func() []int64 { return []int64{4, 5, 6} })
	udfValuesWithLimit := Func1[int64, []int64]("enum-values-limit", func(limit int64) []int64 {
		values := []int64{4, 5, 6}
		if limit < 0 {
			return nil
		}
		if limit > int64(len(values)) {
			limit = int64(len(values))
		}
		return values[:limit]
	}, Literal(int64(2)))
	udfValuesWithRange := Func2[int64, int64, []int64]("enum-values-range", func(start, count int64) []int64 {
		result := make([]int64, 0, count)
		for offset := int64(0); offset < count; offset++ {
			result = append(result, start+offset)
		}
		return result
	}, Literal(int64(7)), Literal(int64(2)))
	query := Select(
		From[enumExpressionContainer](env, "EnumSourceTrigger"),
		Alias("ids", EnumSelect[Event, string](filteredEvents, EnumField[Event, string]("id"))),
		Alias("score-sum", EnumSum[int64](scores)),
		Alias("udf-sum", EnumSum[int64](udfValues)),
		Alias("udf-arg-sum", EnumSum[int64](udfValuesWithLimit)),
		Alias("udf-two-arg-sum", EnumSum[int64](udfValuesWithRange)),
	).Query(StatementName("enum-subquery-udf"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("subquery/UDF result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "EnumSourceWindow", enumExpressionItem{ID: "E1", Score: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "EnumSourceWindow", enumExpressionItem{ID: "E2", Score: 3}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), enumExpressionContainer{}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("subquery/UDF row count = %d", len(rows))
	}
	ids, err := As[[]string](rows[0].Get("ids"))
	if err != nil || !reflect.DeepEqual(ids, []string{"E2"}) {
		t.Fatalf("subquery event ids = %#v, err=%v", ids, err)
	}
	scoreSum, err := As[int64](rows[0].Get("score-sum"))
	if err != nil || scoreSum != 4 {
		t.Fatalf("subquery projected sum = %d, err=%v", scoreSum, err)
	}
	udfSum, err := As[int64](rows[0].Get("udf-sum"))
	if err != nil || udfSum != 15 {
		t.Fatalf("zero-argument UDF sum = %d, err=%v", udfSum, err)
	}
	udfArgSum, err := As[int64](rows[0].Get("udf-arg-sum"))
	if err != nil || udfArgSum != 9 {
		t.Fatalf("one-argument UDF sum = %d, err=%v", udfArgSum, err)
	}
	udfTwoArgSum, err := As[int64](rows[0].Get("udf-two-arg-sum"))
	if err != nil || udfTwoArgSum != 15 {
		t.Fatalf("two-argument UDF sum = %d, err=%v", udfTwoArgSum, err)
	}
}

func TestUDFArityThreeAndFourRemainChainableAndSafe(t *testing.T) {
	values := Func3[int64, int64, int64, []int64]("range3", func(start, count, step int64) []int64 {
		result := make([]int64, 0, count)
		for index := int64(0); index < count; index++ {
			result = append(result, start+index*step)
		}
		return result
	}, Literal(int64(2)), Literal(int64(3)), Literal(int64(4)))
	if got := EnumSum[int64](values).eval(EvalContext{}); !got.Equal(Present(int64(18))) {
		t.Fatalf("three-argument collection UDF = %v", got)
	}

	total := Func4[int64, int64, int64, int64, int64]("sum4", func(one, two, three, four int64) int64 {
		return one + two + three + four
	}, Literal(int64(1)), Literal(int64(2)), Literal(int64(3)), Literal(int64(4)))
	if got := total.eval(EvalContext{}); !got.Equal(Present(int64(10))) {
		t.Fatalf("four-argument UDF = %v", got)
	}

	panicking := Func0[int64]("panic-udf", func() int64 { panic("udf failure") })
	if got := panicking.eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("panicking UDF = %v, want null", got)
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
		want string
	}{
		{name: "missing-values", expr: EnumCount[int64](nil), want: `enumeration method "count" requires a collection expression`},
		{name: "where-predicate", expr: EnumWhere[int64](values, nil), want: `enumeration method "where" requires all selector expressions`},
		{name: "select-selector", expr: EnumSelect[int64, int64](values, nil), want: `enumeration method "select" requires all selector expressions`},
		{name: "union-right", expr: EnumUnion[int64](values, nil), want: `enumeration method "union" requires all selector expressions`},
		{name: "min-by-selector", expr: EnumMinBy[int64, int64](values, nil), want: `enumeration method "min-by" requires all selector expressions`},
		{name: "to-map-value", expr: EnumToMap[int64, string, int64](values, Literal("key"), nil), want: `enumeration method "to-map" requires all selector expressions`},
		{name: "group-by-value-selector", expr: EnumGroupBySelect[int64, int64, int64](values, Literal(int64(1)), nil), want: `enumeration method "group-by-select" requires all selector expressions`},
		{name: "first-of-too-many-predicates", expr: EnumFirstOf[int64](values, Literal(true), Literal(false)), want: `enumeration method "first-of" accepts at most one predicate expression`},
		{name: "last-of-too-many-predicates", expr: EnumLastOf[int64](values, Literal(true), Literal(false)), want: `enumeration method "last-of" accepts at most one predicate expression`},
		{name: "collect-values", expr: EnumCollect[int64](nil), want: `enumeration method "collect" requires a collection expression`},
		{name: "udf-name", expr: Func0[int64]("", func() int64 { return 1 }), want: "udf function name is required"},
		{name: "udf-function", expr: Func3[int64, int64, int64, []int64]("missing-udf", nil, Literal(int64(1)), Literal(int64(2)), Literal(int64(3))), want: `udf "missing-udf" requires a function`},
		{name: "udf-argument", expr: Func3[int64, int64, int64, []int64]("missing-argument", func(int64, int64, int64) []int64 { return nil }, Literal(int64(1)), nil, Literal(int64(3))), want: `udf "missing-argument" argument 1 is required`},
	}
	for _, testCase := range invalid {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := env.Build(Select(stream, Alias("invalid", testCase.expr)).Query(StatementName("enum-invalid-" + testCase.name))); err == nil {
				t.Fatal("Build accepted an enumeration expression with a missing required expression")
			} else if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("Build error = %v, want fragment %q", err, testCase.want)
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
	itemsForGroups := Literal([]enumExpressionItem{
		{ID: "A", Score: 1},
		{ID: "B", Score: 2},
		{ID: "A", Score: 3},
	})
	groupedValues := EnumGroupBySelect[enumExpressionItem, string, int64](
		itemsForGroups,
		EnumField[enumExpressionItem, string]("id"),
		EnumField[enumExpressionItem, int64]("score"),
	).eval(EvalContext{})
	if !groupedValues.Equal(Present(map[string][]int64{"A": {1, 3}, "B": {2}})) {
		t.Fatalf("group-by selector = %v", groupedValues)
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
	if got := EnumMostFrequentBy[enumExpressionItem, int64](items, score).eval(EvalContext{}); !got.Equal(Present(int64(1))) {
		t.Fatalf("most-frequent-by = %v, want 1", got)
	}
}

func containsString(value, fragment string) bool {
	return strings.Contains(value, fragment)
}
