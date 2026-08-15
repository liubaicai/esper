package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const rollupOutputSnapshotOrderLimitJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var rollupOutputSnapshotOrderLimitJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowPerGroupRollup.java",
}

var (
	rollupOutputSnapshotOrderLimitJavaRuntimeIDs = []string{
		"java-runtime-3a98c0c727c06ec21cfd",
	}
	rollupOutputSnapshotOrderLimitJavaExecutions = []string{
		"ResultSetOutputSnapshotOrderWLimit",
	}
)

func runRollupOutputSnapshotOrderLimitScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "snapshot-order-limit"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("rollup-output-snapshot-order-limit scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[rollupBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	theString := esper.Field[rollupBean, string]("theString")
	intPrimitive := esper.Field[rollupBean, int]("intPrimitive")
	query := esper.From[rollupBean](env, "SupportBean").
		GroupByRollup(theString).
		Select(
			esper.Alias("c0", theString),
			esper.Alias("c1", esper.Sum[int](intPrimitive)),
		).Query(
		esper.StatementName("s0"),
		esper.WithOutput(esper.OutputSnapshotEvery(time.Second)),
		esper.OrderBy(esper.Ascending(esper.ResultField[int]("c1"))),
		esper.Limit(3),
	)
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeRollupPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown rollup-output-snapshot-order-limit statement %q", name)
		}
		return statement, nil
	})
}
