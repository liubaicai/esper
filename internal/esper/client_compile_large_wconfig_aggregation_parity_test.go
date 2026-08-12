package esper

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

type clientCompileLargeAggregateEvent struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func newClientCompileLargeAggregateEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientCompileLargeAggregateEvent](env, "ClientCompileLargeAggregateEvent"); err != nil {
		t.Fatal(err)
	}
	return env
}

func TestClientCompileLargeAggregationMatchesEsper(t *testing.T) {
	const columnCount = 5000
	env := newClientCompileLargeAggregateEnvironment(t)
	value := Field[clientCompileLargeAggregateEvent, int]("intPrimitive")
	selections := make([]Selection, columnCount)
	for index := range selections {
		selections[index] = Alias(
			fmt.Sprintf("c%d", index),
			Sum[int](Add[int](value, Literal(index))),
		)
	}
	plan, err := env.Build(
		From[clientCompileLargeAggregateEvent](env, "ClientCompileLargeAggregateEvent").
			Window(LastEvent()).
			Aggregate(selections...).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement, ok := deployment.Statement("s0")
	if !ok {
		t.Fatal("large aggregate statement s0 is missing")
	}
	var input int
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) != 1 {
			t.Fatalf("large aggregate batch = %#v", batch)
		}
		row, ok := batch.New[0].Row()
		if !ok || len(row.Schema().Fields()) != columnCount {
			t.Fatalf("large aggregate result = row %t fields %d", ok, len(row.Schema().Fields()))
		}
		for index := 0; index < columnCount; index++ {
			if got := row.Get(fmt.Sprintf("c%d", index)).Any(); got != input+index {
				t.Fatalf("large aggregate c%d = %#v, want %d", index, got, input+index)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, next := range []int{10, 50} {
		input = next
		if err := engine.Send(context.Background(), "ClientCompileLargeAggregateEvent", clientCompileLargeAggregateEvent{TheString: "x", IntPrimitive: next}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestClientCompileLargeAggregationAccessMatchesEsper(t *testing.T) {
	const accessCount = 1000
	env := newClientCompileLargeAggregateEnvironment(t)
	value := Field[clientCompileLargeAggregateEvent, int]("intPrimitive")
	eventValue := EventValue[clientCompileLargeAggregateEvent]()
	selections := make([]Selection, 0, accessCount*2)
	for index := 0; index < accessCount; index++ {
		sorted := SortedAccessBy[clientCompileLargeAggregateEvent, int](
			eventValue,
			Add[int](value, Literal(index)),
		)
		selections = append(selections,
			Alias(fmt.Sprintf("c%d", index), sorted.FirstEvent()),
			Alias(fmt.Sprintf("d%d", index), sorted.Sorted()),
		)
	}
	plan, err := env.Build(
		From[clientCompileLargeAggregateEvent](env, "ClientCompileLargeAggregateEvent").
			Window(KeepAll()).
			Aggregate(selections...).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement, ok := deployment.Statement("s0")
	if !ok {
		t.Fatal("large access aggregate statement s0 is missing")
	}
	want := clientCompileLargeAggregateEvent{TheString: "E1", IntPrimitive: 10}
	received := false
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) != 1 {
			t.Fatalf("large access aggregate batch = %#v", batch)
		}
		row, ok := batch.New[0].Row()
		if !ok || len(row.Schema().Fields()) != accessCount*2 {
			t.Fatalf("large access aggregate result = row %t fields %d", ok, len(row.Schema().Fields()))
		}
		for index := 0; index < accessCount; index++ {
			if got := row.Get(fmt.Sprintf("c%d", index)).Any(); !reflect.DeepEqual(got, want) {
				t.Fatalf("large access c%d = %#v, want %#v", index, got, want)
			}
			if got := row.Get(fmt.Sprintf("d%d", index)).Any(); !reflect.DeepEqual(got, []clientCompileLargeAggregateEvent{want}) {
				t.Fatalf("large access d%d = %#v", index, got)
			}
		}
		received = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "ClientCompileLargeAggregateEvent", want); err != nil {
		t.Fatal(err)
	}
	if !received {
		t.Fatal("large access aggregate produced no result")
	}
}
