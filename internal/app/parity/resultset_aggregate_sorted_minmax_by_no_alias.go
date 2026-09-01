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

type resultsetAggregateSortedMinMaxByNoAliasBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const (
	resultsetAggregateSortedMinMaxByNoAliasID          = "resultset-aggregate-sorted-minmax-by-no-alias"
	resultsetAggregateSortedMinMaxByNoAliasJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetAggregateSortedMinMaxByNoAliasDescription = "ResultSetAggregateSortedMinMaxBy ordinal 3: unaliased min-by/max-by and sorted projections auto-name their output columns; the deployment is acknowledged and the ordered property names/types are recorded without any events."
	resultsetAggregateSortedMinMaxByNoAliasSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateSortedMinMaxBy.java"
	resultsetAggregateSortedMinMaxByNoAliasRuntimeID   = "java-runtime-bb6969a66cad8464ae18"
	resultsetAggregateSortedMinMaxByNoAliasStaticID    = "java-553516b9d01c12a13172"
	resultsetAggregateSortedMinMaxByNoAliasExecution   = "ResultSetAggregateNoAlias"
	resultsetAggregateSortedMinMaxByNoAliasCase        = "no-alias"
	// resultsetAggregateSortedMinMaxByNoAliasEPL pins the exact Java source
	// EPL; the typed Go surface expresses the same five projections with the
	// Java auto-generated property names as explicit aliases.
	resultsetAggregateSortedMinMaxByNoAliasEPL = "@name('s0') select maxby(intPrimitive).theString, minby(intPrimitive),maxbyever(intPrimitive).theString, minbyever(intPrimitive),sorted(intPrimitive asc, theString desc) from SupportBean#time(10)"
)

var (
	resultsetAggregateSortedMinMaxByNoAliasJavaRuntimeIDs = []string{resultsetAggregateSortedMinMaxByNoAliasRuntimeID}
	resultsetAggregateSortedMinMaxByNoAliasJavaSources    = []string{resultsetAggregateSortedMinMaxByNoAliasSource}
	resultsetAggregateSortedMinMaxByNoAliasJavaExecutions = []string{resultsetAggregateSortedMinMaxByNoAliasExecution}
)

// resultsetAggregateSortedMinMaxByNoAliasTypes pins the five Java source
// property names in declaration order. Only the three String-typed columns
// are differential: Java renders whole-event minby/minbyever outputs and
// sorted() rows as java.util.Map (map-underlying render), while the Go typed
// API keeps the SupportBean event identity, so those three columns are
// registered as representation differences in the manifest instead.
var resultsetAggregateSortedMinMaxByNoAliasTypes = []struct {
	name         string
	token        string
	differential bool
}{
	{"maxby(intPrimitive).theString", "String", true},
	{"minby(intPrimitive)", "SupportBean", false},
	{"maxbyever(intPrimitive).theString", "String", true},
	{"minbyever(intPrimitive)", "SupportBean", false},
	{"sorted(intPrimitive,theString desc)", "SupportBean[]", false},
}

// resultsetAggregateSortedMinMaxByNoAliasTypeToken maps a Go output field
// type to the Java boxed-class simple name token used by the types record.
// javaTypeName intentionally lacks slice support; the sorted projection is
// SupportBean[] in Java, so this runner extends the token mapping locally
// instead of changing the shared helper.
func resultsetAggregateSortedMinMaxByNoAliasTypeToken(t reflect.Type) string {
	if t == nil {
		return ""
	}
	if t == reflect.TypeOf(esper.Event{}) {
		// MinBy/MinByEver projections keep the whole SupportBean event;
		// Java reports the underlying class simple name.
		return "SupportBean"
	}
	if t.Kind() == reflect.Slice {
		if element := resultsetAggregateSortedMinMaxByNoAliasTypeToken(t.Elem()); element != "" {
			return element + "[]"
		}
		return ""
	}
	switch t.Kind() {
	case reflect.Bool:
		return "Boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return "Integer"
	case reflect.Int64, reflect.Uint64:
		return "Long"
	case reflect.Float32:
		return "Float"
	case reflect.Float64:
		return "Double"
	case reflect.String:
		return "String"
	default:
		return ""
	}
}

