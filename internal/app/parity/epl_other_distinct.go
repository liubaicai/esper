package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type eplOtherDistinctEvent struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

var eplOtherDistinctJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherDistinct.java",
}

var (
	eplOtherDistinctJavaRuntimeIDs = []string{
		"java-runtime-4b62c52a89cf86a3843a",
		"java-runtime-2e0f0a1356ea1b8e3299",
		"java-runtime-0318c1bce4d5e806aa11",
	}
	eplOtherDistinctJavaExecutions = []string{
		"EPLOtherOutputSimpleColumn",
		"EPLOtherOutputLimitEveryColumn",
		"EPLOtherBatchWindow",
	}
)

// runEplOtherDistinctScenario replays select-distinct view-flow scenarios over
// SupportBean, covering keep-all dedup, output-every batching with dedup, and
// length-batch flush dedup. Rows preserve first-seen insertion order.
func runEplOtherDistinctScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"distinct-simple-column", "distinct-output-every-column", "distinct-batch-window"}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("epl-other-distinct scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runEplOtherDistinctCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("epl-other-distinct case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runEplOtherDistinctCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[eplOtherDistinctEvent](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	str := esper.Field[eplOtherDistinctEvent, string]("theString")
	num := esper.Field[eplOtherDistinctEvent, int32]("intPrimitive")

	var query esper.Query
	switch caseName {
	case "distinct-simple-column":
		ws := esper.From[eplOtherDistinctEvent](env, "SupportBean").Window(esper.KeepAll())
		query = esper.Select(ws,
			esper.Alias("theString", str),
			esper.Alias("intPrimitive", num),
		).Query(
			esper.StatementName("s0"),
			esper.WithDistinct(),
		)
	case "distinct-output-every-column":
		ws := esper.From[eplOtherDistinctEvent](env, "SupportBean")
		query = esper.Select(ws,
			esper.Alias("theString", str),
			esper.Alias("intPrimitive", num),
		).Query(
			esper.StatementName("s0"),
			esper.WithDistinct(),
			esper.WithOutput(esper.OutputAllEveryEvents(3)),
		)
	case "distinct-batch-window":
		ws := esper.From[eplOtherDistinctEvent](env, "SupportBean").Window(esper.LengthBatch(3))
		query = esper.Select(ws,
			esper.Alias("theString", str),
			esper.Alias("intPrimitive", num),
		).Query(
			esper.StatementName("s0"),
			esper.WithDistinct(),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported epl-other-distinct case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeEplOtherDistinctPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown epl-other-distinct statement %q", name)
		}
		return statement, nil
	})
}

func decodeEplOtherDistinctPayload(step compat.Step) (any, error) {
	if step.EventType == "SupportBean" {
		var event eplOtherDistinctEvent
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("epl-other-distinct SupportBean: %w", err)
		}
		return event, nil
	}
	return nil, fmt.Errorf("epl-other-distinct: unsupported event type %q", step.EventType)
}
