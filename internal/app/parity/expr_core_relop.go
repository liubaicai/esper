package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type exprCoreRelOpBean struct {
	TheString       string   `json:"theString" esper:"theString"`
	IntPrimitive    int      `json:"intPrimitive" esper:"intPrimitive"`
	IntBoxed        *int     `json:"intBoxed" esper:"intBoxed"`
	LongBoxed       *int64   `json:"longBoxed" esper:"longBoxed"`
	FloatPrimitive  float32  `json:"floatPrimitive" esper:"floatPrimitive"`
	DoublePrimitive float64  `json:"doublePrimitive" esper:"doublePrimitive"`
	DoubleBoxed     *float64 `json:"doubleBoxed" esper:"doubleBoxed"`
	BigDecimal      *big.Rat `json:"bigDecimal" esper:"bigDecimal"`
	BigInteger      *big.Int `json:"bigInteger" esper:"bigInteger"`
}

const exprCoreRelOpJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprCoreRelOpJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreRelOp.java",
}

var exprCoreRelOpJavaRuntimeIDs = []string{
	"java-runtime-615cb125ab25e488f40c",
	"java-runtime-402d95bd69d700735100",
}

var exprCoreRelOpJavaExecutions = []string{
	"ExprCoreRelOpTypes",
	"ExprCoreRelOpNull",
}

var exprCoreRelOpCaseOrder = []string{
	"relop-string",
	"relop-int",
	"relop-long",
	"relop-float",
	"relop-double",
	"relop-big-decimal",
	"relop-int-big-decimal",
	"relop-big-integer",
	"relop-int-big-integer",
	"relop-null",
}

func runExprCoreRelOpScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateExprCoreRelOpScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(exprCoreRelOpCaseOrder))
	for _, caseName := range exprCoreRelOpCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runExprCoreRelOpCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("expr-core-relop case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("expr-core-relop scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func validateExprCoreRelOpScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	activeCase := -1
	caseCounts := make([]int, len(exprCoreRelOpCaseOrder))
	for index, step := range scenario.Steps {
		switch step.Op {
		case "case":
			activeCase++
			if activeCase >= len(exprCoreRelOpCaseOrder) || step.Case != exprCoreRelOpCaseOrder[activeCase] {
				return fmt.Errorf("expr-core-relop step %d has case %q; want case %q at position %d", index, step.Case, exprCoreRelOpCaseOrder[minInt(activeCase, len(exprCoreRelOpCaseOrder)-1)], activeCase)
			}
		case "send":
			if activeCase < 0 {
				return fmt.Errorf("expr-core-relop step %d sends before the first case", index)
			}
			if step.EventType != "SupportBean" {
				return fmt.Errorf("expr-core-relop step %d has unsupported event type %q", index, step.EventType)
			}
			caseCounts[activeCase]++
		default:
			return fmt.Errorf("expr-core-relop step %d has unsupported operation %q", index, step.Op)
		}
	}
	if activeCase != len(exprCoreRelOpCaseOrder)-1 {
		return fmt.Errorf("expr-core-relop scenario has %d cases; want %d", activeCase+1, len(exprCoreRelOpCaseOrder))
	}
	for index, count := range caseCounts {
		if count != 3 {
			return fmt.Errorf("expr-core-relop case %q has %d sends; want 3", exprCoreRelOpCaseOrder[index], count)
		}
	}
	return nil
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func runExprCoreRelOpCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[exprCoreRelOpBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	input := esper.From[exprCoreRelOpBean](env, "SupportBean")
	var selections []esper.Selection
	switch caseName {
	case "relop-string":
		selections = exprCoreRelOpSelections(
			esper.Field[exprCoreRelOpBean, string]("theString"),
			esper.Literal("B"),
		)
	case "relop-int":
		selections = exprCoreRelOpSelections(
			esper.Field[exprCoreRelOpBean, int]("intPrimitive"),
			esper.Literal(2),
		)
	case "relop-long":
		selections = exprCoreRelOpSelections(
			esper.Field[exprCoreRelOpBean, *int64]("longBoxed"),
			esper.Literal(int64(2)),
		)
	case "relop-float":
		selections = exprCoreRelOpSelections(
			esper.Field[exprCoreRelOpBean, float32]("floatPrimitive"),
			esper.Literal(float32(2)),
		)
	case "relop-double":
		selections = exprCoreRelOpSelections(
			esper.Field[exprCoreRelOpBean, float64]("doublePrimitive"),
			esper.Literal(float64(2)),
		)
	case "relop-big-decimal":
		var two big.Rat
		two.SetInt64(2)
		selections = exprCoreRelOpSelections(
			esper.Field[exprCoreRelOpBean, *big.Rat]("bigDecimal"),
			esper.Literal(two),
		)
	case "relop-int-big-decimal":
		var two big.Rat
		two.SetInt64(2)
		selections = exprCoreRelOpSelections(
			esper.Field[exprCoreRelOpBean, int]("intPrimitive"),
			esper.Literal(two),
		)
	case "relop-big-integer":
		var two big.Int
		two.SetInt64(2)
		selections = exprCoreRelOpSelections(
			esper.Field[exprCoreRelOpBean, *big.Int]("bigInteger"),
			esper.Literal(two),
		)
	case "relop-int-big-integer":
		var two big.Int
		two.SetInt64(2)
		selections = exprCoreRelOpSelections(
			esper.Field[exprCoreRelOpBean, int]("intPrimitive"),
			esper.Literal(two),
		)
	case "relop-null":
		intPrimitive := esper.Field[exprCoreRelOpBean, int]("intPrimitive")
		intBoxed := esper.Field[exprCoreRelOpBean, *int]("intBoxed")
		doubleBoxed := esper.Field[exprCoreRelOpBean, *float64]("doubleBoxed")
		nullInt := esper.NullLiteral[int]()
		selections = []esper.Selection{
			esper.Alias("c0", esper.GreaterOf(intPrimitive, nullInt)),
			esper.Alias("c1", esper.GreaterOf(intBoxed, esper.Literal(0))),
			esper.Alias("c2", esper.GreaterOf(nullInt, intPrimitive)),
			esper.Alias("c3", esper.GreaterOf(nullInt, intBoxed)),
			esper.Alias("c4", esper.GreaterOf(nullInt, nullInt)),
			esper.Alias("c5", esper.GreaterOf(intPrimitive, intBoxed)),
			esper.Alias("c6", esper.GreaterOf(intBoxed, intPrimitive)),
			esper.Alias("c7", esper.GreaterOf(doubleBoxed, intBoxed)),
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported expr-core-relop case %q", caseName)
	}
	query := esper.Select(input, selections...).Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeExprCoreRelOpPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown expr-core-relop statement %q", name)
		}
		return statement, nil
	})
}

