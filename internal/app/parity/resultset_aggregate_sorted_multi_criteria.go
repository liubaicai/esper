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

type resultsetAggregateSortedMultiCriteriaBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type resultsetAggregateSortedMultiCriteriaTrigger struct {
	ID int `esper:"id"`
}

const resultsetAggregateSortedMultiCriteriaJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateSortedMultiCriteriaJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregationMethodSorted.java",
}

var resultsetAggregateSortedMultiCriteriaJavaRuntimeIDs = []string{
	"java-runtime-dd79ba5aba4eb4ec0a1a",
}

var resultsetAggregateSortedMultiCriteriaJavaExecutions = []string{
	"ResultSetAggregateSortedMultiCriteria",
}

const resultsetAggregateSortedMultiCriteriaCase = "multi-criteria"

type resultsetAggregateSortedMultiCriteriaExpectedEvent struct {
	theString    string
	intPrimitive int
}

var resultsetAggregateSortedMultiCriteriaExpected = []resultsetAggregateSortedMultiCriteriaExpectedEvent{
	{theString: "E1a", intPrimitive: 1},
	{theString: "E1b", intPrimitive: 1},
	{theString: "E4b", intPrimitive: 4},
	{theString: "E6a", intPrimitive: 6},
	{theString: "E6b", intPrimitive: 6},
	{theString: "E8", intPrimitive: 8},
	{theString: "E9", intPrimitive: 9},
}

