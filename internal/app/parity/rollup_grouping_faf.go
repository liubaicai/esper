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
	rollupGroupingFAFJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	rollupGroupingFAFID          = "rollup-grouping-funcs-faf-dedicated"
	rollupGroupingFAFDescription = "ResultSetQueryTypeRollupGroupingFuncs ordinal 2: fire-and-forget grouping sets over a public CarWindow."
	rollupGroupingFAFEPL         = "select name, place, sum(count), grouping(name), grouping(place), grouping_id(name,place) as gid from CarWindow group by grouping sets((name, place), name, place, ())"
	rollupGroupingFAFStaticID    = "java-1ae66c9985dc53282af0"
	rollupGroupingFAFCase        = "faf-grouping-snapshot"
)

var (
	rollupGroupingFAFJavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRollupGroupingFuncs.java",
	}
	rollupGroupingFAFJavaRuntimeIDs = []string{
		"java-runtime-e8f49362431d4c33baa8",
	}
	rollupGroupingFAFJavaExecutions = []string{
		"ResultSetQueryTypeFAFCarEventAndGroupingFunc",
	}
)

type rollupGroupingFAFCarEvent struct {
	Name  string `esper:"name"`
	Place string `esper:"place"`
	Count int    `esper:"count"`
}

func loadRollupGroupingFAFScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read rollup-grouping-faf scenario: %w", err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode rollup-grouping-faf scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode rollup-grouping-faf scenario: %w", err)
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf scenario contains unexpected or missing fields")
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf scenario is missing field %q", name)
		}
	}
	var version, id, description, javaCommit, javaSource string
	for name, target := range map[string]*string{
		"version": &version, "id": &id, "description": &description,
		"javaCommit": &javaCommit, "javaSource": &javaSource,
	} {
		if err := json.Unmarshal(root[name], target); err != nil {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf scenario %s must be a string", name)
		}
	}
	if version != compat.ScenarioVersion || id != rollupGroupingFAFID ||
		description != rollupGroupingFAFDescription || javaCommit != rollupGroupingFAFJavaCommit ||
		javaSource != rollupGroupingFAFJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf scenario metadata is not pinned")
	}
	for name, expected := range map[string][]string{
		"javaRuntimes":  rollupGroupingFAFJavaRuntimeIDs,
		"javaNames":     rollupGroupingFAFJavaExecutions,
		"javaStaticIds": {rollupGroupingFAFStaticID},
		"javaFlags":     {"FIREANDFORGET"},
	} {
		if err := validateResultSetQueryTypeRowForAllHavingSumStringArray(root[name], expected, name); err != nil {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf %w", err)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != 1 {
		return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf scenario must contain exactly one case")
	}
	var caseObject map[string]json.RawMessage
	if err := strictObject(rawCases[0], &caseObject); err != nil {
		return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf case: %w", err)
	}
	requiredCase := []string{"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"}
	if len(caseObject) != len(requiredCase) {
		return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf case contains unexpected or missing fields")
	}
	for _, name := range requiredCase {
		if _, ok := caseObject[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf case is missing field %q", name)
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
	if err := json.Unmarshal(rawCases[0], &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf case has invalid metadata: %w", err)
	}
	if metadata.Case != rollupGroupingFAFCase || metadata.Ordinal != 2 ||
		metadata.RuntimeID != rollupGroupingFAFJavaRuntimeIDs[0] ||
		metadata.ExecutionName != rollupGroupingFAFJavaExecutions[0] ||
		metadata.Observation != "faf" || metadata.IteratorSnapshots != 0 ||
		metadata.EPL != rollupGroupingFAFEPL {
		return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf case metadata is not pinned")
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 8 {
		return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf scenario must contain exactly eight steps")
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf step %d: %w", index, err)
		}
		expectedFields := 3
		if index == 0 || index == len(rawSteps)-1 {
			expectedFields = 2
		}
		if len(object) != expectedFields {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf step %d fields are not pinned", index)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf step %d: %w", index, err)
		}
		if index > 0 && index < len(rawSteps)-1 {
			var payload map[string]json.RawMessage
			if err := strictObject(steps[index].Payload, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf step %d payload: %w", index, err)
			}
			if len(payload) != 3 {
				return compat.Scenario{}, fmt.Errorf("rollup-grouping-faf step %d payload fields are not pinned", index)
			}
		}
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: steps}
	if err := validateRollupGroupingFAFScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateRollupGroupingFAFScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != rollupGroupingFAFID || len(scenario.Steps) != 8 {
		return fmt.Errorf("rollup-grouping-faf scenario steps are not pinned")
	}
	marker := scenario.Steps[0]
	if marker.Op != "case" || marker.Case != rollupGroupingFAFCase {
		return fmt.Errorf("rollup-grouping-faf step 0 must start case %q", rollupGroupingFAFCase)
	}
	expected := []rollupGroupingFAFCarEvent{
		{Name: "skoda", Place: "france", Count: 10000},
		{Name: "skoda", Place: "germany", Count: 5000},
		{Name: "bmw", Place: "france", Count: 100},
		{Name: "bmw", Place: "germany", Count: 1000},
		{Name: "opel", Place: "france", Count: 7000},
		{Name: "opel", Place: "germany", Count: 7000},
	}
	for index, want := range expected {
		stepIndex := index + 1
		step := scenario.Steps[stepIndex]
		if step.Op != "send" || step.EventType != "SupportCarEvent" || step.Case != "" {
			return fmt.Errorf("rollup-grouping-faf step %d must send SupportCarEvent", stepIndex)
		}
		value, err := decodeRollupGroupingFAFPayload(step)
		if err != nil {
			return fmt.Errorf("rollup-grouping-faf step %d: %w", stepIndex, err)
		}
		if value != want {
			return fmt.Errorf("rollup-grouping-faf step %d payload is not pinned", stepIndex)
		}
	}
	faf := scenario.Steps[7]
	if faf.Op != "faf" || faf.Statement != "s0" || faf.Case != "" {
		return fmt.Errorf("rollup-grouping-faf step 7 must execute FAF statement s0")
	}
	return nil
}

func runRollupGroupingFAFScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateRollupGroupingFAFScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	caseScenario, err := scenarioForCase(scenario, rollupGroupingFAFCase)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	carSchema, err := esper.RegisterStruct[rollupGroupingFAFCarEvent](env, "SupportCarEvent")
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateNamedWindow(env, "CarWindow", carSchema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
		return compat.Trace{}, err
	}
	carSource := esper.From[rollupGroupingFAFCarEvent](env, "SupportCarEvent")
	insertPlan, err := env.Build(esper.OnEvent(carSource).
		InsertIntoNamedWindow("CarWindow", esper.CopyMatchingFields()).
		Query(esper.StatementName("insert")))
	if err != nil {
		return compat.Trace{}, err
	}
	name := esper.Field[rollupGroupingFAFCarEvent, string]("name")
	place := esper.Field[rollupGroupingFAFCarEvent, string]("place")
	count := esper.Field[rollupGroupingFAFCarEvent, int]("count")
	fafQuery := esper.FromNamedWindowAs[rollupGroupingFAFCarEvent](env, "CarWindow").
		GroupByGroupingSets(
			esper.GroupingSet(name, place), esper.GroupingSet(name), esper.GroupingSet(place), esper.GroupingSet(),
		).
		Select(
			esper.Alias("name", name),
			esper.Alias("place", place),
			esper.Alias("sum(count)", esper.Sum[int](count)),
			esper.Alias("grouping(name)", esper.Grouping(name)),
			esper.Alias("grouping(place)", esper.Grouping(place)),
			esper.Alias("gid", esper.GroupingID(name, place)),
		).
		Query(esper.StatementName("s0"))
	fafPlan, err := env.Build(fafQuery)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()), esper.WithRuntimeURI(rollupGroupingFAFJavaRuntimeIDs[0]))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, insertPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	if statements := deployment.Statements(); len(statements) != 1 || statements[0].Name() != "insert" {
		return compat.Trace{}, fmt.Errorf("expected one CarWindow insert statement")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	fafCount := 0
	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			payload, err := decodeRollupGroupingFAFPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "faf":
			if step.Statement != "s0" || fafCount != 0 {
				return trace, fmt.Errorf("rollup-grouping-faf has unexpected FAF statement %q", step.Statement)
			}
			result, err := engine.ExecuteFireAndForget(ctx, fafPlan)
			if err != nil {
				return trace, err
			}
			newRows := compat.NormalizeResults(result.Batch.New)
			if newRows == nil {
				newRows = []compat.ResultRecord{}
			}
			oldRows := compat.NormalizeResults(result.Batch.Old)
			if oldRows == nil {
				oldRows = []compat.ResultRecord{}
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      rollupGroupingFAFCase,
				Operation: "faf",
				Statement: step.Statement,
				Sequence:  0,
				Time:      currentTimeString(engine),
				New:       newRows,
				Old:       oldRows,
			})
			fafCount++
		default:
			return trace, fmt.Errorf("unsupported rollup-grouping-faf step op %q", step.Op)
		}
	}
	if fafCount != 1 || len(trace.Records) != 1 {
		return trace, fmt.Errorf("rollup-grouping-faf expected one FAF record, got %d", fafCount)
	}
	return trace, nil
}

func decodeRollupGroupingFAFPayload(step compat.Step) (any, error) {
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
	var event rollupGroupingFAFCarEvent
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
