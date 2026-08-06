package esper

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
)

// AddOf is the explicit-result-type form of Add. It accepts operands whose
// concrete numeric Go types differ, which is the fluent equivalent of
// Esper's numeric coercion rules. The result type is deliberately visible at
// the call site.
func AddOf[T Numeric](left, right Expr) Expression[T] {
	return nativeMathExpression[T]("add", "+", left, right, func(l, r *big.Rat) (*big.Rat, bool) {
		return new(big.Rat).Add(l, r), true
	})
}

// SubtractOf is the mixed-operand counterpart of Subtract.
func SubtractOf[T Numeric](left, right Expr) Expression[T] {
	return nativeMathExpression[T]("subtract", "-", left, right, func(l, r *big.Rat) (*big.Rat, bool) {
		return new(big.Rat).Sub(l, r), true
	})
}

// MultiplyOf is the mixed-operand counterpart of Multiply.
func MultiplyOf[T Numeric](left, right Expr) Expression[T] {
	return nativeMathExpression[T]("multiply", "*", left, right, func(l, r *big.Rat) (*big.Rat, bool) {
		return new(big.Rat).Mul(l, r), true
	})
}

// ModuloOf evaluates a numeric remainder while preserving the explicitly
// requested result type. Integral operands use arbitrary-precision remainder
// arithmetic; floating operands use IEEE remainder semantics.
func ModuloOf[T Numeric](left, right Expr) Expression[T] {
	return mathExpression[T]("modulo", "%", left, right, func(l, r Value) Value {
		leftNumber, leftOK := enumRatFromValue(l)
		rightNumber, rightOK := enumRatFromValue(r)
		if !leftOK || !rightOK || rightNumber.Sign() == 0 {
			return Null()
		}
		if leftNumber.IsInt() && rightNumber.IsInt() {
			leftInteger := new(big.Int).Set(leftNumber.Num())
			rightInteger := new(big.Int).Set(rightNumber.Num())
			return nativeRatResult[T](new(big.Rat).SetInt(new(big.Int).Rem(leftInteger, rightInteger)))
		}
		leftFloat, leftNumeric := numericValue(l)
		rightFloat, rightNumeric := numericValue(r)
		if !leftNumeric || !rightNumeric {
			return Null()
		}
		return Present(convertNumeric[T](math.Mod(leftFloat, rightFloat)))
	})
}

// DivisionOption configures DivideWithOptions. Options are local to one
// expression, keeping a Plan immutable and making a rule's division policy
// explicit in source and in the canonical plan.
type DivisionOption func(*divisionConfig)

type divisionConfig struct {
	integerDivision           bool
	divisionByZeroReturnsNull bool
}

// WithIntegerDivision selects Java-convention integer division for integral
// operands. When false, division keeps the fractional result; callers should
// normally choose float64 as the result type for that mode.
func WithIntegerDivision(enabled bool) DivisionOption {
	return func(config *divisionConfig) { config.integerDivision = enabled }
}

// WithDivisionByZeroReturnsNull controls the Esper compiler option that turns
// division by zero into Null. The default for DivideWithOptions is IEEE
// infinity/NaN for a floating result, matching Esper's default configuration.
func WithDivisionByZeroReturnsNull(enabled bool) DivisionOption {
	return func(config *divisionConfig) { config.divisionByZeroReturnsNull = enabled }
}

// DivideWithOptions is the configurable division form. It accepts mixed
// numeric operands and carries the requested result type through the plan.
// Divide and Modulo retain their historical safe-null behavior; new rules
// that need Esper's configuration-level division semantics should use this
// constructor explicitly.
func DivideWithOptions[T Numeric](left, right Expr, options ...DivisionOption) Expression[T] {
	config := divisionConfig{}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	return mathExpression[T]("divide", "/", left, right, func(l, r Value) Value {
		leftNumber, leftOK := enumRatFromValue(l)
		rightNumber, rightOK := enumRatFromValue(r)
		if !leftOK || !rightOK {
			return Null()
		}
		if rightNumber.Sign() == 0 {
			if config.divisionByZeroReturnsNull || config.integerDivision {
				return Null()
			}
			return divideByZeroResult[T](l, r)
		}
		// The result is kept as a rational until the explicit result type is
		// applied. This avoids losing large integral values through float64
		// before AddOf/DivideWithOptions performs its final conversion.
		return nativeRatResult[T](new(big.Rat).Quo(leftNumber, rightNumber))
	})
}

// DivideFloat is a concise default-configuration division expression. It is
// useful for Esper's default non-integer division result type.
func DivideFloat(left, right Expr, options ...DivisionOption) Expression[float64] {
	return DivideWithOptions[float64](left, right, options...)
}

// AddExact adds native or arbitrary-precision numeric operands and returns a
// big.Int or big.Rat result selected by T. big.Int rejects fractional results;
// big.Rat is the exact Go representation used for Esper BigDecimal values.
func AddExact[T ExactNumeric](left, right Expr) Expression[T] {
	return exactMathExpression[T]("add-exact", "+", left, right, func(l, r *big.Rat) (*big.Rat, bool) {
		return new(big.Rat).Add(l, r), true
	})
}

