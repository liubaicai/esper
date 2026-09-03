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
	resultsetOutputLimitRowLimitContextGroupedID              = "resultset-output-limit-row-limit-context-grouped"
	resultsetOutputLimitRowLimitContextGroupedJavaCommit      = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOutputLimitRowLimitContextGroupedDescription     = "ResultSetOutputLimitRowLimit ordinals 0 and 4: order-optimized limit-one batch/context termination and fully grouped ordered aggregate iterator behavior."
	resultsetOutputLimitRowLimitContextGroupedSource          = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowLimit.java"
	resultsetOutputLimitRowLimitContextGroupedBatchEPL        = "@name('s0') select theString from SupportBean#length_batch(10) order by theString limit 1"
	resultsetOutputLimitRowLimitContextGroupedFullyGroupedEPL = "@name('s0') select theString, sum(intPrimitive) as mysum from SupportBean#length(5) group by theString order by sum(intPrimitive) limit 2"
	resultsetOutputLimitRowLimitContextGroupedLimitCase       = "limit-one-order-optimization"
	resultsetOutputLimitRowLimitContextGroupedGroupedCase     = "fully-grouped-ordered"
)

var (
	resultsetOutputLimitRowLimitContextGroupedJavaRuntimeIDs = []string{
		"java-runtime-0a4f187046b734b5dc9a",
		"java-runtime-19d52dc587ea246a3e07",
	}
	resultsetOutputLimitRowLimitContextGroupedJavaStaticIDs = []string{
		"java-3f5845fdac39997e9b3d",
		"java-bf49a03f52cc732a2cf6",
	}
	resultsetOutputLimitRowLimitContextGroupedJavaSources = []string{
		resultsetOutputLimitRowLimitContextGroupedSource,
	}
	resultsetOutputLimitRowLimitContextGroupedJavaExecutions = []string{
		"ResultSetLimitOneWithOrderOptimization",
		"ResultSetFullyGroupedOrdered",
	}
	resultsetOutputLimitRowLimitContextGroupedCases = []string{
		resultsetOutputLimitRowLimitContextGroupedLimitCase,
		resultsetOutputLimitRowLimitContextGroupedGroupedCase,
	}
	resultsetOutputLimitRowLimitContextGroupedOrdinals = []int{0, 4}
)

type resultsetOutputLimitRowLimitContextGroupedBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type resultsetOutputLimitRowLimitContextGroupedBoundary struct {
	ID int `esper:"id"`
}

