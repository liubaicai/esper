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

const (
	resultsetOutputLimitMicrosecondID          = "resultset-output-limit-microsecond-resolution"
	resultsetOutputLimitMicrosecondDescription = "ResultSetOutputLimitMicrosecondResolution: whole-second and 0.1-second output-every schedules under externally advanced millisecond time; each case arms s0 after an initial advance, sends E1 and E2, and delivers exactly one new-only single-row full-event callback at each flip instant."
	resultsetOutputLimitMicrosecondJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOutputLimitMicrosecondSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitMicrosecondResolution.java"
)

var (
	resultsetOutputLimitMicrosecondJavaSources = []string{
		resultsetOutputLimitMicrosecondSource,
	}
	resultsetOutputLimitMicrosecondJavaRuntimeIDs = []string{
		"java-runtime-fb9601d7d5288b0b7ba5",
	}
	resultsetOutputLimitMicrosecondJavaExecutions = []string{
		"ResultSetOutputLimitMicrosecondResolution",
	}
	resultsetOutputLimitMicrosecondJavaStaticIDs = []string{
		"java-c68fd57c7e6ce42ed2d5",
	}
	resultsetOutputLimitMicrosecondCases = []string{
		"every-one-second",
		"fractional-100ms",
	}
	resultsetOutputLimitMicrosecondOrdinals = []int{0, 0}
	resultsetOutputLimitMicrosecondEPLs     = []string{
		"@name('s0') select * from SupportBean output every 1 seconds",
		"@name('s0') select * from SupportBean output every 0.1 seconds",
	}
)

type resultsetOutputLimitMicrosecondBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func loadResultsetOutputLimitMicrosecondScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOutputLimitMicrosecondID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOutputLimitMicrosecondID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitMicrosecondID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitMicrosecondID, err)
	}
	if err := requireResultsetOutputLimitMicrosecondFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOutputLimitMicrosecondID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetOutputLimitMicrosecondID ||
		metadata.Description != resultsetOutputLimitMicrosecondDescription ||
		metadata.JavaCommit != resultsetOutputLimitMicrosecondJavaCommit ||
		metadata.JavaSource != resultsetOutputLimitMicrosecondSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOutputLimitMicrosecondID)
	}
	if err := validateResultsetOutputLimitMicrosecondStringArray(root["javaRuntimes"], resultsetOutputLimitMicrosecondJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitMicrosecondStringArray(root["javaNames"], resultsetOutputLimitMicrosecondJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitMicrosecondStringArray(root["javaStaticIds"], resultsetOutputLimitMicrosecondJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitMicrosecondStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetOutputLimitMicrosecondCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly two cases", resultsetOutputLimitMicrosecondID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetOutputLimitMicrosecondFields(object,
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
		if definition.Case != resultsetOutputLimitMicrosecondCases[index] ||
			definition.Ordinal != resultsetOutputLimitMicrosecondOrdinals[index] ||
			definition.RuntimeID != resultsetOutputLimitMicrosecondJavaRuntimeIDs[0] ||
			definition.ExecutionName != resultsetOutputLimitMicrosecondJavaExecutions[0] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != resultsetOutputLimitMicrosecondEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetOutputLimitMicrosecondID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 16 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly sixteen steps", resultsetOutputLimitMicrosecondID)
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
			if err := requireResultsetOutputLimitMicrosecondFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireResultsetOutputLimitMicrosecondFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeResultsetOutputLimitMicrosecondPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		case "advance-time":
			if err := requireResultsetOutputLimitMicrosecondFields(object, "op", "at"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step compat.Step
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := time.Parse(time.RFC3339Nano, step.At); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d advance-time is not pinned", index)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateResultsetOutputLimitMicrosecondScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

type resultsetOutputLimitMicrosecondStepTiming struct {
	advanceAt string
	bean      resultsetOutputLimitMicrosecondBean
}

var resultsetOutputLimitMicrosecondOneSecondSteps = []resultsetOutputLimitMicrosecondStepTiming{
	{advanceAt: "1970-01-01T00:00:00Z"},
	{bean: resultsetOutputLimitMicrosecondBean{TheString: "E1", IntPrimitive: 10}},
	{advanceAt: "1970-01-01T00:00:00.999Z"},
	{advanceAt: "1970-01-01T00:00:01Z"},
	{bean: resultsetOutputLimitMicrosecondBean{TheString: "E2", IntPrimitive: 10}},
	{advanceAt: "1970-01-01T00:00:01.999Z"},
	{advanceAt: "1970-01-01T00:00:02Z"},
}

var resultsetOutputLimitMicrosecondFractionalSteps = []resultsetOutputLimitMicrosecondStepTiming{
	{advanceAt: "1995-01-03T08:57:36.789Z"},
	{bean: resultsetOutputLimitMicrosecondBean{TheString: "E1", IntPrimitive: 10}},
	{advanceAt: "1995-01-03T08:57:36.888Z"},
	{advanceAt: "1995-01-03T08:57:36.889Z"},
	{bean: resultsetOutputLimitMicrosecondBean{TheString: "E2", IntPrimitive: 10}},
	{advanceAt: "1995-01-03T08:57:36.988Z"},
	{advanceAt: "1995-01-03T08:57:36.989Z"},
}

func validateResultsetOutputLimitMicrosecondScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetOutputLimitMicrosecondID || len(scenario.Steps) != 16 {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetOutputLimitMicrosecondID)
	}
	index := 0
	if err := validateResultsetOutputLimitMicrosecondCaseMarker(scenario.Steps[index], resultsetOutputLimitMicrosecondCases[0]); err != nil {
		return fmt.Errorf("one-second marker: %w", err)
	}
	index++
	if err := validateResultsetOutputLimitMicrosecondTimingSteps(scenario.Steps[index:], resultsetOutputLimitMicrosecondOneSecondSteps, "one-second"); err != nil {
		return err
	}
	index += len(resultsetOutputLimitMicrosecondOneSecondSteps)
	if err := validateResultsetOutputLimitMicrosecondCaseMarker(scenario.Steps[index], resultsetOutputLimitMicrosecondCases[1]); err != nil {
		return fmt.Errorf("fractional marker: %w", err)
	}
	index++
	if err := validateResultsetOutputLimitMicrosecondTimingSteps(scenario.Steps[index:], resultsetOutputLimitMicrosecondFractionalSteps, "fractional"); err != nil {
		return err
	}
	index += len(resultsetOutputLimitMicrosecondFractionalSteps)
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetOutputLimitMicrosecondID)
	}
	return nil
}

