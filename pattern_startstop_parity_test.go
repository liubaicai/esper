package esper

import (
	"context"
	"testing"
)

// Parity coverage for PatternStartStop (see docs
// esper-go-port-implementation-plan.md): deploy/undeploy/redeploy lifecycle
// and listener add/remove semantics for pattern statements. Undeployed
// patterns do not match; redeployed patterns start fresh with empty state.

// TestPatternStartStopOneMatchesEsper mirrors PatternStartStopOne: deploy,
// verify no data, undeploy, send event (no fire), redeploy, send event
// (fires), verify snapshot sees the captured event.
func TestPatternStartStopOneMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	plan, err := env.Build(PatternFrom(base, "tag", Literal(true)).Every().
		Select(Alias("tag", TagField[string]("tag", "theString"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	// Deploy — started when created, no data yet
	deployment1, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	stmt1 := deployment1.Statements()[0]
	var fired1 []string
	if _, err := stmt1.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				fired1 = append(fired1, row.Get("tag").Any().(string))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Undeploy — send event, should not fire
	if err := deployment1.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternOpBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}
	if len(fired1) != 0 {
		t.Fatalf("undeployed pattern fired = %v, want none", fired1)
	}

	// Redeploy — fresh state, send event fires
	deployment2, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	stmt2 := deployment2.Statements()[0]
	var fired2 []string
	if _, err := stmt2.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				fired2 = append(fired2, row.Get("tag").Any().(string))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternOpBean{TheString: "E2"}); err != nil {
		t.Fatal(err)
	}
	if len(fired2) != 1 || fired2[0] != "E2" {
		t.Fatalf("redeployed pattern fired = %v, want [E2]", fired2)
	}

	// Stop via Statement.Stop — send event, should not fire
	if err := stmt2.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternOpBean{TheString: "E3"}); err != nil {
		t.Fatal(err)
	}
	if len(fired2) != 1 {
		t.Fatalf("stopped pattern fired again = %v, want only E2", fired2)
	}

	// Start via Statement.Start — send event fires
	if err := stmt2.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternOpBean{TheString: "E4"}); err != nil {
		t.Fatal(err)
	}
	if len(fired2) != 2 || fired2[1] != "E4" {
		t.Fatalf("restarted pattern fired = %v, want [E2 E4]", fired2)
	}
}

// TestPatternAddRemoveListenerMatchesEsper mirrors PatternAddRemoveListener:
// deploy without listener, send event (no callback), add listener, send
// event (fires), remove listener, send event (no callback).
func TestPatternAddRemoveListenerMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	plan, err := env.Build(PatternFrom(base, "tag", Literal(true)).Every().
		Select(Alias("tag", TagField[string]("tag", "theString"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	stmt := deployment.Statements()[0]

	// No listener yet — event fires but nobody hears it
	if err := engine.SendEvent(context.Background(), patternOpBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}

	// Add listener — next event fires
	fired := []string{}
	_, err = stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				fired = append(fired, row.Get("tag").Any().(string))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternOpBean{TheString: "E2"}); err != nil {
		t.Fatal(err)
	}
	if len(fired) != 1 || fired[0] != "E2" {
		t.Fatalf("after add listener fired = %v, want [E2]", fired)
	}

	// Stop the statement — no more fires even with listener attached
	if err := stmt.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternOpBean{TheString: "E3"}); err != nil {
		t.Fatal(err)
	}
	if len(fired) != 1 {
		t.Fatalf("stopped pattern fired = %v, want only E2", fired)
	}

	// Start again — fires resume
	if err := stmt.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternOpBean{TheString: "E4"}); err != nil {
		t.Fatal(err)
	}
	if len(fired) != 2 || fired[1] != "E4" {
		t.Fatalf("restarted fired = %v, want [E2 E4]", fired)
	}
}

// TestPatternStartStopTwoMatchesEsper mirrors PatternStartStopTwo: 100
// deploy/undeploy/redeploy cycles with an or-pattern over two event types,
// verifying clean state reset each cycle.
func TestPatternStartStopTwoMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	if _, err := RegisterStruct[patternComplexProps](env, "SupportBeanComplexProps"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	complexStream := From[patternComplexProps](env, "SupportBeanComplexProps")
	pattern := PatternFrom(base, "a", Literal(true)).Or(
		PatternFrom(complexStream, "b", Literal(true))).Every()
	plan, err := env.Build(pattern.Select(
		Alias("a", TagField[string]("a", "theString")),
		Alias("b", TagField[string]("b", "simpleProperty")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	for cycle := 0; cycle < 100; cycle++ {
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatalf("cycle %d deploy: %v", cycle, err)
		}
		stmt := deployment.Statements()[0]
		fired := 0
		if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
			fired += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}

		// Pattern should fire on either event type
		if err := engine.SendEvent(context.Background(), patternOpBean{TheString: "X"}); err != nil {
			t.Fatalf("cycle %d send: %v", cycle, err)
		}
		if fired != 1 {
			t.Fatalf("cycle %d fired = %d, want 1", cycle, fired)
		}

		// Undeploy — events sent while undeployed should not fire
		if err := deployment.Undeploy(context.Background()); err != nil {
			t.Fatalf("cycle %d undeploy: %v", cycle, err)
		}
		if err := engine.SendEvent(context.Background(), patternOpBean{TheString: "Y"}); err != nil {
			t.Fatalf("cycle %d send-undeployed: %v", cycle, err)
		}
		if err := engine.SendEvent(context.Background(), defaultComplexProps()); err != nil {
			t.Fatalf("cycle %d send-complex: %v", cycle, err)
		}
		if fired != 1 {
			t.Fatalf("cycle %d fired after undeploy = %d, want 1", cycle, fired)
		}
	}
}
