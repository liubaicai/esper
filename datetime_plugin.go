package esper

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// DateTimeInputKind describes the statically known input family offered to a
// registered date-time method. Any is useful for methods that provide a
// handler for more than one Go time representation.
type DateTimeInputKind uint8

const (
	DateTimeInputAny DateTimeInputKind = iota
	DateTimeInputTime
	DateTimeInputEpochMillis
)

func (kind DateTimeInputKind) String() string {
	switch kind {
	case DateTimeInputAny:
		return "any"
	case DateTimeInputTime:
		return "time"
	case DateTimeInputEpochMillis:
		return "epoch-millis"
	default:
		return "unknown"
	}
}

// DateTimeParameterKind is the compact type contract used by a date-time
// method footprint. Specific parameters carry their reflect.Type separately.
type DateTimeParameterKind uint8

const (
	DateTimeParameterAny DateTimeParameterKind = iota
	DateTimeParameterString
	DateTimeParameterBoolean
	DateTimeParameterNumeric
	DateTimeParameterSpecific
)

func (kind DateTimeParameterKind) String() string {
	switch kind {
	case DateTimeParameterAny:
		return "any"
	case DateTimeParameterString:
		return "string"
	case DateTimeParameterBoolean:
		return "boolean"
	case DateTimeParameterNumeric:
		return "numeric"
	case DateTimeParameterSpecific:
		return "specific"
	default:
		return "unknown"
	}
}

// DateTimeMethodParameter describes one ordinary argument. Date-time method
// plugins do not use enumeration lambdas; each argument is evaluated once for
// the receiver value.
type DateTimeMethodParameter struct {
	Description string
	Expected    DateTimeParameterKind
	Type        reflect.Type
}

// DateTimeMethodFootprint is one accepted receiver/argument signature.
type DateTimeMethodFootprint struct {
	Input      DateTimeInputKind
	Parameters []DateTimeMethodParameter
}

// DateTimePluginMetadata exposes the static method information attached to a
// date-time plugin expression. Slices are copied on read.
type DateTimePluginMetadata struct {
	Method     string
	InputType  reflect.Type
	ResultType reflect.Type
	Footprints []DateTimeMethodFootprint
}

// DateTimePluginMetadataOf returns static metadata for a date-time plugin
// expression without evaluating its receiver or handler.
func DateTimePluginMetadataOf(expression Expr) (DateTimePluginMetadata, bool) {
	if expression == nil || expression.node() == nil || expression.node().dateTimePluginMetadata == nil {
		return DateTimePluginMetadata{}, false
	}
	metadata := *expression.node().dateTimePluginMetadata
	metadata.Footprints = cloneDateTimeMethodFootprints(metadata.Footprints)
	return metadata, true
}

// DateTimePluginFactoryContext is supplied to the stateless operation factory
// at Build and evaluation time. A factory returning nil means that the plugin
// has no implementation for the requested receiver/result shape.
type DateTimePluginFactoryContext struct {
	Name        string
	InputType   reflect.Type
	ResultType  reflect.Type
	Environment *Environment
	Evaluation  EvalContext
}

// DateTimePluginOps is the Go-native equivalent of Esper's date-time method
// operation modes. It receives the already-evaluated receiver and arguments
// and returns a tagged Value so Null/Missing can remain distinct.
type DateTimePluginOps interface {
	Apply(input Value, arguments []Value) (Value, error)
}

// DateTimePluginOpsFunc adapts a function into DateTimePluginOps.
type DateTimePluginOpsFunc func(input Value, arguments []Value) (Value, error)

func (function DateTimePluginOpsFunc) Apply(input Value, arguments []Value) (Value, error) {
	if function == nil {
		return Null(), fmt.Errorf("date-time plugin operation is nil")
	}
	return function(input, arguments)
}

