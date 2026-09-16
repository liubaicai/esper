package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// resultsetOutputLimitRowPerGroupEventsMarket mirrors SupportMarketDataBean.
// Volume and feed are pointers so null stays distinct from zero, exactly like
// the Java Long/String members; the pinned sends carry volume=0 and feed=null.
type resultsetOutputLimitRowPerGroupEventsMarket struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume *int64  `esper:"volume"`
	Feed   *string `esper:"feed"`
}

// resultsetOutputLimitRowPerGroupEventsString mirrors SupportBeanString.
type resultsetOutputLimitRowPerGroupEventsString struct {
	TheString string `esper:"theString"`
}

const resultsetOutputLimitRowPerGroupEventsJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetOutputLimitRowPerGroupEventsJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowPerGroup.java",
}

var (
	// Inventory order is authoritative: GroupByDefault is ordinal 27,
	// NoJoinLast 29, NoJoinAll 32, JoinLast 33 and JoinAll 34 in
	// ResultSetOutputLimitRowPerGroup.executions().
	resultsetOutputLimitRowPerGroupEventsJavaRuntimeIDs = []string{
		"java-runtime-fb1d7cc0c950463969d1", // ResultSetGroupByDefault
		"java-runtime-c55c536922e604536dd8", // ResultSetNoJoinLast
		"java-runtime-33f3e496a6431e2b7d10", // ResultSetNoJoinAll
		"java-runtime-896a57e1bb14df31d330", // ResultSetJoinLast
		"java-runtime-897df7824f16ef7db1c9", // ResultSetJoinAll
	}
	resultsetOutputLimitRowPerGroupEventsJavaExecutions = []string{
		"ResultSetGroupByDefault",
		"ResultSetNoJoinLast",
		"ResultSetNoJoinAll",
		"ResultSetJoinLast",
		"ResultSetJoinAll",
	}
)

const (
	resultsetOutputLimitRowPerGroupEventsGroupByDefault = "group-by-default"
	resultsetOutputLimitRowPerGroupEventsNoJoinLast     = "no-join-last"
	resultsetOutputLimitRowPerGroupEventsNoJoinAll      = "no-join-all"
	resultsetOutputLimitRowPerGroupEventsJoinLast       = "join-last"
	resultsetOutputLimitRowPerGroupEventsJoinAll        = "join-all"
	resultsetOutputLimitRowPerGroupEventsID             = "resultset-output-limit-row-per-group-events"
)

var resultsetOutputLimitRowPerGroupEventsCaseOrder = []string{
	resultsetOutputLimitRowPerGroupEventsGroupByDefault,
	resultsetOutputLimitRowPerGroupEventsNoJoinLast,
	resultsetOutputLimitRowPerGroupEventsNoJoinAll,
	resultsetOutputLimitRowPerGroupEventsJoinLast,
	resultsetOutputLimitRowPerGroupEventsJoinAll,
}

// runResultSetOutputLimitRowPerGroupEventsScenario replays the five
// event-count output-limiting executions from
// ResultSetOutputLimitRowPerGroup. Each case receives a fresh environment and
// runtime, matching the isolated Java execution lifecycle.
func runResultSetOutputLimitRowPerGroupEventsScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(resultsetOutputLimitRowPerGroupEventsCaseOrder))
	for _, caseName := range resultsetOutputLimitRowPerGroupEventsCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetOutputLimitRowPerGroupEventsCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOutputLimitRowPerGroupEventsID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOutputLimitRowPerGroupEventsID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetOutputLimitRowPerGroupEventsCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOutputLimitRowPerGroupEventsMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetOutputLimitRowPerGroupEventsString](env, "SupportBeanString"); err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetOutputLimitRowPerGroupEventsRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultsetOutputLimitRowPerGroupEventsID}
	var sequence uint64
	record := func(batch esper.ResultBatch) {
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
		query, err := resultsetOutputLimitRowPerGroupEventsQuery(env, caseName, deployIndex)
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
		case "send":
			payload, err := decodeResultSetOutputLimitRowPerGroupEventsPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", resultsetOutputLimitRowPerGroupEventsID, step.Op)
		}
	}
	return trace, nil
}

