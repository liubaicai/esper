package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetOutputLimitRowPerGroupNoneDefaultJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetOutputLimitRowPerGroupNoneDefaultJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowPerGroup.java",
}

var (
	// Inventory order is authoritative: ResultSet1NoneNoHavingNoJoin is ordinal 1
	// through ResultSet8DefaultHavingJoin ordinal 8 in
	// ResultSetOutputLimitRowPerGroup.executions().
	resultsetOutputLimitRowPerGroupNoneJavaRuntimeIDs = []string{
		"java-runtime-047b01d4e6e6e73101c3", // ResultSet1NoneNoHavingNoJoin
		"java-runtime-f6dc738e9e1219068421", // ResultSet2NoneNoHavingJoin
		"java-runtime-815886be170eed7aee20", // ResultSet3NoneHavingNoJoin
		"java-runtime-7c4e54256aa53004caa5", // ResultSet4NoneHavingJoin
	}
	resultsetOutputLimitRowPerGroupDefaultJavaRuntimeIDs = []string{
		"java-runtime-b6a1986826894ec12fd1", // ResultSet5DefaultNoHavingNoJoin
		"java-runtime-66eefdbac64b7ab79a9b", // ResultSet6DefaultNoHavingJoin
		"java-runtime-3df0124bc42a5e107a8c", // ResultSet7DefaultHavingNoJoin
		"java-runtime-56e8b086b5eaf29a9494", // ResultSet8DefaultHavingJoin
	}
	resultsetOutputLimitRowPerGroupNoneJavaExecutions = []string{
		"ResultSet1NoneNoHavingNoJoin",
		"ResultSet2NoneNoHavingJoin",
		"ResultSet3NoneHavingNoJoin",
		"ResultSet4NoneHavingJoin",
	}
	resultsetOutputLimitRowPerGroupDefaultJavaExecutions = []string{
		"ResultSet5DefaultNoHavingNoJoin",
		"ResultSet6DefaultNoHavingJoin",
		"ResultSet7DefaultHavingNoJoin",
		"ResultSet8DefaultHavingJoin",
	}
)

const (
	resultsetOutputLimitRowPerGroupNoneID    = "resultset-output-limit-row-per-group-none"
	resultsetOutputLimitRowPerGroupDefaultID = "resultset-output-limit-row-per-group-default"

	resultsetOutputLimitRowPerGroupNoneNoHavingNoJoinCase = "none-no-having-no-join"
	resultsetOutputLimitRowPerGroupNoneNoHavingJoinCase   = "none-no-having-join"
	resultsetOutputLimitRowPerGroupNoneHavingNoJoinCase   = "none-having-no-join"
	resultsetOutputLimitRowPerGroupNoneHavingJoinCase     = "none-having-join"

	resultsetOutputLimitRowPerGroupDefaultNoHavingNoJoinCase = "default-no-having-no-join"
	resultsetOutputLimitRowPerGroupDefaultNoHavingJoinCase   = "default-no-having-join"
	resultsetOutputLimitRowPerGroupDefaultHavingNoJoinCase   = "default-having-no-join"
	resultsetOutputLimitRowPerGroupDefaultHavingJoinCase     = "default-having-join"
)

var resultsetOutputLimitRowPerGroupNoneCaseOrder = []string{
	resultsetOutputLimitRowPerGroupNoneNoHavingNoJoinCase,
	resultsetOutputLimitRowPerGroupNoneNoHavingJoinCase,
	resultsetOutputLimitRowPerGroupNoneHavingNoJoinCase,
	resultsetOutputLimitRowPerGroupNoneHavingJoinCase,
}

var resultsetOutputLimitRowPerGroupDefaultCaseOrder = []string{
	resultsetOutputLimitRowPerGroupDefaultNoHavingNoJoinCase,
	resultsetOutputLimitRowPerGroupDefaultNoHavingJoinCase,
	resultsetOutputLimitRowPerGroupDefaultHavingNoJoinCase,
	resultsetOutputLimitRowPerGroupDefaultHavingJoinCase,
}

// runResultSetOutputLimitRowPerGroupNoneScenario replays the four
// no-output-clause (per-event immediate output) executions from
// ResultSetOutputLimitRowPerGroup ordinals 1-4: grouped sum(price) over
// SupportMarketDataBean#time(5.5 sec).  The no-having twins add order by
// symbol; the having twins add having sum(price)>50.  Mirroring
// ResultAssertExecution, each case runs the EPL twice — the plain istream
// select then the select irstream twin — over the shared ResultAssertInput
// schedule, bracketed by deploy/undeploy-all steps.
func runResultSetOutputLimitRowPerGroupNoneScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(resultsetOutputLimitRowPerGroupNoneCaseOrder))
	for _, caseName := range resultsetOutputLimitRowPerGroupNoneCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetOutputLimitRowPerGroupNoneDefaultCase(ctx, scenario, caseName, resultsetOutputLimitRowPerGroupNoneID)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOutputLimitRowPerGroupNoneID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOutputLimitRowPerGroupNoneID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

