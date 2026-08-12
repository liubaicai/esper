package esper

import (
	"context"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

type relationalParityEvent struct {
	Text        string   `esper:"text"`
	IntValue    int      `esper:"int_value"`
	LongValue   *int64   `esper:"long_value"`
	FloatValue  float32  `esper:"float_value"`
	DoubleValue *float64 `esper:"double_value"`
	BigInteger  big.Int  `esper:"big_integer"`
	BigDecimal  big.Rat  `esper:"big_decimal"`
	Flag        bool     `esper:"flag"`
}

func TestMixedRelationalExpressionsMatchJavaNumericStringAndNullSemantics(t *testing.T) {
	if got := GreaterOf(Literal(2), Literal(int64(1))).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("mixed greater = %v, want true", got)
	}
	if got := GreaterOrEqualOf(Literal(int16(2)), Literal(2.0)).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("mixed greater-equal = %v, want true", got)
	}
	if got := LessOf(Literal(float32(1.5)), Literal(2)).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("mixed less = %v, want true", got)
	}
	if got := LessOrEqualOf(Literal("B"), Literal("B")).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("string less-equal = %v, want true", got)
	}
	if got := GreaterOf(Literal(exactRelationalInt("100000000000000000000")), Literal(exactRelationalInt("99999999999999999999"))).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("BigInteger greater = %v, want true", got)
	}
	if got := LessOf(Literal(exactRelationalRat("1/3")), Literal(exactRelationalRat("1/2"))).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("BigDecimal less = %v, want true", got)
	}
	if got := GreaterOf(Literal(1), NullLiteral[int64]()).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("null relational comparison = %v, want null", got)
	}
	var nilLong *int64
	if got := LessOrEqualOf(Literal(1), Literal(nilLong)).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("typed nil relational comparison = %v, want null", got)
	}
}

func TestMixedRelationalExpressionsBuildLiveProjectionAndPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[relationalParityEvent](env, "RelationalParityEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[relationalParityEvent](env, "RelationalParityEvent")
	textValue := Field[relationalParityEvent, string]("text")
	intValue := Field[relationalParityEvent, int]("int_value")
	longValue := Field[relationalParityEvent, *int64]("long_value")
	floatValue := Field[relationalParityEvent, float32]("float_value")
	doubleValue := Field[relationalParityEvent, *float64]("double_value")
	bigInteger := Field[relationalParityEvent, big.Int]("big_integer")
	bigDecimal := Field[relationalParityEvent, big.Rat]("big_decimal")
	query := Select(input,
		Alias("greater", GreaterOf(intValue, longValue)),
		Alias("greater_equal", GreaterOrEqualOf(longValue, Literal(int64(10)))),
		Alias("less", LessOf(floatValue, doubleValue)),
		Alias("less_equal", LessOrEqualOf(bigDecimal, Literal(exactRelationalRat("1/2")))),
		Alias("big_greater", GreaterOf(bigInteger, Literal(exactRelationalInt("9")))),
		Alias("text_less", LessOf(textValue, Literal("z"))),
	).Query(StatementName("relational-parity"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent relational plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	different, err := env.Build(Select(input,
		Alias("greater", LessOf(intValue, longValue)),
		Alias("greater_equal", GreaterOrEqualOf(longValue, Literal(int64(10)))),
		Alias("less", LessOf(floatValue, doubleValue)),
		Alias("less_equal", LessOrEqualOf(bigDecimal, Literal(exactRelationalRat("1/2")))),
		Alias("big_greater", GreaterOf(bigInteger, Literal(exactRelationalInt("9")))),
		Alias("text_less", LessOf(textValue, Literal("z"))),
	).Query(StatementName("relational-parity")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == different.Hash() {
		t.Fatal("relational operator did not enter Plan identity")
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("relational result schema is missing")
	}
	for _, fieldName := range []string{"greater", "greater_equal", "less", "less_equal", "big_greater", "text_less"} {
		field, ok := resultSchema.Field(fieldName)
		if !ok || field.Type != typeOf[bool]() {
			t.Fatalf("relational result %q = %#v, want bool", fieldName, field)
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
				return NewError(ErrorTypeMismatch, "relational result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	longNumber := int64(10)
	doubleNumber := 2.0
	if err := engine.SendEvent(context.Background(), relationalParityEvent{
		Text: "a", IntValue: 11, LongValue: &longNumber, FloatValue: 1.5, DoubleValue: &doubleNumber,
		BigInteger: exactRelationalInt("10"), BigDecimal: exactRelationalRat("1/3"),
	}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("relational rows = %d, want 1", len(rows))
	}
	want := map[string]Value{
		"greater": Present(true), "greater_equal": Present(true), "less": Present(true),
		"less_equal": Present(true), "big_greater": Present(true), "text_less": Present(true),
	}
	for name, expected := range want {
		if !rows[0].Get(name).Equal(expected) {
			t.Fatalf("relational row %s = %v, want %v", name, rows[0].Get(name), expected)
		}
	}
	if err := engine.SendEvent(context.Background(), relationalParityEvent{Text: "a", IntValue: 11, LongValue: nil, FloatValue: 1.5, DoubleValue: &doubleNumber, BigInteger: exactRelationalInt("10"), BigDecimal: exactRelationalRat("1/3")}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !rows[1].Get("greater").IsNull() || !rows[1].Get("greater_equal").IsNull() {
		t.Fatalf("relational null row = %#v", rows)
	}
}

func TestMixedRelationalExpressionsRejectInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[relationalParityEvent](env, "RelationalInvalid"); err != nil {
		t.Fatal(err)
	}
	input := From[relationalParityEvent](env, "RelationalInvalid")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "nil-left", expr: GreaterOf(nil, Literal(1)), want: "left operand"},
		{name: "nil-right", expr: LessOf(Literal(1), nil), want: "right operand"},
		{name: "bool", expr: GreaterOf(Literal(true), Literal(false)), want: "must be ordered"},
		{name: "slice", expr: LessOrEqualOf(Literal([]int{1}), Literal([]int{2})), want: "must be ordered"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("relational-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid relational error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func exactRelationalInt(text string) big.Int {
	var result big.Int
	if _, ok := result.SetString(text, 10); !ok {
		panic("invalid relational integer " + text)
	}
	return result
}

func exactRelationalRat(text string) big.Rat {
	var result big.Rat
	if _, ok := result.SetString(text); !ok {
		panic("invalid relational rational " + text)
	}
	return result
}
