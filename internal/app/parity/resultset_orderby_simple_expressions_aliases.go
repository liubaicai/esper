package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultsetOrderbySimpleExpressionsAliasesID          = "resultset-orderby-simple-expressions-aliases"
	resultsetOrderbySimpleExpressionsAliasesDescription = "ResultSetOrderBySimple ordinals 5-9: expression, alias, join, and multi-key order-by over length windows with output every 6 events; ordinal 9 variant 5 drops the output clause so each of its seven sends delivers a one-row batch."
	resultsetOrderbySimpleExpressionsAliasesJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOrderbySimpleExpressionsAliasesSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderBySimple.java"
)

var (
	resultsetOrderbySimpleExpressionsAliasesJavaSources = []string{
		resultsetOrderbySimpleExpressionsAliasesSource,
	}
	resultsetOrderbySimpleExpressionsAliasesJavaRuntimeIDs = []string{
		"java-runtime-0a7a2ff59087a96fe0e1",
		"java-runtime-fc69777a3bedc3ad6c52",
		"java-runtime-b0f39fb300a33fa7ddbf",
		"java-runtime-92e69b0657b10a656fdc",
		"java-runtime-178a1d0412982958a3cc",
	}
	resultsetOrderbySimpleExpressionsAliasesJavaExecutions = []string{
		"ResultSetExpressions",
		"ResultSetAliasesSimple",
		"ResultSetExpressionsJoin",
		"ResultSetMultipleKeys",
		"ResultSetAliases",
	}
	resultsetOrderbySimpleExpressionsAliasesJavaStaticIDs = []string{
		"java-042c1b302e7183feb8a6",
		"java-042c1b302e7183feb8a6",
		"java-042c1b302e7183feb8a6",
		"java-042c1b302e7183feb8a6",
		"java-042c1b302e7183feb8a6",
	}
	resultsetOrderbySimpleExpressionsAliasesCases = []string{
		"expressions-v1",
		"expressions-v2",
		"expressions-v3",
		"expressions-v4",
		"aliases-simple-v1",
		"aliases-simple-v2",
		"aliases-simple-v3",
		"aliases-simple-v4",
		"expressions-join-v1",
		"expressions-join-v2",
		"expressions-join-v3",
		"expressions-join-v4",
		"multiple-keys-v1",
		"multiple-keys-v2",
		"multiple-keys-v3",
		"aliases-v1",
		"aliases-v2",
		"aliases-v3",
		"aliases-v4",
		"aliases-v5",
	}
	resultsetOrderbySimpleExpressionsAliasesOrdinals = []int{
		5, 5, 5, 5,
		6, 6, 6, 6,
		7, 7, 7, 7,
		8, 8, 8,
		9, 9, 9, 9, 9,
	}
	resultsetOrderbySimpleExpressionsAliasesEPLs = []string{
		"@name('s0') select symbol from SupportMarketDataBean#length(10) output every 6 events order by (price * 6) + 5",
		"@name('s0') select symbol, price from SupportMarketDataBean#length(10) output every 6 events order by (price * 6) + 5, price",
		"@name('s0') select symbol, 1+volume*23 from SupportMarketDataBean#length(10) output every 6 events order by (price * 6) + 5, price, volume",
		"@name('s0') select symbol from SupportMarketDataBean#length(10) output every 6 events order by volume*price, symbol",
		"@name('s0') select symbol as mySymbol from SupportMarketDataBean#length(5) output every 6 events order by mySymbol",
		"@name('s0') select symbol as mySymbol, price as myPrice from SupportMarketDataBean#length(5) output every 6 events order by myPrice",
		"@name('s0') select symbol, price as myPrice from SupportMarketDataBean#length(10) output every 6 events order by (myPrice * 6) + 5, price",
		"@name('s0') select symbol, 1+volume*23 as myVol from SupportMarketDataBean#length(10) output every 6 events order by (price * 6) + 5, price, myVol",
		"@name('s0') select symbol from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by (price * 6) + 5",
		"@name('s0') select symbol, price from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by (price * 6) + 5, price",
		"@name('s0') select symbol, 1+volume*23 from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by (price * 6) + 5, price, volume",
		"@name('s0') select symbol from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by volume*price, symbol",
		"@name('s0') select symbol from SupportMarketDataBean#length(10) output every 6 events order by symbol, price",
		"@name('s0') select symbol from SupportMarketDataBean#length(10) output every 6 events order by price, symbol, volume",
		"@name('s0') select symbol, volume*2 from SupportMarketDataBean#length(10) output every 6 events order by price, volume",
		"@name('s0') select symbol as mySymbol from SupportMarketDataBean#length(5) output every 6 events order by mySymbol",
		"@name('s0') select symbol as mySymbol, price as myPrice from SupportMarketDataBean#length(5) output every 6 events order by myPrice",
		"@name('s0') select symbol, price as myPrice from SupportMarketDataBean#length(10) output every 6 events order by (myPrice * 6) + 5, price",
		"@name('s0') select symbol, 1+volume*23 as myVol from SupportMarketDataBean#length(10) output every 6 events order by (price * 6) + 5, price, myVol",
		"@name('s0') select symbol as mySymbol from SupportMarketDataBean#length(5) order by price, mySymbol",
	}
	resultsetOrderbySimpleExpressionsAliasesObservations = []string{
		"listener; ordinal 5 variant 1 orders the six-row output batch by (price * 6) + 5",
		"listener; ordinal 5 variant 2 selects symbol, price and orders the six-row output batch by (price * 6) + 5, price",
		"listener; ordinal 5 variant 3 selects symbol and the computed column 1+volume*23, ordering the six-row output batch by (price * 6) + 5, price, volume",
		"listener; ordinal 5 variant 4 orders the six-row output batch by volume*price, symbol; the constant zero volume makes the symbol key decide",
		"listener; ordinal 6 variant 1 selects symbol as mySymbol over a length(5) window and orders the six-row output batch by the mySymbol alias",
		"listener; ordinal 6 variant 2 selects symbol as mySymbol, price as myPrice and orders the six-row output batch by the myPrice alias",
		"listener; ordinal 6 variant 3 selects symbol, price as myPrice and orders the six-row output batch by (myPrice * 6) + 5, price",
		"listener; ordinal 6 variant 4 selects symbol, 1+volume*23 as myVol and orders the six-row output batch by (price * 6) + 5, price, myVol",
		"listener; ordinal 7 variant 1 orders the six-row join output batch by (price * 6) + 5; the price-6 tie keeps join-generation order so CAT precedes IBM",
		"listener; ordinal 7 variant 2 selects symbol, price and orders the six-row join output batch by (price * 6) + 5, price",
		"listener; ordinal 7 variant 3 selects symbol and the computed column 1+volume*23, ordering the six-row join output batch by (price * 6) + 5, price, volume",
		"listener; ordinal 7 variant 4 orders the six-row join output batch by volume*price, symbol; the constant zero volume makes the symbol key decide",
		"listener; ordinal 8 variant 1 orders the six-row output batch by symbol, price",
		"listener; ordinal 8 variant 2 orders the six-row output batch by price, symbol, volume so the price-6 tie resolves CAT, CAT, IBM",
		"listener; ordinal 8 variant 3 selects symbol, volume*2 and orders the six-row output batch by price, volume; the constant zero volume keeps insertion order at the price-6 tie",
		"listener; ordinal 9 variant 1 replays the ordinal 6 variant 1 statement, ordering the six-row output batch by the mySymbol alias",
		"listener; ordinal 9 variant 2 replays the ordinal 6 variant 2 statement, ordering the six-row output batch by the myPrice alias",
		"listener; ordinal 9 variant 3 replays the ordinal 6 variant 3 statement, ordering the six-row output batch by (myPrice * 6) + 5, price",
		"listener; ordinal 9 variant 4 replays the ordinal 6 variant 4 statement, ordering the six-row output batch by (price * 6) + 5, price, myVol",
		"listener; ordinal 9 variant 5 has no output clause so each of the seven sends delivers its own one-row batch ordered by price, mySymbol",
	}
)

