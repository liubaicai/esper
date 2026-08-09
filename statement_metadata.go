package esper

import (
	"fmt"
	"reflect"
	"strings"
)

// StatementHintKind identifies one of Esper's built-in statement hints. The
// Go API keeps hints typed instead of accepting the comma-separated EPL
// representation.
type StatementHintKind string

const (
	HintIterateOnly                     StatementHintKind = "ITERATE_ONLY"
	HintDisableReclaimGroup             StatementHintKind = "DISABLE_RECLAIM_GROUP"
	HintReclaimGroupAged                StatementHintKind = "RECLAIM_GROUP_AGED"
	HintReclaimGroupFreq                StatementHintKind = "RECLAIM_GROUP_FREQ"
	HintEnableWindowSubqueryIndexShare  StatementHintKind = "ENABLE_WINDOW_SUBQUERY_INDEXSHARE"
	HintDisableWindowSubqueryIndexShare StatementHintKind = "DISABLE_WINDOW_SUBQUERY_INDEXSHARE"
	HintSetNoIndex                      StatementHintKind = "SET_NOINDEX"
	HintForceNestedIteration            StatementHintKind = "FORCE_NESTED_ITER"
	HintPreferMergeJoin                 StatementHintKind = "PREFER_MERGE_JOIN"
	HintIndex                           StatementHintKind = "INDEX"
	HintExcludePlan                     StatementHintKind = "EXCLUDE_PLAN"
	HintDisableUniqueImplicitIndex      StatementHintKind = "DISABLE_UNIQUE_IMPLICIT_IDX"
	HintMaxFilterWidth                  StatementHintKind = "MAX_FILTER_WIDTH"
	HintFilterIndex                     StatementHintKind = "FILTERINDEX"
	HintDisableWhereMoveToFilter        StatementHintKind = "DISABLE_WHEREEXPR_MOVETO_FILTER"
	HintEnableOutputLimitOptimization   StatementHintKind = "ENABLE_OUTPUTLIMIT_OPT"
	HintDisableOutputLimitOptimization  StatementHintKind = "DISABLE_OUTPUTLIMIT_OPT"
	HintSilentDelete                    StatementHintKind = "SILENT_DELETE"
)

type statementHintContract struct {
	acceptsParameters bool
	requiresParameter bool
	requiresList      bool
}

var statementHintContracts = map[StatementHintKind]statementHintContract{
	HintIterateOnly:                     {},
	HintDisableReclaimGroup:             {},
	HintReclaimGroupAged:                {acceptsParameters: true, requiresParameter: true},
	HintReclaimGroupFreq:                {acceptsParameters: true, requiresParameter: true},
	HintEnableWindowSubqueryIndexShare:  {},
	HintDisableWindowSubqueryIndexShare: {},
	HintSetNoIndex:                      {},
	HintForceNestedIteration:            {},
	HintPreferMergeJoin:                 {},
	HintIndex:                           {acceptsParameters: true, requiresParameter: true, requiresList: true},
	HintExcludePlan:                     {acceptsParameters: true, requiresParameter: true, requiresList: true},
	HintDisableUniqueImplicitIndex:      {},
	HintMaxFilterWidth:                  {acceptsParameters: true, requiresParameter: true},
	HintFilterIndex:                     {acceptsParameters: true, requiresParameter: true, requiresList: true},
	HintDisableWhereMoveToFilter:        {},
	HintEnableOutputLimitOptimization:   {},
	HintDisableOutputLimitOptimization:  {},
	HintSilentDelete:                    {},
}

// StatementHint is an immutable typed hint value. Parameters correspond to
// the assigned value for scalar hints and to the parenthesized values for
// INDEX, EXCLUDE_PLAN and FILTERINDEX.
type StatementHint struct {
	kind       StatementHintKind
	parameters []string
}

