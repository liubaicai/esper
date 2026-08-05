package esper

import (
	"context"
	"testing"
)

// TestDataflowEPStatementSourceByNameTracksDeploymentLifecycleMatchesEsper
// covers EPLDataflowStmtNameDynamic: the dataflow can start before its named
// statement exists, follows deployment and undeployment, and attaches to a
// later deployment with the same name.
func TestDataflowEPStatementSourceByNameTracksDeploymentLifecycleMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	definition, err := DefineDataflow(env, "dynamic-statement-flow").
		EPStatementSourceByName("source", "dynamic-statement").
		Emitter("emit").
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
	if instance.State() != DataflowRunning {
		t.Fatalf("named statement source without a statement = %v", instance.State())
	}

	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "before"}); err != nil {
		t.Fatal(err)
	}
	if got := len(instance.Outputs()); got != 0 {
		t.Fatalf("named source emitted before statement deployment: %d", got)
	}

	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("dynamic-statement")))
	if err != nil {
		t.Fatal(err)
	}
	firstDeployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "first"}); err != nil {
		t.Fatal(err)
	}
	assertDataflowEventSymbols(t, instance.Outputs(), []string{"first"})

	if err := firstDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "after-undeploy"}); err != nil {
		t.Fatal(err)
	}
	assertDataflowEventSymbols(t, instance.Outputs(), []string{"first"})

	secondDeployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "second"}); err != nil {
		t.Fatal(err)
	}
	assertDataflowEventSymbols(t, instance.Outputs(), []string{"first", "second"})

	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "after-cancel"}); err != nil {
		t.Fatal(err)
	}
	assertDataflowEventSymbols(t, instance.Outputs(), []string{"first", "second"})
	if err := secondDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDataflowEPStatementSourceByNameWithFilterMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	definition, err := DefineDataflow(env, "dynamic-statement-filter-flow").
		EPStatementSourceByNameWithFilter(
			"source",
			"dynamic-filter-statement",
			Greater[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0)),
		).
		Emitter("emit").
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

	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("dynamic-filter-statement")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "low", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "high", Price: 12}); err != nil {
		t.Fatal(err)
	}
	assertDataflowEventSymbols(t, instance.Outputs(), []string{"high"})

	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDataflowEPStatementSourceByNameRequiresName(t *testing.T) {
	env, _ := newRuntimeTest(t)
	if _, err := DefineDataflow(env, "missing-dynamic-statement").
		EPStatementSourceByName("source", "").
		Emitter("emit").
		Build(); err == nil {
		t.Fatal("empty named statement source was accepted")
	}
}

func assertDataflowEventSymbols(t *testing.T, values []any, want []string) {
	t.Helper()
	if len(values) != len(want) {
		t.Fatalf("dataflow values = %#v, want %d values", values, len(want))
	}
	for index, value := range values {
		event, ok := value.(Event)
		if !ok {
			t.Fatalf("dataflow value %d = %#v, want Event", index, value)
		}
		trade, ok := event.Underlying().(runtimeTestTrade)
		if !ok || trade.Symbol != want[index] {
			t.Fatalf("dataflow event %d = %#v, want symbol %q", index, value, want[index])
		}
	}
}
