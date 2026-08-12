package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type clientRuntimeAnnotationEnum string

const (
	clientRuntimeAnnotationEnumOne   clientRuntimeAnnotationEnum = "ENUM_VALUE_1"
	clientRuntimeAnnotationEnumTwo   clientRuntimeAnnotationEnum = "ENUM_VALUE_2"
	clientRuntimeAnnotationEnumThree clientRuntimeAnnotationEnum = "ENUM_VALUE_3"
)

func TestClientRuntimeStatementAnnotationBuiltinParity(t *testing.T) {
	env, engine := newRuntimeTest(t)
	iterateOnly := mustClientRuntimeStatementHint(t, HintIterateOnly)
	disableReclaim := mustClientRuntimeStatementHint(t, HintDisableReclaimGroup)
	reclaimAged := mustClientRuntimeStatementHint(t, HintReclaimGroupAged, "10")
	index := mustClientRuntimeStatementHint(t, HintIndex, "one", "two")

	plain, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("plain")))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("MyTestStmt"),
		StatementDescription("MyTestStmt description"),
		StatementTag("UserId", "value"),
		WithStatementHints(iterateOnly, disableReclaim, iterateOnly, reclaimAged, index),
		StatementNoLock(),
	))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == plain.Hash() {
		t.Fatal("statement metadata did not enter plan identity")
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	metadata := statement.Metadata()
	if metadata.Name != "MyTestStmt" || !metadata.HasDescription || metadata.Description != "MyTestStmt description" {
		t.Fatalf("built-in metadata = %#v", metadata)
	}
	if !reflect.DeepEqual(metadata.Tags, []StatementTagMetadata{{Name: "UserId", Value: "value"}}) {
		t.Fatalf("statement tags = %#v", metadata.Tags)
	}
	if !metadata.NoLock || !statement.HasNoLock() {
		t.Fatal("NoLock metadata is false")
	}
	if len(metadata.Hints) != 5 || metadata.Hints[0].Kind() != HintIterateOnly || metadata.Hints[2].Kind() != HintIterateOnly {
		t.Fatalf("statement hints = %#v", metadata.Hints)
	}
	if got := metadata.Hints[3].Parameters(); !reflect.DeepEqual(got, []string{"10"}) {
		t.Fatalf("reclaim hint parameters = %v", got)
	}
	if got := metadata.Hints[4].Parameters(); !reflect.DeepEqual(got, []string{"one", "two"}) {
		t.Fatalf("index hint parameters = %v", got)
	}

	metadata.Tags[0].Value = "mutated"
	metadata.Hints[4].parameters[0] = "mutated"
	again := statement.Metadata()
	if again.Tags[0].Value != "value" || again.Hints[4].Parameters()[0] != "one" {
		t.Fatalf("statement metadata was mutable through snapshot: %#v", again)
	}

	var delivered int
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		delivered += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E1"}); err != nil {
		t.Fatal(err)
	}
	if delivered != 1 {
		t.Fatalf("statement delivered %d events, want 1", delivered)
	}
}

