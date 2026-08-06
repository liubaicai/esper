package esper

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

// AggregateMultiPluginMethod describes one chainable accessor exposed by a
// multi-function aggregate provider. StateKey is the provider-defined state
// identity: methods with the same key share one state for a group, while
// different keys receive independent states.
type AggregateMultiPluginMethod struct {
	Name       string
	ResultType reflect.Type
	StateKey   string
}

// AggregateMultiMethod creates method metadata with a typed result. When no
// state key is supplied, each expression instance receives an independent
// state. Supplying the same non-empty key for multiple methods is the Go
// equivalent of Esper's AggregationMultiFunctionStateKey sharing contract.
func AggregateMultiMethod[T any](name string, sharedStateKey ...string) AggregateMultiPluginMethod {
	stateKey := ""
	if len(sharedStateKey) > 0 {
		stateKey = strings.TrimSpace(sharedStateKey[0])
	}
	return AggregateMultiPluginMethod{
		Name:       strings.TrimSpace(name),
		ResultType: typeOf[T](),
		StateKey:   stateKey,
	}
}

// AggregateMultiPluginFactoryContext is the immutable context supplied when a
// multi-function provider creates one state for one aggregate group.
type AggregateMultiPluginFactoryContext struct {
	Provider   string
	Event      Event
	Engine     *Engine
	Now        time.Time
	Variables  map[string]Value
	Parameters map[string]Value
	Methods    map[string]AggregateMultiPluginMethod
}

// AggregateMultiPluginState is the lifecycle contract for a shared
// multi-function aggregate state. Each Enter/Leave call receives the values
// for one retained event. For an omitted input expression the vector contains
// the current Event; AggregatePluginInputs is passed as its declared vector.
type AggregateMultiPluginState interface {
	Enter(values []Value)
	Leave(values []Value)
	Value(method string) (any, bool)
	Clear()
}

// AggregateMultiPluginFactory creates one independent shared state per
// aggregate group and state key.
type AggregateMultiPluginFactory func(AggregateMultiPluginFactoryContext) AggregateMultiPluginState

type aggregateMultiPluginDefinition struct {
	methods map[string]AggregateMultiPluginMethod
	factory aggregateMultiPluginFactory
}

type aggregateMultiPluginFactory func(AggregateMultiPluginFactoryContext) aggregateMultiPluginState

type aggregateMultiPluginState interface {
	sync([][]Value)
	value(string) (Value, bool)
	clear()
}

func aggregateExpressionScope(parent, child string) string {
	child = strings.TrimSpace(child)
	if parent == "" {
		return child
	}
	if child == "" {
		return parent
	}
	return parent + ">" + child
}

type aggregateMultiPluginStateAdapter struct {
	state       AggregateMultiPluginState
	inputs      [][]Value
	initialized bool
}

func (a *aggregateMultiPluginStateAdapter) sync(inputs [][]Value) {
	if a == nil || a.state == nil {
		return
	}
	if a.initialized {
		for _, values := range a.inputs {
			a.state.Leave(append([]Value(nil), values...))
		}
	} else {
		a.state.Clear()
	}
	for _, values := range inputs {
		a.state.Enter(append([]Value(nil), values...))
	}
	a.inputs = cloneAggregateMultiInputs(inputs)
	a.initialized = true
}

func (a *aggregateMultiPluginStateAdapter) value(method string) (Value, bool) {
	if a == nil || a.state == nil {
		return Missing(), false
	}
	value, present := a.state.Value(method)
	if !present {
		return Null(), false
	}
	return Present(value), true
}

func (a *aggregateMultiPluginStateAdapter) clear() {
	if a == nil || a.state == nil {
		return
	}
	a.state.Clear()
	a.inputs = nil
	a.initialized = false
}

func adaptAggregateMultiPluginFactory(factory AggregateMultiPluginFactory, methods map[string]AggregateMultiPluginMethod) aggregateMultiPluginFactory {
	if factory == nil {
		return nil
	}
	return func(ctx AggregateMultiPluginFactoryContext) aggregateMultiPluginState {
		ctx.Methods = cloneAggregateMultiPluginMethods(methods)
		state := factory(ctx)
		if state == nil {
			return nil
		}
		return &aggregateMultiPluginStateAdapter{state: state}
	}
}

func cloneAggregateMultiInputs(inputs [][]Value) [][]Value {
	if inputs == nil {
		return nil
	}
	clone := make([][]Value, len(inputs))
	for index, values := range inputs {
		clone[index] = append([]Value(nil), values...)
	}
	return clone
}

func cloneAggregateMultiPluginMethods(methods map[string]AggregateMultiPluginMethod) map[string]AggregateMultiPluginMethod {
	if methods == nil {
		return nil
	}
	clone := make(map[string]AggregateMultiPluginMethod, len(methods))
	for name, method := range methods {
		clone[name] = method
	}
	return clone
}

