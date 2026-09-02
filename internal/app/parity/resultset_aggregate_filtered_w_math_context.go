package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetAggregateFilteredWMathContextNumeric struct {
	BigInt big.Int `esper:"bigint"`
	BigDec big.Rat `esper:"bigdec"`
}

const (
	resultsetAggregateFilteredWMathContextID          = "resultset-aggregate-filtered-w-math-context"
	resultsetAggregateFilteredWMathContextDescription = "ResultSetAggregateFilteredWMathContext ordinal 0: compiler MathContext precision 2 HALF_UP rounds an unbounded BigDecimal average; scale-discarding inputs 0, 0, and 1 produce new-only listener rows 0, 0, and 0.33."
	resultsetAggregateFilteredWMathContextJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetAggregateFilteredWMathContextSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilteredWMathContext.java"
	resultsetAggregateFilteredWMathContextRuntimeID   = "java-runtime-fa8b6d5d6fb58a905f23"
	resultsetAggregateFilteredWMathContextStaticID    = "java-aba2cfbf41a3be9809f1"
	resultsetAggregateFilteredWMathContextExecution   = "ResultSetAggregateFilteredWMathContext"
	resultsetAggregateFilteredWMathContextCase        = resultsetAggregateFilteredWMathContextID
	resultsetAggregateFilteredWMathContextEPL         = "@name('s0') select avg(bigdec) as c0 from SupportBeanNumeric"
)

var (
	resultsetAggregateFilteredWMathContextJavaSources    = []string{resultsetAggregateFilteredWMathContextSource}
	resultsetAggregateFilteredWMathContextJavaRuntimeIDs = []string{resultsetAggregateFilteredWMathContextRuntimeID}
	resultsetAggregateFilteredWMathContextJavaExecutions = []string{resultsetAggregateFilteredWMathContextExecution}
	resultsetAggregateFilteredWMathContextCases          = []string{resultsetAggregateFilteredWMathContextCase}
)

func loadResultSetAggregateFilteredWMathContextScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetAggregateFilteredWMathContextID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetAggregateFilteredWMathContextID, err)
	}
	if err := rejectResultSetAggregateFilteredWMathContextDuplicateKeys(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateFilteredWMathContextID, err)
	}
	root, err := decodeResultSetAggregateFilteredWMathContextJSONObject(raw)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateFilteredWMathContextID, err)
	}
	if err := requireResultSetAggregateFilteredWMathContextFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	metadata := make(map[string]string, 5)
	for _, name := range []string{"version", "id", "description", "javaCommit", "javaSource"} {
		var value string
		if err := json.Unmarshal(root[name], &value); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario %s must be a string", name)
		}
		metadata[name] = value
	}
	if metadata["version"] != compat.ScenarioVersion || metadata["id"] != resultsetAggregateFilteredWMathContextID ||
		metadata["description"] != resultsetAggregateFilteredWMathContextDescription ||
		metadata["javaCommit"] != resultsetAggregateFilteredWMathContextJavaCommit ||
		metadata["javaSource"] != resultsetAggregateFilteredWMathContextSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetAggregateFilteredWMathContextID)
	}
	for name, expected := range map[string][]string{
		"javaRuntimes":  resultsetAggregateFilteredWMathContextJavaRuntimeIDs,
		"javaNames":     resultsetAggregateFilteredWMathContextJavaExecutions,
		"javaStaticIds": {resultsetAggregateFilteredWMathContextStaticID},
		"javaFlags":     {},
	} {
		if err := validateResultSetAggregateFilteredWMathContextStringArray(root[name], expected, name); err != nil {
			return compat.Scenario{}, err
		}
	}

	rawCases, err := decodeResultSetAggregateFilteredWMathContextJSONArray(root["cases"])
	if err != nil || len(rawCases) != 1 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly one case", resultsetAggregateFilteredWMathContextID)
	}
	caseObject, err := decodeResultSetAggregateFilteredWMathContextJSONObject(rawCases[0])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	if err := requireResultSetAggregateFilteredWMathContextFields(caseObject,
		"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	var caseMeta struct {
		Case              string `json:"case"`
		Ordinal           int    `json:"ordinal"`
		RuntimeID         string `json:"runtimeId"`
		ExecutionName     string `json:"executionName"`
		Observation       string `json:"observation"`
		IteratorSnapshots int    `json:"iteratorSnapshots"`
		EPL               string `json:"epl"`
	}
	caseData, _ := json.Marshal(caseObject)
	if err := json.Unmarshal(caseData, &caseMeta); err != nil ||
		caseMeta.Case != resultsetAggregateFilteredWMathContextCase || caseMeta.Ordinal != 0 ||
		caseMeta.RuntimeID != resultsetAggregateFilteredWMathContextRuntimeID ||
		caseMeta.ExecutionName != resultsetAggregateFilteredWMathContextExecution ||
		caseMeta.Observation != "listener" || caseMeta.IteratorSnapshots != 0 ||
		caseMeta.EPL != resultsetAggregateFilteredWMathContextEPL {
		return compat.Scenario{}, fmt.Errorf("%s scenario case metadata is not pinned", resultsetAggregateFilteredWMathContextID)
	}

	rawSteps, err := decodeResultSetAggregateFilteredWMathContextJSONArray(root["steps"])
	if err != nil || len(rawSteps) != 4 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly four steps", resultsetAggregateFilteredWMathContextID)
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		object, err := decodeResultSetAggregateFilteredWMathContextJSONObject(rawStep)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if index == 0 {
			if err := requireResultSetAggregateFilteredWMathContextFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		} else if err := requireResultSetAggregateFilteredWMathContextFields(object, "op", "eventType", "payload"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		encoded, _ := json.Marshal(object)
		if err := json.Unmarshal(encoded, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata["version"], ID: metadata["id"], Steps: steps}
	if err := validateResultSetAggregateFilteredWMathContextScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultSetAggregateFilteredWMathContextScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetAggregateFilteredWMathContextID || len(scenario.Steps) != 4 {
		return fmt.Errorf("%s scenario steps are not pinned", resultsetAggregateFilteredWMathContextID)
	}
	marker := scenario.Steps[0]
	if marker.Op != "case" || marker.Case != resultsetAggregateFilteredWMathContextCase {
		return fmt.Errorf("%s scenario must start with case %q", resultsetAggregateFilteredWMathContextID, resultsetAggregateFilteredWMathContextCase)
	}
	for index, expected := range []string{"0", "0", "1"} {
		step := scenario.Steps[index+1]
		if step.Op != "send" || step.Case != "" || step.EventType != "SupportBeanNumeric" {
			return fmt.Errorf("%s scenario step %d must be an unscoped SupportBeanNumeric send", resultsetAggregateFilteredWMathContextID, index+1)
		}
		var payload struct {
			BigInt json.RawMessage `json:"bigint"`
			BigDec string          `json:"bigdec"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil || string(bytes.TrimSpace(payload.BigInt)) != "null" || payload.BigDec != expected {
			return fmt.Errorf("%s scenario step %d payload is not pinned", resultsetAggregateFilteredWMathContextID, index+1)
		}
		var object map[string]json.RawMessage
		if err := strictResultSetAggregateFilteredWMathContextObject(step.Payload, &object); err != nil || len(object) != 2 {
			return fmt.Errorf("%s scenario step %d payload is not pinned", resultsetAggregateFilteredWMathContextID, index+1)
		}
	}
	return nil
}

func runResultSetAggregateFilteredWMathContextScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateFilteredWMathContextScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	caseScenario, err := scenarioForCase(scenario, resultsetAggregateFilteredWMathContextCase)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment(esper.WithDecimalMathContext(esper.DecimalMathContext{
		Precision: 2,
		Rounding:  esper.DecimalRoundHalfUp,
	}))
	if _, err := esper.RegisterStruct[resultsetAggregateFilteredWMathContextNumeric](env, "SupportBeanNumeric"); err != nil {
		return compat.Trace{}, err
	}
	bigDec := esper.Field[resultsetAggregateFilteredWMathContextNumeric, big.Rat]("bigdec")
	query := esper.From[resultsetAggregateFilteredWMathContextNumeric](env, "SupportBeanNumeric").Aggregate(
		esper.Alias("c0", esper.AvgExact[big.Rat](bigDec)),
	).Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateFilteredWMathContextRuntimeID),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one %s statement, got %d", resultsetAggregateFilteredWMathContextID, len(statements))
	}
	statement := statements[0]
	trace, err := compat.ReplayWithStatements(ctx, engine, statement, caseScenario,
		decodeResultSetAggregateFilteredWMathContextPayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetAggregateFilteredWMathContextID, name)
			}
			return statement, nil
		})
	if err != nil {
		return compat.Trace{}, err
	}
	for index := range trace.Records {
		trace.Records[index].New = normalizeResultSetAggregateFilteredWMathContextNumbers(trace.Records[index].New)
		trace.Records[index].Old = normalizeResultSetAggregateFilteredWMathContextNumbers(trace.Records[index].Old)
	}
	return trace, nil
}

func decodeResultSetAggregateFilteredWMathContextPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBeanNumeric" {
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetAggregateFilteredWMathContextID, step.EventType)
	}
	var raw struct {
		BigInt json.RawMessage `json:"bigint"`
		BigDec string          `json:"bigdec"`
	}
	if err := json.Unmarshal(step.Payload, &raw); err != nil {
		return nil, fmt.Errorf("decode SupportBeanNumeric: %w", err)
	}
	if string(bytes.TrimSpace(raw.BigInt)) != "null" {
		return nil, fmt.Errorf("decode bigint: expected null")
	}
	bigDec, ok := new(big.Rat).SetString(raw.BigDec)
	if !ok {
		return nil, fmt.Errorf("decode bigdec: invalid exact decimal %q", raw.BigDec)
	}
	return resultsetAggregateFilteredWMathContextNumeric{BigDec: *bigDec}, nil
}
func normalizeResultSetAggregateFilteredWMathContextNumbers(rows []compat.ResultRecord) []compat.ResultRecord {
	for rowIndex := range rows {
		for field, value := range rows[rowIndex].Fields {
			switch typed := value.(type) {
			case big.Int:
				rows[rowIndex].Fields[field] = typed.String()
			case *big.Int:
				if typed == nil {
					rows[rowIndex].Fields[field] = nil
				} else {
					rows[rowIndex].Fields[field] = typed.String()
				}
			case big.Rat:
				rows[rowIndex].Fields[field] = resultsetAggregateFilteredWMathContextRatText(&typed)
			case *big.Rat:
				if typed == nil {
					rows[rowIndex].Fields[field] = nil
				} else {
					rows[rowIndex].Fields[field] = resultsetAggregateFilteredWMathContextRatText(typed)
				}
			}
		}
	}
	return rows
}

func resultsetAggregateFilteredWMathContextRatText(value *big.Rat) string {
	return bigRatExactDecimal(*value)
}

func rejectResultSetAggregateFilteredWMathContextDuplicateKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkResultSetAggregateFilteredWMathContextJSON(decoder); err != nil {
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

func walkResultSetAggregateFilteredWMathContextJSON(decoder *json.Decoder) error {
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
			if err := walkResultSetAggregateFilteredWMathContextJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("JSON object is not closed")
		}
	case '[':
		for decoder.More() {
			if err := walkResultSetAggregateFilteredWMathContextJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return fmt.Errorf("JSON array is not closed")
		}
	case ']', '}':
		return fmt.Errorf("unexpected JSON delimiter %q", token)
	}
	return nil
}

func decodeResultSetAggregateFilteredWMathContextJSONObject(raw []byte) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := strictResultSetAggregateFilteredWMathContextObject(raw, &object); err != nil {
		return nil, err
	}
	return object, nil
}

func strictResultSetAggregateFilteredWMathContextObject(raw []byte, target *map[string]json.RawMessage) error {
	if err := rejectResultSetAggregateFilteredWMathContextDuplicateKeys(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var object map[string]json.RawMessage
	if err := decoder.Decode(&object); err != nil || object == nil {
		return fmt.Errorf("JSON value must be an object")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("JSON value contains trailing data")
		}
		return err
	}
	if target != nil {
		*target = object
	}
	return nil
}

func decodeResultSetAggregateFilteredWMathContextJSONArray(raw json.RawMessage) ([]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var values []json.RawMessage
	if err := decoder.Decode(&values); err != nil || values == nil {
		return nil, fmt.Errorf("JSON value must be an array")
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

func requireResultSetAggregateFilteredWMathContextFields(object map[string]json.RawMessage, names ...string) error {
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

func validateResultSetAggregateFilteredWMathContextStringArray(raw json.RawMessage, expected []string, name string) error {
	values, err := decodeResultSetAggregateFilteredWMathContextJSONArray(raw)
	if err != nil || len(values) != len(expected) {
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
