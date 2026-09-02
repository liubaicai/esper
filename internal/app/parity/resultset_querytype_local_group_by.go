package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultSetQueryTypeLocalGroupByJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeLocalGroupByID          = "resultset-querytype-local-group-by"
	resultSetQueryTypeLocalGroupByDescription = "ResultSetQueryTypeLocalGroupBy ordinal 13: grouped multi-level window access with local group_by dimensions."
	resultSetQueryTypeLocalGroupBySource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
	resultSetQueryTypeLocalGroupByCase        = "multi-level-access"
	resultSetQueryTypeLocalGroupByExecution   = "ResultSetLocalGroupedMultiLevelAccess"
	resultSetQueryTypeLocalGroupByRuntimeID   = "java-runtime-87b3bcfdf164bc939fe5"
	resultSetQueryTypeLocalGroupByStaticID    = "java-b0b3444c9ac5a1297cd0"
	resultSetQueryTypeLocalGroupByEPL         = "@name('s0') select" +
		"   theString, intPrimitive," +
		"   window(*, group_by:(intPrimitive, theString)) as c0," +
		"   window(*) as c1," +
		"   window(*, group_by:theString) as c2," +
		"   window(*, group_by:intPrimitive) as c3," +
		"   window(*, group_by:()) as c4" +
		" from SupportBean#keepall" +
		" group by theString, intPrimitive" +
		" output snapshot every 10 seconds" +
		" order by theString, intPrimitive"
)

var (
	resultSetQueryTypeLocalGroupByJavaSources    = []string{resultSetQueryTypeLocalGroupBySource}
	resultSetQueryTypeLocalGroupByJavaRuntimeIDs = []string{resultSetQueryTypeLocalGroupByRuntimeID}
	resultSetQueryTypeLocalGroupByJavaExecutions = []string{resultSetQueryTypeLocalGroupByExecution}
)

// resultSetQueryTypeLocalGroupByBean mirrors all properties exposed by the
// Java SupportBean. The replay payload supplies only the three fields used by
// the statement; the remaining primitive fields retain Java's zero defaults
// and boxed fields remain null. CharPrimitive is a string so Java's default
// '\u0000' renders as a character rather than the numeric Go zero.
type resultSetQueryTypeLocalGroupByBean struct {
	TheString       string   `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int32    `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	CharPrimitive   string   `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	IntBoxed        *int32   `esper:"intBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	CharBoxed       *string  `esper:"charBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BigDecimal      *big.Rat `esper:"bigDecimal"`
	BigInteger      *big.Int `esper:"bigInteger"`
	EnumValue       *string  `esper:"enumValue"`
}

