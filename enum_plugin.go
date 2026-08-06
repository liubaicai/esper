package esper

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
)

// EnumPluginContext contains the type metadata available when an
// enumeration-plugin state is created. The state is created once per
// evaluation and must not be shared between evaluations.
type EnumPluginContext struct {
	Name        string
	ElementType reflect.Type
	ResultType  reflect.Type
	Environment *Environment
	Evaluation  EvalContext
}

// EnumPluginState is the Go-native lifecycle contract for one enumeration
// method state. Parameters are supplied once before iteration. Add receives
// the current item and the values of the declared lambda arguments. A state
// may stop iteration early by returning true from Completed.
type EnumPluginState[T any] interface {
	SetParameter(index int, value Value)
	Add(item Value, lambdaValues []Value)
	Completed() bool
	Result() (T, bool)
}

// EnumPluginFactory is the typed state factory used by RegisterEnumPlugin and
// EnumPlugin. It is explicit and stable; no Java class loading or reflection
// is involved in the Go rule plan.
type EnumPluginFactory[T any] func(EnumPluginContext) EnumPluginState[T]

type enumPluginState interface {
	setParameter(index int, value Value)
	add(item Value, lambdaValues []Value)
	completed() bool
	result() (Value, bool)
}

type enumPluginFactory func(EnumPluginContext) enumPluginState

type enumPluginStateAdapter[T any] struct {
	state EnumPluginState[T]
}

func (state *enumPluginStateAdapter[T]) setParameter(index int, value Value) {
	state.state.SetParameter(index, value)
}

func (state *enumPluginStateAdapter[T]) add(item Value, lambdaValues []Value) {
	state.state.Add(item, lambdaValues)
}

func (state *enumPluginStateAdapter[T]) completed() bool { return state.state.Completed() }

func (state *enumPluginStateAdapter[T]) result() (Value, bool) {
	if state == nil || state.state == nil {
		return Missing(), false
	}
	value, present := state.state.Result()
	if !present {
		return Null(), false
	}
	return Present(value), true
}

func adaptEnumPluginFactory[T any](factory EnumPluginFactory[T]) enumPluginFactory {
	if factory == nil {
		return nil
	}
	return func(context EnumPluginContext) enumPluginState {
		state := factory(context)
		if state == nil {
			return nil
		}
		return &enumPluginStateAdapter[T]{state: state}
	}
}

type enumPluginDefinition struct {
	resultType reflect.Type
	footprints []EnumMethodFootprint
	factory    enumPluginFactory
}

type enumPluginArgumentMeta struct {
	typ                  reflect.Type
	lambda               bool
	lambdaParameterCount int
}

// EnumPluginArgument marks a normal method argument or a lambda body. The
// wrapper is deliberate: a literal expression can still be a valid lambda
// body, so inferring lambda-ness from AST references would be ambiguous.
type EnumPluginArgument struct {
	expression Expr
	lambda     bool
	arity      int
}

func EnumArgument(expression Expr) EnumPluginArgument {
	return EnumPluginArgument{expression: expression}
}

func EnumLambda1(expression Expr) EnumPluginArgument {
	return EnumPluginArgument{expression: expression, lambda: true, arity: 1}
}

func EnumLambda2(expression Expr) EnumPluginArgument {
	return EnumPluginArgument{expression: expression, lambda: true, arity: 2}
}

func EnumLambda3(expression Expr) EnumPluginArgument {
	return EnumPluginArgument{expression: expression, lambda: true, arity: 3}
}

// EnumLambda4 marks an expression as a four-parameter enumeration lambda.
// The runtime passes all lambda values as an ordered slice, so extensions may
// use the same marker for custom footprints beyond the built-in forms.
func EnumLambda4(expression Expr) EnumPluginArgument {
	return EnumPluginArgument{expression: expression, lambda: true, arity: 4}
}

