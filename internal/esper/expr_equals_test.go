package esper

import (
	"context"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

func TestEqualityExpressionsMatchJavaCoercionArrayAndNullSemantics(t *testing.T) {
	if got := EqualOf(Literal(1), Literal(int64(1))).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("mixed equality = %v, want true", got)
	}
	if got := NotEqualOf(Literal(1), Literal(int64(2))).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("mixed inequality = %v, want true", got)
	}
	if got := EqualOf(Literal(exactEqualsInt("2")), Literal(2)).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("BigInteger equality = %v, want true", got)
	}
	if got := EqualOf(Literal([]int{1, 2}), Literal([]int{1, 2})).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("array equality = %v, want true", got)
	}
	if got := Is(Literal([]int{1, 2}), Literal([]int{1, 2})).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("array IS = %v, want true", got)
	}
	if got := Is(NullLiteral[string](), NullLiteral[int]()).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("null IS = %v, want true", got)
	}
	if got := IsNot(NullLiteral[string](), Literal(1)).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("null IS NOT present = %v, want true", got)
	}
	if got := IsNot(NullLiteral[string](), NullLiteral[int]()).eval(EvalContext{}); !got.Equal(Present(false)) {
		t.Fatalf("null IS NOT null = %v, want false", got)
	}
	if got := EqualOf(NullLiteral[string](), Literal("x")).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("null equality = %v, want null", got)
	}

	env := NewEnvironment()
	if _, err := RegisterStruct[equalityParityEvent](env, "EqualityNullEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[equalityParityEvent](env, "EqualityNullEvent")
	if _, err := env.Build(Select(input,
		Alias("null_is_number", Is(NullLiteral[string](), Literal(1))),
		Alias("number_is_null", Is(Literal(1), NullLiteral[string]())),
		Alias("null_equals_number", EqualOf(NullLiteral[string](), Literal(1))),
	).Query(StatementName("equality-null-cross-type"))); err != nil {
		t.Fatalf("null cross-type equality should build: %v", err)
	}
	var nilNumber *int64
	if got := EqualOf(Literal(nilNumber), Literal(int64(1))).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("typed-nil equality = %v, want null", got)
	}
	if got := Is(Literal(nilNumber), NullLiteral[int]()).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("typed-nil IS null = %v, want true", got)
	}
}

type equalityParityEvent struct {
	IntValue   int     `esper:"int_value"`
	LongValue  *int64  `esper:"long_value"`
	LeftArray  []int   `esper:"left_array"`
	RightArray []int   `esper:"right_array"`
	Text       string  `esper:"text"`
	Flag       bool    `esper:"flag"`
	BigValue   big.Int `esper:"big_value"`
}

func TestEqualityExpressionsBuildLiveProjectionAndPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[equalityParityEvent](env, "EqualityParityEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[equalityParityEvent](env, "EqualityParityEvent")
	intValue := Field[equalityParityEvent, int]("int_value")
	longValue := Field[equalityParityEvent, *int64]("long_value")
	leftArray := Field[equalityParityEvent, []int]("left_array")
	rightArray := Field[equalityParityEvent, []int]("right_array")
	textValue := Field[equalityParityEvent, string]("text")
	flag := Field[equalityParityEvent, bool]("flag")
	bigValue := Field[equalityParityEvent, big.Int]("big_value")
	query := Select(input,
		Alias("equal", EqualOf(intValue, longValue)),
		Alias("not_equal", NotEqualOf(intValue, longValue)),
		Alias("array_equal", Is(leftArray, rightArray)),
		Alias("null_is", Is(NullLiteral[string](), NullLiteral[int]())),
		Alias("text_is", Is(textValue, Literal("A"))),
		Alias("flag_is_not", IsNot(flag, Literal(true))),
		Alias("big_equal", EqualOf(bigValue, Literal(2))),
	).Query(StatementName("equality-parity"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent equality plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	different, err := env.Build(Select(input,
		Alias("equal", EqualOf(intValue, longValue)),
		Alias("not_equal", NotEqualOf(intValue, longValue)),
		Alias("array_equal", Is(leftArray, rightArray)),
		Alias("text_is", Is(textValue, Literal("B"))),
		Alias("flag_is_not", IsNot(flag, Literal(false))),
		Alias("big_equal", EqualOf(bigValue, Literal(2))),
	).Query(StatementName("equality-parity")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == different.Hash() {
		t.Fatal("equality literal did not enter Plan identity")
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("equality result schema is missing")
	}
	for _, name := range []string{"equal", "not_equal", "array_equal", "null_is", "text_is", "flag_is_not", "big_equal"} {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != typeOf[bool]() {
			t.Fatalf("equality result field %q = %#v, want bool", name, field)
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
				return NewError(ErrorTypeMismatch, "equality result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	longNumber := int64(2)
	if err := engine.SendEvent(context.Background(), equalityParityEvent{IntValue: 2, LongValue: &longNumber, LeftArray: []int{1, 2}, RightArray: []int{1, 2}, Text: "A", Flag: false, BigValue: exactEqualsInt("2")}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), equalityParityEvent{IntValue: 2, LongValue: nil, LeftArray: []int{1}, RightArray: []int{2}, Text: "B", Flag: true, BigValue: exactEqualsInt("3")}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("equality rows = %d, want 2", len(rows))
	}
	if !rows[0].Get("equal").Equal(Present(true)) || !rows[0].Get("not_equal").Equal(Present(false)) || !rows[0].Get("array_equal").Equal(Present(true)) || !rows[0].Get("text_is").Equal(Present(true)) || !rows[0].Get("flag_is_not").Equal(Present(true)) || !rows[0].Get("big_equal").Equal(Present(true)) {
		t.Fatalf("first equality row = %#v", rows[0].AsMap())
	}
	if !rows[0].Get("null_is").Equal(Present(true)) {
		t.Fatalf("first equality null-is = %v, want true", rows[0].Get("null_is"))
	}
	if !rows[1].Get("equal").IsNull() || !rows[1].Get("not_equal").IsNull() || !rows[1].Get("array_equal").Equal(Present(false)) || !rows[1].Get("text_is").Equal(Present(false)) || !rows[1].Get("flag_is_not").Equal(Present(false)) || !rows[1].Get("big_equal").Equal(Present(false)) {
		t.Fatalf("second equality row = %#v", rows[1].AsMap())
	}
}

func TestEqualityExpressionsRejectInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[equalityParityEvent](env, "EqualityInvalidEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[equalityParityEvent](env, "EqualityInvalidEvent")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "bool-number", expr: EqualOf(Literal(true), Literal(1)), want: "not compatible"},
		{name: "slice-number", expr: Is(Literal([]int{1}), Literal(1)), want: "not compatible"},
		{name: "different-array-elements", expr: Is(Literal([]int{1}), Literal([]int64{1})), want: "not compatible"},
		{name: "interface-array-elements", expr: Is(Literal([]any{1}), Literal([]bool{true})), want: "not compatible"},
		{name: "nil-left", expr: Is(nil, Literal(1)), want: "left operand"},
		{name: "nil-right", expr: Is(Literal(1), nil), want: "right operand"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("equality-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid equality error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func exactEqualsInt(text string) big.Int {
	var result big.Int
	if _, ok := result.SetString(text, 10); !ok {
		panic("invalid equality integer " + text)
	}
	return result
}