type resultsetOrderbySimpleExpressionsAliasesBean struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
}

type resultsetOrderbySimpleExpressionsAliasesString struct {
	TheString string `esper:"theString"`
}

// resultsetOrderbySimpleExpressionsAliasesEvents pins the shared six-event
// send sequence every case replays (Java sendEvent(symbol, price) with
// volume=0L): IBM@2, KGB@1, CMU@3, IBM@6, CAT@6, CAT@5.
var resultsetOrderbySimpleExpressionsAliasesEvents = []resultsetOrderbySimpleExpressionsAliasesBean{
	{Symbol: "IBM", Price: 2, Volume: 0},
	{Symbol: "KGB", Price: 1, Volume: 0},
	{Symbol: "CMU", Price: 3, Volume: 0},
	{Symbol: "IBM", Price: 6, Volume: 0},
	{Symbol: "CAT", Price: 6, Volume: 0},
	{Symbol: "CAT", Price: 5, Volume: 0},
}

// resultsetOrderbySimpleExpressionsAliasesJoinStrings pins the
// sendJoinEvents SupportBeanString sequence the ordinal 7 join cases send
// after the six market events: CAT, IBM, CMU, KGB, DOG.
var resultsetOrderbySimpleExpressionsAliasesJoinStrings = []string{"CAT", "IBM", "CMU", "KGB", "DOG"}

