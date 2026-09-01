package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetAggregateWindowBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type resultsetAggregateWindowTrigger struct {
	ID int `esper:"id"`
}

const (
	resultsetAggregateWindowJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetAggregateWindowID          = "resultset-aggregate-window"
	resultsetAggregateWindowDescription = "ResultSetAggregationMethodWindow ordinals 1-3: table window access, first/last property access and list reference."
	resultsetAggregateWindowSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregationMethodWindow.java"
	resultsetAggregateWindowInventoryID = "java-313287657d14b5c686f8"
)

var (
	resultsetAggregateWindowJavaRuntimeIDs = []string{
		"java-runtime-785f2999e48fbaa6eb77",
		"java-runtime-9ef8f9a367e788b5afec",
		"java-runtime-6652f083e2b0f3dffc04",
	}
	resultsetAggregateWindowJavaStaticIDs = []string{
		"java-f881d0116e155d3e9063",
		"java-804f7ea3bb20de0d35e8",
		"java-f1014305fc8b701e8ea0",
	}
	resultsetAggregateWindowJavaExecutions = []string{
		"ResultSetAggregateWindowTableAccess",
		"ResultSetAggregateWindowTableIdentWCount",
		"ResultSetAggregateWindowListReference",
	}
	resultsetAggregateWindowCases = []string{
		"table-access",
		"table-ident-count",
		"table-list-reference",
	}
	resultsetAggregateWindowOrdinals = []int{1, 2, 3}
	resultsetAggregateWindowEPLs     = []string{
		"create table MyTable(windowcol window(*) @type('SupportBean')); into table MyTable select window(*) as windowcol from SupportBean#length(2); @name('s0') select MyTable.windowcol.first() as c0, MyTable.windowcol.last() as c1 from SupportBean_S0",
		"create table MyTable(windowcol window(*) @type('SupportBean')); into table MyTable select window(*) as windowcol from SupportBean; @name('s0') select windowcol.first(intPrimitive) as c0, windowcol.last(intPrimitive) as c1, windowcol.countEvents() as c2 from SupportBean_S0, MyTable",
		"create table MyTable(windowcol window(*) @type('SupportBean')); into table MyTable select window(*) as windowcol from SupportBean; @name('s0') select MyTable.windowcol.listReference() as collref from SupportBean_S0",
	}
)

type resultsetAggregateWindowExpectedEvent struct {
	theString    string
	intPrimitive int
}

var resultsetAggregateWindowTableAccessEvents = []resultsetAggregateWindowExpectedEvent{
	{theString: "E1", intPrimitive: 10},
	{theString: "E2", intPrimitive: 20},
	{theString: "E3", intPrimitive: 0},
}

var resultsetAggregateWindowTableIdentEvents = []resultsetAggregateWindowExpectedEvent{
	{theString: "E1", intPrimitive: 10},
	{theString: "E2", intPrimitive: 20},
	{theString: "E3", intPrimitive: 30},
}

var resultsetAggregateWindowListReferenceEvents = []resultsetAggregateWindowExpectedEvent{
	{theString: "E1", intPrimitive: 10},
	{theString: "E1", intPrimitive: 10},
}

func loadResultSetAggregateWindowScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetAggregateWindowID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetAggregateWindowID, err)
	}
	if err := rejectResultSetAggregateWindowDuplicateKeys(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateWindowID, err)
	}
	root, err := decodeResultSetAggregateWindowJSONObject(raw)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateWindowID, err)
	}
	if err := requireResultSetAggregateWindowFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	metadata := map[string]string{}
	for _, name := range []string{"version", "id", "description", "javaCommit", "javaSource"} {
		var value string
		if err := json.Unmarshal(root[name], &value); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario %s must be a string", name)
		}
		metadata[name] = value
	}
	if metadata["version"] != compat.ScenarioVersion || metadata["id"] != resultsetAggregateWindowID ||
		metadata["description"] != resultsetAggregateWindowDescription || metadata["javaCommit"] != resultsetAggregateWindowJavaCommit ||
		metadata["javaSource"] != resultsetAggregateWindowSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetAggregateWindowID)
	}
	if err := validateResultSetAggregateWindowStringArray(root["javaRuntimes"], resultsetAggregateWindowJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateWindowStringArray(root["javaNames"], resultsetAggregateWindowJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateWindowStringArray(root["javaStaticIds"], resultsetAggregateWindowJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateWindowStringArray(root["javaFlags"], nil, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	rawCases, err := decodeResultSetAggregateWindowJSONArray(root["cases"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario cases: %w", err)
	}
	if len(rawCases) != len(resultsetAggregateWindowCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly three cases", resultsetAggregateWindowID)
	}
	for index, rawCase := range rawCases {
		object, err := decodeResultSetAggregateWindowJSONObject(rawCase)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultSetAggregateWindowFields(object, "case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
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
			return compat.Scenario{}, fmt.Errorf("scenario case %d metadata has invalid types", index)
		}
		if entry.Case != resultsetAggregateWindowCases[index] || entry.Ordinal != resultsetAggregateWindowOrdinals[index] ||
			entry.RuntimeID != resultsetAggregateWindowJavaRuntimeIDs[index] || entry.ExecutionName != resultsetAggregateWindowJavaExecutions[index] ||
			entry.Observation != "listener" || entry.IteratorSnapshots != 0 || entry.EPL != resultsetAggregateWindowEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetAggregateWindowID, index)
		}
	}

	rawSteps, err := decodeResultSetAggregateWindowJSONArray(root["steps"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario steps: %w", err)
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		object, err := decodeResultSetAggregateWindowJSONObject(rawStep)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if object["op"] == nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d is missing op", index)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		switch operation {
		case "case":
			if err := requireResultSetAggregateWindowFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireResultSetAggregateWindowFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata["version"], ID: metadata["id"], Steps: steps}
	if err := validateResultSetAggregateWindowScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func runResultSetAggregateWindowScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateWindowScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultsetAggregateWindowCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultSetAggregateWindowCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetAggregateWindowID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetAggregateWindowID, scenario.ID)
	}
	return trace, nil
}

func runResultSetAggregateWindowCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateWindowBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetAggregateWindowTrigger](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateTable(env, "MyTable", []esper.TableColumn{
		esper.TableColumnOf[esper.WindowAccessValue[esper.Event]]("windowcol"),
	}); err != nil {
		return compat.Trace{}, err
	}

	eventValue := esper.EventValue[esper.Event]()
	window := esper.WindowAccessBy[esper.Event](eventValue)
	source := esper.From[resultsetAggregateWindowBean](env, "SupportBean")
	if caseName == "table-access" {
		source = source.Window(esper.LengthWindow(2))
	} else {
		source = source.Window(esper.KeepAll())
	}
	aggregatePlan, err := env.Build(source.Aggregate(
		esper.Alias("windowcol", window),
	).IntoTable("MyTable", esper.StatementName("aggregate")))
	if err != nil {
		return compat.Trace{}, err
	}

	access := esper.TableField[esper.WindowAccessValue[esper.Event]]("windowcol")
	selections := make([]esper.Selection, 0, 3)
	switch caseName {
	case "table-access":
		selections = append(selections,
			esper.Alias("c0", esper.Method[esper.Event](access, "First")),
			esper.Alias("c1", esper.Method[esper.Event](access, "Last")),
		)
	case "table-ident-count":
		first := esper.Property[int](esper.Method[esper.Event](access, "First"), "intPrimitive")
		last := esper.Property[int](esper.Method[esper.Event](access, "Last"), "intPrimitive")
		selections = append(selections,
			esper.Alias("c0", first),
			esper.Alias("c1", last),
			esper.Alias("c2", esper.Method[int64](access, "CountEvents")),
		)
	case "table-list-reference":
		selections = append(selections, esper.Alias("collref", esper.Method[[]esper.Event](access, "Values")))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported %s case %q", resultsetAggregateWindowID, caseName)
	}
	triggerPlan, err := env.Build(esper.OnEvent(esper.From[resultsetAggregateWindowTrigger](env, "SupportBean_S0")).
		SelectFromTableWhere("MyTable", esper.Literal(true), selections...).
		Query(esper.StatementName("s0")))
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateWindowRuntimeID(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := engine.Deploy(ctx, aggregatePlan); err != nil {
		return compat.Trace{}, err
	}
	deployment, err := engine.Deploy(ctx, triggerPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one trigger statement, got %d", len(statements))
	}
	statement := statements[0]
	trace, err := compat.ReplayWithStatements(ctx, engine, statement, scenario, decodeResultSetAggregateWindowPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown %s statement %q", resultsetAggregateWindowID, name)
		}
		return statement, nil
	})
	if err != nil {
		return compat.Trace{}, err
	}
	if err := validateResultSetAggregateWindowTrace(trace, caseName); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

