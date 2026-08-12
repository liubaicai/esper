package esper

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

type clientCompileToolBean struct {
	Value string `esper:"value"`
}

type clientCompileRecordingProvider struct {
	calls       int
	environment *Environment
	description string
}

func (provider *clientCompileRecordingProvider) Compile(request CompilerRequest) (Plan, error) {
	provider.calls++
	provider.environment = request.Environment()
	provider.description = request.Description()
	return request.CompileDefault()
}

func TestClientCompileToolCompilerBasicMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[clientCompileToolBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	query := From[clientCompileToolBean](env, "SupportBean").Query(StatementName("s0"))
	provider := &clientCompileRecordingProvider{}
	plan, err := CompileWithProvider(env, query, provider)
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 || provider.environment != env || !strings.Contains(provider.description, "SupportBean") {
		t.Fatalf("compiler provider calls=%d env=%p description=%q", provider.calls, provider.environment, provider.description)
	}
	if plan.Query().Name() != "s0" || plan.Hash() == "" || plan.Manifest().CompilerVersion != CompilerVersion {
		t.Fatalf("provider plan name=%q hash=%q manifest=%#v", plan.Query().Name(), plan.Hash(), plan.Manifest())
	}

	native, err := CompileWithProvider(env, query, NativeCompilerProvider{})
	if err != nil {
		t.Fatal(err)
	}
	if native.Hash() != plan.Hash() {
		t.Fatalf("native provider hash %q differs from wrapped provider %q", native.Hash(), plan.Hash())
	}
}

func TestCompileWithProviderRejectsInvalidProviderResults(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[clientCompileToolBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	query := From[clientCompileToolBean](env, "SupportBean").Query()

	if _, err := CompileWithProvider(nil, query, NativeCompilerProvider{}); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("nil environment error = %v", err)
	}
	if _, err := CompileWithProvider(env, query, nil); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("nil provider error = %v", err)
	}
	var nilPointer *clientCompileRecordingProvider
	if _, err := CompileWithProvider(env, query, nilPointer); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("typed nil provider error = %v", err)
	}
	var nilFunc CompilerProviderFunc
	if _, err := CompileWithProvider(env, query, nilFunc); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("nil provider function error = %v", err)
	}
	if _, err := CompileWithProvider(env, query, CompilerProviderFunc(func(CompilerRequest) (Plan, error) {
		return Plan{}, fmt.Errorf("provider failed")
	})); err == nil || !errors.Is(err, ErrorInvalidRule) || !strings.Contains(err.Error(), "provider failed") {
		t.Fatalf("provider failure error = %v", err)
	}
	if _, err := CompileWithProvider(env, query, CompilerProviderFunc(func(CompilerRequest) (Plan, error) {
		return Plan{}, nil
	})); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("empty provider plan error = %v", err)
	}

	other := NewEnvironment()
	if _, err := RegisterStruct[clientCompileToolBean](other, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	foreign, err := other.Build(From[clientCompileToolBean](other, "SupportBean").Query())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CompileWithProvider(env, query, CompilerProviderFunc(func(CompilerRequest) (Plan, error) {
		return foreign, nil
	})); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("foreign provider plan error = %v", err)
	}

	var empty CompilerRequest
	if _, err := empty.CompileDefault(); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("empty compiler request error = %v", err)
	}
}
