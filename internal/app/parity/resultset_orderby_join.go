package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultsetOrderbyJoinID          = "resultset-orderby-join"
	resultsetOrderbyJoinDescription = "ResultSetOrderBySimple ordinals 1-2: join order-by with statement-iterator snapshots and output-limit over join deltas."
	resultsetOrderbyJoinJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOrderbyJoinSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderBySimple.java"
)

var (
	resultsetOrderbyJoinJavaSources = []string{
		resultsetOrderbyJoinSource,
	}
	resultsetOrderbyJoinJavaRuntimeIDs = []string{
		"java-runtime-53aea47cdd71b80fdbb9",
		"java-runtime-4a0adaff4741914b9ad1",
	}
	resultsetOrderbyJoinJavaExecutions = []string{
		"ResultSetIterator",
		"ResultSetAcrossJoin",
	}
	resultsetOrderbyJoinJavaStaticIDs = []string{
		"java-3d5138ecf75168eb575d",
		"java-61243b98a3005c75950a",
	}
	resultsetOrderbyJoinCases = []string{
		"iterator",
		"across-join-price",
		"across-join-symbol-price",
	}
	resultsetOrderbyJoinOrdinals = []int{1, 2, 2}
	resultsetOrderbyJoinEPLs     = []string{
		"@name('s0') select symbol, theString, price from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString order by price",
		"@name('s0') select symbol, theString from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by price",
		"@name('s0') select symbol from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by theString, price",
	}
)

type resultsetOrderbyJoinMarket struct {
	Symbol string  `esper:"symbol"`
	Volume int64   `esper:"volume"`
	Price  float64 `esper:"price"`
}

type resultsetOrderbyJoinString struct {
	TheString string `esper:"theString"`
}

var resultsetOrderbyJoinIteratorSeeds = []string{"CAT", "IBM", "CMU", "KGB", "DOG"}
var resultsetOrderbyJoinIteratorFirstSymbols = []string{"CAT", "IBM", "CAT", "IBM"}
var resultsetOrderbyJoinIteratorFirstVolumes = []int64{0, 0, 0, 0}
var resultsetOrderbyJoinIteratorFirstPrices = []float64{50, 49, 15, 100}
var resultsetOrderbyJoinAcrossSymbols = []string{"IBM", "KGB", "CMU", "IBM", "CAT", "CAT"}
var resultsetOrderbyJoinAcrossVolumes = []int64{0, 0, 0, 0, 0, 0}
var resultsetOrderbyJoinAcrossPrices = []float64{2, 1, 3, 6, 6, 5}

func loadResultsetOrderbyJoinScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOrderbyJoinID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOrderbyJoinID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOrderbyJoinID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOrderbyJoinID, err)
	}
	if err := requireResultsetOrderbyJoinFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOrderbyJoinID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetOrderbyJoinID ||
		metadata.Description != resultsetOrderbyJoinDescription ||
		metadata.JavaCommit != resultsetOrderbyJoinJavaCommit ||
		metadata.JavaSource != resultsetOrderbyJoinSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOrderbyJoinID)
	}
	if err := validateResultsetOrderbyJoinStringArray(root["javaRuntimes"], resultsetOrderbyJoinJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbyJoinStringArray(root["javaNames"], resultsetOrderbyJoinJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbyJoinStringArray(root["javaStaticIds"], resultsetOrderbyJoinJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbyJoinStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetOrderbyJoinCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly three cases", resultsetOrderbyJoinID)
	}
	observations := []string{"iterator", "listener", "listener"}
	iteratorSnapshots := []int{2, 0, 0}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetOrderbyJoinFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case              string `json:"case"`
			Ordinal           int    `json:"ordinal"`
			RuntimeID         string `json:"runtimeId"`
			ExecutionName     string `json:"executionName"`
			Observation       string `json:"observation"`
			IteratorSnapshots int    `json:"iteratorSnapshots"`
			EPL               string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if definition.Case != resultsetOrderbyJoinCases[index] ||
			definition.Ordinal != resultsetOrderbyJoinOrdinals[index] ||
			definition.RuntimeID != resultsetOrderbyJoinRuntimeID(resultsetOrderbyJoinCases[index]) ||
			definition.ExecutionName != resultsetOrderbyJoinExecutionName(resultsetOrderbyJoinCases[index]) ||
			definition.Observation != observations[index] ||
			definition.IteratorSnapshots != iteratorSnapshots[index] ||
			definition.EPL != resultsetOrderbyJoinEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetOrderbyJoinID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 37 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly thirty-seven steps", resultsetOrderbyJoinID)
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
			if err := requireResultsetOrderbyJoinFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireResultsetOrderbyJoinFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeResultsetOrderbyJoinPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		case "snapshot":
			if err := requireResultsetOrderbyJoinFields(object, "op", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step compat.Step
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != "s0" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshot is not pinned", index)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateResultsetOrderbyJoinScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetOrderbyJoinScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetOrderbyJoinID || len(scenario.Steps) != 37 {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetOrderbyJoinID)
	}
	index := 0
	if err := validateResultsetOrderbyJoinCaseMarker(scenario.Steps[index], resultsetOrderbyJoinCases[0]); err != nil {
		return fmt.Errorf("iterator marker: %w", err)
	}
	index++
	for _, seed := range resultsetOrderbyJoinIteratorSeeds {
		if err := validateResultsetOrderbyJoinStringStep(scenario.Steps[index], seed); err != nil {
			return fmt.Errorf("iterator seed %q: %w", seed, err)
		}
		index++
	}
	for eventIndex := range resultsetOrderbyJoinIteratorFirstSymbols {
		if err := validateResultsetOrderbyJoinIteratorMarketStep(scenario.Steps[index], eventIndex); err != nil {
			return fmt.Errorf("iterator market step %d: %w", eventIndex, err)
		}
		index++
	}
	if err := validateResultsetOrderbyJoinSnapshotStep(scenario.Steps[index]); err != nil {
		return fmt.Errorf("iterator first snapshot: %w", err)
	}
	index++
	if err := validateResultsetOrderbyJoinIteratorMarketStep(scenario.Steps[index], 4); err != nil {
		return fmt.Errorf("iterator final market step: %w", err)
	}
	index++
	if err := validateResultsetOrderbyJoinSnapshotStep(scenario.Steps[index]); err != nil {
		return fmt.Errorf("iterator second snapshot: %w", err)
	}
	index++
	for caseIndex := 1; caseIndex <= 2; caseIndex++ {
		if err := validateResultsetOrderbyJoinCaseMarker(scenario.Steps[index], resultsetOrderbyJoinCases[caseIndex]); err != nil {
			return fmt.Errorf("case %d marker: %w", caseIndex, err)
		}
		index++
		for eventIndex := range resultsetOrderbyJoinAcrossSymbols {
			if err := validateResultsetOrderbyJoinAcrossMarketStep(scenario.Steps[index], eventIndex); err != nil {
				return fmt.Errorf("case %d market step %d: %w", caseIndex, eventIndex, err)
			}
			index++
		}
		for _, seed := range resultsetOrderbyJoinIteratorSeeds {
			if err := validateResultsetOrderbyJoinStringStep(scenario.Steps[index], seed); err != nil {
				return fmt.Errorf("case %d seed %q: %w", caseIndex, seed, err)
			}
			index++
		}
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetOrderbyJoinID)
	}
	return nil
}

func validateResultsetOrderbyJoinCaseMarker(step compat.Step, expected string) error {
	if step.Op != "case" || step.Case != expected {
		return fmt.Errorf("must start case %q", expected)
	}
	return nil
}

func validateResultsetOrderbyJoinStringStep(step compat.Step, expected string) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportBeanString" {
		return fmt.Errorf("must send SupportBeanString")
	}
	value, err := decodeResultsetOrderbyJoinString(step)
	if err != nil {
		return err
	}
	if value.TheString != expected {
		return fmt.Errorf("string payload is not pinned")
	}
	return nil
}

func validateResultsetOrderbyJoinIteratorMarketStep(step compat.Step, index int) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportMarketDataBean" {
		return fmt.Errorf("must send SupportMarketDataBean")
	}
	var symbol string
	var volume int64
	var price float64
	if index < len(resultsetOrderbyJoinIteratorFirstSymbols) {
		symbol = resultsetOrderbyJoinIteratorFirstSymbols[index]
		volume = resultsetOrderbyJoinIteratorFirstVolumes[index]
		price = resultsetOrderbyJoinIteratorFirstPrices[index]
	} else {
		symbol, volume, price = "KGB", 0, 75
	}
	market, err := decodeResultsetOrderbyJoinMarket(step)
	if err != nil {
		return err
	}
	if market.Symbol != symbol || market.Volume != volume || market.Price != price {
		return fmt.Errorf("market payload is not pinned")
	}
	return nil
}

