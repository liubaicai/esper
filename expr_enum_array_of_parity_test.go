package esper

import (
	"context"
	"strconv"
	"testing"
)

type enumArrayOfScalarEvent struct {
	StrVals []string `esper:"strvals"`
}

type enumArrayOfItem struct {
	ID  string `esper:"id"`
	P00 int64  `esper:"p00"`
}

type enumArrayOfContainerEvent struct {
	Contained []enumArrayOfItem `esper:"contained"`
}

func enumArrayOfItoa(value int64) string {
	return strconv.FormatInt(value, 10)
}

func enumArrayOfParseInt(value string) int {
	parsed, _ := strconv.Atoi(value)
	return parsed
}

// TestExprEnumArrayOfScalarParity covers ExprEnumArrayOfScalar: arrayOf()
// without a lambda, with v, (v,i), (v,i,s) and (v,i) returning the index.
func TestExprEnumArrayOfScalarParity(t *testing.T) {
	values := Literal([]string{"A", "B", "C"})
	assertEnumEval(t, EnumArrayOf[string](values), []string{"A", "B", "C"}, "array-of-copy")
	assertEnumEval(t, EnumArrayOfSelect[string, string](values, EnumElement[string]()), []string{"A", "B", "C"}, "array-of-v")

	withIndex := EnumArrayOfSelect[string, string](values, Concat(
		EnumElement[string](),
		Concat(Literal("_"), Func1[int64, string]("itoa", enumArrayOfItoa, EnumIndex())),
	))
	assertEnumEval(t, withIndex, []string{"A_0", "B_1", "C_2"}, "array-of-v-index")

	withIndexSize := EnumArrayOfSelect[string, string](values, Concat(
		EnumElement[string](),
		Concat(Literal("_"), Concat(
			Func1[int64, string]("itoa", enumArrayOfItoa, EnumIndex()),
			Concat(Literal("_"), Func1[int64, string]("itoa", enumArrayOfItoa, EnumSize())),
		)),
	))
	assertEnumEval(t, withIndexSize, []string{"A_0_3", "B_1_3", "C_2_3"}, "array-of-v-index-size")

	indexOnly := EnumArrayOfSelect[string, int64](values, EnumIndex())
	assertEnumEval(t, indexOnly, []int64{0, 1, 2}, "array-of-index")

	empty := Literal([]string{})
	assertEnumEval(t, EnumArrayOf[string](empty), []string{}, "array-of-empty")
	assertEnumEval(t, EnumArrayOfSelect[string, string](empty, EnumElement[string]()), []string{}, "array-of-empty-select")
	assertEnumEval(t, EnumArrayOfSelect[string, int64](empty, EnumIndex()), []int64{}, "array-of-empty-index")

	null := NullLiteral[[]string]()
	if got := EnumArrayOf[string](null).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("array-of-null = %v, want null", got)
	}
	if got := EnumArrayOfSelect[string, string](null, EnumElement[string]()).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("array-of-null-select = %v, want null", got)
	}
}

// TestExprEnumArrayOfEventsParity covers ExprEnumArrayOfEvents: arrayOf over
// event collections with element, index and size lambda parameters.
func TestExprEnumArrayOfEventsParity(t *testing.T) {
	items := Literal([]enumArrayOfItem{
		{ID: "E1", P00: 1},
		{ID: "E2", P00: 9},
		{ID: "E2", P00: 2},
	})
	p00 := EnumField[enumArrayOfItem, int64]("p00")
	ten := Literal(int64(10))
	hundred := Literal(int64(100))

	plain := EnumArrayOfSelect[enumArrayOfItem, int64](items, p00)
	assertEnumEval(t, plain, []int64{1, 9, 2}, "array-of-event-value")

	withIndex := EnumArrayOfSelect[enumArrayOfItem, int64](items, Add[int64](p00, Multiply[int64](ten, EnumIndex())))
	assertEnumEval(t, withIndex, []int64{1, 19, 22}, "array-of-event-index")

	withIndexSize := EnumArrayOfSelect[enumArrayOfItem, int64](items, Add[int64](
		Add[int64](p00, Multiply[int64](ten, EnumIndex())),
		Multiply[int64](hundred, EnumSize()),
	))
	assertEnumEval(t, withIndexSize, []int64{301, 319, 322}, "array-of-event-index-size")

	empty := Literal([]enumArrayOfItem{})
	assertEnumEval(t, EnumArrayOfSelect[enumArrayOfItem, int64](empty, p00), []int64{}, "array-of-event-empty")

	null := NullLiteral[[]enumArrayOfItem]()
	if got := EnumArrayOfSelect[enumArrayOfItem, int64](null, p00).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("array-of-event-null = %v, want null", got)
	}
}

