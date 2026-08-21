package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type orderBySimpleEvent struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
}

var orderBySimpleJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderBySimple.java",
}

var (
	orderBySimpleJavaRuntimeIDs = []string{
		"java-runtime-d530652f60616c332431",
		"java-runtime-9668909b2b2f00769dab",
		"java-runtime-92e69b0657b10a656fdc",
	}
	orderBySimpleJavaExecutions = []string{
		"ResultSetOrderBySimple",
		"ResultSetOrderByDescending",
		"ResultSetOrderByMultipleKeys",
	}
)

// runOrderBySimpleScenario replays view-flow order-by scenarios over
// SupportMarketDataBean with a length(5) or length(10) window and output
// every 6 events, covering asc/desc/multi-key ordering with stable ties.
func runOrderBySimpleScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"orderby-simple", "orderby-descending", "orderby-multiple-keys"}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("orderby-simple scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runOrderBySimpleCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("orderby-simple case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runOrderBySimpleCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[orderBySimpleEvent](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}

	symbol := esper.Field[orderBySimpleEvent, string]("symbol")
	price := esper.Field[orderBySimpleEvent, float64]("price")

	ws := esper.From[orderBySimpleEvent](env, "SupportMarketDataBean").Window(esper.LengthWindow(5))
	if caseName == "orderby-multiple-keys" {
	}

	var query esper.Query
	switch caseName {
	case "orderby-simple":
		query = esper.Select(ws,
			esper.Alias("symbol", symbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(price)),
		)
	case "orderby-descending":
		query = esper.Select(ws,
			esper.Alias("symbol", symbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Descending(price)),
		)
	case "orderby-multiple-keys":
		ws10 := esper.From[orderBySimpleEvent](env, "SupportMarketDataBean").Window(esper.LengthWindow(10))
		query = esper.Select(ws10,
			esper.Alias("symbol", symbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(symbol),
				esper.Ascending(price),
			),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported orderby-simple case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeOrderBySimplePayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown orderby-simple statement %q", name)
		}
		return statement, nil
	})
}

func decodeOrderBySimplePayload(step compat.Step) (any, error) {
	if step.EventType == "SupportMarketDataBean" {
		var event orderBySimpleEvent
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("orderby-simple SupportMarketDataBean: %w", err)
		}
		return event, nil
	}
	return nil, fmt.Errorf("orderby-simple: unsupported event type %q", step.EventType)
}