func resultsetAggregateWindowRuntimeID(caseName string) string {
	for index, candidate := range resultsetAggregateWindowCases {
		if candidate == caseName {
			return resultsetAggregateWindowJavaRuntimeIDs[index]
		}
	}
	return "parity-" + resultsetAggregateWindowID + "-" + caseName
}

func validateResultSetAggregateWindowScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetAggregateWindowID {
		return fmt.Errorf("%s scenario has unsupported id %q", resultsetAggregateWindowID, scenario.ID)
	}
	expectedSends := []int{7, 4, 3}
	stepIndex := 0
	for caseIndex := range resultsetAggregateWindowCases {
		caseName := resultsetAggregateWindowCases[caseIndex]
		if stepIndex >= len(scenario.Steps) {
			return fmt.Errorf("%s scenario is missing case marker %q", resultsetAggregateWindowID, caseName)
		}
		marker := scenario.Steps[stepIndex]
		if marker.Op != "case" || marker.Case != caseName {
			return fmt.Errorf("%s scenario case marker %d is not pinned to %q", resultsetAggregateWindowID, caseIndex, caseName)
		}
		stepIndex++
		if stepIndex+expectedSends[caseIndex] > len(scenario.Steps) {
			return fmt.Errorf("%s scenario case %q is missing sends", resultsetAggregateWindowID, caseName)
		}
		for sendIndex := range expectedSends[caseIndex] {
			if scenario.Steps[stepIndex+sendIndex].Op != "send" {
				return fmt.Errorf("%s scenario case %q contains an unexpected case marker", resultsetAggregateWindowID, caseName)
			}
		}
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return fmt.Errorf("%s scenario: %w", resultsetAggregateWindowID, err)
		}
		if err := validateResultSetAggregateWindowCaseSteps(caseScenario, caseIndex, caseName); err != nil {
			return err
		}
		stepIndex += expectedSends[caseIndex]
	}
	if stepIndex != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps after the final case", resultsetAggregateWindowID)
	}
	return nil
}

func validateResultSetAggregateWindowCaseSteps(scenario compat.Scenario, index int, caseName string) error {
	if len(scenario.Steps) == 0 || scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != caseName {
		return fmt.Errorf("%s case %q must start with its case marker", resultsetAggregateWindowID, caseName)
	}
	var expected []resultsetAggregateWindowExpectedEvent
	var expectedTypes []string
	switch index {
	case 0:
		expected = resultsetAggregateWindowTableAccessEvents
		expectedTypes = []string{"SupportBean_S0", "SupportBean", "SupportBean_S0", "SupportBean", "SupportBean_S0", "SupportBean", "SupportBean_S0"}
	case 1:
		expected = resultsetAggregateWindowTableIdentEvents
		expectedTypes = []string{"SupportBean", "SupportBean", "SupportBean", "SupportBean_S0"}
	case 2:
		expected = resultsetAggregateWindowListReferenceEvents
		expectedTypes = []string{"SupportBean", "SupportBean", "SupportBean_S0"}
	default:
		return fmt.Errorf("%s has unsupported case index %d", resultsetAggregateWindowID, index)
	}
	if len(scenario.Steps)-1 != len(expectedTypes) {
		return fmt.Errorf("%s case %q has %d sends, want %d", resultsetAggregateWindowID, caseName, len(scenario.Steps)-1, len(expectedTypes))
	}
	beanIndex := 0
	for stepIndex, step := range scenario.Steps[1:] {
		if step.Op != "send" || step.Case != "" || step.EventType != expectedTypes[stepIndex] {
			return fmt.Errorf("%s case %q step %d is not the pinned %s send", resultsetAggregateWindowID, caseName, stepIndex+1, expectedTypes[stepIndex])
		}
		switch step.EventType {
		case "SupportBean":
			if beanIndex >= len(expected) {
				return fmt.Errorf("%s case %q has too many SupportBean sends", resultsetAggregateWindowID, caseName)
			}
			actual, err := decodeResultSetAggregateWindowBeanPayload(step)
			if err != nil {
				return fmt.Errorf("%s case %q event %d: %w", resultsetAggregateWindowID, caseName, stepIndex, err)
			}
			if actual != expected[beanIndex] {
				return fmt.Errorf("%s case %q event %d = %#v, want %#v", resultsetAggregateWindowID, caseName, stepIndex, actual, expected[beanIndex])
			}
			beanIndex++
		case "SupportBean_S0":
			var payload map[string]json.RawMessage
			if _, err := decodeResultSetAggregateWindowPayloadObject(step.Payload, &payload); err != nil {
				return fmt.Errorf("%s case %q trigger: %w", resultsetAggregateWindowID, caseName, err)
			}
			if len(payload) != 1 || string(bytes.TrimSpace(payload["id"])) != "-1" {
				return fmt.Errorf("%s case %q trigger id is not pinned to -1", resultsetAggregateWindowID, caseName)
			}
		}
	}
	if beanIndex != len(expected) {
		return fmt.Errorf("%s case %q has %d bean sends, want %d", resultsetAggregateWindowID, caseName, beanIndex, len(expected))
	}
	return nil
}

