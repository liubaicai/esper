package esper

import (
	"math/big"
	"strings"
	"testing"
)

func TestBigNumberScalarExpressionsMatchJavaSupportMatrix(t *testing.T) {
	integerTen := exactBigNumberInt("10")
	integerTwo := exactBigNumberInt("2")
	decimalTen := exactBigNumberRat("10")
	decimalTwo := exactBigNumberRat("2")
	if got := MinExactOf[big.Int](Literal(integerTen), Literal(int64(2))).eval(EvalContext{}); !got.Equal(Present(integerTwo)) {
		t.Fatalf("BigInteger scalar min = %v, want 2", got)
	}
	if got := MaxExactOf[big.Rat](Literal(decimalTen), Literal(int64(20))).eval(EvalContext{}); !got.Equal(Present(exactBigNumberRat("20"))) {
		t.Fatalf("BigDecimal scalar max = %v, want 20", got)
	}
	if got := EqualOf(Literal(integerTen), Literal(int64(10))).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("BigInteger equality = %v, want true", got)
	}
	if got := LessOrEqualOf(Literal(decimalTwo), Literal(2.0)).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("BigDecimal relational comparison = %v, want true", got)
	}
	if got := BetweenOf(Literal(decimalTen), Literal(9), Literal(11)).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("BigDecimal between = %v, want true", got)
	}
	if got := InOf(Literal(integerTwo), Literal(1), Literal(integerTwo), Literal(3)).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("BigInteger in = %v, want true", got)
	}
}

func TestBigNumberExactScalarMinMaxRejectInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[mathParityEvent](env, "BigNumberInvalidEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[mathParityEvent](env, "BigNumberInvalidEvent")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "too-few", expr: MinExactOf[big.Int](Literal(exactBigNumberInt("1"))), want: "at least two"},
		{name: "nil", expr: MaxExactOf[big.Rat](Literal(exactBigNumberRat("1")), nil), want: "operand 1 is required"},
		{name: "non-numeric", expr: MinExactOf[big.Int](Literal(exactBigNumberInt("1")), Literal("x")), want: "must be numeric"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("big-number-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid BigNumber error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func exactBigNumberInt(text string) big.Int {
	var result big.Int
	if _, ok := result.SetString(text, 10); !ok {
		panic("invalid BigInteger " + text)
	}
	return result
}

func exactBigNumberRat(text string) big.Rat {
	var result big.Rat
	if _, ok := result.SetString(text); !ok {
		panic("invalid BigDecimal " + text)
	}
	return result
}
