package esper

import (
	"reflect"
	"strings"
)

// This file compiles a stateless plan's proven-pure predicate chain into
// specialized closures. The compiler never re-implements expression
// semantics: every operation reuses the exact generic function (EqualValues,
// compareValues, As, exactNumericCompare, equalValuesUnwrapped, the string
// helpers) and every field read reuses the same structFieldTable candidate
// paths the generic Schema.get walk resolves. What is removed is only the
// per-eval name resolution and closure-graph dispatch: the candidate index
// paths are resolved once at plan build, and one closure per node is replaced
// by one straight-line call per event.
//
// Any node the compiler does not recognize makes the whole chain fall back to
// the generic evaluation; no partial compilation exists, so a compiled chain
// and the generic chain evaluate exactly the same operations in exactly the
// same order on the same Values.

// statelessValueFn evaluates one proven-pure expression node against an event
// carrying the plan's source schema.
type statelessValueFn func(Event) Value

// compileStatelessChain compiles every predicate of a stateless plan, or
// returns nil when any predicate contains a node outside the compiled set.
func compileStatelessChain(schema Schema, predicates []Expr) []statelessValueFn {
	if len(predicates) == 0 {
		return nil
	}
	compiled := make([]statelessValueFn, len(predicates))
	for index, predicate := range predicates {
		fn := compileStatelessExpression(schema, predicate.node())
		if fn == nil {
			return nil
		}
		compiled[index] = fn
	}
	return compiled
}

// statelessCompiledMatch evaluates the compiled predicate chain exactly as
// the generic filter walk does: every predicate must yield a present true.
func statelessCompiledMatch(compiled []statelessValueFn, event Event) bool {
	for _, evaluate := range compiled {
		matched, ok := boolValue(evaluate(event))
		if !ok || !matched {
			return false
		}
	}
	return true
}

// compileStatelessExpression mirrors the generic eval closure of one pure
// expression node. The returned closure is semantically identical to
// evaluating the node through its typedExpr closure for events carrying the
// source schema; it returns nil when the node kind is not compiled.
func compileStatelessExpression(schema Schema, node *exprNode) statelessValueFn {
	if node == nil {
		return nil
	}
	switch node.kind {
	case "literal":
		boxed := Present(node.literalValue)
		return func(Event) Value { return boxed }
	case "null":
		return func(Event) Value { return Null() }
	case "field":
		return compileStatelessField(schema, node)
	case "and", "or":
		if len(node.children) != 2 {
			return nil
		}
		left := compileStatelessExpression(schema, node.children[0])
		right := compileStatelessExpression(schema, node.children[1])
		if left == nil || right == nil {
			return nil
		}
		if node.kind == "and" {
			return func(event Event) Value {
				leftValue := left(event)
				leftBool, leftOK := boolValue(leftValue)
				if leftOK && !leftBool {
					return Present(false)
				}
				rightValue := right(event)
				rightBool, rightOK := boolValue(rightValue)
				if rightOK && !rightBool {
					return Present(false)
				}
				if leftOK && rightOK {
					return Present(leftBool && rightBool)
				}
				return Null()
			}
		}
		return func(event Event) Value {
			leftValue := left(event)
			leftBool, leftOK := boolValue(leftValue)
			if leftOK && leftBool {
				return Present(true)
			}
			rightValue := right(event)
			rightBool, rightOK := boolValue(rightValue)
			if rightOK && rightBool {
				return Present(true)
			}
			if leftOK && rightOK {
				return Present(leftBool || rightBool)
			}
			return Null()
		}
	case "not":
		if len(node.children) != 1 {
			return nil
		}
		operand := compileStatelessExpression(schema, node.children[0])
		if operand == nil {
			return nil
		}
		return func(event Event) Value {
			result, ok := boolValue(operand(event))
			if !ok {
				return Null()
			}
			return Present(!result)
		}
	case "eq":
		return compileStatelessBinary(schema, node.children, EqualValues)
	case "neq":
		return compileStatelessBinary(schema, node.children, func(l, r Value) Value {
			result := EqualValues(l, r)
			if !result.IsPresent() {
				return result
			}
			return Present(!result.Any().(bool))
		})
	case "gt", "gte", "lt", "lte":
		var matches func(int) bool
		switch node.kind {
		case "gt":
			matches = func(c int) bool { return c > 0 }
		case "gte":
			matches = func(c int) bool { return c >= 0 }
		case "lt":
			matches = func(c int) bool { return c < 0 }
		default:
			matches = func(c int) bool { return c <= 0 }
		}
		return compileStatelessBinary(schema, node.children, func(l, r Value) Value {
			comparison, ok := compareValues(l, r)
			if !ok {
				return Null()
			}
			return Present(matches(comparison))
		})
	case "exact-lt", "exact-lte", "exact-gt", "exact-gte":
		var matches func(int) bool
		switch node.kind {
		case "exact-lt":
			matches = func(c int) bool { return c < 0 }
		case "exact-lte":
			matches = func(c int) bool { return c <= 0 }
		case "exact-gt":
			matches = func(c int) bool { return c > 0 }
		default:
			matches = func(c int) bool { return c >= 0 }
		}
		return compileStatelessBinary(schema, node.children, func(l, r Value) Value {
			comparison, ok := exactNumericCompare(l, r)
			if !ok {
				return Null()
			}
			return Present(matches(comparison))
		})
	case "contains", "starts-with", "ends-with":
		var operation func(string, string) bool
		switch node.kind {
		case "contains":
			operation = strings.Contains
		case "starts-with":
			operation = strings.HasPrefix
		default:
			operation = strings.HasSuffix
		}
		return compileStatelessBinary(schema, node.children, func(l, r Value) Value {
			left, leftErr := As[string](l)
			right, rightErr := As[string](r)
			if leftErr != nil || rightErr != nil {
				return Null()
			}
			return Present(operation(left, right))
		})
	case "in":
		return compileStatelessIn(schema, node)
	case "is-null", "is-missing":
		if len(node.children) != 1 {
			return nil
		}
		operand := compileStatelessExpression(schema, node.children[0])
		if operand == nil {
			return nil
		}
		if node.kind == "is-null" {
			return func(event Event) Value { return Present(operand(event).IsNull()) }
		}
		return func(event Event) Value { return Present(operand(event).IsMissing()) }
	case "udf":
		return compileStatelessBuiltin(schema, node)
	default:
		return nil
	}
}

