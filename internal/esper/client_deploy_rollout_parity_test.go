package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type clientDeployRolloutEvent struct {
	P string `esper:"p"`
}

func newClientDeployRolloutEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientDeployRolloutEvent](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func buildClientDeployRolloutPlan(t *testing.T, module Module, query Query) Plan {
	t.Helper()
	plan, err := module.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func clientDeployRolloutStatement(t *testing.T, deployment *Deployment, name string) *Statement {
	t.Helper()
	statement, ok := deployment.Statement(name)
	if !ok {
		t.Fatalf("statement %q is missing from deployment %q", name, deployment.ID())
	}
	return statement
}

func TestClientDeployRolloutTwoInterdependentModulesMatchesEsper(t *testing.T) {
	env := newClientDeployRolloutEnvironment(t)
	provider, err := env.RegisterModule("rollout.two.provider")
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := env.RegisterModule("rollout.two.consumer", WithModuleUses(provider.Name()))
	if err != nil {
		t.Fatal(err)
	}
	source := From[clientDeployRolloutEvent](env, "SupportBean")
	providerPlan := buildClientDeployRolloutPlan(t, provider, source.Query(StatementName("type")))
	consumerPlan := buildClientDeployRolloutPlan(t, consumer,
		Select(source, Alias("p", Field[clientDeployRolloutEvent, string]("p"))).Query(StatementName("s0")))

	engine := NewEngine(env)
	rollout, err := engine.Rollout(context.Background(),
		RolloutPlans(providerPlan).WithOptions(WithDeploymentID("provider")),
		RolloutPlans(consumerPlan).WithOptions(WithDeploymentID("consumer")),
	)
	if err != nil {
		t.Fatal(err)
	}
	items := rollout.Items()
	if len(items) != 2 || items[0].Deployment().ID() != "provider" || items[1].Deployment().ID() != "consumer" {
		t.Fatalf("rollout items = %#v", rollout.Deployments())
	}
	if dependencies := items[1].Deployment().Dependencies(); !reflect.DeepEqual(dependencies, []string{"provider"}) {
		t.Fatalf("consumer dependencies = %v", dependencies)
	}
	var values []string
	statement := clientDeployRolloutStatement(t, items[1].Deployment(), "s0")
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			values = append(values, result.Get("p").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"a", "b"} {
		if err := engine.SendEvent(context.Background(), clientDeployRolloutEvent{P: value}); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(values, []string{"a", "b"}) {
		t.Fatalf("rollout values = %v", values)
	}
}

func TestClientDeployRolloutFourInterdependentModulesAndStatementSequenceMatchesEsper(t *testing.T) {
	env := newClientDeployRolloutEnvironment(t)
	base, err := env.RegisterModule("rollout.four.base")
	if err != nil {
		t.Fatal(err)
	}
	child0, err := env.RegisterModule("rollout.four.child0", WithModuleUses(base.Name()))
	if err != nil {
		t.Fatal(err)
	}
	child1, err := env.RegisterModule("rollout.four.child1", WithModuleUses(base.Name()))
	if err != nil {
		t.Fatal(err)
	}
	child11, err := env.RegisterModule("rollout.four.child11", WithModuleUses(base.Name(), child1.Name()))
	if err != nil {
		t.Fatal(err)
	}
	source := From[clientDeployRolloutEvent](env, "SupportBean")
	basePlan := buildClientDeployRolloutPlan(t, base, source.Query(StatementName("basevar")))
	s0 := buildClientDeployRolloutPlan(t, child0,
		Select(source, Alias("basevar", Literal(1))).Query(StatementName("s0")))
	child1Variable := buildClientDeployRolloutPlan(t, child1, source.Query(StatementName("child1var")))
	s1 := buildClientDeployRolloutPlan(t, child1,
		Select(source, Alias("basevar", Literal(1)), Alias("child1var", Literal(2))).Query(StatementName("s1")))
	s2 := buildClientDeployRolloutPlan(t, child11,
		Select(source, Alias("basevar", Literal(1)), Alias("child1var", Literal(2))).Query(StatementName("s2")))

	engine := NewEngine(env)
	rollout, err := engine.Rollout(context.Background(),
		RolloutPlans(basePlan).WithOptions(WithDeploymentID("base")),
		RolloutPlans(s0).WithOptions(WithDeploymentID("child0")),
		RolloutPlans(child1Variable, s1).WithOptions(WithDeploymentID("child1")),
		RolloutPlans(s2).WithOptions(WithDeploymentID("child11")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployments := rollout.Deployments()
	if len(deployments) != 4 {
		t.Fatalf("rollout deployment count = %d", len(deployments))
	}
	wantNames := []string{"basevar", "s0", "child1var", "s1", "s2"}
	wantSequence := uint64(1)
	for _, deployment := range deployments {
		for _, statement := range deployment.Statements() {
			if statement.Name() != wantNames[wantSequence-1] || statement.Sequence() != wantSequence {
				t.Fatalf("statement sequence %d = %q/%d", wantSequence, statement.Name(), statement.Sequence())
			}
			wantSequence++
		}
	}

	counts := map[string]int{}
	for _, name := range []string{"s0", "s1", "s2"} {
		statement, ok := engine.Statement(map[string]string{"s0": "child0", "s1": "child1", "s2": "child11"}[name], name)
		if !ok {
			t.Fatalf("statement %q is missing", name)
		}
		capturedName := name
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			counts[capturedName] += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SendEvent(context.Background(), clientDeployRolloutEvent{P: "first"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(counts, map[string]int{"s0": 1, "s1": 1, "s2": 1}) {
		t.Fatalf("rollout listener counts = %v", counts)
	}

	child12, err := env.RegisterModule("rollout.four.child12", WithModuleUses(base.Name(), child1.Name()))
	if err != nil {
		t.Fatal(err)
	}
	s3 := buildClientDeployRolloutPlan(t, child12,
		Select(source, Alias("basevar", Literal(1)), Alias("child1var", Literal(2))).Query(StatementName("s3")))
	additional, err := engine.Rollout(context.Background(), RolloutPlans(s3).WithOptions(WithDeploymentID("child12")))
	if err != nil {
		t.Fatal(err)
	}
	additionalDeployment := additional.Deployments()[0]
	if dependencies := additionalDeployment.Dependencies(); !reflect.DeepEqual(dependencies, []string{"base", "child1"}) {
		t.Fatalf("s3 dependencies = %v", dependencies)
	}
	if sequence := additionalDeployment.Statements()[0].Sequence(); sequence != 6 {
		t.Fatalf("s3 sequence = %d", sequence)
	}

	active := append(deployments, additionalDeployment)
	for index := len(active) - 1; index >= 0; index-- {
		if err := active[index].Undeploy(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	standalonePlan, err := env.Build(source.Query(StatementName("standalone")))
	if err != nil {
		t.Fatal(err)
	}
	standalone, err := engine.Deploy(context.Background(), standalonePlan, WithDeploymentID("standalone"))
	if err != nil {
		t.Fatal(err)
	}
	if sequence := standalone.Statements()[0].Sequence(); sequence != 7 {
		t.Fatalf("standalone sequence = %d", sequence)
	}
	_, err = engine.Rollout(context.Background(), RolloutPlans(s0).WithOptions(WithDeploymentID("missing-base")))
	assertClientDeployRolloutError(t, err, 0, ErrorDependency)
	if got := engine.Deployments(); !reflect.DeepEqual(got, []*Deployment{standalone}) {
		t.Fatalf("failed dependency rollout changed active state = %#v", got)
	}
}

func TestClientDeployRolloutInvalidIsAtomicMatchesEsper(t *testing.T) {
	env := newClientDeployRolloutEnvironment(t)
	source := From[clientDeployRolloutEvent](env, "SupportBean")
	plain, err := env.Build(source.Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := env.Build(source.Query(StatementName("s1")))
	if err != nil {
		t.Fatal(err)
	}
	parameterized, err := env.Build(source.Filter(Equal[string](
		Field[clientDeployRolloutEvent, string]("p"), Parameter[string]("p"),
	)).Query(StatementName("parameterized")))
	if err != nil {
		t.Fatal(err)
	}
	missing, err := env.RegisterModule("rollout.invalid.consumer", WithModuleUses("rollout.invalid.missing"))
	if err != nil {
		t.Fatal(err)
	}
	missingPlan := buildClientDeployRolloutPlan(t, missing, source.Query(StatementName("missing")))
	protected, err := env.RegisterModule("rollout.invalid.protected", ProtectedModule())
	if err != nil {
		t.Fatal(err)
	}
	protectedPlan := buildClientDeployRolloutPlan(t, protected, source.Query(StatementName("protected")))

	engine := NewEngine(env)
	baselineDeployment, err := engine.Deploy(context.Background(), baseline, WithDeploymentID("baseline"))
	if err != nil {
		t.Fatal(err)
	}
	var failedRolloutEvents []DeploymentStateEvent
	if err := engine.AddDeploymentStateListener(DeploymentStateListenerFunc(func(event DeploymentStateEvent) {
		failedRolloutEvents = append(failedRolloutEvents, event)
	})); err != nil {
		t.Fatal(err)
	}
	assertBaselineOnly := func() {
		t.Helper()
		if got := engine.Deployments(); !reflect.DeepEqual(got, []*Deployment{baselineDeployment}) {
			t.Fatalf("invalid rollout changed active deployments = %#v", got)
		}
		if len(failedRolloutEvents) != 0 {
			t.Fatalf("invalid rollout emitted lifecycle events = %#v", failedRolloutEvents)
		}
	}

	_, err = engine.Rollout(context.Background(),
		RolloutPlans(plain).WithOptions(WithDeploymentID("first")),
		RolloutPlans(missingPlan).WithOptions(WithDeploymentID("missing")),
	)
	assertClientDeployRolloutError(t, err, 1, ErrorDependency)
	assertBaselineOnly()

	_, err = engine.Rollout(context.Background(),
		RolloutPlans(protectedPlan).WithOptions(WithDeploymentID("protected-a")),
		RolloutPlans(protectedPlan).WithOptions(WithDeploymentID("protected-b")),
	)
	assertClientDeployRolloutError(t, err, 1, ErrorDeployment)
	assertBaselineOnly()

	_, err = engine.Rollout(context.Background(),
		RolloutPlans(plain).WithOptions(WithDeploymentID("duplicate")),
		RolloutPlans(plain).WithOptions(WithDeploymentID("duplicate")),
	)
	assertClientDeployRolloutError(t, err, 1, ErrorDeployment)
	assertBaselineOnly()

	_, err = engine.Rollout(context.Background(),
		RolloutPlans(plain).WithOptions(WithDeploymentID("new")),
		RolloutPlans(plain).WithOptions(WithDeploymentID("baseline")),
	)
	assertClientDeployRolloutError(t, err, 1, ErrorDeployment)
	assertBaselineOnly()

	_, err = engine.Rollout(context.Background(),
		RolloutPlans(plain).WithOptions(WithDeploymentID("new")),
		RolloutPlans(parameterized).WithOptions(WithDeploymentID("parameterized")),
	)
	assertClientDeployRolloutError(t, err, 1, ErrorInvalidRule)
	assertBaselineOnly()
}

func TestClientDeployStateListenerMatchesEsper(t *testing.T) {
	env := newClientDeployRolloutEnvironment(t)
	source := From[clientDeployRolloutEvent](env, "SupportBean")
	build := func(name string) Plan {
		t.Helper()
		plan, err := env.Build(source.Query(StatementName(name)))
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	engine := NewEngine(env, WithRuntimeURI("default"))
	var events []DeploymentStateEvent
	listener := DeploymentStateListenerFunc(func(event DeploymentStateEvent) {
		events = append(events, event)
	})
	if err := engine.AddDeploymentStateListener(listener); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), build("s0"), WithDeploymentID("ordinary"))
	if err != nil {
		t.Fatal(err)
	}
	assertClientDeployStateEvent(t, events, 0, DeploymentStateDeployed, "ordinary", 1, -1)
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertClientDeployStateEvent(t, events, 1, DeploymentStateUndeployed, "ordinary", 1, -1)
	if listeners := engine.DeploymentStateListeners(); len(listeners) != 1 {
		t.Fatalf("deployment listeners = %d", len(listeners))
	}
	engine.RemoveDeploymentStateListener(listener)
	if listeners := engine.DeploymentStateListeners(); len(listeners) != 0 {
		t.Fatalf("deployment listener remove left %d", len(listeners))
	}
	if err := engine.AddDeploymentStateListener(listener); err != nil {
		t.Fatal(err)
	}
	engine.RemoveDeploymentStateListeners()
	if listeners := engine.DeploymentStateListeners(); len(listeners) != 0 {
		t.Fatalf("remove-all deployment listeners left %d", len(listeners))
	}
	if err := engine.AddDeploymentStateListener(listener); err != nil {
		t.Fatal(err)
	}

	rollout, err := engine.Rollout(context.Background(),
		RolloutPlans(build("s0"), build("s1")).WithOptions(WithDeploymentID("rollout-0")),
		RolloutPlans(build("s2")).WithOptions(WithDeploymentID("rollout-1")),
	)
	if err != nil {
		t.Fatal(err)
	}
	assertClientDeployStateEvent(t, events, 2, DeploymentStateDeployed, "rollout-0", 2, 0)
	assertClientDeployStateEvent(t, events, 3, DeploymentStateDeployed, "rollout-1", 1, 1)
	engine.RemoveDeploymentStateListeners()
	for index := len(rollout.Deployments()) - 1; index >= 0; index-- {
		if err := rollout.Deployments()[index].Undeploy(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(events) != 4 {
		t.Fatalf("removed listener received undeploy events: %#v", events)
	}
}

func assertClientDeployRolloutError(t *testing.T, err error, itemIndex int, category error) {
	t.Helper()
	if err == nil || !errors.Is(err, category) {
		t.Fatalf("rollout error = %v, want category %v", err, category)
	}
	var rolloutErr *DeploymentRolloutError
	if !errors.As(err, &rolloutErr) || rolloutErr.RolloutItemIndex() != itemIndex {
		t.Fatalf("rollout error index = %#v, want %d", rolloutErr, itemIndex)
	}
	if !strings.Contains(err.Error(), "rollout item") {
		t.Fatalf("rollout error has no item context: %v", err)
	}
}

func assertClientDeployStateEvent(t *testing.T, events []DeploymentStateEvent, index int, state DeploymentState, deploymentID string, statements, rolloutItem int) {
	t.Helper()
	if len(events) <= index {
		t.Fatalf("deployment state events = %#v", events)
	}
	event := events[index]
	if event.State != state || event.DeploymentID != deploymentID || event.RuntimeURI != "default" ||
		len(event.Statements) != statements || event.RolloutItemIndex != rolloutItem {
		t.Fatalf("deployment state event %d = %#v", index, event)
	}
}
