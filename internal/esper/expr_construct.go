package esper

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
)

// ValueConstructor is the explicit Go extension point used by Construct.
// Value states are preserved so a factory can distinguish Missing from Null
// instead of relying on an untyped nil argument.
type ValueConstructor[T any] func([]Value) (T, error)

// Construct invokes a named, typed Go factory with analyzable expression
// arguments. The name is part of the plan identity and replaces Java's
// class/constructor lookup, which is intentionally not reproduced through
// reflection or runtime code loading.
func Construct[T any](name string, factory ValueConstructor[T], arguments ...Expr) Expression[T] {
	children := make([]*exprNode, len(arguments))
	argumentDescriptions := make([]string, len(arguments))
	invalid := ""
	if strings.TrimSpace(name) == "" {
		invalid = "constructor name is required"
	} else if factory == nil {
		invalid = fmt.Sprintf("constructor %q requires a factory", name)
	}
	for index, argument := range arguments {
		if argument == nil || argument.node() == nil {
			if invalid == "" {
				invalid = fmt.Sprintf("constructor %q argument %d is required", name, index)
			}
			argumentDescriptions[index] = "<nil>"
			continue
		}
		children[index] = argument.node()
		argumentDescriptions[index] = argument.Description()
	}
	description := "construct(" + name
	if len(argumentDescriptions) > 0 {
		description += ";" + strings.Join(argumentDescriptions, ",")
	}
	description += ")"
	node := &exprNode{
		kind:               "construct",
		typ:                typeOf[T](),
		description:        description,
		methodName:         name,
		children:           children,
		configurationError: invalid,
	}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) (result Value) {
		if invalid != "" || factory == nil {
			return Null()
		}
		defer func() {
			if recover() != nil {
				result = Null()
			}
		}()
		values := make([]Value, len(arguments))
		for index, argument := range arguments {
			values[index] = argument.eval(ctx)
		}
		value, err := factory(values)
		if err != nil || isNilReflectValue(reflect.ValueOf(value)) {
			return Null()
		}
		return Present(value)
	}}
}

// MakeArray constructs a zero-initialized Go slice from a runtime dimension.
// It is the Go-style counterpart of a one-dimensional `new T[n]` expression.
func MakeArray[T any](length Expr) Expression[[]T] {
	node := &exprNode{kind: "make-array", typ: typeOf[[]T](), description: "make-array(<nil>)", children: []*exprNode{nil}}
	if length == nil || length.node() == nil {
		node.configurationError = "array length expression is required"
	} else {
		node.children[0] = length.node()
		node.description = "make-array(" + length.Description() + ")"
		if !constructorIntegralType(length.Type()) {
			node.configurationError = fmt.Sprintf("array length must be integral, got %s", constructorTypeDescription(length.Type()))
		}
	}
	return typedExpr[[]T]{n: node, fn: func(ctx EvalContext) Value {
		if length == nil {
			return Null()
		}
		count, ok := constructorLengthValue(length.eval(ctx))
		if !ok {
			return Null()
		}
		return Present(make([]T, count))
	}}
}

// MakeArray2D constructs a zero-initialized two-dimensional slice. Each
// dimension is evaluated at event time and both dimensions participate in the
// logical plan.
func MakeArray2D[T any](rows, columns Expr) Expression[[][]T] {
	node := &exprNode{kind: "make-array-2d", typ: typeOf[[][]T](), description: "make-array-2d(<nil>,<nil>)", children: []*exprNode{nil, nil}}
	if rows == nil || rows.node() == nil {
		node.configurationError = "array row length expression is required"
	} else if columns == nil || columns.node() == nil {
		node.configurationError = "array column length expression is required"
	} else if !constructorIntegralType(rows.Type()) {
		node.configurationError = fmt.Sprintf("array row length must be integral, got %s", constructorTypeDescription(rows.Type()))
	} else if !constructorIntegralType(columns.Type()) {
		node.configurationError = fmt.Sprintf("array column length must be integral, got %s", constructorTypeDescription(columns.Type()))
	}
	if rows != nil && rows.node() != nil {
		node.children[0] = rows.node()
	}
	if columns != nil && columns.node() != nil {
		node.children[1] = columns.node()
	}
	if rows != nil && columns != nil && rows.node() != nil && columns.node() != nil {
		node.description = "make-array-2d(" + rows.Description() + "," + columns.Description() + ")"
	}
	return typedExpr[[][]T]{n: node, fn: func(ctx EvalContext) Value {
		if rows == nil || columns == nil {
			return Null()
		}
		rowCount, rowOK := constructorLengthValue(rows.eval(ctx))
		columnCount, columnOK := constructorLengthValue(columns.eval(ctx))
		if !rowOK || !columnOK {
			return Null()
		}
		result := make([][]T, rowCount)
		for index := range result {
			result[index] = make([]T, columnCount)
		}
		return Present(result)
	}}
}

func constructorIntegralType(typ reflect.Type) bool {
	if typ == nil || typ == typeOf[any]() {
		return true
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		if typ == nil {
			return false
		}
	}
	if typ.Kind() == reflect.Interface || typ == reflect.TypeOf(big.Int{}) {
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

func constructorLengthValue(value Value) (int, bool) {
	number, ok := enumRatFromValue(value)
	if !ok || !number.IsInt() || number.Sign() < 0 || !number.Num().IsInt64() {
		return 0, false
	}
	requested := number.Num().Int64()
	maxInt := int64(^uint(0) >> 1)
	if requested > maxInt {
		return 0, false
	}
	return int(requested), true
}

func constructorTypeDescription(typ reflect.Type) string {
	if typ == nil {
		return "any"
	}
	return typ.String()
}
