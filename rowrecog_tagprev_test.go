package esper

import (
	"context"
	"errors"
	"testing"
)

func TestRowRecogTagAwarePrevAndPriorEvaluateAgainstPreviousEvent(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	current := TagField[float64]("A", "price")
	previous := PrevTag[float64](1, "A", "price")
	prior := PriorTag[float64](0, "A", "price")
	query := stream.MatchRecognize(RowSequence(RowVar("A"))).
		Define("A", And(
			Greater[float64](current, previous),
			Equal[float64](previous, prior),
		)).
		Measures(
			Alias("symbol", TagField[string]("A", "symbol")),
			Alias("previous", previous),
			Alias("prior", prior),
		).
		Query(StatementName("rowrecog-tag-aware-prev-prior"))
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
	for _, event := range []rowRecogTestEvent{
		{Symbol: "E1", Price: 5},
		{Symbol: "E2", Price: 3},
		{Symbol: "E3", Price: 6},
		{Symbol: "E4", Price: 4},
		{Symbol: "E5", Price: 7},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 2 {
		t.Fatalf("tag-aware previous listener rows = %#v", *rows)
	}
	if (*rows)[0].Get("symbol").Any() != "E3" || (*rows)[0].Get("previous").Any() != float64(3) || (*rows)[0].Get("prior").Any() != float64(3) ||
		(*rows)[1].Get("symbol").Any() != "E5" || (*rows)[1].Get("previous").Any() != float64(4) || (*rows)[1].Get("prior").Any() != float64(4) {
		t.Fatalf("tag-aware previous listener rows = %#v", *rows)
	}
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 2 {
		t.Fatalf("tag-aware previous snapshot = %#v", snapshot.Results())
	}
	first, firstOK := snapshot.Results()[0].Row()
	second, secondOK := snapshot.Results()[1].Row()
	if !firstOK || !secondOK || first.Get("previous").Any() != float64(3) || second.Get("prior").Any() != float64(4) {
		t.Fatalf("tag-aware previous snapshot rows = %#v", snapshot.Results())
	}

	invalid := stream.MatchRecognize(RowVar("A")).
		Define("A", Greater[float64](current, PrevTag[float64](1, "Unknown", "price"))).
		Measures(Alias("symbol", TagField[string]("A", "symbol"))).
		Query(StatementName("rowrecog-unknown-prev-tag"))
	if _, err := env.Build(invalid); err == nil || !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("unknown tag-aware previous error = %v", err)
	}
}

func TestRowRecogTagAwarePrevBindsRepeatedCapture(t *testing.T) {
	env, engine := newRowRecogRepetitionTest(t)
	stream := From[rowRecogRepetitionEvent](env, "RowRecogRepetitionEvent").Window(KeepAll())
	value := Field[rowRecogRepetitionEvent, int]("value")
	query := stream.MatchRecognize(RowVar("A").Repeat(3, 3)).
		Define("A", Greater[int](value, PrevTag[int](1, "A", "value"))).
		Measures(
			Alias("first", TagFieldAt[string]("A", 0, "name")),
			Alias("last", TagField[string]("A", "name")),
			Alias("count", TagCount("A")),
		).
		Query(StatementName("rowrecog-tag-aware-prev-repeated"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogRepetitionEvent{
		{Name: "A1", Value: 1},
		{Name: "A2", Value: 4},
		{Name: "A3", Value: 2},
		{Name: "A4", Value: 6},
		{Name: "A5", Value: 5},
		{Name: "A6", Value: 6},
		{Name: "A7", Value: 7},
		{Name: "A9", Value: 8},
	} {
		sendRowRecogRepetition(t, engine, event)
	}
	if len(*rows) != 1 {
		t.Fatalf("repeated tag-aware PREV rows = %#v", *rows)
	}
	row := (*rows)[0]
	if row.Get("first").Any() != "A6" || row.Get("last").Any() != "A9" || row.Get("count").Any() != int64(3) {
		t.Fatalf("repeated tag-aware PREV row = %#v, want A6..A9 count 3", *rows)
	}
}
