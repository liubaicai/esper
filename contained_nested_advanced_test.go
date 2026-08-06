package esper

import (
	"context"
	"reflect"
	"testing"
)

type containedAdvancedReview struct {
	ReviewID int64 `esper:"reviewId"`
	MatchID  int64 `esper:"matchId"`
}

type containedAdvancedBook struct {
	BookID  string                    `esper:"bookId"`
	Title   string                    `esper:"title"`
	Reviews []containedAdvancedReview `esper:"reviews"`
}

type containedAdvancedOrder struct {
	OrderID string                  `esper:"orderId"`
	Books   []containedAdvancedBook `esper:"books"`
}

type containedAdvancedSupport struct {
	TheString    string `esper:"theString"`
	IntPrimitive int64  `esper:"intPrimitive"`
	MatchID      int64  `esper:"matchId"`
}

func buildContainedAdvancedReviews(env *Environment) (Stream[containedAdvancedReview], error) {
	orders := From[containedAdvancedOrder](env, "ContainedAdvancedOrder")
	books := Unnest[containedAdvancedOrder, containedAdvancedBook](orders, Property[[]containedAdvancedBook](EventValue[containedAdvancedOrder](), "books"))
	reviews := Unnest[containedAdvancedBook, containedAdvancedReview](books, Property[[]containedAdvancedReview](EventValue[containedAdvancedBook](), "reviews"))
	return reviews, nil
}

