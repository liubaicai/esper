package esper

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// AccessorStyle controls how Go struct properties are exposed. PUBLIC derives
// properties from exported fields, JAVABEAN discovers GetX/IsX and SetX
// methods, and EXPLICIT exposes only properties registered with schema options.
type AccessorStyle uint8

const (
	AccessorPublic AccessorStyle = iota
	AccessorExplicit
	AccessorJavaBean
)

// PropertyResolutionStyle controls case handling and ambiguity.
type PropertyResolutionStyle uint8

const (
	PropertyCaseSensitive PropertyResolutionStyle = iota
	PropertyCaseInsensitive
	PropertyDistinctCaseInsensitive
)

// SchemaKind identifies the underlying event representation.
type SchemaKind uint8

const (
	SchemaStruct SchemaKind = iota
	SchemaMap
	SchemaJSON
	SchemaXML
	SchemaAvro
	SchemaObjectArray
	SchemaVariant
)

// VariantMode controls how a variant schema resolves properties and accepts
// member event types. PREDEFINED exposes only properties common to all listed
// members; ANY accepts registered event types without a fixed member list and
// resolves properties dynamically as any.
type VariantMode uint8

const (
	VariantPredefined VariantMode = iota
	VariantAny
)

// FieldSpec describes a named event property.
type FieldSpec struct {
	Name           string
	Type           reflect.Type
	Optional       bool
	StartTimestamp bool
	EndTimestamp   bool
}

// PropertyAccessKind describes the shape of a declared property access.
type PropertyAccessKind uint8

const (
	PropertySimple PropertyAccessKind = iota
	PropertyIndexed
	PropertyMapped
	PropertyDynamic
)

// PropertyDescriptor is the Go-facing metadata contract for an event
// property. It is intentionally immutable from the caller's perspective.
type PropertyDescriptor struct {
	Name     string
	Type     reflect.Type
	Optional bool
	Kind     PropertyAccessKind
}

// FieldDef creates a field definition for NewSchema or NewMapSchema.
func FieldDef(name string, typ reflect.Type) FieldSpec {
	return FieldSpec{Name: name, Type: typ}
}

// OptionalFieldDef creates a nullable/optional field definition.
func OptionalFieldDef(name string, typ reflect.Type) FieldSpec {
	return FieldSpec{Name: name, Type: typ, Optional: true}
}

// PropertyGetter is a Go-facing explicit property accessor. The input is the
// event underlying value (rather than an Event envelope); Missing and Null
// may be returned when the accessor needs to preserve Esper's value state.
type PropertyGetter func(underlying any) (Value, error)

// PropertySetter writes one property on an addressable event underlying
// value. Struct materialization passes a pointer to the newly allocated Go
// value so callbacks can preserve setter-side validation and normalization.
type PropertySetter func(underlying any, value any) error

type schemaGetterSpec struct {
	name     string
	typ      reflect.Type
	optional bool
	getter   PropertyGetter
	method   string
	path     string
}

type schemaSetterSpec struct {
	name   string
	typ    reflect.Type
	setter PropertySetter
	method string
}

// SchemaOption changes event property resolution, accessor behavior, or
// dynamic-field behavior.
type SchemaOption func(*schemaConfig)

type schemaConfig struct {
	resolution   PropertyResolutionStyle
	accessor     AccessorStyle
	allowDynamic bool
	parents      []Schema
	getters      []schemaGetterSpec
	setters      []schemaSetterSpec
	nested       map[string]Schema
}

func WithPropertyResolution(style PropertyResolutionStyle) SchemaOption {
	return func(cfg *schemaConfig) { cfg.resolution = style }
}

func WithAccessorStyle(style AccessorStyle) SchemaOption {
	return func(cfg *schemaConfig) { cfg.accessor = style }
}

// WithPropertyGetter registers an explicit property accessor. The declared
// type is used for metadata and nested/indexed property resolution.
func WithPropertyGetter(name string, typ reflect.Type, getter PropertyGetter) SchemaOption {
	return func(cfg *schemaConfig) {
		cfg.getters = append(cfg.getters, schemaGetterSpec{name: name, typ: typ, getter: getter})
	}
}

// WithTypedPropertyGetter is the type-safe convenience form of
// WithPropertyGetter. A returned Go nil is represented as Null.
func WithTypedPropertyGetter[T any](name string, getter func(any) (T, error)) SchemaOption {
	return WithPropertyGetter(name, typeOf[T](), func(underlying any) (Value, error) {
		if getter == nil {
			return Missing(), fmt.Errorf("esper: property getter %q is nil", name)
		}
		value, err := getter(underlying)
		if err != nil {
			return Missing(), err
		}
		return presentPropertyValue(value), nil
	})
}

// WithPropertySetter registers an explicit property writer. The declared
// type participates in schema metadata validation and value conversion.
func WithPropertySetter(name string, typ reflect.Type, setter PropertySetter) SchemaOption {
	return func(cfg *schemaConfig) {
		cfg.setters = append(cfg.setters, schemaSetterSpec{name: name, typ: typ, setter: setter})
	}
}

// WithTypedPropertySetter is the type-safe convenience form of
// WithPropertySetter.
func WithTypedPropertySetter[T any](name string, setter func(any, T) error) SchemaOption {
	return WithPropertySetter(name, typeOf[T](), func(underlying any, value any) error {
		if setter == nil {
			return fmt.Errorf("esper: property setter %q is nil", name)
		}
		converted, err := assignReflectValue(typeOf[T](), value)
		if err != nil {
			return err
		}
		return setter(underlying, converted.Interface().(T))
	})
}

// WithPropertyMethod maps a property to an exported Go method. A method may
// be a JavaBean-style zero-argument getter, or accept indexed/mapped path
// arguments. When typ is omitted, the schema derives it from the method when
// possible and otherwise uses any.
func WithPropertyMethod(name, method string, typ ...reflect.Type) SchemaOption {
	var declared reflect.Type
	if len(typ) > 0 {
		declared = typ[0]
	}
	return func(cfg *schemaConfig) {
		cfg.getters = append(cfg.getters, schemaGetterSpec{name: name, typ: declared, method: method})
	}
}

// WithPropertySetterMethod maps a property to an exported Go method taking
// one value and returning either nothing or error.
func WithPropertySetterMethod(name, method string, typ ...reflect.Type) SchemaOption {
	var declared reflect.Type
	if len(typ) > 0 {
		declared = typ[0]
	}
	return func(cfg *schemaConfig) {
		cfg.setters = append(cfg.setters, schemaSetterSpec{name: name, typ: declared, method: method})
	}
}

// WithPropertyPath maps a property to a nested field/tag path on the event
// underlying value, for example "legacy.nested.value". The optional type is
// used as the root metadata type and is otherwise inferred when practical.
func WithPropertyPath(name, path string, typ ...reflect.Type) SchemaOption {
	var declared reflect.Type
	if len(typ) > 0 {
		declared = typ[0]
	}
	return func(cfg *schemaConfig) {
		cfg.getters = append(cfg.getters, schemaGetterSpec{name: name, typ: declared, path: path})
	}
}

// WithNestedPropertySchema associates a declared root property with the
// schema used for its nested object (or for each element of an indexed
// property). This keeps nested EXPLICIT accessors scoped to their own type.
func WithNestedPropertySchema(name string, nested Schema) SchemaOption {
	return func(cfg *schemaConfig) {
		if cfg.nested == nil {
			cfg.nested = make(map[string]Schema)
		}
		cfg.nested[name] = nested
	}
}

func AllowDynamicFields() SchemaOption {
	return func(cfg *schemaConfig) { cfg.allowDynamic = true }
}

// WithSchemaParent adds an event-type parent. Parent fields precede child
// fields, which also defines the positional order for ObjectArray schemas.
func WithSchemaParent(parent Schema) SchemaOption {
	return func(cfg *schemaConfig) { cfg.parents = append(cfg.parents, parent) }
}

// Schema is immutable after construction and safe for concurrent reads.
type Schema struct {
	name           string
	kind           SchemaKind
	fields         []FieldSpec
	fieldIndex     map[string]int
	getters        map[string]schemaGetterSpec
	setters        map[string]schemaSetterSpec
	nested         map[string]Schema
	goType         reflect.Type
	resolution     PropertyResolutionStyle
	accessor       AccessorStyle
	allowDynamic   bool
	variantMode    VariantMode
	variantMembers []string
	parents        []Schema
	variantSchemas []Schema
	parentNames    []string
}

// NewSchema constructs a statically described event schema.
func NewSchema(name string, fields ...FieldSpec) (Schema, error) {
	return newSchema(name, SchemaStruct, nil, fields, nil)
}

// NewSchemaWithOptions is the option-bearing form for callers that need
// explicit parent schemas or accessor policy on a generic schema.
func NewSchemaWithOptions(name string, fields []FieldSpec, opts ...SchemaOption) (Schema, error) {
	return newSchema(name, SchemaStruct, nil, fields, opts)
}

// NewMapSchema constructs a map-backed schema. Unknown fields are missing
// unless AllowDynamicFields is supplied.
func NewMapSchema(name string, fields []FieldSpec, opts ...SchemaOption) (Schema, error) {
	return newSchema(name, SchemaMap, nil, fields, opts)
}

// NewJSONSchema constructs a JSON-backed schema. Its runtime representation is
// a map, but the kind remains visible for rendering and diagnostics.
func NewJSONSchema(name string, fields []FieldSpec, opts ...SchemaOption) (Schema, error) {
	return newSchema(name, SchemaJSON, nil, fields, opts)
}

// NewJSONSchemaFor constructs a JSON event schema whose underlying value is a
// typed Go struct. When fields is empty, exported struct fields (including
// esper/json tags) provide the schema metadata, just like StructSchema. The
// JSON kind is retained for parser/renderer behavior while property access
// uses the typed underlying value.
func NewJSONSchemaFor[T any](name string, fields []FieldSpec, opts ...SchemaOption) (Schema, error) {
	typ := typeOf[T]()
	base := typ
	for base.Kind() == reflect.Pointer {
		base = base.Elem()
	}
	if base.Kind() != reflect.Struct {
		return Schema{}, fmt.Errorf("esper: typed JSON schema %q requires a struct type, got %s", name, typ)
	}
	if len(fields) == 0 && schemaConfigFromOptions(opts).accessor != AccessorExplicit {
		inferred, err := structFields(base, nil)
		if err != nil {
			return Schema{}, err
		}
		fields = inferred
	}
	return newSchema(name, SchemaJSON, typ, fields, opts)
}

func NewXMLSchema(name string, fields []FieldSpec, opts ...SchemaOption) (Schema, error) {
	return newSchema(name, SchemaXML, nil, fields, opts)
}

// NewAvroSchema describes an Avro datum represented at runtime by an
// *AvroRecord. Map and JSON inputs are accepted at ingestion boundaries and
// normalized to the schema-bound native record.
func NewAvroSchema(name string, fields []FieldSpec, opts ...SchemaOption) (Schema, error) {
	return newSchema(name, SchemaAvro, nil, fields, opts)
}

// NewObjectArraySchema describes an event whose properties are stored in a
// positional []any value. Field order is part of the immutable schema
// contract, matching Esper's object-array event representation.
func NewObjectArraySchema(name string, fields []FieldSpec, opts ...SchemaOption) (Schema, error) {
	return newSchema(name, SchemaObjectArray, nil, fields, opts)
}

// NewVariantSchema constructs a PREDEFINED variant schema from registered
// member schemas. The variant keeps the actual member Event at runtime while
// its declared fields are the common, type-compatible member properties.
func NewVariantSchema(name string, members ...Schema) (Schema, error) {
	if len(members) == 0 {
		return Schema{}, fmt.Errorf("esper: variant schema %q requires at least one member", name)
	}
	fields, err := commonVariantFields(members)
	if err != nil {
		return Schema{}, err
	}
	schema, err := newSchema(name, SchemaVariant, nil, fields, nil)
	if err != nil {
		return Schema{}, err
	}
	schema.variantMode = VariantPredefined
	schema.variantMembers = variantMemberNames(members)
	schema.variantSchemas = append([]Schema(nil), members...)
	return schema, nil
}

// NewAnyVariantSchema constructs an ANY variant. It accepts any event type
// registered in the environment and permits dynamic property expressions.
func NewAnyVariantSchema(name string, opts ...SchemaOption) (Schema, error) {
	options := append([]SchemaOption{AllowDynamicFields()}, opts...)
	schema, err := newSchema(name, SchemaVariant, nil, nil, options)
	if err != nil {
		return Schema{}, err
	}
	schema.variantMode = VariantAny
	return schema, nil
}

