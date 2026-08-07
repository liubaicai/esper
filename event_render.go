package esper

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// JSONRenderConfig controls the deterministic JSON event renderer.
type JSONRenderConfig struct {
	Title    string
	Indent   string
	MaxDepth int
}

// JSONRenderOption changes JSON rendering behavior.
type JSONRenderOption func(*JSONRenderConfig)

type orderedJSONField struct {
	name  string
	value any
}

// orderedJSONObject is used for schema-backed JSON objects. encoding/json
// sorts ordinary map keys, while Esper's JSON renderer writes declared fields
// in schema/class order. A small Marshaler keeps that observable contract and
// still delegates scalar escaping and number validation to encoding/json.
type orderedJSONObject struct {
	fields []orderedJSONField
}

func (object orderedJSONObject) MarshalJSON() ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteByte('{')
	for index, field := range object.fields {
		if index > 0 {
			buffer.WriteByte(',')
		}
		name, err := json.Marshal(field.name)
		if err != nil {
			return nil, err
		}
		buffer.Write(name)
		buffer.WriteByte(':')
		value, err := json.Marshal(field.value)
		if err != nil {
			return nil, err
		}
		buffer.Write(value)
	}
	buffer.WriteByte('}')
	return buffer.Bytes(), nil
}

// WithJSONTitle wraps the rendered event in an object with the supplied key.
func WithJSONTitle(title string) JSONRenderOption {
	return func(config *JSONRenderConfig) { config.Title = strings.TrimSpace(title) }
}

// WithJSONIndent enables pretty JSON output. An empty string keeps compact
// output; a non-empty value is used as the indentation unit.
func WithJSONIndent(indent string) JSONRenderOption {
	return func(config *JSONRenderConfig) { config.Indent = indent }
}

// WithJSONMaxDepth bounds recursive values and rejects cyclic/deep payloads.
func WithJSONMaxDepth(depth int) JSONRenderOption {
	return func(config *JSONRenderConfig) { config.MaxDepth = depth }
}

// RenderJSON renders an Event using its schema field order for event-backed
// values and stable key ordering for dynamic maps. Missing properties are
// omitted while explicit Null properties are emitted as JSON null.
func RenderJSON(event Event, options ...JSONRenderOption) (string, error) {
	config := JSONRenderConfig{MaxDepth: 64}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	if config.MaxDepth <= 0 {
		return "", fmt.Errorf("esper: JSON render max depth must be positive")
	}
	value, err := renderJSONEvent(event, config.MaxDepth)
	if err != nil {
		return "", err
	}
	if config.Title != "" {
		value = orderedJSONObject{fields: []orderedJSONField{{name: config.Title, value: value}}}
	}
	var encoded []byte
	if config.Indent == "" {
		encoded, err = json.Marshal(value)
	} else {
		encoded, err = json.MarshalIndent(value, "", config.Indent)
	}
	if err != nil {
		return "", fmt.Errorf("esper: render JSON event %q: %w", event.TypeName(), err)
	}
	return string(encoded), nil
}

func renderJSONEvent(event Event, maxDepth int) (any, error) {
	if !event.Schema().valid() {
		return nil, fmt.Errorf("esper: cannot render an event with an empty schema")
	}
	return renderJSONSchemaValueWithRaw(event.Schema(), event.Underlying(), maxDepth, 0, event.jsonRaw)
}

func renderJSONSchemaValue(schema Schema, underlying any, maxDepth, depth int) (any, error) {
	return renderJSONSchemaValueWithRaw(schema, underlying, maxDepth, depth, nil)
}

func renderJSONSchemaValueWithRaw(schema Schema, underlying any, maxDepth, depth int, raw any) (any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("esper: JSON event value exceeds max depth %d", maxDepth)
	}
	if underlying == nil {
		return nil, nil
	}
	if nested, ok := underlying.(Event); ok {
		return renderJSONEventAtDepthWithRaw(nested, maxDepth, depth+1, raw)
	}
	if row, ok := underlying.(Row); ok {
		return renderJSONRowWithRaw(row, maxDepth, depth+1, raw)
	}
	if schema.valid() && len(schema.fields) > 0 {
		return renderJSONSchemaObjectWithRaw(schema, underlying, maxDepth, depth, raw)
	}
	return renderJSONValueWithRaw(underlying, maxDepth, depth, raw, nil)
}

