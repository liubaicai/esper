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

type resultsetAggregateMinMaxNoDataWindowSubqueryBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type resultsetAggregateMinMaxNoDataWindowSubqueryS0 struct {
	ID int `esper:"id"`
}

const (
	resultsetAggregateMinMaxNoDataWindowSubqueryJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetAggregateMinMaxNoDataWindowSubqueryCase       = "no-data-window-subquery"
)

var resultsetAggregateMinMaxNoDataWindowSubqueryJavaSources = []string{"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateMinMax.java"}
var resultsetAggregateMinMaxNoDataWindowSubqueryJavaRuntimeIDs = []string{"java-runtime-f1fedff9a25cca57fefa"}
var resultsetAggregateMinMaxNoDataWindowSubqueryJavaExecutions = []string{"ResultSetAggregateMinMaxNoDataWindowSubquery"}

const resultsetAggregateMinMaxNoDataWindowSubqueryDescription = "ResultSetAggregateMinMax ordinal 0 NoDataWindowSubquery: unbounded SupportBean max/min plus scalar last-event SupportBean_S0 max/min. S0 updates alone produce no callback; four SupportBean events produce new-only rows."

func loadResultSetAggregateMinMaxNoDataWindowSubqueryScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-minmax-no-data-window-subquery scenario reader is required")
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read resultset-aggregate-minmax-no-data-window-subquery scenario: %w", err)
	}
	if err := rejectResultSetAggregateMinMaxNoDataWindowSubqueryDuplicateKeys(data); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-aggregate-minmax-no-data-window-subquery scenario: %w", err)
	}
	root, err := decodeResultSetAggregateMinMaxNoDataWindowSubqueryJSONObject(data)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-aggregate-minmax-no-data-window-subquery scenario: %w", err)
	}
	if err := requireResultSetAggregateMinMaxNoDataWindowSubqueryFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
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
	if version != compat.ScenarioVersion || id != "resultset-aggregate-minmax-no-data-window-subquery" ||
		description != resultsetAggregateMinMaxNoDataWindowSubqueryDescription ||
		javaCommit != resultsetAggregateMinMaxNoDataWindowSubqueryJavaCommit ||
		javaSource != resultsetAggregateMinMaxNoDataWindowSubqueryJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-minmax-no-data-window-subquery metadata is not pinned")
	}
	var runtimes, names, staticIDs, flags []string
	for name, target := range map[string]*[]string{
		"javaRuntimes": &runtimes, "javaNames": &names, "javaStaticIds": &staticIDs, "javaFlags": &flags,
	} {
		if err := json.Unmarshal(root[name], target); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario %s must be a string array", name)
		}
	}
	if !resultsetAggregateMinMaxNoDataWindowSubqueryStringsEqual(runtimes, resultsetAggregateMinMaxNoDataWindowSubqueryJavaRuntimeIDs) ||
		!resultsetAggregateMinMaxNoDataWindowSubqueryStringsEqual(names, resultsetAggregateMinMaxNoDataWindowSubqueryJavaExecutions) ||
		!resultsetAggregateMinMaxNoDataWindowSubqueryStringsEqual(staticIDs, []string{"java-347fe4c3739663bf45c4"}) ||
		!resultsetAggregateMinMaxNoDataWindowSubqueryStringsEqual(flags, []string{"EXCLUDEWHENINSTRUMENTED"}) {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-minmax-no-data-window-subquery Java references are not pinned")
	}
	cases, err := decodeResultSetMinMaxNoDataWindowSubqueryJSONArray(root["cases"])
	if err != nil || len(cases) != 1 {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-minmax-no-data-window-subquery must contain exactly one case")
	}
	caseObject, err := decodeResultSetAggregateMinMaxNoDataWindowSubqueryJSONObject(cases[0])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	if err := requireResultSetAggregateMinMaxNoDataWindowSubqueryFields(caseObject, "case", "ordinal", "runtimeId", "executionName", "observation", "epl"); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	var caseMeta struct {
		Case          string `json:"case"`
		Ordinal       int    `json:"ordinal"`
		RuntimeID     string `json:"runtimeId"`
		ExecutionName string `json:"executionName"`
		Observation   string `json:"observation"`
		EPL           string `json:"epl"`
	}
	caseData, _ := json.Marshal(caseObject)
	if err := json.Unmarshal(caseData, &caseMeta); err != nil || caseMeta.Case != resultsetAggregateMinMaxNoDataWindowSubqueryCase || caseMeta.Ordinal != 0 || caseMeta.RuntimeID != resultsetAggregateMinMaxNoDataWindowSubqueryJavaRuntimeIDs[0] || caseMeta.ExecutionName != resultsetAggregateMinMaxNoDataWindowSubqueryJavaExecutions[0] || caseMeta.Observation != "listener" || caseMeta.EPL != "@name('s0') select max(intPrimitive) as maxi, min(intPrimitive) as mini,(select max(id) from SupportBean_S0#lastevent) as max0, (select min(id) from SupportBean_S0#lastevent) as min0 from SupportBean" {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-minmax-no-data-window-subquery case metadata is not pinned")
	}
	stepValues, err := decodeResultSetMinMaxNoDataWindowSubqueryJSONArray(root["steps"])
	if err != nil || len(stepValues) != 7 {
		return compat.Scenario{}, fmt.Errorf("resultset-aggregate-minmax-no-data-window-subquery must contain exactly seven steps")
	}
	steps := make([]compat.Step, 0, len(stepValues))
	for index, rawStep := range stepValues {
		stepObject, err := decodeResultSetAggregateMinMaxNoDataWindowSubqueryJSONObject(rawStep)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if index == 0 {
			if err := requireResultSetAggregateMinMaxNoDataWindowSubqueryFields(stepObject, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		} else if err := requireResultSetAggregateMinMaxNoDataWindowSubqueryFields(stepObject, "op", "eventType", "payload"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		encoded, _ := json.Marshal(stepObject)
		var step compat.Step
		if err := json.Unmarshal(encoded, &step); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		steps = append(steps, step)
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: steps}
	if err := validateResultSetAggregateMinMaxNoDataWindowSubqueryScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func resultsetAggregateMinMaxNoDataWindowSubqueryStringsEqual(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}

func rejectResultSetAggregateMinMaxNoDataWindowSubqueryDuplicateKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkResultSetAggregateMinMaxNoDataWindowSubqueryJSON(decoder); err != nil {
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

func walkResultSetAggregateMinMaxNoDataWindowSubqueryJSON(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
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
			if err := walkResultSetAggregateMinMaxNoDataWindowSubqueryJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("JSON object is not closed")
		}
	case '[':
		for decoder.More() {
			if err := walkResultSetAggregateMinMaxNoDataWindowSubqueryJSON(decoder); err != nil {
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
	return nil
}

func decodeResultSetAggregateMinMaxNoDataWindowSubqueryJSONObject(raw []byte) (map[string]json.RawMessage, error) {
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
		return nil, fmt.Errorf("JSON object contains trailing data")
	}
	return object, nil
}

func decodeResultSetMinMaxNoDataWindowSubqueryJSONArray(raw json.RawMessage) ([]json.RawMessage, error) {
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
		return nil, fmt.Errorf("JSON array contains trailing data")
	}
	return values, nil
}

func requireResultSetAggregateMinMaxNoDataWindowSubqueryFields(object map[string]json.RawMessage, names ...string) error {
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

func runResultSetAggregateMinMaxNoDataWindowSubqueryScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateMinMaxNoDataWindowSubqueryScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	caseScenario, err := scenarioForCase(scenario, resultsetAggregateMinMaxNoDataWindowSubqueryCase)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateMinMaxNoDataWindowSubqueryBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetAggregateMinMaxNoDataWindowSubqueryS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	outer := esper.From[resultsetAggregateMinMaxNoDataWindowSubqueryBean](env, "SupportBean")
	inner := esper.From[resultsetAggregateMinMaxNoDataWindowSubqueryS0](env, "SupportBean_S0").Window(esper.LastEvent()).AsRecord()
	value := esper.Field[resultsetAggregateMinMaxNoDataWindowSubqueryBean, int]("intPrimitive")
	id := esper.Field[resultsetAggregateMinMaxNoDataWindowSubqueryS0, int]("id")
	query := outer.Aggregate(
		esper.Alias("maxi", esper.Max[int](value)),
		esper.Alias("mini", esper.Min[int](value)),
		esper.Alias("max0", esper.SubqueryValue[int](inner, esper.Max[int](id))),
		esper.Alias("min0", esper.SubqueryValue[int](inner, esper.Min[int](id))),
	).Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()), esper.WithRuntimeURI(resultsetAggregateMinMaxNoDataWindowSubqueryJavaRuntimeIDs[0]))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one resultset aggregate statement, got %d", len(statements))
	}
	statement := statements[0]
	trace, err := compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateMinMaxNoDataWindowSubqueryPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-minmax-no-data-window-subquery statement %q", name)
		}
		return statement, nil
	})
	if err != nil {
		return compat.Trace{}, err
	}
	trace.ID = scenario.ID
	trace.Version = scenario.Version
	return trace, nil
}

