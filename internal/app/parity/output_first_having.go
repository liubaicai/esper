package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type outputFirstHavingBean struct {
	DoublePrimitive float64 `esper:"doublePrimitive"`
}

const (
	outputFirstHavingCaseEvents = "events"
	outputFirstHavingCaseTime   = "time"
)

const outputFirstHavingJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var outputFirstHavingJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitFirstHaving.java",
}

var (
	outputFirstHavingJavaRuntimeIDs = []string{
		"java-runtime-fe5c5a8ab0d234ff63c7",
		"java-runtime-52135d56e04dfb024bfe",
	}
	outputFirstHavingJavaExecutions = []string{
		"ResultSetHavingNoAvgOutputFirstEvents",
		"ResultSetHavingNoAvgOutputFirstMinutes",
	}
)

// runOutputFirstHavingScenario replays both ungrouped-HAVING output-first
// policies in their own runtime and concatenates the normalized traces,
// matching the Java oracle's per-case runtime isolation.
func runOutputFirstHavingScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{outputFirstHavingCaseEvents, outputFirstHavingCaseTime}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runOutputFirstHavingCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("output first having case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("output first having scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runOutputFirstHavingCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[outputFirstHavingBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	value := esper.Field[outputFirstHavingBean, float64]("doublePrimitive")
	var query esper.Query
	switch caseName {
	case outputFirstHavingCaseEvents:
		query = esper.From[outputFirstHavingBean](env, "SupportBean").Aggregate(
			esper.Alias("doublePrimitive", value),
		).Having(esper.Greater[float64](value, esper.Literal(1.0))).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputFirstEveryEvents(2)),
		)
	case outputFirstHavingCaseTime:
		sum := esper.Sum[float64](value)
		query = esper.From[outputFirstHavingBean](env, "SupportBean").Window(esper.LengthWindow(5)).Aggregate(
			esper.Alias("val0", sum),
		).Having(esper.Greater[float64](sum, esper.Literal(100.0))).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputFirstEveryTime(2*time.Second)),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported output first having case %q", caseName)
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
		return compat.Trace{}, fmt.Errorf("expected one output first having statement, got %d", len(statements))
	}
	statement := statements[0]
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeOutputFirstHavingPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown output first having statement %q", name)
		}
		return statement, nil
	})
}

func decodeOutputFirstHavingPayload(step compat.Step) (any, error) {
	var value outputFirstHavingBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
