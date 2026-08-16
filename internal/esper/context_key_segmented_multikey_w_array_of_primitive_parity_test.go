package esper

import (
	"context"
	"testing"
)

type keySegMultikeyArrayBean struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
	Value int    `esper:"value"`
}

// TestKeyContextMultikeyArrayOfPrimitiveParity locks the array-keyed
// context partitioning verified against
// ContextKeySegmentedMultikeyWArrayOfPrimitive: `partition by array from
// SupportEventWithIntArray` with a per-partition sum. Equal array content
// shares a partition, while an empty array and a null array are distinct
// keys, matching Java's value-based array key semantics.
func TestKeyContextMultikeyArrayOfPrimitiveParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegMultikeyArrayBean](env, "SupportEventWithIntArray"); err != nil {
		t.Fatal(err)
	}
	source := From[keySegMultikeyArrayBean](env, "SupportEventWithIntArray")
	array := Field[keySegMultikeyArrayBean, []int]("array")
	value := Field[keySegMultikeyArrayBean, int]("value")
	if _, err := CreateKeyContext(env, "PartitionByArray", array); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(source.Aggregate(
		Alias("thesum", Sum[int](value)),
	).Query(StatementName("s0"), WithContext("PartitionByArray")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var sums []int
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				sums = append(sums, row.Get("thesum").Any().(int))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(id string, array []int, value int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegMultikeyArrayBean{ID: id, Array: array, Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	send("E1", []int{1, 2}, 10)
	send("E2", []int{1, 2}, 11)
	send("E3", []int{1}, 12)
	send("E4", []int{}, 13)
	send("E5", nil, 14)
	send("E10", nil, 20)
	send("E11", []int{1, 2}, 21)
	send("E12", []int{1}, 22)
	send("E13", []int{}, 23)
	want := []int{10, 21, 12, 13, 14, 34, 42, 34, 36}
	if len(sums) != len(want) {
		t.Fatalf("sums = %#v, want %#v", sums, want)
	}
	for index := range want {
		if sums[index] != want[index] {
			t.Fatalf("sums[%d] = %d, want %d (full: %#v)", index, sums[index], want[index], sums)
		}
	}
}