func exprCoreRelOpSelections(left, right esper.Expr) []esper.Selection {
	return []esper.Selection{
		esper.Alias("c0", esper.GreaterOrEqualOf(left, right)),
		esper.Alias("c1", esper.GreaterOf(left, right)),
		esper.Alias("c2", esper.LessOrEqualOf(left, right)),
		esper.Alias("c3", esper.LessOf(left, right)),
	}
}

func decodeExprCoreRelOpPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("expr-core-relop: unsupported event type %q", step.EventType)
	}
	var raw struct {
		TheString       string          `json:"theString"`
		IntPrimitive    int             `json:"intPrimitive"`
		IntBoxed        *int            `json:"intBoxed"`
		LongBoxed       *int64          `json:"longBoxed"`
		FloatPrimitive  float32         `json:"floatPrimitive"`
		DoublePrimitive float64         `json:"doublePrimitive"`
		DoubleBoxed     *float64        `json:"doubleBoxed"`
		BigDecimal      json.RawMessage `json:"bigDecimal"`
		BigInteger      json.RawMessage `json:"bigInteger"`
	}
	if err := json.Unmarshal(step.Payload, &raw); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	bigDecimal, err := parseExprCoreRelOpRat(raw.BigDecimal)
	if err != nil {
		return nil, fmt.Errorf("decode bigDecimal: %w", err)
	}
	bigInteger, err := parseExprCoreRelOpInt(raw.BigInteger)
	if err != nil {
		return nil, fmt.Errorf("decode bigInteger: %w", err)
	}
	return exprCoreRelOpBean{
		TheString:       raw.TheString,
		IntPrimitive:    raw.IntPrimitive,
		IntBoxed:        raw.IntBoxed,
		LongBoxed:       raw.LongBoxed,
		FloatPrimitive:  raw.FloatPrimitive,
		DoublePrimitive: raw.DoublePrimitive,
		DoubleBoxed:     raw.DoubleBoxed,
		BigDecimal:      bigDecimal,
		BigInteger:      bigInteger,
	}, nil
}

func parseExprCoreRelOpText(raw json.RawMessage) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return "", err
	}
	return number.String(), nil
}

func parseExprCoreRelOpRat(raw json.RawMessage) (*big.Rat, error) {
	text, err := parseExprCoreRelOpText(raw)
	if err != nil || text == "" {
		return nil, err
	}
	value := new(big.Rat)
	if _, ok := value.SetString(text); !ok {
		return nil, fmt.Errorf("invalid exact decimal %q", text)
	}
	return value, nil
}

func parseExprCoreRelOpInt(raw json.RawMessage) (*big.Int, error) {
	text, err := parseExprCoreRelOpText(raw)
	if err != nil || text == "" {
		return nil, err
	}
	value := new(big.Int)
	if _, ok := value.SetString(text, 10); !ok {
		return nil, fmt.Errorf("invalid exact integer %q", text)
	}
	return value, nil
}
