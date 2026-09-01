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

type resultsetAggregateSortedGroupedBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type resultsetAggregateSortedGroupedTrigger struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

const (
	resultsetAggregateSortedGroupedID          = "resultset-aggregate-sorted-grouped"
	resultsetAggregateSortedGroupedJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetAggregateSortedGroupedDescription = "ResultSetAggregationMethodSorted ordinal 11: grouped table-backed sorted collection selector with first/last key access."
	resultsetAggregateSortedGroupedSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregationMethodSorted.java"
	resultsetAggregateSortedGroupedRuntimeID   = "java-runtime-3ffe177eb6a90acb699a"
	resultsetAggregateSortedGroupedStaticID    = "java-5a1abe9fb7420df7353d"
	resultsetAggregateSortedGroupedExecution   = "ResultSetAggregateSortedGrouped"
	resultsetAggregateSortedGroupedCase        = "grouped"
	resultsetAggregateSortedGroupedEPL         = "create table MyTable(k0 string primary key, sortcol sorted(intPrimitive) @type('SupportBean'));\ninto table MyTable select sorted(*) as sortcol from SupportBean group by theString;\n@name('s0') select MyTable[p00].sortcol.sorted() as sortcol,MyTable[p00].sortcol.firstKey() as firstkey,MyTable[p00].sortcol.lastKey() as lastkey from SupportBean_S0"
)

var (
	resultsetAggregateSortedGroupedJavaRuntimeIDs = []string{resultsetAggregateSortedGroupedRuntimeID}
	resultsetAggregateSortedGroupedJavaSources    = []string{resultsetAggregateSortedGroupedSource}
	resultsetAggregateSortedGroupedJavaExecutions = []string{resultsetAggregateSortedGroupedExecution}
)

type resultsetAggregateSortedGroupedExpectedEvent struct {
	name string
	key  int
}

var resultsetAggregateSortedGroupedExpectedEvents = []resultsetAggregateSortedGroupedExpectedEvent{
	{name: "A", key: 10},
	{name: "A", key: 20},
	{name: "A", key: 10},
	{name: "A", key: 21},
	{name: "B", key: 100},
}

func resultsetAggregateSortedGroupedNull() map[string]any {
	return map[string]any{"state": "null"}
}

func loadResultSetAggregateSortedGroupedScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetAggregateSortedGroupedID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetAggregateSortedGroupedID, err)
	}
	if err := rejectResultSetAggregateSortedGroupedDuplicateKeys(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateSortedGroupedID, err)
	}
	root, err := decodeResultSetAggregateSortedGroupedJSONObject(raw)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateSortedGroupedID, err)
	}
	if err := requireResultSetAggregateSortedGroupedFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	metadata := make(map[string]string, 5)
	for _, name := range []string{"version", "id", "description", "javaCommit", "javaSource"} {
		value, err := decodeResultSetAggregateSortedGroupedString(root[name], name)
		if err != nil {
			return compat.Scenario{}, err
		}
		metadata[name] = value
	}
	if metadata["version"] != compat.ScenarioVersion || metadata["id"] != resultsetAggregateSortedGroupedID ||
		metadata["description"] != resultsetAggregateSortedGroupedDescription ||
		metadata["javaCommit"] != resultsetAggregateSortedGroupedJavaCommit ||
		metadata["javaSource"] != resultsetAggregateSortedGroupedSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetAggregateSortedGroupedID)
	}
	if err := validateResultSetAggregateSortedGroupedStringArray(root["javaRuntimes"], resultsetAggregateSortedGroupedJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateSortedGroupedStringArray(root["javaNames"], resultsetAggregateSortedGroupedJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateSortedGroupedStringArray(root["javaStaticIds"], []string{resultsetAggregateSortedGroupedStaticID}, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateSortedGroupedStringArray(root["javaFlags"], nil, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	rawCases, err := decodeResultSetAggregateSortedGroupedJSONArray(root["cases"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario cases: %w", err)
	}
	if len(rawCases) != 1 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly one case", resultsetAggregateSortedGroupedID)
	}
	caseObject, err := decodeResultSetAggregateSortedGroupedJSONObject(rawCases[0])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	if err := requireResultSetAggregateSortedGroupedFields(caseObject,
		"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	caseName, err := decodeResultSetAggregateSortedGroupedString(caseObject["case"], "case")
	if err != nil {
		return compat.Scenario{}, err
	}
	ordinal, err := decodeResultSetAggregateSortedGroupedInt(caseObject["ordinal"], "ordinal")
	if err != nil {
		return compat.Scenario{}, err
	}
	runtimeID, err := decodeResultSetAggregateSortedGroupedString(caseObject["runtimeId"], "runtimeId")
	if err != nil {
		return compat.Scenario{}, err
	}
	executionName, err := decodeResultSetAggregateSortedGroupedString(caseObject["executionName"], "executionName")
	if err != nil {
		return compat.Scenario{}, err
	}
	observation, err := decodeResultSetAggregateSortedGroupedString(caseObject["observation"], "observation")
	if err != nil {
		return compat.Scenario{}, err
	}
	iteratorSnapshots, err := decodeResultSetAggregateSortedGroupedInt(caseObject["iteratorSnapshots"], "iteratorSnapshots")
	if err != nil {
		return compat.Scenario{}, err
	}
	epl, err := decodeResultSetAggregateSortedGroupedString(caseObject["epl"], "epl")
	if err != nil {
		return compat.Scenario{}, err
	}
	if caseName != resultsetAggregateSortedGroupedCase || ordinal != 11 || runtimeID != resultsetAggregateSortedGroupedRuntimeID ||
		executionName != resultsetAggregateSortedGroupedExecution || observation != "listener" || iteratorSnapshots != 0 ||
		epl != resultsetAggregateSortedGroupedEPL {
		return compat.Scenario{}, fmt.Errorf("%s scenario case metadata is not pinned", resultsetAggregateSortedGroupedID)
	}

	rawSteps, err := decodeResultSetAggregateSortedGroupedJSONArray(root["steps"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario steps: %w", err)
	}
	if len(rawSteps) != 11 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly eleven steps", resultsetAggregateSortedGroupedID)
	}
	steps := make([]compat.Step, 0, len(rawSteps))
	for index, rawStep := range rawSteps {
		object, err := decodeResultSetAggregateSortedGroupedJSONObject(rawStep)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		op, err := decodeResultSetAggregateSortedGroupedString(object["op"], fmt.Sprintf("scenario step %d op", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		switch op {
		case "case":
			if err := requireResultSetAggregateSortedGroupedFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			caseName, err := decodeResultSetAggregateSortedGroupedString(object["case"], fmt.Sprintf("scenario step %d case", index))
			if err != nil {
				return compat.Scenario{}, err
			}
			steps = append(steps, compat.Step{Op: op, Case: caseName})
		case "send":
			if err := requireResultSetAggregateSortedGroupedFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			eventType, err := decodeResultSetAggregateSortedGroupedString(object["eventType"], fmt.Sprintf("scenario step %d eventType", index))
			if err != nil {
				return compat.Scenario{}, err
			}
			if _, err := decodeResultSetAggregateSortedGroupedJSONObject(object["payload"]); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			steps = append(steps, compat.Step{Op: op, EventType: eventType, Payload: append(json.RawMessage(nil), object["payload"]...)})
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, op)
		}
	}
	scenario := compat.Scenario{Version: metadata["version"], ID: metadata["id"], Steps: steps}
	if err := validateResultSetAggregateSortedGroupedScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func runResultSetAggregateSortedGroupedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateSortedGroupedScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateSortedGroupedBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetAggregateSortedGroupedTrigger](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateTable(env, "MyTable", []esper.TableColumn{
		esper.PrimaryKeyColumn[string]("k0"),
		esper.TableColumnOf[esper.SortedAccessValue[int, esper.Event]]("sortcol"),
	}); err != nil {
		return compat.Trace{}, err
	}

	theString := esper.Field[resultsetAggregateSortedGroupedBean, string]("theString")
	intPrimitive := esper.Field[resultsetAggregateSortedGroupedBean, int]("intPrimitive")
	sorted := esper.SortedAccessBy[esper.Event, int](esper.EventValue[esper.Event](), intPrimitive)
	aggregatePlan, err := env.Build(esper.From[resultsetAggregateSortedGroupedBean](env, "SupportBean").
		Window(esper.KeepAll()).
		GroupBy(theString).
		Select(esper.Alias("k0", theString), esper.Alias("sortcol", sorted)).
		IntoTable("MyTable", esper.StatementName("aggregate")))
	if err != nil {
		return compat.Trace{}, err
	}

	sortcol := esper.TableField[esper.SortedAccessValue[int, esper.Event]]("sortcol")
	triggerP00 := esper.Field[resultsetAggregateSortedGroupedTrigger, string]("p00")
	triggerPlan, err := env.Build(esper.OnEvent(esper.From[resultsetAggregateSortedGroupedTrigger](env, "SupportBean_S0")).
		SelectFromTable("MyTable", []esper.Expr{triggerP00},
			esper.Alias("sortcol", esper.Method[[]esper.Event](sortcol, "Sorted")),
			esper.Alias("firstkey", esper.Method[int](sortcol, "FirstKey")),
			esper.Alias("lastkey", esper.Method[int](sortcol, "LastKey")),
		).
		Query(esper.StatementName("s0")))
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateSortedGroupedRuntimeID),
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
	trace, err := compat.ReplayWithStatements(ctx, engine, statement, scenario,
		decodeResultSetAggregateSortedGroupedPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetAggregateSortedGroupedID, name)
			}
			return statement, nil
		})
	if err != nil {
		return compat.Trace{}, err
	}
	trace = normalizeResultSetAggregateSortedGroupedTrace(trace)
	if err := validateResultSetAggregateSortedGroupedTrace(trace); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

func normalizeResultSetAggregateSortedGroupedTrace(trace compat.Trace) compat.Trace {
	for recordIndex := range trace.Records {
		record := &trace.Records[recordIndex]
		for _, rows := range [][]compat.ResultRecord{record.New, record.Old} {
			for rowIndex := range rows {
				if _, ok := rows[rowIndex].Fields["sortcol"]; ok {
					rows[rowIndex].Fields["sortcol"] = resultsetAggregateSortedGroupedNull()
				}
			}
		}
	}
	return trace
}

func validateResultSetAggregateSortedGroupedTrace(trace compat.Trace) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != resultsetAggregateSortedGroupedID {
		return fmt.Errorf("%s trace identity is not pinned", resultsetAggregateSortedGroupedID)
	}
	if len(trace.Records) != 5 {
		return fmt.Errorf("%s trace must contain exactly five records", resultsetAggregateSortedGroupedID)
	}
	want := []struct {
		first, last any
	}{
		{first: resultsetAggregateSortedGroupedNull(), last: resultsetAggregateSortedGroupedNull()},
		{first: 10, last: 20},
		{first: 10, last: 21},
		{first: 10, last: 21},
		{first: 100, last: 100},
	}
	for index, record := range trace.Records {
		if record.Case != resultsetAggregateSortedGroupedCase || record.Operation != "listener" || record.Statement != "s0" ||
			record.Sequence != uint64(index+1) || record.Time != "1970-01-01T00:00:00Z" || len(record.New) != 1 || len(record.Old) != 0 {
			return fmt.Errorf("%s trace record %d metadata is not pinned", resultsetAggregateSortedGroupedID, index)
		}
		row := record.New[0]
		if row.Kind != "row" || len(row.Fields) != 3 {
			return fmt.Errorf("%s trace record %d row shape is not pinned", resultsetAggregateSortedGroupedID, index)
		}
		if actual := row.Fields["sortcol"]; !reflect.DeepEqual(actual, resultsetAggregateSortedGroupedNull()) {
			return fmt.Errorf("%s trace record %d field %q = %#v, want null", resultsetAggregateSortedGroupedID, index, "sortcol", actual)
		}
		for name, expected := range map[string]any{"firstkey": want[index].first, "lastkey": want[index].last} {
			if actual := row.Fields[name]; !reflect.DeepEqual(actual, expected) {
				return fmt.Errorf("%s trace record %d field %q = %#v, want %#v", resultsetAggregateSortedGroupedID, index, name, actual, expected)
			}
		}
	}
	return nil
}

func decodeResultSetAggregateSortedGroupedPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		value, err := decodeResultSetAggregateSortedGroupedBeanPayload(step.Payload)
		if err != nil {
			return nil, err
		}
		return resultsetAggregateSortedGroupedBean{TheString: value.name, IntPrimitive: value.key}, nil
	case "SupportBean_S0":
		payload, err := decodeResultSetAggregateSortedGroupedJSONObject(step.Payload)
		if err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		if err := requireResultSetAggregateSortedGroupedFields(payload, "id", "p00"); err != nil {
			return nil, err
		}
		id, err := decodeResultSetAggregateSortedGroupedInt(payload["id"], "id")
		if err != nil {
			return nil, err
		}
		p00, err := decodeResultSetAggregateSortedGroupedString(payload["p00"], "p00")
		if err != nil {
			return nil, err
		}
		return resultsetAggregateSortedGroupedTrigger{ID: id, P00: p00}, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetAggregateSortedGroupedID, step.EventType)
	}
}

