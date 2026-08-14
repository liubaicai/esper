package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetGroupedTimeWindowMarket struct {
	Symbol string  `esper:"symbol"`
	Volume int64   `esper:"volume"`
	Price  float64 `esper:"price"`
}

const (
	resultsetGroupedTimeWindowCaseGrouped = "grouped"
	resultsetGroupedTimeWindowCaseHaving  = "having"
)

const resultsetGroupedTimeWindowJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetGroupedTimeWindowJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAggregateGrouped.java",
}

var (
	resultsetGroupedTimeWindowJavaRuntimeIDs = []string{
		"java-runtime-9061246c4c67a2c25364",
		"java-runtime-11c5505c0b19dd86671c",
	}
	resultsetGroupedTimeWindowJavaExecutions = []string{
		"ResultSet1NoneNoHavingNoJoin",
		"ResultSet3NoneHavingNoJoin",
	}
)

// runResultSetGroupedTimeWindowScenario replays a grouped time-window sum
// with no output policy, including window-expiry old rows.
func runResultSetGroupedTimeWindowScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if !scenarioHasCase(scenario, resultsetGroupedTimeWindowCaseGrouped) {
		return compat.Trace{}, fmt.Errorf("resultset grouped time window scenario %q has no supported cases", scenario.ID)
	}
	caseOrder := []string{resultsetGroupedTimeWindowCaseGrouped, resultsetGroupedTimeWindowCaseHaving}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runResultSetGroupedTimeWindowCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset grouped time window case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetGroupedTimeWindowCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, resultsetGroupedTimeWindowCaseGrouped)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	symbol := esper.Field[resultsetGroupedTimeWindowMarket, string]("symbol")
	volume := esper.Field[resultsetGroupedTimeWindowMarket, int64]("volume")
	price := esper.Field[resultsetGroupedTimeWindowMarket, float64]("price")
	sum := esper.Sum[float64](price)
	grouped := esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
		Window(esper.TimeWindow(5500 * time.Millisecond)).
		GroupBy(symbol)
	var query esper.Query
	if caseName == resultsetGroupedTimeWindowCaseHaving {
		query = grouped.Having(esper.Greater[float64](sum, esper.Literal(50.0))).Select(
			esper.Alias("symbol", symbol),
			esper.Alias("volume", volume),
			esper.Alias("sum(price)", sum),
		).Query(esper.StatementName("s0"))
	} else if caseName == resultsetGroupedTimeWindowCaseGrouped {
		query = grouped.Select(
			esper.Alias("symbol", symbol),
			esper.Alias("volume", volume),
			esper.Alias("sum(price)", sum),
		).Query(esper.StatementName("s0"))
	} else {
		return compat.Trace{}, fmt.Errorf("unsupported resultset grouped time window case %q", caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	trace, err := compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetGroupedTimeWindowPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset grouped time window statement %q", name)
		}
		return statement, nil
	})
	if err != nil {
		return compat.Trace{}, err
	}
	if caseName == resultsetGroupedTimeWindowCaseHaving {
		for index := range trace.Records {
			trace.Records[index].Case = resultsetGroupedTimeWindowCaseHaving
		}
	}
	return trace, nil
}

func decodeResultSetGroupedTimeWindowPayload(step compat.Step) (any, error) {
	var value resultsetGroupedTimeWindowMarket
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
	}
	return value, nil
}
