package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const rollupOutputFirstJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var rollupOutputFirstJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowPerGroupRollup.java",
}

var (
	rollupOutputFirstJavaRuntimeIDs = []string{
		"java-runtime-d3ba0dd07bb4e54ef9af",
	}
	rollupOutputFirstJavaExecutions = []string{
		"ResultSetOutputFirst{join=false}",
	}
)

func runRollupOutputFirstScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "first"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("rollup-output-first scenario %q has no supported cases", scenario.ID)
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
	query := esper.From[rollupBean](env, "SupportBean").Window(esper.TimeWindow(3500*time.Millisecond)).
		GroupByRollup(theString, intPrimitive).
		Select(
			esper.Alias("c0", theString),
			esper.Alias("c1", intPrimitive),
			esper.Alias("c2", esper.Sum[int64](esper.Field[rollupBean, int64]("longBoxed"))),
		).Query(
		esper.StatementName("s0"),
		esper.WithOldStream(),
		esper.WithOutput(esper.OutputFirstEveryTime(time.Second)),
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
			return nil, fmt.Errorf("unknown rollup-output-first statement %q", name)
		}
		return statement, nil
	})
}
