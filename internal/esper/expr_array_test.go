package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type arrayExpressionParityEvent struct {
	Index    int      `esper:"index"`
	Numbers  []int    `esper:"numbers"`
	Fixed    [3]int   `esper:"fixed"`
	Nullable []string `esper:"nullable"`
}

func TestArrayExpressionsMatchJavaLiteralAndIndexedAccessSemantics(t *testing.T) {
	if got := ArrayOf[int64](Literal(int(1)), Literal(int16(2))).eval(EvalContext{}); !got.Equal(Present([]int64{1, 2})) {
		t.Fatalf("mixed array literal = %v, want [1 2]", got)
	}
	if got := ArrayLiteral[string](Literal("a"), Literal("b")).eval(EvalContext{}); !got.Equal(Present([]string{"a", "b"})) {
		t.Fatalf("string array literal = %v, want [a b]", got)
	}
	dynamic := ArrayOf[any](Literal("a"), Literal(1), NullLiteral[int]()).eval(EvalContext{})
	dynamicItems, dynamicErr := As[[]any](dynamic)
	if dynamicErr != nil || len(dynamicItems) != 3 || dynamicItems[0] != "a" || dynamicItems[1] != 1 || dynamicItems[2] != nil {
		t.Fatalf("dynamic array literal = %v, items=%#v, err=%v", dynamic, dynamicItems, dynamicErr)
	}
	if got := ArraySize(Literal([3]int{1, 2, 3})).eval(EvalContext{}); !got.Equal(Present(int64(3))) {
		t.Fatalf("fixed array size = %v, want 3", got)
	}
	if got := ArrayLength(Literal([]string{"a", "b"})).eval(EvalContext{}); !got.Equal(Present(int64(2))) {
		t.Fatalf("slice array length = %v, want 2", got)
	}
	if got := ArrayAt[int](Literal([3]int{4, 5, 6}), Literal(int8(1))).eval(EvalContext{}); !got.Equal(Present(5)) {
		t.Fatalf("fixed array access = %v, want 5", got)
	}
	nested := ArrayOf[[]int](Literal([]int{1, 2}), Literal([]int{3, 4}))
	if got := ArrayAt[[]int](nested, Literal(int64(1))).eval(EvalContext{}); !got.Equal(Present([]int{3, 4})) {
		t.Fatalf("nested array access = %v, want [3 4]", got)
	}
	if got := ArrayElementAt[string](Literal([]string{"a", "b"}), Literal(int64(1))).eval(EvalContext{}); !got.Equal(Present("b")) {
		t.Fatalf("slice array access = %v, want b", got)
	}
	if got := ArrayAt[int](Literal([]int{1}), Literal(int64(-1))).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("negative array access = %v, want null", got)
	}
	if got := ArrayAt[int](Literal([]int{1}), Literal(int64(2))).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("out-of-range array access = %v, want null", got)
	}
	if got := ArraySize(NullLiteral[[]int]()).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("null array size = %v, want null", got)
	}
}

func TestArrayExpressionsBuildTypedLiveProjectionAndPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[arrayExpressionParityEvent](env, "ArrayExpressionParityEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[arrayExpressionParityEvent](env, "ArrayExpressionParityEvent")
	numbers := Field[arrayExpressionParityEvent, []int]("numbers")
	fixed := Field[arrayExpressionParityEvent, [3]int]("fixed")
	index := Field[arrayExpressionParityEvent, int]("index")
	query := Select(input,
		Alias("literal", ArrayOf[int64](Literal(1), Literal(int16(2)))),
		Alias("size", ArraySize(numbers)),
		Alias("indexed", ArrayAt[int](numbers, index)),
		Alias("fixed_indexed", ArrayAt[int](fixed, index)),
		Alias("missing", ArrayAt[int](numbers, Literal(int64(99)))),
	).Query(StatementName("array-expression-parity"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent array plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	different, err := env.Build(Select(input,
		Alias("literal", ArrayOf[int64](Literal(2), Literal(int16(1)))),
		Alias("size", ArraySize(numbers)),
		Alias("indexed", ArrayAt[int](numbers, index)),
		Alias("fixed_indexed", ArrayAt[int](fixed, index)),
		Alias("missing", ArrayAt[int](numbers, Literal(int64(99)))),
	).Query(StatementName("array-expression-parity")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == different.Hash() {
		t.Fatal("array literal order did not enter Plan identity")
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("array result schema is missing")
	}
	wantTypes := map[string]reflect.Type{
		"literal":       typeOf[[]int64](),
		"size":          typeOf[int64](),
		"indexed":       typeOf[int](),
		"fixed_indexed": typeOf[int](),
		"missing":       typeOf[int](),
	}
	for name, wantType := range wantTypes {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != wantType {
			t.Fatalf("array result %q = %#v, want %v", name, field, wantType)
		}
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 2)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "array result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), arrayExpressionParityEvent{Index: 1, Numbers: []int{10, 20}, Fixed: [3]int{30, 40, 50}}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), arrayExpressionParityEvent{Index: 2, Numbers: []int{10}, Fixed: [3]int{30, 40, 50}}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("array rows = %d, want 2", len(rows))
	}
	if !rows[0].Get("literal").Equal(Present([]int64{1, 2})) ||
		!rows[0].Get("size").Equal(Present(int64(2))) ||
		!rows[0].Get("indexed").Equal(Present(20)) ||
		!rows[0].Get("fixed_indexed").Equal(Present(40)) ||
		!rows[0].Get("missing").IsNull() {
		t.Fatalf("array first row = %#v", rows[0])
	}
	if !rows[1].Get("size").Equal(Present(int64(1))) || !rows[1].Get("indexed").IsNull() || !rows[1].Get("fixed_indexed").Equal(Present(50)) {
		t.Fatalf("array second row = %#v", rows[1])
	}
}

func TestArrayExpressionsRejectInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[arrayExpressionParityEvent](env, "ArrayExpressionInvalid"); err != nil {
		t.Fatal(err)
	}
	input := From[arrayExpressionParityEvent](env, "ArrayExpressionInvalid")
	numbers := Field[arrayExpressionParityEvent, []int]("numbers")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "literal-type", expr: ArrayOf[int](Literal("text")), want: "not assignable"},
		{name: "literal-nil", expr: ArrayOf[int](nil), want: "element 0 is required"},
		{name: "size-scalar", expr: ArraySize(Literal(1)), want: "array or slice"},
		{name: "access-scalar", expr: ArrayAt[int](Literal(1), Literal(0)), want: "array or slice"},
		{name: "access-index", expr: ArrayAt[int](numbers, Literal("bad")), want: "index must be an integer"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("array-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid array error = %v, want %q", err, testCase.want)
			}
		})
	}
}
