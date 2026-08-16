package esper

import "reflect"

// jsonUnboxAdapterValue prepares a stored underlying value for a JSON field
// adapter's Write method.
//
// Esper's adapter contract hands the adapter the boxed object property value:
// JsonWriteForgeProvidedStringAdapter generates `adapter.write(field, writer)`
// with the field as stored, and the support adapters receive the nullable
// object and decide on their own whether it is null
// (regression-lib .../support/json/SupportJsonFieldAdapterStringDate.java).
// Go represents a nullable field as *T, so without normalization the renderer
// would pass the extra pointer wrapper to an adapter declared for T and the
// round trip would fail for the exact nullable shape that Esper supports.
//
// The helper unboxes pointer wrappers only when the adapter's own value type
// is non-pointer; adapters declared for pointer values keep the pointer. A nil
// stored pointer (the Null state, normally handled before Write is called)
// is returned as nil so adapters never observe a typed nil.
func jsonUnboxAdapterValue(adapter JSONFieldAdapter, value any) any {
	if value == nil || adapter == nil {
		return value
	}
	valueType := adapter.ValueType()
	if valueType == nil || valueType.Kind() == reflect.Pointer {
		return value
	}
	current := reflect.ValueOf(value)
	for current.IsValid() && current.Kind() == reflect.Pointer {
		if current.IsNil() {
			return nil
		}
		current = current.Elem()
	}
	if !current.IsValid() || !current.Type().AssignableTo(valueType) {
		return value
	}
	return current.Interface()
}

// jsonCharacterValue renders a Go rune (Java char/Character) as the JSON
// string form that Esper's JSON event type writer produces for Character
// fields: JsonForgeFactoryBuiltinClassTyped binds Character to
// JsonWriteForgeStringWithToString, which calls JsonWriteUtil.
// writeNullableStringToString and therefore writes the Character's string
// value, not a numeric code point.
//
// Parsed events already keep the original JSON text through the raw tree
// (rawJSONCharacter); this fallback covers programmatic and derived events
// whose rune value was never read from a JSON document. A declared int32
// (rune) type with a numeric raw value is handled earlier by
// jsonRawNumberCompatible and is unaffected.
func jsonCharacterValue(declaredType reflect.Type, value reflect.Value) (string, bool) {
	if declaredType == nil {
		return "", false
	}
	for declaredType.Kind() == reflect.Pointer {
		declaredType = declaredType.Elem()
	}
	if declaredType.Kind() != reflect.Int32 {
		return "", false
	}
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return "", false
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Int32 {
		return "", false
	}
	return string(value.Interface().(rune)), true
}
