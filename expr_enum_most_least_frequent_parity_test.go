package esper

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type enumMostLeastFreqItem struct {
	ID    string `esper:"id"`
	Score int64  `esper:"score"`
}

type enumMostLeastFreqContainer struct {
	Items []enumMostLeastFreqItem `esper:"items"`
}

func enumMostLeastFreqExtractNum(value string) int {
	parsed, _ := strconv.Atoi(value[1:])
	return parsed
}

func enumMostLeastFreqAssert(t *testing.T, expr Expression[any], want any, label string) {
	t.Helper()
	got := expr.eval(EvalContext{})
	if want == nil {
		if !got.IsNull() {
			t.Fatalf("%s = %v, want null", label, got)
		}
		return
	}
	if !got.Equal(Present(want)) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}

// TestExprEnumMostLeastFrequentScalarNoParamParity covers the no-parameter
// scalar footprint: mostFrequent()/leastFrequent() on string collections.
func TestExprEnumMostLeastFrequentScalarNoParamParity(t *testing.T) {
	values := Literal([]string{"E2", "E1", "E2", "E1", "E3", "E3", "E4", "E3"})
	assertEnumEval(t, EnumMostFrequent[string](values), "E3", "scalar-noparam-most")
	assertEnumEval(t, EnumLeastFrequent[string](values), "E4", "scalar-noparam-least")

	single := Literal([]string{"E1"})
	assertEnumEval(t, EnumMostFrequent[string](single), "E1", "scalar-noparam-single-most")
	assertEnumEval(t, EnumLeastFrequent[string](single), "E1", "scalar-noparam-single-least")

	// Empty collection -> null
	empty := Literal([]string{})
	if got := EnumMostFrequent[string](empty).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("scalar-noparam-empty-most = %v, want null", got)
	}
	if got := EnumLeastFrequent[string](empty).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("scalar-noparam-empty-least = %v, want null", got)
	}

	// Null collection -> null
	if got := EnumMostFrequent[string](NullLiteral[[]string]()).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("scalar-noparam-null-most = %v, want null", got)
	}
	if got := EnumLeastFrequent[string](NullLiteral[[]string]()).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("scalar-noparam-null-least = %v, want null", got)
	}
}

// TestExprEnumMostLeastFrequentScalarParity covers the scalar footprint with
// selector lambdas: mostFrequent(v => extractNum(v)), (v,i) and (v,i,s) forms.
func TestExprEnumMostLeastFrequentScalarParity(t *testing.T) {
	values := Literal([]string{"E2", "E1", "E2", "E1", "E3", "E3", "E4", "E3"})
	extract := Func1[string, int]("extractNum", enumMostLeastFreqExtractNum, EnumElement[string]())

	most := EnumMostFrequentBy[string, int](values, extract)
	assertEnumEval(t, most, 3, "scalar-most-by")

	least := EnumLeastFrequentBy[string, int](values, extract)
	assertEnumEval(t, least, 4, "scalar-least-by")

	// (v, i) => extractNum(v) + i*10
	mostIdx := EnumMostFrequentBy[string, int](values, Add[int](
		Func1[string, int]("extractNum", enumMostLeastFreqExtractNum, EnumElement[string]()),
		Multiply[int](Literal(10), EnumIndex()),
	))
	assertEnumEval(t, mostIdx, 2, "scalar-most-by-index")

	leastIdx := EnumLeastFrequentBy[string, int](values, Add[int](
		Func1[string, int]("extractNum", enumMostLeastFreqExtractNum, EnumElement[string]()),
		Multiply[int](Literal(10), EnumIndex()),
	))
	assertEnumEval(t, leastIdx, 2, "scalar-least-by-index")

	// (v, i, s) => extractNum(v) + i*10 + s*100
	mostIdxSize := EnumMostFrequentBy[string, int](values, Add[int](
		Add[int](
			Func1[string, int]("extractNum", enumMostLeastFreqExtractNum, EnumElement[string]()),
			Multiply[int](Literal(10), EnumIndex()),
		),
		Multiply[int](Literal(100), EnumSize()),
	))
	assertEnumEval(t, mostIdxSize, 802, "scalar-most-by-index-size")

	leastIdxSize := EnumLeastFrequentBy[string, int](values, Add[int](
		Add[int](
			Func1[string, int]("extractNum", enumMostLeastFreqExtractNum, EnumElement[string]()),
			Multiply[int](Literal(10), EnumIndex()),
		),
		Multiply[int](Literal(100), EnumSize()),
	))
	assertEnumEval(t, leastIdxSize, 802, "scalar-least-by-index-size")

	// Single item
	single := Literal([]string{"E1"})
	singleExtract := Func1[string, int]("extractNum", enumMostLeastFreqExtractNum, EnumElement[string]())
	assertEnumEval(t, EnumMostFrequentBy[string, int](single, singleExtract), 1, "scalar-single-most")
	assertEnumEval(t, EnumLeastFrequentBy[string, int](single, singleExtract), 1, "scalar-single-least")

	// Null and empty -> null
	if got := EnumMostFrequentBy[string, int](NullLiteral[[]string](), extract).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("scalar-null-most = %v", got)
	}
	if got := EnumMostFrequentBy[string, int](Literal([]string{}), extract).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("scalar-empty-most = %v", got)
	}
}

