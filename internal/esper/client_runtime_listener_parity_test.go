package esper

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
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

func TestClientRuntimeListenerErrorStillDrainsQueuedRouteParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "ListenerErrorTrigger", []FieldSpec{FieldDef("id", typeOf[int]())}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "ListenerErrorTarget", []FieldSpec{FieldDef("id", typeOf[int]())}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	ctx := context.Background()
	var routed []int
	targetPlan, err := env.Build(FromAny(env, "ListenerErrorTarget").Query(StatementName("listener-error-target")))
	if err != nil {
		t.Fatal(err)
	}
	targetDeployment, err := engine.Deploy(ctx, targetPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := targetDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			routed = append(routed, result.Get("id").Any().(int))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	triggerPlan, err := env.Build(FromAny(env, "ListenerErrorTrigger").Query(StatementName("listener-error-trigger")))
	if err != nil {
		t.Fatal(err)
	}
	triggerDeployment, err := engine.Deploy(ctx, triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := triggerDeployment.Statements()[0].Subscribe(func(ctx context.Context, batch ResultBatch) error {
		if len(batch.New) != 1 {
			return NewError(ErrorState, "listener-error trigger batch size")
		}
		return engine.Route(ctx, "ListenerErrorTarget", map[string]any{"id": batch.New[0].Get("id").Any()})
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := triggerDeployment.Statements()[0].Subscribe(func(_ context.Context, _ ResultBatch) error {
		return NewError(ErrorState, "intentional listener failure")
	}); err != nil {
		t.Fatal(err)
	}
	err = engine.SendRecord(ctx, "ListenerErrorTrigger", map[string]any{"id": 7})
	if err == nil || !strings.Contains(err.Error(), "intentional listener failure") {
		t.Fatalf("listener error = %v, want intentional listener failure", err)
	}
	if !reflect.DeepEqual(routed, []int{7}) {
		t.Fatalf("queued route after listener error = %#v, want [7]", routed)
	}
}

func TestClientRuntimeTimerRouteDefersUntilSiblingListenersParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[clientRuntimeListenerTrigger](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[clientRuntimeListenerBean](env, "RoutedBeanEvent"); err != nil {
		t.Fatal(err)
	}
	origin := time.Unix(0, 0).UTC()
	engine := NewEngine(env, WithStartTime(origin))
	var order []string
	targetPlan, err := env.Build(From[clientRuntimeListenerBean](env, "RoutedBeanEvent").Query(StatementName("timer-route-target")))
	if err != nil {
		t.Fatal(err)
	}
	targetDeployment, err := engine.Deploy(context.Background(), targetPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := targetDeployment.Statements()[0].Subscribe(func(_ context.Context, _ ResultBatch) error {
		order = append(order, "routed")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	source := From[clientRuntimeListenerTrigger](env, "SupportBean")
	firstPlan, err := env.Build(TimerAt(source, origin.Add(time.Second)).Select(
		Alias("tick", Literal(true)),
	).Query(StatementName("timer-route-first")))
	if err != nil {
		t.Fatal(err)
	}
	secondPlan, err := env.Build(TimerAt(source, origin.Add(time.Second)).Select(
		Alias("tick", Literal(true)),
	).Query(StatementName("timer-route-second")))
	if err != nil {
		t.Fatal(err)
	}
	firstDeployment, err := engine.Deploy(context.Background(), firstPlan)
	if err != nil {
		t.Fatal(err)
	}
	secondDeployment, err := engine.Deploy(context.Background(), secondPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := firstDeployment.Statements()[0].Subscribe(func(ctx context.Context, _ ResultBatch) error {
		order = append(order, "first")
		return engine.Route(ctx, "RoutedBeanEvent", &clientRuntimeListenerBean{Ident: "timer"})
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := secondDeployment.Statements()[0].Subscribe(func(_ context.Context, _ ResultBatch) error {
		order = append(order, "second")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"first", "second", "routed"}) {
		t.Fatalf("timer route order = %#v, want [first second routed]", order)
	}
}

func TestClientRuntimeNamedWindowListenerRouteDefersUntilSiblingsParity(t *testing.T) {
	env := NewEnvironment()
	windowFields := []FieldSpec{FieldDef("id", reflect.TypeOf(int(0)))}
	if _, err := RegisterMap(env, "RawRouteWindowEvent", windowFields); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "RawRouteTargetEvent", windowFields); err != nil {
		t.Fatal(err)
	}
	windowSchema, ok := env.Schema("RawRouteWindowEvent")
	if !ok {
		t.Fatal("raw route window schema missing")
	}
	if _, err := CreateNamedWindow(env, "RawRouteWindow", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	targetPlan, err := env.Build(FromAny(env, "RawRouteTargetEvent").Query(StatementName("raw-route-target")))
	if err != nil {
		t.Fatal(err)
	}
	targetDeployment, err := engine.Deploy(context.Background(), targetPlan)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	if _, err := targetDeployment.Statements()[0].Subscribe(func(_ context.Context, _ ResultBatch) error {
		order = append(order, "routed")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("RawRouteWindow")
	if !ok {
		t.Fatal("raw route window missing")
	}
	if _, err := window.Subscribe(func(ctx context.Context, _ NamedWindowDelta) error {
		order = append(order, "first")
		return engine.Route(ctx, "RawRouteTargetEvent", map[string]any{"id": 1})
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := window.Subscribe(func(_ context.Context, _ NamedWindowDelta) error {
		order = append(order, "second")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "RawRouteWindow", map[string]any{"id": 1}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"first", "second", "routed"}) {
		t.Fatalf("named-window route order = %#v, want [first second routed]", order)
	}
}
