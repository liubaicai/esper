package esper

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type splitStreamSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type splitStreamRecorder struct {
	events []Event
}

func newSplitStreamEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[splitStreamSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func registerSplitBeanTarget(t *testing.T, env *Environment, name string) {
	t.Helper()
	if _, err := RegisterMap(env, name, []FieldSpec{
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(int(0))),
	}); err != nil {
		t.Fatal(err)
	}
}

func registerSplitStringTargets(t *testing.T, env *Environment, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := RegisterMap(env, name, []FieldSpec{FieldDef("theString", reflect.TypeOf(""))}); err != nil {
			t.Fatal(err)
		}
	}
}

func deploySplitPlan(t *testing.T, engine *Engine, plan Plan) (*Deployment, *splitStreamRecorder) {
	t.Helper()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	recorder := &splitStreamRecorder{}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("split-stream result is not an event: %#v", result)
			}
			recorder.events = append(recorder.events, event)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return deployment, recorder
}

func deploySplitConsumer(t *testing.T, env *Environment, engine *Engine, eventType string) *splitStreamRecorder {
	t.Helper()
	plan, err := env.Build(FromAny(env, eventType).Query(StatementName("consume-" + eventType)))
	if err != nil {
		t.Fatal(err)
	}
	_, recorder := deploySplitPlan(t, engine, plan)
	return recorder
}

func splitEventStrings(recorder *splitStreamRecorder) []string {
	if recorder == nil {
		return nil
	}
	values := make([]string, 0, len(recorder.events))
	for _, event := range recorder.events {
		value, _ := event.Get("theString").Any().(string)
		values = append(values, value)
	}
	return values
}