// NewStatementHint validates and constructs a typed built-in hint.
func NewStatementHint(kind StatementHintKind, parameters ...string) (StatementHint, error) {
	kind = StatementHintKind(strings.ToUpper(strings.TrimSpace(string(kind))))
	contract, ok := statementHintContracts[kind]
	if !ok {
		return StatementHint{}, fmt.Errorf("esper: unknown statement hint %q", kind)
	}
	if !contract.acceptsParameters && len(parameters) != 0 {
		return StatementHint{}, fmt.Errorf("esper: statement hint %q does not accept parameters", kind)
	}
	if contract.requiresParameter && len(parameters) == 0 {
		return StatementHint{}, fmt.Errorf("esper: statement hint %q requires a parameter", kind)
	}
	if !contract.requiresList && len(parameters) > 1 {
		return StatementHint{}, fmt.Errorf("esper: statement hint %q accepts exactly one parameter", kind)
	}
	copyParameters := make([]string, len(parameters))
	for index, parameter := range parameters {
		parameter = strings.TrimSpace(parameter)
		if parameter == "" {
			return StatementHint{}, fmt.Errorf("esper: statement hint %q parameter %d cannot be blank", kind, index)
		}
		copyParameters[index] = parameter
	}
	return StatementHint{kind: kind, parameters: copyParameters}, nil
}

func (h StatementHint) Kind() StatementHintKind { return h.kind }

func (h StatementHint) Parameters() []string {
	return append([]string(nil), h.parameters...)
}

func (h StatementHint) valid() bool {
	validated, err := NewStatementHint(h.kind, h.parameters...)
	return err == nil && validated.kind == h.kind
}

func cloneStatementHint(hint StatementHint) StatementHint {
	return StatementHint{kind: hint.kind, parameters: append([]string(nil), hint.parameters...)}
}

// StatementTagMetadata is the immutable name/value form exposed through a
// statement metadata snapshot.
type StatementTagMetadata struct {
	Name  string
	Value string
}

// AnnotationAttributeDefinition declares one custom annotation attribute.
// Use RequiredAnnotationAttribute, DefaultedAnnotationAttribute or the nested
// annotation helpers to construct values.
type AnnotationAttributeDefinition struct {
	name         string
	typ          reflect.Type
	required     bool
	hasDefault   bool
	defaultValue any
	nested       *statementAnnotationDefinition
	nestedArray  bool
}

// RequiredAnnotationAttribute declares a required primitive, string, enum,
// reflect.Type, slice or array attribute.
func RequiredAnnotationAttribute[T any](name string) AnnotationAttributeDefinition {
	return AnnotationAttributeDefinition{name: name, typ: typeOf[T](), required: true}
}

// DefaultedAnnotationAttribute declares an attribute whose value is supplied
// from an immutable deep copy when omitted by an annotation instance.
func DefaultedAnnotationAttribute[T any](name string, value T) AnnotationAttributeDefinition {
	return AnnotationAttributeDefinition{name: name, typ: typeOf[T](), hasDefault: true, defaultValue: value}
}

// RequiredNestedAnnotationAttribute declares one required nested custom
// annotation value.
func RequiredNestedAnnotationAttribute(name string, definition StatementAnnotationDefinition) AnnotationAttributeDefinition {
	return AnnotationAttributeDefinition{name: name, required: true, nested: definition.definition}
}

// RequiredNestedAnnotationArrayAttribute declares a required slice of nested
// custom annotations, each using the supplied definition.
func RequiredNestedAnnotationArrayAttribute(name string, definition StatementAnnotationDefinition) AnnotationAttributeDefinition {
	return AnnotationAttributeDefinition{name: name, required: true, nested: definition.definition, nestedArray: true}
}

type statementAnnotationDefinition struct {
	name       string
	attributes []AnnotationAttributeDefinition
	index      map[string]int
}

// StatementAnnotationDefinition is an immutable custom annotation contract.
type StatementAnnotationDefinition struct {
	definition *statementAnnotationDefinition
}

