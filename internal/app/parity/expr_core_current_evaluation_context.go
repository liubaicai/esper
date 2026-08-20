package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type exprCoreCurrentEvaluationContextBean struct {
	TheString string `esper:"theString"`
}

const exprCoreCurrentEvaluationContextJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprCoreCurrentEvaluationContextJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreCurrentEvaluationContext.java",
}

var (
	exprCoreCurrentEvaluationContextJavaRuntimeIDs = []string{
		"java-runtime-2efbbb4fce55aa6ce513",
		"java-runtime-ca0799a6f2d163a49f7f",
	}
	exprCoreCurrentEvaluationContextJavaExecutions = []string{
		"ExprCoreCurrentEvalCtx{soda=false}",
		"ExprCoreCurrentEvalCtx{soda=true}",
	}
)

const (
	exprCoreCurrentEvaluationContextEPLCase  = "current-evaluation-context-epl"
	exprCoreCurrentEvaluationContextSODACase = "current-evaluation-context-soda"
)

func runExprCoreCurrentEvaluationContextScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		exprCoreCurrentEvaluationContextEPLCase,
		exprCoreCurrentEvaluationContextSODACase,
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runExprCoreCurrentEvaluationContextCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("expr-core-current-evaluation-context case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("expr-core-current-evaluation-context scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runExprCoreCurrentEvaluationContextCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[exprCoreCurrentEvaluationContextBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	current := esper.CurrentEvaluationContext()
	query := esper.Select(
		esper.From[exprCoreCurrentEvaluationContextBean](env, "SupportBean"),
		esper.Alias("c0", current),
		esper.Alias("current_evaluation_context()", current),
		esper.Alias("c2", esper.Property[string](current, "RuntimeURI")),
	).Query(
		esper.StatementName("s0"),
		esper.WithStatementUserObject("my_user_object"),
	)
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(
		env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(exprCoreCurrentEvaluationContextRuntimeURI(caseName)),
	)
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		_ = engine.Close(context.Background())
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		_ = engine.Close(context.Background())
		return compat.Trace{}, fmt.Errorf("expected one current-evaluation-context parity statement, got %d", len(statements))
	}
	statement := statements[0]
	defer func() { _ = engine.Close(context.Background()) }()

	trace, err := compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeExprCoreCurrentEvaluationContextPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown expr-core-current-evaluation-context statement %q", name)
		}
		return statement, nil
	})
	if err != nil {
		return compat.Trace{}, err
	}
	return normalizeExprCoreCurrentEvaluationContextTrace(trace), nil
}

func exprCoreCurrentEvaluationContextRuntimeURI(caseName string) string {
	return "parity-expr-core-current-evaluation-context-" + caseName
}

func normalizeExprCoreCurrentEvaluationContextTrace(trace compat.Trace) compat.Trace {
	for recordIndex := range trace.Records {
		trace.Records[recordIndex].New = normalizeExprCoreCurrentEvaluationContextResults(trace.Records[recordIndex].New)
		trace.Records[recordIndex].Old = normalizeExprCoreCurrentEvaluationContextResults(trace.Records[recordIndex].Old)
	}
	return trace
}

func normalizeExprCoreCurrentEvaluationContextResults(results []compat.ResultRecord) []compat.ResultRecord {
	for resultIndex := range results {
		for name, value := range results[resultIndex].Fields {
			metadata, ok := value.(esper.ExpressionEvaluationContext)
			if !ok {
				continue
			}
			results[resultIndex].Fields[name] = map[string]any{
				"runtimeURI":          metadata.RuntimeURI,
				"statementName":       metadata.StatementName,
				"contextPartitionID":  metadata.ContextPartitionID,
				"statementUserObject": metadata.StatementUserObject,
			}
		}
	}
	return results
}

func decodeExprCoreCurrentEvaluationContextPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("expr-core-current-evaluation-context: unsupported event type %q", step.EventType)
	}
	var value exprCoreCurrentEvaluationContextBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
