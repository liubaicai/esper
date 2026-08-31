package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultSetQueryTypeRollupHavingIteratorJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeRollupHavingIteratorID          = "resultset-querytype-rollup-having-iterator"
	resultSetQueryTypeRollupHavingIteratorDescription = "ResultSetQueryTypeRollupHavingAndOrderBy ordinals 2-3: length-window rollup iterator snapshots with an optional keepall join."
	resultSetQueryTypeRollupHavingIteratorSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRollupHavingAndOrderBy.java"
	resultSetQueryTypeRollupHavingIteratorStaticID    = "java-efb06d5c38363ea3a5d0"
)

var (
	resultSetQueryTypeRollupHavingIteratorJavaRuntimeIDs = []string{
		"java-runtime-1ce0d3fc4b6fed76d38d",
		"java-runtime-7730d794daee6dac92c7",
	}
	resultSetQueryTypeRollupHavingIteratorJavaExecutions = []string{
		"ResultSetQueryTypeIteratorWindow{join=false}",
		"ResultSetQueryTypeIteratorWindow{join=true}",
	}
	resultSetQueryTypeRollupHavingIteratorCases = []string{
		"iterator-window-no-join",
		"iterator-window-join",
	}
	resultSetQueryTypeRollupHavingIteratorOrdinals = []int{2, 3}
)

const (
	resultSetQueryTypeRollupHavingIteratorNoJoinEPL = "@Name('s0') select theString as c0, sum(intPrimitive) as c1 from SupportBean#length(3) group by rollup(theString)"
	resultSetQueryTypeRollupHavingIteratorJoinEPL   = "@Name('s0') select theString as c0, sum(intPrimitive) as c1 from SupportBean#length(3), SupportBean_S0#keepall group by rollup(theString)"
)

var resultSetQueryTypeRollupHavingIteratorJavaSources = []string{
	resultSetQueryTypeRollupHavingIteratorSource,
}

type resultSetQueryTypeRollupHavingIteratorBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type resultSetQueryTypeRollupHavingIteratorS0 struct {
	ID int `esper:"id"`
}