func validateResultSetAggregateMinMaxNoDataWindowSubqueryScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != "resultset-aggregate-minmax-no-data-window-subquery" {
		return fmt.Errorf("resultset-aggregate-minmax-no-data-window-subquery scenario has unsupported id %q", scenario.ID)
	}
	if len(scenario.Steps) != 7 {
		return fmt.Errorf("resultset-aggregate-minmax-no-data-window-subquery scenario must contain one case and six sends")
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != resultsetAggregateMinMaxNoDataWindowSubqueryCase {
		return fmt.Errorf("resultset-aggregate-minmax-no-data-window-subquery scenario must start with case %q", resultsetAggregateMinMaxNoDataWindowSubqueryCase)
	}
	expected := []struct {
		typ     string
		payload string
		id      int
	}{{"SupportBean", `{"theString":"E1","intPrimitive":3}`, 0}, {"SupportBean", `{"theString":"E2","intPrimitive":4}`, 0}, {"SupportBean_S0", `{"id":2}`, 2}, {"SupportBean", `{"theString":"E3","intPrimitive":4}`, 0}, {"SupportBean_S0", `{"id":1}`, 1}, {"SupportBean", `{"theString":"E4","intPrimitive":5}`, 0}}
	for i, e := range expected {
		s := scenario.Steps[i+1]
		if s.Op != "send" || s.EventType != e.typ {
			return fmt.Errorf("... step %d must be %s send", i+1, e.typ)
		}
		if e.typ == "SupportBean" {
			v, err := decodeResultSetAggregateMinMaxNoDataWindowSubqueryBean(s)
			if err != nil {
				return fmt.Errorf("step %d: %w", i+1, err)
			}
			var want resultsetAggregateMinMaxNoDataWindowSubqueryBean
			_ = json.Unmarshal([]byte(e.payload), &want)
			if v != want {
				return fmt.Errorf("step %d has unexpected SupportBean payload", i+1)
			}
		} else {
			v, err := decodeResultSetAggregateMinMaxNoDataWindowSubqueryS0Payload(s)
			if err != nil {
				return fmt.Errorf("step %d: %w", i+1, err)
			}
			if v.ID != e.id {
				return fmt.Errorf("step %d has unexpected S0 payload", i+1)
			}
		}
	}
	return nil
}

