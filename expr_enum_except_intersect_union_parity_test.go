package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type enumSetItem struct {
	ID    string `esper:"id"`
	Score int64  `esper:"score"`
}

type enumSetContainer struct {
	Items    []enumSetItem `esper:"items"`
	ItemsTwo []enumSetItem `esper:"itemstwo"`
}

// TestExprEnumExceptIntersectUnionScalarParity covers the scalar footprint:
// except/intersect/union over string collections including length, empty,
// null-left and null-right semantics, matching Java removeAll/retainAll/concat.
func TestExprEnumExceptIntersectUnionScalarParity(t *testing.T) {
	strs := func(values ...string) []string { return values }
	empty := []string{}

	except := func(left, right []string) Expression[[]string] {
		return EnumExcept[string](Literal(left), Literal(right))
	}
	intersect := func(left, right []string) Expression[[]string] {
		return EnumIntersect[string](Literal(left), Literal(right))
	}
	union := func(left, right []string) Expression[[]string] {
		return EnumUnion[string](Literal(left), Literal(right))
	}

	// E1,E2 except/intersect/union E3,E4
	assertEnumEval(t, except(strs("E1", "E2"), strs("E3", "E4")), strs("E1", "E2"), "scalar-except-disjoint")
	if got := intersect(strs("E1", "E2"), strs("E3", "E4")).eval(EvalContext{}); !got.Equal(Present([]string{})) {
		t.Fatalf("scalar-intersect-disjoint = %v, want empty", got)
	}
	assertEnumEval(t, union(strs("E1", "E2"), strs("E3", "E4")), strs("E1", "E2", "E3", "E4"), "scalar-union-disjoint")

	// E1,E3,E5 except/intersect/union E3,E4
	assertEnumEval(t, except(strs("E1", "E3", "E5"), strs("E3", "E4")), strs("E1", "E5"), "scalar-except-overlap")
	assertEnumEval(t, intersect(strs("E1", "E3", "E5"), strs("E3", "E4")), strs("E3"), "scalar-intersect-overlap")
	assertEnumEval(t, union(strs("E1", "E3", "E5"), strs("E3", "E4")), strs("E1", "E3", "E5", "E3", "E4"), "scalar-union-overlap")

	// Empty left
	if got := except(empty, strs("E3", "E4")).eval(EvalContext{}); !got.Equal(Present(empty)) {
		t.Fatalf("scalar-except-empty-left = %v", got)
	}
	if got := intersect(empty, strs("E3", "E4")).eval(EvalContext{}); !got.Equal(Present(empty)) {
		t.Fatalf("scalar-intersect-empty-left = %v", got)
	}
	if got := union(empty, strs("E3", "E4")).eval(EvalContext{}); !got.Equal(Present(strs("E3", "E4"))) {
		t.Fatalf("scalar-union-empty-left = %v", got)
	}

	// Empty right -> return left unchanged (Java: other.isEmpty())
	assertEnumEval(t, except(strs("E1", "E2"), empty), strs("E1", "E2"), "scalar-except-empty-right")
	assertEnumEval(t, intersect(strs("E1", "E2"), empty), strs("E1", "E2"), "scalar-intersect-empty-right")
	assertEnumEval(t, union(strs("E1", "E2"), empty), strs("E1", "E2"), "scalar-union-empty-right")

	// Null left -> null result
	if got := EnumExcept[string](NullLiteral[[]string](), Literal(strs("E3", "E4"))).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("scalar-except-null-left = %v, want null", got)
	}
	if got := EnumIntersect[string](NullLiteral[[]string](), Literal(strs("E3", "E4"))).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("scalar-intersect-null-left = %v, want null", got)
	}
	if got := EnumUnion[string](NullLiteral[[]string](), Literal(strs("E3", "E4"))).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("scalar-union-null-left = %v, want null", got)
	}

	// Null right -> return left unchanged (Java: other == null)
	assertEnumEval(t, EnumExcept[string](Literal(strs("E1", "E2")), NullLiteral[[]string]()), strs("E1", "E2"), "scalar-except-null-right")
	assertEnumEval(t, EnumIntersect[string](Literal(strs("E1", "E2")), NullLiteral[[]string]()), strs("E1", "E2"), "scalar-intersect-null-right")
	assertEnumEval(t, EnumUnion[string](Literal(strs("E1", "E2")), NullLiteral[[]string]()), strs("E1", "E2"), "scalar-union-null-right")

	// Both null -> null
	if got := EnumExcept[string](NullLiteral[[]string](), NullLiteral[[]string]()).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("scalar-except-both-null = %v, want null", got)
	}
}

