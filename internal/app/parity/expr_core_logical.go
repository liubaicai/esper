package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type exprCoreLogicalBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	BoolPrimitive bool   `esper:"boolPrimitive"`
	BoolBoxed     *bool  `esper:"boolBoxed"`
}

const exprCoreLogicalJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprCoreLogicalJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreAndOrNot.java",
}

var (
	exprCoreLogicalJavaRuntimeIDs = []string{
		"java-runtime-e48bf14356e3aeb838b5",
		"java-runtime-c63d599754acde7bb4dc",
		"java-runtime-b9d938f2dc52682b77c0",
	}
	exprCoreLogicalJavaExecutions = []string{
		"ExprCoreAndOrNotCombined",
		"ExprCoreNotWithVariable",
		"ExprCoreAndOrNotNull",
	}
)

const (
	exprCoreLogicalCombinedCase = "and-or-not-combined"
	exprCoreLogicalVariableCase = "not-with-variable"
	exprCoreLogicalNullCase     = "and-or-not-null"
	exprCoreLogicalVariableName = "thing"
)

func runExprCoreLogicalScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		exprCoreLogicalCombinedCase,
		exprCoreLogicalVariableCase,
		exprCoreLogicalNullCase,
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runExprCoreLogicalCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("expr-core-logical case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("expr-core-logical scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runExprCoreLogicalCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[exprCoreLogicalBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	var variableModule esper.Module
	input := esper.From[exprCoreLogicalBean](env, "SupportBean")
	var expressions []esper.Expression[bool]
	switch caseName {
	case exprCoreLogicalCombinedCase:
		intValue := esper.Field[exprCoreLogicalBean, int]("intPrimitive")
		expressions = []esper.Expression[bool]{
			esper.Or(esper.Equal[int](intValue, esper.Literal(1)), esper.Equal[int](intValue, esper.Literal(2))),
			esper.And(esper.Greater[int](intValue, esper.Literal(0)), esper.Less[int](intValue, esper.Literal(3))),
			esper.Not(esper.Equal[int](intValue, esper.Literal(2))),
		}
	case exprCoreLogicalVariableCase:
		module, err := env.RegisterModule("expr-core-logical", esper.ProtectedModule())
		if err != nil {
			return compat.Trace{}, err
		}
		variableModule = module
		if err := module.RegisterVariable(exprCoreLogicalVariableName, "Hello World"); err != nil {
			return compat.Trace{}, err
		}
		expressions = []esper.Expression[bool]{esper.Not(esper.Contains(
			esper.ModuleVariableRef[string](module, exprCoreLogicalVariableName),
			esper.Field[exprCoreLogicalBean, string]("theString"),
		))}
	case exprCoreLogicalNullCase:
		boxed := esper.Property[bool](esper.EventValue[exprCoreLogicalBean](), "BoolBoxed")
		primitive := esper.Field[exprCoreLogicalBean, bool]("boolPrimitive")
		expressions = []esper.Expression[bool]{
			esper.And(esper.NullLiteral[bool](), esper.NullLiteral[bool]()),
			esper.And(primitive, boxed),
			esper.And(boxed, primitive),
			esper.Or(primitive, boxed),
			esper.Or(boxed, primitive),
			esper.Not(boxed),
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported expr-core-logical case %q", caseName)
	}

	aliases := make([]esper.Selection, 0, len(expressions))
	for index, expression := range expressions {
		aliases = append(aliases, esper.Alias(fmt.Sprintf("c%d", index), expression))
	}
	query := esper.Select(input, aliases...).Query(esper.StatementName("s0"))
	var selectPlan esper.Plan
	if caseName == exprCoreLogicalVariableCase {
		selectPlan, err = variableModule.Build(query)
	} else {
		selectPlan, err = env.Build(query)
	}
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env)
	deployment, err := engine.Deploy(ctx, selectPlan)
	if err != nil {
		_ = engine.Close(context.Background())
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		_ = engine.Close(context.Background())
		return compat.Trace{}, fmt.Errorf("expected one expr-core-logical statement, got %d", len(statements))
	}
	statement := statements[0]
	defer func() { _ = engine.Close(context.Background()) }()

	handlers := map[string]compat.StepHandler{}
	if caseName == exprCoreLogicalVariableCase {
		handlers["set-variable"] = func(step compat.Step, _ func(*esper.Statement) error) ([]compat.TraceRecord, error) {
			var value string
			if err := json.Unmarshal(step.Payload, &value); err != nil {
				return nil, fmt.Errorf("decode logical variable value: %w", err)
			}
			return nil, engine.SetVariable(ctx, variableModule.QualifiedName(step.Name), value)
		}
	}

	return compat.ReplayWithStatementsAndHandlers(ctx, engine, statement, caseScenario, decodeExprCoreLogicalPayload,
		func(name string) (*esper.Statement, error) {
			if name == statement.Name() {
				return statement, nil
			}
			return nil, fmt.Errorf("unknown expr-core-logical statement %q", name)
		}, handlers)
}

func decodeExprCoreLogicalPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value exprCoreLogicalBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("expr-core-logical: unsupported event type %q", step.EventType)
	}
}