// resultsetOrderbySimpleExpressionsAliasesExtraEvent pins the FOX@10 send
// ordinal 9 variant 5 performs after the shared sequence.
var resultsetOrderbySimpleExpressionsAliasesExtraEvent = resultsetOrderbySimpleExpressionsAliasesBean{Symbol: "FOX", Price: 10, Volume: 0}

func loadResultsetOrderbySimpleExpressionsAliasesScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOrderbySimpleExpressionsAliasesID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOrderbySimpleExpressionsAliasesID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOrderbySimpleExpressionsAliasesID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOrderbySimpleExpressionsAliasesID, err)
	}
	if err := requireResultsetOrderbySimpleExpressionsAliasesFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOrderbySimpleExpressionsAliasesID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetOrderbySimpleExpressionsAliasesID ||
		metadata.Description != resultsetOrderbySimpleExpressionsAliasesDescription ||
		metadata.JavaCommit != resultsetOrderbySimpleExpressionsAliasesJavaCommit ||
		metadata.JavaSource != resultsetOrderbySimpleExpressionsAliasesSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOrderbySimpleExpressionsAliasesID)
	}
	if err := validateResultsetOrderbySimpleExpressionsAliasesStringArray(root["javaRuntimes"], resultsetOrderbySimpleExpressionsAliasesJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbySimpleExpressionsAliasesStringArray(root["javaNames"], resultsetOrderbySimpleExpressionsAliasesJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbySimpleExpressionsAliasesStringArray(root["javaStaticIds"], resultsetOrderbySimpleExpressionsAliasesJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbySimpleExpressionsAliasesStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetOrderbySimpleExpressionsAliasesCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly twenty cases", resultsetOrderbySimpleExpressionsAliasesID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetOrderbySimpleExpressionsAliasesFields(object,
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
		if definition.Case != resultsetOrderbySimpleExpressionsAliasesCases[index] ||
			definition.Ordinal != resultsetOrderbySimpleExpressionsAliasesOrdinals[index] ||
			definition.RuntimeID != resultsetOrderbySimpleExpressionsAliasesRuntimeID(definition.Case) ||
			definition.ExecutionName != resultsetOrderbySimpleExpressionsAliasesExecutionName(definition.Case) ||
			definition.Observation != resultsetOrderbySimpleExpressionsAliasesObservations[index] ||
			definition.IteratorSnapshots != 0 ||
			definition.EPL != resultsetOrderbySimpleExpressionsAliasesEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetOrderbySimpleExpressionsAliasesID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 161 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 161 steps", resultsetOrderbySimpleExpressionsAliasesID)
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
			if err := requireResultsetOrderbySimpleExpressionsAliasesFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireResultsetOrderbySimpleExpressionsAliasesFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeResultsetOrderbySimpleExpressionsAliasesPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateResultsetOrderbySimpleExpressionsAliasesScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetOrderbySimpleExpressionsAliasesScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetOrderbySimpleExpressionsAliasesID || len(scenario.Steps) != 161 {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetOrderbySimpleExpressionsAliasesID)
	}
	index := 0
	for caseIndex, caseName := range resultsetOrderbySimpleExpressionsAliasesCases {
		if err := validateResultsetOrderbySimpleExpressionsAliasesCaseMarker(scenario.Steps[index], caseName); err != nil {
			return fmt.Errorf("case %d marker: %w", caseIndex, err)
		}
		index++
		for _, expected := range resultsetOrderbySimpleExpressionsAliasesEvents {
			if err := validateResultsetOrderbySimpleExpressionsAliasesBeanStep(scenario.Steps[index], expected); err != nil {
				return fmt.Errorf("case %d bean step: %w", caseIndex, err)
			}
			index++
		}
		if resultsetOrderbySimpleExpressionsAliasesJoinCase(caseName) {
			for _, expected := range resultsetOrderbySimpleExpressionsAliasesJoinStrings {
				if err := validateResultsetOrderbySimpleExpressionsAliasesStringStep(scenario.Steps[index], expected); err != nil {
					return fmt.Errorf("case %d string step: %w", caseIndex, err)
				}
				index++
			}
		}
		if caseName == "aliases-v5" {
			if err := validateResultsetOrderbySimpleExpressionsAliasesBeanStep(scenario.Steps[index], resultsetOrderbySimpleExpressionsAliasesExtraEvent); err != nil {
				return fmt.Errorf("case %d extra bean step: %w", caseIndex, err)
			}
			index++
		}
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetOrderbySimpleExpressionsAliasesID)
	}
	return nil
}