// StructSchema derives a schema from exported Go fields and `esper`/`json`
// tags. It is intentionally a top-level generic function because Go does not
// support generic methods.
func StructSchema[T any](name string, opts ...SchemaOption) (Schema, error) {
	var pointer *T
	typ := reflect.TypeOf(pointer).Elem()
	return newStructSchema(name, typ, opts)
}

func newStructSchema(name string, typ reflect.Type, opts []SchemaOption) (Schema, error) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return Schema{}, fmt.Errorf("esper: struct schema %q requires a struct type, got %s", name, typ)
	}
	var fields []FieldSpec
	if schemaConfigFromOptions(opts).accessor != AccessorExplicit {
		var err error
		fields, err = structFields(typ, nil)
		if err != nil {
			return Schema{}, err
		}
	}
	return newSchema(name, SchemaStruct, typ, fields, opts)
}

func structFields(typ reflect.Type, stack map[reflect.Type]bool) ([]FieldSpec, error) {
	if stack == nil {
		stack = make(map[reflect.Type]bool)
	}
	if stack[typ] {
		return nil, fmt.Errorf("esper: recursive embedded struct %s", typ)
	}
	stack[typ] = true
	defer delete(stack, typ)

	fields := make([]FieldSpec, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("esper")
		if tag == "-" {
			continue
		}
		name, options := parseFieldTag(tag)
		if name == "" {
			jsonTag := field.Tag.Get("json")
			name, options = parseFieldTag(jsonTag)
		}
		if name == "-" {
			continue
		}

		// Anonymous structs are flattened unless they carry an explicit tag.
		fieldType := field.Type
		baseType := fieldType
		for baseType.Kind() == reflect.Pointer {
			baseType = baseType.Elem()
		}
		if field.Anonymous && name == "" && baseType.Kind() == reflect.Struct {
			nested, err := structFields(baseType, stack)
			if err != nil {
				return nil, err
			}
			fields = append(fields, nested...)
			continue
		}
		if name == "" {
			if field.PkgPath != "" { // unexported
				continue
			}
			name = field.Name
		}
		if field.PkgPath != "" && field.Anonymous {
			continue
		}
		fieldSpec := FieldSpec{Name: name, Type: fieldType}
		for _, option := range options {
			switch option {
			case "optional":
				fieldSpec.Optional = true
			case "start", "start_timestamp":
				fieldSpec.StartTimestamp = true
			case "end", "end_timestamp":
				fieldSpec.EndTimestamp = true
			}
		}
		if fieldType.Kind() == reflect.Pointer || fieldType.Kind() == reflect.Interface {
			fieldSpec.Optional = true
		}
		fields = append(fields, fieldSpec)
	}
	return fields, nil
}

func parseFieldTag(tag string) (string, []string) {
	if tag == "" {
		return "", nil
	}
	parts := strings.Split(tag, ",")
	name := parts[0]
	if name == "" {
		return "", parts[1:]
	}
	return name, parts[1:]
}

func schemaConfigFromOptions(opts []SchemaOption) schemaConfig {
	cfg := schemaConfig{resolution: PropertyCaseSensitive, accessor: AccessorPublic}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

func newSchema(name string, kind SchemaKind, goType reflect.Type, fields []FieldSpec, opts []SchemaOption) (Schema, error) {
	if strings.TrimSpace(name) == "" {
		return Schema{}, fmt.Errorf("esper: schema name is required")
	}
	cfg := schemaConfigFromOptions(opts)
	copyFields := make([]FieldSpec, 0, len(fields))
	parentNames := make([]string, 0, len(cfg.parents))
	getters := make(map[string]schemaGetterSpec)
	setters := make(map[string]schemaSetterSpec)
	nestedSchemas := make(map[string]Schema)
	inheritedFields := make(map[string]FieldSpec)
	for index, parent := range cfg.parents {
		if !parent.valid() {
			return Schema{}, fmt.Errorf("esper: schema %q has invalid parent at index %d", name, index)
		}
		if parent.Name() == name {
			return Schema{}, fmt.Errorf("esper: schema %q cannot inherit itself", name)
		}
		parentNames = append(parentNames, parent.Name())
		for _, field := range parent.fields {
			if inherited, exists := inheritedFields[field.Name]; exists {
				if inherited.Type != field.Type || inherited.Optional != field.Optional || inherited.StartTimestamp != field.StartTimestamp || inherited.EndTimestamp != field.EndTimestamp {
					return Schema{}, fmt.Errorf("esper: schema %q inherits conflicting definitions for property %q", name, field.Name)
				}
				continue
			}
			inheritedFields[field.Name] = field
			copyFields = append(copyFields, field)
		}
		for getterName, getter := range parent.getters {
			getters[getterName] = getter
		}
		for setterName, setter := range parent.setters {
			setters[setterName] = setter
		}
		for nestedName, nested := range parent.nested {
			nestedSchemas[nestedName] = nested
		}
		cfg.allowDynamic = cfg.allowDynamic || parent.allowDynamic
	}
	if kind == SchemaAvro && cfg.allowDynamic {
		return Schema{}, fmt.Errorf("esper: Avro schema %q does not allow dynamic fields", name)
	}
	copyFields = append(copyFields, fields...)
	if len(cfg.setters) > 0 && goType == nil {
		return Schema{}, fmt.Errorf("esper: schema %q property setters require a Go struct underlying type", name)
	}

	for getterIndex, configured := range cfg.getters {
		getter, err := normalizeGetterSpec(configured, goType)
		if err != nil {
			return Schema{}, fmt.Errorf("esper: schema %q property accessor %d: %w", name, getterIndex, err)
		}
		if _, exists := getters[getter.name]; exists {
			return Schema{}, fmt.Errorf("esper: schema %q duplicates property accessor %q", name, getter.name)
		}
		getters[getter.name] = getter
		copyFields = mergeGetterField(copyFields, getter)
	}
	for setterIndex, configured := range cfg.setters {
		setter, err := normalizeSetterSpec(configured, goType)
		if err != nil {
			return Schema{}, fmt.Errorf("esper: schema %q property setter %d: %w", name, setterIndex, err)
		}
		if _, exists := setters[setter.name]; exists {
			return Schema{}, fmt.Errorf("esper: schema %q duplicates property setter %q", name, setter.name)
		}
		setters[setter.name] = setter
		var mergeErr error
		copyFields, mergeErr = mergeSetterField(copyFields, setter)
		if mergeErr != nil {
			return Schema{}, fmt.Errorf("esper: schema %q: %w", name, mergeErr)
		}
	}

	if cfg.accessor == AccessorJavaBean && goType != nil {
		for _, discovered := range discoverJavaBeanGetters(goType) {
			if _, exists := getters[discovered.name]; exists {
				continue
			}
			getters[discovered.name] = discovered
			copyFields = mergeGetterField(copyFields, discovered)
		}
		for _, discovered := range discoverJavaBeanSetters(goType) {
			if !schemaFieldsContain(copyFields, discovered.name) {
				// JavaBeans write-only descriptors are not event properties.
				// A setter augments an existing field/getter but never makes a
				// setter-only property readable through event metadata.
				continue
			}
			if _, exists := setters[discovered.name]; exists {
				continue
			}
			setters[discovered.name] = discovered
			var mergeErr error
			copyFields, mergeErr = mergeSetterField(copyFields, discovered)
			if mergeErr != nil {
				return Schema{}, fmt.Errorf("esper: schema %q: %w", name, mergeErr)
			}
		}
	}
	for nestedName, nested := range cfg.nested {
		nestedName = strings.TrimSpace(nestedName)
		if nestedName == "" {
			return Schema{}, fmt.Errorf("esper: schema %q has an empty nested property schema name", name)
		}
		if !nested.valid() {
			return Schema{}, fmt.Errorf("esper: schema %q has invalid nested property schema %q", name, nestedName)
		}
		if _, exists := nestedSchemas[nestedName]; exists {
			return Schema{}, fmt.Errorf("esper: schema %q duplicates nested property schema %q", name, nestedName)
		}
		nestedSchemas[nestedName] = nested
	}

	index := make(map[string]int, len(copyFields))
	for i, field := range copyFields {
		if strings.TrimSpace(field.Name) == "" {
			return Schema{}, fmt.Errorf("esper: schema %q contains an empty field name", name)
		}
		if field.Type == nil {
			field.Type = reflect.TypeOf((*any)(nil)).Elem()
			copyFields[i] = field
		}
		if _, exists := index[field.Name]; exists {
			return Schema{}, fmt.Errorf("esper: schema %q duplicates field %q", name, field.Name)
		}
		index[field.Name] = i
	}
	return Schema{
		name:         name,
		kind:         kind,
		fields:       copyFields,
		fieldIndex:   index,
		getters:      getters,
		setters:      setters,
		nested:       nestedSchemas,
		goType:       goType,
		resolution:   cfg.resolution,
		accessor:     cfg.accessor,
		allowDynamic: cfg.allowDynamic,
		parents:      append([]Schema(nil), cfg.parents...),
		parentNames:  parentNames,
	}, nil
}

func schemaFieldsContain(fields []FieldSpec, name string) bool {
	for _, field := range fields {
		if field.Name == name {
			return true
		}
	}
	return false
}

func normalizeGetterSpec(spec schemaGetterSpec, goType reflect.Type) (schemaGetterSpec, error) {
	spec.name = strings.TrimSpace(spec.name)
	if spec.name == "" {
		return schemaGetterSpec{}, fmt.Errorf("property accessor name is required")
	}
	segments, err := parsePropertyPath(spec.name)
	if err != nil || len(segments) != 1 || segments[0].name != spec.name || len(segments[0].accessors) != 0 {
		return schemaGetterSpec{}, fmt.Errorf("property accessor name %q must be a simple root property", spec.name)
	}
	spec.method = strings.TrimSpace(spec.method)
	spec.path = strings.TrimSpace(spec.path)
	if spec.getter == nil && spec.method == "" && spec.path == "" {
		return schemaGetterSpec{}, fmt.Errorf("property accessor %q has no getter, method, or path", spec.name)
	}
	if spec.getter != nil && (spec.method != "" || spec.path != "") {
		return schemaGetterSpec{}, fmt.Errorf("property accessor %q combines multiple accessor forms", spec.name)
	}
	if spec.method != "" && spec.path != "" {
		return schemaGetterSpec{}, fmt.Errorf("property accessor %q combines method and path", spec.name)
	}
	if spec.path != "" {
		if _, err := parsePropertyPath(spec.path); err != nil {
			return schemaGetterSpec{}, fmt.Errorf("property path %q: %w", spec.path, err)
		}
	}
	if spec.method != "" && goType != nil {
		methodType, err := propertyMethodReturnType(goType, spec.method)
		if err != nil {
			return schemaGetterSpec{}, err
		}
		if spec.typ == nil {
			spec.typ = methodType
		}
	}
	if spec.path != "" && spec.typ == nil && goType != nil {
		spec.typ = inferPropertyPathType(goType, spec.path)
	}
	if spec.typ == nil {
		spec.typ = typeOf[any]()
	}
	if !spec.optional {
		spec.optional = isOptionalReflectType(spec.typ)
	}
	return spec, nil
}

func normalizeSetterSpec(spec schemaSetterSpec, goType reflect.Type) (schemaSetterSpec, error) {
	spec.name = strings.TrimSpace(spec.name)
	if spec.name == "" {
		return schemaSetterSpec{}, fmt.Errorf("property setter name is required")
	}
	segments, err := parsePropertyPath(spec.name)
	if err != nil || len(segments) != 1 || segments[0].name != spec.name || len(segments[0].accessors) != 0 {
		return schemaSetterSpec{}, fmt.Errorf("property setter name %q must be a simple root property", spec.name)
	}
	spec.method = strings.TrimSpace(spec.method)
	if spec.setter == nil && spec.method == "" {
		return schemaSetterSpec{}, fmt.Errorf("property setter %q has no callback or method", spec.name)
	}
	if spec.setter != nil && spec.method != "" {
		return schemaSetterSpec{}, fmt.Errorf("property setter %q combines callback and method forms", spec.name)
	}
	if spec.method != "" && goType != nil {
		methodType, err := propertySetterMethodType(goType, spec.method)
		if err != nil {
			return schemaSetterSpec{}, err
		}
		if spec.typ == nil {
			spec.typ = methodType
		} else if spec.typ != typeOf[any]() && methodType != typeOf[any]() &&
			!spec.typ.AssignableTo(methodType) && !methodType.AssignableTo(spec.typ) && !numericTypes(spec.typ, methodType) {
			return schemaSetterSpec{}, fmt.Errorf("property setter %q declares %s but method accepts %s", spec.name, spec.typ, methodType)
		}
	}
	if spec.typ == nil {
		spec.typ = typeOf[any]()
	}
	return spec, nil
}

func isOptionalReflectType(typ reflect.Type) bool {
	if typ == nil {
		return true
	}
	return typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Interface || typ.Kind() == reflect.Map || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Func || typ.Kind() == reflect.Chan
}

func mergeGetterField(fields []FieldSpec, getter schemaGetterSpec) []FieldSpec {
	for index, field := range fields {
		if field.Name != getter.name {
			continue
		}
		if getter.typ != nil && getter.typ != typeOf[any]() {
			fields[index].Type = getter.typ
		}
		fields[index].Optional = fields[index].Optional || getter.optional
		return fields
	}
	return append(fields, FieldSpec{Name: getter.name, Type: getter.typ, Optional: getter.optional})
}

func mergeSetterField(fields []FieldSpec, setter schemaSetterSpec) ([]FieldSpec, error) {
	for index, field := range fields {
		if field.Name != setter.name {
			continue
		}
		if field.Type != nil && field.Type != typeOf[any]() && setter.typ != nil && setter.typ != typeOf[any]() &&
			!field.Type.AssignableTo(setter.typ) && !setter.typ.AssignableTo(field.Type) && !numericTypes(field.Type, setter.typ) {
			return nil, fmt.Errorf("property %q getter/field type %s conflicts with setter type %s", setter.name, field.Type, setter.typ)
		}
		if (field.Type == nil || field.Type == typeOf[any]()) && setter.typ != nil {
			fields[index].Type = setter.typ
		}
		return fields, nil
	}
	return append(fields, FieldSpec{Name: setter.name, Type: setter.typ, Optional: isOptionalReflectType(setter.typ)}), nil
}

func propertyMethodReturnType(typ reflect.Type, methodName string) (reflect.Type, error) {
	candidates := []reflect.Type{typ}
	if typ.Kind() != reflect.Pointer {
		candidates = append(candidates, reflect.PointerTo(typ))
	}
	for _, candidate := range candidates {
		method, ok := candidate.MethodByName(methodName)
		if !ok {
			continue
		}
		methodType := method.Type
		if methodType.NumIn() == 0 || methodType.NumOut() == 0 || methodType.NumOut() > 2 {
			return nil, fmt.Errorf("property method %q must return one value or (value, error)", methodName)
		}
		if methodType.NumOut() == 2 && !methodType.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			return nil, fmt.Errorf("property method %q second return value must implement error", methodName)
		}
		return methodType.Out(0), nil
	}
	return nil, fmt.Errorf("property method %q is not exported on %s", methodName, typ)
}

