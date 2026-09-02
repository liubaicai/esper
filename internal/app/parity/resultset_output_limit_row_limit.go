package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultsetOutputLimitRowLimitID          = "resultset-output-limit-row-limit"
	resultsetOutputLimitRowLimitJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOutputLimitRowLimitDescription = "ResultSetOutputLimitRowLimit ordinals 1 and 3: length-batch wildcard row limit with old/new streams and SODA round-trip."
	resultsetOutputLimitRowLimitSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowLimit.java"
)

var (
	resultsetOutputLimitRowLimitJavaRuntimeIDs = []string{
		"java-runtime-840e283d8ac639591083",
		"java-runtime-20d0d1451ce549bf53b5",
	}
	resultsetOutputLimitRowLimitJavaStaticIDs = []string{
		"java-2d2f2a8e404c07c322c6",
		"java-789d7de75e91392eb5e1",
	}
	resultsetOutputLimitRowLimitJavaSources    = []string{resultsetOutputLimitRowLimitSource}
	resultsetOutputLimitRowLimitJavaExecutions = []string{
		"ResultSetBatchNoOffsetNoOrder",
		"ResultSetBatchOffsetNoOrderOM",
	}
	resultsetOutputLimitRowLimitCases = []string{
		"batch-no-offset-no-order",
		"batch-offset-no-order-om",
	}
	resultsetOutputLimitRowLimitOrdinals = []int{1, 3}
	resultsetOutputLimitRowLimitEPLs     = []string{
		"@name('s0') select irstream * from SupportBean#length_batch(3) limit 1",
		"select irstream * from SupportBean#length_batch(3) limit 1",
	}
)

type resultsetOutputLimitRowLimitBean struct {
	TheString       string   `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int      `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	CharPrimitive   string   `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	IntBoxed        *int     `esper:"intBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	CharBoxed       *string  `esper:"charBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BigDecimal      *float64 `esper:"bigDecimal"`
	BigInteger      *int64   `esper:"bigInteger"`
	EnumValue       *string  `esper:"enumValue"`
}

func loadResultsetOutputLimitRowLimitScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOutputLimitRowLimitID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOutputLimitRowLimitID, err)
	}
	if err := rejectResultsetOutputLimitRowLimitDuplicateKeys(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitRowLimitID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitRowLimitID, err)
	}
	if err := requireResultsetOutputLimitRowLimitFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOutputLimitRowLimitID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetOutputLimitRowLimitID ||
		metadata.Description != resultsetOutputLimitRowLimitDescription || metadata.JavaCommit != resultsetOutputLimitRowLimitJavaCommit ||
		metadata.JavaSource != resultsetOutputLimitRowLimitSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOutputLimitRowLimitID)
	}
	if err := validateResultsetOutputLimitRowLimitStringArray(root["javaRuntimes"], resultsetOutputLimitRowLimitJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitRowLimitStringArray(root["javaNames"], resultsetOutputLimitRowLimitJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitRowLimitStringArray(root["javaStaticIds"], resultsetOutputLimitRowLimitJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitRowLimitStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetOutputLimitRowLimitCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly two cases", resultsetOutputLimitRowLimitID)
	}
	for index, rawCase := range rawCases {
		var entry struct {
			Case              string `json:"case"`
			Ordinal           int    `json:"ordinal"`
			RuntimeID         string `json:"runtimeId"`
			ExecutionName     string `json:"executionName"`
			Observation       string `json:"observation"`
			IteratorSnapshots int    `json:"iteratorSnapshots"`
			EPL               string `json:"epl"`
		}
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetOutputLimitRowLimitFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := json.Unmarshal(rawCase, &entry); err != nil || entry.Case != resultsetOutputLimitRowLimitCases[index] ||
			entry.Ordinal != resultsetOutputLimitRowLimitOrdinals[index] || entry.RuntimeID != resultsetOutputLimitRowLimitJavaRuntimeIDs[index] ||
			entry.ExecutionName != resultsetOutputLimitRowLimitJavaExecutions[index] || entry.Observation != "listener+iterator" &&
			entry.Observation != "listener+iterator+soda" || entry.IteratorSnapshots != 6 || entry.EPL != resultsetOutputLimitRowLimitEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetOutputLimitRowLimitID, index)
		}
		if index == 0 && entry.Observation != "listener+iterator" || index == 1 && entry.Observation != "listener+iterator+soda" {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d observation is not pinned", resultsetOutputLimitRowLimitID, index)
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps: %w", resultsetOutputLimitRowLimitID, err)
	}
	if len(rawSteps) != 26 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 26 steps", resultsetOutputLimitRowLimitID)
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
		expected := []string{"op", "case", "statement"}
		if operation == "case" {
			expected = []string{"op", "case"}
		} else if operation == "send" {
			expected = []string{"op", "case", "eventType", "payload"}
		}
		if err := requireResultsetOutputLimitRowLimitFields(object, expected...); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateResultsetOutputLimitRowLimitScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetOutputLimitRowLimitScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetOutputLimitRowLimitID || len(scenario.Steps) != 26 {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetOutputLimitRowLimitID)
	}
	index := 0
	for caseIndex, caseName := range resultsetOutputLimitRowLimitCases {
		if scenario.Steps[index].Op != "case" || scenario.Steps[index].Case != caseName {
			return fmt.Errorf("%s case marker %d is not pinned", resultsetOutputLimitRowLimitID, caseIndex)
		}
		index++
		for sendIndex := 0; sendIndex < 6; sendIndex++ {
			if scenario.Steps[index].Op != "snapshot" || scenario.Steps[index].Case != caseName || scenario.Steps[index].Statement != "s0" {
				return fmt.Errorf("%s case %q snapshot %d is not pinned", resultsetOutputLimitRowLimitID, caseName, sendIndex)
			}
			index++
			step := scenario.Steps[index]
			if step.Op != "send" || step.Case != caseName || step.EventType != "SupportBean" {
				return fmt.Errorf("%s case %q send %d is not pinned", resultsetOutputLimitRowLimitID, caseName, sendIndex)
			}
			decoded, err := decodeResultsetOutputLimitRowLimitPayload(step)
			if err != nil {
				return fmt.Errorf("%s case %q send %d: %w", resultsetOutputLimitRowLimitID, caseName, sendIndex, err)
			}
			payload, ok := decoded.(resultsetOutputLimitRowLimitBean)
			if !ok || payload.TheString != fmt.Sprintf("E%d", sendIndex+1) || payload.IntPrimitive != sendIndex+1 {
				return fmt.Errorf("%s case %q send %d payload is not pinned", resultsetOutputLimitRowLimitID, caseName, sendIndex)
			}
			index++
		}
		if caseIndex == 0 {
			// The first case ends with E6 and the next case marker follows it.
			continue
		}
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetOutputLimitRowLimitID)
	}
	return nil
}

func runResultsetOutputLimitRowLimitScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetOutputLimitRowLimitScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range resultsetOutputLimitRowLimitCases {
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultsetOutputLimitRowLimitCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOutputLimitRowLimitID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultsetOutputLimitRowLimitCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOutputLimitRowLimitBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	plan, err := env.Build(esper.From[resultsetOutputLimitRowLimitBean](env, "SupportBean").
		Window(esper.LengthBatch(3)).
		Query(esper.StatementName("s0"), esper.WithOldStream(), esper.Limit(1)))
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, resultsetOutputLimitRowLimitJavaRuntimeID(caseName))
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario,
		decodeResultsetOutputLimitRowLimitPayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetOutputLimitRowLimitID, name)
			}
			return statement, nil
		})
}

func resultsetOutputLimitRowLimitJavaRuntimeID(caseName string) string {
	for index, candidate := range resultsetOutputLimitRowLimitCases {
		if candidate == caseName {
			return resultsetOutputLimitRowLimitJavaRuntimeIDs[index]
		}
	}
	return "parity-" + resultsetOutputLimitRowLimitID + "-" + caseName
}

func decodeResultsetOutputLimitRowLimitPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, fmt.Errorf("SupportBean payload: %w", err)
	}
	if err := requireResultsetOutputLimitRowLimitFields(fields, "theString", "intPrimitive"); err != nil {
		return nil, fmt.Errorf("SupportBean payload: %w", err)
	}
	if string(bytes.TrimSpace(fields["theString"])) == "null" {
		return nil, fmt.Errorf("SupportBean payload theString must be a string")
	}
	var theString string
	if err := json.Unmarshal(fields["theString"], &theString); err != nil {
		return nil, fmt.Errorf("SupportBean payload theString must be a string")
	}
	intPrimitive, err := decodeResultsetOutputLimitRowLimitInteger(fields["intPrimitive"], "SupportBean payload intPrimitive")
	if err != nil {
		return nil, err
	}
	return resultsetOutputLimitRowLimitBean{TheString: theString, IntPrimitive: intPrimitive, CharPrimitive: "\u0000"}, nil
}

func requireResultsetOutputLimitRowLimitFields(object map[string]json.RawMessage, expected ...string) error {
	if len(object) != len(expected) {
		return fmt.Errorf("JSON object contains unexpected or missing fields")
	}
	for _, name := range expected {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

func validateResultsetOutputLimitRowLimitStringArray(raw json.RawMessage, expected []string, name string) error {
	var actual []string
	if err := json.Unmarshal(raw, &actual); err != nil || actual == nil || len(actual) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}

func rejectResultsetOutputLimitRowLimitDuplicateKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkResultsetOutputLimitRowLimitJSON(decoder); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON")
		}
		return err
	}
	return nil
}

func walkResultsetOutputLimitRowLimitJSON(decoder *json.Decoder) error {
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
			if err := walkResultsetOutputLimitRowLimitJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("JSON object is not closed")
		}
	case '[':
		for decoder.More() {
			if err := walkResultsetOutputLimitRowLimitJSON(decoder); err != nil {
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

func decodeResultsetOutputLimitRowLimitInteger(raw json.RawMessage, name string) (int, error) {
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
	value, err := strconv.ParseInt(string(number), 10, 64)
	if err != nil || int64(int(value)) != value {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	return int(value), nil
}
