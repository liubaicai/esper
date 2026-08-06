package esper

import (
	"fmt"
	"reflect"
	"strings"
)

// ArrayOf constructs a typed array literal from analyzable element
// expressions. T is explicit so a rule author can choose the result element
// type when operands have different native Go numeric types; use any or a
// pointer element type when the array must retain Null elements.
func ArrayOf[T any](values ...Expr) Expression[[]T] {
	return arrayLiteral[T](values...)
}

// ArrayLiteral is a descriptive alias for ArrayOf.
func ArrayLiteral[T any](values ...Expr) Expression[[]T] {
	return ArrayOf[T](values...)
}

func arrayLiteral[T any](values ...Expr) Expression[[]T] {
	children := make([]*exprNode, 0, len(values))
	descriptions := make([]string, 0, len(values))
	node := &exprNode{
		kind:        "array-literal",
		typ:         typeOf[[]T](),
		description: fmt.Sprintf("array<%s>(%s)", typeOf[T](), strings.Join(descriptions, ",")),
		children:    children,
	}
	for index, value := range values {
		if value == nil || value.node() == nil {
			node.children = append(node.children, nil)
			node.configurationError = fmt.Sprintf("array literal element %d is required", index)
			descriptions = append(descriptions, "<nil>")
			continue
		}
		node.children = append(node.children, value.node())
		descriptions = append(descriptions, value.Description())
		if !arrayElementTypeCompatible(value.Type(), typeOf[T]()) {
			node.configurationError = fmt.Sprintf("array literal element %d has type %s, which is not assignable to %s", index, arrayTypeDescription(value.Type()), typeOf[T]())
			continue
		}
		if value.node().kind == "null" && !isNilableType(typeOf[T]()) {
			node.configurationError = fmt.Sprintf("array literal element %d is null but %s cannot represent null", index, typeOf[T]())
		}
	}
	node.description = fmt.Sprintf("array<%s>(%s)", typeOf[T](), strings.Join(descriptions, ","))
	return typedExpr[[]T]{n: node, fn: func(ctx EvalContext) Value {
		result := make([]T, 0, len(values))
		for _, value := range values {
			if value == nil {
				return Null()
			}
			item, ok := arrayElementResult[T](value.eval(ctx))
			if !ok {
				return Null()
			}
			result = append(result, item)
		}
		return Present(result)
	}}
}

// ArraySize returns the number of elements in an array or slice. A typed nil
// slice has length zero; a Null/Missing/non-array value produces Null.
func ArraySize(values Expr) Expression[int64] {
	return arraySizeExpression("array-size", values)
}

// ArrayLength is an alias for ArraySize for callers that prefer the Go
// collection vocabulary.
func ArrayLength(values Expr) Expression[int64] {
	return ArraySize(values)
}

func arraySizeExpression(kind string, values Expr) Expression[int64] {
	node := &exprNode{kind: kind, typ: typeOf[int64](), description: "array-size(<nil>)"}
	if values == nil || values.node() == nil {
		node.configurationError = "array size requires an array operand"
	} else {
		node.description = "array-size(" + values.Description() + ")"
		node.children = []*exprNode{values.node()}
		_, collection, dynamic := arrayTypeInfo(values.Type())
		if !collection && !dynamic {
			node.configurationError = fmt.Sprintf("array size operand must be an array or slice, got %s", arrayTypeDescription(values.Type()))
		}
	}
	return typedExpr[int64]{n: node, fn: func(ctx EvalContext) Value {
		if values == nil {
			return Null()
		}
		raw, ok := arrayReflectValue(values.eval(ctx))
		if !ok {
			return Null()
		}
		return Present(int64(raw.Len()))
	}}
}

// ArrayElementAt is the general indexed-array constructor. ArrayAt is kept
// as the concise spelling used by existing rules.
func ArrayElementAt[T any](values, index Expr) Expression[T] {
	return arrayElementAt[T](values, index)
}

func arrayElementAt[T any](values, index Expr) Expression[T] {
	node := &exprNode{
		kind:        "array-at",
		typ:         typeOf[T](),
		description: fmt.Sprintf("array-at<%s>(%s,%s)", typeOf[T](), expressionDescription(values), expressionDescription(index)),
	}
	if values == nil || values.node() == nil {
		node.configurationError = "array access requires an array operand"
	} else {
		node.children = append(node.children, values.node())
		elementType, collection, dynamic := arrayTypeInfo(values.Type())
		if !collection && !dynamic {
			node.configurationError = fmt.Sprintf("array access operand must be an array or slice, got %s", arrayTypeDescription(values.Type()))
		} else if collection && !arrayElementTypeCompatible(elementType, typeOf[T]()) {
			node.configurationError = fmt.Sprintf("array access result %s is incompatible with element type %s", typeOf[T](), arrayTypeDescription(elementType))
		}
	}
	if index == nil || index.node() == nil {
		if node.configurationError == "" {
			node.configurationError = "array access requires an index expression"
		}
		node.children = append(node.children, nil)
	} else {
		node.children = append(node.children, index.node())
		if !arrayIndexType(index.Type()) {
			node.configurationError = fmt.Sprintf("array access index must be an integer, got %s", arrayTypeDescription(index.Type()))
		}
	}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		if values == nil || index == nil {
			return Null()
		}
		position, ok := arrayIndexValue(index.eval(ctx))
		if !ok {
			return Null()
		}
		raw, ok := arrayReflectValue(values.eval(ctx))
		if !ok || position < 0 || position >= int64(raw.Len()) {
			return Null()
		}
		return arrayElementValue[T](raw.Index(int(position)))
	}}
}

