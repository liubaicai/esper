package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultsetOrderbySimpleDescendingOMID          = "resultset-orderby-simple-descending-om"
	resultsetOrderbySimpleDescendingOMDescription = "ResultSetOrderBySimple ordinals 3-4: descending order-by over a length(5) window with output every 6 events; ordinal 3's SODA object-model assertions (toEPL text, serialization) are pinned as unrepresentable records while its runtime flow replays identically to ordinal 4 variant 1, and ordinal 4 variants 2-6 cover price desc+symbol asc, price asc, symbol desc, symbol desc+price desc, and symbol+price."
	resultsetOrderbySimpleDescendingOMJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOrderbySimpleDescendingOMSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderBySimple.java"
)

var (
	resultsetOrderbySimpleDescendingOMJavaSources = []string{
		resultsetOrderbySimpleDescendingOMSource,
	}
	resultsetOrderbySimpleDescendingOMJavaRuntimeIDs = []string{
		"java-runtime-df8ea61ff025609ce309",
		"java-runtime-9668909b2b2f00769dab",
	}
	resultsetOrderbySimpleDescendingOMJavaExecutions = []string{
		"ResultSetDescendingOM",
		"ResultSetDescending",
	}
	resultsetOrderbySimpleDescendingOMJavaStaticIDs = []string{
		"java-042c1b302e7183feb8a6",
		"java-042c1b302e7183feb8a6",
	}
	resultsetOrderbySimpleDescendingOMCases = []string{
		"descending-om",
		"descending-v2",
		"descending-v3",
		"descending-v4",
		"descending-v5",
		"descending-v6",
	}
	resultsetOrderbySimpleDescendingOMOrdinals = []int{3, 4, 4, 4, 4, 4}
	resultsetOrderbySimpleDescendingOMEPLs     = []string{
		"select symbol from SupportMarketDataBean#length(5) output every 6 events order by price desc",
		"@name('s0') select symbol from SupportMarketDataBean#length(5) output every 6 events order by price desc, symbol asc",
		"@name('s0') select symbol from SupportMarketDataBean#length(5) output every 6 events order by price asc",
		"@name('s0') select symbol, volume from SupportMarketDataBean#length(5) output every 6 events order by symbol desc",
		"@name('s0') select symbol, price from SupportMarketDataBean#length(5) output every 6 events order by symbol desc, price desc",
		"@name('s0') select symbol, price from SupportMarketDataBean#length(5) output every 6 events order by symbol, price",
	}
	resultsetOrderbySimpleDescendingOMObservations = []string{
		"unrepresentable+listener; the SODA object-model assertions (serialization round-trip, toEPL text) have no typed Go boundary so they pin unrepresentable records; the deployed statement is annotated @name('s0') and its runtime flow is identical to ordinal 4 variant 1, delivering one six-row batch ordered by price desc",
		"listener; ordinal 4 variant 2 orders the six-row output batch by price desc, symbol asc so the price-6 tie resolves CAT before IBM",
		"listener; ordinal 4 variant 3 orders the six-row output batch by price asc",
		"listener; ordinal 4 variant 4 selects symbol, volume and orders the six-row output batch by symbol desc",
		"listener; ordinal 4 variant 5 selects symbol, price and orders the six-row output batch by symbol desc, price desc",
		"listener; ordinal 4 variant 6 selects symbol, price and orders the six-row output batch by symbol, price",
	}
)

// resultsetOrderbySimpleDescendingOMSerializationNote pins the ord-3
// SerializableObjectCopier.copyMayFail round-trip assertion: the Java
// object model survives Java serialization, a Java-API surface with no Go
// boundary.
const resultsetOrderbySimpleDescendingOMSerializationNote = "SerializableObjectCopier.copyMayFail round-trips the EPStatementObjectModel through Java serialization; no Go object-model serialization boundary exists"

// resultsetOrderbySimpleDescendingOMToEPLNote pins the ord-3
// model.toEPL() text assertion: the SODA object model renders the exact
// stmtText the case pins, while EPL text is not the typed Go entry point.
const resultsetOrderbySimpleDescendingOMToEPLNote = "EPStatementObjectModel.toEPL() renders 'select symbol from SupportMarketDataBean#length(5) output every 6 events order by price desc'; EPL text is not the typed Go entry point"