// SubtractExact subtracts native or arbitrary-precision numeric operands.
func SubtractExact[T ExactNumeric](left, right Expr) Expression[T] {
	return exactMathExpression[T]("subtract-exact", "-", left, right, func(l, r *big.Rat) (*big.Rat, bool) {
		return new(big.Rat).Sub(l, r), true
	})
}

// MultiplyExact multiplies native or arbitrary-precision numeric operands.
func MultiplyExact[T ExactNumeric](left, right Expr) Expression[T] {
	return exactMathExpression[T]("multiply-exact", "*", left, right, func(l, r *big.Rat) (*big.Rat, bool) {
		return new(big.Rat).Mul(l, r), true
	})
}

// DivideExact performs exact division. A zero divisor and a non-integral
// result requested as big.Int evaluate to Null.
func DivideExact[T ExactNumeric](left, right Expr) Expression[T] {
	return exactMathExpression[T]("divide-exact", "/", left, right, func(l, r *big.Rat) (*big.Rat, bool) {
		if r.Sign() == 0 {
			return nil, false
		}
		return new(big.Rat).Quo(l, r), true
	})
}

func nativeMathExpression[T Numeric](kind, symbol string, left, right Expr, operation func(*big.Rat, *big.Rat) (*big.Rat, bool)) Expression[T] {
	return mathExpression[T](kind, symbol, left, right, func(l, r Value) Value {
		leftNumber, leftOK := enumRatFromValue(l)
		rightNumber, rightOK := enumRatFromValue(r)
		if !leftOK || !rightOK {
			return Null()
		}
		result, ok := operation(leftNumber, rightNumber)
		if !ok {
			return Null()
		}
		return nativeRatResult[T](result)
	})
}

func exactMathExpression[T ExactNumeric](kind, symbol string, left, right Expr, operation func(*big.Rat, *big.Rat) (*big.Rat, bool)) Expression[T] {
	return mathExpression[T](kind, symbol, left, right, func(l, r Value) Value {
		leftNumber, leftOK := enumRatFromValue(l)
		rightNumber, rightOK := enumRatFromValue(r)
		if !leftOK || !rightOK {
			return Null()
		}
		result, ok := operation(leftNumber, rightNumber)
		if !ok {
			return Null()
		}
		converted, ok := enumRatTo[T](result)
		if !ok {
			return Null()
		}
		return Present(converted)
	})
}

func mathExpression[T any](kind, symbol string, left, right Expr, evaluate func(Value, Value) Value) Expression[T] {
	leftDescription := mathOperandDescription(left)
	rightDescription := mathOperandDescription(right)
	children := make([]*exprNode, 0, 2)
	if left != nil {
		children = append(children, left.node())
	} else {
		children = append(children, nil)
	}
	if right != nil {
		children = append(children, right.node())
	} else {
		children = append(children, nil)
	}
	node := &exprNode{
		kind:        kind,
		typ:         typeOf[T](),
		description: "(" + leftDescription + " " + symbol + " " + rightDescription + ")",
		children:    children,
	}
	for index, operand := range []Expr{left, right} {
		if operand == nil || operand.node() == nil {
			node.configurationError = fmt.Sprintf("%s operator requires two operands", kind)
			break
		}
		if !isMathNumericType(operand.Type()) {
			node.configurationError = fmt.Sprintf("%s operand %d must be numeric, got %s", kind, index, mathTypeDescription(operand.Type()))
			break
		}
	}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		if left == nil || right == nil {
			return Null()
		}
		return evaluate(left.eval(ctx), right.eval(ctx))
	}}
}

func mathOperandDescription(expression Expr) string {
	if expression == nil {
		return "<nil>"
	}
	return expression.Description()
}

func mathTypeDescription(typ reflect.Type) string {
	if typ == nil {
		return "any"
	}
	return typ.String()
}

func isMathNumericType(typ reflect.Type) bool {
	if typ == nil || typ == typeOf[any]() {
		return true
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		if typ == nil {
			return false
		}
	}
	if typ.Kind() == reflect.Interface {
		return true
	}
	return isNumericType(typ) || typ == typeOf[big.Int]() || typ == typeOf[big.Rat]()
}

func nativeRatResult[T Numeric](number *big.Rat) Value {
	if number == nil {
		return Null()
	}
	converted, ok := enumRatTo[T](number)
	if !ok {
		return Null()
	}
	return Present(converted)
}

func divideByZeroResult[T Numeric](left, right Value) Value {
	leftFloat, leftOK := numericValue(left)
	rightFloat, rightOK := numericValue(right)
	if !leftOK || !rightOK {
		return Null()
	}
	if typeOf[T]().Kind() != reflect.Float32 && typeOf[T]().Kind() != reflect.Float64 {
		return Null()
	}
	if leftFloat == 0 {
		if typeOf[T]().Kind() == reflect.Float32 {
			return Present(float32(math.NaN()))
		}
		return Present(math.NaN())
	}
	sign := 1.0
	if (leftFloat < 0) != (rightFloat < 0) {
		sign = -1
	}
	if typeOf[T]().Kind() == reflect.Float32 {
		return Present(float32(math.Copysign(math.Inf(1), sign)))
	}
	return Present(math.Copysign(math.Inf(1), sign))
}