func propertySetterMethodType(typ reflect.Type, methodName string) (reflect.Type, error) {
	candidates := []reflect.Type{typ}
	if typ.Kind() != reflect.Pointer {
		candidates = append(candidates, reflect.PointerTo(typ))
	}
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	for _, candidate := range candidates {
		method, ok := candidate.MethodByName(methodName)
		if !ok {
			continue
		}
		methodType := method.Type
		if methodType.NumIn() != 2 || methodType.NumOut() > 1 {
			return nil, fmt.Errorf("property setter method %q must accept one value and return nothing or error", methodName)
		}
		if methodType.In(0).Kind() != reflect.Pointer {
			return nil, fmt.Errorf("property setter method %q must use a pointer receiver", methodName)
		}
		if methodType.NumOut() == 1 && !methodType.Out(0).Implements(errorType) {
			return nil, fmt.Errorf("property setter method %q return value must implement error", methodName)
		}
		return methodType.In(1), nil
	}
	return nil, fmt.Errorf("property setter method %q is not exported on %s", methodName, typ)
}

func discoverJavaBeanGetters(typ reflect.Type) []schemaGetterSpec {
	candidates := []reflect.Type{typ}
	if typ.Kind() != reflect.Pointer {
		candidates = append(candidates, reflect.PointerTo(typ))
	}
	byName := make(map[string]schemaGetterSpec)
	for _, candidate := range candidates {
		for index := 0; index < candidate.NumMethod(); index++ {
			method := candidate.Method(index)
			property, ok := javaBeanPropertyName(method.Name)
			if !ok || method.Type.NumIn() != 1 || method.Type.NumOut() == 0 || method.Type.NumOut() > 2 {
				continue
			}
			if strings.HasPrefix(method.Name, "Is") && method.Type.Out(0).Kind() != reflect.Bool {
				continue
			}
			if method.Type.NumOut() == 2 && !method.Type.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				continue
			}
			if _, exists := byName[property]; exists && strings.HasPrefix(method.Name, "Is") {
				continue
			}
			byName[property] = schemaGetterSpec{name: property, typ: method.Type.Out(0), method: method.Name, optional: isOptionalReflectType(method.Type.Out(0))}
		}
	}
	properties := make([]schemaGetterSpec, 0, len(byName))
	for _, getter := range byName {
		properties = append(properties, getter)
	}
	sort.Slice(properties, func(i, j int) bool { return properties[i].name < properties[j].name })
	return properties
}

func discoverJavaBeanSetters(typ reflect.Type) []schemaSetterSpec {
	candidates := []reflect.Type{typ}
	if typ.Kind() != reflect.Pointer {
		candidates = append(candidates, reflect.PointerTo(typ))
	}
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	byName := make(map[string]schemaSetterSpec)
	for _, candidate := range candidates {
		for index := 0; index < candidate.NumMethod(); index++ {
			method := candidate.Method(index)
			property, ok := javaBeanSetterPropertyName(method.Name)
			if !ok || method.Type.NumIn() != 2 || method.Type.NumOut() > 1 {
				continue
			}
			if method.Type.In(0).Kind() != reflect.Pointer {
				continue
			}
			if method.Type.NumOut() == 1 && !method.Type.Out(0).Implements(errorType) {
				continue
			}
			byName[property] = schemaSetterSpec{name: property, typ: method.Type.In(1), method: method.Name}
		}
	}
	properties := make([]schemaSetterSpec, 0, len(byName))
	for _, setter := range byName {
		properties = append(properties, setter)
	}
	sort.Slice(properties, func(i, j int) bool { return properties[i].name < properties[j].name })
	return properties
}

func javaBeanPropertyName(methodName string) (string, bool) {
	prefix := ""
	switch {
	case strings.HasPrefix(methodName, "Get") && len(methodName) > len("Get"):
		prefix = "Get"
	case strings.HasPrefix(methodName, "Is") && len(methodName) > len("Is"):
		prefix = "Is"
	default:
		return "", false
	}
	name := methodName[len(prefix):]
	if len(name) == 0 {
		return "", false
	}
	if len(name) > 1 && name[0] >= 'A' && name[0] <= 'Z' && name[1] >= 'A' && name[1] <= 'Z' {
		return name, true
	}
	return strings.ToLower(name[:1]) + name[1:], true
}

func javaBeanSetterPropertyName(methodName string) (string, bool) {
	if !strings.HasPrefix(methodName, "Set") || len(methodName) == len("Set") {
		return "", false
	}
	return javaBeanPropertyName("Get" + methodName[len("Set"):])
}

func inferPropertyPathType(root reflect.Type, path string) reflect.Type {
	segments, err := parsePropertyPath(path)
	if err != nil {
		return nil
	}
	current := root
	for _, segment := range segments {
		if segment.name != "" {
			var ok bool
			current, ok = nestedPropertyType(current, segment.name)
			if !ok {
				return nil
			}
		}
		for _, accessor := range segment.accessors {
			current = accessorPropertyType(current, accessor.kind)
		}
	}
	return current
}

func (s Schema) valid() bool { return s.name != "" }

func (s Schema) Name() string                                { return s.name }
func (s Schema) Kind() SchemaKind                            { return s.kind }
func (s Schema) GoType() reflect.Type                        { return s.goType }
func (s Schema) AccessorStyle() AccessorStyle                { return s.accessor }
func (s Schema) PropertyResolution() PropertyResolutionStyle { return s.resolution }

func (s Schema) VariantMode() VariantMode { return s.variantMode }

func (s Schema) IsVariantAny() bool { return s.kind == SchemaVariant && s.variantMode == VariantAny }

func (s Schema) VariantMembers() []string { return append([]string(nil), s.variantMembers...) }

func (s Schema) ParentNames() []string { return append([]string(nil), s.parentNames...) }

func (s Schema) AllowsDynamicProperties() bool { return s.allowDynamic }

func (s Schema) Fields() []FieldSpec { return append([]FieldSpec(nil), s.fields...) }

// Properties returns metadata for statically declared root properties.
func (s Schema) Properties() []PropertyDescriptor {
	properties := make([]PropertyDescriptor, 0, len(s.fields))
	for _, field := range s.fields {
		properties = append(properties, PropertyDescriptor{
			Name:     field.Name,
			Type:     field.Type,
			Optional: field.Optional,
			Kind:     propertyKindForType(field.Type),
		})
	}
	return properties
}

// PropertyNames returns a copy of the declared root property names.
func (s Schema) PropertyNames() []string {
	names := make([]string, 0, len(s.fields))
	for _, field := range s.fields {
		names = append(names, field.Name)
	}
	return names
}

// Property resolves metadata for a root or nested indexed/mapped property.
// Dynamic schemas return an any-typed descriptor for unknown properties.
func (s Schema) Property(name string) (PropertyDescriptor, bool) {
	segments, err := parsePropertyPath(name)
	if err != nil || len(segments) == 0 {
		return PropertyDescriptor{}, false
	}
	if index, ok := s.fieldIndex[name]; ok {
		field := s.fields[index]
		descriptor := PropertyDescriptor{Name: name, Type: field.Type, Optional: field.Optional, Kind: PropertySimple}
		for _, segment := range segments {
			for _, accessor := range segment.accessors {
				descriptor.Kind = propertyAccessKind(accessor.kind)
			}
		}
		return descriptor, true
	}
	if len(segments) == 1 && len(segments[0].accessors) == 0 {
		if field, canonical, lookupErr := s.lookupField(name); lookupErr == nil {
			kind := PropertySimple
			if _, declared := s.fieldIndex[canonical]; !declared && s.allowDynamic {
				kind = PropertyDynamic
			}
			return PropertyDescriptor{Name: name, Type: field.Type, Optional: field.Optional, Kind: kind}, true
		}
	}
	field, canonical, lookupErr := s.lookupField(segments[0].name)
	if lookupErr != nil {
		return PropertyDescriptor{}, false
	}
	descriptor := PropertyDescriptor{Name: name, Type: field.Type, Optional: field.Optional, Kind: PropertySimple}
	if canonical == "" {
		canonical = field.Name
	}
	propertySchema := s
	for segmentIndex, segment := range segments {
		if segmentIndex > 0 {
			nestedSchemaFound := false
			if nested, ok := propertySchema.lookupNestedSchema(segments[segmentIndex-1].name); ok {
				propertySchema = nested
				nestedSchemaFound = true
			}
			if nestedSchemaFound {
				nestedField, _, nestedErr := propertySchema.lookupField(segment.name)
				if nestedErr == nil {
					descriptor.Type = nestedField.Type
					descriptor.Optional = descriptor.Optional || nestedField.Optional
				} else {
					descriptor.Type = typeOf[any]()
					descriptor.Optional = true
				}
			} else {
				var ok bool
				descriptor.Type, ok = nestedPropertyType(descriptor.Type, segment.name)
				if !ok {
					descriptor.Type = typeOf[any]()
					descriptor.Optional = true
				}
			}
		}
		for _, accessor := range segment.accessors {
			descriptor.Kind = propertyAccessKind(accessor.kind)
			descriptor.Type = accessorPropertyType(descriptor.Type, accessor.kind)
			descriptor.Optional = descriptor.Optional || descriptor.Type == typeOf[any]()
		}
	}
	return descriptor, true
}

// PropertyType returns the declared result type for a property path.
func (s Schema) PropertyType(name string) (reflect.Type, bool) {
	descriptor, ok := s.Property(name)
	if !ok {
		return nil, false
	}
	return descriptor.Type, true
}

func propertyKindForType(typ reflect.Type) PropertyAccessKind {
	if typ == nil {
		return PropertyDynamic
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Map:
		return PropertyMapped
	case reflect.Array, reflect.Slice:
		return PropertyIndexed
	default:
		return PropertySimple
	}
}

func propertyAccessKind(kind propertyAccessorKind) PropertyAccessKind {
	if kind == propertyMap {
		return PropertyMapped
	}
	return PropertyIndexed
}

