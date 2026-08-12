package esper

import (
	"context"
	"reflect"
	"testing"
)

// assertConsumedDependencies pins the consumed-side dependency list of one
// deployment. The Go API returns items deterministically ordered by kind and
// name; Esper asserts them any-order.
func assertConsumedDependencies(t *testing.T, engine *Engine, deploymentID string, want ...DeploymentDependencyConsumedItem) {
	t.Helper()
	consumed, ok := engine.DeploymentDependenciesConsumed(deploymentID)
	if !ok {
		t.Fatalf("consumed dependencies for %q not found", deploymentID)
	}
	if len(want) == 0 {
		want = []DeploymentDependencyConsumedItem{}
	}
	if !reflect.DeepEqual(consumed.Items, want) {
		t.Fatalf("consumed(%s) = %#v, want %#v", deploymentID, consumed.Items, want)
	}
}

func assertProvidedDependencies(t *testing.T, engine *Engine, deploymentID string, want ...DeploymentDependencyProvidedItem) {
	t.Helper()
	provided, ok := engine.DeploymentDependenciesProvided(deploymentID)
	if !ok {
		t.Fatalf("provided dependencies for %q not found", deploymentID)
	}
	if len(want) == 0 {
		want = []DeploymentDependencyProvidedItem{}
	}
	if !reflect.DeepEqual(provided.Items, want) {
		t.Fatalf("provided(%s) = %#v, want %#v", deploymentID, provided.Items, want)
	}
}

