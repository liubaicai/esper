package esper

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"
)

type dataflowBeaconFieldEvent struct {
	P0 string  `esper:"p0"`
	P1 int64   `esper:"p1"`
	P2 float64 `esper:"p2"`
}

type dataflowBeaconSetterBean struct {
	myfield string
}

func (event dataflowBeaconSetterBean) GetMyfield() string { return event.myfield }
func (event *dataflowBeaconSetterBean) SetMyfield(value string) {
	event.myfield = value
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

func TestDataflowBeaconJavaBeanSetterMaterializationMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dataflowBeaconSetterBean](env, "BeaconSetterBean", WithAccessorStyle(AccessorJavaBean)); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "beacon-setter-bean").
		BeaconEventSourceWithUnderlying("source", "BeaconSetterBean", DataflowBeaconOptions{Iterations: 1},
			Alias("myfield", Literal("abc")),
		).
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
	if len(outputs) != 1 {
		t.Fatalf("setter beacon outputs = %#v", outputs)
	}
	bean, ok := outputs[0].(dataflowBeaconSetterBean)
	if !ok || bean.GetMyfield() != "abc" {
		t.Fatalf("setter beacon underlying = %#v", outputs[0])
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

func TestDataflowBeaconFieldParametersOverridePerInstanceAndRouteEventBusMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dataflowBeaconFieldEvent](env, "BeaconParameterizedEvent"); err != nil {
		t.Fatal(err)
	}
	parameterNames := []string{" p0 ", "p1"}
	definition, err := DefineDataflow(env, "beacon-field-parameters").
		BeaconEventSourceWithUnderlying("source", "BeaconParameterizedEvent", DataflowBeaconOptions{
			Iterations:      1,
			FieldParameters: parameterNames,
		},
			Alias("p0", Literal("default")),
			Alias("p1", Literal(int64(10))),
			Alias("p2", Literal(float64(1))),
		).
		EventBusSink("sink", "BeaconParameterizedEvent").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	parameterNames[0] = "missing"
	operators := definition.Operators()
	if got := operators[0].BeaconOptions.FieldParameters; !reflect.DeepEqual(got, []string{"p0", "p1"}) {
		t.Fatalf("beacon field parameter copy = %#v", got)
	}

	consumerPlan, err := env.Build(From[dataflowBeaconFieldEvent](env, "BeaconParameterizedEvent").Query(StatementName("beacon-parameter-consumer")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	consumer, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var received []dataflowBeaconFieldEvent
	if _, err := consumer.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				continue
			}
			switch value := event.Underlying().(type) {
			case dataflowBeaconFieldEvent:
				received = append(received, value)
			case *dataflowBeaconFieldEvent:
				received = append(received, *value)
			default:
				return fmt.Errorf("parameterized event underlying = %T", event.Underlying())
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var contexts []DataflowParameterContext
	run := func(instanceID, p0 string, p1 any, provideP1 bool) {
		t.Helper()
		instance, instantiateErr := engine.InstantiateDataflowWithOptions(context.Background(), definition, DataflowOptions{
			InstanceID: instanceID,
			ParameterProvider: func(parameter DataflowParameterContext) (any, bool) {
				contexts = append(contexts, parameter)
				switch parameter.ParameterName {
				case "p0":
					return p0, true
				case "p1":
					return p1, provideP1
				default:
					return nil, false
				}
			},
		})
		if instantiateErr != nil {
			t.Fatal(instantiateErr)
		}
		if runErr := instance.Run(context.Background()); runErr != nil {
			t.Fatal(runErr)
		}
	}
	run("beacon-parameter-one", "E1", nil, false)
	run("beacon-parameter-two", "E2", int64(20), true)

	want := []dataflowBeaconFieldEvent{
		{P0: "E1", P1: 10, P2: 1},
		{P0: "E2", P1: 20, P2: 1},
	}
	if !reflect.DeepEqual(received, want) {
		t.Fatalf("parameterized beacon events = %#v, want %#v", received, want)
	}
	if len(contexts) != 10 {
		t.Fatalf("beacon parameter contexts = %#v", contexts)
	}
	wantParameterNames := []string{"initialDelay", "interval", "iterations", "p0", "p1"}
	for index, parameter := range contexts {
		if parameter.OperatorName != "source" || parameter.OperatorNum != 0 {
			t.Fatalf("beacon parameter context[%d] = %#v", index, parameter)
		}
		if parameter.Factory.Kind != BeaconSourceKind || !parameter.Factory.IsBuiltin() || parameter.Factory.OperatorFactory != nil || parameter.Factory.SourceFactory != nil {
			t.Fatalf("beacon parameter factory context[%d] = %#v", index, parameter.Factory)
		}
		if parameter.ParameterName != wantParameterNames[index%len(wantParameterNames)] {
			t.Fatalf("beacon parameter order[%d] = %#v", index, parameter)
		}
		switch parameter.ParameterName {
		case DataflowBeaconInitialDelayParameter, DataflowBeaconIntervalParameter:
			if value, ok := parameter.DefaultValue.(time.Duration); !ok || value != 0 {
				t.Fatalf("beacon duration default[%d] = %#v", index, parameter.DefaultValue)
			}
		case DataflowBeaconIterationsParameter:
			if parameter.DefaultValue != 1 {
				t.Fatalf("beacon iterations default[%d] = %#v", index, parameter.DefaultValue)
			}
		default:
			if parameter.DefaultValue != nil {
				t.Fatalf("beacon field default[%d] = %#v", index, parameter.DefaultValue)
			}
		}
	}
	if len(definition.Operators()[0].Properties) != 0 {
		t.Fatalf("definition was mutated by instance parameters: %#v", definition.Operators()[0].Properties)
	}
}

func TestDataflowBeaconFieldParametersHaveStablePlanIdentity(t *testing.T) {
	build := func(parameters []string) Plan {
		t.Helper()
		env := NewEnvironment()
		if _, err := RegisterStruct[dataflowBeaconFieldEvent](env, "BeaconParameterPlanEvent"); err != nil {
			t.Fatal(err)
		}
		if _, err := DefineDataflow(env, "beacon-parameter-plan").
			BeaconEventSource("source", "BeaconParameterPlanEvent", DataflowBeaconOptions{
				Iterations:      1,
				FieldParameters: parameters,
			}).
			Emitter("sink").
			Connect("source", "sink").
			Build(); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(From[dataflowBeaconFieldEvent](env, "BeaconParameterPlanEvent").Query())
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	forward := build([]string{"p0", "p1"})
	reversed := build([]string{"p1", "p0"})
	different := build([]string{"p0"})
	if forward.Hash() != reversed.Hash() {
		t.Fatalf("field parameter order changed plan identity: %s != %s", forward.Hash(), reversed.Hash())
	}
	if forward.Hash() == different.Hash() {
		t.Fatalf("different field parameters share plan identity %s", forward.Hash())
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
	if _, err := DefineDataflow(env, "beacon-untyped-field-parameter").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{Iterations: 1, FieldParameters: []string{"p0"}}).
		Build(); err == nil {
		t.Fatal("untyped beacon accepted a field parameter")
	}
	if _, err := DefineDataflow(env, "beacon-unknown-field-parameter").
		BeaconEventSource("source", "BeaconValidation", DataflowBeaconOptions{Iterations: 1, FieldParameters: []string{"missing"}}).
		Build(); err == nil {
		t.Fatal("typed beacon accepted an unknown field parameter")
	}
	if _, err := DefineDataflow(env, "beacon-duplicate-field-parameter").
		BeaconEventSource("source", "BeaconValidation", DataflowBeaconOptions{Iterations: 1, FieldParameters: []string{"p0", " p0 "}}).
		Build(); err == nil {
		t.Fatal("typed beacon accepted duplicate field parameters")
	}
	parameterized, err := DefineDataflow(env, "beacon-invalid-field-parameter-value").
		BeaconEventSource("source", "BeaconValidation", DataflowBeaconOptions{Iterations: 1, FieldParameters: []string{"p1"}}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewEngine(env).InstantiateDataflowWithOptions(context.Background(), parameterized, DataflowOptions{
		ParameterProvider: func(parameter DataflowParameterContext) (any, bool) {
			if parameter.ParameterName == "p1" {
				return "wrong", true
			}
			return nil, false
		},
	}); err == nil {
		t.Fatal("typed beacon accepted an incompatible field parameter value")
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
	if _, err := RegisterMap(env, "BeaconReservedField", []FieldSpec{FieldDef("interval", reflect.TypeOf(time.Duration(0)))}); err != nil {
		t.Fatal(err)
	}
	if _, err := DefineDataflow(env, "beacon-reserved-field").
		BeaconEventSource("source", "BeaconReservedField", DataflowBeaconOptions{Iterations: 1}, Alias("interval", Literal(time.Second))).
		Build(); err == nil {
		t.Fatal("typed beacon accepted a reserved timing parameter as an event field")
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
	case SchemaAvro:
		value, ok := output.(*AvroRecord)
		if !ok || value.Get("p0") != "abc" || value.Get("p1") != int64(7) || value.Get("p2") != float64(1) {
			t.Fatalf("typed beacon Avro record = %#v", output)
		}
	default:
		value, ok := output.(map[string]any)
		if !ok || value["p0"] != "abc" || value["p1"] != int64(7) || value["p2"] != float64(1) {
			t.Fatalf("typed beacon map-like value = %#v", output)
		}
	}
}
