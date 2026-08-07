package esper

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math/big"
	"reflect"
	"sort"
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
		value = map[string]any{config.Title: value}
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
	return renderJSONSchemaValue(event.Schema(), event.Underlying(), maxDepth, 0)
}

func renderJSONSchemaValue(schema Schema, underlying any, maxDepth, depth int) (any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("esper: JSON event value exceeds max depth %d", maxDepth)
	}
	if underlying == nil {
		return nil, nil
	}
	if nested, ok := underlying.(Event); ok {
		return renderJSONEventAtDepth(nested, maxDepth, depth+1)
	}
	if row, ok := underlying.(Row); ok {
		return renderJSONRow(row, maxDepth, depth+1)
	}
	if schema.valid() && len(schema.fields) > 0 {
		return renderJSONSchemaObject(schema, underlying, maxDepth, depth)
	}
	return renderJSONValue(underlying, maxDepth, depth)
}

func renderJSONSchemaObject(schema Schema, underlying any, maxDepth, depth int) (map[string]any, error) {
	result := make(map[string]any, len(schema.fields))
	known := make(map[string]struct{}, len(schema.fields))
	for _, field := range schema.fields {
		known[field.Name] = struct{}{}
		value := schema.get(underlying, field.Name)
		if value.IsMissing() {
			continue
		}
		normalized, err := renderJSONSchemaProperty(schema, field.Name, value, maxDepth, depth+1)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", field.Name, err)
		}
		result[field.Name] = normalized
	}
	for name, value := range dynamicMapFields(underlying) {
		if _, exists := known[name]; exists {
			continue
		}
		normalized, err := renderJSONValue(value, maxDepth, depth+1)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", name, err)
		}
		result[name] = normalized
	}
	return result, nil
}

func renderJSONSchemaProperty(schema Schema, name string, value Value, maxDepth, depth int) (any, error) {
	nested, ok := schema.lookupNestedSchema(name)
	if !ok || value.IsMissing() || value.IsNull() {
		return renderJSONValueState(value, maxDepth, depth)
	}
	return renderJSONNestedSchemaValue(nested, value.Any(), maxDepth, depth)
}

func renderJSONNestedSchemaValue(schema Schema, underlying any, maxDepth, depth int) (any, error) {
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
		for index := 0; index < value.Len(); index++ {
			item, err := renderJSONNestedSchemaValue(schema, value.Index(index).Interface(), maxDepth, depth+1)
			if err != nil {
				return nil, err
			}
			result[index] = item
		}
		return result, nil
	}
	return renderJSONSchemaValue(schema, underlying, maxDepth, depth)
}

func renderJSONEventAtDepth(event Event, maxDepth, depth int) (any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("esper: JSON event value exceeds max depth %d", maxDepth)
	}
	return renderJSONSchemaValue(event.Schema(), event.Underlying(), maxDepth, depth)
}

func renderJSONRow(row Row, maxDepth, depth int) (any, error) {
	fields := row.schema.fields
	return renderJSONObject(fields, row.Get, nil, maxDepth, depth)
}

func renderJSONObject(fields []FieldSpec, get func(string) Value, underlying any, maxDepth, depth int) (map[string]any, error) {
	result := make(map[string]any, len(fields))
	known := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		known[field.Name] = struct{}{}
		value := get(field.Name)
		if value.IsMissing() {
			continue
		}
		normalized, err := renderJSONValueState(value, maxDepth, depth+1)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", field.Name, err)
		}
		result[field.Name] = normalized
	}
	for name, value := range dynamicMapFields(underlying) {
		if _, exists := known[name]; exists {
			continue
		}
		normalized, err := renderJSONValue(value, maxDepth, depth+1)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", name, err)
		}
		result[name] = normalized
	}
	return result, nil
}

func renderJSONValueState(value Value, maxDepth, depth int) (any, error) {
	if value.IsMissing() || value.IsNull() {
		return nil, nil
	}
	return renderJSONValue(value.Any(), maxDepth, depth)
}

func renderJSONValue(value any, maxDepth, depth int) (any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("value exceeds max depth %d", maxDepth)
	}
	if value == nil {
		return nil, nil
	}
	if wrapped, ok := value.(Value); ok {
		return renderJSONValueState(wrapped, maxDepth, depth+1)
	}
	if event, ok := value.(Event); ok {
		return renderJSONEventAtDepth(event, maxDepth, depth+1)
	}
	if row, ok := value.(Row); ok {
		return renderJSONRow(row, maxDepth, depth+1)
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
	if reflectValue.Type() == reflect.TypeOf(time.Time{}) {
		return reflectValue.Interface().(time.Time).Format(time.RFC3339Nano), nil
	}
	if reflectValue.Type() == reflect.TypeOf(DateOnly("")) {
		return string(reflectValue.Interface().(DateOnly)), nil
	}
	if reflectValue.Type() == reflect.TypeOf(UUID{}) {
		return reflectValue.Interface().(UUID).String(), nil
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
		result := make(map[string]any, reflectValue.Len())
		iterator := reflectValue.MapRange()
		for iterator.Next() {
			key := fmt.Sprint(iterator.Key().Interface())
			normalized, err := renderJSONValue(iterator.Value().Interface(), maxDepth, depth+1)
			if err != nil {
				return nil, err
			}
			result[key] = normalized
		}
		return result, nil
	case reflect.Struct:
		fields := genericStructFields(reflectValue)
		return renderJSONObject(fields, func(name string) Value {
			return genericStructProperty(reflectValue, name)
		}, value, maxDepth, depth)
	case reflect.Array, reflect.Slice:
		if reflectValue.Kind() == reflect.Slice && reflectValue.IsNil() {
			return nil, nil
		}
		result := make([]any, reflectValue.Len())
		for index := 0; index < reflectValue.Len(); index++ {
			normalized, err := renderJSONValue(reflectValue.Index(index).Interface(), maxDepth, depth+1)
			if err != nil {
				return nil, err
			}
			result[index] = normalized
		}
		return result, nil
	default:
		return value, nil
	}
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
	sort.Slice(fields, func(left, right int) bool { return fields[left].Name < fields[right].Name })
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
