package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	outputAfterEventsID          = "output-after-events"
	outputAfterEventsDescription = "ResultSetOutputLimitAfter event-count after-gate: direct after-3-events delivery and the when-then variable side-effect form."
	outputAfterEventsJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	outputAfterEventsSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitAfter.java"
)

var (
	outputAfterEventsJavaSources = []string{
		outputAfterEventsSource,
	}
	outputAfterEventsJavaRuntimeIDs = []string{
		"java-runtime-34bfe3c4c56f505d50cd",
		"java-runtime-499038c2c0fca09551c1",
	}
	outputAfterEventsJavaExecutions = []string{
		"ResultSetDirectNumberOfEvents",
		"ResultSetOutputWhenThen",
	}
	outputAfterEventsJavaStaticIDs = []string{
		"java-aacc84310d1fab722e9f",
		"java-310737339ed16858a1fc",
	}
	outputAfterEventsCases = []string{
		"after-3-events",
		"after-3-events-when-then",
	}
	outputAfterEventsOrdinals = []int{3, 6}
	outputAfterEventsEPLs     = []string{
		"@name('s0') select theString from SupportBean#keepall output after 3 events",
		"@Name('s0') select a.* from SupportBean#time(10) a output after 3 events when myvar0=true then set myvar1=true, myvar2=true",
	}
)

type outputAfterEventsBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

var outputAfterEventsFirstBatch = []outputAfterEventsBean{
	{TheString: "E1", IntPrimitive: 0},
	{TheString: "E2", IntPrimitive: 0},
	{TheString: "E3", IntPrimitive: 0},
	{TheString: "E4", IntPrimitive: 0},
	{TheString: "E5", IntPrimitive: 0},
}
var outputAfterEventsWhenBatch = []outputAfterEventsBean{
	{TheString: "E1", IntPrimitive: 0},
	{TheString: "E2", IntPrimitive: 0},
	{TheString: "E3", IntPrimitive: 0},
}

func loadOutputAfterEventsScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", outputAfterEventsID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", outputAfterEventsID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", outputAfterEventsID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", outputAfterEventsID, err)
	}
	if err := requireOutputAfterEventsFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", outputAfterEventsID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != outputAfterEventsID ||
		metadata.Description != outputAfterEventsDescription ||
		metadata.JavaCommit != outputAfterEventsJavaCommit ||
		metadata.JavaSource != outputAfterEventsSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", outputAfterEventsID)
	}
	if err := validateOutputAfterEventsStringArray(root["javaRuntimes"], outputAfterEventsJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateOutputAfterEventsStringArray(root["javaNames"], outputAfterEventsJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateOutputAfterEventsStringArray(root["javaStaticIds"], outputAfterEventsJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateOutputAfterEventsStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(outputAfterEventsCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly two cases", outputAfterEventsID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireOutputAfterEventsFields(object,
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
		if definition.Case != outputAfterEventsCases[index] ||
			definition.Ordinal != outputAfterEventsOrdinals[index] ||
			definition.RuntimeID != outputAfterEventsJavaRuntimeIDs[index] ||
			definition.ExecutionName != outputAfterEventsJavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != outputAfterEventsEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", outputAfterEventsID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 14 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly fourteen steps", outputAfterEventsID)
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
			if err := requireOutputAfterEventsFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireOutputAfterEventsFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeOutputAfterEventsPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		case "set-variable":
			if err := requireOutputAfterEventsFields(object, "op", "name", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step compat.Step
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Name != "myvar0" || !bytes.Equal(bytes.TrimSpace(step.Payload), []byte("true")) {
				return compat.Scenario{}, fmt.Errorf("scenario step %d set-variable is not pinned", index)
			}
		case "read-variable":
			if err := requireOutputAfterEventsFields(object, "op", "name"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step compat.Step
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Name != "myvar1" && step.Name != "myvar2" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d read-variable is not pinned", index)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateOutputAfterEventsScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateOutputAfterEventsScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != outputAfterEventsID || len(scenario.Steps) != 14 {
		return fmt.Errorf("%s scenario shape is not pinned", outputAfterEventsID)
	}
	index := 0
	if err := validateOutputAfterEventsCaseMarker(scenario.Steps[index], outputAfterEventsCases[0]); err != nil {
		return fmt.Errorf("case marker: %w", err)
	}
	index++
	for eventIndex, expected := range outputAfterEventsFirstBatch {
		if err := validateOutputAfterEventsBeanStep(scenario.Steps[index], expected); err != nil {
			return fmt.Errorf("event step %d: %w", eventIndex, err)
		}
		index++
	}
	if err := validateOutputAfterEventsCaseMarker(scenario.Steps[index], outputAfterEventsCases[1]); err != nil {
		return fmt.Errorf("when-then marker: %w", err)
	}
	index++
	for eventIndex, expected := range outputAfterEventsWhenBatch {
		if err := validateOutputAfterEventsBeanStep(scenario.Steps[index], expected); err != nil {
			return fmt.Errorf("when-then event step %d: %w", eventIndex, err)
		}
		index++
	}
	if err := validateOutputAfterEventsSetVariableStep(scenario.Steps[index]); err != nil {
		return fmt.Errorf("when-then set-variable: %w", err)
	}
	index++
	if err := validateOutputAfterEventsBeanStep(scenario.Steps[index], outputAfterEventsBean{TheString: "E4", IntPrimitive: 0}); err != nil {
		return fmt.Errorf("when-then final event: %w", err)
	}
	index++
	if err := validateOutputAfterEventsReadVariableStep(scenario.Steps[index], "myvar1"); err != nil {
		return fmt.Errorf("when-then first read: %w", err)
	}
	index++
	if err := validateOutputAfterEventsReadVariableStep(scenario.Steps[index], "myvar2"); err != nil {
		return fmt.Errorf("when-then second read: %w", err)
	}
	index++
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", outputAfterEventsID)
	}
	return nil
}

func validateOutputAfterEventsCaseMarker(step compat.Step, expected string) error {
	if step.Op != "case" || step.Case != expected {
		return fmt.Errorf("must start case %q", expected)
	}
	return nil
}

func validateOutputAfterEventsBeanStep(step compat.Step, expected outputAfterEventsBean) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportBean" {
		return fmt.Errorf("must send SupportBean")
	}
	bean, err := decodeOutputAfterEventsPayload(step)
	if err != nil {
		return err
	}
	if bean != expected {
		return fmt.Errorf("bean payload is not pinned")
	}
	return nil
}

