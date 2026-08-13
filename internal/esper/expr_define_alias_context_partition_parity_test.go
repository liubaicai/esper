package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type exprDefineAliasContextBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestExprDefineAliasForContextPartitionParity covers Java's
// ExprDefineContextPartition. A one-shot timer context starts at the virtual
// deployment time, and the zero-parameter declaration filters events inside
// that active partition.
func TestExprDefineAliasForContextPartitionParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprDefineAliasContextBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}

	predicate := And(
		Equal[string](Field[exprDefineAliasContextBean, string]("theString"), Literal("a")),
		Equal[int](Field[exprDefineAliasContextBean, int]("intPrimitive"), Literal(1)),
	)
	if err := env.DefineExpression("the_expr", predicate); err != nil {
		t.Fatal(err)
	}
	definition, ok := env.Expression("the_expr")
	if !ok || len(definition.Parameters) != 0 {
		t.Fatalf("context alias definition = %#v, want zero parameters", definition)
	}

	origin := time.Unix(0, 0).UTC()
	base := From[exprDefineAliasContextBean](env, "SupportBean")
	if _, err := CreatePatternInitiatedTerminatedContext(
		env,
		"the_context",
		TimerAt(base, origin),
		TimerInterval(base, 10*time.Minute),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(base.Filter(ExpressionRef[bool](env, "the_expr")),
		Alias("theString", Field[exprDefineAliasContextBean, string]("theString")),
		Alias("intPrimitive", Field[exprDefineAliasContextBean, int]("intPrimitive")),
	).Query(StatementName("s0"), WithContext("the_context")))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("context alias result schema is missing")
	}
	if field, ok := resultSchema.Field("theString"); !ok || field.Type != reflect.TypeOf("") {
		t.Fatalf("context alias string field = %#v, want string", field)
	}
	if field, ok := resultSchema.Field("intPrimitive"); !ok || field.Type != reflect.TypeOf(0) {
		t.Fatalf("context alias int field = %#v, want int", field)
	}

	engine := NewEngine(env, WithStartTime(origin))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	rows := collectDotRows(t, deployment)

	if err := engine.AdvanceTime(context.Background(), origin); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("the_context"); err != nil || count != 1 {
		t.Fatalf("context alias active partitions = %d, err=%v, want 1", count, err)
	}
	if err := engine.SendEvent(context.Background(), exprDefineAliasContextBean{TheString: "a", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 || (*rows)[0].Get("theString").Any() != "a" || (*rows)[0].Get("intPrimitive").Any() != 1 {
		t.Fatalf("context alias rows = %#v, want a/1", *rows)
	}
	if err := engine.SendEvent(context.Background(), exprDefineAliasContextBean{TheString: "b", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 {
		t.Fatalf("context alias nonmatch emitted row: %#v", *rows)
	}

	if err := engine.AdvanceTime(context.Background(), origin.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("the_context"); err != nil || count != 0 {
		t.Fatalf("context alias partitions after termination = %d, err=%v, want 0", count, err)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(20*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("the_context"); err != nil || count != 0 {
		t.Fatalf("context alias one-shot partition reopened: count=%d err=%v", count, err)
	}
}