// RegisterEnumPlugin registers a named enumeration method and its accepted
// Java-style method footprints. The method is invoked through
// EnumPluginRef; callers still construct the surrounding rule with typed Go
// expressions.
func RegisterEnumPlugin[T any](env *Environment, name string, footprints []EnumMethodFootprint, factory EnumPluginFactory[T]) error {
	if env == nil {
		return NewError(ErrorDependency, "nil environment")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return NewError(ErrorInvalidRule, "enumeration plugin name is required")
	}
	if len(footprints) == 0 {
		return NewError(ErrorInvalidRule, fmt.Sprintf("enumeration plugin %q requires at least one footprint", name))
	}
	if factory == nil {
		return NewError(ErrorInvalidRule, fmt.Sprintf("enumeration plugin %q has no factory", name))
	}
	if err := validateEnumPluginFootprintDeclarations(name, footprints); err != nil {
		return err
	}
	env.mu.Lock()
	defer env.mu.Unlock()
	if _, exists := env.enumPlugins[name]; exists {
		return NewError(ErrorDependency, fmt.Sprintf("enumeration plugin %q is already registered", name))
	}
	if _, exists := env.aggregatePlugins[name]; exists {
		return NewError(ErrorDependency, fmt.Sprintf("enumeration plugin %q conflicts with aggregate plugin", name))
	}
	env.enumPlugins[name] = enumPluginDefinition{
		resultType: typeOf[T](),
		footprints: cloneEnumMethodFootprints(footprints),
		factory:    adaptEnumPluginFactory(factory),
	}
	return nil
}

// EnumPluginRef creates a typed reference to a registered enumeration method.
// The first type parameter is the collection component type; the second is
// the plugin result type.
func EnumPluginRef[I, R any](env *Environment, name string, values Expression[[]I], arguments ...EnumPluginArgument) Expression[R] {
	name = strings.TrimSpace(name)
	node, _, metadata := newEnumPluginNode[I, R]("enum-plugin-ref", name, env, values, arguments)
	if env != nil && name != "" {
		env.mu.RLock()
		definition, registered := env.enumPlugins[name]
		env.mu.RUnlock()
		if registered {
			node.enumPluginFootprints = cloneEnumMethodFootprints(definition.footprints)
			metadata.Footprints = cloneEnumMethodFootprints(definition.footprints)
			node.enumMetadata = &metadata
		}
	}
	if env == nil {
		node.configurationError = "enumeration plugin reference requires an environment"
	}
	if name == "" {
		node.configurationError = "enumeration plugin name is required"
	}
	return typedExpr[R]{n: node, fn: func(ctx EvalContext) (result Value) {
		defer func() {
			if recover() != nil {
				result = Null()
			}
		}()
		if env == nil || name == "" || values == nil {
			return Missing()
		}
		env.mu.RLock()
		definition, ok := env.enumPlugins[name]
		env.mu.RUnlock()
		if !ok || definition.factory == nil {
			return Missing()
		}
		if len(metadata.Footprints) == 0 {
			metadata = EnumMethodMetadata{Method: name, CollectionType: values.Type(), ElementType: typeOf[I](), ResultType: typeOf[R](), Footprints: cloneEnumMethodFootprints(definition.footprints)}
			node.enumMetadata = &metadata
		}
		return evaluateEnumPlugin(ctx, node, values, arguments, definition.factory, definition.resultType)
	}}
}

// EnumPlugin is the inline form for a rule module that does not need to put
// the plugin in an Environment registry. It is useful for local extensions
// and has the same Build-time footprint checks as EnumPluginRef.
func EnumPlugin[I, R any](name string, values Expression[[]I], footprints []EnumMethodFootprint, factory EnumPluginFactory[R], arguments ...EnumPluginArgument) Expression[R] {
	name = strings.TrimSpace(name)
	node, _, metadata := newEnumPluginNode[I, R]("enum-plugin", name, nil, values, arguments)
	node.enumPluginFootprints = cloneEnumMethodFootprints(footprints)
	node.enumPluginFactory = adaptEnumPluginFactory(factory)
	node.enumPluginReady = factory != nil
	metadata.Footprints = cloneEnumMethodFootprints(footprints)
	node.enumMetadata = &metadata
	return typedExpr[R]{n: node, fn: func(ctx EvalContext) (result Value) {
		defer func() {
			if recover() != nil {
				result = Null()
			}
		}()
		return evaluateEnumPlugin(ctx, node, values, arguments, node.enumPluginFactory, typeOf[R]())
	}}
}

