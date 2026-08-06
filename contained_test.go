package esper

import (
	"context"
	"testing"
)

type unnestBook struct {
	ID      string         `esper:"id"`
	Price   float64        `esper:"price"`
	Reviews []unnestReview `esper:"reviews"`
}

type unnestReview struct {
	ID     string `esper:"id"`
	Rating int64  `esper:"rating"`
}

type unnestOrder struct {
	OrderID string       `esper:"orderId"`
	Books   []unnestBook `esper:"books"`
}

type unnestIntContainer struct {
	IDs []int64 `esper:"ids"`
}

func buildUnnestBookStream(env *Environment) Stream[unnestBook] {
	books := Property[[]unnestBook](EventValue[unnestOrder](), "books")
	return Unnest[unnestOrder, unnestBook](From[unnestOrder](env, "UnnestOrder"), books)
}

func TestUnnestProjectsContainedStructsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[unnestOrder](env, "UnnestOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[unnestBook](env, "UnnestBook"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	children := buildUnnestBookStream(env)
	plan, err := env.Build(Select(children,
		Alias("id", Field[unnestBook, string]("id")),
		Alias("price", Field[unnestBook, float64]("price")),
	).Query(StatementName("unnest-project")))
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
			if !ok {
				t.Fatalf("unnest projection is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), unnestOrder{
		OrderID: "O-1",
		Books:   []unnestBook{{ID: "B-1", Price: 10}, {ID: "B-2", Price: 27}, {ID: "B-3", Price: 35}},
	}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("unnest rows = %#v", rows)
	}
	want := []struct {
		id    string
		price float64
	}{{"B-1", 10}, {"B-2", 27}, {"B-3", 35}}
	for index, expected := range want {
		if got := rows[index]; got.Get("id").Any() != expected.id || got.Get("price").Any() != expected.price {
			t.Fatalf("unnest row %d = %#v, want %#v", index, got, expected)
		}
	}
}

func TestUnnestLengthWindowEmitsContainedOldAndNewRows(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[unnestOrder](env, "UnnestOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[unnestBook](env, "UnnestBook"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	children := buildUnnestBookStream(env).Window(LengthWindow(2))
	plan, err := env.Build(Select(children, Alias("id", Field[unnestBook, string]("id"))).Query(
		StatementName("unnest-window"), WithOldStream(),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var received ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		received = batch
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), unnestOrder{
		OrderID: "O-2",
		Books:   []unnestBook{{ID: "B-1"}, {ID: "B-2"}, {ID: "B-3"}},
	}); err != nil {
		t.Fatal(err)
	}
	if len(received.New) != 3 || len(received.Old) != 1 {
		t.Fatalf("unnest window delta = %#v", received)
	}
	old, ok := received.Old[0].Row()
	if !ok || old.Get("id").Any() != "B-1" {
		t.Fatalf("unnest window old row = %#v", received.Old[0])
	}
}

func TestUnnestAggregateCountMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[unnestOrder](env, "UnnestOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[unnestBook](env, "UnnestBook"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	plan, err := env.Build(buildUnnestBookStream(env).Aggregate(
		Alias("count", CountAll()),
	).Query(StatementName("unnest-count")))
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
	if err := engine.SendEvent(context.Background(), unnestOrder{
		OrderID: "O-3",
		Books:   []unnestBook{{ID: "B-1"}, {ID: "B-2"}, {ID: "B-3"}},
	}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("unnest aggregate batches = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("count").Any() != int64(3) {
		t.Fatalf("unnest aggregate row = %#v", batches[0].New[0])
	}
}

func TestUnnestNestedStructsPreserveParentArrayOrder(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[unnestOrder](env, "UnnestOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[unnestBook](env, "UnnestBook"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[unnestReview](env, "UnnestReview"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	books := buildUnnestBookStream(env)
	reviews := Property[[]unnestReview](EventValue[unnestBook](), "reviews")
	reviewStream := Unnest[unnestBook, unnestReview](books, reviews)
	plan, err := env.Build(Select(reviewStream,
		Alias("id", Field[unnestReview, string]("id")),
		Alias("rating", Field[unnestReview, int64]("rating")),
	).Query(StatementName("unnest-nested")))
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
			if !ok {
				t.Fatalf("nested unnest result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), unnestOrder{
		OrderID: "O-nested",
		Books: []unnestBook{
			{ID: "B-1", Reviews: []unnestReview{{ID: "R-1", Rating: 5}, {ID: "R-2", Rating: 4}}},
			{ID: "B-2", Reviews: []unnestReview{{ID: "R-3", Rating: 3}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("nested unnest rows = %#v", rows)
	}
	want := []struct {
		id     string
		rating int64
	}{{"R-1", 5}, {"R-2", 4}, {"R-3", 3}}
	for index, expected := range want {
		if got := rows[index]; got.Get("id").Any() != expected.id || got.Get("rating").Any() != expected.rating {
			t.Fatalf("nested unnest row %d = %#v, want %#v", index, got, expected)
		}
	}
}

func TestUnnestValuesProjectsScalarArrayElements(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[unnestIntContainer](env, "UnnestIntContainer"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[ContainedValue[int64]](env, "UnnestInt"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	ids := Property[[]int64](EventValue[unnestIntContainer](), "ids")
	values := UnnestValues[unnestIntContainer, int64](From[unnestIntContainer](env, "UnnestIntContainer"), ids)
	plan, err := env.Build(Select(values,
		Alias("value", Field[ContainedValue[int64], int64]("value")),
	).Query(StatementName("unnest-values")))
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
			if !ok {
				t.Fatalf("scalar unnest result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), unnestIntContainer{IDs: []int64{2, 4, 8}}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("scalar unnest rows = %#v", rows)
	}
	for index, expected := range []int64{2, 4, 8} {
		if got := rows[index].Get("value").Any(); got != expected {
			t.Fatalf("scalar unnest row %d = %#v, want %d", index, got, expected)
		}
	}
}