func loadResultsetOutputLimitRowLimitContextGroupedScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOutputLimitRowLimitContextGroupedID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOutputLimitRowLimitContextGroupedID, err)
	}
	if err := rejectResultsetOutputLimitRowLimitContextGroupedJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitRowLimitContextGroupedID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitRowLimitContextGroupedID, err)
	}
	if err := requireResultsetOutputLimitRowLimitContextGroupedFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOutputLimitRowLimitContextGroupedID, err)
	}
	if metadata.Version != compat.ScenarioVersion ||
		metadata.ID != resultsetOutputLimitRowLimitContextGroupedID ||
		metadata.Description != resultsetOutputLimitRowLimitContextGroupedDescription ||
		metadata.JavaCommit != resultsetOutputLimitRowLimitContextGroupedJavaCommit ||
		metadata.JavaSource != resultsetOutputLimitRowLimitContextGroupedSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOutputLimitRowLimitContextGroupedID)
	}
	if err := validateResultsetOutputLimitRowLimitContextGroupedStringArray(root["javaRuntimes"], resultsetOutputLimitRowLimitContextGroupedJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitRowLimitContextGroupedStringArray(root["javaNames"], resultsetOutputLimitRowLimitContextGroupedJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitRowLimitContextGroupedStringArray(root["javaStaticIds"], resultsetOutputLimitRowLimitContextGroupedJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitRowLimitContextGroupedStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetOutputLimitRowLimitContextGroupedCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly two cases", resultsetOutputLimitRowLimitContextGroupedID)
	}
	observations := []string{"listener+iterator", "iterator"}
	iteratorSnapshots := []int{12, 6}
	epls := []string{resultsetOutputLimitRowLimitContextGroupedBatchEPL, resultsetOutputLimitRowLimitContextGroupedFullyGroupedEPL}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetOutputLimitRowLimitContextGroupedFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var entry struct {
			Case              string `json:"case"`
			Ordinal           int    `json:"ordinal"`
			RuntimeID         string `json:"runtimeId"`
			ExecutionName     string `json:"executionName"`
			Observation       string `json:"observation"`
			IteratorSnapshots int    `json:"iteratorSnapshots"`
			EPL               string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &entry); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if entry.Case != resultsetOutputLimitRowLimitContextGroupedCases[index] ||
			entry.Ordinal != resultsetOutputLimitRowLimitContextGroupedOrdinals[index] ||
			entry.RuntimeID != resultsetOutputLimitRowLimitContextGroupedJavaRuntimeIDs[index] ||
			entry.ExecutionName != resultsetOutputLimitRowLimitContextGroupedJavaExecutions[index] ||
			entry.Observation != observations[index] ||
			entry.IteratorSnapshots != iteratorSnapshots[index] ||
			entry.EPL != epls[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetOutputLimitRowLimitContextGroupedID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 141 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 141 steps", resultsetOutputLimitRowLimitContextGroupedID)
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
		expected := []string{"op", "case"}
		switch operation {
		case "send":
			expected = []string{"op", "case", "eventType", "payload"}
		case "snapshot":
			expected = []string{"op", "case", "statement", "label", "mode"}
			if index >= 129 {
				expected = []string{"op", "case", "statement", "mode"}
			}
		}
		if err := requireResultsetOutputLimitRowLimitContextGroupedFields(object, expected...); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateResultsetOutputLimitRowLimitContextGroupedScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetOutputLimitRowLimitContextGroupedScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetOutputLimitRowLimitContextGroupedID || len(scenario.Steps) != 141 {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetOutputLimitRowLimitContextGroupedID)
	}
	index := 0
	if scenario.Steps[index].Op != "case" || scenario.Steps[index].Case != resultsetOutputLimitRowLimitContextGroupedLimitCase {
		return fmt.Errorf("%s first case marker is not pinned", resultsetOutputLimitRowLimitContextGroupedID)
	}
	index++
	labels := []string{
		"batch-single-1", "batch-single-2", "batch-single-3",
		"batch-multi-1", "batch-multi-2", "batch-multi-3",
		"context-single-1", "context-single-2", "context-single-3",
		"context-multi-1", "context-multi-2", "context-multi-3",
	}
	singles := [][]struct {
		value  string
		number int
	}{
		{{"F", 0}, {"Q", 0}, {"R", 0}, {"T", 0}, {"M", 0}, {"T", 0}, {"A", 0}, {"I", 0}, {"P", 0}, {"B", 0}},
		{{"P", 0}, {"Q", 0}, {"P", 0}, {"T", 0}, {"P", 0}, {"T", 0}, {"P", 0}, {"P", 0}, {"P", 0}, {"B", 0}},
		{{"C", 0}, {"P", 0}, {"Q", 0}, {"P", 0}, {"T", 0}, {"P", 0}, {"T", 0}, {"P", 0}, {"P", 0}, {"P", 0}, {"X", 0}},
	}
	multis := [][]struct {
		value  string
		number int
	}{
		{{"F", 10}, {"X", 8}, {"F", 8}, {"G", 10}, {"X", 1}},
		{{"X", 10}, {"G", 12}, {"H", 100}, {"G", 10}, {"X", 1}},
		{{"G", 10}, {"G", 8}, {"G", 8}, {"G", 10}, {"G", 11}},
	}
	for phase := 0; phase < 12; phase++ {
		contextPhase := phase >= 6
		values := singles[phase%3]
		if phase >= 3 && phase < 6 || phase >= 9 {
			values = multis[phase%3]
		}
		if err := validateContextGroupedBoundaryStep(scenario.Steps[index], resultsetOutputLimitRowLimitContextGroupedLimitCase, "SupportBean_S0", 0); err != nil {
			return fmt.Errorf("phase %d start: %w", phase, err)
		}
		index++
		beforeSnapshot := len(values) - 1
		if contextPhase {
			beforeSnapshot = len(values)
		}
		for event := 0; event < beforeSnapshot; event++ {
			if err := validateContextGroupedBeanStep(scenario.Steps[index], resultsetOutputLimitRowLimitContextGroupedLimitCase, values[event].value, values[event].number); err != nil {
				return fmt.Errorf("phase %d source: %w", phase, err)
			}
			index++
		}
		step := scenario.Steps[index]
		if step.Op != "snapshot" || step.Case != resultsetOutputLimitRowLimitContextGroupedLimitCase ||
			step.Statement != "s0" || step.Label != labels[phase] || step.Mode != "ordered" {
			return fmt.Errorf("phase %d snapshot is not pinned", phase)
		}
		index++
		for event := beforeSnapshot; event < len(values); event++ {
			if err := validateContextGroupedBeanStep(scenario.Steps[index], resultsetOutputLimitRowLimitContextGroupedLimitCase, values[event].value, values[event].number); err != nil {
				return fmt.Errorf("phase %d boundary source: %w", phase, err)
			}
			index++
		}
		if err := validateContextGroupedBoundaryStep(scenario.Steps[index], resultsetOutputLimitRowLimitContextGroupedLimitCase, "SupportBean_S1", 0); err != nil {
			return fmt.Errorf("phase %d end: %w", phase, err)
		}
		index++
	}
	if scenario.Steps[index].Op != "case" || scenario.Steps[index].Case != resultsetOutputLimitRowLimitContextGroupedGroupedCase {
		return fmt.Errorf("fully grouped marker is not pinned")
	}
	index++
	if step := scenario.Steps[index]; step.Op != "snapshot" || step.Case != resultsetOutputLimitRowLimitContextGroupedGroupedCase || step.Statement != "s0" || step.Mode != "ordered" {
		return fmt.Errorf("fully grouped initial snapshot is not pinned")
	}
	index++
	values := []struct {
		value  string
		number int
	}{{"E1", 90}, {"E2", 5}, {"E3", 60}, {"E3", 40}, {"E2", 1000}}
	for _, value := range values {
		if err := validateContextGroupedBeanStep(scenario.Steps[index], resultsetOutputLimitRowLimitContextGroupedGroupedCase, value.value, value.number); err != nil {
			return err
		}
		index++
		if step := scenario.Steps[index]; step.Op != "snapshot" || step.Case != resultsetOutputLimitRowLimitContextGroupedGroupedCase || step.Statement != "s0" || step.Mode != "ordered" {
			return fmt.Errorf("fully grouped snapshot is not pinned")
		}
		index++
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetOutputLimitRowLimitContextGroupedID)
	}
	return nil
}

