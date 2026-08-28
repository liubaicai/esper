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

type resultsetAggregateSortedMinMaxByBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

const resultsetAggregateSortedMinMaxByJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateSortedMinMaxByJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateSortedMinMaxBy.java",
}

var resultsetAggregateSortedMinMaxByJavaRuntimeIDs = []string{
	"java-runtime-f1f2e402fb4c6783cf33",
	"java-runtime-7387fafedcfa741088b9",
	"java-runtime-5609d311148a91227b28",
}

var resultsetAggregateSortedMinMaxByJavaExecutions = []string{
	"ResultSetAggregateGroupedSortedMinMax",
	"ResultSetAggregateMultipleOverlappingCategories",
	"ResultSetAggregateMinByMaxByOverWindow",
}

var resultsetAggregateSortedMinMaxByCases = []string{
	"grouped",
	"overlap",
	"over-window",
}

type resultsetAggregateSortedMinMaxByExpectedEvent struct {
	theString     string
	intPrimitive  int
	longPrimitive int64
}

var resultsetAggregateSortedMinMaxByExpected = map[string][]resultsetAggregateSortedMinMaxByExpectedEvent{
	"grouped": {
		{theString: "E1", intPrimitive: 1, longPrimitive: 1},
		{theString: "E2", intPrimitive: 2, longPrimitive: 1},
		{theString: "E3", intPrimitive: 0, longPrimitive: 1},
		{theString: "E4", intPrimitive: 3, longPrimitive: 1},
		{theString: "E5", intPrimitive: -1, longPrimitive: 2},
		{theString: "E6", intPrimitive: -1, longPrimitive: 1},
		{theString: "E7", intPrimitive: 2, longPrimitive: 2},
	},
	"overlap": {
		{theString: "C", intPrimitive: 10, longPrimitive: 1},
		{theString: "P", intPrimitive: 5, longPrimitive: 2},
		{theString: "G", intPrimitive: 7, longPrimitive: 3},
		{theString: "A", intPrimitive: 7, longPrimitive: 4},
		{theString: "G", intPrimitive: 1, longPrimitive: 5},
		{theString: "X", intPrimitive: 7, longPrimitive: 6},
		{theString: "G", intPrimitive: 100, longPrimitive: 7},
		{theString: "Z", intPrimitive: 1000, longPrimitive: 8},
	},
	"over-window": {
		{theString: "E1", intPrimitive: 1, longPrimitive: 10},
		{theString: "E2", intPrimitive: 2, longPrimitive: 20},
		{theString: "E3", intPrimitive: 3, longPrimitive: 5},
		{theString: "E4", intPrimitive: 4, longPrimitive: 5},
		{theString: "E5", intPrimitive: 5, longPrimitive: 20},
		{theString: "E6", intPrimitive: 6, longPrimitive: 10},
		{theString: "E7", intPrimitive: 7, longPrimitive: 20},
		{theString: "E8", intPrimitive: 8, longPrimitive: 20},
		{theString: "E9", intPrimitive: 9, longPrimitive: 19},
		{theString: "E10", intPrimitive: 10, longPrimitive: 12},
	},
}

// runResultSetAggregateSortedMinMaxByScenario replays the first three
// ResultSetAggregateSortedMinMaxBy executions. Each execution gets a fresh
// environment and runtime, preserving the Java regression lifecycle while
// exposing only typed, analyzable aggregate builders to Go callers.
func runResultSetAggregateSortedMinMaxByScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateSortedMinMaxByScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range resultsetAggregateSortedMinMaxByCases {
		caseTrace, err := runResultSetAggregateSortedMinMaxByCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-sorted-minmax-by case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func validateResultSetAggregateSortedMinMaxByScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != "resultset-aggregate-sorted-minmax-by" {
		return fmt.Errorf("resultset-aggregate-sorted-minmax-by scenario has unsupported id %q", scenario.ID)
	}
	var expectedSteps int
	for _, caseName := range resultsetAggregateSortedMinMaxByCases {
		expectedSteps += 1 + len(resultsetAggregateSortedMinMaxByExpected[caseName])
	}
	if len(scenario.Steps) != expectedSteps {
		return fmt.Errorf("resultset-aggregate-sorted-minmax-by scenario must contain %d steps", expectedSteps)
	}
	stepIndex := 0
	for _, caseName := range resultsetAggregateSortedMinMaxByCases {
		step := scenario.Steps[stepIndex]
		if step.Op != "case" || step.Case != caseName {
			return fmt.Errorf("resultset-aggregate-sorted-minmax-by step %d must start case %q", stepIndex, caseName)
		}
		stepIndex++
		for eventIndex, expected := range resultsetAggregateSortedMinMaxByExpected[caseName] {
			step := scenario.Steps[stepIndex]
			if step.Op != "send" || step.EventType != "SupportBean" {
				return fmt.Errorf("resultset-aggregate-sorted-minmax-by step %d must be a SupportBean send", stepIndex)
			}
			actual, err := decodeResultSetAggregateSortedMinMaxByPayloadValue(step)
			if err != nil {
				return fmt.Errorf("resultset-aggregate-sorted-minmax-by case %q event %d: %w", caseName, eventIndex, err)
			}
			if actual.theString != expected.theString || actual.intPrimitive != expected.intPrimitive || actual.longPrimitive != expected.longPrimitive {
				return fmt.Errorf("resultset-aggregate-sorted-minmax-by case %q event %d = %#v, want %#v", caseName, eventIndex, actual, expected)
			}
			stepIndex++
		}
	}
	return nil
}

func runResultSetAggregateSortedMinMaxByCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateSortedMinMaxByBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	theString := esper.Field[resultsetAggregateSortedMinMaxByBean, string]("theString")
	intPrimitive := esper.Field[resultsetAggregateSortedMinMaxByBean, int]("intPrimitive")
	longPrimitive := esper.Field[resultsetAggregateSortedMinMaxByBean, int64]("longPrimitive")
	var query esper.Query
	switch caseName {
	case "grouped":
		query = esper.From[resultsetAggregateSortedMinMaxByBean](env, "SupportBean").
			Window(esper.GroupWindow(longPrimitive, esper.LengthWindow(3))).
			GroupBy(longPrimitive).
			Select(
				esper.Alias("c0", esper.WindowEvents()),
				esper.Alias("c1", esper.SortedEvents(esper.Descending(intPrimitive))),
				esper.Alias("c2", esper.SortedEvents(esper.Ascending(intPrimitive))),
				esper.Alias("c3", esper.MaxBy[esper.Event, int](esper.EventValue[esper.Event](), intPrimitive)),
				esper.Alias("c4", esper.MinBy[esper.Event, int](esper.EventValue[esper.Event](), intPrimitive)),
				esper.Alias("c5", esper.MaxByEver[esper.Event, int](esper.EventValue[esper.Event](), intPrimitive)),
				esper.Alias("c6", esper.MinByEver[esper.Event, int](esper.EventValue[esper.Event](), intPrimitive)),
			).
			Query(esper.StatementName("s0"))
	case "overlap":
		maxIntEver := esper.MaxByEver[esper.Event, int](esper.EventValue[esper.Event](), intPrimitive)
		minIntEver := esper.MinByEver[esper.Event, int](esper.EventValue[esper.Event](), intPrimitive)
		maxStringEver := esper.MaxByEver[esper.Event, string](esper.EventValue[esper.Event](), theString)
		minStringEver := esper.MinByEver[esper.Event, string](esper.EventValue[esper.Event](), theString)
		maxInt := esper.MaxBy[esper.Event, int](esper.EventValue[esper.Event](), intPrimitive)
		minInt := esper.MinBy[esper.Event, int](esper.EventValue[esper.Event](), intPrimitive)
		maxString := esper.MaxBy[esper.Event, string](esper.EventValue[esper.Event](), theString)
		minString := esper.MinBy[esper.Event, string](esper.EventValue[esper.Event](), theString)
		query = esper.From[resultsetAggregateSortedMinMaxByBean](env, "SupportBean").
			Window(esper.KeepAll()).
			Aggregate(
				esper.Alias("c0", esper.Property[int64](maxIntEver, "longPrimitive")),
				esper.Alias("c1", esper.Property[int64](maxStringEver, "longPrimitive")),
				esper.Alias("c2", esper.Property[int64](minIntEver, "longPrimitive")),
				esper.Alias("c3", esper.Property[int64](minStringEver, "longPrimitive")),
				esper.Alias("c4", esper.Property[int64](maxInt, "longPrimitive")),
				esper.Alias("c5", esper.Property[int64](maxString, "longPrimitive")),
				esper.Alias("c6", esper.Property[int64](minInt, "longPrimitive")),
				esper.Alias("c7", esper.Property[int64](minString, "longPrimitive")),
			).
			Query(esper.StatementName("s0"))
	case "over-window":
		maxEver := esper.MaxByEver[esper.Event, int64](esper.EventValue[esper.Event](), longPrimitive)
		minEver := esper.MinByEver[esper.Event, int64](esper.EventValue[esper.Event](), longPrimitive)
		maxCurrent := esper.MaxBy[esper.Event, int64](esper.EventValue[esper.Event](), longPrimitive)
		minCurrent := esper.MinBy[esper.Event, int64](esper.EventValue[esper.Event](), longPrimitive)
		query = esper.From[resultsetAggregateSortedMinMaxByBean](env, "SupportBean").
			Window(esper.LengthWindow(5)).
			Aggregate(
				esper.Alias("c0", maxEver),
				esper.Alias("c1", minEver),
				esper.Alias("c2", esper.Property[int64](maxCurrent, "longPrimitive")),
				esper.Alias("c3", esper.Property[string](maxCurrent, "theString")),
				esper.Alias("c4", esper.Property[int](maxCurrent, "intPrimitive")),
				esper.Alias("c5", maxCurrent),
				esper.Alias("c6", esper.Property[int64](minCurrent, "longPrimitive")),
				esper.Alias("c7", esper.Property[string](minCurrent, "theString")),
				esper.Alias("c8", esper.Property[int](minCurrent, "intPrimitive")),
				esper.Alias("c9", minCurrent),
			).
			Query(esper.StatementName("s0"))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset-aggregate-sorted-minmax-by case %q", caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateSortedMinMaxByJavaRuntimeIDs[resultsetAggregateSortedMinMaxByCaseIndex(caseName)]),
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateSortedMinMaxByPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-sorted-minmax-by statement %q", name)
		}
		return statement, nil
	})
}