// NewStatementAnnotationDefinition validates a custom annotation contract.
func NewStatementAnnotationDefinition(name string, attributes ...AnnotationAttributeDefinition) (StatementAnnotationDefinition, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return StatementAnnotationDefinition{}, fmt.Errorf("esper: statement annotation name is required")
	}
	definition := &statementAnnotationDefinition{name: name, index: make(map[string]int, len(attributes))}
	definition.attributes = make([]AnnotationAttributeDefinition, len(attributes))
	for index, attribute := range attributes {
		attribute.name = strings.TrimSpace(attribute.name)
		if attribute.name == "" {
			return StatementAnnotationDefinition{}, fmt.Errorf("esper: statement annotation %q attribute %d has a blank name", name, index)
		}
		if _, exists := definition.index[attribute.name]; exists {
			return StatementAnnotationDefinition{}, fmt.Errorf("esper: statement annotation %q duplicates attribute %q", name, attribute.name)
		}
		if attribute.nested != nil {
			if attribute.typ != nil || attribute.hasDefault {
				return StatementAnnotationDefinition{}, fmt.Errorf("esper: statement annotation %q nested attribute %q has an invalid contract", name, attribute.name)
			}
		} else if attribute.typ == nil {
			return StatementAnnotationDefinition{}, fmt.Errorf("esper: statement annotation %q attribute %q has no type", name, attribute.name)
		}
		if attribute.hasDefault {
			cloned, err := validateAndCloneAnnotationValue(attribute.typ, attribute.defaultValue)
			if err != nil {
				return StatementAnnotationDefinition{}, fmt.Errorf("esper: statement annotation %q default for %q: %w", name, attribute.name, err)
			}
			attribute.defaultValue = cloned
		}
		definition.index[attribute.name] = index
		definition.attributes[index] = attribute
	}
	return StatementAnnotationDefinition{definition: definition}, nil
}

func (d StatementAnnotationDefinition) Name() string {
	if d.definition == nil {
		return ""
	}
	return d.definition.name
}

// AnnotationAttributeValue supplies one named value to a custom annotation.
type AnnotationAttributeValue struct {
	name  string
	value any
}

// AnnotationField supplies one attribute value. New validates duplicates,
// required attributes, type contracts and nested annotation identity.
func AnnotationField(name string, value any) AnnotationAttributeValue {
	return AnnotationAttributeValue{name: name, value: value}
}

type statementAnnotationAttribute struct {
	name  string
	value any
}

// StatementAnnotation is one immutable custom annotation instance.
type StatementAnnotation struct {
	definition *statementAnnotationDefinition
	attributes []statementAnnotationAttribute
}

// New creates one custom annotation instance using the definition contract.
func (d StatementAnnotationDefinition) New(values ...AnnotationAttributeValue) (StatementAnnotation, error) {
	if d.definition == nil {
		return StatementAnnotation{}, fmt.Errorf("esper: invalid statement annotation definition")
	}
	provided := make(map[string]any, len(values))
	for index, value := range values {
		name := strings.TrimSpace(value.name)
		if name == "" {
			return StatementAnnotation{}, fmt.Errorf("esper: statement annotation %q value %d has a blank attribute name", d.definition.name, index)
		}
		if _, exists := provided[name]; exists {
			return StatementAnnotation{}, fmt.Errorf("esper: statement annotation %q duplicates attribute value %q", d.definition.name, name)
		}
		if _, exists := d.definition.index[name]; !exists {
			return StatementAnnotation{}, fmt.Errorf("esper: statement annotation %q has no attribute %q", d.definition.name, name)
		}
		provided[name] = value.value
	}
	annotation := StatementAnnotation{definition: d.definition, attributes: make([]statementAnnotationAttribute, 0, len(d.definition.attributes))}
	for _, attribute := range d.definition.attributes {
		value, exists := provided[attribute.name]
		if !exists {
			if attribute.hasDefault {
				cloned, err := cloneAnnotationValue(attribute.defaultValue)
				if err != nil {
					return StatementAnnotation{}, err
				}
				annotation.attributes = append(annotation.attributes, statementAnnotationAttribute{name: attribute.name, value: cloned})
				continue
			}
			if attribute.required {
				return StatementAnnotation{}, fmt.Errorf("esper: statement annotation %q requires attribute %q", d.definition.name, attribute.name)
			}
			continue
		}
		cloned, err := validateAndCloneAnnotationAttribute(attribute, value)
		if err != nil {
			return StatementAnnotation{}, fmt.Errorf("esper: statement annotation %q attribute %q: %w", d.definition.name, attribute.name, err)
		}
		annotation.attributes = append(annotation.attributes, statementAnnotationAttribute{name: attribute.name, value: cloned})
	}
	return annotation, nil
}

func (a StatementAnnotation) Name() string {
	if a.definition == nil {
		return ""
	}
	return a.definition.name
}

// Attribute returns an immutable copy of a named annotation value.
func (a StatementAnnotation) Attribute(name string) (any, bool) {
	for _, attribute := range a.attributes {
		if attribute.name != name {
			continue
		}
		cloned, err := cloneAnnotationValue(attribute.value)
		if err != nil {
			return nil, false
		}
		return cloned, true
	}
	return nil, false
}

