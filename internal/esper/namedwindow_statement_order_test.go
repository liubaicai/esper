package esper

import (
	"context"
	"testing"
)

// TestNamedWindowStatementOutputPrecedesConsumers pins Esper's tail-view
// dispatch order for a named window: the window's own statement output (the
// direct create-window child) is delivered before the deferred consumer
// dispatches for the same window delta, on every delta kind — window inserts
// driven by an insert-into trigger and window rows removed by an on-delete
// trigger alike (InfraNamedWindowViews ordinals 39-42 and the checked-in Java
// trace of testdata/parity/named-window-mutation.evidence.json both record the
// window statement before the consumer).
func TestNamedWindowStatementOutputPrecedesConsumers(t *testing.T) {
	type bean struct {
		TheString string `esper:"theString"`
		LongBoxed int64  `esper:"longBoxed"`
	}
	type market struct {
		Symbol string `esper:"symbol"`
	}
	type kv struct {
		Key   string `esper:"key"`
		Value int64  `esper:"value"`
	}

	env := NewEnvironment()
	if _, err := RegisterStruct[bean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[market](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	schema, err := RegisterStruct[kv](env, "MyWindow")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	createPlan, err := env.Build(FromNamedWindow(env, "MyWindow").
		CreateNamedWindowQuery(StatementName("create"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnEvent(From[bean](env, "SupportBean")).InsertIntoNamedWindow("MyWindow",
		SetColumn("key", Field[bean, string]("theString")),
		SetColumn("value", Field[bean, int64]("longBoxed")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromNamedWindow(env, "MyWindow").Select(
		Alias("key", Field[any, string]("key")),
		Alias("value", Field[any, int64]("value")),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[market](env, "SupportMarketDataBean")).DeleteFromNamedWindow("MyWindow",
		Equal[string](NamedWindowField[string]("key"), Field[market, string]("symbol"))).
		Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	engine := NewEngine(env)
	defer func() { _ = engine.Close(ctx) }()
	var order []string
	deploy := func(plan Plan) {
		t.Helper()
		if _, err := engine.Deploy(ctx, plan); err != nil {
			t.Fatal(err)
		}
	}
	subscribe := func(plan Plan, label string) {
		t.Helper()
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			kind := "new"
			if len(batch.New) == 0 && len(batch.Old) > 0 {
				kind = "old"
			}
			order = append(order, label+"/"+kind)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	// The window statement and its consumer carry listeners, mirroring the
	// executions this test pins (the insert-into and on-delete statements are
	// unlistened there, so their own trigger output is not part of the order).
	subscribe(createPlan, "create")
	deploy(insertPlan)
	subscribe(consumerPlan, "s0")
	deploy(deletePlan)

	if err := engine.SendEvent(ctx, bean{TheString: "E1", LongBoxed: 10}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"create/new", "s0/new"}; !equalStrings(order, want...) {
		t.Fatalf("insert wave order = %v, want %v", order, want)
	}
	order = nil

	if err := engine.SendEvent(ctx, market{Symbol: "E1"}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"create/old", "s0/old"}; !equalStrings(order, want...) {
		t.Fatalf("delete wave order = %v, want %v", order, want)
	}
}
