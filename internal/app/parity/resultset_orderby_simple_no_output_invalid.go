package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultsetOrderbySimpleNoOutputInvalidID          = "resultset-orderby-simple-no-output-invalid"
	resultsetOrderbySimpleNoOutputInvalidDescription = "ResultSetOrderBySimple ordinals 15-17: order-by without an output clause over a length window and a time batch, the same two shapes over the SupportMarketDataBean/SupportBeanString join, and six build-error probes pinning the aggregate order-by compile diagnostic."
	resultsetOrderbySimpleNoOutputInvalidJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOrderbySimpleNoOutputInvalidSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderBySimple.java"
	resultsetOrderbySimpleNoOutputInvalidError       = "Aggregate functions in the order-by clause must also occur in the select expression"
)

var (
	resultsetOrderbySimpleNoOutputInvalidJavaSources = []string{
		resultsetOrderbySimpleNoOutputInvalidSource,
	}
	resultsetOrderbySimpleNoOutputInvalidJavaRuntimeIDs = []string{
		"java-runtime-6de2b14776f97a0a69b2",
		"java-runtime-091c77bae73759b7cb3e",
		"java-runtime-bbc3ab446f26d0f99488",
	}
	resultsetOrderbySimpleNoOutputInvalidJavaExecutions = []string{
		"ResultSetNoOutputClauseView",
		"ResultSetNoOutputClauseJoin",
		"ResultSetInvalid",
	}
	resultsetOrderbySimpleNoOutputInvalidJavaStaticIDs = []string{
		"java-042c1b302e7183feb8a6",
		"java-042c1b302e7183feb8a6",
		"java-042c1b302e7183feb8a6",
	}
	resultsetOrderbySimpleNoOutputInvalidCases = []string{
		"no-output-view-v1",
		"no-output-view-v2",
		"no-output-join-v1",
		"no-output-join-v2",
		"invalid",
	}
	resultsetOrderbySimpleNoOutputInvalidOrdinals = []int{
		15, 15,
		16, 16,
		17,
	}
	resultsetOrderbySimpleNoOutputInvalidEPLs = []string{
		"@name('s0') select symbol from SupportMarketDataBean#length(5) order by price",
		"@name('s0') select symbol from SupportMarketDataBean#time_batch(1 sec) order by price",
		"@name('s0') select symbol from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString order by price",
		"@name('s0') select symbol from SupportMarketDataBean#time_batch(1) as one, SupportBeanString#length(100) as two where one.symbol = two.theString order by price, symbol",
		"@name('s0') select symbol from SupportMarketDataBean#length(5) output every 6 events order by sum(price)",
	}
	resultsetOrderbySimpleNoOutputInvalidObservations = []string{
		"listener; ordinal 15 variant 1 emits one sorted single-row batch per market event over length(5) with no output clause",
		"listener; ordinal 15 variant 2 emits one six-row batch sorted by price when the time_batch(1 sec) window flushes at the one-second boundary",
		"listener; ordinal 16 variant 1 emits one sorted single-row batch per matching join row over length(10) with no output clause",
		"listener; ordinal 16 variant 2 emits one six-row join batch sorted by price, symbol when the time_batch(1) window flushes at the one-second boundary",
		"compile-only; ordinal 17 rejects six statements whose order-by aggregate does not occur identically in the select expression",
	}
)

// resultsetOrderbySimpleNoOutputInvalidProbes pins the six ordinal 17
// tryInvalidCompile calls: three single-stream statements and three join
// statements whose order-by aggregate is absent from the select expression.
// Java asserts the message prefix; the scenario pins the full compiler
// diagnostic including the trailing [epl] the compiler appends.
var resultsetOrderbySimpleNoOutputInvalidProbes = []struct {
	statement string
	epl       string
}{
	{"single-sum-missing", "@name('s0') select symbol from SupportMarketDataBean#length(5) output every 6 events order by sum(price)"},
	{"single-sum-different", "@name('s0') select sum(price) from SupportMarketDataBean#length(5) output every 6 events order by sum(price + 6)"},
	{"single-sum-input-different", "@name('s0') select sum(price + 6) from SupportMarketDataBean#length(5) output every 6 events order by sum(price)"},
	{"join-sum-missing", "@name('s0') select symbol from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by sum(price)"},
	{"join-sum-different", "@name('s0') select sum(price) from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by sum(price + 6)"},
	{"join-sum-input-different", "@name('s0') select sum(price + 6) from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by sum(price)"},
}

