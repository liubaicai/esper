package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetAggregateSortedMultiCriteriaSimpleBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const resultsetAggregateSortedMultiCriteriaSimpleJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateSortedMultiCriteriaSimpleJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateSortedMinMaxBy.java",
}

var resultsetAggregateSortedMultiCriteriaSimpleJavaRuntimeIDs = []string{
	"java-runtime-9f50a7b345919de5a8cf",
}

var resultsetAggregateSortedMultiCriteriaSimpleJavaExecutions = []string{
	"ResultSetAggregateMultipleCriteriaSimple",
}

const resultsetAggregateSortedMultiCriteriaSimpleCase = "multiple-criteria-simple"

type resultsetAggregateSortedMultiCriteriaSimpleExpectedEvent struct {
	theString    string
	intPrimitive int
}

var resultsetAggregateSortedMultiCriteriaSimpleExpected = []resultsetAggregateSortedMultiCriteriaSimpleExpectedEvent{
	{theString: "C", intPrimitive: 10},
	{theString: "D", intPrimitive: 20},
	{theString: "C", intPrimitive: 15},
	{theString: "D", intPrimitive: 19},
}

func runResultSetAggregateSortedMultiCriteriaSimpleScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateSortedMultiCriteriaSimpleScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	caseScenario, err := scenarioForCase(scenario, resultsetAggregateSortedMultiCriteriaSimpleCase)
	if err != nil {
		return compat.Trace{}, err
	}

	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateSortedMultiCriteriaSimpleBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	theString := esper.Field[resultsetAggregateSortedMultiCriteriaSimpleBean, string]("theString")
	intPrimitive := esper.Field[resultsetAggregateSortedMultiCriteriaSimpleBean, int]("intPrimitive")
	query := esper.From[resultsetAggregateSortedMultiCriteriaSimpleBean](env, "SupportBean").
		Window(esper.KeepAll()).
		Aggregate(
			esper.Alias("c0", esper.SortedEvents(esper.Descending(theString), esper.Descending(intPrimitive))),
		).
		Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateSortedMultiCriteriaSimpleJavaRuntimeIDs[0]),
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateSortedMultiCriteriaSimplePayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-sorted-multi-criteria-simple statement %q", name)
		}
		return statement, nil
	})
}

func validateResultSetAggregateSortedMultiCriteriaSimpleScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != "resultset-aggregate-sorted-multi-criteria-simple" {
		return fmt.Errorf("resultset-aggregate-sorted-multi-criteria-simple scenario has unsupported id %q", scenario.ID)
	}
	expectedSteps := 1 + len(resultsetAggregateSortedMultiCriteriaSimpleExpected)
	if len(scenario.Steps) != expectedSteps {
		return fmt.Errorf("resultset-aggregate-sorted-multi-criteria-simple scenario must contain one %s case and four sends", resultsetAggregateSortedMultiCriteriaSimpleCase)
	}
	first := scenario.Steps[0]
	if first.Op != "case" || first.Case != resultsetAggregateSortedMultiCriteriaSimpleCase {
		return fmt.Errorf("resultset-aggregate-sorted-multi-criteria-simple scenario must start with case %q", resultsetAggregateSortedMultiCriteriaSimpleCase)
	}
	for index, expected := range resultsetAggregateSortedMultiCriteriaSimpleExpected {
		stepNumber := index + 1
		step := scenario.Steps[stepNumber]
		if step.Op != "send" || step.EventType != "SupportBean" {
			return fmt.Errorf("resultset-aggregate-sorted-multi-criteria-simple step %d must be a SupportBean send", stepNumber)
		}
		actual, err := decodeResultSetAggregateSortedMultiCriteriaSimplePayloadValue(step)
		if err != nil {
			return fmt.Errorf("resultset-aggregate-sorted-multi-criteria-simple event %d: %w", index, err)
		}
		if actual != expected {
			return fmt.Errorf("resultset-aggregate-sorted-multi-criteria-simple event %d = %#v, want %#v", index, actual, expected)
		}
	}
	return nil
}

func decodeResultSetAggregateSortedMultiCriteriaSimplePayload(step compat.Step) (any, error) {
	value, err := decodeResultSetAggregateSortedMultiCriteriaSimplePayloadValue(step)
	if err != nil {
		return nil, err
	}
	return resultsetAggregateSortedMultiCriteriaSimpleBean{
		TheString:    value.theString,
		IntPrimitive: value.intPrimitive,
	}, nil
}

func decodeResultSetAggregateSortedMultiCriteriaSimplePayloadValue(step compat.Step) (resultsetAggregateSortedMultiCriteriaSimpleExpectedEvent, error) {
	if step.EventType != "SupportBean" {
		return resultsetAggregateSortedMultiCriteriaSimpleExpectedEvent{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	payload, err := decodeResultSetAggregateSortedMultiCriteriaSimplePayloadObject(step.Payload)
	if err != nil {
		return resultsetAggregateSortedMultiCriteriaSimpleExpectedEvent{}, fmt.Errorf("payload must be an object: %w", err)
	}
	if len(payload) != 2 {
		return resultsetAggregateSortedMultiCriteriaSimpleExpectedEvent{}, fmt.Errorf("payload must contain exactly theString and intPrimitive")
	}
	stringPayload, ok := payload["theString"]
	if !ok || string(stringPayload) == "null" {
		return resultsetAggregateSortedMultiCriteriaSimpleExpectedEvent{}, fmt.Errorf("payload theString must be a string")
	}
	var value resultsetAggregateSortedMultiCriteriaSimpleExpectedEvent
	if err := json.Unmarshal(stringPayload, &value.theString); err != nil {
		return resultsetAggregateSortedMultiCriteriaSimpleExpectedEvent{}, fmt.Errorf("payload theString must be a string")
	}
	intPayload, ok := payload["intPrimitive"]
	if !ok || string(intPayload) == "null" {
		return resultsetAggregateSortedMultiCriteriaSimpleExpectedEvent{}, fmt.Errorf("payload intPrimitive must be an integer")
	}
	var intPrimitive int32
	if err := json.Unmarshal(intPayload, &intPrimitive); err != nil {
		return resultsetAggregateSortedMultiCriteriaSimpleExpectedEvent{}, fmt.Errorf("payload intPrimitive must be an integer")
	}
	value.intPrimitive = int(intPrimitive)
	return value, nil
}

func decodeResultSetAggregateSortedMultiCriteriaSimplePayloadObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('{') {
		return nil, fmt.Errorf("payload must be an object")
	}
	payload := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("payload object key is not a string")
		}
		if _, exists := payload[key]; exists {
			return nil, fmt.Errorf("payload contains duplicate field %q", key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		payload[key] = value
	}
	token, err = decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('}') {
		return nil, fmt.Errorf("payload object is not closed")
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("payload contains trailing JSON")
		}
		return nil, err
	}
	return payload, nil
}
