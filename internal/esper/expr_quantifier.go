package esper

import (
	"fmt"
	"reflect"
	"strings"
)

// QuantifierComparison is the comparison used by AnyOf/SomeOf/AllOf. It is
// an alias of the subquery comparison enum so the same explicit comparison
// vocabulary can be used for scalar lists and subquery rows.
type QuantifierComparison = SubqueryComparison

const (
	QuantifierEqual          = SubqueryEqual
	QuantifierNotEqual       = SubqueryNotEqual
	QuantifierGreater        = SubqueryGreater
	QuantifierGreaterOrEqual = SubqueryGreaterOrEqual
	QuantifierLess           = SubqueryLess
	QuantifierLessOrEqual    = SubqueryLessOrEqual
)

// AnyOf returns true when at least one scalar or collection candidate matches
// value. Candidate arrays, slices and maps are expanded without exposing a
// Java collection abstraction; map keys follow Esper's ANY/ALL collection
// contract. Null and Missing candidates participate in three-valued logic.
func AnyOf(value Expr, comparison QuantifierComparison, candidates ...Expr) Expression[bool] {
	return quantifiedExpression("any", false, value, comparison, candidates...)
}

// SomeOf is the SQL/Esper synonym for AnyOf.
func SomeOf(value Expr, comparison QuantifierComparison, candidates ...Expr) Expression[bool] {
	return quantifiedExpression("some", false, value, comparison, candidates...)
}

// AllOf returns true only when every scalar or collection candidate matches
// value. A definite false wins over a later Null; otherwise Null is preserved.
func AllOf(value Expr, comparison QuantifierComparison, candidates ...Expr) Expression[bool] {
	return quantifiedExpression("all", true, value, comparison, candidates...)
}

func quantifiedExpression(kind string, all bool, value Expr, comparison QuantifierComparison, candidates ...Expr) Expression[bool] {
	parts := make([]string, 0, 1+len(candidates))
	children := make([]*exprNode, 0, 1+len(candidates))
	if value == nil {
		parts = append(parts, "<nil>")
		children = append(children, nil)
	} else {
		parts = append(parts, value.Description())
		children = append(children, value.node())
	}
	for _, candidate := range candidates {
		if candidate == nil {
			parts = append(parts, "<nil>")
			children = append(children, nil)
			continue
		}
		parts = append(parts, candidate.Description())
		children = append(children, candidate.node())
	}
	description := fmt.Sprintf("%s %s %s (%s)", parts[0], comparison.symbol(), kind, strings.Join(parts[1:], ","))
	node := &exprNode{kind: "quantifier-" + kind, typ: typeOf[bool](), description: description, children: children}
	if value == nil || value.node() == nil {
		node.configurationError = "quantified comparison requires a scalar value expression"
	} else if quantifiedCollectionType(value.Type()) {
		node.configurationError = "quantified comparison left operand must be scalar"
	} else if value.node().kind == "null" {
		node.configurationError = "quantified comparison left operand must not be null"
	} else if len(candidates) == 0 {
		node.configurationError = "quantified comparison requires at least one candidate"
	} else if comparison < SubqueryEqual || comparison > SubqueryLessOrEqual {
		node.configurationError = "quantified comparison has an invalid operator"
	} else {
		for index, candidate := range candidates {
			if candidate == nil || candidate.node() == nil {
				node.configurationError = fmt.Sprintf("quantified comparison candidate %d is required", index)
				break
			}
		}
	}
	return typedExpr[bool]{n: node, fn: func(ctx EvalContext) Value {
		if value == nil {
			return Null()
		}
		rows := make([]Value, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate == nil {
				rows = append(rows, Missing())
				continue
			}
			appendQuantifiedValues(candidate.eval(ctx), &rows)
		}
		return evaluateQuantifiedValues(value.eval(ctx), rows, comparison, all)
	}}
}

func quantifiedCollectionType(typ reflect.Type) bool {
	if typ == nil || typ == typeOf[any]() {
		return false
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		if typ == nil {
			return false
		}
	}
	switch typ.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map:
		return true
	default:
		return false
	}
}

