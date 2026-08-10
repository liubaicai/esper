package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// assertDuplicateModuleObject pins the Go registration-time equivalent of
// Esper's deploy-time PathExceptionAlreadyRegistered surfaced as
// EPDeployPreconditionException: Go materializes EPL create statements as
// Environment/Module registrations, so the duplicate precondition is enforced
// at the registration boundary with Esper's final message text. Java's
// rolloutItemNumber is always -1 in this suite and has no registration-time
// counterpart.
func assertDuplicateModuleObject(t *testing.T, err error, kind DeploymentResourceKind, name, moduleName string) {
	t.Helper()
	if err == nil {
		t.Fatalf("duplicate %s %q registration succeeded", kind.dependencyLabel(), name)
	}
	if !errors.Is(err, ErrorDependency) {
		t.Fatalf("duplicate registration = %v, want ErrorDependency", err)
	}
	var dup *DuplicateModuleObjectError
	if !errors.As(err, &dup) {
		t.Fatalf("duplicate registration type = %T (%v)", err, err)
	}
	if dup.Kind != kind || dup.Name != name || dup.ModuleName != moduleName {
		t.Fatalf("duplicate registration details = %#v", dup)
	}
	rendered := moduleName
	if rendered == "" {
		rendered = "unnamed"
	}
	want := "A precondition is not satisfied: " + kind.duplicateLabel() + " by name '" + name + "' has already been created for module '" + rendered + "'"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("duplicate registration message = %q, want substring %q", err.Error(), want)
	}
}

func registerDuplicateTestModule(t *testing.T, env *Environment, name string) Module {
	t.Helper()
	module, err := env.RegisterModule(name, PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	return module
}

// TestClientDeployPrecondDupNamedWindowMatchesEsper covers
// ClientDeployPrecondDupNamedWindow: Java deploys "@public create window
// SimpleWindow#keepall as SupportBean" twice (unnamed module, then module
// ABC) and fails the second deploy. Go registers the window once per catalog
// identity, so the second registration reports the same precondition text.
func TestClientDeployPrecondDupNamedWindowMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema is missing")
	}

	if _, err := env.RegisterNamedWindow("SimpleWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	_, err := env.RegisterNamedWindow("SimpleWindow", schema, NamedWindowRetention(KeepAll()))
	assertDuplicateModuleObject(t, err, DeploymentResourceNamedWindow, "SimpleWindow", "")

	moduleABC := registerDuplicateTestModule(t, env, "ABC")
	if _, err := moduleABC.RegisterNamedWindow("SimpleWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	_, err = moduleABC.RegisterNamedWindow("SimpleWindow", schema, NamedWindowRetention(KeepAll()))
	assertDuplicateModuleObject(t, err, DeploymentResourceNamedWindow, "SimpleWindow", "ABC")

	// Java's path registry keys entries per module: a different module exposes
	// the same logical name without a duplicate conflict, and resolution then
	// follows the explicit module path.
	moduleDEF := registerDuplicateTestModule(t, env, "DEF")
	if _, err := moduleDEF.RegisterNamedWindow("SimpleWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatalf("different-module same-name registration = %v", err)
	}
	window, err := moduleABC.Path().NamedWindow("SimpleWindow")
	if err != nil || window.node == nil || window.node.moduleName != "ABC" {
		t.Fatalf("provider window resolution = %#v (%v)", window.node, err)
	}

	// The rejected duplicates leave the provider catalog intact: the provider
	// module deploys and a cross-module consumer passes the deploy-time path
	// precondition.
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployUndeployResourceProvider(t, engine, moduleABC, "provider")
	deployUndeployResourceConsumer(t, env, engine,
		FromNamedWindowInModule(env, "ABC", "SimpleWindow").Query(StatementName("consumer")), "consumer")
}

// TestClientDeployPrecondDupTableMatchesEsper covers
// ClientDeployPrecondDupTable for the unnamed module.
func TestClientDeployPrecondDupTableMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	if _, err := env.RegisterTable("SimpleTable", []TableColumn{TableColumnOf[string]("col1")}); err != nil {
		t.Fatal(err)
	}
	_, err := env.RegisterTable("SimpleTable", []TableColumn{TableColumnOf[string]("col1")})
	assertDuplicateModuleObject(t, err, DeploymentResourceTable, "SimpleTable", "")

	if _, ok := env.Table("SimpleTable"); !ok {
		t.Fatal("provider table registration was lost after duplicate rejection")
	}
}

