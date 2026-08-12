package esper

import (
	"context"
	"reflect"
	"testing"
)

func TestCurrentEvaluationContextMatchesJavaMetadataContract(t *testing.T) {
	env, _ := newRuntimeTest(t)
	stream := From[runtimeTestTrade](env, "Trade")
	plan, err := env.Build(Select(stream,
		Alias("ctx", CurrentEvaluationContext()),
		Alias("ctxAgain", CurrentEvaluationContext()),
	).Query(
		StatementName("s0"),
		WithStatementUserObject("my_user_object"),
	))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("evaluation context result schema is missing")
	}
	field, ok := resultSchema.Field("ctx")
	if !ok || field.Type != reflect.TypeOf(ExpressionEvaluationContext{}) {
		t.Fatalf("evaluation context result type = %#v, want %v", field, reflect.TypeOf(ExpressionEvaluationContext{}))
	}

	engine := NewEngine(env, WithRuntimeURI("runtime-test"))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var row Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) == 0 {
			return nil
		}
		var ok bool
		row, ok = batch.New[0].Row()
		if !ok {
			return NewError(ErrorTypeMismatch, "evaluation context result is not a row")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "ESPER", Price: 1}); err != nil {
		t.Fatal(err)
	}

	ctx, err := As[ExpressionEvaluationContext](row.Get("ctx"))
	if err != nil {
		t.Fatalf("evaluation context = %v (%v)", row.Get("ctx"), err)
	}
	if ctx.RuntimeURI != "runtime-test" || ctx.StatementName != "s0" || ctx.ContextPartitionID != -1 || ctx.StatementUserObject != "my_user_object" {
		t.Fatalf("evaluation context metadata = %#v", ctx)
	}
	again, err := As[ExpressionEvaluationContext](row.Get("ctxAgain"))
	if err != nil || !reflect.DeepEqual(ctx, again) {
		t.Fatalf("repeated evaluation context = %#v (%v), first %#v", again, err, ctx)
	}
}

func TestCurrentEvaluationContextNormalizesDirectEvaluationPartitionID(t *testing.T) {
	got, err := As[ExpressionEvaluationContext](CurrentEvaluationContext().eval(EvalContext{}))
	if err != nil {
		t.Fatal(err)
	}
	if got.ContextPartitionID != -1 || got.RuntimeURI != "" || got.StatementName != "" {
		t.Fatalf("direct evaluation context = %#v", got)
	}
}
