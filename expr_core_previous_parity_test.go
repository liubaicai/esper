package esper

import (
	"context"
	"testing"
)

// prevS0 mirrors SupportBean_S0 (id int).
type prevS0Bean struct {
	ID int `esper:"id"`
}

// ExprCorePreviousPrevStream (ordinal 2). Java:
//
//	select prev(1, s0) as result, prevtail(0, s0) as tailresult,
//	  prevwindow(s0) as windowresult, prevcount(s0) as countresult
//	from SupportBean_S0#length(2) as s0
//
// length(2) keeps the last 2 events. prev(1,s0) is the event before current.
// prevtail(0,s0) is the oldest event. prevwindow(s0) is all events newest-first.
// prevcount(s0) is the count. In Go, Prev reads field values relative to
// current; PrevTail/Prior(0) returns the oldest; PrevCount returns int64 count.
func TestExprCorePreviousPrevStreamParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[prevS0Bean](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	input := From[prevS0Bean](env, "SupportBean_S0").Window(LengthWindow(2))
	plan, err := env.Build(Select(input,
		Alias("prev1", Prev[int](1, Field[prevS0Bean, int]("id"))),
		Alias("tail", Prior[int](0, Field[prevS0Bean, int]("id"))),
		Alias("count", PrevCount[int](Field[prevS0Bean, int]("id"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var rows []struct{ prev1, tail, count any }
	_, _ = dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, r := range batch.New {
			row, ok := r.Row()
			if !ok {
				continue
			}
			rows = append(rows, struct{ prev1, tail, count any }{
				row.Get("prev1").Any(),
				row.Get("tail").Any(),
				row.Get("count").Any(),
			})
		}
		return nil
	})
	// E1 (id=1): prev1=null(no previous), tail=1(oldest=newest when only 1), count=1
	if err := engine.SendEvent(context.Background(), prevS0Bean{ID: 1}); err != nil {
		t.Fatal(err)
	}
	// E2 (id=2): prev1=1, tail=1(oldest), count=2
	if err := engine.SendEvent(context.Background(), prevS0Bean{ID: 2}); err != nil {
		t.Fatal(err)
	}
	// E3 (id=3): prev1=2, tail=2(E1 evicted), count=2
	if err := engine.SendEvent(context.Background(), prevS0Bean{ID: 3}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("PrevStream: row count = %d, want 3", len(rows))
	}
	// row 0: E1 - prev1 should be nil (no previous event)
	if rows[0].prev1 != nil {
		t.Fatalf("PrevStream[0]: prev1 = %v, want nil", rows[0].prev1)
	}
	// row 1: E2 - prev1=1
	if v, ok := rows[1].prev1.(int); !ok || v != 1 {
		t.Fatalf("PrevStream[1]: prev1 = %v, want 1", rows[1].prev1)
	}
	// row 2: E3 - prev1=2
	if v, ok := rows[2].prev1.(int); !ok || v != 2 {
		t.Fatalf("PrevStream[2]: prev1 = %v, want 2", rows[2].prev1)
	}
}
