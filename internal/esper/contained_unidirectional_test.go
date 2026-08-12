package esper

import (
	"context"
	"testing"
)

type containedUnidirectionalBook struct {
	ID    string `esper:"bookId"`
	Title string `esper:"title"`
}

type containedUnidirectionalItem struct {
	ProductID string `esper:"productId"`
	Amount    int64  `esper:"amount"`
}

type containedUnidirectionalOrderDetail struct {
	Items []containedUnidirectionalItem `esper:"items"`
}

type containedUnidirectionalOrder struct {
	OrderID     string                             `esper:"orderId"`
	Books       []containedUnidirectionalBook      `esper:"books"`
	OrderDetail containedUnidirectionalOrderDetail `esper:"orderdetail"`
}

func containedUnidirectionalStreams(env *Environment) (Stream[containedUnidirectionalOrder], Stream[containedUnidirectionalBook], Stream[containedUnidirectionalItem]) {
	orders := From[containedUnidirectionalOrder](env, "ContainedUnidirectionalOrder")
	books := Unnest[containedUnidirectionalOrder, containedUnidirectionalBook](orders, Property[[]containedUnidirectionalBook](EventValue[containedUnidirectionalOrder](), "books"))
	detail := Property[containedUnidirectionalOrderDetail](EventValue[containedUnidirectionalOrder](), "orderdetail")
	items := Unnest[containedUnidirectionalOrder, containedUnidirectionalItem](orders, Property[[]containedUnidirectionalItem](detail, "items"))
	return orders, books, items
}

func registerContainedUnidirectionalTypes(t *testing.T, env *Environment) {
	t.Helper()
	for _, register := range []func() error{
		func() error {
			_, err := RegisterStruct[containedUnidirectionalOrder](env, "ContainedUnidirectionalOrder")
			return err
		},
		func() error {
			_, err := RegisterStruct[containedUnidirectionalBook](env, "ContainedUnidirectionalBook")
			return err
		},
		func() error {
			_, err := RegisterStruct[containedUnidirectionalItem](env, "ContainedUnidirectionalItem")
			return err
		},
	} {
		if err := register(); err != nil {
			t.Fatal(err)
		}
	}
}

func containedUnidirectionalOrderOne() containedUnidirectionalOrder {
	return containedUnidirectionalOrder{
		OrderID: "PO200901",
		Books: []containedUnidirectionalBook{
			{ID: "10020", Title: "Enders Game"},
			{ID: "10021", Title: "Foundation 1"},
		},
		OrderDetail: containedUnidirectionalOrderDetail{Items: []containedUnidirectionalItem{
			{ProductID: "10020", Amount: 10},
			{ProductID: "10020", Amount: 30},
			{ProductID: "10021", Amount: 25},
		}},
	}
}

func containedUnidirectionalOrderTwo() containedUnidirectionalOrder {
	return containedUnidirectionalOrder{
		OrderID:     "PO200902",
		Books:       []containedUnidirectionalBook{{ID: "10022", Title: "Stranger in a Strange Land"}},
		OrderDetail: containedUnidirectionalOrderDetail{Items: []containedUnidirectionalItem{{ProductID: "10022", Amount: 5}}},
	}
}

func containedUnidirectionalOrderThree() containedUnidirectionalOrder {
	return containedUnidirectionalOrder{
		OrderID:     "PO200903",
		Books:       []containedUnidirectionalBook{{ID: "10021", Title: "Foundation 1"}},
		OrderDetail: containedUnidirectionalOrderDetail{Items: []containedUnidirectionalItem{{ProductID: "10021", Amount: 50}}},
	}
}

func TestContainedUnidirectionalJoinMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	registerContainedUnidirectionalTypes(t, env)
	orders, books, items := containedUnidirectionalStreams(env)
	plan, err := env.Build(JoinMany(
		JoinSource(orders).Unidirectional(),
		JoinSource(books),
		JoinSource(items),
	).On(OnSourcesEqual(
		1, Field[containedUnidirectionalBook, string]("bookId"),
		2, Field[containedUnidirectionalItem, string]("productId"),
	)).Select(
		SelectFrom(0, "orderId", JoinField[string](0, "orderId")),
		SelectFrom(1, "bookId", JoinField[string](1, "bookId")),
		SelectFrom(1, "title", JoinField[string](1, "title")),
		SelectFrom(2, "amount", JoinField[int64](2, "amount")),
	).Query(
		StatementName("contained-unidirectional-join"),
		OrderBy(Ascending(JoinField[string](1, "bookId")), Ascending(JoinField[int64](2, "amount"))),
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

	for _, order := range []containedUnidirectionalOrder{
		containedUnidirectionalOrderOne(),
		containedUnidirectionalOrderTwo(),
		containedUnidirectionalOrderThree(),
	} {
		if err := engine.SendEvent(context.Background(), order); err != nil {
			t.Fatal(err)
		}
	}

	if len(batches) != 3 {
		t.Fatalf("contained unidirectional join batches = %#v", batches)
	}
	want := [][]any{
		{"PO200901", "10020", "Enders Game", int64(10)},
		{"PO200901", "10020", "Enders Game", int64(30)},
		{"PO200901", "10021", "Foundation 1", int64(25)},
		{"PO200902", "10022", "Stranger in a Strange Land", int64(5)},
		{"PO200903", "10021", "Foundation 1", int64(50)},
	}
	var got []Row
	for _, batch := range batches {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("contained unidirectional join result is not a row: %#v", result)
			}
			got = append(got, row)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("contained unidirectional join rows = %#v, want %#v", got, want)
	}
	for index, expected := range want {
		for field, value := range map[string]any{
			"orderId": expected[0], "bookId": expected[1], "title": expected[2], "amount": expected[3],
		} {
			if actual := got[index].Get(field).Any(); actual != value {
				t.Fatalf("contained unidirectional join row %d field %s = %#v, want %#v", index, field, actual, value)
			}
		}
	}
}

func TestContainedUnidirectionalJoinCountMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	registerContainedUnidirectionalTypes(t, env)
	orders, books, items := containedUnidirectionalStreams(env)
	plan, err := env.Build(JoinMany(
		JoinSource(orders).Unidirectional(),
		JoinSource(books),
		JoinSource(items),
	).On(OnSourcesEqual(
		1, Field[containedUnidirectionalBook, string]("bookId"),
		2, Field[containedUnidirectionalItem, string]("productId"),
	)).Aggregate(
		Alias("count", CountAll()),
	).Query(StatementName("contained-unidirectional-count")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var counts []int64
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorState, "contained unidirectional count result is not a row")
			}
			value := row.Get("count").Any()
			count, ok := value.(int64)
			if !ok {
				return NewError(ErrorState, "contained unidirectional count has unexpected type")
			}
			counts = append(counts, count)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, order := range []containedUnidirectionalOrder{
		containedUnidirectionalOrderOne(),
		containedUnidirectionalOrderTwo(),
		containedUnidirectionalOrderThree(),
	} {
		if err := engine.SendEvent(context.Background(), order); err != nil {
			t.Fatal(err)
		}
	}
	if len(counts) != 3 || counts[0] != 3 || counts[1] != 1 || counts[2] != 1 {
		t.Fatalf("contained unidirectional counts = %#v, want [3 1 1]", counts)
	}
}
