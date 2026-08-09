package esper

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

type clientRuntimeListenerTrigger struct {
	TheString string `esper:"theString"`
}

type clientRuntimeListenerBean struct {
	Ident string `esper:"ident"`
}

func TestClientRuntimeListenerRouteParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[clientRuntimeListenerTrigger](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[clientRuntimeListenerBean](env, "RoutedBeanEvent"); err != nil {
		t.Fatal(err)
	}
	fields := []FieldSpec{FieldDef("ident", reflect.TypeOf(""))}
	registrations := []func() error{
		func() error { _, err := RegisterMap(env, "RoutedMap", fields); return err },
		func() error { _, err := RegisterObjectArray(env, "RoutedObjectArray", fields); return err },
		func() error { _, err := RegisterXML(env, "RoutedXML", fields); return err },
		func() error { _, err := RegisterAvro(env, "RoutedAvro", fields); return err },
		func() error { _, err := RegisterJSON(env, "JsonEvent", fields); return err },
	}
	for _, register := range registrations {
		if err := register(); err != nil {
			t.Fatal(err)
		}
	}

	engine := NewEngine(env)
	received := make(map[string][]string)
	for _, eventType := range []string{"RoutedMap", "RoutedBeanEvent", "RoutedObjectArray", "RoutedXML", "RoutedAvro", "JsonEvent"} {
		plan, err := env.Build(FromAny(env, eventType).Query(StatementName("consumer-" + eventType)))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		name := eventType
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				received[name] = append(received[name], result.Get("ident").Any().(string))
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}

	triggerPlan, err := env.Build(From[clientRuntimeListenerTrigger](env, "SupportBean").Query(StatementName("trigger")))
	if err != nil {
		t.Fatal(err)
	}
	triggerDeployment, err := engine.Deploy(context.Background(), triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := triggerDeployment.Statements()[0].Subscribe(func(ctx context.Context, batch ResultBatch) error {
		if len(batch.New) != 1 {
			return NewError(ErrorState, fmt.Sprintf("route trigger new-stream size %d", len(batch.New)))
		}
		ident := batch.New[0].Get("theString").Any().(string)
		if err := engine.Route(ctx, "RoutedBeanEvent", &clientRuntimeListenerBean{Ident: ident}); err != nil {
			return err
		}
		if err := engine.Route(ctx, "RoutedMap", map[string]any{"ident": ident}); err != nil {
			return err
		}
		if err := engine.Route(ctx, "RoutedObjectArray", []any{ident}); err != nil {
			return err
		}
		if err := engine.SendXML(ctx, "RoutedXML", []byte(`<myevent><ident>`+ident+`</ident></myevent>`)); err != nil {
			return err
		}
		avroSchema, _ := env.Schema("RoutedAvro")
		avro, err := NewAvroRecordFromMap(avroSchema, map[string]any{"ident": ident})
		if err != nil {
			return err
		}
		if err := engine.Route(ctx, "RoutedAvro", avro); err != nil {
			return err
		}
		jsonSender, err := engine.JSONSender("JsonEvent")
		if err != nil {
			return err
		}
		return jsonSender.Route(ctx, []byte(`{"ident":"`+ident+`"}`))
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), &clientRuntimeListenerTrigger{TheString: "xy"}); err != nil {
		t.Fatal(err)
	}
	for _, eventType := range []string{"RoutedMap", "RoutedBeanEvent", "RoutedObjectArray", "RoutedXML", "RoutedAvro", "JsonEvent"} {
		if !reflect.DeepEqual(received[eventType], []string{"xy"}) {
			t.Fatalf("listener route %s = %#v, want [xy]", eventType, received[eventType])
		}
	}
}
