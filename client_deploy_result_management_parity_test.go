package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type clientDeployResultEvent struct {
	Value string `esper:"value"`
}

func newClientDeployResultEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientDeployResultEvent](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func TestClientDeployResultSimpleMatchesEsper(t *testing.T) {
	env := newClientDeployResultEnvironment(t)
	source := From[clientDeployResultEvent](env, "SupportBean")
	first, err := env.Build(source.Query(StatementName("StmtOne")))
	if err != nil {
		t.Fatal(err)
	}
	second, err := env.Build(
		Select(source, Alias("value", Field[clientDeployResultEvent, string]("value"))).Query(StatementName("StmtTwo")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.DeployPlans(context.Background(), []Plan{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if deployment.ID() == "" || !engine.IsDeployed(deployment.ID()) {
		t.Fatalf("deployment identity/active = %q/%t", deployment.ID(), engine.IsDeployed(deployment.ID()))
	}
	if resolved, ok := engine.Deployment(deployment.ID()); !ok || resolved != deployment {
		t.Fatalf("deployment lookup = %p, %t; want %p", resolved, ok, deployment)
	}
	if got := engine.Deployments(); !reflect.DeepEqual(got, []*Deployment{deployment}) {
		t.Fatalf("active deployments = %#v", got)
	}
	statements := deployment.Statements()
	if len(statements) != 2 || statements[0].Name() != "StmtOne" || statements[1].Name() != "StmtTwo" {
		t.Fatalf("deployment statements = %v", clientRuntimeStatementNames(statements))
	}
	if dependencies := deployment.Dependencies(); len(dependencies) != 0 {
		t.Fatalf("unexpected deployment dependencies = %v", dependencies)
	}
	if !strings.Contains(statements[0].Plan().Query().TypedDescription(), "SupportBean") ||
		!strings.Contains(statements[1].Plan().Query().TypedDescription(), "SupportBean") {
		t.Fatalf("typed statement descriptions = %q / %q",
			statements[0].Plan().Query().TypedDescription(), statements[1].Plan().Query().TypedDescription())
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if engine.IsDeployed(deployment.ID()) || len(engine.Deployments()) != 0 {
		t.Fatalf("undeployed result remains active: %q / %#v", deployment.ID(), engine.Deployments())
	}
}

func TestClientDeployGetStatementByDeploymentIDAndNameMatchesEsper(t *testing.T) {
	env := newClientDeployResultEnvironment(t)
	base, err := env.Build(From[clientDeployResultEvent](env, "SupportBean").Query())
	if err != nil {
		t.Fatal(err)
	}
	deploymentIDs := []string{"A", "B", "C", "D", "E"}
	names := []string{"s1", "s2", "s3--0", "s3", "s3"}
	engine := NewEngine(env)
	statements := make([]*Statement, len(names))
	for index := range names {
		name := names[index]
		deployment, deployErr := engine.Deploy(context.Background(), base,
			WithDeploymentID(deploymentIDs[index]),
			WithDeploymentStatementNameResolver(func(DeploymentStatementNameContext) (string, error) {
				return name, nil
			}),
		)
		if deployErr != nil {
			t.Fatal(deployErr)
		}
		statements[index] = deployment.Statements()[0]
	}
	for index := range statements {
		resolved, ok := engine.Statement(deploymentIDs[index], names[index])
		if !ok || resolved != statements[index] {
			t.Fatalf("statement lookup %s/%s = %p, %t; want %p", deploymentIDs[index], names[index], resolved, ok, statements[index])
		}
	}

	trimmedPlan, err := env.Build(
		From[clientDeployResultEvent](env, "SupportBean").Query(StatementName(" stmt0  ")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if trimmedPlan.Query().Name() != "stmt0" {
		t.Fatalf("trimmed Plan statement name = %q", trimmedPlan.Query().Name())
	}
	trimmedDeployment, err := engine.Deploy(context.Background(), trimmedPlan, WithDeploymentID("TRIM"))
	if err != nil {
		t.Fatal(err)
	}
	if statement, ok := engine.Statement(" TRIM ", " stmt0 "); !ok || statement != trimmedDeployment.Statements()[0] {
		t.Fatalf("trimmed statement lookup = %p, %t", statement, ok)
	}
	for _, lookup := range []struct {
		deploymentID string
		name         string
	}{
		{deploymentID: "", name: ""},
		{deploymentID: "x", name: ""},
		{deploymentID: "x", name: "y"},
		{deploymentID: "TRIM", name: "y"},
		{deploymentID: "x", name: "stmt0"},
	} {
		if statement, ok := engine.Statement(lookup.deploymentID, lookup.name); ok || statement != nil {
			t.Fatalf("unknown statement lookup %q/%q = %p, %t", lookup.deploymentID, lookup.name, statement, ok)
		}
	}
}

func TestClientDeploySameDeploymentIDMatchesEsper(t *testing.T) {
	env := newClientDeployResultEnvironment(t)
	plan, err := env.Build(From[clientDeployResultEvent](env, "SupportBean").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	first, err := engine.Deploy(context.Background(), plan, WithDeploymentID("ABC"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan, WithDeploymentID("ABC")); err == nil ||
		!errors.Is(err, ErrorDeployment) || !strings.Contains(err.Error(), "ABC") {
		t.Fatalf("duplicate deployment error = %v", err)
	}
	if deployments := engine.Deployments(); !reflect.DeepEqual(deployments, []*Deployment{first}) {
		t.Fatalf("duplicate deployment changed active set = %#v", deployments)
	}
}

func TestDeploymentDependenciesFollowActiveModuleUses(t *testing.T) {
	env := newClientDeployResultEnvironment(t)
	moduleA, err := env.RegisterModule("deploy.result.a")
	if err != nil {
		t.Fatal(err)
	}
	moduleB, err := env.RegisterModule("deploy.result.b", WithModuleUses("deploy.result.a"))
	if err != nil {
		t.Fatal(err)
	}
	source := From[clientDeployResultEvent](env, "SupportBean")
	planA, err := moduleA.Build(source.Query(StatementName("a")))
	if err != nil {
		t.Fatal(err)
	}
	planB, err := moduleB.Build(source.Query(StatementName("b")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deploymentA, err := engine.Deploy(context.Background(), planA, WithDeploymentID("module-a"))
	if err != nil {
		t.Fatal(err)
	}
	deploymentB, err := engine.Deploy(context.Background(), planB, WithDeploymentID("module-b"))
	if err != nil {
		t.Fatal(err)
	}
	if dependencies := deploymentB.Dependencies(); !reflect.DeepEqual(dependencies, []string{deploymentA.ID()}) {
		t.Fatalf("module deployment dependencies = %v, want [%s]", dependencies, deploymentA.ID())
	}
	detached := deploymentB.Dependencies()
	detached[0] = "mutated"
	if dependencies := deploymentB.Dependencies(); !reflect.DeepEqual(dependencies, []string{deploymentA.ID()}) {
		t.Fatalf("dependency snapshot was not detached: %v", dependencies)
	}
}
