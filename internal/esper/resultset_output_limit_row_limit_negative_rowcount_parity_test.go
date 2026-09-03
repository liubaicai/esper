package esper

import (
	"context"
	"testing"
	"time"
)

type resultsetOutputLimitNegativeRowcountBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestResultSetOutputLimitRowLimitNegativeRowcountParity mirrors
// ResultSetGroupedSnapshotNegativeRowcount: a negative limit means unlimited,
// while offset still removes the first ordered result.
func TestResultSetOutputLimitRowLimitNegativeRowcountParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[resultsetOutputLimitNegativeRowcountBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	theString := Field[resultsetOutputLimitNegativeRowcountBean, string]("theString")
	intPrimitive := Field[resultsetOutputLimitNegativeRowcountBean, int]("intPrimitive")
	query := From[resultsetOutputLimitNegativeRowcountBean](env, "SupportBean").
		Window(LengthWindow(5)).
		GroupBy(theString).
		Select(Alias("theString", theString), Alias("mysum", Sum[int](intPrimitive))).
		Query(StatementName("s0"), WithOutput(OutputSnapshotEvery(10*time.Second)),
			OrderBy(Descending(ResultField[int]("mysum"))), Limit(-1), Offset(1))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.Build(From[resultsetOutputLimitNegativeRowcountBean](env, "SupportBean").Query(Offset(-1))); err == nil {
		t.Fatal("negative offset should be rejected")
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	if snapshot, err := statement.Snapshot(context.Background()); err != nil {
		t.Fatal(err)
	} else if len(snapshot.Results()) != 0 {
		t.Fatalf("initial iterator = %#v, want empty", snapshot.Results())
	}

	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []resultsetOutputLimitNegativeRowcountBean{
		{TheString: "E1", IntPrimitive: 10},
		{TheString: "E2", IntPrimitive: 5},
		{TheString: "E3", IntPrimitive: 20},
		{TheString: "E1", IntPrimitive: 30},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 0 {
		t.Fatalf("input-event listener output = %#v, want none", batches)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(11, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("snapshot batches = %d, want 1", len(batches))
	}
	batch := batches[0]
	if len(batch.New) != 2 || len(batch.Old) != 0 {
		t.Fatalf("snapshot batch = %#v, want two new and no old", batch)
	}
	for i, want := range []struct {
		name string
		sum  int
	}{{"E3", 20}, {"E2", 5}} {
		row, ok := batch.New[i].Row()
		if !ok {
			t.Fatalf("new result %d is not a row: %#v", i, batch.New[i])
		}
		if got := row.Get("theString").Any(); got != want.name {
			t.Fatalf("new row %d theString = %v, want %s", i, got, want.name)
		}
		if got := row.Get("mysum").Any(); got != want.sum {
			t.Fatalf("new row %d mysum = %v, want %d", i, got, want.sum)
		}
	}
}
