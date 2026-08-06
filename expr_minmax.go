package esper

import (
	"fmt"
	"reflect"
	"strings"
)

// MinOf returns the smallest value among two or more ordered expressions.
//
// The explicit result type keeps the fluent API predictable when operands
// have different Go numeric types, for example MinOf[int64](intField,
// shortField). As in Esper's scalar min/max operators, a Null or Missing
// operand makes the complete result Null.
func MinOf[T Ordered](values ...Expr) Expression[T] {
	return scalarMinMax[T]("min-of", "min", values, true)
}

// MaxOf returns the largest value among two or more ordered expressions.
// It is the Go-style counterpart of Esper's multi-operand scalar max
// expression; aggregate Max remains available for window state.
func MaxOf[T Ordered](values ...Expr) Expression[T] {
	return scalarMinMax[T]("max-of", "max", values, false)
}

func scalarMinMax[T Ordered](kind, symbol string, values []Expr, chooseMinimum bool) Expression[T] {
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
		description: symbol + "(" + strings.Join(descriptions, ",") + ")",
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
			if !isScalarOrderedType(value.Type()) {
				node.configurationError = fmt.Sprintf("%s operand %d must be ordered, got %s", symbol, index, value.Type())
				break
			}
		}
	}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		if len(values) < 2 {
			return Null()
		}
		var selected Value
		for index, value := range values {
			if value == nil {
				return Null()
			}
			candidate := value.eval(ctx)
			if !candidate.IsPresent() || isNilReflectValue(reflect.ValueOf(candidate.Any())) {
				return Null()
			}
			if index == 0 {
				selected = candidate
				continue
			}
			comparison, ok := scalarOrderedCompare(candidate, selected)
			if !ok {
				return Null()
			}
			if (chooseMinimum && comparison < 0) || (!chooseMinimum && comparison > 0) {
				selected = candidate
			}
		}
		return castPropertyValue[T](selected)
	}}
}

func isScalarOrderedType(typ reflect.Type) bool {
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
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		return true
	default:
		return false
	}
}

func scalarOrderedCompare(left, right Value) (int, bool) {
	leftNumber, leftNumeric := enumRatFromValue(left)
	rightNumber, rightNumeric := enumRatFromValue(right)
	if leftNumeric || rightNumeric {
		if !leftNumeric || !rightNumeric {
			return 0, false
		}
		return leftNumber.Cmp(rightNumber), true
	}
	return compareValues(left, right)
}
