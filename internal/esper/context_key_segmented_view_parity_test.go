package esper

import (
	"context"
	"testing"
)

type keySegmentedViewBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestKeyContextPerPartitionWindowAndPrevWindowParity locks the per-partition
// view semantics verified against ContextKeySegmentedViewSceneOne: a keyed
// context partitioned by theString keeps an independent length(2) window per
// partition, prevwindow projects newest-to-oldest, the iterator spans all
// partitions, and evicted rows carry a null prevwindow.
func TestKeyContextPerPartitionWindowAndPrevWindowParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegmentedViewBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	source := From[keySegmentedViewBean](env, "SupportBean")
	if _, err := CreateKeyContext(env, "SegmentedByString", Field[keySegmentedViewBean, string]("theString")); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(source.Window(LengthWindow(2)),
		Alias("intPrimitive", Field[keySegmentedViewBean, int]("intPrimitive")),
		Alias("pw", PrevWindow[Event](EventValue[Event]())),
	).Query(StatementName("s0"), WithContext("SegmentedByString"), WithOldStream()))
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
	var rows []Row
	var oldRows []Row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		for _, result := range batch.Old {
			if row, ok := result.Row(); ok {
				oldRows = append(oldRows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegmentedViewBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	send("G1", 10)
	send("G2", 20)
	send("G1", 11)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	third := rows[2]
	window, ok := third.Get("pw").Any().([]Event)
	if !ok || len(window) != 2 || window[0].Get("theString").Any() != "G1" || window[1].Get("theString").Any() != "G1" ||
		window[0].Get("intPrimitive").Any() != 11 || window[1].Get("intPrimitive").Any() != 10 {
		t.Fatalf("G1 second row pw = %#v", third.Get("pw"))
	}
	// Iterator spans both partitions in partition order.
	queryResult, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(queryResult.Batch.New) != 3 {
		t.Fatalf("iterator rows = %d, want 3", len(queryResult.Batch.New))
	}
	// G1/12 evicts G1/10: the old row's prevwindow is null.
	send("G1", 12)
	if len(oldRows) != 1 {
		t.Fatalf("old rows = %d, want 1", len(oldRows))
	}
	if !oldRows[0].Get("pw").IsNull() || oldRows[0].Get("intPrimitive").Any() != 10 {
		t.Fatalf("evicted row = %#v", oldRows[0].AsMap())
	}
	// G2 stays isolated: G2 partition window is [G2,20].
	send("G2", 21)
	if len(rows) != 5 {
		t.Fatalf("rows = %d, want 5", len(rows))
	}
	fifth := rows[4]
	windowTwo, ok := fifth.Get("pw").Any().([]Event)
	if !ok || len(windowTwo) != 2 || windowTwo[0].Get("theString").Any() != "G2" || windowTwo[1].Get("theString").Any() != "G2" {
		t.Fatalf("G2 second row pw = %#v", fifth.Get("pw"))
	}
}
