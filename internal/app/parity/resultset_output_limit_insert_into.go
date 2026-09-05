package parity

import (
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
	resultsetOutputLimitInsertIntoID          = "resultset-output-limit-insert-into"
	resultsetOutputLimitInsertIntoDescription = "ResultSetOutputLimitInsertInto: output-limited insert-into producers observed through both the producing statement and the routed target stream."
	resultsetOutputLimitInsertIntoJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOutputLimitInsertIntoSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitInsertInto.java"
)

var (
	resultsetOutputLimitInsertIntoJavaSources = []string{
		resultsetOutputLimitInsertIntoSource,
	}
	resultsetOutputLimitInsertIntoJavaRuntimeIDs = []string{
		"java-runtime-e57a7555b3a0303c0303",
		"java-runtime-cd524991d69bc898c061",
	}
	resultsetOutputLimitInsertIntoJavaExecutions = []string{
		"ResultSetOutputLimitInsertFirst",
		"ResultSetOutputLimitInsertSnapshot",
	}
	resultsetOutputLimitInsertIntoJavaStaticIDs = []string{
		"java-00bb626878ef4282b5d2",
		"java-460f4166b6f5e1099aae",
	}
	resultsetOutputLimitInsertIntoCases = []string{
		"insert-first",
		"insert-snapshot",
	}
	resultsetOutputLimitInsertIntoOrdinals = []int{0, 1}
	resultsetOutputLimitInsertIntoEPLs     = []string{
		"@name('s0') insert into MyStream select * from SupportBean output first every 1 second;@name('s1') select * from MyStream",
		"@name('s0') insert into MyStream select * from SupportBean#keepall output snapshot every 1 second;@name('s1') select * from MyStream",
	}
)

type resultsetOutputLimitInsertIntoBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

var resultsetOutputLimitInsertIntoFirstSends = []resultsetOutputLimitInsertIntoBean{
	{TheString: "E1", IntPrimitive: 0},
	{TheString: "E2", IntPrimitive: 0},
	{TheString: "E2", IntPrimitive: 0},
}
var resultsetOutputLimitInsertIntoSnapshotSends = []resultsetOutputLimitInsertIntoBean{
	{TheString: "E1", IntPrimitive: 0},
	{TheString: "E2", IntPrimitive: 0},
}

func loadResultsetOutputLimitInsertIntoScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOutputLimitInsertIntoID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOutputLimitInsertIntoID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitInsertIntoID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitInsertIntoID, err)
	}
	if err := requireResultsetOutputLimitInsertIntoFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOutputLimitInsertIntoID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetOutputLimitInsertIntoID ||
		metadata.Description != resultsetOutputLimitInsertIntoDescription ||
		metadata.JavaCommit != resultsetOutputLimitInsertIntoJavaCommit ||
		metadata.JavaSource != resultsetOutputLimitInsertIntoSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOutputLimitInsertIntoID)
	}
	if err := validateResultsetOutputLimitInsertIntoStringArray(root["javaRuntimes"], resultsetOutputLimitInsertIntoJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitInsertIntoStringArray(root["javaNames"], resultsetOutputLimitInsertIntoJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitInsertIntoStringArray(root["javaStaticIds"], resultsetOutputLimitInsertIntoJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitInsertIntoStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetOutputLimitInsertIntoCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly two cases", resultsetOutputLimitInsertIntoID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetOutputLimitInsertIntoFields(object,
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
		if definition.Case != resultsetOutputLimitInsertIntoCases[index] ||
			definition.Ordinal != resultsetOutputLimitInsertIntoOrdinals[index] ||
			definition.RuntimeID != resultsetOutputLimitInsertIntoRuntimeID(resultsetOutputLimitInsertIntoCases[index]) ||
			definition.ExecutionName != resultsetOutputLimitInsertIntoExecutionName(resultsetOutputLimitInsertIntoCases[index]) ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != resultsetOutputLimitInsertIntoEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetOutputLimitInsertIntoID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 10 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly ten steps", resultsetOutputLimitInsertIntoID)
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
			if err := requireResultsetOutputLimitInsertIntoFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireResultsetOutputLimitInsertIntoFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeResultsetOutputLimitInsertIntoPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		case "advance-time":
			if err := requireResultsetOutputLimitInsertIntoFields(object, "op", "at"); err != nil {
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
	if err := validateResultsetOutputLimitInsertIntoScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetOutputLimitInsertIntoScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetOutputLimitInsertIntoID || len(scenario.Steps) != 10 {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetOutputLimitInsertIntoID)
	}
	index := 0
	if err := validateResultsetOutputLimitInsertIntoCaseMarker(scenario.Steps[index], resultsetOutputLimitInsertIntoCases[0]); err != nil {
		return fmt.Errorf("insert-first marker: %w", err)
	}
	index++
	for _, stepTiming := range resultsetOutputLimitInsertIntoFirstSteps {
		if stepTiming.advanceAt != "" {
			if err := validateResultsetOutputLimitInsertIntoAdvanceStep(scenario.Steps[index], stepTiming.advanceAt); err != nil {
				return fmt.Errorf("insert-first advance: %w", err)
			}
			index++
			continue
		}
		if err := validateResultsetOutputLimitInsertIntoBeanStep(scenario.Steps[index], stepTiming.bean); err != nil {
			return fmt.Errorf("insert-first event: %w", err)
		}
		index++
	}
	if err := validateResultsetOutputLimitInsertIntoCaseMarker(scenario.Steps[index], resultsetOutputLimitInsertIntoCases[1]); err != nil {
		return fmt.Errorf("insert-snapshot marker: %w", err)
	}
	index++
	for _, stepTiming := range resultsetOutputLimitInsertIntoSnapshotSteps {
		if stepTiming.advanceAt != "" {
			if err := validateResultsetOutputLimitInsertIntoAdvanceStep(scenario.Steps[index], stepTiming.advanceAt); err != nil {
				return fmt.Errorf("insert-snapshot advance: %w", err)
			}
			index++
			continue
		}
		if err := validateResultsetOutputLimitInsertIntoBeanStep(scenario.Steps[index], stepTiming.bean); err != nil {
			return fmt.Errorf("insert-snapshot event: %w", err)
		}
		index++
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetOutputLimitInsertIntoID)
	}
	return nil
}

type resultsetOutputLimitInsertIntoStepTiming struct {
	advanceAt string
	bean      resultsetOutputLimitInsertIntoBean
}

var resultsetOutputLimitInsertIntoFirstSteps = []resultsetOutputLimitInsertIntoStepTiming{
	{bean: resultsetOutputLimitInsertIntoBean{TheString: "E1", IntPrimitive: 0}},
	{bean: resultsetOutputLimitInsertIntoBean{TheString: "E2", IntPrimitive: 0}},
	{advanceAt: "1970-01-01T00:00:01Z"},
	{bean: resultsetOutputLimitInsertIntoBean{TheString: "E2", IntPrimitive: 0}},
}

var resultsetOutputLimitInsertIntoSnapshotSteps = []resultsetOutputLimitInsertIntoStepTiming{
	{bean: resultsetOutputLimitInsertIntoBean{TheString: "E1", IntPrimitive: 0}},
	{advanceAt: "1970-01-01T00:00:01Z"},
	{bean: resultsetOutputLimitInsertIntoBean{TheString: "E2", IntPrimitive: 0}},
	{advanceAt: "1970-01-01T00:00:02Z"},
}

func validateResultsetOutputLimitInsertIntoAdvanceStep(step compat.Step, expected string) error {
	if step.Op != "advance-time" || step.Case != "" || step.At != expected {
		return fmt.Errorf("must advance time to %q", expected)
	}
	return nil
}

func validateResultsetOutputLimitInsertIntoCaseMarker(step compat.Step, expected string) error {
	if step.Op != "case" || step.Case != expected {
		return fmt.Errorf("must start case %q", expected)
	}
	return nil
}

func validateResultsetOutputLimitInsertIntoBeanStep(step compat.Step, expected resultsetOutputLimitInsertIntoBean) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportBean" {
		return fmt.Errorf("must send SupportBean")
	}
	bean, err := decodeResultsetOutputLimitInsertIntoBean(step)
	if err != nil {
		return err
	}
	if bean != expected {
		return fmt.Errorf("bean payload is not pinned")
	}
	return nil
}

func runResultsetOutputLimitInsertIntoScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetOutputLimitInsertIntoScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultsetOutputLimitInsertIntoCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultsetOutputLimitInsertIntoCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOutputLimitInsertIntoID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOutputLimitInsertIntoID, scenario.ID)
	}
	return trace, nil
}

func runResultsetOutputLimitInsertIntoCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOutputLimitInsertIntoBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterMap(env, "MyStream", []esper.FieldSpec{
		esper.FieldDef("theString", reflect.TypeOf("")),
		esper.FieldDef("intPrimitive", reflect.TypeOf(int(0))),
	}); err != nil {
		return compat.Trace{}, err
	}

	producerStream := esper.From[resultsetOutputLimitInsertIntoBean](env, "SupportBean")
	var producer esper.Query
	switch caseName {
	case "insert-first":
		producer = producerStream.InsertInto("MyStream",
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputFirstEveryTime(time.Second)),
		)
	case "insert-snapshot":
		producer = producerStream.Window(esper.KeepAll()).InsertInto("MyStream",
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputSnapshotEvery(time.Second)),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported %s case %q", resultsetOutputLimitInsertIntoID, caseName)
	}
	consumer := esper.FromAny(env, "MyStream").Query(esper.StatementName("s1"))
	producerPlan, err := env.Build(producer)
	if err != nil {
		return compat.Trace{}, err
	}
	consumerPlan, err := env.Build(consumer)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithRuntimeURI(resultsetOutputLimitInsertIntoRuntimeID(caseName)))
	defer func() { _ = engine.Close(context.Background()) }()
	producerDeployment, err := engine.Deploy(ctx, producerPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := engine.Deploy(ctx, consumerPlan); err != nil {
		return compat.Trace{}, err
	}
	producerStatement := findInsertIntoStatement(producerDeployment, "s0", caseName)
	consumerStatement := findInsertIntoStatementByDeploys(engine, "s1", caseName)
	return compat.ReplayWithStatements(ctx, engine, producerStatement, scenario,
		decodeResultsetOutputLimitInsertIntoPayload,
		func(name string) (*esper.Statement, error) {
			switch name {
			case producerStatement.Name():
				return producerStatement, nil
			case consumerStatement.Name():
				return consumerStatement, nil
			}
			return nil, fmt.Errorf("unknown %s statement %q", resultsetOutputLimitInsertIntoID, name)
		}, consumerStatement)
}

func findInsertIntoStatement(deployment *esper.Deployment, name, caseName string) *esper.Statement {
	for _, statement := range deployment.Statements() {
		if statement.Name() == name {
			return statement
		}
	}
	panic(fmt.Sprintf("%s case %q did not deploy statement %q", resultsetOutputLimitInsertIntoID, caseName, name))
}

func findInsertIntoStatementByDeploys(engine *esper.Engine, name, caseName string) *esper.Statement {
	for _, deployment := range engine.Deployments() {
		for _, statement := range deployment.Statements() {
			if statement.Name() == name {
				return statement
			}
		}
	}
	panic(fmt.Sprintf("%s case %q did not deploy consumer statement %q", resultsetOutputLimitInsertIntoID, caseName, name))
}

func decodeResultsetOutputLimitInsertIntoPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var fields map[string]json.RawMessage
		if err := strictObject(step.Payload, &fields); err != nil {
			return nil, err
		}
		if err := requireResultsetOutputLimitInsertIntoFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var value resultsetOutputLimitInsertIntoBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOutputLimitInsertIntoID, step.EventType)
	}
}

func requireResultsetOutputLimitInsertIntoFields(object map[string]json.RawMessage, names ...string) error {
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

func validateResultsetOutputLimitInsertIntoStringArray(raw json.RawMessage, expected []string, name string) error {
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

func resultsetOutputLimitInsertIntoRuntimeID(caseName string) string {
	if caseName == "insert-first" {
		return resultsetOutputLimitInsertIntoJavaRuntimeIDs[0]
	}
	return resultsetOutputLimitInsertIntoJavaRuntimeIDs[1]
}

func resultsetOutputLimitInsertIntoExecutionName(caseName string) string {
	if caseName == "insert-first" {
		return resultsetOutputLimitInsertIntoJavaExecutions[0]
	}
	return resultsetOutputLimitInsertIntoJavaExecutions[1]
}

func decodeResultsetOutputLimitInsertIntoBean(step compat.Step) (resultsetOutputLimitInsertIntoBean, error) {
	if step.EventType != "SupportBean" {
		return resultsetOutputLimitInsertIntoBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOutputLimitInsertIntoBean{}, err
	}
	if err := requireResultsetOutputLimitInsertIntoFields(fields, "theString", "intPrimitive"); err != nil {
		return resultsetOutputLimitInsertIntoBean{}, err
	}
	var value resultsetOutputLimitInsertIntoBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return resultsetOutputLimitInsertIntoBean{}, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
