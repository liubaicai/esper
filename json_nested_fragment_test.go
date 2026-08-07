package esper

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

type jsonFragmentNode struct {
	ID    string            `json:"id"`
	Child *jsonFragmentNode `json:"child"`
}

type jsonFragmentRoot struct {
	Local *jsonFragmentNode  `json:"local"`
	Items []jsonFragmentNode `json:"items"`
}

func TestJSONProvidedNestedSchemasExposeRecursiveFragmentsAndMetadata(t *testing.T) {
	schema, err := NewJSONSchemaFor[jsonFragmentRoot]("RecursiveJSON", nil)
	if err != nil {
		t.Fatal(err)
	}
	localSchema, ok := schema.NestedSchema("local")
	if !ok || localSchema.Kind() != SchemaJSON {
		t.Fatalf("local fragment schema = %#v, ok=%t", localSchema, ok)
	}
	itemsSchema, ok := schema.FragmentSchema("items")
	if !ok || itemsSchema.Name() != localSchema.Name() {
		t.Fatalf("array fragment schema = %q/%q, ok=%t", itemsSchema.Name(), localSchema.Name(), ok)
	}
	childSchema, ok := localSchema.NestedSchema("child")
	if !ok || childSchema.Name() != localSchema.Name() {
		t.Fatalf("recursive child schema = %q/%q, ok=%t", childSchema.Name(), localSchema.Name(), ok)
	}
	if got := strings.Join(schema.NestedSchemaNames(), ","); got != "items,local" {
		t.Fatalf("nested schema names = %q", got)
	}
	property, ok := schema.Property("items[0].child.child.id")
	if !ok || property.Type != reflect.TypeOf("") || property.Kind != PropertyIndexed {
		t.Fatalf("recursive property metadata = %#v, ok=%t", property, ok)
	}
	getter, ok := schema.Getter("local.child.child.id")
	if !ok || getter.Type() != reflect.TypeOf("") {
		t.Fatalf("recursive getter metadata = %#v, ok=%t", getter.Property(), ok)
	}

	input := `{"local":{"id":"a","child":{"id":"b","child":{"id":"c","child":null}}},"items":[{"id":"x","child":{"id":"y","child":null}}]}`
	event, err := ParseJSON(schema, []byte(input), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if got := getter.Get(event.Underlying()).Any(); got != "c" {
		t.Fatalf("recursive getter value = %#v", got)
	}
	local, ok := event.GetFragment("local")
	if !ok || local.Get("id").Any() != "a" {
		t.Fatalf("local fragment = %#v, ok=%t", local, ok)
	}
	child, ok := local.GetFragment("child")
	if !ok || child.Get("id").Any() != "b" {
		t.Fatalf("child fragment = %#v, ok=%t", child, ok)
	}
	grandchild, ok := child.GetFragment("child")
	if !ok || grandchild.Get("id").Any() != "c" {
		t.Fatalf("grandchild fragment = %#v, ok=%t", grandchild, ok)
	}
	items, ok := event.GetFragments("items")
	if !ok || len(items) != 1 || items[0].Get("id").Any() != "x" {
		t.Fatalf("indexed fragments = %#v, ok=%t", items, ok)
	}
	itemChild, ok := items[0].GetFragment("child")
	if !ok || itemChild.Get("id").Any() != "y" {
		t.Fatalf("indexed child fragment = %#v, ok=%t", itemChild, ok)
	}
	if rendered, err := RenderJSON(event); err != nil || rendered != input {
		t.Fatalf("recursive JSON render = %s (%v)", rendered, err)
	}
}

func TestJSONMapSchemaInfersStructFragmentAndPreservesExplicitNestedSchema(t *testing.T) {
	stringType := reflect.TypeOf("")
	mapSchema, err := NewJSONSchema("TypedMapJSON", []FieldSpec{
		FieldDef("person", reflect.TypeOf(jsonFragmentNode{})),
		FieldDef("label", stringType),
	})
	if err != nil {
		t.Fatal(err)
	}
	person, ok := mapSchema.NestedSchema("person")
	if !ok || person.PropertyNames()[0] != "id" {
		t.Fatalf("inferred map fragment = %#v, ok=%t", person, ok)
	}
	event, err := ParseJSON(mapSchema, []byte(`{"person":{"id":"p","child":null},"label":"L"}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	fragment, ok := event.GetFragment("person")
	if !ok || fragment.Get("id").Any() != "p" {
		t.Fatalf("map typed fragment = %#v, ok=%t", fragment, ok)
	}

	names, err := NewJSONSchema("NamesForExplicit", []FieldSpec{FieldDef("first", stringType)})
	if err != nil {
		t.Fatal(err)
	}
	explicit, err := NewJSONSchema("ExplicitNested", []FieldSpec{
		FieldDef("person", reflect.TypeOf(map[string]any{})),
	}, WithNestedPropertySchema("person", names))
	if err != nil {
		t.Fatal(err)
	}
	actual, ok := explicit.NestedSchema("person")
	if !ok || actual.Name() != names.Name() {
		t.Fatalf("explicit nested schema replaced = %q/%q, ok=%t", actual.Name(), names.Name(), ok)
	}
}
