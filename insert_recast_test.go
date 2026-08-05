package esper

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

type insertRecastSource struct {
	P0 string `esper:"p0"`
	P1 int64  `esper:"p1"`
}

type insertRecastTarget struct {
	P0 string `esper:"p0"`
	P1 int64  `esper:"p1"`
	C0 *int64 `esper:"c0"`
}

func TestInsertIntoWildcardRecastAcrossRepresentations(t *testing.T) {
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
	for _, source := range representations {
		for _, target := range representations {
			t.Run(source.name+"-to-"+target.name, func(t *testing.T) {
				env := NewEnvironment()
				sourceName := fmt.Sprintf("RecastSource_%s_%s", source.name, target.name)
				targetName := fmt.Sprintf("RecastTarget_%s_%s", source.name, target.name)
				if err := registerInsertRecastSchema(env, sourceName, source.kind, false); err != nil {
					t.Fatal(err)
				}
				if err := registerInsertRecastSchema(env, targetName, target.kind, true); err != nil {
					t.Fatal(err)
				}

				routePlan, err := env.Build(FromAny(env, sourceName).InsertInto(targetName, StatementName("recast-route")))
				if err != nil {
					t.Fatal(err)
				}
				consumerPlan, err := env.Build(FromAny(env, targetName).Query(StatementName("recast-consumer")))
				if err != nil {
					t.Fatal(err)
				}
				engine := NewEngine(env)
				if _, err := engine.Deploy(context.Background(), routePlan); err != nil {
					t.Fatal(err)
				}
				consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
				if err != nil {
					t.Fatal(err)
				}
				var routed []Event
				if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
					for _, result := range batch.New {
						if event, ok := result.Event(); ok {
							routed = append(routed, event)
						}
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				if err := sendInsertRecastSource(engine, source.kind, sourceName); err != nil {
					t.Fatal(err)
				}
				if len(routed) != 1 {
					t.Fatalf("recast events = %#v", routed)
				}
				event := routed[0]
				if event.TypeName() != targetName || event.Get("p0").Any() != "a" || event.Get("p1").Any() != int64(10) || !event.Get("c0").IsNull() {
					t.Fatalf("recast event = type=%s p0=%v p1=%v c0=%v underlying=%#v", event.TypeName(), event.Get("p0"), event.Get("p1"), event.Get("c0"), event.Underlying())
				}
				assertInsertRecastUnderlying(t, event, target.kind)
			})
		}
	}
}

func registerInsertRecastSchema(env *Environment, name string, kind SchemaKind, target bool) error {
	fields := []FieldSpec{
		FieldDef("p0", reflect.TypeOf("")),
		FieldDef("p1", reflect.TypeOf(int64(0))),
	}
	if target {
		fields = append(fields, FieldDef("c0", reflect.TypeOf(int64(0))))
	}
	if kind == SchemaStruct {
		if target {
			_, err := RegisterStruct[insertRecastTarget](env, name)
			return err
		}
		_, err := RegisterStruct[insertRecastSource](env, name)
		return err
	}
	switch kind {
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
		return fmt.Errorf("unsupported recast schema kind %d", kind)
	}
}

func sendInsertRecastSource(engine *Engine, kind SchemaKind, eventType string) error {
	ctx := context.Background()
	switch kind {
	case SchemaStruct:
		return engine.SendEvent(ctx, insertRecastSource{P0: "a", P1: 10})
	case SchemaMap:
		return engine.SendRecord(ctx, eventType, map[string]any{"p0": "a", "p1": int64(10)})
	case SchemaObjectArray:
		return engine.SendObjectArray(ctx, eventType, []any{"a", int64(10)})
	case SchemaJSON:
		return engine.SendJSON(ctx, eventType, []byte(`{"p0":"a","p1":10}`))
	case SchemaXML:
		return engine.SendXML(ctx, eventType, []byte(`<event><p0>a</p0><p1>10</p1></event>`))
	case SchemaAvro:
		return engine.SendAvroJSON(ctx, eventType, []byte(`{"p0":"a","p1":10}`))
	default:
		return fmt.Errorf("unsupported recast source kind %d", kind)
	}
}

func assertInsertRecastUnderlying(t *testing.T, event Event, kind SchemaKind) {
	t.Helper()
	switch kind {
	case SchemaStruct:
		value, ok := event.Underlying().(insertRecastTarget)
		if !ok || value.P0 != "a" || value.P1 != 10 || value.C0 != nil {
			t.Fatalf("recast struct underlying = %#v", event.Underlying())
		}
	case SchemaObjectArray:
		value, ok := event.Underlying().([]any)
		if !ok || len(value) != 3 || value[0] != "a" || value[1] != int64(10) || value[2] != nil {
			t.Fatalf("recast object-array underlying = %#v", event.Underlying())
		}
	default:
		value, ok := event.Underlying().(map[string]any)
		if !ok || value["p0"] != "a" || value["p1"] != int64(10) || value["c0"] != nil {
			t.Fatalf("recast kind %d underlying = %#v", kind, event.Underlying())
		}
	}
}
