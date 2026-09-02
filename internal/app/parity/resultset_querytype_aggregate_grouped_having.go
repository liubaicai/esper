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

type resultsetQueryTypeAggregateGroupedHavingMarketData struct {
	Symbol string  `esper:"symbol"`
	ID     *string `esper:"id"`
	Price  float64 `esper:"price"`
	Volume *int64  `esper:"volume"`
	Feed   *string `esper:"feed"`
}

type resultsetQueryTypeAggregateGroupedHavingBean struct {
	TheString       string   `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int      `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	CharPrimitive   string   `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	IntBoxed        *int     `esper:"intBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	CharBoxed       *string  `esper:"charBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BigDecimal      *float64 `esper:"bigDecimal"`
	BigInteger      *int64   `esper:"bigInteger"`
	EnumValue       *string  `esper:"enumValue"`
}

type resultsetQueryTypeAggregateGroupedHavingBeanString struct {
	TheString string `esper:"theString"`
}

type resultsetQueryTypeAggregateGroupedHavingS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

const resultsetQueryTypeAggregateGroupedHavingID = "resultset-querytype-aggregate-grouped-having"

const resultsetQueryTypeAggregateGroupedHavingDescription = "Grouped aggregates with having replaying ResultSetQueryTypeAggregateGroupedHaving executions over SupportBean/SupportBeanString/SupportBean_S0/SupportMarketDataBean: length_batch(3) count gate with per-event flush rows (plain wildcard and column selects), irstream symbol/volume/sum(price) per symbol with pre-subtraction remove-stream old rows (view and SupportBeanString join twins)"

const resultsetQueryTypeAggregateGroupedHavingJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetQueryTypeAggregateGroupedHavingJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeAggregateGroupedHaving.java",
}

// Case order fixes the runtime-ID index mapping below. The four executions
// share one Java class, one scenario, one oracle and one runner surface, so
// the unit ports them together (the ECSM multi-runtime precedent).
var resultsetQueryTypeAggregateGroupedHavingJavaRuntimeIDs = []string{
	"java-runtime-1474d172cf4f2a19b5d7",
	"java-runtime-88a7913c4758d8de0bc9",
	"java-runtime-dbe24180b80fd3c64d66",
	"java-runtime-aafa294bfd2a1104befd",
}

var resultsetQueryTypeAggregateGroupedHavingJavaExecutions = []string{
	"ResultSetQueryTypeGroupByHaving{join=false}",
	"ResultSetQueryTypeGroupByHaving{join=true}",
	"ResultSetQueryTypeSumOneView",
	"ResultSetQueryTypeSumJoin",
}

var resultsetQueryTypeAggregateGroupedHavingCases = []string{
	"groupby-having-nojoin",
	"groupby-having-join",
	"sum-one-view",
	"sum-join",
}

var resultsetQueryTypeAggregateGroupedHavingJavaStaticIDs = []string{
	"java-29aa3ed4e339f5786c1f",
	"java-cc01f532a81e94885155",
	"java-a24c796fd8d8a7f7d632",
}

var resultsetQueryTypeAggregateGroupedHavingOrdinals = []int{0, 1, 2, 3}

var resultsetQueryTypeAggregateGroupedHavingCaseEPLs = []string{
	"@name('s0') select * from SupportBean#length_batch(3) group by theString having count(*) > 1",
	"@name('s0') select theString, intPrimitive from SupportBean_S0#lastevent, SupportBean#length_batch(3) group by theString having count(*) > 1",
	"@name('s0') select irstream symbol, volume, sum(price) as mySum from SupportMarketDataBean#length(3) where symbol='DELL' or symbol='IBM' or symbol='GE' group by symbol having sum(price) >= 50",
	"@name('s0') select irstream symbol, volume, sum(price) as mySum from SupportBeanString#length(100) as one, SupportMarketDataBean#length(3) as two where (symbol='DELL' or symbol='IBM' or symbol='GE')   and one.theString = two.symbol group by symbol having sum(price) >= 50",
}

