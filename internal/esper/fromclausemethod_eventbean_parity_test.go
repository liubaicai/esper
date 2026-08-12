package esper

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// fcmEventWithStaticMethod mirrors the event-only view of Java's
// SupportEventWithStaticMethod; the static bound methods become Go providers.
type fcmEventWithStaticMethod struct {
	Value int `esper:"value"`
}

// makeFCMConstBoundProvider returns one row {value: bound}, mirroring the
// constant returnLower/returnUpper static methods.
func makeFCMConstBoundProvider(schema Schema, bound int) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		return newEventsFrom(schema, []map[string]any{{"value": bound}}, request.Now)
	})
}

// TestFromClauseMethod2JoinEventItselfProvidesMethodParity mirrors
// EPLFromClauseMethod2JoinEventItselfProvidesMethod: the event class's own
// static methods provide the [lower:upper] range rows used by the where
// clause, so 9/21 are filtered and 10/20 pass.
func TestFromClauseMethod2JoinEventItselfProvidesMethodParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[fcmEventWithStaticMethod](env, "SupportEventWithStaticMethod"); err != nil {
		t.Fatal(err)
	}
	boundSchema := fcmMapSchema(t, env, "FCMBoundRow", FieldDef("value", reflect.TypeOf(0)))
	event := From[fcmEventWithStaticMethod](env, "SupportEventWithStaticMethod")
	lower := FromMethod[map[string]any](env, "lower", boundSchema, makeFCMConstBoundProvider(boundSchema, 10)).EvaluateOnce()
	upper := FromMethod[map[string]any](env, "upper", boundSchema, makeFCMConstBoundProvider(boundSchema, 20)).EvaluateOnce()
	query := JoinMany(JoinSource(event), JoinSource(lower), JoinSource(upper)).Select(
		SelectFrom(0, "value", Field[fcmEventWithStaticMethod, int]("value")),
	).Where(
		BetweenOf(JoinField[int](0, "value"), JoinField[int](1, "value"), JoinField[int](2, "value")),
	).Query(StatementName("s0"))

	engine, _, listener := fcmOuterDeploy(t, env, query)
	sendAssert := func(value int, expected bool) {
		t.Helper()
		listener.invoked = false
		listener.lastNew = nil
		if err := engine.SendEvent(context.Background(), fcmEventWithStaticMethod{Value: value}); err != nil {
			t.Fatal(err)
		}
		if listener.invoked != expected {
			t.Fatalf("value %d: invoked = %v, want %v (rows %#v)", value, listener.invoked, expected, listener.lastNew)
		}
	}

	sendAssert(9, false)
	sendAssert(10, true)
	sendAssert(20, true)
	sendAssert(21, false)
}

// makeFCMStreamNameWContextProvider mirrors getStreamNameWContext and
// getWithMethodResultParam: both produce one row p00='somevalue',
// p01=trigger.theString, p02=statement name; the nested method-result
// parameter of the latter is folded into the provider.
func makeFCMStreamNameWContextProvider(schema Schema) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		row := map[string]any{
			"p00": "somevalue",
			"p01": request.Trigger.Get("theString").Any(),
			"p02": request.Invocation.StatementName,
		}
		return newEventsFrom(schema, []map[string]any{row}, request.Now)
	})
}

// runFCMContextRowParity deploys a SupportBean x method join and asserts the
// single context-aware row, shared by the StreamNameWContext and
// WithMethodResultParam variants.
func runFCMContextRowParity(t *testing.T, name string) {
	t.Helper()
	env := newFCMEnvironment(t)
	schema := fcmMapSchema(t, env, "FCMContextRow",
		FieldDef("p00", reflect.TypeOf("")),
		FieldDef("p01", reflect.TypeOf("")),
		FieldDef("p02", reflect.TypeOf("")))
	stream := From[fcmSupportBean](env, "SupportBean")
	method := FromMethod[map[string]any](env, "s", schema, makeFCMStreamNameWContextProvider(schema))
	query := Join(stream, method).Select(
		SelectRight("p00", Field[map[string]any, string]("p00")),
		SelectRight("p01", Field[map[string]any, string]("p01")),
		SelectRight("p02", Field[map[string]any, string]("p02")),
	).Query(StatementName("s0"))

	engine, _, listener := fcmOuterDeploy(t, env, query)
	if err := engine.SendEvent(context.Background(), fcmSupportBean{TheString: "E1", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	if !listener.invoked {
		t.Fatalf("%s: listener not invoked", name)
	}
	fcmOuterAssertRows(t, listener.lastNew, []string{"p00", "p01", "p02"},
		[][]string{{"somevalue", "E1", "s0"}}, name)
}

// TestFromClauseMethodStreamNameWContextParity mirrors
// EPLFromClauseMethodStreamNameWContext: the provider observes the statement
// name through the invocation context.
func TestFromClauseMethodStreamNameWContextParity(t *testing.T) {
	runFCMContextRowParity(t, "stream-name-w-context")
}

// TestFromClauseMethodWithMethodResultParamParity mirrors
// EPLFromClauseMethodWithMethodResultParam: a method result used as a method
// invocation parameter; Go folds the nested evaluation into the provider.
func TestFromClauseMethodWithMethodResultParamParity(t *testing.T) {
	runFCMContextRowParity(t, "with-method-result-param")
}

// makeFCMEventBeanArrayProvider mirrors eventBeanArrayForString /
// eventBeanCollectionForString / eventBeanIteratorForString: the trigger's
// theString is split into one MyItemEvent row per part; Java's array,
// collection and iterator return shapes all map to Go's []Event contract.
func makeFCMEventBeanArrayProvider(schema Schema) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		value := request.Trigger.Get("theString").Any().(string)
		parts := strings.Split(value, ",")
		rows := make([]map[string]any, 0, len(parts))
		for _, part := range parts {
			rows = append(rows, map[string]any{"p0": part})
		}
		return newEventsFrom(schema, rows, request.Now)
	})
}

