package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const exprCoreInstanceOfJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprCoreInstanceOfJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreInstanceOf.java",
}

var exprCoreInstanceOfJavaRuntimeIDs = []string{
	"java-runtime-f57ec2f2f04aad28961d",
	"java-runtime-fc4c87ba7677b72cab9a",
	"java-runtime-c5af420270464e5633f7",
	"java-runtime-0423d81802ef0e7eb910",
	"java-runtime-f446c95462162907b0c6",
}

var exprCoreInstanceOfJavaExecutions = []string{
	"ExprCoreInstanceofSimple",
	"ExprCoreInstanceofStringAndNullOM",
	"ExprCoreInstanceofStringAndNullCompile",
	"ExprCoreDynamicPropertyJavaTypes",
	"ExprCoreDynamicSuperTypeAndInterface",
}

var exprCoreInstanceOfCaseOrder = []string{
	"instanceof-simple",
	"instanceof-string-om",
	"instanceof-string-compile",
	"instanceof-dynamic-types",
	"instanceof-dynamic-hierarchy",
}

type exprCoreInstanceOfExpectedSend struct {
	eventType string
	fields    map[string]string
}

var exprCoreInstanceOfExpectedSends = [][]exprCoreInstanceOfExpectedSend{
	{
		{eventType: "SupportBean", fields: map[string]string{
			"theString":    `"abc"`,
			"intPrimitive": "100",
			"intBoxed":     "null",
			"floatBoxed":   "100.0",
		}},
		{eventType: "SupportBean", fields: map[string]string{
			"theString":    "null",
			"intPrimitive": "100",
			"intBoxed":     "null",
			"floatBoxed":   "null",
		}},
	},
	{
		{eventType: "SupportBean", fields: map[string]string{
			"theString":    `"abc"`,
			"intPrimitive": "100",
		}},
		{eventType: "SupportBean", fields: map[string]string{
			"theString":    "null",
			"intPrimitive": "100",
		}},
	},
	{
		{eventType: "SupportBean", fields: map[string]string{
			"theString":    `"abc"`,
			"intPrimitive": "100",
		}},
		{eventType: "SupportBean", fields: map[string]string{
			"theString":    "null",
			"intPrimitive": "100",
		}},
	},
	{
		{eventType: "SupportBeanDynRoot", fields: map[string]string{
			"itemType":  `"string"`,
			"itemValue": `"abc"`,
		}},
		{eventType: "SupportBeanDynRoot", fields: map[string]string{
			"itemType":  `"float32"`,
			"itemValue": "100",
		}},
		{eventType: "SupportBeanDynRoot", fields: map[string]string{
			"itemType": `"null"`,
		}},
		{eventType: "SupportBeanDynRoot", fields: map[string]string{
			"itemType":  `"int"`,
			"itemValue": "10",
		}},
		{eventType: "SupportBeanDynRoot", fields: map[string]string{
			"itemType":  `"int64"`,
			"itemValue": "99",
		}},
	},
	{
		{eventType: "SupportBeanDynRoot", fields: map[string]string{
			"itemType":  `"dyn-root"`,
			"itemValue": `"abc"`,
		}},
		{eventType: "SupportBeanDynRoot", fields: map[string]string{
			"itemType": `"plus"`,
		}},
		{eventType: "SupportBeanDynRoot", fields: map[string]string{
			"itemType":    `"superg-impl"`,
			"valueG":      `""`,
			"valueA":      `""`,
			"valueBaseAB": `""`,
		}},
		{eventType: "SupportBeanDynRoot", fields: map[string]string{
			"itemType":    `"base-ab-impl"`,
			"valueBaseAB": `""`,
		}},
		{eventType: "SupportBeanDynRoot", fields: map[string]string{
			"itemType":    `"b-impl"`,
			"valueB":      `""`,
			"valueBaseAB": `""`,
		}},
		{eventType: "SupportBeanDynRoot", fields: map[string]string{
			"itemType":    `"a-impl"`,
			"valueA":      `""`,
			"valueBaseAB": `""`,
		}},
	},
}

type exprCoreInstanceOfMarker interface {
	exprCoreInstanceOfMarker()
}

