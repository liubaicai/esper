package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// resultsetAggregateMedianAndDeviationMarketData is the typed
// SupportMarketDataBean shape used by all three median/deviation executions.
// Volume is retained even though the aggregates in this execution use price;
// it is part of the SupportMarketDataBean protocol payload and constructor.
type resultsetAggregateMedianAndDeviationMarketData struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
}

type resultsetAggregateMedianAndDeviationStringBean struct {
	TheString string `esper:"theString"`
}

const resultsetAggregateMedianAndDeviationJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateMedianAndDeviationJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateMedianAndDeviation.java",
}

var resultsetAggregateMedianAndDeviationJavaRuntimeIDs = []string{
	"java-runtime-101db664ea346721e986",
	"java-runtime-2f533a8a1bc0d93ae649",
	"java-runtime-2667a7eadeb7a458d7ce",
}

var resultsetAggregateMedianAndDeviationJavaExecutions = []string{
	"ResultSetAggregateStmt",
	"ResultSetAggregateStmtJoinOM",
	"ResultSetAggregateStmtJoin",
}

// The static candidates are source facts used by the Java oracle's build
// assertion. They are kept here for parity metadata consumers even though the
// typed Go runner intentionally builds an equivalent Query directly.
var resultsetAggregateMedianAndDeviationStaticCandidates = []string{
	"java-b11b233b0ea7da05217f",
	"java-66a516ce6e14456627b2",
	"java-a5c65b307ecdb2d0355b",
}

const (
	resultsetAggregateMedianAndDeviationStmtCase   = "stmt-main"
	resultsetAggregateMedianAndDeviationJoinOMCase = "join-om"
	resultsetAggregateMedianAndDeviationJoinCase   = "join-epl"
)

var resultsetAggregateMedianAndDeviationCases = []string{
	resultsetAggregateMedianAndDeviationStmtCase,
	resultsetAggregateMedianAndDeviationJoinOMCase,
	resultsetAggregateMedianAndDeviationJoinCase,
}

var resultsetAggregateMedianAndDeviationPrices = []float64{10, 20, 20, 90, 5, 90, 30}

const resultsetAggregateMedianAndDeviationDescription = "ResultSetAggregateMedianAndDeviation ordinals 0-2: grouped and join median, distinct median, sample stddev, and avedev over the finite DELL price sequence 10,20,20,90,5,90,30. The source NaN sensitivity phase is covered by a focused Go semantic test and is not represented in JSON."

var resultsetAggregateMedianAndDeviationJavaCaseEPL = []string{
	`@name('s0') select irstream symbol,median(all price) as myMedian,median(distinct price) as myDistMedian,stddev(all price) as myStdev,avedev(all price) as myAvedev from SupportMarketDataBean#length(5) where symbol='DELL' or symbol='IBM' or symbol='GE' group by symbol`,
	`select irstream symbol, median(price) as myMedian, median(distinct price) as myDistMedian, stddev(price) as myStdev, avedev(price) as myAvedev from SupportBeanString#length(100) as one, SupportMarketDataBean#length(5) as two where (symbol="DELL" or symbol="IBM" or symbol="GE") and one.theString=two.symbol group by symbol`,
	`@name('s0') select irstream symbol,median(price) as myMedian,median(distinct price) as myDistMedian,stddev(price) as myStdev,avedev(price) as myAvedev from SupportBeanString#length(100) as one, SupportMarketDataBean#length(5) as two where (symbol='DELL' or symbol='IBM' or symbol='GE')        and one.theString = two.symbol group by symbol`,
}

func loadResultSetAggregateMedianAndDeviationScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-median-and-deviation scenario reader is required")
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read resultset-aggregate-median-and-deviation scenario: %w", err)
	}
	if err := rejectResultSetAggregateMedianAndDeviationDuplicateKeys(data); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-aggregate-median-and-deviation scenario: %w", err)
	}
	root, err := decodeResultSetAggregateMedianAndDeviationJSONObject(data)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-aggregate-median-and-deviation scenario: %w", err)
	}
	if err := requireResultSetAggregateMedianAndDeviationJSONFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	version, err := decodeResultSetAggregateMedianAndDeviationJSONString(root["version"], "version")
	if err != nil {
		return compat.Scenario{}, err
	}
	if version != compat.ScenarioVersion {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-median-and-deviation scenario has unsupported version %q", version)
	}
	id, err := decodeResultSetAggregateMedianAndDeviationJSONString(root["id"], "id")
	if err != nil {
		return compat.Scenario{}, err
	}
	if id != "resultset-aggregate-median-and-deviation" {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-median-and-deviation scenario has unsupported id %q", id)
	}
	description, err := decodeResultSetAggregateMedianAndDeviationJSONString(root["description"], "description")
	if err != nil {
		return compat.Scenario{}, err
	}
	if description != resultsetAggregateMedianAndDeviationDescription {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-median-and-deviation description is not pinned")
	}
	javaCommit, err := decodeResultSetAggregateMedianAndDeviationJSONString(root["javaCommit"], "javaCommit")
	if err != nil {
		return compat.Scenario{}, err
	}
	if javaCommit != resultsetAggregateMedianAndDeviationJavaCommit {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-median-and-deviation javaCommit is not pinned")
	}
	javaSource, err := decodeResultSetAggregateMedianAndDeviationJSONString(root["javaSource"], "javaSource")
	if err != nil {
		return compat.Scenario{}, err
	}
	if len(resultsetAggregateMedianAndDeviationJavaSources) != 1 || javaSource != resultsetAggregateMedianAndDeviationJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-median-and-deviation javaSource is not pinned")
	}
	if err := validateResultSetAggregateMedianAndDeviationStringArray(root["javaRuntimes"], resultsetAggregateMedianAndDeviationJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateMedianAndDeviationStringArray(root["javaNames"], resultsetAggregateMedianAndDeviationJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateMedianAndDeviationStringArray(root["javaStaticIds"], resultsetAggregateMedianAndDeviationStaticCandidates, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	caseValues, err := decodeResultSetAggregateMedianAndDeviationJSONArray(root["cases"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario cases: %w", err)
	}
	if len(caseValues) != len(resultsetAggregateMedianAndDeviationCases) {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-median-and-deviation scenario must contain exactly three cases")
	}
	for index, rawCase := range caseValues {
		definition, err := decodeResultSetAggregateMedianAndDeviationJSONObject(rawCase)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultSetAggregateMedianAndDeviationJSONFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		caseName, err := decodeResultSetAggregateMedianAndDeviationJSONString(definition["case"], fmt.Sprintf("scenario case %d case", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		ordinal, err := decodeResultSetAggregateMedianAndDeviationJSONInteger(definition["ordinal"], fmt.Sprintf("scenario case %d ordinal", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		runtimeID, err := decodeResultSetAggregateMedianAndDeviationJSONString(definition["runtimeId"], fmt.Sprintf("scenario case %d runtimeId", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		executionName, err := decodeResultSetAggregateMedianAndDeviationJSONString(definition["executionName"], fmt.Sprintf("scenario case %d executionName", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		observation, err := decodeResultSetAggregateMedianAndDeviationJSONString(definition["observation"], fmt.Sprintf("scenario case %d observation", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		epl, err := decodeResultSetAggregateMedianAndDeviationJSONString(definition["epl"], fmt.Sprintf("scenario case %d epl", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		if caseName != resultsetAggregateMedianAndDeviationCases[index] || ordinal != index || runtimeID != resultsetAggregateMedianAndDeviationJavaRuntimeIDs[index] || executionName != resultsetAggregateMedianAndDeviationJavaExecutions[index] || observation != "listener" || epl != resultsetAggregateMedianAndDeviationJavaCaseEPL[index] {
			return compat.Scenario{}, fmt.Errorf("scenario case metadata mismatch at index %d", index)
		}
	}
	stepValues, err := decodeResultSetAggregateMedianAndDeviationJSONArray(root["steps"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario steps: %w", err)
	}
	if len(stepValues) != 30 {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-median-and-deviation scenario must contain exactly 30 steps")
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: make([]compat.Step, 0, len(stepValues))}
	for index, rawStep := range stepValues {
		stepObject, err := decodeResultSetAggregateMedianAndDeviationJSONObject(rawStep)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if index == 0 || index == 8 || index == 19 {
			if err := requireResultSetAggregateMedianAndDeviationJSONFields(stepObject, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		} else if err := requireResultSetAggregateMedianAndDeviationJSONFields(stepObject, "op", "eventType", "payload"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		encoded, err := json.Marshal(stepObject)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var step compat.Step
		if err := json.Unmarshal(encoded, &step); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		scenario.Steps = append(scenario.Steps, step)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func rejectResultSetAggregateMedianAndDeviationDuplicateKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkResultSetAggregateMedianAndDeviationJSONValue(decoder); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("JSON value contains trailing data")
		}
		return err
	}
	return nil
}

func walkResultSetAggregateMedianAndDeviationJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("JSON object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("JSON object contains duplicate field %q", key)
			}
			seen[key] = struct{}{}
			if err := walkResultSetAggregateMedianAndDeviationJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim('}') {
			return fmt.Errorf("JSON object is not closed")
		}
	case '[':
		for decoder.More() {
			if err := walkResultSetAggregateMedianAndDeviationJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim(']') {
			return fmt.Errorf("JSON array is not closed")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	return nil
}

func decodeResultSetAggregateMedianAndDeviationJSONObject(raw []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('{') {
		return nil, fmt.Errorf("JSON value must be an object")
	}
	object := make(map[string]json.RawMessage)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, fmt.Errorf("JSON object key is not a string")
		}
		if _, exists := object[key]; exists {
			return nil, fmt.Errorf("JSON object contains duplicate field %q", key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		object[key] = value
	}
	closing, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if closing != json.Delim('}') {
		return nil, fmt.Errorf("JSON object is not closed")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("JSON value contains trailing data")
		}
		return nil, err
	}
	return object, nil
}

func decodeResultSetAggregateMedianAndDeviationJSONArray(raw json.RawMessage) ([]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('[') {
		return nil, fmt.Errorf("JSON value must be an array")
	}
	values := make([]json.RawMessage, 0)
	for decoder.More() {
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	closing, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if closing != json.Delim(']') {
		return nil, fmt.Errorf("JSON array is not closed")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("JSON value contains trailing data")
		}
		return nil, err
	}
	return values, nil
}

func requireResultSetAggregateMedianAndDeviationJSONFields(object map[string]json.RawMessage, names ...string) error {
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

func decodeResultSetAggregateMedianAndDeviationJSONString(raw json.RawMessage, name string) (string, error) {
	if len(raw) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return "", fmt.Errorf("%s must be a string", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func decodeResultSetAggregateMedianAndDeviationJSONInteger(raw json.RawMessage, name string) (int, error) {
	if len(raw) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	var number float64
	if err := json.Unmarshal(raw, &number); err != nil || math.IsNaN(number) || math.IsInf(number, 0) || number != math.Trunc(number) || number < math.MinInt32 || number > math.MaxInt32 {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	return int(number), nil
}

func validateResultSetAggregateMedianAndDeviationStringArray(raw json.RawMessage, expected []string, name string) error {
	values, err := decodeResultSetAggregateMedianAndDeviationJSONArray(raw)
	if err != nil {
		return fmt.Errorf("%s must be an array: %w", name, err)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s must contain exactly %d values", name, len(expected))
	}
	for index, rawValue := range values {
		value, err := decodeResultSetAggregateMedianAndDeviationJSONString(rawValue, fmt.Sprintf("%s[%d]", name, index))
		if err != nil {
			return err
		}
		if value != expected[index] {
			return fmt.Errorf("%s mismatch at index %d", name, index)
		}
	}
	return nil
}

// runResultSetAggregateMedianAndDeviationScenario replays the three finite
// executions in source order. The Java execution's NaN redeployment phase is
// deliberately outside this finite scenario; it is covered by a focused Go
// semantic test instead of being serialized into the differential protocol.

func runResultSetAggregateMedianAndDeviationScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateMedianAndDeviationScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for index, caseName := range resultsetAggregateMedianAndDeviationCases {
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultSetAggregateMedianAndDeviationCase(ctx, caseScenario, caseName, resultsetAggregateMedianAndDeviationJavaRuntimeIDs[index])
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-median-and-deviation case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateMedianAndDeviationCase(ctx context.Context, scenario compat.Scenario, caseName, runtimeURI string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateMedianAndDeviationMarketData](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}

	symbol := esper.Field[resultsetAggregateMedianAndDeviationMarketData, string]("symbol")
	price := esper.Field[resultsetAggregateMedianAndDeviationMarketData, float64]("price")
	filtered := esper.From[resultsetAggregateMedianAndDeviationMarketData](env, "SupportMarketDataBean").
		Filter(esper.Or(
			esper.Or(
				esper.Equal[string](symbol, esper.Literal("DELL")),
				esper.Equal[string](symbol, esper.Literal("IBM")),
			),
			esper.Equal[string](symbol, esper.Literal("GE")),
		))

	var query esper.Query
	switch caseName {
	case resultsetAggregateMedianAndDeviationStmtCase:
		grouped := filtered.Window(esper.LengthWindow(5)).GroupBy(symbol)
		query = grouped.Select(
			esper.Alias("symbol", symbol),
			esper.Alias("myMedian", esper.Median[float64](price)),
			esper.Alias("myDistMedian", esper.DistinctAggregate[float64](esper.Median[float64](price), price)),
			esper.Alias("myStdev", esper.StdDev[float64](price)),
			esper.Alias("myAvedev", esper.Avedev[float64](price)),
		).Query(esper.StatementName("s0"), esper.WithOldStream())
	case resultsetAggregateMedianAndDeviationJoinOMCase, resultsetAggregateMedianAndDeviationJoinCase:
		if _, err := esper.RegisterStruct[resultsetAggregateMedianAndDeviationStringBean](env, "SupportBeanString"); err != nil {
			return compat.Trace{}, err
		}
		theString := esper.Field[resultsetAggregateMedianAndDeviationStringBean, string]("theString")
		left := esper.From[resultsetAggregateMedianAndDeviationStringBean](env, "SupportBeanString").Window(esper.LengthWindow(100))
		right := filtered.Window(esper.LengthWindow(5))
		joined := esper.Join(left, right, esper.OnEqual(theString, symbol)).GroupBy(esper.JoinField[string](1, "symbol"))
		joinedPrice := esper.JoinField[float64](1, "price")
		query = joined.Select(
			esper.Alias("symbol", esper.JoinField[string](1, "symbol")),
			esper.Alias("myMedian", esper.Median[float64](joinedPrice)),
			esper.Alias("myDistMedian", esper.DistinctAggregate[float64](esper.Median[float64](joinedPrice), joinedPrice)),
			esper.Alias("myStdev", esper.StdDev[float64](joinedPrice)),
			esper.Alias("myAvedev", esper.Avedev[float64](joinedPrice)),
		).Query(esper.StatementName("s0"), esper.WithOldStream())
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset-aggregate-median-and-deviation case %q", caseName)
	}

	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(runtimeURI),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one resultset aggregate statement, got %d", len(statements))
	}
	statement := statements[0]
	return compat.ReplayWithStatements(ctx, engine, statement, scenario, decodeResultSetAggregateMedianAndDeviationPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-median-and-deviation statement %q", name)
		}
		return statement, nil
	})
}

func validateResultSetAggregateMedianAndDeviationScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	const scenarioID = "resultset-aggregate-median-and-deviation"
	if scenario.ID != scenarioID {
		return fmt.Errorf("%s scenario has unsupported id %q", scenarioID, scenario.ID)
	}
	// stmt-main has one marker and seven market sends. Each join case has one
	// marker, three seed events, and the same seven market sends.
	expectedSteps := 1 + len(resultsetAggregateMedianAndDeviationPrices)
	expectedSteps += 2 * (1 + 3 + len(resultsetAggregateMedianAndDeviationPrices))
	if len(scenario.Steps) != expectedSteps {
		return fmt.Errorf("%s scenario must contain %d steps", scenarioID, expectedSteps)
	}

	stepIndex := 0
	if err := validateResultSetAggregateMedianAndDeviationCaseMarker(scenario, stepIndex, resultsetAggregateMedianAndDeviationStmtCase); err != nil {
		return err
	}
	stepIndex++
	stepIndex, err := validateResultSetAggregateMedianAndDeviationMarketSteps(scenario, stepIndex, resultsetAggregateMedianAndDeviationStmtCase)
	if err != nil {
		return err
	}

	for _, caseName := range []string{resultsetAggregateMedianAndDeviationJoinOMCase, resultsetAggregateMedianAndDeviationJoinCase} {
		if err := validateResultSetAggregateMedianAndDeviationCaseMarker(scenario, stepIndex, caseName); err != nil {
			return err
		}
		stepIndex++
		for seedIndex, expected := range []string{"DELL", "IBM", "AAA"} {
			step := scenario.Steps[stepIndex]
			if step.Op != "send" || step.Case != "" || step.EventType != "SupportBeanString" {
				return fmt.Errorf("%s case %q seed %d step %d must be a SupportBeanString send", scenarioID, caseName, seedIndex, stepIndex)
			}
			if err := validateResultSetAggregateMedianAndDeviationStepMetadata(step, "send"); err != nil {
				return fmt.Errorf("%s case %q seed %d step %d: %w", scenarioID, caseName, seedIndex, stepIndex, err)
			}
			actual, err := decodeResultSetAggregateMedianAndDeviationString(step)
			if err != nil {
				return fmt.Errorf("%s case %q seed %d: %w", scenarioID, caseName, seedIndex, err)
			}
			if actual.TheString != expected {
				return fmt.Errorf("%s case %q seed %d = %q, want %q", scenarioID, caseName, seedIndex, actual.TheString, expected)
			}
			stepIndex++
		}
		stepIndex, err = validateResultSetAggregateMedianAndDeviationMarketSteps(scenario, stepIndex, caseName)
		if err != nil {
			return err
		}
	}
	if stepIndex != len(scenario.Steps) {
		return fmt.Errorf("%s scenario contains trailing steps", scenarioID)
	}
	return nil
}

func validateResultSetAggregateMedianAndDeviationCaseMarker(scenario compat.Scenario, stepIndex int, caseName string) error {
	step := scenario.Steps[stepIndex]
	if step.Op != "case" || step.Case != caseName {
		return fmt.Errorf("%s scenario step %d must start case %q", scenario.ID, stepIndex, caseName)
	}
	if err := validateResultSetAggregateMedianAndDeviationStepMetadata(step, "case"); err != nil {
		return fmt.Errorf("%s scenario step %d: %w", scenario.ID, stepIndex, err)
	}
	return nil
}

func validateResultSetAggregateMedianAndDeviationStepMetadata(step compat.Step, kind string) error {
	if kind == "case" {
		if step.Statement != "" || step.Selector != "" || len(step.Hashes) != 0 || len(step.IDs) != 0 ||
			step.FilterProperty != "" || step.FilterValue != "" || step.ExpectError != "" || step.EventType != "" ||
			step.At != "" || step.Name != "" || step.Source != "" || len(step.PropertyOrder) != 0 ||
			len(step.PropertyTypes) != 0 || len(step.Payload) != 0 || step.Epl != "" || step.Mode != "" || step.Label != "" {
			return fmt.Errorf("case step has unsupported metadata")
		}
		return nil
	}
	if step.Case != "" || step.Statement != "" || step.Selector != "" || len(step.Hashes) != 0 || len(step.IDs) != 0 ||
		step.FilterProperty != "" || step.FilterValue != "" || step.ExpectError != "" || step.At != "" ||
		step.Name != "" || step.Source != "" || len(step.PropertyOrder) != 0 || len(step.PropertyTypes) != 0 ||
		step.Epl != "" || step.Mode != "" || step.Label != "" {
		return fmt.Errorf("send step has unsupported metadata")
	}
	return nil
}

func validateResultSetAggregateMedianAndDeviationMarketSteps(scenario compat.Scenario, stepIndex int, caseName string) (int, error) {
	for eventIndex, expectedPrice := range resultsetAggregateMedianAndDeviationPrices {
		step := scenario.Steps[stepIndex]
		if step.Op != "send" || step.Case != "" || step.EventType != "SupportMarketDataBean" {
			return stepIndex, fmt.Errorf("%s case %q event %d step %d must be a SupportMarketDataBean send", scenario.ID, caseName, eventIndex, stepIndex)
		}
		if err := validateResultSetAggregateMedianAndDeviationStepMetadata(step, "send"); err != nil {
			return stepIndex, fmt.Errorf("%s case %q event %d step %d: %w", scenario.ID, caseName, eventIndex, stepIndex, err)
		}
		actual, err := decodeResultSetAggregateMedianAndDeviationMarket(step)
		if err != nil {
			return stepIndex, fmt.Errorf("%s case %q event %d: %w", scenario.ID, caseName, eventIndex, err)
		}
		if actual.Symbol != "DELL" || actual.Price != expectedPrice || actual.Volume != 0 {
			return stepIndex, fmt.Errorf("%s case %q event %d = %#v, want symbol=DELL price=%v volume=0", scenario.ID, caseName, eventIndex, actual, expectedPrice)
		}
		stepIndex++
	}
	return stepIndex, nil
}

func decodeResultSetAggregateMedianAndDeviationPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportMarketDataBean":
		return decodeResultSetAggregateMedianAndDeviationMarket(step)
	case "SupportBeanString":
		return decodeResultSetAggregateMedianAndDeviationString(step)
	default:
		return nil, fmt.Errorf("unsupported resultset-aggregate-median-and-deviation event type %q", step.EventType)
	}
}

func decodeResultSetAggregateMedianAndDeviationMarket(step compat.Step) (resultsetAggregateMedianAndDeviationMarketData, error) {
	if step.EventType != "SupportMarketDataBean" {
		return resultsetAggregateMedianAndDeviationMarketData{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	payload, err := decodeResultSetAggregateMedianAndDeviationPayloadObject(step.Payload)
	if err != nil {
		return resultsetAggregateMedianAndDeviationMarketData{}, fmt.Errorf("payload must be an object: %w", err)
	}
	if len(payload) != 3 {
		return resultsetAggregateMedianAndDeviationMarketData{}, fmt.Errorf("payload must contain exactly symbol, price and volume")
	}
	var value resultsetAggregateMedianAndDeviationMarketData
	raw, ok := payload["symbol"]
	if !ok || string(bytes.TrimSpace(raw)) == "null" || json.Unmarshal(raw, &value.Symbol) != nil {
		return resultsetAggregateMedianAndDeviationMarketData{}, fmt.Errorf("payload symbol must be a string")
	}
	raw, ok = payload["price"]
	if !ok || string(bytes.TrimSpace(raw)) == "null" || json.Unmarshal(raw, &value.Price) != nil || math.IsNaN(value.Price) || math.IsInf(value.Price, 0) {
		return resultsetAggregateMedianAndDeviationMarketData{}, fmt.Errorf("payload price must be a finite number")
	}
	raw, ok = payload["volume"]
	if !ok || string(bytes.TrimSpace(raw)) == "null" || json.Unmarshal(raw, &value.Volume) != nil {
		return resultsetAggregateMedianAndDeviationMarketData{}, fmt.Errorf("payload volume must be an integer")
	}
	return value, nil
}

func decodeResultSetAggregateMedianAndDeviationString(step compat.Step) (resultsetAggregateMedianAndDeviationStringBean, error) {
	if step.EventType != "SupportBeanString" {
		return resultsetAggregateMedianAndDeviationStringBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	payload, err := decodeResultSetAggregateMedianAndDeviationPayloadObject(step.Payload)
	if err != nil {
		return resultsetAggregateMedianAndDeviationStringBean{}, fmt.Errorf("payload must be an object: %w", err)
	}
	if len(payload) != 1 {
		return resultsetAggregateMedianAndDeviationStringBean{}, fmt.Errorf("payload must contain exactly theString")
	}
	raw, ok := payload["theString"]
	if !ok || string(bytes.TrimSpace(raw)) == "null" {
		return resultsetAggregateMedianAndDeviationStringBean{}, fmt.Errorf("payload theString must be a string")
	}
	var value resultsetAggregateMedianAndDeviationStringBean
	if err := json.Unmarshal(raw, &value.TheString); err != nil {
		return resultsetAggregateMedianAndDeviationStringBean{}, fmt.Errorf("payload theString must be a string")
	}
	return value, nil
}

func decodeResultSetAggregateMedianAndDeviationPayloadObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('{') {
		return nil, fmt.Errorf("payload must be an object")
	}
	payload := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("payload object key is not a string")
		}
		if _, exists := payload[key]; exists {
			return nil, fmt.Errorf("payload contains duplicate field %q", key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		payload[key] = value
	}
	token, err = decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('}') {
		return nil, fmt.Errorf("payload object is not closed")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("payload contains trailing JSON")
		}
		return nil, err
	}
	return payload, nil
}
