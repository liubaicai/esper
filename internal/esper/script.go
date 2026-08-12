package esper

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// ScriptContext is the Go-native invocation boundary for a registered
// script. Evaluation is the surrounding expression scope and Arguments
// preserves Missing and Null as distinct Value states.
type ScriptContext struct {
	Name         string
	Dialect      string
	Environment  *Environment
	Evaluation   EvalContext
	Arguments    []Value
	attributes   map[string]Value
	attributesMu *sync.RWMutex
}

// Argument returns an argument snapshot or Missing for an out-of-range index.
func (s ScriptContext) Argument(index int) Value {
	if index < 0 || index >= len(s.Arguments) {
		return Missing()
	}
	return s.Arguments[index]
}

// GetScriptAttribute reads a provider-local attribute.
func (s ScriptContext) GetScriptAttribute(name string) (Value, bool) {
	if s.attributes == nil {
		return Missing(), false
	}
	if s.attributesMu != nil {
		s.attributesMu.RLock()
		defer s.attributesMu.RUnlock()
	}
	value, ok := s.attributes[strings.TrimSpace(name)]
	return value, ok
}

// SetScriptAttribute updates a provider-local attribute. A zero context has
// no backing store and therefore ignores the write.
func (s ScriptContext) SetScriptAttribute(name string, value any) {
	if s.attributes == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name != "" {
		if s.attributesMu != nil {
			s.attributesMu.Lock()
			defer s.attributesMu.Unlock()
		}
		s.attributes[name] = Present(value)
	}
}

// ScriptProvider is the Value-oriented execution boundary for a registered
// script. It is useful for dynamic result types and for providers that want to
// preserve Null/Missing explicitly.
type ScriptProvider func(ScriptContext) (Value, error)

type scriptDefinition struct {
	dialect          string
	resultType       reflect.Type
	argumentTypes    []reflect.Type
	argumentTypesSet bool
	provider         ScriptProvider
	attributes       map[string]Value
	attributesMu     *sync.RWMutex
}

// ScriptOption configures a registered script's declarative metadata.
type ScriptOption func(*scriptDefinition)

// ScriptDialect records a descriptive provider dialect in the plan identity.
// It does not load a JavaScript/JVM runtime; the provider remains an explicit
// Go callback.
func ScriptDialect(dialect string) ScriptOption {
	return func(definition *scriptDefinition) {
		definition.dialect = strings.TrimSpace(dialect)
	}
}

// WithScriptDialect is the explicit-option spelling for ScriptDialect.
func WithScriptDialect(dialect string) ScriptOption { return ScriptDialect(dialect) }

// ScriptArgumentTypes declares expected argument expression types. Omitting
// this option leaves arity/type checking to the provider.
func ScriptArgumentTypes(types ...reflect.Type) ScriptOption {
	return func(definition *scriptDefinition) {
		definition.argumentTypes = append([]reflect.Type(nil), types...)
		definition.argumentTypesSet = true
	}
}

// WithScriptArgumentTypes is the explicit-option spelling for
// ScriptArgumentTypes.
func WithScriptArgumentTypes(types ...reflect.Type) ScriptOption {
	return ScriptArgumentTypes(types...)
}

func registerScriptDefinition(env *Environment, name string, definition scriptDefinition) error {
	if env == nil {
		return NewError(ErrorInvalidRule, "nil environment")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return NewError(ErrorInvalidRule, "script name is required")
	}
	if strings.TrimSpace(definition.dialect) == "" {
		return NewError(ErrorInvalidRule, "script dialect is required")
	}
	if definition.provider == nil {
		return NewError(ErrorInvalidRule, fmt.Sprintf("script %q provider is required", name))
	}
	if definition.resultType == nil {
		definition.resultType = typeOf[any]()
	}
	if definition.attributes == nil {
		definition.attributes = make(map[string]Value)
	}
	if definition.attributesMu == nil {
		definition.attributesMu = &sync.RWMutex{}
	}
	env.mu.Lock()
	defer env.mu.Unlock()
	if _, exists := env.scripts[name]; exists {
		return duplicateScriptError(name, definition)
	}
	env.scripts[name] = definition
	return nil
}

// DefineScript registers the EvalContext-oriented typed provider used by
// declarative expression definitions. Provider errors become Null during
// expression evaluation, matching Esper's nullable script result boundary.
func DefineScript[T any](env *Environment, name, dialect string, provider func(EvalContext, []Value) (T, error), options ...ScriptOption) error {
	if provider == nil {
		return NewError(ErrorInvalidRule, fmt.Sprintf("script %q provider is required", name))
	}
	definition := scriptDefinition{
		dialect:    strings.TrimSpace(dialect),
		resultType: typeOf[T](),
		provider: func(script ScriptContext) (Value, error) {
			result, err := provider(script.Evaluation, script.Arguments)
			if err != nil {
				return Null(), err
			}
			return Present(result), nil
		},
	}
	for _, option := range options {
		if option != nil {
			option(&definition)
		}
	}
	return registerScriptDefinition(env, name, definition)
}

