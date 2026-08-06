package esper

import (
	"fmt"
	"reflect"
	"strings"
)

// ExpressionDefinition is an immutable, no-argument named expression in an
// Environment. It is the Go-native counterpart of an Esper expression
// declaration; the expression itself remains an analyzable Go AST rather
// than an EPL string or an opaque callback.
type ExpressionDefinition struct {
	Name string
	Expr Expr
}

// DefineExpression registers a named expression in the environment. Define
// expressions before creating ExpressionRef nodes so their dependency tree
// can be captured in the plan during Build. Definitions are immutable after
// registration and duplicate names are rejected.
func (e *Environment) DefineExpression(name string, expression Expr) error {
	if e == nil {
		return NewError(ErrorInvalidRule, "nil environment")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return NewError(ErrorInvalidRule, "expression definition name is required")
	}
	if expression == nil || expression.node() == nil {
		return NewError(ErrorInvalidRule, fmt.Sprintf("expression definition %q requires an expression", name))
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.expressions[name]; exists {
		return NewError(ErrorInvalidRule, fmt.Sprintf("expression definition %q is already registered", name))
	}
	e.expressions[name] = ExpressionDefinition{Name: name, Expr: expression}
	return nil
}

// DefineExpression is the typed convenience form for callers that prefer a
// top-level registration function in a fluent rule module.
func DefineExpression[T any](env *Environment, name string, expression Expression[T]) error {
	return env.DefineExpression(name, expression)
}

// Expression returns a registered named expression definition.
func (e *Environment) Expression(name string) (ExpressionDefinition, bool) {
	if e == nil {
		return ExpressionDefinition{}, false
	}
	name = strings.TrimSpace(name)
	e.mu.RLock()
	defer e.mu.RUnlock()
	definition, ok := e.expressions[name]
	return definition, ok
}

// ExpressionRef creates a typed reference to a registered no-argument
// expression. The reference evaluates the definition with the caller's
// current event, aggregate group, variables, parameters and engine, so a
// subquery definition remains correlated to its surrounding statement.
func ExpressionRef[T any](env *Environment, name string) Expression[T] {
	name = strings.TrimSpace(name)
	children := make([]*exprNode, 0, 1)
	if env != nil {
		if definition, ok := env.Expression(name); ok && definition.Expr != nil && definition.Expr.node() != nil {
			children = append(children, definition.Expr.node())
		}
	}
	node := &exprNode{
		kind:                  "expression-ref",
		typ:                   typeOf[T](),
		description:           name + "()",
		expressionName:        name,
		expressionEnvironment: env,
		children:              children,
	}
	if env == nil {
		node.configurationError = "expression reference requires an environment"
	}
	if name == "" {
		node.configurationError = "expression reference name is required"
	}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		if env == nil || name == "" {
			return Missing()
		}
		definition, ok := env.Expression(name)
		if !ok || definition.Expr == nil {
			return Missing()
		}
		return castValue[T](definition.Expr.eval(ctx))
	}}
}

func expressionTypesCompatible(expected, actual reflect.Type) bool {
	if expected == nil || actual == nil {
		return true
	}
	return expected == actual || expected.AssignableTo(actual) || actual.AssignableTo(expected) || numericTypes(expected, actual)
}
