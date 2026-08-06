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
