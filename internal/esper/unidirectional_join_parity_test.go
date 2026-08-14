package esper

import (
	"context"
	"testing"
)

type unidirectionalJoinParityMarket struct {
	Symbol string `esper:"symbol"`
	Volume int64  `esper:"volume"`
}

type unidirectionalJoinParityBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestUnidirectionalAggregateJoinParity mirrors the shared
// unidirectional-aggregate-join scenario (Java EPLJoin2TableJoinGrouped):
// only a SupportMarketDataBean driver emits, and each trigger emits one
// grouped irstream pair over all retained SupportBean rows.
func TestUnidirectionalAggregateJoinParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[unidirectionalJoinParityMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[unidirectionalJoinParityBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	query := Join(
		From[unidirectionalJoinParityMarket](env, "SupportMarketDataBean"),
		From[unidirectionalJoinParityBean](env, "SupportBean").Window(KeepAll()),
		OnEqual(
			Field[unidirectionalJoinParityMarket, string]("symbol"),
			Field[unidirectionalJoinParityBean, string]("theString"),
		),
	).Unidirectional(JoinLeft).GroupBy(
		JoinField[string](0, "symbol"),
		JoinField[string](1, "theString"),
	).Select(
		Alias("symbol", JoinField[string](0, "symbol")),
		Alias("cnt", CountAll()),
	).Query(StatementName("s0"), WithOldStream())
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendMarket := func(symbol string, volume int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), unidirectionalJoinParityMarket{Symbol: symbol, Volume: volume}); err != nil {
			t.Fatal(err)
		}
	}
	sendBean := func(theString string, value int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), unidirectionalJoinParityBean{TheString: theString, IntPrimitive: value}); err != nil {
			t.Fatal(err)
		}
	}
	sendMarket("E1", 1)
	sendBean("E1", 10)
	if len(batches) != 0 {
		t.Fatalf("passive or unmatched events emitted = %#v", batches)
	}
	sendMarket("E1", 2)
	assertUnidirectionalAggregateJoinPair(t, batches, 0, "E1", 1)
	sendBean("E1", 20)
	if len(batches) != 1 {
		t.Fatalf("passive bean emitted = %#v", batches)
	}
	sendMarket("E1", 3)
	assertUnidirectionalAggregateJoinPair(t, batches, 1, "E1", 2)
	sendBean("E2", 40)
	if len(batches) != 2 {
		t.Fatalf("second passive bean emitted = %#v", batches)
	}
	sendMarket("E2", 4)
	assertUnidirectionalAggregateJoinPair(t, batches, 2, "E2", 1)
}

func assertUnidirectionalAggregateJoinPair(t *testing.T, batches []ResultBatch, index int, symbol string, count int64) {
	t.Helper()
	if len(batches) <= index || len(batches[index].New) != 1 || len(batches[index].Old) != 1 {
		t.Fatalf("unidirectional aggregate join batch %d = %#v", index, batches)
	}
	newRow, ok := batches[index].New[0].Row()
	if !ok || newRow.Get("symbol").Any() != symbol || newRow.Get("cnt").Any() != count {
		t.Fatalf("unidirectional aggregate join new row %d = %#v", index, batches[index].New)
	}
	oldRow, ok := batches[index].Old[0].Row()
	// The unidirectional driver is transient, so every trigger resets the
	// group's aggregate state: the old row always reports the zero count.
	if !ok || oldRow.Get("symbol").Any() != symbol || oldRow.Get("cnt").Any() != int64(0) {
		t.Fatalf("unidirectional aggregate join old row %d = %#v (values %v)",
			index, batches[index].Old, oldRow.AsMap())
	}
}
