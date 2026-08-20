package esper

import (
	"fmt"
	"math"
	"reflect"
	"time"
)

// DateTimeBetween compares date-time expressions by epoch milliseconds. The
// two-argument Esper form is inclusive and normalizes reversed bounds.
func DateTimeBetween(value, lower, upper Expr) Expression[bool] {
	return dateTimeBetweenExpression("date-time-between", value, lower, upper, Literal(true), Literal(true))
}

// DateTimeBetweenWithEndpoints compares date-time expressions with endpoint
// flags that are evaluated with the current event. Literal(true) and
// Literal(false) model Esper's constant four-argument form, while
// VariableRef[bool] supplies runtime endpoint flags.
func DateTimeBetweenWithEndpoints(value, lower, upper Expr, lowerInclusive, upperInclusive Expression[bool]) Expression[bool] {
	return dateTimeBetweenExpression("date-time-between-endpoints", value, lower, upper, lowerInclusive, upperInclusive)
}

// DateTimeBetweenRangeOf is the static-flag counterpart of
// DateTimeBetweenWithEndpoints. It is useful when the endpoint policy is part
// of a reusable rule definition rather than a runtime variable.
func DateTimeBetweenRangeOf(value, lower, upper Expr, lowerInclusive, upperInclusive bool) Expression[bool] {
	return DateTimeBetweenWithEndpoints(value, lower, upper, Literal(lowerInclusive), Literal(upperInclusive))
}

// DateTimeAfter reports whether value is strictly after bound. Null and
// missing operands produce Null, matching Esper's boxed Boolean result.
func DateTimeAfter(value, bound Expr) Expression[bool] {
	children := []*exprNode{nil, nil}
	if value != nil {
		children[0] = value.node()
	}
	if bound != nil {
		children[1] = bound.node()
	}
	node := &exprNode{
		kind:        "date-time-after",
		typ:         typeOf[bool](),
		description: fmt.Sprintf("(%s after %s)", expressionDescription(value), expressionDescription(bound)),
		children:    children,
	}
	validateDateTimeOperand(node, "value", value)
	if node.configurationError == "" {
		validateDateTimeOperand(node, "bound", bound)
	}
	return typedExpr[bool]{n: node, fn: func(ctx EvalContext) Value {
		if value == nil || bound == nil {
			return Null()
		}
		left, ok := dateTimeEpochMillis(value.eval(ctx))
		if !ok {
			return Null()
		}
		right, ok := dateTimeEpochMillis(bound.eval(ctx))
		if !ok {
			return Null()
		}
		return Present(left > right)
	}}
}

// BetweenDateTimeOf and BetweenDateTimeRangeOf are descriptive aliases for
// callers that prefer the operation-first naming used by other expressions.
func BetweenDateTimeOf(value, lower, upper Expr) Expression[bool] {
	return DateTimeBetween(value, lower, upper)
}

func BetweenDateTimeRangeOf(value, lower, upper Expr, lowerInclusive, upperInclusive bool) Expression[bool] {
	return DateTimeBetweenRangeOf(value, lower, upper, lowerInclusive, upperInclusive)
}

