package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetGroupOutputBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongBoxed     int64  `esper:"longBoxed"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

const resultsetAggregateGroupOutputJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateGroupOutputJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAggregateGrouped.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitSimple.java",
}

var (
	resultsetAggregateGroupOutputJavaRuntimeIDs = []string{
		"java-runtime-fbb40900955b851741c6",
		"java-runtime-34a8b14ea96443475b9c",
		"java-runtime-decf2ff614722c1273d1",
		"java-runtime-132314df55366e67ec84",
	}
	resultsetAggregateGroupOutputJavaExecutions = []string{
		"ResultSetLastNoDataWindow",
		"ResultSetWildcardRowPerGroup",
		"ResultSetUnaggregatedOutputFirst",
		"ResultSetFirstSimpleHavingAndNoHaving",
	}
)

// runResultSetAggregateGroupOutputScenario replays the unaggregated grouped
// output policies (last every time, last/all every events, first every time,
// first every events with and without a non-aggregated having), matching
// ResultSetLastNoDataWindow, ResultSetWildcardRowPerGroup,
// ResultSetUnaggregatedOutputFirst and ResultSetFirstSimpleHavingAndNoHaving.
func runResultSetAggregateGroupOutputScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		"last-no-data-window",
		"wildcard-last",
		"wildcard-all",
		"unaggregated-first",
		"first-simple",
		"first-simple-having",
	}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-group-output scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runResultSetAggregateGroupOutputCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-group-output case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateGroupOutputCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetGroupOutputBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	theString := esper.Field[resultsetGroupOutputBean, string]("theString")
	intPrimitive := esper.Field[resultsetGroupOutputBean, int]("intPrimitive")
	longBoxed := esper.Field[resultsetGroupOutputBean, int64]("longBoxed")
	longPrimitive := esper.Field[resultsetGroupOutputBean, int64]("longPrimitive")
	wildcard := func() esper.AggregateStream {
		return esper.From[resultsetGroupOutputBean](env, "SupportBean").
			GroupBy(theString).
			Select(
				esper.Alias("theString", theString),
				esper.Alias("intPrimitive", intPrimitive),
				esper.Alias("longBoxed", longBoxed),
				esper.Alias("longPrimitive", longPrimitive),
			)
	}
	var query esper.Query
	switch caseName {
	case "last-no-data-window":
		query = esper.From[resultsetGroupOutputBean](env, "SupportBean").
			GroupBy(theString).
			Select(
				esper.Alias("theString", theString),
				esper.Alias("intp", intPrimitive),
			).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputLastEveryTime(time.Second)),
			esper.OrderBy(esper.Ascending(esper.ResultField[string]("theString"))),
		)
	case "wildcard-last":
		query = wildcard().Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputLastEveryEvents(3)),
			esper.OrderBy(esper.Ascending(esper.ResultField[string]("theString"))),
		)
	case "wildcard-all":
		query = wildcard().Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(3)),
		)
	case "unaggregated-first":
		query = wildcard().Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputFirstEveryTime(10*time.Second)),
		)
	case "first-simple":
		query = esper.Select[resultsetGroupOutputBean](
			esper.From[resultsetGroupOutputBean](env, "SupportBean"),
			esper.Alias("theString", theString),
		).
			Query(
				esper.StatementName("s0"),
				esper.WithOutput(esper.OutputFirstEveryEvents(3)),
			)
	case "first-simple-having":
		query = esper.Select[resultsetGroupOutputBean](
			esper.From[resultsetGroupOutputBean](env, "SupportBean"),
			esper.Alias("theString", theString),
		).
			Having(esper.NotEqual[int](intPrimitive, esper.Literal(0))).
			Query(
				esper.StatementName("s0"),
				esper.WithOutput(esper.OutputFirstEveryEvents(3)),
			)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset-aggregate-group-output case %q", caseName)
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

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateGroupOutputPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-group-output statement %q", name)
		}
		return statement, nil
	})
}

func decodeResultSetAggregateGroupOutputPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value resultsetGroupOutputBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported resultset-aggregate-group-output event type %q", step.EventType)
	}
}
