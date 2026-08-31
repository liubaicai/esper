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
	resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeRowForAllSelectAvgExprStdGroupByCase        = "avg-expr-std-group-by"
	resultSetQueryTypeRowForAllSelectAvgExprStdGroupByID          = "resultset-querytype-row-for-all-select-avg-expr-std-group-by"
	resultSetQueryTypeRowForAllSelectAvgExprStdGroupByDescription = "ResultSetQueryTypeRowForAll ordinal 9: average expression with standard group-by."
	resultSetQueryTypeRowForAllSelectAvgExprStdGroupByEPL         = "@name('s0') select istream avg(price) as aprice from SupportMarketDataBean#groupwin(symbol)#length(2)"
	resultSetQueryTypeRowForAllSelectAvgExprStdGroupByStaticID    = "java-d8d6953f53a7a43d2324"
)

var (
	resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRowForAll.java",
	}
	resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaRuntimeIDs = []string{
		"java-runtime-873d7f4fc2344cd9c631",
	}
	resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaExecutions = []string{
		"ResultSetQueryTypeRowForAllSelectAvgExprStdGroupBy",
	}
)

type resultSetQueryTypeRowForAllSelectAvgExprStdGroupByMarketData struct {
	Symbol string  `esper:"symbol"`
	ID     *string `esper:"id"`
	Price  float64 `esper:"price"`
	Volume *int64  `esper:"volume"`
	Feed   *string `esper:"feed"`
}

