package esper

import (
	"context"
	"testing"
)

type rowRecogUnboundEvent struct {
	String string `esper:"string"`
	Value  int    `esper:"value"`
}

func TestRowRecogUnboundStreamListenerAndEmptyIterator(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[rowRecogUnboundEvent](env, "RowRecogUnboundEvent"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	stringField := Field[rowRecogUnboundEvent, string]("string")
	query := From[rowRecogUnboundEvent](env, "RowRecogUnboundEvent").
		MatchRecognize(RowSequence(RowVar("A"))).
		Define("A", Equal[string](Prev[string](1, stringField), stringField)).
		Measures(
			Alias("string", TagField[string]("A", "string")),
			Alias("value", TagField[int]("A", "value")),
		).
		Query(StatementName("rowrecog-unbound-no-iterator"))
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
	for _, event := range []rowRecogUnboundEvent{
		{String: "s1", Value: 1},
		{String: "s2", Value: 2},
		{String: "s1", Value: 3},
		{String: "s3", Value: 4},
		{String: "s2", Value: 5},
		{String: "s1", Value: 6},
		{String: "s1", Value: 7},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 1 || (*rows)[0].Get("string").Any() != "s1" || (*rows)[0].Get("value").Any() != 7 {
		t.Fatalf("unbound listener rows = %#v", *rows)
	}
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 0 {
		t.Fatalf("unbound iterator exposed rows = %#v", snapshot.Results())
	}
}