// AnnotationAttributeAs is the typed convenience form of Attribute.
func AnnotationAttributeAs[T any](annotation StatementAnnotation, name string) (T, bool) {
	var zero T
	value, ok := annotation.Attribute(name)
	if !ok {
		return zero, false
	}
	typed, ok := value.(T)
	return typed, ok
}

func cloneStatementAnnotation(annotation StatementAnnotation) StatementAnnotation {
	clone := StatementAnnotation{definition: annotation.definition, attributes: make([]statementAnnotationAttribute, len(annotation.attributes))}
	for index, attribute := range annotation.attributes {
		value, err := cloneAnnotationValue(attribute.value)
		if err != nil {
			continue
		}
		clone.attributes[index] = statementAnnotationAttribute{name: attribute.name, value: value}
	}
	return clone
}

func cloneStatementAnnotations(annotations []StatementAnnotation) []StatementAnnotation {
	clones := make([]StatementAnnotation, len(annotations))
	for index, annotation := range annotations {
		clones[index] = cloneStatementAnnotation(annotation)
	}
	return clones
}

func validateAndCloneAnnotationAttribute(attribute AnnotationAttributeDefinition, value any) (any, error) {
	if attribute.nested == nil {
		return validateAndCloneAnnotationValue(attribute.typ, value)
	}
	if !attribute.nestedArray {
		annotation, ok := value.(StatementAnnotation)
		if !ok || annotation.definition == nil {
			return nil, fmt.Errorf("expects nested annotation %q, got %T", attribute.nested.name, value)
		}
		if annotation.definition != attribute.nested {
			return nil, fmt.Errorf("expects nested annotation %q, got %q", attribute.nested.name, annotation.Name())
		}
		return cloneStatementAnnotation(annotation), nil
	}
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() || (reflected.Kind() != reflect.Slice && reflected.Kind() != reflect.Array) {
		return nil, fmt.Errorf("expects an array of nested annotation %q, got %T", attribute.nested.name, value)
	}
	if reflected.Kind() == reflect.Slice && reflected.IsNil() {
		return nil, fmt.Errorf("expects a non-nil array of nested annotation %q", attribute.nested.name)
	}
	result := make([]StatementAnnotation, reflected.Len())
	for index := 0; index < reflected.Len(); index++ {
		item, ok := reflected.Index(index).Interface().(StatementAnnotation)
		if !ok || item.definition == nil {
			return nil, fmt.Errorf("nested annotation array element %d expects %q, got %T", index, attribute.nested.name, reflected.Index(index).Interface())
		}
		if item.definition != attribute.nested {
			return nil, fmt.Errorf("nested annotation array element %d expects %q, got %q", index, attribute.nested.name, item.Name())
		}
		result[index] = cloneStatementAnnotation(item)
	}
	return result, nil
}

func validateAndCloneAnnotationValue(expected reflect.Type, value any) (any, error) {
	if value == nil {
		return nil, fmt.Errorf("requires a non-nil %s value", expected)
	}
	actual := reflect.TypeOf(value)
	if !actual.AssignableTo(expected) {
		return nil, fmt.Errorf("expects %s, got %s", expected, actual)
	}
	return cloneAnnotationValue(value)
}

var statementAnnotationType = reflect.TypeOf(StatementAnnotation{})

func cloneAnnotationValue(value any) (any, error) {
	if value == nil {
		return nil, fmt.Errorf("annotation values cannot be nil")
	}
	if _, ok := value.(reflect.Type); ok {
		return value, nil
	}
	if annotation, ok := value.(StatementAnnotation); ok {
		if annotation.definition == nil {
			return nil, fmt.Errorf("annotation values cannot contain an invalid nested annotation")
		}
		return cloneStatementAnnotation(annotation), nil
	}
	cloned, err := cloneAnnotationReflectValue(reflect.ValueOf(value))
	if err != nil {
		return nil, err
	}
	return cloned.Interface(), nil
}

