package esper

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
)

const clientRuntimeTestServiceName = "TEST_SERVICE_NAME"

type clientRuntimeLocalService struct {
	secret int
}

type clientRuntimeAnonymousBean struct {
	Prop int `esper:"prop"`
}

func TestClientRuntimeItselfTransientConfigurationParity(t *testing.T) {
	env, _ := newRuntimeTest(t)
	engine := NewEngine(env, WithRuntimeService(clientRuntimeTestServiceName, &clientRuntimeLocalService{secret: 12345}))
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	secret := 0
	if _, err := deployment.Statements()[0].Subscribe(func(context.Context, ResultBatch) error {
		service, ok := engine.RuntimeService(clientRuntimeTestServiceName)
		if !ok {
			return NewError(ErrorDependency, "runtime service is missing")
		}
		secret = service.(*clientRuntimeLocalService).secret
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E1"}); err != nil {
		t.Fatal(err)
	}
	if secret != 12345 {
		t.Fatalf("runtime service secret = %d, want 12345", secret)
	}
	if _, ok := engine.RuntimeService("missing"); ok {
		t.Fatal("missing runtime service unexpectedly resolved")
	}
}

func TestClientRuntimeSPIStatementSelectionParity(t *testing.T) {
	env, engine := newRuntimeTest(t)
	source := From[runtimeTestTrade](env, "Trade")
	plans := []Query{
		source.Query(StatementName("a")),
		source.Filter(Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("xxx"))).Query(StatementName("b")),
	}
	for _, query := range plans {
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}

	var traversed []string
	if err := engine.TraverseStatements(context.Background(), func(deployment *Deployment, statement *Statement) error {
		if deployment.ID() != statement.DeploymentID() {
			return NewError(ErrorState, "statement deployment metadata does not match")
		}
		traversed = append(traversed, statement.Name())
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(traversed, []string{"a", "b"}) {
		t.Fatalf("traversed statements = %v, want [a b]", traversed)
	}
	assertClientRuntimeStatementNames(t, engine, []string{"b"}, StatementNameEquals("b"))
	assertClientRuntimeStatementNames(t, engine, []string{"b"}, StatementContains("xxx"))
	assertClientRuntimeStatementNames(t, engine, nil, StatementDeploymentIDContains("x"))
	if _, err := engine.Statements(context.Background(), nil); !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("nil statement predicate error = %v, want %s", err, ErrorInvalidRule)
	}
}

func TestClientRuntimeSPIBeanAnonymousTypeParity(t *testing.T) {
	schema, err := StructSchema[clientRuntimeAnonymousBean]("anonymous-client-runtime-bean")
	if err != nil {
		t.Fatal(err)
	}
	propertyType, ok := schema.PropertyType("prop")
	if !ok || propertyType != reflect.TypeOf(int(0)) {
		t.Fatalf("anonymous bean prop type = (%v, %t), want int", propertyType, ok)
	}
}

func TestClientRuntimeSPICompileReflectiveTypedParity(t *testing.T) {
	env, engine := newClientRuntimeItselfWindow(t)
	window := FromNamedWindow(env, "MyWindow")
	projection := []Selection{Alias("symbol", Field[any, string]("symbol"))}

	fafPlan, err := env.Build(window.Select(projection...).Query(StatementName("faf")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.ExecuteFireAndForget(context.Background(), fafPlan)
	if err != nil {
		t.Fatal(err)
	}
	assertClientRuntimeSymbolResult(t, result, "E1")

	for _, name := range []string{"s0", "s1"} {
		plan, err := env.Build(window.Select(projection...).Query(StatementName(name)))
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Canonical()) == 0 {
			t.Fatalf("typed plan %s has no canonical representation", name)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		assertClientRuntimeSymbolResult(t, snapshot, "E1")
	}
	if got := Multiply[int](Literal(1), Literal(1)).eval(EvalContext{}); !got.Equal(Present(1)) {
		t.Fatalf("typed expression 1*1 = %v, want 1", got)
	}
}

func TestClientRuntimeWrongCompileMethodApprovedDifference(t *testing.T) {
	env, engine := newClientRuntimeItselfWindow(t)
	plan, err := env.Build(FromNamedWindow(env, "MyWindow").Select(
		Alias("symbol", Field[any, string]("symbol")),
	).Query(StatementName("shared-plan")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	assertClientRuntimeSymbolResult(t, result, "E1")
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertClientRuntimeSymbolResult(t, snapshot, "E1")
}

func newClientRuntimeItselfWindow(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env, _ := newRuntimeTest(t)
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "MyWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	source := From[runtimeTestTrade](env, "Trade")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow("MyWindow",
		SetColumn("symbol", Field[runtimeTestTrade, string]("symbol")),
		SetColumn("price", Field[runtimeTestTrade, float64]("price")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E1", Price: 10}); err != nil {
		t.Fatal(err)
	}
	return env, engine
}

func assertClientRuntimeStatementNames(t *testing.T, engine *Engine, want []string, predicates ...StatementPredicate) {
	t.Helper()
	statements, err := engine.Statements(context.Background(), predicates...)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(statements))
	for index, statement := range statements {
		names[index] = statement.Name()
	}
	if !slices.Equal(names, want) {
		t.Fatalf("selected statements = %v, want %v", names, want)
	}
}

func assertClientRuntimeSymbolResult(t *testing.T, result QueryResult, want string) {
	t.Helper()
	results := result.Results()
	if len(results) != 1 || results[0].Get("symbol").Any() != want {
		t.Fatalf("query results = %#v, want symbol=%q", results, want)
	}
}
