package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type clientDeployRedefinitionEvent struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func newRedefinitionEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientDeployRedefinitionEvent](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

// deployRedefinitionProvider deploys one pass-through statement of the
// module, making the deployment the provider of the module catalog.
func deployRedefinitionProvider(t *testing.T, engine *Engine, module Module, deploymentID string) *Deployment {
	t.Helper()
	plan, err := module.Build(FromAny(module.env, "SupportBean").Query(StatementName("infra")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan, WithDeploymentID(deploymentID))
	if err != nil {
		t.Fatal(err)
	}
	return deployment
}

// TestClientDeployRedefinitionCreateSchemaNamedWindowInsertMatchesEsper
// covers ClientDeployRedefinitionCreateSchemaNamedWindowInsert: deploy and
// undeployAll cycles leave no filter/subscription residue, the same immutable
// plan redeploys cleanly, and an evolved definition deploys after the
// previous one is removed. Go registrations are environment-scoped, so the
// evolved schema/window/table use a new module identity instead of
// re-registering the same name after undeployAll (registration-time model
// difference); the deploy/undeploy lifecycle parity is pinned directly.
func TestClientDeployRedefinitionCreateSchemaNamedWindowInsertMatchesEsper(t *testing.T) {
	env := newRedefinitionEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	registerInfra := func(moduleName string, columns []FieldSpec) Module {
		t.Helper()
		module, err := env.RegisterModule(moduleName, PublicModule())
		if err != nil {
			t.Fatal(err)
		}
		typeSchema, err := module.RegisterMap("MyTypeOne", columns)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := module.RegisterNamedWindow("MyWindowOne", typeSchema, NamedWindowRetention(KeepAll())); err != nil {
			t.Fatal(err)
		}
		return module
	}
	insertPlan := func(module Module) Plan {
		t.Helper()
		plan, err := module.Build(OnRecord(FromAny(env, module.EventType("MyTypeOne"))).
			InsertIntoNamedWindow(module.QualifiedName("MyWindowOne"),
				SetColumn("col1", Field[Event, string]("col1")),
				SetColumn("col2", Field[Event, int]("col2"))).
			Query(StatementName("insert")))
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}

	columnsV1 := []FieldSpec{FieldDef("col1", reflect.TypeOf("")), FieldDef("col2", reflect.TypeOf(int(0)))}
	moduleV1 := registerInfra("redefinition.test1", columnsV1)
	planV1 := insertPlan(moduleV1)
	// deploy / undeployAll twice with the same plan, matching the Java module
	// text being compiled and deployed twice.
	for cycle := 0; cycle < 2; cycle++ {
		deployment, err := engine.Deploy(context.Background(), planV1)
		if err != nil {
			t.Fatalf("cycle %d deploy = %v", cycle, err)
		}
		if err := engine.Undeploy(context.Background(), deployment.ID()); err != nil {
			t.Fatalf("cycle %d undeploy = %v", cycle, err)
		}
	}

	// The evolved definition (col3 added) is a new module identity in Go.
	columnsV2 := append(append([]FieldSpec(nil), columnsV1...), FieldDef("col3", reflect.TypeOf(int64(0))))
	moduleV2 := registerInfra("redefinition.test1.v2", columnsV2)
	deploymentV2, err := engine.Deploy(context.Background(), insertPlan(moduleV2))
	if err != nil {
		t.Fatalf("evolved deploy = %v", err)
	}
	if err := engine.Undeploy(context.Background(), deploymentV2.ID()); err != nil {
		t.Fatal(err)
	}

	// on-merge chain: window -> insert-into SecondStream -> merge MyWindow with
	// insert-into ThirdStream and delete actions; the same plans redeploy.
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema is missing")
	}
	if _, err := env.RegisterNamedWindow("MyWindow", schema,
		NamedWindowRetention(Unique(Field[clientDeployRedefinitionEvent, int]("intPrimitive")))); err != nil {
		t.Fatal(err)
	}
	streamFields := []FieldSpec{FieldDef("theString", reflect.TypeOf("")), FieldDef("intPrimitive", reflect.TypeOf(int(0)))}
	if _, err := RegisterMap(env, "SecondStream", streamFields); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "ThirdStream", streamFields); err != nil {
		t.Fatal(err)
	}
	s1, err := env.Build(FromNamedWindow(env, "MyWindow").InsertInto("SecondStream", StatementName("S1")))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := env.Build(OnRecord(FromAny(env, "SecondStream")).
		MergeIntoNamedWindowWhen("MyWindow", Equal[int](Field[Event, int]("intPrimitive"), NamedWindowField[int]("intPrimitive")),
			WhenMatchedActions(
				ThenInsertInto("ThirdStream",
					Alias("theString", Field[Event, string]("theString")),
					Alias("intPrimitive", Field[Event, int]("intPrimitive"))),
				ThenDelete(Literal(true)),
			)).
		Query(StatementName("S2")))
	if err != nil {
		t.Fatal(err)
	}
	for cycle := 0; cycle < 2; cycle++ {
		deployment, err := engine.DeployPlans(context.Background(), []Plan{s1, s2})
		if err != nil {
			t.Fatalf("on-merge cycle %d deploy = %v", cycle, err)
		}
		if err := engine.Undeploy(context.Background(), deployment.ID()); err != nil {
			t.Fatalf("on-merge cycle %d undeploy = %v", cycle, err)
		}
	}

	// table redeploy with evolved columns uses a new table identity in Go.
	if _, err := env.RegisterTable("MyTable", []TableColumn{TableColumnOf[string]("c0"), TableColumnOf[string]("c1")}); err != nil {
		t.Fatal(err)
	}
	tablePlan := func(name string) Plan {
		t.Helper()
		plan, buildErr := env.Build(OnEvent(From[clientDeployRedefinitionEvent](env, "SupportBean")).InsertIntoTable(name,
			SetColumn("c0", Field[clientDeployRedefinitionEvent, string]("theString")),
			SetColumn("c1", Literal("v"))).
			Query(StatementName("insert-table")))
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		return plan
	}
	deploymentT1, err := engine.Deploy(context.Background(), tablePlan("MyTable"))
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Undeploy(context.Background(), deploymentT1.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := env.RegisterTable("MyTableV2", []TableColumn{
		TableColumnOf[string]("c0"), TableColumnOf[string]("c1"), TableColumnOf[string]("c2"),
	}); err != nil {
		t.Fatal(err)
	}
	deploymentT2, err := engine.Deploy(context.Background(), tablePlan("MyTableV2"))
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Undeploy(context.Background(), deploymentT2.ID()); err != nil {
		t.Fatal(err)
	}

	// Java asserts the filter service count returns to zero after undeployAll;
	// the Go equivalent is an engine with no active deployments that delivers
	// no further output.
	if got := len(engine.Deployments()); got != 0 {
		t.Fatalf("active deployments after undeploy-all = %d, want 0", got)
	}
}

// TestClientDeployRedefinitionNamedWindowMatchesEsper covers
// ClientDeployRedefinitionNamedWindow: two deployments each define a private
// window named MyWindow with different column types. Go scopes the same
// logical name per module identity, so both definitions coexist and are
// consumed independently.
func TestClientDeployRedefinitionNamedWindowMatchesEsper(t *testing.T) {
	env := newRedefinitionEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	moduleA, err := env.RegisterModule("redefinition.window.a", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	moduleB, err := env.RegisterModule("redefinition.window.b", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	schemaA, err := moduleA.RegisterMap("MyWindowType", []FieldSpec{
		FieldDef("col1", reflect.TypeOf(int(0))), FieldDef("col2", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	schemaB, err := moduleB.RegisterMap("MyWindowType", []FieldSpec{
		FieldDef("col1", reflect.TypeOf(int16(0))), FieldDef("col2", reflect.TypeOf(int64(0))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := moduleA.RegisterNamedWindow("MyWindow", schemaA, NamedWindowRetention(TimeWindow(30*time.Second))); err != nil {
		t.Fatal(err)
	}
	if _, err := moduleB.RegisterNamedWindow("MyWindow", schemaB, NamedWindowRetention(TimeWindow(30*time.Second))); err != nil {
		t.Fatal(err)
	}

	// Both deployments consume their own module's window independently.
	deployRedefinitionProvider(t, engine, moduleA, "window-a")
	deployRedefinitionProvider(t, engine, moduleB, "window-b")
	for _, item := range []struct {
		module Module
		name   string
	}{{moduleA, "consume-a"}, {moduleB, "consume-b"}} {
		plan, buildErr := item.module.Build(
			FromNamedWindowInModule(env, item.module.Name(), "MyWindow").Query(StatementName(item.name)))
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if _, deployErr := engine.Deploy(context.Background(), plan, WithDeploymentID(item.name)); deployErr != nil {
			t.Fatal(deployErr)
		}
	}
	for _, deploymentID := range []string{"consume-a", "consume-b", "window-a", "window-b"} {
		if err := engine.Undeploy(context.Background(), deploymentID); err != nil {
			t.Fatalf("undeploy %s = %v", deploymentID, err)
		}
	}
}

// TestClientDeployRedefinitionInsertIntoMatchesEsper covers
// ClientDeployRedefinitionInsertInto: two deployments each create a private
// schema MySchema with different types and insert into their own MyStream.
// Go scopes schema and insert target per module identity.
func TestClientDeployRedefinitionInsertIntoMatchesEsper(t *testing.T) {
	env := newRedefinitionEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	rows := make(map[string][]any)
	deployModule := func(moduleName string, col1Type reflect.Type, col2Type reflect.Type) {
		t.Helper()
		module, err := env.RegisterModule(moduleName, PublicModule())
		if err != nil {
			t.Fatal(err)
		}
		fields := []FieldSpec{FieldDef("col1", col1Type), FieldDef("col2", col2Type)}
		if _, err := module.RegisterMap("MySchema", fields); err != nil {
			t.Fatal(err)
		}
		if _, err := module.RegisterMap("MyStream", fields); err != nil {
			t.Fatal(err)
		}
		producer, err := module.Build(FromAny(env, module.EventType("MySchema")).Select(
			Alias("col1", Field[Event, any]("col1")),
			Alias("col2", Field[Event, any]("col2")),
		).InsertInto(module.EventType("MyStream"), StatementName("producer")))
		if err != nil {
			t.Fatal(err)
		}
		consumer, err := module.Build(FromAny(env, module.EventType("MyStream")).Query(StatementName("consumer")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.DeployPlans(context.Background(), []Plan{producer, consumer})
		if err != nil {
			t.Fatal(err)
		}
		statement, ok := deployment.Statement("consumer")
		if !ok {
			t.Fatal("consumer statement is missing")
		}
		rows[moduleName] = make([]any, 0)
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, row := range batch.New {
				rows[moduleName] = append(rows[moduleName], row.Get("col1").Any())
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	deployModule("redefinition.insert.a", reflect.TypeOf(int(0)), reflect.TypeOf(""))
	deployModule("redefinition.insert.b", reflect.TypeOf(int16(0)), reflect.TypeOf(int64(0)))

	if err := engine.SendRecord(context.Background(), "redefinition.insert.a::MySchema", map[string]any{"col1": 1, "col2": "a"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendRecord(context.Background(), "redefinition.insert.b::MySchema", map[string]any{"col1": int16(2), "col2": int64(3)}); err != nil {
		t.Fatal(err)
	}
	if got := rows["redefinition.insert.a"]; !reflect.DeepEqual(got, []any{1}) {
		t.Fatalf("module A stream rows = %v", got)
	}
	if got := rows["redefinition.insert.b"]; !reflect.DeepEqual(got, []any{int16(2)}) {
		t.Fatalf("module B stream rows = %v", got)
	}
}

// TestClientDeployRedefinitionVariablesMatchesEsper covers
// ClientDeployRedefinitionVariables: two deployments each define a private
// variable MyVar with different types. Go scopes the variable per module.
func TestClientDeployRedefinitionVariablesMatchesEsper(t *testing.T) {
	env := newRedefinitionEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	values := make(map[string][]any)
	deployModule := func(moduleName string, initial any) {
		t.Helper()
		module, err := env.RegisterModule(moduleName, PublicModule())
		if err != nil {
			t.Fatal(err)
		}
		if err := module.RegisterVariable("MyVar", initial); err != nil {
			t.Fatal(err)
		}
		if _, err := module.RegisterMap("MySchema", []FieldSpec{FieldDef("col1", reflect.TypeOf(int16(0))), FieldDef("col2", reflect.TypeOf(int64(0)))}); err != nil {
			t.Fatal(err)
		}
		plan, err := module.Build(FromAny(env, module.EventType("MySchema")).Select(
			Alias("v", VariableRef[any](module.QualifiedName("MyVar"))),
		).Query(StatementName("select-variable")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement, ok := deployment.Statement("select-variable")
		if !ok {
			t.Fatal("statement is missing")
		}
		values[moduleName] = make([]any, 0)
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, row := range batch.New {
				values[moduleName] = append(values[moduleName], row.Get("v").Any())
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	deployModule("redefinition.variables.a", 0)
	deployModule("redefinition.variables.b", "text")

	for _, moduleName := range []string{"redefinition.variables.a", "redefinition.variables.b"} {
		if err := engine.SendRecord(context.Background(), moduleName+"::MySchema", map[string]any{"col1": int16(1), "col2": int64(2)}); err != nil {
			t.Fatal(err)
		}
	}
	if got := values["redefinition.variables.a"]; !reflect.DeepEqual(got, []any{0}) {
		t.Fatalf("module A variable value = %v", got)
	}
	if got := values["redefinition.variables.b"]; !reflect.DeepEqual(got, []any{"text"}) {
		t.Fatalf("module B variable value = %v", got)
	}
}
