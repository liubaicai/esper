package esper

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type clientDeployVersionEvent struct {
	Value string `esper:"value"`
}

func TestClientDeployVersionMinorCheckMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[clientDeployVersionEvent](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(
		Select(
			From[clientDeployVersionEvent](env, "SupportBean"),
			Alias("value", Field[clientDeployVersionEvent, string]("value")),
		).Query(StatementName("version-check")),
	)
	if err != nil {
		t.Fatal(err)
	}

	legacy := plan
	legacy.compilerVersion = "esper-go-compiler/v0"
	if manifest := legacy.Manifest(); manifest.CompilerVersion != "esper-go-compiler/v0" {
		t.Fatalf("legacy manifest = %#v", manifest)
	}

	engine := NewEngine(env)
	assertCompatibilityError(t, func() error {
		_, err := engine.Deploy(context.Background(), legacy)
		return err
	}(), false, 0)
	assertCompatibilityError(t, func() error {
		_, err := engine.DeployPlans(context.Background(), []Plan{legacy})
		return err
	}(), true, 0)
	rolloutErr := func() error {
		_, err := engine.Rollout(context.Background(), RolloutPlans(legacy))
		return err
	}()
	assertCompatibilityError(t, rolloutErr, false, 0)
	var indexedRolloutErr *DeploymentRolloutError
	if !errors.As(rolloutErr, &indexedRolloutErr) || indexedRolloutErr.RolloutItemIndex() != 0 {
		t.Fatalf("rollout compatibility item = %#v", indexedRolloutErr)
	}
	assertCompatibilityError(t, func() error {
		_, err := engine.ExecuteFireAndForget(context.Background(), legacy)
		return err
	}(), false, 0)

	if active := engine.RuntimePath().DeploymentIDs(); len(active) != 0 {
		t.Fatalf("incompatible plans left active deployments: %v", active)
	}

	encoded, err := legacy.MarshalArtifact()
	if err != nil {
		t.Fatal(err)
	}
	_, err = LoadPlanArtifact(encoded)
	assertCompatibilityError(t, err, false, 0)

	legacySchema := plan
	legacySchema.schemaVersion = "esper-go-plan/v1"
	encoded, err = legacySchema.MarshalArtifact()
	if err != nil {
		t.Fatal(err)
	}
	_, err = LoadPlanArtifact(encoded)
	assertCompatibilityError(t, err, false, 0)
}

func assertCompatibilityError(t *testing.T, err error, wantIndex bool, wantPlanIndex int) {
	t.Helper()
	if err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("compatibility error = %v, want %s", err, ErrorDependency)
	}
	var failure *PlanCompatibilityError
	if !errors.As(err, &failure) {
		t.Fatalf("compatibility error type = %T, want *PlanCompatibilityError", err)
	}
	if failure.RuntimeSchemaVersion != planSchemaVersion || failure.RuntimeCompilerVersion != CompilerVersion {
		t.Fatalf("runtime versions = schema %q compiler %q", failure.RuntimeSchemaVersion, failure.RuntimeCompilerVersion)
	}
	if !strings.Contains(err.Error(), planSchemaVersion) ||
		!strings.Contains(err.Error(), CompilerVersion) ||
		!strings.Contains(err.Error(), failure.PlanSchemaVersion) ||
		!strings.Contains(err.Error(), failure.PlanCompilerVersion) {
		t.Fatalf("compatibility message does not identify both contracts: %v", err)
	}
	index, hasIndex := failure.DeploymentPlanIndex()
	if hasIndex != wantIndex || (hasIndex && index != wantPlanIndex) {
		t.Fatalf("deployment plan index = %d, %t; want %d, %t", index, hasIndex, wantPlanIndex, wantIndex)
	}
}