func validateContextGroupedBoundaryStep(step compat.Step, caseName, eventType string, id int) error {
	if step.Op != "send" || step.Case != caseName || step.EventType != eventType {
		return fmt.Errorf("boundary send is not pinned")
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return err
	}
	if err := requireResultsetOutputLimitRowLimitContextGroupedFields(fields, "id"); err != nil {
		return err
	}
	actual, err := decodeResultsetOutputLimitRowLimitContextGroupedInteger(fields["id"], "boundary id")
	if err != nil || actual != id {
		return fmt.Errorf("boundary payload is not pinned")
	}
	return nil
}

func validateContextGroupedBeanStep(step compat.Step, caseName, wantString string, wantInt int) error {
	if step.Op != "send" || step.Case != caseName || step.EventType != "SupportBean" {
		return fmt.Errorf("bean send is not pinned")
	}
	payload, err := decodeResultsetOutputLimitRowLimitContextGroupedBean(step)
	if err != nil {
		return err
	}
	if payload.TheString != wantString || payload.IntPrimitive != wantInt {
		return fmt.Errorf("bean payload is not pinned")
	}
	return nil
}

func decodeResultsetOutputLimitRowLimitContextGroupedBean(step compat.Step) (resultsetOutputLimitRowLimitContextGroupedBean, error) {
	if step.EventType != "SupportBean" {
		return resultsetOutputLimitRowLimitContextGroupedBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOutputLimitRowLimitContextGroupedBean{}, err
	}
	if err := requireResultsetOutputLimitRowLimitContextGroupedFields(fields, "theString", "intPrimitive"); err != nil {
		return resultsetOutputLimitRowLimitContextGroupedBean{}, err
	}
	if string(bytes.TrimSpace(fields["theString"])) == "null" {
		return resultsetOutputLimitRowLimitContextGroupedBean{}, fmt.Errorf("theString must be a string")
	}
	var payload resultsetOutputLimitRowLimitContextGroupedBean
	if err := json.Unmarshal(fields["theString"], &payload.TheString); err != nil {
		return resultsetOutputLimitRowLimitContextGroupedBean{}, fmt.Errorf("theString must be a string")
	}
	intPrimitive, err := decodeResultsetOutputLimitRowLimitContextGroupedInteger(fields["intPrimitive"], "intPrimitive")
	if err != nil {
		return resultsetOutputLimitRowLimitContextGroupedBean{}, err
	}
	payload.IntPrimitive = intPrimitive
	return payload, nil
}