func loadResultsetQueryTypeAggregateGroupedHavingScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetQueryTypeAggregateGroupedHavingID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetQueryTypeAggregateGroupedHavingID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetQueryTypeAggregateGroupedHavingID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetQueryTypeAggregateGroupedHavingID, err)
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("%s scenario contains unexpected or missing fields", resultsetQueryTypeAggregateGroupedHavingID)
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing field %q", resultsetQueryTypeAggregateGroupedHavingID, name)
		}
	}
	metadata := make(map[string]string, 5)
	for _, name := range []string{"version", "id", "description", "javaCommit", "javaSource"} {
		value, err := decodeResultsetQueryTypeAggregateGroupedHavingString(root[name], name)
		if err != nil {
			return compat.Scenario{}, err
		}
		metadata[name] = value
	}
	if metadata["version"] != compat.ScenarioVersion || metadata["id"] != resultsetQueryTypeAggregateGroupedHavingID ||
		metadata["description"] != resultsetQueryTypeAggregateGroupedHavingDescription ||
		metadata["javaCommit"] != resultsetQueryTypeAggregateGroupedHavingJavaCommit ||
		metadata["javaSource"] != resultsetQueryTypeAggregateGroupedHavingJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetQueryTypeAggregateGroupedHavingID)
	}
	for name, expected := range map[string][]string{
		"javaRuntimes":  resultsetQueryTypeAggregateGroupedHavingJavaRuntimeIDs,
		"javaNames":     resultsetQueryTypeAggregateGroupedHavingJavaExecutions,
		"javaStaticIds": resultsetQueryTypeAggregateGroupedHavingJavaStaticIDs,
		"javaFlags":     {},
	} {
		if err := validateResultsetQueryTypeAggregateGroupedHavingStringArray(root[name], expected, name); err != nil {
			return compat.Scenario{}, err
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetQueryTypeAggregateGroupedHavingCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly four cases", resultsetQueryTypeAggregateGroupedHavingID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		caseFields := []string{"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"}
		if err := requireResultsetQueryTypeAggregateGroupedHavingFields(object, caseFields...); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		caseName, err := decodeResultsetQueryTypeAggregateGroupedHavingString(object["case"], "case")
		if err != nil {
			return compat.Scenario{}, err
		}
		ordinal, err := decodeResultsetQueryTypeAggregateGroupedHavingInteger(object["ordinal"], "ordinal")
		if err != nil {
			return compat.Scenario{}, err
		}
		runtimeID, err := decodeResultsetQueryTypeAggregateGroupedHavingString(object["runtimeId"], "runtimeId")
		if err != nil {
			return compat.Scenario{}, err
		}
		executionName, err := decodeResultsetQueryTypeAggregateGroupedHavingString(object["executionName"], "executionName")
		if err != nil {
			return compat.Scenario{}, err
		}
		observation, err := decodeResultsetQueryTypeAggregateGroupedHavingString(object["observation"], "observation")
		if err != nil {
			return compat.Scenario{}, err
		}
		iteratorSnapshots, err := decodeResultsetQueryTypeAggregateGroupedHavingInteger(object["iteratorSnapshots"], "iteratorSnapshots")
		if err != nil {
			return compat.Scenario{}, err
		}
		epl, err := decodeResultsetQueryTypeAggregateGroupedHavingString(object["epl"], "epl")
		if err != nil {
			return compat.Scenario{}, err
		}
		if caseName != resultsetQueryTypeAggregateGroupedHavingCases[index] || ordinal != int64(resultsetQueryTypeAggregateGroupedHavingOrdinals[index]) ||
			runtimeID != resultsetQueryTypeAggregateGroupedHavingJavaRuntimeIDs[index] ||
			executionName != resultsetQueryTypeAggregateGroupedHavingJavaExecutions[index] || observation != "listener" || iteratorSnapshots != 0 ||
			epl != resultsetQueryTypeAggregateGroupedHavingCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetQueryTypeAggregateGroupedHavingID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 26 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 26 steps", resultsetQueryTypeAggregateGroupedHavingID)
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		op, err := decodeResultsetQueryTypeAggregateGroupedHavingString(object["op"], fmt.Sprintf("scenario step %d op", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		switch op {
		case "case":
			if err := requireResultsetQueryTypeAggregateGroupedHavingFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			caseName, err := decodeResultsetQueryTypeAggregateGroupedHavingString(object["case"], fmt.Sprintf("scenario step %d case", index))
			if err != nil {
				return compat.Scenario{}, err
			}
			steps[index] = compat.Step{Op: op, Case: caseName}
		case "send":
			if err := requireResultsetQueryTypeAggregateGroupedHavingFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			eventType, err := decodeResultsetQueryTypeAggregateGroupedHavingString(object["eventType"], fmt.Sprintf("scenario step %d eventType", index))
			if err != nil {
				return compat.Scenario{}, err
			}
			payload := compat.Step{Op: op, EventType: eventType, Payload: append(json.RawMessage(nil), object["payload"]...)}
			if _, err := decodeResultsetQueryTypeAggregateGroupedHavingPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			steps[index] = payload
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, op)
		}
	}
	scenario := compat.Scenario{Version: metadata["version"], ID: metadata["id"], Steps: steps}
	if err := validateResultsetQueryTypeAggregateGroupedHavingScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetQueryTypeAggregateGroupedHavingScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetQueryTypeAggregateGroupedHavingID || len(scenario.Steps) != 26 {
		return fmt.Errorf("%s scenario steps are not pinned", resultsetQueryTypeAggregateGroupedHavingID)
	}
	markers := map[int]string{
		0:  resultsetQueryTypeAggregateGroupedHavingCases[0],
		5:  resultsetQueryTypeAggregateGroupedHavingCases[1],
		10: resultsetQueryTypeAggregateGroupedHavingCases[2],
		17: resultsetQueryTypeAggregateGroupedHavingCases[3],
	}
	for index, step := range scenario.Steps {
		if expected, ok := markers[index]; ok {
			if step.Op != "case" || step.Case != expected {
				return fmt.Errorf("scenario step %d must start case %q", index, expected)
			}
			continue
		}
		if step.Op != "send" || step.Case != "" {
			return fmt.Errorf("scenario step %d must be an unlabelled send", index)
		}
	}
	expected := []struct {
		index     int
		eventType string
		payload   any
	}{
		{1, "SupportBean_S0", resultsetQueryTypeAggregateGroupedHavingS0{ID: 1}},
		{2, "SupportBean", resultsetQueryTypeAggregateGroupedHavingBean{TheString: "E1", IntPrimitive: 10}},
		{3, "SupportBean", resultsetQueryTypeAggregateGroupedHavingBean{TheString: "E2", IntPrimitive: 20}},
		{4, "SupportBean", resultsetQueryTypeAggregateGroupedHavingBean{TheString: "E2", IntPrimitive: 21}},
		{6, "SupportBean_S0", resultsetQueryTypeAggregateGroupedHavingS0{ID: 1}},
		{7, "SupportBean", resultsetQueryTypeAggregateGroupedHavingBean{TheString: "E1", IntPrimitive: 10}},
		{8, "SupportBean", resultsetQueryTypeAggregateGroupedHavingBean{TheString: "E2", IntPrimitive: 20}},
		{9, "SupportBean", resultsetQueryTypeAggregateGroupedHavingBean{TheString: "E2", IntPrimitive: 21}},
		{11, "SupportMarketDataBean", resultsetQueryTypeAggregateGroupedHavingMarketData{Symbol: "XXX", Price: 1, Volume: groupedHavingInt64Pointer(0)}},
		{12, "SupportMarketDataBean", resultsetQueryTypeAggregateGroupedHavingMarketData{Symbol: "DELL", Price: 49, Volume: groupedHavingInt64Pointer(10000)}},
		{13, "SupportMarketDataBean", resultsetQueryTypeAggregateGroupedHavingMarketData{Symbol: "DELL", Price: 54, Volume: groupedHavingInt64Pointer(20000)}},
		{14, "SupportMarketDataBean", resultsetQueryTypeAggregateGroupedHavingMarketData{Symbol: "IBM", Price: 10, Volume: groupedHavingInt64Pointer(1000)}},
		{15, "SupportMarketDataBean", resultsetQueryTypeAggregateGroupedHavingMarketData{Symbol: "IBM", Price: 20, Volume: groupedHavingInt64Pointer(5000)}},
		{16, "SupportMarketDataBean", resultsetQueryTypeAggregateGroupedHavingMarketData{Symbol: "IBM", Price: 5, Volume: groupedHavingInt64Pointer(6000)}},
		{18, "SupportBeanString", resultsetQueryTypeAggregateGroupedHavingBeanString{TheString: "DELL"}},
		{19, "SupportBeanString", resultsetQueryTypeAggregateGroupedHavingBeanString{TheString: "IBM"}},
		{20, "SupportMarketDataBean", resultsetQueryTypeAggregateGroupedHavingMarketData{Symbol: "XXX", Price: 1, Volume: groupedHavingInt64Pointer(0)}},
		{21, "SupportMarketDataBean", resultsetQueryTypeAggregateGroupedHavingMarketData{Symbol: "DELL", Price: 49, Volume: groupedHavingInt64Pointer(10000)}},
		{22, "SupportMarketDataBean", resultsetQueryTypeAggregateGroupedHavingMarketData{Symbol: "DELL", Price: 54, Volume: groupedHavingInt64Pointer(20000)}},
		{23, "SupportMarketDataBean", resultsetQueryTypeAggregateGroupedHavingMarketData{Symbol: "IBM", Price: 10, Volume: groupedHavingInt64Pointer(1000)}},
		{24, "SupportMarketDataBean", resultsetQueryTypeAggregateGroupedHavingMarketData{Symbol: "IBM", Price: 20, Volume: groupedHavingInt64Pointer(5000)}},
		{25, "SupportMarketDataBean", resultsetQueryTypeAggregateGroupedHavingMarketData{Symbol: "IBM", Price: 5, Volume: groupedHavingInt64Pointer(6000)}},
	}
	for _, want := range expected {
		step := scenario.Steps[want.index]
		if step.EventType != want.eventType {
			return fmt.Errorf("scenario step %d event type = %q, want %q", want.index, step.EventType, want.eventType)
		}
		actual, err := decodeResultsetQueryTypeAggregateGroupedHavingPayload(step)
		if err != nil {
			return fmt.Errorf("scenario step %d payload: %w", want.index, err)
		}
		if !reflect.DeepEqual(actual, want.payload) {
			return fmt.Errorf("scenario step %d payload is not pinned", want.index)
		}
	}
	return nil
}

func groupedHavingInt64Pointer(value int64) *int64 {
	return &value
}

func decodeResultsetQueryTypeAggregateGroupedHavingPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireResultsetQueryTypeAggregateGroupedHavingFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		name, err := decodeResultsetQueryTypeAggregateGroupedHavingString(fields["theString"], "theString")
		if err != nil {
			return nil, err
		}
		value, err := decodeResultsetQueryTypeAggregateGroupedHavingInteger(fields["intPrimitive"], "intPrimitive")
		if err != nil || int64(int(value)) != value {
			return nil, fmt.Errorf("intPrimitive must be an integer JSON number")
		}
		return resultsetQueryTypeAggregateGroupedHavingBean{TheString: name, IntPrimitive: int(value)}, nil
	case "SupportBeanString":
		if err := requireResultsetQueryTypeAggregateGroupedHavingFields(fields, "theString"); err != nil {
			return nil, err
		}
		name, err := decodeResultsetQueryTypeAggregateGroupedHavingString(fields["theString"], "theString")
		if err != nil {
			return nil, err
		}
		return resultsetQueryTypeAggregateGroupedHavingBeanString{TheString: name}, nil
	case "SupportBean_S0":
		if err := requireResultsetQueryTypeAggregateGroupedHavingFields(fields, "id"); err != nil {
			return nil, err
		}
		value, err := decodeResultsetQueryTypeAggregateGroupedHavingInteger(fields["id"], "id")
		if err != nil || int64(int(value)) != value {
			return nil, fmt.Errorf("id must be an integer JSON number")
		}
		return resultsetQueryTypeAggregateGroupedHavingS0{ID: int(value)}, nil
	case "SupportMarketDataBean":
		if err := requireResultsetQueryTypeAggregateGroupedHavingFields(fields, "symbol", "price", "volume"); err != nil {
			return nil, err
		}
		symbol, err := decodeResultsetQueryTypeAggregateGroupedHavingString(fields["symbol"], "symbol")
		if err != nil {
			return nil, err
		}
		price, err := decodeResultsetQueryTypeAggregateGroupedHavingFloat(fields["price"], "price")
		if err != nil {
			return nil, err
		}
		volume, err := decodeResultsetQueryTypeAggregateGroupedHavingInteger(fields["volume"], "volume")
		if err != nil {
			return nil, err
		}
		return resultsetQueryTypeAggregateGroupedHavingMarketData{Symbol: symbol, Price: price, Volume: groupedHavingInt64Pointer(volume)}, nil
	default:
		return nil, fmt.Errorf("unknown event type %q", step.EventType)
	}
}

