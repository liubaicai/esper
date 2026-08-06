package esper

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// AvroRecord is the native Go underlying value for an Avro event schema.
// It mirrors Avro GenericRecord semantics without exposing the mutable map
// used internally: every declared field exists, an unset field reads as nil,
// and Set performs schema-directed type conversion.
//
// AvroRecord is intentionally schema-bound. A record can only be sent as the
// Avro event type whose schema was used to construct it. Use Clone before
// changing a record that may already be retained by a statement or window.
type AvroRecord struct {
	schema Schema
	values map[string]any
}

// NewAvroRecord constructs an empty record for an Avro schema. Declared
// fields are initialized to nil, matching GenericData.Record reads before a
// value is assigned.
func NewAvroRecord(schema Schema) (*AvroRecord, error) {
	return NewAvroRecordFromMap(schema, nil)
}

// NewAvroRecordFromMap constructs and validates a record from named values.
// Input values are copied and numeric values are safely converted to the
// declared field type when possible.
func NewAvroRecordFromMap(schema Schema, values map[string]any) (*AvroRecord, error) {
	if !schema.valid() || schema.Kind() != SchemaAvro {
		return nil, fmt.Errorf("esper: schema %q is not Avro", schema.Name())
	}
	record := &AvroRecord{
		schema: schema,
		values: make(map[string]any, len(schema.fields)+len(values)),
	}
	for _, field := range schema.fields {
		record.values[field.Name] = nil
	}
	for name, value := range values {
		if err := record.Set(name, value); err != nil {
			return nil, err
		}
	}
	return record, nil
}

// Schema returns the immutable event schema carried by the record.
func (r *AvroRecord) Schema() Schema {
	if r == nil {
		return Schema{}
	}
	return r.schema
}

// Get returns a field value. Unknown fields and nil receivers return nil,
// matching GenericRecord's convenient read shape; use Lookup when the caller
// needs to distinguish an unknown field from a declared nil field.
func (r *AvroRecord) Get(name string) any {
	value, _ := r.Lookup(name)
	return value
}

// Lookup returns a field value and whether the field exists in this record.
func (r *AvroRecord) Lookup(name string) (any, bool) {
	if r == nil || !r.schema.valid() {
		return nil, false
	}
	_, canonical, err := r.schema.lookupField(name)
	if err != nil {
		return nil, false
	}
	value, exists := r.values[canonical]
	return value, exists
}

// Set assigns one field after resolving its canonical name and validating its
// value against the declared Go type.
func (r *AvroRecord) Set(name string, value any) error {
	if r == nil {
		return fmt.Errorf("esper: cannot set field %q on a nil Avro record", name)
	}
	if !r.schema.valid() || r.schema.Kind() != SchemaAvro {
		return fmt.Errorf("esper: record has no valid Avro schema")
	}
	field, canonical, err := r.schema.lookupField(name)
	if err != nil {
		return err
	}
	converted := value
	if value != nil && field.Type != nil && field.Type != typeOf[any]() {
		assigned, assignErr := assignReflectValue(field.Type, value)
		if assignErr != nil {
			return fmt.Errorf("esper: Avro record %q field %q: %w", r.schema.Name(), canonical, assignErr)
		}
		converted = assigned.Interface()
	}
	if r.values == nil {
		r.values = make(map[string]any, len(r.schema.fields))
		for _, declared := range r.schema.fields {
			r.values[declared.Name] = nil
		}
	}
	r.values[canonical] = converted
	return nil
}

// AsMap returns an independent shallow copy of the record values.
func (r *AvroRecord) AsMap() map[string]any {
	if r == nil {
		return nil
	}
	result := make(map[string]any, len(r.values))
	for name, value := range r.values {
		result[name] = value
	}
	return result
}

// Clone returns an independently writable record with the same schema and
// field values.
func (r *AvroRecord) Clone() *AvroRecord {
	if r == nil {
		return nil
	}
	return &AvroRecord{schema: r.schema, values: r.AsMap()}
}

// MarshalJSON exposes the record as its field object for connectors and
// renderers that use encoding/json.
func (r *AvroRecord) MarshalJSON() ([]byte, error) {
	if r == nil {
		return []byte("null"), nil
	}
	return json.Marshal(r.values)
}

func avroRecordProperty(underlying any, name string) (Value, bool) {
	var record *AvroRecord
	switch current := underlying.(type) {
	case *AvroRecord:
		record = current
	case AvroRecord:
		record = &current
	default:
		return Missing(), false
	}
	if record == nil {
		return Null(), true
	}
	value, exists := record.Lookup(name)
	if !exists {
		return Missing(), true
	}
	return presentPropertyValue(value), true
}

func normalizeAvroRecord(schema Schema, underlying any) (*AvroRecord, error) {
	if !schema.valid() || schema.Kind() != SchemaAvro {
		return nil, fmt.Errorf("esper: schema %q is not Avro", schema.Name())
	}
	switch current := underlying.(type) {
	case *AvroRecord:
		if current == nil {
			return nil, fmt.Errorf("esper: Avro event %q record is nil", schema.Name())
		}
		if !avroSchemasEqual(schema, current.schema) {
			return nil, fmt.Errorf("esper: Avro event %q received record for schema %q", schema.Name(), current.schema.Name())
		}
		return current, nil
	case AvroRecord:
		if !avroSchemasEqual(schema, current.schema) {
			return nil, fmt.Errorf("esper: Avro event %q received record for schema %q", schema.Name(), current.schema.Name())
		}
		return current.Clone(), nil
	case map[string]any:
		return NewAvroRecordFromMap(schema, current)
	default:
		return nil, fmt.Errorf("esper: Avro event %q expects *esper.AvroRecord or map[string]any, got %T", schema.Name(), underlying)
	}
}

func avroSchemasEqual(left, right Schema) bool {
	if !left.valid() || !right.valid() || left.Kind() != SchemaAvro || right.Kind() != SchemaAvro {
		return false
	}
	if left.Name() != right.Name() || left.resolution != right.resolution || left.allowDynamic != right.allowDynamic || len(left.fields) != len(right.fields) {
		return false
	}
	for index, field := range left.fields {
		other := right.fields[index]
		if field.Name != other.Name || field.Type != other.Type || field.Optional != other.Optional || field.StartTimestamp != other.StartTimestamp || field.EndTimestamp != other.EndTimestamp {
			return false
		}
	}
	return true
}

func avroRecordType() reflect.Type {
	return reflect.TypeOf((*AvroRecord)(nil))
}