func renderJSONSchemaObject(schema Schema, underlying any, maxDepth, depth int) (orderedJSONObject, error) {
	return renderJSONSchemaObjectWithRaw(schema, underlying, maxDepth, depth, nil)
}

func renderJSONSchemaObjectWithRaw(schema Schema, underlying any, maxDepth, depth int, raw any) (orderedJSONObject, error) {
	result := orderedJSONObject{fields: make([]orderedJSONField, 0, len(schema.fields))}
	known := make(map[string]struct{}, len(schema.fields))
	for _, field := range schema.fields {
		known[field.Name] = struct{}{}
		value := schema.get(underlying, field.Name)
		if value.IsMissing() {
			continue
		}
		rawValue, _ := rawJSONSchemaField(raw, schema, field.Name)
		normalized, err := renderJSONSchemaPropertyWithRaw(schema, field.Name, value, rawValue, field.Type, maxDepth, depth+1)
		if err != nil {
			return orderedJSONObject{}, fmt.Errorf("property %q: %w", field.Name, err)
		}
		result.fields = append(result.fields, orderedJSONField{name: field.Name, value: normalized})
	}
	dynamic := dynamicMapFields(underlying)
	names := make([]string, 0, len(dynamic))
	for name := range dynamic {
		if _, exists := known[name]; exists {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := dynamic[name]
		rawValue, _ := rawJSONNamedField(raw, name)
		normalized, err := renderJSONValueWithRaw(value, maxDepth, depth+1, rawValue, nil)
		if err != nil {
			return orderedJSONObject{}, fmt.Errorf("property %q: %w", name, err)
		}
		result.fields = append(result.fields, orderedJSONField{name: name, value: normalized})
	}
	return result, nil
}

func renderJSONSchemaProperty(schema Schema, name string, value Value, maxDepth, depth int) (any, error) {
	return renderJSONSchemaPropertyWithRaw(schema, name, value, nil, nil, maxDepth, depth)
}

func renderJSONSchemaPropertyWithRaw(schema Schema, name string, value Value, raw any, declaredType reflect.Type, maxDepth, depth int) (any, error) {
	nested, ok := schema.lookupNestedSchema(name)
	if !ok || value.IsMissing() || value.IsNull() {
		return renderJSONValueStateWithRaw(value, maxDepth, depth, raw, declaredType)
	}
	return renderJSONNestedSchemaValueWithRaw(nested, value.Any(), maxDepth, depth, raw, declaredType)
}

func renderJSONNestedSchemaValue(schema Schema, underlying any, maxDepth, depth int) (any, error) {
	return renderJSONNestedSchemaValueWithRaw(schema, underlying, maxDepth, depth, nil, nil)
}

func renderJSONNestedSchemaValueWithRaw(schema Schema, underlying any, maxDepth, depth int, raw any, declaredType reflect.Type) (any, error) {
	if underlying == nil {
		return nil, nil
	}
	value := reflect.ValueOf(underlying)
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return nil, nil
		}
		value = value.Elem()
	}
	if value.IsValid() && (value.Kind() == reflect.Array || value.Kind() == reflect.Slice) && value.Type() != reflect.TypeOf([]byte{}) {
		result := make([]any, value.Len())
		elementType := reflect.Type(nil)
		if declaredType != nil {
			elementType = reflectElementType(declaredType)
		}
		if elementType == nil {
			elementType = value.Type().Elem()
		}
		for index := 0; index < value.Len(); index++ {
			rawItem, _ := rawJSONIndex(raw, index)
			item, err := renderJSONNestedSchemaValueWithRaw(schema, value.Index(index).Interface(), maxDepth, depth+1, rawItem, elementType)
			if err != nil {
				return nil, err
			}
			result[index] = item
		}
		return result, nil
	}
	return renderJSONSchemaValueWithRaw(schema, underlying, maxDepth, depth, raw)
}

func renderJSONEventAtDepth(event Event, maxDepth, depth int) (any, error) {
	return renderJSONEventAtDepthWithRaw(event, maxDepth, depth, nil)
}

