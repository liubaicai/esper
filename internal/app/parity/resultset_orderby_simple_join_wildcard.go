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
	resultsetOrderbySimpleJoinWildcardID          = "resultset-orderby-simple-join-wildcard"
	resultsetOrderbySimpleJoinWildcardDescription = "ResultSetOrderBySimple ordinals 10-14: multi-key, simple, and wildcard order-by over length windows with output every 6 events; ordinals 10, 12, and 14 join SupportMarketDataBean to SupportBeanString and send five string seeds after the shared six market events, ordinal 13 selects the wildcard bean including null id and feed properties, and ordinal 14 selects the join wildcard as event-valued one/two stream properties."
	resultsetOrderbySimpleJoinWildcardJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOrderbySimpleJoinWildcardSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderBySimple.java"
)

var (
	resultsetOrderbySimpleJoinWildcardJavaSources = []string{
		resultsetOrderbySimpleJoinWildcardSource,
	}
	resultsetOrderbySimpleJoinWildcardJavaRuntimeIDs = []string{
		"java-runtime-6c1c5e581115d83acac7",
		"java-runtime-2524d06789dd35e36c8e",
		"java-runtime-d530652f60616c332431",
		"java-runtime-9455a5a72c84baa7bdc2",
		"java-runtime-4aaec5e95dbea839ece3",
	}
	resultsetOrderbySimpleJoinWildcardJavaExecutions = []string{
		"ResultSetMultipleKeysJoin",
		"ResultSetSimple",
		"ResultSetSimpleJoin",
		"ResultSetWildcard",
		"ResultSetWildcardJoin",
	}
	resultsetOrderbySimpleJoinWildcardJavaStaticIDs = []string{
		"java-042c1b302e7183feb8a6",
		"java-042c1b302e7183feb8a6",
		"java-042c1b302e7183feb8a6",
		"java-042c1b302e7183feb8a6",
		"java-042c1b302e7183feb8a6",
	}
	resultsetOrderbySimpleJoinWildcardCases = []string{
		"multiple-keys-join-v1",
		"multiple-keys-join-v2",
		"multiple-keys-join-v3",
		"simple-v1",
		"simple-v2",
		"simple-v3",
		"simple-v4",
		"simple-v5",
		"simple-v6",
		"simple-join-v1",
		"simple-join-v2",
		"simple-join-v3",
		"simple-join-v4",
		"simple-join-v5",
		"simple-join-v6",
		"wildcard-v1",
		"wildcard-v2",
		"wildcard-join-v1",
		"wildcard-join-v2",
	}
	resultsetOrderbySimpleJoinWildcardOrdinals = []int{
		10, 10, 10,
		11, 11, 11, 11, 11, 11,
		12, 12, 12, 12, 12, 12,
		13, 13,
		14, 14,
	}
	resultsetOrderbySimpleJoinWildcardEPLs = []string{
		"@name('s0') select symbol from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by symbol, price",
		"@name('s0') select symbol from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by price, symbol, volume",
		"@name('s0') select symbol, volume*2 from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by price, volume",
		"@name('s0') select symbol from SupportMarketDataBean#length(5) output every 6 events order by price",
		"@name('s0') select symbol, price from SupportMarketDataBean#length(5) output every 6 events order by price",
		"@name('s0') select symbol, volume from SupportMarketDataBean#length(5) output every 6 events order by price",
		"@name('s0') select symbol, volume*2 from SupportMarketDataBean#length(5) output every 6 events order by price",
		"@name('s0') select symbol, volume from SupportMarketDataBean#length(5) output every 6 events order by symbol",
		"@name('s0') select price from SupportMarketDataBean#length(5) output every 6 events order by symbol",
		"@name('s0') select symbol from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by price",
		"@name('s0') select symbol, price from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by price",
		"@name('s0') select symbol, volume from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by price",
		"@name('s0') select symbol, volume*2 from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by price",
		"@name('s0') select symbol, volume from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by symbol",
		"@name('s0') select price from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by symbol, price",
		"@name('s0') select * from SupportMarketDataBean#length(5) output every 6 events order by price",
		"@name('s0') select * from SupportMarketDataBean#length(5) output every 6 events order by symbol",
		"@name('s0') select * from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by price",
		"@name('s0') select * from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by symbol, price",
	}
	resultsetOrderbySimpleJoinWildcardObservations = []string{
		"listener; ordinal 10 variant 1 orders the six-row join output batch by symbol, price",
		"listener; ordinal 10 variant 2 orders the six-row join output batch by price, symbol, volume so the price-6 tie resolves CAT, CAT, IBM",
		"listener; ordinal 10 variant 3 selects symbol, volume*2 and orders the six-row join output batch by price, volume; the constant zero volume keeps join-generation order at the price-6 tie",
		"listener; ordinal 11 variant 1 orders the six-row output batch by price",
		"listener; ordinal 11 variant 2 selects symbol, price and orders the six-row output batch by price",
		"listener; ordinal 11 variant 3 selects symbol, volume and orders the six-row output batch by price",
		"listener; ordinal 11 variant 4 selects symbol, volume*2 and orders the six-row output batch by price",
		"listener; ordinal 11 variant 5 selects symbol, volume and orders the six-row output batch by symbol",
		"listener; ordinal 11 variant 6 selects price and orders the six-row output batch by symbol",
		"listener; ordinal 12 variant 1 orders the six-row join output batch by price",
		"listener; ordinal 12 variant 2 selects symbol, price and orders the six-row join output batch by price",
		"listener; ordinal 12 variant 3 selects symbol, volume and orders the six-row join output batch by price",
		"listener; ordinal 12 variant 4 selects symbol, volume*2 and orders the six-row join output batch by price",
		"listener; ordinal 12 variant 5 selects symbol, volume and orders the six-row join output batch by symbol",
		"listener; ordinal 12 variant 6 selects price and orders the six-row join output batch by symbol, price",
		"listener; ordinal 13 variant 1 selects the wildcard bean and orders the six-row output batch by price; rows expose the full property set including null id and feed",
		"listener; ordinal 13 variant 2 selects the wildcard bean and orders the six-row output batch by symbol; rows expose the full property set including null id and feed",
		"listener; ordinal 14 variant 1 selects the join wildcard and orders the six-row join output batch by price; rows carry the event-valued stream properties one and two",
		"listener; ordinal 14 variant 2 selects the join wildcard and orders the six-row join output batch by symbol, price; rows carry the event-valued stream properties one and two",
	}
)

