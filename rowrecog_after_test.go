package esper

import (
	"context"
	"fmt"
	"testing"
)

func TestRowRecogAfterNextRowContinuation(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B").ZeroOrMore())).
		Define("A", StartsWith(Field[rowRecogTestEvent, string]("symbol"), Literal("A"))).
		Define("B", StartsWith(Field[rowRecogTestEvent, string]("symbol"), Literal("B"))).
		FirstMatch().
		SkipToNextRow().
		Measures(
			Alias("a", TagField[string]("A", "symbol")),
			Alias("b0", TagFieldAt[string]("B", 0, "symbol")),
			Alias("b1", TagFieldAt[string]("B", 1, "symbol")),
		).
		Query(StatementName("rowrecog-after-next-row-continuation"))
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
	send := func(event rowRecogTestEvent, listener, snapshot []string) {
		t.Helper()
		before := len(*rows)
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		assertRowRecogAfterState(t, rows, statement, before, listener, snapshot, "a", "b0", "b1")
	}
	send(rowRecogTestEvent{Symbol: "A1"}, []string{"A1//"}, []string{"A1//"})
	send(rowRecogTestEvent{Symbol: "B1"}, nil, []string{"A1/B1/"})
}

func TestRowRecogAfterSkipToNextRowDataSet(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	price := Field[rowRecogTestEvent, float64]("price")
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		Define("B", Greater[float64](price, TagField[float64]("A", "price"))).
		AllMatches().
		SkipToNextRow().
		Measures(
			Alias("a", TagField[string]("A", "symbol")),
			Alias("b", TagField[string]("B", "symbol")),
		).
		Query(
			StatementName("rowrecog-after-skip-next-row"),
			OrderBy(Ascending(ResultField[string]("a")), Ascending(ResultField[string]("b"))),
		)
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
	send := func(event rowRecogTestEvent, listener, snapshot []string) {
		t.Helper()
		before := len(*rows)
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		assertRowRecogAfterState(t, rows, statement, before, listener, snapshot, "a", "b")
	}
	send(rowRecogTestEvent{Symbol: "E1", Price: 5}, nil, nil)
	send(rowRecogTestEvent{Symbol: "E2", Price: 3}, nil, nil)
	send(rowRecogTestEvent{Symbol: "E3", Price: 6}, []string{"E2/E3"}, []string{"E2/E3"})
	send(rowRecogTestEvent{Symbol: "E4", Price: 4}, nil, []string{"E2/E3"})
	send(rowRecogTestEvent{Symbol: "E5", Price: 6}, []string{"E4/E5"}, []string{"E2/E3", "E4/E5"})
	send(rowRecogTestEvent{Symbol: "E6", Price: 10}, []string{"E5/E6"}, []string{"E2/E3", "E4/E5", "E5/E6"})
	send(rowRecogTestEvent{Symbol: "E7", Price: 9}, nil, []string{"E2/E3", "E4/E5", "E5/E6"})
	send(rowRecogTestEvent{Symbol: "E8", Price: 4}, nil, []string{"E2/E3", "E4/E5", "E5/E6"})
}

func TestRowRecogAfterSkipToNextRowRepeatedVariable(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	price := Field[rowRecogTestEvent, float64]("price")
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"), RowVar("A"))).
		Define("A", Equal[float64](price, Literal(1.0))).
		Define("B", Equal[float64](price, Literal(2.0))).
		AllMatches().
		SkipToNextRow().
		Measures(
			Alias("a0", TagFieldAt[string]("A", 0, "symbol")),
			Alias("b", TagField[string]("B", "symbol")),
			Alias("a1", TagFieldAt[string]("A", 1, "symbol")),
		).
		Query(
			StatementName("rowrecog-after-skip-next-repeated"),
			OrderBy(Ascending(ResultField[string]("a0")), Ascending(ResultField[string]("b")), Ascending(ResultField[string]("a1"))),
		)
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
	send := func(event rowRecogTestEvent, listener, snapshot []string) {
		t.Helper()
		before := len(*rows)
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		assertRowRecogAfterState(t, rows, statement, before, listener, snapshot, "a0", "b", "a1")
	}
	send(rowRecogTestEvent{Symbol: "E1", Price: 3}, nil, nil)
	send(rowRecogTestEvent{Symbol: "E2", Price: 1}, nil, nil)
	send(rowRecogTestEvent{Symbol: "E3", Price: 2}, nil, nil)
	send(rowRecogTestEvent{Symbol: "E4", Price: 5}, nil, nil)
	send(rowRecogTestEvent{Symbol: "E5", Price: 1}, nil, nil)
	send(rowRecogTestEvent{Symbol: "E6", Price: 2}, nil, nil)
	send(rowRecogTestEvent{Symbol: "E7", Price: 1}, []string{"E5/E6/E7"}, []string{"E5/E6/E7"})
	send(rowRecogTestEvent{Symbol: "E8", Price: 2}, nil, []string{"E5/E6/E7"})
	send(rowRecogTestEvent{Symbol: "E9", Price: 1}, []string{"E7/E8/E9"}, []string{"E5/E6/E7", "E7/E8/E9"})
}

