package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type exprCoreCurrentTimestampBean struct {
	TheString string `esper:"theString"`
}

const exprCoreCurrentTimestampJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprCoreCurrentTimestampJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreCurrentTimestamp.java",
}

var (
	exprCoreCurrentTimestampJavaRuntimeIDs = []string{
		"java-runtime-c1c1fd3dc31af4864a50",
		"java-runtime-96c8b8cb4cf36a523669",
		"java-runtime-5b126fe7fb865be8b293",
	}
	exprCoreCurrentTimestampJavaExecutions = []string{
		"ExprCoreCurrentTimestampGet",
		"ExprCoreCurrentTimestampOM",
		"ExprCoreCurrentTimestampCompile",
	}
)

func runExprCoreCurrentTimestampScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"current-timestamp-get", "current-timestamp-om", "current-timestamp-compile"}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runExprCoreCurrentTimestampCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("expr-core-current-timestamp case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("expr-core-current-timestamp scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runExprCoreCurrentTimestampCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[exprCoreCurrentTimestampBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	input := esper.From[exprCoreCurrentTimestampBean](env, "SupportBean")
	current := esper.CurrentTimestamp()
	var query esper.Query
	switch caseName {
	case "current-timestamp-get":
		query = esper.Select(input,
			esper.Alias("current_timestamp()", current),
			esper.Alias("t0", current),
			esper.Alias("t1", esper.CurrentTimestamp()),
			esper.Alias("t2", esper.Add[int64](current, esper.Literal[int64](1))),
		).Query(esper.StatementName("s0"))
	case "current-timestamp-om", "current-timestamp-compile":
		query = esper.Select(input, esper.Alias("t0", current)).Query(esper.StatementName("s0"))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported expr-core-current-timestamp case %q", caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		_ = engine.Close(context.Background())
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		_ = engine.Close(context.Background())
		return compat.Trace{}, fmt.Errorf("expected one current-timestamp parity statement, got %d", len(statements))
	}
	statement := statements[0]
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeExprCoreCurrentTimestampPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown expr-core-current-timestamp statement %q", name)
		}
		return statement, nil
	})
}

func decodeExprCoreCurrentTimestampPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("expr-core-current-timestamp: unsupported event type %q", step.EventType)
	}
	var value exprCoreCurrentTimestampBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