// TestFromClauseMethodEventBeanArrayParity mirrors
// EPLFromClauseMethodEventBeanArray: a method returning EventBean instances
// typed as MyItemEvent joins with the triggering SupportBean. The @type
// annotation has no Go counterpart because the provider's schema already
// declares the event type.
func TestFromClauseMethodEventBeanArrayParity(t *testing.T) {
	for _, variant := range []string{"array", "collection", "iterator"} {
		t.Run(variant, func(t *testing.T) {
			env := newFCMEnvironment(t)
			itemSchema := fcmMapSchema(t, env, "MyItemEvent", FieldDef("p0", reflect.TypeOf("")))
			stream := From[fcmSupportBean](env, "SupportBean")
			method := FromMethod[map[string]any](env, "items", itemSchema, makeFCMEventBeanArrayProvider(itemSchema))
			query := Join(stream, method).Select(
				SelectRight("p0", Field[map[string]any, string]("p0")),
			).Query(StatementName("s0"))

			engine, _, listener := fcmOuterDeploy(t, env, query)
			if err := engine.SendEvent(context.Background(), fcmSupportBean{TheString: "a,b"}); err != nil {
				t.Fatal(err)
			}
			if !listener.invoked {
				t.Fatalf("%s: listener not invoked", variant)
			}
			fcmOuterAssertRows(t, listener.lastNew, []string{"p0"}, [][]string{{"a"}, {"b"}}, variant)
		})
	}
}

// makeFCMItemProducerProvider mirrors myItemProducerUDF and the
// myItemProducerScript JS body: both return two ItemEvent rows id1/id3. The
// JS script host is a JVM capability, so the script's observable behavior is
// folded into the same Go provider.
func makeFCMItemProducerProvider(schema Schema) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		rows := []map[string]any{{"id": "id1"}, {"id": "id3"}}
		return newEventsFrom(schema, rows, request.Now)
	})
}

// TestFromClauseMethodUDFAndScriptReturningEventsParity mirrors
// EPLFromClauseMethodUDFAndScriptReturningEvents: a UDF (and, in Java, a JS
// script) returning EventBean instances acts as a method source. Go models
// both through the typed provider contract.
func TestFromClauseMethodUDFAndScriptReturningEventsParity(t *testing.T) {
	for _, variant := range []string{"udf", "script"} {
		t.Run(variant, func(t *testing.T) {
			env := newFCMEnvironment(t)
			itemSchema := fcmMapSchema(t, env, "ItemEvent", FieldDef("id", reflect.TypeOf("")))
			stream := From[fcmSupportBean](env, "SupportBean")
			method := FromMethod[map[string]any](env, "producer", itemSchema, makeFCMItemProducerProvider(itemSchema))
			query := Join(stream, method).Select(
				SelectRight("id", Field[map[string]any, string]("id")),
			).Query(StatementName("s0"))

			engine, _, listener := fcmOuterDeploy(t, env, query)
			if err := engine.SendEvent(context.Background(), fcmSupportBean{}); err != nil {
				t.Fatal(err)
			}
			if !listener.invoked {
				t.Fatalf("%s: listener not invoked", variant)
			}
			fcmOuterAssertRows(t, listener.lastNew, []string{"id"}, [][]string{{"id1"}, {"id3"}}, variant)
		})
	}
}

// TestFromClauseMethodInvocationTargetExParity mirrors
// EPLFromClauseMethodInvocationTargetEx: a provider failure surfaces as the
// SendEvent error, matching Esper's rethrown invocation target exception.
func TestFromClauseMethodInvocationTargetExParity(t *testing.T) {
	env := newFCMEnvironment(t)
	schema := fcmMapSchema(t, env, "FCMThrowRow", FieldDef("value", reflect.TypeOf(0)))
	provider := MethodProviderFunc(func(_ context.Context, _ MethodRequest) ([]Event, error) {
		return nil, fmt.Errorf("throwException text here")
	})
	stream := From[fcmSupportBean](env, "SupportBean").Window(LengthWindow(3))
	method := FromMethod[map[string]any](env, "thrower", schema, provider)
	query := Join(stream, method).Select(
		SelectLeft("theString", Field[fcmSupportBean, string]("theString")),
	).Query(StatementName("s0"))

	engine, _, _ := fcmOuterDeploy(t, env, query)
	if err := engine.SendEvent(context.Background(), fcmSupportBean{TheString: "E1"}); err == nil {
		t.Fatal("expected provider error to surface from SendEvent")
	}
}