func renderJSONEventAtDepthWithRaw(event Event, maxDepth, depth int, raw any) (any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("esper: JSON event value exceeds max depth %d", maxDepth)
	}
	if raw == nil {
		raw = event.jsonRaw
	}
	return renderJSONSchemaValueWithRaw(event.Schema(), event.Underlying(), maxDepth, depth, raw)
}

func renderJSONRow(row Row, maxDepth, depth int) (any, error) {
	return renderJSONRowWithRaw(row, maxDepth, depth, nil)
}

func renderJSONRowWithRaw(row Row, maxDepth, depth int, raw any) (any, error) {
	fields := row.schema.fields
	return renderJSONObjectWithRaw(fields, row.Get, nil, maxDepth, depth, raw)
}

func renderJSONObject(fields []FieldSpec, get func(string) Value, underlying any, maxDepth, depth int) (orderedJSONObject, error) {
	return renderJSONObjectWithRaw(fields, get, underlying, maxDepth, depth, nil)
}

func renderJSONObjectWithRaw(fields []FieldSpec, get func(string) Value, underlying any, maxDepth, depth int, raw any) (orderedJSONObject, error) {
	result := orderedJSONObject{fields: make([]orderedJSONField, 0, len(fields))}
	known := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		known[field.Name] = struct{}{}
		value := get(field.Name)
		if value.IsMissing() {
			continue
		}
		rawValue, _ := rawJSONNamedField(raw, field.Name)
		normalized, err := renderJSONValueStateWithRaw(value, maxDepth, depth+1, rawValue, field.Type)
		if err != nil {
			return orderedJSONObject{}, fmt.Errorf("property %q: %w", field.Name, err)
		}
		result.fields = append(result.fields, orderedJSONField{name: field.Name, value: normalized})
	}
	dynamic := dynamicMapFields(underlying)
	names := make([]string, 0, len(dynamic))
	for name := range dynamic {
		if _, exists := known[name]; exists {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := dynamic[name]
		rawValue, _ := rawJSONNamedField(raw, name)
		normalized, err := renderJSONValueWithRaw(value, maxDepth, depth+1, rawValue, nil)
		if err != nil {
			return orderedJSONObject{}, fmt.Errorf("property %q: %w", name, err)
		}
		result.fields = append(result.fields, orderedJSONField{name: name, value: normalized})
	}
	return result, nil
}

func renderJSONValueState(value Value, maxDepth, depth int) (any, error) {
	return renderJSONValueStateWithRaw(value, maxDepth, depth, nil, nil)
}

func renderJSONValueStateWithRaw(value Value, maxDepth, depth int, raw any, declaredType reflect.Type) (any, error) {
	if value.IsMissing() || value.IsNull() {
		return nil, nil
	}
	return renderJSONValueWithRaw(value.Any(), maxDepth, depth, raw, declaredType)
}

func renderJSONValue(value any, maxDepth, depth int) (any, error) {
	return renderJSONValueWithRaw(value, maxDepth, depth, nil, nil)
}

