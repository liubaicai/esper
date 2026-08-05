package esper

import (
	"context"
	"testing"
)

type aggregateEverEvent struct {
	Name  string `esper:"theString"`
	Value int    `esper:"intPrimitive"`
	Boxed *int   `esper:"intBoxed"`
	Flag  bool   `esper:"boolPrimitive"`
}

type aggregateEverDeleteEvent struct {
	ID string `esper:"id"`
}

func newAggregateEverTest(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[aggregateEverEvent](env, "AggregateEverEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[aggregateEverDeleteEvent](env, "AggregateEverDeleteEvent"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

func TestAggregateFirstLastEverWindowAndFilteredTrace(t *testing.T) {
	env, engine := newAggregateEverTest(t)
	name := Field[aggregateEverEvent, string]("theString")
	boxed := Field[aggregateEverEvent, *int]("intBoxed")
	flag := Field[aggregateEverEvent, bool]("boolPrimitive")
	plan, err := env.Build(From[aggregateEverEvent](env, "AggregateEverEvent").Window(LengthWindow(2)).Aggregate(
		Alias("firstEver", FirstEver[string](name)),
		Alias("lastEver", LastEver[string](name)),
		Alias("first", First[string](name)),
		Alias("last", Last[string](name)),
		Alias("countEver", CountEver()),
		Alias("countEverValue", CountEver(boxed)),
		Alias("countEverFiltered", FilterAggregate[int64](CountEver(), flag)),
		Alias("countEverValueFiltered", FilterAggregate[int64](CountEver(boxed), flag)),
	).Query(StatementName("aggregate-first-last-ever-trace")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var latest Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) == 0 {
			return nil
		}
		row, ok := batch.New[len(batch.New)-1].Row()
		if !ok {
			t.Fatalf("ever aggregate result is not a row: %#v", batch.New[len(batch.New)-1])
		}
		latest = row
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	boxed100, boxed120, boxed130 := 100, 120, 130
	events := []struct {
		event                 aggregateEverEvent
		firstEver, lastEver   string
		first, last           string
		countEver, valueCount int64
		filtered, valueFilter int64
	}{
		{aggregateEverEvent{Name: "E1", Value: 10, Boxed: &boxed100, Flag: true}, "E1", "E1", "E1", "E1", 1, 1, 1, 1},
		{aggregateEverEvent{Name: "E2", Value: 11, Flag: true}, "E1", "E2", "E1", "E2", 2, 1, 2, 1},
		{aggregateEverEvent{Name: "E3", Value: 12, Boxed: &boxed120, Flag: false}, "E1", "E3", "E2", "E3", 3, 2, 2, 1},
		{aggregateEverEvent{Name: "E4", Value: 13, Boxed: &boxed130, Flag: true}, "E1", "E4", "E3", "E4", 4, 3, 3, 2},
	}
	for index, expected := range events {
		if err := engine.SendEvent(context.Background(), expected.event); err != nil {
			t.Fatal(err)
		}
		if latest.Get("firstEver").Any() != expected.firstEver || latest.Get("lastEver").Any() != expected.lastEver ||
			latest.Get("first").Any() != expected.first || latest.Get("last").Any() != expected.last ||
			latest.Get("countEver").Any() != expected.countEver || latest.Get("countEverValue").Any() != expected.valueCount ||
			latest.Get("countEverFiltered").Any() != expected.filtered || latest.Get("countEverValueFiltered").Any() != expected.valueFilter {
			t.Fatalf("ever aggregate row %d = %#v", index, latest.AsMap())
		}
	}
}

func TestAggregateFirstLastEverNamedWindowDeleteRetainsHistory(t *testing.T) {
	env, _ := newAggregateEverTest(t)
	schema, ok := env.Schema("AggregateEverEvent")
	if !ok {
		t.Fatal("aggregate ever schema is missing")
	}
	if _, err := CreateNamedWindow(env, "aggregate-ever-window", schema); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	source := From[aggregateEverEvent](env, "AggregateEverEvent")
	name := Field[aggregateEverEvent, string]("theString")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow("aggregate-ever-window",
		SetColumn("theString", name),
		SetColumn("intPrimitive", Field[aggregateEverEvent, int]("intPrimitive")),
		SetColumn("intBoxed", Field[aggregateEverEvent, *int]("intBoxed")),
		SetColumn("boolPrimitive", Field[aggregateEverEvent, bool]("boolPrimitive")),
	).Query(StatementName("aggregate-ever-insert")))
	if err != nil {
		t.Fatal(err)
	}
	windowName := Field[any, string]("theString")
	aggregatePlan, err := env.Build(FromNamedWindow(env, "aggregate-ever-window").Aggregate(
		Alias("firstEver", FirstEver[string](windowName)),
		Alias("lastEver", LastEver[string](windowName)),
		Alias("countEver", CountEver()),
	).Query(StatementName("aggregate-ever-window-query")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[aggregateEverDeleteEvent](env, "AggregateEverDeleteEvent")).DeleteFromNamedWindow(
		"aggregate-ever-window",
		Equal[string](NamedWindowField[string]("theString"), Field[aggregateEverDeleteEvent, string]("id")),
	).Query(StatementName("aggregate-ever-delete")))
	if err != nil {
		t.Fatal(err)
	}
	aggregateDeployment, err := engine.Deploy(context.Background(), aggregatePlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), deletePlan); err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 6)
	if _, err := aggregateDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []aggregateEverEvent{{Name: "E1"}, {Name: "E2"}, {Name: "E3"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"E2", "E3", "E1"} {
		if err := engine.SendEvent(context.Background(), aggregateEverDeleteEvent{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 6 {
		t.Fatalf("ever aggregate delete rows = %d", len(rows))
	}
	expectedLast := []string{"E1", "E2", "E3", "E3", "E3", "E3"}
	expectedCount := []int64{1, 2, 3, 3, 3, 3}
	for index, row := range rows {
		if row.Get("firstEver").Any() != "E1" || row.Get("lastEver").Any() != expectedLast[index] || row.Get("countEver").Any() != expectedCount[index] {
			t.Fatalf("ever aggregate row %d = %#v", index, row.AsMap())
		}
	}
}

func TestAggregateCountEverRejectsMultipleExpressions(t *testing.T) {
	env, _ := newAggregateEverTest(t)
	name := Field[aggregateEverEvent, string]("theString")
	flag := Field[aggregateEverEvent, bool]("boolPrimitive")
	query := From[aggregateEverEvent](env, "AggregateEverEvent").Aggregate(
		Alias("invalid", CountEver(name, flag)),
	).Query(StatementName("invalid-count-ever"))
	if _, err := env.Build(query); err == nil {
		t.Fatal("CountEver with multiple expressions unexpectedly built")
	}
}
