package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type exprCoreInBetweenBean struct {
	TheString    *string  `json:"theString" esper:"theString"`
	IntPrimitive int      `json:"intPrimitive" esper:"intPrimitive"`
	DoubleBoxed  *float64 `json:"doubleBoxed" esper:"doubleBoxed"`
}

const exprCoreInBetweenJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprCoreInBetweenJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreInBetween.java",
}

var exprCoreInBetweenJavaRuntimeIDs = []string{
	"java-runtime-791d833152f1266fa139",
	"java-runtime-ffe28bce1e8e2a0dc4a0",
	"java-runtime-143c2fcf5bb1e4e6aa4a",
	"java-runtime-1145ffc38eea9ce1624e",
	"java-runtime-31934462edbc04c83973",
}

var exprCoreInBetweenJavaExecutions = []string{
	"ExprCoreInNumeric",
	"ExprCoreInStringExpr",
	"ExprCoreBetweenStringExpr",
	"ExprCoreBetweenNumericExpr",
	"ExprCoreInRange",
}

var exprCoreInBetweenCaseOrder = []string{
	"in-numeric",
	"in-string",
	"between-string",
	"between-numeric",
	"in-range",
}

var exprCoreInBetweenCaseSendCounts = []int{22, 36, 48, 48, 10}

// These vectors are part of the frozen Java/Go contract. Keep the JSON value
// spelling so scenario mutations cannot silently change numeric precision or
// turn a missing field into an unrelated default value.
var exprCoreInBetweenPayloadFields = []string{
	"doubleBoxed",
	"theString",
	"theString",
	"doubleBoxed",
}

var exprCoreInBetweenPayloadValues = [][]string{
	{
		"1", "null", "1.1", "1", "1.0999999999", "2", "4", "2",
		"2.3333333333333335", "null", "5", "5", "0", "null", "-1", "1",
		"null", "1.1", "1", "1.0999999999", "2", "4",
	},
	{
		`"0"`, `"a"`, `"b"`, `"c"`, `"d"`, "null", `"0"`, `"a"`, `"b"`,
		`"c"`, `"d"`, "null", `"0"`, `"b"`, `"a"`, `"c"`, `"d"`, "null",
		`"0"`, `"b"`, `"a"`, `"c"`, `"d"`, "null", `"0"`, "null", `"b"`,
		`"0"`, `"a"`, `"b"`, `"c"`, `"d"`, "null", `"0"`, "null", `"b"`,
	},
	{
		`"0"`, `"a1"`, `"a10"`, `"c"`, `"d"`, "null", `"a0"`, `"b9"`,
		`"b90"`, `"0"`, `"a1"`, `"a10"`, `"c"`, `"d"`, "null", `"a0"`,
		`"b9"`, `"b90"`, `"0"`, "null", `"a0"`, `"b9"`, `"0"`, "null",
		`"a0"`, `"b9"`, `"0"`, "null", `"a0"`, `"b9"`, `"0"`, `"a1"`,
		`"a10"`, `"c"`, `"d"`, "null", `"a0"`, `"b9"`, `"b90"`, `"0"`,
		`"a1"`, `"a10"`, `"c"`, `"d"`, "null", `"a0"`, `"b9"`, `"b90"`,
	},
	{
		"1", "null", "1.1", "2", "1.0999999999", "2", "4", "15", "15.00001",
		"1", "null", "1.1", "2", "1.0999999999", "2", "4", "15", "15.00001",
		"1", "null", "1.1", "1", "null", "1.1", "1", "null", "1.1", "1", "null", "1.1",
		"2", "1.0999999999", "2", "4", "15", "15.00001",
		"1", "null", "1.1", "2", "1.0999999999", "2", "4", "15", "15.00001",
		"1", "null", "1.1",
	},
}

var exprCoreInBetweenRangePayloadValues = []string{
	`"E1"`, `"E1"`, `"E1"`, `"E1"`, `"E1"`, `"E1"`, `"a"`, `"b"`, `"c"`, `"d"`,
}

var exprCoreInBetweenRangeIntValues = []string{"1", "2", "3", "4", "5", "3", "5", "5", "5", "5"}

func runExprCoreInBetweenScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateExprCoreInBetweenScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(exprCoreInBetweenCaseOrder))
	for _, caseName := range exprCoreInBetweenCaseOrder {
		trace, err := runExprCoreInBetweenCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("expr-core-in-between case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func validateExprCoreInBetweenScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != "expr-core-in-between" {
		return fmt.Errorf("expr-core-in-between scenario has id %q", scenario.ID)
	}
	stepIndex := 0
	for caseIndex, caseName := range exprCoreInBetweenCaseOrder {
		if stepIndex >= len(scenario.Steps) {
			return fmt.Errorf("expr-core-in-between scenario is missing case %q", caseName)
		}
		marker := scenario.Steps[stepIndex]
		if marker.Op != "case" || marker.Case != caseName {
			return fmt.Errorf("expr-core-in-between step %d has case %q; want %q", stepIndex, marker.Case, caseName)
		}
		stepIndex++
		for sendIndex, expectedValue := range exprCoreInBetweenExpectedValues(caseIndex) {
			if stepIndex >= len(scenario.Steps) {
				return fmt.Errorf("expr-core-in-between case %q is missing send %d", caseName, sendIndex)
			}
			step := scenario.Steps[stepIndex]
			if step.Op != "send" || step.EventType != "SupportBean" {
				return fmt.Errorf("expr-core-in-between step %d has op %q/event type %q; want SupportBean send for %q", stepIndex, step.Op, step.EventType, caseName)
			}
			if err := validateExprCoreInBetweenPayload(caseIndex, sendIndex, expectedValue, step.Payload); err != nil {
				return fmt.Errorf("expr-core-in-between case %q send %d: %w", caseName, sendIndex, err)
			}
			stepIndex++
		}
	}
	if stepIndex != len(scenario.Steps) {
		return fmt.Errorf("expr-core-in-between scenario has %d trailing steps", len(scenario.Steps)-stepIndex)
	}
	return nil
}

func exprCoreInBetweenExpectedValues(caseIndex int) []string {
	if caseIndex == len(exprCoreInBetweenCaseOrder)-1 {
		return exprCoreInBetweenRangePayloadValues
	}
	return exprCoreInBetweenPayloadValues[caseIndex]
}