// EnumPluginStateValue returns the current plugin state from inside a lambda
// expression. It is Missing outside an active plugin iteration.
func EnumPluginStateValue[T any]() Expression[T] {
	return makeExpr[T]("enum-plugin-state", fmt.Sprintf("plugin-state<%s>()", typeOf[T]()), nil, func(ctx EvalContext) Value {
		if !ctx.enumPluginStateActive {
			return Missing()
		}
		return ctx.enumPluginStateValue
	})
}

func newEnumPluginNode[I, R any](kind, name string, env *Environment, values Expression[[]I], arguments []EnumPluginArgument) (*exprNode, []*exprNode, EnumMethodMetadata) {
	children := make([]*exprNode, 0, 1+len(arguments))
	if values == nil {
		children = append(children, nil)
	} else {
		children = append(children, values.node())
	}
	argumentMeta := make([]enumPluginArgumentMeta, 0, len(arguments))
	parts := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		if argument.expression == nil || argument.expression.node() == nil {
			children = append(children, nil)
			argumentMeta = append(argumentMeta, enumPluginArgumentMeta{lambda: argument.lambda, lambdaParameterCount: argument.arity})
			parts = append(parts, "<nil>")
			continue
		}
		children = append(children, argument.expression.node())
		argumentMeta = append(argumentMeta, enumPluginArgumentMeta{typ: argument.expression.Type(), lambda: argument.lambda, lambdaParameterCount: argument.arity})
		parts = append(parts, argument.expression.Description())
	}
	description := "enum-plugin(<invalid>)"
	if name != "" {
		description = "enum-plugin(" + name
		if values != nil {
			description += "," + values.Description()
		}
		if len(parts) > 0 {
			description += "," + strings.Join(parts, ",")
		}
		description += ")"
	}
	node := &exprNode{
		kind:                  kind,
		typ:                   typeOf[R](),
		description:           description,
		enumPluginName:        name,
		enumPluginEnvironment: env,
		enumPluginArguments:   argumentMeta,
		enumInputRequired:     true,
		children:              children,
	}
	collectionType, elementType := enumCollectionTypes(nil)
	if values != nil {
		collectionType, elementType = values.Type(), typeOf[I]()
	}
	metadata := EnumMethodMetadata{Method: name, CollectionType: collectionType, ElementType: elementType, ResultType: typeOf[R]()}
	node.enumMetadata = &metadata
	return node, children, metadata
}

func validateEnumPluginFootprintDeclarations(name string, footprints []EnumMethodFootprint) error {
	for index, footprint := range footprints {
		for parameterIndex, parameter := range footprint.Parameters {
			if parameter.LambdaParameterCount < 0 {
				return NewError(ErrorInvalidRule, fmt.Sprintf("enumeration plugin %q footprint %d parameter %d has invalid lambda arity %d", name, index, parameterIndex, parameter.LambdaParameterCount))
			}
		}
	}
	return nil
}

func (e *Environment) validateEnumPluginNodes(node *exprNode) error {
	if node == nil {
		return nil
	}
	if node.kind == "enum-plugin-ref" || node.kind == "enum-plugin" {
		var definition enumPluginDefinition
		var ok bool
		if node.kind == "enum-plugin-ref" {
			if node.enumPluginEnvironment == nil || node.enumPluginEnvironment != e {
				return NewError(ErrorDependency, fmt.Sprintf("enumeration plugin %q belongs to a different environment", node.enumPluginName))
			}
			e.mu.RLock()
			definition, ok = e.enumPlugins[node.enumPluginName]
			e.mu.RUnlock()
			if !ok {
				return NewError(ErrorUnknownName, fmt.Sprintf("enumeration plugin %q is not registered", node.enumPluginName))
			}
		} else {
			if strings.TrimSpace(node.enumPluginName) == "" {
				return NewError(ErrorInvalidRule, "inline enumeration plugin name is required")
			}
			definition = enumPluginDefinition{resultType: node.typ, footprints: node.enumPluginFootprints, factory: node.enumPluginFactory}
			ok = true
		}
		if !ok || definition.factory == nil || (node.kind == "enum-plugin" && !node.enumPluginReady) {
			return NewError(ErrorInvalidRule, fmt.Sprintf("enumeration plugin %q has no factory", node.enumPluginName))
		}
		if definition.resultType != nil && node.typ != nil && !expressionTypesCompatible(definition.resultType, node.typ) {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("enumeration plugin %q returns %s, reference expects %s", node.enumPluginName, definition.resultType, node.typ))
		}
		if len(node.children) == 0 || node.children[0] == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("enumeration plugin %q requires a collection expression", node.enumPluginName))
		}
		if err := validateEnumPluginFootprint(node.children[0], node.enumPluginArguments, definition.footprints, node.enumPluginName); err != nil {
			return err
		}
		node.enumPluginFootprints = cloneEnumMethodFootprints(definition.footprints)
		node.enumMetadata = &EnumMethodMetadata{Method: node.enumPluginName, CollectionType: node.children[0].typ, ElementType: enumPluginElementType(node.children[0].typ), ResultType: node.typ, Footprints: cloneEnumMethodFootprints(definition.footprints)}
	}
	for _, child := range node.children {
		if err := e.validateEnumPluginNodes(child); err != nil {
			return err
		}
	}
	return nil
}