// resultsetOrderbySimpleNoOutputInvalidBean mirrors the regression
// SupportMarketDataBean: the pinned sends only carry symbol, volume, and
// price, matching Java's sendEvent(symbol, price) with volume=0L.
type resultsetOrderbySimpleNoOutputInvalidBean struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
}

type resultsetOrderbySimpleNoOutputInvalidString struct {
	TheString string `esper:"theString"`
}

// resultsetOrderbySimpleNoOutputInvalidEvents pins the shared six-event send
// sequence createAndSend replays in every ordinal 15/16 variant:
// IBM@2, KGB@1, CMU@3, IBM@6, CAT@6, CAT@5.
var resultsetOrderbySimpleNoOutputInvalidEvents = []resultsetOrderbySimpleNoOutputInvalidBean{
	{Symbol: "IBM", Price: 2, Volume: 0},
	{Symbol: "KGB", Price: 1, Volume: 0},
	{Symbol: "CMU", Price: 3, Volume: 0},
	{Symbol: "IBM", Price: 6, Volume: 0},
	{Symbol: "CAT", Price: 6, Volume: 0},
	{Symbol: "CAT", Price: 5, Volume: 0},
}

// resultsetOrderbySimpleNoOutputInvalidJoinStrings pins the sendJoinEvents
// SupportBeanString sequence the ordinal 16 join variants send after the six
// market events: CAT, IBM, CMU, KGB, DOG.
var resultsetOrderbySimpleNoOutputInvalidJoinStrings = []string{"CAT", "IBM", "CMU", "KGB", "DOG"}

