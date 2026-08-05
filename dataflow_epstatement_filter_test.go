package esper

import (
	"context"
	"testing"
)

// TestDataflowEPStatementSourceWithStatementFilterTracksAllMatchingStatements
// covers Java EPLDataflowStatementFilter with a Go selector: an existing
// statement is attached at Start, later statements are considered, and a
// re-deployed matching statement is attached again after undeployment.
func TestDataflowEPStatementSourceWithStatementFilterTracksAllMatchingStatements(t *testing.T) {
	env, engine := newRuntimeTest(t)
	selectedPlan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("selected-statement")))
	if err != nil {
		t.Fatal(err)
	}
	selectedDeployment, err := engine.Deploy(context.Background(), selectedPlan)
	if err != nil {
		t.Fatal(err)
	}

	var selectorCalls int
	definition, err := DefineDataflow(env, "statement-filter-flow").
		EPStatementSourceWithStatementFilter("source", func(ctx DataflowStatementSourceContext) bool {
			selectorCalls++
			return ctx.StatementName == "selected-statement" && ctx.DeploymentID != "" && ctx.Statement != nil
		}).
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
	if selectorCalls != 1 {
		t.Fatalf("selector calls for existing statement = %d, want 1", selectorCalls)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "existing"}); err != nil {
		t.Fatal(err)
	}
	assertDataflowEventSymbols(t, instance.Outputs(), []string{"existing"})

	ignoredPlan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("ignored-statement")))
	if err != nil {
		t.Fatal(err)
	}
	ignoredDeployment, err := engine.Deploy(context.Background(), ignoredPlan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "ignored-only"}); err != nil {
		t.Fatal(err)
	}
	assertDataflowEventSymbols(t, instance.Outputs(), []string{"existing", "ignored-only"})

	if err := selectedDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "detached"}); err != nil {
		t.Fatal(err)
	}
	assertDataflowEventSymbols(t, instance.Outputs(), []string{"existing", "ignored-only"})
	if err := ignoredDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}

	redeployed, err := engine.Deploy(context.Background(), selectedPlan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "reconnected"}); err != nil {
		t.Fatal(err)
	}
	assertDataflowEventSymbols(t, instance.Outputs(), []string{"existing", "ignored-only", "reconnected"})

	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := redeployed.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDataflowEPStatementSourceWithStatementFilterRequiresSelector(t *testing.T) {
	env, _ := newRuntimeTest(t)
	if _, err := DefineDataflow(env, "missing-statement-filter").
		EPStatementSourceWithStatementFilter("source", nil).
		Emitter("emit").
		Build(); err == nil {
		t.Fatal("nil statement selector was accepted")
	}
}
