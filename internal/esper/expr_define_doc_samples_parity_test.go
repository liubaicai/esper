package esper

import (
	"context"
	"math"
	"reflect"
	"testing"
	"time"
)

// TestExprDefineDocSamplesParity covers the two declaration examples from
// Java's ExprDefineAliasFor.ExprDefineDocSamples. The Java execution only
// verifies that each declaration, schema and consuming statement compiles and
// deploys; it does not send events.
func TestExprDefineDocSamplesParity(t *testing.T) {
	t.Run("constant", testExprDefineDocSamplesConstant)
	t.Run("aggregate", testExprDefineDocSamplesAggregate)
}

func testExprDefineDocSamplesConstant(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterMap(env, "SampleEvent", nil, AllowDynamicFields())
	if err != nil {
		t.Fatal(err)
	}
	if schema.Name() != "SampleEvent" || len(schema.Fields()) != 0 || !schema.AllowsDynamicProperties() {
		t.Fatalf("SampleEvent schema = %#v, want empty dynamic schema", schema)
	}
	if err := env.DefineExpression("twoPI", Literal(math.Pi*2)); err != nil {
		t.Fatal(err)
	}
	definition, ok := env.Expression("twoPI")
	if !ok || len(definition.Parameters) != 0 || definition.Expr.Type() != reflect.TypeOf(float64(0)) {
		t.Fatalf("twoPI definition = %#v, want zero-parameter float64", definition)
	}

	plan, err := env.Build(FromAny(env, "SampleEvent").Select(
		Alias("twoPI", ExpressionRef[float64](env, "twoPI")),
	).Query(StatementName("expr-define-doc-two-pi")))
	if err != nil {
		t.Fatal(err)
	}
	assertExprDefineDocResultField(t, plan, "twoPI", reflect.TypeOf(float64(0)))
	deployExprDefineDocPlan(t, env, plan)
}

func testExprDefineDocSamplesAggregate(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterMap(env, "EnterRoomEvent", nil, AllowDynamicFields())
	if err != nil {
		t.Fatal(err)
	}
	if schema.Name() != "EnterRoomEvent" || len(schema.Fields()) != 0 || !schema.AllowsDynamicProperties() {
		t.Fatalf("EnterRoomEvent schema = %#v, want empty dynamic schema", schema)
	}
	if err := env.DefineExpression("countPeople", CountAll()); err != nil {
		t.Fatal(err)
	}
	definition, ok := env.Expression("countPeople")
	if !ok || len(definition.Parameters) != 0 || definition.Expr.Type() != reflect.TypeOf(int64(0)) {
		t.Fatalf("countPeople definition = %#v, want zero-parameter int64", definition)
	}

	countPeople := ExpressionRef[int64](env, "countPeople")
	plan, err := env.Build(FromAny(env, "EnterRoomEvent").Window(TimeWindow(10 * time.Second)).Aggregate(
		Alias("countPeople", countPeople),
	).Having(Greater[int64](countPeople, Literal(int64(10)))).Query(
		StatementName("expr-define-doc-count-people"),
	))
	if err != nil {
		t.Fatal(err)
	}
	assertExprDefineDocResultField(t, plan, "countPeople", reflect.TypeOf(int64(0)))
	deployExprDefineDocPlan(t, env, plan)
}

func assertExprDefineDocResultField(t *testing.T, plan Plan, name string, want reflect.Type) {
	t.Helper()
	schema, ok := plan.ResultSchema()
	if !ok {
		t.Fatalf("result schema is missing for %q", name)
	}
	field, ok := schema.Field(name)
	if !ok || field.Type != want {
		t.Fatalf("result field %q = %#v, want %v", name, field, want)
	}
}

func deployExprDefineDocPlan(t *testing.T, env *Environment, plan Plan) {
	t.Helper()
	engine := NewEngine(env)
	deployed, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		_ = engine.Close(context.Background())
		t.Fatal(err)
	}
	if len(deployed.Statements()) != 1 {
		_ = deployed.Undeploy(context.Background())
		_ = engine.Close(context.Background())
		t.Fatalf("deployed statement count = %d, want 1", len(deployed.Statements()))
	}
	if err := deployed.Undeploy(context.Background()); err != nil {
		_ = engine.Close(context.Background())
		t.Fatal(err)
	}
	if err := engine.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}