// TestClientDeployPrecondDupEventTypeMatchesEsper covers
// ClientDeployPrecondDupEventType: Java's "@public create schema MySchema
// (col1 string)" deployed twice.
func TestClientDeployPrecondDupEventTypeMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	if _, err := RegisterMap(env, "MySchema", []FieldSpec{FieldDef("col1", reflect.TypeOf(""))}); err != nil {
		t.Fatal(err)
	}
	_, err := RegisterMap(env, "MySchema", []FieldSpec{FieldDef("col1", reflect.TypeOf(""))})
	assertDuplicateModuleObject(t, err, DeploymentResourceEventType, "MySchema", "")

	moduleABC := registerDuplicateTestModule(t, env, "ABC")
	if _, err := moduleABC.RegisterMap("MySchema", []FieldSpec{FieldDef("col1", reflect.TypeOf(""))}); err != nil {
		t.Fatal(err)
	}
	_, err = moduleABC.RegisterMap("MySchema", []FieldSpec{FieldDef("col1", reflect.TypeOf(""))})
	assertDuplicateModuleObject(t, err, DeploymentResourceEventType, "MySchema", "ABC")

	if _, ok := env.Schema("MySchema"); !ok {
		t.Fatal("provider schema registration was lost after duplicate rejection")
	}
	if _, err := env.Build(FromAny(env, "MySchema").Query(StatementName("consumer"))); err != nil {
		t.Fatalf("provider schema no longer builds: %v", err)
	}
}

// TestClientDeployPrecondDupVariableMatchesEsper covers
// ClientDeployPrecondDupVariable: Java's "@public create variable string
// myvariable" deployed twice.
func TestClientDeployPrecondDupVariableMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	if err := env.RegisterVariable("myvariable", "a"); err != nil {
		t.Fatal(err)
	}
	assertDuplicateModuleObject(t, env.RegisterVariable("myvariable", "a"), DeploymentResourceVariable, "myvariable", "")

	moduleABC := registerDuplicateTestModule(t, env, "ABC")
	if err := moduleABC.RegisterVariable("myvariable", "a"); err != nil {
		t.Fatal(err)
	}
	assertDuplicateModuleObject(t, moduleABC.RegisterVariable("myvariable", "a"), DeploymentResourceVariable, "myvariable", "ABC")

	// The rejected duplicate leaves the original variable value untouched.
	definition, ok := env.Variable("myvariable")
	if !ok {
		t.Fatal("provider variable registration was lost after duplicate rejection")
	}
	if got, want := definition.Initial().Any(), "a"; got != want {
		t.Fatalf("provider variable initial value = %v, want %v", got, want)
	}
}

// TestClientDeployPrecondDupExprDeclMatchesEsper covers
// ClientDeployPrecondDupExprDecl: Java's "@public create expression expr_one
// {0}" deployed twice.
func TestClientDeployPrecondDupExprDeclMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	if err := env.DefineExpression("expr_one", Literal(0)); err != nil {
		t.Fatal(err)
	}
	assertDuplicateModuleObject(t, env.DefineExpression("expr_one", Literal(0)), DeploymentResourceExpression, "expr_one", "")

	moduleABC := registerDuplicateTestModule(t, env, "ABC")
	if err := moduleABC.DefineExpression("expr_one", Literal(0)); err != nil {
		t.Fatal(err)
	}
	assertDuplicateModuleObject(t, moduleABC.DefineExpression("expr_one", Literal(0)), DeploymentResourceExpression, "expr_one", "ABC")

	if _, ok := env.Expression("expr_one"); !ok {
		t.Fatal("provider expression registration was lost after duplicate rejection")
	}
}

// TestClientDeployPrecondDupScriptMatchesEsper covers
// ClientDeployPrecondDupScript: Java's "@public create expression double
// myscript(stringvalue) [0]" deployed twice. The registered argument-types
// metadata renders Esper's NameAndParamNum identity form.
func TestClientDeployPrecondDupScriptMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	provider := func(ScriptContext) (float64, error) { return 0, nil }
	if err := RegisterScript[float64](env, "myscript", "go", provider, ScriptArgumentTypes(reflect.TypeOf(""))); err != nil {
		t.Fatal(err)
	}
	assertDuplicateModuleObject(t,
		RegisterScript[float64](env, "myscript", "go", provider, ScriptArgumentTypes(reflect.TypeOf(""))),
		DeploymentResourceScript, "myscript (1 parameters)", "")

	moduleABC := registerDuplicateTestModule(t, env, "ABC")
	if err := RegisterModuleScript[float64](moduleABC, "myscript", "go", provider, ScriptArgumentTypes(reflect.TypeOf(""))); err != nil {
		t.Fatal(err)
	}
	assertDuplicateModuleObject(t,
		RegisterModuleScript[float64](moduleABC, "myscript", "go", provider, ScriptArgumentTypes(reflect.TypeOf(""))),
		DeploymentResourceScript, "myscript (1 parameters)", "ABC")

	// A script without declared argument types keeps the plain logical name.
	if err := RegisterScript[float64](env, "untyped-script", "go", provider); err != nil {
		t.Fatal(err)
	}
	assertDuplicateModuleObject(t, RegisterScript[float64](env, "untyped-script", "go", provider),
		DeploymentResourceScript, "untyped-script", "")
}

