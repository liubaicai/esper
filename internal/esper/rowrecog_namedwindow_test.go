package esper

import (
	"context"
	"testing"
)

func TestRowRecogNamedWindowConsumerStartsFromRetainedState(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[rowRecogDataWindowEvent](env, "RowRecogNamedWindowEvent")
	if err != nil {
		t.Fatal(err)
	}
	const windowName = "rowrecog-named-window-datawin"
	if _, err := CreateNamedWindow(env, windowName, schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	name := Field[rowRecogDataWindowEvent, string]("name")
	group := Field[rowRecogDataWindowEvent, string]("group")
	value := Field[rowRecogDataWindowEvent, int]("value")
	source := From[rowRecogDataWindowEvent](env, "RowRecogNamedWindowEvent")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow(windowName,
		SetColumn("name", name),
		SetColumn("group", group),
		SetColumn("value", value),
	).Query(StatementName("rowrecog-named-window-insert")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}

	for _, event := range []rowRecogDataWindowEvent{
		{Name: "A", Value: 1},
		{Name: "A", Value: 2},
		{Name: "B", Value: 1},
		{Name: "C", Value: 3},
	} {
		sendRowRecogDataWindowEvent(t, engine, event)
	}

	query := FromNamedWindow(env, windowName).
		MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		PartitionBy(group).
		Define("B", Equal[int](value, TagField[int]("A", "value"))).
		Measures(
			Alias("name", TagField[string]("A", "name")),
			Alias("aValue", TagField[int]("A", "value")),
			Alias("bValue", TagField[int]("B", "value")),
		).
		Query(StatementName("rowrecog-named-window-consumer"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)

	// The existing C/3 row is part of the consumer's retained state. The new
	// C/3 row must therefore complete the A-B sequence immediately after the
	// consumer is deployed, matching the Java named-window execution.
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "C", Value: 3})
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "E", Value: 5})
	sendRowRecogDataWindowEvent(t, engine, rowRecogDataWindowEvent{Name: "E", Value: 5})

	if len(*rows) != 2 {
		t.Fatalf("named-window row-recognize rows = %#v, want two", *rows)
	}
	if (*rows)[0].Get("name").Any() != "C" || (*rows)[0].Get("aValue").Any() != 3 || (*rows)[0].Get("bValue").Any() != 3 {
		t.Fatalf("named-window first row = %#v, want C/3/3", *rows)
	}
	if (*rows)[1].Get("name").Any() != "E" || (*rows)[1].Get("aValue").Any() != 5 || (*rows)[1].Get("bValue").Any() != 5 {
		t.Fatalf("named-window second row = %#v, want E/5/5", *rows)
	}
}
