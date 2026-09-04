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
	resultsetOrderbySelfJoinID          = "resultset-orderby-self-join"
	resultsetOrderbySelfJoinDescription = "ResultSetOrderBySelfJoin ordinals 0: three-way self-join with ungrouped count and order-by on a join field."
	resultsetOrderbySelfJoinJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOrderbySelfJoinSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderBySelfJoin.java"
)

var (
	resultsetOrderbySelfJoinJavaSources = []string{
		resultsetOrderbySelfJoinSource,
	}
	resultsetOrderbySelfJoinJavaRuntimeIDs = []string{
		"java-runtime-74b0c1ce48febfe83007",
	}
	resultsetOrderbySelfJoinJavaExecutions = []string{
		"ResultSetOrderBySelfJoinSimple",
	}
	resultsetOrderbySelfJoinJavaStaticIDs = []string{
		"java-7cbb50aac0764e25b3bc",
	}
	resultsetOrderbySelfJoinCases = []string{
		"selfjoin",
	}
	resultsetOrderbySelfJoinOrdinals = []int{0}
	resultsetOrderbySelfJoinEPLs     = []string{
		"@name('s0') select c1.event_criteria_id as ecid, c1.priority as priority, c2.priority as prio, cast(count(*), int) as cnt from SupportHierarchyEvent#lastevent as c1, SupportHierarchyEvent#groupwin(event_criteria_id)#lastevent as c2, SupportHierarchyEvent#groupwin(event_criteria_id)#lastevent as p where c2.event_criteria_id in (c1.event_criteria_id,2,1) and p.event_criteria_id in (c1.parent_event_criteria_id, c1.event_criteria_id) order by c2.priority asc",
	}
)

type resultsetOrderbySelfJoinHierarchy struct {
	EventCriteriaID       int  `esper:"event_criteria_id"`
	Priority              int  `esper:"priority"`
	ParentEventCriteriaID *int `esper:"parent_event_criteria_id"`
}

var resultsetOrderbySelfJoinEvents = []resultsetOrderbySelfJoinHierarchy{
	{EventCriteriaID: 1, Priority: 1, ParentEventCriteriaID: nil},
	{EventCriteriaID: 3, Priority: 2, ParentEventCriteriaID: intPtr(2)},
	{EventCriteriaID: 3, Priority: 2, ParentEventCriteriaID: intPtr(2)},
}

func intPtr(value int) *int { return &value }

func loadResultsetOrderbySelfJoinScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOrderbySelfJoinID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOrderbySelfJoinID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOrderbySelfJoinID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOrderbySelfJoinID, err)
	}
	if err := requireResultsetOrderbySelfJoinFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOrderbySelfJoinID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetOrderbySelfJoinID ||
		metadata.Description != resultsetOrderbySelfJoinDescription ||
		metadata.JavaCommit != resultsetOrderbySelfJoinJavaCommit ||
		metadata.JavaSource != resultsetOrderbySelfJoinSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOrderbySelfJoinID)
	}
	if err := validateResultsetOrderbySelfJoinStringArray(root["javaRuntimes"], resultsetOrderbySelfJoinJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbySelfJoinStringArray(root["javaNames"], resultsetOrderbySelfJoinJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbySelfJoinStringArray(root["javaStaticIds"], resultsetOrderbySelfJoinJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbySelfJoinStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetOrderbySelfJoinCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly one case", resultsetOrderbySelfJoinID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetOrderbySelfJoinFields(object,
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
		if definition.Case != resultsetOrderbySelfJoinCases[index] ||
			definition.Ordinal != resultsetOrderbySelfJoinOrdinals[index] ||
			definition.RuntimeID != resultsetOrderbySelfJoinJavaRuntimeIDs[0] ||
			definition.ExecutionName != resultsetOrderbySelfJoinJavaExecutions[0] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 1 ||
			definition.EPL != resultsetOrderbySelfJoinEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetOrderbySelfJoinID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 5 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly five steps", resultsetOrderbySelfJoinID)
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
			if err := requireResultsetOrderbySelfJoinFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireResultsetOrderbySelfJoinFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeResultsetOrderbySelfJoinPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		case "snapshot":
			if err := requireResultsetOrderbySelfJoinFields(object, "op", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step compat.Step
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != "s0" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshot is not pinned", index)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateResultsetOrderbySelfJoinScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetOrderbySelfJoinScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetOrderbySelfJoinID || len(scenario.Steps) != 5 {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetOrderbySelfJoinID)
	}
	index := 0
	if err := validateResultsetOrderbySelfJoinCaseMarker(scenario.Steps[index], resultsetOrderbySelfJoinCases[0]); err != nil {
		return fmt.Errorf("case marker: %w", err)
	}
	index++
	for eventIndex, expected := range resultsetOrderbySelfJoinEvents {
		if err := validateResultsetOrderbySelfJoinSendStep(scenario.Steps[index], expected); err != nil {
			return fmt.Errorf("event step %d: %w", eventIndex, err)
		}
		index++
	}
	if err := validateResultsetOrderbySelfJoinSnapshotStep(scenario.Steps[index]); err != nil {
		return fmt.Errorf("final snapshot: %w", err)
	}
	index++
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetOrderbySelfJoinID)
	}
	return nil
}

