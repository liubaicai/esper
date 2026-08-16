package esper

import (
	"context"
	"testing"
	"time"
)

type partitionSelectionBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type partitionSelectionS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 string  `esper:"p01"`
}

type partitionSelectionS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 string  `esper:"p11"`
}

// TestContextPartitionSelectionIteratorsAndPropertiesParity locks the
// iterator-selector surface verified against
// ContextInitTermContextPartitionSelection: an overlapping filter-initiated
// context terminated by a correlated id filter exposes per-partition
// descriptors with startTime (epoch millis) and the initiating event's p00,
// SnapshotWithSelector selects partitions by ID or by initiating-event
// property, and a segmented selector is rejected with the
// invalid-context-partition-selector category.
func TestContextPartitionSelectionIteratorsAndPropertiesParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[partitionSelectionBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[partitionSelectionS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[partitionSelectionS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}
	beanSource := From[partitionSelectionBean](env, "SupportBean")
	isS0 := Equal[string](TypeName(EventValue[Event]()), Literal("SupportBean_S0"))
	end := And(
		Equal[string](TypeName(EventValue[Event]()), Literal("SupportBean_S1")),
		Equal[int](Field[partitionSelectionS1, int]("id"), Property[int](ContextInitiatingEvent(), "id")))
	if _, err := CreateOverlappingInitiatedTerminatedContext(env, "MyCtx", Literal("global"), isS0, end); err != nil {
		t.Fatal(err)
	}
	query := beanSource.Window(KeepAll()).GroupBy(
		Field[partitionSelectionBean, string]("theString"),
	).Select(
		Alias("c0", ContextField[int]("id")),
		Alias("c1", Property[string](ContextInitiatingEvent(), "p00")),
		Alias("c2", Field[partitionSelectionBean, string]("theString")),
		Alias("c3", Sum[int](Field[partitionSelectionBean, int]("intPrimitive"))),
	).Query(StatementName("s0"), WithContext("MyCtx"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(1000).UTC()); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	p00One := "S0_1"
	if err := engine.SendEvent(context.Background(), partitionSelectionS0{ID: 1, P00: &p00One}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), partitionSelectionBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), partitionSelectionBean{TheString: "E2", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(2000).UTC()); err != nil {
		t.Fatal(err)
	}
	p00Two := "S0_2"
	if err := engine.SendEvent(context.Background(), partitionSelectionS0{ID: 2, P00: &p00Two}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), partitionSelectionBean{TheString: "E3", IntPrimitive: 100}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), partitionSelectionBean{TheString: "E1", IntPrimitive: 3}); err != nil {
		t.Fatal(err)
	}

	descriptors, err := engine.ContextPartitionDescriptors("MyCtx", ContextPartitionSelectorAll{})
	if err != nil {
		t.Fatal(err)
	}
	if len(descriptors) != 2 {
		t.Fatalf("partition descriptors = %d, want 2", len(descriptors))
	}
	start0, _ := descriptors[0].Property("startTime")
	start1, _ := descriptors[1].Property("startTime")
	if got := start0.Any().(time.Time).UnixMilli(); got != 1000 {
		t.Fatalf("partition 0 startTime = %v, want 1000", got)
	}
	if got := start1.Any().(time.Time).UnixMilli(); got != 2000 {
		t.Fatalf("partition 1 startTime = %v, want 2000", got)
	}
	initiating0, _ := descriptors[0].Property("initiating_event")
	initiating1, _ := descriptors[1].Property("initiating_event")
	if got := derefString(initiating0.Any().(Event).Get("p00").Any()); got != "S0_1" {
		t.Fatalf("partition 0 initiating p00 = %v, want S0_1", got)
	}
	if got := derefString(initiating1.Any().(Event).Get("p00").Any()); got != "S0_2" {
		t.Fatalf("partition 1 initiating p00 = %v, want S0_2", got)
	}

	// Snapshot with the by-ID selector returns only partition 1.
	byID, err := statement.SnapshotWithSelector(context.Background(), SelectContextPartitionIDs(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(byID.Batch.New) != 2 {
		t.Fatalf("by-id snapshot rows = %d, want 2", len(byID.Batch.New))
	}
	for _, result := range byID.Batch.New {
		row, ok := result.Row()
		if !ok || row.Get("c0").Any() != 1 {
			t.Fatalf("by-id snapshot row = %#v, want partition 1", result)
		}
	}

	// Snapshot with a descriptor selector matching the initiating event p00.
	filtered := ContextPartitionSelectorDescriptorFunc(func(descriptor ContextPartitionDescriptor) bool {
		initiating, ok := descriptor.Property("initiating_event")
		if !ok || !initiating.IsPresent() {
			return false
		}
		event, ok := initiating.Any().(Event)
		if !ok {
			return false
		}
		return derefString(event.Get("p00").Any()) == "S0_2"
	})
	byFilter, err := statement.SnapshotWithSelector(context.Background(), filtered)
	if err != nil {
		t.Fatal(err)
	}
	if len(byFilter.Batch.New) != 2 {
		t.Fatalf("filtered snapshot rows = %d, want 2", len(byFilter.Batch.New))
	}

	// Segmented selector is rejected for an initiated context.
	_, err = statement.SnapshotWithSelector(context.Background(), SelectContextPartitionSegments([]any{}))
	if err == nil {
		t.Fatal("segmented selector on initiated context must be rejected")
	}
	if _, ok := err.(*Error); !ok {
		t.Fatalf("selector error = %#v, want typed Error", err)
	}
}

func derefString(value any) string {
	if pointer, ok := value.(*string); ok && pointer != nil {
		return *pointer
	}
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