func cloneAnnotationReflectValue(value reflect.Value) (reflect.Value, error) {
	if !value.IsValid() {
		return reflect.Value{}, fmt.Errorf("annotation values cannot be nil")
	}
	if value.CanInterface() {
		if _, ok := value.Interface().(reflect.Type); ok {
			return value, nil
		}
	}
	if value.Type() == statementAnnotationType {
		annotation := cloneStatementAnnotation(value.Interface().(StatementAnnotation))
		return reflect.ValueOf(annotation), nil
	}
	switch value.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.String:
		return value, nil
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Value{}, fmt.Errorf("annotation values cannot contain nil interface values")
		}
		cloned, err := cloneAnnotationReflectValue(value.Elem())
		if err != nil {
			return reflect.Value{}, err
		}
		wrapped := reflect.New(value.Type()).Elem()
		wrapped.Set(cloned)
		return wrapped, nil
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Value{}, fmt.Errorf("annotation array values cannot be nil")
		}
		clone := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := 0; index < value.Len(); index++ {
			item, err := cloneAnnotationReflectValue(value.Index(index))
			if err != nil {
				return reflect.Value{}, fmt.Errorf("annotation array element %d: %w", index, err)
			}
			clone.Index(index).Set(item)
		}
		return clone, nil
	case reflect.Array:
		clone := reflect.New(value.Type()).Elem()
		for index := 0; index < value.Len(); index++ {
			item, err := cloneAnnotationReflectValue(value.Index(index))
			if err != nil {
				return reflect.Value{}, fmt.Errorf("annotation array element %d: %w", index, err)
			}
			clone.Index(index).Set(item)
		}
		return clone, nil
	default:
		return reflect.Value{}, fmt.Errorf("annotation value type %s is not supported", value.Type())
	}
}

// ParseAnnotationStringEnum resolves a Go string enum without Java imports or
// class loading. Matching is case-insensitive while the returned value keeps
// the caller's canonical enum constant.
func ParseAnnotationStringEnum[T ~string](text string, values ...T) (T, error) {
	var zero T
	text = strings.TrimSpace(text)
	for _, value := range values {
		if strings.EqualFold(text, string(value)) {
			return value, nil
		}
	}
	return zero, fmt.Errorf("esper: annotation enum value %q is not recognized", text)
}

type statementMetadata struct {
	description    string
	descriptionSet bool
	tags           []StatementTagMetadata
	hints          []StatementHint
	noLock         bool
	annotations    []StatementAnnotation
}

func cloneStatementMetadata(metadata statementMetadata) statementMetadata {
	clone := metadata
	clone.tags = append([]StatementTagMetadata(nil), metadata.tags...)
	clone.hints = make([]StatementHint, len(metadata.hints))
	for index, hint := range metadata.hints {
		clone.hints[index] = cloneStatementHint(hint)
	}
	clone.annotations = cloneStatementAnnotations(metadata.annotations)
	return clone
}

func validateStatementMetadata(metadata statementMetadata) error {
	seenAnnotations := make(map[string]struct{}, len(metadata.annotations))
	for index, tag := range metadata.tags {
		if strings.TrimSpace(tag.Name) == "" {
			return fmt.Errorf("esper: statement tag %d has a blank name", index)
		}
	}
	for index, hint := range metadata.hints {
		if !hint.valid() {
			return fmt.Errorf("esper: statement hint %d is invalid", index)
		}
	}
	for index, annotation := range metadata.annotations {
		if annotation.definition == nil || annotation.Name() == "" {
			return fmt.Errorf("esper: custom statement annotation %d is invalid", index)
		}
		if _, exists := seenAnnotations[annotation.Name()]; exists {
			return fmt.Errorf("esper: duplicate custom statement annotation %q", annotation.Name())
		}
		seenAnnotations[annotation.Name()] = struct{}{}
	}
	return nil
}

func validateSchemaAnnotations(annotations []StatementAnnotation) error {
	seen := make(map[string]struct{}, len(annotations))
	for index, annotation := range annotations {
		if annotation.definition == nil || annotation.Name() == "" {
			return fmt.Errorf("custom schema annotation %d is invalid", index)
		}
		if _, exists := seen[annotation.Name()]; exists {
			return fmt.Errorf("duplicate custom schema annotation %q", annotation.Name())
		}
		seen[annotation.Name()] = struct{}{}
	}
	return nil
}

