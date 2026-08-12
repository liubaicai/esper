package esper

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type dataflowJoinProjectedEvent struct {
	LeftID  int `esper:"leftID"`
	RightID int `esper:"rightID"`
}

func TestDataflowSelectJoinEventMaterializesRegisteredRepresentations(t *testing.T) {
	fields := []FieldSpec{
		FieldDef("leftID", reflect.TypeOf(int(0))),
		FieldDef("rightID", reflect.TypeOf(int(0))),
	}
	tests := []struct {
		name       string
		kind       SchemaKind
		register   func(*Environment, string) (Schema, error)
		underlying func(any) bool
	}{
		{
			name: "struct",
			kind: SchemaStruct,
			register: func(env *Environment, name string) (Schema, error) {
				return RegisterStruct[dataflowJoinProjectedEvent](env, name)
			},
			underlying: func(value any) bool {
				return value == (dataflowJoinProjectedEvent{LeftID: 1, RightID: 10})
			},
		},
		{
			name: "map",
			kind: SchemaMap,
			register: func(env *Environment, name string) (Schema, error) {
				return RegisterMap(env, name, fields)
			},
			underlying: dataflowJoinProjectedMap,
		},
		{
			name: "object-array",
			kind: SchemaObjectArray,
			register: func(env *Environment, name string) (Schema, error) {
				return RegisterObjectArray(env, name, fields)
			},
			underlying: func(value any) bool {
				values, ok := value.([]any)
				return ok && len(values) == 2 && values[0] == 1 && values[1] == 10
			},
		},
		{
			name: "json",
			kind: SchemaJSON,
			register: func(env *Environment, name string) (Schema, error) {
				return RegisterJSON(env, name, fields)
			},
			underlying: dataflowJoinProjectedMap,
		},
		{
			name: "xml",
			kind: SchemaXML,
			register: func(env *Environment, name string) (Schema, error) {
				return RegisterXML(env, name, fields)
			},
			underlying: dataflowJoinProjectedMap,
		},
		{
			name: "avro",
			kind: SchemaAvro,
			register: func(env *Environment, name string) (Schema, error) {
				return RegisterAvro(env, name, fields)
			},
			underlying: func(value any) bool {
				record, ok := value.(*AvroRecord)
				return ok && record.Get("leftID") == 1 && record.Get("rightID") == 10
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[dataflowJoinLeft](env, "JoinLeft"); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[dataflowJoinRight](env, "JoinRight"); err != nil {
				t.Fatal(err)
			}
			eventType := "JoinProjection" + strings.ReplaceAll(test.name, "-", "")
			if _, err := test.register(env, eventType); err != nil {
				t.Fatal(err)
			}
			definition, err := DefineDataflow(env, "dataflow-join-event-"+test.name).
				Emitter("left").
				Emitter("right").
				SelectJoinEvent("select", eventType, DataflowJoinOptions{Inputs: 2},
					Alias("leftID", JoinField[int](0, "id")),
					Alias("rightID", JoinField[int](1, "id")),
				).
				Emitter("sink").
				ConnectInput("left", "select", 0).
				ConnectInput("right", "select", 1).
				Connect("select", "sink").
				Build()
			if err != nil {
				t.Fatal(err)
			}
			instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
			if err != nil {
				t.Fatal(err)
			}
			captive, err := instance.StartCaptive(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = instance.Cancel(context.Background()) })
			left, _ := captive.Emitter("left")
			right, _ := captive.Emitter("right")
			if err := left.Submit(context.Background(), dataflowJoinLeft{ID: 1}); err != nil {
				t.Fatal(err)
			}
			if err := right.Submit(context.Background(), dataflowJoinRight{ID: 10}); err != nil {
				t.Fatal(err)
			}
			outputs := instance.Outputs()
			if len(outputs) != 1 {
				t.Fatalf("%s join event outputs = %#v, want one", test.name, outputs)
			}
			event, ok := outputs[0].(Event)
			if !ok {
				t.Fatalf("%s join output = %T, want Event", test.name, outputs[0])
			}
			if event.TypeName() != eventType || event.Schema().Kind() != test.kind {
				t.Fatalf("%s join event identity = %q/%v", test.name, event.TypeName(), event.Schema().Kind())
			}
			if event.Get("leftID").Any() != 1 || event.Get("rightID").Any() != 10 || !test.underlying(event.Underlying()) {
				t.Fatalf("%s join event = %#v", test.name, event)
			}
		})
	}
}

func dataflowJoinProjectedMap(value any) bool {
	values, ok := value.(map[string]any)
	return ok && values["leftID"] == 1 && values["rightID"] == 10
}

