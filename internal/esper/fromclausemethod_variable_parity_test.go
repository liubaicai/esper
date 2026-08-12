package esper

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// fcmSupportBeanS0 mirrors SupportBean_S0 (id, p00).
type fcmSupportBeanS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// fcmSupportBeanS1 mirrors the id-only use of SupportBean_S1.
type fcmSupportBeanS1 struct {
	ID int `esper:"id"`
}

// fcmSupportBeanS2 mirrors SupportBean_S2 (id, p20).
type fcmSupportBeanS2 struct {
	ID  int    `esper:"id"`
	P20 string `esper:"p20"`
}

// makeFCMFetchABeanProvider mirrors fetchABean(intPrimitive) of the variable
// service classes: id = "_" + intPrimitive + "_" + postfix. The constant
// service is modeled by an empty postfix variable name; the non-constant and
// context-variable services read the postfix from the statement variables.
func makeFCMFetchABeanProvider(schema Schema, postfixVar string) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		number, _ := request.Trigger.Get("intPrimitive").Any().(int)
		postfix := ""
		if postfixVar != "" {
			if value, ok := request.Variables[postfixVar]; ok && value.Any() != nil {
				postfix, _ = value.Any().(string)
			}
		}
		return newEventsFrom(schema, []map[string]any{{"id": fmt.Sprintf("_%d_%s", number, postfix)}}, request.Now)
	})
}

func fcmFetchABeanQuery(t *testing.T, env *Environment, schema Schema, provider MethodProvider, name string, options ...QueryOption) Query {
	t.Helper()
	stream := From[fcmSupportBean](env, "SupportBean")
	method := FromMethod[map[string]any](env, "h0", schema, provider)
	return Join(stream, method).
		Select(SelectRight("c0", Field[map[string]any, string]("id"))).
		Query(append([]QueryOption{StatementName(name)}, options...)...)
}

// TestFromClauseMethodConstantVariableParity mirrors
// EPLFromClauseMethodConstantVariable: the constant service variable renders
// "_" + intPrimitive + "_".
func TestFromClauseMethodConstantVariableParity(t *testing.T) {
	env := newFCMEnvironment(t)
	schema := fcmMapSchema(t, env, "FCMFetchABeanRow", FieldDef("id", reflect.TypeOf("")))
	engine, _, listener := fcmOuterDeploy(t, env, fcmFetchABeanQuery(t, env, schema, makeFCMFetchABeanProvider(schema, ""), "s0"))
	fields := []string{"c0"}

	fcmMultikeyStep(t, engine, listener, fields, fcmSupportBean{TheString: "E1", IntPrimitive: 10}, [][]string{{"_10_"}}, "E1")
	fcmMultikeyStep(t, engine, listener, fields, fcmSupportBean{TheString: "E2", IntPrimitive: 20}, [][]string{{"_20_"}}, "E2")
}

