package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetAggregateMinMaxGroupByJoinSelectHavingJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateMinMaxGroupByJoinSelectHavingJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateMaxMinGroupBy.java",
}

var resultsetAggregateMinMaxGroupByJoinSelectHavingJavaRuntimeIDs = []string{
	"java-runtime-802aec9425dc772e3aa5",
	"java-runtime-060f5af73edf420ef45a",
}

var resultsetAggregateMinMaxGroupByJoinSelectHavingJavaExecutions = []string{
	"ResultSetAggregateMinMaxJoin",
	"ResultSetAggregateMinNoGroupSelectHaving",
}

const (
	resultsetAggregateMinMaxGroupByJoinSelectHavingJoinCase   = "minmax-join"
	resultsetAggregateMinMaxGroupByJoinSelectHavingSelectCase = "min-no-group-select-having"
)

var resultsetAggregateMinMaxGroupByJoinSelectHavingCases = []string{
	resultsetAggregateMinMaxGroupByJoinSelectHavingJoinCase,
	resultsetAggregateMinMaxGroupByJoinSelectHavingSelectCase,
}

var resultsetAggregateMinMaxGroupByJoinSelectHavingMarketInputs = []resultsetAggregateMinMaxGroupByExpectedInput{
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
}

var resultsetAggregateMinMaxGroupByJoinSelectHavingSelectInputs = []resultsetAggregateMinMaxGroupByExpectedInput{
	{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(100)},
	{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(105)},
	{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(100)},
	{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(131)},
	{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(132)},
	{symbol: "DELL", volume: resultsetAggregateMinMaxGroupByVolume(129)},
}

func runResultSetAggregateMinMaxGroupByJoinSelectHavingScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateMinMaxGroupByJoinSelectHavingScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for index, caseName := range resultsetAggregateMinMaxGroupByJoinSelectHavingCases {
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultSetAggregateMinMaxGroupByJoinSelectHavingCase(ctx, caseScenario, caseName, resultsetAggregateMinMaxGroupByJoinSelectHavingJavaRuntimeIDs[index])
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-minmax-groupby-join-select-having case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateMinMaxGroupByJoinSelectHavingCase(ctx context.Context, scenario compat.Scenario, caseName, runtimeURI string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateMinMaxGroupByMarketData](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}

	var query esper.Query
	var decode compat.DecodePayload
	symbol := esper.Field[resultsetAggregateMinMaxGroupByMarketData, string]("symbol")
	volume := esper.Field[resultsetAggregateMinMaxGroupByMarketData, *int64]("volume")
	switch caseName {
	case resultsetAggregateMinMaxGroupByJoinSelectHavingJoinCase:
		if _, err := esper.RegisterStruct[resultsetAggregateMinMaxGroupByStringBean](env, "SupportBeanString"); err != nil {
			return compat.Trace{}, err
		}
		theString := esper.Field[resultsetAggregateMinMaxGroupByStringBean, string]("theString")
		volumeLong := esper.Cast[*int64, int64](esper.JoinField[*int64](1, "volume"))
		joined := esper.Join(
			esper.From[resultsetAggregateMinMaxGroupByStringBean](env, "SupportBeanString").Window(esper.LengthWindow(100)),
			esper.From[resultsetAggregateMinMaxGroupByMarketData](env, "SupportMarketDataBean").
				Filter(esper.Or(
					esper.Or(
						esper.Equal[string](symbol, esper.Literal("DELL")),
						esper.Equal[string](symbol, esper.Literal("IBM")),
					),
					esper.Equal[string](symbol, esper.Literal("GE")),
				)).
				Window(esper.LengthWindow(3)),
			esper.OnEqual(theString, symbol),
		).GroupBy(esper.JoinField[string](1, "symbol"))
		query = joined.Select(
			esper.Alias("symbol", esper.JoinField[string](1, "symbol")),
			esper.Alias("minVol", esper.Min[int64](volumeLong)),
			esper.Alias("maxVol", esper.Max[int64](volumeLong)),
			esper.Alias("minDistVol", esper.DistinctAggregate[int64](esper.Min[int64](volumeLong), volumeLong)),
			esper.Alias("maxDistVol", esper.DistinctAggregate[int64](esper.Max[int64](volumeLong), volumeLong)),
		).Query(esper.StatementName("s0"), esper.WithOldStream())
		decode = decodeResultSetAggregateMinMaxGroupByJoinSelectHavingPayload
	case resultsetAggregateMinMaxGroupByJoinSelectHavingSelectCase:
		volumeFloat := esper.Cast[*int64, float64](volume)
		minimum := esper.Min[float64](volumeFloat)
		query = esper.From[resultsetAggregateMinMaxGroupByMarketData](env, "SupportMarketDataBean").
			Window(esper.LengthWindow(5)).
			Aggregate(
				esper.Alias("symbol", symbol),
				esper.Alias("mymin", esper.Min[int64](esper.Cast[*int64, int64](volume))),
			).
			Having(esper.Greater[float64](volumeFloat, esper.Multiply[float64](minimum, esper.Literal(1.3)))).
			Query(esper.StatementName("s0"))
		decode = decodeResultSetAggregateMinMaxGroupByJoinSelectHavingPayload
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset-aggregate-minmax-groupby-join-select-having case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, scenario, decode, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-minmax-groupby-join-select-having statement %q", name)
		}
		return statement, nil
	})
}