func validateResultSetAggregateWindowTrace(trace compat.Trace, caseName string) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != resultsetAggregateWindowID {
		return fmt.Errorf("%s trace identity is not pinned", resultsetAggregateWindowID)
	}
	expected := resultsetAggregateWindowExpectedRows(caseName)
	if len(trace.Records) != len(expected) {
		return fmt.Errorf("%s case %q trace has %d records, want %d", resultsetAggregateWindowID, caseName, len(trace.Records), len(expected))
	}
	for index, record := range trace.Records {
		if record.Case != caseName || record.Operation != "listener" || record.Statement != "s0" ||
			record.Sequence != uint64(index+1) || record.Time != "1970-01-01T00:00:00Z" || len(record.New) != 1 || len(record.Old) != 0 {
			return fmt.Errorf("%s case %q trace record %d metadata is not pinned", resultsetAggregateWindowID, caseName, index)
		}
		row := record.New[0]
		if row.Kind != "row" || len(row.Fields) != len(expected[index]) {
			return fmt.Errorf("%s case %q trace record %d row shape is not pinned", resultsetAggregateWindowID, caseName, index)
		}
		for name, want := range expected[index] {
			if got := row.Fields[name]; !reflect.DeepEqual(got, want) {
				return fmt.Errorf("%s case %q trace record %d field %q = %#v, want %#v", resultsetAggregateWindowID, caseName, index, name, got, want)
			}
		}
	}
	return nil
}

func resultsetAggregateWindowExpectedRows(caseName string) []map[string]any {
	event := func(value resultsetAggregateWindowExpectedEvent) map[string]any {
		return map[string]any{"kind": "row", "fields": map[string]any{
			"theString":    value.theString,
			"intPrimitive": value.intPrimitive,
		}}
	}
	null := map[string]any{"state": "null"}
	switch caseName {
	case "table-access":
		return []map[string]any{
			{"c0": null, "c1": null},
			{"c0": event(resultsetAggregateWindowTableAccessEvents[0]), "c1": event(resultsetAggregateWindowTableAccessEvents[0])},
			{"c0": event(resultsetAggregateWindowTableAccessEvents[0]), "c1": event(resultsetAggregateWindowTableAccessEvents[1])},
			{"c0": event(resultsetAggregateWindowTableAccessEvents[1]), "c1": event(resultsetAggregateWindowTableAccessEvents[2])},
		}
	case "table-ident-count":
		return []map[string]any{{"c0": 10, "c1": 30, "c2": int64(3)}}
	case "table-list-reference":
		return []map[string]any{{"collref": []any{event(resultsetAggregateWindowListReferenceEvents[0]), event(resultsetAggregateWindowListReferenceEvents[1])}}}
	default:
		return nil
	}
}

func decodeResultSetAggregateWindowPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		value, err := decodeResultSetAggregateWindowBeanPayload(step)
		if err != nil {
			return nil, err
		}
		return resultsetAggregateWindowBean{TheString: value.theString, IntPrimitive: value.intPrimitive}, nil
	case "SupportBean_S0":
		payload, err := decodeResultSetAggregateWindowPayloadObject(step.Payload, nil)
		if err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		if err := requireResultSetAggregateWindowFields(payload, "id"); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		id, err := decodeResultSetAggregateWindowInteger(payload["id"], "id")
		if err != nil {
			return nil, err
		}
		return resultsetAggregateWindowTrigger{ID: id}, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetAggregateWindowID, step.EventType)
	}
}

func decodeResultSetAggregateWindowBeanPayload(step compat.Step) (resultsetAggregateWindowExpectedEvent, error) {
	if step.EventType != "SupportBean" {
		return resultsetAggregateWindowExpectedEvent{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	payload, err := decodeResultSetAggregateWindowPayloadObject(step.Payload, nil)
	if err != nil {
		return resultsetAggregateWindowExpectedEvent{}, fmt.Errorf("payload must be an object: %w", err)
	}
	if err := requireResultSetAggregateWindowFields(payload, "theString", "intPrimitive"); err != nil {
		return resultsetAggregateWindowExpectedEvent{}, fmt.Errorf("payload: %w", err)
	}
	var value resultsetAggregateWindowExpectedEvent
	if err := json.Unmarshal(payload["theString"], &value.theString); err != nil {
		return resultsetAggregateWindowExpectedEvent{}, fmt.Errorf("payload theString must be a string")
	}
	value.intPrimitive, err = decodeResultSetAggregateWindowInteger(payload["intPrimitive"], "intPrimitive")
	if err != nil {
		return resultsetAggregateWindowExpectedEvent{}, err
	}
	return value, nil
}

func decodeResultSetAggregateWindowPayloadObject(raw json.RawMessage, target *map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	object, err := decodeResultSetAggregateWindowJSONObject(raw)
	if err != nil {
		return nil, err
	}
	if target != nil {
		*target = object
	}
	return object, nil
}

func decodeResultSetAggregateWindowInteger(raw json.RawMessage, name string) (int, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	number, ok := token.(json.Number)
	if err != nil || !ok || number == "" {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	value, err := strconv.ParseInt(string(number), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	return int(value), nil
}

func rejectResultSetAggregateWindowDuplicateKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkResultSetAggregateWindowJSON(decoder); err != nil {
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

func walkResultSetAggregateWindowJSON(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch token {
	case json.Delim('{'):
		seen := map[string]struct{}{}
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
			if err := walkResultSetAggregateWindowJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("JSON object is not closed")
		}
	case json.Delim('['):
		for decoder.More() {
			if err := walkResultSetAggregateWindowJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return fmt.Errorf("JSON array is not closed")
		}
	case json.Delim(']'), json.Delim('}'):
		return fmt.Errorf("unexpected JSON delimiter %q", token)
	}
	return nil
}

func decodeResultSetAggregateWindowJSONObject(raw []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("JSON value must be an object")
	}
	object := make(map[string]json.RawMessage)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, fmt.Errorf("JSON object key is not a string")
		}
		if _, exists := object[key]; exists {
			return nil, fmt.Errorf("JSON object contains duplicate field %q", key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		object[key] = value
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return nil, fmt.Errorf("JSON object is not closed")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("JSON value contains trailing data")
		}
		return nil, err
	}
	return object, nil
}

func decodeResultSetAggregateWindowJSONArray(raw json.RawMessage) ([]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		return nil, fmt.Errorf("JSON value must be an array")
	}
	values := make([]json.RawMessage, 0)
	for decoder.More() {
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim(']') {
		return nil, fmt.Errorf("JSON array is not closed")
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

func requireResultSetAggregateWindowFields(object map[string]json.RawMessage, names ...string) error {
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

func validateResultSetAggregateWindowStringArray(raw json.RawMessage, expected []string, name string) error {
	values, err := decodeResultSetAggregateWindowJSONArray(raw)
	if err != nil {
		return fmt.Errorf("%s must be an array: %w", name, err)
	}
	if len(values) != len(expected) {
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
