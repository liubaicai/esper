package esper

import (
	"context"
	"reflect"
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

type unnestPayment struct {
	BookID string `esper:"bookId"`
	Amount int64  `esper:"amount"`
}

type containedNestedSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int64  `esper:"intPrimitive"`
}

type containedNestedSubqueryEvent struct {
	TheString string `esper:"theString"`
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

func TestUnnestNestedNamedWindowFilterMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	orderSchema, err := RegisterStruct[unnestOrder](env, "ContainedNestedNamedOrder")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[unnestBook](env, "ContainedNestedNamedBook"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[unnestReview](env, "ContainedNestedNamedReview"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "ContainedNestedNamedWindow", orderSchema, NamedWindowRetention(LastEvent())); err != nil {
		t.Fatal(err)
	}

	orders := From[unnestOrder](env, "ContainedNestedNamedOrder")
	insertPlan, err := env.Build(OnEvent(orders).InsertIntoNamedWindow(
		"ContainedNestedNamedWindow",
		SetColumn("orderId", Field[unnestOrder, string]("orderId")),
		SetColumn("books", Field[unnestOrder, []unnestBook]("books")),
	).Query(StatementName("contained-nested-named-insert")))
	if err != nil {
		t.Fatal(err)
	}

	windowOrders := FromNamedWindowAs[unnestOrder](env, "ContainedNestedNamedWindow")
	books := Unnest[unnestOrder, unnestBook](windowOrders, Property[[]unnestBook](EventValue[unnestOrder](), "books"))
	reviews := Unnest[unnestBook, unnestReview](books, Property[[]unnestReview](EventValue[unnestBook](), "reviews"))
	consumerPlan, err := env.Build(Select(reviews,
		Alias("id", Field[unnestReview, string]("id")),
	).Query(
		StatementName("contained-nested-named-consumer"),
		OrderBy(Ascending(Field[unnestReview, string]("id"))),
	))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = consumerDeployment.Undeploy(context.Background()) }()
	insertDeployment, err := engine.Deploy(context.Background(), insertPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = insertDeployment.Undeploy(context.Background()) }()
	var rows []string
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("nested named-window result is not a row: %#v", result)
			}
			rows = append(rows, row.Get("id").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), unnestOrder{
		OrderID: "O-nwf-1",
		Books: []unnestBook{
			{ID: "B-1", Reviews: []unnestReview{{ID: "R01"}, {ID: "R02"}}},
			{ID: "B-2", Reviews: []unnestReview{{ID: "R10"}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), unnestOrder{
		OrderID: "O-nwf-2",
		Books:   []unnestBook{{ID: "B-3", Reviews: []unnestReview{{ID: "R201"}}}},
	}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"R01", "R02", "R10", "R201"}; !reflect.DeepEqual(rows, want) {
		t.Fatalf("nested named-window review rows = %#v, want %#v", rows, want)
	}
}

func TestUnnestNestedNamedWindowSubqueryMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	orderSchema, err := RegisterStruct[unnestOrder](env, "ContainedNestedSubqueryOrder")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[unnestBook](env, "ContainedNestedSubqueryBook"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[containedNestedSubqueryEvent](env, "ContainedNestedSubqueryEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "ContainedNestedSubqueryWindow", orderSchema, NamedWindowRetention(LastEvent())); err != nil {
		t.Fatal(err)
	}

	orders := From[unnestOrder](env, "ContainedNestedSubqueryOrder")
	insertPlan, err := env.Build(OnEvent(orders).InsertIntoNamedWindow(
		"ContainedNestedSubqueryWindow",
		SetColumn("orderId", Field[unnestOrder, string]("orderId")),
		SetColumn("books", Field[unnestOrder, []unnestBook]("books")),
	).Query(StatementName("contained-nested-subquery-insert")))
	if err != nil {
		t.Fatal(err)
	}

	windowOrders := FromNamedWindowAs[unnestOrder](env, "ContainedNestedSubqueryWindow")
	books := Unnest[unnestOrder, unnestBook](windowOrders, Property[[]unnestBook](EventValue[unnestOrder](), "books"))
	totalPrice := SubquerySum[float64](books.AsRecord(), Field[any, float64]("price"))
	outer := From[containedNestedSubqueryEvent](env, "ContainedNestedSubqueryEvent")
	outerPlan, err := env.Build(Select(outer,
		Alias("theString", Field[containedNestedSubqueryEvent, string]("theString")),
		Alias("totalPrice", totalPrice),
	).Query(StatementName("contained-nested-subquery-consumer")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	outerDeployment, err := engine.Deploy(context.Background(), outerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = outerDeployment.Undeploy(context.Background()) }()
	insertDeployment, err := engine.Deploy(context.Background(), insertPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = insertDeployment.Undeploy(context.Background()) }()
	var rows []Row
	if _, err := outerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("nested named-window subquery result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), unnestOrder{
		OrderID: "O-nws-1",
		Books: []unnestBook{
			{ID: "B-1", Price: 24},
			{ID: "B-2", Price: 35},
			{ID: "B-3", Price: 27},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), containedNestedSubqueryEvent{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), unnestOrder{
		OrderID: "O-nws-2",
		Books: []unnestBook{
			{ID: "B-4", Price: 15},
			{ID: "B-5", Price: 13},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), containedNestedSubqueryEvent{TheString: "E2"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Get("theString").Any() != "E1" || rows[0].Get("totalPrice").Any() != float64(86) ||
		rows[1].Get("theString").Any() != "E2" || rows[1].Get("totalPrice").Any() != float64(28) {
		t.Fatalf("nested named-window subquery rows = %#v", rows)
	}
}

func TestUnnestNestedNamedWindowOnTriggerMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	orderSchema, err := RegisterStruct[unnestOrder](env, "ContainedNestedTriggerOrder")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[unnestBook](env, "ContainedNestedTriggerBook"); err != nil {
		t.Fatal(err)
	}
	supportSchema, err := RegisterStruct[containedNestedSupportBean](env, "ContainedNestedTriggerSupport")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "ContainedNestedTriggerOrders", orderSchema, NamedWindowRetention(LastEvent())); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "ContainedNestedTriggerSupportWindow", supportSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	windowOrders := FromNamedWindowAs[unnestOrder](env, "ContainedNestedTriggerOrders")
	books := Unnest[unnestOrder, unnestBook](windowOrders, Property[[]unnestBook](EventValue[unnestOrder](), "books"))
	trigger, err := env.Build(OnEvent(books).SelectFromNamedWindow(
		"ContainedNestedTriggerSupportWindow",
		Equal[string](NamedWindowField[string]("theString"), Field[unnestBook, string]("id")),
		Alias("theString", NamedWindowField[string]("theString")),
		Alias("intPrimitive", NamedWindowField[int64]("intPrimitive")),
	).Query(StatementName("contained-nested-named-on-trigger")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), trigger)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("nested named-window on-trigger result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "ContainedNestedTriggerSupportWindow", containedNestedSupportBean{TheString: "B-2", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "ContainedNestedTriggerOrders", unnestOrder{OrderID: "O-trigger-1", Books: []unnestBook{{ID: "B-1"}, {ID: "B-2"}}}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("theString").Any() != "B-2" || rows[0].Get("intPrimitive").Any() != int64(1) {
		t.Fatalf("nested named-window on-trigger rows = %#v, want B-2/1", rows)
	}
	if err := engine.InsertNamedWindow(context.Background(), "ContainedNestedTriggerSupportWindow", containedNestedSupportBean{TheString: "B-1", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "ContainedNestedTriggerOrders", unnestOrder{OrderID: "O-trigger-2", Books: []unnestBook{{ID: "B-3"}, {ID: "B-1"}}}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1].Get("theString").Any() != "B-1" || rows[1].Get("intPrimitive").Any() != int64(2) {
		t.Fatalf("nested named-window on-trigger rows = %#v, want second B-1/2", rows)
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

func TestUnnestParticipatesInJoin(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[unnestOrder](env, "UnnestOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[unnestBook](env, "UnnestBook"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[unnestPayment](env, "UnnestPayment"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	books := buildUnnestBookStream(env)
	payments := From[unnestPayment](env, "UnnestPayment")
	query := Join(books, payments, OnEqual(
		Field[unnestBook, string]("id"),
		Field[unnestPayment, string]("bookId"),
	)).Select(
		SelectFrom(0, "bookID", JoinField[string](0, "id")),
		SelectFrom(1, "amount", JoinField[int64](1, "amount")),
	).Query(StatementName("unnest-join"))
	plan, err := env.Build(query)
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
	if err := engine.SendEvent(context.Background(), unnestOrder{
		OrderID: "O-join",
		Books:   []unnestBook{{ID: "B-1"}, {ID: "B-2"}},
	}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("incomplete contained join emitted: %#v", rows)
	}
	if err := engine.SendEvent(context.Background(), unnestPayment{BookID: "B-2", Amount: 19}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("bookID").Any() != "B-2" || rows[0].Get("amount").Any() != int64(19) {
		t.Fatalf("contained join rows = %#v", rows)
	}
}
