package esper

import (
	"context"
	"testing"
)

func TestRowRecogMeasureAggregationMatrix(t *testing.T) {
	env, engine := newRowRecogPreviousTest(t)
	stream := From[rowRecogPreviousEvent](env, "RowRecogPreviousEvent").Window(KeepAll())
	value := Field[rowRecogPreviousEvent, int]("value")
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B").ZeroOrMore(), RowVar("C"))).
		Define("A", Equal[int](value, Literal(0))).
		Define("B", NotEqual[int](value, Literal(1))).
		Define("C", Equal[int](value, Literal(1))).
		AllMatches().
		Measures(
			Alias("a_string", TagField[string]("A", "name")),
			Alias("c_string", TagField[string]("C", "name")),
			Alias("maxb", TagMax[int]("B", value)),
			Alias("minb", TagMin[int]("B", value)),
			Alias("minb2x", Multiply[int](Literal(2), TagMin[int]("B", value))),
			Alias("lastb", TagLast[int]("B", value)),
			Alias("firstb", TagFirst[int]("B", value)),
			Alias("countb", TagCount("B")),
		).
		Query(
			StatementName("rowrecog-measure-aggregation"),
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
	for _, event := range []rowRecogPreviousEvent{
		{Name: "E1", Value: 0},
		{Name: "E2", Value: 1},
		{Name: "E3", Value: 0},
		{Name: "E4", Value: 5},
		{Name: "E5", Value: 3},
		{Name: "E6", Value: 1},
		{Name: "E7", Value: 0},
		{Name: "E8", Value: 4},
		{Name: "E9", Value: -1},
		{Name: "E10", Value: 7},
		{Name: "E11", Value: 2},
		{Name: "E12", Value: 1},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	if len(*rows) != 3 {
		t.Fatalf("measure aggregation rows = %#v", *rows)
	}
	if first := (*rows)[0]; first.Get("a_string").Any() != "E1" || first.Get("c_string").Any() != "E2" ||
		first.Get("maxb").IsPresent() || first.Get("minb").IsPresent() || first.Get("minb2x").IsPresent() ||
		first.Get("lastb").IsPresent() || first.Get("firstb").IsPresent() || first.Get("countb").Any() != int64(0) {
		t.Fatalf("measure aggregation empty B row = %#v", first)
	}
	if second := (*rows)[1]; second.Get("a_string").Any() != "E3" || second.Get("c_string").Any() != "E6" ||
		second.Get("maxb").Any() != 5 || second.Get("minb").Any() != 3 || second.Get("minb2x").Any() != 6 ||
		second.Get("lastb").Any() != 3 || second.Get("firstb").Any() != 5 || second.Get("countb").Any() != int64(2) {
		t.Fatalf("measure aggregation second row = %#v", second)
	}
	if third := (*rows)[2]; third.Get("a_string").Any() != "E7" || third.Get("c_string").Any() != "E12" ||
		third.Get("maxb").Any() != 7 || third.Get("minb").Any() != -1 || third.Get("minb2x").Any() != -2 ||
		third.Get("lastb").Any() != 2 || third.Get("firstb").Any() != 4 || third.Get("countb").Any() != int64(4) {
		t.Fatalf("measure aggregation third row = %#v", third)
	}
	statement := deployment.Statements()[0]
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 3 {
		t.Fatalf("measure aggregation snapshot = %#v", snapshot.Results())
	}
	for index, expected := range []struct {
		a, c                  string
		maxb, minb, minb2x    any
		lastb, firstb, countb any
	}{
		{"E1", "E2", nil, nil, nil, nil, nil, int64(0)},
		{"E3", "E6", 5, 3, 6, 3, 5, int64(2)},
		{"E7", "E12", 7, -1, -2, 2, 4, int64(4)},
	} {
		row, ok := snapshot.Results()[index].Row()
		if !ok || row.Get("a_string").Any() != expected.a || row.Get("c_string").Any() != expected.c ||
			row.Get("maxb").Any() != expected.maxb || row.Get("minb").Any() != expected.minb ||
			row.Get("minb2x").Any() != expected.minb2x || row.Get("lastb").Any() != expected.lastb ||
			row.Get("firstb").Any() != expected.firstb || row.Get("countb").Any() != expected.countb {
			t.Fatalf("measure aggregation snapshot row %d = %#v", index, snapshot.Results()[index])
		}
	}
}

func TestRowRecogMeasureAggregationPartitioned(t *testing.T) {
	env, engine := newRowRecogPreviousTest(t)
	stream := From[rowRecogPreviousEvent](env, "RowRecogPreviousEvent").Window(KeepAll())
	category := Field[rowRecogPreviousEvent, string]("category")
	value := Field[rowRecogPreviousEvent, int]("value")
	plusA := Add[int](value, TagField[int]("A", "value"))
	query := stream.MatchRecognize(RowSequence(
		RowVar("A"), RowVar("B"), RowVar("B"), RowVar("C"), RowVar("C"), RowVar("D"),
	)).
		PartitionBy(category).
		Define("A", GreaterOrEqual[int](value, Literal(10))).
		Define("B", Greater[int](value, Literal(1))).
		Define("C", Less[int](value, Literal(-1))).
		Define("D", Equal[int](value, Literal(999))).
		Measures(
			Alias("cat", TagField[string]("A", "category")),
			Alias("a_string", TagField[string]("A", "name")),
			Alias("d_string", TagField[string]("D", "name")),
			Alias("sumb", TagSum[int]("B", value)),
			Alias("sumc", TagSum[int]("C", value)),
			Alias("sumaplusb", TagSum[int]("B", plusA)),
			Alias("sumaplusc", TagSum[int]("C", plusA)),
		).
		Query(
			StatementName("rowrecog-measure-aggregation-partitioned"),
			OrderBy(Ascending(ResultField[string]("cat"))),
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
	for _, event := range []rowRecogPreviousEvent{
		{Name: "E1", Category: "x", Value: 10},
		{Name: "E2", Category: "y", Value: 20},
		{Name: "E3", Category: "x", Value: 7},
		{Name: "E4", Category: "y", Value: 5},
		{Name: "E5", Category: "x", Value: 8},
		{Name: "E6", Category: "y", Value: 2},
		{Name: "E7", Category: "x", Value: -2},
		{Name: "E8", Category: "y", Value: -7},
		{Name: "E9", Category: "x", Value: -5},
		{Name: "E10", Category: "y", Value: -4},
		{Name: "E11", Category: "y", Value: 999},
		{Name: "E12", Category: "x", Value: 999},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	if len(*rows) != 2 {
		t.Fatalf("partitioned aggregation rows = %#v", *rows)
	}
	want := map[string]struct {
		name, d                  string
		sumb, sumc, sumab, sumac int
	}{
		"x": {"E1", "E12", 15, -7, 35, 13},
		"y": {"E2", "E11", 7, -11, 47, 29},
	}
	checkPartitioned := func(row Row) {
		cat := fmtRowValue(row.Get("cat"))
		values, ok := want[cat]
		if !ok || row.Get("a_string").Any() != values.name || row.Get("d_string").Any() != values.d ||
			row.Get("sumb").Any() != values.sumb || row.Get("sumc").Any() != values.sumc ||
			row.Get("sumaplusb").Any() != values.sumab || row.Get("sumaplusc").Any() != values.sumac {
			t.Fatalf("partitioned aggregation row = %#v", row)
		}
	}
	for _, row := range *rows {
		checkPartitioned(row)
	}
	snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 2 {
		t.Fatalf("partitioned aggregation snapshot = %#v", snapshot.Results())
	}
	for _, result := range snapshot.Results() {
		row, ok := result.Row()
		if !ok {
			t.Fatalf("partitioned aggregation snapshot result = %#v", result)
		}
		checkPartitioned(row)
	}
}

func fmtRowValue(value Value) string {
	if !value.IsPresent() {
		return ""
	}
	text, _ := value.Any().(string)
	return text
}
