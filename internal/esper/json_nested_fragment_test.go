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

func TestJSONNamedNestedSchemaCatalogMatchesJavaCreateSchemaFragments(t *testing.T) {
	env := NewEnvironment()
	stringType := reflect.TypeOf("")
	friend, err := RegisterJSON(env, "Friend", []FieldSpec{
		FieldDef("id", stringType),
		FieldDef("name", stringType),
	})
	if err != nil {
		t.Fatal(err)
	}
	friendOption, err := WithNestedPropertySchemaFrom(env, "friends", "Friend")
	if err != nil {
		t.Fatal(err)
	}
	user, err := RegisterJSON(env, "User", []FieldSpec{
		FieldDef("id", stringType),
		FieldDef("friends", reflect.TypeOf([]map[string]any{})),
	}, friendOption)
	if err != nil {
		t.Fatal(err)
	}
	usersOption, err := WithNestedPropertySchemaFrom(env, "users", "User")
	if err != nil {
		t.Fatal(err)
	}
	users, err := RegisterJSON(env, "Users", []FieldSpec{
		FieldDef("users", reflect.TypeOf([]map[string]any{})),
	}, usersOption)
	if err != nil {
		t.Fatal(err)
	}

	if got, ok := user.NestedSchema("friends"); !ok || got.Name() != friend.Name() || got.Kind() != SchemaJSON {
		t.Fatalf("named Friend fragment = %q/%v, ok=%t", got.Name(), got.Kind(), ok)
	}
	if got, ok := users.FragmentSchema("users"); !ok || got.Name() != user.Name() {
		t.Fatalf("named User fragment = %q, want %q, ok=%t", got.Name(), user.Name(), ok)
	}
	if descriptor, ok := users.Property("users[0].friends[0].name"); !ok || descriptor.Type != stringType || descriptor.Kind != PropertyIndexed {
		t.Fatalf("named fragment property metadata = %#v, ok=%t", descriptor, ok)
	}
	getter, ok := users.Getter("users[0].friends[0].name")
	if !ok || getter.Type() != stringType {
		t.Fatalf("named fragment getter metadata = %#v, ok=%t", getter.Property(), ok)
	}

	input := `{"users":[{"id":"U1","friends":[{"id":"F1","name":"Alice"}]}]}`
	event, err := ParseJSON(users, []byte(input), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if got := getter.Get(event.Underlying()).Any(); got != "Alice" {
		t.Fatalf("named fragment getter value = %#v", got)
	}
	userFragments, ok := event.GetFragments("users")
	if !ok || len(userFragments) != 1 {
		t.Fatalf("named User fragments = %#v, ok=%t", userFragments, ok)
	}
	friends, ok := userFragments[0].GetFragments("friends")
	if !ok || len(friends) != 1 || friends[0].Get("name").Any() != "Alice" {
		t.Fatalf("named Friend fragments = %#v, ok=%t", friends, ok)
	}
	if rendered, err := RenderJSON(event); err != nil || rendered != input {
		t.Fatalf("named fragment render = %s (%v)", rendered, err)
	}
	plan, err := env.Build(FromAny(env, "Users").Query(StatementName("named-fragment-catalog")))
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(FromAny(env, "Users").Query(StatementName("named-fragment-catalog")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !strings.Contains(string(plan.Canonical()), "nested=users=User") || !strings.Contains(string(plan.Canonical()), "nested=friends=Friend") {
		t.Fatalf("named fragment plan identity = %s\n%s", plan.Hash(), plan.Canonical())
	}
}

func TestJSONNamedNestedSchemaCatalogRejectsMissingAndCrossEnvironmentReferences(t *testing.T) {
	if _, err := WithNestedPropertySchemaFrom(nil, "child", "Child"); err == nil || !strings.Contains(err.Error(), "nil environment") {
		t.Fatalf("nil catalog reference error = %v", err)
	}
	env := NewEnvironment()
	if _, err := WithNestedPropertySchemaFrom(env, "child", "Missing"); err == nil || !strings.Contains(err.Error(), "unregistered") {
		t.Fatalf("missing catalog reference error = %v", err)
	}
	if _, err := WithNestedPropertySchemaFrom(env, "", "Child"); err == nil || !strings.Contains(err.Error(), "property name") {
		t.Fatalf("empty property reference error = %v", err)
	}
	if _, err := WithNestedPropertySchemaFrom(env, "child", ""); err == nil || !strings.Contains(err.Error(), "schema name") {
		t.Fatalf("empty schema reference error = %v", err)
	}

	first := NewEnvironment()
	if _, err := RegisterJSON(first, "Child", []FieldSpec{FieldDef("value", reflect.TypeOf(""))}); err != nil {
		t.Fatal(err)
	}
	second := NewEnvironment()
	if _, err := WithNestedPropertySchemaFrom(second, "child", "Child"); err == nil || !strings.Contains(err.Error(), "unregistered") {
		t.Fatalf("cross-environment reference error = %v", err)
	}
}

func TestJSONNamedNestedMapSchemaCatalogSupportsCrossRepresentationFragments(t *testing.T) {
	env := NewEnvironment()
	stringType := reflect.TypeOf("")
	names, err := RegisterMap(env, "Names", []FieldSpec{
		FieldDef("first", stringType),
		FieldDef("last", stringType),
	})
	if err != nil {
		t.Fatal(err)
	}
	namesOption, err := WithNestedPropertySchemaFrom(env, "author", "Names")
	if err != nil {
		t.Fatal(err)
	}
	book, err := RegisterJSON(env, "Book", []FieldSpec{
		FieldDef("author", reflect.TypeOf(map[string]any{})),
	}, namesOption)
	if err != nil {
		t.Fatal(err)
	}
	if nested, ok := book.NestedSchema("author"); !ok || nested.Name() != names.Name() || nested.Kind() != SchemaMap {
		t.Fatalf("cross-representation nested schema = %q/%v, ok=%t", nested.Name(), nested.Kind(), ok)
	}
	event, err := ParseJSON(book, []byte(`{"author":{"first":"Jane","last":"Doe"}}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	fragment, ok := event.GetFragment("author")
	if !ok || fragment.Schema().Kind() != SchemaMap || fragment.Get("last").Any() != "Doe" {
		t.Fatalf("cross-representation fragment = %#v/%v, ok=%t", fragment.Underlying(), fragment.Schema().Kind(), ok)
	}
	if rendered, err := RenderJSON(event); err != nil || rendered != `{"author":{"first":"Jane","last":"Doe"}}` {
		t.Fatalf("cross-representation render = %s (%v)", rendered, err)
	}
}