func runResultsetOutputLimitRowLimitContextGroupedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetOutputLimitRowLimitContextGroupedScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	limitScenario, err := scenarioForCase(scenario, resultsetOutputLimitRowLimitContextGroupedLimitCase)
	if err != nil {
		return compat.Trace{}, err
	}
	groupedScenario, err := scenarioForCase(scenario, resultsetOutputLimitRowLimitContextGroupedGroupedCase)
	if err != nil {
		return compat.Trace{}, err
	}
	limitTrace, err := runResultsetOutputLimitRowLimitContextGroupedLimit(ctx, limitScenario)
	if err != nil {
		return compat.Trace{}, err
	}
	groupedTrace, err := runResultsetOutputLimitRowLimitContextGroupedGrouped(ctx, groupedScenario)
	if err != nil {
		return compat.Trace{}, err
	}
	limitTrace.Records = append(limitTrace.Records, groupedTrace.Records...)
	limitTrace.ID = scenario.ID
	return limitTrace, nil
}

func runResultsetOutputLimitRowLimitContextGroupedLimit(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	labels := []string{
		"batch-single-1", "batch-single-2", "batch-single-3",
		"batch-multi-1", "batch-multi-2", "batch-multi-3",
		"context-single-1", "context-single-2", "context-single-3",
		"context-multi-1", "context-multi-2", "context-multi-3",
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	index := 1
	var listenerSequence uint64
	for phase := 0; phase < len(labels); phase++ {
		contextPhase := phase >= 6
		values, single := resultsetOutputLimitRowLimitContextGroupedPhaseValues(phase)
		beforeSnapshot := len(values) - 1
		if contextPhase {
			beforeSnapshot = len(values)
		}
		phaseSteps := len(values) + 3
		if index+phaseSteps > len(scenario.Steps) {
			return compat.Trace{}, fmt.Errorf("%s phase %d is truncated", resultsetOutputLimitRowLimitContextGroupedID, phase)
		}
		phaseScenario := compat.Scenario{Version: scenario.Version, ID: scenario.ID}
		phaseScenario.Steps = append(phaseScenario.Steps, scenario.Steps[0])
		phaseScenario.Steps = append(phaseScenario.Steps, scenario.Steps[index:index+phaseSteps]...)
		phaseTrace, err := runResultsetOutputLimitRowLimitContextGroupedLimitPhase(ctx, phaseScenario, phase, contextPhase, single)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s phase %q: %w", resultsetOutputLimitRowLimitContextGroupedID, labels[phase], err)
		}
		if len(phaseTrace.Records) != 2 {
			return compat.Trace{}, fmt.Errorf("phase %q produced %d records, want 2", labels[phase], len(phaseTrace.Records))
		}
		for recordIndex, record := range phaseTrace.Records {
			if record.Operation == "listener" {
				listenerSequence++
				record.Sequence = listenerSequence
			}
			if record.Operation == "snapshot" {
				record.Partitions = nil
				if record.New == nil {
					record.New = []compat.ResultRecord{}
				}
			}
			wantOperation := "snapshot"
			if phase == 2 && recordIndex == 0 {
				// The tenth event completes length_batch(10) before the
				// scenario's snapshot step, so Java records the listener first.
				wantOperation = "listener"
			} else if phase != 2 && recordIndex == 1 {
				wantOperation = "listener"
			}
			if record.Operation != wantOperation {
				return compat.Trace{}, fmt.Errorf("phase %q record %d operation %q, want %q", labels[phase], recordIndex, record.Operation, wantOperation)
			}
			trace.Records = append(trace.Records, record)
		}
		index += phaseSteps
		_ = beforeSnapshot
	}
	if index != len(scenario.Steps) || listenerSequence != 12 {
		return compat.Trace{}, fmt.Errorf("limit-one replay consumed %d steps with %d listeners", index, listenerSequence)
	}
	return trace, nil
}