func renderJSONValueWithRaw(value any, maxDepth, depth int, raw any, declaredType reflect.Type) (any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("value exceeds max depth %d", maxDepth)
	}
	if value == nil {
		return nil, nil
	}
	if wrapped, ok := value.(Value); ok {
		return renderJSONValueStateWithRaw(wrapped, maxDepth, depth+1, raw, declaredType)
	}
	if event, ok := value.(Event); ok {
		return renderJSONEventAtDepthWithRaw(event, maxDepth, depth+1, raw)
	}
	if row, ok := value.(Row); ok {
		return renderJSONRowWithRaw(row, maxDepth, depth+1, raw)
	}
	reflectValue := reflect.ValueOf(value)
	for reflectValue.Kind() == reflect.Pointer || reflectValue.Kind() == reflect.Interface {
		if reflectValue.IsNil() {
			return nil, nil
		}
		reflectValue = reflectValue.Elem()
	}
	if !reflectValue.IsValid() {
		return nil, nil
	}
	if number, ok := raw.(json.Number); ok && jsonRawNumberCompatible(number, declaredType, reflectValue) {
		return number, nil
	}
	if character, ok := rawJSONCharacter(raw, declaredType, reflectValue); ok {
		return character, nil
	}
	if reflectValue.Type() == reflect.TypeOf(time.Time{}) {
		if text, ok := raw.(string); ok {
			return text, nil
		}
		return reflectValue.Interface().(time.Time).Format(time.RFC3339Nano), nil
	}
	if reflectValue.Type() == reflect.TypeOf(DateOnly("")) {
		return string(reflectValue.Interface().(DateOnly)), nil
	}
	if reflectValue.Type() == reflect.TypeOf(UUID{}) {
		return reflectValue.Interface().(UUID).String(), nil
	}
	if reflectValue.Type() == reflect.TypeOf(URL("")) {
		return string(reflectValue.Interface().(URL)), nil
	}
	if reflectValue.Type() == reflect.TypeOf(URI("")) {
		return string(reflectValue.Interface().(URI)), nil
	}
	if reflectValue.Type() == reflect.TypeOf(big.Int{}) {
		integer := reflectValue.Interface().(big.Int)
		return json.Number(integer.String()), nil
	}
	if reflectValue.Type() == reflect.TypeOf(big.Rat{}) {
		return json.Number(bigRatJSONText(reflectValue.Interface().(big.Rat))), nil
	}
	switch reflectValue.Kind() {
	case reflect.Map:
		if reflectValue.IsNil() {
			return nil, nil
		}
		result := orderedJSONObject{fields: make([]orderedJSONField, 0, reflectValue.Len())}
		elementType := reflectElementType(declaredType)
		if elementType == nil {
			elementType = reflectValue.Type().Elem()
		}
		iterator := reflectValue.MapRange()
		fields := make([]struct {
			name  string
			value reflect.Value
		}, 0, reflectValue.Len())
		for iterator.Next() {
			fields = append(fields, struct {
				name  string
				value reflect.Value
			}{name: fmt.Sprint(iterator.Key().Interface()), value: iterator.Value()})
		}
		sort.Slice(fields, func(left, right int) bool { return fields[left].name < fields[right].name })
		for _, field := range fields {
			rawValue, _ := rawJSONNamedField(raw, field.name)
			normalized, err := renderJSONValueWithRaw(field.value.Interface(), maxDepth, depth+1, rawValue, elementType)
			if err != nil {
				return nil, err
			}
			result.fields = append(result.fields, orderedJSONField{name: field.name, value: normalized})
		}
		return result, nil
	case reflect.Struct:
		fields := genericStructFields(reflectValue)
		return renderJSONObjectWithRaw(fields, func(name string) Value {
			return genericStructProperty(reflectValue, name)
		}, value, maxDepth, depth, raw)
	case reflect.Array, reflect.Slice:
		if reflectValue.Kind() == reflect.Slice && reflectValue.IsNil() {
			return nil, nil
		}
		result := make([]any, reflectValue.Len())
		elementType := reflectElementType(declaredType)
		if elementType == nil {
			elementType = reflectValue.Type().Elem()
		}
		for index := 0; index < reflectValue.Len(); index++ {
			rawValue, _ := rawJSONIndex(raw, index)
			normalized, err := renderJSONValueWithRaw(reflectValue.Index(index).Interface(), maxDepth, depth+1, rawValue, elementType)
			if err != nil {
				return nil, err
			}
			result[index] = normalized
		}
		return result, nil
	case reflect.Float32, reflect.Float64:
		text := strconv.FormatFloat(reflectValue.Float(), 'g', -1, reflectValue.Type().Bits())
		if !strings.ContainsAny(text, ".eE") {
			text += ".0"
		}
		return json.Number(text), nil
	default:
		return value, nil
	}
}

func reflectElementType(typ reflect.Type) reflect.Type {
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == nil || (typ.Kind() != reflect.Array && typ.Kind() != reflect.Slice && typ.Kind() != reflect.Map) {
		return nil
	}
	return typ.Elem()
}