func validateEnumPluginFootprint(input *exprNode, arguments []enumPluginArgumentMeta, footprints []EnumMethodFootprint, name string) error {
	inputKind := enumPluginInputKind(input.typ)
	for _, footprint := range footprints {
		if !enumPluginInputCompatible(footprint.Input, inputKind) || len(footprint.Parameters) != len(arguments) {
			continue
		}
		matched := true
		for index, parameter := range footprint.Parameters {
			argument := arguments[index]
			if argument.typ == nil {
				matched = false
				break
			}
			if parameter.LambdaParameterCount == 0 {
				if argument.lambda || !enumPluginParameterTypeCompatible(parameter.Expected, argument.typ) {
					matched = false
					break
				}
			} else if !argument.lambda || argument.lambdaParameterCount != parameter.LambdaParameterCount || !enumPluginParameterTypeCompatible(parameter.Expected, argument.typ) {
				matched = false
				break
			}
		}
		if matched {
			return nil
		}
	}
	return NewError(ErrorInvalidRule, fmt.Sprintf("enumeration plugin %q has no matching method footprint for input %s and %d arguments", name, inputKind, len(arguments)))
}

func enumPluginElementType(typ reflect.Type) reflect.Type {
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ != nil && (typ.Kind() == reflect.Array || typ.Kind() == reflect.Slice) {
		return typ.Elem()
	}
	return typ
}

func enumPluginInputKind(typ reflect.Type) EnumInputKind {
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == nil {
		return EnumInputAny
	}
	if typ.Kind() == reflect.Array || typ.Kind() == reflect.Slice {
		element := typ.Elem()
		for element.Kind() == reflect.Pointer {
			element = element.Elem()
		}
		if element == typeOf[Event]() {
			return EnumInputEventCollection
		}
		if enumPluginNumericType(element) {
			return EnumInputScalarNumeric
		}
		return EnumInputScalarAny
	}
	if typ.Kind() == reflect.Func && typ.NumIn() == 1 && typ.NumOut() == 0 {
		yieldType := typ.In(0)
		if yieldType.Kind() == reflect.Func && yieldType.NumIn() == 1 && yieldType.NumOut() == 1 && yieldType.Out(0).Kind() == reflect.Bool {
			element := yieldType.In(0)
			for element.Kind() == reflect.Pointer {
				element = element.Elem()
			}
			if element == typeOf[Event]() {
				return EnumInputEventCollection
			}
			if enumPluginNumericType(element) {
				return EnumInputScalarNumeric
			}
			return EnumInputScalarAny
		}
	}
	return EnumInputAny
}

func enumPluginInputCompatible(expected, actual EnumInputKind) bool {
	if expected == EnumInputAny || actual == EnumInputAny {
		return true
	}
	if expected == EnumInputScalarAny {
		return actual == EnumInputScalarAny || actual == EnumInputScalarNumeric
	}
	return expected == actual
}

func enumPluginParameterTypeCompatible(expected EnumParameterKind, typ reflect.Type) bool {
	switch expected {
	case EnumParameterBoolean:
		return typ != nil && typ.Kind() == reflect.Bool
	case EnumParameterNumeric:
		return enumPluginNumericType(typ)
	case EnumParameterCollection:
		return typ != nil && (typ.Kind() == reflect.Array || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Map)
	default:
		return true
	}
}

func enumPluginNumericType(typ reflect.Type) bool {
	return isNumericType(typ) || typ == reflect.TypeOf(big.Int{}) || typ == reflect.TypeOf(big.Rat{})
}

