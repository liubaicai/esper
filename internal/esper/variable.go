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

type variableConfig struct{ constant bool }

func ConstantVariable() VariableOption {
	return func(config *variableConfig) { config.constant = true }
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
	if isNumericType(got) && isNumericType(v.typ) && reflect.ValueOf(value).Type().ConvertibleTo(v.typ) {
		return reflect.ValueOf(value).Convert(v.typ).Interface(), nil
	}
	return nil, fmt.Errorf("variable %q expects %s, got %s", v.name, v.typ, got)
}