func decodeResultSetAggregateSortedGroupedBeanPayload(raw json.RawMessage) (resultsetAggregateSortedGroupedExpectedEvent, error) {
	payload, err := decodeResultSetAggregateSortedGroupedJSONObject(raw)
	if err != nil {
		return resultsetAggregateSortedGroupedExpectedEvent{}, fmt.Errorf("payload must be an object: %w", err)
	}
	if err := requireResultSetAggregateSortedGroupedFields(payload, "theString", "intPrimitive"); err != nil {
		return resultsetAggregateSortedGroupedExpectedEvent{}, err
	}
	name, err := decodeResultSetAggregateSortedGroupedString(payload["theString"], "theString")
	if err != nil {
		return resultsetAggregateSortedGroupedExpectedEvent{}, err
	}
	key, err := decodeResultSetAggregateSortedGroupedInt(payload["intPrimitive"], "intPrimitive")
	if err != nil {
		return resultsetAggregateSortedGroupedExpectedEvent{}, err
	}
	return resultsetAggregateSortedGroupedExpectedEvent{name: name, key: key}, nil
}

func validateResultSetAggregateSortedGroupedScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetAggregateSortedGroupedID {
		return fmt.Errorf("%s scenario has unsupported id %q", resultsetAggregateSortedGroupedID, scenario.ID)
	}
	if len(scenario.Steps) != 11 {
		return fmt.Errorf("%s scenario must contain exactly eleven steps", resultsetAggregateSortedGroupedID)
	}
	marker := scenario.Steps[0]
	if marker.Op != "case" || marker.Case != resultsetAggregateSortedGroupedCase {
		return fmt.Errorf("%s scenario must start with case %q", resultsetAggregateSortedGroupedID, resultsetAggregateSortedGroupedCase)
	}
	if err := validateResultSetAggregateSortedGroupedTrigger(scenario.Steps[1], "A"); err != nil {
		return fmt.Errorf("%s step 1: %w", resultsetAggregateSortedGroupedID, err)
	}
	for index, expected := range resultsetAggregateSortedGroupedExpectedEvents {
		stepIndex := index + 2
		if index == 2 {
			stepIndex = 5
		}
		if index == 3 {
			stepIndex = 6
		}
		if index == 4 {
			stepIndex = 8
		}
		step := scenario.Steps[stepIndex]
		if step.Op != "send" || step.Case != "" || step.EventType != "SupportBean" {
			return fmt.Errorf("%s step %d must be an unscoped SupportBean send", resultsetAggregateSortedGroupedID, stepIndex)
		}
		actual, err := decodeResultSetAggregateSortedGroupedBeanPayload(step.Payload)
		if err != nil {
			return fmt.Errorf("%s step %d: %w", resultsetAggregateSortedGroupedID, stepIndex, err)
		}
		if actual != expected {
			return fmt.Errorf("%s step %d = %#v, want %#v", resultsetAggregateSortedGroupedID, stepIndex, actual, expected)
		}
	}
	for _, trigger := range []struct {
		position int
		p00      string
	}{{position: 4, p00: "A"}, {position: 7, p00: "A"}, {position: 9, p00: "A"}, {position: 10, p00: "B"}} {
		if err := validateResultSetAggregateSortedGroupedTrigger(scenario.Steps[trigger.position], trigger.p00); err != nil {
			return fmt.Errorf("%s step %d: %w", resultsetAggregateSortedGroupedID, trigger.position, err)
		}
	}
	return nil
}

