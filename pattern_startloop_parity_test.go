package esper

import (
	"context"
	"fmt"
	"testing"
)

// TestPatternStartLoopMatchesEsper mirrors PatternStartLoop: a listener
// deploys and undeploys pattern statements reentrantly from inside the
// update callback (Java deploys ST1..ST10 this way). Java fires a bare
// not-root at deploy time (begin-state true) and the loop is driven by that
// deploy-time fire; the Go engine rejects a bare negative root at Build
// ("root negative pattern has no positive completion"), so the Go replay
// registers that difference explicitly and drives the same listener-callback
// deploy/undeploy cycles with a valid followed-by pattern. The smoke
// property is identical: callback reentrancy neither deadlocks nor corrupts
// the engine.
func TestPatternStartLoopMatchesEsper(t *testing.T) {
	env := newPoolParityEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	sbStream := From[patternOpBean](env, "SupportBean")

	// Java's statement, select * from pattern [not SupportBean], is a bare
	// negative root: it fires at deploy in Esper and is rejected at Build in
	// the Go port.
	if _, err := env.Build(PatternFrom(sbStream, "nb", Literal(true)).Not().
		Select(Alias("c", Literal(1))).Query(StatementName("s0"))); err == nil {
		t.Fatal("bare not-root pattern was accepted at build")
	}

	buildSeq := func(name string) Plan {
		t.Helper()
		plan, err := env.Build(PatternFrom(sbStream, "na", Literal(true)).
			Then(PatternFrom(sbStream, "nb", Literal(true))).
			Select(Alias("c", Literal(1))).Query(StatementName(name)))
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}

	cycles := 0
	driver := PatternFrom(sbStream, "drv", Literal(true)).Every().
		Select(Alias("c", Literal(1))).Query(StatementName("driver"))
	driverPlan, err := env.Build(driver)
	if err != nil {
		t.Fatal(err)
	}
	driverDeployment, err := engine.Deploy(context.Background(), driverPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driverDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for range batch.New {
			for i := 0; i < 10; i++ {
				name := fmt.Sprintf("ST%d", i+1)
				deployment, err := engine.Deploy(context.Background(), buildSeq(name))
				if err != nil {
					return err
				}
				if _, err := deployment.Statements()[0].Subscribe(func(context.Context, ResultBatch) error { return nil }); err != nil {
					return err
				}
				if err := deployment.Undeploy(context.Background()); err != nil {
					return err
				}
				deployment, err = engine.Deploy(context.Background(), buildSeq(name))
				if err != nil {
					return err
				}
				if _, err := deployment.Statements()[0].Subscribe(func(context.Context, ResultBatch) error { return nil }); err != nil {
					return err
				}
				cycles++
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), patternOpBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if cycles != 10 {
		t.Fatalf("listener-driven deploy/undeploy cycles = %d, want 10", cycles)
	}

	// Java closes with undeployAll + redeploy + undeployAll of the same
	// expression; the engine must stay healthy through the cycle.
	deployed, err := engine.Deploy(context.Background(), buildSeq("s0"))
	if err != nil {
		t.Fatal(err)
	}
	if err := deployed.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	redeployed, err := engine.Deploy(context.Background(), buildSeq("s0"))
	if err != nil {
		t.Fatal(err)
	}
	if err := redeployed.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := driverDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}
