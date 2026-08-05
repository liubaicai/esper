package esper

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

func TestRankWindowPreviousAccessMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[rankPreviousTrade](env, "RankPreviousTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	symbol := Field[rankPreviousTrade, string]("symbol")
	price := Field[rankPreviousTrade, int]("price")
	current := EventValue[rankPreviousTrade]()
	stream := From[rankPreviousTrade](env, "RankPreviousTrade").Window(RankWindowBy(3, []Expr{symbol}, Ascending(price)))
	projected := Select(stream,
		Alias("win", PrevWindow[rankPreviousTrade](current)),
		Alias("prev0", Prev[rankPreviousTrade](0, current)),
		Alias("prev1", Prev[rankPreviousTrade](1, current)),
		Alias("prev2", Prev[rankPreviousTrade](2, current)),
		Alias("prev3", Prev[rankPreviousTrade](3, current)),
		Alias("prev4", Prev[rankPreviousTrade](4, current)),
	)
	plan, err := env.Build(projected.Query(StatementName("rank-previous")))
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

	events := []rankPreviousTrade{
		{Symbol: "E1", Price: 100, Sequence: 0},
		{Symbol: "E2", Price: 99, Sequence: 0},
		{Symbol: "E1", Price: 98, Sequence: 1},
		{Symbol: "E3", Price: 98, Sequence: 0},
		{Symbol: "E2", Price: 97, Sequence: 1},
	}
	want := [][]rankPreviousTrade{
		{{Symbol: "E1", Price: 100, Sequence: 0}},
		{{Symbol: "E2", Price: 99, Sequence: 0}, {Symbol: "E1", Price: 100, Sequence: 0}},
		{{Symbol: "E1", Price: 98, Sequence: 1}, {Symbol: "E2", Price: 99, Sequence: 0}},
		{{Symbol: "E1", Price: 98, Sequence: 1}, {Symbol: "E3", Price: 98, Sequence: 0}, {Symbol: "E2", Price: 99, Sequence: 0}},
		{{Symbol: "E2", Price: 97, Sequence: 1}, {Symbol: "E1", Price: 98, Sequence: 1}, {Symbol: "E3", Price: 98, Sequence: 0}},
	}
	for index, event := range events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		if len(batches) != index+1 || len(batches[index].New) != 1 {
			t.Fatalf("rank previous batch %d = %#v", index, batches)
		}
		assertRankPreviousRow(t, batches[index].New[0], want[index], index)
	}
}

func TestGroupedRankWindowMatchesEsperReplacementLifecycle(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[groupedRankTrade](env, "GroupedRankTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	group := Field[groupedRankTrade, string]("symbol")
	uniqueKey := Field[groupedRankTrade, int]("intKey")
	sortKey := Field[groupedRankTrade, float64]("score")
	stream := From[groupedRankTrade](env, "GroupedRankTrade").Window(
		GroupWindow(group, RankWindowBy(2, []Expr{uniqueKey}, Ascending(sortKey))),
	)
	statement, batches := deployViewTest(t, env, engine, stream, "grouped-rank")
	events := []groupedRankTrade{
		{Symbol: "E1", IntKey: 100, Score: 1},
		{Symbol: "E2", IntKey: 100, Score: 2},
		{Symbol: "E1", IntKey: 200, Score: 0.5},
		{Symbol: "E2", IntKey: 200, Score: 2.5},
		{Symbol: "E1", IntKey: 300, Score: 0.1},
	}
	for _, event := range events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*batches) != len(events) || len((*batches)[4].New) != 1 || len((*batches)[4].Old) != 1 {
		t.Fatalf("grouped rank batches = %#v", *batches)
	}
	old, ok := (*batches)[4].Old[0].Event()
	if !ok || !reflect.DeepEqual(old.Underlying(), groupedRankTrade{Symbol: "E1", IntKey: 100, Score: 1}) {
		t.Fatalf("grouped rank eviction = %#v", (*batches)[4].Old)
	}
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []groupedRankTrade{
		{Symbol: "E1", IntKey: 300, Score: 0.1},
		{Symbol: "E1", IntKey: 200, Score: 0.5},
		{Symbol: "E2", IntKey: 100, Score: 2},
		{Symbol: "E2", IntKey: 200, Score: 2.5},
	}
	if len(snapshot.Results()) != len(want) {
		t.Fatalf("grouped rank snapshot = %#v, want %#v", snapshot.Results(), want)
	}
	for index, result := range snapshot.Results() {
		event, ok := result.Event()
		if !ok || !reflect.DeepEqual(event.Underlying(), want[index]) {
			t.Fatalf("grouped rank snapshot[%d] = %#v, want %#v", index, event.Underlying(), want[index])
		}
	}
}

type rankPreviousTrade struct {
	Symbol   string `esper:"symbol"`
	Price    int    `esper:"price"`
	Sequence int64  `esper:"sequence"`
}

type groupedRankTrade struct {
	Symbol string  `esper:"symbol"`
	IntKey int     `esper:"intKey"`
	Score  float64 `esper:"score"`
}

func assertRankPreviousRow(t *testing.T, result Result, want []rankPreviousTrade, index int) {
	t.Helper()
	row, ok := result.Row()
	if !ok {
		t.Fatalf("rank previous row %d is not a row", index)
	}
	window, ok := row.Get("win").Any().([]rankPreviousTrade)
	if !ok || !reflect.DeepEqual(window, want) {
		t.Fatalf("rank previous window %d = %#v, want %#v", index, window, want)
	}
	for offset := 0; offset < 5; offset++ {
		value := row.Get(fmt.Sprintf("prev%d", offset))
		if offset >= len(want) {
			if !value.IsNull() {
				t.Fatalf("rank previous prev%d[%d] = %v, want null", offset, index, value)
			}
			continue
		}
		actual, ok := value.Any().(rankPreviousTrade)
		if !ok || actual != want[offset] {
			t.Fatalf("rank previous prev%d[%d] = %#v, want %#v", offset, index, actual, want[offset])
		}
	}
}
