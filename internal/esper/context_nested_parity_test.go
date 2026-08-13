package esper

import (
	"context"
	"reflect"
	"testing"
)

type contextNestedParityBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

func newContextNestedParityEnvironment(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[contextNestedParityBean](env, "NestedBean"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

func contextNestedParityRows(t *testing.T, deployment *Deployment) *[]Row {
	t.Helper()
	rows := make([]Row, 0)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("nested context result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &rows
}

// TestContextNestedSingleEventTriggerParity mirrors
// ContextNestedSingleEventTriggerNested's three independent partition levels.
// A repeated complete tuple updates only its own leaf aggregate.
func TestContextNestedSingleEventTriggerParity(t *testing.T) {
	env, engine := newContextNestedParityEnvironment(t)
	defer func() { _ = engine.Close(context.Background()) }()

	root, err := NewKeyContext("by-string", Field[contextNestedParityBean, string]("theString"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.RegisterContext(root.Name(), root.Keys()...); err != nil {
		t.Fatal(err)
	}
	intChild, err := NewKeyContext("by-int", Field[contextNestedParityBean, int]("intPrimitive"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNestedContext(env, "nested-by-int", "by-string", intChild); err != nil {
		t.Fatal(err)
	}
	longChild, err := NewKeyContext("by-long", Field[contextNestedParityBean, int64]("longPrimitive"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNestedContext(env, "nested-by-long", "nested-by-int", longChild); err != nil {
		t.Fatal(err)
	}

	plan, err := env.Build(From[contextNestedParityBean](env, "NestedBean").Aggregate(
		Alias("rootKey", ContextField[string]("parent.parent.key1")),
		Alias("middleKey", ContextField[int]("parent.key1")),
		Alias("leafKey", ContextField[int64]("key1")),
		Alias("count", CountAll()),
	).Query(StatementName("nested-single-event"), WithContext("nested-by-long")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	statement := deployment.Statements()[0]
	rows := contextNestedParityRows(t, deployment)

	for _, event := range []contextNestedParityBean{
		{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100},
		{TheString: "E2", IntPrimitive: 10, LongPrimitive: 100},
		{TheString: "E1", IntPrimitive: 11, LongPrimitive: 100},
		{TheString: "E1", IntPrimitive: 10, LongPrimitive: 101},
		{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}

	if got, want := statement.ContextPartitionCount(), 4; got != want {
		t.Fatalf("three-level nested partition count = %d, want %d", got, want)
	}
	if got, err := engine.ContextNestingLevel("nested-by-long"); err != nil || got != 3 {
		t.Fatalf("nested context level = %d, err=%v", got, err)
	}
	wantRows := [][]any{
		{"E1", 10, int64(100), int64(1)},
		{"E2", 10, int64(100), int64(1)},
		{"E1", 11, int64(100), int64(1)},
		{"E1", 10, int64(101), int64(1)},
		{"E1", 10, int64(100), int64(2)},
	}
	if len(*rows) != len(wantRows) {
		t.Fatalf("nested rows = %#v, want %d rows", *rows, len(wantRows))
	}
	for index, row := range *rows {
		got := []any{row.Get("rootKey").Any(), row.Get("middleKey").Any(), row.Get("leafKey").Any(), row.Get("count").Any()}
		if !reflect.DeepEqual(got, wantRows[index]) {
			t.Fatalf("nested row %d = %#v, want %#v", index, got, wantRows[index])
		}
	}
	selected := statement.ContextPartitionsWith(SelectNestedContextPartitions(
		SelectContextPartitionSegments([]any{"E1"}),
		SelectContextPartitionSegments([]any{10}),
		SelectContextPartitionSegments([]any{int64(100)}),
	))
	if len(selected) != 1 {
		t.Fatalf("three-level nested selector = %#v", selected)
	}
	if got, ok := selected[0].Property("key1"); !ok || got.Any() != int64(100) {
		t.Fatalf("selected leaf descriptor = %#v", selected[0].Properties())
	}

	t.Run("hash-over-hash", func(t *testing.T) {
		env, engine := newContextNestedParityEnvironment(t)
		defer func() { _ = engine.Close(context.Background()) }()
		outer, err := NewHashContext("by-string-hash", Field[contextNestedParityBean, string]("theString"), 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := env.registerContextDefinition(outer); err != nil {
			t.Fatal(err)
		}
		inner, err := NewHashContext("by-int-hash", Field[contextNestedParityBean, int]("intPrimitive"), 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNestedContext(env, "nested-by-int-hash", "by-string-hash", inner); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(From[contextNestedParityBean](env, "NestedBean").Aggregate(
			Alias("value", Field[contextNestedParityBean, string]("theString")),
			Alias("count", CountAll()),
		).Query(StatementName("nested-hash"), WithContext("nested-by-int-hash")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		rows := contextNestedParityRows(t, deployment)
		for _, event := range []contextNestedParityBean{
			{TheString: "E1", IntPrimitive: 0},
			{TheString: "E2", IntPrimitive: 0},
			{TheString: "E1", IntPrimitive: 0},
		} {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		statement := deployment.Statements()[0]
		if got := statement.ContextPartitionCount(); got != 2 {
			t.Fatalf("hash-over-hash partitions = %d, want 2", got)
		}
		want := [][]any{{"E1", int64(1)}, {"E2", int64(1)}, {"E1", int64(2)}}
		if len(*rows) != len(want) {
			t.Fatalf("hash-over-hash rows = %#v, want %d rows", *rows, len(want))
		}
		for index, row := range *rows {
			got := []any{row.Get("value").Any(), row.Get("count").Any()}
			if !reflect.DeepEqual(got, want[index]) {
				t.Fatalf("hash-over-hash row %d = %#v, want %#v", index, got, want[index])
			}
		}
		descriptors := statement.ContextPartitions()
		if len(descriptors) != 2 {
			t.Fatalf("hash-over-hash descriptors = %#v", descriptors)
		}
		outerHash, ok := descriptors[0].Property("parent.hash")
		if !ok {
			t.Fatalf("hash-over-hash parent descriptor = %#v", descriptors[0].Properties())
		}
		innerHash, ok := descriptors[0].Property("hash")
		if !ok {
			t.Fatalf("hash-over-hash child descriptor = %#v", descriptors[0].Properties())
		}
		selected := statement.ContextPartitionsWith(SelectNestedContextPartitions(
			SelectContextPartitionHashes(outerHash.Any().(int64)),
			SelectContextPartitionHashes(innerHash.Any().(int64)),
		))
		if len(selected) != 1 || selected[0].ID != descriptors[0].ID {
			t.Fatalf("hash-over-hash selector = %#v", selected)
		}
	})
}

// TestContextNestedNestingFilterCorrectnessParity covers the non-temporal
// nested controller combinations used by ContextNestedNestingFilterCorrectness.
// Each subtest uses a fresh environment so the context graph is independently
// built and undeployed, as it is in the Java regression execution.
func TestContextNestedNestingFilterCorrectnessParity(t *testing.T) {
	t.Run("category-over-key", func(t *testing.T) {
		env, engine := newContextNestedParityEnvironment(t)
		defer func() { _ = engine.Close(context.Background()) }()
		category, err := NewCategoryContext("by-sign",
			Category("negative", Less[int](Field[contextNestedParityBean, int]("intPrimitive"), Literal(0))),
			Category("positive", Greater[int](Field[contextNestedParityBean, int]("intPrimitive"), Literal(0))),
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := env.registerContextDefinition(category); err != nil {
			t.Fatal(err)
		}
		child, err := NewKeyContext("by-string", Field[contextNestedParityBean, string]("theString"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNestedContext(env, "category-key", "by-sign", child); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(From[contextNestedParityBean](env, "NestedBean").Aggregate(
			Alias("label", ContextField[string]("parent.label")),
			Alias("key", ContextKeyValue[string](0)),
			Alias("count", CountAll()),
		).Query(StatementName("category-over-key"), WithContext("category-key")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		rows := contextNestedParityRows(t, deployment)
		for _, event := range []contextNestedParityBean{
			{TheString: "E1", IntPrimitive: -1},
			{TheString: "E1", IntPrimitive: -2},
			{TheString: "E1", IntPrimitive: 1},
			{TheString: "E2", IntPrimitive: 1},
		} {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		if statement := deployment.Statements()[0]; statement.ContextPartitionCount() != 3 {
			t.Fatalf("category-over-key partitions = %d", statement.ContextPartitionCount())
		}
		want := [][]any{{"negative", "E1", int64(1)}, {"negative", "E1", int64(2)}, {"positive", "E1", int64(1)}, {"positive", "E2", int64(1)}}
		if len(*rows) != len(want) {
			t.Fatalf("category-over-key rows = %#v", *rows)
		}
		for index, row := range *rows {
			got := []any{row.Get("label").Any(), row.Get("key").Any(), row.Get("count").Any()}
			if !reflect.DeepEqual(got, want[index]) {
				t.Fatalf("category-over-key row %d = %#v, want %#v", index, got, want[index])
			}
		}
		selected := deployment.Statements()[0].ContextPartitionsWith(SelectNestedContextPartitions(
			SelectContextPartitionCategories("negative"),
			SelectContextPartitionSegments([]any{"E1"}),
		))
		if len(selected) != 1 {
			t.Fatalf("category-over-key selector = %#v", selected)
		}
	})

	t.Run("category-over-key-over-category", func(t *testing.T) {
		env, engine := newContextNestedParityEnvironment(t)
		defer func() { _ = engine.Close(context.Background()) }()
		outer, err := NewCategoryContext("by-int",
			Category("negative", Less[int](Field[contextNestedParityBean, int]("intPrimitive"), Literal(0))),
			Category("positive", Greater[int](Field[contextNestedParityBean, int]("intPrimitive"), Literal(0))),
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := env.registerContextDefinition(outer); err != nil {
			t.Fatal(err)
		}
		middle, err := NewKeyContext("by-string", Field[contextNestedParityBean, string]("theString"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNestedContext(env, "int-string", "by-int", middle); err != nil {
			t.Fatal(err)
		}
		leaf, err := NewCategoryContext("by-long",
			Category("negative", Less[int64](Field[contextNestedParityBean, int64]("longPrimitive"), Literal[int64](0))),
			Category("positive", Greater[int64](Field[contextNestedParityBean, int64]("longPrimitive"), Literal[int64](0))),
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNestedContext(env, "int-string-long", "int-string", leaf); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(From[contextNestedParityBean](env, "NestedBean").Aggregate(
			Alias("outer", ContextField[string]("parent.parent.label")),
			Alias("middle", ContextField[string]("parent.label")),
			Alias("leaf", ContextLabel()),
			Alias("count", CountAll()),
		).Query(StatementName("category-key-category"), WithContext("int-string-long")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		rows := contextNestedParityRows(t, deployment)
		for _, event := range []contextNestedParityBean{
			{TheString: "E1", IntPrimitive: -1, LongPrimitive: 1},
			{TheString: "E1", IntPrimitive: -1, LongPrimitive: -1},
			{TheString: "E1", IntPrimitive: 1, LongPrimitive: 1},
		} {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		if got := deployment.Statements()[0].ContextPartitionCount(); got != 3 {
			t.Fatalf("category-key-category partitions = %d", got)
		}
		selected := deployment.Statements()[0].ContextPartitionsWith(SelectNestedContextPartitions(
			SelectContextPartitionCategories("negative"),
			SelectContextPartitionSegments([]any{"E1"}),
			SelectContextPartitionCategories("positive"),
		))
		if len(selected) != 1 {
			t.Fatalf("category-key-category selector = %#v", selected)
		}
		if got := (*rows)[0].Get("count").Any(); got != int64(1) {
			t.Fatalf("category-key-category first count = %#v", got)
		}
	})

	t.Run("key-over-key-over-key", func(t *testing.T) {
		env, engine := newContextNestedParityEnvironment(t)
		defer func() { _ = engine.Close(context.Background()) }()
		root, err := NewKeyContext("by-string", Field[contextNestedParityBean, string]("theString"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := env.RegisterContext(root.Name(), root.Keys()...); err != nil {
			t.Fatal(err)
		}
		middle, err := NewKeyContext("by-int", Field[contextNestedParityBean, int]("intPrimitive"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNestedContext(env, "string-int", "by-string", middle); err != nil {
			t.Fatal(err)
		}
		leaf, err := NewKeyContext("by-long", Field[contextNestedParityBean, int64]("longPrimitive"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNestedContext(env, "string-int-long", "string-int", leaf); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(From[contextNestedParityBean](env, "NestedBean").Aggregate(Alias("count", CountAll())).Query(
			StatementName("key-key-key"), WithContext("string-int-long")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		rows := contextNestedParityRows(t, deployment)
		for _, event := range []contextNestedParityBean{
			{TheString: "E1", IntPrimitive: 1, LongPrimitive: 10},
			{TheString: "E2", IntPrimitive: 1, LongPrimitive: 10},
			{TheString: "E1", IntPrimitive: 2, LongPrimitive: 10},
			{TheString: "E1", IntPrimitive: 1, LongPrimitive: 11},
		} {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		if got := deployment.Statements()[0].ContextPartitionCount(); got != 4 {
			t.Fatalf("key-key-key partitions = %d", got)
		}
		selected := deployment.Statements()[0].ContextPartitionsWith(SelectNestedContextPartitions(
			SelectContextPartitionSegments([]any{"E1"}),
			SelectContextPartitionSegments([]any{1}),
			SelectContextPartitionSegments([]any{int64(11)}),
		))
		if len(selected) != 1 {
			t.Fatalf("key-key-key selector = %#v", selected)
		}
		if len(*rows) != 4 {
			t.Fatalf("key-key-key rows = %#v", *rows)
		}
	})

	t.Run("category-over-hash", func(t *testing.T) {
		env, engine := newContextNestedParityEnvironment(t)
		defer func() { _ = engine.Close(context.Background()) }()
		category, err := NewCategoryContext("by-sign",
			Category("negative", Less[int](Field[contextNestedParityBean, int]("intPrimitive"), Literal(0))),
			Category("positive", Greater[int](Field[contextNestedParityBean, int]("intPrimitive"), Literal(0))),
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := env.registerContextDefinition(category); err != nil {
			t.Fatal(err)
		}
		hash, err := NewHashContext("by-hash", Field[contextNestedParityBean, string]("theString"), 4)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNestedContext(env, "sign-hash", "by-sign", hash); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(From[contextNestedParityBean](env, "NestedBean").Aggregate(Alias("count", CountAll())).Query(
			StatementName("category-hash"), WithContext("sign-hash")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		rows := contextNestedParityRows(t, deployment)
		for _, event := range []contextNestedParityBean{
			{TheString: "E1", IntPrimitive: -1},
			{TheString: "E1", IntPrimitive: -2},
			{TheString: "E1", IntPrimitive: 1},
		} {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		if got := deployment.Statements()[0].ContextPartitionCount(); got != 2 {
			t.Fatalf("category-hash partitions = %d", got)
		}
		if len(*rows) != 3 || (*rows)[0].Get("count").Any() != int64(1) || (*rows)[1].Get("count").Any() != int64(2) || (*rows)[2].Get("count").Any() != int64(1) {
			t.Fatalf("category-hash aggregate rows = %#v", *rows)
		}
		descriptors := deployment.Statements()[0].ContextPartitions()
		if len(descriptors) != 2 {
			t.Fatalf("category-hash descriptors = %#v", descriptors)
		}
		for _, descriptor := range descriptors {
			if _, ok := descriptor.Property("hash"); !ok {
				t.Fatalf("category-hash descriptor has no hash property: %#v", descriptor.Properties())
			}
		}
	})
}