func loadResultsetOrderbySimpleNoOutputInvalidScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOrderbySimpleNoOutputInvalidID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOrderbySimpleNoOutputInvalidID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOrderbySimpleNoOutputInvalidID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOrderbySimpleNoOutputInvalidID, err)
	}
	if err := requireResultsetOrderbySimpleNoOutputInvalidFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOrderbySimpleNoOutputInvalidID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetOrderbySimpleNoOutputInvalidID ||
		metadata.Description != resultsetOrderbySimpleNoOutputInvalidDescription ||
		metadata.JavaCommit != resultsetOrderbySimpleNoOutputInvalidJavaCommit ||
		metadata.JavaSource != resultsetOrderbySimpleNoOutputInvalidSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOrderbySimpleNoOutputInvalidID)
	}
	if err := validateResultsetOrderbySimpleNoOutputInvalidStringArray(root["javaRuntimes"], resultsetOrderbySimpleNoOutputInvalidJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbySimpleNoOutputInvalidStringArray(root["javaNames"], resultsetOrderbySimpleNoOutputInvalidJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbySimpleNoOutputInvalidStringArray(root["javaStaticIds"], resultsetOrderbySimpleNoOutputInvalidJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbySimpleNoOutputInvalidStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetOrderbySimpleNoOutputInvalidCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly five cases", resultsetOrderbySimpleNoOutputInvalidID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetOrderbySimpleNoOutputInvalidFields(object,
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
		if definition.Case != resultsetOrderbySimpleNoOutputInvalidCases[index] ||
			definition.Ordinal != resultsetOrderbySimpleNoOutputInvalidOrdinals[index] ||
			definition.RuntimeID != resultsetOrderbySimpleNoOutputInvalidRuntimeID(definition.Case) ||
			definition.ExecutionName != resultsetOrderbySimpleNoOutputInvalidExecutionName(definition.Case) ||
			definition.Observation != resultsetOrderbySimpleNoOutputInvalidObservations[index] ||
			definition.IteratorSnapshots != 0 ||
			definition.EPL != resultsetOrderbySimpleNoOutputInvalidEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetOrderbySimpleNoOutputInvalidID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 51 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 51 steps", resultsetOrderbySimpleNoOutputInvalidID)
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
			if err := requireResultsetOrderbySimpleNoOutputInvalidFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireResultsetOrderbySimpleNoOutputInvalidFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeResultsetOrderbySimpleNoOutputInvalidPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		case "advance-time":
			if err := requireResultsetOrderbySimpleNoOutputInvalidFields(object, "op", "at"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "build-error":
			if err := requireResultsetOrderbySimpleNoOutputInvalidFields(object, "op", "statement", "epl", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateResultsetOrderbySimpleNoOutputInvalidScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetOrderbySimpleNoOutputInvalidScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetOrderbySimpleNoOutputInvalidID || len(scenario.Steps) != 51 {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetOrderbySimpleNoOutputInvalidID)
	}
	index := 0
	for caseIndex, caseName := range resultsetOrderbySimpleNoOutputInvalidCases {
		if err := validateResultsetOrderbySimpleNoOutputInvalidCaseMarker(scenario.Steps[index], caseName); err != nil {
			return fmt.Errorf("case %d marker: %w", caseIndex, err)
		}
		index++
		switch caseName {
		case "no-output-view-v1":
			for _, expected := range resultsetOrderbySimpleNoOutputInvalidEvents {
				if err := validateResultsetOrderbySimpleNoOutputInvalidBeanStep(scenario.Steps[index], expected); err != nil {
					return fmt.Errorf("case %d bean step: %w", caseIndex, err)
				}
				index++
			}
			if err := validateResultsetOrderbySimpleNoOutputInvalidBeanStep(scenario.Steps[index], resultsetOrderbySimpleNoOutputInvalidBean{Symbol: "FOX", Price: 10, Volume: 0}); err != nil {
				return fmt.Errorf("case %d bean step: %w", caseIndex, err)
			}
			index++
		case "no-output-view-v2":
			if err := validateResultsetOrderbySimpleNoOutputInvalidAdvanceStep(scenario.Steps[index], "1970-01-01T00:00:00.000Z"); err != nil {
				return fmt.Errorf("case %d advance step: %w", caseIndex, err)
			}
			index++
			for _, expected := range resultsetOrderbySimpleNoOutputInvalidEvents {
				if err := validateResultsetOrderbySimpleNoOutputInvalidBeanStep(scenario.Steps[index], expected); err != nil {
					return fmt.Errorf("case %d bean step: %w", caseIndex, err)
				}
				index++
			}
			if err := validateResultsetOrderbySimpleNoOutputInvalidAdvanceStep(scenario.Steps[index], "1970-01-01T00:00:01.000Z"); err != nil {
				return fmt.Errorf("case %d advance step: %w", caseIndex, err)
			}
			index++
		case "no-output-join-v1":
			for _, expected := range resultsetOrderbySimpleNoOutputInvalidEvents {
				if err := validateResultsetOrderbySimpleNoOutputInvalidBeanStep(scenario.Steps[index], expected); err != nil {
					return fmt.Errorf("case %d bean step: %w", caseIndex, err)
				}
				index++
			}
			for _, expected := range resultsetOrderbySimpleNoOutputInvalidJoinStrings {
				if err := validateResultsetOrderbySimpleNoOutputInvalidStringStep(scenario.Steps[index], expected); err != nil {
					return fmt.Errorf("case %d string step: %w", caseIndex, err)
				}
				index++
			}
			if err := validateResultsetOrderbySimpleNoOutputInvalidBeanStep(scenario.Steps[index], resultsetOrderbySimpleNoOutputInvalidBean{Symbol: "DOG", Price: 10, Volume: 0}); err != nil {
				return fmt.Errorf("case %d bean step: %w", caseIndex, err)
			}
			index++
		case "no-output-join-v2":
			if err := validateResultsetOrderbySimpleNoOutputInvalidAdvanceStep(scenario.Steps[index], "1970-01-01T00:00:00.000Z"); err != nil {
				return fmt.Errorf("case %d advance step: %w", caseIndex, err)
			}
			index++
			for _, expected := range resultsetOrderbySimpleNoOutputInvalidEvents {
				if err := validateResultsetOrderbySimpleNoOutputInvalidBeanStep(scenario.Steps[index], expected); err != nil {
					return fmt.Errorf("case %d bean step: %w", caseIndex, err)
				}
				index++
			}
			for _, expected := range resultsetOrderbySimpleNoOutputInvalidJoinStrings {
				if err := validateResultsetOrderbySimpleNoOutputInvalidStringStep(scenario.Steps[index], expected); err != nil {
					return fmt.Errorf("case %d string step: %w", caseIndex, err)
				}
				index++
			}
			if err := validateResultsetOrderbySimpleNoOutputInvalidAdvanceStep(scenario.Steps[index], "1970-01-01T00:00:01.000Z"); err != nil {
				return fmt.Errorf("case %d advance step: %w", caseIndex, err)
			}
			index++
		case "invalid":
			for _, probe := range resultsetOrderbySimpleNoOutputInvalidProbes {
				if err := validateResultsetOrderbySimpleNoOutputInvalidProbeStep(scenario.Steps[index], probe.statement, probe.epl); err != nil {
					return fmt.Errorf("case %d probe step: %w", caseIndex, err)
				}
				index++
			}
		}
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetOrderbySimpleNoOutputInvalidID)
	}
	return nil
}

func validateResultsetOrderbySimpleNoOutputInvalidCaseMarker(step compat.Step, expected string) error {
	if step.Op != "case" || step.Case != expected {
		return fmt.Errorf("must start case %q", expected)
	}
	return nil
}

func validateResultsetOrderbySimpleNoOutputInvalidBeanStep(step compat.Step, expected resultsetOrderbySimpleNoOutputInvalidBean) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportMarketDataBean" {
		return fmt.Errorf("must send SupportMarketDataBean")
	}
	bean, err := decodeResultsetOrderbySimpleNoOutputInvalidBean(step)
	if err != nil {
		return err
	}
	if bean.Symbol != expected.Symbol || bean.Price != expected.Price || bean.Volume != expected.Volume {
		return fmt.Errorf("bean payload is not pinned")
	}
	return nil
}

