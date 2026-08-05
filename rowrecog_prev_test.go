package esper

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"
)

type rowRecogPreviousEvent struct {
	Name     string `esper:"name"`
	Category string `esper:"category"`
	Value    int    `esper:"value"`
}

func newRowRecogPreviousTest(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[rowRecogPreviousEvent](env, "RowRecogPreviousEvent"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
}

func rowRecogPreviousAt(milliseconds int64) time.Time {
	return time.Unix(0, milliseconds*int64(time.Millisecond)).UTC()
}

func sendRowRecogPrevious(t *testing.T, engine *Engine, event rowRecogPreviousEvent) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
}

func advanceRowRecogPrevious(t *testing.T, engine *Engine, milliseconds int64) {
	t.Helper()
	if err := engine.AdvanceTime(context.Background(), rowRecogPreviousAt(milliseconds)); err != nil {
		t.Fatal(err)
	}
}

func rowRecogPreviousValue(row Row, name string) string {
	value := row.Get(name)
	if !value.IsPresent() {
		return ""
	}
	return fmt.Sprint(value.Any())
}

func rowRecogPreviousNames(t *testing.T, result QueryResult) []string {
	t.Helper()
	names := make([]string, 0, len(result.Results()))
	for _, item := range result.Results() {
		row, ok := item.Row()
		if !ok {
			t.Fatalf("row-recognition result is not a row: %#v", item)
		}
		names = append(names, rowRecogPreviousValue(row, "a"))
	}
	sort.Strings(names)
	return names
}

func assertRowRecogPreviousNames(t *testing.T, result QueryResult, expected ...string) {
	t.Helper()
	got := rowRecogPreviousNames(t, result)
	want := append([]string(nil), expected...)
	sort.Strings(want)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("row-recognition previous names = %#v, want %#v", got, want)
	}
}

func TestRowRecogPreviousHistorySurvivesTimeWindowEviction(t *testing.T) {
	env, engine := newRowRecogPreviousTest(t)
	name := Field[rowRecogPreviousEvent, string]("name")
	value := Field[rowRecogPreviousEvent, int]("value")
	stream := From[rowRecogPreviousEvent](env, "RowRecogPreviousEvent").Window(TimeWindow(5 * time.Second))
	a := And(
		And(
			Equal[string](Prev[string](3, name), Literal("P3")),
			Equal[string](Prev[string](2, name), Literal("P2")),
		),
		And(
			Equal[string](Prev[string](4, name), Literal("P4")),
			GreaterOrEqual[int](Abs[int](Prev[int](0, value)), Literal(0)),
		),
	)
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		Define("A", a).
		Define("B", In[int](value, Prev[int](4, value), Prev[int](2, value))).
		Measures(
			Alias("a", TagField[string]("A", "name")),
			Alias("b", TagField[string]("B", "name")),
		).
		Query(StatementName("rowrecog-prev-time-window"))
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

	advanceRowRecogPrevious(t, engine, 1000)
	for _, event := range []rowRecogPreviousEvent{
		{Name: "P2", Value: 1},
		{Name: "P1", Value: 2},
		{Name: "P3", Value: 3},
		{Name: "P4", Value: 4},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	advanceRowRecogPrevious(t, engine, 2000)
	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "P2", Value: 1})
	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E1", Value: 3})
	advanceRowRecogPrevious(t, engine, 3000)
	for _, event := range []rowRecogPreviousEvent{
		{Name: "P4", Value: 11},
		{Name: "P3", Value: 12},
		{Name: "P2", Value: 13},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	advanceRowRecogPrevious(t, engine, 4000)
	for _, event := range []rowRecogPreviousEvent{
		{Name: "xx", Value: 4},
		{Name: "E2", Value: -1},
		{Name: "E3", Value: 12},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	if len(*rows) != 1 || (*rows)[0].Get("a").Any() != "E2" || (*rows)[0].Get("b").Any() != "E3" {
		t.Fatalf("first PREV match = %#v", *rows)
	}
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot, "E2")

	advanceRowRecogPrevious(t, engine, 5000)
	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "P4", Value: 21})
	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "P3", Value: 22})
	advanceRowRecogPrevious(t, engine, 6000)
	for _, event := range []rowRecogPreviousEvent{
		{Name: "P2", Value: 23},
		{Name: "xx", Value: -2},
		{Name: "E5", Value: -1},
		{Name: "E6", Value: -2},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	if len(*rows) != 2 || (*rows)[1].Get("a").Any() != "E5" || (*rows)[1].Get("b").Any() != "E6" {
		t.Fatalf("second PREV match = %#v", *rows)
	}
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot, "E2", "E5")
	advanceRowRecogPrevious(t, engine, 9500)
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot, "E5")
	advanceRowRecogPrevious(t, engine, 11500)
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot)
}