func arrayTypeInfo(typ reflect.Type) (element reflect.Type, collection, dynamic bool) {
	if typ == nil || typ == typeOf[any]() {
		return nil, false, true
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		if typ == nil {
			return nil, false, false
		}
	}
	if typ.Kind() == reflect.Interface {
		return nil, false, true
	}
	if typ.Kind() != reflect.Array && typ.Kind() != reflect.Slice {
		return nil, false, false
	}
	return typ.Elem(), true, false
}

func arrayIndexType(typ reflect.Type) bool {
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
	return isIntegralType(typ)
}

func arrayElementTypeCompatible(source, target reflect.Type) bool {
	if source == nil || target == nil || source == typeOf[any]() || target == typeOf[any]() {
		return true
	}
	if source.AssignableTo(target) || (target.Kind() == reflect.Interface && source.Implements(target)) {
		return true
	}
	if numericTypes(source, target) {
		return true
	}
	if source.ConvertibleTo(target) && source.Kind() == target.Kind() {
		return true
	}
	if source.Kind() == reflect.Pointer && target.Kind() != reflect.Pointer {
		return arrayElementTypeCompatible(source.Elem(), target)
	}
	return false
}

func arrayTypeDescription(typ reflect.Type) string {
	if typ == nil {
		return "any"
	}
	return typ.String()
}

func arrayReflectValue(value Value) (reflect.Value, bool) {
	if !value.IsPresent() {
		return reflect.Value{}, false
	}
	raw := reflect.ValueOf(value.Any())
	for raw.IsValid() && (raw.Kind() == reflect.Interface || raw.Kind() == reflect.Pointer) {
		if raw.IsNil() {
			return reflect.Value{}, false
		}
		raw = raw.Elem()
	}
	if !raw.IsValid() || (raw.Kind() != reflect.Array && raw.Kind() != reflect.Slice) {
		return reflect.Value{}, false
	}
	return raw, true
}

func arrayIndexValue(value Value) (int64, bool) {
	if !value.IsPresent() {
		return 0, false
	}
	raw := reflect.ValueOf(value.Any())
	for raw.IsValid() && (raw.Kind() == reflect.Interface || raw.Kind() == reflect.Pointer) {
		if raw.IsNil() {
			return 0, false
		}
		raw = raw.Elem()
	}
	if !raw.IsValid() {
		return 0, false
	}
	switch raw.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		position := raw.Int()
		return position, position >= 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		position := raw.Uint()
		maxInt64 := uint64(^uint64(0) >> 1)
		if position > maxInt64 {
			return 0, false
		}
		return int64(position), true
	default:
		return 0, false
	}
}

func arrayElementValue[T any](value reflect.Value) Value {
	if !value.IsValid() || !value.CanInterface() {
		return Missing()
	}
	if wrapped, ok := value.Interface().(Value); ok {
		return arrayElementResultValue[T](wrapped)
	}
	if isNilReflectValue(value) {
		return arrayElementResultValue[T](Null())
	}
	return arrayElementResultValue[T](Present(value.Interface()))
}

func arrayElementResult[T any](value Value) (T, bool) {
	var zero T
	if !value.IsPresent() {
		if isNilableType(typeOf[T]()) {
			return zero, true
		}
		return zero, false
	}
	if isNilReflectValue(reflect.ValueOf(value.Any())) {
		if isNilableType(typeOf[T]()) {
			return zero, true
		}
		return zero, false
	}
	converted := castPropertyValue[T](value)
	if !converted.IsPresent() {
		if isNilableType(typeOf[T]()) && converted.IsNull() {
			return zero, true
		}
		return zero, false
	}
	result, err := As[T](converted)
	if err != nil {
		return zero, false
	}
	return result, true
}

func arrayElementResultValue[T any](value Value) Value {
	result, ok := arrayElementResult[T](value)
	if !ok {
		return Null()
	}
	if isNilReflectValue(reflect.ValueOf(result)) {
		return Null()
	}
	return Present(result)
}
