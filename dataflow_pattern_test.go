package esper

import (
	"context"
	"testing"
)

// TestDataflowEPStatementSourceConsumesPatternRowsMatchesEsper exercises the
// Java EPStatementSource contract with a fluent Pattern statement. Pattern
// statements publish projection Rows rather than Events; the dataflow source
// must preserve that result shape while forwarding the new-stream batch.
func TestDataflowEPStatementSourceConsumesPatternRowsMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(
		base,
		"a",
		Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A")),
	).FollowedBy(
		"b",
		Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("B")),
	).Every()
	patternPlan, err := env.Build(pattern.Select(
		Alias("first", TagField[string]("a", "symbol")),
		Alias("second", TagField[string]("b", "symbol")),
	).Query(StatementName("pattern-source")))
	if err != nil {
		t.Fatal(err)
	}
	patternDeployment, err := engine.Deploy(context.Background(), patternPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer patternDeployment.Undeploy(context.Background())

	definition, err := DefineDataflow(env, "pattern-to-dataflow").
		EPStatementSource("pattern", patternDeployment.Statements()[0]).
		Emitter("sink").
		Connect("pattern", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer instance.Cancel(context.Background())

	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if got := instance.Outputs(); len(got) != 0 {
		t.Fatalf("pattern dataflow emitted before completion = %#v", got)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B"}); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 1 {
		t.Fatalf("pattern dataflow outputs = %#v", outputs)
	}
	row, ok := outputs[0].(Row)
	if !ok {
		t.Fatalf("pattern dataflow output = %#v, want Row", outputs[0])
	}
	if row.Get("first").Any() != "A" || row.Get("second").Any() != "B" {
		t.Fatalf("pattern dataflow row = %#v", row)
	}
}
