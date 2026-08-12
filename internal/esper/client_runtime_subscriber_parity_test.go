package esper

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type clientRuntimeSubscriberBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type clientRuntimeSubscriberMarket struct {
	Symbol string `esper:"symbol"`
}

type clientRuntimeSubscriberWindowRow struct {
	Key   string `esper:"key"`
	Value int    `esper:"value"`
}

func newClientRuntimeSubscriberEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientRuntimeSubscriberBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[clientRuntimeSubscriberMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func deployClientRuntimeSubscriber(t *testing.T, engine *Engine, plan Plan) (*Deployment, *Statement) {
	t.Helper()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		t.Fatalf("subscriber deployment statements = %d, want 1", len(statements))
	}
	return deployment, statements[0]
}

func subscriberRowAnyValues(row SubscriberRow) []any {
	values := row.Values()
	result := make([]any, len(values))
	for index, value := range values {
		if value.IsPresent() {
			result[index] = value.Any()
		}
	}
	return result
}

func TestClientRuntimeSubscriberBindingsParity(t *testing.T) {
	t.Run("wildcard-underlying-and-map", func(t *testing.T) {
		env := newClientRuntimeSubscriberEnv(t)
		query := From[clientRuntimeSubscriberBean](env, "SupportBean").
			Filter(Equal[string](Field[clientRuntimeSubscriberBean, string]("theString"), Literal("E2"))).
			Query(StatementName("wildcard"))
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		_, statement := deployClientRuntimeSubscriber(t, engine, plan)
		var updates []SubscriberUpdate
		if err := statement.SetSubscriber(func(_ context.Context, update SubscriberUpdate) error {
			updates = append(updates, update)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		unmatched := &clientRuntimeSubscriberBean{TheString: "E1", IntPrimitive: 1}
		matched := &clientRuntimeSubscriberBean{TheString: "E2", IntPrimitive: 2}
		if err := engine.SendEvent(context.Background(), unmatched); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), matched); err != nil {
			t.Fatal(err)
		}
		if len(updates) != 1 || updates[0].Statement != statement || len(updates[0].NewRows()) != 1 {
			t.Fatalf("wildcard subscriber updates = %#v", updates)
		}
		row := updates[0].NewRows()[0]
		if row.Underlying() != matched {
			t.Fatalf("wildcard underlying = %#v, want original event", row.Underlying())
		}
		if values := subscriberRowAnyValues(row); len(values) != 1 || values[0] != matched {
			t.Fatalf("wildcard values = %#v", values)
		}
		if got := row.Map(); !reflect.DeepEqual(got, map[string]any{"theString": "E2", "intPrimitive": 2}) {
			t.Fatalf("wildcard map = %#v", got)
		}
	})

	t.Run("ordered-projection-and-ir-batch", func(t *testing.T) {
		env := newClientRuntimeSubscriberEnv(t)
		stream := From[clientRuntimeSubscriberBean](env, "SupportBean").Window(LengthBatch(2))
		query := Select(stream,
			Alias("theString", Field[clientRuntimeSubscriberBean, string]("theString")),
			Alias("intPrimitive", Field[clientRuntimeSubscriberBean, int]("intPrimitive")),
			Alias("plusTwo", Add[int](Field[clientRuntimeSubscriberBean, int]("intPrimitive"), Literal(2))),
			Alias("nullValue", NullLiteral[string]()),
		).Query(StatementName("projection"), WithOldStream())
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		_, statement := deployClientRuntimeSubscriber(t, engine, plan)
		var updates []SubscriberUpdate
		if err := statement.SetSubscriber(func(_ context.Context, update SubscriberUpdate) error {
			updates = append(updates, update)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		for index := 1; index <= 4; index++ {
			if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberBean{TheString: fmt.Sprintf("E%d", index), IntPrimitive: index}); err != nil {
				t.Fatal(err)
			}
		}
		if len(updates) != 2 || len(updates[0].NewRows()) != 2 || len(updates[0].OldRows()) != 0 || len(updates[1].NewRows()) != 2 || len(updates[1].OldRows()) != 2 {
			t.Fatalf("projection subscriber boundaries = %#v", updates)
		}
		if got := subscriberRowAnyValues(updates[0].NewRows()[0]); !reflect.DeepEqual(got, []any{"E1", 1, 3, nil}) {
			t.Fatalf("ordered projection values = %#v", got)
		}
		if got := updates[1].OldRows()[0].Map(); !reflect.DeepEqual(got, map[string]any{"theString": "E1", "intPrimitive": 1, "plusTwo": 3, "nullValue": nil}) {
			t.Fatalf("old projection map = %#v", got)
		}
	})
}

func TestClientRuntimeSubscriberSubscriberAndListenerParity(t *testing.T) {
	env := newClientRuntimeSubscriberEnv(t)
	plan, err := env.Build(Select(From[clientRuntimeSubscriberBean](env, "SupportBean"),
		Alias("theString", Field[clientRuntimeSubscriberBean, string]("theString")),
		Alias("intPrimitive", Field[clientRuntimeSubscriberBean, int]("intPrimitive")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	_, statement := deployClientRuntimeSubscriber(t, engine, plan)
	var subscriberCalls, listenerCalls int
	if err := statement.SetSubscriber(func(_ context.Context, update SubscriberUpdate) error {
		subscriberCalls++
		if update.Statement != statement || update.NewRows()[0].Get("theString").Any() != "E1" {
			t.Fatalf("subscriber update = %#v", update)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		listenerCalls++
		if batch.New[0].Get("intPrimitive").Any() != 1 {
			t.Fatalf("listener batch = %#v", batch)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if subscriberCalls != 1 || listenerCalls != 1 {
		t.Fatalf("subscriber/listener calls = %d/%d, want 1/1", subscriberCalls, listenerCalls)
	}
}

func TestClientRuntimeSubscriberBindWildcardJoinParity(t *testing.T) {
	env := newClientRuntimeSubscriberEnv(t)
	left := From[clientRuntimeSubscriberBean](env, "SupportBean").Window(KeepAll())
	right := From[clientRuntimeSubscriberMarket](env, "SupportMarketDataBean").Window(KeepAll())
	query := Join(left, right, OnSourcesEqual(
		0, Field[clientRuntimeSubscriberBean, string]("theString"),
		1, Field[clientRuntimeSubscriberMarket, string]("symbol"),
	)).Select(
		SelectSourceEvent(0, "s0"),
		SelectSourceEvent(1, "s1"),
	).Query(StatementName("join"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	_, statement := deployClientRuntimeSubscriber(t, engine, plan)
	var rows []SubscriberRow
	if err := statement.SetSubscriber(func(_ context.Context, update SubscriberUpdate) error {
		rows = append(rows, update.NewRows()...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	bean := &clientRuntimeSubscriberBean{TheString: "E1", IntPrimitive: 100}
	market := &clientRuntimeSubscriberMarket{Symbol: "E1"}
	if err := engine.SendEvent(context.Background(), bean); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), market); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("wildcard join rows = %d, want 1", len(rows))
	}
	if got := subscriberRowAnyValues(rows[0]); !reflect.DeepEqual(got, []any{bean, market}) {
		t.Fatalf("wildcard join values = %#v", got)
	}
}

func TestClientRuntimeSubscriberInvocationTargetExceptionParity(t *testing.T) {
	env := newClientRuntimeSubscriberEnv(t)
	plan, err := env.Build(From[clientRuntimeSubscriberMarket](env, "SupportMarketDataBean").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	_, statement := deployClientRuntimeSubscriber(t, engine, plan)
	var failures []SubscriberError
	engine.SetSubscriberErrorHandler(func(_ context.Context, failure SubscriberError) {
		failures = append(failures, failure)
	})
	listenerCalls := 0
	if _, err := statement.Subscribe(func(context.Context, ResultBatch) error {
		listenerCalls++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := statement.SetSubscriber(func(context.Context, SubscriberUpdate) error {
		return errors.New("subscriber-returned-error")
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberMarket{Symbol: "IBM"}); err != nil {
		t.Fatalf("subscriber error escaped SendEvent: %v", err)
	}
	if err := statement.SetSubscriber(func(context.Context, SubscriberUpdate) error {
		panic("subscriber-panic")
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberMarket{Symbol: "MSFT"}); err != nil {
		t.Fatalf("subscriber panic escaped SendEvent: %v", err)
	}
	if listenerCalls != 2 || len(failures) != 2 || failures[0].Cause == nil || failures[1].Panic != "subscriber-panic" || len(failures[1].Stack) == 0 {
		t.Fatalf("isolated subscriber failures = %#v, listener calls %d", failures, listenerCalls)
	}
}

func TestClientRuntimeSubscriberNamedWindowParity(t *testing.T) {
	env := newClientRuntimeSubscriberEnv(t)
	windowSchema, err := RegisterStruct[clientRuntimeSubscriberWindowRow](env, "MyWindowSchema")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindow", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnEvent(From[clientRuntimeSubscriberBean](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindow",
		SetColumn("key", Field[clientRuntimeSubscriberBean, string]("theString")),
		SetColumn("value", Field[clientRuntimeSubscriberBean, int]("intPrimitive")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[clientRuntimeSubscriberMarket](env, "SupportMarketDataBean")).DeleteFromNamedWindow(
		"MyWindow",
		Equal[string](NamedWindowField[string]("key"), Field[clientRuntimeSubscriberMarket, string]("symbol")),
	).Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}
	selectPlan, err := env.Build(OnEvent(From[clientRuntimeSubscriberMarket](env, "SupportMarketDataBean")).SelectFromNamedWindow(
		"MyWindow", nil,
		Alias("key", NamedWindowField[string]("key")),
		Alias("value", NamedWindowField[int]("value")),
	).Query(StatementName("select")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	_, insertStatement := deployClientRuntimeSubscriber(t, engine, insertPlan)
	_, deleteStatement := deployClientRuntimeSubscriber(t, engine, deletePlan)
	_, selectStatement := deployClientRuntimeSubscriber(t, engine, selectPlan)
	captured := make(map[string][]map[string]any)
	for name, statement := range map[string]*Statement{"insert": insertStatement, "delete": deleteStatement, "select": selectStatement} {
		name := name
		if err := statement.SetSubscriber(func(_ context.Context, update SubscriberUpdate) error {
			for _, row := range update.NewRows() {
				captured[name] = append(captured[name], row.Map())
			}
			for _, row := range update.OldRows() {
				captured[name] = append(captured[name], row.Map())
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberMarket{Symbol: "E1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberBean{TheString: "E2", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberMarket{Symbol: "M1"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"insert", "delete", "select"} {
		if len(captured[name]) == 0 {
			t.Fatalf("named-window subscriber %s received no rows: %#v", name, captured)
		}
	}
	if !reflect.DeepEqual(captured["select"][len(captured["select"])-1], map[string]any{"key": "E2", "value": 2}) {
		t.Fatalf("named-window select rows = %#v", captured["select"])
	}
}

func TestClientRuntimeSubscriberStartStopStatementParity(t *testing.T) {
	env := newClientRuntimeSubscriberEnv(t)
	plan, err := env.Build(From[clientRuntimeSubscriberBean](env, "SupportBean").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, statement := deployClientRuntimeSubscriber(t, engine, plan)
	var values []string
	subscriber := Subscriber(func(_ context.Context, update SubscriberUpdate) error {
		values = append(values, update.NewRows()[0].Get("theString").Any().(string))
		return nil
	})
	if err := statement.SetSubscriber(subscriber); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberBean{TheString: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberBean{TheString: "B"}); err != nil {
		t.Fatal(err)
	}
	_, redeployed := deployClientRuntimeSubscriber(t, engine, plan)
	if err := redeployed.SetSubscriber(subscriber); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberBean{TheString: "C"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values, []string{"A", "C"}) {
		t.Fatalf("subscriber lifecycle values = %#v, want A,C", values)
	}
	if err := statement.SetSubscriber(subscriber); !errors.Is(err, ErrorState) {
		t.Fatalf("destroyed statement SetSubscriber error = %v, want %s", err, ErrorState)
	}
}

func TestClientRuntimeSubscriberVariablesParity(t *testing.T) {
	env := newClientRuntimeSubscriberEnv(t)
	if err := env.RegisterVariable("myvar", "abc"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(OnEvent(From[clientRuntimeSubscriberBean](env, "SupportBean")).SetVariables(
		SetVariableExpr("myvar", Field[clientRuntimeSubscriberBean, string]("theString")),
	).Query(StatementName("set-variable")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	_, statement := deployClientRuntimeSubscriber(t, engine, plan)
	var rows []map[string]any
	if err := statement.SetSubscriber(func(_ context.Context, update SubscriberUpdate) error {
		for _, row := range update.NewRows() {
			rows = append(rows, row.Map())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberBean{TheString: "def"}); err != nil {
		t.Fatal(err)
	}
	value, ok := engine.GetVariable("myvar")
	if !ok {
		t.Fatal("variable myvar is missing")
	}
	if value.Any() != "def" || len(rows) != 1 || rows[0]["myvar"] != "def" {
		t.Fatalf("variable subscriber rows = %#v variable=%#v", rows, value)
	}
}

func TestClientRuntimeSubscriberSimpleSelectUpdateOnlyParity(t *testing.T) {
	env := newClientRuntimeSubscriberEnv(t)
	plan, err := env.Build(Select(From[clientRuntimeSubscriberBean](env, "SupportBean").Window(LastEvent()),
		Alias("theString", Field[clientRuntimeSubscriberBean, string]("theString")),
		Alias("intPrimitive", Field[clientRuntimeSubscriberBean, int]("intPrimitive")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	_, statement := deployClientRuntimeSubscriber(t, engine, plan)
	var first, second []string
	if err := statement.SetSubscriber(func(_ context.Context, update SubscriberUpdate) error {
		first = append(first, update.NewRows()[0].Get("theString").Any().(string))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	listenerCalls := 0
	subscription, err := statement.Subscribe(func(context.Context, ResultBatch) error {
		listenerCalls++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if err := subscription.Close(); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberBean{TheString: "E2", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if err := statement.SetSubscriber(func(_ context.Context, update SubscriberUpdate) error {
		second = append(second, update.NewRows()[0].Get("theString").Any().(string))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberBean{TheString: "E3", IntPrimitive: 3}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, []string{"E1", "E2"}) || !reflect.DeepEqual(second, []string{"E3"}) || listenerCalls != 1 {
		t.Fatalf("subscriber replacement first=%#v second=%#v listener=%d", first, second, listenerCalls)
	}
}

func TestClientRuntimeSubscriberPerformanceSyntheticUndeliveredParity(t *testing.T) {
	env := newClientRuntimeSubscriberEnv(t)
	plan, err := env.Build(From[clientRuntimeSubscriberBean](env, "SupportBean").Filter(
		Greater[int](Field[clientRuntimeSubscriberBean, int]("intPrimitive"), Literal(10)),
	).Query(StatementName("undelivered")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployClientRuntimeSubscriber(t, engine, plan)
	for index := 0; index < 1000; index++ {
		if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberBean{TheString: "E1", IntPrimitive: 1000 + index}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestClientRuntimeSubscriberPerformanceSyntheticParity(t *testing.T) {
	env := newClientRuntimeSubscriberEnv(t)
	plan, err := env.Build(From[clientRuntimeSubscriberBean](env, "SupportBean").Filter(
		Greater[int](Field[clientRuntimeSubscriberBean, int]("intPrimitive"), Literal(10)),
	).Query(StatementName("delivered")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	_, statement := deployClientRuntimeSubscriber(t, engine, plan)
	count := 0
	if err := statement.SetSubscriber(func(_ context.Context, update SubscriberUpdate) error {
		count += len(update.NewRows())
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 1000; index++ {
		if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberBean{TheString: "E1", IntPrimitive: 1000 + index}); err != nil {
			t.Fatal(err)
		}
	}
	if count != 1000 {
		t.Fatalf("subscriber delivered count = %d, want 1000", count)
	}
}

func TestClientRuntimeSubscriberAddRemoveParity(t *testing.T) {
	env := newClientRuntimeSubscriberEnv(t)
	plan, err := env.Build(From[clientRuntimeSubscriberBean](env, "SupportBean").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	_, statement := deployClientRuntimeSubscriber(t, engine, plan)
	count := 0
	if err := statement.SetSubscriber(func(context.Context, SubscriberUpdate) error {
		count++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !statement.HasSubscriber() {
		t.Fatal("statement does not report installed subscriber")
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberBean{}); err != nil {
		t.Fatal(err)
	}
	if err := statement.SetSubscriber(nil); err != nil {
		t.Fatal(err)
	}
	if statement.HasSubscriber() {
		t.Fatal("nil subscriber did not remove installation")
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberBean{}); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("subscriber calls after removal = %d, want 1", count)
	}
}

func TestClientRuntimeSubscriberDisallowedParity(t *testing.T) {
	env := newClientRuntimeSubscriberEnv(t)
	allowedPlan, err := env.Build(From[clientRuntimeSubscriberBean](env, "SupportBean").Query(StatementName("allowed")))
	if err != nil {
		t.Fatal(err)
	}
	disallowedPlan, err := env.Build(From[clientRuntimeSubscriberBean](env, "SupportBean").Query(
		StatementName("disallowed"),
		DisallowSubscriber(),
	))
	if err != nil {
		t.Fatal(err)
	}
	if allowedPlan.Hash() == disallowedPlan.Hash() {
		t.Fatal("subscriber policy is missing from canonical plan identity")
	}
	engine := NewEngine(env)
	_, statement := deployClientRuntimeSubscriber(t, engine, disallowedPlan)
	if err := statement.SetSubscriber(func(context.Context, SubscriberUpdate) error { return nil }); !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("disallowed subscriber error = %v, want %s", err, ErrorInvalidRule)
	}
	listenerCalls := 0
	if _, err := statement.Subscribe(func(context.Context, ResultBatch) error {
		listenerCalls++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeSubscriberBean{}); err != nil {
		t.Fatal(err)
	}
	if listenerCalls != 1 {
		t.Fatalf("disallow subscriber also blocked listener: calls=%d", listenerCalls)
	}
}
