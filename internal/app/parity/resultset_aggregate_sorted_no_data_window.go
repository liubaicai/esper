package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// ResultSetAggregateSortedMinMaxBy ordinal 6 exercises current and ever
// min/max-by access without declaring a data window. The typed query below
// keeps the Java observable contract while avoiding an EPL-facing API.
type resultsetAggregateSortedNoDataWindowBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const (
	resultsetAggregateSortedNoDataWindowJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetAggregateSortedNoDataWindowID          = "resultset-aggregate-sorted-no-data-window"
	resultsetAggregateSortedNoDataWindowCase        = resultsetAggregateSortedNoDataWindowID
	resultsetAggregateSortedNoDataWindowDescription = "ResultSetAggregateSortedMinMaxBy ordinal 6: unwindowed current and ever min/max-by projections."
	resultsetAggregateSortedNoDataWindowSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateSortedMinMaxBy.java"
	resultsetAggregateSortedNoDataWindowRuntimeID   = "java-runtime-af551963966a83d26468"
	resultsetAggregateSortedNoDataWindowStaticID    = "java-f645b6fb41fd8c63d7f0"
	resultsetAggregateSortedNoDataWindowExecution   = "ResultSetAggregateNoDataWindow"
	resultsetAggregateSortedNoDataWindowEPL         = "@name('s0') select maxbyever(intPrimitive).theString as c0, minbyever(intPrimitive).theString as c1, maxby(intPrimitive).theString as c2, minby(intPrimitive).theString as c3 from SupportBean"
)

var (
	resultsetAggregateSortedNoDataWindowJavaRuntimeIDs = []string{resultsetAggregateSortedNoDataWindowRuntimeID}
	resultsetAggregateSortedNoDataWindowJavaSources    = []string{resultsetAggregateSortedNoDataWindowSource}
	resultsetAggregateSortedNoDataWindowJavaExecutions = []string{resultsetAggregateSortedNoDataWindowExecution}
)

type resultsetAggregateSortedNoDataWindowExpectedEvent struct {
	theString    string
	intPrimitive int
}

var resultsetAggregateSortedNoDataWindowExpected = []resultsetAggregateSortedNoDataWindowExpectedEvent{
	{theString: "E1", intPrimitive: 1},
	{theString: "E2", intPrimitive: 2},
	{theString: "E3", intPrimitive: 0},
	{theString: "E4", intPrimitive: 3},
}

var resultsetAggregateSortedNoDataWindowExpectedRows = [][]string{
	{"E1", "E1", "E1", "E1"},
	{"E2", "E1", "E2", "E1"},
	{"E2", "E3", "E2", "E3"},
	{"E4", "E3", "E4", "E3"},
}

