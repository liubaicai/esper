package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetOutputLimitRowPerGroupAllJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetOutputLimitRowPerGroupAllJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowPerGroup.java",
}

var (
	// Inventory order is authoritative: ResultSet9AllNoHavingNoJoin is ordinal
	// 9, ResultSet10AllNoHavingJoin 10, ResultSet11AllHavingNoJoin 11, and
	// ResultSet12AllHavingJoin 12 in
	// ResultSetOutputLimitRowPerGroup.executions().
	resultsetOutputLimitRowPerGroupAllJavaRuntimeIDs = []string{
		"java-runtime-3cb45ffbbce3a1039f72", // ResultSet9AllNoHavingNoJoin
		"java-runtime-2cfe8200a666f421582d", // ResultSet10AllNoHavingJoin
		"java-runtime-2a425da1594958f77a88", // ResultSet11AllHavingNoJoin
		"java-runtime-073783d29500f840e025", // ResultSet12AllHavingJoin
	}
	resultsetOutputLimitRowPerGroupAllJavaExecutions = []string{
		"ResultSet9AllNoHavingNoJoin",
		"ResultSet10AllNoHavingJoin",
		"ResultSet11AllHavingNoJoin",
		"ResultSet12AllHavingJoin",
	}
)

const (
	resultsetOutputLimitRowPerGroupAllNoHavingNoJoin = "all-no-having-no-join"
	resultsetOutputLimitRowPerGroupAllNoHavingJoin   = "all-no-having-join"
	resultsetOutputLimitRowPerGroupAllHavingNoJoin   = "all-having-no-join"
	resultsetOutputLimitRowPerGroupAllHavingJoin     = "all-having-join"
	resultsetOutputLimitRowPerGroupAllID             = "resultset-output-limit-row-per-group-all"
)

var resultsetOutputLimitRowPerGroupAllCaseOrder = []string{
	resultsetOutputLimitRowPerGroupAllNoHavingNoJoin,
	resultsetOutputLimitRowPerGroupAllNoHavingJoin,
	resultsetOutputLimitRowPerGroupAllHavingNoJoin,
	resultsetOutputLimitRowPerGroupAllHavingJoin,
}

// runResultSetOutputLimitRowPerGroupAllScenario replays the four
// ResultAssertExecution virtual-time executions from
// ResultSetOutputLimitRowPerGroup ordinals 9-12: grouped sum(price) over
// SupportMarketDataBean#time(5.5 sec) under output all every 1s.  The
// no-having twins add order by symbol; the having twins add having
// sum(price)>50 across the three output-limit-opt hint variants (no order-by —
// the ENABLE hint is a compile error with order-by).  Mirroring
// ResultAssertExecution, each variant runs the EPL twice — the plain istream
// select then the select irstream twin — over the shared ResultAssertInput
// schedule, bracketed by deploy/undeploy-all steps.
func runResultSetOutputLimitRowPerGroupAllScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(resultsetOutputLimitRowPerGroupAllCaseOrder))
	for _, caseName := range resultsetOutputLimitRowPerGroupAllCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetOutputLimitRowPerGroupAllCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOutputLimitRowPerGroupAllID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOutputLimitRowPerGroupAllID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetOutputLimitRowPerGroupAllCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
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
		esper.WithRuntimeURI(resultsetOutputLimitRowPerGroupAllRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultsetOutputLimitRowPerGroupAllID}
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
		query, err := resultsetOutputLimitRowPerGroupAllQuery(env, caseName, deployIndex)
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
				return compat.Trace{}, fmt.Errorf("%s advance-time: %w", resultsetOutputLimitRowPerGroupAllID, err)
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return compat.Trace{}, err
			}
		case "send":
			payload, err := decodeResultSetOutputLimitRowPerGroupAllPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", resultsetOutputLimitRowPerGroupAllID, step.Op)
		}
	}
	return trace, nil
}