func loadResultSetQueryTypeRollupHavingIteratorScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-rollup-having-iterator scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read resultset-querytype-rollup-having-iterator scenario: %w", err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-querytype-rollup-having-iterator scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-querytype-rollup-having-iterator scenario: %w", err)
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-rollup-having-iterator scenario contains unexpected or missing fields")
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("resultset-querytype-rollup-having-iterator scenario is missing field %q", name)
		}
	}
	var version, id, description, javaCommit, javaSource string
	for name, target := range map[string]*string{
		"version": &version, "id": &id, "description": &description,
		"javaCommit": &javaCommit, "javaSource": &javaSource,
	} {
		if err := json.Unmarshal(root[name], target); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario %s must be a string", name)
		}
	}
	if version != compat.ScenarioVersion || id != resultSetQueryTypeRollupHavingIteratorID ||
		description != resultSetQueryTypeRollupHavingIteratorDescription ||
		javaCommit != resultSetQueryTypeRollupHavingIteratorJavaCommit ||
		javaSource != resultSetQueryTypeRollupHavingIteratorSource {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-rollup-having-iterator scenario metadata is not pinned")
	}
	for name, expected := range map[string][]string{
		"javaRuntimes":  resultSetQueryTypeRollupHavingIteratorJavaRuntimeIDs,
		"javaNames":     resultSetQueryTypeRollupHavingIteratorJavaExecutions,
		"javaStaticIds": {resultSetQueryTypeRollupHavingIteratorStaticID},
		"javaFlags":     {},
	} {
		if err := validateResultSetQueryTypeRowForAllHavingSumStringArray(root[name], expected, name); err != nil {
			return compat.Scenario{}, fmt.Errorf("resultset-querytype-rollup-having-iterator %w", err)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != 2 {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-rollup-having-iterator scenario must contain exactly two cases")
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		requiredCase := []string{"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"}
		if len(object) != len(requiredCase) {
			return compat.Scenario{}, fmt.Errorf("scenario case %d contains unexpected or missing fields", index)
		}
		for _, name := range requiredCase {
			if _, ok := object[name]; !ok {
				return compat.Scenario{}, fmt.Errorf("scenario case %d is missing field %q", index, name)
			}
		}
		var metadata struct {
			Case              string `json:"case"`
			Ordinal           int    `json:"ordinal"`
			RuntimeID         string `json:"runtimeId"`
			ExecutionName     string `json:"executionName"`
			Observation       string `json:"observation"`
			IteratorSnapshots int    `json:"iteratorSnapshots"`
			EPL               string `json:"epl"`
		}
		ordinal, ordinalErr := decodeResultSetQueryTypeRowForAllHavingSumInteger(object["ordinal"], "ordinal")
		iteratorSnapshots, iteratorErr := decodeResultSetQueryTypeRowForAllHavingSumInteger(object["iteratorSnapshots"], "iteratorSnapshots")
		if ordinalErr != nil || iteratorErr != nil || json.Unmarshal(rawCase, &metadata) != nil ||
			metadata.Case != resultSetQueryTypeRollupHavingIteratorCases[index] ||
			ordinal != resultSetQueryTypeRollupHavingIteratorOrdinals[index] ||
			metadata.RuntimeID != resultSetQueryTypeRollupHavingIteratorJavaRuntimeIDs[index] ||
			metadata.ExecutionName != resultSetQueryTypeRollupHavingIteratorJavaExecutions[index] ||
			metadata.Observation != "iterator" || iteratorSnapshots != 4 ||
			metadata.EPL != resultSetQueryTypeRollupHavingIteratorEPL(index) {
			return compat.Scenario{}, fmt.Errorf("scenario case %d metadata is not pinned", index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 20 {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-rollup-having-iterator scenario must contain exactly twenty steps")
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		position := index % 10
		expectedFields := 3
		if position == 0 {
			expectedFields = 2
		}
		if len(object) != expectedFields {
			return compat.Scenario{}, fmt.Errorf("scenario step %d fields are not pinned", index)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: steps}
	if err := validateResultSetQueryTypeRollupHavingIteratorScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func resultSetQueryTypeRollupHavingIteratorEPL(index int) string {
	if index == 1 {
		return resultSetQueryTypeRollupHavingIteratorJoinEPL
	}
	return resultSetQueryTypeRollupHavingIteratorNoJoinEPL
}

func validateResultSetQueryTypeRollupHavingIteratorScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeRollupHavingIteratorID || len(scenario.Steps) != 20 {
		return fmt.Errorf("resultset-querytype-rollup-having-iterator scenario steps are not pinned")
	}
	for caseIndex, caseName := range resultSetQueryTypeRollupHavingIteratorCases {
		base := caseIndex * 10
		marker := scenario.Steps[base]
		if marker.Op != "case" || marker.Case != caseName {
			return fmt.Errorf("scenario step %d must start case %q", base, caseName)
		}
		seed := scenario.Steps[base+1]
		if seed.Op != "send" || seed.EventType != "SupportBean_S0" {
			return fmt.Errorf("scenario step %d must send SupportBean_S0", base+1)
		}
		seedValue, err := decodeResultSetQueryTypeRollupHavingIteratorS0Payload(seed)
		if err != nil || seedValue.ID != 1 {
			return fmt.Errorf("scenario step %d has unexpected SupportBean_S0 payload", base+1)
		}
		expected := []resultSetQueryTypeRollupHavingIteratorBean{
			{TheString: "E1", IntPrimitive: 1},
			{TheString: "E2", IntPrimitive: 2},
			{TheString: "E1", IntPrimitive: 3},
			{TheString: "E2", IntPrimitive: 4},
		}
		for sendIndex, want := range expected {
			sendStepIndex := base + 2 + sendIndex*2
			send := scenario.Steps[sendStepIndex]
			if send.Op != "send" || send.EventType != "SupportBean" {
				return fmt.Errorf("scenario step %d must send SupportBean", sendStepIndex)
			}
			value, err := decodeResultSetQueryTypeRollupHavingIteratorBeanPayload(send)
			if err != nil || value != want {
				return fmt.Errorf("scenario step %d has unexpected SupportBean payload", sendStepIndex)
			}
			snapshot := scenario.Steps[sendStepIndex+1]
			if snapshot.Op != "snapshot" || snapshot.Statement != "s0" || snapshot.Mode != "any" {
				return fmt.Errorf("scenario step %d must be an any-order s0 snapshot", sendStepIndex+1)
			}
		}
	}
	return nil
}

func runResultSetQueryTypeRollupHavingIteratorScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeRollupHavingIteratorScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for index, caseName := range resultSetQueryTypeRollupHavingIteratorCases {
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultSetQueryTypeRollupHavingIteratorCase(ctx, caseScenario, index)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-querytype-rollup-having-iterator case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if len(trace.Records) != 8 {
		return compat.Trace{}, fmt.Errorf("resultset-querytype-rollup-having-iterator expected eight snapshot records, got %d", len(trace.Records))
	}
	return trace, nil
}

func runResultSetQueryTypeRollupHavingIteratorCase(ctx context.Context, scenario compat.Scenario, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeRollupHavingIteratorBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultSetQueryTypeRollupHavingIteratorS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}

	beanStream := esper.From[resultSetQueryTypeRollupHavingIteratorBean](env, "SupportBean").Window(esper.LengthWindow(3))
	var query esper.Query
	if caseIndex == 0 {
		theString := esper.Field[resultSetQueryTypeRollupHavingIteratorBean, string]("theString")
		intPrimitive := esper.Field[resultSetQueryTypeRollupHavingIteratorBean, int]("intPrimitive")
		query = beanStream.GroupByRollup(theString).Select(
			esper.Alias("c0", theString),
			esper.Alias("c1", esper.Sum[int](intPrimitive)),
		).Query(esper.StatementName("s0"))
	} else {
		s0Stream := esper.From[resultSetQueryTypeRollupHavingIteratorS0](env, "SupportBean_S0").Window(esper.KeepAll())
		joined := esper.Join(beanStream, s0Stream)
		theString := esper.JoinField[string](0, "theString")
		sum := esper.Sum[int](esper.JoinField[int](0, "intPrimitive"))
		query = joined.Aggregate().Rollup(theString).Select(
			esper.Alias("c0", theString),
			esper.Alias("c1", sum),
		).Query(esper.StatementName("s0"))
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, resultSetQueryTypeRollupHavingIteratorJavaRuntimeIDs[caseIndex])
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	caseName := ""
	var sequence uint64
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return trace, err
		}
		if step.Op == "case" {
			caseName = step.Case
			continue
		}
		switch step.Op {
		case "send":
			payload, err := decodeResultSetQueryTypeRollupHavingIteratorPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "snapshot":
			if step.Statement != statement.Name() || step.Mode != "any" {
				return trace, fmt.Errorf("snapshot metadata is not pinned")
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return trace, err
			}
			rows := compat.NormalizeResults(result.Batch.New)
			if rows == nil {
				rows = []compat.ResultRecord{}
			}
			sequence++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: statement.Name(),
				Sequence:  sequence,
				Time:      engine.Now().UTC().Format(time.RFC3339Nano),
				New:       rows,
			})
		default:
			return trace, fmt.Errorf("unsupported resultset-querytype-rollup-having-iterator step op %q", step.Op)
		}
	}
	if len(trace.Records) != 4 {
		return trace, fmt.Errorf("resultset-querytype-rollup-having-iterator case %q expected four snapshots, got %d", caseName, len(trace.Records))
	}
	return trace, nil
}

