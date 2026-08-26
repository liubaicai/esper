package esper

import (
	"fmt"
	"reflect"
	"strings"
)

type VariableDefinition struct {
	name     string
	context  string
	typ      reflect.Type
	initial  Value
	constant bool
}

type VariableOption func(*variableConfig)

type variableConfig struct {
	constant bool
	typ      reflect.Type
}

func ConstantVariable() VariableOption {
	return func(config *variableConfig) { config.constant = true }
}

// VariableType overrides the declared variable type for variables whose
// initial value is untyped nil (a typed null), matching Java's
// addVariable(name, Type, null) configuration form.
func VariableType(t reflect.Type) VariableOption {
	return func(config *variableConfig) { config.typ = t }
}

func newVariableDefinition(name string, initial any, options ...VariableOption) (VariableDefinition, error) {
	return newVariableDefinitionForContext("", name, initial, options...)
}

func newVariableDefinitionForContext(contextName, name string, initial any, options ...VariableOption) (VariableDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return VariableDefinition{}, NewError(ErrorInvalidRule, "variable name is required")
	}
	contextName = strings.TrimSpace(contextName)
	config := variableConfig{}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	value := Present(initial)
	typ := reflect.TypeOf(initial)
	if initial == nil {
		value = Null()
		typ = typeOf[any]()
	}
	if config.typ != nil {
		typ = config.typ
		if value.IsNull() || value.IsMissing() {
			value = Null()
		}
	}
	return VariableDefinition{name: name, context: contextName, typ: typ, initial: value, constant: config.constant}, nil
}

func (v VariableDefinition) Name() string        { return v.name }
func (v VariableDefinition) ContextName() string { return v.context }
func (v VariableDefinition) Type() reflect.Type  { return v.typ }
func (v VariableDefinition) Initial() Value      { return v.initial }
func (v VariableDefinition) Constant() bool      { return v.constant }

func (v VariableDefinition) validate(value any) error {
	_, err := v.coerce(value)
	return err
}

func (v VariableDefinition) coerce(value any) (any, error) {
	// Dereference pointer values so a nullable *int field assigned to a
	// variable is stored as the underlying value. A nil pointer is null.
	if ptr := reflect.ValueOf(value); ptr.Kind() == reflect.Pointer {
		if ptr.IsNil() {
			return nil, nil
		}
		value = ptr.Elem().Interface()
	}
	if v.typ == nil || v.typ == typeOf[any]() || value == nil {
		return value, nil
	}
	got := reflect.TypeOf(value)
	if got.AssignableTo(v.typ) {
		return value, nil
	}
	if numericFamilyWidening(got, v.typ) {
		return reflect.ValueOf(value).Convert(v.typ).Interface(), nil
	}
	return nil, fmt.Errorf("variable %q expects %s, got %s", v.name, v.typ, got)
}

// javaNumericRank ranks a Go numeric type along Java's widening chain
// Byte < Short < Integer < Long < Float < Double. Unsigned kinds map onto
// their same-width signed family member (uint8 behaves as Java byte,
// uint32 as Java int, and so on) because the port represents Java unsigned
// quantities with the corresponding signed Go kinds.
func javaNumericRank(t reflect.Type) (int, bool) {
	switch t.Kind() {
	case reflect.Int8, reflect.Uint8:
		return 0, true // Byte
	case reflect.Int16, reflect.Uint16:
		return 1, true // Short
	case reflect.Int32, reflect.Int, reflect.Uint32, reflect.Uint, reflect.Uintptr:
		return 2, true // Integer
	case reflect.Int64, reflect.Uint64:
		return 3, true // Long
	case reflect.Float32:
		return 4, true // Float
	case reflect.Float64:
		return 5, true // Double
	default:
		return 0, false
	}
}

// numericFamilyWidening reports whether a value of type from may be stored
// in a variable of type to under Java's widening primitive conversion: the
// same family or strictly later in the Byte<Short<Integer<Long<Float<Double
// chain. Narrowing (long to int) and incomparable moves (double to float,
// any float to an integer family) are rejected, matching the engine oracle.
func numericFamilyWidening(from, to reflect.Type) bool {
	fromRank, fromOK := javaNumericRank(from)
	toRank, toOK := javaNumericRank(to)
	return fromOK && toOK && toRank >= fromRank
}
