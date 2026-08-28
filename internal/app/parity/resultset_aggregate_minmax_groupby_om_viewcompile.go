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

type resultsetAggregateMinMaxGroupByOMViewCompileMarketData struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume *int64  `esper:"volume"`
}

const resultsetAggregateMinMaxGroupByOMViewCompileJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateMinMaxGroupByOMViewCompileJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateMaxMinGroupBy.java",
}

var resultsetAggregateMinMaxGroupByOMViewCompileJavaRuntimeIDs = []string{
	"java-runtime-2408009c4113ea2214c4",
	"java-runtime-30dbed587648347e610a",
}

var resultsetAggregateMinMaxGroupByOMViewCompileJavaExecutions = []string{
	"ResultSetAggregateMinMaxOM",
	"ResultSetAggregateMinMaxViewCompile",
}

const (
	resultsetAggregateMinMaxGroupByOMViewCompileOMCase          = "minmax-om"
	resultsetAggregateMinMaxGroupByOMViewCompileViewCompileCase = "minmax-view-compile"
)

var resultsetAggregateMinMaxGroupByOMViewCompileCases = []string{
	resultsetAggregateMinMaxGroupByOMViewCompileOMCase,
	resultsetAggregateMinMaxGroupByOMViewCompileViewCompileCase,
}

type resultsetAggregateMinMaxGroupByOMViewCompileExpectedInput struct {
	symbol string
	volume *int64
}

func resultsetAggregateMinMaxGroupByOMViewCompileVolume(value int64) *int64 {
	return &value
}

var resultsetAggregateMinMaxGroupByOMViewCompileExpected = map[string][]resultsetAggregateMinMaxGroupByOMViewCompileExpectedInput{
	resultsetAggregateMinMaxGroupByOMViewCompileOMCase: {
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(50)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(30)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(30)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(90)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(100)},
		{symbol: "IBM", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(20)},
		{symbol: "IBM", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(5)},
		{symbol: "IBM", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(15)},
		{symbol: "IBM", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(18)},
		{symbol: "IBM", volume: nil},
		{symbol: "IBM", volume: nil},
		{symbol: "IBM", volume: nil},
	},
	resultsetAggregateMinMaxGroupByOMViewCompileViewCompileCase: {
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(50)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(30)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(30)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(90)},
		{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(100)},
		{symbol: "IBM", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(20)},
		{symbol: "IBM", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(5)},
		{symbol: "IBM", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(15)},
		{symbol: "IBM", volume: resultsetAggregateMinMaxGroupByOMViewCompileVolume(18)},
		{symbol: "IBM", volume: nil},
		{symbol: "IBM", volume: nil},
		{symbol: "IBM", volume: nil},
	},
}

func runResultSetAggregateMinMaxGroupByOMViewCompileScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateMinMaxGroupByOMViewCompileScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for index, caseName := range resultsetAggregateMinMaxGroupByOMViewCompileCases {
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultSetAggregateMinMaxGroupByOMViewCompileCase(ctx, caseScenario, caseName, resultsetAggregateMinMaxGroupByOMViewCompileJavaRuntimeIDs[index])
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-minmax-groupby-om-viewcompile case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateMinMaxGroupByOMViewCompileCase(ctx context.Context, scenario compat.Scenario, caseName, runtimeURI string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateMinMaxGroupByOMViewCompileMarketData](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	symbol := esper.Field[resultsetAggregateMinMaxGroupByOMViewCompileMarketData, string]("symbol")
	volume := esper.Field[resultsetAggregateMinMaxGroupByOMViewCompileMarketData, *int64]("volume")
	volumeLong := esper.Cast[*int64, int64](volume)
	filtered := esper.From[resultsetAggregateMinMaxGroupByOMViewCompileMarketData](env, "SupportMarketDataBean").
		Filter(esper.Or(
			esper.Or(
				esper.Equal[string](symbol, esper.Literal("DELL")),
				esper.Equal[string](symbol, esper.Literal("IBM")),
			),
			esper.Equal[string](symbol, esper.Literal("GE")),
		)).
		Window(esper.LengthWindow(3)).
		GroupBy(symbol)
	query := filtered.Select(
		esper.Alias("symbol", symbol),
		esper.Alias("minVol", esper.Min[int64](volumeLong)),
		esper.Alias("maxVol", esper.Max[int64](volumeLong)),
		esper.Alias("minDistVol", esper.DistinctAggregate[int64](esper.Min[int64](volumeLong), volumeLong)),
		esper.Alias("maxDistVol", esper.DistinctAggregate[int64](esper.Max[int64](volumeLong), volumeLong)),
	).Query(esper.StatementName("s0"), esper.WithOldStream())
	if caseName != resultsetAggregateMinMaxGroupByOMViewCompileOMCase && caseName != resultsetAggregateMinMaxGroupByOMViewCompileViewCompileCase {
		return compat.Trace{}, fmt.Errorf("unsupported resultset-aggregate-minmax-groupby-om-viewcompile case %q", caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()), esper.WithRuntimeURI(runtimeURI))
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
	return compat.ReplayWithStatements(ctx, engine, statement, scenario, decodeResultSetAggregateMinMaxGroupByOMViewCompilePayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-minmax-groupby-om-viewcompile statement %q", name)
		}
		return statement, nil
	})
}