// TestExprEnumArrayOfSelectFromParity covers the selectFrom(...).arrayOf()
// chained variants: scalar parse, scalar with index, and event projection.
func TestExprEnumArrayOfSelectFromParity(t *testing.T) {
	values := Literal([]string{"1", "2", "3"})
	parsed := EnumArrayOf[int](EnumSelect[string, int](values, Func1[string, int]("parseInt", enumArrayOfParseInt, EnumElement[string]())))
	assertEnumEval(t, parsed, []int{1, 2, 3}, "select-from-parse-array-of")

	withIndex := EnumArrayOf[string](EnumSelect[string, string](values, Concat(
		EnumElement[string](),
		Concat(Literal("-"), Func1[int64, string]("itoa", enumArrayOfItoa, EnumIndex())),
	)))
	assertEnumEval(t, withIndex, []string{"1-0", "2-1", "3-2"}, "select-from-index-array-of")

	items := Literal([]enumArrayOfItem{
		{ID: "E1", P00: 12},
		{ID: "E2", P00: 11},
		{ID: "E3", P00: 2},
	})
	ids := EnumArrayOf[string](EnumSelect[enumArrayOfItem, string](items, EnumField[enumArrayOfItem, string]("id")))
	assertEnumEval(t, ids, []string{"E1", "E2", "E3"}, "select-from-event-array-of")
}

