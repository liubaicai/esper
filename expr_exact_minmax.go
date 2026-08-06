package esper

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
)

// MinExactOf returns the smallest exact numeric value among two or more
// scalar expressions. It is the BigInteger/BigDecimal counterpart of MinOf.
func MinExactOf[T ExactNumeric](values ...Expr) Expression[T] {
	return exactScalarMinMax[T]("min-exact-of", "min", values, true)
}

// MaxExactOf returns the largest exact numeric value among two or more
// scalar expressions. It is the BigInteger/BigDecimal counterpart of MaxOf.
func MaxExactOf[T ExactNumeric](values ...Expr) Expression[T] {
	return exactScalarMinMax[T]("max-exact-of", "max", values, false)
}

func exactScalarMinMax[T ExactNumeric](kind, symbol string, values []Expr, chooseMinimum bool) Expression[T] {
	children := make([]*exprNode, 0, len(values))
	descriptions := make([]string, 0, len(values))
	for _, value := range values {
		if value == nil {
			children = append(children, nil)
			descriptions = append(descriptions, "<nil>")
			continue
		}
		children = append(children, value.node())
		descriptions = append(descriptions, value.Description())
	}
	node := &exprNode{
		kind:        kind,
		typ:         typeOf[T](),
		description: symbol + "-exact(" + strings.Join(descriptions, ",") + ")",
		children:    children,
	}
	if len(values) < 2 {
		node.configurationError = fmt.Sprintf("%s requires at least two operands", symbol)
	} else {
		for index, value := range values {
			if value == nil || value.node() == nil {
				node.configurationError = fmt.Sprintf("%s operand %d is required", symbol, index)
				break
			}
			if !isMathNumericType(value.Type()) {
				node.configurationError = fmt.Sprintf("%s operand %d must be numeric, got %s", symbol, index, mathTypeDescription(value.Type()))
				break
			}
		}
	}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		if len(values) < 2 {
			return Null()
		}
		var selected *big.Rat
		for index, value := range values {
			if value == nil {
				return Null()
			}
			number, ok := enumRatFromValue(value.eval(ctx))
			if !ok {
				return Null()
			}
			if index == 0 {
				selected = number
				continue
			}
			comparison := number.Cmp(selected)
			if (chooseMinimum && comparison < 0) || (!chooseMinimum && comparison > 0) {
				selected = number
			}
		}
		converted, ok := enumRatTo[T](selected)
		if !ok {
			return Null()
		}
		if isNilReflectValue(reflect.ValueOf(converted)) {
			return Null()
		}
		return Present(converted)
	}}
}
