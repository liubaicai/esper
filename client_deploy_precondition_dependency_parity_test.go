package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// assertDeployPathPrecondition pins the deploy-time precondition contract
// Esper raises when a consumer is deployed while the module that provides a
// referenced catalog object has no active deployment: the error carries the
// object kind, logical name and owning module, and the failed deploy leaves
// the active deployment set unchanged.
func assertDeployPathPrecondition(t *testing.T, engine *Engine, plan Plan, kind DeploymentResourceKind, name, moduleName string) {
	t.Helper()
	before := len(engine.Deployments())
	if _, err := engine.Deploy(context.Background(), plan); err == nil {
		t.Fatalf("deploy without provider succeeded; want %s %q precondition", kind.dependencyLabel(), name)
	} else {
		if !errors.Is(err, ErrorDependency) {
			t.Fatalf("deploy precondition = %v", err)
		}
		var precondition *DeployPreconditionError
		if !errors.As(err, &precondition) {
			t.Fatalf("deploy precondition type = %T (%v)", err, err)
		}
		if precondition.Kind != kind || precondition.Name != name || precondition.ModuleName != moduleName || precondition.RolloutItemIndex != -1 {
			t.Fatalf("deploy precondition details = %#v", precondition)
		}
		want := "A precondition is not satisfied: Required dependency " + kind.dependencyLabel() + " '" + name + "' module '" + moduleName + "' cannot be found"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("deploy precondition message = %q, want substring %q", err.Error(), want)
		}
	}
	if got := len(engine.Deployments()); got != before {
		t.Fatalf("failed deploy changed active deployments = %d, want %d", got, before)
	}
}

// deployPreconditionConsumer builds the module-less consumer plan. The
// deployment has no module of its own, so every module-owned reference it
// carries must be provided by another active deployment.
func deployPreconditionConsumerPlan(t *testing.T, env *Environment, query Query) Plan {
	t.Helper()
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func deployPreconditionPlan(t *testing.T, engine *Engine, plan Plan, deploymentID string) *Deployment {
	t.Helper()
	deployment, err := engine.Deploy(context.Background(), plan, WithDeploymentID(deploymentID))
	if err != nil {
		t.Fatal(err)
	}
	return deployment
}

func TestClientDeployPreconditionDepNamedWindowMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	module, err := env.RegisterModule("deploy-precondition-named-window", PublicModule())
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

	consumer := deployPreconditionConsumerPlan(t, env,
		FromNamedWindowInModule(env, module.Name(), "SimpleWindow").Query(StatementName("consumer")))

	// Rollout surfaces the same precondition wrapped with the failing item
	// index, matching EPDeployPreconditionException's rollout item number.
	if _, err := engine.Rollout(context.Background(), RolloutPlans(consumer)); err == nil {
		t.Fatal("rollout without provider succeeded")
	} else {
		var rolloutFailure *DeploymentRolloutError
		if !errors.As(err, &rolloutFailure) || rolloutFailure.ItemIndex != 0 {
			t.Fatalf("rollout failure = %v", err)
		}
		var precondition *DeployPreconditionError
		if !errors.As(err, &precondition) || precondition.Kind != DeploymentResourceNamedWindow {
			t.Fatalf("rollout precondition = %v", err)
		}
	}
	assertDeployPathPrecondition(t, engine, consumer, DeploymentResourceNamedWindow, "SimpleWindow", module.Name())

	deployUndeployResourceProvider(t, engine, module, "provider")
	deployPreconditionPlan(t, engine, consumer, "consumer")
	if got := len(engine.Deployments()); got != 2 {
		t.Fatalf("active deployments after ordered deploy = %d, want 2", got)
	}
	if err := engine.Undeploy(context.Background(), "consumer"); err != nil {
		t.Fatal(err)
	}
	if err := engine.Undeploy(context.Background(), "provider"); err != nil {
		t.Fatal(err)
	}
}

func TestClientDeployPreconditionDepTableMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	module, err := env.RegisterModule("deploy-precondition-table", PublicModule())
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

	consumer := deployPreconditionConsumerPlan(t, env,
		Select(From[clientDeployUndeployResourceEvent](env, "SupportBean"),
			Alias("row", SubqueryValue[any](FromTableInModule(env, module.Name(), "SimpleTable"), EventValue[any](),
				Equal[string](Field[any, string]("col1"), Literal("a"))))).Query(StatementName("consumer")))
	assertDeployPathPrecondition(t, engine, consumer, DeploymentResourceTable, "SimpleTable", module.Name())

	deployUndeployResourceProvider(t, engine, module, "provider")
	deployPreconditionPlan(t, engine, consumer, "consumer")
}

func TestClientDeployPreconditionDepVariablePathMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	module, err := env.RegisterModule("deploy-precondition-variable", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if err := module.RegisterVariable("somevariable", "a"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	consumer := deployPreconditionConsumerPlan(t, env,
		Select(From[clientDeployUndeployResourceEvent](env, "SupportBean"),
			Alias("v", ModuleVariableRef[string](module, "somevariable"))).Query(StatementName("consumer")))
	assertDeployPathPrecondition(t, engine, consumer, DeploymentResourceVariable, "somevariable", module.Name())

	deployUndeployResourceProvider(t, engine, module, "provider")
	deployPreconditionPlan(t, engine, consumer, "consumer")
}

func TestClientDeployPreconditionDepExprDeclMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	module, err := env.RegisterModule("deploy-precondition-expression", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if err := module.DefineExpression("someexpression", Literal(0)); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	consumer := deployPreconditionConsumerPlan(t, env,
		Select(From[clientDeployUndeployResourceEvent](env, "SupportBean"),
			Alias("col", ModuleExpressionRef[int](module, "someexpression"))).Query(StatementName("consumer")))
	assertDeployPathPrecondition(t, engine, consumer, DeploymentResourceExpression, "someexpression", module.Name())

	deployUndeployResourceProvider(t, engine, module, "provider")
	deployPreconditionPlan(t, engine, consumer, "consumer")
}

func TestClientDeployPreconditionDepScriptMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	module, err := env.RegisterModule("deploy-precondition-script", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	// Java's "create expression double myscript(stringvalue) [0]" is a Go
	// script provider registered under the module-local script identity. The
	// deploy precondition names the plain script name, matching Esper's
	// NameAndModule rendering for path scripts.
	if err := RegisterModuleScript[float64](module, "myscript", "js", func(ctx ScriptContext) (Value, error) {
		return Present(0.0), nil
	}, ScriptArgumentTypes(reflect.TypeOf(""))); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	consumer := deployPreconditionConsumerPlan(t, env,
		Select(From[clientDeployUndeployResourceEvent](env, "SupportBean"),
			Alias("col", ScriptCall[float64](env, module.QualifiedName("myscript"), Literal("abc")))).Query(StatementName("consumer")))
	assertDeployPathPrecondition(t, engine, consumer, DeploymentResourceScript, "myscript", module.Name())

	deployUndeployResourceProvider(t, engine, module, "provider")
	deployPreconditionPlan(t, engine, consumer, "consumer")
}

func TestClientDeployPreconditionDepContextMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	module, err := env.RegisterModule("deploy-precondition-context", PublicModule())
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

	consumer := deployPreconditionConsumerPlan(t, env,
		From[clientDeployUndeployResourceEvent](env, "SupportBean").
			Aggregate(Alias("c", CountAll())).
			Query(StatementName("consumer"), WithContext(module.Context("MyContext"))))
	assertDeployPathPrecondition(t, engine, consumer, DeploymentResourceContext, "MyContext", module.Name())

	deployUndeployResourceProvider(t, engine, module, "provider")
	deployPreconditionPlan(t, engine, consumer, "consumer")
}

