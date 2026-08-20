package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type exprCoreLikeRegexpBean struct {
	TheString    *string `json:"theString" esper:"theString"`
	IntPrimitive int     `json:"intPrimitive" esper:"intPrimitive"`
}

type exprCoreLikeRegexpS0 struct {
	ID  int     `json:"id" esper:"id"`
	P00 *string `json:"p00" esper:"p00"`
	P01 *string `json:"p01" esper:"p01"`
	P02 *string `json:"p02" esper:"p02"`
	P03 *string `json:"p03" esper:"p03"`
}

const exprCoreLikeRegexpJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprCoreLikeRegexpJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreLikeRegexp.java",
}

var exprCoreLikeRegexpJavaRuntimeIDs = []string{
	"java-runtime-376f347aa8fc8367fcbc",
	"java-runtime-09f15b6eb39cbee59b0d",
	"java-runtime-7dc8b98263cf5e9e4c3f",
	"java-runtime-8a8794063a0b9c4bfd0e",
}

var exprCoreLikeRegexpJavaExecutions = []string{
	"ExprCoreLikeWConstants",
	"ExprCoreLikeWExprs",
	"ExprCoreRegexpWConstants",
	"ExprCoreRegexpWExprs",
}

var exprCoreLikeRegexpCaseOrder = []string{
	"like-constants",
	"like-expressions",
	"regexp-constants",
	"regexp-expressions",
}

var exprCoreLikeRegexpCaseSendCounts = []int{4, 5, 4, 6}

func runExprCoreLikeRegexpScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateExprCoreLikeRegexpScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(exprCoreLikeRegexpCaseOrder))
	for _, caseName := range exprCoreLikeRegexpCaseOrder {
		trace, err := runExprCoreLikeRegexpCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("expr-core-like-regexp case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func validateExprCoreLikeRegexpScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != "expr-core-like-regexp" {
		return fmt.Errorf("expr-core-like-regexp scenario has id %q", scenario.ID)
	}
	activeCase := -1
	caseCounts := make([]int, len(exprCoreLikeRegexpCaseOrder))
	for index, step := range scenario.Steps {
		switch step.Op {
		case "case":
			activeCase++
			if activeCase >= len(exprCoreLikeRegexpCaseOrder) || step.Case != exprCoreLikeRegexpCaseOrder[activeCase] {
				return fmt.Errorf("expr-core-like-regexp step %d has case %q at position %d", index, step.Case, activeCase)
			}
		case "send":
			if activeCase < 0 {
				return fmt.Errorf("expr-core-like-regexp step %d sends before the first case", index)
			}
			wantType := "SupportBean"
			if activeCase == 1 || activeCase == 3 {
				wantType = "SupportBean_S0"
			}
			if step.EventType != wantType {
				return fmt.Errorf("expr-core-like-regexp step %d has event type %q; want %q", index, step.EventType, wantType)
			}
			caseCounts[activeCase]++
		default:
			return fmt.Errorf("expr-core-like-regexp step %d has unsupported operation %q", index, step.Op)
		}
	}
	if activeCase != len(exprCoreLikeRegexpCaseOrder)-1 {
		return fmt.Errorf("expr-core-like-regexp scenario has %d cases; want %d", activeCase+1, len(exprCoreLikeRegexpCaseOrder))
	}
	for index, count := range caseCounts {
		if count != exprCoreLikeRegexpCaseSendCounts[index] {
			return fmt.Errorf("expr-core-like-regexp case %q has %d sends; want %d", exprCoreLikeRegexpCaseOrder[index], count, exprCoreLikeRegexpCaseSendCounts[index])
		}
	}
	return nil
}

func runExprCoreLikeRegexpCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	var query esper.Query
	var decode compat.DecodePayload
	switch caseName {
	case "like-constants":
		if _, err := esper.RegisterStruct[exprCoreLikeRegexpBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[exprCoreLikeRegexpBean](env, "SupportBean")
		query = esper.Select(input,
			esper.Alias("c0", esper.LikeOf(esper.Field[exprCoreLikeRegexpBean, *string]("theString"), esper.Literal("A%"))),
			esper.Alias("c1", esper.LikeOf(esper.Field[exprCoreLikeRegexpBean, int]("intPrimitive"), esper.Literal("1%"))),
		).Query(esper.StatementName("s0"))
		decode = decodeExprCoreLikeRegexpPayload
	case "like-expressions":
		if _, err := esper.RegisterStruct[exprCoreLikeRegexpS0](env, "SupportBean_S0"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[exprCoreLikeRegexpS0](env, "SupportBean_S0")
		query = esper.Select(input,
			esper.Alias("c0", esper.LikeOf(
				esper.Field[exprCoreLikeRegexpS0, *string]("p00"),
				esper.Field[exprCoreLikeRegexpS0, *string]("p01"),
			)),
			esper.Alias("c1", esper.LikeOf(
				esper.Field[exprCoreLikeRegexpS0, int]("id"),
				esper.Field[exprCoreLikeRegexpS0, *string]("p02"),
			)),
		).Query(esper.StatementName("s0"))
		decode = decodeExprCoreLikeRegexpPayload
	case "regexp-constants":
		if _, err := esper.RegisterStruct[exprCoreLikeRegexpBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[exprCoreLikeRegexpBean](env, "SupportBean")
		query = esper.Select(input,
			esper.Alias("c0", esper.RegexpMatchOf(esper.Field[exprCoreLikeRegexpBean, *string]("theString"), esper.Literal(".*Jack.*"))),
			esper.Alias("c1", esper.RegexpMatchOf(esper.Field[exprCoreLikeRegexpBean, int]("intPrimitive"), esper.Literal(".*1.*"))),
		).Query(esper.StatementName("s0"))
		decode = decodeExprCoreLikeRegexpPayload
	case "regexp-expressions":
		if _, err := esper.RegisterStruct[exprCoreLikeRegexpS0](env, "SupportBean_S0"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[exprCoreLikeRegexpS0](env, "SupportBean_S0")
		query = esper.Select(input,
			esper.Alias("c0", esper.RegexpMatchOf(
				esper.Field[exprCoreLikeRegexpS0, *string]("p00"),
				esper.Field[exprCoreLikeRegexpS0, *string]("p01"),
			)),
			esper.Alias("c1", esper.RegexpMatchOf(
				esper.Field[exprCoreLikeRegexpS0, int]("id"),
				esper.Field[exprCoreLikeRegexpS0, *string]("p02"),
			)),
		).Query(esper.StatementName("s0"))
		decode = decodeExprCoreLikeRegexpPayload
	default:
		return compat.Trace{}, fmt.Errorf("unsupported expr-core-like-regexp case %q", caseName)
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
			return nil, fmt.Errorf("unknown expr-core-like-regexp statement %q", name)
		}
		return statement, nil
	})
}

func decodeExprCoreLikeRegexpPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value exprCoreLikeRegexpBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("expr-core-like-regexp: decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value exprCoreLikeRegexpS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("expr-core-like-regexp: decode SupportBean_S0: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("expr-core-like-regexp: unsupported event type %q", step.EventType)
	}
}