func TestRowRecogPreviousHistoryIsPartitionLocal(t *testing.T) {
	env, engine := newRowRecogPreviousTest(t)
	category := Field[rowRecogPreviousEvent, string]("category")
	value := Field[rowRecogPreviousEvent, int]("value")
	stream := From[rowRecogPreviousEvent](env, "RowRecogPreviousEvent").Window(TimeWindow(5 * time.Second))
	query := stream.MatchRecognize(RowSequence(RowVar("A"))).
		PartitionBy(category).
		Define("A", Equal[int](Prev[int](1, value), Subtract[int](value, Literal(1)))).
		Measures(
			Alias("a", TagField[string]("A", "name")),
			Alias("category", TagField[string]("A", "category")),
		).
		Query(StatementName("rowrecog-prev-partitioned"))
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

	advanceRowRecogPrevious(t, engine, 1000)
	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E1", Category: "S1", Value: 100})
	advanceRowRecogPrevious(t, engine, 2000)
	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E2", Category: "S3", Value: 100})
	advanceRowRecogPrevious(t, engine, 2500)
	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E3", Category: "S2", Value: 102})
	advanceRowRecogPrevious(t, engine, 6200)
	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E4", Category: "S1", Value: 101})
	advanceRowRecogPrevious(t, engine, 6500)
	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E5", Category: "S3", Value: 101})
	advanceRowRecogPrevious(t, engine, 7000)
	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E6", Category: "S1", Value: 102})
	advanceRowRecogPrevious(t, engine, 10000)
	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E7", Category: "S2", Value: 103})
	for _, event := range []rowRecogPreviousEvent{
		{Name: "E8", Category: "S2", Value: 102},
		{Name: "E8", Category: "S1", Value: 101},
		{Name: "E8", Category: "S2", Value: 104},
		{Name: "E8", Category: "S1", Value: 105},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	if len(*rows) != 4 {
		t.Fatalf("partitioned PREV listener rows = %#v", *rows)
	}
	got := []string{fmt.Sprint((*rows)[0].Get("a").Any()), fmt.Sprint((*rows)[1].Get("a").Any()), fmt.Sprint((*rows)[2].Get("a").Any()), fmt.Sprint((*rows)[3].Get("a").Any())}
	sort.Strings(got)
	if fmt.Sprint(got) != fmt.Sprint([]string{"E4", "E5", "E6", "E7"}) {
		t.Fatalf("partitioned PREV listener names = %#v", got)
	}
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot, "E4", "E5", "E6", "E7")
	advanceRowRecogPrevious(t, engine, 11200)
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot, "E5", "E6", "E7")
	advanceRowRecogPrevious(t, engine, 11600)
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot, "E6", "E7")
	advanceRowRecogPrevious(t, engine, 16000)
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot)
}