func TestRowRecogAfterSkipToNextRowPartitioned(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	symbol := Field[rowRecogTestEvent, string]("symbol")
	price := Field[rowRecogTestEvent, float64]("price")
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		PartitionBy(symbol).
		Define("B", Greater[float64](price, TagField[float64]("A", "price"))).
		AllMatches().
		SkipToNextRow().
		Measures(
			Alias("a_string", TagField[string]("A", "symbol")),
			Alias("a_value", TagField[float64]("A", "price")),
			Alias("b_value", TagField[float64]("B", "price")),
		).
		Query(
			StatementName("rowrecog-after-skip-next-partitioned"),
			OrderBy(Ascending(ResultField[string]("a_string"))),
		)
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
	send := func(event rowRecogTestEvent, listener, snapshot []string) {
		t.Helper()
		before := len(*rows)
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		assertRowRecogAfterState(t, rows, statement, before, listener, snapshot, "a_string", "a_value", "b_value")
	}
	send(rowRecogTestEvent{Symbol: "S1", Price: 5}, nil, nil)
	send(rowRecogTestEvent{Symbol: "S2", Price: 6}, nil, nil)
	send(rowRecogTestEvent{Symbol: "S3", Price: 3}, nil, nil)
	send(rowRecogTestEvent{Symbol: "S4", Price: 4}, nil, nil)
	send(rowRecogTestEvent{Symbol: "S1", Price: 5}, nil, nil)
	send(rowRecogTestEvent{Symbol: "S2", Price: 5}, nil, nil)
	send(rowRecogTestEvent{Symbol: "S1", Price: 4}, nil, nil)
	send(rowRecogTestEvent{Symbol: "S4", Price: -1}, nil, nil)
	send(rowRecogTestEvent{Symbol: "S1", Price: 6}, []string{"S1/4/6"}, []string{"S1/4/6"})
	send(rowRecogTestEvent{Symbol: "S4", Price: 10}, []string{"S4/-1/10"}, []string{"S1/4/6", "S4/-1/10"})
	send(rowRecogTestEvent{Symbol: "S4", Price: 11}, []string{"S4/10/11"}, []string{"S1/4/6", "S4/-1/10", "S4/10/11"})
	send(rowRecogTestEvent{Symbol: "S3", Price: 3}, nil, []string{"S1/4/6", "S4/-1/10", "S4/10/11"})
	send(rowRecogTestEvent{Symbol: "S4", Price: -1}, nil, []string{"S1/4/6", "S4/-1/10", "S4/10/11"})
	send(rowRecogTestEvent{Symbol: "S3", Price: 2}, nil, []string{"S1/4/6", "S4/-1/10", "S4/10/11"})
	send(rowRecogTestEvent{Symbol: "S1", Price: 4}, nil, []string{"S1/4/6", "S4/-1/10", "S4/10/11"})
	send(rowRecogTestEvent{Symbol: "S1", Price: 7}, []string{"S1/4/7"}, []string{"S1/4/6", "S1/4/7", "S4/-1/10", "S4/10/11"})
	send(rowRecogTestEvent{Symbol: "S4", Price: 12}, []string{"S4/-1/12"}, []string{"S1/4/6", "S1/4/7", "S4/-1/10", "S4/10/11", "S4/-1/12"})
	send(rowRecogTestEvent{Symbol: "S4", Price: 12}, nil, []string{"S1/4/6", "S1/4/7", "S4/-1/10", "S4/10/11", "S4/-1/12"})
	send(rowRecogTestEvent{Symbol: "S1", Price: 7}, nil, []string{"S1/4/6", "S1/4/7", "S4/-1/10", "S4/10/11", "S4/-1/12"})
	send(rowRecogTestEvent{Symbol: "S2", Price: 4}, nil, []string{"S1/4/6", "S1/4/7", "S4/-1/10", "S4/10/11", "S4/-1/12"})
	send(rowRecogTestEvent{Symbol: "S1", Price: 5}, nil, []string{"S1/4/6", "S1/4/7", "S4/-1/10", "S4/10/11", "S4/-1/12"})
	send(rowRecogTestEvent{Symbol: "S2", Price: 5}, []string{"S2/4/5"}, []string{"S1/4/6", "S1/4/7", "S2/4/5", "S4/-1/10", "S4/10/11", "S4/-1/12"})
}

