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

const (
	resultsetOutputLimitRowLimitVariableID          = "resultset-output-limit-row-limit-variable"
	resultsetOutputLimitRowLimitVariableJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOutputLimitRowLimitVariableDescription = "ResultSetOutputLimitRowLimit ordinal 9: variable-backed dynamic limit and offset across comma, keyword, and SODA deployment forms."
	resultsetOutputLimitRowLimitVariableSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowLimit.java"
)

var (
	resultsetOutputLimitRowLimitVariableJavaRuntimeIDs = []string{
		"java-runtime-c591e5e05eb22cddfd00",
		"java-runtime-c591e5e05eb22cddfd00",
		"java-runtime-c591e5e05eb22cddfd00",
	}
	resultsetOutputLimitRowLimitVariableJavaStaticIDs = []string{
		"java-299a17bb6319c7f2e6de",
		"java-299a17bb6319c7f2e6de",
		"java-299a17bb6319c7f2e6de",
	}
	resultsetOutputLimitRowLimitVariableJavaSources = []string{
		resultsetOutputLimitRowLimitVariableSource,
	}
	resultsetOutputLimitRowLimitVariableJavaExecutions = []string{
		"ResultSetLengthOffsetVariable",
		"ResultSetLengthOffsetVariable",
		"ResultSetLengthOffsetVariable",
	}
	resultsetOutputLimitRowLimitVariableCases = []string{
		"variable-comma",
		"variable-keyword",
		"variable-soda",
	}
	resultsetOutputLimitRowLimitVariableOrdinals = []int{9, 9, 9}
	resultsetOutputLimitRowLimitVariableEPLs     = []string{
		"@name('s0') select * from SupportBean#length(5) output every 5 events limit myoffset, myrows",
		"@name('s0') select * from SupportBean#length(5) output every 5 events limit myrows offset myoffset",
		"@name('s0') select * from SupportBean#length(5) output every 5 events limit myrows offset myoffset",
	}
)

