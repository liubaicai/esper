package esper

import (
	"context"
	"testing"
)

type clientInstrumentSupportBean struct{}

func TestClientInstrumentInstrumentationDisabledParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[clientInstrumentSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(
		From[clientInstrumentSupportBean](env, "SupportBean"),
		Alias("equals", Equal[int](Literal(1), Literal(2))),
	).Query(StatementName("instrumentation-disabled")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	rows, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows.Results()) != 0 {
		t.Fatalf("instrumentation-disabled initial snapshot = %#v, want empty", rows)
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := statement.Snapshot(context.Background()); err == nil {
		t.Fatal("destroyed instrumentation-disabled statement snapshot succeeded")
	}
}
