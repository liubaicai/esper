package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetOutputLimitRowPerGroupLastJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetOutputLimitRowPerGroupLastJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowPerGroup.java",
}

var (
	// Inventory order is authoritative: ResultSet13LastNoHavingNoJoin is
	// ordinal 13, ResultSet14LastNoHavingJoin 14, and the WOrderBy twins 15/16
	// in ResultSetOutputLimitRowPerGroup.executions().
	resultsetOutputLimitRowPerGroupLastJavaRuntimeIDs = []string{
		"java-runtime-930299a6192880bda3bc", // ResultSet13LastNoHavingNoJoin
		"java-runtime-79057f1ac92150b68e68", // ResultSet14LastNoHavingJoin
		"java-runtime-c30ea1262639b9008249", // ResultSet13LastNoHavingNoJoinWOrderBy
		"java-runtime-101fbc773a180b386246", // ResultSet14LastNoHavingJoinWOrderBy
	}
	resultsetOutputLimitRowPerGroupLastJavaExecutions = []string{
		"ResultSet13LastNoHavingNoJoin",
		"ResultSet14LastNoHavingJoin",
		"ResultSet13LastNoHavingNoJoinWOrderBy",
		"ResultSet14LastNoHavingJoinWOrderBy",
	}
)

const (
	resultsetOutputLimitRowPerGroupLastNoJoin        = "last-no-having-no-join"
	resultsetOutputLimitRowPerGroupLastJoin          = "last-no-having-join"
	resultsetOutputLimitRowPerGroupLastNoJoinOrderBy = "last-no-having-no-join-worderby"
	resultsetOutputLimitRowPerGroupLastJoinOrderBy   = "last-no-having-join-worderby"
	resultsetOutputLimitRowPerGroupLastID            = "resultset-output-limit-row-per-group-last"
)

var resultsetOutputLimitRowPerGroupLastCaseOrder = []string{
	resultsetOutputLimitRowPerGroupLastNoJoin,
	resultsetOutputLimitRowPerGroupLastJoin,
	resultsetOutputLimitRowPerGroupLastNoJoinOrderBy,
	resultsetOutputLimitRowPerGroupLastJoinOrderBy,
}

// runResultSetOutputLimitRowPerGroupLastScenario replays the four
// ResultAssertExecution virtual-time executions from
// ResultSetOutputLimitRowPerGroup. Each case receives a fresh environment and
// runtime, matching the isolated Java execution lifecycle; each case runs the
// EPL twice (plain istream then select irstream) over the shared schedule.
func runResultSetOutputLimitRowPerGroupLastScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(resultsetOutputLimitRowPerGroupLastCaseOrder))
	for _, caseName := range resultsetOutputLimitRowPerGroupLastCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetOutputLimitRowPerGroupLastCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOutputLimitRowPerGroupLastID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOutputLimitRowPerGroupLastID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetOutputLimitRowPerGroupLastCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
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
		esper.WithRuntimeURI(resultsetOutputLimitRowPerGroupLastRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultsetOutputLimitRowPerGroupLastID}
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
		query, err := resultsetOutputLimitRowPerGroupLastQuery(env, caseName, deployIndex)
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
				return compat.Trace{}, fmt.Errorf("%s advance-time: %w", resultsetOutputLimitRowPerGroupLastID, err)
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return compat.Trace{}, err
			}
		case "send":
			payload, err := decodeResultSetOutputLimitRowPerGroupLastPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", resultsetOutputLimitRowPerGroupLastID, step.Op)
		}
	}
	return trace, nil
}

// resultsetOutputLimitRowPerGroupLastQuery builds the deploy variant for a
// case: variant 0 is the plain istream select (no remove stream), variant 1 is
// the select irstream twin (WithOldStream). The WOrderBy cases append
// order-by-symbol.
func resultsetOutputLimitRowPerGroupLastQuery(env *esper.Environment, caseName string, variant int) (esper.Query, error) {
	if variant != 0 && variant != 1 {
		return esper.Query{}, fmt.Errorf("%s has no variant %d", resultsetOutputLimitRowPerGroupLastID, variant)
	}
	options := []esper.QueryOption{esper.StatementName("s0")}
	if variant == 1 {
		options = append(options, esper.WithOldStream())
	}
	options = append(options, esper.WithOutput(esper.OutputLastEveryTime(time.Second)))

	symbol := esper.Field[resultsetGroupedTimeWindowMarket, string]("symbol")
	price := esper.Field[resultsetGroupedTimeWindowMarket, float64]("price")
	sum := esper.Sum[float64](price)

	join := caseName == resultsetOutputLimitRowPerGroupLastJoin || caseName == resultsetOutputLimitRowPerGroupLastJoinOrderBy
	orderBy := caseName == resultsetOutputLimitRowPerGroupLastNoJoinOrderBy || caseName == resultsetOutputLimitRowPerGroupLastJoinOrderBy

	if join {
		theString := esper.Field[unidirectionalSupportBean, string]("theString")
		joinSymbol := esper.JoinField[string](0, "symbol")
		joinPrice := esper.JoinField[float64](0, "price")
		joined := esper.Join(
			esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
				Window(esper.TimeWindow(5500*time.Millisecond)),
			esper.From[unidirectionalSupportBean](env, "SupportBean").
				Window(esper.KeepAll()),
			esper.OnEqual(symbol, theString),
		)
		grouped := joined.GroupBy(joinSymbol).Select(
			esper.Alias("symbol", joinSymbol),
			esper.Alias("sum(price)", esper.Sum[float64](joinPrice)),
		)
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
			esper.Alias("sum(price)", sum),
		)
	if orderBy {
		options = append(options, esper.OrderBy(esper.Ascending(symbol)))
	}
	return grouped.Query(options...), nil
}

func resultsetOutputLimitRowPerGroupLastRuntimeURI(caseName string) string {
	switch caseName {
	case resultsetOutputLimitRowPerGroupLastNoJoin:
		return resultsetOutputLimitRowPerGroupLastJavaRuntimeIDs[0]
	case resultsetOutputLimitRowPerGroupLastJoin:
		return resultsetOutputLimitRowPerGroupLastJavaRuntimeIDs[1]
	case resultsetOutputLimitRowPerGroupLastNoJoinOrderBy:
		return resultsetOutputLimitRowPerGroupLastJavaRuntimeIDs[2]
	case resultsetOutputLimitRowPerGroupLastJoinOrderBy:
		return resultsetOutputLimitRowPerGroupLastJavaRuntimeIDs[3]
	default:
		return "parity-" + resultsetOutputLimitRowPerGroupLastID + "-" + caseName
	}
}

func decodeResultSetOutputLimitRowPerGroupLastPayload(step compat.Step) (any, error) {
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
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOutputLimitRowPerGroupLastID, step.EventType)
	}
}