func validateExprCoreInBetweenPayload(caseIndex, sendIndex int, expectedValue string, raw json.RawMessage) error {
	actual := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &actual); err != nil {
		return fmt.Errorf("decode payload object: %w", err)
	}
	expected := map[string]string{}
	if caseIndex == len(exprCoreInBetweenCaseOrder)-1 {
		expected["theString"] = expectedValue
		expected["intPrimitive"] = exprCoreInBetweenRangeIntValues[sendIndex]
	} else {
		expected[exprCoreInBetweenPayloadFields[caseIndex]] = expectedValue
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

func runExprCoreInBetweenCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	if caseName == "in-range" {
		return runExprCoreInRangeCase(ctx, scenario)
	}
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[exprCoreInBetweenBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	input := esper.From[exprCoreInBetweenBean](env, "SupportBean")
	var selections []esper.Selection
	switch caseName {
	case "in-numeric":
		value := esper.Field[exprCoreInBetweenBean, *float64]("doubleBoxed")
		arithmetic := esper.InOf(value,
			esper.Literal(1.1),
			esper.Divide[float64](esper.Literal(7.0), esper.Literal(3.5)),
			esper.Multiply[float64](esper.Literal(2.0), esper.Divide[float64](esper.Literal(6.0), esper.Literal(3.0))),
			esper.Literal(0.0),
		)
		selections = []esper.Selection{
			esper.Alias("c0", arithmetic),
			esper.Alias("c1", esper.InOf(value, esper.Divide[float64](esper.Literal(7.0), esper.Literal(3.0)), esper.NullLiteral[float64]())),
			esper.Alias("c2", esper.InOf(value, esper.Literal(5.0), esper.Literal(5.0), esper.Literal(5.0), esper.Literal(5.0), esper.Literal(5.0), esper.Literal(-1.0))),
			esper.Alias("c3", esper.NotInOf(value,
				esper.Literal(1.1),
				esper.Divide[float64](esper.Literal(7.0), esper.Literal(3.5)),
				esper.Multiply[float64](esper.Literal(2.0), esper.Divide[float64](esper.Literal(6.0), esper.Literal(3.0))),
				esper.Literal(0.0),
			)),
		}
	case "in-string":
		value := esper.Field[exprCoreInBetweenBean, *string]("theString")
		selections = []esper.Selection{
			esper.Alias("c0", esper.InOf(value, esper.Literal("a"), esper.Literal("b"), esper.Literal("c"))),
			esper.Alias("c1", esper.InOf(value, esper.Literal("a"))),
			esper.Alias("c2", esper.InOf(value, esper.Literal("a"), esper.Literal("b"))),
			esper.Alias("c3", esper.InOf(value, esper.Literal("a"), esper.NullLiteral[string]())),
			esper.Alias("c4", esper.InOf(value, esper.NullLiteral[string]())),
			esper.Alias("c5", esper.NotInOf(value, esper.Literal("a"), esper.Literal("b"), esper.Literal("c"))),
			esper.Alias("c6", esper.NotInOf(value, esper.NullLiteral[string]())),
		}
	case "between-string":
		value := esper.Field[exprCoreInBetweenBean, *string]("theString")
		a0, b9 := esper.Literal("a0"), esper.Literal("b9")
		selections = []esper.Selection{
			esper.Alias("c0", esper.BetweenOf(value, a0, b9)),
			esper.Alias("c1", esper.BetweenOf(value, b9, a0)),
			esper.Alias("c2", esper.BetweenOf(value, esper.NullLiteral[string](), b9)),
			esper.Alias("c3", esper.BetweenOf(value, esper.NullLiteral[string](), esper.NullLiteral[string]())),
			esper.Alias("c4", esper.BetweenOf(value, a0, esper.NullLiteral[string]())),
			esper.Alias("c5", esper.NotBetweenOf(value, a0, b9)),
			esper.Alias("c6", esper.NotBetweenOf(value, b9, a0)),
		}
	case "between-numeric":
		value := esper.Field[exprCoreInBetweenBean, *float64]("doubleBoxed")
		onePointOne, fifteen := esper.Literal(1.1), esper.Literal(15.0)
		selections = []esper.Selection{
			esper.Alias("c0", esper.BetweenOf(value, onePointOne, fifteen)),
			esper.Alias("c1", esper.BetweenOf(value, fifteen, onePointOne)),
			esper.Alias("c2", esper.BetweenOf(value, esper.NullLiteral[float64](), fifteen)),
			esper.Alias("c3", esper.BetweenOf(value, fifteen, esper.NullLiteral[float64]())),
			esper.Alias("c4", esper.BetweenOf(value, esper.NullLiteral[float64](), esper.NullLiteral[float64]())),
			esper.Alias("c5", esper.NotBetweenOf(value, onePointOne, fifteen)),
			esper.Alias("c6", esper.NotBetweenOf(value, fifteen, onePointOne)),
			esper.Alias("c7", esper.NotBetweenOf(value, fifteen, esper.NullLiteral[float64]())),
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported expr-core-in-between case %q", caseName)
	}
	plan, err := env.Build(esper.Select(input, selections...).Query(esper.StatementName("s0")))
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeExprCoreInBetweenPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown expr-core-in-between statement %q", name)
		}
		return statement, nil
	})
}

