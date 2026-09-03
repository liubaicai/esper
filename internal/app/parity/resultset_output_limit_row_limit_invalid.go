package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultsetOutputLimitRowLimitInvalidID          = "resultset-output-limit-row-limit-invalid"
	resultsetOutputLimitRowLimitInvalidJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOutputLimitRowLimitInvalidDescription = "ResultSetOutputLimitRowLimit ordinal 8: invalid variable row-limit compile diagnostics."
	resultsetOutputLimitRowLimitInvalidSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowLimit.java"
	resultsetOutputLimitRowLimitInvalidRuntimeID   = "java-runtime-1d2703c5e4976fa0dd90"
	resultsetOutputLimitRowLimitInvalidStaticID    = "java-b2d63277ca12f283120d"
	resultsetOutputLimitRowLimitInvalidExecution   = "ResultSetInvalid"
	resultsetOutputLimitRowLimitInvalidCase        = "invalid-variable-row-limit"
)

var (
	resultsetOutputLimitRowLimitInvalidJavaRuntimeIDs = []string{resultsetOutputLimitRowLimitInvalidRuntimeID}
	resultsetOutputLimitRowLimitInvalidJavaStaticIDs  = []string{resultsetOutputLimitRowLimitInvalidStaticID}
	resultsetOutputLimitRowLimitInvalidJavaSources    = []string{resultsetOutputLimitRowLimitInvalidSource}
	resultsetOutputLimitRowLimitInvalidJavaExecutions = []string{resultsetOutputLimitRowLimitInvalidExecution}
)

var resultsetOutputLimitRowLimitInvalidProbes = []struct {
	statement   string
	epl         string
	expectError string
}{
	{"limit-myrows", "select * from SupportBean limit myrows", "Limit clause requires a variable of numeric type [select * from SupportBean limit myrows]"},
	{"offset-myrows", "select * from SupportBean limit 1, myrows", "Limit clause requires a variable of numeric type [select * from SupportBean limit 1, myrows]"},
	{"limit-dummy", "select * from SupportBean limit dummy", "Limit clause variable by name 'dummy' has not been declared [select * from SupportBean limit dummy]"},
	{"offset-dummy", "select * from SupportBean limit 1,dummy", "Limit clause variable by name 'dummy' has not been declared [select * from SupportBean limit 1,dummy]"},
}

type resultsetOutputLimitRowLimitInvalidSupportBean struct {
	TheString string `esper:"theString"`
}

