package esper

import (
	"context"
	"sort"
	"testing"
	"time"
)

type rowRecogDataWindowEvent struct {
	Name  string `esper:"name"`
	Group string `esper:"group"`
	Value int    `esper:"value"`
}

func newRowRecogDataWindowTest(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[rowRecogDataWindowEvent](env, "RowRecogDataWindowEvent"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
}

func rowRecogDataWindowTime(milliseconds int64) time.Time {
	return time.Unix(0, milliseconds*int64(time.Millisecond)).UTC()
}

func sendRowRecogDataWindowEvent(t *testing.T, engine *Engine, event rowRecogDataWindowEvent) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
}

func advanceRowRecogDataWindow(t *testing.T, engine *Engine, milliseconds int64) {
	t.Helper()
	if err := engine.AdvanceTime(context.Background(), rowRecogDataWindowTime(milliseconds)); err != nil {
		t.Fatal(err)
	}
}

func rowRecogDataWindowRowNames(t *testing.T, result QueryResult) []string {
	t.Helper()
	names := make([]string, 0, len(result.Results()))
	for _, item := range result.Results() {
		row, ok := item.Row()
		if !ok {
			t.Fatalf("row-recognition result is not a row: %#v", item)
		}
		names = append(names, rowRecogDataWindowField(row, "a")+"/"+rowRecogDataWindowField(row, "b")+"/"+rowRecogDataWindowField(row, "c"))
	}
	return names
}

func rowRecogDataWindowField(row Row, name string) string {
	value := row.Get(name)
	if !value.IsPresent() {
		return ""
	}
	text, _ := value.Any().(string)
	return text
}

func TestRowRecogTimeWindowIteratorAndExpiry(t *testing.T) {
	env, engine := newRowRecogDataWindowTest(t)
	stream := From[rowRecogDataWindowEvent](env, "RowRecogDataWindowEvent").Window(TimeWindow(5 * time.Second))
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"), RowVar("C"))).
		Define("A", Equal[int](Field[rowRecogDataWindowEvent, int]("value"), Literal(1))).
		Define("B", Equal[int](Field[rowRecogDataWindowEvent, int]("value"), Literal(2))).
		Define("C", Equal[int](Field[rowRecogDataWindowEvent, int]("value"), Literal(3))).
		Measures(
			Alias("a", TagField[string]("A", "name")),
			Alias("b", TagField[string]("B", "name")),
			Alias("c", TagField[string]("C", "name")),
		).
		Query(StatementName("rowrecog-time-window-datawin"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	rows := collectRowRecogRows(t, deployment)

	advanceRowRecogDataWindow(t, engine, 50)
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "E1", Value: 1})
	advanceRowRecogDataWindow(t, engine, 1000)
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "E2", Value: 2})
	advanceRowRecogDataWindow(t, engine, 6000)
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "E3", Value: 3})
	if len(*rows) != 0 {
		t.Fatalf("expired prefix unexpectedly matched: %#v", *rows)
	}
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 0 {
		t.Fatalf("time-window snapshot retained expired prefix: %#v", snapshot.Results())
	}

	advanceRowRecogDataWindow(t, engine, 7000)
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "E4", Value: 1})
	advanceRowRecogDataWindow(t, engine, 8000)
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "E5", Value: 2})
	advanceRowRecogDataWindow(t, engine, 11500)
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "E6", Value: 3})
	if len(*rows) != 1 || (*rows)[0].Get("a").Any() != "E4" || (*rows)[0].Get("b").Any() != "E5" || (*rows)[0].Get("c").Any() != "E6" {
		t.Fatalf("time-window listener rows = %#v", *rows)
	}

	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := rowRecogDataWindowRowNames(t, snapshot); len(got) != 1 || got[0] != "E4/E5/E6" {
		t.Fatalf("time-window iterator before expiry = %#v", got)
	}
	advanceRowRecogDataWindow(t, engine, 12000)
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 0 {
		t.Fatalf("time-window iterator retained match after A expiry: %#v", snapshot.Results())
	}
	if len(*rows) != 1 {
		t.Fatalf("time-window expiry dispatched an extra match: %#v", *rows)
	}
}

