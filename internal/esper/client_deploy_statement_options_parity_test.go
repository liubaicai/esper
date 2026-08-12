package esper

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type clientDeployStatementOptionEvent struct {
	Value string `esper:"value"`
}

type clientDeployRuntimeUserObject struct {
	ID string
}

func newClientDeployStatementOptionEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientDeployStatementOptionEvent](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func TestClientDeployStatementNameResolveContextMatchesEsper(t *testing.T) {
	env := newClientDeployStatementOptionEnvironment(t)
	plan, err := env.Build(
		Select(
			From[clientDeployStatementOptionEvent](env, "SupportBean"),
			Alias("value", Field[clientDeployStatementOptionEvent, string]("value")),
		).Query(StatementName("s0"), StatementDescription("runtime name context")),
	)
	if err != nil {
		t.Fatal(err)
	}
	var captured DeploymentStatementNameContext
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan,
		WithDeploymentID("deploy-name-context"),
		WithDeploymentStatementNameResolver(func(resolverContext DeploymentStatementNameContext) (string, error) {
			captured = resolverContext
			return "hello", nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if captured.Index != 0 || captured.OriginalName != "s0" || captured.DeploymentID != deployment.ID() {
		t.Fatalf("statement-name resolver identity = %#v", captured)
	}
	if captured.Plan.Hash() != plan.Hash() || !strings.Contains(captured.TypedDescription, "SupportBean") {
		t.Fatalf("statement-name resolver plan/description = %q / %q", captured.Plan.Hash(), captured.TypedDescription)
	}
	if captured.Metadata.Name != "s0" || !captured.Metadata.HasDescription || captured.Metadata.Description != "runtime name context" {
		t.Fatalf("statement-name resolver metadata = %#v", captured.Metadata)
	}
	statement, ok := deployment.Statement("hello")
	if !ok || statement.Name() != "hello" || statement.DeploymentID() != deployment.ID() || statement.Metadata().Name != "hello" {
		t.Fatalf("resolved statement = %#v, found=%t", statement, ok)
	}

	var values []string
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			values = append(values, result.Get("value").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientDeployStatementOptionEvent{Value: "E1"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values, []string{"E1"}) {
		t.Fatalf("runtime-renamed statement values = %v", values)
	}
}

func TestClientDeployUserObjectValuesMatchEsper(t *testing.T) {
	testCases := []struct {
		name  string
		value any
	}{
		{name: "nil", value: nil},
		{name: "string", value: "ABC"},
		{name: "struct", value: clientDeployRuntimeUserObject{ID: "hello"}},
	}
	for index, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			env := newClientDeployStatementOptionEnvironment(t)
			plan, err := env.Build(
				From[clientDeployStatementOptionEvent](env, "SupportBean").Query(
					StatementName("s0"),
					WithStatementUserObject("compile-default"),
				),
			)
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			deployment, err := engine.Deploy(context.Background(), plan,
				WithDeploymentID("deploy-user-value-"+strconv.Itoa(index)),
				WithDeploymentStatementUserObjectResolver(func(DeploymentStatementUserObjectContext) (any, error) {
					return testCase.value, nil
				}),
			)
			if err != nil {
				t.Fatal(err)
			}
			if got := deployment.Statements()[0].UserObject(); !reflect.DeepEqual(got, testCase.value) {
				t.Fatalf("deployment user object = %#v, want %#v", got, testCase.value)
			}
		})
	}
}

func TestClientDeployUserObjectResolveContextMatchesEsper(t *testing.T) {
	env := newClientDeployStatementOptionEnvironment(t)
	plan, err := env.Build(
		Select(
			From[clientDeployStatementOptionEvent](env, "SupportBean"),
			Alias("ctx", CurrentEvaluationContext()),
		).Query(
			StatementName("s0"),
			StatementDescription("runtime user context"),
			WithStatementUserObject("compile-default"),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	wantUserObject := clientDeployRuntimeUserObject{ID: "runtime-owner"}
	var captured DeploymentStatementUserObjectContext
	engine := NewEngine(env, WithRuntimeURI("deploy-options"))
	deployment, err := engine.Deploy(context.Background(), plan,
		WithDeploymentID("deploy-user-context"),
		WithDeploymentStatementNameResolver(func(DeploymentStatementNameContext) (string, error) {
			return "hello", nil
		}),
		WithDeploymentStatementUserObjectResolver(func(resolverContext DeploymentStatementUserObjectContext) (any, error) {
			captured = resolverContext
			return wantUserObject, nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if captured.Index != 0 || captured.OriginalName != "s0" || captured.StatementName != "hello" ||
		captured.DeploymentID != deployment.ID() || captured.StatementID != deployment.ID()+":hello" {
		t.Fatalf("user-object resolver identity = %#v", captured)
	}
	if captured.Plan.Hash() != plan.Hash() || !strings.Contains(captured.TypedDescription, "SupportBean") ||
		!captured.Metadata.HasDescription || captured.Metadata.Description != "runtime user context" || captured.Metadata.Name != "s0" {
		t.Fatalf("user-object resolver plan/metadata = %#v", captured)
	}
	statement := deployment.Statements()[0]
	if statement.Name() != "hello" || !reflect.DeepEqual(statement.UserObject(), wantUserObject) {
		t.Fatalf("runtime statement name/user object = %q / %#v", statement.Name(), statement.UserObject())
	}

	var evaluated ExpressionEvaluationContext
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) == 0 {
			return nil
		}
		value, valueErr := As[ExpressionEvaluationContext](batch.New[0].Get("ctx"))
		if valueErr != nil {
			return valueErr
		}
		evaluated = value
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientDeployStatementOptionEvent{Value: "E1"}); err != nil {
		t.Fatal(err)
	}
	if evaluated.RuntimeURI != "deploy-options" || evaluated.StatementName != "hello" ||
		!reflect.DeepEqual(evaluated.StatementUserObject, wantUserObject) {
		t.Fatalf("runtime evaluation context = %#v", evaluated)
	}

	resolverErr := errors.New("user object resolver failed")
	failedEngine := NewEngine(env)
	if _, err := failedEngine.Deploy(context.Background(), plan,
		WithDeploymentID("deploy-user-failure"),
		WithDeploymentStatementUserObjectResolver(func(DeploymentStatementUserObjectContext) (any, error) {
			return nil, resolverErr
		}),
	); !errors.Is(err, resolverErr) || !errors.Is(err, ErrorDeployment) {
		t.Fatalf("user-object resolver failure = %v", err)
	}
	if active := failedEngine.RuntimePath().DeploymentIDs(); len(active) != 0 {
		t.Fatalf("failed user-object resolver left deployments = %v", active)
	}
}

func TestClientDeployClassLoaderOptionIsIntentionallyNotExposed(t *testing.T) {
	env := newClientDeployStatementOptionEnvironment(t)
	plan, err := env.Build(
		From[clientDeployStatementOptionEvent](env, "SupportBean").Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	manifest := plan.Manifest()
	if manifest.Provider != compilerProvider || manifest.CompilerVersion != CompilerVersion {
		t.Fatalf("Go-native plan manifest = %#v", manifest)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if statement, ok := deployment.Statement("s0"); !ok || statement.Plan().Hash() != plan.Hash() {
		t.Fatalf("Go-native deployment statement = %#v, found=%t", statement, ok)
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}