func validateResultsetOrderbySimpleNoOutputInvalidStringStep(step compat.Step, expected string) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportBeanString" {
		return fmt.Errorf("must send SupportBeanString")
	}
	bean, err := decodeResultsetOrderbySimpleNoOutputInvalidString(step)
	if err != nil {
		return err
	}
	if bean.TheString != expected {
		return fmt.Errorf("string payload is not pinned")
	}
	return nil
}

func validateResultsetOrderbySimpleNoOutputInvalidAdvanceStep(step compat.Step, expected string) error {
	if step.Op != "advance-time" || step.Case != "" || step.At != expected {
		return fmt.Errorf("must advance time to %s", expected)
	}
	return nil
}

func validateResultsetOrderbySimpleNoOutputInvalidProbeStep(step compat.Step, statement, epl string) error {
	expected := resultsetOrderbySimpleNoOutputInvalidError + " [" + epl + "]"
	if step.Op != "build-error" || step.Case != "" || step.Statement != statement || step.Epl != epl || step.ExpectError != expected {
		return fmt.Errorf("build-error probe %q is not pinned", statement)
	}
	return nil
}

func runResultsetOrderbySimpleNoOutputInvalidScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetOrderbySimpleNoOutputInvalidScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultsetOrderbySimpleNoOutputInvalidCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		var caseTrace compat.Trace
		if caseName == "invalid" {
			caseTrace, err = runResultsetOrderbySimpleNoOutputInvalidProbes(ctx)
		} else {
			caseTrace, err = runResultsetOrderbySimpleNoOutputInvalidCase(ctx, caseScenario, caseName)
		}
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOrderbySimpleNoOutputInvalidID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOrderbySimpleNoOutputInvalidID, scenario.ID)
	}
	return trace, nil
}

