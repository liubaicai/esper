package esper

import (
	"context"
	"testing"
)

type keySegNullKeysBean struct {
	TheString    *string `esper:"theString"`
	IntBoxed     *int    `esper:"intBoxed"`
	IntPrimitive int     `esper:"intPrimitive"`
}

// TestKeyContextNullKeyPartitioningParity locks the null-key partitioning
// of keyed contexts verified against ContextKeySegmentedNullSingleKey and
// ContextKeySegmentedNullKeyMultiKey. Null key values share one partition
// across events (two null-key events count 1 then 2), and a present value
// starts a separate partition (count restarts at 1). The multi-key form
// combines a nullable intBoxed with theString and intPrimitive.
func TestKeyContextNullKeyPartitioningParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegNullKeysBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	source := From[keySegNullKeysBean](env, "SupportBean")
	theString := Field[keySegNullKeysBean, *string]("theString")
	if _, err := CreateKeyContext(env, "MyContext", theString); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(source.Aggregate(Alias("cnt", CountAll())).Query(StatementName("s0"), WithContext("MyContext")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var counts []int64
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				counts = append(counts, row.Get("cnt").Any().(int64))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(s string, i int) {
		t.Helper()
		var sp *string
		if s != "" {
			sp = &s
		}
		if err := engine.SendEvent(context.Background(), keySegNullKeysBean{TheString: sp, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	send("", 10)
	send("", 20)
	send("A", 30)
	want := []int64{1, 2, 1}
	if len(counts) != len(want) {
		t.Fatalf("counts = %#v, want %#v", counts, want)
	}
	for index := range want {
		if counts[index] != want[index] {
			t.Fatalf("counts[%d] = %d, want %d", index, counts[index], want[index])
		}
	}
	_ = engine.Close(context.Background())

	// Multi-key form: (A, null, 1) twice shares a partition, (A, 10, 1)
	// starts a fresh one.
	env2 := NewEnvironment()
	if _, err := RegisterStruct[keySegNullKeysBean](env2, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	source2 := From[keySegNullKeysBean](env2, "SupportBean")
	theString2 := Field[keySegNullKeysBean, *string]("theString")
	intBoxed := Field[keySegNullKeysBean, *int]("intBoxed")
	intPrimitive := Field[keySegNullKeysBean, int]("intPrimitive")
	if _, err := CreateKeyContext(env2, "MyContext", theString2, intBoxed, intPrimitive); err != nil {
		t.Fatal(err)
	}
	plan2, err := env2.Build(source2.Aggregate(Alias("cnt", CountAll())).Query(StatementName("s0"), WithContext("MyContext")))
	if err != nil {
		t.Fatal(err)
	}
	engine2 := NewEngine(env2)
	defer func() { _ = engine2.Close(context.Background()) }()
	deployment2, err := engine2.Deploy(context.Background(), plan2)
	if err != nil {
		t.Fatal(err)
	}
	statement2 := deployment2.Statements()[0]
	counts2 := []int64(nil)
	if _, err := statement2.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				counts2 = append(counts2, row.Get("cnt").Any().(int64))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send2 := func(s string, boxed *int, i int) {
		t.Helper()
		var sp *string
		if s != "" {
			sp = &s
		}
		if err := engine2.SendEvent(context.Background(), keySegNullKeysBean{TheString: sp, IntBoxed: boxed, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	send2("A", nil, 1)
	send2("A", nil, 1)
	send2("A", intPtr(10), 1)
	want2 := []int64{1, 2, 1}
	if len(counts2) != len(want2) {
		t.Fatalf("multi-key counts = %#v, want %#v", counts2, want2)
	}
	for index := range want2 {
		if counts2[index] != want2[index] {
			t.Fatalf("multi-key counts[%d] = %d, want %d", index, counts2[index], want2[index])
		}
	}
}