// loadResultSetQueryTypeLocalGroupByScenario performs the strict, duplicate-
// rejecting load used by the parity dispatcher. It validates metadata and
// action shape before exposing a compat.Scenario to the runtime.
func loadResultSetQueryTypeLocalGroupByScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultSetQueryTypeLocalGroupByID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultSetQueryTypeLocalGroupByID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupByID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupByID, err)
	}
	if err := requireResultSetQueryTypeLocalGroupByFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}

	version, err := decodeResultSetQueryTypeLocalGroupByString(root["version"], "version")
	if err != nil {
		return compat.Scenario{}, err
	}
	id, err := decodeResultSetQueryTypeLocalGroupByString(root["id"], "id")
	if err != nil {
		return compat.Scenario{}, err
	}
	description, err := decodeResultSetQueryTypeLocalGroupByString(root["description"], "description")
	if err != nil {
		return compat.Scenario{}, err
	}
	javaCommit, err := decodeResultSetQueryTypeLocalGroupByString(root["javaCommit"], "javaCommit")
	if err != nil {
		return compat.Scenario{}, err
	}
	javaSource, err := decodeResultSetQueryTypeLocalGroupByString(root["javaSource"], "javaSource")
	if err != nil {
		return compat.Scenario{}, err
	}
	if version != compat.ScenarioVersion || id != resultSetQueryTypeLocalGroupByID ||
		description != resultSetQueryTypeLocalGroupByDescription || javaCommit != resultSetQueryTypeLocalGroupByJavaCommit ||
		javaSource != resultSetQueryTypeLocalGroupBySource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultSetQueryTypeLocalGroupByID)
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaRuntimes"], resultSetQueryTypeLocalGroupByJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaNames"], resultSetQueryTypeLocalGroupByJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaStaticIds"], []string{resultSetQueryTypeLocalGroupByStaticID}, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != 1 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly one case", resultSetQueryTypeLocalGroupByID)
	}
	var caseObject map[string]json.RawMessage
	if err := strictObject(rawCases[0], &caseObject); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	if err := requireResultSetQueryTypeLocalGroupByFields(caseObject,
		"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	ordinal, err := decodeResultSetQueryTypeLocalGroupByInteger(caseObject["ordinal"], "ordinal")
	if err != nil {
		return compat.Scenario{}, err
	}
	iteratorSnapshots, err := decodeResultSetQueryTypeLocalGroupByInteger(caseObject["iteratorSnapshots"], "iteratorSnapshots")
	if err != nil {
		return compat.Scenario{}, err
	}
	var caseMeta struct {
		Case              string `json:"case"`
		Ordinal           int    `json:"ordinal"`
		RuntimeID         string `json:"runtimeId"`
		ExecutionName     string `json:"executionName"`
		Observation       string `json:"observation"`
		IteratorSnapshots int    `json:"iteratorSnapshots"`
		EPL               string `json:"epl"`
	}
	if err := json.Unmarshal(rawCases[0], &caseMeta); err != nil ||
		caseMeta.Case != resultSetQueryTypeLocalGroupByCase || ordinal != 13 || caseMeta.Ordinal != ordinal ||
		caseMeta.RuntimeID != resultSetQueryTypeLocalGroupByRuntimeID || caseMeta.ExecutionName != resultSetQueryTypeLocalGroupByExecution ||
		caseMeta.Observation != "listener" || iteratorSnapshots != 0 || caseMeta.IteratorSnapshots != iteratorSnapshots ||
		caseMeta.EPL != resultSetQueryTypeLocalGroupByEPL {
		return compat.Scenario{}, fmt.Errorf("%s scenario case metadata is not pinned", resultSetQueryTypeLocalGroupByID)
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 8 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly eight steps", resultSetQueryTypeLocalGroupByID)
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		switch index {
		case 0:
			err = requireResultSetQueryTypeLocalGroupByFields(object, "op", "case")
		case 1, 7:
			err = requireResultSetQueryTypeLocalGroupByFields(object, "op", "at")
		default:
			err = requireResultSetQueryTypeLocalGroupByFields(object, "op", "eventType", "payload")
		}
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: steps}
	if err := validateResultSetQueryTypeLocalGroupByScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func requireResultSetQueryTypeLocalGroupByFields(object map[string]json.RawMessage, expected ...string) error {
	if len(object) != len(expected) {
		return fmt.Errorf("JSON object contains unexpected or missing fields")
	}
	for _, name := range expected {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

func decodeResultSetQueryTypeLocalGroupByString(raw json.RawMessage, name string) (string, error) {
	if string(bytes.TrimSpace(raw)) == "null" {
		return "", fmt.Errorf("%s must be a string", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func decodeResultSetQueryTypeLocalGroupByInteger(raw json.RawMessage, name string) (int, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var number json.Number
	if err := decoder.Decode(&number); err != nil || number == "" {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	value, err := strconv.ParseInt(string(number), 10, 64)
	if err != nil || value < 0 || int64(int(value)) != value {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	return int(value), nil
}

func validateResultSetQueryTypeLocalGroupByStringArray(raw json.RawMessage, expected []string, name string) error {
	var actual []string
	if err := json.Unmarshal(raw, &actual); err != nil || actual == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if len(actual) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}

func validateResultSetQueryTypeLocalGroupByScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeLocalGroupByID || len(scenario.Steps) != 8 {
		return fmt.Errorf("%s scenario steps are not pinned", resultSetQueryTypeLocalGroupByID)
	}
	if marker := scenario.Steps[0]; marker.Op != "case" || marker.Case != resultSetQueryTypeLocalGroupByCase {
		return fmt.Errorf("%s scenario must start with case %q", resultSetQueryTypeLocalGroupByID, resultSetQueryTypeLocalGroupByCase)
	}
	for _, index := range []int{1, 7} {
		step := scenario.Steps[index]
		want := "1970-01-01T00:00:00Z"
		if index == 7 {
			want = "1970-01-01T00:00:10Z"
		}
		if step.Op != "advance-time" || step.At != want || step.Case != "" {
			return fmt.Errorf("%s scenario step %d must advance time to %s", resultSetQueryTypeLocalGroupByID, index, want)
		}
	}
	expected := []resultSetQueryTypeLocalGroupByPayload{
		{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100},
		{TheString: "E1", IntPrimitive: 20, LongPrimitive: 202},
		{TheString: "E2", IntPrimitive: 10, LongPrimitive: 303},
		{TheString: "E1", IntPrimitive: 10, LongPrimitive: 404},
		{TheString: "E2", IntPrimitive: 10, LongPrimitive: 505},
	}
	for offset, want := range expected {
		index := offset + 2
		step := scenario.Steps[index]
		if step.Op != "send" || step.EventType != "SupportBean" || step.Case != "" {
			return fmt.Errorf("%s scenario step %d must be an unlabelled SupportBean send", resultSetQueryTypeLocalGroupByID, index)
		}
		actual, err := decodeResultSetQueryTypeLocalGroupByPayload(step)
		if err != nil {
			return fmt.Errorf("%s scenario step %d: %w", resultSetQueryTypeLocalGroupByID, index, err)
		}
		if actual != want {
			return fmt.Errorf("%s scenario step %d payload is not pinned", resultSetQueryTypeLocalGroupByID, index)
		}
	}
	return nil
}

type resultSetQueryTypeLocalGroupByPayload struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int32  `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

func decodeResultSetQueryTypeLocalGroupByPayload(step compat.Step) (resultSetQueryTypeLocalGroupByPayload, error) {
	if step.EventType != "SupportBean" {
		return resultSetQueryTypeLocalGroupByPayload{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultSetQueryTypeLocalGroupByPayload{}, err
	}
	if err := requireResultSetQueryTypeLocalGroupByFields(fields, "theString", "intPrimitive", "longPrimitive"); err != nil {
		return resultSetQueryTypeLocalGroupByPayload{}, fmt.Errorf("SupportBean payload: %w", err)
	}
	if string(bytes.TrimSpace(fields["theString"])) == "null" || string(bytes.TrimSpace(fields["intPrimitive"])) == "null" || string(bytes.TrimSpace(fields["longPrimitive"])) == "null" {
		return resultSetQueryTypeLocalGroupByPayload{}, fmt.Errorf("SupportBean payload fields cannot be null")
	}
	var result resultSetQueryTypeLocalGroupByPayload
	if err := json.Unmarshal(fields["theString"], &result.TheString); err != nil {
		return resultSetQueryTypeLocalGroupByPayload{}, fmt.Errorf("theString must be a string")
	}
	if err := json.Unmarshal(fields["intPrimitive"], &result.IntPrimitive); err != nil {
		return resultSetQueryTypeLocalGroupByPayload{}, fmt.Errorf("intPrimitive must be an integer")
	}
	if err := json.Unmarshal(fields["longPrimitive"], &result.LongPrimitive); err != nil {
		return resultSetQueryTypeLocalGroupByPayload{}, fmt.Errorf("longPrimitive must be an integer")
	}
	return result, nil
}

func runResultSetQueryTypeLocalGroupByScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeLocalGroupByScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	caseScenario, err := scenarioForCase(scenario, resultSetQueryTypeLocalGroupByCase)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeLocalGroupByBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	theString := esper.Field[resultSetQueryTypeLocalGroupByBean, string]("theString")
	intPrimitive := esper.Field[resultSetQueryTypeLocalGroupByBean, int32]("intPrimitive")
	query := esper.From[resultSetQueryTypeLocalGroupByBean](env, "SupportBean").
		Window(esper.KeepAll()).
		GroupBy(theString, intPrimitive).
		Select(
			esper.Alias("theString", theString),
			esper.Alias("intPrimitive", intPrimitive),
			esper.Alias("c0", esper.LocalGroupBy[[]esper.Event](esper.WindowEvents(), intPrimitive, theString)),
			esper.Alias("c1", esper.WindowEvents()),
			esper.Alias("c2", esper.LocalGroupBy[[]esper.Event](esper.WindowEvents(), theString)),
			esper.Alias("c3", esper.LocalGroupBy[[]esper.Event](esper.WindowEvents(), intPrimitive)),
			esper.Alias("c4", esper.LocalGroupBy[[]esper.Event](esper.WindowEvents())),
		).
		Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputSnapshotEvery(10*time.Second)),
			esper.OrderBy(esper.Ascending(theString), esper.Ascending(intPrimitive)),
		)
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, resultSetQueryTypeLocalGroupByRuntimeID)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, func(step compat.Step) (any, error) {
		payload, err := decodeResultSetQueryTypeLocalGroupByPayload(step)
		if err != nil {
			return nil, err
		}
		return resultSetQueryTypeLocalGroupByBean{
			TheString: payload.TheString, IntPrimitive: payload.IntPrimitive,
			LongPrimitive: payload.LongPrimitive, CharPrimitive: "\u0000",
		}, nil
	}, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown %s statement %q", resultSetQueryTypeLocalGroupByID, name)
		}
		return statement, nil
	})
}