func loadResultSetQueryTypeRowForAllSelectAvgExprStdGroupByScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-select-avg-expr-std-group-by scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, err
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-querytype-row-for-all-select-avg-expr-std-group-by scenario: %w", err)
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
	if version != compat.ScenarioVersion || id != resultSetQueryTypeRowForAllSelectAvgExprStdGroupByID ||
		description != resultSetQueryTypeRowForAllSelectAvgExprStdGroupByDescription ||
		javaCommit != resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaCommit ||
		javaSource != resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-select-avg-expr-std-group-by metadata is not pinned")
	}
	var runtimes, names, staticIDs, flags []string
	for name, expected := range map[string][]string{
		"javaRuntimes":  resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaRuntimeIDs,
		"javaNames":     resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaExecutions,
		"javaStaticIds": {resultSetQueryTypeRowForAllSelectAvgExprStdGroupByStaticID},
		"javaFlags":     {},
	} {
		if err := validateResultSetQueryTypeRowForAllHavingSumStringArray(root[name], expected, name); err != nil {
			return compat.Scenario{}, err
		}
	}
	if err := json.Unmarshal(root["javaRuntimes"], &runtimes); err != nil ||
		!equalStrings(runtimes, resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaRuntimeIDs) {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-select-avg-expr-std-group-by Java runtime references are not pinned")
	}
	if err := json.Unmarshal(root["javaNames"], &names); err != nil ||
		!equalStrings(names, resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaExecutions) {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-select-avg-expr-std-group-by Java execution references are not pinned")
	}
	if err := json.Unmarshal(root["javaStaticIds"], &staticIDs); err != nil ||
		!equalStrings(staticIDs, []string{resultSetQueryTypeRowForAllSelectAvgExprStdGroupByStaticID}) {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-select-avg-expr-std-group-by Java static references are not pinned")
	}
	if err := json.Unmarshal(root["javaFlags"], &flags); err != nil || len(flags) != 0 {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-select-avg-expr-std-group-by Java flags are not pinned")
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
	if ordinalErr != nil || iteratorErr != nil || ordinal != 9 || iteratorSnapshots != 0 ||
		json.Unmarshal(rawCases[0], &caseMeta) != nil || caseMeta.Case != resultSetQueryTypeRowForAllSelectAvgExprStdGroupByCase ||
		caseMeta.RuntimeID != resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaRuntimeIDs[0] ||
		caseMeta.ExecutionName != resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaExecutions[0] ||
		caseMeta.Observation != "listener" || caseMeta.EPL != resultSetQueryTypeRowForAllSelectAvgExprStdGroupByEPL {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-select-avg-expr-std-group-by case metadata is not pinned")
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 3 {
		return compat.Scenario{}, fmt.Errorf("scenario must contain exactly three steps")
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
	if err := validateResultSetQueryTypeRowForAllSelectAvgExprStdGroupByScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultSetQueryTypeRowForAllSelectAvgExprStdGroupByScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeRowForAllSelectAvgExprStdGroupByID || len(scenario.Steps) != 3 {
		return fmt.Errorf("resultset-querytype-row-for-all-select-avg-expr-std-group-by scenario steps are not pinned")
	}
	if marker := scenario.Steps[0]; marker.Op != "case" || marker.Case != resultSetQueryTypeRowForAllSelectAvgExprStdGroupByCase {
		return fmt.Errorf("scenario must start with case %q", resultSetQueryTypeRowForAllSelectAvgExprStdGroupByCase)
	}
	expectedSymbols := []string{"A", "B"}
	expectedPrices := []float64{1, 3}
	for index := range expectedSymbols {
		step := scenario.Steps[index+1]
		if step.Op != "send" || step.EventType != "SupportMarketDataBean" {
			return fmt.Errorf("scenario step %d must send SupportMarketDataBean", index+1)
		}
		marketValue, err := decodeResultSetQueryTypeRowForAllSelectAvgExprStdGroupByPayload(step)
		if err != nil {
			return fmt.Errorf("scenario step %d: %w", index+1, err)
		}
		market, ok := marketValue.(resultSetQueryTypeRowForAllSelectAvgExprStdGroupByMarketData)
		if !ok {
			return fmt.Errorf("scenario step %d decoded an unexpected payload type", index+1)
		}
		if market.Symbol != expectedSymbols[index] || market.ID != nil || market.Volume != nil || market.Feed != nil || market.Price != expectedPrices[index] {
			return fmt.Errorf("scenario step %d has unexpected SupportMarketDataBean payload", index+1)
		}
	}
	return nil
}

func runResultSetQueryTypeRowForAllSelectAvgExprStdGroupByScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeRowForAllSelectAvgExprStdGroupByScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeRowForAllSelectAvgExprStdGroupByMarketData](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	symbol := esper.Field[resultSetQueryTypeRowForAllSelectAvgExprStdGroupByMarketData, string]("symbol")
	price := esper.Field[resultSetQueryTypeRowForAllSelectAvgExprStdGroupByMarketData, float64]("price")
	query := esper.From[resultSetQueryTypeRowForAllSelectAvgExprStdGroupByMarketData](env, "SupportMarketDataBean").
		Window(esper.GroupWindow(symbol, esper.LengthWindow(2))).
		Aggregate(esper.Alias("aprice", esper.Avg[float64](price))).
		Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaRuntimeIDs[0])
	if err != nil {
		return compat.Trace{}, err
	}
	defer engine.Close(context.Background())
	return compat.ReplayWithStatements(ctx, engine, statement, scenario, decodeResultSetQueryTypeRowForAllSelectAvgExprStdGroupByPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-querytype-row-for-all-select-avg-expr-std-group-by statement %q", name)
		}
		return statement, nil
	})
}

func decodeResultSetQueryTypeRowForAllSelectAvgExprStdGroupByPayload(step compat.Step) (any, error) {
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
	var result resultSetQueryTypeRowForAllSelectAvgExprStdGroupByMarketData
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
	if string(bytes.TrimSpace(fields["price"])) == "null" {
		return nil, fmt.Errorf("price must be a finite JSON number")
	}
	if err := json.Unmarshal(fields["price"], &result.Price); err != nil || math.IsNaN(result.Price) || math.IsInf(result.Price, 0) {
		return nil, fmt.Errorf("price must be a finite JSON number")
	}
	return result, nil
}
