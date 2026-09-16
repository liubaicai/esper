package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetOutputLimitRowPerGroupHavingFirstSnapJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetOutputLimitRowPerGroupHavingFirstSnapJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowPerGroup.java",
}

var (
	// Inventory order is authoritative: ResultSet15LastHavingNoJoin is ordinal
	// 17, ResultSet16LastHavingJoin 18, the ResultSet17First twins 19/20, and
	// the ResultSet18Snapshot twins 21/22 in
	// ResultSetOutputLimitRowPerGroup.executions().
	resultsetOutputLimitRowPerGroupHavingFirstSnapJavaRuntimeIDs = []string{
		"java-runtime-0537627a9e2ced9a101d", // ResultSet15LastHavingNoJoin
		"java-runtime-5662e97901c7b1960530", // ResultSet16LastHavingJoin
		"java-runtime-7c3c8427c5d48c24774e", // ResultSet17FirstNoHavingNoJoin
		"java-runtime-54b18a72ff7fa42b3afe", // ResultSet17FirstNoHavingJoin
		"java-runtime-ebfb2b66f2dd8778a08b", // ResultSet18SnapshotNoHavingNoJoin
		"java-runtime-5d74da396c739c122464", // ResultSet18SnapshotNoHavingJoin
	}
	resultsetOutputLimitRowPerGroupHavingFirstSnapJavaExecutions = []string{
		"ResultSet15LastHavingNoJoin",
		"ResultSet16LastHavingJoin",
		"ResultSet17FirstNoHavingNoJoin",
		"ResultSet17FirstNoHavingJoin",
		"ResultSet18SnapshotNoHavingNoJoin",
		"ResultSet18SnapshotNoHavingJoin",
	}
)

const (
	resultsetOutputLimitRowPerGroupHavingFirstSnapLastNoJoin     = "last-having-no-join"
	resultsetOutputLimitRowPerGroupHavingFirstSnapLastJoin       = "last-having-join"
	resultsetOutputLimitRowPerGroupHavingFirstSnapFirstNoJoin    = "first-no-having-no-join"
	resultsetOutputLimitRowPerGroupHavingFirstSnapFirstJoin      = "first-no-having-join"
	resultsetOutputLimitRowPerGroupHavingFirstSnapSnapshotNoJoin = "snapshot-no-having-no-join"
	resultsetOutputLimitRowPerGroupHavingFirstSnapSnapshotJoin   = "snapshot-no-having-join"
	resultsetOutputLimitRowPerGroupHavingFirstSnapID             = "resultset-output-limit-row-per-group-having-first-snap"
)

var resultsetOutputLimitRowPerGroupHavingFirstSnapCaseOrder = []string{
	resultsetOutputLimitRowPerGroupHavingFirstSnapLastNoJoin,
	resultsetOutputLimitRowPerGroupHavingFirstSnapLastJoin,
	resultsetOutputLimitRowPerGroupHavingFirstSnapFirstNoJoin,
	resultsetOutputLimitRowPerGroupHavingFirstSnapFirstJoin,
	resultsetOutputLimitRowPerGroupHavingFirstSnapSnapshotNoJoin,
	resultsetOutputLimitRowPerGroupHavingFirstSnapSnapshotJoin,
}

// runResultSetOutputLimitRowPerGroupHavingFirstSnapScenario replays the six
// ResultAssertExecution virtual-time executions from
// ResultSetOutputLimitRowPerGroup ordinals 17-22.  The last-having twins add
// having sum(price)>50 under output last every 1s across the three
// output-limit-opt hint variants; the first twins use output first every 1s;
// the snapshot twins use output snapshot every 1s order by symbol.  Mirroring
// ResultAssertExecution, each variant runs the EPL twice — the plain istream
// select then the select irstream twin — over the shared ResultAssertInput
// schedule, bracketed by deploy/undeploy-all steps.
func runResultSetOutputLimitRowPerGroupHavingFirstSnapScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(resultsetOutputLimitRowPerGroupHavingFirstSnapCaseOrder))
	for _, caseName := range resultsetOutputLimitRowPerGroupHavingFirstSnapCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetOutputLimitRowPerGroupHavingFirstSnapCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOutputLimitRowPerGroupHavingFirstSnapID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOutputLimitRowPerGroupHavingFirstSnapID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetOutputLimitRowPerGroupHavingFirstSnapCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
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
		esper.WithRuntimeURI(resultsetOutputLimitRowPerGroupHavingFirstSnapRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultsetOutputLimitRowPerGroupHavingFirstSnapID}
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
		query, err := resultsetOutputLimitRowPerGroupHavingFirstSnapQuery(env, caseName, deployIndex)
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
				return compat.Trace{}, fmt.Errorf("%s advance-time: %w", resultsetOutputLimitRowPerGroupHavingFirstSnapID, err)
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return compat.Trace{}, err
			}
		case "send":
			payload, err := decodeResultSetOutputLimitRowPerGroupHavingFirstSnapPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", resultsetOutputLimitRowPerGroupHavingFirstSnapID, step.Op)
		}
	}
	return trace, nil
}