func loadResultSetAggregateSortedMultiCriteriaScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-sorted-multi-criteria scenario reader is required")
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read resultset-aggregate-sorted-multi-criteria scenario: %w", err)
	}
	root, err := decodeResultSetAggregateSortedMultiCriteriaJSONObject(data)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-aggregate-sorted-multi-criteria scenario: %w", err)
	}
	if err := requireResultSetAggregateSortedMultiCriteriaJSONFields(root, "version", "id", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	version, err := decodeResultSetAggregateSortedMultiCriteriaJSONString(root["version"], "version")
	if err != nil {
		return compat.Scenario{}, err
	}
	id, err := decodeResultSetAggregateSortedMultiCriteriaJSONString(root["id"], "id")
	if err != nil {
		return compat.Scenario{}, err
	}
	stepValues, err := decodeResultSetAggregateSortedMultiCriteriaJSONArray(root["steps"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario steps: %w", err)
	}
	if len(stepValues) != len(resultsetAggregateSortedMultiCriteriaExpected)+2 {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-sorted-multi-criteria scenario must contain one case, seven SupportBean sends, and one trigger")
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: make([]compat.Step, 0, len(stepValues))}
	for index, rawStep := range stepValues {
		stepObject, err := decodeResultSetAggregateSortedMultiCriteriaJSONObject(rawStep)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if index == 0 {
			if err := requireResultSetAggregateSortedMultiCriteriaJSONFields(stepObject, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			caseName, err := decodeResultSetAggregateSortedMultiCriteriaJSONString(stepObject["case"], fmt.Sprintf("scenario step %d case", index))
			if err != nil {
				return compat.Scenario{}, err
			}
			op, err := decodeResultSetAggregateSortedMultiCriteriaJSONString(stepObject["op"], fmt.Sprintf("scenario step %d op", index))
			if err != nil {
				return compat.Scenario{}, err
			}
			scenario.Steps = append(scenario.Steps, compat.Step{Op: op, Case: caseName})
			continue
		}
		if err := requireResultSetAggregateSortedMultiCriteriaJSONFields(stepObject, "op", "eventType", "payload"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		op, err := decodeResultSetAggregateSortedMultiCriteriaJSONString(stepObject["op"], fmt.Sprintf("scenario step %d op", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		eventType, err := decodeResultSetAggregateSortedMultiCriteriaJSONString(stepObject["eventType"], fmt.Sprintf("scenario step %d eventType", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		scenario.Steps = append(scenario.Steps, compat.Step{
			Op:        op,
			EventType: eventType,
			Payload:   append(json.RawMessage(nil), stepObject["payload"]...),
		})
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func decodeResultSetAggregateSortedMultiCriteriaJSONObject(raw []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('{') {
		return nil, fmt.Errorf("JSON value must be an object")
	}
	object := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
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
	token, err = decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('}') {
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

func decodeResultSetAggregateSortedMultiCriteriaJSONArray(raw json.RawMessage) ([]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('[') {
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
	token, err = decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim(']') {
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

func requireResultSetAggregateSortedMultiCriteriaJSONFields(object map[string]json.RawMessage, names ...string) error {
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

func decodeResultSetAggregateSortedMultiCriteriaJSONString(raw json.RawMessage, name string) (string, error) {
	if len(raw) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return "", fmt.Errorf("%s must be a string", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func runResultSetAggregateSortedMultiCriteriaScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateSortedMultiCriteriaScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	caseScenario, err := scenarioForCase(scenario, resultsetAggregateSortedMultiCriteriaCase)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateSortedMultiCriteriaBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetAggregateSortedMultiCriteriaTrigger](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateTable(env, "MyTable", []esper.TableColumn{
		esper.TableColumnOf[esper.SortedAccessValue[esper.SortedMultiKey, esper.Event]]("sortcol"),
	}); err != nil {
		return compat.Trace{}, err
	}
	theString := esper.Field[resultsetAggregateSortedMultiCriteriaBean, string]("theString")
	intPrimitive := esper.Field[resultsetAggregateSortedMultiCriteriaBean, int]("intPrimitive")
	aggregatePlan, err := env.Build(esper.From[resultsetAggregateSortedMultiCriteriaBean](env, "SupportBean").
		Window(esper.KeepAll()).
		Aggregate(esper.Alias("sortcol", esper.SortedAccessByMulti[esper.Event, string, int](esper.EventValue[esper.Event](), theString, intPrimitive))).
		IntoTable("MyTable", esper.StatementName("aggregate")))
	if err != nil {
		return compat.Trace{}, err
	}
	sortcol := esper.TableField[esper.SortedAccessValue[esper.SortedMultiKey, esper.Event]]("sortcol")
	trigger := esper.From[resultsetAggregateSortedMultiCriteriaTrigger](env, "SupportBean_S0")
	triggerPlan, err := env.Build(esper.OnEvent(trigger).SelectFromTableWhere("MyTable", esper.Literal(true),
		esper.Alias("firstkey", esper.Method[esper.SortedMultiKey](sortcol, "FirstKey")),
		esper.Alias("lastkey", esper.Method[esper.SortedMultiKey](sortcol, "LastKey")),
		esper.Alias("lowerkey", esper.Method[esper.SortedMultiKey](sortcol, "LowerKey", esper.Literal(esper.NewSortedMultiKey("E4", 1)))),
		esper.Alias("higherkey", esper.Method[esper.SortedMultiKey](sortcol, "HigherKey", esper.Literal(esper.NewSortedMultiKey("E4b", -1)))),
	).Query(esper.StatementName("s0")))
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateSortedMultiCriteriaJavaRuntimeIDs[0]),
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
		return compat.Trace{}, fmt.Errorf("expected one resultset aggregate trigger statement, got %d", len(statements))
	}
	statement := statements[0]
	trace, err := compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateSortedMultiCriteriaPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-sorted-multi-criteria statement %q", name)
		}
		return statement, nil
	})
	if err != nil {
		return compat.Trace{}, err
	}
	return normalizeResultSetAggregateSortedMultiCriteriaTrace(trace), nil
}

func normalizeResultSetAggregateSortedMultiCriteriaTrace(trace compat.Trace) compat.Trace {
	for recordIndex := range trace.Records {
		record := &trace.Records[recordIndex]
		for _, rows := range [][]compat.ResultRecord{record.New, record.Old} {
			for rowIndex := range rows {
				for name, value := range rows[rowIndex].Fields {
					key, ok := value.(esper.SortedMultiKey)
					if !ok {
						continue
					}
					parts := key.Parts()
					if len(parts) != 2 {
						continue
					}
					rows[rowIndex].Fields[name] = map[string]any{
						"kind": "row",
						"fields": map[string]any{
							"intPrimitive": parts[1],
							"theString":    parts[0],
						},
					}
				}
			}
		}
	}
	return trace
}

func validateResultSetAggregateSortedMultiCriteriaScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != "resultset-aggregate-sorted-multi-criteria" {
		return fmt.Errorf("resultset-aggregate-sorted-multi-criteria scenario has unsupported id %q", scenario.ID)
	}
	if len(scenario.Steps) != len(resultsetAggregateSortedMultiCriteriaExpected)+2 {
		return fmt.Errorf("resultset-aggregate-sorted-multi-criteria scenario must contain one case, seven SupportBean sends, and one trigger")
	}
	marker := scenario.Steps[0]
	if marker.Op != "case" || marker.Case != resultsetAggregateSortedMultiCriteriaCase {
		return fmt.Errorf("resultset-aggregate-sorted-multi-criteria scenario must start with case %q", resultsetAggregateSortedMultiCriteriaCase)
	}
	for index, expected := range resultsetAggregateSortedMultiCriteriaExpected {
		stepNumber := index + 1
		step := scenario.Steps[stepNumber]
		if step.Op != "send" || step.EventType != "SupportBean" {
			return fmt.Errorf("resultset-aggregate-sorted-multi-criteria step %d must be a SupportBean send", stepNumber)
		}
		actual, err := decodeResultSetAggregateSortedMultiCriteriaPayloadValue(step)
		if err != nil {
			return fmt.Errorf("resultset-aggregate-sorted-multi-criteria event %d: %w", index, err)
		}
		if actual != expected {
			return fmt.Errorf("resultset-aggregate-sorted-multi-criteria event %d = %#v, want %#v", index, actual, expected)
		}
	}
	trigger := scenario.Steps[len(scenario.Steps)-1]
	if trigger.Op != "send" || trigger.EventType != "SupportBean_S0" {
		return fmt.Errorf("resultset-aggregate-sorted-multi-criteria scenario must end with SupportBean_S0 trigger")
	}
	payload, err := decodeResultSetAggregateSortedMultiCriteriaTrigger(trigger.Payload)
	if err != nil {
		return fmt.Errorf("resultset-aggregate-sorted-multi-criteria trigger: %w", err)
	}
	if payload.ID != -1 {
		return fmt.Errorf("resultset-aggregate-sorted-multi-criteria trigger id must be -1")
	}
	return nil
}

func decodeResultSetAggregateSortedMultiCriteriaTrigger(raw json.RawMessage) (resultsetAggregateSortedMultiCriteriaTrigger, error) {
	payload, err := decodeResultSetAggregateSortedMultiCriteriaPayloadObject(raw)
	if err != nil {
		return resultsetAggregateSortedMultiCriteriaTrigger{}, fmt.Errorf("payload must be an object: %w", err)
	}
	if len(payload) != 1 {
		return resultsetAggregateSortedMultiCriteriaTrigger{}, fmt.Errorf("payload must contain exactly id")
	}
	idPayload, ok := payload["id"]
	if !ok || string(bytes.TrimSpace(idPayload)) == "null" {
		return resultsetAggregateSortedMultiCriteriaTrigger{}, fmt.Errorf("payload id must be an integer")
	}
	var id int32
	if err := json.Unmarshal(idPayload, &id); err != nil {
		return resultsetAggregateSortedMultiCriteriaTrigger{}, fmt.Errorf("payload id must be an integer")
	}
	return resultsetAggregateSortedMultiCriteriaTrigger{ID: int(id)}, nil
}

func decodeResultSetAggregateSortedMultiCriteriaPayload(step compat.Step) (any, error) {
	if step.EventType == "SupportBean" {
		value, err := decodeResultSetAggregateSortedMultiCriteriaPayloadValue(step)
		if err != nil {
			return nil, err
		}
		return resultsetAggregateSortedMultiCriteriaBean{TheString: value.theString, IntPrimitive: value.intPrimitive}, nil
	}
	if step.EventType == "SupportBean_S0" {
		value, err := decodeResultSetAggregateSortedMultiCriteriaTrigger(step.Payload)
		if err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	}
	return nil, fmt.Errorf("unsupported resultset-aggregate-sorted-multi-criteria event type %q", step.EventType)
}

func decodeResultSetAggregateSortedMultiCriteriaPayloadValue(step compat.Step) (resultsetAggregateSortedMultiCriteriaExpectedEvent, error) {
	if step.EventType != "SupportBean" {
		return resultsetAggregateSortedMultiCriteriaExpectedEvent{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	payload, err := decodeResultSetAggregateSortedMultiCriteriaPayloadObject(step.Payload)
	if err != nil {
		return resultsetAggregateSortedMultiCriteriaExpectedEvent{}, fmt.Errorf("payload must be an object: %w", err)
	}
	if len(payload) != 2 {
		return resultsetAggregateSortedMultiCriteriaExpectedEvent{}, fmt.Errorf("payload must contain exactly theString and intPrimitive")
	}
	var value resultsetAggregateSortedMultiCriteriaExpectedEvent
	stringPayload, ok := payload["theString"]
	if !ok || string(stringPayload) == "null" || json.Unmarshal(stringPayload, &value.theString) != nil {
		return resultsetAggregateSortedMultiCriteriaExpectedEvent{}, fmt.Errorf("payload theString must be a string")
	}
	intPayload, ok := payload["intPrimitive"]
	if !ok || string(intPayload) == "null" || json.Unmarshal(intPayload, &value.intPrimitive) != nil {
		return resultsetAggregateSortedMultiCriteriaExpectedEvent{}, fmt.Errorf("payload intPrimitive must be an integer")
	}
	return value, nil
}

func decodeResultSetAggregateSortedMultiCriteriaPayloadObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('{') {
		return nil, fmt.Errorf("payload must be an object")
	}
	payload := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("payload object key is not a string")
		}
		if _, exists := payload[key]; exists {
			return nil, fmt.Errorf("payload contains duplicate field %q", key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		payload[key] = value
	}
	token, err = decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('}') {
		return nil, fmt.Errorf("payload object is not closed")
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("payload contains trailing JSON")
		}
		return nil, err
	}
	return payload, nil
}