func TestDataflowSelectJoinEventRejectsInvalidOutputType(t *testing.T) {
	build := func(eventType string) error {
		env := NewEnvironment()
		if _, err := RegisterStruct[dataflowJoinLeft](env, "JoinLeft"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[dataflowJoinRight](env, "JoinRight"); err != nil {
			t.Fatal(err)
		}
		_, err := DefineDataflow(env, "invalid-dataflow-join-event").
			Emitter("left").
			Emitter("right").
			SelectJoinEvent("select", eventType, DataflowJoinOptions{Inputs: 2},
				Alias("leftID", JoinField[int](0, "id")),
			).
			Emitter("sink").
			ConnectInput("left", "select", 0).
			ConnectInput("right", "select", 1).
			Connect("select", "sink").
			Build()
		return err
	}
	if err := build(""); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("empty join output event type error = %v", err)
	}
	if err := build("MissingJoinOutput"); err == nil || !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("unknown join output event type error = %v", err)
	}
	env := NewEnvironment()
	if _, err := DefineDataflow(env, "invalid-dataflow-select-event").
		Emitter("source").
		SelectEvent("select", "").
		Emitter("sink").
		Build(); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("empty ordinary SelectEvent output type error = %v", err)
	}
}

func TestDataflowSelectOptionsEnterPlanIdentity(t *testing.T) {
	type planInput struct {
		ID int `esper:"id"`
	}
	build := func(configure func(DataflowBuilder) DataflowBuilder) Plan {
		t.Helper()
		env := NewEnvironment()
		if _, err := RegisterStruct[planInput](env, "PlanInput"); err != nil {
			t.Fatal(err)
		}
		fields := []FieldSpec{FieldDef("id", reflect.TypeOf(int(0)))}
		if _, err := RegisterMap(env, "PlanOutputA", fields); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterMap(env, "PlanOutputB", fields); err != nil {
			t.Fatal(err)
		}
		builder := configure(DefineDataflow(env, "select-plan-identity").Emitter("source"))
		if _, err := builder.Emitter("sink").Build(); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(From[planInput](env, "PlanInput").Query())
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	selection := Alias("id", Field[planInput, int]("id"))
	oneSecond := build(func(builder DataflowBuilder) DataflowBuilder {
		return builder.SelectTimeWindow("select", time.Second, selection)
	})
	twoSeconds := build(func(builder DataflowBuilder) DataflowBuilder {
		return builder.SelectTimeWindow("select", 2*time.Second, selection)
	})
	if oneSecond.Hash() == twoSeconds.Hash() {
		t.Fatal("different Select time windows share Plan identity")
	}
	preserve := build(func(builder DataflowBuilder) DataflowBuilder {
		return builder.SelectEvent("select", "PlanOutputA", selection)
	})
	projectOnly := build(func(builder DataflowBuilder) DataflowBuilder {
		return builder.SelectWithOptions("select", DataflowSelectOptions{OutputEventType: "PlanOutputA"}, selection)
	})
	if preserve.Hash() == projectOnly.Hash() {
		t.Fatal("Select preserve-input policy is absent from Plan identity")
	}
	outputA := build(func(builder DataflowBuilder) DataflowBuilder {
		return builder.SelectWithOptions("select", DataflowSelectOptions{OutputEventType: "PlanOutputA"}, selection)
	})
	outputB := build(func(builder DataflowBuilder) DataflowBuilder {
		return builder.SelectWithOptions("select", DataflowSelectOptions{OutputEventType: "PlanOutputB"}, selection)
	})
	if outputA.Hash() == outputB.Hash() {
		t.Fatal("different Select output event types share Plan identity")
	}
	ascending := build(func(builder DataflowBuilder) DataflowBuilder {
		return builder.SelectIterate("select", []Expr{Field[planInput, int]("id")}, []SortKey{Ascending(ResultField[int]("id"))}, selection)
	})
	descending := build(func(builder DataflowBuilder) DataflowBuilder {
		return builder.SelectIterate("select", []Expr{Field[planInput, int]("id")}, []SortKey{Descending(ResultField[int]("id"))}, selection)
	})
	if ascending.Hash() == descending.Hash() {
		t.Fatal("Select iterate ordering is absent from Plan identity")
	}
	canonical := string(ascending.Canonical())
	if ascending.SchemaVersion() != "esper-go-plan/v2" {
		t.Fatalf("Select Plan schema version = %q, want v2", ascending.SchemaVersion())
	}
	legacyArtifact, err := json.Marshal(PlanArtifact{
		SchemaVersion: "esper-go-plan/v1",
		Hash:          ascending.Hash(),
		Canonical:     ascending.Canonical(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPlanArtifact(legacyArtifact); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("legacy Select Plan artifact error = %v", err)
	}
	for _, token := range []string{"select=output=", "preserve=false", "iterate=true", "group=", "order="} {
		if !strings.Contains(canonical, token) {
			t.Fatalf("Select Plan canonical is missing %q: %s", token, canonical)
		}
	}
}