func accessorPropertyType(typ reflect.Type, kind propertyAccessorKind) reflect.Type {
	if typ == nil {
		return typeOf[any]()
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if kind == propertyIndex && (typ.Kind() == reflect.Array || typ.Kind() == reflect.Slice) {
		return typ.Elem()
	}
	if kind == propertyMap && typ.Kind() == reflect.Map {
		return typ.Elem()
	}
	return typeOf[any]()
}

func nestedPropertyType(typ reflect.Type, name string) (reflect.Type, bool) {
	if typ == nil {
		return typeOf[any](), false
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() == reflect.Map || typ.Kind() == reflect.Interface {
		return typeOf[any](), true
	}
	if typ.Kind() != reflect.Struct {
		return nil, false
	}
	fields, err := structFields(typ, nil)
	if err != nil {
		return nil, false
	}
	for _, field := range fields {
		if field.Name == name || strings.EqualFold(field.Name, name) {
			return field.Type, true
		}
	}
	return nil, false
}

func (s Schema) Field(name string) (FieldSpec, bool) {
	field, _, err := s.lookupField(name)
	return field, err == nil
}

func (s Schema) lookupField(name string) (FieldSpec, string, error) {
	if index, ok := s.fieldIndex[name]; ok {
		return s.fields[index], name, nil
	}
	if s.resolution == PropertyCaseSensitive {
		if s.allowDynamic {
			return FieldSpec{Name: name, Type: reflect.TypeOf((*any)(nil)).Elem()}, name, nil
		}
		return FieldSpec{}, "", fmt.Errorf("esper: schema %q has no field %q", s.name, name)
	}
	var match *FieldSpec
	var matchedName string
	for _, field := range s.fields {
		if strings.EqualFold(field.Name, name) {
			copyField := field
			if match != nil && s.resolution == PropertyDistinctCaseInsensitive {
				return FieldSpec{}, "", fmt.Errorf("esper: schema %q has ambiguous field %q", s.name, name)
			}
			match = &copyField
			matchedName = field.Name
		}
	}
	if match != nil {
		return *match, matchedName, nil
	}
	if s.allowDynamic {
		return FieldSpec{Name: name, Type: reflect.TypeOf((*any)(nil)).Elem()}, name, nil
	}
	return FieldSpec{}, "", fmt.Errorf("esper: schema %q has no field %q", s.name, name)
}

func (s Schema) lookupGetter(name string) (schemaGetterSpec, string, bool) {
	if getter, ok := s.getters[name]; ok {
		return getter, name, true
	}
	if s.resolution == PropertyCaseSensitive {
		return schemaGetterSpec{}, "", false
	}
	var match *schemaGetterSpec
	matchedName := ""
	for propertyName, getter := range s.getters {
		if !strings.EqualFold(propertyName, name) {
			continue
		}
		if match != nil && s.resolution == PropertyDistinctCaseInsensitive {
			return schemaGetterSpec{}, "", false
		}
		copyGetter := getter
		match = &copyGetter
		matchedName = propertyName
	}
	if match == nil {
		return schemaGetterSpec{}, "", false
	}
	return *match, matchedName, true
}

func (s Schema) lookupSetter(name string) (schemaSetterSpec, string, bool) {
	if setter, ok := s.setters[name]; ok {
		return setter, name, true
	}
	if s.resolution == PropertyCaseSensitive {
		return schemaSetterSpec{}, "", false
	}
	var match *schemaSetterSpec
	matchedName := ""
	for propertyName, setter := range s.setters {
		if !strings.EqualFold(propertyName, name) {
			continue
		}
		if match != nil && s.resolution == PropertyDistinctCaseInsensitive {
			return schemaSetterSpec{}, "", false
		}
		copySetter := setter
		match = &copySetter
		matchedName = propertyName
	}
	if match == nil {
		return schemaSetterSpec{}, "", false
	}
	return *match, matchedName, true
}

func (s Schema) lookupNestedSchema(name string) (Schema, bool) {
	if nested, ok := s.nested[name]; ok {
		return nested, true
	}
	if s.resolution == PropertyCaseSensitive {
		return Schema{}, false
	}
	var match Schema
	found := false
	for nestedName, nested := range s.nested {
		if !strings.EqualFold(nestedName, name) {
			continue
		}
		if found && s.resolution == PropertyDistinctCaseInsensitive {
			return Schema{}, false
		}
		match = nested
		found = true
	}
	return match, found
}

func (s Schema) acceptsEventType(eventType string) bool {
	if s.kind != SchemaVariant || strings.TrimSpace(eventType) == "" {
		return false
	}
	if s.variantMode == VariantAny {
		return true
	}
	for _, member := range s.variantMembers {
		if member == eventType {
			return true
		}
	}
	return false
}

func (s Schema) acceptsEventSchema(event Schema) bool {
	if s.kind != SchemaVariant || !event.valid() {
		return false
	}
	if s.variantMode == VariantAny {
		return true
	}
	for _, member := range s.variantSchemas {
		if schemaDescendsFrom(event, member.Name(), make(map[string]struct{})) {
			return true
		}
	}
	return false
}

func schemaDescendsFrom(event Schema, targetName string, visited map[string]struct{}) bool {
	if !event.valid() || strings.TrimSpace(targetName) == "" {
		return false
	}
	if event.Name() == targetName {
		return true
	}
	if visited == nil {
		visited = make(map[string]struct{})
	}
	if _, seen := visited[event.Name()]; seen {
		return false
	}
	visited[event.Name()] = struct{}{}
	for _, parent := range event.parents {
		if schemaDescendsFrom(parent, targetName, visited) {
			return true
		}
	}
	return false
}

func (s Schema) get(underlying any, name string) Value {
	if underlying == nil {
		return Null()
	}
	if value, record := avroRecordProperty(underlying, name); record && !value.IsMissing() {
		return value
	}
	if values, ok := underlying.(map[string]Value); ok {
		if value, exists := values[name]; exists {
			return value
		}
	}
	if values, ok := underlying.(map[string]any); ok {
		if value, exists := values[name]; exists {
			return presentPropertyValue(value)
		}
	}
	segments, err := parsePropertyPath(name)
	if err != nil {
		return Missing()
	}
	current := Present(underlying)
	currentSchema := s
	for _, segment := range segments {
		if segment.name != "" {
			current = currentSchema.getOneWithAccessors(current.Any(), segment.name, segment.accessors)
		} else {
			for _, accessor := range segment.accessors {
				current = applyPropertyAccessor(current, accessor)
			}
		}
		if !current.IsPresent() {
			return current
		}
		if segment.name != "" {
			if nested, ok := currentSchema.lookupNestedSchema(segment.name); ok {
				currentSchema = nested
			}
		}
	}
	return current
}

// propertyPathSegment is the parsed form of one event-property path segment.
// A segment can contain a named property followed by literal indexed or
// mapped access, for example "items[0]" or "labels('primary')".  The
// optional "?" suffix used by Esper property getters is accepted and has no
// effect on the Value state: Missing and Null remain distinguishable.
type propertyPathSegment struct {
	name      string
	accessors []propertyAccessor
}

type propertyAccessor struct {
	kind  propertyAccessorKind
	index int
	key   string
}

type propertyAccessorKind uint8

const (
	propertyIndex propertyAccessorKind = iota
	propertyMap
)

func getPropertyPath(underlying any, name string, one func(any, string) Value) Value {
	if value, record := avroRecordProperty(underlying, name); record && !value.IsMissing() {
		return value
	}
	if values, ok := underlying.(map[string]Value); ok {
		if value, exists := values[name]; exists {
			return value
		}
	}
	if values, ok := underlying.(map[string]any); ok {
		if value, exists := values[name]; exists {
			return presentPropertyValue(value)
		}
	}
	segments, err := parsePropertyPath(name)
	if err != nil {
		return Missing()
	}
	current := Present(underlying)
	for _, segment := range segments {
		if segment.name != "" {
			current = one(current.Any(), segment.name)
		}
		for _, accessor := range segment.accessors {
			current = applyPropertyAccessor(current, accessor)
		}
		if !current.IsPresent() {
			return current
		}
	}
	return current
}

func parsePropertyPath(path string) ([]propertyPathSegment, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("empty property path")
	}
	parts, err := splitPropertyPath(path)
	if err != nil {
		return nil, err
	}
	segments := make([]propertyPathSegment, 0, len(parts))
	for _, part := range parts {
		segment, err := parsePropertySegment(part)
		if err != nil {
			return nil, err
		}
		segments = append(segments, segment)
	}
	return segments, nil
}

func splitPropertyPath(path string) ([]string, error) {
	parts := make([]string, 0, 4)
	start := 0
	parenDepth, bracketDepth := 0, 0
	var quote byte
	escaped := false
	for index := 0; index < len(path); index++ {
		character := path[index]
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if character == quote {
				quote = 0
			}
			continue
		}
		switch character {
		case '\'', '"':
			if parenDepth > 0 {
				quote = character
			}
		case '(':
			parenDepth++
		case ')':
			parenDepth--
			if parenDepth < 0 {
				return nil, fmt.Errorf("unmatched ')' in property path %q", path)
			}
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
			if bracketDepth < 0 {
				return nil, fmt.Errorf("unmatched ']' in property path %q", path)
			}
		case '.':
			if parenDepth == 0 && bracketDepth == 0 {
				part := strings.TrimSpace(path[start:index])
				if part == "" {
					return nil, fmt.Errorf("empty segment in property path %q", path)
				}
				parts = append(parts, part)
				start = index + 1
			}
		}
	}
	if escaped || quote != 0 || parenDepth != 0 || bracketDepth != 0 {
		return nil, fmt.Errorf("unterminated accessor in property path %q", path)
	}
	part := strings.TrimSpace(path[start:])
	if part == "" {
		return nil, fmt.Errorf("empty segment in property path %q", path)
	}
	parts = append(parts, part)
	return parts, nil
}

func parsePropertySegment(part string) (propertyPathSegment, error) {
	segment := propertyPathSegment{}
	part = strings.TrimSpace(part)
	index := 0
	for index < len(part) && part[index] != '[' && part[index] != '(' && part[index] != '?' {
		index++
	}
	segment.name = unescapePropertyName(strings.TrimSpace(part[:index]))
	if segment.name == "" {
		return propertyPathSegment{}, fmt.Errorf("empty property name in %q", part)
	}
	for index < len(part) {
		switch part[index] {
		case '?':
			if strings.TrimSpace(part[index+1:]) != "" {
				return propertyPathSegment{}, fmt.Errorf("unexpected text after '?' in %q", part)
			}
			index = len(part)
		case '[':
			close := strings.IndexByte(part[index+1:], ']')
			if close < 0 {
				return propertyPathSegment{}, fmt.Errorf("unterminated index in %q", part)
			}
			close += index + 1
			text := strings.TrimSpace(part[index+1 : close])
			if text == "" {
				return propertyPathSegment{}, fmt.Errorf("empty index in %q", part)
			}
			position, err := strconv.Atoi(text)
			if err != nil {
				return propertyPathSegment{}, fmt.Errorf("invalid index %q in %q", text, part)
			}
			segment.accessors = append(segment.accessors, propertyAccessor{kind: propertyIndex, index: position})
			index = close + 1
		case '(':
			close, key, err := parseMappedAccessor(part, index)
			if err != nil {
				return propertyPathSegment{}, err
			}
			segment.accessors = append(segment.accessors, propertyAccessor{kind: propertyMap, key: key})
			index = close + 1
		default:
			return propertyPathSegment{}, fmt.Errorf("unexpected character %q in property path %q", part[index], part)
		}
	}
	return segment, nil
}

func parseMappedAccessor(part string, start int) (int, string, error) {
	index := start + 1
	for index < len(part) && part[index] == ' ' {
		index++
	}
	if index >= len(part) {
		return 0, "", fmt.Errorf("unterminated mapped accessor in %q", part)
	}
	var key string
	if part[index] == '\'' || part[index] == '"' {
		quote := part[index]
		index++
		var builder strings.Builder
		escaped := false
		for index < len(part) {
			character := part[index]
			index++
			if escaped {
				builder.WriteByte(character)
				escaped = false
				continue
			}
			if character == '\\' {
				escaped = true
				continue
			}
			if character == quote {
				break
			}
			builder.WriteByte(character)
		}
		if index > len(part) || index == 0 || part[index-1] != quote {
			return 0, "", fmt.Errorf("unterminated mapped key in %q", part)
		}
		key = builder.String()
		for index < len(part) && part[index] == ' ' {
			index++
		}
	} else {
		close := strings.IndexByte(part[index:], ')')
		if close < 0 {
			return 0, "", fmt.Errorf("unterminated mapped accessor in %q", part)
		}
		close += index
		key = strings.TrimSpace(part[index:close])
		if key == "" {
			return 0, "", fmt.Errorf("empty mapped key in %q", part)
		}
		return close, unescapePropertyName(key), nil
	}
	if index >= len(part) || part[index] != ')' {
		return 0, "", fmt.Errorf("mapped accessor requires ')' in %q", part)
	}
	return index, key, nil
}