func registerContainedAdvancedTypes(t *testing.T, env *Environment) {
	t.Helper()
	for _, register := range []func() (Schema, error){
		func() (Schema, error) { return RegisterStruct[containedAdvancedOrder](env, "ContainedAdvancedOrder") },
		func() (Schema, error) { return RegisterStruct[containedAdvancedBook](env, "ContainedAdvancedBook") },
		func() (Schema, error) { return RegisterStruct[containedAdvancedReview](env, "ContainedAdvancedReview") },
		func() (Schema, error) {
			return RegisterStruct[containedAdvancedSupport](env, "ContainedAdvancedSupport")
		},
	} {
		if _, err := register(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestContainedNestedPatternSelectMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	registerContainedAdvancedTypes(t, env)
	reviews, err := buildContainedAdvancedReviews(env)
	if err != nil {
		t.Fatal(err)
	}
	support := From[containedAdvancedSupport](env, "ContainedAdvancedSupport")

	// Every() is applied to the complete sequence so each contained review
	// gets its own waiting branch, matching `every r=... -> SupportBean(...)`.
	left := PatternFrom(reviews, "r", Literal[bool](true))
	right := PatternFrom(support, "s", Equal[int64](
		Field[containedAdvancedSupport, int64]("intPrimitive"),
		TagField[int64]("r", "reviewId"),
	))
	plan, err := env.Build(left.Then(right).Every().Select(
		Alias("reviewId", TagField[int64]("r", "reviewId")),
	).Query(StatementName("contained-nested-pattern")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	var rows []int64
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("contained pattern result is not a row: %#v", result)
			}
			rows = append(rows, row.Get("reviewId").Any().(int64))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), containedAdvancedOrder{
		OrderID: "O-pattern",
		Books: []containedAdvancedBook{
			{BookID: "B-1", Reviews: []containedAdvancedReview{{ReviewID: 1, MatchID: 1}, {ReviewID: 2, MatchID: 2}}},
			{BookID: "B-2", Reviews: []containedAdvancedReview{{ReviewID: 10, MatchID: 10}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), containedAdvancedSupport{TheString: "E1", IntPrimitive: 1, MatchID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), containedAdvancedSupport{TheString: "E-no", IntPrimitive: -1, MatchID: -1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), containedAdvancedSupport{TheString: "E2", IntPrimitive: 10, MatchID: 10}); err != nil {
		t.Fatal(err)
	}
	if want := []int64{1, 10}; !reflect.DeepEqual(rows, want) {
		t.Fatalf("contained pattern rows = %#v, want %#v", rows, want)
	}
}

func TestContainedNestedWhereMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	registerContainedAdvancedTypes(t, env)
	orders := From[containedAdvancedOrder](env, "ContainedAdvancedOrder")
	books := Unnest[containedAdvancedOrder, containedAdvancedBook](orders, Property[[]containedAdvancedBook](EventValue[containedAdvancedOrder](), "books"))
	reviews := Unnest[containedAdvancedBook, containedAdvancedReview](books, Property[[]containedAdvancedReview](EventValue[containedAdvancedBook](), "reviews"))
	bookTitle := Equal[string](Field[containedAdvancedBook, string]("title"), Literal("Enders Game"))
	reviewID := Field[containedAdvancedReview, int64]("reviewId")
	reviewSet := InSlice[int64](reviewID, Literal([]int64{1, 10}))

	build := func(name string, stream Stream[containedAdvancedReview]) (*Engine, *Deployment, *[]int64) {
		t.Helper()
		plan, err := env.Build(Select(stream, Alias("reviewId", reviewID)).Query(StatementName(name), OrderBy(Ascending(reviewID))))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := make([]int64, 0)
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok {
					return nil
				}
				rows = append(rows, row.Get("reviewId").Any().(int64))
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return engine, deployment, &rows
	}

	rootEngine, rootFiltered, rootRows := build("contained-nested-where-root", Unnest[containedAdvancedBook, containedAdvancedReview](books.Filter(bookTitle), Property[[]containedAdvancedReview](EventValue[containedAdvancedBook](), "reviews")))
	defer func() { _ = rootFiltered.Undeploy(context.Background()) }()
	leafEngine, leafFiltered, leafRows := build("contained-nested-where-leaf", reviews.Filter(reviewSet))
	defer func() { _ = leafFiltered.Undeploy(context.Background()) }()
	combinedEngine, combined, combinedRows := build("contained-nested-where-combined", reviews.Filter(And(
		reviewSet,
		Equal[string](ContainedParentField[string]("title"), Literal("Enders Game")),
	)))
	defer func() { _ = combined.Undeploy(context.Background()) }()

	order := containedAdvancedOrder{
		OrderID: "O-where",
		Books: []containedAdvancedBook{
			{BookID: "B-1", Title: "Enders Game", Reviews: []containedAdvancedReview{{ReviewID: 1}, {ReviewID: 2}}},
			{BookID: "B-2", Title: "Other", Reviews: []containedAdvancedReview{{ReviewID: 10}}},
		},
	}
	for _, engine := range []*Engine{rootEngine, leafEngine, combinedEngine} {
		if err := engine.SendEvent(context.Background(), order); err != nil {
			t.Fatal(err)
		}
	}
	if want := []int64{1, 2}; !reflect.DeepEqual(*rootRows, want) {
		t.Fatalf("contained root where rows = %#v, want %#v", *rootRows, want)
	}
	if want := []int64{1, 10}; !reflect.DeepEqual(*leafRows, want) {
		t.Fatalf("contained leaf where rows = %#v, want %#v", *leafRows, want)
	}
	if want := []int64{1}; !reflect.DeepEqual(*combinedRows, want) {
		t.Fatalf("contained combined where rows = %#v, want %#v", *combinedRows, want)
	}
}

func TestContainedNestedSubselectMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	registerContainedAdvancedTypes(t, env)
	reviews, err := buildContainedAdvancedReviews(env)
	if err != nil {
		t.Fatal(err)
	}
	inner := Select(reviews).Window(Unique(Field[containedAdvancedReview, int64]("reviewId")))
	outer := From[containedAdvancedSupport](env, "ContainedAdvancedSupport").Filter(SubqueryExists(
		inner,
		Equal[int64](
			Field[containedAdvancedReview, int64]("reviewId"),
			OuterField[int64]("intPrimitive"),
		),
	))
	plan, err := env.Build(Select(outer,
		Alias("theString", Field[containedAdvancedSupport, string]("theString")),
	).Query(StatementName("contained-nested-subselect")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	var rows []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("contained subselect result is not a row: %#v", result)
			}
			rows = append(rows, row.Get("theString").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), containedAdvancedOrder{
		OrderID: "O-subselect",
		Books: []containedAdvancedBook{
			{BookID: "B-1", Reviews: []containedAdvancedReview{{ReviewID: 1, MatchID: 1}, {ReviewID: 2, MatchID: 2}}},
			{BookID: "B-2", Reviews: []containedAdvancedReview{{ReviewID: 10, MatchID: 10}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []containedAdvancedSupport{
		{TheString: "E1", IntPrimitive: 1},
		{TheString: "E-no", IntPrimitive: -1},
		{TheString: "E10", IntPrimitive: 10},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if want := []string{"E1", "E10"}; !reflect.DeepEqual(rows, want) {
		t.Fatalf("contained subselect rows = %#v, want %#v", rows, want)
	}
}

func TestContainedNestedUnderlyingParentSelectionMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	registerContainedAdvancedTypes(t, env)
	reviews, err := buildContainedAdvancedReviews(env)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(reviews,
		Alias("orderId", ContainedAncestorField[string](2, "orderId")),
		Alias("bookId", ContainedParentField[string]("bookId")),
		Alias("reviewId", Field[containedAdvancedReview, int64]("reviewId")),
	).Query(
		StatementName("contained-nested-underlying"),
		OrderBy(
			Ascending(Field[containedAdvancedReview, int64]("reviewId")),
		),
	))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("contained underlying result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), containedAdvancedOrder{
		OrderID: "PO200901",
		Books: []containedAdvancedBook{
			{BookID: "10020", Reviews: []containedAdvancedReview{{ReviewID: 2}, {ReviewID: 1}}},
			{BookID: "10021", Reviews: []containedAdvancedReview{{ReviewID: 10}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("contained underlying rows = %#v", rows)
	}
	want := []struct {
		orderID string
		bookID  string
		review  int64
	}{
		{"PO200901", "10020", 1},
		{"PO200901", "10020", 2},
		{"PO200901", "10021", 10},
	}
	for index, expected := range want {
		if got := rows[index]; got.Get("orderId").Any() != expected.orderID || got.Get("bookId").Any() != expected.bookID || got.Get("reviewId").Any() != expected.review {
			t.Fatalf("contained underlying row %d = %#v, want %#v", index, got, expected)
		}
	}
}

func TestContainedNestedParentFragmentsMatchEsper(t *testing.T) {
	env := NewEnvironment()
	registerContainedAdvancedTypes(t, env)
	reviews, err := buildContainedAdvancedReviews(env)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(reviews,
		Alias("orderFragment", ContainedAncestorEvent(2)),
		Alias("bookFragment", ContainedParentEvent()),
		Alias("reviewFragment", EventValue[Event]()),
	).Query(StatementName("contained-nested-fragments")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("contained fragment result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), containedAdvancedOrder{
		OrderID: "PO-fragment",
		Books: []containedAdvancedBook{
			{BookID: "B-fragment", Title: "Enders Game", Reviews: []containedAdvancedReview{{ReviewID: 7, MatchID: 7}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("contained fragment rows = %#v", rows)
	}
	order, ok := rows[0].Get("orderFragment").Any().(Event)
	if !ok || order.Get("orderId").Any() != "PO-fragment" {
		t.Fatalf("contained order fragment = %#v", rows[0].Get("orderFragment"))
	}
	book, ok := rows[0].Get("bookFragment").Any().(Event)
	if !ok || book.Get("bookId").Any() != "B-fragment" || book.Get("title").Any() != "Enders Game" {
		t.Fatalf("contained book fragment = %#v", rows[0].Get("bookFragment"))
	}
	review, ok := rows[0].Get("reviewFragment").Any().(Event)
	if !ok || review.Get("reviewId").Any() != int64(7) {
		t.Fatalf("contained review fragment = %#v", rows[0].Get("reviewFragment"))
	}
}

func TestContainedNestedInvalidRules(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[containedAdvancedOrder](env, "ContainedAdvancedOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[containedAdvancedBook](env, "ContainedAdvancedBook"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[containedAdvancedReview](env, "ContainedAdvancedReview"); err != nil {
		t.Fatal(err)
	}
	orders := From[containedAdvancedOrder](env, "ContainedAdvancedOrder")
	books := Unnest[containedAdvancedOrder, containedAdvancedBook](orders, Property[[]containedAdvancedBook](EventValue[containedAdvancedOrder](), "books"))
	reviews := Unnest[containedAdvancedBook, containedAdvancedReview](books, Property[[]containedAdvancedReview](EventValue[containedAdvancedBook](), "reviews"))

	if _, err := env.Build(Select(reviews,
		Alias("bad", ContainedAncestorField[string](2, "doesNotExist")),
	).Query(StatementName("contained-invalid-parent-field"))); err == nil {
		t.Fatal("expected unknown contained ancestor field to fail Build")
	}
	if _, err := env.Build(Select(reviews,
		Alias("bad", ContainedAncestorField[string](3, "orderId")),
	).Query(StatementName("contained-invalid-parent-depth"))); err == nil {
		t.Fatal("expected excessive contained ancestor depth to fail Build")
	}
	if _, err := env.Build(Select(reviews,
		Alias("bad", ContainedParentField[string]("orderId")),
	).Query(StatementName("contained-invalid-parent-schema"))); err == nil {
		t.Fatal("expected immediate contained parent schema mismatch to fail Build")
	}
}