func normalizeAggregateMultiPluginMethods(provider string, declarations []AggregateMultiPluginMethod) (map[string]AggregateMultiPluginMethod, error) {
	if len(declarations) == 0 {
		return nil, NewError(ErrorInvalidRule, fmt.Sprintf("aggregate multi plugin %q requires at least one method", provider))
	}
	methods := make(map[string]AggregateMultiPluginMethod, len(declarations))
	for index, declaration := range declarations {
		name := strings.TrimSpace(declaration.Name)
		if name == "" {
			return nil, NewError(ErrorInvalidRule, fmt.Sprintf("aggregate multi plugin %q method %d requires a name", provider, index))
		}
		if declaration.ResultType == nil {
			return nil, NewError(ErrorInvalidRule, fmt.Sprintf("aggregate multi plugin %q method %q requires a result type", provider, name))
		}
		if _, exists := methods[name]; exists {
			return nil, NewError(ErrorDependency, fmt.Sprintf("aggregate multi plugin %q method %q is declared more than once", provider, name))
		}
		declaration.Name = name
		declaration.StateKey = strings.TrimSpace(declaration.StateKey)
		methods[name] = declaration
	}
	return methods, nil
}

// RegisterAggregateMultiPlugin registers a named multi-function aggregate
// provider in an Environment. The provider is explicit and typed at each
// expression call site; no Java class loading or EPL parser is involved.
func RegisterAggregateMultiPlugin(env *Environment, provider string, methods []AggregateMultiPluginMethod, factory AggregateMultiPluginFactory) error {
	if env == nil {
		return NewError(ErrorDependency, "nil environment")
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return NewError(ErrorInvalidRule, "aggregate multi plugin provider is required")
	}
	if factory == nil {
		return NewError(ErrorInvalidRule, fmt.Sprintf("aggregate multi plugin %q has no factory", provider))
	}
	normalized, err := normalizeAggregateMultiPluginMethods(provider, methods)
	if err != nil {
		return err
	}
	env.mu.Lock()
	defer env.mu.Unlock()
	if _, exists := env.aggregatePlugins[provider]; exists {
		return NewError(ErrorDependency, fmt.Sprintf("aggregate multi plugin %q conflicts with aggregate plugin", provider))
	}
	if _, exists := env.aggregateMultiPlugins[provider]; exists {
		return NewError(ErrorDependency, fmt.Sprintf("aggregate multi plugin %q is already registered", provider))
	}
	env.aggregateMultiPlugins[provider] = aggregateMultiPluginDefinition{
		methods: normalized,
		factory: adaptAggregateMultiPluginFactory(factory, normalized),
	}
	return nil
}

// PluginAggregateMulti creates an inline multi-function aggregate expression.
// The same factory can expose several methods and share state by giving their
// AggregateMultiMethod declarations the same StateKey.
func PluginAggregateMulti[T any](provider, method string, input Expr, factory AggregateMultiPluginFactory, methods ...AggregateMultiPluginMethod) AggregateExpression[T] {
	node := newAggregateMultiPluginNode("aggregate-multi-plugin", provider, method, input, nil, methods)
	node.typ = typeOf[T]()
	node.aggregateMultiPluginFactory = adaptAggregateMultiPluginFactory(factory, node.aggregateMultiMethods)
	node.aggregateMultiPluginReady = factory != nil
	return aggregateExpr[T]{typedExpr: typedExpr[T]{
		n: node,
		fn: func(ctx EvalContext) Value {
			return evaluateAggregateMultiPlugin(ctx, node, input, node.aggregateMultiPluginFactory)
		},
	}}
}

// PluginAggregateMultiRef creates a typed reference to a registered
// multi-function provider and one of its named accessors.
func PluginAggregateMultiRef[T any](env *Environment, provider, method string, input Expr) AggregateExpression[T] {
	node := newAggregateMultiPluginNode("aggregate-multi-plugin-ref", provider, method, input, env, nil)
	node.typ = typeOf[T]()
	return aggregateExpr[T]{typedExpr: typedExpr[T]{
		n: node,
		fn: func(ctx EvalContext) Value {
			if env == nil {
				return Missing()
			}
			env.mu.RLock()
			definition, ok := env.aggregateMultiPlugins[node.aggregateMultiPluginName]
			env.mu.RUnlock()
			if !ok {
				return Missing()
			}
			_, ok = definition.methods[node.aggregateMultiPluginMethod]
			if !ok {
				return Missing()
			}
			return evaluateAggregateMultiPlugin(ctx, node, input, definition.factory)
		},
	}}
}