type resultsetOutputLimitRowLimitVariableSupportBean struct {
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

type resultsetOutputLimitRowLimitVariableSupportBeanNumeric struct {
	IntOne *int `esper:"intOne"`
	IntTwo *int `esper:"intTwo"`
}

func loadResultsetOutputLimitRowLimitVariableScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOutputLimitRowLimitVariableID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOutputLimitRowLimitVariableID, err)
	}
	if err := rejectResultsetOutputLimitRowLimitVariableDuplicateKeys(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitRowLimitVariableID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitRowLimitVariableID, err)
	}
	if err := requireResultsetOutputLimitRowLimitVariableFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOutputLimitRowLimitVariableID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetOutputLimitRowLimitVariableID ||
		metadata.Description != resultsetOutputLimitRowLimitVariableDescription ||
		metadata.JavaCommit != resultsetOutputLimitRowLimitVariableJavaCommit ||
		metadata.JavaSource != resultsetOutputLimitRowLimitVariableSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOutputLimitRowLimitVariableID)
	}
	if err := validateResultsetOutputLimitRowLimitVariableStringArray(root["javaRuntimes"], resultsetOutputLimitRowLimitVariableJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitRowLimitVariableStringArray(root["javaNames"], resultsetOutputLimitRowLimitVariableJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitRowLimitVariableStringArray(root["javaStaticIds"], resultsetOutputLimitRowLimitVariableJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitRowLimitVariableStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetOutputLimitRowLimitVariableCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly three cases", resultsetOutputLimitRowLimitVariableID)
	}
	observations := []string{"listener+iterator", "listener+iterator", "listener+iterator+soda"}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetOutputLimitRowLimitVariableFields(object,
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
		if definition.Case != resultsetOutputLimitRowLimitVariableCases[index] ||
			definition.Ordinal != resultsetOutputLimitRowLimitVariableOrdinals[index] ||
			definition.RuntimeID != resultsetOutputLimitRowLimitVariableJavaRuntimeIDs[index] ||
			definition.ExecutionName != resultsetOutputLimitRowLimitVariableJavaExecutions[index] ||
			definition.Observation != observations[index] || definition.IteratorSnapshots != 21 ||
			definition.EPL != resultsetOutputLimitRowLimitVariableEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetOutputLimitRowLimitVariableID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 141 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 141 steps", resultsetOutputLimitRowLimitVariableID)
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
		var expected []string
		switch operation {
		case "case":
			expected = []string{"op", "case"}
		case "snapshot":
			expected = []string{"op", "case", "statement"}
		case "send":
			expected = []string{"op", "case", "eventType", "payload"}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := requireResultsetOutputLimitRowLimitVariableFields(object, expected...); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateResultsetOutputLimitRowLimitVariableScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetOutputLimitRowLimitVariableScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetOutputLimitRowLimitVariableID || len(scenario.Steps) != 141 {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetOutputLimitRowLimitVariableID)
	}
	finalSetters := resultsetOutputLimitRowLimitVariableFinalSetters()
	index := 0
	for caseIndex, caseName := range resultsetOutputLimitRowLimitVariableCases {
		if index >= len(scenario.Steps) || scenario.Steps[index].Op != "case" || scenario.Steps[index].Case != caseName {
			return fmt.Errorf("%s case marker %d is not pinned", resultsetOutputLimitRowLimitVariableID, caseIndex)
		}
		index++
		if err := validateResultsetOutputLimitRowLimitVariableSnapshot(scenario.Steps[index], caseName); err != nil {
			return fmt.Errorf("%s case %q initial snapshot: %w", resultsetOutputLimitRowLimitVariableID, caseName, err)
		}
		index++
		if err := validateResultsetOutputLimitRowLimitVariableBeanSend(scenario.Steps[index], caseName, 1); err != nil {
			return fmt.Errorf("%s case %q E1: %w", resultsetOutputLimitRowLimitVariableID, caseName, err)
		}
		index++
		for eventNumber := 2; eventNumber <= 6; eventNumber++ {
			if err := validateResultsetOutputLimitRowLimitVariableBeanSend(scenario.Steps[index], caseName, eventNumber); err != nil {
				return fmt.Errorf("%s case %q E%d: %w", resultsetOutputLimitRowLimitVariableID, caseName, eventNumber, err)
			}
			index++
			if err := validateResultsetOutputLimitRowLimitVariableSnapshot(scenario.Steps[index], caseName); err != nil {
				return fmt.Errorf("%s case %q E%d snapshot: %w", resultsetOutputLimitRowLimitVariableID, caseName, eventNumber, err)
			}
			index++
		}
		initialSetters := []resultsetOutputLimitRowLimitVariableSetter{
			{IntOne: resultsetOutputLimitRowLimitVariableIntPointer(2), IntTwo: resultsetOutputLimitRowLimitVariableIntPointer(3)},
			{IntOne: resultsetOutputLimitRowLimitVariableIntPointer(-1), IntTwo: resultsetOutputLimitRowLimitVariableIntPointer(0)},
			{IntOne: resultsetOutputLimitRowLimitVariableIntPointer(10), IntTwo: resultsetOutputLimitRowLimitVariableIntPointer(0)},
			{IntOne: resultsetOutputLimitRowLimitVariableIntPointer(6), IntTwo: resultsetOutputLimitRowLimitVariableIntPointer(3)},
		}
		for offset, setter := range initialSetters {
			if err := validateResultsetOutputLimitRowLimitVariableNumericSend(scenario.Steps[index], caseName, setter); err != nil {
				return fmt.Errorf("%s case %q setter %d: %w", resultsetOutputLimitRowLimitVariableID, caseName, offset, err)
			}
			index++
			if err := validateResultsetOutputLimitRowLimitVariableBeanSend(scenario.Steps[index], caseName, 7+offset); err != nil {
				return fmt.Errorf("%s case %q E%d: %w", resultsetOutputLimitRowLimitVariableID, caseName, 7+offset, err)
			}
			index++
			if err := validateResultsetOutputLimitRowLimitVariableSnapshot(scenario.Steps[index], caseName); err != nil {
				return fmt.Errorf("%s case %q E%d snapshot: %w", resultsetOutputLimitRowLimitVariableID, caseName, 7+offset, err)
			}
			index++
		}
		for setterIndex, setter := range finalSetters {
			if err := validateResultsetOutputLimitRowLimitVariableNumericSend(scenario.Steps[index], caseName, setter); err != nil {
				return fmt.Errorf("%s case %q final setter %d: %w", resultsetOutputLimitRowLimitVariableID, caseName, setterIndex, err)
			}
			index++
			if err := validateResultsetOutputLimitRowLimitVariableSnapshot(scenario.Steps[index], caseName); err != nil {
				return fmt.Errorf("%s case %q final setter %d snapshot: %w", resultsetOutputLimitRowLimitVariableID, caseName, setterIndex, err)
			}
			index++
		}
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetOutputLimitRowLimitVariableID)
	}
	return nil
}

func runResultsetOutputLimitRowLimitVariableScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetOutputLimitRowLimitVariableScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range resultsetOutputLimitRowLimitVariableCases {
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultsetOutputLimitRowLimitVariableCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOutputLimitRowLimitVariableID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultsetOutputLimitRowLimitVariableCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOutputLimitRowLimitVariableSupportBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetOutputLimitRowLimitVariableSupportBeanNumeric](env, "SupportBeanNumeric"); err != nil {
		return compat.Trace{}, err
	}
	intType := reflect.TypeOf(int(0))
	if err := env.RegisterVariable("myrows", int(2), esper.VariableType(intType)); err != nil {
		return compat.Trace{}, err
	}
	if err := env.RegisterVariable("myoffset", int(1), esper.VariableType(intType)); err != nil {
		return compat.Trace{}, err
	}

	intOne := esper.Cast[*int, int](esper.Field[resultsetOutputLimitRowLimitVariableSupportBeanNumeric, *int]("intOne"))
	intTwo := esper.Cast[*int, int](esper.Field[resultsetOutputLimitRowLimitVariableSupportBeanNumeric, *int]("intTwo"))
	setQuery := esper.OnEvent(esper.From[resultsetOutputLimitRowLimitVariableSupportBeanNumeric](env, "SupportBeanNumeric")).SetVariables(
		esper.SetVariableExpr("myrows", intOne),
		esper.SetVariableExpr("myoffset", intTwo),
	).Query(esper.StatementName("set"))
	setPlan, err := env.Build(setQuery)
	if err != nil {
		return compat.Trace{}, err
	}

	stream := esper.From[resultsetOutputLimitRowLimitVariableSupportBean](env, "SupportBean").Window(esper.LengthWindow(5))
	query := stream.Query(
		esper.StatementName("s0"),
		esper.WithOutput(esper.OutputEvery(5)),
		esper.LimitExpression(esper.VariableRef[int]("myrows")),
		esper.OffsetExpression(esper.VariableRef[int]("myoffset")),
	)
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetOutputLimitRowLimitVariableJavaRuntimeID(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	setDeployment, err := engine.Deploy(ctx, setPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	if len(setDeployment.Statements()) != 1 || setDeployment.Statements()[0].Name() != "set" {
		return compat.Trace{}, fmt.Errorf("expected one set statement for %s", resultsetOutputLimitRowLimitVariableID)
	}
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one select statement for %s, got %d", resultsetOutputLimitRowLimitVariableID, len(statements))
	}
	statement := statements[0]
	if statement.Name() != "s0" {
		return compat.Trace{}, fmt.Errorf("expected statement s0 for %s, got %q", resultsetOutputLimitRowLimitVariableID, statement.Name())
	}
	return compat.ReplayWithStatements(ctx, engine, statement, scenario,
		decodeResultsetOutputLimitRowLimitVariablePayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetOutputLimitRowLimitVariableID, name)
			}
			return statement, nil
		})
}

