package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeRowForAllSelectAvgStdGroupByUniCase        = "avg-std-group-by-uni"
	resultSetQueryTypeRowForAllSelectAvgStdGroupByUniID          = "resultset-querytype-row-for-all-select-avg-std-group-by-uni"
	resultSetQueryTypeRowForAllSelectAvgStdGroupByUniDescription = "ResultSetQueryTypeRowForAll ordinal 10: average with standard group-by and unique price view."
	resultSetQueryTypeRowForAllSelectAvgStdGroupByUniEPL         = "@name('s0') select istream average as aprice from SupportMarketDataBean#groupwin(symbol)#length(2)#uni(price)"
	resultSetQueryTypeRowForAllSelectAvgStdGroupByUniStaticID    = "java-7ba8b4893d30071e2fa5"
)

var (
	resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRowForAll.java",
	}
	resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaRuntimeIDs = []string{
		"java-runtime-de3a01d12a22bb33ee43",
	}
	resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaExecutions = []string{
		"ResultSetQueryTypeRowForAllSelectAvgStdGroupByUni",
	}
)

type resultSetQueryTypeRowForAllSelectAvgStdGroupByUniMarketData struct {
	Symbol string  `esper:"symbol"`
	ID     *string `esper:"id"`
	Price  float64 `esper:"price"`
	Volume *int64  `esper:"volume"`
	Feed   *string `esper:"feed"`
}

