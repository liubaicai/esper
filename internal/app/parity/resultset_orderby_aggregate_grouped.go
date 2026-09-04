package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultsetOrderbyAggregateGroupedID          = "resultset-orderby-aggregate-grouped"
	resultsetOrderbyAggregateGroupedDescription = "ResultSetOrderByAggregateGrouped ordinals 0-7: grouped aggregate order-by aliases, row-per-event switch, output-last batches, and grouped-join iterator snapshots."
	resultsetOrderbyAggregateGroupedJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOrderbyAggregateGroupedSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderByAggregateGrouped.java"
)

var (
	resultsetOrderbyAggregateGroupedJavaSources = []string{
		resultsetOrderbyAggregateGroupedSource,
	}
	resultsetOrderbyAggregateGroupedJavaRuntimeIDs = []string{
		"java-runtime-46cf1731d511ce733720",
		"java-runtime-99e2349823d5d0643cb7",
		"java-runtime-85ba512296ed6b671ece",
		"java-runtime-8295fe09d8727195fb80",
		"java-runtime-08747f9055632cef41c6",
		"java-runtime-12d689fc781af934b76d",
		"java-runtime-680e7b6016501fed42cb",
		"java-runtime-18eacf385e5ac00e8a80",
	}
	resultsetOrderbyAggregateGroupedJavaExecutions = []string{
		"ResultSetAliasesAggregationCompile",
		"ResultSetAliasesAggregationOM",
		"ResultSetAliases",
		"ResultSetGroupBySwitch",
		"ResultSetGroupBySwitchJoin",
		"ResultSetLastJoin",
		"ResultSetIterator",
		"ResultSetLast",
	}
	resultsetOrderbyAggregateGroupedJavaStaticIDs = []string{
		"java-a8a71b78b77eec10ee62",
		"java-015649f9c0449597e55a",
		"java-e002cbfd72133e0c59f0",
		"java-610fffded9e2415c805e",
		"java-b312c9571904401d3fdf",
		"java-663c2d183bbd4dc98fa2",
		"java-0de948d5133afc56e9fe",
		"java-726632af45ffc7989e27",
	}
	resultsetOrderbyAggregateGroupedCases = []string{
		"aliases-aggregation-compile",
		"aliases-aggregation-om",
		"aliases",
		"group-by-switch",
		"group-by-switch-join",
		"last-join",
		"iterator",
		"last",
	}
	resultsetOrderbyAggregateGroupedOrdinals = []int{0, 1, 2, 3, 4, 5, 6, 7}
	resultsetOrderbyAggregateGroupedEPLs     = []string{
		"@name('s0') select symbol, volume, sum(price) as mySum from SupportMarketDataBean#length(20) group by symbol output every 6 events order by sum(price), symbol",
		"select symbol, volume, sum(price) as mySum from SupportMarketDataBean#length(20) group by symbol output every 6 events order by sum(price), symbol",
		"@name('s0') select symbol, volume, sum(price) as mySum from SupportMarketDataBean#length(20) group by symbol output every 6 events order by mySum, symbol",
		"@name('s0') select symbol, sum(price) from SupportMarketDataBean#length(20) group by symbol output every 6 events order by sum(price), symbol, volume",
		"@name('s0') select symbol, sum(price) from SupportMarketDataBean#length(20) as one, SupportBeanString#length(100) as two where one.symbol = two.theString group by symbol output every 6 events order by sum(price), symbol, volume",
		"@name('s0') select symbol, volume, sum(price) from SupportMarketDataBean#length(20) as one, SupportBeanString#length(100) as two where one.symbol = two.theString group by symbol output last every 6 events order by sum(price)",
		"@name('s0') select symbol, theString, sum(price) as sumPrice from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString group by symbol order by symbol",
		"@name('s0') select symbol, volume, sum(price) from SupportMarketDataBean#length(20) group by symbol output last every 6 events order by sum(price)",
	}
)

type resultsetOrderbyAggregateGroupedMarket struct {
	Symbol string  `esper:"symbol"`
	Volume int64   `esper:"volume"`
	Price  float64 `esper:"price"`
}