func runResultsetOrderbySimpleNoOutputInvalidCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOrderbySimpleNoOutputInvalidBean](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetOrderbySimpleNoOutputInvalidString](env, "SupportBeanString"); err != nil {
		return compat.Trace{}, err
	}

	symbol := esper.Field[resultsetOrderbySimpleNoOutputInvalidBean, string]("symbol")
	price := esper.Field[resultsetOrderbySimpleNoOutputInvalidBean, float64]("price")
	theString := esper.Field[resultsetOrderbySimpleNoOutputInvalidString, string]("theString")

	joinSymbol := esper.JoinField[string](0, "symbol")
	joinPrice := esper.JoinField[float64](0, "price")

	var query esper.Query
	switch caseName {
	case "no-output-view-v1":
		query = esper.Select(
			esper.From[resultsetOrderbySimpleNoOutputInvalidBean](env, "SupportMarketDataBean").Window(esper.LengthWindow(5)),
			esper.Alias("symbol", symbol),
		).Query(
			esper.StatementName("s0"),
			esper.OrderBy(esper.Ascending(price)),
		)
	case "no-output-view-v2":
		query = esper.Select(
			esper.From[resultsetOrderbySimpleNoOutputInvalidBean](env, "SupportMarketDataBean").Window(esper.TimeBatch(time.Second)),
			esper.Alias("symbol", symbol),
		).Query(
			esper.StatementName("s0"),
			esper.OrderBy(esper.Ascending(price)),
		)
	case "no-output-join-v1":
		market := esper.From[resultsetOrderbySimpleNoOutputInvalidBean](env, "SupportMarketDataBean").Window(esper.LengthWindow(10))
		seed := esper.From[resultsetOrderbySimpleNoOutputInvalidString](env, "SupportBeanString").Window(esper.LengthWindow(100))
		query = esper.Join(market, seed, esper.OnEqual(symbol, theString)).Select(
			esper.SelectFrom(0, "symbol", joinSymbol),
		).Query(
			esper.StatementName("s0"),
			esper.OrderBy(esper.Ascending(joinPrice)),
		)
	case "no-output-join-v2":
		market := esper.From[resultsetOrderbySimpleNoOutputInvalidBean](env, "SupportMarketDataBean").Window(esper.TimeBatch(time.Second))
		seed := esper.From[resultsetOrderbySimpleNoOutputInvalidString](env, "SupportBeanString").Window(esper.LengthWindow(100))
		query = esper.Join(market, seed, esper.OnEqual(symbol, theString)).Select(
			esper.SelectFrom(0, "symbol", joinSymbol),
		).Query(
			esper.StatementName("s0"),
			esper.OrderBy(
				esper.Ascending(joinPrice),
				esper.Ascending(joinSymbol),
			),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported %s case %q", resultsetOrderbySimpleNoOutputInvalidID, caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(resultsetOrderbySimpleNoOutputInvalidRuntimeID(caseName)),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("%s case %q deployed %d statements", resultsetOrderbySimpleNoOutputInvalidID, caseName, len(statements))
	}
	statement := statements[0]
	return compat.ReplayWithStatements(ctx, engine, statement, scenario,
		decodeResultsetOrderbySimpleNoOutputInvalidPayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetOrderbySimpleNoOutputInvalidID, name)
			}
			return statement, nil
		})
}

