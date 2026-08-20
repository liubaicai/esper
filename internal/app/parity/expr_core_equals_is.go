package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type exprCoreEqualsIsSupportBean struct {
	TheString     *string `json:"theString" esper:"theString"`
	IntPrimitive  int     `json:"intPrimitive" esper:"intPrimitive"`
	LongPrimitive int64   `json:"longPrimitive" esper:"longPrimitive"`
}

type exprCoreEqualsIsSupportBeanS0 struct {
	ID  int     `json:"id" esper:"id"`
	P00 *string `json:"p00" esper:"p00"`
	P01 *string `json:"p01" esper:"p01"`
	P02 *string `json:"p02" esper:"p02"`
}

type exprCoreEqualsIsArrayEvent struct {
	ID          string  `json:"id" esper:"id"`
	IntOne      []int   `json:"intOne" esper:"intOne"`
	IntTwo      []int   `json:"intTwo" esper:"intTwo"`
	IntBoxedOne []*int  `json:"intBoxedOne" esper:"intBoxedOne"`
	IntBoxedTwo []*int  `json:"intBoxedTwo" esper:"intBoxedTwo"`
	Int2DimOne  [][]int `json:"int2DimOne" esper:"int2DimOne"`
	Int2DimTwo  [][]int `json:"int2DimTwo" esper:"int2DimTwo"`
	ObjectOne   []any   `json:"objectOne" esper:"objectOne"`
	ObjectTwo   []any   `json:"objectTwo" esper:"objectTwo"`
}

const exprCoreEqualsIsJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprCoreEqualsIsJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreEqualsIs.java",
}

var exprCoreEqualsIsJavaRuntimeIDs = []string{
	"java-runtime-fdef3bed6ec0b16d36db",
	"java-runtime-1eaef3b328c31a863b26",
	"java-runtime-2fb582ea3ac2dc026c82",
	"java-runtime-d6084d5b7a191cbde6cd",
}

var exprCoreEqualsIsJavaExecutions = []string{
	"ExprCoreEqualsIsCoercion",
	"ExprCoreEqualsIsCoercionSameType",
	"ExprCoreEqualsIsMultikeyWArray",
	"ExprCoreEqualsNull",
}

var exprCoreEqualsIsCaseOrder = []string{
	"equals-coercion",
	"equals-same-type",
	"equals-array",
	"equals-null",
}

type exprCoreEqualsIsExpectedSend struct {
	eventType string
	fields    map[string]string
}

var exprCoreEqualsIsExpectedSends = [][]exprCoreEqualsIsExpectedSend{
	{
		{eventType: "SupportBean", fields: map[string]string{
			"intPrimitive": "1", "longPrimitive": "1",
		}},
		{eventType: "SupportBean", fields: map[string]string{
			"intPrimitive": "1", "longPrimitive": "2",
		}},
	},
	{
		{eventType: "SupportBean_S0", fields: map[string]string{
			"id": "1", "p00": `"a"`, "p01": `"a"`, "p02": `"a"`,
		}},
		{eventType: "SupportBean_S0", fields: map[string]string{
			"id": "1", "p00": `"a"`, "p01": `"b"`, "p02": "null",
		}},
	},
	{
		{eventType: "SupportEventWithManyArray", fields: map[string]string{
			"id":          `"E1"`,
			"intOne":      `[1, 2]`,
			"intTwo":      `[1, 2]`,
			"intBoxedOne": `[1, 2]`,
			"intBoxedTwo": `[1, 2]`,
			"int2DimOne":  `[[1, 2], [3, 4]]`,
			"int2DimTwo":  `[[1, 2], [3, 4]]`,
			"objectOne":   `["a", [1]]`,
			"objectTwo":   `["a", [1]]`,
		}},
		{eventType: "SupportEventWithManyArray", fields: map[string]string{
			"id":          `"E1"`,
			"intOne":      `[1, 2]`,
			"intTwo":      `[1]`,
			"intBoxedOne": `[1, 2]`,
			"intBoxedTwo": `[1]`,
			"int2DimOne":  `[[1, 2], [3, 4]]`,
			"int2DimTwo":  `[[1, 2], [3]]`,
			"objectOne":   `["a", 2]`,
			"objectTwo":   `["a"]`,
		}},
	},
	{
		{eventType: "SupportBean", fields: map[string]string{"theString": `"x"`}},
		{eventType: "SupportBean", fields: map[string]string{"theString": "null"}},
	},
}

func runExprCoreEqualsIsScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateExprCoreEqualsIsScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(exprCoreEqualsIsCaseOrder))
	for _, caseName := range exprCoreEqualsIsCaseOrder {
		trace, err := runExprCoreEqualsIsCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("expr-core-equals-is case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func validateExprCoreEqualsIsScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != "expr-core-equals-is" {
		return fmt.Errorf("expr-core-equals-is scenario has id %q", scenario.ID)
	}
	stepIndex := 0
	for caseIndex, caseName := range exprCoreEqualsIsCaseOrder {
		if stepIndex >= len(scenario.Steps) {
			return fmt.Errorf("expr-core-equals-is scenario is missing case %q", caseName)
		}
		marker := scenario.Steps[stepIndex]
		if marker.Op != "case" || marker.Case != caseName {
			return fmt.Errorf("expr-core-equals-is step %d has case %q; want %q", stepIndex, marker.Case, caseName)
		}
		stepIndex++
		for sendIndex, expected := range exprCoreEqualsIsExpectedSends[caseIndex] {
			if stepIndex >= len(scenario.Steps) {
				return fmt.Errorf("expr-core-equals-is case %q is missing send %d", caseName, sendIndex)
			}
			step := scenario.Steps[stepIndex]
			if step.Op != "send" || step.EventType != expected.eventType {
				return fmt.Errorf("expr-core-equals-is case %q send %d has op %q/event type %q; want %q", caseName, sendIndex, step.Op, step.EventType, expected.eventType)
			}
			if err := validateExprCoreEqualsIsPayload(expected.fields, step.Payload); err != nil {
				return fmt.Errorf("expr-core-equals-is case %q send %d: %w", caseName, sendIndex, err)
			}
			stepIndex++
		}
	}
	if stepIndex != len(scenario.Steps) {
		return fmt.Errorf("expr-core-equals-is scenario has %d trailing steps", len(scenario.Steps)-stepIndex)
	}
	return nil
}

