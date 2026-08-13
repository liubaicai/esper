package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestExprDefineAliasForAliasAggregationParity covers Java's
// ExprDefineAliasAggregation. The declared expression is an aggregate itself,
// and its reference is reused in the projection just like total and total+1
// in the Java statement.
func TestExprDefineAliasForAliasAggregationParity(t *testing.T) {
	env, engine := defineBasicEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	if err := env.DefineExpression("total", Sum[int](Field[any, int]("intPrimitive"))); err != nil {
		t.Fatal(err)
	}
	definition, ok := env.Expression("total")
	if !ok || len(definition.Parameters) != 0 || definition.Expr.Type() != reflect.TypeOf(0) {
		t.Fatalf("total definition = %#v, want zero-parameter int aggregate", definition)
	}

	total := ExpressionRef[int](env, "total")
	plan, err := env.Build(From[defineBasicBean](env, "SupportBean").Aggregate(
		Alias("total", total),
		Alias("total+1", Add[int](total, Literal(1))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("alias aggregation result schema is missing")
	}
	for _, name := range []string{"total", "total+1"} {
		field, exists := resultSchema.Field(name)
		if !exists || field.Type != reflect.TypeOf(0) {
			t.Fatalf("alias aggregation result field %q = %#v, want int", name, field)
		}
	}

	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	rows := collectDefineRows(t, deployment)
	if err := engine.SendEvent(context.Background(), defineBasicBean{TheString: "E1", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 {
		t.Fatalf("alias aggregation rows = %#v, want one row", *rows)
	}
	if got := (*rows)[0].Get("total").Any(); got != 10 {
		t.Fatalf("alias aggregation total = %#v, want 10", got)
	}
	if got := (*rows)[0].Get("total+1").Any(); got != 11 {
		t.Fatalf("alias aggregation total+1 = %#v, want 11", got)
	}
}

// TestExprDefineAliasForInvalidParity covers the invalid AliasFor cases that
// have typed equivalents: declaration-body field validation and named
// expression argument-count validation. EPL keyword and script parse errors
// have no corresponding fluent AST node.
func TestExprDefineAliasForInvalidParity(t *testing.T) {
	t.Run("unknown declaration field", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[defineBasicBean](env, "SupportBean"); err != nil {
			t.Fatal(err)
		}
		if err := env.DefineExpression("total", Sum[int](Field[any, int]("xxx"))); err != nil {
			t.Fatal(err)
		}
		ref := ExpressionRef[int](env, "total")
		_, err := env.Build(Select(From[defineBasicBean](env, "SupportBean"),
			Alias("total+1", Add[int](ref, Literal(1))),
		).Query(StatementName("invalid-unknown-field")))
		if err == nil || !strings.Contains(err.Error(), `unknown field "xxx"`) {
			t.Fatalf("unknown declaration field error = %v, want unknown field", err)
		}
		if !errors.Is(err, ErrorInvalidRule) {
			t.Fatalf("unknown declaration field error code = %v, want ErrorInvalidRule", err)
		}
	})

	t.Run("too many arguments", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[defineBasicBean](env, "SupportBean"); err != nil {
			t.Fatal(err)
		}
		if err := env.DefineExpression("total", Literal(1)); err != nil {
			t.Fatal(err)
		}
		ref := ExpressionRef[int](env, "total", Literal(1))
		_, err := env.Build(Select(From[defineBasicBean](env, "SupportBean"),
			Alias("total", ref),
		).Query(StatementName("invalid-too-many-arguments")))
		if err == nil || !errors.Is(err, ErrorInvalidRule) || !strings.Contains(err.Error(), "expects 0 arguments, received 1") {
			t.Fatalf("too many arguments error = %v, want arity validation", err)
		}
	})

	t.Run("missing arguments", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[defineBasicBean](env, "SupportBean"); err != nil {
			t.Fatal(err)
		}
		parameter := ExpressionParam[int]("a")
		if err := env.DefineExpression("total", parameter); err != nil {
			t.Fatal(err)
		}
		ref := ExpressionRef[int](env, "total")
		_, err := env.Build(Select(From[defineBasicBean](env, "SupportBean"),
			Alias("total", ref),
		).Query(StatementName("invalid-missing-arguments")))
		if err == nil || !errors.Is(err, ErrorInvalidRule) || !strings.Contains(err.Error(), "expects 1 arguments, received 0") {
			t.Fatalf("missing arguments error = %v, want arity validation", err)
		}
	})
}
