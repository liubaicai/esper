package esper

import (
	"context"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

func TestDecimalMathContextMatchesJavaDecimal32Division(t *testing.T) {
	context32 := MathContextDECIMAL32
	if got := DivideExactWithContext(Literal(exactDecimalRat("16/10")), Literal(exactDecimalRat("92/10")), context32).eval(EvalContext{}); !got.Equal(Present(exactDecimalRat("173913/1000000"))) {
		t.Fatalf("DECIMAL32 division = %v, want 0.1739130", got)
	}
	if got := DivideExactWithContext(Literal(exactDecimalRat("10")), Literal(exactDecimalRat("5")), context32).eval(EvalContext{}); !got.Equal(Present(exactDecimalRat("2"))) {
		t.Fatalf("exact DECIMAL32 division = %v, want 2", got)
	}
	if got := AddExactWithContext(Literal(exactDecimalRat("99999995")), Literal(exactDecimalRat("1")), DecimalMathContext{Precision: 7, Rounding: DecimalRoundHalfEven}).eval(EvalContext{}); !got.Equal(Present(exactDecimalRat("100000000"))) {
		t.Fatalf("rounded decimal addition = %v, want 100000000", got)
	}
	if got := RoundDecimal(Literal(exactDecimalRat("1/8")), DecimalMathContext{Precision: 2, Rounding: DecimalRoundHalfUp}).eval(EvalContext{}); !got.Equal(Present(exactDecimalRat("13/100"))) {
		t.Fatalf("rounded decimal = %v, want 0.13", got)
	}
}

func TestDecimalMathContextSupportsRoundingModesAndExactNumbers(t *testing.T) {
	value := Literal(exactDecimalRat("-125/100"))
	cases := []struct {
		name string
		mode DecimalRoundingMode
		want string
	}{
		{name: "half-even", mode: DecimalRoundHalfEven, want: "-12/10"},
		{name: "half-up", mode: DecimalRoundHalfUp, want: "-13/10"},
		{name: "half-down", mode: DecimalRoundHalfDown, want: "-12/10"},
		{name: "up", mode: DecimalRoundUp, want: "-13/10"},
		{name: "down", mode: DecimalRoundDown, want: "-12/10"},
		{name: "ceiling", mode: DecimalRoundCeiling, want: "-12/10"},
		{name: "floor", mode: DecimalRoundFloor, want: "-13/10"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := RoundDecimal(value, DecimalMathContext{Precision: 2, Rounding: testCase.mode}).eval(EvalContext{})
			if !got.Equal(Present(exactDecimalRat(testCase.want))) {
				t.Fatalf("rounding mode %s = %v, want %s", testCase.name, got, testCase.want)
			}
		})
	}
	if got := RoundDecimal(Literal(exactDecimalRat("1/3")), DecimalMathContext{Precision: 2, Rounding: DecimalRoundUnnecessary}).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("unnecessary rounding = %v, want null", got)
	}
	if got := AddExactWithContext(Literal(exactDecimalRat("9007199254740993")), Literal(exactDecimalRat("1")), DecimalMathContext{}).eval(EvalContext{}); !got.Equal(Present(exactDecimalRat("9007199254740994"))) {
		t.Fatalf("unlimited exact addition = %v", got)
	}
}

type decimalMathContextEvent struct {
	Numerator   big.Rat `esper:"numerator"`
	Denominator big.Rat `esper:"denominator"`
}

func TestDecimalMathContextBuildsTypedLiveProjectionAndPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[decimalMathContextEvent](env, "DecimalMathContextEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[decimalMathContextEvent](env, "DecimalMathContextEvent")
	numerator := Field[decimalMathContextEvent, big.Rat]("numerator")
	denominator := Field[decimalMathContextEvent, big.Rat]("denominator")
	query := Select(input, Alias("value", DivideExactWithContext(numerator, denominator, MathContextDECIMAL32))).Query(StatementName("decimal-context"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	other, err := env.Build(Select(input, Alias("value", DivideExactWithContext(numerator, denominator, DecimalMathContext{Precision: 7, Rounding: DecimalRoundHalfUp}))).Query(StatementName("decimal-context")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == other.Hash() || reflect.DeepEqual(plan.Canonical(), other.Canonical()) {
		t.Fatal("decimal rounding mode did not enter Plan identity")
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("decimal result schema is missing")
	}
	field, ok := resultSchema.Field("value")
	if !ok || field.Type != reflect.TypeOf(big.Rat{}) {
		t.Fatalf("decimal result type = %#v, want big.Rat", field)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 1)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "decimal result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), decimalMathContextEvent{Numerator: exactDecimalRat("16/10"), Denominator: exactDecimalRat("92/10")}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("decimal rows = %d, want 1", len(rows))
	}
	if got := rows[0].Get("value"); !got.Equal(Present(exactDecimalRat("173913/1000000"))) {
		t.Fatalf("decimal projected value = %v", got)
	}
}

func TestDecimalMathContextRejectsInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[decimalMathContextEvent](env, "DecimalMathInvalidEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[decimalMathContextEvent](env, "DecimalMathInvalidEvent")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "negative-precision", expr: RoundDecimal(Literal(exactDecimalRat("1/2")), DecimalMathContext{Precision: -1}), want: "precision must not be negative"},
		{name: "invalid-rounding", expr: RoundDecimal(Literal(exactDecimalRat("1/2")), DecimalMathContext{Precision: 2, Rounding: DecimalRoundingMode(99)}), want: "invalid rounding mode"},
		{name: "non-numeric", expr: RoundDecimal(Literal("1"), MathContextDECIMAL32), want: "must be numeric"},
		{name: "nil-value", expr: RoundDecimal(nil, MathContextDECIMAL32), want: "requires a value expression"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("decimal-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid decimal error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func exactDecimalRat(text string) big.Rat {
	var result big.Rat
	if _, ok := result.SetString(text); !ok {
		panic("invalid exact decimal " + text)
	}
	return result
}