func unescapePropertyName(name string) string {
	if !strings.Contains(name, "\\") {
		return name
	}
	var builder strings.Builder
	escaped := false
	for index := 0; index < len(name); index++ {
		character := name[index]
		if escaped {
			builder.WriteByte(character)
			escaped = false
			continue
		}
		if character == '\\' {
			escaped = true
			continue
		}
		builder.WriteByte(character)
	}
	if escaped {
		builder.WriteByte('\\')
	}
	return builder.String()
}

func applyPropertyAccessor(value Value, accessor propertyAccessor) Value {
	if !value.IsPresent() {
		return value
	}
	underlying := value.Any()
	if underlying == nil {
		return value
	}
	if wrapped, ok := underlying.(Value); ok {
		return applyPropertyAccessor(wrapped, accessor)
	}
	switch accessor.kind {
	case propertyIndex:
		return indexedValue(underlying, accessor.index)
	case propertyMap:
		return mappedValue(underlying, accessor.key)
	default:
		return Missing()
	}
}

func indexedValue(underlying any, index int) Value {
	if index < 0 {
		return Missing()
	}
	value := reflect.ValueOf(underlying)
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return Null()
		}
		value = value.Elem()
	}
	if !value.IsValid() || (value.Kind() != reflect.Array && value.Kind() != reflect.Slice) || index >= value.Len() {
		return Missing()
	}
	return reflectValueToValue(value.Index(index))
}

func mappedValue(underlying any, key string) Value {
	if values, ok := underlying.(map[string]Value); ok {
		if value, exists := values[key]; exists {
			return value
		}
		return Missing()
	}
	value := reflect.ValueOf(underlying)
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return Null()
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Map {
		return Missing()
	}
	mapKey := reflect.ValueOf(key)
	if !mapKey.Type().AssignableTo(value.Type().Key()) {
		if mapKey.Type().ConvertibleTo(value.Type().Key()) {
			mapKey = mapKey.Convert(value.Type().Key())
		} else {
			return Missing()
		}
	}
	return reflectValueToValue(value.MapIndex(mapKey))
}

func reflectValueToValue(value reflect.Value) Value {
	if !value.IsValid() {
		return Missing()
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return Null()
		}
		value = value.Elem()
	}
	if !value.IsValid() || !value.CanInterface() {
		return Missing()
	}
	if wrapped, ok := value.Interface().(Value); ok {
		return wrapped
	}
	return Present(value.Interface())
}

func (s Schema) getOneWithAccessors(underlying any, name string, accessors []propertyAccessor) Value {
	if getter, _, ok := s.lookupGetter(name); ok {
		value, invoked := invokeRegisteredGetter(underlying, getter, accessors, s.resolution)
		if invoked {
			return value
		}
		return Missing()
	}
	value := s.getOne(underlying, name)
	if value.IsMissing() && s.accessor == AccessorExplicit && !schemaRootUnderlying(underlying, s.goType) {
		value = rawPropertyValue(underlying, name, s.resolution)
	}
	for _, accessor := range accessors {
		value = applyPropertyAccessor(value, accessor)
	}
	return value
}

func invokeRegisteredGetter(underlying any, getter schemaGetterSpec, accessors []propertyAccessor, resolution PropertyResolutionStyle) (Value, bool) {
	target := propertyUnderlying(underlying)
	if getter.getter != nil {
		value, err := getter.getter(target)
		if err != nil {
			return Missing(), true
		}
		for _, accessor := range accessors {
			value = applyPropertyAccessor(value, accessor)
		}
		return value, true
	}
	if getter.method != "" {
		if len(accessors) > 0 {
			if value, ok := invokeRegisteredPropertyMethod(target, getter.method, accessors); ok {
				return value, true
			}
		}
		value, ok := invokeRegisteredPropertyMethod(target, getter.method, nil)
		if !ok {
			return Missing(), false
		}
		for _, accessor := range accessors {
			value = applyPropertyAccessor(value, accessor)
		}
		return value, true
	}
	if getter.path != "" {
		value := getPropertyPath(target, getter.path, func(value any, property string) Value {
			return rawPropertyValue(value, property, resolution)
		})
		for _, accessor := range accessors {
			value = applyPropertyAccessor(value, accessor)
		}
		return value, true
	}
	return Missing(), false
}

func propertyUnderlying(underlying any) any {
	if wrapped, ok := underlying.(Value); ok {
		if !wrapped.IsPresent() {
			return nil
		}
		underlying = wrapped.Any()
	}
	if event, ok := underlying.(Event); ok {
		return event.Underlying()
	}
	return underlying
}

func invokeRegisteredPropertyMethod(underlying any, method string, accessors []propertyAccessor) (value Value, ok bool) {
	if underlying == nil {
		return Missing(), false
	}
	arguments := make([]Value, 0, len(accessors))
	for _, accessor := range accessors {
		switch accessor.kind {
		case propertyIndex:
			arguments = append(arguments, Present(accessor.index))
		case propertyMap:
			arguments = append(arguments, Present(accessor.key))
		}
	}
	defer func() {
		if recover() != nil {
			value, ok = Missing(), false
		}
	}()
	result, status := invokeReflectMethod(underlying, method, arguments)
	if status == reflectMethodMissing {
		return Missing(), false
	}
	if status == reflectMethodNull {
		return Null(), true
	}
	return reflectValueToValue(result), true
}

func rawPropertyValue(underlying any, name string, resolution PropertyResolutionStyle) Value {
	if underlying == nil {
		return Null()
	}
	if wrapped, ok := underlying.(Value); ok {
		if !wrapped.IsPresent() {
			return wrapped
		}
		underlying = wrapped.Any()
	}
	if event, ok := underlying.(Event); ok {
		return event.Get(name)
	}
	if row, ok := underlying.(Row); ok {
		return row.Get(name)
	}
	if value, record := avroRecordProperty(underlying, name); record {
		return value
	}
	if values, ok := underlying.(map[string]Value); ok {
		if value, exists := values[name]; exists {
			return value
		}
		return Missing()
	}
	if values, ok := underlying.(map[string]any); ok {
		if value, exists := values[name]; exists {
			return presentPropertyValue(value)
		}
		return Missing()
	}
	value := reflect.ValueOf(underlying)
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return Null()
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return Null()
	}
	if value.Kind() == reflect.Map {
		return reflectMapValue(value, name)
	}
	fieldValue := structFieldValue(value, name, resolution, nil)
	if !fieldValue.IsValid() || !fieldValue.CanInterface() {
		return Missing()
	}
	return reflectValueToValue(fieldValue)
}

func (s Schema) getOne(underlying any, name string) Value {
	if underlying == nil {
		return Null()
	}
	if wrapped, ok := underlying.(Value); ok {
		if !wrapped.IsPresent() {
			return wrapped
		}
		underlying = wrapped.Any()
		if underlying == nil {
			return Null()
		}
	}
	if row, ok := underlying.(Row); ok {
		return row.Get(name)
	}
	if event, ok := underlying.(Event); ok {
		return event.Get(name)
	}
	if value, record := avroRecordProperty(underlying, name); record {
		return value
	}
	if values, ok := underlying.(map[string]Value); ok {
		if value, exists := values[name]; exists {
			return value
		}
		return Missing()
	}
	if values, ok := underlying.(map[string]any); ok {
		if value, exists := values[name]; exists {
			return presentPropertyValue(value)
		}
		canonicalName := name
		if field, canonical, err := s.lookupField(name); err == nil {
			canonicalName = field.Name
			if canonical != "" {
				canonicalName = canonical
			}
		}
		if value, exists := values[canonicalName]; exists {
			return presentPropertyValue(value)
		}
		if s.resolution != PropertyCaseSensitive {
			for key, value := range values {
				if strings.EqualFold(key, canonicalName) {
					return presentPropertyValue(value)
				}
			}
		}
		return Missing()
	}

	value := reflect.ValueOf(underlying)
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return Null()
		}
		value = value.Elem()
	}
	if value.Kind() == reflect.Map {
		return reflectMapValue(value, name)
	}
	if value.Kind() == reflect.Array || value.Kind() == reflect.Slice {
		field, canonicalName, err := s.lookupField(name)
		if err != nil {
			return Missing()
		}
		index, exists := s.fieldIndex[canonicalName]
		if !exists || index < 0 || index >= value.Len() {
			_ = field
			return Missing()
		}
		item := value.Index(index)
		if item.Kind() == reflect.Interface && item.IsNil() {
			return Null()
		}
		if item.Kind() == reflect.Pointer && item.IsNil() {
			return Null()
		}
		if item.CanInterface() {
			if typed, ok := item.Interface().(Value); ok {
				return typed
			}
			return Present(item.Interface())
		}
		return Missing()
	}
	if value.Kind() != reflect.Struct {
		return Missing()
	}
	fieldSpec, canonicalName, err := s.lookupField(name)
	if err != nil {
		if s.accessor != AccessorExplicit {
			return dynamicStructField(underlying, name, s.resolution)
		}
		return Missing()
	}
	_ = fieldSpec
	fieldValue := structFieldValue(value, canonicalName, s.resolution, nil)
	if !fieldValue.IsValid() || !fieldValue.CanInterface() {
		return Missing()
	}
	if (fieldValue.Kind() == reflect.Pointer || fieldValue.Kind() == reflect.Interface) && fieldValue.IsNil() {
		return Null()
	}
	return Present(fieldValue.Interface())
}

func schemaRootUnderlying(underlying any, schemaType reflect.Type) bool {
	if schemaType == nil {
		return true
	}
	underlying = propertyUnderlying(underlying)
	if underlying == nil {
		return false
	}
	got := reflect.TypeOf(underlying)
	if got.AssignableTo(schemaType) {
		return true
	}
	return got.Kind() == reflect.Pointer && got.Elem().AssignableTo(schemaType)
}

func reflectMapValue(value reflect.Value, name string) Value {
	if !value.IsValid() || value.Kind() != reflect.Map {
		return Missing()
	}
	key := reflect.ValueOf(name)
	if !key.Type().AssignableTo(value.Type().Key()) {
		if key.Type().ConvertibleTo(value.Type().Key()) {
			key = key.Convert(value.Type().Key())
		} else {
			return Missing()
		}
	}
	return reflectValueToValue(value.MapIndex(key))
}

func presentPropertyValue(value any) Value {
	if value == nil {
		return Null()
	}
	if wrapped, ok := value.(Value); ok {
		return wrapped
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if reflected.IsNil() {
			return Null()
		}
	}
	return Present(value)
}

func dynamicStructField(underlying any, name string, resolution PropertyResolutionStyle) Value {
	value := reflect.ValueOf(underlying)
	fieldValue := structFieldValue(value, name, resolution, nil)
	if fieldValue.IsValid() && fieldValue.CanInterface() {
		if (fieldValue.Kind() == reflect.Pointer || fieldValue.Kind() == reflect.Interface) && fieldValue.IsNil() {
			return Null()
		}
		return Present(fieldValue.Interface())
	}
	if methodValue, ok := invokePropertyMethod(value, name); ok {
		return methodValue
	}
	return Missing()
}

func invokePropertyMethod(value reflect.Value, name string) (Value, bool) {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return Null(), true
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return Missing(), false
	}
	target := strings.TrimSpace(name)
	if target == "" {
		return Missing(), false
	}
	candidates := []string{target}
	upper := strings.ToUpper(target[:1]) + target[1:]
	candidates = append(candidates, "Get"+upper, "Is"+upper)
	for _, candidate := range candidates {
		method := value.MethodByName(candidate)
		if !method.IsValid() && value.CanAddr() {
			method = value.Addr().MethodByName(candidate)
		}
		if !method.IsValid() || method.Type().NumIn() != 0 || method.Type().NumOut() == 0 || method.Type().NumOut() > 2 {
			continue
		}
		var outputs []reflect.Value
		called := true
		func() {
			defer func() {
				if recover() != nil {
					called = false
				}
			}()
			outputs = method.Call(nil)
		}()
		if !called || len(outputs) == 0 {
			continue
		}
		if len(outputs) == 2 && !isNilReflectValue(outputs[1]) {
			if err, ok := outputs[1].Interface().(error); ok && err != nil {
				return Missing(), true
			}
		}
		return reflectValueToValue(outputs[0]), true
	}
	return Missing(), false
}