// runResultsetOrderbySimpleNoOutputInvalidProbes replays ordinal 17: each
// pinned statement must fail to build with the Java aggregate order-by
// diagnostic. The Go plan builder applies the same rule, so a successful
// build or a drifting message fails the parity run.
func runResultsetOrderbySimpleNoOutputInvalidProbes(ctx context.Context) (compat.Trace, error) {
	if err := ctx.Err(); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOrderbySimpleNoOutputInvalidBean](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetOrderbySimpleNoOutputInvalidString](env, "SupportBeanString"); err != nil {
		return compat.Trace{}, err
	}

	symbol := esper.Field[resultsetOrderbySimpleNoOutputInvalidBean, string]("symbol")
	price := esper.Field[resultsetOrderbySimpleNoOutputInvalidBean, float64]("price")
	pricePlusSix := esper.AddOf[float64](price, esper.Literal(6))
	theString := esper.Field[resultsetOrderbySimpleNoOutputInvalidString, string]("theString")

	joinPrice := esper.JoinField[float64](0, "price")
	joinPricePlusSix := esper.AddOf[float64](joinPrice, esper.Literal(6))

	market5 := esper.From[resultsetOrderbySimpleNoOutputInvalidBean](env, "SupportMarketDataBean").Window(esper.LengthWindow(5))
	market10 := esper.From[resultsetOrderbySimpleNoOutputInvalidBean](env, "SupportMarketDataBean").Window(esper.LengthWindow(10))
	seed := esper.From[resultsetOrderbySimpleNoOutputInvalidString](env, "SupportBeanString").Window(esper.LengthWindow(100))
	joined := esper.Join(market10, seed, esper.OnEqual(symbol, theString))

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultsetOrderbySimpleNoOutputInvalidID}
	for index, probe := range resultsetOrderbySimpleNoOutputInvalidProbes {
		var query esper.Query
		switch probe.statement {
		case "single-sum-missing":
			query = esper.Select(market5,
				esper.Alias("symbol", symbol),
			).Query(
				esper.StatementName("s0"),
				esper.WithOutput(esper.OutputAllEveryEvents(6)),
				esper.OrderBy(esper.Ascending(esper.Sum[float64](price))),
			)
		case "single-sum-different":
			query = market5.Aggregate(
				esper.Alias("sum(price)", esper.Sum[float64](price)),
			).Query(
				esper.StatementName("s0"),
				esper.WithOutput(esper.OutputAllEveryEvents(6)),
				esper.OrderBy(esper.Ascending(esper.Sum[float64](pricePlusSix))),
			)
		case "single-sum-input-different":
			query = market5.Aggregate(
				esper.Alias("sum(price + 6)", esper.Sum[float64](pricePlusSix)),
			).Query(
				esper.StatementName("s0"),
				esper.WithOutput(esper.OutputAllEveryEvents(6)),
				esper.OrderBy(esper.Ascending(esper.Sum[float64](price))),
			)
		case "join-sum-missing":
			query = joined.Select(
				esper.SelectFrom(0, "symbol", esper.JoinField[string](0, "symbol")),
			).Query(
				esper.StatementName("s0"),
				esper.WithOutput(esper.OutputAllEveryEvents(6)),
				esper.OrderBy(esper.Ascending(esper.Sum[float64](joinPrice))),
			)
		case "join-sum-different":
			query = joined.Aggregate(
				esper.Alias("sum(price)", esper.Sum[float64](joinPrice)),
			).Query(
				esper.StatementName("s0"),
				esper.WithOutput(esper.OutputAllEveryEvents(6)),
				esper.OrderBy(esper.Ascending(esper.Sum[float64](joinPricePlusSix))),
			)
		case "join-sum-input-different":
			query = joined.Aggregate(
				esper.Alias("sum(price + 6)", esper.Sum[float64](joinPricePlusSix)),
			).Query(
				esper.StatementName("s0"),
				esper.WithOutput(esper.OutputAllEveryEvents(6)),
				esper.OrderBy(esper.Ascending(esper.Sum[float64](joinPrice))),
			)
		default:
			return trace, fmt.Errorf("unsupported %s probe %q", resultsetOrderbySimpleNoOutputInvalidID, probe.statement)
		}
		_, buildErr := env.Build(query)
		record := compat.TraceRecord{
			Case:      "invalid",
			Operation: "compile-rejected",
			Statement: probe.statement,
			Sequence:  uint64(index + 1),
			Time:      "1970-01-01T00:00:00Z",
		}
		if buildErr == nil {
			record.Value = "<no-error>"
		} else {
			record.Value = resultsetOrderbySimpleNoOutputInvalidDiagnostic(buildErr, probe.epl)
		}
		expected := resultsetOrderbySimpleNoOutputInvalidError + " [" + probe.epl + "]"
		actual, ok := record.Value.(string)
		if !ok || actual != expected {
			return trace, fmt.Errorf("%s diagnostic drift for %q: expected %q got %q", resultsetOrderbySimpleNoOutputInvalidID, probe.statement, expected, record.Value)
		}
		trace.Records = append(trace.Records, record)
	}
	return trace, nil
}

func resultsetOrderbySimpleNoOutputInvalidDiagnostic(err error, epl string) string {
	message := resultsetOrderbySimpleNoOutputInvalidBareMessage(err)
	if message == "" {
		return "<no-error>"
	}
	return message + " [" + epl + "]"
}

func resultsetOrderbySimpleNoOutputInvalidBareMessage(err error) string {
	message := ""
	for current := err; current != nil; current = errors.Unwrap(current) {
		var typed *esper.Error
		if errors.As(current, &typed) && typed.Message != "" {
			message = typed.Message
		}
	}
	if message == "" && err != nil {
		message = err.Error()
	}
	return message
}

func decodeResultsetOrderbySimpleNoOutputInvalidPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportMarketDataBean":
		return decodeResultsetOrderbySimpleNoOutputInvalidBean(step)
	case "SupportBeanString":
		return decodeResultsetOrderbySimpleNoOutputInvalidString(step)
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOrderbySimpleNoOutputInvalidID, step.EventType)
	}
}

