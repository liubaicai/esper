package esper

import (
	"context"
	"testing"
)

// WithSelfSubselectPreeval(false) defers the triggering event's acceptance
// into the statement's own subselect windows until after the statement's
// filter and where clauses evaluate, so a self-subselect not-in filter
// observes the pre-arrival window state (Java Esper
// selfSubselectPreeval=false). The default must keep the preeval-on
// behavior: the event is in the window before the clauses evaluate.
func TestSelfSubselectPreevalDefaultAcceptsBeforeEvaluation(t *testing.T) {
	first, second := runSelfSubselectPreevalScenario(t, true)
	if len(first) != 0 || len(second) != 0 {
		t.Fatalf("preeval-on default: first = %v second = %v, want both silent (5 in {5} after pre-arrival acceptance)", first, second)
	}
}

func TestSelfSubselectPreevalOffDefersAcceptance(t *testing.T) {
	first, second := runSelfSubselectPreevalScenario(t, false)
	if len(first) != 1 || first[0] != int64(5) {
		t.Fatalf("preeval-off first send: rows = %v, want [5] (empty window at where-time)", first)
	}
	if len(second) != 0 {
		t.Fatalf("preeval-off second send: rows = %v, want none (deferred acceptance ingested the event)", second)
	}
}

// TestSelfSubselectPreevalOffIngestsFilterFailingEvent pins the posteval
// acceptance of an event that fails the outer filter: the subselect window
// has no outer-filter of its own (EPL1's inner stream is unfiltered), so an
// intPrimitive=15 event rejected by the where-clause round must still enter
// the unique window, making a later E1/5 see {5,15} and stay silent.
func TestSelfSubselectPreevalOffIngestsFilterFailingEvent(t *testing.T) {
	env := NewEnvironment()
	type bean struct {
		TheString    string `esper:"theString"`
		IntPrimitive int    `esper:"intPrimitive"`
	}
	if _, err := RegisterStruct[bean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	intPrimitive := Field[bean, int]("intPrimitive")
	inner := From[bean](env, "SupportBean").Window(Unique(intPrimitive)).AsRecord()
	query := From[bean](env, "SupportBean").
		Filter(Not(SubqueryIn[int](intPrimitive, inner, intPrimitive))).
		Query(StatementName("s0"), WithSelfSubselectPreeval(false))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var sendRows [][]int64
	for _, statement := range deployment.Statements() {
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			rows := make([]int64, 0, len(batch.New))
			for _, row := range batch.New {
				rows = append(rows, int64(row.Get("intPrimitive").Any().(int)))
			}
			sendRows = append(sendRows, rows)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	// Round 1: an event failing no filter here is not available in this
	// shape, so pin the ingestion of an out-of-window value directly: send
	// 15 (enters the window posteval), then 5 (the window now holds {15,5}
	// because 15 was deferred-in; 5 itself defers, so the where sees {15}
	// and 5 not in {15} passes).
	if err := engine.Send(context.Background(), "SupportBean", bean{TheString: "F1", IntPrimitive: 15}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SupportBean", bean{TheString: "E1", IntPrimitive: 5}); err != nil {
		t.Fatal(err)
	}
	if len(sendRows) != 2 || len(sendRows[1]) != 1 || sendRows[1][0] != int64(5) {
		t.Fatalf("posteval ingestion rows = %v, want round 2 to emit [5] (15 was ingested by the deferred acceptance of round 1)", sendRows)
	}
}

// runSelfSubselectPreevalScenario sends the same E1/5 event twice and
// returns the listener rows of each send. Under preeval-on both sends are
// silent (the first event already entered the unique window before the
// where evaluated); under preeval-off the first send passes the not-in
// filter over the still-empty window, and the deferred acceptance makes the
// second send silent.
func runSelfSubselectPreevalScenario(t *testing.T, preeval bool) ([]int64, []int64) {
	t.Helper()
	env := NewEnvironment()
	type bean struct {
		TheString    string `esper:"theString"`
		IntPrimitive int    `esper:"intPrimitive"`
	}
	if _, err := RegisterStruct[bean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	intPrimitive := Field[bean, int]("intPrimitive")
	inner := From[bean](env, "SupportBean").Window(Unique(intPrimitive)).AsRecord()
	query := From[bean](env, "SupportBean").
		Filter(Not(SubqueryIn[int](intPrimitive, inner, intPrimitive))).
		Query(StatementName("s0"), WithSelfSubselectPreeval(preeval))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var sendRows [][]int64
	for _, statement := range deployment.Statements() {
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			rows := make([]int64, 0, len(batch.New))
			for _, row := range batch.New {
				rows = append(rows, int64(row.Get("intPrimitive").Any().(int)))
			}
			sendRows = append(sendRows, rows)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	for send := 0; send < 2; send++ {
		if err := engine.Send(context.Background(), "SupportBean", bean{TheString: "E1", IntPrimitive: 5}); err != nil {
			t.Fatal(err)
		}
	}
	if len(sendRows) == 0 {
		return nil, nil
	}
	if len(sendRows) == 1 {
		return sendRows[0], nil
	}
	return sendRows[0], sendRows[1]
}