// resultsetOutputLimitRowPerGroupHavingFirstSnapQuery builds the deploy
// variant for a case.  For the last-having twins deployIndex/2 selects the
// output-limit-opt hint variant (0=DEFAULT, 1=ENABLED, 2=DISABLED) and
// deployIndex%2 the istream twin; for the first/snapshot cases deployIndex is
// the istream twin directly (no hint variants).  Variant 0 of each pair is the
// plain istream select (no remove stream), variant 1 the select irstream twin
// (WithOldStream).  The snapshot cases append order-by-symbol.
func resultsetOutputLimitRowPerGroupHavingFirstSnapQuery(env *esper.Environment, caseName string, deployIndex int) (esper.Query, error) {
	hintIndex := deployIndex / 2
	irstream := deployIndex%2 == 1
	switch caseName {
	case resultsetOutputLimitRowPerGroupHavingFirstSnapFirstNoJoin,
		resultsetOutputLimitRowPerGroupHavingFirstSnapFirstJoin,
		resultsetOutputLimitRowPerGroupHavingFirstSnapSnapshotNoJoin,
		resultsetOutputLimitRowPerGroupHavingFirstSnapSnapshotJoin:
		hintIndex = 0
		irstream = deployIndex == 1
	}

	options := []esper.QueryOption{esper.StatementName("s0")}
	if irstream {
		options = append(options, esper.WithOldStream())
	}

	having := caseName == resultsetOutputLimitRowPerGroupHavingFirstSnapLastNoJoin ||
		caseName == resultsetOutputLimitRowPerGroupHavingFirstSnapLastJoin
	orderBy := caseName == resultsetOutputLimitRowPerGroupHavingFirstSnapSnapshotNoJoin ||
		caseName == resultsetOutputLimitRowPerGroupHavingFirstSnapSnapshotJoin
	join := caseName == resultsetOutputLimitRowPerGroupHavingFirstSnapLastJoin ||
		caseName == resultsetOutputLimitRowPerGroupHavingFirstSnapFirstJoin ||
		caseName == resultsetOutputLimitRowPerGroupHavingFirstSnapSnapshotJoin

	switch caseName {
	case resultsetOutputLimitRowPerGroupHavingFirstSnapLastNoJoin,
		resultsetOutputLimitRowPerGroupHavingFirstSnapLastJoin:
		options = append(options, esper.WithOutput(esper.OutputLastEveryTime(time.Second)))
	case resultsetOutputLimitRowPerGroupHavingFirstSnapFirstNoJoin,
		resultsetOutputLimitRowPerGroupHavingFirstSnapFirstJoin:
		options = append(options, esper.WithOutput(esper.OutputFirstEveryTime(time.Second)))
	case resultsetOutputLimitRowPerGroupHavingFirstSnapSnapshotNoJoin,
		resultsetOutputLimitRowPerGroupHavingFirstSnapSnapshotJoin:
		options = append(options, esper.WithOutput(esper.OutputSnapshotEvery(time.Second)))
	default:
		return esper.Query{}, fmt.Errorf("%s has no case %q", resultsetOutputLimitRowPerGroupHavingFirstSnapID, caseName)
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

// resultsetOutputLimitOptHintOptions maps the SupportOutputLimitOpt hint index
// to the Go statement-hint query option: 0=DEFAULT (no hint), 1=ENABLED,
// 2=DISABLED.
func resultsetOutputLimitOptHintOptions(hintIndex int) ([]esper.QueryOption, error) {
	var kind esper.StatementHintKind
	switch hintIndex {
	case 0:
		return nil, nil
	case 1:
		kind = esper.HintEnableOutputLimitOptimization
	case 2:
		kind = esper.HintDisableOutputLimitOptimization
	default:
		return nil, fmt.Errorf("no output-limit-opt hint variant %d", hintIndex)
	}
	hint, err := esper.NewStatementHint(kind)
	if err != nil {
		return nil, err
	}
	return []esper.QueryOption{esper.WithStatementHints(hint)}, nil
}

func resultsetOutputLimitRowPerGroupHavingFirstSnapRuntimeURI(caseName string) string {
	switch caseName {
	case resultsetOutputLimitRowPerGroupHavingFirstSnapLastNoJoin:
		return resultsetOutputLimitRowPerGroupHavingFirstSnapJavaRuntimeIDs[0]
	case resultsetOutputLimitRowPerGroupHavingFirstSnapLastJoin:
		return resultsetOutputLimitRowPerGroupHavingFirstSnapJavaRuntimeIDs[1]
	case resultsetOutputLimitRowPerGroupHavingFirstSnapFirstNoJoin:
		return resultsetOutputLimitRowPerGroupHavingFirstSnapJavaRuntimeIDs[2]
	case resultsetOutputLimitRowPerGroupHavingFirstSnapFirstJoin:
		return resultsetOutputLimitRowPerGroupHavingFirstSnapJavaRuntimeIDs[3]
	case resultsetOutputLimitRowPerGroupHavingFirstSnapSnapshotNoJoin:
		return resultsetOutputLimitRowPerGroupHavingFirstSnapJavaRuntimeIDs[4]
	case resultsetOutputLimitRowPerGroupHavingFirstSnapSnapshotJoin:
		return resultsetOutputLimitRowPerGroupHavingFirstSnapJavaRuntimeIDs[5]
	default:
		return "parity-" + resultsetOutputLimitRowPerGroupHavingFirstSnapID + "-" + caseName
	}
}

func decodeResultSetOutputLimitRowPerGroupHavingFirstSnapPayload(step compat.Step) (any, error) {
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
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOutputLimitRowPerGroupHavingFirstSnapID, step.EventType)
	}
}