func validateResultsetOrderbySimpleExpressionsAliasesCaseMarker(step compat.Step, expected string) error {
	if step.Op != "case" || step.Case != expected {
		return fmt.Errorf("must start case %q", expected)
	}
	return nil
}

func validateResultsetOrderbySimpleExpressionsAliasesBeanStep(step compat.Step, expected resultsetOrderbySimpleExpressionsAliasesBean) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportMarketDataBean" {
		return fmt.Errorf("must send SupportMarketDataBean")
	}
	bean, err := decodeResultsetOrderbySimpleExpressionsAliasesBean(step)
	if err != nil {
		return err
	}
	if bean.Symbol != expected.Symbol || bean.Price != expected.Price || bean.Volume != expected.Volume {
		return fmt.Errorf("bean payload is not pinned")
	}
	return nil
}

func validateResultsetOrderbySimpleExpressionsAliasesStringStep(step compat.Step, expected string) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportBeanString" {
		return fmt.Errorf("must send SupportBeanString")
	}
	bean, err := decodeResultsetOrderbySimpleExpressionsAliasesString(step)
	if err != nil {
		return err
	}
	if bean.TheString != expected {
		return fmt.Errorf("string payload is not pinned")
	}
	return nil
}

func runResultsetOrderbySimpleExpressionsAliasesScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetOrderbySimpleExpressionsAliasesScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultsetOrderbySimpleExpressionsAliasesCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultsetOrderbySimpleExpressionsAliasesCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOrderbySimpleExpressionsAliasesID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOrderbySimpleExpressionsAliasesID, scenario.ID)
	}
	return trace, nil
}

