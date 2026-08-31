package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultSetQueryTypeRowForAllHavingAvgJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeRowForAllHavingAvgCase        = "avg-group-window"
	resultSetQueryTypeRowForAllHavingAvgID          = "resultset-querytype-row-for-all-having-avg-group-window"
	resultSetQueryTypeRowForAllHavingAvgDescription = "ResultSetQueryTypeRowForAllHaving ordinal 2: unique-symbol average with having threshold."
	resultSetQueryTypeRowForAllHavingAvgEPL         = "@name('s0') select istream avg(price) as aprice from SupportMarketDataBean#unique(symbol) having avg(price) <= 0"
	resultSetQueryTypeRowForAllHavingAvgStaticID    = "java-45a56190252ed051c35f"
)

var (
	resultSetQueryTypeRowForAllHavingAvgJavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRowForAllHaving.java",
	}
	resultSetQueryTypeRowForAllHavingAvgJavaRuntimeIDs = []string{
		"java-runtime-d20a1ee344797d87678b",
	}
	resultSetQueryTypeRowForAllHavingAvgJavaExecutions = []string{
		"ResultSetQueryTypeAvgRowForAllWHavingGroupWindow",
	}
)

type resultSetQueryTypeRowForAllHavingAvgMarketData struct {
	Symbol string  `esper:"symbol"`
	ID     *string `esper:"id"`
	Price  float64 `esper:"price"`
	Volume *int64  `esper:"volume"`
	Feed   *string `esper:"feed"`
}

