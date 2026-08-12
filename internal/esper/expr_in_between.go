package esper

import (
	"fmt"
	"math/big"
	"reflect"
)

// InOf compares a scalar value with mixed scalar, array, slice, collection or
// map-key candidates. It is the explicit-result-type counterpart of In for
// fluent rules that need Java's numeric coercion and collection expansion.
func InOf(value Expr, candidates ...Expr) Expression[bool] {
	return inExpression("in-of", value, false, candidates...)
}

// NotInOf is the Null-aware negated form of InOf.
func NotInOf(value Expr, candidates ...Expr) Expression[bool] {
	return inExpression("not-in-of", value, true, candidates...)
}

// BetweenOf compares mixed ordered operands and normalizes reversed bounds,
// matching Esper's between semantics. A Null/Missing value or bound yields
// false, including for NotBetweenOf, as in the Java regression matrix.
func BetweenOf(value, lower, upper Expr) Expression[bool] {
	return betweenExpression("between-of", value, lower, upper, false)
}

// BetweenRangeOf compares mixed ordered operands with independently selected
// lower and upper boundary inclusivity. It is the fluent counterpart of
// Esper's four bracketed range forms. Reversed numeric bounds are normalized
// before comparison while the boundary markers remain attached to the lower
// and upper comparison positions, matching Esper's range normalization.
func BetweenRangeOf(value, lower, upper Expr, lowerInclusive, upperInclusive bool) Expression[bool] {
	return betweenExpressionWithBounds("between-range-of", value, lower, upper, false, lowerInclusive, upperInclusive)
}

// NotBetweenOf is the explicit fluent negation of BetweenOf. It retains
// Esper's rule that an absent value or bound yields false rather than true.
func NotBetweenOf(value, lower, upper Expr) Expression[bool] {
	return betweenExpression("not-between-of", value, lower, upper, true)
}

// NotBetweenRangeOf is the null-aware negated form of BetweenRangeOf.
func NotBetweenRangeOf(value, lower, upper Expr, lowerInclusive, upperInclusive bool) Expression[bool] {
	return betweenExpressionWithBounds("not-between-range-of", value, lower, upper, true, lowerInclusive, upperInclusive)
}

func inExpression(kind string, value Expr, negate bool, candidates ...Expr) Expression[bool] {
	parts := make([]string, 0, 1+len(candidates))
	parts = append(parts, expressionDescription(value))
	children := []*exprNode{nil}
	if value != nil {
		children[0] = value.node()
	}
	for _, candidate := range candidates {
		parts = append(parts, expressionDescription(candidate))
		if candidate == nil {
			children = append(children, nil)
		} else {
			children = append(children, candidate.node())
		}
	}
	description := fmt.Sprintf("%s (%s)", kind, joinExpressionParts(parts, ","))
	node := &exprNode{kind: kind, typ: typeOf[bool](), description: description, children: children}
	if value == nil || value.node() == nil {
		node.configurationError = "IN comparison requires a scalar value expression"
	} else if inCollectionType(value.Type()) {
		node.configurationError = "IN comparison left operand must be scalar"
	} else if value.node().kind == "null" {
		node.configurationError = "IN comparison left operand must not be null"
	} else if len(candidates) == 0 {
		node.configurationError = "IN comparison requires at least one candidate"
	} else {
		for index, candidate := range candidates {
			if candidate == nil || candidate.node() == nil {
				node.configurationError = fmt.Sprintf("IN comparison candidate %d is required", index)
				break
			}
		}
	}
	return typedExpr[bool]{n: node, fn: func(ctx EvalContext) Value {
		if value == nil {
			return Null()
		}
		left := value.eval(ctx)
		if !left.IsPresent() || inValueIsNil(left) {
			return Null()
		}
		rights := make([]inCandidateValue, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate == nil {
				rights = append(rights, inCandidateValue{value: Missing()})
				continue
			}
			candidateValue := candidate.eval(ctx)
			if !candidateValue.IsPresent() {
				if !inCollectionType(candidate.Type()) {
					rights = append(rights, inCandidateValue{value: Null()})
				}
				continue
			}
			appendInValues(candidateValue, &rights)
		}
		hasNull := false
		for _, right := range rights {
			if !right.value.IsPresent() {
				hasNull = true
				continue
			}
			comparison, comparable := inValuesEqual(left, right.value, right.numericCoercion)
			if !comparable {
				continue
			}
			matched, ok := boolValue(comparison)
			if !ok {
				hasNull = true
				continue
			}
			if matched {
				if negate {
					return Present(false)
				}
				return Present(true)
			}
		}
		if hasNull {
			return Null()
		}
		if negate {
			return Present(true)
		}
		return Present(false)
	}}
}

