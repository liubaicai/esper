package esper

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

type dataflowBeaconFieldEvent struct {
	P0 string  `esper:"p0"`
	P1 int64   `esper:"p1"`
	P2 float64 `esper:"p2"`
}

func TestDataflowBeaconEventFieldsMaterializeAllRepresentationsMatchesEsper(t *testing.T) {
	representations := []struct {
		name string
		kind SchemaKind
	}{
		{name: "struct", kind: SchemaStruct},
		{name: "map", kind: SchemaMap},
		{name: "object-array", kind: SchemaObjectArray},
		{name: "json", kind: SchemaJSON},
		{name: "xml", kind: SchemaXML},
		{name: "avro", kind: SchemaAvro},
	}
	modes := []struct {
		name       string
		underlying bool
	}{
		{name: "event"},
		{name: "underlying", underlying: true},
	}
	for _, representation := range representations {
		for _, mode := range modes {
			t.Run(representation.name+"-"+mode.name, func(t *testing.T) {
				env := NewEnvironment()
				eventType := "BeaconField_" + representation.name + "_" + mode.name
				if err := registerDataflowBeaconRepresentation(env, eventType, representation.kind); err != nil {
					t.Fatal(err)
				}
				if err := env.RegisterVariable("beacon_p1", int64(7)); err != nil {
					t.Fatal(err)
				}
				builder := DefineDataflow(env, "beacon-field-"+representation.name+"-"+mode.name)
				fields := []Selection{
					Alias("p0", Literal("abc")),
					Alias("p1", VariableRef[int64]("beacon_p1")),
					Alias("p2", Literal(float64(1))),
				}
				if mode.underlying {
					builder = builder.BeaconEventSourceWithUnderlying("source", eventType, DataflowBeaconOptions{Iterations: 2}, fields...)
				} else {
					builder = builder.BeaconEventSource("source", eventType, DataflowBeaconOptions{Iterations: 2}, fields...)
				}
				definition, err := builder.Emitter("sink").Connect("source", "sink").Build()
				if err != nil {
					t.Fatal(err)
				}
				operators := definition.Operators()
				if len(operators) == 0 {
					t.Fatal("typed beacon definition has no operators")
				}
				wantPortType := reflect.TypeOf(Event{})
				if mode.underlying {
					schema, _ := env.Schema(eventType)
					wantPortType = dataflowSchemaUnderlyingType(schema)
				}
				if got := operators[0].OutputPortTypes["out"]; got != wantPortType {
					t.Fatalf("typed beacon output port = %v, want %v", got, wantPortType)
				}
				instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
				if err != nil {
					t.Fatal(err)
				}
				if err := instance.Run(context.Background()); err != nil {
					t.Fatal(err)
				}
				outputs := instance.Outputs()
				if len(outputs) != 2 {
					t.Fatalf("typed beacon outputs = %#v", outputs)
				}
				for _, output := range outputs {
					if mode.underlying {
						assertDataflowBeaconUnderlying(t, output, representation.kind)
						continue
					}
					event, ok := output.(Event)
					if !ok || event.TypeName() != eventType || event.Get("p0").Any() != "abc" || event.Get("p1").Any() != int64(7) || event.Get("p2").Any() != float64(1) {
						t.Fatalf("typed beacon event = %#v", output)
					}
				}
			})
		}
	}
}

func TestDataflowBeaconEventFactoryMaterializesPerIterationBaseValues(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dataflowBeaconFieldEvent](env, "BeaconFactoryEvent"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "beacon-event-factory").
		BeaconEventSourceWithUnderlying("source", "BeaconFactoryEvent", DataflowBeaconOptions{
			Iterations: 3,
			Factory: func(_ context.Context, beacon DataflowBeaconContext) (any, error) {
				return map[string]any{
					"p0": fmt.Sprintf("E%d", beacon.Iteration),
					"p1": int64(beacon.Iteration),
					"p2": float64(beacon.Iteration) + 0.5,
				}, nil
			},
		}).
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
	if err := instance.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 3 {
		t.Fatalf("typed factory outputs = %#v", outputs)
	}
	for index, output := range outputs {
		value, ok := output.(dataflowBeaconFieldEvent)
		if !ok || value.P0 != fmt.Sprintf("E%d", index) || value.P1 != int64(index) || value.P2 != float64(index)+0.5 {
			t.Fatalf("typed factory output[%d] = %#v", index, output)
		}
	}
}

func TestDataflowBeaconIterationsExpressionUsesVariableAtInstantiationMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if err := env.RegisterVariable("var_iterations", int64(3)); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "beacon-variable-iterations").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{
			IterationsExpression: VariableRef[int64]("var_iterations"),
		}, "value").
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
	if err := instance.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if outputs := instance.Outputs(); len(outputs) != 3 {
		t.Fatalf("variable iteration outputs = %#v", outputs)
	}
	if signals := instance.Signals(); len(signals) != 1 || !isDataflowFinalMarker(signals[0]) {
		t.Fatalf("variable iteration signals = %#v", signals)
	}
}

