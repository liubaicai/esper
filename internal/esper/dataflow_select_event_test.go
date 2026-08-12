package esper

import (
	"context"
	"reflect"
	"testing"
)

func TestDataflowSelectPassThroughPreservesEventMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	fields := []FieldSpec{FieldDef("value", reflect.TypeOf(int(0)))}
	if _, err := RegisterMap(env, "WrapperInput", fields); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "dataflow-select-pass-through").
		EventBusSource("source", "WrapperInput").
		SelectPassThrough("select").
		Emitter("sink").
		Connect("source", "select").
		Connect("select", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	assertDataflowPortType(t, definition, "select", true, "out", dataflowRecordType())

	engine := NewEngine(env)
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer instance.Cancel(context.Background())
	if err := engine.SendRecord(context.Background(), "WrapperInput", map[string]any{"value": 10}); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 1 {
		t.Fatalf("pass-through outputs = %#v, want one event", outputs)
	}
	event, ok := outputs[0].(Event)
	if !ok || event.TypeName() != "WrapperInput" || event.Get("value").Any() != 10 {
		t.Fatalf("pass-through output = %#v, want original Event", outputs[0])
	}
}

func TestDataflowSelectEventProjectionPreservesInputAndAddsPropertiesMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	inputFields := []FieldSpec{FieldDef("value", reflect.TypeOf(int(0)))}
	outputFields := []FieldSpec{
		FieldDef("value", reflect.TypeOf(int(0))),
		FieldDef("hello", reflect.TypeOf("")),
	}
	if _, err := RegisterMap(env, "WrapperInput", inputFields); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "WrapperOutput", outputFields); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "dataflow-select-event").
		EventBusSource("source", "WrapperInput").
		SelectEvent("select", "WrapperOutput", Alias("hello", Literal("a"))).
		Emitter("sink").
		Connect("source", "select").
		Connect("select", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	assertDataflowPortType(t, definition, "select", true, "out", reflect.TypeOf(Event{}))

	engine := NewEngine(env)
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer instance.Cancel(context.Background())
	if err := engine.SendRecord(context.Background(), "WrapperInput", map[string]any{"value": 10}); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 1 {
		t.Fatalf("event projection outputs = %#v, want one event", outputs)
	}
	event, ok := outputs[0].(Event)
	if !ok {
		t.Fatalf("event projection output = %#v, want Event", outputs[0])
	}
	if event.TypeName() != "WrapperOutput" || event.Get("value").Any() != 10 || event.Get("hello").Any() != "a" {
		t.Fatalf("event projection output = %#v, want value=10 hello=a", event)
	}
	underlying, ok := event.Underlying().(map[string]any)
	if !ok || underlying["value"] != 10 || underlying["hello"] != "a" {
		t.Fatalf("event projection underlying = %#v, want retained and added properties", event.Underlying())
	}
}

func TestDataflowSelectEventStarPreservesEventMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	fields := []FieldSpec{FieldDef("value", reflect.TypeOf(int(0)))}
	if _, err := RegisterMap(env, "WrapperStar", fields); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "dataflow-select-event-star").
		EventBusSource("source", "WrapperStar").
		SelectEvent("select", "WrapperStar").
		Emitter("sink").
		Connect("source", "select").
		Connect("select", "sink").
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
	defer instance.Cancel(context.Background())
	if err := engine.SendRecord(context.Background(), "WrapperStar", map[string]any{"value": 10}); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 1 {
		t.Fatalf("event-star outputs = %#v, want one event", outputs)
	}
	event, ok := outputs[0].(Event)
	if !ok || event.TypeName() != "WrapperStar" || event.Get("value").Any() != 10 {
		t.Fatalf("event-star output = %#v, want typed select-star Event", outputs[0])
	}
}

func TestDataflowSelectEventRejectsUnknownOutputAndInvalidPassThrough(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "WrapperInput", []FieldSpec{FieldDef("value", reflect.TypeOf(int(0)))}); err != nil {
		t.Fatal(err)
	}
	if _, err := DefineDataflow(env, "dataflow-select-event-unknown").
		EventBusSource("source", "WrapperInput").
		SelectEvent("select", "MissingOutput").
		Connect("source", "select").
		Build(); err == nil {
		t.Fatal("SelectEvent accepted an unknown output event type")
	}
	if _, err := DefineDataflow(env, "dataflow-select-pass-through-projection").
		EventBusSource("source", "WrapperInput").
		SelectWithOptions("select", DataflowSelectOptions{PreserveInput: true}, Alias("value", Field[any, int]("value"))).
		Connect("source", "select").
		Build(); err == nil {
		t.Fatal("pass-through accepted a projection without an output event type")
	}
}
