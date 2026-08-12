package esper

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type enumGroupByItem struct {
	ID    string `esper:"id"`
	Score int64  `esper:"score"`
}

type enumGroupByContainer struct {
	Items []enumGroupByItem `esper:"items"`
}

func enumGroupByAfterUnderscore(value string) string {
	index := strings.IndexByte(value, '_')
	if index == -1 {
		return ""
	}
	return value[index+1:]
}

func enumGroupByIntString(value int64) string {
	return strconv.FormatInt(value, 10)
}

func enumGroupByIndexedKey(value Expression[string]) Expression[string] {
	return Concat(value, Concat(Literal("_"), Func1[int64, string]("itoa", enumGroupByIntString, EnumIndex())))
}

func enumGroupBySizedKey(value Expression[string]) Expression[string] {
	return Concat(enumGroupByIndexedKey(value), Concat(Literal("_"), Func1[int64, string]("itoa", enumGroupByIntString, EnumSize())))
}

func enumGroupByIndexedValue(value Expression[string]) Expression[string] {
	return Concat(value, Concat(Literal("_"), Func1[int64, string]("itoa", enumGroupByIntString, EnumIndex())))
}

func enumGroupBySizedValue(value Expression[string]) Expression[string] {
	return Concat(enumGroupByIndexedValue(value), Concat(Literal("_"), Func1[int64, string]("itoa", enumGroupByIntString, EnumSize())))
}

func enumGroupByIndexedNumber(value Expression[int64]) Expression[int64] {
	return Add[int64](value, Multiply[int64](Literal(int64(10)), EnumIndex()))
}

func enumGroupBySizedNumber(value Expression[int64]) Expression[int64] {
	return Add[int64](enumGroupByIndexedNumber(value), Multiply[int64](Literal(int64(100)), EnumSize()))
}

// TestExprEnumGroupByOneParamScalarParity covers scalar grouping by element,
// index and size selectors plus Null and empty collection behavior.
func TestExprEnumGroupByOneParamScalarParity(t *testing.T) {
	values := Literal([]string{"E1_2", "E2_1", "E3_2"})
	element := EnumElement[string]()
	key := Func1[string, string]("afterUnderscore", enumGroupByAfterUnderscore, element)
	assertEnumEval(t, EnumGroupBy[string, string](values, key), map[string][]string{
		"2": {"E1_2", "E3_2"},
		"1": {"E2_1"},
	}, "group-by-scalar")
	assertEnumEval(t, EnumGroupBy[string, string](values, enumGroupByIndexedKey(key)), map[string][]string{
		"2_0": {"E1_2"},
		"1_1": {"E2_1"},
		"2_2": {"E3_2"},
	}, "group-by-scalar-index")
	assertEnumEval(t, EnumGroupBy[string, string](values, enumGroupBySizedKey(key)), map[string][]string{
		"2_0_3": {"E1_2"},
		"1_1_3": {"E2_1"},
		"2_2_3": {"E3_2"},
	}, "group-by-scalar-index-size")

	empty := EnumGroupBy[string, string](Literal([]string{}), EnumElement[string]()).eval(EvalContext{})
	if !empty.Equal(Present(map[string][]string{})) {
		t.Fatalf("group-by scalar empty = %v, want empty map", empty)
	}
	null := EnumGroupBy[string, string](NullLiteral[[]string](), EnumElement[string]()).eval(EvalContext{})
	if !null.IsNull() {
		t.Fatalf("group-by scalar null = %v, want null", null)
	}
}

