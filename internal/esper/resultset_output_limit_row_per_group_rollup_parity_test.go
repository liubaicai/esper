package esper

import (
	"context"
	"testing"
	"time"
)

type rollupLimitBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

func newRollupLimitEnvironment(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[rollupLimitBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	return env, engine
}

func rollupLimitBuild(t *testing.T, env *Environment, policy OutputPolicy, orderBy bool) Plan {
	t.Helper()
	theString := Field[rollupLimitBean, string]("theString")
	intPrimitive := Field[rollupLimitBean, int]("intPrimitive")
	options := []QueryOption{StatementName("s0"), WithOldStream(), WithOutput(policy)}
	if orderBy {
		options = append(options, OrderBy(Ascending(ResultField[string]("c0")), Ascending(ResultField[int]("c1"))))
	}
	plan, err := env.Build(From[rollupLimitBean](env, "SupportBean").Window(TimeWindow(3500*time.Millisecond)).
		GroupByRollup(theString, intPrimitive).
		Select(
			Alias("c0", theString),
			Alias("c1", intPrimitive),
			Alias("c2", Sum[int64](Field[rollupLimitBean, int64]("longPrimitive"))),
		).Query(options...))
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func rollupLimitDeploy(t *testing.T, engine *Engine, plan Plan) (func() []ResultBatch, *Deployment) {
	t.Helper()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return func() []ResultBatch { return append([]ResultBatch(nil), batches...) }, deployment
}

func rollupLimitSend(t *testing.T, engine *Engine, theString string, intPrimitive int, longPrimitive int64) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), rollupLimitBean{TheString: theString, IntPrimitive: intPrimitive, LongPrimitive: longPrimitive}); err != nil {
		t.Fatal(err)
	}
}

func rollupLimitAdvance(t *testing.T, engine *Engine, ms int64) {
	t.Helper()
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(ms).UTC()); err != nil {
		t.Fatal(err)
	}
}