// TestFromClauseMethodNonConstantVariableParity mirrors
// EPLFromClauseMethodNonConstantVariable (both soda variants): an on-set
// statement replaces the postfix the method source renders.
func TestFromClauseMethodNonConstantVariableParity(t *testing.T) {
	env := newFCMEnvironment(t)
	if _, err := RegisterStruct[fcmSupportBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("postfix", "postfix"); err != nil {
		t.Fatal(err)
	}
	setter, err := env.Build(OnEvent(From[fcmSupportBeanS0](env, "SupportBean_S0")).
		SetVariable("postfix", Field[fcmSupportBeanS0, string]("p00")).
		Query(StatementName("postfix-set")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	t.Cleanup(func() { _ = engine.Close(context.Background()) })
	if _, err := engine.Deploy(context.Background(), setter); err != nil {
		t.Fatal(err)
	}
	schema := fcmMapSchema(t, env, "FCMFetchABeanRow", FieldDef("id", reflect.TypeOf("")))
	plan, err := env.Build(fcmFetchABeanQuery(t, env, schema, makeFCMFetchABeanProvider(schema, "postfix"), "s0"))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	listener := fcmOuterSubscribe(t, deployment.Statements()[0])
	fields := []string{"c0"}

	fcmMultikeyStep(t, engine, listener, fields, fcmSupportBean{TheString: "E1", IntPrimitive: 10}, [][]string{{"_10_postfix"}}, "E1/10")
	if err := engine.SendEvent(context.Background(), fcmSupportBeanS0{ID: 1, P00: "newpostfix"}); err != nil {
		t.Fatal(err)
	}
	fcmMultikeyStep(t, engine, listener, fields, fcmSupportBean{TheString: "E1", IntPrimitive: 20}, [][]string{{"_20_newpostfix"}}, "E1/20")
	if err := engine.SendEvent(context.Background(), fcmSupportBeanS0{ID: 2, P00: "postfix"}); err != nil {
		t.Fatal(err)
	}
	fcmMultikeyStep(t, engine, listener, fields, fcmSupportBean{TheString: "E1", IntPrimitive: 30}, [][]string{{"_30_postfix"}}, "E1/30")
}

// TestFromClauseMethodContextVariableParity mirrors
// EPLFromClauseMethodContextVariable: a pattern-initiated context holds one
// postfix variable per partition, the method source resolves it per
// partition, and an in-context on-set statement updates only the owning
// partition. The trailing build-error assertions mirror the invalid
// context-variable uses exercised by the same Java execution.
func TestFromClauseMethodContextVariableParity(t *testing.T) {
	env := newFCMEnvironment(t)
	if _, err := RegisterStruct[fcmSupportBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[fcmSupportBeanS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[fcmSupportBeanS2](env, "SupportBean_S2"); err != nil {
		t.Fatal(err)
	}
	start := PatternFrom(From[fcmSupportBeanS0](env, "SupportBean_S0"), "c_s0", Literal(true))
	end := PatternFrom(From[fcmSupportBeanS1](env, "SupportBean_S1"), "c_s1",
		Equal[int](Field[fcmSupportBeanS1, int]("id"), TagField[int]("c_s0", "id")))
	// Esper's initiated-by-event context allocates one partition per
	// initiating event, hence the overlapping form.
	if _, err := CreateOverlappingPatternInitiatedTerminatedContext(env, "MyContext", start, end); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterContextVariable("MyContext", "postfix", "context_postfix"); err != nil {
		t.Fatal(err)
	}

	schema := fcmMapSchema(t, env, "FCMFetchABeanRow", FieldDef("id", reflect.TypeOf("")))
	sb := From[fcmSupportBean](env, "SupportBean").
		Filter(Equal[int](Field[fcmSupportBean, int]("intPrimitive"), ContextPatternField[int]("c_s0", "id")))
	method := FromMethod[map[string]any](env, "h0", schema, makeFCMFetchABeanProvider(schema, "postfix"))
	joinPlan, err := env.Build(Join(sb, method).
		Select(SelectRight("c0", Field[map[string]any, string]("id"))).
		Query(StatementName("s0"), WithContext("MyContext")))
	if err != nil {
		t.Fatal(err)
	}
	setterPlan, err := env.Build(OnEvent(From[fcmSupportBeanS2](env, "SupportBean_S2").
		Filter(Equal[int](Field[fcmSupportBeanS2, int]("id"), ContextPatternField[int]("c_s0", "id")))).
		SetVariable("postfix", Field[fcmSupportBeanS2, string]("p20")).
		Query(StatementName("s2-set"), WithContext("MyContext")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	t.Cleanup(func() { _ = engine.Close(context.Background()) })
	if _, err := engine.Deploy(context.Background(), setterPlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), joinPlan)
	if err != nil {
		t.Fatal(err)
	}
	listener := fcmOuterSubscribe(t, deployment.Statements()[0])
	fields := []string{"c0"}

	if err := engine.SendEvent(context.Background(), fcmSupportBeanS0{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), fcmSupportBeanS0{ID: 2}); err != nil {
		t.Fatal(err)
	}
	fcmMultikeyStep(t, engine, listener, fields, fcmSupportBean{TheString: "E1", IntPrimitive: 1}, [][]string{{"_1_context_postfix"}}, "E1/1 initial")
	fcmMultikeyStep(t, engine, listener, fields, fcmSupportBean{TheString: "E2", IntPrimitive: 2}, [][]string{{"_2_context_postfix"}}, "E2/2 initial")

	if err := engine.SendEvent(context.Background(), fcmSupportBeanS2{ID: 1, P20: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), fcmSupportBeanS2{ID: 2, P20: "b"}); err != nil {
		t.Fatal(err)
	}
	fcmMultikeyStep(t, engine, listener, fields, fcmSupportBean{TheString: "E1", IntPrimitive: 1}, [][]string{{"_1_a"}}, "E1/1 updated")
	fcmMultikeyStep(t, engine, listener, fields, fcmSupportBean{TheString: "E2", IntPrimitive: 2}, [][]string{{"_2_b"}}, "E2/2 updated")

	// Invalid uses: a context variable may only be referenced inside its own
	// context (mirrors the tryInvalidCompile pair in the Java execution).
	outside := Select(
		From[fcmSupportBean](env, "SupportBean"),
		Alias("c0", VariableRef[string]("postfix")),
	).Query(StatementName("bad-no-context"))
	if _, err := env.Build(outside); err == nil || !strings.Contains(err.Error(), "can only be accessed within context") {
		t.Fatalf("no-context build error = %v", err)
	}
	if _, err := CreateKeyContext(env, "ABC", Field[fcmSupportBean, string]("theString")); err != nil {
		t.Fatal(err)
	}
	wrongContext := Select(
		From[fcmSupportBean](env, "SupportBean"),
		Alias("c0", VariableRef[string]("postfix")),
	).Query(StatementName("bad-wrong-context"), WithContext("ABC"))
	if _, err := env.Build(wrongContext); err == nil || !strings.Contains(err.Error(), "belongs to context") {
		t.Fatalf("wrong-context build error = %v", err)
	}
}

// TestFromClauseMethodVariableMapAndOAParity mirrors
// EPLFromClauseMethodVariableMapAndOA: a triggerless variable-backed method
// source exposes its rows through the iterator. The map-shaped and
// object-array-shaped Java handlers both materialize as rows
// {field1: "a", field2: "b"}.
func TestFromClauseMethodVariableMapAndOAParity(t *testing.T) {
	for _, variant := range []string{"map", "oa"} {
		t.Run(variant, func(t *testing.T) {
			env := newFCMEnvironment(t)
			schema := fcmMapSchema(t, env, "FCMHandlerRow-"+variant,
				FieldDef("field1", reflect.TypeOf("")),
				FieldDef("field2", reflect.TypeOf("")))
			provider := MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
				return newEventsFrom(schema, []map[string]any{{"field1": "a", "field2": "b"}}, request.Now)
			})
			query := Select(
				FromMethod[map[string]any](env, "h0", schema, provider),
				Alias("field1", Field[map[string]any, string]("field1")),
				Alias("field2", Field[map[string]any, string]("field2")),
			).Query(StatementName("s0"))
			engine, stmt, _ := fcmOuterDeploy(t, env, query)
			_ = engine
			fcmOuterAssertRows(t, fcmOuterSnapshot(t, stmt), []string{"field1", "field2"}, [][]string{{"a", "b"}}, variant+" iterator")
		})
	}
}
