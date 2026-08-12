package esper

import (
	"context"
	"fmt"
	"sort"
	"testing"
)

func TestRowRecogDataSetFinancialPattern(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	price := Field[rowRecogTestEvent, float64]("price")
	previous := func() Expression[float64] { return Prev[float64](1, price) }
	query := stream.MatchRecognize(RowSequence(
		RowVar("A"),
		RowVar("B"),
		RowVar("C").ZeroOrMore(),
		RowVar("D"),
		RowVar("E").ZeroOrMore(),
		RowVar("F").OneOrMore(),
	)).
		Define("B", Less[float64](price, previous())).
		Define("C", LessOrEqual[float64](price, previous())).
		Define("D", Less[float64](price, previous())).
		Define("E", GreaterOrEqual[float64](price, previous())).
		Define("F", And(
			GreaterOrEqual[float64](price, previous()),
			Greater[float64](price, TagField[float64]("A", "price")),
		)).
		AllMatches().
		SkipToCurrentRow().
		Measures(
			Alias("a", TagField[string]("A", "symbol")),
			Alias("b", TagField[string]("B", "symbol")),
			Alias("c0", TagFieldAt[string]("C", 0, "symbol")),
			Alias("c1", TagFieldAt[string]("C", 1, "symbol")),
			Alias("d", TagField[string]("D", "symbol")),
			Alias("e0", TagFieldAt[string]("E", 0, "symbol")),
			Alias("e1", TagFieldAt[string]("E", 1, "symbol")),
			Alias("f0", TagFieldAt[string]("F", 0, "symbol")),
			Alias("f1", TagFieldAt[string]("F", 1, "symbol")),
		).
		Query(StatementName("rowrecog-dataset-financial"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	events := []rowRecogTestEvent{
		{Symbol: "E1", Price: 100},
		{Symbol: "E2", Price: 98},
		{Symbol: "E3", Price: 75},
		{Symbol: "E4", Price: 61},
		{Symbol: "E5", Price: 50},
		{Symbol: "E6", Price: 49},
		{Symbol: "E7", Price: 64},
		{Symbol: "E8", Price: 78},
		{Symbol: "E9", Price: 84},
	}
	expectedBatches := [][]string{
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		{"E4/E5///E6///E7/"},
		{"E3/E4/E5//E6/E7//E8/", "E4/E5///E6/E7//E8/", "E4/E5///E6///E7/E8"},
		{"E3/E4/E5//E6/E7//E8/E9", "E3/E4/E5//E6/E7/E8/E9/", "E4/E5///E6/E7/E8/E9/", "E4/E5///E6/E7//E8/E9", "E4/E5///E6///E7/E8"},
	}
	total := 0
	for index, event := range events {
		before := len(*rows)
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		batch := rowRecogDataSetKeys(t, (*rows)[before:])
		if got, want := batch, sortedStrings(expectedBatches[index]); fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("financial pattern event %s listener batch = %#v, want %#v", event.Symbol, got, want)
		}
		total += len(expectedBatches[index])
		if len(*rows) != total {
			t.Fatalf("financial pattern event %s cumulative listener rows = %d, want %d", event.Symbol, len(*rows), total)
		}
	}

	expected := []string{
		"E4/E5///E6///E7/",
		"E3/E4/E5//E6/E7//E8/",
		"E4/E5///E6/E7//E8/",
		"E4/E5///E6///E7/E8",
		"E3/E4/E5//E6/E7//E8/E9",
		"E3/E4/E5//E6/E7/E8/E9/",
		"E4/E5///E6/E7/E8/E9/",
		"E4/E5///E6/E7//E8/E9",
		"E4/E5///E6///E7/E8",
	}
	got := rowRecogDataSetKeys(t, *rows)
	if fmt.Sprint(got) != fmt.Sprint(sortedStrings(expected)) {
		t.Fatalf("financial pattern listener rows = %#v, want %#v", got, sortedStrings(expected))
	}
}

func TestRowRecogAfterCurrentRowKeepsTheRecognizingBranch(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B").ZeroOrMore())).
		Define("A", StartsWith(Field[rowRecogTestEvent, string]("symbol"), Literal("A"))).
		Define("B", StartsWith(Field[rowRecogTestEvent, string]("symbol"), Literal("B"))).
		FirstMatch().
		SkipToCurrentRow().
		Measures(
			Alias("a", TagField[string]("A", "symbol")),
			Alias("b0", TagFieldAt[string]("B", 0, "symbol")),
			Alias("b1", TagFieldAt[string]("B", 1, "symbol")),
		).
		Query(StatementName("rowrecog-after-current-row"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	statement := deployment.Statements()[0]
	if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "A1"}); err != nil {
		t.Fatal(err)
	}
	if got := rowRecogAfterCurrentRowKeys(*rows); fmt.Sprint(got) != fmt.Sprint([]string{"A1//"}) {
		t.Fatalf("current-row initial listener rows = %#v", got)
	}
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := rowRecogAfterCurrentRowResultKeys(t, snapshot.Results()); fmt.Sprint(got) != fmt.Sprint([]string{"A1//"}) {
		t.Fatalf("current-row initial snapshot = %#v", got)
	}

	if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "B1"}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 2 {
		t.Fatalf("current-row continuation listener row count = %d, all=%#v", len(*rows), *rows)
	}
	if got := rowRecogAfterCurrentRowKeys((*rows)[1:]); fmt.Sprint(got) != fmt.Sprint([]string{"A1/B1/"}) {
		t.Fatalf("current-row continuation listener rows = %#v, all=%#v", got, *rows)
	}
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := rowRecogAfterCurrentRowResultKeys(t, snapshot.Results()); fmt.Sprint(got) != fmt.Sprint([]string{"A1/B1/"}) {
		t.Fatalf("current-row continuation snapshot = %#v", got)
	}
}

func rowRecogDataSetKeys(t *testing.T, rows []Row) []string {
	t.Helper()
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		field := func(name string) string {
			value := row.Get(name)
			if !value.IsPresent() {
				return ""
			}
			return fmt.Sprint(value.Any())
		}
		keys = append(keys, fmt.Sprintf("%s/%s/%s/%s/%s/%s/%s/%s/%s",
			field("a"), field("b"), field("c0"), field("c1"), field("d"), field("e0"), field("e1"), field("f0"), field("f1")))
	}
	return sortedStrings(keys)
}

func rowRecogAfterCurrentRowKeys(rows []Row) []string {
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		field := func(name string) string {
			value := row.Get(name)
			if !value.IsPresent() {
				return ""
			}
			return fmt.Sprint(value.Any())
		}
		keys = append(keys, fmt.Sprintf("%s/%s/%s", field("a"), field("b0"), field("b1")))
	}
	return keys
}

func rowRecogAfterCurrentRowResultKeys(t *testing.T, results []Result) []string {
	t.Helper()
	rows := make([]Row, 0, len(results))
	for _, result := range results {
		row, ok := result.Row()
		if !ok {
			t.Fatalf("row-recognize snapshot result is not a row: %#v", result)
		}
		rows = append(rows, row)
	}
	return rowRecogAfterCurrentRowKeys(rows)
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}