func resultsetAggregateSortedMinMaxByCaseIndex(caseName string) int {
	for index, candidate := range resultsetAggregateSortedMinMaxByCases {
		if candidate == caseName {
			return index
		}
	}
	return 0
}

func decodeResultSetAggregateSortedMinMaxByPayload(step compat.Step) (any, error) {
	value, err := decodeResultSetAggregateSortedMinMaxByPayloadValue(step)
	if err != nil {
		return nil, err
	}
	return resultsetAggregateSortedMinMaxByBean{
		TheString:     value.theString,
		IntPrimitive:  value.intPrimitive,
		LongPrimitive: value.longPrimitive,
	}, nil
}

func decodeResultSetAggregateSortedMinMaxByPayloadValue(step compat.Step) (resultsetAggregateSortedMinMaxByExpectedEvent, error) {
	if step.EventType != "SupportBean" {
		return resultsetAggregateSortedMinMaxByExpectedEvent{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	payload, err := decodeResultSetAggregateSortedMinMaxByPayloadObject(step.Payload)
	if err != nil {
		return resultsetAggregateSortedMinMaxByExpectedEvent{}, fmt.Errorf("payload must be an object: %w", err)
	}
	if len(payload) != 3 {
		return resultsetAggregateSortedMinMaxByExpectedEvent{}, fmt.Errorf("payload must contain exactly theString, intPrimitive and longPrimitive")
	}
	var value resultsetAggregateSortedMinMaxByExpectedEvent
	stringPayload, ok := payload["theString"]
	if !ok || string(stringPayload) == "null" || json.Unmarshal(stringPayload, &value.theString) != nil {
		return resultsetAggregateSortedMinMaxByExpectedEvent{}, fmt.Errorf("payload theString must be a string")
	}
	intPayload, ok := payload["intPrimitive"]
	if !ok || string(intPayload) == "null" || json.Unmarshal(intPayload, &value.intPrimitive) != nil {
		return resultsetAggregateSortedMinMaxByExpectedEvent{}, fmt.Errorf("payload intPrimitive must be an integer")
	}
	longPayload, ok := payload["longPrimitive"]
	if !ok || string(longPayload) == "null" || json.Unmarshal(longPayload, &value.longPrimitive) != nil {
		return resultsetAggregateSortedMinMaxByExpectedEvent{}, fmt.Errorf("payload longPrimitive must be an integer")
	}
	return value, nil
}

func decodeResultSetAggregateSortedMinMaxByPayloadObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
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