// resultsetOutputLimitRowPerGroupAllQuery builds the deploy variant for a
// case.  For the having twins deployIndex/2 selects the output-limit-opt hint
// variant (0=DEFAULT, 1=ENABLED, 2=DISABLED) and deployIndex%2 the istream
// twin; for the no-having cases deployIndex is the istream twin directly (no
// hint variants).  Variant 0 of each pair is the plain istream select (no
// remove stream), variant 1 the select irstream twin (WithOldStream).  The
// no-having cases append order-by-symbol; the having cases add having
// sum(price)>50 (no order-by — the ENABLE hint is a compile error with
// order-by).
func resultsetOutputLimitRowPerGroupAllQuery(env *esper.Environment, caseName string, deployIndex int) (esper.Query, error) {
	hintIndex := deployIndex / 2
	irstream := deployIndex%2 == 1
	switch caseName {
	case resultsetOutputLimitRowPerGroupAllNoHavingNoJoin,
		resultsetOutputLimitRowPerGroupAllNoHavingJoin:
		hintIndex = 0
		irstream = deployIndex == 1
	}

	options := []esper.QueryOption{esper.StatementName("s0")}
	if irstream {
		options = append(options, esper.WithOldStream())
	}
	options = append(options, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))

	having := caseName == resultsetOutputLimitRowPerGroupAllHavingNoJoin ||
		caseName == resultsetOutputLimitRowPerGroupAllHavingJoin
	orderBy := caseName == resultsetOutputLimitRowPerGroupAllNoHavingNoJoin ||
		caseName == resultsetOutputLimitRowPerGroupAllNoHavingJoin
	join := caseName == resultsetOutputLimitRowPerGroupAllNoHavingJoin ||
		caseName == resultsetOutputLimitRowPerGroupAllHavingJoin

	switch caseName {
	case resultsetOutputLimitRowPerGroupAllNoHavingNoJoin,
		resultsetOutputLimitRowPerGroupAllNoHavingJoin,
		resultsetOutputLimitRowPerGroupAllHavingNoJoin,
		resultsetOutputLimitRowPerGroupAllHavingJoin:
	default:
		return esper.Query{}, fmt.Errorf("%s has no case %q", resultsetOutputLimitRowPerGroupAllID, caseName)
	}

	hintOptions, err := resultsetOutputLimitOptHintOptions(hintIndex)
	if err != nil {
		return esper.Query{}, err
	}
	options = append(options, hintOptions...)

	symbol := esper.Field[resultsetGroupedTimeWindowMarket, string]("symbol")
	price := esper.Field[resultsetGroupedTimeWindowMarket, float64]("price")

	if join {
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
		if having {
			grouped = grouped.Having(esper.Greater[float64](esper.Sum[float64](joinPrice), esper.Literal(50.0)))
		}
		if orderBy {
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
	if having {
		grouped = grouped.Having(esper.Greater[float64](esper.Sum[float64](price), esper.Literal(50.0)))
	}
	if orderBy {
		options = append(options, esper.OrderBy(esper.Ascending(symbol)))
	}
	return grouped.Query(options...), nil
}

func resultsetOutputLimitRowPerGroupAllRuntimeURI(caseName string) string {
	switch caseName {
	case resultsetOutputLimitRowPerGroupAllNoHavingNoJoin:
		return resultsetOutputLimitRowPerGroupAllJavaRuntimeIDs[0]
	case resultsetOutputLimitRowPerGroupAllNoHavingJoin:
		return resultsetOutputLimitRowPerGroupAllJavaRuntimeIDs[1]
	case resultsetOutputLimitRowPerGroupAllHavingNoJoin:
		return resultsetOutputLimitRowPerGroupAllJavaRuntimeIDs[2]
	case resultsetOutputLimitRowPerGroupAllHavingJoin:
		return resultsetOutputLimitRowPerGroupAllJavaRuntimeIDs[3]
	default:
		return "parity-" + resultsetOutputLimitRowPerGroupAllID + "-" + caseName
	}
}

func decodeResultSetOutputLimitRowPerGroupAllPayload(step compat.Step) (any, error) {
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
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOutputLimitRowPerGroupAllID, step.EventType)
	}
}