func runExprCoreInRangeCase(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, "in-range")
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[exprCoreInBetweenBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	input := esper.From[exprCoreInBetweenBean](env, "SupportBean")
	value := esper.Field[exprCoreInBetweenBean, int]("intPrimitive")
	text := esper.Field[exprCoreInBetweenBean, *string]("theString")
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	phases := []struct {
		name       string
		start, end int
		build      func() (esper.Plan, error)
	}{
		{
			name: "s0", start: 0, end: 5,
			build: func() (esper.Plan, error) {
				return env.Build(esper.Select(input,
					esper.Alias("ro", esper.BetweenRangeOf(value, esper.Literal(2), esper.Literal(4), false, false)),
					esper.Alias("rc", esper.BetweenRangeOf(value, esper.Literal(2), esper.Literal(4), true, true)),
					esper.Alias("rho", esper.BetweenRangeOf(value, esper.Literal(2), esper.Literal(4), true, false)),
					esper.Alias("rhc", esper.BetweenRangeOf(value, esper.Literal(2), esper.Literal(4), false, true)),
					esper.Alias("nro", esper.NotBetweenRangeOf(value, esper.Literal(2), esper.Literal(4), false, false)),
					esper.Alias("nrc", esper.NotBetweenRangeOf(value, esper.Literal(2), esper.Literal(4), true, true)),
					esper.Alias("nrho", esper.NotBetweenRangeOf(value, esper.Literal(2), esper.Literal(4), true, false)),
					esper.Alias("nrhc", esper.NotBetweenRangeOf(value, esper.Literal(2), esper.Literal(4), false, true)),
				).Query(esper.StatementName("s0")))
			},
		},
		{
			name: "s1", start: 5, end: 6,
			build: func() (esper.Plan, error) {
				return env.Build(esper.Select(input,
					esper.Alias("r1", esper.BetweenOf(value, esper.Literal(4), esper.Literal(2))),
					esper.Alias("r2", esper.BetweenRangeOf(value, esper.Literal(4), esper.Literal(2), true, true)),
				).Query(esper.StatementName("s1")))
			},
		},
		{
			name: "s2", start: 6, end: 10,
			build: func() (esper.Plan, error) {
				return env.Build(esper.Select(input,
					esper.Alias("ro", esper.BetweenRangeOf(text, esper.Literal("a"), esper.Literal("d"), false, false)),
				).Query(esper.StatementName("s2")))
			},
		},
	}

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, phase := range phases {
		plan, err := phase.build()
		if err != nil {
			return compat.Trace{}, err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return compat.Trace{}, err
		}
		statements := deployment.Statements()
		if len(statements) != 1 {
			return compat.Trace{}, fmt.Errorf("expected one expr-core-in-between range statement for %s, got %d", phase.name, len(statements))
		}
		statement := statements[0]
		phaseScenario, err := exprCoreInBetweenRangePhase(caseScenario, phase.start, phase.end)
		if err != nil {
			return compat.Trace{}, err
		}
		phaseTrace, replayErr := compat.ReplayWithStatements(ctx, engine, statement, phaseScenario, decodeExprCoreInBetweenPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown expr-core-in-between range statement %q", name)
			}
			return statement, nil
		})
		undeployErr := deployment.Undeploy(ctx)
		if replayErr != nil {
			return compat.Trace{}, replayErr
		}
		if undeployErr != nil {
			return compat.Trace{}, undeployErr
		}
		trace.Records = append(trace.Records, phaseTrace.Records...)
	}
	return trace, nil
}

func exprCoreInBetweenRangePhase(caseScenario compat.Scenario, start, end int) (compat.Scenario, error) {
	phase := compat.Scenario{Version: caseScenario.Version, ID: caseScenario.ID}
	phase.Steps = append(phase.Steps, compat.Step{Op: "case", Case: "in-range"})
	sendIndex := 0
	for _, step := range caseScenario.Steps {
		if step.Op != "send" {
			continue
		}
		if sendIndex >= start && sendIndex < end {
			phase.Steps = append(phase.Steps, step)
		}
		sendIndex++
	}
	if sendIndex != exprCoreInBetweenCaseSendCounts[len(exprCoreInBetweenCaseSendCounts)-1] {
		return compat.Scenario{}, fmt.Errorf("expr-core-in-between in-range has %d sends; want %d", sendIndex, exprCoreInBetweenCaseSendCounts[len(exprCoreInBetweenCaseSendCounts)-1])
	}
	if err := phase.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return phase, nil
}

func decodeExprCoreInBetweenPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("expr-core-in-between: unsupported event type %q", step.EventType)
	}
	var value exprCoreInBetweenBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("expr-core-in-between: decode SupportBean: %w", err)
	}
	return value, nil
}
