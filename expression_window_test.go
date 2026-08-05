package esper

import (
	"context"
	"testing"
)

func TestExpressionWindowExpiresOldestUntilPredicateIsTrue(t *testing.T) {
	env, engine := newRuntimeTest(t)
	keep := LessOrEqual[int64](CountAll(), Literal(int64(2)))
	_, batches := deployViewTest(t, env, engine, From[runtimeTestTrade](env, "Trade").Window(ExpressionWindow(keep)), "expression-window")
	for _, symbol := range []string{"A", "B", "C"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if len(*batches) != 3 || len((*batches)[2].New) != 1 || len((*batches)[2].Old) != 1 {
		t.Fatalf("expression window batches = %#v", *batches)
	}
	old, ok := (*batches)[2].Old[0].Event()
	if !ok || old.Underlying().(runtimeTestTrade).Symbol != "A" {
		t.Fatalf("expression window old = %#v", old)
	}
}

func TestExpressionBatchWithAndWithoutTriggerEvent(t *testing.T) {
	env, engine := newRuntimeTest(t)
	trigger := GreaterOrEqual[int64](CountAll(), Literal(int64(3)))
	_, excluded := deployViewTest(t, env, engine, From[runtimeTestTrade](env, "Trade").Window(ExpressionBatch(trigger)), "expression-batch-excluded")
	for _, symbol := range []string{"A", "B", "C", "D"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if len(*excluded) != 1 || len((*excluded)[0].New) != 2 || len((*excluded)[0].Old) != 0 {
		t.Fatalf("excluded trigger batches = %#v", *excluded)
	}

	_, included := deployViewTest(t, env, engine, From[runtimeTestTrade](env, "Trade").Window(ExpressionBatch(trigger, IncludeTriggerEvent())), "expression-batch-included")
	for _, symbol := range []string{"A", "B", "C", "D", "E", "F"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if len(*included) != 2 || len((*included)[0].New) != 3 || len((*included)[1].New) != 3 || len((*included)[1].Old) != 3 {
		t.Fatalf("included trigger batches = %#v", *included)
	}
}

func TestExpressionWindowValidation(t *testing.T) {
	env, _ := newRuntimeTest(t)
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(ExpressionWindow(Literal(int64(1)))).Query()); err == nil {
		t.Fatal("expected expression window boolean validation error")
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(ExpressionBatch(Literal(int64(1)))).Query()); err == nil {
		t.Fatal("expected expression batch boolean validation error")
	}
}
