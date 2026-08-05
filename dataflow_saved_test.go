package esper

import (
	"context"
	"errors"
	"testing"
)

func TestDataflowSavedConfigurationAndInstanceMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	event, err := newEvent(schema, runtimeTestTrade{Symbol: "saved", Price: 12}, engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "saved-flow").
		BeaconSource("source", event).
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	if names := env.SavedDataflowConfigurations(); len(names) != 0 {
		t.Fatalf("initial saved configurations = %#v", names)
	}
	if _, ok := engine.LoadDataflowInstance("missing"); ok {
		t.Fatal("missing saved instance was found")
	}
	if err := engine.DeleteDataflowInstance("missing"); !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("delete missing instance error = %v", err)
	}
	if err := env.SaveDataflowConfigurationAs("MyFirstFlow", definition.Name()); err != nil {
		t.Fatal(err)
	}
	if names := env.SavedDataflowConfigurations(); len(names) != 1 || names[0] != "MyFirstFlow" {
		t.Fatalf("saved configuration names = %#v", names)
	}
	loaded, ok := env.LoadDataflowConfiguration("MyFirstFlow")
	if !ok || loaded.Name() != "saved-flow" {
		t.Fatalf("loaded configuration = %#v, ok=%v", loaded, ok)
	}
	if err := env.SaveDataflowConfigurationAs("MyFirstFlow", definition.Name()); !errors.Is(err, ErrorDependency) {
		t.Fatalf("duplicate configuration error = %v", err)
	}

	instance, err := engine.InstantiateSavedDataflow(context.Background(), "MyFirstFlow")
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if instance.DataflowName() != "saved-flow" {
		t.Fatalf("saved instance dataflow name = %q", instance.DataflowName())
	}
	if err := engine.SaveDataflowInstance("F1", instance); err != nil {
		t.Fatal(err)
	}
	if names := engine.SavedDataflowInstances(); len(names) != 1 || names[0] != "F1" {
		t.Fatalf("saved instance names = %#v", names)
	}
	loadedInstance, ok := engine.LoadDataflowInstance("F1")
	if !ok || loadedInstance != instance {
		t.Fatalf("loaded instance = %p/%v, want %p/true", loadedInstance, ok, instance)
	}
	if err := engine.SaveDataflowInstance("F1", instance); !errors.Is(err, ErrorDependency) {
		t.Fatalf("duplicate instance error = %v", err)
	}
	if err := engine.DeleteDataflowInstance("F1"); err != nil {
		t.Fatal(err)
	}
	if names := engine.SavedDataflowInstances(); len(names) != 0 {
		t.Fatalf("saved instance names after delete = %#v", names)
	}
	if err := env.DeleteDataflowConfiguration("MyFirstFlow"); err != nil {
		t.Fatal(err)
	}
}
