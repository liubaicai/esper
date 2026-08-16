package esper

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

// JSONFieldAdapter converts a JSON string field to and from a Go value.
//
// Esper exposes this capability through a Java class named by an annotation.
// Go callers register an explicit value instead, which keeps construction
// type-safe and avoids runtime class loading. Adapters are invoked only for
// non-null JSON string values; null remains the event's Null state.
type JSONFieldAdapter interface {
	// ValueType identifies the Go value produced by Parse and accepted by
	// Write. It is used during schema construction for early type validation.
	ValueType() reflect.Type
	Parse(string) (any, error)
	Write(any) (string, error)
}

// JSONFieldAdapterFuncs is a convenient non-generic adapter implementation.
// Type must match the value returned by Parse and accepted by Write.
type JSONFieldAdapterFuncs struct {
	// Name is an optional stable identity for plan canonicalization. It is
	// useful when ParseFunc/WriteFunc are closures whose captured values are
	// semantically significant.
	Name      string
	Type      reflect.Type
	ParseFunc func(string) (any, error)
	WriteFunc func(any) (string, error)
}

func (a JSONFieldAdapterFuncs) ValueType() reflect.Type { return a.Type }

func (a JSONFieldAdapterFuncs) Parse(value string) (any, error) {
	if a.ParseFunc == nil {
		return nil, fmt.Errorf("esper: JSON field adapter parse function is nil")
	}
	return a.ParseFunc(value)
}

func (a JSONFieldAdapterFuncs) Write(value any) (string, error) {
	if a.WriteFunc == nil {
		return "", fmt.Errorf("esper: JSON field adapter write function is nil")
	}
	return a.WriteFunc(value)
}

func (a JSONFieldAdapterFuncs) PlanIdentity() string {
	if name := strings.TrimSpace(a.Name); name != "" {
		return "name:" + name
	}
	return fmt.Sprintf("type:%s;parse:%s;write:%s", a.Type, functionIdentity(a.ParseFunc), functionIdentity(a.WriteFunc))
}

type jsonFieldAdapterTyped[T any] struct {
	name  string
	parse func(string) (T, error)
	write func(T) (string, error)
}

func (a jsonFieldAdapterTyped[T]) ValueType() reflect.Type { return typeOf[T]() }

func (a jsonFieldAdapterTyped[T]) Parse(value string) (any, error) {
	if a.parse == nil {
		return nil, fmt.Errorf("esper: JSON field adapter parse function is nil")
	}
	return a.parse(value)
}

func (a jsonFieldAdapterTyped[T]) Write(value any) (string, error) {
	if a.write == nil {
		return "", fmt.Errorf("esper: JSON field adapter write function is nil")
	}
	target := typeOf[T]()
	for target.Kind() != reflect.Pointer && value != nil {
		current := reflect.ValueOf(value)
		if current.Kind() != reflect.Pointer {
			break
		}
		if current.IsNil() {
			break
		}
		value = current.Elem().Interface()
	}
	converted, err := assignReflectValue(target, value)
	if err != nil {
		return "", err
	}
	return a.write(converted.Interface().(T))
}

func (a jsonFieldAdapterTyped[T]) PlanIdentity() string {
	if name := strings.TrimSpace(a.name); name != "" {
		return "name:" + name
	}
	return fmt.Sprintf("type:%s;parse:%s;write:%s", typeOf[T](), functionIdentity(a.parse), functionIdentity(a.write))
}

// NewJSONFieldAdapter creates a typed string adapter. The returned adapter
// can be passed to WithJSONFieldAdapter.
func NewJSONFieldAdapter[T any](parse func(string) (T, error), write func(T) (string, error)) JSONFieldAdapter {
	return jsonFieldAdapterTyped[T]{parse: parse, write: write}
}

// NewNamedJSONFieldAdapter is the stable-identity form of
// NewJSONFieldAdapter. The name participates in Plan canonicalization and is
// therefore required to remain stable when the adapter behavior is unchanged.
func NewNamedJSONFieldAdapter[T any](name string, parse func(string) (T, error), write func(T) (string, error)) JSONFieldAdapter {
	return jsonFieldAdapterTyped[T]{name: name, parse: parse, write: write}
}

func functionIdentity(function any) string {
	if function == nil {
		return "nil"
	}
	value := reflect.ValueOf(function)
	if value.Kind() != reflect.Func || value.IsNil() {
		return fmt.Sprintf("%T", function)
	}
	if metadata := runtime.FuncForPC(value.Pointer()); metadata != nil {
		return metadata.Name()
	}
	return fmt.Sprintf("%T", function)
}

func jsonFieldAdapterPlanIdentity(adapter JSONFieldAdapter) string {
	if isNilJSONFieldAdapter(adapter) {
		return "nil"
	}
	if identified, ok := adapter.(interface{ PlanIdentity() string }); ok {
		if identity := strings.TrimSpace(identified.PlanIdentity()); identity != "" {
			return identity
		}
	}
	return fmt.Sprintf("type:%T;value:%s", adapter, adapter.ValueType())
}

func isNilJSONFieldAdapter(adapter JSONFieldAdapter) bool {
	if adapter == nil {
		return true
	}
	value := reflect.ValueOf(adapter)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func schemaKindName(kind SchemaKind) string {
	switch kind {
	case SchemaStruct:
		return "Struct"
	case SchemaMap:
		return "Map"
	case SchemaJSON:
		return "JSON"
	case SchemaXML:
		return "XML"
	case SchemaAvro:
		return "Avro"
	case SchemaObjectArray:
		return "ObjectArray"
	case SchemaVariant:
		return "Variant"
	default:
		return fmt.Sprintf("SchemaKind(%d)", kind)
	}
}

func lookupFieldInList(fields []FieldSpec, name string, resolution PropertyResolutionStyle) (FieldSpec, string, error) {
	for _, field := range fields {
		if field.Name == name {
			return field, field.Name, nil
		}
	}
	if resolution != PropertyCaseSensitive {
		var match *FieldSpec
		for _, field := range fields {
			if !equalFold(field.Name, name) {
				continue
			}
			if match != nil && resolution == PropertyDistinctCaseInsensitive {
				return FieldSpec{}, "", fmt.Errorf("property name %q is ambiguous", name)
			}
			copyField := field
			match = &copyField
		}
		if match != nil {
			return *match, match.Name, nil
		}
	}
	return FieldSpec{}, "", fmt.Errorf("schema has no declared field %q", name)
}

func equalFold(left, right string) bool {
	return strings.EqualFold(left, right)
}

func jsonAdapterTypesCompatible(fieldType, valueType reflect.Type) bool {
	if fieldType == nil || fieldType == typeOf[any]() {
		return true
	}
	if valueType == nil {
		return false
	}
	if valueType.AssignableTo(fieldType) || valueType.ConvertibleTo(fieldType) && numericTypes(valueType, fieldType) {
		return true
	}
	if fieldType.Kind() == reflect.Pointer && valueType.AssignableTo(fieldType.Elem()) {
		return true
	}
	return false
}

func validateJSONFieldTypes(fields []FieldSpec) error {
	for _, field := range fields {
		if field.Type == nil || field.Type == typeOf[any]() {
			continue
		}
		typ := field.Type
		for typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		switch typ.Kind() {
		case reflect.Interface, reflect.Chan, reflect.Func, reflect.UnsafePointer:
			return fmt.Errorf("field %q uses unsupported JSON type %s", field.Name, field.Type)
		}
	}
	return nil
}