func validateResultsetOrderbyJoinAcrossMarketStep(step compat.Step, index int) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportMarketDataBean" {
		return fmt.Errorf("must send SupportMarketDataBean")
	}
	market, err := decodeResultsetOrderbyJoinMarket(step)
	if err != nil {
		return err
	}
	if market.Symbol != resultsetOrderbyJoinAcrossSymbols[index] ||
		market.Volume != resultsetOrderbyJoinAcrossVolumes[index] ||
		market.Price != resultsetOrderbyJoinAcrossPrices[index] {
		return fmt.Errorf("market payload is not pinned")
	}
	return nil
}

func validateResultsetOrderbyJoinSnapshotStep(step compat.Step) error {
	if step.Op != "snapshot" || step.Case != "" || step.Statement != "s0" {
		return fmt.Errorf("must snapshot statement s0")
	}
	return nil
}

func runResultsetOrderbyJoinScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetOrderbyJoinScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultsetOrderbyJoinCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultsetOrderbyJoinCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOrderbyJoinID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOrderbyJoinID, scenario.ID)
	}
	return trace, nil
}

func runResultsetOrderbyJoinCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOrderbyJoinMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetOrderbyJoinString](env, "SupportBeanString"); err != nil {
		return compat.Trace{}, err
	}

	market := esper.From[resultsetOrderbyJoinMarket](env, "SupportMarketDataBean").Window(esper.LengthWindow(10))
	seed := esper.From[resultsetOrderbyJoinString](env, "SupportBeanString").Window(esper.LengthWindow(100))
	symbol := esper.Field[resultsetOrderbyJoinMarket, string]("symbol")
	seedString := esper.Field[resultsetOrderbyJoinString, string]("theString")
	joinSymbol := esper.JoinField[string](0, "symbol")
	joinTheString := esper.JoinField[string](1, "theString")
	joinPrice := esper.JoinField[float64](0, "price")

	var query esper.Query
	switch caseName {
	case "iterator":
		query = esper.Join(market, seed, esper.OnEqual(symbol, seedString)).
			Select(
				esper.SelectFrom(0, "symbol", joinSymbol),
				esper.SelectFrom(1, "theString", joinTheString),
				esper.SelectFrom(0, "price", joinPrice),
			).Query(
			esper.StatementName("s0"),
			esper.OrderBy(esper.Ascending(joinPrice)),
		)
	case "across-join-price":
		query = esper.Join(market, seed, esper.OnEqual(symbol, seedString)).
			Select(
				esper.SelectFrom(0, "symbol", joinSymbol),
				esper.SelectFrom(1, "theString", joinTheString),
			).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(joinPrice)),
		)
	case "across-join-symbol-price":
		query = esper.Join(market, seed, esper.OnEqual(symbol, seedString)).
			Select(
				esper.SelectFrom(0, "symbol", joinSymbol),
			).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(joinTheString),
				esper.Ascending(joinPrice),
			),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported %s case %q", resultsetOrderbyJoinID, caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithRuntimeURI(resultsetOrderbyJoinRuntimeID(caseName)))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("%s case %q deployed %d statements", resultsetOrderbyJoinID, caseName, len(statements))
	}
	statement := statements[0]
	if caseName == "iterator" {
		return runResultsetOrderbyJoinIteratorReplay(ctx, engine, statement, scenario)
	}
	return compat.ReplayWithStatements(ctx, engine, statement, scenario,
		decodeResultsetOrderbyJoinPayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetOrderbyJoinID, name)
			}
			return statement, nil
		})
}

// runResultsetOrderbyJoinIteratorReplay mirrors the Java iterator execution:
// the statement listener stays attached but records nothing, and only the two
// pinned statement-iterator snapshots are observed.
func runResultsetOrderbyJoinIteratorReplay(ctx context.Context, engine *esper.Engine, statement *esper.Statement, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	snapshots := 0
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "send":
			payload, err := decodeResultsetOrderbyJoinPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "snapshot":
			if step.Statement != statement.Name() {
				return trace, fmt.Errorf("%s iterator snapshot metadata is not pinned", resultsetOrderbyJoinID)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return trace, err
			}
			rows := compat.NormalizeResults(result.Batch.New)
			if rows == nil {
				rows = []compat.ResultRecord{}
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      "iterator",
				Operation: "snapshot",
				Statement: statement.Name(),
				Sequence:  0,
				Time:      engine.Now().UTC().Format(time.RFC3339Nano),
				New:       rows,
			})
			snapshots++
		default:
			return trace, fmt.Errorf("unsupported %s iterator step op %q", resultsetOrderbyJoinID, step.Op)
		}
	}
	if snapshots != 2 {
		return trace, fmt.Errorf("%s iterator case produced %d snapshots, want two", resultsetOrderbyJoinID, snapshots)
	}
	return trace, nil
}

func decodeResultsetOrderbyJoinPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportMarketDataBean":
		return decodeResultsetOrderbyJoinMarket(step)
	case "SupportBeanString":
		return decodeResultsetOrderbyJoinString(step)
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOrderbyJoinID, step.EventType)
	}
}

func decodeResultsetOrderbyJoinMarket(step compat.Step) (resultsetOrderbyJoinMarket, error) {
	if step.EventType != "SupportMarketDataBean" {
		return resultsetOrderbyJoinMarket{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOrderbyJoinMarket{}, err
	}
	if err := requireResultsetOrderbyJoinFields(fields, "symbol", "volume", "price"); err != nil {
		return resultsetOrderbyJoinMarket{}, err
	}
	symbol, err := decodeResultsetOrderbyJoinStringValue(fields["symbol"], "symbol")
	if err != nil {
		return resultsetOrderbyJoinMarket{}, err
	}
	volume, err := decodeResultsetOrderbyJoinInteger(fields["volume"], "volume")
	if err != nil {
		return resultsetOrderbyJoinMarket{}, err
	}
	price, err := decodeResultsetOrderbyJoinFloat(fields["price"], "price")
	if err != nil {
		return resultsetOrderbyJoinMarket{}, err
	}
	return resultsetOrderbyJoinMarket{Symbol: symbol, Volume: volume, Price: price}, nil
}

func decodeResultsetOrderbyJoinString(step compat.Step) (resultsetOrderbyJoinString, error) {
	if step.EventType != "SupportBeanString" {
		return resultsetOrderbyJoinString{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOrderbyJoinString{}, err
	}
	if err := requireResultsetOrderbyJoinFields(fields, "theString"); err != nil {
		return resultsetOrderbyJoinString{}, err
	}
	value, err := decodeResultsetOrderbyJoinStringValue(fields["theString"], "theString")
	if err != nil {
		return resultsetOrderbyJoinString{}, err
	}
	return resultsetOrderbyJoinString{TheString: value}, nil
}

func requireResultsetOrderbyJoinFields(object map[string]json.RawMessage, names ...string) error {
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

func decodeResultsetOrderbyJoinStringValue(raw json.RawMessage, name string) (string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must be a string", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func decodeResultsetOrderbyJoinInteger(raw json.RawMessage, name string) (int64, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	number, ok := token.(json.Number)
	if !ok || !resultsetOrderbyJoinIntegerSyntax(string(number)) {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	value, err := strconv.ParseInt(string(number), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	return value, nil
}

func decodeResultsetOrderbyJoinFloat(raw json.RawMessage, name string) (float64, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s must be a JSON number", name)
	}
	number, ok := token.(json.Number)
	if !ok {
		return 0, fmt.Errorf("%s must be a JSON number", name)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return 0, fmt.Errorf("%s must be a JSON number", name)
	}
	value, err := strconv.ParseFloat(string(number), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("%s must be a JSON number", name)
	}
	return value, nil
}

func resultsetOrderbyJoinIntegerSyntax(text string) bool {
	if text == "0" {
		return true
	}
	if text == "" {
		return false
	}
	if text[0] == '-' {
		text = text[1:]
	}
	if text == "" || (len(text) > 1 && text[0] == '0') {
		return false
	}
	for _, character := range text {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validateResultsetOrderbyJoinStringArray(raw json.RawMessage, expected []string, name string) error {
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

func resultsetOrderbyJoinRuntimeID(caseName string) string {
	switch caseName {
	case "iterator":
		return resultsetOrderbyJoinJavaRuntimeIDs[0]
	case "across-join-price", "across-join-symbol-price":
		return resultsetOrderbyJoinJavaRuntimeIDs[1]
	}
	return "resultset-orderby-join-unknown"
}

func resultsetOrderbyJoinExecutionName(caseName string) string {
	switch caseName {
	case "iterator":
		return resultsetOrderbyJoinJavaExecutions[0]
	case "across-join-price", "across-join-symbol-price":
		return resultsetOrderbyJoinJavaExecutions[1]
	}
	return "resultset-orderby-join-unknown"
}
