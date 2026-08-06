package esper

import (
	"fmt"
	"reflect"
	"strings"
)

// StructOf constructs a Go-native anonymous record expression.
//
// The result is an ordered, analyzable map whose field names and child
// expressions are retained in the plan. A fresh map is materialized for each
// evaluation, so downstream expressions can safely enrich or inspect the
// result without mutating a prior row.
func StructOf(fields ...Selection) Expression[map[string]any] {
	children := make([]*exprNode, 0, len(fields))
	descriptions := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	invalid := ""
	for _, field := range fields {
		name := field.Name
		if strings.TrimSpace(name) == "" {
			if invalid == "" {
				invalid = "struct field name is required"
			}
		}
		if _, exists := seen[name]; exists {
			if invalid == "" {
				invalid = fmt.Sprintf("struct field %q is declared more than once", name)
			}
		}
		seen[name] = struct{}{}
		if field.Expr == nil || field.Expr.node() == nil {
			if invalid == "" {
				invalid = fmt.Sprintf("struct field %q requires an expression", name)
			}
			children = append(children, nil)
			descriptions = append(descriptions, fmt.Sprintf("%q=<nil>", name))
			continue
		}
		children = append(children, field.Expr.node())
		descriptions = append(descriptions, fmt.Sprintf("%q=%s", name, field.Expr.Description()))
	}
	node := &exprNode{
		kind:               "struct",
		typ:                typeOf[map[string]any](),
		description:        "struct{" + strings.Join(descriptions, ",") + "}",
		children:           children,
		configurationError: invalid,
	}
	return typedExpr[map[string]any]{n: node, fn: func(ctx EvalContext) Value {
		if invalid != "" {
			return Null()
		}
		result := make(map[string]any, len(fields))
		for _, field := range fields {
			value := field.Expr.eval(ctx)
			if !value.IsPresent() || isNilReflectValue(reflect.ValueOf(value.Any())) {
				result[field.Name] = nil
				continue
			}
			result[field.Name] = value.Any()
		}
		return Present(result)
	}}
}

// StructField is a readable helper for constructing StructOf fields without
// conflating record members with a query projection at the call site.
func StructField(name string, expression Expr) Selection {
	return Selection{Name: name, Expr: expression}
}