func jsonRawNumberCompatible(number json.Number, declaredType reflect.Type, value reflect.Value) bool {
	if declaredType != nil {
		for declaredType.Kind() == reflect.Pointer {
			declaredType = declaredType.Elem()
		}
		if declaredType.Kind() == reflect.Interface || declaredType.Kind() == reflect.String || declaredType.Kind() == reflect.Bool {
			return declaredType == reflect.TypeOf(json.Number(""))
		}
		if declaredType == reflect.TypeOf(time.Time{}) || declaredType == reflect.TypeOf(DateOnly("")) || declaredType == reflect.TypeOf(UUID{}) || declaredType == reflect.TypeOf(URL("")) || declaredType == reflect.TypeOf(URI("")) {
			return false
		}
		if declaredType == reflect.TypeOf(big.Int{}) || declaredType == reflect.TypeOf(big.Rat{}) {
			return true
		}
		switch declaredType.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
			return true
		default:
			return false
		}
	}
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return false
	}
	return value.Type() == reflect.TypeOf(json.Number("")) || isNumericKind(value.Kind())
}

func rawJSONCharacter(raw any, declaredType reflect.Type, value reflect.Value) (string, bool) {
	text, ok := raw.(string)
	if !ok || declaredType == nil {
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
	if !value.IsValid() || value.Kind() != reflect.Int32 || len([]rune(text)) == 0 {
		return "", false
	}
	return string([]rune(text)[0]), true
}

func rawJSONSchemaField(raw any, schema Schema, name string) (any, bool) {
	object, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	if key, exists := findJSONField(object, schema, name); exists {
		return object[key], true
	}
	return nil, false
}

func rawJSONNamedField(raw any, name string) (any, bool) {
	object, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	if value, exists := object[name]; exists {
		return value, true
	}
	for key, value := range object {
		if strings.EqualFold(key, name) {
			return value, true
		}
	}
	return nil, false
}

func rawJSONIndex(raw any, index int) (any, bool) {
	items, ok := raw.([]any)
	if !ok || index < 0 || index >= len(items) {
		return nil, false
	}
	return items[index], true
}

// XMLRenderConfig controls XML output. XML uses repeated element names for
// array values, matching Esper's event renderer contract.
type XMLRenderConfig struct {
	Title              string
	Indent             string
	DefaultAsAttribute bool
	MaxDepth           int
}

// XMLRenderOption changes XML rendering behavior.
type XMLRenderOption func(*XMLRenderConfig)

func WithXMLTitle(title string) XMLRenderOption {
	return func(config *XMLRenderConfig) { config.Title = strings.TrimSpace(title) }
}

func WithXMLIndent(indent string) XMLRenderOption {
	return func(config *XMLRenderConfig) { config.Indent = indent }
}

func WithXMLDefaultAsAttribute(enabled bool) XMLRenderOption {
	return func(config *XMLRenderConfig) { config.DefaultAsAttribute = enabled }
}

func WithXMLMaxDepth(depth int) XMLRenderOption {
	return func(config *XMLRenderConfig) { config.MaxDepth = depth }
}

// RenderXML renders an Event as XML with escaped text and attributes. External
// entities are never resolved because rendering only consumes in-memory Go
// values; parsing remains the responsibility of ParseXML.
func RenderXML(event Event, options ...XMLRenderOption) (string, error) {
	config := XMLRenderConfig{Indent: "  ", MaxDepth: 64}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	if config.MaxDepth <= 0 {
		return "", fmt.Errorf("esper: XML render max depth must be positive")
	}
	rootName := event.TypeName()
	if config.Title != "" {
		rootName = config.Title
	}
	root, err := xmlRenderNodeForEvent(event, rootName, config, 0)
	if err != nil {
		return "", err
	}
	var buffer bytes.Buffer
	buffer.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>")
	encoder := xml.NewEncoder(&buffer)
	if config.Indent != "" {
		encoder.Indent("", config.Indent)
	}
	if err := writeXMLRenderNode(encoder, root); err != nil {
		return "", fmt.Errorf("esper: render XML event %q: %w", event.TypeName(), err)
	}
	if err := encoder.Flush(); err != nil {
		return "", fmt.Errorf("esper: flush XML event %q: %w", event.TypeName(), err)
	}
	return buffer.String(), nil
}

type xmlRenderNode struct {
	name     string
	attrs    []xml.Attr
	text     *string
	children []xmlRenderNode
}

func xmlRenderNodeForEvent(event Event, name string, config XMLRenderConfig, depth int) (xmlRenderNode, error) {
	if !event.Schema().valid() {
		return xmlRenderNode{}, fmt.Errorf("esper: cannot render an event with an empty schema")
	}
	return xmlRenderNodeForSchema(event.Schema(), name, event.Underlying(), config, depth)
}

func xmlRenderNodeForSchema(schema Schema, name string, underlying any, config XMLRenderConfig, depth int) (xmlRenderNode, error) {
	if depth > config.MaxDepth {
		return xmlRenderNode{}, fmt.Errorf("esper: XML event value exceeds max depth %d", config.MaxDepth)
	}
	node := xmlRenderNode{name: xmlName(name)}
	known := make(map[string]struct{}, len(schema.fields))
	for _, field := range schema.fields {
		known[field.Name] = struct{}{}
		value := schema.get(underlying, field.Name)
		if value.IsMissing() {
			continue
		}
		if config.DefaultAsAttribute && xmlIsScalarState(value) {
			node.attrs = append(node.attrs, xml.Attr{Name: xml.Name{Local: xmlName(field.Name)}, Value: xmlScalarState(value)})
			continue
		}
		children, err := xmlFieldNodesForSchema(field.Name, value, schema, config, depth+1)
		if err != nil {
			return xmlRenderNode{}, err
		}
		node.children = append(node.children, children...)
	}
	for field, value := range dynamicMapFields(underlying) {
		if _, exists := known[field]; exists {
			continue
		}
		fieldValue := Present(value)
		if wrapped, ok := value.(Value); ok {
			fieldValue = wrapped
		}
		children, err := xmlFieldNodes(field, fieldValue, config, depth+1)
		if err != nil {
			return xmlRenderNode{}, err
		}
		node.children = append(node.children, children...)
	}
	return node, nil
}

func xmlRenderNodeForObject(name string, fields []FieldSpec, get func(string) Value, underlying any, config XMLRenderConfig, depth int) (xmlRenderNode, error) {
	if depth > config.MaxDepth {
		return xmlRenderNode{}, fmt.Errorf("esper: XML event value exceeds max depth %d", config.MaxDepth)
	}
	node := xmlRenderNode{name: xmlName(name)}
	known := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		known[field.Name] = struct{}{}
		value := get(field.Name)
		if value.IsMissing() {
			continue
		}
		if config.DefaultAsAttribute && xmlIsScalarState(value) {
			node.attrs = append(node.attrs, xml.Attr{Name: xml.Name{Local: xmlName(field.Name)}, Value: xmlScalarState(value)})
			continue
		}
		children, err := xmlFieldNodes(field.Name, value, config, depth+1)
		if err != nil {
			return xmlRenderNode{}, err
		}
		node.children = append(node.children, children...)
	}
	for field, value := range dynamicMapFields(underlying) {
		if _, exists := known[field]; exists {
			continue
		}
		fieldValue := Present(value)
		if wrapped, ok := value.(Value); ok {
			fieldValue = wrapped
		}
		children, err := xmlFieldNodes(field, fieldValue, config, depth+1)
		if err != nil {
			return xmlRenderNode{}, err
		}
		node.children = append(node.children, children...)
	}
	return node, nil
}

