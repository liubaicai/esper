package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetAggregateFirstLastWindowIndexedBean struct {
	IntPrimitive int `esper:"intPrimitive"`
}

const resultsetAggregateFirstLastWindowIndexedJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateFirstLastWindowIndexedJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFirstLastWindow.java",
}

var resultsetAggregateFirstLastWindowIndexedJavaRuntimeIDs = []string{
	"java-runtime-30f62dc2e86a7a1e80a0",
}

var resultsetAggregateFirstLastWindowIndexedJavaExecutions = []string{
	"ResultSetAggregateFirstLastIndexed",
}

const resultsetAggregateFirstLastWindowIndexedCase = "indexed"

func runResultSetAggregateFirstLastWindowIndexedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if !scenarioHasCase(scenario, resultsetAggregateFirstLastWindowIndexedCase) {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-firstlastwindow-indexed scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, resultsetAggregateFirstLastWindowIndexedCase)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateFirstLastWindowIndexedBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	value := esper.Field[resultsetAggregateFirstLastWindowIndexedBean, int]("intPrimitive")
	query := esper.From[resultsetAggregateFirstLastWindowIndexedBean](env, "SupportBean").
		Window(esper.LengthWindow(3)).
		Aggregate(
			esper.Alias("f0", esper.First[int](value, 0)),
			esper.Alias("f1", esper.First[int](value, 1)),
			esper.Alias("f2", esper.First[int](value, 2)),
			esper.Alias("f3", esper.First[int](value, 3)),
			esper.Alias("l0", esper.Last[int](value, 0)),
			esper.Alias("l1", esper.Last[int](value, 1)),
			esper.Alias("l2", esper.Last[int](value, 2)),
			esper.Alias("l3", esper.Last[int](value, 3)),
		).
		Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateFirstLastWindowIndexedJavaRuntimeIDs[0]),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one resultset aggregate statement, got %d", len(statements))
	}
	statement := statements[0]
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateFirstLastWindowIndexedPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-firstlastwindow-indexed statement %q", name)
		}
		return statement, nil
	})
}

func decodeResultSetAggregateFirstLastWindowIndexedPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("unsupported resultset-aggregate-firstlastwindow-indexed event type %q", step.EventType)
	}
	var value resultsetAggregateFirstLastWindowIndexedBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
