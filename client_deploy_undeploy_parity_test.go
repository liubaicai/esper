package esper

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type clientDeployUndeployEvent struct {
	Value string `esper:"value"`
}

func TestClientUndeployInvalidMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	engine := NewEngine(env)
	if err := engine.Undeploy(context.Background(), "nofound"); err == nil || !errors.Is(err, ErrorDeployment) {
		t.Fatalf("unknown undeploy error = %v", err)
	}
}

func TestClientUndeployDependencyChainMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[clientDeployUndeployEvent](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	moduleA, err := env.RegisterModule("undeploy.chain.a")
	if err != nil {
		t.Fatal(err)
	}
	moduleB, err := env.RegisterModule("undeploy.chain.b", WithModuleUses(moduleA.Name()))
	if err != nil {
		t.Fatal(err)
	}
	moduleC, err := env.RegisterModule("undeploy.chain.c", WithModuleUses(moduleB.Name()))
	if err != nil {
		t.Fatal(err)
	}
	moduleD, err := env.RegisterModule("undeploy.chain.d", WithModuleUses(moduleC.Name()))
	if err != nil {
		t.Fatal(err)
	}
	source := From[clientDeployUndeployEvent](env, "SupportBean")
	build := func(module Module, name string) Plan {
		t.Helper()
		plan, buildErr := module.Build(
			Select(source, Alias("value", Literal(10))).Query(StatementName(name)),
		)
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		return plan
	}
	engine := NewEngine(env)
	deployments := make([]*Deployment, 0, 4)
	for index, item := range []struct {
		module Module
		name   string
		id     string
	}{
		{module: moduleA, name: "A", id: "A"},
		{module: moduleB, name: "B", id: "B"},
		{module: moduleC, name: "C", id: "C"},
		{module: moduleD, name: "D", id: "D"},
	} {
		deployment, deployErr := engine.Deploy(context.Background(), build(item.module, item.name), WithDeploymentID(item.id))
		if deployErr != nil {
			t.Fatalf("deploy item %d: %v", index, deployErr)
		}
		deployments = append(deployments, deployment)
	}
	for index, want := range [][]string{nil, {"A"}, {"B"}, {"C"}} {
		if dependencies := deployments[index].Dependencies(); !reflect.DeepEqual(dependencies, want) {
			t.Fatalf("deployment %s dependencies = %v, want %v", deployments[index].ID(), dependencies, want)
		}
	}

	var values []int
	statement, ok := deployments[3].Statement("D")
	if !ok {
		t.Fatal("statement D is missing")
	}
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			values = append(values, row.Get("value").Any().(int))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientDeployUndeployEvent{Value: "event"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values, []int{10}) {
		t.Fatalf("dependency-chain value = %v", values)
	}

	assertUndeployPrecondition(t, engine.Undeploy(context.Background(), "A"), "A", "B")
	assertUndeployPrecondition(t, engine.Undeploy(context.Background(), "B"), "B", "C")
	if got := engine.Deployments(); !reflect.DeepEqual(got, deployments) {
		t.Fatalf("failed undeploy changed active deployments = %#v", got)
	}
	for index := len(deployments) - 1; index >= 0; index-- {
		if err := deployments[index].Undeploy(context.Background()); err != nil {
			t.Fatalf("undeploy %s: %v", deployments[index].ID(), err)
		}
	}
	if got := engine.Deployments(); len(got) != 0 {
		t.Fatalf("dependency chain remains active = %#v", got)
	}
}

func assertUndeployPrecondition(t *testing.T, err error, deploymentID, referencedBy string) {
	t.Helper()
	if err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("undeploy precondition = %v", err)
	}
	var precondition *UndeployPreconditionError
	if !errors.As(err, &precondition) || precondition.DeploymentID != deploymentID || precondition.ReferencedBy != referencedBy {
		t.Fatalf("undeploy precondition details = %#v", precondition)
	}
}
