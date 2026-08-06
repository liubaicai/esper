package esper

import (
	"context"
	"strings"
	"testing"
)

func TestUnidirectionalJoinRetainsOnlyPassiveSourceAndRejectsSnapshot(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "UniOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "UniPayment"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Join(
		From[joinOrder](env, "UniOrder"),
		From[joinPayment](env, "UniPayment"),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	).Unidirectional(JoinLeft).Select(
		SelectLeft("order", Field[joinOrder, string]("symbol")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Query(StatementName("unidirectional-two-stream")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.Send(context.Background(), "UniPayment", joinPayment{OrderID: "O1", Amount: 7}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatalf("passive event emitted = %#v", batches)
	}
	if err := engine.Send(context.Background(), "UniOrder", joinOrder{OrderID: "O1", Symbol: "first"}); err != nil {
		t.Fatal(err)
	}
	assertUnidirectionalBatchRow(t, batches, 0, "first", 7)

	// The trigger source is not retained: a second trigger still produces one
	// row rather than joining with the first trigger as an ordinary keep-all
	// stream would.
	if err := engine.Send(context.Background(), "UniOrder", joinOrder{OrderID: "O1", Symbol: "second"}); err != nil {
		t.Fatal(err)
	}
	assertUnidirectionalBatchRow(t, batches, 1, "second", 7)

	if err := engine.Send(context.Background(), "UniPayment", joinPayment{OrderID: "O1", Amount: 9}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 {
		t.Fatalf("second passive event emitted = %#v", batches)
	}
	if err := engine.Send(context.Background(), "UniOrder", joinOrder{OrderID: "O1", Symbol: "third"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 3 || len(batches[2].New) != 2 {
		t.Fatalf("current passive rows = %#v", batches)
	}

	if _, err := statement.Snapshot(context.Background()); err == nil || !strings.Contains(err.Error(), "iteration over a unidirectional join is not supported") {
		t.Fatalf("unidirectional snapshot error = %v", err)
	}
}

func TestUnidirectionalJoinAggregateRecomputesCurrentTriggerProbe(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "UniAggregateOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "UniAggregatePayment"); err != nil {
		t.Fatal(err)
	}
	amount := JoinField[float64](1, "amount")
	plan, err := env.Build(Join(
		From[joinOrder](env, "UniAggregateOrder"),
		From[joinPayment](env, "UniAggregatePayment"),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	).Unidirectional(JoinLeft).GroupBy(JoinField[string](1, "orderID")).Select(
		Alias("orderID", JoinField[string](1, "orderID")),
		Alias("count", Count[float64](amount)),
		Alias("total", Sum[float64](amount)),
	).Query(StatementName("unidirectional-aggregate"), WithOldStream()))
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

	if err := engine.Send(context.Background(), "UniAggregatePayment", joinPayment{OrderID: "O1", Amount: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "UniAggregateOrder", joinOrder{OrderID: "O1"}); err != nil {
		t.Fatal(err)
	}
	assertUnidirectionalAggregateBatch(t, batches, 0, 1, 10)

	if err := engine.Send(context.Background(), "UniAggregatePayment", joinPayment{OrderID: "O1", Amount: 20}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("passive aggregate event emitted = %#v", batches)
	}
	if err := engine.Send(context.Background(), "UniAggregateOrder", joinOrder{OrderID: "O1"}); err != nil {
		t.Fatal(err)
	}
	// The second trigger sees both passive rows, but does not accumulate the
	// first trigger's contribution a second time.
	assertUnidirectionalAggregateBatch(t, batches, 1, 2, 30)
}

func TestUnidirectionalMultiJoinAndFullOuterProbe(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "UniMultiOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "UniMultiPayment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinShipment](env, "UniMultiShipment"); err != nil {
		t.Fatal(err)
	}
	order := JoinSource(From[joinOrder](env, "UniMultiOrder")).Unidirectional()
	payment := JoinSource(From[joinPayment](env, "UniMultiPayment"))
	shipment := JoinSource(From[joinShipment](env, "UniMultiShipment"))
	plan, err := env.Build(JoinMany(order, payment, shipment).On(
		OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 1, Field[joinPayment, string]("orderID")),
		OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 2, Field[joinShipment, string]("orderID")),
	).Select(
		SelectFrom(0, "order", Field[joinOrder, string]("orderID")),
		SelectFrom(1, "amount", Field[joinPayment, float64]("amount")),
		SelectFrom(2, "carrier", Field[joinShipment, string]("carrier")),
	).Query(StatementName("unidirectional-multi")))
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
	if err := engine.Send(context.Background(), "UniMultiPayment", joinPayment{OrderID: "O1", Amount: 4}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "UniMultiShipment", joinShipment{OrderID: "O1", Carrier: "carrier"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "UniMultiOrder", joinOrder{OrderID: "O1"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("multi unidirectional batches = %#v", batches)
	}

	fullPlan, err := env.Build(Join(
		From[joinOrder](env, "UniMultiOrder"),
		From[joinPayment](env, "UniMultiPayment"),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	).FullOuter().Unidirectional(JoinLeft).Select(
		SelectLeft("order", Field[joinOrder, string]("orderID")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Query(StatementName("unidirectional-full-outer")))
	if err != nil {
		t.Fatal(err)
	}
	if fullPlan.Hash() == plan.Hash() {
		t.Fatal("unidirectional/full-outer plan identity was not distinct")
	}
	fullDeployment, err := engine.Deploy(context.Background(), fullPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer fullDeployment.Undeploy(context.Background())
	var fullBatches []ResultBatch
	if _, err := fullDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		fullBatches = append(fullBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "UniMultiPayment", joinPayment{OrderID: "O2", Amount: 8}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "UniMultiOrder", joinOrder{OrderID: "O2"}); err != nil {
		t.Fatal(err)
	}
	if len(fullBatches) != 1 || len(fullBatches[0].New) != 1 {
		t.Fatalf("full outer matched trigger = %#v", fullBatches)
	}
	matched, ok := fullBatches[0].New[0].Row()
	if !ok || matched.Get("order").Any() != "O2" || matched.Get("amount").Any() != float64(8) {
		t.Fatalf("full outer matched row = %#v", fullBatches[0].New)
	}
	if err := engine.Send(context.Background(), "UniMultiOrder", joinOrder{OrderID: "O3"}); err != nil {
		t.Fatal(err)
	}
	if len(fullBatches) != 2 || len(fullBatches[1].New) != 1 {
		t.Fatalf("full outer unmatched trigger = %#v", fullBatches)
	}
	unmatched, ok := fullBatches[1].New[0].Row()
	if !ok || unmatched.Get("order").Any() != "O3" || !unmatched.Get("amount").IsNull() {
		t.Fatalf("full outer unmatched row = %#v", fullBatches[1].New)
	}

	rightPlan, err := env.Build(Join(
		From[joinOrder](env, "UniMultiOrder"),
		From[joinPayment](env, "UniMultiPayment"),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	).FullOuter().Unidirectional(JoinRight).Select(
		SelectLeft("order", Field[joinOrder, string]("orderID")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Query(StatementName("unidirectional-right-driver")))
	if err != nil {
		t.Fatal(err)
	}
	rightDeployment, err := engine.Deploy(context.Background(), rightPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer rightDeployment.Undeploy(context.Background())
	var rightBatches []ResultBatch
	if _, err := rightDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		rightBatches = append(rightBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "UniMultiOrder", joinOrder{OrderID: "O4"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "UniMultiPayment", joinPayment{OrderID: "O4", Amount: 11}); err != nil {
		t.Fatal(err)
	}
	if len(rightBatches) != 1 || len(rightBatches[0].New) != 1 {
		t.Fatalf("right-driver full outer = %#v", rightBatches)
	}
}

func TestUnidirectionalJoinContextPartitionsPassiveState(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "UniContextOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "UniContextPayment"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "uni-context", Field[joinOrder, string]("orderID")); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Join(
		From[joinOrder](env, "UniContextOrder"),
		From[joinPayment](env, "UniContextPayment"),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	).Unidirectional(JoinLeft).Select(
		SelectLeft("order", Field[joinOrder, string]("orderID")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Query(StatementName("unidirectional-context"), WithContext("uni-context")))
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
	if err := engine.Send(context.Background(), "UniContextPayment", joinPayment{OrderID: "A", Amount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "UniContextPayment", joinPayment{OrderID: "B", Amount: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "UniContextOrder", joinOrder{OrderID: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "UniContextOrder", joinOrder{OrderID: "B"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || len(batches[0].New) != 1 || len(batches[1].New) != 1 {
		t.Fatalf("context unidirectional batches = %#v", batches)
	}
	first, firstOK := batches[0].New[0].Row()
	second, secondOK := batches[1].New[0].Row()
	if !firstOK || !secondOK || first.Get("amount").Any() != float64(1) || second.Get("amount").Any() != float64(2) {
		t.Fatalf("context passive isolation rows = %#v / %#v", first.AsMap(), second.AsMap())
	}
}

func TestUnidirectionalJoinChainOuterProbe(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "UniChainOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "UniChainPayment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinShipment](env, "UniChainShipment"); err != nil {
		t.Fatal(err)
	}
	chain := JoinChain(JoinSource(From[joinOrder](env, "UniChainOrder")).Unidirectional()).
		FullOuterJoin(JoinSource(From[joinPayment](env, "UniChainPayment")), OnSourcesEqual(
			0, Field[joinOrder, string]("orderID"), 1, Field[joinPayment, string]("orderID"),
		)).
		FullOuterJoin(JoinSource(From[joinShipment](env, "UniChainShipment")), OnSourcesEqual(
			0, Field[joinOrder, string]("orderID"), 2, Field[joinShipment, string]("orderID"),
		))
	plan, err := env.Build(chain.Select(
		SelectFrom(0, "order", Field[joinOrder, string]("orderID")),
		SelectFrom(1, "amount", Field[joinPayment, float64]("amount")),
		SelectFrom(2, "carrier", Field[joinShipment, string]("carrier")),
	).Query(StatementName("unidirectional-chain")))
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
	if err := engine.Send(context.Background(), "UniChainPayment", joinPayment{OrderID: "O1", Amount: 5}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "UniChainShipment", joinShipment{OrderID: "O1", Carrier: "carrier"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatalf("chain passive events emitted = %#v", batches)
	}
	if err := engine.Send(context.Background(), "UniChainOrder", joinOrder{OrderID: "O1"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("chain trigger batches = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("order").Any() != "O1" || row.Get("amount").Any() != float64(5) || row.Get("carrier").Any() != "carrier" {
		t.Fatalf("chain trigger row = %#v", batches[0].New)
	}
}

func TestUnidirectionalJoinValidation(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "UniValidationOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "UniValidationPayment"); err != nil {
		t.Fatal(err)
	}
	windowed := JoinSource(From[joinOrder](env, "UniValidationOrder").Window(KeepAll())).Unidirectional()
	passive := JoinSource(From[joinPayment](env, "UniValidationPayment"))
	_, err := env.Build(JoinMany(windowed, passive).On(
		OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 1, Field[joinPayment, string]("orderID")),
	).Select(
		SelectFrom(0, "order", Field[joinOrder, string]("orderID")),
	).Query(StatementName("invalid-unidirectional-view")))
	if err == nil || !strings.Contains(err.Error(), "cannot declare a window view") {
		t.Fatalf("windowed unidirectional error = %v", err)
	}

	allDrivers := JoinMany(
		JoinSource(From[joinOrder](env, "UniValidationOrder")).Unidirectional(),
		JoinSource(From[joinPayment](env, "UniValidationPayment")).Unidirectional(),
	).FullOuter().On(OnSourcesEqual(
		0, Field[joinOrder, string]("orderID"), 1, Field[joinPayment, string]("orderID"),
	)).Select(SelectFrom(0, "order", Field[joinOrder, string]("orderID"))).Query(StatementName("invalid-unidirectional-multiple"))
	allPlan, err := env.Build(allDrivers)
	if err != nil {
		t.Fatalf("full outer all-driver plan rejected = %v", err)
	}
	engine := NewEngine(env)
	allDeployment, err := engine.Deploy(context.Background(), allPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer allDeployment.Undeploy(context.Background())
	var allBatches []ResultBatch
	if _, err := allDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		allBatches = append(allBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "UniValidationOrder", joinOrder{OrderID: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "UniValidationPayment", joinPayment{OrderID: "B", Amount: 2}); err != nil {
		t.Fatal(err)
	}
	if len(allBatches) != 2 || len(allBatches[0].New) != 1 || len(allBatches[1].New) != 1 {
		t.Fatalf("all-driver full outer batches = %#v", allBatches)
	}

	invalidMultiple := JoinMany(
		JoinSource(From[joinOrder](env, "UniValidationOrder")).Unidirectional(),
		JoinSource(From[joinPayment](env, "UniValidationPayment")).Unidirectional(),
	).On(OnSourcesEqual(
		0, Field[joinOrder, string]("orderID"), 1, Field[joinPayment, string]("orderID"),
	)).Select(SelectFrom(0, "order", Field[joinOrder, string]("orderID"))).Query(StatementName("invalid-unidirectional-multiple-inner"))
	if _, err := env.Build(invalidMultiple); err == nil || !strings.Contains(err.Error(), "full outer") {
		t.Fatalf("multiple inner-driver error = %v", err)
	}
}

func assertUnidirectionalBatchRow(t *testing.T, batches []ResultBatch, index int, order string, amount float64) {
	t.Helper()
	if len(batches) <= index || len(batches[index].New) != 1 {
		t.Fatalf("unidirectional batch %d = %#v", index, batches)
	}
	row, ok := batches[index].New[0].Row()
	if !ok || row.Get("order").Any() != order || row.Get("amount").Any() != amount {
		t.Fatalf("unidirectional row %d = %#v", index, batches[index].New)
	}
}

func assertUnidirectionalAggregateBatch(t *testing.T, batches []ResultBatch, index int, count int64, total float64) {
	t.Helper()
	if len(batches) <= index || len(batches[index].New) != 1 {
		t.Fatalf("unidirectional aggregate batch %d = %#v", index, batches)
	}
	row, ok := batches[index].New[0].Row()
	if !ok || row.Get("count").Any() != count || row.Get("total").Any() != total {
		t.Fatalf("unidirectional aggregate row %d = %#v", index, batches[index].New)
	}
}
