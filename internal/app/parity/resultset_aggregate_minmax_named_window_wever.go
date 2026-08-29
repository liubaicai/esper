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

type resultsetAggregateMinMaxNamedWindowWEverBean struct {
	TheString    *string `esper:"theString"`
	IntPrimitive int     `esper:"intPrimitive"`
}

const (
	resultsetAggregateMinMaxNamedWindowWEverJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetAggregateMinMaxNamedWindowWEverSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateMinMax.java"
	resultsetAggregateMinMaxNamedWindowWEverDescription = "ResultSetAggregateMinMax ordinals 2-3 NamedWindowWEver: public length(2) named window min/max current plus minever/maxever; four SupportBean sends produce new-only rows and length eviction changes current extrema while ever values retain the first event."
	resultsetAggregateMinMaxNamedWindowWEverQueryEPL    = "@name('s0') select min(intPrimitive) as lower, max(intPrimitive) as upper, minever(intPrimitive) as lowerever, maxever(intPrimitive) as upperever from NamedWindow5m"
)

var (
	resultsetAggregateMinMaxNamedWindowWEverJavaRuntimeIDs = []string{
		"java-runtime-7d0a94525b0038b397fd",
		"java-runtime-840f1ca5610dd00d3ec0",
	}
	resultsetAggregateMinMaxNamedWindowWEverJavaExecutions = []string{
		"ResultSetAggregateMinMaxNamedWindowWEver{soda=false}",
		"ResultSetAggregateMinMaxNamedWindowWEver{soda=true}",
	}
	resultsetAggregateMinMaxNamedWindowWEverCases = []string{
		"named-window-wever-soda-false",
		"named-window-wever-soda-true",
	}
	resultsetAggregateMinMaxNamedWindowWEverOrdinals = []int{2, 3}
)

var resultsetAggregateMinMaxNamedWindowWEverJavaSources = []string{resultsetAggregateMinMaxNamedWindowWEverSource}
var resultsetAggregateMinMaxNamedWindowWEverStaticCandidates = []string{"java-09115f6e876ce88a42f9"}