func TestClientRuntimeStatementAnnotationAppSimpleParity(t *testing.T) {
	simple := mustClientRuntimeAnnotationDefinition(t, "MyAnnotationSimple")
	value := mustClientRuntimeAnnotationDefinition(t, "MyAnnotationValue",
		RequiredAnnotationAttribute[string]("value"),
	)
	defaulted := mustClientRuntimeAnnotationDefinition(t, "MyAnnotationValueDefaulted",
		DefaultedAnnotationAttribute("value", "XYZ"),
	)
	pair := mustClientRuntimeAnnotationDefinition(t, "MyAnnotationValuePair",
		RequiredAnnotationAttribute[string]("stringVal"),
		RequiredAnnotationAttribute[int]("intVal"),
		RequiredAnnotationAttribute[int64]("longVal"),
		RequiredAnnotationAttribute[bool]("booleanVal"),
		RequiredAnnotationAttribute[rune]("charVal"),
		RequiredAnnotationAttribute[int8]("byteVal"),
		RequiredAnnotationAttribute[int16]("shortVal"),
		RequiredAnnotationAttribute[float64]("doubleVal"),
		DefaultedAnnotationAttribute("stringValDef", "def"),
		DefaultedAnnotationAttribute("intValDef", 100),
		DefaultedAnnotationAttribute("longValDef", int64(200)),
		DefaultedAnnotationAttribute("booleanValDef", true),
		DefaultedAnnotationAttribute("charValDef", rune('D')),
		DefaultedAnnotationAttribute("doubleValDef", 1.1),
	)
	array := mustClientRuntimeAnnotationDefinition(t, "MyAnnotationValueArray",
		RequiredAnnotationAttribute[[]int64]("value"),
		RequiredAnnotationAttribute[[]int]("intArray"),
		RequiredAnnotationAttribute[[]float64]("doubleArray"),
		RequiredAnnotationAttribute[[]string]("stringArray"),
		DefaultedAnnotationAttribute("stringArrayDef", []string{"XYZ"}),
	)
	enum := mustClientRuntimeAnnotationDefinition(t, "MyAnnotationValueEnum",
		RequiredAnnotationAttribute[clientRuntimeAnnotationEnum]("supportEnum"),
		DefaultedAnnotationAttribute("supportEnumDef", clientRuntimeAnnotationEnumTwo),
	)

	simpleAnnotation := mustClientRuntimeAnnotation(t, simple)
	valueAnnotation := mustClientRuntimeAnnotation(t, value, AnnotationField("value", "abc"))
	defaultedAnnotation := mustClientRuntimeAnnotation(t, defaulted)
	pairAnnotation := mustClientRuntimeAnnotation(t, pair,
		AnnotationField("stringVal", "a"), AnnotationField("intVal", -1),
		AnnotationField("longVal", int64(2)), AnnotationField("booleanVal", true),
		AnnotationField("charVal", rune('x')), AnnotationField("byteVal", int8(10)),
		AnnotationField("shortVal", int16(20)), AnnotationField("doubleVal", 2.5),
	)
	longs := []int64{1, 2, 3}
	stringsValue := []string{"X"}
	arrayAnnotation := mustClientRuntimeAnnotation(t, array,
		AnnotationField("value", longs), AnnotationField("intArray", []int{4, 5}),
		AnnotationField("doubleArray", []float64{}), AnnotationField("stringArray", stringsValue),
	)
	enumAnnotation := mustClientRuntimeAnnotation(t, enum, AnnotationField("supportEnum", clientRuntimeAnnotationEnumThree))
	longs[0] = 99
	stringsValue[0] = "mutated"

	statement := deployClientRuntimeAnnotationStatement(t,
		WithStatementAnnotation(simpleAnnotation),
		WithStatementAnnotation(valueAnnotation),
		WithStatementAnnotation(defaultedAnnotation),
		WithStatementAnnotation(enumAnnotation),
		WithStatementAnnotation(pairAnnotation),
		WithStatementAnnotation(arrayAnnotation),
	)
	if got := len(statement.Metadata().Annotations); got != 6 {
		t.Fatalf("custom annotation count = %d, want 6", got)
	}
	assertClientRuntimeAnnotationAttribute(t, statement, "MyAnnotationValue", "value", "abc")
	assertClientRuntimeAnnotationAttribute(t, statement, "MyAnnotationValueDefaulted", "value", "XYZ")
	assertClientRuntimeAnnotationAttribute(t, statement, "MyAnnotationValueEnum", "supportEnum", clientRuntimeAnnotationEnumThree)
	assertClientRuntimeAnnotationAttribute(t, statement, "MyAnnotationValueEnum", "supportEnumDef", clientRuntimeAnnotationEnumTwo)
	assertClientRuntimeAnnotationAttribute(t, statement, "MyAnnotationValuePair", "stringVal", "a")
	assertClientRuntimeAnnotationAttribute(t, statement, "MyAnnotationValuePair", "intVal", -1)
	assertClientRuntimeAnnotationAttribute(t, statement, "MyAnnotationValuePair", "longValDef", int64(200))
	assertClientRuntimeAnnotationAttribute(t, statement, "MyAnnotationValuePair", "charValDef", rune('D'))
	assertClientRuntimeAnnotationAttribute(t, statement, "MyAnnotationValueArray", "value", []int64{1, 2, 3})
	assertClientRuntimeAnnotationAttribute(t, statement, "MyAnnotationValueArray", "stringArray", []string{"X"})
	assertClientRuntimeAnnotationAttribute(t, statement, "MyAnnotationValueArray", "stringArrayDef", []string{"XYZ"})

	annotation, _ := statement.Annotation("MyAnnotationValueArray")
	returned, _ := AnnotationAttributeAs[[]int64](annotation, "value")
	returned[0] = 77
	assertClientRuntimeAnnotationAttribute(t, statement, "MyAnnotationValueArray", "value", []int64{1, 2, 3})
}