func decodeResultsetOrderbySimpleNoOutputInvalidBean(step compat.Step) (resultsetOrderbySimpleNoOutputInvalidBean, error) {
	if step.EventType != "SupportMarketDataBean" {
		return resultsetOrderbySimpleNoOutputInvalidBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOrderbySimpleNoOutputInvalidBean{}, err
	}
	if err := requireResultsetOrderbySimpleNoOutputInvalidFields(fields, "symbol", "volume", "price"); err != nil {
		return resultsetOrderbySimpleNoOutputInvalidBean{}, err
	}
	symbol, err := decodeResultsetOrderbySimpleNoOutputInvalidStringValue(fields["symbol"], "symbol")
	if err != nil {
		return resultsetOrderbySimpleNoOutputInvalidBean{}, err
	}
	volume, err := decodeResultsetOrderbySimpleNoOutputInvalidInteger(fields["volume"], "volume")
	if err != nil {
		return resultsetOrderbySimpleNoOutputInvalidBean{}, err
	}
	price, err := decodeResultsetOrderbySimpleNoOutputInvalidFloat(fields["price"], "price")
	if err != nil {
		return resultsetOrderbySimpleNoOutputInvalidBean{}, err
	}
	return resultsetOrderbySimpleNoOutputInvalidBean{Symbol: symbol, Price: price, Volume: volume}, nil
}

func decodeResultsetOrderbySimpleNoOutputInvalidString(step compat.Step) (resultsetOrderbySimpleNoOutputInvalidString, error) {
	if step.EventType != "SupportBeanString" {
		return resultsetOrderbySimpleNoOutputInvalidString{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOrderbySimpleNoOutputInvalidString{}, err
	}
	if err := requireResultsetOrderbySimpleNoOutputInvalidFields(fields, "theString"); err != nil {
		return resultsetOrderbySimpleNoOutputInvalidString{}, err
	}
	theString, err := decodeResultsetOrderbySimpleNoOutputInvalidStringValue(fields["theString"], "theString")
	if err != nil {
		return resultsetOrderbySimpleNoOutputInvalidString{}, err
	}
	return resultsetOrderbySimpleNoOutputInvalidString{TheString: theString}, nil
}

func requireResultsetOrderbySimpleNoOutputInvalidFields(object map[string]json.RawMessage, names ...string) error {
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

func decodeResultsetOrderbySimpleNoOutputInvalidStringValue(raw json.RawMessage, name string) (string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must be a string", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func decodeResultsetOrderbySimpleNoOutputInvalidInteger(raw json.RawMessage, name string) (int64, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	number, ok := token.(json.Number)
	if !ok || !resultsetOrderbySimpleNoOutputInvalidIntegerSyntax(string(number)) {
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

func decodeResultsetOrderbySimpleNoOutputInvalidFloat(raw json.RawMessage, name string) (float64, error) {
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

func resultsetOrderbySimpleNoOutputInvalidIntegerSyntax(text string) bool {
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

func validateResultsetOrderbySimpleNoOutputInvalidStringArray(raw json.RawMessage, expected []string, name string) error {
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

func resultsetOrderbySimpleNoOutputInvalidOrdinal(caseName string) int {
	for index, name := range resultsetOrderbySimpleNoOutputInvalidCases {
		if name == caseName {
			return resultsetOrderbySimpleNoOutputInvalidOrdinals[index]
		}
	}
	return -1
}

func resultsetOrderbySimpleNoOutputInvalidRuntimeID(caseName string) string {
	ordinal := resultsetOrderbySimpleNoOutputInvalidOrdinal(caseName)
	if ordinal < 15 || ordinal > 17 {
		return "resultset-orderby-simple-no-output-invalid-unknown"
	}
	return resultsetOrderbySimpleNoOutputInvalidJavaRuntimeIDs[ordinal-15]
}

func resultsetOrderbySimpleNoOutputInvalidExecutionName(caseName string) string {
	ordinal := resultsetOrderbySimpleNoOutputInvalidOrdinal(caseName)
	if ordinal < 15 || ordinal > 17 {
		return "resultset-orderby-simple-no-output-invalid-unknown"
	}
	return resultsetOrderbySimpleNoOutputInvalidJavaExecutions[ordinal-15]
}
