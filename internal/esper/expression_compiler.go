package esper

import (
	"fmt"
	"reflect"
	"time"
)

// CompiledExpressionContext supplies the optional runtime state visible to
// a standalone compiled expression. Constant expressions use the zero value.
type CompiledExpressionContext struct {
	Event     Event
	Engine    *Engine
	Now       time.Time
	Variables map[string]Value
}

// CompiledExpression is an immutable, validated typed expression artifact.
// It retains the ordinary Plan produced by the same fluent AST so callers can
// inspect stable canonical identity without depending on internal exprNode or
// evaluator/forge implementation types.
type CompiledExpression[T any] struct {
	expression Expression[T]
	plan       Plan
}

// CompileExpression validates a typed standalone expression against env.
// Event-field references are rejected because the expression has no source;
// registered variables, scripts, enum/date-time plugins and declared
// expressions use the same validation path as a source-less fluent Plan.
func CompileExpression[T any](env *Environment, expression Expression[T]) (CompiledExpression[T], error) {
	if env == nil {
		return CompiledExpression[T]{}, NewError(ErrorInvalidRule, "nil environment")
	}
	if expression == nil || expression.node() == nil {
		return CompiledExpression[T]{}, NewError(ErrorInvalidRule, "standalone expression is required")
	}
	plan, err := env.Build(SelectOnce(env, Alias("value", expression)))
	if err != nil {
		return CompiledExpression[T]{}, WrapError(ErrorInvalidRule, "compile expression", err)
	}
	return CompiledExpression[T]{expression: expression, plan: plan}, nil
}

// Type reports the typed result metadata.
func (c CompiledExpression[T]) Type() reflect.Type {
	if c.expression == nil {
		return nil
	}
	return c.expression.Type()
}

// Description returns the stable fluent AST description.
func (c CompiledExpression[T]) Description() string {
	if c.expression == nil {
		return ""
	}
	return c.expression.Description()
}

// Plan returns the immutable source-less Plan used for validation and
// canonical identity.
func (c CompiledExpression[T]) Plan() Plan { return c.plan }

// Evaluate evaluates with an empty event/runtime context.
func (c CompiledExpression[T]) Evaluate() (T, ValueState, error) {
	return c.EvaluateWith(CompiledExpressionContext{})
}

// EvaluateWith evaluates against an explicit detached context. Missing and
// Null preserve their ValueState and return the zero T; a present value is
// checked against T before it is returned.
func (c CompiledExpression[T]) EvaluateWith(context CompiledExpressionContext) (result T, state ValueState, err error) {
	if c.expression == nil || c.expression.node() == nil || c.plan.SchemaVersion() == "" {
		return result, ValueMissing, NewError(ErrorState, "compiled expression is empty")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			result = *new(T)
			state = ValueMissing
			err = NewError(ErrorState, fmt.Sprintf("standalone expression evaluation panicked: %v", recovered))
		}
	}()
	value := c.expression.eval(EvalContext{
		Event:      context.Event,
		OuterEvent: context.Event,
		Engine:     context.Engine,
		Now:        context.Now,
		Variables:  cloneValues(context.Variables),
	})
	state = value.State()
	if !value.IsPresent() {
		return result, state, nil
	}
	result, err = As[T](value)
	if err != nil {
		return *new(T), state, WrapError(ErrorTypeMismatch, "compiled expression result", err)
	}
	return result, state, nil
}
