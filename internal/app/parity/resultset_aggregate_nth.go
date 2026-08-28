package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetAggregateNthBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const resultsetAggregateNthJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateNthJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateNTh.java",
}

var resultsetAggregateNthJavaRuntimeIDs = []string{
	"java-runtime-1a257602734874ff3fc4",
}

var resultsetAggregateNthJavaExecutions = []string{
	"ResultSetAggregateNTh",
}

const resultsetAggregateNthCase = "nth"

// runResultSetAggregateNthScenario replays the grouped keep-all nth aggregate
// through fresh typed Go deployments for the Java EPL and model/SODA phases.
// The two Java syntax paths have identical observable behavior, but remain
// separate trace phases so the direct execution's lifecycle is preserved.
func runResultSetAggregateNthScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateNthScenario(scenario); err != nil {
		return compat.Trace{}, err
	}

	phases := []string{"epl", "soda"}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for phaseIndex, phase := range phases {
		phaseTrace, err := runResultSetAggregateNthPhase(ctx, scenario, phase)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-nth phase %q: %w", phase, err)
		}
		for recordIndex := range phaseTrace.Records {
			phaseTrace.Records[recordIndex].Case = phase
			phaseTrace.Records[recordIndex].Sequence = uint64(phaseIndex*3 + recordIndex + 1)
		}
		trace.Records = append(trace.Records, phaseTrace.Records...)
	}
	return trace, nil
}

func validateResultSetAggregateNthScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != "resultset-aggregate-nth" {
		return fmt.Errorf("resultset-aggregate-nth scenario has unsupported id %q", scenario.ID)
	}
	if len(scenario.Steps) != 10 {
		return fmt.Errorf("resultset-aggregate-nth scenario must contain one nth case and nine sends")
	}
	first := scenario.Steps[0]
	if first.Op != "case" || first.Case != resultsetAggregateNthCase {
		return fmt.Errorf("resultset-aggregate-nth scenario must start with case %q", resultsetAggregateNthCase)
	}
	for index, step := range scenario.Steps[1:] {
		stepNumber := index + 1
		if step.Op != "send" || step.EventType != "SupportBean" {
			return fmt.Errorf("resultset-aggregate-nth step %d must be a SupportBean send", stepNumber)
		}
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(step.Payload, &payload); err != nil || payload == nil {
			return fmt.Errorf("resultset-aggregate-nth step %d payload must be an object", stepNumber)
		}
		if len(payload) != 2 {
			return fmt.Errorf("resultset-aggregate-nth step %d payload must contain exactly theString and intPrimitive", stepNumber)
		}
		stringPayload, ok := payload["theString"]
		if !ok || string(stringPayload) == "null" {
			return fmt.Errorf("resultset-aggregate-nth step %d payload requires string theString", stepNumber)
		}
		var theString string
		if err := json.Unmarshal(stringPayload, &theString); err != nil {
			return fmt.Errorf("resultset-aggregate-nth step %d payload theString must be a string", stepNumber)
		}
		intPayload, ok := payload["intPrimitive"]
		if !ok || string(intPayload) == "null" {
			return fmt.Errorf("resultset-aggregate-nth step %d payload requires integer intPrimitive", stepNumber)
		}
		var intPrimitive int
		if err := json.Unmarshal(intPayload, &intPrimitive); err != nil {
			return fmt.Errorf("resultset-aggregate-nth step %d payload intPrimitive must be an integer", stepNumber)
		}
	}
	return nil
}

func runResultSetAggregateNthPhase(ctx context.Context, scenario compat.Scenario, _ string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, resultsetAggregateNthCase)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateNthBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	key := esper.Field[resultsetAggregateNthBean, string]("theString")
	value := esper.Field[resultsetAggregateNthBean, int]("intPrimitive")
	query := esper.From[resultsetAggregateNthBean](env, "SupportBean").
		Window(esper.KeepAll()).
		GroupBy(key).
		Select(
			esper.Alias("theString", key),
			esper.Alias("int1", esper.Nth[int](value, 0)),
			esper.Alias("int2", esper.Nth[int](value, 1)),
		).
		Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputLastEveryEvents(3)),
			esper.OrderBy(esper.Ascending(esper.ResultField[string]("theString"))),
		)
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateNthJavaRuntimeIDs[0]),
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateNthPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-nth statement %q", name)
		}
		return statement, nil
	})
}

func decodeResultSetAggregateNthPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("unsupported resultset-aggregate-nth event type %q", step.EventType)
	}
	var value resultsetAggregateNthBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}