func requireResultsetQueryTypeAggregateGroupedHavingFields(object map[string]json.RawMessage, names ...string) error {
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

func decodeResultsetQueryTypeAggregateGroupedHavingString(raw json.RawMessage, name string) (string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must be a string", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func decodeResultsetQueryTypeAggregateGroupedHavingInteger(raw json.RawMessage, name string) (int64, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	number, ok := token.(json.Number)
	if !ok || !resultsetQueryTypeAggregateGroupedHavingIntegerSyntax(string(number)) {
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

func decodeResultsetQueryTypeAggregateGroupedHavingFloat(raw json.RawMessage, name string) (float64, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s must be a JSON number", name)
	}
	number, ok := token.(json.Number)
	if !ok {
		return 0, fmt.Errorf("%s must be a JSON number", name)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return 0, fmt.Errorf("%s must be a JSON number", name)
	}
	value, err := strconv.ParseFloat(string(number), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("%s must be a JSON number", name)
	}
	return value, nil
}

func resultsetQueryTypeAggregateGroupedHavingIntegerSyntax(text string) bool {
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

func validateResultsetQueryTypeAggregateGroupedHavingStringArray(raw json.RawMessage, expected []string, name string) error {
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

// runResultsetQueryTypeAggregateGroupedHavingScenario replays the four grouped
// having executions. Java semantics pinned by the oracle: the length_batch(3)
// count gate emits one row per event of a passing group at flush (Java
// generateOutputBatchedViewUnkeyed loops every batch event, having evaluated
// per event with the flush-time aggregate state), the wildcard row renders the
// full SupportBean property map, and the irstream sum executions post
// pre-subtraction remove-stream old rows.
func runResultsetQueryTypeAggregateGroupedHavingScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultsetQueryTypeAggregateGroupedHavingCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultsetQueryTypeAggregateGroupedHavingCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset querytype aggregate-grouped having case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("resultset querytype aggregate-grouped having scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runResultsetQueryTypeAggregateGroupedHavingCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetQueryTypeAggregateGroupedHavingBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetQueryTypeAggregateGroupedHavingMarketData](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetQueryTypeAggregateGroupedHavingS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	needsString := caseName == "groupby-having-join" || caseName == "sum-join"
	if needsString {
		if _, err := esper.RegisterStruct[resultsetQueryTypeAggregateGroupedHavingBeanString](env, "SupportBeanString"); err != nil {
			return compat.Trace{}, err
		}
	}

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(ctx) }()

	var plan esper.Plan
	var planErr error
	switch caseName {
	case "groupby-having-nojoin":
		// Java: select * from SupportBean#length_batch(3) group by theString
		// having count(*) > 1. The wildcard row renders every SupportBean
		// property; the flush emits one row per event of a passing group.
		plan, planErr = env.Build(esper.From[resultsetQueryTypeAggregateGroupedHavingBean](env, "SupportBean").
			Window(esper.LengthBatch(3)).
			GroupBy(esper.Field[resultsetQueryTypeAggregateGroupedHavingBean, string]("theString")).
			Select(resultsetQueryTypeRowPerGroupHavingWildcardSelections()...).
			Having(esper.Greater[int64](esper.CountAll(), esper.Literal(int64(1)))).
			Query(esper.StatementName("s0")))
	case "groupby-having-join":
		// Java: select theString, intPrimitive from SupportBean_S0#lastevent,
		// SupportBean#length_batch(3) group by theString having count(*) > 1.
		// No join condition: the S0#lastevent row cross-joins each batch
		// event; the flush semantics are identical to the nojoin twin.
		plan, planErr = env.Build(esper.JoinMany(
			esper.JoinSource(esper.From[resultsetQueryTypeAggregateGroupedHavingS0](env, "SupportBean_S0").Window(esper.LastEvent())),
			esper.JoinSource(esper.From[resultsetQueryTypeAggregateGroupedHavingBean](env, "SupportBean").Window(esper.LengthBatch(3))),
		).GroupBy(esper.JoinField[string](1, "theString")).Select(
			esper.Alias("theString", esper.JoinField[string](1, "theString")),
			esper.Alias("intPrimitive", esper.JoinField[int](1, "intPrimitive")),
		).Having(esper.Greater[int64](esper.CountAll(), esper.Literal(int64(1)))).
			Query(esper.StatementName("s0")))
	case "sum-one-view", "sum-join":
		// Java: select irstream symbol, volume, sum(price) as mySum over
		// SupportMarketDataBean#length(3) group by symbol having
		// sum(price) >= 50 [joined with SupportBeanString#length(100)].
		plan, planErr = buildResultsetQueryTypeAggregateGroupedHavingSum(env, needsString)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset querytype aggregate-grouped having case %q", caseName)
	}
	if planErr != nil {
		return compat.Trace{}, planErr
	}

	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one statement, got %d", len(statements))
	}
	statement := statements[0]
	seq := uint64(0)
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		newRows := compat.NormalizeResults(batch.New)
		oldRows := compat.NormalizeResults(batch.Old)
		if len(newRows) == 0 && len(oldRows) == 0 {
			return nil
		}
		seq++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "listener", Statement: statement.Name(),
			Sequence: seq, Time: batch.Time.UTC().Format(time.RFC3339), New: newRows, Old: oldRows,
		})
		return nil
	}); err != nil {
		return compat.Trace{}, err
	}

	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			if err := sendResultsetQueryTypeAggregateGroupedHavingEvent(ctx, engine, step); err != nil {
				return trace, err
			}
		default:
			return trace, fmt.Errorf("unsupported step %q", step.Op)
		}
	}
	return trace, nil
}