func dateTimeBetweenExpression(kind string, value, lower, upper Expr, lowerInclusive, upperInclusive Expr) Expression[bool] {
	children := make([]*exprNode, 0, 5)
	for _, operand := range []Expr{value, lower, upper, lowerInclusive, upperInclusive} {
		if operand == nil {
			children = append(children, nil)
			continue
		}
		children = append(children, operand.node())
	}
	node := &exprNode{
		kind:        kind,
		typ:         typeOf[bool](),
		description: fmt.Sprintf("%s(%s,%s,%s,%s,%s)", kind, expressionDescription(value), expressionDescription(lower), expressionDescription(upper), expressionDescription(lowerInclusive), expressionDescription(upperInclusive)),
		children:    children,
	}
	for index, operand := range []Expr{value, lower, upper} {
		if operand == nil || operand.node() == nil {
			node.configurationError = fmt.Sprintf("%s operand %d is required", kind, index)
			break
		}
		validateDateTimeOperand(node, fmt.Sprintf("operand %d", index), operand)
		if node.configurationError != "" {
			break
		}
	}
	if node.configurationError == "" {
		validateDateTimeEndpoint(node, "lower endpoint", lowerInclusive)
	}
	if node.configurationError == "" {
		validateDateTimeEndpoint(node, "upper endpoint", upperInclusive)
	}

	// Esper's constant-parameter forge normalizes reversed bounds whenever all
	// four parameters are compile-time constants. The non-constant forge also
	// normalizes the inclusive two-argument form; mixed endpoint flags retain
	// operand order unless both bounds are literal constants.
	normalizeBounds := isLiteralTrue(lowerInclusive) && isLiteralTrue(upperInclusive)
	if !normalizeBounds && isLiteralDateTime(lower) && isLiteralDateTime(upper) &&
		isLiteralBoolean(lowerInclusive) && isLiteralBoolean(upperInclusive) {
		normalizeBounds = true
	}
	return typedExpr[bool]{n: node, fn: func(ctx EvalContext) Value {
		if value == nil || lower == nil || upper == nil || lowerInclusive == nil || upperInclusive == nil {
			return Null()
		}
		current, ok := dateTimeEpochMillis(value.eval(ctx))
		if !ok {
			return Null()
		}
		low, ok := dateTimeEpochMillis(lower.eval(ctx))
		if !ok {
			return Null()
		}
		high, ok := dateTimeEpochMillis(upper.eval(ctx))
		if !ok {
			return Null()
		}
		includeLow, ok := dateTimeEndpointValue(lowerInclusive.eval(ctx))
		if !ok {
			return Null()
		}
		includeHigh, ok := dateTimeEndpointValue(upperInclusive.eval(ctx))
		if !ok {
			return Null()
		}
		if normalizeBounds {
			if low > high {
				low, high = high, low
			}
			return Present((current > low || (includeLow && current == low)) &&
				(current < high || (includeHigh && current == high)))
		}
		return Present((current > low || (includeLow && current == low)) &&
			(current < high || (includeHigh && current == high)))
	}}
}

func validateDateTimeOperand(node *exprNode, label string, operand Expr) {
	if operand == nil || operand.node() == nil {
		return
	}
	if operand.node().kind != "null" && !isDateTimeOperandType(operand.Type()) {
		node.configurationError = fmt.Sprintf("date-time %s must be epoch milliseconds or time.Time, got %s", label, mathTypeDescription(operand.Type()))
	}
}

func validateDateTimeEndpoint(node *exprNode, label string, endpoint Expr) {
	if endpoint == nil || endpoint.node() == nil {
		node.configurationError = fmt.Sprintf("date-time %s is required", label)
		return
	}
	typ := endpoint.Type()
	if typ != nil && typ != typeOf[any]() && typ.Kind() != reflect.Bool && endpoint.node().kind != "null" {
		node.configurationError = fmt.Sprintf("date-time %s must be boolean, got %s", label, mathTypeDescription(typ))
	}
}

func isDateTimeOperandType(typ reflect.Type) bool {
	if typ == nil || typ == typeOf[any]() {
		return true
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		if typ == nil {
			return false
		}
	}
	if typ == reflect.TypeOf(time.Time{}) {
		return true
	}
	if typ.Kind() == reflect.Interface {
		return true
	}
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}

func dateTimeEndpointValue(value Value) (bool, bool) {
	if !value.IsPresent() {
		return false, false
	}
	return boolValue(value)
}

func isLiteralTrue(expression Expr) bool {
	value, ok := literalBoolean(expression)
	return ok && value
}

func isLiteralBoolean(expression Expr) bool {
	_, ok := literalBoolean(expression)
	return ok
}

func literalBoolean(expression Expr) (bool, bool) {
	if expression == nil || expression.node() == nil || expression.node().kind != "literal" {
		return false, false
	}
	value, ok := expression.node().literalValue.(bool)
	return value, ok
}

func isLiteralDateTime(expression Expr) bool {
	if expression == nil || expression.node() == nil || expression.node().kind != "literal" {
		return false
	}
	_, ok := dateTimeEpochMillis(Present(expression.node().literalValue))
	return ok
}

func dateTimeEpochMillis(value Value) (int64, bool) {
	if !value.IsPresent() || isNilReflectValue(reflect.ValueOf(value.Any())) {
		return 0, false
	}
	reflected := reflect.ValueOf(value.Any())
	for reflected.IsValid() && (reflected.Kind() == reflect.Pointer || reflected.Kind() == reflect.Interface) {
		if reflected.IsNil() {
			return 0, false
		}
		reflected = reflected.Elem()
	}
	if !reflected.IsValid() {
		return 0, false
	}
	if reflected.Type() == reflect.TypeOf(time.Time{}) {
		return reflected.Interface().(time.Time).UnixMilli(), true
	}
	switch reflected.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflected.Int(), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value := reflected.Uint()
		if value > math.MaxInt64 {
			return 0, false
		}
		return int64(value), true
	default:
		return 0, false
	}
}
