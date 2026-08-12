package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestTimeOrderPreviousAccessMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[externalTrade](env, "ExternalTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	symbol := Field[externalTrade, string]("symbol")
	stream := From[externalTrade](env, "ExternalTrade").Window(TimeOrder(Field[externalTrade, int64]("timestamp"), 10*time.Second))
	projected := Select(stream,
		Alias("id", symbol),
		Alias("prev0", Prev[string](0, symbol)),
		Alias("prev1", Prev[string](1, symbol)),
		Alias("prior0", Prior[string](0, symbol)),
		Alias("tail0", PrevTail[string](0, symbol)),
		Alias("tail1", PrevTail[string](1, symbol)),
		Alias("count", PrevCount[string](symbol)),
		Alias("window", PrevWindow[string](symbol)),
	)
	plan, err := env.Build(projected.Query(StatementName("time-order-previous"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	batches := make([]ResultBatch, 0, 4)
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, event := range []externalTrade{
		{Symbol: "E1", Timestamp: 25000},
		{Symbol: "E2", Timestamp: 21000},
		{Symbol: "E3", Timestamp: 22000},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}

	wantNew := []map[string]any{
		{"id": "E1", "prev0": "E1", "prev1": nil, "prior0": nil, "tail0": "E1", "tail1": nil, "count": int64(1), "window": []string{"E1"}},
		{"id": "E2", "prev0": "E2", "prev1": "E1", "prior0": "E1", "tail0": "E1", "tail1": "E2", "count": int64(2), "window": []string{"E2", "E1"}},
		{"id": "E3", "prev0": "E2", "prev1": "E3", "prior0": "E2", "tail0": "E1", "tail1": "E3", "count": int64(3), "window": []string{"E2", "E3", "E1"}},
	}
	if len(batches) != len(wantNew) {
		t.Fatalf("time-order batches = %d, want %d", len(batches), len(wantNew))
	}
	for index, want := range wantNew {
		assertPreviousProjection(t, batches[index].New, want)
	}

	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(31000).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 4 {
		t.Fatalf("time-order expiry batches = %d, want 4", len(batches))
	}
	assertPreviousProjection(t, batches[3].Old, map[string]any{
		"id": "E2", "prev0": nil, "prev1": nil, "prior0": "E1", "tail0": nil, "tail1": nil, "count": nil, "window": nil,
	})
}

func TestSortedPreviousAccessUsesPostEvictionViewForNewRows(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[externalTrade](env, "ExternalTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	symbol := Field[externalTrade, string]("symbol")
	stream := From[externalTrade](env, "ExternalTrade").Window(SortWindow(3, Ascending(symbol)))
	projected := Select(stream,
		Alias("id", symbol),
		Alias("prev1", Prev[string](1, symbol)),
		Alias("prior0", Prior[string](0, symbol)),
		Alias("tail0", PrevTail[string](0, symbol)),
		Alias("count", PrevCount[string](symbol)),
		Alias("window", PrevWindow[string](symbol)),
	)
	plan, err := env.Build(projected.Query(StatementName("sorted-previous"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	batches := make([]ResultBatch, 0, 5)
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, symbolValue := range []string{"B1", "D1", "C1", "A1", "F1"} {
		if err := engine.SendEvent(context.Background(), externalTrade{Symbol: symbolValue}); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 5 {
		t.Fatalf("sorted previous batches = %d, want 5", len(batches))
	}
	want := []map[string]any{
		{"id": "B1", "prev1": nil, "prior0": nil, "tail0": "B1", "count": int64(1), "window": []string{"B1"}},
		{"id": "D1", "prev1": "D1", "prior0": "B1", "tail0": "D1", "count": int64(2), "window": []string{"B1", "D1"}},
		{"id": "C1", "prev1": "C1", "prior0": "D1", "tail0": "D1", "count": int64(3), "window": []string{"B1", "C1", "D1"}},
		{"id": "A1", "prev1": "B1", "prior0": "C1", "tail0": "C1", "count": int64(3), "window": []string{"A1", "B1", "C1"}},
		{"id": "F1", "prev1": "B1", "prior0": "A1", "tail0": "C1", "count": int64(3), "window": []string{"A1", "B1", "C1"}},
	}
	for index, expected := range want {
		assertPreviousProjection(t, batches[index].New, expected)
	}
}

func TestGroupedSortedPreviousAccessUsesPartitionView(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[groupedPreviousTrade](env, "GroupedPreviousTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	group := Field[groupedPreviousTrade, string]("group")
	id := Field[groupedPreviousTrade, string]("id")
	price := Field[groupedPreviousTrade, float64]("price")
	stream := From[groupedPreviousTrade](env, "GroupedPreviousTrade").Window(GroupWindow(group, SortWindow(2, Ascending(price))))
	projected := Select(stream,
		Alias("id", id),
		Alias("prev1", Prev[string](1, id)),
		Alias("prior0", Prior[string](0, id)),
		Alias("tail0", PrevTail[string](0, id)),
		Alias("count", PrevCount[string](id)),
		Alias("window", PrevWindow[string](id)),
	)
	plan, err := env.Build(projected.Query(StatementName("grouped-sorted-previous"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	batches := make([]ResultBatch, 0, 3)
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []groupedPreviousTrade{
		{ID: "A1", Group: "A", Price: 2},
		{ID: "A0", Group: "A", Price: 1},
		{ID: "A2", Group: "A", Price: 3},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 3 {
		t.Fatalf("grouped sorted previous batches = %d, want 3", len(batches))
	}
	assertPreviousProjection(t, batches[2].New, map[string]any{
		"id": "A2", "prev1": "A1", "prior0": "A0", "tail0": "A1", "count": int64(2), "window": []string{"A0", "A1"},
	})
}

type groupedPreviousTrade struct {
	ID    string  `esper:"id"`
	Group string  `esper:"group"`
	Price float64 `esper:"price"`
}

func assertPreviousProjection(t *testing.T, results []Result, expected map[string]any) {
	t.Helper()
	if len(results) != 1 {
		t.Fatalf("previous projection results = %#v, want one row", results)
	}
	row, ok := results[0].Row()
	if !ok {
		t.Fatalf("previous projection result = %#v, want row", results[0])
	}
	for name, want := range expected {
		value := row.Get(name)
		if want == nil {
			if !value.IsNull() {
				t.Errorf("previous projection %s = %v, want null", name, value)
			}
			continue
		}
		if !value.IsPresent() || !reflect.DeepEqual(value.Any(), want) {
			t.Errorf("previous projection %s = %#v, want %#v", name, value.Any(), want)
		}
	}
}