// DateTimePluginFactory creates the operation set for one receiver/result
// shape. The same registered method may return different operations for
// time.Time and epoch-millisecond receivers, matching Java's Calendar/Date/
// long/LocalDateTime/ZonedDateTime mode selection.
type DateTimePluginFactory func(DateTimePluginFactoryContext) DateTimePluginOps

type dateTimePluginFactory func(DateTimePluginFactoryContext) DateTimePluginOps

type dateTimePluginDefinition struct {
	footprints []DateTimeMethodFootprint
	factory    dateTimePluginFactory
}

type DateTimePluginArgument struct {
	expression Expr
}

// DateTimeArgument marks one ordinary plugin argument while retaining the
// expression in the analyzable AST.
func DateTimeArgument(expression Expr) DateTimePluginArgument {
	return DateTimePluginArgument{expression: expression}
}

type dateTimePluginArgumentMeta struct {
	typ reflect.Type
}

// RegisterDateTimePlugin registers a named date-time method in an Environment.
// Registration is explicit and portable; it does not load Java classes or Go
// shared objects at runtime.
func RegisterDateTimePlugin(env *Environment, name string, footprints []DateTimeMethodFootprint, factory DateTimePluginFactory) error {
	if env == nil {
		return NewError(ErrorDependency, "nil environment")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return NewError(ErrorInvalidRule, "date-time plugin name is required")
	}
	if len(footprints) == 0 {
		return NewError(ErrorInvalidRule, fmt.Sprintf("date-time plugin %q requires at least one footprint", name))
	}
	if factory == nil {
		return NewError(ErrorInvalidRule, fmt.Sprintf("date-time plugin %q has no factory", name))
	}
	if err := validateDateTimePluginFootprintDeclarations(name, footprints); err != nil {
		return err
	}
	env.mu.Lock()
	defer env.mu.Unlock()
	if _, exists := env.dateTimePlugins[name]; exists {
		return NewError(ErrorDependency, fmt.Sprintf("date-time plugin %q is already registered", name))
	}
	if _, exists := env.enumPlugins[name]; exists {
		return NewError(ErrorDependency, fmt.Sprintf("date-time plugin %q conflicts with enumeration plugin", name))
	}
	if _, exists := env.aggregatePlugins[name]; exists {
		return NewError(ErrorDependency, fmt.Sprintf("date-time plugin %q conflicts with aggregate plugin", name))
	}
	env.dateTimePlugins[name] = dateTimePluginDefinition{
		footprints: cloneDateTimeMethodFootprints(footprints),
		factory:    dateTimePluginFactory(factory),
	}
	return nil
}

// DateTimePluginRef creates a typed reference to a registered date-time
// method. The receiver is an Expr so callers can chain through time.Time,
// epoch-millisecond and dynamically typed values with one Go entry point.
func DateTimePluginRef[R any](env *Environment, name string, input Expr, arguments ...DateTimePluginArgument) Expression[R] {
	name = strings.TrimSpace(name)
	node, metadata := newDateTimePluginNode[R]("datetime-plugin-ref", name, env, input, arguments)
	if env != nil && name != "" {
		env.mu.RLock()
		definition, registered := env.dateTimePlugins[name]
		env.mu.RUnlock()
		if registered {
			node.dateTimePluginFootprints = cloneDateTimeMethodFootprints(definition.footprints)
			metadata.Footprints = cloneDateTimeMethodFootprints(definition.footprints)
			node.dateTimePluginMetadata = &metadata
		}
	}
	if env == nil {
		node.configurationError = "date-time plugin reference requires an environment"
	}
	if name == "" {
		node.configurationError = "date-time plugin name is required"
	}
	return typedExpr[R]{n: node, fn: func(ctx EvalContext) (result Value) {
		defer func() {
			if recover() != nil {
				result = Null()
			}
		}()
		if env == nil || name == "" || input == nil {
			return Missing()
		}
		env.mu.RLock()
		definition, ok := env.dateTimePlugins[name]
		env.mu.RUnlock()
		if !ok || definition.factory == nil {
			return Missing()
		}
		return evaluateDateTimePlugin[R](ctx, node, input, arguments, definition.factory)
	}}
}