func isNilReflectValue(value reflect.Value) bool {
	if !value.IsValid() {
		return true
	}
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func structFieldValue(value reflect.Value, target string, resolution PropertyResolutionStyle, stack map[reflect.Type]bool) reflect.Value {
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	if stack == nil {
		stack = make(map[reflect.Type]bool)
	}
	if stack[value.Type()] {
		return reflect.Value{}
	}
	stack[value.Type()] = true
	defer delete(stack, value.Type())

	typ := value.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("esper")
		if tag == "-" {
			continue
		}
		name, _ := parseFieldTag(tag)
		if name == "" {
			name, _ = parseFieldTag(field.Tag.Get("json"))
		}
		if name == "-" {
			continue
		}
		fieldValue := value.Field(i)
		baseType := field.Type
		for baseType.Kind() == reflect.Pointer {
			baseType = baseType.Elem()
		}
		if field.Anonymous && name == "" && baseType.Kind() == reflect.Struct {
			if nested := structFieldValue(fieldValue, target, resolution, stack); nested.IsValid() {
				return nested
			}
			continue
		}
		if name == "" {
			name = field.Name
		}
		matches := name == target || field.Name == target
		if resolution != PropertyCaseSensitive {
			matches = strings.EqualFold(name, target) || strings.EqualFold(field.Name, target)
		}
		if matches {
			return fieldValue
		}
	}
	return reflect.Value{}
}

type eventIdentityToken struct {
	marker byte
}

// Event is a typed event envelope used by the runtime and listeners. The
// private identity token is stable across envelope copies and distinguishes
// two separately ingested events that happen to contain equal values.
type Event struct {
	identity   *eventIdentityToken
	typeName   string
	streamType string
	schema     Schema
	underlying any
	receivedAt time.Time
	parent     *Event
}

func newEvent(schema Schema, underlying any, receivedAt time.Time) (Event, error) {
	if !schema.valid() {
		return Event{}, fmt.Errorf("esper: cannot create an event with an empty schema")
	}
	if underlying == nil {
		return Event{}, fmt.Errorf("esper: event %q underlying value is nil", schema.name)
	}
	if schema.kind == SchemaVariant {
		routed, ok := underlying.(Event)
		if !ok || !routed.Schema().valid() {
			return Event{}, fmt.Errorf("esper: variant event %q requires a routed member Event underlying", schema.name)
		}
		if !schema.acceptsEventSchema(routed.Schema()) {
			return Event{}, fmt.Errorf("esper: event type %q is not a valid member of variant schema %q", routed.TypeName(), schema.name)
		}
		identity := routed.identity
		if identity == nil {
			identity = &eventIdentityToken{marker: 1}
		}
		return Event{identity: identity, typeName: routed.TypeName(), streamType: schema.name, schema: routed.Schema(), underlying: routed.Underlying(), receivedAt: receivedAt}, nil
	}
	if schema.kind == SchemaObjectArray {
		normalized, err := normalizeObjectArray(schema, underlying)
		if err != nil {
			return Event{}, err
		}
		underlying = normalized
	}
	if schema.kind == SchemaAvro {
		normalized, err := normalizeAvroRecord(schema, underlying)
		if err != nil {
			return Event{}, err
		}
		underlying = normalized
	}
	if schema.goType != nil {
		got := reflect.TypeOf(underlying)
		if !got.AssignableTo(schema.goType) && !(got.Kind() == reflect.Pointer && got.Elem().AssignableTo(schema.goType)) {
			return Event{}, fmt.Errorf("esper: event %q expects %s, got %s", schema.name, schema.goType, got)
		}
	}
	return Event{identity: &eventIdentityToken{marker: 1}, typeName: schema.name, schema: schema, underlying: underlying, receivedAt: receivedAt}, nil
}

// NewEvent creates a schema-bound Event envelope for Go extension functions
// that return events from a contained split or method source. The schema is
// still authoritative: variants require a routed member Event and ordinary
// schemas validate/materialize the supplied underlying value.
func NewEvent(schema Schema, underlying any, receivedAt time.Time) (Event, error) {
	return newEvent(schema, underlying, receivedAt)
}

func variantMemberNames(members []Schema) []string {
	names := make([]string, len(members))
	for index, member := range members {
		names[index] = member.Name()
	}
	return names
}

func commonVariantFields(members []Schema) ([]FieldSpec, error) {
	for index, member := range members {
		if !member.valid() {
			return nil, fmt.Errorf("esper: variant member %d is empty", index)
		}
		for previous := 0; previous < index; previous++ {
			if members[previous].Name() == member.Name() {
				return nil, fmt.Errorf("esper: variant schema duplicates member %q", member.Name())
			}
		}
	}
	if len(members) == 0 {
		return nil, nil
	}
	fields := make([]FieldSpec, 0, len(members[0].fields))
	for _, candidate := range members[0].fields {
		common := candidate
		present := true
		for _, member := range members[1:] {
			field, ok := member.Field(candidate.Name)
			if !ok {
				present = false
				break
			}
			common.Optional = common.Optional || field.Optional
			if common.Type != field.Type {
				// The declared Variant property type must be able to represent
				// values from every member. If one side is assignable to the
				// other, keep the wider target type rather than the narrower
				// member type. This matters for Java-style interface/property
				// coercion: a concrete member getter and a sibling interface
				// getter should expose their shared interface, not whichever
				// member happened to be listed first.
				if common.Type != nil && field.Type != nil && common.Type.AssignableTo(field.Type) {
					common.Type = field.Type
					continue
				}
				if common.Type != nil && field.Type != nil && field.Type.AssignableTo(common.Type) {
					continue
				}
				if numericTypes(common.Type, field.Type) {
					common.Type = typeOf[any]()
					continue
				}
				common.Type = typeOf[any]()
			}
		}
		if present {
			fields = append(fields, common)
		}
	}
	return fields, nil
}

// mergeSchemaUnderlying creates a target event value while preserving fields
// not mentioned by an on-trigger assignment. Map-backed schemas remain maps;
// struct-backed schemas are copied into a fresh addressable value so updates
// do not mutate an event already retained by a named window.
func mergeSchemaUnderlying(schema Schema, original any, updates map[string]any) (any, error) {
	if schema.kind == SchemaObjectArray {
		values := make([]any, len(schema.fields))
		if original != nil {
			for index, field := range schema.fields {
				value := schema.get(original, field.Name)
				if value.IsPresent() || value.IsNull() {
					values[index] = value.Any()
				}
			}
		}
		for name, update := range updates {
			_, canonicalName, err := schema.lookupField(name)
			if err != nil {
				return nil, err
			}
			index, exists := schema.fieldIndex[canonicalName]
			if !exists {
				return nil, fmt.Errorf("target field %q is not present on object-array schema %s", name, schema.name)
			}
			values[index] = update
		}
		return normalizeObjectArray(schema, values)
	}
	if schema.kind == SchemaAvro {
		var record *AvroRecord
		var err error
		if original == nil {
			record, err = NewAvroRecord(schema)
		} else {
			record, err = normalizeAvroRecord(schema, original)
			if err == nil {
				record = record.Clone()
			}
		}
		if err != nil {
			return nil, err
		}
		for name, value := range updates {
			if err := record.Set(name, value); err != nil {
				return nil, err
			}
		}
		return record, nil
	}
	if schema.goType == nil {
		result := make(map[string]any, len(schema.fields)+len(updates))
		if original != nil {
			for _, field := range schema.fields {
				value := schema.get(original, field.Name)
				if value.IsPresent() || value.IsNull() {
					result[field.Name] = value.Any()
				}
			}
			if values, ok := original.(map[string]any); ok {
				for name, value := range values {
					result[name] = value
				}
			}
		}
		for name, value := range updates {
			result[name] = value
		}
		return result, nil
	}
	value := reflect.New(schema.goType).Elem()
	if original != nil {
		originalValue := reflect.ValueOf(original)
		for originalValue.Kind() == reflect.Pointer {
			if originalValue.IsNil() {
				break
			}
			originalValue = originalValue.Elem()
		}
		if originalValue.IsValid() && originalValue.Type().AssignableTo(value.Type()) {
			value.Set(originalValue)
		}
	}
	for name, update := range updates {
		if setter, canonicalName, ok := schema.lookupSetter(name); ok {
			if err := invokeRegisteredSetter(value, setter, update); err != nil {
				return nil, fmt.Errorf("target property %q: %w", canonicalName, err)
			}
			continue
		}
		if err := setStructField(value, name, update, schema.resolution); err != nil {
			return nil, err
		}
	}
	return value.Interface(), nil
}

func invokeRegisteredSetter(value reflect.Value, setter schemaSetterSpec, update any) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("setter panicked: %v", recovered)
		}
	}()
	if setter.setter != nil {
		if !value.CanAddr() {
			return fmt.Errorf("underlying %s is not addressable", value.Type())
		}
		converted := update
		if setter.typ != nil && setter.typ != typeOf[any]() {
			reflected, conversionErr := assignReflectValue(setter.typ, update)
			if conversionErr != nil {
				return conversionErr
			}
			converted = reflected.Interface()
		}
		return setter.setter(value.Addr().Interface(), converted)
	}
	if setter.method == "" {
		return fmt.Errorf("setter is not configured")
	}
	target := value
	if target.Kind() != reflect.Pointer {
		if !target.CanAddr() {
			return fmt.Errorf("underlying %s is not addressable", target.Type())
		}
		target = target.Addr()
	}
	method := target.MethodByName(setter.method)
	if !method.IsValid() || method.Type().NumIn() != 1 {
		return fmt.Errorf("setter method %q is unavailable", setter.method)
	}
	converted, conversionErr := assignReflectValue(method.Type().In(0), update)
	if conversionErr != nil {
		return conversionErr
	}
	results := method.Call([]reflect.Value{converted})
	if len(results) == 1 && !results[0].IsNil() {
		return results[0].Interface().(error)
	}
	return nil
}

func setStructField(value reflect.Value, target string, update any, resolution PropertyResolutionStyle) error {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			if !value.CanSet() {
				return fmt.Errorf("cannot initialize struct field %q", target)
			}
			value.Set(reflect.New(value.Type().Elem()))
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return fmt.Errorf("target %q is not a struct field", target)
	}
	typ := value.Type()
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		fieldValue := value.Field(index)
		name, _ := parseFieldTag(field.Tag.Get("esper"))
		if name == "" {
			name, _ = parseFieldTag(field.Tag.Get("json"))
		}
		if name == "-" {
			continue
		}
		baseType := field.Type
		for baseType.Kind() == reflect.Pointer {
			baseType = baseType.Elem()
		}
		if field.Anonymous && name == "" && baseType.Kind() == reflect.Struct {
			if err := setStructField(fieldValue, target, update, resolution); err == nil {
				return nil
			}
			continue
		}
		if name == "" {
			if field.PkgPath != "" {
				continue
			}
			name = field.Name
		}
		matched := name == target
		if resolution != PropertyCaseSensitive {
			matched = strings.EqualFold(name, target)
		}
		if !matched {
			continue
		}
		if !fieldValue.CanSet() {
			return fmt.Errorf("target field %q is not settable", target)
		}
		converted, err := assignReflectValue(fieldValue.Type(), update)
		if err != nil {
			return fmt.Errorf("target field %q: %w", target, err)
		}
		fieldValue.Set(converted)
		return nil
	}
	return fmt.Errorf("target field %q is not present on struct %s", target, value.Type())
}

func assignReflectValue(target reflect.Type, update any) (reflect.Value, error) {
	if update == nil {
		switch target.Kind() {
		case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func:
			return reflect.Zero(target), nil
		default:
			return reflect.Zero(target), nil
		}
	}
	value := reflect.ValueOf(update)
	if value.Type().AssignableTo(target) {
		return value, nil
	}
	if value.Type().ConvertibleTo(target) && numericTypes(value.Type(), target) {
		return value.Convert(target), nil
	}
	if target.Kind() == reflect.Pointer && value.Type().AssignableTo(target.Elem()) {
		pointer := reflect.New(target.Elem())
		pointer.Elem().Set(value)
		return pointer, nil
	}
	return reflect.Value{}, fmt.Errorf("expects %s, got %s", target, value.Type())
}

func (e Event) TypeName() string { return e.typeName }