func decodeResultSetQueryTypeRollupHavingIteratorPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		return decodeResultSetQueryTypeRollupHavingIteratorBeanPayload(step)
	case "SupportBean_S0":
		return decodeResultSetQueryTypeRollupHavingIteratorS0Payload(step)
	default:
		return nil, fmt.Errorf("unknown resultset-querytype-rollup-having-iterator event type %q", step.EventType)
	}
}

func decodeResultSetQueryTypeRollupHavingIteratorBeanPayload(step compat.Step) (resultSetQueryTypeRollupHavingIteratorBean, error) {
	if step.EventType != "SupportBean" {
		return resultSetQueryTypeRollupHavingIteratorBean{}, fmt.Errorf("step must send SupportBean")
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultSetQueryTypeRollupHavingIteratorBean{}, err
	}
	if len(fields) != 2 {
		return resultSetQueryTypeRollupHavingIteratorBean{}, fmt.Errorf("SupportBean payload must contain exactly theString and intPrimitive")
	}
	if string(bytes.TrimSpace(fields["theString"])) == "null" {
		return resultSetQueryTypeRollupHavingIteratorBean{}, fmt.Errorf("theString must be a string")
	}
	var result resultSetQueryTypeRollupHavingIteratorBean
	if err := json.Unmarshal(fields["theString"], &result.TheString); err != nil {
		return resultSetQueryTypeRollupHavingIteratorBean{}, fmt.Errorf("theString must be a string")
	}
	value, err := decodeResultSetQueryTypeRowForAllHavingSumInteger(fields["intPrimitive"], "intPrimitive")
	if err != nil {
		return resultSetQueryTypeRollupHavingIteratorBean{}, err
	}
	result.IntPrimitive = value
	return result, nil
}

func decodeResultSetQueryTypeRollupHavingIteratorS0Payload(step compat.Step) (resultSetQueryTypeRollupHavingIteratorS0, error) {
	if step.EventType != "SupportBean_S0" {
		return resultSetQueryTypeRollupHavingIteratorS0{}, fmt.Errorf("step must send SupportBean_S0")
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultSetQueryTypeRollupHavingIteratorS0{}, err
	}
	if len(fields) != 1 {
		return resultSetQueryTypeRollupHavingIteratorS0{}, fmt.Errorf("SupportBean_S0 payload must contain exactly id")
	}
	value, err := decodeResultSetQueryTypeRowForAllHavingSumInteger(fields["id"], "id")
	if err != nil {
		return resultSetQueryTypeRollupHavingIteratorS0{}, err
	}
	return resultSetQueryTypeRollupHavingIteratorS0{ID: value}, nil
}

var _ = time.Unix
