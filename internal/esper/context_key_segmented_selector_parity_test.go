package esper

import (
	"context"
	"testing"
)

// TestKeyContextSelectorKeyProjectionParity locks the keyed context key
// property projection verified against ContextKeySegmentedSelector:
// `partition by theString` with `select context.key1 as c0, sum(intPrimitive)
// as c1 from SupportBean#length(5)`. Every listener row carries the
// partition key, the sum accumulates per partition, and the cross-partition
// statement snapshot holds one row per partition.
func TestKeyContextSelectorKeyProjectionParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegPatternBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	source := From[keySegPatternBean](env, "SupportBean")
	theString := Field[keySegPatternBean, string]("theString")
	intPrimitive := Field[keySegPatternBean, int]("intPrimitive")
	if _, err := CreateKeyContext(env, "PartitionedByString", theString); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(source.Window(LengthWindow(5)).Aggregate(
		Alias("c0", ContextField[string]("key1")),
		Alias("c1", Sum[int](intPrimitive)),
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
	statement := deployment.Statements()[0]
	type row struct {
		key string
		sum int
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				rows = append(rows, row{
					key: rowValue.Get("c0").Any().(string),
					sum: rowValue.Get("c1").Any().(int),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegPatternBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	send("E1", 10)
	send("E2", 20)
	send("E2", 21)
	want := []row{{key: "E1", sum: 10}, {key: "E2", sum: 20}, {key: "E2", sum: 41}}
	if len(rows) != len(want) {
		t.Fatalf("rows = %#v, want %#v", rows, want)
	}
	for index := range want {
		if rows[index] != want[index] {
			t.Fatalf("rows[%d] = %#v, want %#v", index, rows[index], want[index])
		}
	}
}
