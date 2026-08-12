package esper

import (
	"context"
	"testing"
	"time"
)

func TestPreviousViewNavigationUsesWindowAndArrivalOrders(t *testing.T) {
	t.Run("length", func(t *testing.T) {
		env, engine := newRuntimeTest(t)
		price := Field[runtimeTestTrade, float64]("price")
		symbol := Field[runtimeTestTrade, string]("symbol")
		stream := From[runtimeTestTrade](env, "Trade").Window(LengthWindow(3))
		projected := Select(stream,
			Alias("symbol", symbol),
			Alias("prev", Prev[float64](1, price)),
			Alias("prior", Prior[float64](0, price)),
			Alias("tail", PrevTail[float64](0, price)),
			Alias("count", PrevCount[float64](price)),
			Alias("window", PrevWindow[float64](price)),
		)
		plan, err := env.Build(projected.Query(StatementName("previous-length")))
		if err != nil {
			t.Fatal(err)
		}
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
		for _, trade := range []runtimeTestTrade{{Symbol: "A", Price: 10}, {Symbol: "B", Price: 20}, {Symbol: "C", Price: 30}} {
			if err := engine.SendEvent(context.Background(), trade); err != nil {
				t.Fatal(err)
			}
		}
		if len(batches) != 3 {
			t.Fatalf("length navigation batches = %d, want 3", len(batches))
		}
		want := []map[string]Value{
			{"symbol": Present("A"), "prev": Null(), "prior": Null(), "tail": Present(10.0), "count": Present(int64(1)), "window": Present([]float64{10})},
			{"symbol": Present("B"), "prev": Present(10.0), "prior": Present(10.0), "tail": Present(10.0), "count": Present(int64(2)), "window": Present([]float64{20, 10})},
			{"symbol": Present("C"), "prev": Present(20.0), "prior": Present(20.0), "tail": Present(10.0), "count": Present(int64(3)), "window": Present([]float64{30, 20, 10})},
		}
		for index, batch := range batches {
			if len(batch.New) != 1 {
				t.Fatalf("length batch %d = %#v", index, batch)
			}
			row, ok := batch.New[0].Row()
			if !ok {
				t.Fatalf("length batch %d is not a row", index)
			}
			for name, expected := range want[index] {
				if got := row.Get(name); !got.Equal(expected) {
					t.Fatalf("length batch %d %s = %v, want %v", index, name, got, expected)
				}
			}
		}
	})

	t.Run("time-order", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[externalTrade](env, "ExternalTrade"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithStartTime(time.Unix(20, 0).UTC()))
		timestamp := Field[externalTrade, int64]("timestamp")
		symbol := Field[externalTrade, string]("symbol")
		stream := From[externalTrade](env, "ExternalTrade").Window(TimeOrder(timestamp, 10*time.Second))
		projected := Select(stream,
			Alias("symbol", symbol),
			Alias("prev0", Prev[string](0, symbol)),
			Alias("prev1", Prev[string](1, symbol)),
			Alias("prior", Prior[string](0, symbol)),
			Alias("tail0", PrevTail[string](0, symbol)),
			Alias("tail1", PrevTail[string](1, symbol)),
			Alias("count", PrevCount[string](symbol)),
			Alias("window", PrevWindow[string](symbol)),
		)
		plan, err := env.Build(projected.Query(StatementName("previous-time-order"), WithOldStream()))
		if err != nil {
			t.Fatal(err)
		}
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
		send := func(trade externalTrade) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), trade); err != nil {
				t.Fatal(err)
			}
		}
		send(externalTrade{Symbol: "E1", Timestamp: 25000})
		send(externalTrade{Symbol: "E2", Timestamp: 21000})
		send(externalTrade{Symbol: "E3", Timestamp: 22000})
		if len(batches) != 3 {
			t.Fatalf("time-order navigation batches = %d, want 3", len(batches))
		}
		want := []map[string]Value{
			{"symbol": Present("E1"), "prev0": Present("E1"), "prev1": Null(), "prior": Null(), "tail0": Present("E1"), "tail1": Null(), "count": Present(int64(1)), "window": Present([]string{"E1"})},
			{"symbol": Present("E2"), "prev0": Present("E2"), "prev1": Present("E1"), "prior": Present("E1"), "tail0": Present("E1"), "tail1": Present("E2"), "count": Present(int64(2)), "window": Present([]string{"E2", "E1"})},
			{"symbol": Present("E3"), "prev0": Present("E2"), "prev1": Present("E3"), "prior": Present("E2"), "tail0": Present("E1"), "tail1": Present("E3"), "count": Present(int64(3)), "window": Present([]string{"E2", "E3", "E1"})},
		}
		for index, batch := range batches {
			if len(batch.New) != 1 {
				t.Fatalf("time-order new batch %d = %#v", index, batch)
			}
			assertNavigationRow(t, batch.New[0], want[index], "time-order new", index)
		}

		if err := engine.AdvanceTime(context.Background(), time.Unix(31, 0).UTC()); err != nil {
			t.Fatal(err)
		}
		if len(batches) != 4 || len(batches[3].Old) != 1 {
			t.Fatalf("time-order expiry batch = %#v", batches)
		}
		oldWant := map[string]Value{
			"symbol": Present("E2"),
			"prev0":  Null(),
			"prev1":  Null(),
			"prior":  Present("E1"),
			"tail0":  Null(),
			"tail1":  Null(),
			"count":  Null(),
			"window": Null(),
		}
		assertNavigationRow(t, batches[3].Old[0], oldWant, "time-order old", 0)

		snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(snapshot.Results()) != 2 {
			t.Fatalf("time-order snapshot rows = %d, want 2", len(snapshot.Results()))
		}
		assertNavigationRow(t, snapshot.Results()[0], map[string]Value{
			"symbol": Present("E3"), "prev0": Present("E3"), "prev1": Present("E1"), "prior": Present("E1"), "tail0": Present("E1"), "tail1": Present("E3"), "count": Present(int64(2)), "window": Present([]string{"E3", "E1"}),
		}, "time-order snapshot", 0)
	})
}

