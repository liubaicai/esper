package esper

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
)

type clientCompileModuleEvent struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func newClientCompileModuleEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientCompileModuleEvent](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func TestClientCompileModuleWImportsMatchesEsper(t *testing.T) {
	env := newClientCompileModuleEnvironment(t)
	module, err := env.RegisterModule(
		"com.testit",
		WithModuleImports("com.espertech.esper.regressionlib.support.epl.*"),
	)
	if err != nil {
		t.Fatal(err)
	}
	value := Field[clientCompileModuleEvent, int]("intPrimitive")
	plan, err := module.Build(
		Select(
			From[clientCompileModuleEvent](env, "SupportBean"),
			Alias("val", Add[int](value, Literal(1))),
		).Query(StatementName("A")),
	)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	assertValue := func(input, expected int) {
		t.Helper()
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		metadata := deployment.ModuleMetadata()
		if deployment.Module() != "com.testit" || !reflect.DeepEqual(metadata.Imports, []string{"com.espertech.esper.regressionlib.support.epl.*"}) {
			t.Fatalf("module deployment name=%q metadata=%#v", deployment.Module(), metadata)
		}
		var values []int
		statement, ok := deployment.Statement("A")
		if !ok {
			t.Fatal("module import statement A is missing")
		}
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				values = append(values, result.Get("val").Any().(int))
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), clientCompileModuleEvent{TheString: "E1", IntPrimitive: input}); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(values, []int{expected}) {
			t.Fatalf("module import values = %v, want [%d]", values, expected)
		}
		if err := deployment.Undeploy(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	assertValue(4, 5)
	assertValue(6, 7)
}

func TestClientCompileModuleTwoModulesMatchesEsper(t *testing.T) {
	env := newClientCompileModuleEnvironment(t)
	moduleOne, err := env.RegisterModule(
		"regression.test",
		WithModuleURI("uri1"),
		WithModuleArchiveName("archive1"),
		WithModuleUserObject("obj1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	moduleTwo, err := env.RegisterModule(
		"regression.test.two",
		WithModuleURI("uri2"),
		WithModuleArchiveName("archive2"),
		WithModuleUserObject("obj2"),
		WithModuleUses("a", "b"),
		WithModuleImports("c", "d"),
	)
	if err != nil {
		t.Fatal(err)
	}
	buildPlans := func(module Module, prefix string) []Plan {
		t.Helper()
		value := Field[clientCompileModuleEvent, int]("intPrimitive")
		queries := []Query{
			Select(From[clientCompileModuleEvent](env, "SupportBean"), Alias("value", value)).Query(StatementName(prefix + "-schema")),
			Select(From[clientCompileModuleEvent](env, "SupportBean"), Alias("value", Add[int](value, Literal(1)))).Query(StatementName(prefix + "-select")),
		}
		plans := make([]Plan, len(queries))
		for index, query := range queries {
			plan, err := module.Build(query)
			if err != nil {
				t.Fatal(err)
			}
			plans[index] = plan
		}
		return plans
	}
	engine := NewEngine(env)
	deploymentOne, err := engine.DeployPlans(context.Background(), buildPlans(moduleOne, "one"))
	if err != nil {
		t.Fatal(err)
	}
	deploymentTwo, err := engine.DeployPlans(context.Background(), buildPlans(moduleTwo, "two"))
	if err != nil {
		t.Fatal(err)
	}
	if got := engine.RuntimePath().DeploymentIDs(); len(got) != 2 {
		t.Fatalf("active module deployments = %v", got)
	}
	deployments := []*Deployment{deploymentOne, deploymentTwo}
	sort.Slice(deployments, func(left, right int) bool { return deployments[left].Module() < deployments[right].Module() })
	want := []struct {
		name     string
		metadata ModuleMetadata
	}{
		{name: "regression.test", metadata: ModuleMetadata{URI: "uri1", ArchiveName: "archive1", UserObject: "obj1"}},
		{name: "regression.test.two", metadata: ModuleMetadata{URI: "uri2", ArchiveName: "archive2", UserObject: "obj2", Uses: []string{"a", "b"}, Imports: []string{"c", "d"}}},
	}
	for index, deployment := range deployments {
		if deployment.Module() != want[index].name || len(deployment.Statements()) != 2 {
			t.Fatalf("deployment %d module=%q statements=%d", index, deployment.Module(), len(deployment.Statements()))
		}
		if metadata := deployment.ModuleMetadata(); !reflect.DeepEqual(metadata, want[index].metadata) {
			t.Fatalf("deployment %q metadata = %#v, want %#v", deployment.Module(), metadata, want[index].metadata)
		}
		if deployment.LastUpdatedAt().IsZero() {
			t.Fatalf("deployment %q has zero update time", deployment.Module())
		}
	}
	detached := deploymentTwo.ModuleMetadata()
	detached.Uses[0] = "mutated"
	detached.Imports[0] = "mutated"
	if metadata := deploymentTwo.ModuleMetadata(); !reflect.DeepEqual(metadata.Uses, []string{"a", "b"}) || !reflect.DeepEqual(metadata.Imports, []string{"c", "d"}) {
		t.Fatalf("deployment metadata was not detached: %#v", metadata)
	}
}

func TestClientCompileModuleTextParsingIsIntentionallyNotExposed(t *testing.T) {
	env := newClientCompileModuleEnvironment(t)
	module, err := env.RegisterModule(
		"org.mycompany.events",
		WithModuleUses("seconds.until.every.where", "seconds.until.every.where"),
		WithModuleImports("com.mycompany.pck1", "com.mycompany.*"),
	)
	if err != nil {
		t.Fatal(err)
	}
	metadata := module.Metadata()
	if !reflect.DeepEqual(metadata.Uses, []string{"seconds.until.every.where"}) || !reflect.DeepEqual(metadata.Imports, []string{"com.mycompany.pck1", "com.mycompany.*"}) {
		t.Fatalf("typed module declarations = %#v", metadata)
	}
	plan, err := module.Build(
		Select(
			From[clientCompileModuleEvent](env, "SupportBean"),
			Alias("value", Field[clientCompileModuleEvent, int]("intPrimitive")),
		).Query(StatementName("typed-module-item")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.Query().ModuleUses(); !reflect.DeepEqual(got, []string{"seconds.until.every.where"}) {
		t.Fatalf("typed module plan uses = %v", got)
	}
	engine := NewEngine(env)
	deployment, err := engine.DeployPlans(context.Background(), []Plan{plan})
	if err != nil {
		t.Fatal(err)
	}
	if deployment.Module() != "org.mycompany.events" || len(deployment.Statements()) != 1 {
		t.Fatalf("typed module deployment = module %q statements %d", deployment.Module(), len(deployment.Statements()))
	}
	if _, err := engine.DeployPlans(context.Background(), nil); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("empty typed module deployment error = %v", err)
	}
	for _, testCase := range []struct {
		name    string
		options []ModuleOption
	}{
		{name: "blank-uses", options: []ModuleOption{WithModuleUses(" ")}},
		{name: "blank-import", options: []ModuleOption{WithModuleImports(" ")}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := NewEnvironment().RegisterModule("typed.invalid", testCase.options...); err == nil || !errors.Is(err, ErrorInvalidRule) {
				t.Fatalf("typed module declaration error = %v", err)
			}
		})
	}
}
