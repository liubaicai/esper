package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetAggregateFilterNamedParameterBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

const (
	resultsetAggregateFilterNamedParameterID          = "resultset-aggregate-filter-named-parameter"
	resultsetAggregateFilterNamedParameterDescription = "Named filter aggregate methods replaying ResultSetAggregateFilterNamedParameter executions: leaving, nth, virtual-time rate and timestamp rate."
	resultsetAggregateFilterNamedParameterJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetAggregateFilterNamedParameterSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilterNamedParameter.java"
	resultsetAggregateFilterNamedParameterStaticID    = "java-0c29efb6d43971aba5c4"
)

var (
	resultsetAggregateFilterNamedParameterJavaRuntimeIDs = []string{
		"java-runtime-7bc068fcf2ea07b9c1f7",
		"java-runtime-dd319218418ee0418b21",
		"java-runtime-4b24ef27ade0eae24258",
		"java-runtime-3d732054eac8d5b8ba14",
	}
	resultsetAggregateFilterNamedParameterJavaExecutions = []string{
		"ResultSetAggregateMethodAggLeaving",
		"ResultSetAggregateMethodAggNth",
		"ResultSetAggregateMethodAggRateUnbound",
		"ResultSetAggregateMethodAggRateBound",
	}
	resultsetAggregateFilterNamedParameterCases = []string{
		"leaving",
		"nth",
		"rate-unbound",
		"rate-bound",
	}
	resultsetAggregateFilterNamedParameterOrdinals = []int{4, 5, 6, 7}
	resultsetAggregateFilterNamedParameterCaseEPLs = []string{
		"@name('s0') select leaving(filter:intPrimitive=1) as c0,leaving(filter:intPrimitive=2) as c1 from SupportBean#length(2)",
		"@name('s0') select nth(intPrimitive, 1, filter:theString like 'A%') as c0 from SupportBean",
		"@name('s0') select rate(1, filter:theString like 'A%') as c0 from SupportBean",
		"@name('s0') select rate(longPrimitive, filter:theString like 'A%') as myrate, rate(longPrimitive, intPrimitive, filter:theString like 'A%') as myqtyrate from SupportBean#length(3)",
	}
)

