package esper

import (
	"context"
	"testing"
)

func TestJoinChainFullThenInnerPreservesIntermediateOptionalRowsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "MixedOuterInnerOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "MixedOuterInnerPayment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinShipment](env, "MixedOuterInnerShipment"); err != nil {
		t.Fatal(err)
	}

	chain := JoinChain(JoinSource(From[joinOrder](env, "MixedOuterInnerOrder"))).
		FullOuterJoin(JoinSource(From[joinPayment](env, "MixedOuterInnerPayment")), OnSourcesEqual(
			0, Field[joinOrder, string]("orderID"),
			1, Field[joinPayment, string]("orderID"),
		)).
		InnerJoin(JoinSource(From[joinShipment](env, "MixedOuterInnerShipment")), OnSourcesEqual(
			1, Field[joinPayment, string]("orderID"),
			2, Field[joinShipment, string]("orderID"),
		))
	plan, err := env.Build(chain.Select(
		SelectFrom(0, "s0", JoinField[string](0, "orderID")),
		SelectFrom(1, "s1", JoinField[string](1, "orderID")),
		SelectFrom(2, "s2", JoinField[string](2, "orderID")),
	).Query(StatementName("mixed-full-inner-chain")))
	if err != nil {
		t.Fatal(err)
	}

	engine := env.NewEngine()
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

	send := func(source string, event any) {
		t.Helper()
		if err := engine.Send(context.Background(), source, event); err != nil {
			t.Fatal(err)
		}
	}
	assertNew := func(batchIndex int, s0, s1, s2 string, null0 bool) {
		t.Helper()
		if batchIndex >= len(batches) || len(batches[batchIndex].New) != 1 {
			t.Fatalf("mixed full-inner batches = %#v, missing batch %d", batches, batchIndex)
		}
		row, ok := batches[batchIndex].New[0].Row()
		if !ok {
			t.Fatalf("mixed full-inner result is not a row: %#v", batches[batchIndex].New[0])
		}
		if null0 != row.Get("s0").IsNull() || (!null0 && row.Get("s0").Any() != s0) || row.Get("s1").Any() != s1 || row.Get("s2").Any() != s2 {
			t.Fatalf("mixed full-inner row = %#v, want (%q,%q,%q), null0=%t", row.AsMap(), s0, s1, s2, null0)
		}
	}

	send("MixedOuterInnerPayment", joinPayment{OrderID: "A", Amount: 1})
	if len(batches) != 0 {
		t.Fatalf("payment without inner source emitted: %#v", batches)
	}
	send("MixedOuterInnerShipment", joinShipment{OrderID: "A", Carrier: "carrier-A"})
	assertNew(0, "", "A", "A", true)
	send("MixedOuterInnerOrder", joinOrder{OrderID: "A", Symbol: "order-A"})
	assertNew(1, "A", "A", "A", false)

	send("MixedOuterInnerPayment", joinPayment{OrderID: "B", Amount: 2})
	send("MixedOuterInnerShipment", joinShipment{OrderID: "B", Carrier: "carrier-B"})
	assertNew(2, "", "B", "B", true)
	send("MixedOuterInnerOrder", joinOrder{OrderID: "B", Symbol: "order-B"})
	assertNew(3, "B", "B", "B", false)

	// An s0/s2 pair cannot pass the second inner edge until s1 arrives.
	send("MixedOuterInnerOrder", joinOrder{OrderID: "C", Symbol: "order-C"})
	send("MixedOuterInnerShipment", joinShipment{OrderID: "C", Carrier: "carrier-C"})
	if len(batches) != 4 {
		t.Fatalf("incomplete C chain emitted: %#v", batches)
	}
	send("MixedOuterInnerPayment", joinPayment{OrderID: "C", Amount: 3})
	assertNew(4, "C", "C", "C", false)
}