// StreamType is the logical source/bus type that delivered the event. Direct
// events use TypeName; insert-into routes can keep the concrete member in
// TypeName while using the target stream as StreamType.
func (e Event) StreamType() string {
	if e.streamType != "" {
		return e.streamType
	}
	return e.typeName
}
func (e Event) Schema() Schema        { return e.schema }
func (e Event) Underlying() any       { return e.underlying }
func (e Event) ReceivedAt() time.Time { return e.receivedAt }
func (e Event) Get(name string) Value { return e.schema.get(e.underlying, name) }
func (e Event) Identity() any         { return e.underlying }

// Parent returns the event that produced this event through a contained
// expansion. Ordinary events have no parent. The returned event is a value
// snapshot of the envelope; its underlying value remains owned by the
// runtime/event source.
func (e Event) Parent() (Event, bool) {
	if e.parent == nil || !e.parent.Schema().valid() {
		return Event{}, false
	}
	return *e.parent, true
}

// Ancestor returns the requested contained-event ancestor. Level 1 is the
// immediate parent, level 2 is the grandparent, and so on. This keeps nested
// contained projections explicit in the Go API instead of relying on an
// implicit EPL alias scope.
func (e Event) Ancestor(level int) (Event, bool) {
	if level <= 0 {
		return Event{}, false
	}
	current := e
	for index := 0; index < level; index++ {
		parent, ok := current.Parent()
		if !ok {
			return Event{}, false
		}
		current = parent
	}
	return current, true
}

func (e Event) PropertyNames() []string { return e.schema.PropertyNames() }

func (e Event) Property(name string) (PropertyDescriptor, bool) {
	return e.schema.Property(name)
}

func (e Event) PropertyType(name string) (reflect.Type, bool) {
	return e.schema.PropertyType(name)
}

func (e Event) HasProperty(name string) bool {
	if _, ok := e.schema.Property(name); !ok {
		return false
	}
	return true
}

// GetFragment returns a nested Event value when a property carries an Event
// envelope. Map/struct fragments remain available through Get and Property;
// they are not fabricated into an event without a registered nested schema.
func (e Event) GetFragment(name string) (Event, bool) {
	value := e.Get(name)
	if !value.IsPresent() {
		return Event{}, false
	}
	if fragment, ok := value.Any().(Event); ok {
		return fragment, true
	}
	if fragment, ok := value.Any().(*Event); ok && fragment != nil {
		return *fragment, true
	}
	segments, err := parsePropertyPath(name)
	if err == nil && len(segments) == 1 && len(segments[0].accessors) == 0 {
		if nested, ok := e.schema.lookupNestedSchema(segments[0].name); ok {
			fragment, err := newEvent(nested, value.Any(), e.receivedAt)
			if err == nil {
				return fragment, true
			}
		}
	}
	return Event{}, false
}

// Row is an ordered projection result. The schema owns the field order and
// values are immutable after construction.
type Row struct {
	schema Schema
	values []Value
}

func newRow(schema Schema, values []Value) Row {
	return Row{schema: schema, values: append([]Value(nil), values...)}
}

func (r Row) Schema() Schema { return r.schema }

func (r Row) Get(name string) Value {
	field, canonical, err := r.schema.lookupField(name)
	if err != nil {
		return Missing()
	}
	index := r.schema.fieldIndex[canonical]
	if index < 0 || index >= len(r.values) {
		_ = field
		return Missing()
	}
	return r.values[index]
}

func (r Row) Values() []Value { return append([]Value(nil), r.values...) }

func (r Row) AsMap() map[string]any {
	result := make(map[string]any, len(r.schema.fields))
	for i, field := range r.schema.fields {
		if i < len(r.values) {
			result[field.Name] = r.values[i].Any()
		}
	}
	return result
}

// JSONParseConfig controls typed JSON event decoding. Unknown fields remain
// accepted by default for compatibility with dynamic JSON schemas.
type JSONParseConfig struct {
	MaxDepth            int
	RejectUnknownFields bool
}

// JSONParseOption changes JSON decoding behavior.
type JSONParseOption func(*JSONParseConfig)

func WithJSONParseMaxDepth(depth int) JSONParseOption {
	return func(config *JSONParseConfig) { config.MaxDepth = depth }
}

func WithJSONRejectUnknownFields() JSONParseOption {
	return func(config *JSONParseConfig) { config.RejectUnknownFields = true }
}

// ParseJSON decodes a JSON object into a map-backed event. UseNumber keeps
// integer and decimal text available for schema-directed conversion instead
// of silently losing precision through float64.
func ParseJSON(schema Schema, data []byte, receivedAt time.Time) (Event, error) {
	return ParseJSONWithOptions(schema, data, receivedAt)
}

// ParseJSONWithOptions performs schema-directed conversion for nested structs,
// maps, slices, arrays, enum-like string types, time.Time, big.Int and
// big.Rat. The conversion is deliberately reflection-bound to the declared
// schema type and never invokes arbitrary methods.
func ParseJSONWithOptions(schema Schema, data []byte, receivedAt time.Time, options ...JSONParseOption) (Event, error) {
	config := JSONParseConfig{MaxDepth: 64}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	if config.MaxDepth <= 0 {
		return Event{}, fmt.Errorf("esper: JSON parse max depth must be positive")
	}
	var object map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&object); err != nil {
		return Event{}, fmt.Errorf("esper: decode JSON for %q: %w", schema.Name(), err)
	}
	if object == nil {
		return Event{}, fmt.Errorf("esper: JSON event %q must be an object", schema.Name())
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Event{}, fmt.Errorf("esper: JSON event %q contains trailing data", schema.Name())
		}
		return Event{}, fmt.Errorf("esper: decode trailing JSON for %q: %w", schema.Name(), err)
	}
	if err := validateJSONDepth(object, config.MaxDepth, 0); err != nil {
		return Event{}, fmt.Errorf("esper: JSON event %q: %w", schema.Name(), err)
	}
	if config.RejectUnknownFields && !schema.allowDynamic {
		for name := range object {
			if _, _, err := schema.lookupField(name); err != nil {
				return Event{}, fmt.Errorf("esper: JSON event %q contains undeclared field %q", schema.Name(), name)
			}
		}
	}
	updates := make(map[string]any, len(schema.fields))
	for _, field := range schema.fields {
		if key, exists := findJSONField(object, schema, field.Name); exists {
			converted, err := coerceJSONField(object[key], field.Type)
			if err != nil {
				return Event{}, fmt.Errorf("esper: JSON event %q field %q: %w", schema.Name(), field.Name, err)
			}
			converted = normalizeDynamicJSONValue(converted)
			object[key] = converted
			if schema.goType != nil {
				updates[field.Name] = converted
			}
		}
	}
	for key, value := range object {
		object[key] = normalizeDynamicJSONValue(value)
	}
	if schema.goType != nil {
		underlying, err := mergeSchemaUnderlying(schema, nil, updates)
		if err != nil {
			return Event{}, fmt.Errorf("esper: materialize typed JSON event %q: %w", schema.Name(), err)
		}
		return newEvent(schema, underlying, receivedAt)
	}
	return newEvent(schema, object, receivedAt)
}

func findJSONField(object map[string]any, schema Schema, name string) (string, bool) {
	if _, exists := object[name]; exists {
		return name, true
	}
	if schema.resolution == PropertyCaseSensitive {
		return "", false
	}
	for key := range object {
		if strings.EqualFold(key, name) {
			return key, true
		}
	}
	return "", false
}

// normalizeDynamicJSONValue maps decoder-level json.Number values to the
// ordinary Go values exposed by a dynamic JSON property. Declared numeric
// fields are converted before this function runs, while arbitrary-precision
// declared fields (big.Int/big.Rat) are already materialized and remain
// untouched. The recursive walk also normalizes numbers inside a declared
// map[string]any or []any field.
func normalizeDynamicJSONValue(value any) any {
	switch current := value.(type) {
	case json.Number:
		text := string(current)
		if strings.ContainsAny(text, ".eE") {
			if parsed, err := strconv.ParseFloat(text, 64); err == nil {
				return parsed
			}
			return current
		}
		if parsed, err := strconv.ParseInt(text, 10, 64); err == nil {
			if strconv.IntSize == 64 || (parsed >= -1<<31 && parsed <= 1<<31-1) {
				return int(parsed)
			}
			return parsed
		}
		if parsed, ok := new(big.Int).SetString(text, 10); ok {
			return *parsed
		}
		return current
	case map[string]any:
		for key, nested := range current {
			current[key] = normalizeDynamicJSONValue(nested)
		}
		return current
	case []any:
		for index, nested := range current {
			current[index] = normalizeDynamicJSONValue(nested)
		}
		return current
	default:
		return value
	}
}

