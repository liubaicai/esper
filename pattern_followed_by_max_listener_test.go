package esper

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type recordingPatternLimitListener struct {
	engine *Engine
	events []PatternSubexpressionLimitEvent
}

func (l *recordingPatternLimitListener) OnPatternSubexpressionLimit(event PatternSubexpressionLimitEvent) {
	l.events = append(l.events, event)
	// The callback contract promises execution outside Engine.mu. Re-entering
	// a lock-taking API here turns that promise into a deadlock regression test.
	_ = l.engine.PatternSubexpressionLimitListeners()
}

func TestPatternFollowedByMaxReportsRejectedSubexpressionMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	symbol := func(prefix string) Expression[bool] {
		return StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal(prefix))
	}
	pattern := PatternFrom(base, "a", symbol("A")).Every().FollowedByMax(2, "b", symbol("B"))
	plan, err := env.Build(pattern.Select(
		Alias("a", TagField[string]("a", "symbol")),
		Alias("b", TagField[string]("b", "symbol")),
	).Query(StatementName("pattern-followed-by-limit-diagnostic")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	listener := &recordingPatternLimitListener{engine: engine}
	if err := engine.AddPatternSubexpressionLimitListener(listener); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A1"}, {Symbol: "A2"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(listener.events) != 0 {
		t.Fatalf("accepted branches reported limits = %#v", listener.events)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A3"}); err != nil {
		t.Fatal(err)
	}
	if len(listener.events) != 1 {
		t.Fatalf("pattern limit events = %#v, want one", listener.events)
	}
	event := listener.events[0]
	if event.DeploymentID != deployment.ID() || event.StatementName != "pattern-followed-by-limit-diagnostic" {
		t.Fatalf("pattern limit owner = %#v", event)
	}
	if event.Maximum != 2 || event.Attempted != 3 || !strings.Contains(event.Edge, "-[2]>") {
		t.Fatalf("pattern limit detail = %#v", event)
	}

	// Completing both admitted branches releases capacity; a later third
	// start produces one new diagnostic, not a replay of the previous one.
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B1"}); err != nil {
		t.Fatal(err)
	}
	for _, next := range []runtimeTestTrade{{Symbol: "A4"}, {Symbol: "A5"}, {Symbol: "A6"}} {
		if err := engine.SendEvent(context.Background(), next); err != nil {
			t.Fatal(err)
		}
	}
	if len(listener.events) != 2 || listener.events[1].Maximum != 2 || listener.events[1].Attempted != 3 {
		t.Fatalf("reused pattern capacity diagnostics = %#v", listener.events)
	}
	engine.RemovePatternSubexpressionLimitListener(listener)
	if len(engine.PatternSubexpressionLimitListeners()) != 0 {
		t.Fatal("pattern limit listener was not removed")
	}
}

func TestPatternFollowedByMaxNestedEdgeDiagnosticIsCoalescedPerInput(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	symbol := func(prefix string) Expression[bool] {
		return StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal(prefix))
	}
	pattern := PatternFrom(base, "a", symbol("A")).Every().
		FollowedBy("b", symbol("B")).
		FollowedByMax(3, "c", symbol("C"))
	plan, err := env.Build(pattern.Select(
		Alias("a", TagField[string]("a", "symbol")),
		Alias("b", TagField[string]("b", "symbol")),
		Alias("c", TagField[string]("c", "symbol")),
	).Query(StatementName("pattern-followed-by-limit-nested")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	listener := &recordingPatternLimitListener{engine: engine}
	if err := engine.AddPatternSubexpressionLimitHandler(listener); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "A1"}, {Symbol: "A2"}, {Symbol: "B1"},
		{Symbol: "A3"}, {Symbol: "A4"}, {Symbol: "B2"},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(listener.events) != 1 {
		t.Fatalf("nested pattern limit events = %#v, want one coalesced event", listener.events)
	}
	event := listener.events[0]
	if event.Maximum != 3 || event.Attempted != 4 || !strings.Contains(event.Edge, "-[3]>") {
		t.Fatalf("nested pattern limit detail = %#v", event)
	}
	engine.RemovePatternSubexpressionLimitHandler(listener)
}

func TestPatternSubexpressionLimitListenerRejectsNil(t *testing.T) {
	_, engine := newRuntimeTest(t)
	if err := engine.AddPatternSubexpressionLimitListener(nil); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("nil pattern limit listener error = %v", err)
	}
	var typedNil *recordingPatternLimitListener
	if err := engine.AddPatternSubexpressionLimitListener(typedNil); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("typed-nil pattern limit listener error = %v", err)
	}
	var nilFunc PatternSubexpressionLimitListenerFunc
	if err := engine.AddPatternSubexpressionLimitListener(nilFunc); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("nil function pattern limit listener error = %v", err)
	}
}
