package esper

import (
	"context"
	"reflect"
	"testing"
)

type containedNamedWindowBook struct {
	ID    string  `esper:"bookId"`
	Price float64 `esper:"price"`
}

type containedNamedWindowOrder struct {
	Books []containedNamedWindowBook `esper:"books"`
}

func TestContainedNamedWindowUpdatesBeforeNextChildProbeMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[containedNamedWindowOrder](env, "ContainedNamedWindowOrder"); err != nil {
		t.Fatal(err)
	}
	bookSchema, err := RegisterStruct[containedNamedWindowBook](env, "ContainedNamedWindowBook")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[containedNamedWindowBook](env, "ContainedBookStream"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "ContainedBookWindow", bookSchema, NamedWindowRetention(LastEvent())); err != nil {
		t.Fatal(err)
	}

	orders := From[containedNamedWindowOrder](env, "ContainedNamedWindowOrder")
	books := Unnest[containedNamedWindowOrder, containedNamedWindowBook](
		orders,
		Property[[]containedNamedWindowBook](EventValue[containedNamedWindowOrder](), "books"),
	)
	bookRoutePlan, err := env.Build(books.InsertInto(
		"ContainedBookStream",
		StatementName("contained-named-window-book-stream"),
	))
	if err != nil {
		t.Fatal(err)
	}
	window := FromNamedWindow(env, "ContainedBookWindow")
	previousBookIsMoreExpensive := SubqueryExists(
		window,
		Greater[float64](Field[any, float64]("price"), OuterField[float64]("price")),
	)
	insertPlan, err := env.Build(OnEvent(From[containedNamedWindowBook](env, "ContainedBookStream").Filter(Not(previousBookIsMoreExpensive))).InsertIntoNamedWindow(
		"ContainedBookWindow",
		SetColumn("bookId", Field[containedNamedWindowBook, string]("bookId")),
		SetColumn("price", Field[containedNamedWindowBook, float64]("price")),
	).Query(StatementName("contained-named-window-insert")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	bookDeployment, err := engine.Deploy(context.Background(), bookRoutePlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = bookDeployment.Undeploy(context.Background()) }()
	insertDeployment, err := engine.Deploy(context.Background(), insertPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = insertDeployment.Undeploy(context.Background()) }()
	var routed []string
	if _, err := bookDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				continue
			}
			routed = append(routed, event.Get("bookId").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	runtimeWindow, ok := engine.NamedWindow("ContainedBookWindow")
	if !ok {
		t.Fatal("contained named window is missing")
	}
	var inserted []string
	if _, err := runtimeWindow.Subscribe(func(_ context.Context, delta NamedWindowDelta) error {
		for _, event := range delta.New {
			inserted = append(inserted, event.Get("bookId").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), containedNamedWindowOrder{Books: []containedNamedWindowBook{
		{ID: "B35", Price: 35},
		{ID: "B10", Price: 10},
		{ID: "B27", Price: 27},
	}}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(routed, []string{"B35", "B10", "B27"}) {
		t.Fatalf("contained route events = %#v, want parent-array order", routed)
	}

	events, err := runtimeWindow.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(inserted, []string{"B35"}) {
		t.Fatalf("contained named-window inserts = %#v, want preemptive highest-book insert", inserted)
	}
	if len(events) != 1 || events[0].Get("bookId").Any() != "B35" || events[0].Get("price").Any() != float64(35) {
		t.Fatalf("contained named-window snapshot = %#v, inserted=%v, want only highest book", events, inserted)
	}
}

func TestContainedNamedWindowDirectTriggerTraversalIsPreemptive(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[containedNamedWindowOrder](env, "ContainedDirectOrder"); err != nil {
		t.Fatal(err)
	}
	bookSchema, err := RegisterStruct[containedNamedWindowBook](env, "ContainedDirectBook")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "ContainedDirectWindow", bookSchema, NamedWindowRetention(LastEvent())); err != nil {
		t.Fatal(err)
	}

	orders := From[containedNamedWindowOrder](env, "ContainedDirectOrder")
	books := Unnest[containedNamedWindowOrder, containedNamedWindowBook](
		orders,
		Property[[]containedNamedWindowBook](EventValue[containedNamedWindowOrder](), "books"),
	)
	window := FromNamedWindow(env, "ContainedDirectWindow")
	previousBookIsMoreExpensive := SubqueryExists(
		window,
		Greater[float64](Field[any, float64]("price"), OuterField[float64]("price")),
	)
	plan, err := env.Build(OnEvent(books.Filter(Not(previousBookIsMoreExpensive))).InsertIntoNamedWindow(
		"ContainedDirectWindow",
		SetColumn("bookId", Field[containedNamedWindowBook, string]("bookId")),
		SetColumn("price", Field[containedNamedWindowBook, float64]("price")),
	).Query(StatementName("contained-named-window-direct-trigger")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), containedNamedWindowOrder{Books: []containedNamedWindowBook{
		{ID: "B35", Price: 35},
		{ID: "B10", Price: 10},
		{ID: "B27", Price: 27},
	}}); err != nil {
		t.Fatal(err)
	}

	runtimeWindow, ok := engine.NamedWindow("ContainedDirectWindow")
	if !ok {
		t.Fatal("contained direct named window is missing")
	}
	events, err := runtimeWindow.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Get("bookId").Any() != "B35" || events[0].Get("price").Any() != float64(35) {
		t.Fatalf("direct contained named-window snapshot = %#v, want only highest book", events)
	}
}