func validateResultsetOrderbySelfJoinCaseMarker(step compat.Step, expected string) error {
	if step.Op != "case" || step.Case != expected {
		return fmt.Errorf("must start case %q", expected)
	}
	return nil
}

func validateResultsetOrderbySelfJoinSendStep(step compat.Step, expected resultsetOrderbySelfJoinHierarchy) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportHierarchyEvent" {
		return fmt.Errorf("must send SupportHierarchyEvent")
	}
	bean, err := decodeResultsetOrderbySelfJoinHierarchy(step)
	if err != nil {
		return err
	}
	if bean.EventCriteriaID != expected.EventCriteriaID ||
		bean.Priority != expected.Priority ||
		!equalResultsetOrderbySelfJoinIntPtr(bean.ParentEventCriteriaID, expected.ParentEventCriteriaID) {
		return fmt.Errorf("hierarchy payload is not pinned")
	}
	return nil
}

func equalResultsetOrderbySelfJoinIntPtr(left, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func validateResultsetOrderbySelfJoinSnapshotStep(step compat.Step) error {
	if step.Op != "snapshot" || step.Case != "" || step.Statement != "s0" {
		return fmt.Errorf("must snapshot statement s0")
	}
	return nil
}

func runResultsetOrderbySelfJoinScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetOrderbySelfJoinScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	caseScenario, err := scenarioForCase(scenario, resultsetOrderbySelfJoinCases[0])
	if err != nil {
		return compat.Trace{}, err
	}
	caseTrace, err := runResultsetOrderbySelfJoinCase(ctx, caseScenario, resultsetOrderbySelfJoinCases[0])
	if err != nil {
		return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOrderbySelfJoinID, resultsetOrderbySelfJoinCases[0], err)
	}
	trace.Records = append(trace.Records, caseTrace.Records...)
	return trace, nil
}

func runResultsetOrderbySelfJoinCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOrderbySelfJoinHierarchy](env, "SupportHierarchyEvent"); err != nil {
		return compat.Trace{}, err
	}

	ecid := esper.Field[resultsetOrderbySelfJoinHierarchy, int]("event_criteria_id")

	c1 := esper.From[resultsetOrderbySelfJoinHierarchy](env, "SupportHierarchyEvent").Window(esper.LastEvent())
	c2 := esper.From[resultsetOrderbySelfJoinHierarchy](env, "SupportHierarchyEvent").
		Window(esper.GroupWindow(ecid, esper.LastEvent()))
	p := esper.From[resultsetOrderbySelfJoinHierarchy](env, "SupportHierarchyEvent").
		Window(esper.GroupWindow(ecid, esper.LastEvent()))

	where := esper.And(
		esper.In[int](esper.JoinField[int](1, "event_criteria_id"),
			esper.JoinField[int](0, "event_criteria_id"), esper.Literal(2), esper.Literal(1)),
		esper.InOf(esper.JoinField[int](2, "event_criteria_id"),
			esper.JoinField[*int](0, "parent_event_criteria_id"), esper.JoinField[int](0, "event_criteria_id")),
	)

	joinPriority := esper.JoinField[int](1, "priority")
	query := esper.JoinMany(
		esper.JoinSource(c1),
		esper.JoinSource(c2),
		esper.JoinSource(p),
	).Aggregate().Where(where).Select(
		esper.Alias("ecid", esper.JoinField[int](0, "event_criteria_id")),
		esper.Alias("priority", esper.JoinField[int](0, "priority")),
		esper.Alias("prio", joinPriority),
		esper.Alias("cnt", esper.Cast[int64, int](esper.CountAll())),
	).Query(
		esper.StatementName("s0"),
		esper.OrderBy(esper.Ascending(joinPriority)),
	)
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithRuntimeURI(resultsetOrderbySelfJoinJavaRuntimeIDs[0]))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("%s case %q deployed %d statements", resultsetOrderbySelfJoinID, caseName, len(statements))
	}
	statement := statements[0]
	if caseName == "selfjoin" {
		return runResultsetOrderbySelfJoinIteratorReplay(ctx, engine, statement, scenario)
	}
	return compat.ReplayWithStatements(ctx, engine, statement, scenario,
		decodeResultsetOrderbySelfJoinPayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetOrderbySelfJoinID, name)
			}
			return statement, nil
		})
}