type exprCoreInstanceOfBaseAB interface {
	exprCoreInstanceOfBaseAB()
}

type exprCoreInstanceOfA interface {
	exprCoreInstanceOfBaseAB
	exprCoreInstanceOfA()
}

type exprCoreInstanceOfB interface {
	exprCoreInstanceOfBaseAB
	exprCoreInstanceOfB()
}

type exprCoreInstanceOfSuperG interface {
	exprCoreInstanceOfA
	exprCoreInstanceOfSuperG()
}

type exprCoreInstanceOfDynamicRoot struct {
	value string
}

func (*exprCoreInstanceOfDynamicRoot) exprCoreInstanceOfMarker() {}

type exprCoreInstanceOfPlus struct{}

func (*exprCoreInstanceOfPlus) exprCoreInstanceOfBaseAB() {}
func (*exprCoreInstanceOfPlus) exprCoreInstanceOfA()      {}
func (*exprCoreInstanceOfPlus) exprCoreInstanceOfB()      {}
func (*exprCoreInstanceOfPlus) exprCoreInstanceOfSuperG() {}

type exprCoreInstanceOfSuperGImpl struct {
	valueG      string
	valueA      string
	valueBaseAB string
}

func (*exprCoreInstanceOfSuperGImpl) exprCoreInstanceOfBaseAB() {}
func (*exprCoreInstanceOfSuperGImpl) exprCoreInstanceOfA()      {}
func (*exprCoreInstanceOfSuperGImpl) exprCoreInstanceOfSuperG() {}

type exprCoreInstanceOfBaseABImpl struct {
	valueBaseAB string
}

func (*exprCoreInstanceOfBaseABImpl) exprCoreInstanceOfBaseAB() {}

type exprCoreInstanceOfBImpl struct {
	valueB      string
	valueBaseAB string
}

func (*exprCoreInstanceOfBImpl) exprCoreInstanceOfBaseAB() {}
func (*exprCoreInstanceOfBImpl) exprCoreInstanceOfB()      {}

type exprCoreInstanceOfAImpl struct {
	valueA      string
	valueBaseAB string
}

func (*exprCoreInstanceOfAImpl) exprCoreInstanceOfBaseAB() {}
func (*exprCoreInstanceOfAImpl) exprCoreInstanceOfA()      {}

type exprCoreInstanceOfAtoFBase struct{}

func exprCoreInstanceOfAnyType() reflect.Type {
	return reflect.TypeOf((*any)(nil)).Elem()
}

func registerExprCoreInstanceOfMap(env *esper.Environment, name string, fields ...string) error {
	fieldSpecs := make([]esper.FieldSpec, 0, len(fields))
	for _, field := range fields {
		fieldSpecs = append(fieldSpecs, esper.FieldDef(field, exprCoreInstanceOfAnyType()))
	}
	_, err := esper.RegisterMap(env, name, fieldSpecs, esper.AllowDynamicFields())
	return err
}

func exprCoreInstanceOfAnyOf(value esper.Expr, predicates ...esper.Expression[bool]) esper.Expression[bool] {
	if len(predicates) == 0 {
		return esper.Literal(false)
	}
	result := predicates[0]
	for _, predicate := range predicates[1:] {
		result = esper.Or(result, predicate)
	}
	return result
}

func exprCoreInstanceOfNumeric(value esper.Expr) esper.Expression[bool] {
	return exprCoreInstanceOfAnyOf(value,
		esper.InstanceOf[int](value),
		esper.InstanceOf[int64](value),
		esper.InstanceOf[float32](value),
	)
}

func runExprCoreInstanceOfScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateExprCoreInstanceOfScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(exprCoreInstanceOfCaseOrder))
	for _, caseName := range exprCoreInstanceOfCaseOrder {
		trace, err := runExprCoreInstanceOfCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("expr-core-instanceof %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func validateExprCoreInstanceOfScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != "expr-core-instanceof" {
		return fmt.Errorf("expr-core-instanceof scenario has id %q", scenario.ID)
	}
	stepIndex := 0
	for caseIndex, caseName := range exprCoreInstanceOfCaseOrder {
		if stepIndex >= len(scenario.Steps) {
			return fmt.Errorf("expr-core-instanceof scenario is missing case %q", caseName)
		}
		marker := scenario.Steps[stepIndex]
		if marker.Op != "case" || marker.Case != caseName {
			return fmt.Errorf("expr-core-instanceof step %d has case %q; want %q", stepIndex, marker.Case, caseName)
		}
		if err := validateExprCoreInstanceOfStepMetadata(marker, "case"); err != nil {
			return fmt.Errorf("expr-core-instanceof step %d: %w", stepIndex, err)
		}
		stepIndex++
		for sendIndex, expected := range exprCoreInstanceOfExpectedSends[caseIndex] {
			if stepIndex >= len(scenario.Steps) {
				return fmt.Errorf("expr-core-instanceof case %q is missing send %d", caseName, sendIndex)
			}
			step := scenario.Steps[stepIndex]
			if step.Op != "send" || step.EventType != expected.eventType {
				return fmt.Errorf("expr-core-instanceof case %q send %d has op %q/event type %q; want %q", caseName, sendIndex, step.Op, step.EventType, expected.eventType)
			}
			if err := validateExprCoreInstanceOfStepMetadata(step, "send"); err != nil {
				return fmt.Errorf("expr-core-instanceof case %q send %d: %w", caseName, sendIndex, err)
			}
			if err := validateExprCoreInstanceOfPayload(expected.fields, step.Payload); err != nil {
				return fmt.Errorf("expr-core-instanceof case %q send %d: %w", caseName, sendIndex, err)
			}
			stepIndex++
		}
	}
	if stepIndex != len(scenario.Steps) {
		return fmt.Errorf("expr-core-instanceof scenario has %d trailing steps", len(scenario.Steps)-stepIndex)
	}
	return nil
}

func validateExprCoreInstanceOfStepMetadata(step compat.Step, op string) error {
	if op == "case" {
		if step.Statement != "" || step.Selector != "" || len(step.Hashes) != 0 || len(step.IDs) != 0 ||
			step.FilterProperty != "" || step.FilterValue != "" || step.ExpectError != "" ||
			step.EventType != "" || step.At != "" || step.Name != "" || step.Source != "" ||
			len(step.PropertyOrder) != 0 || len(step.PropertyTypes) != 0 || len(step.Payload) != 0 {
			return fmt.Errorf("case step has unsupported metadata")
		}
		return nil
	}
	if step.Case != "" || step.Statement != "" || step.Selector != "" || len(step.Hashes) != 0 || len(step.IDs) != 0 ||
		step.FilterProperty != "" || step.FilterValue != "" || step.ExpectError != "" || step.At != "" ||
		step.Name != "" || step.Source != "" || len(step.PropertyOrder) != 0 || len(step.PropertyTypes) != 0 {
		return fmt.Errorf("send step has unsupported metadata")
	}
	return nil
}

func validateExprCoreInstanceOfPayload(expected map[string]string, raw json.RawMessage) error {
	actual := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &actual); err != nil {
		return fmt.Errorf("decode payload object: %w", err)
	}
	if len(actual) != len(expected) {
		return fmt.Errorf("payload fields = %d; want %d", len(actual), len(expected))
	}
	for field, want := range expected {
		value, ok := actual[field]
		if !ok {
			return fmt.Errorf("payload is missing field %q", field)
		}
		if string(value) != want {
			return fmt.Errorf("payload field %q = %s; want %s", field, value, want)
		}
	}
	return nil
}

func runExprCoreInstanceOfCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	var query esper.Query
	switch caseName {
	case "instanceof-simple":
		if err := registerExprCoreInstanceOfMap(env, "SupportBean", "theString", "intPrimitive", "intBoxed", "floatBoxed"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "SupportBean")
		theString := esper.Field[map[string]any, any]("theString")
		intBoxed := esper.Field[map[string]any, any]("intBoxed")
		floatBoxed := esper.Field[map[string]any, any]("floatBoxed")
		intPrimitive := esper.Field[map[string]any, any]("intPrimitive")
		query = esper.Select(input,
			esper.Alias("c0", esper.InstanceOf[string](theString)),
			esper.Alias("c1", esper.InstanceOf[int](intBoxed)),
			esper.Alias("c2", esper.InstanceOf[float32](floatBoxed)),
			esper.Alias("c3", exprCoreInstanceOfAnyOf(theString,
				esper.InstanceOf[float32](theString),
				esper.InstanceOf[uint16](theString),
				esper.InstanceOf[int8](theString))),
			esper.Alias("c4", esper.InstanceOf[int](intPrimitive)),
			esper.Alias("c5", esper.InstanceOf[int64](intPrimitive)),
			esper.Alias("c6", exprCoreInstanceOfNumeric(intPrimitive)),
			esper.Alias("c7", exprCoreInstanceOfAnyOf(floatBoxed,
				esper.InstanceOf[int64](floatBoxed),
				esper.InstanceOf[float32](floatBoxed))),
		).Query(esper.StatementName("s0"))
	case "instanceof-string-om", "instanceof-string-compile":
		if err := registerExprCoreInstanceOfMap(env, "SupportBean", "theString", "intPrimitive"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "SupportBean")
		theString := esper.Field[map[string]any, any]("theString")
		query = esper.Select(input,
			esper.Alias("t0", esper.InstanceOf[string](theString)),
			esper.Alias("t1", exprCoreInstanceOfAnyOf(theString,
				esper.InstanceOf[float32](theString),
				esper.InstanceOf[string](theString),
				esper.InstanceOf[int](theString))),
		).Query(esper.StatementName("s0"))
	case "instanceof-dynamic-types":
		if err := registerExprCoreInstanceOfMap(env, "SupportBeanDynRoot", "item"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "SupportBeanDynRoot")
		item := esper.Field[map[string]any, any]("item")
		query = esper.Select(input,
			esper.Alias("t0", esper.InstanceOf[string](item)),
			esper.Alias("t1", esper.InstanceOf[int](item)),
			esper.Alias("t2", esper.InstanceOf[float32](item)),
			esper.Alias("t3", exprCoreInstanceOfAnyOf(item,
				esper.InstanceOf[float32](item),
				esper.InstanceOf[uint16](item),
				esper.InstanceOf[int8](item))),
			esper.Alias("t4", esper.InstanceOf[int](item)),
			esper.Alias("t5", esper.InstanceOf[int64](item)),
			esper.Alias("t6", exprCoreInstanceOfNumeric(item)),
			esper.Alias("t7", exprCoreInstanceOfAnyOf(item,
				esper.InstanceOf[int64](item),
				esper.InstanceOf[float32](item))),
		).Query(esper.StatementName("s0"))
	case "instanceof-dynamic-hierarchy":
		if err := registerExprCoreInstanceOfMap(env, "SupportBeanDynRoot", "item"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "SupportBeanDynRoot")
		item := esper.Field[map[string]any, any]("item")
		query = esper.Select(input,
			esper.Alias("t0", esper.InstanceOf[exprCoreInstanceOfMarker](item)),
			esper.Alias("t1", esper.InstanceOf[exprCoreInstanceOfA](item)),
			esper.Alias("t2", esper.InstanceOf[exprCoreInstanceOfBaseAB](item)),
			esper.Alias("t3", esper.InstanceOf[*exprCoreInstanceOfBaseABImpl](item)),
			esper.Alias("t4", exprCoreInstanceOfAnyOf(item,
				esper.InstanceOf[exprCoreInstanceOfA](item),
				esper.InstanceOf[exprCoreInstanceOfB](item))),
			esper.Alias("t5", exprCoreInstanceOfAnyOf(item,
				esper.InstanceOf[exprCoreInstanceOfBaseAB](item),
				esper.InstanceOf[exprCoreInstanceOfB](item))),
			esper.Alias("t6", exprCoreInstanceOfAnyOf(item,
				esper.InstanceOf[exprCoreInstanceOfSuperG](item),
				esper.InstanceOf[exprCoreInstanceOfB](item))),
			esper.Alias("t7", exprCoreInstanceOfAnyOf(item,
				esper.InstanceOf[*exprCoreInstanceOfPlus](item),
				esper.InstanceOf[*exprCoreInstanceOfAtoFBase](item))),
		).Query(esper.StatementName("s0"))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported expr-core-instanceof case %q", caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeExprCoreInstanceOfPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown expr-core-instanceof statement %q", name)
		}
		return statement, nil
	})
}