// TestClientDeployPrecondDupContextMatchesEsper covers
// ClientDeployPrecondDupContext: Java's "@public create context MyContext as
// partition by theString from SupportBean" deployed twice.
func TestClientDeployPrecondDupContextMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	if _, err := env.RegisterContext("MyContext", Field[clientDeployUndeployResourceEvent, string]("theString")); err != nil {
		t.Fatal(err)
	}
	_, err := env.RegisterContext("MyContext", Field[clientDeployUndeployResourceEvent, string]("theString"))
	assertDuplicateModuleObject(t, err, DeploymentResourceContext, "MyContext", "")

	moduleABC := registerDuplicateTestModule(t, env, "ABC")
	definition, err := NewKeyContext("ignored", Field[clientDeployUndeployResourceEvent, string]("theString"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := moduleABC.RegisterContextDefinition("MyContext", definition); err != nil {
		t.Fatal(err)
	}
	_, err = moduleABC.RegisterContextDefinition("MyContext", definition)
	assertDuplicateModuleObject(t, err, DeploymentResourceContext, "MyContext", "ABC")

	if _, ok := env.Context("MyContext"); !ok {
		t.Fatal("provider context registration was lost after duplicate rejection")
	}
}

// TestClientDeployPrecondDupIndexDisposition fixes the approved difference
// for ClientDeployPrecondDupIndex: Java tracks create-index statements as
// separately deployed path objects and rejects a duplicate index name on the
// same infra at deploy time. Go declares indexes as registration-time
// definition options of their owning Table/Named Window, so the duplicate
// precondition is enforced when the definition is built, and index names stay
// scoped to their owning infra.
func TestClientDeployPrecondDupIndexDisposition(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	columns := []TableColumn{PrimaryKeyColumn[string]("col1"), TableColumnOf[string]("col2")}
	if _, err := NewTableDefinition("MyTable", columns,
		SecondaryIndex("MyIndexOnTable", "col2"), SecondaryIndex("MyIndexOnTable", "col1")); err == nil ||
		!strings.Contains(err.Error(), "table duplicates index \"MyIndexOnTable\"") {
		t.Fatalf("duplicate table index definition error = %v", err)
	}

	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema is missing")
	}
	if _, err := NewNamedWindowDefinition("MyWindow", schema,
		NamedWindowIndex("MyIndexOnNW", "theString"), NamedWindowIndex("MyIndexOnNW", "theString")); err == nil ||
		!strings.Contains(err.Error(), "named-window duplicates index \"MyIndexOnNW\"") {
		t.Fatalf("duplicate named-window index definition error = %v", err)
	}

	// The same index name on a different infra is an independent definition.
	if _, err := env.RegisterTable("MyTable", columns, SecondaryIndex("MyIndexOnTable", "col2")); err != nil {
		t.Fatal(err)
	}
	if _, err := env.RegisterTable("OtherTable", columns, SecondaryIndex("MyIndexOnTable", "col2")); err != nil {
		t.Fatalf("same index name on a second table = %v", err)
	}
	if _, err := env.RegisterNamedWindow("MyWindow", schema, NamedWindowIndex("MyIndexOnNW", "theString")); err != nil {
		t.Fatal(err)
	}
	if _, err := env.RegisterNamedWindow("OtherWindow", schema, NamedWindowIndex("MyIndexOnNW", "theString")); err != nil {
		t.Fatalf("same index name on a second named window = %v", err)
	}
}

// TestClientDeployPrecondDupClassDisposition fixes the approved difference
// for ClientDeployPrecondDupClass: Java's application-inlined class is a JVM
// deployment artifact tracked in its own path registry. Go has no
// runtime-compiled class artifact; the callable equivalent is a statically
// linked module expression/UDF whose duplicate registration is the declared
// expression precondition, and a module without class artifacts deploys and
// undeploys cleanly.
func TestClientDeployPrecondDupClassDisposition(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	moduleABC := registerDuplicateTestModule(t, env, "ABC")

	// Java's "MyClass" with its static doIt() is represented by an analyzable
	// module-scoped declared expression.
	if err := moduleABC.DefineExpression("MyClass.doIt", Literal("def")); err != nil {
		t.Fatal(err)
	}
	assertDuplicateModuleObject(t, moduleABC.DefineExpression("MyClass.doIt", Literal("def")),
		DeploymentResourceExpression, "MyClass.doIt", "ABC")

	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment := deployUndeployResourceProvider(t, engine, moduleABC, "provider")
	if err := engine.Undeploy(context.Background(), deployment.ID()); err != nil {
		t.Fatalf("module without class artifacts failed to undeploy: %v", err)
	}
}