// runResultsetOrderbySelfJoinIteratorReplay mirrors the Java execution: the
// three continuous join deliveries are recorded as listener batches and the
// single pinned statement-iterator snapshot is appended after the sends.
func runResultsetOrderbySelfJoinIteratorReplay(ctx context.Context, engine *esper.Engine, statement *esper.Statement, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	var listenerRecords []compat.TraceRecord
	_, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		rows := compat.NormalizeResults(batch.New)
		listenerRecords = append(listenerRecords, compat.TraceRecord{
			Case:      "selfjoin",
			Operation: "listener",
			Statement: statement.Name(),
			Time:      formatEngineTime(engine),
			New:       rows,
		})
		return nil
	})
	if err != nil {
		return trace, err
	}
	snapshots := 0
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "send":
			payload, err := decodeResultsetOrderbySelfJoinPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "snapshot":
			if step.Statement != statement.Name() {
				return trace, fmt.Errorf("%s snapshot metadata is not pinned", resultsetOrderbySelfJoinID)
			}
			for index := range listenerRecords {
				listenerRecords[index].Sequence = uint64(index + 1)
				trace.Records = append(trace.Records, listenerRecords[index])
			}
			listenerRecords = nil
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return trace, err
			}
			rows := compat.NormalizeResults(result.Batch.New)
			if rows == nil {
				rows = []compat.ResultRecord{}
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      "selfjoin",
				Operation: "snapshot",
				Statement: statement.Name(),
				Sequence:  0,
				Time:      formatEngineTime(engine),
				New:       rows,
			})
			snapshots++
		default:
			return trace, fmt.Errorf("unsupported %s step op %q", resultsetOrderbySelfJoinID, step.Op)
		}
	}
	for index := range listenerRecords {
		listenerRecords[index].Sequence = uint64(index + 1)
		trace.Records = append(trace.Records, listenerRecords[index])
	}
	if snapshots != 1 {
		return trace, fmt.Errorf("%s produced %d snapshots, want one", resultsetOrderbySelfJoinID, snapshots)
	}
	return trace, nil
}

func formatEngineTime(engine *esper.Engine) string {
	return engine.Now().UTC().Format(time.RFC3339Nano)
}

func decodeResultsetOrderbySelfJoinPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportHierarchyEvent":
		return decodeResultsetOrderbySelfJoinHierarchy(step)
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOrderbySelfJoinID, step.EventType)
	}
}

func decodeResultsetOrderbySelfJoinHierarchy(step compat.Step) (resultsetOrderbySelfJoinHierarchy, error) {
	if step.EventType != "SupportHierarchyEvent" {
		return resultsetOrderbySelfJoinHierarchy{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOrderbySelfJoinHierarchy{}, err
	}
	if err := requireResultsetOrderbySelfJoinFields(fields, "event_criteria_id", "priority", "parent_event_criteria_id"); err != nil {
		return resultsetOrderbySelfJoinHierarchy{}, err
	}
	ecid, err := decodeResultsetOrderbySelfJoinInt(fields["event_criteria_id"], "event_criteria_id")
	if err != nil {
		return resultsetOrderbySelfJoinHierarchy{}, err
	}
	priority, err := decodeResultsetOrderbySelfJoinInt(fields["priority"], "priority")
	if err != nil {
		return resultsetOrderbySelfJoinHierarchy{}, err
	}
	parent, err := decodeResultsetOrderbySelfJoinNullableInt(fields["parent_event_criteria_id"], "parent_event_criteria_id")
	if err != nil {
		return resultsetOrderbySelfJoinHierarchy{}, err
	}
	return resultsetOrderbySelfJoinHierarchy{EventCriteriaID: ecid, Priority: priority, ParentEventCriteriaID: parent}, nil
}

func decodeResultsetOrderbySelfJoinInt(raw json.RawMessage, name string) (int, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var value int
	if err := decoder.Decode(&value); err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	return value, nil
}

func decodeResultsetOrderbySelfJoinNullableInt(raw json.RawMessage, name string) (*int, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	value, err := decodeResultsetOrderbySelfJoinInt(raw, name)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func requireResultsetOrderbySelfJoinFields(object map[string]json.RawMessage, names ...string) error {
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

func validateResultsetOrderbySelfJoinStringArray(raw json.RawMessage, expected []string, name string) error {
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