func loadResultSetAggregateSortedMinMaxByNoAliasScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetAggregateSortedMinMaxByNoAliasID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetAggregateSortedMinMaxByNoAliasID, err)
	}
	if err := rejectResultSetAggregateSortedMinMaxByNoAliasDuplicateKeys(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateSortedMinMaxByNoAliasID, err)
	}
	root, err := decodeResultSetAggregateSortedMinMaxByNoAliasJSONObject(raw)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateSortedMinMaxByNoAliasID, err)
	}
	if err := requireResultSetAggregateSortedMinMaxByNoAliasFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	metadata := make(map[string]string, 5)
	for _, name := range []string{"version", "id", "description", "javaCommit", "javaSource"} {
		value, err := decodeResultSetAggregateSortedMinMaxByNoAliasString(root[name], name)
		if err != nil {
			return compat.Scenario{}, err
		}
		metadata[name] = value
	}
	if metadata["version"] != compat.ScenarioVersion || metadata["id"] != resultsetAggregateSortedMinMaxByNoAliasID ||
		metadata["description"] != resultsetAggregateSortedMinMaxByNoAliasDescription ||
		metadata["javaCommit"] != resultsetAggregateSortedMinMaxByNoAliasJavaCommit ||
		metadata["javaSource"] != resultsetAggregateSortedMinMaxByNoAliasSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetAggregateSortedMinMaxByNoAliasID)
	}
	if err := validateResultSetAggregateSortedMinMaxByNoAliasStringArray(root["javaRuntimes"], resultsetAggregateSortedMinMaxByNoAliasJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateSortedMinMaxByNoAliasStringArray(root["javaNames"], resultsetAggregateSortedMinMaxByNoAliasJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateSortedMinMaxByNoAliasStringArray(root["javaStaticIds"], []string{resultsetAggregateSortedMinMaxByNoAliasStaticID}, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateSortedMinMaxByNoAliasStringArray(root["javaFlags"], nil, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	rawCases, err := decodeResultSetAggregateSortedMinMaxByNoAliasJSONArray(root["cases"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario cases: %w", err)
	}
	if len(rawCases) != 1 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly one case", resultsetAggregateSortedMinMaxByNoAliasID)
	}
	caseObject, err := decodeResultSetAggregateSortedMinMaxByNoAliasJSONObject(rawCases[0])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	if err := requireResultSetAggregateSortedMinMaxByNoAliasFields(caseObject,
		"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	caseName, err := decodeResultSetAggregateSortedMinMaxByNoAliasString(caseObject["case"], "case")
	if err != nil {
		return compat.Scenario{}, err
	}
	ordinal, err := decodeResultSetAggregateSortedMinMaxByNoAliasInt(caseObject["ordinal"], "ordinal")
	if err != nil {
		return compat.Scenario{}, err
	}
	runtimeID, err := decodeResultSetAggregateSortedMinMaxByNoAliasString(caseObject["runtimeId"], "runtimeId")
	if err != nil {
		return compat.Scenario{}, err
	}
	executionName, err := decodeResultSetAggregateSortedMinMaxByNoAliasString(caseObject["executionName"], "executionName")
	if err != nil {
		return compat.Scenario{}, err
	}
	observation, err := decodeResultSetAggregateSortedMinMaxByNoAliasString(caseObject["observation"], "observation")
	if err != nil {
		return compat.Scenario{}, err
	}
	iteratorSnapshots, err := decodeResultSetAggregateSortedMinMaxByNoAliasInt(caseObject["iteratorSnapshots"], "iteratorSnapshots")
	if err != nil {
		return compat.Scenario{}, err
	}
	epl, err := decodeResultSetAggregateSortedMinMaxByNoAliasString(caseObject["epl"], "epl")
	if err != nil {
		return compat.Scenario{}, err
	}
	if caseName != resultsetAggregateSortedMinMaxByNoAliasCase || ordinal != 3 ||
		runtimeID != resultsetAggregateSortedMinMaxByNoAliasRuntimeID || executionName != resultsetAggregateSortedMinMaxByNoAliasExecution ||
		observation != "statement-metadata" || iteratorSnapshots != 0 || epl != resultsetAggregateSortedMinMaxByNoAliasEPL {
		return compat.Scenario{}, fmt.Errorf("%s scenario case metadata is not pinned", resultsetAggregateSortedMinMaxByNoAliasID)
	}

	rawSteps, err := decodeResultSetAggregateSortedMinMaxByNoAliasJSONArray(root["steps"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario steps: %w", err)
	}
	if len(rawSteps) != 3 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly three steps", resultsetAggregateSortedMinMaxByNoAliasID)
	}
	steps := make([]compat.Step, 0, len(rawSteps))
	for index, rawStep := range rawSteps {
		object, err := decodeResultSetAggregateSortedMinMaxByNoAliasJSONObject(rawStep)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		op, err := decodeResultSetAggregateSortedMinMaxByNoAliasString(object["op"], fmt.Sprintf("scenario step %d op", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		switch op {
		case "case":
			if err := requireResultSetAggregateSortedMinMaxByNoAliasFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			stepCase, err := decodeResultSetAggregateSortedMinMaxByNoAliasString(object["case"], fmt.Sprintf("scenario step %d case", index))
			if err != nil {
				return compat.Scenario{}, err
			}
			steps = append(steps, compat.Step{Op: op, Case: stepCase})
		case "deployed", "types":
			if err := requireResultSetAggregateSortedMinMaxByNoAliasFields(object, "op", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			statement, err := decodeResultSetAggregateSortedMinMaxByNoAliasString(object["statement"], fmt.Sprintf("scenario step %d statement", index))
			if err != nil {
				return compat.Scenario{}, err
			}
			steps = append(steps, compat.Step{Op: op, Statement: statement})
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, op)
		}
	}
	scenario := compat.Scenario{Version: metadata["version"], ID: metadata["id"], Steps: steps}
	if err := validateResultSetAggregateSortedMinMaxByNoAliasScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultSetAggregateSortedMinMaxByNoAliasScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetAggregateSortedMinMaxByNoAliasID {
		return fmt.Errorf("%s scenario has unsupported id %q", resultsetAggregateSortedMinMaxByNoAliasID, scenario.ID)
	}
	if len(scenario.Steps) != 3 {
		return fmt.Errorf("%s scenario must contain exactly three steps", resultsetAggregateSortedMinMaxByNoAliasID)
	}
	marker := scenario.Steps[0]
	if marker.Op != "case" || marker.Case != resultsetAggregateSortedMinMaxByNoAliasCase {
		return fmt.Errorf("%s scenario must start with case %q", resultsetAggregateSortedMinMaxByNoAliasID, resultsetAggregateSortedMinMaxByNoAliasCase)
	}
	deployed := scenario.Steps[1]
	if deployed.Op != "deployed" || deployed.Statement != "s0" {
		return fmt.Errorf("%s scenario step 1 must deploy s0", resultsetAggregateSortedMinMaxByNoAliasID)
	}
	types := scenario.Steps[2]
	if types.Op != "types" || types.Statement != "s0" {
		return fmt.Errorf("%s scenario step 2 must read s0 types", resultsetAggregateSortedMinMaxByNoAliasID)
	}
	return nil
}

func runResultSetAggregateSortedMinMaxByNoAliasScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateSortedMinMaxByNoAliasScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateSortedMinMaxByNoAliasBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	theString := esper.Field[resultsetAggregateSortedMinMaxByNoAliasBean, string]("theString")
	intPrimitive := esper.Field[resultsetAggregateSortedMinMaxByNoAliasBean, int]("intPrimitive")
	maxInt := esper.MaxBy[esper.Event, int](esper.EventValue[esper.Event](), intPrimitive)
	minInt := esper.MinBy[esper.Event, int](esper.EventValue[esper.Event](), intPrimitive)
	maxIntEver := esper.MaxByEver[esper.Event, int](esper.EventValue[esper.Event](), intPrimitive)
	minIntEver := esper.MinByEver[esper.Event, int](esper.EventValue[esper.Event](), intPrimitive)
	query := esper.From[resultsetAggregateSortedMinMaxByNoAliasBean](env, "SupportBean").
		Window(esper.TimeWindow(10*time.Second)).
		Aggregate(
			// Java auto-names unaliased aggregate projections by their
			// expression text; the Go surface pins those exact names as
			// explicit aliases so the observable schema is identical.
			esper.Alias("maxby(intPrimitive).theString", esper.Property[string](maxInt, "theString")),
			esper.Alias("minby(intPrimitive)", minInt),
			esper.Alias("maxbyever(intPrimitive).theString", esper.Property[string](maxIntEver, "theString")),
			esper.Alias("minbyever(intPrimitive)", minIntEver),
			esper.Alias("sorted(intPrimitive,theString desc)", esper.SortedEvents(esper.Ascending(intPrimitive), esper.Descending(theString))),
		).
		Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateSortedMinMaxByNoAliasRuntimeID),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one no-alias statement, got %d", len(statements))
	}
	statement := statements[0]
	schema, hasSchema := plan.ResultSchema()
	if !hasSchema {
		return compat.Trace{}, fmt.Errorf("%s statement has no result schema", resultsetAggregateSortedMinMaxByNoAliasID)

	}
	handlers := map[string]compat.StepHandler{
		"deployed": func(step compat.Step, _ func(*esper.Statement) error) ([]compat.TraceRecord, error) {
			return []compat.TraceRecord{{
				Case:      resultsetAggregateSortedMinMaxByNoAliasCase,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  0,
			}}, nil
		},
		"types": func(step compat.Step, _ func(*esper.Statement) error) ([]compat.TraceRecord, error) {
			fields := schema.Fields()
			if len(fields) != len(resultsetAggregateSortedMinMaxByNoAliasTypes) {
				return nil, fmt.Errorf("%s schema has %d fields, want %d", resultsetAggregateSortedMinMaxByNoAliasID, len(fields), len(resultsetAggregateSortedMinMaxByNoAliasTypes))
			}
			value := make([]map[string]any, 0, len(fields))
			for index, field := range fields {
				pinned := resultsetAggregateSortedMinMaxByNoAliasTypes[index]
				if field.Name != pinned.name {
					return nil, fmt.Errorf("%s field %d = %q, want %q", resultsetAggregateSortedMinMaxByNoAliasID, index, field.Name, pinned.name)
				}
				if !pinned.differential {
					// Java renders whole-event minby/minbyever outputs and
					// sorted() rows as java.util.Map (map-underlying render);
					// the Go typed API keeps the SupportBean event identity.
					// Those columns are registered as representation
					// differences in the manifest, not differential fields.
					continue
				}
				token := resultsetAggregateSortedMinMaxByNoAliasTypeToken(field.Type)
				if token != pinned.token {
					return nil, fmt.Errorf("%s field %q type = %q, want %q", resultsetAggregateSortedMinMaxByNoAliasID, field.Name, token, pinned.token)
				}
				value = append(value, map[string]any{"name": field.Name, "type": token})
			}
			return []compat.TraceRecord{{
				Case:      resultsetAggregateSortedMinMaxByNoAliasCase,
				Operation: "types",
				Statement: step.Statement,
				Sequence:  0,
				Value:     value,
			}}, nil
		},
	}
	trace, err := compat.ReplayWithStatementsAndHandlers(ctx, engine, statement, scenario,
		func(compat.Step) (any, error) {
			return nil, fmt.Errorf("%s scenario must not send events", resultsetAggregateSortedMinMaxByNoAliasID)
		},
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetAggregateSortedMinMaxByNoAliasID, name)
			}
			return statement, nil
		}, handlers)
	if err != nil {
		return compat.Trace{}, err
	}
	if err := validateResultSetAggregateSortedMinMaxByNoAliasTrace(trace); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

func validateResultSetAggregateSortedMinMaxByNoAliasTrace(trace compat.Trace) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != resultsetAggregateSortedMinMaxByNoAliasID {
		return fmt.Errorf("%s trace identity is not pinned", resultsetAggregateSortedMinMaxByNoAliasID)
	}
	if len(trace.Records) != 2 {
		return fmt.Errorf("%s trace must contain exactly two records", resultsetAggregateSortedMinMaxByNoAliasID)
	}
	deployed := trace.Records[0]
	if deployed.Case != resultsetAggregateSortedMinMaxByNoAliasCase || deployed.Operation != "deployed" ||
		deployed.Statement != "s0" || deployed.Sequence != 0 || deployed.Time != "" ||
		len(deployed.New) != 0 || len(deployed.Old) != 0 || deployed.Value != nil {
		return fmt.Errorf("%s deployed record is not pinned", resultsetAggregateSortedMinMaxByNoAliasID)
	}
	types := trace.Records[1]
	if types.Case != resultsetAggregateSortedMinMaxByNoAliasCase || types.Operation != "types" ||
		types.Statement != "s0" || types.Sequence != 0 || types.Time != "" ||
		len(types.New) != 0 || len(types.Old) != 0 {
		return fmt.Errorf("%s types record is not pinned", resultsetAggregateSortedMinMaxByNoAliasID)
	}
	entries, ok := types.Value.([]map[string]any)
	if !ok {
		return fmt.Errorf("%s types value must be an ordered array", resultsetAggregateSortedMinMaxByNoAliasID)
	}
	want := make([]struct{ name, token string }, 0, len(resultsetAggregateSortedMinMaxByNoAliasTypes))
	for _, pinned := range resultsetAggregateSortedMinMaxByNoAliasTypes {
		if pinned.differential {
			want = append(want, struct{ name, token string }{pinned.name, pinned.token})
		}
	}
	if len(entries) != len(want) {
		return fmt.Errorf("%s types value has %d entries, want %d", resultsetAggregateSortedMinMaxByNoAliasID, len(entries), len(want))
	}
	for index, entry := range entries {
		if len(entry) != 2 || entry["name"] != want[index].name || entry["type"] != want[index].token {
			return fmt.Errorf("%s types entry %d = %#v, want {name %q type %q}", resultsetAggregateSortedMinMaxByNoAliasID, index, entry, want[index].name, want[index].token)
		}
	}
	return nil
}

