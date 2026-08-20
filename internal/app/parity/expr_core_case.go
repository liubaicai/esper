package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type exprCoreCaseMarketData struct {
	Symbol string  `json:"symbol" esper:"symbol"`
	Volume int64   `json:"volume" esper:"volume"`
	Price  float64 `json:"price" esper:"price"`
}

type exprCoreCaseBean struct {
	IntPrimitive    int     `json:"intPrimitive" esper:"intPrimitive"`
	LongPrimitive   int64   `json:"longPrimitive" esper:"longPrimitive"`
	FloatPrimitive  float32 `json:"floatPrimitive" esper:"floatPrimitive"`
	DoublePrimitive float64 `json:"doublePrimitive" esper:"doublePrimitive"`
}

const exprCoreCaseJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprCoreCaseJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreCase.java",
}

var exprCoreCaseJavaRuntimeIDs = []string{
	"java-runtime-e9ae8d32b9f6155877ef",
	"java-runtime-725a9999f48d69d792a2",
	"java-runtime-0f635579498bb6de3a4c",
}

var exprCoreCaseJavaExecutions = []string{
	"ExprCoreCaseSyntax1WithElse",
	"ExprCoreCaseSyntax1Branches3",
	"ExprCoreCaseSyntax2",
}

var exprCoreCaseOrder = []string{
	"case-with-else",
	"case-branches3",
	"case-simple-numeric",
}

type exprCoreCaseExpectedSend struct {
	eventType string
	fields    map[string]string
}

var exprCoreCaseExpectedSends = [][]exprCoreCaseExpectedSend{
	{
		{eventType: "SupportMarketDataBean", fields: map[string]string{
			"symbol": `"CSCO"`, "volume": "4000", "price": "0",
		}},
		{eventType: "SupportMarketDataBean", fields: map[string]string{
			"symbol": `"DELL"`, "volume": "20", "price": "0",
		}},
	},
	{
		{eventType: "SupportMarketDataBean", fields: map[string]string{
			"symbol": `"DELL"`, "volume": "10000", "price": "0",
		}},
		{eventType: "SupportMarketDataBean", fields: map[string]string{
			"symbol": `"MSFT"`, "volume": "10000", "price": "0",
		}},
		{eventType: "SupportMarketDataBean", fields: map[string]string{
			"symbol": `"GE"`, "volume": "10000", "price": "0",
		}},
	},
	{
		{eventType: "SupportBean", fields: map[string]string{
			"intPrimitive": "2", "longPrimitive": "2", "floatPrimitive": "1", "doublePrimitive": "1",
		}},
		{eventType: "SupportBean", fields: map[string]string{
			"intPrimitive": "5", "longPrimitive": "1", "floatPrimitive": "1", "doublePrimitive": "5",
		}},
		{eventType: "SupportBean", fields: map[string]string{
			"intPrimitive": "12", "longPrimitive": "1", "floatPrimitive": "12", "doublePrimitive": "4",
		}},
		{eventType: "SupportBean", fields: map[string]string{
			"intPrimitive": "1", "longPrimitive": "2", "floatPrimitive": "3", "doublePrimitive": "4",
		}},
	},
}

func runExprCoreCaseScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateExprCoreCaseScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(exprCoreCaseOrder))
	for _, caseName := range exprCoreCaseOrder {
		trace, err := runExprCoreCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("expr-core-case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func validateExprCoreCaseScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != "expr-core-case" {
		return fmt.Errorf("expr-core-case scenario has id %q", scenario.ID)
	}
	stepIndex := 0
	for caseIndex, caseName := range exprCoreCaseOrder {
		if stepIndex >= len(scenario.Steps) {
			return fmt.Errorf("expr-core-case scenario is missing case %q", caseName)
		}
		marker := scenario.Steps[stepIndex]
		if marker.Op != "case" || marker.Case != caseName {
			return fmt.Errorf("expr-core-case step %d has case %q; want %q", stepIndex, marker.Case, caseName)
		}
		if err := validateExprCoreCaseStepMetadata(marker, "case"); err != nil {
			return fmt.Errorf("expr-core-case step %d: %w", stepIndex, err)
		}
		stepIndex++
		for sendIndex, expected := range exprCoreCaseExpectedSends[caseIndex] {
			if stepIndex >= len(scenario.Steps) {
				return fmt.Errorf("expr-core-case case %q is missing send %d", caseName, sendIndex)
			}
			step := scenario.Steps[stepIndex]
			if step.Op != "send" || step.EventType != expected.eventType {
				return fmt.Errorf("expr-core-case case %q send %d has op %q/event type %q; want %q", caseName, sendIndex, step.Op, step.EventType, expected.eventType)
			}
			if err := validateExprCoreCaseStepMetadata(step, "send"); err != nil {
				return fmt.Errorf("expr-core-case case %q send %d: %w", caseName, sendIndex, err)
			}
			if err := validateExprCoreCasePayload(expected.fields, step.Payload); err != nil {
				return fmt.Errorf("expr-core-case case %q send %d: %w", caseName, sendIndex, err)
			}
			stepIndex++
		}
	}
	if stepIndex != len(scenario.Steps) {
		return fmt.Errorf("expr-core-case scenario has %d trailing steps", len(scenario.Steps)-stepIndex)
	}
	return nil
}

