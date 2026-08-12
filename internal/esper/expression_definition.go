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
	Name       string
	Expr       Expr
	Parameters []ExpressionParameterSpec
}

// ExpressionParameterSpec describes one parameter of a named expression in
// declaration order.  The type is carried by the AST node created with
// ExpressionParam, so callers do not need reflection or string signatures.
type ExpressionParameterSpec struct {
	Name string
	Type reflect.Type
}

// expressionParameterBinding keeps a named-expression call argument lazy.
// Aggregate expressions evaluate their input expression once for every event
// in the current group; evaluating the argument before entering that loop
// would incorrectly bind the whole declaration to the call's representative
// event.
type expressionParameterBinding struct {
	expression Expr
	context    EvalContext
	hasContext bool
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
	parameters, err := expressionParameterSpecs(expression)
	if err != nil {
		return WrapError(ErrorInvalidRule, fmt.Sprintf("expression definition %q", name), err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.expressions[name]; exists {
		return duplicateModuleObjectError(DeploymentResourceExpression, name)
	}
	e.expressions[name] = ExpressionDefinition{Name: name, Expr: expression, Parameters: parameters}
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
	if ok {
		definition.Parameters = append([]ExpressionParameterSpec(nil), definition.Parameters...)
	}
	return definition, ok
}

// ExpressionRef creates a typed reference to a registered named expression.
// Arguments are optional for a no-argument definition and are substituted for
// ExpressionParam nodes for a parameterized definition.  The reference keeps
// the caller's event, join tuple, aggregate state, variables, parameters and
// engine, so a definition containing a subquery remains correlated to its
// surrounding statement.
func ExpressionRef[T any](env *Environment, name string, arguments ...Expr) Expression[T] {
	name = strings.TrimSpace(name)
	children := make([]*exprNode, 0, len(arguments))
	for _, argument := range arguments {
		if argument == nil {
			children = append(children, nil)
			continue
		}
		children = append(children, argument.node())
	}
	description := name + "()"
	if len(arguments) > 0 {
		parts := make([]string, 0, len(arguments))
		for _, argument := range arguments {
			parts = append(parts, expressionDescription(argument))
		}
		description = name + "(" + strings.Join(parts, ",") + ")"
	}
	var body *exprNode
	if env != nil {
		if definition, ok := env.Expression(name); ok && definition.Expr != nil && definition.Expr.node() != nil {
			body = definition.Expr.node()
		}
	}
	node := &exprNode{
		kind:                  "expression-ref",
		typ:                   typeOf[T](),
		description:           description,
		expressionName:        name,
		expressionEnvironment: env,
		expressionArguments:   children,
		expressionBody:        body,
		children:              append([]*exprNode(nil), children...),
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
		if len(arguments) != len(definition.Parameters) {
			return Missing()
		}
		if len(arguments) == 0 {
			return castValue[T](definition.Expr.eval(ctx))
		}
		bindings := make(map[string]Value, len(arguments))
		// A scalar declared expression evaluates its arguments in the lexical
		// call-site scope, even when its body enters a correlated subquery. An
		// aggregate declaration must remain lazy so Sum/Count can evaluate the
		// argument once for each event in the aggregate group.
		captureArgumentContext := !expressionNodeContainsAggregate(definition.Expr.node())
		for index, argument := range arguments {
			if argument == nil {
				return Missing()
			}
			bindings[definition.Parameters[index].Name] = Present(expressionParameterBinding{
				expression: argument,
				context:    ctx,
				hasContext: captureArgumentContext,
			})
		}
		invocationContext := ctx
		parameters := make(map[string]Value, len(ctx.Parameters)+len(bindings))
		for parameterName, value := range ctx.Parameters {
			parameters[parameterName] = value
		}
		for parameterName, value := range bindings {
			parameters[parameterName] = value
		}
		invocationContext.Parameters = parameters
		return castValue[T](definition.Expr.eval(invocationContext))
	}}
}

// expressionParameterSpecs discovers named-expression parameters in source
// order.  A nested ExpressionRef is intentionally treated as a call node and
// is not expanded here; its parameters belong to the nested definition, not
// to the enclosing definition.
func expressionParameterSpecs(expression Expr) ([]ExpressionParameterSpec, error) {
	if expression == nil || expression.node() == nil {
		return nil, nil
	}
	result := make([]ExpressionParameterSpec, 0)
	byName := make(map[string]int)
	var visitExpr func(Expr) error
	var visitNode func(*exprNode) error
	visitExpr = func(value Expr) error {
		if value == nil {
			return nil
		}
		return visitNode(value.node())
	}
	visitNode = func(node *exprNode) error {
		if node == nil {
			return nil
		}
		if node.kind == "expression-parameter" {
			name := strings.TrimSpace(node.parameterName)
			if name == "" {
				return NewError(ErrorInvalidRule, "named expression parameter name is required")
			}
			if index, exists := byName[name]; exists {
				previous := result[index].Type
				if !expressionTypesCompatible(previous, node.typ) {
					return NewError(ErrorTypeMismatch, fmt.Sprintf("named expression parameter %q has incompatible types %s and %s", name, previous, node.typ))
				}
				if previous == typeOf[any]() && node.typ != typeOf[any]() {
					result[index].Type = node.typ
				}
			} else {
				byName[name] = len(result)
				result = append(result, ExpressionParameterSpec{Name: name, Type: node.typ})
			}
		}
		for _, child := range node.children {
			if err := visitNode(child); err != nil {
				return err
			}
		}
		if node.subquery != nil {
			if err := visitExpr(node.subquery.predicate); err != nil {
				return err
			}
			if err := visitExpr(node.subquery.projection); err != nil {
				return err
			}
			for _, selection := range node.subquery.columns {
				if err := visitExpr(selection.Expr); err != nil {
					return err
				}
			}
			if err := visitExpr(node.subquery.groupBy); err != nil {
				return err
			}
			if err := visitExpr(node.subquery.having); err != nil {
				return err
			}
			for _, order := range node.subquery.orderBy {
				if err := visitExpr(order.Expression); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visitNode(expression.node()); err != nil {
		return nil, err
	}
	return result, nil
}

func expressionTypesCompatible(expected, actual reflect.Type) bool {
	if expected == nil || actual == nil {
		return true
	}
	return expected == actual || expected.AssignableTo(actual) || actual.AssignableTo(expected) || numericTypes(expected, actual)
}

func evaluateExpressionParameterBinding(value Value, ctx EvalContext) Value {
	for depth := 0; depth < 32; depth++ {
		binding, ok := value.Any().(expressionParameterBinding)
		if !ok {
			return value
		}
		if binding.expression == nil {
			return Missing()
		}
		evaluation := ctx
		if binding.hasContext {
			evaluation = binding.context
		}
		value = binding.expression.eval(evaluation)
	}
	return Missing()
}