func loadResultSetAggregateSortedNoDataWindowScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetAggregateSortedNoDataWindowID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetAggregateSortedNoDataWindowID, err)
	}
	if err := rejectResultSetAggregateSortedNoDataWindowDuplicateKeys(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateSortedNoDataWindowID, err)
	}
	root, err := decodeResultSetAggregateSortedNoDataWindowJSONObject(raw)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateSortedNoDataWindowID, err)
	}
	if err := requireResultSetAggregateSortedNoDataWindowFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}

	metadata := map[string]*string{
		"version":     new(string),
		"id":          new(string),
		"description": new(string),
		"javaCommit":  new(string),
		"javaSource":  new(string),
	}
	for name, target := range metadata {
		if err := json.Unmarshal(root[name], target); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario %s must be a string", name)
		}
	}
	if *metadata["version"] != compat.ScenarioVersion ||
		*metadata["id"] != resultsetAggregateSortedNoDataWindowID ||
		*metadata["description"] != resultsetAggregateSortedNoDataWindowDescription ||
		*metadata["javaCommit"] != resultsetAggregateSortedNoDataWindowJavaCommit ||
		*metadata["javaSource"] != resultsetAggregateSortedNoDataWindowSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetAggregateSortedNoDataWindowID)
	}
	if err := validateResultSetAggregateSortedNoDataWindowStringArray(root["javaRuntimes"], resultsetAggregateSortedNoDataWindowJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateSortedNoDataWindowStringArray(root["javaNames"], resultsetAggregateSortedNoDataWindowJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateSortedNoDataWindowStringArray(root["javaStaticIds"], []string{resultsetAggregateSortedNoDataWindowStaticID}, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateSortedNoDataWindowStringArray(root["javaFlags"], nil, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	rawCases, err := decodeResultSetAggregateSortedNoDataWindowJSONArray(root["cases"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario cases: %w", err)
	}
	if len(rawCases) != 1 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly one case", resultsetAggregateSortedNoDataWindowID)
	}
	caseObject, err := decodeResultSetAggregateSortedNoDataWindowJSONObject(rawCases[0])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	if err := requireResultSetAggregateSortedNoDataWindowFields(caseObject,
		"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	var caseMetadata struct {
		Case              string `json:"case"`
		Ordinal           int    `json:"ordinal"`
		RuntimeID         string `json:"runtimeId"`
		ExecutionName     string `json:"executionName"`
		Observation       string `json:"observation"`
		IteratorSnapshots int    `json:"iteratorSnapshots"`
		EPL               string `json:"epl"`
	}
	if err := json.Unmarshal(rawCases[0], &caseMetadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case metadata has invalid types")
	}
	if caseMetadata.Case != resultsetAggregateSortedNoDataWindowCase ||
		caseMetadata.Ordinal != 6 ||
		caseMetadata.RuntimeID != resultsetAggregateSortedNoDataWindowRuntimeID ||
		caseMetadata.ExecutionName != resultsetAggregateSortedNoDataWindowExecution ||
		caseMetadata.Observation != "listener" || caseMetadata.IteratorSnapshots != 0 ||
		caseMetadata.EPL != resultsetAggregateSortedNoDataWindowEPL {
		return compat.Scenario{}, fmt.Errorf("%s scenario case metadata is not pinned", resultsetAggregateSortedNoDataWindowID)
	}

	rawSteps, err := decodeResultSetAggregateSortedNoDataWindowJSONArray(root["steps"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario steps: %w", err)
	}
	if len(rawSteps) != 1+len(resultsetAggregateSortedNoDataWindowExpected) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly five steps", resultsetAggregateSortedNoDataWindowID)
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		object, err := decodeResultSetAggregateSortedNoDataWindowJSONObject(rawStep)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if index == 0 {
			if err := requireResultSetAggregateSortedNoDataWindowFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		} else if err := requireResultSetAggregateSortedNoDataWindowFields(object, "op", "eventType", "payload"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: *metadata["version"], ID: *metadata["id"], Steps: steps}
	if err := validateResultSetAggregateSortedNoDataWindowScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func runResultSetAggregateSortedNoDataWindowScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateSortedNoDataWindowScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	caseScenario, err := scenarioForCase(scenario, resultsetAggregateSortedNoDataWindowCase)
	if err != nil {
		return compat.Trace{}, err
	}

	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateSortedNoDataWindowBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	intPrimitive := esper.Field[resultsetAggregateSortedNoDataWindowBean, int]("intPrimitive")
	eventValue := esper.EventValue[esper.Event]()
	maxEver := esper.MaxByEver[esper.Event, int](eventValue, intPrimitive)
	minEver := esper.MinByEver[esper.Event, int](eventValue, intPrimitive)
	maxCurrent := esper.MaxBy[esper.Event, int](eventValue, intPrimitive)
	minCurrent := esper.MinBy[esper.Event, int](eventValue, intPrimitive)
	query := esper.From[resultsetAggregateSortedNoDataWindowBean](env, "SupportBean").Aggregate(
		esper.Alias("c0", esper.Property[string](maxEver, "theString")),
		esper.Alias("c1", esper.Property[string](minEver, "theString")),
		esper.Alias("c2", esper.Property[string](maxCurrent, "theString")),
		esper.Alias("c3", esper.Property[string](minCurrent, "theString")),
	).Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateSortedNoDataWindowRuntimeID),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one %s statement, got %d", resultsetAggregateSortedNoDataWindowID, len(statements))
	}
	statement := statements[0]
	trace, err := compat.ReplayWithStatements(ctx, engine, statement, caseScenario,
		decodeResultSetAggregateSortedNoDataWindowPayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetAggregateSortedNoDataWindowID, name)
			}
			return statement, nil
		})
	if err != nil {
		return compat.Trace{}, err
	}
	if err := validateResultSetAggregateSortedNoDataWindowTrace(trace); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

func validateResultSetAggregateSortedNoDataWindowScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetAggregateSortedNoDataWindowID {
		return fmt.Errorf("%s scenario has unsupported id %q", resultsetAggregateSortedNoDataWindowID, scenario.ID)
	}
	if len(scenario.Steps) != 1+len(resultsetAggregateSortedNoDataWindowExpected) {
		return fmt.Errorf("%s scenario must contain exactly five steps", resultsetAggregateSortedNoDataWindowID)
	}
	marker := scenario.Steps[0]
	if marker.Op != "case" || marker.Case != resultsetAggregateSortedNoDataWindowCase {
		return fmt.Errorf("%s scenario must start with case %q", resultsetAggregateSortedNoDataWindowID, resultsetAggregateSortedNoDataWindowCase)
	}
	for index, expected := range resultsetAggregateSortedNoDataWindowExpected {
		step := scenario.Steps[index+1]
		if step.Op != "send" || step.Case != "" || step.EventType != "SupportBean" {
			return fmt.Errorf("%s scenario step %d must be an unscoped SupportBean send", resultsetAggregateSortedNoDataWindowID, index+1)
		}
		actual, err := decodeResultSetAggregateSortedNoDataWindowPayloadValue(step)
		if err != nil {
			return fmt.Errorf("%s scenario event %d: %w", resultsetAggregateSortedNoDataWindowID, index, err)
		}
		if actual != expected {
			return fmt.Errorf("%s scenario event %d = %#v, want %#v", resultsetAggregateSortedNoDataWindowID, index, actual, expected)
		}
	}
	return nil
}

