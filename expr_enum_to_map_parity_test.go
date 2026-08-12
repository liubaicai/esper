package esper

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type enumToMapItem struct {
	ID    string `esper:"id"`
	Score int64  `esper:"score"`
}

type enumToMapContainer struct {
	Items []enumToMapItem `esper:"items"`
}

type enumToMapNullableItem struct {
	ID    *string `esper:"id"`
	Score *int64  `esper:"score"`
}

func enumToMapExtractNum(value string) int64 {
	parsed, _ := strconv.ParseInt(strings.TrimPrefix(value, "E"), 10, 64)
	return parsed
}

func enumToMapIntString(value int64) string {
	return strconv.FormatInt(value, 10)
}

func enumToMapIndexedKey(value Expression[string]) Expression[string] {
	return Concat(value, Concat(Literal("_"), Func1[int64, string]("itoa", enumToMapIntString, EnumIndex())))
}

func enumToMapSizedKey(value Expression[string]) Expression[string] {
	return Concat(enumToMapIndexedKey(value), Concat(Literal("_"), Func1[int64, string]("itoa", enumToMapIntString, EnumSize())))
}

func enumToMapIndexedValue(value Expression[int64]) Expression[int64] {
	return Add[int64](value, Multiply[int64](Literal(int64(10)), EnumIndex()))
}

func enumToMapSizedValue(value Expression[int64]) Expression[int64] {
	return Add[int64](enumToMapIndexedValue(value), Multiply[int64](Literal(int64(100)), EnumSize()))
}

// TestExprEnumToMapScalarParity covers scalar key/value selectors with
// element, index and size context, plus empty and Null input semantics.
func TestExprEnumToMapScalarParity(t *testing.T) {
	values := Literal([]string{"E2", "E1", "E3"})
	element := EnumElement[string]()
	extracted := Func1[string, int64]("extractNum", enumToMapExtractNum, element)

	assertEnumEval(t, EnumToMap[string, string, int64](values, element, extracted), map[string]int64{
		"E1": 1,
		"E2": 2,
		"E3": 3,
	}, "to-map-scalar")
	assertEnumEval(t, EnumToMap[string, string, int64](values, enumToMapIndexedKey(element), enumToMapIndexedValue(extracted)), map[string]int64{
		"E1_1": 11,
		"E2_0": 2,
		"E3_2": 23,
	}, "to-map-scalar-index")
	assertEnumEval(t, EnumToMap[string, string, int64](values, enumToMapSizedKey(element), enumToMapSizedValue(extracted)), map[string]int64{
		"E1_1_3": 311,
		"E2_0_3": 302,
		"E3_2_3": 323,
	}, "to-map-scalar-index-size")

	single := Literal([]string{"E1"})
	singleElement := EnumElement[string]()
	singleExtracted := Func1[string, int64]("extractNum", enumToMapExtractNum, singleElement)
	assertEnumEval(t, EnumToMap[string, string, int64](single, enumToMapSizedKey(singleElement), enumToMapSizedValue(singleExtracted)), map[string]int64{
		"E1_0_1": 101,
	}, "to-map-scalar-single")

	empty := EnumToMap[string, string, int64](Literal([]string{}), EnumElement[string](), Literal(int64(1))).eval(EvalContext{})
	if !empty.Equal(Present(map[string]int64{})) {
		t.Fatalf("to-map empty = %v, want empty map", empty)
	}
	null := EnumToMap[string, string, int64](NullLiteral[[]string](), EnumElement[string](), Literal(int64(1))).eval(EvalContext{})
	if !null.IsNull() {
		t.Fatalf("to-map null = %v, want null", null)
	}
}

