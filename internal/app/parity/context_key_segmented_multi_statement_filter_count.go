package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextKeySegmentedMultiStatementFilterCountBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type contextKeySegmentedMultiStatementFilterCountS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

const contextKeySegmentedMultiStatementFilterCountJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedMultiStatementFilterCountJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedMultiStatementFilterCountJavaRuntimeIDs = []string{
	"java-runtime-7c4cef4f2c1d8bf8f07a", // ContextKeySegmentedMultiStatementFilterCount
}

var contextKeySegmentedMultiStatementFilterCountJavaExecutions = []string{
	"ContextKeySegmentedMultiStatementFilterCount",
}

// runContextKeySegmentedMultiStatementFilterCountScenario replays the
// multi-statement execution of ContextKeySegmented: a multi-stream
// segmented context partitioning SupportBean by theString and
// SupportBean_S0 by p00, shared by two statements (s0 sums SupportBean_S0
// ids per p00 partition, s1 sums SupportBean intPrimitive per theString
// partition). Each statement sees only its own stream's events and keeps
// per-partition aggregate state; the s0 and s1 partition spaces are
// independent even though the context is shared.
func runContextKeySegmentedMultiStatementFilterCountScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedMultiStatementFilterCountCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-multi-statement-filter-count case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedMultiStatementFilterCountCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedMultiStatementFilterCountBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextKeySegmentedMultiStatementFilterCountS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	theString := esper.Field[contextKeySegmentedMultiStatementFilterCountBean, string]("theString")
	p00 := esper.Field[contextKeySegmentedMultiStatementFilterCountS0, string]("p00")
	if _, err := esper.CreateKeyContextByStreams(env, "SegmentedByAString",
		esper.KeyContextStream{Type: "SupportBean", Keys: []esper.Expr{theString}},
		esper.KeyContextStream{Type: "SupportBean_S0", Keys: []esper.Expr{p00}},
	); err != nil {
		return compat.Trace{}, err
	}
	queryS0 := esper.From[contextKeySegmentedMultiStatementFilterCountS0](env, "SupportBean_S0").Aggregate(
		esper.Alias("col1", esper.Sum[int](esper.Field[contextKeySegmentedMultiStatementFilterCountS0, int]("id"))),
	).Query(esper.StatementName("s0"), esper.WithContext("SegmentedByAString"))
	planS0, err := env.Build(queryS0)
	if err != nil {
		return compat.Trace{}, err
	}
	querySB := esper.From[contextKeySegmentedMultiStatementFilterCountBean](env, "SupportBean").Aggregate(
		esper.Alias("col1", esper.Sum[int](esper.Field[contextKeySegmentedMultiStatementFilterCountBean, int]("intPrimitive"))),
	).Query(esper.StatementName("s1"), esper.WithContext("SegmentedByAString"))
	planSB, err := env.Build(querySB)
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env)
	deploymentS0, err := engine.Deploy(ctx, planS0)
	if err != nil {
		_ = engine.Close(context.Background())
		return compat.Trace{}, err
	}
	deploymentSB, err := engine.Deploy(ctx, planSB)
	if err != nil {
		_ = engine.Close(context.Background())
		return compat.Trace{}, err
	}
	if len(deploymentS0.Statements()) != 1 || len(deploymentSB.Statements()) != 1 {
		_ = engine.Close(context.Background())
		return compat.Trace{}, fmt.Errorf("expected one statement per deployment, got %d and %d", len(deploymentS0.Statements()), len(deploymentSB.Statements()))
	}
	statementS0 := deploymentS0.Statements()[0]
	statementSB := deploymentSB.Statements()[0]
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statementS0, caseScenario, decodeContextKeySegmentedMultiStatementFilterCountPayload, func(name string) (*esper.Statement, error) {
		switch name {
		case "s0":
			return statementS0, nil
		case "s1":
			return statementSB, nil
		default:
			return nil, fmt.Errorf("unknown context-key-segmented-multi-statement-filter-count statement %q", name)
		}
	}, statementSB)
}

func decodeContextKeySegmentedMultiStatementFilterCountPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextKeySegmentedMultiStatementFilterCountBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextKeySegmentedMultiStatementFilterCountS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unknown context-key-segmented-multi-statement-filter-count event type %q", step.EventType)
	}
}
