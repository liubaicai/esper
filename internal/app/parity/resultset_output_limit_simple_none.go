package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetOutputLimitSimpleNoneJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetOutputLimitSimpleNoneJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitSimple.java",
}

var (
	// Inventory order is authoritative: ResultSet1NoneNoHavingNoJoin is ordinal 0
	// through ResultSet4NoneHavingJoin ordinal 3 in
	// ResultSetOutputLimitSimple.executions().
	resultsetOutputLimitSimpleNoneJavaRuntimeIDs = []string{
		"java-runtime-427e3f367e9556c57969", // ResultSet1NoneNoHavingNoJoin
		"java-runtime-74646a15e15f49fdd1a1", // ResultSet2NoneNoHavingJoin
		"java-runtime-eb78eaf610806ed398a6", // ResultSet3NoneHavingNoJoin
		"java-runtime-ddd51d27fc7e1de1efb8", // ResultSet4NoneHavingJoin
	}
	resultsetOutputLimitSimpleNoneJavaExecutions = []string{
		"ResultSet1NoneNoHavingNoJoin",
		"ResultSet2NoneNoHavingJoin",
		"ResultSet3NoneHavingNoJoin",
		"ResultSet4NoneHavingJoin",
	}
)

const (
	resultsetOutputLimitSimpleNoneID = "resultset-outputlimit-simple-none"

	resultsetOutputLimitSimpleNoneNoHavingNoJoinCase = "none-no-having-no-join"
	resultsetOutputLimitSimpleNoneNoHavingJoinCase   = "none-no-having-join"
	resultsetOutputLimitSimpleNoneHavingNoJoinCase   = "none-having-no-join"
	resultsetOutputLimitSimpleNoneHavingJoinCase     = "none-having-join"
)

var resultsetOutputLimitSimpleNoneCaseOrder = []string{
	resultsetOutputLimitSimpleNoneNoHavingNoJoinCase,
	resultsetOutputLimitSimpleNoneNoHavingJoinCase,
	resultsetOutputLimitSimpleNoneHavingNoJoinCase,
	resultsetOutputLimitSimpleNoneHavingJoinCase,
}

// runResultSetOutputLimitSimpleNoneScenario replays the four no-output-clause
// (per-event immediate output) executions from ResultSetOutputLimitSimple
// ordinals 0-3: ungrouped symbol/volume/price rows over
// SupportMarketDataBean#time(5.5 sec).  The join twins add
// SupportBean#keepall on theString=symbol; the having twins add
// having price > 10.  Mirroring ResultAssertExecution, each case runs the EPL
// twice — the plain istream select then the select irstream twin — over the
// shared ResultAssertInput schedule, bracketed by deploy/undeploy-all steps.
func runResultSetOutputLimitSimpleNoneScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if err := validateResultSetOutputLimitSimpleNoneScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(resultsetOutputLimitSimpleNoneCaseOrder))
	for _, caseName := range resultsetOutputLimitSimpleNoneCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetOutputLimitSimpleNoneCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOutputLimitSimpleNoneID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOutputLimitSimpleNoneID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

// validateResultSetOutputLimitSimpleNoneScenario pins the scenario metadata
// against the Java contract: version, id, and per-case order.
func validateResultSetOutputLimitSimpleNoneScenario(scenario compat.Scenario) error {
	if scenario.Version != compat.ScenarioVersion || scenario.ID != resultsetOutputLimitSimpleNoneID {
		return fmt.Errorf("%s scenario metadata is not pinned", resultsetOutputLimitSimpleNoneID)
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("%s scenario has no steps", resultsetOutputLimitSimpleNoneID)
	}
	caseIndex := 0
	for _, step := range scenario.Steps {
		if step.Op != "case" {
			continue
		}
		if caseIndex >= len(resultsetOutputLimitSimpleNoneCaseOrder) {
			return fmt.Errorf("%s scenario has unexpected case %q", resultsetOutputLimitSimpleNoneID, step.Case)
		}
		if step.Case != resultsetOutputLimitSimpleNoneCaseOrder[caseIndex] {
			return fmt.Errorf("%s scenario case %d = %q, want %q", resultsetOutputLimitSimpleNoneID, caseIndex, step.Case, resultsetOutputLimitSimpleNoneCaseOrder[caseIndex])
		}
		caseIndex++
	}
	if caseIndex != len(resultsetOutputLimitSimpleNoneCaseOrder) {
		return fmt.Errorf("%s scenario has %d cases, want %d", resultsetOutputLimitSimpleNoneID, caseIndex, len(resultsetOutputLimitSimpleNoneCaseOrder))
	}
	return nil
}

func runResultSetOutputLimitSimpleNoneCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
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
		esper.WithRuntimeURI(resultsetOutputLimitSimpleNoneRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultsetOutputLimitSimpleNoneID}
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
		query, err := resultsetOutputLimitSimpleNoneQuery(env, caseName, deployIndex)
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
				return compat.Trace{}, fmt.Errorf("%s advance-time: %w", resultsetOutputLimitSimpleNoneID, err)
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return compat.Trace{}, err
			}
		case "send":
			payload, err := decodeResultSetOutputLimitSimpleNonePayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", resultsetOutputLimitSimpleNoneID, step.Op)
		}
	}
	return trace, nil
}

// resultsetOutputLimitSimpleNoneQuery builds the deploy variant for a case.
// deployIndex selects the istream twin: variant 0 is the plain istream select
// (no remove stream), variant 1 the select irstream twin (WithOldStream).  All
// cases carry no output clause (per-event immediate output).  The join twins
// add SupportBean#keepall on theString=symbol; the having twins add
// having price > 10.  The no-join having maps to a Filter below the window:
// Esper's ungrouped having suppresses the row on both insert and expiry, and
// a pre-window filter reproduces that observable contract exactly (a failing
// event never enters the window, so it emits no new row and no old row).
func resultsetOutputLimitSimpleNoneQuery(env *esper.Environment, caseName string, deployIndex int) (esper.Query, error) {
	irstream := deployIndex == 1

	isJoin := caseName == resultsetOutputLimitSimpleNoneNoHavingJoinCase ||
		caseName == resultsetOutputLimitSimpleNoneHavingJoinCase
	isHaving := caseName == resultsetOutputLimitSimpleNoneHavingNoJoinCase ||
		caseName == resultsetOutputLimitSimpleNoneHavingJoinCase

	switch caseName {
	case resultsetOutputLimitSimpleNoneNoHavingNoJoinCase,
		resultsetOutputLimitSimpleNoneNoHavingJoinCase,
		resultsetOutputLimitSimpleNoneHavingNoJoinCase,
		resultsetOutputLimitSimpleNoneHavingJoinCase:
	default:
		return esper.Query{}, fmt.Errorf("%s has no case %q", resultsetOutputLimitSimpleNoneID, caseName)
	}

	options := []esper.QueryOption{esper.StatementName("s0")}
	if irstream {
		options = append(options, esper.WithOldStream())
	}

	symbol := esper.Field[resultsetGroupedTimeWindowMarket, string]("symbol")
	volume := esper.Field[resultsetGroupedTimeWindowMarket, int64]("volume")
	price := esper.Field[resultsetGroupedTimeWindowMarket, float64]("price")

	if isJoin {
		theString := esper.Field[unidirectionalSupportBean, string]("theString")
		joinSymbol := esper.JoinField[string](0, "symbol")
		joinVolume := esper.JoinField[int64](0, "volume")
		joinPrice := esper.JoinField[float64](0, "price")
		joined := esper.Join(
			esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean").
				Window(esper.TimeWindow(5500*time.Millisecond)),
			esper.From[unidirectionalSupportBean](env, "SupportBean").
				Window(esper.KeepAll()),
			esper.OnEqual(symbol, theString),
		).Select(
			esper.SelectFrom(0, "symbol", joinSymbol),
			esper.SelectFrom(0, "volume", joinVolume),
			esper.SelectFrom(0, "price", joinPrice),
		)
		if isHaving {
			joined = joined.Having(esper.Greater[float64](joinPrice, esper.Literal(10.0)))
		}
		return joined.Query(options...), nil
	}

	source := esper.From[resultsetGroupedTimeWindowMarket](env, "SupportMarketDataBean")
	if isHaving {
		// Esper's ungrouped having suppresses the row on insert and on
		// expiry; a pre-window filter reproduces that observable contract
		// exactly (see the function comment).
		source = source.Filter(esper.Greater[float64](price, esper.Literal(10.0)))
	}
	selected := esper.Select[resultsetGroupedTimeWindowMarket](
		source.Window(esper.TimeWindow(5500*time.Millisecond)),
		esper.Alias("symbol", symbol),
		esper.Alias("volume", volume),
		esper.Alias("price", price),
	)
	return selected.Query(options...), nil
}

