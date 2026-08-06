package esper

import (
	"context"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

type mathParityEvent struct {
	IntPrimitive   int     `esper:"int_primitive"`
	IntBoxed       *int    `esper:"int_boxed"`
	LongPrimitive  int64   `esper:"long_primitive"`
	ShortPrimitive int16   `esper:"short_primitive"`
	BytePrimitive  int8    `esper:"byte_primitive"`
	FloatPrimitive float32 `esper:"float_primitive"`
	BigInteger     big.Int `esper:"big_integer"`
	BigDecimal     big.Rat `esper:"big_decimal"`
}

func TestMathExpressionsMatchJavaPromotionAndDivisionPolicies(t *testing.T) {
	if got := AddOf[int](Literal(int16(5)), Literal(int8(6))).eval(EvalContext{}); !got.Equal(Present(11)) {
		t.Fatalf("short/byte add = %v, want 11", got)
	}
	if got := SubtractOf[int](Literal(int16(5)), Literal(int8(6))).eval(EvalContext{}); !got.Equal(Present(-1)) {
		t.Fatalf("short/byte subtract = %v, want -1", got)
	}
	if got := MultiplyOf[int](Literal(int16(5)), Literal(int8(6))).eval(EvalContext{}); !got.Equal(Present(30)) {
		t.Fatalf("short/byte multiply = %v, want 30", got)
	}
	if got := ModuloOf[int64](Literal(int64(5)), Literal(int8(3))).eval(EvalContext{}); !got.Equal(Present(int64(2))) {
		t.Fatalf("mixed modulo = %v, want 2", got)
	}
	if got := DivideFloat(Literal(int64(10)), Literal(int64(3))).eval(EvalContext{}); !got.Equal(Present(10.0 / 3.0)) {
		t.Fatalf("long division = %v, want %v", got, 10.0/3.0)
	}
	if got := DivideWithOptions[int](Literal(10), Literal(3), WithIntegerDivision(true), WithDivisionByZeroReturnsNull(true)).eval(EvalContext{}); !got.Equal(Present(3)) {
		t.Fatalf("integer division = %v, want 3", got)
	}
	if got := DivideWithOptions[float64](Literal(10), Literal(3), WithIntegerDivision(true)).eval(EvalContext{}); !got.Equal(Present(float64(3))) {
		t.Fatalf("integer division with float result = %v, want 3", got)
	}
	if got := DivideWithOptions[float64](Literal(10), Literal(3), WithDivisionByZeroReturnsNull(true)).eval(EvalContext{}); !got.Equal(Present(10.0 / 3.0)) {
		t.Fatalf("configured floating division = %v, want %v", got, 10.0/3.0)
	}
	if got := DivideWithOptions[float64](Literal(10), Literal(0), WithDivisionByZeroReturnsNull(true)).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("configured divide by zero = %v, want null", got)
	}
	if got := DivideFloat(Literal(10), Literal(0)).eval(EvalContext{}); !got.IsPresent() || !math.IsInf(got.Any().(float64), 1) {
		t.Fatalf("default divide by zero = %v, want +Inf", got)
	}
	if got := DivideFloat(Literal(0), Literal(0)).eval(EvalContext{}); !got.IsPresent() || !math.IsNaN(got.Any().(float64)) {
		t.Fatalf("zero divided by zero = %v, want NaN", got)
	}

	env := NewEnvironment()
	defaultPlan, err := env.Build(SelectOnce(env, Alias("value", DivideFloat(Literal(10), Literal(3)))))
	if err != nil {
		t.Fatal(err)
	}
	nullPlan, err := env.Build(SelectOnce(env, Alias("value", DivideFloat(Literal(10), Literal(3), WithDivisionByZeroReturnsNull(true)))))
	if err != nil {
		t.Fatal(err)
	}
	if defaultPlan.Hash() == nullPlan.Hash() {
		t.Fatal("division-by-zero policy did not enter Plan identity")
	}

	var nilInt *int
	if got := AddOf[int](Literal(nilInt), Literal(1)).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("typed nil arithmetic = %v, want null", got)
	}
	if got := AddOf[int](NullLiteral[int](), Literal(1)).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("null arithmetic = %v, want null", got)
	}
}

