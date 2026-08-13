package esper

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"
)

type contextKeySegmentedParityBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type contextKeySegmentedNullableBean struct {
	TheString    *string `esper:"theString"`
	IntPrimitive int     `esper:"intPrimitive"`
}

type contextKeySegmentedParityS0 struct {
	ID int `esper:"id"`
}

func contextKeySegmentedRows(t *testing.T, result QueryResult) []Row {
	t.Helper()
	rows := make([]Row, 0, len(result.Results()))
	for _, item := range result.Results() {
		row, ok := item.Row()
		if !ok {
			t.Fatalf("context key segmented snapshot result is not a row: %#v", item)
		}
		rows = append(rows, row)
	}
	return rows
}

// TestContextKeySegmentedSelectorParity mirrors ContextKeySegmentedSelector.
// SnapshotWithSelector is the typed snapshot boundary for Esper's targeted,
// filtered, empty and unknown partition iterator cases.
func TestContextKeySegmentedSelectorParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextKeySegmentedParityBean](env, "ContextKeySegmentedBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "PartitionedByString", Field[contextKeySegmentedParityBean, string]("theString")); err != nil {
		t.Fatal(err)
	}

	plan, err := env.Build(From[contextKeySegmentedParityBean](env, "ContextKeySegmentedBean").Window(LengthWindow(5)).Aggregate(
		Alias("c0", ContextKeyValue[string](0)),
		Alias("c1", Sum[int](Field[contextKeySegmentedParityBean, int]("intPrimitive"))),
	).Query(StatementName("s0"), WithContext("PartitionedByString")))
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

	statement := deployment.Statements()[0]
	send := func(event contextKeySegmentedParityBean) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	send(contextKeySegmentedParityBean{TheString: "E1", IntPrimitive: 10})
	send(contextKeySegmentedParityBean{TheString: "E2", IntPrimitive: 20})
	send(contextKeySegmentedParityBean{TheString: "E2", IntPrimitive: 21})

	all, err := statement.SnapshotWithSelector(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	allRows := contextKeySegmentedRows(t, all)
	wantAll := [][]any{{"E1", 10}, {"E2", 41}}
	if len(allRows) != len(wantAll) {
		t.Fatalf("all context key rows = %#v, want %#v", allRows, wantAll)
	}
	for index, row := range allRows {
		got := []any{row.Get("c0").Any(), row.Get("c1").Any()}
		if !reflect.DeepEqual(got, wantAll[index]) {
			t.Fatalf("all context key row %d = %#v, want %#v", index, got, wantAll[index])
		}
	}

	targeted, err := statement.SnapshotWithSelector(context.Background(), SelectContextPartitionSegments([]any{"E2"}))
	if err != nil {
		t.Fatal(err)
	}
	if rows := contextKeySegmentedRows(t, targeted); len(rows) != 1 || !reflect.DeepEqual([]any{rows[0].Get("c0").Any(), rows[0].Get("c1").Any()}, []any{"E2", 41}) {
		t.Fatalf("targeted context key snapshot = %#v", rows)
	}

	filtered, err := statement.SnapshotWithSelector(context.Background(), ContextPartitionSelectorDescriptorFunc(func(descriptor ContextPartitionDescriptor) bool {
		value, ok := descriptor.Property("key1")
		return ok && value.Any() == "E2"
	}))
	if err != nil {
		t.Fatal(err)
	}
	if rows := contextKeySegmentedRows(t, filtered); len(rows) != 1 || rows[0].Get("c1").Any() != 41 {
		t.Fatalf("filtered context key snapshot = %#v", rows)
	}

	for name, selector := range map[string]ContextPartitionSelector{
		"empty":   SelectContextPartitionSegments(),
		"unknown": SelectContextPartitionSegments([]any{"EX"}),
	} {
		result, err := statement.SnapshotWithSelector(context.Background(), selector)
		if err != nil {
			t.Fatalf("%s selector: %v", name, err)
		}
		if rows := contextKeySegmentedRows(t, result); len(rows) != 0 {
			t.Fatalf("%s context key snapshot = %#v, want empty", name, rows)
		}
	}

	if _, err := statement.SnapshotWithSelector(context.Background(), SelectContextPartitionCategories("E2")); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("incompatible context selector error = %v, want ErrorInvalidRule", err)
	}
}