// TestClientDeployListDependenciesObjectTypesMatchesEsper covers
// ClientDeployListDependenciesObjectTypes: a provider module owns every
// catalog object kind and one consumer deployment references all of them.
// Java's index and application-inlined class items have no Go dependency
// objects (indexes are registration-time definition options, classes are
// statically linked); the class equivalent rides the declared-expression edge
// and an index-hinted consumer still rides the owning table edge.
func TestClientDeployListDependenciesObjectTypesMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	module, err := env.RegisterModule("list-dependencies-object-types", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema is missing")
	}
	if _, err := module.RegisterNamedWindow("MyWindow", schema,
		NamedWindowRetention(KeepAll()), NamedWindowIndex("MyIndexA", "theString")); err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterTable("MyTable", []TableColumn{
		PrimaryKeyColumn[string]("k"), TableColumnOf[string]("value"),
	}, SecondaryIndex("MyIndexB", "value")); err != nil {
		t.Fatal(err)
	}
	if err := module.RegisterVariable("MyVariable", 0); err != nil {
		t.Fatal(err)
	}
	contextDefinition, err := NewKeyContext("ignored", Field[clientDeployUndeployResourceEvent, string]("theString"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterContextDefinition("MyContext", contextDefinition); err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterMap("MyEventType", []FieldSpec{FieldDef("col1", reflect.TypeOf(""))}); err != nil {
		t.Fatal(err)
	}
	if err := module.DefineExpression("MyExpression", Literal(0)); err != nil {
		t.Fatal(err)
	}
	// Java's application-inlined class MyClass with its static doIt() is a
	// module-scoped declared expression here.
	if err := module.DefineExpression("MyClass.doIt", Literal("abc")); err != nil {
		t.Fatal(err)
	}
	scriptProvider := func(ScriptContext) (float64, error) { return 0, nil }
	if err := RegisterModuleScript[float64](module, "MyScript", "go", scriptProvider, ScriptArgumentTypes(reflect.TypeOf(""))); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployUndeployResourceProvider(t, engine, module, "provide")

	consumerPlans := make([]Plan, 0, 5)
	build := func(query Query) {
		t.Helper()
		plan, buildErr := env.Build(query)
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		consumerPlans = append(consumerPlans, plan)
	}
	// context MyContext select MyVariable, count(*), MyTable-subquery from MyWindow
	build(FromNamedWindowInModule(env, module.Name(), "MyWindow").Select(
		Alias("v", VariableRef[int](module.QualifiedName("MyVariable"))),
		Alias("rows", CountAll()),
		Alias("tableValue", SubqueryValueWithOptions[any](FromTableInModule(env, module.Name(), "MyTable"), EventValue[any](),
			SubqueryWhere(Equal[string](Field[any, string]("value"), Literal("a"))),
			SubqueryUseIndex("MyIndexB"))),
	).Query(StatementName("consume-infra"), WithContext(module.Context("MyContext"))))
	// select MyExpression(), MyScript('a'), MyClass.doIt() from MyEventType
	build(FromAny(env, module.EventType("MyEventType")).Select(
		Alias("e", ExpressionRef[int](env, module.QualifiedName("MyExpression"))),
		Alias("s", ScriptCall[float64](env, module.QualifiedName("MyScript"), Literal("a"))),
		Alias("c", ExpressionRef[string](env, module.QualifiedName("MyClass.doIt"))),
	).Query(StatementName("consume-callables")))
	consumer, err := engine.DeployPlans(context.Background(), consumerPlans, WithDeploymentID("consume"))
	if err != nil {
		t.Fatal(err)
	}

	wantConsumed := []DeploymentDependencyConsumedItem{
		{DeploymentID: "provide", Kind: DeploymentResourceNamedWindow, Name: "MyWindow"},
		{DeploymentID: "provide", Kind: DeploymentResourceTable, Name: "MyTable"},
		{DeploymentID: "provide", Kind: DeploymentResourceVariable, Name: "MyVariable"},
		{DeploymentID: "provide", Kind: DeploymentResourceContext, Name: "MyContext"},
		{DeploymentID: "provide", Kind: DeploymentResourceEventType, Name: "MyEventType"},
		{DeploymentID: "provide", Kind: DeploymentResourceExpression, Name: "MyClass.doIt"},
		{DeploymentID: "provide", Kind: DeploymentResourceExpression, Name: "MyExpression"},
		{DeploymentID: "provide", Kind: DeploymentResourceScript, Name: "MyScript#1"},
	}
	assertConsumedDependencies(t, engine, "consume", wantConsumed...)

	wantProvided := []DeploymentDependencyProvidedItem{
		{Kind: DeploymentResourceNamedWindow, Name: "MyWindow", ConsumerIDs: []string{"consume"}},
		{Kind: DeploymentResourceTable, Name: "MyTable", ConsumerIDs: []string{"consume"}},
		{Kind: DeploymentResourceVariable, Name: "MyVariable", ConsumerIDs: []string{"consume"}},
		{Kind: DeploymentResourceContext, Name: "MyContext", ConsumerIDs: []string{"consume"}},
		{Kind: DeploymentResourceEventType, Name: "MyEventType", ConsumerIDs: []string{"consume"}},
		{Kind: DeploymentResourceExpression, Name: "MyClass.doIt", ConsumerIDs: []string{"consume"}},
		{Kind: DeploymentResourceExpression, Name: "MyExpression", ConsumerIDs: []string{"consume"}},
		{Kind: DeploymentResourceScript, Name: "MyScript#1", ConsumerIDs: []string{"consume"}},
	}
	assertProvidedDependencies(t, engine, "provide", wantProvided...)

	if dependencies := consumer.Dependencies(); !reflect.DeepEqual(dependencies, []string{"provide"}) {
		t.Fatalf("consumer deployment dependencies = %v, want [provide]", dependencies)
	}
}

// TestClientDeployListDependenciesWModuleNameMatchesEsper covers
// ClientDeployListDependenciesWModuleName: two modules each own a window with
// the same logical name, and same-module consumers produce per-deployment
// dependency edges that also block the provider's undeploy, matching Java's
// path registry dependency entries and Undeployer check.
func TestClientDeployListDependenciesWModuleNameMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	// Java marks the windows @protected (path-visible but not public); Go
	// models object visibility at module granularity and a protected module
	// allows a single active deployment, so public modules carry the
	// multi-deployment provider/consumer shape of this execution.
	moduleA, err := env.RegisterModule("list-dependencies-a", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	moduleB, err := env.RegisterModule("list-dependencies-b", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema is missing")
	}
	if _, err := moduleA.RegisterNamedWindow("MyWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	if _, err := moduleB.RegisterNamedWindow("MyWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployUndeployResourceProvider(t, engine, moduleA, "createA")
	deployUndeployResourceProvider(t, engine, moduleB, "createB")

	consumer := func(module Module, name string) {
		t.Helper()
		plan, buildErr := module.Build(
			FromNamedWindowInModule(env, module.Name(), "MyWindow").Query(StatementName(name)))
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if _, deployErr := engine.Deploy(context.Background(), plan, WithDeploymentID(name)); deployErr != nil {
			t.Fatal(deployErr)
		}
	}
	consumer(moduleB, "B1")
	consumer(moduleA, "A1")
	consumer(moduleA, "A2")
	consumer(moduleB, "B2")

	assertProvidedDependencies(t, engine, "createA",
		DeploymentDependencyProvidedItem{Kind: DeploymentResourceNamedWindow, Name: "MyWindow", ConsumerIDs: []string{"A1", "A2"}})
	assertProvidedDependencies(t, engine, "createB",
		DeploymentDependencyProvidedItem{Kind: DeploymentResourceNamedWindow, Name: "MyWindow", ConsumerIDs: []string{"B1", "B2"}})
	for _, name := range []string{"A1", "A2"} {
		assertConsumedDependencies(t, engine, name,
			DeploymentDependencyConsumedItem{DeploymentID: "createA", Kind: DeploymentResourceNamedWindow, Name: "MyWindow"})
	}
	for _, name := range []string{"B1", "B2"} {
		assertConsumedDependencies(t, engine, name,
			DeploymentDependencyConsumedItem{DeploymentID: "createB", Kind: DeploymentResourceNamedWindow, Name: "MyWindow"})
	}

	// Java's Undeployer.checkDependency blocks the provider while a same-module
	// consumer deployment is active; undeploying the consumers frees it.
	assertUndeployPrecondition(t, engine.Undeploy(context.Background(), "createA"), "createA", "A1")
	for _, name := range []string{"A1", "A2"} {
		if err := engine.Undeploy(context.Background(), name); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.Undeploy(context.Background(), "createA"); err != nil {
		t.Fatalf("provider undeploy after consumers = %v", err)
	}
}

// TestClientDeployListDependencyStarMatchesEsper covers
// ClientDeployListDependencyStar: event types referenced as schema property
// types are deployment dependencies. Go registers the schema graph in one
// provider module and collects the direct nested property schemas of every
// referenced event type, so consumers of TypeD also consume TypeC.
func TestClientDeployListDependencyStarMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	module, err := env.RegisterModule("list-dependencies-star", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	mapField := reflect.TypeOf(map[string]any{})
	typeA, err := module.RegisterMap("TypeA", []FieldSpec{})
	if err != nil {
		t.Fatal(err)
	}
	typeB, err := module.RegisterMap("TypeB", []FieldSpec{})
	if err != nil {
		t.Fatal(err)
	}
	typeC, err := module.RegisterMap("TypeC", []FieldSpec{FieldDef("a", mapField), FieldDef("b", mapField)},
		WithNestedPropertySchema("a", typeA), WithNestedPropertySchema("b", typeB))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterMap("TypeD", []FieldSpec{FieldDef("c", mapField)},
		WithNestedPropertySchema("c", typeC)); err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterMap("TypeE", []FieldSpec{FieldDef("c", mapField)},
		WithNestedPropertySchema("c", typeC)); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployUndeployResourceProvider(t, engine, module, "provide")

	consume := func(name, eventType string) {
		t.Helper()
		plan, buildErr := env.Build(FromAny(env, module.EventType(eventType)).Query(StatementName(name)))
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if _, deployErr := engine.Deploy(context.Background(), plan, WithDeploymentID(name)); deployErr != nil {
			t.Fatal(deployErr)
		}
	}
	consume("useC", "TypeC")
	consume("useD", "TypeD")
	consume("useE", "TypeE")

	assertProvidedDependencies(t, engine, "provide",
		DeploymentDependencyProvidedItem{Kind: DeploymentResourceEventType, Name: "TypeA", ConsumerIDs: []string{"useC"}},
		DeploymentDependencyProvidedItem{Kind: DeploymentResourceEventType, Name: "TypeB", ConsumerIDs: []string{"useC"}},
		DeploymentDependencyProvidedItem{Kind: DeploymentResourceEventType, Name: "TypeC", ConsumerIDs: []string{"useC", "useD", "useE"}},
		DeploymentDependencyProvidedItem{Kind: DeploymentResourceEventType, Name: "TypeD", ConsumerIDs: []string{"useD"}},
		DeploymentDependencyProvidedItem{Kind: DeploymentResourceEventType, Name: "TypeE", ConsumerIDs: []string{"useE"}})
	assertConsumedDependencies(t, engine, "useC",
		DeploymentDependencyConsumedItem{DeploymentID: "provide", Kind: DeploymentResourceEventType, Name: "TypeA"},
		DeploymentDependencyConsumedItem{DeploymentID: "provide", Kind: DeploymentResourceEventType, Name: "TypeB"},
		DeploymentDependencyConsumedItem{DeploymentID: "provide", Kind: DeploymentResourceEventType, Name: "TypeC"})
	assertConsumedDependencies(t, engine, "useD",
		DeploymentDependencyConsumedItem{DeploymentID: "provide", Kind: DeploymentResourceEventType, Name: "TypeC"},
		DeploymentDependencyConsumedItem{DeploymentID: "provide", Kind: DeploymentResourceEventType, Name: "TypeD"})
	assertConsumedDependencies(t, engine, "useE",
		DeploymentDependencyConsumedItem{DeploymentID: "provide", Kind: DeploymentResourceEventType, Name: "TypeC"},
		DeploymentDependencyConsumedItem{DeploymentID: "provide", Kind: DeploymentResourceEventType, Name: "TypeE"})

	deployment, ok := engine.Deployment("useD")
	if !ok {
		t.Fatal("useD deployment is missing")
	}
	if dependencies := deployment.Dependencies(); !reflect.DeepEqual(dependencies, []string{"provide"}) {
		t.Fatalf("useD dependencies = %v, want [provide]", dependencies)
	}
}

// TestClientDeployListDependenciesNoDependenciesMatchesEsper covers
// ClientDeployListDependenciesNoDependencies: deployments without catalog
// references and a provider without consumers both report empty lists.
func TestClientDeployListDependenciesNoDependenciesMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(From[clientDeployUndeployResourceEvent](env, "SupportBean").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan, WithDeploymentID("s0")); err != nil {
		t.Fatal(err)
	}
	assertProvidedDependencies(t, engine, "s0")
	assertConsumedDependencies(t, engine, "s0")

	module, err := env.RegisterModule("list-dependencies-none", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterTable("MyTable", []TableColumn{TableColumnOf[string]("k"), TableColumnOf[string]("v")}); err != nil {
		t.Fatal(err)
	}
	deployUndeployResourceProvider(t, engine, module, "table")
	assertProvidedDependencies(t, engine, "table")
	assertConsumedDependencies(t, engine, "table")
}

// TestClientDeployListDependenciesInvalidMatchesEsper covers
// ClientDeployListDependenciesInvalid: unknown deployment IDs report
// not-found; Java additionally raises IllegalArgumentException for null IDs,
// which has no Go counterpart, so blank IDs report not-found as well.
func TestClientDeployListDependenciesInvalidMatchesEsper(t *testing.T) {
	env := newUndeployResourceEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	if _, ok := engine.DeploymentDependenciesConsumed("dummy"); ok {
		t.Fatal("consumed dependencies of unknown deployment found")
	}
	if _, ok := engine.DeploymentDependenciesProvided("dummy"); ok {
		t.Fatal("provided dependencies of unknown deployment found")
	}
	if _, ok := engine.DeploymentDependenciesConsumed(""); ok {
		t.Fatal("consumed dependencies of blank deployment found")
	}
	if _, ok := engine.DeploymentDependenciesProvided(""); ok {
		t.Fatal("provided dependencies of blank deployment found")
	}
}