// TestExprEnumArrayOfStatementProjectionParity runs the same projections
// through Environment.Build/Engine.Deploy so event-property sources and the
// typed Plan path are exercised, not just direct expression evaluation.
func TestExprEnumArrayOfStatementProjectionParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumArrayOfScalarEvent](env, "SupportCollection"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[enumArrayOfContainerEvent](env, "SupportBean_ST0_Container"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)

	values := Field[enumArrayOfScalarEvent, []string]("strvals")
	scalarPlan, err := env.Build(Select(From[enumArrayOfScalarEvent](env, "SupportCollection"),
		Alias("c0", EnumArrayOf[string](values)),
		Alias("c1", EnumArrayOfSelect[string, string](values, EnumElement[string]())),
		Alias("c2", EnumArrayOfSelect[string, string](values, Concat(
			EnumElement[string](),
			Concat(Literal("_"), Func1[int64, string]("itoa", enumArrayOfItoa, EnumIndex())),
		))),
		Alias("c3", EnumArrayOfSelect[string, string](values, Concat(
			EnumElement[string](),
			Concat(Literal("_"), Concat(
				Func1[int64, string]("itoa", enumArrayOfItoa, EnumIndex()),
				Concat(Literal("_"), Func1[int64, string]("itoa", enumArrayOfItoa, EnumSize())),
			)),
		))),
		Alias("c4", EnumArrayOfSelect[string, int64](values, EnumIndex())),
	).Query(StatementName("array-of-scalar")))
	if err != nil {
		t.Fatal(err)
	}
	scalarDeployment, err := engine.Deploy(context.Background(), scalarPlan)
	if err != nil {
		t.Fatal(err)
	}
	scalarRows := collectDotRows(t, scalarDeployment)
	if err := engine.SendEvent(context.Background(), enumArrayOfScalarEvent{StrVals: []string{"A", "B", "C"}}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), enumArrayOfScalarEvent{StrVals: []string{}}); err != nil {
		t.Fatal(err)
	}
	if len(*scalarRows) != 2 {
		t.Fatalf("scalar array-of rows = %d, want 2", len(*scalarRows))
	}
	first := (*scalarRows)[0]
	if !first.Get("c0").Equal(Present([]string{"A", "B", "C"})) || !first.Get("c1").Equal(Present([]string{"A", "B", "C"})) {
		t.Fatalf("scalar array-of row 0 = %#v", first.AsMap())
	}
	if !first.Get("c2").Equal(Present([]string{"A_0", "B_1", "C_2"})) || !first.Get("c3").Equal(Present([]string{"A_0_3", "B_1_3", "C_2_3"})) || !first.Get("c4").Equal(Present([]int64{0, 1, 2})) {
		t.Fatalf("scalar array-of row 0 projections = %#v", first.AsMap())
	}
	second := (*scalarRows)[1]
	if !second.Get("c0").Equal(Present([]string{})) || !second.Get("c1").Equal(Present([]string{})) || !second.Get("c2").Equal(Present([]string{})) || !second.Get("c3").Equal(Present([]string{})) || !second.Get("c4").Equal(Present([]int64{})) {
		t.Fatalf("scalar array-of empty row = %#v", second.AsMap())
	}

	contained := Field[enumArrayOfContainerEvent, []enumArrayOfItem]("contained")
	p00 := EnumField[enumArrayOfItem, int64]("p00")
	eventPlan, err := env.Build(Select(From[enumArrayOfContainerEvent](env, "SupportBean_ST0_Container"),
		Alias("c0", EnumArrayOfSelect[enumArrayOfItem, int64](contained, p00)),
		Alias("c1", EnumArrayOfSelect[enumArrayOfItem, int64](contained, Add[int64](p00, Multiply[int64](Literal(int64(10)), EnumIndex())))),
		Alias("c2", EnumArrayOfSelect[enumArrayOfItem, int64](contained, Add[int64](
			Add[int64](p00, Multiply[int64](Literal(int64(10)), EnumIndex())),
			Multiply[int64](Literal(int64(100)), EnumSize()),
		))),
	).Query(StatementName("array-of-event")))
	if err != nil {
		t.Fatal(err)
	}
	eventDeployment, err := engine.Deploy(context.Background(), eventPlan)
	if err != nil {
		t.Fatal(err)
	}
	eventRows := collectDotRows(t, eventDeployment)
	if err := engine.SendEvent(context.Background(), enumArrayOfContainerEvent{Contained: []enumArrayOfItem{
		{ID: "E1", P00: 1},
		{ID: "E2", P00: 9},
		{ID: "E2", P00: 2},
	}}); err != nil {
		t.Fatal(err)
	}
	if len(*eventRows) != 1 {
		t.Fatalf("event array-of rows = %d, want 1", len(*eventRows))
	}
	eventRow := (*eventRows)[0]
	if !eventRow.Get("c0").Equal(Present([]int64{1, 9, 2})) || !eventRow.Get("c1").Equal(Present([]int64{1, 19, 22})) || !eventRow.Get("c2").Equal(Present([]int64{301, 319, 322})) {
		t.Fatalf("event array-of row = %#v", eventRow.AsMap())
	}
}

// TestExprEnumArrayOfInvalidParity covers ExprArrayOfInvalid's observable
// builder contract: a missing collection or missing selector lambda is
// rejected at Build time. Java additionally rejects a null-typed lambda
// return with a compiler diagnostic; Go's typed expressions cannot carry an
// untyped null, so the missing-selector form is the Go-style equivalent.
func TestExprEnumArrayOfInvalidParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumArrayOfScalarEvent](env, "SupportCollectionInvalid"); err != nil {
		t.Fatal(err)
	}
	stream := From[enumArrayOfScalarEvent](env, "SupportCollectionInvalid")
	values := Field[enumArrayOfScalarEvent, []string]("strvals")
	invalid := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "missing-values", expr: EnumArrayOf[string](nil), want: "enumeration method \"array-of\" requires a collection expression"},
		{name: "missing-selector", expr: EnumArrayOfSelect[string, string](values, nil), want: "enumeration method \"array-of\" requires all selector expressions"},
	}
	for _, testCase := range invalid {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(stream, Alias("invalid", testCase.expr)).Query(StatementName("array-of-invalid-" + testCase.name)))
			if err == nil || !containsString(err.Error(), testCase.want) {
				t.Fatalf("Build error = %v, want fragment %q", err, testCase.want)
			}
		})
	}
}