func validateResultsetOutputLimitMicrosecondTimingSteps(steps []compat.Step, timings []resultsetOutputLimitMicrosecondStepTiming, label string) error {
	if len(steps) < len(timings) {
		return fmt.Errorf("%s timing steps truncated", label)
	}
	for offset, timing := range timings {
		step := steps[offset]
		if timing.advanceAt != "" {
			if step.Op != "advance-time" || step.Case != "" || step.At != timing.advanceAt {
				return fmt.Errorf("%s step %d must advance time to %q", label, offset, timing.advanceAt)
			}
			continue
		}
		if step.Op != "send" || step.Case != "" || step.EventType != "SupportBean" {
			return fmt.Errorf("%s step %d must send SupportBean", label, offset)
		}
		var fields map[string]json.RawMessage
		if err := strictObject(step.Payload, &fields); err != nil {
			return fmt.Errorf("%s step %d payload: %w", label, offset, err)
		}
		if err := requireResultsetOutputLimitMicrosecondFields(fields, "theString", "intPrimitive"); err != nil {
			return fmt.Errorf("%s step %d payload: %w", label, offset, err)
		}
		var bean resultsetOutputLimitMicrosecondBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return fmt.Errorf("%s step %d payload: %w", label, offset, err)
		}
		if bean != timing.bean {
			return fmt.Errorf("%s step %d payload is not pinned", label, offset)
		}
	}
	return nil
}

func validateResultsetOutputLimitMicrosecondCaseMarker(step compat.Step, expected string) error {
	if step.Op != "case" || step.Case != expected {
		return fmt.Errorf("must start case %q", expected)
	}
	return nil
}

func resultsetOutputLimitMicrosecondRuntimeID(caseName string) string {
	if caseName == "fractional-100ms" {
		return resultsetOutputLimitMicrosecondJavaRuntimeIDs[0]
	}
	return resultsetOutputLimitMicrosecondJavaRuntimeIDs[0]
}

func runResultsetOutputLimitMicrosecondScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetOutputLimitMicrosecondScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultsetOutputLimitMicrosecondCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultsetOutputLimitMicrosecondCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOutputLimitMicrosecondID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOutputLimitMicrosecondID, scenario.ID)
	}
	return trace, nil
}

func runResultsetOutputLimitMicrosecondCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOutputLimitMicrosecondBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	interval := time.Second
	if caseName == "fractional-100ms" {
		interval = 100 * time.Millisecond
	}
	start := time.Unix(0, 0).UTC()
	if caseName == "fractional-100ms" {
		start = time.Unix(789123456, 789000000).UTC()
	}
	query := esper.From[resultsetOutputLimitMicrosecondBean](env, "SupportBean").Query(
		esper.StatementName("s0"),
		esper.WithOutput(esper.OutputEveryTime(interval)),
	)
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(resultsetOutputLimitMicrosecondRuntimeID(caseName)),
		esper.WithStartTime(start),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("%s case %q deployed %d statements", resultsetOutputLimitMicrosecondID, caseName, len(statements))
	}
	return compat.ReplayWithStatements(ctx, engine, statements[0], scenario,
		decodeResultsetOutputLimitMicrosecondPayload,
		func(name string) (*esper.Statement, error) {
			if name != statements[0].Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetOutputLimitMicrosecondID, name)
			}
			return statements[0], nil
		})
}

func decodeResultsetOutputLimitMicrosecondPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var fields map[string]json.RawMessage
		if err := strictObject(step.Payload, &fields); err != nil {
			return nil, err
		}
		if err := requireResultsetOutputLimitMicrosecondFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var value resultsetOutputLimitMicrosecondBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOutputLimitMicrosecondID, step.EventType)
	}
}

func requireResultsetOutputLimitMicrosecondFields(object map[string]json.RawMessage, names ...string) error {
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

func validateResultsetOutputLimitMicrosecondStringArray(raw json.RawMessage, expected []string, name string) error {
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
