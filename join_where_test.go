package esper

import (
	"context"
	"testing"
)

func TestJoinWhereFiltersMatchedTuplesAfterOn(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "JoinWhereOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "JoinWherePayment"); err != nil {
		t.Fatal(err)
	}
	query := Join(
		From[joinOrder](env, "JoinWhereOrder"),
		From[joinPayment](env, "JoinWherePayment"),
		OnEqual(
			Field[joinOrder, string]("orderID"),
			Field[joinPayment, string]("orderID"),
		),
	).Select(
		SelectLeft("orderID", Field[joinOrder, string]("orderID")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Where(
		GreaterOrEqual[float64](JoinField[float64](1, "amount"), Literal(10.0)),
	).Query(StatementName("join-where"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	withoutWhere, err := env.Build(Join(
		From[joinOrder](env, "JoinWhereOrder"),
		From[joinPayment](env, "JoinWherePayment"),
		OnEqual(
			Field[joinOrder, string]("orderID"),
			Field[joinPayment, string]("orderID"),
		),
	).Select(
		SelectLeft("orderID", Field[joinOrder, string]("orderID")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Query(StatementName("join-without-where")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == withoutWhere.Hash() {
		t.Fatal("join Where predicate must affect plan identity")
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(source string, value any) {
		t.Helper()
		if err := engine.Send(context.Background(), source, value); err != nil {
			t.Fatal(err)
		}
	}
	send("JoinWhereOrder", joinOrder{OrderID: "O1"})
	send("JoinWhereOrder", joinOrder{OrderID: "O2"})
	send("JoinWherePayment", joinPayment{OrderID: "O1", Amount: 5})
	send("JoinWherePayment", joinPayment{OrderID: "O2", Amount: 10})
	if len(rows) != 1 || rows[0].Get("orderID").Any() != "O2" || rows[0].Get("amount").Any() != float64(10) {
		t.Fatalf("join Where rows = %#v", rows)
	}
}

func TestFireAndForgetJoinWhereFiltersNamedWindowSnapshot(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "JoinWhereTrade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("JoinWhereTrade")
	if !ok {
		t.Fatal("JoinWhereTrade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "join-where-left", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "join-where-right", schema); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(JoinMany(
		JoinRecordSource(FromNamedWindow(env, "join-where-left")),
		JoinRecordSource(FromNamedWindow(env, "join-where-right")),
	).On(
		OnSourcesEqual(0, Field[any, string]("symbol"), 1, Field[any, string]("symbol")),
	).Select(
		SelectFrom(0, "symbol", Field[any, string]("symbol")),
		SelectFrom(1, "price", Field[any, float64]("price")),
	).Where(
		GreaterOrEqual[float64](JoinField[float64](1, "price"), Literal(3.0)),
	).Query(StatementName("faf-join-where")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, event := range []runtimeTestTrade{
		{Symbol: "A", Price: 1},
		{Symbol: "B", Price: 2},
	} {
		if err := engine.InsertNamedWindow(context.Background(), "join-where-left", event); err != nil {
			t.Fatal(err)
		}
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "A", Price: 3},
		{Symbol: "B", Price: 2},
	} {
		if err := engine.InsertNamedWindow(context.Background(), "join-where-right", event); err != nil {
			t.Fatal(err)
		}
	}
	result, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil || len(result.Results()) != 1 {
		t.Fatalf("fire-and-forget Join Where result = %#v, err=%v", result.Results(), err)
	}
	row, ok := result.Results()[0].Row()
	if !ok || row.Get("symbol").Any() != "A" || row.Get("price").Any() != float64(3) {
		t.Fatalf("fire-and-forget Join Where row = %#v", result.Results()[0])
	}
}

func TestJoinWhereCorrelatedSubqueryUsesCompleteTupleScope(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "JoinWhereSubqueryOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "JoinWhereSubqueryPayment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinSubqueryReference](env, "JoinWhereSubqueryReference"); err != nil {
		t.Fatal(err)
	}
	inner := Select(From[joinSubqueryReference](env, "JoinWhereSubqueryReference")).Window(KeepAll())
	where := SubqueryExists(
		inner,
		Equal[string](
			Field[joinSubqueryReference, string]("orderID"),
			JoinField[string](1, "orderID"),
		),
	)
	plan, err := env.Build(Join(
		From[joinOrder](env, "JoinWhereSubqueryOrder"),
		From[joinPayment](env, "JoinWhereSubqueryPayment"),
		OnEqual(
			Field[joinOrder, string]("orderID"),
			Field[joinPayment, string]("orderID"),
		),
	).Select(
		SelectLeft("orderID", Field[joinOrder, string]("orderID")),
	).Where(where).Query(StatementName("join-where-subquery")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinWhereSubqueryReference", joinSubqueryReference{OrderID: "O1", Value: "v1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinWhereSubqueryPayment", joinPayment{OrderID: "O2", Amount: 20}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinWhereSubqueryOrder", joinOrder{OrderID: "O2"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("join Where subquery matched wrong key = %#v", rows)
	}
	if err := engine.Send(context.Background(), "JoinWhereSubqueryPayment", joinPayment{OrderID: "O1", Amount: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinWhereSubqueryOrder", joinOrder{OrderID: "O1"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("orderID").Any() != "O1" {
		t.Fatalf("join Where subquery rows = %#v", rows)
	}
}

func TestJoinWhereRejectsUnscopedFieldExpression(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "JoinWhereInvalidOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "JoinWhereInvalidPayment"); err != nil {
		t.Fatal(err)
	}
	query := Join(
		From[joinOrder](env, "JoinWhereInvalidOrder"),
		From[joinPayment](env, "JoinWhereInvalidPayment"),
		OnEqual(
			Field[joinOrder, string]("orderID"),
			Field[joinPayment, string]("orderID"),
		),
	).Select(
		SelectLeft("orderID", Field[joinOrder, string]("orderID")),
	).Where(
		Equal[string](Field[joinOrder, string]("orderID"), Literal("O1")),
	).Query(StatementName("join-where-invalid"))
	if _, err := env.Build(query); err == nil {
		t.Fatal("unscoped Join Where field unexpectedly built")
	}
}

func TestContextJoinWhereKeepsOuterNullFilteringPartitionLocal(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "ContextJoinWhereOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "ContextJoinWherePayment"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "context-join-where", Field[any, string]("orderID")); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Join(
		From[joinOrder](env, "ContextJoinWhereOrder"),
		From[joinPayment](env, "ContextJoinWherePayment"),
		OnEqual(
			Field[joinOrder, string]("orderID"),
			Field[joinPayment, string]("orderID"),
		),
	).LeftOuter().Select(
		SelectLeft("orderID", Field[joinOrder, string]("orderID")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Where(IsNull[float64](JoinField[float64](1, "amount"))).Query(
		StatementName("context-join-where"),
		WithContext("context-join-where"),
		WithOldStream(),
	))
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
	send := func(source string, value any) {
		t.Helper()
		if err := engine.Send(context.Background(), source, value); err != nil {
			t.Fatal(err)
		}
	}
	send("ContextJoinWhereOrder", joinOrder{OrderID: "O1"})
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("context Join Where first unmatched row = %#v", batches)
	}
	first, ok := batches[0].New[0].Row()
	if !ok || first.Get("orderID").Any() != "O1" || !first.Get("amount").IsNull() {
		t.Fatalf("context Join Where first row = %#v", batches[0].New[0])
	}
	send("ContextJoinWhereOrder", joinOrder{OrderID: "O2"})
	if len(batches) != 2 || len(batches[1].New) != 1 {
		t.Fatalf("context Join Where second partition row = %#v", batches)
	}
	second, ok := batches[1].New[0].Row()
	if !ok || second.Get("orderID").Any() != "O2" || !second.Get("amount").IsNull() {
		t.Fatalf("context Join Where second row = %#v", batches[1].New[0])
	}
	send("ContextJoinWherePayment", joinPayment{OrderID: "O1", Amount: 3})
	if len(batches) != 3 || len(batches[2].Old) != 1 || len(batches[2].New) != 0 {
		t.Fatalf("context Join Where filtered transition = %#v", batches)
	}
	old, ok := batches[2].Old[0].Row()
	if !ok || old.Get("orderID").Any() != "O1" || !old.Get("amount").IsNull() {
		t.Fatalf("context Join Where old row = %#v", batches[2].Old[0])
	}
	if deployment.Statements()[0].ContextPartitionCount() != 2 {
		t.Fatalf("context Join Where partitions = %d", deployment.Statements()[0].ContextPartitionCount())
	}
}

func TestContextFireAndForgetJoinWhereHonorsPartitionSelector(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "ContextFAFJoinWhereTrade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("ContextFAFJoinWhereTrade")
	if !ok {
		t.Fatal("ContextFAFJoinWhereTrade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "context-faf-join-where-left", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "context-faf-join-where-right", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "context-faf-join-where", Field[any, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(JoinMany(
		JoinRecordSource(FromNamedWindow(env, "context-faf-join-where-left")),
		JoinRecordSource(FromNamedWindow(env, "context-faf-join-where-right")),
	).On(
		OnSourcesEqual(0, Field[any, string]("symbol"), 1, Field[any, string]("symbol")),
	).Select(
		SelectFrom(0, "symbol", Field[any, string]("symbol")),
		SelectFrom(1, "price", Field[any, float64]("price")),
	).Where(
		GreaterOrEqual[float64](JoinField[float64](1, "price"), Literal(3.0)),
	).Query(StatementName("context-faf-join-where"), WithContext("context-faf-join-where")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, item := range []struct {
		window string
		trade  runtimeTestTrade
	}{
		{window: "context-faf-join-where-left", trade: runtimeTestTrade{Symbol: "A", Price: 1}},
		{window: "context-faf-join-where-left", trade: runtimeTestTrade{Symbol: "B", Price: 2}},
		{window: "context-faf-join-where-right", trade: runtimeTestTrade{Symbol: "A", Price: 3}},
		{window: "context-faf-join-where-right", trade: runtimeTestTrade{Symbol: "B", Price: 2}},
	} {
		if err := engine.InsertNamedWindow(context.Background(), item.window, item.trade); err != nil {
			t.Fatal(err)
		}
	}
	all, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, ContextPartitionSelectorAll{})
	if err != nil || len(all.Results()) != 1 {
		t.Fatalf("context FAF Join Where all result = %#v, err=%v", all.Results(), err)
	}
	row, ok := all.Results()[0].Row()
	if !ok || row.Get("symbol").Any() != "A" || row.Get("price").Any() != float64(3) {
		t.Fatalf("context FAF Join Where all row = %#v", all.Results()[0])
	}
	keyB := encodeKey([]any{ValuePresent, "B"})
	selected, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, SelectContextPartitions(keyB))
	if err != nil || len(selected.Results()) != 0 {
		t.Fatalf("context FAF Join Where selected B result = %#v, err=%v", selected.Results(), err)
	}
}
