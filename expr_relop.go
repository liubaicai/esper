package esper

import (
	"fmt"
	"math/big"
	"reflect"
)

// GreaterOf compares ordered expressions whose concrete Go types may differ.
// The explicit Of form mirrors Esper's numeric coercion while keeping the
// operands visible in the fluent plan.
func GreaterOf(left, right Expr) Expression[bool] {
	return orderedComparisonOf("greater-of", ">", left, right, func(comparison int) bool { return comparison > 0 })
}

// GreaterOrEqualOf is the mixed-type >= counterpart of GreaterOf.
func GreaterOrEqualOf(left, right Expr) Expression[bool] {
	return orderedComparisonOf("greater-equal-of", ">=", left, right, func(comparison int) bool { return comparison >= 0 })
}

// LessOf compares ordered expressions using the explicit mixed-type form.
func LessOf(left, right Expr) Expression[bool] {
	return orderedComparisonOf("less-of", "<", left, right, func(comparison int) bool { return comparison < 0 })
}

// LessOrEqualOf is the mixed-type <= counterpart of LessOf.
func LessOrEqualOf(left, right Expr) Expression[bool] {
	return orderedComparisonOf("less-equal-of", "<=", left, right, func(comparison int) bool { return comparison <= 0 })
}

func orderedComparisonOf(kind, symbol string, left, right Expr, predicate func(int) bool) Expression[bool] {
	leftDescription := expressionDescription(left)
	rightDescription := expressionDescription(right)
	node := &exprNode{
		kind:        kind,
		typ:         typeOf[bool](),
		description: "(" + leftDescription + " " + symbol + " " + rightDescription + ")",
	}
	if left != nil {
		node.children = append(node.children, left.node())
	} else {
		node.children = append(node.children, nil)
	}
	if right != nil {
		node.children = append(node.children, right.node())
	} else {
		node.children = append(node.children, nil)
	}
	if left == nil || left.node() == nil {
		node.configurationError = "ordered comparison requires a left operand"
	} else if right == nil || right.node() == nil {
		node.configurationError = "ordered comparison requires a right operand"
	} else if !isRelationalOrderedType(left.Type()) {
		node.configurationError = fmt.Sprintf("ordered comparison left operand must be ordered, got %s", mathTypeDescription(left.Type()))
	} else if !isRelationalOrderedType(right.Type()) {
		node.configurationError = fmt.Sprintf("ordered comparison right operand must be ordered, got %s", mathTypeDescription(right.Type()))
	}
	return typedExpr[bool]{n: node, fn: func(ctx EvalContext) Value {
		if left == nil || right == nil || predicate == nil {
			return Null()
		}
		comparison, ok := scalarOrderedCompare(left.eval(ctx), right.eval(ctx))
		if !ok {
			return Null()
		}
		return Present(predicate(comparison))
	}}
}

func isRelationalOrderedType(typ reflect.Type) bool {
	if typ == nil || typ == typeOf[any]() {
		return true
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		if typ == nil {
			return false
		}
	}
	if typ.Kind() == reflect.Interface || typ == typeOf[big.Int]() || typ == typeOf[big.Rat]() {
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
