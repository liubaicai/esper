package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type dataflowRepresentationEvent struct {
	MyString string `esper:"myString"`
	MyInt    int    `esper:"myInt"`
}

// TestDataflowSelectConsumesAllRegisteredRepresentationsMatchesEsper mirrors
// EPLDataflowOpSelect.EPLDataflowAllTypes. Each representation uses the same
// fluent Select aggregate so field lookup and state accumulation are checked
// through the Dataflow graph rather than only at the schema unit boundary.
func TestDataflowSelectConsumesAllRegisteredRepresentationsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	fields := []FieldSpec{
		FieldDef("myString", reflect.TypeOf("")),
		FieldDef("myInt", reflect.TypeOf(int(0))),
	}
	structSchema, err := RegisterStruct[dataflowRepresentationEvent](env, "RepresentationStruct")
	if err != nil {
		t.Fatal(err)
	}
	mapSchema, err := RegisterMap(env, "RepresentationMap", fields)
	if err != nil {
		t.Fatal(err)
	}
	objectArraySchema, err := RegisterObjectArray(env, "RepresentationObjectArray", fields)
	if err != nil {
		t.Fatal(err)
	}
	xmlSchema, err := RegisterXML(env, "RepresentationXML", fields)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Unix(0, 0).UTC()
	structOne, err := newEvent(structSchema, dataflowRepresentationEvent{MyString: "one", MyInt: 1}, now)
	if err != nil {
		t.Fatal(err)
	}
	structTwo, err := newEvent(structSchema, dataflowRepresentationEvent{MyString: "two", MyInt: 2}, now)
	if err != nil {
		t.Fatal(err)
	}
	mapOne, err := newEvent(mapSchema, map[string]any{"myString": "one", "myInt": 1}, now)
	if err != nil {
		t.Fatal(err)
	}
	mapTwo, err := newEvent(mapSchema, map[string]any{"myString": "two", "myInt": 2}, now)
	if err != nil {
		t.Fatal(err)
	}
	objectArrayOne, err := ParseObjectArray(objectArraySchema, []any{"one", 1}, now)
	if err != nil {
		t.Fatal(err)
	}
	objectArrayTwo, err := ParseObjectArray(objectArraySchema, []any{"two", 2}, now)
	if err != nil {
		t.Fatal(err)
	}
	xmlOne, err := ParseXML(xmlSchema, []byte(`<event><myString>one</myString><myInt>1</myInt></event>`), now)
	if err != nil {
		t.Fatal(err)
	}
	xmlTwo, err := ParseXML(xmlSchema, []byte(`<event><myString>two</myString><myInt>2</myInt></event>`), now)
	if err != nil {
		t.Fatal(err)
	}

	runDataflowRepresentationPair(t, env, "struct", structOne, structTwo)
	runDataflowRepresentationPair(t, env, "map", mapOne, mapTwo)
	runDataflowRepresentationPair(t, env, "object-array", objectArrayOne, objectArrayTwo)
	runDataflowRepresentationPair(t, env, "xml", xmlOne, xmlTwo)
}

func TestDataflowEventBusSourceConsumesAllRegisteredRepresentationsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	fields := []FieldSpec{
		FieldDef("myString", reflect.TypeOf("")),
		FieldDef("myInt", reflect.TypeOf(int(0))),
	}
	if _, err := RegisterStruct[dataflowRepresentationEvent](env, "BusRepresentationStruct"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "BusRepresentationMap", fields); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterObjectArray(env, "BusRepresentationObjectArray", fields); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterXML(env, "BusRepresentationXML", fields); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	cases := []struct {
		name      string
		eventType string
		send      func() error
	}{
		{
			name:      "struct",
			eventType: "BusRepresentationStruct",
			send: func() error {
				return engine.SendEvent(context.Background(), dataflowRepresentationEvent{MyString: "one", MyInt: 1})
			},
		},
		{
			name:      "map",
			eventType: "BusRepresentationMap",
			send: func() error {
				return engine.SendRecord(context.Background(), "BusRepresentationMap", map[string]any{"myString": "one", "myInt": 1})
			},
		},
		{
			name:      "object-array",
			eventType: "BusRepresentationObjectArray",
			send: func() error {
				return engine.SendObjectArray(context.Background(), "BusRepresentationObjectArray", []any{"one", 1})
			},
		},
		{
			name:      "xml",
			eventType: "BusRepresentationXML",
			send: func() error {
				return engine.SendXML(context.Background(), "BusRepresentationXML", []byte(`<event><myString>one</myString><myInt>1</myInt></event>`))
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			definition, err := DefineDataflow(env, "dataflow-bus-representation-"+testCase.name).
				EventBusSource("source", testCase.eventType).
				Select("select", Alias("myString", Field[any, string]("myString")), Alias("myInt", Field[any, int]("myInt"))).
				Emitter("sink").
				Connect("source", "select").
				Connect("select", "sink").
				Build()
			if err != nil {
				t.Fatal(err)
			}
			instance, err := engine.InstantiateDataflow(context.Background(), definition)
			if err != nil {
				t.Fatal(err)
			}
			if err := instance.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			defer instance.Cancel(context.Background())
			if err := testCase.send(); err != nil {
				t.Fatal(err)
			}
			outputs := instance.Outputs()
			if len(outputs) != 1 {
				t.Fatalf("event-bus %s outputs = %#v, want one row", testCase.name, outputs)
			}
			row, ok := outputs[0].(Row)
			if !ok || row.Get("myString").Any() != "one" || row.Get("myInt").Any() != 1 {
				t.Fatalf("event-bus %s row = %#v", testCase.name, outputs[0])
			}
		})
	}
}

func runDataflowRepresentationPair(t *testing.T, env *Environment, name string, first, second Event) {
	t.Helper()
	definition, err := DefineDataflow(env, "dataflow-representation-"+name).
		BeaconSource("source", first, second).
		Select("select",
			Alias("myString", Field[any, string]("myString")),
			Alias("total", Sum[int](Field[any, int]("myInt"))),
		).
		Emitter("sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if instance.State() != DataflowComplete {
		t.Fatalf("dataflow %q state = %v, want complete", name, instance.State())
	}
	outputs := instance.Outputs()
	if len(outputs) != 2 {
		t.Fatalf("dataflow %q outputs = %#v, want two rows", name, outputs)
	}
	assertDataflowRepresentationRow(t, outputs, 0, "one", 1)
	assertDataflowRepresentationRow(t, outputs, 1, "two", 3)
}

func assertDataflowRepresentationRow(t *testing.T, outputs []any, index int, wantString string, wantTotal int) {
	t.Helper()
	if index < 0 || index >= len(outputs) {
		t.Fatalf("representation output index %d missing in %#v", index, outputs)
	}
	row, ok := outputs[index].(Row)
	if !ok {
		t.Fatalf("representation output %d = %#v, want Row", index, outputs[index])
	}
	if got := row.Get("myString").Any(); got != wantString {
		t.Fatalf("representation myString[%d] = %#v, want %q", index, got, wantString)
	}
	if got := row.Get("total").Any(); got != wantTotal {
		t.Fatalf("representation total[%d] = %#v, want %d", index, got, wantTotal)
	}
}