func loadResultSetQueryTypeRowForAllHavingAvgScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-having-avg scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, err
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-querytype-row-for-all-having-avg scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, err
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("scenario contains unexpected or missing fields")
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("scenario is missing field %q", name)
		}
	}
	var version, id, description, javaCommit, javaSource string
	for name, target := range map[string]*string{
		"version": &version, "id": &id, "description": &description,
		"javaCommit": &javaCommit, "javaSource": &javaSource,
	} {
		if err := json.Unmarshal(root[name], target); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario %s must be a string", name)
		}
	}
	if version != compat.ScenarioVersion || id != resultSetQueryTypeRowForAllHavingAvgID ||
		description != resultSetQueryTypeRowForAllHavingAvgDescription ||
		javaCommit != resultSetQueryTypeRowForAllHavingAvgJavaCommit ||
		javaSource != resultSetQueryTypeRowForAllHavingAvgJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-having-avg metadata is not pinned")
	}
	var runtimes, names, staticIDs, flags []string
	for name, expected := range map[string][]string{
		"javaRuntimes":  resultSetQueryTypeRowForAllHavingAvgJavaRuntimeIDs,
		"javaNames":     resultSetQueryTypeRowForAllHavingAvgJavaExecutions,
		"javaStaticIds": {resultSetQueryTypeRowForAllHavingAvgStaticID},
		"javaFlags":     {},
	} {
		if err := validateResultSetQueryTypeRowForAllHavingSumStringArray(root[name], expected, name); err != nil {
			return compat.Scenario{}, err
		}
	}
	if err := json.Unmarshal(root["javaRuntimes"], &runtimes); err != nil {
		return compat.Scenario{}, err
	}
	if err := json.Unmarshal(root["javaNames"], &names); err != nil {
		return compat.Scenario{}, err
	}
	if err := json.Unmarshal(root["javaStaticIds"], &staticIDs); err != nil {
		return compat.Scenario{}, err
	}
	if err := json.Unmarshal(root["javaFlags"], &flags); err != nil {
		return compat.Scenario{}, err
	}
	if !equalStrings(runtimes, resultSetQueryTypeRowForAllHavingAvgJavaRuntimeIDs) ||
		!equalStrings(names, resultSetQueryTypeRowForAllHavingAvgJavaExecutions) ||
		!equalStrings(staticIDs, []string{resultSetQueryTypeRowForAllHavingAvgStaticID}) || len(flags) != 0 {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-having-avg Java references are not pinned")
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != 1 {
		return compat.Scenario{}, fmt.Errorf("scenario must contain exactly one case")
	}
	var caseObject map[string]json.RawMessage
	if err := strictObject(rawCases[0], &caseObject); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	caseFields := []string{"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"}
	if len(caseObject) != len(caseFields) {
		return compat.Scenario{}, fmt.Errorf("scenario case contains unexpected or missing fields")
	}
	for _, name := range caseFields {
		if _, ok := caseObject[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("scenario case is missing field %q", name)
		}
	}
	var caseMeta struct {
		Case              string `json:"case"`
		Ordinal           int    `json:"ordinal"`
		RuntimeID         string `json:"runtimeId"`
		ExecutionName     string `json:"executionName"`
		Observation       string `json:"observation"`
		IteratorSnapshots int    `json:"iteratorSnapshots"`
		EPL               string `json:"epl"`
	}
	ordinal, ordinalErr := decodeResultSetQueryTypeRowForAllHavingSumInteger(caseObject["ordinal"], "ordinal")
	iteratorSnapshots, iteratorErr := decodeResultSetQueryTypeRowForAllHavingSumInteger(caseObject["iteratorSnapshots"], "iteratorSnapshots")
	if ordinalErr != nil || iteratorErr != nil || ordinal != 2 || iteratorSnapshots != 0 ||
		json.Unmarshal(rawCases[0], &caseMeta) != nil || caseMeta.Case != resultSetQueryTypeRowForAllHavingAvgCase ||
		caseMeta.RuntimeID != resultSetQueryTypeRowForAllHavingAvgJavaRuntimeIDs[0] ||
		caseMeta.ExecutionName != resultSetQueryTypeRowForAllHavingAvgJavaExecutions[0] ||
		caseMeta.Observation != "listener" || caseMeta.EPL != resultSetQueryTypeRowForAllHavingAvgEPL {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-having-avg case metadata is not pinned")
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 7 {
		return compat.Scenario{}, fmt.Errorf("scenario must contain exactly seven steps")
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		expectedFields := 2
		if index > 0 {
			expectedFields = 3
		}
		if len(object) != expectedFields {
			return compat.Scenario{}, fmt.Errorf("scenario step %d fields are not pinned", index)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: steps}
	if err := validateResultSetQueryTypeRowForAllHavingAvgScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultSetQueryTypeRowForAllHavingAvgScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeRowForAllHavingAvgID || len(scenario.Steps) != 7 {
		return fmt.Errorf("resultset-querytype-row-for-all-having-avg scenario steps are not pinned")
	}
	if marker := scenario.Steps[0]; marker.Op != "case" || marker.Case != resultSetQueryTypeRowForAllHavingAvgCase {
		return fmt.Errorf("scenario must start with case %q", resultSetQueryTypeRowForAllHavingAvgCase)
	}
	expectedSymbols := []string{"A", "A", "B", "C", "C", "C"}
	expectedPrices := []float64{-1, 5, -6, 2, 3, -2}
	for index := range expectedSymbols {
		step := scenario.Steps[index+1]
		if step.Op != "send" || step.EventType != "SupportMarketDataBean" {
			return fmt.Errorf("scenario step %d must send SupportMarketDataBean", index+1)
		}
		marketValue, err := decodeResultSetQueryTypeRowForAllHavingAvgPayload(step)
		if err != nil {
			return fmt.Errorf("scenario step %d: %w", index+1, err)
		}
		market, ok := marketValue.(resultSetQueryTypeRowForAllHavingAvgMarketData)
		if !ok {
			return fmt.Errorf("scenario step %d decoded an unexpected payload type", index+1)
		}
		if market.Symbol != expectedSymbols[index] || market.ID != nil || market.Volume != nil || market.Feed != nil ||
			market.Price != expectedPrices[index] {
			return fmt.Errorf("scenario step %d has unexpected SupportMarketDataBean payload", index+1)
		}
	}
	return nil
}

func runResultSetQueryTypeRowForAllHavingAvgScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeRowForAllHavingAvgScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeRowForAllHavingAvgMarketData](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	symbol := esper.Field[resultSetQueryTypeRowForAllHavingAvgMarketData, string]("symbol")
	price := esper.Field[resultSetQueryTypeRowForAllHavingAvgMarketData, float64]("price")
	avg := esper.Avg[float64](price)
	query := esper.From[resultSetQueryTypeRowForAllHavingAvgMarketData](env, "SupportMarketDataBean").
		Window(esper.Unique(symbol)).
		Aggregate(esper.Alias("aprice", avg)).
		Having(esper.LessOrEqual[float64](avg, esper.Literal(0.0))).
		Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, resultSetQueryTypeRowForAllHavingAvgJavaRuntimeIDs[0])
	if err != nil {
		return compat.Trace{}, err
	}
	defer engine.Close(context.Background())
	return compat.ReplayWithStatements(ctx, engine, statement, scenario, decodeResultSetQueryTypeRowForAllHavingAvgPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-querytype-row-for-all-having-avg statement %q", name)
		}
		return statement, nil
	})
}

func decodeResultSetQueryTypeRowForAllHavingAvgPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportMarketDataBean" {
		return nil, fmt.Errorf("step must send SupportMarketDataBean")
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	if len(fields) != 5 {
		return nil, fmt.Errorf("SupportMarketDataBean payload must contain exactly symbol, id, price, volume, and feed")
	}
	var result resultSetQueryTypeRowForAllHavingAvgMarketData
	if err := json.Unmarshal(fields["symbol"], &result.Symbol); err != nil {
		return nil, fmt.Errorf("symbol must be a string")
	}
	if string(bytes.TrimSpace(fields["id"])) != "null" {
		return nil, fmt.Errorf("id must be JSON null")
	}
	if string(bytes.TrimSpace(fields["volume"])) != "null" {
		return nil, fmt.Errorf("volume must be JSON null")
	}
	if string(bytes.TrimSpace(fields["feed"])) != "null" {
		return nil, fmt.Errorf("feed must be JSON null")
	}
	if err := json.Unmarshal(fields["price"], &result.Price); err != nil || math.IsNaN(result.Price) || math.IsInf(result.Price, 0) {
		return nil, fmt.Errorf("price must be a finite JSON number")
	}
	return result, nil
}