// resultsetOrderbySimpleJoinWildcardBean mirrors the regression
// SupportMarketDataBean: the wildcard cases expose the full property set, so
// the nullable id and feed columns are declared even though the pinned sends
// only carry symbol, volume, and price.
type resultsetOrderbySimpleJoinWildcardBean struct {
	Symbol string  `esper:"symbol"`
	ID     *string `esper:"id"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
	Feed   *string `esper:"feed"`
}

type resultsetOrderbySimpleJoinWildcardString struct {
	TheString string `esper:"theString"`
}

// resultsetOrderbySimpleJoinWildcardEvents pins the shared six-event send
// sequence every case replays (Java sendEvent(symbol, price) with
// volume=0L): IBM@2, KGB@1, CMU@3, IBM@6, CAT@6, CAT@5.
var resultsetOrderbySimpleJoinWildcardEvents = []resultsetOrderbySimpleJoinWildcardBean{
	{Symbol: "IBM", Price: 2, Volume: 0},
	{Symbol: "KGB", Price: 1, Volume: 0},
	{Symbol: "CMU", Price: 3, Volume: 0},
	{Symbol: "IBM", Price: 6, Volume: 0},
	{Symbol: "CAT", Price: 6, Volume: 0},
	{Symbol: "CAT", Price: 5, Volume: 0},
}

// resultsetOrderbySimpleJoinWildcardJoinStrings pins the sendJoinEvents
// SupportBeanString sequence the ordinal 10, 12, and 14 join cases send after
// the six market events: CAT, IBM, CMU, KGB, DOG.
var resultsetOrderbySimpleJoinWildcardJoinStrings = []string{"CAT", "IBM", "CMU", "KGB", "DOG"}

func loadResultsetOrderbySimpleJoinWildcardScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOrderbySimpleJoinWildcardID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOrderbySimpleJoinWildcardID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOrderbySimpleJoinWildcardID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOrderbySimpleJoinWildcardID, err)
	}
	if err := requireResultsetOrderbySimpleJoinWildcardFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOrderbySimpleJoinWildcardID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetOrderbySimpleJoinWildcardID ||
		metadata.Description != resultsetOrderbySimpleJoinWildcardDescription ||
		metadata.JavaCommit != resultsetOrderbySimpleJoinWildcardJavaCommit ||
		metadata.JavaSource != resultsetOrderbySimpleJoinWildcardSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOrderbySimpleJoinWildcardID)
	}
	if err := validateResultsetOrderbySimpleJoinWildcardStringArray(root["javaRuntimes"], resultsetOrderbySimpleJoinWildcardJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbySimpleJoinWildcardStringArray(root["javaNames"], resultsetOrderbySimpleJoinWildcardJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbySimpleJoinWildcardStringArray(root["javaStaticIds"], resultsetOrderbySimpleJoinWildcardJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbySimpleJoinWildcardStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetOrderbySimpleJoinWildcardCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly nineteen cases", resultsetOrderbySimpleJoinWildcardID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetOrderbySimpleJoinWildcardFields(object,
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
		if definition.Case != resultsetOrderbySimpleJoinWildcardCases[index] ||
			definition.Ordinal != resultsetOrderbySimpleJoinWildcardOrdinals[index] ||
			definition.RuntimeID != resultsetOrderbySimpleJoinWildcardRuntimeID(definition.Case) ||
			definition.ExecutionName != resultsetOrderbySimpleJoinWildcardExecutionName(definition.Case) ||
			definition.Observation != resultsetOrderbySimpleJoinWildcardObservations[index] ||
			definition.IteratorSnapshots != 0 ||
			definition.EPL != resultsetOrderbySimpleJoinWildcardEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetOrderbySimpleJoinWildcardID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 188 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 188 steps", resultsetOrderbySimpleJoinWildcardID)
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
			if err := requireResultsetOrderbySimpleJoinWildcardFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireResultsetOrderbySimpleJoinWildcardFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeResultsetOrderbySimpleJoinWildcardPayload(payload); err != nil {
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
	if err := validateResultsetOrderbySimpleJoinWildcardScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetOrderbySimpleJoinWildcardScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetOrderbySimpleJoinWildcardID || len(scenario.Steps) != 188 {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetOrderbySimpleJoinWildcardID)
	}
	index := 0
	for caseIndex, caseName := range resultsetOrderbySimpleJoinWildcardCases {
		if err := validateResultsetOrderbySimpleJoinWildcardCaseMarker(scenario.Steps[index], caseName); err != nil {
			return fmt.Errorf("case %d marker: %w", caseIndex, err)
		}
		index++
		for _, expected := range resultsetOrderbySimpleJoinWildcardEvents {
			if err := validateResultsetOrderbySimpleJoinWildcardBeanStep(scenario.Steps[index], expected); err != nil {
				return fmt.Errorf("case %d bean step: %w", caseIndex, err)
			}
			index++
		}
		if resultsetOrderbySimpleJoinWildcardJoinCase(caseName) {
			for _, expected := range resultsetOrderbySimpleJoinWildcardJoinStrings {
				if err := validateResultsetOrderbySimpleJoinWildcardStringStep(scenario.Steps[index], expected); err != nil {
					return fmt.Errorf("case %d string step: %w", caseIndex, err)
				}
				index++
			}
		}
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetOrderbySimpleJoinWildcardID)
	}
	return nil
}

func validateResultsetOrderbySimpleJoinWildcardCaseMarker(step compat.Step, expected string) error {
	if step.Op != "case" || step.Case != expected {
		return fmt.Errorf("must start case %q", expected)
	}
	return nil
}

func validateResultsetOrderbySimpleJoinWildcardBeanStep(step compat.Step, expected resultsetOrderbySimpleJoinWildcardBean) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportMarketDataBean" {
		return fmt.Errorf("must send SupportMarketDataBean")
	}
	bean, err := decodeResultsetOrderbySimpleJoinWildcardBean(step)
	if err != nil {
		return err
	}
	if bean.Symbol != expected.Symbol || bean.Price != expected.Price || bean.Volume != expected.Volume {
		return fmt.Errorf("bean payload is not pinned")
	}
	return nil
}

func validateResultsetOrderbySimpleJoinWildcardStringStep(step compat.Step, expected string) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportBeanString" {
		return fmt.Errorf("must send SupportBeanString")
	}
	bean, err := decodeResultsetOrderbySimpleJoinWildcardString(step)
	if err != nil {
		return err
	}
	if bean.TheString != expected {
		return fmt.Errorf("string payload is not pinned")
	}
	return nil
}

func runResultsetOrderbySimpleJoinWildcardScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetOrderbySimpleJoinWildcardScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultsetOrderbySimpleJoinWildcardCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultsetOrderbySimpleJoinWildcardCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOrderbySimpleJoinWildcardID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOrderbySimpleJoinWildcardID, scenario.ID)
	}
	return trace, nil
}

func runResultsetOrderbySimpleJoinWildcardCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOrderbySimpleJoinWildcardBean](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetOrderbySimpleJoinWildcardString](env, "SupportBeanString"); err != nil {
		return compat.Trace{}, err
	}

	symbol := esper.Field[resultsetOrderbySimpleJoinWildcardBean, string]("symbol")
	price := esper.Field[resultsetOrderbySimpleJoinWildcardBean, float64]("price")
	volume := esper.Field[resultsetOrderbySimpleJoinWildcardBean, int64]("volume")
	theString := esper.Field[resultsetOrderbySimpleJoinWildcardString, string]("theString")
	windowed5 := esper.From[resultsetOrderbySimpleJoinWildcardBean](env, "SupportMarketDataBean").Window(esper.LengthWindow(5))
	volumeTimesTwo := esper.MultiplyOf[int64](volume, esper.Literal(int64(2)))

	joinSymbol := esper.JoinField[string](0, "symbol")
	joinPrice := esper.JoinField[float64](0, "price")
	joinVolume := esper.JoinField[int64](0, "volume")
	joinVolumeTimesTwo := esper.MultiplyOf[int64](joinVolume, esper.Literal(int64(2)))

	market := esper.From[resultsetOrderbySimpleJoinWildcardBean](env, "SupportMarketDataBean").Window(esper.LengthWindow(10))
	seed := esper.From[resultsetOrderbySimpleJoinWildcardString](env, "SupportBeanString").Window(esper.LengthWindow(100))
	joined := esper.Join(market, seed, esper.OnEqual(symbol, theString))

	var query esper.Query
	switch caseName {
	case "multiple-keys-join-v1":
		query = joined.Select(
			esper.SelectFrom(0, "symbol", joinSymbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(joinSymbol),
				esper.Ascending(joinPrice),
			),
		)
	case "multiple-keys-join-v2":
		query = joined.Select(
			esper.SelectFrom(0, "symbol", joinSymbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(joinPrice),
				esper.Ascending(joinSymbol),
				esper.Ascending(joinVolume),
			),
		)
	case "multiple-keys-join-v3":
		query = joined.Select(
			esper.SelectFrom(0, "symbol", joinSymbol),
			esper.SelectFrom(0, "volume*2", joinVolumeTimesTwo),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(joinPrice),
				esper.Ascending(joinVolume),
			),
		)
	case "simple-v1":
		query = esper.Select(windowed5,
			esper.Alias("symbol", symbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(price)),
		)
	case "simple-v2":
		query = esper.Select(windowed5,
			esper.Alias("symbol", symbol),
			esper.Alias("price", price),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(price)),
		)
	case "simple-v3":
		query = esper.Select(windowed5,
			esper.Alias("symbol", symbol),
			esper.Alias("volume", volume),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(price)),
		)
	case "simple-v4":
		query = esper.Select(windowed5,
			esper.Alias("symbol", symbol),
			esper.Alias("volume*2", volumeTimesTwo),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(price)),
		)
	case "simple-v5":
		query = esper.Select(windowed5,
			esper.Alias("symbol", symbol),
			esper.Alias("volume", volume),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(symbol)),
		)
	case "simple-v6":
		query = esper.Select(windowed5,
			esper.Alias("price", price),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(symbol)),
		)
	case "simple-join-v1":
		query = joined.Select(
			esper.SelectFrom(0, "symbol", joinSymbol),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(joinPrice)),
		)
	case "simple-join-v2":
		query = joined.Select(
			esper.SelectFrom(0, "symbol", joinSymbol),
			esper.SelectFrom(0, "price", joinPrice),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(joinPrice)),
		)
	case "simple-join-v3":
		query = joined.Select(
			esper.SelectFrom(0, "symbol", joinSymbol),
			esper.SelectFrom(0, "volume", joinVolume),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(joinPrice)),
		)
	case "simple-join-v4":
		query = joined.Select(
			esper.SelectFrom(0, "symbol", joinSymbol),
			esper.SelectFrom(0, "volume*2", joinVolumeTimesTwo),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(joinPrice)),
		)
	case "simple-join-v5":
		query = joined.Select(
			esper.SelectFrom(0, "symbol", joinSymbol),
			esper.SelectFrom(0, "volume", joinVolume),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(joinSymbol)),
		)
	case "simple-join-v6":
		query = joined.Select(
			esper.SelectFrom(0, "price", joinPrice),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(joinSymbol),
				esper.Ascending(joinPrice),
			),
		)
	case "wildcard-v1":
		query = windowed5.Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(price)),
		)
	case "wildcard-v2":
		query = windowed5.Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(symbol)),
		)
	case "wildcard-join-v1":
		query = joined.Select(
			esper.SelectSourceEvent(0, "one"),
			esper.SelectSourceEvent(1, "two"),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(esper.Ascending(joinPrice)),
		)
	case "wildcard-join-v2":
		query = joined.Select(
			esper.SelectSourceEvent(0, "one"),
			esper.SelectSourceEvent(1, "two"),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(6)),
			esper.OrderBy(
				esper.Ascending(joinSymbol),
				esper.Ascending(joinPrice),
			),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported %s case %q", resultsetOrderbySimpleJoinWildcardID, caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithRuntimeURI(resultsetOrderbySimpleJoinWildcardRuntimeID(caseName)))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("%s case %q deployed %d statements", resultsetOrderbySimpleJoinWildcardID, caseName, len(statements))
	}
	statement := statements[0]
	return compat.ReplayWithStatements(ctx, engine, statement, scenario,
		decodeResultsetOrderbySimpleJoinWildcardPayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetOrderbySimpleJoinWildcardID, name)
			}
			return statement, nil
		})
}

func decodeResultsetOrderbySimpleJoinWildcardPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportMarketDataBean":
		return decodeResultsetOrderbySimpleJoinWildcardBean(step)
	case "SupportBeanString":
		return decodeResultsetOrderbySimpleJoinWildcardString(step)
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOrderbySimpleJoinWildcardID, step.EventType)
	}
}

func decodeResultsetOrderbySimpleJoinWildcardBean(step compat.Step) (resultsetOrderbySimpleJoinWildcardBean, error) {
	if step.EventType != "SupportMarketDataBean" {
		return resultsetOrderbySimpleJoinWildcardBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOrderbySimpleJoinWildcardBean{}, err
	}
	if err := requireResultsetOrderbySimpleJoinWildcardFields(fields, "symbol", "volume", "price"); err != nil {
		return resultsetOrderbySimpleJoinWildcardBean{}, err
	}
	symbol, err := decodeResultsetOrderbySimpleJoinWildcardStringValue(fields["symbol"], "symbol")
	if err != nil {
		return resultsetOrderbySimpleJoinWildcardBean{}, err
	}
	volume, err := decodeResultsetOrderbySimpleJoinWildcardInteger(fields["volume"], "volume")
	if err != nil {
		return resultsetOrderbySimpleJoinWildcardBean{}, err
	}
	price, err := decodeResultsetOrderbySimpleJoinWildcardFloat(fields["price"], "price")
	if err != nil {
		return resultsetOrderbySimpleJoinWildcardBean{}, err
	}
	return resultsetOrderbySimpleJoinWildcardBean{Symbol: symbol, Price: price, Volume: volume}, nil
}

func decodeResultsetOrderbySimpleJoinWildcardString(step compat.Step) (resultsetOrderbySimpleJoinWildcardString, error) {
	if step.EventType != "SupportBeanString" {
		return resultsetOrderbySimpleJoinWildcardString{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOrderbySimpleJoinWildcardString{}, err
	}
	if err := requireResultsetOrderbySimpleJoinWildcardFields(fields, "theString"); err != nil {
		return resultsetOrderbySimpleJoinWildcardString{}, err
	}
	theString, err := decodeResultsetOrderbySimpleJoinWildcardStringValue(fields["theString"], "theString")
	if err != nil {
		return resultsetOrderbySimpleJoinWildcardString{}, err
	}
	return resultsetOrderbySimpleJoinWildcardString{TheString: theString}, nil
}

func requireResultsetOrderbySimpleJoinWildcardFields(object map[string]json.RawMessage, names ...string) error {
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

func decodeResultsetOrderbySimpleJoinWildcardStringValue(raw json.RawMessage, name string) (string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must be a string", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func decodeResultsetOrderbySimpleJoinWildcardInteger(raw json.RawMessage, name string) (int64, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	number, ok := token.(json.Number)
	if !ok || !resultsetOrderbySimpleJoinWildcardIntegerSyntax(string(number)) {
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

func decodeResultsetOrderbySimpleJoinWildcardFloat(raw json.RawMessage, name string) (float64, error) {
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

func resultsetOrderbySimpleJoinWildcardIntegerSyntax(text string) bool {
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

func validateResultsetOrderbySimpleJoinWildcardStringArray(raw json.RawMessage, expected []string, name string) error {
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

func resultsetOrderbySimpleJoinWildcardOrdinal(caseName string) int {
	for index, name := range resultsetOrderbySimpleJoinWildcardCases {
		if name == caseName {
			return resultsetOrderbySimpleJoinWildcardOrdinals[index]
		}
	}
	return -1
}

// resultsetOrderbySimpleJoinWildcardJoinCase reports whether the case sends
// the five SupportBeanString seeds after the shared market events: ordinals
// 10, 12, and 14 are the join executions.
func resultsetOrderbySimpleJoinWildcardJoinCase(caseName string) bool {
	ordinal := resultsetOrderbySimpleJoinWildcardOrdinal(caseName)
	return ordinal == 10 || ordinal == 12 || ordinal == 14
}

func resultsetOrderbySimpleJoinWildcardRuntimeID(caseName string) string {
	ordinal := resultsetOrderbySimpleJoinWildcardOrdinal(caseName)
	if ordinal < 10 || ordinal > 14 {
		return "resultset-orderby-simple-join-wildcard-unknown"
	}
	return resultsetOrderbySimpleJoinWildcardJavaRuntimeIDs[ordinal-10]
}

func resultsetOrderbySimpleJoinWildcardExecutionName(caseName string) string {
	ordinal := resultsetOrderbySimpleJoinWildcardOrdinal(caseName)
	if ordinal < 10 || ordinal > 14 {
		return "resultset-orderby-simple-join-wildcard-unknown"
	}
	return resultsetOrderbySimpleJoinWildcardJavaExecutions[ordinal-10]
}