func runResultsetOrderbySimpleExpressionsAliasesCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOrderbySimpleExpressionsAliasesBean](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetOrderbySimpleExpressionsAliasesString](env, "SupportBeanString"); err != nil {
		return compat.Trace{}, err
	}

	symbol := esper.Field[resultsetOrderbySimpleExpressionsAliasesBean, string]("symbol")
	price := esper.Field[resultsetOrderbySimpleExpressionsAliasesBean, float64]("price")
	volume := esper.Field[resultsetOrderbySimpleExpressionsAliasesBean, int64]("volume")
	theString := esper.Field[resultsetOrderbySimpleExpressionsAliasesString, string]("theString")
	windowed5 := esper.From[resultsetOrderbySimpleExpressionsAliasesBean](env, "SupportMarketDataBean").Window(esper.LengthWindow(5))
	windowed10 := esper.From[resultsetOrderbySimpleExpressionsAliasesBean](env, "SupportMarketDataBean").Window(esper.LengthWindow(10))

	priceTimesSixPlusFive := esper.AddOf[float64](esper.MultiplyOf[float64](price, esper.Literal(6.0)), esper.Literal(5.0))
	volumeTimesPrice := esper.MultiplyOf[float64](volume, price)
	onePlusVolumeTimes23 := esper.AddOf[int64](esper.Literal(int64(1)), esper.MultiplyOf[int64](volume, esper.Literal(int64(23))))
	volumeTimesTwo := esper.MultiplyOf[int64](volume, esper.Literal(int64(2)))

	mySymbol := esper.ResultField[string]("mySymbol")
	myPrice := esper.ResultField[float64]("myPrice")
	myVol := esper.ResultField[int64]("myVol")
	myPriceTimesSixPlusFive := esper.AddOf[float64](esper.MultiplyOf[float64](myPrice, esper.Literal(6.0)), esper.Literal(5.0))

	joinSymbol := esper.JoinField[string](0, "symbol")
	joinPrice := esper.JoinField[float64](0, "price")
	joinVolume := esper.JoinField[int64](0, "volume")
	joinPriceTimesSixPlusFive := esper.AddOf[float64](esper.MultiplyOf[float64](joinPrice, esper.Literal(6.0)), esper.Literal(5.0))
	joinVolumeTimesPrice := esper.MultiplyOf[float64](joinVolume, joinPrice)
	joinOnePlusVolumeTimes23 := esper.AddOf[int64](esper.Literal(int64(1)), esper.MultiplyOf[int64](joinVolume, esper.Literal(int64(23))))

	market := esper.From[resultsetOrderbySimpleExpressionsAliasesBean](env, "SupportMarketDataBean").Window(esper.LengthWindow(10))
	seed := esper.From[resultsetOrderbySimpleExpressionsAliasesString](env, "SupportBeanString").Window(esper.LengthWindow(100))
	joined := esper.Join(market, seed, esper.OnEqual(symbol, theString))

	var query esper.Query
	switch caseName {
	case "expressions-v1":
		query = esper.Select(windowed10,
			esper.Alias("symbol", symbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(priceTimesSixPlusFive)),
		)
	case "expressions-v2":
		query = esper.Select(windowed10,
			esper.Alias("symbol", symbol),
			esper.Alias("price", price),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(priceTimesSixPlusFive),
				esper.Ascending(price),
			),
		)
	case "expressions-v3":
		query = esper.Select(windowed10,
			esper.Alias("symbol", symbol),
			esper.Alias("1+volume*23", onePlusVolumeTimes23),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(priceTimesSixPlusFive),
				esper.Ascending(price),
				esper.Ascending(volume),
			),
		)
	case "expressions-v4":
		query = esper.Select(windowed10,
			esper.Alias("symbol", symbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(volumeTimesPrice),
				esper.Ascending(symbol),
			),
		)
	case "aliases-simple-v1", "aliases-v1":
		query = esper.Select(windowed5,
			esper.Alias("mySymbol", symbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(mySymbol)),
		)
	case "aliases-simple-v2", "aliases-v2":
		query = esper.Select(windowed5,
			esper.Alias("mySymbol", symbol),
			esper.Alias("myPrice", price),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(myPrice)),
		)
	case "aliases-simple-v3", "aliases-v3":
		query = esper.Select(windowed10,
			esper.Alias("symbol", symbol),
			esper.Alias("myPrice", price),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(myPriceTimesSixPlusFive),
				esper.Ascending(price),
			),
		)
	case "aliases-simple-v4", "aliases-v4":
		query = esper.Select(windowed10,
			esper.Alias("symbol", symbol),
			esper.Alias("myVol", onePlusVolumeTimes23),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(priceTimesSixPlusFive),
				esper.Ascending(price),
				esper.Ascending(myVol),
			),
		)
	case "expressions-join-v1":
		query = joined.Select(
			esper.SelectFrom(0, "symbol", joinSymbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(joinPriceTimesSixPlusFive)),
		)
	case "expressions-join-v2":
		query = joined.Select(
			esper.SelectFrom(0, "symbol", joinSymbol),
			esper.SelectFrom(0, "price", joinPrice),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(joinPriceTimesSixPlusFive),
				esper.Ascending(joinPrice),
			),
		)
	case "expressions-join-v3":
		query = joined.Select(
			esper.SelectFrom(0, "symbol", joinSymbol),
			esper.SelectFrom(0, "1+volume*23", joinOnePlusVolumeTimes23),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(joinPriceTimesSixPlusFive),
				esper.Ascending(joinPrice),
				esper.Ascending(joinVolume),
			),
		)
	case "expressions-join-v4":
		query = joined.Select(
			esper.SelectFrom(0, "symbol", joinSymbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(joinVolumeTimesPrice),
				esper.Ascending(joinSymbol),
			),
		)
	case "multiple-keys-v1":
		query = esper.Select(windowed10,
			esper.Alias("symbol", symbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(symbol),
				esper.Ascending(price),
			),
		)
	case "multiple-keys-v2":
		query = esper.Select(windowed10,
			esper.Alias("symbol", symbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(price),
				esper.Ascending(symbol),
				esper.Ascending(volume),
			),
		)
	case "multiple-keys-v3":
		query = esper.Select(windowed10,
			esper.Alias("symbol", symbol),
			esper.Alias("volume*2", volumeTimesTwo),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(price),
				esper.Ascending(volume),
			),
		)
	case "aliases-v5":
		query = esper.Select(windowed5,
			esper.Alias("mySymbol", symbol),
		).Query(
			esper.StatementName("s0"),
			esper.OrderBy(
				esper.Ascending(price),
				esper.Ascending(mySymbol),
			),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported %s case %q", resultsetOrderbySimpleExpressionsAliasesID, caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithRuntimeURI(resultsetOrderbySimpleExpressionsAliasesRuntimeID(caseName)))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("%s case %q deployed %d statements", resultsetOrderbySimpleExpressionsAliasesID, caseName, len(statements))
	}
	statement := statements[0]
	return compat.ReplayWithStatements(ctx, engine, statement, scenario,
		decodeResultsetOrderbySimpleExpressionsAliasesPayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetOrderbySimpleExpressionsAliasesID, name)
			}
			return statement, nil
		})
}

