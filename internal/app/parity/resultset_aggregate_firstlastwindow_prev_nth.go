package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetAggregateFirstLastWindowPrevNthBean struct {
	IntPrimitive int `esper:"intPrimitive"`
}

const resultsetAggregateFirstLastWindowPrevNthJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateFirstLastWindowPrevNthJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFirstLastWindow.java",
}

var resultsetAggregateFirstLastWindowPrevNthJavaRuntimeIDs = []string{
	"java-runtime-3733f40a5c6d7be2c175",
}

var resultsetAggregateFirstLastWindowPrevNthJavaExecutions = []string{
	"ResultSetAggregatePrevNthIndexedFirstLast",
}

const resultsetAggregateFirstLastWindowPrevNthCase = "prev-nth"

func runResultSetAggregateFirstLastWindowPrevNthScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if !scenarioHasCase(scenario, resultsetAggregateFirstLastWindowPrevNthCase) {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-firstlastwindow-prev-nth scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, resultsetAggregateFirstLastWindowPrevNthCase)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateFirstLastWindowPrevNthBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	value := esper.Field[resultsetAggregateFirstLastWindowPrevNthBean, int]("intPrimitive")
	query := esper.From[resultsetAggregateFirstLastWindowPrevNthBean](env, "SupportBean").
		Window(esper.LengthWindow(3)).
		Aggregate(
			esper.Alias("p0", esper.Prev[int](0, value)),
			esper.Alias("p1", esper.Prev[int](1, value)),
			esper.Alias("p2", esper.Prev[int](2, value)),
			esper.Alias("n0", esper.Nth[int](value, 0)),
			esper.Alias("n1", esper.Nth[int](value, 1)),
			esper.Alias("n2", esper.Nth[int](value, 2)),
			esper.Alias("l1", esper.Last[int](value, 0)),
			esper.Alias("l2", esper.Last[int](value, 1)),
			esper.Alias("l3", esper.Last[int](value, 2)),
		).
		Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateFirstLastWindowPrevNthJavaRuntimeIDs[0]),
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateFirstLastWindowPrevNthPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-firstlastwindow-prev-nth statement %q", name)
		}
		return statement, nil
	})
}

func decodeResultSetAggregateFirstLastWindowPrevNthPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("unsupported resultset-aggregate-firstlastwindow-prev-nth event type %q", step.EventType)
	}
	var value resultsetAggregateFirstLastWindowPrevNthBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