// RegisterScript registers a typed ScriptContext provider. The provider
// parameter is intentionally accepted as any so legacy typed providers using
// func(EvalContext, []Value) (T, error) can be migrated without introducing a
// second public script name; both forms remain statically checked inside the
// registration boundary.
func RegisterScript[T any](env *Environment, name, dialect string, provider any, options ...ScriptOption) error {
	definition := scriptDefinition{dialect: strings.TrimSpace(dialect), resultType: typeOf[T]()}
	switch typed := provider.(type) {
	case func(ScriptContext) (T, error):
		if typed == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("script %q provider is required", name))
		}
		definition.provider = func(script ScriptContext) (Value, error) {
			result, err := typed(script)
			if err != nil {
				return Null(), err
			}
			return Present(result), nil
		}
	case func(EvalContext, []Value) (T, error):
		if typed == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("script %q provider is required", name))
		}
		definition.provider = func(script ScriptContext) (Value, error) {
			result, err := typed(script.Evaluation, script.Arguments)
			if err != nil {
				return Null(), err
			}
			return Present(result), nil
		}
	case ScriptProvider:
		if typed == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("script %q provider is required", name))
		}
		definition.provider = typed
	case func(ScriptContext) (Value, error):
		if typed == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("script %q provider is required", name))
		}
		definition.provider = typed
	case func(EvalContext, []Value) (Value, error):
		if typed == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("script %q provider is required", name))
		}
		definition.provider = func(script ScriptContext) (Value, error) {
			return typed(script.Evaluation, script.Arguments)
		}
	default:
		return NewError(ErrorInvalidRule, fmt.Sprintf("script %q provider has unsupported signature", name))
	}
	for _, option := range options {
		if option != nil {
			option(&definition)
		}
	}
	return registerScriptDefinition(env, name, definition)
}

// RegisterValueScript registers an already Value-oriented provider. It is
// the non-generic escape hatch for dynamic result types.
func RegisterValueScript(env *Environment, name, dialect string, resultType reflect.Type, provider ScriptProvider, options ...ScriptOption) error {
	definition := scriptDefinition{
		dialect:    strings.TrimSpace(dialect),
		resultType: resultType,
		provider:   provider,
	}
	for _, option := range options {
		if option != nil {
			option(&definition)
		}
	}
	return registerScriptDefinition(env, name, definition)
}

// ScriptCall creates an analyzable call to a registered script. Arguments are
// evaluated left-to-right and passed as Value snapshots to the provider.
func ScriptCall[T any](env *Environment, name string, arguments ...Expr) Expression[T] {
	name = strings.TrimSpace(name)
	children := make([]*exprNode, 0, len(arguments))
	parts := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		if argument == nil {
			parts = append(parts, "<nil>")
			children = append(children, nil)
			continue
		}
		parts = append(parts, argument.Description())
		children = append(children, argument.node())
	}
	description := "script(" + name + "(" + strings.Join(parts, ",") + "))"
	node := &exprNode{
		kind:              "script",
		typ:               typeOf[T](),
		description:       description,
		scriptName:        name,
		scriptEnvironment: env,
		children:          children,
	}
	if env == nil {
		node.configurationError = "script requires an environment"
	}
	if name == "" {
		node.configurationError = "script name is required"
	}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		if env == nil || name == "" {
			return Missing()
		}
		env.mu.RLock()
		definition, ok := env.scripts[name]
		env.mu.RUnlock()
		if !ok || definition.provider == nil {
			return Missing()
		}
		args := make([]Value, 0, len(arguments))
		for _, argument := range arguments {
			if argument == nil {
				args = append(args, Missing())
				continue
			}
			args = append(args, argument.eval(ctx))
		}
		script := ScriptContext{
			Name:         name,
			Dialect:      definition.dialect,
			Environment:  env,
			Evaluation:   ctx,
			Arguments:    append([]Value(nil), args...),
			attributes:   definition.attributes,
			attributesMu: definition.attributesMu,
		}
		value, err := definition.provider(script)
		if err != nil {
			return Null()
		}
		return castValue[T](value)
	}}
}

// Script is the concise expression spelling for ScriptCall.
func Script[T any](env *Environment, name string, arguments ...Expr) Expression[T] {
	return ScriptCall[T](env, name, arguments...)
}
