package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	rollupGroupingFuncsJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	rollupGroupingFuncsID          = "rollup-grouping-funcs-dedicated"
	rollupGroupingFuncsDescription = "ResultSetQueryTypeRollupGroupingFuncs ordinal 0: three compilation paths for the SupportCarEvent grouping-function documentation sample."
	rollupGroupingFuncsEPL         = "@name('s0') select name, place, sum(count), grouping(name), grouping(place), grouping_id(name,place) as gid from SupportCarEvent group by grouping sets((name, place), name, place, ())"
	rollupGroupingFuncsStaticID    = "java-1ae66c9985dc53282af0"
)

var (
	rollupGroupingFuncsJavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRollupGroupingFuncs.java",
	}
	rollupGroupingFuncsJavaRuntimeIDs = []string{
		"java-runtime-52a4f853b6052fb47437",
	}
	rollupGroupingFuncsJavaExecutions = []string{
		"ResultSetQueryTypeDocSampleCarEventAndGroupingFunc",
	}
	rollupGroupingFuncsCases = []string{
		"doc-sample-plain",
		"doc-sample-audit",
		"doc-sample-model",
	}
)

var rollupGroupingFuncsCaseEPL = []string{
	rollupGroupingFuncsEPL,
	rollupGroupingFuncsEPL,
	rollupGroupingFuncsEPL,
}

var rollupGroupingFuncsCaseOrdinals = []int{0, 0, 0}

var rollupGroupingFuncsCaseRuntimes = []string{
	"java-runtime-52a4f853b6052fb47437",
	"java-runtime-52a4f853b6052fb47437",
	"java-runtime-52a4f853b6052fb47437",
}

type rollupGroupingFuncsCarEvent struct {
	Name  string `esper:"name"`
	Place string `esper:"place"`
	Count int    `esper:"count"`
}

func loadRollupGroupingFuncsScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read rollup-grouping-funcs scenario: %w", err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode rollup-grouping-funcs scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode rollup-grouping-funcs scenario: %w", err)
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs scenario contains unexpected or missing fields")
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs scenario is missing field %q", name)
		}
	}
	var version, id, description, javaCommit, javaSource string
	for name, target := range map[string]*string{
		"version": &version, "id": &id, "description": &description,
		"javaCommit": &javaCommit, "javaSource": &javaSource,
	} {
		if err := json.Unmarshal(root[name], target); err != nil {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs scenario %s must be a string", name)
		}
	}
	if version != compat.ScenarioVersion || id != rollupGroupingFuncsID ||
		description != rollupGroupingFuncsDescription || javaCommit != rollupGroupingFuncsJavaCommit ||
		javaSource != rollupGroupingFuncsJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs scenario metadata is not pinned")
	}
	for name, expected := range map[string][]string{
		"javaRuntimes":  rollupGroupingFuncsJavaRuntimeIDs,
		"javaNames":     rollupGroupingFuncsJavaExecutions,
		"javaStaticIds": {rollupGroupingFuncsStaticID},
		"javaFlags":     {},
	} {
		if err := validateResultSetQueryTypeRowForAllHavingSumStringArray(root[name], expected, name); err != nil {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs %w", err)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(rollupGroupingFuncsCases) {
		return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs scenario must contain exactly three cases")
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs case %d: %w", index, err)
		}
		requiredCase := []string{"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"}
		if len(object) != len(requiredCase) {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs case %d contains unexpected or missing fields", index)
		}
		for _, name := range requiredCase {
			if _, ok := object[name]; !ok {
				return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs case %d is missing field %q", index, name)
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
		if err := json.Unmarshal(rawCase, &metadata); err != nil {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs case %d has invalid metadata: %w", index, err)
		}
		if metadata.Case != rollupGroupingFuncsCases[index] ||
			metadata.Ordinal != rollupGroupingFuncsCaseOrdinals[index] ||
			metadata.RuntimeID != rollupGroupingFuncsCaseRuntimes[index] ||
			metadata.ExecutionName != rollupGroupingFuncsJavaExecutions[0] ||
			metadata.Observation != "listener" || metadata.IteratorSnapshots != 0 ||
			metadata.EPL != rollupGroupingFuncsCaseEPL[index] {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs case %d metadata is not pinned", index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 9 {
		return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs scenario must contain exactly nine steps")
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs step %d: %w", index, err)
		}
		expectedFields := 2
		if index%3 != 0 {
			expectedFields = 3
		}
		if len(object) != expectedFields {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs step %d fields are not pinned", index)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs step %d: %w", index, err)
		}
		if index%3 != 0 {
			var payload map[string]json.RawMessage
			if err := strictObject(steps[index].Payload, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs step %d payload: %w", index, err)
			}
			if len(payload) != 3 {
				return compat.Scenario{}, fmt.Errorf("rollup-grouping-funcs step %d payload fields are not pinned", index)
			}
		}
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: steps}
	if err := validateRollupGroupingFuncsScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateRollupGroupingFuncsScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != rollupGroupingFuncsID || len(scenario.Steps) != 9 {
		return fmt.Errorf("rollup-grouping-funcs scenario steps are not pinned")
	}
	for phaseIndex, caseName := range rollupGroupingFuncsCases {
		markerIndex := phaseIndex * 3
		marker := scenario.Steps[markerIndex]
		if marker.Op != "case" || marker.Case != caseName {
			return fmt.Errorf("rollup-grouping-funcs step %d must start case %q", markerIndex, caseName)
		}
		for sendIndex, expected := range []rollupGroupingFuncsCarEvent{
			{Name: "skoda", Place: "france", Count: 100},
			{Name: "skoda", Place: "germany", Count: 75},
		} {
			stepIndex := markerIndex + sendIndex + 1
			step := scenario.Steps[stepIndex]
			if step.Op != "send" || step.EventType != "SupportCarEvent" {
				return fmt.Errorf("rollup-grouping-funcs step %d must send SupportCarEvent", stepIndex)
			}
			value, err := decodeRollupGroupingFuncsPayload(step)
			if err != nil {
				return fmt.Errorf("rollup-grouping-funcs step %d: %w", stepIndex, err)
			}
			if value != expected {
				return fmt.Errorf("rollup-grouping-funcs step %d payload is not pinned", stepIndex)
			}
		}
	}
	return nil
}

func runRollupGroupingFuncsScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateRollupGroupingFuncsScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range rollupGroupingFuncsCases {
		caseTrace, err := runRollupGroupingFuncsCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("rollup-grouping-funcs case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runRollupGroupingFuncsCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[rollupGroupingFuncsCarEvent](env, "SupportCarEvent"); err != nil {
		return compat.Trace{}, err
	}
	name := esper.Field[rollupGroupingFuncsCarEvent, string]("name")
	place := esper.Field[rollupGroupingFuncsCarEvent, string]("place")
	count := esper.Field[rollupGroupingFuncsCarEvent, int]("count")
	query := esper.From[rollupGroupingFuncsCarEvent](env, "SupportCarEvent").GroupByGroupingSets(
		esper.GroupingSet(name, place), esper.GroupingSet(name), esper.GroupingSet(place), esper.GroupingSet(),
	).Select(
		esper.Alias("name", name),
		esper.Alias("place", place),
		esper.Alias("sum(count)", esper.Sum[int](count)),
		esper.Alias("grouping(name)", esper.Grouping(name)),
		esper.Alias("grouping(place)", esper.Grouping(place)),
		esper.Alias("gid", esper.GroupingID(name, place)),
	).Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, rollupGroupingFuncsJavaRuntimeIDs[0])
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeRollupGroupingFuncsPayload, func(statementName string) (*esper.Statement, error) {
		if statementName != statement.Name() {
			return nil, fmt.Errorf("unknown rollup-grouping-funcs statement %q", statementName)
		}
		return statement, nil
	})
}

func decodeRollupGroupingFuncsPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportCarEvent" {
		return nil, fmt.Errorf("step must send SupportCarEvent")
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	if len(fields) != 3 {
		return nil, fmt.Errorf("SupportCarEvent payload must contain exactly name, place, and count")
	}
	var event rollupGroupingFuncsCarEvent
	if string(bytes.TrimSpace(fields["name"])) == "null" {
		return nil, fmt.Errorf("name must be a string")
	}
	if err := json.Unmarshal(fields["name"], &event.Name); err != nil {
		return nil, fmt.Errorf("name must be a string")
	}
	if string(bytes.TrimSpace(fields["place"])) == "null" {
		return nil, fmt.Errorf("place must be a string")
	}
	if err := json.Unmarshal(fields["place"], &event.Place); err != nil {
		return nil, fmt.Errorf("place must be a string")
	}
	if string(bytes.TrimSpace(fields["count"])) == "null" {
		return nil, fmt.Errorf("count must be an integer")
	}
	if err := json.Unmarshal(fields["count"], &event.Count); err != nil {
		return nil, fmt.Errorf("count must be an integer")
	}
	return event, nil
}
