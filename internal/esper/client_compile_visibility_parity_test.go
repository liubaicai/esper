package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type clientVisibilityCatalog struct {
	module Module
	value  string
}

func registerClientVisibilityCatalog(t *testing.T, env *Environment, module Module, value string) clientVisibilityCatalog {
	t.Helper()
	if _, err := module.RegisterMap("MySchema", []FieldSpec{FieldDef("p1", reflect.TypeOf(""))}); err != nil {
		t.Fatal(err)
	}
	if err := module.RegisterVariable("abc", value); err != nil {
		t.Fatal(err)
	}
	definition, err := NewKeyContext("ignored", Field[clientMultitenancySupportBean, string]("theString"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterContextDefinition("MyContext", definition); err != nil {
		t.Fatal(err)
	}
	supportBean, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema is missing")
	}
	if _, err := module.RegisterNamedWindow("MyWindow", supportBean, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterTable("MyTable", []TableColumn{TableColumnOf[string]("c1")}); err != nil {
		t.Fatal(err)
	}
	if err := module.DefineExpression("MyExpr", Literal(value)); err != nil {
		t.Fatal(err)
	}
	if err := RegisterModuleScript[string](module, "myscript", "go", func(ScriptContext) (string, error) {
		return value, nil
	}, ScriptArgumentTypes()); err != nil {
		t.Fatal(err)
	}
	// Java's application-inlined class is represented by an analyzable named
	// expression/UDF boundary. Go deliberately does not load JVM classes.
	if err := module.DefineExpression("MyClass.doIt", Literal(value)); err != nil {
		t.Fatal(err)
	}
	return clientVisibilityCatalog{module: module, value: value}
}

func assertClientVisibilityAllResolve(t *testing.T, path ModulePath, moduleName string) {
	t.Helper()
	assertIdentity := func(kind, got string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("resolve %s: %v", kind, err)
		}
		want := catalogKey(moduleName, map[string]string{
			"event type":          "MySchema",
			"variable":            "abc",
			"context":             "MyContext",
			"declared expression": "MyExpr",
			"script":              "myscript",
			"inline equivalent":   "MyClass.doIt",
		}[kind])
		if got != want {
			t.Fatalf("resolved %s = %q, want %q", kind, got, want)
		}
	}
	eventType, err := path.EventType("MySchema")
	assertIdentity("event type", eventType, err)
	variable, err := path.Variable("abc")
	assertIdentity("variable", variable, err)
	contextName, err := path.Context("MyContext")
	assertIdentity("context", contextName, err)
	expression, err := path.resolve(moduleObjectExpression, "MyExpr")
	assertIdentity("declared expression", expression, err)
	script, err := path.resolve(moduleObjectScript, "myscript")
	assertIdentity("script", script, err)
	inlineEquivalent, err := path.resolve(moduleObjectExpression, "MyClass.doIt")
	assertIdentity("inline equivalent", inlineEquivalent, err)
	window, err := path.NamedWindow("MyWindow")
	if err != nil {
		t.Fatal(err)
	}
	windowBase := window.node
	if windowBase == nil || windowBase.moduleName != moduleName || windowBase.sourceName != "MyWindow" {
		t.Fatalf("resolved named window = %#v", windowBase)
	}
	table, err := path.Table("MyTable")
	if err != nil {
		t.Fatal(err)
	}
	tableBase := table.node
	if tableBase == nil || tableBase.moduleName != moduleName || tableBase.sourceName != "MyTable" {
		t.Fatalf("resolved table = %#v", tableBase)
	}
}

func assertClientVisibilityNoneResolve(t *testing.T, path ModulePath) {
	t.Helper()
	checks := []func() error{
		func() error { _, err := path.EventType("MySchema"); return err },
		func() error { _, err := path.Variable("abc"); return err },
		func() error { _, err := path.Context("MyContext"); return err },
		func() error { _, err := path.NamedWindow("MyWindow"); return err },
		func() error { _, err := path.Table("MyTable"); return err },
		func() error { _, err := path.resolve(moduleObjectExpression, "MyExpr"); return err },
		func() error { _, err := path.resolve(moduleObjectScript, "myscript"); return err },
		func() error { _, err := path.resolve(moduleObjectExpression, "MyClass.doIt"); return err },
	}
	for index, check := range checks {
		if err := check(); err == nil || !errors.Is(err, ErrorUnknownName) {
			t.Fatalf("invisible definition %d error = %v, want ErrorUnknownName", index, err)
		}
	}
}

// deployClientVisibilityProvider deploys one pass-through statement of the
// provider module, mirroring Java deploying the module that contains the
// @public create statements before consumers compiled against its path are
// deployed themselves.
func deployClientVisibilityProvider(t *testing.T, engine *Engine, module Module) {
	t.Helper()
	plan, err := module.Build(
		Select(From[clientMultitenancySupportBean](module.env, "SupportBean"),
			Alias("theString", Field[clientMultitenancySupportBean, string]("theString"))).Query(StatementName("provider")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
}

func deployClientVisibilityProjection(t *testing.T, path ModulePath, providers ...Module) Row {
	t.Helper()
	variable, err := ModulePathVariableRef[string](path, "abc")
	if err != nil {
		t.Fatal(err)
	}
	expression, err := ModulePathExpressionRef[string](path, "MyExpr")
	if err != nil {
		t.Fatal(err)
	}
	script, err := ModulePathScriptCall[string](path, "myscript")
	if err != nil {
		t.Fatal(err)
	}
	inlineEquivalent, err := ModulePathExpressionRef[string](path, "MyClass.doIt")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := path.Build(Select(From[clientMultitenancySupportBean](path.env, "SupportBean"),
		Alias("variable", variable),
		Alias("expression", expression),
		Alias("script", script),
		Alias("inline", inlineEquivalent),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(path.env)
	for _, provider := range providers {
		deployClientVisibilityProvider(t, engine, provider)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement, _ := deployment.Statement("s0")
	rows := make([]Row, 0, 1)
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientMultitenancySupportBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("projection rows = %#v", rows)
	}
	return rows[0]
}

func TestClientCompileVisibilityDefaultAndExplicitPrivateMatchEsper(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		options []ModuleOption
	}{{name: "default"}, {name: "annotation-private", options: []ModuleOption{PrivateModule()}}} {
		t.Run(testCase.name, func(t *testing.T) {
			env := newClientMultitenancyEnvironment(t)
			module, err := env.RegisterModule("a.b.c", testCase.options...)
			if err != nil {
				t.Fatal(err)
			}
			if !module.Private() {
				t.Fatalf("module visibility = %s, want private", module.Visibility())
			}
			registerClientVisibilityCatalog(t, env, module, "private")
			assertClientVisibilityNoneResolve(t, env.Path())
			assertClientVisibilityAllResolve(t, module.Path(), module.Name())
			row := deployClientVisibilityProjection(t, module.Path())
			for _, field := range []string{"variable", "expression", "script", "inline"} {
				if got := row.Get(field).Any(); got != "private" {
					t.Fatalf("%s = %#v", field, got)
				}
			}
		})
	}
}

func TestClientCompileVisibilityProtectedAndPublicMatchEsper(t *testing.T) {
	t.Run("protected", func(t *testing.T) {
		env := newClientMultitenancyEnvironment(t)
		owner, err := env.RegisterModule("a.b.c", ProtectedModule())
		if err != nil {
			t.Fatal(err)
		}
		other, err := env.RegisterModule("a.b.d")
		if err != nil {
			t.Fatal(err)
		}
		registerClientVisibilityCatalog(t, env, owner, "protected")
		assertClientVisibilityAllResolve(t, owner.Path(), owner.Name())
		assertClientVisibilityNoneResolve(t, other.Path())
		assertClientVisibilityNoneResolve(t, env.Path())
	})

	t.Run("public", func(t *testing.T) {
		env := newClientMultitenancyEnvironment(t)
		module, err := env.RegisterModule("a.b.c", PublicModule())
		if err != nil {
			t.Fatal(err)
		}
		registerClientVisibilityCatalog(t, env, module, "public")
		assertClientVisibilityAllResolve(t, env.Path(), module.Name())
		row := deployClientVisibilityProjection(t, env.Path(), module)
		if row.Get("script").Any() != "public" {
			t.Fatalf("public script = %#v", row.Get("script"))
		}
	})
}

func TestClientCompileVisibilityAmbiguousPathAndUsesDisambiguationMatchEsper(t *testing.T) {
	env := newClientMultitenancyEnvironment(t)
	moduleA, err := env.RegisterModule("a", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	moduleB, err := env.RegisterModule("b", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	registerClientVisibilityCatalog(t, env, moduleA, "abc")
	registerClientVisibilityCatalog(t, env, moduleB, "def")

	ambiguous := []func() error{
		func() error { _, err := env.Path().EventType("MySchema"); return err },
		func() error { _, err := env.Path().Variable("abc"); return err },
		func() error { _, err := env.Path().Context("MyContext"); return err },
		func() error { _, err := env.Path().NamedWindow("MyWindow"); return err },
		func() error { _, err := env.Path().Table("MyTable"); return err },
		func() error { _, err := env.Path().resolve(moduleObjectExpression, "MyExpr"); return err },
		func() error { _, err := env.Path().resolve(moduleObjectScript, "myscript"); return err },
		func() error { _, err := env.Path().resolve(moduleObjectExpression, "MyClass.doIt"); return err },
	}
	for index, check := range ambiguous {
		if err := check(); err == nil || !errors.Is(err, ErrorAmbiguous) {
			t.Fatalf("ambiguous definition %d error = %v, want ErrorAmbiguous", index, err)
		}
	}

	path := env.Uses(moduleB)
	assertClientVisibilityAllResolve(t, path, moduleB.Name())
	// Java deploys both providing modules before the consumer.
	row := deployClientVisibilityProjection(t, path, moduleA, moduleB)
	for _, field := range []string{"variable", "expression", "script", "inline"} {
		if got := row.Get(field).Any(); got != "def" {
			t.Fatalf("uses b %s = %#v, want def", field, got)
		}
	}
	plan, err := path.Build(From[clientMultitenancySupportBean](env, "SupportBean").Query())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plan.Canonical()), "uses(b)") || len(plan.Query().ModuleUses()) != 1 {
		t.Fatalf("uses dependency missing from plan identity: %s", plan.Canonical())
	}
}

func TestClientCompileVisibilityAmbiguousWithPreconfiguredCatalogMatchesEsper(t *testing.T) {
	env := newClientMultitenancyEnvironment(t)
	if err := env.RegisterVariable("preconfigured_variable", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "SupportBean_S1", []FieldSpec{FieldDef("p0", reflect.TypeOf(""))}); err != nil {
		t.Fatal(err)
	}
	module, err := env.RegisterModule("path", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if err := module.RegisterVariable("preconfigured_variable", 2); err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterMap("SupportBean_S1", []FieldSpec{FieldDef("p0", reflect.TypeOf(""))}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Path().Variable("preconfigured_variable"); err == nil || !errors.Is(err, ErrorAmbiguous) || !strings.Contains(err.Error(), "preconfigured") {
		t.Fatalf("variable ambiguity = %v", err)
	}
	if _, err := env.Path().EventType("SupportBean_S1"); err == nil || !errors.Is(err, ErrorAmbiguous) || !strings.Contains(err.Error(), "preconfigured") {
		t.Fatalf("event-type ambiguity = %v", err)
	}
}

func TestClientCompileVisibilityModuleNameOptionOverridesRuleIdentityMatchEsper(t *testing.T) {
	env := newClientMultitenancyEnvironment(t)
	moduleX, err := env.RegisterModule("x")
	if err != nil {
		t.Fatal(err)
	}
	moduleABC, err := env.RegisterModule("abc")
	if err != nil {
		t.Fatal(err)
	}
	query := Select(From[clientMultitenancySupportBean](env, "SupportBean"), Alias("one", Literal(1))).Query()
	query.moduleName = moduleX.Name()
	plan, err := moduleABC.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Query().Module() != "abc" || strings.Contains(string(plan.Canonical()), "module(x)") {
		t.Fatalf("module override plan = %s", plan.Canonical())
	}
	deployment, err := NewEngine(env).Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if deployment.Module() != "abc" {
		t.Fatalf("deployment module = %q", deployment.Module())
	}
}

func TestClientCompileVisibilityAnnotationsAndBusEventTypeMatchEsper(t *testing.T) {
	for _, options := range [][]ModuleOption{
		{PrivateModule(), ProtectedModule()},
		{PrivateModule(), PublicModule()},
		{ProtectedModule(), PublicModule()},
	} {
		if _, err := NewEnvironment().RegisterModule("invalid", options...); err == nil || !errors.Is(err, ErrorInvalidRule) {
			t.Fatalf("conflicting visibility options error = %v", err)
		}
	}

	for _, option := range []ModuleOption{PrivateModule(), ProtectedModule()} {
		env := NewEnvironment()
		module, err := env.RegisterModule("not-public", option)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := module.RegisterMap("ABC", nil, BusEventType()); err == nil || !errors.Is(err, ErrorInvalidRule) {
			t.Fatalf("non-public bus schema error = %v", err)
		}
	}

	env := NewEnvironment()
	module, err := env.RegisterModule("events", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterMap("MyEvent", []FieldSpec{FieldDef("p0", reflect.TypeOf(""))}, BusEventType()); err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterMap("InternalEvent", nil); err != nil {
		t.Fatal(err)
	}
	path := env.Path()
	eventType, err := path.EventType("MyEvent")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := path.Build(FromAny(env, eventType).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	// Java deploys the module that contains the @Public @BusEventType schema
	// before the consumer statement; the Go chain mirrors that provider-first
	// order with a pass-through deployment of the owning module.
	providerPlan, err := module.Build(FromAny(env, module.EventType("MyEvent")).Query(StatementName("provider")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), providerPlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement, _ := deployment.Statement("s0")
	invoked := 0
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		invoked += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendBusRecord(context.Background(), eventType, map[string]any{"p0": "E1"}); err != nil {
		t.Fatal(err)
	}
	if invoked != 1 {
		t.Fatalf("bus listener invocation count = %d", invoked)
	}
	if err := engine.SendBusRecord(context.Background(), module.EventType("InternalEvent"), map[string]any{}); err == nil || !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("internal event bus ingress error = %v", err)
	}
}

func TestClientCompileVisibilityPublicNamedWindowAcrossDeploymentsMatchEsper(t *testing.T) {
	env := newClientMultitenancyEnvironment(t)
	module, err := env.RegisterModule("window-owner", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	schema, _ := env.Schema("SupportBean")
	if _, err := module.RegisterNamedWindow("MyWindow", schema, NamedWindowRetention(LengthWindow(2))); err != nil {
		t.Fatal(err)
	}
	insertQuery := OnEvent(From[clientMultitenancySupportBean](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindow",
		SetColumn("theString", Field[clientMultitenancySupportBean, string]("theString")),
		SetColumn("intPrimitive", Field[clientMultitenancySupportBean, int]("intPrimitive")),
	).InModule(module).Query(StatementName("insert"))
	// The insert statement belongs to the owning module deployment, matching
	// Java deploying the module that contains the create-window before
	// consumers in other deployments.
	insertPlan, err := module.Build(insertQuery)
	if err != nil {
		t.Fatal(err)
	}
	window, err := env.Path().NamedWindow("MyWindow")
	if err != nil {
		t.Fatal(err)
	}
	selectPlan, err := env.Path().Build(window.Aggregate(
		Alias("c0", Field[any, string]("theString")),
		Alias("c1", Sum[int](Field[any, int]("intPrimitive"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	consumer, err := engine.Deploy(context.Background(), selectPlan)
	if err != nil {
		t.Fatal(err)
	}
	statement, _ := consumer.Statement("s0")
	rows := make([]Row, 0, 4)
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []clientMultitenancySupportBean{
		{TheString: "E1", IntPrimitive: 10},
		{TheString: "E2", IntPrimitive: 20},
		{TheString: "E3", IntPrimitive: 25},
		{TheString: "E4", IntPrimitive: 26},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	wantNames := []string{"E1", "E2", "E3", "E4"}
	wantSums := []int{10, 30, 45, 51}
	if len(rows) != len(wantNames) {
		t.Fatalf("named-window rows = %#v", rows)
	}
	for index, row := range rows {
		if row.Get("c0").Any() != wantNames[index] || row.Get("c1").Any() != wantSums[index] {
			t.Fatalf("named-window row %d = %#v", index, row)
		}
	}
}