func decodeResultSetAggregateSortedMinMaxByNoAliasJSONObject(raw []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
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

func decodeResultSetAggregateSortedMinMaxByNoAliasJSONArray(raw json.RawMessage) ([]json.RawMessage, error) {
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

func requireResultSetAggregateSortedMinMaxByNoAliasFields(object map[string]json.RawMessage, names ...string) error {
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

func decodeResultSetAggregateSortedMinMaxByNoAliasString(raw json.RawMessage, name string) (string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must be a string", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func decodeResultSetAggregateSortedMinMaxByNoAliasInt(raw json.RawMessage, name string) (int, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	number, ok := token.(json.Number)
	if !ok || !resultsetAggregateSortedMinMaxByNoAliasIntegerSyntax(string(number)) {
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

func resultsetAggregateSortedMinMaxByNoAliasIntegerSyntax(text string) bool {
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

func validateResultSetAggregateSortedMinMaxByNoAliasStringArray(raw json.RawMessage, expected []string, name string) error {
	values, err := decodeResultSetAggregateSortedMinMaxByNoAliasJSONArray(raw)
	if err != nil {
		return fmt.Errorf("%s must be an array: %w", name, err)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index, rawValue := range values {
		value, err := decodeResultSetAggregateSortedMinMaxByNoAliasString(rawValue, name)
		if err != nil || value != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}

func rejectResultSetAggregateSortedMinMaxByNoAliasDuplicateKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkResultSetAggregateSortedMinMaxByNoAliasJSON(decoder); err != nil {
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

func walkResultSetAggregateSortedMinMaxByNoAliasJSON(decoder *json.Decoder) error {
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
			if err := walkResultSetAggregateSortedMinMaxByNoAliasJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("JSON object is not closed")
		}
	case json.Delim('['):
		for decoder.More() {
			if err := walkResultSetAggregateSortedMinMaxByNoAliasJSON(decoder); err != nil {
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
