package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type outputPolicyTrade struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
}

const (
	outputPolicyCaseAllEvery3   = "all-every-3"
	outputPolicyCaseFirstEvery3 = "first-every-3"
	outputPolicyCaseLastEvery3  = "last-every-3"
)

const outputPolicyJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var outputPolicyJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAggregateGrouped.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitSimple.java",
}

var (
	outputPolicyJavaRuntimeIDs = []string{
		"java-runtime-34a8b14ea96443475b9c",
		"java-runtime-132314df55366e67ec84",
		"java-runtime-decf2ff614722c1273d1",
	}
	outputPolicyJavaExecutions = []string{
		"ResultSetWildcardRowPerGroup",
		"ResultSetFirstSimpleHavingAndNoHaving",
		"ResultSetUnaggregatedOutputFirst",
	}
)

func runOutputPolicyScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		outputPolicyCaseAllEvery3,
		outputPolicyCaseFirstEvery3,
		outputPolicyCaseLastEvery3,
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runOutputPolicyCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("output policy case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("output policy scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runOutputPolicyCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[outputPolicyTrade](env, "Trade"); err != nil {
		return compat.Trace{}, err
	}
	symbol := esper.Field[outputPolicyTrade, string]("symbol")
	price := esper.Field[outputPolicyTrade, float64]("price")
	query := esper.From[outputPolicyTrade](env, "Trade").
		Window(esper.LengthWindow(2)).
		Filter(esper.Greater[float64](price, esper.Literal(10.0))).
		GroupBy(symbol).
		Select(
			esper.Alias("symbol", symbol),
			esper.Alias("total", esper.Sum[float64](price)),
		)
	var policy esper.OutputPolicy
	switch caseName {
	case outputPolicyCaseAllEvery3:
		policy = esper.OutputSnapshotEveryEvents(3)
	case outputPolicyCaseFirstEvery3:
		policy = esper.OutputFirstEveryEvents(3)
	case outputPolicyCaseLastEvery3:
		policy = esper.OutputLastEveryEvents(3)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported output policy case %q", caseName)
	}
	plan, err := env.Build(query.Query(esper.StatementName("s0"), esper.WithOutput(policy)))
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeOutputPolicyPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown output policy statement %q", name)
		}
		return statement, nil
	})
}

func decodeOutputPolicyPayload(step compat.Step) (any, error) {
	var value outputPolicyTrade
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode Trade: %w", err)
	}
	return value, nil
}