// resultsetOrderbySimpleDescendingOMUnrepresentable pins the ord-3
// object-model assertions in Java execution order: the serialization
// round-trip precedes the toEPL text assertion.
var resultsetOrderbySimpleDescendingOMUnrepresentable = []struct {
	statement string
	note      string
}{
	{"object-model-serialization", resultsetOrderbySimpleDescendingOMSerializationNote},
	{"object-model-to-epl", resultsetOrderbySimpleDescendingOMToEPLNote},
}

type resultsetOrderbySimpleDescendingOMBean struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
}

// resultsetOrderbySimpleDescendingOMEvents pins the shared six-event send
// sequence every case replays (Java sendEvent(symbol, price) with
// volume=0L): IBM@2, KGB@1, CMU@3, IBM@6, CAT@6, CAT@5.
var resultsetOrderbySimpleDescendingOMEvents = []resultsetOrderbySimpleDescendingOMBean{
	{Symbol: "IBM", Price: 2, Volume: 0},
	{Symbol: "KGB", Price: 1, Volume: 0},
	{Symbol: "CMU", Price: 3, Volume: 0},
	{Symbol: "IBM", Price: 6, Volume: 0},
	{Symbol: "CAT", Price: 6, Volume: 0},
	{Symbol: "CAT", Price: 5, Volume: 0},
}

func loadResultsetOrderbySimpleDescendingOMScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOrderbySimpleDescendingOMID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOrderbySimpleDescendingOMID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOrderbySimpleDescendingOMID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOrderbySimpleDescendingOMID, err)
	}
	if err := requireResultsetOrderbySimpleDescendingOMFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOrderbySimpleDescendingOMID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetOrderbySimpleDescendingOMID ||
		metadata.Description != resultsetOrderbySimpleDescendingOMDescription ||
		metadata.JavaCommit != resultsetOrderbySimpleDescendingOMJavaCommit ||
		metadata.JavaSource != resultsetOrderbySimpleDescendingOMSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOrderbySimpleDescendingOMID)
	}
	if err := validateResultsetOrderbySimpleDescendingOMStringArray(root["javaRuntimes"], resultsetOrderbySimpleDescendingOMJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbySimpleDescendingOMStringArray(root["javaNames"], resultsetOrderbySimpleDescendingOMJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbySimpleDescendingOMStringArray(root["javaStaticIds"], resultsetOrderbySimpleDescendingOMJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbySimpleDescendingOMStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetOrderbySimpleDescendingOMCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly six cases", resultsetOrderbySimpleDescendingOMID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetOrderbySimpleDescendingOMFields(object,
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
		if definition.Case != resultsetOrderbySimpleDescendingOMCases[index] ||
			definition.Ordinal != resultsetOrderbySimpleDescendingOMOrdinals[index] ||
			definition.RuntimeID != resultsetOrderbySimpleDescendingOMRuntimeID(definition.Case) ||
			definition.ExecutionName != resultsetOrderbySimpleDescendingOMExecutionName(definition.Case) ||
			definition.Observation != resultsetOrderbySimpleDescendingOMObservations[index] ||
			definition.IteratorSnapshots != 0 ||
			definition.EPL != resultsetOrderbySimpleDescendingOMEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetOrderbySimpleDescendingOMID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 44 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly forty-four steps", resultsetOrderbySimpleDescendingOMID)
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
			if err := requireResultsetOrderbySimpleDescendingOMFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireResultsetOrderbySimpleDescendingOMFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeResultsetOrderbySimpleDescendingOMPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		case "unrepresentable":
			if err := requireResultsetOrderbySimpleDescendingOMFields(object, "op", "case", "statement", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateResultsetOrderbySimpleDescendingOMScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetOrderbySimpleDescendingOMScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetOrderbySimpleDescendingOMID || len(scenario.Steps) != 44 {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetOrderbySimpleDescendingOMID)
	}
	index := 0
	for caseIndex, caseName := range resultsetOrderbySimpleDescendingOMCases {
		if err := validateResultsetOrderbySimpleDescendingOMCaseMarker(scenario.Steps[index], caseName); err != nil {
			return fmt.Errorf("case %d marker: %w", caseIndex, err)
		}
		index++
		if caseIndex == 0 {
			for _, pinned := range resultsetOrderbySimpleDescendingOMUnrepresentable {
				if err := validateResultsetOrderbySimpleDescendingOMUnrepresentableStep(scenario.Steps[index], caseName, pinned.statement, pinned.note); err != nil {
					return fmt.Errorf("case %d unrepresentable step: %w", caseIndex, err)
				}
				index++
			}
		}
		for _, expected := range resultsetOrderbySimpleDescendingOMEvents {
			if err := validateResultsetOrderbySimpleDescendingOMBeanStep(scenario.Steps[index], expected); err != nil {
				return fmt.Errorf("case %d bean step: %w", caseIndex, err)
			}
			index++
		}
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetOrderbySimpleDescendingOMID)
	}
	return nil
}

func validateResultsetOrderbySimpleDescendingOMCaseMarker(step compat.Step, expected string) error {
	if step.Op != "case" || step.Case != expected {
		return fmt.Errorf("must start case %q", expected)
	}
	return nil
}

func validateResultsetOrderbySimpleDescendingOMUnrepresentableStep(step compat.Step, caseName, statement, note string) error {
	if step.Op != "unrepresentable" || step.Case != caseName || step.Statement != statement || step.ExpectError != note {
		return fmt.Errorf("must pin unrepresentable %q", statement)
	}
	return nil
}

func validateResultsetOrderbySimpleDescendingOMBeanStep(step compat.Step, expected resultsetOrderbySimpleDescendingOMBean) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportMarketDataBean" {
		return fmt.Errorf("must send SupportMarketDataBean")
	}
	bean, err := decodeResultsetOrderbySimpleDescendingOMBean(step)
	if err != nil {
		return err
	}
	if bean.Symbol != expected.Symbol || bean.Price != expected.Price || bean.Volume != expected.Volume {
		return fmt.Errorf("bean payload is not pinned")
	}
	return nil
}

func runResultsetOrderbySimpleDescendingOMScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetOrderbySimpleDescendingOMScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultsetOrderbySimpleDescendingOMCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultsetOrderbySimpleDescendingOMCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOrderbySimpleDescendingOMID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOrderbySimpleDescendingOMID, scenario.ID)
	}
	return trace, nil
}

func runResultsetOrderbySimpleDescendingOMCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOrderbySimpleDescendingOMBean](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}

	symbol := esper.Field[resultsetOrderbySimpleDescendingOMBean, string]("symbol")
	price := esper.Field[resultsetOrderbySimpleDescendingOMBean, float64]("price")
	volume := esper.Field[resultsetOrderbySimpleDescendingOMBean, int64]("volume")
	windowed := esper.From[resultsetOrderbySimpleDescendingOMBean](env, "SupportMarketDataBean").Window(esper.LengthWindow(5))

	var query esper.Query
	switch caseName {
	case "descending-om":
		query = esper.Select(windowed,
			esper.Alias("symbol", symbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Descending(price)),
		)
	case "descending-v2":
		query = esper.Select(windowed,
			esper.Alias("symbol", symbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Descending(price),
				esper.Ascending(symbol),
			),
		)
	case "descending-v3":
		query = esper.Select(windowed,
			esper.Alias("symbol", symbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(price)),
		)
	case "descending-v4":
		query = esper.Select(windowed,
			esper.Alias("symbol", symbol),
			esper.Alias("volume", volume),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Descending(symbol)),
		)
	case "descending-v5":
		query = esper.Select(windowed,
			esper.Alias("symbol", symbol),
			esper.Alias("price", price),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Descending(symbol),
				esper.Descending(price),
			),
		)
	case "descending-v6":
		query = esper.Select(windowed,
			esper.Alias("symbol", symbol),
			esper.Alias("price", price),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(symbol),
				esper.Ascending(price),
			),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported %s case %q", resultsetOrderbySimpleDescendingOMID, caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithRuntimeURI(resultsetOrderbySimpleDescendingOMRuntimeID(caseName)))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("%s case %q deployed %d statements", resultsetOrderbySimpleDescendingOMID, caseName, len(statements))
	}
	statement := statements[0]
	if caseName == "descending-om" {
		return runResultsetOrderbySimpleDescendingOMReplay(ctx, engine, statement, scenario, caseName)
	}
	return compat.ReplayWithStatements(ctx, engine, statement, scenario,
		decodeResultsetOrderbySimpleDescendingOMPayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetOrderbySimpleDescendingOMID, name)
			}
			return statement, nil
		})
}