func evaluateEnumPlugin(ctx EvalContext, node *exprNode, values Expr, arguments []EnumPluginArgument, factory enumPluginFactory, resultType reflect.Type) Value {
	if values == nil || factory == nil {
		return Missing()
	}
	input := values.eval(ctx)
	if !input.IsPresent() {
		return input
	}
	items, ok := enumPluginItems(input.Any())
	if !ok {
		return Null()
	}
	state := factory(EnumPluginContext{Name: node.enumPluginName, ElementType: enumPluginElementType(values.Type()), ResultType: resultType, Environment: node.enumPluginEnvironment, Evaluation: ctx})
	if state == nil {
		return Null()
	}
	parameterIndex := 0
	for _, argument := range arguments {
		if argument.expression == nil || argument.expression.node() == nil {
			return Null()
		}
		if argument.lambda {
			continue
		}
		value := argument.expression.eval(ctx)
		if !value.IsPresent() {
			return Null()
		}
		// Esper numbers state parameters independently from lambda
		// parameters; a lambda does not consume a setParameter slot.
		state.setParameter(parameterIndex, value)
		parameterIndex++
	}
	for index, item := range items {
		lambdaValues := make([]Value, 0, len(arguments))
		for _, argument := range arguments {
			if !argument.lambda {
				continue
			}
			if argument.expression == nil || argument.expression.node() == nil {
				return Null()
			}
			nested := enumElementContext(ctx, item, index, len(items))
			stateValue, present := state.result()
			if present {
				nested.enumPluginStateValue = stateValue
			} else {
				nested.enumPluginStateValue = Null()
			}
			nested.enumPluginStateActive = true
			lambdaValues = append(lambdaValues, argument.expression.eval(nested))
		}
		state.add(Present(item), lambdaValues)
		if state.completed() {
			break
		}
	}
	result, present := state.result()
	if !present {
		return Null()
	}
	return result
}

func enumPluginItems(raw any) ([]any, bool) {
	if raw == nil {
		return nil, false
	}
	value := reflect.ValueOf(raw)
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return nil, false
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return nil, false
	}
	if value.Kind() == reflect.Array || value.Kind() == reflect.Slice {
		result := make([]any, value.Len())
		for index := 0; index < value.Len(); index++ {
			if !value.Index(index).CanInterface() {
				return nil, false
			}
			result[index] = value.Index(index).Interface()
		}
		return result, true
	}
	if value.Kind() == reflect.Func && value.Type().NumIn() == 1 && value.Type().NumOut() == 0 {
		if value.IsNil() {
			return nil, false
		}
		itemType := value.Type().In(0)
		if itemType.Kind() == reflect.Func && itemType.NumIn() == 1 && itemType.NumOut() == 1 && itemType.Out(0).Kind() == reflect.Bool {
			result := make([]any, 0)
			yield := reflect.MakeFunc(itemType, func(args []reflect.Value) []reflect.Value {
				if len(args) == 1 && args[0].CanInterface() {
					result = append(result, args[0].Interface())
				}
				return []reflect.Value{reflect.ValueOf(true)}
			})
			value.Call([]reflect.Value{yield})
			return result, true
		}
	}
	return nil, false
}

func enumPluginFootprintsCanonical(footprints []EnumMethodFootprint) string {
	parts := make([]string, 0, len(footprints))
	for _, footprint := range footprints {
		parameters := make([]string, 0, len(footprint.Parameters))
		for _, parameter := range footprint.Parameters {
			parameters = append(parameters, fmt.Sprintf("%d:%s:%d", parameter.LambdaParameterCount, parameter.Description, parameter.Expected))
		}
		parts = append(parts, fmt.Sprintf("%s[%s]", footprint.Input, strings.Join(parameters, ",")))
	}
	return strings.Join(parts, ";")
}

// EnumPluginFootprints returns a defensive copy of the registered method
// footprints for tooling and parity reports.
func (e *Environment) EnumPluginFootprints(name string) ([]EnumMethodFootprint, bool) {
	if e == nil {
		return nil, false
	}
	e.mu.RLock()
	definition, ok := e.enumPlugins[strings.TrimSpace(name)]
	e.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return cloneEnumMethodFootprints(definition.footprints), true
}
