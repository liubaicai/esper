package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// filterWindowAggregateTrade is the shared neutral Trade payload used by the
// filter + length-window + grouped-aggregate + output-policy scenario.
type filterWindowAggregateTrade struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
}

const (
	filterWindowAggregateCasePlain  = "plain"
	filterWindowAggregateCaseEvery3 = "every-3"
)

const filterWindowAggregateJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var filterWindowAggregateJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAggregateGrouped.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitSimple.java",
}

var (
	filterWindowAggregateJavaRuntimeIDs = []string{
		"java-runtime-dc2ac7e7293a204aa102",
		"java-runtime-d7e3f4e3457216b42c0b",
		"java-runtime-a02b2a7f648b50c0eeb3",
	}
	filterWindowAggregateJavaExecutions = []string{
		"ResultSetNoOutputClauseView",
		"ResultSetNoJoinAll",
		"ResultSetSimpleNoJoinAll",
	}
)

// runFilterWindowAggregateScenario replays each case in its own statement and
// concatenates the normalized traces, matching the Java oracle's per-case
// runtime isolation.
func runFilterWindowAggregateScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{filterWindowAggregateCasePlain, filterWindowAggregateCaseEvery3}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runFilterWindowAggregateCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("filter window aggregate case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("filter window aggregate scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runFilterWindowAggregateCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[filterWindowAggregateTrade](env, "Trade"); err != nil {
		return compat.Trace{}, err
	}
	symbol := esper.Field[filterWindowAggregateTrade, string]("symbol")
	price := esper.Field[filterWindowAggregateTrade, float64]("price")
	query := esper.From[filterWindowAggregateTrade](env, "Trade").
		Window(esper.LengthWindow(2)).
		Filter(esper.Greater[float64](price, esper.Literal(10.0))).
		GroupBy(symbol).
		Select(
			esper.Alias("symbol", symbol),
			esper.Alias("total", esper.Sum[float64](price)),
		)
	options := []esper.QueryOption{esper.StatementName("s0")}
	if caseName == filterWindowAggregateCaseEvery3 {
		options = append(options, esper.WithOutput(esper.OutputEvery(3)))
	}
	plan, err := env.Build(query.Query(options...))
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeFilterWindowAggregatePayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown filter window aggregate statement %q", name)
		}
		return statement, nil
	})
}

func decodeFilterWindowAggregatePayload(step compat.Step) (any, error) {
	var value filterWindowAggregateTrade
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode Trade: %w", err)
	}
	return value, nil
}