// resultsetOutputLimitRowPerGroupEventsQuery builds the deploy variant for a
// case: group-by-default has a single variant; the last/all cases cycle the
// default, ENABLE_OUTPUTLIMIT_OPT and DISABLE_OUTPUTLIMIT_OPT hint rounds in
// step order, mirroring the suite's SupportOutputLimitOpt loop.
func resultsetOutputLimitRowPerGroupEventsQuery(env *esper.Environment, caseName string, variant int) (esper.Query, error) {
	options, err := resultsetOutputLimitRowPerGroupEventsOptions(variant)
	if err != nil {
		return esper.Query{}, err
	}
	symbol := esper.Field[resultsetOutputLimitRowPerGroupEventsMarket, string]("symbol")
	price := esper.Field[resultsetOutputLimitRowPerGroupEventsMarket, float64]("price")
	filtered := func(stream esper.Stream[resultsetOutputLimitRowPerGroupEventsMarket]) esper.Stream[resultsetOutputLimitRowPerGroupEventsMarket] {
		return stream.Filter(esper.Or(esper.Or(
			esper.Equal[string](symbol, esper.Literal("DELL")),
			esper.Equal[string](symbol, esper.Literal("IBM")),
		), esper.Equal[string](symbol, esper.Literal("GE"))))
	}
	switch caseName {
	case resultsetOutputLimitRowPerGroupEventsGroupByDefault:
		return esper.From[resultsetOutputLimitRowPerGroupEventsMarket](env, "SupportMarketDataBean").
			Window(esper.LengthWindow(5)).
			GroupBy(symbol).
			Select(
				esper.Alias("symbol", symbol),
				esper.Alias("sum(price)", esper.Sum[float64](price)),
			).
			Query(append(options, esper.WithOutput(esper.OutputEvery(5)))...), nil
	case resultsetOutputLimitRowPerGroupEventsNoJoinLast:
		return filtered(esper.From[resultsetOutputLimitRowPerGroupEventsMarket](env, "SupportMarketDataBean").
			Window(esper.LengthWindow(3))).
			GroupBy(symbol).
			Select(
				esper.Alias("symbol", symbol),
				esper.Alias("mySum", esper.Sum[float64](price)),
				esper.Alias("myAvg", esper.Avg[float64](price)),
			).
			Query(append(options, esper.WithOutput(esper.OutputLastEveryEvents(2)))...), nil
	case resultsetOutputLimitRowPerGroupEventsNoJoinAll:
		return filtered(esper.From[resultsetOutputLimitRowPerGroupEventsMarket](env, "SupportMarketDataBean").
			Window(esper.LengthWindow(5))).
			GroupBy(symbol).
			Select(
				esper.Alias("symbol", symbol),
				esper.Alias("mySum", esper.Sum[float64](price)),
				esper.Alias("myAvg", esper.Avg[float64](price)),
			).
			Query(append(options, esper.WithOutput(esper.OutputAllEveryEvents(2)))...), nil
	case resultsetOutputLimitRowPerGroupEventsJoinLast, resultsetOutputLimitRowPerGroupEventsJoinAll:
		theString := esper.Field[resultsetOutputLimitRowPerGroupEventsString, string]("theString")
		left := esper.From[resultsetOutputLimitRowPerGroupEventsString](env, "SupportBeanString").
			Window(esper.LengthWindow(100))
		marketLength := 3
		if caseName == resultsetOutputLimitRowPerGroupEventsJoinAll {
			marketLength = 5
		}
		right := filtered(esper.From[resultsetOutputLimitRowPerGroupEventsMarket](env, "SupportMarketDataBean").
			Window(esper.LengthWindow(marketLength)))
		joinSymbol := esper.JoinField[string](1, "symbol")
		joinPrice := esper.JoinField[float64](1, "price")
		output := esper.OutputLastEveryEvents(2)
		if caseName == resultsetOutputLimitRowPerGroupEventsJoinAll {
			output = esper.OutputAllEveryEvents(2)
		}
		return esper.Join(left, right, esper.OnEqual(theString, symbol)).
			GroupBy(joinSymbol).
			Select(
				esper.Alias("symbol", joinSymbol),
				esper.Alias("mySum", esper.Sum[float64](joinPrice)),
				esper.Alias("myAvg", esper.Avg[float64](joinPrice)),
			).
			Query(append(options, esper.WithOutput(output))...), nil
	default:
		return esper.Query{}, fmt.Errorf("unsupported %s case %q", resultsetOutputLimitRowPerGroupEventsID, caseName)
	}
}

// resultsetOutputLimitRowPerGroupEventsOptions pins the s0 statement name and
// the irstream selector shared by every variant, prepending the round's
// output-limit-optimization hint for variants 1 (ENABLE) and 2 (DISABLE).
func resultsetOutputLimitRowPerGroupEventsOptions(variant int) ([]esper.QueryOption, error) {
	opts := []esper.QueryOption{esper.StatementName("s0"), esper.WithOldStream()}
	var kind esper.StatementHintKind
	switch variant {
	case 0:
	case 1:
		kind = esper.HintEnableOutputLimitOptimization
	case 2:
		kind = esper.HintDisableOutputLimitOptimization
	default:
		return nil, fmt.Errorf("%s has no variant %d", resultsetOutputLimitRowPerGroupEventsID, variant)
	}
	if kind != "" {
		hint, err := esper.NewStatementHint(kind)
		if err != nil {
			return nil, err
		}
		opts = append(opts, esper.WithStatementHints(hint))
	}
	return opts, nil
}

func resultsetOutputLimitRowPerGroupEventsRuntimeURI(caseName string) string {
	switch caseName {
	case resultsetOutputLimitRowPerGroupEventsGroupByDefault:
		return resultsetOutputLimitRowPerGroupEventsJavaRuntimeIDs[0]
	case resultsetOutputLimitRowPerGroupEventsNoJoinLast:
		return resultsetOutputLimitRowPerGroupEventsJavaRuntimeIDs[1]
	case resultsetOutputLimitRowPerGroupEventsNoJoinAll:
		return resultsetOutputLimitRowPerGroupEventsJavaRuntimeIDs[2]
	case resultsetOutputLimitRowPerGroupEventsJoinLast:
		return resultsetOutputLimitRowPerGroupEventsJavaRuntimeIDs[3]
	case resultsetOutputLimitRowPerGroupEventsJoinAll:
		return resultsetOutputLimitRowPerGroupEventsJavaRuntimeIDs[4]
	default:
		return "parity-" + resultsetOutputLimitRowPerGroupEventsID + "-" + caseName
	}
}

func decodeResultSetOutputLimitRowPerGroupEventsPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportMarketDataBean":
		var value resultsetOutputLimitRowPerGroupEventsMarket
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return value, nil
	case "SupportBeanString":
		var value resultsetOutputLimitRowPerGroupEventsString
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBeanString: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOutputLimitRowPerGroupEventsID, step.EventType)
	}
}