func TestClientRuntimeStatementAnnotationAppNestedParity(t *testing.T) {
	nestableSimple := mustClientRuntimeAnnotationDefinition(t, "MyAnnotationNestableSimple")
	nestableValues := mustClientRuntimeAnnotationDefinition(t, "MyAnnotationNestableValues",
		RequiredAnnotationAttribute[int]("val"),
		RequiredAnnotationAttribute[[]int]("arr"),
	)
	nestableNested := mustClientRuntimeAnnotationDefinition(t, "MyAnnotationNestableNestable",
		RequiredAnnotationAttribute[string]("value"),
	)
	nested := mustClientRuntimeAnnotationDefinition(t, "MyAnnotationNested",
		RequiredNestedAnnotationAttribute("nestableSimple", nestableSimple),
		RequiredNestedAnnotationAttribute("nestableValues", nestableValues),
		RequiredNestedAnnotationAttribute("nestableNestable", nestableNested),
	)
	priority := mustClientRuntimeAnnotationDefinition(t, "Priority", RequiredAnnotationAttribute[int]("value"))
	arrayAndClass := mustClientRuntimeAnnotationDefinition(t, "MyAnnotationWArrayAndClass",
		RequiredNestedAnnotationArrayAttribute("priorities", priority),
		RequiredAnnotationAttribute[reflect.Type]("classOne"),
		RequiredAnnotationAttribute[reflect.Type]("classTwo"),
	)

	nestedAnnotation := mustClientRuntimeAnnotation(t, nested,
		AnnotationField("nestableSimple", mustClientRuntimeAnnotation(t, nestableSimple)),
		AnnotationField("nestableValues", mustClientRuntimeAnnotation(t, nestableValues,
			AnnotationField("val", 999), AnnotationField("arr", []int{2, 1}))),
		AnnotationField("nestableNestable", mustClientRuntimeAnnotation(t, nestableNested, AnnotationField("value", "CDF"))),
	)
	classesAnnotation := mustClientRuntimeAnnotation(t, arrayAndClass,
		AnnotationField("priorities", []StatementAnnotation{
			mustClientRuntimeAnnotation(t, priority, AnnotationField("value", 1)),
			mustClientRuntimeAnnotation(t, priority, AnnotationField("value", 3)),
		}),
		AnnotationField("classOne", reflect.TypeOf("")),
		AnnotationField("classTwo", reflect.TypeOf(int(0))),
	)
	statement := deployClientRuntimeAnnotationStatement(t,
		WithStatementAnnotation(nestedAnnotation),
		WithStatementAnnotation(classesAnnotation),
	)

	gotNested, _ := statement.Annotation("MyAnnotationNested")
	values, ok := AnnotationAttributeAs[StatementAnnotation](gotNested, "nestableValues")
	if !ok {
		t.Fatal("nested values annotation is missing")
	}
	if got, _ := AnnotationAttributeAs[int](values, "val"); got != 999 {
		t.Fatalf("nested val = %d, want 999", got)
	}
	if got, _ := AnnotationAttributeAs[[]int](values, "arr"); !reflect.DeepEqual(got, []int{2, 1}) {
		t.Fatalf("nested arr = %v", got)
	}
	leaf, _ := AnnotationAttributeAs[StatementAnnotation](gotNested, "nestableNestable")
	if got, _ := AnnotationAttributeAs[string](leaf, "value"); got != "CDF" {
		t.Fatalf("nested leaf = %q, want CDF", got)
	}

	gotClasses, _ := statement.Annotation("MyAnnotationWArrayAndClass")
	priorities, ok := AnnotationAttributeAs[[]StatementAnnotation](gotClasses, "priorities")
	if !ok || len(priorities) != 2 {
		t.Fatalf("nested priorities = %#v", priorities)
	}
	first, _ := AnnotationAttributeAs[int](priorities[0], "value")
	second, _ := AnnotationAttributeAs[int](priorities[1], "value")
	if first != 1 || second != 3 {
		t.Fatalf("nested priority values = %d, %d", first, second)
	}
	if classOne, _ := AnnotationAttributeAs[reflect.Type](gotClasses, "classOne"); classOne != reflect.TypeOf("") {
		t.Fatalf("classOne = %v", classOne)
	}
	if classTwo, _ := AnnotationAttributeAs[reflect.Type](gotClasses, "classTwo"); classTwo != reflect.TypeOf(int(0)) {
		t.Fatalf("classTwo = %v", classTwo)
	}
}