type exprCoreInstanceOfSupportBeanPayload struct {
	TheString    *string  `json:"theString"`
	IntPrimitive int      `json:"intPrimitive"`
	IntBoxed     *int     `json:"intBoxed"`
	FloatBoxed   *float32 `json:"floatBoxed"`
}

type exprCoreInstanceOfDynamicPayload struct {
	ItemType    string          `json:"itemType"`
	ItemValue   json.RawMessage `json:"itemValue"`
	ValueG      string          `json:"valueG"`
	ValueA      string          `json:"valueA"`
	ValueBaseAB string          `json:"valueBaseAB"`
	ValueB      string          `json:"valueB"`
}

func decodeExprCoreInstanceOfPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var payload exprCoreInstanceOfSupportBeanPayload
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("expr-core-instanceof: decode SupportBean: %w", err)
		}
		value := map[string]any{
			"theString":    nil,
			"intPrimitive": payload.IntPrimitive,
			"intBoxed":     nil,
			"floatBoxed":   nil,
		}
		if payload.TheString != nil {
			value["theString"] = *payload.TheString
		}
		if payload.IntBoxed != nil {
			value["intBoxed"] = *payload.IntBoxed
		}
		if payload.FloatBoxed != nil {
			value["floatBoxed"] = *payload.FloatBoxed
		}
		return value, nil
	case "SupportBeanDynRoot":
		var payload exprCoreInstanceOfDynamicPayload
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("expr-core-instanceof: decode SupportBeanDynRoot: %w", err)
		}
		item, err := decodeExprCoreInstanceOfItem(payload)
		if err != nil {
			return nil, err
		}
		return map[string]any{"item": item}, nil
	default:
		return nil, fmt.Errorf("expr-core-instanceof: unsupported event type %q", step.EventType)
	}
}

func decodeExprCoreInstanceOfItem(payload exprCoreInstanceOfDynamicPayload) (any, error) {
	switch payload.ItemType {
	case "string":
		var value string
		if err := json.Unmarshal(payload.ItemValue, &value); err != nil {
			return nil, fmt.Errorf("expr-core-instanceof: decode string item: %w", err)
		}
		return value, nil
	case "float32":
		var value float32
		if err := json.Unmarshal(payload.ItemValue, &value); err != nil {
			return nil, fmt.Errorf("expr-core-instanceof: decode float32 item: %w", err)
		}
		return value, nil
	case "int":
		var value int
		if err := json.Unmarshal(payload.ItemValue, &value); err != nil {
			return nil, fmt.Errorf("expr-core-instanceof: decode int item: %w", err)
		}
		return value, nil
	case "int64":
		var value int64
		if err := json.Unmarshal(payload.ItemValue, &value); err != nil {
			return nil, fmt.Errorf("expr-core-instanceof: decode int64 item: %w", err)
		}
		return value, nil
	case "null":
		return nil, nil
	case "dyn-root":
		var value string
		if err := json.Unmarshal(payload.ItemValue, &value); err != nil {
			return nil, fmt.Errorf("expr-core-instanceof: decode dynamic root item: %w", err)
		}
		return &exprCoreInstanceOfDynamicRoot{value: value}, nil
	case "plus":
		return &exprCoreInstanceOfPlus{}, nil
	case "superg-impl":
		return &exprCoreInstanceOfSuperGImpl{
			valueG: payload.ValueG, valueA: payload.ValueA, valueBaseAB: payload.ValueBaseAB,
		}, nil
	case "base-ab-impl":
		return &exprCoreInstanceOfBaseABImpl{valueBaseAB: payload.ValueBaseAB}, nil
	case "b-impl":
		return &exprCoreInstanceOfBImpl{valueB: payload.ValueB, valueBaseAB: payload.ValueBaseAB}, nil
	case "a-impl":
		return &exprCoreInstanceOfAImpl{valueA: payload.ValueA, valueBaseAB: payload.ValueBaseAB}, nil
	default:
		return nil, fmt.Errorf("expr-core-instanceof: unsupported itemType %q", payload.ItemType)
	}
}