func betweenExpression(kind string, value, lower, upper Expr, negate bool) Expression[bool] {
	return betweenExpressionWithBounds(kind, value, lower, upper, negate, true, true)
}

func betweenExpressionWithBounds(kind string, value, lower, upper Expr, negate, lowerInclusive, upperInclusive bool) Expression[bool] {
	parts := []string{expressionDescription(value), expressionDescription(lower), expressionDescription(upper)}
	children := make([]*exprNode, 0, 3)
	for _, operand := range []Expr{value, lower, upper} {
		if operand == nil {
			children = append(children, nil)
		} else {
			children = append(children, operand.node())
		}
	}
	description := fmt.Sprintf("%s(%s)", kind, joinExpressionParts(parts, ","))
	node := &exprNode{kind: kind, typ: typeOf[bool](), description: description, children: children}
	for index, operand := range []Expr{value, lower, upper} {
		if operand == nil || operand.node() == nil {
			node.configurationError = fmt.Sprintf("%s operand %d is required", kind, index)
			break
		}
		if operand.node().kind != "null" && !isRelationalOrderedType(operand.Type()) {
			node.configurationError = fmt.Sprintf("%s operand %d must be ordered, got %s", kind, index, mathTypeDescription(operand.Type()))
			break
		}
	}
	if node.configurationError == "" {
		// Esper rejects ranges whose operands belong to different comparison
		// domains (for example a string field between numeric bounds, or a
		// numeric field between string bounds) because no implicit conversion
		// exists between them. Untyped and interface operands defer to the
		// runtime evaluation, which yields false for incomparable values.
		referenceDomain := ""
		referenceType := ""
		for _, operand := range []Expr{value, lower, upper} {
			domain := betweenComparisonDomain(operand.Type())
			if domain == "" {
				continue
			}
			if referenceDomain == "" {
				referenceDomain = domain
				referenceType = mathTypeDescription(operand.Type())
				continue
			}
			if domain != referenceDomain {
				node.configurationError = fmt.Sprintf("%s operands are not compatible: %s and %s", kind, referenceType, mathTypeDescription(operand.Type()))
				break
			}
		}
	}
	return typedExpr[bool]{n: node, fn: func(ctx EvalContext) Value {
		if value == nil || lower == nil || upper == nil {
			return Null()
		}
		current := value.eval(ctx)
		low := lower.eval(ctx)
		high := upper.eval(ctx)
		if !current.IsPresent() || !low.IsPresent() || !high.IsPresent() {
			return Present(false)
		}
		bounds, comparable := enumCompareValues(low, high)
		if !comparable {
			return Present(false)
		}
		if bounds > 0 {
			low, high = high, low
		}
		lowerComparison, lowerOK := enumCompareValues(current, low)
		upperComparison, upperOK := enumCompareValues(current, high)
		if !lowerOK || !upperOK {
			return Present(false)
		}
		matched := (lowerComparison > 0 || (lowerInclusive && lowerComparison == 0)) &&
			(upperComparison < 0 || (upperInclusive && upperComparison == 0))
		if negate {
			return Present(!matched)
		}
		return Present(matched)
	}}
}

