package esper

import (
	"context"
	"testing"
	"time"
)

type rowRecogPerfEvent struct {
	Name      string `esper:"name"`
	Category  string `esper:"cat"`
	Partition int    `esper:"value"`
}

func TestRowRecogPerformanceBaseline(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[rowRecogPerfEvent](env, "RowRecogPerfEvent"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	stream := From[rowRecogPerfEvent](env, "RowRecogPerfEvent")
	category := Field[rowRecogPerfEvent, string]("cat")
	partition := Field[rowRecogPerfEvent, int]("value")
	query := stream.MatchRecognize(RowSequence(
		RowVar("A"),
		RowVar("B").ZeroOrMore().Reluctant(),
		RowVar("C"),
	)).
		PartitionBy(partition).
		AllMatches().
		Define("A", Equal[string](category, Literal("1"))).
		Define("B", Equal[string](category, Literal("2"))).
		Define("C", Equal[string](category, Literal("3"))).
		Measures(
			Alias("a", TagField[string]("A", "name")),
			Alias("c", TagField[string]("C", "name")),
		).
		Query(StatementName("rowrecog-performance-baseline"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)

	const eventsPerPartition = 25000
	started := time.Now()
	for value := 0; value < 2; value++ {
		rowsBefore := len(*rows)
		if err := engine.SendEvent(context.Background(), rowRecogPerfEvent{Name: "A", Category: "1", Partition: value}); err != nil {
			t.Fatal(err)
		}
		for index := 0; index < eventsPerPartition; index++ {
			if err := engine.SendEvent(context.Background(), rowRecogPerfEvent{Name: "B", Category: "2", Partition: value}); err != nil {
				t.Fatal(err)
			}
		}
		if len(*rows) != rowsBefore {
			t.Fatalf("row-recog performance emitted before terminal C in partition %d: %d rows", value, len(*rows))
		}
		if err := engine.SendEvent(context.Background(), rowRecogPerfEvent{Name: "C", Category: "3", Partition: value}); err != nil {
			t.Fatal(err)
		}
		if len(*rows) <= value {
			t.Fatalf("row-recog performance emitted no match for partition %d: %#v", value, *rows)
		}
		row := (*rows)[value]
		if row.Get("a").Any() != "A" || row.Get("c").Any() != "C" {
			t.Fatalf("row-recog performance partition %d row = %#v", value, row)
		}
	}
	t.Logf("row-recog performance baseline: %s for %d events", time.Since(started), 2*(eventsPerPartition+2))
}