func TestDataflowBeaconEventFieldsFlowIntoEventBusSinkMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dataflowBeaconFieldEvent](env, "BeaconBusEvent"); err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(From[dataflowBeaconFieldEvent](env, "BeaconBusEvent").Query(StatementName("beacon-bus-consumer")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	consumer, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var received []Event
	if _, err := consumer.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				received = append(received, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "beacon-event-bus").
		BeaconEventSourceWithUnderlying("source", "BeaconBusEvent", DataflowBeaconOptions{Iterations: 1},
			Alias("p0", Literal("E1")),
			Alias("p1", Literal(int64(10))),
			Alias("p2", Literal(float64(1))),
		).
		EventBusSink("sink", "BeaconBusEvent").
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
	if len(received) != 1 || received[0].Get("p0").Any() != "E1" || received[0].Get("p1").Any() != int64(10) {
		t.Fatalf("typed beacon event-bus results = %#v", received)
	}
}

func TestDataflowBeaconEventFieldsRejectInvalidDefinitions(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "BeaconValidation", []FieldSpec{
		FieldDef("p0", reflect.TypeOf("")),
		FieldDef("p1", reflect.TypeOf(int64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := DefineDataflow(env, "beacon-missing-type").
		BeaconEventSource("source", "MissingType", DataflowBeaconOptions{Iterations: 1}).
		Build(); err == nil {
		t.Fatal("typed beacon accepted an unknown event type")
	}
	if _, err := DefineDataflow(env, "beacon-empty-type").
		BeaconEventSource("source", "", DataflowBeaconOptions{Iterations: 1}).
		Build(); err == nil {
		t.Fatal("typed beacon accepted an empty event type")
	}
	if _, err := DefineDataflow(env, "beacon-missing-field").
		BeaconEventSource("source", "BeaconValidation", DataflowBeaconOptions{Iterations: 1}, Alias("missing", Literal("x"))).
		Build(); err == nil {
		t.Fatal("typed beacon accepted an unknown event field")
	}
	if _, err := DefineDataflow(env, "beacon-field-type").
		BeaconEventSource("source", "BeaconValidation", DataflowBeaconOptions{Iterations: 1}, Alias("p1", Literal("wrong"))).
		Build(); err == nil {
		t.Fatal("typed beacon accepted an incompatible field expression")
	}
	if _, err := DefineDataflow(env, "beacon-input-field").
		BeaconEventSource("source", "BeaconValidation", DataflowBeaconOptions{Iterations: 1}, Alias("p0", Field[Event, string]("p0"))).
		Build(); err == nil {
		t.Fatal("typed beacon accepted an input event field expression")
	}
	if _, err := DefineDataflow(env, "beacon-query-parameter").
		BeaconEventSource("source", "BeaconValidation", DataflowBeaconOptions{Iterations: 1}, Alias("p0", Parameter[string]("p0"))).
		Build(); err == nil {
		t.Fatal("typed beacon accepted an unbound query parameter")
	}
	if _, err := DefineDataflow(env, "beacon-ambiguous-iterations").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{Iterations: 1, IterationsExpression: Literal(int64(2))}).
		Build(); err == nil {
		t.Fatal("beacon accepted fixed and expression iteration counts together")
	}
	if _, err := DefineDataflow(env, "beacon-float-iterations").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{IterationsExpression: Literal(1.5)}).
		Build(); err == nil {
		t.Fatal("beacon accepted a non-integer iterations expression")
	}
	member, err := RegisterMap(env, "BeaconMember", []FieldSpec{FieldDef("p0", reflect.TypeOf(""))})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterVariant(env, "BeaconVariant", member); err != nil {
		t.Fatal(err)
	}
	if _, err := DefineDataflow(env, "beacon-variant").
		BeaconEventSource("source", "BeaconVariant", DataflowBeaconOptions{Iterations: 1}, Alias("p0", Literal("x"))).
		Build(); err == nil {
		t.Fatal("typed beacon accepted a variant target without member identity")
	}
}

func registerDataflowBeaconRepresentation(env *Environment, name string, kind SchemaKind) error {
	fields := []FieldSpec{
		FieldDef("p0", reflect.TypeOf("")),
		FieldDef("p1", reflect.TypeOf(int64(0))),
		FieldDef("p2", reflect.TypeOf(float64(0))),
	}
	switch kind {
	case SchemaStruct:
		_, err := RegisterStruct[dataflowBeaconFieldEvent](env, name)
		return err
	case SchemaMap:
		_, err := RegisterMap(env, name, fields)
		return err
	case SchemaObjectArray:
		_, err := RegisterObjectArray(env, name, fields)
		return err
	case SchemaJSON:
		_, err := RegisterJSON(env, name, fields)
		return err
	case SchemaXML:
		_, err := RegisterXML(env, name, fields)
		return err
	case SchemaAvro:
		_, err := RegisterAvro(env, name, fields)
		return err
	default:
		return fmt.Errorf("unsupported beacon representation %d", kind)
	}
}

func assertDataflowBeaconUnderlying(t *testing.T, output any, kind SchemaKind) {
	t.Helper()
	switch kind {
	case SchemaStruct:
		value, ok := output.(dataflowBeaconFieldEvent)
		if !ok || value != (dataflowBeaconFieldEvent{P0: "abc", P1: 7, P2: 1}) {
			t.Fatalf("typed beacon struct = %#v", output)
		}
	case SchemaObjectArray:
		value, ok := output.([]any)
		if !ok || len(value) != 3 || value[0] != "abc" || value[1] != int64(7) || value[2] != float64(1) {
			t.Fatalf("typed beacon object-array = %#v", output)
		}
	default:
		value, ok := output.(map[string]any)
		if !ok || value["p0"] != "abc" || value["p1"] != int64(7) || value["p2"] != float64(1) {
			t.Fatalf("typed beacon map-like value = %#v", output)
		}
	}
}