// DateTimePlugin is the inline counterpart to DateTimePluginRef for a local
// rule module that does not need Environment registration.
func DateTimePlugin[R any](name string, input Expr, footprints []DateTimeMethodFootprint, factory DateTimePluginFactory, arguments ...DateTimePluginArgument) Expression[R] {
	name = strings.TrimSpace(name)
	node, metadata := newDateTimePluginNode[R]("datetime-plugin", name, nil, input, arguments)
	node.dateTimePluginFootprints = cloneDateTimeMethodFootprints(footprints)
	node.dateTimePluginFactory = dateTimePluginFactory(factory)
	node.dateTimePluginReady = factory != nil
	metadata.Footprints = cloneDateTimeMethodFootprints(footprints)
	node.dateTimePluginMetadata = &metadata
	return typedExpr[R]{n: node, fn: func(ctx EvalContext) (result Value) {
		defer func() {
			if recover() != nil {
				result = Null()
			}
		}()
		return evaluateDateTimePlugin[R](ctx, node, input, arguments, node.dateTimePluginFactory)
	}}
}

func newDateTimePluginNode[R any](kind, name string, env *Environment, input Expr, arguments []DateTimePluginArgument) (*exprNode, DateTimePluginMetadata) {
	children := make([]*exprNode, 0, 1+len(arguments))
	if input == nil {
		children = append(children, nil)
	} else {
		children = append(children, input.node())
	}
	argumentMeta := make([]dateTimePluginArgumentMeta, 0, len(arguments))
	parts := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		if argument.expression == nil || argument.expression.node() == nil {
			children = append(children, nil)
			argumentMeta = append(argumentMeta, dateTimePluginArgumentMeta{})
			parts = append(parts, "<nil>")
			continue
		}
		children = append(children, argument.expression.node())
		argumentMeta = append(argumentMeta, dateTimePluginArgumentMeta{typ: argument.expression.Type()})
		parts = append(parts, argument.expression.Description())
	}
	description := "datetime-plugin(<invalid>)"
	if name != "" {
		description = "datetime-plugin(" + name
		if input != nil {
			description += "," + input.Description()
		}
		if len(parts) > 0 {
			description += "," + strings.Join(parts, ",")
		}
		description += ")"
	}
	node := &exprNode{
		kind:                      kind,
		typ:                       typeOf[R](),
		description:               description,
		dateTimePluginName:        name,
		dateTimePluginEnvironment: env,
		dateTimePluginArguments:   argumentMeta,
		children:                  children,
	}
	var inputType reflect.Type
	if input != nil {
		inputType = input.Type()
	}
	metadata := DateTimePluginMetadata{Method: name, InputType: inputType, ResultType: typeOf[R]()}
	node.dateTimePluginMetadata = &metadata
	return node, metadata
}

func validateDateTimePluginFootprintDeclarations(name string, footprints []DateTimeMethodFootprint) error {
	for index, footprint := range footprints {
		if footprint.Input > DateTimeInputEpochMillis {
			return NewError(ErrorInvalidRule, fmt.Sprintf("date-time plugin %q footprint %d has invalid input kind %d", name, index, footprint.Input))
		}
		for parameterIndex, parameter := range footprint.Parameters {
			if parameter.Expected > DateTimeParameterSpecific {
				return NewError(ErrorInvalidRule, fmt.Sprintf("date-time plugin %q footprint %d parameter %d has invalid parameter kind %d", name, index, parameterIndex, parameter.Expected))
			}
			if parameter.Expected == DateTimeParameterSpecific && parameter.Type == nil {
				return NewError(ErrorInvalidRule, fmt.Sprintf("date-time plugin %q footprint %d parameter %d requires a specific type", name, index, parameterIndex))
			}
			if parameter.Expected != DateTimeParameterSpecific && parameter.Type != nil {
				return NewError(ErrorInvalidRule, fmt.Sprintf("date-time plugin %q footprint %d parameter %d has an unexpected specific type", name, index, parameterIndex))
			}
		}
	}
	return nil
}