func validateOutputAfterEventsSetVariableStep(step compat.Step) error {
	if step.Op != "set-variable" || step.Case != "" || step.Name != "myvar0" {
		return fmt.Errorf("must set variable myvar0")
	}
	var value bool
	if err := json.Unmarshal(step.Payload, &value); err != nil || !value {
		return fmt.Errorf("set-variable payload is not pinned")
	}
	return nil
}

func validateOutputAfterEventsReadVariableStep(step compat.Step, expected string) error {
	if step.Op != "read-variable" || step.Case != "" || step.Name != expected {
		return fmt.Errorf("must read variable %q", expected)
	}
	return nil
}

func runOutputAfterEventsScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateOutputAfterEventsScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range outputAfterEventsCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runOutputAfterEventsCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", outputAfterEventsID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", outputAfterEventsID, scenario.ID)
	}
	return trace, nil
}

func runOutputAfterEventsCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[outputAfterEventsBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	boolType := reflect.TypeOf(false)
	for _, name := range []string{"myvar0", "myvar1", "myvar2"} {
		if err := env.RegisterVariable(name, false, esper.VariableType(boolType)); err != nil {
			return compat.Trace{}, err
		}
	}

	theString := esper.Field[outputAfterEventsBean, string]("theString")
	var query esper.Query
	switch caseName {
	case "after-3-events":
		kept := esper.From[outputAfterEventsBean](env, "SupportBean").Window(esper.KeepAll())
		query = esper.Select(kept,
			esper.Alias("theString", theString),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAfterEvents(3)),
		)
	case "after-3-events-when-then":
		gated := esper.From[outputAfterEventsBean](env, "SupportBean").
			Window(esper.TimeWindow(10 * time.Second))
		query = esper.Select(gated).
			Query(
				esper.StatementName("s0"),
				esper.WithOutput(esper.OutputAfterEvents(3, esper.OutputWhen(
					esper.Equal[bool](esper.VariableRef[bool]("myvar0"), esper.Literal(true)),
					esper.SetOutputVariable("myvar1", esper.Literal(true)),
					esper.SetOutputVariable("myvar2", esper.Literal(true)),
				))),
			)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported %s case %q", outputAfterEventsID, caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithRuntimeURI(outputAfterEventsRuntimeID(caseName)))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("%s case %q deployed %d statements", outputAfterEventsID, caseName, len(statements))
	}
	statement := statements[0]
	if caseName == "after-3-events-when-then" {
		handlers := map[string]compat.StepHandler{}
		handlers["set-variable"] = func(step compat.Step, _ func(*esper.Statement) error) ([]compat.TraceRecord, error) {
			var value bool
			if err := json.Unmarshal(step.Payload, &value); err != nil {
				return nil, fmt.Errorf("decode set-variable value: %w", err)
			}
			return nil, engine.SetVariable(ctx, step.Name, value)
		}
		handlers["read-variable"] = func(step compat.Step, _ func(*esper.Statement) error) ([]compat.TraceRecord, error) {
			value, ok := engine.GetVariable(step.Name)
			if !ok {
				return nil, fmt.Errorf("variable %q not found", step.Name)
			}
			record := compat.TraceRecord{
				Case:      "after-3-events-when-then",
				Operation: "variable",
				Name:      step.Name,
				Value:     value.Any(),
			}
			return []compat.TraceRecord{record}, nil
		}
		return compat.ReplayWithStatementsAndHandlers(ctx, engine, statement, scenario, decodeOutputAfterEventsPayload,
			func(name string) (*esper.Statement, error) {
				if name != statement.Name() {
					return nil, fmt.Errorf("unknown %s statement %q", outputAfterEventsID, name)
				}
				return statement, nil
			}, handlers)
	}
	return compat.ReplayWithStatements(ctx, engine, statement, scenario, decodeOutputAfterEventsPayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", outputAfterEventsID, name)
			}
			return statement, nil
		})
}

func decodeOutputAfterEventsPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var fields map[string]json.RawMessage
		if err := strictObject(step.Payload, &fields); err != nil {
			return nil, err
		}
		if err := requireOutputAfterEventsFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var value outputAfterEventsBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", outputAfterEventsID, step.EventType)
	}
}

func requireOutputAfterEventsFields(object map[string]json.RawMessage, names ...string) error {
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

func validateOutputAfterEventsStringArray(raw json.RawMessage, expected []string, name string) error {
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

func outputAfterEventsRuntimeID(caseName string) string {
	if caseName == "after-3-events-when-then" {
		return outputAfterEventsJavaRuntimeIDs[1]
	}
	return outputAfterEventsJavaRuntimeIDs[0]
}