// runResultSetOutputLimitRowPerGroupDefaultScenario replays the four
// output-every-1-seconds executions from ResultSetOutputLimitRowPerGroup
// ordinals 5-8: grouped sum(price) over SupportMarketDataBean#time(5.5 sec)
// under output every 1 seconds.  The no-having twins add order by symbol; the
// having twins add having sum(price)>50 (no order-by).  Mirroring
// ResultAssertExecution, each case runs the EPL twice — the plain istream
// select then the select irstream twin — over the shared ResultAssertInput
// schedule, bracketed by deploy/undeploy-all steps.
func runResultSetOutputLimitRowPerGroupDefaultScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(resultsetOutputLimitRowPerGroupDefaultCaseOrder))
	for _, caseName := range resultsetOutputLimitRowPerGroupDefaultCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetOutputLimitRowPerGroupNoneDefaultCase(ctx, scenario, caseName, resultsetOutputLimitRowPerGroupDefaultID)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOutputLimitRowPerGroupDefaultID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOutputLimitRowPerGroupDefaultID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetOutputLimitRowPerGroupNoneDefaultCase(ctx context.Context, scenario compat.Scenario, caseName, scenarioID string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[unidirectionalSupportBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetOutputLimitRowPerGroupNoneDefaultRuntimeURI(scenarioID, caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenarioID}
	var sequence uint64
	record := func(batch esper.ResultBatch) {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			// Force-dispatched empty pairs are not recorded (Java oracle
			// convention: payload-carrying callbacks only).
			return
		}
		sequence++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: "s0",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
	}

	var deployment *esper.Deployment
	deployIndex := 0
	deployVariant := func() error {
		query, err := resultsetOutputLimitRowPerGroupNoneDefaultQuery(env, caseName, deployIndex)
		if err != nil {
			return err
		}
		deployIndex++
		plan, err := env.Build(query)
		if err != nil {
			return fmt.Errorf("build %q variant %d: %w", caseName, deployIndex-1, err)
		}
		deployment, err = engine.Deploy(ctx, plan)
		if err != nil {
			return fmt.Errorf("deploy %q variant %d: %w", caseName, deployIndex-1, err)
		}
		for _, statement := range deployment.Statements() {
			if statement.Name() == "s0" {
				if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					record(batch)
					return nil
				}); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
		case "deploy":
			if err := deployVariant(); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
			if deployment != nil {
				if err := deployment.Undeploy(ctx); err != nil {
					return compat.Trace{}, fmt.Errorf("undeploy %q: %w", caseName, err)
				}
				deployment = nil
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("%s advance-time: %w", scenarioID, err)
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return compat.Trace{}, err
			}
		case "send":
			payload, err := decodeResultSetOutputLimitRowPerGroupNoneDefaultPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", scenarioID, step.Op)
		}
	}
	return trace, nil
}