func TestExactMathExpressionsPreserveBigIntegerAndDecimalValues(t *testing.T) {
	got, err := As[big.Int](AddExact[big.Int](Literal(exactMathInt("10")), Literal(int64(5))).eval(EvalContext{}))
	if err != nil {
		t.Fatal(err)
	}
	if got.Cmp(big.NewInt(15)) != 0 {
		t.Fatalf("big integer add = %s, want 15", got.String())
	}
	multiplied, err := As[big.Int](MultiplyExact[big.Int](Literal(exactMathInt("123456789012345678901234567890")), Literal(exactMathInt("9"))).eval(EvalContext{}))
	if err != nil {
		t.Fatal(err)
	}
	if multiplied.String() != "1111111101111111110111111111010" {
		t.Fatalf("big integer multiply = %s", multiplied.String())
	}
	if got := DivideExact[big.Int](Literal(exactMathInt("10")), Literal(exactMathInt("3"))).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("non-integral big integer division = %v, want null", got)
	}

	decimalDivision := DivideExact[big.Rat](Literal(exactMathRat("16/10")), Literal(exactMathRat("92/10"))).eval(EvalContext{})
	decimal, err := As[big.Rat](decimalDivision)
	if err != nil {
		t.Fatal(err)
	}
	want := exactMathRat("4/23")
	if !decimalDivision.IsPresent() || decimal.Cmp(&want) != 0 {
		t.Fatalf("exact decimal division = %s, want %s", decimal.RatString(), want.RatString())
	}
	if got, err := As[big.Rat](AddExact[big.Rat](Literal(exactMathInt("10")), Literal(exactMathRat("5"))).eval(EvalContext{})); err != nil || got.Cmp(big.NewRat(15, 1)) != 0 {
		t.Fatalf("mixed BigInteger/BigDecimal add = %s, err=%v", got.RatString(), err)
	}
}

