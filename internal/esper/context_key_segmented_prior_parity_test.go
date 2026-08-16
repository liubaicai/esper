package esper

import (
	"context"
	"testing"
)

// TestKeyContextPriorPerPartitionParity locks the per-partition prior
// history of a keyed context verified against ContextKeySegmentedPrior:
// `partition by theString` projecting `prior(1, intPrimitive)` (Go Prior
// offset 0). Each partition keeps its own arrival history, so interleaved
// G1/G2 streams see only their own previous event; the first event of a
// partition has null prior.
func TestKeyContextPriorPerPartitionParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegPatternBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	source := From[keySegPatternBean](env, "SupportBean")
	theString := Field[keySegPatternBean, string]("theString")
	intPrimitive := Field[keySegPatternBean, int]("intPrimitive")
	if _, err := CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(source,
		Alias("val0", intPrimitive),
		Alias("val1", Prior[int](0, intPrimitive)),
	).Query(StatementName("s0"), WithContext("SegmentedByString")))
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
		val0, val1 int
		hasPrior   bool
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				value := rowValue.Get("val1").Any()
				entry := row{val0: rowValue.Get("val0").Any().(int)}
				if value == nil {
					entry.hasPrior = false
				} else {
					entry.val1 = value.(int)
					entry.hasPrior = true
				}
				rows = append(rows, entry)
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
	send("G1", 10)
	send("G2", 20)
	send("G1", 11)
	send("G2", 21)
	send("G1", 12)
	send("G2", 22)
	want := []row{
		{val0: 10, hasPrior: false},
		{val0: 20, hasPrior: false},
		{val0: 11, val1: 10, hasPrior: true},
		{val0: 21, val1: 20, hasPrior: true},
		{val0: 12, val1: 11, hasPrior: true},
		{val0: 22, val1: 21, hasPrior: true},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %#v, want %#v", rows, want)
	}
	for index := range want {
		if rows[index] != want[index] {
			t.Fatalf("rows[%d] = %#v, want %#v", index, rows[index], want[index])
		}
	}
}