func resultsetOutputLimitRowLimitVariableJavaRuntimeID(caseName string) string {
	for index, candidate := range resultsetOutputLimitRowLimitVariableCases {
		if candidate == caseName {
			return resultsetOutputLimitRowLimitVariableJavaRuntimeIDs[index]
		}
	}
	return "parity-" + resultsetOutputLimitRowLimitVariableID + "-" + caseName
}

func decodeResultsetOutputLimitRowLimitVariablePayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		return decodeResultsetOutputLimitRowLimitVariableSupportBean(step)
	case "SupportBeanNumeric":
		return decodeResultsetOutputLimitRowLimitVariableSupportBeanNumeric(step)
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOutputLimitRowLimitVariableID, step.EventType)
	}
}

func decodeResultsetOutputLimitRowLimitVariableSupportBean(step compat.Step) (resultsetOutputLimitRowLimitVariableSupportBean, error) {
	if step.EventType != "SupportBean" {
		return resultsetOutputLimitRowLimitVariableSupportBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOutputLimitRowLimitVariableSupportBean{}, fmt.Errorf("SupportBean payload: %w", err)
	}
	if err := requireResultsetOutputLimitRowLimitVariableFields(fields, "theString", "intPrimitive"); err != nil {
		return resultsetOutputLimitRowLimitVariableSupportBean{}, fmt.Errorf("SupportBean payload: %w", err)
	}
	if string(bytes.TrimSpace(fields["theString"])) == "null" {
		return resultsetOutputLimitRowLimitVariableSupportBean{}, fmt.Errorf("SupportBean payload theString must be a string")
	}
	var value resultsetOutputLimitRowLimitVariableSupportBean
	if err := json.Unmarshal(fields["theString"], &value.TheString); err != nil {
		return resultsetOutputLimitRowLimitVariableSupportBean{}, fmt.Errorf("SupportBean payload theString must be a string")
	}
	var err error
	value.IntPrimitive, err = decodeResultsetOutputLimitRowLimitVariableInteger(fields["intPrimitive"], "SupportBean payload intPrimitive")
	if err != nil {
		return resultsetOutputLimitRowLimitVariableSupportBean{}, err
	}
	value.CharPrimitive = "\u0000"
	return value, nil
}

