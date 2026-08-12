package esper

import (
	"fmt"
	"math/big"
)

// DecimalRoundingMode is the decimal rounding policy used by a
// DecimalMathContext. The names mirror java.math.RoundingMode while keeping
// the public API Go-native and independent of the Java runtime.
type DecimalRoundingMode uint8

const (
	DecimalRoundHalfEven DecimalRoundingMode = iota
	DecimalRoundHalfUp
	DecimalRoundHalfDown
	DecimalRoundUp
	DecimalRoundDown
	DecimalRoundCeiling
	DecimalRoundFloor
	DecimalRoundUnnecessary
)

func (mode DecimalRoundingMode) String() string {
	switch mode {
	case DecimalRoundHalfEven:
		return "half-even"
	case DecimalRoundHalfUp:
		return "half-up"
	case DecimalRoundHalfDown:
		return "half-down"
	case DecimalRoundUp:
		return "up"
	case DecimalRoundDown:
		return "down"
	case DecimalRoundCeiling:
		return "ceiling"
	case DecimalRoundFloor:
		return "floor"
	case DecimalRoundUnnecessary:
		return "unnecessary"
	default:
		return "invalid"
	}
}

// DecimalMathContext controls significant-digit rounding for exact decimal
// expressions. Precision zero means unlimited precision, like
// java.math.MathContext.UNLIMITED. big.Rat is used as the Go representation
// of BigDecimal, so decimal trailing zeroes are represented by the exact
// rational value rather than a separate scale field.
type DecimalMathContext struct {
	Precision int
	Rounding  DecimalRoundingMode
}

// MathContextDECIMAL32 is the Java MathContext.DECIMAL32 equivalent:
// seven significant digits with HALF_EVEN rounding.
var MathContextDECIMAL32 = DecimalMathContext{Precision: 7, Rounding: DecimalRoundHalfEven}

func (context DecimalMathContext) description() string {
	return fmt.Sprintf("precision=%d,rounding=%s", context.Precision, context.Rounding)
}

func (context DecimalMathContext) validate() error {
	if context.Precision < 0 {
		return fmt.Errorf("decimal math context precision must not be negative")
	}
	if context.Rounding > DecimalRoundUnnecessary {
		return fmt.Errorf("decimal math context has an invalid rounding mode %d", context.Rounding)
	}
	return nil
}

// AddExactWithContext rounds an exact decimal result using context.
func AddExactWithContext(left, right Expr, context DecimalMathContext) Expression[big.Rat] {
	return decimalMathExpression("add-decimal", "+", left, right, context, func(l, r *big.Rat) (*big.Rat, bool) {
		return new(big.Rat).Add(l, r), true
	})
}

// SubtractExactWithContext rounds an exact decimal result using context.
func SubtractExactWithContext(left, right Expr, context DecimalMathContext) Expression[big.Rat] {
	return decimalMathExpression("subtract-decimal", "-", left, right, context, func(l, r *big.Rat) (*big.Rat, bool) {
		return new(big.Rat).Sub(l, r), true
	})
}

// MultiplyExactWithContext rounds an exact decimal result using context.
func MultiplyExactWithContext(left, right Expr, context DecimalMathContext) Expression[big.Rat] {
	return decimalMathExpression("multiply-decimal", "*", left, right, context, func(l, r *big.Rat) (*big.Rat, bool) {
		return new(big.Rat).Mul(l, r), true
	})
}

// DivideExactWithContext performs exact decimal division and applies context
// rounding. A zero divisor returns Null, matching the existing expression
// division contract.
func DivideExactWithContext(left, right Expr, context DecimalMathContext) Expression[big.Rat] {
	return decimalMathExpression("divide-decimal", "/", left, right, context, func(l, r *big.Rat) (*big.Rat, bool) {
		if r.Sign() == 0 {
			return nil, false
		}
		return new(big.Rat).Quo(l, r), true
	})
}

// RoundDecimal applies context to an exact decimal expression without
// changing its explicit result type.
func RoundDecimal(value Expr, context DecimalMathContext) Expression[big.Rat] {
	description := "round-decimal(<nil>)"
	var children []*exprNode
	invalid := context.validate()
	if value == nil || value.node() == nil {
		if invalid == nil {
			invalid = fmt.Errorf("decimal rounding requires a value expression")
		}
	} else {
		description = "round-decimal(" + value.Description() + ";" + context.description() + ")"
		children = []*exprNode{value.node()}
		if !isMathNumericType(value.Type()) && invalid == nil {
			invalid = fmt.Errorf("decimal rounding value must be numeric, got %s", mathTypeDescription(value.Type()))
		}
	}
	node := &exprNode{
		kind:               "round-decimal",
		typ:                typeOf[big.Rat](),
		description:        description,
		children:           children,
		configurationError: errorMessage(invalid),
	}
	return typedExpr[big.Rat]{n: node, fn: func(ctx EvalContext) Value {
		if value == nil {
			return Null()
		}
		number, ok := enumRatFromValue(value.eval(ctx))
		if !ok {
			return Null()
		}
		rounded, err := roundDecimalRat(number, context)
		if err != nil {
			return Null()
		}
		return Present(*rounded)
	}}
}

