package esper

import (
	"context"
	"reflect"
	"testing"
)

func TestDataflowBuiltinPortInferenceMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)

	eventDefinition, err := DefineDataflow(env, "builtin-event-ports").
		EventBusSource("source", "Trade").
		Filter("filter", Greater[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0))).
		Select("select", Alias("symbol", Field[runtimeTestTrade, string]("symbol"))).
		Emitter("sink").
		Connect("source", "filter").
		Connect("filter", "select").
		Connect("select", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	assertDataflowPortType(t, eventDefinition, "source", true, "out", reflect.TypeOf(Event{}))
	assertDataflowPortType(t, eventDefinition, "filter", false, "in", reflect.TypeOf(Event{}))
	assertDataflowPortType(t, eventDefinition, "filter", true, "out", reflect.TypeOf(Event{}))
	assertDataflowPortType(t, eventDefinition, "select", false, "in", reflect.TypeOf(Event{}))
	assertDataflowPortType(t, eventDefinition, "select", true, "out", reflect.TypeOf(Row{}))

	eventPlan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("event-port-source")))
	if err != nil {
		t.Fatal(err)
	}
	eventDeployment, err := engine.Deploy(context.Background(), eventPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer eventDeployment.Undeploy(context.Background())

	rowPlan, err := env.Build(Select(
		From[runtimeTestTrade](env, "Trade"),
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
	).Query(StatementName("row-port-source")))
	if err != nil {
		t.Fatal(err)
	}
	rowDeployment, err := engine.Deploy(context.Background(), rowPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer rowDeployment.Undeploy(context.Background())

	statementDefinition, err := DefineDataflow(env, "statement-ports").
		EPStatementSource("events", eventDeployment.Statements()[0]).
		EPStatementSource("rows", rowDeployment.Statements()[0]).
		Emitter("event-sink").
		Emitter("row-sink").
		Connect("events", "event-sink").
		Connect("rows", "row-sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	assertDataflowPortType(t, statementDefinition, "events", true, "out", reflect.TypeOf(Event{}))
	assertDataflowPortType(t, statementDefinition, "rows", true, "out", reflect.TypeOf(Row{}))

	if _, err := DefineDataflow(env, "row-to-event-sink").
		EPStatementSource("rows", rowDeployment.Statements()[0]).
		EventBusSink("sink", "Trade").
		Connect("rows", "sink").
		Build(); err == nil {
		t.Fatal("row output was accepted by an EventBusSink")
	}
	if _, err := DefineDataflow(env, "row-pass-through-to-event-sink").
		EPStatementSource("rows", rowDeployment.Statements()[0]).
		SelectPassThrough("pass").
		EventBusSink("sink", "Trade").
		Connect("rows", "pass").
		Connect("pass", "sink").
		Build(); err == nil {
		t.Fatal("row pass-through output was accepted by an EventBusSink")
	}
	if _, err := DefineDataflow(env, "event-to-event-sink").
		EPStatementSource("events", eventDeployment.Statements()[0]).
		EventBusSink("sink", "Trade").
		Connect("events", "sink").
		Build(); err != nil {
		t.Fatalf("event output was rejected by an EventBusSink: %v", err)
	}
}

func assertDataflowPortType(t *testing.T, definition DataflowDefinition, name string, output bool, port string, want reflect.Type) {
	t.Helper()
	for _, operator := range definition.Operators() {
		if operator.Name != name {
			continue
		}
		got := dataflowPortType(operator, output, port)
		if got != want {
			t.Fatalf("dataflow %q %s port %q type = %v, want %v", name, map[bool]string{true: "output", false: "input"}[output], port, got, want)
		}
		return
	}
	t.Fatalf("dataflow operator %q not found", name)
}
