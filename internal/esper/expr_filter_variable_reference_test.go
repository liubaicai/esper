package esper

import (
	"context"
	"testing"
)

// Regression coverage for variable references and boxed fields inside filter
// expressions evaluated under aliased projections.
//
// Nullable boxed properties surface from struct schemas as pointers even
// when the expression declares the unboxed type, so every comparison inside
// a filter must dereference through EqualValues semantics. In/InSlice
// previously compared via strict Value.Equal (Go interface identity) and
// silently never matched boxed fields, while Equal/Between dereferenced.

type filterVariableRefBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     *int32 `esper:"intBoxed"`
}

// filterVariableRefRow is one truth-table row: the boxed value to send and
// whether the deployed filter statement must fire.
type filterVariableRefRow struct {
	boxed *int32
	want  bool
}

func filterVariableRefBoxed(v int32) *int32 { return &v }

// newFilterVariableRefEnvironment registers SupportBean with a nullable
// intBoxed plus the constant variables used by the filter regressions:
// MYCONST = 10 (Integer) and var_ints = {8, 10} (Integer[]).
func newFilterVariableRefEnvironment(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[filterVariableRefBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("MYCONST", int32(10), ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var_ints", []int32{8, 10}, ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

// assertFilterVariableRefFired deploys the built query, applies the rows in
// order against a counting listener, and requires fire/silence per row.
func assertFilterVariableRefFired(t *testing.T, env *Environment, engine *Engine, name string, build func(env *Environment) Query, rows ...filterVariableRefRow) {
	t.Helper()
	plan, err := env.Build(build(env))
	if err != nil {
		t.Fatalf("%s: build: %v", name, err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("%s: deploy: %v", name, err)
	}
	defer func() {
		if err := deployment.Undeploy(context.Background()); err != nil {
			t.Fatalf("%s: undeploy: %v", name, err)
		}
	}()
	got := &[]Result{}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*got = append(*got, batch.New...)
		return nil
	}); err != nil {
		t.Fatalf("%s: subscribe: %v", name, err)
	}
	fired := 0
	for index, row := range rows {
		if err := engine.SendEvent(context.Background(), filterVariableRefBean{TheString: "S", IntPrimitive: 1, IntBoxed: row.boxed}); err != nil {
			t.Fatalf("%s row %d: %v", name, index, err)
		}
		if row.want {
			fired++
		}
		if len(*got) != fired {
			t.Fatalf("%s row %d: results = %d, want fired = %#v", name, index, len(*got), rows[index].want)
		}
	}
}

// TestAliasedSelectInSliceConstantVariableFilter pins InSlice membership over
// a constant []int32 variable inside an aliased projection where the scalar
// side is a nullable boxed field declared as its unboxed element type. The
// same filter must behave identically without aliases, and null never
// matches.
func TestAliasedSelectInSliceConstantVariableFilter(t *testing.T) {
	rows := []filterVariableRefRow{
		{boxed: filterVariableRefBoxed(8), want: true},
		{boxed: filterVariableRefBoxed(9), want: false},
		{boxed: filterVariableRefBoxed(10), want: true},
		{boxed: filterVariableRefBoxed(7), want: false},
		{boxed: nil, want: false},
	}
	builds := map[string]func(env *Environment) Query{
		"aliased": func(env *Environment) Query {
			return Select(
				From[filterVariableRefBean](env, "SupportBean").Filter(
					InSlice[int32](
						Field[filterVariableRefBean, int32]("intBoxed"),
						VariableRef[[]int32]("var_ints"),
					),
				),
				Alias("c0", Field[filterVariableRefBean, string]("theString")),
				Alias("c1", Field[filterVariableRefBean, int]("intPrimitive")),
			).Query(StatementName("in-slice-aliased"))
		},
		"unaliased": func(env *Environment) Query {
			return Select(
				From[filterVariableRefBean](env, "SupportBean").Filter(
					InSlice[int32](
						Field[filterVariableRefBean, int32]("intBoxed"),
						VariableRef[[]int32]("var_ints"),
					),
				),
			).Query(StatementName("in-slice-unaliased"))
		},
	}
	for name, build := range builds {
		env, engine := newFilterVariableRefEnvironment(t)
		defer engine.Close(context.Background())
		assertFilterVariableRefFired(t, env, engine, "InSlice/"+name, build, rows...)
	}
}

// TestAliasedSelectEqualVariableFilter pins variable equality inside an
// aliased projection: a constant Integer variable against the nullable boxed
// field dereferences on both sides, and pure variable predicates evaluate
// against environment variables regardless of aliases.
func TestAliasedSelectEqualVariableFilter(t *testing.T) {
	env, engine := newFilterVariableRefEnvironment(t)
	defer engine.Close(context.Background())
	assertFilterVariableRefFired(t, env, engine, "Equal/constant-variable",
		func(env *Environment) Query {
			return Select(
				From[filterVariableRefBean](env, "SupportBean").Filter(
					Equal[int32](
						VariableRef[int32]("MYCONST"),
						Cast[*int32, int32](Field[filterVariableRefBean, *int32]("intBoxed")),
					),
				),
				Alias("c0", Field[filterVariableRefBean, string]("theString")),
				Alias("c1", Field[filterVariableRefBean, int]("intPrimitive")),
			).Query(StatementName("equal-const"))
		},
		filterVariableRefRow{boxed: filterVariableRefBoxed(10), want: true},
		filterVariableRefRow{boxed: filterVariableRefBoxed(9), want: false},
		filterVariableRefRow{boxed: nil, want: false})

	env2, engine2 := newFilterVariableRefEnvironment(t)
	defer engine2.Close(context.Background())
	assertFilterVariableRefFired(t, env2, engine2, "Equal/variable-predicate-match",
		func(env *Environment) Query {
			return Select(
				From[filterVariableRefBean](env, "SupportBean").Filter(
					Equal[int32](VariableRef[int32]("MYCONST"), Literal(int32(10))),
				),
				Alias("c0", Field[filterVariableRefBean, string]("theString")),
			).Query(StatementName("equal-var-match"))
		},
		filterVariableRefRow{boxed: nil, want: true},
		filterVariableRefRow{boxed: filterVariableRefBoxed(1), want: true})

	env3, engine3 := newFilterVariableRefEnvironment(t)
	defer engine3.Close(context.Background())
	assertFilterVariableRefFired(t, env3, engine3, "Equal/variable-predicate-no-match",
		func(env *Environment) Query {
			return Select(
				From[filterVariableRefBean](env, "SupportBean").Filter(
					Equal[int32](VariableRef[int32]("MYCONST"), Literal(int32(9))),
				),
				Alias("c0", Field[filterVariableRefBean, string]("theString")),
			).Query(StatementName("equal-var-nomatch"))
		},
		filterVariableRefRow{boxed: filterVariableRefBoxed(9), want: false})
}

// TestAliasedSelectBoxedFieldDereference pins the unboxed dereference of a
// pointer-boxed property across every comparison family that reads it inside
// an aliased projection: Equal, Greater, In, InSlice and Between must all
// agree with their unboxed counterparts, and null never matches.
func TestAliasedSelectBoxedFieldDereference(t *testing.T) {
	cases := []struct {
		name   string
		filter func(env *Environment) Expression[bool]
		want   []bool // per sent bean: 10, 7, null
	}{
		{"equal-literal", func(env *Environment) Expression[bool] {
			return Equal[int32](Literal(int32(7)), Field[filterVariableRefBean, int32]("intBoxed"))
		}, []bool{false, true, false}},
		{"greater-literal", func(env *Environment) Expression[bool] {
			return Greater[int32](Field[filterVariableRefBean, int32]("intBoxed"), Literal(int32(8)))
		}, []bool{true, false, false}},
		{"in-literals", func(env *Environment) Expression[bool] {
			return In[int32](Field[filterVariableRefBean, int32]("intBoxed"), Literal(int32(7)), Literal(int32(10)))
		}, []bool{true, true, false}},
		{"in-slice-variable", func(env *Environment) Expression[bool] {
			return InSlice[int32](Field[filterVariableRefBean, int32]("intBoxed"), VariableRef[[]int32]("var_ints"))
		}, []bool{true, false, false}},
		{"between-literals", func(env *Environment) Expression[bool] {
			return Between[int32](Field[filterVariableRefBean, int32]("intBoxed"), Literal(int32(8)), Literal(int32(10)))
		}, []bool{true, false, false}},
	}
	for _, testCase := range cases {
		env, engine := newFilterVariableRefEnvironment(t)
		defer engine.Close(context.Background())
		rows := []filterVariableRefRow{
			{boxed: filterVariableRefBoxed(10), want: testCase.want[0]},
			{boxed: filterVariableRefBoxed(7), want: testCase.want[1]},
			{boxed: nil, want: testCase.want[2]},
		}
		assertFilterVariableRefFired(t, env, engine, "dereference/"+testCase.name,
			func(env *Environment) Query {
				return Select(
					From[filterVariableRefBean](env, "SupportBean").Filter(testCase.filter(env)),
					Alias("c0", Field[filterVariableRefBean, string]("theString")),
					Alias("c1", Field[filterVariableRefBean, int]("intPrimitive")),
				).Query(StatementName("dereference-" + testCase.name))
			}, rows...)
	}
}

// filterVariableRefEnum is the named comparable Go representation of an
// enum-like event field and variable type.
type filterVariableRefEnum string

const (
	filterVariableRefEnumValue1 filterVariableRefEnum = "ENUM_VALUE_1"
	filterVariableRefEnumValue2 filterVariableRefEnum = "ENUM_VALUE_2"
	filterVariableRefEnumValue3 filterVariableRefEnum = "ENUM_VALUE_3"
)

type filterVariableRefEnumBean struct {
	TheString string                `esper:"theString"`
	EnumValue filterVariableRefEnum `esper:"enumValue"`
}

// TestNamedKindMapSendCoercesTypedValue pins that a Map send against a
// struct-registered event type materializes the declared struct and coerces
// raw string payloads into the named string kind: the stored field is a
// typed enum value, not the plain string from the payload.
func TestNamedKindMapSendCoercesTypedValue(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[filterVariableRefEnumBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer engine.Close(context.Background())
	plan, err := env.Build(
		From[filterVariableRefEnumBean](env, "SupportBean").Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var seen []any
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				seen = append(seen, event.Underlying())
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	payloads := []filterVariableRefEnum{
		filterVariableRefEnumValue3,
		filterVariableRefEnumValue2,
		filterVariableRefEnumValue1,
	}
	for _, value := range payloads {
		if err := engine.Send(context.Background(), "SupportBean", map[string]any{
			"theString": "S",
			"enumValue": string(value),
		}); err != nil {
			t.Fatalf("map send %q: %v", value, err)
		}
	}
	if len(seen) != len(payloads) {
		t.Fatalf("delivered %d events, want %d", len(seen), len(payloads))
	}
	for index, underlying := range seen {
		bean, ok := underlying.(filterVariableRefEnumBean)
		if !ok {
			t.Fatalf("row %d underlying = %#v (%T), want %T", index, underlying, underlying, filterVariableRefEnumBean{})
		}
		if bean.EnumValue != payloads[index] {
			t.Fatalf("row %d enumValue = %#v (%T), want %q",
				index, bean.EnumValue, bean.EnumValue, payloads[index])
		}
	}
}

// TestAliasedSelectEnumOrMembershipFilter pins the final constant-variable
// shape end to end under an aliased projection with Map sends carrying raw
// strings: `enumValue in (var_enumarr, var_enumone)` as
// Or(InSlice(enumField, sliceVariable), Equal(scalarVariable, enumField))
// must fire for every matching candidate element - both the slice arms
// (ENUM_VALUE_2 via var_enumarr, ENUM_VALUE_1 via var_enumarr) and the
// scalar arm (ENUM_VALUE_2 via var_enumone) - and stay silent otherwise.
func TestAliasedSelectEnumOrMembershipFilter(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[filterVariableRefEnumBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var_enumone", filterVariableRefEnumValue2, ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var_enumarr", []filterVariableRefEnum{filterVariableRefEnumValue2, filterVariableRefEnumValue1}, ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer engine.Close(context.Background())
	enumField := Field[filterVariableRefEnumBean, filterVariableRefEnum]("enumValue")
	plan, err := env.Build(Select(
		From[filterVariableRefEnumBean](env, "SupportBean").Filter(
			Or(
				InSlice[filterVariableRefEnum](enumField, VariableRef[[]filterVariableRefEnum]("var_enumarr")),
				Equal[filterVariableRefEnum](VariableRef[filterVariableRefEnum]("var_enumone"), enumField),
			),
		),
		Alias("c0", Field[filterVariableRefEnumBean, string]("theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	got := &[]Result{}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*got = append(*got, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sends := []struct {
		value string
		want  bool
	}{
		{"ENUM_VALUE_3", false},
		{"ENUM_VALUE_2", true}, // var_enumarr element + var_enumone scalar arm
		{"ENUM_VALUE_1", true}, // var_enumarr second element
	}
	fired := 0
	for index, send := range sends {
		if err := engine.Send(context.Background(), "SupportBean", map[string]any{
			"theString": "S",
			"enumValue": send.value,
		}); err != nil {
			t.Fatalf("map send row %d: %v", index, err)
		}
		if send.want {
			fired++
		}
		if len(*got) != fired {
			t.Fatalf("row %d (%s): results = %d, want fired = %v", index, send.value, len(*got), send.want)
		}
	}
}

// TestMembershipMultiMatchPerCandidateSlot pins the Java per-slot IN
// delivery with the minimal constant-variable repro: enumValue in
// (var_enumarr{V2,V1}, var_enumone=V2) under an aliased projection emits one
// row per matching candidate slot - ENUM_VALUE_2 matches the array element
// AND the scalar candidate (two rows), ENUM_VALUE_1 one row, ENUM_VALUE_3
// none.
func TestMembershipMultiMatchPerCandidateSlot(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[filterVariableRefEnumBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var_enumarr", []filterVariableRefEnum{filterVariableRefEnumValue2, filterVariableRefEnumValue1}, ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var_enumone", filterVariableRefEnumValue2, ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer engine.Close(context.Background())
	plan, err := env.Build(Select(
		From[filterVariableRefEnumBean](env, "SupportBean").Filter(
			InOf(
				Field[filterVariableRefEnumBean, filterVariableRefEnum]("enumValue"),
				VariableRef[[]filterVariableRefEnum]("var_enumarr"),
				VariableRef[filterVariableRefEnum]("var_enumone"),
			),
		),
		Alias("c0", Field[filterVariableRefEnumBean, string]("theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	got := &[]Result{}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*got = append(*got, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sends := []struct {
		value string
		tag   string
		want  int
	}{
		{"ENUM_VALUE_3", "S1", 0},
		{"ENUM_VALUE_2", "S2", 2},
		{"ENUM_VALUE_1", "S3", 1},
	}
	delivered := 0
	for _, send := range sends {
		if err := engine.Send(context.Background(), "SupportBean", map[string]any{
			"theString": send.tag,
			"enumValue": send.value,
		}); err != nil {
			t.Fatalf("map send %s: %v", send.value, err)
		}
		delivered += send.want
		if len(*got) != delivered {
			t.Fatalf("send %s: cumulative rows = %d, want %d", send.value, len(*got), delivered)
		}
	}
	if len(*got) != 3 {
		t.Fatalf("total rows = %d, want 3 (S2 doubled, S3 single)", len(*got))
	}
	for _, index := range []int{0, 1} {
		row, ok := (*got)[index].Row()
		if !ok || row.Get("c0").Any() != "S2" {
			t.Fatalf("row %d = %#v, want the doubled S2 row", index, (*got)[index])
		}
	}
	row, ok := (*got)[2].Row()
	if !ok || row.Get("c0").Any() != "S3" {
		t.Fatalf("row 2 = %#v (%v), want the S3 row", (*got)[2], ok)
	}
}

// TestMembershipSingleMatchArrayFilterStaysSingle pins that single-element
// membership stays one row per event: every value of intBoxed matches at
// most one element of var_ints{8,10}, so counts never multiply.
func TestMembershipSingleMatchArrayFilterStaysSingle(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[filterVariableRefBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var_ints", []int32{8, 10}, ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer engine.Close(context.Background())
	plan, err := env.Build(Select(
		From[filterVariableRefBean](env, "SupportBean").Filter(
			InSlice[int32](
				Field[filterVariableRefBean, int32]("intBoxed"),
				VariableRef[[]int32]("var_ints"),
			),
		),
		Alias("c0", Field[filterVariableRefBean, string]("theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	got := &[]Result{}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*got = append(*got, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sends := []struct {
		boxed *int32
		want  int
	}{
		{boxed: filterVariableRefBoxed(8), want: 1},
		{boxed: filterVariableRefBoxed(10), want: 1},
		{boxed: filterVariableRefBoxed(7), want: 0},
		{boxed: nil, want: 0},
	}
	delivered := 0
	for index, send := range sends {
		if err := engine.SendEvent(context.Background(), filterVariableRefBean{TheString: "S", IntPrimitive: 1, IntBoxed: send.boxed}); err != nil {
			t.Fatal(err)
		}
		delivered += send.want
		if len(*got) != delivered {
			t.Fatalf("send %d: cumulative rows = %d, want %d", index, len(*got), delivered)
		}
	}
}

// TestMembershipMixedArrayAndScalarCounting pins slot counting for mixed
// array and scalar candidates in one membership: intBoxed in
// (var_ints{8,10}, var_intstwo{9}, MYCONST=10) yields two rows for 10
// (array element + scalar), one row each for 8 and 9, none for 7.
func TestMembershipMixedArrayAndScalarCounting(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[filterVariableRefBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var_ints", []int32{8, 10}, ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var_intstwo", []int32{9}, ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("MYCONST", int32(10), ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer engine.Close(context.Background())
	plan, err := env.Build(Select(
		From[filterVariableRefBean](env, "SupportBean").Filter(
			InOf(
				Field[filterVariableRefBean, int32]("intBoxed"),
				VariableRef[[]int32]("var_ints"),
				VariableRef[[]int32]("var_intstwo"),
				VariableRef[int32]("MYCONST"),
			),
		),
		Alias("c0", Field[filterVariableRefBean, string]("theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	got := &[]Result{}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*got = append(*got, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sends := []struct {
		boxed *int32
		want  int
	}{
		{boxed: filterVariableRefBoxed(10), want: 2},
		{boxed: filterVariableRefBoxed(9), want: 1},
		{boxed: filterVariableRefBoxed(8), want: 1},
		{boxed: filterVariableRefBoxed(7), want: 0},
	}
	delivered := 0
	for index, send := range sends {
		if err := engine.SendEvent(context.Background(), filterVariableRefBean{TheString: "S", IntPrimitive: 1, IntBoxed: send.boxed}); err != nil {
			t.Fatal(err)
		}
		delivered += send.want
		if len(*got) != delivered {
			t.Fatalf("send %d: cumulative rows = %d, want %d", index, len(*got), delivered)
		}
	}
}