func statementMetadataCanonical(metadata statementMetadata) string {
	parts := make([]string, 0, 4+len(metadata.tags)+len(metadata.hints)+len(metadata.annotations))
	if metadata.descriptionSet {
		parts = append(parts, fmt.Sprintf("description(%q)", metadata.description))
	}
	for _, tag := range metadata.tags {
		parts = append(parts, fmt.Sprintf("tag(%q=%q)", tag.Name, tag.Value))
	}
	for _, hint := range metadata.hints {
		parameters := make([]string, len(hint.parameters))
		for index, parameter := range hint.parameters {
			parameters[index] = fmt.Sprintf("%q", parameter)
		}
		parts = append(parts, fmt.Sprintf("hint(%s:%s)", hint.kind, strings.Join(parameters, ",")))
	}
	if metadata.noLock {
		parts = append(parts, "no-lock")
	}
	if annotations := statementAnnotationsCanonical(metadata.annotations); annotations != "" {
		parts = append(parts, "annotations("+annotations+")")
	}
	if len(parts) == 0 {
		return ""
	}
	return "statement-metadata(" + strings.Join(parts, ",") + ")"
}

func statementAnnotationsCanonical(annotations []StatementAnnotation) string {
	parts := make([]string, len(annotations))
	for index, annotation := range annotations {
		attributes := make([]string, len(annotation.attributes))
		for attributeIndex, attribute := range annotation.attributes {
			attributes[attributeIndex] = attribute.name + "=" + annotationValueCanonical(attribute.value)
		}
		parts[index] = annotation.Name() + "(" + strings.Join(attributes, ",") + ")"
	}
	return strings.Join(parts, ";")
}

func annotationValueCanonical(value any) string {
	if value == nil {
		return "<nil>"
	}
	if typ, ok := value.(reflect.Type); ok {
		return "type(" + typ.String() + ")"
	}
	if annotation, ok := value.(StatementAnnotation); ok {
		return statementAnnotationsCanonical([]StatementAnnotation{annotation})
	}
	return annotationReflectValueCanonical(reflect.ValueOf(value))
}

func annotationReflectValueCanonical(value reflect.Value) string {
	if !value.IsValid() {
		return "<nil>"
	}
	if value.CanInterface() {
		if typ, ok := value.Interface().(reflect.Type); ok {
			return "type(" + typ.String() + ")"
		}
		if annotation, ok := value.Interface().(StatementAnnotation); ok {
			return statementAnnotationsCanonical([]StatementAnnotation{annotation})
		}
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return "<nil>"
		}
		return annotationReflectValueCanonical(value.Elem())
	case reflect.String:
		return value.Type().String() + "(" + fmt.Sprintf("%q", value.String()) + ")"
	case reflect.Bool:
		return value.Type().String() + "(" + fmt.Sprintf("%t", value.Bool()) + ")"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Type().String() + "(" + fmt.Sprintf("%d", value.Int()) + ")"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return value.Type().String() + "(" + fmt.Sprintf("%d", value.Uint()) + ")"
	case reflect.Float32, reflect.Float64:
		return value.Type().String() + "(" + fmt.Sprintf("%g", value.Float()) + ")"
	case reflect.Slice, reflect.Array:
		items := make([]string, value.Len())
		for index := 0; index < value.Len(); index++ {
			items[index] = annotationReflectValueCanonical(value.Index(index))
		}
		return value.Type().String() + "[" + strings.Join(items, ",") + "]"
	default:
		return value.Type().String()
	}
}

// StatementMetadata is an immutable snapshot of a statement's built-in and
// application-defined metadata. Returned slices and annotation values are
// detached from the deployed plan.
type StatementMetadata struct {
	Name           string
	Description    string
	HasDescription bool
	Tags           []StatementTagMetadata
	Hints          []StatementHint
	NoLock         bool
	Annotations    []StatementAnnotation
}

func statementMetadataSnapshot(name string, metadata statementMetadata) StatementMetadata {
	clone := cloneStatementMetadata(metadata)
	return StatementMetadata{
		Name: name, Description: clone.description, HasDescription: clone.descriptionSet,
		Tags: clone.tags, Hints: clone.hints, NoLock: clone.noLock, Annotations: clone.annotations,
	}
}

func statementAnnotationByName(annotations []StatementAnnotation, name string) (StatementAnnotation, bool) {
	for _, annotation := range annotations {
		if annotation.Name() == name {
			return cloneStatementAnnotation(annotation), true
		}
	}
	return StatementAnnotation{}, false
}
