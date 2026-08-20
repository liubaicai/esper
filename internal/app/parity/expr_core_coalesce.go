package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type exprCoreCoalesceBean struct {
	TheString string `esper:"theString"`
}

type exprCoreCoalesceNumericBean struct {
	Byte   *int8    `json:"byteBoxed" esper:"byteBoxed"`
	Short  *int16   `json:"shortBoxed" esper:"shortBoxed"`
	Int    *int     `json:"intBoxed" esper:"intBoxed"`
	Long   *int64   `json:"longBoxed" esper:"longBoxed"`
	Float  *float32 `json:"floatBoxed" esper:"floatBoxed"`
	Double *float64 `json:"doubleBoxed" esper:"doubleBoxed"`
}

const exprCoreCoalesceJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprCoreCoalesceJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreCoalesce.java",
}

var exprCoreCoalesceJavaRuntimeIDs = []string{
	"java-runtime-6464df255cd4e23a89e7",
	"java-runtime-0a352fd0c30e1b3a175c",
	"java-runtime-0ee4f2c3f1d9129e2598",
	"java-runtime-7cac5278b06087f8e7f2",
	"java-runtime-9341a1983c1f4fb3fdda",
	"java-runtime-c9475d47ffbc6275e330",
}

var exprCoreCoalesceJavaExecutions = []string{
	"ExprCoreCoalesceBeans",
	"ExprCoreCoalesceLong",
	"ExprCoreCoalesceLongOM",
	"ExprCoreCoalesceLongCompile",
	"ExprCoreCoalesceDouble",
	"ExprCoreCoalesceNull",
}

var exprCoreCoalesceCaseOrder = []string{
	"coalesce-beans",
	"coalesce-long",
	"coalesce-long-om",
	"coalesce-long-compile",
	"coalesce-double",
	"coalesce-null",
}

func runExprCoreCoalesceScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(exprCoreCoalesceCaseOrder))
	for _, caseName := range exprCoreCoalesceCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runExprCoreCoalesceCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("expr-core-coalesce case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("expr-core-coalesce scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runExprCoreCoalesceCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	var (
		query  esper.Query
		decode compat.DecodePayload
	)
	switch caseName {
	case "coalesce-beans":
		if _, err := esper.RegisterStruct[exprCoreCoalesceBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[exprCoreCoalesceBean](env, "SupportBean")
		left := esper.PatternFrom(input, "a", esper.Equal[string](
			esper.Field[exprCoreCoalesceBean, string]("theString"), esper.Literal("s0"),
		))
		right := esper.PatternFrom(input, "b", esper.Equal[string](
			esper.Field[exprCoreCoalesceBean, string]("theString"), esper.Literal("s1"),
		))
		query = left.Or(right).Every().Select(
			esper.Alias("myString", esper.Coalesce[string](
				esper.TagField[string]("a", "theString"),
				esper.TagField[string]("b", "theString"),
			)),
			esper.Alias("myBean", esper.CoalesceOf[esper.Event](
				esper.PatternEvent("a"), esper.PatternEvent("b"),
			)),
		).Query(esper.StatementName("s0"))
		decode = decodeExprCoreCoalesceBeanPayload
	case "coalesce-long", "coalesce-long-om", "coalesce-long-compile":
		if _, err := esper.RegisterStruct[exprCoreCoalesceNumericBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[exprCoreCoalesceNumericBean](env, "SupportBean")
		query = esper.Select(input, esper.Alias("result", esper.CoalesceOf[int64](
			esper.Field[exprCoreCoalesceNumericBean, *int64]("longBoxed"),
			esper.Field[exprCoreCoalesceNumericBean, *int]("intBoxed"),
			esper.Field[exprCoreCoalesceNumericBean, *int16]("shortBoxed"),
		))).Query(esper.StatementName("s0"))
		decode = decodeExprCoreCoalesceNumericPayload
	case "coalesce-double":
		if _, err := esper.RegisterStruct[exprCoreCoalesceNumericBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[exprCoreCoalesceNumericBean](env, "SupportBean")
		query = esper.Select(input, esper.Alias("c0", esper.CoalesceOf[float64](
			esper.NullLiteral[float64](),
			esper.Field[exprCoreCoalesceNumericBean, *int8]("byteBoxed"),
			esper.Field[exprCoreCoalesceNumericBean, *int16]("shortBoxed"),
			esper.Field[exprCoreCoalesceNumericBean, *int]("intBoxed"),
			esper.Field[exprCoreCoalesceNumericBean, *int64]("longBoxed"),
			esper.Field[exprCoreCoalesceNumericBean, *float32]("floatBoxed"),
			esper.Field[exprCoreCoalesceNumericBean, *float64]("doubleBoxed"),
		))).Query(esper.StatementName("s0"))
		decode = decodeExprCoreCoalesceNumericPayload
	case "coalesce-null":
		if _, err := esper.RegisterStruct[exprCoreCoalesceNumericBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[exprCoreCoalesceNumericBean](env, "SupportBean")
		query = esper.Select(input, esper.Alias("c0", esper.CoalesceOf[any](
			esper.NullLiteral[any](), esper.NullLiteral[any](),
		))).Query(esper.StatementName("s0"))
		decode = decodeExprCoreCoalesceNumericPayload
	default:
		return compat.Trace{}, fmt.Errorf("unsupported expr-core-coalesce case %q", caseName)
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
			return nil, fmt.Errorf("unknown expr-core-coalesce statement %q", name)
		}
		return statement, nil
	})
}

func decodeExprCoreCoalesceBeanPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("expr-core-coalesce: unsupported event type %q", step.EventType)
	}
	var value exprCoreCoalesceBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}

func decodeExprCoreCoalesceNumericPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("expr-core-coalesce: unsupported event type %q", step.EventType)
	}
	var value exprCoreCoalesceNumericBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