func validateJSONDepth(value any, maxDepth, depth int) error {
	if depth > maxDepth {
		return fmt.Errorf("value exceeds max depth %d", maxDepth)
	}
	switch current := value.(type) {
	case map[string]any:
		for _, child := range current {
			if err := validateJSONDepth(child, maxDepth, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range current {
			if err := validateJSONDepth(child, maxDepth, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func ParseAvroJSON(schema Schema, data []byte, receivedAt time.Time) (Event, error) {
	if schema.Kind() != SchemaAvro {
		return Event{}, fmt.Errorf("esper: schema %q is not Avro", schema.Name())
	}
	event, err := ParseJSON(schema, data, receivedAt)
	if err != nil {
		return Event{}, err
	}
	return event, nil
}

// ParseObjectArray wraps and validates a positional object-array event.
func ParseObjectArray(schema Schema, values []any, receivedAt time.Time) (Event, error) {
	if schema.Kind() != SchemaObjectArray {
		return Event{}, fmt.Errorf("esper: schema %q is not an object-array schema", schema.Name())
	}
	return newEvent(schema, values, receivedAt)
}

func normalizeObjectArray(schema Schema, underlying any) ([]any, error) {
	value := reflect.ValueOf(underlying)
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, fmt.Errorf("esper: object-array event %q is a nil pointer", schema.name)
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Array && value.Kind() != reflect.Slice {
		return nil, fmt.Errorf("esper: object-array event %q expects an array or slice, got %s", schema.name, value.Kind())
	}
	if value.Len() != len(schema.fields) {
		return nil, fmt.Errorf("esper: object-array event %q expects %d values, got %d", schema.name, len(schema.fields), value.Len())
	}
	result := make([]any, len(schema.fields))
	for index, field := range schema.fields {
		item := value.Index(index)
		if item.Kind() == reflect.Interface && item.IsNil() {
			result[index] = nil
			continue
		}
		if item.Kind() == reflect.Pointer && item.IsNil() {
			result[index] = nil
			continue
		}
		if !item.CanInterface() {
			return nil, fmt.Errorf("esper: object-array event %q value %d is not accessible", schema.name, index)
		}
		candidate := item.Interface()
		if field.Type != nil && field.Type != typeOf[any]() {
			converted, err := assignReflectValue(field.Type, candidate)
			if err != nil {
				return nil, fmt.Errorf("esper: object-array event %q field %q: %w", schema.name, field.Name, err)
			}
			candidate = converted.Interface()
		}
		result[index] = candidate
	}
	return result, nil
}

// XMLParseConfig controls the secure, tree-backed XML event parser.
type XMLParseConfig struct {
	MaxDepth int
}

// XMLParseOption changes XML decoding behavior.
type XMLParseOption func(*XMLParseConfig)

func WithXMLParseMaxDepth(depth int) XMLParseOption {
	return func(config *XMLParseConfig) { config.MaxDepth = depth }
}

// ParseXML supports scalar paths, attributes (using an @name segment),
// repeated child elements and indexed nested paths. Fields may be named by
// their leaf element or by a path below the document root (for example
// "trade.symbol"). Strict XML decoding rejects malformed input and does not
// resolve external entities.
func ParseXML(schema Schema, data []byte, receivedAt time.Time) (Event, error) {
	return ParseXMLWithOptions(schema, data, receivedAt)
}

func ParseXMLWithOptions(schema Schema, data []byte, receivedAt time.Time, options ...XMLParseOption) (Event, error) {
	if schema.Kind() != SchemaXML {
		return Event{}, fmt.Errorf("esper: schema %q is not XML", schema.Name())
	}
	config := XMLParseConfig{MaxDepth: 64}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	if config.MaxDepth <= 0 {
		return Event{}, fmt.Errorf("esper: XML parse max depth must be positive")
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true
	frames := make([]xmlParseFrame, 0, 8)
	var root any
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Event{}, fmt.Errorf("esper: decode XML for %q: %w", schema.Name(), err)
		}
		switch current := token.(type) {
		case xml.StartElement:
			if len(frames) >= config.MaxDepth {
				return Event{}, fmt.Errorf("esper: XML event %q exceeds max depth %d", schema.Name(), config.MaxDepth)
			}
			frame := xmlParseFrame{name: current.Name.Local, attributes: make(map[string]any), children: make(map[string][]any)}
			for _, attribute := range current.Attr {
				frame.attributes["@"+attribute.Name.Local] = attribute.Value
			}
			frames = append(frames, frame)
		case xml.CharData:
			if len(frames) > 0 {
				frames[len(frames)-1].text.Write([]byte(current))
			}
		case xml.EndElement:
			if len(frames) == 0 {
				return Event{}, fmt.Errorf("esper: malformed XML nesting for %q", schema.Name())
			}
			frame := frames[len(frames)-1]
			if frame.name != current.Name.Local {
				return Event{}, fmt.Errorf("esper: XML end element %q does not match %q", current.Name.Local, frame.name)
			}
			value := frame.value()
			frames = frames[:len(frames)-1]
			if len(frames) == 0 {
				root = map[string]any{frame.name: value}
			} else {
				frames[len(frames)-1].children[frame.name] = append(frames[len(frames)-1].children[frame.name], value)
			}
		}
	}
	if root == nil {
		return Event{}, fmt.Errorf("esper: XML event %q has no root element", schema.Name())
	}
	values, ok := root.(map[string]any)
	if !ok {
		return Event{}, fmt.Errorf("esper: XML event %q root is not an object", schema.Name())
	}
	for _, field := range schema.fields {
		value, exists := lookupXMLField(values, field.Name)
		if !exists {
			continue
		}
		values[field.Name] = coerceXMLValue(value, field.Type)
	}
	return newEvent(schema, values, receivedAt)
}

type xmlParseFrame struct {
	name       string
	attributes map[string]any
	children   map[string][]any
	text       strings.Builder
}

func (frame *xmlParseFrame) value() any {
	text := strings.TrimSpace(frame.text.String())
	if len(frame.attributes) == 0 && len(frame.children) == 0 {
		return text
	}
	result := make(map[string]any, len(frame.attributes)+len(frame.children)+1)
	for name, value := range frame.attributes {
		result[name] = value
	}
	for name, values := range frame.children {
		if len(values) == 1 {
			result[name] = values[0]
		} else {
			result[name] = values
		}
	}
	if text != "" {
		result["#text"] = text
	}
	return result
}

func lookupXMLField(values map[string]any, name string) (any, bool) {
	parsed := (Schema{resolution: PropertyCaseSensitive}).get(values, name)
	if parsed.IsPresent() {
		return parsed.Any(), true
	}
	if parsed.IsNull() {
		return nil, true
	}
	if !strings.ContainsAny(name, ".[('") {
		return findXMLLeaf(values, name)
	}
	return nil, false
}

func findXMLLeaf(value any, name string) (any, bool) {
	if values, ok := value.(map[string]any); ok {
		if child, exists := values[name]; exists {
			return child, true
		}
		for _, child := range values {
			if found, exists := findXMLLeaf(child, name); exists {
				return found, true
			}
		}
		return nil, false
	}
	if values, ok := value.([]any); ok {
		for index := len(values) - 1; index >= 0; index-- {
			if found, exists := findXMLLeaf(values[index], name); exists {
				return found, true
			}
		}
	}
	return nil, false
}

func coerceXMLValue(value any, target reflect.Type) any {
	if value == nil {
		return nil
	}
	if items, ok := value.([]any); ok && target != nil && (target.Kind() == reflect.Slice || target.Kind() == reflect.Array) {
		if target.Kind() == reflect.Array && len(items) != target.Len() {
			return value
		}
		var result reflect.Value
		if target.Kind() == reflect.Array {
			result = reflect.New(target).Elem()
		} else {
			result = reflect.MakeSlice(target, len(items), len(items))
		}
		for index, item := range items {
			converted := coerceXMLValue(item, target.Elem())
			assigned, err := assignReflectValue(target.Elem(), converted)
			if err != nil {
				return value
			}
			result.Index(index).Set(assigned)
		}
		return result.Interface()
	}
	if text, ok := value.(string); ok {
		return coerceTextValue(text, target)
	}
	if target != nil {
		return coerceJSONValue(value, target)
	}
	return value
}

func coerceTextValue(text string, target reflect.Type) any {
	if target == nil || target == typeOf[any]() || target.Kind() == reflect.String {
		return text
	}
	if target == reflect.TypeOf(time.Time{}) {
		if parsed, err := time.Parse(time.RFC3339Nano, text); err == nil {
			return parsed
		}
		return text
	}
	switch target.Kind() {
	case reflect.Bool:
		if parsed, err := strconv.ParseBool(text); err == nil {
			return parsed
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if parsed, err := strconv.ParseInt(text, 10, target.Bits()); err == nil {
			value := reflect.New(target).Elem()
			value.SetInt(parsed)
			return value.Interface()
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if parsed, err := strconv.ParseUint(text, 10, target.Bits()); err == nil {
			value := reflect.New(target).Elem()
			value.SetUint(parsed)
			return value.Interface()
		}
	case reflect.Float32, reflect.Float64:
		if parsed, err := strconv.ParseFloat(text, target.Bits()); err == nil {
			value := reflect.New(target).Elem()
			value.SetFloat(parsed)
			return value.Interface()
		}
	}
	return text
}

func coerceJSONValue(value any, target reflect.Type) any {
	converted, ok := coerceJSONReflect(value, target)
	if ok && converted.IsValid() && converted.CanInterface() {
		return converted.Interface()
	}
	return value
}

func coerceJSONField(value any, target reflect.Type) (any, error) {
	if value == nil || target == nil || target.Kind() == reflect.Interface {
		return value, nil
	}
	converted, ok := coerceJSONReflect(value, target)
	if ok && converted.IsValid() && converted.CanInterface() {
		return converted.Interface(), nil
	}
	if jsonInputShapeMatchesTarget(value, target) {
		return nil, fmt.Errorf("cannot convert JSON value %q to %s", jsonValueText(value), target)
	}
	// A scalar declaration receiving an object/array is the Java JSON
	// parser's lax null case. Keep the property declared but clear its value.
	return nil, nil
}

func jsonInputShapeMatchesTarget(value any, target reflect.Type) bool {
	if target == nil || value == nil {
		return false
	}
	for target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	if target == reflect.TypeOf(time.Time{}) || target == reflect.TypeOf(big.Int{}) || target == reflect.TypeOf(big.Rat{}) {
		return false
	}
	source := reflect.ValueOf(value)
	if !source.IsValid() {
		return false
	}
	switch target.Kind() {
	case reflect.Slice, reflect.Array:
		return source.Kind() == reflect.Slice || source.Kind() == reflect.Array
	case reflect.Map, reflect.Struct:
		return source.Kind() == reflect.Map
	default:
		return source.Kind() != reflect.Map && source.Kind() != reflect.Slice && source.Kind() != reflect.Array
	}
}

func jsonValueText(value any) string {
	if text, ok := jsonText(value); ok {
		return text
	}
	return fmt.Sprintf("%T", value)
}

func coerceJSONReflect(value any, target reflect.Type) (reflect.Value, bool) {
	if value == nil {
		return reflect.Zero(target), true
	}
	if target == nil {
		return reflect.ValueOf(value), true
	}
	source := reflect.ValueOf(value)
	if source.IsValid() && source.Type().AssignableTo(target) {
		return source, true
	}
	if target.Kind() == reflect.Interface {
		return source, source.IsValid() && source.Type().AssignableTo(target)
	}
	if target.Kind() == reflect.Pointer {
		converted, ok := coerceJSONReflect(value, target.Elem())
		if !ok {
			return reflect.Value{}, false
		}
		pointer := reflect.New(target.Elem())
		pointer.Elem().Set(converted)
		return pointer, true
	}
	if target == reflect.TypeOf(time.Time{}) {
		text, ok := jsonText(value)
		if !ok {
			return reflect.Value{}, false
		}
		parsed, err := time.Parse(time.RFC3339Nano, text)
		if err != nil {
			return reflect.Value{}, false
		}
		return reflect.ValueOf(parsed), true
	}
	if target == reflect.TypeOf(big.Int{}) {
		text, ok := jsonText(value)
		if !ok {
			return reflect.Value{}, false
		}
		parsed := new(big.Int)
		if _, ok := parsed.SetString(text, 10); !ok {
			return reflect.Value{}, false
		}
		return reflect.ValueOf(*parsed), true
	}
	if target == reflect.TypeOf(big.Rat{}) {
		text, ok := jsonText(value)
		if !ok {
			return reflect.Value{}, false
		}
		parsed := new(big.Rat)
		if _, ok := parsed.SetString(text); !ok {
			return reflect.Value{}, false
		}
		return reflect.ValueOf(*parsed), true
	}
	switch target.Kind() {
	case reflect.String:
		text, ok := jsonText(value)
		if !ok {
			return reflect.Value{}, false
		}
		converted := reflect.New(target).Elem()
		converted.SetString(text)
		return converted, true
	case reflect.Bool:
		if boolean, ok := value.(bool); ok {
			converted := reflect.New(target).Elem()
			converted.SetBool(boolean)
			return converted, true
		}
		if text, ok := jsonText(value); ok {
			if boolean, err := strconv.ParseBool(text); err == nil {
				converted := reflect.New(target).Elem()
				converted.SetBool(boolean)
				return converted, true
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if text, ok := jsonText(value); ok {
			if parsed, err := strconv.ParseInt(text, 10, target.Bits()); err == nil {
				converted := reflect.New(target).Elem()
				converted.SetInt(parsed)
				return converted, true
			}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if text, ok := jsonText(value); ok {
			if parsed, err := strconv.ParseUint(text, 10, target.Bits()); err == nil {
				converted := reflect.New(target).Elem()
				converted.SetUint(parsed)
				return converted, true
			}
		}
	case reflect.Float32, reflect.Float64:
		if text, ok := jsonText(value); ok {
			if parsed, err := strconv.ParseFloat(text, target.Bits()); err == nil {
				converted := reflect.New(target).Elem()
				converted.SetFloat(parsed)
				return converted, true
			}
		}
	case reflect.Slice, reflect.Array:
		items := reflect.ValueOf(value)
		if !items.IsValid() || (items.Kind() != reflect.Slice && items.Kind() != reflect.Array) {
			return reflect.Value{}, false
		}
		if target.Kind() == reflect.Array {
			if items.Len() != target.Len() {
				return reflect.Value{}, false
			}
		} else {
			result := reflect.MakeSlice(target, items.Len(), items.Len())
			for index := 0; index < items.Len(); index++ {
				converted, ok := coerceJSONReflect(items.Index(index).Interface(), target.Elem())
				if !ok {
					return reflect.Value{}, false
				}
				result.Index(index).Set(converted)
			}
			return result, true
		}
		result := reflect.New(target).Elem()
		for index := 0; index < items.Len(); index++ {
			converted, ok := coerceJSONReflect(items.Index(index).Interface(), target.Elem())
			if !ok {
				return reflect.Value{}, false
			}
			result.Index(index).Set(converted)
		}
		return result, true
	case reflect.Map:
		items := reflect.ValueOf(value)
		if !items.IsValid() || items.Kind() != reflect.Map {
			return reflect.Value{}, false
		}
		result := reflect.MakeMapWithSize(target, items.Len())
		iterator := items.MapRange()
		for iterator.Next() {
			key, ok := coerceJSONReflect(iterator.Key().Interface(), target.Key())
			if !ok {
				return reflect.Value{}, false
			}
			item, ok := coerceJSONReflect(iterator.Value().Interface(), target.Elem())
			if !ok {
				return reflect.Value{}, false
			}
			result.SetMapIndex(key, item)
		}
		return result, true
	case reflect.Struct:
		items, ok := value.(map[string]any)
		if !ok {
			return reflect.Value{}, false
		}
		result := reflect.New(target).Elem()
		for name, raw := range items {
			field := structFieldValue(result, name, PropertyCaseInsensitive, nil)
			if !field.IsValid() || !field.CanSet() {
				continue
			}
			converted, ok := coerceJSONReflect(raw, field.Type())
			if ok {
				field.Set(converted)
			}
		}
		return result, true
	}
	return reflect.Value{}, false
}

func jsonText(value any) (string, bool) {
	switch typed := value.(type) {
	case json.Number:
		return typed.String(), true
	case string:
		return typed, true
	case bool:
		return strconv.FormatBool(typed), true
	case int:
		return strconv.Itoa(typed), true
	case int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(typed), true
	default:
		return "", false
	}
}