func validateResultSetAggregateMinMaxGroupByOMViewCompileScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != "resultset-aggregate-minmax-groupby-om-viewcompile" {
		return fmt.Errorf("resultset-aggregate-minmax-groupby-om-viewcompile scenario has unsupported id %q", scenario.ID)
	}
	expectedSteps := 0
	for _, caseName := range resultsetAggregateMinMaxGroupByOMViewCompileCases {
		expectedSteps++
		expectedSteps += len(resultsetAggregateMinMaxGroupByOMViewCompileExpected[caseName])
	}
	if len(scenario.Steps) != expectedSteps {
		return fmt.Errorf("resultset-aggregate-minmax-groupby-om-viewcompile scenario must contain %d steps", expectedSteps)
	}
	stepIndex := 0
	for _, caseName := range resultsetAggregateMinMaxGroupByOMViewCompileCases {
		marker := scenario.Steps[stepIndex]
		if marker.Op != "case" || marker.Case != caseName {
			return fmt.Errorf("resultset-aggregate-minmax-groupby-om-viewcompile step %d must start case %q", stepIndex, caseName)
		}
		stepIndex++
		for eventIndex, expected := range resultsetAggregateMinMaxGroupByOMViewCompileExpected[caseName] {
			step := scenario.Steps[stepIndex]
			if step.Op != "send" || step.EventType != "SupportMarketDataBean" {
				return fmt.Errorf("resultset-aggregate-minmax-groupby-om-viewcompile case %q step %d must be a SupportMarketDataBean send", caseName, stepIndex)
			}
			actual, err := decodeResultSetAggregateMinMaxGroupByOMViewCompilePayloadValue(step)
			if err != nil {
				return fmt.Errorf("resultset-aggregate-minmax-groupby-om-viewcompile case %q event %d: %w", caseName, eventIndex, err)
			}
			if actual.Symbol != expected.symbol || actual.Price != 0 || !resultsetAggregateMinMaxGroupByOMViewCompileVolumeEqual(actual.Volume, expected.volume) {
				return fmt.Errorf("resultset-aggregate-minmax-groupby-om-viewcompile case %q event %d = %#v, want symbol=%q price=0 volume=%v", caseName, eventIndex, actual, expected.symbol, expected.volume)
			}
			stepIndex++
		}
	}
	return nil
}

func resultsetAggregateMinMaxGroupByOMViewCompileVolumeEqual(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func decodeResultSetAggregateMinMaxGroupByOMViewCompilePayload(step compat.Step) (any, error) {
	value, err := decodeResultSetAggregateMinMaxGroupByOMViewCompilePayloadValue(step)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func decodeResultSetAggregateMinMaxGroupByOMViewCompilePayloadValue(step compat.Step) (resultsetAggregateMinMaxGroupByOMViewCompileMarketData, error) {
	if step.EventType != "SupportMarketDataBean" {
		return resultsetAggregateMinMaxGroupByOMViewCompileMarketData{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	payload, err := decodeResultSetAggregateMinMaxGroupByOMViewCompilePayloadObject(step.Payload)
	if err != nil {
		return resultsetAggregateMinMaxGroupByOMViewCompileMarketData{}, fmt.Errorf("payload must be an object: %w", err)
	}
	if len(payload) != 3 {
		return resultsetAggregateMinMaxGroupByOMViewCompileMarketData{}, fmt.Errorf("payload must contain exactly symbol, volume and price")
	}
	var value resultsetAggregateMinMaxGroupByOMViewCompileMarketData
	symbolPayload, ok := payload["symbol"]
	if !ok || string(bytes.TrimSpace(symbolPayload)) == "null" || json.Unmarshal(symbolPayload, &value.Symbol) != nil {
		return resultsetAggregateMinMaxGroupByOMViewCompileMarketData{}, fmt.Errorf("payload symbol must be a string")
	}
	pricePayload, ok := payload["price"]
	if !ok || string(bytes.TrimSpace(pricePayload)) == "null" || json.Unmarshal(pricePayload, &value.Price) != nil {
		return resultsetAggregateMinMaxGroupByOMViewCompileMarketData{}, fmt.Errorf("payload price must be a number")
	}
	volumePayload, ok := payload["volume"]
	if !ok {
		return resultsetAggregateMinMaxGroupByOMViewCompileMarketData{}, fmt.Errorf("payload volume must be an integer or null")
	}
	if string(bytes.TrimSpace(volumePayload)) != "null" {
		var volume int64
		if err := json.Unmarshal(volumePayload, &volume); err != nil {
			return resultsetAggregateMinMaxGroupByOMViewCompileMarketData{}, fmt.Errorf("payload volume must be an integer or null")
		}
		value.Volume = &volume
	}
	return value, nil
}

func decodeResultSetAggregateMinMaxGroupByOMViewCompilePayloadObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
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
