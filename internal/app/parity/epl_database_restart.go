package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Output-parity coverage for the third EPLDatabaseJoin slice — the
// EPLDatabaseRestartStatement execution (java-runtime-0d41625df368897ca97b).
// The fixed lifecycle contract is undeploy, send while undeployed, redeploy,
// send with a fresh listener, repeated 100 times. The scenario pins that
// contract and the runner validates and consumes its cycle count.
//
// This Go replay uses a function-fed HistoricalProvider over the canonical
// fixture rows. It does not exercise database/sql, MySQL/JDBC connection
// acquisition or release, or connection-pool limits. The checked-in Java/Go
// evidence therefore proves only positive listener-output equivalence; the
// real MySQL/JDBC lifecycle accounting remains open.
//
// The runner also checks that the undeployed send produces no in-process
// delivery, but that negative guard is not represented as a trace record.
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

// eplDatabaseRestartCycles is the fixed Java lifecycle count pinned by the
// scenario contract.
const eplDatabaseRestartCycles = 100

const eplDatabaseRestartLifecycle = "undeploy-send-redeploy-send"

func validateEplDatabaseRestartScenario(scenario compat.Scenario) (int, error) {
	if err := scenario.Validate(); err != nil {
		return 0, err
	}
	if len(scenario.Steps) != 2 {
		return 0, fmt.Errorf("epl-database-restart scenario requires exactly 2 steps")
	}
	caseStep, advanceStep := scenario.Steps[0], scenario.Steps[1]
	if caseStep.Op != "case" || caseStep.Case != "restart-statement" ||
		caseStep.Label != eplDatabaseRestartLifecycle || caseStep.Count == nil ||
		*caseStep.Count != eplDatabaseRestartCycles {
		return 0, fmt.Errorf("epl-database-restart lifecycle contract is not pinned to %d %s cycles", eplDatabaseRestartCycles, eplDatabaseRestartLifecycle)
	}
	if advanceStep.Op != "advance-time" || advanceStep.At != "1970-01-01T00:00:00Z" {
		return 0, fmt.Errorf("epl-database-restart scenario must pin the epoch advance-time step")
	}
	return int(*caseStep.Count), nil
}

func runEplDatabaseRestartScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	cycles, err := validateEplDatabaseRestartScenario(scenario)
	if err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range eplDatabaseRestartCases {
		caseTrace, err := runEplDatabaseRestartCase(ctx, caseName, caseIndex, cycles)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("epl-database-restart case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runEplDatabaseRestartCase(ctx context.Context, caseName string, caseIndex, cycles int) ([]compat.TraceRecord, error) {
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
		// The initial deployment is listener-less, matching the Java setup.
		// Subscribe only after each fresh deployment; deliveries counts every
		// listener invocation across cycles for the per-phase guards.
		deliveries := 0
		subscribe := func(deployment *esper.Deployment) error {
			_, subErr := deployment.Statements()[0].Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				deliveries++
				emitListener(batch)
				return nil
			})
			return subErr
		}
		for cycle := 1; cycle <= cycles; cycle++ {
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
