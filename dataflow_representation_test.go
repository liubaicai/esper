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

// TestDataflowEventBusSinkSendsAllRegisteredRepresentationsMatchesEsper
// mirrors EPLDataflowOpEventBusSink.EPLDataflowAllTypes. Java's graph source
// emits native POJO/Map/ObjectArray/XML values, while EventBusSink performs
// the event materialization at its typed input boundary.
func TestDataflowEventBusSinkSendsAllRegisteredRepresentationsMatchesEsper(t *testing.T) {
	fields := []FieldSpec{
		FieldDef("myString", reflect.TypeOf("")),
		FieldDef("myInt", reflect.TypeOf(int(0))),
	}
	cases := []struct {
		name      string
		eventType string
		register  func(*Environment, string, []FieldSpec) (Schema, error)
		value     func(Schema) any
		assert    func(*testing.T, any)
	}{
		{
			name:      "struct",
			eventType: "SinkRepresentationStruct",
			register: func(env *Environment, name string, _ []FieldSpec) (Schema, error) {
				return RegisterStruct[dataflowRepresentationEvent](env, name)
			},
			value: func(Schema) any {
				return dataflowRepresentationEvent{MyString: "one", MyInt: 1}
			},
			assert: func(t *testing.T, value any) {
				if value != (dataflowRepresentationEvent{MyString: "one", MyInt: 1}) {
					t.Fatalf("sink struct underlying = %#v", value)
				}
			},
		},
		{
			name:      "map",
			eventType: "SinkRepresentationMap",
			register: func(env *Environment, name string, fields []FieldSpec) (Schema, error) {
				return RegisterMap(env, name, fields)
			},
			value: func(Schema) any {
				return map[string]any{"myString": "one", "myInt": 1}
			},
			assert: func(t *testing.T, value any) {
				got, ok := value.(map[string]any)
				if !ok || got["myString"] != "one" || got["myInt"] != 1 {
					t.Fatalf("sink map underlying = %#v", value)
				}
			},
		},
		{
			name:      "object-array",
			eventType: "SinkRepresentationObjectArray",
			register: func(env *Environment, name string, fields []FieldSpec) (Schema, error) {
				return RegisterObjectArray(env, name, fields)
			},
			value: func(Schema) any {
				return []any{"one", 1}
			},
			assert: func(t *testing.T, value any) {
				got, ok := value.([]any)
				if !ok || len(got) != 2 || got[0] != "one" || got[1] != 1 {
					t.Fatalf("sink object-array underlying = %#v", value)
				}
			},
		},
		{
			name:      "xml",
			eventType: "SinkRepresentationXML",
			register: func(env *Environment, name string, fields []FieldSpec) (Schema, error) {
				return RegisterXML(env, name, fields)
			},
			value: func(schema Schema) any {
				event, err := ParseXML(schema, []byte(`<event><myString>one</myString><myInt>1</myInt></event>`), time.Unix(0, 0).UTC())
				if err != nil {
					panic(err)
				}
				return event.Underlying()
			},
			assert: func(t *testing.T, value any) {
				got, ok := value.(map[string]any)
				if !ok || got["myString"] != "one" || got["myInt"] != 1 {
					t.Fatalf("sink XML underlying = %#v", value)
				}
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env := NewEnvironment()
			schema, err := testCase.register(env, testCase.eventType, fields)
			if err != nil {
				t.Fatal(err)
			}
			value := testCase.value(schema)
			engine := NewEngine(env)
			plan, err := env.Build(FromAny(env, testCase.eventType).Query(StatementName("sink-representation-" + testCase.name)))
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			defer deployment.Undeploy(context.Background())
			var received []Event
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, result := range batch.New {
					if event, ok := result.Event(); ok {
						received = append(received, event)
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			definition, err := DefineDataflow(env, "dataflow-sink-representation-"+testCase.name).
				BeaconSource("source", value).
				EventBusSink("sink", testCase.eventType).
				Connect("source", "sink").
				Build()
			if err != nil {
				t.Fatal(err)
			}
			instance, err := engine.InstantiateDataflow(context.Background(), definition)
			if err != nil {
				t.Fatal(err)
			}
			if err := instance.Run(context.Background()); err != nil {
				t.Fatal(err)
			}
			if len(received) != 1 {
				t.Fatalf("sink %s received %d events, want one", testCase.name, len(received))
			}
			testCase.assert(t, received[0].Underlying())
		})
	}
}

func TestDataflowEventBusSourceWithUnderlyingAllRegisteredRepresentationsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	fields := []FieldSpec{
		FieldDef("myString", reflect.TypeOf("")),
		FieldDef("myInt", reflect.TypeOf(int(0))),
	}
	if _, err := RegisterStruct[dataflowRepresentationEvent](env, "UnderlyingRepresentationStruct"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "UnderlyingRepresentationMap", fields); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterObjectArray(env, "UnderlyingRepresentationObjectArray", fields); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterXML(env, "UnderlyingRepresentationXML", fields); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name      string
		eventType string
		send      func(*Engine) error
		assert    func(*testing.T, any)
	}{
		{
			name:      "struct",
			eventType: "UnderlyingRepresentationStruct",
			send: func(engine *Engine) error {
				return engine.SendEvent(context.Background(), dataflowRepresentationEvent{MyString: "one", MyInt: 1})
			},
			assert: func(t *testing.T, value any) {
				if value != (dataflowRepresentationEvent{MyString: "one", MyInt: 1}) {
					t.Fatalf("underlying struct = %#v", value)
				}
			},
		},
		{
			name:      "map",
			eventType: "UnderlyingRepresentationMap",
			send: func(engine *Engine) error {
				return engine.SendRecord(context.Background(), "UnderlyingRepresentationMap", map[string]any{"myString": "one", "myInt": 1})
			},
			assert: func(t *testing.T, value any) {
				got, ok := value.(map[string]any)
				if !ok || got["myString"] != "one" || got["myInt"] != 1 {
					t.Fatalf("underlying map = %#v", value)
				}
			},
		},
		{
			name:      "object-array",
			eventType: "UnderlyingRepresentationObjectArray",
			send: func(engine *Engine) error {
				return engine.SendObjectArray(context.Background(), "UnderlyingRepresentationObjectArray", []any{"one", 1})
			},
			assert: func(t *testing.T, value any) {
				got, ok := value.([]any)
				if !ok || len(got) != 2 || got[0] != "one" || got[1] != 1 {
					t.Fatalf("underlying object-array = %#v", value)
				}
			},
		},
		{
			name:      "xml",
			eventType: "UnderlyingRepresentationXML",
			send: func(engine *Engine) error {
				return engine.SendXML(context.Background(), "UnderlyingRepresentationXML", []byte(`<event><myString>one</myString><myInt>1</myInt></event>`))
			},
			assert: func(t *testing.T, value any) {
				got, ok := value.(map[string]any)
				if !ok || got["myString"] != "one" || got["myInt"] != 1 {
					t.Fatalf("underlying XML = %#v", value)
				}
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			definition, err := DefineDataflow(env, "dataflow-underlying-representation-"+testCase.name).
				EventBusSourceWithUnderlying("source", testCase.eventType).
				Emitter("sink").
				Connect("source", "sink").
				Build()
			if err != nil {
				t.Fatal(err)
			}
			instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
			if err != nil {
				t.Fatal(err)
			}
			if err := instance.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			engine := instance.engine
			if err := testCase.send(engine); err != nil {
				t.Fatal(err)
			}
			outputs := instance.Outputs()
			if len(outputs) != 1 {
				t.Fatalf("underlying %s outputs = %#v, want one", testCase.name, outputs)
			}
			testCase.assert(t, outputs[0])
			if err := instance.Cancel(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}
