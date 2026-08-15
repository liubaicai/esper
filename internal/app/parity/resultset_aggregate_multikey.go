package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetMultikeySupportBean struct {
	TheString     string `esper:"theString"`
	LongPrimitive int64  `esper:"longPrimitive"`
	IntPrimitive  int    `esper:"intPrimitive"`
}

const resultsetAggregateMultikeyJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateMultikeyJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAggregateGrouped.java",
}

var (
	resultsetAggregateMultikeyJavaRuntimeIDs = []string{
		"java-runtime-418605c41b57b8a28c09",
		"java-runtime-080737b1c181da82710e",
	}
	resultsetAggregateMultikeyJavaExecutions = []string{
		"ResultSetOutputAllMultikeyWArray",
		"ResultSetOutputLastMultikeyWArray",
	}
)

// runResultSetAggregateMultikeyScenario replays the SupportBean keepall
// grouped-by (theString, longPrimitive) sum with output last/all every 1
// seconds, matching ResultSetOutputLastMultikeyWArray and
// ResultSetOutputAllMultikeyWArray.
func runResultSetAggregateMultikeyScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"multikey-last", "multikey-all"}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-multikey scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runResultSetAggregateMultikeyCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-multikey case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateMultikeyCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetMultikeySupportBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	theString := esper.Field[resultsetMultikeySupportBean, string]("theString")
	longPrimitive := esper.Field[resultsetMultikeySupportBean, int64]("longPrimitive")
	intPrimitive := esper.Field[resultsetMultikeySupportBean, int]("intPrimitive")
	query := esper.From[resultsetMultikeySupportBean](env, "SupportBean").
		Window(esper.KeepAll()).
		GroupBy(theString, longPrimitive).
		Select(
			esper.Alias("theString", theString),
			esper.Alias("longPrimitive", longPrimitive),
			esper.Alias("intPrimitive", intPrimitive),
			esper.Alias("thesum", esper.Sum[int](intPrimitive)),
		).Query(
		esper.StatementName("s0"),
		esper.WithOutput(esper.OutputLastEveryTime(time.Second)),
	)
	if caseName == "multikey-all" {
		query = esper.From[resultsetMultikeySupportBean](env, "SupportBean").
			Window(esper.KeepAll()).
			GroupBy(theString, longPrimitive).
			Select(
				esper.Alias("theString", theString),
				esper.Alias("longPrimitive", longPrimitive),
				esper.Alias("intPrimitive", intPrimitive),
				esper.Alias("thesum", esper.Sum[int](intPrimitive)),
			).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryTime(time.Second)),
		)
	} else if caseName != "multikey-last" {
		return compat.Trace{}, fmt.Errorf("unsupported resultset-aggregate-multikey case %q", caseName)
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

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateMultikeyPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-multikey statement %q", name)
		}
		return statement, nil
	})
}

func decodeResultSetAggregateMultikeyPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value resultsetMultikeySupportBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported resultset-aggregate-multikey event type %q", step.EventType)
	}
}