func TestClientRuntimeStatementAnnotationInvalidParity(t *testing.T) {
	if _, err := NewStatementAnnotationDefinition(" "); err == nil {
		t.Fatal("blank annotation name unexpectedly accepted")
	}
	if _, err := NewStatementAnnotationDefinition("Duplicate",
		RequiredAnnotationAttribute[string]("value"),
		RequiredAnnotationAttribute[int]("value"),
	); err == nil {
		t.Fatal("duplicate annotation attribute unexpectedly accepted")
	}
	invalidDefault := AnnotationAttributeDefinition{name: "value", typ: reflect.TypeOf(""), hasDefault: true, defaultValue: 1}
	if _, err := NewStatementAnnotationDefinition("InvalidDefault", invalidDefault); err == nil {
		t.Fatal("wrongly typed annotation default unexpectedly accepted")
	}

	marker := mustClientRuntimeAnnotationDefinition(t, "Marker")
	value := mustClientRuntimeAnnotationDefinition(t, "Value", RequiredAnnotationAttribute[string]("value"))
	array := mustClientRuntimeAnnotationDefinition(t, "Array", RequiredAnnotationAttribute[[]any]("value"))
	unsupported := mustClientRuntimeAnnotationDefinition(t, "Unsupported", RequiredAnnotationAttribute[map[string]string]("value"))
	wrongNested := mustClientRuntimeAnnotationDefinition(t, "WrongNested")
	nested := mustClientRuntimeAnnotationDefinition(t, "Nested", RequiredNestedAnnotationAttribute("value", marker))

	invalidCases := []struct {
		name  string
		build func() error
	}{
		{name: "missing-required", build: func() error { _, err := value.New(); return err }},
		{name: "wrong-type", build: func() error { _, err := value.New(AnnotationField("value", 5)); return err }},
		{name: "nil-array", build: func() error { _, err := array.New(AnnotationField("value", []any(nil))); return err }},
		{name: "nil-array-element", build: func() error { _, err := array.New(AnnotationField("value", []any{"X", nil})); return err }},
		{name: "duplicate-value", build: func() error {
			_, err := value.New(AnnotationField("value", "a"), AnnotationField("value", "b"))
			return err
		}},
		{name: "unknown-attribute", build: func() error { _, err := value.New(AnnotationField("other", "a")); return err }},
		{name: "marker-value", build: func() error { _, err := marker.New(AnnotationField("value", 5)); return err }},
		{name: "unsupported-value", build: func() error {
			_, err := unsupported.New(AnnotationField("value", map[string]string{"a": "b"}))
			return err
		}},
		{name: "wrong-nested-definition", build: func() error {
			_, err := nested.New(AnnotationField("value", mustClientRuntimeAnnotation(t, wrongNested)))
			return err
		}},
	}
	for _, testCase := range invalidCases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.build(); err == nil {
				t.Fatal("invalid annotation unexpectedly accepted")
			}
		})
	}

	hintCases := []struct {
		kind       StatementHintKind
		parameters []string
	}{
		{kind: StatementHintKind("XXX")},
		{kind: HintReclaimGroupAged},
		{kind: HintIterateOnly, parameters: []string{"5"}},
		{kind: HintIndex},
		{kind: HintIndex, parameters: []string{" "}},
	}
	for _, testCase := range hintCases {
		if _, err := NewStatementHint(testCase.kind, testCase.parameters...); err == nil {
			t.Fatalf("invalid hint %q %v unexpectedly accepted", testCase.kind, testCase.parameters)
		}
	}

	env, _ := newRuntimeTest(t)
	annotation := mustClientRuntimeAnnotation(t, marker)
	_, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		WithStatementAnnotation(annotation), WithStatementAnnotation(annotation),
	))
	if err == nil || !strings.Contains(err.Error(), "duplicate custom statement annotation") {
		t.Fatalf("duplicate statement annotation build error = %v", err)
	}
	_, err = env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementTag(" ", "value")))
	if err == nil || !strings.Contains(err.Error(), "blank name") {
		t.Fatalf("blank statement tag build error = %v", err)
	}
}