func decodeResultsetOrderbySimpleExpressionsAliasesPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportMarketDataBean":
		return decodeResultsetOrderbySimpleExpressionsAliasesBean(step)
	case "SupportBeanString":
		return decodeResultsetOrderbySimpleExpressionsAliasesString(step)
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOrderbySimpleExpressionsAliasesID, step.EventType)
	}
}

func decodeResultsetOrderbySimpleExpressionsAliasesBean(step compat.Step) (resultsetOrderbySimpleExpressionsAliasesBean, error) {
	if step.EventType != "SupportMarketDataBean" {
		return resultsetOrderbySimpleExpressionsAliasesBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOrderbySimpleExpressionsAliasesBean{}, err
	}
	if err := requireResultsetOrderbySimpleExpressionsAliasesFields(fields, "symbol", "volume", "price"); err != nil {
		return resultsetOrderbySimpleExpressionsAliasesBean{}, err
	}
	symbol, err := decodeResultsetOrderbySimpleExpressionsAliasesStringValue(fields["symbol"], "symbol")
	if err != nil {
		return resultsetOrderbySimpleExpressionsAliasesBean{}, err
	}
	volume, err := decodeResultsetOrderbySimpleExpressionsAliasesInteger(fields["volume"], "volume")
	if err != nil {
		return resultsetOrderbySimpleExpressionsAliasesBean{}, err
	}
	price, err := decodeResultsetOrderbySimpleExpressionsAliasesFloat(fields["price"], "price")
	if err != nil {
		return resultsetOrderbySimpleExpressionsAliasesBean{}, err
	}
	return resultsetOrderbySimpleExpressionsAliasesBean{Symbol: symbol, Price: price, Volume: volume}, nil
}