func loadResultsetOutputLimitRowLimitInvalidScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOutputLimitRowLimitInvalidID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, err
	}
	if err := rejectResultsetOutputLimitRowLimitInvalidDuplicateKeys(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitRowLimitInvalidID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, err
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if err := requireResultsetOutputLimitRowLimitInvalidFields(root, required...); err != nil {
		return compat.Scenario{}, err
	}
	var meta struct {
		Version     string `json:"version"`
		ID          string `json:"id"`
		Description string `json:"description"`
		JavaCommit  string `json:"javaCommit"`
		JavaSource  string `json:"javaSource"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return compat.Scenario{}, err
	}
	if meta.Version != compat.ScenarioVersion || meta.ID != resultsetOutputLimitRowLimitInvalidID || meta.Description != resultsetOutputLimitRowLimitInvalidDescription || meta.JavaCommit != resultsetOutputLimitRowLimitInvalidJavaCommit || meta.JavaSource != resultsetOutputLimitRowLimitInvalidSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOutputLimitRowLimitInvalidID)
	}
	if err := validateResultsetOutputLimitRowLimitInvalidStringArray(root["javaRuntimes"], resultsetOutputLimitRowLimitInvalidJavaRuntimeIDs); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitRowLimitInvalidStringArray(root["javaNames"], resultsetOutputLimitRowLimitInvalidJavaExecutions); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitRowLimitInvalidStringArray(root["javaStaticIds"], resultsetOutputLimitRowLimitInvalidJavaStaticIDs); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitRowLimitInvalidStringArray(root["javaFlags"], []string{}); err != nil {
		return compat.Scenario{}, err
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != 1 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly one case", resultsetOutputLimitRowLimitInvalidID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetOutputLimitRowLimitInvalidFields(object,
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
		if err := json.Unmarshal(rawCase, &definition); err != nil || definition.Case != resultsetOutputLimitRowLimitInvalidCase || definition.Ordinal != 8 || definition.RuntimeID != resultsetOutputLimitRowLimitInvalidRuntimeID || definition.ExecutionName != resultsetOutputLimitRowLimitInvalidExecution || definition.Observation != "compile-only" || definition.IteratorSnapshots != 0 || definition.EPL != resultsetOutputLimitRowLimitInvalidProbes[0].epl {
			return compat.Scenario{}, fmt.Errorf("%s case metadata is not pinned", resultsetOutputLimitRowLimitInvalidID)
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 5 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly five steps", resultsetOutputLimitRowLimitInvalidID)
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
		var expected []string
		switch operation {
		case "case":
			expected = []string{"op", "case"}
		case "build-error":
			expected = []string{"op", "statement", "epl", "expectError"}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := requireResultsetOutputLimitRowLimitInvalidFields(object, expected...); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: meta.Version, ID: meta.ID, Steps: steps}
	if err := validateResultsetOutputLimitRowLimitInvalidScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetOutputLimitRowLimitInvalidScenario(s compat.Scenario) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if s.ID != resultsetOutputLimitRowLimitInvalidID || len(s.Steps) != 5 || s.Steps[0].Op != "case" || s.Steps[0].Case != resultsetOutputLimitRowLimitInvalidCase {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetOutputLimitRowLimitInvalidID)
	}
	for i, probe := range resultsetOutputLimitRowLimitInvalidProbes {
		step := s.Steps[i+1]
		if step.Op != "build-error" || step.Statement != probe.statement || step.Epl != probe.epl || step.ExpectError != probe.expectError {
			return fmt.Errorf("%s build-error probe %d is not pinned", resultsetOutputLimitRowLimitInvalidID, i)
		}
	}
	return nil
}

func runResultsetOutputLimitRowLimitInvalidScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetOutputLimitRowLimitInvalidScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	if err := ctx.Err(); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if err := env.RegisterVariable("myrows", "abc"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetOutputLimitRowLimitInvalidSupportBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for i, probe := range resultsetOutputLimitRowLimitInvalidProbes {
		input := esper.From[resultsetOutputLimitRowLimitInvalidSupportBean](env, "SupportBean")
		var query esper.Query
		switch i {
		case 0:
			query = input.Query(esper.LimitExpression(esper.VariableRef[string]("myrows")))
		case 1:
			query = input.Query(esper.LimitExpression(esper.Literal(1)), esper.OffsetExpression(esper.VariableRef[string]("myrows")))
		case 2:
			query = input.Query(esper.LimitExpression(esper.VariableRef[string]("dummy")))
		case 3:
			query = input.Query(esper.LimitExpression(esper.Literal(1)), esper.OffsetExpression(esper.VariableRef[string]("dummy")))
		}
		_, buildErr := env.Build(query)
		record := compat.TraceRecord{Case: resultsetOutputLimitRowLimitInvalidCase, Operation: "compile-rejected", Statement: probe.statement, Sequence: uint64(i + 1), Time: "1970-01-01T00:00:00Z"}
		if buildErr == nil {
			record.Value = "<no-error>"
		} else {
			record.Value = resultsetOutputLimitRowLimitInvalidDiagnostic(buildErr, probe.epl)
		}
		actual, ok := record.Value.(string)
		if !ok || actual != probe.expectError {
			return trace, fmt.Errorf("%s diagnostic drift for %q: expected %q got %q", resultsetOutputLimitRowLimitInvalidID, probe.statement, probe.expectError, record.Value)
		}
		trace.Records = append(trace.Records, record)
	}
	return trace, nil
}

func resultsetOutputLimitRowLimitInvalidDiagnostic(err error, epl string) string {
	message := resultsetOutputLimitRowLimitInvalidBareMessage(err)
	if message == "" {
		return "<no-error>"
	}
	return message + " [" + epl + "]"
}
func resultsetOutputLimitRowLimitInvalidBareMessage(err error) string {
	message := ""
	for current := err; current != nil; current = errors.Unwrap(current) {
		var typed *esper.Error
		if errors.As(current, &typed) && typed.Message != "" {
			message = typed.Message
		}
	}
	if message == "" && err != nil {
		message = err.Error()
	}
	return message
}
func requireResultsetOutputLimitRowLimitInvalidFields(object map[string]json.RawMessage, expected ...string) error {
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
func validateResultsetOutputLimitRowLimitInvalidStringArray(raw json.RawMessage, expected []string) error {
	var actual []string
	if err := json.Unmarshal(raw, &actual); err != nil || actual == nil || len(actual) != len(expected) {
		return fmt.Errorf("invalid metadata array")
	}
	for i := range expected {
		if actual[i] != expected[i] {
			return fmt.Errorf("invalid metadata array")
		}
	}
	return nil
}
func rejectResultsetOutputLimitRowLimitInvalidDuplicateKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkResultsetOutputLimitRowLimitInvalidJSON(decoder); err != nil {
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
func walkResultsetOutputLimitRowLimitInvalidJSON(decoder *json.Decoder) error {
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
		seen := map[string]struct{}{}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok {
				return fmt.Errorf("JSON object key is not a string")
			}
			if _, exists := seen[name]; exists {
				return fmt.Errorf("duplicate JSON field %q", name)
			}
			seen[name] = struct{}{}
			if err := walkResultsetOutputLimitRowLimitInvalidJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("JSON object is not closed")
		}
	case '[':
		for decoder.More() {
			if err := walkResultsetOutputLimitRowLimitInvalidJSON(decoder); err != nil {
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