// TestExprEnumExceptIntersectUnionContainedParity covers the event-collection
// footprint: except/intersect/union over struct-valued items deployed through
// the normal Build/Deploy path. Items are compared by value equality.
func TestExprEnumExceptIntersectUnionContainedParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumSetContainer](env, "SetContainer"); err != nil {
		t.Fatal(err)
	}

	items := Field[enumSetContainer, []enumSetItem]("items")
	itemsTwo := Field[enumSetContainer, []enumSetItem]("itemstwo")
	exceptIDs := EnumSelect[enumSetItem, string](EnumExcept[enumSetItem](items, itemsTwo), EnumField[enumSetItem, string]("id"))
	intersectIDs := EnumSelect[enumSetItem, string](EnumIntersect[enumSetItem](items, itemsTwo), EnumField[enumSetItem, string]("id"))
	unionIDs := EnumSelect[enumSetItem, string](EnumUnion[enumSetItem](items, itemsTwo), EnumField[enumSetItem, string]("id"))

	plan, err := env.Build(Select(From[enumSetContainer](env, "SetContainer"),
		Alias("except_ids", exceptIDs),
		Alias("intersect_ids", intersectIDs),
		Alias("union_ids", unionIDs),
	).Query(StatementName("set-contained")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}

	rows := collectDotRows(t, deployment)
	first := []enumSetItem{
		{ID: "E1", Score: 1}, {ID: "E2", Score: 10}, {ID: "E3", Score: 1}, {ID: "E4", Score: 10}, {ID: "E5", Score: 11},
	}
	second := []enumSetItem{
		{ID: "E1", Score: 1}, {ID: "E3", Score: 1}, {ID: "E4", Score: 10},
	}
	if err := engine.SendEvent(context.Background(), enumSetContainer{Items: first, ItemsTwo: second}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 {
		t.Fatalf("contained set rows = %d, want 1", len(*rows))
	}
	row := (*rows)[0]
	if !row.Get("except_ids").Equal(Present([]string{"E2", "E5"})) {
		t.Fatalf("except IDs = %#v, want E2,E5", row.Get("except_ids"))
	}
	if !row.Get("intersect_ids").Equal(Present([]string{"E1", "E3", "E4"})) {
		t.Fatalf("intersect IDs = %#v, want E1,E3,E4", row.Get("intersect_ids"))
	}
	if !row.Get("union_ids").Equal(Present([]string{"E1", "E2", "E3", "E4", "E5", "E1", "E3", "E4"})) {
		t.Fatalf("union IDs = %#v, want E1,E2,E3,E4,E5,E1,E3,E4", row.Get("union_ids"))
	}
}

// TestExprEnumExceptIntersectUnionStringArrayIntersectionParity covers
// ExprEnumStringArrayIntersection: a filter using intersect().countOf() > 0
// on two string-array properties, deployed through a map schema.
func TestExprEnumExceptIntersectUnionStringArrayIntersectionParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "StringArrayEvent", []FieldSpec{
		FieldDef("meta1", reflect.TypeOf([]string{})),
		FieldDef("meta2", reflect.TypeOf([]string{})),
	}); err != nil {
		t.Fatal(err)
	}

	meta1 := Field[map[string]any, []string]("meta1")
	meta2 := Field[map[string]any, []string]("meta2")
	intersectCount := EnumCount[string](EnumIntersect[string](meta1, meta2))
	plan, err := env.Build(Select(
		From[map[string]any](env, "StringArrayEvent").Filter(Greater[int64](intersectCount, Literal(int64(0)))),
		Alias("matched", Literal(true)),
	).Query(StatementName("string-array-intersect")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	matched := collectDotRows(t, deployment)

	cases := []struct {
		meta1    []string
		meta2    []string
		expected bool
	}{
		{[]string{"a", "b"}, []string{"a", "b"}, true},
		{[]string{"c", "d"}, []string{"a", "b"}, false},
		{[]string{"c", "d"}, []string{"a", "d"}, true},
		{[]string{"a", "d", "a", "a"}, []string{"b", "c"}, false},
		{[]string{"a", "d", "a", "a"}, []string{"b", "d"}, true},
	}
	for _, tc := range cases {
		before := len(*matched)
		if err := engine.Send(context.Background(), "StringArrayEvent", map[string]any{
			"meta1": tc.meta1,
			"meta2": tc.meta2,
		}); err != nil {
			t.Fatal(err)
		}
		after := len(*matched)
		if tc.expected && after != before+1 {
			t.Fatalf("meta1=%v meta2=%v: expected match but got %d rows", tc.meta1, tc.meta2, after-before)
		}
		if !tc.expected && after != before {
			t.Fatalf("meta1=%v meta2=%v: expected no match but got %d rows", tc.meta1, tc.meta2, after-before)
		}
	}
}

// TestExprEnumUnionWhereParity covers ExprEnumUnionWhere: two where-filtered
// event collections combined with union, deployed through Build/Deploy.
func TestExprEnumUnionWhereParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumSetContainer](env, "UnionWhereContainer"); err != nil {
		t.Fatal(err)
	}

	items := Field[enumSetContainer, []enumSetItem]("items")
	score := EnumField[enumSetItem, int64]("score")
	id := EnumField[enumSetItem, string]("id")
	where10 := EnumWhere[enumSetItem](items, Equal[int64](score, Literal(int64(10))))
	where11 := EnumWhere[enumSetItem](items, Equal[int64](score, Literal(int64(11))))
	unionIDs := EnumSelect[enumSetItem, string](EnumUnion[enumSetItem](where10, where11), id)

	plan, err := env.Build(Select(From[enumSetContainer](env, "UnionWhereContainer"),
		Alias("val0", unionIDs),
	).Query(StatementName("union-where")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectDotRows(t, deployment)

	// Case 1: E1(1), E2(10), E3(1), E4(10), E5(11) -> union(p10, p11) = E2,E4,E5
	if err := engine.SendEvent(context.Background(), enumSetContainer{Items: []enumSetItem{
		{ID: "E1", Score: 1}, {ID: "E2", Score: 10}, {ID: "E3", Score: 1}, {ID: "E4", Score: 10}, {ID: "E5", Score: 11},
	}}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 || !(*rows)[0].Get("val0").Equal(Present([]string{"E2", "E4", "E5"})) {
		t.Fatalf("union-where case 1 = %#v, want E2,E4,E5", safeRowIDs(*rows, "val0"))
	}

	// Case 2: E1(10), E2(1), E3(1) -> union(p10, p11) = E1
	if err := engine.SendEvent(context.Background(), enumSetContainer{Items: []enumSetItem{
		{ID: "E1", Score: 10}, {ID: "E2", Score: 1}, {ID: "E3", Score: 1},
	}}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 2 || !(*rows)[1].Get("val0").Equal(Present([]string{"E1"})) {
		t.Fatalf("union-where case 2 = %#v, want E1", safeRowIDs((*rows)[1:], "val0"))
	}

	// Case 3: E1(1), E2(1), E3(10), E4(11) -> union(p10, p11) = E3,E4
	if err := engine.SendEvent(context.Background(), enumSetContainer{Items: []enumSetItem{
		{ID: "E1", Score: 1}, {ID: "E2", Score: 1}, {ID: "E3", Score: 10}, {ID: "E4", Score: 11},
	}}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 3 || !(*rows)[2].Get("val0").Equal(Present([]string{"E3", "E4"})) {
		t.Fatalf("union-where case 3 = %#v, want E3,E4", safeRowIDs((*rows)[2:], "val0"))
	}

	// Case 5: empty items -> empty
	if err := engine.SendEvent(context.Background(), enumSetContainer{Items: []enumSetItem{}}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 4 || !(*rows)[3].Get("val0").Equal(Present([]string{})) {
		t.Fatalf("union-where case 5 = %#v, want empty", (*rows)[3].Get("val0"))
	}

	// Null items through direct evaluation: NullLiteral propagates through
	// EnumWhere -> EnumUnion, matching Java make2Value((String[]) null).
	nullItems := NullLiteral[[]enumSetItem]()
	nullWhere := EnumWhere[enumSetItem](nullItems, Equal[int64](score, Literal(int64(10))))
	nullUnion := EnumUnion[enumSetItem](nullWhere, nullWhere)
	if got := nullUnion.eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("union-where null items = %v, want null", got)
	}
}

// TestExprEnumExceptIntersectUnionInvalidParity covers the typed-builder
// failure that has a direct Go representation. Java additionally rejects
// event-type mismatches through EPL compilation; those are recorded as a
// difference because Go's generic collection type is explicit.
func TestExprEnumExceptIntersectUnionInvalidParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "SetInvalid", nil); err != nil {
		t.Fatal(err)
	}
	stream := From[map[string]any](env, "SetInvalid")
	invalid := EnumUnion[string](nil, nil)
	_, err := env.Build(Select(stream, Alias("same", invalid)).Query(StatementName("set-invalid")))
	if err == nil || !strings.Contains(err.Error(), `enumeration method "union" requires a collection expression`) {
		t.Fatalf("set invalid error = %v", err)
	}
}

func safeRowIDs(rows []Row, field string) any {
	if len(rows) == 0 {
		return "no rows"
	}
	return rows[0].Get(field)
}