// buildResultsetQueryTypeAggregateGroupedHavingSum constructs the irstream
// symbol/volume/sum(price) having statement shared by ordinals 2 and 3. The
// join twin reads market fields through JoinField(0, ...) so the group key and
// aggregate bind against the driving market side of the tuple.
func buildResultsetQueryTypeAggregateGroupedHavingSum(env *esper.Environment, join bool) (esper.Plan, error) {
	market := esper.From[resultsetQueryTypeAggregateGroupedHavingMarketData](env, "SupportMarketDataBean").
		Window(esper.LengthWindow(3)).
		Filter(esper.Or(
			esper.Equal[string](esper.Field[resultsetQueryTypeAggregateGroupedHavingMarketData, string]("symbol"), esper.Literal("DELL")),
			esper.Or(
				esper.Equal[string](esper.Field[resultsetQueryTypeAggregateGroupedHavingMarketData, string]("symbol"), esper.Literal("IBM")),
				esper.Equal[string](esper.Field[resultsetQueryTypeAggregateGroupedHavingMarketData, string]("symbol"), esper.Literal("GE")),
			),
		))
	if !join {
		symbol := esper.Field[resultsetQueryTypeAggregateGroupedHavingMarketData, string]("symbol")
		price := esper.Field[resultsetQueryTypeAggregateGroupedHavingMarketData, float64]("price")
		volume := esper.Field[resultsetQueryTypeAggregateGroupedHavingMarketData, *int64]("volume")
		sumPrice := esper.Sum[float64](price)
		return env.Build(market.GroupBy(symbol).Select(
			esper.Alias("symbol", symbol),
			esper.Alias("volume", volume),
			esper.Alias("mySum", sumPrice),
		).Having(esper.GreaterOrEqual[float64](sumPrice, esper.Literal(50.0))).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
	}
	seed := esper.From[resultsetQueryTypeAggregateGroupedHavingBeanString](env, "SupportBeanString").Window(esper.LengthWindow(100))
	symbol := esper.JoinField[string](0, "symbol")
	price := esper.JoinField[float64](0, "price")
	volume := esper.JoinField[*int64](0, "volume")
	sumPrice := esper.Sum[float64](price)
	joined := esper.Join(market, seed, esper.OnEqual(
		esper.JoinField[string](1, "theString"),
		symbol,
	))
	return env.Build(joined.GroupBy(symbol).Select(
		esper.Alias("symbol", symbol),
		esper.Alias("volume", volume),
		esper.Alias("mySum", sumPrice),
	).Having(esper.GreaterOrEqual[float64](sumPrice, esper.Literal(50.0))).
		Query(esper.StatementName("s0"), esper.WithOldStream()))
}

func sendResultsetQueryTypeAggregateGroupedHavingEvent(ctx context.Context, engine *esper.Engine, step compat.Step) error {
	switch step.EventType {
	case "SupportBean":
		var payload struct {
			TheString    string `json:"theString"`
			IntPrimitive int    `json:"intPrimitive"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		return engine.Send(ctx, "SupportBean", resultsetQueryTypeAggregateGroupedHavingBean{
			TheString: payload.TheString, IntPrimitive: payload.IntPrimitive,
			CharPrimitive: "\u0000",
		})
	case "SupportBeanString":
		var payload struct {
			TheString string `json:"theString"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		return engine.Send(ctx, "SupportBeanString", resultsetQueryTypeAggregateGroupedHavingBeanString{TheString: payload.TheString})
	case "SupportBean_S0":
		var payload struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		return engine.Send(ctx, "SupportBean_S0", resultsetQueryTypeAggregateGroupedHavingS0{ID: payload.ID})
	case "SupportMarketDataBean":
		var payload struct {
			Symbol string  `json:"symbol"`
			Price  float64 `json:"price"`
			Volume *int64  `json:"volume"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		return engine.Send(ctx, "SupportMarketDataBean", resultsetQueryTypeAggregateGroupedHavingMarketData{
			Symbol: payload.Symbol, Price: payload.Price, Volume: payload.Volume,
		})
	}
	return fmt.Errorf("unknown event type %q", step.EventType)
}

func resultsetQueryTypeAggregateGroupedHavingRuntimeID(caseName string) string {
	for index, name := range resultsetQueryTypeAggregateGroupedHavingCases {
		if name == caseName {
			return resultsetQueryTypeAggregateGroupedHavingJavaRuntimeIDs[index]
		}
	}
	return "resultset-querytype-aggregate-grouped-having-unknown"
}
