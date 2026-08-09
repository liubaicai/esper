package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type clientCompileOptionsEvent struct {
	P0 string `esper:"p0"`
}

type clientCompileUserObject struct {
	ID string
}

func newClientCompileOptionsEnvironment(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientCompileOptionsEvent](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

func TestClientCompileStatementNameResolverContextAndDeploymentMatchEsper(t *testing.T) {
	env, engine := newClientCompileOptionsEnvironment(t)
	query := From[clientCompileOptionsEvent](env, "SupportBean").Query()
	var captured StatementCompileContext
	plan, err := env.Build(query, WithStatementNameResolver(func(ctx StatementCompileContext) (string, error) {
		captured = ctx
		return "hello", nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if captured.StatementNumber != 0 || captured.StatementName != "" || captured.ModuleName != "" {
		t.Fatalf("statement-name context identity = %#v", captured)
	}
	if captured.Query.Name() != "" || captured.Metadata.Name != "" || len(captured.Metadata.Annotations) != 0 {
		t.Fatalf("statement-name context query/metadata = %#v / %#v", captured.Query, captured.Metadata)
	}
	if !strings.Contains(captured.TypedDescription, "SupportBean") {
		t.Fatalf("typed description = %q, want SupportBean source", captured.TypedDescription)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	if statement.Name() != "hello" || statement.Metadata().Name != "hello" {
		t.Fatalf("resolved statement name = %q metadata=%#v", statement.Name(), statement.Metadata())
	}
}

func TestClientCompileUserObjectTypesAndResolverContextMatchEsper(t *testing.T) {
	cases := []struct {
		name  string
		value any
	}{
		{name: "string", value: "ABC"},
		{name: "slice", value: []int{1, 2, 3}},
		{name: "nil", value: nil},
		{name: "struct", value: clientCompileUserObject{ID: "hello"}},
	}
	for index, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env, engine := newClientCompileOptionsEnvironment(t)
			query := From[clientCompileOptionsEvent](env, "SupportBean").Query(StatementName("s0"))
			var captured StatementCompileContext
			plan, err := env.Build(query, WithStatementUserObjectResolver(func(ctx StatementCompileContext) (any, error) {
				captured = ctx
				return testCase.value, nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			if captured.StatementNumber != 0 || captured.StatementName != "s0" || captured.ModuleName != "" {
				t.Fatalf("user-object context identity = %#v", captured)
			}
			if captured.Query.Name() != "s0" || captured.Metadata.Name != "s0" || !strings.Contains(captured.TypedDescription, "SupportBean") {
				t.Fatalf("user-object context query/metadata = %#v / %#v", captured.Query, captured.Metadata)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			statement := deployment.Statements()[0]
			if !reflect.DeepEqual(statement.UserObject(), testCase.value) {
				t.Fatalf("case %d user object = %#v, want %#v", index, statement.UserObject(), testCase.value)
			}
		})
	}

	env, _ := newClientCompileOptionsEnvironment(t)
	query := From[clientCompileOptionsEvent](env, "SupportBean").Query()
	var userContext StatementCompileContext
	plan, err := env.Build(query,
		WithStatementNameResolver(func(StatementCompileContext) (string, error) { return "resolved", nil }),
		WithStatementUserObjectResolver(func(ctx StatementCompileContext) (any, error) {
			userContext = ctx
			return "owner", nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if userContext.StatementName != "resolved" || plan.Query().Name() != "resolved" {
		t.Fatalf("resolver order context=%#v plan-name=%q", userContext, plan.Query().Name())
	}
}

func TestClientCompileResolversRejectErrorsAndBlankNames(t *testing.T) {
	env, _ := newClientCompileOptionsEnvironment(t)
	query := From[clientCompileOptionsEvent](env, "SupportBean").Query()
	resolverErr := errors.New("resolver failed")
	if _, err := env.Build(query, WithStatementNameResolver(func(StatementCompileContext) (string, error) {
		return "", resolverErr
	})); !errors.Is(err, resolverErr) || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("statement-name resolver error = %v", err)
	}
	if _, err := env.Build(query, WithStatementUserObjectResolver(func(StatementCompileContext) (any, error) {
		return nil, resolverErr
	})); !errors.Is(err, resolverErr) || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("user-object resolver error = %v", err)
	}
	if _, err := env.Build(query, WithStatementNameResolver(func(StatementCompileContext) (string, error) {
		return "   ", nil
	})); err == nil {
		t.Fatal("blank resolved statement name unexpectedly built")
	}
}

func TestClientCompileOutputManifestMatchesEsperBoundary(t *testing.T) {
	env, _ := newClientCompileOptionsEnvironment(t)
	plan, err := env.Build(From[clientCompileOptionsEvent](env, "SupportBean").Query())
	if err != nil {
		t.Fatal(err)
	}
	manifest := plan.Manifest()
	if manifest.CompilerVersion != CompilerVersion {
		t.Fatalf("compiler version = %q, want %q", manifest.CompilerVersion, CompilerVersion)
	}
	if manifest.Provider != compilerProvider || strings.TrimSpace(manifest.Provider) == "" {
		t.Fatalf("compiler provider = %q", manifest.Provider)
	}
	if zero := (Plan{}).Manifest(); zero != (PlanManifest{}) {
		t.Fatalf("zero Plan manifest = %#v", zero)
	}
}