func validateExprCoreCaseStepMetadata(step compat.Step, op string) error {
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

func validateExprCoreCasePayload(expected map[string]string, raw json.RawMessage) error {
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

func runExprCoreCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	var query esper.Query
	var decode compat.DecodePayload
	switch caseName {
	case "case-with-else":
		if _, err := esper.RegisterStruct[exprCoreCaseMarketData](env, "SupportMarketDataBean"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[exprCoreCaseMarketData](env, "SupportMarketDataBean").Window(esper.LengthWindow(3))
		symbol := esper.Field[exprCoreCaseMarketData, string]("symbol")
		volume := esper.Field[exprCoreCaseMarketData, int64]("volume")
		expr := esper.CaseWhen[int64](
			esper.Equal[string](symbol, esper.Literal("DELL")),
			esper.Multiply[int64](volume, esper.Literal(int64(3))),
		).Else(volume)
		query = esper.Select(input, esper.Alias("p1", expr)).Query(esper.StatementName("s0"))
		decode = decodeExprCoreCasePayload
	case "case-branches3":
		if _, err := esper.RegisterStruct[exprCoreCaseMarketData](env, "SupportMarketDataBean"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[exprCoreCaseMarketData](env, "SupportMarketDataBean")
		symbol := esper.Field[exprCoreCaseMarketData, string]("symbol")
		volume := esper.Field[exprCoreCaseMarketData, int64]("volume")
		expr := esper.CaseWhen[float64](
			esper.Equal[string](symbol, esper.Literal("GE")),
			esper.Cast[int64, float64](volume),
		).When(
			esper.Equal[string](symbol, esper.Literal("DELL")),
			esper.DivideFloat(volume, esper.Literal(2.0)),
		).When(
			esper.Equal[string](symbol, esper.Literal("MSFT")),
			esper.DivideFloat(volume, esper.Literal(3.0)),
		).Build()
		query = esper.Select(input, esper.Alias("c0", expr)).Query(esper.StatementName("s0"))
		decode = decodeExprCoreCasePayload
	case "case-simple-numeric":
		if _, err := esper.RegisterStruct[exprCoreCaseBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[exprCoreCaseBean](env, "SupportBean")
		intValue := esper.Field[exprCoreCaseBean, int]("intPrimitive")
		longValue := esper.Field[exprCoreCaseBean, int64]("longPrimitive")
		floatValue := esper.Field[exprCoreCaseBean, float32]("floatPrimitive")
		doubleValue := esper.Field[exprCoreCaseBean, float64]("doublePrimitive")
		expr := esper.CaseValue[float64](
			intValue,
			longValue,
			esper.AddOf[float64](intValue, longValue),
		).When(
			doubleValue,
			esper.MultiplyOf[float64](intValue, doubleValue),
		).When(
			floatValue,
			esper.DivideFloat(floatValue, doubleValue),
		).Else(
			esper.AddOf[float64](
				esper.AddOf[float64](intValue, longValue),
				esper.AddOf[float64](floatValue, doubleValue),
			),
		)
		query = esper.Select(input, esper.Alias("c0", expr)).Query(esper.StatementName("s0"))
		decode = decodeExprCoreCasePayload
	default:
		return compat.Trace{}, fmt.Errorf("unsupported expr-core-case case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decode, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown expr-core-case statement %q", name)
		}
		return statement, nil
	})
}

func decodeExprCoreCasePayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportMarketDataBean":
		var value exprCoreCaseMarketData
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("expr-core-case: decode SupportMarketDataBean: %w", err)
		}
		return value, nil
	case "SupportBean":
		var value exprCoreCaseBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("expr-core-case: decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("expr-core-case: unsupported event type %q", step.EventType)
	}
}
