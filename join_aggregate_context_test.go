package esper

import (
	"context"
	"testing"
)

type joinUnidirectionalWhereRow struct {
	ID string `esper:"id"`
}

type joinLifecycleOrder struct {
	OrderID string `esper:"orderID"`
	Symbol  string `esper:"symbol"`
	Kind    string `esper:"kind"`
}

type joinLifecyclePayment struct {
	OrderID string  `esper:"orderID"`
	Amount  float64 `esper:"amount"`
}

func TestUnidirectionalFullOuterWhereFiltersCurrentDriverTuple(t *testing.T) {
	env := NewEnvironment()
	for _, name := range []string{"UniWhereA", "UniWhereB", "UniWhereC"} {
		if _, err := RegisterStruct[joinUnidirectionalWhereRow](env, name); err != nil {
			t.Fatal(err)
		}
	}
	rowID := func(source int) Expression[string] {
		return JoinField[string](source, "id")
	}
	predicate := Or(
		Or(
			Equal[string](rowID(0), Literal("YES")),
			Equal[string](rowID(1), Literal("YES")),
		),
		Equal[string](rowID(2), Literal("YES")),
	)
	plan, err := env.Build(JoinMany(
		JoinSource(From[joinUnidirectionalWhereRow](env, "UniWhereA")).Unidirectional(),
		JoinSource(From[joinUnidirectionalWhereRow](env, "UniWhereB")).Unidirectional(),
		JoinSource(From[joinUnidirectionalWhereRow](env, "UniWhereC")).Unidirectional(),
	).FullOuter().Select(
		SelectFrom(0, "a", rowID(0)),
		SelectFrom(1, "b", rowID(1)),
		SelectFrom(2, "c", rowID(2)),
	).Where(predicate).Query(StatementName("unidirectional-full-outer-where")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(source, id string) {
		t.Helper()
		if err := engine.Send(context.Background(), source, joinUnidirectionalWhereRow{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	send("UniWhereA", "NO")
	send("UniWhereB", "YES")
	send("UniWhereC", "NO")
	send("UniWhereC", "YES")
	if len(batches) != 2 || len(batches[0].New) != 1 || len(batches[1].New) != 1 {
		t.Fatalf("unidirectional full-outer where batches = %#v", batches)
	}
	for index, batch := range batches {
		row, ok := batch.New[0].Row()
		hasYes := ok && (row.Get("a").Any() == "YES" || row.Get("b").Any() == "YES" || row.Get("c").Any() == "YES")
		if !hasYes {
			t.Fatalf("unidirectional full-outer where row %d = %#v", index, batch.New)
		}
	}
}

func TestInitiatedTerminatedContextJoinAggregateResetsState(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinLifecycleOrder](env, "LifecycleJoinOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinLifecyclePayment](env, "LifecycleJoinPayment"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateInitiatedTerminatedContext(
		env,
		"lifecycle-join",
		Field[any, string]("orderID"),
		Equal[string](Field[joinLifecycleOrder, string]("kind"), Literal("start")),
		Equal[string](Field[joinLifecycleOrder, string]("kind"), Literal("end")),
	); err != nil {
		t.Fatal(err)
	}
	amount := JoinField[float64](1, "amount")
	plan, err := env.Build(Join(
		From[joinLifecycleOrder](env, "LifecycleJoinOrder").Window(KeepAll()),
		From[joinLifecyclePayment](env, "LifecycleJoinPayment").Window(KeepAll()),
		OnEqual(
			Field[joinLifecycleOrder, string]("orderID"),
			Field[joinLifecyclePayment, string]("orderID"),
		),
	).LeftOuter().GroupBy(JoinField[string](0, "symbol")).Select(
		Alias("symbol", JoinField[string](0, "symbol")),
		Alias("total", Sum[float64](amount)),
	).Where(
		GreaterOrEqual[float64](amount, Literal(1.0)),
	).Query(StatementName("lifecycle-join-aggregate"), WithContext("lifecycle-join"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendOrder := func(orderID, symbol, kind string) {
		t.Helper()
		if err := engine.Send(context.Background(), "LifecycleJoinOrder", joinLifecycleOrder{OrderID: orderID, Symbol: symbol, Kind: kind}); err != nil {
			t.Fatal(err)
		}
	}
	sendPayment := func(orderID string, amount float64) {
		t.Helper()
		if err := engine.Send(context.Background(), "LifecycleJoinPayment", joinLifecyclePayment{OrderID: orderID, Amount: amount}); err != nil {
			t.Fatal(err)
		}
	}

	sendOrder("O1", "A", "start")
	sendPayment("O1", 10)
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("first lifecycle join aggregate = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("symbol").Any() != "A" || row.Get("total").Any() != float64(10) {
		t.Fatalf("first lifecycle join row = %#v", row.AsMap())
	}

	// Termination removes the partition and its join/aggregate state. Events
	// received while it is inactive must not contribute to the next instance.
	sendOrder("O1", "", "end")
	if deployment.Statements()[0].ContextPartitionCount() != 0 {
		t.Fatalf("lifecycle join partition survived end: %v", deployment.Statements()[0].ContextPartitions())
	}
	sendPayment("O1", 20)
	sendOrder("O1", "A", "start")
	sendPayment("O1", 20)
	// The terminating event is still delivered to the active statement before
	// the partition is deallocated, so it may produce one final active-row
	// result. The subsequent start/payment pair must be the last batch.
	if len(batches) != 3 || len(batches[2].New) != 1 {
		t.Fatalf("second lifecycle join aggregate = %#v", batches)
	}
	row, ok = batches[2].New[0].Row()
	if !ok || row.Get("symbol").Any() != "A" || row.Get("total").Any() != float64(20) {
		t.Fatalf("second lifecycle join row retained old state = %#v", row.AsMap())
	}
}
