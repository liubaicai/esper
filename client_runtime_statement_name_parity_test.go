package esper

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type clientRuntimeStatementNameEvent struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func TestClientRuntimeStatementAllowNameDuplicateParity(t *testing.T) {
	env, engine := newRuntimeTest(t)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("a")))
	if err != nil {
		t.Fatal(err)
	}
	first, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	firstStatement := first.Statements()[0]
	secondStatement := second.Statements()[0]
	if firstStatement.Name() != "a" || secondStatement.Name() != "a" {
		t.Fatalf("duplicate runtime names = %q, %q", firstStatement.Name(), secondStatement.Name())
	}
	if first.ID() == second.ID() || firstStatement.ID() == secondStatement.ID() {
		t.Fatal("same-name statements do not have distinct deployment identities")
	}
	var firstCount, secondCount int
	if _, err := firstStatement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		firstCount += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := secondStatement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		secondCount += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E1"}); err != nil {
		t.Fatal(err)
	}
	if firstCount != 1 || secondCount != 1 {
		t.Fatalf("same-name delivery counts = %d, %d", firstCount, secondCount)
	}
	if err := first.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E2"}); err != nil {
		t.Fatal(err)
	}
	if firstCount != 1 || secondCount != 2 {
		t.Fatalf("post-undeploy same-name delivery counts = %d, %d", firstCount, secondCount)
	}
}

func TestClientRuntimeSingleModuleTwoStatementsNoDependencyParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[clientRuntimeStatementNameEvent](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	source := From[clientRuntimeStatementNameEvent](env, "SupportBean")
	intPlan, err := env.Build(Select(source,
		Alias("intPrimitive", Field[clientRuntimeStatementNameEvent, int]("intPrimitive")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	stringPlan, err := env.Build(Select(source,
		Alias("theString", Field[clientRuntimeStatementNameEvent, string]("theString")),
	).Query(StatementName("s1")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.DeployPlans(context.Background(), []Plan{intPlan, stringPlan})
	if err != nil {
		t.Fatal(err)
	}
	statements := deployment.Statements()
	if len(statements) != 2 || statements[0].Name() != "s0" || statements[1].Name() != "s1" {
		t.Fatalf("module statements = %#v", clientRuntimeStatementNames(statements))
	}
	if statements[0].DeploymentID() != deployment.ID() || statements[1].DeploymentID() != deployment.ID() {
		t.Fatal("multi-plan statements do not share one deployment")
	}
	var ints []int
	var stringsSeen []string
	if _, err := statements[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			ints = append(ints, row.Get("intPrimitive").Any().(int))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := statements[1].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			stringsSeen = append(stringsSeen, row.Get("theString").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []clientRuntimeStatementNameEvent{{TheString: "E1", IntPrimitive: 10}, {TheString: "E2", IntPrimitive: 20}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(ints, []int{10, 20}) || !reflect.DeepEqual(stringsSeen, []string{"E1", "E2"}) {
		t.Fatalf("module results ints=%v strings=%v", ints, stringsSeen)
	}
}

func TestClientRuntimeStatementNameUnassignedParity(t *testing.T) {
	env, engine := newRuntimeTest(t)
	first, err := env.Build(From[runtimeTestTrade](env, "Trade").Query())
	if err != nil {
		t.Fatal(err)
	}
	second, err := env.Build(Select(From[runtimeTestTrade](env, "Trade"),
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
	).Query())
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.DeployPlans(context.Background(), []Plan{first, second})
	if err != nil {
		t.Fatal(err)
	}
	statements := deployment.Statements()
	if got := clientRuntimeStatementNames(statements); !reflect.DeepEqual(got, []string{"stmt-0", "stmt-1"}) {
		t.Fatalf("unassigned names = %v, want [stmt-0 stmt-1]", got)
	}
	if statement, ok := deployment.Statement("stmt-1"); !ok || statement != statements[1] {
		t.Fatalf("deployment statement lookup = (%p, %t), want %p", statement, ok, statements[1])
	}
	if _, ok := deployment.Statement("missing"); ok {
		t.Fatal("missing deployment statement unexpectedly resolved")
	}

	single, err := engine.Deploy(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	if got := single.Statements()[0].Name(); got != "stmt-0" {
		t.Fatalf("single unassigned name = %q, want stmt-0", got)
	}
}

func TestClientRuntimeStatementNameRuntimeResolverDuplicateParity(t *testing.T) {
	env, engine := newRuntimeTest(t)
	first, err := env.Build(From[runtimeTestTrade](env, "Trade").Query())
	if err != nil {
		t.Fatal(err)
	}
	second, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("compiled")))
	if err != nil {
		t.Fatal(err)
	}
	var contexts []DeploymentStatementNameContext
	_, err = engine.DeployPlans(context.Background(), []Plan{first, second},
		WithDeploymentStatementNameResolver(func(ctx DeploymentStatementNameContext) (string, error) {
			contexts = append(contexts, ctx)
			return "x", nil
		}),
	)
	if err == nil || !errors.Is(err, ErrorDeployment) {
		t.Fatalf("duplicate resolver error = %v, want %s", err, ErrorDeployment)
	}
	if len(contexts) != 2 || contexts[0].Index != 0 || contexts[0].OriginalName != "" || contexts[1].Index != 1 || contexts[1].OriginalName != "compiled" {
		t.Fatalf("resolver contexts = %#v", contexts)
	}
	if statements, statementsErr := engine.Statements(context.Background()); statementsErr != nil || len(statements) != 0 {
		t.Fatalf("failed resolver left statements = %v, err=%v", clientRuntimeStatementNames(statements), statementsErr)
	}

	deployment, err := engine.DeployPlans(context.Background(), []Plan{first, second},
		WithDeploymentStatementNameResolver(func(ctx DeploymentStatementNameContext) (string, error) {
			return "runtime-" + string(rune('0'+ctx.Index)), nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := clientRuntimeStatementNames(deployment.Statements()); !reflect.DeepEqual(got, []string{"runtime-0", "runtime-1"}) {
		t.Fatalf("resolved runtime names = %v", got)
	}

	if _, err := engine.DeployPlans(context.Background(), nil); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("empty deployment error = %v", err)
	}
	duplicateNamePlan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("same")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.DeployPlans(context.Background(), []Plan{duplicateNamePlan, duplicateNamePlan}); err == nil || !errors.Is(err, ErrorDeployment) {
		t.Fatalf("same-deployment duplicate name error = %v", err)
	}
	resolverErr := errors.New("resolver failed")
	if _, err := engine.DeployPlans(context.Background(), []Plan{first}, WithDeploymentStatementNameResolver(func(DeploymentStatementNameContext) (string, error) {
		return "", resolverErr
	})); !errors.Is(err, resolverErr) {
		t.Fatalf("resolver callback error = %v, want %v", err, resolverErr)
	}
}

func clientRuntimeStatementNames(statements []*Statement) []string {
	names := make([]string, len(statements))
	for index, statement := range statements {
		if statement != nil {
			names[index] = statement.Name()
		}
	}
	return names
}