// TestExprEnumGroupByTwoParamScalarParity covers independent key and value
// selectors sharing the same element/index/size context.
func TestExprEnumGroupByTwoParamScalarParity(t *testing.T) {
	values := Literal([]string{"E1_2", "E2_1", "E3_2"})
	element := EnumElement[string]()
	key := Func1[string, string]("afterUnderscore", enumGroupByAfterUnderscore, element)
	assertEnumEval(t, EnumGroupBySelect[string, string, string](values, key, element), map[string][]string{
		"2": {"E1_2", "E3_2"},
		"1": {"E2_1"},
	}, "group-by-select-scalar")
	assertEnumEval(t, EnumGroupBySelect[string, string, string](values, enumGroupByIndexedKey(key), enumGroupByIndexedValue(element)), map[string][]string{
		"2_0": {"E1_2_0"},
		"1_1": {"E2_1_1"},
		"2_2": {"E3_2_2"},
	}, "group-by-select-scalar-index")
	assertEnumEval(t, EnumGroupBySelect[string, string, string](values, enumGroupBySizedKey(key), enumGroupBySizedValue(element)), map[string][]string{
		"2_0_3": {"E1_2_0_3"},
		"1_1_3": {"E2_1_1_3"},
		"2_2_3": {"E3_2_2_3"},
	}, "group-by-select-scalar-index-size")

	empty := EnumGroupBySelect[string, string, string](Literal([]string{}), EnumElement[string](), EnumElement[string]()).eval(EvalContext{})
	if !empty.Equal(Present(map[string][]string{})) {
		t.Fatalf("group-by-select scalar empty = %v, want empty map", empty)
	}
	null := EnumGroupBySelect[string, string, string](NullLiteral[[]string](), EnumElement[string](), EnumElement[string]()).eval(EvalContext{})
	if !null.IsNull() {
		t.Fatalf("group-by-select scalar null = %v, want null", null)
	}
}

// TestExprEnumGroupByEventParity covers one- and two-selector event grouping
// through Build/Deploy, including typed map metadata and bucket input order.
func TestExprEnumGroupByEventParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumGroupByContainer](env, "GroupByContainer"); err != nil {
		t.Fatal(err)
	}

	items := Field[enumGroupByContainer, []enumGroupByItem]("items")
	id := EnumField[enumGroupByItem, string]("id")
	score := EnumField[enumGroupByItem, int64]("score")
	plan, err := env.Build(Select(From[enumGroupByContainer](env, "GroupByContainer"),
		Alias("one", EnumGroupBy[enumGroupByItem, string](items, id)),
		Alias("one_index", EnumGroupBy[enumGroupByItem, string](items, enumGroupByIndexedKey(id))),
		Alias("one_size", EnumGroupBy[enumGroupByItem, string](items, enumGroupBySizedKey(id))),
		Alias("two", EnumGroupBySelect[enumGroupByItem, string, int64](items, id, score)),
		Alias("two_index", EnumGroupBySelect[enumGroupByItem, string, int64](items, enumGroupByIndexedKey(id), enumGroupByIndexedNumber(score))),
		Alias("two_size", EnumGroupBySelect[enumGroupByItem, string, int64](items, enumGroupBySizedKey(id), enumGroupBySizedNumber(score))),
	).Query(StatementName("enum-group-by-event")))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("group-by result schema is missing")
	}
	for _, name := range []string{"one", "one_index", "one_size"} {
		field, exists := resultSchema.Field(name)
		if !exists || field.Type != reflect.TypeOf(map[string][]enumGroupByItem{}) {
			t.Fatalf("group-by result field %q = %#v, want map[string][]enumGroupByItem", name, field)
		}
	}
	for _, name := range []string{"two", "two_index", "two_size"} {
		field, exists := resultSchema.Field(name)
		if !exists || field.Type != reflect.TypeOf(map[string][]int64{}) {
			t.Fatalf("group-by result field %q = %#v, want map[string][]int64", name, field)
		}
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectDotRows(t, deployment)
	input := []enumGroupByItem{{ID: "E1", Score: 1}, {ID: "E1", Score: 2}, {ID: "E2", Score: 5}}
	if err := engine.SendEvent(context.Background(), enumGroupByContainer{Items: input}); err != nil {
		t.Fatal(err)
	}
	row := (*rows)[0]
	if !row.Get("one").Equal(Present(map[string][]enumGroupByItem{
		"E1": {input[0], input[1]},
		"E2": {input[2]},
	})) || !row.Get("one_index").Equal(Present(map[string][]enumGroupByItem{
		"E1_0": {input[0]}, "E1_1": {input[1]}, "E2_2": {input[2]},
	})) || !row.Get("one_size").Equal(Present(map[string][]enumGroupByItem{
		"E1_0_3": {input[0]}, "E1_1_3": {input[1]}, "E2_2_3": {input[2]},
	})) {
		t.Fatalf("group-by event one-selector row = %#v", row.AsMap())
	}
	if !row.Get("two").Equal(Present(map[string][]int64{"E1": {1, 2}, "E2": {5}})) ||
		!row.Get("two_index").Equal(Present(map[string][]int64{"E1_0": {1}, "E1_1": {12}, "E2_2": {25}})) ||
		!row.Get("two_size").Equal(Present(map[string][]int64{"E1_0_3": {301}, "E1_1_3": {312}, "E2_2_3": {325}})) {
		t.Fatalf("group-by event two-selector row = %#v", row.AsMap())
	}

	if err := engine.SendEvent(context.Background(), enumGroupByContainer{Items: []enumGroupByItem{}}); err != nil {
		t.Fatal(err)
	}
	empty := (*rows)[1]
	for _, name := range []string{"one", "one_index", "one_size"} {
		if !empty.Get(name).Equal(Present(map[string][]enumGroupByItem{})) {
			t.Fatalf("group-by empty %s = %v", name, empty.Get(name))
		}
	}
	for _, name := range []string{"two", "two_index", "two_size"} {
		if !empty.Get(name).Equal(Present(map[string][]int64{})) {
			t.Fatalf("group-by-select empty %s = %v", name, empty.Get(name))
		}
	}
}