func xmlFieldNodes(name string, value Value, config XMLRenderConfig, depth int) ([]xmlRenderNode, error) {
	return xmlFieldNodesWithNestedSchema(name, value, Schema{}, false, config, depth)
}

func xmlFieldNodesForSchema(name string, value Value, schema Schema, config XMLRenderConfig, depth int) ([]xmlRenderNode, error) {
	nested, ok := schema.lookupNestedSchema(name)
	return xmlFieldNodesWithNestedSchema(name, value, nested, ok, config, depth)
}

func xmlFieldNodesWithNestedSchema(name string, value Value, nested Schema, hasNested bool, config XMLRenderConfig, depth int) ([]xmlRenderNode, error) {
	if value.IsMissing() {
		return nil, nil
	}
	if depth > config.MaxDepth {
		return nil, fmt.Errorf("esper: XML event value exceeds max depth %d", config.MaxDepth)
	}
	if value.IsNull() {
		return []xmlRenderNode{{name: xmlName(name)}}, nil
	}
	underlying := value.Any()
	if wrapped, ok := underlying.(Value); ok {
		return xmlFieldNodesWithNestedSchema(name, wrapped, nested, hasNested, config, depth+1)
	}
	if hasNested {
		reflectValue := reflect.ValueOf(underlying)
		for reflectValue.IsValid() && (reflectValue.Kind() == reflect.Pointer || reflectValue.Kind() == reflect.Interface) {
			if reflectValue.IsNil() {
				return []xmlRenderNode{{name: xmlName(name)}}, nil
			}
			reflectValue = reflectValue.Elem()
		}
		if reflectValue.IsValid() && (reflectValue.Kind() == reflect.Array || reflectValue.Kind() == reflect.Slice) && reflectValue.Type() != reflect.TypeOf([]byte{}) {
			result := make([]xmlRenderNode, 0, reflectValue.Len())
			for index := 0; index < reflectValue.Len(); index++ {
				item, err := xmlFieldNodesWithNestedSchema(name, reflectValueToValue(reflectValue.Index(index)), nested, true, config, depth+1)
				if err != nil {
					return nil, err
				}
				result = append(result, item...)
			}
			return result, nil
		}
		node, err := xmlRenderNodeForSchema(nested, name, underlying, config, depth+1)
		if err != nil {
			return nil, err
		}
		return []xmlRenderNode{node}, nil
	}
	reflectValue := reflect.ValueOf(underlying)
	for reflectValue.IsValid() && (reflectValue.Kind() == reflect.Pointer || reflectValue.Kind() == reflect.Interface) {
		if reflectValue.IsNil() {
			return []xmlRenderNode{{name: xmlName(name)}}, nil
		}
		reflectValue = reflectValue.Elem()
	}
	if reflectValue.IsValid() && (reflectValue.Kind() == reflect.Array || reflectValue.Kind() == reflect.Slice) && !(reflectValue.Type() == reflect.TypeOf([]byte{})) {
		if reflectValue.Kind() == reflect.Slice && reflectValue.IsNil() {
			return []xmlRenderNode{{name: xmlName(name)}}, nil
		}
		result := make([]xmlRenderNode, 0, reflectValue.Len())
		for index := 0; index < reflectValue.Len(); index++ {
			item, err := xmlFieldNodes(name, reflectValueToValue(reflectValue.Index(index)), config, depth+1)
			if err != nil {
				return nil, err
			}
			result = append(result, item...)
		}
		return result, nil
	}
	if nested, ok := underlying.(Event); ok {
		node, err := xmlRenderNodeForEvent(nested, name, config, depth+1)
		if err != nil {
			return nil, err
		}
		return []xmlRenderNode{node}, nil
	}
	if row, ok := underlying.(Row); ok {
		node, err := xmlRenderNodeForObject(name, row.schema.fields, row.Get, nil, config, depth+1)
		if err != nil {
			return nil, err
		}
		return []xmlRenderNode{node}, nil
	}
	if isXMLObject(reflectValue) {
		fields := genericStructFields(reflectValue)
		if reflectValue.Kind() == reflect.Map {
			fields = genericMapFields(reflectValue)
		}
		node, err := xmlRenderNodeForObject(name, fields, func(field string) Value {
			if reflectValue.Kind() == reflect.Map {
				return genericMapProperty(reflectValue, field)
			}
			return genericStructProperty(reflectValue, field)
		}, underlying, config, depth+1)
		if err != nil {
			return nil, err
		}
		return []xmlRenderNode{node}, nil
	}
	text := xmlScalar(underlying)
	return []xmlRenderNode{{name: xmlName(name), text: &text}}, nil
}