func TestRowRecogTimeBatchWindowPendingIteratorAndBoundary(t *testing.T) {
	env, engine := newRowRecogDataWindowTest(t)
	name := Field[rowRecogDataWindowEvent, string]("name")
	group := Field[rowRecogDataWindowEvent, string]("group")
	value := Field[rowRecogDataWindowEvent, int]("value")
	stream := From[rowRecogDataWindowEvent](env, "RowRecogDataWindowEvent").Window(TimeBatch(5 * time.Second))
	query := stream.MatchRecognize(RowSequence(RowAlternation(RowVar("A"), RowVar("B")), RowVar("C"))).
		PartitionBy(group).
		Define("A", StartsWith(name, Literal("A"))).
		Define("B", StartsWith(name, Literal("B"))).
		Define("C", And(
			StartsWith(name, Literal("C")),
			In[int](value, TagField[int]("A", "value"), TagField[int]("B", "value")),
		)).
		Measures(
			Alias("a", TagField[string]("A", "name")),
			Alias("b", TagField[string]("B", "name")),
			Alias("c", TagField[string]("C", "name")),
		).
		Query(StatementName("rowrecog-time-batch-datawin"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	rows := collectRowRecogRows(t, deployment)

	advanceRowRecogDataWindow(t, engine, 50)
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "A1", Group: "group001", Value: 1})
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "B1", Group: "group002", Value: 1})
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "B2", Group: "group002", Value: 4})
	if snapshot, snapshotErr := statement.Snapshot(context.Background()); snapshotErr != nil {
		t.Fatal(snapshotErr)
	} else if len(snapshot.Results()) != 0 {
		t.Fatalf("incomplete first batch unexpectedly matched: %#v", snapshot.Results())
	}

	advanceRowRecogDataWindow(t, engine, 4000)
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "C1", Group: "group002", Value: 4})
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "C2", Group: "group002", Value: 5})
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "B3", Group: "group003", Value: -1})
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := rowRecogDataWindowRowNames(t, snapshot); len(got) != 1 || got[0] != "/B2/C1" {
		t.Fatalf("pending time-batch iterator = %#v", got)
	}
	if len(*rows) != 0 {
		t.Fatalf("pending time-batch rows dispatched too early: %#v", *rows)
	}

	advanceRowRecogDataWindow(t, engine, 5050)
	if len(*rows) != 1 || (*rows)[0].Get("a").IsPresent() || (*rows)[0].Get("b").Any() != "B2" || (*rows)[0].Get("c").Any() != "C1" {
		t.Fatalf("first time-batch boundary rows = %#v", *rows)
	}
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 0 {
		t.Fatalf("time-batch iterator retained completed batch: %#v", snapshot.Results())
	}

	advanceRowRecogDataWindow(t, engine, 6000)
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "C3", Group: "group003", Value: -1})
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "C4", Group: "group001", Value: 1})
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 0 {
		t.Fatalf("second time-batch pending rows unexpectedly matched: %#v", snapshot.Results())
	}
	advanceRowRecogDataWindow(t, engine, 10050)
	if len(*rows) != 1 {
		t.Fatalf("second time-batch boundary dispatched stale rows: %#v", *rows)
	}

	advanceRowRecogDataWindow(t, engine, 14000)
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "A2", Group: "group002", Value: 0})
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "B4", Group: "group003", Value: 10})
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "C5", Group: "group002", Value: 0})
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "C6", Group: "group003", Value: 10})
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := rowRecogDataWindowRowNames(t, snapshot)
	sort.Strings(got)
	if len(got) != 2 || got[0] != "/B4/C6" || got[1] != "A2//C5" {
		t.Fatalf("second pending time-batch iterator = %#v", got)
	}
	advanceRowRecogDataWindow(t, engine, 15050)
	if len(*rows) != 3 {
		t.Fatalf("second time-batch boundary rows = %#v", *rows)
	}
	if (*rows)[1].Get("a").Any() != "A2" || (*rows)[1].Get("c").Any() != "C5" || (*rows)[2].Get("b").Any() != "B4" || (*rows)[2].Get("c").Any() != "C6" {
		t.Fatalf("second time-batch boundary order = %#v", *rows)
	}
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 0 {
		t.Fatalf("time-batch iterator retained second completed batch: %#v", snapshot.Results())
	}
}