// TestExprEnumGroupByNullSelectorsParity verifies Java-compatible null keys
// and null selected values with pointer/interface output types.
func TestExprEnumGroupByNullSelectorsParity(t *testing.T) {
	items := Literal([]enumGroupByItem{{ID: "E1", Score: 1}, {ID: "E1", Score: 2}, {ID: "E2", Score: 5}})
	allNull := EnumGroupBy[enumGroupByItem, any](items, NullLiteral[any]()).eval(EvalContext{})
	if !allNull.Equal(Present(map[any][]enumGroupByItem{nil: {
		{ID: "E1", Score: 1}, {ID: "E1", Score: 2}, {ID: "E2", Score: 5},
	}})) {
		t.Fatalf("group-by null key = %v", allNull)
	}

	nullValues := EnumGroupBySelect[enumGroupByItem, string, *int64](
		items,
		EnumField[enumGroupByItem, string]("id"),
		NullLiteral[*int64](),
	).eval(EvalContext{})
	want := map[string][]*int64{"E1": {nil, nil}, "E2": {nil}}
	if !nullValues.Equal(Present(want)) {
		t.Fatalf("group-by null selected values = %v, want %v", nullValues, want)
	}
}

// TestExprEnumGroupByInvalidParity covers missing collection/key/value
// selectors. Java mismatched lambda arities are structurally impossible in
// the fluent API because both selectors share EnumElement/Index/Size nodes.
func TestExprEnumGroupByInvalidParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "GroupByInvalid", nil); err != nil {
		t.Fatal(err)
	}
	stream := From[map[string]any](env, "GroupByInvalid")
	values := Literal([]string{"E1"})
	for _, testCase := range []struct {
		name string
		expr Expr
	}{
		{name: "missing-values", expr: EnumGroupBy[string, string](nil, EnumElement[string]())},
		{name: "missing-key", expr: EnumGroupBy[string, string](values, nil)},
		{name: "missing-selected-key", expr: EnumGroupBySelect[string, string, string](values, nil, EnumElement[string]())},
		{name: "missing-selected-value", expr: EnumGroupBySelect[string, string, string](values, EnumElement[string](), nil)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(stream, Alias("result", testCase.expr)).Query(StatementName("group-by-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), `enumeration method "group-by`) {
				t.Fatalf("group-by invalid error = %v", err)
			}
		})
	}
}