func writeXMLRenderNode(encoder *xml.Encoder, node xmlRenderNode) error {
	start := xml.StartElement{Name: xml.Name{Local: node.name}, Attr: node.attrs}
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}
	if node.text != nil {
		if err := encoder.EncodeToken(xml.CharData([]byte(*node.text))); err != nil {
			return err
		}
	}
	for _, child := range node.children {
		if err := writeXMLRenderNode(encoder, child); err != nil {
			return err
		}
	}
	return encoder.EncodeToken(start.End())
}

func dynamicMapFields(underlying any) map[string]any {
	if underlying == nil {
		return nil
	}
	if record, ok := underlying.(*AvroRecord); ok {
		return record.AsMap()
	}
	if record, ok := underlying.(AvroRecord); ok {
		return record.AsMap()
	}
	value := reflect.ValueOf(underlying)
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Map || value.Type().Key().Kind() != reflect.String {
		return nil
	}
	result := make(map[string]any, value.Len())
	iterator := value.MapRange()
	for iterator.Next() {
		result[fmt.Sprint(iterator.Key().Interface())] = iterator.Value().Interface()
	}
	return result
}

func genericStructFields(value reflect.Value) []FieldSpec {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return nil
	}
	fields := make([]FieldSpec, 0, value.NumField())
	typ := value.Type()
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		if field.PkgPath != "" {
			continue
		}
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
		if name == "" {
			name = field.Name
		}
		fields = append(fields, FieldSpec{Name: name, Type: field.Type})
	}
	return fields
}