func TestTimeOrderPreviousNavigationMatchesExpiryLifecycle(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[externalTrade](env, "ExternalTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	timestamp := Field[externalTrade, int64]("timestamp")
	symbol := Field[externalTrade, string]("symbol")
	stream := From[externalTrade](env, "ExternalTrade").Window(TimeOrder(timestamp, 10*time.Second))
	projected := Select(stream,
		Alias("symbol", symbol),
		Alias("prev", Prev[string](1, symbol)),
		Alias("prior", Prior[string](0, symbol)),
		Alias("tail", PrevTail[string](0, symbol)),
		Alias("count", PrevCount[string](symbol)),
		Alias("window", PrevWindow[string](symbol)),
	)
	plan, err := env.Build(projected.Query(StatementName("previous-time-order-lifecycle"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
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
	send := func(trade externalTrade) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	advance := func(seconds int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.Unix(seconds, 0).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	send(externalTrade{Symbol: "E1", Timestamp: 1000})
	assertNavigationRow(t, batches[0].New[0], map[string]Value{
		"symbol": Present("E1"), "prev": Null(), "prior": Null(), "tail": Present("E1"), "count": Present(int64(1)), "window": Present([]string{"E1"}),
	}, "lifecycle new", 0)

	advance(10)
	send(externalTrade{Symbol: "E2", Timestamp: 10000})
	assertNavigationRow(t, batches[1].New[0], map[string]Value{
		"symbol": Present("E2"), "prev": Present("E2"), "prior": Present("E1"), "tail": Present("E2"), "count": Present(int64(2)), "window": Present([]string{"E1", "E2"}),
	}, "lifecycle new", 1)

	if err := engine.AdvanceTime(context.Background(), time.Unix(10, 500000000).UTC()); err != nil {
		t.Fatal(err)
	}
	send(externalTrade{Symbol: "E3", Timestamp: 8000})
	assertNavigationRow(t, batches[2].New[0], map[string]Value{
		"symbol": Present("E3"), "prev": Present("E3"), "prior": Present("E2"), "tail": Present("E2"), "count": Present(int64(3)), "window": Present([]string{"E1", "E3", "E2"}),
	}, "lifecycle new", 2)

	advance(11)
	if len(batches) != 4 || len(batches[3].Old) != 1 {
		t.Fatalf("lifecycle E1 expiry = %#v", batches)
	}
	assertNavigationRow(t, batches[3].Old[0], map[string]Value{
		"symbol": Present("E1"), "prev": Null(), "prior": Null(), "tail": Null(), "count": Null(), "window": Null(),
	}, "lifecycle old", 0)

	advance(12)
	send(externalTrade{Symbol: "E4", Timestamp: 7000})
	assertNavigationRow(t, batches[4].New[0], map[string]Value{
		"symbol": Present("E4"), "prev": Present("E3"), "prior": Present("E3"), "tail": Present("E2"), "count": Present(int64(3)), "window": Present([]string{"E4", "E3", "E2"}),
	}, "lifecycle new", 3)

	advance(17)
	if len(batches) != 6 || len(batches[5].Old) != 1 {
		t.Fatalf("lifecycle E4 expiry = %#v", batches)
	}
	assertNavigationRow(t, batches[5].Old[0], map[string]Value{
		"symbol": Present("E4"), "prev": Null(), "prior": Present("E3"), "tail": Null(), "count": Null(), "window": Null(),
	}, "lifecycle old", 1)

	advance(18)
	if len(batches) != 7 || len(batches[6].Old) != 1 {
		t.Fatalf("lifecycle E3 expiry = %#v", batches)
	}
	assertNavigationRow(t, batches[6].Old[0], map[string]Value{
		"symbol": Present("E3"), "prev": Null(), "prior": Present("E2"), "tail": Null(), "count": Null(), "window": Null(),
	}, "lifecycle old", 2)
}

func assertNavigationRow(t *testing.T, result Result, want map[string]Value, label string, index int) {
	t.Helper()
	row, ok := result.Row()
	if !ok {
		t.Fatalf("%s[%d] is not a row", label, index)
	}
	for name, expected := range want {
		if got := row.Get(name); !got.Equal(expected) {
			t.Fatalf("%s[%d] %s = %v, want %v", label, index, name, got, expected)
		}
	}
}