func resultsetOutputLimitRowLimitContextGroupedPhaseValues(phase int) ([]struct {
	value  string
	number int
}, bool) {
	singleValues := [][]string{
		{"F", "Q", "R", "T", "M", "T", "A", "I", "P", "B"},
		{"P", "Q", "P", "T", "P", "T", "P", "P", "P", "B"},
		{"C", "P", "Q", "P", "T", "P", "T", "P", "P", "P", "X"},
	}
	multiValues := [][]struct {
		value  string
		number int
	}{
		{{"F", 10}, {"X", 8}, {"F", 8}, {"G", 10}, {"X", 1}},
		{{"X", 10}, {"G", 12}, {"H", 100}, {"G", 10}, {"X", 1}},
		{{"G", 10}, {"G", 8}, {"G", 8}, {"G", 10}, {"G", 11}},
	}
	if (phase >= 3 && phase < 6) || phase >= 9 {
		return multiValues[phase%3], false
	}
	values := make([]struct {
		value  string
		number int
	}, len(singleValues[phase%3]))
	for index, value := range singleValues[phase%3] {
		values[index].value = value
	}
	return values, true
}

func runResultsetOutputLimitRowLimitContextGroupedLimitPhase(ctx context.Context, scenario compat.Scenario, phase int, contextPhase, single bool) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOutputLimitRowLimitContextGroupedBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetOutputLimitRowLimitContextGroupedBoundary](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetOutputLimitRowLimitContextGroupedBoundary](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	if contextPhase {
		isStart := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
		isEnd := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S1"))
		if _, err := esper.CreateInitiatedTerminatedContext(env, "StartS0EndS1", esper.Literal("global"), isStart, isEnd); err != nil {
			return compat.Trace{}, err
		}
	}
	theString := esper.Field[resultsetOutputLimitRowLimitContextGroupedBean, string]("theString")
	intPrimitive := esper.Field[resultsetOutputLimitRowLimitContextGroupedBean, int]("intPrimitive")
	var query esper.Query
	if single {
		stream := esper.From[resultsetOutputLimitRowLimitContextGroupedBean](env, "SupportBean")
		if contextPhase {
			stream = stream.Window(esper.KeepAll())
			query = esper.Select(stream, esper.Alias("theString", theString)).Query(
				esper.StatementName("s0"), esper.WithContext("StartS0EndS1"),
				esper.WithOutput(esper.OutputSnapshotWhenTerminated()),
				esper.OrderBy(esper.Ascending(esper.ResultField[string]("theString"))), esper.Limit(1))
		} else {
			stream = stream.Window(esper.LengthBatch(10))
			query = esper.Select(stream, esper.Alias("theString", theString)).Query(
				esper.StatementName("s0"), esper.OrderBy(esper.Ascending(esper.ResultField[string]("theString"))), esper.Limit(1))
		}
	} else {
		stream := esper.From[resultsetOutputLimitRowLimitContextGroupedBean](env, "SupportBean")
		if contextPhase {
			stream = stream.Window(esper.KeepAll())
			query = esper.Select(stream, esper.Alias("theString", theString), esper.Alias("intPrimitive", intPrimitive)).Query(
				esper.StatementName("s0"), esper.WithContext("StartS0EndS1"),
				esper.WithOutput(esper.OutputSnapshotWhenTerminated()),
				esper.OrderBy(esper.Ascending(esper.ResultField[string]("theString")), esper.Descending(esper.ResultField[int]("intPrimitive"))), esper.Limit(1))
		} else {
			stream = stream.Window(esper.LengthBatch(5))
			query = esper.Select(stream, esper.Alias("theString", theString), esper.Alias("intPrimitive", intPrimitive)).Query(
				esper.StatementName("s0"), esper.OrderBy(esper.Ascending(esper.ResultField[string]("theString")), esper.Descending(esper.ResultField[int]("intPrimitive"))), esper.Limit(1))
		}
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, fmt.Sprintf("%s-%d", resultsetOutputLimitRowLimitContextGroupedJavaRuntimeIDs[0], phase))
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statement, scenario, decodeResultsetOutputLimitRowLimitContextGroupedPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown statement %q", name)
		}
		return statement, nil
	})
}
func decodeResultsetOutputLimitRowLimitContextGroupedPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		return decodeResultsetOutputLimitRowLimitContextGroupedBean(step)
	case "SupportBean_S0", "SupportBean_S1":
		var fields map[string]json.RawMessage
		if err := strictObject(step.Payload, &fields); err != nil {
			return nil, err
		}
		if err := requireResultsetOutputLimitRowLimitContextGroupedFields(fields, "id"); err != nil {
			return nil, err
		}
		id, err := decodeResultsetOutputLimitRowLimitContextGroupedInteger(fields["id"], "boundary id")
		if err != nil {
			return nil, err
		}
		return resultsetOutputLimitRowLimitContextGroupedBoundary{ID: id}, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}