func newAggregateMultiPluginNode(kind, provider, method string, input Expr, env *Environment, methods []AggregateMultiPluginMethod) *exprNode {
	provider = strings.TrimSpace(provider)
	method = strings.TrimSpace(method)
	node := &exprNode{
		kind:                            kind,
		typ:                             typeOf[any](),
		description:                     "aggregate-multi-plugin(<invalid>)",
		aggregateMultiPluginName:        provider,
		aggregateMultiPluginMethod:      method,
		aggregateMultiPluginEnvironment: env,
		aggregateMultiStateKey:          method,
	}
	if provider != "" && method != "" {
		node.description = aggregateMultiPluginDescription(provider, method, input, nil)
	}
	if input != nil {
		node.children = []*exprNode{input.node()}
	}
	if env != nil {
		env.mu.RLock()
		if definition, ok := env.aggregateMultiPlugins[provider]; ok {
			node.description = aggregateMultiPluginDescription(provider, method, input, definition.methods)
			if methodDefinition, exists := definition.methods[method]; exists {
				node.aggregateMultiStateKey = methodDefinition.StateKey
				node.aggregateMultiStateShared = methodDefinition.StateKey != ""
			}
		}
		env.mu.RUnlock()
	}
	if methods != nil {
		if normalized, err := normalizeAggregateMultiPluginMethods(provider, methods); err == nil {
			node.aggregateMultiMethods = normalized
			node.description = aggregateMultiPluginDescription(provider, method, input, normalized)
			if methodDefinition, exists := normalized[method]; exists {
				node.aggregateMultiStateKey = methodDefinition.StateKey
				node.aggregateMultiStateShared = methodDefinition.StateKey != ""
			}
		} else {
			node.configurationError = err.Error()
		}
	}
	return node
}

func aggregateMultiPluginDescription(provider, method string, input Expr, methods map[string]AggregateMultiPluginMethod) string {
	description := "aggregate-multi-plugin(" + provider + "." + method
	if methodDefinition, ok := methods[method]; ok {
		stateKey := methodDefinition.StateKey
		if stateKey == "" {
			stateKey = "<isolated>"
		}
		description += fmt.Sprintf(":%s:%s", methodDefinition.ResultType, stateKey)
	}
	if input != nil {
		description += "," + input.Description()
	}
	description += ")"
	return description
}

func evaluateAggregateMultiPlugin(ctx EvalContext, node *exprNode, input Expr, factory aggregateMultiPluginFactory) Value {
	if node == nil || factory == nil || strings.TrimSpace(node.aggregateMultiPluginMethod) == "" {
		return Missing()
	}
	stateKey := node.aggregateMultiStateKey
	if node.aggregateMultiStateShared && stateKey != "" {
		// Explicit state keys follow Esper's provider-key contract: accessors
		// with the same provider key and local-group scope share one state even
		// when their parameter expressions differ.
		stateKey = "shared:" + stateKey
	} else {
		// A provider normally returns a fresh state key for an unshared
		// aggregation invocation. The expression node is the stable Go-native
		// identity for that invocation.
		stateKey = fmt.Sprintf("isolated:%p", node)
	}
	key := node.aggregateMultiPluginName + "|" + stateKey + "|" + ctx.aggregateMultiScope
	state := aggregateMultiPluginState(nil)
	if ctx.aggregateMultiPluginStates != nil {
		state = ctx.aggregateMultiPluginStates[key]
	}
	if state == nil {
		state = factory(AggregateMultiPluginFactoryContext{
			Provider:   node.aggregateMultiPluginName,
			Event:      ctx.Event,
			Engine:     ctx.Engine,
			Now:        ctx.Now,
			Variables:  ctx.Variables,
			Parameters: ctx.Parameters,
		})
		if state == nil {
			return Missing()
		}
		if ctx.aggregateMultiPluginStates != nil {
			ctx.aggregateMultiPluginStates[key] = state
		}
	}
	inputs := make([][]Value, 0, len(ctx.Group))
	for _, event := range ctx.Group {
		if input == nil {
			inputs = append(inputs, []Value{Present(event)})
			continue
		}
		inputContext := EvalContext{
			Event:                event,
			JoinEvents:           joinTupleEvents(event),
			OuterEvent:           ctx.OuterEvent,
			ContainedParentEvent: ctx.ContainedParentEvent,
			Engine:               ctx.Engine,
			Now:                  ctx.Now,
			Variables:            ctx.Variables,
			Parameters:           ctx.Parameters,
		}
		value := input.eval(inputContext)
		if input.node() != nil && input.node().kind == "aggregate-plugin-inputs" && value.IsPresent() {
			if vector, ok := value.Any().([]Value); ok {
				inputs = append(inputs, append([]Value(nil), vector...))
				continue
			}
		}
		inputs = append(inputs, []Value{value})
	}
	state.sync(inputs)
	value, present := state.value(node.aggregateMultiPluginMethod)
	if !present {
		return Null()
	}
	return value
}

func aggregateMultiPluginMethodsCanonical(methods map[string]AggregateMultiPluginMethod) string {
	if len(methods) == 0 {
		return ""
	}
	names := make([]string, 0, len(methods))
	for name := range methods {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		method := methods[name]
		stateKey := method.StateKey
		if stateKey == "" {
			stateKey = "<isolated>"
		}
		parts = append(parts, fmt.Sprintf("%s:%s:%s", name, method.ResultType, stateKey))
	}
	return strings.Join(parts, ",")
}