func TestRowRecogPreviousHistoryForPartitionedSequence(t *testing.T) {
	env, engine := newRowRecogPreviousTest(t)
	name := Field[rowRecogPreviousEvent, string]("name")
	category := Field[rowRecogPreviousEvent, string]("category")
	value := Field[rowRecogPreviousEvent, int]("value")
	stream := From[rowRecogPreviousEvent](env, "RowRecogPreviousEvent").Window(TimeWindow(5 * time.Second))
	defineA := And(
		Equal[string](Prev[string](3, name), Literal("P3")),
		And(
			Equal[string](Prev[string](2, name), Literal("P2")),
			Equal[string](Prev[string](4, name), Literal("P4")),
		),
	)
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		PartitionBy(category).
		Define("A", defineA).
		Define("B", In[int](value, Prev[int](4, value), Prev[int](2, value))).
		Measures(
			Alias("a", TagField[string]("A", "name")),
			Alias("b", TagField[string]("B", "name")),
			Alias("category", TagField[string]("A", "category")),
		).
		Query(StatementName("rowrecog-prev-partitioned-sequence"))
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

	advanceRowRecogPrevious(t, engine, 1000)
	for _, event := range []rowRecogPreviousEvent{
		{Name: "P4", Category: "c2", Value: 1},
		{Name: "P3", Category: "c1", Value: 2},
		{Name: "P2", Category: "c2", Value: 3},
		{Name: "xx", Category: "c1", Value: 4},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	advanceRowRecogPrevious(t, engine, 2000)
	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "P2", Category: "c1", Value: 1})
	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E1", Category: "c1", Value: 3})
	advanceRowRecogPrevious(t, engine, 3000)
	for _, event := range []rowRecogPreviousEvent{
		{Name: "P4", Category: "c1", Value: 11},
		{Name: "P3", Category: "c1", Value: 12},
		{Name: "P2", Category: "c1", Value: 13},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	advanceRowRecogPrevious(t, engine, 4000)
	for _, event := range []rowRecogPreviousEvent{
		{Name: "xx", Category: "c1", Value: 4},
		{Name: "E2", Category: "c1", Value: -1},
		{Name: "E3", Category: "c1", Value: 12},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	advanceRowRecogPrevious(t, engine, 5000)
	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "P4", Category: "c2", Value: 21})
	sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "P3", Category: "c2", Value: 22})
	advanceRowRecogPrevious(t, engine, 6000)
	for _, event := range []rowRecogPreviousEvent{
		{Name: "P2", Category: "c2", Value: 23},
		{Name: "xx", Category: "c2", Value: -2},
		{Name: "E5", Category: "c2", Value: -1},
		{Name: "E6", Category: "c2", Value: -2},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	if len(*rows) != 2 {
		t.Fatalf("partitioned sequence PREV rows = %#v", *rows)
	}
	got := []string{fmt.Sprint((*rows)[0].Get("a").Any()), fmt.Sprint((*rows)[1].Get("a").Any())}
	sort.Strings(got)
	if fmt.Sprint(got) != fmt.Sprint([]string{"E2", "E5"}) {
		t.Fatalf("partitioned sequence PREV names = %#v", got)
	}
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot, "E2", "E5")
	advanceRowRecogPrevious(t, engine, 9500)
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot, "E5")
	advanceRowRecogPrevious(t, engine, 11500)
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRowRecogPreviousNames(t, snapshot)
}