// betweenComparisonDomain classifies the static comparison domain of an
// ordered range operand. Numeric operands (including big.Int and big.Rat)
// share one domain and strings form the other; pointer chains unwrap to the
// element domain. Untyped or interface operands return an empty domain and
// defer compatibility to runtime evaluation.
func betweenComparisonDomain(typ reflect.Type) string {
	if typ == nil || typ == typeOf[any]() {
		return ""
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		if typ == nil {
			return ""
		}
	}
	if typ.Kind() == reflect.Interface {
		return ""
	}
	if typ == typeOf[big.Int]() || typ == typeOf[big.Rat]() {
		return "numeric"
	}
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "numeric"
	case reflect.String:
		return "string"
	default:
		return ""
	}
}

func inCollectionType(typ reflect.Type) bool {
	if typ == nil || typ == typeOf[any]() {
		return false
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		if typ == nil {
			return false
		}
	}
	return typ.Kind() == reflect.Array || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Map
}

type inCandidateValue struct {
	value           Value
	numericCoercion bool
}

func appendInValues(value Value, destination *[]inCandidateValue) {
	if !value.IsPresent() {
		// A Null collection has no rows. A scalar Null is represented by the
		// caller as a typed non-collection expression and reaches this helper
		// as an absent value, so preserve it for the common literal path.
		return
	}
	raw := reflect.ValueOf(value.Any())
	for raw.IsValid() && raw.Kind() == reflect.Interface {
		if raw.IsNil() {
			*destination = append(*destination, inCandidateValue{value: Null()})
			return
		}
		raw = raw.Elem()
	}
	if !raw.IsValid() {
		*destination = append(*destination, inCandidateValue{value: Null()})
		return
	}
	if raw.Kind() == reflect.Pointer {
		if raw.IsNil() {
			*destination = append(*destination, inCandidateValue{value: Null()})
			return
		}
		raw = raw.Elem()
	}
	switch raw.Kind() {
	case reflect.Array, reflect.Slice:
		numericCoercion := inCollectionNumericCoercion(raw.Type().Elem())
		for index := 0; index < raw.Len(); index++ {
			*destination = append(*destination, inCandidateValue{
				value:           inReflectValue(raw.Index(index)),
				numericCoercion: numericCoercion,
			})
		}
	case reflect.Map:
		numericCoercion := inCollectionNumericCoercion(raw.Type().Key())
		for _, key := range raw.MapKeys() {
			*destination = append(*destination, inCandidateValue{
				value:           inReflectValue(key),
				numericCoercion: numericCoercion,
			})
		}
	default:
		*destination = append(*destination, inCandidateValue{value: value, numericCoercion: true})
	}
}

func inReflectValue(value reflect.Value) Value {
	if !value.IsValid() || !value.CanInterface() {
		return Missing()
	}
	if isNilReflectValue(value) {
		return Null()
	}
	return Present(value.Interface())
}

func inValuesEqual(left, right Value, numericCoercion bool) (Value, bool) {
	if !left.IsPresent() || !right.IsPresent() || inValueIsNil(left) || inValueIsNil(right) {
		return Null(), true
	}
	if !numericCoercion {
		return Present(left.Equal(right)), true
	}
	leftNumber, leftNumeric := enumRatFromValue(left)
	rightNumber, rightNumeric := enumRatFromValue(right)
	if leftNumeric || rightNumeric {
		if !leftNumeric || !rightNumeric {
			return Null(), false
		}
		return Present(leftNumber.Cmp(rightNumber) == 0), true
	}
	result := EqualValues(left, right)
	return result, result.IsPresent()
}

func inValueIsNil(value Value) bool {
	return value.IsPresent() && isNilReflectValue(reflect.ValueOf(value.Any()))
}

func inCollectionNumericCoercion(typ reflect.Type) bool {
	if typ == nil {
		return false
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		if typ == nil {
			return false
		}
	}
	if typ.Kind() == reflect.Interface {
		return false
	}
	if typ == typeOf[big.Int]() || typ == typeOf[big.Rat]() {
		return true
	}
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func joinExpressionParts(parts []string, separator string) string {
	result := ""
	for index, part := range parts {
		if index > 0 {
			result += separator
		}
		result += part
	}
	return result
}