// TestResultSetOutputLimitRowPerGroupRollupParity covers ten executions from
// ResultSetOutputLimitRowPerGroupRollup using the shared 3.5s rollup scenario:
// ResultSet1NoOutputLimit, ResultSet2OutputLimitDefault,
// ResultSet4OutputLimitLast, ResultSet5OutputLimitFirst,
// ResultSet6OutputLimitSnapshot{join=false}, ResultSetOutputSnapshotOrderWLimit,
// ResultSetOutputDefault{join=false}, ResultSetOutputDefaultSorted{join=false},
// ResultSetOutputLast{join=false} and ResultSetOutputLastSorted{join=false}.
func TestResultSetOutputLimitRowPerGroupRollupParity(t *testing.T) {
	t.Run("no-output-limit", func(t *testing.T) {
		env, engine := newRollupLimitEnvironment(t)
		defer func() { _ = engine.Close(context.Background()) }()
		batches, deployment := rollupLimitDeploy(t, engine, rollupLimitBuild(t, env, OutputAll(), false))
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		rollupLimitSend(t, engine, "E1", 1, 10)
		if got := batches(); len(got) != 1 || len(got[0].New) != 3 {
			t.Fatalf("no-output-limit batches = %#v", got)
		}
	})

	t.Run("default-every-time", func(t *testing.T) {
		env, engine := newRollupLimitEnvironment(t)
		defer func() { _ = engine.Close(context.Background()) }()
		batches, deployment := rollupLimitDeploy(t, engine, rollupLimitBuild(t, env, OutputEveryTime(time.Second), false))
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		rollupLimitSend(t, engine, "E1", 1, 10)
		rollupLimitSend(t, engine, "E1", 2, 20)
		rollupLimitSend(t, engine, "E1", 1, 30)
		rollupLimitAdvance(t, engine, 1000)
		got := batches()
		if len(got) != 1 || len(got[0].New) != 9 || len(got[0].Old) != 9 {
			t.Fatalf("default-every-time batches = %#v", got)
		}
	})

	t.Run("last-every-time", func(t *testing.T) {
		env, engine := newRollupLimitEnvironment(t)
		defer func() { _ = engine.Close(context.Background()) }()
		batches, deployment := rollupLimitDeploy(t, engine, rollupLimitBuild(t, env, OutputLastEveryTime(time.Second), false))
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		rollupLimitSend(t, engine, "E1", 1, 10)
		rollupLimitSend(t, engine, "E1", 2, 20)
		rollupLimitSend(t, engine, "E1", 1, 30)
		rollupLimitAdvance(t, engine, 1000)
		got := batches()
		if len(got) != 1 || len(got[0].New) != 4 || len(got[0].Old) != 4 {
			t.Fatalf("last-every-time batches = %#v", got)
		}
	})

	t.Run("first-every-time", func(t *testing.T) {
		env, engine := newRollupLimitEnvironment(t)
		defer func() { _ = engine.Close(context.Background()) }()
		batches, deployment := rollupLimitDeploy(t, engine, rollupLimitBuild(t, env, OutputFirstEveryTime(time.Second), false))
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		rollupLimitSend(t, engine, "E1", 1, 10)
		if first := batches(); len(first) != 1 || len(first[0].New) != 3 {
			t.Fatalf("first-every-time initial batches = %#v", first)
		}
		rollupLimitSend(t, engine, "E1", 2, 20)
		rollupLimitSend(t, engine, "E1", 1, 30)
		rollupLimitAdvance(t, engine, 1000)
		if got := batches(); len(got) < 2 {
			t.Fatalf("first-every-time later batches = %#v", got)
		}
	})

	t.Run("snapshot-every-time", func(t *testing.T) {
		env, engine := newRollupLimitEnvironment(t)
		defer func() { _ = engine.Close(context.Background()) }()
		batches, deployment := rollupLimitDeploy(t, engine, rollupLimitBuild(t, env, OutputSnapshotEvery(time.Second), false))
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		rollupLimitSend(t, engine, "E1", 1, 10)
		rollupLimitSend(t, engine, "E1", 2, 20)
		rollupLimitSend(t, engine, "E1", 1, 30)
		rollupLimitAdvance(t, engine, 1000)
		got := batches()
		if len(got) != 1 || len(got[0].New) != 4 || len(got[0].Old) != 0 {
			t.Fatalf("snapshot-every-time batches = %#v", got)
		}
	})

	t.Run("snapshot-order-limit", func(t *testing.T) {
		env, engine := newRollupLimitEnvironment(t)
		defer func() { _ = engine.Close(context.Background()) }()
		plan, err := env.Build(From[rollupLimitBean](env, "SupportBean").Window(TimeWindow(10*time.Second)).
			GroupByRollup(Field[rollupLimitBean, string]("theString")).
			Select(
				Alias("c0", Field[rollupLimitBean, string]("theString")),
				Alias("c1", Sum[int](Field[rollupLimitBean, int]("intPrimitive"))),
			).Query(
			StatementName("s0"),
			WithOutput(OutputSnapshotEvery(time.Second)),
			OrderBy(Ascending(ResultField[int]("c1"))),
			Limit(3),
		))
		if err != nil {
			t.Fatal(err)
		}
		batches, deployment := rollupLimitDeploy(t, engine, plan)
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		rollupLimitSend(t, engine, "E1", 12, 0)
		rollupLimitSend(t, engine, "E2", 11, 0)
		rollupLimitSend(t, engine, "E3", 10, 0)
		rollupLimitSend(t, engine, "E4", 13, 0)
		rollupLimitSend(t, engine, "E2", 5, 0)
		rollupLimitAdvance(t, engine, 1000)
		got := batches()
		if len(got) != 1 || len(got[0].New) != 3 {
			t.Fatalf("snapshot-order-limit batches = %#v", got)
		}
		first := got[0].New[0]
		row, ok := first.Row()
		if !ok || row.Get("c1").Any() != 10 {
			t.Fatalf("snapshot-order-limit first row = %#v", first)
		}
	})

	for _, tc := range []struct {
		name    string
		policy  OutputPolicy
		orderBy bool
		wantNew int
		wantOld int
	}{
		{"default", OutputEveryTime(time.Second), false, 9, 9},
		{"default-sorted", OutputEveryTime(time.Second), true, 9, 9},
		{"last", OutputLastEveryTime(time.Second), false, 4, 4},
		{"last-sorted", OutputLastEveryTime(time.Second), true, 4, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, engine := newRollupLimitEnvironment(t)
			defer func() { _ = engine.Close(context.Background()) }()
			batches, deployment := rollupLimitDeploy(t, engine, rollupLimitBuild(t, env, tc.policy, tc.orderBy))
			defer func() { _ = deployment.Undeploy(context.Background()) }()
			rollupLimitSend(t, engine, "E1", 1, 10)
			rollupLimitSend(t, engine, "E1", 2, 20)
			rollupLimitSend(t, engine, "E1", 1, 30)
			rollupLimitAdvance(t, engine, 1000)
			got := batches()
			if len(got) != 1 || len(got[0].New) != tc.wantNew || len(got[0].Old) != tc.wantOld {
				t.Fatalf("%s batches = %#v", tc.name, got)
			}
		})
	}
}