func TestClientDeployPreconditionDepEventTypePathMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	module, err := env.RegisterModule("deploy-precondition-event-type", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterMap("MySchema", []FieldSpec{
		FieldDef("col1", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// Java's consumer is "insert into MySchema select 'a' as col1 from
	// SupportBean"; the Go chain keeps the same insert-into dependency.
	consumer := deployPreconditionConsumerPlan(t, env,
		Select(From[clientDeployUndeployResourceEvent](env, "SupportBean"),
			Alias("col1", Literal("a"))).InsertInto(module.EventType("MySchema"), StatementName("consumer")))
	assertDeployPathPrecondition(t, engine, consumer, DeploymentResourceEventType, "MySchema", module.Name())

	deployUndeployResourceProvider(t, engine, module, "provider")
	deployPreconditionPlan(t, engine, consumer, "consumer")
}

func TestClientDeployPreconditionDepNamedWindowOfNamedModuleMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema is missing")
	}
	// Java: "module ABC; create window ..." is private and deployed, while
	// "module DEF; @public create window ..." is compiled to the path but not
	// deployed; the consumer resolved through DEF fails naming module DEF.
	moduleABC, err := env.RegisterModule("ABC")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := moduleABC.RegisterNamedWindow("MyWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	moduleDEF, err := env.RegisterModule("DEF", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := moduleDEF.RegisterNamedWindow("MyWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	window, err := env.Uses(moduleDEF).NamedWindow("MyWindow")
	if err != nil {
		t.Fatal(err)
	}
	consumer := deployPreconditionConsumerPlan(t, env, window.Query(StatementName("consumer")))

	deployUndeployResourceProvider(t, engine, moduleABC, "abc-provider")
	// The active ABC deployment must not satisfy the DEF-owned window.
	assertDeployPathPrecondition(t, engine, consumer, DeploymentResourceNamedWindow, "MyWindow", "DEF")

	deployUndeployResourceProvider(t, engine, moduleDEF, "def-provider")
	deployPreconditionPlan(t, engine, consumer, "consumer")
}

// TestClientDeployPreconditionDepIndexDisposition fixes the approved
// difference for Java's create-index deployments: Go declares indexes as
// table/named-window definition options at registration time, so an index
// always travels with its owning infrastructure. The observable deploy
// dependency stays on the owner: without the provider the index-hinted
// consumer fails with the Table precondition, and once the provider is
// active the definition's indexes are available immediately.
func TestClientDeployPreconditionDepIndexDisposition(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	module, err := env.RegisterModule("deploy-precondition-index", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterTable("MyTable", []TableColumn{
		PrimaryKeyColumn[string]("col1"),
		TableColumnOf[string]("col2"),
	}, SecondaryIndex("MyIndexForTable", "col2")); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// Java's consumer joins SupportBean to MyTable on col2 through the
	// not-yet-deployed MyIndexForTable; the Go chain keeps the same
	// index-selected table access as an index-hinted subquery.
	consumer := deployPreconditionConsumerPlan(t, env,
		Select(From[clientDeployUndeployResourceEvent](env, "SupportBean"),
			Alias("row", SubqueryValueWithOptions[any](FromTableInModule(env, module.Name(), "MyTable"), EventValue[any](),
				SubqueryWhere(Equal[string](Field[any, string]("col2"), Literal("a"))),
				SubqueryUseIndex("MyIndexForTable")))).Query(StatementName("consumer")))
	assertDeployPathPrecondition(t, engine, consumer, DeploymentResourceTable, "MyTable", module.Name())

	deployUndeployResourceProvider(t, engine, module, "provider")
	deployPreconditionPlan(t, engine, consumer, "consumer")
}

// TestClientDeployPreconditionDepClassDisposition fixes the approved
// difference for Java's application-inlined classes: Go has no
// runtime-compiled class artifact, so a callable equivalent is statically
// linked into the Plan and carries no deployment dependency at all.
func TestClientDeployPreconditionDepClassDisposition(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// Java's "MyClass.doIt()" is a statically linked Go UDF here: nothing is
	// compiled to a path and no provider deployment exists to require.
	consumer := deployPreconditionConsumerPlan(t, env,
		Select(From[clientDeployUndeployResourceEvent](env, "SupportBean"),
			Alias("col", Func1("MyClass.doIt", func(string) string { return "def" }, Literal("abc")))).Query(StatementName("consumer")))
	deployment := deployPreconditionPlan(t, engine, consumer, "consumer")

	var values []string
	statement, ok := deployment.Statement("consumer")
	if !ok {
		t.Fatal("consumer statement is missing")
	}
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			values = append(values, row.Get("col").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientDeployUndeployResourceEvent{TheString: "event"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values, []string{"def"}) {
		t.Fatalf("inlined-class equivalent value = %v", values)
	}
}

// TestClientDeployPreconditionDepVariablePreconfigDisposition fixes the
// approved difference for Java's pre-configured variable precondition. The
// Go chain API has no compile-configuration/runtime-configuration skew:
// env.Build is the compiler and validates preconfigured variables against
// the same environment the engine serves, and plan ownership rejects
// deploying a plan into an engine of a different environment. The deploy-time
// failure Java raises is therefore enforced at the Go compile boundary.
func TestClientDeployPreconditionDepVariablePreconfigDisposition(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	if _, err := env.Build(Select(From[clientDeployUndeployResourceEvent](env, "SupportBean"),
		Alias("v", VariableRef[string]("mypublicvariable"))).Query(StatementName("consumer"))); err == nil ||
		!errors.Is(err, ErrorUnknownName) || !strings.Contains(err.Error(), "unknown variable \"mypublicvariable\"") {
		t.Fatalf("unregistered pre-configured variable build error = %v", err)
	}

	if err := env.RegisterVariable("mypublicvariable", "a"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(From[clientDeployUndeployResourceEvent](env, "SupportBean"),
		Alias("v", VariableRef[string]("mypublicvariable"))).Query(StatementName("consumer")))
	if err != nil {
		t.Fatal(err)
	}
	deployPreconditionPlan(t, engine, plan, "consumer")

	// Java compiles against a deep-copied configuration and deploys into the
	// unmodified runtime; Go's plan ownership makes that skew a deploy error.
	other := newUndeployResourceEnvironment(t)
	if _, err := NewEngine(other).Deploy(context.Background(), plan); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("cross-environment deploy error = %v", err)
	}
}

// TestClientDeployPreconditionDepEventTypePreconfigDisposition fixes the
// approved difference for Java's pre-configured event-type precondition, for
// the same reason as the variable case: the Go compile boundary validates
// that every referenced event type is registered in the serving environment.
func TestClientDeployPreconditionDepEventTypePreconfigDisposition(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	if _, err := env.Build(FromAny(env, "SomeEvent").Query(StatementName("consumer"))); err == nil ||
		!strings.Contains(err.Error(), "\"SomeEvent\" has no registered schema") {
		t.Fatalf("unregistered pre-configured event type build error = %v", err)
	}

	if _, err := RegisterMap(env, "SomeEvent", []FieldSpec{FieldDef("col1", reflect.TypeOf(""))}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "SomeEvent").Query(StatementName("consumer")))
	if err != nil {
		t.Fatal(err)
	}
	deployPreconditionPlan(t, engine, plan, "consumer")

	other := newUndeployResourceEnvironment(t)
	if _, err := NewEngine(other).Deploy(context.Background(), plan); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("cross-environment deploy error = %v", err)
	}
}
