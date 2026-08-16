package esper

import (
	"context"
	"testing"
)

// TestKeyContextMultikeyArrayTwoFieldParity locks the two-field array-keyed
// context partitioning verified against
// ContextKeySegmentedMultikeyWArrayTwoField: `partition by id, array from
// SupportEventWithIntArray` with a per-partition sum. Both key fields
// participate — equal arrays under different ids are separate partitions,
// and different arrays under the same id accumulate independently.
func TestKeyContextMultikeyArrayTwoFieldParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegMultikeyArrayBean](env, "SupportEventWithIntArray"); err != nil {
		t.Fatal(err)
	}
	source := From[keySegMultikeyArrayBean](env, "SupportEventWithIntArray")
	id := Field[keySegMultikeyArrayBean, string]("id")
	array := Field[keySegMultikeyArrayBean, []int]("array")
	value := Field[keySegMultikeyArrayBean, int]("value")
	if _, err := CreateKeyContext(env, "PartitionByArray", id, array); err != nil {
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
	send("G1", []int{1, 2}, 1)
	send("G2", []int{1, 2}, 2)
	send("G1", []int{1}, 3)
	send("G2", []int{1, 2}, 10)
	send("G1", []int{1, 2}, 15)
	send("G1", []int{1}, 18)
	want := []int{1, 2, 3, 12, 16, 21}
	if len(sums) != len(want) {
		t.Fatalf("sums = %#v, want %#v", sums, want)
	}
	for index := range want {
		if sums[index] != want[index] {
			t.Fatalf("sums[%d] = %d, want %d (full: %#v)", index, sums[index], want[index], sums)
		}
	}
}