func loadResultSetAggregateMinMaxNamedWindowWEverScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-minmax-named-window-wever scenario reader is required")
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read resultset-aggregate-minmax-named-window-wever scenario: %w", err)
	}
	if err := rejectResultSetAggregateMinMaxNamedWindowWEverDuplicateKeys(data); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-aggregate-minmax-named-window-wever scenario: %w", err)
	}
	root, err := decodeResultSetAggregateMinMaxNamedWindowWEverJSONObject(data)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-aggregate-minmax-named-window-wever scenario: %w", err)
	}
	if err := requireResultSetAggregateMinMaxNamedWindowWEverFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}

	version, err := decodeResultSetAggregateMinMaxNamedWindowWEverString(root["version"], "version")
	if err != nil {
		return compat.Scenario{}, err
	}
	id, err := decodeResultSetAggregateMinMaxNamedWindowWEverString(root["id"], "id")
	if err != nil {
		return compat.Scenario{}, err
	}
	description, err := decodeResultSetAggregateMinMaxNamedWindowWEverString(root["description"], "description")
	if err != nil {
		return compat.Scenario{}, err
	}
	javaCommit, err := decodeResultSetAggregateMinMaxNamedWindowWEverString(root["javaCommit"], "javaCommit")
	if err != nil {
		return compat.Scenario{}, err
	}
	javaSource, err := decodeResultSetAggregateMinMaxNamedWindowWEverString(root["javaSource"], "javaSource")
	if err != nil {
		return compat.Scenario{}, err
	}
	if version != compat.ScenarioVersion || id != "resultset-aggregate-minmax-named-window-wever" ||
		description != resultsetAggregateMinMaxNamedWindowWEverDescription ||
		javaCommit != resultsetAggregateMinMaxNamedWindowWEverJavaCommit || javaSource != resultsetAggregateMinMaxNamedWindowWEverSource {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-minmax-named-window-wever metadata is not pinned")
	}
	if err := validateResultSetAggregateMinMaxNamedWindowWEverStringArray(root["javaRuntimes"], resultsetAggregateMinMaxNamedWindowWEverJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateMinMaxNamedWindowWEverStringArray(root["javaNames"], resultsetAggregateMinMaxNamedWindowWEverJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateMinMaxNamedWindowWEverStringArray(root["javaStaticIds"], resultsetAggregateMinMaxNamedWindowWEverStaticCandidates, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateMinMaxNamedWindowWEverStringArray(root["javaFlags"], []string{"EXCLUDEWHENINSTRUMENTED"}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	caseValues, err := decodeResultSetAggregateMinMaxNamedWindowWEverJSONArray(root["cases"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario cases: %w", err)
	}
	if len(caseValues) != len(resultsetAggregateMinMaxNamedWindowWEverCases) {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-minmax-named-window-wever scenario must contain exactly two cases")
	}
	for index, rawCase := range caseValues {
		definition, err := decodeResultSetAggregateMinMaxNamedWindowWEverJSONObject(rawCase)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultSetAggregateMinMaxNamedWindowWEverFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		caseName, err := decodeResultSetAggregateMinMaxNamedWindowWEverString(definition["case"], fmt.Sprintf("scenario case %d case", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		ordinal, err := decodeResultSetAggregateMinMaxNamedWindowWEverInteger(definition["ordinal"], fmt.Sprintf("scenario case %d ordinal", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		runtimeID, err := decodeResultSetAggregateMinMaxNamedWindowWEverString(definition["runtimeId"], fmt.Sprintf("scenario case %d runtimeId", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		executionName, err := decodeResultSetAggregateMinMaxNamedWindowWEverString(definition["executionName"], fmt.Sprintf("scenario case %d executionName", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		observation, err := decodeResultSetAggregateMinMaxNamedWindowWEverString(definition["observation"], fmt.Sprintf("scenario case %d observation", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		epl, err := decodeResultSetAggregateMinMaxNamedWindowWEverString(definition["epl"], fmt.Sprintf("scenario case %d epl", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		if caseName != resultsetAggregateMinMaxNamedWindowWEverCases[index] ||
			ordinal != resultsetAggregateMinMaxNamedWindowWEverOrdinals[index] ||
			runtimeID != resultsetAggregateMinMaxNamedWindowWEverJavaRuntimeIDs[index] ||
			executionName != resultsetAggregateMinMaxNamedWindowWEverJavaExecutions[index] ||
			observation != "listener" || epl != resultsetAggregateMinMaxNamedWindowWEverQueryEPL {
			return compat.Scenario{}, fmt.Errorf("scenario case metadata mismatch at index %d", index)
		}
	}

	stepValues, err := decodeResultSetAggregateMinMaxNamedWindowWEverJSONArray(root["steps"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario steps: %w", err)
	}
	if len(stepValues) != len(resultsetAggregateMinMaxNamedWindowWEverCases)*5 {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-minmax-named-window-wever scenario must contain exactly ten steps")
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: make([]compat.Step, 0, len(stepValues))}
	for index, rawStep := range stepValues {
		stepObject, err := decodeResultSetAggregateMinMaxNamedWindowWEverJSONObject(rawStep)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if index%5 == 0 {
			if err := requireResultSetAggregateMinMaxNamedWindowWEverFields(stepObject, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			caseName, err := decodeResultSetAggregateMinMaxNamedWindowWEverString(stepObject["case"], fmt.Sprintf("scenario step %d case", index))
			if err != nil {
				return compat.Scenario{}, err
			}
			if string(bytes.TrimSpace(stepObject["op"])) != `"case"` || caseName != resultsetAggregateMinMaxNamedWindowWEverCases[index/5] {
				return compat.Scenario{}, fmt.Errorf("scenario step %d case marker mismatch", index)
			}
		} else {
			if err := requireResultSetAggregateMinMaxNamedWindowWEverFields(stepObject, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			step := compat.Step{Op: "send", EventType: "SupportBean", Payload: append(json.RawMessage(nil), stepObject["payload"]...)}
			if string(bytes.TrimSpace(stepObject["op"])) != `"send"` || string(bytes.TrimSpace(stepObject["eventType"])) != `"SupportBean"` {
				return compat.Scenario{}, fmt.Errorf("scenario step %d must send SupportBean", index)
			}
			if _, err := decodeResultSetAggregateMinMaxNamedWindowWEverBean(step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
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
	if err := validateResultSetAggregateMinMaxNamedWindowWEverScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func runResultSetAggregateMinMaxNamedWindowWEverScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateMinMaxNamedWindowWEverScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for index, caseName := range resultsetAggregateMinMaxNamedWindowWEverCases {
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultSetAggregateMinMaxNamedWindowWEverCase(ctx, caseScenario, caseName, resultsetAggregateMinMaxNamedWindowWEverJavaRuntimeIDs[index])
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-minmax-named-window-wever case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateMinMaxNamedWindowWEverCase(ctx context.Context, scenario compat.Scenario, caseName, runtimeURI string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	beanSchema, err := esper.RegisterStruct[resultsetAggregateMinMaxNamedWindowWEverBean](env, "SupportBean")
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateNamedWindow(env, "NamedWindow5m", beanSchema, esper.NamedWindowRetention(esper.LengthWindow(2))); err != nil {
		return compat.Trace{}, err
	}

	beanSource := esper.From[resultsetAggregateMinMaxNamedWindowWEverBean](env, "SupportBean")
	insertPlan, err := env.Build(esper.OnEvent(beanSource).InsertIntoNamedWindow(
		"NamedWindow5m", esper.CopyMatchingFields(),
	).Query(esper.StatementName("insert")))
	if err != nil {
		return compat.Trace{}, err
	}
	intPrimitive := esper.Field[resultsetAggregateMinMaxNamedWindowWEverBean, int]("intPrimitive")
	query := esper.FromNamedWindowAs[resultsetAggregateMinMaxNamedWindowWEverBean](env, "NamedWindow5m").Aggregate(
		esper.Alias("lower", esper.Min[int](intPrimitive)),
		esper.Alias("upper", esper.Max[int](intPrimitive)),
		esper.Alias("lowerever", esper.MinEver[int](intPrimitive)),
		esper.Alias("upperever", esper.MaxEver[int](intPrimitive)),
	).Query(esper.StatementName("s0"))
	aggregatePlan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()), esper.WithRuntimeURI(runtimeURI))
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := engine.Deploy(ctx, insertPlan); err != nil {
		return compat.Trace{}, err
	}
	deployment, err := engine.Deploy(ctx, aggregatePlan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one resultset aggregate statement, got %d", len(statements))
	}
	statement := statements[0]
	if statement.Name() != "s0" {
		return compat.Trace{}, fmt.Errorf("expected resultset aggregate statement s0, got %q", statement.Name())
	}
	return compat.ReplayWithStatements(ctx, engine, statement, scenario, decodeResultSetAggregateMinMaxNamedWindowWEverPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-minmax-named-window-wever statement %q", name)
		}
		return statement, nil
	})
}

func validateResultSetAggregateMinMaxNamedWindowWEverScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != "resultset-aggregate-minmax-named-window-wever" {
		return fmt.Errorf("resultset-aggregate-minmax-named-window-wever scenario has unsupported id %q", scenario.ID)
	}
	if len(scenario.Steps) != len(resultsetAggregateMinMaxNamedWindowWEverCases)*5 {
		return fmt.Errorf("resultset-aggregate-minmax-named-window-wever scenario must contain exactly ten steps")
	}
	for caseIndex, caseName := range resultsetAggregateMinMaxNamedWindowWEverCases {
		offset := caseIndex * 5
		marker := scenario.Steps[offset]
		if marker.Op != "case" || marker.Case != caseName {
			return fmt.Errorf("resultset-aggregate-minmax-named-window-wever step %d must start case %q", offset, caseName)
		}
		for eventIndex, expected := range []int{1, 5, 3, 6} {
			step := scenario.Steps[offset+eventIndex+1]
			if step.Op != "send" || step.EventType != "SupportBean" || step.Case != "" {
				return fmt.Errorf("resultset-aggregate-minmax-named-window-wever case %q event %d must be an unscoped SupportBean send", caseName, eventIndex)
			}
			value, err := decodeResultSetAggregateMinMaxNamedWindowWEverBean(step)
			if err != nil {
				return fmt.Errorf("resultset-aggregate-minmax-named-window-wever case %q event %d: %w", caseName, eventIndex, err)
			}
			if value.TheString != nil || value.IntPrimitive != expected {
				return fmt.Errorf("resultset-aggregate-minmax-named-window-wever case %q event %d has unexpected payload", caseName, eventIndex)
			}
		}
	}
	return nil
}

func decodeResultSetAggregateMinMaxNamedWindowWEverPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("unsupported resultset-aggregate-minmax-named-window-wever event type %q", step.EventType)
	}
	return decodeResultSetAggregateMinMaxNamedWindowWEverBean(step)
}

func decodeResultSetAggregateMinMaxNamedWindowWEverBean(step compat.Step) (resultsetAggregateMinMaxNamedWindowWEverBean, error) {
	if step.EventType != "SupportBean" {
		return resultsetAggregateMinMaxNamedWindowWEverBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	payload, err := decodeResultSetAggregateMinMaxNamedWindowWEverPayloadObject(step.Payload)
	if err != nil {
		return resultsetAggregateMinMaxNamedWindowWEverBean{}, fmt.Errorf("payload must be an object: %w", err)
	}
	if len(payload) != 2 {
		return resultsetAggregateMinMaxNamedWindowWEverBean{}, fmt.Errorf("SupportBean payload must contain exactly theString and intPrimitive")
	}
	value := resultsetAggregateMinMaxNamedWindowWEverBean{}
	text, ok := payload["theString"]
	if !ok || string(bytes.TrimSpace(text)) != "null" {
		return resultsetAggregateMinMaxNamedWindowWEverBean{}, fmt.Errorf("theString must be JSON null")
	}
	intPrimitive, ok := payload["intPrimitive"]
	if !ok {
		return resultsetAggregateMinMaxNamedWindowWEverBean{}, fmt.Errorf("intPrimitive must be an integer")
	}
	number, err := decodeResultSetAggregateMinMaxNamedWindowWEverInteger(intPrimitive, "intPrimitive")
	if err != nil {
		return resultsetAggregateMinMaxNamedWindowWEverBean{}, err
	}
	value.IntPrimitive = number
	return value, nil
}

func rejectResultSetAggregateMinMaxNamedWindowWEverDuplicateKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkResultSetAggregateMinMaxNamedWindowWEverJSON(decoder); err != nil {
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

func walkResultSetAggregateMinMaxNamedWindowWEverJSON(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
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
			if err := walkResultSetAggregateMinMaxNamedWindowWEverJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("JSON object is not closed")
		}
	case '[':
		for decoder.More() {
			if err := walkResultSetAggregateMinMaxNamedWindowWEverJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return fmt.Errorf("JSON array is not closed")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	return nil
}

func decodeResultSetAggregateMinMaxNamedWindowWEverJSONObject(raw []byte) (map[string]json.RawMessage, error) {
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
		return nil, fmt.Errorf("JSON object contains trailing data")
	}
	return object, nil
}

func decodeResultSetAggregateMinMaxNamedWindowWEverJSONArray(raw json.RawMessage) ([]json.RawMessage, error) {
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
		return nil, fmt.Errorf("JSON array contains trailing data")
	}
	return values, nil
}

func requireResultSetAggregateMinMaxNamedWindowWEverFields(object map[string]json.RawMessage, names ...string) error {
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

func decodeResultSetAggregateMinMaxNamedWindowWEverString(raw json.RawMessage, name string) (string, error) {
	if len(raw) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return "", fmt.Errorf("%s must be a string", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func decodeResultSetAggregateMinMaxNamedWindowWEverInteger(raw json.RawMessage, name string) (int, error) {
	if len(raw) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	var number float64
	if err := json.Unmarshal(raw, &number); err != nil || math.IsNaN(number) || math.IsInf(number, 0) || number != math.Trunc(number) || number < math.MinInt32 || number > math.MaxInt32 {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	return int(number), nil
}

func validateResultSetAggregateMinMaxNamedWindowWEverStringArray(raw json.RawMessage, expected []string, name string) error {
	values, err := decodeResultSetAggregateMinMaxNamedWindowWEverJSONArray(raw)
	if err != nil {
		return fmt.Errorf("%s must be an array: %w", name, err)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s must contain exactly %d values", name, len(expected))
	}
	for index, rawValue := range values {
		value, err := decodeResultSetAggregateMinMaxNamedWindowWEverString(rawValue, fmt.Sprintf("%s[%d]", name, index))
		if err != nil {
			return err
		}
		if value != expected[index] {
			return fmt.Errorf("%s mismatch at index %d", name, index)
		}
	}
	return nil
}

func decodeResultSetAggregateMinMaxNamedWindowWEverPayloadObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("payload must be an object")
	}
	payload := make(map[string]json.RawMessage)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyToken.(string)
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
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return nil, fmt.Errorf("payload object is not closed")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("payload contains trailing JSON")
	}
	return payload, nil
}