func decimalMathExpression(kind, symbol string, left, right Expr, context DecimalMathContext, operation func(*big.Rat, *big.Rat) (*big.Rat, bool)) Expression[big.Rat] {
	expression := mathExpression[big.Rat](kind, symbol, left, right, func(leftValue, rightValue Value) Value {
		leftNumber, leftOK := enumRatFromValue(leftValue)
		rightNumber, rightOK := enumRatFromValue(rightValue)
		if !leftOK || !rightOK {
			return Null()
		}
		result, ok := operation(leftNumber, rightNumber)
		if !ok {
			return Null()
		}
		rounded, err := roundDecimalRat(result, context)
		if err != nil {
			return Null()
		}
		return Present(*rounded)
	})
	if invalid := context.validate(); invalid != nil {
		if expression.node().configurationError == "" {
			expression.node().configurationError = invalid.Error()
		}
	} else {
		expression.node().description += "[" + context.description() + "]"
	}
	return expression
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func roundDecimalRat(value *big.Rat, context DecimalMathContext) (*big.Rat, error) {
	if value == nil {
		return nil, fmt.Errorf("decimal value is nil")
	}
	if err := context.validate(); err != nil {
		return nil, err
	}
	if context.Precision == 0 || value.Sign() == 0 {
		return new(big.Rat).Set(value), nil
	}

	negative := value.Sign() < 0
	numerator := new(big.Int).Abs(value.Num())
	denominator := new(big.Int).Set(value.Denom())
	exponent := decimalExponent(numerator, denominator)
	scale := context.Precision - 1 - exponent
	roundingNumerator := new(big.Int).Set(numerator)
	roundingDenominator := new(big.Int).Set(denominator)
	if scale >= 0 {
		roundingNumerator.Mul(roundingNumerator, decimalPower10(scale))
	} else {
		roundingDenominator.Mul(roundingDenominator, decimalPower10(-scale))
	}

	quotient, remainder := new(big.Int).QuoRem(roundingNumerator, roundingDenominator, new(big.Int))
	if remainder.Sign() != 0 {
		increment, err := decimalShouldIncrement(quotient, remainder, roundingDenominator, negative, context.Rounding)
		if err != nil {
			return nil, err
		}
		if increment {
			quotient.Add(quotient, big.NewInt(1))
		}
	}
	if negative {
		quotient.Neg(quotient)
	}
	if scale >= 0 {
		return new(big.Rat).SetFrac(quotient, decimalPower10(scale)), nil
	}
	return new(big.Rat).SetInt(new(big.Int).Mul(quotient, decimalPower10(-scale))), nil
}

func decimalExponent(numerator, denominator *big.Int) int {
	// The digit-count estimate is at most one away from floor(log10(value));
	// exact cross multiplication fixes that boundary without float64 loss.
	exponent := len(numerator.String()) - len(denominator.String())
	for decimalComparePower10(numerator, denominator, exponent) < 0 {
		exponent--
	}
	for decimalComparePower10(numerator, denominator, exponent+1) >= 0 {
		exponent++
	}
	return exponent
}

func decimalComparePower10(numerator, denominator *big.Int, exponent int) int {
	left := new(big.Int).Set(numerator)
	right := new(big.Int).Set(denominator)
	if exponent >= 0 {
		right.Mul(right, decimalPower10(exponent))
	} else {
		left.Mul(left, decimalPower10(-exponent))
	}
	return left.Cmp(right)
}

func decimalPower10(exponent int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil)
}

func decimalShouldIncrement(quotient, remainder, denominator *big.Int, negative bool, mode DecimalRoundingMode) (bool, error) {
	switch mode {
	case DecimalRoundDown:
		return false, nil
	case DecimalRoundUp:
		return true, nil
	case DecimalRoundCeiling:
		return !negative, nil
	case DecimalRoundFloor:
		return negative, nil
	case DecimalRoundHalfUp, DecimalRoundHalfDown, DecimalRoundHalfEven:
		doubled := new(big.Int).Lsh(new(big.Int).Set(remainder), 1)
		comparison := doubled.Cmp(denominator)
		if comparison > 0 || (comparison == 0 && mode == DecimalRoundHalfUp) {
			return true, nil
		}
		if comparison < 0 || mode == DecimalRoundHalfDown {
			return false, nil
		}
		return quotient.Bit(0) == 1, nil
	case DecimalRoundUnnecessary:
		return false, fmt.Errorf("decimal rounding would require a discarded fraction")
	default:
		return false, fmt.Errorf("decimal math context has an invalid rounding mode %d", mode)
	}
}