func decodeResultsetOrderbySimpleExpressionsAliasesString(step compat.Step) (resultsetOrderbySimpleExpressionsAliasesString, error) {
	if step.EventType != "SupportBeanString" {
		return resultsetOrderbySimpleExpressionsAliasesString{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOrderbySimpleExpressionsAliasesString{}, err
	}
	if err := requireResultsetOrderbySimpleExpressionsAliasesFields(fields, "theString"); err != nil {
		return resultsetOrderbySimpleExpressionsAliasesString{}, err
	}
	theString, err := decodeResultsetOrderbySimpleExpressionsAliasesStringValue(fields["theString"], "theString")
	if err != nil {
		return resultsetOrderbySimpleExpressionsAliasesString{}, err
	}
	return resultsetOrderbySimpleExpressionsAliasesString{TheString: theString}, nil
}

func requireResultsetOrderbySimpleExpressionsAliasesFields(object map[string]json.RawMessage, names ...string) error {
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

func decodeResultsetOrderbySimpleExpressionsAliasesStringValue(raw json.RawMessage, name string) (string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must be a string", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func decodeResultsetOrderbySimpleExpressionsAliasesInteger(raw json.RawMessage, name string) (int64, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	number, ok := token.(json.Number)
	if !ok || !resultsetOrderbySimpleExpressionsAliasesIntegerSyntax(string(number)) {
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

func decodeResultsetOrderbySimpleExpressionsAliasesFloat(raw json.RawMessage, name string) (float64, error) {
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

func resultsetOrderbySimpleExpressionsAliasesIntegerSyntax(text string) bool {
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

func validateResultsetOrderbySimpleExpressionsAliasesStringArray(raw json.RawMessage, expected []string, name string) error {
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

func resultsetOrderbySimpleExpressionsAliasesOrdinal(caseName string) int {
	for index, name := range resultsetOrderbySimpleExpressionsAliasesCases {
		if name == caseName {
			return resultsetOrderbySimpleExpressionsAliasesOrdinals[index]
		}
	}
	return -1
}

func resultsetOrderbySimpleExpressionsAliasesJoinCase(caseName string) bool {
	return resultsetOrderbySimpleExpressionsAliasesOrdinal(caseName) == 7
}

func resultsetOrderbySimpleExpressionsAliasesRuntimeID(caseName string) string {
	ordinal := resultsetOrderbySimpleExpressionsAliasesOrdinal(caseName)
	if ordinal < 5 || ordinal > 9 {
		return "resultset-orderby-simple-expressions-aliases-unknown"
	}
	return resultsetOrderbySimpleExpressionsAliasesJavaRuntimeIDs[ordinal-5]
}

func resultsetOrderbySimpleExpressionsAliasesExecutionName(caseName string) string {
	ordinal := resultsetOrderbySimpleExpressionsAliasesOrdinal(caseName)
	if ordinal < 5 || ordinal > 9 {
		return "resultset-orderby-simple-expressions-aliases-unknown"
	}
	return resultsetOrderbySimpleExpressionsAliasesJavaExecutions[ordinal-5]
}
