package esper

import (
	"context"
	"reflect"
	"testing"
)

type insertJoinWildcardLeft struct {
	Key   string `esper:"key"`
	Value int64  `esper:"leftValue"`
}

type insertJoinWildcardRight struct {
	Key   string `esper:"key"`
	Value int64  `esper:"rightValue"`
}

func TestInsertIntoJoinWildcardPreservesSourceEventsAcrossRepresentations(t *testing.T) {
	cases := []struct {
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
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env := NewEnvironment()
			leftName := "JoinWildcardLeft_" + testCase.name
			rightName := "JoinWildcardRight_" + testCase.name
			fieldsLeft := []FieldSpec{
				FieldDef("key", reflect.TypeOf("")),
				FieldDef("leftValue", reflect.TypeOf(int64(0))),
			}
			fieldsRight := []FieldSpec{
				FieldDef("key", reflect.TypeOf("")),
				FieldDef("rightValue", reflect.TypeOf(int64(0))),
			}
			if testCase.kind == SchemaStruct {
				if _, err := RegisterStruct[insertJoinWildcardLeft](env, leftName); err != nil {
					t.Fatal(err)
				}
				if _, err := RegisterStruct[insertJoinWildcardRight](env, rightName); err != nil {
					t.Fatal(err)
				}
			} else {
				register := func(name string, fields []FieldSpec) error {
					switch testCase.kind {
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
						return nil
					}
				}
				if err := register(leftName, fieldsLeft); err != nil {
					t.Fatal(err)
				}
				if err := register(rightName, fieldsRight); err != nil {
					t.Fatal(err)
				}
			}
			targetName := "JoinWildcardTarget_" + testCase.name
			if _, err := RegisterMap(env, targetName, []FieldSpec{
				FieldDef("s0", reflect.TypeOf(Event{})),
				FieldDef("s1", reflect.TypeOf(Event{})),
			}); err != nil {
				t.Fatal(err)
			}

			condition := OnSourcesEqual(
				0, Field[Event, string]("key"),
				1, Field[Event, string]("key"),
			)
			query := JoinMany(
				JoinRecordSource(FromAny(env, leftName)),
				JoinRecordSource(FromAny(env, rightName)),
			).On(condition).Select(
				SelectSourceEvent(0, "s0"),
				SelectSourceEvent(1, "s1"),
			).InsertInto(targetName, StatementName("join-wildcard-"+testCase.name))
			plan, err := env.Build(query)
			if err != nil {
				t.Fatal(err)
			}
			consumerPlan, err := env.Build(FromAny(env, targetName).Query(StatementName("join-wildcard-consumer-" + testCase.name)))
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			if _, err := engine.Deploy(context.Background(), plan); err != nil {
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

			if err := sendInsertJoinWildcardEvents(engine, testCase.kind, leftName, rightName); err != nil {
				t.Fatal(err)
			}
			if len(routed) != 1 {
				t.Fatalf("join wildcard routed events = %#v", routed)
			}
			left, err := As[Event](routed[0].Get("s0"))
			if err != nil || left.TypeName() != leftName || left.Get("key").Any() != "J-1" {
				t.Fatalf("join wildcard left event = %#v", routed[0].Get("s0"))
			}
			right, err := As[Event](routed[0].Get("s1"))
			if err != nil || right.TypeName() != rightName || right.Get("key").Any() != "J-1" {
				t.Fatalf("join wildcard right event = %#v", routed[0].Get("s1"))
			}
			if left.Get("leftValue").Any() != int64(11) || right.Get("rightValue").Any() != int64(22) {
				t.Fatalf("join wildcard source values = left=%v right=%v", left.Get("leftValue"), right.Get("rightValue"))
			}
		})
	}
}

func sendInsertJoinWildcardEvents(engine *Engine, kind SchemaKind, leftName, rightName string) error {
	ctx := context.Background()
	switch kind {
	case SchemaStruct:
		if err := engine.SendEvent(ctx, insertJoinWildcardLeft{Key: "J-1", Value: 11}); err != nil {
			return err
		}
		return engine.SendEvent(ctx, insertJoinWildcardRight{Key: "J-1", Value: 22})
	case SchemaMap:
		if err := engine.SendRecord(ctx, leftName, map[string]any{"key": "J-1", "leftValue": int64(11)}); err != nil {
			return err
		}
		return engine.SendRecord(ctx, rightName, map[string]any{"key": "J-1", "rightValue": int64(22)})
	case SchemaObjectArray:
		if err := engine.SendObjectArray(ctx, leftName, []any{"J-1", int64(11)}); err != nil {
			return err
		}
		return engine.SendObjectArray(ctx, rightName, []any{"J-1", int64(22)})
	case SchemaJSON:
		if err := engine.SendJSON(ctx, leftName, []byte(`{"key":"J-1","leftValue":11}`)); err != nil {
			return err
		}
		return engine.SendJSON(ctx, rightName, []byte(`{"key":"J-1","rightValue":22}`))
	case SchemaXML:
		if err := engine.SendXML(ctx, leftName, []byte(`<event><key>J-1</key><leftValue>11</leftValue></event>`)); err != nil {
			return err
		}
		return engine.SendXML(ctx, rightName, []byte(`<event><key>J-1</key><rightValue>22</rightValue></event>`))
	case SchemaAvro:
		if err := engine.SendAvroJSON(ctx, leftName, []byte(`{"key":"J-1","leftValue":11}`)); err != nil {
			return err
		}
		return engine.SendAvroJSON(ctx, rightName, []byte(`{"key":"J-1","rightValue":22}`))
	default:
		return nil
	}
}
