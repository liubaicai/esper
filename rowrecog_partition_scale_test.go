package esper

import (
	"context"
	"fmt"
	"testing"
)

func TestRowRecogUnlimitedPartitionLifecycle(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	group := Field[rowRecogTestEvent, string]("group")
	symbol := Field[rowRecogTestEvent, string]("symbol")
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		PartitionBy(group).
		AllMatches().
		Define("A", Equal[string](symbol, Literal("A"))).
		Define("B", Equal[string](symbol, Literal("B"))).
		Measures(
			Alias("group", TagField[string]("A", "group")),
			Alias("a", TagField[string]("A", "symbol")),
			Alias("b", TagField[string]("B", "symbol")),
		).
		Query(StatementName("rowrecog-unlimited-partition"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)

	const partitionCount = 64
	for index := 0; index < partitionCount; index++ {
		groupName := fmt.Sprintf("first-%04d", index)
		if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "A", Group: groupName}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "B", Group: groupName}); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != partitionCount {
		t.Fatalf("first high-partition batch rows = %d, want %d", len(*rows), partitionCount)
	}

	for index := 0; index < partitionCount; index++ {
		groupName := fmt.Sprintf("second-%04d", index)
		if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "A", Group: groupName}); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != partitionCount {
		t.Fatalf("second high-partition pending rows = %d, want no additional matches", len(*rows))
	}
	for index := 0; index < partitionCount; index++ {
		groupName := fmt.Sprintf("second-%04d", index)
		if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "B", Group: groupName}); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 2*partitionCount {
		t.Fatalf("second high-partition batch rows = %d, want %d", len(*rows), 2*partitionCount)
	}

	seen := make(map[string]int, 2*partitionCount)
	for _, row := range *rows {
		groupValue, ok := row.Get("group").Any().(string)
		if !ok || row.Get("a").Any() != "A" || row.Get("b").Any() != "B" {
			t.Fatalf("high-partition row = %#v", row)
		}
		seen[groupValue]++
	}
	if len(seen) != 2*partitionCount {
		t.Fatalf("high-partition groups = %d, want %d", len(seen), 2*partitionCount)
	}
	for groupName, count := range seen {
		if count != 1 {
			t.Fatalf("high-partition group %s matched %d times", groupName, count)
		}
	}
}