func TestMathExpressionsBuildTypedPlanAndLiveProjection(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[mathParityEvent](env, "MathParityEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[mathParityEvent](env, "MathParityEvent")
	intPrimitive := Field[mathParityEvent, int]("int_primitive")
	intBoxed := Field[mathParityEvent, *int]("int_boxed")
	longPrimitive := Field[mathParityEvent, int64]("long_primitive")
	shortPrimitive := Field[mathParityEvent, int16]("short_primitive")
	bytePrimitive := Field[mathParityEvent, int8]("byte_primitive")
	floatPrimitive := Field[mathParityEvent, float32]("float_primitive")
	bigInteger := Field[mathParityEvent, big.Int]("big_integer")
	bigDecimal := Field[mathParityEvent, big.Rat]("big_decimal")
	query := Select(input,
		Alias("short_byte_add", AddOf[int](shortPrimitive, bytePrimitive)),
		Alias("long_division", DivideFloat(longPrimitive, Literal(int64(2)))),
		Alias("integer_division", DivideWithOptions[int](intPrimitive, intBoxed, WithIntegerDivision(true), WithDivisionByZeroReturnsNull(true))),
		Alias("float_product", MultiplyOf[float32](floatPrimitive, Literal(float32(2)))),
		Alias("big_integer_sum", AddExact[big.Int](bigInteger, Literal(int64(5)))),
		Alias("big_decimal_sum", AddExact[big.Rat](bigDecimal, Literal(int64(2)))),
		Alias("big_integer_division", DivideExact[big.Int](bigInteger, Literal(exactMathInt("2")))),
	).Query(StatementName("math-parity"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent math plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("math result schema is missing")
	}
	wantTypes := map[string]reflect.Type{
		"short_byte_add":       typeOf[int](),
		"long_division":        typeOf[float64](),
		"integer_division":     typeOf[int](),
		"float_product":        typeOf[float32](),
		"big_integer_sum":      typeOf[big.Int](),
		"big_decimal_sum":      typeOf[big.Rat](),
		"big_integer_division": typeOf[big.Int](),
	}
	for name, wantType := range wantTypes {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != wantType {
			t.Fatalf("math result %q type = %#v, want %v", name, field, wantType)
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
				return NewError(ErrorTypeMismatch, "math result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	boxed := 3
	if err := engine.SendEvent(context.Background(), mathParityEvent{
		IntPrimitive:   100,
		IntBoxed:       &boxed,
		LongPrimitive:  10,
		ShortPrimitive: 5,
		BytePrimitive:  4,
		FloatPrimitive: 1.5,
		BigInteger:     exactMathInt("10"),
		BigDecimal:     exactMathRat("7/2"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), mathParityEvent{
		IntPrimitive:   100,
		LongPrimitive:  10,
		ShortPrimitive: 5,
		BytePrimitive:  4,
		FloatPrimitive: 1.5,
		BigInteger:     exactMathInt("10"),
		BigDecimal:     exactMathRat("7/2"),
	}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("math rows = %d, want 2", len(rows))
	}
	if !rows[0].Get("short_byte_add").Equal(Present(9)) ||
		!rows[0].Get("long_division").Equal(Present(float64(5))) ||
		!rows[0].Get("integer_division").Equal(Present(33)) ||
		!rows[0].Get("float_product").Equal(Present(float32(3))) {
		t.Fatalf("native math row = %#v", rows[0])
	}
	assertMathBigInt(t, rows[0].Get("big_integer_sum"), "15")
	assertMathBigRat(t, rows[0].Get("big_decimal_sum"), "11/2")
	assertMathBigInt(t, rows[0].Get("big_integer_division"), "5")
	if !rows[1].Get("integer_division").IsNull() {
		t.Fatalf("nil boxed integer division = %v, want null", rows[1].Get("integer_division"))
	}

	invalid := Select(input, Alias("bad", AddOf[int](Literal("text"), Literal(1)))).Query(StatementName("bad-math"))
	if _, err := env.Build(invalid); err == nil || !strings.Contains(err.Error(), "must be numeric") {
		t.Fatalf("invalid math builder error = %v", err)
	}
	invalidNil := Select(input, Alias("bad", AddOf[int](nil, Literal(1)))).Query(StatementName("bad-math-nil"))
	if _, err := env.Build(invalidNil); err == nil || !strings.Contains(err.Error(), "requires two operands") {
		t.Fatalf("nil math builder error = %v", err)
	}
	nestedInvalid := Select(input, Alias("bad", AddOf[int](Literal(1), AddOf[int](nil, Literal(2))))).Query(StatementName("bad-math-nested"))
	if _, err := env.Build(nestedInvalid); err == nil || !strings.Contains(err.Error(), "requires two operands") {
		t.Fatalf("nested nil math builder error = %v", err)
	}
}

func exactMathInt(text string) big.Int {
	var result big.Int
	if _, ok := result.SetString(text, 10); !ok {
		panic("invalid exact integer " + text)
	}
	return result
}

func exactMathRat(text string) big.Rat {
	var result big.Rat
	if _, ok := result.SetString(text); !ok {
		panic("invalid exact rational " + text)
	}
	return result
}

func assertMathBigInt(t *testing.T, value Value, text string) {
	t.Helper()
	actual, err := As[big.Int](value)
	if err != nil {
		t.Fatal(err)
	}
	want := exactMathInt(text)
	if !value.IsPresent() || actual.Cmp(&want) != 0 {
		t.Fatalf("big integer = %s, want %s", actual.String(), want.String())
	}
}

func assertMathBigRat(t *testing.T, value Value, text string) {
	t.Helper()
	actual, err := As[big.Rat](value)
	if err != nil {
		t.Fatal(err)
	}
	want := exactMathRat(text)
	if !value.IsPresent() || actual.Cmp(&want) != 0 {
		t.Fatalf("big rational = %s, want %s", actual.RatString(), want.RatString())
	}
}
