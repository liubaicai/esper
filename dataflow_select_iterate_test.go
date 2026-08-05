package esper

import (
	"context"
	"testing"
	"time"
)

type dataflowIterateEvent struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func TestDataflowSelectIterateFinalMarkerMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dataflowIterateEvent](env, "IterateEvent"); err != nil {
		t.Fatal(err)
	}
	groupKey := Field[dataflowIterateEvent, string]("theString")
	definition, err := DefineDataflow(env, "dataflow-select-iterate-final-marker").
		Emitter("source").
		SelectIterate("select", []Expr{groupKey}, []SortKey{Ascending(groupKey)},
			Alias("theString", groupKey),
			Alias("sumInt", Sum[int](Field[dataflowIterateEvent, int]("intPrimitive"))),
		).
		Emitter("sink").
		Connect("source", "select").
		Connect("select", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer instance.Cancel(context.Background())
	emitter, err := instance.CaptiveEmitter("source")
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []dataflowIterateEvent{
		{TheString: "E3", IntPrimitive: 4},
		{TheString: "E2", IntPrimitive: 3},
		{TheString: "E1", IntPrimitive: 1},
		{TheString: "E2", IntPrimitive: 2},
		{TheString: "E1", IntPrimitive: 5},
	} {
		if err := emitter.Submit(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if got := instance.Outputs(); len(got) != 0 {
		t.Fatalf("iterate Select emitted before FinalMarker = %#v", got)
	}
	if err := emitter.SubmitSignal(context.Background(), FinalMarker{}); err != nil {
		t.Fatal(err)
	}
	assertDataflowIterateRows(t, instance.Outputs(), []struct {
		name string
		sum  int
	}{{"E1", 6}, {"E2", 5}, {"E3", 4}})
}

func TestDataflowSelectIterateFinalMarkerLinearPathMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dataflowIterateEvent](env, "IterateEventLinear"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("IterateEventLinear")
	if !ok {
		t.Fatal("linear iterate event schema missing")
	}
	first, err := newEvent(schema, dataflowIterateEvent{TheString: "B", IntPrimitive: 2}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := newEvent(schema, dataflowIterateEvent{TheString: "A", IntPrimitive: 1}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	third, err := newEvent(schema, dataflowIterateEvent{TheString: "B", IntPrimitive: 3}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	groupKey := Field[dataflowIterateEvent, string]("theString")
	definition, err := DefineDataflow(env, "dataflow-select-iterate-linear").
		BeaconSource("source",
			first,
			second,
			third,
			FinalMarker{},
		).
		SelectIterate("select", []Expr{groupKey}, []SortKey{Ascending(groupKey)},
			Alias("theString", groupKey),
			Alias("sumInt", Sum[int](Field[dataflowIterateEvent, int]("intPrimitive"))),
		).
		Emitter("sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer instance.Cancel(context.Background())
	assertDataflowIterateRows(t, instance.Outputs(), []struct {
		name string
		sum  int
	}{{"A", 1}, {"B", 5}})
}

func TestDataflowSelectIterateRejectsRateLimitAndNilOrdering(t *testing.T) {
	env := NewEnvironment()
	_, err := DefineDataflow(env, "dataflow-select-iterate-invalid-rate").
		BeaconSource("source").
		SelectWithOptions("select", DataflowSelectOptions{
			IterateOnFinalMarker: true,
			OutputSnapshotEvery:  time.Second,
		}, Alias("value", Literal[int](1))).
		Emitter("sink").
		Build()
	if err == nil {
		t.Fatal("iterate Select accepted output snapshot rate limiting")
	}

	_, err = DefineDataflow(env, "dataflow-select-iterate-invalid-order").
		BeaconSource("source").
		SelectIterate("select", nil, []SortKey{{}}, Alias("value", Literal[int](1))).
		Emitter("sink").
		Build()
	if err == nil {
		t.Fatal("iterate Select accepted nil order-by expression")
	}
}

func assertDataflowIterateRows(t *testing.T, outputs []any, expected []struct {
	name string
	sum  int
}) {
	t.Helper()
	if len(outputs) != len(expected) {
		t.Fatalf("iterate Select output count = %d, want %d: %#v", len(outputs), len(expected), outputs)
	}
	for index, want := range expected {
		row, ok := outputs[index].(Row)
		if !ok {
			t.Fatalf("iterate Select output %d = %#v, want Row", index, outputs[index])
		}
		if got := row.Get("theString").Any(); got != want.name {
			t.Fatalf("iterate Select theString[%d] = %#v, want %q", index, got, want.name)
		}
		if got := row.Get("sumInt").Any(); got != want.sum {
			t.Fatalf("iterate Select sumInt[%d] = %#v, want %d", index, got, want.sum)
		}
	}
}