func (e *Environment) validateDateTimePluginNodes(node *exprNode) error {
	if node == nil {
		return nil
	}
	if node.kind == "datetime-plugin-ref" || node.kind == "datetime-plugin" {
		var definition dateTimePluginDefinition
		var ok bool
		if node.kind == "datetime-plugin-ref" {
			if node.dateTimePluginEnvironment == nil || node.dateTimePluginEnvironment != e {
				return NewError(ErrorDependency, fmt.Sprintf("date-time plugin %q belongs to a different environment", node.dateTimePluginName))
			}
			e.mu.RLock()
			definition, ok = e.dateTimePlugins[node.dateTimePluginName]
			e.mu.RUnlock()
			if !ok {
				return NewError(ErrorUnknownName, fmt.Sprintf("date-time plugin %q is not registered", node.dateTimePluginName))
			}
		} else {
			if strings.TrimSpace(node.dateTimePluginName) == "" {
				return NewError(ErrorInvalidRule, "inline date-time plugin name is required")
			}
			definition = dateTimePluginDefinition{footprints: node.dateTimePluginFootprints, factory: node.dateTimePluginFactory}
			ok = true
		}
		if !ok || definition.factory == nil || (node.kind == "datetime-plugin" && !node.dateTimePluginReady) {
			return NewError(ErrorInvalidRule, fmt.Sprintf("date-time plugin %q has no factory", node.dateTimePluginName))
		}
		if len(node.children) == 0 || node.children[0] == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("date-time plugin %q requires a receiver expression", node.dateTimePluginName))
		}
		if err := validateDateTimePluginFootprintDeclarations(node.dateTimePluginName, definition.footprints); err != nil {
			return err
		}
		if err := validateDateTimePluginFootprint(node.children[0], node.dateTimePluginArguments, definition.footprints, node.dateTimePluginName); err != nil {
			return err
		}
		if dateTimePluginOperations(definition.factory, DateTimePluginFactoryContext{
			Name:        node.dateTimePluginName,
			InputType:   node.children[0].typ,
			ResultType:  node.typ,
			Environment: e,
		}) == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("date-time plugin %q has no operation for input %s and result %s", node.dateTimePluginName, node.children[0].typ, node.typ))
		}
		node.dateTimePluginFootprints = cloneDateTimeMethodFootprints(definition.footprints)
		node.dateTimePluginMetadata = &DateTimePluginMetadata{Method: node.dateTimePluginName, InputType: node.children[0].typ, ResultType: node.typ, Footprints: cloneDateTimeMethodFootprints(definition.footprints)}
	}
	for _, child := range node.children {
		if err := e.validateDateTimePluginNodes(child); err != nil {
			return err
		}
	}
	return nil
}

func validateDateTimePluginFootprint(input *exprNode, arguments []dateTimePluginArgumentMeta, footprints []DateTimeMethodFootprint, name string) error {
	inputKind := dateTimePluginInputKind(input.typ)
	for _, footprint := range footprints {
		if !dateTimePluginInputCompatible(footprint.Input, inputKind) || len(footprint.Parameters) != len(arguments) {
			continue
		}
		matched := true
		for index, parameter := range footprint.Parameters {
			argument := arguments[index]
			if argument.typ == nil || !dateTimePluginParameterTypeCompatible(parameter, argument.typ) {
				matched = false
				break
			}
		}
		if matched {
			return nil
		}
	}
	return NewError(ErrorInvalidRule, fmt.Sprintf("date-time plugin %q has no matching method footprint for input %s and %d arguments", name, inputKind, len(arguments)))
}

func dateTimePluginInputKind(typ reflect.Type) DateTimeInputKind {
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == typeOf[time.Time]() {
		return DateTimeInputTime
	}
	if isNumericType(typ) {
		return DateTimeInputEpochMillis
	}
	return DateTimeInputAny
}