func TestSplitStream2SplitNoDefaultOutputFirstParity(t *testing.T) {
	env := newSplitStreamEnvironment(t)
	registerSplitBeanTarget(t, env, "AStream2SP")
	registerSplitBeanTarget(t, env, "BStream2SP")

	source := From[splitStreamSupportBean](env, "SupportBean")
	intPrimitive := Field[splitStreamSupportBean, int]("intPrimitive")
	plan, err := env.Build(OnEvent(source).SplitFirst(
		SplitIntoWhen(Equal[int](intPrimitive, Literal(1)), "AStream2SP"),
		SplitIntoWhen(Or(Equal[int](intPrimitive, Literal(1)), Equal[int](intPrimitive, Literal(2))), "BStream2SP"),
	).Query(StatementName("split")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	a := deploySplitConsumer(t, env, engine, "AStream2SP")
	b := deploySplitConsumer(t, env, engine, "BStream2SP")
	_, fallback := deploySplitPlan(t, engine, plan)
	for _, event := range []splitStreamSupportBean{{"E1", 1}, {"E2", 2}, {"E3", 1}, {"E4", -999}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if got := splitEventStrings(a); !reflect.DeepEqual(got, []string{"E1", "E3"}) {
		t.Fatalf("AStream2SP = %#v", got)
	}
	if got := splitEventStrings(b); !reflect.DeepEqual(got, []string{"E2"}) {
		t.Fatalf("BStream2SP = %#v", got)
	}
	if got := splitEventStrings(fallback); !reflect.DeepEqual(got, []string{"E4"}) {
		t.Fatalf("split fallback = %#v", got)
	}
}

func TestSplitStream1SplitDefaultParity(t *testing.T) {
	t.Run("wildcard", func(t *testing.T) {
		env := newSplitStreamEnvironment(t)
		registerSplitBeanTarget(t, env, "AStream")
		plan, err := env.Build(OnEvent(From[splitStreamSupportBean](env, "SupportBean")).SplitFirst(
			SplitInto("AStream"),
		).Query(StatementName("insert")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		a := deploySplitConsumer(t, env, engine, "AStream")
		_, fallback := deploySplitPlan(t, engine, plan)
		if err := engine.SendEvent(context.Background(), splitStreamSupportBean{"E1", 1}); err != nil {
			t.Fatal(err)
		}
		if got := splitEventStrings(a); !reflect.DeepEqual(got, []string{"E1"}) || len(fallback.events) != 0 {
			t.Fatalf("wildcard route=%#v fallback=%#v", got, fallback.events)
		}
	})

	t.Run("projection", func(t *testing.T) {
		env := newSplitStreamEnvironment(t)
		if _, err := RegisterMap(env, "BStreamABC", []FieldSpec{FieldDef("value", reflect.TypeOf(int(0)))}); err != nil {
			t.Fatal(err)
		}
		intPrimitive := Field[splitStreamSupportBean, int]("intPrimitive")
		plan, err := env.Build(OnEvent(From[splitStreamSupportBean](env, "SupportBean")).SplitFirst(
			SplitInto("BStreamABC", Alias("value", Multiply[int](Literal(3), intPrimitive))),
		).Query(StatementName("s1")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		consumer := deploySplitConsumer(t, env, engine, "BStreamABC")
		_, fallback := deploySplitPlan(t, engine, plan)
		if err := engine.SendEvent(context.Background(), splitStreamSupportBean{"E1", 6}); err != nil {
			t.Fatal(err)
		}
		if len(consumer.events) != 1 || consumer.events[0].Get("value").Any() != 18 || len(fallback.events) != 0 {
			t.Fatalf("projection route=%#v fallback=%#v", consumer.events, fallback.events)
		}
	})
}

func TestSplitStream2SplitNoDefaultOutputAllParity(t *testing.T) {
	env := newSplitStreamEnvironment(t)
	registerSplitStringTargets(t, env, "AStream2S", "BStream2S")
	source := From[splitStreamSupportBean](env, "SupportBean")
	theString := Field[splitStreamSupportBean, string]("theString")
	intPrimitive := Field[splitStreamSupportBean, int]("intPrimitive")
	plan, err := env.Build(OnEvent(source).SplitAll(
		SplitIntoWhen(Equal[int](intPrimitive, Literal(1)), "AStream2S", Alias("theString", theString)),
		SplitIntoWhen(Or(Equal[int](intPrimitive, Literal(1)), Equal[int](intPrimitive, Literal(2))), "BStream2S", Alias("theString", theString)),
	).Query(StatementName("split")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	a := deploySplitConsumer(t, env, engine, "AStream2S")
	b := deploySplitConsumer(t, env, engine, "BStream2S")
	_, fallback := deploySplitPlan(t, engine, plan)
	for _, event := range []splitStreamSupportBean{{"E1", 1}, {"E2", 2}, {"E3", 1}, {"E4", -999}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if got := splitEventStrings(a); !reflect.DeepEqual(got, []string{"E1", "E3"}) {
		t.Fatalf("AStream2S = %#v", got)
	}
	if got := splitEventStrings(b); !reflect.DeepEqual(got, []string{"E1", "E2", "E3"}) {
		t.Fatalf("BStream2S = %#v", got)
	}
	if got := splitEventStrings(fallback); !reflect.DeepEqual(got, []string{"E4"}) {
		t.Fatalf("split fallback = %#v", got)
	}
}

func TestSplitStream3SplitOutputAllParity(t *testing.T) {
	env := newSplitStreamEnvironment(t)
	registerSplitStringTargets(t, env, "AStream2S", "BStream2S", "CStream2S")
	source := From[splitStreamSupportBean](env, "SupportBean")
	theString := Field[splitStreamSupportBean, string]("theString")
	intPrimitive := Field[splitStreamSupportBean, int]("intPrimitive")
	plan, err := env.Build(OnEvent(source).SplitAll(
		SplitIntoWhen(Or(Equal[int](intPrimitive, Literal(1)), Equal[int](intPrimitive, Literal(2))), "AStream2S", Alias("theString", Concat(theString, Literal("_1")))),
		SplitIntoWhen(Or(Equal[int](intPrimitive, Literal(2)), Equal[int](intPrimitive, Literal(3))), "BStream2S", Alias("theString", Concat(theString, Literal("_2")))),
		SplitInto("CStream2S", Alias("theString", Concat(theString, Literal("_3")))),
	).Query(StatementName("split")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	a := deploySplitConsumer(t, env, engine, "AStream2S")
	b := deploySplitConsumer(t, env, engine, "BStream2S")
	c := deploySplitConsumer(t, env, engine, "CStream2S")
	_, fallback := deploySplitPlan(t, engine, plan)
	for _, event := range []splitStreamSupportBean{{"E1", 2}, {"E2", 1}, {"E3", 3}, {"E4", -999}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if got := splitEventStrings(a); !reflect.DeepEqual(got, []string{"E1_1", "E2_1"}) {
		t.Fatalf("AStream2S = %#v", got)
	}
	if got := splitEventStrings(b); !reflect.DeepEqual(got, []string{"E1_2", "E3_2"}) {
		t.Fatalf("BStream2S = %#v", got)
	}
	if got := splitEventStrings(c); !reflect.DeepEqual(got, []string{"E1_3", "E2_3", "E3_3", "E4_3"}) || len(fallback.events) != 0 {
		t.Fatalf("CStream2S=%#v fallback=%#v", got, fallback.events)
	}
}

func TestSplitStream3SplitDefaultOutputFirstParity(t *testing.T) {
	env := newSplitStreamEnvironment(t)
	registerSplitStringTargets(t, env, "AStream34", "BStream34", "CStream34")
	source := From[splitStreamSupportBean](env, "SupportBean")
	theString := Field[splitStreamSupportBean, string]("theString")
	intPrimitive := Field[splitStreamSupportBean, int]("intPrimitive")
	firstPlan, err := env.Build(OnEvent(source).SplitFirst(
		SplitIntoWhen(Equal[int](intPrimitive, Literal(1)), "AStream34", Alias("theString", Concat(theString, Literal("_1")))),
		SplitIntoWhen(Equal[int](intPrimitive, Literal(2)), "BStream34", Alias("theString", Concat(theString, Literal("_2")))),
		SplitInto("CStream34", Alias("theString", Concat(theString, Literal("_3")))),
	).Query(StatementName("split")))
	if err != nil {
		t.Fatal(err)
	}
	allPlan, err := env.Build(OnEvent(source).SplitAll(
		SplitIntoWhen(Equal[int](intPrimitive, Literal(1)), "AStream34", Alias("theString", theString)),
	).Query(StatementName("split-all-identity")))
	if err != nil {
		t.Fatal(err)
	}
	if firstPlan.Hash() == allPlan.Hash() {
		t.Fatal("split-first and split-all must have different plan identity")
	}

	engine := NewEngine(env)
	a := deploySplitConsumer(t, env, engine, "AStream34")
	b := deploySplitConsumer(t, env, engine, "BStream34")
	c := deploySplitConsumer(t, env, engine, "CStream34")
	_, fallback := deploySplitPlan(t, engine, firstPlan)
	for _, event := range []splitStreamSupportBean{{"E1", 1}, {"E2", 2}, {"E3", 1}, {"E4", -999}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if got := splitEventStrings(a); !reflect.DeepEqual(got, []string{"E1_1", "E3_1"}) {
		t.Fatalf("AStream34 = %#v", got)
	}
	if got := splitEventStrings(b); !reflect.DeepEqual(got, []string{"E2_2"}) {
		t.Fatalf("BStream34 = %#v", got)
	}
	if got := splitEventStrings(c); !reflect.DeepEqual(got, []string{"E4_3"}) || len(fallback.events) != 0 {
		t.Fatalf("CStream34=%#v fallback=%#v", got, fallback.events)
	}
}

func TestSplitStream4SplitParity(t *testing.T) {
	env := newSplitStreamEnvironment(t)
	registerSplitStringTargets(t, env, "AStream34", "BStream34", "CStream34", "DStream34")
	source := From[splitStreamSupportBean](env, "SupportBean")
	theString := Field[splitStreamSupportBean, string]("theString")
	intPrimitive := Field[splitStreamSupportBean, int]("intPrimitive")
	plan, err := env.Build(OnEvent(source).SplitFirst(
		SplitIntoWhen(Equal[int](intPrimitive, Literal(10)), "AStream34", Alias("theString", Concat(theString, Literal("_1")))),
		SplitIntoWhen(Equal[int](intPrimitive, Literal(20)), "BStream34", Alias("theString", Concat(theString, Literal("_2")))),
		SplitIntoWhen(Less[int](intPrimitive, Literal(0)), "CStream34", Alias("theString", Concat(theString, Literal("_3")))),
		SplitInto("DStream34", Alias("theString", Concat(theString, Literal("_4")))),
	).Query(StatementName("split")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	a := deploySplitConsumer(t, env, engine, "AStream34")
	b := deploySplitConsumer(t, env, engine, "BStream34")
	c := deploySplitConsumer(t, env, engine, "CStream34")
	d := deploySplitConsumer(t, env, engine, "DStream34")
	_, fallback := deploySplitPlan(t, engine, plan)
	for _, event := range []splitStreamSupportBean{{"E5", -999}, {"E6", 9999}, {"E7", 20}, {"E8", 10}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if got := splitEventStrings(a); !reflect.DeepEqual(got, []string{"E8_1"}) {
		t.Fatalf("AStream34 = %#v", got)
	}
	if got := splitEventStrings(b); !reflect.DeepEqual(got, []string{"E7_2"}) {
		t.Fatalf("BStream34 = %#v", got)
	}
	if got := splitEventStrings(c); !reflect.DeepEqual(got, []string{"E5_3"}) {
		t.Fatalf("CStream34 = %#v", got)
	}
	if got := splitEventStrings(d); !reflect.DeepEqual(got, []string{"E6_4"}) || len(fallback.events) != 0 {
		t.Fatalf("DStream34=%#v fallback=%#v", got, fallback.events)
	}
}

func TestSplitStreamInvalidParity(t *testing.T) {
	env := newSplitStreamEnvironment(t)
	if _, err := RegisterMap(env, "AStream", []FieldSpec{FieldDef("value", reflect.TypeOf(int(0)))}); err != nil {
		t.Fatal(err)
	}
	source := From[splitStreamSupportBean](env, "SupportBean")
	intPrimitive := Field[splitStreamSupportBean, int]("intPrimitive")
	if _, err := env.Build(OnEvent(source).SplitFirst().Query()); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("missing split branches error = %v", err)
	}
	if _, err := env.Build(OnEvent(source).SplitFirst(SplitInto("MissingStream")).Query()); err == nil || !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("unknown split target error = %v", err)
	}
	if _, err := env.Build(OnEvent(source).SplitFirst(
		SplitIntoWhen(Literal(1), "AStream", Alias("value", intPrimitive)),
	).Query()); err == nil || !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("non-boolean split condition error = %v", err)
	}
	if _, err := env.Build(OnEvent(source).SplitFirst(
		SplitInto("AStream", Alias("value", Sum[int](intPrimitive))),
	).Query()); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("aggregate split projection error = %v", err)
	}
}