func decodeResultSetAggregateMinMaxNoDataWindowSubqueryPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		return decodeResultSetAggregateMinMaxNoDataWindowSubqueryBean(step)
	case "SupportBean_S0":
		return decodeResultSetAggregateMinMaxNoDataWindowSubqueryS0Payload(step)
	default:
		return nil, fmt.Errorf("unsupported resultset-aggregate-minmax-no-data-window-subquery event type %q", step.EventType)
	}
}
func decodeResultSetAggregateMinMaxNoDataWindowSubqueryBean(step compat.Step) (resultsetAggregateMinMaxNoDataWindowSubqueryBean, error) {
	if step.EventType != "SupportBean" {
		return resultsetAggregateMinMaxNoDataWindowSubqueryBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var m map[string]json.RawMessage
	if err := strictObject(step.Payload, &m); err != nil {
		return resultsetAggregateMinMaxNoDataWindowSubqueryBean{}, err
	}
	if len(m) != 2 {
		return resultsetAggregateMinMaxNoDataWindowSubqueryBean{}, fmt.Errorf("SupportBean payload must contain exactly theString and intPrimitive")
	}
	if string(bytes.TrimSpace(m["theString"])) == "null" || string(bytes.TrimSpace(m["intPrimitive"])) == "null" {
		return resultsetAggregateMinMaxNoDataWindowSubqueryBean{}, fmt.Errorf("SupportBean fields cannot be null")
	}
	var v resultsetAggregateMinMaxNoDataWindowSubqueryBean
	if err := json.Unmarshal(m["theString"], &v.TheString); err != nil {
		return v, fmt.Errorf("theString must be a string")
	}
	var intPrimitive int32
	if err := json.Unmarshal(m["intPrimitive"], &intPrimitive); err != nil {
		return v, fmt.Errorf("intPrimitive must be a Java Integer")
	}
	v.IntPrimitive = int(intPrimitive)
	return v, nil
}
func decodeResultSetAggregateMinMaxNoDataWindowSubqueryS0Payload(step compat.Step) (resultsetAggregateMinMaxNoDataWindowSubqueryS0, error) {
	if step.EventType != "SupportBean_S0" {
		return resultsetAggregateMinMaxNoDataWindowSubqueryS0{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var m map[string]json.RawMessage
	if err := strictObject(step.Payload, &m); err != nil {
		return resultsetAggregateMinMaxNoDataWindowSubqueryS0{}, err
	}
	if len(m) != 1 || string(bytes.TrimSpace(m["id"])) == "null" {
		return resultsetAggregateMinMaxNoDataWindowSubqueryS0{}, fmt.Errorf("SupportBean_S0 payload must contain non-null integer id")
	}
	var id int32
	if err := json.Unmarshal(m["id"], &id); err != nil {
		return resultsetAggregateMinMaxNoDataWindowSubqueryS0{}, fmt.Errorf("id must be a Java Integer")
	}
	return resultsetAggregateMinMaxNoDataWindowSubqueryS0{ID: int(id)}, nil
}
func strictObject(raw json.RawMessage, m *map[string]json.RawMessage) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return fmt.Errorf("payload must be an object")
	}
	*m = make(map[string]json.RawMessage)
	for d.More() {
		k, err := d.Token()
		if err != nil {
			return err
		}
		key, ok := k.(string)
		if !ok {
			return fmt.Errorf("payload object key is not a string")
		}
		if _, ok := (*m)[key]; ok {
			return fmt.Errorf("payload contains duplicate field %q", key)
		}
		var v json.RawMessage
		if err := d.Decode(&v); err != nil {
			return err
		}
		(*m)[key] = v
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') {
		return fmt.Errorf("payload object is not closed")
	}
	var extra json.RawMessage
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("payload contains trailing JSON")
	}
	return nil
}