// TestContextKeySegmentedLargeNumberPartitionsParity mirrors
// ContextKeySegmentedLargeNumberPartitions. The plan deliberately retains
// previous-access and scalar last-event subquery expressions even though the
// Java execution asserts only the sum column.
func TestContextKeySegmentedLargeNumberPartitionsParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextKeySegmentedParityBean](env, "LargeContextKeyBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[contextKeySegmentedParityS0](env, "LargeContextKeyS0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "SegmentedByAString", Field[contextKeySegmentedParityBean, string]("theString")); err != nil {
		t.Fatal(err)
	}

	lastEvent := From[contextKeySegmentedParityS0](env, "LargeContextKeyS0").Window(LastEvent()).AsRecord()
	plan, err := env.Build(From[contextKeySegmentedParityBean](env, "LargeContextKeyBean").Window(KeepAll()).Aggregate(
		Alias("col1", Sum[int](Field[contextKeySegmentedParityBean, int]("intPrimitive"))),
		Alias("prev1", Prev[int](1, Field[contextKeySegmentedParityBean, int]("intPrimitive"))),
		Alias("prior1", Prior[int](0, Field[contextKeySegmentedParityBean, int]("intPrimitive"))),
		Alias("lastID", SubqueryValue[int](lastEvent, Field[contextKeySegmentedParityS0, int]("id"))),
	).Query(StatementName("s0"), WithContext("SegmentedByAString")))
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

	var rows int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, item := range batch.New {
			row, ok := item.Row()
			if !ok {
				return errors.New("large context key result is not a row")
			}
			want := rows
			if got := row.Get("col1").Any(); got != want {
				return errors.New("large context key sum mismatch")
			}
			if !row.Get("lastID").IsNull() {
				return errors.New("large context key last-event subquery should be null")
			}
			rows++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for index := 0; index < 10000; index++ {
		if err := engine.SendEvent(context.Background(), contextKeySegmentedParityBean{TheString: "E" + strconv.Itoa(index), IntPrimitive: index}); err != nil {
			t.Fatal(err)
		}
	}
	if rows != 10000 {
		t.Fatalf("large context key output rows = %d, want 10000", rows)
	}
}

// TestContextKeySegmentedNullSingleKeyParity mirrors
// ContextKeySegmentedNullSingleKey and verifies repeated nil keys share one
// partition while a present key remains independent.
func TestContextKeySegmentedNullSingleKeyParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextKeySegmentedNullableBean](env, "NullableContextKeyBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "MyContext", Field[contextKeySegmentedNullableBean, *string]("theString")); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[contextKeySegmentedNullableBean](env, "NullableContextKeyBean").Aggregate(
		Alias("cnt", CountAll()),
	).Query(StatementName("s0"), WithContext("MyContext")))
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

	var counts []int64
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, item := range batch.New {
			row, ok := item.Row()
			if !ok {
				return errors.New("nullable context key result is not a row")
			}
			value := row.Get("cnt").Any()
			count, ok := value.(int64)
			if !ok {
				return errors.New("nullable context key count has unexpected type")
			}
			counts = append(counts, count)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), contextKeySegmentedNullableBean{IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), contextKeySegmentedNullableBean{IntPrimitive: 20}); err != nil {
		t.Fatal(err)
	}
	key := "A"
	if err := engine.SendEvent(context.Background(), contextKeySegmentedNullableBean{TheString: &key, IntPrimitive: 30}); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(counts, []int64{1, 2, 1}) {
		t.Fatalf("nullable context key counts = %#v, want [1 2 1]", counts)
	}
	if got := deployment.Statements()[0].ContextPartitionCount(); got != 2 {
		t.Fatalf("nullable context key partitions = %d, want 2", got)
	}
}
