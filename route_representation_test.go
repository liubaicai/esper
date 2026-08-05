package esper

import (
	"context"
	"reflect"
	"testing"
)

func TestInsertIntoPartialColumnsMaterializesNullAcrossRepresentations(t *testing.T) {
	representations := []struct {
		name string
		kind SchemaKind
	}{
		{name: "map", kind: SchemaMap},
		{name: "object-array", kind: SchemaObjectArray},
		{name: "json", kind: SchemaJSON},
		{name: "xml", kind: SchemaXML},
		{name: "avro", kind: SchemaAvro},
	}
	for _, representation := range representations {
		t.Run(representation.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[variantOrder](env, "PartialRouteSource"); err != nil {
				t.Fatal(err)
			}
			fields := []FieldSpec{
				FieldDef("c0", reflect.TypeOf("")),
				FieldDef("c1", reflect.TypeOf(int64(0))),
			}
			var err error
			switch representation.kind {
			case SchemaMap:
				_, err = RegisterMap(env, "PartialRouteTarget", fields)
			case SchemaObjectArray:
				_, err = RegisterObjectArray(env, "PartialRouteTarget", fields)
			case SchemaJSON:
				_, err = RegisterJSON(env, "PartialRouteTarget", fields)
			case SchemaXML:
				_, err = RegisterXML(env, "PartialRouteTarget", fields)
			case SchemaAvro:
				_, err = RegisterAvro(env, "PartialRouteTarget", fields)
			}
			if err != nil {
				t.Fatal(err)
			}

			first := Select(
				From[variantOrder](env, "PartialRouteSource"),
				Alias("c0", Field[variantOrder, string]("id")),
			).InsertInto("PartialRouteTarget", StatementName("partial-route-c0"))
			second := Select(
				From[variantOrder](env, "PartialRouteSource"),
				Alias("c1", Field[variantOrder, int64]("amount")),
			).InsertInto("PartialRouteTarget", StatementName("partial-route-c1"))
			consumer := FromAny(env, "PartialRouteTarget").Query(StatementName("partial-route-consumer"))
			plans := make([]Plan, 0, 3)
			for _, query := range []Query{first, second, consumer} {
				plan, buildErr := env.Build(query)
				if buildErr != nil {
					t.Fatal(buildErr)
				}
				plans = append(plans, plan)
			}
			engine := NewEngine(env)
			var consumerDeployment *Deployment
			for index, plan := range plans {
				deployed, err := engine.Deploy(context.Background(), plan)
				if err != nil {
					t.Fatal(err)
				}
				if index == 2 {
					consumerDeployment = deployed
				}
			}
			var routed []Event
			consumerStatement := consumerDeployment.Statements()[0]
			if _, err := consumerStatement.Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, result := range batch.New {
					if event, ok := result.Event(); ok {
						routed = append(routed, event)
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(context.Background(), variantOrder{ID: "E1", Amount: 7}); err != nil {
				t.Fatal(err)
			}
			if len(routed) != 2 {
				t.Fatalf("routed partial events = %#v", routed)
			}
			var c0Event, c1Event Event
			for _, event := range routed {
				if event.Get("c0").Any() == "E1" {
					c0Event = event
				} else {
					c1Event = event
				}
			}
			if !c0Event.Get("c0").IsPresent() || c0Event.Get("c0").Any() != "E1" || !c0Event.Get("c1").IsNull() {
				t.Fatalf("partial c0 event = %#v", c0Event)
			}
			if !c1Event.Get("c0").IsNull() || c1Event.Get("c1").Any() != int64(7) {
				t.Fatalf("partial c1 event = %#v", c1Event)
			}
			if representation.kind == SchemaObjectArray {
				values, ok := c0Event.Underlying().([]any)
				if !ok || len(values) != 2 || values[0] != "E1" || values[1] != nil {
					t.Fatalf("partial object-array underlying = %#v", c0Event.Underlying())
				}
			}
		})
	}
}