func validateResultSetAggregateMinMaxGroupByJoinSelectHavingScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	const scenarioID = "resultset-aggregate-minmax-groupby-join-select-having"
	if scenario.ID != scenarioID {
		return fmt.Errorf("%s scenario has unsupported id %q", scenarioID, scenario.ID)
	}
	expectedSteps := 1 + 2 + len(resultsetAggregateMinMaxGroupByJoinSelectHavingMarketInputs) + 1 + len(resultsetAggregateMinMaxGroupByJoinSelectHavingSelectInputs)
	if len(scenario.Steps) != expectedSteps {
		return fmt.Errorf("%s scenario must contain %d steps", scenarioID, expectedSteps)
	}
	var err error
	stepIndex := 0
	if marker := scenario.Steps[stepIndex]; marker.Op != "case" || marker.Case != resultsetAggregateMinMaxGroupByJoinSelectHavingJoinCase {
		return fmt.Errorf("step %d must start case %q", stepIndex, resultsetAggregateMinMaxGroupByJoinSelectHavingJoinCase)
	}
	stepIndex++
	for _, expected := range []string{"DELL", "IBM"} {
		step := scenario.Steps[stepIndex]
		actual, err := decodeResultSetAggregateMinMaxGroupByJoinSelectHavingString(step)
		if err != nil {
			return fmt.Errorf("join seed %d: %w", stepIndex, err)
		}
		if actual.TheString != expected {
			return fmt.Errorf("join seed %d = %q, want %q", stepIndex, actual.TheString, expected)
		}
		stepIndex++
	}
	for inputIndex, expected := range resultsetAggregateMinMaxGroupByJoinSelectHavingMarketInputs {
		stepIndex, err = validateResultSetAggregateMinMaxGroupByJoinSelectHavingMarketStep(scenario, stepIndex, resultsetAggregateMinMaxGroupByJoinSelectHavingJoinCase, inputIndex, expected)
		if err != nil {
			return err
		}
	}
	if marker := scenario.Steps[stepIndex]; marker.Op != "case" || marker.Case != resultsetAggregateMinMaxGroupByJoinSelectHavingSelectCase {
		return fmt.Errorf("step %d must start case %q", stepIndex, resultsetAggregateMinMaxGroupByJoinSelectHavingSelectCase)
	}
	stepIndex++
	for inputIndex, expected := range resultsetAggregateMinMaxGroupByJoinSelectHavingSelectInputs {
		stepIndex, err = validateResultSetAggregateMinMaxGroupByJoinSelectHavingMarketStep(scenario, stepIndex, resultsetAggregateMinMaxGroupByJoinSelectHavingSelectCase, inputIndex, expected)
		if err != nil {
			return err
		}
	}
	if stepIndex != len(scenario.Steps) {
		return fmt.Errorf("%s scenario contains trailing steps", scenarioID)
	}
	return nil
}

func validateResultSetAggregateMinMaxGroupByJoinSelectHavingMarketStep(scenario compat.Scenario, stepIndex int, caseName string, inputIndex int, expected resultsetAggregateMinMaxGroupByExpectedInput) (int, error) {
	step := scenario.Steps[stepIndex]
	if step.Op != "send" || step.EventType != "SupportMarketDataBean" {
		return stepIndex, fmt.Errorf("%s case %q event %d step %d must send SupportMarketDataBean", scenario.ID, caseName, inputIndex, stepIndex)
	}
	actual, err := decodeResultSetAggregateMinMaxGroupByJoinSelectHavingMarket(step)
	if err != nil {
		return stepIndex, fmt.Errorf("%s case %q event %d: %w", scenario.ID, caseName, inputIndex, err)
	}
	if actual.Symbol != expected.symbol || actual.Price != 0 || !resultsetAggregateMinMaxGroupByVolumeEqual(actual.Volume, expected.volume) {
		return stepIndex, fmt.Errorf("%s case %q event %d = %#v, want symbol=%q price=0 volume=%v", scenario.ID, caseName, inputIndex, actual, expected.symbol, expected.volume)
	}
	return stepIndex + 1, nil
}

func decodeResultSetAggregateMinMaxGroupByJoinSelectHavingPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBeanString":
		return decodeResultSetAggregateMinMaxGroupByJoinSelectHavingString(step)
	case "SupportMarketDataBean":
		return decodeResultSetAggregateMinMaxGroupByJoinSelectHavingMarket(step)
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}

func decodeResultSetAggregateMinMaxGroupByJoinSelectHavingString(step compat.Step) (resultsetAggregateMinMaxGroupByStringBean, error) {
	if step.EventType != "SupportBeanString" {
		return resultsetAggregateMinMaxGroupByStringBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	payload, err := decodeResultSetAggregateMinMaxGroupByPayloadObject(step.Payload)
	if err != nil {
		return resultsetAggregateMinMaxGroupByStringBean{}, fmt.Errorf("payload must be an object: %w", err)
	}
	if len(payload) != 1 {
		return resultsetAggregateMinMaxGroupByStringBean{}, fmt.Errorf("payload must contain exactly theString")
	}
	raw, ok := payload["theString"]
	if !ok || string(bytes.TrimSpace(raw)) == "null" {
		return resultsetAggregateMinMaxGroupByStringBean{}, fmt.Errorf("payload theString must be a string")
	}
	var value resultsetAggregateMinMaxGroupByStringBean
	if err := json.Unmarshal(raw, &value.TheString); err != nil {
		return resultsetAggregateMinMaxGroupByStringBean{}, fmt.Errorf("payload theString must be a string")
	}
	return value, nil
}

func decodeResultSetAggregateMinMaxGroupByJoinSelectHavingMarket(step compat.Step) (resultsetAggregateMinMaxGroupByMarketData, error) {
	value, err := decodeResultSetAggregateMinMaxGroupByPayloadValue(step)
	if err != nil {
		return resultsetAggregateMinMaxGroupByMarketData{}, err
	}
	return value, nil
}