// compileStatelessBinary compiles a binary operation over two child operands.
func compileStatelessBinary(schema Schema, children []*exprNode, operation func(Value, Value) Value) statelessValueFn {
	if len(children) != 2 {
		return nil
	}
	left := compileStatelessExpression(schema, children[0])
	right := compileStatelessExpression(schema, children[1])
	if left == nil || right == nil {
		return nil
	}
	return func(event Event) Value {
		return operation(left(event), right(event))
	}
}

// compileStatelessIn mirrors the generic In closure: null value or all-null
// candidates produce Null, membership uses the unwrapped equality contract.
// The multi-slot delivery form is already excluded from stateless plans.
func compileStatelessIn(schema Schema, node *exprNode) statelessValueFn {
	if len(node.children) < 2 {
		return nil
	}
	operands := make([]statelessValueFn, len(node.children))
	for index, child := range node.children {
		fn := compileStatelessExpression(schema, child)
		if fn == nil {
			return nil
		}
		operands[index] = fn
	}
	current := operands[0]
	candidates := operands[1:]
	return func(event Event) Value {
		value := current(event)
		if !value.IsPresent() {
			return Null()
		}
		hasNull := false
		for _, candidate := range candidates {
			other := candidate(event)
			if !other.IsPresent() {
				hasNull = true
				continue
			}
			if equalValuesUnwrapped(value, other) {
				return Present(true)
			}
		}
		if hasNull {
			return Null()
		}
		return Present(false)
	}
}

// compileStatelessBuiltin compiles the four engine-built pure string
// functions. Only these constructors set pureBuiltin, and the function is
// identified by the description prefix those constructors produce; any other
// shape stays generic.
func compileStatelessBuiltin(schema Schema, node *exprNode) statelessValueFn {
	if !node.pureBuiltin || len(node.children) != 1 {
		return nil
	}
	var operation func(string) any
	switch {
	case strings.HasPrefix(node.description, "lower("):
		operation = func(input string) any { return strings.ToLower(input) }
	case strings.HasPrefix(node.description, "upper("):
		operation = func(input string) any { return strings.ToUpper(input) }
	case strings.HasPrefix(node.description, "trim("):
		operation = func(input string) any { return strings.TrimSpace(input) }
	case strings.HasPrefix(node.description, "length("):
		operation = func(input string) any { return int64(len([]rune(input))) }
	default:
		return nil
	}
	argument := compileStatelessExpression(schema, node.children[0])
	if argument == nil {
		return nil
	}
	// The generic Func1 closure converts the argument with As[string],
	// yields Null for absent or non-convertible input, and never panics for
	// these four functions, so its recover wrapper cannot fire.
	return func(event Event) Value {
		input := argument(event)
		if !input.IsPresent() {
			return Null()
		}
		value, err := As[string](input)
		if err != nil {
			return Null()
		}
		return Present(operation(value))
	}
}

// compileStatelessField compiles a plain-name field read on the plan's source
// schema. The generic path resolves Schema.get -> getOne ->
// structFieldValue(table.lookup(canonicalName)); because the plan already
// proved the name is declared on this schema with no registered getters, the
// compiled read performs the same candidate walk over the precomputed index
// paths and the same Value wrapping. Representations the struct walk cannot
// serve (nil, Value/Row/Event shells, non-struct values) mirror getOne's own
// branches or fall back to the event's generic Get.
func compileStatelessField(schema Schema, node *exprNode) statelessValueFn {
	_, canonical, err := schema.lookupField(node.fieldName)
	if err != nil {
		return nil
	}
	structType := schema.goType
	if structType == nil {
		return nil
	}
	for structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
	}
	if structType.Kind() != reflect.Struct {
		return nil
	}
	paths := structFieldTableFor(structType).lookupPaths(canonical, schema.resolution)
	if len(paths) == 0 {
		return nil
	}
	name := node.fieldName
	return func(event Event) Value {
		underlying := event.underlying
		if underlying == nil {
			return Null()
		}
		if wrapped, ok := underlying.(Value); ok {
			if !wrapped.IsPresent() {
				return wrapped
			}
			underlying = wrapped.Any()
			if underlying == nil {
				return Null()
			}
		}
		if row, ok := underlying.(Row); ok {
			return row.Get(name)
		}
		if nested, ok := underlying.(Event); ok {
			return nested.Get(name)
		}
		value := reflect.ValueOf(underlying)
		for value.Kind() == reflect.Pointer {
			if value.IsNil() {
				return Null()
			}
			value = value.Elem()
		}
		if value.Kind() != reflect.Struct {
			return event.Get(name)
		}
		for _, path := range paths {
			field, ok := resolveStructFieldPath(value, path)
			if !ok {
				continue
			}
			if !field.CanInterface() {
				return Missing()
			}
			if (field.Kind() == reflect.Pointer || field.Kind() == reflect.Interface) && field.IsNil() {
				return Null()
			}
			return Present(field.Interface())
		}
		return Missing()
	}
}