// resultsetOutputLimitRowPerGroupNoneDefaultQuery builds the deploy variant for
// a case.  deployIndex selects the istream twin: variant 0 is the plain istream
// select (no remove stream), variant 1 the select irstream twin (WithOldStream).
// The none cases carry no output clause (per-event immediate output); the
// default cases add output every 1 seconds.  The no-having twins append
// order-by-symbol; the having twins add having sum(price)>50 (no order-by).
func resultsetOutputLimitRowPerGroupNoneDefaultQuery(env *esper.Environment, caseName string, deployIndex int) (esper.Query, error) {
	irstream := deployIndex == 1

	isJoin := caseName == resultsetOutputLimitRowPerGroupNoneNoHavingJoinCase ||
		caseName == resultsetOutputLimitRowPerGroupNoneHavingJoinCase ||
		caseName == resultsetOutputLimitRowPerGroupDefaultNoHavingJoinCase ||
		caseName == resultsetOutputLimitRowPerGroupDefaultHavingJoinCase
	isHaving := caseName == resultsetOutputLimitRowPerGroupNoneHavingNoJoinCase ||
		caseName == resultsetOutputLimitRowPerGroupNoneHavingJoinCase ||
		caseName == resultsetOutputLimitRowPerGroupDefaultHavingNoJoinCase ||
		caseName == resultsetOutputLimitRowPerGroupDefaultHavingJoinCase
	isDefault := caseName == resultsetOutputLimitRowPerGroupDefaultNoHavingNoJoinCase ||
		caseName == resultsetOutputLimitRowPerGroupDefaultNoHavingJoinCase ||
		caseName == resultsetOutputLimitRowPerGroupDefaultHavingNoJoinCase ||
		caseName == resultsetOutputLimitRowPerGroupDefaultHavingJoinCase
	hasOrderBy := !isHaving

	switch caseName {
	case resultsetOutputLimitRowPerGroupNoneNoHavingNoJoinCase,
		resultsetOutputLimitRowPerGroupNoneNoHavingJoinCase,
		resultsetOutputLimitRowPerGroupNoneHavingNoJoinCase,
		resultsetOutputLimitRowPerGroupNoneHavingJoinCase,
		resultsetOutputLimitRowPerGroupDefaultNoHavingNoJoinCase,
		resultsetOutputLimitRowPerGroupDefaultNoHavingJoinCase,
		resultsetOutputLimitRowPerGroupDefaultHavingNoJoinCase,
		resultsetOutputLimitRowPerGroupDefaultHavingJoinCase:
	default:
		return esper.Query{}, fmt.Errorf("%s has no case %q", resultsetOutputLimitRowPerGroupNoneID+"/"+resultsetOutputLimitRowPerGroupDefaultID, caseName)
	}

	options := []esper.QueryOption{esper.StatementName("s0")}
	if irstream {
		options = append(options, esper.WithOldStream())
	}
	if isDefault {
		options = append(options, esper.WithOutput(esper.OutputEveryTime(time.Second)))
	}

	symbol := esper.Field[resultsetGroupedTimeWindowMarket, string]("symbol")
	price := esper.Field[resultsetGroupedTimeWindowMarket, float64]("price")

	if isJoin {
		theString := esper.Field[unidirectionalSupportBean, string]("theString")
		joinSymbol := esper.JoinField[string](0, "symbol")
		joinPrice := esper.JoinField[float64](0, "price")
		grouped := esper.Join(
			esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
				Window(esper.TimeWindow(5500*time.Millisecond)),
			esper.From[unidirectionalSupportBean](env, "SupportBean").
				Window(esper.KeepAll()),
			esper.OnEqual(symbol, theString),
		).GroupBy(joinSymbol).Select(
			esper.Alias("symbol", joinSymbol),
			esper.Alias("sum(price)", esper.Sum[float64](joinPrice)),
		)
		if isHaving {
			grouped = grouped.Having(esper.Greater[float64](esper.Sum[float64](joinPrice), esper.Literal(50.0)))
		}
		if hasOrderBy {
			options = append(options, esper.OrderBy(esper.Ascending(joinSymbol)))
		}
		return grouped.Query(options...), nil
	}

	grouped := esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
		Window(esper.TimeWindow(5500*time.Millisecond)).
		GroupBy(symbol).
		Select(
			esper.Alias("symbol", symbol),
			esper.Alias("sum(price)", esper.Sum[float64](price)),
		)
	if isHaving {
		grouped = grouped.Having(esper.Greater[float64](esper.Sum[float64](price), esper.Literal(50.0)))
	}
	if hasOrderBy {
		options = append(options, esper.OrderBy(esper.Ascending(symbol)))
	}
	return grouped.Query(options...), nil
}

func resultsetOutputLimitRowPerGroupNoneDefaultRuntimeURI(scenarioID, caseName string) string {
	if scenarioID == resultsetOutputLimitRowPerGroupNoneID {
		switch caseName {
		case resultsetOutputLimitRowPerGroupNoneNoHavingNoJoinCase:
			return resultsetOutputLimitRowPerGroupNoneJavaRuntimeIDs[0]
		case resultsetOutputLimitRowPerGroupNoneNoHavingJoinCase:
			return resultsetOutputLimitRowPerGroupNoneJavaRuntimeIDs[1]
		case resultsetOutputLimitRowPerGroupNoneHavingNoJoinCase:
			return resultsetOutputLimitRowPerGroupNoneJavaRuntimeIDs[2]
		case resultsetOutputLimitRowPerGroupNoneHavingJoinCase:
			return resultsetOutputLimitRowPerGroupNoneJavaRuntimeIDs[3]
		}
	} else {
		switch caseName {
		case resultsetOutputLimitRowPerGroupDefaultNoHavingNoJoinCase:
			return resultsetOutputLimitRowPerGroupDefaultJavaRuntimeIDs[0]
		case resultsetOutputLimitRowPerGroupDefaultNoHavingJoinCase:
			return resultsetOutputLimitRowPerGroupDefaultJavaRuntimeIDs[1]
		case resultsetOutputLimitRowPerGroupDefaultHavingNoJoinCase:
			return resultsetOutputLimitRowPerGroupDefaultJavaRuntimeIDs[2]
		case resultsetOutputLimitRowPerGroupDefaultHavingJoinCase:
			return resultsetOutputLimitRowPerGroupDefaultJavaRuntimeIDs[3]
		}
	}
	return "parity-" + scenarioID + "-" + caseName
}

func decodeResultSetOutputLimitRowPerGroupNoneDefaultPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportMarketDataBean":
		var value resultsetGroupedTimeWindowMarket
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return value, nil
	case "SupportBean":
		var value unidirectionalSupportBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