func dateTimePluginInputCompatible(expected, actual DateTimeInputKind) bool {
	return expected == DateTimeInputAny || actual == DateTimeInputAny || expected == actual
}

func dateTimePluginParameterTypeCompatible(parameter DateTimeMethodParameter, typ reflect.Type) bool {
	if typ == nil || typ == typeOf[any]() {
		return true
	}
	switch parameter.Expected {
	case DateTimeParameterAny:
		return true
	case DateTimeParameterString:
		return typ.Kind() == reflect.String
	case DateTimeParameterBoolean:
		return typ.Kind() == reflect.Bool
	case DateTimeParameterNumeric:
		return isNumericType(typ)
	case DateTimeParameterSpecific:
		return parameter.Type == typ || parameter.Type.AssignableTo(typ) || typ.AssignableTo(parameter.Type)
	default:
		return false
	}
}

func evaluateDateTimePlugin[R any](ctx EvalContext, node *exprNode, input Expr, arguments []DateTimePluginArgument, factory dateTimePluginFactory) Value {
	if input == nil || factory == nil {
		return Missing()
	}
	inputValue := input.eval(ctx)
	if !inputValue.IsPresent() {
		return inputValue
	}
	ops := dateTimePluginOperations(factory, DateTimePluginFactoryContext{
		Name:        node.dateTimePluginName,
		InputType:   input.Type(),
		ResultType:  node.typ,
		Environment: node.dateTimePluginEnvironment,
		Evaluation:  ctx,
	})
	if ops == nil {
		return Null()
	}
	evaluated := make([]Value, 0, len(arguments))
	for _, argument := range arguments {
		if argument.expression == nil {
			return Null()
		}
		value := argument.expression.eval(ctx)
		if !value.IsPresent() {
			return Null()
		}
		evaluated = append(evaluated, value)
	}
	result, err := ops.Apply(inputValue, evaluated)
	if err != nil || !result.IsPresent() {
		if result.IsMissing() {
			return Null()
		}
		return result
	}
	if _, err := As[R](result); err != nil {
		return Null()
	}
	return result
}

func dateTimePluginOperations(factory dateTimePluginFactory, context DateTimePluginFactoryContext) (operations DateTimePluginOps) {
	if factory == nil {
		return nil
	}
	defer func() {
		if recover() != nil {
			operations = nil
		}
	}()
	return factory(context)
}

// DateTimePluginFootprints returns a defensive copy of a registered method's
// accepted footprints.
func (e *Environment) DateTimePluginFootprints(name string) ([]DateTimeMethodFootprint, bool) {
	if e == nil {
		return nil, false
	}
	e.mu.RLock()
	definition, ok := e.dateTimePlugins[strings.TrimSpace(name)]
	e.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return cloneDateTimeMethodFootprints(definition.footprints), true
}

func cloneDateTimeMethodFootprints(source []DateTimeMethodFootprint) []DateTimeMethodFootprint {
	if len(source) == 0 {
		return nil
	}
	result := make([]DateTimeMethodFootprint, len(source))
	for index, footprint := range source {
		result[index] = DateTimeMethodFootprint{Input: footprint.Input, Parameters: append([]DateTimeMethodParameter(nil), footprint.Parameters...)}
	}
	return result
}

func dateTimePluginFootprintsCanonical(footprints []DateTimeMethodFootprint) string {
	parts := make([]string, 0, len(footprints))
	for _, footprint := range footprints {
		parameters := make([]string, 0, len(footprint.Parameters))
		for _, parameter := range footprint.Parameters {
			typ := ""
			if parameter.Type != nil {
				typ = parameter.Type.String()
			}
			parameters = append(parameters, fmt.Sprintf("%s:%s:%s", parameter.Expected, parameter.Description, typ))
		}
		parts = append(parts, fmt.Sprintf("%s[%s]", footprint.Input, strings.Join(parameters, ",")))
	}
	return strings.Join(parts, ";")
}
