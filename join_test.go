package esper

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

type joinOrder struct {
	OrderID string `esper:"orderID"`
	Symbol  string `esper:"symbol"`
}

type joinPayment struct {
	OrderID string  `esper:"orderID"`
	Amount  float64 `esper:"amount"`
}

type joinShipment struct {
	OrderID string `esper:"orderID"`
	Carrier string `esper:"carrier"`
}

type joinLimitOrder struct {
	OrderID string  `esper:"orderID"`
	Limit   float64 `esper:"limit"`
}

type joinTableProbe struct {
	Symbol string `esper:"symbol"`
}

func TestLiveJoinRefreshesTableSnapshotForEachTrigger(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinTableProbe](env, "TableProbe"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "Positions", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[float64]("price"),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	table, ok := engine.Table("Positions")
	if !ok {
		t.Fatal("positions table is missing")
	}
	if _, err := table.Upsert(context.Background(), map[string]any{"symbol": "A", "price": 10.0}); err != nil {
		t.Fatal(err)
	}
	if _, err := table.Upsert(context.Background(), map[string]any{"symbol": "B", "price": 20.0}); err != nil {
		t.Fatal(err)
	}
	probe := From[joinTableProbe](env, "TableProbe")
	plan, err := env.Build(JoinMany(
		JoinSource(probe),
		JoinRecordSource(FromTable(env, "Positions")),
	).On(OnSourcesEqual(0, Field[joinTableProbe, string]("symbol"), 1, Field[any, string]("symbol"))).Select(
		SelectFrom(0, "symbol", Field[joinTableProbe, string]("symbol")),
		SelectFrom(1, "price", Field[any, float64]("price")),
	).Query(StatementName("live-table-join")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
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
	if err := engine.SendEvent(context.Background(), joinTableProbe{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("symbol").Any() != "A" || rows[0].Get("price").Any() != float64(10) {
		t.Fatalf("live table join rows = %#v", rows)
	}
	if _, err := table.Update(context.Background(), []any{"A"}, map[string]any{"price": 15.0}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), joinTableProbe{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[1].Get("price").Any() != float64(15) || rows[2].Get("price").Any() != float64(15) {
		t.Fatalf("live table join refreshed rows = %#v", rows)
	}
}

func TestInnerJoinBuilderAndRuntime(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "Order"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "Payment"); err != nil {
		t.Fatal(err)
	}
	orders := From[joinOrder](env, "Order")
	payments := From[joinPayment](env, "Payment")
	joined := Join(orders, payments, OnEqual(
		Field[joinOrder, string]("orderID"),
		Field[joinPayment, string]("orderID"),
	))
	query := joined.Select(
		SelectLeft("symbol", Field[joinOrder, string]("symbol")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Query(StatementName("order-payment"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Payment", joinPayment{OrderID: "O-1", Amount: 9.5}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatalf("payment before order should not join: %#v", batches)
	}
	if err := engine.Send(context.Background(), "Order", joinOrder{OrderID: "O-1", Symbol: "ESPER"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("join batches = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok {
		t.Fatal("join result is not a Row")
	}
	if row.Get("symbol").Any() != "ESPER" || row.Get("amount").Any() != 9.5 {
		t.Fatalf("join row = %#v", row.AsMap())
	}
}

func TestJoinAggregateGroupsTuplesAndEmitsOldNewRows(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "Order"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "Payment"); err != nil {
		t.Fatal(err)
	}
	orders := From[joinOrder](env, "Order").Window(KeepAll())
	payments := From[joinPayment](env, "Payment").Window(LengthWindow(2))
	orderSymbol := JoinField[string](0, "symbol")
	amount := JoinField[float64](1, "amount")
	firstOrder := First[Event](JoinEventValue[Event](0))
	plan, err := env.Build(Join(orders, payments,
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	).GroupBy(orderSymbol).Select(
		Alias("symbol", orderSymbol),
		Alias("total", Sum[float64](amount)),
		Alias("firstSymbol", Property[string](firstOrder, "symbol")),
		Alias("window", WindowValues[float64](amount)),
	).Query(StatementName("join-aggregate"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := make([]ResultBatch, 0, 3)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Order", joinOrder{OrderID: "O-1", Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Payment", joinPayment{OrderID: "O-1", Amount: 10}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("first join aggregate batch = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("symbol").Any() != "A" || row.Get("total").Any() != float64(10) || row.Get("firstSymbol").Any() != "A" {
		t.Fatalf("first join aggregate row = %#v", row.AsMap())
	}
	if got, ok := row.Get("window").Any().([]float64); !ok || !reflect.DeepEqual(got, []float64{10}) {
		t.Fatalf("first join aggregate window = %#v", row.Get("window").Any())
	}
	if err := engine.Send(context.Background(), "Payment", joinPayment{OrderID: "O-1", Amount: 20}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || len(batches[1].Old) != 1 || len(batches[1].New) != 1 {
		t.Fatalf("join aggregate old/new batch = %#v", batches)
	}
	oldRow, oldOK := batches[1].Old[0].Row()
	newRow, newOK := batches[1].New[0].Row()
	if !oldOK || !newOK || oldRow.Get("total").Any() != float64(10) || newRow.Get("total").Any() != float64(30) {
		t.Fatalf("join aggregate totals old=%#v new=%#v", oldRow.AsMap(), newRow.AsMap())
	}
	if err := engine.Send(context.Background(), "Payment", joinPayment{OrderID: "O-1", Amount: 30}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 3 || len(batches[2].Old) != 1 || len(batches[2].New) != 1 {
		t.Fatalf("join aggregate eviction batch = %#v", batches)
	}
	oldRow, oldOK = batches[2].Old[0].Row()
	newRow, newOK = batches[2].New[0].Row()
	if !oldOK || !newOK || oldRow.Get("total").Any() != float64(30) || newRow.Get("total").Any() != float64(50) {
		t.Fatalf("join aggregate eviction totals old=%#v new=%#v", oldRow.AsMap(), newRow.AsMap())
	}
}

func TestOuterJoinAggregateCountsOnlyMatchedSourceValues(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "Order"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "Payment"); err != nil {
		t.Fatal(err)
	}
	orderSymbol := JoinField[string](0, "symbol")
	amount := JoinField[float64](1, "amount")
	plan, err := env.Build(Join(
		From[joinOrder](env, "Order").Window(KeepAll()),
		From[joinPayment](env, "Payment").Window(KeepAll()),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	).LeftOuter().GroupBy(orderSymbol).Select(
		Alias("symbol", orderSymbol),
		Alias("matchedCount", Count[float64](amount)),
		Alias("matchedTotal", Sum[float64](amount)),
	).Query(StatementName("outer-join-aggregate"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := make([]ResultBatch, 0, 2)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Order", joinOrder{OrderID: "O-2", Symbol: "LEFT"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("outer join aggregate unmatched batch = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("matchedCount").Any() != int64(0) || !row.Get("matchedTotal").IsNull() {
		t.Fatalf("outer join aggregate unmatched row = %#v", row.AsMap())
	}
	if err := engine.Send(context.Background(), "Payment", joinPayment{OrderID: "O-2", Amount: 3}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || len(batches[1].Old) != 1 || len(batches[1].New) != 1 {
		t.Fatalf("outer join aggregate transition = %#v", batches)
	}
	oldRow, oldOK := batches[1].Old[0].Row()
	newRow, newOK := batches[1].New[0].Row()
	if !oldOK || !newOK || oldRow.Get("matchedCount").Any() != int64(0) || newRow.Get("matchedCount").Any() != int64(1) || newRow.Get("matchedTotal").Any() != float64(3) {
		t.Fatalf("outer join aggregate old/new = %#v / %#v", oldRow.AsMap(), newRow.AsMap())
	}
}

func TestJoinAggregateWhereFiltersTuplesBeforeGrouping(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "AggregateWhereOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "AggregateWherePayment"); err != nil {
		t.Fatal(err)
	}
	amount := JoinField[float64](1, "amount")
	symbol := JoinField[string](0, "symbol")
	plan, err := env.Build(Join(
		From[joinOrder](env, "AggregateWhereOrder").Window(KeepAll()),
		From[joinPayment](env, "AggregateWherePayment").Window(KeepAll()),
		OnEqual(
			Field[joinOrder, string]("orderID"),
			Field[joinPayment, string]("orderID"),
		),
	).LeftOuter().GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("count", Count[float64](amount)),
		Alias("total", Sum[float64](amount)),
	).Where(
		GreaterOrEqual[float64](amount, Literal(10.0)),
	).Query(StatementName("join-aggregate-where"), WithOldStream()))
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

	sendOrder := func(orderID, value string) {
		t.Helper()
		if err := engine.Send(context.Background(), "AggregateWhereOrder", joinOrder{OrderID: orderID, Symbol: value}); err != nil {
			t.Fatal(err)
		}
	}
	sendPayment := func(orderID string, value float64) {
		t.Helper()
		if err := engine.Send(context.Background(), "AggregateWherePayment", joinPayment{OrderID: orderID, Amount: value}); err != nil {
			t.Fatal(err)
		}
	}

	// The unmatched outer tuple and a below-threshold match are both filtered
	// before they can create a group.
	sendOrder("O1", "A")
	sendPayment("O1", 5)
	if len(batches) != 0 {
		t.Fatalf("filtered aggregate emitted early = %#v", batches)
	}

	// A later qualifying tuple creates the group, and a second qualifying row
	// emits the previous and current aggregate values.
	sendPayment("O1", 10)
	if len(batches) != 1 || len(batches[0].New) != 1 || len(batches[0].Old) != 0 {
		t.Fatalf("first filtered aggregate batch = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("symbol").Any() != "A" || row.Get("count").Any() != int64(1) || row.Get("total").Any() != float64(10) {
		t.Fatalf("first filtered aggregate row = %#v", row.AsMap())
	}
	sendPayment("O1", 20)
	if len(batches) != 2 || len(batches[1].Old) != 1 || len(batches[1].New) != 1 {
		t.Fatalf("filtered aggregate old/new batch = %#v", batches)
	}
	oldRow, oldOK := batches[1].Old[0].Row()
	newRow, newOK := batches[1].New[0].Row()
	if !oldOK || !newOK || oldRow.Get("count").Any() != int64(1) || oldRow.Get("total").Any() != float64(10) ||
		newRow.Get("count").Any() != int64(2) || newRow.Get("total").Any() != float64(30) {
		t.Fatalf("filtered aggregate totals old=%#v new=%#v", oldRow.AsMap(), newRow.AsMap())
	}
}

func TestMultiJoinAggregateReadsIndexedTupleSources(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "Order"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "Payment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinShipment](env, "Shipment"); err != nil {
		t.Fatal(err)
	}
	orders := From[joinOrder](env, "Order").Window(KeepAll())
	payments := From[joinPayment](env, "Payment").Window(KeepAll())
	shipments := From[joinShipment](env, "Shipment").Window(KeepAll())
	orderSymbol := JoinField[string](0, "symbol")
	amount := JoinField[float64](1, "amount")
	carrier := JoinField[string](2, "carrier")
	plan, err := env.Build(JoinMany(
		JoinSource(orders), JoinSource(payments), JoinSource(shipments),
	).On(
		OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 1, Field[joinPayment, string]("orderID")),
		OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 2, Field[joinShipment, string]("orderID")),
	).GroupBy(orderSymbol).Select(
		Alias("symbol", orderSymbol),
		Alias("total", Sum[float64](amount)),
		Alias("firstCarrier", First[string](carrier)),
	).Query(StatementName("multi-join-aggregate")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Order", joinOrder{OrderID: "O-3", Symbol: "MULTI"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Payment", joinPayment{OrderID: "O-3", Amount: 7}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("multi join aggregate emitted before final source: %#v", rows)
	}
	if err := engine.Send(context.Background(), "Shipment", joinShipment{OrderID: "O-3", Carrier: "carrier-1"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("symbol").Any() != "MULTI" || rows[0].Get("total").Any() != float64(7) || rows[0].Get("firstCarrier").Any() != "carrier-1" {
		t.Fatalf("multi join aggregate rows = %#v", rows)
	}
}

func TestJoinAggregateRejectsImplicitOrUnknownSourceFields(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "Order"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "Payment"); err != nil {
		t.Fatal(err)
	}
	joined := Join(
		From[joinOrder](env, "Order"),
		From[joinPayment](env, "Payment"),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	)
	implicit := joined.GroupBy(Field[joinOrder, string]("symbol")).Select(Alias("count", CountAll())).Query()
	if _, err := env.Build(implicit); err == nil {
		t.Fatal("join aggregate accepted an implicit unscoped field")
	}
	unknown := joined.GroupBy(JoinField[string](2, "symbol")).Select(Alias("count", CountAll())).Query()
	if _, err := env.Build(unknown); err == nil {
		t.Fatal("join aggregate accepted an unknown source index")
	}
}

func TestFireAndForgetNamedWindowJoinUsesSnapshotSources(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "left-trades", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "right-trades", schema); err != nil {
		t.Fatal(err)
	}
	left := FromNamedWindow(env, "left-trades")
	right := FromNamedWindow(env, "right-trades")
	joined := JoinMany(JoinRecordSource(left), JoinRecordSource(right)).On(
		OnSourcesEqual(0, Field[any, string]("symbol"), 1, Field[any, string]("symbol")),
	).Select(
		SelectFrom(0, "left-price", Field[any, float64]("price")),
		SelectFrom(1, "right-price", Field[any, float64]("price")),
	).Query(StatementName("faf-named-window-join"))
	plan, err := env.Build(joined)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if err := engine.InsertNamedWindow(context.Background(), "left-trades", runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "left-trades", runtimeTestTrade{Symbol: "B", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "right-trades", runtimeTestTrade{Symbol: "A", Price: 3}); err != nil {
		t.Fatal(err)
	}
	result, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil || len(result.Results()) != 1 {
		t.Fatalf("named-window join result = %#v, err=%v", result.Results(), err)
	}
	row, ok := result.Results()[0].Row()
	if !ok || row.Get("left-price").Any() != float64(1) || row.Get("right-price").Any() != float64(3) {
		t.Fatalf("named-window join row = %#v", result.Results()[0])
	}
}

func TestFireAndForgetJoinAggregateWhereFiltersSnapshotTuples(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "JoinAggregateFAFTrade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("JoinAggregateFAFTrade")
	if !ok {
		t.Fatal("JoinAggregateFAFTrade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "join-aggregate-faf-left", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "join-aggregate-faf-right", schema); err != nil {
		t.Fatal(err)
	}
	price := JoinField[float64](1, "price")
	plan, err := env.Build(JoinMany(
		JoinRecordSource(FromNamedWindow(env, "join-aggregate-faf-left")),
		JoinRecordSource(FromNamedWindow(env, "join-aggregate-faf-right")),
	).On(
		OnSourcesEqual(0, Field[any, string]("symbol"), 1, Field[any, string]("symbol")),
	).GroupBy(JoinField[string](0, "symbol")).Select(
		Alias("symbol", JoinField[string](0, "symbol")),
		Alias("count", Count[float64](price)),
		Alias("sum", Sum[float64](price)),
	).Where(GreaterOrEqual[float64](price, Literal(10.0))).Query(
		StatementName("join-aggregate-faf-where")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, item := range []struct {
		window string
		trade  runtimeTestTrade
	}{
		{window: "join-aggregate-faf-left", trade: runtimeTestTrade{Symbol: "A", Price: 1}},
		{window: "join-aggregate-faf-right", trade: runtimeTestTrade{Symbol: "A", Price: 5}},
		{window: "join-aggregate-faf-right", trade: runtimeTestTrade{Symbol: "A", Price: 15}},
	} {
		if err := engine.InsertNamedWindow(context.Background(), item.window, item.trade); err != nil {
			t.Fatal(err)
		}
	}
	result, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil || len(result.Results()) != 1 {
		t.Fatalf("join aggregate FAF result = %#v, err=%v", result.Results(), err)
	}
	row, ok := result.Results()[0].Row()
	if !ok || row.Get("symbol").Any() != "A" || row.Get("count").Any() != int64(1) || row.Get("sum").Any() != float64(15) {
		t.Fatalf("join aggregate FAF row = %#v", result.Results()[0])
	}
}

func TestContextFireAndForgetJoinAggregateWhereHonorsPartitionSelector(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "ContextJoinAggregateFAFTrade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("ContextJoinAggregateFAFTrade")
	if !ok {
		t.Fatal("ContextJoinAggregateFAFTrade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "context-join-aggregate-faf-left", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "context-join-aggregate-faf-right", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "context-join-aggregate-faf", Field[any, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	price := JoinField[float64](1, "price")
	plan, err := env.Build(JoinMany(
		JoinRecordSource(FromNamedWindow(env, "context-join-aggregate-faf-left")),
		JoinRecordSource(FromNamedWindow(env, "context-join-aggregate-faf-right")),
	).On(
		OnSourcesEqual(0, Field[any, string]("symbol"), 1, Field[any, string]("symbol")),
	).GroupBy(JoinField[string](0, "symbol")).Select(
		Alias("symbol", JoinField[string](0, "symbol")),
		Alias("count", Count[float64](price)),
		Alias("sum", Sum[float64](price)),
	).Where(GreaterOrEqual[float64](price, Literal(10.0))).Query(
		StatementName("context-join-aggregate-faf-where"), WithContext("context-join-aggregate-faf")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, item := range []struct {
		window string
		trade  runtimeTestTrade
	}{
		{window: "context-join-aggregate-faf-left", trade: runtimeTestTrade{Symbol: "A", Price: 1}},
		{window: "context-join-aggregate-faf-left", trade: runtimeTestTrade{Symbol: "B", Price: 1}},
		{window: "context-join-aggregate-faf-right", trade: runtimeTestTrade{Symbol: "A", Price: 15}},
		{window: "context-join-aggregate-faf-right", trade: runtimeTestTrade{Symbol: "B", Price: 5}},
	} {
		if err := engine.InsertNamedWindow(context.Background(), item.window, item.trade); err != nil {
			t.Fatal(err)
		}
	}
	all, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, ContextPartitionSelectorAll{})
	if err != nil || len(all.Results()) != 1 {
		t.Fatalf("context join aggregate FAF all result = %#v, err=%v", all.Results(), err)
	}
	row, ok := all.Results()[0].Row()
	if !ok || row.Get("symbol").Any() != "A" || row.Get("sum").Any() != float64(15) {
		t.Fatalf("context join aggregate FAF all row = %#v", all.Results()[0])
	}
	keyB := encodeKey([]any{ValuePresent, "B"})
	selected, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, SelectContextPartitions(keyB))
	if err != nil || len(selected.Results()) != 0 {
		t.Fatalf("context join aggregate FAF selected B result = %#v, err=%v", selected.Results(), err)
	}
}

func TestContextFireAndForgetJoinHonorsPartitionSelector(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "context-left", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "context-right", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "by-trade-symbol", Field[any, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(JoinMany(
		JoinRecordSource(FromNamedWindow(env, "context-left")),
		JoinRecordSource(FromNamedWindow(env, "context-right")),
	).On(
		OnSourcesEqual(0, Field[any, string]("symbol"), 1, Field[any, string]("symbol")),
	).Select(
		SelectFrom(0, "left-price", Field[any, float64]("price")),
		SelectFrom(1, "right-price", Field[any, float64]("price")),
	).Query(StatementName("context-faf-join"), WithContext("by-trade-symbol")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, item := range []struct {
		window string
		trade  runtimeTestTrade
	}{
		{window: "context-left", trade: runtimeTestTrade{Symbol: "A", Price: 1}},
		{window: "context-left", trade: runtimeTestTrade{Symbol: "B", Price: 2}},
		{window: "context-right", trade: runtimeTestTrade{Symbol: "A", Price: 3}},
		{window: "context-right", trade: runtimeTestTrade{Symbol: "B", Price: 4}},
	} {
		if err := engine.InsertNamedWindow(context.Background(), item.window, item.trade); err != nil {
			t.Fatal(err)
		}
	}

	all, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, ContextPartitionSelectorAll{})
	if err != nil || len(all.Results()) != 2 {
		t.Fatalf("all context FAF join results = %#v, err=%v", all.Results(), err)
	}
	keyA := encodeKey([]any{ValuePresent, "A"})
	selected, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, SelectContextPartitions(keyA))
	if err != nil || len(selected.Results()) != 1 {
		t.Fatalf("selected context FAF join results = %#v, err=%v", selected.Results(), err)
	}
	row, ok := selected.Results()[0].Row()
	if !ok || row.Get("left-price").Any() != float64(1) || row.Get("right-price").Any() != float64(3) {
		t.Fatalf("selected context FAF join row = %#v", selected.Results()[0])
	}
	selectedByID, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, SelectContextPartitionIDs(0))
	if err != nil || len(selectedByID.Results()) != 1 {
		t.Fatalf("selected context FAF join id results = %#v, err=%v", selectedByID.Results(), err)
	}
	row, ok = selectedByID.Results()[0].Row()
	if !ok || row.Get("left-price").Any() != float64(1) || row.Get("right-price").Any() != float64(3) {
		t.Fatalf("selected context FAF join id row = %#v", selectedByID.Results()[0])
	}
}

func TestContextPartitionedJoinKeepsJoinStateIsolated(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "Order"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "Payment"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "by-order", Field[any, string]("orderID")); err != nil {
		t.Fatal(err)
	}
	joined := Join(
		From[joinOrder](env, "Order"),
		From[joinPayment](env, "Payment"),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	).Select(
		SelectLeft("symbol", Field[joinOrder, string]("symbol")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Query(StatementName("context-join"), WithContext("by-order"))
	plan, err := env.Build(joined)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("context join result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Payment", joinPayment{OrderID: "O-1", Amount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Order", joinOrder{OrderID: "O-2", Symbol: "B"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Payment", joinPayment{OrderID: "O-2", Amount: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Order", joinOrder{OrderID: "O-1", Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Get("symbol").Any() != "B" || rows[0].Get("amount").Any() != float64(2) || rows[1].Get("symbol").Any() != "A" || rows[1].Get("amount").Any() != float64(1) {
		t.Fatalf("context-partitioned join rows = %#v", rows)
	}
	if deployment.Statements()[0].ContextPartitionCount() != 2 {
		t.Fatalf("context-partitioned join partitions = %d", deployment.Statements()[0].ContextPartitionCount())
	}
}

func TestJoinBuildRequiresProjection(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "Order"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "Payment"); err != nil {
		t.Fatal(err)
	}
	joined := Join(
		From[joinOrder](env, "Order"),
		From[joinPayment](env, "Payment"),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	)
	if _, err := env.Build(joined.Query()); err == nil {
		t.Fatal("join without projection must fail Build")
	}
}

func TestLeftOuterJoinTransitionsFromUnmatchedToMatched(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "Order"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "Payment"); err != nil {
		t.Fatal(err)
	}
	joined := Join(
		From[joinOrder](env, "Order"),
		From[joinPayment](env, "Payment"),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	).LeftOuter()
	plan, err := env.Build(joined.Select(
		SelectLeft("symbol", Field[joinOrder, string]("symbol")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Query(StatementName("left-outer"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Order", joinOrder{OrderID: "O-2", Symbol: "LEFT"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("unmatched outer row = %#v", batches)
	}
	row, _ := batches[0].New[0].Row()
	if !row.Get("amount").IsNull() {
		t.Fatalf("unmatched right side = %v", row.Get("amount"))
	}
	if err := engine.Send(context.Background(), "Payment", joinPayment{OrderID: "O-2", Amount: 3}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || len(batches[1].Old) != 1 || len(batches[1].New) != 1 {
		t.Fatalf("outer transition = %#v", batches)
	}
	oldRow, _ := batches[1].Old[0].Row()
	newRow, _ := batches[1].New[0].Row()
	if !oldRow.Get("amount").IsNull() || newRow.Get("amount").Any() != float64(3) {
		t.Fatalf("outer old/new rows = %#v / %#v", oldRow.AsMap(), newRow.AsMap())
	}
}

func TestJoinWhereFiltersPostJoinOuterRowsAndOldNewTransitions(t *testing.T) {
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
	).LeftOuter().Select(
		SelectLeft("symbol", Field[joinOrder, string]("symbol")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Where(IsNull[float64](JoinField[float64](1, "amount"))).Query(StatementName("join-where"), WithOldStream())
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
	).LeftOuter().Select(
		SelectLeft("symbol", Field[joinOrder, string]("symbol")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Query(StatementName("join-without-where")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == withoutWhere.Hash() {
		t.Fatal("join where predicate did not enter plan identity")
	}
	if _, err := env.Build(Join(
		From[joinOrder](env, "JoinWhereOrder"),
		From[joinPayment](env, "JoinWherePayment"),
		OnEqual(
			Field[joinOrder, string]("orderID"),
			Field[joinPayment, string]("orderID"),
		),
	).Select(
		SelectLeft("symbol", Field[joinOrder, string]("symbol")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Where(Equal[string](Field[joinOrder, string]("symbol"), Literal("A"))).Query()); err == nil {
		t.Fatal("join where accepted an implicit unscoped field")
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
	if err := engine.Send(context.Background(), "JoinWhereOrder", joinOrder{OrderID: "O1", Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	firstRow, firstOK := Row{}, false
	if len(batches) == 1 && len(batches[0].New) == 1 {
		firstRow, firstOK = batches[0].New[0].Row()
	}
	if !firstOK || !firstRow.Get("amount").IsNull() {
		t.Fatalf("post-join where unmatched row = %#v", batches)
	}
	if err := engine.Send(context.Background(), "JoinWherePayment", joinPayment{OrderID: "O1", Amount: 3}); err != nil {
		t.Fatal(err)
	}
	oldRow, oldOK := Row{}, false
	if len(batches) == 2 && len(batches[1].Old) == 1 {
		oldRow, oldOK = batches[1].Old[0].Row()
	}
	if !oldOK || len(batches[1].New) != 0 || !oldRow.Get("amount").IsNull() {
		t.Fatalf("post-join where outer transition = %#v", batches)
	}
}

func TestCompoundJoinConditionSupportsRangePredicate(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinLimitOrder](env, "LimitOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "Payment"); err != nil {
		t.Fatal(err)
	}
	condition := AllJoin(
		OnEqual(Field[joinLimitOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
		OnGreaterOrEqual(Field[joinLimitOrder, float64]("limit"), Field[joinPayment, float64]("amount")),
	)
	plan, err := env.Build(Join(
		From[joinLimitOrder](env, "LimitOrder"),
		From[joinPayment](env, "Payment"),
		condition,
	).Select(
		SelectLeft("limit", Field[joinLimitOrder, float64]("limit")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Query(StatementName("compound-join")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "LimitOrder", joinLimitOrder{OrderID: "O-3", Limit: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Payment", joinPayment{OrderID: "O-3", Amount: 12}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatalf("out-of-range payment joined: %#v", batches)
	}
	if err := engine.Send(context.Background(), "Payment", joinPayment{OrderID: "O-3", Amount: 8}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("range join batches = %#v", batches)
	}
}

func TestMultiJoinProducesCrossStreamTuple(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "Order"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "Payment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinShipment](env, "Shipment"); err != nil {
		t.Fatal(err)
	}
	orders := From[joinOrder](env, "Order")
	payments := From[joinPayment](env, "Payment")
	shipments := From[joinShipment](env, "Shipment")
	multi := JoinMany(JoinSource(orders), JoinSource(payments), JoinSource(shipments)).On(
		OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 1, Field[joinPayment, string]("orderID")),
		OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 2, Field[joinShipment, string]("orderID")),
	)
	plan, err := env.Build(multi.Select(
		SelectFrom(0, "symbol", Field[joinOrder, string]("symbol")),
		SelectFrom(1, "amount", Field[joinPayment, float64]("amount")),
		SelectFrom(2, "carrier", Field[joinShipment, string]("carrier")),
	).Query(StatementName("three-way-join")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Order", joinOrder{OrderID: "O-4", Symbol: "ESPER"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Payment", joinPayment{OrderID: "O-4", Amount: 5}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatalf("incomplete tuple emitted: %#v", batches)
	}
	if err := engine.Send(context.Background(), "Shipment", joinShipment{OrderID: "O-4", Carrier: "go"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("three-way join batches = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("symbol").Any() != "ESPER" || row.Get("amount").Any() != float64(5) || row.Get("carrier").Any() != "go" {
		t.Fatalf("three-way join row = %#v", row.AsMap())
	}
}

func TestMultiJoinWithoutOnProducesCartesianTuples(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "CartesianOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "CartesianPayment"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(JoinMany(
		JoinSource(From[joinOrder](env, "CartesianOrder")),
		JoinSource(From[joinPayment](env, "CartesianPayment")),
	).Select(
		SelectFrom(0, "orderID", Field[joinOrder, string]("orderID")),
		SelectFrom(1, "amount", Field[joinPayment, float64]("amount")),
	).Query(StatementName("cartesian-join")))
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
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "CartesianOrder", joinOrder{OrderID: "O1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "CartesianOrder", joinOrder{OrderID: "O2"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "CartesianPayment", joinPayment{OrderID: "P1", Amount: 1}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("cartesian join rows = %#v", rows)
	}
	if rows[0].Get("orderID").Any() != "O1" || rows[1].Get("orderID").Any() != "O2" || rows[0].Get("amount").Any() != float64(1) || rows[1].Get("amount").Any() != float64(1) {
		t.Fatalf("cartesian join tuple values = %#v", rows)
	}
}

func TestTwoStreamJoinWithoutConditionProducesCartesianTuples(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "TwoStreamCartesianOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "TwoStreamCartesianPayment"); err != nil {
		t.Fatal(err)
	}
	withoutCondition, err := env.Build(Join(
		From[joinOrder](env, "TwoStreamCartesianOrder"),
		From[joinPayment](env, "TwoStreamCartesianPayment"),
	).Select(
		SelectLeft("orderID", Field[joinOrder, string]("orderID")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Query(StatementName("two-stream-cartesian")))
	if err != nil {
		t.Fatal(err)
	}
	withCondition, err := env.Build(Join(
		From[joinOrder](env, "TwoStreamCartesianOrder"),
		From[joinPayment](env, "TwoStreamCartesianPayment"),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	).Select(
		SelectLeft("orderID", Field[joinOrder, string]("orderID")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Query(StatementName("two-stream-equality")))
	if err != nil {
		t.Fatal(err)
	}
	if withoutCondition.Hash() == withCondition.Hash() {
		t.Fatal("two-stream Cartesian and equality joins must have different plan identities")
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), withoutCondition)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "TwoStreamCartesianOrder", joinOrder{OrderID: "O1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "TwoStreamCartesianOrder", joinOrder{OrderID: "O2"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "TwoStreamCartesianPayment", joinPayment{OrderID: "P1", Amount: 1}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Get("orderID").Any() != "O1" || rows[1].Get("orderID").Any() != "O2" {
		t.Fatalf("two-stream Cartesian rows = %#v", rows)
	}
}

func TestMultiJoinRejectsInvalidSource(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "Order"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "Payment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinShipment](env, "Shipment"); err != nil {
		t.Fatal(err)
	}
	invalid := JoinMany(
		JoinSource(From[joinOrder](env, "Order")),
		JoinSource(From[joinPayment](env, "Payment")),
		JoinSource(From[joinShipment](env, "Shipment")),
	).On(OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 3, Field[joinShipment, string]("orderID")))
	if _, err := env.Build(invalid.Select(
		SelectFrom(0, "symbol", Field[joinOrder, string]("symbol")),
		SelectFrom(1, "amount", Field[joinPayment, float64]("amount")),
		SelectFrom(2, "carrier", Field[joinShipment, string]("carrier")),
	).Query()); err == nil {
		t.Fatal("out-of-range multi-join source was accepted")
	}
}

func TestMultiJoinLeftOuterEmitsUnmatchedAndMatchedTransitions(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "Order"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "Payment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinShipment](env, "Shipment"); err != nil {
		t.Fatal(err)
	}
	multi := JoinMany(
		JoinSource(From[joinOrder](env, "Order")),
		JoinSource(From[joinPayment](env, "Payment")),
		JoinSource(From[joinShipment](env, "Shipment")),
	).On(
		OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 1, Field[joinPayment, string]("orderID")),
		OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 2, Field[joinShipment, string]("orderID")),
	).LeftOuter()
	plan, err := env.Build(multi.Select(
		SelectFrom(0, "symbol", Field[joinOrder, string]("symbol")),
		SelectFrom(1, "amount", Field[joinPayment, float64]("amount")),
		SelectFrom(2, "carrier", Field[joinShipment, string]("carrier")),
	).Query(StatementName("three-way-left-outer"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Order", joinOrder{OrderID: "O-5", Symbol: "LEFT"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("unmatched three-way outer = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("symbol").Any() != "LEFT" || !row.Get("amount").IsNull() || !row.Get("carrier").IsNull() {
		t.Fatalf("unmatched three-way row = %#v", row.AsMap())
	}
	if err := engine.Send(context.Background(), "Payment", joinPayment{OrderID: "O-5", Amount: 7}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("partial three-way match changed output = %#v", batches)
	}
	if err := engine.Send(context.Background(), "Shipment", joinShipment{OrderID: "O-5", Carrier: "go"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || len(batches[1].Old) != 1 || len(batches[1].New) != 1 {
		t.Fatalf("matched three-way transition = %#v", batches)
	}
	newRow, ok := batches[1].New[0].Row()
	if !ok || newRow.Get("amount").Any() != float64(7) || newRow.Get("carrier").Any() != "go" {
		t.Fatalf("matched three-way row = %#v", newRow.AsMap())
	}
}

func TestMultiJoinFullOuterEmitsUnmatchedEverySource(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "FullOuterOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "FullOuterPayment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinShipment](env, "FullOuterShipment"); err != nil {
		t.Fatal(err)
	}
	multi := JoinMany(
		JoinSource(From[joinOrder](env, "FullOuterOrder")),
		JoinSource(From[joinPayment](env, "FullOuterPayment")),
		JoinSource(From[joinShipment](env, "FullOuterShipment")),
	).On(
		OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 1, Field[joinPayment, string]("orderID")),
		OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 2, Field[joinShipment, string]("orderID")),
	).FullOuter()
	plan, err := env.Build(multi.Select(
		SelectFrom(0, "orderID", Field[joinOrder, string]("orderID")),
		SelectFrom(1, "amount", Field[joinPayment, float64]("amount")),
		SelectFrom(2, "carrier", Field[joinShipment, string]("carrier")),
	).Query(StatementName("three-way-full-outer"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
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
	send("FullOuterPayment", joinPayment{OrderID: "F-1", Amount: 7})
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("middle full-outer unmatched row = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || !row.Get("orderID").IsNull() || row.Get("amount").Any() != float64(7) || !row.Get("carrier").IsNull() {
		t.Fatalf("middle full-outer row = %#v", row.AsMap())
	}
	send("FullOuterShipment", joinShipment{OrderID: "F-1", Carrier: "go"})
	if len(batches) != 2 || len(batches[1].Old) != 0 || len(batches[1].New) != 1 {
		t.Fatalf("right full-outer unmatched transition = %#v", batches)
	}
	row, ok = batches[1].New[0].Row()
	if !ok || !row.Get("orderID").IsNull() || !row.Get("amount").IsNull() || row.Get("carrier").Any() != "go" {
		t.Fatalf("right full-outer row = %#v", row.AsMap())
	}
	send("FullOuterOrder", joinOrder{OrderID: "F-1"})
	if len(batches) != 3 || len(batches[2].Old) != 2 || len(batches[2].New) != 1 {
		t.Fatalf("full-outer completion transition = %#v", batches)
	}
	row, ok = batches[2].New[0].Row()
	if !ok || row.Get("orderID").Any() != "F-1" || row.Get("amount").Any() != float64(7) || row.Get("carrier").Any() != "go" {
		t.Fatalf("full-outer completed row = %#v", row.AsMap())
	}
}
