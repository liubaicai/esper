package esper

import (
	"context"
	"testing"
)

func newJavaMaxStatesThreeInstancePlan(t *testing.T, env *Environment, name, group string) Plan {
	t.Helper()
	stream := From[rowRecogStatePoolEvent](env, "RowRecogStatePoolEvent").
		Filter(Equal[string](Field[rowRecogStatePoolEvent, string]("group"), Literal(group))).
		Window(KeepAll())
	phase := Field[rowRecogStatePoolEvent, int]("phase")
	id := Field[rowRecogStatePoolEvent, int64]("id")
	query := stream.MatchRecognize(RowSequence(RowVar("P1"), RowVar("P2"), RowVar("P3"))).
		Define("P1", Equal[int](phase, Literal(1))).
		Define("P2", Equal[int](phase, Literal(1))).
		Define("P3", And(
			Equal[int](phase, Literal(2)),
			Equal[int64](id, TagField[int64]("P1", "id")),
		)).
		Measures(Alias("id", TagField[int64]("P1", "id"))).
		Query(StatementName(name))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func newJavaMaxStatesFourInstancePlan(t *testing.T, env *Environment, name, group string, window WindowSpec) Plan {
	t.Helper()
	stream := From[rowRecogStatePoolEvent](env, "RowRecogStatePoolEvent").
		Filter(Equal[string](Field[rowRecogStatePoolEvent, string]("group"), Literal(group))).
		Window(window)
	partition := Field[rowRecogStatePoolEvent, int64]("id")
	phase := Field[rowRecogStatePoolEvent, int]("phase")
	query := stream.MatchRecognize(RowSequence(RowVar("P1"), RowVar("P2"))).
		PartitionBy(partition).
		Define("P1", Equal[int](phase, Literal(1))).
		Define("P2", Equal[int](phase, Literal(2))).
		Measures(Alias("id", TagField[int64]("P1", "id"))).
		Query(StatementName(name))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func expectRowRecogPoolOverflow(t *testing.T, limits *[]MatchRecognizeStateLimitEvent, max int64, counts map[string]int64) {
	t.Helper()
	if len(*limits) != 1 {
		t.Fatalf("state-pool overflow events = %#v, want one", *limits)
	}
	event := (*limits)[0]
	if event.MaxStates != max {
		t.Fatalf("state-pool max = %d, want %d", event.MaxStates, max)
	}
	if len(event.Counts) != len(counts) {
		t.Fatalf("state-pool counts = %#v, want %#v", event.Counts, counts)
	}
	for owner, want := range counts {
		if event.Counts[owner] != want {
			t.Fatalf("state-pool count for %s = %d, want %d (all=%#v)", owner, event.Counts[owner], want, event.Counts)
		}
	}
	*limits = nil
}

func TestRowRecogMaxStatesEngineWideThreeInstanceJavaTrace(t *testing.T) {
	env, engine := newRowRecogStatePoolTest(t, WithMatchRecognizeStateLimit(3, true))
	first, err := engine.Deploy(context.Background(), newJavaMaxStatesThreeInstancePlan(t, env, "rowrecog-java-s1", "A"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.Deploy(context.Background(), newJavaMaxStatesThreeInstancePlan(t, env, "rowrecog-java-s2", "B"))
	if err != nil {
		t.Fatal(err)
	}
	firstRows := collectRowRecogRows(t, first)
	secondRows := collectRowRecogRows(t, second)
	var limits []MatchRecognizeStateLimitEvent
	if err := engine.AddMatchRecognizeStateLimitListener(MatchRecognizeStateLimitListenerFunc(func(event MatchRecognizeStateLimitEvent) {
		limits = append(limits, event)
	})); err != nil {
		t.Fatal(err)
	}

	send := func(group string, phase int, id int64) {
		t.Helper()
		sendRowRecogStatePoolEvent(t, engine, group, phase, id)
	}
	send("A", 1, 10)
	send("B", 1, 11)
	send("A", 1, 12)
	if len(limits) != 0 {
		t.Fatalf("unexpected early state-pool overflow = %#v", limits)
	}
	send("B", 1, 13)
	expectRowRecogPoolOverflow(t, &limits, 3, map[string]int64{
		first.Statements()[0].ID():  2,
		second.Statements()[0].ID(): 1,
	})

	// The admitted B branch reaches P2 and can terminate on B(2,11). The
	// rejected B(1,13) start must not become a recognition branch.
	send("B", 2, 11)
	if len(*secondRows) != 1 || (*secondRows)[0].Get("id").Any() != int64(11) {
		t.Fatalf("B termination rows = %#v", *secondRows)
	}
	send("B", 1, 15)
	if len(limits) != 0 {
		t.Fatalf("released B slot still overflowed = %#v", limits)
	}
	send("B", 1, 16)
	expectRowRecogPoolOverflow(t, &limits, 3, map[string]int64{
		first.Statements()[0].ID():  2,
		second.Statements()[0].ID(): 1,
	})

	send("A", 2, 10)
	if len(*firstRows) != 1 || (*firstRows)[0].Get("id").Any() != int64(10) {
		t.Fatalf("A termination rows = %#v", *firstRows)
	}
	send("B", 1, 17)
	send("B", 1, 18)
	send("A", 1, 19)
	if len(limits) != 0 {
		t.Fatalf("state-pool overflow before final fill = %#v", limits)
	}
	send("A", 1, 20)
	expectRowRecogPoolOverflow(t, &limits, 3, map[string]int64{
		first.Statements()[0].ID():  1,
		second.Statements()[0].ID(): 2,
	})
}

func TestRowRecogMaxStatesEngineWideFourInstanceJavaTrace(t *testing.T) {
	env, engine := newRowRecogStatePoolTest(t, WithMatchRecognizeStateLimit(4, true))
	first, err := engine.Deploy(context.Background(), newJavaMaxStatesFourInstancePlan(t, env, "rowrecog-java-four-s1", "A", KeepAll()))
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.Deploy(context.Background(), newJavaMaxStatesFourInstancePlan(t, env, "rowrecog-java-four-s2", "B", LengthWindow(2)))
	if err != nil {
		t.Fatal(err)
	}
	firstRows := collectRowRecogRows(t, first)
	secondRows := collectRowRecogRows(t, second)
	var limits []MatchRecognizeStateLimitEvent
	if err := engine.AddMatchRecognizeStateLimitListener(MatchRecognizeStateLimitListenerFunc(func(event MatchRecognizeStateLimitEvent) {
		limits = append(limits, event)
	})); err != nil {
		t.Fatal(err)
	}

	send := func(group string, phase int, id int64) {
		t.Helper()
		sendRowRecogStatePoolEvent(t, engine, group, phase, id)
	}
	send("A", 1, 100)
	send("A", 1, 200)
	send("B", 1, 100)
	send("B", 1, 200)
	send("B", 1, 300)
	send("B", 1, 400)
	send("A", 1, 300)
	expectRowRecogPoolOverflow(t, &limits, 4, map[string]int64{
		first.Statements()[0].ID():  2,
		second.Statements()[0].ID(): 2,
	})

	send("B", 2, 400)
	if len(*secondRows) != 1 || (*secondRows)[0].Get("id").Any() != int64(400) {
		t.Fatalf("B length-window termination rows = %#v", *secondRows)
	}
	send("A", 2, 100)
	if len(*firstRows) != 1 || (*firstRows)[0].Get("id").Any() != int64(100) {
		t.Fatalf("A termination rows = %#v", *firstRows)
	}
	send("A", 1, 300)
	send("A", 1, 400)
	send("A", 1, 500)
	if len(limits) != 0 {
		t.Fatalf("unexpected overflow while filling A = %#v", limits)
	}
	send("B", 1, 500)
	expectRowRecogPoolOverflow(t, &limits, 4, map[string]int64{
		first.Statements()[0].ID():  4,
		second.Statements()[0].ID(): 0,
	})

	if err := engine.Undeploy(context.Background(), first.ID()); err != nil {
		t.Fatal(err)
	}
	send("B", 1, 600)
	send("B", 1, 700)
	send("B", 1, 800)
	send("B", 1, 900)
	if len(limits) != 0 {
		t.Fatalf("undeploy did not release A state pool = %#v", limits)
	}
}