// loadResultSetAggregateFilterNamedParameterScenario validates the complete
// language-neutral asset, including Java metadata and every deterministic
// input. The generic compat loader intentionally does not know these fields;
// this loader keeps the oracle contract pinned to the fixed source.
func loadResultSetAggregateFilterNamedParameterScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetAggregateFilterNamedParameterID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetAggregateFilterNamedParameterID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateFilterNamedParameterID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateFilterNamedParameterID, err)
	}
	if err := requireResultSetAggregateFilterNamedParameterFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	metadata := make(map[string]string, 5)
	for _, name := range []string{"version", "id", "description", "javaCommit", "javaSource"} {
		value, err := decodeResultSetAggregateFilterNamedParameterString(root[name], name)
		if err != nil {
			return compat.Scenario{}, err
		}
		metadata[name] = value
	}
	if metadata["version"] != compat.ScenarioVersion ||
		metadata["id"] != resultsetAggregateFilterNamedParameterID ||
		metadata["description"] != resultsetAggregateFilterNamedParameterDescription ||
		metadata["javaCommit"] != resultsetAggregateFilterNamedParameterJavaCommit ||
		metadata["javaSource"] != resultsetAggregateFilterNamedParameterSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetAggregateFilterNamedParameterID)
	}
	for name, expected := range map[string][]string{
		"javaRuntimes":  resultsetAggregateFilterNamedParameterJavaRuntimeIDs,
		"javaNames":     resultsetAggregateFilterNamedParameterJavaExecutions,
		"javaStaticIds": {resultsetAggregateFilterNamedParameterStaticID},
		"javaFlags":     {},
	} {
		if err := validateResultSetAggregateFilterNamedParameterStringArray(root[name], expected, name); err != nil {
			return compat.Scenario{}, err
		}
	}

	rawCases, err := decodeResultSetAggregateFilterNamedParameterJSONArray(root["cases"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario cases: %w", err)
	}
	if len(rawCases) != len(resultsetAggregateFilterNamedParameterCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly four cases", resultsetAggregateFilterNamedParameterID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultSetAggregateFilterNamedParameterFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		caseName, err := decodeResultSetAggregateFilterNamedParameterString(object["case"], "case")
		if err != nil {
			return compat.Scenario{}, err
		}
		ordinal, err := decodeResultSetAggregateFilterNamedParameterInteger(object["ordinal"], "ordinal")
		if err != nil {
			return compat.Scenario{}, err
		}
		runtimeID, err := decodeResultSetAggregateFilterNamedParameterString(object["runtimeId"], "runtimeId")
		if err != nil {
			return compat.Scenario{}, err
		}
		executionName, err := decodeResultSetAggregateFilterNamedParameterString(object["executionName"], "executionName")
		if err != nil {
			return compat.Scenario{}, err
		}
		observation, err := decodeResultSetAggregateFilterNamedParameterString(object["observation"], "observation")
		if err != nil {
			return compat.Scenario{}, err
		}
		iteratorSnapshots, err := decodeResultSetAggregateFilterNamedParameterInteger(object["iteratorSnapshots"], "iteratorSnapshots")
		if err != nil {
			return compat.Scenario{}, err
		}
		epl, err := decodeResultSetAggregateFilterNamedParameterString(object["epl"], "epl")
		if err != nil {
			return compat.Scenario{}, err
		}
		if caseName != resultsetAggregateFilterNamedParameterCases[index] ||
			ordinal != int64(resultsetAggregateFilterNamedParameterOrdinals[index]) ||
			runtimeID != resultsetAggregateFilterNamedParameterJavaRuntimeIDs[index] ||
			executionName != resultsetAggregateFilterNamedParameterJavaExecutions[index] ||
			observation != "listener" || iteratorSnapshots != 0 ||
			epl != resultsetAggregateFilterNamedParameterCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetAggregateFilterNamedParameterID, index)
		}
	}

	rawSteps, err := decodeResultSetAggregateFilterNamedParameterJSONArray(root["steps"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario steps: %w", err)
	}
	if len(rawSteps) != 29 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 29 steps", resultsetAggregateFilterNamedParameterID)
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		op, err := decodeResultSetAggregateFilterNamedParameterString(object["op"], fmt.Sprintf("scenario step %d op", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		switch op {
		case "case":
			if err := requireResultSetAggregateFilterNamedParameterFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			caseName, err := decodeResultSetAggregateFilterNamedParameterString(object["case"], fmt.Sprintf("scenario step %d case", index))
			if err != nil {
				return compat.Scenario{}, err
			}
			steps[index] = compat.Step{Op: op, Case: caseName}
		case "send":
			if err := requireResultSetAggregateFilterNamedParameterFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			eventType, err := decodeResultSetAggregateFilterNamedParameterString(object["eventType"], fmt.Sprintf("scenario step %d eventType", index))
			if err != nil {
				return compat.Scenario{}, err
			}
			steps[index] = compat.Step{Op: op, EventType: eventType, Payload: append(json.RawMessage(nil), object["payload"]...)}
		case "advance-time":
			if err := requireResultSetAggregateFilterNamedParameterFields(object, "op", "at"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			at, err := decodeResultSetAggregateFilterNamedParameterString(object["at"], fmt.Sprintf("scenario step %d at", index))
			if err != nil {
				return compat.Scenario{}, err
			}
			steps[index] = compat.Step{Op: op, At: at}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, op)
		}
	}
	scenario := compat.Scenario{Version: metadata["version"], ID: metadata["id"], Steps: steps}
	if err := validateResultSetAggregateFilterNamedParameterScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func requireResultSetAggregateFilterNamedParameterFields(object map[string]json.RawMessage, names ...string) error {
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

func decodeResultSetAggregateFilterNamedParameterJSONArray(raw json.RawMessage) ([]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var values []json.RawMessage
	if err := decoder.Decode(&values); err != nil || values == nil {
		return nil, fmt.Errorf("JSON value must be an array")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("JSON value contains trailing data")
	}
	return values, nil
}

func decodeResultSetAggregateFilterNamedParameterString(raw json.RawMessage, name string) (string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must be a string", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func decodeResultSetAggregateFilterNamedParameterInteger(raw json.RawMessage, name string) (int64, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	number, ok := token.(json.Number)
	if !ok || !resultsetAggregateFilterNamedParameterIntegerSyntax(string(number)) {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	value, err := strconv.ParseInt(string(number), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	return value, nil
}

func resultsetAggregateFilterNamedParameterIntegerSyntax(text string) bool {
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

func validateResultSetAggregateFilterNamedParameterStringArray(raw json.RawMessage, expected []string, name string) error {
	values, err := decodeResultSetAggregateFilterNamedParameterJSONArray(raw)
	if err != nil || len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index, rawValue := range values {
		value, err := decodeResultSetAggregateFilterNamedParameterString(rawValue, name)
		if err != nil || value != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}

type resultsetAggregateFilterNamedParameterExpectedEvent struct {
	eventType     string
	fields        []string
	theString     string
	intPrimitive  int
	longPrimitive int64
}

var resultsetAggregateFilterNamedParameterExpectedEvents = map[int]resultsetAggregateFilterNamedParameterExpectedEvent{
	1:  {eventType: "SupportBean", fields: []string{"theString", "intPrimitive"}, theString: "E1", intPrimitive: 2},
	2:  {eventType: "SupportBean", fields: []string{"theString", "intPrimitive"}, theString: "E2", intPrimitive: 1},
	3:  {eventType: "SupportBean", fields: []string{"theString", "intPrimitive"}, theString: "E3", intPrimitive: 3},
	4:  {eventType: "SupportBean", fields: []string{"theString", "intPrimitive"}, theString: "E4", intPrimitive: 4},
	6:  {eventType: "SupportBean", fields: []string{"theString", "intPrimitive"}, theString: "X1", intPrimitive: 0},
	7:  {eventType: "SupportBean", fields: []string{"theString", "intPrimitive"}, theString: "X2", intPrimitive: 0},
	8:  {eventType: "SupportBean", fields: []string{"theString", "intPrimitive"}, theString: "A3", intPrimitive: 1},
	9:  {eventType: "SupportBean", fields: []string{"theString", "intPrimitive"}, theString: "A4", intPrimitive: 2},
	10: {eventType: "SupportBean", fields: []string{"theString", "intPrimitive"}, theString: "X3", intPrimitive: 0},
	11: {eventType: "SupportBean", fields: []string{"theString", "intPrimitive"}, theString: "A5", intPrimitive: 3},
	12: {eventType: "SupportBean", fields: []string{"theString", "intPrimitive"}, theString: "X4", intPrimitive: 0},
	14: {eventType: "SupportBean", fields: []string{"theString"}, theString: "X1"},
	15: {eventType: "SupportBean", fields: []string{"theString"}, theString: "A1"},
	17: {eventType: "SupportBean", fields: []string{"theString"}, theString: "X2"},
	18: {eventType: "SupportBean", fields: []string{"theString"}, theString: "A2"},
	19: {eventType: "SupportBean", fields: []string{"theString"}, theString: "A3"},
	21: {eventType: "SupportBean", fields: []string{"theString", "longPrimitive", "intPrimitive"}, theString: "X1", longPrimitive: 1000, intPrimitive: 10},
	22: {eventType: "SupportBean", fields: []string{"theString", "longPrimitive", "intPrimitive"}, theString: "X2", longPrimitive: 1200, intPrimitive: 0},
	23: {eventType: "SupportBean", fields: []string{"theString", "longPrimitive", "intPrimitive"}, theString: "X2", longPrimitive: 1300, intPrimitive: 0},
	24: {eventType: "SupportBean", fields: []string{"theString", "longPrimitive", "intPrimitive"}, theString: "A1", longPrimitive: 1000, intPrimitive: 10},
	25: {eventType: "SupportBean", fields: []string{"theString", "longPrimitive", "intPrimitive"}, theString: "A2", longPrimitive: 1200, intPrimitive: 0},
	26: {eventType: "SupportBean", fields: []string{"theString", "longPrimitive", "intPrimitive"}, theString: "A3", longPrimitive: 1300, intPrimitive: 0},
	27: {eventType: "SupportBean", fields: []string{"theString", "longPrimitive", "intPrimitive"}, theString: "A4", longPrimitive: 1500, intPrimitive: 14},
	28: {eventType: "SupportBean", fields: []string{"theString", "longPrimitive", "intPrimitive"}, theString: "A5", longPrimitive: 2000, intPrimitive: 11},
}

func validateResultSetAggregateFilterNamedParameterScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetAggregateFilterNamedParameterID || len(scenario.Steps) != 29 {
		return fmt.Errorf("%s scenario steps are not pinned", resultsetAggregateFilterNamedParameterID)
	}
	markers := map[int]string{0: "leaving", 5: "nth", 13: "rate-unbound", 20: "rate-bound"}
	for index, step := range scenario.Steps {
		if caseName, ok := markers[index]; ok {
			if step.Op != "case" || step.Case != caseName {
				return fmt.Errorf("scenario step %d must start case %q", index, caseName)
			}
			continue
		}
		if index == 16 {
			if step.Op != "advance-time" || step.Case != "" || step.At != "1970-01-01T00:00:01Z" {
				return fmt.Errorf("scenario step %d must advance time to the pinned rate boundary", index)
			}
			continue
		}
		if step.Op != "send" || step.Case != "" || step.EventType != "SupportBean" {
			return fmt.Errorf("scenario step %d must be an unlabelled SupportBean send", index)
		}
		expected, ok := resultsetAggregateFilterNamedParameterExpectedEvents[index]
		if !ok {
			return fmt.Errorf("scenario step %d has no pinned event", index)
		}
		actual, err := decodeResultSetAggregateFilterNamedParameterPayloadValue(step, expected.fields...)
		if err != nil {
			return fmt.Errorf("scenario step %d payload: %w", index, err)
		}
		if !reflect.DeepEqual(actual, expected) {
			return fmt.Errorf("scenario step %d payload is not pinned", index)
		}
	}
	return nil
}

func decodeResultSetAggregateFilterNamedParameterPayloadValue(step compat.Step, fields ...string) (resultsetAggregateFilterNamedParameterExpectedEvent, error) {
	if step.EventType != "SupportBean" {
		return resultsetAggregateFilterNamedParameterExpectedEvent{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var object map[string]json.RawMessage
	if err := strictObject(step.Payload, &object); err != nil {
		return resultsetAggregateFilterNamedParameterExpectedEvent{}, err
	}
	if err := requireResultSetAggregateFilterNamedParameterFields(object, fields...); err != nil {
		return resultsetAggregateFilterNamedParameterExpectedEvent{}, err
	}
	value := resultsetAggregateFilterNamedParameterExpectedEvent{eventType: step.EventType, fields: append([]string(nil), fields...)}
	var err error
	if _, ok := object["theString"]; ok {
		value.theString, err = decodeResultSetAggregateFilterNamedParameterString(object["theString"], "theString")
		if err != nil {
			return resultsetAggregateFilterNamedParameterExpectedEvent{}, err
		}
	}
	if _, ok := object["intPrimitive"]; ok {
		integer, err := decodeResultSetAggregateFilterNamedParameterInteger(object["intPrimitive"], "intPrimitive")
		if err != nil || integer < math.MinInt32 || integer > math.MaxInt32 {
			return resultsetAggregateFilterNamedParameterExpectedEvent{}, fmt.Errorf("intPrimitive must be a Java int")
		}
		value.intPrimitive = int(integer)
	}
	if _, ok := object["longPrimitive"]; ok {
		value.longPrimitive, err = decodeResultSetAggregateFilterNamedParameterInteger(object["longPrimitive"], "longPrimitive")
		if err != nil {
			return resultsetAggregateFilterNamedParameterExpectedEvent{}, err
		}
	}
	return value, nil
}

func runResultSetAggregateFilterNamedParameterScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateFilterNamedParameterScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultsetAggregateFilterNamedParameterCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultSetAggregateFilterNamedParameterCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset aggregate filter named-parameter case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario has no supported cases", resultsetAggregateFilterNamedParameterID)
	}
	return trace, nil
}

func runResultSetAggregateFilterNamedParameterCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateFilterNamedParameterBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	theString := esper.Field[resultsetAggregateFilterNamedParameterBean, string]("theString")
	intPrimitive := esper.Field[resultsetAggregateFilterNamedParameterBean, int]("intPrimitive")
	longPrimitive := esper.Field[resultsetAggregateFilterNamedParameterBean, int64]("longPrimitive")
	filter := esper.StartsWith(theString, esper.Literal("A"))
	var query esper.Query
	switch caseName {
	case "leaving":
		query = esper.From[resultsetAggregateFilterNamedParameterBean](env, "SupportBean").Window(esper.LengthWindow(2)).Aggregate(
			esper.Alias("c0", esper.Leaving(esper.Equal[int](intPrimitive, esper.Literal(1)))),
			esper.Alias("c1", esper.Leaving(esper.Equal[int](intPrimitive, esper.Literal(2)))),
		).Query(esper.StatementName("s0"))
	case "nth":
		query = esper.From[resultsetAggregateFilterNamedParameterBean](env, "SupportBean").Aggregate(
			esper.Alias("c0", esper.FilterAggregate[int](esper.Nth[int](intPrimitive, 1), filter)),
		).Query(esper.StatementName("s0"))
	case "rate-unbound":
		query = esper.From[resultsetAggregateFilterNamedParameterBean](env, "SupportBean").Aggregate(
			esper.Alias("c0", esper.Rate(time.Second, filter)),
		).Query(esper.StatementName("s0"))
	case "rate-bound":
		query = esper.From[resultsetAggregateFilterNamedParameterBean](env, "SupportBean").Window(esper.LengthWindow(3)).Aggregate(
			esper.Alias("myrate", esper.RateByTimestamp[int64](longPrimitive, filter)),
			esper.Alias("myqtyrate", esper.RateQuantityByTimestamp[int64, int](longPrimitive, intPrimitive, filter)),
		).Query(esper.StatementName("s0"))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported %s case %q", resultsetAggregateFilterNamedParameterID, caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateFilterNamedParameterRuntimeID(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one %s statement, got %d", resultsetAggregateFilterNamedParameterID, len(statements))
	}
	statement := statements[0]
	return compat.ReplayWithStatements(ctx, engine, statement, scenario,
		decodeResultSetAggregateFilterNamedParameterPayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetAggregateFilterNamedParameterID, name)
			}
			return statement, nil
		})
}

func resultsetAggregateFilterNamedParameterRuntimeID(caseName string) string {
	for index, name := range resultsetAggregateFilterNamedParameterCases {
		if name == caseName {
			return resultsetAggregateFilterNamedParameterJavaRuntimeIDs[index]
		}
	}
	return resultsetAggregateFilterNamedParameterID + "-unknown"
}

func decodeResultSetAggregateFilterNamedParameterPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetAggregateFilterNamedParameterID, step.EventType)
	}
	var value resultsetAggregateFilterNamedParameterBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