func TestRowRecogAfterSkipPastLastRow(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	price := Field[rowRecogTestEvent, float64]("price")
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		Define("B", Greater[float64](price, TagField[float64]("A", "price"))).
		AllMatches().
		SkipPastLastRow().
		Measures(
			Alias("a", TagField[string]("A", "symbol")),
			Alias("b", TagField[string]("B", "symbol")),
		).
		Query(
			StatementName("rowrecog-after-skip-past-last"),
			OrderBy(Ascending(ResultField[string]("a")), Ascending(ResultField[string]("b"))),
		)
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
	send := func(event rowRecogTestEvent, listener, snapshot []string) {
		t.Helper()
		before := len(*rows)
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		assertRowRecogAfterState(t, rows, statement, before, listener, snapshot, "a", "b")
	}
	send(rowRecogTestEvent{Symbol: "E1", Price: 5}, nil, nil)
	send(rowRecogTestEvent{Symbol: "E2", Price: 3}, nil, nil)
	send(rowRecogTestEvent{Symbol: "E3", Price: 6}, []string{"E2/E3"}, []string{"E2/E3"})
	send(rowRecogTestEvent{Symbol: "E4", Price: 4}, nil, []string{"E2/E3"})
	send(rowRecogTestEvent{Symbol: "E5", Price: 6}, []string{"E4/E5"}, []string{"E2/E3", "E4/E5"})
	send(rowRecogTestEvent{Symbol: "E6", Price: 10}, nil, []string{"E2/E3", "E4/E5"})
	send(rowRecogTestEvent{Symbol: "E7", Price: 9}, nil, []string{"E2/E3", "E4/E5"})
	send(rowRecogTestEvent{Symbol: "E8", Price: 4}, nil, []string{"E2/E3", "E4/E5"})
}

func assertRowRecogAfterState(t *testing.T, rows *[]Row, statement *Statement, before int, listener, snapshot []string, fields ...string) {
	t.Helper()
	gotListener := rowRecogAfterKeys((*rows)[before:], fields...)
	wantListener := sortedStrings(listener)
	if fmt.Sprint(gotListener) != fmt.Sprint(wantListener) {
		t.Fatalf("row-recognize listener batch = %#v, want %#v", gotListener, wantListener)
	}
	result, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	gotSnapshot := rowRecogAfterResultKeys(t, result.Results(), fields...)
	wantSnapshot := sortedStrings(snapshot)
	if fmt.Sprint(gotSnapshot) != fmt.Sprint(wantSnapshot) {
		t.Fatalf("row-recognize snapshot = %#v, want %#v", gotSnapshot, wantSnapshot)
	}
}

func rowRecogAfterKeys(rows []Row, fields ...string) []string {
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		values := make([]string, 0, len(fields))
		for _, field := range fields {
			value := row.Get(field)
			if value.IsPresent() {
				values = append(values, fmt.Sprint(value.Any()))
			} else {
				values = append(values, "")
			}
		}
		keys = append(keys, joinRowRecogAfterValues(values))
	}
	return sortedStrings(keys)
}

func rowRecogAfterResultKeys(t *testing.T, results []Result, fields ...string) []string {
	t.Helper()
	rows := make([]Row, 0, len(results))
	for _, result := range results {
		row, ok := result.Row()
		if !ok {
			t.Fatalf("row-recognize snapshot result is not a row: %#v", result)
		}
		rows = append(rows, row)
	}
	return rowRecogAfterKeys(rows, fields...)
}

func joinRowRecogAfterValues(values []string) string {
	if len(values) == 0 {
		return ""
	}
	result := values[0]
	for _, value := range values[1:] {
		result += "/" + value
	}
	return result
}