func appendQuantifiedValues(value Value, destination *[]Value) {
	if !value.IsPresent() {
		*destination = append(*destination, value)
		return
	}
	raw := reflect.ValueOf(value.Any())
	for raw.IsValid() && raw.Kind() == reflect.Interface {
		if raw.IsNil() {
			*destination = append(*destination, Null())
			return
		}
		raw = raw.Elem()
	}
	if !raw.IsValid() {
		*destination = append(*destination, Null())
		return
	}
	if raw.Kind() == reflect.Pointer {
		if raw.IsNil() {
			*destination = append(*destination, Null())
			return
		}
		appendQuantifiedValues(Present(raw.Elem().Interface()), destination)
		return
	}
	switch raw.Kind() {
	case reflect.Array, reflect.Slice:
		for index := 0; index < raw.Len(); index++ {
			*destination = append(*destination, quantifiedReflectValue(raw.Index(index)))
		}
	case reflect.Map:
		for _, key := range raw.MapKeys() {
			*destination = append(*destination, quantifiedReflectValue(key))
		}
	default:
		*destination = append(*destination, value)
	}
}

func quantifiedReflectValue(value reflect.Value) Value {
	if !value.IsValid() {
		return Null()
	}
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return Null()
		}
		return quantifiedReflectValue(value.Elem())
	}
	if isNilReflectValue(value) {
		return Null()
	}
	if !value.CanInterface() {
		return Null()
	}
	return Present(value.Interface())
}

func evaluateQuantifiedValues(left Value, rows []Value, comparison QuantifierComparison, all bool) Value {
	if len(rows) == 0 {
		return Null()
	}
	if !left.IsPresent() {
		return Null()
	}
	hasComparable := false
	hasNull := false
	nullAffectsAny := comparison == SubqueryEqual || comparison == SubqueryNotEqual
	for _, right := range rows {
		if !right.IsPresent() {
			// A relational NULL is ignored by ANY/SOME, but remains an
			// unresolved value for ALL. Equality and inequality NULLs affect
			// both quantifiers. Keep scanning so a definite result later in
			// the collection can win regardless of candidate order.
			if all || nullAffectsAny {
				hasNull = true
			}
			continue
		}
		result, comparable := compareQuantifiedValues(left, right, comparison)
		if !comparable {
			if !right.IsPresent() {
				hasNull = true
			}
			continue
		}
		hasComparable = true
		matched, present := boolValue(result)
		if !present {
			hasNull = true
			continue
		}
		if all && !matched {
			return Present(false)
		}
		if !all && matched {
			return Present(true)
		}
	}
	if !hasComparable || hasNull {
		return Null()
	}
	if all {
		return Present(true)
	}
	return Present(false)
}

func compareQuantifiedValues(left, right Value, comparison QuantifierComparison) (Value, bool) {
	if !left.IsPresent() || !right.IsPresent() {
		return Null(), false
	}
	if comparison == SubqueryEqual || comparison == SubqueryNotEqual {
		leftNumber, leftOK := enumRatFromValue(left)
		rightNumber, rightOK := enumRatFromValue(right)
		if leftOK && rightOK {
			equal := leftNumber.Cmp(rightNumber) == 0
			if comparison == SubqueryNotEqual {
				equal = !equal
			}
			return Present(equal), true
		}
		result := EqualValues(left, right)
		if !result.IsPresent() {
			return Null(), false
		}
		if comparison == SubqueryNotEqual {
			return Present(!result.Any().(bool)), true
		}
		return result, true
	}
	ordering, ok := enumCompareValues(left, right)
	if !ok {
		return Null(), false
	}
	switch comparison {
	case SubqueryGreater:
		return Present(ordering > 0), true
	case SubqueryGreaterOrEqual:
		return Present(ordering >= 0), true
	case SubqueryLess:
		return Present(ordering < 0), true
	case SubqueryLessOrEqual:
		return Present(ordering <= 0), true
	default:
		return Null(), false
	}
}