func runResultsetOutputLimitRowLimitContextGroupedGrouped(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOutputLimitRowLimitContextGroupedBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	theString := esper.Field[resultsetOutputLimitRowLimitContextGroupedBean, string]("theString")
	intPrimitive := esper.Field[resultsetOutputLimitRowLimitContextGroupedBean, int]("intPrimitive")
	plan, err := env.Build(esper.From[resultsetOutputLimitRowLimitContextGroupedBean](env, "SupportBean").Window(esper.LengthWindow(5)).GroupBy(theString).Select(
		esper.Alias("theString", theString),
		esper.Alias("mysum", esper.Sum[int](intPrimitive)),
	).Query(
		esper.StatementName("s0"),
		esper.OrderBy(esper.Ascending(esper.ResultField[int]("mysum"))),
		esper.Limit(2),
	))
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, resultsetOutputLimitRowLimitContextGroupedJavaRuntimeIDs[1])
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	listenerRecords := 0
	trace, err := compat.ReplayWithStatements(ctx, engine, statement, scenario, func(step compat.Step) (any, error) {
		return decodeResultsetOutputLimitRowLimitContextGroupedBean(step)
	}, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown statement %q", name)
		}
		return statement, nil
	})
	if err != nil {
		return compat.Trace{}, err
	}
	for _, record := range trace.Records {
		if record.Operation == "listener" {
			listenerRecords++
		}
	}
	if listenerRecords != 0 {
		return compat.Trace{}, fmt.Errorf("fully grouped iterator emitted %d listener records", listenerRecords)
	}
	return trace, nil
}

func requireResultsetOutputLimitRowLimitContextGroupedFields(object map[string]json.RawMessage, expected ...string) error {
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

func validateResultsetOutputLimitRowLimitContextGroupedStringArray(raw json.RawMessage, expected []string, name string) error {
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

func decodeResultsetOutputLimitRowLimitContextGroupedInteger(raw json.RawMessage, name string) (int, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var number json.Number
	if err := decoder.Decode(&number); err != nil || number == "" {
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

func rejectResultsetOutputLimitRowLimitContextGroupedJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkResultsetOutputLimitRowLimitContextGroupedJSON(decoder); err != nil {
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

func walkResultsetOutputLimitRowLimitContextGroupedJSON(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delim, ok := token.(json.Delim); ok {
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
				if err := walkResultsetOutputLimitRowLimitContextGroupedJSON(decoder); err != nil {
					return err
				}
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim('}') {
				return fmt.Errorf("JSON object is not closed")
			}
		case '[':
			for decoder.More() {
				if err := walkResultsetOutputLimitRowLimitContextGroupedJSON(decoder); err != nil {
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
	}
	return nil
}
