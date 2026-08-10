package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
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

// assertUndeployResourcePrecondition pins the resource-level precondition
// contract Esper raises for module-owned catalog objects: the error carries
// the provider deployment, the earliest external dependent and the blocking
// resource, and neither side changes state.
func assertUndeployResourcePrecondition(t *testing.T, engine *Engine, deploymentID, referencedBy string, kind DeploymentResourceKind, resourceName string) {
	t.Helper()
	err := engine.Undeploy(context.Background(), deploymentID)
	if err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("undeploy %s resource precondition = %v", deploymentID, err)
	}
	var precondition *UndeployPreconditionError
	if !errors.As(err, &precondition) || precondition.DeploymentID != deploymentID || precondition.ReferencedBy != referencedBy {
		t.Fatalf("undeploy %s resource precondition details = %#v", deploymentID, precondition)
	}
	if precondition.Resource == nil || precondition.Resource.Kind != kind || precondition.Resource.Name != resourceName {
		t.Fatalf("undeploy %s resource = %#v, want kind %d name %q", deploymentID, precondition.Resource, kind, resourceName)
	}
	if !strings.Contains(err.Error(), kind.label()+" \""+resourceName+"\" cannot be un-deployed as it is referenced by deployment \""+referencedBy+"\"") {
		t.Fatalf("undeploy %s resource message = %q", deploymentID, err.Error())
	}
	if !engine.IsDeployed(deploymentID) || !engine.IsDeployed(referencedBy) {
		t.Fatalf("failed undeploy changed active deployments: %s=%v %s=%v", deploymentID, engine.IsDeployed(deploymentID), referencedBy, engine.IsDeployed(referencedBy))
	}
}

type clientDeployUndeployResourceEvent struct {
	TheString string `esper:"theString"`
}

func newUndeployResourceEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientDeployUndeployResourceEvent](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

// deployUndeployResourceProvider deploys one pass-through statement of the
// provider module. Module-granular provision makes the deployment the owner
// of every catalog object registered in that module, mirroring Java deploying
// the module that contains the @public create statements.
func deployUndeployResourceProvider(t *testing.T, engine *Engine, module Module, deploymentID string) *Deployment {
	t.Helper()
	plan, err := module.Build(
		Select(From[clientDeployUndeployResourceEvent](module.env, "SupportBean"), Alias("theString", Field[clientDeployUndeployResourceEvent, string]("theString"))).Query(StatementName("infra")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan, WithDeploymentID(deploymentID))
	if err != nil {
		t.Fatal(err)
	}
	return deployment
}

func deployUndeployResourceConsumer(t *testing.T, env *Environment, engine *Engine, query Query, deploymentID string) *Deployment {
	t.Helper()
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan, WithDeploymentID(deploymentID))
	if err != nil {
		t.Fatal(err)
	}
	return deployment
}

func TestClientUndeployPrecondDepNamedWindowMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	module, err := env.RegisterModule("undeploy.named-window", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema is missing")
	}
	if _, err := module.RegisterNamedWindow("SimpleWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployUndeployResourceProvider(t, engine, module, "provider")

	consumerA := FromNamedWindowInModule(env, module.Name(), "SimpleWindow").Query(StatementName("A"))
	deployUndeployResourceConsumer(t, env, engine, consumerA, "consumer-a")
	assertUndeployResourcePrecondition(t, engine, "provider", "consumer-a", DeploymentResourceNamedWindow, "SimpleWindow")
	if err := engine.Undeploy(context.Background(), "consumer-a"); err != nil {
		t.Fatal(err)
	}

	consumerB := Select(From[clientDeployUndeployResourceEvent](env, "SupportBean"),
		Alias("w", SubqueryValue[any](FromNamedWindowInModule(env, module.Name(), "SimpleWindow"), EventValue[any]()))).Query(StatementName("B"))
	deployUndeployResourceConsumer(t, env, engine, consumerB, "consumer-b")
	assertUndeployResourcePrecondition(t, engine, "provider", "consumer-b", DeploymentResourceNamedWindow, "SimpleWindow")
	if err := engine.Undeploy(context.Background(), "consumer-b"); err != nil {
		t.Fatal(err)
	}

	if err := engine.Undeploy(context.Background(), "provider"); err != nil {
		t.Fatalf("provider undeploy after dependents removed: %v", err)
	}
	if got := engine.Deployments(); len(got) != 0 {
		t.Fatalf("deployments remain active = %#v", got)
	}
}

func TestClientUndeployPrecondDepTableMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	module, err := env.RegisterModule("undeploy.table", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterTable("SimpleTable", []TableColumn{
		PrimaryKeyColumn[string]("col1"),
		TableColumnOf[string]("col2"),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployUndeployResourceProvider(t, engine, module, "provider")

	// Java's SimpleTable['a'] accessor is a primary-key table subquery in the
	// Go chain; the undeploy dependency on the table is identical.
	consumerA := Select(From[clientDeployUndeployResourceEvent](env, "SupportBean"),
		Alias("row", SubqueryValue[any](FromTableInModule(env, module.Name(), "SimpleTable"), EventValue[any](),
			Equal[string](Field[any, string]("col1"), Literal("a"))))).Query(StatementName("A"))
	deployUndeployResourceConsumer(t, env, engine, consumerA, "consumer-a")
	assertUndeployResourcePrecondition(t, engine, "provider", "consumer-a", DeploymentResourceTable, "SimpleTable")
	if err := engine.Undeploy(context.Background(), "consumer-a"); err != nil {
		t.Fatal(err)
	}

	consumerB := Select(From[clientDeployUndeployResourceEvent](env, "SupportBean"),
		Alias("row", SubqueryValue[any](FromTableInModule(env, module.Name(), "SimpleTable"), EventValue[any]()))).Query(StatementName("B"))
	deployUndeployResourceConsumer(t, env, engine, consumerB, "consumer-b")
	assertUndeployResourcePrecondition(t, engine, "provider", "consumer-b", DeploymentResourceTable, "SimpleTable")
	if err := engine.Undeploy(context.Background(), "consumer-b"); err != nil {
		t.Fatal(err)
	}

	// Java's third consumer is "create index MyIndex on SimpleTable(col2)" in
	// its own deployment. Go declares indexes as table-definition options at
	// registration time, so no separate index deployment exists to depend on
	// the table; this is an approved representation difference.
	if err := engine.Undeploy(context.Background(), "provider"); err != nil {
		t.Fatalf("provider undeploy after dependents removed: %v", err)
	}
	if got := engine.Deployments(); len(got) != 0 {
		t.Fatalf("deployments remain active = %#v", got)
	}
}

func TestClientUndeployPrecondDepVariableMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	module, err := env.RegisterModule("undeploy.variable", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if err := module.RegisterVariable("varstring", ""); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployUndeployResourceProvider(t, engine, module, "provider")

	consumerA := Select(From[clientDeployUndeployResourceEvent](env, "SupportBean"),
		Alias("v", ModuleVariableRef[string](module, "varstring"))).Query(StatementName("A"))
	deployUndeployResourceConsumer(t, env, engine, consumerA, "consumer-a")
	assertUndeployResourcePrecondition(t, engine, "provider", "consumer-a", DeploymentResourceVariable, "varstring")
	if err := engine.Undeploy(context.Background(), "consumer-a"); err != nil {
		t.Fatal(err)
	}

	consumerB := OnEvent(From[clientDeployUndeployResourceEvent](env, "SupportBean")).
		SetVariable(module.QualifiedName("varstring"), Literal("a")).Query(StatementName("B"))
	deployUndeployResourceConsumer(t, env, engine, consumerB, "consumer-b")
	assertUndeployResourcePrecondition(t, engine, "provider", "consumer-b", DeploymentResourceVariable, "varstring")
	if err := engine.Undeploy(context.Background(), "consumer-b"); err != nil {
		t.Fatal(err)
	}

	if err := engine.Undeploy(context.Background(), "provider"); err != nil {
		t.Fatalf("provider undeploy after dependents removed: %v", err)
	}
	if got := engine.Deployments(); len(got) != 0 {
		t.Fatalf("deployments remain active = %#v", got)
	}
}

func TestClientUndeployPrecondDepContextMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	module, err := env.RegisterModule("undeploy.context", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	definition, err := NewKeyContext("ignored", Field[clientDeployUndeployResourceEvent, string]("theString"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterContextDefinition("MyContext", definition); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployUndeployResourceProvider(t, engine, module, "provider")

	consumerA := From[clientDeployUndeployResourceEvent](env, "SupportBean").
		Aggregate(Alias("c", CountAll())).
		Query(StatementName("A"), WithContext(module.Context("MyContext")))
	deployUndeployResourceConsumer(t, env, engine, consumerA, "consumer-a")
	assertUndeployResourcePrecondition(t, engine, "provider", "consumer-a", DeploymentResourceContext, "MyContext")
	if err := engine.Undeploy(context.Background(), "consumer-a"); err != nil {
		t.Fatal(err)
	}

	if err := engine.Undeploy(context.Background(), "provider"); err != nil {
		t.Fatalf("provider undeploy after dependents removed: %v", err)
	}
	if got := engine.Deployments(); len(got) != 0 {
		t.Fatalf("deployments remain active = %#v", got)
	}
}

func TestClientUndeployPrecondDepEventTypeMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	module, err := env.RegisterModule("undeploy.event-type", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterMap("MySchema", []FieldSpec{
		FieldDef("col", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployUndeployResourceProvider(t, engine, module, "provider")

	consumerA := Select(From[clientDeployUndeployResourceEvent](env, "SupportBean"),
		Alias("col", Literal("a"))).InsertInto(module.EventType("MySchema"), StatementName("A"))
	deployUndeployResourceConsumer(t, env, engine, consumerA, "consumer-a")
	assertUndeployResourcePrecondition(t, engine, "provider", "consumer-a", DeploymentResourceEventType, "MySchema")
	if err := engine.Undeploy(context.Background(), "consumer-a"); err != nil {
		t.Fatal(err)
	}

	consumerB := module.Stream("MySchema").Aggregate(Alias("c", CountAll())).Query(StatementName("B"))
	planB, err := env.Build(consumerB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), planB, WithDeploymentID("consumer-b")); err != nil {
		t.Fatal(err)
	}
	assertUndeployResourcePrecondition(t, engine, "provider", "consumer-b", DeploymentResourceEventType, "MySchema")
	if err := engine.Undeploy(context.Background(), "consumer-b"); err != nil {
		t.Fatal(err)
	}

	if err := engine.Undeploy(context.Background(), "provider"); err != nil {
		t.Fatalf("provider undeploy after dependents removed: %v", err)
	}
	if got := engine.Deployments(); len(got) != 0 {
		t.Fatalf("deployments remain active = %#v", got)
	}
}

func TestClientUndeployPrecondDepExprDeclMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	module, err := env.RegisterModule("undeploy.expression", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if err := module.DefineExpression("myexpression", Literal(0)); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployUndeployResourceProvider(t, engine, module, "provider")

	consumerA := Select(From[clientDeployUndeployResourceEvent](env, "SupportBean"),
		Alias("col", ModuleExpressionRef[int](module, "myexpression"))).Query(StatementName("A"))
	deployUndeployResourceConsumer(t, env, engine, consumerA, "consumer-a")
	assertUndeployResourcePrecondition(t, engine, "provider", "consumer-a", DeploymentResourceExpression, "myexpression")
	if err := engine.Undeploy(context.Background(), "consumer-a"); err != nil {
		t.Fatal(err)
	}

	inner := FromAny(env, "SupportBean").Window(KeepAll())
	consumerB := Select(From[clientDeployUndeployResourceEvent](env, "SupportBean"),
		Alias("col", SubqueryValue[int](inner, ModuleExpressionRef[int](module, "myexpression")))).Query(StatementName("B"))
	deployUndeployResourceConsumer(t, env, engine, consumerB, "consumer-b")
	assertUndeployResourcePrecondition(t, engine, "provider", "consumer-b", DeploymentResourceExpression, "myexpression")
	if err := engine.Undeploy(context.Background(), "consumer-b"); err != nil {
		t.Fatal(err)
	}

	if err := engine.Undeploy(context.Background(), "provider"); err != nil {
		t.Fatalf("provider undeploy after dependents removed: %v", err)
	}
	if got := engine.Deployments(); len(got) != 0 {
		t.Fatalf("deployments remain active = %#v", got)
	}
}
