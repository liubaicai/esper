package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func newProtectedClientModule(t *testing.T, env *Environment, name string) Module {
	t.Helper()
	module, err := env.RegisterModule(name, ProtectedModule())
	if err != nil {
		t.Fatal(err)
	}
	return module
}

func protectedSnapshotRows(t *testing.T, deployment *Deployment, statementName string) []Result {
	t.Helper()
	statement, ok := deployment.Statement(statementName)
	if !ok {
		t.Fatalf("deployment %s statement %q is missing", deployment.ID(), statementName)
	}
	result, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return result.Results()
}

func buildProtectedInfraDeployment(t *testing.T, env *Environment, module Module, namedWindow bool, ident string) []Plan {
	t.Helper()
	source := From[clientMultitenancySupportBean](env, "SupportBean")
	assignments := []TableAssignment{
		SetColumn("col1", Field[clientMultitenancySupportBean, string]("theString")),
		SetColumn("myident", Literal(ident)),
	}
	var insert Query
	var create Query
	if namedWindow {
		schema, err := module.RegisterMap("MyInfraRow", []FieldSpec{
			FieldDef("col1", reflect.TypeOf("")),
			FieldDef("myident", reflect.TypeOf("")),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := module.RegisterNamedWindow("MyInfra", schema, NamedWindowRetention(KeepAll())); err != nil {
			t.Fatal(err)
		}
		insert = OnEvent(source).InsertIntoNamedWindow("MyInfra", assignments...).InModule(module).Query(StatementName("insert"))
		create = module.NamedWindow("MyInfra").Query(StatementName("create"))
	} else {
		if _, err := module.RegisterTable("MyInfra", []TableColumn{
			PrimaryKeyColumn[string]("col1"),
			TableColumnOf[string]("myident"),
		}); err != nil {
			t.Fatal(err)
		}
		insert = OnEvent(source).InsertIntoTable("MyInfra", assignments...).InModule(module).Query(StatementName("insert"))
		create = module.Table("MyInfra").Query(StatementName("create"))
	}
	if insert.trigger == nil || insert.trigger.moduleName != module.Name() {
		t.Fatalf("infra trigger module = %#v, want %q", insert.trigger, module.Name())
	}
	if namedWindow {
		if _, ok := env.NamedWindowInModule(module.Name(), "MyInfra"); !ok {
			t.Fatalf("registered Named Window is missing from module %q", module.Name())
		}
	} else if _, ok := env.TableInModule(module.Name(), "MyInfra"); !ok {
		t.Fatalf("registered Table is missing from module %q", module.Name())
	}
	insertPlan, err := module.Build(insert)
	if err != nil {
		t.Fatal(err)
	}
	createPlan, err := module.Build(create)
	if err != nil {
		t.Fatal(err)
	}
	return []Plan{insertPlan, createPlan}
}

func TestClientMultitenancyProtectedInfraParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		name := "table"
		if namedWindow {
			name = "named-window"
		}
		t.Run(name, func(t *testing.T) {
			env := newClientMultitenancyEnvironment(t)
			moduleA := newProtectedClientModule(t, env, "infra-a")
			moduleB := newProtectedClientModule(t, env, "infra-b")
			plansA := buildProtectedInfraDeployment(t, env, moduleA, namedWindow, "A")
			plansB := buildProtectedInfraDeployment(t, env, moduleB, namedWindow, "B")
			if plansA[0].Hash() == plansB[0].Hash() || !strings.Contains(string(plansA[0].Canonical()), "module(infra-a)") {
				t.Fatal("protected module identity is missing from the immutable plan")
			}

			engine := NewEngine(env)
			if namedWindow {
				if _, ok := engine.NamedWindowInModule(moduleA.Name(), "MyInfra"); ok {
					t.Fatal("protected Named Window materialized before deployment")
				}
			} else if _, ok := engine.TableInModule(moduleA.Name(), "MyInfra"); ok {
				t.Fatal("protected Table materialized before deployment")
			}
			deploymentA, err := engine.DeployPlans(context.Background(), plansA)
			if err != nil {
				t.Fatal(err)
			}
			deploymentB, err := engine.DeployPlans(context.Background(), plansB)
			if err != nil {
				t.Fatal(err)
			}
			for _, event := range []clientMultitenancySupportBean{{TheString: "E1"}, {TheString: "E2"}} {
				if err := engine.SendEvent(context.Background(), event); err != nil {
					t.Fatal(err)
				}
			}
			assertRows := func(deployment *Deployment, ident string) {
				t.Helper()
				rows := protectedSnapshotRows(t, deployment, "create")
				if len(rows) != 2 {
					t.Fatalf("%s rows = %#v", ident, rows)
				}
				for index, row := range rows {
					if row.Get("col1").Any() != []string{"E1", "E2"}[index] || row.Get("myident").Any() != ident {
						t.Fatalf("%s row %d = %#v", ident, index, row)
					}
				}
			}
			assertRows(deploymentA, "A")
			assertRows(deploymentB, "B")

			if err := deploymentA.Undeploy(context.Background()); err != nil {
				t.Fatal(err)
			}
			active, err := engine.Statements(context.Background(), StatementDeploymentIDContains(deploymentA.ID()))
			if err != nil {
				t.Fatal(err)
			}
			if len(active) != 0 {
				t.Fatalf("undeployed statements remain active: %#v", active)
			}
			if namedWindow {
				if _, ok := engine.NamedWindowInModule(moduleA.Name(), "MyInfra"); ok {
					t.Fatal("undeployed protected Named Window is still active")
				}
			} else if _, ok := engine.TableInModule(moduleA.Name(), "MyInfra"); ok {
				t.Fatal("undeployed protected Table is still active")
			}
			assertRows(deploymentB, "B")
			if err := deploymentB.Undeploy(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func buildProtectedVariablePlan(t *testing.T, env *Environment, module Module, initial int) Plan {
	t.Helper()
	if err := module.RegisterVariable("myvar", initial); err != nil {
		t.Fatal(err)
	}
	variableName := module.QualifiedName("myvar")
	query := TimerInterval(From[clientMultitenancySupportBean](env, "SupportBean"), 10*time.Second).
		Every().
		Select(Alias("tick", Literal(true))).
		Query(
			StatementName("set"),
			WithOutput(OutputWhen(
				Literal(true),
				SetOutputVariable(variableName, Add[int](ModuleVariableRef[int](module, "myvar"), Literal(1))),
			)),
		)
	plan, err := module.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestClientMultitenancyProtectedVariableParity(t *testing.T) {
	env := newClientMultitenancyEnvironment(t)
	moduleA := newProtectedClientModule(t, env, "variable-a")
	moduleB := newProtectedClientModule(t, env, "variable-b")
	planA := buildProtectedVariablePlan(t, env, moduleA, 10)
	planB := buildProtectedVariablePlan(t, env, moduleB, 20)
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deploymentA, err := engine.Deploy(context.Background(), planA)
	if err != nil {
		t.Fatal(err)
	}
	deploymentB, err := engine.Deploy(context.Background(), planB)
	if err != nil {
		t.Fatal(err)
	}
	assertVariable := func(module Module, want int) {
		t.Helper()
		value, ok := engine.GetVariable(module.QualifiedName("myvar"))
		if !ok || value.Any() != want {
			t.Fatalf("module %s myvar = %#v, want %d", module.Name(), value, want)
		}
	}
	assertVariable(moduleA, 10)
	assertVariable(moduleB, 20)
	if err := engine.AdvanceTime(context.Background(), time.Unix(10, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	assertVariable(moduleA, 11)
	assertVariable(moduleB, 21)
	if err := deploymentA.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := engine.GetVariable(moduleA.QualifiedName("myvar")); ok {
		t.Fatal("undeployed protected variable is still active")
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(20, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	assertVariable(moduleB, 22)
	if err := deploymentB.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func buildProtectedContextPlan(t *testing.T, env *Environment, module Module, startValue string) Plan {
	t.Helper()
	definition, err := NewInitiatedContext(
		"ignored",
		Literal("singleton"),
		Equal[string](Field[clientMultitenancySupportBean, string]("theString"), Literal(startValue)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterContextDefinition("MyContext", definition); err != nil {
		t.Fatal(err)
	}
	query := From[clientMultitenancySupportBean](env, "SupportBean").
		Aggregate(Alias("cnt", CountAll())).
		Query(StatementName("s0"), WithContext(module.Context("MyContext")))
	plan, err := module.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestClientMultitenancyProtectedContextParity(t *testing.T) {
	env := newClientMultitenancyEnvironment(t)
	moduleA := newProtectedClientModule(t, env, "context-a")
	moduleB := newProtectedClientModule(t, env, "context-b")
	planA := buildProtectedContextPlan(t, env, moduleA, "A")
	planB := buildProtectedContextPlan(t, env, moduleB, "B")
	engine := NewEngine(env)
	deploymentA, err := engine.Deploy(context.Background(), planA)
	if err != nil {
		t.Fatal(err)
	}
	deploymentB, err := engine.Deploy(context.Background(), planB)
	if err != nil {
		t.Fatal(err)
	}
	assertCount := func(deployment *Deployment, want *int64) {
		t.Helper()
		rows := protectedSnapshotRows(t, deployment, "s0")
		if want == nil {
			if len(rows) != 0 {
				t.Fatalf("context rows = %#v, want none", rows)
			}
			return
		}
		if len(rows) != 1 || rows[0].Get("cnt").Any() != *want {
			t.Fatalf("context rows = %#v, want %d", rows, *want)
		}
	}
	assertCount(deploymentA, nil)
	assertCount(deploymentB, nil)
	if err := engine.SendEvent(context.Background(), clientMultitenancySupportBean{TheString: "B"}); err != nil {
		t.Fatal(err)
	}
	one := int64(1)
	assertCount(deploymentA, nil)
	assertCount(deploymentB, &one)
	for range 2 {
		if err := engine.SendEvent(context.Background(), clientMultitenancySupportBean{TheString: "A"}); err != nil {
			t.Fatal(err)
		}
	}
	two, three := int64(2), int64(3)
	assertCount(deploymentA, &two)
	assertCount(deploymentB, &three)
	if err := deploymentA.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientMultitenancySupportBean{TheString: "X"}); err != nil {
		t.Fatal(err)
	}
	four := int64(4)
	assertCount(deploymentB, &four)
	if err := deploymentB.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func buildProtectedEventTypePlans(t *testing.T, env *Environment, module Module, count bool) []Plan {
	t.Helper()
	fields := []FieldSpec{FieldDef("col1", reflect.TypeOf(""))}
	selections := []Selection{Alias("col1", Field[clientMultitenancySupportBean, string]("theString"))}
	aggregateSelection := Alias("c0", CountAll())
	if !count {
		fields = []FieldSpec{FieldDef("totalme", reflect.TypeOf(int(0)))}
		selections = []Selection{Alias("totalme", Field[clientMultitenancySupportBean, int]("intPrimitive"))}
		aggregateSelection = Alias("c0", Sum[int](Field[any, int]("totalme")))
	}
	if _, err := module.RegisterMap("MySchema", fields); err != nil {
		t.Fatal(err)
	}
	route, err := module.Build(Select(From[clientMultitenancySupportBean](env, "SupportBean"), selections...).
		InsertInto(module.EventType("MySchema"), StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	selectPlan, err := module.Build(module.Stream("MySchema").Aggregate(aggregateSelection).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	return []Plan{route, selectPlan}
}

func TestClientMultitenancyProtectedEventTypeParity(t *testing.T) {
	env := newClientMultitenancyEnvironment(t)
	moduleA := newProtectedClientModule(t, env, "event-type-a")
	moduleB := newProtectedClientModule(t, env, "event-type-b")
	engine := NewEngine(env)
	deploymentA, err := engine.DeployPlans(context.Background(), buildProtectedEventTypePlans(t, env, moduleA, true))
	if err != nil {
		t.Fatal(err)
	}
	deploymentB, err := engine.DeployPlans(context.Background(), buildProtectedEventTypePlans(t, env, moduleB, false))
	if err != nil {
		t.Fatal(err)
	}
	assertValue := func(deployment *Deployment, want any) {
		t.Helper()
		rows := protectedSnapshotRows(t, deployment, "s0")
		if len(rows) != 1 || !reflect.DeepEqual(rows[0].Get("c0").Any(), want) {
			t.Fatalf("aggregate rows = %#v, want %#v", rows, want)
		}
	}
	assertValue(deploymentA, int64(0))
	assertValue(deploymentB, nil)
	if err := engine.SendEvent(context.Background(), clientMultitenancySupportBean{TheString: "E1", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	assertValue(deploymentA, int64(1))
	assertValue(deploymentB, 10)
	if err := engine.SendEvent(context.Background(), clientMultitenancySupportBean{TheString: "E2", IntPrimitive: 20}); err != nil {
		t.Fatal(err)
	}
	assertValue(deploymentA, int64(2))
	assertValue(deploymentB, 30)
	if err := deploymentA.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientMultitenancySupportBean{TheString: "E3", IntPrimitive: 30}); err != nil {
		t.Fatal(err)
	}
	assertValue(deploymentB, 60)
	if err := deploymentB.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func buildProtectedExpressionPlan(t *testing.T, env *Environment, module Module, value int) Plan {
	t.Helper()
	if err := module.DefineExpression("my_expression", Literal(value)); err != nil {
		t.Fatal(err)
	}
	input := From[clientMultitenancySupportBean](env, "SupportBean").Window(LengthWindow(1))
	query := Select(input, Alias("c0", ModuleExpressionRef[int](module, "my_expression"))).
		Query(StatementName("s0"))
	plan, err := module.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestClientMultitenancyProtectedExpressionParity(t *testing.T) {
	env := newClientMultitenancyEnvironment(t)
	moduleA := newProtectedClientModule(t, env, "expression-a")
	moduleB := newProtectedClientModule(t, env, "expression-b")
	engine := NewEngine(env)
	deploymentA, err := engine.Deploy(context.Background(), buildProtectedExpressionPlan(t, env, moduleA, 1))
	if err != nil {
		t.Fatal(err)
	}
	deploymentB, err := engine.Deploy(context.Background(), buildProtectedExpressionPlan(t, env, moduleB, 2))
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientMultitenancySupportBean{}); err != nil {
		t.Fatal(err)
	}
	assertValue := func(deployment *Deployment, want int) {
		t.Helper()
		rows := protectedSnapshotRows(t, deployment, "s0")
		if len(rows) != 1 || rows[0].Get("c0").Any() != want {
			t.Fatalf("expression rows = %#v, want %d", rows, want)
		}
	}
	assertValue(deploymentA, 1)
	assertValue(deploymentB, 2)
	if err := deploymentA.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertValue(deploymentB, 2)
	if err := deploymentB.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestClientMultitenancyProtectedDeploymentLifecycleValidation(t *testing.T) {
	env := newClientMultitenancyEnvironment(t)
	module := newProtectedClientModule(t, env, "lifecycle")
	plan := buildProtectedExpressionPlan(t, env, module, 1)
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if deployment.Module() != module.Name() {
		t.Fatalf("deployment module = %q", deployment.Module())
	}
	if _, err := engine.Deploy(context.Background(), plan); err == nil || !errors.Is(err, ErrorDeployment) {
		t.Fatalf("duplicate protected deployment error = %v", err)
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	redeployed, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("redeploy protected module: %v", err)
	}
	if err := redeployed.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}

	other := newProtectedClientModule(t, env, "other-lifecycle")
	otherPlan := buildProtectedExpressionPlan(t, env, other, 2)
	if _, err := engine.DeployPlans(context.Background(), []Plan{plan, otherPlan}); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("mixed protected module deployment error = %v", err)
	}
}