func validateResultSetAggregateSortedNoDataWindowTrace(trace compat.Trace) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != resultsetAggregateSortedNoDataWindowID {
		return fmt.Errorf("%s trace identity is not pinned", resultsetAggregateSortedNoDataWindowID)
	}
	if len(trace.Records) != len(resultsetAggregateSortedNoDataWindowExpectedRows) {
		return fmt.Errorf("%s trace must contain exactly four records", resultsetAggregateSortedNoDataWindowID)
	}
	for index, record := range trace.Records {
		if record.Case != resultsetAggregateSortedNoDataWindowCase || record.Operation != "listener" ||
			record.Statement != "s0" || record.Sequence != uint64(index+1) ||
			record.Time != "1970-01-01T00:00:00Z" || len(record.New) != 1 || len(record.Old) != 0 {
			return fmt.Errorf("%s trace record %d metadata is not pinned", resultsetAggregateSortedNoDataWindowID, index)
		}
		row := record.New[0]
		if row.Kind != "row" || len(row.Fields) != 4 {
			return fmt.Errorf("%s trace record %d row shape is not pinned", resultsetAggregateSortedNoDataWindowID, index)
		}
		for fieldIndex, expected := range resultsetAggregateSortedNoDataWindowExpectedRows[index] {
			value, ok := row.Fields[fmt.Sprintf("c%d", fieldIndex)].(string)
			if !ok || value != expected {
				return fmt.Errorf("%s trace record %d field c%d = %#v, want %q", resultsetAggregateSortedNoDataWindowID, index, fieldIndex, row.Fields[fmt.Sprintf("c%d", fieldIndex)], expected)
			}
		}
	}
	return nil
}

func decodeResultSetAggregateSortedNoDataWindowPayload(step compat.Step) (any, error) {
	value, err := decodeResultSetAggregateSortedNoDataWindowPayloadValue(step)
	if err != nil {
		return nil, err
	}
	return resultsetAggregateSortedNoDataWindowBean{
		TheString:    value.theString,
		IntPrimitive: value.intPrimitive,
	}, nil
}

func decodeResultSetAggregateSortedNoDataWindowPayloadValue(step compat.Step) (resultsetAggregateSortedNoDataWindowExpectedEvent, error) {
	if step.EventType != "SupportBean" {
		return resultsetAggregateSortedNoDataWindowExpectedEvent{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	payload, err := decodeResultSetAggregateSortedNoDataWindowJSONObject(step.Payload)
	if err != nil {
		return resultsetAggregateSortedNoDataWindowExpectedEvent{}, fmt.Errorf("payload must be an object: %w", err)
	}
	if err := requireResultSetAggregateSortedNoDataWindowFields(payload, "theString", "intPrimitive"); err != nil {
		return resultsetAggregateSortedNoDataWindowExpectedEvent{}, fmt.Errorf("payload: %w", err)
	}
	var value resultsetAggregateSortedNoDataWindowExpectedEvent
	if err := json.Unmarshal(payload["theString"], &value.theString); err != nil {
		return resultsetAggregateSortedNoDataWindowExpectedEvent{}, fmt.Errorf("payload theString must be a string")
	}
	value.intPrimitive, err = decodeResultSetAggregateSortedNoDataWindowInteger(payload["intPrimitive"], "intPrimitive")
	if err != nil {
		return resultsetAggregateSortedNoDataWindowExpectedEvent{}, err
	}
	return value, nil
}

func decodeResultSetAggregateSortedNoDataWindowInteger(raw json.RawMessage, name string) (int, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	number, ok := token.(json.Number)
	if err != nil || !ok || number == "" {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	value, err := strconv.ParseInt(string(number), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	return int(value), nil
}

func rejectResultSetAggregateSortedNoDataWindowDuplicateKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkResultSetAggregateSortedNoDataWindowJSON(decoder); err != nil {
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

func walkResultSetAggregateSortedNoDataWindowJSON(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch token {
	case json.Delim('{'):
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
			if err := walkResultSetAggregateSortedNoDataWindowJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("JSON object is not closed")
		}
	case json.Delim('['):
		for decoder.More() {
			if err := walkResultSetAggregateSortedNoDataWindowJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return fmt.Errorf("JSON array is not closed")
		}
	case json.Delim(']'), json.Delim('}'):
		return fmt.Errorf("unexpected JSON delimiter %q", token)
	}
	return nil
}

func decodeResultSetAggregateSortedNoDataWindowJSONObject(raw []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
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
	if err != nil || closing != json.Delim('}') {
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

func decodeResultSetAggregateSortedNoDataWindowJSONArray(raw json.RawMessage) ([]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
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
	if err != nil || closing != json.Delim(']') {
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

func requireResultSetAggregateSortedNoDataWindowFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("JSON object has unexpected or missing fields")
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

func validateResultSetAggregateSortedNoDataWindowStringArray(raw json.RawMessage, expected []string, name string) error {
	values, err := decodeResultSetAggregateSortedNoDataWindowJSONArray(raw)
	if err != nil {
		return fmt.Errorf("%s must be an array: %w", name, err)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index, rawValue := range values {
		var value string
		if err := json.Unmarshal(rawValue, &value); err != nil || value != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}
