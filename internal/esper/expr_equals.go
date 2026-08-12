package esper

import (
	"fmt"
	"math/big"
	"reflect"
)

// EqualOf compares mixed scalar or array expressions using Esper's
// expression-level numeric coercion. Null/Missing operands produce Null.
func EqualOf(left, right Expr) Expression[bool] {
	return equalityExpression("equal-of", "=", left, right, false)
}

// NotEqualOf is the mixed-type counterpart of NotEqual.
func NotEqualOf(left, right Expr) Expression[bool] {
	return equalityExpression("not-equal-of", "!=", left, right, false)
}

// Is is Esper's null-safe equality operator. Two Null/Missing values compare
// equal, one absent value compares unequal to a present value, and present
// values use the same coercion/deep-equality path as EqualOf.
func Is(left, right Expr) Expression[bool] {
	return equalityExpression("is", "is", left, right, true)
}

// IsNot is the null-safe negated equality operator.
func IsNot(left, right Expr) Expression[bool] {
	return equalityExpression("is-not", "is not", left, right, true)
}

func equalityExpression(kind, symbol string, left, right Expr, nullSafe bool) Expression[bool] {
	description := fmt.Sprintf("(%s %s %s)", expressionDescription(left), symbol, expressionDescription(right))
	node := &exprNode{kind: kind, typ: typeOf[bool](), description: description, children: []*exprNode{nil, nil}}
	if left != nil {
		node.children[0] = left.node()
	}
	if right != nil {
		node.children[1] = right.node()
	}
	if left == nil || left.node() == nil {
		node.configurationError = "equality comparison requires a left operand"
	} else if right == nil || right.node() == nil {
		node.configurationError = "equality comparison requires a right operand"
	} else if left.node().kind != "null" && right.node().kind != "null" && !equalityTypesCompatible(left.Type(), right.Type()) {
		node.configurationError = fmt.Sprintf("equality operands are not compatible: %s and %s", equalityTypeDescription(left.Type()), equalityTypeDescription(right.Type()))
	}
	return typedExpr[bool]{n: node, fn: func(ctx EvalContext) Value {
		if left == nil || right == nil {
			return Null()
		}
		leftValue := left.eval(ctx)
		rightValue := right.eval(ctx)
		if nullSafe {
			leftAbsent := !leftValue.IsPresent() || equalityValueIsNil(leftValue)
			rightAbsent := !rightValue.IsPresent() || equalityValueIsNil(rightValue)
			if leftAbsent || rightAbsent {
				matched := leftAbsent && rightAbsent
				if kind == "is-not" {
					matched = !matched
				}
				return Present(matched)
			}
		}
		result, comparable := equalExpressionValues(leftValue, rightValue)
		if !comparable {
			return Null()
		}
		matched, ok := boolValue(result)
		if !ok {
			return Null()
		}
		if kind == "not-equal-of" || kind == "is-not" {
			matched = !matched
		}
		return Present(matched)
	}}
}

func equalExpressionValues(left, right Value) (Value, bool) {
	if !left.IsPresent() || !right.IsPresent() || equalityValueIsNil(left) || equalityValueIsNil(right) {
		return Null(), false
	}
	leftNumber, leftNumeric := enumRatFromValue(left)
	rightNumber, rightNumeric := enumRatFromValue(right)
	if leftNumeric || rightNumeric {
		if !leftNumeric || !rightNumeric {
			return Null(), false
		}
		return Present(leftNumber.Cmp(rightNumber) == 0), true
	}
	return EqualValues(left, right), true
}

func equalityValueIsNil(value Value) bool {
	return value.IsPresent() && isNilReflectValue(reflect.ValueOf(value.Any()))
}

func equalityTypesCompatible(left, right reflect.Type) bool {
	if left == nil || right == nil || left == typeOf[any]() || right == typeOf[any]() {
		return true
	}
	left = unwrapEqualityType(left)
	right = unwrapEqualityType(right)
	if left == nil || right == nil {
		return true
	}
	if left == right || left.AssignableTo(right) || right.AssignableTo(left) {
		return true
	}
	if isEqualityNumericType(left) && isEqualityNumericType(right) {
		return true
	}
	if left.Kind() == reflect.Interface || right.Kind() == reflect.Interface {
		return true
	}
	if left.Kind() == reflect.Array || left.Kind() == reflect.Slice || right.Kind() == reflect.Array || right.Kind() == reflect.Slice {
		// Go slices and arrays are invariant.  In particular, []int and
		// []int64 (or []any and []bool) must not be treated as scalar numeric
		// coercions: Esper rejects the corresponding different Java array
		// component types during equality validation.
		return false
	}
	return false
}

func unwrapEqualityType(typ reflect.Type) reflect.Type {
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}

func isEqualityNumericType(typ reflect.Type) bool {
	if typ == nil {
		return false
	}
	return isNumericType(typ) || typ == reflect.TypeOf(big.Int{}) || typ == reflect.TypeOf(big.Rat{})
}

func equalityTypeDescription(typ reflect.Type) string {
	if typ == nil {
		return "any"
	}
	return typ.String()
}