type resultsetOrderbyAggregateGroupedString struct {
	TheString string `esper:"theString"`
}

var resultsetOrderbyAggregateGroupedMarketSymbols = []string{"IBM", "IBM", "CMU", "CMU", "CAT", "CAT"}
var resultsetOrderbyAggregateGroupedMarketVolumes = []int64{110, 120, 130, 140, 150, 160}
var resultsetOrderbyAggregateGroupedMarketPrices = []float64{3, 4, 1, 2, 5, 6}
var resultsetOrderbyAggregateGroupedJoinSeeds = []string{"CAT", "IBM", "CMU", "KGB", "DOG"}

var resultsetOrderbyAggregateGroupedLastSymbols = []string{
	"IBM", "IBM", "CMU", "CMU", "CAT", "CAT",
	"IBM", "IBM", "CMU", "CMU", "DOG", "DOG",
}
var resultsetOrderbyAggregateGroupedLastVolumes = []int64{
	101, 102, 103, 104, 105, 106,
	201, 202, 203, 204, 205, 206,
}
var resultsetOrderbyAggregateGroupedLastPrices = []float64{
	3, 4, 1, 2, 5, 6,
	3, 4, 5, 5, 0, 1,
}
var resultsetOrderbyAggregateGroupedIteratorFirstSymbols = []string{"CAT", "IBM", "CAT", "IBM"}
var resultsetOrderbyAggregateGroupedIteratorFirstVolumes = []int64{0, 0, 0, 0}
var resultsetOrderbyAggregateGroupedIteratorFirstPrices = []float64{50, 49, 15, 100}

func loadResultsetOrderbyAggregateGroupedScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOrderbyAggregateGroupedID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOrderbyAggregateGroupedID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOrderbyAggregateGroupedID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOrderbyAggregateGroupedID, err)
	}
	if err := requireResultsetOrderbyAggregateGroupedFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version     string `json:"version"`
		ID          string `json:"id"`
		Description string `json:"description"`
		JavaCommit  string `json:"javaCommit"`
		JavaSource  string `json:"javaSource"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOrderbyAggregateGroupedID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetOrderbyAggregateGroupedID ||
		metadata.Description != resultsetOrderbyAggregateGroupedDescription ||
		metadata.JavaCommit != resultsetOrderbyAggregateGroupedJavaCommit ||
		metadata.JavaSource != resultsetOrderbyAggregateGroupedSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOrderbyAggregateGroupedID)
	}
	if err := validateResultsetOrderbyAggregateGroupedStringArray(root["javaRuntimes"], resultsetOrderbyAggregateGroupedJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbyAggregateGroupedStringArray(root["javaNames"], resultsetOrderbyAggregateGroupedJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbyAggregateGroupedStringArray(root["javaStaticIds"], resultsetOrderbyAggregateGroupedJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbyAggregateGroupedStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetOrderbyAggregateGroupedCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly eight cases", resultsetOrderbyAggregateGroupedID)
	}
	observations := []string{"listener", "listener", "listener", "listener", "listener", "listener", "iterator", "listener"}
	iteratorSnapshots := []int{0, 0, 0, 0, 0, 0, 2, 0}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetOrderbyAggregateGroupedFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case              string `json:"case"`
			Ordinal           int    `json:"ordinal"`
			RuntimeID         string `json:"runtimeId"`
			ExecutionName     string `json:"executionName"`
			Observation       string `json:"observation"`
			IteratorSnapshots int    `json:"iteratorSnapshots"`
			EPL               string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if definition.Case != resultsetOrderbyAggregateGroupedCases[index] ||
			definition.Ordinal != resultsetOrderbyAggregateGroupedOrdinals[index] ||
			definition.RuntimeID != resultsetOrderbyAggregateGroupedJavaRuntimeIDs[index] ||
			definition.ExecutionName != resultsetOrderbyAggregateGroupedJavaExecutions[index] ||
			definition.Observation != observations[index] ||
			definition.IteratorSnapshots != iteratorSnapshots[index] ||
			definition.EPL != resultsetOrderbyAggregateGroupedEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetOrderbyAggregateGroupedID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 84 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly eighty-four steps", resultsetOrderbyAggregateGroupedID)
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		switch operation {
		case "case":
			if err := requireResultsetOrderbyAggregateGroupedFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireResultsetOrderbyAggregateGroupedFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeResultsetOrderbyAggregateGroupedPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		case "snapshot":
			if err := requireResultsetOrderbyAggregateGroupedFields(object, "op", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step compat.Step
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != "s0" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshot is not pinned", index)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateResultsetOrderbyAggregateGroupedScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetOrderbyAggregateGroupedScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetOrderbyAggregateGroupedID || len(scenario.Steps) != 84 {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetOrderbyAggregateGroupedID)
	}
	index := 0
	for caseIndex := 0; caseIndex < 4; caseIndex++ {
		if err := validateResultsetOrderbyAggregateGroupedCaseMarker(scenario.Steps[index], resultsetOrderbyAggregateGroupedCases[caseIndex]); err != nil {
			return fmt.Errorf("case %d marker: %w", caseIndex, err)
		}
		index++
		for eventIndex := range resultsetOrderbyAggregateGroupedMarketSymbols {
			if err := validateResultsetOrderbyAggregateGroupedMarketStep(scenario.Steps[index], eventIndex); err != nil {
				return fmt.Errorf("case %d market step %d: %w", caseIndex, eventIndex, err)
			}
			index++
		}
	}
	if err := validateResultsetOrderbyAggregateGroupedCaseMarker(scenario.Steps[index], resultsetOrderbyAggregateGroupedCases[4]); err != nil {
		return fmt.Errorf("join marker: %w", err)
	}
	index++
	for _, seed := range resultsetOrderbyAggregateGroupedJoinSeeds {
		if err := validateResultsetOrderbyAggregateGroupedStringStep(scenario.Steps[index], seed); err != nil {
			return fmt.Errorf("join seed %q: %w", seed, err)
		}
		index++
	}
	for eventIndex := range resultsetOrderbyAggregateGroupedMarketSymbols {
		if err := validateResultsetOrderbyAggregateGroupedMarketStep(scenario.Steps[index], eventIndex); err != nil {
			return fmt.Errorf("join market step %d: %w", eventIndex, err)
		}
		index++
	}
	if err := validateResultsetOrderbyAggregateGroupedCaseMarker(scenario.Steps[index], resultsetOrderbyAggregateGroupedCases[5]); err != nil {
		return fmt.Errorf("last-join marker: %w", err)
	}
	index++
	for _, seed := range resultsetOrderbyAggregateGroupedJoinSeeds {
		if err := validateResultsetOrderbyAggregateGroupedStringStep(scenario.Steps[index], seed); err != nil {
			return fmt.Errorf("last-join seed %q: %w", seed, err)
		}
		index++
	}
	for eventIndex := range resultsetOrderbyAggregateGroupedLastSymbols {
		if err := validateResultsetOrderbyAggregateGroupedLastMarketStep(scenario.Steps[index], eventIndex); err != nil {
			return fmt.Errorf("last-join market step %d: %w", eventIndex, err)
		}
		index++
	}
	if err := validateResultsetOrderbyAggregateGroupedCaseMarker(scenario.Steps[index], resultsetOrderbyAggregateGroupedCases[6]); err != nil {
		return fmt.Errorf("iterator marker: %w", err)
	}
	index++
	for _, seed := range resultsetOrderbyAggregateGroupedJoinSeeds {
		if err := validateResultsetOrderbyAggregateGroupedStringStep(scenario.Steps[index], seed); err != nil {
			return fmt.Errorf("iterator seed %q: %w", seed, err)
		}
		index++
	}
	for eventIndex := range resultsetOrderbyAggregateGroupedIteratorFirstSymbols {
		if err := validateResultsetOrderbyAggregateGroupedIteratorMarketStep(scenario.Steps[index], eventIndex); err != nil {
			return fmt.Errorf("iterator market step %d: %w", eventIndex, err)
		}
		index++
	}
	if err := validateResultsetOrderbyAggregateGroupedSnapshotStep(scenario.Steps[index]); err != nil {
		return fmt.Errorf("iterator first snapshot: %w", err)
	}
	index++
	if err := validateResultsetOrderbyAggregateGroupedIteratorMarketStep(scenario.Steps[index], 4); err != nil {
		return fmt.Errorf("iterator final market step: %w", err)
	}
	index++
	if err := validateResultsetOrderbyAggregateGroupedSnapshotStep(scenario.Steps[index]); err != nil {
		return fmt.Errorf("iterator second snapshot: %w", err)
	}
	index++
	if err := validateResultsetOrderbyAggregateGroupedCaseMarker(scenario.Steps[index], resultsetOrderbyAggregateGroupedCases[7]); err != nil {
		return fmt.Errorf("last marker: %w", err)
	}
	index++
	for eventIndex := range resultsetOrderbyAggregateGroupedLastSymbols {
		if err := validateResultsetOrderbyAggregateGroupedLastMarketStep(scenario.Steps[index], eventIndex); err != nil {
			return fmt.Errorf("last market step %d: %w", eventIndex, err)
		}
		index++
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetOrderbyAggregateGroupedID)
	}
	return nil
}

func validateResultsetOrderbyAggregateGroupedCaseMarker(step compat.Step, expected string) error {
	if step.Op != "case" || step.Case != expected {
		return fmt.Errorf("must start case %q", expected)
	}
	return nil
}

func validateResultsetOrderbyAggregateGroupedMarketStep(step compat.Step, index int) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportMarketDataBean" {
		return fmt.Errorf("must send SupportMarketDataBean")
	}
	market, err := decodeResultsetOrderbyAggregateGroupedMarket(step)
	if err != nil {
		return err
	}
	if market.Symbol != resultsetOrderbyAggregateGroupedMarketSymbols[index] ||
		market.Volume != resultsetOrderbyAggregateGroupedMarketVolumes[index] ||
		market.Price != resultsetOrderbyAggregateGroupedMarketPrices[index] {
		return fmt.Errorf("market payload is not pinned")
	}
	return nil
}

func validateResultsetOrderbyAggregateGroupedStringStep(step compat.Step, expected string) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportBeanString" {
		return fmt.Errorf("must send SupportBeanString")
	}
	value, err := decodeResultsetOrderbyAggregateGroupedString(step)
	if err != nil {
		return err
	}
	if value.TheString != expected {
		return fmt.Errorf("string payload is not pinned")
	}
	return nil
}

func validateResultsetOrderbyAggregateGroupedLastMarketStep(step compat.Step, index int) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportMarketDataBean" {
		return fmt.Errorf("must send SupportMarketDataBean")
	}
	market, err := decodeResultsetOrderbyAggregateGroupedMarket(step)
	if err != nil {
		return err
	}
	if market.Symbol != resultsetOrderbyAggregateGroupedLastSymbols[index] ||
		market.Volume != resultsetOrderbyAggregateGroupedLastVolumes[index] ||
		market.Price != resultsetOrderbyAggregateGroupedLastPrices[index] {
		return fmt.Errorf("market payload is not pinned")
	}
	return nil
}

func validateResultsetOrderbyAggregateGroupedIteratorMarketStep(step compat.Step, index int) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportMarketDataBean" {
		return fmt.Errorf("must send SupportMarketDataBean")
	}
	var symbol string
	var volume int64
	var price float64
	if index < len(resultsetOrderbyAggregateGroupedIteratorFirstSymbols) {
		symbol = resultsetOrderbyAggregateGroupedIteratorFirstSymbols[index]
		volume = resultsetOrderbyAggregateGroupedIteratorFirstVolumes[index]
		price = resultsetOrderbyAggregateGroupedIteratorFirstPrices[index]
	} else {
		symbol, volume, price = "KGB", 0, 75
	}
	market, err := decodeResultsetOrderbyAggregateGroupedMarket(step)
	if err != nil {
		return err
	}
	if market.Symbol != symbol || market.Volume != volume || market.Price != price {
		return fmt.Errorf("market payload is not pinned")
	}
	return nil
}

func validateResultsetOrderbyAggregateGroupedSnapshotStep(step compat.Step) error {
	if step.Op != "snapshot" || step.Case != "" || step.Statement != "s0" {
		return fmt.Errorf("must snapshot statement s0")
	}
	return nil
}

func runResultsetOrderbyAggregateGroupedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetOrderbyAggregateGroupedScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultsetOrderbyAggregateGroupedCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultsetOrderbyAggregateGroupedCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOrderbyAggregateGroupedID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOrderbyAggregateGroupedID, scenario.ID)
	}
	return trace, nil
}

func runResultsetOrderbyAggregateGroupedCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOrderbyAggregateGroupedMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	joinCases := map[string]bool{"group-by-switch-join": true, "last-join": true, "iterator": true}
	if joinCases[caseName] {
		if _, err := esper.RegisterStruct[resultsetOrderbyAggregateGroupedString](env, "SupportBeanString"); err != nil {
			return compat.Trace{}, err
		}
	}

	market := esper.From[resultsetOrderbyAggregateGroupedMarket](env, "SupportMarketDataBean").Window(esper.LengthWindow(20))
	symbol := esper.Field[resultsetOrderbyAggregateGroupedMarket, string]("symbol")
	volume := esper.Field[resultsetOrderbyAggregateGroupedMarket, int64]("volume")
	price := esper.Field[resultsetOrderbyAggregateGroupedMarket, float64]("price")
	sum := esper.Sum[float64](price)
	outputOptions := func(keys ...esper.SortKey) []esper.QueryOption {
		return []esper.QueryOption{
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputEvery(6)),
			esper.OrderBy(keys...),
		}
	}

	var query esper.Query
	switch caseName {
	case "aliases-aggregation-compile", "aliases-aggregation-om":
		query = market.GroupBy(symbol).Select(
			esper.Alias("symbol", symbol),
			esper.Alias("volume", volume),
			esper.Alias("mySum", sum),
		).Query(outputOptions(esper.Ascending(sum), esper.Ascending(symbol))...)
	case "aliases":
		query = market.GroupBy(symbol).Select(
			esper.Alias("symbol", symbol),
			esper.Alias("volume", volume),
			esper.Alias("mySum", sum),
		).Query(outputOptions(
			esper.Ascending(esper.ResultField[float64]("mySum")),
			esper.Ascending(symbol),
		)...)
	case "group-by-switch":
		query = market.GroupBy(symbol).Select(
			esper.Alias("symbol", symbol),
			esper.Alias("sum(price)", sum),
		).Query(outputOptions(
			esper.Ascending(sum),
			esper.Ascending(symbol),
			esper.Ascending(volume),
		)...)
	case "group-by-switch-join":
		seed := esper.From[resultsetOrderbyAggregateGroupedString](env, "SupportBeanString").Window(esper.LengthWindow(100))
		seedString := esper.Field[resultsetOrderbyAggregateGroupedString, string]("theString")
		joinSymbol := esper.JoinField[string](0, "symbol")
		joinVolume := esper.JoinField[int64](0, "volume")
		joinSum := esper.Sum[float64](esper.JoinField[float64](0, "price"))
		query = esper.Join(market, seed, esper.OnEqual(symbol, seedString)).
			GroupBy(joinSymbol).
			Select(
				esper.Alias("symbol", joinSymbol),
				esper.Alias("sum(price)", joinSum),
			).Query(outputOptions(
			esper.Ascending(joinSum),
			esper.Ascending(joinSymbol),
			esper.Ascending(joinVolume),
		)...)
	case "last":
		query = market.GroupBy(symbol).Select(
			esper.Alias("symbol", symbol),
			esper.Alias("volume", volume),
			esper.Alias("sum(price)", sum),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputLastEveryEvents(6)),
			esper.OrderBy(esper.Ascending(sum)),
		)
	case "last-join":
		seed := esper.From[resultsetOrderbyAggregateGroupedString](env, "SupportBeanString").Window(esper.LengthWindow(100))
		seedString := esper.Field[resultsetOrderbyAggregateGroupedString, string]("theString")
		joinSymbol := esper.JoinField[string](0, "symbol")
		joinVolume := esper.JoinField[int64](0, "volume")
		joinSum := esper.Sum[float64](esper.JoinField[float64](0, "price"))
		query = esper.Join(market, seed, esper.OnEqual(symbol, seedString)).
			GroupBy(joinSymbol).
			Select(
				esper.Alias("symbol", joinSymbol),
				esper.Alias("volume", joinVolume),
				esper.Alias("sum(price)", joinSum),
			).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputLastEveryEvents(6)),
			esper.OrderBy(esper.Ascending(joinSum)),
		)
	case "iterator":
		iteratorMarket := esper.From[resultsetOrderbyAggregateGroupedMarket](env, "SupportMarketDataBean").Window(esper.LengthWindow(10))
		seed := esper.From[resultsetOrderbyAggregateGroupedString](env, "SupportBeanString").Window(esper.LengthWindow(100))
		seedString := esper.Field[resultsetOrderbyAggregateGroupedString, string]("theString")
		joinSymbol := esper.JoinField[string](0, "symbol")
		joinTheString := esper.JoinField[string](1, "theString")
		joinSum := esper.Sum[float64](esper.JoinField[float64](0, "price"))
		query = esper.Join(iteratorMarket, seed, esper.OnEqual(symbol, seedString)).
			GroupBy(joinSymbol).
			Select(
				esper.Alias("symbol", joinSymbol),
				esper.Alias("theString", joinTheString),
				esper.Alias("sumPrice", joinSum),
			).Query(
			esper.StatementName("s0"),
			esper.OrderBy(esper.Ascending(joinSymbol)),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported %s case %q", resultsetOrderbyAggregateGroupedID, caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithRuntimeURI(resultsetOrderbyAggregateGroupedRuntimeID(caseName)))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("%s case %q deployed %d statements", resultsetOrderbyAggregateGroupedID, caseName, len(statements))
	}
	statement := statements[0]
	if caseName == "iterator" {
		return runResultsetOrderbyAggregateGroupedIteratorReplay(ctx, engine, statement, scenario)
	}
	return compat.ReplayWithStatements(ctx, engine, statement, scenario,
		decodeResultsetOrderbyAggregateGroupedPayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetOrderbyAggregateGroupedID, name)
			}
			return statement, nil
		})
}

// runResultsetOrderbyAggregateGroupedIteratorReplay mirrors the Java iterator
// execution: the statement listener stays attached but records nothing, and
// only the two pinned statement-iterator snapshots are observed.
func runResultsetOrderbyAggregateGroupedIteratorReplay(ctx context.Context, engine *esper.Engine, statement *esper.Statement, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	snapshots := 0
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "send":
			payload, err := decodeResultsetOrderbyAggregateGroupedPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "snapshot":
			if step.Statement != statement.Name() {
				return trace, fmt.Errorf("%s iterator snapshot metadata is not pinned", resultsetOrderbyAggregateGroupedID)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return trace, err
			}
			rows := compat.NormalizeResults(result.Batch.New)
			if rows == nil {
				rows = []compat.ResultRecord{}
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      "iterator",
				Operation: "snapshot",
				Statement: statement.Name(),
				Sequence:  0,
				Time:      engine.Now().UTC().Format(time.RFC3339Nano),
				New:       rows,
			})
			snapshots++
		default:
			return trace, fmt.Errorf("unsupported %s iterator step op %q", resultsetOrderbyAggregateGroupedID, step.Op)
		}
	}
	if snapshots != 2 {
		return trace, fmt.Errorf("%s iterator case produced %d snapshots, want two", resultsetOrderbyAggregateGroupedID, snapshots)
	}
	return trace, nil
}

func decodeResultsetOrderbyAggregateGroupedPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportMarketDataBean":
		return decodeResultsetOrderbyAggregateGroupedMarket(step)
	case "SupportBeanString":
		return decodeResultsetOrderbyAggregateGroupedString(step)
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOrderbyAggregateGroupedID, step.EventType)
	}
}

func decodeResultsetOrderbyAggregateGroupedMarket(step compat.Step) (resultsetOrderbyAggregateGroupedMarket, error) {
	if step.EventType != "SupportMarketDataBean" {
		return resultsetOrderbyAggregateGroupedMarket{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOrderbyAggregateGroupedMarket{}, err
	}
	if err := requireResultsetOrderbyAggregateGroupedFields(fields, "symbol", "volume", "price"); err != nil {
		return resultsetOrderbyAggregateGroupedMarket{}, err
	}
	symbol, err := decodeResultsetOrderbyAggregateGroupedStringValue(fields["symbol"], "symbol")
	if err != nil {
		return resultsetOrderbyAggregateGroupedMarket{}, err
	}
	volume, err := decodeResultsetOrderbyAggregateGroupedInteger(fields["volume"], "volume")
	if err != nil {
		return resultsetOrderbyAggregateGroupedMarket{}, err
	}
	price, err := decodeResultsetOrderbyAggregateGroupedFloat(fields["price"], "price")
	if err != nil {
		return resultsetOrderbyAggregateGroupedMarket{}, err
	}
	return resultsetOrderbyAggregateGroupedMarket{Symbol: symbol, Volume: volume, Price: price}, nil
}

func decodeResultsetOrderbyAggregateGroupedString(step compat.Step) (resultsetOrderbyAggregateGroupedString, error) {
	if step.EventType != "SupportBeanString" {
		return resultsetOrderbyAggregateGroupedString{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOrderbyAggregateGroupedString{}, err
	}
	if err := requireResultsetOrderbyAggregateGroupedFields(fields, "theString"); err != nil {
		return resultsetOrderbyAggregateGroupedString{}, err
	}
	value, err := decodeResultsetOrderbyAggregateGroupedStringValue(fields["theString"], "theString")
	if err != nil {
		return resultsetOrderbyAggregateGroupedString{}, err
	}
	return resultsetOrderbyAggregateGroupedString{TheString: value}, nil
}

func requireResultsetOrderbyAggregateGroupedFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("JSON object has unexpected fields")
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

func decodeResultsetOrderbyAggregateGroupedStringValue(raw json.RawMessage, name string) (string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must be a string", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func decodeResultsetOrderbyAggregateGroupedInteger(raw json.RawMessage, name string) (int64, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	number, ok := token.(json.Number)
	if !ok || !resultsetOrderbyAggregateGroupedIntegerSyntax(string(number)) {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	value, err := strconv.ParseInt(string(number), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	return value, nil
}

func decodeResultsetOrderbyAggregateGroupedFloat(raw json.RawMessage, name string) (float64, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s must be a JSON number", name)
	}
	number, ok := token.(json.Number)
	if !ok {
		return 0, fmt.Errorf("%s must be a JSON number", name)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return 0, fmt.Errorf("%s must be a JSON number", name)
	}
	value, err := strconv.ParseFloat(string(number), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("%s must be a JSON number", name)
	}
	return value, nil
}

func resultsetOrderbyAggregateGroupedIntegerSyntax(text string) bool {
	if text == "0" {
		return true
	}
	if text == "" {
		return false
	}
	if text[0] == '-' {
		text = text[1:]
	}
	if text == "" || (len(text) > 1 && text[0] == '0') {
		return false
	}
	for _, character := range text {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validateResultsetOrderbyAggregateGroupedStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index := range expected {
		if values[index] != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}

func resultsetOrderbyAggregateGroupedRuntimeID(caseName string) string {
	for index, name := range resultsetOrderbyAggregateGroupedCases {
		if name == caseName {
			return resultsetOrderbyAggregateGroupedJavaRuntimeIDs[index]
		}
	}
	return "resultset-orderby-aggregate-grouped-unknown"
}