// runResultsetOrderbySimpleDescendingOMReplay mirrors the ord-3 Java
// execution: the SODA object-model assertions have no typed Go boundary, so
// the pinned unrepresentable records are emitted verbatim before the shared
// replay sends the six events and records the listener batch.
func runResultsetOrderbySimpleDescendingOMReplay(ctx context.Context, engine *esper.Engine, statement *esper.Statement, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, step := range scenario.Steps {
		if step.Op != "unrepresentable" {
			continue
		}
		note, ok := resultsetOrderbySimpleDescendingOMUnrepresentableNote(step.Statement)
		if !ok || step.Case != caseName || step.ExpectError != note {
			return compat.Trace{}, fmt.Errorf("%s unrepresentable step %q is not pinned", resultsetOrderbySimpleDescendingOMID, step.Statement)
		}
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "unrepresentable",
			Statement: step.Statement,
			Value:     step.ExpectError,
		})
	}
	replayed, err := compat.ReplayWithStatements(ctx, engine, statement, scenario,
		decodeResultsetOrderbySimpleDescendingOMPayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetOrderbySimpleDescendingOMID, name)
			}
			return statement, nil
		})
	if err != nil {
		return compat.Trace{}, err
	}
	trace.Records = append(trace.Records, replayed.Records...)
	return trace, nil
}

func resultsetOrderbySimpleDescendingOMUnrepresentableNote(statement string) (string, bool) {
	for _, pinned := range resultsetOrderbySimpleDescendingOMUnrepresentable {
		if pinned.statement == statement {
			return pinned.note, true
		}
	}
	return "", false
}

func decodeResultsetOrderbySimpleDescendingOMPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportMarketDataBean":
		return decodeResultsetOrderbySimpleDescendingOMBean(step)
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOrderbySimpleDescendingOMID, step.EventType)
	}
}

func decodeResultsetOrderbySimpleDescendingOMBean(step compat.Step) (resultsetOrderbySimpleDescendingOMBean, error) {
	if step.EventType != "SupportMarketDataBean" {
		return resultsetOrderbySimpleDescendingOMBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOrderbySimpleDescendingOMBean{}, err
	}
	if err := requireResultsetOrderbySimpleDescendingOMFields(fields, "symbol", "volume", "price"); err != nil {
		return resultsetOrderbySimpleDescendingOMBean{}, err
	}
	symbol, err := decodeResultsetOrderbySimpleDescendingOMString(fields["symbol"], "symbol")
	if err != nil {
		return resultsetOrderbySimpleDescendingOMBean{}, err
	}
	volume, err := decodeResultsetOrderbySimpleDescendingOMInteger(fields["volume"], "volume")
	if err != nil {
		return resultsetOrderbySimpleDescendingOMBean{}, err
	}
	price, err := decodeResultsetOrderbySimpleDescendingOMFloat(fields["price"], "price")
	if err != nil {
		return resultsetOrderbySimpleDescendingOMBean{}, err
	}
	return resultsetOrderbySimpleDescendingOMBean{Symbol: symbol, Price: price, Volume: volume}, nil
}

func requireResultsetOrderbySimpleDescendingOMFields(object map[string]json.RawMessage, names ...string) error {
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

func decodeResultsetOrderbySimpleDescendingOMString(raw json.RawMessage, name string) (string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must be a string", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func decodeResultsetOrderbySimpleDescendingOMInteger(raw json.RawMessage, name string) (int64, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	number, ok := token.(json.Number)
	if !ok || !resultsetOrderbySimpleDescendingOMIntegerSyntax(string(number)) {
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

func decodeResultsetOrderbySimpleDescendingOMFloat(raw json.RawMessage, name string) (float64, error) {
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

func resultsetOrderbySimpleDescendingOMIntegerSyntax(text string) bool {
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

func validateResultsetOrderbySimpleDescendingOMStringArray(raw json.RawMessage, expected []string, name string) error {
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

func resultsetOrderbySimpleDescendingOMRuntimeID(caseName string) string {
	switch caseName {
	case "descending-om":
		return resultsetOrderbySimpleDescendingOMJavaRuntimeIDs[0]
	case "descending-v2", "descending-v3", "descending-v4", "descending-v5", "descending-v6":
		return resultsetOrderbySimpleDescendingOMJavaRuntimeIDs[1]
	}
	return "resultset-orderby-simple-descending-om-unknown"
}

func resultsetOrderbySimpleDescendingOMExecutionName(caseName string) string {
	switch caseName {
	case "descending-om":
		return resultsetOrderbySimpleDescendingOMJavaExecutions[0]
	case "descending-v2", "descending-v3", "descending-v4", "descending-v5", "descending-v6":
		return resultsetOrderbySimpleDescendingOMJavaExecutions[1]
	}
	return "resultset-orderby-simple-descending-om-unknown"
}