func TestRowRecogPreviousHistorySupportsMultiFieldPartitions(t *testing.T) {
	env, engine := newRowRecogPreviousTest(t)
	name := Field[rowRecogPreviousEvent, string]("name")
	category := Field[rowRecogPreviousEvent, string]("category")
	value := Field[rowRecogPreviousEvent, int]("value")
	stream := From[rowRecogPreviousEvent](env, "RowRecogPreviousEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		PartitionBy(name, category).
		Define("A", Greater[int](value, Prev[int](1, value))).
		Define("B", Greater[int](value, Prev[int](1, value))).
		Measures(
			Alias("a", TagField[string]("A", "name")),
			Alias("category", TagField[string]("A", "category")),
			Alias("aValue", TagField[int]("A", "value")),
			Alias("bValue", TagField[int]("B", "value")),
		).
		Query(StatementName("rowrecog-prev-multikey"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogPreviousEvent{
		{Name: "S1", Category: "T1", Value: 5},
		{Name: "S2", Category: "T1", Value: 110},
		{Name: "S1", Category: "T2", Value: 21},
		{Name: "S1", Category: "T1", Value: 7},
		{Name: "S2", Category: "T1", Value: 111},
		{Name: "S1", Category: "T2", Value: 20},
		{Name: "S2", Category: "T1", Value: 110},
		{Name: "S2", Category: "T2", Value: 1000},
		{Name: "S2", Category: "T2", Value: 1001},
		{Name: "S1", Value: 9},
		{Name: "S1", Category: "T1", Value: 9},
		{Name: "S2", Category: "T2", Value: 1001},
		{Name: "S2", Category: "T1", Value: 109},
		{Name: "S1", Category: "T2", Value: 25},
		{Name: "S2", Category: "T2", Value: 1002},
		{Name: "S2", Category: "T2", Value: 1003},
		{Name: "S1", Category: "T2", Value: 28},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	if len(*rows) != 3 {
		t.Fatalf("multi-key PREV rows = %#v", *rows)
	}
	want := map[string]struct{}{
		"S1/T1/7/9":       {},
		"S1/T2/25/28":     {},
		"S2/T2/1002/1003": {},
	}
	got := make(map[string]struct{}, len(*rows))
	for _, row := range *rows {
		got[fmt.Sprintf("%s/%s/%v/%v", rowRecogPreviousValue(row, "a"), rowRecogPreviousValue(row, "category"), row.Get("aValue").Any(), row.Get("bValue").Any())] = struct{}{}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("multi-key PREV result set = %#v, want %#v", got, want)
	}
}

func TestRowRecogPreviousHistoryOnUnpartitionedKeepAll(t *testing.T) {
	env, engine := newRowRecogPreviousTest(t)
	value := Field[rowRecogPreviousEvent, int]("value")
	stream := From[rowRecogPreviousEvent](env, "RowRecogPreviousEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(RowVar("A"))).
		Define("A", Greater[int](value, Prev[int](1, value))).
		Measures(Alias("a", TagField[string]("A", "name"))).
		Query(StatementName("rowrecog-prev-keepall"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogPreviousEvent{
		{Name: "E1", Value: 5}, {Name: "E2", Value: 3}, {Name: "E3", Value: 6},
		{Name: "E4", Value: 4}, {Name: "E5", Value: 6}, {Name: "E6", Value: 10},
		{Name: "E7", Value: 9}, {Name: "E8", Value: 4},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	if got := rowRecogPreviousNames(t, QueryResult{Batch: ResultBatch{New: rowsToResults(*rows)}}); fmt.Sprint(got) != fmt.Sprint([]string{"E3", "E5", "E6"}) {
		t.Fatalf("keepall PREV listener names = %#v", got)
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}

	query = stream.MatchRecognize(RowSequence(RowVar("A"))).
		Define("A", Equal[int](Prev[int](2, value), Literal(5))).
		Measures(Alias("a", TagField[string]("A", "name"))).
		Query(StatementName("rowrecog-prev-keepall-offset"))
	plan, err = env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err = engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows = collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogPreviousEvent{
		{Name: "E1", Value: 5}, {Name: "E2", Value: 4}, {Name: "E3", Value: 6},
		{Name: "E4", Value: 3}, {Name: "E5a", Value: 3}, {Name: "E5b", Value: 5},
		{Name: "E6", Value: 5}, {Name: "E7", Value: 6}, {Name: "E8", Value: 6},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	if len(*rows) != 3 || (*rows)[0].Get("a").Any() != "E3" || (*rows)[1].Get("a").Any() != "E7" || (*rows)[2].Get("a").Any() != "E8" {
		t.Fatalf("keepall PREV offset rows = %#v", *rows)
	}
}

func rowsToResults(rows []Row) []Result {
	result := make([]Result, 0, len(rows))
	for _, row := range rows {
		result = append(result, resultRow(row))
	}
	return result
}