func loadResultSetQueryTypeRowForAllSelectAvgStdGroupByUniScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-select-avg-std-group-by-uni scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, err
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-querytype-row-for-all-select-avg-std-group-by-uni scenario: %w", err)
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
	if version != compat.ScenarioVersion || id != resultSetQueryTypeRowForAllSelectAvgStdGroupByUniID ||
		description != resultSetQueryTypeRowForAllSelectAvgStdGroupByUniDescription ||
		javaCommit != resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaCommit ||
		javaSource != resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-select-avg-std-group-by-uni metadata is not pinned")
	}
	var runtimes, names, staticIDs, flags []string
	for name, expected := range map[string][]string{
		"javaRuntimes":  resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaRuntimeIDs,
		"javaNames":     resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaExecutions,
		"javaStaticIds": {resultSetQueryTypeRowForAllSelectAvgStdGroupByUniStaticID},
		"javaFlags":     {},
	} {
		if err := validateResultSetQueryTypeRowForAllHavingSumStringArray(root[name], expected, name); err != nil {
			return compat.Scenario{}, err
		}
	}
	if err := json.Unmarshal(root["javaRuntimes"], &runtimes); err != nil ||
		!equalStrings(runtimes, resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaRuntimeIDs) {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-select-avg-std-group-by-uni Java runtime references are not pinned")
	}
	if err := json.Unmarshal(root["javaNames"], &names); err != nil ||
		!equalStrings(names, resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaExecutions) {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-select-avg-std-group-by-uni Java execution references are not pinned")
	}
	if err := json.Unmarshal(root["javaStaticIds"], &staticIDs); err != nil ||
		!equalStrings(staticIDs, []string{resultSetQueryTypeRowForAllSelectAvgStdGroupByUniStaticID}) {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-select-avg-std-group-by-uni Java static references are not pinned")
	}
	if err := json.Unmarshal(root["javaFlags"], &flags); err != nil || len(flags) != 0 {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-select-avg-std-group-by-uni Java flags are not pinned")
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
	if ordinalErr != nil || iteratorErr != nil || ordinal != 10 || iteratorSnapshots != 0 ||
		json.Unmarshal(rawCases[0], &caseMeta) != nil || caseMeta.Case != resultSetQueryTypeRowForAllSelectAvgStdGroupByUniCase ||
		caseMeta.RuntimeID != resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaRuntimeIDs[0] ||
		caseMeta.ExecutionName != resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaExecutions[0] ||
		caseMeta.Observation != "listener" || caseMeta.EPL != resultSetQueryTypeRowForAllSelectAvgStdGroupByUniEPL {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-select-avg-std-group-by-uni case metadata is not pinned")
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 6 {
		return compat.Scenario{}, fmt.Errorf("scenario must contain exactly six steps")
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
	if err := validateResultSetQueryTypeRowForAllSelectAvgStdGroupByUniScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultSetQueryTypeRowForAllSelectAvgStdGroupByUniScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeRowForAllSelectAvgStdGroupByUniID || len(scenario.Steps) != 6 {
		return fmt.Errorf("resultset-querytype-row-for-all-select-avg-std-group-by-uni scenario steps are not pinned")
	}
	if marker := scenario.Steps[0]; marker.Op != "case" || marker.Case != resultSetQueryTypeRowForAllSelectAvgStdGroupByUniCase {
		return fmt.Errorf("scenario must start with case %q", resultSetQueryTypeRowForAllSelectAvgStdGroupByUniCase)
	}
	expectedSymbols := []string{"A", "B", "A", "A", "A"}
	expectedPrices := []float64{1, 3, 3, 10, 20}
	for index := range expectedSymbols {
		step := scenario.Steps[index+1]
		if step.Op != "send" || step.EventType != "SupportMarketDataBean" {
			return fmt.Errorf("scenario step %d must send SupportMarketDataBean", index+1)
		}
		marketValue, err := decodeResultSetQueryTypeRowForAllSelectAvgStdGroupByUniPayload(step)
		if err != nil {
			return fmt.Errorf("scenario step %d: %w", index+1, err)
		}
		market, ok := marketValue.(resultSetQueryTypeRowForAllSelectAvgStdGroupByUniMarketData)
		if !ok {
			return fmt.Errorf("scenario step %d decoded an unexpected payload type", index+1)
		}
		if market.Symbol != expectedSymbols[index] || market.ID != nil || market.Volume != nil ||
			market.Feed != nil || market.Price != expectedPrices[index] {
			return fmt.Errorf("scenario step %d has unexpected SupportMarketDataBean payload", index+1)
		}
	}
	return nil
}

func runResultSetQueryTypeRowForAllSelectAvgStdGroupByUniScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeRowForAllSelectAvgStdGroupByUniScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeRowForAllSelectAvgStdGroupByUniMarketData](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	symbol := esper.Field[resultSetQueryTypeRowForAllSelectAvgStdGroupByUniMarketData, string]("symbol")
	price := esper.Field[resultSetQueryTypeRowForAllSelectAvgStdGroupByUniMarketData, float64]("price")
	average := esper.UnivariateStatistics[float64](price).Average()
	query := esper.From[resultSetQueryTypeRowForAllSelectAvgStdGroupByUniMarketData](env, "SupportMarketDataBean").
		Window(esper.GroupWindow(symbol, esper.LengthWindow(2))).
		Aggregate(esper.Alias("aprice", average)).
		Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaRuntimeIDs[0])
	if err != nil {
		return compat.Trace{}, err
	}
	defer engine.Close(context.Background())

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	callbackAverages := []float64{1, 3, 2, 6.5, 15}
	callbackCount := 0
	sequence := uint64(0)
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		callback := callbackCount + 1
		if callback > len(callbackAverages) {
			return fmt.Errorf("unexpected listener callback after five callbacks")
		}
		if len(batch.New) != 1 || len(batch.Old) != 0 {
			return fmt.Errorf("listener callback %d must contain one new row only", callback)
		}
		if !batch.Time.Equal(time.Unix(0, 0).UTC()) {
			return fmt.Errorf("unexpected callback time at callback %d: %s", callback, batch.Time.UTC().Format(time.RFC3339Nano))
		}
		rows := compat.NormalizeResults(batch.New)
		if len(rows) != 1 || rows[0].Kind != "row" || len(rows[0].Fields) != 1 {
			return fmt.Errorf("listener callback %d has unexpected row shape", callback)
		}
		actual, ok := resultSetQueryTypeRowForAllSelectAvgStdGroupByUniFloat(rows[0].Fields["aprice"])
		if !ok || actual != callbackAverages[callback-1] {
			return fmt.Errorf("listener callback %d average = %v, want %v", callback, rows[0].Fields["aprice"], callbackAverages[callback-1])
		}
		callbackCount = callback
		if callback == 4 {
			// Java calls listenerReset after A/10; discard that callback from
			// the shared trace while retaining the callback/value invariant.
			return nil
		}
		sequence++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: resultSetQueryTypeRowForAllSelectAvgStdGroupByUniCase, Operation: "listener",
			Statement: statement.Name(), Sequence: sequence,
			Time: batch.Time.UTC().Format(time.RFC3339), New: rows,
		})
		return nil
	}); err != nil {
		return compat.Trace{}, err
	}
	for index, step := range scenario.Steps {
		select {
		case <-ctx.Done():
			return trace, ctx.Err()
		default:
		}
		switch step.Op {
		case "case":
			continue
		case "send":
			payload, err := decodeResultSetQueryTypeRowForAllSelectAvgStdGroupByUniPayload(step)
			if err != nil {
				return trace, fmt.Errorf("decode send step %d: %w", index, err)
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		default:
			return trace, fmt.Errorf("unsupported step %q", step.Op)
		}
	}
	if callbackCount != len(callbackAverages) || sequence != 4 {
		return trace, fmt.Errorf("listener callbacks/records = %d/%d, want 5/4", callbackCount, sequence)
	}
	return trace, nil
}

func resultSetQueryTypeRowForAllSelectAvgStdGroupByUniFloat(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, !math.IsNaN(number) && !math.IsInf(number, 0)
	case json.Number:
		result, err := number.Float64()
		return result, err == nil && !math.IsNaN(result) && !math.IsInf(result, 0)
	default:
		return 0, false
	}
}

func decodeResultSetQueryTypeRowForAllSelectAvgStdGroupByUniPayload(step compat.Step) (any, error) {
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
	var result resultSetQueryTypeRowForAllSelectAvgStdGroupByUniMarketData
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