func validateExprCoreEqualsIsPayload(expected map[string]string, raw json.RawMessage) error {
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

func runExprCoreEqualsIsCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	var query esper.Query
	var decode compat.DecodePayload
	switch caseName {
	case "equals-coercion":
		if _, err := esper.RegisterStruct[exprCoreEqualsIsSupportBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[exprCoreEqualsIsSupportBean](env, "SupportBean")
		intValue := esper.Field[exprCoreEqualsIsSupportBean, int]("intPrimitive")
		longValue := esper.Field[exprCoreEqualsIsSupportBean, int64]("longPrimitive")
		query = esper.Select(input,
			esper.Alias("c0", esper.EqualOf(intValue, longValue)),
			esper.Alias("c1", esper.Is(intValue, longValue)),
		).Query(esper.StatementName("s0"))
		decode = decodeExprCoreEqualsIsPayload
	case "equals-same-type":
		if _, err := esper.RegisterStruct[exprCoreEqualsIsSupportBeanS0](env, "SupportBean_S0"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[exprCoreEqualsIsSupportBeanS0](env, "SupportBean_S0")
		query = esper.Select(input,
			esper.Alias("c0", esper.EqualOf(
				esper.Field[exprCoreEqualsIsSupportBeanS0, *string]("p00"),
				esper.Field[exprCoreEqualsIsSupportBeanS0, *string]("p01"),
			)),
			esper.Alias("c1", esper.EqualOf(
				esper.Field[exprCoreEqualsIsSupportBeanS0, int]("id"),
				esper.Field[exprCoreEqualsIsSupportBeanS0, int]("id"),
			)),
			esper.Alias("c2", esper.IsNot(
				esper.Field[exprCoreEqualsIsSupportBeanS0, *string]("p02"),
				esper.NullLiteral[string](),
			)),
		).Query(esper.StatementName("s0"))
		decode = decodeExprCoreEqualsIsPayload
	case "equals-array":
		if _, err := esper.RegisterStruct[exprCoreEqualsIsArrayEvent](env, "SupportEventWithManyArray"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[exprCoreEqualsIsArrayEvent](env, "SupportEventWithManyArray")
		intOne := esper.Field[exprCoreEqualsIsArrayEvent, []int]("intOne")
		intTwo := esper.Field[exprCoreEqualsIsArrayEvent, []int]("intTwo")
		boxedOne := esper.Field[exprCoreEqualsIsArrayEvent, []*int]("intBoxedOne")
		boxedTwo := esper.Field[exprCoreEqualsIsArrayEvent, []*int]("intBoxedTwo")
		dimOne := esper.Field[exprCoreEqualsIsArrayEvent, [][]int]("int2DimOne")
		dimTwo := esper.Field[exprCoreEqualsIsArrayEvent, [][]int]("int2DimTwo")
		objectOne := esper.Field[exprCoreEqualsIsArrayEvent, []any]("objectOne")
		objectTwo := esper.Field[exprCoreEqualsIsArrayEvent, []any]("objectTwo")
		query = esper.Select(input,
			esper.Alias("c0", esper.EqualOf(intOne, intTwo)),
			esper.Alias("c1", esper.Is(intOne, intTwo)),
			esper.Alias("c2", esper.EqualOf(boxedOne, boxedTwo)),
			esper.Alias("c3", esper.Is(boxedOne, boxedTwo)),
			esper.Alias("c4", esper.EqualOf(dimOne, dimTwo)),
			esper.Alias("c5", esper.Is(dimOne, dimTwo)),
			esper.Alias("c6", esper.EqualOf(objectOne, objectTwo)),
			esper.Alias("c7", esper.Is(objectOne, objectTwo)),
		).Query(esper.StatementName("s0"))
		decode = decodeExprCoreEqualsIsPayload
	case "equals-null":
		if _, err := esper.RegisterStruct[exprCoreEqualsIsSupportBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[exprCoreEqualsIsSupportBean](env, "SupportBean")
		value := esper.Field[exprCoreEqualsIsSupportBean, *string]("theString")
		nullValue := esper.NullLiteral[string]()
		query = esper.Select(input,
			esper.Alias("c0", esper.EqualOf(value, nullValue)),
			esper.Alias("c1", esper.Is(value, nullValue)),
			esper.Alias("c2", esper.EqualOf(nullValue, value)),
			esper.Alias("c3", esper.Is(nullValue, value)),
			esper.Alias("c4", esper.EqualOf(nullValue, nullValue)),
			esper.Alias("c5", esper.Is(nullValue, nullValue)),
		).Query(esper.StatementName("s0"))
		decode = decodeExprCoreEqualsIsPayload
	default:
		return compat.Trace{}, fmt.Errorf("unsupported expr-core-equals-is case %q", caseName)
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
			return nil, fmt.Errorf("unknown expr-core-equals-is statement %q", name)
		}
		return statement, nil
	})
}

func decodeExprCoreEqualsIsPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value exprCoreEqualsIsSupportBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("expr-core-equals-is: decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value exprCoreEqualsIsSupportBeanS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("expr-core-equals-is: decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportEventWithManyArray":
		var value exprCoreEqualsIsArrayEvent
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("expr-core-equals-is: decode SupportEventWithManyArray: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("expr-core-equals-is: unsupported event type %q", step.EventType)
	}
}