// TestExprEnumToMapEventParity covers event selectors and duplicate-key
// replacement through the normal Build, Deploy and listener path.
func TestExprEnumToMapEventParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumToMapContainer](env, "ToMapContainer"); err != nil {
		t.Fatal(err)
	}

	items := Field[enumToMapContainer, []enumToMapItem]("items")
	id := EnumField[enumToMapItem, string]("id")
	score := EnumField[enumToMapItem, int64]("score")
	plan, err := env.Build(Select(From[enumToMapContainer](env, "ToMapContainer"),
		Alias("c0", EnumToMap[enumToMapItem, string, int64](items, id, score)),
		Alias("c1", EnumToMap[enumToMapItem, string, int64](items, enumToMapIndexedKey(id), enumToMapIndexedValue(score))),
		Alias("c2", EnumToMap[enumToMapItem, string, int64](items, enumToMapSizedKey(id), enumToMapSizedValue(score))),
	).Query(StatementName("to-map-event")))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("to-map result schema is missing")
	}
	for _, name := range []string{"c0", "c1", "c2"} {
		field, exists := resultSchema.Field(name)
		if !exists || field.Type != reflect.TypeOf(map[string]int64{}) {
			t.Fatalf("to-map result field %q = %#v, want map[string]int64", name, field)
		}
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectDotRows(t, deployment)

	if err := engine.SendEvent(context.Background(), enumToMapContainer{Items: []enumToMapItem{
		{ID: "E1", Score: 1},
		{ID: "E3", Score: 12},
		{ID: "E2", Score: 5},
	}}); err != nil {
		t.Fatal(err)
	}
	first := (*rows)[0]
	if !first.Get("c0").Equal(Present(map[string]int64{"E1": 1, "E3": 12, "E2": 5})) ||
		!first.Get("c1").Equal(Present(map[string]int64{"E1_0": 1, "E3_1": 22, "E2_2": 25})) ||
		!first.Get("c2").Equal(Present(map[string]int64{"E1_0_3": 301, "E3_1_3": 322, "E2_2_3": 325})) {
		t.Fatalf("to-map event row 0 = %#v", first.AsMap())
	}

	if err := engine.SendEvent(context.Background(), enumToMapContainer{Items: []enumToMapItem{
		{ID: "E1", Score: 1},
		{ID: "E3", Score: 4},
		{ID: "E2", Score: 7},
		{ID: "E1", Score: 2},
	}}); err != nil {
		t.Fatal(err)
	}
	second := (*rows)[1]
	if !second.Get("c0").Equal(Present(map[string]int64{"E1": 2, "E3": 4, "E2": 7})) {
		t.Fatalf("to-map duplicate key = %#v, want last E1 value 2", second.Get("c0"))
	}
	if !second.Get("c1").Equal(Present(map[string]int64{"E1_0": 1, "E3_1": 14, "E2_2": 27, "E1_3": 32})) ||
		!second.Get("c2").Equal(Present(map[string]int64{"E1_0_4": 401, "E3_1_4": 414, "E2_2_4": 427, "E1_3_4": 432})) {
		t.Fatalf("to-map event row 1 = %#v", second.AsMap())
	}
}

// TestExprEnumToMapNullKeyValueParity verifies Java HashMap-compatible Null
// keys and values using pointer result types so Null remains distinguishable.
func TestExprEnumToMapNullKeyValueParity(t *testing.T) {
	items := Literal([]enumToMapNullableItem{{ID: nil, Score: nil}})
	result := EnumToMap[enumToMapNullableItem, *string, *int64](
		items,
		EnumField[enumToMapNullableItem, *string]("id"),
		EnumField[enumToMapNullableItem, *int64]("score"),
	).eval(EvalContext{})
	if !result.Equal(Present(map[*string]*int64{nil: nil})) {
		t.Fatalf("to-map null key/value = %v, want {nil:nil}", result)
	}
}

// TestExprEnumToMapInvalidParity covers missing selector diagnostics in the
// typed builder. Go selector expressions share one explicit element context,
// so Java's lambda-arity mismatch has no corresponding ambiguous syntax.
func TestExprEnumToMapInvalidParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "ToMapInvalid", nil); err != nil {
		t.Fatal(err)
	}
	stream := From[map[string]any](env, "ToMapInvalid")
	values := Literal([]string{"E1"})
	for _, testCase := range []struct {
		name string
		expr Expr
	}{
		{name: "missing-values", expr: EnumToMap[string, string, int64](nil, EnumElement[string](), Literal(int64(1)))},
		{name: "missing-key", expr: EnumToMap[string, string, int64](values, nil, Literal(int64(1)))},
		{name: "missing-value", expr: EnumToMap[string, string, int64](values, EnumElement[string](), nil)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(stream, Alias("result", testCase.expr)).Query(StatementName("to-map-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), `enumeration method "to-map" requires`) {
				t.Fatalf("to-map invalid error = %v", err)
			}
		})
	}
}
