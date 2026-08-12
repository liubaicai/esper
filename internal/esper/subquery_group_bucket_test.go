package esper

import (
	"context"
	"reflect"
	"testing"
)

type subqueryArrayGroupEvent struct {
	ID    string  `esper:"id"`
	Array []int64 `esper:"array"`
	Value int64   `esper:"value"`
}

type subqueryArrayGroupTrigger struct {
	ID int64 `esper:"id"`
}

func TestSubqueryGroupByAnySupportsArrayKeysAndStableBuckets(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[subqueryArrayGroupEvent](env, "SubqueryArrayGroupEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subqueryArrayGroupTrigger](env, "SubqueryArrayGroupTrigger"); err != nil {
		t.Fatal(err)
	}

	inner := Select(From[subqueryArrayGroupEvent](env, "SubqueryArrayGroupEvent")).Window(KeepAll())
	key := Field[any, []int64]("array")
	value := Field[any, int64]("value")
	groups := SubqueryGroupByAny[[]int64, int64](inner, key, value)
	sums := SubqueryGroupByAny[[]int64, int64](inner, key, Sum[int64](value))
	plan, err := env.Build(Select(
		From[subqueryArrayGroupTrigger](env, "SubqueryArrayGroupTrigger"),
		Alias("groups", groups),
		Alias("sums", sums),
	).Query(StatementName("subquery-array-groups")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("array-key subquery result = %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	send := func(event subqueryArrayGroupEvent) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	send(subqueryArrayGroupEvent{ID: "E1", Array: []int64{1, 2}, Value: 10})
	send(subqueryArrayGroupEvent{ID: "E2", Array: []int64{1, 2}, Value: 11})
	send(subqueryArrayGroupEvent{ID: "E3", Array: []int64{1}, Value: 13})
	send(subqueryArrayGroupEvent{ID: "E4", Array: []int64{1, 2}, Value: 12})
	send(subqueryArrayGroupEvent{ID: "E5", Array: nil, Value: 99})
	if err := engine.SendEvent(context.Background(), subqueryArrayGroupTrigger{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("array-key subquery rows = %#v", rows)
	}

	got, ok := rows[0].Get("groups").Any().([]SubqueryGroupBucket[[]int64, int64])
	if !ok {
		t.Fatalf("array-key group result type = %T", rows[0].Get("groups").Any())
	}
	if len(got) != 3 {
		t.Fatalf("array-key bucket count = %#v", got)
	}
	if !reflect.DeepEqual(got[0].Key, []int64{1, 2}) || !reflect.DeepEqual(got[0].Values, []int64{10, 11, 12}) || !got[0].KeyValue.IsPresent() {
		t.Fatalf("first array-key bucket = %#v", got[0])
	}
	if !reflect.DeepEqual(got[1].Key, []int64{1}) || !reflect.DeepEqual(got[1].Values, []int64{13}) {
		t.Fatalf("second array-key bucket = %#v", got[1])
	}
	// A nil slice stored in a struct is a present typed-nil value in the Go
	// Value model; it is intentionally distinct from a nil interface/Null.
	if got[2].Key != nil || !got[2].KeyValue.IsPresent() || !reflect.DeepEqual(got[2].Values, []int64{99}) {
		t.Fatalf("typed-nil array-key bucket = %#v", got[2])
	}

	groupSums, ok := rows[0].Get("sums").Any().([]SubqueryGroupBucket[[]int64, int64])
	if !ok || len(groupSums) != 3 || !reflect.DeepEqual(groupSums[0].Values, []int64{33}) || !reflect.DeepEqual(groupSums[1].Values, []int64{13}) || !reflect.DeepEqual(groupSums[2].Values, []int64{99}) {
		t.Fatalf("array-key aggregate buckets = %#v", rows[0].Get("sums"))
	}
}
