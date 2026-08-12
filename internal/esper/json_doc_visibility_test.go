package esper

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

type jsonDocPerson struct {
	Name string `json:"name"`
	ID   UUID   `json:"id"`
}

func TestJSONDocSamplesNestedDynamicAndApplicationClassMatchEsper(t *testing.T) {
	stringType := reflect.TypeOf("")
	intType := reflect.TypeOf(int(0))

	car, err := NewJSONSchema("CarLocUpdateEvent", []FieldSpec{
		FieldDef("carId", stringType),
		FieldDef("direction", intType),
	})
	if err != nil {
		t.Fatal(err)
	}
	carEvent, err := ParseJSON(car, []byte(`{"carId":"A123456","direction":1}`), time.Unix(0, 0).UTC())
	if err != nil || carEvent.Get("carId").Any() != "A123456" || carEvent.Get("direction").Any() != 1 {
		t.Fatalf("car sample = %#v (%v)", carEvent.Underlying(), err)
	}

	names, err := NewJSONSchema("Names", []FieldSpec{FieldDef("lastname", stringType), FieldDef("firstname", stringType)})
	if err != nil {
		t.Fatal(err)
	}
	book, err := NewJSONSchema("BookEvent", []FieldSpec{
		FieldDef("isbn", stringType),
		FieldDef("author", reflect.TypeOf(map[string]any{})),
		FieldDef("editor", reflect.TypeOf(map[string]any{})),
		FieldDef("title", stringType),
		FieldDef("category", reflect.TypeOf([]string{})),
	}, WithNestedPropertySchema("author", names), WithNestedPropertySchema("editor", names))
	if err != nil {
		t.Fatal(err)
	}
	bookEvent, err := ParseJSON(book, []byte(`{"isbn":"123-456-222","author":{"lastname":"Doe","firstname":"Jane"},"editor":{"lastname":"Smith","firstname":"Jane"},"title":"The Ultimate Database Study Guide","category":["Non-Fiction","Technology"]}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if bookEvent.Get("author.lastname").Any() != "Doe" || bookEvent.Get("editor.lastname").Any() != "Smith" || bookEvent.Get("category[0]").Any() != "Non-Fiction" {
		t.Fatalf("book nested sample = %#v", bookEvent.Underlying())
	}

	idAndType, err := NewJSONSchema("IdAndType", []FieldSpec{FieldDef("id", stringType), FieldDef("type", stringType)})
	if err != nil {
		t.Fatal(err)
	}
	batters, err := NewJSONSchema("Batters", []FieldSpec{
		FieldDef("machine", stringType),
		FieldDef("batter", reflect.TypeOf([]map[string]any{})),
	}, WithNestedPropertySchema("batter", idAndType))
	if err != nil {
		t.Fatal(err)
	}
	cake, err := NewJSONSchema("CakeEvent", []FieldSpec{
		FieldDef("id", stringType),
		FieldDef("type", stringType),
		FieldDef("name", stringType),
		FieldDef("batters", reflect.TypeOf(map[string]any{})),
		FieldDef("topping", reflect.TypeOf([]map[string]any{})),
	}, WithNestedPropertySchema("batters", batters), WithNestedPropertySchema("topping", idAndType))
	if err != nil {
		t.Fatal(err)
	}
	cakeEvent, err := ParseJSON(cake, []byte(`{"id":"0001","type":"donut","name":"Cake","batters":{"machine":"machine A","batter":[{"id":"1001","type":"Regular"},{"id":"1002","type":"Chocolate"}]},"topping":[{"id":"5001","type":"None"},{"id":"5002","type":"Glazed"}]}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if cakeEvent.Get("batters.machine").Any() != "machine A" || cakeEvent.Get("batters.batter[0].type").Any() != "Regular" || cakeEvent.Get("topping[0].type").Any() != "None" {
		t.Fatalf("cake nested sample = %#v", cakeEvent.Underlying())
	}

	sensor, err := NewJSONSchema("SensorEvent", nil, AllowDynamicFields())
	if err != nil {
		t.Fatal(err)
	}
	sensorEvent, err := ParseJSON(sensor, []byte(`{"entityID":"cd9f930e","temperature":70,"status":true,"entityName":{"english":"Cooling Water Temperature"},"vt":["2014-08-20T15:30:23.524Z"],"flags":null}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if sensorEvent.Get("entityID").Any() != "cd9f930e" || sensorEvent.Get("temperature").Any() != 70 || sensorEvent.Get("status").Any() != true || sensorEvent.Get("entityName.english").Any() != "Cooling Water Temperature" || !sensorEvent.Get("flags").IsNull() {
		t.Fatalf("dynamic sample = %#v", sensorEvent.Underlying())
	}

	application, err := NewJSONSchema("JsonEvent", []FieldSpec{FieldDef("person", reflect.TypeOf(jsonDocPerson{}))})
	if err != nil {
		t.Fatal(err)
	}
	uuid, err := ParseUUID("123e4567-e89b-12d3-a456-426614174000")
	if err != nil {
		t.Fatal(err)
	}
	applicationEvent, err := ParseJSON(application, []byte(`{"person":{"name":"Joe","id":"123e4567-e89b-12d3-a456-426614174000"}}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	person, err := As[jsonDocPerson](applicationEvent.Get("person"))
	if err != nil || person.Name != "Joe" || person.ID != uuid {
		t.Fatalf("application class sample = %#v (%v)", person, err)
	}
}

func TestJSONVisibilityUsesExplicitEnvironmentCatalogBoundary(t *testing.T) {
	first := NewEnvironment()
	second := NewEnvironment()
	if _, err := RegisterJSON(first, "JsonSchema", []FieldSpec{FieldDef("fruit", reflect.TypeOf(""))}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterJSON(second, "JsonSchema", []FieldSpec{FieldDef("carId", reflect.TypeOf(""))}); err != nil {
		t.Fatal(err)
	}
	firstSchema, _ := first.Schema("JsonSchema")
	secondSchema, _ := second.Schema("JsonSchema")
	firstEvent, err := ParseJSON(firstSchema, []byte(`{"fruit":"Apple"}`), time.Unix(0, 0).UTC())
	if err != nil || firstEvent.Get("fruit").Any() != "Apple" {
		t.Fatalf("first catalog event = %#v (%v)", firstEvent.Underlying(), err)
	}
	secondEvent, err := ParseJSON(secondSchema, []byte(`{"carId":"E1"}`), time.Unix(0, 0).UTC())
	if err != nil || secondEvent.Get("carId").Any() != "E1" {
		t.Fatalf("second catalog event = %#v (%v)", secondEvent.Underlying(), err)
	}
	if _, err := RegisterJSON(first, "JsonSchema", []FieldSpec{FieldDef("size", reflect.TypeOf(""))}); err == nil || !strings.Contains(err.Error(), "An event type by name 'JsonSchema' has already been created for module 'unnamed'") {
		t.Fatalf("same-catalog duplicate schema error = %v", err)
	}
	// Java's public/protected module/path visibility is intentionally not
	// inferred here: an Environment is an explicit Go catalog boundary.
}