// TestExprEnumMostLeastFrequentEventsParity covers the event-collection
// footprint: mostFrequent/leastFrequent with selectors over struct-valued
// items deployed through the normal Build/Deploy path.
func TestExprEnumMostLeastFrequentEventsParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumMostLeastFreqContainer](env, "FreqContainer"); err != nil {
		t.Fatal(err)
	}

	items := Field[enumMostLeastFreqContainer, []enumMostLeastFreqItem]("items")
	score := EnumField[enumMostLeastFreqItem, int64]("score")
	plan, err := env.Build(Select(From[enumMostLeastFreqContainer](env, "FreqContainer"),
		Alias("c0", EnumMostFrequentBy[enumMostLeastFreqItem, int64](items, score)),
		Alias("c1", EnumLeastFrequentBy[enumMostLeastFreqItem, int64](items, score)),
	).Query(StatementName("freq-events")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectDotRows(t, deployment)

	type freqCase struct {
		items []enumMostLeastFreqItem
		most  any
		least any
	}
	cases := []freqCase{
		{
			items: []enumMostLeastFreqItem{{ID: "E1", Score: 12}, {ID: "E2", Score: 11}, {ID: "E2", Score: 2}, {ID: "E3", Score: 12}},
			most:  int64(12), least: int64(11),
		},
		{
			items: []enumMostLeastFreqItem{{ID: "E1", Score: 12}},
			most:  int64(12), least: int64(12),
		},
		{
			items: []enumMostLeastFreqItem{{ID: "E1", Score: 12}, {ID: "E2", Score: 11}, {ID: "E2", Score: 2}, {ID: "E3", Score: 12}, {ID: "E1", Score: 12}, {ID: "E2", Score: 11}, {ID: "E3", Score: 11}},
			most:  int64(12), least: int64(2),
		},
	}
	for i, tc := range cases {
		if err := engine.SendEvent(context.Background(), enumMostLeastFreqContainer{Items: tc.items}); err != nil {
			t.Fatal(err)
		}
		row := (*rows)[i]
		if !row.Get("c0").Equal(Present(tc.most)) {
			t.Fatalf("freq-events case %d most = %#v, want %v", i, row.Get("c0"), tc.most)
		}
		if !row.Get("c1").Equal(Present(tc.least)) {
			t.Fatalf("freq-events case %d least = %#v, want %v", i, row.Get("c1"), tc.least)
		}
	}
}

// TestExprEnumMostLeastFrequentMapParity covers the map-schema deployment path
// with two scalar properties using mostFrequent/leastFrequent without parameters.
func TestExprEnumMostLeastFrequentMapParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "FreqScalar", []FieldSpec{
		FieldDef("strvals", reflect.TypeOf([]string{})),
	}); err != nil {
		t.Fatal(err)
	}

	strvals := Field[map[string]any, []string]("strvals")
	plan, err := env.Build(Select(From[map[string]any](env, "FreqScalar"),
		Alias("c0", EnumMostFrequent[string](strvals)),
		Alias("c1", EnumLeastFrequent[string](strvals)),
	).Query(StatementName("freq-scalar-map")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectDotRows(t, deployment)

	if err := engine.Send(context.Background(), "FreqScalar", map[string]any{
		"strvals": []string{"E2", "E1", "E2", "E1", "E3", "E3", "E4", "E3"},
	}); err != nil {
		t.Fatal(err)
	}
	row := (*rows)[0]
	if !row.Get("c0").Equal(Present("E3")) {
		t.Fatalf("freq-scalar-map most = %#v, want E3", row.Get("c0"))
	}
	if !row.Get("c1").Equal(Present("E4")) {
		t.Fatalf("freq-scalar-map least = %#v, want E4", row.Get("c1"))
	}
}

// TestExprEnumMostLeastFrequentInvalidParity covers the typed-builder failure
// that has a direct Go representation.
func TestExprEnumMostLeastFrequentInvalidParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "FreqInvalid", nil); err != nil {
		t.Fatal(err)
	}
	stream := From[map[string]any](env, "FreqInvalid")
	invalid := EnumMostFrequent[string](nil)
	_, err := env.Build(Select(stream, Alias("same", invalid)).Query(StatementName("freq-invalid")))
	if err == nil || !strings.Contains(err.Error(), `enumeration method "most-frequent" requires a collection expression`) {
		t.Fatalf("freq invalid error = %v", err)
	}
}
