package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the third EPLDatabaseJoin slice — the
// EPLDatabaseRestartStatement execution (java-runtime-0d41625df368897ca97b).
// The statement `@name('s0') select mychar from SupportBean_S0 as s0,
// sql:MyDBWithRetain ['select mychar from mytesttable where ${id} =
// mytesttable.mybigint'] as s1` is deployed once and then cycled 100 times:
// undeployModuleContaining("s0"), send S0(id=1) with no listener attached
// (nothing delivers), deploy the same statement again with the listener
// re-added, send S0(id=1) for exactly one {mychar:"Z"} delivery. The loop is
// the Java regression's guard against "Too many connections" — all 100
// cycles must deliver. The Go engine has no connection pool to leak, which
// is the point of the differential: the redeploy path must never strand the
// historical join or the delivery.
//
// The Go side reuses the first slice's canonical mytesttable fixture
// verbatim through eplDatabaseJoinSeedRowMaps(), the full 9-column schema
// via eplDatabaseJoinFullRowSchema(), and the function-fed
// eplDatabaseJoinProvider keyed on the trigger id (no database driver); the
// Java oracle runs against the esper-mysql mysql:8.0 Docker fixture and
// records the identical canonical forms.
const eplDatabaseRestartJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplDatabaseRestartJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/database/EPLDatabaseJoin.java",
}

var eplDatabaseRestartJavaRuntimeIDs = []string{
	"java-runtime-0d41625df368897ca97b", // restart-statement
}

var eplDatabaseRestartJavaExecutions = []string{
	"EPLDatabaseRestartStatement",
}

var eplDatabaseRestartCases = []string{
	"restart-statement",
}

// eplDatabaseRestartS0 mirrors the SupportBean_S0 trigger event.
type eplDatabaseRestartS0 struct {
	Id int64 `esper:"id"`
}

// eplDatabaseRestartCycles mirrors the Java loop length: 100
// undeploy/redeploy cycles, each ending in exactly one delivery.
const eplDatabaseRestartCycles = 100

func runEplDatabaseRestartScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("epl-database-restart scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range eplDatabaseRestartCases {
		caseTrace, err := runEplDatabaseRestartCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("epl-database-restart case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runEplDatabaseRestartCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	now := time.Unix(0, 0).UTC()
	sequence := uint64(0)
	var records []compat.TraceRecord
	emitListener := func(batch esper.ResultBatch) {
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: "s0",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			New:       compat.NormalizeResults(batch.New),
		})
	}

	switch caseIndex {
	case 0: // restart-statement — undeploy/redeploy cycles over one join
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[eplDatabaseRestartS0](env, "SupportBean_S0"); err != nil {
			return nil, err
		}
		rowSchema, err := eplDatabaseJoinFullRowSchema()
		if err != nil {
			return nil, err
		}
		provider := &eplDatabaseJoinProvider{
			schema:   rowSchema,
			rows:     eplDatabaseJoinSeedRowMaps(),
			keyField: "mybigint",
			key: func(request esper.HistoricalRequest) (int64, bool) {
				id, ok := request.Trigger.Get("id").Any().(int64)
				return id, ok
			},
		}
		query := esper.JoinMany(
			esper.JoinSource(esper.From[eplDatabaseRestartS0](env, "SupportBean_S0").Window(esper.KeepAll())),
			esper.JoinSource(esper.FromHistoricalOn[map[string]any](env, "s1", "SupportBean_S0", rowSchema, provider)),
		).Select(
			esper.SelectFrom(1, "mychar", esper.Field[map[string]any, string]("mychar")),
		).Query(esper.StatementName("s0"))
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(eplDatabaseRestartJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		plan, err := env.Build(query)
		if err != nil {
			return nil, err
		}
		deployed, err := engine.Deploy(ctx, plan)
		if err != nil {
			return nil, err
		}
		// subscribe re-adds the listener on each freshly deployed statement;
		// deliveries counts every listener invocation across the cycles so
		// the per-phase guards can compare deltas.
		deliveries := 0
		subscribe := func(deployment *esper.Deployment) error {
			_, subErr := deployment.Statements()[0].Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				deliveries++
				emitListener(batch)
				return nil
			})
			return subErr
		}
		if err := subscribe(deployed); err != nil {
			return nil, err
		}
		for cycle := 1; cycle <= eplDatabaseRestartCycles; cycle++ {
			// undeployModuleContaining("s0"), then the silent send while no
			// statement exists: nothing may deliver.
			before := deliveries
			if err := engine.Undeploy(ctx, deployed.ID()); err != nil {
				return nil, fmt.Errorf("cycle %d: undeploy: %w", cycle, err)
			}
			if err := engine.SendEvent(ctx, eplDatabaseRestartS0{Id: 1}); err != nil {
				return nil, fmt.Errorf("cycle %d: undeployed send: %w", cycle, err)
			}
			if deliveries != before {
				return nil, fmt.Errorf("cycle %d: deliveries while undeployed = %d, want 0", cycle, deliveries-before)
			}
			// Deploy the same statement again and re-add the listener; the
			// Go engine has no connection pool to exhaust, so every cycle
			// must still deliver exactly one row.
			deployed, err = engine.Deploy(ctx, plan)
			if err != nil {
				return nil, fmt.Errorf("cycle %d: redeploy: %w", cycle, err)
			}
			if err := subscribe(deployed); err != nil {
				return nil, fmt.Errorf("cycle %d: subscribe: %w", cycle, err)
			}
			before = deliveries
			if err := engine.SendEvent(ctx, eplDatabaseRestartS0{Id: 1}); err != nil {
				return nil, fmt.Errorf("cycle %d: send: %w", cycle, err)
			}
			if deliveries != before+1 {
				return nil, fmt.Errorf("cycle %d: deliveries = %d, want 1", cycle, deliveries-before)
			}
		}
	default:
		return nil, fmt.Errorf("unsupported epl-database-restart case index %d", caseIndex)
	}
	return records, nil
}
