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

type resultsetAggregateMinMaxGroupByMarketData struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume *int64  `esper:"volume"`
}

const resultsetAggregateMinMaxGroupByJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateMinMaxGroupByJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateMaxMinGroupBy.java",
}

var resultsetAggregateMinMaxGroupByJavaRuntimeIDs = []string{
	"java-runtime-6ee286d6f857ddbbd091",
	"java-runtime-cd645170c5defa3996da",
}

var resultsetAggregateMinMaxGroupByJavaExecutions = []string{
	"ResultSetAggregateMinMax",
	"ResultSetAggregateMinNoGroupHaving",
}

const (
	resultsetAggregateMinMaxGroupByMinMaxCase = "minmax"
	resultsetAggregateMinMaxGroupByHavingCase = "min-no-group-having"
)

var resultsetAggregateMinMaxGroupByCases = []string{
	resultsetAggregateMinMaxGroupByMinMaxCase,
	resultsetAggregateMinMaxGroupByHavingCase,
}

type resultsetAggregateMinMaxGroupByExpectedInput struct {
	symbol string
	volume *int64
}

func resultsetAggregateMinMaxGroupByVolume(value int64) *int64 {
	return &value
}

var resultsetAggregateMinMaxGroupByExpected = map[string][]resultsetAggregateMinMaxGroupByExpectedInput{
	resultsetAggregateMinMaxGroupByMinMaxCase: {
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(50)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(30)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(30)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(90)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(100)},
		{symbol: "IBM", volume: resultsetAggregateMinMaxGroupByVolume(20)},
		{symbol: "IBM", volume: resultsetAggregateMinMaxGroupByVolume(5)},
		{symbol: "IBM", volume: resultsetAggregateMinMaxGroupByVolume(15)},
		{symbol: "IBM", volume: resultsetAggregateMinMaxGroupByVolume(18)},
		{symbol: "IBM", volume: nil},
		{symbol: "IBM", volume: nil},
		{symbol: "IBM", volume: nil},
	},
	resultsetAggregateMinMaxGroupByHavingCase: {
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(100)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(105)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(100)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(131)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(132)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(129)},
	},
}

// runResultSetAggregateMinMaxGroupByScenario replays the two bounded
// ResultSetAggregateMaxMinGroupBy executions represented by the scenario. Each
// case gets a fresh typed environment and the Java execution's runtime URI.
func runResultSetAggregateMinMaxGroupByScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateMinMaxGroupByScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for index, caseName := range resultsetAggregateMinMaxGroupByCases {
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultSetAggregateMinMaxGroupByCase(ctx, caseScenario, caseName, resultsetAggregateMinMaxGroupByJavaRuntimeIDs[index])
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-minmax-groupby case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateMinMaxGroupByCase(ctx context.Context, scenario compat.Scenario, caseName, runtimeURI string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateMinMaxGroupByMarketData](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}

	symbol := esper.Field[resultsetAggregateMinMaxGroupByMarketData, string]("symbol")
	volume := esper.Field[resultsetAggregateMinMaxGroupByMarketData, *int64]("volume")
	var query esper.Query
	switch caseName {
	case resultsetAggregateMinMaxGroupByMinMaxCase:
		volumeLong := esper.Cast[*int64, int64](volume)
		filtered := esper.From[resultsetAggregateMinMaxGroupByMarketData](env, "SupportMarketDataBean").
			Filter(esper.Or(
				esper.Or(
					esper.Equal[string](symbol, esper.Literal("DELL")),
					esper.Equal[string](symbol, esper.Literal("IBM")),
				),
				esper.Equal[string](symbol, esper.Literal("GE")),
			)).
			Window(esper.LengthWindow(3)).
			GroupBy(symbol)
		query = filtered.Select(
			esper.Alias("symbol", symbol),
			esper.Alias("minVol", esper.Min[int64](volumeLong)),
			esper.Alias("maxVol", esper.Max[int64](volumeLong)),
			esper.Alias("minDistVol", esper.DistinctAggregate[int64](esper.Min[int64](volumeLong), volumeLong)),
			esper.Alias("maxDistVol", esper.DistinctAggregate[int64](esper.Max[int64](volumeLong), volumeLong)),
		).Query(esper.StatementName("s0"), esper.WithOldStream())
	case resultsetAggregateMinMaxGroupByHavingCase:
		volumeFloat := esper.Cast[*int64, float64](volume)
		minimum := esper.Min[float64](volumeFloat)
		query = esper.From[resultsetAggregateMinMaxGroupByMarketData](env, "SupportMarketDataBean").
			Window(esper.TimeWindow(5 * time.Second)).
			Aggregate(esper.Alias("symbol", symbol)).
			Having(esper.Greater[float64](volumeFloat, esper.Multiply[float64](minimum, esper.Literal(1.3)))).
			Query(esper.StatementName("s0"))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset-aggregate-minmax-groupby case %q", caseName)
	}

	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(runtimeURI),
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
	return compat.ReplayWithStatements(ctx, engine, statement, scenario, decodeResultSetAggregateMinMaxGroupByPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-minmax-groupby statement %q", name)
		}
		return statement, nil
	})
}

func validateResultSetAggregateMinMaxGroupByScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != "resultset-aggregate-minmax-groupby" {
		return fmt.Errorf("resultset-aggregate-minmax-groupby scenario has unsupported id %q", scenario.ID)
	}
	expectedSteps := 0
	for _, caseName := range resultsetAggregateMinMaxGroupByCases {
		expectedSteps++
		expectedSteps += len(resultsetAggregateMinMaxGroupByExpected[caseName])
	}
	if len(scenario.Steps) != expectedSteps {
		return fmt.Errorf("resultset-aggregate-minmax-groupby scenario must contain %d steps", expectedSteps)
	}
	stepIndex := 0
	for _, caseName := range resultsetAggregateMinMaxGroupByCases {
		marker := scenario.Steps[stepIndex]
		if marker.Op != "case" || marker.Case != caseName {
			return fmt.Errorf("resultset-aggregate-minmax-groupby step %d must start case %q", stepIndex, caseName)
		}
		stepIndex++
		for eventIndex, expected := range resultsetAggregateMinMaxGroupByExpected[caseName] {
			step := scenario.Steps[stepIndex]
			if step.Op != "send" || step.EventType != "SupportMarketDataBean" {
				return fmt.Errorf("resultset-aggregate-minmax-groupby case %q step %d must be a SupportMarketDataBean send", caseName, stepIndex)
			}
			actual, err := decodeResultSetAggregateMinMaxGroupByPayloadValue(step)
			if err != nil {
				return fmt.Errorf("resultset-aggregate-minmax-groupby case %q event %d: %w", caseName, eventIndex, err)
			}
			if actual.Symbol != expected.symbol || actual.Price != 0 || !resultsetAggregateMinMaxGroupByVolumeEqual(actual.Volume, expected.volume) {
				return fmt.Errorf("resultset-aggregate-minmax-groupby case %q event %d = %#v, want symbol=%q price=0 volume=%v", caseName, eventIndex, actual, expected.symbol, expected.volume)
			}
			stepIndex++
		}
	}
	return nil
}

func resultsetAggregateMinMaxGroupByVolumeEqual(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func decodeResultSetAggregateMinMaxGroupByPayload(step compat.Step) (any, error) {
	value, err := decodeResultSetAggregateMinMaxGroupByPayloadValue(step)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func decodeResultSetAggregateMinMaxGroupByPayloadValue(step compat.Step) (resultsetAggregateMinMaxGroupByMarketData, error) {
	if step.EventType != "SupportMarketDataBean" {
		return resultsetAggregateMinMaxGroupByMarketData{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	payload, err := decodeResultSetAggregateMinMaxGroupByPayloadObject(step.Payload)
	if err != nil {
		return resultsetAggregateMinMaxGroupByMarketData{}, fmt.Errorf("payload must be an object: %w", err)
	}
	if len(payload) != 3 {
		return resultsetAggregateMinMaxGroupByMarketData{}, fmt.Errorf("payload must contain exactly symbol, volume and price")
	}
	var value resultsetAggregateMinMaxGroupByMarketData
	symbolPayload, ok := payload["symbol"]
	if !ok || string(bytes.TrimSpace(symbolPayload)) == "null" || json.Unmarshal(symbolPayload, &value.Symbol) != nil {
		return resultsetAggregateMinMaxGroupByMarketData{}, fmt.Errorf("payload symbol must be a string")
	}
	pricePayload, ok := payload["price"]
	if !ok || string(bytes.TrimSpace(pricePayload)) == "null" || json.Unmarshal(pricePayload, &value.Price) != nil {
		return resultsetAggregateMinMaxGroupByMarketData{}, fmt.Errorf("payload price must be a number")
	}
	volumePayload, ok := payload["volume"]
	if !ok {
		return resultsetAggregateMinMaxGroupByMarketData{}, fmt.Errorf("payload volume must be an integer or null")
	}
	if string(bytes.TrimSpace(volumePayload)) != "null" {
		var volume int64
		if err := json.Unmarshal(volumePayload, &volume); err != nil {
			return resultsetAggregateMinMaxGroupByMarketData{}, fmt.Errorf("payload volume must be an integer or null")
		}
		value.Volume = &volume
	}
	return value, nil
}

func decodeResultSetAggregateMinMaxGroupByPayloadObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
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
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("payload contains trailing JSON")
		}
		return nil, err
	}
	return payload, nil
}
