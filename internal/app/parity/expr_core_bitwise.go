package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type exprCoreBitwiseBean struct {
	BytePrimitive  int8   `esper:"bytePrimitive"`
	ByteBoxed      *int8  `esper:"byteBoxed"`
	ShortPrimitive int16  `esper:"shortPrimitive"`
	ShortBoxed     *int16 `esper:"shortBoxed"`
	IntPrimitive   int32  `esper:"intPrimitive"`
	IntBoxed       *int32 `esper:"intBoxed"`
	LongPrimitive  int64  `esper:"longPrimitive"`
	LongBoxed      *int64 `esper:"longBoxed"`
	BoolPrimitive  bool   `esper:"boolPrimitive"`
	BoolBoxed      *bool  `esper:"boolBoxed"`
}

const exprCoreBitwiseJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprCoreBitwiseJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreBitWiseOperators.java",
}

var (
	exprCoreBitwiseJavaRuntimeIDs = []string{
		"java-runtime-b0d354033a204970cb26",
		"java-runtime-7819faeecbb3e817d49d",
	}
	exprCoreBitwiseJavaExecutions = []string{
		"ExprCoreBitWiseOp",
		"ExprCoreBitWiseOpOM",
	}
)

func runExprCoreBitwiseScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"bitwise-op", "bitwise-op-om"}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runExprCoreBitwiseCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("expr-core-bitwise case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("expr-core-bitwise scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runExprCoreBitwiseCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[exprCoreBitwiseBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	input := esper.From[exprCoreBitwiseBean](env, "SupportBean")
	query := esper.Select(input,
		esper.Alias("myFirstProperty", esper.BitwiseAndOf[int8](
			esper.Field[exprCoreBitwiseBean, int8]("bytePrimitive"),
			esper.Field[exprCoreBitwiseBean, *int8]("byteBoxed"),
		)),
		esper.Alias("mySecondProperty", esper.BitwiseOrOf[int16](
			esper.Field[exprCoreBitwiseBean, int16]("shortPrimitive"),
			esper.Field[exprCoreBitwiseBean, *int16]("shortBoxed"),
		)),
		esper.Alias("myThirdProperty", esper.BitwiseOrOf[int32](
			esper.Field[exprCoreBitwiseBean, int32]("intPrimitive"),
			esper.Field[exprCoreBitwiseBean, *int32]("intBoxed"),
		)),
		esper.Alias("myFourthProperty", esper.BitwiseXorOf[int64](
			esper.Field[exprCoreBitwiseBean, int64]("longPrimitive"),
			esper.Field[exprCoreBitwiseBean, *int64]("longBoxed"),
		)),
		esper.Alias("myFifthProperty", esper.BitwiseAndOf[bool](
			esper.Field[exprCoreBitwiseBean, bool]("boolPrimitive"),
			esper.Field[exprCoreBitwiseBean, *bool]("boolBoxed"),
		)),
	).Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeExprCoreBitwisePayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown expr-core-bitwise statement %q", name)
		}
		return statement, nil
	})
}

func decodeExprCoreBitwisePayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("expr-core-bitwise: unsupported event type %q", step.EventType)
	}
	var value exprCoreBitwiseBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