func decodeResultsetOutputLimitRowLimitVariableSupportBeanNumeric(step compat.Step) (resultsetOutputLimitRowLimitVariableSupportBeanNumeric, error) {
	if step.EventType != "SupportBeanNumeric" {
		return resultsetOutputLimitRowLimitVariableSupportBeanNumeric{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOutputLimitRowLimitVariableSupportBeanNumeric{}, fmt.Errorf("SupportBeanNumeric payload: %w", err)
	}
	if err := requireResultsetOutputLimitRowLimitVariableFields(fields, "intOne", "intTwo"); err != nil {
		return resultsetOutputLimitRowLimitVariableSupportBeanNumeric{}, fmt.Errorf("SupportBeanNumeric payload: %w", err)
	}
	intOne, err := decodeResultsetOutputLimitRowLimitVariableNullableInteger(fields["intOne"], "SupportBeanNumeric payload intOne")
	if err != nil {
		return resultsetOutputLimitRowLimitVariableSupportBeanNumeric{}, err
	}
	intTwo, err := decodeResultsetOutputLimitRowLimitVariableNullableInteger(fields["intTwo"], "SupportBeanNumeric payload intTwo")
	if err != nil {
		return resultsetOutputLimitRowLimitVariableSupportBeanNumeric{}, err
	}
	return resultsetOutputLimitRowLimitVariableSupportBeanNumeric{IntOne: intOne, IntTwo: intTwo}, nil
}

type resultsetOutputLimitRowLimitVariableSetter struct {
	IntOne *int
	IntTwo *int
}

func validateResultsetOutputLimitRowLimitVariableBeanSend(step compat.Step, caseName string, eventNumber int) error {
	if step.Op != "send" || step.Case != caseName || step.EventType != "SupportBean" {
		return fmt.Errorf("SupportBean send is not pinned")
	}
	value, err := decodeResultsetOutputLimitRowLimitVariableSupportBean(step)
	if err != nil {
		return err
	}
	if value.TheString != fmt.Sprintf("E%d", eventNumber) || value.IntPrimitive != eventNumber {
		return fmt.Errorf("SupportBean payload is not pinned")
	}
	return nil
}

func validateResultsetOutputLimitRowLimitVariableNumericSend(step compat.Step, caseName string, expected resultsetOutputLimitRowLimitVariableSetter) error {
	if step.Op != "send" || step.Case != caseName || step.EventType != "SupportBeanNumeric" {
		return fmt.Errorf("SupportBeanNumeric send is not pinned")
	}
	value, err := decodeResultsetOutputLimitRowLimitVariableSupportBeanNumeric(step)
	if err != nil {
		return err
	}
	if !resultsetOutputLimitRowLimitVariableNullableIntEqual(value.IntOne, expected.IntOne) ||
		!resultsetOutputLimitRowLimitVariableNullableIntEqual(value.IntTwo, expected.IntTwo) {
		return fmt.Errorf("SupportBeanNumeric payload is not pinned")
	}
	return nil
}

func validateResultsetOutputLimitRowLimitVariableSnapshot(step compat.Step, caseName string) error {
	if step.Op != "snapshot" || step.Case != caseName || step.Statement != "s0" {
		return fmt.Errorf("snapshot is not pinned")
	}
	return nil
}

func resultsetOutputLimitRowLimitVariableFinalSetters() []resultsetOutputLimitRowLimitVariableSetter {
	return []resultsetOutputLimitRowLimitVariableSetter{
		{IntOne: resultsetOutputLimitRowLimitVariableIntPointer(1), IntTwo: resultsetOutputLimitRowLimitVariableIntPointer(1)},
		{IntOne: resultsetOutputLimitRowLimitVariableIntPointer(2), IntTwo: resultsetOutputLimitRowLimitVariableIntPointer(1)},
		{IntOne: resultsetOutputLimitRowLimitVariableIntPointer(1), IntTwo: resultsetOutputLimitRowLimitVariableIntPointer(2)},
		{IntOne: resultsetOutputLimitRowLimitVariableIntPointer(6), IntTwo: resultsetOutputLimitRowLimitVariableIntPointer(6)},
		{IntOne: resultsetOutputLimitRowLimitVariableIntPointer(1), IntTwo: resultsetOutputLimitRowLimitVariableIntPointer(4)},
		{IntOne: nil, IntTwo: nil},
		{IntOne: nil, IntTwo: resultsetOutputLimitRowLimitVariableIntPointer(2)},
		{IntOne: resultsetOutputLimitRowLimitVariableIntPointer(2), IntTwo: nil},
		{IntOne: resultsetOutputLimitRowLimitVariableIntPointer(-1), IntTwo: resultsetOutputLimitRowLimitVariableIntPointer(4)},
		{IntOne: resultsetOutputLimitRowLimitVariableIntPointer(-1), IntTwo: resultsetOutputLimitRowLimitVariableIntPointer(0)},
		{IntOne: resultsetOutputLimitRowLimitVariableIntPointer(0), IntTwo: resultsetOutputLimitRowLimitVariableIntPointer(0)},
	}
}

func resultsetOutputLimitRowLimitVariableIntPointer(value int) *int {
	return &value
}

func resultsetOutputLimitRowLimitVariableNullableIntEqual(left, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func requireResultsetOutputLimitRowLimitVariableFields(object map[string]json.RawMessage, expected ...string) error {
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

func validateResultsetOutputLimitRowLimitVariableStringArray(raw json.RawMessage, expected []string, name string) error {
	var actual []string
	if err := json.Unmarshal(raw, &actual); err != nil || actual == nil || len(actual) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}

func decodeResultsetOutputLimitRowLimitVariableNullableInteger(raw json.RawMessage, name string) (*int, error) {
	if string(bytes.TrimSpace(raw)) == "null" {
		return nil, nil
	}
	value, err := decodeResultsetOutputLimitRowLimitVariableInteger(raw, name)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func decodeResultsetOutputLimitRowLimitVariableInteger(raw json.RawMessage, name string) (int, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	number, ok := token.(json.Number)
	if !ok || number == "" {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	value, err := strconv.ParseInt(string(number), 10, 64)
	if err != nil || int64(int(value)) != value {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	return int(value), nil
}

func rejectResultsetOutputLimitRowLimitVariableDuplicateKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkResultsetOutputLimitRowLimitVariableJSON(decoder); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON")
		}
		return err
	}
	return nil
}

func walkResultsetOutputLimitRowLimitVariableJSON(decoder *json.Decoder) error {
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
			if err := walkResultsetOutputLimitRowLimitVariableJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("JSON object is not closed")
		}
	case '[':
		for decoder.More() {
			if err := walkResultsetOutputLimitRowLimitVariableJSON(decoder); err != nil {
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