func resultsetOutputLimitSimpleNoneRuntimeURI(caseName string) string {
	switch caseName {
	case resultsetOutputLimitSimpleNoneNoHavingNoJoinCase:
		return resultsetOutputLimitSimpleNoneJavaRuntimeIDs[0]
	case resultsetOutputLimitSimpleNoneNoHavingJoinCase:
		return resultsetOutputLimitSimpleNoneJavaRuntimeIDs[1]
	case resultsetOutputLimitSimpleNoneHavingNoJoinCase:
		return resultsetOutputLimitSimpleNoneJavaRuntimeIDs[2]
	case resultsetOutputLimitSimpleNoneHavingJoinCase:
		return resultsetOutputLimitSimpleNoneJavaRuntimeIDs[3]
	}
	return "parity-" + resultsetOutputLimitSimpleNoneID + "-" + caseName
}

func decodeResultSetOutputLimitSimpleNonePayload(step compat.Step) (any, error) {
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

const resultsetOutputLimitSimpleNoneDescription = "ResultSetOutputLimitSimple ordinals 0-3 (ResultAssertExecution virtual-time cluster): ungrouped symbol, volume, price rows over SupportMarketDataBean#time(5.5 sec) with no output clause (per-event immediate output). The join twins add SupportBean#keepall on theString=symbol; the having twins add having price > 10. Each case runs the EPL twice (plain istream then select irstream) over the shared ResultAssertInput schedule."

var (
	resultsetOutputLimitSimpleNoneJavaStaticIDs = []string{
		"java-81527f2083bb83f54e8d",
		"java-33954d9fd7b1f13ec109",
		"java-b18c92b42741b1b7b755",
		"java-0614eb588c9cc3219675",
	}
	resultsetOutputLimitSimpleNoneEPLs = []string{
		"@name('s0') select symbol, volume, price from SupportMarketDataBean#time(5.5 sec)",
		"@name('s0') select symbol, volume, price from SupportMarketDataBean#time(5.5 sec), SupportBean#keepall where theString=symbol",
		"@name('s0') select symbol, volume, price from SupportMarketDataBean#time(5.5 sec)  having price > 10",
		"@name('s0') select symbol, volume, price from SupportMarketDataBean#time(5.5 sec), SupportBean#keepall where theString=symbol  having price > 10",
	}
)

// resultsetOutputLimitSimpleNoneExpectedStep is one pinned schedule entry:
// either an advance-time anchor or a send with an exact payload.
type resultsetOutputLimitSimpleNoneExpectedStep struct {
	op        string
	at        string
	eventType string
	symbol    string
	volume    int64
	price     float64
	theString string
}

// resultsetOutputLimitSimpleNoneSchedule is the shared ResultAssertInput
// schedule replayed twice per case (plain istream then select irstream):
// three SupportBean keepall rows, then the timed market-data sends.
var resultsetOutputLimitSimpleNoneSchedule = []resultsetOutputLimitSimpleNoneExpectedStep{
	{op: "send", eventType: "SupportBean", theString: "IBM"},
	{op: "send", eventType: "SupportBean", theString: "MSFT"},
	{op: "send", eventType: "SupportBean", theString: "YAH"},
	{op: "advance-time", at: "1970-01-01T00:00:00.200Z"},
	{op: "send", eventType: "SupportMarketDataBean", symbol: "IBM", volume: 100, price: 25},
	{op: "advance-time", at: "1970-01-01T00:00:00.800Z"},
	{op: "send", eventType: "SupportMarketDataBean", symbol: "MSFT", volume: 5000, price: 9},
	{op: "advance-time", at: "1970-01-01T00:00:01Z"},
	{op: "advance-time", at: "1970-01-01T00:00:01.200Z"},
	{op: "advance-time", at: "1970-01-01T00:00:01.500Z"},
	{op: "send", eventType: "SupportMarketDataBean", symbol: "IBM", volume: 150, price: 24},
	{op: "send", eventType: "SupportMarketDataBean", symbol: "YAH", volume: 10000, price: 1},
	{op: "advance-time", at: "1970-01-01T00:00:02Z"},
	{op: "advance-time", at: "1970-01-01T00:00:02.100Z"},
	{op: "send", eventType: "SupportMarketDataBean", symbol: "IBM", volume: 155, price: 26},
	{op: "advance-time", at: "1970-01-01T00:00:02.200Z"},
	{op: "advance-time", at: "1970-01-01T00:00:02.500Z"},
	{op: "advance-time", at: "1970-01-01T00:00:03Z"},
	{op: "advance-time", at: "1970-01-01T00:00:03.200Z"},
	{op: "advance-time", at: "1970-01-01T00:00:03.500Z"},
	{op: "send", eventType: "SupportMarketDataBean", symbol: "YAH", volume: 11000, price: 2},
	{op: "advance-time", at: "1970-01-01T00:00:04Z"},
	{op: "advance-time", at: "1970-01-01T00:00:04.200Z"},
	{op: "advance-time", at: "1970-01-01T00:00:04.300Z"},
	{op: "send", eventType: "SupportMarketDataBean", symbol: "IBM", volume: 150, price: 22},
	{op: "advance-time", at: "1970-01-01T00:00:04.900Z"},
	{op: "send", eventType: "SupportMarketDataBean", symbol: "YAH", volume: 11500, price: 3},
	{op: "advance-time", at: "1970-01-01T00:00:05Z"},
	{op: "advance-time", at: "1970-01-01T00:00:05.200Z"},
	{op: "advance-time", at: "1970-01-01T00:00:05.700Z"},
	{op: "advance-time", at: "1970-01-01T00:00:05.900Z"},
	{op: "send", eventType: "SupportMarketDataBean", symbol: "YAH", volume: 10500, price: 1},
	{op: "advance-time", at: "1970-01-01T00:00:06Z"},
	{op: "advance-time", at: "1970-01-01T00:00:06.200Z"},
	{op: "advance-time", at: "1970-01-01T00:00:06.300Z"},
	{op: "advance-time", at: "1970-01-01T00:00:07Z"},
	{op: "advance-time", at: "1970-01-01T00:00:07.200Z"},
}

// loadResultSetOutputLimitSimpleNoneScenario decodes the checked-in scenario
// with strict raw-bytes validation: duplicate keys, unknown fields, metadata
// drift, and payload drift are all rejected before replay.
func loadResultSetOutputLimitSimpleNoneScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOutputLimitSimpleNoneID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOutputLimitSimpleNoneID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitSimpleNoneID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitSimpleNoneID, err)
	}
	if err := requireResultSetOutputLimitSimpleNoneFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version     string `json:"version"`
		ID          string `json:"id"`
		Description string `json:"description"`
		JavaCommit  string `json:"javaCommit"`
		JavaSource  string `json:"javaSource"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOutputLimitSimpleNoneID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetOutputLimitSimpleNoneID ||
		metadata.Description != resultsetOutputLimitSimpleNoneDescription ||
		metadata.JavaCommit != resultsetOutputLimitSimpleNoneJavaCommit ||
		metadata.JavaSource != resultsetOutputLimitSimpleNoneJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOutputLimitSimpleNoneID)
	}
	if err := validateResultSetOutputLimitSimpleNoneStringArray(root["javaRuntimes"], resultsetOutputLimitSimpleNoneJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetOutputLimitSimpleNoneStringArray(root["javaNames"], resultsetOutputLimitSimpleNoneJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetOutputLimitSimpleNoneStringArray(root["javaStaticIds"], resultsetOutputLimitSimpleNoneJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetOutputLimitSimpleNoneStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetOutputLimitSimpleNoneCaseOrder) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly four cases", resultsetOutputLimitSimpleNoneID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultSetOutputLimitSimpleNoneFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case          string `json:"case"`
			Ordinal       int    `json:"ordinal"`
			RuntimeID     string `json:"runtimeId"`
			ExecutionName string `json:"executionName"`
			Observation   string `json:"observation"`
			EPL           string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if definition.Case != resultsetOutputLimitSimpleNoneCaseOrder[index] ||
			definition.Ordinal != index ||
			definition.RuntimeID != resultsetOutputLimitSimpleNoneJavaRuntimeIDs[index] ||
			definition.ExecutionName != resultsetOutputLimitSimpleNoneJavaExecutions[index] ||
			definition.Observation != "listener" ||
			definition.EPL != resultsetOutputLimitSimpleNoneEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetOutputLimitSimpleNoneID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 320 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 320 steps", resultsetOutputLimitSimpleNoneID)
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		switch operation {
		case "case":
			if err := requireResultSetOutputLimitSimpleNoneFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireResultSetOutputLimitSimpleNoneFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireResultSetOutputLimitSimpleNoneFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "advance-time":
			if err := requireResultSetOutputLimitSimpleNoneFields(object, "op", "case", "at"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireResultSetOutputLimitSimpleNoneFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeResultSetOutputLimitSimpleNoneStrictPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateResultSetOutputLimitSimpleNoneScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetOutputLimitSimpleNoneSchedule(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateResultSetOutputLimitSimpleNoneSchedule walks the decoded steps and
// pins the full per-case schedule: case marker, epoch anchor, then two
// deploy/schedule/undeploy-all passes over the shared ResultAssertInput
// timetable.
func validateResultSetOutputLimitSimpleNoneSchedule(scenario compat.Scenario) error {
	index := 0
	for caseIndex, caseName := range resultsetOutputLimitSimpleNoneCaseOrder {
		if step := scenario.Steps[index]; step.Op != "case" || step.Case != caseName {
			return fmt.Errorf("%s scenario step %d must start case %q", resultsetOutputLimitSimpleNoneID, index, caseName)
		}
		index++
		if step := scenario.Steps[index]; step.Op != "advance-time" || step.Case != caseName || step.At != "1970-01-01T00:00:00Z" {
			return fmt.Errorf("%s scenario case %d epoch anchor is not pinned", resultsetOutputLimitSimpleNoneID, caseIndex)
		}
		index++
		for pass := 0; pass < 2; pass++ {
			if step := scenario.Steps[index]; step.Op != "deploy" || step.Case != caseName || step.Statement != "s0" {
				return fmt.Errorf("%s scenario case %d pass %d deploy is not pinned", resultsetOutputLimitSimpleNoneID, caseIndex, pass)
			}
			index++
			for _, expected := range resultsetOutputLimitSimpleNoneSchedule {
				if err := validateResultSetOutputLimitSimpleNoneScheduleStep(scenario.Steps[index], caseName, expected); err != nil {
					return fmt.Errorf("%s scenario case %d pass %d: %w", resultsetOutputLimitSimpleNoneID, caseIndex, pass, err)
				}
				index++
			}
			if step := scenario.Steps[index]; step.Op != "undeploy-all" || step.Case != caseName {
				return fmt.Errorf("%s scenario case %d pass %d undeploy-all is not pinned", resultsetOutputLimitSimpleNoneID, caseIndex, pass)
			}
			index++
		}
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetOutputLimitSimpleNoneID)
	}
	return nil
}

func validateResultSetOutputLimitSimpleNoneScheduleStep(step compat.Step, caseName string, expected resultsetOutputLimitSimpleNoneExpectedStep) error {
	if step.Op != expected.op || step.Case != caseName {
		return fmt.Errorf("step %q is not pinned", expected.op)
	}
	switch expected.op {
	case "advance-time":
		if step.At != expected.at {
			return fmt.Errorf("advance-time %q is not pinned", step.At)
		}
	case "send":
		if step.EventType != expected.eventType {
			return fmt.Errorf("send event type %q is not pinned", step.EventType)
		}
		payload, err := decodeResultSetOutputLimitSimpleNoneStrictPayload(step)
		if err != nil {
			return err
		}
		switch expected.eventType {
		case "SupportBean":
			bean, ok := payload.(unidirectionalSupportBean)
			if !ok || bean.TheString != expected.theString || bean.IntPrimitive != 0 {
				return fmt.Errorf("SupportBean payload is not pinned")
			}
		case "SupportMarketDataBean":
			bean, ok := payload.(resultsetGroupedTimeWindowMarket)
			if !ok || bean.Symbol != expected.symbol || bean.Volume != expected.volume || bean.Price != expected.price {
				return fmt.Errorf("SupportMarketDataBean payload is not pinned")
			}
		}
	}
	return nil
}

// decodeResultSetOutputLimitSimpleNoneStrictPayload decodes a send payload
// with a strict field set so extra keys are rejected.
func decodeResultSetOutputLimitSimpleNoneStrictPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireResultSetOutputLimitSimpleNoneFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var value unidirectionalSupportBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportMarketDataBean":
		if err := requireResultSetOutputLimitSimpleNoneFields(fields, "symbol", "volume", "price"); err != nil {
			return nil, err
		}
		var value resultsetGroupedTimeWindowMarket
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}

func requireResultSetOutputLimitSimpleNoneFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("JSON object has unexpected fields")
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

func validateResultSetOutputLimitSimpleNoneStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index := range expected {
		if values[index] != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}