func TestClientRuntimeStatementAnnotationSpecificImportParity(t *testing.T) {
	values := []clientRuntimeAnnotationEnum{
		clientRuntimeAnnotationEnumOne,
		clientRuntimeAnnotationEnumTwo,
		clientRuntimeAnnotationEnumThree,
	}
	for _, testCase := range []struct {
		text string
		want clientRuntimeAnnotationEnum
	}{
		{text: "ENUM_VALUE_2", want: clientRuntimeAnnotationEnumTwo},
		{text: "ENUM_value_3", want: clientRuntimeAnnotationEnumThree},
		{text: "enum_value_1", want: clientRuntimeAnnotationEnumOne},
	} {
		got, err := ParseAnnotationStringEnum(testCase.text, values...)
		if err != nil || got != testCase.want {
			t.Fatalf("ParseAnnotationStringEnum(%q) = (%q, %v), want %q", testCase.text, got, err, testCase.want)
		}
	}
	if _, err := ParseAnnotationStringEnum("missing", values...); err == nil {
		t.Fatal("unknown enum value unexpectedly resolved")
	}
}

func TestClientRuntimeStatementAnnotationRecursiveParity(t *testing.T) {
	definition := mustClientRuntimeAnnotationDefinition(t, "MyAnnotationAPIEventType")
	annotation := mustClientRuntimeAnnotation(t, definition)
	env := NewEnvironment()
	schema, err := RegisterMap(env, "ABC", nil, WithSchemaAnnotation(annotation))
	if err != nil {
		t.Fatal(err)
	}
	stored, ok := schema.Annotation("MyAnnotationAPIEventType")
	if !ok || stored.Name() != "MyAnnotationAPIEventType" || len(schema.Annotations()) != 1 {
		t.Fatalf("schema annotations = %#v", schema.Annotations())
	}
	plan, err := env.Build(FromAny(env, "ABC").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plan.Canonical()), "MyAnnotationAPIEventType") {
		t.Fatal("schema annotation is missing from plan identity")
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var delivered int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		delivered += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "ABC", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if delivered != 1 {
		t.Fatalf("schema-annotated stream delivered %d events, want 1", delivered)
	}
}

func mustClientRuntimeStatementHint(t *testing.T, kind StatementHintKind, parameters ...string) StatementHint {
	t.Helper()
	hint, err := NewStatementHint(kind, parameters...)
	if err != nil {
		t.Fatal(err)
	}
	return hint
}

func mustClientRuntimeAnnotationDefinition(t *testing.T, name string, attributes ...AnnotationAttributeDefinition) StatementAnnotationDefinition {
	t.Helper()
	definition, err := NewStatementAnnotationDefinition(name, attributes...)
	if err != nil {
		t.Fatal(err)
	}
	return definition
}

func mustClientRuntimeAnnotation(t *testing.T, definition StatementAnnotationDefinition, values ...AnnotationAttributeValue) StatementAnnotation {
	t.Helper()
	annotation, err := definition.New(values...)
	if err != nil {
		t.Fatal(err)
	}
	return annotation
}

func deployClientRuntimeAnnotationStatement(t *testing.T, options ...QueryOption) *Statement {
	t.Helper()
	env, engine := newRuntimeTest(t)
	options = append([]QueryOption{StatementName("s0")}, options...)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(options...))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	return deployment.Statements()[0]
}

func assertClientRuntimeAnnotationAttribute(t *testing.T, statement *Statement, annotationName, attributeName string, want any) {
	t.Helper()
	annotation, ok := statement.Annotation(annotationName)
	if !ok {
		t.Fatalf("annotation %q is missing", annotationName)
	}
	got, ok := annotation.Attribute(attributeName)
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("annotation %s.%s = (%#v, %t), want %#v", annotationName, attributeName, got, ok, want)
	}
}