func genericStructProperty(value reflect.Value, name string) Value {
	field := structFieldValue(value, name, PropertyCaseSensitive, nil)
	if field.IsValid() {
		return reflectValueToValue(field)
	}
	return Missing()
}

func genericMapFields(value reflect.Value) []FieldSpec {
	if !value.IsValid() || value.Kind() != reflect.Map || value.Type().Key().Kind() != reflect.String {
		return nil
	}
	fields := make([]FieldSpec, 0, value.Len())
	iterator := value.MapRange()
	for iterator.Next() {
		fields = append(fields, FieldSpec{Name: fmt.Sprint(iterator.Key().Interface())})
	}
	sort.Slice(fields, func(left, right int) bool { return fields[left].Name < fields[right].Name })
	return fields
}

func genericMapProperty(value reflect.Value, name string) Value {
	return reflectMapValue(value, name)
}

func isXMLObject(value reflect.Value) bool {
	if !value.IsValid() {
		return false
	}
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}
	return value.Kind() == reflect.Map || value.Kind() == reflect.Struct
}

func xmlIsScalarState(value Value) bool {
	if !value.IsPresent() {
		return true
	}
	return isXMLScalar(value.Any())
}

func isXMLScalar(value any) bool {
	if value == nil {
		return true
	}
	if _, ok := value.(Event); ok {
		return false
	}
	if _, ok := value.(Row); ok {
		return false
	}
	reflectValue := reflect.ValueOf(value)
	for reflectValue.IsValid() && (reflectValue.Kind() == reflect.Pointer || reflectValue.Kind() == reflect.Interface) {
		if reflectValue.IsNil() {
			return true
		}
		reflectValue = reflectValue.Elem()
	}
	if !reflectValue.IsValid() {
		return true
	}
	return reflectValue.Kind() != reflect.Map && reflectValue.Kind() != reflect.Struct && reflectValue.Kind() != reflect.Array && reflectValue.Kind() != reflect.Slice
}

func xmlScalarState(value Value) string {
	if !value.IsPresent() {
		return ""
	}
	return xmlScalar(value.Any())
}

func xmlScalar(value any) string {
	if value == nil {
		return ""
	}
	if wrapped, ok := value.(Value); ok {
		return xmlScalarState(wrapped)
	}
	if timestamp, ok := value.(time.Time); ok {
		return timestamp.Format(time.RFC3339Nano)
	}
	if integer, ok := value.(big.Int); ok {
		return integer.String()
	}
	if rational, ok := value.(big.Rat); ok {
		return bigRatJSONText(rational)
	}
	text := fmt.Sprint(value)
	var builder strings.Builder
	for _, character := range text {
		if character == '\t' || character == '\n' || character == '\r' || (character >= 0x20 && character <= 0x10ffff) {
			builder.WriteRune(character)
			continue
		}
		builder.WriteString(fmt.Sprintf("\\u%04x", character))
	}
	return builder.String()
}

func bigRatJSONText(value big.Rat) string {
	if value.IsInt() {
		return value.Num().String()
	}
	text := value.FloatString(34)
	text = strings.TrimRight(text, "0")
	text = strings.TrimRight(text, ".")
	if text == "" || text == "-0" {
		return "0"
	}
	return text
}

func xmlName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "event"
	}
	var builder strings.Builder
	for index, character := range name {
		valid := character == '_' || character == '-' || character == '.' || character == ':' ||
			(character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(index > 0 && character >= '0' && character <= '9')
		if !valid || (index == 0 && character >= '0' && character <= '9') {
			builder.WriteByte('_')
		} else {
			builder.WriteRune(character)
		}
	}
	result := builder.String()
	if result == "" {
		return "event"
	}
	return result
}