func validateResultSetAggregateSortedGroupedTrigger(step compat.Step, expectedP00 string) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportBean_S0" {
		return fmt.Errorf("must be an unscoped SupportBean_S0 send")
	}
	payload, err := decodeResultSetAggregateSortedGroupedJSONObject(step.Payload)
	if err != nil {
		return fmt.Errorf("trigger payload: %w", err)
	}
	if err := requireResultSetAggregateSortedGroupedFields(payload, "id", "p00"); err != nil {
		return err
	}
	id, err := decodeResultSetAggregateSortedGroupedInt(payload["id"], "id")
	if err != nil {
		return err
	}
	p00, err := decodeResultSetAggregateSortedGroupedString(payload["p00"], "p00")
	if err != nil {
		return err
	}
	if id != -1 || p00 != expectedP00 {
		return fmt.Errorf("trigger payload is not pinned")
	}
	return nil
}

func requireResultSetAggregateSortedGroupedFields(object map[string]json.RawMessage, names ...string) error {
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

func decodeResultSetAggregateSortedGroupedString(raw json.RawMessage, name string) (string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must be a string", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func decodeResultSetAggregateSortedGroupedInt(raw json.RawMessage, name string) (int, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	number, ok := token.(json.Number)
	if !ok || !resultsetAggregateSortedGroupedIntegerSyntax(string(number)) {
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

func resultsetAggregateSortedGroupedIntegerSyntax(text string) bool {
	if text == "0" {
		return true
	}
	if text == "" {
		return false
	}
	if text[0] == '-' {
		text = text[1:]
	}
	if text == "" || (len(text) > 1 && text[0] == '0') {
		return false
	}
	for _, character := range text {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validateResultSetAggregateSortedGroupedStringArray(raw json.RawMessage, expected []string, name string) error {
	values, err := decodeResultSetAggregateSortedGroupedJSONArray(raw)
	if err != nil {
		return fmt.Errorf("%s must be an array: %w", name, err)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index, rawValue := range values {
		value, err := decodeResultSetAggregateSortedGroupedString(rawValue, name)
		if err != nil || value != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}

func rejectResultSetAggregateSortedGroupedDuplicateKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkResultSetAggregateSortedGroupedJSON(decoder); err != nil {
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

func walkResultSetAggregateSortedGroupedJSON(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch token {
	case json.Delim('{'):
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
			if err := walkResultSetAggregateSortedGroupedJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("JSON object is not closed")
		}
	case json.Delim('['):
		for decoder.More() {
			if err := walkResultSetAggregateSortedGroupedJSON(decoder); err != nil {
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

func decodeResultSetAggregateSortedGroupedJSONObject(raw []byte) (map[string]json.RawMessage, error) {
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

func decodeResultSetAggregateSortedGroupedJSONArray(raw json.RawMessage) ([]json.RawMessage, error) {
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
